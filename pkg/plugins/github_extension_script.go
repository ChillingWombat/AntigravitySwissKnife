package plugins

// GenerateGitHubExtensionScript returns client-side JavaScript injected into Antigravity 2.0
// providing the Left Panel Extensions Bar, GitHub Workspace, Drag & Drop to Chat,
// In-Place Editor, Agent Task Correlation, and Main Stage View.
func GenerateGitHubExtensionScript() string {
	return `/* Antigravity Swiss Knife - Left Panel Extensions & Main Stage Engine */
(() => {
  if (typeof window === "undefined") return;
  const existingStageContainer = document.getElementById("swiss-main-stage-container");
  const existingActiveTab = document.querySelector("#swiss-main-stage-header .swiss-main-stage-tab.active")?.getAttribute("data-ext");
  const prevOpenStageExt = (existingStageContainer && existingStageContainer.style.display === "flex")
    ? (existingActiveTab || window.__swissActiveMainStageExt || "github")
    : (window.__swissActiveMainStageExt || null);
  if (window.__swissGitHubExtInitialized) {
    if (window.__swissGHNavInterval) {
      clearInterval(window.__swissGHNavInterval);
      window.__swissGHNavInterval = null;
    }
    if (window.__swissGHFetchInterval) {
      clearInterval(window.__swissGHFetchInterval);
      window.__swissGHFetchInterval = null;
    }
    if (window.__swissGHDaemonCheckInterval) {
      clearInterval(window.__swissGHDaemonCheckInterval);
      window.__swissGHDaemonCheckInterval = null;
    }
    document.getElementById("swiss-github-ext-styles")?.remove();
    document.getElementById("swiss-main-stage-header")?.remove();
    document.getElementById("swiss-left-nav-group")?.remove();
  }
  window.__swissGitHubExtInitialized = true;

  try {
    if (document.head) {
      let extStyles = document.getElementById("swiss-github-ext-styles");
      if (!extStyles) {
        extStyles = document.createElement("style");
        extStyles.id = "swiss-github-ext-styles";
        document.head.appendChild(extStyles);
      }
      extStyles.textContent = ` + "`" + GenerateGitHubExtensionCSS() + "`" + `;
    }
  } catch (_) {}

  let currentRepo = null;
  let cachedIssues = [];
  let cachedPRs = [];
  let cachedProjects = [];
  let cachedAgentTasks = [];
  let cachedKanbanBoard = null;
  let activeTab = "board"; // "board" | "issues" | "prs" | "tasks"
  let activeFilter = "all"; // "all" | "open" | "mine"
  let activeSearch = "";
  let isLeftPanelOpen = true;
  let activeMainStageExt = null; // null | "github" | "browser" | "files" | "memos"
  let stageViewMode = "kanban"; // "kanban" | "list"
  let selectedItem = null;
  let draggedKanbanCard = null;

  // Extension Scope State & Project Tracking
  let currentScope = "GLOBAL";
  try {
    const savedScope = localStorage.getItem("antigravity_swiss_extension_scope");
    if (savedScope) currentScope = savedScope;
  } catch (_) {}
  let availableProjects = [];
  let knownProjects = [];
  let lastInteractedProject = null;
  let lastInteractedConversation = null;
  try {
    lastInteractedProject = localStorage.getItem("antigravity_swiss_last_project") || null;
    lastInteractedConversation = localStorage.getItem("antigravity_swiss_last_conversation") || null;
  } catch (_) {}

  let auxRepo = null;
  let auxIssues = null;
  let auxPRs = null;
  let auxAgentTasks = null;
  let auxKanbanBoard = null;
  let lastFetchedAuxWs = null;

  let isDaemonOnline = (window.__swissDaemonOnline !== false);
  function setDaemonOnline(online) {
    if (window.__swissDaemonOnline === online) return;
    window.__swissDaemonOnline = online;
    isDaemonOnline = online;
    if (!online) {
      const g = document.getElementById("swiss-left-nav-group");
      if (g) g.remove();
      if (activeMainStageExt !== null) {
        closeMainStage();
      }
      if (typeof window.__swissOnAuxDaemonChanged === "function") {
        window.__swissOnAuxDaemonChanged(false);
      }
    } else {
      setupLeftNavTabs();
      if (typeof window.__swissOnAuxDaemonChanged === "function") {
        window.__swissOnAuxDaemonChanged(true);
      }
    }
  }
  window.setSwissDaemonOnline = setDaemonOnline;

  async function checkDaemonConnection() {
    try {
      const res = await fetch("http://127.0.0.1:8765/api/status", {
        method: "GET",
        signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(1500) : undefined,
      });
      if (res && res.ok) {
        setDaemonOnline(true);
        return;
      }
    } catch (_) {}
    setDaemonOnline(false);
  }

  function getLeftSidebar() {
    return document.getElementById("swiss-left-nav-group")?.closest(".bg-sidebar") ||
      document.querySelector('[data-testid="automations-button"]')?.closest(".bg-sidebar") ||
      document.querySelector('[data-testid="new-conversation-button"]')?.closest(".bg-sidebar") ||
      document.querySelector('[data-testid="conversation-list-sidebar"]')?.closest(".bg-sidebar") ||
      Array.from(document.querySelectorAll(".bg-sidebar")).find(el => el.classList.contains("flex-col") || !el.classList.contains("w-full"));
  }

  function addKnownProject(name) {
    const clean = (name || "").trim();
    if (!clean) return;
    if (!availableProjects.includes(clean)) {
      availableProjects.push(clean);
    }
    if (!knownProjects.some(p => p.name === clean)) {
      knownProjects.push({ name: clean });
    }
  }

  function syncKnownProjectsFromDOM() {
    try {
      availableProjects.forEach(addKnownProject);
      document.querySelectorAll("[data-swiss-project], [data-project-label]").forEach(el => {
        const p = el.getAttribute("data-swiss-project") || el.getAttribute("data-project-label");
        if (p) addKnownProject(p);
      });
      document.querySelectorAll('[data-testid="conversation-row-sidebar"][data-subtext]').forEach(el => {
        const sub = (el.getAttribute("data-subtext") || "").trim();
        if (sub) addKnownProject(sub);
      });
      const links = document.querySelectorAll('a[aria-label="New Conversation in Project"]');
      links.forEach(l => {
        const header = l.closest('.group\\/header, [data-project-card], [data-index]');
        const text = (header ? header.textContent : "").trim();
        if (text) addKnownProject(text);
      });
    } catch (_) {}
  }

  function getConversationIdFromRow(row) {
    if (!row) return "";
    const cascadeId = row.getAttribute?.("data-cascade-id") || row.dataset?.cascadeId;
    if (cascadeId) return cascadeId;
    const directId = row.dataset?.id || row.getAttribute?.("data-id") || row.getAttribute?.("data-conversation-id");
    if (directId) return directId;
    const link = (row.tagName === "A" && row.getAttribute?.("href")?.includes("/c/"))
      ? row
      : row.querySelector?.('a[href^="/c/"], a[href*="/c/"]');
    if (link) {
      const href = link.getAttribute("href") || "";
      const m = href.match(/\/c\/([^/?#]+)/);
      if (m && m[1]) return m[1];
    }
    return "";
  }

  function recordProjectInteraction(projectName, convId) {
    if (projectName && projectName.trim()) {
      lastInteractedProject = projectName.trim();
      addKnownProject(lastInteractedProject);
      try { localStorage.setItem("antigravity_swiss_last_project", lastInteractedProject); } catch (_) {}
    }
    if (convId && convId.trim()) {
      lastInteractedConversation = convId.trim();
      try { localStorage.setItem("antigravity_swiss_last_conversation", lastInteractedConversation); } catch (_) {}
    }
  }

  function extractProjectFromElement(el) {
    if (!el || typeof el.closest !== "function") return "";
    const directAttr = el.closest("[data-swiss-project]")?.getAttribute("data-swiss-project") ||
                       el.closest("[data-project-label]")?.getAttribute("data-project-label");
    if (directAttr && directAttr.trim()) return directAttr.trim();
    const subtextAttr = el.getAttribute?.("data-subtext") ||
                        el.closest("[data-subtext]")?.getAttribute("data-subtext");
    if (subtextAttr && subtextAttr.trim()) return subtextAttr.trim();
    const hdr = el.closest('.group\\/header, [data-project-card]');
    if (hdr && hdr.textContent && hdr.textContent.trim()) return hdr.textContent.trim();
    const idxEl = el.closest("[data-index]");
    let prev = idxEl ? idxEl.previousElementSibling : null;
    while (prev) {
      if (prev.getAttribute?.("data-testid") === "section-header" || prev.querySelector?.('[data-testid="section-header"]')) {
        break;
      }
      const pAttr = prev.getAttribute?.("data-swiss-project") ||
                    prev.querySelector?.("[data-swiss-project]")?.getAttribute("data-swiss-project") ||
                    prev.querySelector?.("[data-project-label]")?.getAttribute("data-project-label");
      if (pAttr && pAttr.trim()) return pAttr.trim();
      const prevHdr = prev.querySelector?.('.group\\/header, [data-project-card]');
      if (prevHdr && prevHdr.textContent && prevHdr.textContent.trim()) return prevHdr.textContent.trim();
      prev = prev.previousElementSibling;
    }
    return "";
  }

  function detectCurrentConversationProject() {
    try {
      syncKnownProjectsFromDOM();
      const m = window.location.pathname ? window.location.pathname.match(/\/c\/([^/?#]+)/) : null;
      const cid = (m && m[1]) ? m[1] : "";
      const sidebar = getLeftSidebar();
      const searchRoot = sidebar || document;

      // 1. Check the first [data-testid="breadcrumb-segment"] text content and match against knownProjects
      const firstBreadcrumb = document.querySelector('[data-testid="breadcrumb-segment"]');
      if (firstBreadcrumb && firstBreadcrumb.textContent) {
        const segmentText = firstBreadcrumb.textContent.trim();
        if (segmentText) {
          const matched = knownProjects.find(p => p.name === segmentText) ||
                          (availableProjects.includes(segmentText) ? { name: segmentText } : null);
          const allBreadcrumbs = document.querySelectorAll('[data-testid="breadcrumb-segment"]');
          if (matched) {
            recordProjectInteraction(matched.name, cid);
            return matched.name;
          } else if (allBreadcrumbs.length >= 2) {
            recordProjectInteraction(segmentText, cid);
            return segmentText;
          }
        }
      }

      // 2. Match conversation ID against sidebar rows (supporting data-cascade-id and a[href^="/c/"])
      if (cid) {
        const rowSelector = '[data-testid="conversation-row-sidebar"][data-cascade-id="' + cid + '"], ' +
                            'a[href^="/c/' + cid + '"], a[href*="/c/' + cid + '"], ' +
                            '[data-testid="conversation-row-sidebar"][data-id="' + cid + '"], ' +
                            '[data-conversation-id="' + cid + '"]';
        const row = searchRoot.querySelector(rowSelector) || document.querySelector(rowSelector);
        if (row) {
          const pName = extractProjectFromElement(row);
          if (pName) {
            recordProjectInteraction(pName, cid);
            return pName;
          }
        }
      }

      // 3. Check active conversation row in getLeftSidebar()
      const activeSelector = '[data-testid="conversation-row-sidebar"][aria-selected="true"], ' +
                             '[data-testid="conversation-row-sidebar"][data-selected="true"], ' +
                             '[data-testid="conversation-row-sidebar"].bg-accent, ' +
                             '[data-testid="conversation-row-sidebar"].bg-sidebar-accent, ' +
                             '[data-testid="conversation-row-sidebar"][data-state="active"]';
      const activeRow = searchRoot.querySelector(activeSelector) || document.querySelector(activeSelector);
      if (activeRow) {
        const activeCid = getConversationIdFromRow(activeRow) || cid;
        const pName = extractProjectFromElement(activeRow);
        if (pName) {
          recordProjectInteraction(pName, activeCid);
          return pName;
        }
      }

      // 4. Fallback to document.title
      const title = document.title || "";
      const parts = title.split(" - ");
      if (parts.length >= 3 && parts[parts.length - 2].trim()) {
        const pName = parts[parts.length - 2].trim();
        recordProjectInteraction(pName, cid);
        return pName;
      }
    } catch (_) {}
    return lastInteractedProject || null;
  }

  function getActiveProjectWorkspacePath() {
    return detectCurrentConversationProject() || lastInteractedProject || (availableProjects.length > 0 ? availableProjects[0] : ".");
  }

  function getEffectiveWorkspacePath(forAux = false) {
    if (forAux || !activeMainStageExt) {
      return getActiveProjectWorkspacePath();
    }
    if (currentScope && currentScope !== "GLOBAL") {
      return currentScope;
    }
    // For extensions requiring a project when in GLOBAL scope, default to latest interacted project
    if (lastInteractedProject || detectCurrentConversationProject()) {
      return lastInteractedProject || detectCurrentConversationProject();
    }
    if (availableProjects.length > 0) {
      return availableProjects[0];
    }
    return "GLOBAL";
  }

  window.__swissGetMainStageScope = () => currentScope || "GLOBAL";
  window.__swissGetActiveProject = () => getActiveProjectWorkspacePath();

  async function fetchAvailableProjects() {
    try {
      const res = await fetch("http://127.0.0.1:8765/api/gui/projects");
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data)) {
          const list = data.filter(p => !p.is_archived).map(p => p.name).filter(Boolean);
          list.forEach(p => addKnownProject(p));
        }
      }
    } catch (_) {}
    syncKnownProjectsFromDOM();
  }

  // 1. Toast notifications helper
  function showToast(msg, isError = false) {
    if (window.__swissToast) {
      window.__swissToast(msg);
      return;
    }
    let toast = document.getElementById("swiss-gh-toast");
    if (!toast) {
      toast = document.createElement("div");
      toast.id = "swiss-gh-toast";
      toast.style.cssText = "position: fixed; bottom: 24px; right: 24px; z-index: 10000; padding: 8px 14px; border-radius: 6px; font-size: 12px; font-weight: 500; box-shadow: 0 4px 12px rgba(0,0,0,0.15); transition: opacity 0.2s; pointer-events: none;";
      document.body.appendChild(toast);
    }
    toast.style.background = isError ? "#ef4444" : "#1a73e8";
    toast.style.color = "#ffffff";
    toast.textContent = msg;
    toast.style.opacity = "1";
    setTimeout(() => { if (toast) toast.style.opacity = "0"; }, 2800);
  }

  // 2. Fetch Repo & Workspace Data
  async function fetchWorkspaceBundle(wsPathEncoded) {
    const bundle = { repo: null, issues: [], prs: [], tasks: [], board: null };
    const resRepo = await fetch("http://127.0.0.1:8765/api/github/repo?workspace_path=" + wsPathEncoded);
    if (resRepo.ok) {
      const data = await resRepo.json();
      if (data.success && data.repo) bundle.repo = data.repo;
    }
    const resIssues = await fetch("http://127.0.0.1:8765/api/github/issues?workspace_path=" + wsPathEncoded + "&state=all");
    if (resIssues.ok) {
      const data = await resIssues.json();
      if (data.success && data.issues) bundle.issues = data.issues;
    }
    const resPRs = await fetch("http://127.0.0.1:8765/api/github/prs?workspace_path=" + wsPathEncoded + "&state=all");
    if (resPRs.ok) {
      const data = await resPRs.json();
      if (data.success && data.prs) bundle.prs = data.prs;
    }
    const resTasks = await fetch("http://127.0.0.1:8765/api/github/agent-tasks?workspace_path=" + wsPathEncoded);
    if (resTasks.ok) {
      const data = await resTasks.json();
      if (data.success && data.tasks) bundle.tasks = data.tasks;
    }
    const resKanban = await fetch("http://127.0.0.1:8765/api/github/kanban?workspace_path=" + wsPathEncoded);
    if (resKanban.ok) {
      const data = await resKanban.json();
      if (data.success && data.board) bundle.board = data.board;
    }
    return bundle;
  }

  async function fetchRepoData() {
    try {
      const stageWs = getEffectiveWorkspacePath(false);
      const auxWs = getEffectiveWorkspacePath(true);
      const wsPath = encodeURIComponent(stageWs);
      const primary = await fetchWorkspaceBundle(wsPath);
      currentRepo = primary.repo || null;
      cachedIssues = primary.issues;
      cachedPRs = primary.prs;
      cachedAgentTasks = primary.tasks;
      cachedKanbanBoard = primary.board || null;

      if (auxWs === stageWs) {
        auxRepo = currentRepo;
        auxIssues = cachedIssues;
        auxPRs = cachedPRs;
        auxAgentTasks = cachedAgentTasks;
        auxKanbanBoard = cachedKanbanBoard;
        lastFetchedAuxWs = auxWs;
      } else {
        const auxContainer = document.querySelector("#swiss-aux-container");
        if (auxContainer) {
          const auxBundle = await fetchWorkspaceBundle(encodeURIComponent(auxWs));
          auxRepo = auxBundle.repo || null;
          auxIssues = auxBundle.issues;
          auxPRs = auxBundle.prs;
          auxAgentTasks = auxBundle.tasks;
          auxKanbanBoard = auxBundle.board || null;
          lastFetchedAuxWs = auxWs;
        }
      }

      if (activeMainStageExt === "github") {
        renderMainStageContent();
      }
      const auxContainer = document.querySelector("#swiss-aux-container");
      if (auxContainer && auxContainer.dataset.renderedTab === "github" && typeof window.renderSwissGitHubWorkspaceView === "function") {
        window.renderSwissGitHubWorkspaceView(auxContainer);
      }
      setupLeftNavTabs();
    } catch (_) {}
  }

  // 3. Inject Left Panel Navigation Tabs (Tab switcher in the same style as Scheduled Tasks, always below factory tabs)
  function setupLeftNavTabs() {
    try {
      const legacyBar = document.getElementById("swiss-left-extensions-bar");
      if (legacyBar) legacyBar.remove();

      const automationsBtn = document.querySelector('[data-testid="automations-button"]');
      const newConvBtn = document.querySelector('[data-testid="new-conversation-button"]');
      const navContainer = automationsBtn ? automationsBtn.parentElement : (newConvBtn ? newConvBtn.parentElement : null);

      function normalizeNewConversationButton() {
        try {
          const btn = document.querySelector('[data-testid="new-conversation-button"]');
          if (!btn) return;
          const isConv = Boolean(
            (window.location.pathname && window.location.pathname.startsWith('/c/')) ||
            document.querySelector('[data-testid="conversation-view"]') ||
            document.querySelector('[data-testid="conversation-row-sidebar"][data-selected="true"]')
          );
          const isHistory = Boolean(
            (window.location.pathname && window.location.pathname.startsWith('/history')) ||
            document.querySelector('[data-testid="history-button"].active') ||
            document.querySelector('[data-testid="history-button"][data-selected="true"]')
          );
          const isStage = Boolean(
            document.body.getAttribute('data-swiss-stage-active') ||
            document.querySelector('.swiss-left-nav-tab.active')
          );
          const isUnselected = isConv || isHistory || isStage || (window.location.pathname && window.location.pathname !== '/');
          if (isUnselected) {
            btn.setAttribute('data-selected', 'false');
            btn.setAttribute('aria-selected', 'false');
            btn.removeAttribute('data-state');
            btn.classList.remove('active');
            btn.classList.add('unselected');
          } else {
            btn.setAttribute('data-selected', 'true');
            btn.setAttribute('aria-selected', 'true');
            btn.classList.remove('unselected');
          }
        } catch (_) {}
      }
      normalizeNewConversationButton();
      window.normalizeNewConversationButton = normalizeNewConversationButton;

      if (!navContainer) return;

      function isLeftPanelEnabled() {
        try {
          const val = localStorage.getItem("antigravity_swiss_left_panel_enabled");
          if (val !== null) return val === "true";
          if (localStorage.getItem("antigravity_swiss_left_nav_enabled") === "false" ||
              localStorage.getItem("antigravity_swiss_sidebar_extensions_enabled") === "false") {
            return false;
          }
          if (window.__SWISS_ENH_CONFIG__ && window.__SWISS_ENH_CONFIG__.left_panel_extensions_enabled !== undefined) {
            return Boolean(window.__SWISS_ENH_CONFIG__.left_panel_extensions_enabled);
          }
        } catch (_) {}
        return true;
      }

      function getLeftPanelMode() {
        try {
          const val = localStorage.getItem("antigravity_swiss_left_panel_mode");
          if (val) return val;
          if (window.__SWISS_ENH_CONFIG__ && window.__SWISS_ENH_CONFIG__.left_panel_extensions_mode) {
            return window.__SWISS_ENH_CONFIG__.left_panel_extensions_mode;
          }
        } catch (_) {}
        return "single";
      }

      function isMainSectionEnabled() {
        try {
          const val = localStorage.getItem("antigravity_swiss_main_section_enabled");
          if (val !== null) return val === "true";
          if (window.__SWISS_ENH_CONFIG__ && window.__SWISS_ENH_CONFIG__.main_section_extensions_enabled !== undefined) {
            return Boolean(window.__SWISS_ENH_CONFIG__.main_section_extensions_enabled);
          }
        } catch (_) {}
        return true;
      }
      window.isSwissMainSectionEnabled = isMainSectionEnabled;

      function isExtensionEnabled(id) {
        try {
          if (!isLeftPanelEnabled()) return false;
          const disabledExts = JSON.parse(localStorage.getItem("antigravity_swiss_disabled_extensions") || "[]");
          if (Array.isArray(disabledExts) && disabledExts.includes(id)) return false;
          const val = localStorage.getItem("antigravity_swiss_ext_" + id + "_enabled");
          if (val === "false") return false;
          if (window.__SWISS_ENH_CONFIG__ && window.__SWISS_ENH_CONFIG__.extensions && window.__SWISS_ENH_CONFIG__.extensions[id] === false) {
            return false;
          }
        } catch (_) {}
        return true;
      }

      let group = document.getElementById("swiss-left-nav-group");
      if (!isLeftPanelEnabled() || window.__swissDaemonOnline === false || !isDaemonOnline) {
        if (group) group.remove();
        return;
      }

      const panelMode = getLeftPanelMode();
      let exts = [];
      if (panelMode === "individual") {
        exts = [
          {
            id: "browser",
            label: "Preview Browser",
            svg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
          },
          {
            id: "files",
            label: "File Explorer",
            svg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>'
          },
          {
            id: "memos",
            label: "Quick Memos",
            svg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M16 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V8Z"/><polyline points="15 3 15 8 20 8"/><line x1="9" y1="13" x2="15" y2="13"/><line x1="9" y1="17" x2="13" y2="17"/></svg>'
          },
          {
            id: "github",
            label: "GitHub Workspace",
            svg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/></svg>'
          }
        ].filter(ext => isExtensionEnabled(ext.id));
      } else {
        exts = [
          {
            id: "swiss-knife",
            label: "Swiss Knife",
            svg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M3 2v1c0 1 2 1 2 2S3 6 3 7s2 1 2 2-2 1-2 2 2 1 2 2"/><path d="M18 6h.01"/><path d="M6 18h.01"/><path d="M20.83 8.83a4 4 0 0 0-5.66-5.66l-12 12a4 4 0 1 0 5.66 5.66Z"/><path d="M18 11.66V22a4 4 0 0 0 4-4V6"/></svg>'
          }
        ];
      }

      if (exts.length === 0) {
        if (group) group.remove();
        return;
      }

      function buildGroupChildren(grp) {
        grp.innerHTML = "";
        const sep = document.createElement("div");
        sep.className = "swiss-left-tabs-separator";
        grp.appendChild(sep);

        const inMain = isMainSectionEnabled();
        exts.forEach(ext => {
          const btn = document.createElement("button");
          btn.className = "inline-flex items-center transition-colors select-none outline-none cursor-pointer disabled:opacity-50 flex-grow w-full justify-start font-normal h-8 min-w-0 gap-1.5 px-2 py-1 rounded-lg bg-transparent text-secondary-foreground hover:bg-sidebar-muted hover:text-foreground swiss-left-nav-tab";
          btn.dataset.swissExt = ext.id;
          btn.title = ext.id === "swiss-knife"
            ? (inMain ? "Open Swiss Knife in Main Section (Replace / Split Chat)" : "Open Swiss Knife in Auxiliary Panel")
            : (inMain ? "Open " + ext.label + " in Main Section (Replace / Split Chat)" : "Open " + ext.label + " in Auxiliary Panel");
          btn.innerHTML = '<span class="shrink-0 flex items-center">' + ext.svg + '</span>' +
                          '<span class="truncate text-sm">' + ext.label + '</span>';
          btn.onclick = (e) => {
            e.stopPropagation();
            const targetExt = ext.id === "swiss-knife" ? (activeMainStageExt || "github") : ext.id;
            if (!isMainSectionEnabled()) {
              if (typeof window.switchAuxTab === "function") {
                window.switchAuxTab("swiss-" + targetExt);
              } else {
                const auxBtn = document.querySelector('[data-tab-id="swiss-' + targetExt + '"]');
                if (auxBtn) auxBtn.click();
              }
              closeMainStage();
              return;
            }
            if (ext.id === "swiss-knife") {
              const stContainer = document.getElementById("swiss-main-stage-container");
              const cView = document.querySelector('[data-testid="conversation-view"]');
              const isDisp = Boolean(stContainer && stContainer.style.display === "flex" && (!cView || cView.style.display === "none"));
              if (isDisp) {
                closeMainStage();
              } else {
                openMainStage(activeMainStageExt || "github");
              }
            } else {
              const stContainer = document.getElementById("swiss-main-stage-container");
              const cView = document.querySelector('[data-testid="conversation-view"]');
              const isDisp = Boolean(stContainer && stContainer.style.display === "flex" && (!cView || cView.style.display === "none"));
              if (activeMainStageExt === ext.id && isDisp) {
                closeMainStage();
              } else {
                openMainStage(ext.id);
              }
            }
          };
          grp.appendChild(btn);
        });

        const sepBottom = document.createElement("div");
        sepBottom.className = "swiss-left-tabs-separator";
        grp.appendChild(sepBottom);
      }

      if (!group) {
        group = document.createElement("div");
        group.id = "swiss-left-nav-group";
        group.className = "swiss-left-nav-group flex flex-col gap-1.5";
        buildGroupChildren(group);
        navContainer.appendChild(group);
      } else {
        if (group.parentElement !== navContainer) {
          navContainer.appendChild(group);
        }
        const existingTabs = Array.from(group.querySelectorAll(".swiss-left-nav-tab"));
        const existingIds = existingTabs.map(t => t.dataset.swissExt).join(",");
        const targetIds = exts.map(e => e.id).join(",");
        if (existingIds !== targetIds) {
          buildGroupChildren(group);
        }
      }

      // Ensure bottom separator exists below button section above projects panel
      if (group && group.children.length > 0) {
        const lastChild = group.lastElementChild;
        if (lastChild && !lastChild.classList.contains("swiss-left-tabs-separator")) {
          const sepBottom = document.createElement("div");
          sepBottom.className = "swiss-left-tabs-separator";
          group.appendChild(sepBottom);
        }
      }

      // Synchronize group gap with navContainer native gap
      try {
        const navStyle = window.getComputedStyle(navContainer);
        const nativeGap = navStyle.rowGap || navStyle.gap || "6px";
        if (group && group.style.gap !== nativeGap) {
          group.style.gap = nativeGap;
        }
      } catch (_) {}

      // Sync active state on tabs
      const stageContainer = document.getElementById("swiss-main-stage-container");
      const convoView = document.querySelector('[data-testid="conversation-view"]');
      const isStageDisplayed = Boolean(
        stageContainer &&
        stageContainer.style.display === "flex" &&
        (!convoView || convoView.style.display === "none")
      );
      if (!isStageDisplayed) {
        activeMainStageExt = null;
        window.__swissActiveMainStageExt = null;
        if (stageContainer && stageContainer.style.display === "flex") {
          stageContainer.style.display = "none";
        }
        if (document.querySelector('[data-swiss-hidden-breadcrumb="true"]')) {
          restoreNativeBreadcrumbBar();
        }
        if (document.querySelector('[data-swiss-hidden-stage-child="true"]')) {
          restoreHiddenStageChildren(stageContainer);
        }
      } else if (activeMainStageExt) {
        hideNativeBreadcrumbBar();
      }

      group.querySelectorAll('.swiss-left-nav-tab').forEach(b => {
        const isSingle = b.dataset.swissExt === "swiss-knife";
        const isActive = Boolean(isStageDisplayed && (isSingle || (activeMainStageExt && b.dataset.swissExt === activeMainStageExt)));
        if (isActive) {
          b.className = "inline-flex items-center transition-colors select-none outline-none cursor-pointer disabled:opacity-50 flex-grow w-full justify-start font-normal h-8 min-w-0 gap-1.5 px-2 py-1 rounded-lg bg-sidebar-secondary text-foreground swiss-left-nav-tab active";
        } else {
          b.className = "inline-flex items-center transition-colors select-none outline-none cursor-pointer disabled:opacity-50 flex-grow w-full justify-start font-normal h-8 min-w-0 gap-1.5 px-2 py-1 rounded-lg bg-transparent text-secondary-foreground hover:bg-sidebar-muted hover:text-foreground swiss-left-nav-tab";
        }
      });

      const sidebar = getLeftSidebar() || navContainer.closest(".bg-sidebar");

      // Active state isolation: suppress native factory tabs/rows when Swiss stage is active, restore when closed
      if (isStageDisplayed) {
        if (document.body) {
          document.body.setAttribute("data-swiss-stage-active", activeMainStageExt || "true");
        }
        if (sidebar && sidebar.setAttribute) {
          sidebar.setAttribute("data-swiss-stage-active", activeMainStageExt || "true");
        }
        if (navContainer && navContainer.setAttribute) {
          navContainer.setAttribute("data-swiss-stage-active", activeMainStageExt || "true");
          const candidateBtns = navContainer.querySelectorAll ? Array.from(navContainer.querySelectorAll('button, [data-testid*="button"], [data-testid]')).filter(b => !b.classList.contains('swiss-left-nav-tab')) : [];
          candidateBtns.forEach(fb => {
            const isFbActive = fb.classList.contains("active") ||
              fb.getAttribute("data-state") === "active" ||
              fb.getAttribute("aria-selected") === "true" ||
              fb.getAttribute("data-selected") === "true" ||
              fb.classList.contains("bg-sidebar-secondary") ||
              fb.classList.contains("bg-sidebar-accent") ||
              fb.classList.contains("bg-background") ||
              fb.classList.contains("border-border");
            if (isFbActive) {
              if (!fb.__swissPrevActiveClasses) {
                fb.__swissPrevActiveClasses = fb.className;
              }
              if (!fb.__swissPrevDataState && fb.getAttribute("data-state")) {
                fb.__swissPrevDataState = fb.getAttribute("data-state");
              }
              if (!fb.__swissPrevAriaSelected && fb.getAttribute("aria-selected")) {
                fb.__swissPrevAriaSelected = fb.getAttribute("aria-selected");
              }
              fb.classList.remove("active", "bg-sidebar-secondary", "bg-sidebar-accent", "bg-background", "border", "border-border");
              fb.removeAttribute("data-state");
              fb.setAttribute("aria-selected", "false");
              fb.setAttribute("data-selected", "false");
              fb.setAttribute("data-swiss-suppressed", "true");
            }
          });
        }
        const searchSb = sidebar || document;
        if (searchSb && searchSb.querySelectorAll) {
          searchSb.querySelectorAll('[data-testid="conversation-row-sidebar"], [data-sidebar*="menu"], [data-active="true"], [data-selected="true"], [aria-selected="true"], [data-state="active"]').forEach(row => {
            const isConvRow = Boolean(row.matches?.('[data-testid="conversation-row-sidebar"]') || row.closest?.('[data-testid="conversation-row-sidebar"]') || row.getAttribute('data-testid') === 'conversation-row-sidebar');
            if (row.classList.contains('swiss-left-nav-tab') ||
                row.hasAttribute('data-project-card') ||
                row.closest?.('[data-project-card]') ||
                row.closest?.('.group\\/header') ||
                (row.querySelector && row.querySelector('[data-project-card]')) ||
                (row.classList && row.classList.contains('group/header')) ||
                (!isConvRow && row.hasAttribute('data-swiss-project'))) return;
            const isRowSelected = row.getAttribute("data-selected") === "true" ||
              row.getAttribute("aria-selected") === "true" ||
              row.getAttribute("data-state") === "active" ||
              row.getAttribute("data-active") === "true" ||
              row.classList.contains("bg-accent") ||
              row.classList.contains("bg-sidebar-accent") ||
              row.classList.contains("bg-primary") ||
              row.classList.contains("bg-sidebar-primary") ||
              row.classList.contains("border-primary") ||
              row.classList.contains("active");
            if (isRowSelected) {
              row.__swissPrevSelected = true;
              if (!row.__swissPrevRowClasses) {
                row.__swissPrevRowClasses = row.className;
              }
              if (!row.__swissPrevDataState && row.getAttribute("data-state")) {
                row.__swissPrevDataState = row.getAttribute("data-state");
              }
              if (!row.__swissPrevDataActive && row.getAttribute("data-active")) {
                row.__swissPrevDataActive = row.getAttribute("data-active");
              }
              row.setAttribute("data-selected", "false");
              row.setAttribute("aria-selected", "false");
              row.setAttribute("data-active", "false");
              row.removeAttribute("data-state");
              row.classList.remove("bg-accent", "bg-sidebar-accent", "bg-primary", "bg-sidebar-primary", "border-primary", "active");
              row.setAttribute("data-swiss-suppressed", "true");
            }
          });
        }
      } else {
        if (document.body) {
          document.body.removeAttribute("data-swiss-stage-active");
        }
        if (sidebar && sidebar.removeAttribute) {
          sidebar.removeAttribute("data-swiss-stage-active");
        }
        if (navContainer && navContainer.removeAttribute) {
          navContainer.removeAttribute("data-swiss-stage-active");
          const candidateBtns = navContainer.querySelectorAll ? Array.from(navContainer.querySelectorAll('[data-swiss-suppressed="true"]')) : [];
          candidateBtns.forEach(fb => {
            fb.removeAttribute("data-swiss-suppressed");
            if (fb.__swissPrevActiveClasses) {
              fb.className = fb.__swissPrevActiveClasses;
              delete fb.__swissPrevActiveClasses;
            }
            if (fb.__swissPrevDataState) {
              fb.setAttribute("data-state", fb.__swissPrevDataState);
              delete fb.__swissPrevDataState;
            }
            if (fb.__swissPrevAriaSelected) {
              fb.setAttribute("aria-selected", fb.__swissPrevAriaSelected);
              delete fb.__swissPrevAriaSelected;
            }
          });
        }
        const searchSb = sidebar || document;
        if (searchSb && searchSb.querySelectorAll) {
          searchSb.querySelectorAll('[data-swiss-suppressed="true"]').forEach(row => {
            row.removeAttribute("data-swiss-suppressed");
            if (row.__swissPrevSelected) {
              row.setAttribute("data-selected", "true");
              row.setAttribute("aria-selected", "true");
              delete row.__swissPrevSelected;
            }
            if (row.__swissPrevDataState) {
              row.setAttribute("data-state", row.__swissPrevDataState);
              delete row.__swissPrevDataState;
            }
            if (row.__swissPrevDataActive) {
              row.setAttribute("data-active", row.__swissPrevDataActive);
              delete row.__swissPrevDataActive;
            }
            if (row.__swissPrevRowClasses) {
              row.className = row.__swissPrevRowClasses;
              delete row.__swissPrevRowClasses;
            }
          });
        }
      }

      // Bind sidebar / project panel click listener to close stage when user clicks a chat or project item
      if (sidebar) {
        if (sidebar.__swissSidebarClickHandler) {
          sidebar.removeEventListener("click", sidebar.__swissSidebarClickHandler);
        }
        sidebar.__swissSidebarClickBound = true;
        sidebar.__swissSidebarClickHandler = (e) => {
          const convRow = e.target.closest('[data-testid="conversation-row-sidebar"]');
          if (convRow) {
            const cid = getConversationIdFromRow(convRow);
            const pName = extractProjectFromElement(convRow);
            recordProjectInteraction(pName, cid);
          } else {
            const pHeader = e.target.closest('.group\\/header, [data-project-card], [data-swiss-project], a[aria-label="New Conversation in Project"]');
            if (pHeader) {
              const pName = extractProjectFromElement(pHeader);
              recordProjectInteraction(pName, "");
            }
          }
          const target = e.target.closest('a, button, [data-testid="conversation-row-sidebar"]');
          if (target && !target.classList.contains('swiss-left-nav-tab')) {
            closeMainStage();
          }
        };
        sidebar.addEventListener("click", sidebar.__swissSidebarClickHandler);
      } else if (navContainer) {
        if (navContainer.__swissFactoryClickHandler) {
          navContainer.removeEventListener("click", navContainer.__swissFactoryClickHandler);
        }
        navContainer.__swissFactoryClickBound = true;
        navContainer.__swissFactoryClickHandler = (e) => {
          const target = e.target.closest('a, button');
          if (target && !target.classList.contains('swiss-left-nav-tab')) {
            closeMainStage();
          }
        };
        navContainer.addEventListener("click", navContainer.__swissFactoryClickHandler);
      }
    } catch (_) {}
  }

  // Global listener: clicking any conversation row, chat tab, or /c/ navigation returns to chat
  if (window.__swissChatNavHandler) {
    document.removeEventListener("click", window.__swissChatNavHandler, true);
  }
  window.__swissChatNavBound = true;
  window.__swissChatNavHandler = (e) => {
    const convRow = e.target.closest('[data-testid="conversation-row-sidebar"]');
    if (convRow) {
      const cid = getConversationIdFromRow(convRow);
      const pName = extractProjectFromElement(convRow);
      recordProjectInteraction(pName, cid);
    } else {
      const pHeader = e.target.closest('.group\\/header, [data-project-card], [data-swiss-project], a[aria-label="New Conversation in Project"]');
      if (pHeader) {
        const pName = extractProjectFromElement(pHeader);
        recordProjectInteraction(pName, "");
      }
    }
    const convTrigger = e.target.closest('[data-testid="conversation-row-sidebar"], [data-testid="new-conversation-button"], [data-testid="app-icon-new-conversation-button"], [data-testid="sidebar-add-project-button"], a[aria-label="New Conversation in Project"], [data-testid="history-button"], [data-testid="automations-button"], a[href*="/customizations"], a[href*="customizations"], [data-testid="settings-button"], a[href*="/c/"]');
    const isStageOpen = Boolean(window.__swissActiveMainStageExt || activeMainStageExt || (document.getElementById("swiss-main-stage-container") && document.getElementById("swiss-main-stage-container").style.display === "flex"));
    if (convTrigger && isStageOpen) {
      if (typeof window.closeMainStage === "function") {
        window.closeMainStage();
      } else {
        closeMainStage();
      }
    }
    if (typeof window.normalizeNewConversationButton === "function") {
      setTimeout(window.normalizeNewConversationButton, 10);
    }
  };
  document.addEventListener("click", window.__swissChatNavHandler, true);

  if (window.__swissPopstateHandler) {
    window.removeEventListener("popstate", window.__swissPopstateHandler);
  }
  window.__swissPopstateBound = true;
  window.__swissPopstateHandler = () => {
    const isStageOpen = Boolean(window.__swissActiveMainStageExt || activeMainStageExt || (document.getElementById("swiss-main-stage-container") && document.getElementById("swiss-main-stage-container").style.display === "flex"));
    if (isStageOpen) {
      if (typeof window.closeMainStage === "function") {
        window.closeMainStage();
      } else {
        closeMainStage();
      }
    }
    if (typeof window.normalizeNewConversationButton === "function") {
      setTimeout(window.normalizeNewConversationButton, 10);
    }
  };
  window.addEventListener("popstate", window.__swissPopstateHandler);

  if (!window.__swissOrigPushState && window.history && typeof window.history.pushState === "function") {
    window.__swissOrigPushState = window.history.pushState;
  }
  if (!window.__swissOrigReplaceState && window.history && typeof window.history.replaceState === "function") {
    window.__swissOrigReplaceState = window.history.replaceState;
  }
  if (window.history && window.__swissOrigPushState) {
    const origPushState = window.__swissOrigPushState;
    window.history.pushState = function(...args) {
      const res = origPushState.apply(this, args);
      try {
        const isStageOpen = Boolean(window.__swissActiveMainStageExt || activeMainStageExt || (document.getElementById("swiss-main-stage-container") && document.getElementById("swiss-main-stage-container").style.display === "flex"));
        if (isStageOpen) {
          const urlStr = args[2] ? String(args[2]) : "";
          if (!urlStr || urlStr === "/" || urlStr.startsWith("/?") || urlStr.startsWith("/c/") || urlStr.includes("customizations")) {
            if (typeof window.closeMainStage === "function") {
              window.closeMainStage();
            } else {
              closeMainStage();
            }
          }
        }
        if (typeof window.normalizeNewConversationButton === "function") {
          setTimeout(window.normalizeNewConversationButton, 10);
        }
      } catch (_) {}
      return res;
    };
  }
  if (window.history && window.__swissOrigReplaceState) {
    const origReplaceState = window.__swissOrigReplaceState;
    window.history.replaceState = function(...args) {
      const res = origReplaceState.apply(this, args);
      try {
        const isStageOpen = Boolean(window.__swissActiveMainStageExt || activeMainStageExt || (document.getElementById("swiss-main-stage-container") && document.getElementById("swiss-main-stage-container").style.display === "flex"));
        if (isStageOpen) {
          const urlStr = args[2] ? String(args[2]) : "";
          if (!urlStr || urlStr === "/" || (urlStr.startsWith("/c/") && !urlStr.includes("stage=true")) || urlStr.includes("customizations")) {
            if (typeof window.closeMainStage === "function") {
              window.closeMainStage();
            } else {
              closeMainStage();
            }
          }
        }
        if (typeof window.normalizeNewConversationButton === "function") {
          setTimeout(window.normalizeNewConversationButton, 10);
        }
      } catch (_) {}
      return res;
    };
  }

  if (window.__swissDocNavKeydownHandler) {
    document.removeEventListener("keydown", window.__swissDocNavKeydownHandler, true);
  }
  window.__swissDocNavKeydownHandler = (e) => {
    const isStageOpen = Boolean(window.__swissActiveMainStageExt || activeMainStageExt || (document.getElementById("swiss-main-stage-container") && document.getElementById("swiss-main-stage-container").style.display === "flex"));
    if (e.key === "Escape" && isStageOpen) {
      const hasOpenDropdown = Boolean(
        document.querySelector('.swiss-scope-dropdown-menu:not([style*="display: none"]), #swiss-gh-context-menu')
      );
      if (!hasOpenDropdown) {
        if (typeof window.closeMainStage === "function") {
          window.closeMainStage();
        } else {
          closeMainStage();
        }
      }
    }
    if ((e.ctrlKey || e.metaKey) && (e.key === "n" || e.key === "N")) {
      if (isStageOpen) {
        if (typeof window.closeMainStage === "function") {
          window.closeMainStage();
        } else {
          closeMainStage();
        }
      }
      if (typeof window.normalizeNewConversationButton === "function") {
        setTimeout(window.normalizeNewConversationButton, 10);
      }
    }
  };
  document.addEventListener("keydown", window.__swissDocNavKeydownHandler, true);

  function navigateToConversation(convId, rootParentId, isPruned) {
    const targetId = rootParentId || convId;
    if (isPruned || (targetId && window.__swissPrunedConversations && window.__swissPrunedConversations.has(targetId))) {
      showToast("Conversation trajectory was pruned by Antigravity (500-session limit reached)");
      return;
    }
    if (!targetId) return;

    closeMainStage();

    // 1. Try finding and clicking matching sidebar conversation element in DOM
    const targetSelector = 'a[href^="/c/' + targetId + '"], a[href*="/c/' + targetId + '"], [data-testid="conversation-row-sidebar"][data-cascade-id="' + targetId + '"], [data-testid="conversation-row-sidebar"][data-id="' + targetId + '"], [data-conversation-id="' + targetId + '"]';
    const sidebarLink = document.querySelector(targetSelector);
    if (sidebarLink) {
      sidebarLink.click();
      return;
    }

    const allLinks = document.querySelectorAll('a[href*="/c/"]');
    for (const a of allLinks) {
      if (a.getAttribute('href')?.includes(targetId)) {
        a.click();
        return;
      }
    }

    // 2. Fallback to pushState with ?section= query preservation
    try {
      const curUrl = new URL(window.location.href);
      const section = curUrl.searchParams.get("section");
      const searchStr = section ? "?section=" + encodeURIComponent(section) : "";
      window.history.pushState({}, "", "/c/" + targetId + searchStr);
      window.dispatchEvent(new PopStateEvent("popstate"));
    } catch (_) {}
  }

  window.navigateToConversation = navigateToConversation;
  window.jumpToConversation = navigateToConversation;

  // 4. Render GitHub Workspace in Right Panel Auxiliary Container (strictly project-specific)
  window.renderSwissGitHubWorkspaceView = function(container) {
    if (!container) return;
    container.innerHTML = "";

    const auxWs = getEffectiveWorkspacePath(true);
    if (lastFetchedAuxWs !== auxWs) {
      lastFetchedAuxWs = auxWs;
      fetchRepoData();
    }

    const auxView = document.createElement("div");
    auxView.className = "swiss-github-aux-view";

    const effRepo = auxRepo || currentRepo;
    const effIssues = auxIssues || cachedIssues;
    const effPRs = auxPRs || cachedPRs;
    const effTasks = auxAgentTasks || cachedAgentTasks;
    const repoName = effRepo ? (effRepo.full_name || effRepo.FullName || "Detecting repo...") : "Detecting repo...";
    const branchName = effRepo ? (effRepo.current_branch || effRepo.CurrentBranch || "main") : "main";
    const filteredItems = getFilteredItems(true);

    auxView.innerHTML = ` + "`" + `
      <div class="swiss-gh-repo-bar">
        <span class="swiss-gh-repo-name" title="${escapeHTML(repoName)}">${escapeHTML(repoName)}</span>
        <div style="display: flex; align-items: center; gap: 4px;">
          <span class="swiss-gh-branch-chip">
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="6" y1="3" x2="6" y2="15"></line>
              <circle cx="18" cy="6" r="3"></circle>
              <circle cx="6" cy="18" r="3"></circle>
              <path d="M18 9a9 9 0 0 1-9 9"></path>
            </svg>
            ${escapeHTML(branchName)}
          </span>
          <button class="swiss-gh-action-btn" id="swiss-gh-aux-refresh" title="Refresh">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="23 4 23 10 17 10"></polyline>
              <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path>
            </svg>
          </button>
          <button class="swiss-gh-action-btn" id="swiss-gh-aux-expand-main" title="Open in Main Area (Replace / Split Chat)">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="15 3 21 3 21 9"></polyline>
              <polyline points="9 21 3 21 3 15"></polyline>
              <line x1="21" y1="3" x2="14" y2="10"></line>
              <line x1="3" y1="21" x2="10" y2="14"></line>
            </svg>
          </button>
        </div>
      </div>
      <div class="swiss-gh-tabs-row">
        <button class="swiss-gh-tab-btn ${activeTab === "board" ? "active" : ""}" data-tab="board">Board</button>
        <button class="swiss-gh-tab-btn ${activeTab === "issues" ? "active" : ""}" data-tab="issues">Issues (${effIssues.length})</button>
        <button class="swiss-gh-tab-btn ${activeTab === "prs" ? "active" : ""}" data-tab="prs">PRs (${effPRs.length})</button>
        <button class="swiss-gh-tab-btn ${activeTab === "tasks" ? "active" : ""}" data-tab="tasks">Agent Tasks (${effTasks.length})</button>
      </div>
      <div class="swiss-gh-search-box">
        <input type="text" class="swiss-gh-search-input" placeholder="Search ${activeTab}..." value="${escapeHTML(activeSearch)}" id="swiss-gh-aux-search-field">
      </div>
      <div class="swiss-gh-list" id="swiss-gh-aux-items-list" style="flex: 1; overflow-y: auto;">
        ${activeTab === "board" ? renderAuxKanbanBoardHTML() : renderCardListHTML(filteredItems)}
      </div>
    ` + "`" + `;

    container.appendChild(auxView);

    auxView.querySelector("#swiss-gh-aux-refresh")?.addEventListener("click", fetchRepoData);
    auxView.querySelector("#swiss-gh-aux-expand-main")?.addEventListener("click", () => openMainStage("github"));

    auxView.querySelectorAll(".swiss-gh-tab-btn").forEach(btn => {
      btn.addEventListener("click", (e) => {
        activeTab = e.currentTarget.dataset.tab;
        window.renderSwissGitHubWorkspaceView(container);
      });
    });

    const searchField = auxView.querySelector("#swiss-gh-aux-search-field");
    searchField?.addEventListener("input", (e) => {
      activeSearch = e.target.value.toLowerCase();
      const listEl = auxView.querySelector("#swiss-gh-aux-items-list");
      if (listEl) {
        listEl.innerHTML = activeTab === "board" ? renderAuxKanbanBoardHTML() : renderCardListHTML(getFilteredItems(true));
        if (activeTab === "board") {
          bindKanbanDragEvents(auxView, true);
        }
        bindCardEventListeners(listEl, true);
      }
    });

    if (activeTab === "board") {
      bindKanbanDragEvents(auxView, true);
    }
    bindCardEventListeners(auxView, true);
  };

  function getFilteredItems(forAux = false) {
    let items = [];
    const issList = forAux ? (auxIssues || cachedIssues) : cachedIssues;
    const prList = forAux ? (auxPRs || cachedPRs) : cachedPRs;
    const taskList = forAux ? (auxAgentTasks || cachedAgentTasks) : cachedAgentTasks;
    const effTab = (!forAux && activeTab === "board") ? "issues" : activeTab;
    if (effTab === "issues") {
      items = issList;
    } else if (effTab === "prs") {
      items = prList;
    } else if (effTab === "tasks") {
      items = taskList;
    }

    if (activeSearch) {
      items = items.filter(it => {
        const title = (it.title || it.conversation_title || "").toLowerCase();
        const num = (it.number || it.bound_issue_number || "").toString();
        const agent = (it.agent_label || it.agent_name || "").toLowerCase();
        return title.includes(activeSearch) || num.includes(activeSearch) || agent.includes(activeSearch);
      });
    }
    return items;
  }

  function renderCardListHTML(items) {
    if (!items || items.length === 0) {
      return '<div style="padding: 16px; text-align: center; font-size: 11px; color: var(--text-muted, #94a3b8);">No items found</div>';
    }

    if (activeTab === "tasks") {
      return items.map(t => {
        const isWorking = t.not_fully_idle;
        const statusClass = isWorking ? "working" : "idle";
        const statusLabel = isWorking ? "Working" : "Idle";
        return ` + "`" + `
          <div class="swiss-gh-card" data-conv-id="${t.conversation_id}">
            <div class="swiss-gh-card-header">
              <span class="swiss-agent-task-badge ${statusClass}">
                <span class="swiss-agent-pulse-dot" style="${isWorking ? "" : "display:none;"}"></span>
                ${statusLabel}: ${t.agent_label || "Agent"}
              </span>
              <span style="font-size: 10px; color: var(--text-muted, #94a3b8);">${t.step_count} steps</span>
            </div>
            <div class="swiss-gh-card-title">${escapeHTML(t.conversation_title || "Active Agent Task")}</div>
            ${t.bound_issue_number ? '<div class="swiss-gh-chips-row"><span class="swiss-gh-label-chip">Issue #' + t.bound_issue_number + '</span></div>' : ""}
            <div class="swiss-gh-card-footer">
              <button class="swiss-gh-action-btn swiss-btn-focus-conv" data-conv-id="${t.conversation_id}">Focus Convo</button>
              <button class="swiss-gh-action-btn swiss-btn-label-agent" data-conv-id="${t.conversation_id}">Label Agent</button>
            </div>
          </div>
        ` + "`" + `;
      }).join("");
    }

    return items.map(it => {
      const isPR = activeTab === "prs";
      const num = it.number;
      const title = it.title;
      const state = it.state ? it.state.toLowerCase() : "open";
      const assigned = it.assigned_agent;
      const labels = it.labels || [];

      let badgeClass = state === "closed" ? "closed" : (isPR ? "pr" : "open");

      let agentHTML = "";
      if (assigned) {
        const isWorking = assigned.not_fully_idle;
        const isPruned = assigned.is_pruned;
        const shortId = assigned.conversation_id ? assigned.conversation_id.slice(0, 6) : "";
        const label = assigned.agent_label || assigned.agent_name || "Agent";
        const targetId = assigned.root_parent_conversation_id || assigned.conversation_id || "";
        const subInfo = assigned.parent_conversation_id ? " · Subagent" : "";
        const badgeTitle = isPruned
          ? ("Agent: " + escapeHTML(label) + " [Archived / Pruned]&#10;Conversation database file is missing")
          : ("Agent: " + escapeHTML(label) + (shortId ? " [#" + shortId + "]" : "") + subInfo + "&#10;Click to jump to conversation");
        agentHTML = ` + "`" + `
          <span class="swiss-agent-task-badge ${isPruned ? "pruned" : (isWorking ? "working" : "idle")}" data-conv-id="${targetId}" data-orig-id="${assigned.conversation_id || ""}" data-is-pruned="${isPruned ? "true" : "false"}" title="${badgeTitle}" style="cursor: ${isPruned ? "default" : "pointer"}; ${isPruned ? "opacity: 0.75; border: 1px dashed #94a3b8;" : ""}">
            <span class="swiss-agent-pulse-dot" style="${isWorking && !isPruned ? "" : "display:none;"}"></span>
            ${isPruned ? "Archived" : (isWorking ? "Working" : "Idle")}: ${escapeHTML(label)}${shortId ? " [#" + shortId + "]" : ""}
          </span>
        ` + "`" + `;
      }

      const labelsHTML = labels.slice(0, 3).map(l => '<span class="swiss-gh-label-chip">' + escapeHTML(l) + '</span>').join("");

      return ` + "`" + `
        <div class="swiss-gh-card" draggable="true" data-item-type="${isPR ? "pr" : "issue"}" data-item-num="${num}">
          <div class="swiss-gh-card-header">
            <div class="swiss-gh-badge-row">
              <svg width="8" height="12" viewBox="0 0 8 16" fill="currentColor" style="opacity: 0.6; cursor: grab; flex-shrink: 0;" title="Drag into chat"><circle cx="2" cy="2" r="1.2"/><circle cx="6" cy="2" r="1.2"/><circle cx="2" cy="8" r="1.2"/><circle cx="6" cy="8" r="1.2"/><circle cx="2" cy="14" r="1.2"/><circle cx="6" cy="14" r="1.2"/></svg>
              <span class="swiss-gh-num-badge ${badgeClass}">#${num} ${state.toUpperCase()}</span>
            </div>
            ${agentHTML}
          </div>
          <div class="swiss-gh-card-title">${escapeHTML(title)}</div>
          ${labelsHTML ? '<div class="swiss-gh-chips-row">' + labelsHTML + '</div>' : ""}
          <div class="swiss-gh-card-footer">
            <div class="swiss-gh-card-actions">
              <button class="swiss-gh-action-btn swiss-btn-send-chat" data-item-num="${num}" data-item-type="${isPR ? "pr" : "issue"}" title="Send to Agent Chat">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path>
                </svg>
                Chat
              </button>
              <button class="swiss-gh-action-btn swiss-btn-edit-modal" data-item-num="${num}" data-item-type="${isPR ? "pr" : "issue"}" title="View & Edit Details">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M12 20h9"></path>
                  <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"></path>
                </svg>
                Edit
              </button>
            </div>
            ${it.url ? '<a href="' + it.url + '" target="_blank" class="swiss-gh-action-btn" title="Open on GitHub"><svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg></a>' : ""}
          </div>
        </div>
      ` + "`" + `;
    }).join("");
  }

  function bindCardEventListeners(parentEl, forAux = false) {
    const isAux = Boolean(forAux || (parentEl && parentEl.closest && parentEl.closest("#swiss-aux-container")));
    const getPRs = () => isAux ? (auxPRs || cachedPRs) : cachedPRs;
    const getIssues = () => isAux ? (auxIssues || cachedIssues) : cachedIssues;
    const getTasks = () => isAux ? (auxAgentTasks || cachedAgentTasks) : cachedAgentTasks;

    // 1. Drag & drop handlers on cards
    parentEl.querySelectorAll('.swiss-gh-card[draggable="true"]').forEach(card => {
      card.addEventListener("dragstart", (e) => {
        const itemType = card.dataset.itemType;
        const itemNum = parseInt(card.dataset.itemNum, 10);
        const item = (itemType === "pr" ? getPRs() : getIssues()).find(i => i.number === itemNum);
        if (!item) return;

        card.classList.add("dragging");
        const payload = formatMarkdownPayload(item, itemType, isAux);
        e.dataTransfer.setData("text/plain", payload);
        e.dataTransfer.setData("application/json", JSON.stringify(item));
        e.dataTransfer.effectAllowed = "copy";
      });

      card.addEventListener("dragend", () => {
        card.classList.remove("dragging");
      });

      card.addEventListener("contextmenu", (e) => {
        e.preventDefault();
        e.stopPropagation();
        const itemType = card.dataset.itemType;
        const itemNum = parseInt(card.dataset.itemNum, 10);
        const cardId = itemType + "-" + itemNum;
        const currentCol = getItemCurrentColumn(itemNum, itemType, isAux);
        showGitHubContextMenu(e, itemNum, itemType, cardId, currentCol, isAux);
      });
    });

    // 2. Send to Chat button
    parentEl.querySelectorAll(".swiss-btn-send-chat").forEach(btn => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        const num = parseInt(btn.dataset.itemNum, 10);
        const type = btn.dataset.itemType;
        const item = (type === "pr" ? getPRs() : getIssues()).find(i => i.number === num);
        if (item) {
          sendToChatComposer(formatMarkdownPayload(item, type, isAux));
          showToast("Attached #" + num + " to prompt");
        }
      });
    });

    // 3. Edit in modal button
    parentEl.querySelectorAll(".swiss-btn-edit-modal").forEach(btn => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        const num = parseInt(btn.dataset.itemNum, 10);
        openIssueDetailModal(num, isAux);
      });
    });

    // 4. Focus conversation
    parentEl.querySelectorAll(".swiss-btn-focus-conv").forEach(btn => {
      btn.addEventListener("click", () => {
        const convId = btn.dataset.convId;
        const task = getTasks().find(t => t.conversation_id === convId);
        navigateToConversation(convId, task?.root_parent_conversation_id, task?.is_pruned);
      });
    });

    parentEl.querySelectorAll(".swiss-agent-task-badge[data-conv-id]").forEach(badge => {
      badge.addEventListener("click", (e) => {
        const convId = badge.dataset.convId;
        const isPruned = badge.dataset.isPruned === "true";
        if (convId) {
          e.stopPropagation();
          const origId = badge.dataset.origId || convId;
          const task = getTasks().find(t => t.conversation_id === origId || t.conversation_id === convId);
          navigateToConversation(convId, task?.root_parent_conversation_id, isPruned || task?.is_pruned);
        }
      });
    });

    // 5. Label agent
    parentEl.querySelectorAll(".swiss-btn-label-agent").forEach(btn => {
      btn.addEventListener("click", async () => {
        const convId = btn.dataset.convId;
        const currentTask = getTasks().find(t => t.conversation_id === convId);
        const currentLabel = currentTask ? currentTask.agent_label : "";
        const label = await showSwissPrompt("Enter custom label for this agent / conversation:", currentLabel);
        if (label !== null) {
          await fetch("http://127.0.0.1:8765/api/github/agent-tasks/label", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ conversation_id: convId, agent_label: label })
          });
          showToast("Updated agent label");
          fetchRepoData();
        }
      });
    });
  }

  // 5. Format Markdown for Chat & Send to Lexical
  function formatMarkdownPayload(item, type, forAux = false) {
    const isPR = type === "pr";
    const repoObj = forAux ? (auxRepo || currentRepo) : currentRepo;
    const repoFullName = repoObj ? (repoObj.full_name || repoObj.FullName || "Repo") : "Repo";
    let text = "### GitHub " + (isPR ? "Pull Request" : "Issue") + " #" + item.number + ": " + item.title + "\n";
    text += "- **Repository:** " + repoFullName + "\n";
    if (item.url) text += "- **URL:** " + item.url + "\n";
    text += "- **State:** " + (item.state || "open") + "\n";
    if (item.labels && item.labels.length > 0) {
      text += "- **Labels:** " + item.labels.join(", ") + "\n";
    }
    text += "\n" + (item.body || "(No description provided)") + "\n";
    return text;
  }

  function sendToChatComposer(text) {
    const lexicalElem = document.querySelector('[data-lexical-editor="true"]') ||
                        document.querySelector('.lexical-container [contenteditable="true"]') ||
                        document.querySelector('[contenteditable="true"]');
    if (lexicalElem) {
      if (typeof lexicalElem.focus === "function") lexicalElem.focus();
      if (lexicalElem.__lexicalEditor) {
        try {
          lexicalElem.__lexicalEditor.update(() => {
            document.execCommand("insertText", false, text);
          });
          return;
        } catch (_) {}
      }
      document.execCommand("insertText", false, text);
      return;
    }
    const textarea = document.querySelector("textarea");
    if (textarea) {
      textarea.value += "\n" + text;
      textarea.dispatchEvent(new Event("input", { bubbles: true }));
    }
  }

  // 6. Setup Drag & Drop into Chat Composer
  function setupDragAndDropToChat() {
    if (window.__swissDragOverHandler) document.removeEventListener("dragover", window.__swissDragOverHandler);
    if (window.__swissDragLeaveHandler) document.removeEventListener("dragleave", window.__swissDragLeaveHandler);
    if (window.__swissDropHandler) document.removeEventListener("drop", window.__swissDropHandler);

    window.__swissDragOverHandler = (e) => {
      const composer = document.querySelector('[contenteditable="true"]');
      if (composer && (composer === e.target || composer.contains(e.target))) {
        e.preventDefault();
        composer.classList.add("swiss-chat-drop-highlight");
      }
    };

    window.__swissDragLeaveHandler = (e) => {
      const composer = document.querySelector('[contenteditable="true"]');
      if (composer && !composer.contains(e.relatedTarget)) {
        composer.classList.remove("swiss-chat-drop-highlight");
      }
    };

    window.__swissDropHandler = (e) => {
      const composer = document.querySelector('[contenteditable="true"]');
      if (composer) {
        composer.classList.remove("swiss-chat-drop-highlight");
        if (composer === e.target || composer.contains(e.target)) {
          e.preventDefault();
          const text = e.dataTransfer.getData("text/plain");
          if (text) {
            sendToChatComposer(text);
            showToast("Added item to prompt");
          }
        }
      }
    };

    document.addEventListener("dragover", window.__swissDragOverHandler);
    document.addEventListener("dragleave", window.__swissDragLeaveHandler);
    document.addEventListener("drop", window.__swissDropHandler);
  }

  // 7. In-Place Detail Viewer & Editor Modal
  async function openIssueDetailModal(number, forAux = false) {
    let modal = document.getElementById("swiss-gh-modal");
    if (modal) modal.remove();

    modal = document.createElement("div");
    modal.id = "swiss-gh-modal";
    modal.className = "swiss-gh-modal-overlay";
    document.body.appendChild(modal);

    modal.innerHTML = ` + "`" + `
      <div class="swiss-gh-modal-dialog">
        <div style="padding: 12px 16px; border-bottom: 1px solid var(--border, #e2e8f0); display: flex; align-items: center; justify-content: space-between;">
          <span style="font-weight: 600; font-size: 14px;">Loading Issue #${number}...</span>
          <button class="swiss-main-stage-close-btn" id="swiss-gh-modal-close" title="Close">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>
      </div>
    ` + "`" + `;

    modal.querySelector("#swiss-gh-modal-close")?.addEventListener("click", () => modal.remove());
    modal.addEventListener("click", (e) => { if (e.target === modal) modal.remove(); });

    try {
      const targetWs = getEffectiveWorkspacePath(forAux);
      const wsParam = encodeURIComponent(targetWs);
      const res = await fetch("http://127.0.0.1:8765/api/github/issues/detail?workspace_path=" + wsParam + "&number=" + number);
      const data = await res.json();
      if (!data.success || !data.issue) {
        showToast("Failed to load issue details", true);
        modal.remove();
        return;
      }

      const iss = data.issue;
      const isOpen = iss.state === "open";

      modal.innerHTML = ` + "`" + `
        <div class="swiss-gh-modal-dialog">
          <div style="padding: 12px 16px; border-bottom: 1px solid var(--border, #e2e8f0); display: flex; align-items: center; justify-content: space-between;">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span class="swiss-gh-num-badge ${isOpen ? "open" : "closed"}">#${iss.number} ${iss.state.toUpperCase()}</span>
              <span style="font-size: 13px; font-weight: 600;">Issue Details</span>
            </div>
            <button class="swiss-main-stage-close-btn" id="swiss-gh-modal-close" title="Close">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>
          <div style="padding: 16px; overflow-y: auto; flex: 1; display: flex; flex-direction: column; gap: 12px;">
            <div>
              <label style="font-size: 11px; font-weight: 600; color: var(--text-muted, #64748b);">TITLE</label>
              <input type="text" id="swiss-modal-title" value="${escapeHTML(iss.title)}" class="swiss-gh-editor-title" style="width: 100%; border: 1px solid var(--border, #e2e8f0); font-size: 14px; margin-top: 4px; box-sizing: border-box;">
            </div>
            <div>
              <label style="font-size: 11px; font-weight: 600; color: var(--text-muted, #64748b);">DESCRIPTION</label>
              <textarea id="swiss-modal-body" class="swiss-gh-textarea" style="margin-top: 4px;">${escapeHTML(iss.body || "")}</textarea>
            </div>
            <div style="display: flex; align-items: center; justify-content: space-between; padding: 8px 0; border-top: 1px solid var(--border, #e2e8f0);">
              <button class="swiss-gh-action-btn" id="swiss-modal-toggle-state" style="padding: 6px 12px; font-weight: 600;">
                ${isOpen ? "Close Issue" : "Reopen Issue"}
              </button>
              <div style="display: flex; gap: 8px;">
                <button class="swiss-gh-save-btn" id="swiss-modal-save">Save Changes</button>
              </div>
            </div>
            <div style="margin-top: 10px; border-top: 1px solid var(--border, #e2e8f0); padding-top: 10px;">
              <label style="font-size: 11px; font-weight: 600; color: var(--text-muted, #64748b);">COMMENTS (${iss.comments ? iss.comments.length : 0})</label>
              <div style="display: flex; flex-direction: column; gap: 8px; margin-top: 8px; max-height: 180px; overflow-y: auto;">
                ${(iss.comments || []).map(c => ` + "`" + `
                  <div style="padding: 8px; border-radius: 6px; background: rgba(0,0,0,0.03); font-size: 12px;">
                    <div style="font-weight: 600; margin-bottom: 4px;">@${c.author}</div>
                    <div style="white-space: pre-wrap;">${escapeHTML(c.body)}</div>
                  </div>
                ` + "`" + `).join("")}
              </div>
              <div style="display: flex; gap: 6px; margin-top: 8px;">
                <input type="text" id="swiss-modal-comment-input" placeholder="Add a comment..." class="swiss-gh-search-input" style="flex: 1;">
                <button class="swiss-gh-action-btn" id="swiss-modal-add-comment" style="padding: 4px 10px; font-weight: 600; background: var(--border, #e2e8f0);">Comment</button>
              </div>
            </div>
          </div>
        </div>
      ` + "`" + `;

      modal.querySelector("#swiss-gh-modal-close")?.addEventListener("click", () => modal.remove());

      modal.querySelector("#swiss-modal-toggle-state")?.addEventListener("click", async () => {
        const nextState = isOpen ? "closed" : "open";
        await fetch("http://127.0.0.1:8765/api/github/issues/update", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ workspace_path: targetWs, number: number, state: nextState })
        });
        showToast("Issue " + nextState);
        modal.remove();
        fetchRepoData();
      });

      modal.querySelector("#swiss-modal-save")?.addEventListener("click", async () => {
        const newTitle = modal.querySelector("#swiss-modal-title").value;
        const newBody = modal.querySelector("#swiss-modal-body").value;
        await fetch("http://127.0.0.1:8765/api/github/issues/update", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ workspace_path: targetWs, number: number, title: newTitle, body: newBody })
        });
        showToast("Issue updated successfully");
        modal.remove();
        fetchRepoData();
      });

      modal.querySelector("#swiss-modal-add-comment")?.addEventListener("click", async () => {
        const commInput = modal.querySelector("#swiss-modal-comment-input");
        const commText = commInput.value.trim();
        if (!commText) return;
        await fetch("http://127.0.0.1:8765/api/github/issues/comment", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ workspace_path: targetWs, number: number, comment: commText })
        });
        showToast("Comment posted");
        openIssueDetailModal(number, forAux);
      });
    } catch (_) {
      modal.remove();
    }
  }

  // 8. Main Stage View Mode (Replaces / Splits Chat Viewport or Landing View)
  function hideNativeBreadcrumbBar() {
    const hiddenEls = new Set();
    const breadcrumbBar = document.querySelector('[data-testid="breadcrumb-segment"]')?.closest('.shrink-0');
    if (breadcrumbBar && breadcrumbBar.id !== "swiss-main-stage-container" && !breadcrumbBar.closest("#swiss-main-stage-container") && !breadcrumbBar.closest('[data-testid="auxiliary-panel"]') && !breadcrumbBar.closest('.part.auxiliarybar')) {
      hiddenEls.add(breadcrumbBar);
    }
    const convoView = document.querySelector('[data-testid="conversation-view"]');
    const centerCol = (convoView && convoView.closest('.flex-1.flex.flex-col.min-w-0.h-full')) ||
                      document.querySelector('.flex-1.flex.flex-col.min-w-0.h-full');
    if (centerCol) {
      Array.from(centerCol.children).forEach(ch => {
        if (ch.id !== "swiss-main-stage-container" && ch.classList?.contains("shrink-0")) {
          hiddenEls.add(ch);
        }
      });
    }
    hiddenEls.forEach(el => {
      if (el.__swissPrevDisplay === undefined) {
        el.__swissPrevDisplay = el.style.display || "";
      }
      el.style.display = "none";
      el.setAttribute("data-swiss-hidden-breadcrumb", "true");
    });
  }

  function restoreNativeBreadcrumbBar() {
    const hiddenEls = new Set();
    document.querySelectorAll('[data-swiss-hidden-breadcrumb="true"]').forEach(el => hiddenEls.add(el));
    const breadcrumbBar = document.querySelector('[data-testid="breadcrumb-segment"]')?.closest('.shrink-0');
    if (breadcrumbBar) hiddenEls.add(breadcrumbBar);
    const centerCol = document.querySelector('.flex-1.flex.flex-col.min-w-0.h-full');
    if (centerCol) {
      Array.from(centerCol.children).forEach(ch => {
        if (ch.classList?.contains("shrink-0")) hiddenEls.add(ch);
      });
    }
    hiddenEls.forEach(el => {
      if (el.__swissPrevDisplay !== undefined) {
        el.style.display = el.__swissPrevDisplay === "none" ? "" : el.__swissPrevDisplay;
        delete el.__swissPrevDisplay;
      } else if (el.getAttribute("data-swiss-hidden-breadcrumb") === "true" || el === breadcrumbBar) {
        el.style.display = "";
      }
      el.removeAttribute("data-swiss-hidden-breadcrumb");
    });
  }

  function restoreHiddenStageChildren(stageContainer) {
    const hiddenChildren = new Set();
    document.querySelectorAll('[data-swiss-hidden-stage-child="true"]').forEach(el => hiddenChildren.add(el));
    if (stageContainer && stageContainer.parentElement) {
      Array.from(stageContainer.parentElement.children).forEach(ch => {
        if (ch !== stageContainer && ch.id !== "swiss-left-nav-group" && ch.__swissPrevDisplay !== undefined) {
          hiddenChildren.add(ch);
        }
      });
    }
    hiddenChildren.forEach(child => {
      if (child.__swissPrevDisplay !== undefined) {
        child.style.display = child.__swissPrevDisplay === "none" ? "" : child.__swissPrevDisplay;
        delete child.__swissPrevDisplay;
      } else {
        child.style.display = "";
      }
      child.removeAttribute("data-swiss-hidden-stage-child");
    });
  }

  function getMainStageParent() {
    const convoView = document.querySelector('[data-testid="conversation-view"]');
    if (convoView && convoView.parentElement) {
      return { parent: convoView.parentElement, convoView: convoView };
    }
    // Fallback: When not inside an active conversation (e.g. landing on / or empty state)
    const centerCol = document.querySelector('.flex-1.flex.flex-col.min-w-0.h-full');
    if (centerCol) {
      return { parent: centerCol, convoView: null };
    }
    const sidebar = getLeftSidebar();
    if (sidebar && sidebar.parentElement) {
      const sibling = Array.from(sidebar.parentElement.children).find(el =>
        el !== sidebar &&
        !el.classList?.contains('bg-sidebar') &&
        el.getAttribute?.('data-testid') !== 'auxiliary-panel' &&
        !el.classList?.contains('auxiliarybar') &&
        el.id !== 'swiss-left-nav-group' &&
        (!el.clientWidth || el.clientWidth > 40)
      );
      if (sibling) {
        return { parent: sibling, convoView: null };
      }
    }
    if (sidebar && sidebar.nextElementSibling && (!sidebar.nextElementSibling.clientWidth || sidebar.nextElementSibling.clientWidth > 40)) {
      return { parent: sidebar.nextElementSibling, convoView: null };
    }
    const mainEl = document.querySelector('main') ||
                   Array.from(document.querySelectorAll('.flex-1.flex.flex-col')).find(el => !el.closest?.('.bg-sidebar') && !el.closest?.('[data-testid="auxiliary-panel"]')) ||
                   document.querySelector('.flex-1.flex.flex-col') ||
                   document.body;
    return { parent: mainEl, convoView: null };
  }

  function openMainStage(extType = "github") {
    detectCurrentConversationProject();
    const target = getMainStageParent();
    if (!target || !target.parent) return;

    activeMainStageExt = extType;
    window.__swissActiveMainStageExt = extType;
    let stageContainer = document.getElementById("swiss-main-stage-container");
    if (!stageContainer) {
      stageContainer = document.createElement("div");
      stageContainer.id = "swiss-main-stage-container";
      target.parent.appendChild(stageContainer);
    } else if (stageContainer.parentElement !== target.parent) {
      restoreHiddenStageChildren(stageContainer);
      target.parent.appendChild(stageContainer);
    }

    applyMainStageLayout();
    renderMainStageUI();
    setupLeftNavTabs();
    fetchAvailableProjects().then(() => {
      if (activeMainStageExt) updateScopePickerDOM();
    });
  }

  function closeMainStage() {
    activeMainStageExt = null;
    window.__swissActiveMainStageExt = null;
    const stageContainer = document.getElementById("swiss-main-stage-container");
    if (stageContainer) stageContainer.style.display = "none";

    restoreNativeBreadcrumbBar();
    restoreHiddenStageChildren(stageContainer);

    if (document.body) {
      document.body.removeAttribute("data-swiss-stage-active");
    }
    const leftSb = getLeftSidebar();
    if (leftSb && leftSb.removeAttribute) {
      leftSb.removeAttribute("data-swiss-stage-active");
    }

    const restoreChildren = (parentEl) => {
      if (!parentEl) return;
      Array.from(parentEl.children).forEach(child => {
        if (child.__swissPrevDisplay !== undefined) {
          child.style.display = child.__swissPrevDisplay === "none" ? "" : child.__swissPrevDisplay;
          delete child.__swissPrevDisplay;
        } else if (child !== stageContainer && child.id !== "swiss-left-nav-group") {
          child.style.display = "";
        }
        child.removeAttribute?.("data-swiss-hidden-stage-child");
      });
    };

    if (stageContainer && stageContainer.parentElement) {
      restoreChildren(stageContainer.parentElement);
    }

    const target = getMainStageParent();
    if (target && target.convoView) {
      target.convoView.style.display = "";
      target.convoView.style.flex = "";
      target.convoView.style.width = "";
      target.convoView.removeAttribute("data-swiss-hidden-stage-child");
      if (target.convoView.parentElement) {
        target.convoView.parentElement.style.display = "";
        target.convoView.parentElement.style.flexDirection = "";
      }
    } else if (target && target.parent) {
      restoreChildren(target.parent);
    }
    setupLeftNavTabs();
  }

  window.closeMainStage = closeMainStage;
  window.openSwissMainStage = openMainStage;

  function applyMainStageLayout() {
    const target = getMainStageParent();
    const stageContainer = document.getElementById("swiss-main-stage-container");
    if (!stageContainer || !target || !target.parent) return;

    hideNativeBreadcrumbBar();

    stageContainer.style.display = "flex";
    stageContainer.style.flex = "1 1 100%";
    stageContainer.style.width = "100%";
    stageContainer.style.height = "100%";
    stageContainer.style.borderLeft = "none";

    if (target.convoView) {
      document.querySelectorAll('[data-swiss-hidden-stage-child="true"]').forEach(el => {
        if (el !== target.convoView) {
          if (el.__swissPrevDisplay !== undefined) {
            el.style.display = el.__swissPrevDisplay === "none" ? "" : el.__swissPrevDisplay;
            delete el.__swissPrevDisplay;
          } else {
            el.style.display = "";
          }
          el.removeAttribute("data-swiss-hidden-stage-child");
        }
      });
      if (target.convoView.__swissPrevDisplay === undefined) {
        target.convoView.__swissPrevDisplay = target.convoView.style.display || "";
      }
      target.convoView.style.display = "none";
      target.convoView.setAttribute("data-swiss-hidden-stage-child", "true");
    } else {
      const leftSidebar = getLeftSidebar();
      Array.from(target.parent.children).forEach(child => {
        const isProtected =
          child === stageContainer ||
          child.id === "swiss-left-nav-group" ||
          child.id === "swiss-gh-toast" ||
          child.id === "swiss-gh-modal" ||
          child === leftSidebar ||
          Boolean(leftSidebar && child.contains?.(leftSidebar)) ||
          child.getAttribute?.("data-testid") === "auxiliary-panel" ||
          child.classList?.contains("auxiliarybar") ||
          Boolean(child.querySelector?.('[data-testid="conversation-list-sidebar"], #swiss-left-nav-group, #swiss-aux-container'));
        if (!isProtected) {
          if (child.__swissPrevDisplay === undefined) {
            child.__swissPrevDisplay = child.style.display || "";
          }
          child.style.display = "none";
          child.setAttribute("data-swiss-hidden-stage-child", "true");
        }
      });
    }

    target.parent.style.display = "flex";
    target.parent.style.flexDirection = "column";
  }

  function triggerNativeSplit(direction = "horizontal") {
    try {
      const root = document.getElementById("root");
      const key = root ? Object.keys(root).find(k => k.startsWith("__reactContainer") || k.startsWith("__reactFiber")) : null;
      let store = null;
      if (key) {
        let fiber = root[key];
        let depth = 0;
        function search(node) {
          if (!node || store || depth > 60) return;
          depth++;
          if (node.memoizedProps?.store) store = node.memoizedProps.store;
          if (!store && node.memoizedState?.element?.props?.store) store = node.memoizedState.element.props.store;
          if (!store && node.child) search(node.child);
          if (!store && node.sibling) search(node.sibling);
          depth--;
        }
        search(fiber);
      }
      if (store) {
        const state = store.getState()?.multiConvoLayout;
        const focused = state?.focusedPaneId || "pane-1";
        store.dispatch({
          type: "multiConvoLayout/splitPane",
          payload: { targetPaneId: focused, cascadeId: "_new", direction }
        });
        showToast("Native split (" + (direction === "horizontal" ? "Right" : "Down") + ")");
        return;
      }

      const stageContainer = document.getElementById("swiss-main-stage-container");
      const pane = stageContainer ? stageContainer.closest("[data-pane-id]") : null;
      const moreBtn = pane ? pane.querySelector('[data-testid="titlebar-more-actions"]') : document.querySelector('[data-testid="titlebar-more-actions"]');
      if (moreBtn) {
        moreBtn.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
        moreBtn.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));
        moreBtn.dispatchEvent(new MouseEvent("click", { bubbles: true }));
        setTimeout(() => {
          const splitMenu = Array.from(document.querySelectorAll('[role="menuitem"]')).find(m => m.innerText.trim().startsWith("Split"));
          if (splitMenu) {
            splitMenu.dispatchEvent(new PointerEvent("pointerenter", { bubbles: true }));
            splitMenu.dispatchEvent(new MouseEvent("mouseenter", { bubbles: true }));
            splitMenu.dispatchEvent(new MouseEvent("mouseover", { bubbles: true }));
            setTimeout(() => {
              const targetText = "Split " + (direction === "horizontal" ? "Right" : "Down");
              const subItem = Array.from(document.querySelectorAll('[role="menuitem"]')).find(m => m.innerText.trim() === targetText || m.innerText.trim().startsWith(targetText));
              if (subItem) {
                subItem.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
                subItem.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));
                subItem.dispatchEvent(new MouseEvent("click", { bubbles: true }));
                showToast("Native split (" + (direction === "horizontal" ? "Right" : "Down") + ")");
                return;
              }
              document.body.dispatchEvent(new MouseEvent("click", { bubbles: true }));
            }, 80);
          } else {
            document.body.dispatchEvent(new MouseEvent("click", { bubbles: true }));
          }
        }, 60);
        return;
      }

      const isMac = typeof navigator !== "undefined" && navigator.platform && navigator.platform.toUpperCase().indexOf("MAC") >= 0;
      document.dispatchEvent(new KeyboardEvent("keydown", {
        key: "d",
        code: "KeyD",
        keyCode: 68,
        which: 68,
        ctrlKey: !isMac,
        metaKey: isMac,
        bubbles: true,
        cancelable: true
      }));
    } catch (_) {}
  }

  if (window.__swissScopeOutsideClickHandler) {
    document.removeEventListener("click", window.__swissScopeOutsideClickHandler);
  }
  window.__swissScopeOutsideClickBound = true;
  window.__swissScopeOutsideClickHandler = (e) => {
    const menu = document.getElementById("swiss-stage-scope-menu");
    const btn = document.getElementById("swiss-stage-scope-btn");
    if (menu && menu.style.display !== "none") {
      if ((!btn || !btn.contains(e.target)) && !menu.contains(e.target)) {
        menu.style.display = "none";
      }
    }
  };
  document.addEventListener("click", window.__swissScopeOutsideClickHandler);

  function bindScopeMenuItems(scopeMenu) {
    if (!scopeMenu) return;
    scopeMenu.querySelectorAll(".swiss-scope-item").forEach(item => {
      item.addEventListener("click", (e) => {
        e.stopPropagation();
        const targetScope = item.dataset.scope || item.getAttribute("data-scope") || "GLOBAL";
        currentScope = targetScope;
        try { localStorage.setItem("antigravity_swiss_extension_scope", targetScope); } catch (_) {}
        if (targetScope !== "GLOBAL") {
          lastInteractedProject = targetScope;
          try { localStorage.setItem("antigravity_swiss_last_project", targetScope); } catch (_) {}
        }
        scopeMenu.style.display = "none";
        fetchRepoData();
        renderMainStageUI();
      });
    });
  }

  function updateScopePickerDOM() {
    const scopeBtn = document.getElementById("swiss-stage-scope-btn");
    const scopeMenu = document.getElementById("swiss-stage-scope-menu");
    if (!scopeBtn || !scopeMenu) return;

    const isGlobal = currentScope === "GLOBAL";
    const scopeDisplay = isGlobal ? "GLOBAL" : currentScope;

    scopeBtn.innerHTML = ` + "`" + `
      <span class="swiss-scope-pill">
        <span class="swiss-scope-label">Scope:</span>
        <span class="swiss-scope-value">${escapeHTML(scopeDisplay)}</span>
      </span>
      <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-left: 2px;"><polyline points="6 9 12 15 18 9"></polyline></svg>
    ` + "`" + `;

    scopeMenu.innerHTML = ` + "`" + `
      <div class="swiss-scope-item ${isGlobal ? "selected" : ""}" data-scope="GLOBAL">
        <span style="font-weight: 500;">GLOBAL</span>
        <span style="font-size: 11px; color: var(--muted-foreground); margin-left: auto;">(All Projects)</span>
      </div>
      ${availableProjects.map(p => ` + "`" + `
        <div class="swiss-scope-item ${currentScope === p ? "selected" : ""}" data-scope="${escapeHTML(p)}">
          <span style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${escapeHTML(p)}</span>
        </div>
      ` + "`" + `).join("")}
    ` + "`" + `;

    bindScopeMenuItems(scopeMenu);
  }

  function renderMainStageUI() {
    const container = document.getElementById("swiss-main-stage-container");
    if (!container) return;

    const isGlobal = currentScope === "GLOBAL";
    const scopeDisplay = isGlobal ? "GLOBAL" : currentScope;

    const effListTab = (activeTab === "board" || !activeTab) ? "issues" : activeTab;

    container.innerHTML = ` + "`" + `
      <div id="swiss-main-stage-header">
        <div class="swiss-main-stage-header-left">
          <!-- 1. Scope Dropdown Selector in front of tab switcher -->
          <div class="swiss-scope-picker" id="swiss-stage-scope-picker">
            <button class="swiss-scope-btn" id="swiss-stage-scope-btn" type="button" title="Select Extension Scope (GLOBAL or specific project)">
              <span class="swiss-scope-pill">
                <span class="swiss-scope-label">Scope:</span>
                <span class="swiss-scope-value">${escapeHTML(scopeDisplay)}</span>
              </span>
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-left: 2px;"><polyline points="6 9 12 15 18 9"></polyline></svg>
            </button>
            <div class="swiss-scope-dropdown-menu" id="swiss-stage-scope-menu" style="display: none;">
              <div class="swiss-scope-item ${isGlobal ? "selected" : ""}" data-scope="GLOBAL">
                <span style="font-weight: 500;">GLOBAL</span>
                <span style="font-size: 11px; color: var(--muted-foreground); margin-left: auto;">(All Projects)</span>
              </div>
              ${availableProjects.map(p => ` + "`" + `
                <div class="swiss-scope-item ${currentScope === p ? "selected" : ""}" data-scope="${escapeHTML(p)}">
                  <span style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${escapeHTML(p)}</span>
                </div>
              ` + "`" + `).join("")}
            </div>
          </div>

          <!-- 2. Extension Tab Switchers in top bar (Order: Browser, File Explorer, Quick Memos, GitHub Workspace) -->
          <div class="swiss-main-stage-tabs">
            <button class="swiss-main-stage-tab ${activeMainStageExt === "browser" ? "active" : ""}" data-ext="browser" title="Preview Browser">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="2" y1="12" x2="22" y2="12"></line>
                <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path>
              </svg>
              Preview Browser
            </button>
            <button class="swiss-main-stage-tab ${activeMainStageExt === "files" ? "active" : ""}" data-ext="files" title="File Explorer">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
                <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"></path>
              </svg>
              File Explorer
            </button>
            <button class="swiss-main-stage-tab ${activeMainStageExt === "memos" ? "active" : ""}" data-ext="memos" title="Quick Memos">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
                <path d="M16 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V8Z"></path>
                <polyline points="15 3 15 8 20 8"></polyline>
                <line x1="9" y1="13" x2="15" y2="13"></line>
                <line x1="9" y1="17" x2="13" y2="17"></line>
              </svg>
              Quick Memos
            </button>
            <button class="swiss-main-stage-tab ${activeMainStageExt === "github" ? "active" : ""}" data-ext="github" title="GitHub Workspace">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
                <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"></path>
              </svg>
              GitHub Workspace
            </button>
          </div>
        </div>

        <div style="display: flex; align-items: center; gap: 6px;">
          ${activeMainStageExt === "github" ? ` + "`" + `
            <div class="swiss-gh-view-switcher" id="swiss-stage-gh-tabs">
              <button class="swiss-gh-view-btn ${stageViewMode === "kanban" ? "active" : ""}" id="swiss-stage-view-kanban" data-stage-tab="board">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <rect x="3" y="3" width="7" height="18" rx="1"></rect>
                  <rect x="14" y="3" width="7" height="18" rx="1"></rect>
                </svg>
                Board
              </button>
              <button class="swiss-gh-view-btn ${stageViewMode === "list" && effListTab === "issues" ? "active" : ""}" id="swiss-stage-view-list" data-stage-tab="issues">
                Issues (${cachedIssues.length})
              </button>
              <button class="swiss-gh-view-btn ${stageViewMode === "list" && effListTab === "prs" ? "active" : ""}" data-stage-tab="prs">
                PRs (${cachedPRs.length})
              </button>
              <button class="swiss-gh-view-btn ${stageViewMode === "list" && effListTab === "tasks" ? "active" : ""}" data-stage-tab="tasks">
                Agent Tasks (${cachedAgentTasks.length})
              </button>
            </div>
            <button class="swiss-gh-action-btn" id="swiss-stage-gh-refresh" title="Refresh">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="23 4 23 10 17 10"></polyline>
                <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path>
              </svg>
            </button>
          ` + "`" + ` : ""}
          <button class="swiss-stage-native-split-btn" id="swiss-stage-native-split-btn" title="Split Right">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <rect width="18" height="18" x="3" y="3" rx="2" />
              <path d="M12 3v18" />
            </svg>
          </button>
          <button class="swiss-main-stage-close-btn" id="swiss-stage-close-btn" title="Back to Chat / Close Stage">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>
      </div>
      <div id="swiss-main-stage-body"></div>
    ` + "`" + `;

    // Event listeners
    container.querySelectorAll(".swiss-main-stage-tab").forEach(tab => {
      tab.addEventListener("click", (e) => {
        activeMainStageExt = e.currentTarget.dataset.ext;
        window.__swissActiveMainStageExt = activeMainStageExt;
        renderMainStageUI();
        setupLeftNavTabs();
      });
    });

    const scopeBtn = container.querySelector("#swiss-stage-scope-btn");
    const scopeMenu = container.querySelector("#swiss-stage-scope-menu");
    if (scopeBtn && scopeMenu) {
      scopeBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        scopeMenu.style.display = scopeMenu.style.display === "none" ? "flex" : "none";
      });
      bindScopeMenuItems(scopeMenu);
    }

    container.querySelector("#swiss-stage-native-split-btn")?.addEventListener("click", () => {
      triggerNativeSplit("horizontal");
    });

    container.querySelector("#swiss-stage-close-btn")?.addEventListener("click", () => {
      closeMainStage();
    });

    container.querySelector("#swiss-stage-gh-refresh")?.addEventListener("click", fetchRepoData);
    container.querySelectorAll("#swiss-stage-gh-tabs [data-stage-tab]").forEach(btn => {
      btn.addEventListener("click", () => {
        const tab = btn.dataset.stageTab;
        if (tab === "board") {
          stageViewMode = "kanban";
        } else {
          activeTab = tab;
          stageViewMode = "list";
        }
        renderMainStageUI();
      });
    });

    renderMainStageContent();
  }

  function renderMainStageContent() {
    const body = document.getElementById("swiss-main-stage-body");
    if (!body) return;
    body.innerHTML = "";

    if (activeMainStageExt === "github") {
      renderGitHubWorkspaceStage(body);
    } else if (activeMainStageExt === "browser") {
      if (typeof window.renderSwissBrowserView === "function") {
        window.renderSwissBrowserView(body);
      } else {
        body.innerHTML = '<iframe src="http://127.0.0.1:8765" style="width: 100%; height: 100%; border: none;"></iframe>';
      }
    } else if (activeMainStageExt === "files") {
      if (typeof window.renderSwissFilesView === "function") {
        window.renderSwissFilesView(body);
      } else {
        body.innerHTML = '<iframe src="http://127.0.0.1:8765" style="width: 100%; height: 100%; border: none;"></iframe>';
      }
    } else if (activeMainStageExt === "memos") {
      if (typeof window.renderSwissMemosView === "function") {
        window.renderSwissMemosView(body);
      } else {
        body.innerHTML = '<iframe src="http://127.0.0.1:8765" style="width: 100%; height: 100%; border: none;"></iframe>';
      }
    }
  }

  // 9. Kanban Data & Operations
  function getEffectiveKanbanBoard(forAux = false) {
    const boardSource = forAux ? (auxKanbanBoard || cachedKanbanBoard) : cachedKanbanBoard;
    const issSource = forAux ? (auxIssues || cachedIssues) : cachedIssues;
    const prSource = forAux ? (auxPRs || cachedPRs) : cachedPRs;

    if (boardSource && boardSource.columns && boardSource.columns.length > 0) {
      return boardSource;
    }
    const colMap = { todo: [], in_progress: [], review: [], done: [] };
    const isWIP = l => ["in progress", "in-progress", "wip", "doing", "active", "working"].includes((l || "").toLowerCase());
    const isRev = l => ["review", "in review", "in-review", "needs review", "under review", "qa"].includes((l || "").toLowerCase());

    (issSource || []).forEach(iss => {
      let col = "todo";
      const st = (iss.state || "open").toLowerCase();
      if (st === "closed") col = "done";
      else if (iss.assigned_agent) col = "in_progress";
      else if ((iss.labels || []).some(isWIP)) col = "in_progress";
      else if ((iss.labels || []).some(isRev)) col = "review";
      colMap[col].push({
        id: "issue-" + iss.number,
        type: "issue",
        number: iss.number,
        title: iss.title,
        body: iss.body,
        state: st,
        column_id: col,
        labels: iss.labels || [],
        assigned_agent: iss.assigned_agent
      });
    });

    (prSource || []).forEach(pr => {
      let col = "review";
      const st = (pr.state || "open").toLowerCase();
      if (st === "closed" || st === "merged") col = "done";
      else if (pr.is_draft || (pr.labels || []).some(isWIP)) col = "in_progress";
      colMap[col].push({
        id: "pr-" + pr.number,
        type: "pr",
        number: pr.number,
        title: pr.title,
        body: pr.body,
        state: st,
        column_id: col,
        labels: pr.labels || [],
        assigned_agent: pr.assigned_agent
      });
    });

    return {
      is_synthesized: true,
      columns: [
        { id: "todo", title: "Todo", cards: colMap.todo },
        { id: "in_progress", title: "In Progress", cards: colMap.in_progress },
        { id: "review", title: "Review", cards: colMap.review },
        { id: "done", title: "Done", cards: colMap.done }
      ]
    };
  }

  function getItemCurrentColumn(number, type, forAux = false) {
    const board = getEffectiveKanbanBoard(forAux);
    if (board && board.columns) {
      for (const col of board.columns) {
        if ((col.cards || []).some(c => c.number === number && (c.type === type || (c.id && c.id.startsWith(type))))) {
          return col.id;
        }
      }
    }
    const prSource = forAux ? (auxPRs || cachedPRs) : cachedPRs;
    const issSource = forAux ? (auxIssues || cachedIssues) : cachedIssues;
    if (type === "pr") {
      const pr = (prSource || []).find(p => p.number === number);
      if (pr) {
        const st = (pr.state || "open").toLowerCase();
        if (st === "closed" || st === "merged") return "done";
        if (pr.is_draft) return "in_progress";
        return "review";
      }
      return "review";
    }
    const iss = (issSource || []).find(i => i.number === number);
    if (iss) {
      const st = (iss.state || "open").toLowerCase();
      if (st === "closed") return "done";
      if (iss.assigned_agent) return "in_progress";
      const isWIP = l => ["in progress", "in-progress", "wip", "doing"].includes((l || "").toLowerCase());
      if ((iss.labels || []).some(isWIP)) return "in_progress";
      const isRev = l => ["review", "in review", "in-review"].includes((l || "").toLowerCase());
      if ((iss.labels || []).some(isRev)) return "review";
      return "todo";
    }
    return "todo";
  }

  async function moveKanbanCard(cardId, cardType, number, sourceCol, targetCol, forAux = false) {
    if (sourceCol === targetCol) return;
    try {
      const res = await fetch("http://127.0.0.1:8765/api/github/kanban/move", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          workspace_path: getEffectiveWorkspacePath(forAux),
          card_id: cardId,
          card_type: cardType || (cardId.startsWith("pr-") ? "pr" : "issue"),
          number: number,
          source_column: sourceCol,
          target_column: targetCol
        })
      });
      const data = await res.json();
      if (data.success) {
        showToast("Moved #" + number + " to " + targetCol.replace("_", " ").toUpperCase());
      } else {
        showToast(data.error || "Failed to move card", true);
      }
    } catch (err) {
      showToast("Move failed: " + err.message, true);
    } finally {
      fetchRepoData();
    }
  }

  // Context Menu Helpers & Implementation
  let activeGHContextMenu = null;

  function removeGHContextMenu() {
    if (activeGHContextMenu) {
      activeGHContextMenu.remove();
      activeGHContextMenu = null;
    }
    document.removeEventListener("click", onGHDocClick);
    document.removeEventListener("contextmenu", onGHDocContextMenu);
    document.removeEventListener("keydown", onGHDocKeydown);
    window.removeEventListener("resize", removeGHContextMenu);
  }

  function onGHDocClick(e) {
    if (activeGHContextMenu && !activeGHContextMenu.contains(e.target)) {
      removeGHContextMenu();
    }
  }

  function onGHDocContextMenu(e) {
    if (activeGHContextMenu && !activeGHContextMenu.contains(e.target)) {
      removeGHContextMenu();
    }
  }

  function onGHDocKeydown(e) {
    if (e.key === "Escape") {
      removeGHContextMenu();
    }
  }

  function showGitHubContextMenu(e, itemNum, itemType, cardId, currentColId, forAux = false) {
    removeGHContextMenu();

    const isPR = itemType === "pr" || (cardId && cardId.startsWith("pr-"));
    const normalizedType = isPR ? "pr" : "issue";
    const prSource = forAux ? (auxPRs || cachedPRs) : cachedPRs;
    const issSource = forAux ? (auxIssues || cachedIssues) : cachedIssues;
    const item = (normalizedType === "pr" ? (prSource || []) : (issSource || [])).find(i => i.number === itemNum);
    const currentCol = currentColId || getItemCurrentColumn(itemNum, normalizedType, forAux);
    const actualCardId = cardId || (normalizedType + "-" + itemNum);

    const menu = document.createElement("div");
    menu.className = "swiss-gh-context-menu";

    const colCategories = [
      { id: "todo", label: "Todo" },
      { id: "in_progress", label: "In Progress" },
      { id: "review", label: "Review" },
      { id: "done", label: "Done" }
    ];

    let categoryItemsHTML = colCategories.map(c => {
      const isCurrent = c.id === currentCol;
      return '<button class="swiss-gh-menu-item ' + (isCurrent ? 'active' : '') + '" data-target-col="' + c.id + '">' +
        '<span>' + c.label + '</span>' +
        (isCurrent ? '<span class="swiss-gh-menu-check"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg></span>' : '') +
      '</button>';
    }).join("");

    let chatLabel = "Chat with Agent";
    if (item && item.assigned_agent && item.assigned_agent.conversation_id) {
      const shortId = item.assigned_agent.conversation_id.slice(0, 6);
      const label = item.assigned_agent.agent_label || item.assigned_agent.agent_name || "Agent";
      const prunedSuffix = item.assigned_agent.is_pruned ? " (Archived)" : "";
      chatLabel = "Jump to " + label + " [#" + shortId + "]" + prunedSuffix;
    }

    let menuHTML = '<div class="swiss-gh-menu-header">Move to Board</div>' +
      categoryItemsHTML +
      '<div class="swiss-gh-menu-divider"></div>' +
      '<button class="swiss-gh-menu-item" id="swiss-gh-ctx-chat"><span>' + escapeHTML(chatLabel) + '</span></button>' +
      '<button class="swiss-gh-menu-item" id="swiss-gh-ctx-edit"><span>View & Edit Details</span></button>';

    if (item && item.url) {
      menuHTML += '<button class="swiss-gh-menu-item" id="swiss-gh-ctx-open"><span>Open on GitHub</span><svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg></button>';
    }

    menu.innerHTML = menuHTML;

    menu.querySelectorAll(".swiss-gh-menu-item[data-target-col]").forEach(btn => {
      btn.addEventListener("click", async (ev) => {
        ev.stopPropagation();
        const targetCol = btn.dataset.targetCol;
        removeGHContextMenu();
        await moveKanbanCard(actualCardId, normalizedType, itemNum, currentCol, targetCol, forAux);
      });
    });

    menu.querySelector("#swiss-gh-ctx-chat")?.addEventListener("click", (ev) => {
      ev.stopPropagation();
      removeGHContextMenu();
      if (item && item.assigned_agent && item.assigned_agent.conversation_id) {
        navigateToConversation(
          item.assigned_agent.conversation_id,
          item.assigned_agent.root_parent_conversation_id,
          item.assigned_agent.is_pruned
        );
      } else if (item) {
        sendToChatComposer(formatMarkdownPayload(item, normalizedType, forAux));
        showToast("Attached #" + itemNum + " to prompt");
      }
    });

    menu.querySelector("#swiss-gh-ctx-edit")?.addEventListener("click", (ev) => {
      ev.stopPropagation();
      removeGHContextMenu();
      openIssueDetailModal(itemNum, forAux);
    });

    menu.querySelector("#swiss-gh-ctx-open")?.addEventListener("click", (ev) => {
      ev.stopPropagation();
      removeGHContextMenu();
      if (item && item.url) {
        window.open(item.url, "_blank");
      }
    });

    menu.style.visibility = "hidden";
    menu.style.left = "0px";
    menu.style.top = "0px";
    document.body.appendChild(menu);

    const rect = menu.getBoundingClientRect();
    const pad = 8;
    let finalX = e.clientX;
    let finalY = e.clientY;
    if (finalX + rect.width > window.innerWidth - pad) {
      finalX = Math.max(pad, window.innerWidth - rect.width - pad);
    }
    if (finalY + rect.height > window.innerHeight - pad) {
      finalY = Math.max(pad, window.innerHeight - rect.height - pad);
    }
    menu.style.left = finalX + "px";
    menu.style.top = finalY + "px";
    menu.style.visibility = "visible";

    activeGHContextMenu = menu;
    setTimeout(() => {
      document.addEventListener("click", onGHDocClick);
      document.addEventListener("contextmenu", onGHDocContextMenu);
      document.addEventListener("keydown", onGHDocKeydown);
      window.addEventListener("resize", removeGHContextMenu);
    }, 10);
  }

  function renderKanbanCardHTML(card, colId) {
    const isPR = card.type === "pr" || (card.id && card.id.startsWith("pr-"));
    const num = card.number;
    const title = card.title || "";
    const state = (card.state || "open").toLowerCase();
    const assigned = card.assigned_agent;
    const labels = card.labels || [];

    let badgeClass = state === "closed" ? "closed" : (isPR ? "pr" : "open");
    let agentHTML = "";
    if (assigned) {
      const isWorking = assigned.not_fully_idle;
      const isPruned = assigned.is_pruned;
      const shortId = assigned.conversation_id ? assigned.conversation_id.slice(0, 6) : "";
      const label = assigned.agent_label || assigned.agent_name || "Agent";
      const targetId = assigned.root_parent_conversation_id || assigned.conversation_id || "";
      const subInfo = assigned.parent_conversation_id ? " · Subagent" : "";
      const badgeTitle = isPruned
        ? ("Agent: " + escapeHTML(label) + " [Archived / Pruned]&#10;Conversation database file is missing")
        : ("Agent: " + escapeHTML(label) + (shortId ? " [#" + shortId + "]" : "") + subInfo + "&#10;Click to jump to conversation");
      agentHTML = ` + "`" + `
        <span class="swiss-agent-task-badge ${isPruned ? "pruned" : (isWorking ? "working" : "idle")}" data-conv-id="${targetId}" data-orig-id="${assigned.conversation_id || ""}" data-is-pruned="${isPruned ? "true" : "false"}" title="${badgeTitle}" style="cursor: ${isPruned ? "default" : "pointer"}; ${isPruned ? "opacity: 0.75; border: 1px dashed #94a3b8;" : ""}">
          <span class="swiss-agent-pulse-dot" style="${isWorking && !isPruned ? "" : "display:none;"}"></span>
          ${isPruned ? "Archived" : (isWorking ? "Working" : "Idle")}: ${escapeHTML(label)}${shortId ? " [#" + shortId + "]" : ""}
        </span>
      ` + "`" + `;
    }

    const labelsHTML = labels.slice(0, 3).map(l => '<span class="swiss-gh-label-chip">' + escapeHTML(l) + '</span>').join("");

    return ` + "`" + `
      <div class="swiss-gh-kanban-card" draggable="true" data-card-id="${card.id}" data-item-type="${isPR ? "pr" : "issue"}" data-item-num="${num}" data-col-id="${colId}">
        <div class="swiss-gh-card-header">
          <div class="swiss-gh-badge-row">
            <svg width="8" height="12" viewBox="0 0 8 16" fill="currentColor" style="opacity: 0.6; cursor: grab; flex-shrink: 0;" title="Drag card across columns or into chat"><circle cx="2" cy="2" r="1.2"/><circle cx="6" cy="2" r="1.2"/><circle cx="2" cy="8" r="1.2"/><circle cx="6" cy="8" r="1.2"/><circle cx="2" cy="14" r="1.2"/><circle cx="6" cy="14" r="1.2"/></svg>
            <span class="swiss-gh-num-badge ${badgeClass}">${isPR ? "PR #" : "#"}${num} ${state.toUpperCase()}</span>
          </div>
          ${agentHTML}
        </div>
        <div class="swiss-gh-card-title">${escapeHTML(title)}</div>
        ${labelsHTML ? '<div class="swiss-gh-chips-row">' + labelsHTML + '</div>' : ""}
        <div class="swiss-gh-card-footer" style="display: flex; justify-content: flex-end; gap: 4px; margin-top: 4px;">
          <button class="swiss-gh-action-btn swiss-btn-send-chat" data-item-num="${num}" data-item-type="${isPR ? "pr" : "issue"}" title="Send to Agent Chat">
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path>
            </svg>
            Chat
          </button>
          <button class="swiss-gh-action-btn swiss-btn-edit-modal" data-item-num="${num}" data-item-type="${isPR ? "pr" : "issue"}" title="View & Edit Details">
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
            </svg>
            Edit
          </button>
        </div>
      </div>
    ` + "`" + `;
  }

  function renderStageKanbanBoardHTML() {
    const board = getEffectiveKanbanBoard(false);
    const cols = board.columns || [];

    return cols.map(col => {
      let cards = col.cards || [];
      if (activeSearch) {
        cards = cards.filter(c => {
          const title = (c.title || "").toLowerCase();
          const num = (c.number || "").toString();
          const agent = (c.assigned_agent ? (c.assigned_agent.agent_label || "") : "").toLowerCase();
          return title.includes(activeSearch) || num.includes(activeSearch) || agent.includes(activeSearch);
        });
      }

      return ` + "`" + `
        <div class="swiss-gh-kanban-col" data-col-id="${col.id}">
          <div class="swiss-gh-kanban-col-header">
            <div class="swiss-gh-kanban-col-title-wrap">
              <span>${escapeHTML(col.title)}</span>
              <span class="swiss-gh-col-counter">${cards.length}</span>
            </div>
          </div>
          <div class="swiss-gh-kanban-col-cards" data-col-id="${col.id}">
            ${cards.length === 0 ? '<div style="padding: 16px; text-align: center; font-size: 11px; color: var(--text-muted, #94a3b8); margin: auto;">No items</div>' : cards.map(c => renderKanbanCardHTML(c, col.id)).join("")}
          </div>
        </div>
      ` + "`" + `;
    }).join("");
  }

  function renderAuxKanbanBoardHTML() {
    const board = getEffectiveKanbanBoard(true);
    const cols = board.columns || [];

    return ` + "`" + `
      <div style="display: flex; gap: 8px; overflow-x: auto; height: 100%; padding: 8px 6px; box-sizing: border-box;">
        ${cols.map(col => {
          let cards = col.cards || [];
          if (activeSearch) {
            cards = cards.filter(c => {
              const title = (c.title || "").toLowerCase();
              const num = (c.number || "").toString();
              return title.includes(activeSearch) || num.includes(activeSearch);
            });
          }
          return ` + "`" + `
            <div class="swiss-gh-kanban-col" style="min-width: 200px; width: 200px; flex-shrink: 0;" data-col-id="${col.id}">
              <div class="swiss-gh-kanban-col-header">
                <div class="swiss-gh-kanban-col-title-wrap">
                  <span>${escapeHTML(col.title)}</span>
                  <span class="swiss-gh-col-counter">${cards.length}</span>
                </div>
              </div>
              <div class="swiss-gh-kanban-col-cards" data-col-id="${col.id}">
                ${cards.length === 0 ? '<div style="padding: 12px; text-align: center; font-size: 10.5px; color: var(--text-muted, #94a3b8); margin: auto;">No items</div>' : cards.map(c => renderKanbanCardHTML(c, col.id)).join("")}
              </div>
            </div>
          ` + "`" + `;
        }).join("")}
      </div>
    ` + "`" + `;
  }

  function bindKanbanDragEvents(containerEl, forAux = false) {
    const isAux = Boolean(forAux || (containerEl && containerEl.closest && containerEl.closest("#swiss-aux-container")));
    const getPRs = () => isAux ? (auxPRs || cachedPRs) : cachedPRs;
    const getIssues = () => isAux ? (auxIssues || cachedIssues) : cachedIssues;

    containerEl.querySelectorAll('.swiss-gh-kanban-card[draggable="true"]').forEach(card => {
      card.addEventListener("dragstart", (e) => {
        const itemType = card.dataset.itemType;
        const itemNum = parseInt(card.dataset.itemNum, 10);
        const item = (itemType === "pr" ? getPRs() : getIssues()).find(i => i.number === itemNum);

        draggedKanbanCard = {
          cardId: card.dataset.cardId,
          type: itemType,
          number: itemNum,
          sourceColumn: card.dataset.colId
        };
        card.classList.add("dragging");

        const markdownPayload = item ? formatMarkdownPayload(item, itemType, isAux) : ("#" + itemNum);
        e.dataTransfer.setData("application/json", JSON.stringify(draggedKanbanCard));
        e.dataTransfer.setData("text/plain", markdownPayload);
        e.dataTransfer.effectAllowed = "move";
      });

      card.addEventListener("dragend", () => {
        card.classList.remove("dragging");
        draggedKanbanCard = null;
        containerEl.querySelectorAll(".swiss-gh-kanban-col-cards").forEach(c => c.classList.remove("drag-over"));
      });

      card.addEventListener("contextmenu", (e) => {
        e.preventDefault();
        e.stopPropagation();
        const itemType = card.dataset.itemType;
        const itemNum = parseInt(card.dataset.itemNum, 10);
        const cardId = card.dataset.cardId || (itemType + "-" + itemNum);
        const currentCol = card.dataset.colId || getItemCurrentColumn(itemNum, itemType, isAux);
        showGitHubContextMenu(e, itemNum, itemType, cardId, currentCol, isAux);
      });
    });

    containerEl.querySelectorAll(".swiss-gh-kanban-col-cards").forEach(dropZone => {
      dropZone.addEventListener("dragover", (e) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        dropZone.classList.add("drag-over");
      });

      dropZone.addEventListener("dragleave", (e) => {
        if (!dropZone.contains(e.relatedTarget)) {
          dropZone.classList.remove("drag-over");
        }
      });

      dropZone.addEventListener("drop", async (e) => {
        e.preventDefault();
        e.stopPropagation();
        dropZone.classList.remove("drag-over");

        const targetCol = dropZone.dataset.colId;
        let cardData = draggedKanbanCard;
        if (!cardData) {
          try {
            const raw = e.dataTransfer.getData("application/json");
            if (raw) cardData = JSON.parse(raw);
          } catch (_) {}
        }

        if (cardData && cardData.sourceColumn !== targetCol) {
          const cardEl = containerEl.querySelector('[data-card-id="' + cardData.cardId + '"]');
          if (cardEl) {
            cardEl.dataset.colId = targetCol;
            dropZone.appendChild(cardEl);
          }
          await moveKanbanCard(cardData.cardId, cardData.type, cardData.number, cardData.sourceColumn, targetCol, isAux);
        }
      });
    });
  }

  function renderStageListViewHTML() {
    return ` + "`" + `
      <div class="swiss-gh-workspace-layout">
        <!-- Column 1: Issues / PRs / Agent Tasks List -->
        <div class="swiss-gh-col-left">
          <div class="swiss-gh-list" id="swiss-stage-gh-list">
            ${renderCardListHTML(getFilteredItems(false))}
          </div>
        </div>

        <!-- Column 2: In-Place Detail Viewer & Editor -->
        <div class="swiss-gh-col-center" id="swiss-stage-detail-col">
          ${renderStageDetailHTML(selectedItem)}
        </div>

        <!-- Column 3: Multi-Agent & Conversation Task Board -->
        <div class="swiss-gh-col-right">
          <div class="swiss-agent-board-header">
            <span>Agent Tasks & Conversations (${cachedAgentTasks.length})</span>
          </div>
          <div style="overflow-y: auto; flex: 1;">
            ${cachedAgentTasks.map(t => {
              const isWorking = t.not_fully_idle;
              return ` + "`" + `
                <div class="swiss-agent-conv-item">
                  <div style="display: flex; align-items: center; justify-content: space-between;">
                    <span class="swiss-agent-task-badge ${t.is_pruned ? "pruned" : (isWorking ? "working" : "idle")}">
                      <span class="swiss-agent-pulse-dot" style="${isWorking && !t.is_pruned ? "" : "display:none;"}"></span>
                      ${t.is_pruned ? "Archived" : (isWorking ? "Working" : "Idle")}: ${t.agent_label || "Agent"}
                    </span>
                    <button class="swiss-gh-action-btn swiss-btn-stage-focus" data-conv-id="${t.root_parent_conversation_id || t.conversation_id}" data-orig-id="${t.conversation_id}" style="font-size: 9.5px;">${t.is_pruned ? "Archived" : "Focus"}</button>
                  </div>
                  <div style="font-size: 11px; font-weight: 500; margin-top: 2px;">${escapeHTML(t.conversation_title || "Conversation")}</div>
                  <div style="display: flex; align-items: center; justify-content: space-between; margin-top: 4px; font-size: 10px; color: var(--text-muted, #94a3b8);">
                    <span>${t.bound_issue_number ? "Bound to #" + t.bound_issue_number : "Unbound"}</span>
                    <button class="swiss-gh-action-btn swiss-btn-stage-bind" data-conv-id="${t.conversation_id}">Bind Task</button>
                  </div>
                </div>
              ` + "`" + `;
            }).join("")}
          </div>
        </div>
      </div>
    ` + "`" + `;
  }

  // 10. Main Stage View Rendering (Kanban & List)
  function renderGitHubWorkspaceStage(container) {
    if (!selectedItem && cachedIssues.length > 0) {
      selectedItem = cachedIssues[0];
    }

    container.innerHTML = ` + "`" + `
      <div style="display: flex; flex-direction: column; width: 100%; height: 100%; overflow: hidden;">
        <div id="swiss-stage-content-body" style="flex: 1; min-height: 0; display: flex; overflow: hidden;">
          ${stageViewMode === "kanban" ? '<div class="swiss-gh-kanban-board" id="swiss-stage-kanban-board">' + renderStageKanbanBoardHTML() + '</div>' : renderStageListViewHTML()}
        </div>
      </div>
    ` + "`" + `;

    if (stageViewMode === "kanban") {
      bindKanbanDragEvents(container, false);
      bindCardEventListeners(container, false);
    } else {
      bindCardEventListeners(container, false);
      container.querySelectorAll(".swiss-gh-card").forEach(card => {
        card.addEventListener("click", () => {
          const num = parseInt(card.dataset.itemNum, 10);
          const item = (activeTab === "prs" ? cachedPRs : cachedIssues).find(i => i.number === num);
          if (item) {
            selectedItem = item;
            const detailCol = document.getElementById("swiss-stage-detail-col");
            if (detailCol) {
              detailCol.innerHTML = renderStageDetailHTML(selectedItem);
              bindStageDetailEvents(detailCol);
            }
          }
        });
      });
      bindStageDetailEvents(container);
    }
  }

  function renderStageDetailHTML(item) {

    if (!item) {
      return '<div style="margin: auto; color: var(--text-muted, #94a3b8); font-size: 13px;">Select an issue or pull request to view and edit</div>';
    }

    const isOpen = (item.state || "open").toLowerCase() === "open";
    return ` + "`" + `
      <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px;">
        <span class="swiss-gh-num-badge ${isOpen ? "open" : "closed"}" style="font-size: 12px; padding: 3px 8px;">
          #${item.number} ${(item.state || "open").toUpperCase()}
        </span>
        <div style="display: flex; gap: 6px;">
          <button class="swiss-gh-action-btn" id="swiss-stage-send-chat-btn" style="padding: 4px 8px; font-weight: 600;">
            Send to Agent Chat
          </button>
          <button class="swiss-gh-action-btn" id="swiss-stage-toggle-state-btn" style="padding: 4px 8px;">
            ${isOpen ? "Close Issue" : "Reopen Issue"}
          </button>
        </div>
      </div>
      <input type="text" id="swiss-stage-title-input" class="swiss-gh-editor-title" value="${escapeHTML(item.title)}">
      <div class="swiss-gh-editor-meta-bar">
        <span>Author: <strong>@${item.author || "user"}</strong></span>
        <span>•</span>
        <span>Created: ${new Date(item.created_at || Date.now()).toLocaleDateString()}</span>
        ${item.labels && item.labels.length > 0 ? "<span>•</span><span>Labels: " + item.labels.map(l => '<span class="swiss-gh-label-chip">' + escapeHTML(l) + '</span>').join(" ") + "</span>" : ""}
      </div>
      <div style="margin-bottom: 12px;">
        <label style="font-size: 11px; font-weight: 600; color: var(--text-muted, #64748b);">DESCRIPTION</label>
        <textarea id="swiss-stage-body-input" class="swiss-gh-textarea" style="margin-top: 4px; min-height: 220px;">${escapeHTML(item.body || "")}</textarea>
      </div>
      <div style="display: flex; justify-content: flex-end;">
        <button class="swiss-gh-save-btn" id="swiss-stage-save-btn">Save Changes to GitHub</button>
      </div>
    ` + "`" + `;
  }

  function bindStageDetailEvents(parentEl) {
    parentEl.querySelector("#swiss-stage-send-chat-btn")?.addEventListener("click", () => {
      if (selectedItem) {
        sendToChatComposer(formatMarkdownPayload(selectedItem, activeTab === "prs" ? "pr" : "issue"));
        showToast("Attached #" + selectedItem.number + " to chat");
      }
    });

    parentEl.querySelector("#swiss-stage-toggle-state-btn")?.addEventListener("click", async () => {
      if (!selectedItem) return;
      const isOpen = (selectedItem.state || "open").toLowerCase() === "open";
      const nextState = isOpen ? "closed" : "open";
      await fetch("http://127.0.0.1:8765/api/github/issues/update", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ workspace_path: getEffectiveWorkspacePath(), number: selectedItem.number, state: nextState })
      });
      showToast("Issue " + nextState);
      fetchRepoData();
    });

    parentEl.querySelector("#swiss-stage-save-btn")?.addEventListener("click", async () => {
      if (!selectedItem) return;
      const newTitle = parentEl.querySelector("#swiss-stage-title-input")?.value;
      const newBody = parentEl.querySelector("#swiss-stage-body-input")?.value;
      await fetch("http://127.0.0.1:8765/api/github/issues/update", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ workspace_path: getEffectiveWorkspacePath(), number: selectedItem.number, title: newTitle, body: newBody })
      });
      showToast("Saved to GitHub");
      fetchRepoData();
    });

    parentEl.querySelectorAll(".swiss-btn-stage-focus").forEach(btn => {
      btn.addEventListener("click", () => {
        const convId = btn.dataset.convId;
        const origId = btn.dataset.origId || convId;
        const task = cachedAgentTasks.find(t => t.conversation_id === origId || t.conversation_id === convId);
        navigateToConversation(convId, task?.root_parent_conversation_id, task?.is_pruned);
      });
    });

    parentEl.querySelectorAll(".swiss-btn-stage-bind").forEach(btn => {
      btn.addEventListener("click", async () => {
        const convId = btn.dataset.convId;
        const numStr = await showSwissPrompt("Enter GitHub Issue # to bind to this conversation (or 0 to unbind):");
        if (numStr !== null) {
          const num = parseInt(numStr, 10) || 0;
          await fetch("http://127.0.0.1:8765/api/github/agent-tasks/bind", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ conversation_id: convId, issue_number: num })
          });
          showToast("Task bound to conversation");
          fetchRepoData();
        }
      });
    });
  }

  function escapeHTML(str) {
    if (!str) return "";
    return str.replace(/[&<>'"]/g, tag => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      "'": '&#39;',
      '"': '&quot;'
    }[tag] || tag));
  }

  // Initial startup loop
  detectCurrentConversationProject();
  fetchAvailableProjects();
  fetchRepoData();
  if (prevOpenStageExt && document.getElementById("swiss-main-stage-container")?.style.display === "flex") {
    openMainStage(prevOpenStageExt);
  }
  setupLeftNavTabs();
  window.setupLeftNavTabs = setupLeftNavTabs;
  if (window.__swissLeftNavConfigHandler) {
    window.removeEventListener("swiss-left-nav-config-updated", window.__swissLeftNavConfigHandler);
  }
  window.__swissLeftNavConfigHandler = (e) => {
    if (e && e.detail) {
      if (e.detail.mode) {
        try { localStorage.setItem("antigravity_swiss_left_panel_mode", e.detail.mode); } catch (_) {}
        if (window.__SWISS_ENH_CONFIG__) window.__SWISS_ENH_CONFIG__.left_panel_extensions_mode = e.detail.mode;
      }
      if (e.detail.enabled !== undefined) {
        try {
          localStorage.setItem("antigravity_swiss_left_panel_enabled", String(e.detail.enabled));
          localStorage.setItem("antigravity_swiss_left_nav_enabled", String(e.detail.enabled));
          localStorage.setItem("antigravity_swiss_sidebar_extensions_enabled", String(e.detail.enabled));
        } catch (_) {}
        if (window.__SWISS_ENH_CONFIG__) window.__SWISS_ENH_CONFIG__.left_panel_extensions_enabled = e.detail.enabled;
      }
      if (e.detail.main_section_enabled !== undefined) {
        try { localStorage.setItem("antigravity_swiss_main_section_enabled", String(e.detail.main_section_enabled)); } catch (_) {}
        if (window.__SWISS_ENH_CONFIG__) window.__SWISS_ENH_CONFIG__.main_section_extensions_enabled = e.detail.main_section_enabled;
      }
    }
    setupLeftNavTabs();
  };
  window.addEventListener("swiss-left-nav-config-updated", window.__swissLeftNavConfigHandler);

  if (window.__swissGHNavInterval) clearInterval(window.__swissGHNavInterval);
  if (window.__swissGHFetchInterval) clearInterval(window.__swissGHFetchInterval);
  if (window.__swissGHDaemonCheckInterval) clearInterval(window.__swissGHDaemonCheckInterval);
  window.__swissGHNavInterval = setInterval(setupLeftNavTabs, 1500);
  window.__swissGHFetchInterval = setInterval(fetchRepoData, 30000);
  checkDaemonConnection();
  const daemonInterval = setInterval(checkDaemonConnection, 3000);
  if (daemonInterval && typeof daemonInterval.unref === "function") {
    daemonInterval.unref();
  }
  window.__swissGHDaemonCheckInterval = daemonInterval;
})();
`
}
