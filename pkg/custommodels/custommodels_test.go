package custommodels

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCustomModel_CalculatePercentage(t *testing.T) {
	// 1. Cost-based
	mCost := CustomModel{
		ID:             "test-cost",
		QuotaType:      QuotaCostBased,
		PrepaidBalance: 25.0,
		TotalBudget:    50.0,
	}
	pct, hasInfo := mCost.CalculatePercentage()
	if !hasInfo || pct != 50 {
		t.Errorf("cost-based: expected (50, true), got (%d, %v)", pct, hasInfo)
	}

	// Cost-based with zero budget
	mCostZero := CustomModel{
		ID:             "test-cost-zero",
		QuotaType:      QuotaCostBased,
		PrepaidBalance: 0,
		TotalBudget:    0,
	}
	pct, hasInfo = mCostZero.CalculatePercentage()
	if hasInfo {
		t.Errorf("cost-based with 0 budget: expected hasInfo=false, got (%d, %v)", pct, hasInfo)
	}

	// 2. Quota-based
	frac := 0.72
	mQuota := CustomModel{
		ID:            "test-quota",
		QuotaType:     QuotaQuotaBased,
		QuotaFraction: &frac,
	}
	pct, hasInfo = mQuota.CalculatePercentage()
	if !hasInfo || pct != 72 {
		t.Errorf("quota-based: expected (72, true), got (%d, %v)", pct, hasInfo)
	}

	// Quota-based nil fraction
	mQuotaNil := CustomModel{
		ID:            "test-quota-nil",
		QuotaType:     QuotaQuotaBased,
		QuotaFraction: nil,
	}
	pct, hasInfo = mQuotaNil.CalculatePercentage()
	if hasInfo {
		t.Errorf("quota-based with nil fraction: expected hasInfo=false, got (%d, %v)", pct, hasInfo)
	}

	// 3. Quota none / untracked -> MUST return hasInfo=false (Empty grey ring, N/A)
	mNone := CustomModel{
		ID:        "test-none",
		QuotaType: QuotaNone,
	}
	pct, hasInfo = mNone.CalculatePercentage()
	if hasInfo {
		t.Errorf("quota none: expected hasInfo=false, got (%d, %v)", pct, hasInfo)
	}
}

func TestCustomModel_Validate(t *testing.T) {
	m := CustomModel{
		ID:      "",
		Name:    "gpt-4o",
		BaseURL: "https://api.openai.com/v1",
	}
	if err := m.Validate(); err == nil {
		t.Errorf("expected error for empty ID")
	}

	m.ID = "gpt4o"
	m.Name = ""
	if err := m.Validate(); err == nil {
		t.Errorf("expected error for empty Name")
	}

	m.Name = "gpt-4o"
	m.BaseURL = ""
	if err := m.Validate(); err == nil {
		t.Errorf("expected error for empty BaseURL")
	}

	m.BaseURL = "https://api.openai.com/v1"
	if err := m.Validate(); err != nil {
		t.Errorf("expected valid model, got: %v", err)
	}
	if m.DisplayName != "gpt-4o" {
		t.Errorf("expected DisplayName to default to Name")
	}
	if len(m.ProjectMappings) == 0 || m.ProjectMappings[0] != "*" {
		t.Errorf("expected default project mappings to [*]")
	}
}

func TestStore_CRUDAndBindings(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Store should initialize clean and empty with no dummy models
	initialModels := store.ListModels()
	if len(initialModels) != 0 {
		t.Errorf("expected clean empty models list, got %d models", len(initialModels))
	}

	// Add new custom model
	newModel := CustomModel{
		ID:              "custom-deepseek",
		Name:            "deepseek-chat",
		DisplayName:     "DeepSeek Chat V3",
		ProviderType:    ProviderOpenAI,
		BaseURL:         "https://api.deepseek.com/v1",
		APIKey:          "sk-deepseek-test",
		ProjectMappings: []string{"Project Alpha", "Project Beta"},
		QuotaType:       QuotaCostBased,
		PrepaidBalance:  10.0,
		TotalBudget:     20.0,
		Enabled:         true,
	}

	if err := store.SaveModel(newModel); err != nil {
		t.Fatalf("failed to save model: %v", err)
	}

	// Retrieve model
	m, err := store.GetModel("custom-deepseek")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}
	if m.DisplayName != "DeepSeek Chat V3" {
		t.Errorf("expected display name %q, got %q", "DeepSeek Chat V3", m.DisplayName)
	}

	// Test project matching
	matched := store.GetModelForProject("Project Alpha")
	if matched == nil || matched.ID != "custom-deepseek" {
		t.Errorf("expected custom-deepseek for Project Alpha")
	}

	// Test explicit project binding
	if err := store.BindProject("SpecialProject", "custom-deepseek"); err != nil {
		t.Fatalf("failed to bind project: %v", err)
	}

	bound := store.GetModelForProject("SpecialProject")
	if bound == nil || bound.ID != "custom-deepseek" {
		t.Errorf("expected custom-deepseek for bound SpecialProject")
	}

	// Test persistence across reloads
	store2, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	mReloaded, err := store2.GetModel("custom-deepseek")
	if err != nil || mReloaded.DisplayName != "DeepSeek Chat V3" {
		t.Errorf("failed to reload custom model: %v", err)
	}

	// Test deletion
	if err := store2.DeleteModel("custom-deepseek"); err != nil {
		t.Fatalf("failed to delete model: %v", err)
	}
	if _, err := store2.GetModel("custom-deepseek"); err == nil {
		t.Errorf("expected error getting deleted model")
	}
}

func TestGetPresets(t *testing.T) {
	presets := GetPresets()
	if len(presets) < 4 {
		t.Errorf("expected at least 4 presets, got %d", len(presets))
	}

	foundOpenAI := false
	foundAnthropic := false
	foundGemini := false
	foundLocal := false

	for _, p := range presets {
		if p.ProviderType == ProviderOpenAI {
			foundOpenAI = true
		}
		if p.ProviderType == ProviderAnthropic {
			foundAnthropic = true
		}
		if p.ProviderType == ProviderGemini {
			foundGemini = true
		}
		if p.ID == "ollama" || p.ID == "vllm" {
			foundLocal = true
		}
	}

	if !foundOpenAI || !foundAnthropic || !foundGemini || !foundLocal {
		t.Errorf("missing core provider presets: openai=%v anthropic=%v gemini=%v local=%v",
			foundOpenAI, foundAnthropic, foundGemini, foundLocal)
	}
}

func TestTester_EndpointResponses(t *testing.T) {
	// 1. Success mock server
	tsSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
	}))
	defer tsSuccess.Close()

	tester := NewTester()
	res, err := tester.TestEndpoint(CustomModel{
		ID:           "test-srv",
		Name:         "test-model",
		ProviderType: ProviderOpenAI,
		BaseURL:      tsSuccess.URL,
		APIKey:       "test-key",
	})
	if err != nil || !res.Success {
		t.Errorf("expected successful test, got res=%+v, err=%v", res, err)
	}

	// 2. Auth error mock server
	tsAuthFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_api_key"}`))
	}))
	defer tsAuthFail.Close()

	resFail, err := tester.TestEndpoint(CustomModel{
		ID:           "test-auth-fail",
		Name:         "test-model",
		ProviderType: ProviderOpenAI,
		BaseURL:      tsAuthFail.URL,
		APIKey:       "bad-key",
	})
	if err != nil || resFail.Success {
		t.Errorf("expected failed auth, got res=%+v", resFail)
	}
	if resFail.StatusCode != 401 {
		t.Errorf("expected 401 status code, got %d", resFail.StatusCode)
	}
}

func TestGenerateCustomModelsScript_CategoryHeaders(t *testing.T) {
	cfg := &Config{
		Models: []CustomModel{
			{
				ID:           "test-model",
				Name:         "gpt-4o",
				DisplayName:  "GPT-4o",
				ProviderType: ProviderOpenAI,
				Enabled:      true,
			},
		},
	}
	script := GenerateCustomModelsScript(cfg)

	// Verify "Native Model" header renaming logic is present
	if !containsSubstring(script, `"Native Model"`) {
		t.Errorf("expected script to contain 'Native Model', got none")
	}

	// Verify "Custom Model" header text is present
	if !containsSubstring(script, `customHeader.textContent = "Custom Model"`) {
		t.Errorf("expected script to set customHeader.textContent to 'Custom Model'")
	}

	// Verify model-selector-header data-testid lookup is present
	if !containsSubstring(script, `model-selector-header`) {
		t.Errorf("expected script to target 'model-selector-header'")
	}
}

func TestResolveEndpoint_Custom(t *testing.T) {
	raw := "https://my-custom-proxy.internal/v1/fast/chat"
	res := ResolveEndpoint(ProviderCustom, raw, "custom-model")
	if res != raw {
		t.Errorf("expected ResolveEndpoint to return exact raw URL %q, got %q", raw, res)
	}
}

func TestDetectModelMetadata(t *testing.T) {
	// Standard model with default 1000000 fallback
	ctx, thinking, _ := detectModelMetadata("some-unknown-model", "", 0)
	if ctx != 1000000 {
		t.Errorf("expected default context window 1000000, got %d", ctx)
	}
	if thinking {
		t.Errorf("expected supports_thinking=false for unknown model")
	}

	// Reasoning model: claude-3-7-sonnet
	ctx2, thinking2, levels2 := detectModelMetadata("claude-3-7-sonnet-20250219", "Claude 3.7", 0)
	if !thinking2 || len(levels2) == 0 {
		t.Errorf("expected claude-3-7 to support thinking, got %v, levels: %v", thinking2, levels2)
	}
	if ctx2 != 200000 {
		t.Errorf("expected claude-3-7 context 200000, got %d", ctx2)
	}

	// DeepSeek R1
	_, thinking3, _ := detectModelMetadata("deepseek-r1", "DeepSeek R1", 0)
	if !thinking3 {
		t.Errorf("expected deepseek-r1 to support thinking")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && stringSearch(s, substr)))
}

func stringSearch(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
