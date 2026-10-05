package keyring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// Store manages account inventory and atomic credentials swapping.
type Store struct {
	accountsPath string
	accounts     map[string]*Account
	activeEmail  string
	mu           sync.RWMutex
}

// NewStore initializes a Keyring store at the specified path (or default).
func NewStore(accountsPath string) (*Store, error) {
	if accountsPath == "" {
		accountsPath = filepath.Join(core.GetConfigDir(), "accounts.json")
	}

	s := &Store{
		accountsPath: accountsPath,
		accounts:     make(map[string]*Account),
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

type rawAccountItem struct {
	Email      string `json:"email"`
	Label      string `json:"label"`
	PlanTier   string `json:"plan_tier,omitempty"`
	Status     string `json:"status,omitempty"`
	IsHealthy  *bool  `json:"is_healthy,omitempty"`
	TOTPSecret string `json:"totp_secret"`
	Credential *struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Expiry       string `json:"expiry"`
	} `json:"credential,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
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
				acc := &Account{
					Email:        em,
					Label:        item.Label,
					PlanTier:     item.PlanTier,
					TOTPSecret:   item.TOTPSecret,
					HasTOTP:      item.TOTPSecret != "",
					IsActive:     (em == s.activeEmail),
					AccessToken:  item.AccessToken,
					RefreshToken: item.RefreshToken,
				}
				if item.Credential != nil {
					if acc.AccessToken == "" {
						acc.AccessToken = item.Credential.AccessToken
					}
					if acc.RefreshToken == "" {
						acc.RefreshToken = item.Credential.RefreshToken
					}
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
			acc := &Account{
				Email:        item.Email,
				Label:        item.Label,
				PlanTier:     item.PlanTier,
				TOTPSecret:   item.TOTPSecret,
				HasTOTP:      item.TOTPSecret != "",
				IsActive:     (item.Email == s.activeEmail),
				AccessToken:  item.AccessToken,
				RefreshToken: item.RefreshToken,
			}
			if item.Credential != nil {
				if acc.AccessToken == "" {
					acc.AccessToken = item.Credential.AccessToken
				}
				if acc.RefreshToken == "" {
					acc.RefreshToken = item.Credential.RefreshToken
				}
			}
			s.accounts[item.Email] = acc
		}
	}
	return nil
}

func (s *Store) save() error {
	dir := filepath.Dir(s.accountsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	type exportedAccount struct {
		Email      string `json:"email"`
		Label      string `json:"label"`
		PlanTier   string `json:"plan_tier,omitempty"`
		TOTPSecret string `json:"totp_secret"`
		IsHealthy  bool   `json:"is_healthy"`
		Credential struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AuthMethod   string `json:"auth_method"`
			TokenType    string `json:"token_type"`
		} `json:"credential"`
	}

	accMap := make(map[string]exportedAccount)
	for _, acc := range s.accounts {
		acc.IsActive = (acc.Email == s.activeEmail)
		acc.HasTOTP = (acc.TOTPSecret != "")

		ea := exportedAccount{
			Email:      acc.Email,
			Label:      acc.Label,
			PlanTier:   acc.PlanTier,
			TOTPSecret: acc.TOTPSecret,
			IsHealthy:  true,
		}
		ea.Credential.AccessToken = acc.AccessToken
		ea.Credential.RefreshToken = acc.RefreshToken
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
	return list
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
		acc.RefreshToken = refreshToken
	}
	if accessToken != "" {
		acc.AccessToken = accessToken
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

// SetActiveAccount switches the active account atomically.
func (s *Store) SetActiveAccount(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.accounts[email]; !exists {
		return fmt.Errorf("%w: %s", core.ErrAccountNotFound, email)
	}

	s.activeEmail = email
	for e, a := range s.accounts {
		a.IsActive = (e == email)
	}
	return s.save()
}

// UpdateAccountWithTier updates label, planTier, totpSecret, refreshToken, and optionally sets active status.
func (s *Store) UpdateAccountWithTier(email, label, planTier, totpSecret, refreshToken string, setActive bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[email]
	if !exists {
		return fmt.Errorf("%w: %s", core.ErrAccountNotFound, email)
	}

	if label != "" {
		acc.Label = label
	}
	if planTier != "" {
		acc.PlanTier = planTier
	}
	acc.TOTPSecret = totpSecret
	acc.HasTOTP = (totpSecret != "")
	if refreshToken != "" {
		acc.RefreshToken = refreshToken
	}

	if setActive {
		s.activeEmail = email
		for e, a := range s.accounts {
			a.IsActive = (e == email)
		}
	} else {
		acc.IsActive = (email == s.activeEmail)
	}

	return s.save()
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
	}

	return s.save()
}

// ActiveAccount returns the currently active account email.
func (s *Store) ActiveAccount() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeEmail
}
