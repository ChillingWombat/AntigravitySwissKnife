package plugins

// GenerateGitHubExtensionCSS returns Google-style minimalist CSS for the Left Panel Extensions Bar,
// GitHub Workspace panel, Agent Task Tracker badges, In-Place Editor, and Main Stage View.
func GenerateGitHubExtensionCSS() string {
	return `/* Antigravity Swiss Knife - Left Panel Extensions & Main Stage View */

/* Left Sidebar Extension Navigation Tabs (Styled exactly like Scheduled Tasks) */
.swiss-left-nav-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.swiss-left-tabs-separator {
  height: 1px;
  background-color: var(--border, #e2e8f0);
  opacity: 0.4;
  margin: 4px 6px;
}

.swiss-left-nav-tab {
  user-select: none;
  font-family: inherit;
  font-size: 13px;
  transition: all 0.15s ease;
}

.swiss-left-nav-tab.active {
  font-weight: 500;
}

/* Right Panel Auxiliary GitHub Workspace View */
.swiss-github-aux-view {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  box-sizing: border-box;
  background: var(--canvas, transparent);
}

.swiss-gh-repo-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  background: rgba(0, 0, 0, 0.02);
  border-bottom: 1px solid var(--border, #e2e8f0);
  font-size: 11px;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-repo-bar {
  background: rgba(255, 255, 255, 0.02);
  border-color: rgba(255, 255, 255, 0.06);
}

.swiss-gh-repo-name {
  font-weight: 600;
  color: var(--text, #1e293b);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-repo-name {
  color: #f1f5f9;
}

.swiss-gh-branch-chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 2px 6px;
  font-size: 10px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.04);
  color: var(--text-muted, #64748b);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-branch-chip {
  background: rgba(255, 255, 255, 0.06);
  color: #94a3b8;
}

/* Filter Sub-Tabs */
.swiss-gh-tabs-row {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--border, #e2e8f0);
  padding: 0 4px;
  background: transparent;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-tabs-row {
  border-color: rgba(255, 255, 255, 0.06);
}

.swiss-gh-tab-btn {
  flex: 1;
  text-align: center;
  padding: 6px 2px;
  font-size: 10.5px;
  font-weight: 500;
  color: var(--text-muted, #64748b);
  border: none;
  border-bottom: 2px solid transparent;
  background: transparent;
  cursor: pointer;
  transition: all 0.12s ease;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-tab-btn {
  color: #94a3b8;
}
.swiss-gh-tab-btn:hover {
  color: var(--text, #0f172a);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-tab-btn:hover {
  color: #f8fafc;
}
.swiss-gh-tab-btn.active {
  color: #1a73e8;
  border-bottom-color: #1a73e8;
  font-weight: 600;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-tab-btn.active {
  color: #8ab4f8;
  border-bottom-color: #8ab4f8;
}

/* Search bar */
.swiss-gh-search-box {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 8px;
  border-bottom: 1px solid var(--border, #e2e8f0);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-search-box {
  border-color: rgba(255, 255, 255, 0.06);
}
.swiss-gh-search-input {
  width: 100%;
  font-size: 11px;
  padding: 3px 6px;
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 4px;
  background: var(--input, transparent);
  color: var(--text, #0f172a);
  outline: none;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-search-input {
  border-color: rgba(255, 255, 255, 0.1);
  color: #f8fafc;
}
.swiss-gh-search-input:focus {
  border-color: #1a73e8;
}

/* Card Items List */
.swiss-gh-list {
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  flex: 1;
  padding: 4px;
  gap: 4px;
}

.swiss-gh-card {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 6px 8px;
  border-radius: 6px;
  border: 1px solid var(--border, #e2e8f0);
  background: var(--card, #ffffff);
  cursor: grab;
  transition: all 0.12s ease;
  user-select: none;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-card {
  background: #1f2022;
  border-color: rgba(255, 255, 255, 0.08);
}
.swiss-gh-card:hover {
  border-color: rgba(26, 115, 232, 0.4);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-card:hover {
  border-color: rgba(138, 180, 248, 0.4);
}
.swiss-gh-card:active {
  cursor: grabbing;
}
.swiss-gh-card.dragging {
  opacity: 0.5;
  border: 1px dashed #1a73e8;
}

.swiss-gh-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.swiss-gh-badge-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.swiss-gh-num-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 4px;
  border-radius: 3px;
  line-height: 1.2;
}
.swiss-gh-num-badge.open {
  background: rgba(34, 197, 94, 0.12);
  color: #16a34a;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-num-badge.open {
  background: rgba(74, 222, 128, 0.15);
  color: #4ade80;
}
.swiss-gh-num-badge.closed {
  background: rgba(100, 116, 139, 0.12);
  color: #64748b;
}
.swiss-gh-num-badge.pr {
  background: rgba(168, 85, 247, 0.12);
  color: #9333ea;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-num-badge.pr {
  background: rgba(192, 132, 252, 0.15);
  color: #c084fc;
}

.swiss-gh-card-title {
  font-size: 11px;
  font-weight: 500;
  color: var(--text, #1e293b);
  line-height: 1.3;
  word-break: break-word;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-card-title {
  color: #f1f5f9;
}

.swiss-gh-chips-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 3px;
  margin-top: 2px;
}

.swiss-gh-label-chip {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.05);
  color: var(--text-muted, #64748b);
  line-height: 1.2;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-label-chip {
  background: rgba(255, 255, 255, 0.08);
  color: #94a3b8;
}

/* Agent Working Badge */
.swiss-agent-task-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border-radius: 12px;
  font-size: 9.5px;
  font-weight: 600;
  line-height: 1;
}
.swiss-agent-task-badge.working {
  background: rgba(34, 197, 94, 0.12);
  color: #15803d;
  border: 1px solid rgba(34, 197, 94, 0.3);
}
:is(.dark, [data-theme="dark"]) .swiss-agent-task-badge.working {
  background: rgba(34, 197, 94, 0.18);
  color: #4ade80;
  border-color: rgba(74, 222, 128, 0.3);
}
.swiss-agent-task-badge.idle {
  background: rgba(148, 163, 184, 0.12);
  color: #475569;
}
:is(.dark, [data-theme="dark"]) .swiss-agent-task-badge.idle {
  background: rgba(148, 163, 184, 0.15);
  color: #94a3b8;
}

.swiss-agent-pulse-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #16a34a;
  animation: swissPulse 1.8s infinite;
}
:is(.dark, [data-theme="dark"]) .swiss-agent-pulse-dot {
  background: #4ade80;
}
@keyframes swissPulse {
  0% { transform: scale(0.9); opacity: 0.8; }
  50% { transform: scale(1.3); opacity: 1; }
  100% { transform: scale(0.9); opacity: 0.8; }
}

.swiss-gh-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 4px;
  padding-top: 4px;
  border-top: 1px dashed var(--border, #f1f5f9);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-card-footer {
  border-color: rgba(255, 255, 255, 0.05);
}

.swiss-gh-card-actions {
  display: flex;
  align-items: center;
  gap: 3px;
}

.swiss-gh-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 2px 5px;
  font-size: 10px;
  border-radius: 4px;
  background: transparent;
  border: none;
  color: var(--text-muted, #64748b);
  cursor: pointer;
  transition: all 0.1s ease;
}
.swiss-gh-action-btn:hover {
  background: rgba(0, 0, 0, 0.06);
  color: var(--text, #0f172a);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-action-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #f8fafc;
}

/* Chat Composer Drop Target Highlight */
.swiss-chat-drop-highlight {
  outline: 2px dashed #1a73e8 !important;
  outline-offset: -2px !important;
  background: rgba(26, 115, 232, 0.05) !important;
}

/* ========================================================================= */
/* MAIN STAGE VIEW CONTAINER (Replaces / Splits Chat Viewport)               */
/* ========================================================================= */
#swiss-main-stage-container {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  box-sizing: border-box;
  overflow: hidden;
  z-index: 20;
}
:is(.dark, [data-theme="dark"]) #swiss-main-stage-container {
  background: #131314;
  color: #f1f5f9;
}

#swiss-main-stage-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 42px;
  min-height: 42px;
  padding: 0 12px;
  background: var(--card, #f8fafc);
  border-bottom: 1px solid var(--border, #e2e8f0);
  box-sizing: border-box;
  user-select: none;
}
:is(.dark, [data-theme="dark"]) #swiss-main-stage-header {
  background: #1e1f20;
  border-color: rgba(255, 255, 255, 0.08);
}

.swiss-main-stage-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
}

.swiss-main-stage-tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: var(--text-muted, #64748b);
  cursor: pointer;
  transition: all 0.12s ease;
}
:is(.dark, [data-theme="dark"]) .swiss-main-stage-tab {
  color: #94a3b8;
}
.swiss-main-stage-tab:hover {
  background: rgba(0, 0, 0, 0.05);
  color: var(--text, #0f172a);
}
:is(.dark, [data-theme="dark"]) .swiss-main-stage-tab:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #f8fafc;
}
.swiss-main-stage-tab.active {
  background: #1a73e8;
  color: #ffffff;
  font-weight: 600;
}
:is(.dark, [data-theme="dark"]) .swiss-main-stage-tab.active {
  background: #8ab4f8;
  color: #101010;
}

.swiss-main-stage-controls {
  display: flex;
  align-items: center;
  gap: 6px;
}

.swiss-stage-mode-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  font-size: 11px;
  border-radius: 5px;
  border: 1px solid var(--border, #e2e8f0);
  background: transparent;
  color: var(--text-muted, #64748b);
  cursor: pointer;
}
:is(.dark, [data-theme="dark"]) .swiss-stage-mode-btn {
  border-color: rgba(255, 255, 255, 0.1);
  color: #94a3b8;
}
.swiss-stage-mode-btn:hover {
  background: rgba(0, 0, 0, 0.04);
  color: var(--text, #0f172a);
}
:is(.dark, [data-theme="dark"]) .swiss-stage-mode-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #f8fafc;
}
.swiss-stage-mode-btn.active {
  background: rgba(26, 115, 232, 0.1);
  color: #1a73e8;
  border-color: #1a73e8;
  font-weight: 600;
}
:is(.dark, [data-theme="dark"]) .swiss-stage-mode-btn.active {
  background: rgba(138, 180, 248, 0.15);
  color: #8ab4f8;
  border-color: #8ab4f8;
}

.swiss-stage-close-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 5px;
  border: none;
  background: rgba(239, 68, 68, 0.1);
  color: #dc2626;
  cursor: pointer;
}
:is(.dark, [data-theme="dark"]) .swiss-stage-close-btn {
  background: rgba(239, 68, 68, 0.18);
  color: #f87171;
}
.swiss-stage-close-btn:hover {
  background: #dc2626;
  color: #ffffff;
}

#swiss-main-stage-body {
  display: flex;
  flex: 1;
  min-height: 0;
  width: 100%;
  overflow: hidden;
  box-sizing: border-box;
}

/* Kanban Board & Columns Styling */
.swiss-gh-kanban-board {
  display: flex;
  flex: 1;
  width: 100%;
  height: 100%;
  gap: 12px;
  padding: 12px;
  overflow-x: auto;
  box-sizing: border-box;
  background: var(--canvas-subtle, rgba(0, 0, 0, 0.01));
}

.swiss-gh-kanban-col {
  display: flex;
  flex-direction: column;
  flex: 1 1 240px;
  min-width: 220px;
  max-width: 360px;
  height: 100%;
  border-radius: 8px;
  border: 1px solid var(--border, #e2e8f0);
  background: var(--canvas, #ffffff);
  overflow: hidden;
  box-sizing: border-box;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-kanban-col {
  background: #18191a;
  border-color: rgba(255, 255, 255, 0.08);
}

.swiss-gh-kanban-col-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border, #e2e8f0);
  background: rgba(0, 0, 0, 0.02);
  font-size: 12px;
  font-weight: 600;
  user-select: none;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-kanban-col-header {
  background: rgba(255, 255, 255, 0.02);
  border-color: rgba(255, 255, 255, 0.06);
}

.swiss-gh-kanban-col-title-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.swiss-gh-col-counter {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.06);
  color: var(--text-muted, #64748b);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-col-counter {
  background: rgba(255, 255, 255, 0.08);
  color: #94a3b8;
}

.swiss-gh-kanban-col-cards {
  display: flex;
  flex-direction: column;
  flex: 1;
  padding: 8px;
  gap: 8px;
  overflow-y: auto;
  min-height: 80px;
  transition: background-color 0.15s ease, border-color 0.15s ease;
}

.swiss-gh-kanban-col-cards.drag-over {
  background: rgba(26, 115, 232, 0.06);
  outline: 2px dashed #1a73e8;
  outline-offset: -3px;
  border-radius: 6px;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-kanban-col-cards.drag-over {
  background: rgba(138, 180, 248, 0.08);
  outline-color: #8ab4f8;
}

/* Kanban Cards */
.swiss-gh-kanban-card {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid var(--border, #e2e8f0);
  background: var(--card, #ffffff);
  cursor: grab;
  transition: transform 0.12s ease, box-shadow 0.12s ease, border-color 0.12s ease;
  user-select: none;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-kanban-card {
  background: #202124;
  border-color: rgba(255, 255, 255, 0.08);
}
.swiss-gh-kanban-card:hover {
  border-color: #1a73e8;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.06);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-kanban-card:hover {
  border-color: #8ab4f8;
}
.swiss-gh-kanban-card:active {
  cursor: grabbing;
}
.swiss-gh-kanban-card.dragging {
  opacity: 0.45;
  border: 1px dashed #1a73e8;
}

/* View Switcher Button Group */
.swiss-gh-view-switcher {
  display: inline-flex;
  align-items: center;
  padding: 2px;
  background: rgba(0, 0, 0, 0.04);
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 6px;
  gap: 2px;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-view-switcher {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.08);
}

.swiss-gh-view-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  font-size: 11px;
  font-weight: 500;
  border: none;
  background: transparent;
  color: var(--text-muted, #64748b);
  border-radius: 4px;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.12s ease;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-view-btn {
  color: #94a3b8;
}
.swiss-gh-view-btn.active {
  background: var(--canvas, #ffffff);
  color: var(--text, #1e293b);
  font-weight: 600;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-view-btn.active {
  background: #2b2c2f;
  color: #f1f5f9;
}

/* GitHub Workspace 3-Column Layout */
.swiss-gh-workspace-layout {

  display: flex;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

.swiss-gh-col-left {
  width: 320px;
  min-width: 260px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border, #e2e8f0);
  background: var(--card, #ffffff);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-col-left {
  background: #18191a;
  border-color: rgba(255, 255, 255, 0.08);
}

.swiss-gh-col-center {
  flex: 1;
  min-width: 360px;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  padding: 16px 20px;
  background: var(--canvas, #ffffff);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-col-center {
  background: #131314;
}

.swiss-gh-col-right {
  width: 300px;
  min-width: 250px;
  display: flex;
  flex-direction: column;
  border-left: 1px solid var(--border, #e2e8f0);
  background: var(--card, #fafafa);
}
:is(.dark, [data-theme="dark"]) .swiss-gh-col-right {
  background: #18191a;
  border-color: rgba(255, 255, 255, 0.08);
}

/* Detail Editor Components */
.swiss-gh-editor-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text, #0f172a);
  margin-bottom: 8px;
  border: 1px solid transparent;
  padding: 4px 6px;
  border-radius: 4px;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-editor-title {
  color: #f8fafc;
}
.swiss-gh-editor-title:focus {
  border-color: #1a73e8;
  background: var(--input, transparent);
  outline: none;
}

.swiss-gh-editor-meta-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border, #e2e8f0);
  font-size: 12px;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-editor-meta-bar {
  border-color: rgba(255, 255, 255, 0.08);
}

.swiss-gh-textarea {
  width: 100%;
  min-height: 160px;
  font-family: inherit;
  font-size: 13px;
  padding: 10px;
  border-radius: 6px;
  border: 1px solid var(--border, #e2e8f0);
  background: var(--input, transparent);
  color: var(--text, #0f172a);
  resize: vertical;
  outline: none;
  box-sizing: border-box;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-textarea {
  border-color: rgba(255, 255, 255, 0.12);
  color: #f8fafc;
}
.swiss-gh-textarea:focus {
  border-color: #1a73e8;
}

.swiss-gh-save-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 6px;
  border: none;
  background: #1a73e8;
  color: #ffffff;
  cursor: pointer;
  transition: all 0.12s ease;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-save-btn {
  background: #8ab4f8;
  color: #101010;
}
.swiss-gh-save-btn:hover {
  opacity: 0.9;
}

/* Agent Task Board (Column 3) */
.swiss-agent-board-header {
  padding: 10px 14px;
  font-size: 12px;
  font-weight: 600;
  border-bottom: 1px solid var(--border, #e2e8f0);
}
:is(.dark, [data-theme="dark"]) .swiss-agent-board-header {
  border-color: rgba(255, 255, 255, 0.08);
}

.swiss-agent-conv-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border, #f1f5f9);
  transition: background 0.1s ease;
}
:is(.dark, [data-theme="dark"]) .swiss-agent-conv-item {
  border-color: rgba(255, 255, 255, 0.04);
}
.swiss-agent-conv-item:hover {
  background: rgba(0, 0, 0, 0.02);
}
:is(.dark, [data-theme="dark"]) .swiss-agent-conv-item:hover {
  background: rgba(255, 255, 255, 0.03);
}

/* Slide-over / Modal for Left Panel editing */
.swiss-gh-modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(2px);
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
}

.swiss-gh-modal-dialog {
  width: 90%;
  max-width: 680px;
  max-height: 85vh;
  background: var(--canvas, #ffffff);
  border-radius: 10px;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.2);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-modal-dialog {
  background: #1e1f20;
  color: #f8fafc;
}

/* GitHub Context Menu */
.swiss-gh-context-menu {
  position: fixed;
  z-index: 100000;
  min-width: 180px;
  background: var(--surface, #ffffff);
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12), 0 1px 3px rgba(0, 0, 0, 0.08);
  padding: 4px 0;
  display: flex;
  flex-direction: column;
  user-select: none;
  font-family: inherit;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-context-menu {
  background: #1e1f22;
  border-color: rgba(255, 255, 255, 0.1);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4), 0 1px 3px rgba(0, 0, 0, 0.2);
}

.swiss-gh-menu-header {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted, #64748b);
  padding: 6px 12px 4px;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-menu-header {
  color: #94a3b8;
}

.swiss-gh-menu-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 12px;
  font-size: 12px;
  color: var(--text, #1e293b);
  background: transparent;
  border: none;
  text-align: left;
  cursor: pointer;
  transition: background 0.1s ease;
  width: 100%;
  box-sizing: border-box;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-menu-item {
  color: #f1f5f9;
}
.swiss-gh-menu-item:hover {
  background: rgba(26, 115, 232, 0.08);
  color: #1a73e8;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-menu-item:hover {
  background: rgba(138, 180, 248, 0.12);
  color: #8ab4f8;
}
.swiss-gh-menu-item.active {
  font-weight: 600;
}
.swiss-gh-menu-check {
  font-size: 11px;
  color: #1a73e8;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-menu-check {
  color: #8ab4f8;
}

.swiss-gh-menu-divider {
  height: 1px;
  background: var(--border, #e2e8f0);
  margin: 4px 0;
  opacity: 0.6;
}
:is(.dark, [data-theme="dark"]) .swiss-gh-menu-divider {
  background: rgba(255, 255, 255, 0.08);
}
`
}
