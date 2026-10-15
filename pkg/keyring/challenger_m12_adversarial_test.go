package keyring

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestChallengerM12_CloneEdgeCases tests edge cases for the Account.Clone() method.
func TestChallengerM12_CloneEdgeCases(t *testing.T) {
	// 1. Nil receiver returns nil
	var nilAcc *Account
	if cloned := nilAcc.Clone(); cloned != nil {
		t.Fatalf("expected nilAcc.Clone() to be nil, got %+v", cloned)
	}

	// 2. Account with nil Credential clones cleanly without panicking
	accNoCred := &Account{
		Email:      "nocred@example.com",
		Label:      "No Cred",
		Credential: nil,
	}
	clonedNoCred := accNoCred.Clone()
	if clonedNoCred == nil {
		t.Fatalf("expected non-nil clone")
	}
	if clonedNoCred == accNoCred {
		t.Fatalf("expected different pointer addresses")
	}
	if clonedNoCred.Credential != nil {
		t.Fatalf("expected cloned Credential to be nil")
	}
	if clonedNoCred.Email != "nocred@example.com" {
		t.Fatalf("field mismatch on clone: got %s", clonedNoCred.Email)
	}

	// 3. Account with populated Credential performs true deep copy
	originalCred := &OAuthCredential{
		AccessToken:  "orig_access",
		RefreshToken: "orig_refresh",
		IDToken:      "orig_id",
		Expiry:       "2026-10-15T12:00:00Z",
	}
	accWithCred := &Account{
		Email:       "cred@example.com",
		Label:       "With Cred",
		Credential:  originalCred,
		AccessToken: "orig_access",
	}
	clonedWithCred := accWithCred.Clone()
	if clonedWithCred == accWithCred {
		t.Fatalf("expected distinct Account pointers")
	}
	if clonedWithCred.Credential == accWithCred.Credential {
		t.Fatalf("expected distinct Credential pointers, but got identical address %p", clonedWithCred.Credential)
	}

	// Mutate clone and assert original is completely untouched
	clonedWithCred.Credential.AccessToken = "mutated_access"
	clonedWithCred.Credential.RefreshToken = "mutated_refresh"
	clonedWithCred.Credential.IDToken = "mutated_id"
	clonedWithCred.Credential.Expiry = "mutated_expiry"
	clonedWithCred.Email = "mutated@example.com"

	if accWithCred.Credential.AccessToken != "orig_access" {
		t.Errorf("original Credential.AccessToken was corrupted: got %s, want orig_access", accWithCred.Credential.AccessToken)
	}
	if accWithCred.Credential.RefreshToken != "orig_refresh" {
		t.Errorf("original Credential.RefreshToken was corrupted: got %s, want orig_refresh", accWithCred.Credential.RefreshToken)
	}
	if accWithCred.Credential.IDToken != "orig_id" {
		t.Errorf("original Credential.IDToken was corrupted: got %s, want orig_id", accWithCred.Credential.IDToken)
	}
	if accWithCred.Credential.Expiry != "2026-10-15T12:00:00Z" {
		t.Errorf("original Credential.Expiry was corrupted: got %s", accWithCred.Credential.Expiry)
	}
	if accWithCred.Email != "cred@example.com" {
		t.Errorf("original Email was corrupted: got %s", accWithCred.Email)
	}
}

// TestChallengerM12_DeepCopyInvariants_GetAccount verifies that modifying an Account
// returned by GetAccount() never mutates the underlying stored Account or its Credential.
func TestChallengerM12_DeepCopyInvariants_GetAccount(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	email := "immutable_target@example.com"
	initialCred := &OAuthCredential{
		AccessToken:  "initial_access_token",
		RefreshToken: "initial_refresh_token",
		IDToken:      "initial_id_token",
		Expiry:       "2026-10-15T15:00:00Z",
	}
	initialAcc := &Account{
		Email:                email,
		Label:                "Immutable Target",
		PlanTier:             "Ultra",
		Status:               "ACTIVE",
		Priority:             "Highest",
		Notes:                "Important account notes",
		Password:             "SuperSecretPass",
		TOTPSecret:           "JBSWY3DPEHPK3PXP",
		Credits:              150.75,
		EnableCreditOverages: true,
		AllowClaudeGPT:       true,
		AccessToken:          "initial_access_token",
		RefreshToken:         "initial_refresh_token",
		IDToken:              "initial_id_token",
		Credential:           initialCred,
	}

	if err := store.AddOrUpdateAccount(initialAcc); err != nil {
		t.Fatalf("AddOrUpdateAccount error: %v", err)
	}

	// 1. Retrieve account via GetAccount
	got1, err := store.GetAccount(email)
	if err != nil {
		t.Fatalf("GetAccount error: %v", err)
	}
	if got1.Credential == nil {
		t.Fatalf("expected non-nil Credential")
	}

	// 2. Retrieve account a second time
	got2, err := store.GetAccount(email)
	if err != nil {
		t.Fatalf("GetAccount second call error: %v", err)
	}

	// Assert pointer disjunction
	if got1 == got2 {
		t.Fatalf("GetAccount must return distinct account instances, got same pointer %p", got1)
	}
	if got1.Credential == got2.Credential {
		t.Fatalf("GetAccount must return distinct Credential instances, got same pointer %p", got1.Credential)
	}

	// 3. Mutate got1 maliciously across all fields and pointers
	got1.Email = "hacked@example.com"
	got1.Label = "Hacked Label"
	got1.PlanTier = "Free"
	got1.Status = "BANNED"
	got1.Priority = "Lowest"
	got1.Notes = "Hacked notes"
	got1.Password = "hacked_password"
	got1.TOTPSecret = "HACKEDTOTP"
	got1.Credits = -9999.0
	got1.EnableCreditOverages = false
	got1.AllowClaudeGPT = false
	got1.AccessToken = "hacked_token"
	got1.RefreshToken = "hacked_refresh"
	got1.IDToken = "hacked_id"
	got1.Credential.AccessToken = "malicious_access"
	got1.Credential.RefreshToken = "malicious_refresh"
	got1.Credential.IDToken = "malicious_id"
	got1.Credential.Expiry = "malicious_expiry"

	// 4. Retrieve stored account anew and verify full immutability
	stored, err := store.GetAccount(email)
	if err != nil {
		t.Fatalf("GetAccount error after mutation: %v", err)
	}

	if stored.Email != email {
		t.Errorf("stored Email leaked mutation: got %s, want %s", stored.Email, email)
	}
	if stored.Label != "Immutable Target" {
		t.Errorf("stored Label leaked mutation: got %s", stored.Label)
	}
	if stored.PlanTier != "Ultra" {
		t.Errorf("stored PlanTier leaked mutation: got %s", stored.PlanTier)
	}
	if stored.Status != "ACTIVE" {
		t.Errorf("stored Status leaked mutation: got %s", stored.Status)
	}
	if stored.Priority != "Highest" {
		t.Errorf("stored Priority leaked mutation: got %s", stored.Priority)
	}
	if stored.Notes != "Important account notes" {
		t.Errorf("stored Notes leaked mutation: got %s", stored.Notes)
	}
	if stored.Password != "SuperSecretPass" {
		t.Errorf("stored Password leaked mutation: got %s", stored.Password)
	}
	if stored.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("stored TOTPSecret leaked mutation: got %s", stored.TOTPSecret)
	}
	if stored.Credits != 150.75 {
		t.Errorf("stored Credits leaked mutation: got %f", stored.Credits)
	}
	if !stored.EnableCreditOverages {
		t.Errorf("stored EnableCreditOverages leaked mutation")
	}
	if !stored.AllowClaudeGPT {
		t.Errorf("stored AllowClaudeGPT leaked mutation")
	}
	if stored.Credential == nil {
		t.Fatalf("stored Credential became nil")
	}
	if stored.Credential.AccessToken != "initial_access_token" {
		t.Errorf("stored Credential.AccessToken was mutated: got %s, want initial_access_token", stored.Credential.AccessToken)
	}
	if stored.Credential.RefreshToken != "initial_refresh_token" {
		t.Errorf("stored Credential.RefreshToken was mutated: got %s, want initial_refresh_token", stored.Credential.RefreshToken)
	}
	if stored.Credential.IDToken != "initial_id_token" {
		t.Errorf("stored Credential.IDToken was mutated: got %s, want initial_id_token", stored.Credential.IDToken)
	}
	if stored.Credential.Expiry != "2026-10-15T15:00:00Z" {
		t.Errorf("stored Credential.Expiry was mutated: got %s", stored.Credential.Expiry)
	}
}

// TestChallengerM12_DeepCopyInvariants_ListAccounts verifies that modifying accounts
// returned by ListAccounts() never affects the stored accounts or other callers.
func TestChallengerM12_DeepCopyInvariants_ListAccounts(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	for i := 0; i < 5; i++ {
		email := fmt.Sprintf("fleet_%d@example.com", i)
		acc := &Account{
			Email:        email,
			Label:        fmt.Sprintf("Fleet Account %d", i),
			Status:       "STANDBY",
			AccessToken:  fmt.Sprintf("token_%d", i),
			RefreshToken: fmt.Sprintf("refresh_%d", i),
			Credential: &OAuthCredential{
				AccessToken:  fmt.Sprintf("token_%d", i),
				RefreshToken: fmt.Sprintf("refresh_%d", i),
			},
		}
		if err := store.AddOrUpdateAccount(acc); err != nil {
			t.Fatalf("AddOrUpdateAccount error: %v", err)
		}
	}

	list1 := store.ListAccounts()
	list2 := store.ListAccounts()

	if len(list1) != 5 || len(list2) != 5 {
		t.Fatalf("expected 5 accounts in both lists")
	}

	for i := 0; i < 5; i++ {
		if list1[i] == list2[i] {
			t.Fatalf("list1[%d] and list2[%d] share identical Account pointer %p", i, i, list1[i])
		}
		if list1[i].Credential == list2[i].Credential {
			t.Fatalf("list1[%d].Credential and list2[%d].Credential share identical Credential pointer %p", i, i, list1[i].Credential)
		}
		// Mutate list1 item
		list1[i].Credential.AccessToken = fmt.Sprintf("corrupted_%d", i)
		list1[i].Email = fmt.Sprintf("corrupted_email_%d", i)
	}

	// Verify list2 is untouched
	for i := 0; i < 5; i++ {
		expectedToken := fmt.Sprintf("token_%d", i)
		expectedEmail := fmt.Sprintf("fleet_%d@example.com", i)
		if list2[i].Credential.AccessToken != expectedToken {
			t.Errorf("list2[%d] was contaminated by list1 mutation: got %s, want %s", i, list2[i].Credential.AccessToken, expectedToken)
		}
		if list2[i].Email != expectedEmail {
			t.Errorf("list2[%d].Email was contaminated: got %s, want %s", i, list2[i].Email, expectedEmail)
		}
	}
}

// TestChallengerM12_ConcurrentReadWriteStressHarness executes a high-concurrency race stress
// harness where dozens of goroutines concurrently read, clone, mutate, and update accounts.
func TestChallengerM12_ConcurrentReadWriteStressHarness(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// Seed 10 accounts with credentials and notes
	numAccounts := 10
	for i := 0; i < numAccounts; i++ {
		email := fmt.Sprintf("user_%d@example.com", i)
		acc := &Account{
			Email:        email,
			Label:        fmt.Sprintf("User %d", i),
			Status:       "STANDBY",
			AccessToken:  fmt.Sprintf("access_%d", i),
			RefreshToken: fmt.Sprintf("refresh_%d", i),
			Notes:        fmt.Sprintf(`{"refresh_token":"recovered_%d","access_token":"token_%d"}`, i, i),
			Credential: &OAuthCredential{
				AccessToken:  fmt.Sprintf("access_%d", i),
				RefreshToken: fmt.Sprintf("refresh_%d", i),
			},
		}
		_ = store.AddOrUpdateAccount(acc)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var readOps, writeOps, exportOps int64

	// 1. 20 Readers calling ListAccounts() and immediately mutating the returned copies
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					accs := store.ListAccounts()
					for _, a := range accs {
						if a != nil && a.Credential != nil {
							// Deliberately mutate returned struct to verify no data race with other readers or store
							a.Credential.AccessToken = fmt.Sprintf("reader_%d_tok", id)
							a.Credential.RefreshToken = fmt.Sprintf("reader_%d_ref", id)
							a.Email = fmt.Sprintf("mutated_%d", rand.Int())
							a.Credits = rand.Float64()
						}
					}
					atomic.AddInt64(&readOps, 1)
				}
			}
		}(i)
	}

	// 2. 20 Readers calling GetAccount() and immediately mutating the returned copy
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					targetEmail := fmt.Sprintf("user_%d@example.com", rand.Intn(numAccounts))
					acc, err := store.GetAccount(targetEmail)
					if err == nil && acc != nil && acc.Credential != nil {
						// Deliberately mutate returned struct
						acc.Credential.AccessToken = fmt.Sprintf("get_%d_tok", id)
						acc.Credential.RefreshToken = fmt.Sprintf("get_%d_ref", id)
						acc.Status = "ACTIVE"
					}
					atomic.AddInt64(&readOps, 1)
				}
			}
		}(i)
	}

	// 3. 10 Writers calling AddOrUpdateAccount, UpdateAccountTokensAndMetadata, UpdateAccountStatusWithError
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					idx := rand.Intn(numAccounts)
					targetEmail := fmt.Sprintf("user_%d@example.com", idx)
					switch rand.Intn(4) {
					case 0:
						_ = store.UpdateAccountTokensAndMetadata(
							targetEmail,
							fmt.Sprintf("new_tok_%d_%d", id, rand.Int()),
							fmt.Sprintf("new_ref_%d_%d", id, rand.Int()),
							"Pro",
							float64(rand.Intn(100)),
						)
					case 1:
						_ = store.UpdateAccountStatusWithError(targetEmail, "STANDBY", "")
					case 2:
						_ = store.SetActiveAccount(targetEmail)
					case 3:
						_ = store.AddOrUpdateAccount(&Account{
							Email:        targetEmail,
							Label:        fmt.Sprintf("Updated User %d", idx),
							Status:       "STANDBY",
							AccessToken:  fmt.Sprintf("at_%d", id),
							RefreshToken: fmt.Sprintf("rt_%d", id),
							Credential: &OAuthCredential{
								AccessToken:  fmt.Sprintf("at_%d", id),
								RefreshToken: fmt.Sprintf("rt_%d", id),
							},
						})
					}
					atomic.AddInt64(&writeOps, 1)
				}
			}
		}(i)
	}

	// 4. 5 Exporters calling ExportAccounts()
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					exports := store.ExportAccounts()
					_ = len(exports)
					_ = store.ActiveAccount()
					atomic.AddInt64(&exportOps, 1)
				}
			}
		}()
	}

	// Let the stress test run under race detector
	time.Sleep(1500 * time.Millisecond)
	close(stop)
	wg.Wait()

	t.Logf("Completed stress harness: %d read ops, %d write ops, %d export ops with 0 data races",
		atomic.LoadInt64(&readOps), atomic.LoadInt64(&writeOps), atomic.LoadInt64(&exportOps))
}
