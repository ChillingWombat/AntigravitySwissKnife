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

// TunnelManager coordinates zero-configuration remote access for Gemini Bridge.
type TunnelManager struct {
	mu         sync.RWMutex
	status     TunnelStatus
	cmd        *exec.Cmd
	cancelFunc context.CancelFunc
	localPort  int
	logs       []string
}

// NewTunnelManager initializes a tunnel manager bound to the local daemon port.
func NewTunnelManager(port int) *TunnelManager {
	if port <= 0 {
		port = 8765
	}
	return &TunnelManager{
		localPort: port,
		status: TunnelStatus{
			Active:     false,
			Type:       "loopback",
			GatewayURL: fmt.Sprintf("http://127.0.0.1:%d/api/bridge/mcp", port),
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
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.appendLogLocked(msg)
}

func (tm *TunnelManager) appendLogLocked(msg string) {
	entry := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
	tm.logs = append(tm.logs, entry)
	if len(tm.logs) > 40 {
		tm.logs = tm.logs[len(tm.logs)-40:]
	}
	tm.status.Logs = tm.logs
}

// Start begins a remote tunnel with zero configuration.
func (tm *TunnelManager) Start(tunnelType string, customURL string) (TunnelStatus, error) {
	tm.Stop()

	tm.mu.Lock()
	defer tm.mu.Unlock()

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
			return tm.status, fmt.Errorf("custom URL is required when choosing custom tunnel type")
		}
		if !strings.HasSuffix(customURL, "/api/bridge/mcp") {
			customURL = strings.TrimRight(customURL, "/") + "/api/bridge/mcp"
		}
		tm.status = TunnelStatus{
			Active:     true,
			Type:       "custom",
			PublicURL:  customURL,
			GatewayURL: customURL,
			StartedAt:  time.Now(),
		}
		tm.appendLogLocked(fmt.Sprintf("Custom remote endpoint configured: %s", customURL))
		return tm.status, nil
	case "loopback":
		tm.status = TunnelStatus{
			Active:     true,
			Type:       "loopback",
			PublicURL:  fmt.Sprintf("http://127.0.0.1:%d", tm.localPort),
			GatewayURL: fmt.Sprintf("http://127.0.0.1:%d/api/bridge/mcp", tm.localPort),
			StartedAt:  time.Now(),
		}
		tm.appendLogLocked("Loopback intranet endpoint configured (127.0.0.1)")
		return tm.status, nil
	default:
		return tm.status, fmt.Errorf("unknown tunnel type: %s", tunnelType)
	}
}

func (tm *TunnelManager) startQuickTunnel() (TunnelStatus, error) {
	bin := LocateCloudflared()
	if bin == "" {
		err := fmt.Errorf("cloudflared binary not found; quick tunnel unavailable")
		tm.status.Error = err.Error()
		return tm.status, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	tm.cancelFunc = cancel

	cmd := exec.CommandContext(ctx, bin, "tunnel", "--url", fmt.Sprintf("http://127.0.0.1:%d", tm.localPort))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return tm.status, fmt.Errorf("pipe stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return tm.status, fmt.Errorf("pipe stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return tm.status, fmt.Errorf("start cloudflared: %w", err)
	}
	tm.cmd = cmd

	tm.status = TunnelStatus{
		Active:    false,
		Type:      "quick",
		StartedAt: time.Now(),
	}

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
	case discoveredURL := <-urlChan:
		gatewayURL := discoveredURL + "/api/bridge/mcp"
		tm.status.Active = true
		tm.status.PublicURL = discoveredURL
		tm.status.GatewayURL = gatewayURL
		tm.status.Error = ""
		tm.appendLogLocked(fmt.Sprintf("✓ Quick Tunnel established: %s", gatewayURL))
		return tm.status, nil
	case waitErr := <-errChan:
		tm.status.Active = false
		tm.status.Error = fmt.Sprintf("Tunnel process terminated: %v", waitErr)
		return tm.status, waitErr
	case <-time.After(12 * time.Second):
		// Fallback check: query logs if URL regex missed it
		for _, l := range tm.logs {
			if match := urlRegex.FindString(l); match != "" {
				gatewayURL := match + "/api/bridge/mcp"
				tm.status.Active = true
				tm.status.PublicURL = match
				tm.status.GatewayURL = gatewayURL
				return tm.status, nil
			}
		}
		tm.status.Active = false
		tm.status.Error = "Timed out waiting for public HTTPS quick tunnel URL"
		return tm.status, fmt.Errorf("quick tunnel timeout")
	}
}

func (tm *TunnelManager) startTailscaleTunnel() (TunnelStatus, error) {
	tsBin := LocateTailscale()
	if tsBin == "" {
		err := fmt.Errorf("tailscale binary not found on PATH or standard locations")
		tm.status.Error = err.Error()
		return tm.status, err
	}

	// 1. Try tailscale funnel or status
	out, err := exec.Command(tsBin, "ip", "-4").Output()
	if err != nil {
		err = fmt.Errorf("tailscale not connected or logged in (%v)", err)
		tm.status.Error = err.Error()
		return tm.status, err
	}
	tsIP := strings.TrimSpace(string(out))
	if tsIP == "" {
		err = fmt.Errorf("tailscale returned empty IP; please run 'tailscale up'")
		tm.status.Error = err.Error()
		return tm.status, err
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

	var gatewayURL string
	var publicURL string
	if magicDNS != "" {
		publicURL = fmt.Sprintf("http://%s:%d", magicDNS, tm.localPort)
		gatewayURL = fmt.Sprintf("http://%s:%d/api/bridge/mcp", magicDNS, tm.localPort)
	} else {
		publicURL = fmt.Sprintf("http://%s:%d", tsIP, tm.localPort)
		gatewayURL = fmt.Sprintf("http://%s:%d/api/bridge/mcp", tsIP, tm.localPort)
	}

	tm.status = TunnelStatus{
		Active:     true,
		Type:       "tailscale",
		PublicURL:  publicURL,
		GatewayURL: gatewayURL,
		StartedAt:  time.Now(),
	}
	tm.appendLogLocked(fmt.Sprintf("Tailscale network gateway active: %s", gatewayURL))
	return tm.status, nil
}

// Stop cleanly terminates any running tunnel subprocess.
func (tm *TunnelManager) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.cancelFunc != nil {
		tm.cancelFunc()
		tm.cancelFunc = nil
	}
	if tm.cmd != nil && tm.cmd.Process != nil {
		_ = tm.cmd.Process.Kill()
		tm.cmd = nil
	}
	tm.status.Active = false
	tm.status.Type = "loopback"
	tm.status.GatewayURL = fmt.Sprintf("http://127.0.0.1:%d/api/bridge/mcp", tm.localPort)
	tm.status.PublicURL = fmt.Sprintf("http://127.0.0.1:%d", tm.localPort)
	tm.status.Error = ""
	tm.appendLogLocked("Remote tunnel stopped; returned to loopback mode.")
}

// GetStatus returns the current tunnel status snapshot.
func (tm *TunnelManager) GetStatus() TunnelStatus {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	res := tm.status
	logsCopy := make([]string, len(tm.logs))
	copy(logsCopy, tm.logs)
	res.Logs = logsCopy
	return res
}
