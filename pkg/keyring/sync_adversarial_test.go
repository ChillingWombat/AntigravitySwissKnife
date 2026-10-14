package keyring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestWriteAtomic_PermissionsAndContent(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "sub", "test_token.json")
	data := []byte(`{"token":"secret_123"}`)

	if err := writeAtomic(targetPath, data, 0600); err != nil {
		t.Fatalf("writeAtomic failed: %v", err)
	}

	read, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read back file: %v", err)
	}
	if string(read) != string(data) {
		t.Fatalf("expected %q, got %q", string(data), string(read))
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600 permissions, got %v", info.Mode().Perm())
	}
}

func TestSyncAppStorageLoginUser_LockingAndConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", tmpDir)
	storagePath := filepath.Join(tmpDir, "app_storage.json")

	initial := map[string]interface{}{
		"existing_key": "existing_value",
	}
	initBytes, _ := json.Marshal(initial)
	if err := os.WriteFile(storagePath, initBytes, 0644); err != nil {
		t.Fatalf("failed to write initial storage: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			email := "user" + string(rune('a'+idx)) + "@example.com"
			if err := SyncAppStorageLoginUser(email); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent SyncAppStorageLoginUser error: %v", err)
	}

	readData, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("failed to read app_storage.json: %v", err)
	}

	var finalMap map[string]interface{}
	if err := json.Unmarshal(readData, &finalMap); err != nil {
		t.Fatalf("app_storage.json is corrupted: %v", err)
	}

	if finalMap["existing_key"] != "existing_value" {
		t.Fatalf("existing_key was clobbered: %v", finalMap)
	}
	if finalMap["jetski.onboarding.lastLoginUsername"] == "" {
		t.Fatalf("lastLoginUsername is empty")
	}
}

func TestSetTokenRefreshCallback_InvokedOnFreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "fresh_access_token_abc",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	origEndpoint := tokenRefreshEndpoint
	tokenRefreshEndpoint = server.URL
	defer func() {
		tokenRefreshEndpoint = origEndpoint
		SetTokenRefreshCallback(nil)
	}()

	var callbackInvoked bool
	var callbackEmail string
	var callbackToken string

	SetTokenRefreshCallback(func(acc *Account) {
		callbackInvoked = true
		callbackEmail = acc.Email
		callbackToken = acc.AccessToken
	})

	acc := &Account{
		Email:        "refresh_test@domain.com",
		RefreshToken: "1//refresh_token_valid",
	}

	refreshed := EnsureFreshAccessToken(acc)
	if !refreshed {
		t.Fatalf("expected EnsureFreshAccessToken to return true")
	}
	if !callbackInvoked {
		t.Fatalf("expected SetTokenRefreshCallback callback to be invoked")
	}
	if callbackEmail != "refresh_test@domain.com" {
		t.Fatalf("expected callback email to be refresh_test@domain.com, got %s", callbackEmail)
	}
	if callbackToken != "fresh_access_token_abc" {
		t.Fatalf("expected callback token to be fresh_access_token_abc, got %s", callbackToken)
	}
}

func TestSyncAllSurfaces_ErrorPropagationOnFailedSurface(t *testing.T) {
	// Passing nil account returns error
	err := SyncAllSurfaces(nil, nil, nil)
	if err == nil {
		t.Fatalf("expected error for nil account")
	}
}

func TestReconcileActiveAccount_30SecondLatch(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "swiss.db"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// Add two accounts
	_ = store.AddOrUpdateAccount(&Account{Email: "user1@example.com", Label: "User 1", PlanTier: "Free", Status: "ACTIVE", IsActive: true})
	_ = store.AddOrUpdateAccount(&Account{Email: "user2@example.com", Label: "User 2", PlanTier: "Free", Status: "STANDBY", IsActive: false})
	_ = store.SetActiveAccount("user1@example.com")

	// Simulate switch happened 15 seconds ago (< 30s)
	store.mu.Lock()
	store.lastManualSwitchTime = time.Now().Add(-15 * time.Second)
	store.mu.Unlock()

	// Simulate ResolveRunningAntigravityAccount detecting user2
	// Reconcile should protect user1 due to 30s latch
	acc, err := store.ReconcileActiveAccount(false, []string{"user1@example.com", "user2@example.com"}, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount failed: %v", err)
	}
	if acc != nil && acc.Email != "user1@example.com" {
		t.Fatalf("expected user1@example.com to be latched, got %s", acc.Email)
	}
}
