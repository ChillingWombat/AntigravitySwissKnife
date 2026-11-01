package webgui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
)

func TestCustomModelCredentialEndpointVisibility(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	t.Setenv("ANTIGRAVITY_DATA_DIR", tempDir)

	store, err := custommodels.NewStore(tempDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SaveModel(custommodels.CustomModel{
		ID:                   "api-model",
		Name:                 "openai-model",
		DisplayName:          "API Model",
		ProviderType:         custommodels.ProviderOpenAI,
		BaseURL:              "https://api.example.com/v1",
		APIKey:               "test-api-credential",
		GCPServiceAccountKey: "test-gcp-credential-on-api-model",
		Enabled:              true,
	}); err != nil {
		t.Fatalf("SaveModel(api): %v", err)
	}
	if err := store.SaveModel(custommodels.CustomModel{
		ID:                   "gcp-model-key-test",
		Name:                 "gemini-4-argon",
		DisplayName:          "GCP Model",
		ProviderType:         custommodels.ProviderGCP,
		BaseURL:              "https://aiplatform.googleapis.com/v1/projects/test/locations/global/publishers/google/models/gemini-4-argon:generateContent",
		APIKey:               "test-api-credential-on-gcp-model",
		GCPServiceAccountKey: `{"type":"service_account","project_id":"test","private_key":"test","client_email":"test@example.com"}`,
		Enabled:              true,
	}); err != nil {
		t.Fatalf("SaveModel(gcp): %v", err)
	}

	server := newTestServer("127.0.0.1:0", filepath.Join(tempDir, "dummy.sock"))
	server.SetCustomModelsStore(store)
	const token = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	server.SetSessionToken(token)
	if err := server.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer server.Stop()

	client := &http.Client{Transport: &http.Transport{}}
	request := func(target string, authenticated bool) *http.Response {
		req, err := http.NewRequest(http.MethodGet, "http://"+server.Addr()+target, nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Session-Token", token)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", target, err)
		}
		return resp
	}
	readCredential := func(field string) (map[string]interface{}, *http.Response, string) {
		id := "api-model"
		if field == "gcp_service_account_key" {
			id = "gcp-model-key-test"
		}
		resp := request("/api/custom_models/key?id="+id+"&field="+field, true)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read credential response: %v", err)
		}
		var payload map[string]interface{}
		if resp.StatusCode == http.StatusOK {
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode credential response: %v", err)
			}
		}
		return payload, resp, string(body)
	}

	directReq := httptest.NewRequest(http.MethodGet, "/api/custom_models/key?id=api-model&field=api_key", nil)
	directRec := httptest.NewRecorder()
	server.setupRoutes().ServeHTTP(directRec, directReq)
	if directRec.Code != http.StatusUnauthorized {
		t.Fatalf("direct unauthenticated status = %d, body = %s", directRec.Code, directRec.Body.String())
	}

	unauthorized := request("/api/custom_models/key?id=api-model&field=api_key", false)
	unauthorizedBody, _ := io.ReadAll(unauthorized.Body)
	unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401, body = %s", unauthorized.StatusCode, string(unauthorizedBody))
	}

	apiPayload, apiResp, apiBody := readCredential("api_key")
	if apiResp.StatusCode != http.StatusOK {
		t.Fatalf("api_key status = %d, body = %s", apiResp.StatusCode, apiBody)
	}
	if apiPayload["id"] != "api-model" || apiPayload["field"] != "api_key" || apiPayload["value"] != "test-api-credential" {
		t.Fatalf("unexpected api_key payload: %v", apiPayload)
	}
	if len(apiPayload) != 3 {
		t.Fatalf("api_key response leaked extra fields: %v", apiPayload)
	}
	if strings.Contains(apiBody, "test-gcp-credential-on-api-model") {
		t.Fatal("api_key response leaked the GCP credential")
	}
	if apiResp.Header.Get("Cache-Control") != "no-store" || apiResp.Header.Get("Pragma") != "no-cache" {
		t.Fatalf("missing no-store headers: Cache-Control=%q Pragma=%q", apiResp.Header.Get("Cache-Control"), apiResp.Header.Get("Pragma"))
	}

	gcpPayload, gcpResp, gcpBody := readCredential("gcp_service_account_key")
	if gcpResp.StatusCode != http.StatusOK {
		t.Fatalf("gcp status = %d, body = %s", gcpResp.StatusCode, gcpBody)
	}
	if gcpPayload["value"] != `{"type":"service_account","project_id":"test","private_key":"test","client_email":"test@example.com"}` || len(gcpPayload) != 3 {
		t.Fatalf("unexpected gcp payload: %v", gcpPayload)
	}
	if strings.Contains(gcpBody, "test-api-credential-on-gcp-model") {
		t.Fatal("gcp response leaked the API key")
	}

	disallowed := request("/api/custom_models/key?id=api-model&field=both", true)
	if disallowed.StatusCode != http.StatusBadRequest {
		t.Fatalf("disallowed field status = %d, want 400", disallowed.StatusCode)
	}
	disallowed.Body.Close()

	missing := request("/api/custom_models/key?id=missing-model&field=api_key", true)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing model status = %d, want 404", missing.StatusCode)
	}
	missing.Body.Close()

	listResp := request("/api/custom_models", true)
	defer listResp.Body.Close()
	var config custommodels.Config
	if err := json.NewDecoder(listResp.Body).Decode(&config); err != nil {
		t.Fatalf("decode model list: %v", err)
	}
	for _, model := range config.Models {
		if model.APIKey != "[REDACTED]" || model.GCPServiceAccountKey != "[REDACTED]" {
			t.Fatalf("model list leaked credentials for %s", model.ID)
		}
	}
}

func TestCustomModelCredentialEndpointRejectsPost(t *testing.T) {
	store, err := custommodels.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	server := &Server{customModelsStore: store}
	const token = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	server.SetSessionToken(token)
	req := httptest.NewRequest(http.MethodPost, "/api/custom_models/key?id=model&field=api_key", bytes.NewReader(nil))
	req.Header.Set("X-Session-Token", token)
	rec := httptest.NewRecorder()
	server.handleCustomModelKey(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
}
