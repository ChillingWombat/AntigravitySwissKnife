package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// TestAdversarial_TunnelManager_HighConcurrencyStress stresses Start, Stop, appendLog,
// and GetStatus across hundreds of concurrent goroutines with race detector enabled.
func TestAdversarial_TunnelManager_HighConcurrencyStress(t *testing.T) {
	tm := NewTunnelManager(8765, "/api/bridge/mcp")

	const workers = 60
	const iterations = 200
	var wg sync.WaitGroup
	var panicCount int64

	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
					t.Errorf("Worker %d panicked: %v", workerID, r)
				}
				wg.Done()
			}()

			for j := 0; j < iterations; j++ {
				switch (workerID + j) % 5 {
				case 0:
					tm.appendLog(fmt.Sprintf("worker-%d-iter-%d log line", workerID, j))
				case 1:
					// Issue #120: the gateway URL is empty while no tunnel is active, so the
					// stress assertion only checks that the snapshot stays internally consistent.
					st := tm.GetStatus()
					if st.Active && st.GatewayURL == "" {
						t.Errorf("worker-%d: active tunnel must report a gateway URL", workerID)
					}
				case 2:
					// Rejected starts (issue #120) are still a valid concurrency path: they
					// take the lock and must never corrupt the status.
					_, _ = tm.Start("loopback", "")
				case 3:
					customURL := fmt.Sprintf("https://worker-%d.example.com", workerID)
					_, _ = tm.Start("custom", customURL)
				case 4:
					tm.Stop()
				}
			}
		}()
	}

	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		// Succeeded without deadlock
	case <-time.After(15 * time.Second):
		t.Fatal("Deadlock detected in TunnelManager under high concurrency!")
	}

	if panicCount > 0 {
		t.Fatalf("%d panics occurred during TunnelManager stress test", panicCount)
	}

	finalStatus := tm.GetStatus()
	if len(finalStatus.Logs) == 0 {
		t.Errorf("Expected logs to contain entries after stress test")
	}
}

// TestAdversarial_RemoteManager_HighConcurrencyStress tests RemoteManager methods under
// concurrent mutation, config saving, and tunnel interaction.
func TestAdversarial_RemoteManager_HighConcurrencyStress(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "remote_control_stress.json")

	rm := NewRemoteManager(cfgPath)

	const workers = 40
	const iterations = 100
	var wg sync.WaitGroup
	var panicCount int64

	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
					t.Errorf("Worker %d panicked: %v", workerID, r)
				}
				wg.Done()
			}()

			for j := 0; j < iterations; j++ {
				op := (workerID*13 + j) % 6
				switch op {
				case 0:
					_ = rm.GetStatus("user@example.com", 1000+workerID)
				case 1:
					_, _ = rm.RotatePIN()
				case 2:
					c := rm.GetConfig()
					c.FrameRate = 15 + (j % 45)
					_ = rm.UpdateConfig(c)
				case 3:
					rm.NotifyAccountSwitch(fmt.Sprintf("user-%d@example.com", j), 2000+j)
				case 4:
					_, _ = rm.StartTunnel("custom", fmt.Sprintf("https://remote-%d.app", j))
				case 5:
					rm.StopTunnel()
				}
			}
		}()
	}

	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		// Succeeded without deadlock
	case <-time.After(20 * time.Second):
		t.Fatal("Deadlock detected in RemoteManager under high concurrency!")
	}

	if panicCount > 0 {
		t.Fatalf("%d panics occurred during RemoteManager stress test", panicCount)
	}
}

// TestAdversarial_TunnelManager_LogRetentionAndSliceBoundary tests rapid log appending
// to confirm no slice bounds out of range or data races.
func TestAdversarial_TunnelManager_LogRetentionAndSliceBoundary(t *testing.T) {
	tm := NewTunnelManager(8765, "/api/bridge/mcp")

	var wg sync.WaitGroup
	const routines = 30
	const msgsPerRoutine = 300

	for i := 0; i < routines; i++ {
		wg.Add(1)
		go func(rID int) {
			defer wg.Done()
			for m := 0; m < msgsPerRoutine; m++ {
				tm.appendLog(fmt.Sprintf("routine %d message %d: %s", rID, m, "data payload"))
				if m%20 == 0 {
					st := tm.GetStatus()
					if len(st.Logs) > 40 {
						t.Errorf("Logs exceeded capacity cap of 40: got %d", len(st.Logs))
					}
				}
			}
		}(i)
	}

	wg.Wait()

	st := tm.GetStatus()
	if len(st.Logs) > 40 {
		t.Errorf("Logs slice not capped at 40 entries, got %d", len(st.Logs))
	}
}

// TestAdversarial_QuickTunnel_CancellationResponsiveness investigates whether Start("quick")
// blocks for the full 12-second timeout when Stop() is invoked while awaiting tunnel URL.
func TestAdversarial_QuickTunnel_CancellationResponsiveness(t *testing.T) {
	bin := LocateCloudflared()
	if bin == "" {
		t.Skip("cloudflared binary not available on host, skipping quick tunnel cancellation test")
	}

	tm := NewTunnelManager(9999, "/api/bridge/mcp")

	startReturned := make(chan time.Duration, 1)

	t0 := time.Now()
	go func() {
		_, _ = tm.Start("quick", "")
		startReturned <- time.Since(t0)
	}()

	// Wait 150ms to let cloudflared process launch
	time.Sleep(150 * time.Millisecond)

	// User requests Stop()
	stopStart := time.Now()
	tm.Stop()
	t.Logf("tm.Stop() executed in %v", time.Since(stopStart))

	// Measure when Start() returns
	select {
	case dur := <-startReturned:
		t.Logf("tm.Start('quick') returned after %v", dur)
		if dur > 5*time.Second {
			t.Errorf("FAILURE/BUG DETECTED: Start('quick') took %v to return after Stop() was called! Expected fast abort (<3s) upon Stop(), but it hung on 12-second timeout.", dur)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Start('quick') did not return within 15 seconds after Stop()")
	}
}

// TestAdversarial_QuickTunnel_ProcessLeakOnTimeout tests whether cloudflared process is
// properly terminated if startQuickTunnel() times out.
func TestAdversarial_QuickTunnel_ProcessLeakOnTimeout(t *testing.T) {
	// We create a dummy script that behaves like cloudflared but never outputs a URL
	tmpDir := t.TempDir()
	dummyBin := filepath.Join(tmpDir, "dummy_cloudflared")
	dummyScript := `#!/bin/sh
# Never outputs a matching trycloudflare.com URL
echo "Starting dummy tunnel..."
while true; do
    sleep 1
done
`
	if err := os.WriteFile(dummyBin, []byte(dummyScript), 0755); err != nil {
		t.Fatalf("failed to write dummy script: %v", err)
	}

	// Override CLOUDFLARED_PATH
	oldPath := os.Getenv("CLOUDFLARED_PATH")
	os.Setenv("CLOUDFLARED_PATH", dummyBin)
	defer os.Setenv("CLOUDFLARED_PATH", oldPath)

	tm := NewTunnelManager(9998, "/api/bridge/mcp")

	t0 := time.Now()
	st, err := tm.Start("quick", "")
	elapsed := time.Since(t0)
	t.Logf("Start returned in %v with err: %v, status: %+v", elapsed, err, st)

	if err == nil {
		t.Fatal("Expected error on timeout, got nil")
	}

	// Check if tm.cmd is still running
	tm.mu.RLock()
	cmd := tm.cmd
	tm.mu.RUnlock()

	if cmd != nil && cmd.Process != nil {
		p, findErr := os.FindProcess(cmd.Process.Pid)
		if findErr == nil && p != nil {
			// In UNIX, sending signal 0 checks if process is alive
			// If cmd was not killed upon timeout, this is a process leak!
			if err := cmd.Process.Signal(os.Signal(syscall.Signal(0))); err == nil {
				t.Errorf("FAILURE/BUG DETECTED: Process %d is still running after 12s timeout! startQuickTunnel leaked the subprocess.", cmd.Process.Pid)
			}
		}
		// Clean up process so it doesn't linger on system
		_ = cmd.Process.Kill()
	}
}
