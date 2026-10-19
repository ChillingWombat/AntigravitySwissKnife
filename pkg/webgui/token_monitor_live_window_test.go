package webgui

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// TestLiveTokenAccounting_BoundedWindow measures the #126 accounting definition
// against the real Antigravity transcripts on this machine: the last 24h window
// is folded into fresh input / cache reads / output, and the report is checked
// against the invariant
//
//	total_tokens = fresh_input + cache_read + output
//
// (cache_read is NOT a subset of input_tokens, so the three fields are summed
// rather than double-counted).
//
// The test skips when no live conversations directory exists, so it is a no-op in
// CI. Transcripts are append-only JSONL, so files not touched since the cutoff
// cannot hold in-window steps and are skipped for speed; every candidate file is
// still parsed and window-filtered with the exact production helpers.
func TestLiveTokenAccounting_BoundedWindow(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no user home directory")
	}
	brainDir := filepath.Join(home, ".gemini", "antigravity", "brain")
	if _, err := os.Stat(brainDir); err != nil {
		t.Skip("no live Antigravity brain directory")
	}

	now := time.Now()

	// Parse every live transcript once, then fold the same real steps into each
	// window (24h / 7d / 30d / all) with the production window filter.
	readEntries, err := os.ReadDir(brainDir)
	if err != nil {
		t.Skipf("cannot read brain dir: %v", err)
	}
	type liveTranscript struct {
		id    string
		path  string
		stats cachedTranscriptStats
	}
	var transcripts []liveTranscript
	for _, entry := range readEntries {
		if !entry.IsDir() {
			continue
		}
		tPath := filepath.Join(brainDir, entry.Name(), ".system_generated", "logs", "transcript.jsonl")
		info, err := os.Stat(tPath)
		if err != nil || info.IsDir() {
			continue
		}
		transcripts = append(transcripts, liveTranscript{id: entry.Name(), path: tPath, stats: parseTranscriptFile(tPath, info)})
	}
	if len(transcripts) == 0 {
		t.Skip("no live transcripts found")
	}

	for _, rangeKey := range []string{tokenRange24h, tokenRange7d, tokenRange30d, tokenRangeAll} {
		key := rangeKey
		t.Run(key, func(t *testing.T) {
			windowCutoff := parseTokenRange(key).cutoff(now)
			var steps []rawStepEvent
			windowFiles := 0
			for _, tr := range transcripts {
				windowSteps := aggregateSteps(tr.stats.steps, windowCutoff)
				if windowSteps.requests == 0 {
					continue
				}
				windowFiles++
				steps = append(steps, windowSteps.steps...)
			}

			agg := aggregateSteps(steps, time.Time{})
			hitRatio := 0.0
			if prompt := agg.freshInput + agg.cachedInput; prompt > 0 {
				hitRatio = float64(agg.cachedInput) / float64(prompt) * 100
			}
			total := agg.freshInput + agg.cachedInput + agg.outputTokens

			t.Logf("live %s window (cutoff %s, files with in-window usage %d of %d):", key, windowCutoff.Format(time.RFC3339), windowFiles, len(transcripts))
			t.Logf("  fresh input : %12d tokens", agg.freshInput)
			t.Logf("  cache reads : %12d tokens", agg.cachedInput)
			t.Logf("  output      : %12d tokens", agg.outputTokens)
			t.Logf("  requests    : %12d", agg.requests)
			t.Logf("  cache hit   : %11.1f%% (cache / (fresh+cache))", hitRatio)
			t.Logf("  total       : %12d tokens (= fresh + cache + output)", total)
			t.Logf("  prompt side : %12d tokens (= fresh + cache, reported as input_tokens)", agg.freshInput+agg.cachedInput)

			// The accounting definition is the regression surface here: whatever
			// the absolute numbers, the total must be the plain sum of the three
			// disjoint fields and the cache-hit ratio must stay inside 0-100.
			if total != agg.freshInput+agg.cachedInput+agg.outputTokens {
				t.Fatalf("total %d != fresh %d + cache %d + output %d", total, agg.freshInput, agg.cachedInput, agg.outputTokens)
			}
			if hitRatio < 0 || hitRatio > 100 {
				t.Fatalf("cache hit ratio %.2f%% out of range", hitRatio)
			}
			if agg.requests > 0 && total == 0 {
				t.Fatalf("%d requests carried zero tokens — accounting dropped the counters", agg.requests)
			}

			if key != tokenRangeAll {
				return
			}

			// No step may be logged in two conversations (parent vs subagent
			// transcripts), otherwise summing every conversation would count the
			// same work twice. Signature = timestamp + token triple.
			type stepSig struct {
				ts                int64
				fresh, cache, out int64
			}
			owners := map[stepSig]string{}
			duplicates := 0
			for _, tr := range transcripts {
				for _, st := range tr.stats.steps {
					sig := stepSig{ts: st.createdAt.UnixNano(), fresh: st.freshInput, cache: st.cachedInput, out: st.output}
					if prev, seen := owners[sig]; seen && prev != tr.id {
						duplicates++
						continue
					}
					owners[sig] = tr.id
				}
			}
			t.Logf("cross-conversation duplicate steps: %d (distinct signatures %d)", duplicates, len(owners))
			if duplicates > 0 {
				t.Fatalf("%d steps are logged in more than one conversation — summing conversations double counts", duplicates)
			}

			// Per-conversation reference row used to align the dashboard with the
			// external Magpie readout (Total Tokens must contain cache reads).
			type convRow struct {
				id  string
				agg tokenStepAgg
				hit float64
			}
			var convs []convRow
			for _, tr := range transcripts {
				cAgg := aggregateSteps(tr.stats.steps, time.Time{})
				hit := 0.0
				if prompt := cAgg.freshInput + cAgg.cachedInput; prompt > 0 {
					hit = float64(cAgg.cachedInput) / float64(prompt) * 100
				}
				convs = append(convs, convRow{id: tr.id, agg: cAgg, hit: hit})
			}
			sort.Slice(convs, func(i, j int) bool {
				return convs[i].agg.freshInput+convs[i].agg.cachedInput+convs[i].agg.outputTokens >
					convs[j].agg.freshInput+convs[j].agg.cachedInput+convs[j].agg.outputTokens
			})
			for i := 0; i < len(convs) && i < 3; i++ {
				c := convs[i]
				cTotal := c.agg.freshInput + c.agg.cachedInput + c.agg.outputTokens
				t.Logf("  top conv #%d %s: req=%d fresh=%d cache=%d out=%d total=%d hit=%.1f%%",
					i+1, c.id, c.agg.requests, c.agg.freshInput, c.agg.cachedInput, c.agg.outputTokens, cTotal, c.hit)
				if cTotal != c.agg.freshInput+c.agg.cachedInput+c.agg.outputTokens {
					t.Fatalf("conversation %s: total mismatch", c.id)
				}
			}
		})
	}
}
