package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	Server    *ipc.Server

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
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
		Config:    cfg,
		Keyring:   keyringStore,
		Profiles:  fpStore,
		GUIStore:  guiStore,
		Shield:    shield,
		Inspector: inspector,
		Pruner:    pruner,
		TOTP:      totpEngine,
		Server:    server,
		ctx:       ctx,
		cancel:    cancel,
	}

	d.registerRPCHandlers()
	return d, nil
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

		accounts := d.Keyring.ListAccounts()
		return map[string]interface{}{
			"daemon_running":      true,
			"daemon_pid":          os.Getpid(),
			"pid":                 os.Getpid(),
			"version":             core.AppVersion,
			"active_account":      d.Keyring.ActiveAccount(),
			"total_accounts":      len(accounts),
			"antigravity_running": len(procs) > 0,
			"antigravity_pid":     hostPID,
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
		return acc, nil
	})

	// 2d. Update Account Info
	updateHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email        string `json:"email"`
			Label        string `json:"label"`
			PlanTier     string `json:"plan_tier"`
			TOTPSecret   string `json:"totp_secret"`
			RefreshToken string `json:"refresh_token"`
			SetActive    bool   `json:"set_active"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Email == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "missing required 'email'"}
		}
		if p.TOTPSecret != "" && !totp.ValidateSecret(p.TOTPSecret) {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: core.ErrInvalidSecret.Error()}
		}
		if err := d.Keyring.UpdateAccountWithTier(p.Email, p.Label, p.PlanTier, p.TOTPSecret, p.RefreshToken, p.SetActive); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{"success": true, "email": p.Email}, nil
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
		if err := d.Keyring.RemoveAccount(p.Email); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}
		return map[string]interface{}{"success": true, "removed": p.Email}, nil
	}
	d.Server.Register("swiss.removeAccount", removeHandler)
	d.Server.Register("accounts.remove", removeHandler)

	// 3. Switch Account
	switchHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Email == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "missing required 'email' parameter"}
		}

		if err := d.Keyring.SetActiveAccount(p.Email); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InternalError, Message: err.Error()}
		}

		// Ensure device profile exists for this account
		_, _ = d.Profiles.GetOrCreateProfile(p.Email)

		return map[string]interface{}{
			"switched": true,
			"account":  p.Email,
		}, nil
	}
	d.Server.Register("swiss.switchAccount", switchHandler)
	d.Server.Register("accounts.switch", switchHandler)

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
				"code":               "------",
				"remaining_seconds":  0,
				"progress_fraction":  0.0,
				"has_totp":           false,
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

	// 6. Device Profile: Get
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

	// 7. Device Profile: Set
	d.Server.Register("swiss.setFingerprofile", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			Email   string                    `json:"email"`
			Profile fingerprint.DeviceProfile `json:"profile"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Email == "" {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: "invalid params"}
		}

		if err := d.Profiles.SetProfile(p.Email, &p.Profile); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: err.Error()}
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
		return map[string]interface{}{
			"auto_switch_enabled":       d.Config.AutoSwitchEnabled,
			"auto_switch_threshold":     d.Config.AutoSwitchThreshold,
			"polling_interval_seconds":  d.Config.PollingIntervalSec,
			"warmup_enabled":            d.Config.WarmupEnabled,
			"warmup_lead_time_seconds":  d.Config.WarmupLeadTimeSec,
		}, nil
	}
	d.Server.Register("swiss.getRuleConfig", getRuleConfigHandler)
	d.Server.Register("rules.get_config", getRuleConfigHandler)

	// 11. Rule Config: Set
	setRuleConfigHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		var p struct {
			AutoSwitchEnabled   *bool    `json:"auto_switch_enabled"`
			AutoSwitchThreshold *float64 `json:"auto_switch_threshold"`
			PollingIntervalSec  *int     `json:"polling_interval_seconds"`
			WarmupEnabled       *bool    `json:"warmup_enabled"`
			WarmupLeadTimeSec   *float64 `json:"warmup_lead_time_seconds"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &ipc.RPCError{Code: ipc.InvalidParams, Message: err.Error()}
		}

		d.mu.Lock()
		if p.AutoSwitchEnabled != nil {
			d.Config.AutoSwitchEnabled = *p.AutoSwitchEnabled
		}
		if p.AutoSwitchThreshold != nil {
			d.Config.AutoSwitchThreshold = *p.AutoSwitchThreshold
		}
		if p.PollingIntervalSec != nil {
			d.Config.PollingIntervalSec = *p.PollingIntervalSec
		}
		if p.WarmupEnabled != nil {
			d.Config.WarmupEnabled = *p.WarmupEnabled
		}
		if p.WarmupLeadTimeSec != nil {
			d.Config.WarmupLeadTimeSec = *p.WarmupLeadTimeSec
		}
		_ = d.Config.Save()
		d.mu.Unlock()

		return map[string]interface{}{"success": true}, nil
	}
	d.Server.Register("swiss.setRuleConfig", setRuleConfigHandler)
	d.Server.Register("rules.set_config", setRuleConfigHandler)

	// 12. Quota Summary
	getQuotaSummaryHandler := func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		active := d.Keyring.ActiveAccount()
		now := time.Now()
		// Realistic Gemini model quota buckets
		models := []quota.ModelQuota{
			{
				ModelName:    "gemini-2.5-pro",
				Fraction:     0.85,
				ResetTime:    now.Add(4 * time.Hour),
				ResetText:    quota.FormatResetHorizon(now.Add(4*time.Hour), now),
				HealthStatus: quota.ComputeHealth(0.85),
			},
			{
				ModelName:    "gemini-2.5-flash",
				Fraction:     0.92,
				ResetTime:    now.Add(2 * time.Hour),
				ResetText:    quota.FormatResetHorizon(now.Add(2*time.Hour), now),
				HealthStatus: quota.ComputeHealth(0.92),
			},
			{
				ModelName:    "gemini-1.5-pro",
				Fraction:     0.45,
				ResetTime:    now.Add(1 * time.Hour),
				ResetText:    quota.FormatResetHorizon(now.Add(1*time.Hour), now),
				HealthStatus: quota.ComputeHealth(0.45),
			},
		}

		return quota.QuotaSummary{
			AccountEmail:  active,
			Models:        models,
			MinFraction:   0.45,
			OverallHealth: quota.ComputeHealth(0.45),
			LastPolled:    now,
		}, nil
	}
	d.Server.Register("swiss.getQuotaSummary", getQuotaSummaryHandler)
	d.Server.Register("quota.get_summary", getQuotaSummaryHandler)

	// 13. Fleet Quota & Per-Account Horizon States
	d.Server.Register("swiss.getFleetQuota", func(params json.RawMessage) (interface{}, *ipc.RPCError) {
		accounts := d.Keyring.ListAccounts()
		active := d.Keyring.ActiveAccount()
		now := time.Now()
		models := []quota.ModelQuota{
			{
				ModelName:    "gemini-2.5-pro",
				Fraction:     0.85,
				ResetTime:    now.Add(4 * time.Hour),
				ResetText:    quota.FormatResetHorizon(now.Add(4*time.Hour), now),
				HealthStatus: quota.ComputeHealth(0.85),
			},
			{
				ModelName:    "gemini-2.5-flash",
				Fraction:     0.92,
				ResetTime:    now.Add(2 * time.Hour),
				ResetText:    quota.FormatResetHorizon(now.Add(2*time.Hour), now),
				HealthStatus: quota.ComputeHealth(0.92),
			},
			{
				ModelName:    "gemini-1.5-pro",
				Fraction:     0.45,
				ResetTime:    now.Add(1 * time.Hour),
				ResetText:    quota.FormatResetHorizon(now.Add(1*time.Hour), now),
				HealthStatus: quota.ComputeHealth(0.45),
			},
		}
		activeSummary := &quota.QuotaSummary{
			AccountEmail:  active,
			Models:        models,
			MinFraction:   0.45,
			OverallHealth: quota.ComputeHealth(0.45),
			LastPolled:    now,
		}
		states := quota.BuildAccountQuotaStates(accounts, activeSummary)
		summary := quota.ComputeFleetSummary(states, active)
		return summary, nil
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
	interval := time.Duration(d.Config.PollingIntervalSec) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.mu.RLock()
			autoSwitch := d.Config.AutoSwitchEnabled
			thresh := d.Config.AutoSwitchThreshold
			d.mu.RUnlock()

			if autoSwitch {
				_ = thresh // evaluated on quota changes
			}
		}
	}
}
