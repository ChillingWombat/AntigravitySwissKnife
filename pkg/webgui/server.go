package webgui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/cache"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/process"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/system"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/templates"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/totp"
)

//go:embed all:dist
var distFS embed.FS

// Server serves the minimalist light theme web UI and REST API.
type Server struct {
	client             *ipc.Client
	guiStore           *gui.Store
	customModelsStore  *custommodels.Store
	customModelsTester *custommodels.Tester
	systemDetector     *system.Detector
	enhancementsStore  *enhancements.Store
	templatesStore     *templates.Store
	httpServer         *http.Server
	addr               string
}

// NewServer initializes the web UI server.
func NewServer(addr string, socketPath string) *Server {
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	client := ipc.NewClient(socketPath)
	guiStore, _ := gui.NewStore("")
	cmStore, _ := custommodels.NewStore("")
	cmTester := custommodels.NewTester()
	sysDetector := system.NewDetector()
	enhStore, _ := enhancements.NewStore("")
	tmplStore, _ := templates.NewStore("")
	return &Server{
		client:             client,
		guiStore:           guiStore,
		customModelsStore:  cmStore,
		customModelsTester: cmTester,
		systemDetector:     sysDetector,
		enhancementsStore:  enhStore,
		templatesStore:     tmplStore,
		addr:               addr,
	}
}

// Start starts listening and serving HTTP requests.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	distSub, errDist := fs.Sub(distFS, "dist")
	var fileServer http.Handler
	if errDist == nil {
		fileServer = http.FileServer(http.FS(distSub))
		mux.Handle("/assets/", fileServer)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			if distSub != nil {
				data, err := fs.ReadFile(distSub, "index.html")
				if err == nil {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(indexHTML))
			return
		}
		if fileServer != nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/accounts", s.handleAccounts)
	mux.HandleFunc("/api/accounts/scan", s.handleAccountsScan)
	mux.HandleFunc("/api/accounts/import", s.handleAccountsImport)
	mux.HandleFunc("/api/accounts/update", s.handleAccountUpdate)
	mux.HandleFunc("/api/accounts/delete", s.handleAccountDelete)
	mux.HandleFunc("/api/switch", s.handleSwitch)
	mux.HandleFunc("/api/totp", s.handleTOTP)
	mux.HandleFunc("/api/fingerprint", s.handleFingerprint)
	mux.HandleFunc("/api/cache/scan", s.handleCacheScan)
	mux.HandleFunc("/api/cache/prune", s.handleCachePrune)
	mux.HandleFunc("/api/quota", s.handleQuota)
	mux.HandleFunc("/api/quota/fleet", s.handleFleetQuota)
	mux.HandleFunc("/api/rules", s.handleRules)
	mux.HandleFunc("/api/rules/auto_switch", s.handleAutoSwitch)
	mux.HandleFunc("/api/gui/config", s.handleGUIConfig)
	mux.HandleFunc("/api/gui/projects", s.handleGUIProjects)
	mux.HandleFunc("/api/gui/projects/color", s.handleGUIProjectColor)
	mux.HandleFunc("/api/gui/projects/delete", s.handleGUIProjectDelete)
	mux.HandleFunc("/api/gui/projects/order", s.handleGUIProjectOrder)
	mux.HandleFunc("/api/gui/projects/archived", s.handleGUIProjectsArchived)
	mux.HandleFunc("/api/gui/projects/archive", s.handleGUIProjectsArchive)
	mux.HandleFunc("/api/gui/projects/restore", s.handleGUIProjectsRestore)
	mux.HandleFunc("/api/gui/projects/open_settings", s.handleGUIProjectsOpenSettings)
	mux.HandleFunc("/api/gui/color", s.handleGUIProjectColor)
	mux.HandleFunc("/api/gui/color/delete", s.handleGUIProjectDelete)
	mux.HandleFunc("/api/gui/reorder", s.handleGUIProjectOrder)
	mux.HandleFunc("/api/gui/apply", s.handleGUIApply)
	mux.HandleFunc("/api/gui/desktop/install", s.handleGUIDesktopInstall)
	mux.HandleFunc("/api/gui/desktop/restore", s.handleGUIDesktopRestore)
	mux.HandleFunc("/api/gui/desktop/status", s.handleGUIDesktopStatus)

	// System installations & updates
	mux.HandleFunc("/api/system/installations", s.handleSystemInstallations)
	mux.HandleFunc("/api/system/check_updates", s.handleSystemCheckUpdates)

	// Custom models provider
	mux.HandleFunc("/api/custom_models", s.handleCustomModels)
	mux.HandleFunc("/api/custom_models/presets", s.handleCustomModelPresets)
	mux.HandleFunc("/api/custom_models/test", s.handleCustomModelTest)
	mux.HandleFunc("/api/custom_models/bind", s.handleCustomModelBind)

	// App Enhancements (Prompt Jump Bar, Tool Density, Breaker Line)
	mux.HandleFunc("/api/enhancements", s.handleEnhancements)
	mux.HandleFunc("/api/enhancements/update", s.handleEnhancementsUpdate)
	mux.HandleFunc("/api/enhancements/apply", s.handleEnhancementsApply)

	// Scheduled Task Templates (Automations)
	mux.HandleFunc("/api/templates", s.handleTemplates)
	mux.HandleFunc("/api/templates/deploy", s.handleTemplatesDeploy)
	mux.HandleFunc("/api/templates/sidecars", s.handleTemplatesSidecars)

	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.addr = l.Addr().String()

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go s.httpServer.Serve(l)
	return nil
}

// Stop gracefully shuts down the web server.
func (s *Server) Stop() error {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Addr returns the network address the server is listening on.
func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var status map[string]interface{}
	if err := s.client.Call("swiss.getStatus", nil, &status); err != nil {
		// Standalone mode fallback
		store, _ := keyring.NewStore("")
		active := ""
		total := 0
		if store != nil {
			active = store.ActiveAccount()
			total = len(store.ListAccounts())
		}
		shield := process.NewShield(0)
		procs, _ := shield.FindAntigravityProcesses()
		hostRunning := len(procs) > 0
		var hostPID int
		if hostRunning {
			hostPID = procs[0].PID
		}
		status = map[string]interface{}{
			"daemon_running":      false,
			"daemon_pid":          0,
			"version":             core.AppVersion,
			"active_account":      active,
			"total_accounts":      total,
			"antigravity_running": hostRunning,
			"antigravity_pid":     hostPID,
		}
	}
	writeJSON(w, status)
}

func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	var accounts []interface{}
	if err := s.client.Call("swiss.listAccounts", nil, &accounts); err != nil {
		store, _ := keyring.NewStore("")
		if store != nil {
			writeJSON(w, store.ListAccounts())
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, accounts)
}

func (s *Server) handleAccountsScan(w http.ResponseWriter, r *http.Request) {
	var disc []keyring.DiscoveredAccount
	if err := s.client.Call("swiss.scanLocalAccounts", nil, &disc); err != nil {
		store, _ := keyring.NewStore("")
		scanner := keyring.NewScanner(store)
		res, errScan := scanner.Scan()
		if errScan != nil {
			http.Error(w, errScan.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, res)
		return
	}
	writeJSON(w, disc)
}

func (s *Server) handleAccountsImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Email        string `json:"email"`
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
		Label        string `json:"label"`
		TOTPSecret   string `json:"totp_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Email == "" {
		http.Error(w, "invalid request body, missing email", http.StatusBadRequest)
		return
	}

	var res interface{}
	if err := s.client.Call("swiss.importAccount", p, &res); err != nil {
		store, errStore := keyring.NewStore("")
		if errStore != nil {
			http.Error(w, errStore.Error(), http.StatusInternalServerError)
			return
		}
		acc, errImport := store.ImportAccount(p.Email, p.RefreshToken, p.AccessToken, p.Label, p.TOTPSecret)
		if errImport != nil {
			http.Error(w, errImport.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, acc)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleAccountUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Email        string `json:"email"`
		Label        string `json:"label"`
		PlanTier     string `json:"plan_tier"`
		TOTPSecret   string `json:"totp_secret"`
		RefreshToken string `json:"refresh_token"`
		SetActive    bool   `json:"set_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Email == "" {
		http.Error(w, "invalid request body, missing email", http.StatusBadRequest)
		return
	}
	var res map[string]interface{}
	if err := s.client.Call("swiss.updateAccount", p, &res); err != nil {
		store, storeErr := keyring.NewStore("")
		if storeErr != nil {
			http.Error(w, storeErr.Error(), http.StatusInternalServerError)
			return
		}
		if err := store.UpdateAccountWithTier(p.Email, p.Label, p.PlanTier, p.TOTPSecret, p.RefreshToken, p.SetActive); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		res = map[string]interface{}{"success": true, "email": p.Email}
	}
	writeJSON(w, res)
}

func (s *Server) handleAccountDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Email == "" {
		http.Error(w, "invalid request body, missing email", http.StatusBadRequest)
		return
	}
	var res map[string]interface{}
	if err := s.client.Call("swiss.removeAccount", p, &res); err != nil {
		store, storeErr := keyring.NewStore("")
		if storeErr != nil {
			http.Error(w, storeErr.Error(), http.StatusInternalServerError)
			return
		}
		if err := store.RemoveAccount(p.Email); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		res = map[string]interface{}{"success": true, "removed": p.Email}
	}
	writeJSON(w, res)
}

func (s *Server) handleSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var res map[string]interface{}
	if err := s.client.Call("swiss.switchAccount", p, &res); err != nil {
		store, storeErr := keyring.NewStore("")
		if storeErr != nil {
			http.Error(w, storeErr.Error(), http.StatusInternalServerError)
			return
		}
		if err := store.SetActiveAccount(p.Email); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		res = map[string]interface{}{"switched": true, "account": p.Email}
	}
	writeJSON(w, res)
}

func (s *Server) handleTOTP(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if r.Method == http.MethodPost {
		var p struct {
			Email  string `json:"email"`
			Secret string `json:"secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		var res map[string]interface{}
		if err := s.client.Call("swiss.setTOTPSecret", p, &res); err != nil {
			store, storeErr := keyring.NewStore("")
			if storeErr != nil {
				http.Error(w, storeErr.Error(), http.StatusInternalServerError)
				return
			}
			if err := store.SetTOTPSecret(p.Email, p.Secret); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			res = map[string]interface{}{"success": true}
		}
		writeJSON(w, res)
		return
	}

	var res map[string]interface{}
	if err := s.client.Call("swiss.getTOTPCode", map[string]string{"email": email}, &res); err != nil {
		store, _ := keyring.NewStore("")
		target := email
		if target == "" && store != nil {
			target = store.ActiveAccount()
		}
		if store != nil {
			acc, _ := store.GetAccount(target)
			if acc != nil && acc.TOTPSecret != "" {
				eng := totp.NewEngine()
				now := time.Now()
				code, _ := eng.GenerateCode(acc.TOTPSecret, now)
				cd := eng.GetCountdown(now)
				writeJSON(w, map[string]interface{}{
					"code":              code,
					"remaining_seconds": cd.RemainingSeconds,
					"progress_fraction": cd.ProgressFraction,
					"has_totp":          true,
				})
				return
			}
		}
		writeJSON(w, map[string]interface{}{
			"code":              "------",
			"remaining_seconds": 0,
			"has_totp":          false,
		})
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleFingerprint(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if r.Method == http.MethodPost {
		var p struct {
			Email   string                    `json:"email"`
			Profile fingerprint.DeviceProfile `json:"profile"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		var res map[string]interface{}
		if err := s.client.Call("swiss.setFingerprofile", p, &res); err != nil {
			fpStore, errStore := fingerprint.NewStore("")
			if errStore != nil {
				http.Error(w, errStore.Error(), http.StatusInternalServerError)
				return
			}
			if err := fpStore.SetProfile(p.Email, &p.Profile); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			res = map[string]interface{}{"success": true}
		}
		writeJSON(w, res)
		return
	}

	if r.URL.Query().Get("action") == "generate" {
		fresh, err := fingerprint.GenerateRandom()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, fresh)
		return
	}

	var prof fingerprint.DeviceProfile
	if err := s.client.Call("swiss.getFingerprofile", map[string]string{"email": email}, &prof); err != nil {
		fpStore, errStore := fingerprint.NewStore("")
		if errStore == nil {
			target := email
			if target == "" {
				kStore, _ := keyring.NewStore("")
				if kStore != nil {
					target = kStore.ActiveAccount()
				}
			}
			p, _ := fpStore.GetOrCreateProfile(target)
			if p != nil {
				writeJSON(w, p)
				return
			}
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, prof)
}

func (s *Server) handleCacheScan(w http.ResponseWriter, r *http.Request) {
	ageStr := r.URL.Query().Get("min_age_days")
	age, _ := strconv.ParseFloat(ageStr, 64)
	if age <= 0 {
		age = 3.0
	}
	var bd cache.Breakdown
	if err := s.client.Call("swiss.scanCache", map[string]float64{"min_age_days": age}, &bd); err != nil {
		ins := cache.NewInspector("", "")
		scan, errScan := ins.ScanBreakdown(age)
		if errScan != nil {
			http.Error(w, errScan.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, scan)
		return
	}
	writeJSON(w, bd)
}

func (s *Server) handleCachePrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var opts cache.PruneOptions
	_ = json.NewDecoder(r.Body).Decode(&opts)
	if opts.MinAgeDays <= 0 {
		opts.MinAgeDays = 3.0
	}
	var res cache.PruneResult
	if err := s.client.Call("swiss.pruneCache", opts, &res); err != nil {
		pruner := cache.NewPruner("")
		pruneRes, errPrune := pruner.Prune(opts)
		if errPrune != nil {
			http.Error(w, errPrune.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, pruneRes)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request) {
	var q quota.QuotaSummary
	if err := s.client.Call("swiss.getQuotaSummary", nil, &q); err != nil {
		now := time.Now()
		models := []quota.ModelQuota{
			{ModelName: "gemini-2.5-pro", Fraction: 0.85, ResetTime: now.Add(4 * time.Hour), ResetText: quota.FormatResetHorizon(now.Add(4*time.Hour), now), HealthStatus: core.StatusHealthy},
			{ModelName: "gemini-2.5-flash", Fraction: 0.92, ResetTime: now.Add(2 * time.Hour), ResetText: quota.FormatResetHorizon(now.Add(2*time.Hour), now), HealthStatus: core.StatusHealthy},
			{ModelName: "gemini-1.5-pro", Fraction: 0.45, ResetTime: now.Add(1 * time.Hour), ResetText: quota.FormatResetHorizon(now.Add(1*time.Hour), now), HealthStatus: core.StatusHealthy},
		}
		q = quota.QuotaSummary{
			AccountEmail:  "active",
			Models:        models,
			MinFraction:   0.45,
			OverallHealth: core.StatusHealthy,
		}
	}
	writeJSON(w, q)
}

func (s *Server) handleFleetQuota(w http.ResponseWriter, r *http.Request) {
	var summary quota.FleetQuotaSummary
	if err := s.client.Call("swiss.getFleetQuota", nil, &summary); err != nil {
		store, _ := keyring.NewStore("")
		var accounts []*keyring.Account
		active := ""
		if store != nil {
			accounts = store.ListAccounts()
			active = store.ActiveAccount()
		}
		now := time.Now()
		models := []quota.ModelQuota{
			{ModelName: "gemini-2.5-pro", Fraction: 0.85, ResetTime: now.Add(4 * time.Hour), ResetText: quota.FormatResetHorizon(now.Add(4*time.Hour), now), HealthStatus: core.StatusHealthy},
			{ModelName: "gemini-2.5-flash", Fraction: 0.92, ResetTime: now.Add(2 * time.Hour), ResetText: quota.FormatResetHorizon(now.Add(2*time.Hour), now), HealthStatus: core.StatusHealthy},
			{ModelName: "gemini-1.5-pro", Fraction: 0.45, ResetTime: now.Add(1 * time.Hour), ResetText: quota.FormatResetHorizon(now.Add(1*time.Hour), now), HealthStatus: core.StatusHealthy},
		}
		activeSummary := &quota.QuotaSummary{
			AccountEmail:  active,
			Models:        models,
			MinFraction:   0.45,
			OverallHealth: core.StatusHealthy,
			LastPolled:    now,
		}
		states := quota.BuildAccountQuotaStates(accounts, activeSummary)
		summary = quota.ComputeFleetSummary(states, active)
	}
	writeJSON(w, summary)
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var p map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		var res map[string]interface{}
		if err := s.client.Call("swiss.setRuleConfig", p, &res); err != nil {
			c, errCfg := core.LoadConfig()
			if errCfg != nil {
				http.Error(w, errCfg.Error(), http.StatusInternalServerError)
				return
			}
			if val, ok := p["auto_switch_enabled"].(bool); ok {
				c.AutoSwitchEnabled = val
			}
			if val, ok := p["auto_switch_threshold"].(float64); ok {
				c.AutoSwitchThreshold = val
			}
			if val, ok := p["polling_interval_seconds"].(float64); ok {
				c.PollingIntervalSec = int(val)
			}
			if val, ok := p["warmup_enabled"].(bool); ok {
				c.WarmupEnabled = val
			}
			if val, ok := p["warmup_lead_time_seconds"].(float64); ok {
				c.WarmupLeadTimeSec = val
			}
			if err := c.Save(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			res = map[string]interface{}{"success": true}
		}
		writeJSON(w, res)
		return
	}

	var cfg map[string]interface{}
	if err := s.client.Call("swiss.getRuleConfig", nil, &cfg); err != nil {
		c, _ := core.LoadConfig()
		cfg = map[string]interface{}{
			"auto_switch_enabled":       c.AutoSwitchEnabled,
			"auto_switch_threshold":     c.AutoSwitchThreshold,
			"polling_interval_seconds":  c.PollingIntervalSec,
			"warmup_enabled":            c.WarmupEnabled,
			"warmup_lead_time_seconds":  c.WarmupLeadTimeSec,
		}
	}
	writeJSON(w, cfg)
}

func (s *Server) handleAutoSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		var cfg map[string]interface{}
		if err := s.client.Call("swiss.getRuleConfig", nil, &cfg); err != nil {
			c, _ := core.LoadConfig()
			writeJSON(w, map[string]interface{}{
				"auto_switch_enabled": c.AutoSwitchEnabled,
			})
			return
		}
		writeJSON(w, cfg)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	var res map[string]interface{}
	callPayload := map[string]interface{}{"auto_switch_enabled": p.Enabled}
	if err := s.client.Call("swiss.setRuleConfig", callPayload, &res); err != nil {
		c, errCfg := core.LoadConfig()
		if errCfg != nil {
			http.Error(w, errCfg.Error(), http.StatusInternalServerError)
			return
		}
		c.AutoSwitchEnabled = p.Enabled
		if err := c.Save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		res = map[string]interface{}{"success": true, "auto_switch_enabled": p.Enabled}
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var cfg gui.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		var res gui.Config
		if err := s.client.Call("swiss.setGUIConfig", cfg, &res); err != nil {
			if s.guiStore != nil {
				if errUpdate := s.guiStore.UpdateConfig(&cfg); errUpdate != nil {
					http.Error(w, errUpdate.Error(), http.StatusInternalServerError)
					return
				}
				writeJSON(w, s.guiStore.GetConfig())
				return
			}
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, res)
		return
	}

	var cfg gui.Config
	if err := s.client.Call("swiss.getGUIConfig", nil, &cfg); err != nil {
		if s.guiStore != nil {
			writeJSON(w, s.guiStore.GetConfig())
			return
		}
		writeJSON(w, gui.DefaultConfig())
		return
	}
	writeJSON(w, cfg)
}

func (s *Server) handleGUIProjects(w http.ResponseWriter, r *http.Request) {
	var projects []gui.ProjectItem
	if err := s.client.Call("swiss.getGUIProjects", nil, &projects); err != nil {
		if s.guiStore != nil {
			projs, errDetect := s.guiStore.DetectProjects()
			if errDetect != nil {
				http.Error(w, errDetect.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, projs)
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, projects)
}

func (s *Server) handleGUIProjectColor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Name == "" || p.Color == "" {
		http.Error(w, "invalid parameters, name and color are required", http.StatusBadRequest)
		return
	}

	var res interface{}
	if err := s.client.Call("swiss.setGUIProjectColor", p, &res); err != nil {
		if s.guiStore != nil {
			if errSet := s.guiStore.SetProjectColor(p.Name, p.Color); errSet != nil {
				http.Error(w, errSet.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{"success": true, "name": p.Name, "color": p.Color})
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIProjectDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Name == "" {
		http.Error(w, "invalid parameters, name is required", http.StatusBadRequest)
		return
	}

	if s.guiStore != nil {
		_ = s.guiStore.DeleteProject(p.Name)
	}

	var res interface{}
	if err := s.client.Call("swiss.removeGUIProjectColor", p, &res); err != nil {
		if s.guiStore != nil {
			if errRem := s.guiStore.RemoveProjectColor(p.Name); errRem != nil {
				http.Error(w, errRem.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{"success": true, "name": p.Name})
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIProjectsArchived(w http.ResponseWriter, r *http.Request) {
	if s.guiStore == nil {
		writeJSON(w, []gui.ArchivedProjectItem{})
		return
	}
	archived, err := s.guiStore.GetArchivedProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if archived == nil {
		archived = []gui.ArchivedProjectItem{}
	}
	writeJSON(w, archived)
}

func (s *Server) handleGUIProjectsArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	target := p.Name
	if target == "" {
		target = p.ID
	}
	if target == "" {
		http.Error(w, "project name or id required", http.StatusBadRequest)
		return
	}
	if s.guiStore != nil {
		if err := s.guiStore.ArchiveProject(target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]interface{}{"success": true, "archived": target})
}

func (s *Server) handleGUIProjectsRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	target := p.Name
	if target == "" {
		target = p.ID
	}
	if target == "" {
		http.Error(w, "project name or id required", http.StatusBadRequest)
		return
	}
	if s.guiStore != nil {
		if err := s.guiStore.RestoreProject(target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]interface{}{"success": true, "restored": target})
}

func (s *Server) handleGUIProjectsOpenSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	target := p.Name
	if target == "" {
		target = p.ID
	}
	if target == "" {
		http.Error(w, "project name or id required", http.StatusBadRequest)
		return
	}
	if s.guiStore != nil {
		if err := s.guiStore.OpenProjectSettings(target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]interface{}{"success": true, "project": target})
}

func (s *Server) handleGUIProjectOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Order []string `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid parameters, order array required", http.StatusBadRequest)
		return
	}

	var res interface{}
	if err := s.client.Call("swiss.updateGUIProjectOrder", p, &res); err != nil {
		if s.guiStore != nil {
			if errOrder := s.guiStore.UpdateProjectOrder(p.Order); errOrder != nil {
				http.Error(w, errOrder.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{"success": true, "order": p.Order})
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIApply(w http.ResponseWriter, r *http.Request) {
	var res gui.ApplyResult
	if err := s.client.Call("swiss.applyGUIStyles", nil, &res); err != nil {
		if s.guiStore != nil {
			applyRes, errApply := s.guiStore.Apply()
			if errApply != nil && applyRes == nil {
				http.Error(w, errApply.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, applyRes)
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIDesktopInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var res gui.ApplyResult
	if err := s.client.Call("swiss.installDesktopLoader", nil, &res); err != nil {
		if s.guiStore != nil {
			instRes, errInst := s.guiStore.InstallDesktopLoader()
			if errInst != nil && instRes == nil {
				http.Error(w, errInst.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, instRes)
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIDesktopRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var res gui.ApplyResult
	if err := s.client.Call("swiss.restoreFactoryDefaults", nil, &res); err != nil {
		if s.guiStore != nil {
			restRes, errRest := s.guiStore.RestoreFactoryDefaults()
			if errRest != nil && restRes == nil {
				http.Error(w, errRest.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, restRes)
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleGUIDesktopStatus(w http.ResponseWriter, r *http.Request) {
	var status gui.DesktopStatus
	if err := s.client.Call("swiss.getDesktopStatus", nil, &status); err != nil {
		if s.guiStore != nil {
			writeJSON(w, s.guiStore.GetDesktopStatus())
			return
		}
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleSystemInstallations(w http.ResponseWriter, r *http.Request) {
	if s.systemDetector == nil {
		s.systemDetector = system.NewDetector()
	}
	res := s.systemDetector.DetectAll()
	writeJSON(w, res)
}

func (s *Server) handleSystemCheckUpdates(w http.ResponseWriter, r *http.Request) {
	if s.systemDetector == nil {
		s.systemDetector = system.NewDetector()
	}
	res, err := s.systemDetector.CheckUpdates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleCustomModels(w http.ResponseWriter, r *http.Request) {
	if s.customModelsStore == nil {
		var err error
		s.customModelsStore, err = custommodels.NewStore("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		cfg := s.customModelsStore.GetConfig()
		writeJSON(w, cfg)
	case http.MethodPost:
		var m custommodels.CustomModel
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(m.ID) == "" {
			slug := strings.ToLower(strings.TrimSpace(m.Name))
			slug = strings.ReplaceAll(slug, " ", "-")
			slug = strings.ReplaceAll(slug, "/", "-")
			slug = strings.ReplaceAll(slug, ":", "-")
			if slug == "" {
				slug = "model"
			}
			prefix := string(m.ProviderType)
			if prefix == "" {
				prefix = "custom"
			}
			m.ID = fmt.Sprintf("%s-%s-%d", prefix, slug, time.Now().Unix())
		}
		if err := s.customModelsStore.SaveModel(m); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "model": m})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			var p struct {
				ID string `json:"id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&p)
			id = p.ID
		}
		if id == "" {
			http.Error(w, "missing model id parameter", http.StatusBadRequest)
			return
		}
		if err := s.customModelsStore.DeleteModel(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "deleted": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCustomModelPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, custommodels.GetPresets())
}

func (s *Server) handleCustomModelTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var m custommodels.CustomModel
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if s.customModelsTester == nil {
		s.customModelsTester = custommodels.NewTester()
	}
	res, err := s.customModelsTester.TestEndpoint(m)
	if err != nil {
		writeJSON(w, res)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleCustomModelBind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p struct {
		Project string `json:"project"`
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Project == "" {
		http.Error(w, "invalid request body, missing project", http.StatusBadRequest)
		return
	}
	if s.customModelsStore == nil {
		var err error
		s.customModelsStore, err = custommodels.NewStore("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err := s.customModelsStore.BindProject(p.Project, p.ModelID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "project": p.Project, "model_id": p.ModelID})
}

// Enhancements handlers

func (s *Server) handleEnhancements(w http.ResponseWriter, r *http.Request) {
	if s.enhancementsStore == nil {
		var err error
		s.enhancementsStore, err = enhancements.NewStore("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if r.Method == http.MethodGet {
		writeJSON(w, s.enhancementsStore.GetConfig())
		return
	}

	if r.Method == http.MethodPost {
		var cfg enhancements.EnhancementsConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := s.enhancementsStore.UpdateConfig(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, s.enhancementsStore.GetConfig())
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleEnhancementsUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleEnhancements(w, r)
}

func (s *Server) handleEnhancementsApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Re-apply styles through GUI store which includes enhancements script injection
	if s.guiStore != nil {
		res, err := s.guiStore.Apply()
		if err != nil && res == nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, res)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "message": "Enhancements configuration updated"})
}

// Scheduled Task Templates handlers

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if s.templatesStore == nil {
		var err error
		s.templatesStore, err = templates.NewStore("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	templateID := r.URL.Query().Get("id")
	if templateID != "" {
		tmpl, err := s.templatesStore.GetTemplateByID(templateID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, tmpl)
		return
	}

	writeJSON(w, s.templatesStore.GetTemplates())
}

func (s *Server) handleTemplatesDeploy(w http.ResponseWriter, r *http.Request) {
	if s.templatesStore == nil {
		var err error
		s.templatesStore, err = templates.NewStore("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req templates.DeployTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	info, err := s.templatesStore.DeploySidecar(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"task":    info,
	})
}

func (s *Server) handleTemplatesSidecars(w http.ResponseWriter, r *http.Request) {
	if s.templatesStore == nil {
		var err error
		s.templatesStore, err = templates.NewStore("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if r.Method == http.MethodGet {
		tasks, err := s.templatesStore.ListSidecars()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, tasks)
		return
	}

	if r.Method == http.MethodDelete {
		id := r.URL.Query().Get("id")
		if id == "" {
			var body struct {
				ID string `json:"id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			id = body.ID
		}
		if id == "" {
			http.Error(w, "missing task id parameter", http.StatusBadRequest)
			return
		}

		if err := s.templatesStore.DeleteSidecar(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "deleted": id})
		return
	}

	if r.Method == http.MethodPost {
		// Support action: "delete" in POST
		var body struct {
			Action string `json:"action"`
			ID     string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Action == "delete" && body.ID != "" {
			if err := s.templatesStore.DeleteSidecar(body.ID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{"success": true, "deleted": body.ID})
			return
		}
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

//go:embed index.html
var indexHTML string
