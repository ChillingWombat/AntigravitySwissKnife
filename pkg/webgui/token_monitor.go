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
	AvgTPS        float64 `json:"avg_tps"`
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
	RoutedModel     string                           `json:"routed_model,omitempty"`
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
	lastAccessed      time.Time
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
	steps        []rawStepEvent
	droppedSteps int
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
	modTimeNano    int64
	size           int64
	lastAccessed   time.Time
	modelID        string
	displayName    string
	provider       string
	transcriptPath string
}

type protobufWireField struct {
	number   int
	wireType int
	bytes    []byte
	varint   uint64
}

const (
	tokenTranscriptCacheLimit = 256
	tokenConvModelCacheLimit  = 512
	tokenRetainedStepsLimit   = 2048
	tokenTranscriptCacheTTL   = 15 * time.Minute
	tokenConvModelCacheTTL    = 30 * time.Minute
)

type tokenCacheLimits struct {
	TranscriptEntries    int   `json:"transcript_entries"`
	ConvModelEntries     int   `json:"conv_model_entries"`
	RetainedSteps        int   `json:"retained_steps"`
	TranscriptTTLSeconds int64 `json:"transcript_ttl_seconds"`
	ConvModelTTLSeconds  int64 `json:"conv_model_ttl_seconds"`
}

type tokenCacheDiagnostics struct {
	TranscriptEntries    int              `json:"transcript_entries"`
	ConvModelEntries     int              `json:"conv_model_entries"`
	RetainedSteps        int              `json:"retained_steps"`
	Limits               tokenCacheLimits `json:"limits"`
	Evictions            int64            `json:"evictions"`
	TruncatedTranscripts int              `json:"truncated_transcripts"`
	DroppedSteps         int64            `json:"dropped_steps"`
}

type tokenCaches struct {
	mu          sync.Mutex
	transcripts map[string]cachedTranscriptStats
	convModels  map[string]cachedConvModel
	evictions   int64
}

var tokenCacheStore = newTokenCaches()

func newTokenCaches() *tokenCaches {
	return &tokenCaches{
		transcripts: make(map[string]cachedTranscriptStats),
		convModels:  make(map[string]cachedConvModel),
	}
}

func normalizeTokenCacheKey(path string) string {
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	return filepath.Clean(path)
}

func trimTranscriptSteps(stats *cachedTranscriptStats) {
	if extra := len(stats.steps) - tokenRetainedStepsLimit; extra > 0 {
		stats.steps = append([]rawStepEvent(nil), stats.steps[extra:]...)
		stats.droppedSteps += extra
	}
}

func (c *tokenCaches) sweepLocked(now time.Time) {
	for key, cached := range c.transcripts {
		if now.Sub(cached.lastAccessed) >= tokenTranscriptCacheTTL {
			delete(c.transcripts, key)
			c.evictions++
		}
	}
	for key, cached := range c.convModels {
		if now.Sub(cached.lastAccessed) >= tokenConvModelCacheTTL {
			delete(c.convModels, key)
			c.evictions++
		}
	}
}

func (c *tokenCaches) getTranscript(key string, info os.FileInfo, now time.Time) (cachedTranscriptStats, bool) {
	key = normalizeTokenCacheKey(key)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepLocked(now)
	cached, ok := c.transcripts[key]
	if !ok || cached.modTimeNano != info.ModTime().UnixNano() || cached.size != info.Size() {
		return cachedTranscriptStats{}, false
	}
	cached.lastAccessed = now
	c.transcripts[key] = cached
	return cached, true
}

func (c *tokenCaches) putTranscript(key string, stats cachedTranscriptStats, now time.Time) {
	key = normalizeTokenCacheKey(key)
	trimTranscriptSteps(&stats)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepLocked(now)
	if _, exists := c.transcripts[key]; !exists && len(c.transcripts) >= tokenTranscriptCacheLimit {
		c.evictOldestTranscriptLocked()
	}
	stats.lastAccessed = now
	c.transcripts[key] = stats
}

func (c *tokenCaches) evictOldestTranscriptLocked() {
	var oldestKey string
	var oldestTime time.Time
	for key, cached := range c.transcripts {
		if oldestKey == "" || cached.lastAccessed.Before(oldestTime) || (cached.lastAccessed.Equal(oldestTime) && key < oldestKey) {
			oldestKey = key
			oldestTime = cached.lastAccessed
		}
	}
	if oldestKey != "" {
		delete(c.transcripts, oldestKey)
		c.evictions++
	}
}

func (c *tokenCaches) getConvModel(key string, info os.FileInfo, now time.Time) (cachedConvModel, bool) {
	key = normalizeTokenCacheKey(key)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepLocked(now)
	cached, ok := c.convModels[key]
	if !ok || cached.modTimeNano != info.ModTime().UnixNano() || cached.size != info.Size() {
		return cachedConvModel{}, false
	}
	cached.lastAccessed = now
	c.convModels[key] = cached
	return cached, true
}

func (c *tokenCaches) putConvModel(key string, cached cachedConvModel, now time.Time) {
	key = normalizeTokenCacheKey(key)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepLocked(now)
	if _, exists := c.convModels[key]; !exists && len(c.convModels) >= tokenConvModelCacheLimit {
		c.evictOldestConvModelLocked()
	}
	cached.lastAccessed = now
	c.convModels[key] = cached
}

func (c *tokenCaches) evictOldestConvModelLocked() {
	var oldestKey string
	var oldestTime time.Time
	for key, cached := range c.convModels {
		if oldestKey == "" || cached.lastAccessed.Before(oldestTime) || (cached.lastAccessed.Equal(oldestTime) && key < oldestKey) {
			oldestKey = key
			oldestTime = cached.lastAccessed
		}
	}
	if oldestKey != "" {
		delete(c.convModels, oldestKey)
		c.evictions++
	}
}

func (c *tokenCaches) reconcileTranscriptScan(seen map[string]struct{}, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepLocked(now)
	for key := range c.transcripts {
		if _, ok := seen[key]; !ok {
			delete(c.transcripts, key)
			c.evictions++
		}
	}
	for key, cached := range c.convModels {
		_, keySeen := seen[key]
		_, transcriptSeen := seen[cached.transcriptPath]
		if (!keySeen && cached.transcriptPath == "") || (cached.transcriptPath != "" && !transcriptSeen) {
			delete(c.convModels, key)
			c.evictions++
		}
	}
}

func (c *tokenCaches) diagnostics(now time.Time) tokenCacheDiagnostics {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepLocked(now)
	diagnostics := tokenCacheDiagnostics{
		TranscriptEntries: len(c.transcripts),
		ConvModelEntries:  len(c.convModels),
		Limits: tokenCacheLimits{
			TranscriptEntries:    tokenTranscriptCacheLimit,
			ConvModelEntries:     tokenConvModelCacheLimit,
			RetainedSteps:        tokenRetainedStepsLimit,
			TranscriptTTLSeconds: int64(tokenTranscriptCacheTTL / time.Second),
			ConvModelTTLSeconds:  int64(tokenConvModelCacheTTL / time.Second),
		},
		Evictions: c.evictions,
	}
	for _, cached := range c.transcripts {
		diagnostics.RetainedSteps += len(cached.steps)
		diagnostics.DroppedSteps += int64(cached.droppedSteps)
		if cached.droppedSteps > 0 {
			diagnostics.TruncatedTranscripts++
		}
	}
	return diagnostics
}

func readProtobufVarint(data []byte, offset *int) (uint64, bool) {
	var value uint64
	for shift := uint(0); shift < 64 && *offset < len(data); shift += 7 {
		current := data[*offset]
		*offset = *offset + 1
		value |= uint64(current&0x7f) << shift
		if current&0x80 == 0 {
			return value, true
		}
	}
	return 0, false
}

func parseProtobufWireFields(data []byte) ([]protobufWireField, bool) {
	var fields []protobufWireField
	for offset := 0; offset < len(data); {
		key, ok := readProtobufVarint(data, &offset)
		if !ok {
			return nil, false
		}
		number := int(key >> 3)
		wireType := int(key & 0x7)
		if number <= 0 {
			return nil, false
		}

		field := protobufWireField{number: number, wireType: wireType}
		switch wireType {
		case 0:
			value, ok := readProtobufVarint(data, &offset)
			if !ok {
				return nil, false
			}
			field.varint = value
		case 1:
			if offset+8 > len(data) {
				return nil, false
			}
			offset += 8
		case 2:
			length, ok := readProtobufVarint(data, &offset)
			if !ok || length > uint64(len(data)-offset) {
				return nil, false
			}
			field.bytes = data[offset : offset+int(length)]
			offset += int(length)
		case 5:
			if offset+4 > len(data) {
				return nil, false
			}
			offset += 4
		default:
			return nil, false
		}
		fields = append(fields, field)
	}
	return fields, true
}

func routedModelMetadataPair(data []byte) (key, value string, ok bool) {
	fields, valid := parseProtobufWireFields(data)
	if !valid {
		return "", "", false
	}
	for _, field := range fields {
		if field.wireType != 2 {
			continue
		}
		switch field.number {
		case 1:
			key = string(field.bytes)
		case 2:
			value = string(field.bytes)
		}
	}
	return key, value, key != ""
}

func isRoutedModelIdentifier(value string) bool {
	if len(value) < 3 || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || strings.ContainsRune("._-/:", ch) {
			continue
		}
		return false
	}
	return true
}

func parseRoutedModelMetadata(data []byte) (responseStep int, routedModel string, ok bool) {
	outerFields, valid := parseProtobufWireFields(data)
	if !valid {
		return 0, "", false
	}
	for _, outer := range outerFields {
		if outer.number != 1 || outer.wireType != 2 {
			continue
		}
		generationFields, valid := parseProtobufWireFields(outer.bytes)
		if !valid {
			continue
		}
		lastStepIndex := -1
		var route string
		for _, field := range generationFields {
			switch {
			case field.number == 19 && field.wireType == 2:
				if candidate := string(field.bytes); isRoutedModelIdentifier(candidate) {
					route = candidate
				}
			case field.number == 20 && field.wireType == 2:
				key, value, valid := routedModelMetadataPair(field.bytes)
				if valid && key == "last_step_index" {
					if parsed, err := strconv.Atoi(value); err == nil {
						lastStepIndex = parsed
					}
				}
			}
		}
		if route != "" && lastStepIndex >= 0 {
			return lastStepIndex + 1, route, true
		}
	}
	return 0, "", false
}

func readRoutedModelByStep(dbPath string) map[int]string {
	routes := make(map[int]string)
	if dbPath == "" {
		return routes
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&_busy_timeout=2000", dbPath))
	if err != nil {
		return routes
	}
	defer db.Close()

	rows, err := db.Query("SELECT data FROM gen_metadata ORDER BY idx DESC LIMIT 128")
	if err != nil {
		return routes
	}
	defer rows.Close()

	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if responseStep, routedModel, ok := parseRoutedModelMetadata(data); ok {
			routes[responseStep] = routedModel
		}
	}
	return routes
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
	if env := os.Getenv("ANTIGRAVITY_DATA_DIR"); env != "" {
		return env
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
	cacheKey := normalizeTokenCacheKey(tPath)
	if cached, ok := tokenCacheStore.getTranscript(cacheKey, info, time.Now()); ok {
		return cached
	}

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
		var itemType string
		if bytes.Contains(line, []byte("USER_INPUT")) || bytes.Contains(line, []byte("USER_EXPLICIT")) {
			itemType = "USER_INPUT"
		} else if bytes.Contains(line, []byte("PLANNER_RESPONSE")) {
			itemType = "PLANNER_RESPONSE"
		} else if bytes.Contains(line, []byte("GENERIC")) {
			itemType = "GENERIC"
		}

		if hasTokens {
			var entry struct {
				Type            string `json:"type"`
				StepIndex       int    `json:"step_index"`
				DurationMs      int64  `json:"duration_ms"`
				InputTokens     int64  `json:"input_tokens"`
				OutputTokens    int64  `json:"output_tokens"`
				CacheReadTokens int64  `json:"cache_read_tokens"`
			}
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
			itemType:  itemType,
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
						if strings.HasPrefix(nextItem.itemType, "USER") && diff > 120000 {
							continue
						}
						if impliedTPS >= 3.0 && impliedTPS <= 350.0 {
							durMs = diff
							break
						}
					}
				}
			}
			// Terminal step at end of conversation file: evaluate against file modification time
			if durMs <= 0 && info != nil && !info.ModTime().IsZero() && info.ModTime().After(item.timestamp) {
				mDiff := info.ModTime().Sub(item.timestamp).Milliseconds()
				if mDiff >= 200 && mDiff <= 120000 {
					impliedTPS := float64(item.outputTokens) / (float64(mDiff) / 1000.0)
					if impliedTPS >= 3.0 && impliedTPS <= 350.0 {
						durMs = mDiff
					}
				}
			}
		}

		if durMs > 0 && item.outputTokens > 0 {
			rawTPS := float64(item.outputTokens) / (float64(durMs) / 1000.0)
			if rawTPS >= 3.0 && rawTPS <= 350.0 {
				stats.totalDurationMs += durMs
				stats.timedOutputTokens += item.outputTokens
			} else if rawTPS > 350.0 {
				// Bound realistic generation speed
				clampedDurMs := int64(math.Ceil(float64(item.outputTokens) / 350.0 * 1000.0))
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

	tokenCacheStore.putTranscript(cacheKey, stats, time.Now())
	return stats
}

var modelSelectionRegex = regexp.MustCompile(`(?i)setting\s+[` + "`" + `']Model Selection[` + "`" + `'].*?to\s+(.+?)(?:\.\s+|\.?(?:\r|\n|</USER_SETTINGS_CHANGE>))`)
var effortSuffixRegex = regexp.MustCompile(`(?i)\s*\(?(?:low|medium|high|extra|xhigh|max|off)\)?\s*$`)

func stripEffortSuffix(name string) string {
	cleaned := strings.TrimSpace(effortSuffixRegex.ReplaceAllString(name, ""))
	return strings.TrimSpace(cleaned)
}

type magpieSpeedSnapshot struct {
	overallSpeed float64
	modelSpeeds  map[string]float64
	fetchedAt    time.Time
}

var (
	magpieSpeedMu       sync.RWMutex
	cachedMagpieSpeed   magpieSpeedSnapshot
	magpieUsageEndpoint = "http://127.0.0.1:3425/v1/magpie/usage"
)

// SetMagpieEndpointForTest allows tests to point to a mock Magpie endpoint or empty string to disable.
func SetMagpieEndpointForTest(endpoint string) {
	magpieSpeedMu.Lock()
	defer magpieSpeedMu.Unlock()
	magpieUsageEndpoint = endpoint
	cachedMagpieSpeed = magpieSpeedSnapshot{}
}

func getMagpieSpeedData(ctx context.Context) (float64, map[string]float64) {
	magpieSpeedMu.RLock()
	endpoint := magpieUsageEndpoint
	if endpoint == "" {
		magpieSpeedMu.RUnlock()
		return 0, nil
	}
	if time.Since(cachedMagpieSpeed.fetchedAt) < 15*time.Second && cachedMagpieSpeed.modelSpeeds != nil {
		ov := cachedMagpieSpeed.overallSpeed
		m := cachedMagpieSpeed.modelSpeeds
		magpieSpeedMu.RUnlock()
		return ov, m
	}
	magpieSpeedMu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil
	}
	client := &http.Client{Timeout: 250 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		magpieSpeedMu.Lock()
		cachedMagpieSpeed = magpieSpeedSnapshot{fetchedAt: time.Now()}
		magpieSpeedMu.Unlock()
		return 0, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		magpieSpeedMu.Lock()
		cachedMagpieSpeed = magpieSpeedSnapshot{fetchedAt: time.Now()}
		magpieSpeedMu.Unlock()
		return 0, nil
	}

	var data struct {
		DecodeMs  int64 `json:"decode_ms"`
		DecodeOut int64 `json:"decode_out"`
		Models    []struct {
			ID        string `json:"id"`
			Model     string `json:"model"`
			Name      string `json:"name"`
			DecodeMs  int64  `json:"decode_ms"`
			DecodeOut int64  `json:"decode_out"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, nil
	}

	overall := 0.0
	if data.DecodeMs > 0 && data.DecodeOut > 0 {
		overall = float64(data.DecodeOut) * 1000.0 / float64(data.DecodeMs)
	}

	mSpeeds := make(map[string]float64)
	for _, m := range data.Models {
		if m.DecodeMs > 0 && m.DecodeOut > 0 {
			spd := float64(m.DecodeOut) * 1000.0 / float64(m.DecodeMs)
			recordModelSpeedKey(mSpeeds, m.ID, spd)
			recordModelSpeedKey(mSpeeds, m.Model, spd)
			recordModelSpeedKey(mSpeeds, m.Name, spd)
		}
	}

	magpieSpeedMu.Lock()
	cachedMagpieSpeed = magpieSpeedSnapshot{
		overallSpeed: overall,
		modelSpeeds:  mSpeeds,
		fetchedAt:    time.Now(),
	}
	magpieSpeedMu.Unlock()
	return overall, mSpeeds
}

func recordModelSpeedKey(target map[string]float64, raw string, spd float64) {
	if raw == "" {
		return
	}
	clean := strings.ToLower(stripEffortSuffix(raw))
	target[clean] = spd
	if parts := strings.Split(clean, "/"); len(parts) > 1 {
		target[parts[len(parts)-1]] = spd
	}
	if atIdx := strings.Index(clean, " @ "); atIdx > 0 {
		beforeAt := strings.TrimSpace(clean[:atIdx])
		target[beforeAt] = spd
		if parts := strings.Split(beforeAt, "/"); len(parts) > 1 {
			target[parts[len(parts)-1]] = spd
		}
	}
}

func lookupModelSpeed(modelID, modelName string, speeds map[string]float64) float64 {
	if speeds == nil {
		return 0
	}
	candidates := []string{
		strings.ToLower(modelID),
		strings.ToLower(stripEffortSuffix(modelID)),
		strings.ToLower(modelName),
		strings.ToLower(stripEffortSuffix(modelName)),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if spd, ok := speeds[c]; ok && spd > 0 {
			return spd
		}
		if parts := strings.Split(c, "/"); len(parts) > 1 {
			if spd, ok := speeds[parts[len(parts)-1]]; ok && spd > 0 {
				return spd
			}
		}
	}
	return 0
}

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

	cleanedName := stripEffortSuffix(rawTrimmed)
	lower := strings.ToLower(rawTrimmed)
	lowerClean := strings.ToLower(cleanedName)

	// 1. Match custom models
	for _, cm := range customModels {
		cmName := strings.ToLower(cm.Name)
		cmDisp := strings.ToLower(cm.DisplayName)
		if strings.EqualFold(cm.Name, rawTrimmed) || strings.EqualFold(cm.DisplayName, rawTrimmed) ||
			(cleanedName != "" && (strings.EqualFold(cm.Name, cleanedName) || strings.EqualFold(cm.DisplayName, cleanedName))) ||
			(cmDisp != "" && (strings.Contains(lower, cmDisp) || strings.Contains(lowerClean, cmDisp))) ||
			(cmName != "" && (strings.Contains(lower, cmName) || strings.Contains(lowerClean, cmName))) {
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
	if strings.Contains(lower, "claude opus 5.5") ||
		strings.Contains(lower, "claude-opus-5-5") ||
		strings.Contains(lower, "claude-opus-5.5") ||
		strings.Contains(lower, "claude opus 5-5") ||
		strings.Contains(lower, "opus 5.5") ||
		strings.Contains(lower, "opus-5-5") ||
		strings.Contains(lower, "opus5.5") {
		return "claude-opus-5-5", "Claude Opus 5.5", "anthropic"
	}
	if strings.Contains(lower, "claude opus 4.6") || strings.Contains(lower, "claude-opus-4-6") ||
		strings.Contains(lower, "claude opus 4-6") || strings.Contains(lower, "claude-opus-4.6") {
		return "claude-opus-4-6", "Claude Opus 4.6", "anthropic"
	}
	// Legacy unversioned Opus selections were treated as 4.6, but 5.5 must not fall through here.
	if strings.Contains(lower, "claude opus") &&
		!strings.Contains(lower, "5.5") && !strings.Contains(lower, "5-5") {
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
		cacheKey = normalizeTokenCacheKey(cacheKey)
	}
	transcriptPath := ""
	if tPath != "" {
		transcriptPath = normalizeTokenCacheKey(tPath)
	}

	var convID string
	if convDBPath != "" && strings.HasSuffix(convDBPath, ".db") {
		convID = strings.TrimSuffix(filepath.Base(convDBPath), ".db")
	} else if convDBPath != "" && !strings.Contains(convDBPath, string(filepath.Separator)) {
		convID = convDBPath
	} else if tPath != "" {
		dir := filepath.Dir(tPath)
		dir = filepath.Dir(dir)
		convID = filepath.Base(filepath.Dir(dir))
	}

	// 0. Priority: Check if Swiss Knife proxy recorded an active custom model for this conversation
	if convID != "" {
		if customModelID := custommodels.GetRoutedConversationModel(convID); customModelID != "" {
			for _, cm := range customModels {
				if cm.ID == customModelID || cm.Name == customModelID {
					disp := cm.DisplayName
					if disp == "" {
						disp = cm.Name
					}
					prov := custommodels.NormalizePricingProvider(string(cm.ProviderType), cm.Name)
					if cacheKey != "" {
						tokenCacheStore.putConvModel(cacheKey, cachedConvModel{modTimeNano: cacheModTime, size: cacheSize, modelID: cm.Name, displayName: disp, provider: prov, transcriptPath: transcriptPath}, time.Now())
					}
					return cm.Name, disp, prov
				}
			}
		}
	}

	// The sidecar can change without touching the transcript, so it must run
	// before the transcript-keyed warm cache.
	if cacheKey != "" {
		if info, err := os.Stat(cacheKey); err == nil {
			if cached, ok := tokenCacheStore.getConvModel(cacheKey, info, time.Now()); ok {
				return cached.modelID, cached.displayName, cached.provider
			}
		}
	}

	// 1. Authoritative check: transcript.jsonl step 0 <USER_SETTINGS_CHANGE>
	if tPath != "" {
		if rawName, ok := extractModelFromTranscript(tPath); ok {
			mID, mName, mProv := mapModelNameToMetadata(rawName, customModels, catalogModels)
			if cacheKey != "" {
				tokenCacheStore.putConvModel(cacheKey, cachedConvModel{modTimeNano: cacheModTime, size: cacheSize, modelID: mID, displayName: mName, provider: mProv, transcriptPath: transcriptPath}, time.Now())
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
						tokenCacheStore.putConvModel(cacheKey, cachedConvModel{modTimeNano: cacheModTime, size: cacheSize, modelID: cm.Name, displayName: disp, provider: prov, transcriptPath: transcriptPath}, time.Now())
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
		tokenCacheStore.putConvModel(cacheKey, cachedConvModel{modTimeNano: cacheModTime, size: cacheSize, modelID: mID, displayName: mName, provider: mProv, transcriptPath: transcriptPath}, time.Now())
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

// resolveConversationModel resolves the active model for a conversation ID,
// giving the proxy sidecar priority over transcript and native attribution.
func resolveConversationModel(convID string) (modelID, displayName, provider string) {
	var customModels []custommodels.CustomModel
	if store, err := custommodels.NewStore(""); err == nil {
		customModels = store.ListModels()
	}
	mID, mName, mProv := detectConversationModelWithTranscript("", convID, customModels, nil)
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
	classFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("classification")))
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
	magpieOverall, magpieModelSpeeds := getMagpieSpeedData(r.Context())
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
		durationMs    int64
		timedOutput   int64
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
	seenTranscripts := make(map[string]struct{})
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
			seenTranscripts[normalizeTokenCacheKey(tPath)] = struct{}{}

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

			// Classification filter (#native vs #custom vs all)
			if classFilter == "native" || classFilter == "custom" {
				if string(pRec.Classification) != classFilter {
					continue
				}
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
			if agg.durationMs > 0 && agg.timedOutput > 0 {
				pa.durationMs += agg.durationMs
				pa.timedOutput += agg.timedOutput
			}
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
		if magpieModelSpeeds != nil {
			if spd := lookupModelSpeed(ma.modelID, ma.modelName, magpieModelSpeeds); spd > 0 {
				avgTPS = math.Round(spd*10) / 10
			}
		}
		if avgTPS <= 0 && ma.durationMs > 0 && ma.timedOutput > 0 {
			rawTPS := float64(ma.timedOutput) / (float64(ma.durationMs) / 1000.0)
			if rawTPS > 350.0 {
				rawTPS = 350.0
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
		pAvgTPS := 0.0
		if pa.durationMs > 0 && pa.timedOutput > 0 {
			rawTPS := float64(pa.timedOutput) / (float64(pa.durationMs) / 1000.0)
			if rawTPS > 350.0 {
				rawTPS = 350.0
			}
			pAvgTPS = math.Round(rawTPS*10) / 10
		}
		if pAvgTPS <= 0 && magpieModelSpeeds != nil {
			var weightedSpeedSum float64
			var weightSum float64
			for mName, tok := range pa.tokensByModel {
				if spd := lookupModelSpeed("", mName, magpieModelSpeeds); spd > 0 && tok > 0 {
					weightedSpeedSum += spd * float64(tok)
					weightSum += float64(tok)
				}
			}
			if weightSum > 0 {
				pAvgTPS = math.Round((weightedSpeedSum/weightSum)*10) / 10
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
			SavedUSD:      pa.savedUSD,
			CacheHitRatio: cacheRatio,
			SessionsCount: pa.sessionsCount,
			Requests:      pa.requests,
			AvgTPS:        pAvgTPS,
			LastActive:    formatRelativeTime(pa.lastActive),
			PrimaryModel:  primaryModel,
		})
	}
	tokenCacheStore.reconcileTranscriptScan(seenTranscripts, time.Now())

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
	routedModelsByConversation := make(map[string]map[int]string)
	for _, st := range allRecentSteps {
		if _, exists := routedModelsByConversation[st.convID]; exists {
			continue
		}
		routedModelsByConversation[st.convID] = readRoutedModelByStep(filepath.Join(convDir, st.convID+".db"))
	}
	telemetryEvents := make([]TokenTelemetryEvent, 0, len(allRecentSteps))
	for idx, st := range allRecentSteps {
		tps := 0.0
		durMs := st.durationMs
		if durMs > 0 && st.output > 0 {
			rawTPS := float64(st.output) / (float64(durMs) / 1000.0)
			if rawTPS > 350.0 {
				rawTPS = 350.0
			}
			tps = math.Round(rawTPS*10) / 10
		}
		if (tps <= 0 || durMs <= 0) && st.output > 0 && magpieModelSpeeds != nil {
			if spd := lookupModelSpeed(st.modelID, st.modelName, magpieModelSpeeds); spd > 0 {
				tps = math.Round(spd*10) / 10
				durMs = int64(math.Round(float64(st.output) / spd * 1000.0))
			}
		}
		tsStr := st.createdAt.Local().Format("2006-01-02 15:04:05")
		telemetryEvents = append(telemetryEvents, TokenTelemetryEvent{
			ID:              st.convID + "-" + strconv.Itoa(st.stepIndex) + "-" + strconv.Itoa(idx),
			Timestamp:       tsStr,
			Project:         st.project,
			Model:           st.modelName,
			ModelID:         st.modelID,
			CanonicalID:     st.modelID,
			RoutedModel:     routedModelsByConversation[st.convID][st.stepIndex],
			Provider:        st.provider,
			Classification:  st.classification,
			InputTokens:     st.freshInput + st.cachedInput,
			CachedTokens:    st.cachedInput,
			OutputTokens:    st.output,
			CostUSD:         st.costUSD,
			PriceConfigured: st.priceConfigured,
			DurationMs:      durMs,
			TPS:             tps,
			IsSubagent:      st.isSubagent,
			AgentRole:       st.agentRole,
		})
	}

	avgTPS := 0.0
	if magpieOverall > 0 {
		avgTPS = math.Round(magpieOverall*10) / 10
	} else if totalDurationMs > 0 && totalTimedOutputTokens > 0 {
		rawTPS := float64(totalTimedOutputTokens) / (float64(totalDurationMs) / 1000.0)
		if rawTPS > 350.0 {
			rawTPS = 350.0
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
			if rawTPS > 350.0 {
				rawTPS = 350.0
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
		"cache":           tokenCacheStore.diagnostics(time.Now()),
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

// ChatContextWindow is the active context budget reported by the host language
// server. MaxTokens is the denominator that decides when compaction fires.
type ChatContextWindow struct {
	UsedTokens int64  `json:"used_tokens"`
	MaxTokens  int64  `json:"max_tokens"`
	Source     string `json:"source"`
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
	ContextWindow   *ChatContextWindow  `json:"context_window,omitempty"`
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
			if rawTPS > 350.0 {
				rawTPS = 350.0
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
							if impliedTPS >= 3.0 && impliedTPS <= 350.0 {
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
		if rawSpeed > 350.0 {
			rawSpeed = 350.0
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
			if rawSpeed > 350.0 {
				rawSpeed = 350.0
			}
			overallSpeed = math.Round(rawSpeed*10) / 10
		}
	}

	var latestContext int64
	if len(turns) > 0 {
		latestContext = turns[len(turns)-1].ContextTokens
	}

	var contextWindow *ChatContextWindow
	conversationsDir := filepath.Join(home, ".gemini", "antigravity", "conversations")
	if hostWindow, ok := readHostContextWindow(conversationsDir, convID); ok {
		usedTokens := hostWindow.UsedTokens
		if usedTokens <= 0 {
			usedTokens = latestContext
		}
		contextWindow = &ChatContextWindow{
			UsedTokens: usedTokens,
			MaxTokens:  hostWindow.MaxTokens,
			Source:     "language_server",
		}
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
		Turns:         turns,
		ContextWindow: contextWindow,
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
	s.lifetimeCostsMu.RLock()
	if s.lifetimeCostsCache != nil && time.Now().Before(s.lifetimeCostsExp) {
		cached := make(map[string]float64, len(s.lifetimeCostsCache))
		for k, v := range s.lifetimeCostsCache {
			cached[k] = v
		}
		s.lifetimeCostsMu.RUnlock()
		return cached
	}
	s.lifetimeCostsMu.RUnlock()

	s.lifetimeCostsMu.Lock()
	defer s.lifetimeCostsMu.Unlock()

	// Double-check under write lock
	if s.lifetimeCostsCache != nil && time.Now().Before(s.lifetimeCostsExp) {
		cached := make(map[string]float64, len(s.lifetimeCostsCache))
		for k, v := range s.lifetimeCostsCache {
			cached[k] = v
		}
		return cached
	}

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
		if mDisplayName != "" && !strings.EqualFold(mDisplayName, mID) {
			costs[strings.ToLower(mDisplayName)] += cost
		}
	}

	s.lifetimeCostsCache = costs
	s.lifetimeCostsExp = time.Now().Add(60 * time.Second)
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
