package bridge

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestChallenger_BruteForceLockout_6thAttemptBlocked empirically verifies that sending
// 5 consecutive incorrect pairing PINs locks the attacker's IP address, and that the
// 6th attempt (even when presenting valid credentials) is strictly blocked by the rate-limiting lockout.
// It also verifies that an innocent, separate IP address is unaffected and can pair successfully.
func TestChallenger_BruteForceLockout_6thAttemptBlocked(t *testing.T) {
	validToken := "swiss-gemini-test-token-abcdef123456"
	validPIN := "829471"
	attackerIP := "198.51.100.42:54321"
	attackerHost := "198.51.100.42"
	legitimateIP := "203.0.113.88:54321"

	am := NewAuthManager(validToken, validPIN, nil)

	// Attempts 1 through 4: Should fail with "Missing or invalid bridge credentials"
	// and the IP must NOT be marked as locked yet.
	for attempt := 1; attempt <= 4; attempt++ {
		wrongPIN := fmt.Sprintf("%06d", 100000+attempt)
		req := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", nil)
		req.RemoteAddr = attackerIP
		req.Header.Set("X-Pairing-PIN", wrongPIN)

		granted, reason := am.ValidateRequest(req)
		if granted {
			t.Fatalf("attempt %d: expected authentication to be denied, but was granted", attempt)
		}
		if am.IsIPLocked(attackerHost) {
			t.Fatalf("attempt %d: IP was locked prematurely before reaching 5 failed attempts", attempt)
		}
		if reason != "Missing or invalid bridge credentials" {
			t.Errorf("attempt %d: expected reason 'Missing or invalid bridge credentials', got '%s'", attempt, reason)
		}
	}

	// Attempt 5: 5th consecutive failure.
	// This attempt MUST trigger the 15-minute brute-force lockout.
	req5 := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=999999", nil)
	req5.RemoteAddr = attackerIP
	granted5, reason5 := am.ValidateRequest(req5)
	if granted5 {
		t.Fatalf("attempt 5: expected authentication to be denied, but was granted")
	}
	if !am.IsIPLocked(attackerHost) {
		t.Fatalf("attempt 5: expected attacker IP %s to be locked, but IsIPLocked returned false", attackerHost)
	}
	expectedReason5 := "IP locked for 15 minutes due to excessive failed attempts"
	if reason5 != expectedReason5 {
		t.Errorf("attempt 5: expected reason '%s', got '%s'", expectedReason5, reason5)
	}

	// Attempt 6: Send the CORRECT, VALID PIN.
	// The lockout MUST block this attempt before validating credentials!
	req6 := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", nil)
	req6.RemoteAddr = attackerIP
	req6.Header.Set("X-Pairing-PIN", validPIN)
	granted6, reason6 := am.ValidateRequest(req6)
	if granted6 {
		t.Fatalf("attempt 6: expected locked attacker to be blocked even with the correct PIN, but was granted")
	}
	expectedReason6 := "IP temporarily locked due to excessive failed attempts; please try again later"
	if reason6 != expectedReason6 {
		t.Errorf("attempt 6: expected reason '%s', got '%s'", expectedReason6, reason6)
	}

	// Attempt 7: Send the CORRECT, VALID Bearer token.
	// The lockout MUST also block valid bearer token requests from the locked IP!
	req7 := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", nil)
	req7.RemoteAddr = attackerIP
	req7.Header.Set("Authorization", "Bearer "+validToken)
	granted7, reason7 := am.ValidateRequest(req7)
	if granted7 {
		t.Fatalf("attempt 7: expected locked attacker to be blocked even with valid Bearer token, but was granted")
	}
	if reason7 != expectedReason6 {
		t.Errorf("attempt 7: expected reason '%s', got '%s'", expectedReason6, reason7)
	}

	// Independent legitimate client from a different IP address:
	// Must NOT be affected by the attacker's lockout and should succeed immediately.
	reqLegit := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", nil)
	reqLegit.RemoteAddr = legitimateIP
	reqLegit.Header.Set("X-Pairing-PIN", validPIN)
	grantedLegit, reasonLegit := am.ValidateRequest(reqLegit)
	if !grantedLegit {
		t.Fatalf("legitimate IP: expected authentication granted with valid PIN, got denied: %s", reasonLegit)
	}
}

// TestChallenger_BruteForceLockout_SuccessResetsCounter verifies that a successful
// authentication clears previous failed attempt records for an IP, preventing false
// lockouts for legitimate users who made a few typos before entering the correct PIN.
func TestChallenger_BruteForceLockout_SuccessResetsCounter(t *testing.T) {
	validToken := "swiss-gemini-token-reset-counter"
	validPIN := "123456"
	clientIP := "10.0.0.50:54321"
	clientHost := "10.0.0.50"

	am := NewAuthManager(validToken, validPIN, nil)

	// Send 4 wrong attempts
	for i := 1; i <= 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/bridge/mcp?pin=000000", nil)
		req.RemoteAddr = clientIP
		granted, _ := am.ValidateRequest(req)
		if granted {
			t.Fatalf("attempt %d: expected rejected", i)
		}
	}

	if am.IsIPLocked(clientHost) {
		t.Fatalf("expected IP not locked after 4 failed attempts")
	}

	// Send 1 successful attempt with correct PIN
	reqSuccess := httptest.NewRequest(http.MethodGet, "/api/bridge/mcp?pin=123456", nil)
	reqSuccess.RemoteAddr = clientIP
	grantedSuccess, _ := am.ValidateRequest(reqSuccess)
	if !grantedSuccess {
		t.Fatalf("expected correct PIN to succeed")
	}

	// Send 4 more wrong attempts. Since counter was reset, this should NOT trigger lockout!
	for i := 1; i <= 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/bridge/mcp?pin=000000", nil)
		req.RemoteAddr = clientIP
		granted, _ := am.ValidateRequest(req)
		if granted {
			t.Fatalf("post-reset attempt %d: expected rejected", i)
		}
	}

	if am.IsIPLocked(clientHost) {
		t.Fatalf("expected IP not locked after reset + 4 failed attempts; counter was not cleared on success")
	}

	// 5th attempt after reset: NOW it should lock
	reqLock := httptest.NewRequest(http.MethodGet, "/api/bridge/mcp?pin=000000", nil)
	reqLock.RemoteAddr = clientIP
	grantedLock, _ := am.ValidateRequest(reqLock)
	if grantedLock {
		t.Fatalf("expected rejected on 5th failure post-reset")
	}
	if !am.IsIPLocked(clientHost) {
		t.Fatalf("expected IP to be locked on 5th consecutive failure post-reset")
	}
}

// TestChallenger_BruteForceLockout_ProxyHeaderExtraction tests that client IP is properly
// extracted from reverse proxy headers (X-Forwarded-For, X-Real-IP) and locked out accurately.
func TestChallenger_BruteForceLockout_ProxyHeaderExtraction(t *testing.T) {
	am := NewAuthManager("test-token", "654321", nil)

	// Client behind proxy with X-Forwarded-For: "10.0.0.1, 203.0.113.195" (rightmost entry is real client)
	clientHost := "203.0.113.195"
	proxyAddr := "127.0.0.1:8080"

	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=wrong", nil)
		req.RemoteAddr = proxyAddr
		req.Header.Set("X-Forwarded-For", "10.0.0.1, 203.0.113.195")
		am.ValidateRequest(req)
	}

	// Attacker IP (203.0.113.195) should be locked, NOT the proxy (127.0.0.1)
	if !am.IsIPLocked(clientHost) {
		t.Errorf("expected client IP %s from X-Forwarded-For to be locked", clientHost)
	}
	if am.IsIPLocked("127.0.0.1") {
		t.Errorf("proxy IP 127.0.0.1 was incorrectly locked instead of client IP")
	}
}
