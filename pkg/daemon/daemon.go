package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/cache"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/process"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/totp"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/vault"
)

// Daemon coordinates background services, IPC JSON-RPC dispatch, and scheduler loops.
type Daemon struct {
	Config    *core.Config
	Keyring   *keyring.Store
	Profiles  *fingerprint.Store
	GUIStore  *gui.Store
	Shield    *process.Shield
	Inspector *cache.Inspector
	Pruner    *cache.Pruner
	TOTP      *totp.Engine
	Vault     *vault.Manager
	Server    *ipc.Server

	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	lastSwitchTime time.Time
	mu             sync.RWMutex

	quotaCache     map[string]*quota.QuotaSummary
	quotaCacheMu   sync.RWMutex
	isPollingFleet bool
	pollingMu      sync.Mutex
}

// NewDaemon initializes all subsystem stores and sets up JSON-RPC method handlers.
func NewDaemon(cfg *core.Config, socketPath string) (*Daemon, error) {
	if cfg == nil {
		var err error
		cfg, err = core.LoadConfig()
		if err != nil {
			cfg = core.DefaultConfig()
		}
	}

	keyringStore, err := keyring.NewStore("")
	if err != nil {
		return nil, fmt.Errorf("failed to init keyring store: %w", err)
	}

	fpStore, err := fingerprint.NewStore("")
	if err != nil {
		return nil, fmt.Errorf("failed to init fingerprint store: %w", err)
	}

	shield := process.NewShield(cfg.AntigravityProtectedPID)
	inspector := cache.NewInspector("", "")
	pruner := cache.NewPruner("")
	totpEngine := totp.NewEngine()
	guiStore, err := gui.NewStore("")
	if err != nil {
		return nil, fmt.Errorf("failed to init gui store: %w", err)
	}

	if socketPath == "" {
		socketPath = core.GetSocketPath()
	}
	server := ipc.NewServer(socketPath)

	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		Config:         cfg,
		Keyring:        keyringStore,
		Profiles:       fpStore,
		GUIStore:       guiStore,
		Shield:         shield,
		Inspector:      inspector,
		Pruner:         pruner,
		TOTP:           totpEngine,
		Vault:          vault.NewManager("", ""),
		Server:         server,
		ctx:            ctx,
		cancel:         cancel,
		lastSwitchTime: time.Now(),
		quotaCache:     quota.LoadQuotaCache(),
	}

	d.registerRPCHandlers()

	// Initial reconciliation of active running Antigravity account and cloud accounts
	_ = keyring.SyncStoreFromCloudAccountsDB(d.Keyring, "")
	var initialEmails []string
	for _, a := range d.Keyring.ListAccounts() {
		initialEmails = append(initialEmails, a.Email)
	}
	_, _ = d.Keyring.ReconcileActiveAccount(d.Config.AutoImportActiveAccount, initialEmails, d.Profiles)
	_ = keyring.SyncCloudAccountsAutoSwitch("", d.Config.AutoSwitchEnabled)

	return d, nil
}

func (d *Daemon) getQuotaSummaries() map[string]*quota.QuotaSummary {
	d.quotaCacheMu.RLock()
	defer d.quotaCacheMu.RUnlock()
	res := make(map[string]*quota.QuotaSummary, len(d.quotaCache))
	for k, v := range d.quotaCache {
		res[k] = v
	}
	return res
}

func (d *Daemon) setQuotaSummary(email string, sum *quota.QuotaSummary) {
	if email == "" || sum == nil {
		return
	}
	norm := strings.ToLower(strings.TrimSpace(email))
	d.quotaCacheMu.Lock()
	if d.quotaCache == nil {
		d.quotaCache = make(map[string]*quota.QuotaSummary)
	}
	d.quotaCache[norm] = sum
	cacheCopy := make(map[string]*quota.QuotaSummary, len(d.quotaCache))
	for k, v := range d.quotaCache {
		cacheCopy[k] = v
	}
	d.quotaCacheMu.Unlock()
	_ = quota.SaveQuotaCache(cacheCopy)
}

func (d *Daemon) updateQuotaSummaries(summaries map[string]*quota.QuotaSummary) {
	if len(summaries) == 0 {
		return
	}
	d.quotaCacheMu.Lock()
	if d.quotaCache == nil {
		d.quotaCache = make(map[string]*quota.QuotaSummary)
	}
	for k, v := range summaries {
		if v != nil {
			d.quotaCache[strings.ToLower(strings.TrimSpace(k))] = v
		}
	}
	cacheCopy := make(map[string]*quota.QuotaSummary, len(d.quotaCache))
	for k, v := range d.quotaCache {
		cacheCopy[k] = v
	}
	d.quotaCacheMu.Unlock()
	_ = quota.SaveQuotaCache(cacheCopy)
}

func (d *Daemon) removeQuotaSummary(email string) {
	if email == "" {
		return
	}
	norm := strings.ToLower(strings.TrimSpace(email))
	d.quotaCacheMu.Lock()
	if d.quotaCache != nil {
		delete(d.quotaCache, norm)
	}
	d.quotaCacheMu.Unlock()
	_ = quota.DeleteQuotaCacheEntry(email)
}

func (d *Daemon) syncActiveAccountSurfaces(acc *keyring.Account) {
	if acc == nil {
		return
	}
	var allEmails []string
	for _, a := range d.Keyring.ListAccounts() {
		allEmails = append(allEmails, a.Email)
	}
	_ = keyring.SyncAllSurfaces(acc, allEmails, d.Profiles)
	_, _ = gui.NewInjector(0).RefreshUserStatus()
}

func (d *Daemon) triggerQuotaRefreshAsync() {
	d.pollingMu.Lock()
	if d.isPollingFleet {
		d.pollingMu.Unlock()
		return
	}
	d.isPollingFleet = true
	d.pollingMu.Unlock()

	go func() {
		defer func() {
			d.pollingMu.Lock()
			d.isPollingFleet = false
			d.pollingMu.Unlock()
		}()

		accounts := d.Keyring.ListAccounts()
		if len(accounts) == 0 {
			return
		}

		active := d.Keyring.ActiveAccount()
		oldToken := ""
		if activeAcc, _ := d.Keyring.GetAccount(active); activeAcc != nil {
			oldToken = activeAcc.AccessToken
		}

		summaries := quota.PollFleetAccounts(accounts, d.Keyring)
		if len(summaries) > 0 {
			d.updateQuotaSummaries(summaries)
		}

		if active != "" {
			if updatedAcc, _ := d.Keyring.GetAccount(active); updatedAcc != nil && updatedAcc.AccessToken != "" && updatedAcc.AccessToken != oldToken {
				d.syncActiveAccountSurfaces(updatedAcc)
			}
		}
	}()
}

func (d *Daemon) registerRPCHandlers() {
	// 1. Status
	statusHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		procs, _ := d.Shield.FindAntigravityProcesses()
		var hostPID int
		if len(procs) > 0 {
			hostPID = procs[0].PID
		}
		if d.Shield.ProtectedPID() > 0 {
			hostPID = d.Shield.ProtectedPID()
		}

		// Reconcile active account with running Antigravity sessions across all surfaces
		var allEmails []string
		for _, a := range d.Keyring.ListAccounts() {
			allEmails = append(allEmails, a.Email)
		}
		_, _ = d.Keyring.ReconcileActiveAccount(d.Config.AutoImportActiveAccount, allEmails, d.Profiles)

		accounts := d.Keyring.ListAccounts()
		return map[string]interface{}{
			"daemon_running":              true,
			"daemon_pid":                  os.Getpid(),
			"pid":                         os.Getpid(),
			"version":                     core.AppVersion,
			"active_account":              d.Keyring.ActiveAccount(),
			"total_accounts":              len(accounts),
			"antigravity_running":         len(procs) > 0,
			"antigravity_pid":             hostPID,
			"running_antigravity_account": keyring.ResolveRunningAntigravityAccount("", ""),
		}, nil
	}
	d.Server.Register("swiss.getStatus", statusHandler)
	d.Server.Register("status.get", statusHandler)

	// 2. List Accounts
	listAccountsHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		return d.Keyring.ListAccounts(), nil
	}
	d.Server.Register("swiss.listAccounts", listAccountsHandler)
	d.Server.Register("accounts.list", listAccountsHandler)

	// 2b. Scan Local Accounts
	d.Server.Register("swiss.scanLocalAccounts", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		scanner := keyring.NewScanner(d.Keyring)
		res, err := scanner.Scan()
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return res, nil
	})

	// 2c. Import Account (Email, Refresh Token, Access Token, Label, TOTP)
	d.Server.Register("swiss.importAccount", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email        string `json:"email"`
			RefreshToken string `json:"refresh_token"`
			AccessToken  string `json:"access_token"`
			Label        string `json:"label"`
			TOTPSecret   string `json:"totp_secret"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Email == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "missing required 'email'"}
		}
		acc, err := d.Keyring.ImportAccount(p.Email, p.RefreshToken, p.AccessToken, p.Label, p.TOTPSecret)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		_, _ = d.Profiles.GetOrCreateProfile(p.Email)
		if acc != nil && (acc.AccessToken != "" || acc.RefreshToken != "") {
			summary, pollErr := quota.PollAndCacheAccount(acc, d.Keyring)
			if pollErr == nil && summary != nil {
				d.setQuotaSummary(p.Email, summary)
			}
		}
		return acc, nil
	})

	// 2d. Update Account Info
	updateHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email                string  `json:"email"`
			Label                string  `json:"label"`
			PlanTier             string  `json:"plan_tier"`
			Status               string  `json:"status"`
			Priority             string  `json:"priority"`
			Notes                string  `json:"notes"`
			Password             string  `json:"password"`
			TOTPSecret           string  `json:"totp_secret"`
			RefreshToken         string  `json:"refresh_token"`
			AccessToken          string  `json:"access_token"`
			Credits              float64 `json:"credits"`
			EnableCreditOverages bool    `json:"enable_credit_overages"`
			AllowClaudeGPT       bool    `json:"allow_claude_gpt"`
			SetActive            bool    `json:"set_active"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Email == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "missing required 'email'"}
		}
		if p.TOTPSecret != "" && !totp.ValidateSecret(p.TOTPSecret) {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: core.ErrInvalidSecret.Error()}
		}
		prevActive := d.Keyring.ActiveAccount()
		if err := d.Keyring.UpdateAccountFull(p.Email, p.Label, p.PlanTier, p.Status, p.Priority, p.Notes, p.Password, p.TOTPSecret, p.RefreshToken, p.Credits, p.EnableCreditOverages, p.AllowClaudeGPT, p.SetActive); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		if p.AccessToken != "" {
			_ = d.Keyring.SetAccessToken(p.Email, p.AccessToken)
		}
		if p.SetActive {
			var allEmails []string
			for _, a := range d.Keyring.ListAccounts() {
				allEmails = append(allEmails, a.Email)
			}
			if acc, _ := d.Keyring.GetAccount(p.Email); acc != nil {
				_ = keyring.SyncAllSurfaces(acc, allEmails, d.Profiles)
				_ = d.Keyring.UpdateAccountTokensWithExpiry(acc.Email, acc.AccessToken, acc.RefreshToken, acc.TokenExpiry)
				if !strings.EqualFold(prevActive, p.Email) && d.Shield != nil && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
					_ = gui.NewInjector(0).CaptureActiveConversationPath()
					go func() {
						time.Sleep(200 * time.Millisecond)
						_ = d.Shield.RelaunchHostIDE()
					}()
				} else {
					_, _ = gui.NewInjector(0).RefreshUserStatus()
				}
			}
		}

		acc, _ := d.Keyring.GetAccount(p.Email)
		var summary *quota.QuotaSummary
		if acc != nil && (acc.AccessToken != "" || acc.RefreshToken != "") {
			oldTok := acc.AccessToken
			var pollErr error
			summary, pollErr = quota.PollAndCacheAccount(acc, d.Keyring)
			if pollErr == nil && summary != nil {
				d.setQuotaSummary(p.Email, summary)
				if strings.EqualFold(p.Email, d.Keyring.ActiveAccount()) && acc.AccessToken != "" && acc.AccessToken != oldTok {
					d.syncActiveAccountSurfaces(acc)
				}
			} else {
				d.removeQuotaSummary(p.Email)
				d.triggerQuotaRefreshAsync()
			}
		}

		res := map[string]interface{}{"success": true, "email": p.Email}
		if acc != nil {
			res["plan_tier"] = acc.PlanTier
			res["credits"] = acc.Credits
		}
		if summary != nil {
			res["quota"] = summary
		}
		return res, nil
	}
	d.Server.Register("swiss.updateAccount", updateHandler)
	d.Server.Register("accounts.update", updateHandler)

	// 2e. Remove Account
	removeHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Email == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "missing required 'email'"}
		}
		prevActive := d.Keyring.ActiveAccount()
		if err := d.Keyring.RemoveAccount(p.Email); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		d.removeQuotaSummary(p.Email)
		newActive := d.Keyring.ActiveAccount()
		if newActive != "" && !strings.EqualFold(prevActive, newActive) {
			var allEmails []string
			for _, a := range d.Keyring.ListAccounts() {
				allEmails = append(allEmails, a.Email)
			}
			if acc, _ := d.Keyring.GetAccount(newActive); acc != nil {
				_ = keyring.SyncAllSurfaces(acc, allEmails, d.Profiles)
				_ = d.Keyring.UpdateAccountTokensWithExpiry(acc.Email, acc.AccessToken, acc.RefreshToken, acc.TokenExpiry)
				if d.Shield != nil && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
					_ = gui.NewInjector(0).CaptureActiveConversationPath()
					go func() {
						time.Sleep(200 * time.Millisecond)
						_ = d.Shield.RelaunchHostIDE()
					}()
				}
			}
		}
		return map[string]interface{}{"success": true, "removed": p.Email}, nil
	}
	d.Server.Register("swiss.removeAccount", removeHandler)
	d.Server.Register("accounts.remove", removeHandler)

	// 2f. Batch Import Accounts (JSON format)
	batchImportHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var items []keyring.BatchImportItem
		if err := json.Unmarshal(params, &items); err != nil {
			var wrapper struct {
				Accounts json.RawMessage `json:"accounts"`
			}
			if err2 := json.Unmarshal(params, &wrapper); err2 == nil && len(wrapper.Accounts) > 0 {
				if err3 := json.Unmarshal(wrapper.Accounts, &items); err3 != nil {
					var accMap map[string]keyring.BatchImportItem
					if err4 := json.Unmarshal(wrapper.Accounts, &accMap); err4 == nil {
						for em, it := range accMap {
							if it.Email == "" && it.ID == "" {
								it.Email = em
							}
							items = append(items, it)
						}
					}
				}
			} else {
				var single keyring.BatchImportItem
				if err5 := json.Unmarshal(params, &single); err5 == nil && (single.Email != "" || single.ID != "") {
					items = []keyring.BatchImportItem{single}
				} else {
					return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "invalid accounts payload: " + err.Error()}
				}
			}
		}
		if len(items) == 0 {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "no valid account entries found in JSON"}
		}
		count, err := d.Keyring.BatchImportAccounts(items)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		for _, item := range items {
			em := item.Email
			if em == "" {
				em = item.ID
			}
			if em != "" {
				_, _ = d.Profiles.GetOrCreateProfile(em)
			}
		}
		return map[string]interface{}{"success": true, "imported": count, "message": fmt.Sprintf("Successfully imported %d accounts", count)}, nil
	}
	d.Server.Register("swiss.batchImportAccounts", batchImportHandler)
	d.Server.Register("accounts.batchImport", batchImportHandler)

	// 2g. Export Accounts (JSON format)
	exportHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		return d.Keyring.ExportAccounts(), nil
	}
	d.Server.Register("swiss.exportAccounts", exportHandler)
	d.Server.Register("accounts.export", exportHandler)

	// 3. Switch Account
	switchHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email       string `json:"email"`
			RelaunchIDE *bool  `json:"relaunch_ide"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Email == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "missing required 'email' parameter"}
		}

		if err := d.Keyring.SetActiveAccount(p.Email); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}

		d.mu.Lock()
		d.lastSwitchTime = time.Now()
		d.mu.Unlock()

		// Collect all account emails for old/active tracking
		var allEmails []string
		for _, a := range d.Keyring.ListAccounts() {
			allEmails = append(allEmails, a.Email)
		}

		shouldRelaunch := true
		if p.RelaunchIDE != nil {
			shouldRelaunch = *p.RelaunchIDE
		}

		// Synchronize across Antigravity 2.0 Desktop, Antigravity CLI (agy), and VS Code extension
		if acc, _ := d.Keyring.GetAccount(p.Email); acc != nil {
			_ = keyring.SyncAllSurfaces(acc, allEmails, d.Profiles)
			_ = d.Keyring.UpdateAccountTokensWithExpiry(acc.Email, acc.AccessToken, acc.RefreshToken, acc.TokenExpiry)
			if !shouldRelaunch {
				_, _ = gui.NewInjector(0).RefreshUserStatus()
			}
		}

		if shouldRelaunch && d.Shield != nil && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
			_ = gui.NewInjector(0).CaptureActiveConversationPath()
			go func() {
				time.Sleep(200 * time.Millisecond)
				_ = d.Shield.RelaunchHostIDE()
			}()
		}

		return map[string]interface{}{
			"switched":       true,
			"account":        p.Email,
			"success":        true,
			"active_account": p.Email,
			"relaunch_ide":   shouldRelaunch,
		}, nil
	}
	d.Server.Register("swiss.switchAccount", switchHandler)
	d.Server.Register("accounts.switch", switchHandler)

	// Graceful Host IDE Relaunch
	relaunchHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		if d.Shield == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: "shield manager not initialized"}
		}
		if os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" {
			return map[string]interface{}{
				"success": true,
				"message": "Antigravity host IDE relaunch skipped (dry run)",
			}, nil
		}
		if err := d.Shield.RelaunchHostIDE(); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{
			"success": true,
			"message": "Antigravity host IDE relaunch initiated",
		}, nil
	}
	d.Server.Register("swiss.relaunchIDE", relaunchHandler)
	d.Server.Register("desktop.relaunch", relaunchHandler)

	// 4. Set TOTP Secret
	d.Server.Register("swiss.setTOTPSecret", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email  string `json:"email"`
			Secret string `json:"secret"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Email == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "missing required 'email'"}
		}

		if p.Secret != "" && !totp.ValidateSecret(p.Secret) {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: core.ErrInvalidSecret.Error()}
		}

		if err := d.Keyring.SetTOTPSecret(p.Email, p.Secret); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{"success": true}, nil
	})

	// 5. Get TOTP Code
	d.Server.Register("swiss.getTOTPCode", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "invalid params"}
		}

		target := p.Email
		if target == "" {
			target = d.Keyring.ActiveAccount()
		}

		acc, err := d.Keyring.GetAccount(target)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}

		now := time.Now()
		countdown := d.TOTP.GetCountdown(now)

		if acc.TOTPSecret == "" {
			return map[string]interface{}{
				"code":              "------",
				"remaining_seconds": 0,
				"progress_fraction": 0.0,
				"has_totp":          false,
			}, nil
		}

		code, err := d.TOTP.GenerateCode(acc.TOTPSecret, now)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}

		return map[string]interface{}{
			"code":              code,
			"remaining_seconds": countdown.RemainingSeconds,
			"progress_fraction": countdown.ProgressFraction,
			"has_totp":          true,
		}, nil
	})

	// 6. Device Profile: Get & List
	d.Server.Register("swiss.getFingerprofile", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email string `json:"email"`
		}
		_ = json.Unmarshal(params, &p)
		target := p.Email
		if target == "" {
			target = d.Keyring.ActiveAccount()
		}

		prof, err := d.Profiles.GetOrCreateProfile(target)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return prof, nil
	})

	d.Server.Register("swiss.listFingerprofiles", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		for _, acc := range d.Keyring.ListAccounts() {
			if acc.Email != "" {
				_, _ = d.Profiles.GetOrCreateProfile(acc.Email)
			}
		}
		return d.Profiles.ListProfilesSlice(), nil
	})

	// 7. Device Profile: Set
	d.Server.Register("swiss.setFingerprofile", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var raw struct {
			Email            string                     `json:"email"`
			AccountEmail     string                     `json:"account_email"`
			Profile          *fingerprint.DeviceProfile `json:"profile"`
			MachineID        string                     `json:"machine_id"`
			UpdaterID        string                     `json:"updater_id"`
			InstallationID   string                     `json:"installation_id"`
			InstallationUUID string                     `json:"installation_uuid"`
		}
		if err := json.Unmarshal(params, &raw); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "invalid params"}
		}
		targetEmail := strings.TrimSpace(raw.Email)
		if targetEmail == "" {
			targetEmail = strings.TrimSpace(raw.AccountEmail)
		}
		var prof fingerprint.DeviceProfile
		if raw.Profile != nil {
			prof = *raw.Profile
			if targetEmail == "" {
				targetEmail = strings.TrimSpace(prof.AccountEmail)
			}
		} else {
			prof = fingerprint.DeviceProfile{
				AccountEmail:     targetEmail,
				MachineID:        strings.TrimSpace(raw.MachineID),
				UpdaterID:        strings.TrimSpace(raw.UpdaterID),
				InstallationID:   strings.TrimSpace(raw.InstallationID),
				InstallationUUID: strings.TrimSpace(raw.InstallationUUID),
			}
		}
		if targetEmail == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "missing account email"}
		}

		if err := d.Profiles.SetProfile(targetEmail, &prof); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: err.Error()}
		}
		if strings.EqualFold(targetEmail, d.Keyring.ActiveAccount()) {
			_ = keyring.SyncHardwareProfile(targetEmail, d.Profiles)
		}
		return map[string]interface{}{"success": true}, nil
	})

	// 8. Cache: Scan
	d.Server.Register("swiss.scanCache", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			MinAgeDays float64 `json:"min_age_days"`
		}
		_ = json.Unmarshal(params, &p)
		if p.MinAgeDays <= 0 {
			p.MinAgeDays = 3.0
		}

		bd, err := d.Inspector.ScanBreakdown(p.MinAgeDays)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return bd, nil
	})

	// 9. Cache: Prune
	d.Server.Register("swiss.pruneCache", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var opts cache.PruneOptions
		if err := json.Unmarshal(params, &opts); err != nil {
			opts.MinAgeDays = 3.0
			opts.PruneScratch = true
			opts.PruneSteps = true
			opts.PruneTasks = true
		}

		res, err := d.Pruner.Prune(opts)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return res, nil
	})

	// 10. Rule Config: Get
	getRuleConfigHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		d.mu.RLock()
		defer d.mu.RUnlock()
		prefNative := d.Config.PreferredNativeModel
		if prefNative == "" {
			prefNative = "gemini"
		}
		activePoll := d.Config.ActivePollingIntervalSec
		if activePoll <= 0 {
			activePoll = 120
		}
		standbyPoll := d.Config.StandbyPollingIntervalSec
		if standbyPoll <= 0 {
			standbyPoll = 900
		}
		jitter := d.Config.StandbyRandomJitterSec
		if jitter <= 0 {
			jitter = 30
		}
		geminiReasoning := d.Config.DefaultGeminiReasoningLevel
		if geminiReasoning == "" {
			geminiReasoning = "high"
		}
		defaultGemini := d.Config.DefaultGeminiModel
		if defaultGemini == "" {
			defaultGemini = "gemini-3.8-flash-high"
		}
		defaultNonGemini := d.Config.DefaultNonGeminiModel
		if defaultNonGemini == "" {
			defaultNonGemini = "claude-opus-4-6"
		}
		switchMode := d.Config.SwitchMode
		if switchMode == "" {
			switchMode = core.DefaultSwitchMode
		} else {
			switchMode = quota.NormalizeSwitchMode(switchMode)
		}
		threshWeekly := d.Config.AutoSwitchWeeklyThreshold
		if threshWeekly <= 0 {
			threshWeekly = core.DefaultAutoSwitchWeeklyThresholdFraction
		}
		return map[string]interface{}{
			"auto_switch_enabled":              d.Config.AutoSwitchEnabled,
			"auto_switch_threshold":            d.Config.AutoSwitchThreshold,
			"auto_switch_weekly_threshold":     threshWeekly,
			"switch_mode":                      switchMode,
			"polling_interval_seconds":         d.Config.PollingIntervalSec,
			"active_polling_interval_seconds":  activePoll,
			"standby_polling_interval_seconds": standbyPoll,
			"standby_random_jitter_seconds":    jitter,
			"warmup_enabled":                   d.Config.WarmupEnabled,
			"warmup_lead_time_seconds":         d.Config.WarmupLeadTimeSec,
			"preferred_native_model":           prefNative,
			"allow_ai_credits_usage":           d.Config.AllowAICreditsUsage,
			"allow_non_gemini_native_models":   d.Config.AllowNonGeminiNativeModels,
			"model_source_hierarchy":           d.Config.ModelSourceHierarchy,
			"default_gemini_model":             defaultGemini,
			"default_custom_model":             d.Config.DefaultCustomModel,
			"default_non_gemini_model":         defaultNonGemini,
			"default_gemini_reasoning_level":   geminiReasoning,
			"auto_import_active_account":       d.Config.AutoImportActiveAccount,
		}, nil
	}
	d.Server.Register("swiss.getRuleConfig", getRuleConfigHandler)
	d.Server.Register("rules.get_config", getRuleConfigHandler)

	// 11. Rule Config: Set
	setRuleConfigHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			AutoSwitchEnabled           *bool     `json:"auto_switch_enabled"`
			AutoSwitchThreshold         *float64  `json:"auto_switch_threshold"`
			AutoSwitchWeeklyThreshold   *float64  `json:"auto_switch_weekly_threshold"`
			SwitchMode                  *string   `json:"switch_mode"`
			PollingIntervalSec          *int      `json:"polling_interval_seconds"`
			ActivePollingIntervalSec    *int      `json:"active_polling_interval_seconds"`
			StandbyPollingIntervalSec   *int      `json:"standby_polling_interval_seconds"`
			StandbyRandomJitterSec      *int      `json:"standby_random_jitter_seconds"`
			WarmupEnabled               *bool     `json:"warmup_enabled"`
			WarmupLeadTimeSec           *float64  `json:"warmup_lead_time_seconds"`
			PreferredNativeModel        *string   `json:"preferred_native_model"`
			AllowAICreditsUsage         *bool     `json:"allow_ai_credits_usage"`
			AllowNonGeminiNativeModels  *bool     `json:"allow_non_gemini_native_models"`
			ModelSourceHierarchy        *[]string `json:"model_source_hierarchy"`
			DefaultGeminiModel          *string   `json:"default_gemini_model"`
			DefaultCustomModel          *string   `json:"default_custom_model"`
			DefaultNonGeminiModel       *string   `json:"default_non_gemini_model"`
			DefaultGeminiReasoningLevel *string   `json:"default_gemini_reasoning_level"`
			AutoImportActiveAccount     *bool     `json:"auto_import_active_account"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: err.Error()}
		}

		d.mu.Lock()
		if p.AutoSwitchEnabled != nil {
			d.Config.AutoSwitchEnabled = *p.AutoSwitchEnabled
			_ = keyring.SyncCloudAccountsAutoSwitch("", *p.AutoSwitchEnabled)
		}
		if p.AutoSwitchThreshold != nil {
			d.Config.AutoSwitchThreshold = *p.AutoSwitchThreshold
		}
		if p.AutoSwitchWeeklyThreshold != nil {
			d.Config.AutoSwitchWeeklyThreshold = *p.AutoSwitchWeeklyThreshold
		}
		if p.SwitchMode != nil {
			d.Config.SwitchMode = quota.NormalizeSwitchMode(*p.SwitchMode)
		}
		if p.PollingIntervalSec != nil {
			d.Config.PollingIntervalSec = *p.PollingIntervalSec
		}
		if p.ActivePollingIntervalSec != nil {
			d.Config.ActivePollingIntervalSec = *p.ActivePollingIntervalSec
		}
		if p.StandbyPollingIntervalSec != nil {
			d.Config.StandbyPollingIntervalSec = *p.StandbyPollingIntervalSec
		}
		if p.StandbyRandomJitterSec != nil {
			d.Config.StandbyRandomJitterSec = *p.StandbyRandomJitterSec
		}
		if p.WarmupEnabled != nil {
			d.Config.WarmupEnabled = *p.WarmupEnabled
		}
		if p.WarmupLeadTimeSec != nil {
			d.Config.WarmupLeadTimeSec = *p.WarmupLeadTimeSec
		}
		if p.PreferredNativeModel != nil {
			d.Config.PreferredNativeModel = *p.PreferredNativeModel
		}
		if p.AllowAICreditsUsage != nil {
			d.Config.AllowAICreditsUsage = *p.AllowAICreditsUsage
		}
		if p.AllowNonGeminiNativeModels != nil {
			d.Config.AllowNonGeminiNativeModels = *p.AllowNonGeminiNativeModels
		}
		if p.ModelSourceHierarchy != nil {
			d.Config.ModelSourceHierarchy = *p.ModelSourceHierarchy
		}
		if p.DefaultGeminiModel != nil {
			d.Config.DefaultGeminiModel = *p.DefaultGeminiModel
		}
		if p.DefaultCustomModel != nil {
			d.Config.DefaultCustomModel = *p.DefaultCustomModel
		}
		if p.DefaultNonGeminiModel != nil {
			d.Config.DefaultNonGeminiModel = *p.DefaultNonGeminiModel
		}
		if p.DefaultGeminiReasoningLevel != nil {
			d.Config.DefaultGeminiReasoningLevel = *p.DefaultGeminiReasoningLevel
		}
		if p.AutoImportActiveAccount != nil {
			d.Config.AutoImportActiveAccount = *p.AutoImportActiveAccount
		}
		_ = d.Config.Save()
		d.mu.Unlock()

		return map[string]interface{}{"success": true}, nil
	}
	d.Server.Register("swiss.setRuleConfig", setRuleConfigHandler)
	d.Server.Register("rules.set_config", setRuleConfigHandler)

	// 11b. Multi-Surface Antigravity Session Inspector
	d.Server.Register("swiss.getSurfaces", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		home, _ := os.UserHomeDir()
		configDir := filepath.Join(home, ".config", "Antigravity")
		if custom := os.Getenv("ANTIGRAVITY_CONFIG_DIR"); custom != "" {
			configDir = custom
		}
		return map[string]interface{}{
			"surfaces":               keyring.DetectAllSurfaces(home, configDir),
			"active_surface_account": keyring.ResolveRunningAntigravityAccount(home, configDir),
			"priority_sequence":      []string{"Antigravity 2.0 Desktop", "Antigravity VS Code Extension", "Antigravity CLI"},
		}, nil
	})

	// 12. Quota Summary
	getQuotaSummaryHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email   string `json:"email"`
			Refresh bool   `json:"refresh"`
		}
		if len(params) > 0 {
			_ = json.Unmarshal(params, &p)
		}
		target := strings.TrimSpace(p.Email)
		if target == "" {
			target = d.Keyring.ActiveAccount()
		}
		normTarget := strings.ToLower(target)

		if !p.Refresh {
			d.quotaCacheMu.RLock()
			cachedSum := d.quotaCache[normTarget]
			d.quotaCacheMu.RUnlock()
			if cachedSum != nil {
				return *cachedSum, nil
			}

			diskCache := quota.LoadQuotaCache()
			if cached, ok := diskCache[normTarget]; ok && cached != nil {
				d.setQuotaSummary(target, cached)
				return *cached, nil
			}
		}

		acc, _ := d.Keyring.GetAccount(target)
		now := time.Now()
		if acc != nil && (acc.AccessToken != "" || acc.RefreshToken != "") {
			oldTok := acc.AccessToken
			summary, err := quota.PollAndCacheAccount(acc, d.Keyring)
			if err == nil && summary != nil {
				d.setQuotaSummary(target, summary)
				if strings.EqualFold(target, d.Keyring.ActiveAccount()) && acc.AccessToken != "" && acc.AccessToken != oldTok {
					d.syncActiveAccountSurfaces(acc)
				}
				return *summary, nil
			}
		}
		if !p.Refresh {
			d.quotaCacheMu.RLock()
			cachedSum := d.quotaCache[normTarget]
			d.quotaCacheMu.RUnlock()
			if cachedSum != nil {
				return *cachedSum, nil
			}
		}
		return quota.QuotaSummary{
			AccountEmail:  target,
			Models:        []quota.ModelQuota{},
			MinFraction:   0.0,
			OverallHealth: core.StatusExhausted,
			LastPolled:    now,
		}, nil
	}
	d.Server.Register("swiss.getQuotaSummary", getQuotaSummaryHandler)
	d.Server.Register("quota.get_summary", getQuotaSummaryHandler)

	// 13. Fleet Quota & Per-Account Horizon States
	d.Server.Register("swiss.getFleetQuota", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var allEmails []string
		for _, a := range d.Keyring.ListAccounts() {
			allEmails = append(allEmails, a.Email)
		}
		_, _ = d.Keyring.ReconcileActiveAccount(d.Config.AutoImportActiveAccount, allEmails, d.Profiles)

		accounts := d.Keyring.ListAccounts()
		active := d.Keyring.ActiveAccount()
		summaries := d.getQuotaSummaries()

		if len(summaries) == 0 && len(accounts) > 0 {
			d.triggerQuotaRefreshAsync()
		}

		d.mu.RLock()
		thresh := d.Config.AutoSwitchThreshold
		threshWeekly := d.Config.AutoSwitchWeeklyThreshold
		d.mu.RUnlock()
		if thresh <= 0 {
			thresh = core.DefaultAutoSwitchThresholdFraction
		}
		if threshWeekly <= 0 {
			threshWeekly = core.DefaultAutoSwitchWeeklyThresholdFraction
		}
		states := quota.BuildAccountQuotaStatesFromMapWithThresholds(accounts, summaries, thresh, threshWeekly)
		summary := quota.ComputeFleetSummary(states, active)
		return summary, nil
	})

	d.Server.Register("swiss.refreshFleetQuota", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		d.triggerQuotaRefreshAsync()
		return map[string]interface{}{"status": "refresh_started"}, nil
	})

	// 14. GUI Improvements - Config
	d.Server.Register("swiss.getGUIConfig", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		return d.GUIStore.GetConfig(), nil
	})

	d.Server.Register("swiss.setGUIConfig", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var cfg gui.Config
		if err := json.Unmarshal(params, &cfg); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: err.Error()}
		}
		if err := d.GUIStore.UpdateConfig(&cfg); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return d.GUIStore.GetConfig(), nil
	})

	// 15. GUI Improvements - Projects
	d.Server.Register("swiss.getGUIProjects", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		projects, err := d.GUIStore.DetectProjects()
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return projects, nil
	})

	d.Server.Register("swiss.setGUIProjectColor", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Name == "" || p.Color == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "name and color required"}
		}
		if err := d.GUIStore.SetProjectColor(p.Name, p.Color); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{"success": true, "name": p.Name, "color": p.Color}, nil
	})

	d.Server.Register("swiss.removeGUIProjectColor", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Name == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "name required"}
		}
		if err := d.GUIStore.RemoveProjectColor(p.Name); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		factoryColors := gui.FactoryProjectColors()
		factoryColor, isFactory := factoryColors[p.Name]
		return map[string]interface{}{
			"success":          true,
			"name":             p.Name,
			"reset_to_factory": isFactory,
			"color":            factoryColor,
		}, nil
	})

	d.Server.Register("swiss.deleteGUIProject", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Name == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "name required"}
		}
		if err := d.GUIStore.DeleteProject(p.Name); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{"success": true, "name": p.Name}, nil
	})

	d.Server.Register("swiss.updateGUIProjectOrder", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Order []string `json:"order"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: err.Error()}
		}
		if err := d.GUIStore.UpdateProjectOrder(p.Order); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{"success": true, "order": p.Order}, nil
	})

	d.Server.Register("swiss.applyGUIStyles", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		res, err := d.GUIStore.Apply()
		if err != nil && res == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return res, nil
	})

	d.Server.Register("swiss.installDesktopLoader", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		res, err := d.GUIStore.InstallDesktopLoader()
		if err != nil && res == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return res, nil
	})

	d.Server.Register("swiss.restoreFactoryDefaults", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		res, err := d.GUIStore.RestoreFactoryDefaults()
		if err != nil && res == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return res, nil
	})

	d.Server.Register("swiss.getDesktopStatus", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		return d.GUIStore.GetDesktopStatus(), nil
	})
}

// Start launches the IPC server and background scheduler.
func (d *Daemon) Start() error {
	if err := d.Server.Start(); err != nil {
		return err
	}
	d.wg.Add(1)
	go d.schedulerLoop()
	return nil
}

// Stop gracefully shuts down all services.
func (d *Daemon) Stop() error {
	d.cancel()
	_ = d.Server.Stop()
	d.wg.Wait()
	return nil
}

func (d *Daemon) schedulerLoop() {
	defer d.wg.Done()

	d.mu.RLock()
	activeInterval := time.Duration(d.Config.ActivePollingIntervalSec) * time.Second
	standbyInterval := time.Duration(d.Config.StandbyPollingIntervalSec) * time.Second
	jitterSec := d.Config.StandbyRandomJitterSec
	d.mu.RUnlock()

	if activeInterval < 10*time.Second {
		activeInterval = 120 * time.Second
	}
	if standbyInterval < 30*time.Second {
		standbyInterval = 900 * time.Second
	}
	if jitterSec <= 0 {
		jitterSec = 30
	}

	activeTicker := time.NewTicker(activeInterval)
	defer activeTicker.Stop()

	standbyTicker := time.NewTicker(standbyInterval)
	defer standbyTicker.Stop()

	vaultTicker := time.NewTicker(60 * time.Second)
	defer vaultTicker.Stop()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Warm up fleet quota cache asynchronously in background on startup
	d.triggerQuotaRefreshAsync()

	// Initial conversation vault sync on daemon startup if enabled
	d.mu.RLock()
	initVaultEnabled := d.Config.ConversationVaultEnabled
	d.mu.RUnlock()
	if initVaultEnabled && d.Vault != nil {
		_, _ = d.Vault.Sync()
	}

	for {
		select {
		case <-d.ctx.Done():
			return

		case <-vaultTicker.C:
			d.mu.RLock()
			vaultOn := d.Config.ConversationVaultEnabled
			d.mu.RUnlock()
			if vaultOn && d.Vault != nil {
				_, _ = d.Vault.Sync()
			}

		case <-activeTicker.C:
			// 1. Frequently refresh active account quota (e.g. every 2m)
			_ = gui.NewInjector(0).CaptureActiveConversationPath()
			var tickEmails []string
			for _, a := range d.Keyring.ListAccounts() {
				tickEmails = append(tickEmails, a.Email)
			}
			_, _ = d.Keyring.ReconcileActiveAccount(d.Config.AutoImportActiveAccount, tickEmails, d.Profiles)
			active := d.Keyring.ActiveAccount()
			if active != "" {
				if acc, _ := d.Keyring.GetAccount(active); acc != nil {
					oldTok := acc.AccessToken
					sum, err := quota.PollAccountLiveQuota(acc)
					if err == nil && sum != nil {
						d.setQuotaSummary(active, sum)
						tokenChanged := (acc.AccessToken != "" && acc.AccessToken != oldTok)
						if acc.AccessToken != "" {
							_ = d.Keyring.UpdateAccountTokensWithExpiry(active, acc.AccessToken, acc.RefreshToken, acc.TokenExpiry)
						}
						if tokenChanged {
							d.syncActiveAccountSurfaces(acc)
						}

						d.mu.RLock()
						autoSwitch := d.Config.AutoSwitchEnabled
						thresh := d.Config.AutoSwitchThreshold
						threshWeekly := d.Config.AutoSwitchWeeklyThreshold
						if thresh <= 0 {
							thresh = core.DefaultAutoSwitchThresholdFraction
						}
						if threshWeekly <= 0 {
							threshWeekly = core.DefaultAutoSwitchWeeklyThresholdFraction
						}
						switchMode := d.Config.SwitchMode
						lastSw := d.lastSwitchTime
						d.mu.RUnlock()

						if autoSwitch {
							accounts := d.Keyring.ListAccounts()
							states := quota.BuildAccountQuotaStatesWithThresholds(accounts, sum, thresh, threshWeekly)
							var activeDwellSec float64
							if !lastSw.IsZero() {
								activeDwellSec = time.Since(lastSw).Seconds()
							}
							shouldSwitch, successor, _ := quota.EvaluateAutoSwitchWithThresholds(states, active, thresh, threshWeekly, switchMode, activeDwellSec)
							if shouldSwitch && successor != nil && successor.Email != active {
								_ = d.Keyring.SetActiveAccount(successor.Email)
								d.mu.Lock()
								d.lastSwitchTime = time.Now()
								d.mu.Unlock()
								var allEmails []string
								for _, a := range accounts {
									allEmails = append(allEmails, a.Email)
								}
								if cAcc, _ := d.Keyring.GetAccount(successor.Email); cAcc != nil {
									_ = keyring.SyncAllSurfaces(cAcc, allEmails, d.Profiles)
									if cAcc.AccessToken != "" {
										_ = d.Keyring.UpdateAccountTokensWithExpiry(cAcc.Email, cAcc.AccessToken, cAcc.RefreshToken, cAcc.TokenExpiry)
									}
									if os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
										go func() {
											time.Sleep(200 * time.Millisecond)
											_ = process.NewShield(0).RelaunchHostIDE()
										}()
									}
								}
							}
						}
					}
				}
			}

		case <-standbyTicker.C:
			// 2. Infrequently refresh standby accounts in random order with random time gaps (e.g. roughly every 15m)
			accounts := d.Keyring.ListAccounts()
			active := d.Keyring.ActiveAccount()

			var standby []*keyring.Account
			for _, acc := range accounts {
				if acc.Email != active && acc.Status != "BANNED" {
					standby = append(standby, acc)
				}
			}

			if len(standby) > 0 {
				// Randomize standby accounts order
				r.Shuffle(len(standby), func(i, j int) {
					standby[i], standby[j] = standby[j], standby[i]
				})

				for _, acc := range standby {
					select {
					case <-d.ctx.Done():
						return
					default:
					}

					sum, _ := quota.PollAccountLiveQuota(acc)
					if sum != nil {
						d.setQuotaSummary(acc.Email, sum)
						if acc.AccessToken != "" {
							_ = d.Keyring.UpdateAccountTokensWithExpiry(acc.Email, acc.AccessToken, acc.RefreshToken, acc.TokenExpiry)
						}
					}

					// Add random time gap between standby accounts (5 to jitterSec seconds)
					gap := 5
					if jitterSec > 5 {
						gap = 5 + r.Intn(jitterSec-5)
					}
					select {
					case <-d.ctx.Done():
						return
					case <-time.After(time.Duration(gap) * time.Second):
					}
				}
			}
		}
	}
}
