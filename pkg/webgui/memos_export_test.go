package webgui

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newExportTestServer wires an isolated Server plus a temporary export folder,
// so tests never write to the user's real Downloads directory.
func newExportTestServer(t *testing.T, memos []MemoItem) (*Server, string) {
	t.Helper()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())

	memoPath := filepath.Join(t.TempDir(), "memos.json")
	if err := writeMemosFile(memoPath, memos); err != nil {
		t.Fatalf("writeMemosFile: %v", err)
	}
	srv := &Server{}
	srv.SetGlobalMemosPath(memoPath)

	dir := t.TempDir()
	oldResolver := memoExportDirResolver
	memoExportDirResolver = func() (string, error) { return dir, nil }
	t.Cleanup(func() { memoExportDirResolver = oldResolver })
	return srv, dir
}

func postExport(t *testing.T, srv *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/memos/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.handleMemosExport(rec, req)
	return rec
}

func decodeExportResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return payload
}

// exportedPath asserts a successful response and returns its absolute path.
func exportedPath(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	payload := decodeExportResponse(t, rec)
	if payload["success"] != true {
		t.Fatalf("expected success=true, got %v", payload)
	}
	path, ok := payload["path"].(string)
	if !ok || path == "" {
		t.Fatalf("path missing from response %v", payload)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("path %q is not absolute", path)
	}
	return path
}

func b64(raw []byte) string { return base64.StdEncoding.EncodeToString(raw) }

func TestMemosExportAudioWebm(t *testing.T) {
	audio := append([]byte{0x1A, 0x45, 0xDF, 0xA3}, []byte("webm-payload")...)
	memo := MemoItem{ID: "memo-audio-1", Title: "Morning idea", Type: "audio", AudioData: b64(audio)}
	srv, dir := newExportTestServer(t, []MemoItem{memo})

	path := exportedPath(t, postExport(t, srv, `{"memo_id":"memo-audio-1"}`))
	if filepath.Ext(path) != ".webm" {
		t.Fatalf("extension = %q, want .webm (path %q)", filepath.Ext(path), path)
	}
	if filepath.Dir(path) != filepath.Clean(dir) {
		t.Errorf("file written to %q, want %q", filepath.Dir(path), filepath.Clean(dir))
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if string(written) != string(audio) {
		t.Errorf("exported bytes = %q, want %q", written, audio)
	}
}

func TestMemosExportAudioDataURLPrefix(t *testing.T) {
	audio := []byte("OggS-ogg-payload")
	memo := MemoItem{ID: "memo-audio-2", Title: "Voice note", Type: "audio", AudioData: "data:audio/ogg;base64," + b64(audio)}
	srv, _ := newExportTestServer(t, []MemoItem{memo})

	path := exportedPath(t, postExport(t, srv, `{"memo_id":"memo-audio-2"}`))
	if filepath.Ext(path) != ".ogg" {
		t.Fatalf("extension = %q, want .ogg", filepath.Ext(path))
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if string(written) != string(audio) {
		t.Errorf("exported bytes = %q, want %q", written, audio)
	}
}

func TestMemosExportImageDetectsExtensionFromMagicBytes(t *testing.T) {
	memo := MemoItem{ID: "memo-img-1", Title: "Whiteboard", Type: "image", ImageData: extractTestPNG}
	srv, _ := newExportTestServer(t, []MemoItem{memo})

	path := exportedPath(t, postExport(t, srv, `{"memo_id":"memo-img-1"}`))
	if filepath.Ext(path) != ".png" {
		t.Fatalf("extension = %q, want .png", filepath.Ext(path))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if info.Size() == 0 {
		t.Error("exported image is empty")
	}
}

func TestMemosExportTextWritesContentAndSanitisesTitle(t *testing.T) {
	content := "line one\nline two with unicode ✔\n"
	memo := MemoItem{ID: "memo-text-1", Title: `Plan: Q4/Q1 "draft"`, Type: "text", Content: content}
	srv, _ := newExportTestServer(t, []MemoItem{memo})

	path := exportedPath(t, postExport(t, srv, `{"memo_id":"memo-text-1"}`))
	wantName := `Plan_ Q4_Q1 _draft_.md`
	if filepath.Base(path) != wantName {
		t.Fatalf("filename = %q, want %q", filepath.Base(path), wantName)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if string(written) != content {
		t.Errorf("exported text = %q, want %q", written, content)
	}
}

func TestMemosExportEmptyTitleFallsBackToID(t *testing.T) {
	memo := MemoItem{ID: "memo-text-2", Title: "  ...  ", Type: "text", Content: "body"}
	srv, _ := newExportTestServer(t, []MemoItem{memo})

	path := exportedPath(t, postExport(t, srv, `{"memo_id":"memo-text-2"}`))
	if filepath.Base(path) != "memo-memo-text-2.md" {
		t.Fatalf("filename = %q, want memo-memo-text-2.md", filepath.Base(path))
	}
}

func TestMemosExportNeverOverwrites(t *testing.T) {
	memo := MemoItem{ID: "memo-text-3", Title: "Same title", Type: "text", Content: "first"}
	srv, _ := newExportTestServer(t, []MemoItem{memo})

	first := exportedPath(t, postExport(t, srv, `{"memo_id":"memo-text-3"}`))
	second := exportedPath(t, postExport(t, srv, `{"memo_id":"memo-text-3"}`))
	third := exportedPath(t, postExport(t, srv, `{"memo_id":"memo-text-3"}`))

	if filepath.Base(second) != "Same title (2).md" {
		t.Errorf("second filename = %q, want %q", filepath.Base(second), "Same title (2).md")
	}
	if filepath.Base(third) != "Same title (3).md" {
		t.Errorf("third filename = %q, want %q", filepath.Base(third), "Same title (3).md")
	}
	for _, p := range []string{first, second, third} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("export %q missing: %v", p, err)
		}
	}
}

func TestMemosExportUnknownMemoID(t *testing.T) {
	srv, _ := newExportTestServer(t, nil)

	rec := postExport(t, srv, `{"memo_id":"does-not-exist"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	payload := decodeExportResponse(t, rec)
	if payload["success"] != false {
		t.Errorf("expected success=false, got %v", payload)
	}
	if msg, _ := payload["error"].(string); !strings.Contains(msg, "not found") {
		t.Errorf("error = %q, want a not-found message", msg)
	}
}

func TestMemosExportUnknownType(t *testing.T) {
	memo := MemoItem{ID: "memo-odd-1", Title: "Odd", Type: "video"}
	srv, _ := newExportTestServer(t, []MemoItem{memo})

	rec := postExport(t, srv, `{"memo_id":"memo-odd-1"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if payload := decodeExportResponse(t, rec); payload["success"] != false {
		t.Errorf("expected success=false, got %v", payload)
	}
}

func TestMemosExportUnwritableDestination(t *testing.T) {
	memo := MemoItem{ID: "memo-text-4", Title: "Nope", Type: "text", Content: "body"}
	srv, _ := newExportTestServer(t, []MemoItem{memo})

	// A directory under a regular file cannot be created on any platform,
	// which simulates an unwritable destination without relying on chmod.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	oldResolver := memoExportDirResolver
	memoExportDirResolver = func() (string, error) { return filepath.Join(blocker, "nested"), nil }
	t.Cleanup(func() { memoExportDirResolver = oldResolver })

	rec := postExport(t, srv, `{"memo_id":"memo-text-4"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	payload := decodeExportResponse(t, rec)
	if payload["success"] != false {
		t.Errorf("expected success=false, got %v", payload)
	}
	if msg, _ := payload["error"].(string); strings.TrimSpace(msg) == "" {
		t.Error("expected a non-empty error message")
	}
}

func TestMemosExportMethodNotAllowed(t *testing.T) {
	srv, _ := newExportTestServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/memos/export", nil)
	rec := httptest.NewRecorder()
	srv.handleMemosExport(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// Memos recorded before the naming change carry a "[Voice] " tag in their
// stored title; exported files must use the clean name the UI shows.
func TestMemosExportStripsLegacyVoicePrefix(t *testing.T) {
	memo := MemoItem{ID: "memo-audio-legacy", Title: "[Voice] Voice Memo Note", Type: "audio", AudioData: b64(append([]byte{0x1A, 0x45, 0xDF, 0xA3}, []byte("legacy")...))}
	srv, _ := newExportTestServer(t, []MemoItem{memo})

	path := exportedPath(t, postExport(t, srv, `{"memo_id":"memo-audio-legacy"}`))
	if filepath.Base(path) != "Voice Memo Note.webm" {
		t.Fatalf("filename = %q, want Voice Memo Note.webm", filepath.Base(path))
	}
}
