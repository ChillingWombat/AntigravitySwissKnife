package custommodels

import (
	"bytes"
	"context"
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
		writeCatalogResponse(w, entry.status, entry.header, entry.body)
		return
	}

	call := p.catalog.begin(key, fetch)
	select {
	case <-r.Context().Done():
		return
	case <-call.done:
	}

	if call.result.cacheable {
		writeCatalogResponse(w, call.result.status, call.result.header, call.result.body)
		return
	}
	if entry, ok := p.catalog.current(key); ok {
		writeCatalogResponse(w, entry.status, entry.header, entry.body)
		return
	}
	if call.result.err != nil {
		http.Error(w, "upstream: "+call.result.err.Error(), http.StatusBadGateway)
		return
	}
	writeCatalogResponse(w, call.result.status, call.result.header, call.result.body)
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

func writeCatalogResponse(w http.ResponseWriter, status int, headers http.Header, body []byte) {
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
