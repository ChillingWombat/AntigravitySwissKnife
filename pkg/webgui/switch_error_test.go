package webgui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

type accountNotFoundSwitchService struct {
	backend.AccountService
	called bool
}

func (s *accountNotFoundSwitchService) ListAccounts() []*keyring.Account {
	return nil
}

func (s *accountNotFoundSwitchService) SwitchAccount(backend.AccountSwitch) (backend.AccountSwitchResult, error) {
	s.called = true
	return backend.AccountSwitchResult{}, core.ErrAccountNotFound
}

func TestHandleSwitchMapsBackendAccountNotFound(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	service := &accountNotFoundSwitchService{}
	server := &Server{backend: testBackend{accounts: service}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/switch", bytes.NewBufferString(`{"email":"missing@example.com"}`))

	server.handleSwitch(rec, req)

	if !service.called {
		t.Fatal("expected injected account backend to handle switch")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("switch status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
