package plugins

// GenerateGitHubExtensionScript returns client-side JavaScript injected into Antigravity 2.0
// providing the Left Panel Extensions Bar, GitHub Workspace, Drag & Drop to Chat,
// In-Place Editor, Agent Task Correlation, and Main Stage View.
func GenerateGitHubExtensionScript() string {
	return `/* Antigravity Swiss Knife - Left Panel Extensions & Main Stage Engine */
(() => {
  if (typeof window === "undefined" || window.__swissGitHubExtInitialized) return;
  window.__swissGitHubExtInitialized = true;

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
  async function fetchRepoData() {
    try {
      const resRepo = await fetch("http://127.0.0.1:8765/api/github/repo?workspace_path=.");
      if (resRepo.ok) {
        const data = await resRepo.json();
        if (data.success && data.repo) {
          currentRepo = data.repo;
        }
      }
      const resIssues = await fetch("http://127.0.0.1:8765/api/github/issues?workspace_path=.&state=all");
      if (resIssues.ok) {
        const data = await resIssues.json();
        if (data.success && data.issues) {
          cachedIssues = data.issues;
        }
      }
      const resPRs = await fetch("http://127.0.0.1:8765/api/github/prs?workspace_path=.&state=all");
      if (resPRs.ok) {
        const data = await resPRs.json();
        if (data.success && data.prs) {
          cachedPRs = data.prs;
        }
      }
      const resTasks = await fetch("http://127.0.0.1:8765/api/github/agent-tasks?workspace_path=.");
      if (resTasks.ok) {
        const data = await resTasks.json();
        if (data.success && data.tasks) {
          cachedAgentTasks = data.tasks;
        }
      }
      const resKanban = await fetch("http://127.0.0.1:8765/api/github/kanban?workspace_path=.");
      if (resKanban.ok) {
        const data = await resKanban.json();
        if (data.success && data.board) {
          cachedKanbanBoard = data.board;
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
      if (!navContainer) return;

      let group = document.getElementById("swiss-left-nav-group");
      if (!group) {
        group = document.createElement("div");
        group.id = "swiss-left-nav-group";
        group.className = "swiss-left-nav-group flex flex-col gap-1.5 mt-1";

        const sep = document.createElement("div");
        sep.className = "swiss-left-tabs-separator";
        group.appendChild(sep);

        const exts = [
          {
            id: "github",
            label: "GitHub Workspace",
            svg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/></svg>'
          },
          {
            id: "browser",
            label: "Browser Preview",
            svg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
          },
          {
            id: "files",
            label: "File Explorer",
            svg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>'
          },
          {
            id: "memos",
            label: "Quick Memos",
            svg: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M16 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V8Z"/><polyline points="15 3 15 8 20 8"/><line x1="9" y1="13" x2="15" y2="13"/><line x1="9" y1="17" x2="13" y2="17"/></svg>'
          }
        ];

        exts.forEach(ext => {
          const btn = document.createElement("button");
          btn.className = "inline-flex items-center font-medium transition-colors select-none outline-none cursor-pointer disabled:opacity-50 flex-grow w-full justify-start select-none font-normal h-8 min-w-0 gap-1.5 px-2 py-1 rounded-lg bg-transparent text-secondary-foreground hover:bg-sidebar-muted hover:text-foreground swiss-left-nav-tab";
          btn.dataset.swissExt = ext.id;
          btn.title = "Open " + ext.label + " in Main Area (Replace / Split Chat)";
          btn.innerHTML = '<span class="shrink-0 flex items-center">' + ext.svg + '</span>' +
                          '<span class="truncate text-sm">' + ext.label + '</span>';
          btn.onclick = (e) => {
            e.stopPropagation();
            if (activeMainStageExt === ext.id) {
              closeMainStage();
            } else {
              openMainStage(ext.id);
            }
          };
          group.appendChild(btn);
        });

        const sepBottom = document.createElement("div");
        sepBottom.className = "swiss-left-tabs-separator";
        group.appendChild(sepBottom);

        navContainer.appendChild(group);
      } else if (group.parentElement !== navContainer) {
        navContainer.appendChild(group);
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

      // Sync active state on tabs
      group.querySelectorAll('.swiss-left-nav-tab').forEach(b => {
        const isActive = activeMainStageExt && b.dataset.swissExt === activeMainStageExt;
        if (isActive) {
          b.className = "inline-flex items-center font-medium transition-colors select-none outline-none cursor-pointer disabled:opacity-50 flex-grow w-full justify-start select-none font-normal h-8 min-w-0 gap-1.5 px-2 py-1 rounded-lg border border-border text-foreground bg-background hover:bg-card swiss-left-nav-tab active";
        } else {
          b.className = "inline-flex items-center font-medium transition-colors select-none outline-none cursor-pointer disabled:opacity-50 flex-grow w-full justify-start select-none font-normal h-8 min-w-0 gap-1.5 px-2 py-1 rounded-lg bg-transparent text-secondary-foreground hover:bg-sidebar-muted hover:text-foreground swiss-left-nav-tab";
        }
      });

      // Bind sidebar / project panel click listener to close stage when user clicks a chat or project item
      const sidebar = navContainer.closest(".bg-sidebar");
      if (sidebar && !sidebar.__swissSidebarClickBound) {
        sidebar.__swissSidebarClickBound = true;
        sidebar.addEventListener("click", (e) => {
          const target = e.target.closest('a, button, [data-testid="conversation-row-sidebar"]');
          if (target && !target.classList.contains('swiss-left-nav-tab')) {
            closeMainStage();
          }
        });
      } else if (!navContainer.__swissFactoryClickBound) {
        navContainer.__swissFactoryClickBound = true;
        navContainer.addEventListener("click", (e) => {
          const target = e.target.closest('a, button');
          if (target && !target.classList.contains('swiss-left-nav-tab')) {
            closeMainStage();
          }
        });
      }
    } catch (_) {}
  }

  // Global listener: clicking any conversation row, chat tab, or /c/ navigation returns to chat
  if (!window.__swissChatNavBound) {
    window.__swissChatNavBound = true;
    document.addEventListener("click", (e) => {
      const convTrigger = e.target.closest('[data-testid="conversation-row-sidebar"], [data-testid="new-conversation-button"], [data-testid="history-button"], a[href*="/c/"]');
      if (convTrigger && activeMainStageExt) {
        closeMainStage();
      }
    }, true);
  }

  if (!window.__swissPopstateBound) {
    window.__swissPopstateBound = true;
    window.addEventListener("popstate", () => {
      if (activeMainStageExt) {
        closeMainStage();
      }
    });
  }

  // 4. Render GitHub Workspace in Right Panel Auxiliary Container
  window.renderSwissGitHubWorkspaceView = function(container) {
    if (!container) return;
    container.innerHTML = "";

    const auxView = document.createElement("div");
    auxView.className = "swiss-github-aux-view";

    const repoName = currentRepo ? currentRepo.FullName : "Detecting repo...";
    const branchName = currentRepo ? currentRepo.CurrentBranch : "main";
    const filteredItems = getFilteredItems();

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
        <button class="swiss-gh-tab-btn ${activeTab === "issues" ? "active" : ""}" data-tab="issues">Issues (${cachedIssues.length})</button>
        <button class="swiss-gh-tab-btn ${activeTab === "prs" ? "active" : ""}" data-tab="prs">PRs (${cachedPRs.length})</button>
        <button class="swiss-gh-tab-btn ${activeTab === "tasks" ? "active" : ""}" data-tab="tasks">Agent Tasks (${cachedAgentTasks.length})</button>
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
        listEl.innerHTML = activeTab === "board" ? renderAuxKanbanBoardHTML() : renderCardListHTML(getFilteredItems());
        if (activeTab === "board") {
          bindKanbanDragEvents(auxView);
        }
        bindCardEventListeners(listEl);
      }
    });

    if (activeTab === "board") {
      bindKanbanDragEvents(auxView);
    }
    bindCardEventListeners(auxView);
  };

  function getFilteredItems() {
    let items = [];
    if (activeTab === "issues") {
      items = cachedIssues;
    } else if (activeTab === "prs") {
      items = cachedPRs;
    } else if (activeTab === "tasks") {
      items = cachedAgentTasks;
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
        const shortId = assigned.conversation_id ? assigned.conversation_id.slice(0, 6) : "";
        const label = assigned.agent_label || assigned.agent_name || "Agent";
        agentHTML = ` + "`" + `
          <span class="swiss-agent-task-badge ${isWorking ? "working" : "idle"}" data-conv-id="${assigned.conversation_id || ""}" title="Agent: ${escapeHTML(label)}${shortId ? " [#" + shortId + "]" : ""}&#10;Click to jump to conversation" style="cursor: pointer;">
            <span class="swiss-agent-pulse-dot" style="${isWorking ? "" : "display:none;"}"></span>
            ${escapeHTML(label)}${shortId ? " [#" + shortId + "]" : ""}
          </span>
        ` + "`" + `;
      }

      const labelsHTML = labels.slice(0, 3).map(l => '<span class="swiss-gh-label-chip">' + escapeHTML(l) + '</span>').join("");

      return ` + "`" + `
        <div class="swiss-gh-card" draggable="true" data-item-type="${isPR ? "pr" : "issue"}" data-item-num="${num}">
          <div class="swiss-gh-card-header">
            <div class="swiss-gh-badge-row">
              <span style="cursor: grab; opacity: 0.6; font-size: 10px;" title="Drag into chat">⋮⋮</span>
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
            ${it.url ? '<a href="' + it.url + '" target="_blank" class="swiss-gh-action-btn" title="Open on GitHub">↗</a>' : ""}
          </div>
        </div>
      ` + "`" + `;
    }).join("");
  }

  function bindCardEventListeners(parentEl) {
    // 1. Drag & drop handlers on cards
    parentEl.querySelectorAll('.swiss-gh-card[draggable="true"]').forEach(card => {
      card.addEventListener("dragstart", (e) => {
        const itemType = card.dataset.itemType;
        const itemNum = parseInt(card.dataset.itemNum, 10);
        const item = (itemType === "pr" ? cachedPRs : cachedIssues).find(i => i.number === itemNum);
        if (!item) return;

        card.classList.add("dragging");
        const payload = formatMarkdownPayload(item, itemType);
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
        const currentCol = getItemCurrentColumn(itemNum, itemType);
        showGitHubContextMenu(e, itemNum, itemType, cardId, currentCol);
      });
    });

    // 2. Send to Chat button
    parentEl.querySelectorAll(".swiss-btn-send-chat").forEach(btn => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        const num = parseInt(btn.dataset.itemNum, 10);
        const type = btn.dataset.itemType;
        const item = (type === "pr" ? cachedPRs : cachedIssues).find(i => i.number === num);
        if (item) {
          sendToChatComposer(formatMarkdownPayload(item, type));
          showToast("Attached #" + num + " to prompt");
        }
      });
    });

    // 3. Edit in modal button
    parentEl.querySelectorAll(".swiss-btn-edit-modal").forEach(btn => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        const num = parseInt(btn.dataset.itemNum, 10);
        openIssueDetailModal(num);
      });
    });

    // 4. Focus conversation
    parentEl.querySelectorAll(".swiss-btn-focus-conv").forEach(btn => {
      btn.addEventListener("click", () => {
        const convId = btn.dataset.convId;
        window.location.href = "/c/" + convId;
      });
    });

    parentEl.querySelectorAll(".swiss-agent-task-badge[data-conv-id]").forEach(badge => {
      badge.addEventListener("click", (e) => {
        const convId = badge.dataset.convId;
        if (convId) {
          e.stopPropagation();
          window.location.href = "/c/" + convId;
        }
      });
    });

    // 5. Label agent
    parentEl.querySelectorAll(".swiss-btn-label-agent").forEach(btn => {
      btn.addEventListener("click", async () => {
        const convId = btn.dataset.convId;
        const currentTask = cachedAgentTasks.find(t => t.conversation_id === convId);
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
  function formatMarkdownPayload(item, type) {
    const isPR = type === "pr";
    const repoFullName = currentRepo ? currentRepo.FullName : "Repo";
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
    const dropZones = [
      document.querySelector('[data-testid="conversation-view"]'),
      document.querySelector('[contenteditable="true"]')?.closest('form, [class*="composer"]')
    ].filter(Boolean);

    document.addEventListener("dragover", (e) => {
      const composer = document.querySelector('[contenteditable="true"]');
      if (composer && (composer === e.target || composer.contains(e.target))) {
        e.preventDefault();
        composer.classList.add("swiss-chat-drop-highlight");
      }
    });

    document.addEventListener("dragleave", (e) => {
      const composer = document.querySelector('[contenteditable="true"]');
      if (composer && !composer.contains(e.relatedTarget)) {
        composer.classList.remove("swiss-chat-drop-highlight");
      }
    });

    document.addEventListener("drop", (e) => {
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
    });
  }

  // 7. In-Place Detail Viewer & Editor Modal
  async function openIssueDetailModal(number) {
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
          <button style="border: none; background: transparent; cursor: pointer; font-size: 16px;" id="swiss-gh-modal-close">✕</button>
        </div>
      </div>
    ` + "`" + `;

    modal.querySelector("#swiss-gh-modal-close")?.addEventListener("click", () => modal.remove());
    modal.addEventListener("click", (e) => { if (e.target === modal) modal.remove(); });

    try {
      const res = await fetch("http://127.0.0.1:8765/api/github/issues/detail?workspace_path=.&number=" + number);
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
            <button style="border: none; background: transparent; cursor: pointer; font-size: 16px; color: var(--text-muted, #64748b);" id="swiss-gh-modal-close">✕</button>
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
          body: JSON.stringify({ number: number, state: nextState })
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
          body: JSON.stringify({ number: number, title: newTitle, body: newBody })
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
          body: JSON.stringify({ number: number, comment: commText })
        });
        showToast("Comment posted");
        openIssueDetailModal(number);
      });
    } catch (_) {
      modal.remove();
    }
  }

  // 8. Main Stage View Mode (Replaces / Splits Chat Viewport)
  function openMainStage(extType = "github") {
    activeMainStageExt = extType;
    const convoView = document.querySelector('[data-testid="conversation-view"]');
    if (!convoView) return;

    const convoParent = convoView.parentElement;
    if (!convoParent) return;

    let stageContainer = document.getElementById("swiss-main-stage-container");
    if (!stageContainer) {
      stageContainer = document.createElement("div");
      stageContainer.id = "swiss-main-stage-container";
      convoParent.appendChild(stageContainer);
    }

    applyMainStageLayout();
    renderMainStageUI();
    setupLeftNavTabs();
  }

  function closeMainStage() {
    activeMainStageExt = null;
    const convoView = document.querySelector('[data-testid="conversation-view"]');
    const stageContainer = document.getElementById("swiss-main-stage-container");
    if (stageContainer) stageContainer.style.display = "none";

    if (convoView) {
      convoView.style.display = "";
      convoView.style.flex = "";
      convoView.style.width = "";
      if (convoView.parentElement) {
        convoView.parentElement.style.display = "";
        convoView.parentElement.style.flexDirection = "";
      }
    }
    setupLeftNavTabs();
  }

  function applyMainStageLayout() {
    const convoView = document.querySelector('[data-testid="conversation-view"]');
    const stageContainer = document.getElementById("swiss-main-stage-container");
    if (!convoView || !stageContainer) return;

    const parent = convoView.parentElement;
    stageContainer.style.display = "flex";
    stageContainer.style.flex = "1 1 100%";
    stageContainer.style.width = "100%";
    stageContainer.style.height = "100%";
    stageContainer.style.borderLeft = "none";

    convoView.style.display = "none";
    if (parent) {
      parent.style.display = "flex";
      parent.style.flexDirection = "column";
    }
  }

  function renderMainStageUI() {
    const container = document.getElementById("swiss-main-stage-container");
    if (!container) return;

    container.innerHTML = ` + "`" + `
      <div id="swiss-main-stage-header">
        <div class="swiss-main-stage-tabs">
          <button class="swiss-main-stage-tab ${activeMainStageExt === "github" ? "active" : ""}" data-ext="github">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"></path>
            </svg>
            GitHub Workspace
          </button>
          <button class="swiss-main-stage-tab ${activeMainStageExt === "browser" ? "active" : ""}" data-ext="browser">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="2" y1="12" x2="22" y2="12"></line>
            </svg>
            Browser Preview
          </button>
          <button class="swiss-main-stage-tab ${activeMainStageExt === "files" ? "active" : ""}" data-ext="files">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
            </svg>
            File Explorer
          </button>
          <button class="swiss-main-stage-tab ${activeMainStageExt === "memos" ? "active" : ""}" data-ext="memos">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
            </svg>
            Quick Memos
          </button>
        </div>
      </div>
      <div id="swiss-main-stage-body"></div>
    ` + "`" + `;

    // Event listeners
    container.querySelectorAll(".swiss-main-stage-tab").forEach(tab => {
      tab.addEventListener("click", (e) => {
        activeMainStageExt = e.currentTarget.dataset.ext;
        renderMainStageUI();
        setupLeftNavTabs();
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
  function getEffectiveKanbanBoard() {
    if (cachedKanbanBoard && cachedKanbanBoard.columns && cachedKanbanBoard.columns.length > 0) {
      return cachedKanbanBoard;
    }
    const colMap = { todo: [], in_progress: [], review: [], done: [] };
    const isWIP = l => ["in progress", "in-progress", "wip", "doing", "active", "working"].includes((l || "").toLowerCase());
    const isRev = l => ["review", "in review", "in-review", "needs review", "under review", "qa"].includes((l || "").toLowerCase());

    cachedIssues.forEach(iss => {
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

    cachedPRs.forEach(pr => {
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

  function getItemCurrentColumn(number, type) {
    const board = getEffectiveKanbanBoard();
    if (board && board.columns) {
      for (const col of board.columns) {
        if ((col.cards || []).some(c => c.number === number && (c.type === type || (c.id && c.id.startsWith(type))))) {
          return col.id;
        }
      }
    }
    if (type === "pr") {
      const pr = cachedPRs.find(p => p.number === number);
      if (pr) {
        const st = (pr.state || "open").toLowerCase();
        if (st === "closed" || st === "merged") return "done";
        if (pr.is_draft) return "in_progress";
        return "review";
      }
      return "review";
    }
    const iss = cachedIssues.find(i => i.number === number);
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

  async function moveKanbanCard(cardId, cardType, number, sourceCol, targetCol) {
    if (sourceCol === targetCol) return;
    try {
      const res = await fetch("http://127.0.0.1:8765/api/github/kanban/move", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          workspace_path: ".",
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

  function showGitHubContextMenu(e, itemNum, itemType, cardId, currentColId) {
    removeGHContextMenu();

    const isPR = itemType === "pr" || (cardId && cardId.startsWith("pr-"));
    const normalizedType = isPR ? "pr" : "issue";
    const item = (normalizedType === "pr" ? cachedPRs : cachedIssues).find(i => i.number === itemNum);
    const currentCol = currentColId || getItemCurrentColumn(itemNum, normalizedType);
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
        (isCurrent ? '<span class="swiss-gh-menu-check">✓</span>' : '') +
      '</button>';
    }).join("");

    let chatLabel = "Chat with Agent";
    if (item && item.assigned_agent && item.assigned_agent.conversation_id) {
      const shortId = item.assigned_agent.conversation_id.slice(0, 6);
      const label = item.assigned_agent.agent_label || item.assigned_agent.agent_name || "Agent";
      chatLabel = "Jump to " + label + " [#" + shortId + "]";
    }

    let menuHTML = '<div class="swiss-gh-menu-header">Move to Board</div>' +
      categoryItemsHTML +
      '<div class="swiss-gh-menu-divider"></div>' +
      '<button class="swiss-gh-menu-item" id="swiss-gh-ctx-chat"><span>' + escapeHTML(chatLabel) + '</span></button>' +
      '<button class="swiss-gh-menu-item" id="swiss-gh-ctx-edit"><span>View & Edit Details</span></button>';

    if (item && item.url) {
      menuHTML += '<button class="swiss-gh-menu-item" id="swiss-gh-ctx-open"><span>Open on GitHub ↗</span></button>';
    }

    menu.innerHTML = menuHTML;

    menu.querySelectorAll(".swiss-gh-menu-item[data-target-col]").forEach(btn => {
      btn.addEventListener("click", async (ev) => {
        ev.stopPropagation();
        const targetCol = btn.dataset.targetCol;
        removeGHContextMenu();
        await moveKanbanCard(actualCardId, normalizedType, itemNum, currentCol, targetCol);
      });
    });

    menu.querySelector("#swiss-gh-ctx-chat")?.addEventListener("click", (ev) => {
      ev.stopPropagation();
      removeGHContextMenu();
      if (item && item.assigned_agent && item.assigned_agent.conversation_id) {
        window.location.href = "/c/" + item.assigned_agent.conversation_id;
      } else if (item) {
        sendToChatComposer(formatMarkdownPayload(item, normalizedType));
        showToast("Attached #" + itemNum + " to prompt");
      }
    });

    menu.querySelector("#swiss-gh-ctx-edit")?.addEventListener("click", (ev) => {
      ev.stopPropagation();
      removeGHContextMenu();
      openIssueDetailModal(itemNum);
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
      const shortId = assigned.conversation_id ? assigned.conversation_id.slice(0, 6) : "";
      const label = assigned.agent_label || assigned.agent_name || "Agent";
      agentHTML = ` + "`" + `
        <span class="swiss-agent-task-badge ${isWorking ? "working" : "idle"}" data-conv-id="${assigned.conversation_id || ""}" title="Agent: ${escapeHTML(label)}${shortId ? " [#" + shortId + "]" : ""}&#10;Click to jump to conversation" style="cursor: pointer;">
          <span class="swiss-agent-pulse-dot" style="${isWorking ? "" : "display:none;"}"></span>
          ${isWorking ? "Working" : "Idle"}: ${escapeHTML(label)}${shortId ? " [#" + shortId + "]" : ""}
        </span>
      ` + "`" + `;
    }

    const labelsHTML = labels.slice(0, 3).map(l => '<span class="swiss-gh-label-chip">' + escapeHTML(l) + '</span>').join("");

    return ` + "`" + `
      <div class="swiss-gh-kanban-card" draggable="true" data-card-id="${card.id}" data-item-type="${isPR ? "pr" : "issue"}" data-item-num="${num}" data-col-id="${colId}">
        <div class="swiss-gh-card-header">
          <div class="swiss-gh-badge-row">
            <span style="cursor: grab; opacity: 0.6; font-size: 10px;" title="Drag card across columns or into chat">⋮⋮</span>
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
    const board = getEffectiveKanbanBoard();
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
    const board = getEffectiveKanbanBoard();
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

  function bindKanbanDragEvents(containerEl) {
    containerEl.querySelectorAll('.swiss-gh-kanban-card[draggable="true"]').forEach(card => {
      card.addEventListener("dragstart", (e) => {
        const itemType = card.dataset.itemType;
        const itemNum = parseInt(card.dataset.itemNum, 10);
        const item = (itemType === "pr" ? cachedPRs : cachedIssues).find(i => i.number === itemNum);

        draggedKanbanCard = {
          cardId: card.dataset.cardId,
          type: itemType,
          number: itemNum,
          sourceColumn: card.dataset.colId
        };
        card.classList.add("dragging");

        const markdownPayload = item ? formatMarkdownPayload(item, itemType) : ("#" + itemNum);
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
        const currentCol = card.dataset.colId || getItemCurrentColumn(itemNum, itemType);
        showGitHubContextMenu(e, itemNum, itemType, cardId, currentCol);
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
          await moveKanbanCard(cardData.cardId, cardData.type, cardData.number, cardData.sourceColumn, targetCol);
        }
      });
    });
  }

  function renderStageListViewHTML() {
    return ` + "`" + `
      <div class="swiss-gh-workspace-layout">
        <!-- Column 1: Issues / PRs List -->
        <div class="swiss-gh-col-left">
          <div class="swiss-gh-tabs-row">
            <button class="swiss-gh-tab-btn ${stageViewMode === "kanban" ? "active" : ""}" data-tab="board">Board</button>
            <button class="swiss-gh-tab-btn ${activeTab === "issues" && stageViewMode === "list" ? "active" : ""}" data-tab="issues">Issues (${cachedIssues.length})</button>
            <button class="swiss-gh-tab-btn ${activeTab === "prs" && stageViewMode === "list" ? "active" : ""}" data-tab="prs">PRs (${cachedPRs.length})</button>
          </div>
          <div class="swiss-gh-list" id="swiss-stage-gh-list">
            ${renderCardListHTML(getFilteredItems())}
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
                    <span class="swiss-agent-task-badge ${isWorking ? "working" : "idle"}">
                      <span class="swiss-agent-pulse-dot" style="${isWorking ? "" : "display:none;"}"></span>
                      ${isWorking ? "Working" : "Idle"}: ${t.agent_label || "Agent"}
                    </span>
                    <button class="swiss-gh-action-btn swiss-btn-stage-focus" data-conv-id="${t.conversation_id}" style="font-size: 9.5px;">Focus</button>
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
        <div style="display: flex; align-items: center; justify-content: space-between; padding: 6px 14px; border-bottom: 1px solid var(--border, #e2e8f0); background: var(--canvas, #ffffff); flex-shrink: 0;">
          <div style="display: flex; align-items: center; gap: 8px;">
            <span style="font-weight: 600; font-size: 13px;">${currentRepo ? currentRepo.FullName : "Repository"}</span>
            <span class="swiss-gh-branch-chip">
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="6" y1="3" x2="6" y2="15"></line>
                <circle cx="18" cy="6" r="3"></circle>
                <circle cx="6" cy="18" r="3"></circle>
                <path d="M18 9a9 9 0 0 1-9 9"></path>
              </svg>
              ${currentRepo ? currentRepo.CurrentBranch : "main"}
            </span>
          </div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <div class="swiss-gh-view-switcher">
              <button class="swiss-gh-view-btn ${stageViewMode === "kanban" ? "active" : ""}" id="swiss-stage-view-kanban">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <rect x="3" y="3" width="7" height="18" rx="1"></rect>
                  <rect x="14" y="3" width="7" height="18" rx="1"></rect>
                </svg>
                Kanban Board
              </button>
              <button class="swiss-gh-view-btn ${stageViewMode === "list" ? "active" : ""}" id="swiss-stage-view-list">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="8" y1="6" x2="21" y2="6"></line>
                  <line x1="8" y1="12" x2="21" y2="12"></line>
                  <line x1="8" y1="18" x2="21" y2="18"></line>
                  <line x1="3" y1="6" x2="3.01" y2="6"></line>
                  <line x1="3" y1="12" x2="3.01" y2="12"></line>
                  <line x1="3" y1="18" x2="3.01" y2="18"></line>
                </svg>
                List View
              </button>
            </div>
            <button class="swiss-gh-action-btn" id="swiss-stage-gh-refresh" title="Refresh">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="23 4 23 10 17 10"></polyline>
                <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path>
              </svg>
            </button>
          </div>
        </div>
        <div id="swiss-stage-content-body" style="flex: 1; min-height: 0; display: flex; overflow: hidden;">
          ${stageViewMode === "kanban" ? '<div class="swiss-gh-kanban-board" id="swiss-stage-kanban-board">' + renderStageKanbanBoardHTML() + '</div>' : renderStageListViewHTML()}
        </div>
      </div>
    ` + "`" + `;

    container.querySelector("#swiss-stage-gh-refresh")?.addEventListener("click", fetchRepoData);
    container.querySelector("#swiss-stage-view-kanban")?.addEventListener("click", () => {
      stageViewMode = "kanban";
      renderGitHubWorkspaceStage(container);
    });
    container.querySelector("#swiss-stage-view-list")?.addEventListener("click", () => {
      stageViewMode = "list";
      renderGitHubWorkspaceStage(container);
    });

    if (stageViewMode === "kanban") {
      bindKanbanDragEvents(container);
      bindCardEventListeners(container);
    } else {
      bindCardEventListeners(container);
      container.querySelectorAll(".swiss-gh-col-left .swiss-gh-tab-btn").forEach(btn => {
        btn.addEventListener("click", () => {
          const tab = btn.dataset.tab;
          if (tab === "board") {
            stageViewMode = "kanban";
            renderGitHubWorkspaceStage(container);
          } else {
            activeTab = tab;
            stageViewMode = "list";
            renderGitHubWorkspaceStage(container);
          }
        });
      });
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
        body: JSON.stringify({ number: selectedItem.number, state: nextState })
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
        body: JSON.stringify({ number: selectedItem.number, title: newTitle, body: newBody })
      });
      showToast("Saved to GitHub");
      fetchRepoData();
    });

    parentEl.querySelectorAll(".swiss-btn-stage-focus").forEach(btn => {
      btn.addEventListener("click", () => {
        window.location.href = "/c/" + btn.dataset.convId;
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
  fetchRepoData();
  setupLeftNavTabs();
  setupDragAndDropToChat();

  setInterval(setupLeftNavTabs, 1500);
  setInterval(fetchRepoData, 30000);
})();
`
}
