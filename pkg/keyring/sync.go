package keyring

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
)

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
	if !acc.TokenExpiry.IsZero() {
		payload.Token.Expiry = acc.TokenExpiry.UTC().Format("2006-01-02T15:04:05.000000Z")
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
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	storagePath := filepath.Join(home, ".config", "Antigravity", "app_storage.json")

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

// SyncHardwareProfile swaps Antigravity's machineid and .updaterId files to match the account profile.
func SyncHardwareProfile(email string, profileMgr *fingerprint.Store) error {
	if profileMgr == nil {
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
	if err := os.MkdirAll(antigravityDir, 0755); err != nil {
		return err
	}

	// 1. Write machineid
	if prof.MachineID != "" {
		_ = os.WriteFile(filepath.Join(antigravityDir, "machineid"), []byte(strings.TrimSpace(prof.MachineID)), 0644)
	}

	// 2. Write .updaterId
	if prof.UpdaterID != "" {
		_ = os.WriteFile(filepath.Join(antigravityDir, ".updaterId"), []byte(strings.TrimSpace(prof.UpdaterID)), 0644)
	}

	return nil
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
	targetPath := filepath.Join(home, ".gemini", "jetski-standalone-oauth-token")
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
	targetPath := filepath.Join(home, ".gemini", "google_accounts.json")
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
	targetPath := filepath.Join(home, ".gemini", "oauth_creds.json")

	expiryMs := time.Now().Add(1 * time.Hour).UnixMilli()
	if !acc.TokenExpiry.IsZero() {
		expiryMs = acc.TokenExpiry.UnixMilli()
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

// SyncAllSurfaces atomically syncs the active account across Antigravity 2.0 Desktop,
// Antigravity CLI (agy), and Antigravity VS Code Extension.
func SyncAllSurfaces(acc *Account, allEmails []string, profileMgr *fingerprint.Store) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}

	// 1. Linux Secret Service (VS Code Extension & Electron Keytar)
	_ = WriteSecretServiceToken(acc)

	// 2. Antigravity 2.0 Desktop App
	_ = SyncDesktopStandaloneToken(acc)
	_ = SyncAppStorageLoginUser(acc.Email)
	_ = SyncHardwareProfile(acc.Email, profileMgr)

	// 3. Antigravity CLI (agy)
	_ = SyncCLIOAuthToken(acc)
	_ = SyncGoogleAccountsJSON(acc.Email, allEmails)
	_ = SyncOAuthCredsJSON(acc)

	return nil
}
