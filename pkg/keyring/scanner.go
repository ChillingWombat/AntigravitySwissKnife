package keyring

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

// Scanner discovers accounts and credentials stored across the local system.
type Scanner struct {
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
		antigravityConfigDir: configDir,
		store:                store,
	}
}

// Scan discovers accounts across:
// 1. Antigravity IDE local app_storage.json (active session)
// 2. Linux Secret Service (service=gemini, username=antigravity)
// 3. Swiss Knife accounts vault (~/.config/antigravity-swiss/accounts.json)
func (s *Scanner) Scan() ([]DiscoveredAccount, error) {
	discovered := make(map[string]*DiscoveredAccount)

	// 1. Check existing accounts in Swiss Knife vault
	if s.store != nil {
		for _, acc := range s.store.ListAccounts() {
			discovered[acc.Email] = &DiscoveredAccount{
				Email:          acc.Email,
				Source:         "Swiss Knife Vault",
				HasTokens:      acc.AccessToken != "" || acc.RefreshToken != "",
				HasRefresh:     acc.RefreshToken != "",
				AccessToken:    acc.AccessToken,
				RefreshToken:   acc.RefreshToken,
				AlreadyInVault: true,
			}
		}
	}

	// 2. Scan Antigravity app_storage.json
	appStoragePath := filepath.Join(s.antigravityConfigDir, "app_storage.json")
	if data, err := os.ReadFile(appStoragePath); err == nil {
		var rawMap map[string]interface{}
		if err := json.Unmarshal(data, &rawMap); err == nil {
			if loginUser, ok := rawMap["jetski.onboarding.lastLoginUsername"].(string); ok && loginUser != "" {
				disc, exists := discovered[loginUser]
				if !exists {
					disc = &DiscoveredAccount{
						Email:  loginUser,
						Source: "Antigravity IDE (Active Session)",
					}
					discovered[loginUser] = disc
				} else {
					disc.Source = disc.Source + " + Antigravity IDE (Active)"
				}
				disc.IsActiveInIDE = true
			}
		}
	}

	// 3. Scan Linux Secret Service / Keyring via secret-tool
	if cred, err := readSecretServiceToken(); err == nil && cred != nil {
		// If we found credentials in the keyring, check which account it belongs to
		// Match against active IDE email or store active email
		targetEmail := ""
		for _, d := range discovered {
			if d.IsActiveInIDE {
				targetEmail = d.Email
				break
			}
		}
		if targetEmail == "" && s.store != nil {
			targetEmail = s.store.ActiveAccount()
		}
		if targetEmail != "" {
			disc := discovered[targetEmail]
			if disc != nil {
				disc.HasTokens = true
				if cred.RefreshToken != "" {
					disc.HasRefresh = true
					disc.RefreshToken = cred.RefreshToken
				}
				if cred.AccessToken != "" {
					disc.AccessToken = cred.AccessToken
				}
				disc.Source = disc.Source + " + Linux Keyring"
			}
		}
	}

	result := make([]DiscoveredAccount, 0, len(discovered))
	for _, d := range discovered {
		result = append(result, *d)
	}
	return result, nil
}

type secretServicePayload struct {
	Token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		Expiry       string `json:"expiry"`
	} `json:"token"`
	AuthMethod string `json:"auth_method"`
}

func readSecretServiceToken() (*struct{ AccessToken, RefreshToken string }, error) {
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

	return &struct{ AccessToken, RefreshToken string }{
		AccessToken:  payload.Token.AccessToken,
		RefreshToken: payload.Token.RefreshToken,
	}, nil
}
