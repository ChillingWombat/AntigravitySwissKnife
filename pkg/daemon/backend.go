package daemon

import (
	"fmt"
	"os"
	"strings"
	"time"

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

func (b daemonBackend) CustomModels() backend.CustomModelsService {
	return b.daemon.CustomModels
}

func (b daemonBackend) Enhancements() backend.EnhancementsService {
	return b.daemon.Enhancements
}

func (b daemonBackend) Quota() backend.QuotaService {
	return daemonQuota{daemon: b.daemon}
}

func (b daemonBackend) GUI() backend.GUIService {
	return daemonGUI{daemon: b.daemon}
}

func (b daemonBackend) GitHub() backend.GitHubService {
	return b.daemon.GitHub
}

func (b daemonBackend) Vault() backend.VaultService {
	return b.daemon.Vault
}

func (b daemonBackend) Revival() backend.RevivalService {
	return b.daemon.Revival
}

func (b daemonBackend) ActiveState() backend.ActiveStateService {
	return daemonActiveState{daemon: b.daemon}
}

type daemonQuota struct {
	daemon *Daemon
}

func (s daemonQuota) GetSummary(email string, refresh bool) (*quota.QuotaSummary, error) {
	target := strings.TrimSpace(email)
	if target == "" {
		target = s.daemon.Keyring.ActiveAccount()
	}
	normTarget := strings.ToLower(target)

	if !refresh {
		if cached := s.daemon.getQuotaSummary(target); cached != nil {
			return cached, nil
		}
		diskCache := quota.LoadQuotaCache()
		if cached, ok := diskCache[normTarget]; ok && cached != nil {
			s.daemon.setQuotaSummary(target, cached)
			return cached, nil
		}
	}

	acc, _ := s.daemon.Keyring.GetAccount(target)
	now := time.Now()
	if acc != nil && (acc.AccessToken != "" || acc.RefreshToken != "") {
		oldToken := acc.AccessToken
		summary, err := quota.PollAndCacheAccount(acc, s.daemon.Keyring)
		if summary != nil {
			s.daemon.setQuotaSummary(target, summary)
			if err == nil && strings.EqualFold(target, s.daemon.Keyring.ActiveAccount()) && acc.AccessToken != "" && acc.AccessToken != oldToken {
				s.daemon.syncActiveAccountSurfaces(acc)
			}
			return summary, nil
		}
	}
	if !refresh {
		if cached := s.daemon.getQuotaSummary(target); cached != nil {
			return cached, nil
		}
	}
	return &quota.QuotaSummary{
		AccountEmail:  target,
		Models:        []quota.ModelQuota{},
		MinFraction:   0.0,
		OverallHealth: core.StatusExhausted,
		LastPolled:    now,
	}, nil
}

func (s daemonQuota) GetFleet() *quota.FleetQuotaSummary {
	var allEmails []string
	for _, account := range s.daemon.Keyring.ListAccounts() {
		allEmails = append(allEmails, account.Email)
	}
	_, _ = s.daemon.Keyring.ReconcileActiveAccount(s.daemon.Config.AutoImportActiveAccount, allEmails, s.daemon.Profiles)

	accounts := s.daemon.Keyring.ListAccounts()
	active := s.daemon.Keyring.ActiveAccount()
	summaries := s.daemon.getQuotaSummaries()
	if len(summaries) == 0 && len(accounts) > 0 {
		s.daemon.triggerQuotaRefreshAsync()
	}

	s.daemon.mu.RLock()
	threshold := s.daemon.Config.AutoSwitchThreshold
	weeklyThreshold := s.daemon.Config.AutoSwitchWeeklyThreshold
	switchMode := s.daemon.Config.SwitchMode
	allowNonGemini := s.daemon.Config.AllowNonGeminiNativeModels
	s.daemon.mu.RUnlock()
	if threshold <= 0 {
		threshold = core.DefaultAutoSwitchThresholdFraction
	}
	if weeklyThreshold <= 0 {
		weeklyThreshold = core.DefaultAutoSwitchWeeklyThresholdFraction
	}
	if switchMode == "" {
		switchMode = core.DefaultSwitchMode
	}

	states := quota.BuildAccountQuotaStatesFromMapWithThresholds(accounts, summaries, threshold, weeklyThreshold, allowNonGemini)
	states = quota.SortAccountQuotaStatesWithThresholds(states, active, threshold, weeklyThreshold, "auto", switchMode, allowNonGemini)
	summary := quota.ComputeFleetSummary(states, active)
	for i := range summary.Accounts {
		summary.Accounts[i].Password = ""
		summary.Accounts[i].TOTPSecret = ""
		summary.Accounts[i].AccessToken = ""
		if strings.TrimSpace(summary.Accounts[i].RefreshToken) != "" {
			summary.Accounts[i].RefreshToken = "[REDACTED]"
		}
	}
	s.daemon.pollingMu.Lock()
	summary.Refreshing = s.daemon.isPollingFleet
	s.daemon.pollingMu.Unlock()
	return &summary
}

func (s daemonQuota) RefreshFleet() error {
	s.daemon.triggerQuotaRefreshAsync()
	return nil
}

type daemonGUI struct {
	daemon *Daemon
}

func (s daemonGUI) GetConfig() gui.Config {
	return s.daemon.GUIStore.GetConfig()
}

func (s daemonGUI) UpdateConfigPartial(rawJSON []byte) (*gui.GUIConfig, error) {
	defer s.daemon.invalidateActiveState()
	return s.daemon.GUIStore.UpdateConfigPartial(rawJSON)
}

func (s daemonGUI) DetectProjects() ([]gui.ProjectItem, error) {
	return s.daemon.GUIStore.DetectProjects()
}

func (s daemonGUI) SetProjectColor(projectName, hexColor string) error {
	return s.daemon.GUIStore.SetProjectColor(projectName, hexColor)
}

func (s daemonGUI) DeleteProject(projectName string) error {
	return s.daemon.GUIStore.DeleteProject(projectName)
}

func (s daemonGUI) RemoveProjectColor(projectName string) error {
	return s.daemon.GUIStore.RemoveProjectColor(projectName)
}

func (s daemonGUI) UpdateProjectOrder(order []string) error {
	return s.daemon.GUIStore.UpdateProjectOrder(order)
}

func (s daemonGUI) Apply() (*gui.ApplyResult, error) {
	return s.daemon.GUIStore.Apply()
}

func (s daemonGUI) GetArchivedProjects() ([]gui.ArchivedProjectItem, error) {
	return s.daemon.GUIStore.GetArchivedProjects()
}

func (s daemonGUI) ArchiveProject(projectName string) error {
	return s.daemon.GUIStore.ArchiveProject(projectName)
}

func (s daemonGUI) RestoreProject(projectName string) error {
	return s.daemon.GUIStore.RestoreProject(projectName)
}

func (s daemonGUI) OpenProjectSettings(projectName string) error {
	return s.daemon.GUIStore.OpenProjectSettings(projectName)
}

func (s daemonGUI) ArchiveStaleConversations(horizon string) (*gui.AutoArchiveResult, error) {
	return s.daemon.GUIStore.ArchiveStaleConversations(horizon)
}

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
	defer s.daemon.invalidateActiveState()
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
	defer s.daemon.invalidateActiveState()
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
	defer s.daemon.invalidateActiveState()
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
	defer s.daemon.invalidateActiveState()
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

func (s daemonAccounts) SetTOTPSecret(email, secret string) error {
	return s.daemon.Keyring.SetTOTPSecret(email, secret)
}

func (s daemonAccounts) ExportAccounts() []keyring.AccountExport {
	return s.daemon.Keyring.ExportAccounts()
}

func (s daemonAccounts) BatchImportAccounts(items []keyring.BatchImportItem) (int, error) {
	defer s.daemon.invalidateActiveState()
	return s.daemon.Keyring.BatchImportAccounts(items)
}
