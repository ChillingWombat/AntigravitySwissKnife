package keyring

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
	_ "modernc.org/sqlite"
)

var tokenRefreshEndpoint = GoogleOAuthTokenURL

// EnsureFreshAccessToken exchanges the account's refresh token for a fresh access token
// when the access token is empty, has an unknown expiry, or expires within 15 minutes.
func EnsureFreshAccessToken(acc *Account) bool {
	if acc == nil {
		return false
	}
	if tokenRefreshEndpoint == GoogleOAuthTokenURL {
		if os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" || isTestMockEmail(acc.Email) {
			return false
		}
	}
	rt := strings.TrimSpace(acc.RefreshToken)
	if rt == "" {
		return false
	}
	if tokenRefreshEndpoint == GoogleOAuthTokenURL && !strings.HasPrefix(rt, "1//") {
		return false
	}
	if acc.AccessToken != "" && !acc.TokenExpiry.IsZero() && time.Until(acc.TokenExpiry) > 15*time.Minute {
		return false
	}

	form := url.Values{
		"client_id":     {GoogleDefaultClientID},
		"client_secret": {GoogleDefaultClientSecret},
		"refresh_token": {rt},
		"grant_type":    {"refresh_token"},
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.PostForm(tokenRefreshEndpoint, form)
	if err != nil {
		acc.TokenExpiry = time.Time{}
		return false
	}
	defer resp.Body.Close()

	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || strings.TrimSpace(data.AccessToken) == "" {
		acc.TokenExpiry = time.Time{}
		return false
	}

	acc.AccessToken = strings.TrimSpace(data.AccessToken)
	if strings.TrimSpace(data.RefreshToken) != "" {
		acc.RefreshToken = strings.TrimSpace(data.RefreshToken)
	}
	if strings.TrimSpace(data.IDToken) != "" {
		acc.IDToken = strings.TrimSpace(data.IDToken)
	}
	if data.ExpiresIn > 120 {
		acc.TokenExpiry = time.Now().Add(time.Duration(data.ExpiresIn-60) * time.Second)
	} else {
		acc.TokenExpiry = time.Now().Add(55 * time.Minute)
	}
	return true
}

// AntigravitySecretPayload models the exact JSON schema expected by Antigravity in Linux Secret Service and standalone token files.
type AntigravitySecretPayload struct {
	Token struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		Expiry       string `json:"expiry,omitempty"`
		IDToken      string `json:"id_token,omitempty"`
		ProjectID    string `json:"project_id,omitempty"`
	} `json:"token"`
	AuthMethod string `json:"auth_method"`
}

// mintMinimalIDToken creates a valid unverified JWT carrying the email claim if no real IDToken is present.
func mintMinimalIDToken(email string) string {
	if email == "" {
		return ""
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claimsJSON, err := json.Marshal(map[string]interface{}{
		"email":          strings.TrimSpace(email),
		"email_verified": true,
	})
	if err != nil {
		return ""
	}
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	return header + "." + payload + "."
}

// buildSecretPayload constructs the standardized Antigravity JSON auth token payload.
func buildSecretPayload(acc *Account) ([]byte, error) {
	if acc == nil {
		return nil, fmt.Errorf("nil account")
	}

	payload := AntigravitySecretPayload{
		AuthMethod: "consumer",
	}
	payload.Token.AccessToken = acc.AccessToken
	payload.Token.TokenType = "Bearer"
	payload.Token.RefreshToken = acc.RefreshToken
	payload.Token.ProjectID = "aicode-consumers"
	idToken := acc.IDToken
	if idToken == "" && acc.Email != "" {
		idToken = mintMinimalIDToken(acc.Email)
	}
	payload.Token.IDToken = idToken
	hasRefresh := strings.TrimSpace(acc.RefreshToken) != ""
	if !acc.TokenExpiry.IsZero() && (!hasRefresh || time.Until(acc.TokenExpiry) > 5*time.Minute) {
		payload.Token.Expiry = acc.TokenExpiry.UTC().Format("2006-01-02T15:04:05.000000Z")
	} else if hasRefresh {
		payload.Token.Expiry = time.Now().Add(-1 * time.Minute).UTC().Format("2006-01-02T15:04:05.000000Z")
	} else {
		payload.Token.Expiry = time.Now().Add(1 * time.Hour).UTC().Format("2006-01-02T15:04:05.000000Z")
	}

	return json.Marshal(payload)
}

// WriteSecretServiceToken stores account credentials into Linux Secret Service (service=gemini, username=antigravity).
func WriteSecretServiceToken(acc *Account) error {
	if os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" || acc == nil || (acc.AccessToken == "" && acc.RefreshToken == "") {
		return nil
	}
	payloadBytes, err := buildSecretPayload(acc)
	if err != nil {
		return fmt.Errorf("failed to marshal secret payload: %w", err)
	}

	cmd := exec.Command("secret-tool", "store",
		"--label=Password for 'antigravity' on 'gemini'",
		"service", "gemini",
		"username", "antigravity",
	)
	cmd.Stdin = bytes.NewReader(payloadBytes)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("secret-tool store failed (%v): %s", err, stderr.String())
	}

	return nil
}

// SyncAppStorageLoginUser updates jetski.onboarding.lastLoginUsername in ~/.config/Antigravity/app_storage.json.
func SyncAppStorageLoginUser(email string) error {
	if isTestMockEmail(email) {
		return nil
	}
	storagePath := filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json")

	data, err := os.ReadFile(storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Antigravity config not yet created
		}
		return err
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return fmt.Errorf("failed to parse app_storage.json: %w", err)
	}

	rawMap["jetski.onboarding.lastLoginUsername"] = email
	if _, exists := rawMap["jetski.onboarding.lastLoginIsGcpTos"]; !exists {
		rawMap["jetski.onboarding.lastLoginIsGcpTos"] = "false"
	}

	updated, err := json.MarshalIndent(rawMap, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically via temporary file
	tmpPath := storagePath + ".tmp"
	if err := os.WriteFile(tmpPath, updated, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, storagePath)
}

// SyncHardwareProfileToDirs writes all 4 virtualized device telemetry identifiers
// (machineid, .updaterId, installation_id, and installation_uuid in antigravity_state.pbtxt).
func SyncHardwareProfileToDirs(prof *fingerprint.DeviceProfile, antigravityConfigDir, geminiAntigravityDir string) error {
	if prof == nil {
		return nil
	}

	if antigravityConfigDir != "" {
		if err := os.MkdirAll(antigravityConfigDir, 0755); err != nil {
			return err
		}
		// 1. Write ~/.config/Antigravity/machineid
		if prof.MachineID != "" {
			_ = os.WriteFile(filepath.Join(antigravityConfigDir, "machineid"), []byte(strings.TrimSpace(prof.MachineID)), 0644)
		}
		// 2. Write ~/.config/Antigravity/.updaterId
		if prof.UpdaterID != "" {
			_ = os.WriteFile(filepath.Join(antigravityConfigDir, ".updaterId"), []byte(strings.TrimSpace(prof.UpdaterID)), 0644)
		}
	}

	if geminiAntigravityDir != "" {
		if err := os.MkdirAll(geminiAntigravityDir, 0755); err != nil {
			return err
		}
		// 3. Write ~/.gemini/antigravity/installation_id
		if prof.InstallationID != "" {
			_ = os.WriteFile(filepath.Join(geminiAntigravityDir, "installation_id"), []byte(strings.TrimSpace(prof.InstallationID)+"\n"), 0644)
		}
		// 4. Update installation_uuid in ~/.gemini/antigravity/antigravity_state.pbtxt
		if prof.InstallationUUID != "" {
			statePath := filepath.Join(geminiAntigravityDir, "antigravity_state.pbtxt")
			uuidLine := fmt.Sprintf(`installation_uuid: "%s"`, strings.TrimSpace(prof.InstallationUUID))
			existing, err := os.ReadFile(statePath)
			var updatedContent string
			if err == nil {
				lines := strings.Split(string(existing), "\n")
				replaced := false
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "installation_uuid:") {
						lines[i] = uuidLine
						replaced = true
					}
				}
				if !replaced {
					if len(lines) > 0 && lines[len(lines)-1] == "" {
						lines[len(lines)-1] = uuidLine
						lines = append(lines, "")
					} else {
						lines = append(lines, uuidLine, "")
					}
				}
				updatedContent = strings.Join(lines, "\n")
			} else if os.IsNotExist(err) {
				updatedContent = uuidLine + "\n"
			}
			if updatedContent != "" {
				tmpPath := statePath + ".tmp"
				if errWrite := os.WriteFile(tmpPath, []byte(updatedContent), 0644); errWrite == nil {
					_ = os.Rename(tmpPath, statePath)
				}
			}
		}
	}

	return nil
}

// SyncHardwareProfile swaps Antigravity's machineid, .updaterId, installation_id,
// and antigravity_state.pbtxt installation_uuid to match the account's isolated profile.
func SyncHardwareProfile(email string, profileMgr *fingerprint.Store) error {
	if profileMgr == nil || email == "" || isTestMockEmail(email) || os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" {
		return nil
	}
	prof, err := profileMgr.GetOrCreateProfile(email)
	if err != nil || prof == nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	antigravityDir := filepath.Join(home, ".config", "Antigravity")
	geminiAntigravityDir := filepath.Join(home, ".gemini", "antigravity")
	return SyncHardwareProfileToDirs(prof, antigravityDir, geminiAntigravityDir)
}

// SyncDesktopStandaloneToken writes ~/.gemini/jetski-standalone-oauth-token for Antigravity 2.0.
func SyncDesktopStandaloneToken(acc *Account) error {
	if acc == nil || (acc.AccessToken == "" && acc.RefreshToken == "") {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	geminiDir := filepath.Join(home, ".gemini")
	_ = os.MkdirAll(geminiDir, 0755)
	targetPath := filepath.Join(geminiDir, "jetski-standalone-oauth-token")
	data, err := buildSecretPayload(acc)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0600)
}

// SyncCLIOAuthToken writes ~/.gemini/antigravity-cli/antigravity-oauth-token for Antigravity CLI (agy).
func SyncCLIOAuthToken(acc *Account) error {
	if acc == nil || (acc.AccessToken == "" && acc.RefreshToken == "") {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cliDir := filepath.Join(home, ".gemini", "antigravity-cli")
	_ = os.MkdirAll(cliDir, 0755)
	targetPath := filepath.Join(cliDir, "antigravity-oauth-token")
	data, err := buildSecretPayload(acc)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0600)
}

// SyncGoogleAccountsJSON writes ~/.gemini/google_accounts.json setting active and old accounts.
func SyncGoogleAccountsJSON(activeEmail string, allEmails []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	geminiDir := filepath.Join(home, ".gemini")
	_ = os.MkdirAll(geminiDir, 0755)
	targetPath := filepath.Join(geminiDir, "google_accounts.json")
	var old []string
	seen := make(map[string]bool)
	seen[strings.ToLower(activeEmail)] = true

	for _, em := range allEmails {
		norm := strings.ToLower(em)
		if norm != "" && !seen[norm] {
			seen[norm] = true
			old = append(old, em)
		}
	}

	payload := map[string]interface{}{
		"active": activeEmail,
		"old":    old,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0600)
}

// SyncOAuthCredsJSON writes ~/.gemini/oauth_creds.json used across CLI and extensions.
func SyncOAuthCredsJSON(acc *Account) error {
	if acc == nil || (acc.AccessToken == "" && acc.RefreshToken == "") {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	geminiDir := filepath.Join(home, ".gemini")
	_ = os.MkdirAll(geminiDir, 0755)
	targetPath := filepath.Join(geminiDir, "oauth_creds.json")

	hasRefresh := strings.TrimSpace(acc.RefreshToken) != ""
	expiryMs := time.Now().Add(1 * time.Hour).UnixMilli()
	if !acc.TokenExpiry.IsZero() && (!hasRefresh || time.Until(acc.TokenExpiry) > 5*time.Minute) {
		expiryMs = acc.TokenExpiry.UnixMilli()
	} else if hasRefresh {
		expiryMs = time.Now().Add(-1 * time.Minute).UnixMilli()
	}

	idToken := acc.IDToken
	if idToken == "" && acc.Email != "" {
		idToken = mintMinimalIDToken(acc.Email)
	}

	payload := map[string]interface{}{
		"access_token":  acc.AccessToken,
		"refresh_token": acc.RefreshToken,
		"token_type":    "Bearer",
		"expiry_date":   expiryMs,
		"id_token":      idToken,
		"scope":         "openid https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/cclog https://www.googleapis.com/auth/experimentsandconfigs https://www.googleapis.com/auth/aicode",
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0600)
}

// writeVarint writes an unsigned varint into a bytes.Buffer according to Protobuf specification.
func writeVarint(buf *bytes.Buffer, v uint64) {
	for v >= 0x80 {
		buf.WriteByte(byte(v&0x7f | 0x80))
		v >>= 7
	}
	buf.WriteByte(byte(v))
}

// buildUserStatusSentinel constructs the exact nested protobuf structure expected by
// Antigravity for antigravityUnifiedStateSync.userStatus in state.vscdb.
func buildUserStatusSentinel(email string) string {
	if email == "" {
		return ""
	}
	emailBytes := []byte(email)
	// inner protobuf:
	// field 3: string (tag = 0x1a = 3<<3 | 2)
	// field 7: string (tag = 0x3a = 7<<3 | 2)
	var inner bytes.Buffer
	inner.WriteByte(0x1a)
	writeVarint(&inner, uint64(len(emailBytes)))
	inner.Write(emailBytes)
	inner.WriteByte(0x3a)
	writeVarint(&inner, uint64(len(emailBytes)))
	inner.Write(emailBytes)

	innerB64 := base64.StdEncoding.EncodeToString(inner.Bytes())

	// outer protobuf field 2: message (tag = 0x12) containing field 1: string innerB64 (tag = 0x0a)
	var field2 bytes.Buffer
	field2.WriteByte(0x0a)
	writeVarint(&field2, uint64(len(innerB64)))
	field2.WriteString(innerB64)

	// outer message:
	// field 1: string "userStatusSentinelKey" (tag = 0x0a)
	// field 2: field2
	const sentinelKey = "userStatusSentinelKey"
	var outer bytes.Buffer
	outer.WriteByte(0x0a)
	writeVarint(&outer, uint64(len(sentinelKey)))
	outer.WriteString(sentinelKey)
	outer.WriteByte(0x12)
	writeVarint(&outer, uint64(field2.Len()))
	outer.Write(field2.Bytes())

	// top level message:
	// field 1: outer
	var top bytes.Buffer
	top.WriteByte(0x0a)
	writeVarint(&top, uint64(outer.Len()))
	top.Write(outer.Bytes())

	return base64.StdEncoding.EncodeToString(top.Bytes())
}

// extractPictureFromIDToken extracts the Google profile avatar URL from an unverified JWT ID token.
func extractPictureFromIDToken(idToken string) string {
	if idToken == "" {
		return ""
	}
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return ""
	}
	var payloadBytes []byte
	var err error
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	} {
		payloadBytes, err = enc.DecodeString(parts[1])
		if err == nil {
			break
		}
	}
	if err != nil {
		return ""
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return ""
	}
	if pic, ok := claims["picture"].(string); ok && pic != "" {
		return strings.TrimSpace(pic)
	}
	return ""
}

func protoMsgField(tag int, payload []byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte(byte((tag << 3) | 2))
	writeVarint(&buf, uint64(len(payload)))
	buf.Write(payload)
	return buf.Bytes()
}

func protoStrField(tag int, s string) []byte {
	return protoMsgField(tag, []byte(s))
}

func protoVarintField(tag int, v uint64) []byte {
	var buf bytes.Buffer
	buf.WriteByte(byte((tag << 3) | 0))
	writeVarint(&buf, v)
	return buf.Bytes()
}

// buildOAuthTokenSentinel constructs the exact protobuf structure expected by
// Antigravity for antigravityUnifiedStateSync.oauthToken in state.vscdb.
func buildOAuthTokenSentinel(acc *Account) string {
	if acc == nil || (acc.AccessToken == "" && acc.RefreshToken == "") {
		return ""
	}

	expirySecs := uint64(time.Now().Add(1 * time.Hour).Unix())
	if !acc.TokenExpiry.IsZero() {
		expirySecs = uint64(acc.TokenExpiry.Unix())
	}

	// 1. Construct inner protobuf for token info:
	// tag 1: string access_token
	// tag 2: string "Bearer"
	// tag 3: string refresh_token
	// tag 4: message expiry (tag 1: varint expirySecs, tag 2: varint 0)
	// tag 5: string id_token
	var expBuf bytes.Buffer
	expBuf.Write(protoVarintField(1, expirySecs))
	expBuf.Write(protoVarintField(2, 0))

	var inner bytes.Buffer
	inner.Write(protoStrField(1, acc.AccessToken))
	inner.Write(protoStrField(2, "Bearer"))
	inner.Write(protoStrField(3, acc.RefreshToken))
	inner.Write(protoMsgField(4, expBuf.Bytes()))
	if acc.IDToken != "" {
		inner.Write(protoStrField(5, acc.IDToken))
	}

	innerB64Str := base64.StdEncoding.EncodeToString(inner.Bytes())

	// 2. Construct authStateWithContextSentinelKey message:
	const authStateJSON = `{"state":"signedIn","context":{"project":"","showProjectError":false,"errorMessage":"","ineligibleMessage":"","verificationUrl":"","isGcpTos":false,"browserOpenFailed":false,"appealUrl":"","appealLinkText":""}}`
	var authStateInner bytes.Buffer
	authStateInner.Write(protoStrField(1, "authStateWithContextSentinelKey"))
	authStateInner.Write(protoMsgField(2, protoStrField(1, authStateJSON)))

	// 3. Construct oauthTokenInfoSentinelKey message:
	var tokenInfoInner bytes.Buffer
	tokenInfoInner.Write(protoStrField(1, "oauthTokenInfoSentinelKey"))
	tokenInfoInner.Write(protoMsgField(2, protoStrField(1, innerB64Str)))

	// 4. Combine into top-level repeated message
	var top bytes.Buffer
	top.Write(protoMsgField(1, authStateInner.Bytes()))
	top.Write(protoMsgField(1, tokenInfoInner.Bytes()))

	return base64.StdEncoding.EncodeToString(top.Bytes())
}

// SyncStateVscdb updates profileUrl, userStatus, and oauthToken in Antigravity's state.vscdb SQLite storage if present.
func SyncStateVscdb(acc *Account) error {
	if acc == nil {
		return nil
	}

	vscdbPath := filepath.Join(core.GetAntigravityHostConfigDir(), "User", "globalStorage", "state.vscdb")
	if _, err := os.Stat(vscdbPath); err != nil {
		return nil // state.vscdb not present on host
	}

	db, err := sql.Open("sqlite", vscdbPath)
	if err != nil {
		return fmt.Errorf("failed to open state.vscdb: %w", err)
	}
	defer db.Close()

	// Configure busy timeout to handle concurrent access by Antigravity Electron process
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")

	var tableCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='ItemTable'").Scan(&tableCount)
	if err != nil || tableCount == 0 {
		return nil
	}

	// 1. Update userStatus if email is set
	if acc.Email != "" {
		sentinel := buildUserStatusSentinel(acc.Email)
		if sentinel != "" {
			_, _ = db.Exec("INSERT OR REPLACE INTO ItemTable(key, value) VALUES('antigravityUnifiedStateSync.userStatus', ?)", sentinel)
		}
	}

	// 2. Update profileUrl if present
	picture := extractPictureFromIDToken(acc.IDToken)
	if picture != "" {
		_, _ = db.Exec("INSERT OR REPLACE INTO ItemTable(key, value) VALUES('antigravity.profileUrl', ?)", picture)
	}

	// 3. Update oauthToken to prevent UI-session hydration mismatch on relaunch
	oauthSentinel := buildOAuthTokenSentinel(acc)
	if oauthSentinel != "" {
		_, _ = db.Exec("INSERT OR REPLACE INTO ItemTable(key, value) VALUES('antigravityUnifiedStateSync.oauthToken', ?)", oauthSentinel)
	}

	return nil
}

// SyncAllSurfaces atomically syncs the active account across Antigravity 2.0 Desktop,
// Antigravity CLI (agy), and Antigravity VS Code Extension.
func SyncAllSurfaces(acc *Account, allEmails []string, profileMgr *fingerprint.Store) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}

	// Ensure access token is fresh before writing across all Antigravity surfaces
	_ = EnsureFreshAccessToken(acc)

	// 1. Linux Secret Service (VS Code Extension & Electron Keytar)
	_ = WriteSecretServiceToken(acc)

	// 2. Antigravity 2.0 Desktop App
	_ = SyncDesktopStandaloneToken(acc)
	_ = SyncAppStorageLoginUser(acc.Email)
	_ = SyncHardwareProfile(acc.Email, profileMgr)
	_ = SyncStateVscdb(acc)

	// 3. Antigravity CLI (agy)
	_ = SyncCLIOAuthToken(acc)
	_ = SyncGoogleAccountsJSON(acc.Email, allEmails)
	_ = SyncOAuthCredsJSON(acc)

	// 4. Cloud Accounts DB (if present)
	_ = SyncCloudAccountsActiveAccount("", acc.Email)

	return nil
}

// SyncDesktopSurface synchronizes credentials for Antigravity 2.0 Desktop only.
func SyncDesktopSurface(acc *Account, profileMgr *fingerprint.Store, allEmails ...[]string) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}
	_ = EnsureFreshAccessToken(acc)
	_ = WriteSecretServiceToken(acc)
	_ = SyncDesktopStandaloneToken(acc)
	_ = SyncAppStorageLoginUser(acc.Email)
	_ = SyncHardwareProfile(acc.Email, profileMgr)
	_ = SyncStateVscdb(acc)
	var emails []string
	if len(allEmails) > 0 {
		emails = allEmails[0]
	}
	_ = SyncGoogleAccountsJSON(acc.Email, emails)
	_ = SyncOAuthCredsJSON(acc)
	_ = SyncCloudAccountsActiveAccount("", acc.Email)
	return nil
}

// SyncCLISurface synchronizes credentials for Antigravity CLI (agy) only.
func SyncCLISurface(acc *Account, allEmails []string) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}
	_ = EnsureFreshAccessToken(acc)
	_ = SyncCLIOAuthToken(acc)
	_ = SyncGoogleAccountsJSON(acc.Email, allEmails)
	_ = SyncOAuthCredsJSON(acc)
	return nil
}

// SyncVSCodeSurface synchronizes credentials for VS Code Extension only.
func SyncVSCodeSurface(acc *Account) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}
	_ = EnsureFreshAccessToken(acc)
	_ = WriteSecretServiceToken(acc)
	return nil
}

// SyncSurface routes to the appropriate surface sync based on targetApp.
func SyncSurface(targetApp string, acc *Account, allEmails []string, profileMgr *fingerprint.Store) error {
	switch strings.ToLower(strings.TrimSpace(targetApp)) {
	case "desktop":
		return SyncDesktopSurface(acc, profileMgr, allEmails)
	case "agy":
		return SyncCLISurface(acc, allEmails)
	case "vscode":
		return SyncVSCodeSurface(acc)
	case "all", "":
		return SyncAllSurfaces(acc, allEmails, profileMgr)
	default:
		return fmt.Errorf("unknown target app: %s", targetApp)
	}
}

