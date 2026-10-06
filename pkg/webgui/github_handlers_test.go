package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/github"
)

func TestGitHubHandlers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gh-handler-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "tasks.json")
	store, _ := github.NewStore(storePath)
	tracker := github.NewAgentTracker("", store)
	svc := github.NewService(tracker, store)

	server := NewServer("127.0.0.1:0", "")
	server.SetGitHubService(svc)

	// 1. Test /api/github/repo
	reqRepo := httptest.NewRequest("GET", "/api/github/repo?workspace_path=.", nil)
	rrRepo := httptest.NewRecorder()
	server.handleGitHubRepo(rrRepo, reqRepo)

	if rrRepo.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrRepo.Code)
	}
	var respRepo map[string]interface{}
	if err := json.NewDecoder(rrRepo.Body).Decode(&respRepo); err != nil {
		t.Fatal(err)
	}
	if !respRepo["success"].(bool) {
		t.Fatalf("expected success true, got %v", respRepo)
	}

	// 2. Test /api/github/agent-tasks/label
	labelBody, _ := json.Marshal(map[string]string{
		"conversation_id": "test-conv-001",
		"agent_label":     "Lead Refactorer",
	})
	reqLabel := httptest.NewRequest("POST", "/api/github/agent-tasks/label", bytes.NewReader(labelBody))
	rrLabel := httptest.NewRecorder()
	server.handleGitHubAgentTaskLabel(rrLabel, reqLabel)

	if rrLabel.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrLabel.Code)
	}

	// 3. Test /api/github/agent-tasks/bind
	bindBody, _ := json.Marshal(map[string]interface{}{
		"conversation_id": "test-conv-001",
		"issue_number":    25,
		"agent_label":     "Quota Bug Specialist",
	})
	reqBind := httptest.NewRequest("POST", "/api/github/agent-tasks/bind", bytes.NewReader(bindBody))
	rrBind := httptest.NewRecorder()
	server.handleGitHubAgentTaskBind(rrBind, reqBind)

	if rrBind.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrBind.Code)
	}

	if got := store.GetAgentLabel("test-conv-001"); got != "Quota Bug Specialist" {
		t.Errorf("expected Quota Bug Specialist, got %q", got)
	}
	if got := store.GetBoundIssue("test-conv-001"); got != 25 {
		t.Errorf("expected bound issue 25, got %d", got)
	}
}
