package webgui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
)

func TestCommunityExtensions_HTTPAPI(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webgui-comm-ext-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	client := &http.Client{}
	baseURL := "http://" + srv.Addr()
	token := srv.GetSessionToken()

	doReq := func(method, path string, body []byte) (*http.Response, map[string]interface{}) {
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequest(method, baseURL+path, reader)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("X-Session-Token", token)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var res map[string]interface{}
		_ = json.Unmarshal(respBody, &res)
		return resp, res
	}

	// 1. Initial GET /api/extensions/community
	resp, res := doReq(http.MethodGet, "/api/extensions/community", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/extensions/community returned %d, want 200", resp.StatusCode)
	}
	if res["success"] != true {
		t.Errorf("expected success: true, got %v", res["success"])
	}
	exts, ok := res["extensions"].([]interface{})
	if !ok {
		t.Fatalf("expected extensions array in response, got %T", res["extensions"])
	}
	initialCount := len(exts)

	// 2. POST single direct extension
	singleExt := enhancements.CommunityExtension{
		Name:       "Grafana Dashboards",
		SourceType: "url",
		URL:        "http://localhost:3000",
		Icon:       "globe",
		AuxPanel:   true,
		MainPage:   false,
	}
	payload, _ := json.Marshal(singleExt)
	resp, res = doReq(http.MethodPost, "/api/extensions/community", payload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST direct extension returned %d, want 200", resp.StatusCode)
	}
	if res["success"] != true {
		t.Errorf("expected success: true, got %v", res["success"])
	}
	savedMap, ok := res["extension"].(map[string]interface{})
	if !ok || savedMap["id"] != "grafana-dashboards" {
		t.Errorf("expected saved extension id 'grafana-dashboards', got %v", savedMap)
	}

	// 3. POST wrapper {"extension": {...}}
	wrappedPayload, _ := json.Marshal(map[string]interface{}{
		"extension": enhancements.CommunityExtension{
			ID:         "metrics-gadget",
			Name:       "Metrics Gadget",
			SourceType: "html",
			HTML:       "<div>metrics</div>",
			Icon:       "cpu",
			AuxPanel:   true,
			MainPage:   true,
		},
	})
	resp, res = doReq(http.MethodPost, "/api/extensions/community", wrappedPayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST wrapped extension returned %d, want 200", resp.StatusCode)
	}
	savedMap, _ = res["extension"].(map[string]interface{})
	if savedMap["id"] != "metrics-gadget" {
		t.Errorf("expected saved extension id 'metrics-gadget', got %v", savedMap)
	}

	// 4. POST batch import payload {"extensions": [...]}
	batchPayload, _ := json.Marshal(map[string]interface{}{
		"extensions": []enhancements.CommunityExtension{
			{
				ID:         "tool-a",
				Name:       "Tool A",
				SourceType: "url",
				URL:        "http://example.com/a",
			},
			{
				ID:         "tool-b",
				Name:       "Tool B",
				SourceType: "url",
				URL:        "http://example.com/b",
			},
		},
	})
	resp, res = doReq(http.MethodPost, "/api/extensions/community", batchPayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST batch envelope returned %d, want 200", resp.StatusCode)
	}
	if res["success"] != true {
		t.Errorf("expected success: true, got %v", res["success"])
	}

	// 5. POST validation errors -> 400 Bad Request
	invalidPayload, _ := json.Marshal(enhancements.CommunityExtension{
		Name: "",
	})
	resp, res = doReq(http.MethodPost, "/api/extensions/community", invalidPayload)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty name, got %d", resp.StatusCode)
	}
	if res["success"] != false {
		t.Errorf("expected success: false on validation error")
	}

	collisionPayload, _ := json.Marshal(enhancements.CommunityExtension{
		ID:   "browser",
		Name: "Browser",
		URL:  "http://example.com",
	})
	resp, _ = doReq(http.MethodPost, "/api/extensions/community", collisionPayload)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for builtin ID collision, got %d", resp.StatusCode)
	}

	// 6. DELETE /api/extensions/community?id=...
	resp, res = doReq(http.MethodDelete, "/api/extensions/community?id=tool-a", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE returned %d, want 200", resp.StatusCode)
	}
	if res["success"] != true || res["deleted"] != "tool-a" {
		t.Errorf("expected successful deletion of 'tool-a', got %v", res)
	}

	// 7. DELETE nonexistent -> 400 Bad Request
	resp, res = doReq(http.MethodDelete, "/api/extensions/community?id=nonexistent", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for deleting non-existent extension, got %d", resp.StatusCode)
	}

	// 8. Unsupported Method (PUT) -> 405 Method Not Allowed
	resp, _ = doReq(http.MethodPut, "/api/extensions/community", []byte("{}"))
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed for PUT, got %d", resp.StatusCode)
	}

	// Verify final count via GET
	resp, res = doReq(http.MethodGet, "/api/extensions/community", nil)
	finalExts, _ := res["extensions"].([]interface{})
	// We started with initialCount, added grafana-dashboards, metrics-gadget, tool-a, tool-b, then deleted tool-a
	expectedCount := initialCount + 3
	if len(finalExts) != expectedCount {
		t.Errorf("expected %d extensions after tests, got %d", expectedCount, len(finalExts))
	}
}
