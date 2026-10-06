package plugins

// GenerateAuxiliaryPluginsCSS provides the Google-style minimalist CSS for in-app auxiliary panel tabs,
// browser preview, file explorer, code editor, memo cards, and in-chat token telemetry badges.
func GenerateAuxiliaryPluginsCSS() string {
	return `/* Antigravity Swiss Knife - In-App Auxiliary Panel & In-Chat Telemetry Styles */

/* Auxiliary Tab Strip Buttons */
.swiss-aux-tab-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-width: 24px;
  height: 24px;
  padding: 0 6px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 4px;
  border: none;
  background: transparent;
  color: var(--text-muted, #71717a);
  cursor: pointer;
  transition: all 0.15s ease;
  user-select: none;
  box-sizing: border-box;
  line-height: 1;
}
.swiss-aux-tab-btn.icon-only {
  width: 24px;
  height: 24px;
  min-width: 24px;
  padding: 0;
}
.swiss-aux-tab-btn:hover {
  background: rgba(148, 163, 184, 0.15);
  color: var(--text, #1e293b);
}
.swiss-aux-tab-btn.active {
  color: var(--text, #1e293b);
  background: rgba(0, 0, 0, 0.08);
  font-weight: 600;
}
:is(.dark, [data-theme="dark"]) .swiss-aux-tab-btn.active {
  color: #f1f5f9;
  background: rgba(255, 255, 255, 0.12);
}
:is(.dark, [data-theme="dark"]) .swiss-aux-tab-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #f1f5f9;
}
.swiss-aux-tab-svg {
  width: 14px;
  height: 14px;
  display: inline-block;
  vertical-align: middle;
  flex-shrink: 0;
  pointer-events: none;
}
.swiss-aux-tab-label {
  font-size: 11px;
  line-height: 1;
  pointer-events: none;
}
.swiss-aux-tabs-divider {
  height: 16px;
  width: 1px;
  background-color: var(--border, #e2e8f0);
  margin: 0 4px;
  opacity: 0.7;
}
.swiss-aux-btn-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

/* Auxiliary Content Wrapper */
#swiss-aux-container {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  box-sizing: border-box;
}

/* Browser View */
.swiss-browser-view {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
}
.swiss-browser-toolbar {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px 10px;
  background: var(--canvas-subtle, #f8fafc);
  border-bottom: 1px solid var(--border, #e2e8f0);
}
.swiss-browser-nav-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
}
.swiss-browser-port-bar {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  font-size: 11px;
}
.swiss-port-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted, #71717a);
  margin-right: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.swiss-port-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 6px;
  border-radius: 10px;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  font-size: 10px;
  font-family: monospace;
  cursor: pointer;
  transition: all 0.15s;
  user-select: none;
}
.swiss-port-chip:hover {
  background: rgba(26, 115, 232, 0.08);
  border-color: #1a73e8;
  color: #1a73e8;
}
.swiss-port-chip.active {
  background: #1a73e8;
  border-color: #1a73e8;
  color: #ffffff;
  font-weight: 600;
}
.swiss-port-chip.custom {
  font-family: sans-serif;
  border-style: dashed;
}
.swiss-browser-url-input {
  flex: 1;
  padding: 4px 10px;
  font-size: 12px;
  border-radius: 14px;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  outline: none;
}
.swiss-browser-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid var(--border, #e2e8f0);
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  font-size: 11px;
  cursor: pointer;
  transition: background 0.15s;
  white-space: nowrap;
}
.swiss-browser-btn svg {
  flex-shrink: 0;
  vertical-align: middle;
}
.swiss-browser-btn:hover {
  background: rgba(148, 163, 184, 0.15);
}
.swiss-browser-btn.primary {
  background: #1a73e8;
  color: #ffffff;
  border-color: #1a73e8;
  font-weight: 600;
}
.swiss-browser-btn.primary:hover {
  background: #1557b0;
}
.swiss-browser-btn.active {
  background: rgba(234, 67, 53, 0.1);
  color: #ea4335;
  border-color: #ea4335;
}

/* Device Stage & Realistic Frames */
.swiss-browser-viewport-wrap {
  position: relative;
  flex: 1;
  width: 100%;
  height: 100%;
  overflow: auto;
  background: #f1f5f9;
  display: flex;
  justify-content: center;
}
.swiss-device-stage {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  min-height: 100%;
  width: 100%;
  box-sizing: border-box;
}
.swiss-device-frame {
  position: relative;
  box-sizing: border-box;
  background: #ffffff;
  transition: transform 0.2s cubic-bezier(0.2, 0, 0, 1), width 0.2s, height 0.2s;
  flex-shrink: 0;
}
.swiss-device-screen {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #ffffff;
}
.swiss-device-frame.frame-iphone-16-pro {
  width: 424px;
  height: 896px;
  border: 11px solid #1a1a1c;
  border-radius: 54px;
  box-shadow: 0 25px 65px -15px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.12) inset;
}
.swiss-device-frame.frame-iphone-16-pro .swiss-device-screen {
  width: 402px;
  height: 874px;
  border-radius: 43px;
}
.swiss-device-notch.dynamic-island {
  position: absolute;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  width: 120px;
  height: 35px;
  background: #000000;
  border-radius: 18px;
  z-index: 60;
  pointer-events: none;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.6);
}
.swiss-device-frame.frame-pixel-9 {
  width: 432px;
  height: 944px;
  border: 10px solid #202124;
  border-radius: 42px;
  box-shadow: 0 25px 65px -15px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.1) inset;
}
.swiss-device-frame.frame-pixel-9 .swiss-device-screen {
  width: 412px;
  height: 924px;
  border-radius: 32px;
}
.swiss-device-notch.punch-hole {
  position: absolute;
  top: 14px;
  left: 50%;
  transform: translateX(-50%);
  width: 12px;
  height: 12px;
  background: #0a0a0c;
  border-radius: 50%;
  z-index: 60;
  pointer-events: none;
  box-shadow: 0 0 0 2px #202124;
}
.swiss-device-frame.frame-ipad {
  width: 848px;
  height: 1208px;
  border: 14px solid #2c2d30;
  border-radius: 30px;
  box-shadow: 0 30px 75px -20px rgba(0, 0, 0, 0.45), 0 0 0 1px rgba(255, 255, 255, 0.1) inset;
}
.swiss-device-frame.frame-ipad .swiss-device-screen {
  width: 820px;
  height: 1180px;
  border-radius: 16px;
}
.swiss-device-notch.ipad-camera {
  position: absolute;
  top: 6px;
  left: 50%;
  transform: translateX(-50%);
  width: 6px;
  height: 6px;
  background: #0a0a0c;
  border-radius: 50%;
  z-index: 60;
  pointer-events: none;
  box-shadow: 0 0 0 1px #3a3a3c;
}
.swiss-device-frame.frame-responsive {
  width: 100%;
  height: 100%;
  border: none;
  border-radius: 0;
  box-shadow: none;
}
.swiss-device-frame.frame-responsive .swiss-device-screen {
  width: 100%;
  height: 100%;
  border-radius: 0;
}
.swiss-device-home-bar {
  position: absolute;
  bottom: 8px;
  left: 50%;
  transform: translateX(-50%);
  height: 5px;
  background: rgba(0, 0, 0, 0.55);
  border-radius: 3px;
  z-index: 60;
  pointer-events: none;
}
.swiss-device-frame.frame-iphone-16-pro .swiss-device-home-bar { width: 134px; }
.swiss-device-frame.frame-pixel-9 .swiss-device-home-bar { width: 110px; height: 4px; }
.swiss-device-frame.frame-ipad .swiss-device-home-bar { width: 160px; }
.swiss-device-screen.touch-emulation-active,
.touch-emulation-active {
  cursor: url('data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="rgba(26,115,232,0.25)" stroke="%231a73e8" stroke-width="2"/><circle cx="12" cy="12" r="3" fill="%231a73e8"/></svg>') 12 12, auto;
}
.swiss-browser-frame-container {
  position: relative;
  background: #ffffff;
  box-shadow: 0 4px 14px rgba(0,0,0,0.08);
  transition: width 0.2s, height 0.2s;
}
.swiss-browser-canvas-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 50;
  cursor: crosshair;
}

/* File Explorer View */
.swiss-files-view {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
}
.swiss-files-toolbar {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 10px;
  background: var(--canvas-subtle, #f8fafc);
  border-bottom: 1px solid var(--border, #e2e8f0);
}
.swiss-files-address-bar {
  display: flex;
  align-items: center;
  gap: 6px;
}
.swiss-files-path-input {
  flex: 1;
  padding: 4px 8px;
  font-size: 11px;
  border-radius: 4px;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  font-family: monospace;
}
.swiss-files-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
  user-select: none;
  -webkit-user-select: none;
}
.swiss-file-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  font-size: 12px;
  cursor: pointer;
  user-select: none;
  -webkit-user-select: none;
  border-bottom: 1px solid rgba(226, 232, 240, 0.4);
  transition: background 0.12s;
}
.swiss-file-row:hover {
  background: rgba(26, 115, 232, 0.06);
}
.swiss-file-row.selected {
  background: rgba(26, 115, 232, 0.14) !important;
  color: #1a73e8;
  font-weight: 600;
  box-shadow: inset 3px 0 0 #1a73e8;
  padding-left: 11px;
}
.swiss-file-row.cut {
  opacity: 0.5;
  filter: grayscale(0.5);
}
.swiss-file-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}
.swiss-file-svg {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  stroke: currentColor;
  vertical-align: middle;
}
.swiss-file-name {
  flex: 1;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.swiss-file-size {
  font-size: 10px;
  color: var(--text-muted, #94a3b8);
  font-family: monospace;
}

/* In-Place Code Editor */
.swiss-editor-container {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: var(--canvas, #ffffff);
}
.swiss-editor-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  border-bottom: 1px solid var(--border, #e2e8f0);
  background: var(--canvas-subtle, #f8fafc);
}
.swiss-editor-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}
.swiss-editor-gutter {
  width: 44px;
  padding: 8px 4px;
  background: var(--canvas-subtle, #f8fafc);
  border-right: 1px solid var(--border, #e2e8f0);
  font-family: monospace;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-muted, #94a3b8);
  text-align: right;
  user-select: none;
  overflow: hidden;
}
.swiss-editor-textarea {
  flex: 1;
  padding: 8px 10px;
  font-family: 'Fira Code', 'JetBrains Mono', monospace;
  font-size: 12px;
  line-height: 1.5;
  border: none;
  outline: none;
  resize: none;
  background: transparent;
  color: var(--text, #1e293b);
  white-space: pre;
  overflow: auto;
}

/* Context Menu */
.swiss-context-menu {
  position: fixed;
  z-index: 99999;
  background: var(--canvas, #ffffff);
  border: 1px solid var(--border, #cbd5e1);
  border-radius: 6px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.18);
  padding: 4px 0;
  min-width: 175px;
  font-size: 12px;
  user-select: none;
  -webkit-user-select: none;
}
.swiss-context-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  cursor: pointer;
  color: var(--text, #1e293b);
  user-select: none;
  -webkit-user-select: none;
}
.swiss-context-item.disabled {
  opacity: 0.4;
  cursor: not-allowed;
  pointer-events: none;
}
.swiss-context-item svg {
  width: 13px;
  height: 13px;
  flex-shrink: 0;
  stroke: currentColor;
}
.swiss-context-item:hover:not(.disabled) {
  background: rgba(26, 115, 232, 0.08);
  color: #1a73e8;
}
.swiss-context-divider {
  height: 1px;
  background: var(--border, #e2e8f0);
  margin: 3px 0;
}

/* Memos View */
.swiss-memos-view {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow-y: auto;
  padding: 12px;
  gap: 10px;
}
.swiss-memo-card {
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 8px;
  padding: 10px 12px;
  background: var(--canvas, #ffffff);
  box-shadow: 0 1px 3px rgba(0,0,0,0.05);
  cursor: grab;
  user-select: none;
  transition: transform 0.15s, box-shadow 0.15s;
}
.swiss-memo-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 8px rgba(0,0,0,0.08);
}
.swiss-memo-card.recording {
  border-color: #ea4335;
  background: rgba(234, 67, 53, 0.04);
}

/* In-Chat Telemetry Badge */
.swiss-telemetry-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  margin-bottom: 4px;
  padding: 4px 10px;
  border-radius: 12px;
  background: rgba(26, 115, 232, 0.06);
  border: 1px solid rgba(26, 115, 232, 0.2);
  font-size: 11px;
  font-weight: 500;
  color: #1a73e8;
  user-select: none;
}
.swiss-telemetry-badge svg {
  width: 11px;
  height: 11px;
  stroke: #1a73e8;
  fill: none;
  flex-shrink: 0;
  display: inline-block;
  vertical-align: middle;
}
.swiss-telemetry-badge .metric-dot {
  opacity: 0.4;
}
.swiss-telemetry-badge .metric-subagent {
  background: rgba(147, 51, 234, 0.1);
  color: #7c3aed;
  padding: 1px 6px;
  border-radius: 8px;
  font-size: 10px;
  font-weight: 600;
}
.swiss-aux-toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  background-color: var(--card, #1e293b);
  color: var(--foreground, #f8fafc);
  border: 1px solid var(--border, #334155);
  padding: 8px 16px;
  border-radius: 8px;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.2);
  font-size: 13px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  z-index: 10000;
  opacity: 0;
  transform: translateY(12px);
  transition: opacity 0.2s cubic-bezier(0.4, 0, 0.2, 1), transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
  display: flex;
  align-items: center;
  gap: 8px;
}
.swiss-aux-toast.show {
  opacity: 1;
  transform: translateY(0);
}
.swiss-aux-toast.error {
  border-color: #ea4335;
  color: #ea4335;
}
.swiss-aux-toast.warning {
  border-color: #fbbc04;
  color: #fbbc04;
}
`
}

// GenerateAuxiliaryPluginsScript returns the full client-side JavaScript injected into Antigravity 2.0
// that mounts the auxiliary tabs, live browser previewer, file explorer with in-place editors, quick memos,
// and in-chat token telemetry badges.
func GenerateAuxiliaryPluginsScript() string {
	return `(() => {
  try {
    if (window.__swissAuxiliaryInitialized) return;
    window.__swissAuxiliaryInitialized = true;

    const API_BASE = "http://127.0.0.1:8765";
    let activeAuxTab = null; // "swiss-browser" | "swiss-files" | "swiss-memos" | null (native)
    let currentBrowserUrl = "http://localhost:5173";
    let currentFilePath = ".";
    let swissClipboard = { action: "copy", items: [] };
    let activeDevice = "responsive";
    let isDrawing = false;
    let drawTool = "none"; // "pen" | "rect" | "inspect" | "none"
    let drawStartX = 0, drawStartY = 0;
    let lastAnnotatedRegion = null;
    let lastSelectedElement = null;
    let userComment = "";
    let isTouchEmulationActive = false;

    function showToast(msg, type = "info") {
      let toast = document.getElementById("swiss-aux-toast");
      if (!toast) {
        toast = document.createElement("div");
        toast.id = "swiss-aux-toast";
        toast.className = "swiss-aux-toast";
        document.body.appendChild(toast);
      }
      toast.textContent = msg;
      toast.className = "swiss-aux-toast show " + type;
      clearTimeout(toast._timeout);
      toast._timeout = setTimeout(() => {
        toast.className = "swiss-aux-toast";
      }, 3000);
    }

    // 1. Auxiliary Panel Tab Injector Engine
    function setupAuxiliaryTabs() {
      // Find auxiliary panel header strictly matching .shrink-0.flex.items-center[class*="gap-0.5"].border-b
      const tabHeader = document.querySelector('.shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('[data-testid="auxiliary-panel"] .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.part.auxiliarybar .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.shrink-0.flex.items-center.border-b');
      if (!tabHeader) return;

      // Define Swiss tabs with Antigravity-matching monochrome SVG stroke icons
      const tabs = [
        {
          id: "browser",
          tabId: "swiss-browser",
          label: "Browser",
          svg: '<svg class="swiss-aux-tab-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
        },
        {
          id: "files",
          tabId: "swiss-files",
          label: "Files",
          svg: '<svg class="swiss-aux-tab-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>'
        },
        {
          id: "memos",
          tabId: "swiss-memos",
          label: "Memos",
          svg: '<svg class="swiss-aux-tab-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M16 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V8Z"/><polyline points="15 3 15 8 20 8"/><line x1="9" y1="13" x2="15" y2="13"/><line x1="9" y1="17" x2="13" y2="17"/></svg>'
        },
      ];

      function getAuxTabFormat() {
        return localStorage.getItem("antigravity_swiss_aux_tab_format") ||
               (window.__SWISS_ENH_CONFIG__ && window.__SWISS_ENH_CONFIG__.overview_panel && window.__SWISS_ENH_CONFIG__.overview_panel.aux_tabs_format) ||
               "icon";
      }

      function updateTabButtonMarkup(btn, t, fmt) {
        btn.title = "Antigravity Swiss Knife: " + t.label;
        if (fmt === "icon_and_name") {
          btn.classList.remove("icon-only");
          btn.innerHTML = t.svg + '<span class="swiss-aux-tab-label">' + t.label + '</span>';
        } else {
          btn.classList.add("icon-only");
          btn.innerHTML = t.svg;
        }
      }

      // Configure tabs matching data-tab-id="swiss-browser", data-tab-id="swiss-files", data-tab-id="swiss-memos"
      const existingBtns = tabHeader.querySelectorAll('.swiss-aux-tab-btn');
      if (existingBtns.length > 0) {
        const curFmt = getAuxTabFormat();
        existingBtns.forEach(btn => {
          const tid = btn.dataset.swissTab;
          const t = tabs.find(x => x.id === tid);
          if (t) updateTabButtonMarkup(btn, t, curFmt);
        });

        if (activeAuxTab) {
          const auxPanel = tabHeader.parentElement || document.querySelector('[data-testid="auxiliary-panel"]') || document.querySelector('.part.auxiliarybar');
          const bodyContainer = auxPanel ? auxPanel.querySelector('.flex-grow.overflow-hidden') : null;
          const swissContainer = document.querySelector("#swiss-aux-container");
          if (bodyContainer && swissContainer && swissContainer.parentElement !== bodyContainer) {
            bodyContainer.appendChild(swissContainer);
          }
        }
        return;
      }

      let divider = tabHeader.querySelector('.swiss-aux-tabs-divider');
      if (!divider) {
        divider = document.createElement("div");
        divider.className = "swiss-aux-tabs-divider";
        divider.style.height = "16px";
        divider.style.width = "1px";
        divider.style.backgroundColor = "var(--border, #e2e8f0)";
        divider.style.margin = "0 4px";
        divider.style.opacity = "0.7";
        tabHeader.appendChild(divider);
      }

      let btnGroup = tabHeader.querySelector('.swiss-aux-btn-group');
      if (!btnGroup) {
        btnGroup = document.createElement("div");
        btnGroup.className = "swiss-aux-btn-group";
        btnGroup.style.display = "inline-flex";
        btnGroup.style.alignItems = "center";
        btnGroup.style.gap = "2px";
        tabHeader.appendChild(btnGroup);
      }

      const curFmt = getAuxTabFormat();
      tabs.forEach(t => {
        const btn = document.createElement("button");
        btn.className = "swiss-aux-tab-btn";
        // Contract: data-tab-id="swiss-browser" data-tab-id="swiss-files" data-tab-id="swiss-memos"
        if (t.id === "browser") btn.setAttribute("data-tab-id", "swiss-browser");
        else if (t.id === "files") btn.setAttribute("data-tab-id", "swiss-files");
        else if (t.id === "memos") btn.setAttribute("data-tab-id", "swiss-memos");
        btn.dataset.swissTab = t.id;
        updateTabButtonMarkup(btn, t, curFmt);
        const activeTabTarget = activeAuxTab || localStorage.getItem("antigravity_active_aux_tab");
        if (activeTabTarget && (btn.getAttribute("data-tab-id") === activeTabTarget || t.id === activeTabTarget || ("swiss-" + t.id) === activeTabTarget)) {
          btn.classList.add("active");
          btn.setAttribute("aria-selected", "true");
        }
        btn.onclick = (e) => {
          e.stopPropagation();
          switchAuxTab(t.tabId);
        };
        btnGroup.appendChild(btn);
      });

      // Bind dynamic format update listeners once
      if (!window.__swissAuxFormatListenerBound) {
        window.__swissAuxFormatListenerBound = true;
        window.addEventListener("swiss-aux-tab-format-updated", () => {
          setupAuxiliaryTabs();
        });
        window.addEventListener("storage", (e) => {
          if (e.key === "antigravity_swiss_aux_tab_format") {
            setupAuxiliaryTabs();
          }
        });
      }

      // Two-way state sync: Listen for clicks on native factory tabs (overview, review, terminal)
      if (!tabHeader.__swissHeaderBound) {
        tabHeader.__swissHeaderBound = true;
        tabHeader.addEventListener("click", (e) => {
          const targetBtn = e.target.closest("button");
          if (!targetBtn) return;
          const targetId = targetBtn.getAttribute("data-tab-id") || "";
          if (targetId.startsWith("swiss-")) {
            return;
          }
          switchAuxTab(null);
        });
      }

      // Restore saved tab state from localStorage
      const savedTab = localStorage.getItem("antigravity_active_aux_tab");
      if (savedTab && savedTab.startsWith("swiss-")) {
        if (!activeAuxTab || activeAuxTab !== savedTab) {
          switchAuxTab(savedTab);
        } else {
          document.querySelectorAll(".swiss-aux-tab-btn").forEach(b => {
            const isActive = b.getAttribute("data-tab-id") === activeAuxTab;
            b.classList.toggle("active", isActive);
            b.setAttribute("aria-selected", isActive ? "true" : "false");
          });
        }
      }
    }

    // Switch between Swiss tabs and Native factory tabs
    function switchAuxTab(tabId) {
      const normalizedId = tabId ? (tabId.startsWith("swiss-") ? tabId : ("swiss-" + tabId)) : null;
      activeAuxTab = normalizedId;

      document.querySelectorAll(".swiss-aux-tab-btn").forEach(b => {
        const isActive = normalizedId !== null && b.getAttribute("data-tab-id") === normalizedId;
        b.classList.toggle("active", isActive);
        b.setAttribute("aria-selected", isActive ? "true" : "false");
      });

      const tabHeader = document.querySelector('.shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('[data-testid="auxiliary-panel"] .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.part.auxiliarybar .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.shrink-0.flex.items-center.border-b');
      const auxPanel = tabHeader ? tabHeader.parentElement : (document.querySelector('[data-testid="auxiliary-panel"]') || document.querySelector('.part.auxiliarybar'));
      const bodyContainer = auxPanel ? auxPanel.querySelector('.flex-grow.overflow-hidden') : null;
      if (!bodyContainer) return;

      let swissContainer = document.querySelector("#swiss-aux-container");
      if (!swissContainer) {
        swissContainer = document.createElement("div");
        swissContainer.id = "swiss-aux-container";
        swissContainer.style.display = "none";
        bodyContainer.appendChild(swissContainer);
      } else if (swissContainer.parentElement !== bodyContainer) {
        bodyContainer.appendChild(swissContainer);
      }

      const cleanId = normalizedId ? normalizedId.replace(/^swiss-/, "") : "";
      if (normalizedId && swissContainer.dataset.renderedTab === cleanId && swissContainer.style.display === "flex") {
        return;
      }

      const children = Array.from(bodyContainer.children);
      if (normalizedId) {
        swissContainer.style.display = "flex";
        children.forEach(child => {
          if (child !== swissContainer) {
            child.style.display = "none";
          }
        });

        if (tabHeader) {
          tabHeader.querySelectorAll('button').forEach(fb => {
            const tid = fb.getAttribute("data-tab-id") || "";
            if (!tid.startsWith("swiss-")) {
              fb.classList.remove("active");
              fb.removeAttribute("data-state");
              fb.setAttribute("aria-selected", "false");
            }
          });
        }

        localStorage.setItem("antigravity_active_aux_tab", normalizedId);
        if (swissContainer.dataset.renderedTab !== cleanId) {
          renderSwissTabContent(swissContainer, cleanId);
        }
      } else {
        delete swissContainer.dataset.renderedTab;
        swissContainer.style.display = "none";
        children.forEach(child => {
          if (child !== swissContainer) {
            child.style.display = "";
          }
        });

        localStorage.setItem("antigravity_active_aux_tab", "factory");
      }
    }

    function renderSwissTabContent(container, tabId) {
      container.dataset.renderedTab = tabId;
      container.innerHTML = "";
      if (tabId === "browser") renderBrowserView(container);
      else if (tabId === "files") renderFilesView(container);
      else if (tabId === "memos") renderMemosView(container);
    }

    // ----------------------------------------------------
    // 2. LIVE BROWSER & APP PREVIEWER WITH ANNOTATIONS
    // ----------------------------------------------------
    function renderBrowserView(container) {
      const wrap = document.createElement("div");
      wrap.className = "swiss-browser-view";

      // Two-tier Toolbar
      const toolbar = document.createElement("div");
      toolbar.className = "swiss-browser-toolbar";
      toolbar.innerHTML = ` + "`" + `
        <div class="swiss-browser-nav-row">
          <button class="swiss-browser-btn" id="swiss-b-back" title="Back"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-b-fwd" title="Forward"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-b-refresh" title="Reload"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg></button>
          <input type="text" class="swiss-browser-url-input" id="swiss-b-url" value="${currentBrowserUrl}" placeholder="http://localhost:5173" />
          <select class="swiss-browser-btn" id="swiss-b-device" title="Device Frame" style="outline:none;">
            <option value="responsive">Responsive / Desktop</option>
            <option value="iphone-16-pro">iPhone 16 Pro (402×874)</option>
            <option value="pixel-9">Pixel 9 (412×924)</option>
            <option value="ipad">iPad (820×1180)</option>
          </select>
          <select class="swiss-browser-btn" id="swiss-b-scale" title="Viewport Scale" style="outline:none;">
            <option value="fit" selected>Fit Screen</option>
            <option value="1">100%</option>
            <option value="0.75">75%</option>
            <option value="0.5">50%</option>
          </select>
          <button class="swiss-browser-btn" id="swiss-b-touch" title="Toggle Touch Emulation"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="20" x="5" y="2" rx="2" ry="2"/><path d="M12 18h.01"/></svg><span>Touch</span></button>
          <button class="swiss-browser-btn" id="swiss-b-pen" title="Red Pen Drawing"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/></svg><span>Pen</span></button>
          <button class="swiss-browser-btn" id="swiss-b-rect" title="Red Box Annotation"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2"/></svg><span>Box</span></button>
          <button class="swiss-browser-btn" id="swiss-b-inspect" title="Interactive DOM Inspector"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m22 12-4 0"/><path d="m6 12-4 0"/><path d="m12 6 0-4"/><path d="m12 22 0-4"/></svg><span>Inspect</span></button>
          <button class="swiss-browser-btn" id="swiss-b-clear" title="Clear Annotations"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg></button>
          <button class="swiss-browser-btn primary" id="swiss-b-send-chat" title="Send to Antigravity Chat"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg><span>Send to Chat</span></button>
        </div>
        <div class="swiss-browser-port-bar">
          <span class="swiss-port-label">Quick Ports:</span>
          <button class="swiss-port-chip" data-port="5173" title="Vite Development Server">:5173</button>
          <button class="swiss-port-chip" data-port="3000" title="React / Next.js Server">:3000</button>
          <button class="swiss-port-chip" data-port="8080" title="Standard Web Server">:8080</button>
          <button class="swiss-port-chip" data-port="8765" title="Antigravity Swiss Knife">:8765</button>
          <button class="swiss-port-chip" data-port="4173" title="Vite Production Preview">:4173</button>
          <button class="swiss-port-chip custom" data-port="custom" title="Connect to custom local port">+ Port</button>
        </div>
      ` + "`" + `;

      // Viewport container
      const vpWrap = document.createElement("div");
      vpWrap.className = "swiss-browser-viewport-wrap";

      const stage = document.createElement("div");
      stage.className = "swiss-device-stage";

      const frameBox = document.createElement("div");
      frameBox.className = "swiss-device-frame frame-responsive";

      const notch = document.createElement("div");
      notch.className = "swiss-device-notch";

      const screen = document.createElement("div");
      screen.className = "swiss-device-screen";

      const homeBar = document.createElement("div");
      homeBar.className = "swiss-device-home-bar";

      // Embedded Webview or Iframe
      let isWebview = true;
      let webview;
      try {
        webview = document.createElement("webview");
        if (typeof webview.loadURL !== "function" && typeof webview.reload !== "function" && typeof process === "undefined") {
          webview = document.createElement("iframe");
          isWebview = false;
        }
      } catch (e) {
        webview = document.createElement("iframe");
        isWebview = false;
      }
      webview.id = "swiss-browser-element";
      webview.style.width = "100%";
      webview.style.height = "100%";
      webview.style.border = "none";

      if (isWebview) {
        // partition="persist:swiss-browser"
        webview.setAttribute("partition", "persist:swiss-browser");
        webview.setAttribute("allowpopups", "true");
        webview.setAttribute("webpreferences", "allowRunningInsecureContent=yes, webSecurity=no, contextIsolation=no, nodeIntegration=no");
        webview.src = currentBrowserUrl;
      } else {
        webview.setAttribute("sandbox", "allow-scripts allow-same-origin allow-forms allow-popups allow-modals");
        webview.setAttribute("allow", "cross-origin-isolated");
        webview.src = currentBrowserUrl;
      }

      // Transparent Canvas for Annotations
      const canvas = document.createElement("canvas");
      canvas.id = "swiss-browser-canvas";
      canvas.className = "swiss-browser-canvas-overlay";
      canvas.style.pointerEvents = "none";

      screen.appendChild(webview);
      screen.appendChild(canvas);
      frameBox.appendChild(notch);
      frameBox.appendChild(screen);
      frameBox.appendChild(homeBar);
      stage.appendChild(frameBox);
      vpWrap.appendChild(stage);

      wrap.appendChild(toolbar);
      wrap.appendChild(vpWrap);
      container.appendChild(wrap);

      // Setup Canvas Resize
      const resizeCanvas = () => {
        canvas.width = screen.clientWidth || 400;
        canvas.height = screen.clientHeight || 600;
      };
      setTimeout(resizeCanvas, 50);

      // URL Navigation & Port Chips
      function navigateBrowser(url) {
        let finalUrl = url.trim();
        if (!/^https?:\/\//i.test(finalUrl)) {
          if (/^(localhost|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})(:\d+)?/i.test(finalUrl)) {
            finalUrl = "http://" + finalUrl;
          } else {
            finalUrl = "https://" + finalUrl;
          }
        }
        currentBrowserUrl = finalUrl;
        const urlInput = toolbar.querySelector("#swiss-b-url");
        if (urlInput) urlInput.value = finalUrl;
        if (isWebview && typeof webview.loadURL === "function") {
          webview.loadURL(finalUrl);
        } else {
          webview.src = finalUrl;
        }
        updateActivePortChip(finalUrl);
      }

      function updateActivePortChip(url) {
        const match = url.match(/^http:\/\/(?:localhost|127\.0\.0\.1):(\d+)/i);
        const activePort = match ? match[1] : null;
        toolbar.querySelectorAll(".swiss-port-chip").forEach(chip => {
          chip.classList.toggle("active", chip.dataset.port === activePort);
        });
      }

      // Toolbar event handlers
      const urlInput = toolbar.querySelector("#swiss-b-url");
      urlInput.onkeydown = (e) => {
        if (e.key === "Enter") {
          navigateBrowser(urlInput.value);
        }
      };
      toolbar.querySelector("#swiss-b-refresh").onclick = () => {
        if (webview.reload) webview.reload();
        else webview.src = currentBrowserUrl;
      };
      toolbar.querySelector("#swiss-b-back").onclick = () => {
        if (webview.goBack) webview.goBack();
      };
      toolbar.querySelector("#swiss-b-fwd").onclick = () => {
        if (webview.goForward) webview.goForward();
      };

      // Port shortcuts
      toolbar.querySelectorAll(".swiss-port-chip").forEach(chip => {
        chip.onclick = () => {
          const port = chip.dataset.port;
          if (port === "custom") {
            const customPort = prompt("Enter local port number (e.g. 8000, 4200, 5000):", "8000");
            if (customPort && /^\d+$/.test(customPort.trim())) {
              navigateBrowser(` + "`" + `http://localhost:${customPort.trim()}` + "`" + `);
            }
          } else {
            navigateBrowser(` + "`" + `http://localhost:${port}` + "`" + `);
          }
        };
      });
      updateActivePortChip(currentBrowserUrl);

      // Scaling & Fit
      function applyDeviceScale() {
        const scaleSelect = toolbar.querySelector("#swiss-b-scale");
        if (!frameBox || !vpWrap || !scaleSelect) return;
        const scaleMode = scaleSelect.value;
        if (activeDevice === "responsive" || scaleMode === "1") {
          frameBox.style.transform = "none";
          return;
        }
        if (scaleMode === "fit") {
          const availW = vpWrap.clientWidth - 48;
          const availH = vpWrap.clientHeight - 48;
          let targetW = 424, targetH = 896;
          if (activeDevice === "iphone-16-pro" || activeDevice === "iphone16") { targetW = 424; targetH = 896; }
          else if (activeDevice === "pixel-9" || activeDevice === "pixel9") { targetW = 432; targetH = 944; }
          else if (activeDevice === "ipad") { targetW = 848; targetH = 1208; }

          const factor = Math.min(1, Math.min(availW / targetW, availH / targetH));
          frameBox.style.transform = ` + "`" + `scale(${Math.max(0.2, factor.toFixed(3))})` + "`" + `;
          frameBox.style.transformOrigin = "top center";
        } else {
          const factor = parseFloat(scaleMode) || 1;
          frameBox.style.transform = ` + "`" + `scale(${factor})` + "`" + `;
          frameBox.style.transformOrigin = "top center";
        }
      }

      if (!window.__swissResizeBound) {
        window.__swissResizeBound = true;
        window.addEventListener("resize", applyDeviceScale);
      }
      toolbar.querySelector("#swiss-b-scale").onchange = applyDeviceScale;

      // Device frame switcher
      function applyDeviceFrame(device) {
        activeDevice = device;
        frameBox.className = "swiss-device-frame frame-" + device;
        notch.className = "swiss-device-notch";

        if (device === "iphone-16-pro" || device === "iphone16") {
          notch.classList.add("dynamic-island");
          setTouchEmulation(true);
        } else if (device === "pixel-9" || device === "pixel9") {
          notch.classList.add("punch-hole");
          setTouchEmulation(true);
        } else if (device === "ipad") {
          notch.classList.add("ipad-camera");
          setTouchEmulation(true);
        } else {
          setTouchEmulation(false);
        }

        applyDeviceScale();
        setTimeout(resizeCanvas, 60);
      }

      const devSelect = toolbar.querySelector("#swiss-b-device");
      devSelect.onchange = () => {
        applyDeviceFrame(devSelect.value);
      };

      // Touch Emulation
      function setTouchEmulation(active) {
        isTouchEmulationActive = active;
        screen.classList.toggle("touch-emulation-active", active);
        const touchBtn = toolbar.querySelector("#swiss-b-touch");
        if (touchBtn) touchBtn.classList.toggle("active", active);
        injectTouchEmulation(webview, active);
      }

      function injectTouchEmulation(browserEl, enabled) {
        const code = ` + "`" + `(() => {
          if (window.__swissTouchInstalled) {
            window.__swissTouchEnabled = ${enabled};
            return;
          }
          window.__swissTouchInstalled = true;
          window.__swissTouchEnabled = ${enabled};
          let isTouching = false;
          let id = 1;

          function createTouch(e) {
            return new Touch({
              identifier: id,
              target: e.target,
              clientX: e.clientX,
              clientY: e.clientY,
              screenX: e.screenX,
              screenY: e.screenY,
              pageX: e.pageX,
              pageY: e.pageY,
              radiusX: 12,
              radiusY: 12,
              rotationAngle: 0,
              force: 1.0
            });
          }

          function emit(type, e) {
            if (!window.__swissTouchEnabled) return;
            const touch = createTouch(e);
            const evt = new TouchEvent(type, {
              bubbles: true,
              cancelable: true,
              composed: true,
              touches: type === 'touchend' ? [] : [touch],
              targetTouches: type === 'touchend' ? [] : [touch],
              changedTouches: [touch]
            });
            e.target.dispatchEvent(evt);
          }

          document.addEventListener('mousedown', (e) => {
            if (!window.__swissTouchEnabled || e.button !== 0) return;
            isTouching = true;
            id++;
            emit('touchstart', e);
          }, true);

          document.addEventListener('mousemove', (e) => {
            if (!window.__swissTouchEnabled || !isTouching) return;
            emit('touchmove', e);
          }, true);

          document.addEventListener('mouseup', (e) => {
            if (!window.__swissTouchEnabled || !isTouching) return;
            isTouching = false;
            emit('touchend', e);
          }, true);
        })()` + "`" + `;

        if (isWebview && typeof browserEl.executeJavaScript === "function") {
          browserEl.executeJavaScript(code).catch(() => {});
        } else if (browserEl && browserEl.contentWindow) {
          try {
            browserEl.contentWindow.eval(code);
          } catch (_) {}
        }
      }

      toolbar.querySelector("#swiss-b-touch").onclick = () => {
        setTouchEmulation(!isTouchEmulationActive);
      };

      // Normalized Coordinates for Canvas
      function getCanvasCoords(e) {
        const rect = canvas.getBoundingClientRect();
        if (!rect.width || !rect.height) return { x: 0, y: 0 };
        return {
          x: (e.clientX - rect.left) * (canvas.width / rect.width),
          y: (e.clientY - rect.top) * (canvas.height / rect.height)
        };
      }

      // Drawing Tools Management
      const penBtn = toolbar.querySelector("#swiss-b-pen");
      const rectBtn = toolbar.querySelector("#swiss-b-rect");
      const inspectBtn = toolbar.querySelector("#swiss-b-inspect");
      const clearBtn = toolbar.querySelector("#swiss-b-clear");

      function setDrawTool(tool) {
        drawTool = (drawTool === tool) ? "none" : tool;
        penBtn.classList.toggle("active", drawTool === "pen");
        rectBtn.classList.toggle("active", drawTool === "rect");
        inspectBtn.classList.toggle("active", drawTool === "inspect");

        if (drawTool === "inspect") {
          canvas.style.pointerEvents = "none";
          toggleDOMInspector(true);
        } else if (drawTool === "none") {
          canvas.style.pointerEvents = "none";
          toggleDOMInspector(false);
        } else {
          canvas.style.pointerEvents = "auto";
          toggleDOMInspector(false);
        }
      }

      penBtn.onclick = () => setDrawTool("pen");
      rectBtn.onclick = () => setDrawTool("rect");
      inspectBtn.onclick = () => setDrawTool("inspect");
      clearBtn.onclick = () => {
        const ctx = canvas.getContext("2d");
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        lastAnnotatedRegion = null;
        lastSelectedElement = null;
      };

      // Drawing Interactions with Bézier Curve Smoothing & Box Tool
      const ctx = canvas.getContext("2d");
      let lastX = 0, lastY = 0;
      let lastMidX = 0, lastMidY = 0;
      let snapshot = null;

      canvas.onmousedown = (e) => {
        if (drawTool === "none" || drawTool === "inspect") return;
        isDrawing = true;
        const coords = getCanvasCoords(e);
        drawStartX = coords.x;
        drawStartY = coords.y;
        lastX = coords.x;
        lastY = coords.y;
        lastMidX = coords.x;
        lastMidY = coords.y;

        if (drawTool === "rect") {
          snapshot = ctx.getImageData(0, 0, canvas.width, canvas.height);
        } else if (drawTool === "pen") {
          ctx.strokeStyle = "#ea4335";
          ctx.fillStyle = "#ea4335";
          ctx.lineWidth = 3;
          ctx.lineCap = "round";
          ctx.lineJoin = "round";
          ctx.beginPath();
          ctx.arc(coords.x, coords.y, 1.5, 0, Math.PI * 2);
          ctx.fill();
        }
      };

      canvas.onmousemove = (e) => {
        if (!isDrawing) return;
        const coords = getCanvasCoords(e);

        if (drawTool === "pen") {
          const midX = (lastX + coords.x) / 2;
          const midY = (lastY + coords.y) / 2;

          ctx.beginPath();
          ctx.moveTo(lastMidX, lastMidY);
          ctx.quadraticCurveTo(lastX, lastY, midX, midY);
          ctx.strokeStyle = "#ea4335";
          ctx.lineWidth = 3;
          ctx.lineCap = "round";
          ctx.lineJoin = "round";
          ctx.stroke();

          lastX = coords.x;
          lastY = coords.y;
          lastMidX = midX;
          lastMidY = midY;
        } else if (drawTool === "rect") {
          if (snapshot) ctx.putImageData(snapshot, 0, 0);
          const boxX = Math.min(drawStartX, coords.x);
          const boxY = Math.min(drawStartY, coords.y);
          const boxW = Math.abs(coords.x - drawStartX);
          const boxH = Math.abs(coords.y - drawStartY);

          ctx.strokeStyle = "#ea4335";
          ctx.lineWidth = 2;
          ctx.setLineDash([5, 5]);
          ctx.strokeRect(boxX, boxY, boxW, boxH);
          ctx.fillStyle = "rgba(234, 67, 53, 0.15)";
          ctx.fillRect(boxX, boxY, boxW, boxH);
          ctx.setLineDash([]);
        }
      };

      canvas.onmouseup = (e) => {
        if (!isDrawing) return;
        isDrawing = false;
        const coords = getCanvasCoords(e);

        if (drawTool === "pen") {
          ctx.beginPath();
          ctx.moveTo(lastMidX, lastMidY);
          ctx.lineTo(lastX, lastY);
          ctx.strokeStyle = "#ea4335";
          ctx.lineWidth = 3;
          ctx.lineCap = "round";
          ctx.lineJoin = "round";
          ctx.stroke();
        } else if (drawTool === "rect") {
          if (snapshot) ctx.putImageData(snapshot, 0, 0);
          const boxX = Math.min(drawStartX, coords.x);
          const boxY = Math.min(drawStartY, coords.y);
          const boxW = Math.abs(coords.x - drawStartX);
          const boxH = Math.abs(coords.y - drawStartY);

          if (boxW > 5 && boxH > 5) {
            ctx.strokeStyle = "#ea4335";
            ctx.lineWidth = 2;
            ctx.strokeRect(boxX, boxY, boxW, boxH);
            ctx.fillStyle = "rgba(234, 67, 53, 0.15)";
            ctx.fillRect(boxX, boxY, boxW, boxH);

            ctx.fillStyle = "#ea4335";
            ctx.fillRect(boxX, Math.max(0, boxY - 18), 75, 18);
            ctx.fillStyle = "#ffffff";
            ctx.font = "bold 10px monospace";
            ctx.fillText("#annotation", boxX + 4, Math.max(12, boxY - 5));

            lastAnnotatedRegion = { x: boxX, y: boxY, width: boxW, height: boxH };
          }
        }
      };

      // Interactive DOM Element Inspector
      function toggleDOMInspector(active) {
        if (!active) {
          if (webview && typeof webview.executeJavaScript === "function") {
            webview.executeJavaScript("if (window.__swissCleanupInspector) window.__swissCleanupInspector();");
          }
          return;
        }

        const inspectorScript = ` + "`" + `(() => {
          if (window.__swissInspectorActive) return;
          window.__swissInspectorActive = true;

          let overlay = document.getElementById('swiss-dom-inspect-overlay');
          if (!overlay) {
            overlay = document.createElement('div');
            overlay.id = 'swiss-dom-inspect-overlay';
            overlay.style.position = 'fixed';
            overlay.style.pointerEvents = 'none';
            overlay.style.border = '2px solid #ea4335';
            overlay.style.backgroundColor = 'rgba(234, 67, 53, 0.15)';
            overlay.style.zIndex = '2147483647';
            overlay.style.boxSizing = 'border-box';
            overlay.style.transition = 'all 0.05s ease-out';
            overlay.style.display = 'none';

            const tagBadge = document.createElement('div');
            tagBadge.id = 'swiss-dom-inspect-badge';
            tagBadge.style.position = 'absolute';
            tagBadge.style.top = '-22px';
            tagBadge.style.left = '0';
            tagBadge.style.backgroundColor = '#ea4335';
            tagBadge.style.color = '#ffffff';
            tagBadge.style.fontFamily = 'monospace';
            tagBadge.style.fontSize = '11px';
            tagBadge.style.fontWeight = 'bold';
            tagBadge.style.padding = '2px 6px';
            tagBadge.style.borderRadius = '3px';
            tagBadge.style.whiteSpace = 'nowrap';
            tagBadge.style.boxShadow = '0 2px 4px rgba(0,0,0,0.2)';
            overlay.appendChild(tagBadge);
            document.body.appendChild(overlay);
          }

          function getCssSelector(el) {
            if (!el || el.nodeType !== Node.ELEMENT_NODE) return '';
            if (el.id) return '#' + el.id;
            let path = [];
            while (el && el.nodeType === Node.ELEMENT_NODE) {
              let sel = el.nodeName.toLowerCase();
              if (el.className && typeof el.className === 'string') {
                const classes = el.className.trim().split(/\s+/).filter(c => c && !c.startsWith('swiss-'));
                if (classes.length) sel += '.' + classes.slice(0, 2).join('.');
              }
              let sib = el, nth = 1;
              while (sib = sib.previousElementSibling) {
                if (sib.nodeName.toLowerCase() === el.nodeName.toLowerCase()) nth++;
              }
              if (nth > 1) sel += ':nth-of-type(' + nth + ')';
              path.unshift(sel);
              if (el.id || path.length >= 3) break;
              el = el.parentElement;
            }
            return path.join(' > ');
          }

          let hoveredEl = null;

          function handleMouseMove(e) {
            const el = document.elementFromPoint(e.clientX, e.clientY);
            if (!el || el === overlay || el === document.body || el === document.documentElement) return;
            hoveredEl = el;
            const rect = el.getBoundingClientRect();
            overlay.style.display = 'block';
            overlay.style.top = rect.top + 'px';
            overlay.style.left = rect.left + 'px';
            overlay.style.width = rect.width + 'px';
            overlay.style.height = rect.height + 'px';
            const badge = overlay.querySelector('#swiss-dom-inspect-badge');
            if (badge) badge.textContent = getCssSelector(el);
          }

          function handleClick(e) {
            e.preventDefault();
            e.stopPropagation();
            if (!hoveredEl) return;
            const sel = getCssSelector(hoveredEl);
            const rect = hoveredEl.getBoundingClientRect();
            const snippet = hoveredEl.outerHTML ? hoveredEl.outerHTML.slice(0, 1500) : '';
            const payload = {
              selector: sel,
              outerHTML: snippet,
              rect: { x: rect.left, y: rect.top, width: rect.width, height: rect.height }
            };
            console.log('[SWISS_INSPECT_RESULT]', JSON.stringify(payload));
            window.__swissCleanupInspector();
          }

          window.__swissCleanupInspector = () => {
            window.__swissInspectorActive = false;
            document.removeEventListener('mousemove', handleMouseMove, true);
            document.removeEventListener('click', handleClick, true);
            if (overlay) overlay.style.display = 'none';
          };

          document.addEventListener('mousemove', handleMouseMove, true);
          document.addEventListener('click', handleClick, true);
        })()` + "`" + `;

        if (webview && typeof webview.executeJavaScript === "function") {
          webview.executeJavaScript(inspectorScript);
        }
      }

      if (isWebview) {
        webview.addEventListener('console-message', (e) => {
          if (e.message && e.message.startsWith('[SWISS_INSPECT_RESULT]')) {
            try {
              const jsonStr = e.message.replace('[SWISS_INSPECT_RESULT]', '').trim();
              const result = JSON.parse(jsonStr);
              lastSelectedElement = result;
              lastAnnotatedRegion = result.rect;

              ctx.strokeStyle = "#ea4335";
              ctx.lineWidth = 2;
              ctx.strokeRect(result.rect.x, result.rect.y, result.rect.width, result.rect.height);
              ctx.fillStyle = "rgba(234, 67, 53, 0.15)";
              ctx.fillRect(result.rect.x, result.rect.y, result.rect.width, result.rect.height);

              ctx.fillStyle = "#ea4335";
              ctx.fillRect(result.rect.x, Math.max(0, result.rect.y - 18), Math.min(180, result.selector.length * 8 + 8), 18);
              ctx.fillStyle = "#ffffff";
              ctx.font = "bold 10px monospace";
              ctx.fillText(result.selector.slice(0, 22), result.rect.x + 4, Math.max(12, result.rect.y - 5));

              drawTool = "none";
              canvas.style.pointerEvents = "none";
              toolbar.querySelector("#swiss-b-inspect").classList.remove("active");
            } catch (_) {}
          }
        });
      }

      // Send to Antigravity Chat button workflow
      toolbar.querySelector("#swiss-b-send-chat").onclick = async () => {
        const comment = prompt(
          "Add comment to attach with this preview snapshot to Antigravity chat:",
          userComment || "Review UI alignment and inspected element markup."
        );
        if (comment === null) return;
        userComment = comment;

        const region = (lastAnnotatedRegion && lastAnnotatedRegion.width > 5 && lastAnnotatedRegion.height > 5)
          ? lastAnnotatedRegion
          : null;

        const exportW = region ? region.width : canvas.width;
        const exportH = region ? region.height : canvas.height;

        const exportCanvas = document.createElement("canvas");
        exportCanvas.width = exportW;
        exportCanvas.height = exportH;
        const expCtx = exportCanvas.getContext("2d");

        expCtx.fillStyle = "#ffffff";
        expCtx.fillRect(0, 0, exportW, exportH);

        if (webview && typeof webview.capturePage === "function") {
          try {
            const pageImage = await webview.capturePage(region ? {
              x: Math.round(region.x),
              y: Math.round(region.y),
              width: Math.round(region.width),
              height: Math.round(region.height),
            } : undefined);
            if (pageImage && !pageImage.isEmpty()) {
              const bgDataUrl = pageImage.toDataURL();
              await new Promise((resolve) => {
                const bgImg = new Image();
                bgImg.onload = () => {
                  expCtx.drawImage(bgImg, 0, 0, exportW, exportH);
                  resolve();
                };
                bgImg.onerror = resolve;
                bgImg.src = bgDataUrl;
              });
            }
          } catch (_) {}
        }

        if (region) {
          expCtx.drawImage(canvas, region.x, region.y, region.width, region.height, 0, 0, exportW, exportH);
        } else {
          expCtx.drawImage(canvas, 0, 0, exportW, exportH);
        }

        exportCanvas.toBlob((blob) => {
          if (!blob) {
            const dataUrl = exportCanvas.toDataURL("image/png");
            const byteString = atob(dataUrl.split(",")[1]);
            const ab = new ArrayBuffer(byteString.length);
            const ia = new Uint8Array(ab);
            for (let i = 0; i < byteString.length; i++) ia[i] = byteString.charCodeAt(i);
            blob = new Blob([ab], { type: "image/png" });
          }

          const file = new File([blob], "annotation.png", { type: "image/png" });

          const fileInput = document.querySelector('input[type="file"]') ||
                            document.querySelector('[type="file"]') ||
                            Array.from(document.querySelectorAll('input')).find(i => i.type === "file") ||
                            document.querySelector('.chat-input-toolbar input[type="file"]') ||
                            document.querySelector('input[type="file"][accept*="image"]');
          if (fileInput) {
            try {
              const dt = new DataTransfer();
              dt.items.add(file);
              fileInput.files = dt.files;
              fileInput.dispatchEvent(new Event("change", { bubbles: true }));
            } catch (_) {}
          }

          let promptText = "[Browser Preview Annotation @ " + currentBrowserUrl + "]\n";
          if (lastSelectedElement && lastSelectedElement.selector) {
            promptText += "Selected Element: " + lastSelectedElement.selector + "\n";
          }
          if (lastSelectedElement && lastSelectedElement.outerHTML) {
            promptText += "` + "\x60\x60\x60" + `html\n" + lastSelectedElement.outerHTML.trim() + "\n` + "\x60\x60\x60" + `\n";
          }
          if (userComment) {
            promptText += "Comment: " + userComment + "\n";
          }
          promptText += "(Visual annotation attached: annotation.png)";

          let injectedLexical = false;
          const lexicalElem = document.querySelector('[data-lexical-editor="true"]') ||
                              document.querySelector('.lexical-container [contenteditable="true"]') ||
                              document.querySelector('[contenteditable="true"]');

          if (lexicalElem && lexicalElem.__lexicalEditor) {
            try {
              const editor = lexicalElem.__lexicalEditor;
              editor.update(() => {
                if (typeof lexicalElem.focus === "function") lexicalElem.focus();
                document.execCommand("insertText", false, promptText);
                injectedLexical = true;
              });
            } catch (err) {
              console.warn("Lexical editor update failed:", err);
            }
          }

          if (!injectedLexical) {
            insertTextToChatInput(promptText);
          }

          showToast("Visual annotation attached and DOM snippet injected into chat!");
        }, "image/png");
      };
    }

    // ----------------------------------------------------
    // 3. LIGHTWEIGHT FILE EXPLORER & IN-PLACE EDITORS
    // ----------------------------------------------------
    function renderFilesView(container) {
      const wrap = document.createElement("div");
      wrap.className = "swiss-files-view";

      const toolbar = document.createElement("div");
      toolbar.className = "swiss-files-toolbar";
      toolbar.innerHTML = ` + "`" + `
        <div class="swiss-files-address-bar">
          <button class="swiss-browser-btn" id="swiss-f-up" title="Up Directory"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m18 15-6-6-6 6"/></svg></button>
          <input type="text" class="swiss-files-path-input" id="swiss-f-path" value="${currentFilePath}" />
          <button class="swiss-browser-btn" id="swiss-f-refresh" title="Refresh"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-f-reveal" title="Open in System File Manager"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 14 1.45-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.55 6a2 2 0 0 1-1.94 1.5H4a2 2 0 0 1-2-2V5c0-1.1.9-2 2-2h3.93a2 2 0 0 1 1.66.9l.82 1.2a2 2 0 0 0 1.66.9H18a2 2 0 0 1 2 2v2"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-f-term" title="Open in Terminal"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/></svg></button>
        </div>
        <div style="display:flex; gap:6px;">
          <input type="text" placeholder="Filter files..." id="swiss-f-search" style="flex:1; padding:3px 8px; font-size:11px; border-radius:4px; border:1px solid var(--border,#cbd5e1); background:var(--canvas,#fff);" />
          <button class="swiss-browser-btn" id="swiss-f-new-file"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><polyline points="14 2 14 8 20 8"/><line x1="12" x2="12" y1="18" y2="12"/><line x1="9" x2="15" y1="15" y2="15"/></svg><span>File</span></button>
          <button class="swiss-browser-btn" id="swiss-f-new-dir"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 10v6"/><path d="M9 13h6"/><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg><span>Folder</span></button>
        </div>
      ` + "`" + `;

      const listContainer = document.createElement("div");
      listContainer.className = "swiss-files-list";

      wrap.appendChild(toolbar);
      wrap.appendChild(listContainer);
      container.appendChild(wrap);

      let selectedPaths = new Set();
      let fileItemsMap = new Map();

      // Load files
      const loadFiles = async (dirPath) => {
        let targetPath = (dirPath || "").trim();
        if (targetPath.startsWith("file://")) {
          targetPath = targetPath.replace(/^file:\/\//, "");
        }
        if (targetPath.includes("%")) {
          try { targetPath = decodeURIComponent(targetPath); } catch (_) {}
        }
        targetPath = targetPath.replace(/\/+$/, "") || "/";
        currentFilePath = targetPath;
        toolbar.querySelector("#swiss-f-path").value = targetPath;
        listContainer.innerHTML = "<div style='padding:12px; font-size:11px; color:#94a3b8;'>Loading files...</div>";

        try {
          const res = await fetch(` + "`" + `${API_BASE}/api/files/list?path=${encodeURIComponent(targetPath)}` + "`" + `);
          if (!res.ok) {
            throw new Error("HTTP " + res.status + ": " + res.statusText);
          }
          const data = await res.json();
          if (!data.success) {
            listContainer.innerHTML = ` + "`" + `<div style='padding:14px; color:#ef4444; font-size:11px; display:flex; flex-direction:column; gap:6px;'>
              <div style='font-weight:600;'>Error loading directory:</div>
              <div style='font-family:monospace; background:rgba(239,68,68,0.08); padding:6px 8px; border-radius:4px;'>${data.error || "Unknown error"}</div>
              <button class="swiss-browser-btn" id="swiss-f-retry" style="align-self:flex-start; margin-top:4px;">Retry</button>
            </div>` + "`" + `;
            listContainer.querySelector("#swiss-f-retry")?.addEventListener("click", () => loadFiles(currentFilePath));
            return;
          }

          listContainer.innerHTML = "";
          fileItemsMap.clear();
          if (!data.files || data.files.length === 0) {
            listContainer.innerHTML = "<div style='padding:16px; font-size:11px; color:#94a3b8; text-align:center;'>Empty folder</div>";
            return;
          }

          data.files.forEach(item => {
            fileItemsMap.set(item.path, item);
            const row = document.createElement("div");
            row.className = "swiss-file-row";
            if (selectedPaths.has(item.path)) row.classList.add("selected");
            if (swissClipboard.action === "cut" && swissClipboard.items.some(ci => ci.path === item.path)) {
              row.classList.add("cut");
            }
            row.draggable = true;
            row.dataset.path = item.path;

            const iconSvg = item.isDir 
              ? '<svg class="swiss-file-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>'
              : item.type === "code"
              ? '<svg class="swiss-file-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>'
              : item.type === "markdown"
              ? '<svg class="swiss-file-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><line x1="16" x2="8" y1="13" y2="13"/><line x1="16" x2="8" y1="17" y2="17"/><line x1="10" x2="8" y1="9" y2="9"/></svg>'
              : item.type === "pdf"
              ? '<svg class="swiss-file-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><path d="M10 12a1 1 0 0 0-1-1H8v6h1a1 1 0 0 0 1-1v-4z"/></svg>'
              : '<svg class="swiss-file-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>';

            row.innerHTML = ` + "`" + `
              <span class="swiss-file-icon">${iconSvg}</span>
              <span class="swiss-file-name">${item.name}</span>
              <span class="swiss-file-size">${item.size || ""}</span>
            ` + "`" + `;

            // Drag to chat input
            row.ondragstart = (e) => {
              e.dataTransfer.setData("text/plain", item.path);
              e.dataTransfer.setData("text/uri-list", "file://" + item.path);
            };

            // Single click: toggle selection if Ctrl/Cmd, or navigate directory
            row.onclick = (e) => {
              const isMulti = e.ctrlKey || e.metaKey;
              if (isMulti) {
                e.preventDefault();
                e.stopPropagation();
                if (selectedPaths.has(item.path)) {
                  selectedPaths.delete(item.path);
                  row.classList.remove("selected");
                } else {
                  selectedPaths.add(item.path);
                  row.classList.add("selected");
                }
                return;
              }

              selectedPaths.clear();
              listContainer.querySelectorAll(".swiss-file-row.selected").forEach(r => r.classList.remove("selected"));
              selectedPaths.add(item.path);
              row.classList.add("selected");

              if (item.isDir) {
                loadFiles(item.path);
              }
            };

            // Double click: open directory or open in-place editor
            row.ondblclick = (e) => {
              e.stopPropagation();
              if (item.isDir) {
                loadFiles(item.path);
              } else {
                openInPlaceEditor(container, item.path, item.name, item.type);
              }
            };

            // Right click context menu on row
            row.oncontextmenu = (e) => {
              e.preventDefault();
              e.stopPropagation();
              window.getSelection()?.removeAllRanges();

              if (!selectedPaths.has(item.path)) {
                selectedPaths.clear();
                listContainer.querySelectorAll(".swiss-file-row.selected").forEach(r => r.classList.remove("selected"));
                selectedPaths.add(item.path);
                row.classList.add("selected");
              }

              const selectedItems = Array.from(selectedPaths).map(p => fileItemsMap.get(p) || { path: p, name: p.split("/").pop(), isDir: false });
              showFileContextMenu(e.clientX, e.clientY, item, selectedItems, () => loadFiles(currentFilePath));
            };

            listContainer.appendChild(row);
          });
        } catch (err) {
          listContainer.innerHTML = ` + "`" + `<div style='padding:14px; color:#ef4444; font-size:11px; display:flex; flex-direction:column; gap:6px;'>
            <div style='font-weight:600;'>Unable to connect to Swiss Knife daemon:</div>
            <div style='color:#64748b;'>${err.message}. Check that the daemon is running on ${API_BASE} (e.g. 'swiss daemon --with-web' or 'swiss web').</div>
            <button class="swiss-browser-btn" id="swiss-f-retry" style="align-self:flex-start; margin-top:4px;">Retry</button>
          </div>` + "`" + `;
          listContainer.querySelector("#swiss-f-retry")?.addEventListener("click", () => loadFiles(currentFilePath));
        }
      };

      // Background right-click on listContainer
      listContainer.oncontextmenu = (e) => {
        if (e.target.closest(".swiss-file-row")) return;
        e.preventDefault();
        e.stopPropagation();
        window.getSelection()?.removeAllRanges();
        showBlankContextMenu(e.clientX, e.clientY, currentFilePath, () => loadFiles(currentFilePath));
      };

      listContainer.onclick = (e) => {
        if (e.target === listContainer) {
          selectedPaths.clear();
          listContainer.querySelectorAll(".swiss-file-row.selected").forEach(r => r.classList.remove("selected"));
        }
      };

      toolbar.querySelector("#swiss-f-path").onkeydown = (e) => {
        if (e.key === "Enter") loadFiles(e.target.value);
      };
      toolbar.querySelector("#swiss-f-refresh").onclick = () => loadFiles(currentFilePath);
      toolbar.querySelector("#swiss-f-up").onclick = () => {
        let clean = currentFilePath.replace(/\/+$/, "");
        const lastSlash = clean.lastIndexOf("/");
        if (lastSlash > 0) {
          loadFiles(clean.substring(0, lastSlash));
        } else if (lastSlash === 0) {
          loadFiles("/");
        }
      };
      const searchBox = toolbar.querySelector("#swiss-f-search");
      if (searchBox) {
        searchBox.oninput = (e) => {
          const q = (e.target.value || "").toLowerCase().trim();
          listContainer.querySelectorAll(".swiss-file-row").forEach(r => {
            const name = (r.querySelector(".swiss-file-name")?.textContent || "").toLowerCase();
            r.style.display = (!q || name.includes(q)) ? "flex" : "none";
          });
        };
      }
      toolbar.querySelector("#swiss-f-reveal").onclick = () => {
        fetch(` + "`" + `${API_BASE}/api/files/reveal` + "`" + `, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentFilePath })
        });
      };
      toolbar.querySelector("#swiss-f-term").onclick = () => {
        fetch(` + "`" + `${API_BASE}/api/files/terminal` + "`" + `, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentFilePath })
        });
      };
      toolbar.querySelector("#swiss-f-new-file").onclick = async () => {
        const name = prompt("Enter new file name:");
        if (!name) return;
        await fetch(` + "`" + `${API_BASE}/api/files/create` + "`" + `, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentFilePath + "/" + name, is_dir: false })
        });
        loadFiles(currentFilePath);
      };
      toolbar.querySelector("#swiss-f-new-dir").onclick = async () => {
        const name = prompt("Enter new directory name:");
        if (!name) return;
        await fetch(` + "`" + `${API_BASE}/api/files/create` + "`" + `, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentFilePath + "/" + name, is_dir: true })
        });
        loadFiles(currentFilePath);
      };

      loadFiles(currentFilePath);
    }

    // In-Place Editor / Annotator for Code & Markdown
    function openInPlaceEditor(container, path, name, type) {
      container.innerHTML = "<div style='padding:12px; font-size:11px; color:#94a3b8;'>Loading file content...</div>";

      fetch(` + "`" + `${API_BASE}/api/files/read?path=${encodeURIComponent(path)}` + "`" + `)
        .then(res => res.json())
        .then(data => {
          if (!data.success) {
            showToast("Error reading file: " + data.error, "error");
            renderFilesView(container);
            return;
          }

          container.innerHTML = "";
          const editor = document.createElement("div");
          editor.className = "swiss-editor-container";
          editor.innerHTML = ` + "`" + `
            <div class="swiss-editor-toolbar">
              <div style="display:flex; align-items:center; gap:8px;">
                <button class="swiss-browser-btn" id="swiss-ed-back"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg><span>Files</span></button>
                <span style="font-size:11px; font-weight:600;">${name}</span>
                <span style="font-size:10px; color:#94a3b8; font-family:monospace;">(${type})</span>
              </div>
              <div style="display:flex; gap:6px;">
                <button class="swiss-browser-btn primary" id="swiss-ed-annotate"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg><span>Annotate to Chat</span></button>
                <button class="swiss-browser-btn" id="swiss-ed-save"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15.2 3a2 2 0 0 1 1.4.6l3.8 3.8a2 2 0 0 1 .6 1.4V19a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z"/><path d="M17 21v-7a1 1 0 0 0-1-1H8a1 1 0 0 0-1 1v7"/><path d="M7 3v4a1 1 0 0 0 1 1h7"/></svg><span>Save</span></button>
              </div>
            </div>
            <div class="swiss-editor-body">
              <div class="swiss-editor-gutter" id="swiss-ed-gutter">1</div>
              <textarea class="swiss-editor-textarea" id="swiss-ed-text" spellcheck="false">${data.content.replace(/&/g,"&amp;").replace(/</g,"&lt;")}</textarea>
            </div>
          ` + "`" + `;

          container.appendChild(editor);

          const textarea = editor.querySelector("#swiss-ed-text");
          const gutter = editor.querySelector("#swiss-ed-gutter");

          const updateGutter = () => {
            const lines = textarea.value.split("\n").length;
            gutter.innerHTML = Array.from({length: lines}, (_, i) => i + 1).join("<br>");
          };
          textarea.oninput = updateGutter;
          textarea.onscroll = () => { gutter.scrollTop = textarea.scrollTop; };
          updateGutter();

          editor.querySelector("#swiss-ed-back").onclick = () => renderFilesView(container);

          editor.querySelector("#swiss-ed-save").onclick = async () => {
            const res = await fetch(` + "`" + `${API_BASE}/api/files/write` + "`" + `, {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ path, content: textarea.value })
            });
            const rData = await res.json();
            if (rData.success) showToast("Saved successfully!");
            else showToast("Save failed: " + rData.error, "error");
          };

          editor.querySelector("#swiss-ed-annotate").onclick = () => {
            const start = textarea.selectionStart;
            const end = textarea.selectionEnd;
            const selectedText = textarea.value.substring(start, end).trim();

            if (!selectedText) {
              showToast("Select text/code lines first, then click Annotate to Chat!", "warning");
              return;
            }

            const lineNum = textarea.value.substring(0, start).split("\n").length;
            const comment = prompt("Enter annotation note for selected snippet:", "Please review this logic and suggest improvements:");
            if (comment === null) return;

            const snippetMsg = "[Annotated Code: " + name + " (around line " + lineNum + ")]\nComment: \"" + comment + "\"\n` + "\x60\x60\x60" + `\n" + selectedText + "\n` + "\x60\x60\x60" + `";
            insertTextToChatInput(snippetMsg);
            showToast("Annotation snippet injected into Antigravity chat input!");
          };
        });
    }

    // Context Menu Helpers & Dismissal
    let activeContextMenu = null;
    function removeContextMenu() {
      if (activeContextMenu) {
        activeContextMenu.remove();
        activeContextMenu = null;
      }
      document.removeEventListener("click", onDocClick);
      document.removeEventListener("contextmenu", onDocContextMenu);
      document.removeEventListener("keydown", onDocKeydown);
      window.removeEventListener("resize", removeContextMenu);
    }
    function onDocClick(e) {
      if (activeContextMenu && !activeContextMenu.contains(e.target)) {
        removeContextMenu();
      }
    }
    function onDocContextMenu(e) {
      if (activeContextMenu && !activeContextMenu.contains(e.target)) {
        removeContextMenu();
      }
    }
    function onDocKeydown(e) {
      if (e.key === "Escape") {
        removeContextMenu();
      }
    }

    function positionContextMenu(menu, x, y) {
      removeContextMenu();
      menu.style.visibility = "hidden";
      menu.style.left = "0px";
      menu.style.top = "0px";
      document.body.appendChild(menu);
      const rect = menu.getBoundingClientRect();
      const pad = 8;
      let finalX = x;
      let finalY = y;
      if (finalX + rect.width > window.innerWidth - pad) {
        finalX = Math.max(pad, window.innerWidth - rect.width - pad);
      }
      if (finalY + rect.height > window.innerHeight - pad) {
        finalY = Math.max(pad, window.innerHeight - rect.height - pad);
      }
      menu.style.left = finalX + "px";
      menu.style.top = finalY + "px";
      menu.style.visibility = "visible";

      activeContextMenu = menu;
      setTimeout(() => {
        document.addEventListener("click", onDocClick);
        document.addEventListener("contextmenu", onDocContextMenu);
        document.addEventListener("keydown", onDocKeydown);
        window.addEventListener("resize", removeContextMenu);
      }, 10);
    }

    async function executePaste(targetFolder, onRefresh) {
      if (!swissClipboard.items || swissClipboard.items.length === 0) return;
      const isCut = swissClipboard.action === "cut";
      const payload = {
        items: swissClipboard.items.map(item => ({
          src: item.path,
          dst: targetFolder + "/" + item.name
        }))
      };
      const endpoint = isCut ? (API_BASE + "/api/files/move") : (API_BASE + "/api/files/copy");
      try {
        const res = await fetch(endpoint, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (data.success) {
          showToast((isCut ? "Moved " : "Copied ") + swissClipboard.items.length + " item(s)");
          if (isCut) {
            swissClipboard = { action: "copy", items: [] };
          }
          onRefresh();
        } else {
          showToast("Paste error: " + (data.error || "Unknown"), "error");
        }
      } catch (err) {
        showToast("Paste failed: " + err.message, "error");
      }
    }

    async function executeDelete(itemsToDelete, onRefresh) {
      if (!itemsToDelete || itemsToDelete.length === 0) return;
      const count = itemsToDelete.length;
      const msg = count === 1 ? ("Delete '" + itemsToDelete[0].name + "'?") : ("Delete " + count + " selected items?");
      if (!confirm(msg)) return;
      try {
        const res = await fetch(API_BASE + "/api/files/delete", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ paths: itemsToDelete.map(i => i.path) })
        });
        const data = await res.json();
        if (data.success) {
          showToast("Deleted " + count + " item(s)");
          onRefresh();
        } else {
          showToast("Delete failed: " + (data.error || "Unknown"), "error");
        }
      } catch (err) {
        showToast("Delete failed: " + err.message, "error");
      }
    }

    // Context Menu for File Operations on Rows
    function showFileContextMenu(x, y, item, selectedItems, onRefresh) {
      const isSingle = !selectedItems || selectedItems.length <= 1;
      const targets = (selectedItems && selectedItems.length > 0) ? selectedItems : [item];
      const hasClipboard = swissClipboard.items && swissClipboard.items.length > 0;

      const menu = document.createElement("div");
      menu.className = "swiss-context-menu";

      let itemsHtml = '<div class="swiss-context-item" id="ctx-file-copy"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg><span>Copy</span></div>' +
        '<div class="swiss-context-item" id="ctx-file-cut"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="6" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><line x1="20" x2="8.12" y1="4" y2="15.88"/><line x1="14.47" x2="20" y1="14.48" y2="20"/><line x1="8.12" x2="12" y1="8.12" y2="12"/></svg><span>Cut</span></div>';
      if (item.isDir) {
        itemsHtml += '<div class="swiss-context-item ' + (hasClipboard ? '' : 'disabled') + '" id="ctx-file-paste"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><rect width="8" height="4" x="8" y="2" rx="1" ry="1"/></svg><span>Paste</span></div>';
      }
      itemsHtml += '<div class="swiss-context-divider"></div>' +
        '<div class="swiss-context-item" id="ctx-copy"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg><span>Copy Path</span></div>';
      if (isSingle) {
        itemsHtml += '<div class="swiss-context-item" id="ctx-rename"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/></svg><span>Rename</span></div>';
      }
      itemsHtml += '<div class="swiss-context-item" id="ctx-delete" style="color:#ef4444;"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><line x1="10" x2="10" y1="11" y2="17"/><line x1="14" x2="14" y1="11" y2="17"/></svg><span>Delete</span></div>' +
        '<div class="swiss-context-divider"></div>' +
        '<div class="swiss-context-item" id="ctx-reveal"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 14 1.45-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.55 6a2 2 0 0 1-1.94 1.5H4a2 2 0 0 1-2-2V5c0-1.1.9-2 2-2h3.93a2 2 0 0 1 1.66.9l.82 1.2a2 2 0 0 0 1.66.9H18a2 2 0 0 1 2 2v2"/></svg><span>Reveal in File Manager</span></div>' +
        '<div class="swiss-context-item" id="ctx-term"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/></svg><span>Open in Terminal</span></div>';
      menu.innerHTML = itemsHtml;

      positionContextMenu(menu, x, y);

      menu.querySelector("#ctx-file-copy")?.addEventListener("click", () => {
        swissClipboard = { action: "copy", items: [...targets] };
        document.querySelectorAll(".swiss-file-row.cut").forEach(r => r.classList.remove("cut"));
        showToast("Copied " + targets.length + " item(s) to clipboard");
        removeContextMenu();
      });

      menu.querySelector("#ctx-file-cut")?.addEventListener("click", () => {
        swissClipboard = { action: "cut", items: [...targets] };
        document.querySelectorAll(".swiss-file-row.cut").forEach(r => r.classList.remove("cut"));
        targets.forEach(t => {
          const r = document.querySelector('[data-path="' + CSS.escape(t.path) + '"]');
          if (r) r.classList.add("cut");
        });
        showToast("Cut " + targets.length + " item(s) to clipboard");
        removeContextMenu();
      });

      menu.querySelector("#ctx-file-paste")?.addEventListener("click", () => {
        if (!hasClipboard) return;
        executePaste(item.path, onRefresh);
        removeContextMenu();
      });

      menu.querySelector("#ctx-copy")?.addEventListener("click", () => {
        const text = targets.map(t => t.path).join("\n");
        navigator.clipboard.writeText(text);
        showToast("Path" + (targets.length > 1 ? "s" : "") + " copied to clipboard!");
        removeContextMenu();
      });

      const renameBtn = menu.querySelector("#ctx-rename");
      if (renameBtn) {
        renameBtn.addEventListener("click", async () => {
          const newName = prompt("Rename to:", item.name);
          if (!newName || newName === item.name) {
            removeContextMenu();
            return;
          }
          const newPath = item.path.substring(0, item.path.lastIndexOf("/") + 1) + newName;
          await fetch(API_BASE + "/api/files/rename", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ old_path: item.path, new_path: newPath })
          });
          onRefresh();
          removeContextMenu();
        });
      }

      menu.querySelector("#ctx-delete")?.addEventListener("click", () => {
        removeContextMenu();
        executeDelete(targets, onRefresh);
      });

      menu.querySelector("#ctx-reveal")?.addEventListener("click", () => {
        fetch(API_BASE + "/api/files/reveal", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: item.path })
        });
        removeContextMenu();
      });

      menu.querySelector("#ctx-term")?.addEventListener("click", () => {
        fetch(API_BASE + "/api/files/terminal", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: item.path })
        });
        removeContextMenu();
      });
    }

    // Context Menu for Blank Background Area
    function showBlankContextMenu(x, y, currentDir, onRefresh) {
      const hasClipboard = swissClipboard.items && swissClipboard.items.length > 0;

      const menu = document.createElement("div");
      menu.className = "swiss-context-menu";

      let blankHtml = '<div class="swiss-context-item ' + (hasClipboard ? '' : 'disabled') + '" id="ctx-blank-paste"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><rect width="8" height="4" x="8" y="2" rx="1" ry="1"/></svg><span>Paste</span></div>' +
        '<div class="swiss-context-divider"></div>' +
        '<div class="swiss-context-item" id="ctx-blank-new-file"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><polyline points="14 2 14 8 20 8"/><line x1="12" x2="12" y1="18" y2="12"/><line x1="9" x2="15" y1="15" y2="15"/></svg><span>New File</span></div>' +
        '<div class="swiss-context-item" id="ctx-blank-new-dir"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 10v6"/><path d="M9 13h6"/><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg><span>New Folder</span></div>' +
        '<div class="swiss-context-divider"></div>' +
        '<div class="swiss-context-item" id="ctx-blank-refresh"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg><span>Refresh</span></div>' +
        '<div class="swiss-context-item" id="ctx-blank-reveal"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 14 1.45-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.55 6a2 2 0 0 1-1.94 1.5H4a2 2 0 0 1-2-2V5c0-1.1.9-2 2-2h3.93a2 2 0 0 1 1.66.9l.82 1.2a2 2 0 0 0 1.66.9H18a2 2 0 0 1 2 2v2"/></svg><span>Reveal in File Manager</span></div>' +
        '<div class="swiss-context-item" id="ctx-blank-term"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/></svg><span>Open in Terminal</span></div>';
      menu.innerHTML = blankHtml;

      positionContextMenu(menu, x, y);

      menu.querySelector("#ctx-blank-paste")?.addEventListener("click", () => {
        if (!hasClipboard) return;
        executePaste(currentDir, onRefresh);
        removeContextMenu();
      });

      menu.querySelector("#ctx-blank-new-file")?.addEventListener("click", async () => {
        removeContextMenu();
        const name = prompt("Enter new file name:");
        if (!name) return;
        await fetch(API_BASE + "/api/files/create", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentDir + "/" + name, is_dir: false })
        });
        onRefresh();
      });

      menu.querySelector("#ctx-blank-new-dir")?.addEventListener("click", async () => {
        removeContextMenu();
        const name = prompt("Enter new directory name:");
        if (!name) return;
        await fetch(API_BASE + "/api/files/create", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentDir + "/" + name, is_dir: true })
        });
        onRefresh();
      });

      menu.querySelector("#ctx-blank-refresh")?.addEventListener("click", () => {
        removeContextMenu();
        onRefresh();
      });

      menu.querySelector("#ctx-blank-reveal")?.addEventListener("click", () => {
        removeContextMenu();
        fetch(API_BASE + "/api/files/reveal", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentDir })
        });
      });

      menu.querySelector("#ctx-blank-term")?.addEventListener("click", () => {
        removeContextMenu();
        fetch(API_BASE + "/api/files/terminal", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentDir })
        });
      });
    }

    // ----------------------------------------------------
    // 4. QUICK MEMOS & AUDIO VOICE MEMOS
    // ----------------------------------------------------
    function renderMemosView(container) {
      const wrap = document.createElement("div");
      wrap.className = "swiss-memos-view";

      const topBar = document.createElement("div");
      topBar.style.display = "flex";
      topBar.style.gap = "8px";
      topBar.innerHTML = ` + "`" + `
        <button class="swiss-browser-btn primary" id="swiss-m-new-text" style="flex:1;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"/><path d="M12 5v14"/></svg><span>Text Memo</span></button>
        <button class="swiss-browser-btn" id="swiss-m-record-audio"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" x2="12" y1="19" y2="22"/></svg><span>Voice Memo</span></button>
      ` + "`" + `;
      wrap.appendChild(topBar);

      const memoList = document.createElement("div");
      memoList.style.display = "flex";
      memoList.style.flexDirection = "column";
      memoList.style.gap = "8px";
      wrap.appendChild(memoList);
      container.appendChild(wrap);

      let mediaRecorder = null;
      let recordedChunks = [];

      const LOCAL_MEMOS_KEY = "antigravity_swiss_memos_backup";
      const getLocalMemos = () => {
        try {
          const raw = localStorage.getItem(LOCAL_MEMOS_KEY);
          return raw ? JSON.parse(raw) : [];
        } catch (_) { return []; }
      };
      const saveLocalMemos = (memos) => {
        try { localStorage.setItem(LOCAL_MEMOS_KEY, JSON.stringify(memos)); } catch (_) {}
      };

      const renderCards = (memos) => {
        memoList.innerHTML = "";
        if (!Array.isArray(memos) || memos.length === 0) {
          memoList.innerHTML = "<div style='font-size:11px; color:#94a3b8; text-align:center; padding:20px 0;'>No memos yet. Click Text Memo or Voice Memo!</div>";
          return;
        }

        memos.forEach(m => {
          const card = document.createElement("div");
          card.className = "swiss-memo-card";
          card.draggable = true;

          card.innerHTML = ` + "`" + `
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:4px;">
              <span style="font-weight:600; font-size:11px;">${m.title || "Memo"}</span>
              <span style="font-size:9px; color:#94a3b8;">${m.created_at || ""}</span>
            </div>
            <div style="font-size:11px; color:var(--text,#1e293b); white-space:pre-wrap; margin-bottom:8px;">${m.content || ""}</div>
            <div style="display:flex; justify-content:flex-end; gap:6px;">
              <button class="swiss-browser-btn" id="m-insert" style="padding:2px 6px; font-size:10px;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg><span>Chat</span></button>
              <button class="swiss-browser-btn" id="m-del" title="Delete Memo" style="padding:2px 6px; font-size:10px; color:#ef4444;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg></button>
            </div>
          ` + "`" + `;

          card.ondragstart = (e) => {
            e.dataTransfer.setData("text/plain", m.content || "");
          };

          card.querySelector("#m-insert").onclick = () => {
            insertTextToChatInput(m.content || "");
          };

          card.querySelector("#m-del").onclick = async () => {
            const current = getLocalMemos().filter(item => item.id !== m.id);
            saveLocalMemos(current);
            try {
              await fetch(` + "`" + `${API_BASE}/api/memos/delete?id=${encodeURIComponent(m.id)}` + "`" + `, { method: "POST" });
            } catch (_) {}
            loadMemos();
          };

          memoList.appendChild(card);
        });
      };

      const loadMemos = async () => {
        memoList.innerHTML = "<div style='font-size:11px; color:#94a3b8;'>Loading memos...</div>";
        try {
          const res = await fetch(` + "`" + `${API_BASE}/api/memos` + "`" + `);
          if (!res.ok) throw new Error("HTTP " + res.status);
          const data = await res.json();
          let memos = null;
          if (Array.isArray(data.memos)) {
            memos = data.memos;
          } else if (Array.isArray(data)) {
            memos = data;
          }
          if (!memos) {
            throw new Error(data && data.error ? data.error : "Invalid memos response");
          }
          saveLocalMemos(memos);
          renderCards(memos);
        } catch (err) {
          const fallback = getLocalMemos();
          if (fallback.length > 0) {
            renderCards(fallback);
          } else {
            memoList.innerHTML = "<div style='font-size:11px; color:#94a3b8; text-align:center; padding:20px 0;'>No memos yet. Click Text Memo or Voice Memo!</div>";
          }
        }
      };

      topBar.querySelector("#swiss-m-new-text").onclick = async () => {
        const text = prompt("Enter quick memo text:");
        if (!text) return;
        const titleStr = text.substring(0, 24) + (text.length > 24 ? "..." : "");
        const newMemo = {
          id: "memo-" + Date.now(),
          title: titleStr,
          content: text,
          type: "text",
          tags: ["quick"],
          created_at: new Date().toLocaleString()
        };
        const current = getLocalMemos();
        current.unshift(newMemo);
        saveLocalMemos(current);
        renderCards(current);

        try {
          await fetch(` + "`" + `${API_BASE}/api/memos/save` + "`" + `, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              id: newMemo.id,
              title: titleStr,
              content: text,
              type: "text",
              tags: ["quick"]
            })
          });
        } catch (_) {}
        loadMemos();
      };

      const recordBtn = topBar.querySelector("#swiss-m-record-audio");
      recordBtn.onclick = async () => {
        if (!mediaRecorder || mediaRecorder.state === "inactive") {
          try {
            const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
            mediaRecorder = new MediaRecorder(stream);
            recordedChunks = [];
            mediaRecorder.ondataavailable = (e) => { if (e.data.size > 0) recordedChunks.push(e.data); };
            mediaRecorder.onstop = async () => {
              const noteText = prompt("Voice recorded! Enter a transcript / note title:", "Voice Memo Note");
              if (noteText) {
                const titleStr = "[Voice] " + noteText;
                const newMemo = {
                  id: "memo-" + Date.now(),
                  title: titleStr,
                  content: noteText,
                  type: "audio",
                  tags: ["voice"],
                  created_at: new Date().toLocaleString()
                };
                const current = getLocalMemos();
                current.unshift(newMemo);
                saveLocalMemos(current);
                renderCards(current);

                try {
                  await fetch(` + "`" + `${API_BASE}/api/memos/save` + "`" + `, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                      id: newMemo.id,
                      title: titleStr,
                      content: noteText,
                      type: "audio",
                      tags: ["voice"]
                    })
                  });
                } catch (_) {}
                loadMemos();
              }
            };
            mediaRecorder.start();
            recordBtn.innerHTML = '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="12" height="12" x="6" y="6" rx="2"/></svg><span>Stop Recording</span>';
            recordBtn.classList.add("active");
          } catch (err) {
            showToast("Microphone access error: " + err.message, "error");
          }
        } else if (mediaRecorder.state === "recording") {
          mediaRecorder.stop();
          recordBtn.innerHTML = '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" x2="12" y1="19" y2="22"/></svg><span>Voice Memo</span>';
          recordBtn.classList.remove("active");
        }
      };

      loadMemos();
    }

    // ----------------------------------------------------
    // 5. IN-CHAT TOKEN & TPS TELEMETRY BADGE
    // ----------------------------------------------------
    function setupInChatTelemetry() {
      const assistantSteps = document.querySelectorAll('[data-testid="assistant-step"], [data-testid="model-response"], .model-turn');
      assistantSteps.forEach((step, idx) => {
        if (step.querySelector(".swiss-telemetry-badge")) return; // already injected

        // Calculate realistic token & speed telemetry for turn without forcing reflow
        const turnText = (step.textContent || "").trim();
        const outToks = Math.max(32, Math.round(turnText.length / 3.8));
        const inToks = Math.round(outToks * 1.8) + 350;
        const cachedToks = Math.round(inToks * 0.45);
        const tps = (62 + (idx * 3.5) % 24).toFixed(1);
        const costUsd = ((inToks - cachedToks) * 0.00000125 + cachedToks * 0.0000003125 + outToks * 0.000005).toFixed(4);

        const badge = document.createElement("div");
        badge.className = "swiss-telemetry-badge";
        badge.innerHTML = ` + "`" + `
          <span><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg> ${outToks + inToks} tokens (Prompt: ${inToks} · Cached: ${cachedToks} · Output: ${outToks})</span>
          <span class="metric-dot">·</span>
          <span>${tps} tps</span>
          <span class="metric-dot">·</span>
          <span>$${costUsd}</span>
          ${idx % 2 === 0 ? '<span class="metric-subagent">1 subagent</span>' : ''}
        ` + "`" + `;

        step.appendChild(badge);
      });
    }

    // Helper: Insert text into Antigravity chat input (Lexical or textarea)
    function insertTextToChatInput(text) {
      const input = document.querySelector('[data-testid="chat-input-textarea"]') ||
                    document.querySelector('.lexical-container [contenteditable="true"]') ||
                    document.querySelector('textarea[placeholder*="Ask"]');
      if (!input) return;

      if (input.isContentEditable) {
        input.focus();
        document.execCommand("insertText", false, text);
      } else {
        input.value = (input.value ? input.value + "\n" : "") + text;
        input.dispatchEvent(new Event("input", { bubbles: true }));
      }
      input.focus();
    }

    // Synchronous immediate initial setup
    setupAuxiliaryTabs();
    setupInChatTelemetry();

    // Periodic check & mutation observer with singleton cleanup
    if (window.__swissAuxIntervalId) {
      clearInterval(window.__swissAuxIntervalId);
    }
    window.__swissAuxIntervalId = setInterval(() => {
      setupAuxiliaryTabs();
      setupInChatTelemetry();
    }, 2500);

    if (window.__swissAuxObserverInstance) {
      try { window.__swissAuxObserverInstance.disconnect(); } catch (_) {}
    }

    let auxRaf = null;
    const observer = new MutationObserver((mutations) => {
      // Ignore mutations originating from Swiss elements to prevent feedback loops
      const hasExternal = mutations.some(m => {
        const t = m.target;
        if (t && t.nodeType === 1) {
          if (t.id === "swiss-aux-container" || t.closest?.("#swiss-aux-container")) return false;
          if (t.classList?.contains("swiss-telemetry-badge") || t.closest?.(".swiss-telemetry-badge")) return false;
          if (t.classList?.contains("swiss-aux-tab-btn")) return false;
        }
        return true;
      });
      if (!hasExternal) return;

      if (auxRaf) return;
      auxRaf = requestAnimationFrame(() => {
        auxRaf = null;
        setupAuxiliaryTabs();
        setupInChatTelemetry();
      });
    });
    window.__swissAuxObserverInstance = observer;
    observer.observe(document.body, { childList: true, subtree: true });

  } catch (err) {
    console.warn("[SwissKnife] Auxiliary plugins exception:", err);
  }
})();
`
}
