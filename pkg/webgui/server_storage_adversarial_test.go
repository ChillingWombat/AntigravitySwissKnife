package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/system"
)

func TestAdversarial_HTTPStorageLockdown(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys_config")
	appDir := filepath.Join(tmpDir, "app_dir")
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", sysDir)
	t.Setenv("ANTIGRAVITY_SWISS_APP_DIR", appDir)

	if err := os.MkdirAll(sysDir, 0700); err != nil {
		t.Fatalf("failed to create sysDir: %v", err)
	}
	if err := os.MkdirAll(appDir, 0700); err != nil {
		t.Fatalf("failed to create appDir: %v", err)
	}

	// Write legacy config with "app_portable"
	legacyConfig := filepath.Join(sysDir, "config.json")
	_ = os.WriteFile(legacyConfig, []byte(`{"storage_mode":"app_portable"}`), 0600)
	_ = os.WriteFile(filepath.Join(sysDir, "accounts.json"), []byte(`[{"id":"user1"}]`), 0600)

	srv := NewServer("127.0.0.1:0", filepath.Join(tmpDir, "test.sock"))
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()
	baseURL := "http://" + srv.Addr()

	// 1. GET /api/settings/storage -> must report system_default and can_migrate = false
	resp, err := http.Get(baseURL + "/api/settings/storage")
	if err != nil {
		t.Fatalf("GET /api/settings/storage failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var info system.StorageInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode StorageInfo failed: %v", err)
	}

	if info.StorageMode != "system_default" {
		t.Fatalf("expected storage_mode 'system_default', got %q", info.StorageMode)
	}
	if info.CanMigrate {
		t.Fatalf("expected can_migrate false, got true")
	}

	// 2. POST /api/settings/storage with "app_portable" and migrate_data: true
	// Must return 400 Bad Request, success: false, and never create ./data
	payload := []byte(`{"storage_mode":"app_portable","migrate_data":true}`)
	postResp, err := http.Post(baseURL+"/api/settings/storage", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/settings/storage failed: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for app_portable, got %d", postResp.StatusCode)
	}

	var errResult map[string]interface{}
	if err := json.NewDecoder(postResp.Body).Decode(&errResult); err != nil {
		t.Fatalf("decode error result failed: %v", err)
	}
	if errResult["success"] != false {
		t.Fatalf("expected success: false, got %v", errResult["success"])
	}

	// Verify ./data was NOT created
	if _, statErr := os.Stat(filepath.Join(appDir, "data")); !os.IsNotExist(statErr) {
		t.Fatalf("CRITICAL: ./data directory was created by HTTP endpoint!")
	}

	// 3. POST with directory traversal payload
	traversalPayload := []byte(`{"storage_mode":"../../../../evil","migrate_data":true}`)
	travResp, err := http.Post(baseURL+"/api/settings/storage", "application/json", bytes.NewReader(traversalPayload))
	if err != nil {
		t.Fatalf("POST /api/settings/storage failed: %v", err)
	}
	defer travResp.Body.Close()
	if travResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for traversal payload, got %d", travResp.StatusCode)
	}

	// 4. POST with invalid / malformed JSON
	malformedResp, err := http.Post(baseURL+"/api/settings/storage", "application/json", bytes.NewReader([]byte(`{"storage_mode":`)))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer malformedResp.Body.Close()
	if malformedResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for malformed JSON, got %d", malformedResp.StatusCode)
	}

	// 5. POST with system_default and migrate_data: true -> succeeds with 200 OK
	sysPayload := []byte(`{"storage_mode":"system_default","migrate_data":true}`)
	sysResp, err := http.Post(baseURL+"/api/settings/storage", "application/json", bytes.NewReader(sysPayload))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer sysResp.Body.Close()
	if sysResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for system_default, got %d", sysResp.StatusCode)
	}

	var okResult struct {
		Success bool               `json:"success"`
		Storage system.StorageInfo `json:"storage"`
	}
	if err := json.NewDecoder(sysResp.Body).Decode(&okResult); err != nil {
		t.Fatalf("decode ok result failed: %v", err)
	}
	if !okResult.Success {
		t.Fatalf("expected success: true, got false")
	}
	if okResult.Storage.StorageMode != "system_default" {
		t.Fatalf("expected storage_mode 'system_default', got %q", okResult.Storage.StorageMode)
	}
	if okResult.Storage.CanMigrate {
		t.Fatalf("expected can_migrate false, got true")
	}

	// 6. Test disallowed methods (PUT, DELETE)
	reqPut, _ := http.NewRequest(http.MethodPut, baseURL+"/api/settings/storage", nil)
	putResp, err := http.DefaultClient.Do(reqPut)
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 Method Not Allowed for PUT, got %d", putResp.StatusCode)
	}
}
