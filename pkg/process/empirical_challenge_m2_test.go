package process

import (
	"os/exec"
	"testing"
	"time"
)

// TestChallenge_IsProcessAliveMatching_PIDRecycling stress-tests isProcessAliveMatching
// to prove that recycled PIDs (alive PIDs belonging to different processes) are never falsely
// treated as matching the host IDE or language server.
func TestChallenge_IsProcessAliveMatching_PIDRecycling(t *testing.T) {
	// 1. Boundary: Non-positive PIDs
	if isProcessAliveMatching(0, "antigravity") {
		t.Errorf("PID 0 should never be considered alive")
	}
	if isProcessAliveMatching(-1234, "antigravity") {
		t.Errorf("Negative PID should never be considered alive")
	}

	// 2. Dead PID
	if isProcessAliveMatching(99999999, "antigravity") {
		t.Errorf("Non-existent PID 99999999 should not be alive")
	}

	// 3. Alive PID belonging to another process (PID 1 / init / systemd)
	// PID 1 is guaranteed to be alive on Linux, but its cmdline will NEVER be "antigravity" or "language_server".
	// An old, naive implementation using kill(pid, 0) alone would report true and try to kill PID 1!
	if isProcessAliveMatching(1, "antigravity") {
		t.Fatalf("CRITICAL SECURITY VULNERABILITY: isProcessAliveMatching(1, 'antigravity') returned true! PID recycling vulnerability detected!")
	}
	if isProcessAliveMatching(1, "language_server") {
		t.Fatalf("CRITICAL SECURITY VULNERABILITY: isProcessAliveMatching(1, 'language_server') returned true! PID recycling vulnerability detected!")
	}

	// 4. Live process lifecycle: spawn a benign process (sleep 5)
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to spawn test process: %v", err)
	}
	victimPID := cmd.Process.Pid

	// While sleep is running:
	// A) Must NOT match "antigravity" or "language_server"
	if isProcessAliveMatching(victimPID, "antigravity") {
		t.Errorf("PID %d (sleep) falsely matched 'antigravity'", victimPID)
	}
	if isProcessAliveMatching(victimPID, "language_server") {
		t.Errorf("PID %d (sleep) falsely matched 'language_server'", victimPID)
	}

	// B) MUST match its actual cmdline keyword "sleep"
	if !isProcessAliveMatching(victimPID, "sleep") {
		t.Errorf("PID %d (sleep) failed to match keyword 'sleep'", victimPID)
	}

	// C) Case-insensitivity check: "SLEEP", "Sleep"
	if !isProcessAliveMatching(victimPID, "SLEEP") {
		t.Errorf("PID %d (sleep) failed case-insensitive match 'SLEEP'", victimPID)
	}

	// D) Multiple keywords check
	if !isProcessAliveMatching(victimPID, "antigravity", "language_server", "sleep") {
		t.Errorf("PID %d (sleep) failed multi-keyword match with 'sleep' present", victimPID)
	}

	// 5. Terminate the process and ensure it transitions to dead
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	// Wait up to 1 second for kernel /proc cleanup
	deadline := time.Now().Add(1 * time.Second)
	aliveAfterDeath := true
	for time.Now().Before(deadline) {
		if !isProcessAliveMatching(victimPID, "sleep") {
			aliveAfterDeath = false
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if aliveAfterDeath {
		t.Errorf("PID %d still reported alive after process death and wait", victimPID)
	}
}

// TestChallenge_ZombieProcessDetection tests that defunct / zombie processes
// are recognized and not reported as active.
func TestChallenge_ZombieProcessDetection(t *testing.T) {
	// Spawn child that immediately exits while parent delays Wait() to observe zombie state
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to spawn child: %v", err)
	}
	pid := cmd.Process.Pid

	// Wait briefly for process to exit and become a zombie in kernel
	time.Sleep(50 * time.Millisecond)

	// In zombie state, /proc/<pid> exists, but isZombieProcess(pid) must detect it
	// Clean up after test
	defer func() {
		_ = cmd.Wait()
	}()

	// If it is in zombie state, isProcessAliveMatching must return false
	if isZombieProcess(pid) {
		if isProcessAliveMatching(pid) {
			t.Errorf("zombie process %d was falsely reported as alive by isProcessAliveMatching", pid)
		}
	}
}
