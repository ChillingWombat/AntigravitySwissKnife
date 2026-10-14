package keyring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestSyncAllSurfaces_TransactionalRollbackOnFailure(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	antigravityConfigDir := filepath.Join(tmpHome, ".config", "Antigravity")
	geminiDir := filepath.Join(tmpHome, ".gemini")
	geminiAntigravityDir := filepath.Join(geminiDir, "antigravity")

	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityConfigDir)
	t.Setenv("ANTIGRAVITY_DIR", geminiAntigravityDir)

	_ = os.MkdirAll(antigravityConfigDir, 0755)
	_ = os.MkdirAll(geminiDir, 0755)
	_ = os.MkdirAll(geminiAntigravityDir, 0755)

	// Pre-seed initial state for Account 1
	standaloneTokenPath := filepath.Join(geminiDir, "jetski-standalone-oauth-token")
	initialStandaloneContent := []byte(`{"token":{"access_token":"initial_token_acc1"}}`)
	if err := os.WriteFile(standaloneTokenPath, initialStandaloneContent, 0600); err != nil {
		t.Fatalf("failed to write standalone token: %v", err)
	}

	appStoragePath := filepath.Join(antigravityConfigDir, "app_storage.json")
	initialAppStorageContent := []byte("{\n  \"jetski.onboarding.lastLoginUsername\": \"acc1@enterprise.org\"\n}")
	if err := os.WriteFile(appStoragePath, initialAppStorageContent, 0644); err != nil {
		t.Fatalf("failed to write app_storage: %v", err)
	}

	// Inject error at state.vscdb
	syncStepHookForTest = func(step string) error {
		if step == "state.vscdb" {
			return fmt.Errorf("injected error: state.vscdb sqlite database locked by IDE process")
		}
		return nil
	}
	defer func() {
		syncStepHookForTest = nil
	}()

	acc2 := &Account{
		Email:        "acc2@enterprise.org",
		AccessToken:  "ya29.acc2_new_access_token",
		RefreshToken: "1//acc2_new_refresh_token",
	}

	err := SyncAllSurfaces(acc2, []string{"acc1@enterprise.org", "acc2@enterprise.org"}, nil)
	if err == nil {
		t.Fatalf("expected SyncAllSurfaces to fail due to injected state.vscdb lock")
	}
	if !strings.Contains(err.Error(), "state.vscdb") {
		t.Fatalf("expected error to mention state.vscdb, got: %v", err)
	}

	// Verify rollback restored all pre-existing files to their initial state
	restoredStandalone, err := os.ReadFile(standaloneTokenPath)
	if err != nil {
		t.Fatalf("failed to read standalone token after rollback: %v", err)
	}
	if string(restoredStandalone) != string(initialStandaloneContent) {
		t.Errorf("expected standalone token to be restored to %s, got %s", string(initialStandaloneContent), string(restoredStandalone))
	}

	restoredAppStorage, err := os.ReadFile(appStoragePath)
	if err != nil {
		t.Fatalf("failed to read app_storage after rollback: %v", err)
	}
	if !strings.Contains(string(restoredAppStorage), "acc1@enterprise.org") {
		t.Errorf("expected app_storage to be restored to acc1, got: %s", string(restoredAppStorage))
	}
}

func TestSyncAllSurfaces_CleansUpNewlyCreatedFilesOnFailure(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	antigravityConfigDir := filepath.Join(tmpHome, ".config", "Antigravity")
	geminiDir := filepath.Join(tmpHome, ".gemini")
	geminiAntigravityDir := filepath.Join(geminiDir, "antigravity")

	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityConfigDir)
	t.Setenv("ANTIGRAVITY_DIR", geminiAntigravityDir)

	_ = os.MkdirAll(antigravityConfigDir, 0755)
	_ = os.MkdirAll(geminiDir, 0755)
	_ = os.MkdirAll(geminiAntigravityDir, 0755)

	standaloneTokenPath := filepath.Join(geminiDir, "jetski-standalone-oauth-token")
	_ = os.Remove(standaloneTokenPath)

	// Inject error at app_storage (right after standalone token was written)
	syncStepHookForTest = func(step string) error {
		if step == "app_storage" {
			return fmt.Errorf("injected app_storage failure")
		}
		return nil
	}
	defer func() {
		syncStepHookForTest = nil
	}()

	acc := &Account{
		Email:        "cleanup_test@enterprise.org",
		AccessToken:  "ya29.cleanup_access_token",
		RefreshToken: "1//cleanup_refresh_token",
	}

	err := SyncAllSurfaces(acc, []string{"cleanup_test@enterprise.org"}, nil)
	if err == nil {
		t.Fatalf("expected SyncAllSurfaces to fail")
	}

	// Verify rollback deleted the standalone token file that was created during the failed sync
	if _, err := os.Stat(standaloneTokenPath); !os.IsNotExist(err) {
		t.Errorf("expected newly created standalone token to be removed by rollback, but file still exists")
	}
}

// TestSyncJournal_ReadOnlyRollback tests that sync journal rollback succeeds when restoring pre-existing
// read-only files (e.g., mode 0444/0400) on all operating systems including Windows.
//
// Beginner Explanation:
// In operating systems like Windows, if a file has the "read-only" attribute set, standard file replacement
// via MoveFileExW / os.Rename fails with "Access is denied" unless the read-only attribute is temporarily cleared.
// During an account sync operation, if an unexpected error occurs (such as disk full or network error),
// the sync journal rolls back all changes to restore the user's previous configuration.
// If the user's previous configuration file was read-only, our rollback mechanism must be able to overwrite it
// and restore the original content and permissions without failing.
func TestSyncJournal_ReadOnlyRollback(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "readonly.json")
	origContent := []byte(`{"key":"original_value"}`)

	// Create file with read-only permission (0444)
	if err := os.WriteFile(targetPath, origContent, 0444); err != nil {
		t.Fatalf("failed to write initial read-only file: %v", err)
	}

	journal := newSyncJournal()
	if err := journal.RecordFile(targetPath); err != nil {
		t.Fatalf("failed to record file in journal: %v", err)
	}

	// Modify the file to simulate partial sync changes.
	// On Windows, to overwrite a 0444 file we must make it writable first:
	if runtime.GOOS == "windows" {
		_ = os.Chmod(targetPath, 0666)
	}
	modifiedContent := []byte(`{"key":"modified_value"}`)
	if err := os.WriteFile(targetPath, modifiedContent, 0444); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	// Rollback must succeed without returning "Access is denied"
	if err := journal.Rollback(); err != nil {
		t.Fatalf("Rollback failed on read-only file: %v", err)
	}

	// Verify original content was restored
	restored, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(restored) != string(origContent) {
		t.Fatalf("content not restored: expected %s, got %s", origContent, restored)
	}

	// Also test rollback when a newly created file was marked read-only before rollback
	newReadOnlyFile := filepath.Join(tmpDir, "new_readonly.json")
	journal2 := newSyncJournal()
	if err := journal2.RecordFile(newReadOnlyFile); err != nil {
		t.Fatalf("failed to record non-existent file in journal: %v", err)
	}

	// Create new file as read-only
	if err := os.WriteFile(newReadOnlyFile, []byte(`{"created":"new"}`), 0444); err != nil {
		t.Fatalf("failed to create new read-only file: %v", err)
	}

	// Rollback should delete the newly created read-only file cleanly
	if err := journal2.Rollback(); err != nil {
		t.Fatalf("Rollback failed to delete newly created read-only file: %v", err)
	}

	if _, err := os.Stat(newReadOnlyFile); !os.IsNotExist(err) {
		t.Fatalf("expected newly created read-only file to be deleted by rollback, but it still exists")
	}
}

