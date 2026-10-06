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
	catalogMu     sync.RWMutex
	cachedCatalog *AvailableModelsCatalog
	cachedAt      time.Time
)

// DefaultBaseGeminiModels returns the base up-to-date Gemini models list.
func DefaultBaseGeminiModels() []ModelOption {
	return []ModelOption{
		{
			ID:               "gemini-3.8-flash",
			DisplayName:      "Gemini 3.8 Flash",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Recommended:      true,
			Provider:         "Google",
		},
		{
			ID:               "gemini-3.8-pro",
			DisplayName:      "Gemini 3.8 Pro",
			SupportsThinking: true,
			ThinkingLevels:   []string{"off", "low", "medium", "high"},
			Provider:         "Google",
		},
		{
			ID:               "gemini-3.5-flash-lite",
			DisplayName:      "Gemini 3.5 Flash Lite",
			Provider:         "Google",
		},
		{
			ID:               "gemini-3.1-pro",
			DisplayName:      "Gemini 3.1 Pro",
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
	catalogMu.RLock()
	if !force && cachedCatalog != nil && time.Since(cachedAt) < 2*time.Minute {
		defer catalogMu.RUnlock()
		return cachedCatalog
	}
	catalogMu.RUnlock()

	catalogMu.Lock()
	defer catalogMu.Unlock()

	// Double check cache
	if !force && cachedCatalog != nil && time.Since(cachedAt) < 2*time.Minute {
		return cachedCatalog
	}

	cat := &AvailableModelsCatalog{
		Success:          true,
		GeminiModels:     DefaultBaseGeminiModels(),
		NonGeminiModels:  DefaultBaseNonGeminiModels(),
		DefaultGemini:    "gemini-3.8-flash",
		DefaultNonGemini: "claude-opus-4-6",
		Timestamp:        time.Now(),
	}

	if acc != nil {
		liveModels, err := fetchLiveAvailableModels(acc)
		if err == nil && len(liveModels) > 0 {
			mergeLiveModels(cat, liveModels)
		}
	}

	cachedCatalog = cat
	cachedAt = time.Now()
	return cat
}

// fetchLiveAvailableModels attempts to query CloudCode's fetchAvailableModels endpoint.
func fetchLiveAvailableModels(acc *keyring.Account) ([]ModelOption, error) {
	if acc == nil {
		return nil, fmt.Errorf("nil account")
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
		return nil, fmt.Errorf("no access token available")
	}

	client := &http.Client{Timeout: 4 * time.Second}
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

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		var data struct {
			DefaultAgentModelID string `json:"defaultAgentModelId"`
			TieredModelIDs      struct {
				FlashLite []string `json:"flashLite"`
				Flash     []string `json:"flash"`
				Pro       []string `json:"pro"`
			} `json:"tieredModelIds"`
			Models map[string]struct {
				DisplayName      string `json:"displayName"`
				SupportsThinking bool   `json:"supportsThinking"`
				Recommended      bool   `json:"recommended"`
			} `json:"models"`
		}

		if err := json.Unmarshal(body, &data); err != nil {
			lastErr = err
			continue
		}

		var options []ModelOption
		seen := make(map[string]bool)

		// 1. Process explicit models map
		for mID, mMeta := range data.Models {
			cleanID := strings.TrimSpace(mID)
			if cleanID == "" || seen[cleanID] {
				continue
			}
			seen[cleanID] = true
			disp := mMeta.DisplayName
			if disp == "" {
				disp = formatDisplayNameFromID(cleanID)
			}
			var levels []string
			if mMeta.SupportsThinking {
				levels = []string{"off", "low", "medium", "high"}
			}
			options = append(options, ModelOption{
				ID:               cleanID,
				DisplayName:      disp,
				SupportsThinking: mMeta.SupportsThinking,
				ThinkingLevels:   levels,
				Recommended:      mMeta.Recommended,
				Provider:         detectProvider(cleanID),
			})
		}

		// 2. Process tiered models
		allTiered := append(append(data.TieredModelIDs.FlashLite, data.TieredModelIDs.Flash...), data.TieredModelIDs.Pro...)
		for _, tID := range allTiered {
			cleanID := strings.TrimSpace(tID)
			if cleanID == "" || seen[cleanID] {
				continue
			}
			seen[cleanID] = true
			options = append(options, ModelOption{
				ID:               cleanID,
				DisplayName:      formatDisplayNameFromID(cleanID),
				SupportsThinking: strings.Contains(cleanID, "flash") || strings.Contains(cleanID, "pro"),
				ThinkingLevels:   []string{"off", "low", "medium", "high"},
				Provider:         detectProvider(cleanID),
			})
		}

		if len(options) > 0 {
			return options, nil
		}
	}

	return nil, lastErr
}

func mergeLiveModels(cat *AvailableModelsCatalog, live []ModelOption) {
	geminiIDs := make(map[string]bool)
	for _, m := range cat.GeminiModels {
		geminiIDs[m.ID] = true
	}
	nonGeminiIDs := make(map[string]bool)
	for _, m := range cat.NonGeminiModels {
		nonGeminiIDs[m.ID] = true
	}

	for _, m := range live {
		lower := strings.ToLower(m.ID + " " + m.DisplayName)
		if strings.Contains(lower, "gemini") {
			if !geminiIDs[m.ID] {
				geminiIDs[m.ID] = true
				cat.GeminiModels = append(cat.GeminiModels, m)
			}
		} else {
			if !nonGeminiIDs[m.ID] {
				nonGeminiIDs[m.ID] = true
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
