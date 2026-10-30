package webgui

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/daemon"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestCustomModelHTTPWritesUseDaemonBackend(t *testing.T) {
	d, server := newDaemonBackedServer(t)
	handler := http.HandlerFunc(server.handleCustomModels)

	model := map[string]interface{}{
		"id":                       "http-model",
		"name":                     "qwen2.5:7b",
		"provider_type":            "ollama",
		"base_url":                 "http://127.0.0.1:11434",
		"quota_manual_override":    true,
		"quota_type":               "none",
		"price_source":             "manual",
		"input_price_per_m":        1.0,
		"cached_input_price_per_m": 0.1,
		"output_price_per_m":       2.0,
	}
	rec := callJSONHandler(t, handler, http.MethodPost, "/api/custom_models", model)
	if rec.Code != http.StatusOK {
		t.Fatalf("save custom model status = %d, body = %s", rec.Code, rec.Body.String())
	}

	backendStore := d.Backend().CustomModels()
	saved, err := backendStore.GetModel("http-model")
	if err != nil || saved == nil {
		t.Fatalf("daemon backend GetModel after save: model=%+v err=%v", saved, err)
	}
	if saved.Name != "qwen2.5:7b" {
		t.Fatalf("daemon backend model name = %q, want qwen2.5:7b", saved.Name)
	}

	rec = callJSONHandler(t, http.HandlerFunc(server.handleCustomModelActive), http.MethodPost, "/api/custom_models/active", map[string]string{"model_id": "http-model"})
	if rec.Code != http.StatusOK {
		t.Fatalf("activate custom model status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if active := backendStore.GetConfig().ActiveModelID; active != "http-model" {
		t.Fatalf("daemon backend active model = %q, want http-model", active)
	}

	model["name"] = "qwen2.5:14b"
	rec = callJSONHandler(t, handler, http.MethodPost, "/api/custom_models", model)
	if rec.Code != http.StatusOK {
		t.Fatalf("update custom model status = %d, body = %s", rec.Code, rec.Body.String())
	}
	updated, err := backendStore.GetModel("http-model")
	if err != nil || updated == nil {
		t.Fatalf("daemon backend GetModel after update: model=%+v err=%v", updated, err)
	}
	if updated.Name != "qwen2.5:14b" {
		t.Fatalf("daemon backend updated model name = %q, want qwen2.5:14b", updated.Name)
	}
}

func TestEnhancementHTTPUpdateUsesDaemonBackend(t *testing.T) {
	d, server := newDaemonBackedServer(t)
	handler := http.HandlerFunc(server.handleEnhancements)

	rec := callJSONHandler(t, handler, http.MethodGet, "/api/enhancements", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get enhancements status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var before map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatalf("decode enhancements: %v", err)
	}

	keys := make([]string, 0, len(before))
	for key := range before {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	toggled := ""
	for _, key := range keys {
		value, ok := before[key].(bool)
		if !ok {
			continue
		}
		before[key] = !value
		toggled = key
		break
	}
	if toggled == "" {
		t.Fatal("enhancement config has no boolean field to update")
	}

	rec = callJSONHandler(t, handler, http.MethodPost, "/api/enhancements/update", before)
	if rec.Code != http.StatusOK {
		t.Fatalf("update enhancements status = %d, body = %s", rec.Code, rec.Body.String())
	}
	afterJSON, err := json.Marshal(d.Backend().Enhancements().GetConfig())
	if err != nil {
		t.Fatalf("marshal daemon enhancement config: %v", err)
	}
	var after map[string]interface{}
	if err := json.Unmarshal(afterJSON, &after); err != nil {
		t.Fatalf("decode daemon enhancement config: %v", err)
	}
	want := before[toggled]
	if got := after[toggled]; got != want {
		t.Fatalf("daemon backend %s = %v, want %v", toggled, got, want)
	}
}

func TestBackendRequiresCustomModelAndEnhancementGroups(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "unused.sock")
	_, err := NewServer("127.0.0.1:0", missingCustomModelsBackend{baseBackend(t)}, ipc.NewClient(socketPath))
	if err == nil || !strings.Contains(err.Error(), "custom-model backend is required") {
		t.Fatalf("NewServer error = %v, want custom-model backend failure", err)
	}

	_, err = NewServer("127.0.0.1:0", missingEnhancementsBackend{baseBackend(t)}, ipc.NewClient(socketPath))
	if err == nil || !strings.Contains(err.Error(), "enhancement backend is required") {
		t.Fatalf("NewServer error = %v, want enhancement backend failure", err)
	}
}

func TestBackendModeDoesNotOpenLocalStateStores(t *testing.T) {
	b := baseBackend(t)
	originalCustomModels := newCustomModelsStore
	originalEnhancements := newEnhancementsStore
	newCustomModelsStore = func(string) (*custommodels.Store, error) {
		return nil, errors.New("custom-model constructor called in backend mode")
	}
	newEnhancementsStore = func(string) (*enhancements.Store, error) {
		return nil, errors.New("enhancement constructor called in backend mode")
	}
	t.Cleanup(func() {
		newCustomModelsStore = originalCustomModels
		newEnhancementsStore = originalEnhancements
	})

	server, err := NewServer("127.0.0.1:0", b, ipc.NewClient(filepath.Join(t.TempDir(), "unused.sock")))
	if err != nil {
		t.Fatalf("NewServer opened a local store in backend mode: %v", err)
	}
	if server.getCustomModelsStore() != b.customModels {
		t.Fatal("server custom-model store is not the injected backend owner")
	}
	if server.getEnhancementsStore() != b.enhancements {
		t.Fatal("server enhancement store is not the injected backend owner")
	}
}

type missingCustomModelsBackend struct {
	testBackend
}

func (missingCustomModelsBackend) CustomModels() *custommodels.Store { return nil }

type missingEnhancementsBackend struct {
	testBackend
}

func (missingEnhancementsBackend) Enhancements() *enhancements.Store { return nil }

func baseBackend(t *testing.T) testBackend {
	t.Helper()
	accountsStore, err := keyring.NewStore("")
	if err != nil {
		t.Fatalf("new account store: %v", err)
	}
	guiStore, err := gui.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new GUI store: %v", err)
	}
	customModelsStore, err := custommodels.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new custom-model store: %v", err)
	}
	enhancementsStore, err := enhancements.NewStore(filepath.Join(t.TempDir(), "enhancements.json"))
	if err != nil {
		t.Fatalf("new enhancement store: %v", err)
	}
	return testBackend{
		accounts:     testAccountService{store: accountsStore},
		customModels: customModelsStore,
		enhancements: enhancementsStore,
		quota:        testQuotaService{store: accountsStore},
		gui:          &testGUIService{store: guiStore},
	}
}

func newDaemonBackedServer(t *testing.T) (*daemon.Daemon, *Server) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", configDir)
	t.Setenv("HOME", configDir)
	t.Setenv("USERPROFILE", configDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	socketPath := filepath.Join(configDir, "daemon.sock")
	d, err := daemon.NewDaemon(core.DefaultConfig(), socketPath)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	server, err := NewServer("127.0.0.1:0", d.Backend(), ipc.NewClient(socketPath))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return d, server
}

func callJSONHandler(t *testing.T, handler http.HandlerFunc, method, target string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}
