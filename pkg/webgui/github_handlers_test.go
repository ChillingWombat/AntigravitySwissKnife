package webgui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

	server := newTestServer("127.0.0.1:0", "")
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

func TestGitHubMutationHandlersRejectInvalidExplicitWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX executable shims")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "stale-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("MkdirAll repo: %v", err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", "https://github.com/example/stale-repo.git"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	store, _ := github.NewStore(filepath.Join(tmpDir, "tasks.json"))
	svc := github.NewService(github.NewAgentTracker("", store), store)
	if _, err := svc.DetectRepository(repoDir); err != nil {
		t.Fatalf("seed last-good repository: %v", err)
	}
	server := newTestServer("127.0.0.1:0", "")
	server.SetGitHubService(svc)

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath git: %v", err)
	}
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("MkdirAll bin: %v", err)
	}
	ghLog := filepath.Join(tmpDir, "gh-calls.log")
	writeGitHubTestExecutable(t, filepath.Join(binDir, "git"), fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", quoteGitHubTestShell(realGit)))
	writeGitHubTestExecutable(t, filepath.Join(binDir, "gh"), fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %s\nexit 1\n", quoteGitHubTestShell(ghLog)))
	t.Setenv("PATH", binDir)

	invalidWorkspace := filepath.Join(tmpDir, "does-not-exist")
	cases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		payload map[string]interface{}
	}{
		{
			name:    "issue update",
			handler: server.handleGitHubIssueUpdate,
			payload: map[string]interface{}{
				"workspace_path": invalidWorkspace,
				"number":         1,
				"title":          "must not mutate stale repo",
			},
		},
		{
			name:    "issue comment",
			handler: server.handleGitHubIssueComment,
			payload: map[string]interface{}{
				"workspace_path": invalidWorkspace,
				"number":         1,
				"comment":        "must not mutate stale repo",
			},
		},
		{
			name:    "issue create",
			handler: server.handleGitHubIssueCreate,
			payload: map[string]interface{}{
				"workspace_path": invalidWorkspace,
				"title":          "must not mutate stale repo",
			},
		},
		{
			name:    "kanban move",
			handler: server.handleGitHubKanbanMove,
			payload: map[string]interface{}{
				"workspace_path": invalidWorkspace,
				"number":         1,
				"target_column":  "in_progress",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/github/mutation", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			tc.handler(rec, req)

			var response map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if success, _ := response["success"].(bool); success {
				t.Fatalf("invalid workspace unexpectedly succeeded: %v", response)
			}
			if data, err := os.ReadFile(ghLog); err == nil {
				t.Fatalf("mutation invoked gh through a stale fallback repo: %s", data)
			} else if !os.IsNotExist(err) {
				t.Fatalf("read gh log: %v", err)
			}
			if message, _ := response["error"].(string); !strings.Contains(message, "no git origin remote") {
				t.Fatalf("expected strict repository detection error, got %q", message)
			}
		})
	}
}

func writeGitHubTestExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0700); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func quoteGitHubTestShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
