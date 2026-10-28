package webgui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

const (
	legacyAppPasswordAttemptLimit  = 5
	legacyAppPasswordAttemptWindow = time.Second
)

type appPasswordAttemptWindow struct {
	started  time.Time
	attempts int
}

var localSessionTokenCache struct {
	sync.RWMutex
	path    string
	token   string
	modTime time.Time
	size    int64
}

var verifyAppPasswordForAuth = func(cfg *core.Config, candidate string) bool {
	return cfg.VerifyAppPassword(candidate)
}

var loadConfigForAuth = core.LoadConfig

// GenerateLocalSessionToken returns the 64-character hex-encoded session token.
//
// The token is persisted in session.token (0600) and REUSED across restarts:
// injected IDE pages, the desktop renderer and the tray all hold a copy, and
// rotating on every boot invalidated all of them at once (401s on every
// surface until each one re-acquired the token). A new token is generated
// only when the file is missing or corrupt.
//
// Returns:
//   - string: the 64-character hex-encoded session token.
//   - error: any file creation or permission error encountered.
func GenerateLocalSessionToken() (string, error) {
	cfgDir := core.GetConfigDir()
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create configuration directory %q: %w", cfgDir, err)
	}
	tokenPath := filepath.Join(cfgDir, "session.token")

	if data, err := os.ReadFile(tokenPath); err == nil {
		if tok := strings.TrimSpace(string(data)); isValidSessionToken(tok) {
			return tok, nil
		}
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for session token: %w", err)
	}
	token := hex.EncodeToString(b)
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		return "", fmt.Errorf("failed to write session token file %q: %w", tokenPath, err)
	}
	return token, nil
}

// isSessionTokenShaped checks whether tok is exactly 64 lowercase hex characters.
func isSessionTokenShaped(tok string) bool {
	if len(tok) != 64 {
		return false
	}
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// isValidSessionToken accepts the 64-character hex tokens this generator produces.
func isValidSessionToken(tok string) bool {
	return isSessionTokenShaped(tok)
}

// GetLocalSessionToken reads the current session token, reusing the cached value
// while the token file's modification time and size are unchanged.
//
// Parameters: none.
// Returns:
//   - string: the stored session token string.
//   - error: if file cannot be read or is empty.
func GetLocalSessionToken() (string, error) {
	tokenPath := filepath.Join(core.GetConfigDir(), "session.token")
	info, err := os.Stat(tokenPath)
	if err != nil {
		localSessionTokenCache.Lock()
		if localSessionTokenCache.path == tokenPath {
			localSessionTokenCache.path = ""
			localSessionTokenCache.token = ""
			localSessionTokenCache.modTime = time.Time{}
			localSessionTokenCache.size = 0
		}
		localSessionTokenCache.Unlock()
		return "", err
	}

	localSessionTokenCache.RLock()
	if localSessionTokenCache.path == tokenPath &&
		localSessionTokenCache.token != "" &&
		localSessionTokenCache.modTime.Equal(info.ModTime()) &&
		localSessionTokenCache.size == info.Size() {
		token := localSessionTokenCache.token
		localSessionTokenCache.RUnlock()
		return token, nil
	}
	localSessionTokenCache.RUnlock()

	token, err := core.GetLocalSessionToken()
	if err != nil {
		return "", err
	}

	localSessionTokenCache.Lock()
	localSessionTokenCache.path = tokenPath
	localSessionTokenCache.token = token
	localSessionTokenCache.modTime = info.ModTime()
	localSessionTokenCache.size = info.Size()
	localSessionTokenCache.Unlock()
	return token, nil
}

func (s *Server) allowLegacyAppPasswordAttempt(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		host = r.RemoteAddr
	}
	if host == "" {
		host = "unknown"
	}

	now := time.Now()
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if s.legacyAppPasswordAttempts == nil {
		s.legacyAppPasswordAttempts = make(map[string]appPasswordAttemptWindow)
	}
	for client, attempt := range s.legacyAppPasswordAttempts {
		if now.Sub(attempt.started) >= legacyAppPasswordAttemptWindow {
			delete(s.legacyAppPasswordAttempts, client)
		}
	}
	attempt := s.legacyAppPasswordAttempts[host]
	if attempt.started.IsZero() {
		attempt.started = now
	}
	if attempt.attempts >= legacyAppPasswordAttemptLimit {
		return false
	}
	attempt.attempts++
	s.legacyAppPasswordAttempts[host] = attempt
	return true
}

// isPublicPath checks if a requested HTTP URL path is in the unauthenticated public whitelist.
// Public routes include UI static assets, health checks, authentication unlock endpoints,
// and independent MCP bridge endpoints that enforce their own authentication layer.
//
// Parameters:
//   - p: URL path from http.Request.
//
// Returns:
//   - bool: true if path is publicly accessible without a session token; false otherwise.
func isPublicPath(p string) bool {
	clean := path.Clean(p)
	// 1. Root and standard UI static assets
	if clean == "." || clean == "/" || clean == "/index.html" || clean == "/health" || clean == "/healthz" || clean == "/api/health" {
		return true
	}
	if isPublicIconPath(clean) {
		return true
	}
	// 2. Bundled frontend static assets (CSS, JS, images, logos)
	if strings.HasPrefix(clean, "/assets/") || clean == "/assets" {
		return true
	}
	// 3. Authentication status check and unlock endpoints
	if clean == "/api/auth/status" || clean == "/api/auth/unlock" {
		return true
	}
	// 4. MCP bridge endpoints (protected independently by bridge auth manager)
	if clean == "/mcp" || strings.HasPrefix(clean, "/mcp/") {
		return true
	}
	if clean == "/api/bridge/mcp" || strings.HasPrefix(clean, "/api/bridge/mcp/") {
		return true
	}
	// 5. Remote Control console (enforces independent 6-digit Security PIN authentication)
	if clean == "/remote" || strings.HasPrefix(clean, "/remote/") {
		return true
	}
	// 6. External channel webhooks (enforces hub.verify_token / webhook token authentication)
	if clean == "/api/bridge/channels/telegram/webhook" || clean == "/api/bridge/channels/discord/webhook" {
		// Authenticated by bot token in header / payload
		return true
	}
	if clean == "/api/bridge/channels/webhook" || strings.HasPrefix(clean, "/api/bridge/channels/webhook/") {
		return true
	}
	// 7. OAuth 2.0 / 2.1 RFC metadata and endpoints for Gemini MCP integration
	if strings.HasPrefix(clean, "/.well-known/") {
		return true
	}
	if clean == "/oauth" || strings.HasPrefix(clean, "/oauth/") {
		return true
	}
	if strings.HasPrefix(clean, "/api/bridge/oauth/") {
		return true
	}
	// 8. LLM gateway for custom models. Antigravity surfaces authenticate with the
	// gateway's own bearer token (custommodels.Gateway.authorize), not the GUI
	// session token, because the CLI only sends Authorization: Bearer <key>.
	if clean == "/v1" || strings.HasPrefix(clean, "/v1/") {
		// OpenAI-shaped model gateway: it enforces its own bearer token.
		return true
	}
	if clean == "/v1beta" || strings.HasPrefix(clean, "/v1beta/") {
		return true
	}
	// 9. Cloud Code proxy for the language server. It authenticates upstream with
	// the caller's own Google OAuth bearer and is only reachable on loopback.
	if clean == "/v1internal" || strings.HasPrefix(clean, "/v1internal:") || strings.HasPrefix(clean, "/v1internal/") {
		return true
	}
	return false
}

func isPublicIconPath(clean string) bool {
	switch clean {
	case "/favicon.ico", "/favicon.svg", "/favicon.png", "/logo.png", "/apple-touch-icon.png", "/antigravity_logo.png":
		return true
	}
	return strings.HasPrefix(clean, "/logo/")
}

// validateRequestAuth validates credentials supplied in headers or query parameters.
// Acceptable credentials include:
// 1. X-Session-Token header matching the local startup token or an active session token.
// 2. Authorization: Bearer <token> matching the session token.
// 3. X-App-Password or query password matching VerifyAppPassword when AppPasswordEnabled is on.
//
// Parameters:
//   - r: incoming http.Request.
//
// Returns:
//   - bool: true if request is authorized; false if unauthorized.
func (s *Server) validateRequestAuth(r *http.Request) bool {
	var candidate string

	// 1. Authorization header: "Bearer <token>"
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		candidate = strings.TrimSpace(authHeader[7:])
	}

	// 2. X-Session-Token header
	if candidate == "" {
		candidate = strings.TrimSpace(r.Header.Get("X-Session-Token"))
	}

	// 3. X-App-Password header
	appPasswordCandidate := strings.TrimSpace(r.Header.Get("X-App-Password"))

	// 4. Fallback URL query parameters (for streams / SSE)
	if candidate == "" {
		candidate = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if appPasswordCandidate == "" {
		appPasswordCandidate = strings.TrimSpace(r.URL.Query().Get("password"))
	}

	// Check candidate against daemon startup session token
	if candidate != "" {
		s.authMu.RLock()
		daemonTok := s.daemonSessionToken
		expiry, exists := s.sessionTokens[candidate]
		s.authMu.RUnlock()

		if daemonTok != "" && subtle.ConstantTimeCompare([]byte(candidate), []byte(daemonTok)) == 1 {
			return true
		}

		// Check against active sessionTokens cache
		if exists {
			now := time.Now()
			if now.Before(expiry) {
				return true
			}
			s.authMu.Lock()
			if exp, ok := s.sessionTokens[candidate]; ok && !now.Before(exp) {
				delete(s.sessionTokens, candidate)
			}
			s.authMu.Unlock()
		}

		// Only session-shaped values can be local session tokens.
		if isSessionTokenShaped(candidate) {
			if diskToken, err := GetLocalSessionToken(); err == nil && diskToken != "" && subtle.ConstantTimeCompare([]byte(candidate), []byte(diskToken)) == 1 {
				s.authMu.Lock()
				if s.sessionTokens == nil {
					s.sessionTokens = make(map[string]time.Time)
				}
				s.sessionTokens[candidate] = time.Now().Add(24 * time.Hour)
				s.authMu.Unlock()
				return true
			}
		}
	}

	if appPasswordCandidate == "" && (candidate == "" || isSessionTokenShaped(candidate)) {
		return false
	}

	if !s.allowLegacyAppPasswordAttempt(r) {
		return false
	}

	cfg, err := loadConfigForAuth()
	if err == nil && cfg != nil && cfg.AppPasswordEnabled {
		if appPasswordCandidate != "" {
			return verifyAppPasswordForAuth(cfg, appPasswordCandidate)
		}
		if candidate != "" && !isSessionTokenShaped(candidate) {
			return verifyAppPasswordForAuth(cfg, candidate)
		}
	}

	return false
}

// authMiddleware enforces default-deny authentication on all non-whitelisted routes.
//
// Parameters:
//   - next: the downstream http.Handler (e.g. ServeMux).
//
// Returns:
//   - http.Handler: wrapping handler that validates authentication before invoking next.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. OPTIONS requests pass through (handled by corsMiddleware)
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// 2. Public whitelist check
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// 3. Validate request authentication
		if !s.validateRequestAuth(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Unauthorized: valid session token or app password required",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
