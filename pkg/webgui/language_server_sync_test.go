package webgui

import (
	"errors"
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
	srv.restoreNativeDeps = restoreNativeDeps{
		syncURL:     func(string) error { return nil },
		restartApps: func() error { return nil },
	}
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

func TestHandleCustomModelRestoreNativeFailures(t *testing.T) {
	syncFailure := errors.New("language server sync failed")
	restartFailure := errors.New("app restart failed")
	tests := []struct {
		name        string
		syncErr     error
		restartErr  error
		wantSync    int
		wantRestart int
	}{
		{name: "sync failure", syncErr: syncFailure, wantSync: 1, wantRestart: 1},
		{name: "restart failure", restartErr: restartFailure, wantSync: 1, wantRestart: 1},
		{name: "both failures", syncErr: syncFailure, restartErr: restartFailure, wantSync: 1, wantRestart: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer("127.0.0.1:0", "")
			syncCalls := 0
			restartCalls := 0
			srv.restoreNativeDeps = restoreNativeDeps{
				syncURL: func(target string) error {
					syncCalls++
					return tt.syncErr
				},
				restartApps: func() error {
					restartCalls++
					return tt.restartErr
				},
			}

			req := httptest.NewRequest(http.MethodPost, "/api/custom_models/restore_native", nil)
			rr := httptest.NewRecorder()
			srv.handleCustomModelRestoreNative(rr, req)

			if rr.Code != http.StatusInternalServerError {
				t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
			}
			if syncCalls != tt.wantSync || restartCalls != tt.wantRestart {
				t.Fatalf("calls sync=%d restart=%d, want sync=%d restart=%d", syncCalls, restartCalls, tt.wantSync, tt.wantRestart)
			}
			body := rr.Body.String()
			if !strings.Contains(body, `"success":false`) || !strings.Contains(body, `"restored_native":false`) {
				t.Fatalf("failure response claimed success: %s", body)
			}
			if tt.syncErr != nil && !strings.Contains(body, tt.syncErr.Error()) {
				t.Fatalf("response omitted sync failure: %s", body)
			}
			if tt.restartErr != nil && !strings.Contains(body, tt.restartErr.Error()) {
				t.Fatalf("response omitted restart failure: %s", body)
			}
		})
	}
}
