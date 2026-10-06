package webgui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

func TestWebGUIServesMinimalistLightHTML(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "") // port 0 for random free port
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	// Fetch index
	resp, err := http.Get("http://" + srv.Addr() + "/")
	if err != nil {
		t.Fatalf("http.Get error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if !strings.Contains(html, "Antigravity Swiss Knife") {
		t.Errorf("expected HTML to contain title")
	}
	if !strings.Contains(html, "#f0f4f9") {
		t.Errorf("expected HTML to contain light surface token #f0f4f9")
	}
}

func TestWebGUIEndpoints(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/gui/config
	resp, err := http.Get(baseURL + "/api/gui/config")
	if err != nil {
		t.Fatalf("GET /api/gui/config failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var cfg gui.Config
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}
	resp.Body.Close()

	if !cfg.Enabled || !cfg.ColorStylingEnabled {
		t.Errorf("expected default Enabled and ColorStylingEnabled to be true")
	}

	// 2. POST /api/gui/projects/color
	colorPayload := map[string]string{
		"name":  "TestProject",
		"color": "#123456",
	}
	bodyJSON, _ := json.Marshal(colorPayload)
	resp, err = http.Post(baseURL+"/api/gui/projects/color", "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		t.Fatalf("POST /api/gui/projects/color failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. GET /api/gui/projects
	resp, err = http.Get(baseURL + "/api/gui/projects")
	if err != nil {
		t.Fatalf("GET /api/gui/projects failed: %v", err)
	}
	var projects []gui.ProjectItem
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		t.Fatalf("failed to decode projects: %v", err)
	}
	resp.Body.Close()

	foundTest := false
	for _, p := range projects {
		if p.Name == "TestProject" {
			foundTest = true
			if p.Color != "#123456" {
				t.Errorf("expected #123456, got %s", p.Color)
			}
		}
	}
	if !foundTest {
		t.Errorf("TestProject should be in projects list")
	}

	// 4. POST /api/gui/projects/order
	orderPayload := map[string]interface{}{
		"order": []string{"TestProject", "Arbitrager"},
	}
	orderJSON, _ := json.Marshal(orderPayload)
	resp, err = http.Post(baseURL+"/api/gui/projects/order", "application/json", bytes.NewReader(orderJSON))
	if err != nil {
		t.Fatalf("POST /api/gui/projects/order failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. POST /api/gui/projects/delete
	delPayload := map[string]string{"name": "TestProject"}
	delJSON, _ := json.Marshal(delPayload)
	resp, err = http.Post(baseURL+"/api/gui/projects/delete", "application/json", bytes.NewReader(delJSON))
	if err != nil {
		t.Fatalf("POST /api/gui/projects/delete failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWebGUISystemInstallationsEndpoints(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/system/installations
	resp, err := http.Get(baseURL + "/api/system/installations")
	if err != nil {
		t.Fatalf("GET /api/system/installations failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var sysInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sysInfo); err != nil {
		t.Fatalf("failed to decode system info: %v", err)
	}
	resp.Body.Close()

	if _, ok := sysInfo["desktop_app"]; !ok {
		t.Errorf("expected desktop_app in system installations")
	}
	if _, ok := sysInfo["vscode_extension"]; !ok {
		t.Errorf("expected vscode_extension in system installations")
	}
	if _, ok := sysInfo["platform"]; !ok {
		t.Errorf("expected platform in system installations")
	}

	// 2. POST /api/system/check_updates
	resp, err = http.Post(baseURL+"/api/system/check_updates", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/system/check_updates failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWebGUICustomModelsEndpoints(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/custom_models/presets
	resp, err := http.Get(baseURL + "/api/custom_models/presets")
	if err != nil {
		t.Fatalf("GET /api/custom_models/presets failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var presets []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&presets); err != nil {
		t.Fatalf("failed to decode presets: %v", err)
	}
	resp.Body.Close()
	if len(presets) < 4 {
		t.Errorf("expected at least 4 presets, got %d", len(presets))
	}

	// 2. GET /api/custom_models
	resp, err = http.Get(baseURL + "/api/custom_models")
	if err != nil {
		t.Fatalf("GET /api/custom_models failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var cmCfg map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&cmCfg); err != nil {
		t.Fatalf("failed to decode custom models config: %v", err)
	}
	resp.Body.Close()

	// 3. POST /api/custom_models (create custom model)
	newModel := map[string]interface{}{
		"id":               "test-ollama-qwen",
		"name":             "qwen2.5-coder:32b",
		"display_name":     "Qwen 2.5 Coder 32B (Local)",
		"provider_type":    "local",
		"base_url":         "http://localhost:11434/v1",
		"project_mappings": []string{"Antigravity Swiss Knife"},
		"quota_type":       "none",
		"enabled":          true,
	}
	data, _ := json.Marshal(newModel)
	resp, err = http.Post(baseURL+"/api/custom_models", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST /api/custom_models failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. POST /api/custom_models/bind
	bindPayload := map[string]string{
		"project":  "Antigravity Swiss Knife",
		"model_id": "test-ollama-qwen",
	}
	bData, _ := json.Marshal(bindPayload)
	resp, err = http.Post(baseURL+"/api/custom_models/bind", "application/json", bytes.NewReader(bData))
	if err != nil {
		t.Fatalf("POST /api/custom_models/bind failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. DELETE /api/custom_models?id=test-ollama-qwen
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/api/custom_models?id=test-ollama-qwen", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/custom_models failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWebGUIEnhancementsAndTemplatesEndpoints(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/enhancements
	resp, err := http.Get(baseURL + "/api/enhancements")
	if err != nil {
		t.Fatalf("GET /api/enhancements failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var enhCfg map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&enhCfg); err != nil {
		t.Fatalf("failed to decode enhancements config: %v", err)
	}
	resp.Body.Close()

	if enhCfg["enabled"] != true {
		t.Errorf("expected enhancements enabled true")
	}

	// 2. POST /api/enhancements/update
	enhCfg["tool_density_mode"] = "muted"
	bData, _ := json.Marshal(enhCfg)
	resp, err = http.Post(baseURL+"/api/enhancements/update", "application/json", bytes.NewReader(bData))
	if err != nil {
		t.Fatalf("POST /api/enhancements/update failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. GET /api/templates
	resp, err = http.Get(baseURL + "/api/templates")
	if err != nil {
		t.Fatalf("GET /api/templates failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var tmpls []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tmpls); err != nil {
		t.Fatalf("failed to decode templates: %v", err)
	}
	resp.Body.Close()

	if len(tmpls) == 0 {
		t.Errorf("expected templates catalog not to be empty")
	}

	// 4. GET /api/templates?id=daily-news-feed-digest
	resp, err = http.Get(baseURL + "/api/templates?id=daily-news-feed-digest")
	if err != nil {
		t.Fatalf("GET /api/templates?id=... failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. GET /api/templates/sidecars
	resp, err = http.Get(baseURL + "/api/templates/sidecars")
	if err != nil {
		t.Fatalf("GET /api/templates/sidecars failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWebGUIConversationTabsAndAutoArchiveEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	srv := NewServer("127.0.0.1:0", filepath.Join(tempDir, "isolated.sock"))
	tempStore, err := gui.NewStore(tempDir)
	if err != nil {
		t.Fatalf("gui.NewStore error: %v", err)
	}
	srv.SetGUIStore(tempStore)

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/gui/config - verify defaults
	resp, err := http.Get(baseURL + "/api/gui/config")
	if err != nil {
		t.Fatalf("GET /api/gui/config failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var cfg gui.Config
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}
	resp.Body.Close()

	if cfg.ConversationTabsMode != "dynamic" {
		t.Errorf("expected default tabs mode 'dynamic', got %q", cfg.ConversationTabsMode)
	}
	if cfg.ConversationTabsFixedLimit != 6 {
		t.Errorf("expected default tabs limit 6, got %d", cfg.ConversationTabsFixedLimit)
	}
	if cfg.ConversationTabsAgeThreshold != "14d" {
		t.Errorf("expected default age threshold '14d', got %q", cfg.ConversationTabsAgeThreshold)
	}
	if cfg.ConversationTabsMin != 3 {
		t.Errorf("expected default min tabs 3, got %d", cfg.ConversationTabsMin)
	}
	if cfg.ConversationTabsMax != 6 {
		t.Errorf("expected default max tabs 6, got %d", cfg.ConversationTabsMax)
	}
	if cfg.AutoArchiveHorizon != "14d" {
		t.Errorf("expected default auto archive horizon '14d', got %q", cfg.AutoArchiveHorizon)
	}
	if !cfg.AutoArchiveConversations {
		t.Errorf("expected default auto archive conversations true, got false")
	}

	// 2. POST /api/gui/config - update to dynamic mode with custom thresholds
	cfg.ConversationTabsMode = "dynamic"
	cfg.ConversationTabsAgeThreshold = "3d"
	cfg.ConversationTabsMin = 3
	cfg.ConversationTabsMax = 8
	cfg.AutoArchiveConversations = true
	cfg.AutoArchiveHorizon = "60d"

	bodyJSON, _ := json.Marshal(cfg)
	resp, err = http.Post(baseURL+"/api/gui/config", "application/json", bytes.NewReader(bodyJSON))
	if err != nil {
		t.Fatalf("POST /api/gui/config failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. GET /api/gui/config - verify persisted updates
	resp, err = http.Get(baseURL + "/api/gui/config")
	if err != nil {
		t.Fatalf("GET /api/gui/config failed: %v", err)
	}
	var updatedCfg gui.Config
	_ = json.NewDecoder(resp.Body).Decode(&updatedCfg)
	resp.Body.Close()

	if updatedCfg.ConversationTabsMode != "dynamic" {
		t.Errorf("expected updated tabs mode 'dynamic', got %q", updatedCfg.ConversationTabsMode)
	}
	if updatedCfg.ConversationTabsAgeThreshold != "3d" {
		t.Errorf("expected updated age threshold '3d', got %q", updatedCfg.ConversationTabsAgeThreshold)
	}
	if updatedCfg.ConversationTabsMin != 3 {
		t.Errorf("expected updated min tabs 3, got %d", updatedCfg.ConversationTabsMin)
	}
	if updatedCfg.ConversationTabsMax != 8 {
		t.Errorf("expected updated max tabs 8, got %d", updatedCfg.ConversationTabsMax)
	}
	if !updatedCfg.AutoArchiveConversations {
		t.Errorf("expected auto archive enabled true")
	}
	if updatedCfg.AutoArchiveHorizon != "60d" {
		t.Errorf("expected updated auto archive horizon '60d', got %q", updatedCfg.AutoArchiveHorizon)
	}

	// 4. POST /api/gui/conversations/auto-archive
	archivePayload := map[string]string{"horizon": "90d"}
	archJSON, _ := json.Marshal(archivePayload)
	resp, err = http.Post(baseURL+"/api/gui/conversations/auto-archive", "application/json", bytes.NewReader(archJSON))
	if err != nil {
		t.Fatalf("POST /api/gui/conversations/auto-archive failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var archRes gui.AutoArchiveResult
	if err := json.NewDecoder(resp.Body).Decode(&archRes); err != nil {
		t.Fatalf("failed to decode auto-archive response: %v", err)
	}
	resp.Body.Close()

	if !archRes.Success {
		t.Errorf("expected auto-archive success true, got false (%s)", archRes.Message)
	}
	if archRes.Horizon != "90d" {
		t.Errorf("expected horizon '90d', got %q", archRes.Horizon)
	}
}

func TestWebGUIFileExplorerEndpoints(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	tempDir := t.TempDir()

	// 1. Create directory via POST /api/files/create
	subDir := filepath.Join(tempDir, "subfolder")
	createDirPayload := map[string]interface{}{"path": subDir, "is_dir": true}
	cdData, _ := json.Marshal(createDirPayload)
	resp, err := http.Post(baseURL+"/api/files/create", "application/json", bytes.NewReader(cdData))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/create dir failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Write file via POST /api/files/write
	filePath := filepath.Join(subDir, "hello.txt")
	writeFilePayload := map[string]string{"path": filePath, "content": "Hello Antigravity!"}
	wfData, _ := json.Marshal(writeFilePayload)
	resp, err = http.Post(baseURL+"/api/files/write", "application/json", bytes.NewReader(wfData))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/write failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Read file via GET /api/files/read
	resp, err = http.Get(baseURL + "/api/files/read?path=" + filePath)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/files/read failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var readRes struct {
		Success bool   `json:"success"`
		Content string `json:"content"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&readRes)
	resp.Body.Close()
	if !readRes.Success || readRes.Content != "Hello Antigravity!" {
		t.Errorf("read file mismatch: expected 'Hello Antigravity!', got %q", readRes.Content)
	}

	// 4. List files via GET /api/files/list
	resp, err = http.Get(baseURL + "/api/files/list?path=" + subDir)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/files/list failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var listRes struct {
		Success bool       `json:"success"`
		Files   []FileItem `json:"files"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&listRes)
	resp.Body.Close()
	if !listRes.Success || len(listRes.Files) == 0 || listRes.Files[0].Name != "hello.txt" {
		t.Errorf("list files mismatch: expected hello.txt in listing")
	}

	// 5. Copy file via POST /api/files/copy
	copyPath := filepath.Join(subDir, "hello_copy.txt")
	copyPayload := map[string]string{"src": filePath, "dst": copyPath}
	cpData, _ := json.Marshal(copyPayload)
	resp, err = http.Post(baseURL+"/api/files/copy", "application/json", bytes.NewReader(cpData))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/copy failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 6. Rename/move file via POST /api/files/rename
	renamedPath := filepath.Join(subDir, "hello_renamed.txt")
	renamePayload := map[string]string{"old_path": copyPath, "new_path": renamedPath}
	rnData, _ := json.Marshal(renamePayload)
	resp, err = http.Post(baseURL+"/api/files/rename", "application/json", bytes.NewReader(rnData))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/rename failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 7. Delete file via POST /api/files/delete
	delPayload := map[string]string{"path": renamedPath}
	dlData, _ := json.Marshal(delPayload)
	resp, err = http.Post(baseURL+"/api/files/delete", "application/json", bytes.NewReader(dlData))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/delete failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 8. Reveal file via POST /api/files/reveal
	revPayload := map[string]string{"path": filePath}
	rvData, _ := json.Marshal(revPayload)
	resp, err = http.Post(baseURL+"/api/files/reveal", "application/json", bytes.NewReader(rvData))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/reveal failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWebGUIMemosEndpoints(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. Save memo via POST /api/memos/save
	savePayload := map[string]interface{}{
		"title":   "Test Memo",
		"content": "Quick reminder for Antigravity",
		"type":    "text",
		"tags":    []string{"test", "quick"},
	}
	sData, _ := json.Marshal(savePayload)
	resp, err := http.Post(baseURL+"/api/memos/save", "application/json", bytes.NewReader(sData))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/memos/save failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var saveRes struct {
		Success bool `json:"success"`
		Memo    struct {
			ID string `json:"id"`
		} `json:"memo"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&saveRes)
	resp.Body.Close()
	if !saveRes.Success || saveRes.Memo.ID == "" {
		t.Fatalf("expected memo save success with id")
	}

	// 2. List memos via GET /api/memos
	resp, err = http.Get(baseURL + "/api/memos")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/memos failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var listRes struct {
		Success bool `json:"success"`
		Memos   []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"memos"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&listRes)
	resp.Body.Close()
	if !listRes.Success || len(listRes.Memos) == 0 {
		t.Fatalf("expected at least 1 memo in list")
	}

	// 3. Delete memo via POST /api/memos/delete
	resp, err = http.Post(baseURL+"/api/memos/delete?id="+saveRes.Memo.ID, "application/json", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/memos/delete failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWebGUIUtilitiesImportEndpoints(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/utilities/import/scan?source=claude-code
	resp, err := http.Get(baseURL + "/api/utilities/import/scan?source=claude-code")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/utilities/import/scan failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var scanRes struct {
		Success bool `json:"success"`
		Source  string `json:"source"`
		Count   int `json:"count"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&scanRes)
	resp.Body.Close()
	if !scanRes.Success || scanRes.Source != "claude-code" {
		t.Errorf("expected successful scan for claude-code, got %+v", scanRes)
	}

	// 2. POST /api/utilities/import with empty list
	payload := map[string]interface{}{
		"candidate_ids": []string{},
		"source": "claude-code",
		"mode": "auto",
	}
	pData, _ := json.Marshal(payload)
	resp, err = http.Post(baseURL+"/api/utilities/import", "application/json", bytes.NewReader(pData))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/utilities/import failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWebGUIAvailableModelsAndRules(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	srv := NewServer("127.0.0.1:0", filepath.Join(tempDir, "daemon.sock"))
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/models/available
	resp, err := http.Get(baseURL + "/api/models/available")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/models/available failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var cat quota.AvailableModelsCatalog
	if err := json.NewDecoder(resp.Body).Decode(&cat); err != nil {
		t.Fatalf("failed to decode AvailableModelsCatalog: %v", err)
	}
	resp.Body.Close()

	if !cat.Success {
		t.Errorf("expected cat.Success to be true")
	}
	if cat.DefaultGemini != "gemini-3.8-flash" {
		t.Errorf("expected default gemini 'gemini-3.8-flash', got '%s'", cat.DefaultGemini)
	}
	if cat.DefaultNonGemini != "claude-opus-4-6" {
		t.Errorf("expected default non-gemini 'claude-opus-4-6', got '%s'", cat.DefaultNonGemini)
	}
	if len(cat.GeminiModels) == 0 {
		t.Errorf("expected non-empty GeminiModels")
	}
	if len(cat.NonGeminiModels) == 0 {
		t.Errorf("expected non-empty NonGeminiModels")
	}

	// 2. GET /api/rules default reasoning level
	resp, err = http.Get(baseURL + "/api/rules")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/rules failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var rules map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		t.Fatalf("failed to decode rules: %v", err)
	}
	resp.Body.Close()

	if lvl, ok := rules["default_gemini_reasoning_level"].(string); !ok || lvl != "high" {
		t.Errorf("expected default_gemini_reasoning_level 'high', got '%v'", rules["default_gemini_reasoning_level"])
	}

	// 3. POST /api/rules to update reasoning level and default models
	postData := map[string]interface{}{
		"default_gemini_reasoning_level": "medium",
		"default_custom_model":          "custom-first-model",
		"default_gemini_model":          "gemini-3.8-pro",
		"default_non_gemini_model":      "claude-3-7-sonnet",
	}
	pBytes, _ := json.Marshal(postData)
	resp, err = http.Post(baseURL+"/api/rules", "application/json", bytes.NewReader(pBytes))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/rules failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 4. Verify updated reasoning level and model fields
	resp, err = http.Get(baseURL + "/api/rules")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/rules failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var updatedRules map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&updatedRules)
	resp.Body.Close()

	if lvl, ok := updatedRules["default_gemini_reasoning_level"].(string); !ok || lvl != "medium" {
		t.Errorf("expected default_gemini_reasoning_level 'medium', got '%v'", updatedRules["default_gemini_reasoning_level"])
	}
	if cm, ok := updatedRules["default_custom_model"].(string); !ok || cm != "custom-first-model" {
		t.Errorf("expected default_custom_model 'custom-first-model', got '%v'", updatedRules["default_custom_model"])
	}
	if gm, ok := updatedRules["default_gemini_model"].(string); !ok || gm != "gemini-3.8-pro" {
		t.Errorf("expected default_gemini_model 'gemini-3.8-pro', got '%v'", updatedRules["default_gemini_model"])
	}
	if ngm, ok := updatedRules["default_non_gemini_model"].(string); !ok || ngm != "claude-3-7-sonnet" {
		t.Errorf("expected default_non_gemini_model 'claude-3-7-sonnet', got '%v'", updatedRules["default_non_gemini_model"])
	}
}

func TestWebGUICORSMiddlewareAndColorEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	guiStore, err := gui.NewStore(filepath.Join(tempDir, "gui_config.json"))
	if err != nil {
		t.Fatalf("failed to init guiStore: %v", err)
	}
	cfg := gui.DefaultConfig()
	cfg.Enabled = true
	cfg.ColorStylingEnabled = true
	cfg.ProjectOrder = []string{"Project Alpha", "Project Beta"}
	_ = guiStore.UpdateConfig(cfg)

	srv := NewServer("127.0.0.1:0", filepath.Join(tempDir, "daemon.sock"))
	srv.SetGUIStore(guiStore)
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	client := &http.Client{}

	// 1. CORS Preflight: OPTIONS /api/gui/color from vscode-file origin
	req, _ := http.NewRequest(http.MethodOptions, baseURL+"/api/gui/color", nil)
	req.Header.Set("Origin", "vscode-file://vscode-app")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /api/gui/color failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 No Content for CORS preflight, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "vscode-file://vscode-app" {
		t.Errorf("expected Access-Control-Allow-Origin 'vscode-file://vscode-app', got '%s'", got)
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Methods"), "POST") {
		t.Errorf("expected POST in Access-Control-Allow-Methods, got '%s'", resp.Header.Get("Access-Control-Allow-Methods"))
	}

	// 2. CORS Preflight from localhost origin
	req, _ = http.NewRequest(http.MethodOptions, baseURL+"/api/gui/color", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /api/gui/color failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 for localhost preflight, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://localhost:5173', got '%s'", got)
	}

	// 3. Set Color via POST /api/gui/color
	colorPayload, _ := json.Marshal(map[string]string{
		"name":  "Project Alpha",
		"color": "#7c3aed",
	})
	req, _ = http.NewRequest(http.MethodPost, baseURL+"/api/gui/color", bytes.NewReader(colorPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/gui/color failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	currCfg := guiStore.GetConfig()
	if currCfg.ProjectColors["Project Alpha"] != "#7c3aed" {
		t.Errorf("expected color #7c3aed, got '%s'", currCfg.ProjectColors["Project Alpha"])
	}

	// 4. Delete Color via POST /api/gui/color/delete
	delPayload, _ := json.Marshal(map[string]string{
		"name": "Project Alpha",
	})
	req, _ = http.NewRequest(http.MethodPost, baseURL+"/api/gui/color/delete", bytes.NewReader(delPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/gui/color/delete failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// Verify color was removed, but project order and project settings were NOT wiped!
	afterDelCfg := guiStore.GetConfig()
	if _, exists := afterDelCfg.ProjectColors["Project Alpha"]; exists {
		t.Errorf("expected Project Alpha color to be removed, but still exists: %s", afterDelCfg.ProjectColors["Project Alpha"])
	}
	if len(afterDelCfg.ProjectOrder) != 2 || afterDelCfg.ProjectOrder[0] != "Project Alpha" {
		t.Errorf("expected ProjectOrder to be preserved, got %v", afterDelCfg.ProjectOrder)
	}

	// 5. Test alias /api/gui/projects/color/delete
	_ = guiStore.SetProjectColor("Project Beta", "#059669")
	betaPayload, _ := json.Marshal(map[string]string{"name": "Project Beta"})
	req, _ = http.NewRequest(http.MethodPost, baseURL+"/api/gui/projects/color/delete", bytes.NewReader(betaPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/gui/projects/color/delete failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	afterBetaCfg := guiStore.GetConfig()
	if _, exists := afterBetaCfg.ProjectColors["Project Beta"]; exists {
		t.Errorf("expected Project Beta color to be removed, but still exists")
	}
	if len(afterBetaCfg.ProjectOrder) != 2 {
		t.Errorf("expected ProjectOrder to remain intact, got %v", afterBetaCfg.ProjectOrder)
	}
}

func TestWebGUISettingsStorageAndPrivacy(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", filepath.Join(tmpDir, "config"))
	t.Setenv("ANTIGRAVITY_SWISS_PORTABLE_DIR", filepath.Join(tmpDir, "portable_data"))

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/settings/storage
	resp, err := http.Get(baseURL + "/api/settings/storage")
	if err != nil {
		t.Fatalf("GET /api/settings/storage failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var storageInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&storageInfo); err != nil {
		t.Fatalf("decode storageInfo error: %v", err)
	}
	resp.Body.Close()

	if storageInfo["storage_mode"] != "system_default" {
		t.Errorf("expected storage_mode to be system_default, got %v", storageInfo["storage_mode"])
	}

	// 2. POST /api/settings/storage to switch to app_portable
	switchBody, _ := json.Marshal(map[string]interface{}{
		"storage_mode": "app_portable",
		"migrate_data": true,
	})
	resp, err = http.Post(baseURL+"/api/settings/storage", "application/json", bytes.NewReader(switchBody))
	if err != nil {
		t.Fatalf("POST /api/settings/storage failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var switchResp map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&switchResp)
	resp.Body.Close()
	if switchResp["success"] != true {
		t.Errorf("expected switch success true, got %v", switchResp)
	}

	// 3. GET /api/settings/privacy
	resp, err = http.Get(baseURL + "/api/settings/privacy")
	if err != nil {
		t.Fatalf("GET /api/settings/privacy failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var privInfo map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&privInfo)
	resp.Body.Close()
	if privInfo["github_repo"] != "ChillingWombat/AntigravitySwissKnife" {
		t.Errorf("expected github_repo, got %v", privInfo["github_repo"])
	}

	// 4. POST /api/settings/privacy
	privBody, _ := json.Marshal(map[string]interface{}{
		"anonymous_error_reports": false,
		"anonymous_telemetry":     true,
	})
	resp, err = http.Post(baseURL+"/api/settings/privacy", "application/json", bytes.NewReader(privBody))
	if err != nil {
		t.Fatalf("POST /api/settings/privacy failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updatePrivResp map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&updatePrivResp)
	resp.Body.Close()
	if updatePrivResp["anonymous_error_reports"] != false || updatePrivResp["anonymous_telemetry"] != true {
		t.Errorf("unexpected updated privacy response: %v", updatePrivResp)
	}

	// 5. POST /api/settings/diagnose-issue
	diagBody, _ := json.Marshal(map[string]interface{}{
		"description":         "Daemon IPC socket test error with email dev@company.com and secret sk-1234567890abcdef1234567890",
		"include_system_info": true,
		"include_logs":        true,
	})
	resp, err = http.Post(baseURL+"/api/settings/diagnose-issue", "application/json", bytes.NewReader(diagBody))
	if err != nil {
		t.Fatalf("POST /api/settings/diagnose-issue failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var diagResult map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&diagResult)
	resp.Body.Close()

	if diagResult["success"] != true {
		t.Errorf("expected diagResult.success true, got %v", diagResult)
	}
	if diagResult["sensitive_data_redacted"] != true {
		t.Errorf("expected sensitive_data_redacted true, got %v", diagResult)
	}
	reportStr, _ := diagResult["sanitized_report"].(string)
	if strings.Contains(reportStr, "dev@company.com") || strings.Contains(reportStr, "sk-1234567890abcdef1234567890") {
		t.Errorf("sanitized report contains unredacted credentials: %s", reportStr)
	}
}

func TestCORSAndPrivateNetworkAccessHeaders(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	allowedOrigins := []string{
		"vscode-app://antigravity",
		"electron://main",
		"antigravity://workbench",
		"plugin://swiss-tools",
		"http://localhost:5173",
		"http://127.0.0.1:3000",
	}

	for _, origin := range allowedOrigins {
		req, _ := http.NewRequest("OPTIONS", baseURL+"/api/gui/config", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Private-Network", "true")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("OPTIONS request failed: %v", err)
		}
		resp.Body.Close()

		if resp.Header.Get("Access-Control-Allow-Private-Network") != "true" {
			t.Errorf("expected Access-Control-Allow-Private-Network: true for origin %s", origin)
		}
		if resp.Header.Get("Access-Control-Allow-Origin") != origin {
			t.Errorf("expected Access-Control-Allow-Origin: %s, got %s", origin, resp.Header.Get("Access-Control-Allow-Origin"))
		}
	}
}

func TestFilesEndpointsWithSpacesAndEncoding(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// Create temp directory with spaces
	tempDir := t.TempDir()
	spaceFolder := filepath.Join(tempDir, "Antigravity Project With Spaces")
	if err := os.MkdirAll(spaceFolder, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	testFile := filepath.Join(spaceFolder, "space sample.md")
	content := "# Space Test Content"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 1. cleanUserPath verification
	clean1 := cleanUserPath(spaceFolder)
	if clean1 != spaceFolder {
		t.Errorf("expected %q, got %q", spaceFolder, clean1)
	}

	clean2 := cleanUserPath("file://" + spaceFolder)
	if clean2 != spaceFolder {
		t.Errorf("expected %q, got %q", spaceFolder, clean2)
	}

	clean3 := cleanUserPath(strings.ReplaceAll(spaceFolder, " ", "%20"))
	if clean3 != spaceFolder {
		t.Errorf("expected %q, got %q", spaceFolder, clean3)
	}

	// 2. GET /api/files/list with space folder and percent-encoded folder
	reqPaths := []string{
		spaceFolder,
		url.QueryEscape(spaceFolder),
		"file://" + spaceFolder,
		"file://" + strings.ReplaceAll(spaceFolder, " ", "%20"),
	}

	for _, p := range reqPaths {
		resp, err := http.Get(baseURL + "/api/files/list?path=" + url.QueryEscape(p))
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/files/list failed for path %s: err=%v, code=%d", p, err, resp.StatusCode)
		}
		var listRes struct {
			Success bool       `json:"success"`
			Path    string     `json:"path"`
			Files   []FileItem `json:"files"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&listRes)
		resp.Body.Close()

		if !listRes.Success || len(listRes.Files) == 0 {
			t.Errorf("failed to list files for path %s: success=%v, files=%d", p, listRes.Success, len(listRes.Files))
		}
	}

	// 3. GET /api/files/read with space file
	resp, err := http.Get(baseURL + "/api/files/read?path=" + url.QueryEscape(testFile))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/files/read failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var readRes struct {
		Success bool   `json:"success"`
		Content string `json:"content"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&readRes)
	resp.Body.Close()

	if !readRes.Success || readRes.Content != content {
		t.Errorf("read file mismatch: expected %q, got %q", content, readRes.Content)
	}

	// 4. Unicode directory and files with spaces
	unicodeFolder := filepath.Join(tempDir, "测试 目录 With Spaces")
	if err := os.MkdirAll(unicodeFolder, 0755); err != nil {
		t.Fatalf("MkdirAll unicode folder failed: %v", err)
	}
	unicodeFile := filepath.Join(unicodeFolder, "文档 file.txt")
	uContent := "Unicode and spaces test content"
	if err := os.WriteFile(unicodeFile, []byte(uContent), 0644); err != nil {
		t.Fatalf("WriteFile unicode failed: %v", err)
	}

	resp, err = http.Get(baseURL + "/api/files/read?path=" + url.QueryEscape(unicodeFile))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/files/read unicode failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var uReadRes struct {
		Success bool   `json:"success"`
		Content string `json:"content"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&uReadRes)
	resp.Body.Close()
	if !uReadRes.Success || uReadRes.Content != uContent {
		t.Errorf("unicode read mismatch: expected %q, got %q", uContent, uReadRes.Content)
	}

	// 5. File literally named with % on disk
	pctFile := filepath.Join(spaceFolder, "literal%20file.txt")
	pctContent := "Literal percent test content"
	if err := os.WriteFile(pctFile, []byte(pctContent), 0644); err != nil {
		t.Fatalf("WriteFile pct failed: %v", err)
	}
	cleanPct := cleanUserPath(pctFile)
	if cleanPct != pctFile {
		t.Errorf("expected cleanUserPath to preserve literal percent filename %q, got %q", pctFile, cleanPct)
	}
}

func TestFilesRecursiveCopyMoveAndBatch(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	tempDir := t.TempDir()

	// Setup source structure with spaces:
	// /src folder/
	//   file1.txt
	//   sub dir/
	//     nested.txt
	srcFolder := filepath.Join(tempDir, "src folder")
	subDir := filepath.Join(srcFolder, "sub dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create source structure: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcFolder, "file1.txt"), []byte("file1 content"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested content"), 0644); err != nil {
		t.Fatalf("failed to write nested: %v", err)
	}

	// 1. Recursive Directory Copy to target folder
	dstFolder := filepath.Join(tempDir, "dest folder")
	_ = os.MkdirAll(dstFolder, 0755)

	copyBody, _ := json.Marshal(map[string]string{
		"src": srcFolder,
		"dst": dstFolder,
	})
	resp, err := http.Post(baseURL+"/api/files/copy", "application/json", strings.NewReader(string(copyBody)))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/copy failed: %v", err)
	}
	var copyRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&copyRes)
	resp.Body.Close()

	if copyRes["success"] != true {
		t.Fatalf("copy failed: %v", copyRes["error"])
	}

	// Verify recursive copy
	copiedSubFile := filepath.Join(dstFolder, "src folder", "sub dir", "nested.txt")
	if data, err := os.ReadFile(copiedSubFile); err != nil || string(data) != "nested content" {
		t.Fatalf("recursive copy nested file failed: %v, content=%s", err, string(data))
	}

	// 2. Duplicate in-place (same folder copy should generate unique "(copy)")
	fileToDup := filepath.Join(srcFolder, "file1.txt")
	dupBody, _ := json.Marshal(map[string]string{
		"src": fileToDup,
		"dst": fileToDup,
	})
	resp, err = http.Post(baseURL+"/api/files/copy", "application/json", strings.NewReader(string(dupBody)))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST in-place copy failed: %v", err)
	}
	var dupRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&dupRes)
	resp.Body.Close()

	if dupRes["success"] != true {
		t.Fatalf("in-place copy failed: %v", dupRes["error"])
	}
	expectedDupFile := filepath.Join(srcFolder, "file1 (copy).txt")
	if _, err := os.Stat(expectedDupFile); err != nil {
		t.Fatalf("expected duplicate file %s to exist: %v", expectedDupFile, err)
	}

	// 3. Batch Copy
	batchCopyBody, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]string{
			{"src": filepath.Join(srcFolder, "file1.txt"), "dst": filepath.Join(tempDir, "batch_file1.txt")},
			{"src": subDir, "dst": filepath.Join(tempDir, "batch_subdir")},
		},
	})
	resp, err = http.Post(baseURL+"/api/files/copy", "application/json", strings.NewReader(string(batchCopyBody)))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST batch copy failed: %v", err)
	}
	var batchCopyRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&batchCopyRes)
	resp.Body.Close()

	if batchCopyRes["success"] != true {
		t.Fatalf("batch copy failed: %v", batchCopyRes["error"])
	}
	if _, err := os.Stat(filepath.Join(tempDir, "batch_subdir", "nested.txt")); err != nil {
		t.Fatalf("batch copy directory failed: %v", err)
	}

	// 4. Recursive Move
	moveTo := filepath.Join(tempDir, "moved folder")
	moveBody, _ := json.Marshal(map[string]string{
		"src": filepath.Join(tempDir, "batch_subdir"),
		"dst": moveTo,
	})
	resp, err = http.Post(baseURL+"/api/files/move", "application/json", strings.NewReader(string(moveBody)))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/move failed: %v", err)
	}
	var moveRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&moveRes)
	resp.Body.Close()

	if moveRes["success"] != true {
		t.Fatalf("move failed: %v", moveRes["error"])
	}
	if _, err := os.Stat(filepath.Join(moveTo, "nested.txt")); err != nil {
		t.Fatalf("moved file does not exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "batch_subdir")); !os.IsNotExist(err) {
		t.Fatalf("old moved dir should no longer exist")
	}

	// 5. Batch Delete
	batchDelBody, _ := json.Marshal(map[string]interface{}{
		"paths": []string{
			filepath.Join(tempDir, "batch_file1.txt"),
			moveTo,
		},
	})
	resp, err = http.Post(baseURL+"/api/files/delete", "application/json", strings.NewReader(string(batchDelBody)))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST batch delete failed: %v", err)
	}
	var delRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&delRes)
	resp.Body.Close()

	if delRes["success"] != true {
		t.Fatalf("batch delete failed: %v", delRes["error"])
	}
	if _, err := os.Stat(filepath.Join(tempDir, "batch_file1.txt")); !os.IsNotExist(err) {
		t.Fatalf("batch deleted file should be gone")
	}
	if _, err := os.Stat(moveTo); !os.IsNotExist(err) {
		t.Fatalf("batch deleted directory should be gone")
	}
}


