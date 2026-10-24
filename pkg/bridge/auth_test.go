package bridge

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
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

func TestGeneratePairingPIN_Is8CharAlphanumeric(t *testing.T) {
	for i := 0; i < 32; i++ {
		pin := GeneratePairingPIN()
		if len(pin) != 8 {
			t.Fatalf("expected 8-character PIN, got %q (len=%d)", pin, len(pin))
		}
		if !IsAlphanumericPIN8(pin) {
			t.Fatalf("expected alphanumeric 8-char PIN, got %q", pin)
		}
		hasDigit := false
		hasLetter := false
		for j := 0; j < len(pin); j++ {
			c := pin[j]
			if c >= '0' && c <= '9' {
				hasDigit = true
			} else {
				hasLetter = true
			}
		}
		if !hasDigit || !hasLetter {
			t.Fatalf("expected PIN %q to contain both letters and numbers", pin)
		}
	}
}

func TestVerifyCandidatePIN_SimplePINAndAppPasswordMode(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, ".config"))

	simplePIN := "K7m9P2xQ"

	// 1. Default simple PIN mode (useAppPassword = false)
	if !VerifyCandidatePIN(simplePIN, simplePIN, false) {
		t.Fatalf("expected simple 8-char PIN to verify when useAppPassword=false")
	}
	if VerifyCandidatePIN("wrongPIN", simplePIN, false) {
		t.Fatalf("expected wrong PIN to fail")
	}

	// 2. When useAppPassword = true but no App Password is set in core.Config,
	// it falls back to the simple PIN.
	if !VerifyCandidatePIN(simplePIN, simplePIN, true) {
		t.Fatalf("expected fallback to simple PIN when App Password is not configured")
	}

	// 3. Configure an App Access Password in core.Config
	cfg := core.DefaultConfig()
	if err := cfg.SetAppPassword("MasterSecret99!"); err != nil {
		t.Fatalf("SetAppPassword failed: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("cfg.Save failed: %v", err)
	}

	if !IsAppPasswordConfigured() {
		t.Fatalf("expected IsAppPasswordConfigured() to be true")
	}

	// With useAppPassword = true and App Password configured, only the App Password verifies
	if !VerifyCandidatePIN("MasterSecret99!", simplePIN, true) {
		t.Fatalf("expected App Access Password to verify when useAppPassword=true")
	}
	if VerifyCandidatePIN(simplePIN, simplePIN, true) {
		t.Fatalf("expected simple PIN to be rejected when useAppPassword=true and App Password is set")
	}

	// With useAppPassword = false, the simple PIN still verifies
	if !VerifyCandidatePIN(simplePIN, simplePIN, false) {
		t.Fatalf("expected simple PIN to verify when useAppPassword=false even if App Password is set")
	}
}

func TestGetClientIP_LoopbackAndSpoofedXFF(t *testing.T) {
	// 1. Non-loopback peer with spoofed XFF: must ignore XFF and return RemoteAddr host
	reqNonLoop := httptest.NewRequest(http.MethodGet, "/", nil)
	reqNonLoop.RemoteAddr = "198.51.100.2:12345"
	reqNonLoop.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	reqNonLoop.Header.Set("Cf-Connecting-Ip", "10.0.0.3")
	reqNonLoop.Header.Set("X-Real-IP", "10.0.0.4")
	if ip := getClientIP(reqNonLoop); ip != "198.51.100.2" {
		t.Fatalf("expected non-loopback peer to return RemoteAddr host %q, got %q", "198.51.100.2", ip)
	}

	// 2. Loopback peer with "spoofed, real" XFF: must yield rightmost "real" entry
	reqLoopXFF := httptest.NewRequest(http.MethodGet, "/", nil)
	reqLoopXFF.RemoteAddr = "127.0.0.1:45678"
	reqLoopXFF.Header.Set("X-Forwarded-For", "spoofed, real")
	if ip := getClientIP(reqLoopXFF); ip != "real" {
		t.Fatalf("expected loopback peer with 'spoofed, real' XFF to yield 'real', got %q", ip)
	}

	// 3. Loopback peer with trailing comma / spaces in XFF
	reqLoopTrailing := httptest.NewRequest(http.MethodGet, "/", nil)
	reqLoopTrailing.RemoteAddr = "127.0.0.1:45678"
	reqLoopTrailing.Header.Set("X-Forwarded-For", "attacker-spoofed, 203.0.113.50, ")
	if ip := getClientIP(reqLoopTrailing); ip != "203.0.113.50" {
		t.Fatalf("expected rightmost non-empty XFF '203.0.113.50', got %q", ip)
	}

	// 4. Loopback peer with Cf-Connecting-Ip: takes precedence over XFF and X-Real-IP
	reqLoopCF := httptest.NewRequest(http.MethodGet, "/", nil)
	reqLoopCF.RemoteAddr = "127.0.0.1:45678"
	reqLoopCF.Header.Set("Cf-Connecting-Ip", "198.51.100.88")
	reqLoopCF.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	reqLoopCF.Header.Set("X-Real-IP", "10.0.0.3")
	if ip := getClientIP(reqLoopCF); ip != "198.51.100.88" {
		t.Fatalf("expected Cf-Connecting-Ip precedence '198.51.100.88', got %q", ip)
	}

	// 5. Loopback peer with only X-Real-IP: yields X-Real-IP
	reqLoopXRI := httptest.NewRequest(http.MethodGet, "/", nil)
	reqLoopXRI.RemoteAddr = "127.0.0.1:45678"
	reqLoopXRI.Header.Set("X-Real-IP", "198.51.100.99")
	if ip := getClientIP(reqLoopXRI); ip != "198.51.100.99" {
		t.Fatalf("expected X-Real-IP '198.51.100.99', got %q", ip)
	}
}
