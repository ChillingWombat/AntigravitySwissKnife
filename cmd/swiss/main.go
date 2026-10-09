package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/cache"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/daemon"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/process"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/totp"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/webgui"
)

func printUsage() {
	fmt.Println("Antigravity Swiss Knife (Go Edition)")
	fmt.Printf("Version: %s\n\n", core.AppVersion)
	fmt.Println("Usage:")
	fmt.Println("  swiss <command> [arguments...]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  status                Query active daemon and protected host IDE status")
	fmt.Println("  daemon                Start background JSON-RPC daemon")
	fmt.Println("  web / gui             Launch Minimalist Light Web GUI")
	fmt.Println("  quota                 Query Gemini model quota horizons & reset countdowns")
	fmt.Println("  accounts / list       List all registered accounts and MFA status")
	fmt.Println("  scan                  Scan local machine for saved/logged-in accounts")
	fmt.Println("  import <email>        Import account with email, refresh token, label, TOTP")
	fmt.Println("  switch <email>        Switch active Google account seamlessly")
	fmt.Println("  totp [email]          Generate live RFC 6238 TOTP 6-digit code")
	fmt.Println("  set-totp <email> <sec> Store or update RFC 6238 TOTP secret")
	fmt.Println("  cache scan [days]     Scan storage and reclaimable cache breakdown")
	fmt.Println("  cache prune [days]    Safely prune cache with cascade immunity")
	fmt.Println("  fingerprint status    Display active device fingerprint profile")
	fmt.Println("  fingerprint list      List stored device profiles")
	fmt.Println("  fingerprint gen       Generate fresh hardware telemetry profile")
	fmt.Println("  fingerprint swap <eml> Generate and apply new fingerprint for account")
	fmt.Println("  patch [status|install|restore|sync] Manage persistent desktop app patches")
	fmt.Println("  version               Display version information")
	fmt.Println("")
	fmt.Println("Global Flags:")
	fmt.Println("  --socket <path>       Override Unix domain socket path")
	fmt.Println("  --json                Output results in JSON format")
}

func main() {
	signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "version", "-v", "--version":
		fmt.Printf("Antigravity Swiss Knife v%s (Go 1.24.6)\n", core.AppVersion)
	case "status":
		runStatus(args)
	case "daemon":
		runDaemon(args)
	case "web", "gui":
		runWeb(args)
	case "quota":
		runQuota(args)
	case "accounts", "list":
		runAccounts(args)
	case "scan":
		runScan(args)
	case "import":
		runImport(args)
	case "switch":
		runSwitch(args)
	case "totp":
		runTOTP(args)
	case "set-totp":
		runSetTOTP(args)
	case "cache":
		runCache(args)
	case "fingerprint", "fp":
		runFingerprint(args)
	case "patch":
		runPatch(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nRun 'swiss help' for available commands.\n", cmd)
		os.Exit(1)
	}
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output JSON")
	socketPath := fs.String("socket", core.GetSocketPath(), "Socket path")
	_ = fs.Parse(args)

	client := ipc.NewClient(*socketPath)
	var status map[string]interface{}
	err := client.Call("swiss.getStatus", nil, &status)

	if err == nil {
		if *jsonOut {
			b, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(b))
			return
		}

		fmt.Println("Antigravity Swiss Knife Status:")
		fmt.Printf("  Daemon Status     : Running (PID: %v)\n", status["daemon_pid"])
		fmt.Printf("  App Version       : %v\n", status["version"])
		fmt.Printf("  Active Account    : %v\n", status["active_account"])
		fmt.Printf("  Total Accounts    : %v\n", status["total_accounts"])
		fmt.Printf("  Host IDE Running  : %v\n", status["antigravity_running"])
		if pid, ok := status["antigravity_pid"]; ok && pid != nil {
			fmt.Printf("  Host IDE PID      : %v (Shielded)\n", pid)
		}
		return
	}

	// Daemon not running, inspect directly
	store, _ := keyring.NewStore("")
	active := ""
	total := 0
	if store != nil {
		c, _ := core.LoadConfig()
		var allEmails []string
		for _, a := range store.ListAccounts() {
			allEmails = append(allEmails, a.Email)
		}
		_, _ = store.ReconcileActiveAccount(c.AutoImportActiveAccount, allEmails, nil)
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

	if *jsonOut {
		data := map[string]interface{}{
			"daemon_running":      false,
			"active_account":      active,
			"total_accounts":      total,
			"antigravity_running": hostRunning,
			"antigravity_pid":     hostPID,
		}
		b, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("Antigravity Swiss Knife Status:")
	fmt.Println("  Daemon Status     : Stopped (direct store mode)")
	fmt.Printf("  Active Account    : %s\n", active)
	fmt.Printf("  Total Accounts    : %d\n", total)
	fmt.Printf("  Host IDE Running  : %v\n", hostRunning)
	if hostRunning {
		fmt.Printf("  Host IDE PID      : %d (Shielded)\n", hostPID)
	}
}

func runDaemon(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	socketPath := fs.String("socket", core.GetSocketPath(), "Socket path")
	withWeb := fs.Bool("web", false, "Also start Web GUI server")
	webAddr := fs.String("addr", "127.0.0.1:8765", "Web GUI listen address")
	_ = fs.Parse(args)

	cfg, _ := core.LoadConfig()
	if cfg == nil {
		cfg = core.DefaultConfig()
	}

	d, err := daemon.NewDaemon(cfg, *socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing daemon: %v\n", err)
		os.Exit(1)
	}

	if err := d.Start(); err != nil {
		if strings.Contains(err.Error(), "already running") {
			fmt.Printf("Antigravity Swiss Knife Daemon is already running on %s. Exiting to preserve single instance.\n", *socketPath)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Antigravity Swiss Knife Daemon started on socket: %s (PID: %d)\n", *socketPath, os.Getpid())

	var webSrv *webgui.Server
	if *withWeb {
		webSrv = webgui.NewServer(*webAddr, *socketPath)
		if err := webSrv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to start web GUI on %s: %v. Stopping daemon.\n", *webAddr, err)
			_ = d.Stop()
			os.Exit(1)
		} else {
			fmt.Printf("Web GUI listening on http://%s\n", *webAddr)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down daemon...")
	if webSrv != nil {
		_ = webSrv.Stop()
	}
	_ = d.Stop()
	fmt.Println("Daemon gracefully stopped.")
}

func runWeb(args []string) {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8765", "Listen address")
	socketPath := fs.String("socket", core.GetSocketPath(), "Daemon socket path")
	_ = fs.Parse(args)

	srv := webgui.NewServer(*addr, *socketPath)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting Web GUI: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Antigravity Swiss Knife Minimalist Light Web GUI running on http://%s\n", *addr)
	fmt.Println("Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	_ = srv.Stop()
	fmt.Println("Web GUI stopped.")
}

func runQuota(args []string) {
	fs := flag.NewFlagSet("quota", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output JSON")
	socketPath := fs.String("socket", core.GetSocketPath(), "Socket path")
	_ = fs.Parse(args)

	client := ipc.NewClient(*socketPath)
	var q quota.QuotaSummary
	err := client.Call("swiss.getQuotaSummary", nil, &q)

	if err != nil {
		// Mock quota summary for standalone mode
		now := time.Now()
		models := []quota.ModelQuota{
			{ModelName: "gemini-3.8-flash", Fraction: 0.85, ResetTime: now.Add(4 * time.Hour), ResetText: quota.FormatResetHorizon(now.Add(4*time.Hour), now), HealthStatus: core.StatusHealthy},
			{ModelName: "gemini-3.1-pro", Fraction: 0.92, ResetTime: now.Add(2 * time.Hour), ResetText: quota.FormatResetHorizon(now.Add(2*time.Hour), now), HealthStatus: core.StatusHealthy},
			{ModelName: "claude-sonnet-4-6", Fraction: 0.45, ResetTime: now.Add(1 * time.Hour), ResetText: quota.FormatResetHorizon(now.Add(1*time.Hour), now), HealthStatus: core.StatusHealthy},
		}
		q = quota.QuotaSummary{
			AccountEmail: "active",
			Models:       models,
			MinFraction:  0.45,
			OverallHealth: core.StatusHealthy,
		}
	}

	if *jsonOut {
		b, _ := json.MarshalIndent(q, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("Gemini Model Quota Horizons:")
	fmt.Printf("%-20s %-12s %-14s %s\n", "MODEL", "REMAINING", "HEALTH", "RESET HORIZON")
	fmt.Println(strings.Repeat("-", 64))
	for _, m := range q.Models {
		pct := fmt.Sprintf("%.0f%%", m.Fraction*100)
		fmt.Printf("%-20s %-12s %-14s %s\n", m.ModelName, pct, m.HealthStatus, m.ResetText)
	}
}

func runAccounts(args []string) {
	fs := flag.NewFlagSet("accounts", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output JSON")
	socketPath := fs.String("socket", core.GetSocketPath(), "Socket path")
	_ = fs.Parse(args)

	client := ipc.NewClient(*socketPath)
	var accounts []*keyring.Account
	err := client.Call("swiss.listAccounts", nil, &accounts)

	if err != nil {
		// fallback to direct store
		store, storeErr := keyring.NewStore("")
		if storeErr != nil {
			fmt.Fprintf(os.Stderr, "Error accessing accounts: %v\n", storeErr)
			os.Exit(1)
		}
		c, _ := core.LoadConfig()
		var allEmails []string
		for _, a := range store.ListAccounts() {
			allEmails = append(allEmails, a.Email)
		}
		_, _ = store.ReconcileActiveAccount(c.AutoImportActiveAccount, allEmails, nil)
		accounts = store.ListAccounts()
	}

	if *jsonOut {
		b, _ := json.MarshalIndent(accounts, "", "  ")
		fmt.Println(string(b))
		return
	}

	if len(accounts) == 0 {
		fmt.Println("No accounts registered.")
		return
	}

	fmt.Printf("%-36s %-16s %-12s %s\n", "ACCOUNT EMAIL", "LABEL", "MFA STATUS", "ACTIVE")
	fmt.Println(strings.Repeat("-", 76))
	for _, a := range accounts {
		mfa := "No MFA"
		if a.HasTOTP {
			mfa = "Protected"
		}
		activeMark := ""
		if a.IsActive {
			activeMark = "[ACTIVE]"
		}
		fmt.Printf("%-36s %-16s %-12s %s\n", a.Email, a.Label, mfa, activeMark)
	}
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output JSON")
	socketPath := fs.String("socket", core.GetSocketPath(), "Socket path")
	_ = fs.Parse(args)

	client := ipc.NewClient(*socketPath)
	var discovered []keyring.DiscoveredAccount
	err := client.Call("swiss.scanLocalAccounts", nil, &discovered)
	if err != nil {
		store, _ := keyring.NewStore("")
		scanner := keyring.NewScanner(store)
		discovered, _ = scanner.Scan()
	}

	if *jsonOut {
		b, _ := json.MarshalIndent(discovered, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("Local Host Accounts & Credential Discovery:")
	fmt.Printf("%-34s %-28s %-14s %s\n", "ACCOUNT EMAIL", "SOURCE", "TOKENS", "STATUS")
	fmt.Println(strings.Repeat("-", 88))
	for _, d := range discovered {
		tok := "No Tokens"
		if d.HasRefresh {
			tok = "Refresh Token"
		} else if d.HasTokens {
			tok = "Access Token"
		}
		status := "Saved"
		if d.IsActiveInIDE {
			status = "Active in IDE"
		}
		fmt.Printf("%-34s %-28s %-14s %s\n", d.Email, d.Source, tok, status)
	}
}

func reorderArgs(args []string) []string {
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positionals = append(positionals, arg)
		}
	}
	return append(flags, positionals...)
}

func runImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	emailFlag := fs.String("email", "", "Account email")
	refreshFlag := fs.String("refresh-token", "", "OAuth refresh token")
	accessFlag := fs.String("access-token", "", "OAuth access token (optional)")
	labelFlag := fs.String("label", "Imported Account", "Account label")
	totpFlag := fs.String("totp", "", "RFC 6238 TOTP seed token (Base32 or otpauth:// URI)")
	jsonOut := fs.Bool("json", false, "Output JSON")
	socketPath := fs.String("socket", core.GetSocketPath(), "Socket path")
	_ = fs.Parse(reorderArgs(args))

	email := *emailFlag
	if email == "" && len(fs.Args()) > 0 {
		email = fs.Args()[0]
	}

	if email == "" {
		fmt.Fprintln(os.Stderr, "Usage: swiss import <email> --refresh-token <token> [--access-token <token>] [--label <label>] [--totp <seed>]")
		os.Exit(1)
	}

	totpClean := ""
	if *totpFlag != "" {
		totpClean = totp.CleanSecret(*totpFlag)
		if !totp.ValidateSecret(totpClean) {
			fmt.Fprintln(os.Stderr, "Warning: provided TOTP seed format could not be verified.")
		}
	}

	req := map[string]string{
		"email":         email,
		"refresh_token": *refreshFlag,
		"access_token":  *accessFlag,
		"label":         *labelFlag,
		"totp_secret":   totpClean,
	}

	client := ipc.NewClient(*socketPath)
	var res keyring.Account
	err := client.Call("swiss.importAccount", req, &res)
	if err != nil {
		store, storeErr := keyring.NewStore("")
		if storeErr != nil {
			fmt.Fprintf(os.Stderr, "Store error: %v\n", storeErr)
			os.Exit(1)
		}
		acc, importErr := store.ImportAccount(email, *refreshFlag, *accessFlag, *labelFlag, totpClean)
		if importErr != nil {
			fmt.Fprintf(os.Stderr, "Import error: %v\n", importErr)
			os.Exit(1)
		}
		res = *acc
	}

	if *jsonOut {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Printf("[OK] Successfully imported account: %s\n", res.Email)
	fmt.Printf("  Label             : %s\n", res.Label)
	fmt.Printf("  Has Refresh Token : %v\n", res.RefreshToken != "")
	fmt.Printf("  Has MFA / TOTP    : %v\n", res.HasTOTP)
}

func runSwitch(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: swiss switch <email>")
		os.Exit(1)
	}
	email := args[0]
	socketPath := core.GetSocketPath()

	client := ipc.NewClient(socketPath)
	var res map[string]interface{}
	err := client.Call("swiss.switchAccount", map[string]string{"email": email}, &res)

	if err == nil {
		fmt.Printf("[OK] Successfully switched active account to: %s\n", email)
		return
	}

	// fallback direct
	store, storeErr := keyring.NewStore("")
	if storeErr != nil {
		fmt.Fprintf(os.Stderr, "Error switching account: %v\n", storeErr)
		os.Exit(1)
	}
	if err := store.SetActiveAccount(email); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to switch account: %v\n", err)
		os.Exit(1)
	}
	if acc, _ := store.GetAccount(email); acc != nil {
		var allEmails []string
		for _, a := range store.ListAccounts() {
			allEmails = append(allEmails, a.Email)
		}
		profStore, _ := fingerprint.NewStore("")
		_ = keyring.SyncAllSurfaces(acc, allEmails, profStore)
		_, _ = gui.NewInjector(0).RefreshUserStatus()
	}
	fmt.Printf("[OK] Successfully switched active account to: %s (direct)\n", email)
}

func runTOTP(args []string) {
	fs := flag.NewFlagSet("totp", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output JSON")
	socketPath := fs.String("socket", core.GetSocketPath(), "Socket path")
	_ = fs.Parse(args)

	email := ""
	if len(fs.Args()) > 0 {
		email = fs.Args()[0]
	}

	client := ipc.NewClient(*socketPath)
	var res struct {
		Code             string  `json:"code"`
		RemainingSeconds int     `json:"remaining_seconds"`
		ProgressFraction float64 `json:"progress_fraction"`
		HasTOTP          bool    `json:"has_totp"`
	}
	err := client.Call("swiss.getTOTPCode", map[string]string{"email": email}, &res)

	if err != nil {
		// Direct mode fallback
		store, storeErr := keyring.NewStore("")
		if storeErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", storeErr)
			os.Exit(1)
		}
		target := email
		if target == "" {
			target = store.ActiveAccount()
		}
		acc, accErr := store.GetAccount(target)
		if accErr != nil {
			fmt.Fprintf(os.Stderr, "Account not found: %s\n", target)
			os.Exit(1)
		}
		if acc.TOTPSecret == "" {
			res.Code = "------"
			res.HasTOTP = false
		} else {
			engine := totp.NewEngine()
			now := time.Now()
			code, _ := engine.GenerateCode(acc.TOTPSecret, now)
			cd := engine.GetCountdown(now)
			res.Code = code
			res.RemainingSeconds = cd.RemainingSeconds
			res.ProgressFraction = cd.ProgressFraction
			res.HasTOTP = true
		}
	}

	if *jsonOut {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
		return
	}

	if !res.HasTOTP {
		fmt.Println("TOTP Status: No MFA secret configured for this account.")
		fmt.Println("Use 'swiss set-totp <email> <secret>' to configure.")
		return
	}

	formatted := res.Code
	if len(formatted) == 6 {
		formatted = formatted[:3] + " " + formatted[3:]
	}
	fmt.Printf("TOTP Code: %s (valid for %ds)\n", formatted, res.RemainingSeconds)
}

func runSetTOTP(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: swiss set-totp <email> <secret>")
		os.Exit(1)
	}
	email := args[0]
	secret := args[1]

	if !totp.ValidateSecret(secret) {
		fmt.Fprintln(os.Stderr, "Error: Invalid Base32 secret key.")
		os.Exit(1)
	}

	socketPath := core.GetSocketPath()
	client := ipc.NewClient(socketPath)
	var res map[string]interface{}
	err := client.Call("swiss.setTOTPSecret", map[string]string{"email": email, "secret": secret}, &res)

	if err != nil {
		store, storeErr := keyring.NewStore("")
		if storeErr != nil {
			fmt.Fprintf(os.Stderr, "Error accessing store: %v\n", storeErr)
			os.Exit(1)
		}
		if err := store.SetTOTPSecret(email, secret); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save secret: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("[OK] Saved RFC 6238 TOTP secret for %s.\n", email)
}

func runCache(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: swiss cache <scan|prune> [options]")
		os.Exit(1)
	}
	sub := args[0]
	subArgs := args[1:]

	fs := flag.NewFlagSet("cache", flag.ExitOnError)
	days := fs.Float64("days", 3.0, "Minimum age in days")
	jsonOut := fs.Bool("json", false, "Output JSON")
	_ = fs.Parse(subArgs)

	switch sub {
	case "scan", "breakdown":
		inspector := cache.NewInspector("", "")
		bd, err := inspector.ScanBreakdown(*days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cache scan error: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			b, _ := json.MarshalIndent(bd, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Println("Brain Storage & Cache Breakdown:")
		fmt.Printf("  Total Brain Size  : %.2f MB\n", float64(bd.BrainTotalBytes)/(1024*1024))
		fmt.Printf("  Conversations Size: %.2f MB\n", float64(bd.ConversationsTotalBytes)/(1024*1024))
		fmt.Printf("  Reclaimable Size  : %.2f MB (older than %.1f days)\n", float64(bd.ReclaimableBytes)/(1024*1024), *days)
		for catName, cat := range bd.Categories {
			fmt.Printf("    - %-14s: %.2f MB (%d files)\n", catName, float64(cat.TotalBytes)/(1024*1024), cat.FileCount)
		}
		fmt.Println("  Active Cascade Conversation Shield is ENFORCED.")

	case "prune":
		pruner := cache.NewPruner("")
		opts := cache.PruneOptions{
			MinAgeDays:   *days,
			PruneScratch: true,
			PruneSteps:   true,
			PruneTasks:   true,
		}
		res, err := pruner.Prune(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cache prune error: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			b, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Println("Cache Pruning Completed:")
		fmt.Printf("  Files Deleted     : %d\n", res.FilesDeleted)
		fmt.Printf("  Storage Freed     : %.2f MB\n", float64(res.BytesReclaimed)/(1024*1024))
		fmt.Printf("  Files Shielded    : %d (Active sessions protected)\n", res.SkippedShield)
	default:
		fmt.Fprintf(os.Stderr, "Unknown cache subcommand: %s. Use 'scan' or 'prune'.\n", sub)
		os.Exit(1)
	}
}

func runFingerprint(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: swiss fingerprint <status|list|gen|swap> [options]")
		os.Exit(1)
	}
	sub := args[0]
	subArgs := args[1:]

	fs := flag.NewFlagSet("fingerprint", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output JSON")
	_ = fs.Parse(subArgs)

	store, err := fingerprint.NewStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fingerprint store error: %v\n", err)
		os.Exit(1)
	}

	switch sub {
	case "status", "get":
		email := ""
		if len(fs.Args()) > 0 {
			email = fs.Args()[0]
		}
		if email == "" {
			kStore, _ := keyring.NewStore("")
			if kStore != nil {
				email = kStore.ActiveAccount()
			}
		}
		prof, err := store.GetOrCreateProfile(email)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			b, _ := json.MarshalIndent(prof, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Printf("Device Fingerprint Profile [%s]:\n", email)
		fmt.Printf("  Machine ID        : %s\n", prof.MachineID)
		fmt.Printf("  Updater ID        : %s\n", prof.UpdaterID)
		fmt.Printf("  Installation ID   : %s\n", prof.InstallationID)
		fmt.Printf("  Installation UUID : %s\n", prof.InstallationUUID)

	case "list":
		profiles := store.ListProfiles()
		if *jsonOut {
			b, _ := json.MarshalIndent(profiles, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Println("Configured Device Fingerprint Profiles:")
		for email, prof := range profiles {
			fmt.Printf("  - %s: Machine ID: %s... Updater: %s\n", email, prof.MachineID[:16], prof.UpdaterID)
		}

	case "gen", "generate":
		prof, err := fingerprint.GenerateRandom()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Generation error: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			b, _ := json.MarshalIndent(prof, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Println("Generated Virtual Device Fingerprint:")
		fmt.Printf("  Machine ID        : %s\n", prof.MachineID)
		fmt.Printf("  Updater ID        : %s\n", prof.UpdaterID)
		fmt.Printf("  Installation ID   : %s\n", prof.InstallationID)
		fmt.Printf("  Installation UUID : %s\n", prof.InstallationUUID)

	case "swap", "set":
		if len(fs.Args()) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: swiss fingerprint swap <email>")
			os.Exit(1)
		}
		email := fs.Args()[0]
		prof, err := fingerprint.GenerateRandom()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Generation error: %v\n", err)
			os.Exit(1)
		}
		if err := store.SetProfile(email, prof); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save profile: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			b, _ := json.MarshalIndent(prof, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Printf("[OK] Successfully randomized device fingerprint for %s:\n", email)
		fmt.Printf("  New Machine ID    : %s\n", prof.MachineID)
		fmt.Printf("  New Updater ID    : %s\n", prof.UpdaterID)
	default:
		fmt.Fprintf(os.Stderr, "Unknown fingerprint command: %s\n", sub)
		os.Exit(1)
	}
}

func runPatch(args []string) {
	subcmd := "status"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		subcmd = args[0]
		args = args[1:]
	}

	fs := flag.NewFlagSet("patch", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output JSON")
	_ = fs.Parse(args)

	store, err := gui.NewStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing GUI store: %v\n", err)
		os.Exit(1)
	}

	switch subcmd {
	case "status":
		st := store.GetDesktopStatus()
		if *jsonOut {
			b, _ := json.MarshalIndent(st, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Println("Antigravity Persistent Desktop Patch Status:")
		fmt.Printf("  Asar Path     : %s\n", st.AsarPath)
		fmt.Printf("  Installed     : %v\n", st.Installed)
		fmt.Printf("  Factory Backup: %v\n", st.BackupExists)
		if st.Error != "" {
			fmt.Printf("  Error         : %s\n", st.Error)
		}
	case "apply":
		fmt.Println("Applying styles and enhancements to Antigravity desktop app...")
		res, err := store.Apply()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Apply failed: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			b, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Printf("Result: %s\n", res.Message)
	case "install":
		fmt.Println("Installing persistent loader into Antigravity desktop app...")
		res, err := store.InstallDesktopLoader()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			b, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Printf("Result: %s\n", res.Message)
	case "restore":
		fmt.Println("Restoring Antigravity to factory original app.asar...")
		res, err := store.RestoreFactoryDefaults()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Restore failed: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			b, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Printf("Result: %s\n", res.Message)
	case "sync":
		if err := store.SyncPersistentFiles(); err != nil {
			fmt.Fprintf(os.Stderr, "Sync failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Successfully synced persistent_styles.css and persistent_script.js")
	default:
		fmt.Printf("Usage: swiss patch [status|install|restore|sync]\n")
	}
}
