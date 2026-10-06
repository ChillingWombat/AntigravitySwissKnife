package importer

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMatchWorkspaceProject(t *testing.T) {
	projects := []ProjectInfo{
		{
			ProjectID:   "proj-app",
			ProjectName: "App Workspace",
			Paths:       []string{"/workspace/app"},
		},
		{
			ProjectID:   "proj-notes",
			ProjectName: "Notes",
			Paths:       []string{"/workspace/notes"},
		},
	}

	// 1. Exact match
	pid, pName, status := MatchWorkspaceProject("/workspace/app", projects)
	if pid != "proj-app" || status != "exact" {
		t.Errorf("Expected exact match to proj-app, got pid=%s, status=%s", pid, status)
	}

	// 2. Subpath match
	pid, pName, status = MatchWorkspaceProject("/workspace/app/frontend/src", projects)
	if pid != "proj-app" || status != "heuristic" {
		t.Errorf("Expected subpath match to proj-app, got pid=%s, status=%s", pid, status)
	}

	// 3. Basename match
	pid, pName, status = MatchWorkspaceProject("/tmp/Notes", projects)
	if pid != "proj-notes" || status != "heuristic" {
		t.Errorf("Expected basename match to proj-notes, got pid=%s, status=%s", pid, status)
	}

	// 4. Empty/Fallback match
	pid, _, status = MatchWorkspaceProject("", projects)
	if pid != "proj-app" || status != "fallback" {
		t.Errorf("Expected fallback match to first project, got pid=%s, status=%s", pid, status)
	}
	_ = pName
}

func TestWriteConversationDBAndSummary(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_importer_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	convDir := filepath.Join(tmpDir, "conversations")
	brainDir := filepath.Join(tmpDir, "brain")
	summariesDB := filepath.Join(tmpDir, "conversation_summaries.db")

	cid := "test-conv-12345"
	steps := []ParsedStep{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: "2026-10-06T00:00:00Z",
			Content:   "Please optimize the authentication token refresh pipeline",
		},
		{
			StepIndex: 1,
			Source:    "MODEL",
			Type:      "PLANNER_RESPONSE",
			Status:    "DONE",
			CreatedAt: "2026-10-06T00:00:02Z",
			Content:   "I will examine the token storage and refresh rotation mechanism.",
		},
	}

	// 1. Write per-session DB
	if err := WriteConversationDB(convDir, cid, steps); err != nil {
		t.Fatalf("WriteConversationDB failed: %v", err)
	}

	// Verify per-session DB
	dbPath := filepath.Join(convDir, cid+".db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open session db: %v", err)
	}
	defer db.Close()

	var trajCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM trajectory_meta WHERE cascade_id = ?", cid).Scan(&trajCount); err != nil || trajCount != 1 {
		t.Errorf("Expected 1 trajectory_meta entry, got count=%d, err=%v", trajCount, err)
	}

	var stepCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM steps").Scan(&stepCount); err != nil || stepCount != 2 {
		t.Errorf("Expected 2 steps, got count=%d, err=%v", stepCount, err)
	}

	// 2. Write transcript log
	if err := WriteTranscriptLog(brainDir, cid, steps); err != nil {
		t.Fatalf("WriteTranscriptLog failed: %v", err)
	}
	transcriptPath := filepath.Join(brainDir, cid, ".system_generated", "logs", "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); err != nil {
		t.Errorf("Expected transcript.jsonl to exist at %s", transcriptPath)
	}

	// 3. Write summary DB
	workspaceURIs := []string{"file:///workspace/app"}
	if err := InsertConversationSummary(summariesDB, cid, "[Claude Code] Optimize auth", "Please optimize", len(steps), workspaceURIs, "proj-app", "CLAUDE_CODE_IMPORT", "Claude Code"); err != nil {
		t.Fatalf("InsertConversationSummary failed: %v", err)
	}

	// Verify summary DB
	sdb, err := sql.Open("sqlite", summariesDB)
	if err != nil {
		t.Fatalf("failed to open summaries db: %v", err)
	}
	defer sdb.Close()

	var summaryCount int
	var title, source string
	err = sdb.QueryRow("SELECT count(*), title, source FROM conversation_summaries WHERE conversation_id = ?", cid).Scan(&summaryCount, &title, &source)
	if err != nil || summaryCount != 1 {
		t.Errorf("Expected 1 summary row, got count=%d, err=%v", summaryCount, err)
	}
	if title != "[Claude Code] Optimize auth" || source != "CLAUDE_CODE_IMPORT" {
		t.Errorf("Unexpected title or source: title=%s, source=%s", title, source)
	}
}

func TestParseClaudeCodeAndEndToEndImport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_claude_import_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create fake claude transcript
	fakeJSONL := filepath.Join(tmpDir, "ses_test_123.jsonl")
	lines := `{"type":"user","timestamp":"2026-10-06T01:00:00Z","content":"Audit security probes in custom models"}
{"type":"tool_use","timestamp":"2026-10-06T01:00:02Z","tool_name":"run_command","tool_input":{"cmd":"go test"}}
{"type":"tool_result","timestamp":"2026-10-06T01:00:05Z","tool_name":"run_command","tool_output":"PASS"}
{"type":"assistant","timestamp":"2026-10-06T01:00:06Z","content":"All security probes verified."}
`
	if err := os.WriteFile(fakeJSONL, []byte(lines), 0644); err != nil {
		t.Fatalf("failed to write fake transcript: %v", err)
	}

	steps, title, preview, _, err := ParseClaudeCodeFile(fakeJSONL)
	if err != nil {
		t.Fatalf("ParseClaudeCodeFile failed: %v", err)
	}
	if len(steps) != 4 {
		t.Errorf("Expected 4 steps, got %d", len(steps))
	}
	if title != "[Claude Code] Audit security probes in custom models" {
		t.Errorf("Unexpected title: %s", title)
	}
	if preview != "Audit security probes in custom models" {
		t.Errorf("Unexpected preview: %s", preview)
	}

	// Test End-to-End Import
	antigravityBase := filepath.Join(tmpDir, "antigravity")
	appStorage := filepath.Join(tmpDir, "app_storage.json")
	_ = os.WriteFile(appStorage, []byte(`{"projectsOrder": "[\"test-proj-uuid\"]"}`), 0644)

	res, err := ImportClaudeCodeSession(fakeJSONL, antigravityBase, appStorage, "test-proj-uuid")
	if err != nil {
		t.Fatalf("ImportClaudeCodeSession failed: %v", err)
	}
	if !res.Success {
		t.Errorf("Expected import success, got failure: %s", res.Error)
	}
	if res.StepCount != 4 {
		t.Errorf("Expected step count 4, got %d", res.StepCount)
	}

	// Verify conversation DB exists
	sessionDB := filepath.Join(antigravityBase, "conversations", res.ConversationID+".db")
	if _, err := os.Stat(sessionDB); err != nil {
		t.Errorf("Expected session DB to exist at %s", sessionDB)
	}

	// Verify summaries DB exists
	sumDB := filepath.Join(antigravityBase, "conversation_summaries.db")
	if _, err := os.Stat(sumDB); err != nil {
		t.Errorf("Expected conversation_summaries.db to exist at %s", sumDB)
	}
}
