package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
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

	// 3b. Test /api/github/agent-tasks/report
	reportPayload, _ := json.Marshal(map[string]interface{}{
		"workspace_path":  tmpDir,
		"conversation_id": "test-conv-002",
		"work_item":       "Refactor Token Price Table and Model Pins",
		"agent_label":     "Token Telemetry Worker",
		"issues":          []int{38, 42},
		"prs":             []int{15},
		"status":          "working",
	})
	reqReport := httptest.NewRequest("POST", "/api/github/agent-tasks/report", bytes.NewReader(reportPayload))
	rrReport := httptest.NewRecorder()
	server.handleGitHubAgentTaskReport(rrReport, reqReport)
	if rrReport.Code != http.StatusOK {
		t.Fatalf("expected 200 from report, got %d", rrReport.Code)
	}

	// 4. Test /api/github/kanban
	reqKanban := httptest.NewRequest("GET", "/api/github/kanban?workspace_path=.", nil)
	rrKanban := httptest.NewRecorder()
	server.handleGitHubKanbanBoard(rrKanban, reqKanban)

	if rrKanban.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrKanban.Code)
	}
	var respKanban map[string]interface{}
	if err := json.NewDecoder(rrKanban.Body).Decode(&respKanban); err != nil {
		t.Fatal(err)
	}
	if !respKanban["success"].(bool) {
		t.Fatalf("expected success true, got %v", respKanban)
	}
	boardMap, ok := respKanban["board"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected board object, got %v", respKanban["board"])
	}
	cols, ok := boardMap["columns"].([]interface{})
	if !ok || len(cols) != 4 {
		t.Fatalf("expected 4 columns, got %v", cols)
	}

	// 5. Test /api/github/kanban/move validation
	moveBody, _ := json.Marshal(map[string]interface{}{
		"workspace_path": ".",
		"number":         0,
		"target_column":  "in_progress",
	})
	reqMove := httptest.NewRequest("POST", "/api/github/kanban/move", bytes.NewReader(moveBody))
	rrMove := httptest.NewRecorder()
	server.handleGitHubKanbanMove(rrMove, reqMove)

	if rrMove.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrMove.Code)
	}
	var respMove map[string]interface{}
	_ = json.NewDecoder(rrMove.Body).Decode(&respMove)
	if respMove["success"].(bool) {
		t.Errorf("expected failure for number 0, got success")
	}

	// 6. Test /api/github/kanban/move with invalid target column
	invalidColBody, _ := json.Marshal(map[string]interface{}{
		"workspace_path": ".",
		"number":         1,
		"target_column":  "invalid_column_name",
	})
	reqInvalidCol := httptest.NewRequest("POST", "/api/github/kanban/move", bytes.NewReader(invalidColBody))
	rrInvalidCol := httptest.NewRecorder()
	server.handleGitHubKanbanMove(rrInvalidCol, reqInvalidCol)
	var respInvalidCol map[string]interface{}
	_ = json.NewDecoder(rrInvalidCol.Body).Decode(&respInvalidCol)
	if respInvalidCol["success"].(bool) {
		t.Errorf("expected failure for invalid target column, got success")
	}
}

// The strict repo probe lets the UI gate GitHub Workspace per project. A project
// without a GitHub remote must answer has_repo=false with success=true, so the
// caller can tell "no repository" apart from "daemon error".
func TestHandleGitHubRepoStrict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	store, _ := github.NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	svc := github.NewService(github.NewAgentTracker("", store), store)
	server := NewServer("127.0.0.1:0", "")
	server.SetGitHubService(svc)

	repoDir := t.TempDir()
	gitInit := func(dir string, remote string) {
		run := func(args ...string) {
			cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v failed: %v\n%s", args, err, out)
			}
		}
		run("init", "-q")
		run("config", "user.email", "test@example.com")
		run("config", "user.name", "Test")
		if remote != "" {
			run("remote", "add", "origin", remote)
		}
	}
	gitInit(repoDir, "git@github.com:ChillingWombat/AntigravitySwissKnife.git")
	plainDir := t.TempDir()
	gitInit(plainDir, "")

	decode := func(query string) map[string]interface{} {
		req := httptest.NewRequest("GET", "/api/github/repo?strict=1&workspace_path="+url.QueryEscape(query), nil)
		rr := httptest.NewRecorder()
		server.handleGitHubRepo(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body
	}

	withRepo := decode(repoDir)
	if withRepo["success"] != true || withRepo["has_repo"] != true {
		t.Fatalf("expected has_repo=true for a repo project, got %v", withRepo)
	}
	repo, ok := withRepo["repo"].(map[string]interface{})
	if !ok || repo["full_name"] != "ChillingWombat/AntigravitySwissKnife" {
		t.Fatalf("unexpected repo payload: %v", withRepo["repo"])
	}

	withoutRepo := decode(plainDir)
	if withoutRepo["success"] != true || withoutRepo["has_repo"] != false {
		t.Fatalf("expected success with has_repo=false for a repo-less project, got %v", withoutRepo)
	}
	if withoutRepo["repo"] != nil {
		t.Fatalf("expected no repo for a repo-less project, got %v", withoutRepo["repo"])
	}
}
