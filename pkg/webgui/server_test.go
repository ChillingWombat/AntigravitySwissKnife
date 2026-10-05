package webgui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
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
	srv := NewServer("127.0.0.1:0", "")
	tempStore, err := gui.NewStore(t.TempDir())
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

	if cfg.ConversationTabsMode != "fixed" {
		t.Errorf("expected default tabs mode 'fixed', got %q", cfg.ConversationTabsMode)
	}
	if cfg.ConversationTabsFixedLimit != 6 {
		t.Errorf("expected default tabs limit 6, got %d", cfg.ConversationTabsFixedLimit)
	}
	if cfg.ConversationTabsAgeThreshold != "1d" {
		t.Errorf("expected default age threshold '1d', got %q", cfg.ConversationTabsAgeThreshold)
	}
	if cfg.ConversationTabsMin != 2 {
		t.Errorf("expected default min tabs 2, got %d", cfg.ConversationTabsMin)
	}
	if cfg.ConversationTabsMax != 6 {
		t.Errorf("expected default max tabs 6, got %d", cfg.ConversationTabsMax)
	}
	if cfg.AutoArchiveHorizon != "30d" {
		t.Errorf("expected default auto archive horizon '30d', got %q", cfg.AutoArchiveHorizon)
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



