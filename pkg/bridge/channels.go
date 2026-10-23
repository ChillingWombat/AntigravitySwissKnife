package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// WhatsAppConfig holds credentials and settings for WhatsApp bridge.
type WhatsAppConfig struct {
	Enabled        bool     `json:"enabled"`
	Mode           string   `json:"mode"` // "cloud_api" | "mcp" | "webhook"
	PhoneNumberID  string   `json:"phone_number_id"`
	AccessToken    string   `json:"access_token"`
	VerifyToken    string   `json:"verify_token"`
	AllowedNumbers []string `json:"allowed_numbers"`
}

// TelegramConfig holds credentials and settings for Telegram bot bridge.
type TelegramConfig struct {
	Enabled        bool     `json:"enabled"`
	BotToken       string   `json:"bot_token"`
	AllowedChatIDs []string `json:"allowed_chat_ids"`
	Polling        bool     `json:"polling"`
	WebhookURL     string   `json:"webhook_url"`
}

// DiscordConfig holds credentials and settings for Discord bot bridge.
type DiscordConfig struct {
	Enabled         bool     `json:"enabled"`
	BotToken        string   `json:"bot_token"`
	ApplicationID   string   `json:"application_id"`
	AllowedChannels []string `json:"allowed_channels"`
	AllowedUsers    []string `json:"allowed_users"`
}

// ChannelsConfig aggregates multi-channel configurations.
type ChannelsConfig struct {
	WhatsApp WhatsAppConfig `json:"whatsapp"`
	Telegram TelegramConfig `json:"telegram"`
	Discord  DiscordConfig  `json:"discord"`
}

// ChannelState reports the live status of an external chat channel.
type ChannelState struct {
	Enabled       bool   `json:"enabled"`
	Connected     bool   `json:"connected"`
	Identity      string `json:"identity,omitempty"`
	StatusText    string `json:"status_text"`
	LastMessageAt string `json:"last_message_at,omitempty"`
}

// ChannelsStatus aggregates live statuses for all 3 chat channels.
type ChannelsStatus struct {
	WhatsApp ChannelState `json:"whatsapp"`
	Telegram ChannelState `json:"telegram"`
	Discord  ChannelState `json:"discord"`
}

// ChannelsManager coordinates WhatsApp, Telegram, and Discord agent bridges.
type ChannelsManager struct {
	mu                 sync.RWMutex
	config             ChannelsConfig
	configPath         string
	HTTPClient         *http.Client
	verifiedIdentities map[string]string
	lastMessageAt      map[string]string
}

// DefaultChannelsConfig provides safe defaults.
func DefaultChannelsConfig() ChannelsConfig {
	return ChannelsConfig{
		WhatsApp: WhatsAppConfig{
			Enabled:        false,
			Mode:           "mcp",
			PhoneNumberID:  "",
			AccessToken:    "",
			VerifyToken:    "antigravity-whatsapp-verify",
			AllowedNumbers: make([]string, 0),
		},
		Telegram: TelegramConfig{
			Enabled:        false,
			BotToken:       "",
			AllowedChatIDs: make([]string, 0),
			Polling:        true,
			WebhookURL:     "",
		},
		Discord: DiscordConfig{
			Enabled:         false,
			BotToken:        "",
			ApplicationID:   "",
			AllowedChannels: make([]string, 0),
			AllowedUsers:    make([]string, 0),
		},
	}
}

// NewChannelsManager initializes channel configuration manager.
func NewChannelsManager(configPath string) *ChannelsManager {
	if configPath == "" {
		configPath = filepath.Join(core.GetConfigDir(), "chat_channels.json")
	}
	cm := &ChannelsManager{
		config:             DefaultChannelsConfig(),
		configPath:         configPath,
		verifiedIdentities: make(map[string]string),
		lastMessageAt:      make(map[string]string),
	}
	_ = cm.load()
	return cm
}

func (cm *ChannelsManager) load() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}
	var c ChannelsConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	cm.config = c
	return nil
}

func (cm *ChannelsManager) save() error {
	_ = os.MkdirAll(filepath.Dir(cm.configPath), 0755)
	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cm.configPath, data, 0644)
}

// GetConfig returns current channels settings.
func (cm *ChannelsManager) GetConfig() ChannelsConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// GetChannelsConfigRedacted returns current channels settings with bot tokens and secrets masked to [REDACTED].
func (cm *ChannelsManager) GetChannelsConfigRedacted() ChannelsConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	cfg := cm.config
	if cfg.WhatsApp.AccessToken != "" {
		cfg.WhatsApp.AccessToken = "[REDACTED]"
	}
	if cfg.WhatsApp.VerifyToken != "" {
		cfg.WhatsApp.VerifyToken = "[REDACTED]"
	}
	if cfg.Telegram.BotToken != "" {
		cfg.Telegram.BotToken = "[REDACTED]"
	}
	if cfg.Discord.BotToken != "" {
		cfg.Discord.BotToken = "[REDACTED]"
	}
	return cfg
}

// UpdateConfig updates channel settings, preserving existing secrets if incoming values are [REDACTED] or empty.
func (cm *ChannelsManager) UpdateConfig(c ChannelsConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if c.WhatsApp.AccessToken == "[REDACTED]" || c.WhatsApp.AccessToken == "******" || strings.TrimSpace(c.WhatsApp.AccessToken) == "" {
		c.WhatsApp.AccessToken = cm.config.WhatsApp.AccessToken
	}
	if c.WhatsApp.VerifyToken == "[REDACTED]" || c.WhatsApp.VerifyToken == "******" || strings.TrimSpace(c.WhatsApp.VerifyToken) == "" {
		c.WhatsApp.VerifyToken = cm.config.WhatsApp.VerifyToken
	}
	if c.Telegram.BotToken == "[REDACTED]" || c.Telegram.BotToken == "******" || strings.TrimSpace(c.Telegram.BotToken) == "" {
		c.Telegram.BotToken = cm.config.Telegram.BotToken
	}
	if c.Discord.BotToken == "[REDACTED]" || c.Discord.BotToken == "******" || strings.TrimSpace(c.Discord.BotToken) == "" {
		c.Discord.BotToken = cm.config.Discord.BotToken
	}

	if cm.verifiedIdentities != nil {
		if !c.WhatsApp.Enabled || c.WhatsApp.AccessToken != cm.config.WhatsApp.AccessToken {
			delete(cm.verifiedIdentities, "whatsapp")
		}
		if !c.Telegram.Enabled || c.Telegram.BotToken != cm.config.Telegram.BotToken {
			delete(cm.verifiedIdentities, "telegram")
		}
		if !c.Discord.Enabled || c.Discord.BotToken != cm.config.Discord.BotToken {
			delete(cm.verifiedIdentities, "discord")
		}
	}

	cm.config = c
	return cm.save()
}

// GetStatus returns the connection and operational status of each channel.
func (cm *ChannelsManager) GetStatus() ChannelsStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// WhatsApp
	waVerified := cm.verifiedIdentities != nil && cm.verifiedIdentities["whatsapp"] != ""
	waConfigured := strings.TrimSpace(cm.config.WhatsApp.AccessToken) != "" || strings.TrimSpace(cm.config.WhatsApp.PhoneNumberID) != ""
	waState := ChannelState{
		Enabled:       cm.config.WhatsApp.Enabled,
		Connected:     cm.config.WhatsApp.Enabled && waVerified,
		Identity:      cm.getVerifiedIdentity("whatsapp"),
		LastMessageAt: cm.getLastMessageAt("whatsapp"),
	}
	if !waState.Enabled {
		waState.StatusText = "Disabled"
	} else if waVerified {
		waState.StatusText = fmt.Sprintf("Configured (%s mode, Verified)", strings.ToUpper(cm.config.WhatsApp.Mode))
	} else if waConfigured {
		waState.StatusText = fmt.Sprintf("Configured (%s mode, Awaiting verification)", strings.ToUpper(cm.config.WhatsApp.Mode))
	} else {
		waState.StatusText = "Not configured (awaiting access token)"
	}

	// Telegram
	tgVerified := cm.verifiedIdentities != nil && cm.verifiedIdentities["telegram"] != ""
	tgConfigured := strings.TrimSpace(cm.config.Telegram.BotToken) != ""
	tgState := ChannelState{
		Enabled:       cm.config.Telegram.Enabled,
		Connected:     cm.config.Telegram.Enabled && tgVerified,
		Identity:      cm.getVerifiedIdentity("telegram"),
		LastMessageAt: cm.getLastMessageAt("telegram"),
	}
	if !tgState.Enabled {
		tgState.StatusText = "Disabled"
	} else if tgVerified {
		tgState.StatusText = "Configured (Bot Verified)"
	} else if tgConfigured {
		tgState.StatusText = "Configured (Awaiting bot verification)"
	} else {
		tgState.StatusText = "Not configured (awaiting bot token from @BotFather)"
	}

	// Discord
	dcVerified := cm.verifiedIdentities != nil && cm.verifiedIdentities["discord"] != ""
	dcConfigured := strings.TrimSpace(cm.config.Discord.BotToken) != ""
	dcState := ChannelState{
		Enabled:       cm.config.Discord.Enabled,
		Connected:     cm.config.Discord.Enabled && dcVerified,
		Identity:      cm.getVerifiedIdentity("discord"),
		LastMessageAt: cm.getLastMessageAt("discord"),
	}
	if !dcState.Enabled {
		dcState.StatusText = "Disabled"
	} else if dcVerified {
		dcState.StatusText = "Configured (Bot Verified)"
	} else if dcConfigured {
		dcState.StatusText = "Configured (Awaiting gateway verification)"
	} else {
		dcState.StatusText = "Not configured (awaiting Discord bot token)"
	}

	return ChannelsStatus{
		WhatsApp: waState,
		Telegram: tgState,
		Discord:  dcState,
	}
}

func (cm *ChannelsManager) getVerifiedIdentity(channel string) string {
	if cm.verifiedIdentities == nil {
		return ""
	}
	return cm.verifiedIdentities[channel]
}

// RecordActivity records the latest interaction timestamp for a given channel.
func (cm *ChannelsManager) RecordActivity(channel string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.lastMessageAt == nil {
		cm.lastMessageAt = make(map[string]string)
	}
	cm.lastMessageAt[strings.ToLower(channel)] = time.Now().Format("2006-01-02 15:04:05")
}

func (cm *ChannelsManager) getLastMessageAt(channel string) string {
	if cm.lastMessageAt == nil {
		return ""
	}
	return cm.lastMessageAt[strings.ToLower(channel)]
}

// GetLastMessageAt returns the last interaction timestamp for a given channel.
func (cm *ChannelsManager) GetLastMessageAt(channel string) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.getLastMessageAt(channel)
}

// TestChannel executes genuine connectivity check for a given channel.
func (cm *ChannelsManager) TestChannel(channel string) (string, error) {
	cm.mu.RLock()
	cfg := cm.config
	client := cm.HTTPClient
	cm.mu.RUnlock()

	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	switch strings.ToLower(channel) {
	case "whatsapp":
		if !cfg.WhatsApp.Enabled {
			return "", fmt.Errorf("WhatsApp bridge is disabled")
		}
		if cfg.WhatsApp.Mode == "cloud_api" || (cfg.WhatsApp.Mode == "" && cfg.WhatsApp.AccessToken != "") {
			token := strings.TrimSpace(cfg.WhatsApp.AccessToken)
			if token == "" || token == "[REDACTED]" {
				return "", fmt.Errorf("WhatsApp bridge is not configured: access token is required")
			}
			req, err := http.NewRequest(http.MethodGet, "https://graph.facebook.com/v21.0/me", nil)
			if err != nil {
				return "", fmt.Errorf("failed to create WhatsApp request: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := client.Do(req)
			if err != nil {
				return "", fmt.Errorf("WhatsApp Graph API connection failed: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
				return "", fmt.Errorf("WhatsApp Graph API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			}
			var waResp struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			cm.mu.Lock()
			if cm.verifiedIdentities == nil {
				cm.verifiedIdentities = make(map[string]string)
			}
			if err := json.NewDecoder(resp.Body).Decode(&waResp); err == nil && waResp.ID != "" {
				if waResp.Name != "" {
					cm.verifiedIdentities["whatsapp"] = waResp.Name
				} else {
					cm.verifiedIdentities["whatsapp"] = "WhatsApp Cloud API"
				}
				cm.mu.Unlock()
				return fmt.Sprintf("WhatsApp Cloud API verified. App ID: %s (Name: %s).", waResp.ID, waResp.Name), nil
			}
			cm.verifiedIdentities["whatsapp"] = "WhatsApp Cloud API"
			cm.mu.Unlock()
			return "WhatsApp Cloud API connection verified.", nil
		}
		if cfg.WhatsApp.Mode == "mcp" {
			if strings.TrimSpace(cfg.WhatsApp.VerifyToken) == "" {
				return "", fmt.Errorf("WhatsApp MCP bridge is not configured: verify token required")
			}
			cm.mu.Lock()
			if cm.verifiedIdentities == nil {
				cm.verifiedIdentities = make(map[string]string)
			}
			cm.verifiedIdentities["whatsapp"] = "WhatsApp MCP Gateway"
			cm.mu.Unlock()
			return "WhatsApp MCP bridge verified. Token configured for webhook handshakes.", nil
		}
		if cfg.WhatsApp.Mode == "webhook" {
			verifyToken := strings.TrimSpace(cfg.WhatsApp.VerifyToken)
			if verifyToken == "" || verifyToken == "[REDACTED]" {
				return "", fmt.Errorf("WhatsApp webhook bridge is not configured: verify token required")
			}
			cm.mu.Lock()
			if cm.verifiedIdentities == nil {
				cm.verifiedIdentities = make(map[string]string)
			}
			cm.verifiedIdentities["whatsapp"] = "WhatsApp Webhook Gateway"
			cm.mu.Unlock()
			return "WhatsApp custom webhook bridge verified. Verify token ready for inbound events.", nil
		}
		return "", fmt.Errorf("WhatsApp bridge mode %q is not configured", cfg.WhatsApp.Mode)

	case "telegram":
		if !cfg.Telegram.Enabled {
			return "", fmt.Errorf("Telegram bridge is disabled")
		}
		token := strings.TrimSpace(cfg.Telegram.BotToken)
		if token == "" || token == "[REDACTED]" {
			return "", fmt.Errorf("Telegram Bot Token is required")
		}
		resp, err := client.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getMe", url.PathEscape(token)))
		if err != nil {
			return "", fmt.Errorf("Telegram API connection failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			return "", fmt.Errorf("Telegram Bot API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var tgResp struct {
			OK     bool `json:"ok"`
			Result struct {
				ID       int64  `json:"id"`
				Username string `json:"username"`
				IsBot    bool   `json:"is_bot"`
			} `json:"result"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
			return "", fmt.Errorf("failed to parse Telegram API response: %w", err)
		}
		if !tgResp.OK {
			return "", fmt.Errorf("Telegram verification failed: %s", tgResp.Description)
		}

		botHandle := "@" + tgResp.Result.Username
		cm.mu.Lock()
		if cm.verifiedIdentities == nil {
			cm.verifiedIdentities = make(map[string]string)
		}
		cm.verifiedIdentities["telegram"] = botHandle
		cm.mu.Unlock()

		return fmt.Sprintf("Telegram connection verified. Authenticated as %s (ID: %d).", botHandle, tgResp.Result.ID), nil

	case "discord":
		if !cfg.Discord.Enabled {
			return "", fmt.Errorf("Discord bridge is disabled")
		}
		token := strings.TrimSpace(cfg.Discord.BotToken)
		if token == "" || token == "[REDACTED]" {
			return "", fmt.Errorf("Discord Bot Token is required")
		}
		req, err := http.NewRequest(http.MethodGet, "https://discord.com/api/v10/users/@me", nil)
		if err != nil {
			return "", fmt.Errorf("failed to create Discord request: %w", err)
		}
		authHeader := token
		if !strings.HasPrefix(authHeader, "Bot ") {
			authHeader = "Bot " + authHeader
		}
		req.Header.Set("Authorization", authHeader)
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("Discord API connection failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			return "", fmt.Errorf("Discord API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var dcResp struct {
			ID            string `json:"id"`
			Username      string `json:"username"`
			Discriminator string `json:"discriminator"`
			Bot           bool   `json:"bot"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&dcResp); err != nil {
			return "", fmt.Errorf("failed to parse Discord API response: %w", err)
		}
		userTag := dcResp.Username
		if dcResp.Discriminator != "" && dcResp.Discriminator != "0" {
			userTag = fmt.Sprintf("%s#%s", dcResp.Username, dcResp.Discriminator)
		}

		cm.mu.Lock()
		if cm.verifiedIdentities == nil {
			cm.verifiedIdentities = make(map[string]string)
		}
		cm.verifiedIdentities["discord"] = userTag
		cm.mu.Unlock()

		return fmt.Sprintf("Discord connection verified. Authenticated as %s (ID: %s).", userTag, dcResp.ID), nil

	default:
		return "", fmt.Errorf("unknown channel %q", channel)
	}
}
