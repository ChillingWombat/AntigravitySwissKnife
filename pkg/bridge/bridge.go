package bridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// Config holds persistent settings for Gemini Bridge.
type Config struct {
	Enabled         bool     `json:"enabled"`
	Port            int      `json:"port"`
	OperatingMode   string   `json:"operating_mode"` // "hybrid" | "lead" | "direct"
	TunnelType      string   `json:"tunnel_type"`    // "quick" | "tailscale" | "custom" | "" (not exposed)
	TunnelURL       string   `json:"tunnel_url"`
	AuthToken       string   `json:"auth_token"`
	PairingPIN      string   `json:"pairing_pin"`
	AutoStartTunnel bool     `json:"auto_start_tunnel"`
	AllowedTools    []string `json:"allowed_tools"`
}

// SurfaceStatus details connectivity to local Antigravity runtime environments.
type SurfaceStatus struct {
	DesktopAppConnected bool   `json:"desktop_app_connected"`
	DesktopCDPPort      int    `json:"desktop_cdp_port"`
	DesktopActiveTitle  string `json:"desktop_active_title,omitempty"`
	AgyCLIInstalled     bool   `json:"agy_cli_installed"`
	AgyCLIPath          string `json:"agy_cli_path,omitempty"`
	VSCodeConnected     bool   `json:"vscode_connected"`
	AccountResilience   bool   `json:"account_resilience"`
}

// Invocation records a single remote call from Gemini App / Web.
type Invocation struct {
	ID         string                 `json:"id"`
	Timestamp  string                 `json:"timestamp"`
	Tool       string                 `json:"tool"`
	Mode       string                 `json:"mode,omitempty"`
	Args       map[string]interface{} `json:"args,omitempty"`
	DurationMs int64                  `json:"duration_ms"`
	Status     string                 `json:"status"` // "success" | "error"
	Result     string                 `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// BridgeStatus reports operational health and configuration to UI and clients.
type BridgeStatus struct {
	Enabled           bool          `json:"enabled"`
	OperatingMode     string        `json:"operating_mode"`
	GatewayURL        string        `json:"gateway_url"`
	TunnelType        string        `json:"tunnel_type"`
	TunnelOnline      bool          `json:"tunnel_online"`
	AuthToken         string        `json:"auth_token"`
	PairingPIN        string        `json:"pairing_pin"`
	Surfaces          SurfaceStatus `json:"surfaces"`
	Tunnel            TunnelStatus  `json:"tunnel"`
	ActiveLeadSession *LeadSession  `json:"active_lead_session,omitempty"`
	RecentAttempts    []AuthAttempt `json:"recent_attempts,omitempty"`
	InvocationsCount  int           `json:"invocations_count"`
	RecentInvocations []Invocation  `json:"recent_invocations"`
}

// Service manages two-way bridging, tunnels, authentication, and execution modes.
type Service struct {
	mu          sync.RWMutex
	config      Config
	configPath  string
	invocations []Invocation
	injector    *gui.Injector

	tunnelMgr   *TunnelManager
	authMgr     *AuthManager
	directHdl   *DirectHandler
	leadHdl     *LeadHandler
	remoteMgr   *RemoteManager
	channelsMgr *ChannelsManager
}

// DefaultConfig initializes recommended default bridge settings.
func DefaultConfig() Config {
	return Config{
		Enabled:         false,
		Port:            8765,
		OperatingMode:   "hybrid",
		TunnelType:      "quick",
		TunnelURL:       "",
		AuthToken:       GenerateSecureToken(),
		PairingPIN:      GeneratePairingPIN(),
		AutoStartTunnel: false,
		AllowedTools: []string{
			"antigravity_bridge_set_mode",
			"antigravity_bridge_get_mode",
			"antigravity_lead_create_conversation",
			"antigravity_lead_send_message",
			"antigravity_lead_get_status",
			"antigravity_lead_await_response",
			"antigravity_lead_list_conversations",
			"local_run_command",
			"local_read_file",
			"local_write_file",
			"local_list_directory",
			"local_system_info",
			"local_computer_use",
		},
	}
}

// NewService instantiates a Gemini Bridge manager.
func NewService(configPath string) (*Service, error) {
	if configPath == "" {
		configDir := core.GetConfigDir()
		defaultPath := filepath.Join(configDir, "gemini_bridge.json")
		aliasPath := filepath.Join(configDir, "bridge.json")
		if _, err := os.Stat(aliasPath); err == nil {
			if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
				configPath = aliasPath
			} else {
				configPath = defaultPath
			}
		} else {
			configPath = defaultPath
		}
	}
	s := &Service{
		configPath:  configPath,
		config:      DefaultConfig(),
		invocations: make([]Invocation, 0),
		injector:    gui.NewInjector(0),
		directHdl:   NewDirectHandler(),
		leadHdl:     NewLeadHandler(""),
		remoteMgr:   NewRemoteManager(""),
		channelsMgr: NewChannelsManager(""),
	}
	_ = s.loadConfig()

	s.tunnelMgr = NewTunnelManager(s.config.Port, "/api/bridge/mcp")
	s.authMgr = NewAuthManager(s.config.AuthToken, s.config.PairingPIN, func(token, pin string) error {
		s.mu.Lock()
		s.config.AuthToken = token
		s.config.PairingPIN = pin
		s.mu.Unlock()
		return s.saveConfig()
	})

	// Issue #120: "loopback" is no longer a valid tunnel type. Skip it (and empty) so a stale
	// config value from an older build never triggers a failed auto-start.
	if s.config.AutoStartTunnel && s.config.TunnelType != "" && s.config.TunnelType != "loopback" {
		go func() {
			_, _ = s.tunnelMgr.Start(s.config.TunnelType, s.config.TunnelURL)
		}()
	}

	return s, nil
}

// GetRemoteStatus returns current remote control status.
func (s *Service) GetRemoteStatus(email string, pid int) RemoteControlStatus {
	return s.remoteMgr.GetStatus(email, pid)
}

// GetRemoteConfig returns current remote control settings.
func (s *Service) GetRemoteConfig() RemoteControlConfig {
	return s.remoteMgr.GetConfig()
}

// GetRemoteConfigRedacted returns current remote control settings with SecurityPIN masked.
func (s *Service) GetRemoteConfigRedacted() RemoteControlConfig {
	return s.remoteMgr.GetRemoteConfigRedacted()
}

// UpdateRemoteConfig persists new remote control settings.
func (s *Service) UpdateRemoteConfig(cfg RemoteControlConfig) error {
	return s.remoteMgr.UpdateConfig(cfg)
}

// RotateRemotePIN generates a fresh PIN for remote access.
func (s *Service) RotateRemotePIN() (string, error) {
	return s.remoteMgr.RotatePIN()
}

// GetChannelsStatus returns operational status for WhatsApp, Telegram, Discord.
func (s *Service) GetChannelsStatus() ChannelsStatus {
	return s.channelsMgr.GetStatus()
}

// GetChannelsConfig returns persistent settings for chat channels.
func (s *Service) GetChannelsConfig() ChannelsConfig {
	return s.channelsMgr.GetConfig()
}

// GetChannelsConfigRedacted returns persistent settings for chat channels with secrets masked.
func (s *Service) GetChannelsConfigRedacted() ChannelsConfig {
	return s.channelsMgr.GetChannelsConfigRedacted()
}

// UpdateChannelsConfig updates chat channel settings.
func (s *Service) UpdateChannelsConfig(cfg ChannelsConfig) error {
	return s.channelsMgr.UpdateConfig(cfg)
}

// TestChannel executes connectivity check for a given channel.
func (s *Service) TestChannel(name string) (string, error) {
	return s.channelsMgr.TestChannel(name)
}

// GetRawAuthToken returns the raw unmasked auth token (for internal authentication testing).
func (s *Service) GetRawAuthToken() string {
	return s.authMgr.GetToken()
}

// GetRawPairingPIN returns the raw unmasked pairing PIN (for internal authentication testing).
func (s *Service) GetRawPairingPIN() string {
	return s.authMgr.GetPIN()
}

func (s *Service) loadConfig() error {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		// Try alias if primary config file is not found
		dir := filepath.Dir(s.configPath)
		base := filepath.Base(s.configPath)
		var aliasPath string
		if base == "gemini_bridge.json" {
			aliasPath = filepath.Join(dir, "bridge.json")
		} else if base == "bridge.json" {
			aliasPath = filepath.Join(dir, "gemini_bridge.json")
		}
		if aliasPath != "" {
			if aliasData, aliasErr := os.ReadFile(aliasPath); aliasErr == nil {
				data = aliasData
				err = nil
				s.configPath = aliasPath
			}
		}
	}
	if err != nil {
		return err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	if c.OperatingMode == "" {
		c.OperatingMode = "hybrid"
	}
	if c.AuthToken == "" {
		c.AuthToken = GenerateSecureToken()
	}
	if c.PairingPIN == "" {
		c.PairingPIN = GeneratePairingPIN()
	}
	s.config = c
	return nil
}

func (s *Service) saveConfig() error {
	_ = os.MkdirAll(filepath.Dir(s.configPath), 0755)
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, data, 0644)
}

// GetConfig returns current bridge configuration.
func (s *Service) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates and persists bridge configuration.
func (s *Service) UpdateConfig(c Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = c
	return s.saveConfig()
}

// StartTunnel initiates remote tunneling.
func (s *Service) StartTunnel(tType, customURL string) (TunnelStatus, error) {
	st, err := s.tunnelMgr.Start(tType, customURL)
	s.mu.Lock()
	s.config.TunnelType = st.Type
	if st.GatewayURL != "" {
		s.config.TunnelURL = st.GatewayURL
	}
	_ = s.saveConfig()
	s.mu.Unlock()
	return st, err
}

// StopTunnel terminates remote tunneling.
func (s *Service) StopTunnel() {
	s.tunnelMgr.Stop()
	s.mu.Lock()
	s.config.TunnelType = ""
	_ = s.saveConfig()
	s.mu.Unlock()
}

// StartRemoteTunnel initiates remote tunneling for remote control.
func (s *Service) StartRemoteTunnel(tType, customURL string) (TunnelStatus, error) {
	return s.remoteMgr.StartTunnel(tType, customURL)
}

// StopRemoteTunnel terminates remote tunneling for remote control.
func (s *Service) StopRemoteTunnel() {
	s.remoteMgr.StopTunnel()
}

// GetRemoteTunnelStatus returns current remote control tunnel status.
func (s *Service) GetRemoteTunnelStatus() TunnelStatus {
	return s.remoteMgr.GetTunnelStatus()
}

// RotateCredentials generates a new Bearer token and Pairing PIN.
func (s *Service) RotateCredentials() (string, string, error) {
	return s.authMgr.RotateCredentials()
}

// SetOperatingMode updates the bridge mode ("hybrid" | "lead" | "direct").
func (s *Service) SetOperatingMode(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "hybrid" && mode != "lead" && mode != "direct" {
		return fmt.Errorf("invalid mode %q: must be 'hybrid', 'lead', or 'direct'", mode)
	}
	s.mu.Lock()
	s.config.OperatingMode = mode
	_ = s.saveConfig()
	s.mu.Unlock()
	return nil
}

// ValidateRequest checks HTTP authentication.
func (s *Service) ValidateRequest(r *http.Request) (bool, string) {
	ip := r.RemoteAddr
	granted, reason := s.authMgr.ValidateRequest(r)
	s.authMgr.RecordAttempt(ip, r.Method, granted, reason)
	return granted, reason
}

// DetectSurfaces inspects local machine for running or installed Antigravity surfaces.
func (s *Service) DetectSurfaces() SurfaceStatus {
	surfaces := SurfaceStatus{
		AccountResilience: true,
	}

	if port, err := s.injector.FindDevToolsPort(); err == nil && port > 0 {
		surfaces.DesktopAppConnected = true
		surfaces.DesktopCDPPort = port
		if pages, err := s.injector.GetPageTargets(port); err == nil && len(pages) > 0 {
			surfaces.DesktopActiveTitle = pages[0].Title
		}
	}

	if p, err := exec.LookPath("agy"); err == nil {
		surfaces.AgyCLIInstalled = true
		surfaces.AgyCLIPath = p
	} else if home := os.Getenv("HOME"); home != "" {
		cand := filepath.Join(home, ".local", "bin", "agy")
		if _, statErr := os.Stat(cand); statErr == nil {
			surfaces.AgyCLIInstalled = true
			surfaces.AgyCLIPath = cand
		}
	}

	surfaces.VSCodeConnected = isVSCodeRunning()
	return surfaces
}

// isVSCodeRunning checks whether a VS Code process is currently active on the host.
func isVSCodeRunning() bool {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq Code.exe", "/NH")
	} else {
		cmd = exec.Command("pgrep", "-f", "code")
	}
	core.SetHiddenConsole(cmd) // tasklist is a console app — no visible window flash on status polls
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	if runtime.GOOS == "windows" {
		return strings.Contains(s, "code.exe")
	}
	return strings.TrimSpace(s) != ""
}

// GetStatus returns the live snapshot of Gemini Bridge.
func (s *Service) GetStatus() BridgeStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	surfaces := s.DetectSurfaces()
	tunnelStatus := s.tunnelMgr.GetStatus()

	// Issue #120: no 127.0.0.1 fallback — an empty gateway URL simply means the bridge is not
	// exposed to the internet. Only externally reachable (non-loopback) endpoints are reported.
	gatewayURL := externalEndpoint(tunnelStatus)

	recent := make([]Invocation, len(s.invocations))
	copy(recent, s.invocations)

	return BridgeStatus{
		Enabled:           s.config.Enabled,
		OperatingMode:     s.config.OperatingMode,
		GatewayURL:        gatewayURL,
		TunnelType:        tunnelStatus.Type,
		TunnelOnline:      tunnelStatus.Active,
		AuthToken:         maskToken(s.authMgr.GetToken()),
		PairingPIN:        "******",
		Surfaces:          surfaces,
		Tunnel:            tunnelStatus,
		ActiveLeadSession: s.leadHdl.GetActiveSession(),
		RecentAttempts:    s.authMgr.GetRecentAttempts(),
		InvocationsCount:  len(s.invocations),
		RecentInvocations: recent,
	}
}

func maskToken(tok string) string {
	if tok == "" {
		return ""
	}
	if len(tok) <= 8 {
		return "[REDACTED]"
	}
	return tok[:4] + "..." + tok[len(tok)-4:]
}

// RecordInvocation saves a remote MCP execution audit record.
func (s *Service) RecordInvocation(inv Invocation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if inv.ID == "" {
		inv.ID = fmt.Sprintf("inv-%d", time.Now().UnixNano())
	}
	if inv.Timestamp == "" {
		inv.Timestamp = time.Now().Format("2006-01-02 15:04:05")
	}
	s.invocations = append([]Invocation{inv}, s.invocations...)
	if len(s.invocations) > 50 {
		s.invocations = s.invocations[:50]
	}
}

// ToolManifest returns definitions of exposed tools based on OperatingMode.
func (s *Service) ToolManifest() []map[string]interface{} {
	s.mu.RLock()
	mode := s.config.OperatingMode
	s.mu.RUnlock()

	tools := make([]map[string]interface{}, 0)

	// Mode management tools (always exposed)
	tools = append(tools,
		map[string]interface{}{
			"name":        "antigravity_bridge_set_mode",
			"description": "Switches bridge operational mode between 'direct' (Gemini directly executes tasks on local PC) and 'lead' (Gemini acts as lead/messenger passing tasks to Antigravity and forwarding responses).",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"mode": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"hybrid", "lead", "direct"},
						"description": "Operating mode to engage.",
					},
				},
				"required": []string{"mode"},
			},
		},
		map[string]interface{}{
			"name":        "antigravity_bridge_get_mode",
			"description": "Returns current operational mode, active Antigravity session status, and host metrics.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	)

	// Lead / Messenger tools
	if mode == "hybrid" || mode == "lead" {
		tools = append(tools,
			map[string]interface{}{
				"name":        "antigravity_lead_create_conversation",
				"description": "Creates a new conversation in local Google Antigravity 2.0 desktop app and submits the initial task prompt. Use this when the user asks you to instruct Antigravity or create a subagent/agent to work on a task.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prompt": map[string]interface{}{
							"type":        "string",
							"description": "Detailed task instructions to dispatch to Antigravity.",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Brief title for the new conversation session.",
							"default":     "Gemini Orchestrated Task",
						},
					},
					"required": []string{"prompt"},
				},
			},
			map[string]interface{}{
				"name":        "antigravity_lead_send_message",
				"description": "Sends follow-up directions, corrections, or next assignments into the active Antigravity conversation.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Message or follow-up task instructions to send.",
						},
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Target conversation ID (optional, defaults to active session).",
						},
					},
					"required": []string{"message"},
				},
			},
			map[string]interface{}{
				"name":        "antigravity_lead_get_status",
				"description": "Checks whether Antigravity is actively executing, thinking, or has completed its response.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID to inspect (optional, defaults to active session).",
						},
					},
				},
			},
			map[string]interface{}{
				"name":        "antigravity_lead_await_response",
				"description": "Waits for Antigravity to finish generating its response turn and forwards the full text and generated artifacts back to Gemini chat.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID to wait on (optional, defaults to active session).",
						},
						"timeout_seconds": map[string]interface{}{
							"type":        "integer",
							"description": "Max seconds to wait before returning progress status (default: 60).",
							"default":     60,
						},
					},
				},
			},
			map[string]interface{}{
				"name":        "antigravity_lead_list_conversations",
				"description": "Lists recent Antigravity conversations, status, and artifact counts on the local machine.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Max items to return (default: 10).",
							"default":     10,
						},
					},
				},
			},
		)
	}

	// Direct Mode tools
	if mode == "hybrid" || mode == "direct" {
		tools = append(tools,
			map[string]interface{}{
				"name":        "local_run_command",
				"description": "Directly executes a shell command in the local host environment.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Command line to execute.",
						},
						"cwd": map[string]interface{}{
							"type":        "string",
							"description": "Working directory path.",
							"default":     ".",
						},
						"timeout_seconds": map[string]interface{}{
							"type":        "integer",
							"description": "Execution timeout in seconds (default: 30).",
							"default":     30,
						},
					},
					"required": []string{"command"},
				},
			},
			map[string]interface{}{
				"name":        "local_read_file",
				"description": "Reads contents of a file on the local machine.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Path to file to read.",
						},
						"max_bytes": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum bytes to read (default: 102400).",
							"default":     102400,
						},
					},
					"required": []string{"path"},
				},
			},
			map[string]interface{}{
				"name":        "local_write_file",
				"description": "Writes or overwrites a file on the local machine.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Target file path.",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Text content to write.",
						},
					},
					"required": []string{"path", "content"},
				},
			},
			map[string]interface{}{
				"name":        "local_list_directory",
				"description": "Lists contents and file sizes of a local directory.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path to list.",
							"default":     ".",
						},
					},
				},
			},
			map[string]interface{}{
				"name":        "local_system_info",
				"description": "Returns host OS metrics, CPU, memory, and environment details.",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			map[string]interface{}{
				"name":        "local_computer_use",
				"description": "Executes local computer use UI automation actions (status, screenshot, click, type).",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"status", "screenshot", "click", "type"},
							"description": "Action to perform.",
						},
					},
					"required": []string{"action"},
				},
			},
		)
	}

	// Backward compatibility legacy aliases
	tools = append(tools,
		map[string]interface{}{
			"name":        "antigravity_dispatch_task",
			"description": "Legacy alias for antigravity_lead_create_conversation.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{"type": "string"},
					"target": map[string]interface{}{"type": "string"},
				},
				"required": []string{"prompt"},
			},
		},
		map[string]interface{}{
			"name":        "antigravity_list_sessions",
			"description": "Legacy alias for antigravity_lead_list_conversations.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{"limit": map[string]interface{}{"type": "integer"}},
			},
		},
	)

	return tools
}

// CallTool executes an MCP tool invocation using real local APIs and state.
func (s *Service) CallTool(name string, args map[string]interface{}) (string, error) {
	start := time.Now()
	var result string
	var err error
	var opMode string

	s.mu.RLock()
	currentMode := s.config.OperatingMode
	allowedTools := s.config.AllowedTools
	s.mu.RUnlock()

	// Enforce AllowedTools restriction
	if len(allowedTools) > 0 {
		isAllowed := false
		for _, t := range allowedTools {
			if t == name ||
				(name == "antigravity_dispatch_task" && t == "antigravity_lead_create_conversation") ||
				(name == "antigravity_list_sessions" && t == "antigravity_lead_list_conversations") ||
				(name == "antigravity_inspect_workspace" && t == "local_list_directory") ||
				(name == "antigravity_system_info" && t == "local_system_info") ||
				(name == "antigravity_computer_use" && t == "local_computer_use") {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			return "", fmt.Errorf("tool %q is not in the allowed tools list", name)
		}
	}

	// Mode compatibility enforcement
	isDirect := strings.HasPrefix(name, "local_") || name == "antigravity_inspect_workspace" || name == "antigravity_system_info" || name == "antigravity_computer_use"
	isLead := strings.HasPrefix(name, "antigravity_lead_") || name == "antigravity_dispatch_task" || name == "antigravity_list_sessions"
	if currentMode == "lead" && isDirect {
		return "", fmt.Errorf("tool %q is disabled in %q operational mode", name, currentMode)
	}
	if currentMode == "direct" && isLead {
		return "", fmt.Errorf("tool %q is disabled in %q operational mode", name, currentMode)
	}

	switch name {
	// Mode Management
	case "antigravity_bridge_set_mode":
		mode, _ := args["mode"].(string)
		err = s.SetOperatingMode(mode)
		if err == nil {
			result = fmt.Sprintf("Bridge operational mode switched to %q", mode)
		}
		opMode = "control"

	case "antigravity_bridge_get_mode":
		st := s.GetStatus()
		data, _ := json.MarshalIndent(st, "", "  ")
		result = string(data)
		opMode = "control"

	// Lead / Messenger tools
	case "antigravity_lead_create_conversation", "antigravity_dispatch_task":
		result, err = s.leadHdl.CreateConversation(args)
		opMode = "lead"

	case "antigravity_lead_send_message":
		result, err = s.leadHdl.SendMessage(args)
		opMode = "lead"

	case "antigravity_lead_get_status":
		result, err = s.leadHdl.GetStatus(args)
		opMode = "lead"

	case "antigravity_lead_await_response":
		result, err = s.leadHdl.AwaitResponse(args)
		opMode = "lead"

	case "antigravity_lead_list_conversations", "antigravity_list_sessions":
		result, err = s.leadHdl.ListConversations(args)
		opMode = "lead"

	// Direct Mode tools
	case "local_run_command":
		result, err = s.directHdl.RunCommand(args)
		opMode = "direct"

	case "local_read_file":
		result, err = s.directHdl.ReadFile(args)
		opMode = "direct"

	case "local_write_file":
		result, err = s.directHdl.WriteFile(args)
		opMode = "direct"

	case "local_list_directory", "antigravity_inspect_workspace":
		result, err = s.directHdl.ListDirectory(args)
		opMode = "direct"

	case "local_system_info", "antigravity_system_info":
		result, err = s.directHdl.SystemInfo()
		opMode = "direct"

	case "local_computer_use", "antigravity_computer_use":
		result, err = s.directHdl.ComputerUse(args)
		opMode = "direct"

	default:
		err = fmt.Errorf("unknown tool %q in mode %q", name, currentMode)
	}

	duration := time.Since(start).Milliseconds()
	inv := Invocation{
		Tool:       name,
		Mode:       opMode,
		Args:       args,
		DurationMs: duration,
		Status:     "success",
		Result:     result,
	}
	if err != nil {
		inv.Status = "error"
		inv.Error = err.Error()
	}
	s.RecordInvocation(inv)

	return result, err
}

// HandleMCPRequest processes standard Model Context Protocol JSON-RPC requests.
func (s *Service) HandleMCPRequest(reqBytes []byte) ([]byte, error) {
	var r struct {
		JSONRPC string                 `json:"jsonrpc"`
		ID      interface{}            `json:"id"`
		Method  string                 `json:"method"`
		Params  map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(reqBytes, &r); err != nil {
		return nil, fmt.Errorf("invalid json-rpc: %w", err)
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      r.ID,
	}

	switch r.Method {
	case "initialize":
		response["result"] = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "antigravity-gemini-bridge",
				"version": "1.6.0",
			},
			"instructions": "Gemini Bridge connects you to the user's local PC and Google Antigravity 2.0 app. You have two modes: 1. Direct Mode: work on the local PC directly using local_* tools (run_command, read_file, write_file). 2. Lead/Messenger Mode: act as a lead/coordinator passing tasks to Google Antigravity 2.0 (antigravity_lead_create_conversation, antigravity_lead_send_message, antigravity_lead_await_response), and forward Antigravity's completed responses and artifacts to the user in this chat. Ask the user which mode they prefer if unclear.",
		}
	case "notifications/initialized":
		return nil, nil
	case "tools/list":
		response["result"] = map[string]interface{}{
			"tools": s.ToolManifest(),
		}
	case "tools/call":
		toolName, _ := r.Params["name"].(string)
		args, _ := r.Params["arguments"].(map[string]interface{})
		text, err := s.CallTool(toolName, args)
		if err != nil {
			response["error"] = map[string]interface{}{
				"code":    -32603,
				"message": err.Error(),
			}
		} else {
			response["result"] = map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": text,
					},
				},
			}
		}
	default:
		response["error"] = map[string]interface{}{
			"code":    -32601,
			"message": fmt.Sprintf("method not found: %s", r.Method),
		}
	}

	return json.Marshal(response)
}
