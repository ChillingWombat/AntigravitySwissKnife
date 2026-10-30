package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// The Antigravity desktop app's language server hardcodes its Cloud Code endpoint
// as an argv flag, but honours CLOUD_CODE_URL from its environment (the language
// server logs "Overriding CloudCodeServerURL via CLOUD_CODE_URL environment
// variable"). Launching the app ourselves lets us point it at the local proxy so
// custom models can be served without patching any app file.

// CleanNativeEnvironment filters out CLOUD_CODE_URL and AGY_LLM_GATEWAY_* variables
// from an environment slice to ensure a clean, native Cloud Code execution path.
func CleanNativeEnvironment(baseEnv []string) []string {
	var env []string
	for _, kv := range baseEnv {
		if strings.HasPrefix(kv, "CLOUD_CODE_URL=") ||
			strings.HasPrefix(kv, "AGY_LLM_GATEWAY_URL=") ||
			strings.HasPrefix(kv, "AGY_LLM_GATEWAY_API_KEY=") ||
			strings.HasPrefix(kv, "AGY_LLM_GATEWAY_MODELS=") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

// antigravityLaunchSpec builds the command for the installed desktop app.
func antigravityLaunchSpec(endpoint string) (*exec.Cmd, error) {
	bin := core.GetAntigravityBinaryPath()
	if bin == "" {
		return nil, fmt.Errorf("Antigravity desktop app not found")
	}
	if info, err := os.Stat(bin); err != nil || info.IsDir() {
		return nil, fmt.Errorf("Antigravity desktop app not executable at %s", filepath.Clean(bin))
	}
	cmd := exec.Command(bin)
	env := CleanNativeEnvironment(os.Environ())
	if endpoint != "" {
		env = append(env, "CLOUD_CODE_URL="+strings.TrimRight(endpoint, "/"))
	}
	cmd.Env = env
	return cmd, nil
}

// LaunchAntigravityWithCloudCodeProxy starts the desktop app with CLOUD_CODE_URL
// set to the local proxy. With restart=true an already running instance is
// terminated first (the app is single-instance, so it would otherwise ignore the
// new environment).
func LaunchAntigravityWithCloudCodeProxy(endpoint string, restart bool, shield *Shield) (int, error) {
	if shield == nil {
		shield = NewShield(0)
	}
	procs, err := shield.FindAntigravityProcesses()
	if err == nil && len(procs) > 0 {
		if !restart {
			return procs[0].PID, fmt.Errorf("Antigravity is already running (PID %d); restart it to pick up the Cloud Code proxy", procs[0].PID)
		}
		for _, proc := range procs {
			if err := shield.SafeKill(proc.PID, syscall.SIGTERM); err != nil {
				// Windows does not implement SIGTERM for os.Process.
				_ = shield.SafeKill(proc.PID, syscall.SIGKILL)
			}
		}
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			remaining, findErr := shield.FindAntigravityProcesses()
			if findErr != nil || len(remaining) == 0 {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	cmd, err := antigravityLaunchSpec(endpoint)
	if err != nil {
		return 0, err
	}
	if err := core.LaunchDetachedGUIProcess(cmd); err != nil {
		return 0, fmt.Errorf("launch Antigravity: %w", err)
	}
	return cmd.Process.Pid, nil
}

// RestartAntigravityAppsToNative detects any running Antigravity IDE instances
// and agy language servers, terminates them, and if the desktop app was running,
// relaunches it cleanly with native Google Cloud Code configuration (no proxy variables).
func RestartAntigravityAppsToNative(shield *Shield) error {
	if shield == nil {
		shield = NewShield(0)
	}

	// Never restart during tests unless explicitly permitted
	if (core.IsRunningTests() || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1") && os.Getenv("ANTIGRAVITY_ALLOW_TEST_RELAUNCH") != "1" {
		return nil
	}

	// 1. Check if Antigravity Desktop IDE is currently running
	procs, err := shield.FindAntigravityProcesses()
	wasDesktopRunning := (err == nil && len(procs) > 0)

	// 2. If desktop app was running, cleanly restart it with empty endpoint (native Cloud Code)
	if wasDesktopRunning {
		_, _ = LaunchAntigravityWithCloudCodeProxy("", true, shield)
	}

	// 3. Terminate any active agy language servers (e.g. agy --hub spawned by VS Code extension)
	// so the editor/supervisor respawns them with clean native configuration.
	terminateAgyLanguageServers(shield)

	return nil
}

func terminateAgyLanguageServers(shield *Shield) {
	entries, err := ListProcesses()
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.PID <= 0 || shield.IsProtected(entry.PID) || isZombieProcess(entry.PID) {
			continue
		}
		cmdline := strings.ToLower(entry.Cmdline)
		comm := strings.ToLower(entry.Comm)
		// Match agy --hub or language_server instances
		isAgyHub := (strings.Contains(cmdline, "agy") || strings.Contains(comm, "agy")) && strings.Contains(cmdline, "--hub")
		isLS := strings.Contains(cmdline, "language_server") && (strings.Contains(cmdline, "antigravity") || strings.Contains(cmdline, "subclient_type hub"))
		if isAgyHub || isLS {
			_ = shield.SafeKill(entry.PID, syscall.SIGTERM)
		}
	}
}
