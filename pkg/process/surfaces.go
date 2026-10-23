package process

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// Surface launchers for the Model Source settings.
//
// Profile Isolation runs an app on a second, isolated profile so it can point at
// the Swiss Knife model gateway without touching the native instance. The VS Code
// extension exposes antigravity.serverArgs, which is appended last and therefore
// wins for Go's flag parser — no patching. The desktop app always passes
// --app_data_dir antigravity (paths.js getAppDataDirName, derived from the package
// name), so a second window would share ~/.gemini/antigravity state; that one
// expression has to be patched before an isolated window may start.

const (
	// customProfileSuffix names the isolated profile directories.
	customProfileSuffix = "-swiss-custom"
)

// IsolatedProfileDir returns the isolated profile directory for an app.
func IsolatedProfileDir(app string) string {
	return filepath.Join(core.GetConfigDir(), "profiles", app+customProfileSuffix)
}

// gatewayTierEnv maps the enabled custom models onto the three model tiers the
// language server exposes in its picker. Without them the UI reports "No models
// available" even though the active model id resolves.
func gatewayTierEnv(gatewayModels []string, active string) []string {
	if len(gatewayModels) == 0 {
		return nil
	}
	pick := func(index int) string {
		if index < len(gatewayModels) {
			return gatewayModels[index]
		}
		return gatewayModels[0]
	}
	pro := strings.TrimSpace(active)
	if pro == "" {
		pro = gatewayModels[0]
	}
	return []string{
		"AGY_LLM_GATEWAY_TIER_FLASH=" + pick(0),
		"AGY_LLM_GATEWAY_TIER_FLASH_LITE=" + pick(1),
		"AGY_LLM_GATEWAY_TIER_PRO=" + pro,
	}
}

// LaunchAntigravityProfile starts the desktop app for the given model source.
// With source "custom" it runs on an isolated profile pointed at the gateway; it
// refuses to start unless that profile is genuinely isolated.
func LaunchAntigravityProfile(source, gatewayURL, gatewayKey string, gatewayModels []string, shield *Shield) (int, error) {
	bin := core.GetAntigravityBinaryPath()
	if bin == "" {
		return 0, fmt.Errorf("Antigravity desktop app not found")
	}
	args := []string{}
	env := os.Environ()

	if source == "custom" {
		// The app derives its Gemini state from os.homedir(), so scoping HOME (and
		// USERPROFILE on Windows) isolates it without touching the installation.
		home := IsolatedHomeDir("antigravity")
		if err := PrepareIsolatedHome(home); err != nil {
			return 0, fmt.Errorf("prepare isolated home: %w", err)
		}
		profile := IsolatedProfileDir("antigravity")
		if err := os.MkdirAll(profile, 0700); err != nil {
			return 0, fmt.Errorf("create isolated profile: %w", err)
		}
		env = homeEnvOverride(env, home)
		args = append(args, "--user-data-dir="+profile)
		if strings.TrimSpace(gatewayURL) != "" {
			env = append(env,
				"AGY_LLM_GATEWAY_URL="+strings.TrimRight(gatewayURL, "/"),
				"AGY_LLM_GATEWAY_WIRE_PROTOCOL=openai",
			)
			if key := strings.TrimSpace(gatewayKey); key != "" {
				env = append(env, "AGY_LLM_GATEWAY_API_KEY="+key)
			}
			// The app only enables gateway models from the explicit list; it does
			// not run discovery on its own.
			if len(gatewayModels) > 0 {
				env = append(env, "AGY_LLM_GATEWAY_MODELS="+strings.Join(gatewayModels, ","))
				env = append(env, gatewayTierEnv(gatewayModels, gatewayModels[0])...)
			}
			// The app executes turns through Cloud Code, not through the gateway
			// vars, so point its endpoint at the daemon: the Cloud Code proxy
			// translates a custom model id into its provider and streams the
			// answer back in the envelope the language server expects.
			env = append(env, "CLOUD_CODE_URL="+strings.TrimRight(gatewayURL, "/"))
		}
	}

	cmd := exec.Command(bin, args...)
	cmd.Env = env
	if err := core.LaunchDetachedGUIProcess(cmd); err != nil {
		return 0, fmt.Errorf("launch Antigravity: %w", err)
	}
	return cmd.Process.Pid, nil
}

// VSCodeSettingsPath returns the user settings file of VS Code on this platform.
func VSCodeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	candidates := vscodeSettingsCandidates(home)
	for _, candidate := range candidates {
		if _, statErr := os.Stat(filepath.Dir(candidate)); statErr == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("VS Code user settings directory not found")
}

// ApplyVSCodeModelSource writes antigravity.serverArgs for the requested source:
// custom models isolate the extension's language server and point it at the
// gateway, native clears the setting. Other settings are preserved.
func ApplyVSCodeModelSource(source, gatewayURL, settingsPath string) (string, error) {
	path := settingsPath
	if path == "" {
		resolved, err := VSCodeSettingsPath()
		if err != nil {
			return "", err
		}
		path = resolved
	}
	settings := map[string]interface{}{}
	if data, readErr := os.ReadFile(path); readErr == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return "", fmt.Errorf("parse %s: %w", path, err)
		}
	}

	if source == "custom" {
		// The catalog reaches the extension host through AGY_LLM_GATEWAY_* in the
		// editor process environment (set by LaunchVSCode). Settings only need to
		// keep this window's state out of ~/.gemini/antigravity; serverArgs are
		// appended last, so the flag wins.
		settings["antigravity.serverArgs"] = []string{"--app_data_dir=antigravity-swiss-custom"}
	} else {
		// Native: clear anything a previous run left behind.
		delete(settings, "antigravity.serverArgs")
	}

	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// LaunchVSCode starts VS Code (best effort) so the new serverArgs take effect.
func LaunchVSCode(source, gatewayURL, gatewayKey string, gatewayModels []string) error {
	for _, name := range vscodeBinaryNames() {
		binPath, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		cmd := exec.Command(binPath)
		if source == "custom" {
			// The running editor swallows a second launch (and its environment), so a
			// custom window needs its own user-data-dir to be a separate instance.
			profile := IsolatedProfileDir("vscode")
			if err := os.MkdirAll(profile, 0700); err != nil {
				return fmt.Errorf("create isolated VS Code profile: %w", err)
			}
			cmd.Args = append(cmd.Args, "--user-data-dir="+profile, "-n")
		}
		if source == "custom" && strings.TrimSpace(gatewayURL) != "" {
			// The extension host inherits this environment, so the gateway catalog
			// becomes selectable without any settings plumbing.
			cmd.Env = append(os.Environ(),
				"AGY_LLM_GATEWAY_URL="+strings.TrimRight(gatewayURL, "/"),
				"AGY_LLM_GATEWAY_WIRE_PROTOCOL=openai",
				"AGY_LLM_GATEWAY_API_KEY="+strings.TrimSpace(gatewayKey),
			)
			if len(gatewayModels) > 0 {
				cmd.Env = append(cmd.Env, "AGY_LLM_GATEWAY_MODELS="+strings.Join(gatewayModels, ","))
				cmd.Env = append(cmd.Env, gatewayTierEnv(gatewayModels, gatewayModels[0])...)
			}
		}
		if err := core.LaunchDetachedGUIProcess(cmd); err != nil {
			return fmt.Errorf("launch VS Code: %w", err)
		}
		return nil
	}
	return fmt.Errorf("the VS Code CLI (code) is not on PATH; open VS Code manually")
}

// VSCodeSettingsPathFor returns the settings file of a VS Code instance that runs on
// its own user-data-dir. An isolated instance does not read the default profile's
// settings, so per-mode flags must be written there.
func VSCodeSettingsPathFor(profileDir string) string {
	return filepath.Join(profileDir, "User", "settings.json")
}

// vscodeSettingsCandidates lists the user settings files of the VS Code family for
// this platform.
func vscodeSettingsCandidates(home string) []string {
	roots := []string{"Code", "Code - Insiders", "VSCodium"}
	base := ""
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support")
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
	default:
		base = filepath.Join(home, ".config")
	}
	candidates := make([]string, 0, len(roots))
	for _, root := range roots {
		candidates = append(candidates, filepath.Join(base, root, "User", "settings.json"))
	}
	return candidates
}

// vscodeBinaryNames lists the CLI launcher names for this platform.
func vscodeBinaryNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"code.cmd", "code"}
	}
	return []string{"code"}
}
