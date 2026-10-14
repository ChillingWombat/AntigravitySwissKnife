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
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
	_ "modernc.org/sqlite"
)

var (
	tokenRefreshEndpoint = GoogleOAuthTokenURL
	onTokenRefreshed     func(acc *Account)
)

// SetTokenRefreshCallback registers a callback invoked immediately when EnsureFreshAccessToken updates tokens.
func SetTokenRefreshCallback(cb func(acc *Account)) {
	onTokenRefreshed = cb
}

// writeAtomic writes data to targetPath atomically via temp file in the same directory, fsync, and rename.
func writeAtomic(targetPath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(targetPath)+".tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	for i := 0; i < 50; i++ {
		err = os.Rename(tmpPath, targetPath)
		if err == nil {
			return nil
		}
		if runtime.GOOS == "windows" {
			time.Sleep(time.Duration(5+(i%10)*3) * time.Millisecond)
			continue
		}
		break
	}
	return err
}

// isSafeTestWritePath ensures that during automated unit/integration tests,
// writes to host configuration and token files only occur if targeted within a temporary directory.
func isSafeTestWritePath(path string) bool {
	if !core.IsRunningTests() {
		return true
	}
	cleaned := filepath.Clean(path)
	tempDir := filepath.Clean(os.TempDir())
	if runtime.GOOS == "windows" {
		cleanedLower := strings.ToLower(cleaned)
		tempDirLower := strings.ToLower(tempDir)
		return strings.HasPrefix(cleanedLower, tempDirLower) || strings.Contains(cleanedLower, "\\temp\\") || strings.Contains(cleanedLower, "\\appdata\\local\\temp")
	}
	return strings.HasPrefix(cleaned, tempDir) || strings.HasPrefix(cleaned, "/tmp") || strings.HasPrefix(cleaned, "/private/tmp")
}

// EnsureFreshAccessToken exchanges the account's refresh token for a fresh access token
// when the access token is empty, has an unknown expiry, or expires within 15 minutes.
func EnsureFreshAccessToken(acc *Account) bool {
	if acc == nil {
		return false
	}
	if tokenRefreshEndpoint == GoogleOAuthTokenURL {
		if os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" || core.IsRunningTests() || isTestMockEmail(acc.Email) {
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
	if onTokenRefreshed != nil {
		onTokenRefreshed(acc)
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
	if strings.TrimSpace(acc.AccessToken) != "" {
		if !acc.TokenExpiry.IsZero() && time.Until(acc.TokenExpiry) > 0 {
			payload.Token.Expiry = acc.TokenExpiry.UTC().Format("2006-01-02T15:04:05.000000Z")
		} else {
			payload.Token.Expiry = time.Now().Add(1 * time.Hour).UTC().Format("2006-01-02T15:04:05.000000Z")
		}
	} else if hasRefresh {
		payload.Token.Expiry = time.Now().Add(-1 * time.Minute).UTC().Format("2006-01-02T15:04:05.000000Z")
	} else {
		payload.Token.Expiry = time.Now().Add(1 * time.Hour).UTC().Format("2006-01-02T15:04:05.000000Z")
	}

	return json.Marshal(payload)
}

// WriteSecretServiceToken stores account credentials into Linux Secret Service (service=gemini, username=antigravity).
func WriteSecretServiceToken(acc *Account) error {
	if os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" || core.IsRunningTests() || acc == nil || isTestMockEmail(acc.Email) || (acc.AccessToken == "" && acc.RefreshToken == "") {
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

func lockAppStorage(storagePath string) (*os.File, error) {
	hostDir := filepath.Dir(storagePath)
	_ = os.MkdirAll(hostDir, 0755)
	lockPath := storagePath + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := core.LockFile(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func unlockAppStorage(f *os.File) {
	if f != nil {
		_ = core.UnlockFile(f)
		_ = f.Close()
	}
}

// SyncAppStorageLoginUser updates jetski.onboarding.lastLoginUsername in ~/.config/Antigravity/app_storage.json.
func SyncAppStorageLoginUser(email string) error {
	if isTestMockEmail(email) {
		return nil
	}
	storagePath := filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json")
	if !isSafeTestWritePath(storagePath) {
		return nil
	}

	lockFile, err := lockAppStorage(storagePath)
	if err == nil {
		defer unlockAppStorage(lockFile)
	}

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

	return writeAtomic(storagePath, updated, 0644)
}

// SyncHardwareProfileToDirs writes all 4 virtualized device telemetry identifiers
// (machineid, .updaterId, installation_id, and installation_uuid in antigravity_state.pbtxt).
func SyncHardwareProfileToDirs(prof *fingerprint.DeviceProfile, antigravityConfigDir, geminiAntigravityDir string) error {
	if prof == nil {
		return nil
	}

	if antigravityConfigDir != "" && isSafeTestWritePath(antigravityConfigDir) {
		if err := os.MkdirAll(antigravityConfigDir, 0755); err != nil {
			return err
		}
		// 1. Write ~/.config/Antigravity/machineid
		if prof.MachineID != "" {
			if err := writeAtomic(filepath.Join(antigravityConfigDir, "machineid"), []byte(strings.TrimSpace(prof.MachineID)), 0600); err != nil {
				return err
			}
		}
		// 2. Write ~/.config/Antigravity/.updaterId
		if prof.UpdaterID != "" {
			if err := writeAtomic(filepath.Join(antigravityConfigDir, ".updaterId"), []byte(strings.TrimSpace(prof.UpdaterID)), 0600); err != nil {
				return err
			}
		}
	}

	if geminiAntigravityDir != "" && isSafeTestWritePath(geminiAntigravityDir) {
		if err := os.MkdirAll(geminiAntigravityDir, 0755); err != nil {
			return err
		}
		// 3. Write ~/.gemini/antigravity/installation_id
		if prof.InstallationID != "" {
			if err := writeAtomic(filepath.Join(geminiAntigravityDir, "installation_id"), []byte(strings.TrimSpace(prof.InstallationID)+"\n"), 0600); err != nil {
				return err
			}
		}
		// 4. Update installation_uuid in ~/.gemini/antigravity/antigravity_state.pbtxt
		if prof.InstallationUUID != "" {
			statePath := filepath.Join(geminiAntigravityDir, "antigravity_state.pbtxt")
			uuidLine := fmt.Sprintf(`installation_uuid: "%s"`, strings.TrimSpace(prof.InstallationUUID))
			existing, err := os.ReadFile(statePath)
			var updatedContent string
			if err == nil {
				re := regexp.MustCompile(`(?m)^\s*installation_uuid\s*:\s*.*$`)
				if re.Match(existing) {
					updatedContent = re.ReplaceAllString(string(existing), uuidLine)
				} else {
					trimmed := strings.TrimRight(string(existing), "\n")
					if trimmed == "" {
						updatedContent = uuidLine + "\n"
					} else {
						updatedContent = trimmed + "\n" + uuidLine + "\n"
					}
				}
			} else if os.IsNotExist(err) {
				updatedContent = uuidLine + "\n"
			} else {
				return err
			}
			if updatedContent != "" {
				if err := writeAtomic(statePath, []byte(updatedContent), 0600); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// SyncHardwareProfile swaps Antigravity's machineid, .updaterId, installation_id,
// and antigravity_state.pbtxt installation_uuid to match the account's isolated profile.
func SyncHardwareProfile(email string, profileMgr *fingerprint.Store) error {
	if profileMgr == nil || email == "" || isTestMockEmail(email) || os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" || core.IsRunningTests() {
		return nil
	}
	prof, err := profileMgr.GetOrCreateProfile(email)
	if err != nil || prof == nil {
		return err
	}

	antigravityDir := core.GetAntigravityHostConfigDir()
	geminiAntigravityDir := core.GetAntigravityDir()
	return SyncHardwareProfileToDirs(prof, antigravityDir, geminiAntigravityDir)
}

// SyncDesktopStandaloneToken writes ~/.gemini/jetski-standalone-oauth-token for Antigravity 2.0.
func SyncDesktopStandaloneToken(acc *Account) error {
	if acc == nil || isTestMockEmail(acc.Email) || (acc.AccessToken == "" && acc.RefreshToken == "") {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	geminiDir := filepath.Join(home, ".gemini")
	targetPath := filepath.Join(geminiDir, "jetski-standalone-oauth-token")
	if !isSafeTestWritePath(targetPath) {
		return nil
	}
	_ = os.MkdirAll(geminiDir, 0755)
	data, err := buildSecretPayload(acc)
	if err != nil {
		return err
	}
	return writeAtomic(targetPath, data, 0600)
}

// SyncCLIOAuthToken writes ~/.gemini/antigravity-cli/antigravity-oauth-token for Antigravity CLI (agy).
func SyncCLIOAuthToken(acc *Account) error {
	if acc == nil || isTestMockEmail(acc.Email) || (acc.AccessToken == "" && acc.RefreshToken == "") {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cliDir := filepath.Join(home, ".gemini", "antigravity-cli")
	targetPath := filepath.Join(cliDir, "antigravity-oauth-token")
	if !isSafeTestWritePath(targetPath) {
		return nil
	}
	_ = os.MkdirAll(cliDir, 0755)
	data, err := buildSecretPayload(acc)
	if err != nil {
		return err
	}
	return writeAtomic(targetPath, data, 0600)
}

// SyncGoogleAccountsJSON writes ~/.gemini/google_accounts.json setting active and old accounts.
func SyncGoogleAccountsJSON(activeEmail string, allEmails []string) error {
	if isTestMockEmail(activeEmail) {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	geminiDir := filepath.Join(home, ".gemini")
	targetPath := filepath.Join(geminiDir, "google_accounts.json")
	if !isSafeTestWritePath(targetPath) {
		return nil
	}
	_ = os.MkdirAll(geminiDir, 0755)
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
	return writeAtomic(targetPath, data, 0600)
}

// SyncOAuthCredsJSON writes ~/.gemini/oauth_creds.json used across CLI and extensions.
func SyncOAuthCredsJSON(acc *Account) error {
	if acc == nil || isTestMockEmail(acc.Email) || (acc.AccessToken == "" && acc.RefreshToken == "") {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	geminiDir := filepath.Join(home, ".gemini")
	targetPath := filepath.Join(geminiDir, "oauth_creds.json")
	if !isSafeTestWritePath(targetPath) {
		return nil
	}
	_ = os.MkdirAll(geminiDir, 0755)

	hasRefresh := strings.TrimSpace(acc.RefreshToken) != ""
	expiryMs := time.Now().Add(1 * time.Hour).UnixMilli()
	if strings.TrimSpace(acc.AccessToken) != "" {
		if !acc.TokenExpiry.IsZero() && time.Until(acc.TokenExpiry) > 0 {
			expiryMs = acc.TokenExpiry.UnixMilli()
		} else {
			expiryMs = time.Now().Add(1 * time.Hour).UnixMilli()
		}
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
	return writeAtomic(targetPath, data, 0600)
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

// syncSingleVscdb updates profileUrl, userStatus, and oauthToken in a target state.vscdb SQLite storage.
func syncSingleVscdb(vscdbPath string, acc *Account) error {
	if !isSafeTestWritePath(vscdbPath) {
		return nil
	}
	if _, err := os.Stat(vscdbPath); err != nil {
		return nil // state.vscdb not present on host
	}

	db, err := sql.Open("sqlite", vscdbPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", vscdbPath, err)
	}
	defer db.Close()

	// Configure busy timeout to handle concurrent access by Antigravity / Code process
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")

	var tableCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='ItemTable'").Scan(&tableCount)
	if err != nil || tableCount == 0 {
		return nil
	}

	execWithRetry := func(query string, args ...interface{}) error {
		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			_, lastErr = db.Exec(query, args...)
			if lastErr == nil {
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}
		return lastErr
	}

	// 1. Update userStatus if email is set
	if acc.Email != "" {
		sentinel := buildUserStatusSentinel(acc.Email)
		if sentinel != "" {
			if err := execWithRetry("INSERT OR REPLACE INTO ItemTable(key, value) VALUES('antigravityUnifiedStateSync.userStatus', ?)", sentinel); err != nil {
				return fmt.Errorf("failed to update userStatus in %s: %w", vscdbPath, err)
			}
		}
	}

	// 2. Update profileUrl if present
	picture := extractPictureFromIDToken(acc.IDToken)
	if picture != "" {
		if err := execWithRetry("INSERT OR REPLACE INTO ItemTable(key, value) VALUES('antigravity.profileUrl', ?)", picture); err != nil {
			return fmt.Errorf("failed to update profileUrl in %s: %w", vscdbPath, err)
		}
	}

	// 3. Update oauthToken to prevent UI-session hydration mismatch on relaunch
	oauthSentinel := buildOAuthTokenSentinel(acc)
	if oauthSentinel != "" {
		if err := execWithRetry("INSERT OR REPLACE INTO ItemTable(key, value) VALUES('antigravityUnifiedStateSync.oauthToken', ?)", oauthSentinel); err != nil {
			return fmt.Errorf("failed to update oauthToken in %s: %w", vscdbPath, err)
		}
	}

	return nil
}

// SyncStateVscdb updates profileUrl, userStatus, and oauthToken in Antigravity's
// and VS Code's state.vscdb SQLite storage if present.
func SyncStateVscdb(acc *Account) error {
	if acc == nil || isTestMockEmail(acc.Email) {
		return nil
	}

	var errs []string
	// 1. Antigravity state.vscdb
	vscdbPathAntigravity := filepath.Join(core.GetAntigravityHostConfigDir(), "User", "globalStorage", "state.vscdb")
	if err := syncSingleVscdb(vscdbPathAntigravity, acc); err != nil {
		errs = append(errs, err.Error())
	}

	// 2. VS Code state.vscdb (if present)
	vscdbPathCode := filepath.Join(core.GetVSCodeHostConfigDir(), "User", "globalStorage", "state.vscdb")
	if err := syncSingleVscdb(vscdbPathCode, acc); err != nil {
		errs = append(errs, err.Error())
	}
	_ = core.EnsureVSCodeWindowCloseGuard()

	if len(errs) > 0 {
		return fmt.Errorf("vscdb sync errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

type journalAction struct {
	description string
	rollback    func() error
}

type syncJournal struct {
	mu           sync.Mutex
	actions      []journalAction
	trackedFiles map[string]bool
}

func newSyncJournal() *syncJournal {
	return &syncJournal{
		actions:      make([]journalAction, 0),
		trackedFiles: make(map[string]bool),
	}
}

func getUserHomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

var syncStepHookForTest func(step string) error

func (j *syncJournal) RecordFile(targetPath string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	cleanPath := filepath.Clean(targetPath)
	if j.trackedFiles[cleanPath] {
		return nil
	}
	j.trackedFiles[cleanPath] = true

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			j.actions = append(j.actions, journalAction{
				description: fmt.Sprintf("delete newly created %s", cleanPath),
				rollback: func() error {
					if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
						return err
					}
					return nil
				},
			})
			return nil
		}
		return err
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()

	j.actions = append(j.actions, journalAction{
		description: fmt.Sprintf("restore file %s", cleanPath),
		rollback: func() error {
			return writeAtomic(cleanPath, data, perm)
		},
	})
	return nil
}

func (j *syncJournal) RecordVscdbKeys(vscdbPath string, keys []string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	cleanPath := filepath.Clean(vscdbPath)
	if _, err := os.Stat(cleanPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	db, err := sql.Open("sqlite", cleanPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()

	var tableCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='ItemTable'").Scan(&tableCount)
	if err != nil || tableCount == 0 {
		return nil
	}

	type keyBackup struct {
		key     string
		existed bool
		val     string
	}
	var backups []keyBackup
	for _, k := range keys {
		var val string
		err := db.QueryRow("SELECT value FROM ItemTable WHERE key = ?", k).Scan(&val)
		if err == nil {
			backups = append(backups, keyBackup{key: k, existed: true, val: val})
		} else if err == sql.ErrNoRows {
			backups = append(backups, keyBackup{key: k, existed: false})
		}
	}

	j.actions = append(j.actions, journalAction{
		description: fmt.Sprintf("restore vscdb %s keys", cleanPath),
		rollback: func() error {
			rDb, rErr := sql.Open("sqlite", cleanPath+"?_busy_timeout=5000")
			if rErr != nil {
				return rErr
			}
			defer rDb.Close()

			for _, b := range backups {
				if b.existed {
					if _, err := rDb.Exec("INSERT OR REPLACE INTO ItemTable(key, value) VALUES(?, ?)", b.key, b.val); err != nil {
						return err
					}
				} else {
					if _, err := rDb.Exec("DELETE FROM ItemTable WHERE key = ?", b.key); err != nil {
						return err
					}
				}
			}
			return nil
		},
	})
	return nil
}

func (j *syncJournal) RecordCloudAccountsActive(homeDir string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	dbPath := filepath.Join(homeDir, ".antigravity-agent", "cloud_accounts.db")
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()

	var prevActive string
	err = db.QueryRow("SELECT email FROM accounts WHERE is_active = 1 LIMIT 1").Scan(&prevActive)
	hadActive := (err == nil && prevActive != "")

	j.actions = append(j.actions, journalAction{
		description: "restore cloud_accounts.db active account",
		rollback: func() error {
			rDb, rErr := sql.Open("sqlite", dbPath+"?_busy_timeout=5000")
			if rErr != nil {
				return rErr
			}
			defer rDb.Close()

			if hadActive {
				_, err := rDb.Exec("UPDATE accounts SET is_active = CASE WHEN LOWER(email) = LOWER(?) THEN 1 ELSE 0 END", prevActive)
				return err
			}
			_, err := rDb.Exec("UPDATE accounts SET is_active = 0")
			return err
		},
	})
	return nil
}

func (j *syncJournal) Rollback() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	var errs []string
	for i := len(j.actions) - 1; i >= 0; i-- {
		action := j.actions[i]
		if err := action.rollback(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", action.description, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("surface rollback failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SyncAllSurfaces atomically syncs the active account across Antigravity 2.0 Desktop,
// Antigravity CLI (agy), and Antigravity VS Code Extension with transactional staging and LIFO rollback.
func SyncAllSurfaces(acc *Account, allEmails []string, profileMgr *fingerprint.Store) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}

	// Ensure access token is fresh before writing across all Antigravity surfaces
	_ = EnsureFreshAccessToken(acc)

	// Pre-validate secret payload construction in memory before touching any surfaces
	if _, err := buildSecretPayload(acc); err != nil {
		return fmt.Errorf("staging failed: invalid account payload: %w", err)
	}

	journal := newSyncJournal()
	fail := func(stepName string, err error) error {
		if rbErr := journal.Rollback(); rbErr != nil {
			return fmt.Errorf("surface sync failed at %s: %w (rollback error: %v)", stepName, err, rbErr)
		}
		return fmt.Errorf("surface sync failed at %s: %w", stepName, err)
	}

	home := getUserHomeDir()

	// 1. Linux Secret Service (best-effort)
	_ = WriteSecretServiceToken(acc)

	if syncStepHookForTest != nil {
		if err := syncStepHookForTest("pre-sync"); err != nil {
			return fail("pre-sync hook", err)
		}
	}

	// 2. Antigravity 2.0 Desktop App & VS Code Extension Token
	if home != "" {
		_ = journal.RecordFile(filepath.Join(home, ".gemini", "jetski-standalone-oauth-token"))
	}
	if err := SyncDesktopStandaloneToken(acc); err != nil {
		return fail("desktop token", err)
	}
	if syncStepHookForTest != nil {
		if err := syncStepHookForTest("desktop token"); err != nil {
			return fail("desktop token hook", err)
		}
	}

	// 3. App Storage Login User
	_ = journal.RecordFile(filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json"))
	if err := SyncAppStorageLoginUser(acc.Email); err != nil {
		return fail("app_storage", err)
	}
	if syncStepHookForTest != nil {
		if err := syncStepHookForTest("app_storage"); err != nil {
			return fail("app_storage hook", err)
		}
	}

	// 4. Hardware Profile
	antigravityConfigDir := core.GetAntigravityHostConfigDir()
	geminiAntigravityDir := core.GetAntigravityDir()
	_ = journal.RecordFile(filepath.Join(antigravityConfigDir, "machineid"))
	_ = journal.RecordFile(filepath.Join(antigravityConfigDir, ".updaterId"))
	_ = journal.RecordFile(filepath.Join(geminiAntigravityDir, "installation_id"))
	_ = journal.RecordFile(filepath.Join(geminiAntigravityDir, "antigravity_state.pbtxt"))
	if err := SyncHardwareProfile(acc.Email, profileMgr); err != nil {
		return fail("hardware profile", err)
	}
	if syncStepHookForTest != nil {
		if err := syncStepHookForTest("hardware profile"); err != nil {
			return fail("hardware profile hook", err)
		}
	}

	// 5. State vscdb SQLite
	vscdbKeys := []string{
		"antigravityUnifiedStateSync.userStatus",
		"antigravity.profileUrl",
		"antigravityUnifiedStateSync.oauthToken",
	}
	_ = journal.RecordVscdbKeys(filepath.Join(core.GetAntigravityHostConfigDir(), "User", "globalStorage", "state.vscdb"), vscdbKeys)
	_ = journal.RecordVscdbKeys(filepath.Join(core.GetVSCodeHostConfigDir(), "User", "globalStorage", "state.vscdb"), vscdbKeys)
	if err := SyncStateVscdb(acc); err != nil {
		return fail("state.vscdb", err)
	}
	if syncStepHookForTest != nil {
		if err := syncStepHookForTest("state.vscdb"); err != nil {
			return fail("state.vscdb hook", err)
		}
	}

	// 6. Antigravity CLI (agy)
	if home != "" {
		_ = journal.RecordFile(filepath.Join(home, ".gemini", "antigravity-cli", "antigravity-oauth-token"))
	}
	if err := SyncCLIOAuthToken(acc); err != nil {
		return fail("cli oauth token", err)
	}
	if syncStepHookForTest != nil {
		if err := syncStepHookForTest("cli oauth token"); err != nil {
			return fail("cli oauth token hook", err)
		}
	}

	// 7. Google Accounts JSON
	if home != "" {
		_ = journal.RecordFile(filepath.Join(home, ".gemini", "google_accounts.json"))
	}
	if err := SyncGoogleAccountsJSON(acc.Email, allEmails); err != nil {
		return fail("google_accounts.json", err)
	}
	if syncStepHookForTest != nil {
		if err := syncStepHookForTest("google_accounts.json"); err != nil {
			return fail("google_accounts.json hook", err)
		}
	}

	// 8. OAuth Creds JSON
	if home != "" {
		_ = journal.RecordFile(filepath.Join(home, ".gemini", "oauth_creds.json"))
	}
	if err := SyncOAuthCredsJSON(acc); err != nil {
		return fail("oauth_creds.json", err)
	}
	if syncStepHookForTest != nil {
		if err := syncStepHookForTest("oauth_creds.json"); err != nil {
			return fail("oauth_creds.json hook", err)
		}
	}

	// 9. Cloud Accounts DB
	if home != "" {
		_ = journal.RecordCloudAccountsActive(home)
	}
	if err := SyncCloudAccountsActiveAccount("", acc.Email); err != nil {
		return fail("cloud_accounts.db", err)
	}
	if syncStepHookForTest != nil {
		if err := syncStepHookForTest("cloud_accounts.db"); err != nil {
			return fail("cloud_accounts.db hook", err)
		}
	}

	return nil
}

// SyncDesktopSurface synchronizes credentials for Antigravity 2.0 Desktop only.
func SyncDesktopSurface(acc *Account, profileMgr *fingerprint.Store, allEmails ...[]string) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}
	_ = EnsureFreshAccessToken(acc)
	_ = WriteSecretServiceToken(acc)

	var errs []string
	if err := SyncDesktopStandaloneToken(acc); err != nil {
		errs = append(errs, fmt.Sprintf("desktop token: %v", err))
	}
	if err := SyncAppStorageLoginUser(acc.Email); err != nil {
		errs = append(errs, fmt.Sprintf("app_storage: %v", err))
	}
	if err := SyncHardwareProfile(acc.Email, profileMgr); err != nil {
		errs = append(errs, fmt.Sprintf("hardware profile: %v", err))
	}
	if err := SyncStateVscdb(acc); err != nil {
		errs = append(errs, fmt.Sprintf("state.vscdb: %v", err))
	}
	var emails []string
	if len(allEmails) > 0 {
		emails = allEmails[0]
	}
	if err := SyncGoogleAccountsJSON(acc.Email, emails); err != nil {
		errs = append(errs, fmt.Sprintf("google_accounts.json: %v", err))
	}
	if err := SyncOAuthCredsJSON(acc); err != nil {
		errs = append(errs, fmt.Sprintf("oauth_creds.json: %v", err))
	}
	_ = SyncCloudAccountsActiveAccount("", acc.Email)

	if len(errs) > 0 {
		return fmt.Errorf("desktop surface sync failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SyncCLISurface synchronizes credentials for Antigravity CLI (agy) only.
func SyncCLISurface(acc *Account, allEmails []string) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}
	_ = EnsureFreshAccessToken(acc)
	_ = WriteSecretServiceToken(acc)

	var errs []string
	if err := SyncDesktopStandaloneToken(acc); err != nil {
		errs = append(errs, fmt.Sprintf("desktop token: %v", err))
	}
	if err := SyncCLIOAuthToken(acc); err != nil {
		errs = append(errs, fmt.Sprintf("cli oauth token: %v", err))
	}
	if err := SyncGoogleAccountsJSON(acc.Email, allEmails); err != nil {
		errs = append(errs, fmt.Sprintf("google_accounts.json: %v", err))
	}
	if err := SyncOAuthCredsJSON(acc); err != nil {
		errs = append(errs, fmt.Sprintf("oauth_creds.json: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("cli surface sync failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SyncVSCodeSurface synchronizes credentials for VS Code Extension only.
func SyncVSCodeSurface(acc *Account) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}
	_ = EnsureFreshAccessToken(acc)
	_ = WriteSecretServiceToken(acc)

	var errs []string
	if err := SyncDesktopStandaloneToken(acc); err != nil {
		errs = append(errs, fmt.Sprintf("desktop token: %v", err))
	}
	if err := SyncStateVscdb(acc); err != nil {
		errs = append(errs, fmt.Sprintf("state.vscdb: %v", err))
	}
	if err := SyncCLIOAuthToken(acc); err != nil {
		errs = append(errs, fmt.Sprintf("cli oauth token: %v", err))
	}
	if err := SyncOAuthCredsJSON(acc); err != nil {
		errs = append(errs, fmt.Sprintf("oauth_creds.json: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("vscode surface sync failures: %s", strings.Join(errs, "; "))
	}
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
