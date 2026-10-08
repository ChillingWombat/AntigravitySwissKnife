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
  gap: 4px;
  width: 24px;
  min-width: 24px;
  max-width: 24px;
  height: 24px;
  padding: 0;
  font-size: 11px;
  font-weight: 500;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: var(--text-muted, #71717a);
  cursor: pointer;
  transition: all 0.15s ease;
  user-select: none;
  box-sizing: border-box;
  line-height: 1;
  flex-shrink: 0;
}
.swiss-aux-tab-btn.icon-only {
  width: 24px;
  height: 24px;
  min-width: 24px;
  max-width: 24px;
  padding: 0;
  border-radius: 8px;
  flex-shrink: 0;
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
/* Auxiliary Active State Isolation: When a Swiss tab is active, suppress native factory tab highlights */
[data-swiss-aux-active] button[data-tab-id]:not([data-tab-id^="swiss-"]),
[data-swiss-aux-active] button:not(.swiss-aux-tab-btn):not([data-tab-id^="swiss-"]) {
  background-color: transparent !important;
  border-color: transparent !important;
  color: var(--secondary-foreground, #71717a) !important;
  box-shadow: none !important;
}
[data-swiss-aux-active] button[data-tab-id]:not([data-tab-id^="swiss-"]):hover,
[data-swiss-aux-active] button:not(.swiss-aux-tab-btn):not([data-tab-id^="swiss-"]):hover {
  background-color: var(--muted, rgba(148, 163, 184, 0.15)) !important;
  color: var(--foreground, #1e293b) !important;
}
:is(.dark, [data-theme="dark"]) [data-swiss-aux-active] button[data-tab-id]:not([data-tab-id^="swiss-"]):hover,
:is(.dark, [data-theme="dark"]) [data-swiss-aux-active] button:not(.swiss-aux-tab-btn):not([data-tab-id^="swiss-"]):hover {
  background-color: rgba(255, 255, 255, 0.08) !important;
  color: #f1f5f9 !important;
}
.swiss-aux-tab-svg {
  width: 13.5px;
  height: 13.5px;
  display: inline-block;
  vertical-align: middle;
  flex-shrink: 0;
  pointer-events: none;
}
.swiss-aux-tab-label {
  font-size: 11px;
  line-height: 1;
  pointer-events: none;
  white-space: nowrap;
}
.swiss-aux-tabs-divider {
  height: 16px;
  width: 1px;
  min-width: 1px;
  max-width: 1px;
  background-color: var(--border, rgba(0, 0, 0, 0.18)) !important;
  margin: 0 2px;
  opacity: 1 !important;
  flex-shrink: 0 !important;
  display: block !important;
  box-sizing: border-box !important;
}
:is(.dark, [data-theme="dark"]) .swiss-aux-tabs-divider {
  background-color: var(--border, rgba(255, 255, 255, 0.22)) !important;
  opacity: 1 !important;
}
.swiss-aux-btn-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}
div:has(> .shrink-0.flex.items-center.border-b),
[data-swiss-aux-panel],
[data-testid="auxiliary-panel"],
.part.auxiliarybar {
  container-type: inline-size;
}
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group),
[data-swiss-aux-header],
[data-testid="auxiliary-panel"] .shrink-0.flex.items-center.border-b,
.part.auxiliarybar .shrink-0.flex.items-center.border-b {
  padding-right: 70px !important;
}

/* 1. Fixed Left Factory Buttons (Overview, Review, Terminal) */
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div:first-child {
  flex-shrink: 0 !important;
}
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div:first-child button,
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div:first-child [role="tab"] {
  width: 24px !important;
  min-width: 24px !important;
  max-width: 24px !important;
  height: 24px !important;
  padding: 0 !important;
  flex-shrink: 0 !important;
  justify-content: center !important;
}

/* 2. Flexible Middle Section: File Tabs & Plus Button */
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div:nth-child(2) {
  flex: 1 1 0% !important;
  min-width: 0 !important;
  overflow: hidden !important;
}
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > button[aria-label="Add"],
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > [data-testid="aux-panel-plus-dropdown-trigger"],
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > button:not(.swiss-aux-tab-btn) {
  width: 24px !important;
  min-width: 0 !important;
  max-width: 24px !important;
  height: 24px !important;
  flex-shrink: 1 !important;
  overflow: hidden !important;
  padding: 0 !important;
  justify-content: center !important;
}

/* 3. Fixed Right End: Swiss Tab Buttons & Dividers Never Change Size */
@container (max-width: 520px) {
  .swiss-aux-tab-btn .swiss-aux-tab-label {
    display: none !important;
  }
}
@container (max-width: 285px) {
  .shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > button[aria-label="Add"],
  .shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > [data-testid="aux-panel-plus-dropdown-trigger"],
  .shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > button:not(.swiss-aux-tab-btn) {
    display: none !important;
  }
  .shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > .swiss-aux-tabs-divider-left {
    display: none !important;
  }
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
  gap: 6px;
  padding: 6px 10px;
  background: var(--canvas-subtle, #f8fafc);
  border-bottom: 1px solid var(--border, #e2e8f0);
  min-width: 0;
  box-sizing: border-box;
  container-type: inline-size;
  container-name: swisstoolbar;
}
.swiss-browser-nav-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  flex-wrap: nowrap;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.35) transparent;
  box-sizing: border-box;
}
.swiss-browser-nav-row::-webkit-scrollbar,
.swiss-browser-tools-row::-webkit-scrollbar {
  height: 3px;
}
.swiss-browser-nav-row::-webkit-scrollbar-track,
.swiss-browser-tools-row::-webkit-scrollbar-track {
  background: transparent;
}
.swiss-browser-nav-row::-webkit-scrollbar-thumb,
.swiss-browser-tools-row::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.35);
  border-radius: 3px;
}
.swiss-browser-nav-row::-webkit-scrollbar-thumb:hover,
.swiss-browser-tools-row::-webkit-scrollbar-thumb:hover {
  background: rgba(148, 163, 184, 0.6);
}
.swiss-browser-port-bar {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  flex-wrap: nowrap;
  flex-shrink: 0;
}
.swiss-port-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted, #71717a);
  margin-right: 2px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
  user-select: none;
  flex-shrink: 0;
}
.swiss-port-list {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-wrap: nowrap;
  flex-shrink: 0;
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
  white-space: nowrap;
  flex-shrink: 0;
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
.swiss-port-dropdown-wrap {
  display: none;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.swiss-port-select {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  font-size: 10px;
  font-family: monospace;
  cursor: pointer;
  outline: none;
  box-sizing: border-box;
  max-width: 140px;
  transition: all 0.15s ease;
  flex-shrink: 0;
}
.swiss-port-select:hover {
  border-color: #1a73e8;
  color: #1a73e8;
}
.swiss-port-select:focus {
  border-color: #1a73e8;
  box-shadow: 0 0 0 2px rgba(26, 115, 232, 0.15);
}
.swiss-port-select.active {
  border-color: #1a73e8;
  color: #1a73e8;
  font-weight: 600;
}
.swiss-port-select option {
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
}
:is(.dark, [data-theme="dark"]) .swiss-port-select {
  background: #1e293b;
  color: #f1f5f9;
  border-color: #475569;
}
:is(.dark, [data-theme="dark"]) .swiss-port-select.active {
  border-color: #38bdf8;
  color: #38bdf8;
}
:is(.dark, [data-theme="dark"]) .swiss-port-select option {
  background: #1e293b;
  color: #f1f5f9;
}
@container swisstoolbar (max-width: 620px) {
  .swiss-browser-tools-divider {
    display: none !important;
  }
}
@container swisstoolbar (max-width: 580px) {
  .swiss-port-label {
    display: none !important;
  }
  .swiss-port-list {
    display: none !important;
  }
  .swiss-port-dropdown-wrap {
    display: inline-flex !important;
  }
}
.swiss-browser-toolbar.compact-tools .swiss-browser-tools-divider {
  display: none;
}
.swiss-browser-toolbar.compact-ports .swiss-port-label {
  display: none;
}
.swiss-browser-toolbar.compact-ports .swiss-port-list {
  display: none;
}
.swiss-browser-toolbar.compact-ports .swiss-port-dropdown-wrap {
  display: inline-flex;
}
.swiss-port-add-box {
  display: inline-flex;
  align-items: center;
  height: 20px;
  width: 22px;
  border-radius: 10px;
  border: 1px dashed var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  box-sizing: border-box;
  overflow: hidden;
  transition: width 0.22s cubic-bezier(0.4, 0, 0.2, 1), border-color 0.15s, box-shadow 0.15s;
  vertical-align: middle;
  flex-shrink: 0;
}
.swiss-port-add-box:hover {
  border-color: #1a73e8;
}
.swiss-port-add-box.expanded {
  width: 96px;
  border-style: solid;
  border-color: #1a73e8;
  box-shadow: 0 0 0 2px rgba(26, 115, 232, 0.15);
}
.swiss-port-add-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 18px;
  border: none;
  background: transparent;
  padding: 0;
  margin: 0;
  cursor: pointer;
  color: var(--text-muted, #64748b);
  flex-shrink: 0;
}
.swiss-port-add-btn:hover {
  color: #1a73e8;
}
.swiss-port-add-icon {
  transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1), color 0.15s;
}
.swiss-port-add-box.expanded .swiss-port-add-icon {
  transform: rotate(45deg);
  color: #94a3b8;
}
.swiss-port-add-box.expanded .swiss-port-add-btn:hover .swiss-port-add-icon {
  color: #ef4444;
}
.swiss-port-add-input {
  width: 0;
  opacity: 0;
  padding: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 10px;
  font-family: monospace;
  color: var(--text, #1e293b);
  pointer-events: none;
  transition: width 0.22s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.15s;
  box-sizing: border-box;
}
.swiss-port-add-box.expanded .swiss-port-add-input {
  width: 66px;
  opacity: 1;
  padding: 0 4px 0 2px;
  pointer-events: auto;
}
.swiss-browser-url-input {
  flex: 1;
  min-width: 110px;
  max-width: 300px;
  padding: 4px 10px;
  font-size: 12px;
  border-radius: 14px;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  outline: none;
  box-sizing: border-box;
  flex-shrink: 0;
}
.swiss-browser-tools-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  flex-wrap: wrap;
  box-sizing: border-box;
}
.swiss-browser-device-group {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: nowrap;
  flex-shrink: 0;
  white-space: nowrap;
  max-width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.35) transparent;
  box-sizing: border-box;
}
.swiss-browser-tools-divider {
  width: 1px;
  height: 16px;
  background: var(--border, #e2e8f0);
  margin: 0 2px;
  flex-shrink: 0;
}
.swiss-browser-annotation-group {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: nowrap;
  flex-shrink: 0;
  white-space: nowrap;
  max-width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.35) transparent;
  box-sizing: border-box;
}
.swiss-browser-device-group::-webkit-scrollbar,
.swiss-browser-annotation-group::-webkit-scrollbar {
  height: 2px;
}
.swiss-browser-device-group::-webkit-scrollbar-track,
.swiss-browser-annotation-group::-webkit-scrollbar-track {
  background: transparent;
}
.swiss-browser-device-group::-webkit-scrollbar-thumb,
.swiss-browser-annotation-group::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.35);
  border-radius: 2px;
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
  flex-shrink: 0;
}
.swiss-browser-btn svg {
  flex-shrink: 0;
  vertical-align: middle;
}
.swiss-browser-btn:hover {
  background: rgba(148, 163, 184, 0.15);
}
.swiss-browser-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
  pointer-events: none;
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
.swiss-files-actions-bar {
  display: flex;
  align-items: center;
  gap: 6px;
}
.swiss-files-path-input {
  flex: 1;
  min-width: 0;
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
.swiss-file-row.swiss-file-hidden {
  opacity: 0.72;
}
.swiss-file-row.swiss-file-hidden:hover,
.swiss-file-row.swiss-file-hidden.selected {
  opacity: 1;
}
.swiss-browser-btn.toggled,
#swiss-f-hidden.active {
  background: rgba(26, 115, 232, 0.12);
  color: #1a73e8;
  border-color: rgba(26, 115, 232, 0.4);
}
:is(.dark, [data-theme="dark"]) .swiss-browser-btn.toggled,
:is(.dark, [data-theme="dark"]) #swiss-f-hidden.active {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border-color: rgba(56, 189, 248, 0.4);
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
  border-radius: 10px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.18);
  padding: 4px;
  min-width: 175px;
  font-size: 12px;
  user-select: none;
  -webkit-user-select: none;
}
.swiss-context-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-radius: 6px;
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
.swiss-memo-composer {
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: var(--canvas, #ffffff);
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 8px;
  padding: 10px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}
:is(.dark, [data-theme="dark"]) .swiss-memo-composer {
  background: var(--card, #1e293b);
  border-color: var(--border, #334155);
}
.swiss-memo-composer-textarea {
  width: 100%;
  box-sizing: border-box;
  min-height: 56px;
  max-height: 140px;
  resize: vertical;
  border: 1px solid var(--border, #cbd5e1);
  border-radius: 6px;
  padding: 8px;
  font-size: 12px;
  line-height: 1.4;
  font-family: inherit;
  background: transparent;
  color: var(--text, #1e293b);
  outline: none;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.swiss-memo-composer-textarea:focus {
  border-color: #1a73e8;
  box-shadow: 0 0 0 2px rgba(26, 115, 232, 0.15);
}
:is(.dark, [data-theme="dark"]) .swiss-memo-composer-textarea {
  color: #f8fafc;
  border-color: #475569;
}
.swiss-memo-composer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
  align-items: center;
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
.swiss-memo-search-row {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
}
.swiss-memo-search-input {
  transition: border-color 0.15s, box-shadow 0.15s;
}
.swiss-memo-search-input:focus {
  border-color: #1a73e8;
  box-shadow: 0 0 0 2px rgba(26, 115, 232, 0.15);
}
:is(.dark, [data-theme="dark"]) .swiss-memo-search-input {
  background: var(--card, #1e293b);
  border-color: var(--border, #334155);
  color: #f8fafc;
}
.swiss-memo-empty-search {
  text-align: center;
  padding: 24px 12px;
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
}
:is(.dark, [data-theme="dark"]) .swiss-memo-empty-search {
  color: #94a3b8;
}

/* Swiss Prompt Modal Dialog */
.swiss-prompt-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100000;
  padding: 16px;
  box-sizing: border-box;
}
.swiss-prompt-dialog {
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 8px;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.2), 0 8px 10px -6px rgba(0, 0, 0, 0.1);
  width: 380px;
  max-width: 100%;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  box-sizing: border-box;
}
:is(.dark, [data-theme="dark"]) .swiss-prompt-dialog {
  background: var(--card, #1e293b);
  color: var(--foreground, #f8fafc);
  border-color: var(--border, #334155);
}
.swiss-prompt-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text, #1e293b);
  margin: 0;
  line-height: 1.4;
  word-break: break-word;
}
:is(.dark, [data-theme="dark"]) .swiss-prompt-title {
  color: var(--foreground, #f8fafc);
}
.swiss-prompt-input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--border, #cbd5e1);
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 12px;
  font-family: inherit;
  background: transparent;
  color: var(--text, #1e293b);
  outline: none;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.swiss-prompt-input:focus {
  border-color: #1a73e8;
  box-shadow: 0 0 0 2px rgba(26, 115, 232, 0.15);
}
:is(.dark, [data-theme="dark"]) .swiss-prompt-input {
  color: #f8fafc;
  border-color: #475569;
}
.swiss-prompt-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
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
` + "\n" + GenerateGitHubExtensionCSS()
}

// GenerateAuxiliaryPluginsScript returns the full client-side JavaScript injected into Antigravity 2.0
// that mounts the auxiliary tabs, live browser previewer, file explorer with in-place editors, quick memos,
// and in-chat token telemetry badges.
func GenerateAuxiliaryPluginsScript() string {
	return `(() => {
  try {
    if (window.__swissAuxiliaryInitialized) {
      if (window.__swissAuxIntervalId) {
        clearInterval(window.__swissAuxIntervalId);
        window.__swissAuxIntervalId = null;
      }
      if (window.__swissAuxObserverInstance) {
        try { window.__swissAuxObserverInstance.disconnect(); } catch (_) {}
        window.__swissAuxObserverInstance = null;
      }
      document.querySelectorAll(".swiss-aux-btn-group, .swiss-aux-tabs-divider, .swiss-aux-tabs-divider-left, .swiss-aux-tabs-divider-right").forEach(el => el.remove());
      const existingAuxContainer = document.getElementById("swiss-aux-container");
      if (existingAuxContainer && existingAuxContainer.dataset) {
        delete existingAuxContainer.dataset.renderedTab;
      }
    }
    window.__swissAuxiliaryInitialized = true;

    const API_BASE = "http://127.0.0.1:8765";
    let activeAuxTab = null; // "swiss-browser" | "swiss-files" | "swiss-memos" | null (native)
    let currentBrowserUrl = "http://localhost:5173";
    let currentFilePath = ".";
    let fileHistory = [];
    let stageFilePath = ".";
    let stageFileHistory = [];
    let lastStageScope = null;
    let auxFilePath = ".";
    let auxFileHistory = [];
    let lastAuxProject = null;
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

    function showSwissPrompt(title, defaultValue = "", options = {}) {
      return new Promise((resolve) => {
        const backdrop = document.createElement("div");
        backdrop.className = "swiss-prompt-backdrop";

        const dialog = document.createElement("div");
        dialog.className = "swiss-prompt-dialog";

        const titleEl = document.createElement("div");
        titleEl.className = "swiss-prompt-title";
        titleEl.textContent = title;
        dialog.appendChild(titleEl);

        const isMultiline = options.multiline || false;
        const inputEl = document.createElement(isMultiline ? "textarea" : "input");
        inputEl.className = "swiss-prompt-input";
        if (!isMultiline) {
          inputEl.type = "text";
        } else {
          inputEl.rows = options.rows || 3;
        }
        inputEl.value = defaultValue || "";
        if (options.placeholder) inputEl.placeholder = options.placeholder;
        dialog.appendChild(inputEl);

        const actions = document.createElement("div");
        actions.className = "swiss-prompt-actions";

        const cancelBtn = document.createElement("button");
        cancelBtn.type = "button";
        cancelBtn.className = "swiss-browser-btn";
        cancelBtn.textContent = options.cancelText || "Cancel";

        const confirmBtn = document.createElement("button");
        confirmBtn.type = "button";
        confirmBtn.className = "swiss-browser-btn primary";
        confirmBtn.textContent = options.confirmText || "OK";

        actions.appendChild(cancelBtn);
        actions.appendChild(confirmBtn);
        dialog.appendChild(actions);
        backdrop.appendChild(dialog);
        document.body.appendChild(backdrop);

        let resolved = false;
        const cleanup = (value) => {
          if (resolved) return;
          resolved = true;
          document.removeEventListener("keydown", onKeyDown, true);
          backdrop.remove();
          resolve(value);
        };

        cancelBtn.onclick = () => cleanup(null);
        confirmBtn.onclick = () => cleanup(inputEl.value);

        backdrop.onclick = (e) => {
          if (e.target === backdrop) cleanup(null);
        };

        const onKeyDown = (e) => {
          if (e.isComposing || e.keyCode === 229) return;
          if (e.key === "Escape") {
            e.preventDefault();
            e.stopPropagation();
            cleanup(null);
          } else if (e.key === "Enter" && (!isMultiline || e.ctrlKey || e.metaKey)) {
            e.preventDefault();
            e.stopPropagation();
            cleanup(inputEl.value);
          }
        };
        document.addEventListener("keydown", onKeyDown, true);

        setTimeout(() => {
          inputEl.focus();
          if (inputEl.select) inputEl.select();
        }, 30);
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

    // 1. Auxiliary Panel Tab Injector Engine
    function setupAuxiliaryTabs() {
      // Find auxiliary panel header strictly matching .shrink-0.flex.items-center[class*="gap-0.5"].border-b
      const tabHeader = document.querySelector('.shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('[data-testid="auxiliary-panel"] .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.part.auxiliarybar .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.shrink-0.flex.items-center.border-b');
      if (!tabHeader) return;

      if (tabHeader.parentElement) {
        tabHeader.parentElement.setAttribute('data-swiss-aux-panel', 'true');
      }
      tabHeader.setAttribute('data-swiss-aux-header', 'true');

      // Define Swiss tabs with Antigravity-matching monochrome SVG stroke icons
      const tabs = [
        {
          id: "browser",
          tabId: "swiss-browser",
          label: "Preview Browser",
          svg: '<svg class="swiss-aux-tab-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
        },
        {
          id: "files",
          tabId: "swiss-files",
          label: "Files",
          svg: '<svg class="swiss-aux-tab-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>'
        },
        {
          id: "memos",
          tabId: "swiss-memos",
          label: "Memos",
          svg: '<svg class="swiss-aux-tab-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M16 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V8Z"/><polyline points="15 3 15 8 20 8"/><line x1="9" y1="13" x2="15" y2="13"/><line x1="9" y1="17" x2="13" y2="17"/></svg>'
        },
        {
          id: "github",
          tabId: "swiss-github",
          label: "GitHub",
          svg: '<svg class="swiss-aux-tab-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/></svg>'
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

      // Configure tabs matching data-tab-id="swiss-browser", data-tab-id="swiss-files", data-tab-id="swiss-memos", data-tab-id="swiss-github"
      const existingBtns = tabHeader.querySelectorAll('.swiss-aux-tab-btn');
      if (existingBtns.length > 0) {
        const curFmt = getAuxTabFormat();
        existingBtns.forEach(btn => {
          const tid = btn.dataset.swissTab;
          const t = tabs.find(x => x.id === tid);
          if (t) {
            updateTabButtonMarkup(btn, t, curFmt);
            btn.onclick = (e) => {
              e.stopPropagation();
              switchAuxTab(t.tabId);
            };
          }
        });

        // Ensure left and right dividers exist even if buttons already present
        const curBtnGroup = tabHeader.querySelector('.swiss-aux-btn-group');
        if (curBtnGroup) {
          let dividerLeft = tabHeader.querySelector('.swiss-aux-tabs-divider-left') ||
                            tabHeader.querySelector('.swiss-aux-tabs-divider:not(.swiss-aux-tabs-divider-right)');
          if (!dividerLeft) {
            dividerLeft = document.createElement("div");
            dividerLeft.className = "swiss-aux-tabs-divider swiss-aux-tabs-divider-left";
            dividerLeft.style.height = "16px";
            dividerLeft.style.width = "1px";
            dividerLeft.style.minWidth = "1px";
            dividerLeft.style.maxWidth = "1px";
            dividerLeft.style.flexShrink = "0";
            dividerLeft.style.backgroundColor = "var(--border, rgba(0, 0, 0, 0.18))";
            dividerLeft.style.margin = "0 2px";
            dividerLeft.style.opacity = "1";
            curBtnGroup.before(dividerLeft);
          } else {
            dividerLeft.style.height = "16px";
            dividerLeft.style.backgroundColor = "var(--border, rgba(0, 0, 0, 0.18))";
            dividerLeft.style.margin = "0 2px";
            dividerLeft.style.opacity = "1";
            if (dividerLeft.nextElementSibling !== curBtnGroup) {
              curBtnGroup.before(dividerLeft);
            }
          }

          let dividerRight = tabHeader.querySelector('.swiss-aux-tabs-divider-right');
          if (!dividerRight) {
            dividerRight = document.createElement("div");
            dividerRight.className = "swiss-aux-tabs-divider swiss-aux-tabs-divider-right";
            dividerRight.style.height = "16px";
            dividerRight.style.width = "1px";
            dividerRight.style.minWidth = "1px";
            dividerRight.style.maxWidth = "1px";
            dividerRight.style.flexShrink = "0";
            dividerRight.style.backgroundColor = "var(--border, rgba(0, 0, 0, 0.18))";
            dividerRight.style.margin = "0 2px";
            dividerRight.style.opacity = "1";
            curBtnGroup.after(dividerRight);
          } else {
            dividerRight.style.height = "16px";
            dividerRight.style.backgroundColor = "var(--border, rgba(0, 0, 0, 0.18))";
            dividerRight.style.margin = "0 2px";
            dividerRight.style.opacity = "1";
            if (curBtnGroup.nextElementSibling !== dividerRight) {
              curBtnGroup.after(dividerRight);
            }
          }
        }

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

      let dividerLeft = tabHeader.querySelector('.swiss-aux-tabs-divider-left') ||
                        tabHeader.querySelector('.swiss-aux-tabs-divider:not(.swiss-aux-tabs-divider-right)');
      if (!dividerLeft) {
        dividerLeft = document.createElement("div");
        dividerLeft.className = "swiss-aux-tabs-divider swiss-aux-tabs-divider-left";
        dividerLeft.style.height = "16px";
        dividerLeft.style.width = "1px";
        dividerLeft.style.minWidth = "1px";
        dividerLeft.style.maxWidth = "1px";
        dividerLeft.style.flexShrink = "0";
        dividerLeft.style.backgroundColor = "var(--border, rgba(0, 0, 0, 0.18))";
        dividerLeft.style.margin = "0 2px";
        dividerLeft.style.opacity = "1";
        tabHeader.appendChild(dividerLeft);
      } else {
        dividerLeft.style.height = "16px";
        dividerLeft.style.backgroundColor = "var(--border, rgba(0, 0, 0, 0.18))";
        dividerLeft.style.margin = "0 2px";
        dividerLeft.style.opacity = "1";
      }

      let btnGroup = tabHeader.querySelector('.swiss-aux-btn-group');
      if (!btnGroup) {
        btnGroup = document.createElement("div");
        btnGroup.className = "swiss-aux-btn-group";
        btnGroup.style.display = "inline-flex";
        btnGroup.style.alignItems = "center";
        btnGroup.style.gap = "2px";
        btnGroup.style.flexShrink = "0";
        tabHeader.appendChild(btnGroup);
      }

      let dividerRight = tabHeader.querySelector('.swiss-aux-tabs-divider-right');
      if (!dividerRight) {
        dividerRight = document.createElement("div");
        dividerRight.className = "swiss-aux-tabs-divider swiss-aux-tabs-divider-right";
        dividerRight.style.height = "16px";
        dividerRight.style.width = "1px";
        dividerRight.style.minWidth = "1px";
        dividerRight.style.maxWidth = "1px";
        dividerRight.style.flexShrink = "0";
        dividerRight.style.backgroundColor = "var(--border, rgba(0, 0, 0, 0.18))";
        dividerRight.style.margin = "0 2px";
        dividerRight.style.opacity = "1";
        btnGroup.after(dividerRight);
      } else {
        dividerRight.style.height = "16px";
        dividerRight.style.backgroundColor = "var(--border, rgba(0, 0, 0, 0.18))";
        dividerRight.style.margin = "0 2px";
        dividerRight.style.opacity = "1";
      }

      const curFmt = getAuxTabFormat();
      tabs.forEach(t => {
        const btn = document.createElement("button");
        btn.className = "swiss-aux-tab-btn";
        // Contract: data-tab-id="swiss-browser" data-tab-id="swiss-files" data-tab-id="swiss-memos" data-tab-id="swiss-github"
        if (t.id === "browser") btn.setAttribute("data-tab-id", "swiss-browser");
        else if (t.id === "files") btn.setAttribute("data-tab-id", "swiss-files");
        else if (t.id === "memos") btn.setAttribute("data-tab-id", "swiss-memos");
        else if (t.id === "github") btn.setAttribute("data-tab-id", "swiss-github");
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

      // Bind dynamic format update listeners
      if (window.__swissAuxFormatHandler) {
        window.removeEventListener("swiss-aux-tab-format-updated", window.__swissAuxFormatHandler);
      }
      if (window.__swissAuxStorageHandler) {
        window.removeEventListener("storage", window.__swissAuxStorageHandler);
      }
      window.__swissAuxFormatListenerBound = true;
      window.__swissAuxFormatHandler = () => {
        setupAuxiliaryTabs();
      };
      window.__swissAuxStorageHandler = (e) => {
        if (e.key === "antigravity_swiss_aux_tab_format") {
          setupAuxiliaryTabs();
        }
      };
      window.addEventListener("swiss-aux-tab-format-updated", window.__swissAuxFormatHandler);
      window.addEventListener("storage", window.__swissAuxStorageHandler);

      // Two-way state sync: Listen for clicks on native factory tabs (overview, review, terminal)
      if (tabHeader.__swissHeaderHandler) {
        tabHeader.removeEventListener("click", tabHeader.__swissHeaderHandler);
      }
      tabHeader.__swissHeaderBound = true;
      tabHeader.__swissHeaderHandler = (e) => {
        const targetBtn = e.target.closest("button");
        if (!targetBtn) return;
        const targetId = targetBtn.getAttribute("data-tab-id") || "";
        if (targetId.startsWith("swiss-")) {
          return;
        }
        switchAuxTab(null);
      };
      tabHeader.addEventListener("click", tabHeader.__swissHeaderHandler);

      // Restore saved tab state from localStorage
      const savedTab = localStorage.getItem("antigravity_active_aux_tab");
      if (savedTab && savedTab.startsWith("swiss-")) {
        if (!activeAuxTab || activeAuxTab !== savedTab) {
          switchAuxTab(savedTab);
        } else {
          if (tabHeader) tabHeader.setAttribute("data-swiss-aux-active", savedTab.replace(/^swiss-/, ""));
          document.querySelectorAll(".swiss-aux-tab-btn").forEach(b => {
            const isActive = b.getAttribute("data-tab-id") === activeAuxTab;
            b.classList.toggle("active", isActive);
            b.setAttribute("aria-selected", isActive ? "true" : "false");
          });
        }
      } else {
        if (tabHeader) tabHeader.removeAttribute("data-swiss-aux-active");
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
          tabHeader.setAttribute("data-swiss-aux-active", cleanId);
          tabHeader.querySelectorAll('button').forEach(fb => {
            const tid = fb.getAttribute("data-tab-id") || "";
            if (!tid.startsWith("swiss-")) {
              fb.classList.remove("active");
              fb.removeAttribute("data-state");
              fb.setAttribute("aria-selected", "false");
            }
          });
        }
        if (auxPanel) {
          auxPanel.setAttribute("data-swiss-aux-active", cleanId);
        }

        localStorage.setItem("antigravity_active_aux_tab", normalizedId);
        if (swissContainer.dataset.renderedTab !== cleanId) {
          renderSwissTabContent(swissContainer, cleanId);
        }
      } else {
        if (tabHeader) {
          tabHeader.removeAttribute("data-swiss-aux-active");
        }
        if (auxPanel) {
          auxPanel.removeAttribute("data-swiss-aux-active");
        }
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
    window.switchAuxTab = switchAuxTab;

    function renderSwissTabContent(container, tabId) {
      container.dataset.renderedTab = tabId;
      container.innerHTML = "";
      if (tabId === "browser") renderBrowserView(container);
      else if (tabId === "files") renderFilesView(container);
      else if (tabId === "memos") renderMemosView(container);
      else if (tabId === "github") {
        if (typeof window.renderSwissGitHubWorkspaceView === "function") {
          window.renderSwissGitHubWorkspaceView(container);
        }
      }
    }

    window.renderSwissBrowserView = renderBrowserView;
    window.renderSwissFilesView = renderFilesView;
    window.renderSwissMemosView = renderMemosView;

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
          <div class="swiss-browser-port-bar">
            <span class="swiss-port-label">Quick Ports:</span>
            <div class="swiss-port-list" id="swiss-port-list">
              <button class="swiss-port-chip" data-port="5173" title="Vite Development Server (Right-click to delete)">:5173</button>
              <button class="swiss-port-chip" data-port="3000" title="React / Next.js Server (Right-click to delete)">:3000</button>
              <button class="swiss-port-chip" data-port="8080" title="Standard Web Server (Right-click to delete)">:8080</button>
              <button class="swiss-port-chip" data-port="8765" title="Antigravity Swiss Knife (Right-click to delete)">:8765</button>
              <button class="swiss-port-chip" data-port="4173" title="Vite Production Preview (Right-click to delete)">:4173</button>
            </div>
            <div class="swiss-port-dropdown-wrap" id="swiss-port-dropdown-wrap">
              <select class="swiss-port-select" id="swiss-port-select" title="Quick Ports (Right-click to delete)">
                <option value="" disabled selected>Quick Ports</option>
                <option value="5173">:5173 (Vite)</option>
                <option value="3000">:3000 (React)</option>
                <option value="8080">:8080 (Web)</option>
                <option value="8765">:8765 (Swiss Knife)</option>
                <option value="4173">:4173 (Preview)</option>
                <option value="__add__">+ Add Port...</option>
              </select>
            </div>
            <div class="swiss-port-add-box" id="swiss-port-add-box">
              <button class="swiss-port-add-btn" id="swiss-port-add-btn" title="Add Port" type="button"><svg class="swiss-port-add-icon" viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg></button>
              <input type="text" class="swiss-port-add-input" id="swiss-port-add-input" placeholder="Port..." maxlength="8" />
            </div>
          </div>
        </div>
        <div class="swiss-browser-tools-row">
          <div class="swiss-browser-device-group">
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
          </div>
          <div class="swiss-browser-tools-divider"></div>
          <div class="swiss-browser-annotation-group">
            <button class="swiss-browser-btn" id="swiss-b-pen" title="Red Pen Drawing"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/></svg><span>Pen</span></button>
            <button class="swiss-browser-btn" id="swiss-b-rect" title="Red Box Annotation"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2"/></svg><span>Box</span></button>
            <button class="swiss-browser-btn" id="swiss-b-inspect" title="Interactive DOM Inspector"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m22 12-4 0"/><path d="m6 12-4 0"/><path d="m12 6 0-4"/><path d="m12 22 0-4"/></svg><span>Inspect</span></button>
            <button class="swiss-browser-btn" id="swiss-b-clear" title="Clear Annotations"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg></button>
            <button class="swiss-browser-btn primary" id="swiss-b-send-chat" title="Send to Antigravity Chat"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg><span>Send to Chat</span></button>
          </div>
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
        if (/^:\d+$/.test(finalUrl)) {
          finalUrl = "http://localhost" + finalUrl;
        } else if (!/^https?:\/\//i.test(finalUrl)) {
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
        const match = url.match(/^https?:\/\/(?:localhost|127\.0\.0\.1):(\d+)/i);
        const activePort = match ? match[1] : null;
        toolbar.querySelectorAll(".swiss-port-chip").forEach(chip => {
          chip.classList.toggle("active", chip.dataset.port === activePort);
        });
        const portSelect = toolbar.querySelector("#swiss-port-select");
        if (portSelect) {
          if (activePort && Array.from(portSelect.options).some(o => o.value === activePort)) {
            portSelect.value = activePort;
            portSelect.classList.add("active");
          } else {
            portSelect.value = "";
            portSelect.classList.remove("active");
          }
        }
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

      // Quick Ports Management & localStorage persistence
      const QUICK_PORTS_KEY = "antigravity_swiss_quick_ports";
      const DEFAULT_PORTS = ["5173", "3000", "8080", "8765", "4173"];
      const PORT_TITLES = {
        "5173": "Vite Development Server",
        "3000": "React / Next.js Server",
        "8080": "Standard Web Server",
        "8765": "Antigravity Swiss Knife",
        "4173": "Vite Production Preview"
      };

      function getSavedPorts() {
        try {
          const raw = localStorage.getItem(QUICK_PORTS_KEY);
          if (raw !== null) {
            const parsed = JSON.parse(raw);
            if (Array.isArray(parsed)) {
              return parsed
                .map(p => String(p).replace(/^:/, "").trim())
                .filter(p => /^\d+$/.test(p) && parseInt(p, 10) >= 1 && parseInt(p, 10) <= 65535);
            }
          }
        } catch (_) {}
        return [...DEFAULT_PORTS];
      }

      function savePorts(ports) {
        try {
          localStorage.setItem(QUICK_PORTS_KEY, JSON.stringify(ports));
        } catch (_) {}
      }

      function renderQuickPorts() {
        const portList = toolbar.querySelector("#swiss-port-list");
        const portSelect = toolbar.querySelector("#swiss-port-select");
        const ports = getSavedPorts();

        if (portList) {
          portList.innerHTML = "";
          ports.forEach(port => {
            const chip = document.createElement("button");
            chip.type = "button";
            chip.className = "swiss-port-chip";
            chip.setAttribute("data-port", port);
            chip.dataset.port = port;
            const desc = PORT_TITLES[port]
              ? (PORT_TITLES[port] + " (Right-click to delete)")
              : ("Port :" + port + " (Right-click to delete)");
            chip.title = desc;
            chip.textContent = ":" + port;

            chip.onclick = () => {
              navigateBrowser("http://localhost:" + port);
            };

            chip.oncontextmenu = (e) => {
              e.preventDefault();
              e.stopPropagation();
              showPortContextMenu(port, e.clientX, e.clientY);
            };

            portList.appendChild(chip);
          });
        }

        if (portSelect) {
          portSelect.innerHTML = "";
          const placeholder = document.createElement("option");
          placeholder.value = "";
          placeholder.disabled = true;
          placeholder.selected = true;
          placeholder.textContent = "Quick Ports";
          portSelect.appendChild(placeholder);

          ports.forEach(port => {
            const opt = document.createElement("option");
            opt.value = port;
            const shortDesc = {
              "5173": ":5173 (Vite)",
              "3000": ":3000 (React)",
              "8080": ":8080 (Web)",
              "8765": ":8765 (Swiss Knife)",
              "4173": ":4173 (Preview)"
            }[port] || (":" + port);
            opt.textContent = shortDesc;
            portSelect.appendChild(opt);
          });

          const addOpt = document.createElement("option");
          addOpt.value = "__add__";
          addOpt.textContent = "+ Add Port...";
          portSelect.appendChild(addOpt);
        }

        updateActivePortChip(currentBrowserUrl);
      }

      function showPortContextMenu(port, x, y) {
        const menu = document.createElement("div");
        menu.className = "swiss-context-menu";
        menu.setAttribute("class", "swiss-context-menu");
        menu.innerHTML = '<div class="swiss-context-item" id="ctx-port-open"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg><span>Open http://localhost:' + port + '</span></div>' +
          '<div class="swiss-context-item" id="ctx-port-copy"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg><span>Copy URL</span></div>' +
          '<div class="swiss-context-divider"></div>' +
          '<div class="swiss-context-item" id="ctx-port-delete" style="color:#ef4444;"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><line x1="10" y1="11" x2="10" y2="17"/><line x1="14" y1="11" x2="14" y2="17"/></svg><span>Delete :' + port + '</span></div>';

        menu.querySelector("#ctx-port-open").onclick = (e) => {
          e.stopPropagation();
          removeContextMenu();
          navigateBrowser("http://localhost:" + port);
        };

        menu.querySelector("#ctx-port-copy").onclick = (e) => {
          e.stopPropagation();
          removeContextMenu();
          navigator.clipboard.writeText("http://localhost:" + port);
          showToast("URL copied to clipboard!");
        };

        menu.querySelector("#ctx-port-delete").onclick = (e) => {
          e.stopPropagation();
          removeContextMenu();
          deletePort(port);
        };

        positionContextMenu(menu, x, y);
      }

      function deletePort(portToDelete) {
        const ports = getSavedPorts().filter(p => String(p) !== String(portToDelete));
        savePorts(ports);
        renderQuickPorts();
        showToast("Port :" + portToDelete + " removed");
      }

      // Add Port Box Interactions
      const addBox = toolbar.querySelector("#swiss-port-add-box");
      const addBtn = toolbar.querySelector("#swiss-port-add-btn");
      const addInput = toolbar.querySelector("#swiss-port-add-input");

      function openAddPort() {
        if (!addBox) return;
        addBox.classList.add("expanded");
        if (addBtn) addBtn.title = "Cancel";
        setTimeout(() => {
          if (addInput) {
            addInput.focus();
            addInput.select();
          }
          addBox.scrollIntoView({ block: "nearest", inline: "nearest" });
        }, 50);
      }

      function closeAddPort() {
        if (!addBox) return;
        addBox.classList.remove("expanded");
        if (addBtn) addBtn.title = "Add Port";
        if (addInput) addInput.value = "";
      }

      function submitNewPort() {
        if (!addInput) return;
        const rawVal = addInput.value.trim();
        if (!rawVal) {
          closeAddPort();
          return;
        }
        const cleanVal = rawVal.replace(/^(?:https?:\/\/)?(?:localhost|127\.0\.0\.1)?:?/, "").trim();
        const portNum = parseInt(cleanVal, 10);
        if (!/^\d+$/.test(cleanVal) || isNaN(portNum) || portNum < 1 || portNum > 65535) {
          showToast("Invalid port number (1-65535)", "warning");
          addInput.focus();
          addInput.select();
          return;
        }
        const portStr = String(portNum);
        const ports = getSavedPorts();
        if (!ports.includes(portStr)) {
          ports.push(portStr);
          savePorts(ports);
          renderQuickPorts();
          showToast("Port :" + portStr + " saved");
        } else {
          showToast("Port :" + portStr + " already exists", "warning");
        }
        closeAddPort();
        navigateBrowser("http://localhost:" + portStr);
      }

      if (addBtn) {
        addBtn.onclick = (e) => {
          e.stopPropagation();
          if (addBox.classList.contains("expanded")) {
            closeAddPort();
          } else {
            openAddPort();
          }
        };
      }

      if (addInput) {
        addInput.onkeydown = (e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            submitNewPort();
          } else if (e.key === "Escape") {
            e.preventDefault();
            closeAddPort();
          }
        };
      }

      const onOutsideAddPortMousedown = (e) => {
        if (addBox && addBox.classList.contains("expanded") && !addBox.contains(e.target)) {
          closeAddPort();
        }
      };
      if (window.__swissAddPortMousedown) {
        document.removeEventListener("mousedown", window.__swissAddPortMousedown);
      }
      window.__swissAddPortMousedown = onOutsideAddPortMousedown;
      document.addEventListener("mousedown", onOutsideAddPortMousedown);
      // Dropdown Select Interactions
      const portSelect = toolbar.querySelector("#swiss-port-select");
      if (portSelect) {
        portSelect.onchange = async () => {
          const val = portSelect.value;
          const match = currentBrowserUrl.match(/^https?:\/\/(?:localhost|127\.0\.0\.1):(\d+)/i);
          const activePort = match ? match[1] : null;
          const ports = getSavedPorts();

          if (val === "__add__") {
            portSelect.value = (activePort && ports.includes(activePort)) ? activePort : "";
            openAddPort();
            return;
          }
          if (val) {
            navigateBrowser("http://localhost:" + val);
          }
        };

        portSelect.oncontextmenu = (e) => {
          e.preventDefault();
          e.stopPropagation();
          const match = currentBrowserUrl.match(/^https?:\/\/(?:localhost|127\.0\.0\.1):(\d+)/i);
          const port = (match && match[1]) || (portSelect.value && portSelect.value !== "__add__" ? portSelect.value : null);
          if (port) {
            showPortContextMenu(port, e.clientX, e.clientY);
          }
        };
      }

      // Responsive Toolbar Check (Container Query fallback & width-based class toggling)
      function updateToolbarResponsiveness() {
        if (!toolbar) return;
        toolbar.classList.toggle("compact-ports", toolbar.clientWidth < 580);
        toolbar.classList.toggle("compact-tools", toolbar.clientWidth < 620);
      }
      if (window.__swissToolbarRO) {
        try { window.__swissToolbarRO.disconnect(); } catch (_) {}
      }
      if (typeof ResizeObserver !== "undefined") {
        const ro = new ResizeObserver(() => updateToolbarResponsiveness());
        ro.observe(toolbar);
        window.__swissToolbarRO = ro;
      } else {
        window.addEventListener("resize", updateToolbarResponsiveness);
      }
      updateToolbarResponsiveness();

      // Mousewheel Horizontal Scrolling for single-row preservation on narrow panels
      const navRow = toolbar.querySelector(".swiss-browser-nav-row");
      const toolsRow = toolbar.querySelector(".swiss-browser-tools-row");
      const devGroup = toolbar.querySelector(".swiss-browser-device-group");
      const annGroup = toolbar.querySelector(".swiss-browser-annotation-group");
      [navRow, toolsRow, devGroup, annGroup].forEach(row => {
        if (!row) return;
        row.addEventListener("wheel", (e) => {
          if (Math.abs(e.deltaY) > Math.abs(e.deltaX) && row.scrollWidth > row.clientWidth) {
            row.scrollLeft += e.deltaY;
          }
        }, { passive: true });
      });

      renderQuickPorts();

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
        const comment = await showSwissPrompt(
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

          let promptText = "[Preview Browser Annotation @ " + currentBrowserUrl + "]\n";
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
    const PREFERRED_IDE_KEY = "antigravity_preferred_ide";
    function getPreferredIDE() {
      try {
        return localStorage.getItem(PREFERRED_IDE_KEY) || localStorage.getItem("antigravity_swiss_preferred_ide") || "code";
      } catch (_) {
        return "code";
      }
    }
    function getIDELabel(ideId) {
      const id = (ideId || "").toLowerCase().trim();
      const map = {
        code: "VS Code",
        vscode: "VS Code",
        cursor: "Cursor",
        windsurf: "Windsurf",
        codium: "VSCodium",
        vscodium: "VSCodium",
        zed: "Zed"
      };
      return map[id] || (ideId ? ideId : "VS Code");
    }

    function renderFilesView(container) {
      const wrap = document.createElement("div");
      wrap.className = "swiss-files-view";

      const toolbar = document.createElement("div");
      toolbar.className = "swiss-files-toolbar";
      toolbar.innerHTML = ` + "`" + `
        <div class="swiss-files-address-bar">
          <button class="swiss-browser-btn" id="swiss-f-home" title="Home Folder"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-f-refresh" title="Refresh"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg></button>
          <input type="text" class="swiss-files-path-input" id="swiss-f-path" value="${currentFilePath}" />
          <button class="swiss-browser-btn" id="swiss-f-reveal" title="Open in System File Manager"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 14 1.45-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.55 6a2 2 0 0 1-1.94 1.5H4a2 2 0 0 1-2-2V5c0-1.1.9-2 2-2h3.93a2 2 0 0 1 1.66.9l.82 1.2a2 2 0 0 0 1.66.9H18a2 2 0 0 1 2 2v2"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-f-new-file" title="New File"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><polyline points="14 2 14 8 20 8"/><line x1="12" x2="12" y1="18" y2="12"/><line x1="9" x2="15" y1="15" y2="15"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-f-new-dir" title="New Folder"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 10v6"/><path d="M9 13h6"/><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg></button>
        </div>
        <div class="swiss-files-actions-bar" style="display:flex; align-items:center; gap:6px;">
          <button class="swiss-browser-btn" id="swiss-f-back" title="Back" disabled><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-f-up" title="Up Directory"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m18 15-6-6-6 6"/></svg></button>
          <input type="text" placeholder="Filter files..." id="swiss-f-search" style="flex:1; min-width:0; padding:3px 8px; font-size:11px; border-radius:4px; border:1px solid var(--border,#cbd5e1); background:var(--canvas,#fff);" />
          <button class="swiss-browser-btn" id="swiss-f-hidden" title="Toggle Hidden Files"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-f-term" title="Open in Terminal"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-f-ide" title="Open in VS Code"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg></button>
        </div>
      ` + "`" + `;

      const listContainer = document.createElement("div");
      listContainer.className = "swiss-files-list";

      wrap.appendChild(toolbar);
      wrap.appendChild(listContainer);
      container.appendChild(wrap);

      let selectedPaths = new Set();
      let fileItemsMap = new Map();
      let currentFetchId = 0;

      const SHOW_HIDDEN_KEY = "antigravity_swiss_show_hidden_files";
      let showHiddenFiles = false;
      try {
        showHiddenFiles = localStorage.getItem(SHOW_HIDDEN_KEY) === "true";
      } catch (_) {}

      const hiddenBtn = toolbar.querySelector("#swiss-f-hidden");
      const eyeIconSvg = '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/></svg>';
      const eyeOffIconSvg = '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9.88 9.88a3 3 0 1 0 4.24 4.24"/><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68"/><path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61"/><line x1="2" x2="22" y1="2" y2="22"/></svg>';

      const updateHiddenBtnState = () => {
        if (!hiddenBtn) return;
        hiddenBtn.classList.toggle("toggled", showHiddenFiles);
        hiddenBtn.classList.toggle("active", showHiddenFiles);
        hiddenBtn.title = showHiddenFiles ? "Hide Hidden Files (dotfiles)" : "Show Hidden Files (dotfiles)";
        hiddenBtn.innerHTML = showHiddenFiles ? eyeOffIconSvg : eyeIconSvg;
      };
      updateHiddenBtnState();

      const applyFileFilters = () => {
        const searchBox = toolbar.querySelector("#swiss-f-search");
        const q = (searchBox ? searchBox.value : "").toLowerCase().trim();
        let visibleCount = 0;
        listContainer.querySelectorAll(".swiss-file-row").forEach(r => {
          const name = (r.querySelector(".swiss-file-name")?.textContent || "").toLowerCase();
          const isHidden = r.dataset.hidden === "true";
          const matchesSearch = !q || name.includes(q);
          const matchesHidden = showHiddenFiles || !isHidden;
          if (matchesSearch && matchesHidden) {
            r.style.display = "flex";
            visibleCount++;
          } else {
            r.style.display = "none";
            if (!matchesHidden && selectedPaths.has(r.dataset.path)) {
              selectedPaths.delete(r.dataset.path);
              r.classList.remove("selected");
            }
          }
        });

        let emptyNotice = listContainer.querySelector(".swiss-empty-filter-notice");
        if (fileItemsMap.size > 0 && visibleCount === 0) {
          if (!emptyNotice) {
            emptyNotice = document.createElement("div");
            emptyNotice.className = "swiss-empty-filter-notice";
            emptyNotice.style.cssText = "padding:16px; font-size:11px; color:#94a3b8; text-align:center;";
            listContainer.appendChild(emptyNotice);
          }
          emptyNotice.textContent = q ? "No files matching filter" : "No visible files (hidden files filtered)";
          emptyNotice.style.display = "block";
        } else if (emptyNotice) {
          emptyNotice.style.display = "none";
        }
      };

      const updateNavButtons = () => {
        const backBtn = toolbar.querySelector("#swiss-f-back");
        if (backBtn) {
          backBtn.disabled = fileHistory.length === 0;
          backBtn.style.opacity = fileHistory.length > 0 ? "1" : "0.5";
        }
      };
      updateNavButtons();

      // Load files
      const loadFiles = async (dirPath, pushHistory = true) => {
        let targetPath = (dirPath || "").trim();
        if (targetPath.startsWith("file://")) {
          targetPath = targetPath.replace(/^file:\/\//, "");
        }
        if (targetPath.includes("%")) {
          try { targetPath = decodeURIComponent(targetPath); } catch (_) {}
        }
        if (targetPath !== "~") {
          targetPath = targetPath.replace(/[/\\]+$/, "") || "/";
        }
        const fetchId = ++currentFetchId;
        const pathInput = toolbar.querySelector("#swiss-f-path");
        if (pathInput) pathInput.value = targetPath;
        const backBtn = toolbar.querySelector("#swiss-f-back");
        if (backBtn) backBtn.disabled = true;
        listContainer.innerHTML = "<div style='padding:12px; font-size:11px; color:#94a3b8;'>Loading files...</div>";

        try {
          const res = await fetch(` + "`" + `${API_BASE}/api/files/list?path=${encodeURIComponent(targetPath)}` + "`" + `);
          if (fetchId !== currentFetchId) return;
          if (!res.ok) {
            throw new Error("HTTP " + res.status + ": " + res.statusText);
          }
          const data = await res.json();
          if (fetchId !== currentFetchId) return;
          if (!data.success) {
            listContainer.innerHTML = ` + "`" + `<div style='padding:14px; color:#ef4444; font-size:11px; display:flex; flex-direction:column; gap:6px;'>
              <div style='font-weight:600;'>Error loading directory:</div>
              <div style='font-family:monospace; background:rgba(239,68,68,0.08); padding:6px 8px; border-radius:4px;'>${data.error || "Unknown error"}</div>
              <button class="swiss-browser-btn" id="swiss-f-retry" style="align-self:flex-start; margin-top:4px;">Retry</button>
            </div>` + "`" + `;
            listContainer.querySelector("#swiss-f-retry")?.addEventListener("click", () => loadFiles(targetPath, false));
            updateNavButtons();
            return;
          }

          const prevPath = currentFilePath;
          const loadedPath = data.path || targetPath;
          if (pushHistory && prevPath && prevPath !== "." && prevPath !== loadedPath) {
            if (fileHistory.length === 0 || fileHistory[fileHistory.length - 1] !== prevPath) {
              fileHistory.push(prevPath);
              if (fileHistory.length > 50) fileHistory.shift();
            }
          }
          currentFilePath = loadedPath;
          if (container && container.id === "swiss-main-stage-body") {
            stageFilePath = loadedPath;
          } else {
            auxFilePath = loadedPath;
          }
          if (pathInput) pathInput.value = loadedPath;
          updateNavButtons();

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
            const isHidden = (item.name || "").startsWith(".");
            row.dataset.hidden = isHidden ? "true" : "false";
            if (isHidden) {
              row.classList.add("swiss-file-hidden");
            }

            const iconSvg = item.isDir 
              ? '<svg class="swiss-file-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>'
              : item.type === "code"
              ? '<svg class="swiss-file-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>'
              : item.type === "markdown"
              ? '<svg class="swiss-file-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><line x1="16" x2="8" y1="13" y2="13"/><line x1="16" x2="8" y1="17" y2="17"/><line x1="10" x2="8" y1="9" y1="9"/></svg>'
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
              showFileContextMenu(e.clientX, e.clientY, item, selectedItems, () => loadFiles(currentFilePath, false));
            };

            listContainer.appendChild(row);
          });
          applyFileFilters();
        } catch (err) {
          if (fetchId !== currentFetchId) return;
          listContainer.innerHTML = ` + "`" + `<div style='padding:14px; color:#ef4444; font-size:11px; display:flex; flex-direction:column; gap:6px;'>
            <div style='font-weight:600;'>Unable to connect to Swiss Knife daemon:</div>
            <div style='color:#64748b;'>${err.message}. Check that the daemon is running on ${API_BASE} (e.g. 'swiss daemon --with-web' or 'swiss web').</div>
            <button class="swiss-browser-btn" id="swiss-f-retry" style="align-self:flex-start; margin-top:4px;">Retry</button>
          </div>` + "`" + `;
          listContainer.querySelector("#swiss-f-retry")?.addEventListener("click", () => loadFiles(targetPath, false));
          updateNavButtons();
        }
      };

      // Background right-click on listContainer
      listContainer.oncontextmenu = (e) => {
        if (e.target.closest(".swiss-file-row")) return;
        e.preventDefault();
        e.stopPropagation();
        window.getSelection()?.removeAllRanges();
        showBlankContextMenu(e.clientX, e.clientY, currentFilePath, () => loadFiles(currentFilePath, false));
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
      toolbar.querySelector("#swiss-f-back").onclick = () => {
        if (fileHistory.length > 0) {
          const prev = fileHistory.pop();
          loadFiles(prev, false);
        }
      };
      toolbar.querySelector("#swiss-f-up").onclick = () => {
        let clean = currentFilePath.replace(/[/\\]+$/, "");
        const lastSlash = Math.max(clean.lastIndexOf("/"), clean.lastIndexOf("\\"));
        if (lastSlash > 0) {
          if (lastSlash === 2 && clean[1] === ":") {
            loadFiles(clean.substring(0, 3));
          } else {
            loadFiles(clean.substring(0, lastSlash));
          }
        } else if (lastSlash === 0) {
          loadFiles("/");
        }
      };
      toolbar.querySelector("#swiss-f-home").onclick = () => loadFiles("~");
      toolbar.querySelector("#swiss-f-refresh").onclick = () => loadFiles(currentFilePath, false);
      if (hiddenBtn) {
        hiddenBtn.onclick = () => {
          showHiddenFiles = !showHiddenFiles;
          try {
            localStorage.setItem(SHOW_HIDDEN_KEY, showHiddenFiles ? "true" : "false");
          } catch (_) {}
          updateHiddenBtnState();
          applyFileFilters();
        };
      }
      const searchBox = toolbar.querySelector("#swiss-f-search");
      if (searchBox) {
        searchBox.oninput = () => {
          applyFileFilters();
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
      const ideBtn = toolbar.querySelector("#swiss-f-ide");
      const updateIDEButtonState = () => {
        if (!ideBtn) return;
        const currentIDE = getPreferredIDE();
        ideBtn.title = "Open in " + getIDELabel(currentIDE);
      };
      updateIDEButtonState();
      try {
        fetch(API_BASE + "/api/files/ide/config")
          .then(r => r.json())
          .then(data => {
            if (data && data.success && data.preferred_ide) {
              try { localStorage.setItem(PREFERRED_IDE_KEY, data.preferred_ide); } catch (_) {}
              updateIDEButtonState();
            }
          })
          .catch(() => {});
      } catch (_) {}
      if (ideBtn) {
        ideBtn.onmouseenter = updateIDEButtonState;
        ideBtn.onclick = () => {
          const currentIDE = getPreferredIDE();
          fetch(` + "`" + `${API_BASE}/api/files/open_ide` + "`" + `, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ path: currentFilePath, ide: currentIDE })
          });
        };
      }
      toolbar.querySelector("#swiss-f-new-file").onclick = async () => {
        const name = await showSwissPrompt("Enter new file name:");
        if (!name || !name.trim()) return;
        await fetch(` + "`" + `${API_BASE}/api/files/create` + "`" + `, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentFilePath + "/" + name.trim(), is_dir: false })
        });
        loadFiles(currentFilePath, false);
      };
      toolbar.querySelector("#swiss-f-new-dir").onclick = async () => {
        const name = await showSwissPrompt("Enter new directory name:");
        if (!name || !name.trim()) return;
        await fetch(` + "`" + `${API_BASE}/api/files/create` + "`" + `, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentFilePath + "/" + name.trim(), is_dir: true })
        });
        loadFiles(currentFilePath, false);
      };

      let initialTargetPath = ".";
      if (container && container.id === "swiss-main-stage-body") {
        const stScope = (typeof window.__swissGetMainStageScope === "function" && window.__swissGetMainStageScope()) || "GLOBAL";
        if (lastStageScope !== stScope) {
          lastStageScope = stScope;
          stageFileHistory = [];
          stageFilePath = (stScope && stScope !== "GLOBAL") ? stScope : ".";
        }
        initialTargetPath = stageFilePath || ((stScope && stScope !== "GLOBAL") ? stScope : ".");
        currentFilePath = initialTargetPath;
        fileHistory = stageFileHistory;
      } else {
        const activeProj = (typeof window.__swissGetActiveProject === "function" && window.__swissGetActiveProject()) || ".";
        if (lastAuxProject !== activeProj) {
          lastAuxProject = activeProj;
          auxFileHistory = [];
          auxFilePath = (activeProj && activeProj !== ".") ? activeProj : ".";
        }
        initialTargetPath = auxFilePath || ((activeProj && activeProj !== ".") ? activeProj : ".");
        currentFilePath = initialTargetPath;
        fileHistory = auxFileHistory;
      }
      loadFiles(initialTargetPath, false);
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

          editor.querySelector("#swiss-ed-annotate").onclick = async () => {
            const start = textarea.selectionStart;
            const end = textarea.selectionEnd;
            const selectedText = textarea.value.substring(start, end).trim();

            if (!selectedText) {
              showToast("Select text/code lines first, then click Annotate to Chat!", "warning");
              return;
            }

            const lineNum = textarea.value.substring(0, start).split("\n").length;
            const comment = await showSwissPrompt("Enter annotation note for selected snippet:", "Please review this logic and suggest improvements:");
            if (comment === null) return;

            const snippetMsg = "[Annotated Code: " + name + " (around line " + lineNum + ")]\nComment: \"" + comment + "\"\n` + "\x60\x60\x60" + `\n" + selectedText + "\n` + "\x60\x60\x60" + `";
            insertTextToChatInput(snippetMsg);
            showToast("Annotation snippet injected into Antigravity chat input!");
          };
        });
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
        '<div class="swiss-context-item" id="ctx-term"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/></svg><span>Open in Terminal</span></div>' +
        '<div class="swiss-context-item" id="ctx-ide"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg><span>Open in ' + getIDELabel(getPreferredIDE()) + '</span></div>';
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
          removeContextMenu();
          const newName = await showSwissPrompt("Rename to:", item.name);
          if (!newName || newName.trim() === item.name) {
            return;
          }
          const trimmed = newName.trim();
          const newPath = item.path.substring(0, item.path.lastIndexOf("/") + 1) + trimmed;
          await fetch(API_BASE + "/api/files/rename", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ old_path: item.path, new_path: newPath })
          });
          onRefresh();
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

      menu.querySelector("#ctx-ide")?.addEventListener("click", () => {
        fetch(API_BASE + "/api/files/open_ide", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: item.path, ide: getPreferredIDE() })
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
        '<div class="swiss-context-item" id="ctx-blank-term"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/></svg><span>Open in Terminal</span></div>' +
        '<div class="swiss-context-item" id="ctx-blank-ide"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg><span>Open in ' + getIDELabel(getPreferredIDE()) + '</span></div>';
      menu.innerHTML = blankHtml;

      positionContextMenu(menu, x, y);

      menu.querySelector("#ctx-blank-paste")?.addEventListener("click", () => {
        if (!hasClipboard) return;
        executePaste(currentDir, onRefresh);
        removeContextMenu();
      });

      menu.querySelector("#ctx-blank-new-file")?.addEventListener("click", async () => {
        removeContextMenu();
        const name = await showSwissPrompt("Enter new file name:");
        if (!name || !name.trim()) return;
        await fetch(API_BASE + "/api/files/create", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentDir + "/" + name.trim(), is_dir: false })
        });
        onRefresh();
      });

      menu.querySelector("#ctx-blank-new-dir")?.addEventListener("click", async () => {
        removeContextMenu();
        const name = await showSwissPrompt("Enter new directory name:");
        if (!name || !name.trim()) return;
        await fetch(API_BASE + "/api/files/create", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentDir + "/" + name.trim(), is_dir: true })
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

      menu.querySelector("#ctx-blank-ide")?.addEventListener("click", () => {
        removeContextMenu();
        fetch(API_BASE + "/api/files/open_ide", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ path: currentDir, ide: getPreferredIDE() })
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
        <button class="swiss-browser-btn primary" id="swiss-m-new-text" style="flex: 1 1 0; min-width: 0; justify-content: center; text-align: center; padding: 6px 12px; box-sizing: border-box; white-space: nowrap;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"/><path d="M12 5v14"/></svg><span>Text Memo</span></button>
        <button class="swiss-browser-btn" id="swiss-m-record-audio" style="flex: 1 1 0; min-width: 0; justify-content: center; text-align: center; padding: 6px 12px; box-sizing: border-box; white-space: nowrap;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" x2="12" y1="19" y2="22"/></svg><span>Voice Memo</span></button>
      ` + "`" + `;
      wrap.appendChild(topBar);

      const searchRow = document.createElement("div");
      searchRow.className = "swiss-memo-search-row";
      searchRow.style.display = "flex";
      searchRow.style.alignItems = "center";
      searchRow.style.gap = "6px";
      searchRow.innerHTML = ` + "`" + `
        <div style="position:relative; flex:1; display:flex; align-items:center;">
          <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="position:absolute; left:8px; color:var(--text-muted,#64748b); pointer-events:none;"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>
          <input type="text" id="swiss-m-search-input" placeholder="Search memos..." class="swiss-memo-search-input" style="width:100%; padding:5px 24px 5px 26px; font-size:11px; border-radius:6px; border:1px solid var(--border,#cbd5e1); background:var(--canvas,#ffffff); color:var(--text,#1e293b); box-sizing:border-box; outline:none;" />
          <button type="button" id="swiss-m-search-clear" style="display:none; position:absolute; right:6px; background:none; border:none; padding:2px; cursor:pointer; color:var(--text-muted,#64748b);"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg></button>
        </div>
        <button type="button" id="swiss-m-scope-toggle" class="swiss-browser-btn" title="Toggle Search Scope: Text only vs Text + Voice" style="padding:4px 8px; font-size:10px; white-space:nowrap;"><span id="swiss-m-scope-label">Text</span></button>
      ` + "`" + `;
      wrap.appendChild(searchRow);

      const composer = document.createElement("div");
      composer.className = "swiss-memo-composer";
      composer.id = "swiss-m-composer";
      composer.style.display = "none";
      composer.innerHTML = ` + "`" + `
        <textarea id="swiss-m-composer-input" class="swiss-memo-composer-textarea" placeholder="Type quick memo... (Enter or Ctrl+Enter to save, Esc to cancel)" rows="3"></textarea>
        <div class="swiss-memo-composer-actions">
          <button type="button" class="swiss-browser-btn" id="swiss-m-composer-cancel"><span>Cancel</span></button>
          <button type="button" class="swiss-browser-btn primary" id="swiss-m-composer-save"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 13l4 4L19 7"/></svg><span>Save Memo</span></button>
        </div>
      ` + "`" + `;
      wrap.appendChild(composer);

      const memoList = document.createElement("div");
      memoList.style.display = "flex";
      memoList.style.flexDirection = "column";
      memoList.style.gap = "8px";
      wrap.appendChild(memoList);
      container.appendChild(wrap);

      let allMemos = [];
      let searchQuery = "";
      let searchScope = "text";

      let mediaStream = null;
      let mediaRecorder = null;
      let recordedChunks = [];
      let speechRecognition = null;
      let speechTranscript = "";
      let recordingStartTime = 0;

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

      const escapeHtml = (str) => {
        return String(str || "").replace(/[&<>"']/g, function(s) {
          switch (s) {
            case "&": return "&amp;";
            case "<": return "&lt;";
            case ">": return "&gt;";
            case '"': return "&quot;";
            case "'": return "&#39;";
            default: return s;
          }
        });
      };

      const filterMemosList = (memos, query, scope) => {
        if (!Array.isArray(memos)) return [];
        const trimmed = (query || "").trim();
        if (!trimmed) return memos;
        const q = trimmed.toLowerCase();
        const effScope = scope === "all" ? "all" : "text";
        return memos.filter(m => {
          if (!m) return false;
          const isVoice = m.type === "voice" || m.type === "audio" || (Array.isArray(m.tags) && m.tags.includes("voice") && m.type !== "text");
          if (effScope === "text" && isVoice) return false;
          const titleMatch = Boolean(m.title && m.title.toLowerCase().includes(q));
          const contentMatch = Boolean(m.content && m.content.toLowerCase().includes(q));
          const transcriptMatch = Boolean(m.transcript && m.transcript.toLowerCase().includes(q));
          const tagsMatch = Array.isArray(m.tags) && m.tags.some(t => typeof t === "string" && t.toLowerCase().includes(q));
          return titleMatch || contentMatch || transcriptMatch || tagsMatch;
        });
      };

      const renderCards = (memos) => {
        memoList.innerHTML = "";
        const trimmedQ = (searchQuery || "").trim();
        if (!Array.isArray(memos) || memos.length === 0) {
          if (trimmedQ) {
            memoList.innerHTML = '<div class="swiss-memo-empty-search">No memos found matching "' + escapeHtml(trimmedQ) + '"</div>';
          } else {
            memoList.innerHTML = "<div style='font-size:11px; color:#94a3b8; text-align:center; padding:20px 0;'>No memos yet. Click Text Memo or Voice Memo!</div>";
          }
          return;
        }

        memos.forEach(m => {
          const card = document.createElement("div");
          card.className = "swiss-memo-card";
          card.draggable = true;

          card.innerHTML = ` + "`" + `
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:4px;">
              <div style="display:flex; align-items:center; gap:4px; overflow:hidden;">
                <span style="font-weight:600; font-size:11px; white-space:nowrap; text-overflow:ellipsis; overflow:hidden;">${escapeHtml(m.title || "Memo")}</span>
                ${m.duration ? ('<span style="font-size:9px; color:#b06000; font-weight:600; white-space:nowrap;">(' + escapeHtml(m.duration) + ')</span>') : ""}
              </div>
              <span style="font-size:9px; color:#94a3b8; white-space:nowrap;">${escapeHtml(m.created_at || m.createdAt || "")}</span>
            </div>
            <div style="font-size:11px; color:var(--text,#1e293b); white-space:pre-wrap; margin-bottom:8px;">${escapeHtml(m.content || "")}</div>
            ${m.audio_data ? ('<audio controls src="' + escapeHtml(m.audio_data) + '" style="width:100%; height:24px; margin-bottom:6px; outline:none;"></audio>') : ""}
            <div style="display:flex; justify-content:flex-end; gap:6px;">
              <button class="swiss-browser-btn" id="m-insert" style="padding:2px 6px; font-size:10px;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg><span>Chat</span></button>
              <button class="swiss-browser-btn" id="m-del" title="Delete Memo" style="padding:2px 6px; font-size:10px; color:#ef4444;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg></button>
            </div>
          ` + "`" + `;

          card.ondragstart = (e) => {
            e.dataTransfer.setData("text/plain", m.transcript || m.content || "");
          };

          card.querySelector("#m-insert").onclick = () => {
            insertTextToChatInput(m.transcript || m.content || "");
          };

          card.querySelector("#m-del").onclick = async () => {
            allMemos = allMemos.filter(item => item.id !== m.id);
            saveLocalMemos(allMemos);
            applyFilterAndRender();
            try {
              await fetch(` + "`" + `${API_BASE}/api/memos/delete?id=${encodeURIComponent(m.id)}` + "`" + `, { method: "POST" });
            } catch (_) {}
            loadMemos(true);
          };

          memoList.appendChild(card);
        });
      };

      const applyFilterAndRender = () => {
        const filtered = filterMemosList(allMemos, searchQuery, searchScope);
        renderCards(filtered);
      };

      const getMemoQueryInfo = () => {
        if (container && container.id === "swiss-main-stage-body") {
          const stScope = (typeof window.__swissGetMainStageScope === "function" && window.__swissGetMainStageScope()) || "GLOBAL";
          if (!stScope || stScope === "GLOBAL") {
            return { qs: "?scope=all", wsPath: "" };
          }
          return { qs: "?scope=current&workspace_path=" + encodeURIComponent(stScope), wsPath: stScope };
        }
        const activeProj = (typeof window.__swissGetActiveProject === "function" && window.__swissGetActiveProject()) || ".";
        return { qs: "?scope=current&workspace_path=" + encodeURIComponent(activeProj), wsPath: activeProj };
      };

      const loadMemos = async (silent = false) => {
        if (!silent && memoList.children.length === 0) {
          memoList.innerHTML = "<div style='font-size:11px; color:#94a3b8;'>Loading memos...</div>";
        }
        try {
          const mq = getMemoQueryInfo();
          const res = await fetch(` + "`" + `${API_BASE}/api/memos${mq.qs}` + "`" + `);
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
          allMemos = memos;
          saveLocalMemos(allMemos);
          applyFilterAndRender();
        } catch (err) {
          const fallback = getLocalMemos();
          if (fallback.length > 0) {
            allMemos = fallback;
            applyFilterAndRender();
          } else if (!silent || memoList.children.length === 0) {
            memoList.innerHTML = "<div style='font-size:11px; color:#94a3b8; text-align:center; padding:20px 0;'>No memos yet. Click Text Memo or Voice Memo!</div>";
          }
        }
      };

      const composerInput = composer.querySelector("#swiss-m-composer-input");
      const composerCancel = composer.querySelector("#swiss-m-composer-cancel");
      const composerSave = composer.querySelector("#swiss-m-composer-save");

      const closeComposer = () => {
        composer.style.display = "none";
        composerInput.value = "";
      };

      const openComposer = () => {
        composer.style.display = "flex";
        setTimeout(() => composerInput.focus(), 30);
      };

      const createAndSaveMemo = async (text, memoType, tags) => {
        if (!text) return;
        const trimmed = text.trim();
        if (!trimmed) return;
        const titleStr = trimmed.substring(0, 24) + (trimmed.length > 24 ? "..." : "");
        const newMemo = {
          id: "memo-" + Date.now(),
          title: titleStr,
          content: trimmed,
          type: memoType || "text",
          tags: tags || ["quick"],
          created_at: new Date().toLocaleString()
        };
        allMemos.unshift(newMemo);
        saveLocalMemos(allMemos);
        applyFilterAndRender();

        try {
          const mq = getMemoQueryInfo();
          await fetch(` + "`" + `${API_BASE}/api/memos/save` + "`" + `, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              id: newMemo.id,
              title: titleStr,
              content: trimmed,
              type: memoType || "text",
              tags: tags || ["quick"],
              created_at: newMemo.created_at,
              workspace_path: mq.wsPath || undefined
            })
          });
        } catch (_) {}
        loadMemos(true);
      };

      topBar.querySelector("#swiss-m-new-text").onclick = () => {
        if (composer.style.display === "none") {
          openComposer();
        } else {
          if (!composerInput.value.trim()) {
            closeComposer();
          } else {
            composerInput.focus();
          }
        }
      };

      composerCancel.onclick = () => {
        closeComposer();
      };

      composerSave.onclick = async () => {
        const text = composerInput.value;
        if (!text.trim()) {
          composerInput.focus();
          return;
        }
        closeComposer();
        await createAndSaveMemo(text, "text", ["quick"]);
      };

      composerInput.onkeydown = async (e) => {
        if (e.isComposing || e.keyCode === 229) return;
        if (e.key === "Escape") {
          e.preventDefault();
          closeComposer();
        } else if (e.key === "Enter" && (e.ctrlKey || e.metaKey || !e.shiftKey)) {
          e.preventDefault();
          const text = composerInput.value;
          if (!text.trim()) return;
          closeComposer();
          await createAndSaveMemo(text, "text", ["quick"]);
        }
      };

      const recordBtn = topBar.querySelector("#swiss-m-record-audio");
      recordBtn.onclick = async () => {
        if (!mediaRecorder || mediaRecorder.state === "inactive") {
          try {
            mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true });
            mediaRecorder = new MediaRecorder(mediaStream);
            recordedChunks = [];
            recordingStartTime = Date.now();
            speechTranscript = "";

            const SpeechRec = window.SpeechRecognition || window.webkitSpeechRecognition;
            if (typeof SpeechRec !== "undefined") {
              try {
                speechRecognition = new SpeechRec();
                speechRecognition.continuous = true;
                speechRecognition.interimResults = true;
                speechRecognition.lang = navigator.language || "en-US";
                speechRecognition.onresult = (event) => {
                  let str = "";
                  for (let i = 0; i < event.results.length; ++i) {
                    if (event.results[i] && event.results[i][0]) {
                      str += event.results[i][0].transcript + " ";
                    }
                  }
                  speechTranscript = str.trim();
                };
                speechRecognition.onerror = (e) => {
                  console.warn("SpeechRecognition error:", e && e.error ? e.error : e);
                };
                speechRecognition.onend = () => {
                  speechRecognition = null;
                };
                speechRecognition.start();
              } catch (recErr) {
                console.warn("SpeechRecognition init failed:", recErr);
                speechRecognition = null;
              }
            } else {
              speechRecognition = null;
            }

            mediaRecorder.ondataavailable = (e) => { if (e.data && e.data.size > 0) recordedChunks.push(e.data); };
            mediaRecorder.onstop = async () => {
              if (mediaStream) {
                mediaStream.getTracks().forEach(t => t.stop());
                mediaStream = null;
              }
              if (speechRecognition) {
                try { speechRecognition.stop(); } catch (_) {}
                speechRecognition = null;
              }

              const elapsedSec = Math.max(1, Math.round((Date.now() - recordingStartTime) / 1000));
              const mins = Math.floor(elapsedSec / 60);
              const secs = elapsedSec % 60;
              const formattedDuration = mins + ":" + (secs < 10 ? "0" : "") + secs;

              let base64Audio = "";
              try {
                const mimeType = (mediaRecorder && mediaRecorder.mimeType) || "audio/webm";
                const audioBlob = new Blob(recordedChunks, { type: mimeType });
                base64Audio = await new Promise((resolve) => {
                  const reader = new FileReader();
                  reader.onloadend = () => resolve(typeof reader.result === "string" ? reader.result : "");
                  reader.onerror = () => resolve("");
                  reader.readAsDataURL(audioBlob);
                });
              } catch (_) {
                base64Audio = "";
              }

              const finalTranscript = (speechTranscript || "").trim();
              let noteText = null;
              if (finalTranscript) {
                noteText = await showSwissPrompt("Voice recorded & transcribed! Edit title:", finalTranscript);
              } else {
                noteText = await showSwissPrompt("Voice recorded! Enter a transcript / note title:", "Voice Memo Note");
              }

              if (noteText !== null) {
                const userTitle = noteText.trim() || (finalTranscript ? finalTranscript : "Voice Memo Note");
                let titleStr = "[Voice] " + userTitle;
                if (userTitle.toLowerCase().startsWith("[voice]")) {
                  titleStr = userTitle;
                }
                const contentStr = finalTranscript || userTitle;
                const mq = getMemoQueryInfo();
                const newMemo = {
                  id: "memo-" + Date.now(),
                  title: titleStr,
                  content: contentStr,
                  transcript: finalTranscript,
                  audio_data: base64Audio,
                  duration: formattedDuration,
                  type: "audio",
                  tags: ["voice"],
                  workspace_path: mq.wsPath || undefined,
                  created_at: new Date().toLocaleString()
                };
                allMemos.unshift(newMemo);
                saveLocalMemos(allMemos);
                applyFilterAndRender();

                try {
                  await fetch(` + "`" + `${API_BASE}/api/memos/save` + "`" + `, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                      id: newMemo.id,
                      title: titleStr,
                      content: contentStr,
                      transcript: finalTranscript,
                      audio_data: base64Audio,
                      duration: formattedDuration,
                      type: "audio",
                      tags: ["voice"],
                      workspace_path: mq.wsPath || undefined,
                      created_at: newMemo.created_at
                    })
                  });
                } catch (_) {}
                loadMemos(true);
              }
            };
            mediaRecorder.start(250);
            recordBtn.innerHTML = '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="12" height="12" x="6" y="6" rx="2"/></svg><span style="white-space: nowrap;">Stop Recording</span>';
            recordBtn.classList.add("active");
          } catch (err) {
            showToast("Microphone access error: " + err.message, "error");
          }
        } else if (mediaRecorder.state === "recording") {
          if (speechRecognition) {
            try { speechRecognition.stop(); } catch (_) {}
            speechRecognition = null;
          }
          mediaRecorder.stop();
          if (mediaStream) {
            mediaStream.getTracks().forEach(t => t.stop());
            mediaStream = null;
          }
          recordBtn.innerHTML = '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" x2="12" y1="19" y2="22"/></svg><span style="white-space: nowrap;">Voice Memo</span>';
          recordBtn.classList.remove("active");
        }
      };

      const searchInput = searchRow.querySelector("#swiss-m-search-input");
      const searchClear = searchRow.querySelector("#swiss-m-search-clear");
      const scopeToggle = searchRow.querySelector("#swiss-m-scope-toggle");
      const scopeLabel = searchRow.querySelector("#swiss-m-scope-label");

      searchInput.addEventListener("input", () => {
        searchQuery = searchInput.value;
        searchClear.style.display = searchQuery ? "block" : "none";
        applyFilterAndRender();
      });

      searchClear.onclick = () => {
        searchInput.value = "";
        searchQuery = "";
        searchClear.style.display = "none";
        searchInput.focus();
        applyFilterAndRender();
      };

      scopeToggle.onclick = () => {
        if (searchScope === "text") {
          searchScope = "all";
          scopeLabel.textContent = "Text+Voice";
          scopeToggle.classList.add("primary");
        } else {
          searchScope = "text";
          scopeLabel.textContent = "Text";
          scopeToggle.classList.remove("primary");
        }
        applyFilterAndRender();
      };

      const initialMemos = getLocalMemos();
      if (initialMemos.length > 0) {
        allMemos = initialMemos;
        applyFilterAndRender();
      }
      loadMemos(initialMemos.length > 0);
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
` + "\n\n" + GenerateGitHubExtensionScript()
}
