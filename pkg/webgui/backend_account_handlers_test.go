package webgui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

type accountState struct {
	accounts []*keyring.Account
}

type accountStateService struct {
	backend.AccountService
	state *accountState
}

func (s accountStateService) ListAccounts() []*keyring.Account {
	return s.state.accounts
}

func (s accountStateService) GetAccount(email string) (*keyring.Account, error) {
	for _, account := range s.state.accounts {
		if strings.EqualFold(account.Email, email) {
			return account, nil
		}
	}
	return nil, core.ErrAccountNotFound
}

func (s accountStateService) SetTOTPSecret(email, secret string) error {
	account, err := s.GetAccount(email)
	if err != nil {
		return err
	}
	account.TOTPSecret = secret
	account.HasTOTP = secret != ""
	return nil
}

type recordingRPC struct {
	calls int
}

func (r *recordingRPC) Call(string, interface{}, interface{}) error {
	r.calls++
	return errors.New("unexpected RPC call")
}

type failingRPC struct {
	err error
}

func (r failingRPC) Call(string, interface{}, interface{}) error {
	return r.err
}

func TestAccountHandlersUseBackendOwnedState(t *testing.T) {
	state := &accountState{accounts: []*keyring.Account{
		{Email: "primary@example.com", IsActive: true},
		{Email: "secondary@example.com"},
	}}
	rpc := &recordingRPC{}
	server := &Server{
		backend:       testBackend{accounts: accountStateService{state: state}},
		client:        rpc,
		sessionTokens: map[string]time.Time{"test-token": time.Now().Add(time.Hour)},
	}

	totpBody, _ := json.Marshal(map[string]string{"email": "primary@example.com", "secret": "JBSWY3DPEHPK3PXP"})
	req := httptest.NewRequest(http.MethodPost, "/api/totp", bytes.NewReader(totpBody))
	req.Header.Set("X-Session-Token", "test-token")
	rec := httptest.NewRecorder()
	server.handleTOTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/totp status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/totp?email=primary@example.com", nil)
	req.Header.Set("X-Session-Token", "test-token")
	rec = httptest.NewRecorder()
	server.handleTOTP(rec, req)
	var totp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&totp); err != nil {
		t.Fatalf("decode TOTP response: %v", err)
	}
	if totp["has_totp"] != true || len(totp["code"].(string)) != 6 {
		t.Fatalf("TOTP response = %+v, want backend-owned six-digit code", totp)
	}

	state.accounts[0].IsActive = false
	state.accounts[1].IsActive = true
	rec = httptest.NewRecorder()
	server.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	var status map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if status["active_account"] != "secondary@example.com" || status["total_accounts"] != float64(2) {
		t.Fatalf("status response = %+v, want backend-owned active account", status)
	}
	if rpc.calls != 0 {
		t.Fatalf("account handlers made %d RPC calls with an account backend", rpc.calls)
	}
}

func TestAccountHandlersRejectFailedRPCWithoutLocalMutation(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", configDir)
	t.Setenv("HOME", configDir)
	t.Setenv("USERPROFILE", configDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	store, err := keyring.NewStore("")
	if err != nil {
		t.Fatalf("keyring.NewStore: %v", err)
	}
	const originalSecret = "JBSWY3DPEHPK3PXP"
	if err := store.AddOrUpdateAccount(&keyring.Account{
		Email:      "local@example.com",
		TOTPSecret: originalSecret,
		HasTOTP:    true,
	}); err != nil {
		t.Fatalf("seed local account: %v", err)
	}

	server := &Server{
		client:        failingRPC{err: errors.New("daemon unavailable")},
		sessionTokens: map[string]time.Time{"test-token": time.Now().Add(time.Hour)},
	}
	if _, err := server.loadStatus(); err == nil {
		t.Fatal("loadStatus accepted a failed RPC")
	} else {
		var typed *accountBackendError
		if !errors.As(err, &typed) {
			t.Fatalf("loadStatus error type = %T, want *accountBackendError", err)
		}
	}

	rec := httptest.NewRecorder()
	server.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("GET /api/status status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/totp?email=local@example.com", nil)
	req.Header.Set("X-Session-Token", "test-token")
	rec = httptest.NewRecorder()
	server.handleTOTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("GET /api/totp status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	body, _ := json.Marshal(map[string]string{"email": "local@example.com", "secret": ""})
	req = httptest.NewRequest(http.MethodPost, "/api/totp", bytes.NewReader(body))
	req.Header.Set("X-Session-Token", "test-token")
	rec = httptest.NewRecorder()
	server.handleTOTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("POST /api/totp status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	reopened, err := keyring.NewStore("")
	if err != nil {
		t.Fatalf("reopen keyring store: %v", err)
	}
	account, err := reopened.GetAccount("local@example.com")
	if err != nil {
		fmt.Printf("GetAccount failed: %v\n", err)
		t.Fatalf("read local account: %v", err)
	}
	if account.TOTPSecret != originalSecret {
		t.Fatalf("local TOTP secret = %q, want unchanged %q", account.TOTPSecret, originalSecret)
	}
}
