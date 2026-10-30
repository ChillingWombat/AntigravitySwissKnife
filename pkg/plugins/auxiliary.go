package plugins

import "github.com/ChillingWombat/antigravity-swiss-knife/pkg/injected"

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
/* Auxiliary Active State Isolation: When a Swiss tab is active, suppress native factory tab highlights in header */
[data-swiss-aux-header][data-swiss-aux-active] button[data-tab-id]:not([data-tab-id^="swiss-"]),
[data-swiss-aux-header][data-swiss-aux-active] button:not(.swiss-aux-tab-btn):not([data-tab-id^="swiss-"]),
[data-swiss-aux-panel][data-swiss-aux-active] > [data-swiss-aux-header] button:not(.swiss-aux-tab-btn):not([data-tab-id^="swiss-"]),
[data-swiss-aux-active]:not(#swiss-aux-container):not(#swiss-aux-container *) button[data-tab-id]:not([data-tab-id^="swiss-"]) {
  background-color: transparent !important;
  border-color: transparent !important;
  color: var(--secondary-foreground, #71717a) !important;
  box-shadow: none !important;
}
[data-swiss-aux-header][data-swiss-aux-active] button[data-tab-id]:not([data-tab-id^="swiss-"]):hover,
[data-swiss-aux-header][data-swiss-aux-active] button:not(.swiss-aux-tab-btn):not([data-tab-id^="swiss-"]):hover,
[data-swiss-aux-active]:not(#swiss-aux-container):not(#swiss-aux-container *) button[data-tab-id]:not([data-tab-id^="swiss-"]):hover {
  background-color: var(--muted, rgba(148, 163, 184, 0.15)) !important;
  color: var(--foreground, #1e293b) !important;
}
:is(.dark, [data-theme="dark"]) [data-swiss-aux-header][data-swiss-aux-active] button[data-tab-id]:not([data-tab-id^="swiss-"]):hover,
:is(.dark, [data-theme="dark"]) [data-swiss-aux-header][data-swiss-aux-active] button:not(.swiss-aux-tab-btn):not([data-tab-id^="swiss-"]):hover,
:is(.dark, [data-theme="dark"]) [data-swiss-aux-active]:not(#swiss-aux-container):not(#swiss-aux-container *) button[data-tab-id]:not([data-tab-id^="swiss-"]):hover {
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

/* 1. Fixed Left Factory Buttons (Overview, Review, Terminal) — only icon-only buttons without text */
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div:first-child {
  flex-shrink: 0 !important;
  display: flex !important;
  align-items: center !important;
}
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div:first-child button:not([role="tab"]):not(:has(span, p, .text-xs, [class*="title"], [class*="label"])) {
  width: 24px !important;
  min-width: 24px !important;
  max-width: 24px !important;
  height: 24px !important;
  padding: 0 !important;
  flex-shrink: 0 !important;
  justify-content: center !important;
}

/* 1b. Expanded Auxiliary Conversation / Subagent Tabs */
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div:first-child [role="tab"],
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div:first-child button:has(span, p, .text-xs, [class*="title"], [class*="label"]),
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) [role="tab"]:not(.swiss-aux-tab-btn) {
  width: auto !important;
  min-width: max-content !important;
  max-width: 280px !important;
  height: 24px !important;
  padding: 0 8px !important;
  flex-shrink: 0 !important;
  display: inline-flex !important;
  align-items: center !important;
  gap: 6px !important;
  white-space: nowrap !important;
}

/* 2. Flexible Middle Section: File Tabs & Plus Button */
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div.w-px,
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div.h-4.w-px {
  width: 1px !important;
  min-width: 1px !important;
  max-width: 1px !important;
  flex: 0 0 1px !important;
  flex-shrink: 0 !important;
  flex-grow: 0 !important;
}
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div.overflow-x-auto,
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > div[class*="overflow-x-auto"] {
  flex: 1 1 0% !important;
  min-width: 0 !important;
  overflow: hidden !important;
}
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > button[aria-label="Add"],
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > [data-testid="aux-panel-plus-dropdown-trigger"],
.shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > button:not(.swiss-aux-tab-btn):not([role="tab"]):not(:has(span, p)) {
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
  .shrink-0.flex.items-center.border-b:has(.swiss-aux-btn-group) > button:not(.swiss-aux-tab-btn):not([role="tab"]):not(:has(span, p)) {
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

/* Community Extension View */
.swiss-community-ext-view {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  box-sizing: border-box;
}
.swiss-community-ext-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 10px;
  background: var(--canvas-subtle, #f8fafc);
  border-bottom: 1px solid var(--border, #e2e8f0);
  min-width: 0;
  box-sizing: border-box;
  flex-shrink: 0;
}
:is(.dark, [data-theme="dark"]) .swiss-community-ext-toolbar {
  background: #1e293b;
  border-bottom-color: #334155;
}
.swiss-community-ext-frame {
  flex: 1;
  width: 100%;
  height: 100%;
  border: none;
  background: #ffffff;
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
.swiss-builtin-browser-view {
  position: relative;
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
  background: #ea4335 !important;
  color: #ffffff !important;
  border-color: #ea4335 !important;
  box-shadow: 0 0 0 2px rgba(234, 67, 53, 0.35) !important;
  font-weight: 600;
}
.swiss-browser-btn.active svg {
  stroke: #ffffff !important;
}
:is(.dark, [data-theme="dark"]) .swiss-browser-btn.active {
  background: #ea4335 !important;
  color: #ffffff !important;
  border-color: #ea4335 !important;
  box-shadow: 0 0 0 2px rgba(234, 67, 53, 0.45) !important;
  font-weight: 600;
}
:is(.dark, [data-theme="dark"]) .swiss-browser-btn.active svg {
  stroke: #ffffff !important;
}
#swiss-b-touch.active {
  background: rgba(26, 115, 232, 0.15) !important;
  color: #1a73e8 !important;
  border-color: #1a73e8 !important;
  box-shadow: 0 0 0 1px #1a73e8 !important;
  font-weight: 600;
}
:is(.dark, [data-theme="dark"]) #swiss-b-touch.active {
  background: rgba(56, 189, 248, 0.2) !important;
  color: #38bdf8 !important;
  border-color: #38bdf8 !important;
  box-shadow: 0 0 0 1px #38bdf8 !important;
  font-weight: 600;
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
/* Landscape orientation: swapped width/height for fixed device presets */
.swiss-device-frame.swiss-landscape.frame-iphone-16-pro {
  width: 896px;
  height: 424px;
}
.swiss-device-frame.swiss-landscape.frame-iphone-16-pro .swiss-device-screen {
  width: 874px;
  height: 402px;
}
.swiss-device-frame.swiss-landscape.frame-pixel-9 {
  width: 944px;
  height: 432px;
}
.swiss-device-frame.swiss-landscape.frame-pixel-9 .swiss-device-screen {
  width: 924px;
  height: 412px;
}
.swiss-device-frame.swiss-landscape.frame-ipad {
  width: 1208px;
  height: 848px;
}
.swiss-device-frame.swiss-landscape.frame-ipad .swiss-device-screen {
  width: 1180px;
  height: 820px;
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

/* Element-Specific Annotation Popover */
.swiss-element-annotation-box {
  position: absolute;
  z-index: 70;
  width: 380px;
  max-width: calc(100% - 16px);
  background: var(--canvas, #ffffff);
  border: 1px solid var(--border, #cbd5e1);
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12), 0 0 0 1px rgba(0, 0, 0, 0.05);
  padding: 10px 12px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
:is(.dark, [data-theme="dark"]) .swiss-element-annotation-box {
  background: var(--card, #1e293b);
  border-color: var(--border, #334155);
  color: #f8fafc;
}
.swiss-element-annotation-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}
.swiss-element-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(234, 67, 53, 0.1);
  color: #ea4335;
  font-size: 11px;
  font-family: monospace;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 290px;
}
.swiss-element-annotation-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  height: 22px;
  padding: 0 8px;
  border-radius: 4px;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--surface, #f8fafd);
  color: var(--text-muted, #64748b);
  cursor: pointer;
  font-size: 11px;
  font-weight: 500;
  transition: all 0.15s;
  flex-shrink: 0;
  white-space: nowrap;
}
.swiss-element-annotation-close:hover {
  background: rgba(234, 67, 53, 0.1);
  color: #ea4335;
  border-color: rgba(234, 67, 53, 0.3);
}
:is(.dark, [data-theme="dark"]) .swiss-element-annotation-close {
  background: var(--surface, #1e293b);
  border-color: var(--border, #334155);
  color: #94a3b8;
}
:is(.dark, [data-theme="dark"]) .swiss-element-annotation-close:hover {
  background: rgba(234, 67, 53, 0.2);
  color: #f87171;
  border-color: rgba(234, 67, 53, 0.4);
}
.swiss-element-annotation-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.swiss-element-annotation-input {
  width: 100%;
  box-sizing: border-box;
  min-height: 72px;
  height: 110px;
  resize: vertical;
  padding: 8px 10px;
  border-radius: 4px;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  font-size: 12px;
  line-height: 1.4;
  font-family: inherit;
  outline: none;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.swiss-element-annotation-input:focus {
  border-color: #1a73e8;
  box-shadow: 0 0 0 2px rgba(26, 115, 232, 0.15);
}
:is(.dark, [data-theme="dark"]) .swiss-element-annotation-input {
  background: var(--canvas, #0f172a);
  border-color: var(--border, #334155);
  color: #f8fafc;
}
.swiss-element-annotation-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}
.swiss-element-annotation-send {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 12px;
  border-radius: 4px;
  border: 1px solid #1a73e8;
  background: #1a73e8;
  color: #ffffff;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  flex-shrink: 0;
  transition: background 0.15s;
}
.swiss-element-annotation-send:hover {
  background: #1557b0;
  border-color: #1557b0;
}

/* In-Chat Element Annotation Removable Chip */
.swiss-chat-annotation-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin: 4px 0 6px 0;
  padding: 4px 8px 4px 10px;
  border-radius: 6px;
  background: rgba(234, 67, 53, 0.08);
  border: 1px solid rgba(234, 67, 53, 0.25);
  color: var(--text, #1e293b);
  font-size: 12px;
  max-width: 100%;
  box-sizing: border-box;
}
:is(.dark, [data-theme="dark"]) .swiss-chat-annotation-chip {
  background: rgba(234, 67, 53, 0.15);
  border-color: rgba(234, 67, 53, 0.35);
  color: #f8fafc;
}
.swiss-chat-chip-content {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  overflow: hidden;
}
.swiss-chat-chip-content svg {
  stroke: #ea4335;
  flex-shrink: 0;
}
.swiss-chat-chip-selector {
  font-family: monospace;
  font-size: 11px;
  font-weight: 700;
  color: #ea4335;
  white-space: nowrap;
}
.swiss-chat-chip-comment {
  color: var(--text-muted, #64748b);
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:is(.dark, [data-theme="dark"]) .swiss-chat-chip-comment {
  color: #cbd5e1;
}
.swiss-chat-chip-delete {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 3px;
  border: none;
  background: transparent;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  padding: 0;
  flex-shrink: 0;
  transition: all 0.15s;
}
.swiss-chat-chip-delete:hover {
  background: rgba(234, 67, 53, 0.2);
  color: #ea4335;
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
.swiss-files-path-wrap {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
}
.swiss-files-path-input {
  flex: 1;
  min-width: 0;
  padding: 4px 26px 4px 8px;
  font-size: 11px;
  border-radius: 4px;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  font-family: monospace;
}
.swiss-files-path-caret {
  position: absolute;
  right: 2px;
  top: 50%;
  transform: translateY(-50%);
  width: 20px;
  height: 20px;
  padding: 2px;
  border: none;
  background: transparent;
  color: #64748b;
}
.swiss-files-path-caret:hover,
.swiss-files-path-caret[aria-expanded="true"] {
  background: rgba(148, 163, 184, 0.22);
  color: #1e293b;
}
.swiss-dir-jump-menu {
  /* Fixed + appended to document.body: the file view clips its overflow, and the
     menu must escape that clipping context when the panel is short. */
  position: fixed;
  z-index: 99999;
  min-width: 280px;
  max-width: min(440px, 92vw);
  max-height: 60vh;
  overflow-y: auto;
  padding: 4px;
  background: var(--canvas, #ffffff);
  border: 1px solid var(--border, #cbd5e1);
  border-radius: 8px;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.18);
  font-size: 11px;
  user-select: none;
  -webkit-user-select: none;
}
.swiss-dir-jump-group + .swiss-dir-jump-group {
  margin-top: 4px;
  padding-top: 4px;
  border-top: 1px solid rgba(226, 232, 240, 0.9);
}
.swiss-dir-jump-group-label {
  padding: 4px 8px 2px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #94a3b8;
}
.swiss-dir-jump-list {
  max-height: 168px;
  overflow-y: auto;
}
.swiss-dir-jump-item {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 5px 8px;
  border-radius: 5px;
  color: var(--text, #1e293b);
  cursor: pointer;
  white-space: nowrap;
}
.swiss-dir-jump-item:hover,
.swiss-dir-jump-item:focus {
  background: rgba(26, 115, 232, 0.09);
  outline: none;
}
.swiss-dir-jump-item.active,
.swiss-dir-jump-item[aria-disabled="true"] {
  background: rgba(26, 115, 232, 0.08);
  color: #1a73e8;
  font-weight: 600;
  cursor: default;
}
.swiss-dir-jump-item svg {
  flex-shrink: 0;
}
.swiss-dir-jump-name {
  overflow: hidden;
  text-overflow: ellipsis;
}
.swiss-dir-jump-arrow {
  margin-left: auto;
  opacity: 0.45;
}
.swiss-dir-jump-loading,
.swiss-dir-jump-empty {
  padding: 5px 8px;
  color: #94a3b8;
  font-style: italic;
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
.swiss-context-item:hover:not(.disabled),
.swiss-context-item:focus-visible:not(.disabled) {
  background: rgba(26, 115, 232, 0.08);
  color: #1a73e8;
  outline: none;
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
  /* Issue #145: the view is its own container so the narrow-panel rule below
     tracks the width the memos actually get, in the auxiliary panel and in the
     main stage alike. */
  container-type: inline-size;
  container-name: swissmemo;
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
/* Memo cards are fluid between a minimum width and just under 2x that minimum.
   The host IDE can shrink the right auxiliary panel down to 285px - the
   narrowest width the auxiliary tab header supports (see the
   @container (max-width: 285px) rule above). At that width the memos view has
   285px - 2 * 12px (view padding) - 12px (vertical scrollbar allowance) = 249px
   of content, so 249px is the card minimum and a card never overflows the panel
   horizontally (min(..., 100%) still covers panels narrower than that).
   auto-fill + 1fr builds the requested sawtooth: a single column grows with the
   panel until a second minimum-width column fits (249px * 2 + 8px gap = 506px),
   then both columns reset to the minimum; the same repeats for 3, 4, ...
   columns. */
.swiss-memo-list {
  --swiss-aux-panel-min-width: 285px;
  --swiss-memo-view-padding: 12px;
  --swiss-memo-scrollbar-allowance: 12px;
  --swiss-memo-card-min-width: calc(var(--swiss-aux-panel-min-width) - 2 * var(--swiss-memo-view-padding) - var(--swiss-memo-scrollbar-allowance));
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(var(--swiss-memo-card-min-width), 100%), 1fr));
  gap: 8px;
  align-items: start;
}
.swiss-memo-list > .swiss-memo-empty-search,
.swiss-memo-list > .swiss-memo-list-status {
  grid-column: 1 / -1;
}
/* Memo type filter (in the search row) */
.swiss-memo-filter-wrap {
  position: relative;
  flex-shrink: 0;
}
.swiss-memo-filter-menu {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  z-index: 60;
  min-width: 148px;
  flex-direction: column;
  gap: 2px;
  padding: 4px;
  border-radius: 6px;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.14);
}
.swiss-memo-filter-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 5px 8px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text, #1e293b);
  font-size: 11px;
  text-align: left;
  cursor: pointer;
}
.swiss-memo-filter-item:hover {
  background: rgba(148, 163, 184, 0.15);
}
.swiss-memo-filter-item.active {
  background: rgba(26, 115, 232, 0.12);
  color: #1a73e8;
  font-weight: 600;
}
:is(.dark, [data-theme="dark"]) .swiss-memo-filter-menu {
  background: var(--card, #1e293b);
  border-color: var(--border, #334155);
}
:is(.dark, [data-theme="dark"]) .swiss-memo-filter-item {
  color: #f8fafc;
}
.swiss-memo-filter-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 14px;
  height: 14px;
  padding: 0 4px;
  border-radius: 7px;
  background: rgba(26, 115, 232, 0.15);
  color: #1a73e8;
  font-size: 9px;
  font-weight: 600;
  line-height: 1;
}
:is(.dark, [data-theme="dark"]) .swiss-memo-filter-count {
  background: rgba(96, 165, 250, 0.25);
  color: #bfdbfe;
}
.swiss-memo-card {
  box-sizing: border-box;
  /* The grid decides the width (all cards in a row stay equal) and the height
     follows the golden ratio of the real column width. Without the cap below a
     lone card would reach 2 * min + 8px (column gap) - 1px, i.e. more than twice
     the minimum, before auto-fill opens the second column. */
  width: auto;
  max-width: min(100%, calc(2 * var(--swiss-memo-card-min-width) - 1px));
  aspect-ratio: 1.618 / 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
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
.swiss-memo-card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
  flex-shrink: 0;
}
.swiss-memo-card-title-wrap {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
  min-width: 0;
}
.swiss-memo-card-title {
  font-weight: 600;
  font-size: 11px;
  white-space: nowrap;
  text-overflow: ellipsis;
  overflow: hidden;
}
.swiss-memo-card-duration {
  font-size: 9px;
  color: #b06000;
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
}
.swiss-memo-card-date {
  font-size: 9px;
  color: #94a3b8;
  white-space: nowrap;
  flex-shrink: 0;
}
/* Body keeps the fixed card height: anything that does not fit is clipped. */
.swiss-memo-card-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 11px;
  line-height: 1.35;
  color: var(--text, #1e293b);
}
.swiss-memo-card-text {
  flex: 0 1 auto;
  min-height: 0;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 4;
  overflow: hidden;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
/* A real transcript must stay visible when the body also holds an image/caption. */
.swiss-memo-card-img-wrap ~ .swiss-memo-card-transcript {
  flex: 0 0 auto;
  max-height: 70%;
}
/* Issue #143: daemon-side auto extraction is async, so a freshly saved card
   shows a muted pending line (same line-clamp container) until the transcript
   arrives — it is not an error state. */
.swiss-memo-card-extract-pending {
  color: var(--text-muted, #64748b);
  font-style: italic;
}
.swiss-memo-card-img-wrap {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  max-height: 100%;
  border-radius: 4px;
  overflow: hidden;
  border: 1px solid var(--border, #cbd5e1);
  background: #ffffff;
  box-sizing: border-box;
}
.swiss-memo-card-img {
  width: 100%;
  height: 100%;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  display: block;
}
.swiss-memo-card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  margin-top: 6px;
  flex-shrink: 0;
}
.swiss-memo-card-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}
/* Voice player row (issue #136) */
.swiss-memo-player {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 4px 0 2px;
  flex-shrink: 0;
}
.swiss-memo-play {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border-radius: 50%;
  border: 1px solid var(--border, #cbd5e1);
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  cursor: pointer;
  flex-shrink: 0;
}
.swiss-memo-play:hover {
  border-color: #1a73e8;
  color: #1a73e8;
}
.swiss-memo-progress {
  position: relative;
  flex: 1 1 auto;
  min-width: 0;
  height: 6px;
  border-radius: 3px;
  background: rgba(148, 163, 184, 0.3);
  cursor: pointer;
  touch-action: none;
}
.swiss-memo-progress-fill {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 0;
  border-radius: 3px;
  background: #1a73e8;
  pointer-events: none;
}
.swiss-memo-time {
  font-size: 9px;
  color: var(--text-muted, #64748b);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  flex-shrink: 0;
}
/* Issue #145: at the narrowest panel width the loudness wave squeezes the
   record button and overlaps neighbouring buttons. Drop the wave bars only;
   the icon and elapsed timer stay visible at every width because they are
   what the user needs while recording. */
@container swissmemo (max-width: 285px) {
  .swiss-memo-record-wave {
    display: none !important;
  }
}
.swiss-memo-extract-btn {
  padding: 2px 6px;
  font-size: 10px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.swiss-memo-extract-btn[disabled] {
  opacity: 0.65;
  cursor: default;
}
.swiss-memo-extract-spinner {
  display: inline-block;
  width: 9px;
  height: 9px;
  box-sizing: border-box;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: swiss-memo-spin 0.7s linear infinite;
}
@keyframes swiss-memo-spin {
  to { transform: rotate(360deg); }
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
  width: 460px;
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
textarea.swiss-prompt-input {
  min-height: 120px;
  resize: vertical;
  line-height: 1.4;
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

/* In-Chat Real Telemetry Metrics (At Left End of Chat Action Row) */
[data-testid="conversation-view"] .swiss-inchat-metrics {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 11px;
  color: var(--text-secondary, #94a3b8) !important;
  font-variant-numeric: tabular-nums;
  user-select: none;
  white-space: nowrap;
  line-height: 1.5;
  margin-right: auto;
  padding: 2px 0;
  vertical-align: middle;
}
html:not(.swiss-daemon-online) .swiss-inchat-metrics,
html.swiss-daemon-offline .swiss-inchat-metrics,
html:not(.swiss-daemon-online) #swiss-context-window-ring,
html.swiss-daemon-offline #swiss-context-window-ring,
html:not(.swiss-daemon-online) [id^="swiss-context-window-ring-sub-"],
html.swiss-daemon-offline [id^="swiss-context-window-ring-sub-"],
html:not(.swiss-daemon-online) .swiss-context-window-ring,
html.swiss-daemon-offline .swiss-context-window-ring,
html:not(.swiss-daemon-online) .swiss-context-window-ring-sub,
html.swiss-daemon-offline .swiss-context-window-ring-sub,
[role="dialog"] .swiss-inchat-metrics,
[aria-modal="true"] .swiss-inchat-metrics,
.settings-tab-content .swiss-inchat-metrics,
#swiss-aux-container .swiss-inchat-metrics,
pre .swiss-inchat-metrics,
code .swiss-inchat-metrics {
  display: none !important;
}
.swiss-inchat-metrics .metric-item {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: var(--text-secondary, #94a3b8) !important;
  font-weight: 400 !important;
}
.swiss-inchat-metrics .metric-val {
  font-weight: 500 !important;
  color: var(--text-secondary, #94a3b8) !important;
}
.swiss-inchat-metrics .metric-sep {
  color: var(--text-secondary, #94a3b8) !important;
  opacity: 0.5 !important;
  margin: 0 2px;
}
[data-theme="dark"] .swiss-inchat-metrics,
.dark .swiss-inchat-metrics,
[data-theme="dark"] .swiss-inchat-metrics .metric-item,
.dark .swiss-inchat-metrics .metric-item,
[data-theme="dark"] .swiss-inchat-metrics .metric-val,
.dark .swiss-inchat-metrics .metric-val,
[data-theme="dark"] .swiss-inchat-metrics .metric-sep,
.dark .swiss-inchat-metrics .metric-sep {
  color: #94a3b8 !important;
}
.swiss-telemetry-badge {
  display: none !important;
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

/* WYSIWYG Markdown Editor */
.swiss-wysiwyg-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--canvas, #ffffff);
}
.swiss-wysiwyg-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
  outline: none;
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  line-height: 1.6;
  font-size: 13px;
  box-sizing: border-box;
}
.swiss-wysiwyg-content h1 { font-size: 20px; font-weight: 700; margin: 12px 0 6px; }
.swiss-wysiwyg-content h2 { font-size: 17px; font-weight: 700; margin: 10px 0 5px; }
.swiss-wysiwyg-content h3 { font-size: 15px; font-weight: 600; margin: 8px 0 4px; }
.swiss-wysiwyg-content h4 { font-size: 14px; font-weight: 600; margin: 6px 0 4px; }
.swiss-wysiwyg-content h5 { font-size: 13px; font-weight: 600; margin: 6px 0 2px; }
.swiss-wysiwyg-content h6 { font-size: 12px; font-weight: 600; margin: 4px 0 2px; }
.swiss-wysiwyg-content p { margin: 0 0 8px; }
.swiss-wysiwyg-content ul, .swiss-wysiwyg-content ol { padding-left: 20px; margin: 0 0 8px; }
.swiss-wysiwyg-content li { margin-bottom: 3px; }
.swiss-wysiwyg-content blockquote {
  border-left: 2px solid var(--border, #cbd5e1);
  padding-left: 10px;
  margin: 6px 0;
  color: var(--text-muted, #64748b);
}
.swiss-wysiwyg-content pre {
  background: var(--card, #f1f5f9);
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid var(--border, #e2e8f0);
  font-family: monospace;
  font-size: 11px;
  margin: 6px 0;
  overflow-x: auto;
}
.swiss-wysiwyg-content code {
  background: var(--card, #f1f5f9);
  padding: 2px 4px;
  border-radius: 3px;
  font-family: monospace;
  font-size: 11px;
}
` + "\n" + GenerateGitHubExtensionCSS()
}

// GenerateAuxiliaryPluginsScript returns the full client-side JavaScript injected into Antigravity 2.0
// that mounts the auxiliary tabs, live browser previewer, file explorer with in-place editors, quick memos,
// and in-chat token telemetry badges.
func GenerateAuxiliaryPluginsScript() string {
	var tokenScript string
	// Install the broker bridge before any injected module issues a fetch.
	tokenScript += SwissAuthInterceptorScript
	tokenScript += string(injected.RuntimeBundle())

	return tokenScript + `(() => {
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
        if (typeof existingAuxContainer.__swissTabTeardown === "function") {
          try { existingAuxContainer.__swissTabTeardown(); } catch (_) {}
          existingAuxContainer.__swissTabTeardown = null;
        }
        delete existingAuxContainer.dataset.renderedTab;
      }
    }
    window.__swissAuxiliaryInitialized = true;
    if (typeof window.__swissDaemonOnline === "undefined") {
      window.__swissDaemonOnline = false;
    }

    function getSwissAuthHeaders(customHeaders = {}) {
      return { "Content-Type": "application/json", ...customHeaders };
    }

    const API_BASE = "http://127.0.0.1:8765";
    let activeAuxTab = null; // "swiss-browser" | "swiss-files" | "swiss-memos" | null (native)
    let currentBrowserUrl = "http://localhost:8765";
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
    let deviceOrientation = "portrait";
    try {
      const savedOrientation = localStorage.getItem("antigravity_swiss_browser_orientation");
      if (savedOrientation === "landscape" || savedOrientation === "portrait") {
        deviceOrientation = savedOrientation;
      }
    } catch (_) {}
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
      window.removeEventListener("scroll", removeContextMenu, true);
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
        window.addEventListener("scroll", removeContextMenu, true);
      }, 10);
    }

    window.__swissOnAuxDaemonChanged = function(online) {
      if (!online) {
        document.querySelectorAll(".swiss-aux-btn-group, .swiss-aux-tabs-divider, .swiss-aux-tabs-divider-left, .swiss-aux-tabs-divider-right").forEach(el => el.remove());
        document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
        chatTelemetryCachedData = null;
        chatTelemetryLastFetch = 0;
        if (activeAuxTab !== null) {
          switchAuxTab(null);
        }
      } else {
        chatTelemetryCachedData = null;
        chatTelemetryLastFetch = 0;
        setupAuxiliaryTabs();
        setupInChatTelemetry();
      }
    };
    window.setupAuxiliaryTabs = function() { setupAuxiliaryTabs(); };

    function escapeHtml(str) {
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
    }

    function getCommunityExtensions() {
      try {
        let list = null;
        if (window.__SWISS_ENH_CONFIG__ && Array.isArray(window.__SWISS_ENH_CONFIG__.community_extensions)) {
          list = window.__SWISS_ENH_CONFIG__.community_extensions;
        } else {
          const raw = localStorage.getItem("antigravity_swiss_community_extensions");
          if (raw) list = JSON.parse(raw);
        }
        if (Array.isArray(list)) {
          return list.filter(ext => ext && ext.id && ext.enabled !== false);
        }
      } catch (_) {}
      return [];
    }

    function getCommunityExtensionSvg(icon, cls) {
      const c = cls ? ' class="' + cls + '"' : '';
      const key = (icon || "").toLowerCase().trim();
      switch (key) {
        case "globe":
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>';
        case "code":
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>';
        case "terminal":
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>';
        case "database":
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/><path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/></svg>';
        case "bot":
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>';
        case "wrench":
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>';
        case "book-open":
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/></svg>';
        case "layers":
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/></svg>';
        case "cpu":
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect width="16" height="16" x="4" y="4" rx="2"/><rect width="6" height="6" x="9" y="9" rx="1"/><path d="M15 2v2"/><path d="M15 20v2"/><path d="M2 15h2"/><path d="M2 9h2"/><path d="M20 15h2"/><path d="M20 9h2"/><path d="M9 2v2"/><path d="M9 20v2"/></svg>';
        case "blocks":
        default:
          return '<svg' + c + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect width="7" height="7" x="14" y="3" rx="1"/><path d="M10 21V8a1 1 0 0 0-1-1H4a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-5a1 1 0 0 0-1-1H3"/></svg>';
      }
    }

    window.getSwissCommunityExtensionSvg = getCommunityExtensionSvg;
    window.getSwissCommunityExtensions = getCommunityExtensions;

    // 1. Auxiliary Panel Tab Injector Engine
    function setupAuxiliaryTabs() {
      if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline) {
        document.querySelectorAll(".swiss-aux-btn-group, .swiss-aux-tabs-divider, .swiss-aux-tabs-divider-left, .swiss-aux-tabs-divider-right").forEach(el => el.remove());
        if (activeAuxTab !== null) {
          switchAuxTab(null);
        }
        return;
      }
      // Find auxiliary panel header strictly matching .shrink-0.flex.items-center[class*="gap-0.5"].border-b
      const tabHeader = document.querySelector('[data-testid="auxiliary-panel"] .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.part.auxiliarybar .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('[data-testid="auxiliary-panel"] .shrink-0.flex.items-center.border-b') ||
                        document.querySelector('.part.auxiliarybar .shrink-0.flex.items-center.border-b') ||
                        document.querySelector('.shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.shrink-0.flex.items-center.border-b');
      if (!tabHeader) return;

      if (tabHeader.parentElement) {
        if (tabHeader.parentElement.getAttribute('data-swiss-aux-panel') !== 'true') {
          tabHeader.parentElement.setAttribute('data-swiss-aux-panel', 'true');
        }
      }
      if (tabHeader.getAttribute('data-swiss-aux-header') !== 'true') {
        tabHeader.setAttribute('data-swiss-aux-header', 'true');
      }

      // Per-extension aux_panel switches (EnhancementsConfig.extensions) decide
      // which Swiss tabs appear in the auxiliary panel. Legacy localStorage
      // fallbacks apply only when no visibility map is available.
      function isAuxExtensionVisible(id, extObj) {
        try {
          const cfg = window.__SWISS_ENH_CONFIG__;
          if (cfg && cfg.extensions && typeof cfg.extensions === "object") {
            const vis = cfg.extensions[id];
            if (vis && typeof vis.aux_panel === "boolean") {
              return vis.aux_panel;
            }
          }
        } catch (_) {}
        if (extObj && extObj.isCommunity && extObj.ext) {
          return extObj.ext.aux_panel !== false;
        }
        try {
          const disabledExts = JSON.parse(localStorage.getItem("antigravity_swiss_disabled_extensions") || "[]");
          if (Array.isArray(disabledExts) && disabledExts.includes(id)) return false;
          if (localStorage.getItem("antigravity_swiss_ext_" + id + "_enabled") === "false") return false;
        } catch (_) {}
        return true;
      }

      // Define Swiss tabs with Antigravity-matching monochrome SVG stroke icons
      const allTabs = [
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

      const commExts = getCommunityExtensions();
      commExts.forEach(ext => {
        allTabs.push({
          id: ext.id,
          tabId: "swiss-" + ext.id,
          label: ext.name || ext.id,
          svg: getCommunityExtensionSvg(ext.icon, "swiss-aux-tab-svg"),
          isCommunity: true,
          ext: ext
        });
      });

      // Honor per-extension aux_panel switches; drop tabs the user disabled.
      const tabs = allTabs.filter(t => isAuxExtensionVisible(t.id, t));
      if (tabs.length === 0) {
        document.querySelectorAll(".swiss-aux-btn-group, .swiss-aux-tabs-divider, .swiss-aux-tabs-divider-left, .swiss-aux-tabs-divider-right").forEach(el => el.remove());
        if (activeAuxTab !== null) {
          switchAuxTab(null);
        }
        return;
      }

      function getAuxTabFormat() {
        return localStorage.getItem("antigravity_swiss_aux_tab_format") ||
               (window.__SWISS_ENH_CONFIG__ && window.__SWISS_ENH_CONFIG__.overview_panel && window.__SWISS_ENH_CONFIG__.overview_panel.aux_tabs_format) ||
               "icon";
      }

      function updateTabButtonMarkup(btn, t, fmt) {
        const titleText = "Antigravity Swiss Knife: " + t.label;
        if (btn.title !== titleText) btn.title = titleText;
        if (fmt === "icon_and_name") {
          if (btn.classList.contains("icon-only")) btn.classList.remove("icon-only");
          if (!btn.querySelector(".swiss-aux-tab-label")) {
            btn.innerHTML = t.svg + '<span class="swiss-aux-tab-label">' + t.label + '</span>';
          }
        } else {
          if (!btn.classList.contains("icon-only")) btn.classList.add("icon-only");
          if (btn.querySelector(".swiss-aux-tab-label") || !btn.querySelector("svg")) {
            btn.innerHTML = t.svg;
          }
        }
      }

      // Configure tabs matching data-tab-id="swiss-files", data-tab-id="swiss-memos", data-tab-id="swiss-github"
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
          } else {
            // Extension hidden by its aux_panel switch — drop the stale button.
            btn.remove();
          }
        });

        // Ensure left and right dividers exist even if buttons already present
        const curBtnGroup = tabHeader.querySelector('.swiss-aux-btn-group');
        if (curBtnGroup) {
          tabs.forEach(t => {
            if (!curBtnGroup.querySelector('[data-swiss-tab="' + t.id + '"]')) {
              const btn = document.createElement("button");
              btn.className = "swiss-aux-tab-btn";
              if (t.id === "files") btn.setAttribute("data-tab-id", "swiss-files");
              else if (t.id === "memos") btn.setAttribute("data-tab-id", "swiss-memos");
              else if (t.id === "github") btn.setAttribute("data-tab-id", "swiss-github");
              else btn.setAttribute("data-tab-id", t.tabId || ("swiss-" + t.id));
              btn.dataset.swissTab = t.id;
              updateTabButtonMarkup(btn, t, curFmt);
              btn.onclick = (e) => {
                e.stopPropagation();
                switchAuxTab(t.tabId);
              };
              curBtnGroup.appendChild(btn);
            }
          });

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

      } else {
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
          // Contract: data-tab-id="swiss-files" data-tab-id="swiss-memos" data-tab-id="swiss-github"
          if (t.id === "files") btn.setAttribute("data-tab-id", "swiss-files");
          else if (t.id === "memos") btn.setAttribute("data-tab-id", "swiss-memos");
          else if (t.id === "github") btn.setAttribute("data-tab-id", "swiss-github");
          else btn.setAttribute("data-tab-id", t.tabId || ("swiss-" + t.id));
          btn.dataset.swissTab = t.id;
          updateTabButtonMarkup(btn, t, curFmt);
          const activeTabTarget = activeAuxTab || localStorage.getItem("antigravity_active_aux_tab");
          if (activeTabTarget && (btn.getAttribute("data-tab-id") === activeTabTarget || t.id === activeTabTarget || ("swiss-" + t.id) === activeTabTarget)) {
            btn.classList.add("active");
            btn.setAttribute("aria-selected", "true");
          }
          btn.onclick = (e) => {
            e.stopPropagation();
            switchAuxTab(t.tabId, true);
          };
          btnGroup.appendChild(btn);
        });
      }

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
        const isNativeTab = targetBtn.hasAttribute("data-tab-id") || targetBtn.getAttribute("role") === "tab" || targetBtn.hasAttribute("data-tab-index");
        if (isNativeTab) {
          switchAuxTab(null);
        }
      };
      tabHeader.addEventListener("click", tabHeader.__swissHeaderHandler);

      // Restore / reconcile saved tab state from localStorage
      const savedTab = localStorage.getItem("antigravity_active_aux_tab") || activeAuxTab;
      if (savedTab && savedTab.startsWith("swiss-")) {
        switchAuxTab(savedTab, false);
      } else {
        if (tabHeader) tabHeader.removeAttribute("data-swiss-aux-active");
        const auxPanel = (tabHeader && tabHeader.closest('[data-testid="auxiliary-panel"], .part.auxiliarybar, [data-swiss-aux-panel]')) ||
                         (tabHeader ? tabHeader.parentElement : null) ||
                         document.querySelector('[data-testid="auxiliary-panel"]') ||
                         document.querySelector('.part.auxiliarybar');
        if (auxPanel) auxPanel.removeAttribute("data-swiss-aux-active");
        const swissContainer = document.querySelector("#swiss-aux-container");
        if (swissContainer) {
          swissContainer.style.display = "none";
        }
      }
    }

    // Switch between Swiss tabs and Native factory tabs
    function switchAuxTab(tabId, force = false) {
      const normalizedId = tabId ? (tabId.startsWith("swiss-") ? tabId : ("swiss-" + tabId)) : null;
      activeAuxTab = normalizedId;

      document.querySelectorAll(".swiss-aux-tab-btn").forEach(b => {
        const isActive = normalizedId !== null && b.getAttribute("data-tab-id") === normalizedId;
        b.classList.toggle("active", isActive);
        b.setAttribute("aria-selected", isActive ? "true" : "false");
      });

      const tabHeader = document.querySelector('[data-testid="auxiliary-panel"] .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.part.auxiliarybar .shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('[data-testid="auxiliary-panel"] .shrink-0.flex.items-center.border-b') ||
                        document.querySelector('.part.auxiliarybar .shrink-0.flex.items-center.border-b') ||
                        document.querySelector('.shrink-0.flex.items-center[class*="gap-0.5"].border-b') ||
                        document.querySelector('.shrink-0.flex.items-center.border-b');
      const auxPanel = (tabHeader && tabHeader.closest('[data-testid="auxiliary-panel"], .part.auxiliarybar, [data-swiss-aux-panel]')) ||
                       (tabHeader ? tabHeader.parentElement : null) ||
                       document.querySelector('[data-testid="auxiliary-panel"]') ||
                       document.querySelector('.part.auxiliarybar');
      const bodyContainer = (tabHeader && tabHeader.nextElementSibling) ||
                            (auxPanel ? auxPanel.querySelector('.flex-grow.overflow-hidden') : null) ||
                            (tabHeader && tabHeader.parentElement ? tabHeader.parentElement.querySelector('.flex-grow.overflow-hidden') : null) ||
                            document.querySelector('[data-testid="auxiliary-panel"] .flex-grow.overflow-hidden') ||
                            document.querySelector('.part.auxiliarybar .flex-grow.overflow-hidden');
      if (!bodyContainer) return;

      let swissContainer = bodyContainer.querySelector("#swiss-aux-container") || document.querySelector("#swiss-aux-container");
      if (!swissContainer) {
        swissContainer = document.createElement("div");
        swissContainer.id = "swiss-aux-container";
        swissContainer.style.display = "none";
        bodyContainer.appendChild(swissContainer);
      } else if (swissContainer.parentElement !== bodyContainer) {
        bodyContainer.appendChild(swissContainer);
      }

      const cleanId = normalizedId ? normalizedId.replace(/^swiss-/, "") : "";
      const otherChildrenVisible = Array.from(bodyContainer.children).some(child => child !== swissContainer && child.style.display !== "none");
      if (!force && normalizedId && swissContainer.dataset.renderedTab === cleanId && swissContainer.style.display === "flex" && swissContainer.children.length > 0 && !otherChildrenVisible) {
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
        if (swissContainer.dataset.renderedTab !== cleanId || swissContainer.children.length === 0 || force) {
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
      // Let the outgoing view release listeners, timers and media streams before its DOM is wiped.
      if (container && typeof container.__swissTabTeardown === "function") {
        try { container.__swissTabTeardown(); } catch (_) {}
        container.__swissTabTeardown = null;
      }
      container.dataset.renderedTab = tabId;
      container.innerHTML = "";
      if (tabId === "files") renderFilesView(container);
      else if (tabId === "memos") renderMemosView(container);
      else if (tabId === "github") {
        if (typeof window.renderSwissGitHubWorkspaceView === "function") {
          window.renderSwissGitHubWorkspaceView(container);
        }
      } else {
        const commExts = getCommunityExtensions();
        const commExt = commExts.find(e => e.id === tabId);
        if (commExt) {
          renderCommunityExtensionView(container, commExt);
        }
      }
    }

    function renderCommunityExtensionView(container, ext) {
      if (!container || !ext) return;
      const wrap = document.createElement("div");
      wrap.className = "swiss-community-ext-view";

      const toolbar = document.createElement("div");
      toolbar.className = "swiss-community-ext-toolbar";

      const titleGroup = document.createElement("div");
      titleGroup.style.display = "flex";
      titleGroup.style.alignItems = "center";
      titleGroup.style.gap = "8px";
      titleGroup.style.minWidth = "0";

      const iconSvg = getCommunityExtensionSvg(ext.icon, "swiss-aux-tab-svg");
      const author = ext.author ? escapeHtml(ext.author) : "Community";
      const version = ext.version ? escapeHtml(ext.version) : "1.0.0";
      titleGroup.innerHTML = iconSvg +
        '<span style="font-weight: 600; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">' + escapeHtml(ext.name || ext.id) + '</span>' +
        '<span style="font-size: 11px; color: var(--text-muted, #71717a); white-space: nowrap;">v' + version + ' · ' + author + '</span>';
      toolbar.appendChild(titleGroup);

      const actionsGroup = document.createElement("div");
      actionsGroup.style.display = "flex";
      actionsGroup.style.alignItems = "center";
      actionsGroup.style.gap = "6px";
      actionsGroup.style.flexShrink = "0";

      if (ext.source_type === "url" && ext.url) {
        const urlText = document.createElement("span");
        urlText.style.cssText = "font-size: 11px; color: var(--text-muted, #71717a); max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: monospace;";
        urlText.textContent = ext.url;
        actionsGroup.appendChild(urlText);

        const openBtn = document.createElement("button");
        openBtn.className = "swiss-browser-btn";
        openBtn.type = "button";
        openBtn.title = "Open URL in external browser";
        openBtn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg> Open';
        openBtn.onclick = () => window.open(ext.url, "_blank");
        actionsGroup.appendChild(openBtn);
      }

      const reloadBtn = document.createElement("button");
      reloadBtn.className = "swiss-browser-btn";
      reloadBtn.type = "button";
      reloadBtn.title = "Reload Extension";
      reloadBtn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"></polyline><polyline points="1 20 1 14 7 14"></polyline><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg> Reload';
      actionsGroup.appendChild(reloadBtn);

      toolbar.appendChild(actionsGroup);
      wrap.appendChild(toolbar);

      const iframe = document.createElement("iframe");
      iframe.className = "swiss-community-ext-frame";
      iframe.style.cssText = "flex: 1; width: 100%; height: 100%; border: none;";

      if (ext.source_type === "url") {
        iframe.src = ext.url || "about:blank";
        reloadBtn.onclick = () => {
          try { iframe.src = ext.url; } catch (_) {}
        };
      } else {
        iframe.setAttribute("sandbox", "allow-scripts allow-forms allow-modals");
        const srcDocContent = '<!DOCTYPE html><html><head><meta charset="utf-8"><style>body { margin: 0; padding: 12px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; color: #1e293b; background: #ffffff; } @media (prefers-color-scheme: dark) { body { color: #f1f5f9; background: #0f172a; } } ' + (ext.css || "") + '</style></head><body>' + (ext.html || "") + '<script>' + (ext.js || "") + '<\/script></body></html>';
        iframe.srcdoc = srcDocContent;
        reloadBtn.onclick = () => {
          iframe.srcdoc = srcDocContent;
        };
      }

      const onMessage = (e) => {
        if (e && e.data && e.data.type === "swiss-insert-chat" && typeof e.data.text === "string") {
          if (typeof insertTextToChatInput === "function") {
            insertTextToChatInput(e.data.text);
          }
        }
      };
      window.addEventListener("message", onMessage);
      container.__swissTabTeardown = () => {
        window.removeEventListener("message", onMessage);
      };

      wrap.appendChild(iframe);
      container.appendChild(wrap);
    }

    window.renderSwissBrowserView = renderBrowserView;
    window.renderSwissFilesView = renderFilesView;
    window.renderSwissMemosView = renderMemosView;
    window.renderSwissCommunityExtensionView = renderCommunityExtensionView;

    // ----------------------------------------------------
    // 2. LIVE BROWSER & APP PREVIEWER WITH ANNOTATIONS
    // ----------------------------------------------------
    function renderBrowserView(container, existingWebview = null) {
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
          <input type="text" class="swiss-browser-url-input" id="swiss-b-url" value="${currentBrowserUrl}" placeholder="http://localhost:8765" />
          <div class="swiss-browser-port-bar">
            <span class="swiss-port-label">Quick Ports:</span>
            <div class="swiss-port-list" id="swiss-port-list">
              <button class="swiss-port-chip" data-port="8765" title="Antigravity Swiss Knife (Right-click to delete)">:8765</button>
              <button class="swiss-port-chip" data-port="5173" title="Vite Development Server (Right-click to delete)">:5173</button>
              <button class="swiss-port-chip" data-port="3000" title="React / Next.js Server (Right-click to delete)">:3000</button>
              <button class="swiss-port-chip" data-port="8080" title="Standard Web Server (Right-click to delete)">:8080</button>
              <button class="swiss-port-chip" data-port="4173" title="Vite Production Preview (Right-click to delete)">:4173</button>
            </div>
            <div class="swiss-port-dropdown-wrap" id="swiss-port-dropdown-wrap">
              <select class="swiss-port-select" id="swiss-port-select" title="Quick Ports (Right-click to delete)">
                <option value="" disabled selected>Quick Ports</option>
                <option value="8765">:8765 (Swiss Knife)</option>
                <option value="5173">:5173 (Vite)</option>
                <option value="3000">:3000 (React)</option>
                <option value="8080">:8080 (Web)</option>
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
            <select class="swiss-browser-btn" id="swiss-b-orientation" title="Device Orientation" style="outline:none;">
              <option value="portrait">Portrait</option>
              <option value="landscape">Landscape</option>
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
            <button class="swiss-browser-btn" id="swiss-b-clear" title="Clear Annotations"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m7 21-4.3-4.3c-1-1-1-2.5 0-3.4l9.6-9.6c1-1 2.5-1 3.4 0l5.6 5.6c1 1 1 2.5 0 3.4L13 21"/><path d="M22 21H7"/><path d="m5 11 9 9"/></svg><span>Clear</span></button>
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
      let webview = existingWebview;
      if (!webview) {
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
          webview.src = currentBrowserUrl;
        } else {
          webview.setAttribute("sandbox", "allow-scripts allow-same-origin allow-forms allow-popups allow-modals");
          webview.setAttribute("allow", "cross-origin-isolated");
          webview.src = currentBrowserUrl;
        }
      } else {
        webview.id = webview.id || "swiss-browser-element";
        if (typeof webview.tagName === "string" && webview.tagName.toLowerCase() !== "webview") {
          isWebview = false;
        }
        if (webview.src && webview.src !== "about:blank") {
          currentBrowserUrl = webview.src;
        }
      }

      // Transparent Canvas for Annotations
      const canvas = document.createElement("canvas");
      canvas.id = "swiss-browser-canvas";
      canvas.className = "swiss-browser-canvas-overlay";
      canvas.style.pointerEvents = "none";

      // Overlay geometry: the canvas is displayed stretched into the (possibly transform-scaled)
      // device frame, so the backing store tracks the *visual* size at the current DPR. Strokes
      // are drawn in visual CSS pixels, which keeps the pen width identical everywhere on screen.
      // canvasScale* converts guest-page layout pixels to visual (on-screen) pixels.
      let canvasScaleX = 1;
      let canvasScaleY = 1;

      if (existingWebview && existingWebview.parentElement && existingWebview.parentElement !== screen) {
        screen.appendChild(existingWebview);
      } else if (!existingWebview || existingWebview.parentElement !== screen) {
        screen.appendChild(webview);
      }
      screen.appendChild(canvas);
      frameBox.appendChild(notch);
      frameBox.appendChild(screen);
      frameBox.appendChild(homeBar);
      stage.appendChild(frameBox);
      vpWrap.appendChild(stage);

      wrap.appendChild(toolbar);
      wrap.appendChild(vpWrap);
      container.appendChild(wrap);

      // Setup Canvas Resize — keeps backing store = visual CSS box × devicePixelRatio and carries
      // any in-progress annotation strokes over to the new geometry.
      const resizeCanvas = () => {
        const dpr = window.devicePixelRatio || 1;
        const rect = canvas.getBoundingClientRect();
        const visW = Math.max(1, Math.round(rect.width || screen.clientWidth || 400));
        const visH = Math.max(1, Math.round(rect.height || screen.clientHeight || 600));
        const nextW = Math.round(visW * dpr);
        const nextH = Math.round(visH * dpr);
        const layoutW = screen.clientWidth || visW;
        const layoutH = screen.clientHeight || visH;
        canvasScaleX = visW / layoutW;
        canvasScaleY = visH / layoutH;
        if (canvas.width === nextW && canvas.height === nextH) return;
        let snapshot = null;
        if (canvas.width > 0 && canvas.height > 0) {
          try {
            snapshot = document.createElement("canvas");
            snapshot.width = canvas.width;
            snapshot.height = canvas.height;
            snapshot.getContext("2d").drawImage(canvas, 0, 0);
          } catch (_) {
            snapshot = null;
          }
        }
        canvas.width = nextW;
        canvas.height = nextH;
        const rCtx = canvas.getContext("2d");
        rCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
        if (snapshot) {
          try {
            rCtx.drawImage(snapshot, 0, 0, visW, visH);
          } catch (_) {}
        }
      };
      setTimeout(resizeCanvas, 50);

      // Panel (sash) resizes change the overlay's CSS box without firing a device-scale change
      // (responsive frame has no transform). Observe the canvas/screen box and resync the bitmap
      // so annotation coordinates stay aligned with the cursor.
      if (window.__swissBrowserCanvasRO) {
        try { window.__swissBrowserCanvasRO.disconnect(); } catch (_) {}
      }
      if (typeof ResizeObserver !== "undefined") {
        let canvasResizeRaf = 0;
        const canvasRO = new ResizeObserver(() => {
          const rect = canvas.getBoundingClientRect();
          if (!rect.width || !rect.height) return; // hidden view: resync when visible again
          if (canvasResizeRaf) return;
          canvasResizeRaf = requestAnimationFrame(() => {
            canvasResizeRaf = 0;
            resizeCanvas();
          });
        });
        canvasRO.observe(canvas);
        canvasRO.observe(screen);
        window.__swissBrowserCanvasRO = canvasRO;
      }

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

      const updateUrlFromWebview = (u) => {
        if (!u || u === "about:blank") return;
        currentBrowserUrl = u;
        const inp = toolbar.querySelector("#swiss-b-url");
        if (inp && document.activeElement !== inp) inp.value = u;
        updateActivePortChip(u);
      };
      try {
        webview.addEventListener("did-navigate", (e) => updateUrlFromWebview(e.url));
        webview.addEventListener("did-navigate-in-page", (e) => updateUrlFromWebview(e.url));
      } catch (_) {}

      // Quick Ports Management & localStorage persistence
      const QUICK_PORTS_KEY = "antigravity_swiss_quick_ports";
      const DEFAULT_PORTS = ["8765", "5173", "3000", "8080", "4173"];
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
          setTimeout(resizeCanvas, 60);
          return;
        }
        if (scaleMode === "fit") {
          const availW = vpWrap.clientWidth - 48;
          const availH = vpWrap.clientHeight - 48;
          let targetW = 424, targetH = 896;
          if (activeDevice === "iphone-16-pro" || activeDevice === "iphone16") { targetW = 424; targetH = 896; }
          else if (activeDevice === "pixel-9" || activeDevice === "pixel9") { targetW = 432; targetH = 944; }
          else if (activeDevice === "ipad") { targetW = 848; targetH = 1208; }
          if (deviceOrientation === "landscape") {
            const swapDim = targetW;
            targetW = targetH;
            targetH = swapDim;
          }

          const factor = Math.min(1, Math.min(availW / targetW, availH / targetH));
          frameBox.style.transform = ` + "`" + `scale(${Math.max(0.2, factor.toFixed(3))})` + "`" + `;
          frameBox.style.transformOrigin = "top center";
        } else {
          const factor = parseFloat(scaleMode) || 1;
          frameBox.style.transform = ` + "`" + `scale(${factor})` + "`" + `;
          frameBox.style.transformOrigin = "top center";
        }
        // The visual (post-transform) size of the annotation overlay changed — resync its
        // backing store so pen strokes keep a constant on-screen width.
        setTimeout(resizeCanvas, 60);
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
        // Landscape swaps the fixed preset width/height via .swiss-landscape CSS
        // overrides; "responsive" always fills the viewport and ignores it.
        if (deviceOrientation === "landscape" && device !== "responsive") {
          frameBox.classList.add("swiss-landscape");
        }
        if (orientSelect) {
          orientSelect.disabled = device === "responsive";
        }
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

      // Portrait | Landscape toggle for the fixed device presets
      const orientSelect = toolbar.querySelector("#swiss-b-orientation");
      if (orientSelect) {
        orientSelect.value = deviceOrientation;
        orientSelect.disabled = activeDevice === "responsive";
        orientSelect.title = activeDevice === "responsive"
          ? "Orientation applies to fixed device presets only"
          : "Device Orientation";
        orientSelect.onchange = () => {
          deviceOrientation = orientSelect.value === "landscape" ? "landscape" : "portrait";
          try { localStorage.setItem("antigravity_swiss_browser_orientation", deviceOrientation); } catch (_) {}
          applyDeviceFrame(activeDevice);
        };
      }

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
          let touchCursor = document.getElementById('swiss-touch-cursor');
          if (!touchCursor) {
            touchCursor = document.createElement('div');
            touchCursor.id = 'swiss-touch-cursor';
            touchCursor.style.cssText = 'position:fixed;width:24px;height:24px;border-radius:50%;background:rgba(26,115,232,0.25);border:2px solid #1a73e8;pointer-events:none;z-index:2147483647;transform:translate(-50%,-50%);display:none;transition:transform 0.08s, background 0.08s;';
            document.body.appendChild(touchCursor);
          }

          if (window.__swissTouchInstalled) {
            window.__swissTouchEnabled = ${enabled};
            if (touchCursor) touchCursor.style.display = ${enabled} ? 'block' : 'none';
            return;
          }
          window.__swissTouchInstalled = true;
          window.__swissTouchEnabled = ${enabled};
          let isTouching = false;
          let id = 1;
          let lastY = 0;

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
            lastY = e.clientY;
            id++;
            if (touchCursor) {
              touchCursor.style.transform = 'translate(-50%,-50%) scale(0.85)';
              touchCursor.style.background = 'rgba(26,115,232,0.45)';
            }
            emit('touchstart', e);
          }, true);

          document.addEventListener('mousemove', (e) => {
            if (touchCursor) {
              touchCursor.style.display = window.__swissTouchEnabled ? 'block' : 'none';
              touchCursor.style.left = e.clientX + 'px';
              touchCursor.style.top = e.clientY + 'px';
            }
            if (!window.__swissTouchEnabled || !isTouching) return;
            const deltaY = lastY - e.clientY;
            lastY = e.clientY;
            window.scrollBy({ top: deltaY, behavior: 'instant' });
            emit('touchmove', e);
          }, true);

          document.addEventListener('mouseup', (e) => {
            if (touchCursor) {
              touchCursor.style.transform = 'translate(-50%,-50%) scale(1)';
              touchCursor.style.background = 'rgba(26,115,232,0.25)';
            }
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

      // Normalized Coordinates for Canvas — visual CSS pixels relative to the overlay box; the
      // 2D context carries the DPR transform, so a brush size maps to a constant on-screen width.
      function getCanvasCoords(e) {
        const rect = canvas.getBoundingClientRect();
        if (!rect.width || !rect.height) return { x: 0, y: 0 };
        return {
          x: e.clientX - rect.left,
          y: e.clientY - rect.top
        };
      }

      // Clears the full backing store regardless of the active coordinate transform.
      function clearAnnotationCanvas() {
        const clearCtx = canvas.getContext("2d");
        clearCtx.save();
        clearCtx.setTransform(1, 0, 0, 1, 0, 0);
        clearCtx.clearRect(0, 0, canvas.width, canvas.height);
        clearCtx.restore();
      }

      // Drawing Tools Management
      const penBtn = toolbar.querySelector("#swiss-b-pen");
      const rectBtn = toolbar.querySelector("#swiss-b-rect");
      const inspectBtn = toolbar.querySelector("#swiss-b-inspect");
      const clearBtn = toolbar.querySelector("#swiss-b-clear");

      function exitAllTools() {
        if (drawTool === "inspect") {
          toggleDOMInspector(false);
        }
        drawTool = "none";
        isDrawing = false;
        penBtn.classList.remove("active");
        rectBtn.classList.remove("active");
        inspectBtn.classList.remove("active");
        canvas.style.pointerEvents = "none";
        clearElementAnnotation();
      }

      function exitInspectTool() {
        exitAllTools();
      }

      function setDrawTool(tool) {
        const nextTool = (drawTool === tool) ? "none" : tool;
        if (drawTool === "inspect") {
          toggleDOMInspector(false);
        }
        drawTool = nextTool;
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
      function clearElementAnnotation() {
        const existing = screen.querySelector("#swiss-element-annotation-box");
        if (existing) existing.remove();
        lastSelectedElement = null;
        lastAnnotatedRegion = null;
        userComment = "";
        clearAnnotationCanvas();
      }

      clearBtn.onclick = () => {
        clearElementAnnotation();
      };

      if (window.__swissBrowserKeyHandler) {
        document.removeEventListener("keydown", window.__swissBrowserKeyHandler, true);
        window.removeEventListener("keydown", window.__swissBrowserKeyHandler, true);
      }
      window.__swissBrowserKeyHandler = (e) => {
        if (e.key === "Escape" || e.key === "Esc" || e.keyCode === 27 || e.which === 27) {
          const hasAnnotationBox = !!screen.querySelector("#swiss-element-annotation-box");
          if (drawTool !== "none" || hasAnnotationBox) {
            e.preventDefault();
            e.stopPropagation();
            if (typeof e.stopImmediatePropagation === "function") e.stopImmediatePropagation();
            exitAllTools();
          }
        }
      };
      document.addEventListener("keydown", window.__swissBrowserKeyHandler, true);
      window.addEventListener("keydown", window.__swissBrowserKeyHandler, true);

      // Drawing Interactions with Bézier Curve Smoothing & Box Tool
      const ctx = canvas.getContext("2d");
      let lastX = 0, lastY = 0;
      let lastMidX = 0, lastMidY = 0;
      let snapshot = null;

      canvas.onmousedown = (e) => {
        if (drawTool === "none" || drawTool === "inspect") return;
        isDrawing = true;
        userComment = "";
        lastSelectedElement = null;
        lastAnnotatedRegion = null;
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

            // The drawn box is in visual pixels; guest-page consumers (elementFromPoint, element
            // popover, capturePage) expect layout pixels of the emulated page.
            const pageBoxW = boxW / (canvasScaleX || 1);
            const pageBoxH = boxH / (canvasScaleY || 1);
            const pageBoxX = boxX / (canvasScaleX || 1);
            const pageBoxY = boxY / (canvasScaleY || 1);
            lastAnnotatedRegion = { x: pageBoxX, y: pageBoxY, width: pageBoxW, height: pageBoxH };

            if (webview && typeof webview.executeJavaScript === "function") {
              const cx = Math.round(pageBoxX + pageBoxW / 2);
              const cy = Math.round(pageBoxY + pageBoxH / 2);
              const detectScript = ` + "`" + `(() => {
                const el = document.elementFromPoint(${cx}, ${cy});
                if (!el || el === document.body || el === document.documentElement) return null;
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
                const r = el.getBoundingClientRect();
                return {
                  selector: getCssSelector(el),
                  outerHTML: (el.outerHTML || '').slice(0, 1500),
                  rect: { x: r.left, y: r.top, width: r.width, height: r.height }
                };
              })()` + "`" + `;
              webview.executeJavaScript(detectScript).then(res => {
                if (res && res.selector) {
                  lastSelectedElement = res;
                  renderElementAnnotationBox(res);
                } else {
                  renderElementAnnotationBox({
                    selector: "Box Region (" + Math.round(pageBoxW) + "×" + Math.round(pageBoxH) + ")",
                    outerHTML: "",
                    rect: { x: pageBoxX, y: pageBoxY, width: pageBoxW, height: pageBoxH }
                  });
                }
              }).catch(() => {
                renderElementAnnotationBox({
                  selector: "Box Region (" + Math.round(pageBoxW) + "×" + Math.round(pageBoxH) + ")",
                  outerHTML: "",
                  rect: { x: pageBoxX, y: pageBoxY, width: pageBoxW, height: pageBoxH }
                });
              });
            } else {
              renderElementAnnotationBox({
                selector: "Box Region (" + Math.round(pageBoxW) + "×" + Math.round(pageBoxH) + ")",
                outerHTML: "",
                rect: { x: pageBoxX, y: pageBoxY, width: pageBoxW, height: pageBoxH }
              });
            }
          }
        }
      };

      // Interactive DOM Element Inspector
      function toggleDOMInspector(active) {
        if (!active) {
          if (webview && typeof webview.executeJavaScript === "function") {
            webview.executeJavaScript("if (window.__swissCleanupInspector) window.__swissCleanupInspector();");
          } else {
            try {
              if (webview && webview.contentWindow && typeof webview.contentWindow.__swissCleanupInspector === "function") {
                webview.contentWindow.__swissCleanupInspector();
              }
            } catch (_) {}
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
            overlay.style.backgroundColor = 'transparent'; /* Transparent inner field rather than light red rgba(234, 67, 53, 0.15) */
            overlay.style.zIndex = '2147483647';
            overlay.style.boxSizing = 'border-box';
            overlay.style.transition = 'all 0.05s ease-out';
            overlay.style.display = 'none';

            const tagBadge = document.createElement('div');
            tagBadge.id = 'swiss-dom-inspect-badge';
            tagBadge.style.display = 'none';
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
            if (window.parent && window.parent !== window) {
              try { window.parent.postMessage({ type: 'SWISS_INSPECT_RESULT', payload: payload }, '*'); } catch (_) {}
            }
            window.__swissCleanupInspector();
          }

          function handleKeyDown(e) {
            if (e.key === 'Escape' || e.key === 'Esc' || e.keyCode === 27 || e.which === 27) {
              e.preventDefault();
              e.stopPropagation();
              if (typeof e.stopImmediatePropagation === 'function') e.stopImmediatePropagation();
              console.log('[SWISS_INSPECT_CANCEL]');
              if (window.parent && window.parent !== window) {
                try { window.parent.postMessage({ type: 'SWISS_INSPECT_CANCEL' }, '*'); } catch (_) {}
              }
              window.__swissCleanupInspector();
            }
          }

          window.__swissCleanupInspector = () => {
            window.__swissInspectorActive = false;
            document.removeEventListener('mousemove', handleMouseMove, true);
            document.removeEventListener('click', handleClick, true);
            document.removeEventListener('keydown', handleKeyDown, true);
            window.removeEventListener('keydown', handleKeyDown, true);
            if (overlay) overlay.style.display = 'none';
          };

          document.addEventListener('mousemove', handleMouseMove, true);
          document.addEventListener('click', handleClick, true);
          document.addEventListener('keydown', handleKeyDown, true);
          window.addEventListener('keydown', handleKeyDown, true);
        })()` + "`" + `;

        if (webview && typeof webview.executeJavaScript === "function") {
          webview.executeJavaScript(inspectorScript);
        } else {
          try {
            if (webview && webview.contentDocument) {
              const s = webview.contentDocument.createElement('script');
              s.textContent = inspectorScript;
              webview.contentDocument.body.appendChild(s);
              s.remove();
            }
          } catch (_) {}
        }
      }

      function renderElementAnnotationBox(result) {
        const existing = screen.querySelector("#swiss-element-annotation-box");
        if (existing) existing.remove();

        const box = document.createElement("div");
        box.id = "swiss-element-annotation-box";
        box.className = "swiss-element-annotation-box";
        userComment = "";

        const screenW = screen.clientWidth || 400;
        const screenH = screen.clientHeight || 600;
        const boxW = 460;
        const boxH = 180;
        let left = Math.max(8, Math.min(result.rect.x, screenW - boxW - 8));
        let top = result.rect.y + result.rect.height + 8;
        if (top + boxH > screenH) {
          top = Math.max(8, result.rect.y - boxH - 8);
        }
        box.style.left = left + "px";
        const selectorEscaped = escapeHtml(result.selector || "element");

        box.innerHTML = '<div class="swiss-element-annotation-header">' +
          '<span class="swiss-element-badge" title="' + selectorEscaped + '">' +
            '<svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
              '<circle cx="12" cy="12" r="10"/><path d="m22 12-4 0"/><path d="m6 12-4 0"/><path d="m12 6 0-4"/><path d="m12 22 0-4"/>' +
            '</svg>' +
            '<span>' + selectorEscaped + '</span>' +
          '</span>' +
          '<button class="swiss-element-annotation-close" id="swiss-element-annotation-close" title="Clear annotation & unselect element">' +
            '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
              '<path d="m7 21-4.3-4.3c-1-1-1-2.5 0-3.4l9.6-9.6c1-1 2.5-1 3.4 0l5.6 5.6c1 1 1 2.5 0 3.4L13 21"/>' +
              '<path d="M22 21H7"/>' +
              '<path d="m5 11 9 9"/>' +
            '</svg>' +
            '<span>Clear</span>' +
          '</button>' +
        '</div>' +
        '<div class="swiss-element-annotation-body">' +
          '<textarea class="swiss-element-annotation-input" id="swiss-element-annotation-input" rows="5" placeholder="Enter detailed annotation notes or paragraph for chat..."></textarea>' +
          '<div class="swiss-element-annotation-actions">' +
            '<button class="swiss-element-annotation-send" id="swiss-element-annotation-send" title="Send element annotation to chat">' +
              '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
                '<path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/>' +
              '</svg>' +
              '<span>Send to Chat</span>' +
            '</button>' +
          '</div>' +
        '</div>';

        screen.appendChild(box);

        const input = box.querySelector("#swiss-element-annotation-input");
        const closeBtn = box.querySelector("#swiss-element-annotation-close");
        const sendBtn = box.querySelector("#swiss-element-annotation-send");
        input.value = "";

        input.oninput = () => {
          userComment = input.value.trim();
        };

        input.onkeydown = (e) => {
          if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
            e.preventDefault();
            userComment = input.value.trim();
            executeSendToChatWorkflow();
          } else if (e.key === "Escape" || e.key === "Esc") {
            e.preventDefault();
            clearElementAnnotation();
          }
        };

        closeBtn.onclick = () => {
          clearElementAnnotation();
        };

        sendBtn.onclick = () => {
          userComment = input.value.trim();
          executeSendToChatWorkflow();
        };

        setTimeout(() => {
          input.focus();
        }, 50);
      }

      function handleInspectResult(result) {
        if (!result) return;
        lastSelectedElement = result;
        lastAnnotatedRegion = result.rect;
        userComment = "";

        const ctx = canvas.getContext("2d");
        clearAnnotationCanvas();

        ctx.strokeStyle = "#ea4335";
        ctx.lineWidth = 2;
        // result.rect is in guest-page layout pixels; convert to the overlay's visual pixels.
        ctx.strokeRect(
          result.rect.x * canvasScaleX,
          result.rect.y * canvasScaleY,
          result.rect.width * canvasScaleX,
          result.rect.height * canvasScaleY
        );

        exitInspectTool();
        renderElementAnnotationBox(result);
      }

      function attachChatAnnotationChip(selectedEl, comment) {
        const chatContainer = document.querySelector('.chat-input-toolbar')?.parentElement ||
                              document.querySelector('.chat-input-container') ||
                              document.querySelector('[data-testid="chat-input-textarea"]')?.parentElement ||
                              document.querySelector('.lexical-container')?.parentElement ||
                              document.querySelector('textarea[placeholder*="Ask"]')?.parentElement;
        if (!chatContainer) return;

        const oldChip = document.getElementById("swiss-chat-annotation-chip");
        if (oldChip) oldChip.remove();

        const chip = document.createElement("div");
        chip.id = "swiss-chat-annotation-chip";
        chip.className = "swiss-chat-annotation-chip";
        const selText = selectedEl ? (selectedEl.selector || "element") : "Preview Snapshot";
        const commentText = comment ? ("“" + comment + "”") : "";

        chip.innerHTML = '<div class="swiss-chat-chip-content">' +
          '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
            '<circle cx="12" cy="12" r="10"/><path d="m22 12-4 0"/><path d="m6 12-4 0"/><path d="m12 6 0-4"/><path d="m12 22 0-4"/>' +
          '</svg>' +
          '<span class="swiss-chat-chip-selector">' + escapeHtml(selText) + '</span>' +
          (commentText ? '<span class="swiss-chat-chip-comment">' + escapeHtml(commentText) + '</span>' : '') +
        '</div>' +
        '<button type="button" class="swiss-chat-chip-delete" id="swiss-chat-chip-delete" title="Delete element annotation from chat input">' +
          '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' +
            '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>' +
          '</svg>' +
        '</button>';

        const toolbar = chatContainer.querySelector('.chat-input-toolbar');
        if (toolbar) {
          chatContainer.insertBefore(chip, toolbar);
        } else {
          chatContainer.prepend(chip);
        }

        chip.querySelector("#swiss-chat-chip-delete").onclick = (e) => {
          e.stopPropagation();
          e.preventDefault();
          chip.remove();

          const textarea = document.querySelector('textarea[placeholder*="Ask"]') ||
                           document.querySelector('textarea');
          if (textarea && textarea.value) {
            textarea.value = textarea.value.replace(/\[Preview Browser (Element )?Annotation[\s\S]*?\(Visual annotation attached: annotation\.png\)\n?/g, '').trim();
            textarea.dispatchEvent(new Event("input", { bubbles: true }));
          }

          const lexicalElem = document.querySelector('[data-lexical-editor="true"]') ||
                              document.querySelector('.lexical-container [contenteditable="true"]');
          if (lexicalElem) {
            if (lexicalElem.__lexicalEditor) {
              try {
                lexicalElem.__lexicalEditor.update(() => {
                  const currentText = lexicalElem.innerText || "";
                  const cleaned = currentText.replace(/\[Preview Browser (Element )?Annotation[\s\S]*?\(Visual annotation attached: annotation\.png\)\n?/g, '').trim();
                  if (typeof document.execCommand === "function") {
                    lexicalElem.focus();
                    document.execCommand("selectAll", false, null);
                    document.execCommand("insertText", false, cleaned);
                  }
                });
              } catch (_) {}
            }
          }

          const fileInput = document.querySelector('input[type="file"]') ||
                            document.querySelector('[type="file"]');
          if (fileInput) {
            try {
              fileInput.value = "";
              fileInput.dispatchEvent(new Event("change", { bubbles: true }));
            } catch (_) {}
          }

          clearElementAnnotation();
          showToast("Deleted element annotation from chat input");
        };
      }

      if (isWebview) {
        webview.addEventListener('console-message', (e) => {
          if (e.message && e.message.includes('[SWISS_INSPECT_RESULT]')) {
            try {
              const idx = e.message.indexOf('[SWISS_INSPECT_RESULT]');
              const jsonStr = e.message.slice(idx + '[SWISS_INSPECT_RESULT]'.length).trim();
              const result = JSON.parse(jsonStr);
              handleInspectResult(result);
            } catch (_) {}
          } else if (e.message && e.message.includes('[SWISS_INSPECT_CANCEL]')) {
            exitInspectTool();
          }
        });
      }

      if (window.__swissBrowserMessageHandler) {
        window.removeEventListener('message', window.__swissBrowserMessageHandler);
      }
      window.__swissBrowserMessageHandler = (e) => {
        if (!e.data) return;
        if (e.data.type === 'SWISS_INSPECT_RESULT') {
          handleInspectResult(e.data.payload);
        } else if (e.data.type === 'SWISS_INSPECT_CANCEL' || e.data === 'SWISS_INSPECT_CANCEL') {
          exitInspectTool();
        }
      };
      window.addEventListener('message', window.__swissBrowserMessageHandler);

      try {
        if (webview && webview.contentWindow) {
          webview.contentWindow.addEventListener('keydown', (e) => {
            if ((e.key === "Escape" || e.key === "Esc" || e.keyCode === 27 || e.which === 27) && drawTool === "inspect") {
              e.preventDefault();
              e.stopPropagation();
              if (typeof e.stopImmediatePropagation === 'function') e.stopImmediatePropagation();
              exitInspectTool();
            } else if (e.key === "Escape" || e.key === "Esc" || e.keyCode === 27 || e.which === 27) {
              e.preventDefault();
              e.stopPropagation();
              if (typeof e.stopImmediatePropagation === 'function') e.stopImmediatePropagation();
              exitAllTools();
            }
          }, true);
        }
      } catch (_) {}

      // Send to Antigravity Chat button workflow
      async function executeSendToChatWorkflow() {
        let comment = "";
        const annotInput = screen.querySelector("#swiss-element-annotation-input");
        if (annotInput && annotInput.value.trim()) {
          comment = annotInput.value.trim();
          userComment = comment;
        } else if (!lastSelectedElement && (!lastAnnotatedRegion || (lastAnnotatedRegion.width <= 5 && lastAnnotatedRegion.height <= 5))) {
          comment = await showSwissPrompt(
            "Add comment to attach with this preview snapshot to Antigravity chat:",
            "",
            {
              multiline: true,
              rows: 6,
              placeholder: "Write a detailed note or paragraph for chat...",
              confirmText: "Send to Chat"
            }
          );
          if (comment === null) return;
          userComment = comment;
        } else {
          comment = annotInput ? annotInput.value.trim() : "";
          userComment = comment;
        }

        const region = (lastSelectedElement && lastSelectedElement.rect)
          ? lastSelectedElement.rect
          : ((lastAnnotatedRegion && lastAnnotatedRegion.width > 5 && lastAnnotatedRegion.height > 5)
            ? lastAnnotatedRegion
            : null);

        const exportW = region ? Math.max(20, Math.round(region.width)) : canvas.width;
        const exportH = region ? Math.max(20, Math.round(region.height)) : canvas.height;

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
          // region is in guest-page layout pixels; the overlay bitmap may be larger (frame scale ×
          // devicePixelRatio), so map the source rect into bitmap pixels.
          const bmpPerPageX = canvas.width / Math.max(1, screen.clientWidth);
          const bmpPerPageY = canvas.height / Math.max(1, screen.clientHeight);
          expCtx.drawImage(
            canvas,
            region.x * bmpPerPageX,
            region.y * bmpPerPageY,
            region.width * bmpPerPageX,
            region.height * bmpPerPageY,
            0, 0, exportW, exportH
          );
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

          let promptText = "";
          if (lastSelectedElement && (lastSelectedElement.selector || lastSelectedElement.outerHTML)) {
            promptText = "[Preview Browser Element Annotation @ " + currentBrowserUrl + "]\n";
            if (lastSelectedElement.selector) {
              promptText += "Selected Element: " + lastSelectedElement.selector + "\n";
            }
            if (lastSelectedElement.outerHTML) {
              promptText += "` + "\x60\x60\x60" + `html\n" + lastSelectedElement.outerHTML.trim() + "\n` + "\x60\x60\x60" + `\n";
            }
            if (userComment) {
              promptText += "Annotation: " + userComment + "\n";
            }
            promptText += "(Visual annotation attached: annotation.png)";
          } else {
            promptText = "[Preview Browser Annotation @ " + currentBrowserUrl + "]\n";
            if (lastAnnotatedRegion) {
              promptText += "Region: " + Math.round(lastAnnotatedRegion.width) + "×" + Math.round(lastAnnotatedRegion.height) + " at (" + Math.round(lastAnnotatedRegion.x) + ", " + Math.round(lastAnnotatedRegion.y) + ")\n";
            }
            if (userComment) {
              promptText += "Annotation: " + userComment + "\n";
            }
            promptText += "(Visual annotation attached: annotation.png)";
          }

          let injectedLexical = false;
          const lexicalElem = document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
                              document.querySelector('[data-testid="chat-input-textarea"]') ||
                              document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
                              document.querySelector('[data-lexical-editor="true"]') ||
                              document.querySelector('.lexical-container [contenteditable="true"]') ||
                              document.querySelector('.chat-input-container [contenteditable="true"]') ||
                              document.querySelector('[contenteditable="true"]');

          if (lexicalElem && lexicalElem.isContentEditable) {
            try {
              if (typeof lexicalElem.focus === "function") lexicalElem.focus();
              const sel = window.getSelection();
              if (sel) {
                const range = document.createRange();
                range.selectNodeContents(lexicalElem);
                range.collapse(false);
                sel.removeAllRanges();
                sel.addRange(range);
              }
              const hasContent = (lexicalElem.innerText || "").trim().length > 0;
              const textToInsert = (hasContent ? "\n" : "") + promptText;
              document.execCommand("insertText", false, textToInsert);
              const hasInserted = (lexicalElem.innerText || "").includes(promptText);
              if (!hasInserted) {
                try {
                  lexicalElem.dispatchEvent(new InputEvent("beforeinput", {
                    inputType: "insertText",
                    data: textToInsert,
                    bubbles: true,
                    cancelable: true
                  }));
                } catch (_) {}
              }
              lexicalElem.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
              injectedLexical = true;
            } catch (err) {
              console.warn("Direct editor injection failed:", err);
            }
          }

          if (!injectedLexical || (lexicalElem && !lexicalElem.innerText.includes("[Preview Browser"))) {
            insertTextToChatInput(promptText);
          }

          // 2. Attach screenshot file to fileInput after 120ms to allow React/Lexical state to commit text first
          setTimeout(() => {
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
          }, 120);

          // 3. Robust verification and retry: ensure the prompt text is in the chat input
          setTimeout(() => {
            const curInput = document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
                             document.querySelector('[data-testid="chat-input-textarea"]') ||
                             document.querySelector('[data-lexical-editor="true"]') ||
                             document.querySelector('.lexical-container [contenteditable="true"]') ||
                             document.querySelector('textarea[placeholder*="Ask"]') ||
                             document.querySelector('textarea');
            if (curInput) {
              const textInInput = curInput.isContentEditable ? (curInput.innerText || "") : (curInput.value || "");
              if (!textInInput.includes("[Preview Browser")) {
                insertTextToChatInput(promptText);
              }
            }
          }, 250);

          setTimeout(() => {
            const curInput = document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
                             document.querySelector('[data-testid="chat-input-textarea"]') ||
                             document.querySelector('[data-lexical-editor="true"]') ||
                             document.querySelector('.lexical-container [contenteditable="true"]') ||
                             document.querySelector('textarea[placeholder*="Ask"]') ||
                             document.querySelector('textarea');
            if (curInput) {
              const textInInput = curInput.isContentEditable ? (curInput.innerText || "") : (curInput.value || "");
              if (!textInInput.includes("[Preview Browser")) {
                insertTextToChatInput(promptText);
              }
            }
          }, 600);

          attachChatAnnotationChip(lastSelectedElement, userComment);

          // Clear element, region, and comment state so subsequent annotations never repopulate
          clearElementAnnotation();
          userComment = "";
          lastSelectedElement = null;
          lastAnnotatedRegion = null;

          showToast("Attached element annotation to Antigravity chat");
        }, "image/png");
      }

      toolbar.querySelector("#swiss-b-send-chat").onclick = executeSendToChatWorkflow;

      // Container-level teardown (issue P4-a), invoked before this view's DOM is destroyed on a
      // tab switch. Releases the ResizeObservers and the window resize listeners so they never
      // keep observing detached nodes or run against a destroyed toolbar.
      const teardownBrowserView = () => {
        if (window.__swissBrowserCanvasRO) {
          try { window.__swissBrowserCanvasRO.disconnect(); } catch (_) {}
          window.__swissBrowserCanvasRO = null;
        }
        if (window.__swissToolbarRO) {
          try { window.__swissToolbarRO.disconnect(); } catch (_) {}
          window.__swissToolbarRO = null;
        }
        window.removeEventListener("resize", updateToolbarResponsiveness);
        window.removeEventListener("resize", applyDeviceScale);
        window.__swissResizeBound = false;
      };
      if (container) container.__swissTabTeardown = teardownBrowserView;
      wrap.__swissTeardown = teardownBrowserView;
      if (webview) webview.__swissTeardown = teardownBrowserView;
    }

    // ----------------------------------------------------
    // Built-in Browser Enhancements Adapter
    // ----------------------------------------------------
    function isBrowserEnhancementEnabled() {
      try {
        const cfg = window.__SWISS_ENH_CONFIG__;
        if (cfg && cfg.extensions && typeof cfg.extensions.browser?.aux_panel === "boolean") {
          return cfg.extensions.browser.aux_panel;
        }
        const disabledExts = JSON.parse(localStorage.getItem("antigravity_swiss_disabled_extensions") || "[]");
        if (Array.isArray(disabledExts) && disabledExts.includes("browser")) return false;
        if (localStorage.getItem("antigravity_swiss_ext_browser_enabled") === "false") return false;
      } catch (_) {}
      return true;
    }

    function setupBuiltinBrowserEnhancements() {
      if (!isBrowserEnhancementEnabled()) {
        document.querySelectorAll(".swiss-browser-view.swiss-builtin-browser-view").forEach(v => {
          if (typeof v.__swissTeardown === "function") {
            try { v.__swissTeardown(); } catch (_) {}
          }
          v.remove();
        });
        return;
      }

      const webviews = Array.from(document.querySelectorAll("webview"));
      webviews.forEach(wv => {
        if (wv.hasAttribute("data-swiss-jail") ||
            (wv.getAttribute("partition") && wv.getAttribute("partition").startsWith("persist:script-jail")) ||
            wv.closest?.(".script-jail, .auth-modal, .modal-popup")) {
          return;
        }
        if (wv.__swissEnhanced) {
          const wrap = wv.closest(".swiss-browser-view");
          if (wrap && wrap.isConnected) return;
        }
        attachBuiltinBrowserEnhancements(wv);
      });
    }

    function attachBuiltinBrowserEnhancements(webview) {
      if (!webview) return;
      webview.__swissEnhanced = true;
      const parent = webview.parentElement;
      if (!parent) return;

      const wrap = document.createElement("div");
      wrap.className = "swiss-browser-view swiss-builtin-browser-view";
      wrap.style.width = "100%";
      wrap.style.height = "100%";
      wrap.style.display = "flex";
      wrap.style.flexDirection = "column";
      wrap.style.position = "relative";
      wrap.style.overflow = "hidden";

      parent.insertBefore(wrap, webview);
      renderBrowserView(wrap, webview);
    }

    window.setupBuiltinBrowserEnhancements = setupBuiltinBrowserEnhancements;
    window.attachBuiltinBrowserEnhancements = attachBuiltinBrowserEnhancements;

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

    function isMarkdownFile(item) {
      if (!item) return false;
      if (item.type === "markdown") return true;
      const name = (item.name || item.path || "").toLowerCase();
      return name.endsWith(".md") || name.endsWith(".markdown");
    }

    function renderFilesView(container) {
      // Self-clearing: the WYSIWYG / in-place editors hand control back by calling this function
      // directly. Without this, the explorer would be appended *below* the outgoing editor
      // instead of replacing it (issue #117a — back-to-explorer opened a second file list).
      container.innerHTML = "";

      // The folder jump menu is fixed-positioned on document.body, so re-rendering this
      // view must drop any menu bound to the previous DOM instead of orphaning it.
      const staleDirJumpMenu = document.getElementById("swiss-dir-jump-menu");
      if (staleDirJumpMenu) staleDirJumpMenu.remove();

      const wrap = document.createElement("div");
      wrap.className = "swiss-files-view";

      const toolbar = document.createElement("div");
      toolbar.className = "swiss-files-toolbar";
      toolbar.innerHTML = ` + "`" + `
        <div class="swiss-files-address-bar">
          <button class="swiss-browser-btn" id="swiss-f-home" title="Home Folder"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg></button>
          <button class="swiss-browser-btn" id="swiss-f-refresh" title="Refresh"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg></button>
          <span class="swiss-files-path-wrap">
            <input type="text" class="swiss-files-path-input" id="swiss-f-path" value="${currentFilePath}" />
            <button class="swiss-browser-btn swiss-files-path-caret" id="swiss-f-jump" title="Jump to Folder" aria-haspopup="menu" aria-expanded="false"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg></button>
          </span>
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

      const resolveProjectFolder = (raw) => {
        if (!raw || raw === "." || raw === "GLOBAL") return ".";
        const clean = (raw || "").trim();
        if (typeof window.__swissGetProjectFolder === "function") {
          const folder = window.__swissGetProjectFolder(clean);
          if (folder) return folder;
        }
        try {
          const saved = localStorage.getItem("antigravity_swiss_project_folders");
          if (saved) {
            const parsed = JSON.parse(saved);
            if (parsed[clean]) return parsed[clean];
            if (parsed[clean.toLowerCase()]) return parsed[clean.toLowerCase()];
            for (const k in parsed) {
              if (k.toLowerCase() === clean.toLowerCase()) return parsed[k];
            }
            const sepIdx = clean.search(/[/\\]/);
            if (sepIdx > 0) {
              const prefix = clean.substring(0, sepIdx);
              const rest = clean.substring(sepIdx + 1);
              let top = parsed[prefix] || parsed[prefix.toLowerCase()];
              if (!top) {
                for (const k in parsed) {
                  if (k.toLowerCase() === prefix.toLowerCase()) { top = parsed[k]; break; }
                }
              }
              if (top) {
                return top.replace(/[/\\]+$/, "") + "/" + rest.replace(/^[/\\]+/, "");
              }
            }
          }
        } catch (_) {}
        return clean;
      };

      // Load files
      const loadFiles = async (dirPath, pushHistory = true) => {
        let targetPath = resolveProjectFolder((dirPath || "").trim());
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
              <span class="swiss-file-name">${escapeHtml(item.name)}</span>
              <span class="swiss-file-size">${escapeHtml(item.size || "")}</span>
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

            // Double click: open directory, markdown files enter the WYSIWYG editor directly
            // (issue #117c — single unified editing mode), everything else opens in-place.
            row.ondblclick = (e) => {
              e.stopPropagation();
              if (item.isDir) {
                loadFiles(item.path);
              } else if (isMarkdownFile(item)) {
                openWYSIWYGMarkdownEditor(container, item.path, item.name);
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
              showFileContextMenu(e.clientX, e.clientY, item, selectedItems, () => loadFiles(currentFilePath, false), container);
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

      // Issue #158: folder jump dropdown. The caret sits inside the address field and
      // lists the ancestor chain (Parents) plus the directories next to the current one
      // (Siblings). Navigation reuses loadFiles so history, path input and rows stay in sync.
      const normalizeJumpPath = (raw) => {
        let p = (raw || "").trim();
        if (p.startsWith("file://")) p = p.replace(/^file:\/\//, "");
        if (p.length > 1) p = p.replace(/[/\\]+$/, "");
        return p || "/";
      };
      const isDriveRoot = (p) => /^[A-Za-z]:$/.test(p);
      const parentOfDir = (raw) => {
        const p = normalizeJumpPath(raw);
        if (p === "/" || isDriveRoot(p)) return "";
        const idx = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
        if (idx < 0) return "";
        if (idx === 0) return "/";
        const parent = p.substring(0, idx);
        return isDriveRoot(parent) ? parent + "\\" : parent;
      };
      const dirBaseName = (raw) => {
        const p = normalizeJumpPath(raw);
        if (p === "/" || isDriveRoot(p)) return p;
        const idx = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
        return idx >= 0 ? p.substring(idx + 1) : p;
      };
      const ancestorChain = (raw) => {
        const chain = [];
        let cursor = parentOfDir(raw);
        while (cursor) {
          chain.unshift(cursor);
          cursor = parentOfDir(cursor);
        }
        return chain;
      };

      // Pure placement math so the fixed dropdown stays inside the viewport: align its
      // right edge with the caret, clamp horizontally, flip up when the space below is
      // too small, and cap the height to the space actually available on that side.
      let dirJumpCaret = null;

      const clampMenuPosition = (anchor, menuSize, viewport) => {
        const margin = 8;
        const gap = 4;
        const width = Math.min(menuSize.width, Math.max(0, viewport.width - margin * 2));
        let left = anchor.right - width;
        if (left + width > viewport.width - margin) left = viewport.width - width - margin;
        if (left < margin) left = margin;
        const spaceBelow = Math.max(0, viewport.height - anchor.bottom - gap - margin);
        const spaceAbove = Math.max(0, anchor.top - gap - margin);
        const opensUp = menuSize.height > spaceBelow && spaceAbove > spaceBelow;
        const available = opensUp ? spaceAbove : spaceBelow;
        const maxHeight = Math.min(viewport.height * 0.6, available);
        const height = Math.min(menuSize.height, maxHeight);
        const top = opensUp ? Math.max(margin, anchor.top - height - gap) : anchor.bottom + gap;
        return { left: left, top: top, width: width, maxHeight: maxHeight, opensUp: opensUp };
      };

      const positionDirJumpMenu = (menu) => {
        const anchor = dirJumpCaret
          ? dirJumpCaret.getBoundingClientRect()
          : { top: 0, bottom: 0, left: 0, right: 0 };
        menu.style.maxHeight = "none";
        const natural = menu.getBoundingClientRect();
        const viewport = { width: window.innerWidth, height: window.innerHeight };
        const pos = clampMenuPosition(anchor, { width: natural.width, height: natural.height }, viewport);
        menu.style.left = pos.left + "px";
        menu.style.top = pos.top + "px";
        menu.style.width = pos.width + "px";
        menu.style.minWidth = Math.min(280, Math.max(0, viewport.width - 16)) + "px";
        menu.style.maxHeight = pos.maxHeight + "px";
        menu.style.visibility = "visible";
      };

      const closeDirJumpMenu = () => {
        const open = document.getElementById("swiss-dir-jump-menu");
        if (open) open.remove();
        if (dirJumpCaret) dirJumpCaret.setAttribute("aria-expanded", "false");
        document.removeEventListener("mousedown", onDirJumpOutsideClick, true);
        window.removeEventListener("scroll", onDirJumpScroll, true);
        window.removeEventListener("resize", closeDirJumpMenu);
      };

      const navigateFromDirJump = (target) => {
        if (!target) return;
        closeDirJumpMenu();
        loadFiles(target);
        const input = toolbar.querySelector("#swiss-f-path");
        if (input) input.value = target;
      };

      const buildDirJumpItem = (path, label) => {
        const item = document.createElement("div");
        item.className = "swiss-dir-jump-item";
        item.setAttribute("role", "menuitem");
        item.tabIndex = -1;
        item.dataset.path = path;
        item.title = path;
        item.innerHTML = '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg><span class="swiss-dir-jump-name"></span><svg class="swiss-dir-jump-arrow" viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>';
        item.querySelector(".swiss-dir-jump-name").textContent = label || path;
        item.addEventListener("click", (e) => {
          e.preventDefault();
          e.stopPropagation();
          navigateFromDirJump(path);
        });
        return item;
      };

      const buildDirJumpMenu = () => {
        const menu = document.createElement("div");
        menu.className = "swiss-dir-jump-menu";
        menu.id = "swiss-dir-jump-menu";
        menu.setAttribute("role", "menu");
        menu.setAttribute("aria-label", "Jump to folder");
        menu.tabIndex = -1;

        const current = document.createElement("div");
        current.className = "swiss-dir-jump-item swiss-dir-jump-current active";
        current.setAttribute("role", "menuitem");
        current.setAttribute("aria-disabled", "true");
        current.setAttribute("aria-current", "true");
        current.title = "Current folder";
        current.innerHTML = '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg><span class="swiss-dir-jump-name"></span>';
        current.querySelector(".swiss-dir-jump-name").textContent = currentFilePath;
        menu.appendChild(current);

        const parentsGroup = document.createElement("div");
        parentsGroup.className = "swiss-dir-jump-group";
        parentsGroup.dataset.group = "parents";
        parentsGroup.innerHTML = '<div class="swiss-dir-jump-group-label">Parents</div>';
        const parentsList = document.createElement("div");
        parentsList.className = "swiss-dir-jump-list";
        parentsList.setAttribute("role", "group");
        parentsList.setAttribute("aria-label", "Parents");
        const parents = ancestorChain(currentFilePath);
        if (!parents.length) {
          const empty = document.createElement("div");
          empty.className = "swiss-dir-jump-empty";
          empty.textContent = "No parent folder";
          parentsList.appendChild(empty);
        }
        parents.forEach((p) => {
          parentsList.appendChild(buildDirJumpItem(p, dirBaseName(p)));
        });
        parentsGroup.appendChild(parentsList);
        menu.appendChild(parentsGroup);

        const siblingsGroup = document.createElement("div");
        siblingsGroup.className = "swiss-dir-jump-group";
        siblingsGroup.dataset.group = "siblings";
        siblingsGroup.innerHTML = '<div class="swiss-dir-jump-group-label">Siblings</div>';
        const siblingsList = document.createElement("div");
        siblingsList.className = "swiss-dir-jump-list";
        siblingsList.id = "swiss-dir-jump-siblings";
        siblingsList.setAttribute("role", "group");
        siblingsList.setAttribute("aria-label", "Siblings");
        siblingsList.innerHTML = '<div class="swiss-dir-jump-loading">Loading folders...</div>';
        siblingsGroup.appendChild(siblingsList);
        menu.appendChild(siblingsGroup);

        return menu;
      };

      const loadDirJumpSiblings = async (menu) => {
        const list = menu.querySelector("#swiss-dir-jump-siblings");
        if (!list) return;
        const siblingsGroup = list.closest(".swiss-dir-jump-group");
        const parent = parentOfDir(currentFilePath);
        if (!parent) {
          if (siblingsGroup) siblingsGroup.style.display = "none";
          return;
        }
        let data = null;
        try {
          const res = await fetch(
            API_BASE + "/api/files/list?path=" + encodeURIComponent(parent),
            { headers: getSwissAuthHeaders() }
          );
          if (!res.ok) throw new Error("HTTP " + res.status + ": " + res.statusText);
          data = await res.json();
          if (!data || !data.success) throw new Error((data && data.error) || "Unknown error");
        } catch (err) {
          // Siblings are a shortcut only: a failed lookup must never block parent navigation.
          console.warn("[swiss] Unable to load sibling folders:", err);
          if (siblingsGroup) siblingsGroup.style.display = "none";
          return;
        }
        const currentNorm = normalizeJumpPath(currentFilePath).toLowerCase();
        const others = (data.files || [])
          .filter((item) => item && item.isDir)
          .filter((item) => normalizeJumpPath(item.path).toLowerCase() !== currentNorm)
          .sort((a, b) => (a.name || "").localeCompare(b.name || ""));
        if (!others.length) {
          const empty = document.createElement("div");
          empty.className = "swiss-dir-jump-empty";
          empty.textContent = "No other folders here";
          list.innerHTML = "";
          list.appendChild(empty);
          positionDirJumpMenu(menu);
          return;
        }
        list.innerHTML = "";
        others.forEach((item) => {
          list.appendChild(buildDirJumpItem(item.path, item.name));
        });
        // The menu grew: re-run the placement so an upward flip keeps clearing the caret
        // and the height cap follows the space that is actually left.
        positionDirJumpMenu(menu);
      };

      const dirJumpItems = (menu) => Array.from(menu.querySelectorAll(".swiss-dir-jump-item:not(.active)"));

      const onDirJumpMenuKeydown = (e) => {
        const menu = e.currentTarget;
        if (e.key === "Escape") {
          e.preventDefault();
          e.stopPropagation();
          closeDirJumpMenu();
          const input = toolbar.querySelector("#swiss-f-path");
          if (input) input.focus();
          return;
        }
        if (e.key !== "ArrowDown" && e.key !== "ArrowUp" && e.key !== "Enter") return;
        const items = dirJumpItems(menu);
        if (!items.length) return;
        e.preventDefault();
        e.stopPropagation();
        if (e.key === "Enter") {
          const focused = document.activeElement;
          if (focused && focused.classList && focused.classList.contains("swiss-dir-jump-item")) {
            focused.click();
          }
          return;
        }
        const current = items.indexOf(document.activeElement);
        const next = e.key === "ArrowDown"
          ? (current < 0 ? 0 : (current + 1) % items.length)
          : (current <= 0 ? items.length - 1 : current - 1);
        items[next].focus();
      };

      function onDirJumpOutsideClick(e) {
        const menu = document.getElementById("swiss-dir-jump-menu");
        if (!menu) {
          closeDirJumpMenu();
          return;
        }
        if (menu.contains(e.target)) return;
        if (e.target.closest && e.target.closest("#swiss-f-jump")) return;
        closeDirJumpMenu();
      }

      function onDirJumpScroll(e) {
        const menu = document.getElementById("swiss-dir-jump-menu");
        if (!menu) {
          closeDirJumpMenu();
          return;
        }
        if (e.target && menu.contains(e.target)) return;
        closeDirJumpMenu();
      }

      const toggleDirJumpMenu = (e) => {
        e.preventDefault();
        e.stopPropagation();
        const existing = document.getElementById("swiss-dir-jump-menu");
        if (existing) {
          closeDirJumpMenu();
          return;
        }
        const menu = buildDirJumpMenu();
        menu.addEventListener("keydown", onDirJumpMenuKeydown);
        menu.style.visibility = "hidden";
        // document.body keeps the menu out of every ancestor overflow/transform clip.
        document.body.appendChild(menu);
        positionDirJumpMenu(menu);
        if (dirJumpCaret) dirJumpCaret.setAttribute("aria-expanded", "true");
        try { menu.focus({ preventScroll: true }); } catch (_) { menu.focus(); }
        document.addEventListener("mousedown", onDirJumpOutsideClick, true);
        window.addEventListener("scroll", onDirJumpScroll, true);
        window.addEventListener("resize", closeDirJumpMenu);
        loadDirJumpSiblings(menu);
      };

      dirJumpCaret = toolbar.querySelector("#swiss-f-jump");
      if (dirJumpCaret) {
        dirJumpCaret.onclick = toggleDirJumpMenu;
      }

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
        let fallbackTarget = stageFilePath || ((stScope && stScope !== "GLOBAL") ? stScope : ".");
        if (fallbackTarget === "." && typeof window.__swissGetActiveProject === "function") {
          const actProj = window.__swissGetActiveProject();
          if (actProj && actProj !== "." && actProj !== "GLOBAL") {
            fallbackTarget = actProj;
          }
        }
        initialTargetPath = resolveProjectFolder(fallbackTarget);
        currentFilePath = initialTargetPath;
        fileHistory = stageFileHistory;
      } else {
        const activeProj = (typeof window.__swissGetActiveProject === "function" && window.__swissGetActiveProject()) || ".";
        if (lastAuxProject !== activeProj) {
          lastAuxProject = activeProj;
          auxFileHistory = [];
          auxFilePath = (activeProj && activeProj !== ".") ? activeProj : ".";
        }
        initialTargetPath = resolveProjectFolder(auxFilePath || ((activeProj && activeProj !== ".") ? activeProj : "."));
        currentFilePath = initialTargetPath;
        fileHistory = auxFileHistory;
        // Issue #118: stamp which project this listing belongs to, so the project-scope
        // synchronizer below can detect a stale panel after a project switch.
        container.__swissRenderedProject = activeProj;
      }
      loadFiles(initialTargetPath, false);
    }

    // Issue #118 — auxiliary panel is project-scoped: when the active project changes while the
    // file explorer is showing, replace the stale listing with the new project's contents. The
    // explorer stamps the project it rendered with on the container; editors are left untouched
    // so an open file is never torn down mid-edit.
    function syncAuxProjectScope() {
      try {
        const aux = document.getElementById("swiss-aux-container");
        if (!aux || aux.dataset.renderedTab !== "files") return;
        if (!aux.querySelector(".swiss-files-view")) return;
        const proj = (typeof window.__swissGetActiveProject === "function" && window.__swissGetActiveProject()) || ".";
        if (aux.__swissRenderedProject === proj) return;
        renderSwissTabContent(aux, "files");
      } catch (_) {}
    }

    // WYSIWYG Markdown Editor
    function openWYSIWYGMarkdownEditor(container, path, name) {
      container.innerHTML = "<div style='padding:12px; font-size:11px; color:#94a3b8;'>Loading markdown file...</div>";

      fetch(API_BASE + "/api/files/read?path=" + encodeURIComponent(path))
        .then(res => res.json())
        .then(data => {
          if (!data.success) {
            showToast("Error reading markdown file: " + data.error, "error");
            renderFilesView(container);
            return;
          }

          container.innerHTML = "";
          const editor = document.createElement("div");
          editor.className = "swiss-editor-container swiss-wysiwyg-container";

          editor.innerHTML = ` + "`" + `
            <div class="swiss-editor-toolbar" style="display:flex; justify-content:space-between; align-items:center; padding:6px 10px; background:var(--card,#f8fafc); border-bottom:1px solid var(--border,#e2e8f0); gap:8px; flex-wrap:wrap;">
              <div style="display:flex; align-items:center; gap:8px;">
                <button class="swiss-browser-btn" id="swiss-wy-back"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg><span>Files</span></button>
                <span style="font-size:11px; font-weight:600; color:var(--text,#1e293b);">${escapeHtml(name)}</span>
              </div>
              <div style="display:flex; align-items:center; gap:4px; flex-wrap:wrap;">
                <button type="button" class="swiss-browser-btn" id="swiss-wy-bold" title="Bold (Ctrl+B)" style="padding:3px 7px;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 12h9a4 4 0 0 1 0 8H6v-8Z"/><path d="M6 4h8a4 4 0 0 1 0 8H6V4Z"/></svg></button>
                <button type="button" class="swiss-browser-btn" id="swiss-wy-italic" title="Italic (Ctrl+I)" style="padding:3px 7px;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" x2="10" y1="4" y2="4"/><line x1="14" x2="5" y1="20" y2="20"/><line x1="15" x2="9" y1="4" y2="20"/></svg></button>
                <button type="button" class="swiss-browser-btn" id="swiss-wy-list" title="Bullet List (Ctrl+.)" style="padding:3px 7px;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="8" x2="21" y1="6" y2="6"/><line x1="8" x2="21" y1="12" y2="12"/><line x1="8" x2="21" y1="18" y2="18"/><line x1="3" x2="3.01" y1="6" y2="6"/><line x1="3" x2="3.01" y1="12" y2="12"/><line x1="3" x2="3.01" y1="18" y2="18"/></svg></button>
                <button type="button" class="swiss-browser-btn" id="swiss-wy-code" title="Inline Code / Pre" style="padding:3px 7px;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg></button>
                <select id="swiss-wy-heading" title="Header Level (Ctrl+0 to Ctrl+6)" style="font-size:11px; padding:2px 6px; border-radius:4px; border:1px solid var(--border,#cbd5e1); background:var(--canvas,#ffffff); color:var(--text,#1e293b);">
                  <option value="p">Normal text (Ctrl+0)</option>
                  <option value="h1">Heading 1 (Ctrl+1)</option>
                  <option value="h2">Heading 2 (Ctrl+2)</option>
                  <option value="h3">Heading 3 (Ctrl+3)</option>
                  <option value="h4">Heading 4 (Ctrl+4)</option>
                  <option value="h5">Heading 5 (Ctrl+5)</option>
                  <option value="h6">Heading 6 (Ctrl+6)</option>
                </select>
              </div>
              <div style="display:flex; gap:6px;">
                <button class="swiss-browser-btn" id="swiss-wy-annotate" title="Annotate selection to chat"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg><span>Chat</span></button>
                <button class="swiss-browser-btn primary" id="swiss-wy-save" title="Save file (Ctrl+S)"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15.2 3a2 2 0 0 1 1.4.6l3.8 3.8a2 2 0 0 1 .6 1.4V19a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z"/><path d="M17 21v-7a1 1 0 0 0-1-1H8a1 1 0 0 0-1 1v7"/><path d="M7 3v4a1 1 0 0 0 1 1h7"/></svg><span>Save</span></button>
              </div>
            </div>
            <div class="swiss-editor-body" style="flex:1; display:flex; flex-direction:column; overflow:hidden; position:relative;">
              <div class="swiss-wysiwyg-content" contenteditable="true" spellcheck="false"></div>
            </div>
          ` + "`" + `;

          container.appendChild(editor);

          const visualContent = editor.querySelector(".swiss-wysiwyg-content");
          const btnBold = editor.querySelector("#swiss-wy-bold");
          const btnItalic = editor.querySelector("#swiss-wy-italic");
          const btnList = editor.querySelector("#swiss-wy-list");
          const headingSelect = editor.querySelector("#swiss-wy-heading");
          const btnCode = editor.querySelector("#swiss-wy-code");
          const btnBack = editor.querySelector("#swiss-wy-back");
          const btnSave = editor.querySelector("#swiss-wy-save");
          const btnAnnotate = editor.querySelector("#swiss-wy-annotate");

          const escapeText = (t) => String(t || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
          const formatInline = (t) => {
            return escapeText(t)
              .replace(/\*\*(.+?)\*\*/g, "<b>$1</b>")
              .replace(/__(.+?)__/g, "<b>$1</b>")
              .replace(/\*(.+?)\*/g, "<i>$1</i>")
              .replace(/_(.+?)_/g, "<i>$1</i>")
              .replace(new RegExp("\\x60([^\\x60]+)\\x60", "g"), "<code>$1</code>")
              .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank">$1</a>');
          };

          const mdToHtml = (md) => {
            if (!md) return "<p><br></p>";
            let html = "";
            const lines = md.split("\n");
            let inList = false;
            let inPre = false;
            let preContent = "";

            for (let i = 0; i < lines.length; i++) {
              const line = lines[i];

              if (line.trim().startsWith("\x60\x60\x60")) {
                if (inPre) {
                  html += "<pre><code>" + escapeText(preContent) + "</code></pre>";
                  inPre = false;
                  preContent = "";
                } else {
                  if (inList) { html += "</ul>"; inList = false; }
                  inPre = true;
                  preContent = "";
                }
                continue;
              }

              if (inPre) {
                preContent += (preContent ? "\n" : "") + line;
                continue;
              }

              const listMatch = line.match(/^(\s*)[*-]\s+(.*)$/);
              if (listMatch) {
                if (!inList) {
                  html += "<ul>";
                  inList = true;
                }
                html += "<li>" + formatInline(listMatch[2]) + "</li>";
                continue;
              } else if (inList) {
                html += "</ul>";
                inList = false;
              }

              const hMatch = line.match(/^(#{1,6})\s+(.*)$/);
              if (hMatch) {
                const lvl = hMatch[1].length;
                html += "<h" + lvl + ">" + formatInline(hMatch[2]) + "</h" + lvl + ">";
                continue;
              }

              const bqMatch = line.match(/^>\s+(.*)$/);
              if (bqMatch) {
                html += "<blockquote>" + formatInline(bqMatch[1]) + "</blockquote>";
                continue;
              }

              if (!line.trim()) {
                html += "<p><br></p>";
              } else {
                html += "<p>" + formatInline(line) + "</p>";
              }
            }

            if (inList) html += "</ul>";
            if (inPre) html += "<pre><code>" + escapeText(preContent) + "</code></pre>";
            return html || "<p><br></p>";
          };

          const htmlToMd = (node) => {
            if (!node) return "";
            const walk = (n) => {
              if (n.nodeType === Node.TEXT_NODE) return n.nodeValue;
              if (n.nodeType !== Node.ELEMENT_NODE) return "";

              const tag = n.tagName.toLowerCase();
              let childText = "";
              for (let i = 0; i < n.childNodes.length; i++) {
                childText += walk(n.childNodes[i]);
              }

              switch (tag) {
                case "h1": return "# " + childText.trim() + "\n\n";
                case "h2": return "## " + childText.trim() + "\n\n";
                case "h3": return "### " + childText.trim() + "\n\n";
                case "h4": return "#### " + childText.trim() + "\n\n";
                case "h5": return "##### " + childText.trim() + "\n\n";
                case "h6": return "###### " + childText.trim() + "\n\n";
                case "p":
                  if (childText.trim() === "") return "\n";
                  return childText.trim() + "\n\n";
                case "b":
                case "strong": return "**" + childText + "**";
                case "i":
                case "em": return "*" + childText + "*";
                case "u": return childText;
                case "code":
                  if (n.parentNode && n.parentNode.tagName.toLowerCase() === "pre") return childText;
                  return "\x60" + childText + "\x60";
                case "pre": return "\x60\x60\x60\n" + childText.trim() + "\n\x60\x60\x60\n\n";
                case "ul":
                case "ol": return childText + "\n";
                case "li": return "- " + childText.trim() + "\n";
                case "blockquote": return "> " + childText.trim() + "\n\n";
                case "a": return "[" + childText + "](" + (n.getAttribute("href") || "") + ")";
                case "br": return "\n";
                case "div":
                  if (childText.trim() === "") return "\n";
                  return childText + "\n";
                default: return childText;
              }
            };

            let out = "";
            for (let i = 0; i < node.childNodes.length; i++) {
              out += walk(node.childNodes[i]);
            }
            return out.replace(/\n{3,}/g, "\n\n").trim();
          };

          visualContent.innerHTML = mdToHtml(data.content || "");
          setTimeout(() => visualContent.focus(), 40);

          const format = (cmd, val) => {
            visualContent.focus();
            document.execCommand(cmd, false, val || null);
          };

          btnBold.onclick = () => format("bold");
          btnItalic.onclick = () => format("italic");
          btnList.onclick = () => format("insertUnorderedList");
          btnCode.onclick = () => {
            const sel = window.getSelection();
            if (sel && sel.toString()) {
              document.execCommand("insertHTML", false, "<code>" + escapeText(sel.toString()) + "</code>");
            } else {
              format("formatBlock", "<pre>");
            }
          };

          headingSelect.onchange = () => {
            const val = headingSelect.value;
            format("formatBlock", "<" + val + ">");
          };

          const handleKeyShortcuts = (e) => {
            const isCmd = e.ctrlKey || e.metaKey;
            if (!isCmd) return;

            const key = e.key ? e.key.toLowerCase() : "";

            if (key === "b") {
              e.preventDefault();
              format("bold");
              return;
            }

            if (key === "i") {
              e.preventDefault();
              format("italic");
              return;
            }

            if (e.key === ".") {
              e.preventDefault();
              format("insertUnorderedList");
              return;
            }

            if (e.key >= "0" && e.key <= "6") {
              e.preventDefault();
              const level = parseInt(e.key, 10);
              const tag = level === 0 ? "<p>" : "<h" + level + ">";
              format("formatBlock", tag);
              headingSelect.value = level === 0 ? "p" : "h" + level;
              return;
            }

            if (key === "s") {
              e.preventDefault();
              saveDoc();
              return;
            }
          };

          visualContent.addEventListener("keydown", handleKeyShortcuts);

          const saveDoc = async () => {
            const currentMd = htmlToMd(visualContent);
            try {
              const res = await fetch(API_BASE + "/api/files/write", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ path, content: currentMd })
              });
              const rData = await res.json();
              if (rData.success) {
                showToast("Markdown saved successfully!");
              } else {
                showToast("Save failed: " + (rData.error || "Unknown"), "error");
              }
            } catch (err) {
              showToast("Save error: " + err.message, "error");
            }
          };

          btnSave.onclick = saveDoc;
          btnBack.onclick = () => renderFilesView(container);

          btnAnnotate.onclick = async () => {
            const sel = window.getSelection();
            const selectedText = (sel ? sel.toString() : "").trim();
            if (!selectedText) {
              showToast("Select markdown text first, then click Chat!", "warning");
              return;
            }
            const comment = await showSwissPrompt("Enter annotation note for selected snippet:", "Please review this markdown section:");
            if (comment === null) return;
            const snippetMsg = "[Annotated Markdown: " + name + "]\nComment: \"" + comment + "\"\n" + "\x60\x60\x60markdown\n" + selectedText + "\n\x60\x60\x60";
            insertTextToChatInput(snippetMsg);
            showToast("Annotation snippet injected into Antigravity chat input!");
          };
        });
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
    function showFileContextMenu(x, y, item, selectedItems, onRefresh, container) {
      const isSingle = !selectedItems || selectedItems.length <= 1;
      const targets = (selectedItems && selectedItems.length > 0) ? selectedItems : [item];
      const hasClipboard = swissClipboard.items && swissClipboard.items.length > 0;
      const isMarkdown = !item.isDir && isMarkdownFile(item);

      const menu = document.createElement("div");
      menu.className = "swiss-context-menu";

      let itemsHtml = "";
      if (isSingle && isMarkdown) {
        itemsHtml += '<div class="swiss-context-item" id="ctx-file-wysiwyg"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg><span>WYSIWYG Markdown Editor</span></div>' +
          '<div class="swiss-context-divider"></div>';
      }
      itemsHtml += '<div class="swiss-context-item" id="ctx-file-copy"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg><span>Copy</span></div>' +
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

      menu.querySelector("#ctx-file-wysiwyg")?.addEventListener("click", () => {
        removeContextMenu();
        if (container) {
          openWYSIWYGMarkdownEditor(container, item.path, item.name);
        }
      });

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
        <button class="swiss-browser-btn" id="swiss-m-new-text" style="flex: 1 1 0; min-width: 0; justify-content: center; text-align: center; padding: 6px 8px; box-sizing: border-box; white-space: nowrap;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.375 2.625a1 1 0 0 1 3 3l-9.013 9.014a2 2 0 0 1-.853.505l-2.873.84a.5.5 0 0 1-.62-.62l.84-2.873a2 2 0 0 1 .506-.852z"/></svg><span>Text Memo</span></button>
        <button class="swiss-browser-btn" id="swiss-m-record-audio" style="flex: 1 1 0; min-width: 0; justify-content: center; text-align: center; padding: 6px 8px; box-sizing: border-box; white-space: nowrap;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" x2="12" y1="19" y2="22"/></svg><span>Voice Memo</span></button>
        <button class="swiss-browser-btn" id="swiss-m-new-image" style="flex: 1 1 0; min-width: 0; justify-content: center; text-align: center; padding: 6px 8px; box-sizing: border-box; white-space: nowrap;"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg><span>Image Memo</span></button>
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
        <div class="swiss-memo-filter-wrap">
          <button type="button" id="swiss-m-filter-toggle" class="swiss-browser-btn" title="Filter memos by type" style="padding:4px 8px; font-size:10px; white-space:nowrap;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M7 12h10"/><path d="M11 18h2"/></svg><span id="swiss-m-filter-label">Filter: All</span><span id="swiss-m-filter-count" class="swiss-memo-filter-count">0</span></button>
          <div id="swiss-m-filter-menu" class="swiss-memo-filter-menu" style="display:none;">
            <button type="button" class="swiss-memo-filter-item active" data-filter="all">All Memos</button>
            <button type="button" class="swiss-memo-filter-item" data-filter="text">Text</button>
            <button type="button" class="swiss-memo-filter-item" data-filter="voice">Voice</button>
            <button type="button" class="swiss-memo-filter-item" data-filter="image">Images</button>
          </div>
        </div>
      ` + "`" + `;
      wrap.appendChild(searchRow);

      const composer = document.createElement("div");
      composer.className = "swiss-memo-composer";
      composer.id = "swiss-m-composer";
      composer.style.display = "none";
      composer.innerHTML = ` + "`" + `
        <div class="swiss-prompt-title" id="swiss-m-composer-label">New text memo</div>
        <input type="text" id="swiss-m-composer-title" class="swiss-prompt-input" placeholder="Title (optional)..." />
        <textarea id="swiss-m-composer-input" class="swiss-memo-composer-textarea" placeholder="Type quick memo... (Enter or Ctrl+Enter to save, Esc to cancel)" rows="3"></textarea>
        <div class="swiss-memo-composer-actions">
          <button type="button" class="swiss-browser-btn" id="swiss-m-composer-cancel"><span>Cancel</span></button>
          <button type="button" class="swiss-browser-btn primary" id="swiss-m-composer-save"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 13l4 4L19 7"/></svg><span id="swiss-m-composer-save-label">Save Memo</span></button>
        </div>
      ` + "`" + `;
      composer.setAttribute("role", "dialog");
      composer.setAttribute("aria-label", "Text memo");
      wrap.appendChild(composer);

      const imgComposer = document.createElement("div");
      imgComposer.className = "swiss-memo-img-composer";
      imgComposer.id = "swiss-m-img-composer";
      imgComposer.style.display = "none";
      imgComposer.style.flexDirection = "column";
      imgComposer.style.gap = "8px";
      imgComposer.style.border = "1px solid var(--border,#cbd5e1)";
      imgComposer.style.borderRadius = "8px";
      imgComposer.style.padding = "8px";
      imgComposer.style.background = "var(--canvas,#ffffff)";
      imgComposer.style.marginBottom = "8px";
      imgComposer.innerHTML = ` + "`" + `
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <span style="font-size:11px; font-weight:600; color:var(--text,#1e293b); display:flex; align-items:center; gap:6px;">
            <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>
            <span>Whiteboard Sketch &amp; Image Memo</span>
          </span>
          <div style="display:flex; gap:4px;">
            <input type="file" accept="image/*" id="swiss-m-img-file-input" style="display:none;" />
            <button type="button" class="swiss-browser-btn" id="swiss-m-img-upload-btn" title="Upload Image File" style="padding:3px 6px; font-size:10px;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" x2="12" y1="3" y2="15"/></svg><span>Upload</span></button>
            <button type="button" class="swiss-browser-btn" id="swiss-m-img-clear-btn" title="Clear Canvas" style="padding:3px 6px; font-size:10px;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg><span>Clear</span></button>
          </div>
        </div>
        <div style="display:flex; align-items:center; justify-content:space-between; gap:6px; flex-wrap:wrap; padding:4px; background:var(--card,#f8fafc); border-radius:6px; border:1px solid var(--border,#e2e8f0);">
          <div style="display:flex; align-items:center; gap:4px;">
            <button type="button" class="swiss-browser-btn active" id="swiss-m-tool-pen" title="Pen Tool" style="padding:3px 6px; font-size:10px;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/></svg><span>Pen</span></button>
            <button type="button" class="swiss-browser-btn" id="swiss-m-tool-eraser" title="Eraser Tool" style="padding:3px 6px; font-size:10px;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m7 21-4.3-4.3c-1-1-1-2.5 0-3.4l9.6-9.6c1-1 2.5-1 3.4 0l5.6 5.6c1 1 1 2.5 0 3.4L13 21"/><path d="M22 21H7"/><path d="m5 11 9 9"/></svg><span>Eraser</span></button>
            <select id="swiss-m-brush-size" style="font-size:10px; padding:2px 4px; border-radius:4px; border:1px solid var(--border,#cbd5e1); background:var(--canvas,#ffffff); color:var(--text,#1e293b);">
              <option value="2">Thin (2px)</option>
              <option value="4" selected>Normal (4px)</option>
              <option value="8">Thick (8px)</option>
            </select>
          </div>
          <div id="swiss-m-color-palette" style="display:flex; align-items:center; gap:4px;">
            <span class="swiss-color-swatch active" data-color="#0f172a" style="width:14px; height:14px; border-radius:50%; background:#0f172a; cursor:pointer; border:2px solid #3b82f6; box-sizing:border-box;"></span>
            <span class="swiss-color-swatch" data-color="#ef4444" style="width:14px; height:14px; border-radius:50%; background:#ef4444; cursor:pointer; border:1px solid rgba(0,0,0,0.1); box-sizing:border-box;"></span>
            <span class="swiss-color-swatch" data-color="#3b82f6" style="width:14px; height:14px; border-radius:50%; background:#3b82f6; cursor:pointer; border:1px solid rgba(0,0,0,0.1); box-sizing:border-box;"></span>
            <span class="swiss-color-swatch" data-color="#10b981" style="width:14px; height:14px; border-radius:50%; background:#10b981; cursor:pointer; border:1px solid rgba(0,0,0,0.1); box-sizing:border-box;"></span>
            <span class="swiss-color-swatch" data-color="#f59e0b" style="width:14px; height:14px; border-radius:50%; background:#f59e0b; cursor:pointer; border:1px solid rgba(0,0,0,0.1); box-sizing:border-box;"></span>
            <span class="swiss-color-swatch" data-color="#8b5cf6" style="width:14px; height:14px; border-radius:50%; background:#8b5cf6; cursor:pointer; border:1px solid rgba(0,0,0,0.1); box-sizing:border-box;"></span>
          </div>
        </div>
        <div style="position:relative; width:100%; border:1px solid var(--border,#cbd5e1); border-radius:6px; overflow:hidden; background:#ffffff; box-sizing:border-box;">
          <canvas id="swiss-m-whiteboard-canvas" width="480" height="260" style="width:100%; height:180px; display:block; cursor:crosshair; background:#ffffff; touch-action:none;"></canvas>
          <div id="swiss-m-canvas-hint" style="position:absolute; bottom:4px; right:6px; font-size:9px; color:#94a3b8; pointer-events:none; background:rgba(255,255,255,0.85); padding:1px 4px; border-radius:3px;">Paste (Ctrl+V) or draw</div>
        </div>
        <div style="display:flex; flex-direction:column; gap:4px;">
          <input type="text" id="swiss-m-img-title-input" placeholder="Memo Title (optional)..." style="width:100%; padding:4px 8px; font-size:11px; border-radius:4px; border:1px solid var(--border,#cbd5e1); background:var(--canvas,#ffffff); color:var(--text,#1e293b); box-sizing:border-box;" />
          <input type="text" id="swiss-m-img-caption-input" placeholder="Caption / notes (optional)..." style="width:100%; padding:4px 8px; font-size:11px; border-radius:4px; border:1px solid var(--border,#cbd5e1); background:var(--canvas,#ffffff); color:var(--text,#1e293b); box-sizing:border-box;" />
        </div>
        <div style="display:flex; justify-content:flex-end; gap:6px;">
          <button type="button" class="swiss-browser-btn" id="swiss-m-img-cancel"><span>Cancel</span></button>
          <button type="button" class="swiss-browser-btn primary" id="swiss-m-img-save"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 13l4 4L19 7"/></svg><span>Save Image Memo</span></button>
        </div>
      ` + "`" + `;
      wrap.appendChild(imgComposer);

      const memoList = document.createElement("div");
      memoList.className = "swiss-memo-list";
      wrap.appendChild(memoList);
      container.appendChild(wrap);

      let allMemos = [];
      let searchQuery = "";
      let typeFilter = "all";
      // A drag on a card also emits a dblclick on release in some browsers; the
      // flag keeps the double-click editor from opening after a drag (#153).
      let memoDragInProgress = false;
      const FILTER_LABELS = { all: "All", text: "Text", voice: "Voice", image: "Image" };

      let mediaStream = null;
      let mediaRecorder = null;
      let recordedChunks = [];
      let speechRecognition = null;
      let speechTranscript = "";
      let recordingStartTime = 0;
      let recordingAudioCtx = null;
      let recordingAnalyser = null;
      let recordingWaveRaf = 0;
      let recordingTimerInterval = 0;

      // Issue #143: memos whose transcript is being produced by the daemon after
      // a save. Drives the per-card pending line and the bounded list polling.
      const pendingExtractMemoIds = new Set();
      let pendingExtractTimers = [];

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

      const memoKindOf = (memo) => {
        if (!memo) return "text";
        if (memo.type === "image") return "image";
        if (memo.type === "voice" || memo.type === "audio" || (Array.isArray(memo.tags) && memo.tags.includes("voice") && memo.type !== "text")) return "voice";
        return "text";
      };

      // ---- Memo naming (issue #150) ----
      // Older voice memos were stored with a "[Voice] " marker in the title. New
      // memos never get that prefix and the marker is stripped on render, so an
      // existing memos.json still displays cleanly.
      const stripMemoVoicePrefix = (title) => String(title === undefined || title === null ? "" : title).replace(/^\[Voice\]\s*/i, "").trim();
      const memoDisplayTitle = (memo) => stripMemoVoicePrefix(memo && memo.title) || "Memo";
      const MEMO_TITLE_PREFIXES = { voice: "Voice Memo", image: "Sketch" };

      // Lowest free positive integer for titles shaped "<prefix> <n>". Gaps are
      // reused, non-matching titles are ignored and numbering starts at 1.
      const nextMemoNumber = (titles, prefix) => {
        const escaped = String(prefix || "").replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
        const pattern = new RegExp("^" + escaped + " ([0-9]+)$");
        const taken = [];
        (Array.isArray(titles) ? titles : []).forEach((title) => {
          const match = pattern.exec(String(title === undefined || title === null ? "" : title).trim());
          if (match) taken.push(parseInt(match[1], 10));
        });
        let n = 1;
        while (taken.indexOf(n) !== -1) n++;
        return n;
      };

      const nextMemoTitle = (kind) => {
        const prefix = MEMO_TITLE_PREFIXES[kind];
        if (!prefix) return "";
        const titles = (Array.isArray(allMemos) ? allMemos : []).map(memoDisplayTitle);
        return prefix + " " + nextMemoNumber(titles, prefix);
      };

      const autoMemoTitle = (text) => {
        const trimmed = String(text || "").trim();
        if (!trimmed) return "Memo";
        return trimmed.substring(0, 24) + (trimmed.length > 24 ? "..." : "");
      };

      const PLAY_ICON = '<svg viewBox="0 0 24 24" width="11" height="11" fill="currentColor" stroke="none"><path d="M8 5v14l11-7z"/></svg>';
      const PAUSE_ICON = '<svg viewBox="0 0 24 24" width="11" height="11" fill="currentColor" stroke="none"><rect x="7" y="5" width="3" height="14" rx="1"/><rect x="14" y="5" width="3" height="14" rx="1"/></svg>';

      // HTML ids must be unique per document, so every card scopes its ids with
      // the memo id. Only [A-Za-z0-9_-] survives, keeping the suffix safe inside
      // attribute values and (defensive) selector strings.
      const memoCardDomKey = (memoId) => {
        const raw = memoId === undefined || memoId === null ? "" : String(memoId);
        const safe = raw.replace(/[^A-Za-z0-9_-]/g, "_");
        return safe || "anon";
      };

      const formatPlayerTime = (seconds) => {
        const numeric = Number(seconds);
        const total = isFinite(numeric) && numeric > 0 ? Math.floor(numeric) : 0;
        const mins = Math.floor(total / 60);
        const secs = total % 60;
        return mins + ":" + (secs < 10 ? "0" : "") + secs;
      };

      // "m:ss" duration labels recorded when the memo was created are the reliable
      // fallback for WebM blobs whose <audio>.duration is Infinity/NaN (issue #136).
      const parseDurationLabel = (label) => {
        if (!label || typeof label !== "string") return 0;
        let total = 0;
        const parts = label.trim().split(":");
        for (let i = 0; i < parts.length; i++) {
          const n = parseInt(parts[i], 10);
          if (!isFinite(n)) return 0;
          total = total * 60 + n;
        }
        return total;
      };

      // Single active player: starting one memo stops the previous one.
      let activePlayback = null;
      const stopActivePlayback = () => {
        if (!activePlayback) return;
        const prev = activePlayback;
        activePlayback = null;
        prev.cancel();
        try { prev.audio.pause(); } catch (_) {}
        prev.setPlaying(false);
      };

      const renderTranscriptInCard = (memoId, transcript) => {
        if (!transcript) return;
        const cards = memoList.querySelectorAll(".swiss-memo-card");
        let card = null;
        for (let i = 0; i < cards.length; i++) {
          if (cards[i].dataset && cards[i].dataset.memoId === memoId) { card = cards[i]; break; }
        }
        if (!card) return;
        const body = card.querySelector(".swiss-memo-card-body");
        if (!body) return;
        let textEl = body.querySelector("[data-memo-transcript]");
        if (!textEl) {
          textEl = document.createElement("div");
          textEl.className = "swiss-memo-card-text swiss-memo-card-transcript";
          textEl.setAttribute("data-memo-transcript", "1");
          body.appendChild(textEl);
        }
        textEl.textContent = transcript;
      };

      // Issue #137 (front-end half): ask the daemon to transcribe/OCR the memo.
      // The endpoint persists the transcript; never fabricate one locally.
      const runMemoExtract = async (memo, kind, btn) => {
        if (!memo || !memo.id || !btn || btn.disabled) return;
        const idleLabel = kind === "image" ? "OCR" : "Convert to Text";
        const doneLabel = kind === "image" ? "OCR ✓" : "Text ✓";
        btn.disabled = true;
        btn.innerHTML = '<span class="swiss-memo-extract-spinner"></span><span>' + idleLabel + '</span>';
        try {
          // Cross-workspace memos live in another memos.json: pass the memo's own
          // workspace so the daemon can resolve it outside the current scope.
          const memoWorkspace = memo.workspace_path || getMemoQueryInfo().wsPath || "";
          const extractUrl = API_BASE + "/api/memos/extract" + (memoWorkspace ? ("?workspace_path=" + encodeURIComponent(memoWorkspace)) : "");
          const res = await fetch(extractUrl, {
            method: "POST",
            headers: getSwissAuthHeaders(),
            body: JSON.stringify({ memo_id: memo.id })
          });
          let payload = null;
          try { payload = await res.json(); } catch (_) { payload = null; }
          if (!res.ok || !payload || payload.success !== true) {
            const message = payload && payload.error ? payload.error : ("Extraction failed (HTTP " + res.status + ")");
            showToast(message, "error");
            btn.disabled = false;
            btn.textContent = idleLabel;
            return;
          }
          const transcript = typeof payload.transcript === "string" ? payload.transcript : "";
          memo.transcript = transcript;
          saveLocalMemos(allMemos);
          btn.disabled = true;
          btn.textContent = doneLabel;
          renderTranscriptInCard(memo.id, transcript);
        } catch (err) {
          showToast("Extraction failed: " + (err && err.message ? err.message : "network error"), "error");
          btn.disabled = false;
          btn.textContent = idleLabel;
        }
      };

      const filterMemosList = (memos, query, filter) => {
        if (!Array.isArray(memos)) return [];
        const effFilter = (filter === "text" || filter === "voice" || filter === "image") ? filter : "all";
        const trimmed = (query || "").trim();
        const q = trimmed.toLowerCase();
        return memos.filter(m => {
          if (!m) return false;
          const kind = memoKindOf(m);
          const isVoice = kind === "voice";
          const isImage = kind === "image";
          if (effFilter === "text" && (isVoice || isImage)) return false;
          if (effFilter === "voice" && !isVoice) return false;
          if (effFilter === "image" && !isImage) return false;
          if (!q) return true;
          const titleMatch = Boolean(m.title && m.title.toLowerCase().includes(q));
          const contentMatch = Boolean(m.content && m.content.toLowerCase().includes(q));
          const transcriptMatch = Boolean(m.transcript && m.transcript.toLowerCase().includes(q));
          const tagsMatch = Array.isArray(m.tags) && m.tags.some(t => typeof t === "string" && t.toLowerCase().includes(q));
          return titleMatch || contentMatch || transcriptMatch || tagsMatch;
        });
      };

      const renderCards = (memos) => {
        // Cards own the <audio> elements: stop playback before replacing the DOM.
        stopActivePlayback();
        memoList.innerHTML = "";
        const trimmedQ = (searchQuery || "").trim();
        if (!Array.isArray(memos) || memos.length === 0) {
          if (trimmedQ) {
            memoList.innerHTML = '<div class="swiss-memo-empty-search">No memos found matching "' + escapeHtml(trimmedQ) + '"</div>';
          } else if (typeFilter !== "all") {
            memoList.innerHTML = "<div class='swiss-memo-list-status' style='font-size:11px; color:#94a3b8; text-align:center; padding:20px 0;'>No " + escapeHtml(FILTER_LABELS[typeFilter] || "") + " memos yet.</div>";
          } else {
            memoList.innerHTML = "<div class='swiss-memo-list-status' style='font-size:11px; color:#94a3b8; text-align:center; padding:20px 0;'>No memos yet. Click Text Memo, Voice Memo, or Image Memo!</div>";
          }
          updateFilterUi(0);
          return;
        }

        memos.forEach(m => {
          const card = document.createElement("div");
          card.className = "swiss-memo-card";
          card.draggable = true;
          card.dataset.memoId = m.id || "";

          const kind = memoKindOf(m);
          const primaryText = kind === "voice" ? "" : (m.content || "");
          const transcriptText = kind === "text" ? "" : (m.transcript || "");
          const extractKind = kind === "image" ? "image" : (kind === "voice" ? "voice" : "");
          const extractIdleLabel = extractKind === "image" ? "OCR" : "Convert to Text";
          const extractDoneLabel = extractKind === "image" ? "OCR ✓" : "Text ✓";
          const extractDone = Boolean(m.transcript);
          const extractPending = autoExtractEnabled && !transcriptText && pendingExtractMemoIds.has(m.id);
          const cardKey = memoCardDomKey(m.id);
          const playerHtml = m.audio_data ? (
            '<div class="swiss-memo-player">' +
              '<button type="button" class="swiss-memo-play" id="swiss-memo-play-' + cardKey + '" aria-label="Play" title="Play">' + PLAY_ICON + '</button>' +
              '<div class="swiss-memo-progress" id="swiss-memo-progress-' + cardKey + '" role="slider" tabindex="0" aria-label="Playback position" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0"><div class="swiss-memo-progress-fill" id="swiss-memo-progress-fill-' + cardKey + '"></div></div>' +
              '<span class="swiss-memo-time" id="swiss-memo-time-' + cardKey + '">0:00 / ' + escapeHtml(m.duration || "0:00") + '</span>' +
            '</div>' +
            '<audio class="swiss-memo-audio" id="swiss-memo-audio-' + cardKey + '" preload="metadata" src="' + escapeHtml(m.audio_data) + '" style="display:none;"></audio>'
          ) : "";

          card.innerHTML = ` + "`" + `
            <div class="swiss-memo-card-head">
              <div class="swiss-memo-card-title-wrap">
                <span class="swiss-memo-card-title">${escapeHtml(memoDisplayTitle(m))}</span>
                ${m.duration ? ('<span class="swiss-memo-card-duration">(' + escapeHtml(m.duration) + ')</span>') : ""}
              </div>
              <span class="swiss-memo-card-date">${escapeHtml(m.created_at || m.createdAt || "")}</span>
            </div>
            <div class="swiss-memo-card-body">
              ${m.image_data ? ('<div class="swiss-memo-card-img-wrap"><img class="swiss-memo-card-img" src="' + escapeHtml(m.image_data) + '" alt="' + escapeHtml(memoDisplayTitle(m)) + '" /></div>') : ""}
              ${primaryText ? ('<div class="swiss-memo-card-text">' + escapeHtml(primaryText) + '</div>') : ""}
              ${transcriptText ? ('<div class="swiss-memo-card-text swiss-memo-card-transcript" data-memo-transcript="1">' + escapeHtml(transcriptText) + '</div>') : ""}
              ${extractPending ? '<div class="swiss-memo-card-text swiss-memo-card-transcript swiss-memo-card-extract-pending">Extracting text…</div>' : ""}
            </div>
            ${playerHtml}
            <div class="swiss-memo-card-foot">
              ${(extractKind && !autoExtractEnabled) ? ('<button type="button" class="swiss-browser-btn swiss-memo-extract-btn" id="swiss-memo-extract-' + cardKey + '"' + (extractDone ? ' disabled' : '') + '>' + escapeHtml(extractDone ? extractDoneLabel : extractIdleLabel) + '</button>') : ""}
              <div class="swiss-memo-card-actions">
                <button class="swiss-browser-btn swiss-memo-insert-btn" id="swiss-memo-insert-${cardKey}" style="padding:2px 6px; font-size:10px;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg><span>Chat</span></button>
                <button class="swiss-browser-btn swiss-memo-del-btn" id="swiss-memo-del-${cardKey}" title="Delete Memo" style="padding:2px 6px; font-size:10px; color:#ef4444;"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg></button>
              </div>
            </div>
          ` + "`" + `;

          card.ondragstart = (e) => {
            memoDragInProgress = true;
            if (m.image_data) {
              try {
                const file = dataURLtoFile(m.image_data, (m.title ? m.title.replace(/[^a-z0-9]/gi, '_') : 'memo') + ".png");
                if (file) e.dataTransfer.items.add(file);
              } catch (_) {}
              e.dataTransfer.setData("text/plain", m.content ? (m.content + "\n" + memoDisplayTitle(m)) : memoDisplayTitle(m));
              e.dataTransfer.setData("text/uri-list", m.image_data);
            } else {
              e.dataTransfer.setData("text/plain", m.transcript || m.content || "");
            }
          };
          card.ondragend = () => {
            setTimeout(() => { memoDragInProgress = false; }, 0);
          };

          card.querySelector(".swiss-memo-insert-btn").onclick = () => {
            if (m.image_data) {
              insertImageToChatInput(m.image_data, m.content || m.title || "");
            } else {
              insertTextToChatInput(m.transcript || m.content || "");
            }
          };

          card.querySelector(".swiss-memo-del-btn").onclick = async () => {
            await deleteMemoById(m);
          };

          // Issue #152: right-click opens the per-card menu. The handlers below
          // call preventDefault/stopPropagation so the card's click and dblclick
          // handlers never see this event.
          card.oncontextmenu = (e) => {
            e.preventDefault();
            e.stopPropagation();
            const selection = window.getSelection && window.getSelection();
            if (selection && selection.removeAllRanges) selection.removeAllRanges();
            showMemoCardContextMenu(e.clientX, e.clientY, m);
          };

          // Issue #153: double-click edits text and image memos in place. Voice
          // memos have no editor, and a double-click right after a drag must not
          // open one (the browser still emits dblclick for drag-release pairs).
          card.ondblclick = (e) => {
            if (memoDragInProgress) return;
            if (e.target && e.target.closest && e.target.closest("button, a, input, textarea, .swiss-memo-player, .swiss-memo-progress")) return;
            const kind = memoKindOf(m);
            if (kind === "text") {
              e.preventDefault();
              e.stopPropagation();
              openComposer(m);
            } else if (kind === "image") {
              e.preventDefault();
              e.stopPropagation();
              openImageComposer(m);
            }
          };

          const extractBtn = card.querySelector(".swiss-memo-extract-btn");
          if (extractBtn) {
            extractBtn.onclick = () => { runMemoExtract(m, extractKind, extractBtn); };
          }

          if (m.audio_data) {
            const audio = card.querySelector(".swiss-memo-audio");
            const playBtn = card.querySelector(".swiss-memo-play");
            const track = card.querySelector(".swiss-memo-progress");
            const fill = card.querySelector(".swiss-memo-progress-fill");
            const timeEl = card.querySelector(".swiss-memo-time");
            const fallbackDuration = parseDurationLabel(m.duration);
            let rafId = 0;
            let dragging = false;

            const setPlaying = (playing) => {
              if (!playBtn) return;
              playBtn.innerHTML = playing ? PAUSE_ICON : PLAY_ICON;
              playBtn.setAttribute("aria-label", playing ? "Pause" : "Play");
              playBtn.title = playing ? "Pause" : "Play";
            };

            const resolveDuration = () => {
              const d = audio.duration;
              if (isFinite(d) && d > 0) return d;
              return fallbackDuration;
            };

            const renderProgress = () => {
              const duration = resolveDuration();
              const current = isFinite(audio.currentTime) ? audio.currentTime : 0;
              const ratio = duration > 0 ? Math.max(0, Math.min(1, current / duration)) : 0;
              if (fill) fill.style.width = (ratio * 100) + "%";
              if (timeEl) timeEl.textContent = formatPlayerTime(current) + " / " + formatPlayerTime(duration);
              if (track) track.setAttribute("aria-valuenow", String(Math.round(ratio * 100)));
            };

            const cancelRaf = () => {
              if (rafId) { cancelAnimationFrame(rafId); rafId = 0; }
            };

            const tick = () => {
              rafId = 0;
              if (!audio.isConnected) { cancelRaf(); return; }
              renderProgress();
              if (!audio.paused && !audio.ended) rafId = requestAnimationFrame(tick);
            };

            const startRaf = () => {
              cancelRaf();
              rafId = requestAnimationFrame(tick);
            };

            const seekFromClientX = (clientX) => {
              if (!track) return;
              const rect = track.getBoundingClientRect();
              const duration = resolveDuration();
              if (!rect.width || !(duration > 0) || !isFinite(duration)) return;
              const ratio = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width));
              try { audio.currentTime = ratio * duration; } catch (_) {}
              renderProgress();
            };

            const controller = { audio: audio, cancel: cancelRaf, setPlaying: setPlaying };

            playBtn.onclick = () => {
              if (!audio.paused) {
                audio.pause();
                return;
              }
              if (activePlayback && activePlayback.audio !== audio) stopActivePlayback();
              const playPromise = audio.play();
              if (playPromise && playPromise.catch) {
                playPromise.catch((err) => {
                  showToast("Playback error: " + (err && err.message ? err.message : "unknown error"), "error");
                });
              }
            };

            audio.addEventListener("play", () => {
              if (activePlayback && activePlayback.audio !== audio) stopActivePlayback();
              activePlayback = controller;
              setPlaying(true);
              startRaf();
            });
            audio.addEventListener("pause", () => {
              if (activePlayback && activePlayback.audio === audio) activePlayback = null;
              cancelRaf();
              setPlaying(false);
              renderProgress();
            });
            audio.addEventListener("ended", () => {
              if (activePlayback && activePlayback.audio === audio) activePlayback = null;
              cancelRaf();
              setPlaying(false);
              try { audio.currentTime = 0; } catch (_) {}
              renderProgress();
            });
            audio.addEventListener("timeupdate", renderProgress);
            audio.addEventListener("loadedmetadata", renderProgress);
            audio.addEventListener("durationchange", renderProgress);
            audio.addEventListener("seeking", renderProgress);
            audio.addEventListener("seeked", renderProgress);

            track.addEventListener("pointerdown", (e) => {
              dragging = true;
              try { track.setPointerCapture(e.pointerId); } catch (_) {}
              seekFromClientX(e.clientX);
              e.preventDefault();
            });
            track.addEventListener("pointermove", (e) => {
              if (dragging) seekFromClientX(e.clientX);
            });
            const endDrag = (e) => {
              if (!dragging) return;
              dragging = false;
              if (e && e.pointerId) {
                try { track.releasePointerCapture(e.pointerId); } catch (_) {}
              }
            };
            track.addEventListener("pointerup", endDrag);
            track.addEventListener("pointercancel", endDrag);
            track.addEventListener("keydown", (e) => {
              const duration = resolveDuration();
              if (!(duration > 0) || !isFinite(duration)) return;
              let delta = 0;
              if (e.key === "ArrowRight") delta = 5;
              else if (e.key === "ArrowLeft") delta = -5;
              else return;
              e.preventDefault();
              try { audio.currentTime = Math.max(0, Math.min(duration, audio.currentTime + delta)); } catch (_) {}
              renderProgress();
            });

            renderProgress();
          }

          memoList.appendChild(card);
        });
        updateFilterUi(memos.length);
      };

      const applyFilterAndRender = () => {
        const filtered = filterMemosList(allMemos, searchQuery, typeFilter);
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

      // Upsert an existing memo through the same endpoint the create flows use,
      // so a rename or an in-place edit updates instead of duplicating (#151-#153).
      const persistMemo = async (memo) => {
        if (!memo || !memo.id) return false;
        const mq = getMemoQueryInfo();
        try {
          const res = await fetch(API_BASE + "/api/memos/save", {
            method: "POST",
            headers: getSwissAuthHeaders(),
            body: JSON.stringify({
              id: memo.id,
              title: memo.title || "",
              content: memo.content || "",
              type: memo.type || "text",
              tags: Array.isArray(memo.tags) ? memo.tags : [],
              image_data: memo.image_data || undefined,
              audio_data: memo.audio_data || undefined,
              transcript: memo.transcript || undefined,
              duration: memo.duration || undefined,
              workspace_path: memo.workspace_path || mq.wsPath || undefined,
              created_at: memo.created_at || new Date().toLocaleString()
            })
          });
          if (!res.ok) {
            let payload = null;
            try { payload = await res.json(); } catch (_) { payload = null; }
            showToast((payload && payload.error) || ("Save failed (HTTP " + res.status + ")"), "error");
            return false;
          }
          return true;
        } catch (err) {
          showToast("Save failed: " + (err && err.message ? err.message : "network error"), "error");
          return false;
        } finally {
          loadMemos(true);
        }
      };

      // Shared by the card's trash button and the context menu (#152), so both
      // paths delete exactly the same way.
      async function deleteMemoById(memo) {
        if (!memo || !memo.id) return;
        allMemos = allMemos.filter(item => item.id !== memo.id);
        saveLocalMemos(allMemos);
        applyFilterAndRender();
        try {
          await fetch(API_BASE + "/api/memos/delete?id=" + encodeURIComponent(memo.id), {
            method: "POST",
            headers: getSwissAuthHeaders()
          });
        } catch (_) {}
        loadMemos(true);
      }

      // Issue #152: per-card context menu. Reuses the shared .swiss-context-menu
      // styles/positioning (outside click, Escape, resize and scroll dismissal)
      // and adds roving keyboard focus so Enter/Space activate the focused item.
      function showMemoCardContextMenu(x, y, memo) {
        if (!memo || !memo.id) return;
        const menu = document.createElement("div");
        menu.className = "swiss-context-menu";
        menu.setAttribute("role", "menu");
        menu.setAttribute("aria-label", "Memo actions");
        menu.tabIndex = -1;
        menu.innerHTML =
          '<div class="swiss-context-item" id="ctx-memo-rename" role="menuitem" tabindex="0"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/></svg><span>Change title</span></div>' +
          '<div class="swiss-context-item" id="ctx-memo-export" role="menuitem" tabindex="0"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg><span>Save to computer</span></div>' +
          '<div class="swiss-context-divider"></div>' +
          '<div class="swiss-context-item" id="ctx-memo-delete" role="menuitem" tabindex="0" style="color:#ef4444;"><svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg><span>Delete</span></div>';
        positionContextMenu(menu, x, y);

        const menuItems = Array.prototype.slice.call(menu.querySelectorAll(".swiss-context-item"));
        const focusItemAt = (index) => {
          if (!menuItems.length) return;
          menuItems[(index + menuItems.length) % menuItems.length].focus();
        };
        menu.addEventListener("contextmenu", (e) => { e.preventDefault(); });
        menu.addEventListener("keydown", (e) => {
          if (e.key === "Escape") {
            e.preventDefault();
            e.stopPropagation();
            removeContextMenu();
          } else if (e.key === "Enter" || e.key === " " || e.key === "Spacebar") {
            const active = document.activeElement;
            const target = (active && active.classList && active.classList.contains("swiss-context-item")) ? active : menuItems[0];
            if (target) {
              e.preventDefault();
              e.stopPropagation();
              target.click();
            }
          } else if (e.key === "ArrowDown") {
            e.preventDefault();
            focusItemAt(menuItems.indexOf(document.activeElement) + 1);
          } else if (e.key === "ArrowUp") {
            e.preventDefault();
            focusItemAt(menuItems.indexOf(document.activeElement) - 1);
          } else if (e.key === "Home") {
            e.preventDefault();
            focusItemAt(0);
          } else if (e.key === "End") {
            e.preventDefault();
            focusItemAt(menuItems.length - 1);
          }
        });

        const renameItem = menu.querySelector("#ctx-memo-rename");
        if (renameItem) {
          renameItem.addEventListener("click", async (e) => {
            e.stopPropagation();
            removeContextMenu();
            const currentTitle = memoDisplayTitle(memo);
            const nextTitle = await showSwissPrompt("Rename memo:", currentTitle);
            if (nextTitle === null) return;
            const trimmed = String(nextTitle || "").trim();
            if (!trimmed || trimmed === currentTitle) return;
            memo.title = trimmed;
            saveLocalMemos(allMemos);
            applyFilterAndRender();
            await persistMemo(memo);
          });
        }

        const exportItem = menu.querySelector("#ctx-memo-export");
        if (exportItem) {
          exportItem.addEventListener("click", async (e) => {
            e.stopPropagation();
            removeContextMenu();
            try {
              const res = await fetch(API_BASE + "/api/memos/export", {
                method: "POST",
                headers: getSwissAuthHeaders(),
                body: JSON.stringify({ memo_id: memo.id })
              });
              let payload = null;
              try { payload = await res.json(); } catch (_) { payload = null; }
              if (!res.ok || !payload || payload.success !== true) {
                showToast((payload && payload.error) || ("Export failed (HTTP " + res.status + ")"), "error");
                return;
              }
              showToast("Saved to " + payload.path);
            } catch (err) {
              showToast("Export failed: " + (err && err.message ? err.message : "network error"), "error");
            }
          });
        }

        const deleteItem = menu.querySelector("#ctx-memo-delete");
        if (deleteItem) {
          deleteItem.addEventListener("click", (e) => {
            e.stopPropagation();
            removeContextMenu();
            deleteMemoById(memo);
          });
        }

        focusItemAt(0);
        if (!menu.contains(document.activeElement) && menu.focus) menu.focus();
      }

      // Issue #143 (UI half): automatic extraction is a daemon-side setting. When
      // auto_extract is on the backend transcribes voice memos / OCRs images as
      // they are saved, so the per-card manual buttons are redundant and must not
      // be rendered. The config is refreshed on every memos-view render.
      let autoExtractEnabled = false;
      const readAutoExtractFlag = (payload) => {
        if (!payload || typeof payload !== "object") return false;
        const cfg = (payload.config && typeof payload.config === "object") ? payload.config : payload;
        return cfg.auto_extract === true || cfg.autoExtract === true;
      };

      const loadMemoConfig = async () => {
        try {
          const res = await fetch(API_BASE + "/api/memos/config", { headers: getSwissAuthHeaders() });
          if (!res.ok) return;
          const payload = await res.json();
          const next = readAutoExtractFlag(payload);
          if (next === autoExtractEnabled) return;
          autoExtractEnabled = next;
          // Pick the new mode up without leaving the tab.
          applyFilterAndRender();
        } catch (_) {
          // Keep the manual buttons when the config endpoint is unavailable.
        }
      };

      // Auto extraction runs in the daemon after the save response, so the card
      // cannot know the transcript yet. Poll the list on a bounded schedule
      // (attempts at 2s, 5s, 10s and 20s after the save) instead of calling
      // /api/memos/extract again: a second call would pay for a second model
      // extraction. Stop as soon as the transcript lands or the view is gone.
      const AUTO_EXTRACT_POLL_DELAYS = [2000, 3000, 5000, 10000];
      const findMemoById = (memoId) => {
        for (let i = 0; i < allMemos.length; i++) {
          if (allMemos[i] && allMemos[i].id === memoId) return allMemos[i];
        }
        return null;
      };
      const hasTranscript = (memo) => Boolean(memo && typeof memo.transcript === "string" && memo.transcript.trim());
      const isMemoViewAlive = () => {
        if (!container || container.isConnected === false) return false;
        const renderedTab = container.dataset ? container.dataset.renderedTab : "";
        return !renderedTab || renderedTab === "memos";
      };
      const clearPendingExtractPoll = () => {
        pendingExtractTimers.forEach((timer) => clearTimeout(timer));
        pendingExtractTimers = [];
        pendingExtractMemoIds.clear();
      };
      const scheduleAutoExtractPoll = (memoId) => {
        if (!autoExtractEnabled || !memoId) return;
        if (hasTranscript(findMemoById(memoId))) return;
        pendingExtractMemoIds.add(memoId);
        applyFilterAndRender();
        let elapsedMs = 0;
        AUTO_EXTRACT_POLL_DELAYS.forEach((delayMs, attempt) => {
          elapsedMs += delayMs;
          const timer = setTimeout(async () => {
            pendingExtractTimers = pendingExtractTimers.filter((t) => t !== timer);
            if (!pendingExtractMemoIds.has(memoId)) return;
            if (!isMemoViewAlive() || hasTranscript(findMemoById(memoId))) {
              pendingExtractMemoIds.delete(memoId);
              return;
            }
            await loadMemos(true, true);
            if (hasTranscript(findMemoById(memoId))) {
              pendingExtractMemoIds.delete(memoId);
              return;
            }
            if (attempt === AUTO_EXTRACT_POLL_DELAYS.length - 1) {
              // Give up quietly: the manual path is hidden in auto mode, so the
              // card just keeps whatever the daemon stored.
              pendingExtractMemoIds.delete(memoId);
              applyFilterAndRender();
            }
          }, elapsedMs);
          pendingExtractTimers.push(timer);
        });
      };

      const loadMemos = async (silent = false, force = false) => {
        if (!silent && memoList.children.length === 0) {
          memoList.innerHTML = "<div class='swiss-memo-list-status' style='font-size:11px; color:#94a3b8;'>Loading memos...</div>";
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
          // A forced (polling) refresh must not repaint stale local cache.
          if (force) return;
          const fallback = getLocalMemos();
          if (fallback.length > 0) {
            allMemos = fallback;
            applyFilterAndRender();
          } else if (!silent || memoList.children.length === 0) {
            memoList.innerHTML = "<div class='swiss-memo-list-status' style='font-size:11px; color:#94a3b8; text-align:center; padding:20px 0;'>No memos yet. Click Text Memo, Voice Memo, or Image Memo!</div>";
          }
        }
      };

      const composerInput = composer.querySelector("#swiss-m-composer-input");
      const composerCancel = composer.querySelector("#swiss-m-composer-cancel");
      const composerSave = composer.querySelector("#swiss-m-composer-save");
      const composerTitleInput = composer.querySelector("#swiss-m-composer-title");
      const composerLabel = composer.querySelector("#swiss-m-composer-label");
      const composerSaveLabel = composer.querySelector("#swiss-m-composer-save-label");
      // null while creating a new memo; the memo id while editing one (#153).
      let composerEditingId = null;

      const closeComposer = () => {
        composer.style.display = "none";
        composerInput.value = "";
        composerTitleInput.value = "";
        composerEditingId = null;
        if (composerLabel) composerLabel.textContent = "New text memo";
        if (composerSaveLabel) composerSaveLabel.textContent = "Save Memo";
      };

      // Issue #151: the composer asks for an optional Title next to the body.
      // Double-clicking a text card reuses it as an in-place editor (#153).
      const openComposer = (memo) => {
        closeImageComposer();
        composerEditingId = memo && memo.id ? memo.id : null;
        composerInput.value = memo ? (memo.content || "") : "";
        composerTitleInput.value = memo ? stripMemoVoicePrefix(memo.title) : "";
        if (composerLabel) composerLabel.textContent = memo ? "Edit text memo" : "New text memo";
        if (composerSaveLabel) composerSaveLabel.textContent = memo ? "Update Memo" : "Save Memo";
        composer.style.display = "flex";
        try { composer.scrollIntoView({ block: "nearest" }); } catch (_) {}
        // The title stays optional, so the body keeps the caret.
        setTimeout(() => composerInput.focus(), 30);
      };

      const createAndSaveMemo = async (text, memoType, tags, imageData, customTitle) => {
        const trimmed = (text || "").trim();
        const effType = memoType || "text";
        const explicitTitle = (customTitle || "").trim();
        if (!trimmed && !imageData && !explicitTitle) return;

        const titleStr = explicitTitle || (effType === "image" ? nextMemoTitle("image") : autoMemoTitle(trimmed));

        const newMemo = {
          id: "memo-" + Date.now(),
          title: titleStr,
          content: trimmed,
          type: effType,
          tags: tags || ["quick"],
          image_data: imageData || undefined,
          created_at: new Date().toLocaleString()
        };
        allMemos.unshift(newMemo);
        saveLocalMemos(allMemos);
        applyFilterAndRender();

        try {
          const mq = getMemoQueryInfo();
          const saveRes = await fetch(` + "`" + `${API_BASE}/api/memos/save` + "`" + `, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              id: newMemo.id,
              title: titleStr,
              content: trimmed,
              type: effType,
              tags: tags || ["quick"],
              image_data: imageData || undefined,
              created_at: newMemo.created_at,
              workspace_path: mq.wsPath || undefined
            })
          });
          // Issue #143: only voice/image memos get auto-extracted; text memos
          // already carry their content.
          if (saveRes.ok && autoExtractEnabled && effType !== "text") {
            scheduleAutoExtractPoll(newMemo.id);
          }
        } catch (_) {}
        loadMemos(true);
      };

      const saveTextComposer = async () => {
        const text = composerInput.value;
        const title = composerTitleInput.value.trim();
        if (!text.trim() && !title) {
          composerTitleInput.focus();
          return;
        }
        const editingMemo = composerEditingId ? findMemoById(composerEditingId) : null;
        const wasEditing = Boolean(editingMemo);
        closeComposer();
        if (wasEditing) {
          editingMemo.title = title || autoMemoTitle(text);
          editingMemo.content = text.trim();
          saveLocalMemos(allMemos);
          applyFilterAndRender();
          await persistMemo(editingMemo);
          return;
        }
        await createAndSaveMemo(text, "text", ["quick"], undefined, title);
      };

      topBar.querySelector("#swiss-m-new-text").onclick = () => {
        if (composer.style.display === "none") {
          openComposer();
        } else {
          if (!composerInput.value.trim() && !composerTitleInput.value.trim()) {
            closeComposer();
          } else if (!composerTitleInput.value.trim()) {
            composerTitleInput.focus();
          } else {
            composerInput.focus();
          }
        }
      };

      composerCancel.onclick = () => {
        closeComposer();
      };

      composerSave.onclick = async () => {
        await saveTextComposer();
      };

      const composerKeydown = async (e) => {
        if (e.isComposing || e.keyCode === 229) return;
        if (e.key === "Escape") {
          e.preventDefault();
          closeComposer();
        } else if (e.key === "Enter" && (e.ctrlKey || e.metaKey || !e.shiftKey)) {
          e.preventDefault();
          await saveTextComposer();
        }
      };
      composerInput.onkeydown = composerKeydown;
      // Enter in the optional title field moves on to the body instead of
      // submitting a half-typed memo.
      composerTitleInput.onkeydown = (e) => {
        if (e.isComposing || e.keyCode === 229) return;
        if (e.key === "Escape") {
          e.preventDefault();
          closeComposer();
        } else if (e.key === "Enter") {
          e.preventDefault();
          composerInput.focus();
        }
      };

      // Image Composer & Whiteboard Logic
      const imgTitleInput = imgComposer.querySelector("#swiss-m-img-title-input");
      const imgCaptionInput = imgComposer.querySelector("#swiss-m-img-caption-input");
      const imgCancelBtn = imgComposer.querySelector("#swiss-m-img-cancel");
      const imgSaveBtn = imgComposer.querySelector("#swiss-m-img-save");
      const imgFileInput = imgComposer.querySelector("#swiss-m-img-file-input");
      const imgUploadBtn = imgComposer.querySelector("#swiss-m-img-upload-btn");
      const imgClearBtn = imgComposer.querySelector("#swiss-m-img-clear-btn");
      const toolPenBtn = imgComposer.querySelector("#swiss-m-tool-pen");
      const toolEraserBtn = imgComposer.querySelector("#swiss-m-tool-eraser");
      const brushSizeSelect = imgComposer.querySelector("#swiss-m-brush-size");
      const colorPalette = imgComposer.querySelector("#swiss-m-color-palette");
      const whiteboardCanvas = imgComposer.querySelector("#swiss-m-whiteboard-canvas");
      const canvasCtx = whiteboardCanvas.getContext("2d");

      let isDrawing = false;
      let lastX = 0;
      let lastY = 0;
      let activeTool = "pen";
      let activeColor = "#0f172a";
      let activeSize = 4;
      let whiteboardDpr = window.devicePixelRatio || 1;
      let whiteboardCssW = 480;
      let whiteboardCssH = 180;

      // Fill the canvas with its white backdrop. Operates on the raw device-pixel bitmap so the
      // whole backing store is covered regardless of the active DPR transform.
      const initWhiteboard = () => {
        canvasCtx.setTransform(1, 0, 0, 1, 0, 0);
        canvasCtx.fillStyle = "#ffffff";
        canvasCtx.fillRect(0, 0, whiteboardCanvas.width, whiteboardCanvas.height);
        canvasCtx.setTransform(whiteboardDpr, 0, 0, whiteboardDpr, 0, 0);
      };

      // Keep the backing store in sync with the CSS box (devicePixelRatio aware) and draw in CSS
      // pixels, so a given brush size always renders at the same visual width no matter how wide
      // the panel is or how the display scales the window. Existing strokes are carried over.
      const resizeWhiteboardCanvas = () => {
        const rect = whiteboardCanvas.getBoundingClientRect();
        const cssW = Math.max(1, Math.round(rect.width || whiteboardCanvas.clientWidth || 480));
        const cssH = Math.max(1, Math.round(rect.height || whiteboardCanvas.clientHeight || 180));
        const dpr = window.devicePixelRatio || 1;
        const nextW = Math.round(cssW * dpr);
        const nextH = Math.round(cssH * dpr);
        if (whiteboardCanvas.width === nextW && whiteboardCanvas.height === nextH) {
          canvasCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
          return;
        }
        let snapshot = null;
        if (whiteboardCanvas.width > 0 && whiteboardCanvas.height > 0) {
          try {
            snapshot = document.createElement("canvas");
            snapshot.width = whiteboardCanvas.width;
            snapshot.height = whiteboardCanvas.height;
            snapshot.getContext("2d").drawImage(whiteboardCanvas, 0, 0);
          } catch (_) {
            snapshot = null;
          }
        }
        whiteboardDpr = dpr;
        whiteboardCssW = cssW;
        whiteboardCssH = cssH;
        whiteboardCanvas.width = nextW;
        whiteboardCanvas.height = nextH;
        canvasCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
        initWhiteboard();
        if (snapshot) {
          try {
            // Stretch the previous bitmap into the new CSS-sized box (uniform, both are CSS px).
            canvasCtx.drawImage(snapshot, 0, 0, cssW, cssH);
          } catch (_) {}
        }
      };
      resizeWhiteboardCanvas();

      if (window.__swissMemoCanvasResizeHandler) {
        window.removeEventListener("resize", window.__swissMemoCanvasResizeHandler);
      }
      window.__swissMemoCanvasResizeHandler = () => {
        if (imgComposer.style.display !== "none") resizeWhiteboardCanvas();
      };
      window.addEventListener("resize", window.__swissMemoCanvasResizeHandler);

      // Panel (sash) resizes do not emit window resize events; observe the canvas box directly so
      // the bitmap never goes stale relative to the pointer coordinates.
      let memoCanvasRO = null;
      if (typeof ResizeObserver !== "undefined") {
        let memoCanvasRaf = 0;
        memoCanvasRO = new ResizeObserver(() => {
          if (imgComposer.style.display === "none") return; // hidden composer: resync on open
          if (memoCanvasRaf) return;
          memoCanvasRaf = requestAnimationFrame(() => {
            memoCanvasRaf = 0;
            resizeWhiteboardCanvas();
          });
        });
        memoCanvasRO.observe(whiteboardCanvas);
      }

      const getCanvasCoords = (e) => {
        // Pointer coordinates are CSS pixels; the 2D context already applies the DPR transform.
        const rect = whiteboardCanvas.getBoundingClientRect();
        return [
          e.clientX - rect.left,
          e.clientY - rect.top
        ];
      };

      whiteboardCanvas.onpointerdown = (e) => {
        e.preventDefault();
        try { whiteboardCanvas.setPointerCapture(e.pointerId); } catch (_) {}
        isDrawing = true;
        const [x, y] = getCanvasCoords(e);
        lastX = x;
        lastY = y;
        canvasCtx.beginPath();
        canvasCtx.moveTo(x, y);
      };

      whiteboardCanvas.onpointermove = (e) => {
        if (!isDrawing) return;
        e.preventDefault();
        const [x, y] = getCanvasCoords(e);
        canvasCtx.beginPath();
        canvasCtx.moveTo(lastX, lastY);
        canvasCtx.lineTo(x, y);
        canvasCtx.lineCap = "round";
        canvasCtx.lineJoin = "round";
        if (activeTool === "eraser") {
          canvasCtx.strokeStyle = "#ffffff";
          canvasCtx.lineWidth = activeSize * 4;
        } else {
          canvasCtx.strokeStyle = activeColor;
          canvasCtx.lineWidth = activeSize;
        }
        canvasCtx.stroke();
        lastX = x;
        lastY = y;
      };

      const stopDrawing = (e) => {
        if (isDrawing) {
          isDrawing = false;
          if (e && e.pointerId) {
            try { whiteboardCanvas.releasePointerCapture(e.pointerId); } catch (_) {}
          }
        }
      };
      whiteboardCanvas.onpointerup = stopDrawing;
      whiteboardCanvas.onpointercancel = stopDrawing;

      toolPenBtn.onclick = () => {
        activeTool = "pen";
        toolPenBtn.classList.add("active");
        toolEraserBtn.classList.remove("active");
      };

      toolEraserBtn.onclick = () => {
        activeTool = "eraser";
        toolEraserBtn.classList.add("active");
        toolPenBtn.classList.remove("active");
      };

      brushSizeSelect.onchange = () => {
        activeSize = parseInt(brushSizeSelect.value, 10) || 4;
      };

      colorPalette.querySelectorAll(".swiss-color-swatch").forEach(swatch => {
        swatch.onclick = () => {
          activeColor = swatch.dataset.color || "#0f172a";
          activeTool = "pen";
          toolPenBtn.classList.add("active");
          toolEraserBtn.classList.remove("active");
          colorPalette.querySelectorAll(".swiss-color-swatch").forEach(s => {
            s.style.border = s === swatch ? "2px solid #3b82f6" : "1px solid rgba(0,0,0,0.1)";
            s.classList.toggle("active", s === swatch);
          });
        };
      });

      imgClearBtn.onclick = () => {
        initWhiteboard();
      };

      const drawImageToCanvas = (img) => {
        const cw = whiteboardCssW;
        const ch = whiteboardCssH;
        const ratio = Math.min(cw / img.width, ch / img.height, 1);
        const w = img.width * ratio;
        const h = img.height * ratio;
        const x = (cw - w) / 2;
        const y = (ch - h) / 2;
        canvasCtx.fillStyle = "#ffffff";
        canvasCtx.fillRect(0, 0, cw, ch);
        canvasCtx.drawImage(img, x, y, w, h);
      };

      imgUploadBtn.onclick = () => {
        imgFileInput.click();
      };

      imgFileInput.onchange = (e) => {
        const file = e.target.files && e.target.files[0];
        if (!file) return;
        const reader = new FileReader();
        reader.onload = (re) => {
          const img = new Image();
          img.onload = () => drawImageToCanvas(img);
          img.src = re.target.result;
        };
        reader.readAsDataURL(file);
      };

      imgComposer.addEventListener("paste", (e) => {
        if (!e.clipboardData || !e.clipboardData.items) return;
        for (let i = 0; i < e.clipboardData.items.length; i++) {
          const item = e.clipboardData.items[i];
          if (item.type.indexOf("image") !== -1) {
            e.preventDefault();
            const blob = item.getAsFile();
            const reader = new FileReader();
            reader.onload = (re) => {
              const img = new Image();
              img.onload = () => drawImageToCanvas(img);
              img.src = re.target.result;
            };
            reader.readAsDataURL(blob);
            break;
          }
        }
      });

      // null while drawing a new sketch; the memo id while editing one (#153).
      let imgEditingId = null;

      const closeImageComposer = () => {
        imgComposer.style.display = "none";
        imgTitleInput.value = "";
        imgCaptionInput.value = "";
        imgEditingId = null;
        imgTitleInput.placeholder = "Memo Title (optional)...";
      };

      // Issue #150: new sketches default to "Sketch <n>"; #153: passing a memo
      // preloads its image and saves back into the same memo instead of a new one.
      const openImageComposer = (memo) => {
        closeComposer();
        imgEditingId = memo && memo.id ? memo.id : null;
        imgComposer.style.display = "flex";
        imgTitleInput.value = memo ? stripMemoVoicePrefix(memo.title) : "";
        imgCaptionInput.value = memo ? (memo.content || "") : "";
        if (!memo) imgTitleInput.placeholder = nextMemoTitle("image") + " (optional)...";
        // The canvas has no layout size while the composer is hidden; sync its backing store now
        // that it is visible so drawing works at the correct DPR-aware resolution.
        setTimeout(() => {
          resizeWhiteboardCanvas();
          initWhiteboard();
          if (memo && memo.image_data) {
            const img = new Image();
            img.onload = () => drawImageToCanvas(img);
            img.src = memo.image_data;
          }
          imgTitleInput.focus();
        }, 30);
      };

      imgCancelBtn.onclick = () => {
        closeImageComposer();
      };

      imgSaveBtn.onclick = async () => {
        const dataUrl = whiteboardCanvas.toDataURL("image/png");
        const titleValue = imgTitleInput.value.trim();
        const caption = imgCaptionInput.value.trim();
        const editingMemo = imgEditingId ? findMemoById(imgEditingId) : null;
        if (editingMemo) {
          closeImageComposer();
          editingMemo.title = titleValue || editingMemo.title || nextMemoTitle("image");
          editingMemo.content = caption;
          editingMemo.image_data = dataUrl;
          saveLocalMemos(allMemos);
          applyFilterAndRender();
          await persistMemo(editingMemo);
          return;
        }
        const customTitle = titleValue || nextMemoTitle("image");
        closeImageComposer();
        await createAndSaveMemo(caption, "image", ["image", "sketch"], dataUrl, customTitle);
      };

      topBar.querySelector("#swiss-m-new-image").onclick = () => {
        if (imgComposer.style.display === "none") {
          openImageComposer();
        } else {
          closeImageComposer();
        }
      };

      const recordBtn = topBar.querySelector("#swiss-m-record-audio");
      recordBtn.setAttribute("aria-label", "Voice Memo");
      recordBtn.title = "Voice Memo";

      // ---- Voice memo recording indicator: elapsed time + live loudness wave ----
      const REC_IDLE_LABEL = '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" x2="12" y1="19" y2="22"/></svg><span>Voice Memo</span>';

      const formatRecordElapsed = () => {
        const elapsedSec = Math.max(0, Math.floor((Date.now() - recordingStartTime) / 1000));
        const mins = Math.floor(elapsedSec / 60);
        const secs = elapsedSec % 60;
        return mins + ":" + (secs < 10 ? "0" : "") + secs;
      };

      const stopRecordingIndicator = () => {
        if (recordingTimerInterval) {
          clearInterval(recordingTimerInterval);
          recordingTimerInterval = 0;
        }
        if (recordingWaveRaf) {
          cancelAnimationFrame(recordingWaveRaf);
          recordingWaveRaf = 0;
        }
        if (recordingAnalyser) {
          try { recordingAnalyser.disconnect(); } catch (_) {}
          recordingAnalyser = null;
        }
        if (recordingAudioCtx) {
          const ctxToClose = recordingAudioCtx;
          recordingAudioCtx = null;
          try {
            const closePromise = ctxToClose.close();
            if (closePromise && closePromise.catch) closePromise.catch(() => {});
          } catch (_) {}
        }
      };

      const startRecordingIndicator = () => {
        stopRecordingIndicator();
        recordBtn.classList.add("active");
        // Issue #145: the visible "Stop Recording" text is gone from this narrow
        // button; the accessible name keeps the action discoverable.
        recordBtn.setAttribute("aria-label", "Stop Recording");
        recordBtn.title = "Stop Recording";
        recordBtn.innerHTML =
          '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="12" height="12" x="6" y="6" rx="2"/></svg>' +
          '<span id="swiss-m-record-timer" style="font-variant-numeric:tabular-nums; font-weight:600;">0:00</span>' +
          '<canvas id="swiss-m-record-wave" class="swiss-memo-record-wave" width="48" height="24" style="width:24px; height:12px; display:block;"></canvas>';

        const timerEl = recordBtn.querySelector("#swiss-m-record-timer");
        const waveCanvas = recordBtn.querySelector("#swiss-m-record-wave");
        const waveCtx = waveCanvas ? waveCanvas.getContext("2d") : null;

        recordingTimerInterval = setInterval(() => {
          if (!timerEl || !timerEl.isConnected) {
            // View was destroyed without an explicit teardown: stop the recording, mic included.
            teardownMemosView();
            return;
          }
          timerEl.textContent = formatRecordElapsed();
        }, 1000);

        // Live loudness indicator driven by an AnalyserNode fed by the microphone stream.
        try {
          const AudioCtxCtor = window.AudioContext || window.webkitAudioContext;
          if (AudioCtxCtor && mediaStream) {
            recordingAudioCtx = new AudioCtxCtor();
            if (recordingAudioCtx.state === "suspended" && recordingAudioCtx.resume) {
              const resumePromise = recordingAudioCtx.resume();
              if (resumePromise && resumePromise.catch) resumePromise.catch(() => {});
            }
            const source = recordingAudioCtx.createMediaStreamSource(mediaStream);
            recordingAnalyser = recordingAudioCtx.createAnalyser();
            recordingAnalyser.fftSize = 64;
            recordingAnalyser.smoothingTimeConstant = 0.6;
            source.connect(recordingAnalyser);
          }
        } catch (waveErr) {
          console.warn("Audio analyser init failed:", waveErr);
          recordingAnalyser = null;
        }

        if (recordingAnalyser && waveCtx && waveCanvas) {
          const levels = new Uint8Array(recordingAnalyser.frequencyBinCount);
          const bars = 5;
          const drawWave = () => {
            recordingWaveRaf = 0;
            if (!recordingAnalyser || !waveCtx) return;
            if (!waveCanvas.isConnected) {
              // View destroyed without an explicit teardown: stop everything, mic included.
              teardownMemosView();
              return;
            }
            recordingAnalyser.getByteFrequencyData(levels);
            const w = waveCanvas.width;
            const h = waveCanvas.height;
            waveCtx.clearRect(0, 0, w, h);
            waveCtx.fillStyle = "#ffffff";
            // Canvas backing store is 2x its CSS box, so a 4px bar renders 2px wide
            // (issue #133) while the 5-bar count and level animation stay unchanged.
            const barW = 4;
            const barGap = Math.max(1, Math.floor((w - barW * bars) / (bars - 1)));
            const usableBins = Math.max(1, Math.floor(levels.length * 0.6));
            const binsPerBar = Math.max(1, Math.floor(usableBins / bars));
            for (let i = 0; i < bars; i++) {
              let sum = 0;
              for (let j = 0; j < binsPerBar; j++) sum += levels[i * binsPerBar + j] || 0;
              const level = Math.min(1, (sum / binsPerBar) / 160);
              const barH = Math.max(2, Math.round(level * h));
              const x = i * (barW + barGap);
              waveCtx.fillRect(x, Math.round((h - barH) / 2), barW, barH);
            }
            recordingWaveRaf = requestAnimationFrame(drawWave);
          };
          drawWave();
        }
      };

      const restoreRecordButton = () => {
        stopRecordingIndicator();
        recordBtn.classList.remove("active");
        recordBtn.setAttribute("aria-label", "Voice Memo");
        recordBtn.title = "Voice Memo";
        recordBtn.innerHTML = REC_IDLE_LABEL;
      };

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
              // Idempotent: also recovers the button if the stream ends without a click.
              restoreRecordButton();
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
              // Issue #150: no "[Voice] " marker any more; untitled recordings
              // get the next free "Voice Memo <n>".
              const defaultVoiceTitle = nextMemoTitle("voice");
              let noteText = null;
              if (finalTranscript) {
                noteText = await showSwissPrompt("Voice recorded & transcribed! Edit title:", finalTranscript);
              } else {
                noteText = await showSwissPrompt("Voice recorded! Enter a transcript / note title:", defaultVoiceTitle);
              }

              if (noteText !== null) {
                const userTitle = String(noteText || "").trim() || (finalTranscript ? finalTranscript : defaultVoiceTitle);
                const titleStr = stripMemoVoicePrefix(userTitle) || userTitle;
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
                  const saveRes = await fetch(` + "`" + `${API_BASE}/api/memos/save` + "`" + `, {
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
                  // Issue #143: the daemon transcribes after responding; poll for
                  // it when this recording has no transcript of its own yet.
                  if (saveRes.ok && autoExtractEnabled) {
                    scheduleAutoExtractPoll(newMemo.id);
                  }
                } catch (_) {}
                loadMemos(true);
              }
            };
            mediaRecorder.start(250);
            startRecordingIndicator();
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
          restoreRecordButton();
        }
      };

      // Container-level teardown, invoked before this view's DOM is destroyed (tab switch or
      // script re-injection) so an in-flight recording never keeps the microphone, timers or
      // AudioContext alive. Registered on the host container; the caller invokes it pre-clear.
      const teardownMemosView = () => {
        removeContextMenu();
        memoDragInProgress = false;
        stopActivePlayback();
        clearPendingExtractPoll();
        const wasRecording = Boolean(recordingTimerInterval) ||
          Boolean(recordingWaveRaf) ||
          Boolean(mediaRecorder && mediaRecorder.state !== "inactive");
        if (mediaRecorder) {
          try {
            mediaRecorder.onstop = null;
            mediaRecorder.ondataavailable = null;
          } catch (_) {}
          if (mediaRecorder.state !== "inactive") {
            try { mediaRecorder.stop(); } catch (_) {}
          }
          mediaRecorder = null;
        }
        if (mediaStream) {
          try { mediaStream.getTracks().forEach(t => t.stop()); } catch (_) {}
          mediaStream = null;
        }
        if (speechRecognition) {
          try { speechRecognition.stop(); } catch (_) {}
          speechRecognition = null;
        }
        stopRecordingIndicator();
        if (wasRecording) {
          showToast("Voice recording stopped (memos view closed)", "warning");
        }
        if (window.__swissMemoCanvasResizeHandler) {
          window.removeEventListener("resize", window.__swissMemoCanvasResizeHandler);
          delete window.__swissMemoCanvasResizeHandler;
        }
        if (memoCanvasRO) {
          try { memoCanvasRO.disconnect(); } catch (_) {}
          memoCanvasRO = null;
        }
        if (window.__swissMemoFilterOutsideHandler) {
          document.removeEventListener("click", window.__swissMemoFilterOutsideHandler);
          delete window.__swissMemoFilterOutsideHandler;
        }
      };
      container.__swissTabTeardown = teardownMemosView;

      const searchInput = searchRow.querySelector("#swiss-m-search-input");
      const searchClear = searchRow.querySelector("#swiss-m-search-clear");
      const filterToggle = searchRow.querySelector("#swiss-m-filter-toggle");
      const filterLabel = searchRow.querySelector("#swiss-m-filter-label");
      const filterCount = searchRow.querySelector("#swiss-m-filter-count");
      const filterMenu = searchRow.querySelector("#swiss-m-filter-menu");

      const updateFilterUi = (visibleCount) => {
        if (filterLabel) filterLabel.textContent = "Filter: " + (FILTER_LABELS[typeFilter] || "All");
        if (filterCount) filterCount.textContent = String(visibleCount || 0);
        if (filterMenu) {
          filterMenu.querySelectorAll(".swiss-memo-filter-item").forEach(item => {
            item.classList.toggle("active", (item.dataset.filter || "all") === typeFilter);
          });
        }
      };

      const closeFilterMenu = () => {
        if (filterMenu) filterMenu.style.display = "none";
      };

      filterToggle.onclick = (e) => {
        e.stopPropagation();
        filterMenu.style.display = filterMenu.style.display === "none" ? "flex" : "none";
      };

      filterMenu.querySelectorAll(".swiss-memo-filter-item").forEach(item => {
        item.onclick = (e) => {
          e.stopPropagation();
          typeFilter = item.dataset.filter || "all";
          closeFilterMenu();
          applyFilterAndRender();
        };
      });

      if (window.__swissMemoFilterOutsideHandler) {
        document.removeEventListener("click", window.__swissMemoFilterOutsideHandler);
      }
      window.__swissMemoFilterOutsideHandler = (e) => {
        if (!filterMenu || filterMenu.style.display === "none") return;
        if (filterMenu.contains(e.target) || filterToggle.contains(e.target)) return;
        closeFilterMenu();
      };
      document.addEventListener("click", window.__swissMemoFilterOutsideHandler);

      searchInput.addEventListener("input", () => {
        searchQuery = searchInput.value;
        searchClear.style.display = searchQuery ? "block" : "none";
        applyFilterAndRender();
      });

      searchInput.addEventListener("keydown", (e) => {
        if (e.key === "Escape") closeFilterMenu();
      });

      searchClear.onclick = () => {
        searchInput.value = "";
        searchQuery = "";
        searchClear.style.display = "none";
        searchInput.focus();
        applyFilterAndRender();
      };

      updateFilterUi(allMemos.length);

      const initialMemos = getLocalMemos();
      if (initialMemos.length > 0) {
        allMemos = initialMemos;
        applyFilterAndRender();
      }
      loadMemoConfig().then(() => loadMemos(initialMemos.length > 0));
    }

    // ----------------------------------------------------
    // 5. IN-CHAT TOKEN & TPS TELEMETRY METRICS
    // ----------------------------------------------------
    let chatTelemetryCachedData = null;
    let chatTelemetryLastFetch = 0;

    async function setupInChatTelemetry() {
      // 0. Daemon offline guard: suppress and purge all telemetry when daemon is not alive
      if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline) {
        document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
        chatTelemetryCachedData = null;
        return;
      }

      // 1. Active modal cleanup: purge stray metrics inside dialogs, modals, settings tab content, code blocks, or auxiliary panels
      document.querySelectorAll('[role="dialog"] .swiss-inchat-metrics, [aria-modal="true"] .swiss-inchat-metrics, .settings-tab-content .swiss-inchat-metrics, [data-testid*="settings"] .swiss-inchat-metrics, [data-testid="models-usage-container"] .swiss-inchat-metrics, #swiss-aux-container .swiss-inchat-metrics, [id^="swiss-aux"] .swiss-inchat-metrics, pre .swiss-inchat-metrics, code .swiss-inchat-metrics, .code-block .swiss-inchat-metrics, [data-testid="agent-input-box"] .swiss-inchat-metrics').forEach(el => el.remove());

      // 2. Strict Container Scoping: Require that conversation view exists and is visible
      const convoView = document.querySelector('[data-testid="conversation-view"]');
      if (!convoView || convoView.offsetParent === null || convoView.style.display === "none") {
        document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
        return;
      }

      // 3. Resolve active conversation ID without allowing stale localStorage fallback outside active chats
      let convId = "";
      const convMatch = window.location.pathname.match(/\/c\/([a-zA-Z0-9_-]+)/);
      if (convMatch) {
        convId = convMatch[1];
      }
      if (!convId && window.location.hash) {
        const hashMatch = window.location.hash.match(/\/c\/([a-zA-Z0-9_-]+)/);
        if (hashMatch) convId = hashMatch[1];
      }
      if (!convId) {
        const el = convoView.querySelector('[data-testid="agent-input-box"]') || convoView;
        if (el) {
          try {
            const fiberKey = Object.keys(el).find(k => k.startsWith("__reactFiber"));
            if (fiberKey) {
              let curr = el[fiberKey];
              while (curr) {
                if (curr.memoizedProps) {
                  const id = curr.memoizedProps.conversationId || curr.memoizedProps.cascadeId;
                  if (typeof id === "string" && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)) {
                    convId = id;
                    break;
                  }
                }
                curr = curr.return;
              }
            }
          } catch (_) {}
        }
      }
      if (!convId) {
        const hasActiveChat = Boolean(convoView.querySelector('[data-testid="agent-input-box"], textarea, button[aria-label="Good response"], button[aria-label="Bad response"]'));
        if (hasActiveChat) {
          try {
            const lastPath = localStorage.getItem("antigravity_swiss_last_conversation_path") || "";
            const m = lastPath.match(/\/c\/([a-zA-Z0-9_-]+)/);
            if (m) convId = m[1];
          } catch (_) {}
        }
      }
      if (!convId) {
        document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
        return;
      }

      // 4. Query feedback buttons strictly within convoView and anchor to assistant action rows
      const feedbackButtons = convoView.querySelectorAll('button[aria-label="Good response"], button[aria-label="Bad response"]');
      const actionRowSet = new Set();

      feedbackButtons.forEach(btn => {
        const row = btn.closest('.flex.items-center') || btn.closest('.flex') || btn.parentElement;
        if (!row) return;

        // Apply strict Negative Exclusion Guards:
        if (row.closest('[role="dialog"], [aria-modal="true"], .settings-tab-content, [data-testid*="settings"], [data-testid="models-usage-container"], #swiss-aux-container, [id^="swiss-aux"], pre, code, .code-block, [data-testid="agent-input-box"]')) {
          return;
        }

        // Ensure row is strictly contained within convoView
        if (!convoView.contains(row)) return;

        actionRowSet.add(row);
      });

      const validActionRows = Array.from(actionRowSet);
      if (validActionRows.length === 0) {
        document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
        return;
      }

      // Purge any stray metrics not inside validActionRows
      document.querySelectorAll(".swiss-inchat-metrics").forEach(el => {
        if (!validActionRows.some(row => row.contains(el))) {
          el.remove();
        }
      });

      const now = Date.now();
      if (!chatTelemetryCachedData || chatTelemetryCachedData.convId !== convId || (now - chatTelemetryLastFetch) > 3000) {
        try {
          const cfgRes = await fetch("http://127.0.0.1:8765/api/gui/config", {
            headers: getSwissAuthHeaders(),
            signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(1500) : undefined
          });
          let cfg = {};
          if (cfgRes.ok) {
            const cfgJson = await cfgRes.json();
            cfg = cfgJson.config || cfgJson || {};
          } else {
            document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
            chatTelemetryCachedData = null;
            return;
          }
          const scope = cfg.chat_telemetry_scope || "aggregated";
          const metricsRes = await fetch("http://127.0.0.1:8765/api/tokens/chat-metrics?conversation_id=" + encodeURIComponent(convId) + "&scope=" + encodeURIComponent(scope), {
            headers: getSwissAuthHeaders(),
            signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(1500) : undefined
          });
          if (metricsRes.ok) {
            const metricsJson = await metricsRes.json();
            chatTelemetryCachedData = { convId: convId, cfg: cfg, metrics: metricsJson };
            chatTelemetryLastFetch = now;
          } else {
            document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
            chatTelemetryCachedData = null;
            return;
          }
        } catch (_) {
          document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
          chatTelemetryCachedData = null;
          return;
        }
      }

      if (!chatTelemetryCachedData || !chatTelemetryCachedData.metrics || !chatTelemetryCachedData.metrics.success) {
        document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
        return;
      }

      const showIn = cfg.chat_telemetry_input_tokens !== false;
      const showOut = cfg.chat_telemetry_output_tokens !== false;
      const showCache = cfg.chat_telemetry_cache_hit_ratio !== false;
      const showSpeed = cfg.chat_telemetry_generation_speed !== false;

      if (!showIn && !showOut && !showCache && !showSpeed) {
        document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove());
        return;
      }

      const turns = metrics.turns || [];
      const summary = metrics.summary || {
        input_tokens: 0,
        output_tokens: 0,
        cache_hit_ratio: 0,
        generation_speed: 0,
      };

      const offset = (turns.length > validActionRows.length && validActionRows.length > 0)
        ? turns.length - validActionRows.length
        : 0;

      validActionRows.forEach((row, i) => {
        const turnData = (offset + i < turns.length) ? turns[offset + i] : ((i < turns.length) ? turns[i] : summary);

        const inputVal = (turnData.input_tokens || 0).toLocaleString();
        const outputVal = (turnData.output_tokens || 0).toLocaleString();
        const cacheVal = (turnData.cache_hit_ratio || 0).toFixed(1) + "%";
        let rawSpeed = null;
        if (typeof turnData.speed_tps === 'number' && turnData.speed_tps > 0) {
          rawSpeed = turnData.speed_tps;
        } else if (typeof turnData.generation_speed === 'number' && turnData.generation_speed > 0) {
          rawSpeed = turnData.generation_speed;
        } else if (turnData.speed_tps && !isNaN(Number(turnData.speed_tps)) && Number(turnData.speed_tps) > 0) {
          rawSpeed = Number(turnData.speed_tps);
        } else if (turnData.generation_speed && !isNaN(Number(turnData.generation_speed)) && Number(turnData.generation_speed) > 0) {
          rawSpeed = Number(turnData.generation_speed);
        }
        const speedVal = rawSpeed !== null ? (rawSpeed.toFixed(1) + " TPS") : "— TPS";

        const items = [];
        if (showIn) {
          items.push('<span class="metric-item">Input Tokens: <span class="metric-val">' + inputVal + '</span></span>');
        }
        if (showOut) {
          items.push('<span class="metric-item">Output Tokens: <span class="metric-val">' + outputVal + '</span></span>');
        }
        if (showCache) {
          items.push('<span class="metric-item">Cache Hit Ratio: <span class="metric-val">' + cacheVal + '</span></span>');
        }
        if (showSpeed) {
          items.push('<span class="metric-item">Generation Speed: <span class="metric-val">' + speedVal + '</span></span>');
        }

        const metricsHTML = items.join('<span class="metric-sep">·</span>');

        let metricsEl = row.querySelector(".swiss-inchat-metrics");
        if (metricsEl) {
          if (metricsEl.innerHTML !== metricsHTML) {
            metricsEl.innerHTML = metricsHTML;
          }
        } else {
          metricsEl = document.createElement("div");
          metricsEl.className = "swiss-inchat-metrics";
          metricsEl.innerHTML = metricsHTML;
          row.prepend(metricsEl);
        }
      });
    }

    // Helper: Insert text into Antigravity chat input (Lexical or textarea)
    function insertTextToChatInput(text) {
      const input = document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
                    document.querySelector('[data-testid="chat-input-textarea"]') ||
                    document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
                    document.querySelector('[data-lexical-editor="true"]') ||
                    document.querySelector('.chat-input-container [contenteditable="true"]') ||
                    document.querySelector('.lexical-container [contenteditable="true"]') ||
                    document.querySelector('[role="textbox"][contenteditable="true"]') ||
                    document.querySelector('textarea[placeholder*="Ask"]') ||
                    document.querySelector('.chat-input-container textarea') ||
                    document.querySelector('textarea');
      if (!input) return;

      if (input.isContentEditable) {
        input.focus();
        const lex = input.__lexicalEditor;
        if (lex && typeof lex.update === "function" && lex._nodes) {
          lex.update(() => {
            const ParagraphNode = lex._nodes.get('paragraph')?.klass;
            const TextNode = lex._nodes.get('text')?.klass;
            const root = lex._editorState?._nodeMap?.get('root');
            if (ParagraphNode && TextNode && root) {
              const p = new ParagraphNode();
              const t = new TextNode(text);
              p.append(t);
              root.append(p);
            }
          });
        } else {
          const sel = window.getSelection();
          if (sel) {
            const range = document.createRange();
            range.selectNodeContents(input);
            range.collapse(false);
            sel.removeAllRanges();
            sel.addRange(range);
          }
          const hasContent = (input.innerText || "").trim().length > 0;
          const textToInsert = (hasContent ? "\n" : "") + text;
          try {
            document.execCommand("insertText", false, textToInsert);
          } catch (_) {}
          const hasInserted = (input.innerText || "").includes(text.slice(0, 15));
          if (!hasInserted) {
            try {
              input.dispatchEvent(new InputEvent("beforeinput", {
                inputType: "insertText",
                data: textToInsert,
                bubbles: true,
                cancelable: true
              }));
            } catch (_) {}
          }
        }
        input.dispatchEvent(new Event("input", { bubbles: true, composed: true }));

        // Fallback: paste event if execCommand was swallowed
        if (!input.innerText || !input.innerText.includes(text.slice(0, 15))) {
          try {
            const dt = new DataTransfer();
            dt.setData("text/plain", textToInsert);
            const pasteEvt = new ClipboardEvent("paste", { clipboardData: dt, bubbles: true, cancelable: true });
            input.dispatchEvent(pasteEvt);
          } catch (_) {}
        }
      } else {
        const curVal = input.value || "";
        input.value = (curVal.trim() ? curVal.trim() + "\n" : "") + text;
        input.selectionStart = input.selectionEnd = input.value.length;
        input.dispatchEvent(new Event("input", { bubbles: true }));
        input.dispatchEvent(new Event("change", { bubbles: true }));
      }
      input.focus();
    }

    // Helper: Convert base64 dataURL to File object for Drag and Paste events
    function dataURLtoFile(dataurl, filename) {
      if (!dataurl || typeof dataurl !== "string") return null;
      try {
        const parts = dataurl.split(",");
        if (parts.length < 2) return null;
        const mimeMatch = parts[0].match(/:(.*?);/);
        const mime = mimeMatch ? mimeMatch[1] : "image/png";
        const bstr = atob(parts[1]);
        let n = bstr.length;
        const u8arr = new Uint8Array(n);
        while (n--) {
          u8arr[n] = bstr.charCodeAt(n);
        }
        return new File([u8arr], filename || "sketch.png", { type: mime });
      } catch (_) {
        return null;
      }
    }

    // Helper: Insert image into Antigravity chat input (paste / drop event with File)
    function insertImageToChatInput(imageDataUrl, caption) {
      const input = document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
                    document.querySelector('[data-testid="chat-input-textarea"]') ||
                    document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
                    document.querySelector('[data-lexical-editor="true"]') ||
                    document.querySelector('.chat-input-container [contenteditable="true"]') ||
                    document.querySelector('.lexical-container [contenteditable="true"]') ||
                    document.querySelector('[role="textbox"][contenteditable="true"]') ||
                    document.querySelector('textarea[placeholder*="Ask"]') ||
                    document.querySelector('.chat-input-container textarea') ||
                    document.querySelector('textarea');
      if (!input) return;

      const file = dataURLtoFile(imageDataUrl, "memo-image.png");
      if (file) {
        try {
          const dt = new DataTransfer();
          dt.items.add(file);
          if (caption) dt.setData("text/plain", caption);
          const pasteEvt = new ClipboardEvent("paste", { clipboardData: dt, bubbles: true, cancelable: true });
          input.dispatchEvent(pasteEvt);

          const dropEvt = new DragEvent("drop", { dataTransfer: dt, bubbles: true, cancelable: true });
          input.dispatchEvent(dropEvt);
        } catch (_) {}
      }

      if (caption) {
        insertTextToChatInput(caption);
      }
      input.focus();
    }

    // Synchronous immediate initial setup
    setupAuxiliaryTabs();
    setupInChatTelemetry();
    setupBuiltinBrowserEnhancements();

    // Periodic check & mutation observer with singleton cleanup
    if (window.__swissAuxIntervalId) {
      clearInterval(window.__swissAuxIntervalId);
    }
    window.__swissAuxIntervalId = setInterval(() => {
      setupAuxiliaryTabs();
      setupInChatTelemetry();
      setupBuiltinBrowserEnhancements();
      syncAuxProjectScope();
    }, 2500);

    if (window.__swissAuxObserverInstance) {
      try { window.__swissAuxObserverInstance.disconnect(); } catch (_) {}
    }

    let auxRaf = null;
    const observer = new MutationObserver((mutations) => {
      // Ignore mutations originating from Swiss elements to prevent feedback loops
      const hasExternal = mutations.some(m => {
        if (m.type === "attributes" && m.attributeName && m.attributeName.startsWith("data-swiss-")) return false;
        const t = m.target;
        if (t && t.nodeType === 1) {
          if (t.id === "swiss-aux-container" || t.closest?.("#swiss-aux-container")) return false;
          if (t.classList?.contains("swiss-inchat-metrics") || t.closest?.(".swiss-inchat-metrics") || t.classList?.contains("swiss-telemetry-badge") || t.closest?.(".swiss-telemetry-badge")) return false;
          if (t.classList?.contains("swiss-aux-tab-btn") || t.closest?.(".swiss-aux-tab-btn")) return false;
          if (t.classList?.contains("swiss-aux-tabs-divider") || t.closest?.(".swiss-aux-tabs-divider")) return false;
          if (t.classList?.contains("swiss-aux-btn-group") || t.closest?.(".swiss-aux-btn-group")) return false;
          if (t.classList?.contains("swiss-browser-toolbar") || t.closest?.(".swiss-browser-toolbar")) return false;
          if (t.classList?.contains("swiss-browser-canvas-overlay") || t.closest?.(".swiss-browser-canvas-overlay")) return false;
          if (t.classList?.contains("swiss-builtin-browser-view") || t.closest?.(".swiss-builtin-browser-view")) return false;
        }
        return true;
      });
      if (!hasExternal) return;

      if (auxRaf) return;
      auxRaf = requestAnimationFrame(() => {
        auxRaf = null;
        document.querySelectorAll('[role="dialog"] .swiss-inchat-metrics, [aria-modal="true"] .swiss-inchat-metrics, .settings-tab-content .swiss-inchat-metrics, #swiss-aux-container .swiss-inchat-metrics, pre .swiss-inchat-metrics, code .swiss-inchat-metrics').forEach(el => el.remove());
        setupAuxiliaryTabs();
        setupInChatTelemetry();
        setupBuiltinBrowserEnhancements();
        syncAuxProjectScope();
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
