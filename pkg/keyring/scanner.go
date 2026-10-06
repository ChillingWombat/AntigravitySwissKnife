package keyring

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DiscoveredAccount represents an account found by scanning the local host environment.
type DiscoveredAccount struct {
	Email          string `json:"email"`
	Source         string `json:"source"`
	HasTokens      bool   `json:"has_tokens"`
	HasRefresh     bool   `json:"has_refresh"`
	AccessToken    string `json:"access_token,omitempty"`
	RefreshToken   string `json:"refresh_token,omitempty"`
	IsActiveInIDE  bool   `json:"is_active_in_ide"`
	AlreadyInVault bool   `json:"already_in_vault"`
}

const (
	SurfaceDesktop     = "desktop"
	SurfaceDesktopName = "Antigravity 2.0 Desktop"
	SurfaceVSCode      = "vscode"
	SurfaceVSCodeName  = "Antigravity VS Code Extension"
	SurfaceCLI         = "cli"
	SurfaceCLIName     = "Antigravity CLI"
)

// DetectedSurfaceAccount represents an active session account detected on a specific Antigravity surface.
type DetectedSurfaceAccount struct {
	Email        string `json:"email"`
	Surface      string `json:"surface"`      // "desktop", "vscode", "cli"
	SurfaceName  string `json:"surface_name"` // "Antigravity 2.0 Desktop", etc.
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// Scanner discovers accounts and credentials stored across the local system.
type Scanner struct {
	homeDir              string
	antigravityConfigDir string
	store                *Store
}

// NewScanner creates a new local machine account scanner.
func NewScanner(store *Store) *Scanner {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "Antigravity")
	if custom := os.Getenv("ANTIGRAVITY_CONFIG_DIR"); custom != "" {
		configDir = custom
	}
	return &Scanner{
		homeDir:              home,
		antigravityConfigDir: configDir,
		store:                store,
	}
}

// normalizeEmail canonicalizes email string for deduplication.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// isTestMockEmail filters out unit test fixtures and artificial mock accounts.
func isTestMockEmail(email string) bool {
	norm := normalizeEmail(email)
	if norm == "" {
		return true
	}
	if norm == "david.alt@google.com" || norm == "david.dev@google.com" {
		return true
	}
	if strings.HasSuffix(norm, "@example.com") || strings.HasSuffix(norm, ".test") {
		return true
	}
	return false
}

// parseIDTokenEmail extracts the email claim from a Google OpenID Connect JWT id_token.
func parseIDTokenEmail(idToken string) string {
	parts := strings.Split(strings.TrimSpace(idToken), ".")
	if len(parts) < 2 {
		return ""
	}
	payloadSegment := parts[1]
	// Pad base64 URL string if needed
	if rem := len(payloadSegment) % 4; rem > 0 {
		payloadSegment += strings.Repeat("=", 4-rem)
	}

	data, err := base64.URLEncoding.DecodeString(payloadSegment)
	if err != nil {
		data, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}

	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(data, &claims); err == nil && claims.Email != "" {
		return strings.TrimSpace(claims.Email)
	}
	return ""
}

// DetectAllSurfaces inspects the host environment and returns the detected accounts on each Antigravity surface.
func DetectAllSurfaces(homeDir, configDir string) map[string]*DetectedSurfaceAccount {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	if configDir == "" {
		configDir = filepath.Join(homeDir, ".config", "Antigravity")
		if custom := os.Getenv("ANTIGRAVITY_CONFIG_DIR"); custom != "" {
			configDir = custom
		}
	}

	results := make(map[string]*DetectedSurfaceAccount)

	// 1. Antigravity 2.0 Desktop:
	// Priority 1a: Antigravity app_storage.json (the definitive session configured in Desktop UI)
	appStoragePath := filepath.Join(configDir, "app_storage.json")
	if data, err := os.ReadFile(appStoragePath); err == nil {
		var rawMap map[string]interface{}
		if err := json.Unmarshal(data, &rawMap); err == nil {
			if loginUser, ok := rawMap["jetski.onboarding.lastLoginUsername"].(string); ok {
				loginUser = strings.TrimSpace(loginUser)
				if loginUser != "" && !isTestMockEmail(loginUser) {
					results[SurfaceDesktop] = &DetectedSurfaceAccount{
						Email:       normalizeEmail(loginUser),
						Surface:     SurfaceDesktop,
						SurfaceName: SurfaceDesktopName,
					}
				}
			}
		}
	}

	// Priority 1b: Standalone OAuth Token (~/.gemini/jetski-standalone-oauth-token)
	jetskiTokenPath := filepath.Join(homeDir, ".gemini", "jetski-standalone-oauth-token")
	if data, err := os.ReadFile(jetskiTokenPath); err == nil {
		var payload secretServicePayload
		if err := json.Unmarshal(data, &payload); err == nil {
			email := parseIDTokenEmail(payload.Token.IDToken)
			if email != "" && !isTestMockEmail(email) {
				normEmail := normalizeEmail(email)
				if results[SurfaceDesktop] != nil {
					// If app_storage already identified the desktop user, attach tokens if matching
					if strings.EqualFold(results[SurfaceDesktop].Email, normEmail) {
						results[SurfaceDesktop].AccessToken = payload.Token.AccessToken
						results[SurfaceDesktop].RefreshToken = payload.Token.RefreshToken
						results[SurfaceDesktop].IDToken = payload.Token.IDToken
					}
				} else {
					results[SurfaceDesktop] = &DetectedSurfaceAccount{
						Email:        normEmail,
						Surface:      SurfaceDesktop,
						SurfaceName:  SurfaceDesktopName,
						AccessToken:  payload.Token.AccessToken,
						RefreshToken: payload.Token.RefreshToken,
						IDToken:      payload.Token.IDToken,
					}
				}
			}
		}
	}

	// 2. Antigravity VS Code Extension: Linux Secret Service (service=gemini, username=antigravity)
	if cred, err := readSecretServiceToken(); err == nil && cred != nil {
		email := parseIDTokenEmail(cred.IDToken)
		if email != "" && !isTestMockEmail(email) {
			results[SurfaceVSCode] = &DetectedSurfaceAccount{
				Email:        normalizeEmail(email),
				Surface:      SurfaceVSCode,
				SurfaceName:  SurfaceVSCodeName,
				AccessToken:  cred.AccessToken,
				RefreshToken: cred.RefreshToken,
				IDToken:      cred.IDToken,
			}
		}
	}

	// 3. Antigravity CLI (agy):
	// Priority 3a: Antigravity CLI OAuth Token (~/.gemini/antigravity-cli/antigravity-oauth-token)
	cliTokenPath := filepath.Join(homeDir, ".gemini", "antigravity-cli", "antigravity-oauth-token")
	if data, err := os.ReadFile(cliTokenPath); err == nil {
		var payload secretServicePayload
		if err := json.Unmarshal(data, &payload); err == nil {
			email := parseIDTokenEmail(payload.Token.IDToken)
			if email != "" && !isTestMockEmail(email) {
				results[SurfaceCLI] = &DetectedSurfaceAccount{
					Email:        normalizeEmail(email),
					Surface:      SurfaceCLI,
					SurfaceName:  SurfaceCLIName,
					AccessToken:  payload.Token.AccessToken,
					RefreshToken: payload.Token.RefreshToken,
					IDToken:      payload.Token.IDToken,
				}
			}
		}
	}
	// Priority 3b: Gemini CLI Credentials (~/.gemini/oauth_creds.json)
	if results[SurfaceCLI] == nil {
		oauthCredsPath := filepath.Join(homeDir, ".gemini", "oauth_creds.json")
		if data, err := os.ReadFile(oauthCredsPath); err == nil {
			var creds struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
				IDToken      string `json:"id_token"`
			}
			if err := json.Unmarshal(data, &creds); err == nil {
				email := parseIDTokenEmail(creds.IDToken)
				if email != "" && !isTestMockEmail(email) {
					results[SurfaceCLI] = &DetectedSurfaceAccount{
						Email:        normalizeEmail(email),
						Surface:      SurfaceCLI,
						SurfaceName:  SurfaceCLIName,
						AccessToken:  creds.AccessToken,
						RefreshToken: creds.RefreshToken,
						IDToken:      creds.IDToken,
					}
				}
			}
		}
	}
	// Priority 3c: Gemini Accounts Store (~/.gemini/google_accounts.json)
	if results[SurfaceCLI] == nil {
		googleAccountsPath := filepath.Join(homeDir, ".gemini", "google_accounts.json")
		if data, err := os.ReadFile(googleAccountsPath); err == nil {
			var gAccounts struct {
				Active string `json:"active"`
			}
			if err := json.Unmarshal(data, &gAccounts); err == nil {
				if gAccounts.Active != "" && !isTestMockEmail(gAccounts.Active) {
					results[SurfaceCLI] = &DetectedSurfaceAccount{
						Email:       normalizeEmail(gAccounts.Active),
						Surface:     SurfaceCLI,
						SurfaceName: SurfaceCLIName,
					}
				}
			}
		}
	}

	return results
}

// ResolveRunningAntigravityAccount resolves the single active account running across Antigravity apps
// strictly following the user-defined priority sequence:
// Antigravity 2.0 Desktop > Antigravity VS Code Extension > Antigravity CLI
func ResolveRunningAntigravityAccount(homeDir, configDir string) *DetectedSurfaceAccount {
	surfaces := DetectAllSurfaces(homeDir, configDir)
	// Sequence 1: Antigravity 2.0 Desktop
	if acc, ok := surfaces[SurfaceDesktop]; ok && acc != nil && acc.Email != "" {
		return acc
	}
	// Sequence 2: Antigravity VS Code Extension
	if acc, ok := surfaces[SurfaceVSCode]; ok && acc != nil && acc.Email != "" {
		return acc
	}
	// Sequence 3: Antigravity CLI
	if acc, ok := surfaces[SurfaceCLI]; ok && acc != nil && acc.Email != "" {
		return acc
	}
	return nil
}

// Scan discovers accounts across:
// 1. Swiss Knife accounts vault (~/.config/antigravity-swiss/accounts.json)
// 2. Antigravity 2.0 local app_storage.json (active session)
// 3. Linux Secret Service (service=gemini, username=antigravity)
// 4. Antigravity Desktop Standalone Token (~/.gemini/jetski-standalone-oauth-token)
// 5. Antigravity CLI OAuth Token (~/.gemini/antigravity-cli/antigravity-oauth-token)
// 6. Gemini CLI Credentials (~/.gemini/oauth_creds.json)
// 7. Gemini Accounts Store (~/.gemini/google_accounts.json)
// 8. Google Cloud SDK Credentials (~/.config/gcloud/legacy_credentials/*/adc.json)
// 9. Google Cloud SDK Active Config (~/.config/gcloud/configurations/*)
// 10. Google Cloud SDK Database (~/.config/gcloud/credentials.db)
func (s *Scanner) Scan() ([]DiscoveredAccount, error) {
	discovered := make(map[string]*DiscoveredAccount)

	// Determine current running active account using multi-surface priority sequence:
	// Desktop > VS Code Extension > CLI
	resolvedRunning := ResolveRunningAntigravityAccount(s.homeDir, s.antigravityConfigDir)
	var activeRunningEmail string
	if resolvedRunning != nil {
		activeRunningEmail = normalizeEmail(resolvedRunning.Email)
	}
	// 0. Sync and scan Antigravity Agent Database (~/.antigravity-agent/cloud_accounts.db)
	_ = SyncStoreFromCloudAccountsDB(s.store, s.homeDir)
	if cloudAccs, err := ReadCloudAccountsDB(s.homeDir); err == nil && len(cloudAccs) > 0 {
		for _, ca := range cloudAccs {
			if isTestMockEmail(ca.Email) {
				continue
			}
			key := normalizeEmail(ca.Email)
			disc, exists := discovered[key]
			if !exists {
				disc = &DiscoveredAccount{
					Email:        ca.Email,
					Source:       "Antigravity Agent DB",
					HasTokens:    ca.AccessToken != "" || ca.RefreshToken != "",
					HasRefresh:   ca.RefreshToken != "",
					AccessToken:  ca.AccessToken,
					RefreshToken: ca.RefreshToken,
				}
				discovered[key] = disc
			} else {
				if !strings.Contains(disc.Source, "Agent DB") {
					disc.Source += " + Antigravity Agent DB"
				}
				if ca.RefreshToken != "" {
					disc.HasRefresh = true
					disc.RefreshToken = ca.RefreshToken
				}
				if ca.AccessToken != "" && disc.AccessToken == "" {
					disc.AccessToken = ca.AccessToken
				}
			}
			if ca.IsActive || (activeRunningEmail != "" && key == activeRunningEmail) {
				disc.IsActiveInIDE = true
			}
		}
	}

	// 1. Check existing accounts in Swiss Knife vault
	if s.store != nil {
		for _, acc := range s.store.ListAccounts() {
			if isTestMockEmail(acc.Email) {
				continue
			}
			key := normalizeEmail(acc.Email)
			discovered[key] = &DiscoveredAccount{
				Email:          acc.Email,
				Source:         "Swiss Knife Vault",
				HasTokens:      acc.AccessToken != "" || acc.RefreshToken != "",
				HasRefresh:     acc.RefreshToken != "",
				AccessToken:    acc.AccessToken,
				RefreshToken:   acc.RefreshToken,
				AlreadyInVault: true,
				IsActiveInIDE:  (activeRunningEmail != "" && key == activeRunningEmail),
			}
		}
	}

	// 2. Scan Antigravity app_storage.json for active user
	appStoragePath := filepath.Join(s.antigravityConfigDir, "app_storage.json")
	if data, err := os.ReadFile(appStoragePath); err == nil {
		var rawMap map[string]interface{}
		if err := json.Unmarshal(data, &rawMap); err == nil {
			if loginUser, ok := rawMap["jetski.onboarding.lastLoginUsername"].(string); ok {
				loginUser = strings.TrimSpace(loginUser)
				if loginUser != "" && !isTestMockEmail(loginUser) {
					key := normalizeEmail(loginUser)
					disc, exists := discovered[key]
					if !exists {
						disc = &DiscoveredAccount{
							Email:  loginUser,
							Source: "Antigravity IDE (Active Session)",
						}
						discovered[key] = disc
					} else if !strings.Contains(disc.Source, "Antigravity IDE") {
						disc.Source += " + Antigravity IDE (Active)"
					}
					if activeRunningEmail != "" && key == activeRunningEmail {
						disc.IsActiveInIDE = true
					}
				}
			}
		}
	}

	// 3. Scan Linux Secret Service / Keyring via secret-tool
	if cred, err := readSecretServiceToken(); err == nil && cred != nil {
		emailFromToken := parseIDTokenEmail(cred.IDToken)
		if emailFromToken != "" && !isTestMockEmail(emailFromToken) {
			key := normalizeEmail(emailFromToken)
			disc, exists := discovered[key]
			if !exists {
				disc = &DiscoveredAccount{
					Email:  emailFromToken,
					Source: "Linux Keyring",
				}
				discovered[key] = disc
			} else if !strings.Contains(disc.Source, "Keyring") {
				disc.Source += " + Linux Keyring"
			}
			disc.HasTokens = true
			if cred.RefreshToken != "" {
				disc.HasRefresh = true
				disc.RefreshToken = cred.RefreshToken
			}
			if cred.AccessToken != "" && disc.AccessToken == "" {
				disc.AccessToken = cred.AccessToken
			}
		}
	}

	// 4. Scan Antigravity Desktop Standalone Token
	jetskiTokenPath := filepath.Join(s.homeDir, ".gemini", "jetski-standalone-oauth-token")
	if data, err := os.ReadFile(jetskiTokenPath); err == nil {
		var payload secretServicePayload
		if err := json.Unmarshal(data, &payload); err == nil {
			email := parseIDTokenEmail(payload.Token.IDToken)
			if email != "" && !isTestMockEmail(email) {
				key := normalizeEmail(email)
				disc, exists := discovered[key]
				if !exists {
					disc = &DiscoveredAccount{
						Email:  email,
						Source: "Antigravity Desktop Token",
					}
					discovered[key] = disc
				} else if !strings.Contains(disc.Source, "Desktop Token") {
					disc.Source += " + Desktop Token"
				}
				disc.HasTokens = true
				if payload.Token.RefreshToken != "" {
					disc.HasRefresh = true
					disc.RefreshToken = payload.Token.RefreshToken
				}
				if payload.Token.AccessToken != "" && disc.AccessToken == "" {
					disc.AccessToken = payload.Token.AccessToken
				}
			}
		}
	}

	// 5. Scan Antigravity CLI OAuth Token
	cliTokenPath := filepath.Join(s.homeDir, ".gemini", "antigravity-cli", "antigravity-oauth-token")
	if data, err := os.ReadFile(cliTokenPath); err == nil {
		var payload secretServicePayload
		if err := json.Unmarshal(data, &payload); err == nil {
			email := parseIDTokenEmail(payload.Token.IDToken)
			if email != "" && !isTestMockEmail(email) {
				key := normalizeEmail(email)
				disc, exists := discovered[key]
				if !exists {
					disc = &DiscoveredAccount{
						Email:  email,
						Source: "Antigravity CLI Token",
					}
					discovered[key] = disc
				} else if !strings.Contains(disc.Source, "CLI Token") {
					disc.Source += " + Antigravity CLI Token"
				}
				disc.HasTokens = true
				if payload.Token.RefreshToken != "" {
					disc.HasRefresh = true
					disc.RefreshToken = payload.Token.RefreshToken
				}
				if payload.Token.AccessToken != "" && disc.AccessToken == "" {
					disc.AccessToken = payload.Token.AccessToken
				}
			}
		}
	}

	// 6. Scan Gemini CLI Credentials (oauth_creds.json)
	oauthCredsPath := filepath.Join(s.homeDir, ".gemini", "oauth_creds.json")
	if data, err := os.ReadFile(oauthCredsPath); err == nil {
		var creds struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			IDToken      string `json:"id_token"`
		}
		if err := json.Unmarshal(data, &creds); err == nil {
			email := parseIDTokenEmail(creds.IDToken)
			if email != "" && !isTestMockEmail(email) {
				key := normalizeEmail(email)
				disc, exists := discovered[key]
				if !exists {
					disc = &DiscoveredAccount{
						Email:  email,
						Source: "Gemini CLI Credentials",
					}
					discovered[key] = disc
				} else if !strings.Contains(disc.Source, "Gemini CLI") {
					disc.Source += " + Gemini CLI"
				}
				disc.HasTokens = true
				if creds.RefreshToken != "" {
					disc.HasRefresh = true
					disc.RefreshToken = creds.RefreshToken
				}
				if creds.AccessToken != "" && disc.AccessToken == "" {
					disc.AccessToken = creds.AccessToken
				}
			}
		}
	}

	// 7. Scan Gemini Accounts Store (google_accounts.json)
	googleAccountsPath := filepath.Join(s.homeDir, ".gemini", "google_accounts.json")
	if data, err := os.ReadFile(googleAccountsPath); err == nil {
		var gAccounts struct {
			Active string   `json:"active"`
			Old    []string `json:"old"`
		}
		if err := json.Unmarshal(data, &gAccounts); err == nil {
			if gAccounts.Active != "" && !isTestMockEmail(gAccounts.Active) {
				key := normalizeEmail(gAccounts.Active)
				disc, exists := discovered[key]
				if !exists {
					disc = &DiscoveredAccount{
						Email:  gAccounts.Active,
						Source: "Gemini CLI (Active)",
					}
					discovered[key] = disc
				} else if !strings.Contains(disc.Source, "Gemini CLI") {
					disc.Source += " + Gemini CLI"
				}
			}
			for _, em := range gAccounts.Old {
				em = strings.TrimSpace(em)
				if em != "" && !isTestMockEmail(em) {
					key := normalizeEmail(em)
					disc, exists := discovered[key]
					if !exists {
						disc = &DiscoveredAccount{
							Email:  em,
							Source: "Gemini CLI (Saved)",
						}
						discovered[key] = disc
					} else if !strings.Contains(disc.Source, "Gemini CLI") {
						disc.Source += " + Gemini CLI"
					}
				}
			}
		}
	}

	// 8. Scan Google Cloud SDK Legacy Credentials (~/.config/gcloud/legacy_credentials/*/adc.json)
	gcloudLegacyDir := filepath.Join(s.homeDir, ".config", "gcloud", "legacy_credentials")
	if entries, err := os.ReadDir(gcloudLegacyDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				candidateEmail := strings.TrimSpace(entry.Name())
				if strings.Contains(candidateEmail, "@") && !isTestMockEmail(candidateEmail) {
					adcPath := filepath.Join(gcloudLegacyDir, entry.Name(), "adc.json")
					var refreshToken string
					if adcData, err := os.ReadFile(adcPath); err == nil {
						var adc struct {
							RefreshToken string `json:"refresh_token"`
						}
						if err := json.Unmarshal(adcData, &adc); err == nil {
							refreshToken = adc.RefreshToken
						}
					}
					key := normalizeEmail(candidateEmail)
					disc, exists := discovered[key]
					if !exists {
						disc = &DiscoveredAccount{
							Email:        candidateEmail,
							Source:       "Google Cloud SDK",
							HasTokens:    refreshToken != "",
							HasRefresh:   refreshToken != "",
							RefreshToken: refreshToken,
						}
						discovered[key] = disc
					} else {
						if !strings.Contains(disc.Source, "Google Cloud SDK") {
							disc.Source += " + Google Cloud SDK"
						}
						if refreshToken != "" && disc.RefreshToken == "" {
							disc.RefreshToken = refreshToken
							disc.HasRefresh = true
							disc.HasTokens = true
						}
					}
				}
			}
		}
	}

	// 9. Scan Google Cloud SDK Configurations (~/.config/gcloud/configurations/*)
	gcloudConfigDir := filepath.Join(s.homeDir, ".config", "gcloud", "configurations")
	if entries, err := os.ReadDir(gcloudConfigDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				cfgPath := filepath.Join(gcloudConfigDir, entry.Name())
				if content, err := os.ReadFile(cfgPath); err == nil {
					lines := strings.Split(string(content), "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if strings.HasPrefix(line, "account") && strings.Contains(line, "=") {
							parts := strings.SplitN(line, "=", 2)
							em := strings.TrimSpace(parts[1])
							if strings.Contains(em, "@") && !isTestMockEmail(em) {
								key := normalizeEmail(em)
								disc, exists := discovered[key]
								if !exists {
									disc = &DiscoveredAccount{
										Email:  em,
										Source: "Google Cloud SDK (" + entry.Name() + ")",
									}
									discovered[key] = disc
								} else if !strings.Contains(disc.Source, "Google Cloud SDK") {
									disc.Source += " + Google Cloud SDK"
								}
							}
						}
					}
				}
			}
		}
	}

	// 10. Scan Google Cloud SDK SQLite Database (~/.config/gcloud/credentials.db)
	gcloudDbPath := filepath.Join(s.homeDir, ".config", "gcloud", "credentials.db")
	if data, err := os.ReadFile(gcloudDbPath); err == nil {
		emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
		matches := emailRegex.FindAll(data, -1)
		for _, m := range matches {
			em := string(m)
			if !isTestMockEmail(em) {
				key := normalizeEmail(em)
				disc, exists := discovered[key]
				if !exists {
					disc = &DiscoveredAccount{
						Email:  em,
						Source: "Google Cloud SDK",
					}
					discovered[key] = disc
				} else if !strings.Contains(disc.Source, "Google Cloud SDK") {
					disc.Source += " + Google Cloud SDK"
				}
			}
		}
	}
	// Ensure IsActiveInIDE strictly matches the resolved winning active running session
	for key, disc := range discovered {
		disc.IsActiveInIDE = (activeRunningEmail != "" && key == activeRunningEmail)
	}

	result := make([]DiscoveredAccount, 0, len(discovered))
	for _, d := range discovered {
		result = append(result, *d)
	}

	// Sort results: active accounts first, unimported accounts next, then alphabetical
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsActiveInIDE != result[j].IsActiveInIDE {
			return result[i].IsActiveInIDE
		}
		if result[i].AlreadyInVault != result[j].AlreadyInVault {
			return !result[i].AlreadyInVault // unimported first
		}
		return result[i].Email < result[j].Email
	})

	return result, nil
}

type secretServicePayload struct {
	Token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		Expiry       string `json:"expiry"`
		IDToken      string `json:"id_token,omitempty"`
	} `json:"token"`
	AuthMethod string `json:"auth_method"`
}

type secretTokenData struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
}

func readSecretServiceToken() (*secretTokenData, error) {
	if os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" {
		return nil, nil
	}
	cmd := exec.Command("secret-tool", "lookup", "service", "gemini", "username", "antigravity")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	var payload secretServicePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}

	return &secretTokenData{
		AccessToken:  payload.Token.AccessToken,
		RefreshToken: payload.Token.RefreshToken,
		IDToken:      payload.Token.IDToken,
	}, nil
}
