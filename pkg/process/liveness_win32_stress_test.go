//go:build windows

package process

import (
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// TestStressWin32Liveness_CurrentPID verifies that the current running process (os.Getpid())
// is correctly identified as alive by both isSystemProcessAlive and isProcessAlive.
func TestStressWin32Liveness_CurrentPID(t *testing.T) {
	pid := os.Getpid()
	if !isSystemProcessAlive(pid) {
		t.Fatalf("isSystemProcessAlive(%d) returned false for current process", pid)
	}
	if !isProcessAlive(pid) {
		t.Fatalf("isProcessAlive(%d) returned false for current process", pid)
	}
}

// TestStressWin32Liveness_DeadPID verifies that non-existent PIDs (such as 999999)
// and negative or zero PIDs are reported as not alive.
func TestStressWin32Liveness_DeadPID(t *testing.T) {
	deadPIDs := []int{999999, 888888, 777777, 0, -1, -999}
	for _, pid := range deadPIDs {
		if pid > 0 {
			if isSystemProcessAlive(pid) {
				t.Errorf("isSystemProcessAlive(%d) unexpectedly returned true for dead PID", pid)
			}
		}
		if isProcessAlive(pid) {
			t.Errorf("isProcessAlive(%d) unexpectedly returned true for dead PID", pid)
		}
	}
}

// TestStressWin32Liveness_SpawnAndTerminate spawns an actual Win32 subprocess (cmd.exe),
// verifies it is detected as alive while executing, terminates it, and verifies
// that it immediately transitions to not alive.
func TestStressWin32Liveness_SpawnAndTerminate(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "timeout /t 10 /nobreak >nul")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start child process: %v", err)
	}
	childPID := cmd.Process.Pid

	// Verify child PID is alive
	if !isSystemProcessAlive(childPID) {
		_ = cmd.Process.Kill()
		t.Fatalf("child process %d was not reported alive while running", childPID)
	}
	if !isProcessAlive(childPID) {
		_ = cmd.Process.Kill()
		t.Fatalf("child process %d was not reported alive by isProcessAlive while running", childPID)
	}

	// Kill child process and wait for OS exit
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("failed to kill child process: %v", err)
	}
	_ = cmd.Wait()

	// Wait briefly for Win32 process cleanup
	deadline := time.Now().Add(2 * time.Second)
	alive := true
	for time.Now().Before(deadline) {
		if !isSystemProcessAlive(childPID) && !isProcessAlive(childPID) {
			alive = false
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	if alive {
		t.Fatalf("terminated child PID %d was still reported alive after 2 seconds", childPID)
	}
}

// TestStressWin32Liveness_ConcurrentStress executes high-frequency concurrent queries
// across 50 goroutines to ensure no handle leaks, race conditions, or panics occur in OpenProcess.
func TestStressWin32Liveness_ConcurrentStress(t *testing.T) {
	var wg sync.WaitGroup
	curPID := os.Getpid()
	deadPID := 999999

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if !isSystemProcessAlive(curPID) {
					t.Errorf("concurrent check failed: current PID %d reported dead", curPID)
				}
				if isSystemProcessAlive(deadPID) {
					t.Errorf("concurrent check failed: dead PID %d reported alive", deadPID)
				}
			}
		}()
	}
	wg.Wait()
}
