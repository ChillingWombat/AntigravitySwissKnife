package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestBridgeService(t *testing.T) *Service {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gemini_bridge.json")
	svc, err := NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create test bridge service: %v", err)
	}
	return svc
}

func TestResolveExternalBaseURL(t *testing.T) {
	// 1. Loopback Host without forwarded headers -> http
	req1 := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/bridge/mcp", nil)
	if url := ResolveExternalBaseURL(req1, ""); url != "http://127.0.0.1:8765" {
		t.Errorf("expected http://127.0.0.1:8765, got %q", url)
	}

	reqLocalhost := httptest.NewRequest("GET", "http://localhost:8765/api/bridge/mcp", nil)
	if url := ResolveExternalBaseURL(reqLocalhost, ""); url != "http://localhost:8765" {
		t.Errorf("expected http://localhost:8765, got %q", url)
	}

	// 2. Non-loopback Host (e.g. Cloudflare tunnel or Tailscale) -> https by default
	reqCF := httptest.NewRequest("GET", "http://example-quick.trycloudflare.com/api/bridge/mcp", nil)
	reqCF.Host = "example-quick.trycloudflare.com"
	if url := ResolveExternalBaseURL(reqCF, ""); url != "https://example-quick.trycloudflare.com" {
		t.Errorf("expected https://example-quick.trycloudflare.com, got %q", url)
	}

	reqTS := httptest.NewRequest("GET", "http://my-desktop.ts.net:8765/api/bridge/mcp", nil)
	reqTS.Host = "my-desktop.ts.net:8765"
	if url := ResolveExternalBaseURL(reqTS, ""); url != "https://my-desktop.ts.net:8765" {
		t.Errorf("expected https://my-desktop.ts.net:8765, got %q", url)
	}

	// 3. X-Forwarded-Proto and X-Forwarded-Host overrides
	reqFwd := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/bridge/mcp", nil)
	reqFwd.Header.Set("X-Forwarded-Proto", "https")
	reqFwd.Header.Set("X-Forwarded-Host", "custom.domain.org")
	if url := ResolveExternalBaseURL(reqFwd, ""); url != "https://custom.domain.org" {
		t.Errorf("expected https://custom.domain.org, got %q", url)
	}

	// 4. Fallback gateway URL when request host is loopback but tunnel is active
	reqLoopback := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/bridge/mcp", nil)
	if url := ResolveExternalBaseURL(reqLoopback, "https://live-tunnel.trycloudflare.com"); url != "https://live-tunnel.trycloudflare.com" {
		t.Errorf("expected fallback gateway URL https://live-tunnel.trycloudflare.com, got %q", url)
	}

	// 5. Nil request fallback
	if url := ResolveExternalBaseURL(nil, "https://fallback.example.com"); url != "https://fallback.example.com" {
		t.Errorf("expected https://fallback.example.com, got %q", url)
	}
}

func TestOAuthProtectedResource_Metadata(t *testing.T) {
	svc := setupTestBridgeService(t)

	req := httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil)
	req.Host = "gemini-mcp.trycloudflare.com"
	w := httptest.NewRecorder()

	svc.ServeOAuthProtectedResource(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json, got %q", ct)
	}

	var meta struct {
		Resource               string   `json:"resource"`
		ResourceName           string   `json:"resource_name"`
		LogoURI                string   `json:"logo_uri"`
		IconURI                string   `json:"icon_uri"`
		AuthorizationServers   []string `json:"authorization_servers"`
		BearerMethodsSupported []string `json:"bearer_methods_supported"`
		ScopesSupported        []string `json:"scopes_supported"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	expectedResource := "https://gemini-mcp.trycloudflare.com/api/bridge/mcp"
	if meta.Resource != expectedResource {
		t.Errorf("expected resource %q, got %q", expectedResource, meta.Resource)
	}
	if meta.ResourceName != "AGY Bridge" {
		t.Errorf("expected resource_name AGY Bridge, got %q", meta.ResourceName)
	}
	if meta.LogoURI != "https://gemini-mcp.trycloudflare.com/logo.png" {
		t.Errorf("unexpected logo_uri: %q", meta.LogoURI)
	}
	if meta.IconURI != "https://gemini-mcp.trycloudflare.com/favicon.ico" {
		t.Errorf("unexpected icon_uri: %q", meta.IconURI)
	}
	if len(meta.AuthorizationServers) == 0 || meta.AuthorizationServers[0] != "https://gemini-mcp.trycloudflare.com" {
		t.Errorf("expected auth server https://gemini-mcp.trycloudflare.com, got %v", meta.AuthorizationServers)
	}
	if len(meta.BearerMethodsSupported) == 0 {
		t.Errorf("expected bearer_methods_supported to be populated")
	}
}

func TestOAuthMetadata_AuthorizationServerAndOpenID(t *testing.T) {
	svc := setupTestBridgeService(t)

	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	req.Host = "gemini-mcp.trycloudflare.com"
	w := httptest.NewRecorder()

	svc.ServeOAuthMetadata(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var meta struct {
		Issuer                            string   `json:"issuer"`
		ServiceName                       string   `json:"service_name"`
		LogoURI                           string   `json:"logo_uri"`
		IconURI                           string   `json:"icon_uri"`
		AuthorizationEndpoint             string   `json:"authorization_endpoint"`
		TokenEndpoint                     string   `json:"token_endpoint"`
		RegistrationEndpoint              string   `json:"registration_endpoint"`
		ResponseTypesSupported            []string `json:"response_types_supported"`
		GrantTypesSupported               []string `json:"grant_types_supported"`
		TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
		CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
		ScopesSupported                   []string `json:"scopes_supported"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
		t.Fatalf("failed to decode metadata JSON: %v", err)
	}

	if meta.Issuer != "https://gemini-mcp.trycloudflare.com" {
		t.Errorf("unexpected issuer: %q", meta.Issuer)
	}
	if meta.ServiceName != "AGY Bridge" {
		t.Errorf("expected service_name AGY Bridge, got %q", meta.ServiceName)
	}
	if meta.LogoURI != "https://gemini-mcp.trycloudflare.com/logo.png" {
		t.Errorf("unexpected logo_uri: %q", meta.LogoURI)
	}
	if meta.IconURI != "https://gemini-mcp.trycloudflare.com/favicon.ico" {
		t.Errorf("unexpected icon_uri: %q", meta.IconURI)
	}
	if meta.AuthorizationEndpoint != "https://gemini-mcp.trycloudflare.com/oauth/authorize" {
		t.Errorf("unexpected authorization_endpoint: %q", meta.AuthorizationEndpoint)
	}
	if meta.TokenEndpoint != "https://gemini-mcp.trycloudflare.com/oauth/token" {
		t.Errorf("unexpected token_endpoint: %q", meta.TokenEndpoint)
	}
	if meta.RegistrationEndpoint != "https://gemini-mcp.trycloudflare.com/oauth/register" {
		t.Errorf("unexpected registration_endpoint: %q", meta.RegistrationEndpoint)
	}

	hasS256 := false
	for _, m := range meta.CodeChallengeMethodsSupported {
		if m == "S256" {
			hasS256 = true
		}
	}
	if !hasS256 {
		t.Errorf("expected S256 in code_challenge_methods_supported, got %v", meta.CodeChallengeMethodsSupported)
	}
}

func TestOAuthRegister_DynamicClientRegistration(t *testing.T) {
	svc := setupTestBridgeService(t)

	regBody := map[string]interface{}{
		"client_name":   "Google Gemini Client",
		"redirect_uris": []string{"https://gemini.google.com/auth/callback"},
		"grant_types":   []string{"authorization_code", "refresh_token"},
	}
	bodyBytes, _ := json.Marshal(regBody)

	req := httptest.NewRequest("POST", "/oauth/register", bytes.NewReader(bodyBytes))
	req.Host = "gemini-mcp.trycloudflare.com"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	svc.ServeOAuthRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var resp struct {
		ClientID                string   `json:"client_id"`
		ClientName              string   `json:"client_name"`
		ClientSecret            string   `json:"client_secret"`
		LogoURI                 string   `json:"logo_uri"`
		RedirectURIs            []string `json:"redirect_uris"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode register response: %v", err)
	}

	if resp.ClientID != svc.GetOAuthClientID() {
		t.Errorf("expected client_id %q, got %q", svc.GetOAuthClientID(), resp.ClientID)
	}
	if resp.ClientName != "Google Gemini Client" {
		t.Errorf("expected supplied client_name, got %q", resp.ClientName)
	}
	if resp.ClientSecret != svc.GetRawAuthToken() {
		t.Errorf("expected client_secret matching raw auth token, got %q", resp.ClientSecret)
	}
	if resp.LogoURI != "https://gemini-mcp.trycloudflare.com/logo.png" {
		t.Errorf("unexpected logo_uri: %q", resp.LogoURI)
	}
	if len(resp.RedirectURIs) != 1 || resp.RedirectURIs[0] != "https://gemini.google.com/auth/callback" {
		t.Errorf("unexpected redirect_uris: %v", resp.RedirectURIs)
	}
	if resp.TokenEndpointAuthMethod != "client_secret_post" {
		t.Errorf("expected client_secret_post, got %q", resp.TokenEndpointAuthMethod)
	}

	defaultReq := httptest.NewRequest("POST", "/oauth/register", strings.NewReader(`{}`))
	defaultReq.Host = "gemini-mcp.trycloudflare.com"
	defaultReq.Header.Set("Content-Type", "application/json")
	defaultW := httptest.NewRecorder()
	svc.ServeOAuthRegister(defaultW, defaultReq)
	if defaultW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for default registration, got %d", defaultW.Code)
	}
	var defaultResp struct {
		ClientName string `json:"client_name"`
	}
	if err := json.Unmarshal(defaultW.Body.Bytes(), &defaultResp); err != nil {
		t.Fatalf("failed to decode default register response: %v", err)
	}
	if defaultResp.ClientName != "AGY Bridge" {
		t.Errorf("expected default client_name AGY Bridge, got %q", defaultResp.ClientName)
	}
}

func TestOAuthEndToEnd_AuthorizePKCEAndMCPCall(t *testing.T) {
	svc := setupTestBridgeService(t)
	pin := svc.GetRawPairingPIN()
	if len(pin) != 8 {
		t.Fatalf("expected 8-character pairing PIN, got %q (len=%d)", pin, len(pin))
	}

	redirectURI := "https://gemini.google.com/oauth/callback"
	state := "random-state-xyz-789"

	// 1. GET /oauth/authorize without PIN renders HTML form with 200 OK
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/oauth/authorize?client_id=antigravity-gemini-bridge&redirect_uri=%s&state=%s", url.QueryEscape(redirectURI), state), nil)
	getW := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /oauth/authorize, got %d", getW.Code)
	}
	htmlBody := getW.Body.String()
	if !strings.Contains(htmlBody, "Welcome to Antigravity") ||
		!strings.Contains(htmlBody, "clip0_6153_71") ||
		!strings.Contains(htmlBody, "agy-unlock-btn") ||
		!strings.Contains(htmlBody, "togglePinVisibility") ||
		!strings.Contains(htmlBody, "name=\"pin\"") {
		t.Fatalf("expected official Antigravity 2.0 entrance page HTML, got: %s", htmlBody)
	}

	// 2. GET /oauth/authorize fast-path with valid ?pin= immediately returns 302 redirect
	fastReq := httptest.NewRequest("GET", fmt.Sprintf("/oauth/authorize?client_id=antigravity-gemini-bridge&redirect_uri=%s&state=fastState&pin=%s", url.QueryEscape(redirectURI), pin), nil)
	fastW := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(fastW, fastReq)
	if fastW.Code != http.StatusFound {
		t.Fatalf("expected 302 Found on fast-path authorize, got %d", fastW.Code)
	}
	fastLoc := fastW.Header().Get("Location")
	if !strings.Contains(fastLoc, "code=") || !strings.Contains(fastLoc, "state=fastState") {
		t.Fatalf("expected redirect to include code and state, got %q", fastLoc)
	}

	// 3. POST /oauth/authorize with invalid PIN returns 401 Unauthorized
	invalidForm := url.Values{
		"client_id":    {"antigravity-gemini-bridge"},
		"redirect_uri": {redirectURI},
		"state":        {state},
		"pin":          {"wrongpin"},
	}
	invalidReq := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(invalidForm.Encode()))
	invalidReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidW := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(invalidW, invalidReq)
	if invalidW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on invalid PIN, got %d", invalidW.Code)
	}
	if !strings.Contains(invalidW.Body.String(), "Invalid Security PIN") {
		t.Fatalf("expected error message in HTML body, got: %s", invalidW.Body.String())
	}

	// 4. POST /oauth/authorize with valid 8-character PIN and PKCE S256 challenge
	codeVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk-verifier-string-123456"
	h := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	validForm := url.Values{
		"client_id":             {"antigravity-gemini-bridge"},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"mcp"},
		"pin":                   {pin},
	}
	validReq := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(validForm.Encode()))
	validReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	validW := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(validW, validReq)

	if validW.Code != http.StatusFound {
		t.Fatalf("expected 302 Found, got %d", validW.Code)
	}

	loc := validW.Header().Get("Location")
	parsedLoc, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("failed to parse redirect location %q: %v", loc, err)
	}
	code := parsedLoc.Query().Get("code")
	if code == "" {
		t.Fatalf("expected code query parameter in redirect, got %q", loc)
	}
	if returnedState := parsedLoc.Query().Get("state"); returnedState != state {
		t.Errorf("expected state %q, got %q", state, returnedState)
	}

	// 5. POST /oauth/token with mismatched PKCE code_verifier fails (400 Bad Request)
	// Since auth code is one-time use, we first verify with a wrong verifier which consumes or fails:
	tokenFormFail := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {"wrong-verifier-value"},
		"redirect_uri":  {redirectURI},
		"client_id":     {"antigravity-gemini-bridge"},
	}
	failReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tokenFormFail.Encode()))
	failReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	failW := httptest.NewRecorder()
	svc.ServeOAuthToken(failW, failReq)
	if failW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid PKCE verifier, got %d", failW.Code)
	}

	// 6. Generate fresh code for successful token exchange
	validW2 := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(validW2, validReq)
	if validW2.Code != http.StatusFound {
		t.Fatalf("expected 302 Found on re-authorize, got %d", validW2.Code)
	}
	loc2, _ := url.Parse(validW2.Header().Get("Location"))
	freshCode := loc2.Query().Get("code")
	if freshCode == "" {
		t.Fatalf("expected fresh code")
	}

	// 7. POST /oauth/token with correct code_verifier succeeds (200 OK)
	tokenFormSuccess := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {freshCode},
		"code_verifier": {codeVerifier},
		"redirect_uri":  {redirectURI},
		"client_id":     {"antigravity-gemini-bridge"},
	}
	tokReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tokenFormSuccess.Encode()))
	tokReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokW := httptest.NewRecorder()
	svc.ServeOAuthToken(tokW, tokReq)

	if tokW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on token exchange, got %d: %s", tokW.Code, tokW.Body.String())
	}

	var tokResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(tokW.Body.Bytes(), &tokResp); err != nil {
		t.Fatalf("failed to decode token response: %v", err)
	}

	if tokResp.AccessToken != svc.GetRawAuthToken() {
		t.Errorf("expected access token to match bridge auth token, got %q", tokResp.AccessToken)
	}
	if tokResp.TokenType != "Bearer" {
		t.Errorf("expected Bearer token_type, got %q", tokResp.TokenType)
	}
	if tokResp.RefreshToken != svc.GetRawAuthToken() {
		t.Errorf("expected refresh token to match bridge auth token, got %q", tokResp.RefreshToken)
	}

	// 8. Replay protection: reusing freshCode must fail (400 Bad Request)
	replayW := httptest.NewRecorder()
	svc.ServeOAuthToken(replayW, tokReq)
	if replayW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on code replay, got %d", replayW.Code)
	}

	// 9. Use returned access_token to access /api/bridge/mcp
	mcpReq := httptest.NewRequest("POST", "/api/bridge/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`))
	mcpReq.Header.Set("Authorization", "Bearer "+tokResp.AccessToken)
	granted, reason := svc.ValidateRequest(mcpReq)
	if !granted {
		t.Fatalf("expected Bearer access_token to authenticate successfully, got rejected: %s", reason)
	}

	respBytes, err := svc.HandleMCPRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`))
	if err != nil {
		t.Fatalf("HandleMCPRequest failed: %v", err)
	}
	var mcpResp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBytes, &mcpResp); err != nil {
		t.Fatalf("failed to decode MCP response: %v", err)
	}
	if mcpResp.Result.ProtocolVersion != "2025-03-26" {
		t.Errorf("expected protocolVersion 2025-03-26 echoed back, got %q", mcpResp.Result.ProtocolVersion)
	}
}

func TestOAuthToken_RefreshTokenAndClientCredentials(t *testing.T) {
	svc := setupTestBridgeService(t)

	// 1. Refresh token grant using raw auth token
	rfForm := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {svc.GetRawAuthToken()},
	}
	rfReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(rfForm.Encode()))
	rfReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rfW := httptest.NewRecorder()
	svc.ServeOAuthToken(rfW, rfReq)
	if rfW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for refresh_token grant, got %d", rfW.Code)
	}

	// 2. Client credentials grant using raw pairing PIN
	ccForm := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"antigravity-gemini-bridge"},
		"client_secret": {svc.GetRawPairingPIN()},
	}
	ccReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(ccForm.Encode()))
	ccReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ccW := httptest.NewRecorder()
	svc.ServeOAuthToken(ccW, ccReq)
	if ccW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for client_credentials grant with PIN, got %d", ccW.Code)
	}

	// 3. Client credentials grant using HTTP Basic Auth
	basicReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader("grant_type=client_credentials"))
	basicReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	basicReq.SetBasicAuth("antigravity-gemini-bridge", svc.GetRawAuthToken())
	basicW := httptest.NewRecorder()
	svc.ServeOAuthToken(basicW, basicReq)
	if basicW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for client_credentials with Basic Auth, got %d", basicW.Code)
	}

	// 4. Invalid credentials rejection
	invalidReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader("grant_type=client_credentials&client_secret=badsecret"))
	invalidReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidW := httptest.NewRecorder()
	svc.ServeOAuthToken(invalidW, invalidReq)
	if invalidW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for invalid secret, got %d", invalidW.Code)
	}
}

func TestBridgeStatus_OAuthClientCredentialsAndRotation(t *testing.T) {
	svc := setupTestBridgeService(t)

	st := svc.GetStatus()
	if st.OAuthClientID != "antigravity-gemini-bridge" {
		t.Errorf("expected OAuthClientID 'antigravity-gemini-bridge', got %q", st.OAuthClientID)
	}
	if st.OAuthClientSecret != svc.GetRawAuthToken() {
		t.Errorf("expected OAuthClientSecret to match AuthToken, got %q", st.OAuthClientSecret)
	}

	newTok, newPIN, err := svc.RotateCredentials()
	if err != nil {
		t.Fatalf("RotateCredentials failed: %v", err)
	}

	updatedSt := svc.GetStatus()
	if updatedSt.OAuthClientSecret != newTok {
		t.Errorf("expected updated OAuthClientSecret %q, got %q", newTok, updatedSt.OAuthClientSecret)
	}
	if updatedSt.PairingPIN != newPIN {
		t.Errorf("expected updated PairingPIN %q, got %q", newPIN, updatedSt.PairingPIN)
	}
}

func TestBridgeMCP_UnauthorizedReturnsWWWAuthenticateHeader(t *testing.T) {
	svc := setupTestBridgeService(t)

	// Verify 401 WWW-Authenticate header generation for /api/bridge/mcp
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		granted, reason := svc.ValidateRequest(r)
		if !granted {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer realm=\"Antigravity Swiss Knife MCP\", resource_metadata=\"%s/.well-known/oauth-protected-resource\"", ResolveExternalBaseURL(r, svc.GetStatus().GatewayURL)))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  "unauthorized",
				"reason": reason,
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/api/bridge/mcp", strings.NewReader(`{}`))
	req.Host = "gemini-mcp.trycloudflare.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
	}

	authHeader := w.Header().Get("WWW-Authenticate")
	expected := "Bearer realm=\"Antigravity Swiss Knife MCP\", resource_metadata=\"https://gemini-mcp.trycloudflare.com/.well-known/oauth-protected-resource\""
	if authHeader != expected {
		t.Errorf("expected WWW-Authenticate header %q, got %q", expected, authHeader)
	}
}

func TestOAuthAuthorize_AntigravityEntranceStyle(t *testing.T) {
	svc := setupTestBridgeService(t)

	redirectURI := "https://gemini.google.com/oauth/callback"
	state := "test-state-style"

	req := httptest.NewRequest("GET", fmt.Sprintf("/oauth/authorize?client_id=antigravity-gemini-bridge&redirect_uri=%s&state=%s&response_type=code&scope=mcp&code_challenge=xyz&code_challenge_method=S256", url.QueryEscape(redirectURI), state), nil)
	w := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	requiredSnippets := []struct {
		desc    string
		snippet string
	}{
		{"favicon link", `<link rel="icon" type="image/svg+xml" href="/favicon.svg">`},
		{"light theme variable", `--background: #F9F9F9;`},
		{"dark theme variable", `--background: #101010;`},
		{"prefers-color-scheme media query", `@media (prefers-color-scheme: dark)`},
		{"auth wrapper", `class="auth-wrapper"`},
		{"auth card", `class="auth-card"`},
		{"logo stage", `class="agy-logo-stage"`},
		{"glow backdrop", `class="agy-glow-backdrop"`},
		{"96x96 delta clip path", `clip0_6153_71`},
		{"title", `<div class="auth-title">Welcome to Antigravity</div>`},
		{"subtitle", `<div class="auth-desc">Antigravity Gemini Bridge — Enter your Security PIN</div>`},
		{"entrance auth panel form", `<form id="authForm" class="entrance-auth-panel"`},
		{"empty msg banner", `<div id="authError" class="msg-banner"></div>`},
		{"hidden client_id", `<input type="hidden" name="client_id" value="antigravity-gemini-bridge">`},
		{"hidden redirect_uri", fmt.Sprintf(`<input type="hidden" name="redirect_uri" value="%s">`, redirectURI)},
		{"hidden state", `<input type="hidden" name="state" value="test-state-style">`},
		{"hidden code_challenge", `<input type="hidden" name="code_challenge" value="xyz">`},
		{"hidden code_challenge_method", `<input type="hidden" name="code_challenge_method" value="S256">`},
		{"hidden scope", `<input type="hidden" name="scope" value="mcp">`},
		{"hidden response_type", `<input type="hidden" name="response_type" value="code">`},
		{"pin input field", `<input type="password" id="pinInput" name="pin" class="input-text pin-input" autocomplete="off" placeholder="••••••••" autofocus required>`},
		{"pin eye toggle button", `onclick="togglePinVisibility()"`},
		{"unlock button with lock icon", `<button type="submit" class="agy-unlock-btn">`},
		{"authorize connection text", `<span>Authorize Connection</span>`},
		{"toggle pin script", `function togglePinVisibility()`},
	}

	for _, item := range requiredSnippets {
		if !strings.Contains(body, item.snippet) {
			t.Errorf("missing %s: %q in response", item.desc, item.snippet)
		}
	}

	// Verify error rendering on wrong PIN
	invalidForm := url.Values{
		"client_id":             {"antigravity-gemini-bridge"},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {"xyz"},
		"code_challenge_method": {"S256"},
		"scope":                 {"mcp"},
		"response_type":         {"code"},
		"pin":                   {"wrongpin"},
	}
	invalidReq := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(invalidForm.Encode()))
	invalidReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidW := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(invalidW, invalidReq)

	if invalidW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", invalidW.Code)
	}

	errBody := invalidW.Body.String()
	if !strings.Contains(errBody, `<div id="authError" class="msg-banner error">Invalid Security PIN`) {
		t.Errorf("expected error banner with class 'msg-banner error', got: %s", errBody)
	}
	if !strings.Contains(errBody, `<input type="hidden" name="response_type" value="code">`) {
		t.Errorf("expected preserved response_type hidden input in error page")
	}
}

func TestOAuth_TokenBruteForceLockout(t *testing.T) {
	svc := setupTestBridgeService(t)
	clientIP := "192.168.1.55:1234"

	// 1. GET /oauth/authorize without pin does not increment failed attempts
	getReq := httptest.NewRequest("GET", "/oauth/authorize?client_id=antigravity-gemini-bridge&redirect_uri=http://localhost:3000/callback&response_type=code", nil)
	getReq.RemoteAddr = clientIP
	getW := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET authorize without pin, got %d", getW.Code)
	}
	if svc.authMgr.IsIPLocked("192.168.1.55") {
		t.Fatalf("IP locked prematurely from GET authorize without pin")
	}

	// 2. 5 bad PINs to /oauth/token client_credentials
	for i := 1; i <= 5; i++ {
		form := url.Values{
			"grant_type":    {"client_credentials"},
			"client_secret": {"badpin99"},
		}
		req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = clientIP
		w := httptest.NewRecorder()
		svc.ServeOAuthToken(w, req)

		if i < 5 {
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("attempt %d: expected 401 Unauthorized, got %d", i, w.Code)
			}
		} else {
			// 5th failed attempt triggers the lockout
			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("attempt 5: expected 429 Too Many Requests, got %d", w.Code)
			}
		}
	}

	if !svc.authMgr.IsIPLocked("192.168.1.55") {
		t.Fatalf("expected IP 192.168.1.55 to be locked after 5 failed attempts")
	}

	// 3. 6th attempt with CORRECT PIN is rejected with 429
	correctForm := url.Values{
		"grant_type":    {"client_credentials"},
		"client_secret": {svc.GetRawPairingPIN()},
	}
	req6 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(correctForm.Encode()))
	req6.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req6.RemoteAddr = clientIP
	w6 := httptest.NewRecorder()
	svc.ServeOAuthToken(w6, req6)

	if w6.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests for 6th attempt with correct PIN, got %d", w6.Code)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(w6.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse JSON error: %v", err)
	}
	if errResp["error"] != "invalid_grant" {
		t.Errorf("expected error 'invalid_grant', got %v", errResp["error"])
	}

	// 4. Authorize page also returns 429 when IP is locked:
	// 4a. GET /oauth/authorize without pin must return 429
	authGetReq := httptest.NewRequest("GET", "/oauth/authorize?client_id=antigravity-gemini-bridge&redirect_uri=http://localhost:3000/callback&response_type=code", nil)
	authGetReq.RemoteAddr = clientIP
	authGetW := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(authGetW, authGetReq)
	if authGetW.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on GET authorize without pin for locked IP, got %d", authGetW.Code)
	}

	// 4b. GET /oauth/authorize with pin must return 429
	authGetPinReq := httptest.NewRequest("GET", "/oauth/authorize?client_id=antigravity-gemini-bridge&redirect_uri=http://localhost:3000/callback&response_type=code&pin="+svc.GetRawPairingPIN(), nil)
	authGetPinReq.RemoteAddr = clientIP
	authGetPinW := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(authGetPinW, authGetPinReq)
	if authGetPinW.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on GET authorize with pin for locked IP, got %d", authGetPinW.Code)
	}

	// 4c. POST /oauth/authorize must return 429
	authForm := url.Values{
		"client_id":     {"antigravity-gemini-bridge"},
		"redirect_uri":  {"http://localhost:3000/callback"},
		"response_type": {"code"},
		"pin":           {svc.GetRawPairingPIN()},
	}
	authReq := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(authForm.Encode()))
	authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authReq.RemoteAddr = clientIP
	authW := httptest.NewRecorder()
	svc.ServeOAuthAuthorize(authW, authReq)
	if authW.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on POST authorize page for locked IP, got %d", authW.Code)
	}
}

func TestOAuth_AuthorizationCodeValidation(t *testing.T) {
	svc := setupTestBridgeService(t)
	redirectURI := "http://localhost:3000/callback"
	clientID := "antigravity-gemini-bridge"

	issueCode := func(cid, ruri, challenge, method string) string {
		form := url.Values{
			"client_id":             {cid},
			"redirect_uri":          {ruri},
			"state":                 {"state123"},
			"response_type":         {"code"},
			"pin":                   {svc.GetRawPairingPIN()},
			"code_challenge":        {challenge},
			"code_challenge_method": {method},
		}
		req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		svc.ServeOAuthAuthorize(w, req)
		if w.Code != http.StatusFound {
			t.Fatalf("authorize failed: code=%d", w.Code)
		}
		loc, _ := url.Parse(w.Header().Get("Location"))
		return loc.Query().Get("code")
	}

	// (a) Client ID mismatch -> 400 invalid_grant "client_id mismatch"
	code1 := issueCode(clientID, redirectURI, "", "")
	form1 := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code1},
		"client_id":    {"wrong-client-id"},
		"redirect_uri": {redirectURI},
	}
	req1 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form1.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w1 := httptest.NewRecorder()
	svc.ServeOAuthToken(w1, req1)
	if w1.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on client_id mismatch, got %d", w1.Code)
	}
	if !strings.Contains(w1.Body.String(), "client_id mismatch") {
		t.Errorf("expected 'client_id mismatch' error, got %s", w1.Body.String())
	}

	// (b) Redirect URI mismatch or omitted -> 400 invalid_grant "redirect_uri mismatch"
	code2 := issueCode(clientID, redirectURI, "", "")
	form2 := url.Values{
		"grant_type": {"authorization_code"},
		"code":       {code2},
		"client_id":  {clientID},
	}
	req2 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	svc.ServeOAuthToken(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when redirect_uri omitted, got %d", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "redirect_uri mismatch") {
		t.Errorf("expected 'redirect_uri mismatch', got %s", w2.Body.String())
	}

	// (c) PKCE unsupported method -> 400 invalid_request
	code3 := issueCode(clientID, redirectURI, "some-challenge", "MD5")
	form3 := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code3},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"code_verifier": {"some-verifier"},
	}
	req3 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form3.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w3 := httptest.NewRecorder()
	svc.ServeOAuthToken(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for unsupported PKCE method, got %d", w3.Code)
	}
	if !strings.Contains(w3.Body.String(), "invalid_request") {
		t.Errorf("expected 'invalid_request', got %s", w3.Body.String())
	}

	// (d) PKCE plain method: valid and invalid
	codePlain := issueCode(clientID, redirectURI, "my-secret-plain-verifier", "plain")
	formPlainFail := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {codePlain},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"code_verifier": {"wrong-verifier"},
	}
	reqPFail := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(formPlainFail.Encode()))
	reqPFail.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wPFail := httptest.NewRecorder()
	svc.ServeOAuthToken(wPFail, reqPFail)
	if wPFail.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for wrong plain verifier, got %d", wPFail.Code)
	}

	codePlain2 := issueCode(clientID, redirectURI, "my-secret-plain-verifier", "plain")
	formPlainOK := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {codePlain2},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"code_verifier": {"my-secret-plain-verifier"},
	}
	reqPOK := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(formPlainOK.Encode()))
	reqPOK.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wPOK := httptest.NewRecorder()
	svc.ServeOAuthToken(wPOK, reqPOK)
	if wPOK.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid plain verifier, got %d", wPOK.Code)
	}

	// (e) Basic Auth fallback for client_id
	codeBasic := issueCode(clientID, redirectURI, "", "")
	formBasic := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {codeBasic},
		"redirect_uri": {redirectURI},
	}
	reqBasic := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(formBasic.Encode()))
	reqBasic.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBasic.SetBasicAuth(clientID, "any-password")
	wBasic := httptest.NewRecorder()
	svc.ServeOAuthToken(wBasic, reqBasic)
	if wBasic.Code != http.StatusOK {
		t.Fatalf("expected 200 with Basic Auth client_id fallback, got %d: %s", wBasic.Code, wBasic.Body.String())
	}
}
