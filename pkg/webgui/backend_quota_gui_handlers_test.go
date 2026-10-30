package webgui

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/daemon"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
	_ "modernc.org/sqlite"
)

type ownedQuotaService struct {
	backend.QuotaService
	summary   quota.QuotaSummary
	fleet     quota.FleetQuotaSummary
	refreshes int
}

func (s *ownedQuotaService) GetSummary(email string, refresh bool) (*quota.QuotaSummary, error) {
	summary := s.summary
	if summary.AccountEmail == "" {
		summary.AccountEmail = email
	}
	return &summary, nil
}

func (s *ownedQuotaService) GetFleet() *quota.FleetQuotaSummary {
	fleet := s.fleet
	return &fleet
}

func (s *ownedQuotaService) RefreshFleet() error {
	s.refreshes++
	return nil
}

type ownedGUIService struct {
	backend.GUIService
	config   gui.Config
	projects []gui.ProjectItem
}

func (s *ownedGUIService) GetConfig() gui.Config { return s.config }

func (s *ownedGUIService) DetectProjects() ([]gui.ProjectItem, error) {
	return s.projects, nil
}

func TestQuotaHandlersUseInjectedOwner(t *testing.T) {
	state := &ownedQuotaService{
		summary: quota.QuotaSummary{AccountEmail: "owner@example.com"},
		fleet:   quota.FleetQuotaSummary{},
	}
	rpc := &recordingRPC{}
	server := &Server{backend: testBackend{quota: state}, client: rpc}

	rec := httptest.NewRecorder()
	server.handleQuota(rec, httptest.NewRequest(http.MethodGet, "/api/quota?email=owner@example.com", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("quota status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var summary quota.QuotaSummary
	if err := json.NewDecoder(rec.Body).Decode(&summary); err != nil {
		t.Fatalf("decode quota summary: %v", err)
	}
	if summary.AccountEmail != "owner@example.com" {
		t.Fatalf("quota account = %q, want owner@example.com", summary.AccountEmail)
	}

	rec = httptest.NewRecorder()
	server.handleFleetQuota(rec, httptest.NewRequest(http.MethodGet, "/api/fleet/quota", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("fleet quota status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	server.handleQuotaRefresh(rec, httptest.NewRequest(http.MethodPost, "/api/quota/refresh", nil))
	if rec.Code != http.StatusOK || state.refreshes != 1 {
		t.Fatalf("quota refresh status = %d, refreshes = %d", rec.Code, state.refreshes)
	}
	if rpc.calls != 0 {
		t.Fatalf("quota handlers made %d RPC calls", rpc.calls)
	}
}

func TestGUIReadHandlersUseInjectedOwner(t *testing.T) {
	config := *gui.DefaultConfig()
	config.AutoInject = true
	state := &ownedGUIService{
		config:   config,
		projects: []gui.ProjectItem{{Name: "owner-project"}},
	}
	rpc := &recordingRPC{}
	server := &Server{backend: testBackend{gui: state}, client: rpc}

	rec := httptest.NewRecorder()
	server.handleGUIConfig(rec, httptest.NewRequest(http.MethodGet, "/api/gui/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GUI config status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var returnedConfig gui.Config
	if err := json.NewDecoder(rec.Body).Decode(&returnedConfig); err != nil {
		t.Fatalf("decode GUI config: %v", err)
	}
	if !returnedConfig.AutoInject {
		t.Fatal("GUI config did not come from injected owner")
	}

	rec = httptest.NewRecorder()
	server.handleGUIProjects(rec, httptest.NewRequest(http.MethodGet, "/api/gui/projects", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GUI projects status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var projects []gui.ProjectItem
	if err := json.NewDecoder(rec.Body).Decode(&projects); err != nil {
		t.Fatalf("decode GUI projects: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "owner-project" {
		t.Fatalf("GUI projects = %+v, want owner-project", projects)
	}
	if rpc.calls != 0 {
		t.Fatalf("GUI read handlers made %d RPC calls", rpc.calls)
	}
}

func TestGUIConfigMutationThroughHTTPVisibleThroughRPC(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", configDir)
	t.Setenv("HOME", configDir)
	t.Setenv("USERPROFILE", configDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	socketPath := shortDaemonSocketPath(t)
	d, err := daemon.NewDaemon(core.DefaultConfig(), socketPath)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	if err := d.Start(); err != nil {
		t.Fatalf("daemon Start: %v", err)
	}
	t.Cleanup(func() { _ = d.Stop() })

	server, err := NewServer("127.0.0.1:0", d.Backend(), ipc.NewClient(socketPath))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	want := !d.GUIStore.GetConfig().AutoInject
	body, _ := json.Marshal(map[string]interface{}{"auto_inject": want})
	rec := httptest.NewRecorder()
	server.handleGUIConfig(rec, httptest.NewRequest(http.MethodPost, "/api/gui/config", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("GUI config mutation status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var rpcConfig gui.Config
	if err := ipc.NewClient(socketPath).Call("swiss.getGUIConfig", nil, &rpcConfig); err != nil {
		t.Fatalf("swiss.getGUIConfig: %v", err)
	}
	if rpcConfig.AutoInject != want {
		t.Fatalf("RPC auto_inject = %v, want %v", rpcConfig.AutoInject, want)
	}
}

func TestGUIAutoArchiveThroughHTTPUsesDaemonOwnedGUIState(t *testing.T) {
	configDir := t.TempDir()
	antiDir := filepath.Join(configDir, "antigravity")
	if err := os.MkdirAll(antiDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(antiDir, "conversation_summaries.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE conversation_summaries (
		conversation_id TEXT PRIMARY KEY,
		title TEXT,
		last_modified_time TEXT
	)`); err != nil {
		_ = db.Close()
		t.Fatalf("create conversation_summaries: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close conversation_summaries: %v", err)
	}

	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antiDir)
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", configDir)
	t.Setenv("HOME", configDir)
	t.Setenv("USERPROFILE", configDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	socketPath := shortDaemonSocketPath(t)
	d, err := daemon.NewDaemon(core.DefaultConfig(), socketPath)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	if err := d.Start(); err != nil {
		t.Fatalf("daemon Start: %v", err)
	}
	t.Cleanup(func() { _ = d.Stop() })

	wantHorizon := "37d"
	cfg := d.GUIStore.GetConfig()
	cfg.AutoArchiveHorizon = wantHorizon
	rawConfig, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal GUI config: %v", err)
	}
	if _, err := d.GUIStore.UpdateConfigPartial(rawConfig); err != nil {
		t.Fatalf("set daemon GUI config: %v", err)
	}

	server, err := NewServer("127.0.0.1:0", d.Backend(), ipc.NewClient(socketPath))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if server.guiStore != nil {
		t.Fatal("daemon-backed server opened a local GUI store")
	}

	original := newGUIConfigStore
	localStoreOpens := 0
	newGUIConfigStore = func(string) (*gui.Store, error) {
		localStoreOpens++
		return nil, errors.New("local GUI store must not open")
	}
	t.Cleanup(func() { newGUIConfigStore = original })

	rec := httptest.NewRecorder()
	server.handleGUIConversationsAutoArchive(rec, httptest.NewRequest(
		http.MethodPost,
		"/api/gui/conversations/auto-archive",
		bytes.NewReader([]byte(`{"horizon":""}`)),
	))
	if rec.Code != http.StatusOK {
		t.Fatalf("auto-archive status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if localStoreOpens != 0 {
		t.Fatalf("auto-archive opened %d local GUI stores", localStoreOpens)
	}

	var result gui.AutoArchiveResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode auto-archive result: %v", err)
	}
	if !result.Success || result.Horizon != wantHorizon {
		t.Fatalf("auto-archive result = %+v, want success at %s", result, wantHorizon)
	}

	var rpcConfig gui.Config
	if err := ipc.NewClient(socketPath).Call("swiss.getGUIConfig", nil, &rpcConfig); err != nil {
		t.Fatalf("swiss.getGUIConfig: %v", err)
	}
	if rpcConfig.AutoArchiveHorizon != wantHorizon {
		t.Fatalf("RPC auto_archive_horizon = %q, want %q", rpcConfig.AutoArchiveHorizon, wantHorizon)
	}
}

func TestGUIAutoArchiveStandaloneStoreStillWorks(t *testing.T) {
	antiDir := filepath.Join(t.TempDir(), "antigravity")
	if err := os.MkdirAll(antiDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(antiDir, "conversation_summaries.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE conversation_summaries (
		conversation_id TEXT PRIMARY KEY,
		title TEXT,
		last_modified_time TEXT
	)`); err != nil {
		_ = db.Close()
		t.Fatalf("create conversation_summaries: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close conversation_summaries: %v", err)
	}
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antiDir)

	store, err := gui.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("gui.NewStore: %v", err)
	}
	cfg := store.GetConfig()
	cfg.AutoArchiveHorizon = "21d"
	rawConfig, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal GUI config: %v", err)
	}
	if _, err := store.UpdateConfigPartial(rawConfig); err != nil {
		t.Fatalf("set standalone GUI config: %v", err)
	}

	server := &Server{guiStore: store}
	rec := httptest.NewRecorder()
	server.handleGUIConversationsAutoArchive(rec, httptest.NewRequest(
		http.MethodPost,
		"/api/gui/conversations/auto-archive",
		bytes.NewReader([]byte(`{"horizon":""}`)),
	))
	if rec.Code != http.StatusOK {
		t.Fatalf("auto-archive status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var result gui.AutoArchiveResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode auto-archive result: %v", err)
	}
	if !result.Success || result.Horizon != "21d" {
		t.Fatalf("auto-archive result = %+v, want success at 21d", result)
	}
}

type standaloneQuotaGUIRPC struct {
	calls int
}

func (r *standaloneQuotaGUIRPC) Call(method string, params interface{}, out interface{}) error {
	r.calls++
	switch method {
	case "swiss.getQuotaSummary":
		*out.(*quota.QuotaSummary) = quota.QuotaSummary{AccountEmail: "rpc@example.com"}
	case "swiss.getFleetQuota":
		*out.(*quota.FleetQuotaSummary) = quota.FleetQuotaSummary{}
	case "swiss.getGUIConfig":
		config := gui.DefaultConfig()
		config.AutoInject = true
		*out.(*gui.Config) = *config
	case "swiss.getGUIProjects":
		*out.(*[]gui.ProjectItem) = []gui.ProjectItem{{Name: "rpc-project"}}
	default:
		return errors.New("unexpected RPC method: " + method)
	}
	return nil
}

func TestStandaloneQuotaAndGUIUseExplicitRPC(t *testing.T) {
	rpc := &standaloneQuotaGUIRPC{}
	server := &Server{client: rpc}

	rec := httptest.NewRecorder()
	server.handleQuota(rec, httptest.NewRequest(http.MethodGet, "/api/quota?email=rpc@example.com", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("quota status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	server.handleFleetQuota(rec, httptest.NewRequest(http.MethodGet, "/api/fleet/quota", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("fleet quota status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	server.handleGUIConfig(rec, httptest.NewRequest(http.MethodGet, "/api/gui/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GUI config status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	server.handleGUIProjects(rec, httptest.NewRequest(http.MethodGet, "/api/gui/projects", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GUI projects status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rpc.calls != 4 {
		t.Fatalf("standalone RPC calls = %d, want 4", rpc.calls)
	}
}
