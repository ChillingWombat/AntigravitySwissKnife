package process

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	// Parent process (e.g. Electron shell) is always protected
	if ppid := os.Getppid(); ppid > 1 && pid == ppid {
		return true
	}
	// Any Swiss Knife companion or UI process is ALWAYS protected
	if runtime.GOOS == "linux" {
		exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err == nil {
			exeLower := strings.ToLower(exePath)
			if strings.Contains(exeLower, "swiss") || strings.Contains(exeLower, "antigravity-swiss") {
				return true
			}
		}
		cmdlineBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err == nil {
			cmdline := strings.ToLower(strings.ReplaceAll(string(cmdlineBytes), "\x00", " "))
			if strings.Contains(cmdline, "swiss") || strings.Contains(cmdline, "antigravity-swiss") || strings.Contains(cmdline, "antigravity-swi") {
				return true
			}
		}
		commBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err == nil {
			comm := strings.ToLower(strings.TrimSpace(string(commBytes)))
			if strings.Contains(comm, "swiss") || strings.Contains(comm, "antigravity-swi") || strings.HasPrefix(comm, "antigravity-sw") {
				return true
			}
		}
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

// isAbsoluteRoot checks if a path begins with an absolute root across platforms:
// - Unix root (/)
// - UNC root (\\ or //) including extended prefixes (\\?\ or //?/)
// - Windows drive root (^[a-zA-Z]:[/\\])
func isAbsoluteRoot(path string) bool {
	if len(path) == 0 {
		return false
	}
	if path[0] == '/' {
		return true
	}
	if len(path) >= 2 && ((path[0] == '\\' && path[1] == '\\') || (path[0] == '/' && path[1] == '/')) {
		return true
	}
	if len(path) >= 3 && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) && path[1] == ':' && (path[2] == '/' || path[2] == '\\') {
		return true
	}
	return false
}

// IsExactAntigravityExecutable evaluates whether a given process name, command, or executable path
// corresponds strictly to the official Antigravity desktop IDE application binary.
//
// Parameters:
//   - nameOrPath: The process name (e.g. from /proc/[pid]/comm), executable link (/proc/[pid]/exe),
//     or the primary command token (argv[0]).
//
// Returns:
//   - bool: true if the binary matches "Antigravity.exe" (Windows) or "antigravity" (Linux/macOS),
//     and false if it is an unrelated script, wrapper, or auxiliary command (e.g., "python -m antigravity").
func IsExactAntigravityExecutable(nameOrPath string) bool {
	clean := strings.Trim(nameOrPath, " \t\r\n\"'")
	if clean == "" {
		return false
	}
	// Reject non-space whitespace (tabs, newlines) inside command lines or paths
	if strings.ContainsAny(clean, "\t\r\n") {
		return false
	}
	// Reject directory paths ending in path separators
	if strings.HasSuffix(clean, "/") || strings.HasSuffix(clean, "\\") {
		return false
	}
	// Reject URLs (scheme://) but allow Windows drive letters with double forward slashes (e.g. C://)
	if urlIdx := strings.Index(clean, "://"); urlIdx >= 0 {
		isDriveSlashSlash := urlIdx == 1 && ((clean[0] >= 'a' && clean[0] <= 'z') || (clean[0] >= 'A' && clean[0] <= 'Z'))
		if !isDriveSlashSlash {
			return false
		}
	}
	// Reject whitespace followed by path separators or roots (e.g. "cat /usr/bin/antigravity", "rm -rf /opt/antigravity")
	if strings.Contains(clean, " /") || strings.Contains(clean, " \\") {
		return false
	}
	// Reject whitespace followed by UNC roots (e.g. "notepad.exe \\server\share\Antigravity.exe")
	if strings.Contains(clean, " //") || strings.Contains(clean, " \\\\") {
		return false
	}
	// Reject whitespace followed by relative path roots (e.g. "cat ./antigravity", "sh ../antigravity")
	if strings.Contains(clean, " ./") || strings.Contains(clean, " .\\") ||
		strings.Contains(clean, " ../") || strings.Contains(clean, " ..\\") {
		return false
	}
	// Reject mid-string Windows drive letters (e.g. "notepad.exe C:\tools\Antigravity.exe")
	if colonIdx := strings.Index(clean, ":"); colonIdx >= 0 {
		isDrive := colonIdx == 1 && ((clean[0] >= 'a' && clean[0] <= 'z') || (clean[0] >= 'A' && clean[0] <= 'Z'))
		isExtendedDrive := colonIdx == 5 && ((clean[4] >= 'a' && clean[4] <= 'z') || (clean[4] >= 'A' && clean[4] <= 'Z')) &&
			(strings.HasPrefix(clean, `\\?\`) || strings.HasPrefix(clean, `\\.\`) || strings.HasPrefix(clean, `\??\`) ||
				strings.HasPrefix(clean, `//?/`) || strings.HasPrefix(clean, `//./`))
		if !isDrive && !isExtendedDrive {
			return false
		}
	}

	// Multi-token handling when clean contains spaces
	if strings.Contains(clean, " ") {
		fields := strings.Fields(clean)
		if len(fields) > 1 {
			// An unquoted path with spaces can ONLY be an executable if it starts with an absolute path root
			if !isAbsoluteRoot(clean) {
				return false
			}

			// Reject if any executable extension is followed by a space (command invocation with arguments)
			cleanLower := strings.ToLower(clean)
			if strings.Contains(cleanLower, ".exe ") || strings.Contains(cleanLower, ".bat ") ||
				strings.Contains(cleanLower, ".cmd ") || strings.Contains(cleanLower, ".com ") {
				return false
			}

			// Reject if fields[0] ends with an executable extension
			f0Lower := strings.ToLower(fields[0])
			if strings.HasSuffix(f0Lower, ".exe") || strings.HasSuffix(f0Lower, ".bat") ||
				strings.HasSuffix(f0Lower, ".cmd") || strings.HasSuffix(f0Lower, ".com") {
				return false
			}

			// Inspect fields[0] binary name
			f0Norm := strings.ReplaceAll(fields[0], "\\", "/")
			f0Base := f0Norm
			if lastSlash := strings.LastIndex(f0Norm, "/"); lastSlash >= 0 {
				f0Base = f0Norm[lastSlash+1:]
			}
			f0BaseLower := strings.ToLower(f0Base)

			switch f0BaseLower {
			case "cat", "vim", "vi", "nano", "sh", "bash", "zsh", "python", "python3",
				"node", "docker", "curl", "wget", "git", "grep", "find", "rm", "ls",
				"kubectl", "npm", "yarn", "pnpm", "go", "cargo", "make", "notepad":
				return false
			}

			if strings.HasPrefix(f0Norm, "/bin/") || strings.HasPrefix(f0Norm, "/usr/bin/") ||
				strings.HasPrefix(f0Norm, "/sbin/") || strings.HasPrefix(f0Norm, "/usr/local/bin/") {
				if f0BaseLower != "antigravity" {
					return false
				}
			}

			// If fields[0] is already Antigravity, then len(fields) > 1 means trailing arguments
			if f0BaseLower == "antigravity" || f0BaseLower == "antigravity.exe" {
				return false
			}
		}
	}

	// Normalize all Windows backslashes to forward slashes for cross-platform matching
	normalized := strings.ReplaceAll(clean, "\\", "/")
	base := normalized
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		base = normalized[idx+1:]
	}
	baseLower := strings.ToLower(base)

	// Exclude Swiss Knife daemon companions
	if strings.Contains(baseLower, "swiss") {
		return false
	}

	// Exact matches allowed across platforms:
	// - Windows: "antigravity.exe"
	// - Linux / macOS: "antigravity"
	return baseLower == "antigravity" || baseLower == "antigravity.exe"
}

// extractArgv0 extracts the primary executable token from a command line string,
// handling quotes, leading/trailing whitespace, and unquoted paths containing spaces.
func extractArgv0(cmdline string) string {
	trimmed := strings.TrimSpace(cmdline)
	if trimmed == "" {
		return ""
	}

	// 1. Quoted executable token: "C:\Program Files\..." or 'C:\Program Files\...'
	if trimmed[0] == '"' {
		if idx := strings.IndexByte(trimmed[1:], '"'); idx >= 0 {
			return trimmed[1 : 1+idx]
		}
		return strings.Trim(trimmed, "\"")
	}
	if trimmed[0] == '\'' {
		if idx := strings.IndexByte(trimmed[1:], '\''); idx >= 0 {
			return trimmed[1 : 1+idx]
		}
		return strings.Trim(trimmed, "'")
	}

	// 2. Fast-path: first whitespace-separated token is an exact executable (e.g. /usr/bin/antigravity --arg, cat /usr/bin/antigravity)
	fields := strings.Fields(trimmed)
	if len(fields) > 0 && IsExactAntigravityExecutable(fields[0]) {
		return fields[0]
	}

	// 3. Unquoted Windows executable with spaces followed by arguments (.exe boundary)
	lower := strings.ToLower(trimmed)
	if exeIdx := strings.Index(lower, ".exe "); exeIdx > 0 {
		candidate := strings.TrimSpace(trimmed[:exeIdx+4])
		if IsExactAntigravityExecutable(candidate) {
			return candidate
		}
	}

	// 4. Unquoted executable with spaces followed by dash arguments (e.g. C:\Program Files\...\Antigravity.exe --type=...)
	if dashIdx := strings.Index(trimmed, " -"); dashIdx > 0 {
		candidate := strings.TrimSpace(trimmed[:dashIdx])
		if IsExactAntigravityExecutable(candidate) {
			return candidate
		}
	}

	// 5. Unquoted executable with spaces followed by slash arguments (e.g. C:\Program Files\...\Antigravity.exe /high)
	if slashIdx := strings.Index(trimmed, " /"); slashIdx > 0 {
		candidate := strings.TrimSpace(trimmed[:slashIdx])
		if IsExactAntigravityExecutable(candidate) {
			return candidate
		}
	}

	// 6. Direct exact match if the entire unquoted string is an absolute path to the Antigravity executable (without args)
	if isAbsoluteRoot(trimmed) && !strings.ContainsAny(trimmed, "\t\r\n") && IsExactAntigravityExecutable(trimmed) {
		return trimmed
	}

	// 7. Default fallback to first whitespace-separated token
	if len(fields) > 0 {
		return fields[0]
	}
	return trimmed
}

// FindAntigravityProcesses returns running Antigravity Desktop IDE instances.
// Platform implementations live in finder_windows.go (Toolhelp32 snapshot) and
// finder_unix.go (/proc scan).
func (s *Shield) FindAntigravityProcesses() ([]ProcessInfo, error) {
	return s.findAntigravityProcessesNative()
}

// FindAntigravityUIProcesses returns only the desktop UI process (no renderer/
// gpu/utility children, no language-server sidecars that outlive the window).
func (s *Shield) FindAntigravityUIProcesses() ([]ProcessInfo, error) {
	procs, err := s.findAntigravityProcessesInternal()
	if err != nil {
		return nil, err
	}
	out := make([]ProcessInfo, 0, len(procs))
	for _, p := range procs {
		lower := strings.ToLower(p.Cmdline)
		if strings.Contains(lower, "--type=") || isAntigravitySidecar(lower) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// isAntigravitySidecar reports whether a command line belongs to an Antigravity
// helper process (language-server sidecar) rather than the desktop UI. Sidecars
// keep running after the window is closed, so they must never be mistaken for a
// live app instance.
func isAntigravitySidecar(lowerCmdline string) bool {
	return strings.Contains(lowerCmdline, "server.cjs") ||
		strings.Contains(lowerCmdline, "sidecar_sdk") ||
		strings.Contains(lowerCmdline, "sidecar")
}

// IsAntigravityRunning returns true if an Antigravity main application process is currently running.
func (s *Shield) IsAntigravityRunning() bool {
	procs, err := s.FindAntigravityUIProcesses()
	if err != nil || len(procs) == 0 {
		return false
	}
	for _, p := range procs {
		lower := strings.ToLower(p.Cmdline)

		// Confirm exact executable match on process name or argv[0]
		isExact := IsExactAntigravityExecutable(p.Name)
		if !isExact {
			argv0 := extractArgv0(p.Cmdline)
			isExact = IsExactAntigravityExecutable(argv0)
		}
		if !isExact {
			continue
		}

		// Filter out sub-processes (renderers, zygotes, utility processes) and swiss companions
		if strings.Contains(lower, "--type=") || strings.Contains(lower, "swiss") {
			continue
		}
		return true
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

	// Orphaned sidecars (language servers) survive a closed window and hold the
	// helper ports the fresh instance needs; clear them before spawning.
	s.terminateAntigravitySidecars()

	// Clear stale Chromium/Electron singleton locks & DevTools port file if dead
	if !s.IsAntigravityRunning() {
		hostConfigDir := core.GetAntigravityHostConfigDir()
		_ = os.Remove(filepath.Join(hostConfigDir, "SingletonLock"))
		_ = os.Remove(filepath.Join(hostConfigDir, "SingletonSocket"))
		_ = os.Remove(filepath.Join(hostConfigDir, "SingletonCookie"))
		_ = os.Remove(filepath.Join(hostConfigDir, "DevToolsActivePort"))
	}

	binPath := core.GetAntigravityBinaryPath()
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", "-a", "Antigravity")
	} else {
		cmd = exec.Command(binPath)
	}

	if err := core.LaunchDetachedGUIProcess(cmd); err != nil {
		return fmt.Errorf("failed to launch Antigravity (%s): %w", binPath, err)
	}

	scheduleCDPStyleInjection()
	return nil
}

// terminateAntigravitySidecars stops leftover Antigravity helper processes so a
// fresh app instance can bind its ports. Best-effort: errors are ignored.
func (s *Shield) terminateAntigravitySidecars() {
	procs, err := s.findAntigravityProcessesInternal()
	if err != nil {
		return
	}
	for _, p := range procs {
		lower := strings.ToLower(p.Cmdline)
		if strings.Contains(lower, "--type=") || !isAntigravitySidecar(lower) {
			continue
		}
		if proc, ferr := os.FindProcess(p.PID); ferr == nil {
			_ = proc.Kill()
		}
	}
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
			if (IsExactAntigravityExecutable(p.Name) || IsExactAntigravityExecutable(extractArgv0(p.Cmdline))) &&
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
			if (IsExactAntigravityExecutable(p.Name) || IsExactAntigravityExecutable(extractArgv0(p.Cmdline))) &&
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

	// Phase 1: Graceful SIGTERM to main IDE processes so Electron triggers before-quit
	for _, pid := range mainPIDs {
		if !s.IsProtected(pid) {
			if proc, findErr := os.FindProcess(pid); findErr == nil {
				_ = proc.Signal(syscall.SIGTERM)
			}
		}
	}
	// Poll for main process graceful exit up to 6.5 seconds (Electron before-quit waits up to 5s for language_server)
	for i := 0; i < 65; i++ {
		time.Sleep(100 * time.Millisecond)
		anyAlive := false
		for _, pid := range mainPIDs {
			if !s.IsProtected(pid) && isProcessAliveMatching(pid, "antigravity") {
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
		if !s.IsProtected(pid) && isProcessAliveMatching(pid, "antigravity") {
			if proc, findErr := os.FindProcess(pid); findErr == nil {
				_ = proc.Kill()
			}
		}
	}
	time.Sleep(300 * time.Millisecond)

	// Phase 2: Give language_server up to 4.5-5.0 seconds to checkpoint SQLite (Issue 22)
	if lsProcs, lsErr := s.FindLanguageServerProcesses(); lsErr == nil && len(lsProcs) > 0 {
		for _, lp := range lsProcs {
			if !s.IsProtected(lp.PID) {
				if proc, findErr := os.FindProcess(lp.PID); findErr == nil {
					_ = proc.Signal(syscall.SIGTERM)
				}
			}
		}
		for i := 0; i < 45; i++ {
			time.Sleep(100 * time.Millisecond)
			anyLS := false
			for _, lp := range lsProcs {
				if !s.IsProtected(lp.PID) && isProcessAliveMatching(lp.PID, "language_server") {
					anyLS = true
					break
				}
			}
			if !anyLS {
				break
			}
		}
		for _, lp := range lsProcs {
			if !s.IsProtected(lp.PID) && isProcessAliveMatching(lp.PID, "language_server") {
				if proc, findErr := os.FindProcess(lp.PID); findErr == nil {
					_ = proc.Kill()
				}
			}
		}
	}

	// Phase 3: Terminate surviving Electron child processes (Issue 21)
	if activeProcs, aErr := s.findAntigravityProcessesInternal(); aErr == nil {
		var survivingChildren []int
		for _, p := range activeProcs {
			lower := strings.ToLower(p.Cmdline)
			if !s.IsProtected(p.PID) && !strings.Contains(lower, "swiss") && isProcessAliveMatching(p.PID, "antigravity") {
				survivingChildren = append(survivingChildren, p.PID)
			}
		}
		if len(survivingChildren) > 0 {
			for _, pid := range survivingChildren {
				if !s.IsProtected(pid) {
					if proc, findErr := os.FindProcess(pid); findErr == nil {
						_ = proc.Signal(syscall.SIGTERM)
					}
				}
			}
			for i := 0; i < 15; i++ {
				time.Sleep(100 * time.Millisecond)
				anyChild := false
				for _, pid := range survivingChildren {
					if !s.IsProtected(pid) && isProcessAliveMatching(pid, "antigravity") {
						anyChild = true
						break
					}
				}
				if !anyChild {
					break
				}
			}
			for _, pid := range survivingChildren {
				if !s.IsProtected(pid) && isProcessAliveMatching(pid, "antigravity") {
					if proc, findErr := os.FindProcess(pid); findErr == nil {
						_ = proc.Kill()
					}
				}
			}
		}
	}

	// Phase 4: Verify all Antigravity and language_server processes have exited before unlinking singleton files (Issue 24)
	for i := 0; i < 20; i++ {
		procsLeft, _ := s.findAntigravityProcessesInternal()
		lsLeft, _ := s.FindLanguageServerProcesses()
		alive := false
		for _, p := range procsLeft {
			if !s.IsProtected(p.PID) && isProcessAliveMatching(p.PID, "antigravity") {
				alive = true
				break
			}
		}
		if !alive {
			for _, lp := range lsLeft {
				if !s.IsProtected(lp.PID) && isProcessAliveMatching(lp.PID, "language_server") {
					alive = true
					break
				}
			}
		}
		if !alive {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Settle file handles and clear any stale Chromium/Electron singleton locks & DevTools port file
	time.Sleep(300 * time.Millisecond)
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

	scheduleCDPStyleInjection()
	return nil
}

func scheduleCDPStyleInjection() {
	go func() {
		inj := gui.NewInjector(0)
		for i := 0; i < 30; i++ {
			time.Sleep(500 * time.Millisecond)
			if port, err := inj.FindDevToolsPort(); err == nil && port > 0 {
				if store, sErr := gui.NewStore(""); sErr == nil {
					cfg := store.GetConfig()
					if cfg.Enabled {
						_, _ = inj.ApplyConfig(&cfg)
					}
				}
				break
			}
		}
	}()
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

// isProcessAlive checks whether a process is running without relying on proc.Signal(0).
// On Windows, isSystemProcessAlive delegates to OpenProcess and GetExitCodeProcess (liveness_windows.go).
func isProcessAlive(pid int) bool {
	return isProcessAliveMatching(pid)
}

func isProcessAliveMatching(pid int, keywords ...string) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "linux" {
		procDir := fmt.Sprintf("/proc/%d", pid)
		if _, err := os.Stat(procDir); err != nil {
			return false
		}
		if isZombieProcess(pid) {
			return false
		}
		if len(keywords) > 0 && pid != os.Getpid() {
			exePath, _ := os.Readlink(filepath.Join(procDir, "exe"))
			exeLower := strings.ToLower(exePath)
			if strings.Contains(exeLower, "swiss") || strings.Contains(exeLower, "antigravity-swiss") {
				return false
			}
			commBytes, _ := os.ReadFile(filepath.Join(procDir, "comm"))
			commLower := strings.ToLower(strings.TrimSpace(string(commBytes)))
			if strings.Contains(commLower, "swiss") || strings.Contains(commLower, "antigravity-swi") || strings.HasPrefix(commLower, "antigravity-sw") {
				return false
			}
			cmdlineBytes, err := os.ReadFile(filepath.Join(procDir, "cmdline"))
			if err != nil {
				return false
			}
			cmdline := strings.ToLower(strings.ReplaceAll(string(cmdlineBytes), "\x00", " "))
			if strings.Contains(cmdline, "swiss") || strings.Contains(cmdline, "antigravity-swiss") || strings.Contains(cmdline, "antigravity-swi") {
				return false
			}
			matched := false
			for _, kw := range keywords {
				kwLower := strings.ToLower(kw)
				if kwLower == "antigravity" {
					argv0 := extractArgv0(cmdline)
					if IsExactAntigravityExecutable(commLower) || IsExactAntigravityExecutable(exeLower) || IsExactAntigravityExecutable(argv0) {
						matched = true
						break
					}
				} else {
					if strings.Contains(cmdline, kwLower) || strings.Contains(commLower, kwLower) {
						matched = true
						break
					}
				}
			}
			return matched
		}
		return true
	}
	return isSystemProcessAlive(pid)
}


