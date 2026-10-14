package keyring

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

const (
	GoogleDefaultClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	GoogleDefaultClientSecret = "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"
	GoogleOAuthAuthURL        = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleOAuthTokenURL       = "https://oauth2.googleapis.com/token"
	GoogleOAuthScopes         = "openid email profile https://www.googleapis.com/auth/cloud-platform"
	GoogleUserInfoURL         = "https://www.googleapis.com/oauth2/v3/userinfo"
)

// GoogleOAuthResult holds credentials extracted from a Google OAuth flow.
type GoogleOAuthResult struct {
	Email        string `json:"email"`
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
}

// OAuthStateCache is an expiring, thread-safe cache for one-time OAuth CSRF state parameters.
type OAuthStateCache struct {
	mu     sync.Mutex
	states map[string]time.Time
}

func newOAuthStateCache() *OAuthStateCache {
	return &OAuthStateCache{
		states: make(map[string]time.Time),
	}
}

// Put registers a state string with a specific TTL.
func (c *OAuthStateCache) Put(state string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanExpiredLocked()
	c.states[state] = time.Now().Add(ttl)
}

// ValidateAndConsume checks that the candidate state matches a cached state via constant-time comparison,
// verifies that it has not expired, and removes it upon verification (single-use).
func (c *OAuthStateCache) ValidateAndConsume(candidate string) (bool, string) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false, "missing oauth state parameter"
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var foundKey string
	var foundExp time.Time
	var matched bool

	for s, exp := range c.states {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(s)) == 1 {
			foundKey = s
			foundExp = exp
			matched = true
			break
		}
	}

	c.cleanExpiredLocked()

	if !matched {
		return false, "invalid oauth state parameter (potential CSRF)"
	}

	if time.Now().After(foundExp) {
		delete(c.states, foundKey)
		return false, "oauth state parameter has expired"
	}

	delete(c.states, foundKey)
	return true, ""
}

func (c *OAuthStateCache) cleanExpiredLocked() {
	now := time.Now()
	for s, exp := range c.states {
		if now.After(exp) {
			delete(c.states, s)
		}
	}
}

// GoogleOAuthManager manages loopback token extraction flows.
type GoogleOAuthManager struct {
	clientID     string
	clientSecret string
	mu           sync.Mutex
	activeFlow   *activeGoogleFlow
	stateCache   *OAuthStateCache
}

func (m *GoogleOAuthManager) getStateCache() *OAuthStateCache {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stateCache == nil {
		m.stateCache = newOAuthStateCache()
	}
	return m.stateCache
}

// GetActiveState returns the random state parameter of the currently waiting flow, if any.
func (m *GoogleOAuthManager) GetActiveState() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeFlow != nil {
		return m.activeFlow.state
	}
	return ""
}

type activeGoogleFlow struct {
	authURL      string
	redirectURI  string
	state        string
	createdAt    time.Time
	expiresAt    time.Time
	cachedTokens *GoogleOAuthResult
	result       chan *GoogleOAuthResult
	err          chan error
	server       *http.Server
	cancel       context.CancelFunc
}

// NewGoogleOAuthManager creates a new extractor instance.
func NewGoogleOAuthManager(clientID, clientSecret string) *GoogleOAuthManager {
	if clientID == "" {
		clientID = GoogleDefaultClientID
	}
	if clientSecret == "" {
		clientSecret = GoogleDefaultClientSecret
	}
	return &GoogleOAuthManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		stateCache:   newOAuthStateCache(),
	}
}

// OpenBrowser opens the specified URL in the user's default browser.
func OpenBrowser(targetURL string) error {
	if core.IsRunningTests() {
		return nil
	}
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", targetURL}
	case "darwin":
		cmd = "open"
		args = []string{targetURL}
	default:
		// Linux, BSD, etc.
		cmd = "xdg-open"
		args = []string{targetURL}
	}
	return core.LaunchDetachedProcess(exec.Command(cmd, args...))
}

// CancelFlow aborts any active OAuth loopback flow and shuts down the loopback server immediately.
func (m *GoogleOAuthManager) CancelFlow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeFlow != nil {
		if m.activeFlow.cancel != nil {
			m.activeFlow.cancel()
		}
		if m.activeFlow.server != nil {
			_ = m.activeFlow.server.Close()
		}
		m.activeFlow = nil
	}
}

// IsFlowActive returns whether an OAuth authorization flow is currently waiting for callback.
func (m *GoogleOAuthManager) IsFlowActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeFlow != nil
}

// GetActiveAuthURL returns the authorization URL of the currently waiting flow, if any.
func (m *GoogleOAuthManager) GetActiveAuthURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeFlow != nil {
		return m.activeFlow.authURL
	}
	return ""
}

// Exchange exchanges an authorization code or callback URL for OAuth tokens with CSRF state verification.
func (m *GoogleOAuthManager) Exchange(callbackURL, directCode, directRedirectURI string, directState ...string) (*GoogleOAuthResult, error) {
	callbackURL = strings.TrimSpace(callbackURL)
	callbackURL = strings.Trim(callbackURL, "\"'")
	directCode = strings.TrimSpace(directCode)
	directRedirectURI = strings.TrimSpace(directRedirectURI)

	code := directCode
	redirectURI := directRedirectURI
	var candidateState string
	if len(directState) > 0 {
		candidateState = strings.TrimSpace(directState[0])
	}

	if callbackURL != "" {
		raw := callbackURL
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			if strings.Contains(raw, "/oauth/callback") || strings.Contains(raw, "127.0.0.1") || strings.Contains(raw, "localhost") {
				raw = "http://" + raw
			}
		}

		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
			u, err := url.Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid return URL: %w", err)
			}
			if errParam := u.Query().Get("error"); errParam != "" {
				desc := u.Query().Get("error_description")
				if desc != "" {
					return nil, fmt.Errorf("google returned error: %s (%s)", errParam, desc)
				}
				return nil, fmt.Errorf("google returned error: %s", errParam)
			}
			if parsedCode := u.Query().Get("code"); parsedCode != "" {
				code = parsedCode
			}
			if s := u.Query().Get("state"); s != "" {
				candidateState = s
			}
			if redirectURI == "" {
				redirectURI = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
			}
		} else if strings.Contains(raw, "code=") {
			q := raw
			if idx := strings.Index(q, "?"); idx != -1 {
				q = q[idx+1:]
			}
			vals, err := url.ParseQuery(q)
			if err == nil {
				if vals.Get("code") != "" {
					code = vals.Get("code")
				}
				if vals.Get("state") != "" {
					candidateState = vals.Get("state")
				}
			}
		} else if code == "" {
			code = raw
		}
	}

	if code == "" {
		return nil, fmt.Errorf("no authorization code found in the return URL")
	}

	m.mu.Lock()
	active := m.activeFlow
	if redirectURI == "" && active != nil && active.redirectURI != "" {
		redirectURI = active.redirectURI
	}
	m.mu.Unlock()

	if redirectURI == "" {
		return nil, fmt.Errorf("unable to determine redirect URI; please paste the full return URL starting with http://127.0.0.1:.../oauth/callback")
	}

	// Validate CSRF state parameter
	valid, reason := m.getStateCache().ValidateAndConsume(candidateState)
	if !valid {
		return nil, fmt.Errorf("oauth state verification failed: %s", reason)
	}

	m.mu.Lock()
	if active != nil {
		active.state = ""
	}
	m.mu.Unlock()

	tokens, err := exchangeGoogleCode(m.clientID, m.clientSecret, code, redirectURI)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if m.activeFlow != nil && m.activeFlow == active {
		active.cachedTokens = tokens
		select {
		case active.result <- tokens:
		default:
		}
	}
	m.mu.Unlock()

	return tokens, nil
}

// StartFlow starts the local loopback server, launches the browser, and returns the tokens.
func (m *GoogleOAuthManager) StartFlow(ctx context.Context, openBrowser bool) (*GoogleOAuthResult, error) {
	m.mu.Lock()
	if m.activeFlow != nil {
		if m.activeFlow.cancel != nil {
			m.activeFlow.cancel()
		}
		if m.activeFlow.server != nil {
			_ = m.activeFlow.server.Close()
		}
		m.activeFlow = nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("failed to bind loopback listener: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/oauth/callback", port)

	// Generate 32 bytes of cryptographically secure randomness (64 hex characters)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		m.mu.Unlock()
		_ = listener.Close()
		return nil, fmt.Errorf("failed to generate secure oauth state: %w", err)
	}
	state := hex.EncodeToString(b)
	if m.stateCache == nil {
		m.stateCache = newOAuthStateCache()
	}
	cache := m.stateCache

	flowCtx, cancel := context.WithCancel(ctx)
	flow := &activeGoogleFlow{
		redirectURI: redirectURI,
		state:       state,
		createdAt:   time.Now(),
		expiresAt:   time.Now().Add(10 * time.Minute),
		result:      make(chan *GoogleOAuthResult, 1),
		err:         make(chan error, 1),
		cancel:      cancel,
	}
	m.activeFlow = flow
	m.mu.Unlock()

	cache.Put(state, 10*time.Minute)

	defer func() {
		m.mu.Lock()
		if m.activeFlow == flow {
			m.activeFlow = nil
		}
		m.mu.Unlock()
	}()

	successHTML := []byte(oauthSuccessHTML)
	renderErrorHTML := renderOAuthErrorHTML

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		cached := flow.cachedTokens
		m.mu.Unlock()
		if cached != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(successHTML)
			return
		}

		// Enforce CSRF state verification before any token exchange.
		// Detailed explanation:
		// Parameters:
		//   callbackState - the opaque cryptographic token passed back in the URL query string ("state=...").
		// Purpose:
		//   ValidateAndConsume checks that callbackState matches an active flow in our state cache and has not
		//   been replayed. If validation fails (e.g., from an unauthorized local probe or port scanner),
		//   we reject the caller with HTTP 400 Bad Request.
		// Crucial DoS Protection:
		//   We intentionally do NOT dispatch an error to flow.err here. Sending to flow.err would cause
		//   StartFlow to immediately close the loopback HTTP server and cancel the login flow.
		//   By returning without signaling flow.err, the server stays listening on 127.0.0.1 so that
		//   the legitimate user redirect from Google can still succeed when it arrives.
		callbackState := r.URL.Query().Get("state")
		valid, reason := m.getStateCache().ValidateAndConsume(callbackState)
		if !valid {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write(renderErrorHTML("Security Error: Invalid State", reason))
			return
		}

		m.mu.Lock()
		flow.state = ""
		m.mu.Unlock()

		code := r.URL.Query().Get("code")
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write(renderErrorHTML("Authentication Failed", errParam))
			select {
			case flow.err <- fmt.Errorf("oauth error from google: %s", errParam):
			default:
			}
			return
		}
		if code == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write(renderErrorHTML("Missing Code Parameter", "No authorization code was provided in the callback URL."))
			select {
			case flow.err <- fmt.Errorf("no authorization code provided"):
			default:
			}
			return
		}

		// Exchange code for tokens
		tokens, err := exchangeGoogleCode(m.clientID, m.clientSecret, code, redirectURI)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(renderErrorHTML("Exchange Failed", err.Error()))
			select {
			case flow.err <- err:
			default:
			}
			return
		}

		m.mu.Lock()
		flow.cachedTokens = tokens
		m.mu.Unlock()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(successHTML)

		select {
		case flow.result <- tokens:
		default:
		}
	})

	server := &http.Server{Handler: mux}
	flow.server = server

	go func() {
		_ = server.Serve(listener)
	}()

	authParams := url.Values{
		"client_id":     {m.clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {GoogleOAuthScopes},
		"access_type":   {"offline"},
		"prompt":        {"select_account consent"},
		"state":         {state},
	}
	authURL := fmt.Sprintf("%s?%s", GoogleOAuthAuthURL, authParams.Encode())
	m.mu.Lock()
	if m.activeFlow == flow {
		flow.authURL = authURL
	}
	m.mu.Unlock()

	if openBrowser {
		_ = OpenBrowser(authURL)
	}

	select {
	case res := <-flow.result:
		go func() {
			time.Sleep(6 * time.Second)
			_ = server.Close()
		}()
		return res, nil
	case err := <-flow.err:
		_ = server.Close()
		return nil, err
	case <-flowCtx.Done():
		_ = server.Close()
		return nil, flowCtx.Err()
	}
}

func exchangeGoogleCode(clientID, clientSecret, code, redirectURI string) (*GoogleOAuthResult, error) {
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}

	resp, err := http.PostForm(GoogleOAuthTokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to parse token json: %w", err)
	}

	if data.Error != "" {
		return nil, fmt.Errorf("google returned token error: %s (%s)", data.Error, data.ErrorDesc)
	}

	email := decodeIDTokenEmail(data.IDToken)
	if email == "" && data.AccessToken != "" {
		email = fetchUserInfoEmail(data.AccessToken)
	}

	return &GoogleOAuthResult{
		Email:        email,
		RefreshToken: data.RefreshToken,
		AccessToken:  data.AccessToken,
	}, nil
}

func decodeIDTokenEmail(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payloadSegment := parts[1]
	// URL-safe base64 unpad
	if l := len(payloadSegment) % 4; l > 0 {
		payloadSegment += strings.Repeat("=", 4-l)
	}
	decoded, err := base64.URLEncoding.DecodeString(payloadSegment)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(decoded, &claims); err == nil {
		return claims.Email
	}
	return ""
}

func fetchUserInfoEmail(accessToken string) string {
	req, err := http.NewRequest("GET", GoogleUserInfoURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var claims struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err == nil {
		return claims.Email
	}
	return ""
}

const oauthSuccessHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Antigravity Swiss Knife - Login Successful</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background:#f8fafd; color:#1f1f1f; display:flex; align-items:center; justify-content:center; height:100vh; margin:0;">
  <div style="background:#ffffff; border:1px solid #dadce0; border-radius:12px; padding:36px 48px; text-align:center; max-width:440px; box-shadow: 0 1px 3px rgba(60,64,67,0.08), 0 4px 12px rgba(60,64,67,0.05);">
    <div style="width:52px; height:52px; margin:0 auto 16px; background:#e8f0fe; border-radius:50%; display:flex; align-items:center; justify-content:center; color:#1a73e8;">
      <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#1a73e8" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></svg>
    </div>
    <h2 style="margin:0 0 8px; color:#1f1f1f; font-size:20px; font-weight:700;">Authentication Successful</h2>
    <p style="color:#5f6368; font-size:14px; line-height:1.5; margin:0 0 16px;">Antigravity Swiss Knife has received and verified your credentials. You can safely close this browser window and return to the application.</p>
    <p id="countdown-notice" style="color:#5f6368; font-size:12px; margin:0 0 20px;">This tab will attempt to auto-close in <span id="countdown" style="font-weight:700; color:#1a73e8;">5</span> seconds.</p>
    <button id="close-btn" onclick="safeCloseWindow()" style="background:#1a73e8; color:#ffffff; border:none; border-radius:9999px; padding:10px 28px; font-size:13px; font-weight:600; cursor:pointer; white-space:nowrap; box-shadow:0 1px 2px rgba(26,115,232,0.2);">Close Window</button>
  </div>
  <script>
    function isEmbeddedOrVSCode() {
      try {
        var ua = (navigator && navigator.userAgent) || "";
        if (/Code\/|Electron\/|VSCode/i.test(ua)) return true;
        if (typeof window.vscode !== "undefined" || typeof window.__vscode__ !== "undefined") return true;
        if (window.parent && window.parent !== window) return true;
      } catch (_) {}
      return false;
    }

    function safeCloseWindow() {
      if (isEmbeddedOrVSCode()) {
        var notice = document.getElementById('countdown-notice');
        if (notice) notice.textContent = 'Authentication complete. You can close this editor tab or return to your code.';
        return;
      }
      try {
        if (window.opener) {
          window.close();
        }
      } catch (_) {}
    }

    if (isEmbeddedOrVSCode()) {
      var notice = document.getElementById('countdown-notice');
      if (notice) notice.textContent = 'Authentication complete. You can safely close this editor tab or return to your code.';
      var btn = document.getElementById('close-btn');
      if (btn) btn.style.display = 'none';
    } else {
      let remaining = 5;
      const countEl = document.getElementById('countdown');
      const timer = setInterval(function() {
        remaining--;
        if (countEl) countEl.textContent = remaining;
        if (remaining <= 0) {
          clearInterval(timer);
          safeCloseWindow();
        }
      }, 1000);
    }
  </script>
</body>
</html>`

func renderOAuthErrorHTML(title, detail string) []byte {
	safeTitle := html.EscapeString(title)
	safeDetail := html.EscapeString(detail)
	return []byte(fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Antigravity Swiss Knife - %s</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background:#f8fafd; color:#1f1f1f; display:flex; align-items:center; justify-content:center; height:100vh; margin:0;">
  <div style="background:#ffffff; border:1px solid #dadce0; border-radius:12px; padding:36px 48px; text-align:center; max-width:440px; box-shadow: 0 1px 3px rgba(60,64,67,0.08), 0 4px 12px rgba(60,64,67,0.05);">
    <div style="width:52px; height:52px; margin:0 auto 16px; background:#fce8e6; border-radius:50%%; display:flex; align-items:center; justify-content:center; color:#d93025;">
      <svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#d93025" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/></svg>
    </div>
    <h2 style="margin:0 0 8px; color:#d93025; font-size:20px; font-weight:700;">%s</h2>
    <p style="color:#5f6368; font-size:14px; line-height:1.5; margin:0 0 20px;">%s</p>
    <button id="err-close-btn" onclick="safeCloseWindow()" style="background:#1a73e8; color:#ffffff; border:none; border-radius:9999px; padding:10px 28px; font-size:13px; font-weight:600; cursor:pointer; white-space:nowrap; box-shadow:0 1px 2px rgba(26,115,232,0.2);">Close Window</button>
  </div>
  <script>
    function isEmbeddedOrVSCode() {
      try {
        var ua = (navigator && navigator.userAgent) || "";
        if (/Code\/|Electron\/|VSCode/i.test(ua)) return true;
        if (typeof window.vscode !== "undefined" || typeof window.__vscode__ !== "undefined") return true;
        if (window.parent && window.parent !== window) return true;
      } catch (_) {}
      return false;
    }
    function safeCloseWindow() {
      if (isEmbeddedOrVSCode()) return;
      try { if (window.opener) window.close(); } catch (_) {}
    }
    if (isEmbeddedOrVSCode()) {
      var btn = document.getElementById('err-close-btn');
      if (btn) btn.style.display = 'none';
    }
  </script>
</body>
</html>`, safeTitle, safeTitle, safeDetail))
}
