package keyring

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestChallenge_WriteAtomic_PartialWriteRaces stress-tests writeAtomic under high concurrency.
// 50 goroutines write distinct valid JSON payloads to the exact same file path,
// while 50 goroutines concurrently read from the file.
// Every read must yield a 100% complete, uncorrupted JSON document. No partial/0-byte reads allowed.
func TestChallenge_WriteAtomic_PartialWriteRaces(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "nested", "atomic_test.json")

	type Payload struct {
		WriterID  int    `json:"writer_id"`
		Timestamp int64  `json:"timestamp"`
		Content   string `json:"content"`
	}

	// Write initial file
	initPayload := Payload{
		WriterID:  0,
		Timestamp: time.Now().UnixNano(),
		Content:   strings.Repeat("InitialPayloadHeader-", 20),
	}
	initBytes, err := json.Marshal(initPayload)
	if err != nil {
		t.Fatalf("failed to marshal init payload: %v", err)
	}
	if err := writeAtomic(targetFile, initBytes, 0644); err != nil {
		t.Fatalf("failed to write initial atomic file: %v", err)
	}

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, 200)

	// 50 concurrent writers
	for w := 1; w <= 50; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				p := Payload{
					WriterID:  writerID,
					Timestamp: time.Now().UnixNano(),
					Content:   fmt.Sprintf("Writer-%d-Iter-%d-%s", writerID, i, strings.Repeat("ABCDEF123456", 50)),
				}
				data, mErr := json.Marshal(p)
				if mErr != nil {
					select {
					case errCh <- mErr:
					default:
					}
					return
				}
				if wErr := writeAtomic(targetFile, data, 0600); wErr != nil {
					select {
					case errCh <- fmt.Errorf("writeAtomic failed: %w", wErr):
					default:
					}
					return
				}
			}
		}(w)
	}

	// 50 concurrent readers
	for r := 1; r <= 50; r++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					data, rErr := os.ReadFile(targetFile)
					if rErr != nil {
						select {
						case errCh <- fmt.Errorf("reader %d encountered read error: %w", readerID, rErr):
						default:
						}
						return
					}
					if len(data) == 0 {
						select {
						case errCh <- fmt.Errorf("reader %d encountered zero-byte partial read!", readerID):
						default:
						}
						return
					}
					var p Payload
					if uErr := json.Unmarshal(data, &p); uErr != nil {
						select {
						case errCh <- fmt.Errorf("reader %d encountered corrupted JSON (partial write): %w (data len=%d, snippet=%q)", readerID, uErr, len(data), string(data[:min(50, len(data))])):
						default:
						}
						return
					}
					if p.WriterID < 0 || p.Content == "" {
						select {
						case errCh <- fmt.Errorf("reader %d encountered invalid payload data: %+v", readerID, p):
						default:
						}
						return
					}
				}
			}
		}(r)
	}

	// Wait for writers to complete
	writersDone := make(chan struct{})
	go func() {
		// Wait for the first 50 goroutines (writers)
		// We use a small timer then close stopCh
		time.Sleep(500 * time.Millisecond)
		close(stopCh)
		wg.Wait()
		close(writersDone)
	}()

	select {
	case <-writersDone:
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for atomic write/read stress test")
	}

	close(errCh)
	for e := range errCh {
		t.Fatalf("Adversarial race failure: %v", e)
	}

	// Verify final file permissions and validity
	fi, err := os.Stat(targetFile)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("expected mode 0600, got %v", fi.Mode().Perm())
	}
}

// TestChallenge_CloudAccounts_NativeSQLiteAccess verifies that cloud_accounts.db operations
// function natively via modernc.org/sqlite without any python3 or external process calls.
func TestChallenge_CloudAccounts_NativeSQLiteAccess(t *testing.T) {
	tmpHome := t.TempDir()
	agentDir := filepath.Join(tmpHome, ".antigravity-agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create agent dir: %v", err)
	}
	dbPath := filepath.Join(agentDir, "cloud_accounts.db")

	// 1. Create a real sqlite db and tables
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE accounts (
			email TEXT PRIMARY KEY,
			name TEXT,
			is_active INTEGER,
			token_json TEXT,
			quota_json TEXT,
			status TEXT
		);
		CREATE TABLE settings (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create schema: %v", err)
	}

	// Prepare encrypted AGM token
	masterKeyHex := DefaultAGMMasterKeyHex
	keyBytes, _ := hex.DecodeString(masterKeyHex)
	block, _ := aes.NewCipher(keyBytes)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	_, _ = io.ReadFull(rand.Reader, nonce)

	plainTokenJSON := `{"access_token":"ya29.native_go_token","refresh_token":"1//native_refresh_token","id_token":"id_token_123"}`
	sealed := gcm.Seal(nil, nonce, []byte(plainTokenJSON), nil)
	ct := sealed[:len(sealed)-gcm.Overhead()]
	tag := sealed[len(sealed)-gcm.Overhead():]
	agmEncStr := fmt.Sprintf("%s%s:%s:%s", AGMPrefix, hex.EncodeToString(nonce), hex.EncodeToString(tag), hex.EncodeToString(ct))

	plainQuotaJSON := `{
		"subscription_tier": "Pro",
		"ai_credits": {"credits": 10.5},
		"quota_groups": [
			{
				"display_name": "Gemini Models",
				"buckets": [
					{"window": "5h", "bucket_id": "gemini_5h", "remaining_fraction": 0.85, "reset_time": "2026-10-13T12:00:00Z"},
					{"window": "weekly", "bucket_id": "gemini_weekly", "remaining_fraction": 0.95, "reset_time": "2026-10-20T00:00:00Z"}
				]
			}
		]
	}`

	_, err = db.Exec(`
		INSERT INTO accounts (email, name, is_active, token_json, quota_json, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "native_user@domain.com", "Native User", 1, agmEncStr, plainQuotaJSON, "ACTIVE")
	if err != nil {
		db.Close()
		t.Fatalf("failed to insert test account: %v", err)
	}
	db.Close()

	// 2. Read back via ReadCloudAccountsDB
	accounts, err := ReadCloudAccountsDB(tmpHome)
	if err != nil {
		t.Fatalf("ReadCloudAccountsDB failed: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	acc := accounts[0]
	if acc.Email != "native_user@domain.com" {
		t.Fatalf("expected email native_user@domain.com, got %s", acc.Email)
	}
	if acc.AccessToken != "ya29.native_go_token" {
		t.Fatalf("expected decrypted access token ya29.native_go_token, got %s", acc.AccessToken)
	}
	if acc.RefreshToken != "1//native_refresh_token" {
		t.Fatalf("expected decrypted refresh token 1//native_refresh_token, got %s", acc.RefreshToken)
	}
	if acc.Quota5h != 0.85 || acc.QuotaWeekly != 0.95 {
		t.Fatalf("expected quota 0.85/0.95, got %v/%v", acc.Quota5h, acc.QuotaWeekly)
	}

	// 3. Test SyncCloudAccountsAutoSwitch
	if err := SyncCloudAccountsAutoSwitch(tmpHome, true); err != nil {
		t.Fatalf("SyncCloudAccountsAutoSwitch(true) failed: %v", err)
	}

	// Verify setting in sqlite
	checkDB, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("failed to open check db: %v", err)
	}
	var settingVal string
	err = checkDB.QueryRow("SELECT value FROM settings WHERE key = 'auto_switch_enabled'").Scan(&settingVal)
	if err != nil {
		checkDB.Close()
		t.Fatalf("failed to read setting: %v", err)
	}
	if settingVal != "true" {
		checkDB.Close()
		t.Fatalf("expected setting 'true', got %q", settingVal)
	}
	checkDB.Close()

	// 4. Test non-existent DB
	missingAccounts, err := ReadCloudAccountsDB(filepath.Join(tmpHome, "nonexistent"))
	if err != nil {
		t.Fatalf("expected nil error for missing db, got %v", err)
	}
	if len(missingAccounts) != 0 {
		t.Fatalf("expected 0 accounts for missing db, got %d", len(missingAccounts))
	}
}
