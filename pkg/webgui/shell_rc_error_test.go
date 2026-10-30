package webgui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleCustomModelRestoreNativeReportsMalformedShellRC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	rc := filepath.Join(home, ".zshrc")
	original := "before\n# >>> Antigravity Swiss Knife CLI Integration >>>\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n"
	if err := os.WriteFile(rc, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/custom-models/restore-native", nil)
	(&Server{}).handleCustomModelRestoreNative(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %q", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "failed to clean shell RC hooks") {
		t.Fatalf("body = %q, want shell RC cleanup error", recorder.Body.String())
	}
	data, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("malformed shell RC changed: %q", string(data))
	}
}
