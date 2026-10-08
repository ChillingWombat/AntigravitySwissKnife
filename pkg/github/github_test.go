package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitURLRegex(t *testing.T) {
	tests := []struct {
		url       string
		wantOwner string
		wantRepo  string
	}{
		{"https://github.com/ChillingWombat/AntigravitySwissKnife.git", "ChillingWombat", "AntigravitySwissKnife"},
		{"https://github.com/ChillingWombat/AntigravitySwissKnife", "ChillingWombat", "AntigravitySwissKnife"},
		{"git@github.com:ChillingWombat/AntigravitySwissKnife.git", "ChillingWombat", "AntigravitySwissKnife"},
		{"git@github.com:octocat/Hello-World", "octocat", "Hello-World"},
	}

	for _, tt := range tests {
		m := gitURLRegex.FindStringSubmatch(tt.url)
		if len(m) < 3 {
			t.Fatalf("regex failed to match url %q", tt.url)
		}
		if m[1] != tt.wantOwner {
			t.Errorf("url %q: got owner %q, want %q", tt.url, m[1], tt.wantOwner)
		}
		if m[2] != tt.wantRepo {
			t.Errorf("url %q: got repo %q, want %q", tt.url, m[2], tt.wantRepo)
		}
	}
}

func TestStorePersistenceAndBindings(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gh-store-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbFile := filepath.Join(tmpDir, "github_agent_tasks.json")
	store, err := NewStore(dbFile)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 1. Agent Labels
	if err := store.SetAgentLabel("conv-123", "Lead Orchestrator"); err != nil {
		t.Fatalf("SetAgentLabel failed: %v", err)
	}
	if got := store.GetAgentLabel("conv-123"); got != "Lead Orchestrator" {
		t.Errorf("expected Lead Orchestrator, got %q", got)
	}

	// 2. Issue Bindings
	if err := store.BindIssue("conv-123", 42); err != nil {
		t.Fatalf("BindIssue failed: %v", err)
	}
	if got := store.GetBoundIssue("conv-123"); got != 42 {
		t.Errorf("expected issue 42, got %d", got)
	}

	convs := store.GetConversationsForIssue(42)
	if len(convs) != 1 || convs[0] != "conv-123" {
		t.Errorf("expected [conv-123], got %v", convs)
	}

	// 3. Reload from disk
	store2, err := NewStore(dbFile)
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	if got := store2.GetAgentLabel("conv-123"); got != "Lead Orchestrator" {
		t.Errorf("reloaded: expected Lead Orchestrator, got %q", got)
	}
	if got := store2.GetBoundIssue("conv-123"); got != 42 {
		t.Errorf("reloaded: expected issue 42, got %d", got)
	}

	// 4. Clear binding
	if err := store.BindIssue("conv-123", 0); err != nil {
		t.Fatal(err)
	}
	if got := store.GetBoundIssue("conv-123"); got != 0 {
		t.Errorf("expected unbound 0, got %d", got)
	}
}

func TestWorkspacePathMatching(t *testing.T) {
	uris := `["file:///mnt/Data/Projects/Antigravity%20Swiss%20Knife"]`
	target := "/mnt/Data/Projects/Antigravity Swiss Knife"

	if !matchesWorkspace(uris, normalizePath(target)) {
		t.Errorf("expected matchesWorkspace to return true for %s", target)
	}

	if !matchesWorkspace(uris, normalizePath("Antigravity Swiss Knife")) {
		t.Errorf("expected matchesWorkspace to return true for bare project name")
	}

	if !matchesWorkspace(uris, normalizePath("GLOBAL")) {
		t.Errorf("expected matchesWorkspace to return true for GLOBAL scope")
	}

	if !matchesWorkspace(uris, normalizePath(".")) {
		t.Errorf("expected matchesWorkspace to return true for '.'")
	}

	if matchesWorkspace(uris, normalizePath("/mnt/Data/Other/Project")) {
		t.Errorf("expected matchesWorkspace to return false for unrelated project")
	}

	wantDefault := ResolveProjectPath(".")
	if got := ResolveProjectPath("GLOBAL"); got != wantDefault {
		t.Errorf("expected ResolveProjectPath(\"GLOBAL\") == %q, got %q", wantDefault, got)
	}
	if got := ResolveProjectPath(""); got != wantDefault {
		t.Errorf("expected ResolveProjectPath(\"\") == %q, got %q", wantDefault, got)
	}
	resolved := ResolveProjectPath("Antigravity Swiss Knife")
	if resolved == "" || filepath.Base(resolved) != "Antigravity Swiss Knife" {
		t.Errorf("expected ResolveProjectPath(\"Antigravity Swiss Knife\") to resolve to project dir, got %q", resolved)
	}
}

func TestDetectRepositoryLive(t *testing.T) {
	svc := NewService(nil, nil)
	repo, err := svc.DetectRepository(".")
	if err != nil {
		t.Fatalf("DetectRepository failed: %v", err)
	}

	if repo.Owner != "ChillingWombat" {
		t.Errorf("expected owner ChillingWombat, got %q", repo.Owner)
	}
	if repo.Name != "AntigravitySwissKnife" {
		t.Errorf("expected name AntigravitySwissKnife, got %q", repo.Name)
	}
	if repo.FullName != "ChillingWombat/AntigravitySwissKnife" {
		t.Errorf("expected full name ChillingWombat/AntigravitySwissKnife, got %q", repo.FullName)
	}
}

func TestGetKanbanBoardLive(t *testing.T) {
	svc := NewService(nil, nil)
	repo, err := svc.DetectRepository(".")
	if err != nil {
		t.Fatalf("DetectRepository failed: %v", err)
	}

	board, err := svc.GetKanbanBoard(repo, 0)
	if err != nil {
		t.Fatalf("GetKanbanBoard failed: %v", err)
	}

	if len(board.Columns) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(board.Columns))
	}
	expectedCols := []string{"todo", "in_progress", "review", "done"}
	for i, col := range board.Columns {
		if col.ID != expectedCols[i] {
			t.Errorf("col %d: expected ID %q, got %q", i, expectedCols[i], col.ID)
		}
	}
}

func TestMoveKanbanCardValidation(t *testing.T) {
	svc := NewService(nil, nil)

	// Nil repo
	if err := svc.MoveKanbanCard(nil, &MoveKanbanCardRequest{Number: 1, TargetColumn: "done"}); err == nil {
		t.Errorf("expected error for nil repo")
	}

	// Invalid number
	repo := &RepoInfo{FullName: "test/repo"}
	if err := svc.MoveKanbanCard(repo, &MoveKanbanCardRequest{Number: 0, TargetColumn: "done"}); err == nil {
		t.Errorf("expected error for number 0")
	}

	// Empty target column
	if err := svc.MoveKanbanCard(repo, &MoveKanbanCardRequest{Number: 1, TargetColumn: ""}); err == nil {
		t.Errorf("expected error for empty target column")
	}

	// Invalid target column name
	if err := svc.MoveKanbanCard(repo, &MoveKanbanCardRequest{Number: 1, TargetColumn: "non_existent_column"}); err == nil {
		t.Errorf("expected error for invalid target column name")
	}

	// Nil repo for GetKanbanBoard
	if _, err := svc.GetKanbanBoard(nil, 0); err == nil {
		t.Errorf("expected error for nil repo in GetKanbanBoard")
	}
}


