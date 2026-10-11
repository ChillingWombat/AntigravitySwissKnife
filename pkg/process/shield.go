package process

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/revival"
)

var relaunchMu sync.Mutex

// ProcessInfo holds information about a detected system process.
type ProcessInfo struct {
	PID         int    `json:"pid"`
	PPID        int    `json:"ppid"`
	Name        string `json:"name"`
	Cmdline     string `json:"cmdline"`
	IsHostIDE   bool   `json:"is_host_ide"`
	IsProtected bool   `json:"is_protected"`
}

// Shield safeguards the host Antigravity 2.0 from accidental termination.
type Shield struct {
	protectedPID  int
	processFinder func() ([]ProcessInfo, error)
	revivalEngine *revival.Engine
	focusFunc     func() error
}

// NewShield initializes a Shield. If protectedPID is 0, it reads from environment or detects host.
func NewShield(protectedPID int) *Shield {
	if protectedPID <= 0 {
		if envPID := os.Getenv("ANTIGRAVITY_HOST_PID"); envPID != "" {
			if p, err := strconv.Atoi(envPID); err == nil && p > 0 {
				protectedPID = p
			}
		}
	}
	return &Shield{protectedPID: protectedPID}
}

// SetRevivalEngine sets the revival engine instance for coordination during relaunches.
func (s *Shield) SetRevivalEngine(engine *revival.Engine) {
	s.revivalEngine = engine
}

func (s *Shield) getRevivalEngine() *revival.Engine {
	if s.revivalEngine != nil {
		return s.revivalEngine
	}
	return revival.NewEngine("", 0)
}

// SetProcessFinder overrides process discovery for tests or custom environments.
func (s *Shield) SetProcessFinder(finder func() ([]ProcessInfo, error)) {
	s.processFinder = finder
}

// SetFocusFunc overrides window focus execution for tests or custom environments.
func (s *Shield) SetFocusFunc(fn func() error) {
	s.focusFunc = fn
}

func (s *Shield) findAntigravityProcessesInternal() ([]ProcessInfo, error) {
	if s.processFinder != nil {
		return s.processFinder()
	}
	return s.FindAntigravityProcesses()
}

// ProtectedPID returns the currently protected host PID.
func (s *Shield) ProtectedPID() int {
	return s.protectedPID
}

// SetProtectedPID updates the protected PID.
func (s *Shield) SetProtectedPID(pid int) {
	s.protectedPID = pid
}

// IsProtected returns true if the PID must not be signaled or terminated.
func (s *Shield) IsProtected(pid int) bool {
	if pid <= 0 {
		return true // invalid PID
	}
	if s.protectedPID > 0 && pid == s.protectedPID {
		return true
	}
	// Own process is always protected
	if pid == os.Getpid() {
		return true
	}
	return false
}

// SafeKill sends a signal to a PID only if it is NOT protected.
func (s *Shield) SafeKill(pid int, sig syscall.Signal) error {
	if s.IsProtected(pid) {
		return fmt.Errorf("%w: PID %d", core.ErrHostProtected, pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

// FindAntigravityProcesses scans Linux /proc for running antigravity instances.
func (s *Shield) FindAntigravityProcesses() ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var results []ProcessInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		if isZombieProcess(pid) {
			continue
		}

		cmdlineBytes, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
		if strings.Contains(strings.ToLower(cmdline), "antigravity") {
			commBytes, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
			comm := strings.TrimSpace(string(commBytes))

			isHost := (s.protectedPID > 0 && pid == s.protectedPID)
			results = append(results, ProcessInfo{
				PID:         pid,
				Name:        comm,
				Cmdline:     strings.TrimSpace(cmdline),
				IsHostIDE:   isHost,
				IsProtected: s.IsProtected(pid),
			})
		}
	}

	// Prioritize primary Desktop IDE process at the front (comm == "antigravity" and not an Electron sub-process)
	sort.SliceStable(results, func(i, j int) bool {
		isMainIDE := func(p ProcessInfo) bool {
			lower := strings.ToLower(p.Cmdline)
			return p.Name == "antigravity" && !strings.Contains(lower, "--type=")
		}
		mainI := isMainIDE(results[i])
		mainJ := isMainIDE(results[j])
		if mainI != mainJ {
			return mainI
		}
		return results[i].PID < results[j].PID
	})

	return results, nil
}

// IsAntigravityRunning returns true if an Antigravity main application process is currently running.
func (s *Shield) IsAntigravityRunning() bool {
	procs, err := s.findAntigravityProcessesInternal()
	if err != nil || len(procs) == 0 {
		return false
	}
	for _, p := range procs {
		lower := strings.ToLower(p.Cmdline)
		nameLower := strings.ToLower(p.Name)
		if (nameLower == "antigravity" || nameLower == "antigravity.exe" || strings.HasSuffix(nameLower, "antigravity") || strings.HasSuffix(nameLower, "antigravity.exe")) &&
			!strings.Contains(lower, "--type=") &&
			!strings.Contains(lower, "swiss") {
			return true
		}
	}
	return false
}

// FindLanguageServerProcesses scans for active Antigravity language_server instances.
func (s *Shield) FindLanguageServerProcesses() ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		// Non-Linux or restricted /proc fallback
		return nil, err
	}

	var results []ProcessInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		if isZombieProcess(pid) {
			continue
		}

		cmdlineBytes, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
		lower := strings.ToLower(cmdline)

		// Match language_server spawned for Antigravity IDE hub
		if strings.Contains(lower, "language_server") && (strings.Contains(lower, "antigravity") || strings.Contains(lower, "subclient_type hub")) {
			commBytes, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
			comm := strings.TrimSpace(string(commBytes))

			results = append(results, ProcessInfo{
				PID:         pid,
				Name:        comm,
				Cmdline:     strings.TrimSpace(cmdline),
				IsHostIDE:   false,
				IsProtected: s.IsProtected(pid),
			})
		}
	}
	return results, nil
}

// RestartLanguageServer terminates the running language_server child process so the host IDE supervisor respawns it with new credentials.
func (s *Shield) RestartLanguageServer() error {
	procs, err := s.FindLanguageServerProcesses()
	if err != nil || len(procs) == 0 {
		return fmt.Errorf("no active Antigravity language_server process found")
	}

	for _, p := range procs {
		if s.IsProtected(p.PID) {
			continue
		}
		// Send SIGTERM so Electron's monitorLsCrashInternal detects termination and respawns cleanly
		_ = s.SafeKill(p.PID, syscall.SIGTERM)
	}
	return nil
}

// LaunchHostIDE launches the Antigravity desktop IDE application if it is not currently running.
// If already running, it actively brings the active window to front via CDP.
func (s *Shield) LaunchHostIDE() error {
	relaunchMu.Lock()
	defer relaunchMu.Unlock()

	if s.IsAntigravityRunning() {
		// Already running: bring active window to front via CDP
		if s.focusFunc != nil {
			return s.focusFunc()
		}
		inj := gui.NewInjector(0)
		return inj.FocusActiveWindows()
	}

	// Safeguard: Never spawn during unit test runs unless explicitly authorized
	if (flag.Lookup("test.v") != nil || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1") && os.Getenv("ANTIGRAVITY_ALLOW_TEST_RELAUNCH") != "1" {
		return nil
	}

	// Clear stale Chromium/Electron singleton locks & DevTools port file if dead
	hostConfigDir := core.GetAntigravityHostConfigDir()
	_ = os.Remove(filepath.Join(hostConfigDir, "SingletonLock"))
	_ = os.Remove(filepath.Join(hostConfigDir, "SingletonSocket"))
	_ = os.Remove(filepath.Join(hostConfigDir, "SingletonCookie"))
	_ = os.Remove(filepath.Join(hostConfigDir, "DevToolsActivePort"))

	binPath := core.GetAntigravityBinaryPath()
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", "-a", "Antigravity")
	} else {
		cmd = exec.Command(binPath)
	}

	if err := core.LaunchDetachedProcess(cmd); err != nil {
		return fmt.Errorf("failed to launch Antigravity (%s): %w", binPath, err)
	}

	return nil
}

// RelaunchHostIDE gracefully terminates the running host Antigravity IDE (if alive),
// relaunches it as a detached background process so it reads fresh credentials from disk,
// and restores the previously active conversation view.
func (s *Shield) RelaunchHostIDE() error {
	if os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" {
		return nil
	}

	// Safeguard: Never kill or restart host IDE during unit test runs unless explicitly authorized
	if flag.Lookup("test.v") != nil && os.Getenv("ANTIGRAVITY_ALLOW_TEST_RELAUNCH") != "1" {
		procs, _ := s.findAntigravityProcessesInternal()
		var mainPIDs []int
		for _, p := range procs {
			lower := strings.ToLower(p.Cmdline)
			if (p.Name == "antigravity" || strings.HasSuffix(p.Name, "antigravity")) &&
				!strings.Contains(lower, "--type=") &&
				!strings.Contains(lower, "swiss") {
				mainPIDs = append(mainPIDs, p.PID)
			}
		}
		if len(mainPIDs) == 0 {
			return nil
		}
		return nil
	}

	relaunchMu.Lock()
	defer relaunchMu.Unlock()

	inj := gui.NewInjector(0)
	resumePath := inj.CaptureActiveConversationPath()

	procs, err := s.findAntigravityProcessesInternal()
	var mainPIDs []int
	if err == nil {
		for _, p := range procs {
			lower := strings.ToLower(p.Cmdline)
			if (p.Name == "antigravity" || strings.HasSuffix(p.Name, "antigravity")) &&
				!strings.Contains(lower, "--type=") &&
				!strings.Contains(lower, "swiss") {
				mainPIDs = append(mainPIDs, p.PID)
			}
		}
	}

	// When Antigravity is NOT currently running (len(mainPIDs) == 0), do NOT spawn/launch Antigravity!
	// Only restart/relaunch if Antigravity was actively running prior to switch.
	if len(mainPIDs) == 0 {
		return nil
	}

	revEngine := s.getRevivalEngine()
	revIntent, _ := revEngine.CapturePreSwitchState("desktop")

	for _, pid := range mainPIDs {
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}
	// Poll for graceful exit up to 6.5 seconds (Electron before-quit waits up to 5s for language_server)
	for i := 0; i < 65; i++ {
		time.Sleep(100 * time.Millisecond)
		anyAlive := false
		for _, pid := range mainPIDs {
			if isProcessAlive(pid) {
				anyAlive = true
				break
			}
		}
		if !anyAlive {
			break
		}
	}
	// Force terminate any remaining main processes
	for _, pid := range mainPIDs {
		if isProcessAlive(pid) {
			if proc, findErr := os.FindProcess(pid); findErr == nil {
				_ = proc.Kill()
			}
		}
	}
	time.Sleep(300 * time.Millisecond)

	// Ensure no orphaned language_server processes remain holding SQLite locks
	if lsProcs, lsErr := s.FindLanguageServerProcesses(); lsErr == nil && len(lsProcs) > 0 {
		for _, lp := range lsProcs {
			if proc, findErr := os.FindProcess(lp.PID); findErr == nil {
				_ = proc.Signal(syscall.SIGTERM)
			}
		}
		for i := 0; i < 5; i++ {
			time.Sleep(100 * time.Millisecond)
			anyLS := false
			for _, lp := range lsProcs {
				if isProcessAlive(lp.PID) {
					anyLS = true
					break
				}
			}
			if !anyLS {
				break
			}
		}
		for _, lp := range lsProcs {
			if isProcessAlive(lp.PID) {
				if proc, findErr := os.FindProcess(lp.PID); findErr == nil {
					_ = proc.Kill()
				}
			}
		}
	}

	// Settle file handles and clear any stale Chromium/Electron singleton locks & DevTools port file
	time.Sleep(500 * time.Millisecond)
	hostConfigDir := core.GetAntigravityHostConfigDir()
	_ = os.Remove(filepath.Join(hostConfigDir, "SingletonLock"))
	_ = os.Remove(filepath.Join(hostConfigDir, "SingletonSocket"))
	_ = os.Remove(filepath.Join(hostConfigDir, "SingletonCookie"))
	_ = os.Remove(filepath.Join(hostConfigDir, "DevToolsActivePort"))

	// Launch new detached process
	binPath := core.GetAntigravityBinaryPath()
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", "-a", "Antigravity")
	} else {
		cmd = exec.Command(binPath)
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		defer devNull.Close()
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}

	setDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch Antigravity (%s): %w", binPath, err)
	}
	go func() {
		_ = cmd.Wait()
	}()

	if revIntent != nil {
		go func(it *revival.RevivalIntent) {
			_ = revEngine.ExecutePostRelaunchRevival(it)
		}(revIntent)
	} else if resumePath != "" {
		go func(target string) {
			_ = gui.NewInjector(0).RestoreConversationPath(target, 30*time.Second)
		}(resumePath)
	}
	return nil
}

func isZombieProcess(pid int) bool {
	if pid <= 0 || runtime.GOOS != "linux" {
		return false
	}
	statusBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(statusBytes), "\n") {
		if strings.HasPrefix(line, "State:") {
			stateVal := strings.TrimSpace(strings.TrimPrefix(line, "State:"))
			return strings.HasPrefix(stateVal, "Z")
		}
	}
	return false
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "linux" {
		_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
		return err == nil && !isZombieProcess(pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}


