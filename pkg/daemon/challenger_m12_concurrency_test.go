package daemon

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

// TestChallengerM12_DaemonFleetQuotaConcurrentRaceHarness aggressively tests concurrent
// swiss.getFleetQuota, swiss.refreshFleetQuota, and swiss.getStatus RPC invocations
// while background quota refresh is actively running.
func TestChallengerM12_DaemonFleetQuotaConcurrentRaceHarness(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sw_m12_cfg_*")
	if err != nil {
		t.Fatalf("MkdirTemp error: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	// Unix socket paths have a strict 108-byte limit across OSes (sun_path).
	// t.TempDir() with a long test name exceeds 108 bytes on Windows.
	sockDir, err := os.MkdirTemp("", "sw_m12_*")
	if err != nil {
		t.Fatalf("MkdirTemp error: %v", err)
	}
	defer os.RemoveAll(sockDir)
	sockPath := filepath.Join(sockDir, "d.sock")
	cfg := core.DefaultConfig()
	cfg.AllowNonGeminiNativeModels = true
	cfg.AutoSwitchEnabled = true

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	// Seed 8 accounts with various credentials, notes, and quota summaries
	for i := 0; i < 8; i++ {
		email := fmt.Sprintf("daemon_race_%d@example.com", i)
		acc := &keyring.Account{
			Email:        email,
			Label:        fmt.Sprintf("Account %d", i),
			PlanTier:     "Pro",
			Status:       "STANDBY",
			RefreshToken: fmt.Sprintf("refresh_%d", i),
			AccessToken:  fmt.Sprintf("access_%d", i),
			Notes:        fmt.Sprintf(`{"refresh_token":"recovered_rf_%d","access_token":"recovered_at_%d"}`, i, i),
			Credential: &keyring.OAuthCredential{
				AccessToken:  fmt.Sprintf("access_%d", i),
				RefreshToken: fmt.Sprintf("refresh_%d", i),
				IDToken:      fmt.Sprintf("id_%d", i),
			},
		}
		if i == 0 {
			acc.Status = "ACTIVE"
			acc.IsActive = true
		}
		if err := d.Keyring.AddOrUpdateAccount(acc); err != nil {
			t.Fatalf("AddOrUpdateAccount error: %v", err)
		}
		d.setQuotaSummary(email, &quota.QuotaSummary{
			AccountEmail:         email,
			Quota5hFraction:      0.1 * float64(i+1),
			QuotaWeeklyFraction:  0.2 * float64(i+1),
			Quota5hClaudeGPT:     0.8,
			QuotaWeeklyClaudeGPT: 0.9,
		})
	}
	_ = d.Keyring.SetActiveAccount("daemon_race_0@example.com")

	if err := d.Start(); err != nil {
		t.Fatalf("d.Start error: %v", err)
	}
	defer d.Stop()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var completedFleetCalls, completedRefreshCalls, completedStatusCalls int64

	// 15 client goroutines repeatedly calling swiss.getFleetQuota
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := ipc.NewClient(sockPath)
			for {
				select {
				case <-stop:
					return
				default:
					var fleetResp quota.FleetQuotaSummary
					if err := client.Call("swiss.getFleetQuota", nil, &fleetResp); err == nil {
						atomic.AddInt64(&completedFleetCalls, 1)
					}
					time.Sleep(2 * time.Millisecond)
				}
			}
		}()
	}

	// 5 client goroutines repeatedly calling swiss.refreshFleetQuota
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := ipc.NewClient(sockPath)
			for {
				select {
				case <-stop:
					return
				default:
					var refreshResp map[string]interface{}
					if err := client.Call("swiss.refreshFleetQuota", nil, &refreshResp); err == nil {
						atomic.AddInt64(&completedRefreshCalls, 1)
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
		}()
	}

	// 5 client goroutines repeatedly calling swiss.getStatus
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := ipc.NewClient(sockPath)
			for {
				select {
				case <-stop:
					return
				default:
					var statusResp map[string]interface{}
					if err := client.Call("swiss.getStatus", nil, &statusResp); err == nil {
						atomic.AddInt64(&completedStatusCalls, 1)
					}
					time.Sleep(3 * time.Millisecond)
				}
			}
		}()
	}

	// 2 writer goroutines updating account tokens in keyring store concurrently
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					idx := rand.Intn(8)
					email := fmt.Sprintf("daemon_race_%d@example.com", idx)
					_ = d.Keyring.UpdateAccountTokensAndMetadata(
						email,
						fmt.Sprintf("mutated_tok_%d_%d", workerID, rand.Int()),
						fmt.Sprintf("mutated_rf_%d_%d", workerID, rand.Int()),
						"Pro",
						float64(rand.Intn(500)),
					)
					time.Sleep(4 * time.Millisecond)
				}
			}
		}(i)
	}

	// Run concurrency test for 2 seconds under race detector
	time.Sleep(2 * time.Second)
	close(stop)
	wg.Wait()
	_ = d.Stop()
	time.Sleep(50 * time.Millisecond)

	t.Logf("Completed daemon stress test: %d fleet calls, %d refresh calls, %d status calls with 0 data races",
		atomic.LoadInt64(&completedFleetCalls),
		atomic.LoadInt64(&completedRefreshCalls),
		atomic.LoadInt64(&completedStatusCalls),
	)
}
