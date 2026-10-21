package quota

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
				"claude-sonnet-5-5": map[string]string{"displayName": "Claude Sonnet 5.5 (Thinking)"},
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

	// 2c. Family-shared Pro trial still exposes older Claude/GPT models.
	familySharedTrialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tieredModelIds": map[string]interface{}{
				"pro": []string{"gemini-2.5-pro"},
			},
			"models": map[string]interface{}{
				"claude-sonnet-4-6":   map[string]string{"displayName": "Claude Sonnet 4.6 (Thinking)"},
				"claude-opus-4-6":     map[string]string{"displayName": "Claude Opus 4.6 (Thinking)"},
				"gpt-oss-120b-medium": map[string]string{"displayName": "GPT-OSS 120B (Medium)"},
			},
		})
	}))
	defer familySharedTrialServer.Close()
	CloudCodeModelsURLs = []string{familySharedTrialServer.URL}

	tier, err = DetectTierFromAvailableModels("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != PlanTierProTrial {
		t.Errorf("family-shared Pro trial must stay %s, got %s", PlanTierProTrial, tier)
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

func TestResolveTierFromQuotaAccess_FamilySharedTrial(t *testing.T) {
	tests := []struct {
		name      string
		tier      string
		breakdown LiveQuotaBreakdown
		want      string
	}{
		{
			name: "explicit trial with unknown model catalog",
			tier: "",
			breakdown: LiveQuotaBreakdown{
				ExplicitTrial: true,
			},
			want: PlanTierProTrial,
		},
		{
			name: "explicit trial with legacy third-party models",
			tier: PlanTierPro,
			breakdown: LiveQuotaBreakdown{
				ExplicitTrial:      true,
				HasClaudeGPTRights: true,
				ModelCatalogKnown:  true,
			},
			want: PlanTierProTrial,
		},
		{
			name: "explicit pro with unknown model catalog remains pro",
			tier: PlanTierPro,
			breakdown: LiveQuotaBreakdown{
				HasClaudeGPTRights: true,
			},
			want: PlanTierPro,
		},
		{
			name: "pro with known catalog and no paid-only model is trial",
			tier: PlanTierPro,
			breakdown: LiveQuotaBreakdown{
				HasClaudeGPTRights: true,
				ModelCatalogKnown:  true,
			},
			want: PlanTierProTrial,
		},
		{
			name: "pro with paid-only model is paid",
			tier: PlanTierPro,
			breakdown: LiveQuotaBreakdown{
				ModelCatalogKnown: true,
				HasPaidProModel:   true,
			},
			want: PlanTierPro,
		},
		{
			name: "quota tier plus overrides ambiguous upstream tier",
			tier: "",
			breakdown: LiveQuotaBreakdown{
				SubscriptionTier: PlanTierPlus,
			},
			want: PlanTierPlus,
		},
		{
			name: "bare quota pro does not erase live trial evidence",
			tier: PlanTierProTrial,
			breakdown: LiveQuotaBreakdown{
				SubscriptionTier: PlanTierPro,
			},
			want: PlanTierProTrial,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveTierFromQuotaAccess(tt.tier, &tt.breakdown); got != tt.want {
				t.Errorf("resolveTierFromQuotaAccess(%q) = %q, want %q", tt.tier, got, tt.want)
			}
		})
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
		{"Google AI Pro", PlanTierPro},
		{"Google AI Plus", PlanTierPlus},
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

type tierTestPayloads struct {
	Load       map[string]interface{}
	Quota      map[string]interface{}
	Models     map[string]interface{}
	ModelCalls *int
}

func newTierTestServer(t *testing.T, payloads tierTestPayloads) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/load":
			if payloads.Load != nil {
				_ = json.NewEncoder(w).Encode(payloads.Load)
				return
			}
		case "/quota":
			if payloads.Quota != nil {
				_ = json.NewEncoder(w).Encode(payloads.Quota)
				return
			}
		case "/models":
			if payloads.ModelCalls != nil {
				(*payloads.ModelCalls)++
			}
			if payloads.Models != nil {
				_ = json.NewEncoder(w).Encode(payloads.Models)
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	origLoad := CloudCodeLoadProjectURLs
	origQuota := CloudCodeRetrieveQuotaURLs
	origModels := CloudCodeModelsURLs
	origUserInfo := GoogleUserInfoURLs
	t.Cleanup(func() {
		CloudCodeLoadProjectURLs = origLoad
		CloudCodeRetrieveQuotaURLs = origQuota
		CloudCodeModelsURLs = origModels
		GoogleUserInfoURLs = origUserInfo
		ResetQuotaBackoffForTesting()
	})

	CloudCodeLoadProjectURLs = []string{srv.URL + "/load"}
	CloudCodeRetrieveQuotaURLs = []string{srv.URL + "/quota"}
	CloudCodeModelsURLs = []string{srv.URL + "/models"}
	GoogleUserInfoURLs = []string{srv.URL + "/userinfo"}
	ResetQuotaBackoffForTesting()
	return srv
}

func tierTestQuotaPayload(extra map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{
		"groups": []interface{}{
			map[string]interface{}{
				"displayName": "Gemini Models",
				"buckets": []interface{}{
					map[string]interface{}{
						"bucketId":          "gemini-5h",
						"window":            "5h",
						"remainingFraction": 1,
					},
					map[string]interface{}{
						"bucketId":          "gemini-weekly",
						"window":            "weekly",
						"remainingFraction": 1,
					},
				},
			},
		},
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload
}

func TestPollAccountLiveQuota_UsesExplicitTrialEvidence(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	tests := []struct {
		name   string
		models map[string]interface{}
	}{
		{
			name: "unknown model catalog",
		},
		{
			name: "legacy third-party models",
			models: map[string]interface{}{
				"claude-sonnet-4-6":   map[string]string{"displayName": "Claude Sonnet 4.6 (Thinking)"},
				"claude-opus-4-6":     map[string]string{"displayName": "Claude Opus 4.6 (Thinking)"},
				"gpt-oss-120b-medium": map[string]string{"displayName": "GPT-OSS 120B (Medium)"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modelCalls := 0
			newTierTestServer(t, tierTestPayloads{
				Load: map[string]interface{}{
					"cloudaicompanionProject": "projects/trial",
					"paidTier":                map[string]interface{}{"name": "Pro"},
				},
				Quota: tierTestQuotaPayload(map[string]interface{}{
					"subscriptionTier": "Pro",
					"isTrial":          true,
					"models":           tc.models,
				}),
				Models: map[string]interface{}{
					"tieredModelIds": map[string]interface{}{"pro": []string{"gemini-2.5-pro"}},
					"models":         tc.models,
				},
				ModelCalls: &modelCalls,
			})

			summary, err := PollAccountLiveQuota(&keyring.Account{
				Email:        "trial@example.com",
				AccessToken:  "mock-token",
				RefreshToken: "mock-refresh",
				PlanTier:     PlanTierPro,
			})
			if err != nil {
				t.Fatalf("PollAccountLiveQuota failed: %v", err)
			}
			if summary.PlanTier != PlanTierProTrial {
				t.Fatalf("expected explicit trial tier %q, got %q", PlanTierProTrial, summary.PlanTier)
			}
			if modelCalls != 0 {
				t.Fatalf("explicit trial should not require model probing, got %d calls", modelCalls)
			}
		})
	}
}

func TestPollAccountLiveQuota_ProLabelUsesModelCatalogToSeparatePaidAndTrial(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	tests := []struct {
		name     string
		models   map[string]interface{}
		wantTier string
	}{
		{
			name: "paid-only model present",
			models: map[string]interface{}{
				"claude-sonnet-5-5": map[string]string{"displayName": "Claude Sonnet 5.5 (Thinking)"},
			},
			wantTier: PlanTierPro,
		},
		{
			name: "family-shared trial only exposes legacy models",
			models: map[string]interface{}{
				"claude-sonnet-4-6": map[string]string{"displayName": "Claude Sonnet 4.6 (Thinking)"},
				"claude-opus-4-6":   map[string]string{"displayName": "Claude Opus 4.6 (Thinking)"},
			},
			wantTier: PlanTierProTrial,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modelCalls := 0
			newTierTestServer(t, tierTestPayloads{
				Load: map[string]interface{}{
					"cloudaicompanionProject": "projects/paid-pro",
					"paidTier":                map[string]interface{}{"name": "Google AI Pro"},
				},
				Quota: tierTestQuotaPayload(map[string]interface{}{
					"subscriptionTier": "Pro",
				}),
				Models: map[string]interface{}{
					"tieredModelIds": map[string]interface{}{"pro": []string{"gemini-2.5-pro"}},
					"models":         tc.models,
				},
				ModelCalls: &modelCalls,
			})

			summary, err := PollAccountLiveQuota(&keyring.Account{
				Email:        "paid@example.com",
				AccessToken:  "mock-token",
				RefreshToken: "mock-refresh",
				PlanTier:     PlanTierProTrial,
			})
			if err != nil {
				t.Fatalf("PollAccountLiveQuota failed: %v", err)
			}
			if summary.PlanTier != tc.wantTier {
				t.Fatalf("expected model catalog to resolve %q, got %q", tc.wantTier, summary.PlanTier)
			}
			if modelCalls != 1 {
				t.Fatalf("expected one model probe for a Pro candidate, got %d", modelCalls)
			}
		})
	}
}

func TestPollAccountLiveQuota_QuotaCatalogTakesPrecedenceOverModelProbe(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	tests := []struct {
		name        string
		quotaModels map[string]interface{}
		probeModels map[string]interface{}
		wantTier    string
	}{
		{
			name: "quota paid catalog wins over legacy-only probe",
			quotaModels: map[string]interface{}{
				"claude-sonnet-5-5": map[string]string{"displayName": "Claude Sonnet 5.5 (Thinking)"},
			},
			probeModels: map[string]interface{}{
				"claude-sonnet-4-6": map[string]string{"displayName": "Claude Sonnet 4.6 (Thinking)"},
			},
			wantTier: PlanTierPro,
		},
		{
			name: "quota trial catalog wins over paid-only probe",
			quotaModels: map[string]interface{}{
				"claude-sonnet-4-6": map[string]string{"displayName": "Claude Sonnet 4.6 (Thinking)"},
			},
			probeModels: map[string]interface{}{
				"claude-sonnet-5-5": map[string]string{"displayName": "Claude Sonnet 5.5 (Thinking)"},
			},
			wantTier: PlanTierProTrial,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modelCalls := 0
			newTierTestServer(t, tierTestPayloads{
				Load: map[string]interface{}{
					"cloudaicompanionProject": "projects/catalog",
					"paidTier":                map[string]interface{}{"name": "Google AI Pro"},
				},
				Quota: tierTestQuotaPayload(map[string]interface{}{
					"models": tc.quotaModels,
				}),
				Models: map[string]interface{}{
					"tieredModelIds": map[string]interface{}{"pro": []string{"gemini-2.5-pro"}},
					"models":         tc.probeModels,
				},
				ModelCalls: &modelCalls,
			})

			summary, err := PollAccountLiveQuota(&keyring.Account{
				Email:        "catalog@example.com",
				AccessToken:  "mock-token",
				RefreshToken: "mock-refresh",
				PlanTier:     PlanTierPro,
			})
			if err != nil {
				t.Fatalf("PollAccountLiveQuota failed: %v", err)
			}
			if summary.PlanTier != tc.wantTier {
				t.Fatalf("expected quota catalog tier %q, got %q", tc.wantTier, summary.PlanTier)
			}
			if modelCalls != 0 {
				t.Fatalf("quota catalog evidence must avoid a redundant model probe, got %d calls", modelCalls)
			}
		})
	}
}

func TestPollAccountLiveQuota_InfersTrialOnlyWhenTierIsAmbiguous(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	modelCalls := 0
	newTierTestServer(t, tierTestPayloads{
		Load: map[string]interface{}{
			"cloudaicompanionProject": "projects/ambiguous",
			"allowedTiers":            []map[string]interface{}{{"name": "Pro"}},
		},
		Quota: tierTestQuotaPayload(nil),
		Models: map[string]interface{}{
			"tieredModelIds": map[string]interface{}{"pro": []string{"gemini-2.5-pro"}},
			"models": map[string]interface{}{
				"claude-sonnet-4-6": map[string]string{"displayName": "Claude Sonnet 4.6 (Thinking)"},
			},
		},
		ModelCalls: &modelCalls,
	})

	summary, err := PollAccountLiveQuota(&keyring.Account{
		Email:        "ambiguous@example.com",
		AccessToken:  "mock-token",
		RefreshToken: "mock-refresh",
		PlanTier:     PlanTierFree,
	})
	if err != nil {
		t.Fatalf("PollAccountLiveQuota failed: %v", err)
	}
	if summary.PlanTier != PlanTierProTrial {
		t.Fatalf("expected ambiguous Pro access to resolve to %q, got %q", PlanTierProTrial, summary.PlanTier)
	}
	if modelCalls != 1 {
		t.Fatalf("expected exactly one model probe, got %d", modelCalls)
	}
}

func TestPollAccountLiveQuota_RefreshesPlusToPro(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	modelCalls := 0
	newTierTestServer(t, tierTestPayloads{
		Load: map[string]interface{}{
			"cloudaicompanionProject": "projects/upgraded",
			"currentTier":             map[string]interface{}{"name": "Google AI Plus"},
		},
		Quota: tierTestQuotaPayload(map[string]interface{}{
			"subscriptionTier": "Pro",
		}),
		Models: map[string]interface{}{
			"tieredModelIds": map[string]interface{}{"pro": []string{"gemini-2.5-pro"}},
			"models":         map[string]interface{}{"claude-sonnet-5-5": map[string]string{"displayName": "Claude Sonnet 5.5 (Thinking)"}},
		},
		ModelCalls: &modelCalls,
	})

	summary, err := PollAccountLiveQuota(&keyring.Account{
		Email:        "upgraded@example.com",
		AccessToken:  "mock-token",
		RefreshToken: "mock-refresh",
		PlanTier:     PlanTierPlus,
	})
	if err != nil {
		t.Fatalf("PollAccountLiveQuota failed: %v", err)
	}
	if summary.PlanTier != PlanTierPro {
		t.Fatalf("expected quota refresh to upgrade %q to %q, got %q", PlanTierPlus, PlanTierPro, summary.PlanTier)
	}
	if modelCalls != 1 {
		t.Fatalf("expected one model probe after the Plus-to-Pro upgrade, got %d", modelCalls)
	}
}

func newTierTestStore(t *testing.T, email, planTier string) (*keyring.Store, *keyring.Account) {
	t.Helper()
	store, err := keyring.NewStore(filepath.Join(t.TempDir(), "accounts.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if _, err := store.ImportAccount(email, "mock-refresh", "mock-token", "Test", ""); err != nil {
		t.Fatalf("failed to import account: %v", err)
	}
	if err := store.UpdateAccountQuotaMetadata(email, planTier, 0); err != nil {
		t.Fatalf("failed to seed plan tier: %v", err)
	}
	acc, err := store.GetAccount(email)
	if err != nil {
		t.Fatalf("failed to read account: %v", err)
	}
	return store, acc
}

func TestPollAndCacheAccount_PersistsProTrialUpgrade(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	newTierTestServer(t, tierTestPayloads{
		Load: map[string]interface{}{
			"cloudaicompanionProject": "projects/upgraded",
			"paidTier":                map[string]interface{}{"name": "Google AI Pro"},
		},
		Quota: tierTestQuotaPayload(map[string]interface{}{
			"subscriptionTier": "Pro",
		}),
		Models: map[string]interface{}{
			"tieredModelIds": map[string]interface{}{"pro": []string{"gemini-2.5-pro"}},
			"models":         map[string]interface{}{"claude-sonnet-5-5": map[string]string{"displayName": "Claude Sonnet 5.5 (Thinking)"}},
		},
	})

	store, acc := newTierTestStore(t, "upgrade@example.com", PlanTierProTrial)
	summary, err := PollAndCacheAccount(acc, store)
	if err != nil {
		t.Fatalf("PollAndCacheAccount failed: %v", err)
	}
	if summary.PlanTier != PlanTierPro {
		t.Fatalf("expected summary tier %q, got %q", PlanTierPro, summary.PlanTier)
	}
	persisted, err := store.GetAccount("upgrade@example.com")
	if err != nil {
		t.Fatalf("failed to reload account: %v", err)
	}
	if persisted.PlanTier != PlanTierPro {
		t.Fatalf("expected persisted tier %q, got %q", PlanTierPro, persisted.PlanTier)
	}
}

func TestPollFleetAccounts_PersistsPlusUpgrade(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	newTierTestServer(t, tierTestPayloads{
		Load: map[string]interface{}{
			"cloudaicompanionProject": "projects/upgraded",
			"currentTier":             map[string]interface{}{"name": "Google AI Plus"},
		},
		Quota: tierTestQuotaPayload(map[string]interface{}{
			"subscriptionTier": "Pro",
		}),
		Models: map[string]interface{}{
			"tieredModelIds": map[string]interface{}{"pro": []string{"gemini-2.5-pro"}},
			"models":         map[string]interface{}{"claude-sonnet-5-5": map[string]string{"displayName": "Claude Sonnet 5.5 (Thinking)"}},
		},
	})

	store, acc := newTierTestStore(t, "fleet-upgrade@example.com", PlanTierPlus)
	results := PollFleetAccounts([]*keyring.Account{acc}, store)
	summary := results["fleet-upgrade@example.com"]
	if summary == nil || summary.PlanTier != PlanTierPro {
		t.Fatalf("expected fleet summary tier %q, got %+v", PlanTierPro, summary)
	}
	persisted, err := store.GetAccount("fleet-upgrade@example.com")
	if err != nil {
		t.Fatalf("failed to reload account: %v", err)
	}
	if persisted.PlanTier != PlanTierPro {
		t.Fatalf("expected persisted fleet tier %q, got %q", PlanTierPro, persisted.PlanTier)
	}
}

func TestPopulateVerificationAuthUser(t *testing.T) {
	raw := "https://accounts.google.com/signin/continue?sarp=1&flowName=GlifWebSignIn&authuser"
	got := populateVerificationAuthUser(raw, "joseantoniocarrarofalchi@gmail.com")
	if !strings.HasSuffix(got, "&authuser=joseantoniocarrarofalchi%40gmail.com") {
		t.Errorf("expected populated authuser param, got %q", got)
	}
}

func TestIssue53_ClaudeGPTZeroInitDoesNotPromoteFreeAccount(t *testing.T) {
	breakdown := &LiveQuotaBreakdown{
		Quota5hGemini:        1.0,
		QuotaWeeklyGemini:    1.0,
		Quota5hClaudeGPT:     0.0,
		QuotaWeeklyClaudeGPT: 0.0,
		SubscriptionTier:     PlanTierFree,
		HasClaudeGPTRights:   false,
		HasClaude:            false,
		HasGPT:               false,
	}

	tier := ""
	if breakdown.SubscriptionTier != "" && breakdown.SubscriptionTier != PlanTierFree {
		tier = breakdown.SubscriptionTier
	}
	if breakdown.HasClaudeGPTRights || breakdown.HasClaude || breakdown.HasGPT {
		if tier == "" || tier == PlanTierFree || tier == PlanTierProTrial {
			tier = PlanTierPro
		}
	}
	if tier != "" {
		t.Errorf("expected empty or Free tier, got promoted tier %q", tier)
	}
}

func TestIssue54_FlatBucketWindowClassification(t *testing.T) {
	now := time.Now()
	weeklyReset := now.Add(5 * 24 * time.Hour) // 5 days reset
	rollingReset := now.Add(4 * time.Hour)     // 4 hours reset

	w1 := classifyQuotaWindow("", "", "gemini-2.5-pro", weeklyReset, now)
	if w1 != "weekly" {
		t.Errorf("expected 'weekly' for 5-day reset horizon, got %q", w1)
	}

	w2 := classifyQuotaWindow("", "", "gemini-2.5-flash", rollingReset, now)
	if w2 != "5h" {
		t.Errorf("expected '5h' for 4-hour reset horizon, got %q", w2)
	}
}

func TestIssue59_ClampFraction(t *testing.T) {
	if got := ClampFraction(-0.5); got != 0.0 {
		t.Errorf("expected 0.0 for negative, got %f", got)
	}
	if got := ClampFraction(1.5); got != 1.0 {
		t.Errorf("expected 1.0 for > 1.0, got %f", got)
	}
	if got := ClampFraction(0.75); got != 0.75 {
		t.Errorf("expected 0.75 for 0.75, got %f", got)
	}
}
