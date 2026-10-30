package webgui

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/github"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/revival"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/vault"
)

type backendModeRPC struct {
	calls int
}

func (r *backendModeRPC) Call(string, interface{}, interface{}) error {
	r.calls++
	return nil
}

type fakeGitHubHandlerService struct {
	githubHandlerService
	repo        *github.RepoInfo
	issues      []github.Issue
	workspace   string
	detectCalls int
	listCalls   int
}

func (f *fakeGitHubHandlerService) DetectRepository(workspacePath string) (*github.RepoInfo, error) {
	f.detectCalls++
	f.workspace = workspacePath
	return f.repo, nil
}

func (f *fakeGitHubHandlerService) ListIssues(_ *github.RepoInfo, _, _ string) ([]github.Issue, error) {
	f.listCalls++
	return f.issues, nil
}

func TestGitHubHandlersUseBackendOwnedService(t *testing.T) {
	root := t.TempDir()
	store, err := github.NewStore(filepath.Join(root, "github.json"))
	if err != nil {
		t.Fatalf("github.NewStore: %v", err)
	}
	service := github.NewService(github.NewAgentTracker("", store), store)
	b := baseBackend(t)
	b.github = service
	rpc := &backendModeRPC{}
	server, err := NewServer("127.0.0.1:0", b, rpc)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if server.githubService != service {
		t.Fatal("server did not retain the backend-owned GitHub service")
	}

	fake := &fakeGitHubHandlerService{
		repo:   &github.RepoInfo{},
		issues: []github.Issue{{Number: 7}},
	}
	server.githubService = fake
	workspace := t.TempDir()
	repoRec := callJSONHandler(t, server.handleGitHubRepo, "GET", "/api/github/repo?workspace_path="+url.QueryEscape(workspace), nil)
	var repoResp struct {
		Success bool             `json:"success"`
		HasRepo bool             `json:"has_repo"`
		Repo    *github.RepoInfo `json:"repo"`
	}
	if err := json.Unmarshal(repoRec.Body.Bytes(), &repoResp); err != nil {
		t.Fatalf("decode repo response: %v", err)
	}
	if !repoResp.Success || !repoResp.HasRepo || repoResp.Repo == nil {
		t.Fatalf("repo response = %s", repoRec.Body.String())
	}

	issuesRec := callJSONHandler(t, server.handleGitHubIssues, "GET", "/api/github/issues?workspace_path="+url.QueryEscape(workspace), nil)
	var issuesResp struct {
		Success bool           `json:"success"`
		Issues  []github.Issue `json:"issues"`
	}
	if err := json.Unmarshal(issuesRec.Body.Bytes(), &issuesResp); err != nil {
		t.Fatalf("decode issues response: %v", err)
	}
	if !issuesResp.Success || len(issuesResp.Issues) != 1 || issuesResp.Issues[0].Number != 7 {
		t.Fatalf("issues response = %s", issuesRec.Body.String())
	}
	if fake.workspace != workspace || fake.detectCalls != 2 || fake.listCalls != 1 {
		t.Fatalf("injected service calls = workspace:%q detect:%d list:%d", fake.workspace, fake.detectCalls, fake.listCalls)
	}
	if rpc.calls != 0 {
		t.Fatalf("backend-mode GitHub handlers made %d RPC calls", rpc.calls)
	}
}

func TestVaultToggleUsesBackendOwnedManager(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", filepath.Join(home, "config"))

	manager := vault.NewManager("", "")
	b := baseBackend(t)
	b.vault = manager
	server, err := NewServer("127.0.0.1:0", b, ipc.NewClient(filepath.Join(home, "daemon.sock")))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if server.getVaultManager() != manager {
		t.Fatal("server did not retain the backend-owned vault manager")
	}

	toggleRec := callJSONHandler(t, server.handleVaultToggle, "POST", "/api/vault/toggle", map[string]bool{"enabled": false})
	var toggleResp struct {
		Success bool `json:"success"`
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(toggleRec.Body.Bytes(), &toggleResp); err != nil {
		t.Fatalf("decode toggle response: %v", err)
	}
	if !toggleResp.Success || toggleResp.Enabled {
		t.Fatalf("toggle response = %s", toggleRec.Body.String())
	}

	statusRec := callJSONHandler(t, server.handleVaultStatus, "GET", "/api/vault/status", nil)
	var statusResp map[string]interface{}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if enabled, ok := statusResp["enabled"].(bool); !ok || enabled {
		t.Fatalf("status response = %s", statusRec.Body.String())
	}
	cfg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ConversationVaultEnabled {
		t.Fatal("vault toggle was not persisted")
	}
}

func TestRevivalStatusUsesBackendOwnedEngine(t *testing.T) {
	engine := revival.NewEngine(t.TempDir(), 0)
	if err := engine.Store.SaveIntent(&revival.RevivalIntent{
		RootConversationID: "backend-conversation",
		TriggerPrompt:      "continue through backend",
		Timestamp:          time.Now().UnixMilli(),
		Status:             "pending",
	}); err != nil {
		t.Fatalf("SaveIntent: %v", err)
	}

	b := baseBackend(t)
	b.revival = engine
	rpc := &backendModeRPC{}
	server, err := NewServer("127.0.0.1:0", b, rpc)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if server.getRevivalEngine() != engine {
		t.Fatal("server did not retain the backend-owned revival engine")
	}

	rec := callJSONHandler(t, server.handleConversationsStatus, "GET", "/api/conversations/status?conversation_id=backend-conversation", nil)
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if resp["pending_intent"] == nil {
		t.Fatalf("status response = %s", rec.Body.String())
	}
	if rpc.calls != 0 {
		t.Fatalf("backend-mode revival handler made %d RPC calls", rpc.calls)
	}
}

func TestReviveBackendModeDoesNotFallBackToRPC(t *testing.T) {
	rpc := &backendModeRPC{}
	server := &Server{backend: testBackend{}, client: rpc}
	rec := callJSONHandler(t, server.handleConversationsRevive, "POST", "/api/conversations/revive", map[string]string{})
	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if rpc.calls != 0 {
		t.Fatalf("backend-mode revive made %d RPC calls", rpc.calls)
	}
}
