package custommodels

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const catalogRefreshTimeout = 20 * time.Second

type catalogCache struct {
	mu       sync.Mutex
	entries  map[string]catalogEntry
	inflight map[string]*catalogCall
}

type catalogEntry struct {
	status int
	header http.Header
	body   []byte
}

type catalogCall struct {
	done   chan struct{}
	result catalogResult
}

type catalogResult struct {
	status    int
	header    http.Header
	body      []byte
	err       error
	cacheable bool
}

func isCatalogAction(action string) bool {
	switch action {
	case "loadCodeAssist", "fetchAvailableModels":
		return true
	default:
		return false
	}
}

// serveCatalog isolates catalog refreshes from the language server's request
// context. A canceled caller can leave while the shared refresh completes.
func (p *Proxy) serveCatalog(w http.ResponseWriter, r *http.Request, body []byte) {
	key := catalogCacheKey(r, body)
	fetch := func() catalogResult {
		return p.refreshCatalog(r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone(), body)
	}

	if entry, ok := p.catalog.current(key); ok {
		p.catalog.begin(key, fetch)
		p.writeCatalogResponse(w, entry.status, entry.header, entry.body, r)
		return
	}

	call := p.catalog.begin(key, fetch)
	select {
	case <-r.Context().Done():
		return
	case <-call.done:
	}

	if call.result.cacheable {
		p.writeCatalogResponse(w, call.result.status, call.result.header, call.result.body, r)
		return
	}
	if entry, ok := p.catalog.current(key); ok {
		p.writeCatalogResponse(w, entry.status, entry.header, entry.body, r)
		return
	}
	if call.result.err != nil {
		http.Error(w, "upstream: "+call.result.err.Error(), http.StatusBadGateway)
		return
	}
	p.writeCatalogResponse(w, call.result.status, call.result.header, call.result.body, r)
}

func (c *catalogCache) begin(key string, fetch func() catalogResult) *catalogCall {
	c.mu.Lock()
	if call := c.inflight[key]; call != nil {
		c.mu.Unlock()
		return call
	}
	call := &catalogCall{done: make(chan struct{})}
	if c.inflight == nil {
		c.inflight = make(map[string]*catalogCall)
	}
	c.inflight[key] = call
	c.mu.Unlock()

	go func() {
		result := fetch()
		c.mu.Lock()
		if result.cacheable {
			if c.entries == nil {
				c.entries = make(map[string]catalogEntry)
			}
			c.entries[key] = catalogEntry{
				status: result.status,
				header: result.header,
				body:   result.body,
			}
		}
		call.result = result
		delete(c.inflight, key)
		close(call.done)
		c.mu.Unlock()
	}()
	return call
}

func (c *catalogCache) current(key string) (catalogEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	return entry, ok
}

func (p *Proxy) refreshCatalog(method, path, rawQuery string, headers http.Header, body []byte) catalogResult {
	target := strings.TrimRight(p.Upstream, "/") + path
	if rawQuery != "" {
		target += "?" + rawQuery
	}

	ctx, cancel := context.WithTimeout(context.Background(), catalogRefreshTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return catalogResult{err: err}
	}
	for key, values := range headers {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Host", "")
	req.Host = ""

	client := p.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return catalogResult{err: err}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, proxyBodyLimit+1))
	if err != nil {
		return catalogResult{status: resp.StatusCode, header: resp.Header.Clone(), err: err}
	}
	if len(data) > proxyBodyLimit {
		return catalogResult{status: resp.StatusCode, header: resp.Header.Clone(), err: io.ErrUnexpectedEOF}
	}
	return catalogResult{
		status:    resp.StatusCode,
		header:    resp.Header.Clone(),
		body:      data,
		cacheable: resp.StatusCode == http.StatusOK && validCatalogResponse(path, data),
	}
}

func validCatalogResponse(path string, body []byte) bool {
	if len(bytes.TrimSpace(body)) == 0 || !json.Valid(body) {
		return false
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil || len(payload) == 0 {
		return false
	}
	if _, ok := payload["error"]; ok {
		return false
	}
	if strings.HasSuffix(path, ":fetchAvailableModels") {
		models, ok := payload["models"]
		if ok {
			return nonEmptyCatalogCollection(models)
		}
	}
	return true
}

func nonEmptyCatalogCollection(raw json.RawMessage) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil {
		return len(object) > 0
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		return len(list) > 0
	}
	return false
}

func catalogCacheKey(r *http.Request, body []byte) string {
	digest := sha256.New()
	for _, part := range []string{r.Method, r.URL.Path, r.URL.RawQuery} {
		_, _ = digest.Write([]byte(part))
		_, _ = digest.Write([]byte{0})
	}
	for _, name := range []string{
		"Authorization",
		"Proxy-Authorization",
		"Cookie",
		"X-Goog-Api-Key",
		"X-Api-Key",
	} {
		_, _ = digest.Write([]byte(name))
		_, _ = digest.Write([]byte{0})
		for _, value := range r.Header.Values(name) {
			_, _ = digest.Write([]byte(value))
			_, _ = digest.Write([]byte{0})
		}
	}
	_, _ = digest.Write(body)
	return hex.EncodeToString(digest.Sum(nil))
}

func cleanBaseDisplayName(name string) string {
	s := strings.TrimSpace(name)
	for _, lvl := range []string{"High", "Medium", "Low", "Extra", "Max", "high", "medium", "low", "extra", "max"} {
		suffix := " (" + lvl + ")"
		if strings.HasSuffix(s, suffix) {
			return strings.TrimSpace(strings.TrimSuffix(s, suffix))
		}
	}
	return s
}

func orderedThinkingLevels(levels []string, preferred string) []string {
	if len(levels) == 0 {
		return nil
	}
	pref := strings.ToLower(strings.TrimSpace(preferred))
	if pref == "" {
		pref = "high"
	}
	seen := make(map[string]bool)
	var ordered []string
	if containsFold(levels, pref) {
		ordered = append(ordered, pref)
		seen[pref] = true
	}
	standardOrder := []string{"high", "medium", "low", "xhigh", "max"}
	for _, std := range standardOrder {
		if !seen[std] && containsFold(levels, std) {
			ordered = append(ordered, std)
			seen[std] = true
		}
	}
	for _, l := range levels {
		low := strings.ToLower(strings.TrimSpace(l))
		if !seen[low] && low != "" {
			ordered = append(ordered, low)
			seen[low] = true
		}
	}
	return ordered
}

func containsFold(slice []string, val string) bool {
	for _, item := range slice {
		if strings.EqualFold(strings.TrimSpace(item), val) {
			return true
		}
	}
	return false
}

func (p *Proxy) augmentCatalog(r *http.Request, body []byte) []byte {
	if r == nil || !strings.HasSuffix(r.URL.Path, ":fetchAvailableModels") {
		return body
	}
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	if !strings.Contains(ua, "antigravity/cli") && !strings.Contains(ua, "agy") && r.Header.Get("X-Agy-CLI") != "true" && r.URL.Query().Get("cli") != "true" {
		return body
	}
	if p.gw == nil {
		return body
	}
	enabled := p.gw.enabledModels()
	if len(enabled) == 0 {
		return body
	}

	rawBody := body
	if len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b {
		if gz, err := gzip.NewReader(bytes.NewReader(body)); err == nil {
			if decomp, err := io.ReadAll(gz); err == nil {
				rawBody = decomp
			}
			_ = gz.Close()
		}
	}

	var root map[string]interface{}
	if err := json.Unmarshal(rawBody, &root); err != nil {
		return body
	}

	modelsMap, ok := root["models"].(map[string]interface{})
	if !ok || modelsMap == nil {
		modelsMap = make(map[string]interface{})
		root["models"] = modelsMap
	}

	var customIDs []string
	for _, m := range enabled {
		cliID := m.Name
		if cliID == "" {
			cliID = m.ID
		}
		if cliID == "" {
			continue
		}

		label := m.DisplayName
		if label == "" {
			label = m.Name
		}
		if label == "" {
			label = m.ID
		}

		modelProvider := "MODEL_PROVIDER_DEEPSEEK"
		switch m.ProviderType {
		case ProviderAnthropic:
			modelProvider = "MODEL_PROVIDER_ANTHROPIC"
		case ProviderOpenAI:
			modelProvider = "MODEL_PROVIDER_OPENAI"
		case ProviderGemini, ProviderGCP:
			modelProvider = "MODEL_PROVIDER_GOOGLE"
		}

		baseLabel := cleanBaseDisplayName(label)
		levels := orderedThinkingLevels(m.ThinkingLevels, m.ThinkingLevel)

		if m.SupportsThinking && len(levels) > 1 {
			for _, lvl := range levels {
				variantID := fmt.Sprintf("%s-%s", cliID, strings.ToLower(lvl))
				protoModel := "MODEL_PLACEHOLDER_M318"
				budget := -1
				switch strings.ToLower(lvl) {
				case "high", "max", "xhigh":
					protoModel = "MODEL_PLACEHOLDER_M318"
					budget = -1
				case "medium":
					protoModel = "MODEL_PLACEHOLDER_M319"
					budget = 4000
				case "low":
					protoModel = "MODEL_PLACEHOLDER_M320"
					budget = 1000
				}

				variantMeta := map[string]interface{}{
					"displayName":       fmt.Sprintf("%s (%s)", baseLabel, formatEffortLabel(lvl)),
					"model":             protoModel,
					"modelProvider":     modelProvider,
					"apiProvider":       "API_PROVIDER_GOOGLE_GEMINI",
					"supportsThinking":  true,
					"thinkingBudget":    budget,
					"minThinkingBudget": 32,
					"maxTokens":         1048576,
					"maxOutputTokens":   65536,
				}
				modelsMap[variantID] = variantMeta
				if m.ID != "" && m.ID != cliID {
					modelsMap[fmt.Sprintf("%s-%s", m.ID, strings.ToLower(lvl))] = variantMeta
				}
				customIDs = append(customIDs, variantID)
			}

			baseMeta := map[string]interface{}{
				"displayName":       baseLabel,
				"model":             "MODEL_PLACEHOLDER_M318",
				"modelProvider":     modelProvider,
				"apiProvider":       "API_PROVIDER_GOOGLE_GEMINI",
				"supportsThinking":  true,
				"thinkingBudget":    -1,
				"minThinkingBudget": 32,
				"maxTokens":         1048576,
				"maxOutputTokens":   65536,
			}
			modelsMap[cliID] = baseMeta
			if m.ID != "" && m.ID != cliID {
				modelsMap[m.ID] = baseMeta
			}
		} else {
			customMeta := map[string]interface{}{
				"displayName":      label,
				"model":            "MODEL_PLACEHOLDER_M318",
				"modelProvider":    modelProvider,
				"apiProvider":      "API_PROVIDER_GOOGLE_GEMINI",
				"supportsThinking": m.SupportsThinking,
				"maxTokens":        1048576,
				"maxOutputTokens":  65536,
			}
			if len(m.ThinkingLevels) > 0 {
				customMeta["thinkingLevels"] = m.ThinkingLevels
			}
			modelsMap[cliID] = customMeta
			if m.ID != "" && m.ID != cliID {
				modelsMap[m.ID] = customMeta
			}
			customIDs = append(customIDs, cliID)
		}
	}

	sorts, ok := root["agentModelSorts"].([]interface{})
	if !ok || len(sorts) == 0 {
		root["agentModelSorts"] = []interface{}{
			map[string]interface{}{
				"displayName": "Recommended",
				"groups": []interface{}{
					map[string]interface{}{
						"modelIds": customIDs,
					},
				},
			},
		}
	} else if firstSort, ok := sorts[0].(map[string]interface{}); ok {
		if groups, ok := firstSort["groups"].([]interface{}); ok && len(groups) > 0 {
			if firstGroup, ok := groups[0].(map[string]interface{}); ok {
				existing, _ := firstGroup["modelIds"].([]interface{})
				seen := make(map[string]bool)
				for _, item := range existing {
					if s, ok := item.(string); ok {
						seen[s] = true
					}
				}
				for _, cid := range customIDs {
					if !seen[cid] {
						existing = append(existing, cid)
						seen[cid] = true
					}
				}
				firstGroup["modelIds"] = existing
			}
		}
	}

	augmented, err := json.Marshal(root)
	if err != nil {
		return body
	}
	return augmented
}

func (p *Proxy) writeCatalogResponse(w http.ResponseWriter, status int, headers http.Header, body []byte, r *http.Request) {
	if status == http.StatusOK {
		body = p.augmentCatalog(r, body)
	}
	for key, values := range headers {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
