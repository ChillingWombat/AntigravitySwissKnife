package bridge

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAuthManager_BruteForceLockout verifies that 5 failed attempts within 5 minutes
// triggers an automatic 15-minute IP lockout.
func TestAuthManager_BruteForceLockout(t *testing.T) {
	am := NewAuthManager("test-token-123", "654321", nil)
	attackerIP := "192.168.1.100:12345"
	legitimateIP := "192.168.1.200:12345"

	// First 4 failed attempts should be rejected, but NOT locked yet
	for i := 1; i <= 4; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=000000", nil)
		req.RemoteAddr = attackerIP
		granted, reason := am.ValidateRequest(req)
		if granted {
			t.Fatalf("attempt %d: expected rejected, got granted", i)
		}
		if am.IsIPLocked("192.168.1.100") {
			t.Fatalf("attempt %d: IP locked prematurely", i)
		}
		if reason != "Missing or invalid bridge credentials" {
			t.Errorf("attempt %d: unexpected reason: %s", i, reason)
		}
	}

	// 5th failed attempt triggers the lockout
	req5 := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=000000", nil)
	req5.RemoteAddr = attackerIP
	granted5, reason5 := am.ValidateRequest(req5)
	if granted5 {
		t.Fatalf("attempt 5: expected rejected")
	}
	if !am.IsIPLocked("192.168.1.100") {
		t.Fatalf("attempt 5: expected IP 192.168.1.100 to be locked")
	}
	if reason5 != "IP locked for 15 minutes due to excessive failed attempts" {
		t.Errorf("attempt 5: unexpected lockout reason: %s", reason5)
	}

	// 6th attempt even with CORRECT credentials should be blocked due to lockout
	req6 := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=654321", nil)
	req6.RemoteAddr = attackerIP
	granted6, _ := am.ValidateRequest(req6)
	if granted6 {
		t.Fatalf("expected locked IP to be rejected even with valid PIN")
	}

	// Legitimate client from different IP with correct PIN should succeed immediately
	reqLegit := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=654321", nil)
	reqLegit.RemoteAddr = legitimateIP
	grantedLegit, _ := am.ValidateRequest(reqLegit)
	if !grantedLegit {
		t.Fatalf("expected legitimate IP with valid PIN to be granted")
	}
}

// TestRedactedConfigs_MaskingVerification verifies that GetRemoteConfigRedacted and
// GetChannelsConfigRedacted mask PINs and external bot tokens.
func TestRedactedConfigs_MaskingVerification(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Remote Manager Redaction
	rm := NewRemoteManager(tmpDir)
	_ = rm.UpdateConfig(RemoteControlConfig{
		Enabled:     true,
		SecurityPIN: "987654",
	})

	redactedRemote := rm.GetRemoteConfigRedacted()
	if redactedRemote.SecurityPIN != "******" {
		t.Errorf("expected SecurityPIN '******', got %s", redactedRemote.SecurityPIN)
	}

	// Updating with '******' preserves the real PIN
	_ = rm.UpdateConfig(RemoteControlConfig{
		Enabled:     true,
		SecurityPIN: "******",
	})
	if rm.GetConfig().SecurityPIN != "987654" {
		t.Errorf("expected stored PIN 987654 preserved, got %s", rm.GetConfig().SecurityPIN)
	}

	// 2. Channels Manager Redaction
	cm := NewChannelsManager(tmpDir)
	_ = cm.UpdateConfig(ChannelsConfig{
		WhatsApp: WhatsAppConfig{
			AccessToken: "wa-live-access-token",
			VerifyToken: "wa-live-verify-token",
		},
		Telegram: TelegramConfig{
			BotToken: "tg-live-bot-token",
		},
		Discord: DiscordConfig{
			BotToken: "dc-live-bot-token",
		},
	})

	redactedChannels := cm.GetChannelsConfigRedacted()
	if redactedChannels.WhatsApp.AccessToken != "[REDACTED]" {
		t.Errorf("expected WhatsApp AccessToken [REDACTED], got %s", redactedChannels.WhatsApp.AccessToken)
	}
	if redactedChannels.WhatsApp.VerifyToken != "[REDACTED]" {
		t.Errorf("expected WhatsApp VerifyToken [REDACTED], got %s", redactedChannels.WhatsApp.VerifyToken)
	}
	if redactedChannels.Telegram.BotToken != "[REDACTED]" {
		t.Errorf("expected Telegram BotToken [REDACTED], got %s", redactedChannels.Telegram.BotToken)
	}
	if redactedChannels.Discord.BotToken != "[REDACTED]" {
		t.Errorf("expected Discord BotToken [REDACTED], got %s", redactedChannels.Discord.BotToken)
	}

	// Updating with [REDACTED] preserves existing secrets
	_ = cm.UpdateConfig(redactedChannels)
	raw := cm.GetConfig()
	if raw.Telegram.BotToken != "tg-live-bot-token" {
		t.Errorf("expected preserved Telegram BotToken, got %s", raw.Telegram.BotToken)
	}
	if raw.Discord.BotToken != "dc-live-bot-token" {
		t.Errorf("expected preserved Discord BotToken, got %s", raw.Discord.BotToken)
	}
}
