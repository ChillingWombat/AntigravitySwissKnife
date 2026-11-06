package webgui

import (
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

func newLanguageServerProxyStore(t *testing.T, active bool) *custommodels.Store {
	t.Helper()
	store, err := custommodels.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new custom models store: %v", err)
	}
	if active {
		settings := custommodels.DefaultIntegrationSettings()
		settings.Mode = custommodels.IntegrationAPIProxy
		if err := store.SetIntegrationSettings(settings); err != nil {
			t.Fatalf("activate proxy mode: %v", err)
		}
	}
	return store
}

func disableTestRuntimeDetection(t *testing.T) {
	t.Helper()
	for _, key := range []string{"ANTIGRAVITY_TEST_MODE", "ANTIGRAVITY_TESTING", "ANTIGRAVITY_TEST_DRY_RUN"} {
		t.Setenv(key, "")
	}
	originalArgs := os.Args
	originalCommandLine := flag.CommandLine
	t.Cleanup(func() {
		flag.CommandLine = originalCommandLine
		os.Args = originalArgs
	})
	os.Args = []string{"swiss-webgui"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
}

func TestServerStopClosesSyncURLStopOnce(t *testing.T) {
	srv := &Server{syncLSURLStop: make(chan struct{})}
	stopCh := srv.syncLSURLStop

	if err := srv.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	select {
	case <-stopCh:
	default:
		t.Fatal("expected stopCh to be closed")
	}
	srv.syncLSURLMu.Lock()
	cleared := srv.syncLSURLStop == nil
	srv.syncLSURLMu.Unlock()
	if !cleared {
		t.Fatal("Stop must clear syncLSURLStop")
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func TestServerStopConcurrentCallsCloseSyncURLStopOnce(t *testing.T) {
	const callers = 8
	srv := &Server{syncLSURLStop: make(chan struct{})}
	stopCh := srv.syncLSURLStop
	start := make(chan struct{})
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- srv.Stop()
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Stop: %v", err)
		}
	}
	select {
	case <-stopCh:
	default:
		t.Fatal("expected stopCh to be closed")
	}
}

func TestServerStopRestoresNativeWhenProxyModeActive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := newLanguageServerProxyStore(t, true)
	restoreCalls := 0
	srv := &Server{
		customModelsStore: store,
		syncLSURLStop:     make(chan struct{}),
		restoreNativeDeps: restoreNativeDeps{
			syncURL: func(string) error { return nil },
			restartApps: func() error {
				restoreCalls++
				return nil
			},
		},
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if restoreCalls != 1 {
		t.Fatalf("restore calls = %d, want 1", restoreCalls)
	}
}

func TestServerIsProxyModeActiveUsesStoreOnly(t *testing.T) {
	tests := []struct {
		name  string
		store *custommodels.Store
		want  bool
	}{
		{name: "nil store", want: false},
		{name: "inactive store", store: newLanguageServerProxyStore(t, false), want: false},
		{name: "active store", store: newLanguageServerProxyStore(t, true), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &Server{
				customModelsStore: tt.store,
				syncLSURLStop:     make(chan struct{}),
			}
			if got := srv.isProxyModeActive(); got != tt.want {
				t.Fatalf("isProxyModeActive() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestSyncLanguageServerCloudCodeURLGatedByProxyMode(t *testing.T) {
	originalSyncer := languageServerURLSyncer
	t.Cleanup(func() {
		languageServerURLSyncer = originalSyncer
	})
	disableTestRuntimeDetection(t)

	tests := []struct {
		name     string
		store    *custommodels.Store
		wantURLs []string
	}{
		{name: "inactive store", store: newLanguageServerProxyStore(t, false)},
		{name: "active store", store: newLanguageServerProxyStore(t, true), wantURLs: []string{"http://127.0.0.1:33721"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			languageServerURLSyncer = func(target string) error {
				calls = append(calls, target)
				return nil
			}
			srv := &Server{
				addr:              "127.0.0.1:33721",
				customModelsStore: tt.store,
				syncLSURLStop:     make(chan struct{}),
			}

			srv.syncLanguageServerCloudCodeURL()

			if len(calls) != len(tt.wantURLs) {
				t.Fatalf("sync calls = %v, want %v", calls, tt.wantURLs)
			}
			for i, want := range tt.wantURLs {
				if calls[i] != want {
					t.Fatalf("sync call %d = %q, want %q", i, calls[i], want)
				}
			}
		})
	}
}

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

	srv := newTestServer("127.0.0.1:0", "")
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
	srv := newTestServer("127.0.0.1:0", "")
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

func TestHandleCustomModelRestoreNativeIgnoresMissingLanguageServers(t *testing.T) {
	srv := newTestServer("127.0.0.1:0", "")
	restartCalls := 0
	srv.restoreNativeDeps = restoreNativeDeps{
		syncURL: func(string) error { return quota.ErrNoLanguageServers },
		restartApps: func() error {
			restartCalls++
			return nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/custom_models/restore_native", nil)
	rr := httptest.NewRecorder()
	srv.handleCustomModelRestoreNative(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"restored_native":true`) {
		t.Errorf("expected restored_native true, got: %s", rr.Body.String())
	}
	if restartCalls != 1 {
		t.Fatalf("restart calls = %d, want 1", restartCalls)
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
			srv := newTestServer("127.0.0.1:0", "")
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
