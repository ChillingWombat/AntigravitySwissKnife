package webgui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newAccountsExportMethodHandler(t *testing.T) http.Handler {
	t.Helper()
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	return newTestServer("127.0.0.1:0", "").setupRoutes()
}

func TestAccountsExportAllowsGetAndHead(t *testing.T) {
	handler := newAccountsExportMethodHandler(t)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/accounts/export", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s status = %d, want %d; body = %s", method, rec.Code, http.StatusOK, rec.Body.String())
			}
		})
	}
}

func TestAccountsExportRejectsUnsupportedMethods(t *testing.T) {
	handler := newAccountsExportMethodHandler(t)

	for _, method := range []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/accounts/export", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
			}
			if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
				t.Fatalf("%s Allow header = %q, want %q", method, allow, "GET, HEAD")
			}
		})
	}
}

func TestAccountsExportPreservesOptionsPreflight(t *testing.T) {
	handler := newAccountsExportMethodHandler(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/accounts/export", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
