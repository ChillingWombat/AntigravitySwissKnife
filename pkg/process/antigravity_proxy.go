package process

import (
	"errors"
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
		var killErrs []error
		for _, proc := range procs {
			if err := shield.SafeKill(proc.PID, syscall.SIGTERM); err != nil {
				// Windows does not implement SIGTERM for os.Process.
				if killErr := shield.SafeKill(proc.PID, syscall.SIGKILL); killErr != nil {
					killErrs = append(killErrs, fmt.Errorf("stop Antigravity PID %d: %w", proc.PID, killErr))
				}
			}
		}
		if err := errors.Join(killErrs...); err != nil {
			return 0, err
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			remaining, findErr := shield.FindAntigravityProcesses()
			if findErr != nil {
				return 0, fmt.Errorf("verify Antigravity shutdown: %w", findErr)
			}
			if len(remaining) == 0 {
				break
			}
			if time.Now().After(deadline) {
				return 0, fmt.Errorf("Antigravity did not stop after restart request")
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
type restartNativeDeps struct {
	desktopRunning func(*Shield) (bool, error)
	relaunch       func(*Shield) error
	terminate      func(*Shield) error
}

func defaultRestartNativeDeps() restartNativeDeps {
	return restartNativeDeps{
		desktopRunning: func(shield *Shield) (bool, error) {
			procs, err := shield.FindAntigravityProcesses()
			if err != nil {
				return false, err
			}
			return len(procs) > 0, nil
		},
		relaunch: func(shield *Shield) error {
			_, err := LaunchAntigravityWithCloudCodeProxy("", true, shield)
			return err
		},
		terminate: terminateAgyLanguageServers,
	}
}

func RestartAntigravityAppsToNative(shield *Shield) error {
	if shield == nil {
		shield = NewShield(0)
	}

	// Never restart during tests unless explicitly permitted
	if (core.IsRunningTests() || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1") && os.Getenv("ANTIGRAVITY_ALLOW_TEST_RELAUNCH") != "1" {
		return nil
	}

	return restartAntigravityAppsToNative(shield, defaultRestartNativeDeps())
}

func restartAntigravityAppsToNative(shield *Shield, deps restartNativeDeps) error {
	var restartErrs []error
	running, err := deps.desktopRunning(shield)
	if err != nil {
		restartErrs = append(restartErrs, fmt.Errorf("detect running Antigravity apps: %w", err))
	}
	if running {
		if err := deps.relaunch(shield); err != nil {
			restartErrs = append(restartErrs, fmt.Errorf("relaunch Antigravity natively: %w", err))
		}
	}
	if err := deps.terminate(shield); err != nil {
		restartErrs = append(restartErrs, fmt.Errorf("terminate Antigravity language servers: %w", err))
	}
	return errors.Join(restartErrs...)
}

func terminateAgyLanguageServers(shield *Shield) error {
	entries, err := ListProcesses()
	if err != nil {
		return fmt.Errorf("list processes: %w", err)
	}
	var killErrs []error
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
			if err := shield.SafeKill(entry.PID, syscall.SIGTERM); err != nil {
				killErrs = append(killErrs, fmt.Errorf("terminate PID %d: %w", entry.PID, err))
			}
		}
	}
	return errors.Join(killErrs...)
}
