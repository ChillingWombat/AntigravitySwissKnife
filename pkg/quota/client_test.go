package quota

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestDetectTierFromGoogleUserInfo(t *testing.T) {
	// Test empty token
	_, err := DetectTierFromGoogleUserInfo("", "user@gmail.com")
	if err == nil {
		t.Fatal("expected error for empty access token")
	}

	// 1. Edu domain test
	eduServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"email": "student@stanford.edu",
			"hd":    "stanford.edu",
		})
	}))
	defer eduServer.Close()

	origUserInfoURLs := GoogleUserInfoURLs
	defer func() { GoogleUserInfoURLs = origUserInfoURLs }()
	GoogleUserInfoURLs = []string{eduServer.URL}

	tier, err := DetectTierFromGoogleUserInfo("mock-token", "student@stanford.edu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != PlanTierEdu {
		t.Errorf("expected %s, got %s", PlanTierEdu, tier)
	}

	// 2. Enterprise domain test
	enterpriseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"email": "dev@mycompany.org",
			"hd":    "mycompany.org",
		})
	}))
	defer enterpriseServer.Close()
	GoogleUserInfoURLs = []string{enterpriseServer.URL}

	tier, err = DetectTierFromGoogleUserInfo("mock-token", "dev@mycompany.org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != PlanTierEnterprise {
		t.Errorf("expected %s, got %s", PlanTierEnterprise, tier)
	}

	// 3. Regular consumer Gmail test
	gmailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"email": "regular@gmail.com",
			"hd":    "",
		})
	}))
	defer gmailServer.Close()
	GoogleUserInfoURLs = []string{gmailServer.URL}

	_, err = DetectTierFromGoogleUserInfo("mock-token", "regular@gmail.com")
	if err == nil {
		t.Errorf("expected error / no special tier for regular gmail account")
	}
}

func TestDetectTierFromAvailableModels(t *testing.T) {
	// Test empty token
	_, err := DetectTierFromAvailableModels("")
	if err == nil {
		t.Fatal("expected error for empty access token")
	}

	origModelsURLs := CloudCodeModelsURLs
	defer func() { CloudCodeModelsURLs = origModelsURLs }()

	// 1. Ultra tier models returned
	ultraServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"models": map[string]interface{}{
				"gemini-2.5-ultra": map[string]string{"displayName": "Gemini Ultra 2.5"},
			},
		})
	}))
	defer ultraServer.Close()
	CloudCodeModelsURLs = []string{ultraServer.URL}

	tier, err := DetectTierFromAvailableModels("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != PlanTierUltra20X {
		t.Errorf("expected %s, got %s", PlanTierUltra20X, tier)
	}

	// 2. Pro tier models with third-party models returned (Paid Pro)
	proPaidServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tieredModelIds": map[string]interface{}{
				"pro": []string{"gemini-2.5-pro"},
			},
			"models": map[string]interface{}{
				"claude-3-5-sonnet": map[string]string{"displayName": "Claude 3.5 Sonnet"},
			},
		})
	}))
	defer proPaidServer.Close()
	CloudCodeModelsURLs = []string{proPaidServer.URL}

	tier, err = DetectTierFromAvailableModels("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != PlanTierPro {
		t.Errorf("expected %s, got %s", PlanTierPro, tier)
	}

	// 2b. Pro tier models without third-party models returned (Pro - Trial)
	proTrialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tieredModelIds": map[string]interface{}{
				"pro": []string{"gemini-2.5-pro"},
			},
		})
	}))
	defer proTrialServer.Close()
	CloudCodeModelsURLs = []string{proTrialServer.URL}

	tier, err = DetectTierFromAvailableModels("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != PlanTierProTrial {
		t.Errorf("expected %s, got %s", PlanTierProTrial, tier)
	}

	// 3. Only Flash/Free tier models returned
	freeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tieredModelIds": map[string]interface{}{
				"flash": []string{"gemini-2.5-flash"},
			},
		})
	}))
	defer freeServer.Close()
	CloudCodeModelsURLs = []string{freeServer.URL}

	tier, err = DetectTierFromAvailableModels("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 4. Enterprise models returned via tieredModelIds
	entServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tieredModelIds": map[string]interface{}{
				"enterprise": []string{"gemini-enterprise-advanced"},
			},
		})
	}))
	defer entServer.Close()
	CloudCodeModelsURLs = []string{entServer.URL}

	tier, err = DetectTierFromAvailableModels("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != PlanTierEnterprise {
		t.Errorf("expected %s, got %s", PlanTierEnterprise, tier)
	}
}

func TestFetchProjectAndTier(t *testing.T) {
	origLoadURLs := CloudCodeLoadProjectURLs
	defer func() { CloudCodeLoadProjectURLs = origLoadURLs }()

	// Test 1: Explicit currentTier Pro + credits
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"cloudaicompanionProject": "projects/my-prod-project",
			"currentTier": map[string]interface{}{
				"name": "Pro",
				"availableCredits": []map[string]interface{}{
					{"creditType": "DEFAULT", "creditAmount": 75.5},
				},
			},
		})
	}))
	defer mockServer.Close()
	CloudCodeLoadProjectURLs = []string{mockServer.URL}

	res, err := FetchProjectAndTier("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProjectID != "projects/my-prod-project" {
		t.Errorf("expected project projects/my-prod-project, got %s", res.ProjectID)
	}
	if res.TierName != PlanTierPro {
		t.Errorf("expected tier %s, got %s", PlanTierPro, res.TierName)
	}
	if res.Credits != 75.5 {
		t.Errorf("expected credits 75.5, got %f", res.Credits)
	}

	// Test 2: Ineligible Free tier inference
	ineligibleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"cloudaicompanion_project": "projects/custom-project",
			"ineligible_tiers": []map[string]interface{}{
				{"id": "free-tier"},
			},
		})
	}))
	defer ineligibleServer.Close()
	CloudCodeLoadProjectURLs = []string{ineligibleServer.URL}

	res, err = FetchProjectAndTier("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TierName != PlanTierPro {
		t.Errorf("expected tier %s inferred from ineligible free tier, got %s", PlanTierPro, res.TierName)
	}

	// Test 3: Allowed tiers with Ultra 20X
	allowedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"cloudaicompanionProject": "projects/ultra-project",
			"allowedTiers": []map[string]interface{}{
				{"displayName": "Standard Pro"},
				{"displayName": "Ultra 20X"},
			},
		})
	}))
	defer allowedServer.Close()
	CloudCodeLoadProjectURLs = []string{allowedServer.URL}

	res, err = FetchProjectAndTier("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TierName != PlanTierUltra20X {
		t.Errorf("expected tier %s from allowed tiers, got %s", PlanTierUltra20X, res.TierName)
	}

	// Test 4: Allowed tiers with Ultra 5X (must not be overridden by generic Ultra 20X)
	ultra5Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"cloudaicompanionProject": "projects/ultra5-project",
			"allowedTiers": []map[string]interface{}{
				{"displayName": "Ultra 5X"},
			},
		})
	}))
	defer ultra5Server.Close()
	CloudCodeLoadProjectURLs = []string{ultra5Server.URL}

	res, err = FetchProjectAndTier("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TierName != PlanTierUltra5X {
		t.Errorf("expected tier %s from allowed tiers, got %s", PlanTierUltra5X, res.TierName)
	}

	// Test 5: Protobuf tier enum and isTrial flag
	trialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"cloudaicompanionProject": "projects/trial-project",
			"tier":                    "TIER_PRO",
			"isTrial":                 true,
		})
	}))
	defer trialServer.Close()
	CloudCodeLoadProjectURLs = []string{trialServer.URL}

	res, err = FetchProjectAndTier("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TierName != PlanTierProTrial {
		t.Errorf("expected tier %s for trial account, got %s", PlanTierProTrial, res.TierName)
	}

	// Test 6: Warning notice identifying trial account
	noticeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"cloudaicompanionProject": "projects/notice-project",
			"tier":                    "TIER_PRO",
			"warningMessage":          "Sonnet 5.5 is now available on paid Pro and Ultra plans. Third-party model access will no longer be available on your current plan starting on November 2, 2026.",
		})
	}))
	defer noticeServer.Close()
	CloudCodeLoadProjectURLs = []string{noticeServer.URL}

	res, err = FetchProjectAndTier("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TierName != PlanTierProTrial {
		t.Errorf("expected tier %s for notice account, got %s", PlanTierProTrial, res.TierName)
	}
}

func TestIsTrialWarningTextAndNormalize(t *testing.T) {
	exactNotice := "Sonnet 5.5 is now available on paid Pro and Ultra plans. Third-party model access will no longer be available on your current plan starting on November 2, 2026."
	if !IsTrialWarningText(exactNotice) {
		t.Errorf("expected IsTrialWarningText to be true for exact user tooltip notice")
	}
	if !IsTrialWarningText("Third-party model access will no longer be available on your current plan") {
		t.Errorf("expected IsTrialWarningText to be true for model access notice")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"Pro - Trial", PlanTierProTrial},
		{"trial", PlanTierProTrial},
		{"promo", PlanTierProTrial},
		{"starter pro", PlanTierProTrial},
		{"jio", PlanTierProTrial},
		{"partner", PlanTierProTrial},
		{exactNotice, PlanTierProTrial},
		{"starter quota", PlanTierFree},
		{"starter", PlanTierFree},
		{"Free", PlanTierFree},
		{"Pro", PlanTierPro},
		{"Google AI Pro", PlanTierProTrial},
		{"Ultra 20X", PlanTierUltra20X},
	}

	for _, tc := range tests {
		got := NormalizePlanTier(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizePlanTier(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestFetchLiveQuotaBreakdown_ValidationRequiredShortCircuit(t *testing.T) {
	origQuotaURLs := CloudCodeRetrieveQuotaURLs
	defer func() { CloudCodeRetrieveQuotaURLs = origQuotaURLs }()

	primaryCalls := 0
	fallbackCalls := 0

	primarySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{
			"error": {
				"code": 403,
				"message": "Verify your account to continue.",
				"status": "PERMISSION_DENIED",
				"details": [
					{
						"@type": "type.googleapis.com/google.rpc.ErrorInfo",
						"reason": "VALIDATION_REQUIRED",
						"metadata": {
							"validation_url": "https://accounts.google.com/signin/continue?flowName=GlifWebSignIn&authuser"
						}
					}
				]
			}
		}`))
	}))
	defer primarySrv.Close()

	fallbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer fallbackSrv.Close()

	CloudCodeRetrieveQuotaURLs = []string{primarySrv.URL, fallbackSrv.URL}

	breakdown, err := FetchLiveQuotaBreakdown("mock-token", "")
	if breakdown != nil {
		t.Fatalf("expected nil breakdown on VALIDATION_REQUIRED, got %+v", breakdown)
	}
	apiErr, ok := err.(*QuotaAPIError)
	if !ok {
		t.Fatalf("expected *QuotaAPIError, got %T (%v)", err, err)
	}
	if apiErr.Status != "ERROR" {
		t.Errorf("expected Status ERROR, got %q", apiErr.Status)
	}
	if apiErr.Reason != "VALIDATION_REQUIRED" {
		t.Errorf("expected Reason VALIDATION_REQUIRED, got %q", apiErr.Reason)
	}
	if primaryCalls != 1 {
		t.Errorf("expected primary endpoint to be called once, got %d", primaryCalls)
	}
	if fallbackCalls != 0 {
		t.Errorf("expected fallback endpoint not to be called on VALIDATION_REQUIRED, got %d", fallbackCalls)
	}
}

func TestPollAndCacheAccount_ClearsErrorInStoreOnRecovery(t *testing.T) {
	origLoadURLs := CloudCodeLoadProjectURLs
	origQuotaURLs := CloudCodeRetrieveQuotaURLs
	defer func() {
		CloudCodeLoadProjectURLs = origLoadURLs
		CloudCodeRetrieveQuotaURLs = origQuotaURLs
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"cloudaicompanionProject": "aicode-consumers",
			"paidTier": {"id": "g1-pro-tier", "name": "Google AI Pro"},
			"groups": [
				{
					"displayName": "Gemini Models",
					"buckets": [
						{"bucketId": "gemini-5h", "window": "5h", "remainingFraction": 0.9},
						{"bucketId": "gemini-weekly", "window": "weekly", "remainingFraction": 0.8}
					]
				}
			]
		}`))
	}))
	defer srv.Close()

	CloudCodeLoadProjectURLs = []string{srv.URL}
	CloudCodeRetrieveQuotaURLs = []string{srv.URL}

	tmpPath := filepath.Join(t.TempDir(), "accounts.json")
	store, err := keyring.NewStore(tmpPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	_, err = store.ImportAccount("joseantoniocarrarofalchi@gmail.com", "1//mock-rt", "ya29.mock-at", "Jose", "")
	if err != nil {
		t.Fatalf("failed to import account: %v", err)
	}
	_ = store.UpdateAccountStatusWithError("joseantoniocarrarofalchi@gmail.com", "ERROR", "Verify your account to continue. (VALIDATION_REQUIRED)")

	acc, _ := store.GetAccount("joseantoniocarrarofalchi@gmail.com")
	if acc.Status != "ERROR" || acc.ErrorMessage == "" {
		t.Fatalf("expected initial ERROR status, got %q / %q", acc.Status, acc.ErrorMessage)
	}

	summary, pollErr := PollAndCacheAccount(acc, store)
	if pollErr != nil {
		t.Fatalf("expected nil pollErr on recovery, got %v", pollErr)
	}
	if summary == nil || summary.ErrorStatus != "" {
		t.Fatalf("expected clean summary, got %+v", summary)
	}

	afterAcc, _ := store.GetAccount("joseantoniocarrarofalchi@gmail.com")
	if afterAcc.Status == "ERROR" {
		t.Errorf("expected store status to recover from ERROR, still got %q", afterAcc.Status)
	}
	if afterAcc.ErrorMessage != "" {
		t.Errorf("expected store ErrorMessage to be cleared on recovery, got %q", afterAcc.ErrorMessage)
	}
}

func TestPopulateVerificationAuthUser(t *testing.T) {
	raw := "https://accounts.google.com/signin/continue?sarp=1&flowName=GlifWebSignIn&authuser"
	got := populateVerificationAuthUser(raw, "joseantoniocarrarofalchi@gmail.com")
	if !strings.HasSuffix(got, "&authuser=joseantoniocarrarofalchi%40gmail.com") {
		t.Errorf("expected populated authuser param, got %q", got)
	}
}

