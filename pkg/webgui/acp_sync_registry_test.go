package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/acpmesh"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestHandleUtilitiesACPSyncRegistry(t *testing.T) {
	mockRegistry := `{
		"version": "1.0.0",
		"agents": [
			{
				"id": "antigravity-acp",
				"name": "Google Antigravity",
				"version": "1.4.3"
			}
		]
	}`

	mockCDN := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockRegistry))
	}))
	defer mockCDN.Close()

	// Ensure config dir is isolated
	t.Setenv("HOME", t.TempDir())
	cfg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	_ = cfg.SetACPAgents(nil)

	s := &Server{}

	// Verify method validation
	getReq := httptest.NewRequest(http.MethodGet, "/api/utilities/acp/sync-registry", nil)
	getRec := httptest.NewRecorder()
	s.handleUtilitiesACPSyncRegistry(getRec, getReq)
	if getRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on GET, got %d", getRec.Code)
	}

	// Verify successful sync
	reqBody, _ := json.Marshal(map[string]string{"registry_url": mockCDN.URL})
	postReq := httptest.NewRequest(http.MethodPost, "/api/utilities/acp/sync-registry", bytes.NewBuffer(reqBody))
	postRec := httptest.NewRecorder()
	s.handleUtilitiesACPSyncRegistry(postRec, postReq)

	if postRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST sync-registry, got %d: %s", postRec.Code, postRec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(postRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if synced, ok := resp["synced"].(float64); !ok || synced < 1 {
		t.Errorf("synced count = %v, want >= 1", resp["synced"])
	}

	agents, ok := resp["agents"].([]interface{})
	if !ok || len(agents) == 0 {
		t.Fatalf("expected agents array in response, got %v", resp["agents"])
	}

	var foundAntigravity map[string]interface{}
	for _, a := range agents {
		agentMap, _ := a.(map[string]interface{})
		if agentMap["id"] == acpmesh.PresetAntigravityACP {
			foundAntigravity = agentMap
			break
		}
	}

	if foundAntigravity == nil {
		t.Fatalf("PresetAntigravityACP missing from synced response")
	}
	if foundAntigravity["acp_version"] != "v1.4.3" {
		t.Errorf("acp_version = %v, want v1.4.3", foundAntigravity["acp_version"])
	}
}
