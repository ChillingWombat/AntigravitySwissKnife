package bridge

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func init() {
	if core.IsRunningTests() {
		cfgPath := filepath.Join(os.TempDir(), "antigravity-swiss-test-config", "config.json")
		if cfg, err := core.LoadConfig(); err == nil && cfg != nil {
			cfg.AppPasswordEnabled = false
			cfg.AppPasswordHash = ""
			_ = cfg.Save()
		} else {
			_ = os.Remove(cfgPath)
		}
	}
}

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

// secureCompare compares two secrets in constant time.
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
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
	return expectedPIN != "" && secureCompare(candidate, expectedPIN)
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

// UpdateCredentials replaces the live token, pairing PIN, and App Access Password mode.
func (am *AuthManager) UpdateCredentials(token, pin string, useAppPassword bool) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.token = token
	am.pin = pin
	am.useAppPassword = useAppPassword
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
// Proxy headers (Cf-Connecting-Ip, rightmost X-Forwarded-For entry, X-Real-IP) are only consulted
// when the direct peer (RemoteAddr) is a loopback IP (traffic from local tunnel process).
func getClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && h != "" {
		host = h
	}
	host = strings.Trim(host, "[]")

	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		if cf := strings.TrimSpace(r.Header.Get("Cf-Connecting-Ip")); cf != "" {
			return cf
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			for i := len(parts) - 1; i >= 0; i-- {
				p := strings.TrimSpace(parts[i])
				if p != "" {
					return p
				}
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			return xri
		}
	}
	return host
}

// isLockedLocked checks if clientIP is currently locked out, cleaning expired lockouts.
func (am *AuthManager) isLockedLocked(clientIP string) bool {
	if until, locked := am.lockedUntil[clientIP]; locked {
		if time.Now().Before(until) {
			return true
		}
		delete(am.lockedUntil, clientIP)
		delete(am.failedAttempts, clientIP)
	}
	return false
}

// recordFailureLocked records a failed attempt and evaluates brute force lockout (5 failed attempts within 5 minutes).
func (am *AuthManager) recordFailureLocked(clientIP string) bool {
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
		return true
	}
	return false
}

// recordSuccessLocked clears failed attempt counters on successful verification.
func (am *AuthManager) recordSuccessLocked(clientIP string) {
	delete(am.failedAttempts, clientIP)
}

// IsIPLocked checks if a client IP is currently locked out due to brute-force attempts.
func (am *AuthManager) IsIPLocked(ip string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	until, exists := am.lockedUntil[ip]
	return exists && time.Now().Before(until)
}

// VerifyPIN validates a candidate PIN or token against expected credentials and enforces brute-force lockout.
// When candidate is empty, it returns (false, isLocked) without recording a failure.
func (am *AuthManager) VerifyPIN(r *http.Request, candidate string) (ok bool, locked bool) {
	if am == nil {
		return false, false
	}
	clientIP := getClientIP(r)

	am.mu.Lock()
	defer am.mu.Unlock()

	if am.isLockedLocked(clientIP) {
		return false, true
	}

	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false, false
	}

	valid := VerifyCandidatePIN(candidate, am.pin, am.useAppPassword) || (am.token != "" && secureCompare(candidate, am.token))
	if valid {
		am.recordSuccessLocked(clientIP)
		return true, false
	}

	newlyLocked := am.recordFailureLocked(clientIP)
	return false, newlyLocked
}

// ValidateRequest checks HTTP request headers and query parameters for valid credentials.
// It enforces brute-force protection: 5 failed PIN attempts within 5 minutes locks the client IP for 15 minutes.
func (am *AuthManager) ValidateRequest(r *http.Request) (bool, string) {
	clientIP := getClientIP(r)

	am.mu.Lock()
	defer am.mu.Unlock()

	// Check active lockout
	if am.isLockedLocked(clientIP) {
		return false, "IP temporarily locked due to excessive failed attempts; please try again later"
	}

	expectedToken := am.token
	expectedPIN := am.pin
	useAppPass := am.useAppPassword

	// 1. Authorization header (Bearer <token>)
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			bearerToken := strings.TrimSpace(parts[1])
			if secureCompare(bearerToken, expectedToken) {
				am.recordSuccessLocked(clientIP)
				return true, "Bearer token verified"
			}
			if VerifyCandidatePIN(bearerToken, expectedPIN, useAppPass) {
				am.recordSuccessLocked(clientIP)
				return true, "Pairing PIN verified via Bearer header"
			}
		}
	}

	// 2. Custom headers
	if xToken := r.Header.Get("X-Bridge-Token"); xToken != "" && secureCompare(xToken, expectedToken) {
		am.recordSuccessLocked(clientIP)
		return true, "X-Bridge-Token verified"
	}
	if xPin := r.Header.Get("X-Pairing-PIN"); xPin != "" && VerifyCandidatePIN(xPin, expectedPIN, useAppPass) {
		am.recordSuccessLocked(clientIP)
		return true, "X-Pairing-PIN verified"
	}

	// 3. Query parameters (for SSE / Streamable HTTP GET where headers may be restricted)
	q := r.URL.Query()
	if qToken := q.Get("token"); qToken != "" && secureCompare(qToken, expectedToken) {
		am.recordSuccessLocked(clientIP)
		return true, "Query token verified"
	}
	if qAuth := q.Get("auth"); qAuth != "" && secureCompare(qAuth, expectedToken) {
		am.recordSuccessLocked(clientIP)
		return true, "Query auth verified"
	}
	if qPin := q.Get("pin"); qPin != "" && VerifyCandidatePIN(qPin, expectedPIN, useAppPass) {
		am.recordSuccessLocked(clientIP)
		return true, "Query pairing PIN verified"
	}

	// Record failed attempt and evaluate brute force lockout (5 failed attempts within 5 minutes)
	if am.recordFailureLocked(clientIP) {
		return false, "IP locked for 15 minutes due to excessive failed attempts"
	}

	return false, "Missing or invalid bridge credentials"
}
