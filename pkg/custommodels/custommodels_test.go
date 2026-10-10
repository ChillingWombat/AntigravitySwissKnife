package custommodels

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	if strings.Contains(res.Message, "200") {
		t.Errorf("expected success message not to include status code 200, got: %s", res.Message)
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

	// 3. HTTP 400 Bad Request error mock server - MUST NOT be treated as success!
	ts400 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Model deepseek-v4-flash is unavailable"}}`))
	}))
	defer ts400.Close()

	res400, err := tester.TestEndpoint(CustomModel{
		ID:           "test-400",
		Name:         "deepseek-v4-flash",
		ProviderType: ProviderOpenAI,
		BaseURL:      ts400.URL,
		APIKey:       "test-key",
	})
	if err != nil {
		t.Errorf("expected no transport error, got %v", err)
	}
	if res400.Success {
		t.Errorf("expected HTTP 400 to fail with Success=false, but got Success=true!")
	}
	if res400.StatusCode != 400 {
		t.Errorf("expected StatusCode=400, got %d", res400.StatusCode)
	}
	if !strings.Contains(res400.Message, "Model deepseek-v4-flash is unavailable") {
		t.Errorf("expected error message to be extracted, got: %s", res400.Message)
	}
}

func TestFetchModels_SubpathV1Candidate(t *testing.T) {
	// Server serves /zen/go/v1/models with 200, but /zen/go/models with 404
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/zen/go/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-v4-flash"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>404 Not Found</body></html>`))
	}))
	defer ts.Close()

	res := FetchModels(FetchModelsRequest{
		ProviderType: ProviderOpenAI,
		BaseURL:      ts.URL + "/zen/go",
		APIKey:       "test-key",
	})
	if !res.Success {
		t.Fatalf("expected successful fetch via /v1/models candidate, got: %s", res.Message)
	}
	if len(res.Models) != 1 || res.Models[0].ID != "deepseek-v4-flash" {
		t.Errorf("expected deepseek-v4-flash model, got: %+v", res.Models)
	}
}

func TestFetchModels_SanitizesHTMLError(t *testing.T) {
	// Server returns HTML 404 for all paths
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html lang="en"><head><title>Not Found</title></head><body><h1>404 Not Found</h1></body></html>`))
	}))
	defer ts.Close()

	res := FetchModels(FetchModelsRequest{
		ProviderType: ProviderOpenAI,
		BaseURL:      ts.URL + "/bad-path",
	})
	if res.Success {
		t.Errorf("expected failure for 404 HTML")
	}
	if strings.Contains(res.Message, "<html") || strings.Contains(res.Message, "<!DOCTYPE") {
		t.Errorf("raw HTML was leaked in error message: %s", res.Message)
	}
	if !strings.Contains(res.Message, "404") {
		t.Errorf("expected 404 in error message, got: %s", res.Message)
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

func TestGenerateCustomModelsScript_NoRedundantGeminiReasoningSelector(t *testing.T) {
	script := GenerateCustomModelsScript(nil)
	// Verify it cleans up stale selector if present
	if !containsSubstring(script, "staleGeminiSelector") {
		t.Errorf("expected script to clean up staleGeminiSelector")
	}
	// Verify it does not inject a redundant Reasoning: dropdown
	if containsSubstring(script, `"Reasoning: "`) {
		t.Errorf("expected script NOT to contain redundant 'Reasoning: ' dropdown")
	}
}

func TestResolveEndpoint_Custom(t *testing.T) {
	raw := "https://my-custom-proxy.internal/v1/fast/chat"
	res := ResolveEndpoint(ProviderCustom, raw, "custom-model")
	if res != raw {
		t.Errorf("expected ResolveEndpoint to return exact raw URL %q, got %q", raw, res)
	}

	base := "https://opencode.ai/zen/go"
	resBase := ResolveEndpoint(ProviderCustom, base, "custom-model")
	expected := "https://opencode.ai/zen/go/v1/chat/completions"
	if resBase != expected {
		t.Errorf("expected ResolveEndpoint to resolve %q to %q, got %q", base, expected, resBase)
	}
}

func TestDetectModelMetadata(t *testing.T) {
	// Standard model with default 1048576 fallback
	ctx, thinking, _ := detectModelMetadata("some-unknown-model", "", 0)
	if ctx != 1048576 {
		t.Errorf("expected default context window 1048576, got %d", ctx)
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

func TestAuditor_RunAuditUsesModelNameForModelID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"PONG"},"finish_reason":"stop"}]}`))
	}))
	defer ts.Close()

	auditor := NewAuditor()
	report := auditor.RunAudit(CustomModel{
		ID:           "openai-deepseek-chat-1759989123456",
		Name:         "deepseek-chat",
		DisplayName:  "DeepSeek V3",
		ProviderType: ProviderOpenAI,
		BaseURL:      ts.URL,
		APIKey:       "sk-test",
	})

	if report.ModelID != "deepseek-chat" {
		t.Errorf("expected report.ModelID to be clean model Name %q, got %q", "deepseek-chat", report.ModelID)
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

func TestFetchModelPricing_HierarchyAndNullFallback(t *testing.T) {
	origOR := OpenRouterModelsURL
	origLite := LiteLLMPricingURL
	defer func() {
		OpenRouterModelsURL = origOR
		LiteLLMPricingURL = origLite
		ResetThirdPartyPricingCache()
	}()

	// Mock 3rd-party backup server (OpenRouter format)
	thirdPartyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen/qwen-2.5-coder-32b","pricing":{"prompt":"0.0000002","completion":"0.0000006","input_cache_read":"0.00000005"}}]}`))
	}))
	defer thirdPartyServer.Close()

	noOpLiteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer noOpLiteServer.Close()

	OpenRouterModelsURL = thirdPartyServer.URL
	LiteLLMPricingURL = noOpLiteServer.URL
	ResetThirdPartyPricingCache()

	// 1A. Live Provider Endpoint takes top priority
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"my-provider-model","input_price_per_m":1.75,"cached_input_price_per_m":0.40,"output_price_per_m":7.00}]}`))
	}))
	defer providerServer.Close()

	inP, cacheP, outP, src := FetchModelPricing(t.Context(), "my-provider-model", "custom", providerServer.URL, "sk-test")
	if src != PriceSourceProvider {
		t.Fatalf("expected source %q from live provider endpoint, got %q", PriceSourceProvider, src)
	}
	if inP == nil || *inP != 1.75 || cacheP == nil || *cacheP != 0.40 || outP == nil || *outP != 7.00 {
		t.Fatalf("unexpected provider prices: in=%v cache=%v out=%v", inP, cacheP, outP)
	}

	// 1B. Official Provider Specification (Google Gemini 3.8 Flash)
	inP, cacheP, outP, src = FetchModelPricing(t.Context(), "gemini-3.8-flash", "gemini", "", "")
	if src != PriceSourceProvider || inP == nil || outP == nil {
		t.Fatalf("expected official provider pricing for gemini-3.8-flash, got src=%q in=%v out=%v", src, inP, outP)
	}

	// 2. 3rd-Party Backup when not in provider
	inP, cacheP, outP, src = FetchModelPricing(t.Context(), "qwen-2.5-coder-32b", "custom", "", "")
	if src != PriceSourceThirdParty {
		t.Fatalf("expected source %q from 3rd-party backup, got %q", PriceSourceThirdParty, src)
	}
	if inP == nil || *inP < 0.19 || *inP > 0.21 || outP == nil || *outP < 0.59 || *outP > 0.61 {
		t.Fatalf("unexpected 3rd-party prices: in=%v out=%v", inP, outP)
	}

	// 3. Unknown model -> null (nil pointers) and source "unconfigured"
	inP, cacheP, outP, src = FetchModelPricing(t.Context(), "totally-nonexistent-private-model-xyz", "custom", "", "")
	if src != PriceSourceUnconfigured {
		t.Fatalf("expected source %q for unknown model, got %q", PriceSourceUnconfigured, src)
	}
	if inP != nil || cacheP != nil || outP != nil {
		t.Fatalf("expected nil price pointers for unknown model, got in=%v cache=%v out=%v", inP, cacheP, outP)
	}
}

func TestSyncPricingWithCatalog_ThinkingLevelDeduplicationAndOutdatedLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Initial catalog has multiple thinking levels for Gemini 3.8 Flash and Gemini 3.1 Pro
	initialCatalog := []CatalogModelInput{
		{ID: "gemini-3.8-flash-high", DisplayName: "Gemini 3.8 Flash (High)", Provider: "Google"},
		{ID: "gemini-3.8-flash-medium", DisplayName: "Gemini 3.8 Flash (Medium)", Provider: "Google"},
		{ID: "gemini-3.8-flash-low", DisplayName: "Gemini 3.8 Flash (Low)", Provider: "Google"},
		{ID: "gemini-pro-agent", DisplayName: "Gemini 3.1 Pro (High)", Provider: "Google"},
		{ID: "gemini-3.1-pro-low", DisplayName: "Gemini 3.1 Pro (Low)", Provider: "Google"},
	}

	records := store.SyncPricingWithCatalog(t.Context(), initialCatalog, false)
	if len(records) != 2 {
		t.Fatalf("expected 2 deduplicated canonical models, got %d: %+v", len(records), records)
	}
	if records[0].ModelID != "gemini-3.8-flash" || records[0].Name != "Gemini 3.8 Flash" || records[0].Classification != ClassificationNative {
		t.Errorf("unexpected first record: %+v", records[0])
	}
	if records[1].ModelID != "gemini-3.1-pro" || records[1].Name != "Gemini 3.1 Pro" || records[1].Classification != ClassificationNative {
		t.Errorf("unexpected second record: %+v", records[1])
	}
	if records[0].InternalID <= 0 || records[1].InternalID <= records[0].InternalID {
		t.Errorf("expected strictly increasing InternalIDs, got %d and %d", records[0].InternalID, records[1].InternalID)
	}

	// Subsequent catalog removes Gemini 3.1 Pro; it must NOT be deleted automatically,
	// but reclassified as "custom" (IsOutdatedNative=true) in Token Monitor while not appearing in Custom Models list.
	updatedCatalog := []CatalogModelInput{
		{ID: "gemini-3.8-flash-high", DisplayName: "Gemini 3.8 Flash (High)", Provider: "Google"},
	}
	records2 := store.SyncPricingWithCatalog(t.Context(), updatedCatalog, false)
	if len(records2) != 2 {
		t.Fatalf("expected outdated native model to be retained (2 total), got %d", len(records2))
	}
	if records2[0].Classification != ClassificationNative || records2[0].IsOutdatedNative {
		t.Errorf("expected gemini-3.8-flash to remain active native, got %+v", records2[0])
	}
	if records2[1].Classification != ClassificationCustom || !records2[1].IsOutdatedNative {
		t.Errorf("expected gemini-3.1-pro to be reclassified as custom (outdated native), got %+v", records2[1])
	}
	if len(store.ListModels()) != 0 {
		t.Errorf("outdated native model must not appear in Custom Models page list, got %d models", len(store.ListModels()))
	}

	// Active native model cannot be deleted
	if err := store.DeletePricingModel(records2[0].InternalID, records2[0].ModelID); err == nil {
		t.Errorf("expected error when attempting to delete active native model")
	}

	// Outdated native model (classified as custom) CAN be manually deleted in Token Monitor
	if err := store.DeletePricingModel(records2[1].InternalID, records2[1].ModelID); err != nil {
		t.Fatalf("expected outdated native model deletion to succeed, got: %v", err)
	}
	if !store.IsModelDeleted(records2[1].InternalID, "gemini-3.1-pro-low", "") {
		t.Errorf("expected deleted outdated native model (and its thinking variants) to be marked deleted")
	}
	if len(store.ListPricingRecords()) != 1 {
		t.Errorf("expected 1 pricing record remaining after deletion, got %d", len(store.ListPricingRecords()))
	}
}

func TestSharedPriceDataAndMonotonicInternalID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	inP := 0.50
	cacheP := 0.10
	outP := 2.00
	capVal := 25.0

	cm := CustomModel{
		ID:                   "custom-ds-1",
		Name:                 "deepseek-chat",
		DisplayName:          "DeepSeek V3 Custom",
		ProviderType:         ProviderOpenAI,
		BaseURL:              "https://api.deepseek.com/v1",
		QuotaType:            QuotaTypeBalance,
		QuotaManualOverride:  true,
		BalanceValue:         "$40.00",
		BudgetCapType:        "dollar",
		BudgetCapValue:       &capVal,
		InputPricePerM:       &inP,
		CachedInputPricePerM: &cacheP,
		OutputPricePerM:      &outP,
		PriceSource:          PriceSourceManual,
		Enabled:              true,
	}
	if err := store.SaveModel(cm); err != nil {
		t.Fatalf("SaveModel failed: %v", err)
	}

	saved, err := store.GetModel("custom-ds-1")
	if err != nil || saved == nil {
		t.Fatalf("GetModel failed: %v", err)
	}
	firstInternalID := saved.InternalID
	if firstInternalID <= 0 {
		t.Fatalf("expected positive InternalID, got %d", firstInternalID)
	}
	if saved.BudgetCapType != "dollar" || saved.BudgetCapValue == nil || *saved.BudgetCapValue != 25.0 {
		t.Errorf("expected budget cap dollar=$25.0, got type=%q val=%v", saved.BudgetCapType, saved.BudgetCapValue)
	}

	// Verify PricingRecords has the exact same piece of data and InternalID
	pRecs := store.ListPricingRecords()
	if len(pRecs) != 1 {
		t.Fatalf("expected 1 pricing record synced from SaveModel, got %d", len(pRecs))
	}
	if pRecs[0].InternalID != firstInternalID || *pRecs[0].InputPricePerM != 0.50 || pRecs[0].Source != PriceSourceManual {
		t.Errorf("pricing record out of sync with custom model: %+v", pRecs[0])
	}

	// Update price from Token Price page -> must update CustomModel too
	newIn := 0.75
	newCache := 0.15
	newOut := 3.00
	_, err = store.UpdateModelPricing(UpdatePricingRequest{
		InternalID:           firstInternalID,
		ModelID:              "deepseek-chat",
		InputPricePerM:       &newIn,
		CachedInputPricePerM: &newCache,
		OutputPricePerM:      &newOut,
	})
	if err != nil {
		t.Fatalf("UpdateModelPricing failed: %v", err)
	}

	savedAfter, _ := store.GetModel("custom-ds-1")
	if savedAfter.InputPricePerM == nil || *savedAfter.InputPricePerM != 0.75 || *savedAfter.OutputPricePerM != 3.00 {
		t.Errorf("expected CustomModel price to reflect Token Price edit, got in=%v out=%v", savedAfter.InputPricePerM, savedAfter.OutputPricePerM)
	}

	// Delete CustomModel -> removes from PricingRecords and marks deleted
	if err := store.DeleteModel("custom-ds-1"); err != nil {
		t.Fatalf("DeleteModel failed: %v", err)
	}
	if len(store.ListPricingRecords()) != 0 {
		t.Errorf("expected 0 pricing records after deleting custom model, got %d", len(store.ListPricingRecords()))
	}
	if !store.IsModelDeleted(firstInternalID, "deepseek-chat", "custom-ds-1") {
		t.Errorf("expected deleted custom model to be marked deleted")
	}

	// Adding a new model must receive a strictly higher InternalID
	cm2 := CustomModel{
		ID:           "custom-ds-2",
		Name:         "deepseek-reasoner",
		ProviderType: ProviderOpenAI,
		BaseURL:      "https://api.deepseek.com/v1",
		Enabled:      true,
	}
	if err := store.SaveModel(cm2); err != nil {
		t.Fatalf("SaveModel 2 failed: %v", err)
	}
	saved2, _ := store.GetModel("custom-ds-2")
	if saved2.InternalID <= firstInternalID {
		t.Errorf("expected strictly increasing InternalID > %d, got %d", firstInternalID, saved2.InternalID)
	}
}

func TestCustomModel_InferenceSupportedContract(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.InferenceSupported {
		t.Fatalf("expected DefaultConfig() to have InferenceSupported true")
	}

	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	storeCfg := store.GetConfig()
	if !storeCfg.InferenceSupported {
		t.Fatalf("expected store.GetConfig() to have InferenceSupported true")
	}

	script := GenerateCustomModelsScript(nil)
	if !containsSubstring(script, "inference_supported: true") {
		t.Errorf("expected generated script to contain 'inference_supported: true'")
	}
}


