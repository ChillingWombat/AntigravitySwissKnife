package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// DefaultKeepAliveInterval is the interval between background keep-alive and validity probes (4 minutes).
const DefaultKeepAliveInterval = 4 * time.Minute

// TunnelStatus represents current remote connectivity state.
type TunnelStatus struct {
	Active     bool      `json:"active"`
	Type       string    `json:"type"` // "quick" | "tailscale" | "custom" | "" (not exposed)
	PublicURL  string    `json:"public_url,omitempty"`
	GatewayURL string    `json:"gateway_url"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	Logs       []string  `json:"logs,omitempty"`
}

// TunnelManager coordinates zero-configuration remote access for Gemini Bridge and Remote Control.
type TunnelManager struct {
	mu                sync.RWMutex
	logsMu            sync.Mutex
	status            TunnelStatus
	cmd               *exec.Cmd
	tsListener        net.Listener
	cancelFunc        context.CancelFunc
	keepAliveCancel   context.CancelFunc
	keepAliveInterval time.Duration
	httpClient        *http.Client
	onAutoStop        func()
	localPort         int
	endpointPath      string
	logs              []string
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
		localPort:         port,
		endpointPath:      path,
		keepAliveInterval: DefaultKeepAliveInterval,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
		status: TunnelStatus{
			// Issue #120: there is no loopback fallback endpoint. The bridge is simply
			// "not exposed" until a real tunnel provides an external URL.
			Active: false,
			Type:   "",
		},
		logs: make([]string, 0),
	}
}

// SetKeepAliveInterval overrides the keep-alive and health probe interval.
func (tm *TunnelManager) SetKeepAliveInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	tm.mu.Lock()
	tm.keepAliveInterval = d
	if tm.status.Active {
		tm.startKeepAliveLoopLocked()
	}
	tm.mu.Unlock()
}

// SetHTTPClient overrides the HTTP client used for keep-alive and health probes.
func (tm *TunnelManager) SetHTTPClient(client *http.Client) {
	if client == nil {
		return
	}
	tm.mu.Lock()
	tm.httpClient = client
	tm.mu.Unlock()
}

// SetOnAutoStop registers a callback invoked whenever an active tunnel is invalidated and stopped automatically.
func (tm *TunnelManager) SetOnAutoStop(fn func()) {
	tm.mu.Lock()
	tm.onAutoStop = fn
	tm.mu.Unlock()
}

func (tm *TunnelManager) startKeepAliveLoopLocked() {
	if tm.keepAliveCancel != nil {
		tm.keepAliveCancel()
		tm.keepAliveCancel = nil
	}
	interval := tm.keepAliveInterval
	if interval <= 0 {
		interval = DefaultKeepAliveInterval
	}
	kaCtx, kaCancel := context.WithCancel(context.Background())
	tm.keepAliveCancel = kaCancel

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-kaCtx.Done():
				return
			case <-ticker.C:
				tm.ProbeAndKeepAlive()
			}
		}
	}()
}

func (tm *TunnelManager) invalidateTunnel(reason string) {
	tm.mu.Lock()
	if !tm.status.Active {
		tm.mu.Unlock()
		return
	}
	if tm.keepAliveCancel != nil {
		tm.keepAliveCancel()
		tm.keepAliveCancel = nil
	}
	if tm.cancelFunc != nil {
		tm.cancelFunc()
		tm.cancelFunc = nil
	}
	if tm.cmd != nil && tm.cmd.Process != nil {
		_ = tm.cmd.Process.Kill()
		tm.cmd = nil
	}
	if tm.tsListener != nil {
		_ = tm.tsListener.Close()
		tm.tsListener = nil
	}
	tm.status.Active = false
	tm.status.Type = ""
	tm.status.GatewayURL = ""
	tm.status.PublicURL = ""
	tm.status.Error = reason
	autoStop := tm.onAutoStop
	tm.mu.Unlock()

	tm.appendLog(fmt.Sprintf("Tunnel invalidated and stopped automatically: %s", reason))
	if autoStop != nil {
		autoStop()
	}
}

// ProbeAndKeepAlive sends a lightweight HTTP request through the active tunnel to prevent
// Cloudflare 5-minute idle expiration and verifies the tunnel is still valid. If the tunnel
// is dead or unreachable, it automatically stops remote access.
func (tm *TunnelManager) ProbeAndKeepAlive() bool {
	tm.mu.RLock()
	active := tm.status.Active
	tType := tm.status.Type
	probeURL := tm.status.GatewayURL
	if probeURL == "" {
		probeURL = tm.status.PublicURL
	}
	cmd := tm.cmd
	client := tm.httpClient
	tm.mu.RUnlock()

	if !active || probeURL == "" {
		return false
	}

	if tType == "quick" && cmd != nil && cmd.Process != nil {
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			tm.invalidateTunnel("Quick tunnel process exited")
			return false
		}
	}

	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	req, err := http.NewRequest(http.MethodGet, probeURL, nil)
	if err != nil {
		tm.invalidateTunnel(fmt.Sprintf("Invalid tunnel URL: %v", err))
		return false
	}
	req.Header.Set("User-Agent", "Antigravity-Swiss-Knife-KeepAlive/1.0")
	req.Header.Set("X-Swiss-KeepAlive", "1")

	resp, err := client.Do(req)
	if err != nil {
		tm.invalidateTunnel(fmt.Sprintf("Tunnel endpoint unreachable: %v", err))
		return false
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 521, 522, 523, 524, 530:
		tm.invalidateTunnel(fmt.Sprintf("Tunnel endpoint returned HTTP %d", resp.StatusCode))
		return false
	}

	tm.appendLog(fmt.Sprintf("Keep-alive ping succeeded (HTTP %d)", resp.StatusCode))
	return true
}

// SetPort updates the target loopback port for subsequent or active tunnels.
func (tm *TunnelManager) SetPort(port int) {
	if port <= 0 {
		return
	}
	tm.mu.Lock()
	tm.localPort = port
	tm.mu.Unlock()
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
	if tunnelType == "" || tunnelType == "quick" {
		tunnelType = "quick"
	}

	// Issue #120: validate the request before touching any live tunnel, so a rejected
	// request never tears down a working remote access session.
	switch tunnelType {
	case "quick", "tailscale":
		// No extra inputs required.
	case "custom":
		// Issue #120 review: a custom endpoint must be a well-formed absolute https:// URL
		// with a non-empty, non-loopback host. Host-less input such as "https://" and
		// loopback hosts (127.0.0.1/localhost/::1) are unreachable from remote clients and
		// are rejected here instead of being surfaced to the UI as a dead link.
		normalized, err := normalizeCustomEndpoint(customURL)
		if err != nil {
			tm.mu.RLock()
			st := tm.status
			tm.mu.RUnlock()
			return st, err
		}
		customURL = normalized
	case "loopback":
		// Loopback is not a remote endpoint. 127.0.0.1 is unreachable from any remote client,
		// so it is rejected instead of being offered as a tunnel choice.
		tm.mu.RLock()
		st := tm.status
		tm.mu.RUnlock()
		return st, fmt.Errorf("loopback is not a remote endpoint; choose quick, tailscale or custom")
	default:
		tm.mu.RLock()
		st := tm.status
		tm.mu.RUnlock()
		return st, fmt.Errorf("unknown tunnel type: %s", tunnelType)
	}

	tm.Stop()

	switch tunnelType {
	case "quick":
		return tm.startQuickTunnel()
	case "tailscale":
		return tm.startTailscaleTunnel()
	default: // "custom" — URL validated above
		path := tm.endpointPath
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if !strings.HasSuffix(customURL, path) {
			customURL = strings.TrimRight(customURL, "/") + path
		}
		tm.mu.Lock()
		tm.status = sanitizeTunnelStatus(TunnelStatus{
			Active:     true,
			Type:       "custom",
			PublicURL:  customURL,
			GatewayURL: customURL,
			StartedAt:  time.Now(),
		})
		tm.startKeepAliveLoopLocked()
		st := tm.status
		tm.mu.Unlock()
		tm.appendLog(fmt.Sprintf("Custom remote endpoint configured: %s", customURL))
		return st, nil
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

	cmd := exec.CommandContext(ctx, bin, "tunnel", "--no-autoupdate", "--url", fmt.Sprintf("http://127.0.0.1:%d", tm.localPort))
	core.SetHiddenConsole(cmd) // long-lived console app — keep its window hidden
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
	registeredChan := make(chan struct{}, 1)

	urlRegex := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

	scannerFunc := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			tm.appendLog(line)
			if strings.Contains(line, "Registered tunnel connection") {
				select {
				case registeredChan <- struct{}{}:
				default:
				}
			}
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
		waitErr := cmd.Wait()
		if ctx.Err() == nil {
			if waitErr == nil {
				waitErr = fmt.Errorf("cloudflared exited")
			}
			select {
			case errChan <- waitErr:
			default:
			}
			tm.mu.RLock()
			isCurrentActive := tm.cmd == cmd && tm.status.Active
			tm.mu.RUnlock()
			if isCurrentActive {
				tm.invalidateTunnel(fmt.Sprintf("Quick tunnel process terminated: %v", waitErr))
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
		select {
		case <-registeredChan:
		case <-time.After(2500 * time.Millisecond):
		case <-ctx.Done():
		}

		path := tm.endpointPath
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		gatewayURL := strings.TrimRight(discoveredURL, "/") + path

		tm.mu.Lock()
		tm.status.Active = true
		tm.status.PublicURL = sanitizeEndpointURL(discoveredURL)
		tm.status.GatewayURL = sanitizeEndpointURL(gatewayURL)
		tm.status.Error = ""
		tm.startKeepAliveLoopLocked()
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
				tm.status.PublicURL = sanitizeEndpointURL(match)
				tm.status.GatewayURL = sanitizeEndpointURL(gatewayURL)
				tm.status.Error = ""
				tm.startKeepAliveLoopLocked()
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
	cmdIP := exec.Command(tsBin, "ip", "-4")
	core.SetHiddenConsole(cmdIP)
	out, err := cmdIP.Output()
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
	cmdStatus := exec.Command(tsBin, "status", "--json")
	core.SetHiddenConsole(cmdStatus)
	statusOut, _ := cmdStatus.Output()
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

	// Bind a TCP forwarder on the Tailscale interface IP (<tsIP>:<localPort> -> 127.0.0.1:<localPort>)
	// so Tailnet peers can reach the loopback daemon directly with zero extra setup.
	var forwardPort = tm.localPort
	if ln, listenErr := net.Listen("tcp", fmt.Sprintf("%s:%d", tsIP, tm.localPort)); listenErr == nil {
		if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
			forwardPort = tcpAddr.Port
		}
		tm.mu.Lock()
		if tm.tsListener != nil {
			_ = tm.tsListener.Close()
		}
		tm.tsListener = ln
		tm.mu.Unlock()
		targetLoopback := fmt.Sprintf("127.0.0.1:%d", tm.localPort)
		go func(l net.Listener, target string) {
			for {
				clientConn, acceptErr := l.Accept()
				if acceptErr != nil {
					return
				}
				go func(src net.Conn) {
					defer src.Close()
					dst, dialErr := net.DialTimeout("tcp", target, 3*time.Second)
					if dialErr != nil {
						return
					}
					defer dst.Close()
					done := make(chan struct{}, 2)
					go func() { _, _ = io.Copy(dst, src); done <- struct{}{} }()
					go func() { _, _ = io.Copy(src, dst); done <- struct{}{} }()
					<-done
				}(clientConn)
			}
		}(ln, targetLoopback)
	}

	// Also attempt `tailscale funnel --bg` / `tailscale serve --bg` if operator mode is enabled
	usedHTTPSServe := false
	if magicDNS != "" {
		for _, subcmd := range []string{"funnel", "serve"} {
			c := exec.Command(tsBin, subcmd, "--bg", fmt.Sprintf("http://127.0.0.1:%d", tm.localPort))
			core.SetHiddenConsole(c)
			if err := c.Run(); err == nil {
				usedHTTPSServe = true
				break
			}
		}
	}

	var gatewayURL string
	var publicURL string
	if usedHTTPSServe && magicDNS != "" {
		publicURL = fmt.Sprintf("https://%s", magicDNS)
		gatewayURL = fmt.Sprintf("https://%s%s", magicDNS, path)
	} else if magicDNS != "" {
		publicURL = fmt.Sprintf("http://%s:%d", magicDNS, forwardPort)
		gatewayURL = fmt.Sprintf("http://%s:%d%s", magicDNS, forwardPort, path)
	} else {
		publicURL = fmt.Sprintf("http://%s:%d", tsIP, forwardPort)
		gatewayURL = fmt.Sprintf("http://%s:%d%s", tsIP, forwardPort, path)
	}

	tm.mu.Lock()
	tm.status = sanitizeTunnelStatus(TunnelStatus{
		Active:     true,
		Type:       "tailscale",
		PublicURL:  publicURL,
		GatewayURL: gatewayURL,
		StartedAt:  time.Now(),
	})
	tm.startKeepAliveLoopLocked()
	st := tm.status
	tm.mu.Unlock()

	tm.appendLog(fmt.Sprintf("Tailscale network gateway active: %s", gatewayURL))
	return st, nil
}

// Stop cleanly terminates any running tunnel subprocess.
func (tm *TunnelManager) Stop() {
	tm.mu.Lock()
	if tm.keepAliveCancel != nil {
		tm.keepAliveCancel()
		tm.keepAliveCancel = nil
	}
	if tm.cancelFunc != nil {
		tm.cancelFunc()
		tm.cancelFunc = nil
	}
	if tm.cmd != nil && tm.cmd.Process != nil {
		_ = tm.cmd.Process.Kill()
		tm.cmd = nil
	}
	if tm.tsListener != nil {
		_ = tm.tsListener.Close()
		tm.tsListener = nil
	}
	tm.status.Active = false
	// Issue #120: no loopback fallback endpoint — stopping a tunnel means the bridge is
	// simply no longer exposed to the internet.
	tm.status.Type = ""
	tm.status.GatewayURL = ""
	tm.status.PublicURL = ""
	tm.status.Error = ""
	tm.mu.Unlock()

	tm.appendLog("Remote tunnel stopped; bridge is no longer exposed to the internet.")
}

// isLoopbackURL reports whether raw is an absolute URL whose host is a loopback address
// (127.0.0.0/8, ::1 or "localhost").
func isLoopbackURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// sanitizeEndpointURL returns raw when it is a usable external endpoint URL — an absolute
// URL with a non-empty, non-loopback host — and "" otherwise. Issue #120 review: loopback
// hosts (127.0.0.1/localhost/::1) and host-less values such as "https://" are never valid
// remote endpoints, so they must never be stored in or served from TunnelStatus.
func sanitizeEndpointURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || isLoopbackURL(trimmed) {
		return ""
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return trimmed
}

// sanitizeTunnelStatus strips non-external endpoints from a status snapshot. The raw
// GatewayURL/PublicURL fields are serialized verbatim to the UI (RemoteControlPage.tsx
// renders tunnel.gateway_url directly), so every outward-facing path through TunnelManager —
// Start return values, GetStatus, BridgeStatus.Tunnel, RemoteControlStatus.Tunnel and
// persisted config URLs — is routed through this single chokepoint (issue #120 review).
func sanitizeTunnelStatus(st TunnelStatus) TunnelStatus {
	st.GatewayURL = sanitizeEndpointURL(st.GatewayURL)
	st.PublicURL = sanitizeEndpointURL(st.PublicURL)
	return st
}

// normalizeCustomEndpoint validates a user-supplied custom tunnel endpoint and returns its
// trimmed form. The endpoint must be an absolute https:// URL with a non-empty,
// non-loopback host: "https://" carries no host and a loopback host is unreachable from any
// remote client, so both are rejected instead of being surfaced to the UI as a dead or
// misleading link (issue #120 review). The local bridge itself stays on loopback HTTP behind
// the reverse proxy; only the exposed URL must be https://.
func normalizeCustomEndpoint(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("custom URL is required when choosing custom tunnel type")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid custom remote endpoint %q: %v", raw, err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", fmt.Errorf("custom remote endpoint must use https://, got %q", raw)
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("custom remote endpoint must include a host, got %q", raw)
	}
	if isLoopbackURL(trimmed) {
		return "", fmt.Errorf("loopback is not a remote endpoint; %q is unreachable from remote clients", u.Hostname())
	}
	return trimmed, nil
}

// externalEndpoint returns the externally reachable endpoint URL of a tunnel, or "" when the
// bridge is not exposed to the internet. Loopback URLs are never reported as remote endpoints
// (issue #120).
func externalEndpoint(st TunnelStatus) string {
	if !st.Active {
		return ""
	}
	return sanitizeEndpointURL(st.GatewayURL)
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
	// Defense in depth: sanitize on read as well, so even a future write path that stores a
	// loopback/host-less endpoint can never leak it to the UI (issue #120 review).
	return sanitizeTunnelStatus(res)
}
