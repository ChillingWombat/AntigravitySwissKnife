package webgui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func resetTokenCacheForTest() {
	tokenCacheStore = newTokenCaches()
}

func writeTokenCacheTranscript(t *testing.T, path string) os.FileInfo {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":"PLANNER_RESPONSE","step_index":1,"created_at":"2026-10-30T00:00:00Z","input_tokens":10,"output_tokens":2}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestTokenCacheDeletedTranscriptRemovesStatsAndModel(t *testing.T) {
	resetTokenCacheForTest()
	path := filepath.Join(t.TempDir(), "brain", "conv-1", ".system_generated", "logs", "transcript.jsonl")
	info := writeTokenCacheTranscript(t, path)
	if stats := parseTranscriptFile(path, info); stats.requests != 1 {
		t.Fatalf("expected one parsed request, got %d", stats.requests)
	}
	detectConversationModelWithTranscript(path, "", nil, nil)
	if diagnostics := tokenCacheStore.diagnostics(time.Now()); diagnostics.TranscriptEntries != 1 || diagnostics.ConvModelEntries != 1 {
		t.Fatalf("expected populated transcript and model caches, got %+v", diagnostics)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	tokenCacheStore.reconcileTranscriptScan(map[string]struct{}{}, time.Now())
	diagnostics := tokenCacheStore.diagnostics(time.Now())
	if diagnostics.TranscriptEntries != 0 || diagnostics.ConvModelEntries != 0 {
		t.Fatalf("deleted transcript remained cached: %+v", diagnostics)
	}
}

func TestTokenCacheLRUCapacityAndRetainedStepLimit(t *testing.T) {
	resetTokenCacheForTest()
	base := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "transcript-000.jsonl")
	firstInfo := writeTokenCacheTranscript(t, firstPath)
	tokenCacheStore.putTranscript(firstPath, cachedTranscriptStats{modTimeNano: firstInfo.ModTime().UnixNano(), size: firstInfo.Size()}, base)
	for i := 0; i < tokenTranscriptCacheLimit-1; i++ {
		path := filepath.Join(dir, fmt.Sprintf("transcript-%03d.jsonl", i+1))
		stats := cachedTranscriptStats{steps: []rawStepEvent{{stepIndex: i}}}
		tokenCacheStore.putTranscript(path, stats, base.Add(time.Duration(i)*time.Second))
	}
	tokenCacheStore.getTranscript(firstPath, firstInfo, base.Add(10*time.Minute))
	tokenCacheStore.putTranscript(filepath.Join(dir, "transcript-overflow.jsonl"), cachedTranscriptStats{}, base.Add(10*time.Minute+time.Second))

	tokenCacheStore.mu.Lock()
	_, firstKept := tokenCacheStore.transcripts[normalizeTokenCacheKey(firstPath)]
	_, secondKept := tokenCacheStore.transcripts[normalizeTokenCacheKey(filepath.Join(dir, "transcript-001.jsonl"))]
	tokenCacheStore.mu.Unlock()
	if !firstKept || secondKept {
		t.Fatalf("LRU capacity kept an unexpected transcript: first=%v second=%v", firstKept, secondKept)
	}
	if got := tokenCacheStore.diagnostics(base.Add(10 * time.Minute)).TranscriptEntries; got != tokenTranscriptCacheLimit {
		t.Fatalf("expected %d transcripts, got %d", tokenTranscriptCacheLimit, got)
	}

	for i := 0; i < tokenConvModelCacheLimit+1; i++ {
		path := filepath.Join(dir, fmt.Sprintf("model-%03d", i+1))
		tokenCacheStore.putConvModel(path, cachedConvModel{modelID: "model"}, base.Add(time.Duration(i)*time.Second))
	}
	if got := tokenCacheStore.diagnostics(base.Add(10 * time.Minute)).ConvModelEntries; got != tokenConvModelCacheLimit {
		t.Fatalf("expected %d model entries, got %d", tokenConvModelCacheLimit, got)
	}

	resetTokenCacheForTest()
	steps := make([]rawStepEvent, tokenRetainedStepsLimit+2)
	for i := range steps {
		steps[i] = rawStepEvent{stepIndex: i}
	}
	path := filepath.Join(dir, "large.jsonl")
	tokenCacheStore.putTranscript(path, cachedTranscriptStats{freshInput: 1234, steps: steps}, base)
	diagnostics := tokenCacheStore.diagnostics(base)
	if diagnostics.RetainedSteps != tokenRetainedStepsLimit || diagnostics.DroppedSteps != 2 || diagnostics.TruncatedTranscripts != 1 {
		t.Fatalf("unexpected truncation diagnostics: %+v", diagnostics)
	}
	tokenCacheStore.mu.Lock()
	cached := tokenCacheStore.transcripts[normalizeTokenCacheKey(path)]
	tokenCacheStore.mu.Unlock()
	if cached.freshInput != 1234 || len(cached.steps) != tokenRetainedStepsLimit || cached.steps[0].stepIndex != 2 {
		t.Fatalf("all-time totals or retained step window corrupted: total=%d steps=%d first=%d", cached.freshInput, len(cached.steps), cached.steps[0].stepIndex)
	}
}

func TestTokenCacheTTLExpiryAndDiagnostics(t *testing.T) {
	resetTokenCacheForTest()
	base := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	tokenCacheStore.putTranscript("transcript.jsonl", cachedTranscriptStats{steps: []rawStepEvent{{stepIndex: 1}}}, base)
	tokenCacheStore.putConvModel("model-key", cachedConvModel{modelID: "model"}, base)

	partiallyExpired := tokenCacheStore.diagnostics(base.Add(tokenTranscriptCacheTTL + time.Second))
	if partiallyExpired.TranscriptEntries != 0 || partiallyExpired.ConvModelEntries != 1 {
		t.Fatalf("independent TTLs not applied: %+v", partiallyExpired)
	}
	if partiallyExpired.Limits.TranscriptEntries != tokenTranscriptCacheLimit || partiallyExpired.Limits.ConvModelEntries != tokenConvModelCacheLimit || partiallyExpired.Limits.RetainedSteps != tokenRetainedStepsLimit {
		t.Fatalf("limits not exposed: %+v", partiallyExpired.Limits)
	}
	expired := tokenCacheStore.diagnostics(base.Add(tokenConvModelCacheTTL + time.Second))
	if expired.TranscriptEntries != 0 || expired.ConvModelEntries != 0 || expired.Evictions < 2 {
		t.Fatalf("cache TTL or eviction diagnostics incorrect: %+v", expired)
	}
}
