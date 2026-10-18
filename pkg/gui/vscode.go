package gui

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// FindVSCodeBinaryPath locates the VS Code executable to launch with a debug port.
// Returns "" when no installation is found. VSCODE_BIN_PATH overrides detection.
func FindVSCodeBinaryPath() string {
	if p := strings.TrimSpace(os.Getenv("VSCODE_BIN_PATH")); p != "" {
		return p
	}

	switch runtime.GOOS {
	case "windows":
		var candidates []string
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			candidates = append(candidates,
				filepath.Join(local, "Programs", "Microsoft VS Code", "Code.exe"),
				filepath.Join(local, "Programs", "Microsoft VS Code Insiders", "Code - Insiders.exe"),
			)
		}
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			candidates = append(candidates,
				filepath.Join(pf, "Microsoft VS Code", "Code.exe"),
				filepath.Join(pf, "Microsoft VS Code Insiders", "Code - Insiders.exe"),
			)
		}
		for _, c := range candidates {
			if fileExists(c) {
				return c
			}
		}
	case "darwin":
		candidates := []string{
			"/Applications/Visual Studio Code.app/Contents/MacOS/Electron",
			"/Applications/Visual Studio Code - Insiders.app/Contents/MacOS/Electron",
		}
		for _, c := range candidates {
			if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
				return c
			}
		}
	default:
		for _, name := range []string{"code", "code-insiders"} {
			if p, err := exec.LookPath(name); err == nil && p != "" {
				return p
			}
		}
	}
	return ""
}

// IsLoopbackPortFree reports whether 127.0.0.1:port can be bound right now.
func IsLoopbackPortFree(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// PickFreeLoopbackPort asks the OS for an unused loopback port.
func PickFreeLoopbackPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to allocate a free loopback port: %w", err)
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address %v", ln.Addr())
	}
	return addr.Port, nil
}

// ResolveVSCodeDebugPort returns the port VS Code should be launched with: the
// configured port while it is still free or already serving CDP (an instance
// launched earlier with the same port), otherwise a freshly picked free port.
// (The port can still be taken between this check and VS Code binding it, in
// which case VS Code fails to open DevTools and injection stays a no-op.)
func ResolveVSCodeDebugPort(configured int) (int, error) {
	if configured > 0 && (IsLoopbackPortFree(configured) || isCDPEndpointLive(configured)) {
		return configured, nil
	}
	return PickFreeLoopbackPort()
}

// isCDPEndpointLive reports whether something answers the Chrome DevTools Protocol
// on the loopback port. Electron leaves a stale DevToolsActivePort file behind
// after the app exits, so every port read from disk must be verified.
func isCDPEndpointLive(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	client := &http.Client{Timeout: 600 * time.Millisecond}
	for _, path := range []string{"/json/version", "/json"} {
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, path))
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
	}
	return false
}

// withoutElectronRunAsNode drops an inherited ELECTRON_RUN_AS_NODE from the child
// environment. With that variable set Electron starts as a headless Node
// interpreter: VS Code then rejects --remote-debugging-port as a bad option and
// exits without ever showing a window.
func withoutElectronRunAsNode(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		if strings.HasPrefix(strings.ToUpper(kv), "ELECTRON_RUN_AS_NODE=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// LaunchVSCode starts VS Code with --remote-debugging-port so the daemon can
// attach over CDP, optionally opening a folder.
//
// It launches through core.LaunchDetachedGUIProcess: the hidden-console helpers
// set STARTF_USESHOWWINDOW/SW_HIDE and Chromium would start tray-only with no
// visible window. VS Code reuses an already-running instance, so the debug flag
// only takes effect on a cold start.
func LaunchVSCode(folder string) (*ApplyResult, error) {
	binPath := FindVSCodeBinaryPath()
	if binPath == "" {
		return &ApplyResult{
			Success: false,
			Message: "VS Code executable not found (set VSCODE_BIN_PATH to override detection)",
		}, fmt.Errorf("VS Code executable not found")
	}

	cfg, err := core.LoadConfig()
	if err != nil || cfg == nil {
		cfg = core.DefaultConfig()
	}
	port, err := ResolveVSCodeDebugPort(cfg.GetVSCodeDebugPort())
	if err != nil {
		return &ApplyResult{Success: false, Message: err.Error()}, err
	}
	if port != cfg.GetVSCodeDebugPort() {
		_ = cfg.SetVSCodeDebugPort(port)
	}

	args := []string{fmt.Sprintf("--remote-debugging-port=%d", port)}
	if folder = strings.TrimSpace(folder); folder != "" {
		args = append(args, folder)
	}

	cmd := exec.Command(binPath, args...)
	cmd.Env = withoutElectronRunAsNode(os.Environ())
	if err := core.LaunchDetachedGUIProcess(cmd); err != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to launch VS Code (%s): %v", binPath, err),
		}, fmt.Errorf("failed to launch VS Code (%s): %w", binPath, err)
	}

	return &ApplyResult{
		Success: true,
		Port:    port,
		Message: fmt.Sprintf("VS Code launch initiated with CDP debugging on port %d (an already-running VS Code instance ignores the flag; quit it first to attach)", port),
	}, nil
}
