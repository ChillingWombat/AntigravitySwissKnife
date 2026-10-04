package gui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

const SwissLoaderMarker = "// __SWISS_PRELOAD_LOADER__"

// DesktopStatus reports the installation and backup status of the persistent loader in Antigravity resources.
type DesktopStatus struct {
	Installed    bool   `json:"installed"`
	BackupExists bool   `json:"backup_exists"`
	ResourcesDir string `json:"resources_dir"`
	AsarPath     string `json:"asar_path"`
	Error        string `json:"error,omitempty"`
}

// DesktopManager handles permanent injection into Antigravity desktop resources and factory restoration.
type DesktopManager struct {
	resourcesDir string
	configPath   string
	injector     *Injector
}

// NewDesktopManager creates a DesktopManager instance.
func NewDesktopManager(resourcesDir, configPath string, injector *Injector) *DesktopManager {
	if resourcesDir == "" {
		resourcesDir = core.GetAntigravityDesktopResourcesDir()
	}
	if configPath == "" {
		configPath = filepath.Join(core.GetConfigDir(), "gui_improvements.json")
	}
	if injector == nil {
		injector = NewInjector(0)
	}
	return &DesktopManager{
		resourcesDir: resourcesDir,
		configPath:   configPath,
		injector:     injector,
	}
}

// GetStatus checks whether the loader is installed in app.asar and whether a factory backup exists.
func (dm *DesktopManager) GetStatus() DesktopStatus {
	asarPath := filepath.Join(dm.resourcesDir, "app.asar")
	backupPath := filepath.Join(dm.resourcesDir, "app.asar.factory_backup")

	status := DesktopStatus{
		ResourcesDir: dm.resourcesDir,
		AsarPath:     asarPath,
		BackupExists: fileExists(backupPath),
	}

	if !fileExists(asarPath) {
		status.Error = fmt.Sprintf("app.asar not found at %s", asarPath)
		return status
	}

	data, err := os.ReadFile(asarPath)
	if err != nil {
		status.Error = fmt.Sprintf("cannot read app.asar: %v", err)
		return status
	}

	status.Installed = bytes.Contains(data, []byte(SwissLoaderMarker))
	return status
}

// InstallDesktopLoader patches Antigravity's app.asar to permanently load GUI improvements across app restarts.
func (dm *DesktopManager) InstallDesktopLoader() (*ApplyResult, error) {
	asarPath := filepath.Join(dm.resourcesDir, "app.asar")
	backupPath := filepath.Join(dm.resourcesDir, "app.asar.factory_backup")

	if !fileExists(asarPath) {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Antigravity app.asar not found at %s", asarPath),
		}, fmt.Errorf("app.asar not found at %s", asarPath)
	}

	// 1. Ensure factory backup exists before any modification
	if !fileExists(backupPath) {
		if err := copyFile(asarPath, backupPath); err != nil {
			return &ApplyResult{
				Success: false,
				Message: fmt.Sprintf("Failed to create factory backup: %v", err),
			}, fmt.Errorf("failed to create factory backup: %w", err)
		}
	}

	// 2. Extract asar to temporary directory
	tmpDir, err := os.MkdirTemp("", "swiss_asar_install_*")
	if err != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create temp dir: %v", err),
		}, err
	}
	defer os.RemoveAll(tmpDir)

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := runAsarExtract(asarPath, extractDir); err != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to unpack app.asar: %v", err),
		}, err
	}

	preloadPath := filepath.Join(extractDir, "dist", "preload.js")
	if !fileExists(preloadPath) {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("dist/preload.js not found in extracted app"),
		}, fmt.Errorf("dist/preload.js not found in extracted app")
	}

	preloadContent, err := os.ReadFile(preloadPath)
	if err != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to read preload.js: %v", err),
		}, err
	}

	// 3. Patch or update Swiss loader script in preload.js
	loaderScript := generatePreloadLoaderScript()
	var newContent string
	if idx := bytes.Index(preloadContent, []byte(SwissLoaderMarker)); idx != -1 {
		newContent = string(preloadContent[:idx]) + loaderScript
	} else {
		newContent = string(preloadContent) + "\n\n" + loaderScript
	}
	if err := os.WriteFile(preloadPath, []byte(newContent), 0644); err != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to patch preload.js: %v", err),
		}, err
	}

	// 5. Pack back into asar
	tmpPackAsar := filepath.Join(tmpDir, "app.asar.new")
	if err := runAsarPack(extractDir, tmpPackAsar); err != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to pack modified app.asar: %v", err),
		}, err
	}

	// 6. Atomically replace app.asar
	if err := copyFile(tmpPackAsar, asarPath); err != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to write updated app.asar: %v", err),
		}, err
	}

	// 7. Trigger live injection for currently running windows
	_ = dm.triggerLiveInjection()

	return &ApplyResult{
		Success: true,
		Message: "Successfully installed permanent desktop loader into Antigravity desktop app.",
	}, nil
}

// RestoreFactoryDefaults restores the pristine original app.asar from factory backup and resets GUI settings.
func (dm *DesktopManager) RestoreFactoryDefaults(store *Store) (*ApplyResult, error) {
	asarPath := filepath.Join(dm.resourcesDir, "app.asar")
	backupPath := filepath.Join(dm.resourcesDir, "app.asar.factory_backup")

	if !fileExists(backupPath) {
		return &ApplyResult{
			Success: false,
			Message: "No factory backup found to restore from.",
		}, fmt.Errorf("factory backup not found at %s", backupPath)
	}

	// 1. Restore factory backup over app.asar
	if err := copyFile(backupPath, asarPath); err != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to restore factory backup: %v", err),
		}, fmt.Errorf("failed to restore factory backup: %w", err)
	}

	// 2. Clear injected styles from running windows via CDP
	cleanupScript := `(() => {
		const s = document.getElementById("antigravity-swiss-styles");
		if (s) s.remove();
		document.querySelectorAll("[data-swiss-project]").forEach(el => el.removeAttribute("data-swiss-project"));
		const m = document.getElementById("swiss-project-context-menu");
		if (m) m.remove();
		window.__swissContextMenuBound = false;
		return true;
	})()`

	if port, err := dm.injector.FindDevToolsPort(); err == nil {
		if pages, err := dm.injector.GetPageTargets(port); err == nil {
			for _, p := range pages {
				_, _ = dm.injector.ExecuteScript(p.WebSocketDebuggerURL, cleanupScript)
			}
		}
	}

	// 3. Reset stored configuration to factory default
	if store != nil {
		cfg := DefaultConfig()
		cfg.Enabled = false
		_ = store.UpdateConfig(cfg)
	}

	return &ApplyResult{
		Success: true,
		Message: "Successfully restored Antigravity 2.0 to original factory design.",
	}, nil
}

func (dm *DesktopManager) triggerLiveInjection() error {
	if dm.injector == nil {
		return nil
	}
	cfg := DefaultConfig()
	if data, err := os.ReadFile(dm.configPath); err == nil {
		_ = cfg // cfg loaded
		var c Config
		if err := jsonUnmarshal(data, &c); err == nil {
			cfg = &c
		}
	}
	_, err := dm.injector.ApplyConfig(cfg)
	return err
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Atomic write using temp file in same directory
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, "swiss_copy_*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// Preserve permissions
	if srcInfo, err := os.Stat(src); err == nil {
		_ = os.Chmod(tmpName, srcInfo.Mode())
	}

	return os.Rename(tmpName, dst)
}

func runAsarExtract(asarPath, destDir string) error {
	cmd := exec.Command("npx", "--yes", "@electron/asar", "extract", asarPath, destDir)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("asar extract failed: %v: %s", err, errBuf.String())
	}
	return nil
}

func runAsarPack(srcDir, destAsar string) error {
	cmd := exec.Command("npx", "--yes", "@electron/asar", "pack", srcDir, destAsar, "--unpack-dir", "node_modules/chrome-devtools-mcp")
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("asar pack failed: %v: %s", err, errBuf.String())
	}
	return nil
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func generatePreloadLoaderScript() string {
	return SwissLoaderMarker + `
(() => {
  try {
    const fs = require('fs');
    const path = require('path');
    const os = require('os');
    const { webFrame } = require('electron');

    const configDir = (typeof process !== "undefined" && process.env && process.env.ANTIGRAVITY_SWISS_CONFIG_DIR) ||
      (typeof process !== "undefined" && process.platform === "win32" && process.env && process.env.APPDATA
        ? path.join(process.env.APPDATA, "antigravity-swiss")
        : path.join(os.homedir(), ".config", "antigravity-swiss"));

    const cssPath = path.join(configDir, "persistent_styles.css");
    const jsPath = path.join(configDir, "persistent_script.js");

    function injectSwiss() {
      try {
        // 1. Inject persistent stylesheet into head & webFrame
        if (fs.existsSync(cssPath)) {
          const css = fs.readFileSync(cssPath, 'utf8');
          if (css && css.trim()) {
            let styleEl = document.getElementById("antigravity-swiss-styles");
            if (!styleEl) {
              styleEl = document.createElement("style");
              styleEl.id = "antigravity-swiss-styles";
              (document.head || document.documentElement).appendChild(styleEl);
            }
            if (styleEl.textContent !== css) {
              styleEl.textContent = css;
            }
            try { webFrame.insertCSS(css); } catch (_) {}
          }
        }

        // 2. Inject persistent script into main world
        if (fs.existsSync(jsPath)) {
          const js = fs.readFileSync(jsPath, 'utf8');
          if (js && js.trim()) {
            try {
              webFrame.executeJavaScript(js).catch(() => {});
            } catch (_) {
              const s = document.createElement("script");
              s.textContent = js;
              (document.head || document.documentElement).appendChild(s);
            }
          }
        }
      } catch (e) {
        console.warn("[SwissKnife Preload] Injection failed:", e);
      }
    }

    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", injectSwiss);
    } else {
      injectSwiss();
    }
    window.addEventListener("focus", injectSwiss);
  } catch (err) {
    console.warn("[SwissKnife Preload] Loader initialization failed:", err);
  }
})();
`
}
