package custommodels

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeVertexAIModel(t *testing.T) {
	tests := []struct {
		input             string
		expectedPublisher string
		expectedModel     string
		expectedAction    string
	}{
		{
			input:             "google/gemini-4-argon",
			expectedPublisher: "google",
			expectedModel:     "gemini-4-argon",
			expectedAction:    "generateContent",
		},
		{
			input:             "publishers/google/models/gemini-2.5-flash",
			expectedPublisher: "google",
			expectedModel:     "gemini-2.5-flash",
			expectedAction:    "generateContent",
		},
		{
			input:             "publishers/google/models/google/gemini-4-argon",
			expectedPublisher: "google",
			expectedModel:     "gemini-4-argon",
			expectedAction:    "generateContent",
		},
		{
			input:             "projects/chillingwombat/locations/us-central1/publishers/google/models/gemini-4-argon",
			expectedPublisher: "google",
			expectedModel:     "gemini-4-argon",
			expectedAction:    "generateContent",
		},
		{
			input:             "https://us-east5-aiplatform.googleapis.com/v1/projects/my-p/locations/us-east5/publishers/anthropic/models/claude-3-7-sonnet@20250219:streamRawPredict",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-7-sonnet@20250219",
			expectedAction:    "streamRawPredict",
		},
		{
			input:             "gemini-2.5-flash",
			expectedPublisher: "google",
			expectedModel:     "gemini-2.5-flash",
			expectedAction:    "generateContent",
		},
		{
			input:             "claude-3-7-sonnet@20250219",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-7-sonnet@20250219",
			expectedAction:    "rawPredict",
		},
		{
			input:             "anthropic/claude-3-7-sonnet@20250219",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-7-sonnet@20250219",
			expectedAction:    "rawPredict",
		},
		{
			input:             "meta/llama-3.3-70b-instruct",
			expectedPublisher: "meta",
			expectedModel:     "llama-3.3-70b-instruct",
			expectedAction:    "rawPredict",
		},
		{
			input:             "publishers/meta/models/llama-3.3-70b-instruct:predict",
			expectedPublisher: "meta",
			expectedModel:     "llama-3.3-70b-instruct",
			expectedAction:    "predict",
		},
		{
			input:             "claude-3-7-sonnet",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-7-sonnet@20250219",
			expectedAction:    "rawPredict",
		},
		{
			input:             "claude-3.7-sonnet",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-7-sonnet@20250219",
			expectedAction:    "rawPredict",
		},
		{
			input:             "claude-3-7-sonnet-20250219",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-7-sonnet@20250219",
			expectedAction:    "rawPredict",
		},
		{
			input:             "claude-3-5-sonnet",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-5-sonnet-v2@20241022",
			expectedAction:    "rawPredict",
		},
		{
			input:             "claude-3-5-sonnet-20241022",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-5-sonnet-v2@20241022",
			expectedAction:    "rawPredict",
		},
		{
			input:             "claude-3-5-sonnet@20241022",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-5-sonnet-v2@20241022",
			expectedAction:    "rawPredict",
		},
		{
			input:             "claude-3-5-haiku",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-5-haiku@20241022",
			expectedAction:    "rawPredict",
		},
		{
			input:             "claude-3-opus",
			expectedPublisher: "anthropic",
			expectedModel:     "claude-3-opus@20240229",
			expectedAction:    "rawPredict",
		},
	}

	for _, tc := range tests {
		pub, model, action := NormalizeVertexAIModel(tc.input)
		if pub != tc.expectedPublisher {
			t.Errorf("NormalizeVertexAIModel(%q) publisher = %q; want %q", tc.input, pub, tc.expectedPublisher)
		}
		if model != tc.expectedModel {
			t.Errorf("NormalizeVertexAIModel(%q) model = %q; want %q", tc.input, model, tc.expectedModel)
		}
		if action != tc.expectedAction {
			t.Errorf("NormalizeVertexAIModel(%q) action = %q; want %q", tc.input, action, tc.expectedAction)
		}
	}
}

func TestBuildVertexAIEndpoint_NoDuplicatePublisher(t *testing.T) {
	endpoint := BuildVertexAIEndpoint("us-central1", "chillingwombat", "google/gemini-4-argon")
	expected := "https://us-central1-aiplatform.googleapis.com/v1/projects/chillingwombat/locations/us-central1/publishers/google/models/gemini-4-argon:generateContent"
	if endpoint != expected {
		t.Errorf("BuildVertexAIEndpoint got %q; want %q", endpoint, expected)
	}

	endpointClaude := BuildVertexAIEndpoint("us-east5", "my-project", "claude-3-7-sonnet@20250219")
	expectedClaude := "https://us-east5-aiplatform.googleapis.com/v1/projects/my-project/locations/us-east5/publishers/anthropic/models/claude-3-7-sonnet@20250219:rawPredict"
	if endpointClaude != expectedClaude {
		t.Errorf("BuildVertexAIEndpoint got %q; want %q", endpointClaude, expectedClaude)
	}
}

func TestValidateTokenURI_Security(t *testing.T) {
	// Rejects non-https
	if err := ValidateTokenURI("http://oauth2.googleapis.com/token"); err == nil {
		t.Error("expected error for non-https token URI")
	}

	// Rejects AWS / Cloud Metadata and private IP addresses
	if err := ValidateTokenURI("https://169.254.169.254/latest/meta-data"); err == nil {
		t.Error("expected error for metadata IP token URI")
	}
	if err := ValidateTokenURI("https://evil.attacker.com/token"); err == nil {
		t.Error("expected error for attacker domain token URI")
	}

	// Accepts valid Google OAuth token endpoints
	if err := ValidateTokenURI("https://oauth2.googleapis.com/token"); err != nil {
		t.Errorf("expected valid token URI, got %v", err)
	}
	if err := ValidateTokenURI("https://accounts.google.com/o/oauth2/token"); err != nil {
		t.Errorf("expected valid token URI, got %v", err)
	}
}

func TestPricingLookup_VertexAIClaudeAndGemini4(t *testing.T) {
	// Test Claude with @ version tag
	in, _, out, ok := lookupOfficialProviderPricing("claude-3-7-sonnet@20250219", "gcp")
	if !ok || in == nil || out == nil || *in != 3.00 || *out != 15.00 {
		t.Errorf("expected pricing for claude-3-7-sonnet@20250219, got in=%v out=%v ok=%v", in, out, ok)
	}

	// Test Gemini 4 Argon
	inG, _, outG, okG := lookupOfficialProviderPricing("google/gemini-4-argon", "gcp")
	if !okG || inG == nil || outG == nil || *inG != 0.75 || *outG != 3.75 {
		t.Errorf("expected pricing for google/gemini-4-argon, got in=%v out=%v ok=%v", inG, outG, okG)
	}

	// Test with full resource path
	inP, _, outP, okP := lookupOfficialProviderPricing("publishers/google/models/gemini-4-argon", "gcp")
	if !okP || inP == nil || outP == nil || *inP != 0.75 || *outP != 3.75 {
		t.Errorf("expected pricing for publishers/google/models/gemini-4-argon, got in=%v out=%v ok=%v", inP, outP, okP)
	}
}

func TestStore_PreservesGCPProjectAndRegionOnUpdate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss-gcp-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(filepath.Join(tmpDir, "config"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Initial model
	m := CustomModel{
		ID:                   "gcp-argon-1",
		Name:                 "gemini-4-argon",
		DisplayName:          "Gemini 4 Argon",
		ProviderType:         ProviderGCP,
		GCPProjectID:         "chillingwombat",
		GCPRegion:            "europe-west1",
		GCPServiceAccountKey: `{"type":"service_account","project_id":"chillingwombat","private_key":"dummy","client_email":"sa@chillingwombat.iam.gserviceaccount.com"}`,
		Enabled:              true,
	}

	if err := store.SaveModel(m); err != nil {
		t.Fatalf("failed to save initial model: %v", err)
	}

	// Partial update without project ID or region
	updatePayload := CustomModel{
		ID:           "gcp-argon-1",
		Name:         "gemini-4-argon",
		DisplayName:  "Gemini 4 Argon (Updated)",
		ProviderType: ProviderGCP,
		Enabled:      true,
	}

	if err := store.SaveModel(updatePayload); err != nil {
		t.Fatalf("failed to save updated model: %v", err)
	}

	saved, err := store.GetModel("gcp-argon-1")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}

	if saved.Name != "gemini-4-argon" {
		t.Errorf("saved.Name = %q; want %q", saved.Name, "gemini-4-argon")
	}
	if saved.DisplayName != "Gemini 4 Argon (Updated)" {
		t.Errorf("saved.DisplayName = %q; want %q", saved.DisplayName, "Gemini 4 Argon (Updated)")
	}
	if saved.GCPProjectID != "chillingwombat" {
		t.Errorf("saved.GCPProjectID = %q; want %q", saved.GCPProjectID, "chillingwombat")
	}
	if saved.GCPRegion != "europe-west1" {
		t.Errorf("saved.GCPRegion = %q; want %q", saved.GCPRegion, "europe-west1")
	}
}

func TestFetchGCPBillingInfoWithToken_ForbiddenReturnsActive(t *testing.T) {
	// Mock Cloud Billing API returning 403 Forbidden
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"code":403,"message":"Permission billing.resourceAssociations.list denied"}}`))
	}))
	defer server.Close()

	info, err := FetchGCPBillingInfoWithEndpoint(context.Background(), server.URL, "mock-token", "test-proj")
	if err != nil {
		t.Fatalf("expected nil error on 403 forbidden, got: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil billing info")
	}
	if !info.BillingEnabled {
		t.Errorf("expected BillingEnabled=true, got false")
	}
	if info.StatusMessage != "Active (GCP Billing)" {
		t.Errorf("expected StatusMessage='Active (GCP Billing)', got %q", info.StatusMessage)
	}
}

func TestFetchGCPBillingInfoWithEndpoint_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"projects/test-proj/billingInfo","projectId":"test-proj","billingAccountName":"billingAccounts/012345-6789AB-CDEF01","billingEnabled":true}`))
	}))
	defer server.Close()

	info, err := FetchGCPBillingInfoWithEndpoint(context.Background(), server.URL, "mock-token", "test-proj")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil billing info")
	}
	if !info.BillingEnabled {
		t.Errorf("expected BillingEnabled=true, got false")
	}
	if info.StatusMessage != "Active (Account: 012345-6789AB-CDEF01)" {
		t.Errorf("expected StatusMessage='Active (Account: 012345-6789AB-CDEF01)', got %q", info.StatusMessage)
	}
}

func TestParseGCPErrorMessage(t *testing.T) {
	// Google Vertex AI structured error
	vErr := []byte(`{"error":{"code":404,"message":"Publisher model google/gemini-4-argon not found","status":"NOT_FOUND"}}`)
	got := parseGCPErrorMessage(404, vErr)
	expected := "HTTP 404: Publisher model google/gemini-4-argon not found"
	if got != expected {
		t.Errorf("parseGCPErrorMessage(404, ...) = %q; want %q", got, expected)
	}

	// Anthropic on Vertex AI structured error
	aErr := []byte(`{"type":"error","error":{"type":"not_found_error","message":"Model not found"}}`)
	gotA := parseGCPErrorMessage(404, aErr)
	expectedA := "HTTP 404: Model not found"
	if gotA != expectedA {
		t.Errorf("parseGCPErrorMessage(404, ...) = %q; want %q", gotA, expectedA)
	}

	// Empty body falls back to HTTP status text
	gotEmpty := parseGCPErrorMessage(404, []byte(""))
	expectedEmpty := "HTTP 404 Not Found"
	if gotEmpty != expectedEmpty {
		t.Errorf("parseGCPErrorMessage(404, empty) = %q; want %q", gotEmpty, expectedEmpty)
	}
}

func TestBuildVertexAIEndpoint_ClaudeRegionRemap(t *testing.T) {
	// Claude on Vertex AI remaps us-central1 and empty region to us-east5
	epCentral := BuildVertexAIEndpoint("us-central1", "test-proj", "claude-3-7-sonnet")
	if !strings.Contains(epCentral, "https://us-east5-aiplatform.googleapis.com") {
		t.Errorf("expected Claude to remap us-central1 to us-east5, got %s", epCentral)
	}
	if !strings.Contains(epCentral, "publishers/anthropic/models/claude-3-7-sonnet@20250219:rawPredict") {
		t.Errorf("expected normalized Claude 3.7 model path, got %s", epCentral)
	}

	epEmpty := BuildVertexAIEndpoint("", "test-proj", "claude-3-5-sonnet")
	if !strings.Contains(epEmpty, "https://us-east5-aiplatform.googleapis.com") {
		t.Errorf("expected Claude with empty region to default to us-east5, got %s", epEmpty)
	}

	// Europe region is preserved
	epEurope := BuildVertexAIEndpoint("europe-west1", "test-proj", "claude-3-7-sonnet")
	if !strings.Contains(epEurope, "https://europe-west1-aiplatform.googleapis.com") {
		t.Errorf("expected Claude in europe-west1 to be preserved, got %s", epEurope)
	}

	// Gemini in us-central1 stays us-central1
	epGemini := BuildVertexAIEndpoint("us-central1", "test-proj", "gemini-2.5-flash")
	if !strings.Contains(epGemini, "https://us-central1-aiplatform.googleapis.com") {
		t.Errorf("expected Gemini in us-central1 to remain in us-central1, got %s", epGemini)
	}
}

func TestResolveEndpoint_GCPRootDomainFallback(t *testing.T) {
	// When given bare root domain without /models/, ResolveEndpoint must construct the full model endpoint
	bareURL := "https://us-central1-aiplatform.googleapis.com"
	resolved := ResolveEndpoint(ProviderGCP, bareURL, "gemini-2.5-flash")
	expected := "https://us-central1-aiplatform.googleapis.com/v1/projects//locations/us-central1/publishers/google/models/gemini-2.5-flash:generateContent"
	if resolved != expected {
		t.Errorf("ResolveEndpoint(%q) = %q; want %q", bareURL, resolved, expected)
	}

	// When given project path in base URL, project ID is preserved
	projBase := "https://us-east5-aiplatform.googleapis.com/v1/projects/my-prod-project/locations/us-east5"
	resolvedProj := ResolveEndpoint(ProviderGCP, projBase, "claude-3-7-sonnet")
	expectedProj := "https://us-east5-aiplatform.googleapis.com/v1/projects/my-prod-project/locations/us-east5/publishers/anthropic/models/claude-3-7-sonnet@20250219:rawPredict"
	if resolvedProj != expectedProj {
		t.Errorf("ResolveEndpoint(%q) = %q; want %q", projBase, resolvedProj, expectedProj)
	}

	// When given full /models/ endpoint, it is preserved
	fullURL := "https://us-central1-aiplatform.googleapis.com/v1/projects/p/locations/us-central1/publishers/google/models/gemini-2.5-flash:generateContent"
	resolvedFull := ResolveEndpoint(ProviderGCP, fullURL, "gemini-2.5-flash")
	if resolvedFull != fullURL {
		t.Errorf("ResolveEndpoint fullURL got %q; want %q", resolvedFull, fullURL)
	}
}

func TestDetectAndFetchQuota_GCP_PreservesBalance(t *testing.T) {
	// Model with existing BalanceValue must preserve it and remain QuotaTypeBalance
	m := CustomModel{
		ProviderType: ProviderGCP,
		BaseURL:      "https://us-central1-aiplatform.googleapis.com",
		Name:         "gemini-2.5-flash",
		QuotaType:    QuotaTypeBalance,
		BalanceValue: "Active (Account: 012345-6789AB)",
	}
	res := DetectAndFetchQuota(m)
	if res.QuotaType != QuotaTypeBalance {
		t.Errorf("expected QuotaTypeBalance, got %s", res.QuotaType)
	}
	if res.BalanceValue != "Active (Account: 012345-6789AB)" {
		t.Errorf("expected BalanceValue 'Active (Account: 012345-6789AB)', got %q", res.BalanceValue)
	}
}

func TestAuditor_GCPRootDomainReplacedWithModelEndpoint(t *testing.T) {
	auditor := NewAuditor()
	m := CustomModel{
		ID:           "test-gcp-auditor",
		Name:         "gemini-2.5-flash",
		ProviderType: ProviderGCP,
		GCPProjectID: "audit-proj",
		GCPRegion:    "us-central1",
		BaseURL:      "https://us-central1-aiplatform.googleapis.com",
	}

	report := auditor.RunAudit(m)
	if !strings.Contains(report.Endpoint, "/publishers/google/models/gemini-2.5-flash:generateContent") {
		t.Errorf("expected report.Endpoint to be full Vertex AI model URL, got %s", report.Endpoint)
	}
}

func TestCustomModelValidate_GCP(t *testing.T) {
	m := CustomModel{
		ID:           "test-gcp-validate",
		Name:         "claude-3-7-sonnet",
		ProviderType: ProviderGCP,
		BaseURL:      "https://us-central1-aiplatform.googleapis.com/v1/projects/extracted-proj/locations/us-central1",
	}

	if err := m.Validate(); err != nil {
		t.Fatalf("m.Validate() failed: %v", err)
	}

	if m.GCPProjectID != "extracted-proj" {
		t.Errorf("expected GCPProjectID 'extracted-proj', got %q", m.GCPProjectID)
	}
	if m.GCPRegion != "us-east5" {
		t.Errorf("expected GCPRegion 'us-east5' (auto-remapped for Claude), got %q", m.GCPRegion)
	}
	if m.QuotaType != QuotaTypeBalance {
		t.Errorf("expected QuotaTypeBalance, got %s", m.QuotaType)
	}
	if m.BalanceValue != "Active (GCP Billing)" {
		t.Errorf("expected BalanceValue 'Active (GCP Billing)', got %q", m.BalanceValue)
	}
	if !strings.Contains(m.BaseURL, "/publishers/anthropic/models/claude-3-7-sonnet@20250219:rawPredict") {
		t.Errorf("expected BaseURL to contain full Claude Vertex AI endpoint, got %s", m.BaseURL)
	}
}

func TestStore_ClearDefaultFlags(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if err := store.SaveModel(CustomModel{
		ID:           "m-a",
		Name:         "gemini-2.5-flash",
		ProviderType: ProviderGCP,
		GCPProjectID: "proj",
		GCPRegion:    "us-central1",
		IsDefault:    true,
	}); err != nil {
		t.Fatalf("SaveModel failed: %v", err)
	}
	if m, _ := store.GetModel("m-a"); m == nil || !m.IsDefault {
		t.Fatalf("precondition failed: m-a should be flagged default")
	}

	if err := store.ClearDefaultFlags(); err != nil {
		t.Fatalf("ClearDefaultFlags failed: %v", err)
	}
	if m, _ := store.GetModel("m-a"); m != nil && m.IsDefault {
		t.Errorf("expected m-a.IsDefault=false after ClearDefaultFlags")
	}

	// Idempotent second call must not error.
	if err := store.ClearDefaultFlags(); err != nil {
		t.Fatalf("second ClearDefaultFlags failed: %v", err)
	}
}

func TestCustomModelValidate_GCP_ManualQuotaOverrideRespected(t *testing.T) {
	m := CustomModel{
		ID:                  "manual-quota",
		Name:                "gemini-2.5-flash",
		ProviderType:        ProviderGCP,
		GCPProjectID:        "proj",
		GCPRegion:           "us-central1",
		QuotaManualOverride: true,
		QuotaType:           QuotaTypeQuota,
		QuotaValue:          "GCP Managed",
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if m.QuotaType != QuotaTypeQuota {
		t.Errorf("manual quota override clobbered: QuotaType = %q; want %q", m.QuotaType, QuotaTypeQuota)
	}
	if m.QuotaValue != "GCP Managed" {
		t.Errorf("manual quota override clobbered: QuotaValue = %q; want %q", m.QuotaValue, "GCP Managed")
	}
	if m.BalanceValue != "" {
		t.Errorf("manual quota override should not synthesize BalanceValue, got %q", m.BalanceValue)
	}
}
