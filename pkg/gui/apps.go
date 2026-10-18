package gui

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
)

// Supported injection targets for per-app CDP discovery and enhancement injection.
const (
	AppAntigravity    = "antigravity"
	AppVSCode         = "vscode"
	AppVSCodeInsiders = "vscode-insiders"
)

// NormalizeAppID maps app identifiers and their aliases to a canonical id.
// The empty string and core.TargetAppDesktop ("desktop") both mean Antigravity,
// so existing desktop-only call sites keep working unchanged.
func NormalizeAppID(app string) string {
	switch strings.ToLower(strings.TrimSpace(app)) {
	case "", AppAntigravity, core.TargetAppDesktop:
		return AppAntigravity
	case AppVSCode:
		return AppVSCode
	case AppVSCodeInsiders, "vscode-insider", "code-insiders":
		return AppVSCodeInsiders
	default:
		return strings.ToLower(strings.TrimSpace(app))
	}
}

// appUserDataDir resolves the Electron user-data directory <root>/<name> for the
// given OS: %APPDATA% on Windows, ~/Library/Application Support on macOS and
// ~/.config on Linux. Returns "" when the required root is unknown.
func appUserDataDir(name, goos, appData, home string) string {
	switch goos {
	case "windows":
		if appData == "" {
			return ""
		}
		return filepath.Join(appData, name)
	case "darwin":
		if home == "" {
			return ""
		}
		return filepath.Join(home, "Library", "Application Support", name)
	default:
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".config", name)
	}
}

// devToolsUserDataDirs returns the ordered user-data directories of an app for a
// given OS. Antigravity keeps the legacy ~/.config/antigravity fallback appended
// after the per-OS host directory.
func devToolsUserDataDirs(app, goos, appData, home string) []string {
	switch app {
	case AppAntigravity:
		dirs := []string{appUserDataDir("Antigravity", goos, appData, home)}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, ".config", "antigravity"))
		}
		return dirs
	case AppVSCode:
		return []string{appUserDataDir("Code", goos, appData, home)}
	case AppVSCodeInsiders:
		return []string{appUserDataDir("Code - Insiders", goos, appData, home)}
	default:
		return nil
	}
}

// DevToolsActivePortFiles returns the ordered DevToolsActivePort file paths to
// probe for app on the given OS. Electron writes that file into the app's
// user-data directory when the process runs with --remote-debugging-port.
//
// The function is pure — the OS and the environment roots are parameters — so the
// discovery order is unit-testable from any host.
func DevToolsActivePortFiles(app, goos, appData, home string) []string {
	dirs := devToolsUserDataDirs(NormalizeAppID(app), goos, appData, home)
	files := make([]string, 0, len(dirs))
	seen := make(map[string]bool, len(dirs))
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		path := filepath.Join(dir, "DevToolsActivePort")
		if seen[path] {
			continue
		}
		seen[path] = true
		files = append(files, path)
	}
	return files
}

// appendUniquePaths appends paths not already present in existing.
func appendUniquePaths(existing, paths []string) []string {
	out := existing
	for _, p := range paths {
		dup := false
		for _, e := range out {
			if e == p {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, p)
		}
	}
	return out
}

// DevToolsActivePortCandidatesForApp resolves the port files for the current host,
// honouring the ANTIGRAVITY_HOST_CONFIG_DIR / VSCODE_HOST_CONFIG_DIR overrides that
// the rest of the daemon already uses.
func DevToolsActivePortCandidatesForApp(app string) []string {
	home, _ := os.UserHomeDir()
	app = NormalizeAppID(app)

	var overridden string
	switch app {
	case AppAntigravity:
		overridden = core.GetAntigravityHostConfigDir()
	case AppVSCode, AppVSCodeInsiders:
		overridden = core.GetVSCodeHostConfigDir()
	}

	var files []string
	if overridden != "" {
		files = append(files, filepath.Join(overridden, "DevToolsActivePort"))
	}
	return appendUniquePaths(files, DevToolsActivePortFiles(app, runtime.GOOS, os.Getenv("APPDATA"), home))
}

var (
	appPortCacheMu sync.Mutex
	appPortCache   = map[string]struct {
		port      int
		expiresAt time.Time
	}{}
)

func lookupAppPort(app string) int {
	appPortCacheMu.Lock()
	defer appPortCacheMu.Unlock()
	entry, ok := appPortCache[app]
	if !ok || time.Now().After(entry.expiresAt) {
		return 0
	}
	return entry.port
}

func cacheAppPort(app string, port int) int {
	appPortCacheMu.Lock()
	appPortCache[app] = struct {
		port      int
		expiresAt time.Time
	}{port: port, expiresAt: time.Now().Add(5 * time.Second)}
	appPortCacheMu.Unlock()
	return port
}

// FindDevToolsPortForApp resolves the live CDP port of the given app.
//
// Antigravity delegates to FindDevToolsPort, so the existing desktop discovery
// (custom port, host config dir, ANTIGRAVITY_CDP_PORT override, 9222-9230 scan)
// is untouched. VS Code only exposes CDP when started with
// --remote-debugging-port (Electron then writes DevToolsActivePort), so the
// discovery order is: DevToolsActivePort file, VSCODE_CDP_PORT override, the
// configured vscode_debug_port.
func (inj *Injector) FindDevToolsPortForApp(app string) (int, error) {
	app = NormalizeAppID(app)
	if app == AppAntigravity {
		return inj.FindDevToolsPort()
	}
	switch app {
	case AppVSCode, AppVSCodeInsiders:
	default:
		return 0, fmt.Errorf("unsupported app %q: expected %q, %q or %q", app, AppAntigravity, AppVSCode, AppVSCodeInsiders)
	}
	if inj.customPort > 0 {
		return inj.customPort, nil
	}
	if IsTestExecution() {
		return 0, fmt.Errorf("unit test mode: skipped connecting to live host DevTools port")
	}

	if cached := lookupAppPort(app); cached > 0 && inj.isPortLive(cached) {
		return cached, nil
	}

	for _, portFile := range DevToolsActivePortCandidatesForApp(app) {
		data, err := os.ReadFile(portFile)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 {
			continue
		}
		portStr := strings.TrimSpace(lines[0])
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && inj.isPortLive(p) {
			return cacheAppPort(app, p), nil
		}
	}

	if pStr := os.Getenv("VSCODE_CDP_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 && inj.isPortLive(p) {
			return cacheAppPort(app, p), nil
		}
	}

	if cfg, err := core.LoadConfig(); err == nil && cfg != nil {
		if p := cfg.GetVSCodeDebugPort(); p > 0 && inj.isPortLive(p) {
			return cacheAppPort(app, p), nil
		}
	}

	return 0, fmt.Errorf("VS Code DevTools port not detected (%s is not running with --remote-debugging-port)", app)
}

// IsAntigravityExtensionViewURL reports whether the URL belongs to the Antigravity
// VS Code extension's own content frame. The extension serves its UI (the panel
// with the Antigravity title and its data-testid DOM hooks) from a loopback HTTP
// server as .../?extensionView=true&extensionVariant=vs-code&hostVersion=..., e.g.
// http://127.0.0.1:58544/?extensionView=true&extensionVariant=vs-code&hostVersion=1.8.0
//
// The vscode-webview:// target of the activity-bar panel is only the (empty)
// webview shell; the extension UI we enhance lives in this nested frame.
func IsAntigravityExtensionViewURL(targetURL string) bool {
	if !strings.Contains(targetURL, "extensionView=true") {
		return false
	}
	return strings.HasPrefix(targetURL, "http://127.0.0.1:") ||
		strings.HasPrefix(targetURL, "https://127.0.0.1:") ||
		strings.HasPrefix(targetURL, "http://localhost:") ||
		strings.HasPrefix(targetURL, "https://localhost:")
}

// IsInjectableTarget reports whether a CDP target can carry the Swiss Knife UI
// enhancements for the given app.
//
// Antigravity keeps today's rules (page/webview/app targets with a websocket URL).
// VS Code accepts the workbench renderer (vscode-file:// URLs), extension webviews
// (vscode-webview:// URLs) and the Antigravity extension's own content frame
// (loopback extensionView=true); everything else — devtools:// pages, worker and
// node targets — is skipped. Unknown apps have no injectable targets.
//
// The predicate is pure (plain strings) so it is unit-testable without a live
// CDP endpoint.
func IsInjectableTarget(app, targetType, targetURL, wsURL string) bool {
	if strings.TrimSpace(wsURL) == "" {
		return false
	}
	switch NormalizeAppID(app) {
	case AppVSCode, AppVSCodeInsiders:
		switch targetType {
		case "page", "webview", "iframe":
		default:
			return false
		}
		return strings.HasPrefix(targetURL, "vscode-file://") ||
			strings.HasPrefix(targetURL, "vscode-webview://") ||
			IsAntigravityExtensionViewURL(targetURL)
	case AppAntigravity:
		switch targetType {
		case "page", "webview", "app":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// FilterInjectableTargets returns the subset of targets injectable for app.
func FilterInjectableTargets(app string, targets []DevToolsTarget) []DevToolsTarget {
	out := make([]DevToolsTarget, 0, len(targets))
	for _, t := range targets {
		if IsInjectableTarget(app, t.Type, t.URL, t.WebSocketDebuggerURL) {
			out = append(out, t)
		}
	}
	return out
}

// GetRawTargets fetches every CDP target of the given port without type filtering.
func (inj *Injector) GetRawTargets(port int) ([]DevToolsTarget, error) {
	targets, err := inj.fetchTargets(port)
	if err != nil {
		return nil, err
	}
	return targets, nil
}

// GetInjectableTargets fetches the CDP targets of the port and keeps the ones
// injectable for app.
func (inj *Injector) GetInjectableTargets(app string, port int) ([]DevToolsTarget, error) {
	targets, err := inj.fetchTargets(port)
	if err != nil {
		return nil, err
	}
	return FilterInjectableTargets(app, targets), nil
}

// GenerateVSCodeScript builds the script injected into VS Code renderer targets:
// the shared Swiss Knife stylesheet plus the enhancements script (Prompt Jump Bar,
// tool density, breaker line, telemetry, …).
//
// The DOM hooks the scripts look for mostly belong to Antigravity's own renderer;
// inside VS Code they guard themselves and degrade to a no-op when absent.
func GenerateVSCodeScript(cfg *Config) string {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	var enhCfg *enhancements.EnhancementsConfig
	if store, err := enhancements.NewStore(""); err == nil {
		c := store.GetConfig()
		enhCfg = &c
	}
	return generateStyleScript(cfg) + ";\n\n" + enhancements.GenerateEnhancementsScript(enhCfg) + ";"
}

// generateStyleScript manages the shared #antigravity-swiss-styles element,
// mirroring the stylesheet handling of the desktop script.
func generateStyleScript(cfg *Config) string {
	cssJSON, _ := json.Marshal(GenerateCSS(cfg))
	enabled := cfg.Enabled
	return fmt.Sprintf(`(() => {
  try {
    const enabled = %t;
    let styleEl = document.getElementById("antigravity-swiss-styles");
    if (!enabled) {
      if (styleEl) styleEl.remove();
      return true;
    }
    const css = %s;
    if (!css || !css.trim()) return false;
    if (!styleEl) {
      styleEl = document.createElement("style");
      styleEl.id = "antigravity-swiss-styles";
      (document.head || document.documentElement).appendChild(styleEl);
    }
    if (styleEl.textContent !== css) {
      styleEl.textContent = css;
    }
    return true;
  } catch (_) {
    return false;
  }
})()`, enabled, cssJSON)
}

// ApplyConfigForApp applies the Swiss Knife styling/enhancements to every injectable
// window of the given app.
//
// Antigravity reuses ApplyConfig unchanged. VS Code resolves its own CDP port,
// filters workbench/webview targets and evaluates the enhancements script in each.
// When the app is not running (or has no CDP port) this is a graceful no-op that
// only logs, because the app may simply not have been launched by Swiss Knife.
func (inj *Injector) ApplyConfigForApp(app string, cfg *Config) (*ApplyResult, error) {
	app = NormalizeAppID(app)
	if app == AppAntigravity {
		return inj.ApplyConfig(cfg)
	}

	port, err := inj.FindDevToolsPortForApp(app)
	if err != nil {
		log.Printf("[gui] %s enhancements skipped: %v", app, err)
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("%s enhancements skipped: %v", app, err),
		}, nil
	}

	targets, err := inj.GetInjectableTargets(app, port)
	if err != nil {
		log.Printf("[gui] %s CDP target listing failed on port %d: %v", app, port, err)
	}
	if len(targets) == 0 {
		return &ApplyResult{
			Success: false,
			Port:    port,
			Message: fmt.Sprintf("No active %s CDP targets found on port %d", app, port),
		}, nil
	}

	script := GenerateVSCodeScript(cfg)
	var lastErr error
	appliedCount := 0
	for _, target := range targets {
		if _, err := inj.ExecuteScript(target.WebSocketDebuggerURL, script); err != nil {
			lastErr = err
			continue
		}
		appliedCount++
	}

	if appliedCount == 0 && lastErr != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to inject Swiss Knife enhancements into %s: %v", app, lastErr),
			Port:    port,
		}, lastErr
	}

	return &ApplyResult{
		Success: true,
		Message: fmt.Sprintf("Successfully injected Swiss Knife enhancements into %d %s window(s)", appliedCount, app),
		Port:    port,
	}, nil
}
