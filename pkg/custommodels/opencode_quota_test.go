package custommodels

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCodeUsageBuckets(t *testing.T) {
	body := []byte(`{"usage":{"rolling":{"status":"ok","percent":24,"resetsAt":"2026-10-21T08:02:25.000Z"},"weekly":{"status":"ok","percent":37,"resetsAt":"2026-10-26T00:00:00.000Z"},"monthly":{"status":"ok","percent":22,"resetsAt":"2026-11-18T05:09:29.000Z"}}}`)
	buckets := parseOpenCodeUsageBuckets(body)
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}

	expected := map[string]float64{"5H": 0.76, "7D": 0.63, "30D": 0.78}
	for _, bucket := range buckets {
		want, ok := expected[bucket.Label]
		if !ok {
			t.Fatalf("unexpected bucket label %q", bucket.Label)
		}
		if bucket.Fraction == nil || math.Abs(*bucket.Fraction-want) > 0.0001 {
			t.Errorf("%s fraction = %v, want %.2f", bucket.Label, bucket.Fraction, want)
		}
	}
}

func TestOpenCodeFetchUsesGoUsageContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zen/go/v1/usage" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"rolling":{"status":"ok","percent":24,"resetsAt":"2026-10-21T08:02:25Z"},"weekly":{"status":"ok","percent":37,"resetsAt":"2026-10-26T00:00:00Z"},"monthly":{"status":"ok","percent":22,"resetsAt":"2026-11-18T05:09:29Z"}}}`))
	}))
	defer server.Close()

	var entry *ProviderQuotaRegistryEntry
	for i := range ProviderQuotaRegistry {
		if ProviderQuotaRegistry[i].Name == "OpenCode" {
			entry = &ProviderQuotaRegistry[i]
			break
		}
	}
	if entry == nil {
		t.Fatal("OpenCode registry entry missing")
	}

	result, err := entry.FetchFunc(context.Background(), server.Client(), server.URL+"/zen/go/v1/usage", "test-key")
	if err != nil {
		t.Fatalf("FetchFunc() error = %v", err)
	}
	if result.QuotaType != QuotaTypeQuota {
		t.Errorf("QuotaType = %q, want %q", result.QuotaType, QuotaTypeQuota)
	}
	if result.BalanceValue != "" || result.QuotaValue != "" {
		t.Errorf("unexpected synthetic balance/quota: balance=%q quota=%q", result.BalanceValue, result.QuotaValue)
	}
	if len(result.Buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(result.Buckets))
	}
	if result.Fraction == nil || math.Abs(*result.Fraction-0.63) > 0.0001 {
		t.Errorf("headline fraction = %v, want 0.63", result.Fraction)
	}
}

func TestApplyFetchedQuotaClearsLegacyBudget(t *testing.T) {
	legacyFraction := 1.0
	model := CustomModel{
		QuotaType:      QuotaTypeQuota,
		BalanceValue:   "$100.00",
		QuotaValue:     "$100.00",
		TotalBudget:    100,
		PrepaidBalance: 100,
		QuotaFraction:  &legacyFraction,
	}
	remaining := 0.63
	ApplyFetchedQuota(&model, QuotaResult{
		QuotaType:     QuotaTypeQuota,
		Fraction:      &remaining,
		HasPercentage: true,
		Buckets: []QuotaBucket{
			{Label: "5H", Fraction: ptrFloat(0.76)},
			{Label: "7D", Fraction: &remaining},
			{Label: "30D", Fraction: ptrFloat(0.78)},
		},
	})

	if model.TotalBudget != 0 || model.PrepaidBalance != 0 {
		t.Errorf("legacy budget fields not cleared: total=%v prepaid=%v", model.TotalBudget, model.PrepaidBalance)
	}
	if model.BalanceValue != "" || model.QuotaValue != "" {
		t.Errorf("synthetic dollar values not cleared: balance=%q quota=%q", model.BalanceValue, model.QuotaValue)
	}
	if model.QuotaFraction == nil || *model.QuotaFraction != 0.63 {
		t.Errorf("quota fraction = %v, want 0.63", model.QuotaFraction)
	}
}

func TestOpenCodeLegacyBudgetMigration(t *testing.T) {
	dir := t.TempDir()
	fraction := 1.0
	cfg := DefaultConfig()
	cfg.Models = []CustomModel{{
		ID:             "openai-deepseek-v4-flash-test",
		Name:           "deepseek-v4-flash",
		DisplayName:    "DeepSeek V4.1 Flash",
		ProviderType:   ProviderOpenAI,
		BaseURL:        "https://opencode.ai/zen/go",
		QuotaType:      QuotaTypeQuota,
		BalanceValue:   "$100.00",
		QuotaValue:     "$100.00",
		TotalBudget:    100,
		PrepaidBalance: 100,
		QuotaFraction:  &fraction,
		Enabled:        true,
	}}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "custom_models.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	migrated := store.ListModels()[0]
	if migrated.TotalBudget != 0 || migrated.PrepaidBalance != 0 {
		t.Errorf("legacy budget fields not cleared: total=%v prepaid=%v", migrated.TotalBudget, migrated.PrepaidBalance)
	}
	if migrated.BalanceValue != "" || migrated.QuotaValue != "" || migrated.QuotaFraction != nil {
		t.Errorf("legacy quota values not cleared: balance=%q quota=%q fraction=%v", migrated.BalanceValue, migrated.QuotaValue, migrated.QuotaFraction)
	}

	remaining := 0.63
	ApplyFetchedQuota(&migrated, QuotaResult{
		QuotaType:     QuotaTypeQuota,
		Fraction:      &remaining,
		HasPercentage: true,
		Buckets:       []QuotaBucket{{Label: "7D", Fraction: &remaining, Value: "63% left"}},
	})
	if err := store.SaveModel(migrated); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() reload error = %v", err)
	}
	persisted := reloaded.ListModels()[0]
	if persisted.QuotaFraction == nil || *persisted.QuotaFraction != remaining {
		t.Errorf("live quota fraction was not preserved across reload: %v", persisted.QuotaFraction)
	}
	if len(persisted.QuotaBuckets) != 1 || persisted.QuotaBuckets[0].Label != "7D" {
		t.Errorf("live quota buckets were not preserved across reload: %+v", persisted.QuotaBuckets)
	}
}

func ptrFloat(v float64) *float64 {
	return &v
}
