package webgui

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
)

func TestCustomModelIntegrationReadRedactsPINAndPreservesItOnWrite(t *testing.T) {
	store, err := custommodels.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	settings := custommodels.DefaultIntegrationSettings()
	settings.Mode = custommodels.IntegrationAPIProxy
	settings.Proxy.Scope = custommodels.ProxyScopeExternal
	settings.Proxy.PIN = "secret-pin"
	if err := store.SetIntegrationSettings(settings); err != nil {
		t.Fatalf("SetIntegrationSettings: %v", err)
	}
	server := &Server{addr: "127.0.0.1:8765", customModelsStore: store}

	rec := callJSONHandler(t, server.handleCustomModelIntegration, http.MethodGet, "/api/custom_models/integration", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var getResponse struct {
		Integration custommodels.IntegrationSettings `json:"integration"`
		PINSet      bool                             `json:"pin_set"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&getResponse); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getResponse.Integration.Proxy.PIN != "" {
		t.Fatalf("GET leaked PIN %q", getResponse.Integration.Proxy.PIN)
	}
	if !getResponse.PINSet {
		t.Fatal("GET must report that a PIN is configured")
	}

	payload := getResponse.Integration
	payload.Proxy.PIN = ""
	rec = callJSONHandler(t, server.handleCustomModelIntegration, http.MethodPost, "/api/custom_models/integration", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := store.IntegrationSettings().Proxy.PIN; got != "secret-pin" {
		t.Fatalf("stored PIN = %q, want preserved secret", got)
	}
	var postResponse struct {
		Integration custommodels.IntegrationSettings `json:"integration"`
		PINSet      bool                             `json:"pin_set"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&postResponse); err != nil {
		t.Fatalf("decode POST response: %v", err)
	}
	if postResponse.Integration.Proxy.PIN != "" {
		t.Fatalf("POST leaked PIN %q", postResponse.Integration.Proxy.PIN)
	}
	if !postResponse.PINSet {
		t.Fatal("POST must report that a PIN is configured")
	}
}

func TestCustomModelCLIStatusRedactsGatewayToken(t *testing.T) {
	store, err := custommodels.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if token := store.GatewayToken(); token == "" {
		t.Fatal("expected a gateway token for the redaction test")
	}
	server := &Server{customModelsStore: store}

	rec := callJSONHandler(t, server.handleCustomModelCLIStatus, http.MethodGet, "/api/custom_models/cli_status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Status custommodels.CLIGatewayStatus `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status.GatewayToken != "" {
		t.Fatalf("CLI status leaked gateway token %q", response.Status.GatewayToken)
	}
}
