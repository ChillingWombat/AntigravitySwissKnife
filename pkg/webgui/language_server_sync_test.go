package webgui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLanguageServerURLSyncDisabledDuringTests(t *testing.T) {
	originalSyncer := languageServerURLSyncer
	t.Cleanup(func() {
		languageServerURLSyncer = originalSyncer
	})

	var calls []string
	languageServerURLSyncer = func(target string) error {
		calls = append(calls, target)
		return nil
	}

	srv := NewServer("127.0.0.1:0", "")
	srv.addr = "127.0.0.1:33721"
	srv.syncLanguageServerCloudCodeURL()
	srv.startLanguageServerURLSync()
	defer srv.Stop()

	if len(calls) != 0 {
		t.Fatalf("test server must not redirect a live Antigravity language server, got calls: %v", calls)
	}
	if srv.syncLSURLStop != nil {
		t.Fatal("test server must not start the language server URL sync goroutine")
	}
}

func TestHandleCustomModelRestoreNative(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	req := httptest.NewRequest("POST", "/api/custom_models/restore_native", nil)
	rr := httptest.NewRecorder()
	srv.handleCustomModelRestoreNative(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"restored_native":true`) {
		t.Errorf("expected restored_native true, got: %s", rr.Body.String())
	}
}
