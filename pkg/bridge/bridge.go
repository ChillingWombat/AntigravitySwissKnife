package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
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
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
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
	UseAppPassword  bool     `json:"use_app_password"`
	OAuthClientID   string   `json:"oauth_client_id"`
	AutoStartTunnel bool     `json:"auto_start_tunnel"`
	AllowedTools    []string `json:"allowed_tools"`
	// AgentBackend selects which Antigravity surface lead/model tools drive:
	// "auto" (CLI when installed), "app" (2.0 desktop app) or "cli".
	AgentBackend string `json:"agent_backend"`
	// HTTPAllowlistMode configures outbound HTTP filtering: "always_allow" (default) or "whitelist_only".
	HTTPAllowlistMode string `json:"http_allowlist_mode"`
	// HTTPAllowlist gates local_http_request. An agent may only reach hosts listed
	// here; anything else lands in HTTPPendingApprovals and is never called.
	HTTPAllowlist        []string              `json:"http_allowlist"`
	HTTPPendingApprovals []HTTPApprovalRequest `json:"http_pending_approvals"`
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
	Enabled            bool   `json:"enabled"`
	OperatingMode      string `json:"operating_mode"`
	GatewayURL         string `json:"gateway_url"`
	TunnelType         string `json:"tunnel_type"`
	TunnelOnline       bool   `json:"tunnel_online"`
	AuthToken          string `json:"auth_token"`
	PairingPIN         string `json:"pairing_pin"`
	UseAppPassword     bool   `json:"use_app_password"`
	AppPasswordEnabled bool   `json:"app_password_enabled"`
	OAuthClientID      string `json:"oauth_client_id"`
	OAuthClientSecret  string `json:"oauth_client_secret"`
	// Which Antigravity surface lead/model tools drive, requested vs resolved.
	AgentBackend      string        `json:"agent_backend"`
	ResolvedAgent     string        `json:"resolved_agent"`
	Surfaces          SurfaceStatus `json:"surfaces"`
	Tunnel            TunnelStatus  `json:"tunnel"`
	ActiveLeadSession *LeadSession  `json:"active_lead_session,omitempty"`
	RecentAttempts    []AuthAttempt `json:"recent_attempts,omitempty"`
	InvocationsCount  int           `json:"invocations_count"`
	RecentInvocations []Invocation  `json:"recent_invocations"`
	// Outbound HTTP gate: the mode, hosts agents may call, and the hosts they asked for.
	HTTPAllowlistMode    string                `json:"http_allowlist_mode"`
	HTTPAllowlist        []string              `json:"http_allowlist"`
	HTTPPendingApprovals []HTTPApprovalRequest `json:"http_pending_approvals"`
}

// Service manages two-way bridging, tunnels, authentication, and execution modes.
type Service struct {
	mu          sync.RWMutex
	tunnelMu    sync.Mutex
	oauthMu     sync.Mutex
	authCodes   map[string]authCodeEntry
	config      Config
	configPath  string
	invocations []Invocation
	injector    *gui.Injector

	tunnelMgr *TunnelManager
	authMgr   *AuthManager
	directHdl *DirectHandler
	leadHdl   *LeadHandler
	cliHdl    *CLIHandler
	a2aMgr    *A2ATaskManager
	// Custom models are surfaced through the same model tools as native ones.
	customModels    customModelProvider
	activateCustom  customModelActivator
	gatewayTokenFn  func() string
	remoteMgr       *RemoteManager
	channelsMgr     *ChannelsManager
	discordGW       *DiscordGatewayClient
	discordGWCancel context.CancelFunc
	discordGWMu     sync.Mutex
}

// DefaultConfig initializes recommended default bridge settings.
func DefaultConfig() Config {
	return Config{
		Enabled:           false,
		Port:              8765,
		OperatingMode:     "hybrid",
		AgentBackend:      AgentAuto,
		HTTPAllowlistMode: "always_allow",
		TunnelType:        "quick",
		TunnelURL:         "",
		AuthToken:         GenerateSecureToken(),
		PairingPIN:        GeneratePairingPIN(),
		OAuthClientID:     "antigravity-gemini-bridge",
		AutoStartTunnel:   false,
		AllowedTools: []string{
			"antigravity_bridge_set_mode",
			"antigravity_bridge_get_mode",
			"antigravity_bridge_set_agent",
			"a2a_assign_task",
			"a2a_get_task_status",
			"a2a_await_task",
			"a2a_send_task_message",
			"a2a_cancel_task",
			"a2a_list_tasks",
			"antigravity_lead_create_conversation",
			"antigravity_lead_send_message",
			"antigravity_lead_get_status",
			"antigravity_lead_await_response",
			"antigravity_lead_list_conversations",
			"antigravity_lead_compact_conversation",
			"antigravity_lead_stop_conversation",
			"antigravity_fleet_get_quota",
			"antigravity_fleet_switch_account",
			"antigravity_session_model_list",
			"antigravity_session_model_get",
			"antigravity_session_model_set",
			"local_read_file",
			"local_list_directory",
			"local_search_files",
			"local_system_info",
			"local_http_allowlist",
			"local_obsidian_list_vaults",
			"local_obsidian_search",
			"local_obsidian_read_note",
		},
	}
}

// MachineControlApprovalRequiredError is returned when a machine-control tool
// was not explicitly enabled in the user's bridge configuration.
type MachineControlApprovalRequiredError struct {
	Tool string
}

func (e *MachineControlApprovalRequiredError) Error() string {
	return fmt.Sprintf("APPROVAL_REQUIRED: machine-control tool %q is not explicitly allowed", e.Tool)
}

func (e *MachineControlApprovalRequiredError) Code() string {
	return "APPROVAL_REQUIRED"
}

var machineControlTools = map[string]struct{}{
	"local_run_command":         {},
	"local_write_file":          {},
	"local_computer_use":        {},
	"local_http_request":        {},
	"local_obsidian_write_note": {},
	"antigravity_computer_use":  {},
}

func isMachineControlTool(name string) bool {
	_, ok := machineControlTools[name]
	return ok
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
		authCodes:   make(map[string]authCodeEntry),
		injector:    gui.NewInjector(0),
		directHdl:   NewDirectHandler(),
		leadHdl:     NewLeadHandler(""),
		cliHdl:      NewCLIHandler(),
		remoteMgr:   NewRemoteManager(""),
		channelsMgr: NewChannelsManager(""),
	}
	s.a2aMgr = NewA2ATaskManager(func() agentBackend {
		return s.agent()
	})
	s.remoteMgr.SetLeadHandler(s.leadHdl)
	// Custom models run on the local LLM gateway; the CLI is pointed at it per turn.
	s.cliHdl.SetGatewayEnv(s.cliGatewayEnv)
	_ = s.loadConfig()

	s.tunnelMgr = NewTunnelManager(s.config.Port, "/api/bridge/mcp")
	s.tunnelMgr.SetOnAutoStop(func() {
		s.mu.Lock()
		s.config.Enabled = false
		s.config.TunnelType = ""
		s.config.TunnelURL = ""
		_ = s.saveConfig()
		s.mu.Unlock()
	})
	s.authMgr = NewAuthManager(s.config.AuthToken, s.config.PairingPIN, func(token, pin string) error {
		s.mu.Lock()
		s.config.AuthToken = token
		s.config.PairingPIN = pin
		s.mu.Unlock()
		return s.saveConfig()
	})
	s.authMgr.SetUseAppPassword(s.config.UseAppPassword)

	// Issue #120: "loopback" is no longer a valid tunnel type. Skip it (and empty) so a stale
	// config value from an older build never triggers a failed auto-start.
	if (s.config.AutoStartTunnel || (!gui.IsTestExecution() && s.config.Enabled)) && s.config.TunnelType != "" && s.config.TunnelType != "loopback" {
		tunnelType := s.config.TunnelType
		tunnelURL := s.config.TunnelURL
		go func() {
			_, _ = s.StartTunnel(tunnelType, tunnelURL)
		}()
	}

	if !gui.IsTestExecution() {
		s.syncDiscordGateway()
	}

	return s, nil
}

// SetPort updates the target daemon loopback port used by Gemini Bridge's tunnel manager.
func (s *Service) SetPort(port int) {
	if port <= 0 {
		return
	}
	s.mu.Lock()
	s.config.Port = port
	s.mu.Unlock()
	if s.tunnelMgr != nil {
		s.tunnelMgr.SetPort(port)
	}
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

// ServeRemoteHTTP delegates /remote HTTP requests to the RemoteManager web console.
func (s *Service) ServeRemoteHTTP(w http.ResponseWriter, r *http.Request) {
	s.remoteMgr.ServeHTTP(w, r)
}

// GetChannelsStatus returns operational status for WhatsApp, Telegram, Discord.
func (s *Service) GetChannelsStatus() ChannelsStatus {
	return s.channelsMgr.GetStatus()
}

// RecordChannelActivity notes that a channel delivered or received a message, so the
// status page shows real activity.
func (s *Service) RecordChannelActivity(channel string) {
	s.channelsMgr.RecordActivity(channel)
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
	err := s.channelsMgr.UpdateConfig(cfg)
	if err == nil {
		s.syncDiscordGateway()
	}
	return err
}

// TestChannel executes connectivity check for a given channel.
func (s *Service) TestChannel(name string) (string, error) {
	res, err := s.channelsMgr.TestChannel(name)
	if err == nil && strings.ToLower(name) == "discord" {
		s.syncDiscordGateway()
	}
	return res, err
}

// syncDiscordGateway starts or stops the Discord Gateway WebSocket listener based on configuration.
func (s *Service) syncDiscordGateway() {
	s.discordGWMu.Lock()
	defer s.discordGWMu.Unlock()

	cfg := s.channelsMgr.GetConfig()
	token := strings.TrimSpace(cfg.Discord.BotToken)
	cleanToken := strings.TrimPrefix(token, "Bot ")
	shouldRun := cfg.Discord.Enabled && cleanToken != "" && cleanToken != "[REDACTED]" && cleanToken != "******"

	if s.discordGW != nil {
		if !shouldRun || s.discordGW.token != cleanToken {
			if s.discordGWCancel != nil {
				s.discordGWCancel()
				s.discordGWCancel = nil
			}
			s.discordGW.Stop()
			s.discordGW = nil
		}
	}

	if shouldRun && s.discordGW == nil {
		ctx, cancel := context.WithCancel(context.Background())
		s.discordGWCancel = cancel
		gwURL := os.Getenv("DISCORD_GATEWAY_URL")
		if gwURL == "" {
			gwURL = "wss://gateway.discord.gg/?v=10&encoding=json"
		}
		gw := NewDiscordGatewayClient(gwURL, cleanToken, func(msg DiscordMessage) {
			go s.RelayDiscordMessage(cfg, msg)
		}, func(username, id string) {
			s.channelsMgr.SetVerifiedIdentity("discord", username)
		})
		s.discordGW = gw
		go gw.Start(ctx)
	}
}

// stopDiscordGateway terminates the active Discord Gateway client connection.
func (s *Service) stopDiscordGateway() {
	s.discordGWMu.Lock()
	defer s.discordGWMu.Unlock()
	if s.discordGW != nil {
		if s.discordGWCancel != nil {
			s.discordGWCancel()
			s.discordGWCancel = nil
		}
		s.discordGW.Stop()
		s.discordGW = nil
	}
}

// RelayDiscordMessage processes an inbound Discord message through AnswerChannelMessage
// and posts the generated response back to the originating channel.
func (s *Service) RelayDiscordMessage(cfg ChannelsConfig, msg DiscordMessage) {
	if msg.Author.Bot {
		return
	}
	text := strings.TrimSpace(msg.Content)
	if text == "" {
		return
	}
	if !discordChannelAllowed(cfg.Discord.AllowedChannels, msg.ChannelID) {
		log.Printf("[channels] discord message from channel %s dropped: not in allowed channels", msg.ChannelID)
		return
	}
	if !discordUserAllowed(cfg.Discord.AllowedUsers, msg.Author.ID) {
		log.Printf("[channels] discord message from user %s dropped: not in allowed users", msg.Author.ID)
		return
	}
	if !MarkMessageSeen("dc-" + msg.ID) {
		return
	}

	reply, err := s.AnswerChannelMessage(text, cfg.DefaultProject)
	if err != nil {
		log.Printf("[channels] discord relay failed for %s: %v", msg.ID, err)
		if sendErr := SendDiscordReply(cfg.Discord.BotToken, msg.ChannelID, "The agent could not answer that message. Check the bridge logs."); sendErr != nil {
			log.Printf("[channels] discord failure notice could not be sent: %v", sendErr)
		}
		return
	}
	if err := SendDiscordReply(cfg.Discord.BotToken, msg.ChannelID, reply); err != nil {
		log.Printf("[channels] discord reply failed for %s: %v", msg.ID, err)
		return
	}
	s.RecordChannelActivity("discord")
}

// GetRawAuthToken returns the raw unmasked auth token (for internal authentication testing).
func (s *Service) GetRawAuthToken() string {
	return s.authMgr.GetToken()
}

// GetRawPairingPIN returns the raw unmasked pairing PIN (for internal authentication testing).
func (s *Service) GetRawPairingPIN() string {
	return s.authMgr.GetPIN()
}

// GetOAuthClientID returns the configured OAuth client ID.
func (s *Service) GetOAuthClientID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getOAuthClientID()
}

func (s *Service) getOAuthClientID() string {
	if s.config.OAuthClientID != "" {
		return s.config.OAuthClientID
	}
	return "antigravity-gemini-bridge"
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
	if c.HTTPAllowlistMode == "" || (c.HTTPAllowlistMode != "whitelist_only" && c.HTTPAllowlistMode != "always_allow") {
		c.HTTPAllowlistMode = "always_allow"
	}
	if normalized := normalizeAgentBackend(c.AgentBackend); normalized != agentUnknown {
		c.AgentBackend = normalized
	} else {
		c.AgentBackend = AgentAuto
	}
	if c.Port <= 0 {
		c.Port = 8765
	}
	if c.AuthToken == "" {
		c.AuthToken = GenerateSecureToken()
	}
	upgradedPIN := false
	if c.PairingPIN == "" || (!gui.IsTestExecution() && !IsAlphanumericPIN8(c.PairingPIN)) {
		c.PairingPIN = GeneratePairingPIN()
		upgradedPIN = true
	}
	if c.OAuthClientID == "" {
		c.OAuthClientID = "antigravity-gemini-bridge"
	}
	allowedToolsBeforeMerge := len(c.AllowedTools)
	c.AllowedTools = mergeDefaultAllowedTools(c.AllowedTools)
	// Issue #120 review: never carry over a stale loopback endpoint persisted by an older
	// build — the config API must not serve one.
	c.TunnelURL = sanitizeEndpointURL(c.TunnelURL)
	s.config = c
	if upgradedPIN || len(c.AllowedTools) > allowedToolsBeforeMerge {
		_ = s.saveConfig()
	}
	return nil
}

func (s *Service) saveConfig() error {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateConfig(s.configPath, data)
}

func writePrivateConfig(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if dir != "." {
		if err := os.Chmod(dir, 0700); err != nil {
			return err
		}
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// GetConfig returns current bridge configuration.
func (s *Service) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates and persists bridge configuration.
func (s *Service) UpdateConfig(c Config) error {
	// Issue #120 review: the config API accepts external input — strip any loopback endpoint
	// before it can be persisted or echoed back to callers.
	c.TunnelURL = sanitizeEndpointURL(c.TunnelURL)
	s.mu.Lock()
	if c.Port <= 0 {
		c.Port = s.config.Port
	}
	if c.OperatingMode == "" {
		c.OperatingMode = s.config.OperatingMode
	}
	if c.HTTPAllowlistMode != "" {
		if c.HTTPAllowlistMode == "whitelist_only" {
			c.HTTPAllowlistMode = "whitelist_only"
		} else {
			c.HTTPAllowlistMode = "always_allow"
		}
	} else {
		c.HTTPAllowlistMode = s.config.HTTPAllowlistMode
	}
	if normalized := normalizeAgentBackend(c.AgentBackend); normalized != agentUnknown {
		c.AgentBackend = normalized
	} else {
		c.AgentBackend = s.config.AgentBackend
	}
	if c.AuthToken == "" {
		c.AuthToken = s.config.AuthToken
	}
	if c.PairingPIN == "" {
		c.PairingPIN = s.config.PairingPIN
	}
	if len(c.AllowedTools) == 0 {
		c.AllowedTools = s.config.AllowedTools
	} else {
		c.AllowedTools = mergeDefaultAllowedTools(c.AllowedTools)
	}
	if c.OAuthClientID == "" {
		c.OAuthClientID = "antigravity-gemini-bridge"
	}
	s.config = c
	err := s.saveConfig()
	s.mu.Unlock()
	if s.authMgr != nil {
		s.authMgr.UpdateCredentials(c.AuthToken, c.PairingPIN, c.UseAppPassword)
	}
	return err
}

// SetUseAppPassword toggles whether Gemini Bridge accepts the App Access Password in place of the simple Pairing PIN.
func (s *Service) SetUseAppPassword(use bool) error {
	s.mu.Lock()
	s.config.UseAppPassword = use
	err := s.saveConfig()
	s.mu.Unlock()
	if s.authMgr != nil {
		s.authMgr.SetUseAppPassword(use)
	}
	return err
}

// StartTunnel initiates remote tunneling.
func (s *Service) StartTunnel(tType, customURL string) (TunnelStatus, error) {
	s.tunnelMu.Lock()
	defer s.tunnelMu.Unlock()

	st, err := s.tunnelMgr.Start(tType, customURL)
	if err != nil {
		if !st.Active {
			s.mu.Lock()
			s.config.Enabled = false
			s.config.TunnelType = ""
			s.config.TunnelURL = ""
			_ = s.saveConfig()
			s.mu.Unlock()
		}
		return st, err
	}
	s.mu.Lock()
	s.config.Enabled = true
	s.config.TunnelType = st.Type
	if st.GatewayURL != "" {
		s.config.TunnelURL = st.GatewayURL
	}
	_ = s.saveConfig()
	s.mu.Unlock()
	return st, nil
}

// StopTunnel terminates remote tunneling.
func (s *Service) StopTunnel() {
	s.tunnelMu.Lock()
	defer s.tunnelMu.Unlock()

	s.tunnelMgr.Stop()
	s.mu.Lock()
	s.config.Enabled = false
	s.config.TunnelType = ""
	s.config.TunnelURL = ""
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

// Stop gracefully shuts down all bridge tunnels and active gateway connections.
func (s *Service) Stop() {
	s.StopTunnel()
	s.StopRemoteTunnel()
	s.stopDiscordGateway()
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

	appPassEnabled := IsAppPasswordConfigured()

	// Computed inside the lock through the pure resolver: calling the lock-taking
	// helpers here would re-enter the read lock.
	requestedAgent := normalizeAgentBackend(s.config.AgentBackend)
	if requestedAgent == agentUnknown {
		requestedAgent = AgentAuto
	}
	resolvedAgent := resolveAgentBackend(requestedAgent, s.cliHdl != nil && s.cliHdl.Available())

	return BridgeStatus{
		Enabled:              tunnelStatus.Active,
		OperatingMode:        s.config.OperatingMode,
		AgentBackend:         requestedAgent,
		ResolvedAgent:        resolvedAgent,
		GatewayURL:           gatewayURL,
		TunnelType:           tunnelStatus.Type,
		TunnelOnline:         tunnelStatus.Active,
		AuthToken:            s.authMgr.GetToken(),
		PairingPIN:           s.authMgr.GetPIN(),
		UseAppPassword:       s.config.UseAppPassword && appPassEnabled,
		AppPasswordEnabled:   appPassEnabled,
		OAuthClientID:        s.getOAuthClientID(),
		OAuthClientSecret:    s.authMgr.GetToken(),
		Surfaces:             surfaces,
		Tunnel:               tunnelStatus,
		ActiveLeadSession:    s.leadHdl.GetActiveSession(),
		RecentAttempts:       s.authMgr.GetRecentAttempts(),
		InvocationsCount:     len(s.invocations),
		RecentInvocations:    recent,
		HTTPAllowlistMode:    s.HTTPAllowlistMode(),
		HTTPAllowlist:        append([]string{}, s.config.HTTPAllowlist...),
		HTTPPendingApprovals: append([]HTTPApprovalRequest{}, s.config.HTTPPendingApprovals...),
	}
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
			"name":        "antigravity_bridge_set_agent",
			"description": "Optional configuration: switches the default agent backend between 'auto' (prefers CLI when available), 'app' (desktop GUI), and 'cli'. The bridge already activates the optimal backend by default. Only call this tool if the user explicitly asks to switch between the Antigravity desktop app and CLI.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"auto", "app", "cli"},
						"description": "Preferred backend.",
					},
				},
				"required": []string{"agent"},
			},
		},
		map[string]interface{}{
			"name":        "antigravity_bridge_get_mode",
			"description": "Returns current operational mode, active Antigravity session status, and host metrics.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
			"annotations": map[string]interface{}{
				"readOnly": true,
			},
		},
	)

	// Fleet tools are available in every operating mode.
	tools = append(tools,
		map[string]interface{}{
			"name":        "antigravity_fleet_get_quota",
			"description": "Returns live quota health, 5-hour and weekly remaining percentages, per-model breakdown (including Claude Opus/Sonnet and Gemini Argon), and active Google account status across the Antigravity fleet. Pass {\"force_refresh\": true} to poll live Google servers immediately.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"force_refresh": map[string]interface{}{
						"type":        "boolean",
						"description": "Force an immediate real-time poll from Google servers instead of reading cache.",
					},
				},
			},
			"annotations": map[string]interface{}{
				"readOnly": true,
			},
		},
	)

	// Lead / Messenger & A2A Task tools
	if mode == "hybrid" || mode == "lead" {
		tools = append(tools,
			map[string]interface{}{
				"name":        "a2a_assign_task",
				"description": "Delegates a high-level goal or task to Antigravity for autonomous planning, execution, and verification. Use this instead of micro-managing individual shell commands or file edits. Returns a task ID.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prompt": map[string]interface{}{
							"type":        "string",
							"description": "High-level goal or task instructions to execute.",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Short descriptive title for the task.",
						},
						"skill": map[string]interface{}{
							"type":        "string",
							"enum":        []string{SkillSoftwareEngineering, SkillCodeReview, SkillGeneralAutomation},
							"description": "Optional skill domain (default: software_engineering).",
						},
					},
					"required": []string{"prompt"},
				},
			},
			map[string]interface{}{
				"name":        "a2a_get_task_status",
				"description": "Checks the execution status, progress, and result of an autonomous A2A task.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task_id": map[string]interface{}{
							"type":        "string",
							"description": "Task ID returned by a2a_assign_task.",
						},
					},
					"required": []string{"task_id"},
				},
			},
			map[string]interface{}{
				"name":        "a2a_await_task",
				"description": "Waits for an autonomous A2A task to finish and returns the final response and generated artifacts to forward to the user.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task_id": map[string]interface{}{
							"type":        "string",
							"description": "Task ID to wait on.",
						},
						"timeout_seconds": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum seconds to wait (default 180).",
						},
					},
					"required": []string{"task_id"},
				},
			},
			map[string]interface{}{
				"name":        "a2a_send_task_message",
				"description": "Sends follow-up guidance or user clarification to an ongoing autonomous A2A task.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task_id": map[string]interface{}{
							"type":        "string",
							"description": "Target task ID.",
						},
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Message or clarification to dispatch.",
						},
					},
					"required": []string{"task_id", "message"},
				},
			},
			map[string]interface{}{
				"name":        "a2a_cancel_task",
				"description": "Cancels an ongoing autonomous A2A task.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task_id": map[string]interface{}{
							"type":        "string",
							"description": "Task ID to cancel.",
						},
					},
					"required": []string{"task_id"},
				},
			},
			map[string]interface{}{
				"name":        "a2a_list_tasks",
				"description": "Lists recent autonomous A2A tasks and their execution states.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Optional status filter (submitted, working, completed, failed, canceled).",
						},
					},
				},
			},
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
			map[string]interface{}{
				"name":        "antigravity_lead_compact_conversation",
				"description": "Directly invokes the native /compact command in the active Antigravity 2.0 conversation to summarize history and compress the context window.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Target conversation ID (optional, defaults to active session).",
						},
					},
				},
			},
			map[string]interface{}{
				"name":        "antigravity_lead_stop_conversation",
				"description": "Stops or cancels an in-progress agent generation turn in the active Antigravity 2.0 conversation.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Target conversation ID (optional, defaults to active session).",
						},
					},
				},
			},
			map[string]interface{}{
				"name":        "antigravity_fleet_switch_account",
				"description": "Switches the active Google account in Antigravity to a specified email or rotates to the next healthy standby account.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"email": map[string]interface{}{
							"type":        "string",
							"description": "Target account email to activate (omit to pick the top standby account).",
						},
					},
				},
			},
			map[string]interface{}{
				"name":        "antigravity_session_model_list",
				"description": "Lists every model the open Antigravity conversation can use, with the numeric value to pass to antigravity_session_model_set and the currently selected one.",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			map[string]interface{}{
				"name":        "antigravity_session_model_get",
				"description": "Returns the model the open Antigravity conversation will use for its next turn.",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			map[string]interface{}{
				"name":        "antigravity_session_model_set",
				"description": "Selects the model for the conversation that is open right now — for this session only, nothing is bound to the project or persisted as a global default. Pass a label (\"Gemini 3.8 Flash\") or a numeric value from antigravity_session_model_list; optional effort picks the (High)/(Medium)/(Low) variant.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"model":  map[string]interface{}{"type": "string", "description": "Model label to select."},
						"value":  map[string]interface{}{"type": "integer", "description": "Numeric model value from antigravity_session_model_list."},
						"effort": map[string]interface{}{"type": "string", "description": "Reasoning effort variant: high, medium or low."},
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
				"name":        "local_search_files",
				"description": "Searches files in a local directory for a text or regex pattern and returns matching file paths, line numbers, and snippets.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "Text or regular expression to search for.",
						},
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory to search (default: current directory).",
							"default":     ".",
						},
						"max_results": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of matches to return (default: 50).",
							"default":     50,
						},
					},
					"required": []string{"pattern"},
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
				"description": "Drives the local desktop: screenshot (full or region), mouse move/click/drag/scroll, type text, press keys and hotkeys, wait, and read/write the clipboard. Use it for tasks that only a human would otherwise do in a GUI.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"status", "screenshot", "move", "click", "drag", "scroll", "type", "key", "hotkey", "wait", "clipboard_get", "clipboard_set"},
							"description": "Action to perform.",
						},
						"x":       map[string]interface{}{"type": "integer", "description": "Target X coordinate for move/click/drag/scroll."},
						"y":       map[string]interface{}{"type": "integer", "description": "Target Y coordinate for move/click/drag/scroll."},
						"to_x":    map[string]interface{}{"type": "integer", "description": "Drag end X coordinate."},
						"to_y":    map[string]interface{}{"type": "integer", "description": "Drag end Y coordinate."},
						"button":  map[string]interface{}{"type": "string", "enum": []string{"left", "middle", "right"}, "description": "Mouse button for click/drag (default: left)."},
						"amount":  map[string]interface{}{"type": "integer", "description": "Scroll amount in wheel clicks (positive scrolls down)."},
						"text":    map[string]interface{}{"type": "string", "description": "Text to type, or clipboard content for clipboard_set."},
						"keys":    map[string]interface{}{"type": "string", "description": "Key or key combo for key/hotkey, e.g. \"Return\", \"ctrl+c\", \"alt+Tab\"."},
						"seconds": map[string]interface{}{"type": "number", "description": "Seconds to wait for the wait action (max 30)."},
						"path":    map[string]interface{}{"type": "string", "description": "Output path for screenshot."},
						"region":  map[string]interface{}{"type": "string", "description": "Screenshot region as WxH+X+Y."},
					},
					"required": []string{"action"},
				},
			},
			map[string]interface{}{
				"name":        "local_http_request",
				"description": "Performs an outbound HTTP request (GET/POST/PUT/PATCH/DELETE) with headers and body — calendar APIs, REST endpoints, webhooks. Hosts must be on the outbound allowlist; an unknown host is NOT called and instead returns an approval request the user grants in the desktop app or via local_http_allowlist.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"method":          map[string]interface{}{"type": "string", "description": "HTTP method (default: GET)."},
						"url":             map[string]interface{}{"type": "string", "description": "Absolute https URL."},
						"headers":         map[string]interface{}{"type": "object", "description": "Request headers, e.g. {\"Authorization\":\"Bearer ...\"}."},
						"body":            map[string]interface{}{"type": "string", "description": "Raw request body (JSON is sent with Content-Type: application/json when no header is given)."},
						"timeout_seconds": map[string]interface{}{"type": "integer", "description": "Request timeout in seconds (default 30, max 120)."},
						"reason":          map[string]interface{}{"type": "string", "description": "Human-readable reason shown to the user when a new host needs approval."},
					},
					"required": []string{"url"},
				},
			},
			map[string]interface{}{
				"name":        "local_http_allowlist",
				"description": "Reads or edits the outbound HTTP allowlist. Use action \"allow\" only after the user agreed to contact a new host; \"revoke\" removes it; \"list\" returns the allowlist plus pending approval requests.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{"type": "string", "enum": []string{"list", "allow", "revoke", "deny"}, "description": "Action to perform (default: list)."},
						"host":   map[string]interface{}{"type": "string", "description": "Host to allow, revoke or deny, e.g. \"www.googleapis.com\" or \"*.notion.so\"."},
					},
				},
			},
			map[string]interface{}{
				"name":        "local_obsidian_list_vaults",
				"description": "Lists every Obsidian vault on this machine (directories with a .obsidian folder) with their note counts.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"refresh": map[string]interface{}{"type": "boolean", "description": "Rescan instead of using the one-minute cache."},
					},
				},
			},
			map[string]interface{}{
				"name":        "local_obsidian_search",
				"description": "Searches Obsidian notes by title and body, and understands #tags and [[wikilinks]] as query forms.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{"type": "string", "description": "Text to find, or \"#tag\", or \"[[note name]]\"."},
						"vault": map[string]interface{}{"type": "string", "description": "Vault name or absolute path (optional, defaults to all vaults)."},
						"limit": map[string]interface{}{"type": "integer", "description": "Max matches (default 30, max 200)."},
					},
					"required": []string{"query"},
				},
			},
			map[string]interface{}{
				"name":        "local_obsidian_read_note",
				"description": "Reads one Obsidian note as Markdown. Accepts an absolute path, \"vault:relative/note\", or a path inside the vault.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{"type": "string", "description": "Note path or \"vault:note\" reference."},
						"note": map[string]interface{}{"type": "string", "description": "Alias for path."},
					},
				},
			},
			map[string]interface{}{
				"name":        "local_obsidian_write_note",
				"description": "Creates or updates one Obsidian note (overwrite, append or prepend). Missing folders are created.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path":    map[string]interface{}{"type": "string", "description": "Note path or \"vault:note\" reference."},
						"content": map[string]interface{}{"type": "string", "description": "Markdown content to write."},
						"mode":    map[string]interface{}{"type": "string", "enum": []string{"overwrite", "append", "prepend"}, "description": "Write mode (default: overwrite)."},
					},
					"required": []string{"path", "content"},
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

	// Machine-control tools are denied unless the user explicitly listed them.
	if isMachineControlTool(name) && !toolInList(name, allowedTools) {
		return "", &MachineControlApprovalRequiredError{Tool: name}
	}

	// Enforce AllowedTools restriction for the remaining tools.
	if len(allowedTools) > 0 && !toolInList(name, allowedTools) {
		return "", fmt.Errorf("tool %q is not in the allowed tools list", name)
	}

	// Mode compatibility enforcement
	isDirect := strings.HasPrefix(name, "local_") || name == "antigravity_inspect_workspace" || name == "antigravity_system_info" || name == "antigravity_computer_use"
	isLead := strings.HasPrefix(name, "antigravity_lead_") || strings.HasPrefix(name, "a2a_") || name == "antigravity_dispatch_task" || name == "antigravity_list_sessions" || name == "antigravity_fleet_switch_account"
	if currentMode == "lead" && isDirect {
		return "", fmt.Errorf("tool %q is disabled in %q operational mode", name, currentMode)
	}
	if currentMode == "direct" && isLead {
		return "", fmt.Errorf("tool %q is disabled in %q operational mode", name, currentMode)
	}

	switch name {
	// A2A High-Level Task Delegation
	case "a2a_assign_task":
		prompt, _ := args["prompt"].(string)
		title, _ := args["title"].(string)
		skill, _ := args["skill"].(string)
		task, cErr := s.a2aMgr.CreateTask(skill, prompt, title)
		if cErr != nil {
			err = cErr
			break
		}
		data, _ := json.MarshalIndent(task, "", "  ")
		result = string(data)
		opMode = "a2a"

	case "a2a_get_task_status":
		taskID, _ := args["task_id"].(string)
		task, gErr := s.a2aMgr.GetTask(taskID)
		if gErr != nil {
			err = gErr
			break
		}
		data, _ := json.MarshalIndent(task, "", "  ")
		result = string(data)
		opMode = "a2a"

	case "a2a_await_task":
		taskID, _ := args["task_id"].(string)
		timeoutSec, _ := args["timeout_seconds"].(float64)
		if timeoutSec <= 0 {
			timeoutSec = 180
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		defer cancel()
		task, aErr := s.a2aMgr.AwaitTask(ctx, taskID)
		if aErr != nil {
			err = aErr
			break
		}
		data, _ := json.MarshalIndent(task, "", "  ")
		result = string(data)
		opMode = "a2a"

	case "a2a_send_task_message":
		taskID, _ := args["task_id"].(string)
		msg, _ := args["message"].(string)
		task, sErr := s.a2aMgr.SendTaskMessage(taskID, msg)
		if sErr != nil {
			err = sErr
			break
		}
		data, _ := json.MarshalIndent(task, "", "  ")
		result = string(data)
		opMode = "a2a"

	case "a2a_cancel_task":
		taskID, _ := args["task_id"].(string)
		task, cErr := s.a2aMgr.CancelTask(taskID)
		if cErr != nil {
			err = cErr
			break
		}
		data, _ := json.MarshalIndent(task, "", "  ")
		result = string(data)
		opMode = "a2a"

	case "a2a_list_tasks":
		stFilter, _ := args["status"].(string)
		tasks := s.a2aMgr.ListTasks(stFilter, 10)
		data, _ := json.MarshalIndent(map[string]interface{}{"tasks": tasks}, "", "  ")
		result = string(data)
		opMode = "a2a"
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

	case "antigravity_bridge_set_agent":
		agent, _ := args["agent"].(string)
		resolved, agentErr := s.SetAgentBackend(agent)
		if agentErr != nil {
			err = agentErr
			break
		}
		payload := map[string]interface{}{
			"status":         "ok",
			"agent_backend":  resolved,
			"resolved_agent": s.ResolvedAgent(),
			"cli_available":  s.cliHdl != nil && s.cliHdl.Available(),
			"app_available":  s.GetStatus().Surfaces.DesktopAppConnected,
		}
		result = marshalPayload(payload)
		opMode = "control"

	case "antigravity_fleet_get_quota":
		result, err = s.getFleetQuota()
		opMode = "fleet"

	case "antigravity_fleet_switch_account":
		result, err = s.switchFleetAccount(args)
		opMode = "fleet"

	// Lead / Messenger tools — routed to the resolved Antigravity backend (app or CLI).
	case "antigravity_lead_create_conversation", "antigravity_dispatch_task":
		result, err = s.agent().CreateConversation(args)
		opMode = "lead"

	case "antigravity_lead_send_message":
		result, err = s.agent().SendMessage(args)
		opMode = "lead"

	case "antigravity_lead_get_status":
		result, err = s.agent().GetStatus(args)
		opMode = "lead"

	case "antigravity_lead_await_response":
		result, err = s.agent().AwaitResponse(args)
		opMode = "lead"

	case "antigravity_lead_list_conversations", "antigravity_list_sessions":
		result, err = s.agent().ListConversations(args)
		opMode = "lead"

	case "antigravity_lead_compact_conversation":
		result, err = s.agent().CompactConversation(args)
		opMode = "lead"

	case "antigravity_lead_stop_conversation":
		result, err = s.agent().StopConversation(args)
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

	case "local_search_files":
		result, err = s.directHdl.SearchFiles(args)
		opMode = "direct"

	case "local_system_info", "antigravity_system_info":
		result, err = s.directHdl.SystemInfo()
		opMode = "direct"

	case "local_computer_use", "antigravity_computer_use":
		result, err = s.directHdl.ComputerUse(args)
		opMode = "direct"

	case "local_http_request":
		result, err = s.HTTPRequest(args)
		opMode = "direct"

	case "local_http_allowlist":
		result, err = s.HTTPAllowlistCommand(args)
		opMode = "direct"

	case "local_obsidian_list_vaults":
		result, err = s.directHdl.ObsidianListVaults(args)
		opMode = "direct"

	case "local_obsidian_search":
		result, err = s.directHdl.ObsidianSearch(args)
		opMode = "direct"

	case "local_obsidian_read_note":
		result, err = s.directHdl.ObsidianReadNote(args)
		opMode = "direct"

	case "local_obsidian_write_note":
		result, err = s.directHdl.ObsidianWriteNote(args)
		opMode = "direct"

	case "antigravity_session_model_list":
		result, err = s.agent().ListModels(args)
		opMode = "lead"

	case "antigravity_session_model_get":
		result, err = s.agent().GetModel(args)
		opMode = "lead"

	case "antigravity_session_model_set":
		result, err = s.agent().SetModel(args)
		opMode = "lead"

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
		protoVer := "2024-11-05"
		if reqVer, ok := r.Params["protocolVersion"].(string); ok && reqVer == "2025-03-26" {
			protoVer = "2025-03-26"
		}
		externalBase := strings.TrimRight(s.config.TunnelURL, "/")
		if externalBase == "" && s.tunnelMgr != nil {
			externalBase = strings.TrimRight(s.tunnelMgr.GetStatus().GatewayURL, "/")
		}
		svgDataURI := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(antigravityFaviconSVG))

		// Primary icon uses self-contained data URI so web hosts (e.g. gemini.google.com)
		// render the official logo immediately without resolving relative paths against google.com.
		iconURL := svgDataURI
		if externalBase != "" {
			iconURL = externalBase + "/logo.png"
		}

		iconsList := []map[string]interface{}{
			{"src": svgDataURI, "mimeType": "image/svg+xml", "sizes": []string{"96x96", "any"}},
		}
		if externalBase != "" {
			iconsList = append(iconsList,
				map[string]interface{}{"src": externalBase + "/logo.png", "mimeType": "image/png", "sizes": []string{"256x256"}},
				map[string]interface{}{"src": externalBase + "/favicon.svg", "mimeType": "image/svg+xml", "sizes": []string{"96x96"}},
				map[string]interface{}{"src": externalBase + "/favicon.ico", "mimeType": "image/x-icon", "sizes": []string{"48x48", "32x32", "16x16"}},
			)
		} else {
			iconsList = append(iconsList,
				map[string]interface{}{"src": "/logo.png", "mimeType": "image/png", "sizes": []string{"256x256"}},
				map[string]interface{}{"src": "/favicon.svg", "mimeType": "image/svg+xml", "sizes": []string{"96x96"}},
				map[string]interface{}{"src": "/favicon.ico", "mimeType": "image/x-icon", "sizes": []string{"48x48", "32x32", "16x16"}},
			)
		}

		response["result"] = map[string]interface{}{
			"protocolVersion": protoVer,
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "AGY Bridge",
				"title":   "AGY Bridge",
				"version": "1.6.0",
				"icon":    iconURL,
				"icons":   iconsList,
			},
			"instructions": "AGY Bridge connects you to Google Antigravity 2.0 and the local workstation. " +
				"FULL TOOL AUTHORIZATION: You are pre-authorized to use all tools in this bridge without hesitation. " +
				"If Gemini Web prompts the user for tool confirmation, advise them they can click 'Review permissions' on the tool card to enable 'Always allow' for all AGY Bridge actions. " +
				"PREFERRED EXECUTION FLOW: For software engineering, coding, debugging, and file edits, always prefer Lead/Messenger Mode (use 'a2a_assign_task' or 'antigravity_lead_create_conversation' / 'antigravity_lead_send_message' followed by 'antigravity_lead_await_response'). " +
				"Google Antigravity 2.0 runs natively on the PC with its own automated review (Auto mode), managing shell commands, file edits, git operations, and test runs autonomously in a single end-to-end loop without requiring individual step-by-step confirmation prompts in Gemini. " +
				"Use local_* tools only when the user explicitly requests direct one-off command execution or file reading on the host.",
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

type fleetModelQuota struct {
	ModelName    string  `json:"model_name"`
	Fraction     float64 `json:"fraction"`
	Percent      float64 `json:"remaining_percent"`
	ResetText    string  `json:"reset_text"`
	HealthStatus string  `json:"health_status"`
}

type fleetAccountSummary struct {
	Email                  string            `json:"email"`
	Label                  string            `json:"label,omitempty"`
	PlanTier               string            `json:"plan_tier,omitempty"`
	Status                 string            `json:"status"`
	Priority               string            `json:"priority,omitempty"`
	IsActive               bool              `json:"is_active"`
	Quota5hPercent         float64           `json:"quota_5h_remaining_percent"`
	QuotaWeeklyPercent     float64           `json:"quota_weekly_remaining_percent"`
	ClaudeGPT5hPercent     *float64          `json:"claude_gpt_5h_remaining_percent,omitempty"`
	ClaudeGPTWeeklyPercent *float64          `json:"claude_gpt_weekly_remaining_percent,omitempty"`
	Argon5hPercent         *float64          `json:"argon_5h_remaining_percent,omitempty"`
	ArgonWeeklyPercent     *float64          `json:"argon_weekly_remaining_percent,omitempty"`
	Health                 string            `json:"health"`
	Reset5h                string            `json:"reset_5h,omitempty"`
	ResetWeekly            string            `json:"reset_weekly,omitempty"`
	Models                 []fleetModelQuota `json:"models,omitempty"`
	ExhaustedModels        []string          `json:"exhausted_models,omitempty"`
}

func (s *Service) getFleetQuota(args ...map[string]interface{}) (string, error) {
	store, err := keyring.NewStore("")
	if err != nil {
		return "", fmt.Errorf("load account fleet: %w", err)
	}
	accounts := store.ListAccounts()
	active := store.ActiveAccount()
	cfg, _ := core.LoadConfig()
	threshold5h := core.DefaultAutoSwitchThresholdFraction
	thresholdWeekly := core.DefaultAutoSwitchWeeklyThresholdFraction
	allowNonGemini := false
	if cfg != nil {
		if cfg.AutoSwitchThreshold > 0 {
			threshold5h = cfg.AutoSwitchThreshold
		}
		if cfg.AutoSwitchWeeklyThreshold > 0 {
			thresholdWeekly = cfg.AutoSwitchWeeklyThreshold
		}
		allowNonGemini = cfg.AllowNonGeminiNativeModels
	}

	forceRefresh := false
	if len(args) > 0 && args[0] != nil {
		if fr, ok := args[0]["force_refresh"].(bool); ok && fr {
			forceRefresh = true
		} else if r, ok := args[0]["refresh"].(bool); ok && r {
			forceRefresh = true
		}
	}

	cachedMap := quota.LoadQuotaCache()
	activeCached := cachedMap[strings.ToLower(strings.TrimSpace(active))]

	// Auto-refresh if requested, or if active account cache is missing,
	// or if the active cache is older than 45 seconds (ensures real-time quota accuracy).
	shouldRefreshActive := forceRefresh || activeCached == nil
	if !shouldRefreshActive && activeCached != nil {
		if !activeCached.LastPolled.IsZero() && time.Since(activeCached.LastPolled) > 45*time.Second {
			shouldRefreshActive = true
		}
	}

	refreshedLive := false
	if shouldRefreshActive && active != "" {
		if activeAcc, getErr := store.GetAccount(active); getErr == nil && activeAcc != nil {
			if liveSummary, pollErr := quota.PollAndCacheAccount(activeAcc, store); pollErr == nil && liveSummary != nil {
				cachedMap = quota.LoadQuotaCache()
				activeCached = cachedMap[strings.ToLower(strings.TrimSpace(active))]
				refreshedLive = true
			}
		}
	}

	states := quota.BuildAccountQuotaStatesFromMapWithThresholds(accounts, cachedMap, threshold5h, thresholdWeekly, allowNonGemini)
	states = quota.SortAccountQuotaStatesWithThresholds(states, active, threshold5h, thresholdWeekly, "auto", core.DefaultSwitchMode, allowNonGemini)

	fleetAccounts := make([]fleetAccountSummary, 0, len(states))
	for _, state := range states {
		health := quota.ComputeHealth(state.Quota5hAvailable)
		if strings.EqualFold(state.Status, "ERROR") || strings.EqualFold(state.Status, "BANNED") {
			health = core.StatusExhausted
		}
		item := fleetAccountSummary{
			Email:              state.Email,
			Label:              state.Label,
			PlanTier:           state.PlanTier,
			Status:             state.Status,
			Priority:           state.Priority,
			IsActive:           strings.EqualFold(state.Email, active),
			Quota5hPercent:     state.Quota5hCurrent * 100,
			QuotaWeeklyPercent: state.QuotaWeekly * 100,
			Health:             health,
			Reset5h:            state.ResetHorizonText,
			ResetWeekly:        state.ResetHorizonWeeklyText,
		}

		if accCached := cachedMap[strings.ToLower(strings.TrimSpace(state.Email))]; accCached != nil {
			if accCached.ClaudeWeeklyBucketPresent || accCached.Quota5hClaudeGPT > 0 || accCached.QuotaWeeklyClaudeGPT > 0 {
				c5h := accCached.Quota5hClaudeGPT * 100
				cWk := accCached.QuotaWeeklyClaudeGPT * 100
				item.ClaudeGPT5hPercent = &c5h
				item.ClaudeGPTWeeklyPercent = &cWk
			}
			if accCached.Quota5hArgon != nil {
				a5h := *accCached.Quota5hArgon * 100
				item.Argon5hPercent = &a5h
			}
			if accCached.QuotaWeeklyArgon != nil {
				aWk := *accCached.QuotaWeeklyArgon * 100
				item.ArgonWeeklyPercent = &aWk
			}
			for _, m := range accCached.Models {
				pct := math.Round(m.Fraction * 100)
				item.Models = append(item.Models, fleetModelQuota{
					ModelName:    m.ModelName,
					Fraction:     m.Fraction,
					Percent:      pct,
					ResetText:    m.ResetText,
					HealthStatus: m.HealthStatus,
				})
				if m.Fraction <= 0 || strings.EqualFold(m.HealthStatus, core.StatusExhausted) {
					item.ExhaustedModels = append(item.ExhaustedModels, m.ModelName)
				}
			}
		}

		fleetAccounts = append(fleetAccounts, item)
	}

	var activeSessionModel map[string]interface{}
	var activeModelWarning string
	var activeModelExhausted bool

	if currentModelJSON, err := s.agent().GetModel(nil); err == nil && currentModelJSON != "" {
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(currentModelJSON), &parsed) == nil {
			if m, ok := parsed["model"].(map[string]interface{}); ok {
				activeSessionModel = m
				label := strings.ToLower(fmt.Sprintf("%v", m["label"]))
				id := strings.ToLower(fmt.Sprintf("%v", m["id"]))
				isClaude := strings.Contains(label, "claude") || strings.Contains(id, "claude") ||
					strings.Contains(label, "opus") || strings.Contains(id, "opus") ||
					strings.Contains(label, "sonnet") || strings.Contains(id, "sonnet")
				if activeCached != nil {
					if isClaude && (activeCached.Quota5hClaudeGPT <= 0 || activeCached.QuotaWeeklyClaudeGPT <= 0) {
						activeModelExhausted = true
						activeModelWarning = fmt.Sprintf("CRITICAL: Active session model %q quota is exhausted (5h: %.0f%%, weekly: %.0f%%). Antigravity will hang on 'Connecting...'. Switch model to Gemini 3.8 Flash or Gemini 4 Argon using antigravity_session_model_set.", fmt.Sprintf("%v", m["label"]), activeCached.Quota5hClaudeGPT*100, activeCached.QuotaWeeklyClaudeGPT*100)
					}
				}
			}
		}
	}

	res := map[string]interface{}{
		"active_account":         active,
		"active_session_model":   activeSessionModel,
		"active_model_exhausted": activeModelExhausted,
		"active_model_warning":   activeModelWarning,
		"refreshed_live":         refreshedLive,
		"total_accounts":         len(fleetAccounts),
		"accounts":               fleetAccounts,
		"cached_at":              time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Service) switchFleetAccount(args map[string]interface{}) (string, error) {
	store, err := keyring.NewStore("")
	if err != nil {
		return "", fmt.Errorf("load account fleet: %w", err)
	}
	target := strings.TrimSpace(stringValue(args["email"]))
	current := store.ActiveAccount()
	previousStatus := ""
	if current != "" {
		if acc, getErr := store.GetAccount(current); getErr == nil {
			previousStatus = acc.Status
		}
	}

	if target == "" {
		cached := quota.LoadQuotaCache()
		cfg, _ := core.LoadConfig()
		threshold5h := core.DefaultAutoSwitchThresholdFraction
		thresholdWeekly := core.DefaultAutoSwitchWeeklyThresholdFraction
		allowNonGemini := false
		if cfg != nil {
			if cfg.AutoSwitchThreshold > 0 {
				threshold5h = cfg.AutoSwitchThreshold
			}
			if cfg.AutoSwitchWeeklyThreshold > 0 {
				thresholdWeekly = cfg.AutoSwitchWeeklyThreshold
			}
			allowNonGemini = cfg.AllowNonGeminiNativeModels
		}
		states := quota.BuildAccountQuotaStatesFromMapWithThresholds(store.ListAccounts(), cached, threshold5h, thresholdWeekly, allowNonGemini)
		ranked := quota.RankStandbyAccountsWithThresholds(states, threshold5h, thresholdWeekly, core.DefaultSwitchMode, allowNonGemini)
		if len(ranked) == 0 {
			states = quota.SortAccountQuotaStatesWithThresholds(states, store.ActiveAccount(), threshold5h, thresholdWeekly, "auto", core.DefaultSwitchMode, allowNonGemini)
			for _, state := range states {
				if state.IsActive || !strings.EqualFold(state.Status, core.AccountStatusStandby) {
					continue
				}
				acc, getErr := store.GetAccount(state.Email)
				if getErr != nil || (strings.TrimSpace(acc.RefreshToken) == "" && !core.IsRunningTests()) {
					continue
				}
				target = acc.Email
				break
			}
			if target == "" {
				return "", fmt.Errorf("no healthy standby account found")
			}
		} else {
			target = ranked[0].Email
		}
	}

	account, err := store.GetAccount(target)
	if err != nil {
		return "", err
	}
	if err := store.SetActiveAccount(account.Email); err != nil {
		return "", err
	}
	account.IsActive = true
	account.Status = "ACTIVE"
	cfg, cfgErr := core.LoadConfig()
	if cfgErr == nil {
		cfgErr = cfg.SetActiveAppAccount(core.TargetAppAll, account.Email)
	}
	var emails []string
	for _, acc := range store.ListAccounts() {
		emails = append(emails, acc.Email)
	}
	syncErr := keyring.SyncAllSurfaces(account, emails, nil)

	result := map[string]interface{}{
		"status":            "switched",
		"previous_account":  current,
		"active_account":    account.Email,
		"label":             account.Label,
		"plan_tier":         account.PlanTier,
		"previous_status":   previousStatus,
		"account_status":    account.Status,
		"relaunch_required": syncErr == nil,
		"surfaces_synced":   syncErr == nil,
	}
	if cfgErr != nil {
		result["status"] = "switched_with_warnings"
		result["config_error"] = cfgErr.Error()
	}
	if syncErr != nil {
		result["status"] = "switched_with_warnings"
		result["surface_sync_error"] = syncErr.Error()
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func stringValue(v interface{}) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func allowedToolMatches(name, configured string) bool {
	return configured == name ||
		(name == "antigravity_dispatch_task" && configured == "antigravity_lead_create_conversation") ||
		(name == "antigravity_list_sessions" && configured == "antigravity_lead_list_conversations") ||
		(name == "antigravity_inspect_workspace" && configured == "local_list_directory") ||
		(name == "antigravity_system_info" && configured == "local_system_info") ||
		(name == "antigravity_computer_use" && configured == "local_computer_use")
}

func toolInList(name string, allowed []string) bool {
	for _, configured := range allowed {
		if allowedToolMatches(name, configured) {
			return true
		}
	}
	return false
}

func mergeDefaultAllowedTools(configured []string) []string {
	defaults := DefaultConfig().AllowedTools
	allowedSet := make(map[string]bool, len(configured)+len(defaults))
	merged := append([]string(nil), configured...)
	for _, tool := range merged {
		allowedSet[tool] = true
	}
	for _, tool := range defaults {
		if !allowedSet[tool] {
			merged = append(merged, tool)
			allowedSet[tool] = true
		}
	}
	return merged
}

// A2ATaskManager returns the A2A task manager instance.
func (s *Service) A2ATaskManager() *A2ATaskManager {
	return s.a2aMgr
}

// AgentCard returns the discoverable A2A Agent Card.
func (s *Service) AgentCard(baseURL string) A2AAgentCard {
	return DefaultA2AAgentCard(baseURL)
}
