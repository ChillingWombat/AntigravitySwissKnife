package quota

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// ModelOption represents a model available for selection in the UI.
type ModelOption struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"display_name"`
	SupportsThinking bool     `json:"supports_thinking,omitempty"`
	ThinkingLevels   []string `json:"thinking_levels,omitempty"`
	Recommended      bool     `json:"recommended,omitempty"`
	Provider         string   `json:"provider,omitempty"`
}

// AvailableModelsCatalog encapsulates the discovered and updated models.
type AvailableModelsCatalog struct {
	Success          bool          `json:"success"`
	GeminiModels     []ModelOption `json:"gemini_models"`
	NonGeminiModels  []ModelOption `json:"non_gemini_models"`
	DefaultGemini    string        `json:"default_gemini"`
	DefaultNonGemini string        `json:"default_non_gemini"`
	Timestamp        time.Time     `json:"timestamp"`
}

var (
	catalogMu               sync.RWMutex
	cachedCatalog           *AvailableModelsCatalog
	cachedAt                time.Time
	cachedWithAccount       bool
	DisableLiveCDPDiscovery bool
)

// DefaultBaseGeminiModels returns the base up-to-date Gemini models list.
func DefaultBaseGeminiModels() []ModelOption {
	return []ModelOption{
		{
			ID:               "gemini-3.8-flash-high",
			DisplayName:      "Gemini 3.8 Flash (High)",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Recommended:      true,
			Provider:         "Google",
		},
		{
			ID:               "gemini-3.8-flash",
			DisplayName:      "Gemini 3.8 Flash",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Recommended:      true,
			Provider:         "Google",
		},
		{
			ID:               "gemini-3.8-flash-medium",
			DisplayName:      "Gemini 3.8 Flash (Medium)",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "Google",
		},
		{
			ID:               "gemini-3.8-flash-low",
			DisplayName:      "Gemini 3.8 Flash (Low)",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "Google",
		},
		{
			ID:               "gemini-3.7-flash-high",
			DisplayName:      "Gemini 3.7 Flash (High)",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "Google",
		},
		{
			ID:               "gemini-pro-agent",
			DisplayName:      "Gemini 3.1 Pro (High)",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "Google",
		},
		{
			ID:               "gemini-3.1-pro-low",
			DisplayName:      "Gemini 3.1 Pro (Low)",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "Google",
		},
		{
			ID:               "gemini-2.5-pro",
			DisplayName:      "Gemini 2.5 Pro",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "Google",
		},
		{
			ID:               "gemini-2.5-flash",
			DisplayName:      "Gemini 2.5 Flash",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "Google",
		},
		{
			ID:               "gemini-2.0-flash",
			DisplayName:      "Gemini 2.0 Flash",
			Provider:         "Google",
		},
	}
}

// DefaultBaseNonGeminiModels returns the base up-to-date Non-Gemini native models list.
func DefaultBaseNonGeminiModels() []ModelOption {
	return []ModelOption{
		{
			ID:               "claude-opus-4-6",
			DisplayName:      "Claude Opus 4.6",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Recommended:      true,
			Provider:         "Anthropic",
		},
		{
			ID:               "claude-opus-4-6-thinking",
			DisplayName:      "Claude Opus 4.6 (Thinking)",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Recommended:      true,
			Provider:         "Anthropic",
		},
		{
			ID:               "claude-sonnet-4-6",
			DisplayName:      "Claude Sonnet 4.6 (Thinking)",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Recommended:      true,
			Provider:         "Anthropic",
		},
		{
			ID:               "gpt-oss-120b-medium",
			DisplayName:      "GPT-OSS 120B (Medium)",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "OpenAI",
		},
		{
			ID:               "claude-3-7-sonnet",
			DisplayName:      "Claude 3.7 Sonnet",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "Anthropic",
		},
		{
			ID:               "claude-3-5-sonnet",
			DisplayName:      "Claude 3.5 Sonnet",
			Provider:         "Anthropic",
		},
		{
			ID:               "claude-3-5-haiku",
			DisplayName:      "Claude 3.5 Haiku",
			Provider:         "Anthropic",
		},
		{
			ID:               "gpt-4o",
			DisplayName:      "OpenAI GPT-4o",
			Provider:         "OpenAI",
		},
		{
			ID:               "o3-mini",
			DisplayName:      "OpenAI o3-mini",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "OpenAI",
		},
	}
}

// GetAvailableModelCatalog returns available models, fetching live models if possible.
func GetAvailableModelCatalog(acc *keyring.Account, force bool) *AvailableModelsCatalog {
	hasAcc := acc != nil && (acc.AccessToken != "" || acc.RefreshToken != "")

	catalogMu.RLock()
	if !force && cachedCatalog != nil && time.Since(cachedAt) < 2*time.Minute {
		if !(hasAcc && !cachedWithAccount) {
			defer catalogMu.RUnlock()
			return cachedCatalog
		}
	}
	catalogMu.RUnlock()

	catalogMu.Lock()
	defer catalogMu.Unlock()

	// Double check cache
	if !force && cachedCatalog != nil && time.Since(cachedAt) < 2*time.Minute {
		if !(hasAcc && !cachedWithAccount) {
			return cachedCatalog
		}
	}

	cat := &AvailableModelsCatalog{
		Success:          true,
		GeminiModels:     DefaultBaseGeminiModels(),
		NonGeminiModels:  DefaultBaseNonGeminiModels(),
		DefaultGemini:    "gemini-3.8-flash-high",
		DefaultNonGemini: "claude-opus-4-6",
		Timestamp:        time.Now(),
	}

	liveSuccess := false

	// 1. Prioritize live host Antigravity IDE discovery via CDP
	if !DisableLiveCDPDiscovery {
		if liveGemini, liveNonGemini, defaultAgent, cdpErr := FetchModelsFromLiveAntigravity(); cdpErr == nil && len(liveGemini) > 0 {
			cat.GeminiModels = liveGemini
			if len(liveNonGemini) > 0 {
				cat.NonGeminiModels = liveNonGemini
			}
			if defaultAgent != "" {
				cat.DefaultGemini = defaultAgent
			}
			liveSuccess = true
		}
	}
	if !liveSuccess && hasAcc {
		// 2. Fall back to CloudCode internal fetchAvailableModels
		liveModels, defaultAgentID, err := fetchLiveAvailableModels(acc)
		if err == nil && len(liveModels) > 0 {
			mergeLiveModels(cat, liveModels)
			if defaultAgentID != "" {
				cat.DefaultGemini = defaultAgentID
			}
			liveSuccess = true
		}
	}

	cachedCatalog = cat
	cachedAt = time.Now()
	cachedWithAccount = liveSuccess
	return cat
}

// FetchModelsFromLiveAntigravity discovers live models directly from the running host Antigravity IDE via CDP.
func FetchModelsFromLiveAntigravity() (gemini []ModelOption, nonGemini []ModelOption, defaultAgent string, err error) {
	inj := gui.NewInjector(0)
	port, err := inj.FindDevToolsPort()
	if err != nil {
		return nil, nil, "", err
	}
	pages, err := inj.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		return nil, nil, "", fmt.Errorf("no page targets found on port %d", port)
	}
	ws := pages[0].WebSocketDebuggerURL

	// 1. Click model trigger to open menu if not already open
	_, _ = inj.ExecuteScript(ws, `(() => {
		document.dispatchEvent(new KeyboardEvent('keydown', {'key': 'Escape'}));
		setTimeout(() => {
			const trigger = document.querySelector('[data-testid="model-selector-trigger"]');
			if (trigger) trigger.click();
		}, 60);
	})()`)

	time.Sleep(250 * time.Millisecond)

	// 2. Query open menu
	queryScript := `(() => {
		const allMenus = Array.from(document.querySelectorAll('[role="menu"], [data-radix-menu-content]'));
		const modelMenu = allMenus.find(m => m.textContent && (m.textContent.includes("Gemini") || m.textContent.includes("Model")));
		if (!modelMenu) return { active: "", items: [] };

		const trigger = document.querySelector('[data-testid="model-selector-trigger"]');
		const active = trigger && trigger.innerText ? trigger.innerText.trim() : "";

		const items = Array.from(modelMenu.querySelectorAll('[role="menuitem"], [data-testid="model-selector-item"], div[tabindex]'));
		const list = [];
		for (const el of items) {
			const text = (el.innerText ? el.innerText.trim() : el.textContent.trim());
			if (!text || text === "View Usage" || text === "Model") continue;
			if (!text.includes("Gemini") && !text.includes("Claude") && !text.includes("GPT") && !el.hasAttribute('data-model-label')) continue;
			const modelLabel = el.getAttribute('data-model-label') || "";
			list.push({ text, modelLabel });
		}
		document.dispatchEvent(new KeyboardEvent('keydown', {'key': 'Escape'}));
		return { active, items: list };
	})()`

	res, err := inj.ExecuteScript(ws, queryScript)
	if err != nil {
		return nil, nil, "", err
	}

	data, _ := json.Marshal(res)
	var out struct {
		Active string `json:"active"`
		Items  []struct {
			Text       string `json:"text"`
			ModelLabel string `json:"modelLabel"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, nil, "", err
	}
	if len(out.Items) == 0 {
		return nil, nil, "", fmt.Errorf("no model items discovered in host IDE")
	}

	for _, it := range out.Items {
		lines := strings.Split(it.Text, "\n")
		baseName := strings.TrimSpace(lines[0])
		sub := ""
		if len(lines) > 1 {
			sub = strings.TrimSpace(lines[1])
		}

		isThinkingLevel := sub == "High" || sub == "Medium" || sub == "Low" || sub == "Off"
		displayName := baseName
		cleanID := ""

		if isThinkingLevel {
			displayName = fmt.Sprintf("%s (%s)", baseName, sub)
			cleanID = strings.ToLower(strings.ReplaceAll(baseName, " ", "-")) + "-" + strings.ToLower(sub)
		} else if it.ModelLabel != "" {
			displayName = it.ModelLabel
			cleanID = strings.ToLower(strings.ReplaceAll(it.ModelLabel, " ", "-"))
		} else {
			cleanID = strings.ToLower(strings.ReplaceAll(baseName, " ", "-"))
		}

		cleanID = strings.ReplaceAll(cleanID, "(thinking)", "")
		cleanID = strings.ReplaceAll(cleanID, "(", "")
		cleanID = strings.ReplaceAll(cleanID, ")", "")
		if strings.Contains(cleanID, "claude") {
			cleanID = strings.ReplaceAll(cleanID, ".", "-")
		}
		for strings.Contains(cleanID, "--") {
			cleanID = strings.ReplaceAll(cleanID, "--", "-")
		}
		cleanID = strings.Trim(cleanID, "-")

		lowerName := strings.ToLower(displayName)
		isGemini := strings.Contains(lowerName, "gemini")

		opt := ModelOption{
			ID:               cleanID,
			DisplayName:      displayName,
			SupportsThinking: isThinkingLevel || strings.Contains(lowerName, "thinking"),
			Recommended:      strings.Contains(displayName, "3.8 Flash"),
		}

		if isGemini {
			opt.Provider = "Google"
			if opt.SupportsThinking {
				opt.ThinkingLevels = []string{"off", "low", "medium", "high"}
			}
			gemini = append(gemini, opt)
		} else {
			if strings.Contains(lowerName, "claude") {
				opt.Provider = "Anthropic"
			} else {
				opt.Provider = "OpenAI"
			}
			nonGemini = append(nonGemini, opt)
		}
	}

	// Ensure Claude Opus is first in nonGemini if present
	for i, m := range nonGemini {
		if strings.Contains(strings.ToLower(m.ID), "claude-opus") {
			if i > 0 {
				nonGemini[0], nonGemini[i] = nonGemini[i], nonGemini[0]
			}
			break
		}
	}

	if len(gemini) > 0 {
		defaultAgent = gemini[0].ID
	}
	return gemini, nonGemini, defaultAgent, nil
}

// isInternalModelID identifies internal utility, autocomplete, or quota-bucket models.
func isInternalModelID(id string) bool {
	low := strings.ToLower(id)
	return strings.HasPrefix(low, "tab_") ||
		strings.HasPrefix(low, "tab-") ||
		strings.HasPrefix(low, "chat_") ||
		strings.HasSuffix(low, "_tiered") ||
		strings.HasSuffix(low, "-tiered") ||
		strings.Contains(low, "observer")
}

// detectModelProvider inspects upstream model/api providers or ID conventions.
func detectModelProvider(id, modelProvider, apiProvider string) string {
	upModel := strings.ToUpper(modelProvider)
	upAPI := strings.ToUpper(apiProvider)
	lowerID := strings.ToLower(id)

	if strings.Contains(upModel, "ANTHROPIC") || strings.Contains(upAPI, "ANTHROPIC") || strings.Contains(lowerID, "claude") {
		return "Anthropic"
	}
	if strings.Contains(upModel, "OPENAI") || strings.Contains(upAPI, "OPENAI") || strings.Contains(lowerID, "gpt") || strings.Contains(lowerID, "o3") || strings.Contains(lowerID, "o1") {
		return "OpenAI"
	}
	if strings.Contains(upModel, "GOOGLE") || strings.Contains(upAPI, "GOOGLE") || strings.Contains(lowerID, "gemini") {
		return "Google"
	}
	return "Native"
}

// fetchLiveAvailableModels attempts to query CloudCode's fetchAvailableModels endpoint.
func fetchLiveAvailableModels(acc *keyring.Account) ([]ModelOption, string, error) {
	if acc == nil {
		return nil, "", fmt.Errorf("nil account")
	}

	token := acc.AccessToken
	if token == "" && acc.RefreshToken != "" {
		newToken, err := RefreshGoogleToken(acc.RefreshToken, "", "")
		if err == nil && newToken != "" {
			token = newToken
			acc.AccessToken = newToken
		}
	}

	if token == "" {
		return nil, "", fmt.Errorf("no access token available")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	payload, _ := json.Marshal(map[string]interface{}{"project": ""})

	var lastErr error
	for _, endpoint := range CloudCodeModelsURLs {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "antigravity/2.19.1 linux/amd64")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusUnauthorized && acc.RefreshToken != "" {
			resp.Body.Close()
			newToken, refErr := RefreshGoogleToken(acc.RefreshToken, "", "")
			if refErr == nil && newToken != "" {
				token = newToken
				acc.AccessToken = newToken
				retryReq, rErr := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(payload))
				if rErr == nil {
					retryReq.Header.Set("Authorization", "Bearer "+token)
					retryReq.Header.Set("Content-Type", "application/json")
					retryReq.Header.Set("User-Agent", "antigravity/2.19.1 linux/amd64")
					resp, err = client.Do(retryReq)
					if err != nil {
						lastErr = err
						continue
					}
				}
			}
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		var data struct {
			DefaultAgentModelID string `json:"defaultAgentModelId"`
			AgentModelSorts     []struct {
				DisplayName string `json:"displayName"`
				Groups      []struct {
					ModelIDs []string `json:"modelIds"`
				} `json:"groups"`
			} `json:"agentModelSorts"`
			DeprecatedModelIDs map[string]struct {
				NewModelID string `json:"newModelId"`
			} `json:"deprecatedModelIds"`
			TabModelIDs                []string `json:"tabModelIds"`
			CommandModelIDs            []string `json:"commandModelIds"`
			CommitMessageModelIDs      []string `json:"commitMessageModelIds"`
			ImageGenerationModelIDs    []string `json:"imageGenerationModelIds"`
			MqueryModelIDs             []string `json:"mqueryModelIds"`
			WebSearchModelIDs          []string `json:"webSearchModelIds"`
			AudioTranscriptionModelIDs []string `json:"audioTranscriptionModelIds"`
			TieredModelIDs             struct {
				FlashLite  []string `json:"flashLite"`
				Flash      []string `json:"flash"`
				Pro        []string `json:"pro"`
				Ultra      []string `json:"ultra"`
				Enterprise []string `json:"enterprise"`
			} `json:"tieredModelIds"`
			TieredModelIDsSnake struct {
				FlashLite  []string `json:"flash_lite"`
				Flash      []string `json:"flash"`
				Pro        []string `json:"pro"`
				Ultra      []string `json:"ultra"`
				Enterprise []string `json:"enterprise"`
			} `json:"tiered_model_ids"`
			Models map[string]struct {
				DisplayName       string   `json:"displayName"`
				DisplayNameSnake  string   `json:"display_name"`
				SupportsThinking  bool     `json:"supportsThinking"`
				SupportsThinkingS bool     `json:"supports_thinking"`
				Recommended       bool     `json:"recommended"`
				ThinkingLevels    []string `json:"thinkingLevels"`
				ThinkingLevelsS   []string `json:"thinking_levels"`
				Model             string   `json:"model"`
				ModelProvider     string   `json:"modelProvider"`
				APIProvider       string   `json:"apiProvider"`
			} `json:"models"`
		}

		if err := json.Unmarshal(body, &data); err != nil {
			lastErr = err
			continue
		}

		// Build exclusion set of non-chat subsystem models and deprecated models
		excluded := make(map[string]bool)
		for id := range data.DeprecatedModelIDs {
			excluded[strings.TrimPrefix(strings.TrimSpace(id), "models/")] = true
		}
		for _, id := range data.TabModelIDs {
			excluded[strings.TrimPrefix(strings.TrimSpace(id), "models/")] = true
		}
		for _, id := range data.CommandModelIDs {
			excluded[strings.TrimPrefix(strings.TrimSpace(id), "models/")] = true
		}
		for _, id := range data.CommitMessageModelIDs {
			excluded[strings.TrimPrefix(strings.TrimSpace(id), "models/")] = true
		}
		for _, id := range data.ImageGenerationModelIDs {
			excluded[strings.TrimPrefix(strings.TrimSpace(id), "models/")] = true
		}
		for _, id := range data.MqueryModelIDs {
			excluded[strings.TrimPrefix(strings.TrimSpace(id), "models/")] = true
		}
		for _, id := range data.WebSearchModelIDs {
			excluded[strings.TrimPrefix(strings.TrimSpace(id), "models/")] = true
		}
		for _, id := range data.AudioTranscriptionModelIDs {
			excluded[strings.TrimPrefix(strings.TrimSpace(id), "models/")] = true
		}

		var options []ModelOption
		seen := make(map[string]bool)

		// 1. Prioritize models listed in agentModelSorts (the canonical list Antigravity Desktop renders)
		for _, sortGroup := range data.AgentModelSorts {
			for _, grp := range sortGroup.Groups {
				for _, mID := range grp.ModelIDs {
					cleanID := strings.TrimPrefix(strings.TrimSpace(mID), "models/")
					if cleanID == "" || seen[cleanID] || excluded[cleanID] || isInternalModelID(cleanID) {
						continue
					}
					seen[cleanID] = true

					mMeta := data.Models[mID]
					disp := mMeta.DisplayName
					if disp == "" {
						disp = mMeta.DisplayNameSnake
					}
					if disp == "" {
						disp = formatDisplayNameFromID(cleanID)
					}

					supports := mMeta.SupportsThinking || mMeta.SupportsThinkingS
					levels := mMeta.ThinkingLevels
					if len(levels) == 0 {
						levels = mMeta.ThinkingLevelsS
					}
					if len(levels) == 0 && supports {
						levels = []string{"off", "low", "medium", "high"}
					}

					options = append(options, ModelOption{
						ID:               cleanID,
						DisplayName:      disp,
						SupportsThinking: supports,
						ThinkingLevels:   levels,
						Recommended:      mMeta.Recommended,
						Provider:         detectModelProvider(cleanID, mMeta.ModelProvider, mMeta.APIProvider),
					})
				}
			}
		}

		// 2. Process models from explicit models map if they have a non-empty display name and are not excluded
		for mID, mMeta := range data.Models {
			cleanID := strings.TrimPrefix(strings.TrimSpace(mID), "models/")
			if cleanID == "" || seen[cleanID] || excluded[cleanID] || isInternalModelID(cleanID) {
				continue
			}
			disp := mMeta.DisplayName
			if disp == "" {
				disp = mMeta.DisplayNameSnake
			}
			if disp == "" {
				// Models without a display name are internal non-chat endpoints
				continue
			}
			seen[cleanID] = true

			supports := mMeta.SupportsThinking || mMeta.SupportsThinkingS
			levels := mMeta.ThinkingLevels
			if len(levels) == 0 {
				levels = mMeta.ThinkingLevelsS
			}
			if len(levels) == 0 && supports {
				levels = []string{"off", "low", "medium", "high"}
			}

			options = append(options, ModelOption{
				ID:               cleanID,
				DisplayName:      disp,
				SupportsThinking: supports,
				ThinkingLevels:   levels,
				Recommended:      mMeta.Recommended,
				Provider:         detectModelProvider(cleanID, mMeta.ModelProvider, mMeta.APIProvider),
			})
		}

		// 3. Process tiered models only if not internal and not excluded
		allTiered := append(data.TieredModelIDs.FlashLite, data.TieredModelIDsSnake.FlashLite...)
		allTiered = append(allTiered, append(data.TieredModelIDs.Flash, data.TieredModelIDsSnake.Flash...)...)
		allTiered = append(allTiered, append(data.TieredModelIDs.Pro, data.TieredModelIDsSnake.Pro...)...)
		allTiered = append(allTiered, append(data.TieredModelIDs.Ultra, data.TieredModelIDsSnake.Ultra...)...)
		allTiered = append(allTiered, append(data.TieredModelIDs.Enterprise, data.TieredModelIDsSnake.Enterprise...)...)

		for _, tID := range allTiered {
			cleanID := strings.TrimPrefix(strings.TrimSpace(tID), "models/")
			if cleanID == "" || seen[cleanID] || excluded[cleanID] || isInternalModelID(cleanID) {
				continue
			}
			mMeta, hasMeta := data.Models[cleanID]
			disp := ""
			if hasMeta {
				disp = mMeta.DisplayName
				if disp == "" {
					disp = mMeta.DisplayNameSnake
				}
			}
			if disp == "" {
				// Require hyphenated model naming for any tier fallback
				if !strings.Contains(cleanID, "-") {
					continue
				}
				disp = formatDisplayNameFromID(cleanID)
			}
			seen[cleanID] = true
			supports := strings.Contains(cleanID, "flash") || strings.Contains(cleanID, "pro") || strings.Contains(cleanID, "ultra")
			var levels []string
			if supports {
				levels = []string{"off", "low", "medium", "high"}
			}
			options = append(options, ModelOption{
				ID:               cleanID,
				DisplayName:      disp,
				SupportsThinking: supports,
				ThinkingLevels:   levels,
				Provider:         detectModelProvider(cleanID, mMeta.ModelProvider, mMeta.APIProvider),
			})
		}

		if len(options) > 0 {
			cleanDefaultID := strings.TrimPrefix(strings.TrimSpace(data.DefaultAgentModelID), "models/")
			return options, cleanDefaultID, nil
		}
	}

	return nil, "", lastErr
}

func mergeLiveModels(cat *AvailableModelsCatalog, live []ModelOption) {
	geminiMap := make(map[string]int)
	for i, m := range cat.GeminiModels {
		geminiMap[m.ID] = i
	}
	nonGeminiMap := make(map[string]int)
	for i, m := range cat.NonGeminiModels {
		nonGeminiMap[m.ID] = i
	}

	for _, m := range live {
		m.ID = strings.TrimPrefix(strings.TrimSpace(m.ID), "models/")
		if m.ID == "" || isInternalModelID(m.ID) {
			continue
		}
		lower := strings.ToLower(m.ID + " " + m.DisplayName)
		isGemini := m.Provider == "Google" || strings.Contains(lower, "gemini")

		if isGemini {
			if idx, exists := geminiMap[m.ID]; exists {
				if m.DisplayName != "" {
					cat.GeminiModels[idx].DisplayName = m.DisplayName
				}
				if m.SupportsThinking {
					cat.GeminiModels[idx].SupportsThinking = true
				}
				if len(m.ThinkingLevels) > 0 {
					cat.GeminiModels[idx].ThinkingLevels = m.ThinkingLevels
				}
				if m.Recommended {
					cat.GeminiModels[idx].Recommended = true
				}
			} else {
				geminiMap[m.ID] = len(cat.GeminiModels)
				cat.GeminiModels = append(cat.GeminiModels, m)
			}
		} else {
			// Reject internal background models that do not contain gemini in their name from leaking into non-gemini
			if m.Provider == "Native" && (strings.Contains(lower, "chat_") || strings.Contains(lower, "tab_") || strings.Contains(lower, "_tiered")) {
				continue
			}
			if idx, exists := nonGeminiMap[m.ID]; exists {
				if m.DisplayName != "" {
					cat.NonGeminiModels[idx].DisplayName = m.DisplayName
				}
				if m.SupportsThinking {
					cat.NonGeminiModels[idx].SupportsThinking = true
				}
				if len(m.ThinkingLevels) > 0 {
					cat.NonGeminiModels[idx].ThinkingLevels = m.ThinkingLevels
				}
				if m.Recommended {
					cat.NonGeminiModels[idx].Recommended = true
				}
			} else {
				nonGeminiMap[m.ID] = len(cat.NonGeminiModels)
				cat.NonGeminiModels = append(cat.NonGeminiModels, m)
			}
		}
	}
}

func detectProvider(id string) string {
	lower := strings.ToLower(id)
	if strings.Contains(lower, "gemini") {
		return "Google"
	}
	if strings.Contains(lower, "claude") {
		return "Anthropic"
	}
	if strings.Contains(lower, "gpt") || strings.Contains(lower, "o3") || strings.Contains(lower, "o1") {
		return "OpenAI"
	}
	return "Native"
}

func formatDisplayNameFromID(id string) string {
	id = strings.TrimPrefix(id, "models/")
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
