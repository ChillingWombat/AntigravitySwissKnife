package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
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
	mu          sync.RWMutex
	token       string
	pin         string
	attempts    []AuthAttempt
	onSaveCreds func(token, pin string) error
}

// GenerateSecureToken produces a 32-hex-character crypto random string.
func GenerateSecureToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("swiss-gemini-%s", hex.EncodeToString(b))
}

// GeneratePairingPIN produces a 6-digit numeric pairing PIN.
func GeneratePairingPIN() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "582914"
	}
	return fmt.Sprintf("%06d", n.Int64()+100000)
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
		token:       token,
		pin:         pin,
		attempts:    make([]AuthAttempt, 0),
		onSaveCreds: onSave,
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

// ValidateRequest checks HTTP request headers and query parameters for valid credentials.
func (am *AuthManager) ValidateRequest(r *http.Request) (bool, string) {
	am.mu.RLock()
	expectedToken := am.token
	expectedPIN := am.pin
	am.mu.RUnlock()

	// 1. Authorization header (Bearer <token>)
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			bearerToken := strings.TrimSpace(parts[1])
			if bearerToken == expectedToken {
				return true, "Bearer token verified"
			}
			if bearerToken == expectedPIN {
				return true, "Pairing PIN verified via Bearer header"
			}
		}
	}

	// 2. Custom headers
	if xToken := r.Header.Get("X-Bridge-Token"); xToken != "" && xToken == expectedToken {
		return true, "X-Bridge-Token verified"
	}
	if xPin := r.Header.Get("X-Pairing-PIN"); xPin != "" && xPin == expectedPIN {
		return true, "X-Pairing-PIN verified"
	}

	// 3. Query parameters (for SSE / Streamable HTTP GET where headers may be restricted)
	q := r.URL.Query()
	if qToken := q.Get("token"); qToken != "" && qToken == expectedToken {
		return true, "Query token verified"
	}
	if qAuth := q.Get("auth"); qAuth != "" && qAuth == expectedToken {
		return true, "Query auth verified"
	}
	if qPin := q.Get("pin"); qPin != "" && qPin == expectedPIN {
		return true, "Query pairing PIN verified"
	}

	return false, "Missing or invalid bridge credentials"
}
