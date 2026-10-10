package webgui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/revival"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/system"
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

func TestWebGUIQuotaAndAccountsEndpoints(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/quota/fleet - must return 200 OK immediately (<500ms), not block
	start := time.Now()
	resp, err := http.Get(baseURL + "/api/quota/fleet")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GET /api/quota/fleet failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if elapsed > 5*time.Second {
		t.Errorf("GET /api/quota/fleet took %v, expected non-blocking (<5s)", elapsed)
	}
	var fleet quota.FleetQuotaSummary
	if err := json.NewDecoder(resp.Body).Decode(&fleet); err != nil {
		t.Fatalf("failed to decode fleet quota summary: %v", err)
	}

	// 2. POST /api/quota/refresh - must trigger async refresh and return 200
	respRefresh, err := http.Post(baseURL+"/api/quota/refresh", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/quota/refresh failed: %v", err)
	}
	defer respRefresh.Body.Close()
	if respRefresh.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", respRefresh.StatusCode)
	}

	// 3. GET /api/accounts - must return 200 OK
	respAccs, err := http.Get(baseURL + "/api/accounts")
	if err != nil {
		t.Fatalf("GET /api/accounts failed: %v", err)
	}
	defer respAccs.Body.Close()
	if respAccs.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", respAccs.StatusCode)
	}

	// 4. GET /api/quota and GET /api/quota?email=test - must return 200 OK immediately
	respQuota, err := http.Get(baseURL + "/api/quota?email=test@example.com")
	if err != nil {
		t.Fatalf("GET /api/quota?email=test failed: %v", err)
	}
	defer respQuota.Body.Close()
	if respQuota.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", respQuota.StatusCode)
	}
	var qSummary quota.QuotaSummary
	if err := json.NewDecoder(respQuota.Body).Decode(&qSummary); err != nil {
		t.Fatalf("failed to decode quota summary: %v", err)
	}
	if qSummary.AccountEmail != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", qSummary.AccountEmail)
	}
}

func TestWebGUIPrunedConversationsEndpoint(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	resp, err := http.Get(baseURL + "/api/gui/conversations/pruned")
	if err != nil {
		t.Fatalf("GET /api/gui/conversations/pruned failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var list []string
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode pruned conversations list: %v", err)
	}

	// Verify method not allowed on POST
	postResp, err := http.Post(baseURL+"/api/gui/conversations/pruned", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/gui/conversations/pruned failed: %v", err)
	}
	postResp.Body.Close()
	if postResp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed on POST, got %d", postResp.StatusCode)
	}
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

func TestWebGUIAppReleaseEndpoints(t *testing.T) {
	// Mock GitHub Releases server
	mockReleases := []system.GitHubRelease{
		{
			TagName:     "v2.1.0",
			Name:        "Antigravity Swiss Knife v2.1.0",
			Body:        "Release notes for 2.1.0",
			Draft:       false,
			PublishedAt: time.Now(),
			HTMLURL:     "https://github.com/ChillingWombat/AntigravitySwissKnife/releases/tag/v2.1.0",
			Assets: []system.GitHubReleaseAsset{
				{
					ID:                 101,
					Name:               "Antigravity-Swiss-Knife-2.1.0-x86_64.AppImage",
					Size:               54321,
					BrowserDownloadURL: "http://example.com/download.AppImage",
				},
			},
		},
	}

	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockReleases)
	}))
	defer mockGH.Close()

	srv := NewServer("127.0.0.1:0", "")
	mockRelMgr := system.NewAppReleaseManager(mockGH.URL)
	srv.SetAppReleaseManager(mockRelMgr)

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/system/app_release
	resp, err := http.Get(baseURL + "/api/system/app_release")
	if err != nil {
		t.Fatalf("GET /api/system/app_release failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var cachedInfo system.AppReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&cachedInfo); err != nil {
		t.Fatalf("failed to decode cached info: %v", err)
	}
	resp.Body.Close()

	if cachedInfo.CurrentVersion != "N/A" {
		t.Errorf("expected current version N/A, got %s", cachedInfo.CurrentVersion)
	}
	if cachedInfo.LatestVersion != "N/A" {
		t.Errorf("expected latest version N/A, got %s", cachedInfo.LatestVersion)
	}

	// 2. POST /api/system/check_app_release
	resp, err = http.Post(baseURL+"/api/system/check_app_release", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/system/check_app_release failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var checkedInfo system.AppReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&checkedInfo); err != nil {
		t.Fatalf("failed to decode checked info: %v", err)
	}
	resp.Body.Close()

	if !checkedInfo.HasUpdate {
		t.Errorf("expected HasUpdate true when remote has 2.1.0")
	}
	if checkedInfo.LatestVersion != "2.1.0" {
		t.Errorf("expected latest version 2.1.0, got %s", checkedInfo.LatestVersion)
	}

	// 3. POST /api/system/app_release/settings
	settingsBody := `{"auto_check": true, "auto_upgrade": true}`
	resp, err = http.Post(baseURL+"/api/system/app_release/settings", "application/json", strings.NewReader(settingsBody))
	if err != nil {
		t.Fatalf("POST /api/system/app_release/settings failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var settingsRes map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&settingsRes); err != nil {
		t.Fatalf("failed to decode settings result: %v", err)
	}
	resp.Body.Close()

	if settingsRes["auto_upgrade"] != true {
		t.Errorf("expected auto_upgrade true in settings response")
	}
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

func TestWebGUIMemos_ProjectScopedStorage(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	globalTmp := t.TempDir()
	globalFile := filepath.Join(globalTmp, "global_memos.json")
	srv.SetGlobalMemosPath(globalFile)

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	wsDir := t.TempDir()

	// 1. Save memo targeted to project workspace
	payload := map[string]interface{}{
		"title":   "Project Memo 1",
		"content": "Work on milestone M10",
		"type":    "text",
		"tags":    []string{"project", "m10"},
	}
	pBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/memos/save?storage=project", bytes.NewReader(pBytes))
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-Path", wsDir)

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/memos/save failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var saveRes struct {
		Success  bool `json:"success"`
		Fallback bool `json:"fallback"`
		Memo     struct {
			ID            string `json:"id"`
			Title         string `json:"title"`
			WorkspacePath string `json:"workspace_path"`
			Project       string `json:"project"`
		} `json:"memo"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&saveRes)
	resp.Body.Close()

	if !saveRes.Success || saveRes.Memo.ID == "" {
		t.Fatalf("expected successful memo save with valid ID")
	}
	if saveRes.Fallback {
		t.Fatalf("did not expect fallback for valid project directory")
	}
	if saveRes.Memo.WorkspacePath != wsDir {
		t.Errorf("expected workspace_path %q, got %q", wsDir, saveRes.Memo.WorkspacePath)
	}

	// Verify physical file was written to <wsDir>/.antigravity/memos.json
	projFile := filepath.Join(wsDir, ".antigravity", "memos.json")
	data, err := os.ReadFile(projFile)
	if err != nil {
		t.Fatalf("expected project memos file at %q: %v", projFile, err)
	}
	if !strings.Contains(string(data), "Project Memo 1") {
		t.Fatalf("project memos file does not contain expected title: %s", string(data))
	}

	// Verify global memos file was not written to
	if _, err := os.Stat(globalFile); err == nil {
		gData, _ := os.ReadFile(globalFile)
		if strings.Contains(string(gData), "Project Memo 1") {
			t.Fatalf("global memos file should not contain project-scoped memo")
		}
	}

	// 2. Query memos for current workspace
	reqList, _ := http.NewRequest(http.MethodGet, baseURL+"/api/memos?scope=current", nil)
	reqList.Header.Set("X-Workspace-Path", wsDir)
	respList, err := http.DefaultClient.Do(reqList)
	if err != nil || respList.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/memos?scope=current failed: err=%v, code=%d", err, respList.StatusCode)
	}
	var listRes struct {
		Success bool `json:"success"`
		Memos   []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"memos"`
	}
	_ = json.NewDecoder(respList.Body).Decode(&listRes)
	respList.Body.Close()

	if !listRes.Success || len(listRes.Memos) != 1 || listRes.Memos[0].Title != "Project Memo 1" {
		t.Fatalf("expected 1 project memo matching 'Project Memo 1', got %+v", listRes)
	}
}

func TestWebGUIMemos_GracefulFallback(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	globalTmp := t.TempDir()
	globalFile := filepath.Join(globalTmp, "fallback_global_memos.json")
	srv.SetGlobalMemosPath(globalFile)

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// Case 1: Empty workspace path with storage=project
	payload1 := map[string]interface{}{
		"title":   "Fallback Empty Workspace",
		"content": "No workspace passed",
		"type":    "text",
	}
	pBytes1, _ := json.Marshal(payload1)
	resp1, err := http.Post(baseURL+"/api/memos/save?storage=project", "application/json", bytes.NewReader(pBytes1))
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/memos/save empty ws failed: err=%v, code=%d", err, resp1.StatusCode)
	}
	var res1 struct {
		Success  bool   `json:"success"`
		Fallback bool   `json:"fallback"`
		Storage  string `json:"storage_location_effective"`
		Memo     struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"memo"`
	}
	_ = json.NewDecoder(resp1.Body).Decode(&res1)
	resp1.Body.Close()

	if !res1.Success || !res1.Fallback || res1.Storage != "global" {
		t.Fatalf("expected fallback: true and storage_location_effective: global, got %+v", res1)
	}

	// Verify written to global file
	gData, err := os.ReadFile(globalFile)
	if err != nil || !strings.Contains(string(gData), "Fallback Empty Workspace") {
		t.Fatalf("expected memo in global fallback file: %v", err)
	}

	// Case 2: Nonexistent directory path with storage=project
	payload2 := map[string]interface{}{
		"title":   "Fallback Nonexistent Workspace",
		"content": "Nonexistent path passed",
		"type":    "text",
	}
	pBytes2, _ := json.Marshal(payload2)
	req2, _ := http.NewRequest(http.MethodPost, baseURL+"/api/memos/save?storage=project&workspace_path=/nonexistent/path/999/xyz", bytes.NewReader(pBytes2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/memos/save nonexistent ws failed: err=%v, code=%d", err, resp2.StatusCode)
	}
	var res2 struct {
		Success  bool   `json:"success"`
		Fallback bool   `json:"fallback"`
		Storage  string `json:"storage_location_effective"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&res2)
	resp2.Body.Close()

	if !res2.Success || !res2.Fallback || res2.Storage != "global" {
		t.Fatalf("expected graceful fallback: true for nonexistent workspace, got %+v", res2)
	}
}

func TestWebGUIMemos_ViewScopeFiltering(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	globalTmp := t.TempDir()
	srv.SetGlobalMemosPath(filepath.Join(globalTmp, "global_memos.json"))

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	ws1 := t.TempDir()
	ws2 := t.TempDir()

	// 1. Save Memo A in Workspace 1
	pA, _ := json.Marshal(map[string]interface{}{
		"title":   "Memo in WS1",
		"content": "Workspace 1 specific notes",
		"type":    "text",
	})
	reqA, _ := http.NewRequest(http.MethodPost, baseURL+"/api/memos/save?storage=project&workspace_path="+ws1, bytes.NewReader(pA))
	reqA.Header.Set("Content-Type", "application/json")
	respA, err := http.DefaultClient.Do(reqA)
	if err != nil || respA.StatusCode != http.StatusOK {
		t.Fatalf("save memo A failed: %v", err)
	}
	respA.Body.Close()

	// 2. Save Memo B in Workspace 2
	pB, _ := json.Marshal(map[string]interface{}{
		"title":   "Memo in WS2",
		"content": "Workspace 2 specific notes",
		"type":    "text",
	})
	reqB, _ := http.NewRequest(http.MethodPost, baseURL+"/api/memos/save?storage=project&workspace_path="+ws2, bytes.NewReader(pB))
	reqB.Header.Set("Content-Type", "application/json")
	respB, err := http.DefaultClient.Do(reqB)
	if err != nil || respB.StatusCode != http.StatusOK {
		t.Fatalf("save memo B failed: %v", err)
	}
	respB.Body.Close()

	// 3. Test scope=current on Workspace 1: should return only Memo in WS1
	resp1, err := http.Get(baseURL + "/api/memos?scope=current&workspace_path=" + ws1)
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/memos scope=current ws1 failed: %v", err)
	}
	var res1 struct {
		Success bool `json:"success"`
		Memos   []struct {
			Title string `json:"title"`
		} `json:"memos"`
	}
	_ = json.NewDecoder(resp1.Body).Decode(&res1)
	resp1.Body.Close()

	if len(res1.Memos) != 1 || res1.Memos[0].Title != "Memo in WS1" {
		t.Fatalf("expected only 'Memo in WS1' for ws1, got %+v", res1)
	}

	// 4. Test scope=current on Workspace 2: should return only Memo in WS2
	resp2, err := http.Get(baseURL + "/api/memos?scope=current&workspace_path=" + ws2)
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/memos scope=current ws2 failed: %v", err)
	}
	var res2 struct {
		Success bool `json:"success"`
		Memos   []struct {
			Title string `json:"title"`
		} `json:"memos"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&res2)
	resp2.Body.Close()

	if len(res2.Memos) != 1 || res2.Memos[0].Title != "Memo in WS2" {
		t.Fatalf("expected only 'Memo in WS2' for ws2, got %+v", res2)
	}

	// 5. Test scope=all: should return aggregated memos (both WS1 and WS2)
	respAll, err := http.Get(baseURL + "/api/memos?scope=all")
	if err != nil || respAll.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/memos scope=all failed: %v", err)
	}
	var resAll struct {
		Success bool `json:"success"`
		Memos   []struct {
			Title string `json:"title"`
		} `json:"memos"`
	}
	_ = json.NewDecoder(respAll.Body).Decode(&resAll)
	respAll.Body.Close()

	titles := make(map[string]bool)
	for _, m := range resAll.Memos {
		titles[m.Title] = true
	}
	if !titles["Memo in WS1"] || !titles["Memo in WS2"] {
		t.Fatalf("expected aggregated list to contain both Memo in WS1 and Memo in WS2, got %+v", resAll)
	}
}

func TestWebGUIMemos_DeleteProjectMemo(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	globalTmp := t.TempDir()
	srv.SetGlobalMemosPath(filepath.Join(globalTmp, "global_memos.json"))

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	ws := t.TempDir()

	// 1. Save memo in workspace
	p, _ := json.Marshal(map[string]interface{}{
		"id":      "memo-del-target-99",
		"title":   "Memo to Delete",
		"content": "Will be removed soon",
		"type":    "text",
	})
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/memos/save?storage=project&workspace_path="+ws, bytes.NewReader(p))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("save memo failed: %v", err)
	}
	resp.Body.Close()

	projFile := filepath.Join(ws, ".antigravity", "memos.json")
	content, _ := os.ReadFile(projFile)
	if !strings.Contains(string(content), "memo-del-target-99") {
		t.Fatalf("memo was not saved to project file: %s", string(content))
	}

	// 2. Delete memo
	reqDel, _ := http.NewRequest(http.MethodPost, baseURL+"/api/memos/delete?id=memo-del-target-99&workspace_path="+ws, nil)
	respDel, err := http.DefaultClient.Do(reqDel)
	if err != nil || respDel.StatusCode != http.StatusOK {
		t.Fatalf("delete memo failed: %v", err)
	}
	var delRes struct {
		Success bool   `json:"success"`
		Deleted string `json:"deleted"`
	}
	_ = json.NewDecoder(respDel.Body).Decode(&delRes)
	respDel.Body.Close()

	if !delRes.Success || delRes.Deleted != "memo-del-target-99" {
		t.Fatalf("unexpected delete result: %+v", delRes)
	}

	// 3. Confirm deletion in project file and GET endpoint
	contentAfter, _ := os.ReadFile(projFile)
	if strings.Contains(string(contentAfter), "memo-del-target-99") {
		t.Fatalf("memo was still present in project file after deletion: %s", string(contentAfter))
	}

	respGet, _ := http.Get(baseURL + "/api/memos?scope=current&workspace_path=" + ws)
	var listRes struct {
		Success bool `json:"success"`
		Memos   []struct {
			ID string `json:"id"`
		} `json:"memos"`
	}
	_ = json.NewDecoder(respGet.Body).Decode(&listRes)
	respGet.Body.Close()

	if len(listRes.Memos) != 0 {
		t.Fatalf("expected 0 memos after deletion, got %d", len(listRes.Memos))
	}
}

func TestWebGUIMemos_TranscriptAndVoiceFields(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	globalTmp := t.TempDir()
	srv.SetGlobalMemosPath(filepath.Join(globalTmp, "global_memos.json"))

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	ws := t.TempDir()

	voicePayload := map[string]interface{}{
		"title":      "Voice Note Review",
		"type":       "audio",
		"content":    "Voice memo transcript content",
		"transcript": "Web Speech API transcribed this voice note flawlessly",
		"audio_data": "data:audio/webm;codecs=opus;base64,GkXfo59ChoEBQveBAULygQ8USA0BAAAAAAAA...",
		"duration":   "01:15",
		"tags":       []string{"voice", "review"},
	}
	pBytes, _ := json.Marshal(voicePayload)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/memos/save?storage=project&workspace_path="+ws, bytes.NewReader(pBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("save voice memo failed: %v", err)
	}
	var saveRes struct {
		Success bool `json:"success"`
		Memo    struct {
			ID            string   `json:"id"`
			Title         string   `json:"title"`
			Transcript    string   `json:"transcript"`
			AudioData     string   `json:"audio_data"`
			Duration      string   `json:"duration"`
			WorkspacePath string   `json:"workspace_path"`
			Project       string   `json:"project"`
			Tags          []string `json:"tags"`
		} `json:"memo"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&saveRes)
	resp.Body.Close()

	if !saveRes.Success {
		t.Fatalf("save voice memo failed")
	}
	if saveRes.Memo.Transcript != "Web Speech API transcribed this voice note flawlessly" {
		t.Errorf("transcript mismatch: got %q", saveRes.Memo.Transcript)
	}
	if !strings.HasPrefix(saveRes.Memo.AudioData, "data:audio/webm") {
		t.Errorf("audio_data mismatch: got %q", saveRes.Memo.AudioData)
	}
	if saveRes.Memo.Duration != "01:15" {
		t.Errorf("duration mismatch: got %q", saveRes.Memo.Duration)
	}
	if saveRes.Memo.WorkspacePath != ws {
		t.Errorf("workspace_path mismatch: got %q", saveRes.Memo.WorkspacePath)
	}
	if saveRes.Memo.Project != filepath.Base(ws) {
		t.Errorf("project mismatch: got %q", saveRes.Memo.Project)
	}

	// Verify persistence in GET /api/memos
	respGet, _ := http.Get(baseURL + "/api/memos?scope=current&workspace_path=" + ws)
	var listRes struct {
		Success bool `json:"success"`
		Memos   []struct {
			ID         string `json:"id"`
			Transcript string `json:"transcript"`
			AudioData  string `json:"audio_data"`
			Duration   string `json:"duration"`
		} `json:"memos"`
	}
	_ = json.NewDecoder(respGet.Body).Decode(&listRes)
	respGet.Body.Close()

	if len(listRes.Memos) != 1 {
		t.Fatalf("expected 1 voice memo in list, got %d", len(listRes.Memos))
	}
	if listRes.Memos[0].Transcript != "Web Speech API transcribed this voice note flawlessly" {
		t.Errorf("retrieved transcript mismatch: %q", listRes.Memos[0].Transcript)
	}
	if listRes.Memos[0].Duration != "01:15" {
		t.Errorf("retrieved duration mismatch: %q", listRes.Memos[0].Duration)
	}
}

func TestWebGUIMemos_ConfigEndpoint(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", cfgDir)

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/memos/config -> default settings
	resp, err := http.Get(baseURL + "/api/memos/config")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/memos/config failed: %v", err)
	}
	var getRes struct {
		Success         bool   `json:"success"`
		StorageLocation string `json:"storage_location"`
		ViewScope       string `json:"view_scope"`
		SearchScope     string `json:"search_scope"`
		Config          struct {
			StorageLocation string `json:"storage_location"`
			ViewScope       string `json:"view_scope"`
			SearchScope     string `json:"search_scope"`
		} `json:"config"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&getRes)
	resp.Body.Close()

	if !getRes.Success {
		t.Fatalf("expected config success: true")
	}
	if getRes.StorageLocation != "global" || getRes.ViewScope != "all" || getRes.SearchScope != "text" {
		t.Errorf("unexpected default config: %+v", getRes)
	}

	// 2. POST /api/memos/config -> update settings
	updateBody, _ := json.Marshal(map[string]interface{}{
		"storage_location": "project",
		"view_scope":       "current",
		"search_scope":     "all",
	})
	respPost, err := http.Post(baseURL+"/api/memos/config", "application/json", bytes.NewReader(updateBody))
	if err != nil || respPost.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/memos/config failed: %v", err)
	}
	var postRes struct {
		Success         bool   `json:"success"`
		StorageLocation string `json:"storage_location"`
		ViewScope       string `json:"view_scope"`
		SearchScope     string `json:"search_scope"`
	}
	_ = json.NewDecoder(respPost.Body).Decode(&postRes)
	respPost.Body.Close()

	if !postRes.Success || postRes.StorageLocation != "project" || postRes.ViewScope != "current" || postRes.SearchScope != "all" {
		t.Fatalf("unexpected updated config: %+v", postRes)
	}

	// 3. GET /api/memos/config again -> ensure persistence
	respGet2, err := http.Get(baseURL + "/api/memos/config")
	if err != nil || respGet2.StatusCode != http.StatusOK {
		t.Fatalf("second GET /api/memos/config failed: %v", err)
	}
	var getRes2 struct {
		Success         bool   `json:"success"`
		StorageLocation string `json:"storage_location"`
		ViewScope       string `json:"view_scope"`
		SearchScope     string `json:"search_scope"`
	}
	_ = json.NewDecoder(respGet2.Body).Decode(&getRes2)
	respGet2.Body.Close()

	if getRes2.StorageLocation != "project" || getRes2.ViewScope != "current" || getRes2.SearchScope != "all" {
		t.Fatalf("persisted config did not match updated values: %+v", getRes2)
	}
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
		Success bool   `json:"success"`
		Source  string `json:"source"`
		Count   int    `json:"count"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&scanRes)
	resp.Body.Close()
	if !scanRes.Success || scanRes.Source != "claude-code" {
		t.Errorf("expected successful scan for claude-code, got %+v", scanRes)
	}

	// 2. POST /api/utilities/import with empty list
	payload := map[string]interface{}{
		"candidate_ids": []string{},
		"source":        "claude-code",
		"mode":          "auto",
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
	if cat.DefaultGemini != "gemini-3.8-flash" && cat.DefaultGemini != "gemini-3.8-flash-high" {
		t.Errorf("expected default gemini 'gemini-3.8-flash' or 'gemini-3.8-flash-high', got '%s'", cat.DefaultGemini)
	}
	if cat.DefaultNonGemini != "claude-opus-4-6-thinking" {
		t.Errorf("expected default non-gemini 'claude-opus-4-6-thinking', got '%s'", cat.DefaultNonGemini)
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
		"default_custom_model":           "custom-first-model",
		"default_gemini_model":           "gemini-3.8-pro",
		"default_non_gemini_model":       "claude-3-7-sonnet",
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

	// 2. POST /api/settings/storage to switch to app_portable (fails with 400 Bad Request)
	switchBody, _ := json.Marshal(map[string]interface{}{
		"storage_mode": "app_portable",
		"migrate_data": true,
	})
	resp, err = http.Post(baseURL+"/api/settings/storage", "application/json", bytes.NewReader(switchBody))
	if err != nil {
		t.Fatalf("POST /api/settings/storage failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for app_portable, got %d", resp.StatusCode)
	}
	var switchResp map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&switchResp)
	resp.Body.Close()
	if switchResp["success"] != false {
		t.Errorf("expected switch success false for app_portable, got %v", switchResp)
	}

	// 2b. POST /api/settings/storage to set system_default (succeeds with 200 OK)
	sysBody, _ := json.Marshal(map[string]interface{}{
		"storage_mode": "system_default",
		"migrate_data": false,
	})
	resp, err = http.Post(baseURL+"/api/settings/storage", "application/json", bytes.NewReader(sysBody))
	if err != nil {
		t.Fatalf("POST /api/settings/storage system_default failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for system_default, got %d", resp.StatusCode)
	}
	var sysResp map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&sysResp)
	resp.Body.Close()
	if sysResp["success"] != true {
		t.Errorf("expected switch success true for system_default, got %v", sysResp)
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

func TestCleanUserPath_HomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("UserHomeDir not available")
	}
	clean := cleanUserPath("~")
	if clean != home {
		t.Errorf("expected cleanUserPath(~) = %q, got %q", home, clean)
	}

	cleanSub := cleanUserPath("~/subfolder")
	expectedSub := filepath.Join(home, "subfolder")
	if cleanSub != expectedSub {
		t.Errorf("expected cleanUserPath(~/subfolder) = %q, got %q", expectedSub, cleanSub)
	}

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	resp, err := http.Get(baseURL + "/api/files/list?path=~")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/files/list?path=~ failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var res struct {
		Success bool       `json:"success"`
		Path    string     `json:"path"`
		Files   []FileItem `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode JSON failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success true for path=~")
	}
	if res.Path != home {
		t.Errorf("expected returned path %q, got %q", home, res.Path)
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

func TestWebGUIFilesOpenIDEEndpoint(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", cfgDir)

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	tempDir := t.TempDir()

	// 1. GET /api/files/open_ide returns default or current preferred IDE
	resp, err := http.Get(baseURL + "/api/files/open_ide")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/files/open_ide failed: %v", err)
	}
	var getIDE map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&getIDE)
	resp.Body.Close()
	if getIDE["success"] != true {
		t.Errorf("expected success: true, got %v", getIDE)
	}
	if getIDE["preferred_ide"] != "code" {
		t.Errorf("expected default preferred_ide 'code', got %v", getIDE["preferred_ide"])
	}

	// 2. GET /api/files/ide/config and POST /api/files/ide/config to update preferred IDE
	configBody, _ := json.Marshal(map[string]string{"preferred_ide": "cursor"})
	resp, err = http.Post(baseURL+"/api/files/ide/config", "application/json", bytes.NewReader(configBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/ide/config failed: %v", err)
	}
	var postConfigRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&postConfigRes)
	resp.Body.Close()
	if postConfigRes["success"] != true || postConfigRes["preferred_ide"] != "cursor" {
		t.Errorf("expected preferred_ide cursor, got %v", postConfigRes)
	}

	// Verify persistence via GET
	resp, err = http.Get(baseURL + "/api/files/ide/config")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/files/ide/config failed: %v", err)
	}
	var checkConfigRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&checkConfigRes)
	resp.Body.Close()
	if checkConfigRes["preferred_ide"] != "cursor" {
		t.Errorf("expected preferred_ide cursor, got %v", checkConfigRes)
	}

	// 3. POST /api/files/open_ide with folder workspace
	ideReqBody, _ := json.Marshal(map[string]string{
		"path": tempDir,
		"ide":  "code",
	})
	resp, err = http.Post(baseURL+"/api/files/open_ide", "application/json", bytes.NewReader(ideReqBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/open_ide failed: %v", err)
	}
	var openRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&openRes)
	resp.Body.Close()

	if openRes["dir"] != tempDir {
		t.Errorf("expected dir %s, got %v", tempDir, openRes["dir"])
	}
	if openRes["ide"] != "code" {
		t.Errorf("expected ide code, got %v", openRes["ide"])
	}

	// 4. POST /api/files/open_ide without specifying ide uses persisted preferred_ide (cursor)
	reqNoIDE, _ := json.Marshal(map[string]string{
		"path": tempDir,
	})
	resp, err = http.Post(baseURL+"/api/files/open_ide", "application/json", bytes.NewReader(reqNoIDE))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/open_ide (empty ide) failed: %v", err)
	}
	var openFallbackRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&openFallbackRes)
	resp.Body.Close()
	if openFallbackRes["ide"] != "cursor" {
		t.Errorf("expected fallback to preferred_ide 'cursor', got %v", openFallbackRes["ide"])
	}

	// 5. POST /api/files/ide alias with a file path (should resolve to parent directory)
	subFile := filepath.Join(tempDir, "sample.txt")
	_ = os.WriteFile(subFile, []byte("test"), 0644)
	ideFileReq, _ := json.Marshal(map[string]string{
		"path": subFile,
		"ide":  "zed",
	})
	resp, err = http.Post(baseURL+"/api/files/ide", "application/json", bytes.NewReader(ideFileReq))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/files/ide alias failed: %v", err)
	}
	var aliasRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&aliasRes)
	resp.Body.Close()

	if aliasRes["dir"] != tempDir {
		t.Errorf("expected dir to resolve to parent %s, got %v", tempDir, aliasRes["dir"])
	}
	if aliasRes["ide"] != "zed" {
		t.Errorf("expected ide zed, got %v", aliasRes["ide"])
	}

	// 6. Verify unsupported method returns 405 MethodNotAllowed
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/api/files/open_ide", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/files/open_ide error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", resp.StatusCode)
	}
}

func TestDesktopRelaunchEndpoint(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")
	dummySock := filepath.Join(t.TempDir(), "test.sock")
	srv := NewServer("127.0.0.1:0", dummySock)
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. POST /api/desktop/relaunch
	resp, err := http.Post(baseURL+"/api/desktop/relaunch", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST /api/desktop/relaunch failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	var res map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if res["success"] != true {
		t.Errorf("expected success: true, got %+v", res)
	}

	// 2. Method not allowed for GET
	getResp, err := http.Get(baseURL + "/api/desktop/relaunch")
	if err != nil {
		t.Fatalf("GET /api/desktop/relaunch failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", getResp.StatusCode)
	}
}

func TestTokenMonitorRealTelemetryPricingAndDeletion(t *testing.T) {
	tmpDir := t.TempDir()
	agDir := filepath.Join(tmpDir, "antigravity")
	brainDir := filepath.Join(agDir, "brain")
	convDir := filepath.Join(agDir, "conversations")
	_ = os.MkdirAll(convDir, 0755)

	// Conversation 1: Gemini 3.8 Flash (High) -> MODEL_PLACEHOLDER_M318
	conv1Logs := filepath.Join(brainDir, "conv-1", ".system_generated", "logs")
	_ = os.MkdirAll(conv1Logs, 0755)
	_ = os.WriteFile(filepath.Join(conv1Logs, "transcript.jsonl"), []byte(
		`{"type":"PLANNER_RESPONSE","step_index":1,"created_at":"2026-04-06T03:07:38Z","thinking_Duration":"2.0s","input_tokens":10000,"output_tokens":2000,"cache_read_tokens":5000}`+"\n",
	), 0644)
	_ = os.WriteFile(filepath.Join(convDir, "conv-1.db"), []byte("header\x00model_enum\x12\x16MODEL_PLACEHOLDER_M318\x00"), 0644)

	// Conversation 2: Gemini 3.8 Flash (Medium) -> MODEL_PLACEHOLDER_M319 (must merge into same canonical Gemini 3.8 Flash model!)
	conv2Logs := filepath.Join(brainDir, "conv-2", ".system_generated", "logs")
	_ = os.MkdirAll(conv2Logs, 0755)
	_ = os.WriteFile(filepath.Join(conv2Logs, "transcript.jsonl"), []byte(
		`{"type":"PLANNER_RESPONSE","step_index":1,"created_at":"2026-04-06T03:09:00Z","thinking_Duration":"1.0s","input_tokens":5000,"output_tokens":1000,"cache_read_tokens":2000}`+"\n",
	), 0644)
	_ = os.WriteFile(filepath.Join(convDir, "conv-2.db"), []byte("header\x00model_enum\x12\x16MODEL_PLACEHOLDER_M319\x00"), 0644)

	// Conversation 3: Custom model (deepseek-chat)
	conv3Logs := filepath.Join(brainDir, "conv-3", ".system_generated", "logs")
	_ = os.MkdirAll(conv3Logs, 0755)
	_ = os.WriteFile(filepath.Join(conv3Logs, "transcript.jsonl"), []byte(
		`{"type":"PLANNER_RESPONSE","step_index":1,"created_at":"2026-04-06T03:12:00Z","thinking_Duration":"1.5s","input_tokens":8000,"output_tokens":3000,"cache_read_tokens":0}`+"\n",
	), 0644)
	_ = os.WriteFile(filepath.Join(convDir, "conv-3.db"), []byte("header\x00deepseek-chat\x00"), 0644)

	cmStore, err := custommodels.NewStore(filepath.Join(tmpDir, "cm_cfg"))
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	dummySock := filepath.Join(tmpDir, "test.sock")
	srv := NewServer("127.0.0.1:0", dummySock)
	srv.SetCustomModelsStore(cmStore)
	srv.SetAntigravityDataDir(agDir)
	srv.SetCatalogFetcher(func(force bool) []custommodels.CatalogModelInput {
		return []custommodels.CatalogModelInput{
			{ID: "gemini-3.8-flash-high", DisplayName: "Gemini 3.8 Flash (High)", Provider: "Google"},
			{ID: "gemini-3.8-flash-medium", DisplayName: "Gemini 3.8 Flash (Medium)", Provider: "Google"},
		}
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()
	baseURL := "http://" + srv.Addr()

	// 1. Add Custom Model with manual price & budget cap via POST /api/custom_models
	addReq := map[string]interface{}{
		"id":                       "cm-ds-1",
		"name":                     "deepseek-chat",
		"display_name":             "DeepSeek Chat V3",
		"provider_type":            "openai",
		"base_url":                 "https://api.deepseek.com/v1",
		"quota_type":               "balance",
		"quota_manual_override":    true,
		"balance_value":            "$25.00",
		"budget_cap_type":          "dollar",
		"budget_cap_value":         15.0,
		"input_price_per_m":        0.30,
		"cached_input_price_per_m": 0.08,
		"output_price_per_m":       1.20,
		"price_source":             "manual",
		"enabled":                  true,
	}
	addBytes, _ := json.Marshal(addReq)
	resp, err := http.Post(baseURL+"/api/custom_models", "application/json", bytes.NewReader(addBytes))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/custom_models failed: %v", err)
	}
	resp.Body.Close()

	// 2. GET /api/tokens/summary -> verify Gemini 3.8 Flash (High + Medium merged) and DeepSeek Chat V3
	sumResp, err := http.Get(baseURL + "/api/tokens/summary")
	if err != nil || sumResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/tokens/summary failed: %v", err)
	}
	var sumData struct {
		TotalTokens     int64                        `json:"total_tokens"`
		RequestsCount   int                          `json:"requests_count"`
		ModelBreakdowns []TokenModelBreakdown        `json:"model_breakdowns"`
		TelemetryEvents []TokenTelemetryEvent        `json:"telemetry_events"`
		PricingRecords  []custommodels.ModelPricingRecord `json:"pricing_records"`
		Models          []custommodels.ModelPricingRecord `json:"models"`
	}
	if err := json.NewDecoder(sumResp.Body).Decode(&sumData); err != nil {
		t.Fatalf("decode summary error: %v", err)
	}
	sumResp.Body.Close()

	if len(sumData.ModelBreakdowns) != 2 {
		t.Fatalf("expected 2 merged models in breakdown (Gemini 3.8 Flash + DeepSeek Chat V3), got %d: %+v", len(sumData.ModelBreakdowns), sumData.ModelBreakdowns)
	}
	if sumData.ModelBreakdowns[0].ModelID != "gemini-3.8-flash" || sumData.ModelBreakdowns[0].ModelName != "Gemini 3.8 Flash" {
		t.Errorf("expected top model to be canonical Gemini 3.8 Flash, got %+v", sumData.ModelBreakdowns[0])
	}
	if sumData.ModelBreakdowns[0].CanonicalID != "gemini-3.8-flash" {
		t.Errorf("expected CanonicalID to be populated, got %q", sumData.ModelBreakdowns[0].CanonicalID)
	}
	if sumData.ModelBreakdowns[0].Requests != 2 {
		t.Errorf("expected 2 merged requests for Gemini 3.8 Flash (High + Medium), got %d", sumData.ModelBreakdowns[0].Requests)
	}
	if len(sumData.PricingRecords) == 0 || len(sumData.Models) == 0 {
		t.Errorf("expected pricing_records and models to be populated in summary, got records=%d models=%d", len(sumData.PricingRecords), len(sumData.Models))
	}
	if len(sumData.TelemetryEvents) > 0 {
		ev := sumData.TelemetryEvents[0]
		if ev.CanonicalID == "" || ev.Classification == "" {
			t.Errorf("expected telemetry event to have CanonicalID and Classification, got %+v", ev)
		}
	}

	// Verify GET /api/tokens/pricing returns pricing_records and models
	prResp, err := http.Get(baseURL + "/api/tokens/pricing")
	if err != nil || prResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/tokens/pricing failed: %v", err)
	}
	var prPayload struct {
		Success        bool                              `json:"success"`
		PricingRecords []custommodels.ModelPricingRecord `json:"pricing_records"`
		Models         []custommodels.ModelPricingRecord `json:"models"`
	}
	if err := json.NewDecoder(prResp.Body).Decode(&prPayload); err != nil {
		t.Fatalf("decode pricing response error: %v", err)
	}
	prResp.Body.Close()
	if !prPayload.Success || len(prPayload.PricingRecords) == 0 || len(prPayload.Models) == 0 {
		t.Errorf("expected GET /api/tokens/pricing to return pricing_records and models, got %+v", prPayload)
	}

	// 3. Delete Custom Model via DELETE /api/tokens/pricing using canonical_id alias -> removes both price data and usage info!
	delReq, _ := http.NewRequest(http.MethodDelete, baseURL+"/api/tokens/pricing?canonical_id=deepseek-chat", nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil || delResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE /api/tokens/pricing failed: %v", err)
	}
	delResp.Body.Close()

	// Verify usage info for deepseek-chat is now deleted from /api/tokens/summary
	sumResp2, _ := http.Get(baseURL + "/api/tokens/summary")
	var sumData2 struct {
		ModelBreakdowns []TokenModelBreakdown `json:"model_breakdowns"`
	}
	_ = json.NewDecoder(sumResp2.Body).Decode(&sumData2)
	sumResp2.Body.Close()
	if len(sumData2.ModelBreakdowns) != 1 || sumData2.ModelBreakdowns[0].ModelID != "gemini-3.8-flash" {
		t.Fatalf("expected deleted custom model usage to be purged from summary, got %+v", sumData2.ModelBreakdowns)
	}
}

func TestMultiAppSyncAndSwitchAPI(t *testing.T) {
	os.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")
	defer os.Unsetenv("ANTIGRAVITY_TEST_DRY_RUN")

	tmpDir := t.TempDir()
	os.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("ANTIGRAVITY_SWISS_CONFIG_DIR")

	store, err := keyring.NewStore("")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	_, _ = store.ImportAccount("cli-user@google.com", "rt_cli", "at_cli", "CLI User", "")
	_, _ = store.ImportAccount("fleet-shared@google.com", "rt_shared", "at_shared", "Shared User", "")

	dummySock := filepath.Join(tmpDir, "dummy.sock")
	srv := NewServer("127.0.0.1:0", dummySock)
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/rules checks default multi_app_sync_mode and installed_apps
	resp, err := http.Get(baseURL + "/api/rules")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/rules failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var rules map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		t.Fatalf("failed to decode rules: %v", err)
	}
	resp.Body.Close()

	if mode, ok := rules["multi_app_sync_mode"].(string); !ok || mode != "shared" {
		t.Errorf("expected default multi_app_sync_mode 'shared', got '%v'", rules["multi_app_sync_mode"])
	}
	if _, ok := rules["installed_apps"].(map[string]interface{}); !ok {
		t.Errorf("expected installed_apps to be populated in rules, got '%v'", rules["installed_apps"])
	}

	// 2. POST /api/rules updates multi_app_sync_mode to individual
	postData := map[string]interface{}{
		"multi_app_sync_mode": "individual",
	}
	pBytes, _ := json.Marshal(postData)
	postResp, err := http.Post(baseURL+"/api/rules", "application/json", bytes.NewReader(pBytes))
	if err != nil || postResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/rules failed: err=%v, code=%d", err, postResp.StatusCode)
	}
	postResp.Body.Close()

	// Verify updated rule
	resp2, err := http.Get(baseURL + "/api/rules")
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/rules failed: err=%v, code=%d", err, resp2.StatusCode)
	}
	var updatedRules map[string]interface{}
	_ = json.NewDecoder(resp2.Body).Decode(&updatedRules)
	resp2.Body.Close()

	if mode, ok := updatedRules["multi_app_sync_mode"].(string); !ok || mode != "individual" {
		t.Errorf("expected updated multi_app_sync_mode 'individual', got '%v'", updatedRules["multi_app_sync_mode"])
	}

	// 3. POST /api/switch with target_app = "agy"
	switchPayload := map[string]interface{}{
		"email":        "cli-user@google.com",
		"target_app":   "agy",
		"relaunch_ide": false,
	}
	sBytes, _ := json.Marshal(switchPayload)
	sResp, err := http.Post(baseURL+"/api/switch", "application/json", bytes.NewReader(sBytes))
	if err != nil || sResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/switch failed: err=%v, code=%d", err, sResp.StatusCode)
	}
	var sResult map[string]interface{}
	if err := json.NewDecoder(sResp.Body).Decode(&sResult); err != nil {
		t.Fatalf("decode switch response error: %v", err)
	}
	sResp.Body.Close()

	if success, ok := sResult["success"].(bool); !ok || !success {
		t.Errorf("expected success true in switch response, got %v", sResult)
	}
	if target, ok := sResult["target_app"].(string); !ok || target != "agy" {
		t.Errorf("expected target_app 'agy', got '%v'", sResult["target_app"])
	}
	if activeApps, ok := sResult["active_app_accounts"].(map[string]interface{}); ok {
		if activeApps["agy"] != "cli-user@google.com" {
			t.Errorf("expected active_app_accounts[agy] to be 'cli-user@google.com', got '%v'", activeApps["agy"])
		}
	} else {
		t.Errorf("expected active_app_accounts in response, got %v", sResult["active_app_accounts"])
	}

	// 4. POST /api/switch with target_app = "all"
	switchAllPayload := map[string]interface{}{
		"email":        "fleet-shared@google.com",
		"target_app":   "all",
		"relaunch_ide": false,
	}
	saBytes, _ := json.Marshal(switchAllPayload)
	saResp, err := http.Post(baseURL+"/api/switch", "application/json", bytes.NewReader(saBytes))
	if err != nil || saResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/switch (all) failed: err=%v, code=%d", err, saResp.StatusCode)
	}
	var saResult map[string]interface{}
	_ = json.NewDecoder(saResp.Body).Decode(&saResult)
	saResp.Body.Close()

	if activeApps, ok := saResult["active_app_accounts"].(map[string]interface{}); ok {
		if activeApps["desktop"] != "fleet-shared@google.com" || activeApps["agy"] != "fleet-shared@google.com" || activeApps["vscode"] != "fleet-shared@google.com" {
			t.Errorf("expected all apps updated to 'fleet-shared@google.com', got %+v", activeApps)
		}
	} else {
		t.Errorf("expected active_app_accounts in response, got %v", saResult["active_app_accounts"])
	}
}

func TestServer_ConversationEndpoints(t *testing.T) {
	TestConversationEndpoints(t)
}

func TestConversationEndpoints(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	tmpDir := t.TempDir()
	guiStore, _ := gui.NewStore(tmpDir)
	srv.SetGUIStore(guiStore)
	engine := revival.NewEngine(tmpDir, 9222)
	// Disable live CDP in detector so test runs offline
	engine.Detector.LiveCDPEnabled = false
	srv.SetRevivalEngine(engine)

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")
	validTestConvID := "c1111111-2222-3333-4444-555555555555"

	// 1. GET /api/conversations/active (empty initially)
	resp, err := http.Get(baseURL + "/api/conversations/active")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/conversations/active failed: err=%v, code=%d", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Mock CDP target and script executor on engine
	engine.CDPTrigger.TargetFinder = func(port int) (int, []gui.DevToolsTarget, error) {
		return 49999, []gui.DevToolsTarget{
			{
				URL:                  "https://127.0.0.1:41234/c/" + validTestConvID,
				WebSocketDebuggerURL: "ws://127.0.0.1:49999/devtools/page/1",
			},
		}, nil
	}
	engine.CDPTrigger.ScriptExecutor = func(wsURL, expression string) (map[string]interface{}, error) {
		return map[string]interface{}{"success": true}, nil
	}

	// 3. POST /api/conversations/revive
	revivePayload := map[string]interface{}{
		"target_app":      "desktop",
		"conversation_id": validTestConvID,
		"prompt":          "Continue task execution",
	}
	rBytes, _ := json.Marshal(revivePayload)
	reviveResp, err := http.Post(baseURL+"/api/conversations/revive", "application/json", bytes.NewReader(rBytes))
	if err != nil || reviveResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/conversations/revive failed: err=%v, code=%d", err, reviveResp.StatusCode)
	}
	var reviveResult map[string]interface{}
	_ = json.NewDecoder(reviveResp.Body).Decode(&reviveResult)
	reviveResp.Body.Close()

	if success, ok := reviveResult["success"].(bool); !ok || !success {
		t.Errorf("expected success true in revive response, got %v", reviveResult)
	}

	// 4. GET /api/conversations/status
	statusResp, err := http.Get(baseURL + "/api/conversations/status")
	if err != nil || statusResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/conversations/status failed: err=%v, code=%d", err, statusResp.StatusCode)
	}
	var statusResult map[string]interface{}
	_ = json.NewDecoder(statusResp.Body).Decode(&statusResult)
	statusResp.Body.Close()

	if success, ok := statusResult["success"].(bool); !ok || !success {
		t.Errorf("expected success true in status response, got %v", statusResult)
	}

	// 5. Test alias GET /api/session/continuation
	contResp, err := http.Get(baseURL + "/api/session/continuation")
	if err != nil || contResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/session/continuation failed: err=%v, code=%d", err, contResp.StatusCode)
	}
	contResp.Body.Close()

	// 6. POST /api/conversations/ack
	ackPayload := map[string]interface{}{
		"cascade_id": validTestConvID,
	}
	aBytes, _ := json.Marshal(ackPayload)
	ackResp, err := http.Post(baseURL+"/api/conversations/ack", "application/json", bytes.NewReader(aBytes))
	if err != nil || ackResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/conversations/ack failed: err=%v, code=%d", err, ackResp.StatusCode)
	}
	ackResp.Body.Close()

	// 7. POST /api/session/continuation/ack
	ackContResp, err := http.Post(baseURL+"/api/session/continuation/ack", "application/json", bytes.NewReader(aBytes))
	if err != nil || ackContResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/session/continuation/ack failed: err=%v, code=%d", err, ackContResp.StatusCode)
	}
	ackContResp.Body.Close()
}

func TestWebGUIUtilitiesACPEndpoint(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	resp, err := http.Get(baseURL + "/api/utilities/acp")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/utilities/acp failed: err=%v, code=%d", err, resp.StatusCode)
	}

	var result struct {
		Status          string                   `json:"status"`
		MeshNodes       int                      `json:"mesh_nodes"`
		ProtocolVersion string                   `json:"protocol_version"`
		Agents          []map[string]interface{} `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode ACP response: %v", err)
	}
	resp.Body.Close()

	if result.Status != "online" {
		t.Errorf("expected status 'online', got '%s'", result.Status)
	}
	if result.ProtocolVersion != "v1.2.0-draft" {
		t.Errorf("expected protocol_version 'v1.2.0-draft', got '%s'", result.ProtocolVersion)
	}
	if result.MeshNodes < 9 || len(result.Agents) < 9 {
		t.Errorf("expected at least 9 mesh nodes, got %d (len=%d)", result.MeshNodes, len(result.Agents))
	}

	// Required node IDs to verify
	expectedIDs := map[string]string{
		"agent-antigravity":      "Google Antigravity 2.0",
		"agent-antigravity-cli":  "Antigravity CLI (agy)",
		"agent-devin":            "Devin",
		"agent-opencode":         "OpenCode",
		"agent-deepseek-harness": "DeepSeek Harness",
		"agent-pi":               "Pi",
		"agent-codex":            "Codex",
		"agent-claude-code":      "Claude Code CLI",
		"agent-cursor":           "Cursor Editor Agent",
	}

	foundMap := make(map[string]map[string]interface{})
	for _, a := range result.Agents {
		id, _ := a["id"].(string)
		foundMap[id] = a
	}

	for expID, expName := range expectedIDs {
		agent, ok := foundMap[expID]
		if !ok {
			t.Errorf("missing expected agent node: %s", expID)
			continue
		}
		if name, _ := agent["name"].(string); name != expName {
			t.Errorf("expected agent %s name '%s', got '%s'", expID, expName, name)
		}
		if port, _ := agent["port_socket"].(string); port == "" {
			t.Errorf("agent %s has empty port_socket", expID)
		}
		if ver, _ := agent["acp_version"].(string); ver == "" {
			t.Errorf("agent %s has empty acp_version", expID)
		}
		if tools, ok := agent["supported_tools"].([]interface{}); !ok || len(tools) == 0 {
			t.Errorf("agent %s has empty or invalid supported_tools", expID)
		}
	}

	// Explicitly assert that 'Windsurf Cascade' was renamed to 'Devin'
	if windsurfAgent, exists := foundMap["agent-windsurf"]; exists {
		if name, _ := windsurfAgent["name"].(string); name == "Windsurf Cascade" {
			t.Errorf("expected Windsurf Cascade to be renamed to Devin")
		}
	}

	// Assert distinct separation between Desktop IDE and CLI daemon
	ideAgent := foundMap["agent-antigravity"]
	cliAgent := foundMap["agent-antigravity-cli"]
	if ideAgent != nil && cliAgent != nil {
		if ideAgent["type"] == cliAgent["type"] {
			t.Errorf("expected distinct types for IDE (%v) vs CLI (%v)", ideAgent["type"], cliAgent["type"])
		}
		if ideAgent["port_socket"] == cliAgent["port_socket"] {
			t.Errorf("expected distinct port_socket for IDE (%v) vs CLI (%v)", ideAgent["port_socket"], cliAgent["port_socket"])
		}
	}

	// Test backward compatibility alias query
	compatResp, err := http.Get(baseURL + "/api/utilities/acp?agent=agent-windsurf")
	if err != nil || compatResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/utilities/acp?agent=agent-windsurf failed: %v", err)
	}
	var compatResult struct {
		Agents []map[string]interface{} `json:"agents"`
	}
	_ = json.NewDecoder(compatResp.Body).Decode(&compatResult)
	compatResp.Body.Close()
	if len(compatResult.Agents) != 1 || compatResult.Agents[0]["id"] != "agent-devin" {
		t.Errorf("expected backward compatibility lookup for agent-windsurf to return agent-devin, got: %v", compatResult.Agents)
	}
}

func TestCacheConfig_UnlimitedDefaultsAndPersistence(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	dummySock := filepath.Join(tempDir, "isolated.sock")
	srv := NewServer("127.0.0.1:0", dummySock)
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/cache/config should return 0.0 defaults (Unlimited in age and size)
	resp, err := http.Get(baseURL + "/api/cache/config")
	if err != nil {
		t.Fatalf("GET /api/cache/config failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var getRes map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&getRes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	resp.Body.Close()

	if days, ok := getRes["prune_days"].(float64); !ok || days != 0.0 {
		t.Errorf("expected prune_days to default to 0.0, got %v", getRes["prune_days"])
	}
	if size, ok := getRes["max_size_gb"].(float64); !ok || size != 0.0 {
		t.Errorf("expected max_size_gb to default to 0.0, got %v", getRes["max_size_gb"])
	}

	// 2. POST /api/cache/config with 0.0 for age and size
	postPayload := map[string]interface{}{
		"auto_prune_enabled": true,
		"prune_days":         0.0,
		"max_size_gb":        0.0,
	}
	payloadBytes, _ := json.Marshal(postPayload)
	postResp, err := http.Post(baseURL+"/api/cache/config", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("POST /api/cache/config failed: %v", err)
	}
	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", postResp.StatusCode)
	}
	var postRes map[string]interface{}
	if err := json.NewDecoder(postResp.Body).Decode(&postRes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	postResp.Body.Close()

	if success, ok := postRes["success"].(bool); !ok || !success {
		t.Errorf("expected success true, got %v", postRes["success"])
	}
	if days, ok := postRes["prune_days"].(float64); !ok || days != 0.0 {
		t.Errorf("expected prune_days 0.0, got %v", postRes["prune_days"])
	}
	if size, ok := postRes["max_size_gb"].(float64); !ok || size != 0.0 {
		t.Errorf("expected max_size_gb 0.0, got %v", postRes["max_size_gb"])
	}

	// 3. Verify GET returns persisted 0.0
	resp2, err := http.Get(baseURL + "/api/cache/config")
	if err != nil {
		t.Fatalf("second GET /api/cache/config failed: %v", err)
	}
	defer resp2.Body.Close()
	var getRes2 map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&getRes2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if size, ok := getRes2["max_size_gb"].(float64); !ok || size != 0.0 {
		t.Errorf("expected persisted max_size_gb 0.0, got %v", getRes2["max_size_gb"])
	}
	if days, ok := getRes2["prune_days"].(float64); !ok || days != 0.0 {
		t.Errorf("expected persisted prune_days 0.0, got %v", getRes2["prune_days"])
	}

	// 4. Test Scan endpoint accepts 0.0 without errors
	scanResp, err := http.Get(baseURL + "/api/cache/scan?days=0&max_size_gb=0")
	if err != nil {
		t.Fatalf("GET /api/cache/scan failed: %v", err)
	}
	if scanResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from cache scan, got %d", scanResp.StatusCode)
	}
	scanResp.Body.Close()

	// 5. Test Prune endpoint accepts 0.0 without errors
	prunePayload := map[string]interface{}{
		"min_age_days": 0.0,
		"max_size_gb":  0.0,
		"dry_run":      true,
	}
	pruneBytes, _ := json.Marshal(prunePayload)
	pruneResp, err := http.Post(baseURL+"/api/cache/prune", "application/json", bytes.NewReader(pruneBytes))
	if err != nil {
		t.Fatalf("POST /api/cache/prune failed: %v", err)
	}
	if pruneResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from cache prune, got %d", pruneResp.StatusCode)
	}
	pruneResp.Body.Close()
}

func TestSubagentModelStrategy_RulesAPI(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	dummySock := filepath.Join(tempDir, "dummy.sock")
	srv := NewServer("127.0.0.1:0", dummySock)
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/rules checks default subagent_model_strategy is default_custom_only
	resp, err := http.Get(baseURL + "/api/rules")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/rules failed: err=%v, code=%d", err, resp.StatusCode)
	}
	var rules map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		t.Fatalf("failed to decode rules: %v", err)
	}
	resp.Body.Close()

	if strat, ok := rules["subagent_model_strategy"].(string); !ok || strat != core.SubagentModelStrategyDefaultCustomOnly {
		t.Errorf("expected default subagent_model_strategy %q, got %v", core.SubagentModelStrategyDefaultCustomOnly, rules["subagent_model_strategy"])
	}

	// 2. POST /api/rules updates subagent_model_strategy to auto_decide
	postData := map[string]interface{}{
		"subagent_model_strategy": "auto_decide",
	}
	pBytes, _ := json.Marshal(postData)
	postResp, err := http.Post(baseURL+"/api/rules", "application/json", bytes.NewReader(pBytes))
	if err != nil || postResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/rules failed: err=%v, code=%d", err, postResp.StatusCode)
	}
	postResp.Body.Close()

	// 3. GET /api/rules verifies updated subagent_model_strategy
	resp2, err := http.Get(baseURL + "/api/rules")
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/rules failed: err=%v, code=%d", err, resp2.StatusCode)
	}
	var updatedRules map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&updatedRules); err != nil {
		t.Fatalf("failed to decode updated rules: %v", err)
	}
	resp2.Body.Close()

	if strat, ok := updatedRules["subagent_model_strategy"].(string); !ok || strat != core.SubagentModelStrategyAutoDecide {
		t.Errorf("expected updated subagent_model_strategy %q, got %v", core.SubagentModelStrategyAutoDecide, updatedRules["subagent_model_strategy"])
	}

	// 4. Verify disk persistence via LoadConfig
	cfg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.GetSubagentModelStrategy() != core.SubagentModelStrategyAutoDecide {
		t.Errorf("expected persisted strategy %q, got %q", core.SubagentModelStrategyAutoDecide, cfg.GetSubagentModelStrategy())
	}
}

