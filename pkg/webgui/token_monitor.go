package webgui

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
	PriceConfigured  bool                             `json:"price_configured"`
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
	SessionsCount int     `json:"sessions_count"`
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
}

type cachedTranscriptStats struct {
	modTimeNano     int64
	size            int64
	freshInput      int64
	cachedInput     int64
	outputTokens    int64
	requests        int
	totalDurationMs int64
	lastActive      time.Time
	recentSteps     []rawStepEvent
}

type rawStepEvent struct {
	stepIndex   int
	createdAt   time.Time
	freshInput  int64
	cachedInput int64
	output      int64
	durationMs  int64
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
	{marker: "claude-opus-4-6-thinking", modelID: "claude-opus-4-6", displayName: "Claude Opus 4.6", provider: "anthropic"},
	{marker: "claude-opus-4-6", modelID: "claude-opus-4-6", displayName: "Claude Opus 4.6", provider: "anthropic"},
	{marker: "claude-sonnet-4-6", modelID: "claude-sonnet-4-6", displayName: "Claude Sonnet 4.6", provider: "anthropic"},
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

	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, []byte("input_tokens")) && !bytes.Contains(line, []byte("output_tokens")) {
			continue
		}
		var entry struct {
			Type             string `json:"type"`
			StepIndex        int    `json:"step_index"`
			CreatedAt        string `json:"created_at"`
			ThinkingDuration string `json:"thinking_Duration"`
			InputTokens      int64  `json:"input_tokens"`
			OutputTokens     int64  `json:"output_tokens"`
			CacheReadTokens  int64  `json:"cache_read_tokens"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.InputTokens <= 0 && entry.OutputTokens <= 0 && entry.CacheReadTokens <= 0 {
			continue
		}

		stats.freshInput += entry.InputTokens
		stats.cachedInput += entry.CacheReadTokens
		stats.outputTokens += entry.OutputTokens
		stats.requests++

		var durMs int64
		if entry.ThinkingDuration != "" {
			if d, err := time.ParseDuration(entry.ThinkingDuration); err == nil && d > 0 {
				durMs = d.Milliseconds()
				stats.totalDurationMs += durMs
			}
		}

		stepTime := info.ModTime()
		if entry.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, entry.CreatedAt); err == nil {
				stepTime = t
				if t.After(stats.lastActive) {
					stats.lastActive = t
				}
			}
		}

		stats.recentSteps = append(stats.recentSteps, rawStepEvent{
			stepIndex:   entry.StepIndex,
			createdAt:   stepTime,
			freshInput:  entry.InputTokens,
			cachedInput: entry.CacheReadTokens,
			output:      entry.OutputTokens,
			durationMs:  durMs,
		})
		if len(stats.recentSteps) > 15 {
			stats.recentSteps = stats.recentSteps[len(stats.recentSteps)-15:]
		}
	}

	tokenCacheMu.Lock()
	transcriptCacheMap[tPath] = stats
	tokenCacheMu.Unlock()
	return stats
}

func detectConversationModel(convDBPath string, customModels []custommodels.CustomModel, catalogModels []custommodels.CatalogModelInput) (modelID, displayName, provider string) {
	info, err := os.Stat(convDBPath)
	if err != nil || info.IsDir() {
		return "", "", ""
	}

	tokenCacheMu.RLock()
	if cached, ok := convModelCacheMap[convDBPath]; ok && cached.modTimeNano == info.ModTime().UnixNano() && cached.size == info.Size() {
		tokenCacheMu.RUnlock()
		return cached.modelID, cached.displayName, cached.provider
	}
	tokenCacheMu.RUnlock()

	f, err := os.Open(convDBPath)
	if err != nil {
		return "", "", ""
	}
	defer f.Close()

	// Read up to the first 512KB (and last 128KB if larger) where gen_metadata model_enum records live
	maxHead := int64(512 * 1024)
	readLen := info.Size()
	if readLen > maxHead {
		readLen = maxHead
	}
	data := make([]byte, readLen)
	n, _ := f.Read(data)
	data = data[:n]

	if info.Size() > maxHead {
		tailLen := int64(128 * 1024)
		if info.Size()-maxHead < tailLen {
			tailLen = info.Size() - maxHead
		}
		tailBuf := make([]byte, tailLen)
		if tn, err := f.ReadAt(tailBuf, info.Size()-tailLen); err == nil && tn > 0 {
			data = append(data, tailBuf[:tn]...)
		}
	}

	// 1. Check user-configured Custom Models first if their ID or Name appears in the conversation DB
	for _, cm := range customModels {
		if cm.Name != "" && bytes.Contains(data, []byte(cm.Name)) {
			disp := cm.DisplayName
			if disp == "" {
				disp = cm.Name
			}
			prov := custommodels.NormalizePricingProvider(string(cm.ProviderType), cm.Name)
			res := cachedConvModel{modTimeNano: info.ModTime().UnixNano(), size: info.Size(), modelID: cm.Name, displayName: disp, provider: prov}
			tokenCacheMu.Lock()
			convModelCacheMap[convDBPath] = res
			tokenCacheMu.Unlock()
			return res.modelID, res.displayName, res.provider
		}
	}

	// 2. Check known Antigravity model_enum placeholders and explicit model IDs, picking the one with the latest occurrence in the file
	bestPos := -1
	var bestID, bestName, bestProv string
	for _, entry := range knownNativePlaceholderMap {
		if idx := bytes.LastIndex(data, []byte(entry.marker)); idx > bestPos {
			bestPos = idx
			bestID = entry.modelID
			bestName = entry.displayName
			bestProv = entry.provider
		}
	}

	// 3. Also check any dynamic models from the live catalog
	for _, cm := range catalogModels {
		if cm.ID != "" {
			if idx := bytes.LastIndex(data, []byte(cm.ID)); idx > bestPos {
				bestPos = idx
				canID, canName := custommodels.NormalizeCanonicalNativeModel(cm.ID, cm.DisplayName)
				bestID = canID
				bestName = canName
				if bestName == "" {
					bestName = bestID
				}
				bestProv = custommodels.NormalizePricingProvider(cm.Provider, bestID)
			}
		}
	}

	if bestID != "" {
		res := cachedConvModel{
			modTimeNano: info.ModTime().UnixNano(),
			size:        info.Size(),
			modelID:     bestID,
			displayName: bestName,
			provider:    bestProv,
		}
		tokenCacheMu.Lock()
		convModelCacheMap[convDBPath] = res
		tokenCacheMu.Unlock()
		return res.modelID, res.displayName, res.provider
	}

	return "", "", ""
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

func readConversationWorkspaceMap(summariesDB string) map[string][]string {
	out := make(map[string][]string)
	if _, err := os.Stat(summariesDB); err != nil {
		return out
	}
	db, err := sql.Open("sqlite", summariesDB+"?mode=ro&_busy_timeout=2000")
	if err != nil {
		return out
	}
	defer db.Close()

	rows, err := db.Query("SELECT conversation_id, COALESCE(workspace_uris, '') FROM conversation_summaries")
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var convID, urisJSON string
		if err := rows.Scan(&convID, &urisJSON); err != nil || convID == "" {
			continue
		}
		var uris []string
		if urisJSON != "" {
			if err := json.Unmarshal([]byte(urisJSON), &uris); err != nil {
				uris = []string{urisJSON}
			}
		}
		out[convID] = uris
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

func (s *Server) handleTokensSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	workspaceMap := readConversationWorkspaceMap(summariesDB)

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
		priceConfigured  bool
	}

	type projectAgg struct {
		projectName   string
		workspacePath string
		freshInput    int64
		cachedInput   int64
		outputTokens  int64
		costUSD       float64
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
	var totalSavedUSD float64
	var totalRequests int
	var totalConversations int
	var totalDurationMs int64
	var totalTimedOutputTokens int64

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
	}
	var allRecentSteps []stepWithContext

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

			// Resolve project workspace from conversation_summaries.db
			uris := workspaceMap[convID]
			projName, projPath := resolveProjectFromURI(uris)

			// Resolve actual model from conversations/<convID>.db
			convDBPath := filepath.Join(convDir, convID+".db")
			mID, mDisplayName, mProv := detectConversationModel(convDBPath, customModelsList, catalogModels)
			if mID == "" {
				// Check if project has a bound custom model
				if bound := store.GetModelForProject(projName); bound != nil {
					mID = bound.Name
					mDisplayName = bound.DisplayName
					if mDisplayName == "" {
						mDisplayName = bound.Name
					}
					mProv = custommodels.NormalizePricingProvider(string(bound.ProviderType), bound.Name)
				} else if len(catalogModels) > 0 {
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

			priceConfigured := pRec.InputPricePerM != nil && pRec.OutputPricePerM != nil
			var convCost, convSaved float64
			if priceConfigured {
				inRate := *pRec.InputPricePerM
				cacheRate := inRate * 0.25
				if pRec.CachedInputPricePerM != nil {
					cacheRate = *pRec.CachedInputPricePerM
				}
				outRate := *pRec.OutputPricePerM
				convCost = (float64(tStats.freshInput)*inRate + float64(tStats.cachedInput)*cacheRate + float64(tStats.outputTokens)*outRate) / 1_000_000.0
				if inRate > cacheRate {
					convSaved = float64(tStats.cachedInput) * (inRate - cacheRate) / 1_000_000.0
				}
			}

			totalConversations++
			totalFreshInput += tStats.freshInput
			totalCachedInput += tStats.cachedInput
			totalOutputTokens += tStats.outputTokens
			totalCostUSD += convCost
			totalSavedUSD += convSaved
			totalRequests += tStats.requests
			if tStats.totalDurationMs > 0 {
				totalDurationMs += tStats.totalDurationMs
				totalTimedOutputTokens += tStats.outputTokens
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
			ma.freshInput += tStats.freshInput
			ma.cachedInput += tStats.cachedInput
			ma.outputTokens += tStats.outputTokens
			ma.requests += tStats.requests
			ma.costUSD += convCost
			ma.savedUSD += convSaved

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
			convTotalTokens := tStats.freshInput + tStats.cachedInput + tStats.outputTokens
			pa.freshInput += tStats.freshInput
			pa.cachedInput += tStats.cachedInput
			pa.outputTokens += tStats.outputTokens
			pa.costUSD += convCost
			pa.sessionsCount++
			pa.tokensByModel[mDisplayName] += convTotalTokens
			if tStats.lastActive.After(pa.lastActive) {
				pa.lastActive = tStats.lastActive
			}

			// Collect recent steps for Live Telemetry tab
			for _, st := range tStats.recentSteps {
				var stepCost float64
				if priceConfigured {
					inRate := *pRec.InputPricePerM
					cacheRate := inRate * 0.25
					if pRec.CachedInputPricePerM != nil {
						cacheRate = *pRec.CachedInputPricePerM
					}
					outRate := *pRec.OutputPricePerM
					stepCost = (float64(st.freshInput)*inRate + float64(st.cachedInput)*cacheRate + float64(st.output)*outRate) / 1_000_000.0
				}
				allRecentSteps = append(allRecentSteps, stepWithContext{
					rawStepEvent:    st,
					convID:          convID,
					project:         projName,
					modelName:       mDisplayName,
					modelID:         mID,
					provider:        pRec.Provider,
					classification:  pRec.Classification,
					costUSD:         stepCost,
					priceConfigured: priceConfigured,
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

	// Build sorted model breakdowns
	modelBreakdowns := make([]TokenModelBreakdown, 0, len(modelMap))
	for _, ma := range modelMap {
		mPrompt := ma.freshInput + ma.cachedInput
		mTotal := mPrompt + ma.outputTokens
		pct := 0.0
		if totalTokens > 0 {
			pct = math.Round((float64(mTotal)/float64(totalTokens))*1000) / 10
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
			PriceConfigured:  ma.priceConfigured,
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
		if primaryModel == "—" {
			if bound := store.GetModelForProject(pa.projectName); bound != nil {
				if bound.DisplayName != "" {
					primaryModel = bound.DisplayName
				} else {
					primaryModel = bound.Name
				}
			}
		}
		projectBreakdowns = append(projectBreakdowns, TokenProjectBreakdown{
			ProjectName:   pa.projectName,
			WorkspacePath: pa.workspacePath,
			TotalTokens:   pTotal,
			InputTokens:   pPrompt,
			CachedTokens:  pa.cachedInput,
			OutputTokens:  pa.outputTokens,
			CostUSD:       pa.costUSD,
			SessionsCount: pa.sessionsCount,
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
			tps = math.Round((float64(st.output)/(float64(st.durationMs)/1000.0))*10) / 10
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
		})
	}

	avgTPS := 0.0
	if totalDurationMs > 0 && totalTimedOutputTokens > 0 {
		avgTPS = math.Round((float64(totalTimedOutputTokens)/(float64(totalDurationMs)/1000.0))*10) / 10
	}

	writeJSON(w, map[string]interface{}{
		"total_tokens":        totalTokens,
		"input_tokens":        totalPromptTokens,
		"cached_input_tokens": totalCachedInput,
		"output_tokens":       totalOutputTokens,
		"total_cost_usd":      totalCostUSD,
		"saved_cost_usd":      totalSavedUSD,
		"avg_tps":             avgTPS,
		"requests_count":      totalRequests,
		"conversations_count": totalConversations,
		"model_breakdowns":    modelBreakdowns,
		"project_breakdowns":  projectBreakdowns,
		"telemetry_events":    telemetryEvents,
		"pricing_records":     pricingRecords,
		"models":              pricingRecords,
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
