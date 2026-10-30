package webgui

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func appendProtoVarint(dst []byte, value uint64) []byte {
	for value >= 0x80 {
		dst = append(dst, byte(value)|0x80)
		value >>= 7
	}
	return append(dst, byte(value))
}

func protoVarintField(number int, value uint64) []byte {
	out := appendProtoVarint(nil, uint64(number<<3))
	return appendProtoVarint(out, value)
}

func protoBytesField(number int, data []byte) []byte {
	out := appendProtoVarint(nil, uint64(number<<3|2))
	out = appendProtoVarint(out, uint64(len(data)))
	return append(out, data...)
}

func testHostContextWindowBlob(usedTokens, maxTokens uint64) []byte {
	contextWindow := protoVarintField(1, usedTokens)
	contextWindow = append(contextWindow, protoVarintField(4, maxTokens)...)
	chatStart := protoBytesField(10, contextWindow)
	chatModel := protoBytesField(9, chatStart)
	return protoBytesField(1, chatModel)
}

func TestParseHostContextWindowMetadata(t *testing.T) {
	usedTokens, maxTokens := uint64(49545), uint64(256000)
	cw, ok := parseHostContextWindowMetadata(testHostContextWindowBlob(usedTokens, maxTokens))
	if !ok {
		t.Fatal("expected host context window metadata")
	}
	if cw.UsedTokens != int64(usedTokens) || cw.MaxTokens != int64(maxTokens) {
		t.Fatalf("unexpected context window: %+v", cw)
	}
}

func TestReadHostContextWindowUsesNewestMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "conv.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE gen_metadata (idx integer PRIMARY KEY, data blob, size integer NOT NULL DEFAULT 0)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO gen_metadata (idx, data) VALUES (?, ?)", 0, testHostContextWindowBlob(1000, 128000)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO gen_metadata (idx, data) VALUES (?, ?)", 1, testHostContextWindowBlob(49545, 256000)); err != nil {
		t.Fatal(err)
	}

	cw, ok := readHostContextWindow(tmpDir, "conv")
	if !ok {
		t.Fatal("expected host context window metadata")
	}
	if cw.UsedTokens != 49545 || cw.MaxTokens != 256000 {
		t.Fatalf("expected newest metadata, got %+v", cw)
	}
}

func TestGetChatMetricsForConversation_HostContextWindow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	convID := "test-host-window"
	brainLogDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(brainLogDir, 0755); err != nil {
		t.Fatal(err)
	}
	transcript := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-30T10:00:00Z","content":"hello"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-30T10:00:01Z","input_tokens":1000,"output_tokens":200}
`
	if err := os.WriteFile(filepath.Join(brainLogDir, "transcript.jsonl"), []byte(transcript), 0644); err != nil {
		t.Fatal(err)
	}

	conversationsDir := filepath.Join(tmpDir, ".gemini", "antigravity", "conversations")
	if err := os.MkdirAll(conversationsDir, 0755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(conversationsDir, convID+".db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE gen_metadata (idx integer PRIMARY KEY, data blob, size integer NOT NULL DEFAULT 0)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO gen_metadata (idx, data) VALUES (?, ?)", 0, testHostContextWindowBlob(49545, 256000)); err != nil {
		t.Fatal(err)
	}

	res, err := GetChatMetricsForConversation(convID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if res.ContextWindow == nil {
		t.Fatal("expected host context window in response")
	}
	if res.ContextWindow.UsedTokens != 49545 || res.ContextWindow.MaxTokens != 256000 {
		t.Fatalf("unexpected context window: %+v", res.ContextWindow)
	}
	if res.ContextWindow.Source != "language_server" {
		t.Fatalf("unexpected source: %q", res.ContextWindow.Source)
	}
}
