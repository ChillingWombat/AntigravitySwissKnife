package custommodels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ModelClassification distinguishes active Antigravity native models from custom/outdated models.
type ModelClassification string

const (
	ClassificationNative ModelClassification = "native"
	ClassificationCustom ModelClassification = "custom"
)

// Price source constants shared between Custom Models and Token Monitor's Token Price page.
const (
	PriceSourceProvider     = "provider"
	PriceSourceThirdParty   = "third_party"
	PriceSourceManual       = "manual"
	PriceSourceUnconfigured = "unconfigured"
)

// ModelPricingRecord represents the unified pricing and classification state for a model.
// InternalID is a monotonically increasing internal identifier never exposed in the UI text.
type ModelPricingRecord struct {
	InternalID           int64               `json:"internal_id"`
	ModelID              string              `json:"model_id"`
	CanonicalID          string              `json:"canonical_id,omitempty"`
	CustomModelID        string              `json:"custom_model_id,omitempty"`
	Name                 string              `json:"name"`
	ModelName            string              `json:"model_name,omitempty"`
	Provider             string              `json:"provider"`
	Classification       ModelClassification `json:"classification"`
	IsOutdatedNative     bool                `json:"is_outdated_native,omitempty"`
	InputPricePerM       *float64            `json:"input_price_per_m"`
	CachedInputPricePerM *float64            `json:"cached_input_price_per_m"`
	OutputPricePerM      *float64            `json:"output_price_per_m"`
	Source               string              `json:"source"`
	UpdatedAt            string              `json:"updated_at"`
}

// DeletedModelRecord tracks models whose price and usage data were deleted by the user.
type DeletedModelRecord struct {
	InternalID    int64  `json:"internal_id"`
	ModelID       string `json:"model_id"`
	CustomModelID string `json:"custom_model_id,omitempty"`
	DeletedAt     string `json:"deleted_at"`
}

// CatalogModelInput represents a model discovered from Antigravity's available models catalog.
type CatalogModelInput struct {
	ID          string
	DisplayName string
	Provider    string
}

// UpdatePricingRequest is the payload for manually updating a model's token prices.
type UpdatePricingRequest struct {
	InternalID           int64    `json:"internal_id,omitempty"`
	ModelID              string   `json:"model_id,omitempty"`
	CanonicalID          string   `json:"canonical_id,omitempty"`
	InputPricePerM       *float64 `json:"input_price_per_m"`
	CachedInputPricePerM *float64 `json:"cached_input_price_per_m"`
	OutputPricePerM      *float64 `json:"output_price_per_m"`
}

// Configurable third-party pricing URLs (overridable in tests).
var (
	OpenRouterModelsURL = "https://openrouter.ai/api/v1/models"
	LiteLLMPricingURL   = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

	thirdPartyCacheMu sync.RWMutex
	thirdPartyCache   map[string]rawPriceEntry
	thirdPartyCacheAt time.Time
)

type rawPriceEntry struct {
	inPerM     float64
	cachedPerM float64
	outPerM    float64
}

func floatPtr(v float64) *float64 {
	val := v
	return &val
}

func cloneFloatPtr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// ResetThirdPartyPricingCache clears the cached third-party pricing map (used in tests).
func ResetThirdPartyPricingCache() {
	thirdPartyCacheMu.Lock()
	defer thirdPartyCacheMu.Unlock()
	thirdPartyCache = nil
	thirdPartyCacheAt = time.Time{}
}

// NormalizePricingProvider normalizes provider labels for display and styling.
func NormalizePricingProvider(provider, modelID string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	m := strings.ToLower(strings.TrimSpace(modelID))
	switch {
	case p == "google" || p == "gemini" || strings.Contains(m, "gemini"):
		return "gemini"
	case p == "anthropic" || strings.Contains(m, "claude"):
		return "anthropic"
	case p == "openai" || strings.Contains(m, "gpt") || strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3"):
		return "openai"
	case p == "deepseek" || strings.Contains(m, "deepseek"):
		return "deepseek"
	case p == "local" || strings.Contains(m, "ollama") || strings.Contains(m, "llama"):
		return "local"
	case p != "" && p != "native":
		return p
	default:
		return "other"
	}
}

// FetchModelPricing resolves token pricing (USD per 1M tokens) using a strict hierarchy:
// 1. Model Provider first (live provider endpoint if configured, then official provider specification).
// 2. 3rd-party pricing catalogs as backup (OpenRouter, LiteLLM).
// 3. If not found in either, returns (nil, nil, nil, "unconfigured") so the UI displays null and requires manual entry.
func FetchModelPricing(ctx context.Context, modelID, provider, baseURL, apiKey string) (inPrice, cachedPrice, outPrice *float64, source string) {
	cleanID := strings.TrimSpace(modelID)
	if cleanID == "" {
		return nil, nil, nil, PriceSourceUnconfigured
	}

	// 1A. Live query to the configured model provider endpoint (if baseURL is provided)
	if strings.TrimSpace(baseURL) != "" {
		if inP, cacheP, outP, ok := fetchFromProviderEndpoint(ctx, cleanID, baseURL, apiKey); ok {
			return inP, cacheP, outP, PriceSourceProvider
		}
	}

	// 1B. Official Model Provider published pricing specifications (Google, Anthropic, OpenAI, DeepSeek)
	if inP, cacheP, outP, ok := lookupOfficialProviderPricing(cleanID, provider); ok {
		return inP, cacheP, outP, PriceSourceProvider
	}

	// 2. 3rd-Party Backup (OpenRouter / LiteLLM)
	if inP, cacheP, outP, ok := lookupThirdPartyBackupPricing(ctx, cleanID); ok {
		return inP, cacheP, outP, PriceSourceThirdParty
	}

	// 3. Does not exist -> null (must be manually entered)
	return nil, nil, nil, PriceSourceUnconfigured
}

// fetchFromProviderEndpoint queries a custom/provider endpoint's /v1/models or /api/pricing for embedded pricing metadata.
func fetchFromProviderEndpoint(ctx context.Context, modelID, rawBaseURL, apiKey string) (*float64, *float64, *float64, bool) {
	cleanBase := strings.TrimRight(strings.TrimSpace(rawBaseURL), "/")
	if cleanBase == "" {
		return nil, nil, nil, false
	}

	// Avoid hitting third-party aggregators as "provider" in Tier 1
	lowerBase := strings.ToLower(cleanBase)
	if strings.Contains(lowerBase, "openrouter.ai") || strings.Contains(lowerBase, "raw.githubusercontent.com") {
		return nil, nil, nil, false
	}

	var endpoints []string
	if strings.HasSuffix(cleanBase, "/models") {
		endpoints = append(endpoints, cleanBase)
	} else if strings.HasSuffix(cleanBase, "/chat/completions") {
		endpoints = append(endpoints, strings.TrimSuffix(cleanBase, "/chat/completions")+"/models")
	} else if strings.HasSuffix(cleanBase, "/v1") {
		endpoints = append(endpoints, cleanBase+"/models")
	} else {
		endpoints = append(endpoints, cleanBase+"/v1/models", cleanBase+"/models")
	}

	client := &http.Client{Timeout: 3 * time.Second}
	targetLower := strings.ToLower(modelID)
	if idx := strings.LastIndex(targetLower, "/"); idx != -1 {
		targetLower = targetLower[idx+1:]
	}

	for _, ep := range endpoints {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, ep, nil)
		if err != nil {
			cancel()
			continue
		}
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("x-api-key", apiKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if resp.StatusCode != http.StatusOK || len(body) == 0 {
			continue
		}

		var payload struct {
			Data []struct {
				ID               string          `json:"id"`
				InputPricePerM   *float64        `json:"input_price_per_m"`
				CachedPricePerM  *float64        `json:"cached_input_price_per_m"`
				OutputPricePerM  *float64        `json:"output_price_per_m"`
				InputCostPerTok  *float64        `json:"input_cost_per_token"`
				OutputCostPerTok *float64        `json:"output_cost_per_token"`
				CacheCostPerTok  *float64        `json:"cache_read_input_token_cost"`
				Pricing          json.RawMessage `json:"pricing"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &payload) != nil || len(payload.Data) == 0 {
			continue
		}

		for _, item := range payload.Data {
			itemID := strings.ToLower(strings.TrimSpace(item.ID))
			shortItemID := itemID
			if idx := strings.LastIndex(shortItemID, "/"); idx != -1 {
				shortItemID = shortItemID[idx+1:]
			}
			if itemID != strings.ToLower(modelID) && shortItemID != targetLower {
				continue
			}

			if item.InputPricePerM != nil && item.OutputPricePerM != nil {
				cached := item.CachedPricePerM
				if cached == nil {
					cached = floatPtr(*item.InputPricePerM * 0.25)
				}
				return cloneFloatPtr(item.InputPricePerM), cloneFloatPtr(cached), cloneFloatPtr(item.OutputPricePerM), true
			}
			if item.InputCostPerTok != nil && item.OutputCostPerTok != nil {
				inM := *item.InputCostPerTok * 1_000_000
				outM := *item.OutputCostPerTok * 1_000_000
				cacheM := inM * 0.25
				if item.CacheCostPerTok != nil {
					cacheM = *item.CacheCostPerTok * 1_000_000
				}
				return floatPtr(inM), floatPtr(cacheM), floatPtr(outM), true
			}
			if len(item.Pricing) > 0 {
				if inM, cacheM, outM, ok := parsePricingObject(item.Pricing); ok {
					return floatPtr(inM), floatPtr(cacheM), floatPtr(outM), true
				}
			}
		}
	}

	return nil, nil, nil, false
}

func parsePricingObject(raw json.RawMessage) (float64, float64, float64, bool) {
	var p struct {
		Prompt          json.RawMessage `json:"prompt"`
		Completion      json.RawMessage `json:"completion"`
		InputCacheRead  json.RawMessage `json:"input_cache_read"`
		InputPricePerM  *float64        `json:"input_price_per_m"`
		CachedPricePerM *float64        `json:"cached_input_price_per_m"`
		OutputPricePerM *float64        `json:"output_price_per_m"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return 0, 0, 0, false
	}
	if p.InputPricePerM != nil && p.OutputPricePerM != nil {
		c := *p.InputPricePerM * 0.25
		if p.CachedPricePerM != nil {
			c = *p.CachedPricePerM
		}
		return *p.InputPricePerM, c, *p.OutputPricePerM, true
	}
	promptTok := parseRawFloat(p.Prompt)
	compTok := parseRawFloat(p.Completion)
	cacheTok := parseRawFloat(p.InputCacheRead)
	if promptTok <= 0 && compTok <= 0 {
		return 0, 0, 0, false
	}
	// OpenRouter style per-token values are < 0.01; convert to per-1M tokens.
	if promptTok < 0.05 && compTok < 0.05 {
		inM := promptTok * 1_000_000
		outM := compTok * 1_000_000
		cacheM := cacheTok * 1_000_000
		if cacheM <= 0 {
			cacheM = inM * 0.25
		}
		return inM, cacheM, outM, true
	}
	if cacheTok <= 0 {
		cacheTok = promptTok * 0.25
	}
	return promptTok, cacheTok, compTok, true
}

// lookupOfficialProviderPricing checks official first-party provider rate tables
// (Google Gemini, Anthropic, OpenAI, DeepSeek).
func lookupOfficialProviderPricing(modelID, provider string) (*float64, *float64, *float64, bool) {
	id := strings.ToLower(strings.TrimSpace(modelID))
	id = strings.TrimPrefix(id, "models/")

	// Official provider pricing table (USD per 1M tokens)
	type rate struct {
		in     float64
		cached float64
		out    float64
	}
	exactRates := map[string]rate{
		// Google Gemini 3.8 / 3.7 / 3.6 / 3.5 Flash series
		"gemini-3.8-flash-high":   {0.15, 0.0375, 0.60},
		"gemini-3.8-flash-medium": {0.15, 0.0375, 0.60},
		"gemini-3.8-flash-low":    {0.15, 0.0375, 0.60},
		"gemini-3.8-flash":        {0.15, 0.0375, 0.60},
		"gemini-3.8-flash-tiered": {0.15, 0.0375, 0.60},
		"gemini-3.7-flash-high":   {0.15, 0.0375, 0.60},
		"gemini-3.7-flash-medium": {0.15, 0.0375, 0.60},
		"gemini-3.7-flash-low":    {0.15, 0.0375, 0.60},
		"gemini-3.7-flash":        {0.15, 0.0375, 0.60},
		"gemini-3.6-flash-high":   {0.15, 0.0375, 0.60},
		"gemini-3.6-flash-medium": {0.15, 0.0375, 0.60},
		"gemini-3.6-flash-low":    {0.15, 0.0375, 0.60},
		"gemini-3.6-flash":        {0.15, 0.0375, 0.60},
		"gemini-3.5-flash":        {0.10, 0.025, 0.40},
		"gemini-3.5-flash-lite":   {0.075, 0.01875, 0.30},
		"gemini-3-flash":          {0.15, 0.0375, 0.60},
		"gemini-3-flash-high":     {0.15, 0.0375, 0.60},
		"gemini-3-flash-medium":   {0.15, 0.0375, 0.60},
		"gemini-3-flash-low":      {0.15, 0.0375, 0.60},
		"gemini-3-flash-agent":    {0.15, 0.0375, 0.60},
		// Google Gemini 3.1 Pro / 2.5 Pro / 2.5 Flash
		"gemini-pro-agent":   {1.25, 0.3125, 5.00},
		"gemini-3.1-pro-high": {1.25, 0.3125, 5.00},
		"gemini-3.1-pro-low":  {1.25, 0.3125, 5.00},
		"gemini-3.1-pro":      {1.25, 0.3125, 5.00},
		"gemini-2.5-pro":      {1.25, 0.3125, 5.00},
		"gemini-2.5-flash":    {0.075, 0.01875, 0.30},
		"gemini-2.0-flash":    {0.10, 0.025, 0.40},
		// Anthropic Claude 4.6 / 3.7 / 3.5
		"claude-sonnet-4-6":          {3.00, 0.30, 15.00},
		"claude-opus-4-6-thinking":   {15.00, 1.50, 75.00},
		"claude-opus-4-6":            {15.00, 1.50, 75.00},
		"claude-3-7-sonnet":          {3.00, 0.30, 15.00},
		"claude-3-7-sonnet-20250219": {3.00, 0.30, 15.00},
		"claude-3-5-sonnet":          {3.00, 0.30, 15.00},
		"claude-3-5-sonnet-20241022": {3.00, 0.30, 15.00},
		"claude-3-5-haiku":           {0.80, 0.08, 4.00},
		"claude-3-5-haiku-20241022":  {0.80, 0.08, 4.00},
		"claude-3-opus-20240229":     {15.00, 1.50, 75.00},
		// OpenAI
		"gpt-oss-120b-medium": {0.50, 0.125, 1.50},
		"gpt-oss-120b":        {0.50, 0.125, 1.50},
		"gpt-4o":              {2.50, 1.25, 10.00},
		"gpt-4o-mini":         {0.15, 0.075, 0.60},
		"o1":                  {15.00, 7.50, 60.00},
		"o3-mini":             {1.10, 0.55, 4.40},
		// DeepSeek
		"deepseek-chat":     {0.27, 0.07, 1.10},
		"deepseek-v3":       {0.27, 0.07, 1.10},
		"deepseek-reasoner": {0.55, 0.14, 2.19},
		"deepseek-r1":       {0.55, 0.14, 2.19},
	}

	if r, ok := exactRates[id]; ok {
		return floatPtr(r.in), floatPtr(r.cached), floatPtr(r.out), true
	}
	// Strip vendor prefix if present ONLY when provider is an official vendor
	if idx := strings.LastIndex(id, "/"); idx != -1 {
		prefix := id[:idx]
		shortID := id[idx+1:]
		if prefix == "google" || prefix == "gemini" || prefix == "anthropic" || prefix == "openai" || prefix == "deepseek" {
			if r, ok := exactRates[shortID]; ok {
				return floatPtr(r.in), floatPtr(r.cached), floatPtr(r.out), true
			}
		}
	}

	return nil, nil, nil, false
}

// lookupThirdPartyBackupPricing queries OpenRouter and LiteLLM pricing catalogs as backup.
func lookupThirdPartyBackupPricing(ctx context.Context, modelID string) (*float64, *float64, *float64, bool) {
	thirdPartyCacheMu.RLock()
	cached := thirdPartyCache
	fresh := cached != nil && time.Since(thirdPartyCacheAt) < 10*time.Minute
	thirdPartyCacheMu.RUnlock()

	if !fresh {
		fetched := fetchThirdPartyCatalogs(ctx)
		if len(fetched) > 0 {
			thirdPartyCacheMu.Lock()
			thirdPartyCache = fetched
			thirdPartyCacheAt = time.Now()
			cached = fetched
			thirdPartyCacheMu.Unlock()
		}
	}

	if len(cached) == 0 {
		return nil, nil, nil, false
	}

	target := strings.ToLower(strings.TrimSpace(modelID))
	target = strings.TrimPrefix(target, "models/")
	if e, ok := cached[target]; ok {
		return floatPtr(e.inPerM), floatPtr(e.cachedPerM), floatPtr(e.outPerM), true
	}
	if idx := strings.LastIndex(target, "/"); idx != -1 {
		short := target[idx+1:]
		if e, ok := cached[short]; ok {
			return floatPtr(e.inPerM), floatPtr(e.cachedPerM), floatPtr(e.outPerM), true
		}
	}
	return nil, nil, nil, false
}

func fetchThirdPartyCatalogs(ctx context.Context) map[string]rawPriceEntry {
	res := make(map[string]rawPriceEntry)
	client := &http.Client{Timeout: 3 * time.Second}

	// 1. OpenRouter backup catalog
	if OpenRouterModelsURL != "" {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, OpenRouterModelsURL, nil); err == nil {
			if resp, err := client.Do(req); err == nil {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var orResp struct {
						Data []struct {
							ID      string          `json:"id"`
							Pricing json.RawMessage `json:"pricing"`
						} `json:"data"`
					}
					if json.Unmarshal(body, &orResp) == nil {
						for _, m := range orResp.Data {
							if inM, cacheM, outM, ok := parsePricingObject(m.Pricing); ok {
								k := strings.ToLower(strings.TrimSpace(m.ID))
								entry := rawPriceEntry{inPerM: inM, cachedPerM: cacheM, outPerM: outM}
								res[k] = entry
								if idx := strings.LastIndex(k, "/"); idx != -1 {
									short := k[idx+1:]
									if _, exists := res[short]; !exists {
										res[short] = entry
									}
								}
							}
						}
					}
				}
			}
		}
		cancel()
	}

	// 2. LiteLLM backup catalog
	if LiteLLMPricingURL != "" {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, LiteLLMPricingURL, nil); err == nil {
			if resp, err := client.Do(req); err == nil {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var rawMap map[string]struct {
						InputCostPerToken  *float64 `json:"input_cost_per_token"`
						OutputCostPerToken *float64 `json:"output_cost_per_token"`
						CacheReadCost      *float64 `json:"cache_read_input_token_cost"`
					}
					if json.Unmarshal(body, &rawMap) == nil {
						for k, v := range rawMap {
							if v.InputCostPerToken == nil || v.OutputCostPerToken == nil {
								continue
							}
							inM := *v.InputCostPerToken * 1_000_000
							outM := *v.OutputCostPerToken * 1_000_000
							if inM <= 0 && outM <= 0 {
								continue
							}
							cacheM := inM * 0.25
							if v.CacheReadCost != nil && *v.CacheReadCost > 0 {
								cacheM = *v.CacheReadCost * 1_000_000
							}
							key := strings.ToLower(strings.TrimSpace(k))
							if _, exists := res[key]; !exists {
								res[key] = rawPriceEntry{inPerM: inM, cachedPerM: cacheM, outPerM: outM}
							}
						}
					}
				}
			}
		}
		cancel()
	}

	return res
}

// allocateInternalIDLocked returns a strictly increasing unique internal ID.
func (s *Store) allocateInternalIDLocked() int64 {
	maxID := s.config.NextInternalID
	for _, m := range s.config.Models {
		if m.InternalID > maxID {
			maxID = m.InternalID
		}
	}
	for _, p := range s.config.PricingRecords {
		if p.InternalID > maxID {
			maxID = p.InternalID
		}
	}
	for _, d := range s.config.DeletedModels {
		if d.InternalID > maxID {
			maxID = d.InternalID
		}
	}
	next := maxID + 1
	s.config.NextInternalID = next
	return next
}

// NormalizeCanonicalNativeModel strips thinking-level suffixes (e.g., -high, -medium, -low, -thinking)
// and display-name annotations (e.g., "(High)", "(Medium)", "(Low)", "(Thinking)") so that a native
// model with multiple thinking levels is treated as a single canonical model in Token Price and Monitoring.
func NormalizeCanonicalNativeModel(modelID, displayName string) (canonicalID, canonicalName string) {
	id := strings.TrimSpace(modelID)
	id = strings.TrimPrefix(id, "models/")
	lowID := strings.ToLower(id)

	name := strings.TrimSpace(displayName)
	for _, suffix := range []string{
		" (High)", " (Medium)", " (Low)", " (Off)", " (Thinking)",
		" (high)", " (medium)", " (low)", " (off)", " (thinking)",
		" [High]", " [Medium]", " [Low]", " [Thinking]",
	} {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSpace(strings.TrimSuffix(name, suffix))
		}
	}

	switch lowID {
	case "gemini-pro-agent", "gemini-3.1-pro-high", "gemini-3.1-pro-medium", "gemini-3.1-pro-low", "gemini-3.1-pro-off", "gemini-3.1-pro":
		return "gemini-3.1-pro", "Gemini 3.1 Pro"
	case "gemini-3-flash-agent", "gemini-3-flash-high", "gemini-3-flash-medium", "gemini-3-flash-low", "gemini-3-flash":
		return "gemini-3-flash", "Gemini 3 Flash"
	}

	for _, suffix := range []string{"-thinking", "-tiered", "-high", "-medium", "-low", "-off"} {
		if strings.HasSuffix(lowID, suffix) {
			id = id[:len(id)-len(suffix)]
			lowID = lowID[:len(lowID)-len(suffix)]
			break
		}
	}

	if name == "" {
		switch lowID {
		case "gemini-3.8-flash":
			name = "Gemini 3.8 Flash"
		case "gemini-3.7-flash":
			name = "Gemini 3.7 Flash"
		case "gemini-3.6-flash":
			name = "Gemini 3.6 Flash"
		case "gemini-3.1-pro":
			name = "Gemini 3.1 Pro"
		case "claude-opus-4-6":
			name = "Claude Opus 4.6"
		case "claude-sonnet-4-6":
			name = "Claude Sonnet 4.6"
		case "gpt-oss-120b":
			name = "GPT-OSS 120B"
		default:
			name = id
		}
	}

	return id, name
}

// SyncPricingWithCatalog synchronizes the Token Monitor pricing records with:
// 1. Models currently available on Antigravity (classified as "native"), canonicalized across thinking levels.
// 2. Previously auto-fetched native models no longer available on Antigravity (kept, reclassified as "custom", IsOutdatedNative=true).
// 3. User-configured Custom Models from s.config.Models (classified as "custom", sharing the same InternalID and price data).
func (s *Store) SyncPricingWithCatalog(ctx context.Context, nativeModels []CatalogModelInput, forceRefreshPrices bool) []ModelPricingRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04")

	// Canonicalize incoming native catalog models across thinking levels while preserving catalog order
	canonicalNativeOrder := make([]CatalogModelInput, 0, len(nativeModels))
	activeNativeMap := make(map[string]CatalogModelInput, len(nativeModels))
	for _, nm := range nativeModels {
		canID, canName := NormalizeCanonicalNativeModel(nm.ID, nm.DisplayName)
		if canID == "" {
			continue
		}
		key := strings.ToLower(canID)
		if _, exists := activeNativeMap[key]; !exists {
			canonInput := CatalogModelInput{
				ID:          canID,
				DisplayName: canName,
				Provider:    nm.Provider,
			}
			activeNativeMap[key] = canonInput
			canonicalNativeOrder = append(canonicalNativeOrder, canonInput)
		}
	}

	// Ensure existing records have a valid InternalID and deduplicate any native records that share a canonical ID
	dedupedRecords := make([]ModelPricingRecord, 0, len(s.config.PricingRecords))
	nativeIdxByCanon := make(map[string]int)
	for i := range s.config.PricingRecords {
		rec := s.config.PricingRecords[i]
		if rec.InternalID <= 0 {
			rec.InternalID = s.allocateInternalIDLocked()
		}
		if rec.CustomModelID == "" {
			canID, canName := NormalizeCanonicalNativeModel(rec.ModelID, rec.Name)
			rec.ModelID = canID
			if canName != "" {
				rec.Name = canName
			}
			key := strings.ToLower(canID)
			if existingIdx, exists := nativeIdxByCanon[key]; exists {
				// Keep manual override if this duplicate had one
				if rec.Source == PriceSourceManual && dedupedRecords[existingIdx].Source != PriceSourceManual {
					dedupedRecords[existingIdx].InputPricePerM = cloneFloatPtr(rec.InputPricePerM)
					dedupedRecords[existingIdx].CachedInputPricePerM = cloneFloatPtr(rec.CachedInputPricePerM)
					dedupedRecords[existingIdx].OutputPricePerM = cloneFloatPtr(rec.OutputPricePerM)
					dedupedRecords[existingIdx].Source = rec.Source
					dedupedRecords[existingIdx].UpdatedAt = rec.UpdatedAt
				}
				continue
			}
			nativeIdxByCanon[key] = len(dedupedRecords)
		}
		dedupedRecords = append(dedupedRecords, rec)
	}
	s.config.PricingRecords = dedupedRecords

	// 1. Update existing native/outdated-native records (CustomModelID == "")
	seenNativeIDs := make(map[string]bool)
	for i := range s.config.PricingRecords {
		rec := &s.config.PricingRecords[i]
		if rec.CustomModelID != "" {
			continue
		}
		key := strings.ToLower(rec.ModelID)
		if nm, isStillAvailable := activeNativeMap[key]; isStillAvailable {
			seenNativeIDs[key] = true
			s.clearDeletedModelLocked(rec.ModelID, "")
			rec.Classification = ClassificationNative
			rec.IsOutdatedNative = false
			if nm.DisplayName != "" {
				rec.Name = nm.DisplayName
			}
			rec.Provider = NormalizePricingProvider(nm.Provider, rec.ModelID)
			if rec.Source != PriceSourceManual && (forceRefreshPrices || (rec.InputPricePerM == nil && rec.OutputPricePerM == nil)) {
				inP, cacheP, outP, src := FetchModelPricing(ctx, rec.ModelID, rec.Provider, "", "")
				rec.InputPricePerM = inP
				rec.CachedInputPricePerM = cacheP
				rec.OutputPricePerM = outP
				rec.Source = src
				rec.UpdatedAt = now
			}
		} else {
			// Model was previously auto-fetched from Antigravity, but is no longer available on Antigravity.
			// Do NOT delete automatically; reclassify as "custom" in the Token Monitor (not added to Custom Models page).
			rec.Classification = ClassificationCustom
			rec.IsOutdatedNative = true
		}
	}

	// 2. Add newly discovered Antigravity native models in canonical catalog order
	for _, nm := range canonicalNativeOrder {
		cleanID := strings.TrimSpace(nm.ID)
		key := strings.ToLower(cleanID)
		if cleanID == "" || seenNativeIDs[key] {
			continue
		}
		seenNativeIDs[key] = true
		s.clearDeletedModelLocked(cleanID, "")
		prov := NormalizePricingProvider(nm.Provider, cleanID)
		inP, cacheP, outP, src := FetchModelPricing(ctx, cleanID, prov, "", "")
		disp := strings.TrimSpace(nm.DisplayName)
		if disp == "" {
			disp = cleanID
		}
		s.config.PricingRecords = append(s.config.PricingRecords, ModelPricingRecord{
			InternalID:           s.allocateInternalIDLocked(),
			ModelID:              cleanID,
			CanonicalID:          cleanID,
			Name:                 disp,
			ModelName:            disp,
			Provider:             prov,
			Classification:       ClassificationNative,
			IsOutdatedNative:     false,
			InputPricePerM:       inP,
			CachedInputPricePerM: cacheP,
			OutputPricePerM:      outP,
			Source:               src,
			UpdatedAt:            now,
		})
	}

	// 3. Synchronize user-configured Custom Models from s.config.Models
	for i := range s.config.Models {
		cm := &s.config.Models[i]
		if cm.InternalID <= 0 {
			cm.InternalID = s.allocateInternalIDLocked()
		}
		if cm.PriceSource != PriceSourceManual && (forceRefreshPrices || (cm.InputPricePerM == nil && cm.OutputPricePerM == nil && cm.PriceSource == "")) {
			inP, cacheP, outP, src := FetchModelPricing(ctx, cm.Name, string(cm.ProviderType), cm.BaseURL, cm.APIKey)
			cm.InputPricePerM = inP
			cm.CachedInputPricePerM = cacheP
			cm.OutputPricePerM = outP
			cm.PriceSource = src
			cm.PriceUpdatedAt = now
		}
		s.syncCustomModelToPricingLocked(*cm)
	}

	_ = s.saveLocked()
	return s.listPricingRecordsLocked()
}

// syncCustomModelToPricingLocked ensures a CustomModel's price and status are mirrored in s.config.PricingRecords.
func (s *Store) syncCustomModelToPricingLocked(cm CustomModel) {
	now := time.Now().Format("2006-01-02 15:04")
	updatedAt := cm.PriceUpdatedAt
	if updatedAt == "" {
		updatedAt = now
	}
	src := cm.PriceSource
	if src == "" {
		if cm.InputPricePerM != nil || cm.OutputPricePerM != nil {
			src = PriceSourceManual
		} else {
			src = PriceSourceUnconfigured
		}
	}
	prov := NormalizePricingProvider(string(cm.ProviderType), cm.Name)
	disp := strings.TrimSpace(cm.DisplayName)
	if disp == "" {
		disp = cm.Name
	}

	for i := range s.config.PricingRecords {
		rec := &s.config.PricingRecords[i]
		if (cm.InternalID > 0 && rec.InternalID == cm.InternalID) || (rec.CustomModelID != "" && rec.CustomModelID == cm.ID) {
			rec.InternalID = cm.InternalID
			rec.ModelID = cm.Name
			rec.CanonicalID = cm.Name
			rec.CustomModelID = cm.ID
			rec.Name = disp
			rec.ModelName = disp
			rec.Provider = prov
			rec.Classification = ClassificationCustom
			rec.IsOutdatedNative = false
			rec.InputPricePerM = cloneFloatPtr(cm.InputPricePerM)
			rec.CachedInputPricePerM = cloneFloatPtr(cm.CachedInputPricePerM)
			rec.OutputPricePerM = cloneFloatPtr(cm.OutputPricePerM)
			rec.Source = src
			rec.UpdatedAt = updatedAt
			return
		}
	}

	s.config.PricingRecords = append(s.config.PricingRecords, ModelPricingRecord{
		InternalID:           cm.InternalID,
		ModelID:              cm.Name,
		CanonicalID:          cm.Name,
		CustomModelID:        cm.ID,
		Name:                 disp,
		ModelName:            disp,
		Provider:             prov,
		Classification:       ClassificationCustom,
		IsOutdatedNative:     false,
		InputPricePerM:       cloneFloatPtr(cm.InputPricePerM),
		CachedInputPricePerM: cloneFloatPtr(cm.CachedInputPricePerM),
		OutputPricePerM:      cloneFloatPtr(cm.OutputPricePerM),
		Source:               src,
		UpdatedAt:            updatedAt,
	})
}

func (s *Store) listPricingRecordsLocked() []ModelPricingRecord {
	out := make([]ModelPricingRecord, len(s.config.PricingRecords))
	for i, r := range s.config.PricingRecords {
		rCopy := r
		rCopy.CanonicalID = r.ModelID
		rCopy.ModelName = r.Name
		rCopy.InputPricePerM = cloneFloatPtr(r.InputPricePerM)
		rCopy.CachedInputPricePerM = cloneFloatPtr(r.CachedInputPricePerM)
		rCopy.OutputPricePerM = cloneFloatPtr(r.OutputPricePerM)
		out[i] = rCopy
	}
	return out
}

// ListPricingRecords returns all current pricing records.
func (s *Store) ListPricingRecords() []ModelPricingRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listPricingRecordsLocked()
}

// UpdateModelPricing updates only the price fields of a model in the Token Monitor's Token Price page,
// and synchronizes the updated price & manual status back to the CustomModel if it is a user custom model.
func (s *Store) UpdateModelPricing(req UpdatePricingRequest) (*ModelPricingRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04")
	reqModelID := strings.TrimSpace(req.ModelID)
	if reqModelID == "" {
		reqModelID = strings.TrimSpace(req.CanonicalID)
	}
	canonID, _ := NormalizeCanonicalNativeModel(reqModelID, "")
	targetIdx := -1
	for i, r := range s.config.PricingRecords {
		if req.InternalID > 0 && r.InternalID == req.InternalID {
			targetIdx = i
			break
		}
		if req.InternalID <= 0 && (strings.EqualFold(r.ModelID, reqModelID) || (r.CustomModelID == "" && canonID != "" && strings.EqualFold(r.ModelID, canonID))) {
			targetIdx = i
			break
		}
	}

	if targetIdx == -1 {
		return nil, fmt.Errorf("pricing record not found for model %q", reqModelID)
	}

	rec := &s.config.PricingRecords[targetIdx]
	rec.CanonicalID = rec.ModelID
	rec.ModelName = rec.Name
	rec.InputPricePerM = cloneFloatPtr(req.InputPricePerM)
	rec.CachedInputPricePerM = cloneFloatPtr(req.CachedInputPricePerM)
	rec.OutputPricePerM = cloneFloatPtr(req.OutputPricePerM)
	if rec.InputPricePerM == nil && rec.CachedInputPricePerM == nil && rec.OutputPricePerM == nil {
		rec.Source = PriceSourceUnconfigured
	} else {
		rec.Source = PriceSourceManual
	}
	rec.UpdatedAt = now

	// Sync back to CustomModel if this pricing record belongs to a CustomModel
	for i := range s.config.Models {
		cm := &s.config.Models[i]
		if (rec.InternalID > 0 && cm.InternalID == rec.InternalID) || (rec.CustomModelID != "" && cm.ID == rec.CustomModelID) || strings.EqualFold(cm.Name, rec.ModelID) {
			cm.InputPricePerM = cloneFloatPtr(rec.InputPricePerM)
			cm.CachedInputPricePerM = cloneFloatPtr(rec.CachedInputPricePerM)
			cm.OutputPricePerM = cloneFloatPtr(rec.OutputPricePerM)
			cm.PriceSource = rec.Source
			cm.PriceUpdatedAt = now
		}
	}

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	res := *rec
	res.CanonicalID = rec.ModelID
	res.ModelName = rec.Name
	res.InputPricePerM = cloneFloatPtr(rec.InputPricePerM)
	res.CachedInputPricePerM = cloneFloatPtr(rec.CachedInputPricePerM)
	res.OutputPricePerM = cloneFloatPtr(rec.OutputPricePerM)
	return &res, nil
}

// DeletePricingModel deletes a custom-classified model (either an outdated native model or a user custom model)
// from the Token Monitor, removing its price data, custom model configuration (if any), and recording its
// deletion so usage info is also purged from the Token Monitor.
func (s *Store) DeletePricingModel(internalID int64, modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanModelID := strings.TrimSpace(modelID)
	canonID, _ := NormalizeCanonicalNativeModel(cleanModelID, "")
	targetIdx := -1
	for i, r := range s.config.PricingRecords {
		if internalID > 0 && r.InternalID == internalID {
			targetIdx = i
			break
		}
		if internalID <= 0 && cleanModelID != "" && (strings.EqualFold(r.ModelID, cleanModelID) || strings.EqualFold(r.CustomModelID, cleanModelID) || (r.CustomModelID == "" && canonID != "" && strings.EqualFold(r.ModelID, canonID))) {
			targetIdx = i
			break
		}
	}

	if targetIdx == -1 {
		// Check if it matches a CustomModel directly
		for _, cm := range s.config.Models {
			if (internalID > 0 && cm.InternalID == internalID) || (cleanModelID != "" && (strings.EqualFold(cm.ID, cleanModelID) || strings.EqualFold(cm.Name, cleanModelID))) {
				s.mu.Unlock()
				err := s.DeleteModel(cm.ID)
				s.mu.Lock()
				return err
			}
		}
		return fmt.Errorf("model not found in pricing registry")
	}

	rec := s.config.PricingRecords[targetIdx]
	if rec.Classification == ClassificationNative && !rec.IsOutdatedNative && rec.CustomModelID == "" {
		return fmt.Errorf("active native Antigravity models cannot be deleted")
	}

	// Remove from PricingRecords
	s.config.PricingRecords = append(s.config.PricingRecords[:targetIdx], s.config.PricingRecords[targetIdx+1:]...)

	// If backed by a user CustomModel, remove from s.config.Models and ProjectBinds
	if rec.CustomModelID != "" || rec.InternalID > 0 {
		filtered := make([]CustomModel, 0, len(s.config.Models))
		for _, cm := range s.config.Models {
			if (rec.CustomModelID != "" && cm.ID == rec.CustomModelID) || (rec.InternalID > 0 && cm.InternalID == rec.InternalID) {
				for p, boundID := range s.config.ProjectBinds {
					if boundID == cm.ID {
						delete(s.config.ProjectBinds, p)
					}
				}
				continue
			}
			filtered = append(filtered, cm)
		}
		s.config.Models = filtered
	}

	// Record deletion so Token Monitor usage info is also deleted
	s.recordDeletedModelLocked(rec.InternalID, rec.ModelID, rec.CustomModelID)
	return s.saveLocked()
}

func (s *Store) recordDeletedModelLocked(internalID int64, modelID, customModelID string) {
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range s.config.DeletedModels {
		d := &s.config.DeletedModels[i]
		if (internalID > 0 && d.InternalID == internalID) || (modelID != "" && strings.EqualFold(d.ModelID, modelID)) {
			d.InternalID = internalID
			d.ModelID = modelID
			d.CustomModelID = customModelID
			d.DeletedAt = now
			return
		}
	}
	s.config.DeletedModels = append(s.config.DeletedModels, DeletedModelRecord{
		InternalID:    internalID,
		ModelID:       modelID,
		CustomModelID: customModelID,
		DeletedAt:     now,
	})
}

func (s *Store) clearDeletedModelLocked(modelID, customModelID string) {
	if len(s.config.DeletedModels) == 0 {
		return
	}
	canonID, _ := NormalizeCanonicalNativeModel(modelID, "")
	filtered := make([]DeletedModelRecord, 0, len(s.config.DeletedModels))
	for _, d := range s.config.DeletedModels {
		if (modelID != "" && (strings.EqualFold(d.ModelID, modelID) || (canonID != "" && strings.EqualFold(d.ModelID, canonID)))) || (customModelID != "" && strings.EqualFold(d.CustomModelID, customModelID)) {
			continue
		}
		filtered = append(filtered, d)
	}
	s.config.DeletedModels = filtered
}

// IsModelDeleted returns true if the given model (by internal ID, model ID, or custom model ID) has been deleted.
func (s *Store) IsModelDeleted(internalID int64, modelID, customModelID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	canonID, _ := NormalizeCanonicalNativeModel(modelID, "")
	for _, d := range s.config.DeletedModels {
		if internalID > 0 && d.InternalID == internalID {
			return true
		}
		if modelID != "" && (strings.EqualFold(d.ModelID, modelID) || (canonID != "" && strings.EqualFold(d.ModelID, canonID))) {
			return true
		}
		if customModelID != "" && d.CustomModelID != "" && strings.EqualFold(d.CustomModelID, customModelID) {
			return true
		}
	}
	return false
}

// EnsureHistoricalNativeModels registers models discovered in Antigravity conversation history
// that are no longer in the active available catalog as outdated native models (classified as "custom").
func (s *Store) EnsureHistoricalNativeModels(ctx context.Context, models []CatalogModelInput) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04")
	changed := false
	for _, nm := range models {
		canID, canName := NormalizeCanonicalNativeModel(nm.ID, nm.DisplayName)
		if canID == "" {
			continue
		}
		alreadyDeleted := false
		for _, d := range s.config.DeletedModels {
			if strings.EqualFold(d.ModelID, canID) || strings.EqualFold(d.ModelID, nm.ID) {
				alreadyDeleted = true
				break
			}
		}
		if alreadyDeleted {
			continue
		}
		exists := false
		for _, rec := range s.config.PricingRecords {
			if strings.EqualFold(rec.ModelID, canID) {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		prov := NormalizePricingProvider(nm.Provider, canID)
		inP, cacheP, outP, src := FetchModelPricing(ctx, canID, prov, "", "")
		disp := strings.TrimSpace(canName)
		if disp == "" {
			disp = canID
		}
		s.config.PricingRecords = append(s.config.PricingRecords, ModelPricingRecord{
			InternalID:           s.allocateInternalIDLocked(),
			ModelID:              canID,
			CanonicalID:          canID,
			Name:                 disp,
			ModelName:            disp,
			Provider:             prov,
			Classification:       ClassificationCustom,
			IsOutdatedNative:     true,
			InputPricePerM:       inP,
			CachedInputPricePerM: cacheP,
			OutputPricePerM:      outP,
			Source:               src,
			UpdatedAt:            now,
		})
		changed = true
	}
	if changed {
		_ = s.saveLocked()
	}
}

