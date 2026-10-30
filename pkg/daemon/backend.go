package daemon

import (
	"fmt"
	"os"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/totp"
)

// Backend exposes daemon-owned state to the web layer.
func (d *Daemon) Backend() backend.BackendService {
	return daemonBackend{daemon: d}
}

type daemonBackend struct {
	daemon *Daemon
}

func (b daemonBackend) Accounts() backend.AccountService {
	return daemonAccounts{daemon: b.daemon}
}

func (b daemonBackend) Quota() backend.QuotaService              { return nil }
func (b daemonBackend) CustomModels() backend.CustomModelService { return nil }
func (b daemonBackend) Enhancements() backend.EnhancementService { return nil }
func (b daemonBackend) GitHub() backend.GitHubService            { return nil }
func (b daemonBackend) Vault() backend.VaultService              { return nil }
func (b daemonBackend) Revival() backend.RevivalService          { return nil }

type daemonAccounts struct {
	daemon *Daemon
}

func (s daemonAccounts) ListAccounts() []*keyring.Account {
	return s.daemon.Keyring.ListAccounts()
}

func (s daemonAccounts) GetAccount(email string) (*keyring.Account, error) {
	return s.daemon.Keyring.GetAccount(email)
}

func (s daemonAccounts) ScanAccounts() ([]keyring.DiscoveredAccount, error) {
	return keyring.NewScanner(s.daemon.Keyring).Scan()
}

func (s daemonAccounts) ImportAccount(req backend.AccountImport) (*keyring.Account, error) {
	acc, err := s.daemon.Keyring.ImportAccount(req.Email, req.RefreshToken, req.AccessToken, req.Label, req.TOTPSecret)
	if err != nil {
		return nil, err
	}
	if s.daemon.Profiles != nil {
		_, _ = s.daemon.Profiles.GetOrCreateProfile(req.Email)
	}
	if acc != nil && (acc.AccessToken != "" || acc.RefreshToken != "") {
		summary, _ := quota.PollAndCacheAccount(acc, s.daemon.Keyring)
		if summary != nil {
			s.daemon.setQuotaSummary(req.Email, summary)
		}
		if refreshed, getErr := s.daemon.Keyring.GetAccount(req.Email); getErr == nil {
			acc = refreshed
		}
	}
	return acc, nil
}

func (s daemonAccounts) UpdateAccount(req backend.AccountUpdate) (map[string]interface{}, error) {
	existing, _ := s.daemon.Keyring.GetAccount(req.Email)
	if existing != nil {
		if req.Password == "[REDACTED]" || req.Password == "" {
			req.Password = existing.Password
		}
		if req.TOTPSecret == "[REDACTED]" || req.TOTPSecret == "" {
			req.TOTPSecret = existing.TOTPSecret
		}
		if req.RefreshToken == "[REDACTED]" {
			req.RefreshToken = existing.RefreshToken
		}
		if req.AccessToken == "[REDACTED]" {
			req.AccessToken = existing.AccessToken
		}
	}
	if req.TOTPSecret != "" && !totp.ValidateSecret(req.TOTPSecret) {
		return nil, core.ErrInvalidSecret
	}

	prevActive := s.daemon.Keyring.ActiveAccount()
	if err := s.daemon.Keyring.UpdateAccountFull(req.Email, req.Label, req.PlanTier, req.Status, req.Priority, req.Notes, req.Password, req.TOTPSecret, req.RefreshToken, req.Credits, req.EnableCreditOverages, req.AllowClaudeGPT, req.SetActive); err != nil {
		return nil, err
	}
	if req.AccessToken != "" && req.AccessToken != "[REDACTED]" {
		if err := s.daemon.Keyring.SetAccessToken(req.Email, req.AccessToken); err != nil {
			return nil, err
		}
	}

	acc, _ := s.daemon.Keyring.GetAccount(req.Email)
	var summary *quota.QuotaSummary
	if acc != nil && (acc.AccessToken != "" || acc.RefreshToken != "") {
		oldToken := acc.AccessToken
		var pollErr error
		summary, pollErr = quota.PollAndCacheAccount(acc, s.daemon.Keyring)
		if summary != nil {
			s.daemon.setQuotaSummary(req.Email, summary)
			if pollErr == nil && !req.SetActive && strings.EqualFold(req.Email, s.daemon.Keyring.ActiveAccount()) && acc.AccessToken != "" && acc.AccessToken != oldToken {
				s.daemon.syncActiveAccountSurfaces(acc)
			}
		} else {
			s.daemon.removeQuotaSummary(req.Email)
			s.daemon.triggerQuotaRefreshAsync()
		}
		if refreshed, getErr := s.daemon.Keyring.GetAccount(req.Email); getErr == nil {
			acc = refreshed
		}
	}

	if req.SetActive {
		s.daemon.switchMu.Lock()
		var allEmails []string
		for _, account := range s.daemon.Keyring.ListAccounts() {
			allEmails = append(allEmails, account.Email)
		}
		if refreshed, getErr := s.daemon.Keyring.GetAccount(req.Email); getErr == nil {
			acc = refreshed
			_ = keyring.SyncAllSurfaces(acc, allEmails, s.daemon.Profiles)
			_ = s.daemon.Keyring.UpdateAccountTokensWithExpiry(acc.Email, acc.AccessToken, acc.RefreshToken, acc.TokenExpiry)
			if !strings.EqualFold(prevActive, req.Email) && s.daemon.Shield != nil && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
				if s.daemon.Revival != nil {
					_, _ = s.daemon.Revival.CapturePreSwitchIntents(core.TargetAppDesktop)
				}
				_ = gui.NewInjector(0).CaptureActiveConversationPath()
				_ = s.daemon.Shield.RelaunchHostIDE()
			} else {
				_, _ = gui.NewInjector(0).RefreshUserStatus()
			}
		}
		s.daemon.switchMu.Unlock()
	}

	res := map[string]interface{}{"success": true, "email": req.Email}
	if acc != nil {
		res["plan_tier"] = acc.PlanTier
		res["credits"] = acc.Credits
		res["status"] = acc.Status
		res["error_message"] = acc.ErrorMessage
	}
	if summary != nil {
		res["quota"] = summary
	}
	return res, nil
}

func (s daemonAccounts) RemoveAccount(email string) (map[string]interface{}, error) {
	prevActive := s.daemon.Keyring.ActiveAccount()
	if err := s.daemon.Keyring.RemoveAccount(email); err != nil {
		return nil, err
	}
	s.daemon.removeQuotaSummary(email)
	newActive := s.daemon.Keyring.ActiveAccount()
	if newActive != "" && !strings.EqualFold(prevActive, newActive) {
		s.daemon.switchMu.Lock()
		defer s.daemon.switchMu.Unlock()
		var allEmails []string
		for _, account := range s.daemon.Keyring.ListAccounts() {
			allEmails = append(allEmails, account.Email)
		}
		if acc, err := s.daemon.Keyring.GetAccount(newActive); err == nil {
			_ = keyring.SyncAllSurfaces(acc, allEmails, s.daemon.Profiles)
			_ = s.daemon.Keyring.UpdateAccountTokensWithExpiry(acc.Email, acc.AccessToken, acc.RefreshToken, acc.TokenExpiry)
			if s.daemon.Shield != nil && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
				if s.daemon.Revival != nil {
					_, _ = s.daemon.Revival.CapturePreSwitchIntents(core.TargetAppDesktop)
				}
				_ = gui.NewInjector(0).CaptureActiveConversationPath()
				_ = s.daemon.Shield.RelaunchHostIDE()
			}
		}
	}
	return map[string]interface{}{"success": true, "removed": email, "active_account": newActive}, nil
}

func (s daemonAccounts) SwitchAccount(req backend.AccountSwitch) (backend.AccountSwitchResult, error) {
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return backend.AccountSwitchResult{}, fmt.Errorf("email is required")
	}

	s.daemon.switchMu.Lock()
	defer s.daemon.switchMu.Unlock()
	if err := s.daemon.Keyring.SetActiveAccount(email); err != nil {
		return backend.AccountSwitchResult{}, err
	}

	acc, err := s.daemon.Keyring.GetAccount(email)
	if err != nil {
		return backend.AccountSwitchResult{}, err
	}
	var allEmails []string
	for _, account := range s.daemon.Keyring.ListAccounts() {
		allEmails = append(allEmails, account.Email)
	}
	targetApp := strings.TrimSpace(req.TargetApp)
	if targetApp == "" || strings.EqualFold(targetApp, core.TargetAppAll) || s.daemon.Config.GetMultiAppSyncMode() != core.MultiAppSyncModeIndividual {
		_ = keyring.SyncAllSurfaces(acc, allEmails, s.daemon.Profiles)
		targetApp = core.TargetAppAll
	} else {
		_ = keyring.SyncSurface(targetApp, acc, allEmails, s.daemon.Profiles)
	}
	if err := s.daemon.Config.SetActiveAppAccount(targetApp, acc.Email); err != nil {
		return backend.AccountSwitchResult{}, err
	}
	_ = s.daemon.Keyring.UpdateAccountTokensWithExpiry(acc.Email, acc.AccessToken, acc.RefreshToken, acc.TokenExpiry)

	shouldRelaunch := true
	if req.RelaunchIDE != nil {
		shouldRelaunch = *req.RelaunchIDE
	}
	if shouldRelaunch && s.daemon.Shield != nil && s.daemon.Shield.IsAntigravityRunning() && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
		if s.daemon.Revival != nil {
			_, _ = s.daemon.Revival.CapturePreSwitchIntents(core.TargetAppDesktop)
		}
		_ = gui.NewInjector(0).CaptureActiveConversationPath()
		if relaunchErr := s.daemon.Shield.RelaunchHostIDE(); relaunchErr != nil {
			return backend.AccountSwitchResult{}, relaunchErr
		}
	} else {
		_, _ = gui.NewInjector(0).RefreshUserStatus()
	}

	return backend.AccountSwitchResult{
		Switched:          true,
		Account:           acc.Email,
		Success:           true,
		ActiveAccount:     acc.Email,
		RelaunchIDE:       shouldRelaunch,
		TargetApp:         targetApp,
		MultiAppSyncMode:  s.daemon.Config.GetMultiAppSyncMode(),
		ActiveAppAccounts: s.daemon.Config.GetActiveAppAccounts(),
	}, nil
}

func (s daemonAccounts) ExportAccounts() []keyring.AccountExport {
	return s.daemon.Keyring.ExportAccounts()
}

func (s daemonAccounts) BatchImportAccounts(items []keyring.BatchImportItem) (int, error) {
	return s.daemon.Keyring.BatchImportAccounts(items)
}
