package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// TunnelStatus represents current remote connectivity state.
type TunnelStatus struct {
	Active     bool      `json:"active"`
	Type       string    `json:"type"` // "quick" | "tailscale" | "loopback" | "custom"
	PublicURL  string    `json:"public_url,omitempty"`
	GatewayURL string    `json:"gateway_url"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	Logs       []string  `json:"logs,omitempty"`
}

// TunnelManager coordinates zero-configuration remote access for Gemini Bridge and Remote Control.
type TunnelManager struct {
	mu           sync.RWMutex
	logsMu       sync.Mutex
	status       TunnelStatus
	cmd          *exec.Cmd
	cancelFunc   context.CancelFunc
	localPort    int
	endpointPath string
	logs         []string
}

// NewTunnelManager initializes a tunnel manager bound to the local daemon port and endpoint path.
func NewTunnelManager(port int, endpointPath ...string) *TunnelManager {
	if port <= 0 {
		port = 8765
	}
	path := "/api/bridge/mcp"
	if len(endpointPath) > 0 && endpointPath[0] != "" {
		path = endpointPath[0]
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return &TunnelManager{
		localPort:    port,
		endpointPath: path,
		status: TunnelStatus{
			Active:     false,
			Type:       "loopback",
			GatewayURL: fmt.Sprintf("http://127.0.0.1:%d%s", port, path),
			PublicURL:  fmt.Sprintf("http://127.0.0.1:%d", port),
		},
		logs: make([]string, 0),
	}
}

// LocateCloudflared finds an available cloudflared binary on the host.
func LocateCloudflared() string {
	candidates := []string{
		os.Getenv("CLOUDFLARED_PATH"),
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "cloudflared"),
		"bin/cloudflared",
		filepath.Join(os.Getenv("HOME"), ".local", "share", "antigravity-swiss-knife", "app", "resources", "bin", "cloudflared"),
	}
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}
	return ""
}

// LocateTailscale finds an available tailscale binary on the host.
func LocateTailscale() string {
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	candidates := []string{
		"/usr/bin/tailscale",
		"/usr/local/bin/tailscale",
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "tailscale"),
	}
	for _, cand := range candidates {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}
	return ""
}

func (tm *TunnelManager) appendLog(msg string) {
	tm.logsMu.Lock()
	defer tm.logsMu.Unlock()
	entry := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
	tm.logs = append(tm.logs, entry)
	if len(tm.logs) > 40 {
		tm.logs = tm.logs[len(tm.logs)-40:]
	}
}

func (tm *TunnelManager) appendLogLocked(msg string) {
	tm.appendLog(msg)
}

// Start begins a remote tunnel with zero configuration.
func (tm *TunnelManager) Start(tunnelType string, customURL string) (TunnelStatus, error) {
	tm.Stop()

	if tunnelType == "" || tunnelType == "quick" {
		tunnelType = "quick"
	}

	switch tunnelType {
	case "quick":
		return tm.startQuickTunnel()
	case "tailscale":
		return tm.startTailscaleTunnel()
	case "custom":
		if customURL == "" {
			tm.mu.RLock()
			st := tm.status
			tm.mu.RUnlock()
			return st, fmt.Errorf("custom URL is required when choosing custom tunnel type")
		}
		path := tm.endpointPath
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if !strings.HasSuffix(customURL, path) {
			customURL = strings.TrimRight(customURL, "/") + path
		}
		tm.mu.Lock()
		tm.status = TunnelStatus{
			Active:     true,
			Type:       "custom",
			PublicURL:  customURL,
			GatewayURL: customURL,
			StartedAt:  time.Now(),
		}
		st := tm.status
		tm.mu.Unlock()
		tm.appendLog(fmt.Sprintf("Custom remote endpoint configured: %s", customURL))
		return st, nil
	case "loopback":
		path := tm.endpointPath
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		tm.mu.Lock()
		tm.status = TunnelStatus{
			Active:     true,
			Type:       "loopback",
			PublicURL:  fmt.Sprintf("http://127.0.0.1:%d", tm.localPort),
			GatewayURL: fmt.Sprintf("http://127.0.0.1:%d%s", tm.localPort, path),
			StartedAt:  time.Now(),
		}
		st := tm.status
		tm.mu.Unlock()
		tm.appendLog("Loopback intranet endpoint configured (127.0.0.1)")
		return st, nil
	default:
		tm.mu.RLock()
		st := tm.status
		tm.mu.RUnlock()
		return st, fmt.Errorf("unknown tunnel type: %s", tunnelType)
	}
}

func (tm *TunnelManager) startQuickTunnel() (TunnelStatus, error) {
	bin := LocateCloudflared()
	if bin == "" {
		err := fmt.Errorf("cloudflared binary not found; quick tunnel unavailable")
		tm.mu.Lock()
		tm.status.Error = err.Error()
		st := tm.status
		tm.mu.Unlock()
		return st, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, bin, "tunnel", "--url", fmt.Sprintf("http://127.0.0.1:%d", tm.localPort))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		tm.mu.Lock()
		tm.status.Error = err.Error()
		st := tm.status
		tm.mu.Unlock()
		return st, fmt.Errorf("pipe stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		tm.mu.Lock()
		tm.status.Error = err.Error()
		st := tm.status
		tm.mu.Unlock()
		return st, fmt.Errorf("pipe stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		tm.mu.Lock()
		tm.status.Error = err.Error()
		st := tm.status
		tm.mu.Unlock()
		return st, fmt.Errorf("start cloudflared: %w", err)
	}

	tm.mu.Lock()
	tm.cmd = cmd
	tm.cancelFunc = cancel
	tm.status = TunnelStatus{
		Active:    false,
		Type:      "quick",
		StartedAt: time.Now(),
	}
	tm.mu.Unlock()

	urlChan := make(chan string, 1)
	errChan := make(chan error, 1)

	urlRegex := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

	scannerFunc := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			tm.appendLog(line)
			if match := urlRegex.FindString(line); match != "" {
				select {
				case urlChan <- match:
				default:
				}
			}
		}
	}

	go scannerFunc(stdout)
	go scannerFunc(stderr)

	go func() {
		if waitErr := cmd.Wait(); waitErr != nil && ctx.Err() == nil {
			select {
			case errChan <- waitErr:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		killAndReap(cmd)
		tm.mu.Lock()
		tm.status.Active = false
		tm.status.Error = "Tunnel cancelled"
		st := tm.status
		tm.mu.Unlock()
		return st, ctx.Err()
	case discoveredURL := <-urlChan:
		path := tm.endpointPath
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		gatewayURL := strings.TrimRight(discoveredURL, "/") + path

		tm.mu.Lock()
		tm.status.Active = true
		tm.status.PublicURL = discoveredURL
		tm.status.GatewayURL = gatewayURL
		tm.status.Error = ""
		st := tm.status
		tm.mu.Unlock()

		tm.appendLog(fmt.Sprintf("✓ Quick Tunnel established: %s", gatewayURL))
		return st, nil
	case waitErr := <-errChan:
		cancel()
		killAndReap(cmd)
		tm.mu.Lock()
		tm.status.Active = false
		tm.status.Error = fmt.Sprintf("Tunnel process terminated: %v", waitErr)
		st := tm.status
		tm.mu.Unlock()
		return st, waitErr
	case <-time.After(12 * time.Second):
		// Fallback check: query logs if URL regex missed it
		tm.logsMu.Lock()
		logsCopy := make([]string, len(tm.logs))
		copy(logsCopy, tm.logs)
		tm.logsMu.Unlock()

		for _, l := range logsCopy {
			if match := urlRegex.FindString(l); match != "" {
				path := tm.endpointPath
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}
				gatewayURL := strings.TrimRight(match, "/") + path

				tm.mu.Lock()
				tm.status.Active = true
				tm.status.PublicURL = match
				tm.status.GatewayURL = gatewayURL
				tm.status.Error = ""
				st := tm.status
				tm.mu.Unlock()
				return st, nil
			}
		}

		cancel()
		killAndReap(cmd)
		tm.mu.Lock()
		tm.status.Active = false
		tm.status.Error = "Timed out waiting for public HTTPS quick tunnel URL"
		st := tm.status
		tm.mu.Unlock()
		return st, fmt.Errorf("quick tunnel timeout")
	}
}

func killAndReap(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		for i := 0; i < 50; i++ {
			if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func (tm *TunnelManager) startTailscaleTunnel() (TunnelStatus, error) {
	tsBin := LocateTailscale()
	if tsBin == "" {
		err := fmt.Errorf("tailscale binary not found on PATH or standard locations")
		tm.mu.Lock()
		tm.status.Error = err.Error()
		st := tm.status
		tm.mu.Unlock()
		return st, err
	}

	// 1. Try tailscale funnel or status
	out, err := exec.Command(tsBin, "ip", "-4").Output()
	if err != nil {
		err = fmt.Errorf("tailscale not connected or logged in (%v)", err)
		tm.mu.Lock()
		tm.status.Error = err.Error()
		st := tm.status
		tm.mu.Unlock()
		return st, err
	}
	tsIP := strings.TrimSpace(string(out))
	if tsIP == "" {
		err := fmt.Errorf("tailscale returned empty IP; please run 'tailscale up'")
		tm.mu.Lock()
		tm.status.Error = err.Error()
		st := tm.status
		tm.mu.Unlock()
		return st, err
	}

	// 2. Try getting MagicDNS domain
	statusOut, _ := exec.Command(tsBin, "status", "--json").Output()
	var magicDNS string
	if len(statusOut) > 0 && strings.Contains(string(statusOut), "Self") {
		var st struct {
			Self struct {
				DNSName string `json:"DNSName"`
			} `json:"Self"`
		}
		if jsonErr := json.Unmarshal(statusOut, &st); jsonErr == nil && st.Self.DNSName != "" {
			magicDNS = strings.TrimSuffix(st.Self.DNSName, ".")
		}
	}

	path := tm.endpointPath
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	var gatewayURL string
	var publicURL string
	if magicDNS != "" {
		publicURL = fmt.Sprintf("http://%s:%d", magicDNS, tm.localPort)
		gatewayURL = fmt.Sprintf("http://%s:%d%s", magicDNS, tm.localPort, path)
	} else {
		publicURL = fmt.Sprintf("http://%s:%d", tsIP, tm.localPort)
		gatewayURL = fmt.Sprintf("http://%s:%d%s", tsIP, tm.localPort, path)
	}

	tm.mu.Lock()
	tm.status = TunnelStatus{
		Active:     true,
		Type:       "tailscale",
		PublicURL:  publicURL,
		GatewayURL: gatewayURL,
		StartedAt:  time.Now(),
	}
	st := tm.status
	tm.mu.Unlock()

	tm.appendLog(fmt.Sprintf("Tailscale network gateway active: %s", gatewayURL))
	return st, nil
}

// Stop cleanly terminates any running tunnel subprocess.
func (tm *TunnelManager) Stop() {
	tm.mu.Lock()
	if tm.cancelFunc != nil {
		tm.cancelFunc()
		tm.cancelFunc = nil
	}
	if tm.cmd != nil && tm.cmd.Process != nil {
		_ = tm.cmd.Process.Kill()
		tm.cmd = nil
	}
	path := tm.endpointPath
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	tm.status.Active = false
	tm.status.Type = "loopback"
	tm.status.GatewayURL = fmt.Sprintf("http://127.0.0.1:%d%s", tm.localPort, path)
	tm.status.PublicURL = fmt.Sprintf("http://127.0.0.1:%d", tm.localPort)
	tm.status.Error = ""
	tm.mu.Unlock()

	tm.appendLog("Remote tunnel stopped; returned to loopback mode.")
}

// GetStatus returns the current tunnel status snapshot.
func (tm *TunnelManager) GetStatus() TunnelStatus {
	tm.mu.RLock()
	res := tm.status
	tm.mu.RUnlock()

	tm.logsMu.Lock()
	logsCopy := make([]string, len(tm.logs))
	copy(logsCopy, tm.logs)
	tm.logsMu.Unlock()

	res.Logs = logsCopy
	return res
}
