package keyring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
)

// Store manages account inventory and atomic credentials swapping.
type Store struct {
	accountsPath         string
	homeDir              string
	antigravityConfigDir string
	accounts             map[string]*Account
	activeEmail          string
	lastManualSwitchTime time.Time
	mu                   sync.RWMutex
}

// NewStore initializes a Keyring store at the specified path (or default).
func NewStore(accountsPath string) (*Store, error) {
	if accountsPath == "" {
		accountsPath = filepath.Join(core.GetConfigDir(), "accounts.json")
	}

	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "Antigravity")
	if custom := os.Getenv("ANTIGRAVITY_CONFIG_DIR"); custom != "" {
		configDir = custom
	}

	s := &Store{
		accountsPath:         accountsPath,
		homeDir:              home,
		antigravityConfigDir: configDir,
		accounts:             make(map[string]*Account),
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

type rawAccountItem struct {
	Email                string  `json:"email"`
	Label                string  `json:"label"`
	PlanTier             string  `json:"plan_tier,omitempty"`
	Status               string  `json:"status,omitempty"`
	Priority             string  `json:"priority,omitempty"`
	Notes                string  `json:"notes,omitempty"`
	Password             string  `json:"password,omitempty"`
	IsHealthy            *bool   `json:"is_healthy,omitempty"`
	TOTPSecret           string  `json:"totp_secret"`
	Credits              float64 `json:"credits,omitempty"`
	EnableCreditOverages bool    `json:"enable_credit_overages"`
	AllowClaudeGPT       bool    `json:"allow_claude_gpt"`
	Credential           *struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token,omitempty"`
		Expiry       string `json:"expiry,omitempty"`
	} `json:"credential,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenExpiry  string `json:"token_expiry,omitempty"`
}

func parseTokenExpiry(raw string) time.Time {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.accountsPath)
	if err != nil {
		return err
	}

	type rawFileFormat struct {
		ActiveAccount string          `json:"active_account"`
		Accounts      json.RawMessage `json:"accounts"`
	}

	var ff rawFileFormat
	if err := json.Unmarshal(data, &ff); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.activeEmail = ff.ActiveAccount
	s.accounts = make(map[string]*Account)

	trimmed := bytes.TrimSpace(ff.Accounts)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		// Map format (Python compatibility)
		var accMap map[string]rawAccountItem
		if err := json.Unmarshal(ff.Accounts, &accMap); err == nil {
			for email, item := range accMap {
				em := item.Email
				if em == "" {
					em = email
				}
				isActive := s.activeEmail != "" && strings.EqualFold(em, s.activeEmail)
				status := strings.ToUpper(strings.TrimSpace(item.Status))
				if status == "" {
					if item.IsHealthy != nil && !*item.IsHealthy {
						status = "ERROR"
					} else if isActive {
						status = "ACTIVE"
					} else {
						status = "STANDBY"
					}
				} else if !isActive && status == "ACTIVE" {
					status = "STANDBY"
				} else if isActive && status == "STANDBY" {
					status = "ACTIVE"
				}
				priority := strings.TrimSpace(item.Priority)
				if priority == "" {
					priority = "High"
				}
				password := core.DecryptCredential(item.Password)
				totpSecret := core.DecryptCredential(item.TOTPSecret)
				accessToken := core.DecryptCredential(item.AccessToken)
				refreshToken := core.DecryptCredential(item.RefreshToken)
				idToken := core.DecryptCredential(item.IDToken)
				tokenExpiry := parseTokenExpiry(item.TokenExpiry)
				if item.Credential != nil {
					if accessToken == "" && item.Credential.AccessToken != "" {
						accessToken = core.DecryptCredential(item.Credential.AccessToken)
					}
					if refreshToken == "" && item.Credential.RefreshToken != "" {
						refreshToken = core.DecryptCredential(item.Credential.RefreshToken)
					}
					if idToken == "" && item.Credential.IDToken != "" {
						idToken = core.DecryptCredential(item.Credential.IDToken)
					}
					if tokenExpiry.IsZero() && item.Credential.Expiry != "" {
						tokenExpiry = parseTokenExpiry(item.Credential.Expiry)
					}
				}

				acc := &Account{
					Email:                em,
					Label:                item.Label,
					PlanTier:             item.PlanTier,
					Status:               status,
					Priority:             priority,
					Notes:                item.Notes,
					Password:             password,
					TOTPSecret:           totpSecret,
					HasTOTP:              totpSecret != "",
					IsActive:             isActive,
					AccessToken:          accessToken,
					RefreshToken:         refreshToken,
					IDToken:              idToken,
					TokenExpiry:          tokenExpiry,
					Credits:              item.Credits,
					EnableCreditOverages: item.EnableCreditOverages,
					AllowClaudeGPT:       item.AllowClaudeGPT,
				}
				s.accounts[em] = acc
			}
			return nil
		}
	}

	// Array format fallback
	var accList []rawAccountItem
	if err := json.Unmarshal(ff.Accounts, &accList); err == nil {
		for _, item := range accList {
			isActive := s.activeEmail != "" && strings.EqualFold(item.Email, s.activeEmail)
			status := strings.ToUpper(strings.TrimSpace(item.Status))
			if status == "" {
				if item.IsHealthy != nil && !*item.IsHealthy {
					status = "ERROR"
				} else if isActive {
					status = "ACTIVE"
				} else {
					status = "STANDBY"
				}
			} else if !isActive && status == "ACTIVE" {
				status = "STANDBY"
			} else if isActive && status == "STANDBY" {
				status = "ACTIVE"
			}
			priority := strings.TrimSpace(item.Priority)
			if priority == "" {
				priority = "High"
			}
			password := core.DecryptCredential(item.Password)
			totpSecret := core.DecryptCredential(item.TOTPSecret)
			accessToken := core.DecryptCredential(item.AccessToken)
			refreshToken := core.DecryptCredential(item.RefreshToken)
			idToken := core.DecryptCredential(item.IDToken)
			tokenExpiry := parseTokenExpiry(item.TokenExpiry)
			if item.Credential != nil {
				if accessToken == "" && item.Credential.AccessToken != "" {
					accessToken = core.DecryptCredential(item.Credential.AccessToken)
				}
				if refreshToken == "" && item.Credential.RefreshToken != "" {
					refreshToken = core.DecryptCredential(item.Credential.RefreshToken)
				}
				if idToken == "" && item.Credential.IDToken != "" {
					idToken = core.DecryptCredential(item.Credential.IDToken)
				}
				if tokenExpiry.IsZero() && item.Credential.Expiry != "" {
					tokenExpiry = parseTokenExpiry(item.Credential.Expiry)
				}
			}

			acc := &Account{
				Email:                item.Email,
				Label:                item.Label,
				PlanTier:             item.PlanTier,
				Status:               status,
				Priority:             priority,
				Notes:                item.Notes,
				Password:             password,
				TOTPSecret:           totpSecret,
				HasTOTP:              totpSecret != "",
				IsActive:             isActive,
				AccessToken:          accessToken,
				RefreshToken:         refreshToken,
				IDToken:              idToken,
				TokenExpiry:          tokenExpiry,
				Credits:              item.Credits,
				EnableCreditOverages: item.EnableCreditOverages,
				AllowClaudeGPT:       item.AllowClaudeGPT,
			}
			s.accounts[item.Email] = acc
		}
	}

	if s.activeEmail != "" && s.accounts[s.activeEmail] == nil {
		s.activeEmail = ""
	}
	return nil
}

func (s *Store) save() error {
	dir := filepath.Dir(s.accountsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	type exportedAccount struct {
		Email                string  `json:"email"`
		Label                string  `json:"label"`
		PlanTier             string  `json:"plan_tier,omitempty"`
		Status               string  `json:"status,omitempty"`
		Priority             string  `json:"priority,omitempty"`
		Notes                string  `json:"notes,omitempty"`
		Password             string  `json:"password,omitempty"`
		TOTPSecret           string  `json:"totp_secret"`
		Credits              float64 `json:"credits,omitempty"`
		EnableCreditOverages bool    `json:"enable_credit_overages"`
		AllowClaudeGPT       bool    `json:"allow_claude_gpt"`
		IsHealthy            bool    `json:"is_healthy"`
		TokenExpiry          string  `json:"token_expiry,omitempty"`
		Credential           struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			IDToken      string `json:"id_token,omitempty"`
			Expiry       string `json:"expiry,omitempty"`
			AuthMethod   string `json:"auth_method"`
			TokenType    string `json:"token_type"`
		} `json:"credential"`
	}

	accMap := make(map[string]exportedAccount)
	for _, acc := range s.accounts {
		acc.IsActive = (s.activeEmail != "" && strings.EqualFold(acc.Email, s.activeEmail))
		acc.HasTOTP = (acc.TOTPSecret != "")

		st := strings.ToUpper(strings.TrimSpace(acc.Status))
		if st == "" {
			if acc.IsActive {
				st = "ACTIVE"
			} else {
				st = "STANDBY"
			}
		} else if !acc.IsActive && st == "ACTIVE" {
			st = "STANDBY"
		} else if acc.IsActive && st == "STANDBY" {
			st = "ACTIVE"
		}
		acc.Status = st

		encPassword := core.EncryptCredential(acc.Password)
		encTOTP := core.EncryptCredential(acc.TOTPSecret)
		encAccess := core.EncryptCredential(acc.AccessToken)
		encRefresh := core.EncryptCredential(acc.RefreshToken)
		encIDToken := core.EncryptCredential(acc.IDToken)

		priority := acc.Priority
		if priority == "" {
			priority = "High"
		}

		expiryStr := ""
		if !acc.TokenExpiry.IsZero() {
			expiryStr = acc.TokenExpiry.UTC().Format("2006-01-02T15:04:05.000000Z")
		}

		ea := exportedAccount{
			Email:                acc.Email,
			Label:                acc.Label,
			PlanTier:             acc.PlanTier,
			Status:               st,
			Priority:             priority,
			Notes:                acc.Notes,
			Password:             encPassword,
			TOTPSecret:           encTOTP,
			Credits:              acc.Credits,
			EnableCreditOverages: acc.EnableCreditOverages,
			AllowClaudeGPT:       acc.AllowClaudeGPT,
			IsHealthy:            st != "ERROR" && st != "BANNED",
			TokenExpiry:          expiryStr,
		}
		ea.Credential.AccessToken = encAccess
		ea.Credential.RefreshToken = encRefresh
		ea.Credential.IDToken = encIDToken
		ea.Credential.Expiry = expiryStr
		ea.Credential.AuthMethod = "consumer"
		ea.Credential.TokenType = "Bearer"
		accMap[acc.Email] = ea
	}

	fileContent := map[string]interface{}{
		"version":        1,
		"active_account": s.activeEmail,
		"accounts":       accMap,
	}

	data, err := json.MarshalIndent(fileContent, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.accountsPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.accountsPath)
}

// ListAccounts returns all registered accounts.
func (s *Store) ListAccounts() []*Account {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		copyAcc := *acc
		list = append(list, &copyAcc)
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Email) < strings.ToLower(list[j].Email)
	})
	return list
}

// AccountExport represents exported account data including credentials in JSON.
type AccountExport struct {
	ID                   string  `json:"id"`
	Email                string  `json:"email"`
	Label                string  `json:"label"`
	PlanTier             string  `json:"plan_tier,omitempty"`
	Status               string  `json:"status,omitempty"`
	Priority             string  `json:"priority,omitempty"`
	Notes                string  `json:"notes,omitempty"`
	Password             string  `json:"password,omitempty"`
	MFA                  string  `json:"mfa,omitempty"`
	TOTPSecret           string  `json:"totp_secret,omitempty"`
	HasTOTP              bool    `json:"has_totp"`
	IsActive             bool    `json:"is_active"`
	OathToken            string  `json:"oath_token,omitempty"`
	OAuthToken           string  `json:"oauth_token,omitempty"`
	RefreshToken         string  `json:"refresh_token,omitempty"`
	AccessToken          string  `json:"access_token,omitempty"`
	Credits              float64 `json:"credits,omitempty"`
	EnableCreditOverages bool    `json:"enable_credit_overages"`
	AllowClaudeGPT       bool    `json:"allow_claude_gpt"`
}

// ExportAccounts returns all accounts formatted for external JSON export.
func (s *Store) ExportAccounts() []AccountExport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]AccountExport, 0, len(s.accounts))
	for _, acc := range s.accounts {
		ea := AccountExport{
			ID:                   acc.Email,
			Email:                acc.Email,
			Label:                acc.Label,
			PlanTier:             acc.PlanTier,
			Status:               acc.Status,
			Priority:             acc.Priority,
			Notes:                acc.Notes,
			Password:             acc.Password,
			MFA:                  acc.TOTPSecret,
			TOTPSecret:           acc.TOTPSecret,
			HasTOTP:              acc.HasTOTP || (acc.TOTPSecret != ""),
			IsActive:             acc.IsActive || (acc.Email == s.activeEmail),
			OathToken:            acc.RefreshToken,
			OAuthToken:           acc.RefreshToken,
			RefreshToken:         acc.RefreshToken,
			AccessToken:          acc.AccessToken,
			Credits:              acc.Credits,
			EnableCreditOverages: acc.EnableCreditOverages,
			AllowClaudeGPT:       acc.AllowClaudeGPT,
		}
		result = append(result, ea)
	}
	return result
}

// BatchImportItem represents an incoming account entry during batch JSON import.
type BatchImportItem struct {
	ID                   string   `json:"id"`
	Email                string   `json:"email"`
	Username             string   `json:"username"`
	Label                string   `json:"label"`
	Name                 string   `json:"name"`
	PlanTier             string   `json:"plan_tier"`
	Plan                 string   `json:"plan"`
	Status               string   `json:"status"`
	Priority             string   `json:"priority"`
	Notes                string   `json:"notes"`
	Password             string   `json:"password"`
	Pass                 string   `json:"pass"`
	MFA                  string   `json:"mfa"`
	MFAToken             string   `json:"mfa_token"`
	TOTP                 string   `json:"totp"`
	TOTPSecret           string   `json:"totp_secret"`
	Secret               string   `json:"secret"`
	OathToken            string   `json:"oath_token"`
	OAuthToken           string   `json:"oauth_token"`
	RefreshToken         string   `json:"refresh_token"`
	Token                string   `json:"token"`
	AccessToken          string   `json:"access_token"`
	Credits              *float64 `json:"credits"`
	EnableCreditOverages *bool    `json:"enable_credit_overages"`
	AllowClaudeGPT       *bool    `json:"allow_claude_gpt"`
	SetActive            *bool    `json:"set_active"`
}

// BatchImportAccounts updates or creates multiple accounts from imported JSON items.
func (s *Store) BatchImportAccounts(items []BatchImportItem) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	importedCount := 0
	for _, item := range items {
		email := strings.TrimSpace(item.Email)
		if email == "" {
			email = strings.TrimSpace(item.ID)
		}
		if email == "" {
			email = strings.TrimSpace(item.Username)
		}
		if email == "" {
			continue
		}

		acc, exists := s.accounts[email]
		if !exists {
			acc = &Account{
				Email:  email,
				Status: "STANDBY",
			}
			s.accounts[email] = acc
		}

		label := item.Label
		if label == "" {
			label = item.Name
		}
		if label != "" {
			acc.Label = label
		} else if acc.Label == "" {
			acc.Label = "Imported Account"
		}

		plan := item.PlanTier
		if plan == "" {
			plan = item.Plan
		}
		if plan != "" {
			acc.PlanTier = plan
		}

		if item.Status != "" {
			acc.Status = strings.ToUpper(strings.TrimSpace(item.Status))
		}

		if item.Priority != "" {
			p := strings.Title(strings.ToLower(strings.TrimSpace(item.Priority)))
			if p == "High" || p == "Mid" || p == "Low" {
				acc.Priority = p
			}
		}

		if item.Notes != "" {
			acc.Notes = item.Notes
		}

		pwd := item.Password
		if pwd == "" {
			pwd = item.Pass
		}
		if pwd != "" {
			acc.Password = pwd
		}

		totpSec := item.TOTPSecret
		if totpSec == "" {
			totpSec = item.MFA
		}
		if totpSec == "" {
			totpSec = item.MFAToken
		}
		if totpSec == "" {
			totpSec = item.TOTP
		}
		if totpSec == "" {
			totpSec = item.Secret
		}
		if totpSec != "" {
			acc.TOTPSecret = strings.TrimSpace(totpSec)
			acc.HasTOTP = true
		}

		rToken := item.RefreshToken
		if rToken == "" {
			rToken = item.OAuthToken
		}
		if rToken == "" {
			rToken = item.OathToken
		}
		if rToken == "" {
			rToken = item.Token
		}
		if rToken != "" {
			cleanToken := strings.TrimSpace(rToken)
			if strings.HasPrefix(cleanToken, "ya29.") {
				acc.AccessToken = cleanToken
			} else {
				if acc.RefreshToken != cleanToken && item.AccessToken == "" {
					acc.AccessToken = ""
					acc.TokenExpiry = time.Time{}
				}
				acc.RefreshToken = cleanToken
			}
		}

		if item.AccessToken != "" {
			acc.AccessToken = strings.TrimSpace(item.AccessToken)
		}

		if item.Credits != nil {
			acc.Credits = *item.Credits
		}
		if item.EnableCreditOverages != nil {
			acc.EnableCreditOverages = *item.EnableCreditOverages
		}
		if item.AllowClaudeGPT != nil {
			acc.AllowClaudeGPT = *item.AllowClaudeGPT
		}

		if (item.SetActive != nil && *item.SetActive) || acc.Status == "ACTIVE" {
			s.activeEmail = email
		}
		importedCount++
	}

	if s.activeEmail == "" && len(s.accounts) > 0 {
		for email := range s.accounts {
			s.activeEmail = email
			break
		}
	}

	for e, a := range s.accounts {
		a.IsActive = (e == s.activeEmail)
		a.HasTOTP = (a.TOTPSecret != "")
	}

	if err := s.save(); err != nil {
		return importedCount, err
	}
	return importedCount, nil
}

// GetAccount retrieves a specific account by email.
func (s *Store) GetAccount(email string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, exists := s.accounts[email]
	if !exists {
		return nil, fmt.Errorf("%w: %s", core.ErrAccountNotFound, email)
	}
	copyAcc := *acc
	return &copyAcc, nil
}

// AddOrUpdateAccount adds a new account or updates existing fields.
func (s *Store) AddOrUpdateAccount(acc *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc.HasTOTP = (acc.TOTPSecret != "")
	if s.activeEmail == "" {
		s.activeEmail = acc.Email
		acc.IsActive = true
	} else {
		acc.IsActive = (acc.Email == s.activeEmail)
	}
	s.accounts[acc.Email] = acc
	return s.save()
}

// ImportAccount imports or updates an account with email, refresh token, and optional parameters.
func (s *Store) ImportAccount(email, refreshToken, accessToken, label, totpSecret string) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	email = strings.TrimSpace(email)
	if email == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}

	acc, exists := s.accounts[email]
	if !exists {
		acc = &Account{
			Email: email,
		}
		s.accounts[email] = acc
	}

	if label != "" {
		acc.Label = label
	} else if acc.Label == "" {
		acc.Label = "Imported Account"
	}

	if refreshToken != "" {
		cleanToken := strings.TrimSpace(refreshToken)
		if strings.HasPrefix(cleanToken, "ya29.") {
			acc.AccessToken = cleanToken
		} else {
			if acc.RefreshToken != cleanToken && accessToken == "" {
				acc.AccessToken = ""
				acc.TokenExpiry = time.Time{}
			}
			acc.RefreshToken = cleanToken
		}
	}
	if accessToken != "" {
		acc.AccessToken = strings.TrimSpace(accessToken)
	}
	if totpSecret != "" {
		acc.TOTPSecret = totpSecret
	}
	acc.HasTOTP = (acc.TOTPSecret != "")

	if s.activeEmail == "" {
		s.activeEmail = email
		acc.IsActive = true
	} else {
		acc.IsActive = (acc.Email == s.activeEmail)
	}

	if err := s.save(); err != nil {
		return nil, err
	}
	copyAcc := *acc
	return &copyAcc, nil
}

// SetTOTPSecret updates or removes the TOTP secret for an account.
func (s *Store) SetTOTPSecret(email, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[email]
	if !exists {
		return fmt.Errorf("%w: %s", core.ErrAccountNotFound, email)
	}
	acc.TOTPSecret = secret
	acc.HasTOTP = (secret != "")
	return s.save()
}

// SetAccessToken sets or clears the cached access token for an account.
func (s *Store) SetAccessToken(email, accessToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[email]
	if !exists {
		return fmt.Errorf("%w: %s", core.ErrAccountNotFound, email)
	}
	acc.AccessToken = strings.TrimSpace(accessToken)
	return s.save()
}

// SetActiveAccount switches the active account atomically.
func (s *Store) SetActiveAccount(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target *Account
	for e, a := range s.accounts {
		if strings.EqualFold(e, email) {
			target = a
			break
		}
	}
	if target == nil {
		return fmt.Errorf("%w: %s", core.ErrAccountNotFound, email)
	}

	st := strings.ToUpper(strings.TrimSpace(target.Status))
	if st == "BANNED" {
		return fmt.Errorf("account %s is banned and cannot be switched on", email)
	}

	s.activeEmail = target.Email
	s.lastManualSwitchTime = time.Now()
	for _, a := range s.accounts {
		if strings.EqualFold(a.Email, target.Email) {
			a.IsActive = true
			if a.Status != "BANNED" && a.Status != "ERROR" {
				a.Status = "ACTIVE"
			}
		} else {
			a.IsActive = false
			if a.Status == "ACTIVE" {
				a.Status = "STANDBY"
			}
		}
	}
	return s.save()
}

// UpdateAccountDetails updates label, planTier, status, priority, notes, password, totpSecret, refreshToken, and optionally sets active status.
func (s *Store) UpdateAccountDetails(email, label, planTier, status, priority, notes, password, totpSecret, refreshToken string, setActive bool) error {
	s.mu.RLock()
	acc, exists := s.accounts[email]
	credits := 0.0
	enableOverages := false
	allowClaudeGPT := false
	if exists {
		credits = acc.Credits
		enableOverages = acc.EnableCreditOverages
		allowClaudeGPT = acc.AllowClaudeGPT
	}
	s.mu.RUnlock()
	return s.UpdateAccountFull(email, label, planTier, status, priority, notes, password, totpSecret, refreshToken, credits, enableOverages, allowClaudeGPT, setActive)
}

// UpdateAccountFull updates all account details including credits and credit overages toggle.
func (s *Store) UpdateAccountFull(email, label, planTier, status, priority, notes, password, totpSecret, refreshToken string, credits float64, enableCreditOverages, allowClaudeGPT, setActive bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[email]
	targetStatus := strings.ToUpper(strings.TrimSpace(status))
	if targetStatus == "" && exists {
		targetStatus = strings.ToUpper(strings.TrimSpace(acc.Status))
	}
	if targetStatus == "" {
		targetStatus = "STANDBY"
	}

	if setActive {
		if targetStatus == "BANNED" {
			return fmt.Errorf("account %s is banned and cannot be switched on", email)
		}
	}

	if !exists {
		acc = &Account{
			Email:  email,
			Status: "STANDBY",
		}
		if s.activeEmail == "" && targetStatus != "COOLDOWN" && targetStatus != "COOLING" && targetStatus != "BANNED" && targetStatus != "ERROR" {
			s.activeEmail = email
			acc.IsActive = true
		}
		s.accounts[email] = acc
	}

	if label != "" {
		acc.Label = label
	} else if acc.Label == "" {
		acc.Label = email
	}
	if planTier != "" && planTier != "Free" {
		acc.PlanTier = planTier
	} else if acc.PlanTier == "" {
		if planTier != "" {
			acc.PlanTier = planTier
		} else {
			acc.PlanTier = "Free"
		}
	}
	if status != "" {
		acc.Status = strings.ToUpper(status)
	}
	if priority != "" {
		p := strings.Title(strings.ToLower(strings.TrimSpace(priority)))
		if p == "High" || p == "Mid" || p == "Low" {
			acc.Priority = p
		}
	}
	acc.Notes = notes
	if password != "" {
		acc.Password = password
	}
	acc.TOTPSecret = totpSecret
	acc.HasTOTP = (totpSecret != "")
	if refreshToken != "" {
		cleanToken := strings.TrimSpace(refreshToken)
		if strings.HasPrefix(cleanToken, "ya29.") {
			acc.AccessToken = cleanToken
		} else {
			if acc.RefreshToken != cleanToken {
				acc.AccessToken = ""
				acc.TokenExpiry = time.Time{}
			}
			acc.RefreshToken = cleanToken
		}
	}
	// Anti-downgrade safeguard: only update credits if positive, or if existing credits are already zero.
	// This prevents overwriting auto-detected positive balances with empty form defaults.
	if credits > 0 {
		acc.Credits = credits
	} else if acc.Credits == 0 {
		acc.Credits = credits
	}
	acc.EnableCreditOverages = enableCreditOverages
	acc.AllowClaudeGPT = allowClaudeGPT

	if setActive {
		s.activeEmail = email
		s.lastManualSwitchTime = time.Now()
		for e, a := range s.accounts {
			if strings.EqualFold(e, email) {
				a.IsActive = true
				if a.Status != "BANNED" && a.Status != "ERROR" {
					a.Status = "ACTIVE"
				}
			} else {
				a.IsActive = false
				if a.Status == "ACTIVE" {
					a.Status = "STANDBY"
				}
			}
		}
	} else {
		acc.IsActive = (s.activeEmail != "" && strings.EqualFold(email, s.activeEmail))
		if !acc.IsActive && acc.Status == "ACTIVE" {
			acc.Status = "STANDBY"
		} else if acc.IsActive && acc.Status != "BANNED" && acc.Status != "ERROR" {
			acc.Status = "ACTIVE"
		}
	}

	return s.save()
}

// UpdateAccountStatus updates the status of an account (e.g. STANDBY, COOLDOWN, COOLING, ERROR, BANNED).
func (s *Store) UpdateAccountStatus(email, status string) error {
	return s.UpdateAccountStatusWithError(email, status, "")
}

// UpdateAccountStatusWithError updates the status and error message of an account.
func (s *Store) UpdateAccountStatusWithError(email, status, errorMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target *Account
	for e, a := range s.accounts {
		if strings.EqualFold(e, email) {
			target = a
			break
		}
	}
	if target == nil {
		return fmt.Errorf("%w: %s", core.ErrAccountNotFound, email)
	}

	st := strings.ToUpper(strings.TrimSpace(status))
	if st == "ACTIVE" && !target.IsActive {
		return fmt.Errorf("cannot set ACTIVE status without setting active account")
	}
	if (st == "COOLDOWN" || st == "COOLING" || st == "ERROR" || st == "BANNED") && target.IsActive {
		target.IsActive = false
		if strings.EqualFold(s.activeEmail, target.Email) {
			s.activeEmail = ""
		}
	}
	target.Status = st
	target.ErrorMessage = errorMessage
	return s.save()
}

// UpdateAccountQuotaMetadata safely updates PlanTier and Credits without affecting active state, status, or tokens.
func (s *Store) UpdateAccountQuotaMetadata(email, planTier string, credits float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target *Account
	for e, a := range s.accounts {
		if strings.EqualFold(e, email) {
			target = a
			break
		}
	}
	if target == nil {
		return nil
	}

	modified := false
	if planTier != "" && planTier != "Free" && target.PlanTier != planTier {
		target.PlanTier = planTier
		modified = true
	}
	if credits > 0 && target.Credits != credits {
		target.Credits = credits
		modified = true
	}

	if modified {
		return s.save()
	}
	return nil
}

// UpdateAccountTokens updates the access token (and optionally refresh token) without altering other fields.
func (s *Store) UpdateAccountTokens(email, accessToken, refreshToken string) error {
	return s.UpdateAccountTokensWithExpiry(email, accessToken, refreshToken, time.Time{})
}

// UpdateAccountTokensWithExpiry updates the access token, refresh token, and token expiry.
func (s *Store) UpdateAccountTokensWithExpiry(email, accessToken, refreshToken string, expiry time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target *Account
	for e, a := range s.accounts {
		if strings.EqualFold(e, email) {
			target = a
			break
		}
	}
	if target == nil {
		return nil
	}

	modified := false
	if accessToken != "" && target.AccessToken != accessToken {
		target.AccessToken = accessToken
		if !expiry.IsZero() {
			target.TokenExpiry = expiry
		} else {
			target.TokenExpiry = time.Now().Add(55 * time.Minute)
		}
		modified = true
	} else if !expiry.IsZero() && !target.TokenExpiry.Equal(expiry) {
		target.TokenExpiry = expiry
		modified = true
	}
	if refreshToken != "" && target.RefreshToken != refreshToken {
		target.RefreshToken = refreshToken
		modified = true
	}

	if modified {
		return s.save()
	}
	return nil
}

// UpdateAccountTokensAndMetadata updates tokens, plan tier, and credits in a single write operation.
func (s *Store) UpdateAccountTokensAndMetadata(email, accessToken, refreshToken, planTier string, credits float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target *Account
	for e, a := range s.accounts {
		if strings.EqualFold(e, email) {
			target = a
			break
		}
	}
	if target == nil {
		return nil
	}

	modified := false
	if accessToken != "" && target.AccessToken != accessToken {
		target.AccessToken = accessToken
		target.TokenExpiry = time.Now().Add(55 * time.Minute)
		modified = true
	}
	if refreshToken != "" && target.RefreshToken != refreshToken {
		target.RefreshToken = refreshToken
		modified = true
	}
	if planTier != "" && planTier != "Free" && target.PlanTier != planTier {
		target.PlanTier = planTier
		modified = true
	}
	if credits > 0 && target.Credits != credits {
		target.Credits = credits
		modified = true
	}

	if modified {
		return s.save()
	}
	return nil
}

// UpdateAccountWithStatus updates label, planTier, status, totpSecret, refreshToken, and optionally sets active status.
func (s *Store) UpdateAccountWithStatus(email, label, planTier, status, totpSecret, refreshToken string, setActive bool) error {
	return s.UpdateAccountDetails(email, label, planTier, status, "", "", "", totpSecret, refreshToken, setActive)
}

// UpdateAccountWithTier updates label, planTier, totpSecret, refreshToken, and optionally sets active status.
func (s *Store) UpdateAccountWithTier(email, label, planTier, totpSecret, refreshToken string, setActive bool) error {
	return s.UpdateAccountWithStatus(email, label, planTier, "", totpSecret, refreshToken, setActive)
}

// UpdateAccount updates label, totpSecret, refreshToken, and optionally sets active status.
func (s *Store) UpdateAccount(email, label, totpSecret, refreshToken string, setActive bool) error {
	return s.UpdateAccountWithTier(email, label, "", totpSecret, refreshToken, setActive)
}

// RemoveAccount deletes an account from the store.
func (s *Store) RemoveAccount(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.accounts[email]; !exists {
		return fmt.Errorf("%w: %s", core.ErrAccountNotFound, email)
	}

	delete(s.accounts, email)

	if s.activeEmail == email {
		s.activeEmail = ""
		for e, a := range s.accounts {
			s.activeEmail = e
			a.IsActive = true
			break
		}
		_ = SyncAppStorageLoginUser(s.activeEmail)
	}

	return s.save()
}

// ActiveAccount returns the currently active account email.
func (s *Store) ActiveAccount() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeEmail
}

// ReconcileActiveAccount detects the account actively in use across Antigravity applications
// (strictly following the priority sequence: Antigravity 2.0 Desktop > Antigravity VS Code Extension > Antigravity CLI)
// and reconciles it with the Swiss Knife vault:
// 1. If the running account is already in the vault:
//   - Sets it as active.
//   - Ensures other accounts are not active.
//   - Synchronizes all 3 surfaces to this active account so they remain unified.
//
// 2. If the running account is NOT in the vault:
//   - If autoImport is true:
//     Automatically imports the account into the vault, sets it as active,
//     and synchronizes all 3 surfaces to this active account.
//   - If autoImport is false:
//     No account in Swiss Knife is treated as active (active_account: "", all IsActive: false).
func (s *Store) ReconcileActiveAccount(autoImport bool, allEmails []string, profileMgr *fingerprint.Store) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	detected := ResolveRunningAntigravityAccount(s.homeDir, s.antigravityConfigDir)
	if detected == nil {
		// No running account detected on host surfaces.
		if s.activeEmail != "" {
			if acc, exists := s.accounts[s.activeEmail]; exists {
				return acc, nil
			}
		}
		return nil, nil
	}

	detectedEmail := strings.TrimSpace(detected.Email)
	if detectedEmail == "" {
		return nil, nil
	}

	// In-flight manual switch latch: if an account switch occurred within the last 12 seconds,
	// protect s.activeEmail from being reverted by a stale in-memory session while the IDE respawns.
	if !s.lastManualSwitchTime.IsZero() && time.Since(s.lastManualSwitchTime) < 12*time.Second {
		if s.activeEmail != "" && !strings.EqualFold(detectedEmail, s.activeEmail) {
			if curAcc, ok := s.accounts[s.activeEmail]; ok {
				return curAcc, nil
			}
		}
	}

	// Check if detected account exists in vault
	var targetAcc *Account
	for email, acc := range s.accounts {
		if strings.EqualFold(email, detectedEmail) {
			targetAcc = acc
			break
		}
	}

	if targetAcc != nil {
		// Update tokens if detected tokens are non-empty and target has empty
		if detected.RefreshToken != "" && targetAcc.RefreshToken == "" {
			targetAcc.RefreshToken = detected.RefreshToken
		}
		if detected.AccessToken != "" && targetAcc.AccessToken == "" {
			targetAcc.AccessToken = detected.AccessToken
		}
		if detected.IDToken != "" && targetAcc.IDToken == "" {
			targetAcc.IDToken = detected.IDToken
		}

		prevActive := s.activeEmail
		isSameActive := strings.EqualFold(prevActive, targetAcc.Email)

		s.activeEmail = targetAcc.Email
		for em, a := range s.accounts {
			if strings.EqualFold(em, targetAcc.Email) {
				a.IsActive = true
				if a.Status != "BANNED" && a.Status != "ERROR" {
					a.Status = "ACTIVE"
				}
			} else {
				a.IsActive = false
				if a.Status == "ACTIVE" {
					a.Status = "STANDBY"
				}
			}
		}
		_ = s.save()

		// Only synchronize surfaces if the active account actually changed or had no previous active,
		// AND target account has non-empty credentials so we don't clobber host files with empty data.
		if (!isSameActive || prevActive == "") && (targetAcc.RefreshToken != "" || targetAcc.AccessToken != "") {
			_ = SyncAllSurfaces(targetAcc, allEmails, profileMgr)
			_ = s.save()
		} else {
			_ = SyncAppStorageLoginUser(targetAcc.Email)
		}
		copyAcc := *targetAcc
		return &copyAcc, nil
	}

	// Account is NOT in vault!
	if !autoImport {
		// "if the account running in antigravity has not been imported to our app, then no account should be treated as active."
		s.activeEmail = ""
		for _, a := range s.accounts {
			a.IsActive = false
			if a.Status == "ACTIVE" {
				a.Status = "STANDBY"
			}
		}
		_ = s.save()
		return nil, nil
	}

	// autoImport is true: import account automatically
	label := "Imported (" + detected.SurfaceName + ")"
	acc := &Account{
		Email:        detectedEmail,
		Label:        label,
		PlanTier:     "Free",
		Status:       "ACTIVE",
		Priority:     "High",
		IsActive:     true,
		AccessToken:  detected.AccessToken,
		RefreshToken: detected.RefreshToken,
		IDToken:      detected.IDToken,
	}
	s.accounts[detectedEmail] = acc
	s.activeEmail = detectedEmail

	for em, a := range s.accounts {
		if !strings.EqualFold(em, detectedEmail) {
			a.IsActive = false
			if a.Status == "ACTIVE" {
				a.Status = "STANDBY"
			}
		}
	}
	_ = s.save()

	if acc.RefreshToken != "" || acc.AccessToken != "" {
		allWithNew := append(allEmails, detectedEmail)
		_ = SyncAllSurfaces(acc, allWithNew, profileMgr)
		_ = s.save()
	}
	copyAcc := *acc
	return &copyAcc, nil
}

// ClearActiveAccount sets activeEmail to empty string and marks all accounts as inactive.
func (s *Store) ClearActiveAccount() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.activeEmail = ""
	for _, a := range s.accounts {
		a.IsActive = false
		if a.Status == "ACTIVE" {
			a.Status = "STANDBY"
		}
	}
	return s.save()
}
