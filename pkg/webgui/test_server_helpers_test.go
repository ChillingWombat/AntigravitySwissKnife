package webgui

import (
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

func newTestServer(addr, socketPath string) *Server {
	store, err := keyring.NewStore("")
	if err != nil {
		panic(err)
	}
	guiStore, err := gui.NewStore("")
	if err != nil {
		panic(err)
	}
	customModelsStore, err := custommodels.NewStore("")
	if err != nil {
		panic(err)
	}
	enhancementsStore, err := enhancements.NewStore("")
	if err != nil {
		panic(err)
	}
	server, err := NewServer(addr, testBackend{
		accounts:     testAccountService{store: store},
		customModels: customModelsStore,
		enhancements: enhancementsStore,
		quota:        testQuotaService{store: store},
		gui:          &testGUIService{store: guiStore},
	}, ipc.NewClient(socketPath))
	if err != nil {
		panic(err)
	}
	return server
}

type testBackend struct {
	accounts     backend.AccountService
	customModels backend.CustomModelsService
	enhancements backend.EnhancementsService
	quota        backend.QuotaService
	gui          backend.GUIService
}

func (b testBackend) Accounts() backend.AccountService          { return b.accounts }
func (b testBackend) CustomModels() backend.CustomModelsService { return b.customModels }
func (b testBackend) Enhancements() backend.EnhancementsService { return b.enhancements }
func (b testBackend) Quota() backend.QuotaService               { return b.quota }
func (b testBackend) GUI() backend.GUIService                   { return b.gui }

type testQuotaService struct {
	store *keyring.Store
}

func (s testQuotaService) GetSummary(email string, refresh bool) (*quota.QuotaSummary, error) {
	if email == "" && s.store != nil {
		email = s.store.ActiveAccount()
	}
	return &quota.QuotaSummary{
		AccountEmail:  email,
		Models:        []quota.ModelQuota{},
		OverallHealth: core.StatusExhausted,
		LastPolled:    time.Now(),
	}, nil
}

func (s testQuotaService) GetFleet() *quota.FleetQuotaSummary {
	var accounts []*keyring.Account
	active := ""
	if s.store != nil {
		accounts = s.store.ListAccounts()
		active = s.store.ActiveAccount()
	}
	states := quota.BuildAccountQuotaStatesFromMapWithThresholds(
		accounts,
		nil,
		core.DefaultAutoSwitchThresholdFraction,
		core.DefaultAutoSwitchWeeklyThresholdFraction,
		false,
	)
	summary := quota.ComputeFleetSummary(states, active)
	return &summary
}

func (s testQuotaService) RefreshFleet() error { return nil }

type testGUIService struct {
	store *gui.Store
}

func (s *testGUIService) SetStore(store *gui.Store) { s.store = store }

func (s *testGUIService) GetConfig() gui.Config { return s.store.GetConfig() }

func (s *testGUIService) UpdateConfigPartial(rawJSON []byte) (*gui.GUIConfig, error) {
	return s.store.UpdateConfigPartial(rawJSON)
}

func (s *testGUIService) DetectProjects() ([]gui.ProjectItem, error) {
	return s.store.DetectProjects()
}

func (s *testGUIService) SetProjectColor(projectName, hexColor string) error {
	return s.store.SetProjectColor(projectName, hexColor)
}

func (s *testGUIService) DeleteProject(projectName string) error {
	return s.store.DeleteProject(projectName)
}

func (s *testGUIService) RemoveProjectColor(projectName string) error {
	return s.store.RemoveProjectColor(projectName)
}

func (s *testGUIService) UpdateProjectOrder(order []string) error {
	return s.store.UpdateProjectOrder(order)
}

func (s *testGUIService) Apply() (*gui.ApplyResult, error) { return s.store.Apply() }

func (s *testGUIService) GetArchivedProjects() ([]gui.ArchivedProjectItem, error) {
	return s.store.GetArchivedProjects()
}

func (s *testGUIService) ArchiveProject(projectName string) error {
	return s.store.ArchiveProject(projectName)
}

func (s *testGUIService) RestoreProject(projectName string) error {
	return s.store.RestoreProject(projectName)
}

func (s *testGUIService) ArchiveStaleConversations(horizon string) (*gui.AutoArchiveResult, error) {
	return s.store.ArchiveStaleConversations(horizon)
}

func (s *testGUIService) OpenProjectSettings(projectName string) error {
	return s.store.OpenProjectSettings(projectName)
}

type testAccountService struct {
	store *keyring.Store
}

func (s testAccountService) ListAccounts() []*keyring.Account {
	if store, err := keyring.NewStore(""); err == nil {
		return store.ListAccounts()
	}
	return s.store.ListAccounts()
}

func (s testAccountService) GetAccount(email string) (*keyring.Account, error) {
	if store, err := keyring.NewStore(""); err == nil {
		return store.GetAccount(email)
	}
	return s.store.GetAccount(email)
}

func (s testAccountService) ScanAccounts() ([]keyring.DiscoveredAccount, error) {
	return keyring.NewScanner(s.store).Scan()
}

func (s testAccountService) ImportAccount(req backend.AccountImport) (*keyring.Account, error) {
	account, err := s.store.ImportAccount(req.Email, req.RefreshToken, req.AccessToken, req.Label, req.TOTPSecret)
	if err != nil {
		return nil, err
	}
	if account != nil && (account.AccessToken != "" || account.RefreshToken != "") {
		_, _ = quota.PollAndCacheAccount(account, s.store)
		if refreshed, getErr := s.store.GetAccount(req.Email); getErr == nil {
			account = refreshed
		}
	}
	return account, nil
}

func (s testAccountService) UpdateAccount(req backend.AccountUpdate) (map[string]interface{}, error) {
	existing, _ := s.store.GetAccount(req.Email)
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
	prevActive := s.store.ActiveAccount()
	if err := s.store.UpdateAccountFull(req.Email, req.Label, req.PlanTier, req.Status, req.Priority, req.Notes, req.Password, req.TOTPSecret, req.RefreshToken, req.Credits, req.EnableCreditOverages, req.AllowClaudeGPT, req.SetActive); err != nil {
		return nil, err
	}
	if req.AccessToken != "" && req.AccessToken != "[REDACTED]" {
		if err := s.store.SetAccessToken(req.Email, req.AccessToken); err != nil {
			return nil, err
		}
	}
	account, _ := s.store.GetAccount(req.Email)
	var summary *quota.QuotaSummary
	if account != nil && (account.AccessToken != "" || account.RefreshToken != "") {
		summary, _ = quota.PollAndCacheAccount(account, s.store)
		if refreshed, getErr := s.store.GetAccount(req.Email); getErr == nil {
			account = refreshed
		}
	}
	if req.SetActive && account != nil {
		var allEmails []string
		for _, item := range s.store.ListAccounts() {
			allEmails = append(allEmails, item.Email)
		}
		_ = keyring.SyncAllSurfaces(account, allEmails, nil)
		_ = prevActive
	}
	result := map[string]interface{}{"success": true, "email": req.Email}
	if account != nil {
		result["plan_tier"] = account.PlanTier
		result["credits"] = account.Credits
		result["status"] = account.Status
		result["error_message"] = account.ErrorMessage
	}
	if summary != nil {
		result["quota"] = summary
	}
	return result, nil
}

func (s testAccountService) RemoveAccount(email string) (map[string]interface{}, error) {
	prevActive := s.store.ActiveAccount()
	if err := s.store.RemoveAccount(email); err != nil {
		return nil, err
	}
	active := s.store.ActiveAccount()
	result := map[string]interface{}{"success": true, "removed": email, "active_account": active}
	if active != "" && !strings.EqualFold(prevActive, active) {
		var allEmails []string
		for _, item := range s.store.ListAccounts() {
			allEmails = append(allEmails, item.Email)
		}
		if account, err := s.store.GetAccount(active); err == nil {
			_ = keyring.SyncAllSurfaces(account, allEmails, nil)
		}
	}
	return result, nil
}

func (s testAccountService) SwitchAccount(req backend.AccountSwitch) (backend.AccountSwitchResult, error) {
	store, err := keyring.NewStore("")
	if err != nil {
		return backend.AccountSwitchResult{}, err
	}
	if err := store.SetActiveAccount(req.Email); err != nil {
		return backend.AccountSwitchResult{}, err
	}
	account, err := store.GetAccount(req.Email)
	if err != nil {
		return backend.AccountSwitchResult{}, err
	}
	var allEmails []string
	for _, item := range store.ListAccounts() {
		allEmails = append(allEmails, item.Email)
	}
	targetApp := strings.TrimSpace(req.TargetApp)
	cfg, err := core.LoadConfig()
	if err != nil {
		return backend.AccountSwitchResult{}, err
	}
	if targetApp == "" || strings.EqualFold(targetApp, core.TargetAppAll) || cfg.GetMultiAppSyncMode() != core.MultiAppSyncModeIndividual {
		_ = keyring.SyncAllSurfaces(account, allEmails, nil)
		targetApp = core.TargetAppAll
	} else {
		_ = keyring.SyncSurface(targetApp, account, allEmails, nil)
	}
	if err := cfg.SetActiveAppAccount(targetApp, account.Email); err != nil {
		return backend.AccountSwitchResult{}, err
	}
	shouldRelaunch := true
	if req.RelaunchIDE != nil {
		shouldRelaunch = *req.RelaunchIDE
	}
	return backend.AccountSwitchResult{
		Switched:          true,
		Account:           account.Email,
		Success:           true,
		ActiveAccount:     account.Email,
		RelaunchIDE:       shouldRelaunch,
		TargetApp:         targetApp,
		MultiAppSyncMode:  cfg.GetMultiAppSyncMode(),
		ActiveAppAccounts: cfg.GetActiveAppAccounts(),
	}, nil
}

func (s testAccountService) SetTOTPSecret(email, secret string) error {
	return s.store.SetTOTPSecret(email, secret)
}

func (s testAccountService) ExportAccounts() []keyring.AccountExport {
	return s.store.ExportAccounts()
}

func (s testAccountService) BatchImportAccounts(items []keyring.BatchImportItem) (int, error) {
	return s.store.BatchImportAccounts(items)
}
