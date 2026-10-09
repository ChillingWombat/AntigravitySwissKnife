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
	catalogMu         sync.RWMutex
	cachedCatalog     *AvailableModelsCatalog
	cachedAt          time.Time
	cachedWithAccount bool
)

// DefaultBaseGeminiModels returns the base up-to-date Gemini models list,
// mirroring the CloudCode picker order (agentModelSorts).
func DefaultBaseGeminiModels() []ModelOption {
	think := []string{"off", "low", "medium", "high"}
	return []ModelOption{
		{ID: "gemini-3.8-flash-high", DisplayName: "Gemini 3.8 Flash (High)", SupportsThinking: true, ThinkingLevels: think, Recommended: true, Provider: "Google"},
		{ID: "gemini-3.8-flash-medium", DisplayName: "Gemini 3.8 Flash (Medium)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
		{ID: "gemini-3.8-flash-low", DisplayName: "Gemini 3.8 Flash (Low)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
		{ID: "gemini-3.7-flash-high", DisplayName: "Gemini 3.7 Flash (High)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
		{ID: "gemini-3.7-flash-medium", DisplayName: "Gemini 3.7 Flash (Medium)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
		{ID: "gemini-3.7-flash-low", DisplayName: "Gemini 3.7 Flash (Low)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
		{ID: "gemini-3.6-flash-high", DisplayName: "Gemini 3.6 Flash (High)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
		{ID: "gemini-3.6-flash-medium", DisplayName: "Gemini 3.6 Flash (Medium)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
		{ID: "gemini-3.6-flash-low", DisplayName: "Gemini 3.6 Flash (Low)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
		{ID: "gemini-pro-agent", DisplayName: "Gemini 3.1 Pro (High)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
		{ID: "gemini-3.1-pro-low", DisplayName: "Gemini 3.1 Pro (Low)", SupportsThinking: true, ThinkingLevels: think, Provider: "Google"},
	}
}

// DefaultBaseNonGeminiModels returns the base up-to-date Non-Gemini native models list,
// mirroring the CloudCode picker order (agentModelSorts).
func DefaultBaseNonGeminiModels() []ModelOption {
	think := []string{"off", "low", "medium", "high"}
	return []ModelOption{
		{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6 (Thinking)", SupportsThinking: true, ThinkingLevels: think, Recommended: true, Provider: "Anthropic"},
		{ID: "claude-opus-4-6-thinking", DisplayName: "Claude Opus 4.6 (Thinking)", SupportsThinking: true, ThinkingLevels: think, Recommended: true, Provider: "Anthropic"},
		{ID: "gpt-oss-120b-medium", DisplayName: "GPT-OSS 120B (Medium)", SupportsThinking: true, ThinkingLevels: think, Provider: "OpenAI"},
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
		DefaultNonGemini: "claude-opus-4-6-thinking",
		Timestamp:        time.Now(),
	}

	liveSuccess := false

	if hasAcc {
		// Live CloudCode fetchAvailableModels: the picker order is authoritative,
		// so a successful fetch fully replaces the offline baseline lists.
		liveGemini, liveNonGemini, defaultAgentID, err := fetchLiveAvailableModels(acc)
		if err == nil && (len(liveGemini) > 0 || len(liveNonGemini) > 0) {
			if len(liveGemini) > 0 {
				cat.GeminiModels = liveGemini
			}
			if len(liveNonGemini) > 0 {
				cat.NonGeminiModels = liveNonGemini
			}
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

// fetchLiveAvailableModels queries CloudCode's fetchAvailableModels endpoint and
// builds the picker catalog exclusively from agentModelSorts[].groups[].modelIds,
// in order. MODEL_PROVIDER_GOOGLE entries go to the gemini list, everything else
// to nonGemini. Non-picker ids (tab/command/commit/image/tiered buckets, etc.)
// are never included.
func fetchLiveAvailableModels(acc *keyring.Account) (gemini []ModelOption, nonGemini []ModelOption, defaultID string, err error) {
	if acc == nil {
		return nil, nil, "", fmt.Errorf("nil account")
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
		return nil, nil, "", fmt.Errorf("no access token available")
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
			Models                     map[string]struct {
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
				IsInternal        bool     `json:"isInternal"`
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

		var geminiOpts, nonGeminiOpts []ModelOption
		seen := make(map[string]bool)

		// The picker catalog is built exclusively from agentModelSorts model ids,
		// in order. Anything not listed there (image/command/commit/tab/tiered
		// bucket ids, models-map-only entries) never reaches the picker.
		for _, sortGroup := range data.AgentModelSorts {
			for _, grp := range sortGroup.Groups {
				for _, mID := range grp.ModelIDs {
					cleanID := strings.TrimPrefix(strings.TrimSpace(mID), "models/")
					if cleanID == "" || seen[cleanID] || excluded[cleanID] || isInternalModelID(cleanID) {
						continue
					}
					mMeta, ok := data.Models[cleanID]
					if !ok {
						mMeta, ok = data.Models[mID]
					}
					if !ok || mMeta.IsInternal {
						continue
					}
					seen[cleanID] = true

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

					opt := ModelOption{
						ID:               cleanID,
						DisplayName:      disp,
						SupportsThinking: supports,
						ThinkingLevels:   levels,
						Recommended:      mMeta.Recommended,
						Provider:         detectModelProvider(cleanID, mMeta.ModelProvider, mMeta.APIProvider),
					}
					if strings.EqualFold(mMeta.ModelProvider, "MODEL_PROVIDER_GOOGLE") {
						geminiOpts = append(geminiOpts, opt)
					} else {
						nonGeminiOpts = append(nonGeminiOpts, opt)
					}
				}
			}
		}

		if len(geminiOpts) > 0 || len(nonGeminiOpts) > 0 {
			cleanDefaultID := strings.TrimPrefix(strings.TrimSpace(data.DefaultAgentModelID), "models/")
			return geminiOpts, nonGeminiOpts, cleanDefaultID, nil
		}
		lastErr = fmt.Errorf("no picker models in fetchAvailableModels response")
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no picker models discovered")
	}
	return nil, nil, "", lastErr
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
