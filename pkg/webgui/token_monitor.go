package webgui

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
	_ "modernc.org/sqlite"
)

// TokenModelBreakdown represents real aggregated token consumption for a single model.
type TokenModelBreakdown struct {
	InternalID       int64                            `json:"internal_id"`
	ModelID          string                           `json:"model_id"`
	CanonicalID      string                           `json:"canonical_id,omitempty"`
	ModelName        string                           `json:"model_name"`
	Provider         string                           `json:"provider"`
	Classification   custommodels.ModelClassification `json:"classification"`
	IsOutdatedNative bool                             `json:"is_outdated_native,omitempty"`
	InputTokens      int64                            `json:"input_tokens"`
	CachedTokens     int64                            `json:"cached_tokens"`
	OutputTokens     int64                            `json:"output_tokens"`
	TotalTokens      int64                            `json:"total_tokens"`
	CostUSD          float64                          `json:"cost_usd"`
	SavedUSD         float64                          `json:"saved_usd"`
	PriceConfigured  bool                             `json:"price_configured"`
	AvgTPS           float64                          `json:"avg_tps"`
	Requests         int                              `json:"requests"`
	Percentage       float64                          `json:"percentage"`
}

// TokenProjectBreakdown represents real aggregated token consumption for a workspace project.
type TokenProjectBreakdown struct {
	ProjectName   string  `json:"project_name"`
	WorkspacePath string  `json:"workspace_path"`
	TotalTokens   int64   `json:"total_tokens"`
	InputTokens   int64   `json:"input_tokens"`
	CachedTokens  int64   `json:"cached_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	CostUSD       float64 `json:"cost_usd"`
	SavedUSD      float64 `json:"saved_usd"`
	CacheHitRatio float64 `json:"cache_hit_ratio"`
	SessionsCount int     `json:"sessions_count"`
	Requests      int     `json:"requests"`
	LastActive    string  `json:"last_active"`
	PrimaryModel  string  `json:"primary_model"`
}

// TokenTelemetryEvent represents a real LLM invocation step from transcript.jsonl.
type TokenTelemetryEvent struct {
	ID              string                           `json:"id"`
	Timestamp       string                           `json:"timestamp"`
	Project         string                           `json:"project"`
	Model           string                           `json:"model"`
	ModelID         string                           `json:"model_id"`
	CanonicalID     string                           `json:"canonical_id,omitempty"`
	Provider        string                           `json:"provider"`
	Classification  custommodels.ModelClassification `json:"classification"`
	InputTokens     int64                            `json:"input_tokens"`
	CachedTokens    int64                            `json:"cached_tokens"`
	OutputTokens    int64                            `json:"output_tokens"`
	CostUSD         float64                          `json:"cost_usd"`
	PriceConfigured bool                             `json:"price_configured"`
	DurationMs      int64                            `json:"duration_ms"`
	TPS             float64                          `json:"tps"`
	IsSubagent      bool                             `json:"is_subagent"`
	AgentRole       string                           `json:"agent_role,omitempty"`
}

type cachedTranscriptStats struct {
	modTimeNano       int64
	size              int64
	freshInput        int64
	cachedInput       int64
	outputTokens      int64
	requests          int
	totalDurationMs   int64
	timedOutputTokens int64
	lastActive        time.Time
	isSubagent        bool
	agentRole         string
	// steps holds every token-bearing step with its own timestamp so the
	// summary handler can aggregate over an arbitrary time window (range
	// filter). All-time totals above are kept for cheap validity checks.
	steps []rawStepEvent
}

type rawStepEvent struct {
	stepIndex   int
	createdAt   time.Time
	freshInput  int64
	cachedInput int64
	output      int64
	durationMs  int64
}

// stepWithContext is a token-bearing step enriched with the conversation,
// model and project attribution the summary handler resolved for it.
type stepWithContext struct {
	rawStepEvent
	convID          string
	project         string
	modelName       string
	modelID         string
	provider        string
	classification  custommodels.ModelClassification
	costUSD         float64
	priceConfigured bool
	isSubagent      bool
	agentRole       string
}

type cachedConvModel struct {
	modTimeNano int64
	size        int64
	modelID     string
	displayName string
	provider    string
}

var (
	tokenCacheMu       sync.RWMutex
	transcriptCacheMap = make(map[string]cachedTranscriptStats)
	convModelCacheMap  = make(map[string]cachedConvModel)
)

// knownNativePlaceholderMap maps Antigravity's internal protobuf model_enum placeholders
// in ~/.gemini/antigravity/conversations/*.db to their canonical model IDs and display names
// (unified across thinking levels for token monitoring and pricing).
var knownNativePlaceholderMap = []struct {
	marker      string
	modelID     string
	displayName string
	provider    string
}{
	{marker: "MODEL_PLACEHOLDER_M318", modelID: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", provider: "gemini"},
	{marker: "MODEL_PLACEHOLDER_M322", modelID: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", provider: "gemini"},
	{marker: "MODEL_PLACEHOLDER_M319", modelID: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", provider: "gemini"},
	{marker: "MODEL_PLACEHOLDER_M320", modelID: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", provider: "gemini"},
	{marker: "MODEL_PLACEHOLDER_M16", modelID: "gemini-3.1-pro", displayName: "Gemini 3.1 Pro", provider: "gemini"},
	{marker: "MODEL_PLACEHOLDER_M37", modelID: "gemini-3.1-pro", displayName: "Gemini 3.1 Pro", provider: "gemini"},
	{marker: "gpt-oss-120b-medium", modelID: "gpt-oss-120b", displayName: "GPT-OSS 120B", provider: "openai"},
	{marker: "gpt-oss-120b", modelID: "gpt-oss-120b", displayName: "GPT-OSS 120B", provider: "openai"},
	{marker: "gemini-3.8-flash-high", modelID: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", provider: "gemini"},
	{marker: "gemini-3.8-flash-medium", modelID: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", provider: "gemini"},
	{marker: "gemini-3.8-flash-low", modelID: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", provider: "gemini"},
	{marker: "gemini-3.8-flash-tiered", modelID: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", provider: "gemini"},
	{marker: "gemini-3.8-flash", modelID: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", provider: "gemini"},
	{marker: "gemini-3.7-flash-high", modelID: "gemini-3.7-flash", displayName: "Gemini 3.7 Flash", provider: "gemini"},
	{marker: "gemini-3.7-flash-medium", modelID: "gemini-3.7-flash", displayName: "Gemini 3.7 Flash", provider: "gemini"},
	{marker: "gemini-3.7-flash-low", modelID: "gemini-3.7-flash", displayName: "Gemini 3.7 Flash", provider: "gemini"},
	{marker: "gemini-3.7-flash", modelID: "gemini-3.7-flash", displayName: "Gemini 3.7 Flash", provider: "gemini"},
	{marker: "gemini-3.6-flash-high", modelID: "gemini-3.6-flash", displayName: "Gemini 3.6 Flash", provider: "gemini"},
	{marker: "gemini-3.6-flash-medium", modelID: "gemini-3.6-flash", displayName: "Gemini 3.6 Flash", provider: "gemini"},
	{marker: "gemini-3.6-flash-low", modelID: "gemini-3.6-flash", displayName: "Gemini 3.6 Flash", provider: "gemini"},
	{marker: "gemini-3.6-flash", modelID: "gemini-3.6-flash", displayName: "Gemini 3.6 Flash", provider: "gemini"},
	{marker: "gemini-3-flash-agent", modelID: "gemini-3-flash", displayName: "Gemini 3 Flash", provider: "gemini"},
	{marker: "gemini-pro-agent", modelID: "gemini-3.1-pro", displayName: "Gemini 3.1 Pro", provider: "gemini"},
	{marker: "gemini-3.1-pro-low", modelID: "gemini-3.1-pro", displayName: "Gemini 3.1 Pro", provider: "gemini"},
	{marker: "gemini-3.1-pro-high", modelID: "gemini-3.1-pro", displayName: "Gemini 3.1 Pro", provider: "gemini"},
	{marker: "gemini-3.1-pro", modelID: "gemini-3.1-pro", displayName: "Gemini 3.1 Pro", provider: "gemini"},
	{marker: "gemini-2.5-pro", modelID: "gemini-2.5-pro", displayName: "Gemini 2.5 Pro", provider: "gemini"},
	{marker: "gemini-2.5-flash", modelID: "gemini-2.5-flash", displayName: "Gemini 2.5 Flash", provider: "gemini"},
}

// SetCustomModelsStore replaces the customModelsStore on the server (useful for isolated tests).
func (s *Server) SetCustomModelsStore(store *custommodels.Store) {
	s.customModelsStore = store
}

// SetAntigravityDataDir overrides the ~/.gemini/antigravity directory path (useful for isolated tests).
func (s *Server) SetAntigravityDataDir(dir string) {
	s.memoMu.Lock()
	defer s.memoMu.Unlock()
	s.antigravityDataDir = dir
}

// SetCatalogFetcher overrides the function used to retrieve available Antigravity models (useful for isolated tests).
func (s *Server) SetCatalogFetcher(fn func(force bool) []custommodels.CatalogModelInput) {
	s.memoMu.Lock()
	defer s.memoMu.Unlock()
	s.catalogFetcher = fn
}

func (s *Server) getAntigravityDataDir() string {
	s.memoMu.RLock()
	dir := s.antigravityDataDir
	s.memoMu.RUnlock()
	if dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gemini", "antigravity")
}

func (s *Server) ensureCustomModelsStore() (*custommodels.Store, error) {
	if s.customModelsStore != nil {
		return s.customModelsStore, nil
	}
	store, err := custommodels.NewStore("")
	if err != nil {
		return nil, err
	}
	s.customModelsStore = store
	return store, nil
}

func (s *Server) fetchAvailableCatalogModels(force bool) []custommodels.CatalogModelInput {
	s.memoMu.RLock()
	fn := s.catalogFetcher
	s.memoMu.RUnlock()
	if fn != nil {
		return fn(force)
	}

	kStore, _ := keyring.NewStore("")
	var acc *keyring.Account
	if kStore != nil {
		active := kStore.ActiveAccount()
		if active != "" {
			acc, _ = kStore.GetAccount(active)
		}
	}
	cat := quota.GetAvailableModelCatalog(acc, force)
	if cat == nil {
		return nil
	}
	total := len(cat.GeminiModels) + len(cat.NonGeminiModels)
	if total == 0 {
		return nil
	}
	out := make([]custommodels.CatalogModelInput, 0, total)
	for _, m := range cat.GeminiModels {
		out = append(out, custommodels.CatalogModelInput{
			ID:          m.ID,
			DisplayName: m.DisplayName,
			Provider:    m.Provider,
		})
	}
	for _, m := range cat.NonGeminiModels {
		out = append(out, custommodels.CatalogModelInput{
			ID:          m.ID,
			DisplayName: m.DisplayName,
			Provider:    m.Provider,
		})
	}
	return out
}

func (s *Server) syncTokenPricing(ctx context.Context, force bool) []custommodels.ModelPricingRecord {
	store, err := s.ensureCustomModelsStore()
	if err != nil {
		return nil
	}
	catalogModels := s.fetchAvailableCatalogModels(force)
	return store.SyncPricingWithCatalog(ctx, catalogModels, force)
}

func parseTranscriptFile(tPath string, info os.FileInfo) cachedTranscriptStats {
	tokenCacheMu.RLock()
	if cached, ok := transcriptCacheMap[tPath]; ok && cached.modTimeNano == info.ModTime().UnixNano() && cached.size == info.Size() {
		tokenCacheMu.RUnlock()
		return cached
	}
	tokenCacheMu.RUnlock()

	stats := cachedTranscriptStats{
		modTimeNano: info.ModTime().UnixNano(),
		size:        info.Size(),
		lastActive:  info.ModTime(),
	}

	f, err := os.Open(tPath)
	if err != nil {
		return stats
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 128*1024)
	scanner.Buffer(buf, 4*1024*1024)

	type transcriptItem struct {
		stepIndex       int
		itemType        string
		timestamp       time.Time
		hasTokens       bool
		inputTokens     int64
		cacheReadTokens int64
		outputTokens    int64
		durationMs      int64
	}

	var items []transcriptItem
	for scanner.Scan() {
		line := scanner.Bytes()
		if !stats.isSubagent {
			if bytes.Contains(line, []byte("subagent_reminder")) ||
				bytes.Contains(line, []byte("You are running as a subagent")) ||
				bytes.Contains(line, []byte("You are a pipeline coordinator")) ||
				bytes.Contains(line, []byte("delegate to your worker subagents")) ||
				(bytes.Contains(line, []byte("sender=")) && (bytes.Contains(line, []byte("MESSAGE_PRIORITY")) || bytes.Contains(line, []byte("priority=")))) {
				stats.isSubagent = true
				stats.agentRole = "Sub-Agent"
			}
		}
		if bytes.Contains(line, []byte("invoke_subagent")) && !stats.isSubagent {
			stats.agentRole = "Main Agent"
		}

		var lineTime struct {
			CreatedAt string `json:"created_at"`
		}
		var parsedTime time.Time
		if bytes.Contains(line, []byte("created_at")) {
			if err := json.Unmarshal(line, &lineTime); err == nil && lineTime.CreatedAt != "" {
				if t, err := time.Parse(time.RFC3339Nano, lineTime.CreatedAt); err == nil {
					parsedTime = t
				} else if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", lineTime.CreatedAt); err == nil {
					parsedTime = t
				} else if t, err := time.Parse(time.RFC3339, lineTime.CreatedAt); err == nil {
					parsedTime = t
				}
			}
		}
		if parsedTime.IsZero() {
			parsedTime = info.ModTime()
		}
		if parsedTime.After(stats.lastActive) {
			stats.lastActive = parsedTime
		}

		hasTokens := bytes.Contains(line, []byte("input_tokens")) || bytes.Contains(line, []byte("output_tokens")) || bytes.Contains(line, []byte("cache_read_tokens"))
		var entry struct {
			Type            string `json:"type"`
			StepIndex       int    `json:"step_index"`
			DurationMs      int64  `json:"duration_ms"`
			InputTokens     int64  `json:"input_tokens"`
			OutputTokens    int64  `json:"output_tokens"`
			CacheReadTokens int64  `json:"cache_read_tokens"`
		}
		if hasTokens {
			if err := json.Unmarshal(line, &entry); err == nil {
				if entry.InputTokens > 0 || entry.OutputTokens > 0 || entry.CacheReadTokens > 0 {
					stats.freshInput += entry.InputTokens
					stats.cachedInput += entry.CacheReadTokens
					stats.outputTokens += entry.OutputTokens
					stats.requests++

					items = append(items, transcriptItem{
						stepIndex:       entry.StepIndex,
						itemType:        entry.Type,
						timestamp:       parsedTime,
						hasTokens:       true,
						inputTokens:     entry.InputTokens,
						cacheReadTokens: entry.CacheReadTokens,
						outputTokens:    entry.OutputTokens,
						durationMs:      entry.DurationMs,
					})
					continue
				}
			}
		}

		items = append(items, transcriptItem{
			stepIndex: entry.StepIndex,
			itemType:  entry.Type,
			timestamp: parsedTime,
			hasTokens: false,
		})
	}

	for i := 0; i < len(items); i++ {
		item := items[i]
		if !item.hasTokens {
			continue
		}

		var durMs int64
		if item.durationMs > 0 {
			durMs = item.durationMs
		} else if item.outputTokens > 0 {
			// Forward lookahead interval: nextStepTime.Sub(curStepTime) measuring actual streaming response duration
			for j := i + 1; j < len(items) && j <= i+6; j++ {
				nextItem := items[j]
				if nextItem.timestamp.After(item.timestamp) {
					diff := nextItem.timestamp.Sub(item.timestamp).Milliseconds()
					if diff >= 200 && diff <= 600000 {
						impliedTPS := float64(item.outputTokens) / (float64(diff) / 1000.0)
						if impliedTPS >= 3.0 && impliedTPS <= 220.0 {
							durMs = diff
							break
						}
					}
				}
			}
		}

		if durMs > 0 && item.outputTokens > 0 {
			rawTPS := float64(item.outputTokens) / (float64(durMs) / 1000.0)
			if rawTPS >= 3.0 && rawTPS <= 220.0 {
				stats.totalDurationMs += durMs
				stats.timedOutputTokens += item.outputTokens
			} else if rawTPS > 220.0 {
				// Bound realistic generation speed
				clampedDurMs := int64(math.Ceil(float64(item.outputTokens) / 220.0 * 1000.0))
				durMs = clampedDurMs
				stats.totalDurationMs += clampedDurMs
				stats.timedOutputTokens += item.outputTokens
			} else {
				// unmeasured / stalled step (< 3 TPS)
				durMs = 0
			}
		}

		stats.steps = append(stats.steps, rawStepEvent{
			stepIndex:   item.stepIndex,
			createdAt:   item.timestamp,
			freshInput:  item.inputTokens,
			cachedInput: item.cacheReadTokens,
			output:      item.outputTokens,
			durationMs:  durMs,
		})
	}

	tokenCacheMu.Lock()
	transcriptCacheMap[tPath] = stats
	tokenCacheMu.Unlock()
	return stats
}

var modelSelectionRegex = regexp.MustCompile(`(?i)setting\s+[` + "`" + `']Model Selection[` + "`" + `'].*?to\s+(.+?)(?:\.\s+|\.?(?:\r|\n|</USER_SETTINGS_CHANGE>))`)

// extractModelFromTranscript reads step 0 from transcript.jsonl and extracts any model selection from <USER_SETTINGS_CHANGE>.
func extractModelFromTranscript(tPath string) (modelName string, ok bool) {
	f, err := os.Open(tPath)
	if err != nil {
		return "", false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Read step 0 (first non-empty line)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var step struct {
			StepIndex int    `json:"step_index"`
			Content   string `json:"content"`
		}
		if err := json.Unmarshal(line, &step); err != nil {
			break
		}
		content := step.Content
		if startIdx := strings.Index(content, "<USER_SETTINGS_CHANGE>"); startIdx >= 0 {
			endIdx := strings.Index(content[startIdx:], "</USER_SETTINGS_CHANGE>")
			var block string
			if endIdx >= 0 {
				block = content[startIdx : startIdx+endIdx+len("</USER_SETTINGS_CHANGE>")]
			} else {
				block = content[startIdx:]
			}
			if m := modelSelectionRegex.FindStringSubmatch(block); len(m) > 1 {
				mName := strings.TrimSpace(m[1])
				if mName != "" {
					return mName, true
				}
			}
		}
		break
	}
	return "", false
}

// mapModelNameToMetadata maps raw model strings from settings to canonical modelID, displayName, and provider.
func mapModelNameToMetadata(rawName string, customModels []custommodels.CustomModel, catalogModels []custommodels.CatalogModelInput) (modelID, displayName, provider string) {
	rawTrimmed := strings.TrimSpace(rawName)
	if rawTrimmed == "" {
		return "gemini-3.8-flash", "Gemini 3.8 Flash", "gemini"
	}

	// 1. Match custom models
	for _, cm := range customModels {
		if strings.EqualFold(cm.Name, rawTrimmed) || (cm.DisplayName != "" && strings.EqualFold(cm.DisplayName, rawTrimmed)) {
			disp := cm.DisplayName
			if disp == "" {
				disp = cm.Name
			}
			prov := custommodels.NormalizePricingProvider(string(cm.ProviderType), cm.Name)
			return cm.Name, disp, prov
		}
	}

	// 2. Match catalog models
	for _, cat := range catalogModels {
		if strings.EqualFold(cat.ID, rawTrimmed) || (cat.DisplayName != "" && strings.EqualFold(cat.DisplayName, rawTrimmed)) {
			canID, canName := custommodels.NormalizeCanonicalNativeModel(cat.ID, cat.DisplayName)
			prov := custommodels.NormalizePricingProvider(cat.Provider, canID)
			return canID, canName, prov
		}
	}

	// 3. Gemini Native model family mappings
	lower := strings.ToLower(rawTrimmed)
	if strings.Contains(lower, "gemini 4") || strings.Contains(lower, "gemini-4") {
		return "gemini-4-argon", "Gemini 4 Argon", "gemini"
	}
	if strings.Contains(lower, "gemini 3.8 flash") || strings.Contains(lower, "gemini-3.8-flash") {
		return "gemini-3.8-flash", "Gemini 3.8 Flash", "gemini"
	}
	if strings.Contains(lower, "gemini 3.7 flash") || strings.Contains(lower, "gemini-3.7-flash") {
		return "gemini-3.7-flash", "Gemini 3.7 Flash", "gemini"
	}
	if strings.Contains(lower, "gemini 3.6 flash") || strings.Contains(lower, "gemini-3.6-flash") {
		return "gemini-3.6-flash", "Gemini 3.6 Flash", "gemini"
	}
	if strings.Contains(lower, "gemini 3.1 pro") || strings.Contains(lower, "gemini-3.1-pro") || strings.Contains(lower, "gemini pro") {
		return "gemini-3.1-pro", "Gemini 3.1 Pro", "gemini"
	}
	if strings.Contains(lower, "gemini 3 flash") || strings.Contains(lower, "gemini-3-flash") {
		return "gemini-3-flash", "Gemini 3 Flash", "gemini"
	}
	if strings.Contains(lower, "gemini 2.5 pro") || strings.Contains(lower, "gemini-2.5-pro") {
		return "gemini-2.5-pro", "Gemini 2.5 Pro", "gemini"
	}
	if strings.Contains(lower, "gemini 2.5 flash") || strings.Contains(lower, "gemini-2.5-flash") {
		return "gemini-2.5-flash", "Gemini 2.5 Flash", "gemini"
	}

	// 4. Anthropic models - strictly only when genuinely selected in settings
	if strings.Contains(lower, "claude opus 4.6") || strings.Contains(lower, "claude-opus-4-6") || strings.Contains(lower, "claude opus") {
		return "claude-opus-4-6", "Claude Opus 4.6", "anthropic"
	}
	if strings.Contains(lower, "claude 3.7 sonnet") || strings.Contains(lower, "claude-3-7-sonnet") || strings.Contains(lower, "claude sonnet") {
		return "claude-3-7-sonnet", "Claude 3.7 Sonnet", "anthropic"
	}

	// 5. OpenAI models
	if strings.Contains(lower, "gpt-oss-120b") {
		return "gpt-oss-120b", "GPT-OSS 120B", "openai"
	}

	canID, canName := custommodels.NormalizeCanonicalNativeModel(rawTrimmed, rawTrimmed)
	if canID != "" && canID != rawTrimmed {
		prov := custommodels.NormalizePricingProvider("", canID)
		return canID, canName, prov
	}

	return "gemini-3.8-flash", "Gemini 3.8 Flash", "gemini"
}

// detectConversationModelWithTranscript detects the conversation model using transcript.jsonl step 0 <USER_SETTINGS_CHANGE>,
// eliminating false positives from SQLite database byte-scanning.
func detectConversationModelWithTranscript(tPath string, convDBPath string, customModels []custommodels.CustomModel, catalogModels []custommodels.CatalogModelInput) (modelID, displayName, provider string) {
	// Locate transcript path if not explicitly provided
	if tPath == "" {
		if strings.HasSuffix(convDBPath, "transcript.jsonl") {
			tPath = convDBPath
		} else if strings.HasSuffix(convDBPath, ".db") {
			convID := strings.TrimSuffix(filepath.Base(convDBPath), ".db")
			parentDir := filepath.Dir(filepath.Dir(convDBPath))
			candidate := filepath.Join(parentDir, "brain", convID, ".system_generated", "logs", "transcript.jsonl")
			if _, err := os.Stat(candidate); err == nil {
				tPath = candidate
			} else if home, err := os.UserHomeDir(); err == nil {
				homeCandidate := filepath.Join(home, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs", "transcript.jsonl")
				if _, err := os.Stat(homeCandidate); err == nil {
					tPath = homeCandidate
				}
			}
		} else if convDBPath != "" && !strings.Contains(convDBPath, string(filepath.Separator)) {
			// convDBPath is a bare conversation ID
			convID := convDBPath
			if home, err := os.UserHomeDir(); err == nil {
				homeCandidate := filepath.Join(home, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs", "transcript.jsonl")
				if _, err := os.Stat(homeCandidate); err == nil {
					tPath = homeCandidate
				}
			}
		}
	}

	// Determine cache key and stat info
	var cacheKey string
	var cacheModTime int64
	var cacheSize int64

	if tPath != "" {
		if info, err := os.Stat(tPath); err == nil && !info.IsDir() {
			cacheKey = tPath
			cacheModTime = info.ModTime().UnixNano()
			cacheSize = info.Size()
		}
	}
	if cacheKey == "" && convDBPath != "" {
		if info, err := os.Stat(convDBPath); err == nil && !info.IsDir() {
			cacheKey = convDBPath
			cacheModTime = info.ModTime().UnixNano()
			cacheSize = info.Size()
		}
	}

	if cacheKey != "" {
		tokenCacheMu.RLock()
		if cached, ok := convModelCacheMap[cacheKey]; ok && cached.modTimeNano == cacheModTime && cached.size == cacheSize {
			tokenCacheMu.RUnlock()
			return cached.modelID, cached.displayName, cached.provider
		}
		tokenCacheMu.RUnlock()
	}

	// 1. Authoritative check: transcript.jsonl step 0 <USER_SETTINGS_CHANGE>
	if tPath != "" {
		if rawName, ok := extractModelFromTranscript(tPath); ok {
			mID, mName, mProv := mapModelNameToMetadata(rawName, customModels, catalogModels)
			if cacheKey != "" {
				tokenCacheMu.Lock()
				convModelCacheMap[cacheKey] = cachedConvModel{
					modTimeNano: cacheModTime,
					size:        cacheSize,
					modelID:     mID,
					displayName: mName,
					provider:    mProv,
				}
				tokenCacheMu.Unlock()
			}
			return mID, mName, mProv
		}
	}

	// 2. If step 0 has no settings change, check if convDBPath contains an explicitly configured Custom Model
	if convDBPath != "" && strings.HasSuffix(convDBPath, ".db") {
		if f, err := os.Open(convDBPath); err == nil {
			buf := make([]byte, 64*1024)
			n, _ := f.Read(buf)
			f.Close()
			data := buf[:n]

			for _, cm := range customModels {
				if cm.Name != "" && bytes.Contains(data, []byte(cm.Name)) {
					disp := cm.DisplayName
					if disp == "" {
						disp = cm.Name
					}
					prov := custommodels.NormalizePricingProvider(string(cm.ProviderType), cm.Name)
					if cacheKey != "" {
						tokenCacheMu.Lock()
						convModelCacheMap[cacheKey] = cachedConvModel{
							modTimeNano: cacheModTime,
							size:        cacheSize,
							modelID:     cm.Name,
							displayName: disp,
							provider:    prov,
						}
						tokenCacheMu.Unlock()
					}
					return cm.Name, disp, prov
				}
			}
		}
	}

	// 3. Fall back safely to active native Gemini default
	mID := "gemini-3.8-flash"
	mName := "Gemini 3.8 Flash"
	mProv := "gemini"

	if cacheKey != "" {
		tokenCacheMu.Lock()
		convModelCacheMap[cacheKey] = cachedConvModel{
			modTimeNano: cacheModTime,
			size:        cacheSize,
			modelID:     mID,
			displayName: mName,
			provider:    mProv,
		}
		tokenCacheMu.Unlock()
	}

	return mID, mName, mProv
}

// detectConversationModel detects the model for a conversation, delegating to transcript extraction.
func detectConversationModel(convDBPath string, customModels []custommodels.CustomModel, catalogModels []custommodels.CatalogModelInput) (modelID, displayName, provider string) {
	mID, mName, mProv := detectConversationModelWithTranscript("", convDBPath, customModels, catalogModels)
	if mID == "" {
		return "gemini-3.8-flash", "Gemini 3.8 Flash", "gemini"
	}
	return mID, mName, mProv
}

// resolveConversationModel resolves the active model for a conversation ID or path by reading transcript.jsonl step 0.
func resolveConversationModel(convID string) (modelID, displayName, provider string) {
	mID, mName, mProv := detectConversationModelWithTranscript("", convID, nil, nil)
	if mID == "" {
		return "gemini-3.8-flash", "Gemini 3.8 Flash", "gemini"
	}
	return mID, mName, mProv
}

func resolveProjectFromURI(uris []string) (projectName, workspacePath string) {
	for _, u := range uris {
		trimmed := strings.TrimSpace(u)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "file://") {
			if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
				decoded, err := url.PathUnescape(parsed.Path)
				if err == nil {
					trimmed = decoded
				} else {
					trimmed = parsed.Path
				}
			} else {
				trimmed = strings.TrimPrefix(trimmed, "file://")
			}
		}
		trimmed = strings.TrimRight(trimmed, "/")
		if trimmed != "" {
			base := filepath.Base(trimmed)
			if base != "" && base != "." && base != "/" {
				return base, trimmed
			}
		}
	}
	return "Unassigned Sessions", "local://unassigned"
}

type conversationSummaryMeta struct {
	workspaceURIs []string
	parentConvID  string
	nestingDepth  int
	agentName     string
}

func readConversationMetadataMap(summariesDB string) map[string]conversationSummaryMeta {
	out := make(map[string]conversationSummaryMeta)
	if _, err := os.Stat(summariesDB); err != nil {
		return out
	}
	db, err := sql.Open("sqlite", summariesDB+"?mode=ro&_busy_timeout=2000")
	if err != nil {
		return out
	}
	defer db.Close()

	rows, err := db.Query("SELECT conversation_id, COALESCE(workspace_uris, ''), COALESCE(parent_conversation_id, ''), nesting_depth, COALESCE(agent_name, '') FROM conversation_summaries")
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var convID, urisJSON, parentConvID, agentName string
		var nestingDepth int
		if err := rows.Scan(&convID, &urisJSON, &parentConvID, &nestingDepth, &agentName); err != nil || convID == "" {
			continue
		}
		var uris []string
		if urisJSON != "" {
			if err := json.Unmarshal([]byte(urisJSON), &uris); err != nil {
				uris = []string{urisJSON}
			}
		}
		out[convID] = conversationSummaryMeta{
			workspaceURIs: uris,
			parentConvID:  parentConvID,
			nestingDepth:  nestingDepth,
			agentName:     agentName,
		}
	}
	return out
}

func readConversationWorkspaceMap(summariesDB string) map[string][]string {
	meta := readConversationMetadataMap(summariesDB)
	out := make(map[string][]string, len(meta))
	for k, v := range meta {
		out[k] = v.workspaceURIs
	}
	return out
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	diff := time.Since(t)
	if diff < time.Minute {
		return "Just now"
	}
	if diff < time.Hour {
		return strconv.Itoa(int(diff.Minutes())) + "m ago"
	}
	if diff < 24*time.Hour {
		return strconv.Itoa(int(diff.Hours())) + "h ago"
	}
	days := int(diff.Hours() / 24)
	if days < 30 {
		return strconv.Itoa(days) + "d ago"
	}
	return t.Format("2006-01-02")
}

// tokenRangeSpec describes the aggregation window and histogram bucketing used by
// the token monitor summary endpoint.
type tokenRangeSpec struct {
	Key      string        // canonical range key: "24h" | "7d" | "30d" | "all"
	Duration time.Duration // lookback window; 0 means all recorded history
	Bucket   time.Duration // histogram bucket width (fixed ranges only)
	Buckets  int           // fixed bucket count (fixed ranges only)
}

// Supported token monitor ranges. "all" adapts its bucket width to the recorded span.
const (
	tokenRange24h = "24h"
	tokenRange7d  = "7d"
	tokenRange30d = "30d"
	tokenRangeAll = "all"

	maxAdaptiveBuckets = 120
)

func parseTokenRange(raw string) tokenRangeSpec {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case tokenRange24h:
		return tokenRangeSpec{Key: tokenRange24h, Duration: 24 * time.Hour, Bucket: time.Hour, Buckets: 24}
	case tokenRange7d:
		return tokenRangeSpec{Key: tokenRange7d, Duration: 7 * 24 * time.Hour, Bucket: 24 * time.Hour, Buckets: 7}
	case tokenRange30d:
		return tokenRangeSpec{Key: tokenRange30d, Duration: 30 * 24 * time.Hour, Bucket: 24 * time.Hour, Buckets: 30}
	default:
		return tokenRangeSpec{Key: tokenRangeAll}
	}
}

// cutoff returns the inclusive lower bound of the range (zero time for "all").
func (s tokenRangeSpec) cutoff(now time.Time) time.Time {
	if s.Duration <= 0 {
		return time.Time{}
	}
	if s.Key == tokenRange24h {
		// Hour-aligned so that the last bucket is the current clock hour.
		return now.Truncate(time.Hour).Add(-time.Duration(s.Buckets-1) * s.Bucket)
	}
	// Day-aligned local midnight so daily bars match the user's calendar.
	return startOfLocalDay(now).Add(-time.Duration(s.Buckets-1) * s.Bucket)
}

func startOfLocalDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// tokenStepAgg is the window-filtered aggregate of token-bearing steps.
type tokenStepAgg struct {
	freshInput   int64
	cachedInput  int64
	outputTokens int64
	requests     int
	durationMs   int64
	timedOutput  int64
	steps        []rawStepEvent
}

// aggregateSteps filters steps to the window starting at cutoff (zero = all steps)
// and folds them the same way parseTranscriptFile folds whole files. Because each
// transcript step is read exactly once and every step carries its own timestamp,
// recovered input (fresh), cache reads and output are disjoint quantities per
// step: cache_read_tokens is NOT a subset of input_tokens (live transcripts show
// cache/fresh ratios of 20-25x), so
// fresh_input + cache_read + output is the correct prompt+completion total.
func aggregateSteps(steps []rawStepEvent, cutoff time.Time) tokenStepAgg {
	var agg tokenStepAgg
	for _, st := range steps {
		if !cutoff.IsZero() && st.createdAt.Before(cutoff) {
			continue
		}
		agg.freshInput += st.freshInput
		agg.cachedInput += st.cachedInput
		agg.outputTokens += st.output
		agg.requests++
		if st.durationMs > 0 && st.output > 0 {
			agg.durationMs += st.durationMs
			agg.timedOutput += st.output
		}
		agg.steps = append(agg.steps, st)
	}
	return agg
}

// TokenTrendSegment is one stacked histogram segment inside a trend bucket
// (a model or a project).
type TokenTrendSegment struct {
	Name     string  `json:"name"`
	Tokens   int64   `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
	Requests int     `json:"requests"`
}

// TokenTrendBucket is a single histogram bar: the bucket window plus its real
// aggregated usage, including the per-segment split used for stacked rendering.
type TokenTrendBucket struct {
	Start        int64               `json:"start"` // unix milliseconds
	FreshTokens  int64               `json:"fresh_tokens"`
	CachedTokens int64               `json:"cached_tokens"`
	OutputTokens int64               `json:"output_tokens"`
	TotalTokens  int64               `json:"total_tokens"`
	CostUSD      float64             `json:"cost_usd"`
	Requests     int                 `json:"requests"`
	Models       []TokenTrendSegment `json:"models"`
	Projects     []TokenTrendSegment `json:"projects"`
}

// tokenTrendAccumulator collects real per-bucket usage keyed by model/project.
type tokenTrendAccumulator struct {
	start       []int64
	fresh       []int64
	cached      []int64
	output      []int64
	cost        []float64
	requests    []int
	modelTokens []map[string]int64
	modelCost   []map[string]float64
	modelReqs   []map[string]int
	projTokens  []map[string]int64
	projCost    []map[string]float64
	projReqs    []map[string]int
}

func newTokenTrendAccumulator(buckets int) *tokenTrendAccumulator {
	acc := &tokenTrendAccumulator{
		start:       make([]int64, buckets),
		fresh:       make([]int64, buckets),
		cached:      make([]int64, buckets),
		output:      make([]int64, buckets),
		cost:        make([]float64, buckets),
		requests:    make([]int, buckets),
		modelTokens: make([]map[string]int64, buckets),
		modelCost:   make([]map[string]float64, buckets),
		modelReqs:   make([]map[string]int, buckets),
		projTokens:  make([]map[string]int64, buckets),
		projCost:    make([]map[string]float64, buckets),
		projReqs:    make([]map[string]int, buckets),
	}
	for i := 0; i < buckets; i++ {
		acc.modelTokens[i] = make(map[string]int64)
		acc.modelCost[i] = make(map[string]float64)
		acc.modelReqs[i] = make(map[string]int)
		acc.projTokens[i] = make(map[string]int64)
		acc.projCost[i] = make(map[string]float64)
		acc.projReqs[i] = make(map[string]int)
	}
	return acc
}

func (a *tokenTrendAccumulator) add(idx int, st rawStepEvent, cost float64, modelName, projectName string) {
	if idx < 0 || idx >= len(a.start) {
		return
	}
	total := st.freshInput + st.cachedInput + st.output
	a.fresh[idx] += st.freshInput
	a.cached[idx] += st.cachedInput
	a.output[idx] += st.output
	a.cost[idx] += cost
	a.requests[idx]++
	if modelName != "" {
		a.modelTokens[idx][modelName] += total
		a.modelCost[idx][modelName] += cost
		a.modelReqs[idx][modelName]++
	}
	if projectName != "" {
		a.projTokens[idx][projectName] += total
		a.projCost[idx][projectName] += cost
		a.projReqs[idx][projectName]++
	}
}

func sortedTrendSegments(tokens map[string]int64, costs map[string]float64, reqs map[string]int) []TokenTrendSegment {
	segs := make([]TokenTrendSegment, 0, len(tokens))
	for name, tok := range tokens {
		segs = append(segs, TokenTrendSegment{
			Name:     name,
			Tokens:   tok,
			CostUSD:  costs[name],
			Requests: reqs[name],
		})
	}
	sort.Slice(segs, func(i, j int) bool {
		if segs[i].Tokens != segs[j].Tokens {
			return segs[i].Tokens > segs[j].Tokens
		}
		return segs[i].Name < segs[j].Name
	})
	return segs
}

func (a *tokenTrendAccumulator) buckets() []TokenTrendBucket {
	out := make([]TokenTrendBucket, 0, len(a.start))
	for i := range a.start {
		total := a.fresh[i] + a.cached[i] + a.output[i]
		out = append(out, TokenTrendBucket{
			Start:        a.start[i],
			FreshTokens:  a.fresh[i],
			CachedTokens: a.cached[i],
			OutputTokens: a.output[i],
			TotalTokens:  total,
			CostUSD:      a.cost[i],
			Requests:     a.requests[i],
			Models:       sortedTrendSegments(a.modelTokens[i], a.modelCost[i], a.modelReqs[i]),
			Projects:     sortedTrendSegments(a.projTokens[i], a.projCost[i], a.projReqs[i]),
		})
	}
	return out
}

// buildTokenTrend turns the real in-window steps into the stacked histogram
// shown by the dashboard (#125). Fixed ranges (24h/7d/30d) use their canonical
// bucket width; the "all" range adapts its width to the recorded span, capped
// at maxAdaptiveBuckets. An empty window yields buckets whose totals are all
// zero (the UI renders the axes plus "No data in this range"); nothing is ever
// synthesised or interpolated.
func buildTokenTrend(windowSteps []stepWithContext, rangeSpec tokenRangeSpec, cutoff, now time.Time) ([]TokenTrendBucket, int64) {
	trendSpec := rangeSpec
	trendStart := cutoff
	if trendSpec.Key == tokenRangeAll {
		var earliest time.Time
		for _, st := range windowSteps {
			if earliest.IsZero() || st.createdAt.Before(earliest) {
				earliest = st.createdAt
			}
		}
		if earliest.IsZero() {
			return nil, 0
		}
		span := now.Sub(earliest)
		bucket := 24 * time.Hour
		if span > 60*24*time.Hour {
			bucket = 7 * 24 * time.Hour
		}
		buckets := int(span/bucket) + 1
		for buckets > maxAdaptiveBuckets {
			bucket *= 2
			buckets = int(span/bucket) + 1
		}
		trendSpec = tokenRangeSpec{Key: tokenRangeAll, Bucket: bucket, Buckets: buckets}
		trendStart = startOfLocalDay(earliest)
	}

	if trendSpec.Bucket <= 0 || trendSpec.Buckets <= 0 {
		return nil, 0
	}

	acc := newTokenTrendAccumulator(trendSpec.Buckets)
	for i := range acc.start {
		acc.start[i] = trendStart.Add(time.Duration(i) * trendSpec.Bucket).UnixMilli()
	}
	for _, st := range windowSteps {
		idx := int(st.createdAt.Sub(trendStart) / trendSpec.Bucket)
		if idx < 0 {
			continue
		}
		if idx >= trendSpec.Buckets {
			idx = trendSpec.Buckets - 1
		}
		acc.add(idx, st.rawStepEvent, st.costUSD, st.modelName, st.project)
	}
	return acc.buckets(), trendSpec.Bucket.Milliseconds()
}

func (s *Server) handleTokensSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Time window filter (#125/#126): the KPI cards and the histogram consume the
	// same window so the displayed totals always match the selected range.
	rangeSpec := parseTokenRange(r.URL.Query().Get("range"))
	now := time.Now()
	cutoff := rangeSpec.cutoff(now)

	store, err := s.ensureCustomModelsStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	catalogModels := s.fetchAvailableCatalogModels(false)
	pricingRecords := store.SyncPricingWithCatalog(r.Context(), catalogModels, false)
	customModelsList := store.ListModels()

	dataDir := s.getAntigravityDataDir()
	brainDir := filepath.Join(dataDir, "brain")
	convDir := filepath.Join(dataDir, "conversations")
	summariesDB := filepath.Join(dataDir, "conversation_summaries.db")
	metaMap := readConversationMetadataMap(summariesDB)

	// Build lookup map of pricing records by lowercase ModelID
	pricingByModel := make(map[string]custommodels.ModelPricingRecord, len(pricingRecords))
	for _, rec := range pricingRecords {
		pricingByModel[strings.ToLower(rec.ModelID)] = rec
	}

	type modelAgg struct {
		internalID       int64
		modelID          string
		modelName        string
		provider         string
		classification   custommodels.ModelClassification
		isOutdatedNative bool
		freshInput       int64
		cachedInput      int64
		outputTokens     int64
		requests         int
		costUSD          float64
		savedUSD         float64
		durationMs       int64
		timedOutput      int64
		priceConfigured  bool
	}

	type projectAgg struct {
		projectName   string
		workspacePath string
		freshInput    int64
		cachedInput   int64
		outputTokens  int64
		requests      int
		costUSD       float64
		savedUSD      float64
		sessionsCount int
		lastActive    time.Time
		tokensByModel map[string]int64
	}

	modelMap := make(map[string]*modelAgg)
	projectMap := make(map[string]*projectAgg)

	// Pre-populate configured workspace projects from GUI store so all known projects appear in breakdown
	if s.guiStore != nil {
		guiCfg := s.guiStore.GetConfig()
		for _, pPath := range guiCfg.ProjectOrder {
			clean := strings.TrimRight(strings.TrimSpace(pPath), "/")
			if clean == "" {
				continue
			}
			pName := filepath.Base(clean)
			if _, exists := projectMap[pName]; !exists {
				projectMap[pName] = &projectAgg{
					projectName:   pName,
					workspacePath: clean,
					tokensByModel: make(map[string]int64),
				}
			}
		}
	}

	var totalFreshInput int64
	var totalCachedInput int64
	var totalOutputTokens int64
	var totalCostUSD float64
	var totalInputCostUSD float64
	var totalOutputCostUSD float64
	var totalSavedUSD float64
	var totalRequests int
	var totalConversations int
	var totalDurationMs int64
	var totalTimedOutputTokens int64

	var allRecentSteps []stepWithContext
	// windowSteps holds every token step inside the selected range; the trend
	// histogram is built from these real records (no synthetic interpolation).
	var windowSteps []stepWithContext

	var discoveredHistoricalNative []custommodels.CatalogModelInput

	if entries, err := os.ReadDir(brainDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			convID := entry.Name()
			tPath := filepath.Join(brainDir, convID, ".system_generated", "logs", "transcript.jsonl")
			info, err := os.Stat(tPath)
			if err != nil || info.IsDir() {
				continue
			}

			tStats := parseTranscriptFile(tPath, info)
			if tStats.requests == 0 || (tStats.freshInput == 0 && tStats.cachedInput == 0 && tStats.outputTokens == 0) {
				continue
			}

			// Resolve project workspace and agent role from conversation_summaries.db
			meta := metaMap[convID]
			uris := meta.workspaceURIs
			projName, projPath := resolveProjectFromURI(uris)

			isSub := tStats.isSubagent
			agentRole := tStats.agentRole
			if meta, ok := metaMap[convID]; ok {
				if meta.parentConvID != "" || meta.nestingDepth > 0 || meta.agentName != "" {
					isSub = true
					if meta.agentName != "" {
						agentRole = meta.agentName
					} else if agentRole == "" || agentRole == "Main Agent" {
						agentRole = "Sub-Agent"
					}
				} else {
					isSub = false
					agentRole = "Main Agent"
				}
			} else {
				if !isSub && agentRole == "" {
					agentRole = "Main Agent"
				}
			}

			// Resolve actual model from transcript or conversations/<convID>.db
			convDBPath := filepath.Join(convDir, convID+".db")
			mID, mDisplayName, mProv := detectConversationModelWithTranscript(tPath, convDBPath, customModelsList, catalogModels)
			if mID == "" {
				if len(catalogModels) > 0 {
					mID, mDisplayName = custommodels.NormalizeCanonicalNativeModel(catalogModels[0].ID, catalogModels[0].DisplayName)
					mProv = custommodels.NormalizePricingProvider(catalogModels[0].Provider, mID)
				} else {
					mID = "gemini-3.8-flash"
					mDisplayName = "Gemini 3.8 Flash"
					mProv = "gemini"
				}
			}

			// Skip if user explicitly deleted this model from Token Monitor / Custom Models
			var pInternalID int64
			var pCustomID string
			if pRec, hasRec := pricingByModel[strings.ToLower(mID)]; hasRec {
				pInternalID = pRec.InternalID
				pCustomID = pRec.CustomModelID
			}
			if store.IsModelDeleted(pInternalID, mID, pCustomID) {
				continue
			}

			// Look up pricing record for this model
			pRec, hasRec := pricingByModel[strings.ToLower(mID)]
			if !hasRec {
				// Historical native model used in Antigravity conversations but not yet in PricingRecords
				inP, cacheP, outP, src := custommodels.FetchModelPricing(r.Context(), mID, mProv, "", "")
				pRec = custommodels.ModelPricingRecord{
					ModelID:              mID,
					Name:                 mDisplayName,
					Provider:             mProv,
					Classification:       custommodels.ClassificationCustom,
					IsOutdatedNative:     true,
					InputPricePerM:       inP,
					CachedInputPricePerM: cacheP,
					OutputPricePerM:      outP,
					Source:               src,
				}
				pricingByModel[strings.ToLower(mID)] = pRec
				discoveredHistoricalNative = append(discoveredHistoricalNative, custommodels.CatalogModelInput{
					ID:          mID,
					DisplayName: mDisplayName,
					Provider:    mProv,
				})
			} else if pRec.Name != "" {
				mDisplayName = pRec.Name
			}

			// Window-filtered aggregation of this conversation's real steps.
			agg := aggregateSteps(tStats.steps, cutoff)
			if agg.requests == 0 {
				// No token steps inside the selected range for this conversation.
				continue
			}

			priceConfigured := pRec.InputPricePerM != nil && pRec.OutputPricePerM != nil
			var inRate, cacheRate, outRate float64
			if priceConfigured {
				inRate = *pRec.InputPricePerM
				cacheRate = inRate * 0.25
				if pRec.CachedInputPricePerM != nil {
					cacheRate = *pRec.CachedInputPricePerM
				}
				outRate = *pRec.OutputPricePerM
			}
			stepCost := func(st rawStepEvent) float64 {
				if !priceConfigured {
					return 0
				}
				return (float64(st.freshInput)*inRate + float64(st.cachedInput)*cacheRate + float64(st.output)*outRate) / 1_000_000.0
			}

			var convCost, convSaved float64
			if priceConfigured {
				convInputCost := (float64(agg.freshInput)*inRate + float64(agg.cachedInput)*cacheRate) / 1_000_000.0
				convOutputCost := float64(agg.outputTokens) * outRate / 1_000_000.0
				convCost = convInputCost + convOutputCost
				if inRate > cacheRate {
					convSaved = float64(agg.cachedInput) * (inRate - cacheRate) / 1_000_000.0
				}
				totalInputCostUSD += convInputCost
				totalOutputCostUSD += convOutputCost
			}

			totalConversations++
			totalFreshInput += agg.freshInput
			totalCachedInput += agg.cachedInput
			totalOutputTokens += agg.outputTokens
			totalCostUSD += convCost
			totalSavedUSD += convSaved
			totalRequests += agg.requests
			if agg.durationMs > 0 && agg.timedOutput > 0 {
				totalDurationMs += agg.durationMs
				totalTimedOutputTokens += agg.timedOutput
			}

			// Aggregate by Model
			mKey := strings.ToLower(mID)
			ma, exists := modelMap[mKey]
			if !exists {
				ma = &modelAgg{
					internalID:       pRec.InternalID,
					modelID:          mID,
					modelName:        mDisplayName,
					provider:         pRec.Provider,
					classification:   pRec.Classification,
					isOutdatedNative: pRec.IsOutdatedNative,
					priceConfigured:  priceConfigured,
				}
				modelMap[mKey] = ma
			}
			ma.freshInput += agg.freshInput
			ma.cachedInput += agg.cachedInput
			ma.outputTokens += agg.outputTokens
			ma.requests += agg.requests
			ma.costUSD += convCost
			ma.savedUSD += convSaved
			if agg.durationMs > 0 && agg.timedOutput > 0 {
				ma.durationMs += agg.durationMs
				ma.timedOutput += agg.timedOutput
			}

			// Aggregate by Project
			pa, exists := projectMap[projName]
			if !exists {
				pa = &projectAgg{
					projectName:   projName,
					workspacePath: projPath,
					tokensByModel: make(map[string]int64),
				}
				projectMap[projName] = pa
			}
			convTotalTokens := agg.freshInput + agg.cachedInput + agg.outputTokens
			pa.freshInput += agg.freshInput
			pa.cachedInput += agg.cachedInput
			pa.outputTokens += agg.outputTokens
			pa.requests += agg.requests
			pa.costUSD += convCost
			pa.savedUSD += convSaved
			pa.sessionsCount++
			pa.tokensByModel[mDisplayName] += convTotalTokens
			if tStats.lastActive.After(pa.lastActive) {
				pa.lastActive = tStats.lastActive
			}

			// Collect the most recent steps of this conversation for the Live
			// Telemetry tab (a log: always the latest invocations, independent
			// of the selected aggregation window).
			recent := tStats.steps
			if len(recent) > 15 {
				recent = recent[len(recent)-15:]
			}
			for _, st := range recent {
				allRecentSteps = append(allRecentSteps, stepWithContext{
					rawStepEvent:    st,
					convID:          convID,
					project:         projName,
					modelName:       mDisplayName,
					modelID:         mID,
					provider:        pRec.Provider,
					classification:  pRec.Classification,
					costUSD:         stepCost(st),
					priceConfigured: priceConfigured,
					isSubagent:      isSub,
					agentRole:       agentRole,
				})
			}

			// Collect every in-window step for the trend histogram (#125).
			for _, st := range agg.steps {
				windowSteps = append(windowSteps, stepWithContext{
					rawStepEvent:    st,
					convID:          convID,
					project:         projName,
					modelName:       mDisplayName,
					modelID:         mID,
					provider:        pRec.Provider,
					classification:  pRec.Classification,
					costUSD:         stepCost(st),
					priceConfigured: priceConfigured,
					isSubagent:      isSub,
					agentRole:       agentRole,
				})
			}
		}
	}

	// Ensure any historical native models discovered in conversation logs are persisted in PricingRecords
	// (classified as outdated native -> "custom" so they appear in Token Price table and can be manually deleted)
	if len(discoveredHistoricalNative) > 0 {
		store.EnsureHistoricalNativeModels(r.Context(), discoveredHistoricalNative)
		// Refresh pricingRecords and internal IDs onto modelMap
		pricingRecords = store.ListPricingRecords()
		for _, rec := range pricingRecords {
			if ma, ok := modelMap[strings.ToLower(rec.ModelID)]; ok {
				ma.internalID = rec.InternalID
				ma.classification = rec.Classification
				ma.isOutdatedNative = rec.IsOutdatedNative
			}
		}
	}

	totalPromptTokens := totalFreshInput + totalCachedInput
	totalTokens := totalPromptTokens + totalOutputTokens

	// Build the stacked histogram from the real in-window steps (#125). When the
	// window contains no steps the histogram stays empty (axes + "No data") —
	// no synthetic interpolation is ever injected.
	trendBuckets, bucketMS := buildTokenTrend(windowSteps, rangeSpec, cutoff, now)

	// Build sorted model breakdowns
	modelBreakdowns := make([]TokenModelBreakdown, 0, len(modelMap))
	for _, ma := range modelMap {
		mPrompt := ma.freshInput + ma.cachedInput
		mTotal := mPrompt + ma.outputTokens
		pct := 0.0
		if totalTokens > 0 {
			pct = math.Round((float64(mTotal)/float64(totalTokens))*1000) / 10
		}
		avgTPS := 0.0
		if ma.durationMs > 0 && ma.timedOutput > 0 {
			rawTPS := float64(ma.timedOutput) / (float64(ma.durationMs) / 1000.0)
			if rawTPS > 220.0 {
				rawTPS = 220.0
			}
			avgTPS = math.Round(rawTPS*10) / 10
		}
		modelBreakdowns = append(modelBreakdowns, TokenModelBreakdown{
			InternalID:       ma.internalID,
			ModelID:          ma.modelID,
			CanonicalID:      ma.modelID,
			ModelName:        ma.modelName,
			Provider:         ma.provider,
			Classification:   ma.classification,
			IsOutdatedNative: ma.isOutdatedNative,
			InputTokens:      mPrompt,
			CachedTokens:     ma.cachedInput,
			OutputTokens:     ma.outputTokens,
			TotalTokens:      mTotal,
			CostUSD:          ma.costUSD,
			SavedUSD:         ma.savedUSD,
			PriceConfigured:  ma.priceConfigured,
			AvgTPS:           avgTPS,
			Requests:         ma.requests,
			Percentage:       pct,
		})
	}
	sort.Slice(modelBreakdowns, func(i, j int) bool {
		return modelBreakdowns[i].TotalTokens > modelBreakdowns[j].TotalTokens
	})

	// Build sorted project breakdowns
	projectBreakdowns := make([]TokenProjectBreakdown, 0, len(projectMap))
	for _, pa := range projectMap {
		pPrompt := pa.freshInput + pa.cachedInput
		pTotal := pPrompt + pa.outputTokens
		primaryModel := "—"
		var maxModelTok int64
		for mName, tok := range pa.tokensByModel {
			if tok > maxModelTok {
				maxModelTok = tok
				primaryModel = mName
			}
		}
		cacheRatio := 0.0
		if pPrompt > 0 {
			cacheRatio = math.Round((float64(pa.cachedInput)/float64(pPrompt))*1000) / 10
		}
		projectBreakdowns = append(projectBreakdowns, TokenProjectBreakdown{
			ProjectName:   pa.projectName,
			WorkspacePath: pa.workspacePath,
			TotalTokens:   pTotal,
			InputTokens:   pPrompt,
			CachedTokens:  pa.cachedInput,
			OutputTokens:  pa.outputTokens,
			CostUSD:       pa.costUSD,
			SavedUSD:      pa.savedUSD,
			CacheHitRatio: cacheRatio,
			SessionsCount: pa.sessionsCount,
			Requests:      pa.requests,
			LastActive:    formatRelativeTime(pa.lastActive),
			PrimaryModel:  primaryModel,
		})
	}
	sort.Slice(projectBreakdowns, func(i, j int) bool {
		if projectBreakdowns[i].TotalTokens != projectBreakdowns[j].TotalTokens {
			return projectBreakdowns[i].TotalTokens > projectBreakdowns[j].TotalTokens
		}
		return projectBreakdowns[i].ProjectName < projectBreakdowns[j].ProjectName
	})

	// Sort telemetry events by timestamp descending and keep top 30
	sort.Slice(allRecentSteps, func(i, j int) bool {
		return allRecentSteps[i].createdAt.After(allRecentSteps[j].createdAt)
	})
	if len(allRecentSteps) > 30 {
		allRecentSteps = allRecentSteps[:30]
	}
	telemetryEvents := make([]TokenTelemetryEvent, 0, len(allRecentSteps))
	for idx, st := range allRecentSteps {
		tps := 0.0
		if st.durationMs > 0 && st.output > 0 {
			rawTPS := float64(st.output) / (float64(st.durationMs) / 1000.0)
			if rawTPS > 220.0 {
				rawTPS = 220.0
			}
			tps = math.Round(rawTPS*10) / 10
		}
		tsStr := st.createdAt.Local().Format("2006-01-02 15:04:05")
		telemetryEvents = append(telemetryEvents, TokenTelemetryEvent{
			ID:              st.convID + "-" + strconv.Itoa(st.stepIndex) + "-" + strconv.Itoa(idx),
			Timestamp:       tsStr,
			Project:         st.project,
			Model:           st.modelName,
			ModelID:         st.modelID,
			CanonicalID:     st.modelID,
			Provider:        st.provider,
			Classification:  st.classification,
			InputTokens:     st.freshInput + st.cachedInput,
			CachedTokens:    st.cachedInput,
			OutputTokens:    st.output,
			CostUSD:         st.costUSD,
			PriceConfigured: st.priceConfigured,
			DurationMs:      st.durationMs,
			TPS:             tps,
			IsSubagent:      st.isSubagent,
			AgentRole:       st.agentRole,
		})
	}

	avgTPS := 0.0
	if totalDurationMs > 0 && totalTimedOutputTokens > 0 {
		rawTPS := float64(totalTimedOutputTokens) / (float64(totalDurationMs) / 1000.0)
		if rawTPS > 220.0 {
			rawTPS = 220.0
		}
		avgTPS = math.Round(rawTPS*10) / 10
	} else if len(telemetryEvents) > 0 {
		sum := 0.0
		count := 0
		for _, e := range telemetryEvents {
			if e.TPS > 0 {
				sum += e.TPS
				count++
			}
		}
		if count > 0 {
			rawTPS := sum / float64(count)
			if rawTPS > 220.0 {
				rawTPS = 220.0
			}
			avgTPS = math.Round(rawTPS*10) / 10
		}
	}

	writeJSON(w, map[string]interface{}{
		"range": rangeSpec.Key,
		// Token accounting definition (#126), verified against live transcripts:
		// total_tokens = fresh input + cache reads + output.
		//
		// `cache_read_tokens` is NOT a subset of `input_tokens` in Antigravity
		// transcripts: input_tokens is the *fresh* prompt slice delivered to the
		// model while cache_read_tokens is the cache-hit slice of the same
		// context, so total prompt size is input_tokens + cache_read_tokens
		// (measured cache/fresh ratios of 20-25x on real transcripts). The three
		// fields are therefore disjoint and must be summed, never nested.
		//
		// Cross-check with the external "Magpie" readout, which reports a single
		// conversation rather than a global window. Conversation
		// 60577173-d663-4a89-8482-c0afc2b1fe79: fresh 353,814 + cache 7,904,545
		// + output 49,288 = 8,307,647 tokens over 92 requests with a 95.7% cache
		// hit ratio, matching Magpie's reported 8.1M tokens / 92 requests / 95%
		// hit — i.e. Magpie's "Total Tokens" also contains cache reads (without
		// them this conversation would read ~0.4M). The all-time dashboard total
		// (e.g. 1573.63M) is the honest sum over every conversation, so it only
		// becomes comparable once a bounded range is selected — which is exactly
		// what the range filter above enables.
		//
		// No double counting across dimensions: each token-bearing step is
		// counted exactly once (verified: 14,287 live steps, 0 signatures shared
		// between two conversation files, so parent/subagent transcripts do not
		// duplicate work), and every conversation is attributed to exactly one
		// model, so summing model_breakdowns (native + custom) reproduces
		// total_tokens exactly once. `input_tokens` below is the prompt side
		// (fresh + cached) to keep the I/O ratio and cache-hit ratio meaningful,
		// and `cached_input_tokens` is its cache-read component.
		"total_tokens":        totalTokens,
		"input_tokens":        totalPromptTokens,
		"cached_input_tokens": totalCachedInput,
		"output_tokens":       totalOutputTokens,
		"total_cost_usd":      totalCostUSD,
		"input_cost_usd":      totalInputCostUSD,
		"output_cost_usd":     totalOutputCostUSD,
		"saved_cost_usd":      totalSavedUSD,
		"avg_tps":             avgTPS,
		"requests_count":      totalRequests,
		"conversations_count": totalConversations,
		"model_breakdowns":    modelBreakdowns,
		"project_breakdowns":  projectBreakdowns,
		"telemetry_events":    telemetryEvents,
		"trend": map[string]interface{}{
			"bucket_ms": bucketMS,
			"buckets":   trendBuckets,
		},
		"pricing_records": pricingRecords,
		"models":          pricingRecords,
	})
}

func (s *Server) handleTokensPricing(w http.ResponseWriter, r *http.Request) {
	store, err := s.ensureCustomModelsStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		force := r.URL.Query().Get("refresh") == "true" || r.URL.Query().Get("force") == "true"
		records := s.syncTokenPricing(r.Context(), force)
		writeJSON(w, map[string]interface{}{
			"success":         true,
			"updated_at":      time.Now().Format("2006-01-02 15:04"),
			"source":          "Provider First, 3rd-Party Backup (OpenRouter / LiteLLM)",
			"pricing_records": records,
			"models":          records,
		})

	case http.MethodPost:
		var req custommodels.UpdatePricingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		// Ensure catalog sync has populated records before updating
		if len(store.ListPricingRecords()) == 0 {
			s.syncTokenPricing(r.Context(), false)
		}
		updated, err := store.UpdateModelPricing(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success":         true,
			"updated_at":      updated.UpdatedAt,
			"model":           updated,
			"pricing_record":  updated,
			"models":          store.ListPricingRecords(),
			"pricing_records": store.ListPricingRecords(),
		})

	case http.MethodDelete:
		internalIDStr := r.URL.Query().Get("internal_id")
		modelID := r.URL.Query().Get("model_id")
		if modelID == "" {
			modelID = r.URL.Query().Get("canonical_id")
		}
		var internalID int64
		if internalIDStr != "" {
			internalID, _ = strconv.ParseInt(internalIDStr, 10, 64)
		}
		if internalID == 0 && modelID == "" {
			var body struct {
				InternalID  int64  `json:"internal_id"`
				ModelID     string `json:"model_id"`
				CanonicalID string `json:"canonical_id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			internalID = body.InternalID
			modelID = body.ModelID
			if modelID == "" {
				modelID = body.CanonicalID
			}
		}
		if internalID == 0 && strings.TrimSpace(modelID) == "" {
			http.Error(w, "missing internal_id or model_id", http.StatusBadRequest)
			return
		}
		if err := store.DeletePricingModel(internalID, modelID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success":         true,
			"models":          store.ListPricingRecords(),
			"pricing_records": store.ListPricingRecords(),
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ChatMetricsTurn holds real step metrics extracted from transcript.jsonl.
type ChatMetricsTurn struct {
	StepIndex     int     `json:"step_index"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	CachedTokens  int64   `json:"cached_tokens"`
	ContextTokens int64   `json:"context_tokens,omitempty"`
	CacheHitRatio float64 `json:"cache_hit_ratio"`
	SpeedTPS      float64 `json:"speed_tps"`
}

// ChatMetricsSummary aggregates overall conversation tokens and performance.
type ChatMetricsSummary struct {
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CachedTokens    int64   `json:"cached_tokens"`
	ContextTokens   int64   `json:"context_tokens,omitempty"`
	CacheHitRatio   float64 `json:"cache_hit_ratio"`
	GenerationSpeed float64 `json:"generation_speed"`
}

// chatPromptTextLimit bounds one prompt's navigator label. The rail only shows a
// single ellipsized line, so a longer body buys nothing and the whole prompt
// list travels on every metrics poll.
const chatPromptTextLimit = 400

var (
	chatPromptRequestBlock = regexp.MustCompile(`(?is)<USER_REQUEST>(.*?)</USER_REQUEST>`)
	chatPromptMetaBlock    = regexp.MustCompile(`(?is)<ADDITIONAL_METADATA>.*?</ADDITIONAL_METADATA>`)
	chatPromptWhitespace   = regexp.MustCompile(`\s+`)
)

// ExtractUserPromptText turns one transcript user-input payload into a single
// line of prompt text. The transcript wraps the user's words in
// <USER_REQUEST> and appends an <ADDITIONAL_METADATA> tail with host details.
func ExtractUserPromptText(content string) string {
	text := content
	if match := chatPromptRequestBlock.FindStringSubmatch(text); len(match) == 2 {
		text = match[1]
	} else {
		text = strings.NewReplacer("<USER_REQUEST>", "", "</USER_REQUEST>", "").Replace(text)
	}
	text = chatPromptMetaBlock.ReplaceAllString(text, " ")
	text = chatPromptWhitespace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	if len(text) > chatPromptTextLimit {
		// Cut on a rune boundary so a multi-byte character is never split.
		cut := chatPromptTextLimit
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		text = strings.TrimSpace(text[:cut]) + "…"
	}
	return text
}

// ChatMetricsPrompt is one user prompt of the transcript, in conversation order.
// The chat navigator builds its rail from these, because the virtualization in
// the transcript DOM only ever mounts the turns around the viewport.
type ChatMetricsPrompt struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// ChatMetricsResponse is returned by /api/tokens/chat-metrics.
type ChatMetricsResponse struct {
	Success         bool                `json:"success"`
	ConversationID  string              `json:"conversation_id"`
	Scope           string              `json:"scope"`
	UserPromptCount int                 `json:"user_prompt_count"`
	Prompts         []ChatMetricsPrompt `json:"prompts"`
	Summary         ChatMetricsSummary  `json:"summary"`
	Turns           []ChatMetricsTurn   `json:"turns"`
}

// isSafeConversationID validates conversation identifiers against directory traversal and illegal characters.
func isSafeConversationID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 256 {
		return false
	}
	if filepath.Base(id) != id || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return false
	}
	for _, ch := range id {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}

// GetChatMetricsForConversation extracts real per-turn and conversation-level metrics from disk.
func GetChatMetricsForConversation(convID string, scope string) (*ChatMetricsResponse, error) {
	if convID == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}
	if !isSafeConversationID(convID) {
		return nil, fmt.Errorf("invalid conversation_id: contains forbidden characters or path traversal")
	}
	if scope == "" {
		scope = "aggregated"
	}

	home, _ := os.UserHomeDir()
	brainBase := filepath.Join(home, ".gemini", "antigravity", "brain")
	mainLog := filepath.Join(brainBase, convID, ".system_generated", "logs", "transcript.jsonl")

	const maxTranscriptSize = 10 * 1024 * 1024 // 10MB upper bound ceiling
	fi, err := os.Stat(mainLog)
	if err != nil {
		return nil, fmt.Errorf("failed to read conversation transcript: %w", err)
	}
	if fi.Size() > maxTranscriptSize {
		return nil, fmt.Errorf("conversation transcript exceeds maximum size limit of 10MB")
	}

	data, err := os.ReadFile(mainLog)
	if err != nil {
		return nil, fmt.Errorf("failed to read conversation transcript: %w", err)
	}

	var turns []ChatMetricsTurn
	var prompts []ChatMetricsPrompt
	var totalIn, totalOut, totalCached int64
	var totalDurationMs, totalTimedOut int64
	userPromptCount := 0

	lines := strings.Split(string(data), "\n")
	type jsonStep struct {
		StepIndex        int    `json:"step_index"`
		Source           string `json:"source"`
		Type             string `json:"type"`
		Content          string `json:"content"`
		InputTokens      int64  `json:"input_tokens"`
		CacheReadTokens  int64  `json:"cache_read_tokens"`
		OutputTokens     int64  `json:"output_tokens"`
		DurationMs       int64  `json:"duration_ms"`
		ThinkingDuration string `json:"thinking_duration"`
		CreatedAt        string `json:"created_at"`
	}

	var parsedSteps []jsonStep
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		var st jsonStep
		if err := json.Unmarshal([]byte(l), &st); err != nil {
			continue
		}
		if st.Source == "USER_EXPLICIT" || st.Type == "USER_INPUT" || strings.Contains(st.Source, "USER") {
			prompts = append(prompts, ChatMetricsPrompt{Index: userPromptCount, Text: ExtractUserPromptText(st.Content)})
			userPromptCount++
		}
		parsedSteps = append(parsedSteps, st)
	}

	type turnAcc struct {
		startStepIndex int
		inputTokens    int64
		cachedTokens   int64
		outputTokens   int64
		contextTokens  int64
		durationMs     int64
		stepCount      int
	}

	var curTurn turnAcc
	flushTurn := func() {
		if curTurn.stepCount == 0 {
			return
		}
		turnInput := curTurn.inputTokens
		hitRatio := 0.0
		if turnInput > 0 {
			hitRatio = math.Round((float64(curTurn.cachedTokens)/float64(turnInput)*100.0)*10) / 10
		}
		tps := 0.0
		if curTurn.durationMs > 0 && curTurn.outputTokens > 0 {
			rawTPS := float64(curTurn.outputTokens) / (float64(curTurn.durationMs) / 1000.0)
			if rawTPS > 220.0 {
				rawTPS = 220.0
			}
			tps = math.Round(rawTPS*10) / 10
		}
		turns = append(turns, ChatMetricsTurn{
			StepIndex:     curTurn.startStepIndex,
			InputTokens:   turnInput,
			OutputTokens:  curTurn.outputTokens,
			CachedTokens:  curTurn.cachedTokens,
			ContextTokens: curTurn.contextTokens,
			CacheHitRatio: hitRatio,
			SpeedTPS:      tps,
		})
		curTurn = turnAcc{}
	}

	var lastCreated time.Time
	for i := 0; i < len(parsedSteps); i++ {
		st := parsedSteps[i]
		curTime, _ := time.Parse(time.RFC3339Nano, st.CreatedAt)
		if curTime.IsZero() {
			curTime, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", st.CreatedAt)
		}
		if curTime.IsZero() {
			curTime, _ = time.Parse(time.RFC3339, st.CreatedAt)
		}

		// Turn boundary: User messages or external turn initiators finalize the prior assistant turn
		if st.Source == "USER_EXPLICIT" || st.Type == "USER_INPUT" || strings.Contains(st.Source, "USER") {
			flushTurn()
			if !curTime.IsZero() {
				lastCreated = curTime
			}
			continue
		}
		if (st.Source == "SYSTEM" || st.Source == "SYSTEM_SDK") && st.Type == "SYSTEM_MESSAGE" && curTurn.stepCount > 0 {
			flushTurn()
			if !curTime.IsZero() {
				lastCreated = curTime
			}
			continue
		}

		if st.Source == "MODEL" && st.Type == "PLANNER_RESPONSE" {
			inFresh := st.InputTokens
			cached := st.CacheReadTokens
			out := st.OutputTokens
			stepInput := inFresh + cached

			durMs := st.DurationMs
			if durMs <= 0 && st.ThinkingDuration != "" {
				if d, parseErr := time.ParseDuration(st.ThinkingDuration); parseErr == nil {
					durMs = d.Milliseconds()
				}
			}
			// 1. Prior-step interval (step-completion timestamping convention)
			if durMs <= 0 && !lastCreated.IsZero() && !curTime.IsZero() && curTime.After(lastCreated) {
				diff := curTime.Sub(lastCreated).Milliseconds()
				if diff >= 200 && diff <= 600000 {
					durMs = diff
				}
			}
			// 2. Next-step lookahead interval (step-start timestamping convention)
			if durMs <= 0 && !curTime.IsZero() && i+1 < len(parsedSteps) {
				nextSt := parsedSteps[i+1]
				nextTime, _ := time.Parse(time.RFC3339Nano, nextSt.CreatedAt)
				if nextTime.IsZero() {
					nextTime, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", nextSt.CreatedAt)
				}
				if nextTime.IsZero() {
					nextTime, _ = time.Parse(time.RFC3339, nextSt.CreatedAt)
				}
				if !nextTime.IsZero() && nextTime.After(curTime) {
					diffNext := nextTime.Sub(curTime).Milliseconds()
					isNextUser := nextSt.Source == "USER_EXPLICIT" || nextSt.Type == "USER_INPUT" || strings.Contains(nextSt.Source, "USER")
					if !isNextUser {
						// Autonomous intermediate step (tool execution)
						if diffNext >= 200 && diffNext <= 600000 {
							durMs = diffNext
						}
					} else {
						// Final step followed by user response: count if within 2 minutes and plausible speed
						if diffNext >= 200 && diffNext <= 120000 && out > 0 {
							impliedTPS := float64(out) / (float64(diffNext) / 1000.0)
							if impliedTPS >= 3.0 && impliedTPS <= 220.0 {
								durMs = diffNext
							}
						}
					}
				}
			}

			if curTurn.stepCount == 0 {
				curTurn.startStepIndex = st.StepIndex
			}
			// Prompt tokens of the turn represent the active prompt context of the latest step in this turn
			curTurn.inputTokens = stepInput
			curTurn.cachedTokens = cached
			curTurn.outputTokens += out
			curTurn.contextTokens = stepInput + out
			curTurn.stepCount++
			if durMs > 0 {
				curTurn.durationMs += durMs
			}

			if durMs > 0 && out > 0 {
				totalDurationMs += durMs
				totalTimedOut += out
			}

			totalIn += stepInput
			totalOut += out
			totalCached += cached
		}

		if !curTime.IsZero() {
			lastCreated = curTime
		}
	}
	flushTurn()

	// If aggregated scope is requested, accumulate subagents
	if strings.EqualFold(scope, "aggregated") {
		dbPath := filepath.Join(home, ".gemini", "antigravity", "conversation_summaries.db")
		if db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath)); err == nil {
			defer db.Close()
			rows, qErr := db.Query("SELECT conversation_id FROM conversation_summaries WHERE parent_conversation_id = ?", convID)
			if qErr == nil {
				defer rows.Close()
				for rows.Next() {
					var subID string
					if err := rows.Scan(&subID); err == nil && subID != "" {
						subID = strings.TrimSpace(subID)
						if !isSafeConversationID(subID) {
							continue
						}
						subLog := filepath.Join(brainBase, subID, ".system_generated", "logs", "transcript.jsonl")
						if sfi, sErr := os.Stat(subLog); sErr == nil && sfi.Size() <= maxTranscriptSize {
							if subData, readErr := os.ReadFile(subLog); readErr == nil {
								subLines := strings.Split(string(subData), "\n")
								for _, sl := range subLines {
									sl = strings.TrimSpace(sl)
									if sl == "" {
										continue
									}
									var sStep jsonStep
									if err := json.Unmarshal([]byte(sl), &sStep); err == nil {
										if sStep.Source == "MODEL" && sStep.Type == "PLANNER_RESPONSE" {
											subIn := sStep.InputTokens + sStep.CacheReadTokens
											totalIn += subIn
											totalOut += sStep.OutputTokens
											totalCached += sStep.CacheReadTokens
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	overallHitRatio := 0.0
	if totalIn > 0 {
		overallHitRatio = math.Round((float64(totalCached)/float64(totalIn)*100.0)*10) / 10
	}

	overallSpeed := 0.0
	if totalDurationMs > 0 && totalTimedOut > 0 {
		rawSpeed := float64(totalTimedOut) / (float64(totalDurationMs) / 1000.0)
		if rawSpeed > 220.0 {
			rawSpeed = 220.0
		}
		overallSpeed = math.Round(rawSpeed*10) / 10
	} else if len(turns) > 0 {
		sum := 0.0
		count := 0
		for _, t := range turns {
			if t.SpeedTPS > 0 {
				sum += t.SpeedTPS
				count++
			}
		}
		if count > 0 {
			rawSpeed := sum / float64(count)
			if rawSpeed > 220.0 {
				rawSpeed = 220.0
			}
			overallSpeed = math.Round(rawSpeed*10) / 10
		}
	}

	var latestContext int64
	if len(turns) > 0 {
		latestContext = turns[len(turns)-1].ContextTokens
	}

	return &ChatMetricsResponse{
		Success:         true,
		ConversationID:  convID,
		Scope:           scope,
		UserPromptCount: userPromptCount,
		Prompts:         prompts,
		Summary: ChatMetricsSummary{
			InputTokens:     totalIn,
			OutputTokens:    totalOut,
			CachedTokens:    totalCached,
			ContextTokens:   latestContext,
			CacheHitRatio:   overallHitRatio,
			GenerationSpeed: overallSpeed,
		},
		Turns: turns,
	}, nil
}

func (s *Server) handleTokensChatMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	convID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if convID == "" {
		convID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if convID != "" && !isSafeConversationID(convID) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   "invalid conversation_id: contains forbidden characters or path traversal",
		})
		return
	}
	parentID := strings.TrimSpace(r.URL.Query().Get("parent_id"))
	if parentID == "" {
		parentID = strings.TrimSpace(r.URL.Query().Get("parent_conversation_id"))
	}
	if parentID != "" && !isSafeConversationID(parentID) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   "invalid parent_id: contains forbidden characters or path traversal",
		})
		return
	}
	agentName := strings.TrimSpace(r.URL.Query().Get("agent_name"))
	if convID == "" && parentID != "" && agentName != "" {
		home, _ := os.UserHomeDir()
		dbPath := filepath.Join(home, ".gemini", "antigravity", "conversation_summaries.db")
		if db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath)); err == nil {
			defer db.Close()
			var resolvedID string
			cteQuery := `
WITH RECURSIVE descendants(cid) AS (
    SELECT conversation_id FROM conversation_summaries WHERE parent_conversation_id = ?
    UNION ALL
    SELECT cs.conversation_id FROM conversation_summaries cs JOIN descendants d ON cs.parent_conversation_id = d.cid
)
SELECT cs.conversation_id FROM conversation_summaries cs
WHERE cs.conversation_id IN (SELECT cid FROM descendants)
  AND (cs.agent_name = ? OR cs.title LIKE ?)
ORDER BY cs.last_modified_time DESC LIMIT 1`
			row := db.QueryRow(cteQuery, parentID, agentName, "%"+agentName+"%")
			if err := row.Scan(&resolvedID); err == nil && resolvedID != "" {
				convID = resolvedID
			}
		}
	}
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope == "" {
		if parentID != "" && agentName != "" {
			scope = "conversation"
		} else if s.guiStore != nil {
			cfg := s.guiStore.GetConfig()
			if cfg.ChatTelemetryScope != "" {
				scope = cfg.ChatTelemetryScope
			}
		}
		if scope == "" {
			scope = "aggregated"
		}
	}

	res, err := GetChatMetricsForConversation(convID, scope)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"success":           false,
			"error":             err.Error(),
			"conversation_id":   convID,
			"scope":             scope,
			"user_prompt_count": 0,
			"prompts":           []ChatMetricsPrompt{},
			"summary": ChatMetricsSummary{
				InputTokens:     0,
				OutputTokens:    0,
				CachedTokens:    0,
				CacheHitRatio:   0,
				GenerationSpeed: 0.0,
			},
			"turns": []ChatMetricsTurn{},
		})
		return
	}
	writeJSON(w, res)
}

// computeModelLifetimeCosts computes total USD cost incurred by each model across all transcripts.
func (s *Server) computeModelLifetimeCosts(ctx context.Context) map[string]float64 {
	costs := make(map[string]float64)
	store := s.getCustomModelsStore()
	if store == nil {
		return costs
	}

	catalogModels := s.fetchAvailableCatalogModels(false)
	pricingRecords := store.SyncPricingWithCatalog(ctx, catalogModels, false)
	customModelsList := store.ListModels()

	pricingByModel := make(map[string]custommodels.ModelPricingRecord, len(pricingRecords))
	for _, rec := range pricingRecords {
		pricingByModel[strings.ToLower(rec.ModelID)] = rec
	}
	for _, cm := range customModelsList {
		if cm.InputPricePerM != nil || cm.OutputPricePerM != nil {
			rec := custommodels.ModelPricingRecord{
				ModelID:              cm.ID,
				Name:                 cm.Name,
				Provider:             string(cm.ProviderType),
				InputPricePerM:       cm.InputPricePerM,
				CachedInputPricePerM: cm.CachedInputPricePerM,
				OutputPricePerM:      cm.OutputPricePerM,
			}
			pricingByModel[strings.ToLower(cm.ID)] = rec
			pricingByModel[strings.ToLower(cm.Name)] = rec
		}
	}

	dataDir := s.getAntigravityDataDir()
	brainDir := filepath.Join(dataDir, "brain")
	convDir := filepath.Join(dataDir, "conversations")
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return costs
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		convID := entry.Name()
		tPath := filepath.Join(brainDir, convID, ".system_generated", "logs", "transcript.jsonl")
		info, err := os.Stat(tPath)
		if err != nil || info.IsDir() {
			continue
		}

		tStats := parseTranscriptFile(tPath, info)
		if tStats.requests == 0 || (tStats.freshInput == 0 && tStats.cachedInput == 0 && tStats.outputTokens == 0) {
			continue
		}

		convDBPath := filepath.Join(convDir, convID+".db")
		mID, mDisplayName, _ := detectConversationModelWithTranscript(tPath, convDBPath, customModelsList, catalogModels)
		if mID == "" {
			continue
		}

		pRec, hasRec := pricingByModel[strings.ToLower(mID)]
		if !hasRec {
			pRec, hasRec = pricingByModel[strings.ToLower(mDisplayName)]
		}
		if !hasRec || pRec.InputPricePerM == nil || pRec.OutputPricePerM == nil {
			continue
		}

		inRate := *pRec.InputPricePerM
		cacheRate := inRate * 0.25
		if pRec.CachedInputPricePerM != nil {
			cacheRate = *pRec.CachedInputPricePerM
		}
		outRate := *pRec.OutputPricePerM

		cost := (float64(tStats.freshInput)*inRate + float64(tStats.cachedInput)*cacheRate + float64(tStats.outputTokens)*outRate) / 1_000_000.0
		costs[strings.ToLower(mID)] += cost
		if mDisplayName != "" && strings.ToLower(mDisplayName) != strings.ToLower(mID) {
			costs[strings.ToLower(mDisplayName)] += cost
		}
	}

	return costs
}

// reconcileCustomModelBalances reconciles prepaid balances against cumulative token usage.
func (s *Server) reconcileCustomModelBalances(ctx context.Context) {
	store := s.getCustomModelsStore()
	if store == nil {
		return
	}
	cfg := store.GetConfig()
	hasBudget := false
	for i := range cfg.Models {
		m := &cfg.Models[i]
		if len(m.QuotaBuckets) > 0 {
			continue
		}
		if m.TotalBudget > 0 || m.PrepaidBalance > 0 {
			hasBudget = true
			break
		}
		if m.BalanceValue != "" {
			clean := strings.TrimLeft(strings.TrimSpace(m.BalanceValue), "$¥€£")
			if val, err := strconv.ParseFloat(clean, 64); err == nil && val > 0 {
				m.TotalBudget = val
				m.PrepaidBalance = val
				frac := 1.0
				m.QuotaFraction = &frac
				hasBudget = true
			}
		}
	}
	if !hasBudget {
		return
	}

	costs := s.computeModelLifetimeCosts(ctx)
	for i := range cfg.Models {
		m := &cfg.Models[i]
		if len(m.QuotaBuckets) > 0 {
			continue
		}
		if m.TotalBudget <= 0 && m.PrepaidBalance > 0 {
			m.TotalBudget = m.PrepaidBalance
		}
		if m.TotalBudget <= 0 && m.BalanceValue != "" {
			clean := strings.TrimLeft(strings.TrimSpace(m.BalanceValue), "$¥€£")
			if val, err := strconv.ParseFloat(clean, 64); err == nil && val > 0 {
				m.TotalBudget = val
				m.PrepaidBalance = val
			}
		}
		if m.TotalBudget > 0 {
			cost := costs[strings.ToLower(m.ID)]
			if nameCost, ok := costs[strings.ToLower(m.Name)]; ok && nameCost > cost {
				cost = nameCost
			}
			remaining := m.TotalBudget - cost
			if remaining < 0 {
				remaining = 0
			}
			m.PrepaidBalance = remaining
			frac := remaining / m.TotalBudget
			if frac < 0 {
				frac = 0
			}
			if frac > 1 {
				frac = 1
			}
			m.QuotaFraction = &frac
			m.BalanceValue = fmt.Sprintf("$%.2f", remaining)
			m.QuotaValue = m.BalanceValue
			_ = store.SaveModel(*m)
		}
	}
}
