package quota

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

	// 2. Pro tier models returned
	proServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tieredModelIds": map[string]interface{}{
				"pro": []string{"gemini-2.5-pro"},
			},
		})
	}))
	defer proServer.Close()
	CloudCodeModelsURLs = []string{proServer.URL}

	tier, err = DetectTierFromAvailableModels("mock-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != PlanTierPro {
		t.Errorf("expected %s, got %s", PlanTierPro, tier)
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
}
