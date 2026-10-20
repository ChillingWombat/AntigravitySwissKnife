package bridge

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// RemoteControlConfig holds settings for the account-resilient remote control wrapper.
type RemoteControlConfig struct {
	Enabled           bool   `json:"enabled"`
	BindAddr          string `json:"bind_addr"`
	Port              int    `json:"port"`
	SecurityPIN       string `json:"security_pin"`
	Quality           string `json:"quality"` // "high" | "medium" | "low"
	FrameRate         int    `json:"frame_rate"`
	AccountResilience bool   `json:"account_resilience"`
	TunnelType        string `json:"tunnel_type,omitempty"` // "quick" | "tailscale" | "custom" | "" (not exposed)
	TunnelURL         string `json:"tunnel_url,omitempty"`
}

// RemoteControlStatus details the live state of the wrapper across account switches.
type RemoteControlStatus struct {
	Active                bool         `json:"active"`
	EndpointURL           string       `json:"endpoint_url"`
	AccountResilience     bool         `json:"account_resilience"`
	CurrentAntigravityPID int          `json:"current_antigravity_pid"`
	DevToolsPort          int          `json:"devtools_port"`
	ActiveGoogleAccount   string       `json:"active_google_account"`
	ClientsCount          int          `json:"clients_count"`
	LastRotatedAt         string       `json:"last_rotated_at,omitempty"`
	Tunnel                TunnelStatus `json:"tunnel"`
}

// RemoteManager wraps factory remote control to keep it consistent across account rotations.
type RemoteManager struct {
	mu            sync.RWMutex
	config        RemoteControlConfig
	configPath    string
	injector      *gui.Injector
	lastRotated   time.Time
	tunnelMgr     *TunnelManager
	leadHdl       *LeadHandler
	httpServer    *http.Server
	httpLn        net.Listener
	boundPort     int
	clientsMu     sync.Mutex
	activeClients map[string]time.Time
}

// DefaultRemoteConfig returns standard default remote control settings.
func DefaultRemoteConfig() RemoteControlConfig {
	pin, _ := rand.Int(rand.Reader, big.NewInt(900000))
	pinStr := fmt.Sprintf("%06d", pin.Int64()+100000)
	return RemoteControlConfig{
		Enabled:           false,
		BindAddr:          "127.0.0.1",
		Port:              8766,
		SecurityPIN:       pinStr,
		Quality:           "high",
		FrameRate:         30,
		AccountResilience: true,
		TunnelType:        "",
		TunnelURL:         "",
	}
}

// NewRemoteManager initializes a remote control session wrapper.
func NewRemoteManager(configPath string) *RemoteManager {
	if configPath == "" {
		configPath = filepath.Join(core.GetConfigDir(), "remote_control.json")
	}
	rm := &RemoteManager{
		config:        DefaultRemoteConfig(),
		configPath:    configPath,
		injector:      gui.NewInjector(0),
		leadHdl:       NewLeadHandler(""),
		activeClients: make(map[string]time.Time),
	}
	_ = rm.load()
	port := rm.config.Port
	if port <= 0 {
		port = 8766
	}
	rm.tunnelMgr = NewTunnelManager(port, "/remote")
	if !gui.IsTestExecution() && rm.config.Enabled {
		_ = rm.EnsureServerRunning()
		if rm.config.TunnelType != "" && rm.config.TunnelType != "loopback" {
			tType := rm.config.TunnelType
			tURL := rm.config.TunnelURL
			go func() {
				_, _ = rm.StartTunnel(tType, tURL)
			}()
		}
	}
	return rm
}

// SetLeadHandler sets the lead handler used for Antigravity conversation dispatching.
func (rm *RemoteManager) SetLeadHandler(lh *LeadHandler) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.leadHdl = lh
}

func (rm *RemoteManager) load() error {
	data, err := os.ReadFile(rm.configPath)
	if err != nil {
		return err
	}
	var c RemoteControlConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	if c.BindAddr == "" {
		c.BindAddr = "127.0.0.1"
	}
	if c.Port <= 0 {
		c.Port = 8766
	}
	// Issue #120 review: never carry over a stale loopback endpoint persisted by an older
	// build — the config API must not serve one.
	c.TunnelURL = sanitizeEndpointURL(c.TunnelURL)
	rm.config = c
	return nil
}

func (rm *RemoteManager) save() error {
	_ = os.MkdirAll(filepath.Dir(rm.configPath), 0755)
	data, err := json.MarshalIndent(rm.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rm.configPath, data, 0644)
}

// GetConfig returns current remote control configuration.
func (rm *RemoteManager) GetConfig() RemoteControlConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.config
}

// GetRemoteConfigRedacted returns remote control configuration with SecurityPIN masked.
func (rm *RemoteManager) GetRemoteConfigRedacted() RemoteControlConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	cfg := rm.config
	if cfg.SecurityPIN != "" {
		cfg.SecurityPIN = "******"
	}
	return cfg
}

// EnsureServerRunning binds and starts the loopback HTTP server if not already running.
func (rm *RemoteManager) EnsureServerRunning() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	targetPort := rm.config.Port
	if targetPort <= 0 {
		targetPort = 8766
	}

	if rm.httpLn != nil && rm.boundPort == targetPort {
		return nil
	}

	if rm.httpServer != nil {
		_ = rm.httpServer.Close()
		rm.httpServer = nil
	}
	if rm.httpLn != nil {
		_ = rm.httpLn.Close()
		rm.httpLn = nil
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", targetPort))
	if err != nil {
		// Fall back to 127.0.0.1:0 if port is already in use during parallel tests
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("listen remote server: %w", err)
		}
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return fmt.Errorf("unexpected listener address: %v", ln.Addr())
	}

	rm.boundPort = tcpAddr.Port
	rm.httpLn = ln
	if rm.tunnelMgr != nil {
		rm.tunnelMgr.SetPort(rm.boundPort)
	}

	server := &http.Server{
		Handler:      rm,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	rm.httpServer = server

	go func() {
		_ = server.Serve(ln)
	}()

	return nil
}

func (rm *RemoteManager) stopHTTPServer() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rm.httpServer != nil {
		_ = rm.httpServer.Close()
		rm.httpServer = nil
	}
	if rm.httpLn != nil {
		_ = rm.httpLn.Close()
		rm.httpLn = nil
	}
	rm.boundPort = 0
}

// UpdateConfig persists updated remote control configuration.
func (rm *RemoteManager) UpdateConfig(c RemoteControlConfig) error {
	// Issue #120 review: the config API accepts external input — strip any loopback endpoint
	// before it can be persisted or echoed back to callers.
	c.TunnelURL = sanitizeEndpointURL(c.TunnelURL)
	if c.BindAddr == "" {
		c.BindAddr = "127.0.0.1"
	}
	if c.Port <= 0 {
		c.Port = 8766
	}

	rm.mu.Lock()
	if c.SecurityPIN == "******" || c.SecurityPIN == "[REDACTED]" || strings.TrimSpace(c.SecurityPIN) == "" {
		c.SecurityPIN = rm.config.SecurityPIN
	}
	oldPort := rm.config.Port
	rm.config = c
	serverRunning := rm.httpLn != nil
	boundPort := rm.boundPort
	err := rm.save()
	rm.mu.Unlock()

	if rm.tunnelMgr != nil {
		rm.tunnelMgr.SetPort(c.Port)
	}

	if serverRunning && (boundPort != c.Port || oldPort != c.Port) {
		_ = rm.EnsureServerRunning()
	}
	return err
}

// StartTunnel initiates tunneling for remote control.
func (rm *RemoteManager) StartTunnel(tType, customURL string) (TunnelStatus, error) {
	_ = rm.EnsureServerRunning()
	st, err := rm.tunnelMgr.Start(tType, customURL)
	if err != nil {
		return st, err
	}
	rm.mu.Lock()
	rm.config.Enabled = true
	rm.config.TunnelType = st.Type
	if st.GatewayURL != "" {
		rm.config.TunnelURL = st.GatewayURL
	}
	_ = rm.save()
	rm.mu.Unlock()
	return st, nil
}

// StopTunnel terminates tunneling for remote control.
func (rm *RemoteManager) StopTunnel() {
	rm.tunnelMgr.Stop()
	rm.stopHTTPServer()
	rm.mu.Lock()
	rm.config.Enabled = false
	rm.config.TunnelType = ""
	rm.config.TunnelURL = ""
	_ = rm.save()
	rm.mu.Unlock()
}

// GetTunnelStatus returns the current tunnel status for remote control.
func (rm *RemoteManager) GetTunnelStatus() TunnelStatus {
	return rm.tunnelMgr.GetStatus()
}

// RotatePIN generates a fresh 6-digit PIN and clears active client sessions.
func (rm *RemoteManager) RotatePIN() (string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	pin, _ := rand.Int(rand.Reader, big.NewInt(900000))
	newPIN := fmt.Sprintf("%06d", pin.Int64()+100000)
	rm.config.SecurityPIN = newPIN
	rm.clientsMu.Lock()
	rm.activeClients = make(map[string]time.Time)
	rm.clientsMu.Unlock()
	err := rm.save()
	return newPIN, err
}

// GetStatus returns the live operational status of the wrapper.
func (rm *RemoteManager) GetStatus(activeEmail string, activePID int) RemoteControlStatus {
	rm.mu.RLock()
	cfg := rm.config
	cdpPort := 0
	if port, err := rm.injector.FindDevToolsPort(); err == nil {
		cdpPort = port
	}

	endpoint := ""
	var tSt TunnelStatus
	if rm.tunnelMgr != nil {
		tSt = rm.tunnelMgr.GetStatus()
		endpoint = externalEndpoint(tSt)
	}

	rotatedStr := ""
	if !rm.lastRotated.IsZero() {
		rotatedStr = rm.lastRotated.Format("2006-01-02 15:04:05")
	}
	rm.mu.RUnlock()

	rm.clientsMu.Lock()
	cutoff := time.Now().Add(-2 * time.Minute)
	for ip, lastSeen := range rm.activeClients {
		if lastSeen.Before(cutoff) {
			delete(rm.activeClients, ip)
		}
	}
	clientsCount := len(rm.activeClients)
	rm.clientsMu.Unlock()

	return RemoteControlStatus{
		Active:                cfg.Enabled && cdpPort > 0,
		EndpointURL:           endpoint,
		AccountResilience:     cfg.AccountResilience,
		CurrentAntigravityPID: activePID,
		DevToolsPort:          cdpPort,
		ActiveGoogleAccount:   activeEmail,
		ClientsCount:          clientsCount,
		LastRotatedAt:         rotatedStr,
		Tunnel:                tSt,
	}
}

// NotifyAccountSwitch is called by Daemon when account rotation finishes to seamlessly re-bind remote session.
func (rm *RemoteManager) NotifyAccountSwitch(newAccount string, newPID int) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.lastRotated = time.Now()
}

func (rm *RemoteManager) isPINAuthorized(r *http.Request) bool {
	cfg := rm.GetConfig()
	expectedPIN := strings.TrimSpace(cfg.SecurityPIN)
	if expectedPIN == "" {
		return false
	}

	candidate := strings.TrimSpace(r.Header.Get("X-Remote-PIN"))
	if candidate == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			candidate = strings.TrimSpace(auth[7:])
		}
	}
	if candidate == "" {
		candidate = strings.TrimSpace(r.URL.Query().Get("pin"))
	}
	if candidate == "" {
		if cookie, err := r.Cookie("swiss_remote_pin"); err == nil {
			candidate = strings.TrimSpace(cookie.Value)
		}
	}

	if candidate == "" || candidate != expectedPIN {
		return false
	}

	clientIP := getClientIP(r)
	now := time.Now()
	cutoff := now.Add(-2 * time.Minute)

	rm.clientsMu.Lock()
	rm.activeClients[clientIP] = now
	for ip, lastSeen := range rm.activeClients {
		if lastSeen.Before(cutoff) {
			delete(rm.activeClients, ip)
		}
	}
	rm.clientsMu.Unlock()

	return true
}

// ServeHTTP routes incoming remote HTTP requests to the web console and JSON APIs.
func (rm *RemoteManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Favicon
	if path == "/favicon.ico" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Auth API
	if r.Method == http.MethodPost && (path == "/remote/api/auth" || path == "/api/auth") {
		rm.handleAuth(w, r)
		return
	}

	// Logout API
	if r.Method == http.MethodPost && (path == "/remote/api/logout" || path == "/api/logout") {
		rm.handleLogout(w, r)
		return
	}

	// Status API
	if path == "/remote/api/status" || path == "/api/status" {
		rm.handleStatus(w, r)
		return
	}

	// Screenshot API
	if path == "/remote/api/screenshot" || path == "/api/screenshot" {
		rm.handleScreenshot(w, r)
		return
	}

	// Conversation creation
	if r.Method == http.MethodPost && (path == "/remote/api/conversation/new" || path == "/api/conversation/new") {
		rm.handleConversationNew(w, r)
		return
	}

	// Conversation send message
	if r.Method == http.MethodPost && (path == "/remote/api/conversation/message" || path == "/api/conversation/message") {
		rm.handleConversationMessage(w, r)
		return
	}

	// Conversation status / transcript tail
	if path == "/remote/api/conversation/status" || path == "/api/conversation/status" {
		rm.handleConversationStatus(w, r)
		return
	}

	// Remote Control Web Console HTML
	if path == "/" || path == "/remote" || path == "/remote/" || path == "/index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(remoteHTML))
		return
	}

	http.NotFound(w, r)
}

func (rm *RemoteManager) handleAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.PIN = r.FormValue("pin")
	}
	pin := strings.TrimSpace(body.PIN)
	if pin == "" {
		pin = strings.TrimSpace(r.Header.Get("X-Remote-PIN"))
	}
	expectedPIN := strings.TrimSpace(rm.GetConfig().SecurityPIN)

	if pin == "" || expectedPIN == "" || pin != expectedPIN {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid 6-digit Security PIN",
		})
		return
	}

	clientIP := getClientIP(r)
	now := time.Now()
	cutoff := now.Add(-2 * time.Minute)

	rm.clientsMu.Lock()
	rm.activeClients[clientIP] = now
	for ip, lastSeen := range rm.activeClients {
		if lastSeen.Before(cutoff) {
			delete(rm.activeClients, ip)
		}
	}
	rm.clientsMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "swiss_remote_pin",
		Value:    pin,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (rm *RemoteManager) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "swiss_remote_pin",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (rm *RemoteManager) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !rm.isPINAuthorized(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}

	rm.mu.RLock()
	cfg := rm.config
	lh := rm.leadHdl
	rm.mu.RUnlock()

	cdpPort := 0
	if port, err := rm.injector.FindDevToolsPort(); err == nil {
		cdpPort = port
	}

	var convs interface{} = []interface{}{}
	if lh != nil {
		if convsJSON, err := lh.ListConversations(map[string]interface{}{"limit": float64(10)}); err == nil {
			_ = json.Unmarshal([]byte(convsJSON), &convs)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"active":             true,
		"devtools_port":      cdpPort,
		"quality":            cfg.Quality,
		"frame_rate":         cfg.FrameRate,
		"account_resilience": cfg.AccountResilience,
		"conversations":      convs,
	})
}

func (rm *RemoteManager) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	if !rm.isPINAuthorized(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}

	cdpPort, err := rm.injector.FindDevToolsPort()
	if err != nil || cdpPort <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Antigravity Desktop CDP is not connected",
		})
		return
	}

	pages, err := rm.injector.GetPageTargets(cdpPort)
	if err != nil || len(pages) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Antigravity Desktop CDP is not connected",
		})
		return
	}

	imgBytes, err := rm.injector.CaptureScreenshot(pages[0].WebSocketDebuggerURL, nil)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(imgBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(imgBytes)
}

func (rm *RemoteManager) handleConversationNew(w http.ResponseWriter, r *http.Request) {
	if !rm.isPINAuthorized(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
		Title  string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid JSON: " + err.Error(),
		})
		return
	}

	rm.mu.RLock()
	lh := rm.leadHdl
	rm.mu.RUnlock()

	if lh == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Lead handler is not configured",
		})
		return
	}

	args := map[string]interface{}{
		"prompt": req.Prompt,
		"title":  req.Title,
	}
	res, err := lh.CreateConversation(args)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"result":  res,
	})
}

func (rm *RemoteManager) handleConversationMessage(w http.ResponseWriter, r *http.Request) {
	if !rm.isPINAuthorized(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}

	var req struct {
		ConversationID string `json:"conversation_id"`
		Message        string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid JSON: " + err.Error(),
		})
		return
	}

	rm.mu.RLock()
	lh := rm.leadHdl
	rm.mu.RUnlock()

	if lh == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Lead handler is not configured",
		})
		return
	}

	args := map[string]interface{}{
		"conversation_id": req.ConversationID,
		"message":         req.Message,
	}
	res, err := lh.SendMessage(args)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"result":  res,
	})
}

func (rm *RemoteManager) handleConversationStatus(w http.ResponseWriter, r *http.Request) {
	if !rm.isPINAuthorized(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}

	convID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	rm.mu.RLock()
	lh := rm.leadHdl
	rm.mu.RUnlock()

	if lh == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Lead handler is not configured",
		})
		return
	}

	rawJSON, err := lh.GetStatus(map[string]interface{}{"conversation_id": convID})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":         false,
			"conversation_id": convID,
			"error":           err.Error(),
		})
		return
	}

	var detail map[string]interface{}
	_ = json.Unmarshal([]byte(rawJSON), &detail)
	if detail == nil {
		detail = map[string]interface{}{}
	}
	detail["success"] = true
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(detail)
}

const remoteHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Antigravity Remote Control</title>
<style>
:root {
  --bg: #f8fafc;
  --surface: #ffffff;
  --surface-alt: #f1f5f9;
  --border: #e2e8f0;
  --text-main: #0f172a;
  --text-sub: #64748b;
  --primary: #1a73e8;
  --primary-hover: #1557b0;
  --emerald: #059669;
  --red: #dc2626;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  background: var(--bg);
  color: var(--text-main);
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.icon {
  width: 16px;
  height: 16px;
  display: inline-block;
  vertical-align: middle;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
  fill: none;
}
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 600;
  font-size: 15px;
  letter-spacing: -0.01em;
}
.actions-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  background: rgba(16, 185, 129, 0.12);
  color: var(--emerald);
  border: 1px solid rgba(16, 185, 129, 0.25);
}
.badge-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--emerald);
}
.badge-secondary {
  background: rgba(59, 130, 246, 0.12);
  color: var(--primary);
  border: 1px solid rgba(59, 130, 246, 0.25);
}
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.15s ease;
}
.btn-primary {
  background: var(--primary);
  color: #fff;
}
.btn-primary:hover {
  background: var(--primary-hover);
}
.btn-secondary {
  background: var(--surface-alt);
  color: var(--text-main);
  border-color: var(--border);
}
.btn-secondary:hover {
  background: var(--border);
}
.btn-sm {
  padding: 5px 10px;
  font-size: 12px;
}
.container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 20px;
  width: 100%;
  flex: 1;
}
.grid {
  display: grid;
  grid-template-columns: 1.15fr 0.85fr;
  gap: 20px;
}
@media (max-width: 960px) {
  .grid { grid-template-columns: 1fr; }
}
.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
  white-space: nowrap;
}
.viewport-container {
  position: relative;
  width: 100%;
  aspect-ratio: 16 / 10;
  background: #000;
  border-radius: 6px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border);
}
.viewport-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
}
.viewport-empty {
  color: var(--text-sub);
  font-size: 13px;
  text-align: center;
  padding: 20px;
}
.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  color: var(--text-sub);
}
.input-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.input-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-sub);
}
.input-text, .textarea {
  width: 100%;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--bg);
  border: 1px solid var(--border);
  color: var(--text-main);
  font-size: 13px;
  font-family: inherit;
  outline: none;
}
.input-text:focus, .textarea:focus {
  border-color: var(--primary);
}
.textarea {
  resize: vertical;
  min-height: 80px;
}
.btn-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.convs-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 280px;
  overflow-y: auto;
}
.conv-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--surface-alt);
  border: 1px solid var(--border);
  cursor: pointer;
  transition: all 0.15s ease;
  font-size: 13px;
}
.conv-item:hover {
  border-color: var(--primary);
}
.conv-title {
  font-weight: 500;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 220px;
}
.conv-meta {
  color: var(--text-sub);
  font-size: 11px;
  white-space: nowrap;
}
.auth-wrapper {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}
.auth-card {
  width: 100%;
  max-width: 380px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 28px;
  display: flex;
  flex-direction: column;
  gap: 18px;
  text-align: center;
}
.auth-icon-circle {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: rgba(59, 130, 246, 0.1);
  color: var(--primary);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto;
}
.auth-title {
  font-size: 18px;
  font-weight: 600;
}
.auth-desc {
  font-size: 13px;
  color: var(--text-sub);
  line-height: 1.4;
}
.pin-input {
  letter-spacing: 0.35em;
  font-size: 20px;
  text-align: center;
  font-weight: 600;
}
.msg-banner {
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 12px;
  display: none;
}
.msg-banner.error {
  display: block;
  background: rgba(239, 68, 68, 0.1);
  color: var(--red);
  border: 1px solid rgba(239, 68, 68, 0.25);
}
.msg-banner.success {
  display: block;
  background: rgba(16, 185, 129, 0.1);
  color: var(--emerald);
  border: 1px solid rgba(16, 185, 129, 0.25);
}
</style>
</head>
<body>

<div id="authView" class="auth-wrapper">
  <div class="auth-card">
    <div class="auth-icon-circle">
      <svg class="icon" viewBox="0 0 24 24"><rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
    </div>
    <div class="auth-title">Antigravity Remote Control</div>
    <div class="auth-desc">Enter your 6-digit Security PIN to access remote control console.</div>
    <div id="authError" class="msg-banner error"></div>
    <form id="authForm" onsubmit="handleAuthSubmit(event)">
      <div class="input-group">
        <input type="password" id="pinInput" class="input-text pin-input" inputmode="numeric" maxlength="6" pattern="[0-9]*" placeholder="••••••" autofocus required>
      </div>
      <div style="margin-top:14px;">
        <button type="submit" class="btn btn-primary" style="width:100%;justify-content:center;">Unlock Console</button>
      </div>
    </form>
  </div>
</div>

<div id="consoleView" style="display:none;flex-direction:column;flex:1;">
  <header class="topbar">
    <div class="brand">
      <svg class="icon" viewBox="0 0 24 24"><rect width="20" height="14" x="2" y="3" rx="2"/><line x1="8" x2="16" y1="21" y2="21"/><line x1="12" x2="12" y1="17" y2="21"/></svg>
      <span>Antigravity Remote Control</span>
      <span class="badge"><span class="badge-dot"></span><span>Active</span></span>
    </div>
    <div class="actions-right">
      <span id="cdpBadge" class="badge badge-secondary">CDP</span>
      <button class="btn btn-secondary btn-sm" onclick="handleLogout()">
        <svg class="icon" viewBox="0 0 24 24"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" x2="9" y1="12" y2="12"/></svg>
        <span>Lock</span>
      </button>
    </div>
  </header>

  <main class="container">
    <div class="grid">
      <!-- Left Column: Viewport -->
      <section class="card">
        <div class="card-header">
          <div class="card-title">
            <svg class="icon" viewBox="0 0 24 24"><rect width="20" height="14" x="2" y="3" rx="2"/><line x1="8" x2="16" y1="21" y2="21"/><line x1="12" x2="12" y1="17" y2="21"/></svg>
            <span>Live Viewport</span>
          </div>
          <button class="btn btn-secondary btn-sm" onclick="refreshScreenshot()">
            <svg class="icon" viewBox="0 0 24 24"><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/></svg>
            <span>Refresh</span>
          </button>
        </div>
        <div class="viewport-container">
          <img id="viewportImg" class="viewport-img" src="" alt="Antigravity Viewport" style="display:none;" />
          <div id="viewportFallback" class="viewport-empty">Antigravity Desktop CDP is not connected</div>
        </div>
        <div class="toggle-row">
          <span>Auto-refresh viewport (3s)</span>
          <input type="checkbox" id="autoRefreshToggle" checked onchange="toggleAutoRefresh()">
        </div>
      </section>

      <!-- Right Column: Control & Sessions -->
      <div style="display:flex;flex-direction:column;gap:20px;">
        <!-- Composer Card -->
        <section class="card">
          <div class="card-header">
            <div class="card-title">
              <svg class="icon" viewBox="0 0 24 24"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg>
              <span>Dispatch Task</span>
            </div>
          </div>
          <div id="composerBanner" class="msg-banner"></div>
          <div class="input-group">
            <label class="input-label" for="targetConvInput">Target Conversation ID (optional):</label>
            <input type="text" id="targetConvInput" class="input-text" placeholder="Active conversation or leave blank for new">
          </div>
          <div class="input-group">
            <label class="input-label" for="promptInput">Prompt / Task Message:</label>
            <textarea id="promptInput" class="textarea" placeholder="Instructions for Antigravity..."></textarea>
          </div>
          <div class="btn-row">
            <button class="btn btn-primary" onclick="handleNewConversation()">
              <svg class="icon" viewBox="0 0 24 24"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg>
              <span>Dispatch New Task</span>
            </button>
            <button class="btn btn-secondary" onclick="handleSendMessage()">
              <span>Send Message</span>
            </button>
          </div>
        </section>

        <!-- Recent Conversations Card -->
        <section class="card">
          <div class="card-header">
            <div class="card-title">
              <svg class="icon" viewBox="0 0 24 24"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
              <span>Recent Conversations</span>
            </div>
            <button class="btn btn-secondary btn-sm" onclick="loadStatus()">
              <svg class="icon" viewBox="0 0 24 24"><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/></svg>
            </button>
          </div>
          <div id="convsList" class="convs-container">
            <div class="viewport-empty">Loading conversations...</div>
          </div>
          <div id="convDetailBox" style="display:none;margin-top:8px;padding:10px 12px;border-radius:6px;background:var(--surface-alt);border:1px solid var(--border);font-size:12px;line-height:1.45;white-space:pre-wrap;color:var(--text-main);"></div>
        </section>
      </div>
    </div>
  </main>
</div>

<script>
var basePath = window.location.pathname.indexOf('/remote') === 0 ? '/remote' : '';
var autoRefreshInterval = null;
var activePin = '';
try { activePin = sessionStorage.getItem('swiss_remote_pin') || ''; } catch (e) {}

function getHeaders(extra) {
  var h = extra ? Object.assign({}, extra) : {};
  if (activePin) {
    h['X-Remote-PIN'] = activePin;
  }
  return h;
}

async function checkAuthAndInit() {
  try {
    var res = await fetch(basePath + '/api/status', { headers: getHeaders() });
    if (res.status === 401) {
      showAuth();
      return;
    }
    if (res.ok) {
      var data = await res.json();
      showConsole(data);
      return;
    }
  } catch (e) {}
  showAuth();
}

function showAuth() {
  stopAutoRefresh();
  document.getElementById('consoleView').style.display = 'none';
  document.getElementById('authView').style.display = 'flex';
  var pinInput = document.getElementById('pinInput');
  pinInput.value = '';
  pinInput.focus();
}

function showConsole(data) {
  document.getElementById('authView').style.display = 'none';
  document.getElementById('consoleView').style.display = 'flex';
  renderStatus(data);
  refreshScreenshot();
  toggleAutoRefresh();
}

async function handleAuthSubmit(e) {
  e.preventDefault();
  var pin = document.getElementById('pinInput').value.trim();
  var errEl = document.getElementById('authError');
  errEl.style.display = 'none';

  try {
    var res = await fetch(basePath + '/api/auth', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ pin: pin })
    });
    var data = await res.json();
    if (res.ok && data.success) {
      activePin = pin;
      try { sessionStorage.setItem('swiss_remote_pin', pin); } catch (e) {}
      checkAuthAndInit();
    } else {
      errEl.textContent = data.error || 'Invalid 6-digit Security PIN';
      errEl.style.display = 'block';
    }
  } catch (err) {
    errEl.textContent = 'Connection error: ' + err.message;
    errEl.style.display = 'block';
  }
}

async function handleLogout() {
  activePin = '';
  try { sessionStorage.removeItem('swiss_remote_pin'); } catch (e) {}
  try {
    await fetch(basePath + '/api/logout', { method: 'POST' });
  } catch (e) {}
  showAuth();
}

function refreshScreenshot() {
  var img = document.getElementById('viewportImg');
  var fallback = document.getElementById('viewportFallback');
  var testImg = new Image();
  var pinParam = activePin ? ('&pin=' + encodeURIComponent(activePin)) : '';
  var src = basePath + '/api/screenshot?t=' + Date.now() + pinParam;
  testImg.onload = function() {
    img.src = src;
    img.style.display = 'block';
    fallback.style.display = 'none';
  };
  testImg.onerror = function() {
    img.style.display = 'none';
    fallback.style.display = 'block';
  };
  testImg.src = src;
}

function toggleAutoRefresh() {
  var checked = document.getElementById('autoRefreshToggle').checked;
  stopAutoRefresh();
  if (checked) {
    autoRefreshInterval = setInterval(refreshScreenshot, 3000);
  }
}

function stopAutoRefresh() {
  if (autoRefreshInterval) {
    clearInterval(autoRefreshInterval);
    autoRefreshInterval = null;
  }
}

async function loadStatus() {
  try {
    var res = await fetch(basePath + '/api/status', { headers: getHeaders() });
    if (res.status === 401) {
      showAuth();
      return;
    }
    if (res.ok) {
      var data = await res.json();
      renderStatus(data);
    }
  } catch (e) {}
}

function renderStatus(data) {
  if (!data) return;
  var cdpBadge = document.getElementById('cdpBadge');
  if (data.devtools_port > 0) {
    cdpBadge.textContent = 'CDP :' + data.devtools_port;
    cdpBadge.className = 'badge badge-secondary';
  } else {
    cdpBadge.textContent = 'CDP Disconnected';
    cdpBadge.className = 'badge';
    cdpBadge.style.color = '#ef4444';
    cdpBadge.style.borderColor = 'rgba(239, 68, 68, 0.25)';
    cdpBadge.style.backgroundColor = 'rgba(239, 68, 68, 0.12)';
  }

  var listEl = document.getElementById('convsList');
  if (!data.conversations || data.conversations.length === 0) {
    listEl.innerHTML = '<div class="viewport-empty">No conversations found</div>';
    return;
  }

  var html = '';
  data.conversations.forEach(function(c) {
    var id = c.id || '';
    var title = c.title || id.substring(0, 8);
    var meta = c.last_active || c.status || 'active';
    html += '<div class="conv-item" data-conv-id="' + escapeHTML(id) + '" onclick="selectConversation(this.dataset.convId)">' +
      '<div class="conv-title">' + escapeHTML(title) + '</div>' +
      '<div class="conv-meta">' + escapeHTML(meta) + '</div>' +
    '</div>';
  });
  listEl.innerHTML = html;
}

async function selectConversation(id) {
  if (!id) return;
  document.getElementById('targetConvInput').value = id;
  var detailBox = document.getElementById('convDetailBox');
  if (!detailBox) return;
  detailBox.style.display = 'block';
  detailBox.textContent = 'Loading conversation ' + id.substring(0, 8) + '...';
  try {
    var res = await fetch(basePath + '/api/conversation/status?conversation_id=' + encodeURIComponent(id), {
      headers: getHeaders()
    });
    if (res.ok) {
      var d = await res.json();
      if (d && d.success) {
        detailBox.textContent = 'Conversation: ' + id + '\nStatus: ' + (d.status || 'unknown') + ' (Step ' + (d.latest_step || 0) + ')\n\n' + (d.preview || 'No transcript preview available.');
      } else {
        detailBox.textContent = 'Conversation: ' + id + '\n' + (d.error || 'No transcript available.');
      }
    }
  } catch (e) {
    detailBox.textContent = 'Selected conversation: ' + id;
  }
}

function escapeHTML(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

async function handleNewConversation() {
  var prompt = document.getElementById('promptInput').value.trim();
  var banner = document.getElementById('composerBanner');
  if (!prompt) {
    banner.className = 'msg-banner error';
    banner.textContent = 'Prompt cannot be empty';
    return;
  }
  banner.className = 'msg-banner';
  banner.style.display = 'none';

  var title = prompt.length > 30 ? prompt.substring(0, 30) + '...' : prompt;
  try {
    var res = await fetch(basePath + '/api/conversation/new', {
      method: 'POST',
      headers: getHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ prompt: prompt, title: title })
    });
    var data = await res.json();
    if (res.ok && data.success) {
      banner.className = 'msg-banner success';
      banner.textContent = 'Task dispatched successfully!';
      document.getElementById('promptInput').value = '';
      setTimeout(loadStatus, 1000);
      setTimeout(refreshScreenshot, 1500);
    } else {
      banner.className = 'msg-banner error';
      banner.textContent = data.error || 'Failed to dispatch task';
    }
  } catch (err) {
    banner.className = 'msg-banner error';
    banner.textContent = 'Error: ' + err.message;
  }
}

async function handleSendMessage() {
  var convId = document.getElementById('targetConvInput').value.trim();
  var message = document.getElementById('promptInput').value.trim();
  var banner = document.getElementById('composerBanner');
  if (!message) {
    banner.className = 'msg-banner error';
    banner.textContent = 'Message cannot be empty';
    return;
  }
  banner.className = 'msg-banner';
  banner.style.display = 'none';

  try {
    var res = await fetch(basePath + '/api/conversation/message', {
      method: 'POST',
      headers: getHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ conversation_id: convId, message: message })
    });
    var data = await res.json();
    if (res.ok && data.success) {
      banner.className = 'msg-banner success';
      banner.textContent = 'Message sent successfully!';
      document.getElementById('promptInput').value = '';
      setTimeout(loadStatus, 1000);
      setTimeout(refreshScreenshot, 1500);
    } else {
      banner.className = 'msg-banner error';
      banner.textContent = data.error || 'Failed to send message';
    }
  } catch (err) {
    banner.className = 'msg-banner error';
    banner.textContent = 'Error: ' + err.message;
  }
}

window.addEventListener('DOMContentLoaded', checkAuthAndInit);
</script>
</body>
</html>
`
