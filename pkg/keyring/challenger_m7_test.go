package keyring

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ============================================================================
// 1. P1-1 TOTP PRESERVATION ADVERSARIAL STRESS TEST
// ============================================================================

func TestChallenger_P1_1_TOTPPreservationStress(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	initialSecret := "JBSWY3DPEHPK3PXP"
	acc := &Account{
		Email:      "victim_totp@security.org",
		Label:      "Original Label",
		TOTPSecret: initialSecret,
		HasTOTP:    true,
	}
	if err := store.AddOrUpdateAccount(acc); err != nil {
		t.Fatalf("AddOrUpdateAccount error: %v", err)
	}

	// Matrix of hostile inputs that MUST preserve the existing secret
	preservingInputs := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"single space", " "},
		{"tab character", "\t"},
		{"multiple spaces", "    "},
		{"carriage return and newline", "\r\n"},
		{"mixed whitespace", " \t \r\n \v \f "},
		{"exact redaction marker", "[REDACTED]"},
		{"padded redaction marker", "  [REDACTED]  "},
		{"tab padded redaction", "\t[REDACTED]\t"},
	}

	for i, tc := range preservingInputs {
		t.Run(fmt.Sprintf("Preserve_%s", tc.name), func(t *testing.T) {
			label := fmt.Sprintf("Updated Label %d", i)
			err := store.UpdateAccountFull(
				acc.Email, label, "FREE", "STANDBY", "Mid", "Updated notes", "", tc.input, "", 0, false, false, false,
			)
			if err != nil {
				t.Fatalf("UpdateAccountFull failed on input %q: %v", tc.input, err)
			}

			loaded, err := store.GetAccount(acc.Email)
			if err != nil {
				t.Fatalf("GetAccount failed: %v", err)
			}
			if loaded.TOTPSecret != initialSecret {
				t.Fatalf("SILENT DESTRUCTION: TOTPSecret corrupted for input %q: expected %q, got %q", tc.input, initialSecret, loaded.TOTPSecret)
			}
			if !loaded.HasTOTP {
				t.Fatalf("SILENT DESTRUCTION: HasTOTP flipped to false on input %q", tc.input)
			}
		})
	}

	// Successive chained updates: 10 successive updates with random metadata and empty secret
	t.Run("SuccessiveChainedUpdates", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			err := store.UpdateAccountFull(
				acc.Email, fmt.Sprintf("Rapid Update %d", i), "PRO", "ACTIVE", "High", fmt.Sprintf("Note %d", i), "", "", "", 0, false, false, false,
			)
			if err != nil {
				t.Fatalf("rapid update %d failed: %v", i, err)
			}
			loaded, _ := store.GetAccount(acc.Email)
			if loaded.TOTPSecret != initialSecret || !loaded.HasTOTP {
				t.Fatalf("rapid update %d corrupted TOTP secret: got %q, hasTOTP=%v", i, loaded.TOTPSecret, loaded.HasTOTP)
			}
		}
	})

	// Intentional clear markers
	clearMarkers := []struct {
		name  string
		input string
	}{
		{"bracketed uppercase", "[CLEAR]"},
		{"bare uppercase", "CLEAR"},
		{"padded bracketed uppercase", "   [CLEAR]   "},
		{"padded bare uppercase", " \t CLEAR \n "},
	}

	for _, cm := range clearMarkers {
		t.Run(fmt.Sprintf("Clear_%s", cm.name), func(t *testing.T) {
			// Re-seed secret
			acc.TOTPSecret = initialSecret
			acc.HasTOTP = true
			if err := store.AddOrUpdateAccount(acc); err != nil {
				t.Fatal(err)
			}

			err := store.UpdateAccountFull(
				acc.Email, "Clear Label", "FREE", "STANDBY", "Mid", "Notes", "", cm.input, "", 0, false, false, false,
			)
			if err != nil {
				t.Fatalf("UpdateAccountFull failed on clear marker %q: %v", cm.input, err)
			}

			loaded, err := store.GetAccount(acc.Email)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.TOTPSecret != "" {
				t.Fatalf("expected TOTPSecret to be cleared by marker %q, got %q", cm.input, loaded.TOTPSecret)
			}
			if loaded.HasTOTP {
				t.Fatalf("expected HasTOTP to be false after clear marker %q", cm.input)
			}
		})
	}
}

// ============================================================================
// 2. P1-6 SYNC ALL SURFACES TRANSACTIONAL ATOMICITY & ROLLBACK STRESS TEST
// ============================================================================

func TestChallenger_P1_6_SyncAllSurfaces_AtomicityAcrossAllStages(t *testing.T) {
	stages := []string{
		"pre-sync",
		"desktop token",
		"app_storage",
		"hardware profile",
		"state.vscdb",
		"cli oauth token",
		"google_accounts.json",
		"oauth_creds.json",
		"cloud_accounts.db",
	}

	for _, stage := range stages {
		t.Run("FailureAt_"+strings.ReplaceAll(stage, " ", "_"), func(t *testing.T) {
			tmpHome := t.TempDir()
			t.Setenv("HOME", tmpHome)
			t.Setenv("USERPROFILE", tmpHome)

			antigravityConfigDir := filepath.Join(tmpHome, ".config", "Antigravity")
			geminiDir := filepath.Join(tmpHome, ".gemini")
			geminiAntigravityDir := filepath.Join(geminiDir, "antigravity")
			antigravityAgentDir := filepath.Join(tmpHome, ".antigravity-agent")
			cliDir := filepath.Join(geminiDir, "antigravity-cli")
			vscdbDir := filepath.Join(antigravityConfigDir, "User", "globalStorage")

			t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityConfigDir)
			t.Setenv("ANTIGRAVITY_DIR", geminiAntigravityDir)

			for _, d := range []string{antigravityConfigDir, geminiDir, geminiAntigravityDir, antigravityAgentDir, cliDir, vscdbDir} {
				if err := os.MkdirAll(d, 0755); err != nil {
					t.Fatalf("mkdir %s failed: %v", d, err)
				}
			}

			// Pre-existing file 1: jetski-standalone-oauth-token
			standaloneTokenPath := filepath.Join(geminiDir, "jetski-standalone-oauth-token")
			initialStandalone := []byte(`{"token":{"access_token":"ORIGINAL_STANDALONE_TOKEN_123"}}`)
			if err := os.WriteFile(standaloneTokenPath, initialStandalone, 0600); err != nil {
				t.Fatal(err)
			}

			// Pre-existing file 2: app_storage.json
			appStoragePath := filepath.Join(antigravityConfigDir, "app_storage.json")
			initialAppStorage := []byte(`{"jetski.onboarding.lastLoginUsername":"original_user@corp.org"}`)
			if err := os.WriteFile(appStoragePath, initialAppStorage, 0644); err != nil {
				t.Fatal(err)
			}

			// Pre-existing file 3: google_accounts.json
			googleAccountsPath := filepath.Join(geminiDir, "google_accounts.json")
			initialGoogleAccounts := []byte(`{"accounts":["original_user@corp.org"]}`)
			if err := os.WriteFile(googleAccountsPath, initialGoogleAccounts, 0600); err != nil {
				t.Fatal(err)
			}

			// Pre-existing SQLite 1: state.vscdb with ItemTable
			vscdbPath := filepath.Join(vscdbDir, "state.vscdb")
			db, err := sql.Open("sqlite", vscdbPath)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value TEXT)")
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("INSERT INTO ItemTable (key, value) VALUES ('antigravityUnifiedStateSync.userStatus', 'ORIGINAL_USER_STATUS')")
			if err != nil {
				t.Fatal(err)
			}
			_ = db.Close()

			// Pre-existing SQLite 2: cloud_accounts.db with accounts table
			cloudDBPath := filepath.Join(antigravityAgentDir, "cloud_accounts.db")
			cdb, err := sql.Open("sqlite", cloudDBPath)
			if err != nil {
				t.Fatal(err)
			}
			_, err = cdb.Exec("CREATE TABLE accounts (email TEXT PRIMARY KEY, is_active INTEGER)")
			if err != nil {
				t.Fatal(err)
			}
			_, err = cdb.Exec("INSERT INTO accounts (email, is_active) VALUES ('original_user@corp.org', 1), ('other@corp.org', 0)")
			if err != nil {
				t.Fatal(err)
			}
			_ = cdb.Close()

			// Files that DO NOT exist initially (must be deleted if created during sync)
			oauthCredsPath := filepath.Join(geminiDir, "oauth_creds.json")
			cliTokenPath := filepath.Join(cliDir, "antigravity-oauth-token")
			_ = os.Remove(oauthCredsPath)
			_ = os.Remove(cliTokenPath)

			// Hook injected failure
			failingStage := stage
			syncStepHookForTest = func(currentStep string) error {
				if currentStep == failingStage {
					return fmt.Errorf("injected catastrophic failure at stage %q", currentStep)
				}
				return nil
			}
			defer func() {
				syncStepHookForTest = nil
			}()

			targetAcc := &Account{
				Email:        "attacker_or_new@corp.org",
				AccessToken:  "ya29.brand_new_token_456",
				RefreshToken: "1//brand_new_refresh_token_789",
			}

			syncErr := SyncAllSurfaces(targetAcc, []string{"original_user@corp.org", targetAcc.Email}, nil)
			if syncErr == nil {
				t.Fatalf("expected SyncAllSurfaces to fail on injected stage %q, but succeeded", failingStage)
			}
			if !strings.Contains(syncErr.Error(), failingStage) {
				t.Fatalf("expected error to mention stage %q, got: %v", failingStage, syncErr)
			}

			// VERIFY ROLLBACK:
			// 1. Pre-existing files must match their exact original content
			restoredStandalone, err := os.ReadFile(standaloneTokenPath)
			if err != nil {
				t.Fatalf("standalone token missing after rollback: %v", err)
			}
			if string(restoredStandalone) != string(initialStandalone) {
				t.Fatalf("standalone token not restored! Got: %s", string(restoredStandalone))
			}

			restoredAppStorage, err := os.ReadFile(appStoragePath)
			if err != nil {
				t.Fatalf("app storage missing after rollback: %v", err)
			}
			if string(restoredAppStorage) != string(initialAppStorage) {
				t.Fatalf("app storage not restored! Got: %s", string(restoredAppStorage))
			}

			restoredGoogleAccounts, err := os.ReadFile(googleAccountsPath)
			if err != nil {
				t.Fatalf("google accounts missing after rollback: %v", err)
			}
			if string(restoredGoogleAccounts) != string(initialGoogleAccounts) {
				t.Fatalf("google accounts not restored! Got: %s", string(restoredGoogleAccounts))
			}

			// 2. Newly created files must be cleaned up
			if _, err := os.Stat(oauthCredsPath); !os.IsNotExist(err) {
				t.Fatalf("oauth_creds.json was NOT cleaned up after failed sync!")
			}
			if _, err := os.Stat(cliTokenPath); !os.IsNotExist(err) {
				t.Fatalf("cli antigravity-oauth-token was NOT cleaned up after failed sync!")
			}

			// 3. SQLite vscdb must be restored
			db, err = sql.Open("sqlite", vscdbPath)
			if err != nil {
				t.Fatal(err)
			}
			var restoredStatus string
			err = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityUnifiedStateSync.userStatus'").Scan(&restoredStatus)
			_ = db.Close()
			if err != nil || restoredStatus != "ORIGINAL_USER_STATUS" {
				t.Fatalf("vscdb status not restored! err=%v, val=%s", err, restoredStatus)
			}

			// 4. SQLite cloud_accounts.db must be restored
			cdb, err = sql.Open("sqlite", cloudDBPath)
			if err != nil {
				t.Fatal(err)
			}
			var activeEmail string
			err = cdb.QueryRow("SELECT email FROM accounts WHERE is_active = 1").Scan(&activeEmail)
			_ = cdb.Close()
			if err != nil || activeEmail != "original_user@corp.org" {
				t.Fatalf("cloud_accounts.db active user not restored! err=%v, active=%s", err, activeEmail)
			}
		})
	}
}

// ============================================================================
// 3. P1-12 OAUTH CSRF STATE REPLAY, BYPASS & EXPIRATION STRESS TEST
// ============================================================================

func TestChallenger_P1_12_OAuthStateSecurityHarness(t *testing.T) {
	cache := newOAuthStateCache()

	// 1. Replay Attack
	t.Run("ReplayAttack_Denied", func(t *testing.T) {
		state := "11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff"
		cache.Put(state, 5*time.Minute)

		valid1, reason1 := cache.ValidateAndConsume(state)
		if !valid1 || reason1 != "" {
			t.Fatalf("first validation of valid state failed: %s", reason1)
		}

		// Replay exact same state
		valid2, reason2 := cache.ValidateAndConsume(state)
		if valid2 {
			t.Fatalf("REPLAY ATTACK SUCCEEDED: same state accepted twice!")
		}
		if !strings.Contains(reason2, "invalid oauth state") {
			t.Fatalf("unexpected reason for replayed state: %s", reason2)
		}
	})

	// 2. Length & Content Variations
	t.Run("LengthAndContentVariations", func(t *testing.T) {
		validState := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
		cache.Put(validState, 5*time.Minute)

		testCases := []struct {
			name      string
			candidate string
			expectErr string
		}{
			{"empty string", "", "missing oauth state parameter"},
			{"whitespace string", "   \t\n  ", "missing oauth state parameter"},
			{"short string", "deadbeef", "invalid oauth state parameter"},
			{"single character truncated", validState[:len(validState)-1], "invalid oauth state parameter"},
			{"single character flipped", validState[:len(validState)-1] + "0", "invalid oauth state parameter"},
			{"extra character appended", validState + "a", "invalid oauth state parameter"},
			{"huge payload (100KB)", strings.Repeat("A", 100*1024), "invalid oauth state parameter"},
			{"null byte injection", validState[:10] + "\x00" + validState[11:], "invalid oauth state parameter"},
			{"sql injection string", "' OR '1'='1", "invalid oauth state parameter"},
			{"unicode variation", "ｄｅａｄｂｅｅｆ", "invalid oauth state parameter"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				valid, reason := cache.ValidateAndConsume(tc.candidate)
				if valid {
					t.Fatalf("hostile candidate %q unexpectedly accepted!", tc.candidate)
				}
				if !strings.Contains(reason, tc.expectErr) {
					t.Fatalf("candidate %q expected reason containing %q, got %q", tc.candidate, tc.expectErr, reason)
				}
			})
		}

		// Verify validState is still intact (none of the invalid attempts consumed it)
		ok, r := cache.ValidateAndConsume(validState)
		if !ok {
			t.Fatalf("valid state was prematurely consumed or invalidated: %s", r)
		}
	})

	// 3. Expiration Stress
	t.Run("ExpirationStress", func(t *testing.T) {
		expiringState := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
		// Register with very short TTL
		cache.Put(expiringState, 30*time.Millisecond)

		time.Sleep(60 * time.Millisecond)

		valid, reason := cache.ValidateAndConsume(expiringState)
		if valid {
			t.Fatalf("EXPIRED STATE ACCEPTED: state should have expired after 30ms!")
		}
		if !strings.Contains(reason, "expired") && !strings.Contains(reason, "invalid") {
			t.Fatalf("expected expiration reason, got: %s", reason)
		}
	})

	// 4. Concurrent Access & Race Conditions
	t.Run("ConcurrentThunderingHerd", func(t *testing.T) {
		concurrentState := "99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa"
		cache.Put(concurrentState, 5*time.Minute)

		var wg sync.WaitGroup
		successCount := 0
		var countMu sync.Mutex

		goroutines := 50
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				valid, _ := cache.ValidateAndConsume(concurrentState)
				if valid {
					countMu.Lock()
					successCount++
					countMu.Unlock()
				}
			}()
		}
		wg.Wait()

		if successCount != 1 {
			t.Fatalf("RACE CONDITION: exactly 1 goroutine should consume state, but %d succeeded!", successCount)
		}
	})
}

// 4. Manager & Endpoint Loopback Verification
func TestChallenger_P1_12_ManagerAndCallbackVerification(t *testing.T) {
	// Subtest 1: State Generation and Parameters
	t.Run("StateGenerationAndParameters", func(t *testing.T) {
		mgr := NewGoogleOAuthManager("dummy-client-id", "dummy-client-secret")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		go func() {
			_, _ = mgr.StartFlow(ctx, false)
		}()

		time.Sleep(50 * time.Millisecond)
		if !mgr.IsFlowActive() {
			t.Fatalf("expected flow to be active")
		}
		defer mgr.CancelFlow()

		activeState := mgr.GetActiveState()
		if len(activeState) != 64 {
			t.Fatalf("expected 64-char hex state, got len=%d: %q", len(activeState), activeState)
		}
		authURL := mgr.GetActiveAuthURL()
		parsedURL, err := url.Parse(authURL)
		if err != nil {
			t.Fatalf("invalid auth URL: %v", err)
		}
		if parsedURL.Query().Get("state") != activeState {
			t.Fatalf("auth URL query param state %q does not match active state %q", parsedURL.Query().Get("state"), activeState)
		}
	})

	// Subtest 2: Forged State Callback -> HTTP 400
	t.Run("ForgedStateCallback_Rejected400", func(t *testing.T) {
		mgr := NewGoogleOAuthManager("dummy-client-id", "dummy-client-secret")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		go func() {
			_, _ = mgr.StartFlow(ctx, false)
		}()

		time.Sleep(50 * time.Millisecond)
		if !mgr.IsFlowActive() {
			t.Fatalf("expected flow to be active")
		}
		defer mgr.CancelFlow()

		authURL := mgr.GetActiveAuthURL()
		parsedURL, _ := url.Parse(authURL)
		redirectURI := parsedURL.Query().Get("redirect_uri")

		forgedURL := fmt.Sprintf("%s?state=forged_state_token&code=4/dummy_code", redirectURI)
		resp, err := http.Get(forgedURL)
		if err != nil {
			t.Fatalf("GET forged callback failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400 for forged state on callback, got: %d", resp.StatusCode)
		}
	})

	// Subtest 3: Missing State Callback -> HTTP 400
	t.Run("MissingStateCallback_Rejected400", func(t *testing.T) {
		mgr := NewGoogleOAuthManager("dummy-client-id", "dummy-client-secret")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		go func() {
			_, _ = mgr.StartFlow(ctx, false)
		}()

		time.Sleep(50 * time.Millisecond)
		if !mgr.IsFlowActive() {
			t.Fatalf("expected flow to be active")
		}
		defer mgr.CancelFlow()

		authURL := mgr.GetActiveAuthURL()
		parsedURL, _ := url.Parse(authURL)
		redirectURI := parsedURL.Query().Get("redirect_uri")

		missingURL := fmt.Sprintf("%s?code=4/dummy_code", redirectURI)
		respMissing, err := http.Get(missingURL)
		if err != nil {
			t.Fatalf("GET missing state callback failed: %v", err)
		}
		defer respMissing.Body.Close()
		if respMissing.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400 for missing state on callback, got: %d", respMissing.StatusCode)
		}
	})
}
