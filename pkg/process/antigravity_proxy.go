package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// The Antigravity desktop app's language server hardcodes its Cloud Code endpoint
// as an argv flag, but honours CLOUD_CODE_URL from its environment (the language
// server logs "Overriding CloudCodeServerURL via CLOUD_CODE_URL environment
// variable"). Launching the app ourselves lets us point it at the local proxy so
// custom models can be served without patching any app file.

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
	env := os.Environ()
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
			_ = shield.SafeKill(proc.PID, 0)
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
