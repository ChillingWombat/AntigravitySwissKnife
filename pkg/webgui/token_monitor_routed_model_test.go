package webgui

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func appendRoutedModelTestVarint(dst []byte, value uint64) []byte {
	for value >= 0x80 {
		dst = append(dst, byte(value)|0x80)
		value >>= 7
	}
	return append(dst, byte(value))
}

func routedModelBytesField(number int, payload []byte) []byte {
	out := appendRoutedModelTestVarint(nil, uint64(number<<3|2))
	out = appendRoutedModelTestVarint(out, uint64(len(payload)))
	return append(out, payload...)
}

func routedModelMetadataKV(key, value string) []byte {
	out := routedModelBytesField(1, []byte(key))
	return append(out, routedModelBytesField(2, []byte(value))...)
}

func routedModelMetadataBlob(route, lastStepIndex string) []byte {
	generation := routedModelBytesField(19, []byte(route))
	generation = append(generation, routedModelBytesField(20, routedModelMetadataKV("last_step_index", lastStepIndex))...)
	return routedModelBytesField(1, generation)
}

func TestReadRoutedModelByStep(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "conversation.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE gen_metadata (idx INTEGER PRIMARY KEY, data BLOB NOT NULL)`); err != nil {
		t.Fatalf("create gen_metadata: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO gen_metadata (idx, data) VALUES (0, ?), (1, ?)`,
		routedModelMetadataBlob("gemini-3.8-flash-tiered", "0"),
		routedModelMetadataBlob("claude-sonnet-4-6", "6"),
	); err != nil {
		t.Fatalf("insert gen_metadata: %v", err)
	}

	routes := readRoutedModelByStep(dbPath)
	if got := routes[1]; got != "gemini-3.8-flash-tiered" {
		t.Fatalf("route for response step 1 = %q, want gemini-3.8-flash-tiered", got)
	}
	if got := routes[7]; got != "claude-sonnet-4-6" {
		t.Fatalf("route for response step 7 = %q, want claude-sonnet-4-6", got)
	}
}
