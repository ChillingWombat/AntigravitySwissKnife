package bridge

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestAuthManager_ValidateRequestAcceptsHeaderAndQueryCredentials(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		setup func(*http.Request)
	}{
		{
			name: "X-Bridge-Token with empty PIN header and invalid authorization",
			url:  "/api/bridge/mcp",
			setup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer unrelated-token")
				req.Header.Set("X-Bridge-PIN", "")
				req.Header.Set("X-Bridge-Token", "test-token-123")
			},
		},
		{
			name: "auth query with invalid authorization",
			url:  "/api/bridge/mcp?auth=test-token-123",
			setup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer unrelated-token")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			am := NewAuthManager("test-token-123", "654321", nil)
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			tc.setup(req)
			if granted, reason := am.ValidateRequest(req); !granted {
				t.Fatalf("expected request to be granted, got reason %q", reason)
			}
		})
	}
}

func TestAuthManager_LockoutTrackingIsCapped(t *testing.T) {
	am := NewAuthManager("test-token-123", "654321", nil)
	start := time.Now()

	am.mu.Lock()
	for i := 0; i < maxTrackedLockoutIPs; i++ {
		ip := fmt.Sprintf("198.51.%d.%d", i/256, i%256)
		am.failedAttempts[ip] = []time.Time{start.Add(time.Duration(i) * time.Microsecond)}
	}
	am.recordFailureLocked("203.0.113.200")
	_, oldestTracked := am.failedAttempts["198.51.0.0"]
	_, newestTracked := am.failedAttempts["203.0.113.200"]
	tracked := len(am.failedAttempts)
	am.mu.Unlock()

	if tracked != maxTrackedLockoutIPs {
		t.Fatalf("expected failure tracking to cap at %d IPs, got %d", maxTrackedLockoutIPs, tracked)
	}
	if oldestTracked {
		t.Fatal("expected oldest failed IP to be evicted")
	}
	if !newestTracked {
		t.Fatal("expected newly failed IP to remain tracked")
	}
}

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
	reqLoopXFF.Header.Set("X-Forwarded-For", "not-an-ip, also-invalid")
	if ip := getClientIP(reqLoopXFF); ip != "127.0.0.1" {
		t.Fatalf("expected malformed XFF to fall back to canonical remote IP, got %q", ip)
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

func TestVerifyCandidatePIN_CorruptConfigFailsClosed(t *testing.T) {
	configDir := filepath.Join(os.TempDir(), "antigravity-swiss-test-config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	original, readErr := os.ReadFile(configPath)
	t.Cleanup(func() {
		if readErr == nil {
			_ = os.WriteFile(configPath, original, 0o600)
			return
		}
		_ = os.Remove(configPath)
	})
	if err := os.WriteFile(configPath, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt config: %v", err)
	}
	if _, err := core.LoadConfig(); err == nil {
		t.Fatal("expected corrupt config load to fail")
	}
	if VerifyCandidatePIN("K7m9P2xQ", "K7m9P2xQ", true) {
		t.Fatal("expected corrupt config to fail closed instead of accepting the pairing PIN")
	}
	authManager := NewAuthManager("test-token-123", "123456", nil)
	authManager.mu.Lock()
	authManager.useAppPassword = true
	authManager.mu.Unlock()

	reqWithPIN := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=123456", nil)
	reqWithPIN.RemoteAddr = "127.0.0.1:12345"
	reqWithPIN.Header.Set("X-Bridge-PIN", "123456")
	if granted, _ := authManager.VerifyPIN(reqWithPIN, "123456"); granted {
		t.Fatal("expected VerifyPIN to fail closed with corrupt config")
	}
	if granted, _ := authManager.ValidateRequest(reqWithPIN); granted {
		t.Fatal("expected ValidateRequest to fail closed with corrupt config")
	}

}

func TestAuthManager_SweepsExpiredLockoutEntries(t *testing.T) {
	am := NewAuthManager("test-token", "654321", nil)
	clientIP := "203.0.113.10"
	expired := time.Now().Add(-time.Hour)
	am.mu.Lock()
	am.lockedUntil[clientIP] = expired
	am.failedAttempts[clientIP] = []time.Time{expired}
	am.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=654321", nil)
	req.RemoteAddr = clientIP + ":12345"
	if ok, locked := am.VerifyPIN(req, "654321"); !ok || locked {
		t.Fatalf("expected valid PIN after stale entries are swept, got ok=%v locked=%v", ok, locked)
	}

	am.mu.RLock()
	_, hasLock := am.lockedUntil[clientIP]
	_, hasFailure := am.failedAttempts[clientIP]
	am.mu.RUnlock()
	if hasLock || hasFailure {
		t.Fatalf("expected expired lockout state to be swept, hasLock=%v hasFailure=%v", hasLock, hasFailure)
	}
}
