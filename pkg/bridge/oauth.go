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
		"resource_name":            "AGY Bridge",
		"logo_uri":                 baseURL + "/logo.png",
		"icon_uri":                 baseURL + "/favicon.ico",
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
		"service_name":                          "AGY Bridge",
		"logo_uri":                              baseURL + "/logo.png",
		"icon_uri":                              baseURL + "/favicon.ico",
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
	clientName := strings.TrimSpace(regReq.ClientName)
	if clientName == "" {
		clientName = "AGY Bridge"
	}
	baseURL := ResolveExternalBaseURL(r, s.GetStatus().GatewayURL)

	resp := map[string]interface{}{
		"client_id":                  s.GetOAuthClientID(),
		"client_name":                clientName,
		"client_secret":              s.GetRawAuthToken(),
		"logo_uri":                   baseURL + "/logo.png",
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
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Welcome to Antigravity</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<style>
:root {
  --background: #F9F9F9;
  --foreground: #101010;
  --muted: #ECECEC;
  --secondary: #E2E2E2;
  --muted-foreground: #6e6e6e;
  --placeholder: #8a8a8a;
  --border: #E0E0E0;
  --card: #FFFFFF;
  --bg: var(--background);
  --surface: var(--card);
  --surface-alt: var(--muted);
  --text-main: var(--foreground);
  --text-sub: var(--muted-foreground);
  --primary: #3186FF;
  --primary-hover: #1a73e8;
  --emerald: #58983f;
  --red: #de5555;
  color-scheme: light;
}
@media (prefers-color-scheme: dark) {
  :root {
    --background: #101010;
    --foreground: #cccccc;
    --muted: #151515;
    --secondary: #272727;
    --muted-foreground: #6e6e6e;
    --placeholder: #484848;
    --border: #222222;
    --card: #191919;
    --emerald: #7aae66;
    --red: #de5555;
    color-scheme: dark;
  }
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: var(--bg);
  color: var(--text-main);
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  -webkit-font-smoothing: antialiased;
}
@keyframes logoEntrance {
  from { opacity: 0; transform: translateY(6px); filter: blur(4px); }
  to { opacity: 1; transform: translateY(0); filter: blur(0); }
}
@keyframes textEntrance {
  from { opacity: 0; transform: translateY(6px); filter: blur(4px); }
  to { opacity: 1; transform: translateY(0); filter: blur(0); }
}
.icon {
  width: 16px;
  height: 16px;
  display: inline-block;
  vertical-align: middle;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
  fill: none;
}
.auth-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px 20px;
  position: relative;
  overflow: hidden;
  user-select: none;
}
.auth-card {
  width: 100%;
  max-width: 340px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  position: relative;
  z-index: 10;
}
.agy-logo-stage {
  width: 64px;
  height: 64px;
  margin-bottom: 28px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  animation: logoEntrance 0.55s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.agy-glow-backdrop {
  position: absolute;
  width: 280px;
  height: 280px;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  pointer-events: none;
  z-index: 0;
  opacity: 0.32;
  filter: blur(28px);
}
.auth-title {
  font-size: 24px;
  font-weight: 400;
  color: var(--foreground);
  letter-spacing: -0.015em;
  line-height: 1.25;
  animation: textEntrance 0.6s cubic-bezier(0.16, 1, 0.3, 1) 0.06s both;
}
.auth-desc {
  margin-top: 8px;
  font-size: 13px;
  color: var(--muted-foreground);
  line-height: 1.45;
  animation: textEntrance 0.6s cubic-bezier(0.16, 1, 0.3, 1) 0.1s both;
}
.entrance-auth-panel {
  width: 100%;
  margin-top: 28px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  animation: textEntrance 0.6s cubic-bezier(0.16, 1, 0.3, 1) 0.14s both;
}
.pin-field-wrap {
  position: relative;
  width: 100%;
  display: flex;
  align-items: center;
}
.input-text {
  width: 100%;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--bg);
  border: 1px solid var(--border);
  color: var(--text-main);
  font-size: 13px;
  font-family: inherit;
  outline: none;
}
.input-text:focus {
  border-color: var(--primary);
}
.pin-input {
  width: 100%;
  height: 40px;
  padding: 0 38px;
  border-radius: 8px;
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--foreground);
  letter-spacing: 0.38em;
  font-size: 17px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  text-align: center;
  font-weight: 600;
  outline: none;
  transition: border-color 0.15s ease, background-color 0.15s ease, box-shadow 0.15s ease;
}
.pin-input::placeholder {
  color: var(--placeholder);
  letter-spacing: 0.32em;
}
.pin-input:focus {
  border-color: var(--primary);
  background: var(--card);
  box-shadow: 0 0 0 3px rgba(49, 134, 255, 0.15);
}
.pin-eye-btn {
  position: absolute;
  right: 8px;
  width: 26px;
  height: 26px;
  border: none;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
}
.pin-eye-btn:hover {
  color: var(--foreground);
  background: var(--secondary);
}
.agy-unlock-btn {
  width: 100%;
  height: 38px;
  padding: 0 16px;
  border-radius: 8px;
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--foreground);
  font-size: 13px;
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
  transition: background-color 0.15s ease, border-color 0.15s ease, transform 0.1s ease;
}
.agy-unlock-btn:hover {
  background: var(--secondary);
  border-color: var(--muted-foreground);
}
.msg-banner {
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 12px;
  display: none;
}
.msg-banner:empty {
  display: none !important;
}
.msg-banner.error {
  display: block;
  background: rgba(222, 85, 85, 0.1);
  color: var(--red);
  border: 1px solid rgba(222, 85, 85, 0.28);
}
.msg-banner.success {
  display: block;
  background: rgba(16, 185, 129, 0.1);
  color: var(--emerald);
  border: 1px solid rgba(16, 185, 129, 0.25);
}
</style>
</head>
<body>

<div id="authView" class="auth-wrapper">
  <div class="auth-card">
    <div class="agy-logo-stage">
      <svg class="agy-glow-backdrop" viewBox="0 0 255 266" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle cx="96" cy="98" r="42" fill="#FFEE48" />
        <circle cx="168" cy="96" r="48" fill="#FC413D" />
        <circle cx="88" cy="142" r="48" fill="#00B95C" />
        <circle cx="152" cy="158" r="54" fill="#3186FF" />
      </svg>
      <svg width="64" height="64" viewBox="0 0 96 96" fill="none" xmlns="http://www.w3.org/2000/svg" style="position:relative;z-index:2;">
        <g clip-path="url(#clip0_6153_71)">
          <path d="M85.9734 89.3438C91.2234 93.2812 99.0984 90.6562 91.8796 83.4375C70.2234 62.4375 74.8171 4.6875 47.9109 4.6875C21.0046 4.6875 25.5984 62.4375 3.94213 83.4375C-3.93287 91.3125 4.59838 93.2812 9.84838 89.3438C30.1921 75.5625 28.8796 51.2812 47.9109 51.2812C66.9421 51.2812 65.6296 75.5625 85.9734 89.3438Z" fill="#3186FF"/>
          <mask id="mask0_6153_71" style="mask-type:alpha" maskUnits="userSpaceOnUse" x="0" y="4" width="95" height="88">
            <path d="M85.9734 89.3438C91.2234 93.2812 99.0984 90.6562 91.8796 83.4375C70.2234 62.4375 74.8171 4.6875 47.9109 4.6875C21.0046 4.6875 25.5984 62.4375 3.94213 83.4375C-3.93287 91.3125 4.59838 93.2812 9.84838 89.3438C30.1921 75.5625 28.8796 51.2812 47.9109 51.2812C66.9421 51.2812 65.6296 75.5625 85.9734 89.3438Z" fill="black"/>
          </mask>
          <g mask="url(#mask0_6153_71)">
            <g filter="url(#filter0_f_6153_71)"><ellipse cx="25.6357" cy="30.161" rx="25.6357" ry="30.161" transform="matrix(-0.112784 0.99362 -0.99362 -0.112781 59.5896 -33.5392)" fill="#FFE432"/></g>
            <g filter="url(#filter1_f_6153_71)"><ellipse cx="93.6143" cy="23.4505" rx="33.1883" ry="33.9179" transform="rotate(76.9243 93.6143 23.4505)" fill="#FC413D"/></g>
            <g filter="url(#filter2_f_6153_71)"><ellipse cx="-4.78051" cy="30.8098" rx="34.686" ry="44.9344" transform="rotate(74.1257 -4.78051 30.8098)" fill="#00B95C"/></g>
            <g filter="url(#filter4_f_6153_71)"><ellipse cx="-2.31406" cy="32.1899" rx="33.9975" ry="37.4282" transform="rotate(45.6065 -2.31406 32.1899)" fill="#00B95C"/></g>
            <g filter="url(#filter5_f_6153_71)"><ellipse cx="70.2862" cy="101.861" rx="32.6449" ry="31.4359" transform="rotate(76.9243 70.2862 101.861)" fill="#3186FF"/></g>
            <g filter="url(#filter6_f_6153_71)"><ellipse cx="37.7619" cy="39.8298" rx="37.7619" ry="39.8298" transform="matrix(-0.409539 0.912293 -0.912294 -0.409537 98.9683 -33.1266)" fill="#FBBC04"/></g>
            <g filter="url(#filter7_f_6153_71)"><path d="M-12.0493 152.344C-32.7264 144.227 2.61016 77.4428 11.3972 55.0598C20.1841 32.6769 44.0695 21.1122 64.7466 29.2295C85.4236 37.3467 109.932 82.0552 101.145 104.438C92.3577 126.8 8.6 160.461 -12.0493 152.344Z" fill="#3186FF"/></g>
            <g filter="url(#filter8_f_6153_71)"><path d="M113.238 69.2209C107.702 75.6818 93.256 72.3872 80.9716 61.8621C68.6872 51.337 63.2163 37.567 68.752 31.1061C74.2876 24.6451 88.7336 27.9398 101.018 38.4649C113.302 48.99 118.773 62.7599 113.238 69.2209Z" fill="#749BFF"/></g>
            <g filter="url(#filter9_f_6153_71)"><ellipse cx="89.2491" cy="10.7078" rx="49.7713" ry="30.9393" transform="rotate(34.0763 89.2491 10.7078)" fill="#FC413D"/></g>
            <g filter="url(#filter10_f_6153_71)"><ellipse cx="11.4944" cy="17.2241" rx="26.6705" ry="15.5102" transform="rotate(112.516 11.4944 17.2241)" fill="#FFEE48"/></g>
          </g>
        </g>
        <defs>
          <clipPath id="clip0_6153_71"><rect width="96" height="96" fill="white"/></clipPath>
          <filter id="filter0_f_6153_71" x="-12.13" y="-45.92" width="77.73" height="68.91" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="4.38"/></filter>
          <filter id="filter1_f_6153_71" x="17.41" y="-52.10" width="152.41" height="151.10" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="21.16"/></filter>
          <filter id="filter2_f_6153_71" x="-85.01" y="-40.73" width="160.47" height="143.08" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="17.99"/></filter>
          <filter id="filter4_f_6153_71" x="-74.08" y="-39.50" width="143.53" height="143.38" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="17.99"/></filter>
          <filter id="filter5_f_6153_71" x="4.59" y="35.08" width="131.38" height="133.55" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="17.09"/></filter>
          <filter id="filter6_f_6153_71" x="-23.32" y="-84.10" width="140.97" height="138.22" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="15.49"/></filter>
          <filter id="filter7_f_6153_71" x="-46.11" y="-1.03" width="176.76" height="182.28" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="13.83"/></filter>
          <filter id="filter8_f_6153_71" x="41.83" y="3.04" width="98.32" height="94.25" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="12.38"/></filter>
          <filter id="filter9_f_6153_71" x="23.60" y="-48.08" width="131.29" height="117.57" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="10.46"/></filter>
          <filter id="filter10_f_6153_71" x="-31.98" y="-34.01" width="86.96" height="102.46" filterUnits="userSpaceOnUse"><feGaussianBlur stdDeviation="12.94"/></filter>
        </defs>
      </svg>
    </div>
    <div class="auth-title">Welcome to Antigravity</div>
    <div class="auth-desc">Antigravity Gemini Bridge — Enter your Security PIN</div>
    <form id="authForm" class="entrance-auth-panel" action="{{.ActionPath}}" method="POST">
      <div id="authError" class="msg-banner{{if .Error}} error{{end}}">{{.Error}}</div>
      <input type="hidden" name="client_id" value="{{.ClientID}}">
      <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
      <input type="hidden" name="state" value="{{.State}}">
      <input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
      <input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
      <input type="hidden" name="scope" value="{{.Scope}}">
      <input type="hidden" name="response_type" value="{{.ResponseType}}">
      <div class="pin-field-wrap">
        <input type="password" id="pinInput" name="pin" class="input-text pin-input" autocomplete="off" placeholder="••••••••" autofocus required>
        <button type="button" id="togglePinBtn" class="pin-eye-btn" onclick="togglePinVisibility()" title="Show or hide PIN">
          <svg class="icon" viewBox="0 0 24 24"><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/></svg>
        </button>
      </div>
      <button type="submit" class="agy-unlock-btn">
        <svg class="icon" viewBox="0 0 24 24"><rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
        <span>Authorize Connection</span>
      </button>
    </form>
  </div>
</div>

<script>
function togglePinVisibility() {
  var pinInput = document.getElementById('pinInput');
  if (!pinInput) return;
  pinInput.type = pinInput.type === 'password' ? 'text' : 'password';
  pinInput.focus();
}
</script>
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
	responseType := q.Get("response_type")

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
		if v := r.FormValue("response_type"); v != "" {
			responseType = v
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
	if r.URL.RawQuery != "" {
		actionPath += "?" + r.URL.RawQuery
	} else if clientID != "" || redirectURI != "" || state != "" {
		vals := url.Values{}
		if clientID != "" {
			vals.Set("client_id", clientID)
		}
		if redirectURI != "" {
			vals.Set("redirect_uri", redirectURI)
		}
		if state != "" {
			vals.Set("state", state)
		}
		if codeChallenge != "" {
			vals.Set("code_challenge", codeChallenge)
		}
		if codeChallengeMethod != "" {
			vals.Set("code_challenge_method", codeChallengeMethod)
		}
		if scope != "" {
			vals.Set("scope", scope)
		}
		if responseType != "" {
			vals.Set("response_type", responseType)
		}
		if qStr := vals.Encode(); qStr != "" {
			actionPath += "?" + qStr
		}
	}

	data := map[string]string{
		"ActionPath":          actionPath,
		"ClientID":            clientID,
		"RedirectURI":         redirectURI,
		"State":               state,
		"CodeChallenge":       codeChallenge,
		"CodeChallengeMethod": codeChallengeMethod,
		"Scope":               scope,
		"ResponseType":        responseType,
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
