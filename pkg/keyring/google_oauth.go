package keyring

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
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

// GoogleOAuthManager manages loopback token extraction flows.
type GoogleOAuthManager struct {
	clientID     string
	clientSecret string
	mu           sync.Mutex
	activeFlow   *activeGoogleFlow
}

type activeGoogleFlow struct {
	authURL      string
	redirectURI  string
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
	}
}

// OpenBrowser opens the specified URL in the user's default browser.
func OpenBrowser(targetURL string) error {
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
	return exec.Command(cmd, args...).Start()
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

// Exchange exchanges an authorization code or callback URL for OAuth tokens.
func (m *GoogleOAuthManager) Exchange(callbackURL, directCode, directRedirectURI string) (*GoogleOAuthResult, error) {
	callbackURL = strings.TrimSpace(callbackURL)
	callbackURL = strings.Trim(callbackURL, "\"'")
	directCode = strings.TrimSpace(directCode)
	directRedirectURI = strings.TrimSpace(directRedirectURI)

	code := directCode
	redirectURI := directRedirectURI

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
			if redirectURI == "" {
				redirectURI = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
			}
		} else if strings.Contains(raw, "code=") {
			q := raw
			if idx := strings.Index(q, "?"); idx != -1 {
				q = q[idx+1:]
			}
			vals, err := url.ParseQuery(q)
			if err == nil && vals.Get("code") != "" {
				code = vals.Get("code")
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

	flowCtx, cancel := context.WithCancel(ctx)
	flow := &activeGoogleFlow{
		redirectURI: redirectURI,
		result:      make(chan *GoogleOAuthResult, 1),
		err:         make(chan error, 1),
		cancel:      cancel,
	}
	m.activeFlow = flow
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		if m.activeFlow == flow {
			m.activeFlow = nil
		}
		m.mu.Unlock()
	}()

	successHTML := []byte(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Antigravity Swiss Knife - Login Successful</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background:#131314; color:#e3e3e3; display:flex; align-items:center; justify-content:center; height:100vh; margin:0;">
  <div style="background:#1e1f20; border:1px solid #3c4043; border-radius:16px; padding:36px 48px; text-align:center; max-width:440px; box-shadow: 0 4px 20px rgba(0,0,0,0.5);">
    <div style="width:52px; height:52px; margin:0 auto 16px; background:rgba(129,201,149,0.2); border-radius:50%; display:flex; align-items:center; justify-content:center; color:#81c995; font-size:24px;">&#x2713;</div>
    <h2 style="margin:0 0 8px; color:#fff; font-size:20px;">Authentication Successful</h2>
    <p style="color:#9aa0a6; font-size:14px; line-height:1.5; margin:0 0 16px;">Antigravity Swiss Knife has received and verified your credentials. You can safely close this browser window and return to the application.</p>
    <p style="color:#5f6368; font-size:12px; margin:0 0 20px;">This tab will attempt to auto-close in <span id="countdown" style="font-weight:700; color:#81c995;">5</span> seconds.</p>
    <button onclick="try{window.close();}catch(e){}try{window.open('','_self','');window.close();}catch(e){}" style="background:#81c995; color:#131314; border:none; border-radius:8px; padding:10px 28px; font-size:13px; font-weight:600; cursor:pointer;">Close Window</button>
  </div>
  <script>
    let remaining = 5;
    const countEl = document.getElementById('countdown');
    const timer = setInterval(function() {
      remaining--;
      if (countEl) countEl.textContent = remaining;
      if (remaining <= 0) {
        clearInterval(timer);
        try { window.close(); } catch(e) {}
        try { window.open('', '_self', ''); window.close(); } catch(e) {}
      }
    }, 1000);
  </script>
</body>
</html>`)

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

		code := r.URL.Query().Get("code")
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf("<html><body style='font-family:sans-serif;background:#131314;color:#f28b82;padding:40px;text-align:center;'><h2>Authentication Failed</h2><p>%s</p></body></html>", errParam)))
			select {
			case flow.err <- fmt.Errorf("oauth error from google: %s", errParam):
			default:
			}
			return
		}
		if code == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("<html><body style='font-family:sans-serif;background:#131314;color:#f28b82;padding:40px;text-align:center;'><h2>Missing code parameter</h2></body></html>"))
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
			_, _ = w.Write([]byte(fmt.Sprintf("<html><body style='font-family:sans-serif;background:#131314;color:#f28b82;padding:40px;text-align:center;'><h2>Exchange Failed</h2><p>%s</p></body></html>", err.Error())))
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
