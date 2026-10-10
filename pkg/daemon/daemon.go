package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/revival"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/system"
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
	Revival   *revival.Engine
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

	ignitedAccounts map[string]time.Time
	ignitedMu       sync.RWMutex

	configUpdated chan struct{}
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
	revEngine := revival.NewEngine("", 0)
	shield.SetRevivalEngine(revEngine)
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
		Revival:        revEngine,
		Inspector:      inspector,
		Pruner:         pruner,
		TOTP:           totpEngine,
		Vault:          vault.NewManager("", ""),
		Server:         server,
		ctx:            ctx,
		cancel:         cancel,
		lastSwitchTime: time.Now(),
		quotaCache:     quota.LoadQuotaCache(),
		ignitedAccounts: make(map[string]time.Time),
		configUpdated:   make(chan struct{}, 1),
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

func (d *Daemon) getQuotaSummary(email string) *quota.QuotaSummary {
	if email == "" {
		return nil
	}
	norm := strings.ToLower(strings.TrimSpace(email))
	d.quotaCacheMu.RLock()
	defer d.quotaCacheMu.RUnlock()
	if d.quotaCache == nil {
		return nil
	}
	return d.quotaCache[norm]
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
			d.checkPostResetIgnitions()
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
			summary, _ := quota.PollAndCacheAccount(acc, d.Keyring)
			if summary != nil {
				d.setQuotaSummary(p.Email, summary)
			}
			if refreshed, _ := d.Keyring.GetAccount(p.Email); refreshed != nil {
				acc = refreshed
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
		acc, _ := d.Keyring.GetAccount(p.Email)
		var summary *quota.QuotaSummary
		if acc != nil && (acc.AccessToken != "" || acc.RefreshToken != "") {
			oldTok := acc.AccessToken
			var pollErr error
			summary, pollErr = quota.PollAndCacheAccount(acc, d.Keyring)
			if summary != nil {
				d.setQuotaSummary(p.Email, summary)
				if pollErr == nil && !p.SetActive && strings.EqualFold(p.Email, d.Keyring.ActiveAccount()) && acc.AccessToken != "" && acc.AccessToken != oldTok {
					d.syncActiveAccountSurfaces(acc)
				}
			} else {
				d.removeQuotaSummary(p.Email)
				d.triggerQuotaRefreshAsync()
			}
			if refreshed, _ := d.Keyring.GetAccount(p.Email); refreshed != nil {
				acc = refreshed
			}
		}

		if p.SetActive {
			var allEmails []string
			for _, a := range d.Keyring.ListAccounts() {
				allEmails = append(allEmails, a.Email)
			}
			if acc, _ = d.Keyring.GetAccount(p.Email); acc != nil {
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

		res := map[string]interface{}{"success": true, "email": p.Email}
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
			TargetApp   string `json:"target_app"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Email == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "missing required 'email' parameter"}
		}

		targetApp := strings.ToLower(strings.TrimSpace(p.TargetApp))
		isIndividual := d.Config.GetMultiAppSyncMode() == core.MultiAppSyncModeIndividual
		isAll := targetApp == "" || targetApp == core.TargetAppAll || !isIndividual

		if isAll || targetApp == core.TargetAppDesktop {
			if err := d.Keyring.SetActiveAccount(p.Email); err != nil {
				return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
			}
		}

		if isAll {
			_ = d.Config.SetActiveAppAccount("all", p.Email)
		} else {
			_ = d.Config.SetActiveAppAccount(targetApp, p.Email)
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
		} else if !isAll && targetApp != core.TargetAppDesktop {
			shouldRelaunch = false
		}

		var revivalIntent *revival.RevivalIntent
		if d.Revival != nil {
			revivalIntent, _ = d.Revival.CapturePreSwitchState(targetApp)
		}

		// Synchronize across relevant surfaces
		if acc, _ := d.Keyring.GetAccount(p.Email); acc != nil {
			if isAll {
				_ = keyring.SyncAllSurfaces(acc, allEmails, d.Profiles)
			} else {
				_ = keyring.SyncSurface(targetApp, acc, allEmails, d.Profiles)
			}
			_ = d.Keyring.UpdateAccountTokensWithExpiry(acc.Email, acc.AccessToken, acc.RefreshToken, acc.TokenExpiry)
			if !shouldRelaunch {
				_, _ = gui.NewInjector(0).RefreshUserStatus()
			}
			if d.Revival != nil && revivalIntent != nil {
				go func(it *revival.RevivalIntent) {
					_ = d.Revival.ExecutePostRelaunchRevival(it)
				}(revivalIntent)
			}
		}

		if shouldRelaunch && d.Shield != nil && d.Shield.IsAntigravityRunning() && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
			_ = gui.NewInjector(0).CaptureActiveConversationPath()
			go func() {
				time.Sleep(200 * time.Millisecond)
				_ = d.Shield.RelaunchHostIDE()
			}()
		}

		return map[string]interface{}{
			"switched":            true,
			"account":             p.Email,
			"success":             true,
			"active_account":      p.Email,
			"relaunch_ide":        shouldRelaunch,
			"target_app":          p.TargetApp,
			"multi_app_sync_mode": d.Config.GetMultiAppSyncMode(),
			"active_app_accounts": d.Config.GetActiveAppAccounts(),
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

	// Host IDE Launch (if not running)
	launchHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		if d.Shield == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: "shield manager not initialized"}
		}
		if os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" {
			return map[string]interface{}{
				"success": true,
				"message": "Antigravity host IDE launch skipped (dry run)",
			}, nil
		}
		if err := d.Shield.LaunchHostIDE(); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{
			"success": true,
			"message": "Antigravity host IDE launch initiated",
		}, nil
	}
	d.Server.Register("swiss.launchIDE", launchHandler)
	d.Server.Register("desktop.launch", launchHandler)

	// Active Conversations & Subagent State Detection
	getActiveConversationsHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		targetApp := ""
		if len(params) > 0 {
			var p struct {
				TargetApp string `json:"target_app"`
			}
			_ = json.Unmarshal(params, &p)
			targetApp = p.TargetApp
		}
		if d.Revival == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: "revival engine not initialized"}
		}
		info, err := d.Revival.Detector.Detect(targetApp)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return info, nil
	}
	d.Server.Register("swiss.getActiveConversations", getActiveConversationsHandler)
	d.Server.Register("conversations.getActive", getActiveConversationsHandler)

	// Conversation Revival Trigger
	reviveConversationHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			TargetApp      string `json:"target_app"`
			ConversationID string `json:"conversation_id"`
			Prompt         string `json:"prompt"`
		}
		if len(params) > 0 {
			_ = json.Unmarshal(params, &p)
		}
		if d.Revival == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: "revival engine not initialized"}
		}
		if err := d.Revival.ReviveConversation(p.TargetApp, p.ConversationID, p.Prompt); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{"success": true, "status": "revived"}, nil
	}
	d.Server.Register("swiss.reviveConversation", reviveConversationHandler)
	d.Server.Register("conversations.revive", reviveConversationHandler)

	// Revival Status Query
	getRevivalStatusHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		if d.Revival == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: "revival engine not initialized"}
		}
		var p struct {
			ConversationID string `json:"conversation_id"`
			CascadeID      string `json:"cascade_id"`
		}
		if len(params) > 0 {
			_ = json.Unmarshal(params, &p)
		}
		targetID := p.ConversationID
		if targetID == "" {
			targetID = p.CascadeID
		}
		targetID = strings.TrimPrefix(strings.TrimSpace(targetID), "/c/")

		status, err := d.Revival.GetRevivalStatus(targetID)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return status, nil
	}
	d.Server.Register("swiss.getRevivalStatus", getRevivalStatusHandler)
	d.Server.Register("conversations.getStatus", getRevivalStatusHandler)

	// Acknowledge Continuation
	ackContinuationHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			CascadeID string `json:"cascade_id"`
		}
		if len(params) > 0 {
			_ = json.Unmarshal(params, &p)
		}
		if d.Revival == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: "revival engine not initialized"}
		}
		if err := d.Revival.AcknowledgeContinuation(p.CascadeID); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{"success": true}, nil
	}
	d.Server.Register("swiss.ackContinuation", ackContinuationHandler)
	d.Server.Register("conversations.ack", ackContinuationHandler)

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
			MinAgeDays *float64 `json:"min_age_days"`
			MaxSizeGB  float64  `json:"max_size_gb"`
		}
		_ = json.Unmarshal(params, &p)
		age := 0.0
		if p.MinAgeDays != nil {
			age = *p.MinAgeDays
		}

		maxBytes := int64(0)
		if p.MaxSizeGB > 0 {
			maxBytes = int64(p.MaxSizeGB * 1024 * 1024 * 1024)
		}

		bd, err := d.Inspector.ScanBreakdownWithLimit(age, maxBytes)
		if err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return bd, nil
	})

	// 9. Cache: Prune
	d.Server.Register("swiss.pruneCache", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var opts cache.PruneOptions
		if err := json.Unmarshal(params, &opts); err != nil {
			opts.MinAgeDays = 0.0
			opts.MaxSizeGB = 0.0
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

	// 9b. Cache Config: Get & Set
	d.Server.Register("swiss.getCacheConfig", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		d.mu.RLock()
		defer d.mu.RUnlock()
		maxSize := d.Config.AutoPruneMaxSizeGB
		if maxSize < 0 {
			maxSize = 0.0
		}
		return map[string]interface{}{
			"auto_prune_enabled": d.Config.AutoPruneEnabled,
			"prune_days":         d.Config.AutoPruneMaxAgeDays,
			"max_size_gb":        maxSize,
		}, nil
	})

	d.Server.Register("swiss.setCacheConfig", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			AutoPruneEnabled *bool    `json:"auto_prune_enabled"`
			PruneDays        *float64 `json:"prune_days"`
			MaxSizeGB        *float64 `json:"max_size_gb"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: err.Error()}
		}
		d.mu.Lock()
		if p.AutoPruneEnabled != nil {
			d.Config.AutoPruneEnabled = *p.AutoPruneEnabled
		}
		if p.PruneDays != nil && *p.PruneDays >= 0 {
			d.Config.AutoPruneMaxAgeDays = *p.PruneDays
		}
		if p.MaxSizeGB != nil && *p.MaxSizeGB >= 0 {
			d.Config.AutoPruneMaxSizeGB = *p.MaxSizeGB
		}
		if d.Config.AutoPruneMaxSizeGB < 0 {
			d.Config.AutoPruneMaxSizeGB = 0.0
		}
		_ = d.Config.Save()
		res := map[string]interface{}{
			"success":            true,
			"auto_prune_enabled": d.Config.AutoPruneEnabled,
			"prune_days":         d.Config.AutoPruneMaxAgeDays,
			"max_size_gb":        d.Config.AutoPruneMaxSizeGB,
		}
		d.mu.Unlock()
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
			defaultNonGemini = "claude-opus-4-6-thinking"
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
		inst := system.NewDetector().DetectAll()
		installedMap := map[string]bool{
			"desktop": inst != nil && inst.DesktopApp.Installed,
			"agy":     inst != nil && inst.AgyCLI.Installed,
			"vscode":  inst != nil && inst.VSCodeExtension.Installed,
		}

		return map[string]interface{}{
			"auto_switch_enabled":              d.Config.AutoSwitchEnabled,
			"auto_switch_threshold":            d.Config.AutoSwitchThreshold,
			"auto_switch_weekly_threshold":     threshWeekly,
			"switch_mode":                      switchMode,
			"quota_refresh_mode":               d.Config.GetQuotaRefreshMode(),
			"dynamic_quota_refresh_enabled":    d.Config.IsDynamicQuotaRefreshEnabled(),
			"polling_interval_seconds":         d.Config.PollingIntervalSec,
			"active_polling_interval_seconds":  activePoll,
			"standby_polling_interval_seconds": standbyPoll,
			"standby_random_jitter_seconds":    jitter,
			"warmup_enabled":                   d.Config.WarmupEnabled,
			"warmup_lead_time_seconds":         d.Config.GetPostResetDelaySec(),
			"post_reset_delay_seconds":         d.Config.GetPostResetDelaySec(),
			"preferred_native_model":           prefNative,
			"allow_ai_credits_usage":           d.Config.AllowAICreditsUsage,
			"allow_non_gemini_native_models":   d.Config.AllowNonGeminiNativeModels,
			"model_source_hierarchy":           d.Config.ModelSourceHierarchy,
			"default_gemini_model":             defaultGemini,
			"default_custom_model":             d.Config.DefaultCustomModel,
			"default_non_gemini_model":         defaultNonGemini,
			"default_gemini_reasoning_level":   geminiReasoning,
			"auto_import_active_account":       d.Config.AutoImportActiveAccount,
			"multi_app_sync_mode":              d.Config.GetMultiAppSyncMode(),
			"active_app_accounts":              d.Config.GetActiveAppAccounts(),
			"subagent_custom_models_enabled":   d.Config.GetSubagentCustomModelsEnabled(),
			"subagent_model_strategy":          d.Config.GetSubagentModelStrategy(),
			"installed_apps":                   installedMap,
		}, nil
	}
	d.Server.Register("swiss.getRuleConfig", getRuleConfigHandler)
	d.Server.Register("rules.get_config", getRuleConfigHandler)

	// 11. Rule Config: Set
	setRuleConfigHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			AutoSwitchEnabled           *bool              `json:"auto_switch_enabled"`
			AutoSwitchThreshold         *float64           `json:"auto_switch_threshold"`
			AutoSwitchWeeklyThreshold   *float64           `json:"auto_switch_weekly_threshold"`
			SwitchMode                  *string            `json:"switch_mode"`
			QuotaRefreshMode            *string            `json:"quota_refresh_mode"`
			DynamicQuotaRefreshEnabled  *bool              `json:"dynamic_quota_refresh_enabled"`
			PollingIntervalSec          *int               `json:"polling_interval_seconds"`
			ActivePollingIntervalSec    *int               `json:"active_polling_interval_seconds"`
			StandbyPollingIntervalSec   *int               `json:"standby_polling_interval_seconds"`
			StandbyRandomJitterSec      *int               `json:"standby_random_jitter_seconds"`
			WarmupEnabled               *bool              `json:"warmup_enabled"`
			WarmupLeadTimeSec           *float64           `json:"warmup_lead_time_seconds"`
			PostResetDelaySec           *float64           `json:"post_reset_delay_seconds"`
			PreferredNativeModel        *string            `json:"preferred_native_model"`
			AllowAICreditsUsage         *bool              `json:"allow_ai_credits_usage"`
			AllowNonGeminiNativeModels  *bool              `json:"allow_non_gemini_native_models"`
			ModelSourceHierarchy        *[]string          `json:"model_source_hierarchy"`
			DefaultGeminiModel          *string            `json:"default_gemini_model"`
			DefaultCustomModel          *string            `json:"default_custom_model"`
			DefaultNonGeminiModel       *string            `json:"default_non_gemini_model"`
			DefaultGeminiReasoningLevel *string            `json:"default_gemini_reasoning_level"`
			AutoImportActiveAccount     *bool              `json:"auto_import_active_account"`
			MultiAppSyncMode            *string            `json:"multi_app_sync_mode"`
			ActiveAppAccounts           *map[string]string `json:"active_app_accounts"`
			SubagentCustomModelsEnabled *bool              `json:"subagent_custom_models_enabled"`
			SubagentModelStrategy       *string            `json:"subagent_model_strategy"`
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
		if p.QuotaRefreshMode != nil {
			d.Config.QuotaRefreshMode = core.NormalizeQuotaRefreshMode(*p.QuotaRefreshMode)
			d.Config.DynamicQuotaRefreshEnabled = (d.Config.QuotaRefreshMode == core.QuotaRefreshModeDynamic)
		}
		if p.DynamicQuotaRefreshEnabled != nil {
			d.Config.DynamicQuotaRefreshEnabled = *p.DynamicQuotaRefreshEnabled
			if *p.DynamicQuotaRefreshEnabled {
				d.Config.QuotaRefreshMode = core.QuotaRefreshModeDynamic
			} else {
				d.Config.QuotaRefreshMode = core.QuotaRefreshModeManual
			}
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
		if p.PostResetDelaySec != nil {
			d.Config.PostResetDelaySec = *p.PostResetDelaySec
			d.Config.WarmupLeadTimeSec = *p.PostResetDelaySec
		} else if p.WarmupLeadTimeSec != nil {
			d.Config.WarmupLeadTimeSec = *p.WarmupLeadTimeSec
			d.Config.PostResetDelaySec = *p.WarmupLeadTimeSec
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
		if p.MultiAppSyncMode != nil {
			_ = d.Config.SetMultiAppSyncMode(*p.MultiAppSyncMode)
		}
		if p.ActiveAppAccounts != nil {
			for k, v := range *p.ActiveAppAccounts {
				_ = d.Config.SetActiveAppAccount(k, v)
			}
		}
		if p.SubagentCustomModelsEnabled != nil {
			_ = d.Config.SetSubagentCustomModelsEnabled(*p.SubagentCustomModelsEnabled)
		}
		if p.SubagentModelStrategy != nil {
			_ = d.Config.SetSubagentModelStrategy(*p.SubagentModelStrategy)
		}
		_ = d.Config.Save()
		d.mu.Unlock()
		d.notifyConfigUpdated()

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
			if summary != nil {
				d.setQuotaSummary(target, summary)
				if err == nil && strings.EqualFold(target, d.Keyring.ActiveAccount()) && acc.AccessToken != "" && acc.AccessToken != oldTok {
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
		switchMode := d.Config.SwitchMode
		d.mu.RUnlock()
		if thresh <= 0 {
			thresh = core.DefaultAutoSwitchThresholdFraction
		}
		if threshWeekly <= 0 {
			threshWeekly = core.DefaultAutoSwitchWeeklyThresholdFraction
		}
		if switchMode == "" {
			switchMode = core.DefaultSwitchMode
		}
		states := quota.BuildAccountQuotaStatesFromMapWithThresholds(accounts, summaries, thresh, threshWeekly)
		states = quota.SortAccountQuotaStatesWithThresholds(states, active, thresh, threshWeekly, "auto", switchMode)
		summary := quota.ComputeFleetSummary(states, active)
		d.pollingMu.Lock()
		summary.Refreshing = d.isPollingFleet
		d.pollingMu.Unlock()
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
		d.mu.Lock()
		d.Config.PersistentVisualEffects = true
		_ = d.Config.Save()
		d.mu.Unlock()
		res, err := d.GUIStore.InstallDesktopLoader()
		if err != nil && res == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return res, nil
	})

	d.Server.Register("swiss.setDesktopPersistence", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: err.Error()}
		}
		d.mu.Lock()
		d.Config.PersistentVisualEffects = p.Enabled
		_ = d.Config.Save()
		d.mu.Unlock()

		var res *gui.ApplyResult
		var err error
		if p.Enabled {
			res, err = d.GUIStore.InstallDesktopLoader()
		} else {
			res, err = d.GUIStore.UninstallDesktopLoader()
		}
		if err != nil && res == nil {
			return map[string]interface{}{
				"success":                   true,
				"persistent_visual_effects": p.Enabled,
				"message":                   err.Error(),
			}, nil
		}
		return map[string]interface{}{
			"success":                   true,
			"persistent_visual_effects": p.Enabled,
			"installed":                 p.Enabled && res != nil && res.Success,
			"message":                   res.Message,
		}, nil
	})

	d.Server.Register("swiss.restoreFactoryDefaults", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		res, err := d.GUIStore.RestoreFactoryDefaults()
		if err != nil && res == nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return res, nil
	})

	d.Server.Register("swiss.getDesktopStatus", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		st := d.GUIStore.GetDesktopStatus()
		d.mu.RLock()
		st.PersistentVisualEffects = d.Config.PersistentVisualEffects
		d.mu.RUnlock()
		return st, nil
	})
}

// Start launches the IPC server and background scheduler.
func (d *Daemon) Start() error {
	if err := d.Server.Start(); err != nil {
		return err
	}
	if d.GUIStore != nil {
		go func() {
			time.Sleep(200 * time.Millisecond)
			_, _ = d.GUIStore.Apply()
		}()
	}
	d.wg.Add(1)
	go d.schedulerLoop()
	return nil
}

// Stop gracefully shuts down all services.
func (d *Daemon) Stop() error {
	d.cancel()
	if d.GUIStore != nil {
		d.mu.RLock()
		keepVisual := d.Config.PersistentVisualEffects
		d.mu.RUnlock()
		_ = d.GUIStore.DeactivateOnDaemonStop(keepVisual)
	}
	_ = d.Server.Stop()
	d.wg.Wait()
	return nil
}

func (d *Daemon) notifyConfigUpdated() {
	if d.configUpdated != nil {
		select {
		case d.configUpdated <- struct{}{}:
		default:
		}
	}
}

func resetTimer(t *time.Timer, duration time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(duration)
}

func (d *Daemon) calculateNextActiveInterval(r *rand.Rand) time.Duration {
	d.mu.RLock()
	dynamicEnabled := d.Config.IsDynamicQuotaRefreshEnabled()
	activeIntervalSec := d.Config.ActivePollingIntervalSec
	thresh := d.Config.AutoSwitchThreshold
	d.mu.RUnlock()

	if !dynamicEnabled {
		interval := time.Duration(activeIntervalSec) * time.Second
		if interval < 10*time.Second {
			interval = 120 * time.Second
		}
		return interval
	}

	active := d.Keyring.ActiveAccount()
	if active == "" {
		return quota.RollDynamicJitter(60*time.Second, r)
	}

	cached := d.getQuotaSummary(active)
	if thresh <= 0 {
		thresh = core.DefaultAutoSwitchThresholdFraction
	}

	fraction, ok := quota.ExtractRemaining5HFraction(cached)
	if !ok {
		return quota.RollDynamicJitter(60*time.Second, r)
	}

	return quota.ResolveActiveDynamicIntervalWithJitter(fraction, thresh, r)
}

func (d *Daemon) calculateNextStandbyInterval(r *rand.Rand) time.Duration {
	d.mu.RLock()
	dynamicEnabled := d.Config.IsDynamicQuotaRefreshEnabled()
	standbyIntervalSec := d.Config.StandbyPollingIntervalSec
	d.mu.RUnlock()

	if !dynamicEnabled {
		interval := time.Duration(standbyIntervalSec) * time.Second
		if interval < 30*time.Second {
			interval = 900 * time.Second
		}
		return interval
	}

	accounts := d.Keyring.ListAccounts()
	active := d.Keyring.ActiveAccount()

	var minNominal time.Duration = quota.StandbyIntervalAbundant // 15m default
	hasStandby := false

	for _, acc := range accounts {
		if acc.Email == active || acc.Status == "BANNED" {
			continue
		}
		hasStandby = true
		cached := d.getQuotaSummary(acc.Email)
		fraction, ok := quota.ExtractRemaining5HFraction(cached)
		var nominal time.Duration
		if !ok {
			nominal = quota.StandbyIntervalRecovering // 5m
		} else {
			nominal = quota.ResolveStandbyDynamicInterval(fraction)
		}
		if nominal < minNominal {
			minNominal = nominal
		}
	}

	if !hasStandby {
		minNominal = quota.StandbyIntervalAbundant
	}
	return quota.RollDynamicJitter(minNominal, r)
}

func (d *Daemon) schedulerLoop() {
	defer d.wg.Done()

	d.mu.RLock()
	jitterSec := d.Config.StandbyRandomJitterSec
	d.mu.RUnlock()

	if jitterSec <= 0 {
		jitterSec = 30
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	activeTimer := time.NewTimer(d.calculateNextActiveInterval(r))
	defer activeTimer.Stop()

	standbyTimer := time.NewTimer(d.calculateNextStandbyInterval(r))
	defer standbyTimer.Stop()

	vaultTicker := time.NewTicker(60 * time.Second)
	defer vaultTicker.Stop()

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
			autoPruneOn := d.Config.AutoPruneEnabled
			pruneAge := d.Config.AutoPruneMaxAgeDays
			pruneMaxGB := d.Config.AutoPruneMaxSizeGB
			d.mu.RUnlock()
			if vaultOn && d.Vault != nil {
				_, _ = d.Vault.Sync()
			}
			if autoPruneOn && d.Pruner != nil {
				_, _ = d.Pruner.Prune(cache.PruneOptions{
					MinAgeDays:   pruneAge,
					MaxSizeGB:    pruneMaxGB,
					PruneScratch: true,
					PruneSteps:   true,
					PruneTasks:   true,
				})
			}

		case <-activeTimer.C:
			// 1. Frequently refresh active account quota (e.g. every 2m)
			_ = gui.NewInjector(0).CaptureActiveConversationPath()
			var tickEmails []string
			for _, a := range d.Keyring.ListAccounts() {
				tickEmails = append(tickEmails, a.Email)
			}
			_, _ = d.Keyring.ReconcileActiveAccount(d.Config.AutoImportActiveAccount, tickEmails, d.Profiles)
			active := d.Keyring.ActiveAccount()
			if active != "" {
				accounts := d.Keyring.ListAccounts()
				var allEmails []string
				for _, a := range accounts {
					allEmails = append(allEmails, a.Email)
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
				syncMode := d.Config.GetMultiAppSyncMode()
				d.mu.RUnlock()

				var activeDwellSec float64
				if !lastSw.IsZero() {
					activeDwellSec = time.Since(lastSw).Seconds()
				}

				// Collect accounts needing refresh
				emailsToPoll := []string{active}
				if syncMode == core.MultiAppSyncModeIndividual {
					for _, app := range []string{core.TargetAppDesktop, core.TargetAppCLI, core.TargetAppVSCode} {
						if em := d.Config.GetActiveAppAccount(app); em != "" && em != active {
							emailsToPoll = append(emailsToPoll, em)
						}
					}
				}

				var primarySum *quota.QuotaSummary
				for _, em := range emailsToPoll {
					if acc, _ := d.Keyring.GetAccount(em); acc != nil {
						oldTok := acc.AccessToken
						sum, err := quota.PollAndCacheAccount(acc, d.Keyring)
						if sum != nil {
							d.setQuotaSummary(em, sum)
							if em == active {
								primarySum = sum
							}
						}
						if err == nil && sum != nil && acc.AccessToken != "" && acc.AccessToken != oldTok {
							d.syncActiveAccountSurfaces(acc)
						}
					}
				}

				if autoSwitch && primarySum != nil {
					states := quota.BuildAccountQuotaStatesWithThresholds(accounts, primarySum, thresh, threshWeekly)

					if syncMode == core.MultiAppSyncModeIndividual {
						// Per-app rotation
						inUseEmails := make([]string, 0)
						for _, app := range []string{core.TargetAppDesktop, core.TargetAppCLI, core.TargetAppVSCode} {
							if em := d.Config.GetActiveAppAccount(app); em != "" {
								inUseEmails = append(inUseEmails, em)
							}
						}

						for _, app := range []string{core.TargetAppDesktop, core.TargetAppCLI, core.TargetAppVSCode} {
							appEmail := d.Config.GetActiveAppAccount(app)
							if appEmail == "" {
								continue
							}
							shouldSwitch, successor, _ := quota.EvaluateAutoSwitchForApp(states, appEmail, inUseEmails, thresh, threshWeekly, switchMode, activeDwellSec)
							if shouldSwitch && successor != nil && successor.Email != appEmail {
								cAcc, _ := d.Keyring.GetAccount(successor.Email)
								if cAcc == nil || strings.TrimSpace(cAcc.RefreshToken) == "" {
									continue
								}

								_ = d.Config.SetActiveAppAccount(app, successor.Email)
								if app == core.TargetAppDesktop {
									_ = d.Keyring.SetActiveAccount(successor.Email)
									if !strings.EqualFold(d.Keyring.ActiveAccount(), successor.Email) {
										_ = d.Keyring.SetActiveAccount(successor.Email)
									}
									if !strings.EqualFold(d.Keyring.ActiveAccount(), successor.Email) {
										log.Printf("[Daemon Auto-Switch] Alert: failed to activate successor %s (current: %s); aborting switch", successor.Email, d.Keyring.ActiveAccount())
										continue
									}
								}
								d.mu.Lock()
								d.lastSwitchTime = time.Now()
								d.mu.Unlock()

								var revIntent *revival.RevivalIntent
								if d.Revival != nil {
									revIntent, _ = d.Revival.CapturePreSwitchState(app)
								}

								if cAcc, _ := d.Keyring.GetAccount(successor.Email); cAcc != nil {
									_ = keyring.SyncSurface(app, cAcc, allEmails, d.Profiles)
									if cAcc.AccessToken != "" {
										_ = d.Keyring.UpdateAccountTokensWithExpiry(cAcc.Email, cAcc.AccessToken, cAcc.RefreshToken, cAcc.TokenExpiry)
									}
									if app == core.TargetAppDesktop && d.Shield != nil && d.Shield.IsAntigravityRunning() && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
										_ = gui.NewInjector(0).CaptureActiveConversationPath()
										go func() {
											time.Sleep(200 * time.Millisecond)
											if err := d.Shield.RelaunchHostIDE(); err != nil {
												log.Printf("[Daemon Auto-Switch] RelaunchHostIDE error: %v", err)
											}
											if d.Revival != nil && revIntent != nil {
												time.Sleep(3 * time.Second)
												_ = d.Revival.ExecutePostRelaunchRevival(revIntent)
											}
										}()
									} else if d.Revival != nil && revIntent != nil {
										go func(it *revival.RevivalIntent) {
											_ = d.Revival.ExecutePostRelaunchRevival(it)
										}(revIntent)
									}
								}
								for idx, em := range inUseEmails {
									if em == appEmail {
										inUseEmails[idx] = successor.Email
										break
									}
								}
							}
						}
					} else {
						// Shared mode rotation
						shouldSwitch, successor, _ := quota.EvaluateAutoSwitchWithThresholds(states, active, thresh, threshWeekly, switchMode, activeDwellSec)
						if shouldSwitch && successor != nil && successor.Email != active {
							cAcc, _ := d.Keyring.GetAccount(successor.Email)
							if cAcc == nil || strings.TrimSpace(cAcc.RefreshToken) == "" {
								continue
							}

							_ = d.Keyring.SetActiveAccount(successor.Email)
							if !strings.EqualFold(d.Keyring.ActiveAccount(), successor.Email) {
								_ = d.Keyring.SetActiveAccount(successor.Email)
							}
							if !strings.EqualFold(d.Keyring.ActiveAccount(), successor.Email) {
								log.Printf("[Daemon Auto-Switch] Alert: failed to activate successor %s (current: %s); aborting switch", successor.Email, d.Keyring.ActiveAccount())
								continue
							}
							_ = d.Config.SetActiveAppAccount("all", successor.Email)
							d.mu.Lock()
							d.lastSwitchTime = time.Now()
							d.mu.Unlock()

							var revIntent *revival.RevivalIntent
							if d.Revival != nil {
								revIntent, _ = d.Revival.CapturePreSwitchState(core.TargetAppDesktop)
							}

							if cAcc, _ := d.Keyring.GetAccount(successor.Email); cAcc != nil {
								_ = keyring.SyncAllSurfaces(cAcc, allEmails, d.Profiles)
								if cAcc.AccessToken != "" {
									_ = d.Keyring.UpdateAccountTokensWithExpiry(cAcc.Email, cAcc.AccessToken, cAcc.RefreshToken, cAcc.TokenExpiry)
								}
								if d.Shield != nil && d.Shield.IsAntigravityRunning() && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" {
									_ = gui.NewInjector(0).CaptureActiveConversationPath()
									go func() {
										time.Sleep(200 * time.Millisecond)
										if err := d.Shield.RelaunchHostIDE(); err != nil {
											log.Printf("[Daemon Auto-Switch] RelaunchHostIDE error: %v", err)
										}
										if d.Revival != nil && revIntent != nil {
											time.Sleep(3 * time.Second)
											_ = d.Revival.ExecutePostRelaunchRevival(revIntent)
										}
									}()
								} else if d.Revival != nil && revIntent != nil {
									go func(it *revival.RevivalIntent) {
										_ = d.Revival.ExecutePostRelaunchRevival(it)
									}(revIntent)
								}
							}
						}
					}
				}
				d.checkPostResetIgnitions()
			}
			activeTimer.Reset(d.calculateNextActiveInterval(r))

		case <-standbyTimer.C:
			// 2. Infrequently refresh standby accounts with cooldown skip and post-reset ignition
			d.pollStandbyAccounts(r, jitterSec)
			standbyTimer.Reset(d.calculateNextStandbyInterval(r))

		case <-d.configUpdated:
			resetTimer(activeTimer, d.calculateNextActiveInterval(r))
			resetTimer(standbyTimer, d.calculateNextStandbyInterval(r))
		}
	}
}

// pollStandbyAccounts executes background polling for standby accounts.
// If an account is in cooldown with a known reset horizon (now < resetTime), automatic background polling is skipped
// to eliminate unnecessary network traffic and rate limit pressure.
// When now >= resetTime + postResetDelay, the account is polled to verify quota reset.
// If quota reset is confirmed (healthy / not exhausted), a 1-token keep-alive probe is triggered to start Google's 5-hour rolling timer.
func (d *Daemon) pollStandbyAccounts(r *rand.Rand, jitterSec int) {
	accounts := d.Keyring.ListAccounts()
	active := d.Keyring.ActiveAccount()

	var standby []*keyring.Account
	for _, acc := range accounts {
		if acc.Email != active && acc.Status != "BANNED" {
			standby = append(standby, acc)
		}
	}

	if len(standby) == 0 {
		return
	}

	if r != nil && len(standby) > 1 {
		r.Shuffle(len(standby), func(i, j int) {
			standby[i], standby[j] = standby[j], standby[i]
		})
	}

	d.mu.RLock()
	warmupEnabled := d.Config.WarmupEnabled
	postResetDelaySec := d.Config.GetPostResetDelaySec()
	thresh := d.Config.AutoSwitchThreshold
	d.mu.RUnlock()

	scheduler := &quota.WarmupScheduler{
		PostResetDelay: time.Duration(postResetDelaySec * float64(time.Second)),
	}

	for _, acc := range standby {
		select {
		case <-d.ctx.Done():
			return
		default:
		}

		now := time.Now()
		cached := d.getQuotaSummary(acc.Email)
		var resetTime time.Time
		if cached != nil {
			resetTime = cached.GetResetTime()
		}

		// If dynamic adaptive quota refresh is enabled, skip accounts that are not yet due
		if d.Config.IsDynamicQuotaRefreshEnabled() && cached != nil && !cached.LastPolled.IsZero() {
			fraction, ok := quota.ExtractRemaining5HFraction(cached)
			if ok {
				nominal := quota.ResolveStandbyDynamicInterval(fraction)
				if time.Since(cached.LastPolled) < nominal-30*time.Second {
					continue
				}
			}
		}

		// Check if standby account is in cooldown with a known reset horizon
		if cached != nil && cached.IsCooldown(thresh) && !resetTime.IsZero() {
			// 1. If in cooldown (now < resetTime), skip automatic background polling
			if scheduler.ShouldSkipCooldownPolling(resetTime, now) {
				continue
			}

			// 2. If resetTime has passed but post-reset verification delay has not elapsed, wait
			if !scheduler.ShouldTriggerPostResetIgnition(resetTime, now) {
				continue
			}

			// 3. Delay window elapsed (now >= resetTime + postResetDelay): poll to verify reset
			freshSum, _ := quota.PollAndCacheAccount(acc, d.Keyring)
			if freshSum != nil {
				if !freshSum.IsCooldown(thresh) && freshSum.OverallHealth != core.StatusExhausted {
					if warmupEnabled && d.shouldIgniteAccount(acc.Email, freshSum, thresh) {
						if err := quota.IgniteAccountPostReset(acc, d.Keyring); err == nil {
							d.recordIgnition(acc.Email)
							if postSum, _ := quota.PollAndCacheAccount(acc, d.Keyring); postSum != nil {
								freshSum = postSum
							}
						}
					}
				}
				d.setQuotaSummary(acc.Email, freshSum)
			}
		} else {
			// Normal standby account refresh (not in cooldown)
			sum, _ := quota.PollAndCacheAccount(acc, d.Keyring)
			if sum != nil {
				if warmupEnabled && d.shouldIgniteAccount(acc.Email, sum, thresh) {
					if err := quota.IgniteAccountPostReset(acc, d.Keyring); err == nil {
						d.recordIgnition(acc.Email)
						if postSum, _ := quota.PollAndCacheAccount(acc, d.Keyring); postSum != nil {
							sum = postSum
						}
					}
				}
				d.setQuotaSummary(acc.Email, sum)
			}
		}

		// Add random time gap between polled standby accounts (5 to jitterSec seconds)
		gap := 5
		if jitterSec > 5 && r != nil {
			gap = 5 + r.Intn(jitterSec-5)
		}
		select {
		case <-d.ctx.Done():
			return
		case <-time.After(time.Duration(gap) * time.Second):
		}
	}
}

// checkPostResetIgnitions checks standby accounts in cooldown whose post-reset verification delay has elapsed,
// verifies that their quota has reset, triggers the 1-token ignition probe, and ensures standby accounts
// with unanchored 5-hour rolling timers begin countdown immediately.
func (d *Daemon) checkPostResetIgnitions() {
	accounts := d.Keyring.ListAccounts()
	active := d.Keyring.ActiveAccount()

	d.mu.RLock()
	warmupEnabled := d.Config.WarmupEnabled
	postResetDelaySec := d.Config.GetPostResetDelaySec()
	thresh := d.Config.AutoSwitchThreshold
	d.mu.RUnlock()

	scheduler := &quota.WarmupScheduler{
		PostResetDelay: time.Duration(postResetDelaySec * float64(time.Second)),
	}

	now := time.Now()
	for _, acc := range accounts {
		if acc.Email == active || acc.Status == "BANNED" {
			continue
		}
		cached := d.getQuotaSummary(acc.Email)
		if cached == nil {
			continue
		}

		// Case 1: Account was in cooldown, resetTime has arrived + postResetDelay
		if cached.IsCooldown(thresh) {
			resetTime := cached.GetResetTime()
			if resetTime.IsZero() {
				continue
			}

			// When now >= resetTime + postResetDelay, verify quota reset and ignite
			if scheduler.ShouldTriggerPostResetIgnition(resetTime, now) {
				freshSum, _ := quota.PollAndCacheAccount(acc, d.Keyring)
				if freshSum != nil {
					if !freshSum.IsCooldown(thresh) && freshSum.OverallHealth != core.StatusExhausted {
						if warmupEnabled && d.shouldIgniteAccount(acc.Email, freshSum, thresh) {
							if err := quota.IgniteAccountPostReset(acc, d.Keyring); err == nil {
								d.recordIgnition(acc.Email)
								if postSum, _ := quota.PollAndCacheAccount(acc, d.Keyring); postSum != nil {
									freshSum = postSum
								}
							}
						}
					}
					d.setQuotaSummary(acc.Email, freshSum)
				}
			}
			continue
		}

		// Case 2: Standby account is healthy / reset, but its 5h rolling timer has not been ignited yet!
		if warmupEnabled && d.shouldIgniteAccount(acc.Email, cached, thresh) {
			if err := quota.IgniteAccountPostReset(acc, d.Keyring); err == nil {
				d.recordIgnition(acc.Email)
				if postSum, _ := quota.PollAndCacheAccount(acc, d.Keyring); postSum != nil {
					d.setQuotaSummary(acc.Email, postSum)
				}
			}
		}
	}
}

// shouldIgniteAccount checks if a standby account is eligible for a 1-token keep-alive probe
// to anchor Google's 5-hour rolling reset window early.
func (d *Daemon) shouldIgniteAccount(email string, qs *quota.QuotaSummary, thresh float64) bool {
	if qs == nil {
		return false
	}
	if thresh <= 0 {
		thresh = core.DefaultAutoSwitchThresholdFraction
	}
	// Must not be exhausted or in cooldown
	if qs.OverallHealth == core.StatusExhausted || qs.MinFraction <= thresh {
		return false
	}
	// Must have 5h quota available to ignite (at least near 100%)
	if qs.Quota5hFraction < 0.95 {
		return false
	}
	// Weekly quota must not be depleted below threshold
	if qs.QuotaWeeklyFraction > 0 && qs.QuotaWeeklyFraction <= thresh {
		return false
	}

	d.ignitedMu.RLock()
	lastIgnited, exists := d.ignitedAccounts[strings.ToLower(strings.TrimSpace(email))]
	d.ignitedMu.RUnlock()

	// If ignited within the last 4.5 hours, do not ignite again
	if exists && time.Since(lastIgnited) < 4*time.Hour+30*time.Minute {
		return false
	}

	// An account needs ignition if its 5h reset timer has NOT started rolling:
	// When Google hasn't started the timer, ResetSeconds5h is >= 17700 (near 18000s = 5h)
	// or the model reset time is >= now + 4h55m
	if qs.ResetSeconds5h >= 17700 {
		return true
	}
	rt := qs.GetResetTime()
	if !rt.IsZero() && rt.After(time.Now().Add(4*time.Hour+55*time.Minute)) {
		return true
	}
	return false
}

// recordIgnition records the timestamp an account was successfully ignited.
func (d *Daemon) recordIgnition(email string) {
	d.ignitedMu.Lock()
	if d.ignitedAccounts == nil {
		d.ignitedAccounts = make(map[string]time.Time)
	}
	d.ignitedAccounts[strings.ToLower(strings.TrimSpace(email))] = time.Now()
	d.ignitedMu.Unlock()
}
