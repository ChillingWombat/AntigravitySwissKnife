package plugins

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGenerateGitHubExtensionCSS(t *testing.T) {
	css := GenerateGitHubExtensionCSS()
	if css == "" {
		t.Fatalf("expected non-empty CSS")
	}

	requiredSelectors := []string{
		".swiss-left-nav-group",
		".swiss-left-tabs-separator",
		".swiss-left-nav-tab",
		".swiss-left-nav-tab:hover",
		".swiss-left-nav-tab.active",
		":is(.dark, [data-theme=\"dark\"]) .swiss-left-nav-tab.active",
		".swiss-github-aux-view",
		".swiss-gh-repo-bar",
		".swiss-gh-kanban-board",
		".swiss-gh-kanban-col",
		".swiss-gh-kanban-card",
		"#swiss-main-stage-container",
		"#swiss-main-stage-header",
		"padding: 0 10px;",
		".swiss-stage-native-split-btn",
		".swiss-main-stage-close-btn",
		".swiss-main-stage-tab",
		"#swiss-main-stage-body",
		".swiss-gh-context-menu",
		".swiss-gh-menu-item",
		".swiss-scope-picker",
		".swiss-scope-btn",
		".swiss-scope-dropdown-menu",
		".swiss-scope-item",
		"[data-swiss-stage-active]",
		"[data-swiss-stage-active] [data-testid=\"automations-button\"]",
	}

	for _, sel := range requiredSelectors {
		if !strings.Contains(css, sel) {
			t.Errorf("expected CSS to contain selector %q", sel)
		}
	}
}

func TestGenerateGitHubExtensionScript(t *testing.T) {
	js := GenerateGitHubExtensionScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredIdentifiers := []string{
		"window.__swissGitHubExtInitialized",
		"window.__swissGHNavInterval",
		"window.__swissGHFetchInterval",
		"getLeftSidebar",
		"hideNativeBreadcrumbBar",
		"restoreNativeBreadcrumbBar",
		"knownProjects",
		"data-cascade-id",
		"breadcrumb-segment",
		"fetchRepoData",
		"setupLeftNavTabs",
		"renderSwissGitHubWorkspaceView",
		"renderStageKanbanBoardHTML",
		"renderAuxKanbanBoardHTML",
		"getEffectiveKanbanBoard",
		"bindKanbanDragEvents",
		"moveKanbanCard",
		"showGitHubContextMenu",
		"getItemCurrentColumn",
		"openMainStage",
		"closeMainStage",
		"renderMainStageUI",
		"renderMainStageContent",
		"__swissChatNavBound",
		"__swissScopeOutsideClickBound",
		"isStageDisplayed",
		"window.closeMainStage",
		"currentScope",
		"availableProjects",
		"lastInteractedProject",
		"getMainStageParent",
		"extractProjectFromElement",
		"recordProjectInteraction",
		"getEffectiveWorkspacePath",
		"fetchAvailableProjects",
		"swiss-stage-scope-btn",
		"swiss-stage-scope-menu",
		"swiss-stage-gh-tabs",
		"effRepo.full_name",
		"effRepo.current_branch",
		"triggerNativeSplit",
		"swiss-stage-native-split-btn",
		"data-swiss-stage-active",
		"data-swiss-suppressed",
	}

	for _, id := range requiredIdentifiers {
		if !strings.Contains(js, id) {
			t.Errorf("expected script to contain identifier %q", id)
		}
	}

	// Verify RepoInfo PascalCase fields are not used on currentRepo in Main Stage top bar
	if strings.Contains(js, "currentRepo.FullName") {
		t.Errorf("expected Main Stage top bar to not reference currentRepo.FullName")
	}

	// Verify Board is leftmost tab before Issues in the tabs row
	boardIdx := strings.Index(js, `data-tab="board">Board</button>`)
	issuesIdx := strings.Index(js, `data-tab="issues">Issues`)
	if boardIdx == -1 || issuesIdx == -1 || boardIdx > issuesIdx {
		t.Errorf("expected Board tab (idx %d) to appear before Issues tab (idx %d)", boardIdx, issuesIdx)
	}

	if nodePath, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command(nodePath, "--check")
		cmd.Stdin = strings.NewReader(js)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node syntax error in GenerateGitHubExtensionScript: %v\n%s", err, string(out))
		}
	}
}

func TestGitHubExtensionScriptDOMRuntime(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	js := GenerateGitHubExtensionScript()
	harness := `
const assert = require("assert");

class MockClassList {
  constructor(el) {
    this.el = el;
  }
  _set() {
    return new Set((this.el.className || "").split(/\s+/).filter(Boolean));
  }
  contains(cls) {
    return this._set().has(cls);
  }
  add(...classes) {
    const s = this._set();
    classes.forEach(c => s.add(c));
    this.el.className = Array.from(s).join(" ");
  }
  remove(...classes) {
    const s = this._set();
    classes.forEach(c => s.delete(c));
    this.el.className = Array.from(s).join(" ");
  }
}

class MockElement {
  constructor(tagName = "div") {
    this.tagName = tagName.toUpperCase();
    this.id = "";
    this.className = "";
    this.style = {};
    this.dataset = {};
    this.attributes = {};
    this.children = [];
    this.parentElement = null;
    this.textContent = "";
    this.clientWidth = 300;
    this.listeners = {};
    this.classList = new MockClassList(this);
    this._innerHTML = "";
  }
  set innerHTML(val) {
    this._innerHTML = String(val || "");
    this.children = [];
    if (this._innerHTML.includes('id="swiss-main-stage-header"')) {
      const hdr = new MockElement("div");
      hdr.id = "swiss-main-stage-header";
      const scopeBtn = new MockElement("button");
      scopeBtn.id = "swiss-stage-scope-btn";
      hdr.appendChild(scopeBtn);
      const scopeMenu = new MockElement("div");
      scopeMenu.id = "swiss-stage-scope-menu";
      scopeMenu.style.display = "none";
      hdr.appendChild(scopeMenu);
      ["browser", "files", "memos", "github"].forEach(ext => {
        const tab = new MockElement("button");
        tab.className = "swiss-main-stage-tab" + (this._innerHTML.includes('data-ext="' + ext + '"') && this._innerHTML.includes('active" data-ext="' + ext + '"') ? " active" : "");
        tab.setAttribute("data-ext", ext);
        hdr.appendChild(tab);
      });
      const splitBtn = new MockElement("button");
      splitBtn.id = "swiss-stage-native-split-btn";
      hdr.appendChild(splitBtn);
      const closeBtn = new MockElement("button");
      closeBtn.id = "swiss-stage-close-btn";
      hdr.appendChild(closeBtn);
      this.appendChild(hdr);
      const body = new MockElement("div");
      body.id = "swiss-main-stage-body";
      this.appendChild(body);
    }
  }
  get innerHTML() {
    return this._innerHTML;
  }
  setAttribute(k, v) {
    this.attributes[k] = String(v);
    if (k === "id") this.id = String(v);
    if (k === "class") this.className = String(v);
    if (k.startsWith("data-")) {
      const camel = k.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase());
      this.dataset[camel] = String(v);
    }
  }
  getAttribute(k) {
    if (k === "id") return this.id || null;
    if (k === "class") return this.className || null;
    if (Object.prototype.hasOwnProperty.call(this.attributes, k)) return this.attributes[k];
    if (k.startsWith("data-")) {
      const camel = k.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase());
      if (Object.prototype.hasOwnProperty.call(this.dataset, camel)) return this.dataset[camel];
    }
    return null;
  }
  removeAttribute(k) {
    delete this.attributes[k];
    if (k.startsWith("data-")) {
      const camel = k.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase());
      delete this.dataset[camel];
    }
  }
  hasAttribute(k) {
    return this.getAttribute(k) !== null;
  }
  appendChild(child) {
    if (child.parentElement) {
      const idx = child.parentElement.children.indexOf(child);
      if (idx !== -1) child.parentElement.children.splice(idx, 1);
    }
    child.parentElement = this;
    this.children.push(child);
    return child;
  }
  remove() {
    if (this.parentElement) {
      const idx = this.parentElement.children.indexOf(this);
      if (idx !== -1) this.parentElement.children.splice(idx, 1);
      this.parentElement = null;
    }
  }
  get lastElementChild() {
    return this.children.length > 0 ? this.children[this.children.length - 1] : null;
  }
  get nextElementSibling() {
    if (!this.parentElement) return null;
    const idx = this.parentElement.children.indexOf(this);
    return (idx >= 0 && idx + 1 < this.parentElement.children.length) ? this.parentElement.children[idx + 1] : null;
  }
  get previousElementSibling() {
    if (!this.parentElement) return null;
    const idx = this.parentElement.children.indexOf(this);
    return idx > 0 ? this.parentElement.children[idx - 1] : null;
  }
  addEventListener(ev, fn) {
    this.listeners[ev] = this.listeners[ev] || [];
    this.listeners[ev].push(fn);
  }
  removeEventListener(ev, fn) {
    if (!this.listeners[ev]) return;
    this.listeners[ev] = this.listeners[ev].filter(f => f !== fn);
  }
  contains(target) {
    let cur = target;
    while (cur) {
      if (cur === this) return true;
      cur = cur.parentElement;
    }
    return false;
  }
  matches(sel) {
    sel = sel.trim();
    if (!sel) return false;
    if (sel.includes(",")) {
      return sel.split(",").some(s => this.matches(s));
    }
    if (sel.startsWith("#")) return this.id === sel.slice(1);
    if (/^[a-zA-Z]+$/.test(sel)) return this.tagName === sel.toUpperCase();
    if (sel === ".flex-1.flex.flex-col.min-w-0.h-full") {
      return this.classList.contains("flex-1") && this.classList.contains("flex") && this.classList.contains("flex-col") && this.classList.contains("min-w-0") && this.classList.contains("h-full");
    }
    if (sel === ".bg-sidebar") return this.classList.contains("bg-sidebar");
    if (sel === ".shrink-0") return this.classList.contains("shrink-0");
    if (sel === ".swiss-left-nav-tab") return this.classList.contains("swiss-left-nav-tab");
    if (sel === ".swiss-main-stage-tab") return this.classList.contains("swiss-main-stage-tab");
    if (sel === ".swiss-scope-item") return this.classList.contains("swiss-scope-item");
    if (sel === ".swiss-main-stage-tab.active") return this.classList.contains("swiss-main-stage-tab") && this.classList.contains("active");
    const attrMatch = sel.match(/^(\[[^\]]+\])+$/);
    if (attrMatch) {
      const parts = sel.match(/\[[^\]]+\]/g) || [];
      return parts.every(p => {
        const m = p.match(/^\[([a-zA-Z0-9_-]+)(?:(\^=|=)"([^"]*)")?\]$/);
        if (!m) return false;
        const val = this.getAttribute(m[1]);
        if (!m[2]) return val !== null;
        if (m[2] === "=") return val === m[3];
        if (m[2] === "^=") return typeof val === "string" && val.startsWith(m[3]);
        return false;
      });
    }
    const tagAttr = sel.match(/^([a-zA-Z]+)\[([a-zA-Z0-9_-]+)(\^=|=|\*=)"([^"]*)"\]$/);
    if (tagAttr) {
      if (this.tagName !== tagAttr[1].toUpperCase()) return false;
      const val = this.getAttribute(tagAttr[2]);
      if (typeof val !== "string") return false;
      if (tagAttr[3] === "=") return val === tagAttr[4];
      if (tagAttr[3] === "^=") return val.startsWith(tagAttr[4]);
      if (tagAttr[3] === "*=") return val.includes(tagAttr[4]);
    }
    return false;
  }
  closest(sel) {
    let cur = this;
    while (cur) {
      if (cur.matches && cur.matches(sel)) return cur;
      cur = cur.parentElement;
    }
    return null;
  }
  querySelectorAll(sel) {
    const out = [];
    const walk = (node) => {
      for (const ch of node.children) {
        if (ch.matches(sel)) out.push(ch);
        walk(ch);
      }
    };
    walk(this);
    return out;
  }
  querySelector(sel) {
    return this.querySelectorAll(sel)[0] || null;
  }
}

// Build live Antigravity 2.0 DOM structure:
// body -> [topMenuBar (.bg-sidebar.w-full), mainRow -> [leftSidebar (.bg-sidebar.flex-col), centerCol (.flex-1.flex.flex-col.min-w-0.h-full)]]
const docBody = new MockElement("body");
const docHead = new MockElement("head");

const topMenuBar = new MockElement("div");
topMenuBar.className = "flex items-center gap-1 px-2 h-full bg-sidebar w-full";
docBody.appendChild(topMenuBar);

const mainRow = new MockElement("div");
mainRow.className = "flex-1 flex min-h-0";
docBody.appendChild(mainRow);

const leftSidebar = new MockElement("div");
leftSidebar.className = "h-full w-full flex flex-col pb-2 bg-sidebar";
mainRow.appendChild(leftSidebar);

const navButtonsWrap = new MockElement("div");
const automationsBtn = new MockElement("button");
automationsBtn.setAttribute("data-testid", "automations-button");
navButtonsWrap.appendChild(automationsBtn);
leftSidebar.appendChild(navButtonsWrap);

const projectGroup = new MockElement("div");
projectGroup.setAttribute("data-swiss-project", "CascadeProject");
const convRow = new MockElement("div");
convRow.setAttribute("data-testid", "conversation-row-sidebar");
convRow.setAttribute("data-cascade-id", "cascade-uuid-999");
const convLink = new MockElement("a");
convLink.setAttribute("href", "/c/cascade-uuid-999");
convRow.appendChild(convLink);
projectGroup.appendChild(convRow);
const projectCard = new MockElement("div");
projectCard.setAttribute("data-project-card", "true");
projectCard.setAttribute("data-swiss-project", "CascadeProject");
projectCard.setAttribute("data-selected", "true");
projectCard.classList.add("active");
projectGroup.appendChild(projectCard);
leftSidebar.appendChild(projectGroup);

const knownProjMarker = new MockElement("div");
knownProjMarker.setAttribute("data-swiss-project", "Obsidian-HomePage");
leftSidebar.appendChild(knownProjMarker);

const convSectionHeaderWrap = new MockElement("div");
convSectionHeaderWrap.setAttribute("data-index", "41");
const convSectionHeader = new MockElement("div");
convSectionHeader.setAttribute("data-testid", "section-header");
convSectionHeader.setAttribute("data-title", "Conversations");
convSectionHeaderWrap.appendChild(convSectionHeader);
leftSidebar.appendChild(convSectionHeaderWrap);

const recentConvWrap = new MockElement("div");
recentConvWrap.setAttribute("data-index", "42");
const recentConvRow = new MockElement("div");
recentConvRow.setAttribute("data-testid", "conversation-row-sidebar");
recentConvRow.setAttribute("data-cascade-id", "recent-uuid-777");
recentConvRow.setAttribute("data-subtext", "SubtextProject");
recentConvWrap.appendChild(recentConvRow);
leftSidebar.appendChild(recentConvWrap);

const centerCol = new MockElement("div");
centerCol.className = "flex-1 flex flex-col min-w-0 h-full";
mainRow.appendChild(centerCol);

const breadcrumbBar = new MockElement("div");
breadcrumbBar.className = "shrink-0";
breadcrumbBar.style.height = "40px";
const bcSeg1 = new MockElement("span");
bcSeg1.setAttribute("data-testid", "breadcrumb-segment");
bcSeg1.textContent = "Obsidian-HomePage";
const bcSeg2 = new MockElement("span");
bcSeg2.setAttribute("data-testid", "breadcrumb-segment");
bcSeg2.textContent = "Academic Catch-Up";
breadcrumbBar.appendChild(bcSeg1);
breadcrumbBar.appendChild(bcSeg2);
centerCol.appendChild(breadcrumbBar);

const subShrink = new MockElement("div");
subShrink.className = "shrink-0";
centerCol.appendChild(subShrink);

const convoWrapper = new MockElement("div");
convoWrapper.className = "flex-1 min-h-0";
const convoView = new MockElement("div");
convoView.setAttribute("data-testid", "conversation-view");
convoWrapper.appendChild(convoView);
centerCol.appendChild(convoWrapper);

const storage = {};
global.localStorage = {
  getItem: (k) => Object.prototype.hasOwnProperty.call(storage, k) ? storage[k] : null,
  setItem: (k, v) => { storage[k] = String(v); }
};
global.location = { pathname: "/c/virtualized-uuid-111", href: "" };
const winListeners = {};
global.addEventListener = (ev, fn) => {
  winListeners[ev] = winListeners[ev] || [];
  winListeners[ev].push(fn);
};
global.removeEventListener = (ev, fn) => {
  if (!winListeners[ev]) return;
  winListeners[ev] = winListeners[ev].filter(f => f !== fn);
};
global.dispatchEvent = (e) => {
  if (e && e.type && winListeners[e.type]) {
    winListeners[e.type].forEach(fn => fn(e));
  }
};
global.CustomEvent = class CustomEvent {
  constructor(type, init = {}) {
    this.type = type;
    this.detail = init.detail;
  }
};
global.window = global;
global.document = {
  body: docBody,
  head: docHead,
  title: "Academic Catch-Up - FallbackTitleProj - Antigravity",
  createElement: (tag) => new MockElement(tag),
  getElementById: (id) => docBody.querySelector("#" + id) || docHead.querySelector("#" + id),
  querySelector: (sel) => docBody.querySelector(sel) || docHead.querySelector(sel),
  querySelectorAll: (sel) => [...docHead.querySelectorAll(sel), ...docBody.querySelectorAll(sel)],
  addEventListener: () => {},
  removeEventListener: () => {}
};
global.fetch = async () => ({
  ok: true,
  json: async () => ({ success: true, projects: ["Obsidian-HomePage", "CascadeProject", "SubtextProject"], issues: [], prs: [], tasks: [] })
});

// 1. Run script first time
` + js + `

assert.strictEqual(window.__swissGitHubExtInitialized, true, "script should initialize");
assert.ok(window.__swissGHNavInterval, "nav interval should be stored on window");
assert.ok(window.__swissGHFetchInterval, "fetch interval should be stored on window");

// Verify left sidebar got the click listener, NOT topMenuBar
assert.strictEqual(Boolean(topMenuBar.__swissSidebarClickBound), false, "topMenuBar (.bg-sidebar.w-full) must NOT be treated as left sidebar");
assert.strictEqual(Boolean(leftSidebar.__swissSidebarClickBound), true, "leftSidebar must be detected by getLeftSidebar()");

// Verify setupLeftNavTabs() default single mode
const leftNavGroup = document.getElementById("swiss-left-nav-group");
assert.ok(leftNavGroup, "swiss-left-nav-group should be created");
const singleBtn = leftNavGroup.querySelector('[data-swiss-ext="swiss-knife"]');
assert.ok(singleBtn, "swiss-knife button should exist in single mode");
assert.ok(singleBtn.innerHTML.includes('width="16" height="16"'), "single button should have 16px icon matching factory buttons");
assert.ok(singleBtn.innerHTML.includes("text-sm"), "single button should have text-sm font-size");
assert.ok(singleBtn.className.includes("h-8"), "single button should match factory 32px height");

// Verify clicking swiss-knife button toggles Main Stage
assert.ok(typeof singleBtn.onclick === "function", "singleBtn should have onclick handler");
singleBtn.onclick({ stopPropagation: () => {} });
const stContainerInit = document.getElementById("swiss-main-stage-container");
assert.ok(stContainerInit && stContainerInit.style.display === "flex", "clicking swiss-knife button should open main stage");
singleBtn.onclick({ stopPropagation: () => {} });
assert.ok(stContainerInit && stContainerInit.style.display === "none", "clicking swiss-knife button again should close main stage");

// Verify switching to individual mode
storage["antigravity_swiss_left_panel_mode"] = "individual";
window.setupLeftNavTabs();
const indGroup = document.getElementById("swiss-left-nav-group");
assert.ok(indGroup, "swiss-left-nav-group should exist in individual mode");
const browserBtn = indGroup.querySelector('[data-swiss-ext="browser"]');
const filesBtn = indGroup.querySelector('[data-swiss-ext="files"]');
const memosBtn = indGroup.querySelector('[data-swiss-ext="memos"]');
const ghBtn = indGroup.querySelector('[data-swiss-ext="github"]');
assert.ok(browserBtn && filesBtn && memosBtn && ghBtn, "all 4 individual extension buttons should exist in individual mode");
assert.ok(browserBtn.innerHTML.includes('width="16" height="16"'), "individual button should have 16px icon");
assert.ok(browserBtn.innerHTML.includes("text-sm"), "individual button should have text-sm font-size");
assert.ok(browserBtn.innerHTML.includes("Preview Browser"), "individual button should have Preview Browser text");

// Verify disabling left panel extensions removes group
storage["antigravity_swiss_left_panel_enabled"] = "false";
window.setupLeftNavTabs();
assert.strictEqual(document.getElementById("swiss-left-nav-group"), null, "swiss-left-nav-group should be removed when disabled");
storage["antigravity_swiss_left_panel_enabled"] = "true";
storage["antigravity_swiss_left_panel_mode"] = "single";
window.setupLeftNavTabs();

// Verify single mode works when window.__SWISS_ENH_CONFIG__.extensions is present
window.__SWISS_ENH_CONFIG__ = {
  left_panel_extensions_mode: "single",
  extensions: {
    browser: { aux_panel: true, main_page: false },
    files: { aux_panel: true, main_page: false },
    memos: { aux_panel: true, main_page: false },
    github: { aux_panel: true, main_page: false },
  }
};
storage["antigravity_swiss_left_panel_mode"] = "single";
window.setupLeftNavTabs();
const singleGrpConfig = document.getElementById("swiss-left-nav-group");
assert.ok(singleGrpConfig, "swiss-left-nav-group should exist with __SWISS_ENH_CONFIG__.extensions in single mode");
assert.ok(singleGrpConfig.querySelector('[data-swiss-ext="swiss-knife"]'), "single swiss-knife button must exist even when extensions map is present");

// Verify switching to individual mode via event when extensions map is present
window.__SWISS_ENH_CONFIG__.extensions.github.main_page = true;
window.dispatchEvent(new CustomEvent("swiss-left-nav-config-updated", {
  detail: { mode: "individual" }
}));
const indGrpEvent = document.getElementById("swiss-left-nav-group");
assert.ok(indGrpEvent, "swiss-left-nav-group should update to individual mode via event");
assert.strictEqual(indGrpEvent.querySelector('[data-swiss-ext="swiss-knife"]'), null, "swiss-knife button must not exist in individual mode");
assert.ok(indGrpEvent.querySelector('[data-swiss-ext="github"]'), "github button must exist when main_page is true");
assert.strictEqual(indGrpEvent.querySelector('[data-swiss-ext="browser"]'), null, "browser button must not exist when main_page is false");

// Switch back to single mode via event
window.dispatchEvent(new CustomEvent("swiss-left-nav-config-updated", {
  detail: { mode: "single" }
}));
const singleGrpRestored = document.getElementById("swiss-left-nav-group");
assert.ok(singleGrpRestored.querySelector('[data-swiss-ext="swiss-knife"]'), "swiss-knife button must be restored when switching back to single mode");


// Verify breadcrumb project detection when conversation row is virtualized out of DOM
assert.strictEqual(storage["antigravity_swiss_last_project"], "Obsidian-HomePage", "should detect project from first breadcrumb-segment");

// Verify data-cascade-id project detection when URL matches cascade row
bcSeg1.textContent = "";
global.location.pathname = "/c/cascade-uuid-999";
window.openSwissMainStage("memos");
assert.strictEqual(storage["antigravity_swiss_last_project"], "CascadeProject", "should detect project via data-cascade-id row");

// Verify data-subtext project detection in "Conversations" sidebar section (must not cross section-header into Projects)
global.location.pathname = "/c/recent-uuid-777";
window.openSwissMainStage("memos");
assert.strictEqual(storage["antigravity_swiss_last_project"], "SubtextProject", "should detect project from data-subtext in Conversations section");

// Verify openSwissMainStage("memos") hides native breadcrumb bar and convoView
assert.strictEqual(breadcrumbBar.style.display, "none", "breadcrumbBar should be hidden when Main Stage is open");
assert.strictEqual(subShrink.style.display, "none", "centerCol shrink-0 child should be hidden when Main Stage is open");
assert.strictEqual(convoView.style.display, "none", "convoView should be hidden when Main Stage is open");
const stageContainer = document.getElementById("swiss-main-stage-container");
assert.ok(stageContainer, "stage container should exist");
assert.strictEqual(stageContainer.style.display, "flex", "stage container should be flex");
assert.ok(document.getElementById("swiss-main-stage-header"), "stage header should exist");
assert.ok(document.getElementById("swiss-stage-scope-btn"), "scope picker button should exist");
const splitBtnEl = document.getElementById("swiss-stage-native-split-btn");
assert.ok(splitBtnEl, "split button should exist in stage header");
assert.ok(splitBtnEl.listeners.click && splitBtnEl.listeners.click.length > 0, "split button should have click listener");
const closeBtnEl = document.getElementById("swiss-stage-close-btn");
assert.ok(closeBtnEl, "close button should exist in stage header");
assert.ok(closeBtnEl.listeners.click && closeBtnEl.listeners.click.length > 0, "close button should have click listener");

// Verify clicking close button triggers closeMainStage() and restores native elements
closeBtnEl.listeners.click[0]();
assert.strictEqual(stageContainer.style.display, "none", "clicking close button should hide stage container");
assert.strictEqual(breadcrumbBar.style.display, "", "breadcrumbBar display should be restored on clicking close button");
assert.strictEqual(subShrink.style.display, "", "subShrink display should be restored on clicking close button");
assert.strictEqual(convoView.style.display, "", "convoView display should be restored on clicking close button");

// Re-open stage to verify programmatic closeMainStage()
window.openSwissMainStage("memos");
assert.strictEqual(stageContainer.style.display, "flex", "stage container should reopen");
window.closeMainStage();
assert.strictEqual(breadcrumbBar.style.display, "", "breadcrumbBar display should be restored on closeMainStage");
assert.strictEqual(subShrink.style.display, "", "subShrink display should be restored on closeMainStage");
assert.strictEqual(convoView.style.display, "", "convoView display should be restored on closeMainStage");
assert.strictEqual(stageContainer.style.display, "none", "stage container should be hidden on closeMainStage");

// Verify active state isolation: native automationsBtn and convRow suppressed when Main Stage is open
automationsBtn.classList.add("active");
automationsBtn.setAttribute("data-state", "active");
automationsBtn.setAttribute("aria-selected", "true");
convRow.setAttribute("data-selected", "true");
convRow.setAttribute("data-swiss-project", "CascadeProject");
convRow.classList.add("active");

window.openSwissMainStage("memos");
assert.strictEqual(docBody.getAttribute("data-swiss-stage-active"), "memos", "docBody should have data-swiss-stage-active='memos'");
assert.strictEqual(leftSidebar.getAttribute("data-swiss-stage-active"), "memos", "leftSidebar should have data-swiss-stage-active='memos'");
assert.strictEqual(automationsBtn.classList.contains("active"), false, "automations button must not have active class while Main Stage is open");
assert.strictEqual(automationsBtn.getAttribute("aria-selected"), "false", "automations button aria-selected must be false while Main Stage is open");
assert.strictEqual(convRow.getAttribute("data-selected"), "false", "conversation row data-selected must be false while Main Stage is open");
assert.strictEqual(convRow.classList.contains("active"), false, "conversation row must not have active class while Main Stage is open");
assert.strictEqual(projectCard.getAttribute("data-selected"), "true", "project card must NOT be suppressed when Main Stage is open");
assert.strictEqual(projectCard.classList.contains("active"), true, "project card active class must NOT be suppressed when Main Stage is open");

window.closeMainStage();
assert.strictEqual(docBody.getAttribute("data-swiss-stage-active"), null, "data-swiss-stage-active should be removed on closeMainStage");
assert.strictEqual(leftSidebar.getAttribute("data-swiss-stage-active"), null, "leftSidebar data-swiss-stage-active should be removed on closeMainStage");
assert.strictEqual(automationsBtn.classList.contains("active"), true, "automations button active class should be restored on closeMainStage");
assert.strictEqual(automationsBtn.getAttribute("data-state"), "active", "automations button data-state should be restored on closeMainStage");
assert.strictEqual(automationsBtn.getAttribute("aria-selected"), "true", "automations button aria-selected should be restored on closeMainStage");
assert.strictEqual(convRow.getAttribute("data-selected"), "true", "conversation row data-selected should be restored on closeMainStage");
assert.strictEqual(convRow.classList.contains("active"), true, "conversation row active class should be restored on closeMainStage");

// Verify landing page (no conversation-view) -> conversation reparenting restores convoWrapper cleanly
convoView.setAttribute("data-testid", "conversation-view-absent");
window.openSwissMainStage("memos");
assert.strictEqual(stageContainer.parentElement, centerCol, "stageContainer should mount in centerCol when conversation-view is absent");
assert.strictEqual(convoWrapper.style.display, "none", "convoWrapper should be hidden on landing page when Main Stage is open");
convoView.setAttribute("data-testid", "conversation-view");
window.openSwissMainStage("github");
assert.strictEqual(stageContainer.parentElement, convoWrapper, "stageContainer should reparent into convoView.parentElement when conversation-view mounts");
assert.strictEqual(convoWrapper.style.display, "flex", "convoWrapper must not remain display:none after reparenting stageContainer into it");
assert.strictEqual(convoView.style.display, "none", "convoView should be hidden while Main Stage is open");
window.closeMainStage();
assert.strictEqual(convoWrapper.style.display, "", "convoWrapper display must be restored to empty string on closeMainStage");
assert.strictEqual(convoView.style.display, "", "convoView display must be restored on closeMainStage");

// Verify clicking a header tab updates __swissActiveMainStageExt and survives re-injection
const firstNavInterval = window.__swissGHNavInterval;
window.openSwissMainStage("github");
const memosTab = Array.from(document.querySelectorAll(".swiss-main-stage-tab")).find(t => t.getAttribute("data-ext") === "memos");
assert.ok(memosTab && memosTab.listeners.click && memosTab.listeners.click[0], "memos header tab should have click listener");
memosTab.listeners.click[0]({ currentTarget: memosTab });
assert.strictEqual(window.__swissActiveMainStageExt, "memos", "clicking header tab should update __swissActiveMainStageExt");
` + js + `
assert.notStrictEqual(window.__swissGHNavInterval, firstNavInterval, "re-injection should clear and replace __swissGHNavInterval");
assert.strictEqual(window.__swissActiveMainStageExt, "memos", "re-injection should preserve active tab switched via header");

// Verify daemon offline hides left nav group and closes main stage
let auxDaemonChangedArg = null;
window.__swissOnAuxDaemonChanged = (online) => { auxDaemonChangedArg = online; };
window.openSwissMainStage("github");
assert.ok(document.getElementById("swiss-main-stage-container").style.display === "flex", "main stage should be open");
assert.ok(document.getElementById("swiss-left-nav-group"), "left nav group should exist before offline");

window.setSwissDaemonOnline(false);
assert.strictEqual(document.getElementById("swiss-left-nav-group"), null, "swiss-left-nav-group must be removed when daemon is offline");
assert.strictEqual(document.getElementById("swiss-main-stage-container").style.display, "none", "main stage must be closed when daemon is offline");
assert.strictEqual(auxDaemonChangedArg, false, "aux daemon changed notification must be sent with false");

window.setupLeftNavTabs();
assert.strictEqual(document.getElementById("swiss-left-nav-group"), null, "setupLeftNavTabs must not recreate left nav group when daemon is offline");

window.setSwissDaemonOnline(true);
assert.ok(document.getElementById("swiss-left-nav-group"), "swiss-left-nav-group must be restored when daemon comes back online");
assert.strictEqual(auxDaemonChangedArg, true, "aux daemon changed notification must be sent with true");

clearInterval(window.__swissGHNavInterval);
clearInterval(window.__swissGHFetchInterval);
if (window.__swissGHDaemonCheckInterval) clearInterval(window.__swissGHDaemonCheckInterval);
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(harness)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("DOM runtime test failed: %v\n%s", err, string(out))
	}
}

func TestGitHubExtensionLeftNavProportionsAndBreakerRule(t *testing.T) {
	css := GenerateGitHubExtensionCSS()

	// 1. Left nav tab button proportions matching factory buttons
	if !strings.Contains(css, ".swiss-left-nav-tab") {
		t.Errorf("expected CSS to define .swiss-left-nav-tab")
	}

	// 2. Left tabs separator and nav group gap
	if !strings.Contains(css, ".swiss-left-tabs-separator") {
		t.Errorf("expected CSS to define .swiss-left-tabs-separator")
	}
	if !strings.Contains(css, "height: 1px;") {
		t.Errorf("expected separator to be 1px height")
	}
	if !strings.Contains(css, "margin: 0 4px;") {
		t.Errorf("expected separator margin to be 0 4px for harmonious transition spacing, got: %s", css)
	}
	if !strings.Contains(css, "gap: 6px;") {
		t.Errorf("expected .swiss-left-nav-group to have gap: 6px matching native navContainer gap")
	}

	// 3. Script matches factory proportions (16px icons, text-sm font-size)
	js := GenerateGitHubExtensionScript()
	if !strings.Contains(js, `width="16" height="16"`) {
		t.Errorf("expected left nav script to use 16px icons matching factory buttons")
	}
	if !strings.Contains(js, "text-sm") {
		t.Errorf("expected left nav script to use text-sm matching factory buttons")
	}

	// 4. David-Design Zero Decorative Emojis check in github_extension script
	forbiddenEmojis := []string{
		"🎙️", "💬", "💾", "✏️", "📋", "🗑️", "📂", "⚡", "📁", "📜", "📝", "📕", "📄", "⏹️",
	}
	for _, emoji := range forbiddenEmojis {
		if strings.Contains(js, emoji) {
			t.Errorf("github extension script contains forbidden decorative emoji %q", emoji)
		}
	}
}

func TestProjectCardVisibilityUnderSwissStageActive(t *testing.T) {
	css := GenerateGitHubExtensionCSS()

	// R1: Must NOT suppress project cards or project label badges under [data-swiss-stage-active]
	if strings.Contains(css, "[data-swiss-stage-active] [data-project-card],") ||
		strings.Contains(css, "[data-swiss-stage-active] [data-project-card] {") ||
		strings.Contains(css, "[data-swiss-stage-active] [data-swiss-project],") ||
		strings.Contains(css, "[data-swiss-stage-active] [data-swiss-project] {") {
		t.Errorf("GenerateGitHubExtensionCSS() should NOT contain direct [data-swiss-stage-active] [data-project-card] or [data-swiss-project] suppression rule")
	}

	// Must exclude project cards from generic button suppression
	if !strings.Contains(css, ":not([data-project-card]):not([data-swiss-project])") {
		t.Errorf("GenerateGitHubExtensionCSS() missing :not([data-project-card]):not([data-swiss-project]) exclusion")
	}

	js := GenerateGitHubExtensionScript()
	// Script suppression loop must explicitly skip project cards and headers, including their descendants
	if !strings.Contains(js, "row.hasAttribute('data-project-card')") || !strings.Contains(js, "row.hasAttribute('data-swiss-project')") {
		t.Errorf("GenerateGitHubExtensionScript() active stage suppression must preserve data-project-card and data-swiss-project")
	}
	if !strings.Contains(js, "row.closest?.('[data-project-card]')") || !strings.Contains(js, "row.closest?.('.group\\\\/header')") {
		t.Errorf("GenerateGitHubExtensionScript() active stage suppression must preserve descendants of project cards and headers via closest()")
	}
	if !strings.Contains(js, "!isConvRow") {
		t.Errorf("GenerateGitHubExtensionScript() active stage suppression must distinguish conversation rows from project headers via !isConvRow")
	}
}

func TestNewConversationButtonNormalization(t *testing.T) {
	css := GenerateGitHubExtensionCSS()

	// R2: Must normalize unselected + New Conversation button
	requiredPatterns := []string{
		`[data-testid="new-conversation-button"][data-selected="false"]`,
		`[data-testid="new-conversation-button"].unselected`,
		`body:has([data-testid="conversation-view"]) [data-testid="new-conversation-button"]`,
		`background-color: transparent !important;`,
		`border: none !important;`,
	}
	for _, p := range requiredPatterns {
		if !strings.Contains(css, p) {
			t.Errorf("GenerateGitHubExtensionCSS() missing new conversation normalization rule %q", p)
		}
	}

	js := GenerateGitHubExtensionScript()
	if !strings.Contains(js, "normalizeNewConversationButton") {
		t.Errorf("GenerateGitHubExtensionScript() missing normalizeNewConversationButton()")
	}
	if !strings.Contains(js, "btn.classList.remove('active')") {
		t.Errorf("GenerateGitHubExtensionScript() missing removal of active class on unselected new conversation button")
	}
}

func TestSwissKnifeIconIsPocketKnifeNotWrench(t *testing.T) {
	js := GenerateGitHubExtensionScript()

	// Must contain authentic Swiss Pocket Knife SVG path
	pocketKnifePath := "M20.83 8.83a4 4 0 0 0-5.66-5.66l-12 12a4 4 0 1 0 5.66 5.66Z"
	if !strings.Contains(js, pocketKnifePath) {
		t.Errorf("GenerateGitHubExtensionScript() missing Swiss Pocket Knife SVG path: %q", pocketKnifePath)
	}

	// Must NOT contain old wrench SVG path
	wrenchPath := "M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6"
	if strings.Contains(js, wrenchPath) {
		t.Errorf("GenerateGitHubExtensionScript() still contains old wrench SVG path: %q", wrenchPath)
	}
}

func TestCreateActionsCompatibility(t *testing.T) {
	js := GenerateGitHubExtensionScript()

	// Must trigger closeMainStage on all native Antigravity creation triggers
	createTriggers := []string{
		`[data-testid="app-icon-new-conversation-button"]`,
		`[data-testid="sidebar-add-project-button"]`,
		`a[aria-label="New Conversation in Project"]`,
		`a[href*="/customizations"]`,
		`[data-testid="settings-button"]`,
	}
	for _, trigger := range createTriggers {
		if !strings.Contains(js, trigger) {
			t.Errorf("GenerateGitHubExtensionScript() missing create trigger in convTrigger: %q", trigger)
		}
	}

	// Must hook history.pushState and history.replaceState so client-side navigation closes stage
	historyHooks := []string{
		`__swissOrigPushState`,
		`origPushState`,
		`origReplaceState`,
	}
	for _, hook := range historyHooks {
		if !strings.Contains(js, hook) {
			t.Errorf("GenerateGitHubExtensionScript() missing history navigation hook: %q", hook)
		}
	}

	// Must listen for keydown events to exit stage on Escape and Ctrl+N / Cmd+N
	keydownHooks := []string{
		`__swissDocNavKeydownHandler`,
		`e.key === "Escape"`,
		`e.key === "n" || e.key === "N"`,
	}
	for _, hook := range keydownHooks {
		if !strings.Contains(js, hook) {
			t.Errorf("GenerateGitHubExtensionScript() missing keydown hook: %q", hook)
		}
	}

	css := GenerateGitHubExtensionCSS()
	hoverRules := []string{
		`[data-swiss-stage-active] [data-testid="app-icon-new-conversation-button"]:hover`,
		`[data-swiss-stage-active] [data-testid="sidebar-add-project-button"]:hover`,
		`[data-swiss-stage-active] a[aria-label="New Conversation in Project"]:hover`,
	}
	for _, rule := range hoverRules {
		if !strings.Contains(css, rule) {
			t.Errorf("GenerateGitHubExtensionCSS() missing hover rule for create action: %q", rule)
		}
	}
}

func TestGitHubExtensionScriptPerExtensionMainPage(t *testing.T) {
	js := GenerateGitHubExtensionScript()

	tokens := []string{
		"swissExtVisibility",                    // extensions map handle in setupLeftNavTabs
		"main_page",                             // per-extension main-page visibility key
		"__SWISS_ENH_CONFIG__.extensions",       // reads EnhancementsConfig.extensions
		"isMainSectionEnabled",                  // global main-section floor stays
		"isLeftPanelEnabled",                    // global left-panel floor stays
	}
	for _, tok := range tokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected GitHub extension script to contain %q", tok)
		}
	}

	// The extensions map must gate left-nav entries by main_page === true
	if !strings.Contains(js, "vis.main_page === true") {
		t.Errorf("expected left-nav extension list to filter by main_page === true")
	}
}
