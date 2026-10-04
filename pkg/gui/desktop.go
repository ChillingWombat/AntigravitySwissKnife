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
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
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

	// 2. Check if already installed
	data, err := os.ReadFile(asarPath)
	if err == nil && bytes.Contains(data, []byte(SwissLoaderMarker)) {
		// Already installed, but trigger live injection to make sure active windows have latest styles
		_ = dm.triggerLiveInjection()
		return &ApplyResult{
			Success: true,
			Message: "Permanent desktop loader is already installed and active.",
		}, nil
	}

	// 3. Extract asar to temporary directory
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

	// 4. Append Swiss loader script
	if !bytes.Contains(preloadContent, []byte(SwissLoaderMarker)) {
		loaderScript := generatePreloadLoaderScript()
		newContent := string(preloadContent) + "\n\n" + loaderScript
		if err := os.WriteFile(preloadPath, []byte(newContent), 0644); err != nil {
			return &ApplyResult{
				Success: false,
				Message: fmt.Sprintf("Failed to patch preload.js: %v", err),
			}, err
		}
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
	loaderScript := SwissLoaderMarker + `
(() => {
  try {
    const fs = require('fs');
    const path = require('path');
    const os = require('os');

    const configDir = (typeof process !== "undefined" && process.env && process.env.ANTIGRAVITY_SWISS_CONFIG_DIR) ||
      (typeof process !== "undefined" && process.platform === "win32" && process.env && process.env.APPDATA
        ? path.join(process.env.APPDATA, "antigravity-swiss")
        : path.join(os.homedir(), ".config", "antigravity-swiss"));
    const configPath = path.join(configDir, "gui_improvements.json");

    function readConfig() {
      try {
        if (fs.existsSync(configPath)) {
          return JSON.parse(fs.readFileSync(configPath, 'utf8'));
        }
      } catch (_) {}
      return null;
    }

    function applySwissStyles() {
      const cfg = readConfig();
      if (!cfg || !cfg.enabled) {
        const s = document.getElementById("antigravity-swiss-styles");
        if (s) s.remove();
        document.querySelectorAll("[data-swiss-project]").forEach(el => el.removeAttribute("data-swiss-project"));
        return;
      }

      let css = "";
      if (cfg.color_styling_enabled && cfg.project_colors) {
        const opacity = (cfg.tint_opacity && cfg.tint_opacity > 0) ? cfg.tint_opacity : 0.14;
        const hoverOpacity = Math.min(1.0, opacity + 0.08);
        const selectedOpacity = Math.min(1.0, opacity + 0.14);
        const parts = [];

        for (const [name, hex] of Object.entries(cfg.project_colors)) {
          if (!hex) continue;
          let r = 0, g = 0, b = 0;
          let h = hex.replace("#", "");
          if (h.length === 3) h = h[0]+h[0] + h[1]+h[1] + h[2]+h[2];
          if (h.length === 6) {
            r = parseInt(h.substring(0, 2), 16);
            g = parseInt(h.substring(2, 4), 16);
            b = parseInt(h.substring(4, 6), 16);
          }
          const safeName = name.replace(/"/g, '\\"');
          const borderStyle = cfg.solid_left_edge ? "border-left: 3px solid " + hex + " !important;" : "border-left: none !important;";

          parts.push(
            '[data-swiss-project="' + safeName + '"][data-project-card="true"] { background-color: ' + hex + ' !important; color: #ffffff !important; border-radius: 6px !important; }\n' +
            '[data-swiss-project="' + safeName + '"][data-testid="conversation-row-sidebar"] { background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + opacity + ') !important; ' + borderStyle + ' border-radius: 6px !important; margin-bottom: 2px !important; }\n' +
            '[data-swiss-project="' + safeName + '"][data-testid="conversation-row-sidebar"]:hover { background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + hoverOpacity + ') !important; }\n' +
            '[data-swiss-project="' + safeName + '"][data-testid="conversation-row-sidebar"][data-selected="true"] { background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + selectedOpacity + ') !important; font-weight: 600 !important; }\n' +
            '[data-swiss-project="' + safeName + '"][data-testid="conversation-row-sidebar"] div[style*="linear-gradient"] { background: linear-gradient(to right, transparent 0%, rgba(' + r + ', ' + g + ', ' + b + ', ' + opacity + ') 30%) !important; }\n' +
            '[data-swiss-project="' + safeName + '"][data-testid="conversation-row-sidebar"]:hover div[style*="linear-gradient"] { background: linear-gradient(to right, transparent 0%, rgba(' + r + ', ' + g + ', ' + b + ', ' + hoverOpacity + ') 30%) !important; }'
          );
        }
        css = parts.join("\n");
      }

      let styleEl = document.getElementById("antigravity-swiss-styles");
      if (css) {
        if (!styleEl) {
          styleEl = document.createElement("style");
          styleEl.id = "antigravity-swiss-styles";
          document.head.appendChild(styleEl);
        }
        styleEl.textContent = css;
      } else if (styleEl) {
        styleEl.remove();
      }

      try {
        const indexedSample = document.querySelector(".w-full.relative > [data-index]");
        if (!indexedSample) return;
        const container = indexedSample.parentElement;
        if (!container) return;

        const key = Object.keys(container).find(k => k.startsWith("__reactFiber"));
        if (!key) return;
        let fiber = container[key];
        let items = null;
        while (fiber) {
          if (fiber.memoizedProps?.items) {
            items = fiber.memoizedProps.items;
            break;
          }
          fiber = fiber.return;
        }
        if (!items) return;

        const headerMap = {};
        items.forEach(it => {
          if (it.type === "header") {
            const gid = it.id.replace("header-", "");
            headerMap[gid] = it.label;
          }
        });

        const isColorEnabled = cfg.color_styling_enabled;
        const isDragEnabled = cfg.drag_rearrange_enabled;
        const archivedProjects = cfg.archived_projects || [];

        let offsetAdjustment = 0;
        document.querySelectorAll("[data-index]").forEach(el => {
          const idx = parseInt(el.getAttribute("data-index"), 10);
          const item = items[idx];
          if (!item) return;

          const isArchived = (item.type === "header" && (archivedProjects.includes(item.label) || archivedProjects.includes(item.id.replace("header-", "")))) ||
                             (item.type === "row" && (archivedProjects.includes(headerMap[item.groupId]) || archivedProjects.includes(item.groupId)));

          if (isArchived) {
            el.style.display = "none";
            offsetAdjustment += (item.type === "header" ? 37 : 33);
            return;
          }

          el.style.display = "";
          const origTransform = el.style.transform;
          const m = origTransform ? origTransform.match(/translateY\((\d+(\.\d+)?)px\)/) : null;
          if (m) {
            if (!el.dataset.origTranslateY || Math.abs(parseFloat(el.dataset.origTranslateY) - parseFloat(m[1])) > (offsetAdjustment + 5)) {
              el.dataset.origTranslateY = m[1];
            }
            if (offsetAdjustment > 0) {
              const origY = parseFloat(el.dataset.origTranslateY);
              const newY = Math.max(0, origY - offsetAdjustment);
              el.style.transform = "translateY(" + newY + "px)";
            } else if (el.dataset.origTranslateY) {
              el.style.transform = "translateY(" + el.dataset.origTranslateY + "px)";
              delete el.dataset.origTranslateY;
            }
          }

          if (item.type === "header") {
            const btn = el.querySelector("[data-project-card]");
            if (btn) {
              if (isColorEnabled) btn.setAttribute("data-swiss-project", item.label);
              else btn.removeAttribute("data-swiss-project");
            }
            const headerGroup = el.querySelector(".group\\/header") || el;
            if (headerGroup && isColorEnabled) {
              headerGroup.setAttribute("data-swiss-project", item.label);
            }

            if (isDragEnabled) {
              headerGroup.setAttribute("draggable", "true");
              headerGroup.style.cursor = "grab";
              headerGroup.setAttribute("data-project-id", item.id.replace("header-", ""));
              headerGroup.setAttribute("data-project-label", item.label);

              if (!headerGroup.__swissDragBound) {
                headerGroup.__swissDragBound = true;
                headerGroup.addEventListener("dragstart", (e) => {
                  e.dataTransfer.setData("text/plain", headerGroup.getAttribute("data-project-id"));
                  headerGroup.style.opacity = "0.5";
                });
                headerGroup.addEventListener("dragend", () => {
                  headerGroup.style.opacity = "1";
                });
                headerGroup.addEventListener("dragover", (e) => {
                  e.preventDefault();
                  e.dataTransfer.dropEffect = "move";
                });
                headerGroup.addEventListener("drop", async (e) => {
                  e.preventDefault();
                  const srcId = e.dataTransfer.getData("text/plain");
                  const targetId = headerGroup.getAttribute("data-project-id");
                  if (srcId && targetId && srcId !== targetId) {
                    const curItems = await window.nativeStorage?.getItems();
                    let pOrder = [];
                    try { pOrder = JSON.parse(curItems?.projectsOrder || "[]"); } catch(_) {}
                    if (!pOrder.length) {
                      pOrder = items.filter(it => it.type === "header").map(h => h.id.replace("header-", ""));
                    }
                    const srcIdx = pOrder.indexOf(srcId);
                    const tgtIdx = pOrder.indexOf(targetId);
                    if (srcIdx !== -1 && tgtIdx !== -1) {
                      pOrder.splice(srcIdx, 1);
                      pOrder.splice(tgtIdx, 0, srcId);
                      await window.nativeStorage?.updateItems({
                        projectsOrder: JSON.stringify(pOrder),
                        projectsSortBy: "custom"
                      });
                    }
                  }
                });
              }
            } else {
              headerGroup.removeAttribute("draggable");
              headerGroup.style.cursor = "";
            }
          } else if (item.type === "row") {
            const pName = headerMap[item.groupId];
            if (pName) {
              const row = el.querySelector('[data-testid="conversation-row-sidebar"]');
              if (row) {
                if (isColorEnabled) row.setAttribute("data-swiss-project", pName);
                else row.removeAttribute("data-swiss-project");
              }
            }
          }
        });
      } catch (_) {}

      // Context menu
      const HUES = [0, 24, 42, 85, 145, 175, 205, 250, 285, 325];
        const LIGHTNESSES = [90, 82, 74, 65, 56, 48, 40, 32, 24, 16];
        const GRID_COLORS = [];
        for (let r = 0; r < 10; r++) {
          const l = LIGHTNESSES[r];
          for (let c = 0; c < 10; c++) {
            GRID_COLORS.push("hsl(" + HUES[c] + ", 82%, " + l + "%)");
          }
        }

        const PRESET_COLORS = [
          { name: "Blue", hex: "#0b57d0" },
          { name: "Purple", hex: "#7c3aed" },
          { name: "Emerald", hex: "#059669" },
          { name: "Amber", hex: "#d97706" },
          { name: "Coral", hex: "#dc2626" },
        ];

        window.__showSwissColorMenu = function(x, y, projectName) {
          let menu = document.getElementById("swiss-project-context-menu");
          if (menu) menu.remove();

          menu = document.createElement("div");
          menu.id = "swiss-project-context-menu";
          menu.style.cssText = "position: fixed; left: " + Math.min(x, window.innerWidth - 245) + "px; top: " + Math.min(y, window.innerHeight - 360) + "px; z-index: 999999; background: var(--card, #fbfbfb); border: 1px solid var(--border, rgba(0, 0, 0, 0.08)); border-radius: 10px; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.05); width: 232px; padding: 4px; font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; font-size: 13px; color: var(--foreground, #101010); user-select: none;";

          var html = '<div style="padding: 4px 6px 3px 6px; display: flex; justify-content: space-between; align-items: center; margin-bottom: 2px;">' +
            '<span style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 190px; font-size: 12px; font-weight: 500; color: var(--muted-foreground, #71717a); line-height: 1.4;">' + projectName + '</span>' +
            '<button id="swiss-close-menu" style="border: none; background: transparent; cursor: pointer; color: var(--muted-foreground, #9ca3af); font-size: 15px; line-height: 1; padding: 0 2px; border-radius: 4px; transition: color 0.1s;" onmouseenter="this.style.color=\'var(--foreground, #101010)\'" onmouseleave="this.style.color=\'var(--muted-foreground, #9ca3af)\'">&times;</button>' +
          '</div>' +

          '<div style="height: 1px; background: var(--border, rgba(0, 0, 0, 0.075)); margin: 2px 0 3px 0;"></div>' +

          '<div id="swiss-settings-action" class="swiss-menu-item" style="padding: 5px 8px; border-radius: 6px; cursor: pointer; display: flex; align-items: center; gap: 8px; margin-bottom: 1px; font-size: 13px; font-weight: 400; color: var(--secondary-foreground, #4a4a4a); white-space: nowrap; transition: background 0.1s, color 0.1s;">' +
            '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.8; flex-shrink: 0;"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>' +
            '<span>Project Settings</span>' +
          '</div>' +

          '<div id="swiss-new-convo-action" class="swiss-menu-item" style="padding: 5px 8px; border-radius: 6px; cursor: pointer; display: flex; align-items: center; gap: 8px; margin-bottom: 1px; font-size: 13px; font-weight: 400; color: var(--secondary-foreground, #4a4a4a); white-space: nowrap; transition: background 0.1s, color 0.1s;">' +
            '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.8; flex-shrink: 0;"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>' +
            '<span>New Conversation in Project</span>' +
          '</div>' +

          '<div id="swiss-archive-action" class="swiss-menu-item" style="padding: 5px 8px; border-radius: 6px; cursor: pointer; display: flex; align-items: center; gap: 8px; margin-bottom: 1px; font-size: 13px; font-weight: 400; color: #c2410c; white-space: nowrap; transition: background 0.1s, color 0.1s;">' +
            '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink: 0;"><polyline points="21 8 21 21 3 21 3 8"/><rect width="22" height="5" x="1" y="3" rx="1"/><line x1="10" x2="14" y1="12" y2="12"/></svg>' +
            '<span>Hide / Archive Project</span>' +
          '</div>' +

          '<div id="swiss-copy-action" class="swiss-menu-item" style="padding: 5px 8px; border-radius: 6px; cursor: pointer; display: flex; align-items: center; gap: 8px; margin-bottom: 2px; font-size: 13px; font-weight: 400; color: var(--secondary-foreground, #4a4a4a); white-space: nowrap; transition: background 0.1s, color 0.1s;">' +
            '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.8; flex-shrink: 0;"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>' +
            '<span>Copy Project Name</span>' +
          '</div>' +

          '<div style="height: 1px; background: var(--border, rgba(0, 0, 0, 0.075)); margin: 3px 0 2px 0;"></div>' +

          '<div style="display: flex; justify-content: space-between; align-items: center; padding: 2px 6px; margin-top: 2px; margin-bottom: 4px;">' +
            '<span style="font-size: 11px; font-weight: 500; color: var(--muted-foreground, #71717a); letter-spacing: 0.2px;">Preset Colors</span>' +
            '<button class="swiss-reset-color-btn" style="border: none; background: transparent; color: var(--muted-foreground, #a1a1aa); font-size: 10px; font-weight: 400; cursor: pointer; padding: 0; transition: color 0.1s;" onmouseenter="this.style.color=\'#ef4444\'" onmouseleave="this.style.color=\'var(--muted-foreground, #a1a1aa)\'">Reset</button>' +
          '</div>' +

          '<div style="display: flex; gap: 6px; padding: 1px 6px 4px 6px; align-items: center; justify-content: space-between;">' +
            PRESET_COLORS.map(function(p) {
              return '<div class="swiss-preset-swatch" data-color="' + p.hex + '" title="' + p.name + '" style="width: 15px; height: 15px; border-radius: 50%; background: ' + p.hex + '; cursor: pointer; transition: transform 0.12s, box-shadow 0.12s; border: 1.5px solid transparent; flex-shrink: 0;"></div>';
            }).join("") +
            '<div id="swiss-custom-trigger" title="Custom 10x10 Palette" style="width: 15px; height: 15px; border-radius: 50%; background: conic-gradient(red, yellow, lime, aqua, blue, magenta, red); cursor: pointer; transition: transform 0.12s, box-shadow 0.12s; border: 1.5px solid transparent; flex-shrink: 0;"></div>' +
          '</div>' +

          '<div id="swiss-custom-grid-container" style="display: none; margin-top: 4px; padding: 2px 4px 4px 4px;">' +
            '<div style="font-size: 10px; font-weight: 500; color: var(--muted-foreground, #71717a); margin-bottom: 4px; padding: 0 2px;">10&times;10 Palette</div>' +
            '<div id="swiss-color-grid" style="display: grid; grid-template-columns: repeat(10, 16px); gap: 3px; justify-content: center; background: var(--muted, rgba(0, 0, 0, 0.03)); padding: 6px; border-radius: 6px; border: 1px solid var(--border, rgba(0, 0, 0, 0.075));">' +
              GRID_COLORS.map(function(c) {
                return '<div class="swiss-grid-cell" data-color="' + c + '" style="width: 16px; height: 16px; border-radius: 50%; background: ' + c + '; cursor: pointer; transition: transform 0.12s;"></div>';
              }).join("") +
            '</div>' +
          '</div>';

          menu.innerHTML = html;
          document.body.appendChild(menu);

          document.getElementById("swiss-close-menu").onclick = () => menu.remove();

          const setItemHover = (el, isWarn) => {
            if (!el) return;
            el.onmouseenter = () => {
              el.style.background = "var(--secondary, rgba(0, 0, 0, 0.06))";
              el.style.color = isWarn ? "#b45309" : "var(--foreground, #101010)";
            };
            el.onmouseleave = () => {
              el.style.background = "transparent";
              el.style.color = isWarn ? "#c2410c" : "var(--secondary-foreground, #4a4a4a)";
            };
          };

          const settingsBtn = document.getElementById("swiss-settings-action");
          if (settingsBtn) {
            setItemHover(settingsBtn, false);
            settingsBtn.onclick = async () => {
              menu.remove();
              try {
                await fetch("http://127.0.0.1:8765/api/gui/projects/open_settings", {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ name: projectName })
                });
              } catch (_) {}
            };
          }

          const newConvoBtn = document.getElementById("swiss-new-convo-action");
          if (newConvoBtn) {
            setItemHover(newConvoBtn, false);
            newConvoBtn.onclick = () => {
              menu.remove();
              let gid = null;
              try {
                const indexedSample = document.querySelector(".w-full.relative > [data-index]");
                const container = indexedSample?.parentElement;
                const key = container ? Object.keys(container).find(k => k.startsWith("__reactFiber")) : null;
                let fiber = key ? container[key] : null;
                while (fiber) {
                  if (fiber.memoizedProps?.items) {
                    const h = fiber.memoizedProps.items.find(it => it.type === "header" && it.label === projectName);
                    if (h) gid = h.id.replace("header-", "");
                    break;
                  }
                  fiber = fiber.return;
                }
              } catch (_) {}
              if (gid) {
                window.location.href = "/?section=" + gid;
              }
            };
          }

          const archiveBtn = document.getElementById("swiss-archive-action");
          if (archiveBtn) {
            setItemHover(archiveBtn, true);
            archiveBtn.onclick = async () => {
              menu.remove();
              try {
                if (!archivedProjects.includes(projectName)) {
                  archivedProjects.push(projectName);
                }
                if (window.__swissArchivedProjects && !window.__swissArchivedProjects.includes(projectName)) {
                  window.__swissArchivedProjects.push(projectName);
                }
                requestAnimationFrame(updateTagsAndDraggables);
                await fetch("http://127.0.0.1:8765/api/gui/projects/archive", {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ name: projectName })
                });
                if (window.__swissToast) {
                  window.__swissToast("Project archived: " + projectName);
                }
              } catch (_) {}
            };
          }

          const copyBtn = document.getElementById("swiss-copy-action");
          setItemHover(copyBtn, false);
          copyBtn.onclick = () => {
            navigator.clipboard.writeText(projectName);
            copyBtn.innerHTML = "<span>&check; Copied!</span>";
            setTimeout(() => menu.remove(), 600);
          };

          const customTrig = document.getElementById("swiss-custom-trigger");
          if (customTrig) {
            customTrig.onmouseenter = () => {
              customTrig.style.transform = "scale(1.3)";
              customTrig.style.boxShadow = "0 1px 4px rgba(0,0,0,0.25)";
            };
            customTrig.onmouseleave = () => {
              customTrig.style.transform = "scale(1)";
              customTrig.style.boxShadow = "none";
            };
            customTrig.onclick = (e) => {
              e.stopPropagation();
              const g = document.getElementById("swiss-custom-grid-container");
              if (g) g.style.display = (g.style.display === "none") ? "block" : "none";
            };
          }

          const applyColor = async (colorVal) => {
            menu.remove();
            let hex = colorVal;
            if (colorVal.startsWith("hsl")) {
              const dummy = document.createElement("div");
              dummy.style.color = colorVal;
              document.body.appendChild(dummy);
              const rgb = window.getComputedStyle(dummy).color;
              dummy.remove();
              const m = rgb.match(/\d+/g);
              if (m) {
                hex = "#" + ((1 << 24) + (parseInt(m[0]) << 16) + (parseInt(m[1]) << 8) + parseInt(m[2])).toString(16).slice(1);
              }
            }

            try {
              await fetch("http://127.0.0.1:8765/api/gui/color", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ name: projectName, color: hex })
              });
            } catch (e) {
              console.warn("[SwissKnife] Save color failed:", e);
            }
          };

          menu.querySelectorAll(".swiss-preset-swatch, .swiss-grid-cell").forEach(el => {
            el.onmouseenter = () => {
              el.style.transform = "scale(1.3)";
              el.style.zIndex = "10";
              el.style.boxShadow = "0 1px 4px rgba(0,0,0,0.25)";
            };
            el.onmouseleave = () => {
              el.style.transform = "scale(1)";
              el.style.zIndex = "1";
              el.style.boxShadow = "none";
            };
            el.onclick = () => applyColor(el.getAttribute("data-color"));
          });

          menu.querySelectorAll(".swiss-reset-color-btn").forEach(btn => {
            btn.onclick = async () => {
              menu.remove();
              try {
                await fetch("http://127.0.0.1:8765/api/gui/projects/delete", {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ name: projectName })
                });
              } catch (e) {}
            };
          });

          const onDocClick = (e) => {
            if (!menu.contains(e.target)) {
              menu.remove();
              document.removeEventListener("pointerdown", onDocClick);
            }
          };
          setTimeout(() => document.addEventListener("pointerdown", onDocClick), 50);
        };

        if (!window.__swissToast) {
          window.__swissToast = function(msg) {
            let t = document.getElementById("swiss-toast-notification");
            if (t) t.remove();
            t = document.createElement("div");
            t.id = "swiss-toast-notification";
            t.style.cssText = "position: fixed; bottom: 24px; right: 24px; z-index: 9999999; background: var(--card, #ffffff); color: var(--foreground, #101010); border: 1px solid var(--border, rgba(0, 0, 0, 0.08)); padding: 8px 14px; border-radius: 8px; font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; font-size: 12px; font-weight: 500; box-shadow: 0 4px 12px rgba(0,0,0,0.08); pointer-events: none; transition: opacity 0.3s ease; opacity: 1;";
            t.textContent = msg;
            document.body.appendChild(t);
            setTimeout(() => {
              t.style.opacity = "0";
              setTimeout(() => t.remove(), 300);
            }, 2500);
          };
        }

        if (!window.__swissContextMenuBound) {
          window.__swissContextMenuBound = true;
          document.addEventListener("contextmenu", (e) => {
            const header = e.target.closest("[data-project-card], .group\\/header, [data-project-id], [data-project-label]");
            if (header) {
              e.preventDefault();
              e.stopPropagation();
              let pName = header.getAttribute("data-swiss-project") ||
                          header.getAttribute("data-project-label") ||
                          header.querySelector("[data-project-card]")?.getAttribute("data-swiss-project") ||
                          header.querySelector("[data-swiss-project]")?.getAttribute("data-swiss-project");
              if (!pName) {
                const card = header.querySelector("[data-project-card]") || header;
                pName = card.textContent.trim();
              }
              if (pName) {
                window.__showSwissColorMenu(e.clientX, e.clientY, pName);
              }
            }
          }, true);
        }

      if (!window.__swissObserverAttached) {
        window.__swissObserverAttached = true;
        const scrollContainer = document.querySelector(".w-full.relative > [data-index]")?.parentElement?.parentElement;
        if (scrollContainer) {
          scrollContainer.addEventListener("scroll", () => requestAnimationFrame(applySwissStyles), { passive: true });
        }
        const ob = new MutationObserver(() => requestAnimationFrame(applySwissStyles));
        ob.observe(document.body, { childList: true, subtree: true });
      }
    }

    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", applySwissStyles);
    } else {
      applySwissStyles();
    }
    window.addEventListener("focus", applySwissStyles);
  } catch (err) {
    console.warn("[SwissKnife] Loader exception:", err);
  }
})();
`
	var cmCfg *custommodels.Config
	if cmStore, err := custommodels.NewStore(""); err == nil {
		c := cmStore.GetConfig()
		cmCfg = &c
	}
	customScript := custommodels.GenerateCustomModelsScript(cmCfg)

	var enhCfg *enhancements.EnhancementsConfig
	if enhStore, err := enhancements.NewStore(""); err == nil {
		c := enhStore.GetConfig()
		enhCfg = &c
	}
	enhScript := enhancements.GenerateEnhancementsScript(enhCfg)

	return loaderScript + "\n\n" + customScript + "\n\n" + enhScript
}
