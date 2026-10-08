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
        // 0. Pre-inject disk-saved project colors so they take precedence over factory defaults across port resets
        let diskSavedColors = {};
        let diskDeletedColors = [];
        const configPath = path.join(configDir, "gui_improvements.json");
        if (fs.existsSync(configPath)) {
          try {
            const parsedCfg = JSON.parse(fs.readFileSync(configPath, "utf8"));
            if (parsedCfg && parsedCfg.project_colors && typeof parsedCfg.project_colors === "object") {
              diskSavedColors = parsedCfg.project_colors;
            }
            if (parsedCfg && Array.isArray(parsedCfg.deleted_colors)) {
              diskDeletedColors = parsedCfg.deleted_colors;
            }
          } catch (_) {}
        }
        const diskBootstrap = 'window.__swissDiskSavedColors = ' + JSON.stringify(diskSavedColors) + ';\n' +
          'window.__swissDiskDeletedColors = ' + JSON.stringify(diskDeletedColors) + ';\n';
        try {
          webFrame.executeJavaScript(diskBootstrap).catch(() => {});
        } catch (_) {}

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

        // 2. Inject persistent script into main world once per page lifecycle
        if (!window.__swissDesktopScriptInjected && fs.existsSync(jsPath)) {
          const js = fs.readFileSync(jsPath, 'utf8');
          if (js && js.trim()) {
            window.__swissDesktopScriptInjected = true;
            const fullScript = diskBootstrap + js;
            let executedInMain = false;
            try {
              if (webFrame && typeof webFrame.executeJavaScriptInIsolatedWorld === "function") {
                webFrame.executeJavaScriptInIsolatedWorld(0, [{ code: fullScript }]);
                executedInMain = true;
              }
            } catch (_) {}
            if (!executedInMain) {
              try {
                const s = document.createElement("script");
                s.textContent = fullScript;
                (document.head || document.documentElement).appendChild(s);
                s.remove();
                executedInMain = true;
              } catch (_) {}
            }
            if (!executedInMain) {
              try {
                webFrame.executeJavaScript(fullScript).catch(() => {});
              } catch (_) {}
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

    // On window focus, only refresh CSS stylesheets if modified, never re-executing JS
    window.addEventListener("focus", () => {
      try {
        if (fs.existsSync(cssPath)) {
          const css = fs.readFileSync(cssPath, 'utf8');
          if (css && css.trim()) {
            let styleEl = document.getElementById("antigravity-swiss-styles");
            if (styleEl && styleEl.textContent !== css) {
              styleEl.textContent = css;
            }
          }
        }
      } catch (_) {}
    });

    // Listen for color persistence messages from renderer to update gui_improvements.json & persistent_styles.css
    window.addEventListener("message", (event) => {
      if (event.data && event.data.type === "swiss-persist-project-colors") {
        try {
          const colors = event.data.colors || {};
          const deleted = event.data.deleted || [];
          const configPath = path.join(configDir, "gui_improvements.json");
          let cfgData = {};
          if (fs.existsSync(configPath)) {
            try { cfgData = JSON.parse(fs.readFileSync(configPath, "utf8")) || {}; } catch (_) {}
          }
          cfgData.project_colors = cfgData.project_colors || {};
          if (typeof colors === "object") {
            for (const [k, v] of Object.entries(colors)) {
              if (v) cfgData.project_colors[k] = v;
              else delete cfgData.project_colors[k];
            }
          }
          if (Array.isArray(deleted)) {
            cfgData.deleted_colors = deleted;
            for (const p of deleted) {
              delete cfgData.project_colors[p];
            }
          }
          fs.writeFileSync(configPath, JSON.stringify(cfgData, null, 2), "utf8");

          // Keep window.__swissDiskSavedColors and window.__swissDiskDeletedColors in sync
          try {
            const syncScript = 'window.__swissDiskSavedColors = ' + JSON.stringify(cfgData.project_colors) + ';' +
              'window.__swissDiskDeletedColors = ' + JSON.stringify(cfgData.deleted_colors || []) + ';';
            if (webFrame && typeof webFrame.executeJavaScriptInIsolatedWorld === "function") {
              webFrame.executeJavaScriptInIsolatedWorld(0, [{ code: syncScript }]);
            } else {
              webFrame.executeJavaScript(syncScript).catch(() => {});
            }
          } catch (_) {}

          // Keep persistent_styles.css and in-DOM style element in sync
          if (fs.existsSync(cssPath)) {
            let css = fs.readFileSync(cssPath, "utf8");
            let modified = false;
            const allTargets = new Set([...(Array.isArray(deleted) ? deleted : []), ...Object.keys(colors || {})]);
            for (const p of allTargets) {
              const safeP = p.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
              const patternWithComment = new RegExp('/\\*.*?\\*/\\s*\\[data-swiss-project="' + safeP + '".*?\\n\\}', 'gs');
              if (patternWithComment.test(css)) {
                css = css.replace(patternWithComment, '');
                modified = true;
              }
              const patternDirect = new RegExp('\\[data-swiss-project="' + safeP + '".*?\\n\\}', 'gs');
              if (patternDirect.test(css)) {
                css = css.replace(patternDirect, '');
                modified = true;
              }
            }
            if (typeof colors === "object") {
              for (const [pName, hex] of Object.entries(colors)) {
                if (!hex) continue;
                const safeP = pName.replace(/"/g, '\\"');
                let r = 11, g = 87, b = 208;
                if (hex.startsWith("#")) {
                  const rawHex = hex.slice(1);
                  if (rawHex.length === 3) {
                    r = parseInt(rawHex[0] + rawHex[0], 16) || 11;
                    g = parseInt(rawHex[1] + rawHex[1], 16) || 87;
                    b = parseInt(rawHex[2] + rawHex[2], 16) || 208;
                  } else if (rawHex.length >= 6) {
                    r = parseInt(rawHex.slice(0, 2), 16) || 11;
                    g = parseInt(rawHex.slice(2, 4), 16) || 87;
                    b = parseInt(rawHex.slice(4, 6), 16) || 208;
                  }
                }
                const lum = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255.0;
                const textColor = lum > 0.6 ? '#0f172a' : '#ffffff';
                const projRule = '\n/* Project: ' + pName + ' */\n' +
                  '[data-swiss-project="' + safeP + '"][data-project-card="true"],\n' +
                  '[data-swiss-project="' + safeP + '"] [data-project-card="true"],\n' +
                  '[data-swiss-project="' + safeP + '"][data-project-card],\n' +
                  '[data-swiss-project="' + safeP + '"] [data-project-card],\n' +
                  '[data-project-card][data-swiss-project="' + safeP + '"] {\n' +
                  '  background-color: ' + hex + ' !important;\n' +
                  '  color: ' + textColor + ' !important;\n' +
                  '  border-radius: 8px !important;\n' +
                  '  border: none !important;\n' +
                  '}\n' +
                  '[data-swiss-project="' + safeP + '"][data-project-card="true"] *,\n' +
                  '[data-swiss-project="' + safeP + '"] [data-project-card="true"] *,\n' +
                  '[data-swiss-project="' + safeP + '"][data-project-card] *,\n' +
                  '[data-swiss-project="' + safeP + '"] [data-project-card] *,\n' +
                  '[data-project-card][data-swiss-project="' + safeP + '"] * {\n' +
                  '  color: ' + textColor + ' !important;\n' +
                  '}\n' +
                  '[data-swiss-project="' + safeP + '"] [class*="group/header"] button,\n' +
                  '[data-swiss-project="' + safeP + '"] [class*="group/header"] a,\n' +
                  '[data-swiss-project="' + safeP + '"] a[aria-label*="conversation" i] svg,\n' +
                  '[data-swiss-project="' + safeP + '"] button[aria-label="Project options"] svg,\n' +
                  '[data-swiss-project="' + safeP + '"] button[aria-label*="conversation"] svg {\n' +
                  '  color: ' + textColor + ' !important;\n' +
                  '  fill: ' + textColor + ' !important;\n' +
                  '}\n' +
                  '/* Automate Tasks / Sidecar Workspace Overlay Icon Badge */\n' +
                  '[data-swiss-project="' + safeP + '"] [data-testid*="sidecar-workspace-overlay"],\n' +
                  '[data-swiss-project="' + safeP + '"] [data-project-card] [data-testid*="sidecar-workspace-overlay"],\n' +
                  '[data-swiss-project="' + safeP + '"] [class*="group/headerbtn"] [data-testid*="sidecar-workspace-overlay"],\n' +
                  '[data-swiss-project="' + safeP + '"] [data-project-card] span[class*="rounded-full"][class*="-bottom"],\n' +
                  '[data-project-card][data-swiss-project="' + safeP + '"] [data-testid*="sidecar-workspace-overlay"],\n' +
                  '[data-project-card][data-swiss-project="' + safeP + '"] span[class*="rounded-full"][class*="-bottom"] {\n' +
                  '  background-color: ' + hex + ' !important;\n' +
                  '  color: ' + textColor + ' !important;\n' +
                  '}\n' +
                  '[data-swiss-project="' + safeP + '"] [data-testid*="sidecar-workspace-overlay"]:hover,\n' +
                  '[data-swiss-project="' + safeP + '"] [data-project-card]:hover [data-testid*="sidecar-workspace-overlay"],\n' +
                  '[data-swiss-project="' + safeP + '"] [class*="group/headerbtn"]:hover [data-testid*="sidecar-workspace-overlay"] {\n' +
                  '  background-color: ' + hex + ' !important;\n' +
                  '}\n' +
                  '[data-swiss-project="' + safeP + '"] [data-testid*="sidecar-workspace-overlay"] :is(svg, svg path),\n' +
                  '[data-swiss-project="' + safeP + '"] [data-project-card] [data-testid*="sidecar-workspace-overlay"] :is(svg, svg path),\n' +
                  '[data-swiss-project="' + safeP + '"] [class*="group/headerbtn"] [data-testid*="sidecar-workspace-overlay"] :is(svg, svg path),\n' +
                  '[data-swiss-project="' + safeP + '"] [data-project-card] span[class*="rounded-full"][class*="-bottom"] :is(svg, svg path),\n' +
                  '[data-project-card][data-swiss-project="' + safeP + '"] [data-testid*="sidecar-workspace-overlay"] :is(svg, svg path),\n' +
                  '[data-project-card][data-swiss-project="' + safeP + '"] span[class*="rounded-full"][class*="-bottom"] :is(svg, svg path) {\n' +
                  '  color: ' + textColor + ' !important;\n' +
                  '  fill: ' + textColor + ' !important;\n' +
                  '}\n' +
                  '[data-swiss-project="' + safeP + '"][data-testid="conversation-row-sidebar"],\n' +
                  '[data-swiss-project="' + safeP + '"] [data-testid="conversation-row-sidebar"] {\n' +
                  '  background-color: rgba(' + r + ', ' + g + ', ' + b + ', 0.15) !important;\n' +
                  '  border-radius: 8px !important;\n' +
                  '}\n';
                css += projRule;
                modified = true;
              }
            }
            if (modified) {
              fs.writeFileSync(cssPath, css, "utf8");
              let styleEl = document.getElementById("antigravity-swiss-styles");
              if (styleEl) {
                styleEl.textContent = css;
              }
            }
          }
        } catch (e) {
          console.warn("[SwissKnife Preload] Failed to persist colors to disk:", e);
        }
      }
    });
  } catch (err) {
    console.warn("[SwissKnife Preload] Loader initialization failed:", err);
  }
})();
`
}
