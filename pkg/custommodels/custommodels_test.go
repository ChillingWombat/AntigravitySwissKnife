package custommodels

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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

	// GCP model auto-derives BaseURL
	mGCP := CustomModel{
		ID:           "test-gcp",
		Name:         "gemini-2.5-flash",
		ProviderType: ProviderGCP,
		GCPProjectID: "my-gcp-proj",
		GCPRegion:    "us-central1",
	}
	if err := mGCP.Validate(); err != nil {
		t.Errorf("expected valid GCP model with auto BaseURL, got: %v", err)
	}
	if !strings.Contains(mGCP.BaseURL, "us-central1-aiplatform.googleapis.com") {
		t.Errorf("expected generated Vertex AI BaseURL, got %s", mGCP.BaseURL)
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
		ID:             "custom-deepseek",
		Name:           "deepseek-chat",
		DisplayName:    "DeepSeek Chat V3",
		ProviderType:   ProviderOpenAI,
		BaseURL:        "https://api.deepseek.com/v1",
		APIKey:         "sk-deepseek-test",
		QuotaType:      QuotaCostBased,
		PrepaidBalance: 10.0,
		TotalBudget:    20.0,
		Enabled:        true,
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

	// The active model is app-wide: models are never bound to projects.
	if err := store.SetActiveModel("custom-deepseek"); err != nil {
		t.Fatalf("failed to set active model: %v", err)
	}
	if active := store.GetConfig().ActiveModelID; active != "custom-deepseek" {
		t.Errorf("expected active model custom-deepseek, got %q", active)
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

func TestStore_SetActiveModel(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	m := CustomModel{
		ID:           "backup-model",
		Name:         "backup-llm",
		DisplayName:  "Backup LLM",
		ProviderType: ProviderOpenAI,
		BaseURL:      "https://api.openai.com/v1",
		Enabled:      true,
	}
	if err := store.SaveModel(m); err != nil {
		t.Fatalf("failed to save model: %v", err)
	}

	// Selecting by ID and by name both resolve.
	if err := store.SetActiveModel("backup-model"); err != nil {
		t.Fatalf("SetActiveModel by id: %v", err)
	}
	if got := store.GetConfig().ActiveModelID; got != "backup-model" {
		t.Errorf("expected active model backup-model, got %q", got)
	}

	// "native" and "" both hand the session back to the native model.
	if err := store.SetActiveModel("native"); err != nil {
		t.Fatalf("SetActiveModel native: %v", err)
	}
	if got := store.GetConfig().ActiveModelID; got != "" {
		t.Errorf("native must clear the active model, got %q", got)
	}

	// Unknown models are rejected instead of silently stored.
	if err := store.SetActiveModel("does-not-exist"); err == nil {
		t.Error("expected an error when activating an unknown model")
	}
	if got := store.GetConfig().ActiveModelID; got != "" {
		t.Errorf("a rejected activation must not change the active model, got %q", got)
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
	tester.AllowLoopback = true
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
	// Verify thinking level selector is rendered inside the model selector flyout submenu
	// rather than as a separate composer pill, and uses the factory Antigravity SVG tick and chevron.
	if !containsSubstring(script, "swiss-custom-thinking-submenu") {
		t.Errorf("expected script to contain in-selector thinking submenu 'swiss-custom-thinking-submenu'")
	}
	if !containsSubstring(script, `model-selector-effort-option`) {
		t.Errorf("expected script to render 'model-selector-effort-option' items inside the model selector submenu")
	}
	if !containsSubstring(script, "M382-339.38L727.54-684.92") {
		t.Errorf("expected script to use factory Antigravity CheckIcon SVG path")
	}
	if !containsSubstring(script, "M517.85-480l-184-184") {
		t.Errorf("expected script to use factory Antigravity ChevronRightIcon SVG path")
	}
	if containsSubstring(script, "Thinking:</span>") {
		t.Errorf("expected script NOT to create an external 'Thinking:' composer pill")
	}
	if containsSubstring(script, `cleanActiveName + " \u25be"`) || containsSubstring(script, `cleanDisplayName + " \u25be"`) {
		t.Errorf("expected script NOT to append duplicate '\\u25be' triangle to trigger label")
	}
	// Verify popover selector excludes the thinking submenu
	if !containsSubstring(script, `:not(#swiss-custom-thinking-submenu)`) {
		t.Errorf("expected script to exclude #swiss-custom-thinking-submenu in popover query selector")
	}
	// Verify hover tunnel gap timer
	if !containsSubstring(script, "submenuCloseTimer") {
		t.Errorf("expected script to implement submenuCloseTimer for hover tunnel gap")
	}
}

func TestGenerateCustomModelsScript_SubmenuEdgeCases(t *testing.T) {
	script := GenerateCustomModelsScript(nil)

	// 1. [P1] Exclude #swiss-custom-thinking-submenu in popover selectors
	if !strings.Contains(script, `[role="menu"]:not(#swiss-custom-thinking-submenu)`) {
		t.Errorf("expected script to contain [role=\"menu\"]:not(#swiss-custom-thinking-submenu)")
	}

	// 2. [P2] Hover tunnel gap timer
	if !strings.Contains(script, "submenuCloseTimer") {
		t.Errorf("expected script to define submenuCloseTimer")
	}
	if !strings.Contains(script, "clearSubmenuCloseTimer") {
		t.Errorf("expected script to define clearSubmenuCloseTimer")
	}

	// 3. [P2] Viewport right-edge collision handling
	if !strings.Contains(script, "menuRect.right + 165 > window.innerWidth") {
		t.Errorf("expected script to compute viewport collision using window.innerWidth")
	}
	if !strings.Contains(script, "targetLeft") {
		t.Errorf("expected script to compute targetLeft for submenu placement")
	}

	// 4. [P3] Trigger label re-sync on React re-render & case-insensitive project bind lookup
	if !strings.Contains(script, `!(label.textContent || "").includes(cleanName)`) {
		t.Errorf("expected script to re-sync trigger label if textContent does not include cleanName")
	}
	if strings.Contains(script, "project_binds") {
		t.Errorf("custom models must not be bound to projects: project_binds is gone")
	}

	// 5. [P3] Immediate menu dismissal on selection before network fetch
	selectFnIdx := strings.Index(script, "async function selectCustomModelWithLevel")
	if selectFnIdx == -1 {
		t.Fatalf("expected script to contain selectCustomModelWithLevel function")
	}
	selectFn := script[selectFnIdx:]
	popoverRemoveIdx := strings.Index(selectFn, "popover.remove()")
	fetchActiveIdx := strings.Index(selectFn, "/api/custom_models/active")
	fetchThinkingIdx := strings.Index(selectFn, "/api/custom_models/thinking_level")
	if popoverRemoveIdx == -1 || fetchActiveIdx == -1 || fetchThinkingIdx == -1 {
		t.Errorf("missing expected statements in selectCustomModelWithLevel")
	}
	if popoverRemoveIdx > fetchActiveIdx || popoverRemoveIdx > fetchThinkingIdx {
		t.Errorf("expected popover.remove() to occur before awaiting fetch calls in selectCustomModelWithLevel, got popoverRemoveIdx=%d fetchActiveIdx=%d fetchThinkingIdx=%d", popoverRemoveIdx, fetchActiveIdx, fetchThinkingIdx)
	}
}

func TestGenerateCustomModelsScript_NativeModelUnbindCapture(t *testing.T) {
	script := GenerateCustomModelsScript(nil)

	helperIdx := strings.Index(script, "function clearSessionCustomModel()")
	if helperIdx == -1 {
		t.Fatalf("expected script to define clearSessionCustomModel")
	}
	helperEndIdx := strings.Index(script[helperIdx:], "// 1. Hook Model Selector Trigger Pill & Dropdown")
	if helperEndIdx == -1 {
		t.Fatalf("expected clearSessionCustomModel helper to precede updateModelSelector")
	}
	helperScript := script[helperIdx : helperIdx+helperEndIdx]
	if strings.Contains(helperScript, "project_binds") {
		t.Error("clearSessionCustomModel must not touch project bindings")
	}
	for _, want := range []string{
		`customConfig.active_model_id = "";`,
		`trigger.removeAttribute("data-swiss-custom-bound");`,
		`trigger.removeAttribute("data-swiss-native-label");`,
		`body: JSON.stringify({ model_id: "" })`,
		`/api/custom_models/active`,
	} {
		if !strings.Contains(helperScript, want) {
			t.Errorf("expected unbind helper to contain %q", want)
		}
	}

	nativeListenerIdx := strings.Index(script, `it.addEventListener("click", async () => {`)
	if nativeListenerIdx == -1 {
		t.Fatalf("expected native model menu item click listener")
	}
	nativeListenerEndIdx := strings.Index(script[nativeListenerIdx:], "// Find bottom action")
	if nativeListenerEndIdx == -1 {
		t.Fatalf("expected native model click listener to precede bottom action detection")
	}
	nativeListener := script[nativeListenerIdx : nativeListenerIdx+nativeListenerEndIdx]
	if !strings.Contains(nativeListener, "clearSessionCustomModel();") {
		t.Errorf("expected native model click listener to clear the custom model selection")
	}
	if !strings.Contains(nativeListener, "}, true);") {
		t.Errorf("expected native model click listener to use capture phase")
	}

	captureListenerIdx := strings.Index(script, `if (!window.__swissNativeModelCaptureBound) {`)
	if captureListenerIdx == -1 {
		t.Fatalf("expected document-level capture click listener")
	}
	captureListenerEndIdx := strings.Index(script[captureListenerIdx:], "// 1. Hook Model Selector Trigger Pill & Dropdown")
	if captureListenerEndIdx == -1 {
		t.Fatalf("expected document-level capture listener to precede updateModelSelector")
	}
	captureListener := script[captureListenerIdx : captureListenerIdx+captureListenerEndIdx]
	for _, want := range []string{
		`window.__swissNativeModelCaptureBound`,
		`[data-testid="model-selector-effort-option"]`,
		`[data-testid="model-selector-context-window-option"]`,
		`[data-testid="model-selector-item"]`,
		`[data-testid="model-selector-effort-group"]`,
		`[data-testid="model-selector-panel"] [role="menuitem"]`,
		`target.closest(".swiss-custom-models-menu-group")`,
		`target.closest("#swiss-custom-thinking-submenu")`,
		`(clickedItem.textContent || "").trim() === "View Usage"`,
		`clearSessionCustomModel();`,
		`}, true);`,
	} {
		if !strings.Contains(captureListener, want) {
			t.Errorf("expected native model capture listener to contain %q", want)
		}
	}

	checkIdx := strings.Index(script, `it.querySelectorAll("svg").forEach(svg => {`)
	if checkIdx == -1 {
		t.Fatalf("expected native model SVG checkmark detection")
	}
	checkEndIdx := strings.Index(script[checkIdx:], `if (!it.__swissNativeBound`)
	if checkEndIdx == -1 {
		t.Fatalf("expected SVG checkmark detection to precede native item binding")
	}
	checkScript := script[checkIdx : checkIdx+checkEndIdx]
	for _, want := range []string{
		`path.getAttribute("d")`,
		`(svg.innerHTML || "").includes("M382-339.38")`,
		`pathData.includes("M382-339.38")`,
		`check.style.display = activeCustomModel ? "none" : "";`,
	} {
		if !strings.Contains(checkScript, want) {
			t.Errorf("expected native checkmark hiding to contain %q", want)
		}
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
	expected := "https://opencode.ai/inference/go/openai/v1/chat/completions"
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
	if *inP != 0.75 || *outP != 3.75 || cacheP == nil || *cacheP != 0.075 {
		t.Fatalf("expected official pricing 0.75/0.075/3.75 for gemini-3.8-flash, got in=%v cache=%v out=%v", inP, cacheP, outP)
	}

	// 2. 3rd-Party Backup when not in provider
	inP, _, outP, src = FetchModelPricing(t.Context(), "qwen-2.5-coder-32b", "custom", "", "")
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

func TestFetchModelPricing_DynamicGoogleGeminiPricing(t *testing.T) {
	origGoogle := GoogleGeminiPricingURL
	origDocs := GoogleGeminiDocsPricingURL
	defer func() {
		GoogleGeminiPricingURL = origGoogle
		GoogleGeminiDocsPricingURL = origDocs
		ResetGoogleGeminiPricingCache()
	}()

	mockHTML := `<!doctype html>
<html>
<body>
<div class="models-section">
  <h2 id="gemini-3.8-flash" data-text="Gemini 3.8 Flash">Gemini 3.8 Flash</h2>
  <em><a href="/gemini-api/docs/models/gemini-3.8-flash"><code>gemini-3.8-flash</code></a></em>
</div>
<table class="pricing-table">
  <thead>
    <tr><th></th><th>Free Tier</th><th>Paid Tier</th></tr>
  </thead>
  <tbody>
    <tr><td>Input price</td><td>Free</td><td>$0.75 through December 31, 2026.</td></tr>
    <tr><td>Output price (including thinking)</td><td>Free</td><td>$3.75 per 1M tokens.</td></tr>
    <tr><td>Context caching price</td><td>Free</td><td>$0.075 per 1M tokens.</td></tr>
  </tbody>
</table>

<div class="models-section">
  <h2 id="gemini-3.1-pro-preview">Gemini 3.1 Pro Preview</h2>
  <em><code>gemini-3.1-pro</code></em>
</div>
<table class="pricing-table">
  <tbody>
    <tr><td>Input price</td><td>Free</td><td>$2.00</td></tr>
    <tr><td>Output price</td><td>Free</td><td>$12.00</td></tr>
    <tr><td>Context caching price</td><td>Free</td><td>$0.20</td></tr>
  </tbody>
</table>
</body>
</html>`

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(mockHTML))
	}))
	defer mockServer.Close()

	GoogleGeminiPricingURL = mockServer.URL
	GoogleGeminiDocsPricingURL = mockServer.URL
	ResetGoogleGeminiPricingCache()

	// 1. Verify dynamic fetch and parsing of Gemini 3.8 Flash
	inP, cacheP, outP, src := FetchModelPricing(t.Context(), "gemini-3.8-flash", "gemini", "", "")
	if src != PriceSourceProvider {
		t.Fatalf("expected PriceSourceProvider, got %q", src)
	}
	if inP == nil || *inP != 0.75 || cacheP == nil || *cacheP != 0.075 || outP == nil || *outP != 3.75 {
		t.Fatalf("unexpected dynamic prices for gemini-3.8-flash: in=%v cache=%v out=%v", inP, cacheP, outP)
	}

	// 2. Verify thinking / tiered alias: gemini-3.8-flash-high maps to dynamic gemini-3.8-flash
	inP, cacheP, outP, src = FetchModelPricing(t.Context(), "gemini-3.8-flash-high", "gemini", "", "")
	if src != PriceSourceProvider || inP == nil || *inP != 0.75 || cacheP == nil || *cacheP != 0.075 || outP == nil || *outP != 3.75 {
		t.Fatalf("unexpected dynamic prices for gemini-3.8-flash-high: in=%v cache=%v out=%v", inP, cacheP, outP)
	}

	// 3. Verify Gemini 3.1 Pro
	inP, cacheP, outP, src = FetchModelPricing(t.Context(), "gemini-3.1-pro", "gemini", "", "")
	if src != PriceSourceProvider || inP == nil || *inP != 2.00 || cacheP == nil || *cacheP != 0.20 || outP == nil || *outP != 12.00 {
		t.Fatalf("unexpected dynamic prices for gemini-3.1-pro: in=%v cache=%v out=%v", inP, cacheP, outP)
	}

	// 4. Test offline fallback to official specifications.
	// The bundled table is the LAST resort, so the live third-party catalogs must
	// also be unavailable for this assertion to reach it (they used to sit below
	// the bundled table; the hierarchy now puts every dynamic source first).
	mockServer.Close()
	origOpenRouter, origLiteLLM := OpenRouterModelsURL, LiteLLMPricingURL
	emptyCatalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer emptyCatalog.Close()
	defer func() {
		OpenRouterModelsURL, LiteLLMPricingURL = origOpenRouter, origLiteLLM
	}()
	OpenRouterModelsURL = emptyCatalog.URL
	LiteLLMPricingURL = ""
	thirdPartyCacheMu.Lock()
	thirdPartyCache = nil
	thirdPartyCacheAt = time.Time{}
	thirdPartyCacheMu.Unlock()
	ResetGoogleGeminiPricingCache()

	inP, cacheP, outP, src = FetchModelPricing(t.Context(), "gemini-3.8-flash", "gemini", "", "")
	if src != PriceSourceProvider {
		t.Fatalf("expected PriceSourceProvider on fallback, got %q", src)
	}
	if inP == nil || *inP != 0.75 || cacheP == nil || *cacheP != 0.075 || outP == nil || *outP != 3.75 {
		t.Fatalf("unexpected fallback prices for gemini-3.8-flash: in=%v cache=%v out=%v", inP, cacheP, outP)
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
	if !containsSubstring(script, `"inference_supported":true`) {
		t.Errorf("expected generated script to contain '\"inference_supported\":true'")
	}

	scriptWithFalseCfg := GenerateCustomModelsScript(&Config{InferenceSupported: false})
	if !containsSubstring(scriptWithFalseCfg, `"inference_supported":true`) {
		t.Errorf("expected generated script with false config to enforce inference_supported true")
	}
}

func TestDetectAndFetchQuota_GCP(t *testing.T) {
	m := CustomModel{
		ProviderType: ProviderGCP,
		BaseURL:      "https://us-central1-aiplatform.googleapis.com",
		Name:         "gemini-2.5-flash",
	}
	res := DetectAndFetchQuota(m)
	if res.QuotaType != QuotaTypeQuota || res.QuotaValue != "GCP Managed" {
		t.Errorf("expected QuotaTypeQuota with 'GCP Managed', got type=%s value=%s", res.QuotaType, res.QuotaValue)
	}
}

func TestOpenCodeEndpointResolution(t *testing.T) {
	const want = "https://opencode.ai/inference/go/openai/v1/chat/completions"
	// The retired /zen/go base and the canonical URL both resolve to the current
	// inference endpoint, whichever provider type the entry was saved with.
	cases := []struct {
		provider ProviderType
		base     string
	}{
		{ProviderCustom, "https://opencode.ai/zen/go"},
		{ProviderOpenAI, "https://opencode.ai/zen/go"},
		{ProviderCustom, want},
		{ProviderCustom, "https://opencode.ai/inference/go/openai/v1"},
	}
	for _, tc := range cases {
		if got := ResolveEndpoint(tc.provider, tc.base, "deepseek-v4-flash"); got != want {
			t.Errorf("ResolveEndpoint(%s, %q) = %q, want %q", tc.provider, tc.base, got, want)
		}
	}
}

func TestOpenCodeQuotaLiveDetection(t *testing.T) {
	// Live network test: hits opencode.ai with a real key, so its outcome
	// depends on an external service (key validity, plan, rate limits,
	// egress rules). Opt in explicitly instead of failing every CI run.
	if os.Getenv("SWISS_LIVE_QUOTA_TESTS") != "1" {
		t.Skip("live quota detection against opencode.ai; set SWISS_LIVE_QUOTA_TESTS=1 to run")
	}
	apiKey := os.Getenv("OPENCODE_API_KEY")
	if apiKey == "" {
		apiKey = "oc_sk_da7cccd32ee8_Y87ZRKcQU8ErRhBnZRwPxd8IsGqmYkE2"
	}
	m := CustomModel{
		ProviderType: ProviderOpenAI,
		BaseURL:      "https://opencode.ai/zen/go",
		APIKey:       apiKey,
		Name:         "deepseek-v4-flash",
	}
	res := DetectAndFetchQuota(m)
	if res.QuotaType != QuotaTypeQuota {
		t.Errorf("expected OpenCode quota type 'quota', got %s", res.QuotaType)
	}
	if res.QuotaValue != "Sub Required" && !strings.Contains(res.QuotaValue, "models") {
		t.Errorf("expected 'Sub Required' or models count, got %s", res.QuotaValue)
	}
}

func TestGenerateCustomModelsScript_AuthHeaders(t *testing.T) {
	script := GenerateCustomModelsScript(nil)
	if !strings.Contains(script, "getSwissAuthHeaders") {
		t.Errorf("expected script to contain getSwissAuthHeaders function")
	}
	if !strings.Contains(script, "headers: getSwissAuthHeaders()") {
		t.Errorf("expected script to attach getSwissAuthHeaders to fetch calls")
	}
	if !strings.Contains(script, "X-Session-Token") {
		t.Errorf("expected getSwissAuthHeaders to set X-Session-Token")
	}
}

func TestGenerateCustomModelsScript_BackupModelAutoSelection(t *testing.T) {
	cfg := &Config{
		GeminiQuotaExhausted: true,
		DefaultCustomModel:   "backup-model-1",
		Models: []CustomModel{
			{
				ID:           "backup-model-1",
				Name:         "claude-3-7-sonnet",
				DisplayName:  "Claude 3.7 Sonnet",
				ProviderType: ProviderAnthropic,
				Enabled:      true,
			},
		},
	}
	script := GenerateCustomModelsScript(cfg)

	// Verify gemini_quota_exhausted check is embedded
	if !strings.Contains(script, "customConfig.gemini_quota_exhausted === true") {
		t.Errorf("expected script to verify gemini_quota_exhausted")
	}
	// Verify window.__swiss_session_custom_model_id check is embedded
	if !strings.Contains(script, "window.__swiss_session_custom_model_id") {
		t.Errorf("expected script to track session custom model selection")
	}
	// Verify default_custom_model fallback is present
	if !strings.Contains(script, "customConfig.default_custom_model") {
		t.Errorf("expected script to inspect default_custom_model when quota is exhausted")
	}
	// Verify unbind resets session selection
	if !strings.Contains(script, "window.__swiss_session_custom_model_id = null") {
		t.Errorf("expected unbindCustomModelForActiveProject to clear window.__swiss_session_custom_model_id")
	}
}

func TestGenerateCustomModelsScript_NativeModelSelectionProtection(t *testing.T) {
	script := GenerateCustomModelsScript(nil)
	for _, want := range []string{
		"window.__swiss_native_model_selected = true",
		"window.__swiss_native_model_selected = false",
		"window.__swiss_native_model_selected === true",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("expected script to contain %q for native model selection protection", want)
		}
	}
}

// The Projects panel display-options menu, conversation menus and other host
// surfaces are also [role="menu"]. The custom-model group may only be attached
// to the model selector's own popover, otherwise it shows up in those menus.
func TestGenerateCustomModelsScript_OnlyInjectsIntoModelSelectorPopover(t *testing.T) {
	script := GenerateCustomModelsScript(nil)

	if !containsSubstring(script, "function getModelSelectorPopover()") {
		t.Fatal("expected a getModelSelectorPopover helper anchoring the injection on the model selector")
	}

	// No lookup may start from a bare document-wide role="menu" query.
	for _, banned := range []string{
		`document.querySelector('[role="menu"]:not(#swiss-custom-thinking-submenu), [data-radix-popper-content-wrapper], .model-dropdown-menu')`,
		`document.querySelector('[role="menu"]:not(#swiss-custom-thinking-submenu)`,
	} {
		if containsSubstring(script, banned) {
			t.Errorf("custom model injection must not query %q", banned)
		}
	}

	// Both call sites (unavailable-inference cleanup and the injector) must use it.
	if got := countSubstring(script, "getModelSelectorPopover()"); got < 3 {
		t.Errorf("expected the definition plus two call sites of getModelSelectorPopover(), found %d occurrences", got)
	}

	// The group is what ends up in the wrong menu, so it must be created only
	// after the anchor check succeeded.
	injectAt := indexSubstring(script, "const popover = getModelSelectorPopover();")
	if injectAt == -1 {
		t.Fatal("expected the injector to resolve the popover through getModelSelectorPopover()")
	}
	groupAt := indexSubstring(script, `customContainer.className = "swiss-custom-models-menu-group"`)
	if groupAt == -1 {
		t.Fatal("expected the custom model group element to be created")
	}
	if groupAt < injectAt {
		t.Error("the custom model group must not be created before the model selector popover is resolved")
	}
}

func countSubstring(s, sub string) int {
	count := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			count++
			i += len(sub) - 1
		}
	}
	return count
}

func indexSubstring(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestGenerateCustomModelsScript_CatalogIsolation(t *testing.T) {
	// 1. Test that script contains getEffectiveCatalogMode helper
	script := GenerateCustomModelsScript(nil)
	if !strings.Contains(script, "function getEffectiveCatalogMode()") {
		t.Fatalf("expected script to contain function getEffectiveCatalogMode()")
	}
	if !strings.Contains(script, `"mode":"profile_isolation"`) {
		t.Errorf("expected default cfgJSON to contain profile_isolation mode")
	}
	if !strings.Contains(script, `"antigravity":{"source":"native","subagents":"app"}`) {
		t.Errorf("expected default cfgJSON to contain antigravity app source native")
	}

	// 2. Test profile_isolation with native profile:
	// - Reverts "Native Model" to "Model"
	// - Removes custom group
	// - Ensures native items are visible
	// - Clears session custom model
	// - Does not bind custom model label to trigger pill
	if !strings.Contains(script, `const catalogMode = getEffectiveCatalogMode();`) {
		t.Errorf("expected updateModelSelector to compute catalogMode")
	}
	if !strings.Contains(script, `if (catalogMode === "native_only") {`) {
		t.Errorf("expected updateModelSelector to handle native_only mode")
	}
	if !strings.Contains(script, `existingCustomGroup.remove()`) {
		t.Errorf("expected native_only mode to remove existingCustomGroup")
	}
	if !strings.Contains(script, `clearSessionCustomModel();`) {
		t.Errorf("expected native_only mode to clear session custom model")
	}

	// 3. Test profile_isolation with custom profile:
	// - Hides native model items
	// - Hides effort groups and context window options
	// - Sets custom header to "Model"
	// - Ensures custom model pill is bound
	if !strings.Contains(script, `if (catalogMode === "custom_only") {`) {
		t.Errorf("expected updateModelSelector to handle custom_only mode")
	}
	if !strings.Contains(script, `!el.matches('[data-testid="model-selector-custom-item"]')`) {
		t.Errorf("expected custom_only mode to filter out non-custom items")
	}
	if !strings.Contains(script, `popover.querySelectorAll('[data-testid="model-selector-effort-group"], [data-testid="model-selector-context-window-option"]').forEach`) {
		t.Errorf("expected custom_only mode to query and hide native effort groups and context options")
	}
	if !strings.Contains(script, `customHeader.textContent = "Model"`) {
		t.Errorf("expected customHeader textContent to be 'Model' in custom_only mode")
	}

	// 4. Test api_proxy keeps dual catalog with both "Native Model" and "Custom Model":
	cfgAPIProxy := &Config{
		Integration: IntegrationSettings{
			Mode: IntegrationAPIProxy,
		},
	}
	scriptAPIProxy := GenerateCustomModelsScript(cfgAPIProxy)
	if !strings.Contains(scriptAPIProxy, `"mode":"api_proxy"`) {
		t.Errorf("expected scriptAPIProxy cfgJSON to contain mode api_proxy")
	}
	if !strings.Contains(scriptAPIProxy, `nativeHeader.textContent = "Native Model"`) {
		t.Errorf("expected scriptAPIProxy to rename nativeHeader to 'Native Model'")
	}
	if !strings.Contains(scriptAPIProxy, `customHeader.textContent = "Custom Model"`) {
		t.Errorf("expected scriptAPIProxy to set customHeader to 'Custom Model'")
	}
}

func TestGenerateCustomModelsScript_CatalogModeEvaluation(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available; skipping node runtime verification")
	}

	script := GenerateCustomModelsScript(nil)
	fnStart := strings.Index(script, "function getEffectiveCatalogMode()")
	if fnStart == -1 {
		t.Fatalf("getEffectiveCatalogMode not found in script")
	}
	fnEnd := strings.Index(script[fnStart:], "function getSwissAuthHeaders")
	if fnEnd == -1 {
		t.Fatalf("could not isolate getEffectiveCatalogMode")
	}
	helperCode := script[fnStart : fnStart+fnEnd]

	testRunner := fmt.Sprintf(`
let customConfig = {};
%s
const assert = require("assert");

// Case 1: api_proxy mode -> hybrid
customConfig = { integration: { mode: "api_proxy" } };
assert.strictEqual(getEffectiveCatalogMode(), "hybrid");

// Case 2: profile_isolation with window.__SWISS_WINDOW_PROFILE__ = "custom" -> custom_only
customConfig = { integration: { mode: "profile_isolation" } };
global.window = { __SWISS_WINDOW_PROFILE__: "custom" };
assert.strictEqual(getEffectiveCatalogMode(), "custom_only");

// Case 3: profile_isolation with window.__SWISS_WINDOW_PROFILE__ = "native" -> native_only
global.window = { __SWISS_WINDOW_PROFILE__: "native" };
assert.strictEqual(getEffectiveCatalogMode(), "native_only");

// Case 4: process.env.AGY_LLM_GATEWAY_URL set -> custom_only
delete global.window;
process.env.AGY_LLM_GATEWAY_URL = "http://127.0.0.1:8765";
assert.strictEqual(getEffectiveCatalogMode(), "custom_only");
delete process.env.AGY_LLM_GATEWAY_URL;

// Case 5: fallback to app source custom -> custom_only
customConfig = { integration: { mode: "profile_isolation", apps: { antigravity: { source: "custom" } } } };
assert.strictEqual(getEffectiveCatalogMode(), "custom_only");

// Case 6: fallback to app source native -> native_only
customConfig = { integration: { mode: "profile_isolation", apps: { antigravity: { source: "native" } } } };
assert.strictEqual(getEffectiveCatalogMode(), "native_only");

console.log("ALL_CATALOG_MODES_VERIFIED");
`, helperCode)

	cmd := exec.Command(nodePath, "-e", testRunner)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node verification failed: %v: %s", err, string(out))
	}
	if !strings.Contains(string(out), "ALL_CATALOG_MODES_VERIFIED") {
		t.Fatalf("unexpected node output: %s", string(out))
	}
}

func TestGenerateCustomModelsScript_CustomModelThinkingChevronBehavior(t *testing.T) {
	script := GenerateCustomModelsScript(nil)

	// 1. Initial chevron wrap must be hidden (display: none) for all thinking models, never display: flex
	if strings.Contains(script, `<span class="swiss-custom-chevron-wrap" style="display: flex;`) {
		t.Errorf("expected script NOT to render swiss-custom-chevron-wrap with initial 'display: flex'")
	}
	if !strings.Contains(script, `swiss-custom-chevron-wrap`) {
		t.Fatalf("expected script to contain swiss-custom-chevron-wrap")
	}

	// 2. canChangeThinking must control display of both chk and chv (only when levels.length > 1)
	if !strings.Contains(script, `canChangeThinking = levels.length > 1`) {
		t.Errorf("expected script to define canChangeThinking based on levels.length > 1")
	}
	if !strings.Contains(script, `if (canChangeThinking) {`) {
		t.Errorf("expected script to guard thinking chevron and submenu with canChangeThinking")
	}

	// 3. Chevron wrap must be wired to open thinking submenu on hover and click when present
	if !strings.Contains(script, `chevronWrap.onmouseenter =`) {
		t.Errorf("expected chevronWrap.onmouseenter to be wired")
	}
	if !strings.Contains(script, `chevronWrap.onclick =`) {
		t.Errorf("expected chevronWrap.onclick to be wired")
	}

	// 4. itemEl.onmouseenter opens thinking submenu directly when canChangeThinking is true (matching Antigravity 2.0 app logic)
	itemEnterIdx := strings.Index(script, "itemEl.onmouseenter = () => {")
	if itemEnterIdx == -1 {
		t.Fatalf("expected script to define itemEl.onmouseenter")
	}
	itemEnterBody := script[itemEnterIdx : itemEnterIdx+500]
	if !strings.Contains(itemEnterBody, "openThinkingSubmenu()") {
		t.Errorf("expected itemEl.onmouseenter to call openThinkingSubmenu() when canChangeThinking is true")
	}

	// 5. itemEl.onclick must ignore clicks originating from chevronWrap
	if !strings.Contains(script, `.closest(".swiss-custom-chevron-wrap")`) {
		t.Errorf("expected itemEl.onclick to guard against clicks inside .swiss-custom-chevron-wrap")
	}
}

func TestGenerateCustomModelsScript_SingleVsMultiThinkingLevels(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available; skipping node runtime verification")
	}

	script := GenerateCustomModelsScript(nil)
	fnStart := strings.Index(script, "function getModelThinkingLevels(m)")
	if fnStart == -1 {
		t.Fatalf("getModelThinkingLevels not found in script")
	}
	fnEnd := strings.Index(script[fnStart:], "function getActiveThinkingLevel")
	if fnEnd == -1 {
		t.Fatalf("could not isolate getModelThinkingLevels")
	}
	helperCode := script[fnStart : fnStart+fnEnd]

	testRunner := fmt.Sprintf(`
%s
const assert = require("assert");

// Model with only 1 thinking level (cannot be changed)
const mSingle = { name: "custom-fixed", supports_thinking: true, thinking_levels: ["high"] };
const lvlsSingle = getModelThinkingLevels(mSingle);
assert.strictEqual(lvlsSingle.length, 1);
assert.strictEqual(lvlsSingle[0], "high");
assert.strictEqual(lvlsSingle.length > 0, true, "supportsThinking must be true");
assert.strictEqual(lvlsSingle.length > 1, false, "canChangeThinking must be false");

// Model with multiple thinking levels
const mMulti = { name: "custom-flexible", supports_thinking: true, thinking_levels: ["low", "medium", "high"] };
const lvlsMulti = getModelThinkingLevels(mMulti);
assert.strictEqual(lvlsMulti.length, 3);
assert.strictEqual(lvlsMulti.length > 0, true, "supportsThinking must be true");
assert.strictEqual(lvlsMulti.length > 1, true, "canChangeThinking must be true");

// Model without thinking
const mNone = { name: "custom-basic", supports_thinking: false, thinking_levels: [] };
const lvlsNone = getModelThinkingLevels(mNone);
assert.strictEqual(lvlsNone.length, 0);
assert.strictEqual(lvlsNone.length > 0, false, "supportsThinking must be false");
assert.strictEqual(lvlsNone.length > 1, false, "canChangeThinking must be false");

console.log("ALL_THINKING_LEVEL_VARIANTS_VERIFIED");
`, helperCode)

	cmd := exec.Command(nodePath, "-e", testRunner)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node verification failed: %v: %s", err, string(out))
	}
	if !strings.Contains(string(out), "ALL_THINKING_LEVEL_VARIANTS_VERIFIED") {
		t.Fatalf("unexpected node output: %s", string(out))
	}
}
