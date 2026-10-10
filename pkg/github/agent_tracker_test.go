package github

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestAgentTrackerSubagentAndPrunedResolution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tracker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "conversation_summaries.db")
	convsDir := filepath.Join(tmpDir, "conversations")
	if err := os.MkdirAll(convsDir, 0755); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE conversation_summaries (
		conversation_id text PRIMARY KEY,
		title text,
		step_count integer,
		last_modified_time text,
		workspace_uris text,
		agent_name text,
		not_fully_idle numeric,
		parent_conversation_id text,
		nesting_depth integer
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	workspace := "/test/workspace"
	urisJSON := `["file:///test/workspace"]`
	now := time.Now().Format(time.RFC3339)

	// 1. Root conversation ("root-1") - physical db file exists
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"root-1", "Root Feature Task", 10, now, urisJSON, "Lead", 0, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(convsDir, "root-1.db"), []byte("sqlite"), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Child subagent ("sub-1") with parent "root-1", nesting_depth 1 - physical db file exists
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sub-1", "Subagent Slice", 5, now, urisJSON, "Worker", 1, "root-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(convsDir, "sub-1.db"), []byte("sqlite"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Grandchild subagent ("sub-2") with parent "sub-1", nesting_depth 2 - physical db missing (pruned)
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sub-2", "Grandchild Task", 2, now, urisJSON, "Worker-Leaf", 0, "sub-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	// Note: sub-2.db is NOT created, simulating pruned database

	// 4. Standalone pruned conversation ("pruned-root")
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"pruned-root", "Remove UI Elements", 1, now, urisJSON, "Worker", 0, "", 0)
	if err != nil {
		t.Fatal(err)
	}

	// 5. Child subagent ("sub-3") whose file exists, but its root parent ("pruned-root") is missing
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sub-3", "Child of Pruned Root", 1, now, urisJSON, "Worker", 0, "pruned-root", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(convsDir, "sub-3.db"), []byte("sqlite"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test tracker without SetConversationsDir: should automatically infer convsDir from filepath.Dir(dbPath)/conversations
	tracker := NewAgentTracker(dbPath, nil)

	tasks, err := tracker.ListWorkspaceTasks(workspace)
	if err != nil {
		t.Fatalf("ListWorkspaceTasks failed: %v", err)
	}

	if len(tasks) != 5 {
		t.Fatalf("expected 5 tasks, got %d", len(tasks))
	}

	taskMap := make(map[string]AgentTaskSummary)
	for _, task := range tasks {
		taskMap[task.ConversationID] = task
	}

	// Verify root-1
	rootTask, ok := taskMap["root-1"]
	if !ok {
		t.Fatal("missing root-1 task")
	}
	if rootTask.RootParentConversationID != "" {
		t.Errorf("expected empty RootParentConversationID for root-1, got %q", rootTask.RootParentConversationID)
	}
	if rootTask.IsPruned {
		t.Errorf("expected root-1 IsPruned false, got true")
	}

	// Verify sub-1 (depth 1) -> RootParentConversationID should be "root-1"
	sub1Task, ok := taskMap["sub-1"]
	if !ok {
		t.Fatal("missing sub-1 task")
	}
	if sub1Task.RootParentConversationID != "root-1" {
		t.Errorf("expected sub-1 RootParentConversationID 'root-1', got %q", sub1Task.RootParentConversationID)
	}
	if sub1Task.IsPruned {
		t.Errorf("expected sub-1 IsPruned false, got true")
	}

	// Verify sub-2 (depth 2) -> RootParentConversationID should resolve all the way to "root-1", and IsPruned should be true
	sub2Task, ok := taskMap["sub-2"]
	if !ok {
		t.Fatal("missing sub-2 task")
	}
	if sub2Task.RootParentConversationID != "root-1" {
		t.Errorf("expected sub-2 RootParentConversationID 'root-1' (resolved through sub-1), got %q", sub2Task.RootParentConversationID)
	}
	if !sub2Task.IsPruned {
		t.Errorf("expected sub-2 IsPruned true, got false")
	}

	// Verify pruned-root -> IsPruned should be true
	prunedTask, ok := taskMap["pruned-root"]
	if !ok {
		t.Fatal("missing pruned-root task")
	}
	if !prunedTask.IsPruned {
		t.Errorf("expected pruned-root IsPruned true, got false")
	}

	// Verify sub-3 -> its file exists, but root parent is pruned-root (missing) -> IsPruned must be true
	sub3Task, ok := taskMap["sub-3"]
	if !ok {
		t.Fatal("missing sub-3 task")
	}
	if sub3Task.RootParentConversationID != "pruned-root" {
		t.Errorf("expected sub-3 RootParentConversationID 'pruned-root', got %q", sub3Task.RootParentConversationID)
	}
	if !sub3Task.IsPruned {
		t.Errorf("expected sub-3 IsPruned true (since root parent db is missing), got false")
	}
}

func TestResolveRootParentIDCyclesAndMissing(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "conversation_summaries.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE conversation_summaries (
		conversation_id text PRIMARY KEY,
		parent_conversation_id text,
		nesting_depth integer
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	// 1. Cyclic parent reference: A -> B -> A
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES (?, ?, ?)`, "cycle-a", "cycle-b", 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES (?, ?, ?)`, "cycle-b", "cycle-a", 2)
	if err != nil {
		t.Fatal(err)
	}

	resA := resolveRootParentID(db, "cycle-a")
	if resA != "cycle-a" && resA != "cycle-b" {
		t.Errorf("expected cycle-a or cycle-b for cyclic parent, got %q", resA)
	}

	// 2. Missing parent row: child points to non-existent parent ID
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES (?, ?, ?)`, "orphan", "missing-parent", 1)
	if err != nil {
		t.Fatal(err)
	}
	resOrphan := resolveRootParentID(db, "missing-parent")
	if resOrphan != "missing-parent" {
		t.Errorf("expected missing-parent returned on missing row, got %q", resOrphan)
	}
}

func TestResolveProjectPath(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	parentDir := t.TempDir()
	projA := filepath.Join(parentDir, "ProjectAlpha")
	projB := filepath.Join(parentDir, "ProjectBeta")
	if err := os.MkdirAll(projA, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projB, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projA); err != nil {
		t.Fatal(err)
	}

	// 1. Resolve sibling project by bare name
	gotB := ResolveProjectPath("ProjectBeta")
	if gotB != projB {
		t.Errorf("expected ResolveProjectPath('ProjectBeta') = %q, got %q", projB, gotB)
	}

	// 2. Resolve non-existent new file inside sibling project (e.g. /api/files/write or mkdir)
	newFileRel := filepath.Join("ProjectBeta", "new_notes.md")
	gotNewFile := ResolveProjectPath(newFileRel)
	wantNewFile := filepath.Join(projB, "new_notes.md")
	if gotNewFile != wantNewFile {
		t.Errorf("expected ResolveProjectPath(%q) = %q, got %q", newFileRel, wantNewFile, gotNewFile)
	}

	// 3. Resolve "." returns current working directory
	gotDot := ResolveProjectPath(".")
	if gotDot != projA {
		t.Errorf("expected ResolveProjectPath('.') = %q, got %q", projA, gotDot)
	}

	// 4. Resolve "GLOBAL" returns a non-empty valid path
	gotGlobal := ResolveProjectPath("GLOBAL")
	if gotGlobal == "" || gotGlobal == "GLOBAL" {
		t.Errorf("expected ResolveProjectPath('GLOBAL') to resolve to a real path, got %q", gotGlobal)
	}
}

func TestExtractTaskAndWorkItemsFromTranscript(t *testing.T) {
	tmpDir := t.TempDir()
	brainDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", "test-conv", ".system_generated", "logs")
	if err := os.MkdirAll(brainDir, 0755); err != nil {
		t.Fatal(err)
	}

	transcriptContent := `{"type":"USER_INPUT","role":"user","content":"Please resolve issue #42 and check PR #15 before merging. Also see # 1. Introduction and color #aabbcc"}
{"type":"MODEL_RESPONSE","role":"assistant","content":"I looked at #55, #60, and #999 in the backlog."}
`
	if err := os.WriteFile(filepath.Join(brainDir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	workItem, issues, prs := extractTaskAndWorkItemsFromTranscript("test-conv", "Prompt mentioning fixes #42")
	if workItem == "" {
		t.Errorf("expected workItem extracted from user input")
	}
	if len(issues) != 1 || issues[0] != 42 {
		t.Errorf("expected only issue 42, got %v", issues)
	}
	if len(prs) != 1 || prs[0] != 15 {
		t.Errorf("expected only pr 15, got %v", prs)
	}
}

