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
const maxTrackedLockoutIPs = 1024

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
		cfg, err := core.LoadConfig()
		if err != nil || cfg == nil {
			return false
		}
		if cfg.AppPasswordEnabled && cfg.AppPasswordHash != "" {
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
	remoteIP := canonicalIP(host)
	if remoteIP == "" || !net.ParseIP(remoteIP).IsLoopback() {
		return remoteIP
	}

	if cf := canonicalIP(r.Header.Get("Cf-Connecting-Ip")); cf != "" {
		return cf
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			if ip := canonicalIP(parts[i]); ip != "" {
				return ip
			}
		}
	}
	if xri := canonicalIP(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}
	return remoteIP
}

func canonicalIP(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
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

// sweepLockoutStateLocked removes expired state and caps attacker-controlled map growth.
func (am *AuthManager) sweepLockoutStateLocked(now time.Time) {
	cutoff := now.Add(-5 * time.Minute)
	for ip, until := range am.lockedUntil {
		if !now.Before(until) {
			delete(am.lockedUntil, ip)
			delete(am.failedAttempts, ip)
		}
	}
	for ip, attempts := range am.failedAttempts {
		recent := attempts[:0]
		for _, attemptedAt := range attempts {
			if attemptedAt.After(cutoff) {
				recent = append(recent, attemptedAt)
			}
		}
		if len(recent) == 0 {
			delete(am.failedAttempts, ip)
			continue
		}
		am.failedAttempts[ip] = recent
	}

	for len(am.failedAttempts) > maxTrackedLockoutIPs {
		oldestIP := ""
		var oldest time.Time
		for ip, attempts := range am.failedAttempts {
			lastAttempt := attempts[len(attempts)-1]
			if oldestIP == "" || lastAttempt.Before(oldest) {
				oldestIP = ip
				oldest = lastAttempt
			}
		}
		delete(am.failedAttempts, oldestIP)
	}
	for len(am.lockedUntil) > maxTrackedLockoutIPs {
		oldestIP := ""
		var oldest time.Time
		for ip, until := range am.lockedUntil {
			if oldestIP == "" || until.Before(oldest) {
				oldestIP = ip
				oldest = until
			}
		}
		delete(am.lockedUntil, oldestIP)
		delete(am.failedAttempts, oldestIP)
	}
}

// recordFailureLocked records a failed attempt and evaluates brute force lockout (5 failed attempts within 5 minutes).
func (am *AuthManager) recordFailureLocked(clientIP string) bool {
	now := time.Now()
	am.sweepLockoutStateLocked(now)
	cutoff := now.Add(-5 * time.Minute)
	var recentFails []time.Time
	for _, t := range am.failedAttempts[clientIP] {
		if t.After(cutoff) {
			recentFails = append(recentFails, t)
		}
	}
	recentFails = append(recentFails, now)
	am.failedAttempts[clientIP] = recentFails
	am.sweepLockoutStateLocked(now)

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
	return am.verifyCandidate(r, candidate, true)
}

// VerifyInteractivePIN validates an interactive PIN or App Access Password. Raw bearer tokens are not accepted.
func (am *AuthManager) VerifyInteractivePIN(r *http.Request, candidate string) (ok bool, locked bool) {
	return am.verifyCandidate(r, candidate, false)
}

func (am *AuthManager) verifyCandidate(r *http.Request, candidate string, allowToken bool) (ok bool, locked bool) {
	ok, locked, _ = am.verifyCandidateState(r, candidate, allowToken)
	return ok, locked
}

func (am *AuthManager) verifyCandidateState(r *http.Request, candidate string, allowToken bool) (ok bool, locked bool, alreadyLocked bool) {
	return am.verifyCandidatesState(r, []string{candidate}, allowToken)
}

func (am *AuthManager) verifyCandidatesState(r *http.Request, candidates []string, allowToken bool) (ok bool, locked bool, alreadyLocked bool) {
	if am == nil {
		return false, false, false
	}
	clientIP := getClientIP(r)
	uniqueCandidates := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		uniqueCandidates = append(uniqueCandidates, candidate)
	}

	am.mu.Lock()
	am.sweepLockoutStateLocked(time.Now())
	if am.isLockedLocked(clientIP) {
		am.mu.Unlock()
		return false, true, true
	}
	pin := am.pin
	token := am.token
	useAppPassword := am.useAppPassword
	am.mu.Unlock()

	if len(uniqueCandidates) == 0 {
		return false, false, false
	}

	valid := false
	for _, candidate := range uniqueCandidates {
		if (allowToken && secureCompare(candidate, token)) || secureCompare(candidate, pin) {
			valid = true
			break
		}
	}
	if !valid && useAppPassword {
		for _, candidate := range uniqueCandidates {
			if VerifyCandidatePIN(candidate, pin, useAppPassword) {
				valid = true
				break
			}
		}
	}

	am.mu.Lock()
	defer am.mu.Unlock()
	am.sweepLockoutStateLocked(time.Now())
	if am.isLockedLocked(clientIP) {
		return false, true, true
	}
	if valid {
		am.recordSuccessLocked(clientIP)
		return true, false, false
	}
	return false, am.recordFailureLocked(clientIP), false
}

// ValidateRequest checks HTTP request headers and query parameters for valid credentials.
// It enforces brute-force protection: 5 failed PIN attempts within 5 minutes locks the client IP for 15 minutes.
func (am *AuthManager) ValidateRequest(r *http.Request) (bool, string) {
	candidates := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	addCandidate := func(raw string) {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		fields := strings.Fields(auth)
		if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
			addCandidate(fields[1])
		} else if _, password, ok := r.BasicAuth(); ok {
			addCandidate(password)
		}
	}
	addCandidate(r.Header.Get("X-Bridge-Token"))
	addCandidate(r.Header.Get("X-Bridge-PIN"))
	addCandidate(r.Header.Get("X-Pairing-PIN"))
	_ = r.ParseForm()
	addCandidate(r.URL.Query().Get("token"))
	addCandidate(r.URL.Query().Get("auth"))
	addCandidate(r.FormValue("pin"))
	addCandidate(r.FormValue("password"))
	addCandidate(r.FormValue("token"))

	ok, locked, alreadyLocked := am.verifyCandidatesState(r, candidates, true)
	if ok {
		return true, ""
	}
	if alreadyLocked {
		return false, "IP temporarily locked due to excessive failed attempts; please try again later"
	}
	if locked {
		return false, "IP locked for 15 minutes due to excessive failed attempts"
	}
	return false, "Missing or invalid bridge credentials"
}
