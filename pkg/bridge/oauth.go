package bridge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type authCodeEntry struct {
	Code                string
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	Scope               string
	ExpiresAt           time.Time
}

func (s *Service) saveAuthCode(entry authCodeEntry) {
	s.oauthMu.Lock()
	defer s.oauthMu.Unlock()
	if s.authCodes == nil {
		s.authCodes = make(map[string]authCodeEntry)
	}
	now := time.Now()
	for k, v := range s.authCodes {
		if now.After(v.ExpiresAt) {
			delete(s.authCodes, k)
		}
	}
	s.authCodes[entry.Code] = entry
}

func (s *Service) consumeAuthCode(code string) (authCodeEntry, bool) {
	s.oauthMu.Lock()
	defer s.oauthMu.Unlock()
	if s.authCodes == nil {
		return authCodeEntry{}, false
	}
	entry, exists := s.authCodes[code]
	if !exists {
		return authCodeEntry{}, false
	}
	delete(s.authCodes, code)
	if time.Now().After(entry.ExpiresAt) {
		return authCodeEntry{}, false
	}
	return entry, true
}

func generateOAuthCode() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("code-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func isLoopbackHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	if strings.HasPrefix(h, "http://") {
		h = strings.TrimPrefix(h, "http://")
	} else if strings.HasPrefix(h, "https://") {
		h = strings.TrimPrefix(h, "https://")
	}
	if idx := strings.Index(h, "/"); idx != -1 {
		h = h[:idx]
	}
	if splitHost, _, err := net.SplitHostPort(h); err == nil {
		h = splitHost
	}
	if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// ResolveExternalBaseURL determines the external base URL (e.g. https://... or http://...)
// derived from X-Forwarded-Proto, X-Forwarded-Host, r.Host, or a fallback gateway URL.
// Non-loopback hosts default to https (e.g. *.ts.net, *.trycloudflare.com), while loopback defaults to http.
func ResolveExternalBaseURL(r *http.Request, fallbackGatewayURL string) string {
	fallbackGatewayURL = strings.TrimRight(strings.TrimSpace(fallbackGatewayURL), "/")

	if r == nil {
		if fallbackGatewayURL != "" {
			return fallbackGatewayURL
		}
		return "http://127.0.0.1:8765"
	}

	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}

	if (host == "" || isLoopbackHost(host)) && fallbackGatewayURL != "" && !isLoopbackHost(fallbackGatewayURL) {
		return fallbackGatewayURL
	}

	if host == "" {
		if fallbackGatewayURL != "" {
			return fallbackGatewayURL
		}
		return "http://127.0.0.1:8765"
	}

	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else if isLoopbackHost(host) {
			proto = "http"
		} else {
			proto = "https"
		}
	}

	return fmt.Sprintf("%s://%s", proto, host)
}

// ServeOAuthProtectedResource serves RFC 9728 OAuth 2.0 Protected Resource Metadata.
func (s *Service) ServeOAuthProtectedResource(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	baseURL := ResolveExternalBaseURL(r, s.GetStatus().GatewayURL)
	metadata := map[string]interface{}{
		"resource":                 fmt.Sprintf("%s/api/bridge/mcp", baseURL),
		"authorization_servers":    []string{baseURL},
		"bearer_methods_supported": []string{"header", "body", "query"},
		"scopes_supported":         []string{"mcp", "gemini", "claudeai", "openid", "profile"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(metadata)
}

// ServeOAuthMetadata serves RFC 8414 Authorization Server Metadata and OpenID Configuration.
func (s *Service) ServeOAuthMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	baseURL := ResolveExternalBaseURL(r, s.GetStatus().GatewayURL)
	metadata := map[string]interface{}{
		"issuer":                                baseURL,
		"authorization_endpoint":                fmt.Sprintf("%s/oauth/authorize", baseURL),
		"token_endpoint":                        fmt.Sprintf("%s/oauth/token", baseURL),
		"registration_endpoint":                 fmt.Sprintf("%s/oauth/register", baseURL),
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token", "client_credentials"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic", "none"},
		"code_challenge_methods_supported":      []string{"S256", "plain"},
		"scopes_supported":                      []string{"mcp", "gemini", "claudeai", "openid", "profile"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(metadata)
}

// ServeOAuthRegister serves RFC 7591 Dynamic Client Registration.
func (s *Service) ServeOAuthRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var regReq struct {
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
		GrantTypes   []string `json:"grant_types"`
		Scope        string   `json:"scope"`
	}
	_ = json.NewDecoder(r.Body).Decode(&regReq)

	redirectURIs := regReq.RedirectURIs
	if redirectURIs == nil {
		redirectURIs = []string{}
	}

	resp := map[string]interface{}{
		"client_id":                  s.GetOAuthClientID(),
		"client_secret":              s.GetRawAuthToken(),
		"redirect_uris":              redirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token", "client_credentials"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "client_secret_post",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

var oauthAuthorizeTemplate = template.Must(template.New("oauth_authorize").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Authorize Google Gemini Bridge</title>
  <link rel="icon" type="image/svg+xml" href="/favicon.svg">
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background-color: #0b0f19;
      color: #f1f5f9;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      padding: 20px;
    }
    .card {
      background-color: #131b2e;
      border: 1px solid #1e293b;
      border-radius: 12px;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
      width: 100%;
      max-width: 440px;
      padding: 32px;
    }
    .icon-header {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 20px;
    }
    .icon-wrapper {
      width: 40px;
      height: 40px;
      border-radius: 8px;
      background-color: rgba(37, 99, 235, 0.15);
      border: 1px solid rgba(37, 99, 235, 0.3);
      display: flex;
      align-items: center;
      justify-content: center;
      color: #60a5fa;
    }
    .title {
      font-size: 18px;
      font-weight: 600;
      color: #ffffff;
      line-height: 1.3;
    }
    .subtitle {
      font-size: 13px;
      color: #94a3b8;
      margin-bottom: 24px;
      line-height: 1.5;
    }
    .error-banner {
      background-color: rgba(239, 68, 68, 0.1);
      border: 1px solid rgba(239, 68, 68, 0.25);
      color: #f87171;
      border-radius: 8px;
      padding: 10px 14px;
      font-size: 13px;
      margin-bottom: 20px;
      line-height: 1.4;
    }
    .form-group {
      margin-bottom: 20px;
    }
    .label {
      display: block;
      font-size: 13px;
      font-weight: 500;
      color: #cbd5e1;
      margin-bottom: 8px;
    }
    .input {
      width: 100%;
      background-color: #0b0f19;
      border: 1px solid #334155;
      border-radius: 8px;
      padding: 11px 14px;
      font-size: 15px;
      color: #ffffff;
      letter-spacing: 1px;
      transition: border-color 0.15s ease;
    }
    .input:focus {
      outline: none;
      border-color: #3b82f6;
      box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
    }
    .helper-text {
      font-size: 12px;
      color: #64748b;
      margin-top: 6px;
      line-height: 1.4;
    }
    .submit-btn {
      width: 100%;
      background-color: #2563eb;
      color: #ffffff;
      border: none;
      border-radius: 8px;
      padding: 11px 16px;
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
      white-space: nowrap;
      transition: background-color 0.15s ease;
    }
    .submit-btn:hover {
      background-color: #1d4ed8;
    }
    .footer-note {
      margin-top: 20px;
      font-size: 12px;
      color: #64748b;
      text-align: center;
      line-height: 1.5;
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="icon-header">
      <div class="icon-wrapper">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
        </svg>
      </div>
      <div>
        <h1 class="title">Gemini Bridge Authorization</h1>
      </div>
    </div>
    <p class="subtitle">Google Gemini is requesting access to execute MCP tools on Antigravity Swiss Knife.</p>

    {{if .Error}}
    <div class="error-banner">{{.Error}}</div>
    {{end}}

    <form action="{{.ActionPath}}" method="POST">
      <input type="hidden" name="client_id" value="{{.ClientID}}">
      <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
      <input type="hidden" name="state" value="{{.State}}">
      <input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
      <input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
      <input type="hidden" name="scope" value="{{.Scope}}">

      <div class="form-group">
        <label class="label" for="pin">{{.PINLabel}}</label>
        <input id="pin" name="pin" type="password" class="input" placeholder="{{.PINPlaceholder}}" autocomplete="off" autofocus required>
        <div class="helper-text">{{.PINHelper}}</div>
      </div>

      <button type="submit" class="submit-btn">Authorize Connection</button>
    </form>

    <div class="footer-note">Close this tab to cancel authorization.</div>
  </div>
</body>
</html>`))

func (s *Service) renderAuthorizePage(w http.ResponseWriter, r *http.Request, statusCode int, errMsg string) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	state := q.Get("state")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")
	scope := q.Get("scope")

	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		if v := r.FormValue("client_id"); v != "" {
			clientID = v
		}
		if v := r.FormValue("redirect_uri"); v != "" {
			redirectURI = v
		}
		if v := r.FormValue("state"); v != "" {
			state = v
		}
		if v := r.FormValue("code_challenge"); v != "" {
			codeChallenge = v
		}
		if v := r.FormValue("code_challenge_method"); v != "" {
			codeChallengeMethod = v
		}
		if v := r.FormValue("scope"); v != "" {
			scope = v
		}
	}

	useAppPass := s.config.UseAppPassword && IsAppPasswordConfigured()
	pinLabel := "8-Character Security PIN"
	pinPlaceholder := "Enter 8-character pairing PIN"
	pinHelper := "Find your pairing PIN in Antigravity Swiss Knife Desktop > Gemini Bridge tab."
	if useAppPass {
		pinLabel = "App Access Password"
		pinPlaceholder = "Enter application access password"
		pinHelper = "Use the application access password configured in Settings > General."
	}

	actionPath := r.URL.Path
	if actionPath == "" {
		actionPath = "/oauth/authorize"
	}

	data := map[string]string{
		"ActionPath":          actionPath,
		"ClientID":            clientID,
		"RedirectURI":         redirectURI,
		"State":               state,
		"CodeChallenge":       codeChallenge,
		"CodeChallengeMethod": codeChallengeMethod,
		"Scope":               scope,
		"Error":               errMsg,
		"PINLabel":            pinLabel,
		"PINPlaceholder":      pinPlaceholder,
		"PINHelper":           pinHelper,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = oauthAuthorizeTemplate.Execute(w, data)
}

// ServeOAuthAuthorize handles GET and POST requests for the OAuth 2.0 authorization endpoint.
func (s *Service) ServeOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.WriteHeader(http.StatusOK)
		return
	}

	var clientID, redirectURI, state, codeChallenge, codeChallengeMethod, scope, pin string

	if r.Method == http.MethodGet {
		q := r.URL.Query()
		clientID = q.Get("client_id")
		redirectURI = q.Get("redirect_uri")
		state = q.Get("state")
		codeChallenge = q.Get("code_challenge")
		codeChallengeMethod = q.Get("code_challenge_method")
		scope = q.Get("scope")
		pin = strings.TrimSpace(q.Get("pin"))
		if pin == "" {
			pin = strings.TrimSpace(q.Get("password"))
		}

		if redirectURI == "" {
			http.Error(w, "redirect_uri is required", http.StatusBadRequest)
			return
		}

		// Fast-path: if ?pin= is already valid, immediately issue code and redirect 302
		if pin != "" && VerifyCandidatePIN(pin, s.GetRawPairingPIN(), s.config.UseAppPassword) {
			s.issueCodeAndRedirect(w, r, redirectURI, clientID, state, codeChallenge, codeChallengeMethod, scope)
			return
		}

		// Render authorize page
		s.renderAuthorizePage(w, r, http.StatusOK, "")
		return
	}

	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		clientID = r.FormValue("client_id")
		redirectURI = r.FormValue("redirect_uri")
		state = r.FormValue("state")
		codeChallenge = r.FormValue("code_challenge")
		codeChallengeMethod = r.FormValue("code_challenge_method")
		scope = r.FormValue("scope")
		pin = strings.TrimSpace(r.FormValue("pin"))
		if pin == "" {
			pin = strings.TrimSpace(r.FormValue("password"))
		}

		if redirectURI == "" {
			http.Error(w, "redirect_uri is required", http.StatusBadRequest)
			return
		}

		if VerifyCandidatePIN(pin, s.GetRawPairingPIN(), s.config.UseAppPassword) {
			s.issueCodeAndRedirect(w, r, redirectURI, clientID, state, codeChallenge, codeChallengeMethod, scope)
			return
		}

		s.renderAuthorizePage(w, r, http.StatusUnauthorized, "Invalid Security PIN or App Access Password. Please try again.")
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Service) issueCodeAndRedirect(w http.ResponseWriter, r *http.Request, redirectURI, clientID, state, codeChallenge, codeChallengeMethod, scope string) {
	code := generateOAuthCode()
	s.saveAuthCode(authCodeEntry{
		Code:                code,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Scope:               scope,
		ExpiresAt:           time.Now().Add(10 * time.Minute),
	})

	targetURL, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	q := targetURL.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	targetURL.RawQuery = q.Encode()
	http.Redirect(w, r, targetURL.String(), http.StatusFound)
}

// ServeOAuthToken handles token exchange for authorization_code, refresh_token, and client_credentials grants.
func (s *Service) ServeOAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var grantType, code, codeVerifier, redirectURI, clientID, clientSecret, refreshToken string

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		var body struct {
			GrantType    string `json:"grant_type"`
			Code         string `json:"code"`
			CodeVerifier string `json:"code_verifier"`
			RedirectURI  string `json:"redirect_uri"`
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		grantType = body.GrantType
		code = body.Code
		codeVerifier = body.CodeVerifier
		redirectURI = body.RedirectURI
		clientID = body.ClientID
		clientSecret = body.ClientSecret
		refreshToken = body.RefreshToken
	} else {
		_ = r.ParseForm()
		grantType = r.FormValue("grant_type")
		code = r.FormValue("code")
		codeVerifier = r.FormValue("code_verifier")
		redirectURI = r.FormValue("redirect_uri")
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
		refreshToken = r.FormValue("refresh_token")
	}

	if basicUser, basicPass, ok := r.BasicAuth(); ok {
		if clientID == "" {
			clientID = basicUser
		}
		if clientSecret == "" {
			clientSecret = basicPass
		}
	}

	switch grantType {
	case "authorization_code":
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "invalid_request",
				"error_description": "code is required",
			})
			return
		}

		entry, ok := s.consumeAuthCode(code)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "invalid_grant",
				"error_description": "Authorization code is invalid or has expired",
			})
			return
		}

		if entry.RedirectURI != "" && redirectURI != "" && entry.RedirectURI != redirectURI {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "invalid_grant",
				"error_description": "redirect_uri mismatch",
			})
			return
		}

		// Verify PKCE
		if entry.CodeChallenge != "" {
			if codeVerifier == "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":             "invalid_request",
					"error_description": "code_verifier is required for PKCE",
				})
				return
			}
			if strings.EqualFold(entry.CodeChallengeMethod, "S256") {
				h := sha256.Sum256([]byte(codeVerifier))
				calculated := base64.RawURLEncoding.EncodeToString(h[:])
				if calculated != entry.CodeChallenge {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":             "invalid_grant",
						"error_description": "PKCE verification failed",
					})
					return
				}
			} else {
				if codeVerifier != entry.CodeChallenge {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":             "invalid_grant",
						"error_description": "PKCE verification failed",
					})
					return
				}
			}
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  s.GetRawAuthToken(),
			"token_type":    "Bearer",
			"expires_in":    31536000,
			"refresh_token": s.GetRawAuthToken(),
			"scope":         "mcp",
		})

	case "refresh_token", "client_credentials":
		candidate := refreshToken
		if grantType == "client_credentials" || candidate == "" {
			candidate = clientSecret
		}
		candidate = strings.TrimSpace(candidate)

		valid := candidate != "" && (candidate == s.GetRawAuthToken() || VerifyCandidatePIN(candidate, s.GetRawPairingPIN(), s.config.UseAppPassword))
		if !valid {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "invalid_grant",
				"error_description": "Invalid credentials or refresh token",
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  s.GetRawAuthToken(),
			"token_type":    "Bearer",
			"expires_in":    31536000,
			"refresh_token": s.GetRawAuthToken(),
			"scope":         "mcp",
		})

	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "unsupported_grant_type",
			"error_description": fmt.Sprintf("Unsupported grant_type: %s", grantType),
		})
	}
}
