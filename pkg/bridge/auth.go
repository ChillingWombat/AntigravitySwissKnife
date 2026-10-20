package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// AuthAttempt records an authentication verification event for auditing.
type AuthAttempt struct {
	Timestamp string `json:"timestamp"`
	ClientIP  string `json:"client_ip"`
	Method    string `json:"method"`
	Granted   bool   `json:"granted"`
	Reason    string `json:"reason"`
}

// AuthManager manages authentication credentials, pairing PINs, and request validation.
type AuthManager struct {
	mu             sync.RWMutex
	token          string
	pin            string
	useAppPassword bool
	attempts       []AuthAttempt
	onSaveCreds    func(token, pin string) error
	failedAttempts map[string][]time.Time
	lockedUntil    map[string]time.Time
}

// GenerateSecureToken produces a 32-hex-character crypto random string.
func GenerateSecureToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("swiss-gemini-%s", hex.EncodeToString(b))
}

const pinCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const pinDigits = "0123456789"
const pinLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GeneratePairingPIN produces an 8-character alphanumeric security PIN containing letters and numbers.
func GeneratePairingPIN() string {
	const length = 8
	buf := make([]byte, length)
	hasDigit := false
	hasLetter := false
	charsetLen := big.NewInt(int64(len(pinCharset)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "A7k9M2xP"
		}
		ch := pinCharset[n.Int64()]
		buf[i] = ch
		if ch >= '0' && ch <= '9' {
			hasDigit = true
		} else {
			hasLetter = true
		}
	}
	if !hasDigit {
		if n, err := rand.Int(rand.Reader, big.NewInt(int64(len(pinDigits)))); err == nil {
			buf[0] = pinDigits[n.Int64()]
		} else {
			buf[0] = '7'
		}
	}
	if !hasLetter {
		if n, err := rand.Int(rand.Reader, big.NewInt(int64(len(pinLetters)))); err == nil {
			buf[length-1] = pinLetters[n.Int64()]
		} else {
			buf[length-1] = 'K'
		}
	}
	return string(buf)
}

// IsAlphanumericPIN8 reports whether pin is an 8-character ASCII alphanumeric string.
func IsAlphanumericPIN8(pin string) bool {
	if len(pin) != 8 {
		return false
	}
	for i := 0; i < len(pin); i++ {
		c := pin[i]
		isDigit := c >= '0' && c <= '9'
		isUpper := c >= 'A' && c <= 'Z'
		isLower := c >= 'a' && c <= 'z'
		if !isDigit && !isUpper && !isLower {
			return false
		}
	}
	return true
}

// IsAppPasswordConfigured checks whether an application access password is currently enabled in General Settings.
func IsAppPasswordConfigured() bool {
	cfg, err := core.LoadConfig()
	return err == nil && cfg != nil && cfg.AppPasswordEnabled && cfg.AppPasswordHash != ""
}

// VerifyCandidatePIN validates a candidate PIN against either the App Access Password (when enabled and set)
// or the 8-character simple security PIN.
func VerifyCandidatePIN(candidate, expectedPIN string, useAppPassword bool) bool {
	candidate = strings.TrimSpace(candidate)
	expectedPIN = strings.TrimSpace(expectedPIN)
	if candidate == "" {
		return false
	}
	if useAppPassword {
		if cfg, err := core.LoadConfig(); err == nil && cfg != nil && cfg.AppPasswordEnabled && cfg.AppPasswordHash != "" {
			return cfg.VerifyAppPassword(candidate)
		}
	}
	return expectedPIN != "" && candidate == expectedPIN
}

// NewAuthManager initializes an authentication manager.
func NewAuthManager(token string, pin string, onSave func(token, pin string) error) *AuthManager {
	if token == "" {
		token = GenerateSecureToken()
	}
	if pin == "" {
		pin = GeneratePairingPIN()
	}
	return &AuthManager{
		token:          token,
		pin:            pin,
		attempts:       make([]AuthAttempt, 0),
		onSaveCreds:    onSave,
		failedAttempts: make(map[string][]time.Time),
		lockedUntil:    make(map[string]time.Time),
	}
}

// GetToken returns the current Bearer token.
func (am *AuthManager) GetToken() string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.token
}

// GetPIN returns the current pairing PIN.
func (am *AuthManager) GetPIN() string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.pin
}

// SetUseAppPassword configures whether PIN verification uses the App Access Password when set.
func (am *AuthManager) SetUseAppPassword(use bool) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.useAppPassword = use
}

// GetUseAppPassword returns whether App Access Password mode is enabled on the AuthManager.
func (am *AuthManager) GetUseAppPassword() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.useAppPassword && IsAppPasswordConfigured()
}

// RotateCredentials generates a fresh token and pairing PIN and saves them.
func (am *AuthManager) RotateCredentials() (string, string, error) {
	am.mu.Lock()
	newToken := GenerateSecureToken()
	newPIN := GeneratePairingPIN()
	am.token = newToken
	am.pin = newPIN
	onSave := am.onSaveCreds
	am.mu.Unlock()

	var err error
	if onSave != nil {
		err = onSave(newToken, newPIN)
	}
	return newToken, newPIN, err
}

// RecordAttempt records an access attempt in the audit log.
func (am *AuthManager) RecordAttempt(ip, method string, granted bool, reason string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	att := AuthAttempt{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		ClientIP:  ip,
		Method:    method,
		Granted:   granted,
		Reason:    reason,
	}
	am.attempts = append([]AuthAttempt{att}, am.attempts...)
	if len(am.attempts) > 50 {
		am.attempts = am.attempts[:50]
	}
}

// GetRecentAttempts returns a copy of recent authentication records.
func (am *AuthManager) GetRecentAttempts() []AuthAttempt {
	am.mu.RLock()
	defer am.mu.RUnlock()
	res := make([]AuthAttempt, len(am.attempts))
	copy(res, am.attempts)
	return res
}

// getClientIP extracts the remote client IP address from proxy headers or RemoteAddr.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// IsIPLocked checks if a client IP is currently locked out due to brute-force attempts.
func (am *AuthManager) IsIPLocked(ip string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	until, exists := am.lockedUntil[ip]
	return exists && time.Now().Before(until)
}

// ValidateRequest checks HTTP request headers and query parameters for valid credentials.
// It enforces brute-force protection: 5 failed PIN attempts within 5 minutes locks the client IP for 15 minutes.
func (am *AuthManager) ValidateRequest(r *http.Request) (bool, string) {
	clientIP := getClientIP(r)

	am.mu.Lock()
	defer am.mu.Unlock()

	// Check active lockout
	if until, locked := am.lockedUntil[clientIP]; locked {
		if time.Now().Before(until) {
			return false, "IP temporarily locked due to excessive failed attempts; please try again later"
		}
		delete(am.lockedUntil, clientIP)
		delete(am.failedAttempts, clientIP)
	}

	expectedToken := am.token
	expectedPIN := am.pin
	useAppPass := am.useAppPassword

	// 1. Authorization header (Bearer <token>)
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			bearerToken := strings.TrimSpace(parts[1])
			if bearerToken == expectedToken {
				delete(am.failedAttempts, clientIP)
				return true, "Bearer token verified"
			}
			if VerifyCandidatePIN(bearerToken, expectedPIN, useAppPass) {
				delete(am.failedAttempts, clientIP)
				return true, "Pairing PIN verified via Bearer header"
			}
		}
	}

	// 2. Custom headers
	if xToken := r.Header.Get("X-Bridge-Token"); xToken != "" && xToken == expectedToken {
		delete(am.failedAttempts, clientIP)
		return true, "X-Bridge-Token verified"
	}
	if xPin := r.Header.Get("X-Pairing-PIN"); xPin != "" && VerifyCandidatePIN(xPin, expectedPIN, useAppPass) {
		delete(am.failedAttempts, clientIP)
		return true, "X-Pairing-PIN verified"
	}

	// 3. Query parameters (for SSE / Streamable HTTP GET where headers may be restricted)
	q := r.URL.Query()
	if qToken := q.Get("token"); qToken != "" && qToken == expectedToken {
		delete(am.failedAttempts, clientIP)
		return true, "Query token verified"
	}
	if qAuth := q.Get("auth"); qAuth != "" && qAuth == expectedToken {
		delete(am.failedAttempts, clientIP)
		return true, "Query auth verified"
	}
	if qPin := q.Get("pin"); qPin != "" && VerifyCandidatePIN(qPin, expectedPIN, useAppPass) {
		delete(am.failedAttempts, clientIP)
		return true, "Query pairing PIN verified"
	}

	// Record failed attempt and evaluate brute force lockout (5 failed attempts within 5 minutes)
	now := time.Now()
	cutoff := now.Add(-5 * time.Minute)
	var recentFails []time.Time
	for _, t := range am.failedAttempts[clientIP] {
		if t.After(cutoff) {
			recentFails = append(recentFails, t)
		}
	}
	recentFails = append(recentFails, now)
	am.failedAttempts[clientIP] = recentFails

	if len(recentFails) >= 5 {
		am.lockedUntil[clientIP] = now.Add(15 * time.Minute)
		return false, "IP locked for 15 minutes due to excessive failed attempts"
	}

	return false, "Missing or invalid bridge credentials"
}
