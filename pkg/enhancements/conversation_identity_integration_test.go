package enhancements

import (
	"os/exec"
	"strings"
	"testing"
)

func identityCallChainScript(t *testing.T) string {
	t.Helper()
	script := GenerateEnhancementsScript(DefaultConfig())
	const startMarker = "/* __SWISS_CONVERSATION_IDENTITY_START__ */"
	start := strings.Index(script, startMarker)
	if start < 0 {
		t.Fatalf("identity call-chain start marker missing")
	}
	renderStart := strings.Index(script[start:], "function renderContextWindowProgressRing()")
	if renderStart < 0 {
		t.Fatalf("renderContextWindowProgressRing missing from generated script")
	}
	renderStart += start
	renderCall := strings.Index(script[renderStart:], "\n    renderContextWindowProgressRing();")
	if renderCall < 0 {
		t.Fatalf("could not find the end of renderContextWindowProgressRing")
	}
	return script[start : renderStart+renderCall]
}

func runIdentityCallChain(t *testing.T, scenario string) {
	t.Helper()
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not available: %v", err)
	}
	driver := identityCallChainHarness + "\n" + identityCallChainScript(t) + "\n" + scenario
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("integrated identity harness failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "INTEGRATED_IDENTITY_OK") {
		t.Fatalf("integrated harness did not reach its assertion marker:\n%s", string(out))
	}
}

const identityCallChainHarness = `
const assert = require("assert");

class NodeEl {
  constructor(tagName, testId) {
    this.tagName = String(tagName || "div").toUpperCase();
    this.id = "";
    this.className = "";
    this.attributes = {};
    this.style = {};
    this.children = [];
    this.parentElement = null;
    this.parentNode = null;
    this.nextSibling = null;
    this.previousElementSibling = null;
    this.innerText = "";
    this.textContent = "";
    this.innerHTML = "";
    this.value = "";
    this.href = "";
    this._classes = new Set();
    const self = this;
    this.classList = {
      add: (name) => self._classes.add(name),
      remove: (name) => self._classes.delete(name),
      contains: (name) => self._classes.has(name),
      has: (name) => self._classes.has(name)
    };
    if (testId) this.setAttribute("data-testid", testId);
  }
  setAttribute(name, value) {
    this.attributes[name] = String(value);
    if (name === "id") this.id = String(value);
    if (name === "class") {
      this.className = String(value);
      for (const part of this.className.split(/\s+/)) if (part) this._classes.add(part);
    }
  }
  getAttribute(name) { return this.attributes[name] === undefined ? null : this.attributes[name]; }
  removeAttribute(name) { delete this.attributes[name]; }
  appendChild(child) {
    if (child.parentElement) child.parentElement.removeChild(child);
    child.parentElement = this;
    child.parentNode = this;
    const previous = this.children[this.children.length - 1] || null;
    child.previousElementSibling = previous;
    child.nextSibling = null;
    if (previous) previous.nextSibling = child;
    this.children.push(child);
    return child;
  }
  insertBefore(child, reference) {
    if (child.parentElement) child.parentElement.removeChild(child);
    const index = reference ? this.children.indexOf(reference) : -1;
    if (index === -1) return this.appendChild(child);
    child.parentElement = this;
    child.parentNode = this;
    this.children.splice(index, 0, child);
    child.nextSibling = reference;
    child.previousElementSibling = this.children[index - 1] || null;
    reference.previousElementSibling = child;
    if (child.previousElementSibling) child.previousElementSibling.nextSibling = child;
    return child;
  }
  removeChild(child) {
    const index = this.children.indexOf(child);
    if (index < 0) return;
    this.children.splice(index, 1);
    child.parentElement = null;
    child.parentNode = null;
    for (let i = 0; i < this.children.length; i++) {
      this.children[i].previousElementSibling = this.children[i - 1] || null;
      this.children[i].nextSibling = this.children[i + 1] || null;
    }
  }
  remove() {
    if (this.parentElement) this.parentElement.removeChild(this);
  }
  matches(selector) {
    const testId = this.getAttribute("data-testid");
    if (testId && selector.includes('[data-testid="' + testId + '"]')) return true;
    if (selector.includes("model-selector-trigger") && testId === "model-selector-trigger") return true;
    if (selector.includes("agent-input-box") && testId === "agent-input-box") return true;
    if (selector.includes("conversation-view") && testId === "conversation-view") return true;
    if (selector.includes(".no-focus-agent-input") && this.classList.contains("no-focus-agent-input")) return true;
    if (selector.includes("swiss-context-window-ring-sub")) {
      return this.id.startsWith("swiss-context-window-ring-sub-") || this.classList.contains("swiss-context-window-ring-sub");
    }
    if (selector.includes("swiss-context-window-ring")) {
      return this.id === "swiss-context-window-ring" || this.classList.contains("swiss-context-window-ring");
    }
    if (this.tagName === "A" && this.href && selector.includes('a[href*="/c/"]')) return true;
    if (this.tagName === "A" && this.href && selector === "a[href]") return true;
    return selector === "*";
  }
  closest(selector) {
    let current = this;
    while (current) {
      if (current.matches(selector)) return current;
      current = current.parentElement;
    }
    return null;
  }
  querySelectorAll(selector) {
    const found = [];
    const scan = (node) => {
      for (const child of node.children) {
        if (child.matches(selector)) found.push(child);
        scan(child);
      }
    };
    scan(this);
    return found;
  }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
  addEventListener(type, fn) {
    if (!this._listeners) this._listeners = {};
    if (!this._listeners[type]) this._listeners[type] = [];
    this._listeners[type].push(fn);
  }
  dispatchEvent(event) {
    for (const fn of ((this._listeners && this._listeners[event.type]) || [])) fn(event);
  }
  click() { this.dispatchEvent({ type: "click", target: this, isTrusted: true }); }
  focus() {}
}

function makeStorage(seed) {
  const data = new Map(Object.entries(seed || {}));
  return {
    getItem: (key) => data.has(key) ? data.get(key) : null,
    setItem: (key, value) => data.set(key, String(value)),
    removeItem: (key) => data.delete(key),
    clear: () => data.clear()
  };
}

const documentListeners = {};
const windowListeners = {};
const head = new NodeEl("head");
const body = new NodeEl("body");
const document = {
  head,
  body,
  documentElement: { classList: { contains: () => true } },
  createElement: (tag) => new NodeEl(tag),
  getElementById: (id) => {
    const scan = (node) => {
      if (node.id === id) return node;
      for (const child of node.children) {
        const found = scan(child);
        if (found) return found;
      }
      return null;
    };
    return scan(head) || scan(body);
  },
  querySelector: (selector) => body.querySelector(selector),
  querySelectorAll: (selector) => body.querySelectorAll(selector),
  addEventListener: (type, fn) => {
    if (!documentListeners[type]) documentListeners[type] = [];
    documentListeners[type].push(fn);
  }
};
const location = {
  pathname: "/c/29d106f9-d43c-4cc5-9f9f-971c885d4743",
  search: "",
  hash: "",
  assign: () => {},
  get href() { return this.pathname + this.search + this.hash; }
};
const localStorage = makeStorage();
const sessionStorage = makeStorage();
const window = {
  document,
  location,
  localStorage,
  sessionStorage,
  innerWidth: 1280,
  innerHeight: 900,
  __swissDaemonOnline: true,
  addEventListener: (type, fn) => {
    if (!windowListeners[type]) windowListeners[type] = [];
    windowListeners[type].push(fn);
  },
  removeEventListener: () => {},
  dispatchEvent: () => true,
  postMessage: () => {},
  fetch: null
};

globalThis.window = window;
globalThis.document = document;
globalThis.localStorage = localStorage;
globalThis.sessionStorage = sessionStorage;
globalThis.fetch = (...args) => window.fetch(...args);
globalThis.innerWidth = 1280;
globalThis.innerHeight = 900;
globalThis.setInterval = () => 0;
globalThis.clearInterval = () => {};
globalThis.setTimeout = () => 0;
globalThis.clearTimeout = () => {};
globalThis.requestAnimationFrame = () => 0;
globalThis.cancelAnimationFrame = () => {};
globalThis.StorageEvent = class { constructor(type, init) { Object.assign(this, init || {}); this.type = type; } };
globalThis.InputEvent = class { constructor(type, init) { Object.assign(this, init || {}); this.type = type; } };
globalThis.KeyboardEvent = class { constructor(type, init) { Object.assign(this, init || {}); this.type = type; } };
globalThis.MouseEvent = class { constructor(type, init) { Object.assign(this, init || {}); this.type = type; } };
globalThis.PointerEvent = globalThis.MouseEvent;
globalThis.MutationObserver = class { observe() {} disconnect() {} };

function isContextWindowIndicatorEnabled() { return true; }
function isContextWindowClickToCompactEnabled() { return false; }
function getSwissAuthHeaders() { return {}; }
function renderPromptJumpBar() {}
function checkAndRecoverFromNonexistentConversation() {}
function applyEnhancementsStyles() {}
function flush() { return new Promise((resolve) => { let n = 0; const tick = () => { if (++n >= 20) resolve(); else Promise.resolve().then(tick); }; tick(); }); }
function deferred() { let resolve; const promise = new Promise((done) => { resolve = done; }); return { promise, resolve }; }
function emitDocument(type, target, trusted) {
  const event = { type, target, isTrusted: trusted === undefined ? true : trusted };
  for (const fn of (documentListeners[type] || [])) fn(event);
}
function emitWindow(type, trusted) {
  const event = { type, target: window, isTrusted: trusted };
  for (const fn of (windowListeners[type] || [])) fn(event);
}
`

func TestConversationIdentity_GeneratedMetricsCallChain(t *testing.T) {
	runIdentityCallChain(t, `
(async () => {
  const routeId = "29d106f9-d43c-4cc5-9f9f-971c885d4743";
  const emptyVisibleId = "281241ba-17ff-4904-8305-031db425f416";
  const currentVisibleId = "0f3f3b39-9fda-4b2c-a25f-033e47d491cb";
  const requests = [];
  const oldResponse = deferred();

  const view = new NodeEl("div", "conversation-view");
  const inputBox = new NodeEl("div", "agent-input-box");
  inputBox.className = "no-focus-agent-input";
  inputBox.classList.add("no-focus-agent-input");
  const trigger = new NodeEl("button", "model-selector-trigger");
  inputBox.appendChild(trigger);
  view.appendChild(inputBox);
  document.body.appendChild(view);

  function setVisible(id) {
    view.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
    inputBox.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
  }
  setVisible(emptyVisibleId);
  location.pathname = "/c/" + routeId;
  window.fetch = (url) => {
    if (url.includes("/api/tokens/chat-metrics")) {
      const parsed = new URL(url);
      const id = parsed.searchParams.get("conversation_id");
      requests.push(id);
      if (id === routeId) return oldResponse.promise;
      return Promise.resolve({ ok: true, json: async () => ({ success: true, user_prompt_count: 0, prompts: [], turns: [] }) });
    }
    return Promise.resolve({ ok: true, json: async () => ({ pending_intents: [] }) });
  };

  renderContextWindowProgressRing();
  let ring = document.getElementById("swiss-context-window-ring");
  assert(ring, "blank visible conversation must mount the context ring");
  await flush();
  ring = document.getElementById("swiss-context-window-ring");
  assert(ring, "the context ring must remain mounted after the blank metrics response");
  assert.strictEqual(requests[0], emptyVisibleId, "metrics must request the rendered empty conversation, not the stale route");
  assert(!requests.includes(routeId), "the stale route identity must not be queried while a rendered identity exists");
  assert(/unavailable/i.test(ring.getAttribute("title")), "blank visible conversation must render unavailable: " + ring.getAttribute("title"));
  assert(!ring.getAttribute("title").includes("222"), "the old route usage must never paint");

  setVisible(routeId);
  renderContextWindowProgressRing();
  await flush();
  assert(requests.includes(routeId), "the superseded request fixture must be in flight");

  setVisible(currentVisibleId);
  renderContextWindowProgressRing();
  await flush();
  ring = document.getElementById("swiss-context-window-ring");
  assert(/unavailable/i.test(ring.getAttribute("title")), "the current empty conversation must stay unavailable");

  oldResponse.resolve({
    ok: true,
    json: async () => ({
      conversation_id: routeId,
      success: true,
      user_prompt_count: 1,
      prompts: [{ index: 0, text: "old prompt" }],
      turns: [{ context_tokens: 222465, input_tokens: 222465, output_tokens: 0 }],
      context_window: { used_tokens: 222465, max_tokens: 256000 }
    })
  });
  await flush();
  ring = document.getElementById("swiss-context-window-ring");
  assert(/unavailable/i.test(ring.getAttribute("title")), "a stale response must not repaint the active conversation");
  assert(!ring.getAttribute("title").includes("222"), "a stale response must not mutate the active DOM");

  setVisible(routeId);
  renderContextWindowProgressRing();
  await flush();
  ring = document.getElementById("swiss-context-window-ring");
  assert(/unavailable/i.test(ring.getAttribute("title")), "the stale response must not populate the superseded identity cache");
  assert(!ring.getAttribute("title").includes("222"), "returning to the superseded route cannot resurrect stale metrics");
  console.log("INTEGRATED_IDENTITY_OK");
})().catch((error) => { console.error(error.stack || error); process.exit(1); });
`)
}

func TestConversationIdentity_GeneratedNavigationIntentCallChain(t *testing.T) {
	runIdentityCallChain(t, `
(async () => {
  let statusCalls = 0;
  window.fetch = async (url) => {
    if (url.includes("/api/conversations/status")) {
      statusCalls++;
      return { ok: true, json: async () => ({ pending_intents: [] }) };
    }
    return { ok: true, json: async () => ({}) };
  };

  const newChat = new NodeEl("button", "new-conversation-button");
  const appIcon = new NodeEl("button", "app-icon-new-conversation-button");
  const anchor = new NodeEl("a");
  anchor.href = "/c/281241ba-17ff-4904-8305-031db425f416?tab=chat";
  const sidebar = new NodeEl("div", "conversation-row-sidebar");
  anchor.appendChild(sidebar);
  document.body.appendChild(newChat);
  document.body.appendChild(appIcon);
  document.body.appendChild(anchor);

  emitDocument("click", newChat, true);
  let intent = window.__swissExplicitConversationNavigation;
  assert(intent, "the primary New Conversation control must install route intent");
  assert.strictEqual(intent.kind, "new_conversation", "primary New Conversation intent kind");
  assert(JSON.parse(sessionStorage.getItem("antigravity_swiss_explicit_navigation")).pending, "New Conversation intent must persist while binding");
  assert.strictEqual(loadExplicitConversationNavigationIntent().kind, "new_conversation", "persisted intent must load after a same-tab reload");

  intent.at = Date.now() - 31000;
  await window.__swissCheckRevival();
  await flush();
  assert.strictEqual(statusCalls, 0, "route intent must survive beyond the old 30-second expiry");

  location.pathname = "/";
  location.search = "?section=user-new";
  await window.__swissCheckRevival();
  intent = window.__swissExplicitConversationNavigation;
  assert.strictEqual(intent.pending, false, "New Conversation intent must bind to the resulting host route");
  assert.strictEqual(intent.route, "/?section=user-new", "the bound route must be the explicit current route");

  location.pathname = "/c/aaaaaaaa-1111-2222-3333-444444444444";
  location.search = "";
  assert.strictEqual(isExplicitConversationNavigationActive(window.__swissExplicitConversationNavigation, location), false, "a different current route must not be active");
  await window.__swissCheckRevival();
  await flush();
  assert.strictEqual(statusCalls, 1, "a later unrelated route must not stay fenced by the old intent");

  location.pathname = "/c/29d106f9-d43c-4cc5-9f9f-971c885d4743";
  emitDocument("click", appIcon, true);
  intent = window.__swissExplicitConversationNavigation;
  assert.strictEqual(intent.kind, "new_conversation", "the app-icon New Conversation control must install intent");

  emitDocument("click", sidebar, true);
  intent = window.__swissExplicitConversationNavigation;
  assert.strictEqual(intent.kind, "conversation", "sidebar click must replace stale new-chat intent");
  assert.strictEqual(intent.targetConversationId, "281241ba-17ff-4904-8305-031db425f416", "sidebar intent must target the selected conversation");

  location.pathname = "/c/0f3f3b39-9fda-4b2c-a25f-033e47d491cb";
  location.search = "?tab=chat";
  emitWindow("popstate", true);
  intent = window.__swissExplicitConversationNavigation;
  assert.strictEqual(intent.kind, "history", "trusted back/forward must install history intent");
  assert.strictEqual(intent.route, "/c/0f3f3b39-9fda-4b2c-a25f-033e47d491cb?tab=chat", "history intent must bind to the traversed route");

  location.pathname = "/c/bbbbbbbb-5555-6666-7777-888888888888";
  location.search = "";
  emitWindow("popstate", false);
  assert.strictEqual(window.__swissExplicitConversationNavigation.route, "/c/0f3f3b39-9fda-4b2c-a25f-033e47d491cb?tab=chat", "programmatic popstate must not be classified as user navigation");
  console.log("INTEGRATED_IDENTITY_OK");
})().catch((error) => { console.error(error.stack || error); process.exit(1); });
`)
}

func TestConversationIdentity_GeneratedRouteSynchronizationCallChain(t *testing.T) {
	runIdentityCallChain(t, `
(async () => {
  const staleRouteId = "29d106f9-d43c-4cc5-9f9f-971c885d4743";
  const routedViewId = "281241ba-17ff-4904-8305-031db425f416";
  const blankRouteId = "0f3f3b39-9fda-4b2c-a25f-033e47d491cb";
  const explicitRouteId = "aaaaaaaa-1111-2222-3333-444444444444";
  const explicitViewId = "bbbbbbbb-5555-6666-7777-888888888888";
  const fallbackRouteId = "cccccccc-9999-aaaa-bbbb-dddddddddddd";
  const fallbackViewId = "eeeeeeee-0000-1111-2222-ffffffffffff";
  const metricRequests = [];
  const routerCalls = [];
  const popstateEvents = [];

  window.fetch = async (url) => {
    if (String(url).includes("/api/tokens/chat-metrics")) {
      const parsed = new URL(url);
      metricRequests.push(parsed.searchParams.get("conversation_id"));
      return { ok: true, json: async () => ({ success: true, user_prompt_count: 0, prompts: [], turns: [] }) };
    }
    return { ok: true, json: async () => ({ pending_intents: [] }) };
  };
  window.__TSR_ROUTER__ = { navigate: ({ to }) => routerCalls.push(to) };
  window.dispatchEvent = (event) => { popstateEvents.push(event.type); return true; };

  const view = new NodeEl("div", "conversation-view");
  const inputBox = new NodeEl("div", "agent-input-box");
  inputBox.className = "no-focus-agent-input";
  inputBox.classList.add("no-focus-agent-input");
  const trigger = new NodeEl("button", "model-selector-trigger");
  inputBox.appendChild(trigger);
  view.appendChild(inputBox);
  document.body.appendChild(view);

  function setVisible(id) {
    view.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
    inputBox.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
  }

  location.pathname = "/c/" + blankRouteId;
  location.search = "";
  setVisible(null);
  cachedTokensByConv[blankRouteId] = 149000;
  cachedContextWindowByConv[blankRouteId] = { used_tokens: 149000, max_tokens: 256000 };
  renderContextWindowProgressRing();
  let ring = document.getElementById("swiss-context-window-ring");
  assert(ring, "the blank New Conversation must still mount an unavailable ring");
  assert.strictEqual(conversationMetricIdentity.conversationId, null, "a blank rendered view must not inherit the old route identity");
  assert.strictEqual(metricRequests.length, 0, "a blank rendered view must not query metrics for the old route");
  assert(/unavailable/i.test(ring.getAttribute("title")), "the blank rendered view must show unavailable metrics");
  assert(!/149K|58%/.test(ring.getAttribute("title")), "stale route metrics must never paint on a blank rendered view");

  location.pathname = "/c/" + staleRouteId;
  setVisible(routedViewId);
  renderContextWindowProgressRing();
  assert.deepStrictEqual(routerCalls, ["/c/" + routedViewId], "the rendered conversation must synchronize the stale route once");
  const stableRing = document.getElementById("swiss-context-window-ring");
  renderContextWindowProgressRing();
  renderContextWindowProgressRing();
  renderContextWindowProgressRing();
  assert.strictEqual(routerCalls.length, 1, "observer/render ticks must not loop the router synchronization");
  assert.strictEqual(document.getElementById("swiss-context-window-ring"), stableRing, "route synchronization must not remount the ring");

  location.pathname = "/c/" + explicitRouteId;
  setVisible(explicitRouteId);
  const newChat = {
    closest: (selector) => String(selector).includes("new-conversation-button") ? { setAttribute: () => {} } : null
  };
  window.__swissExplicitConversationNavigation = createExplicitConversationNavigationIntent(newChat, location, 1);
  renderContextWindowProgressRing();
  assert.strictEqual(routerCalls.length, 1, "route synchronization must not steal an explicit user navigation");
  window.__swissExplicitConversationNavigation = null;

  const loadingIdentity = { conversationId: null, phase: "loading", lastRenderedId: null, routeFallbackBlocked: false };
  const loadingGenerations = {};
  assert.strictEqual(
    transitionConversationMetricIdentity(loadingIdentity, loadingGenerations, routedViewId, "/c/" + staleRouteId, true, true),
    routedViewId,
    "the rendered conversation must become the metric identity"
  );
  assert.strictEqual(
    transitionConversationMetricIdentity(loadingIdentity, loadingGenerations, null, "/c/" + staleRouteId, false, true),
    routedViewId,
    "a transient loading gap must retain the real rendered metric identity"
  );
  assert.strictEqual(
    transitionConversationMetricIdentity(loadingIdentity, loadingGenerations, null, "/c/" + staleRouteId, true, true),
    null,
    "a blank rendered view must invalidate the metric identity"
  );
  assert.strictEqual(
    transitionConversationMetricIdentity(loadingIdentity, loadingGenerations, null, "/c/" + staleRouteId, false, true),
    null,
    "the blank-view route fence must survive a subsequent no-view tick"
  );

  location.pathname = "/c/" + fallbackRouteId;
  location.search = "";
  location.hash = "";
  setVisible(fallbackViewId);
  window.__TSR_ROUTER__ = null;
  const replacements = [];
  window.history = {
    state: { hostState: "keep" },
    replaceState: (state, title, url) => {
      replacements.push({ state, title, url });
      location.pathname = String(url).split("?")[0].split("#")[0];
      location.search = "";
      location.hash = "";
    }
  };
  renderContextWindowProgressRing();
  assert.strictEqual(replacements.length, 1, "history fallback must synchronize once when the host router is unavailable");
  assert.strictEqual(replacements[0].url, "/c/" + fallbackViewId, "history fallback must address the rendered conversation");
  assert.deepStrictEqual(replacements[0].state, { hostState: "keep" }, "history fallback must preserve host state");
  renderContextWindowProgressRing();
  assert.strictEqual(replacements.length, 1, "history fallback must not repeat on observer/render ticks");
  assert(!popstateEvents.includes("popstate"), "route synchronization must not fabricate a popstate acknowledgement");

  console.log("INTEGRATED_IDENTITY_OK");
})().catch((error) => { console.error(error.stack || error); process.exit(1); });
`)
}

func TestConversationIdentity_RouteSyncAcknowledgesAsyncRouterNoOp(t *testing.T) {
	runIdentityCallChain(t, `
(async () => {
  const staleId = "9d07fb82-25f8-46d6-9692-16bded70cb35";
  const visibleId = "eef014a4-c5ae-4fbe-9da6-8592c619ced9";
  const hostSetTimeout = require("timers").setTimeout;
  const hostClearTimeout = require("timers").clearTimeout;
  globalThis.setTimeout = hostSetTimeout;
  globalThis.clearTimeout = hostClearTimeout;
  const later = (ms) => new Promise((resolve) => hostSetTimeout(resolve, ms));
  const routerCalls = [];
  const replacements = [];
  const popstateEvents = [];

  function applyLocationURL(url) {
    const parsed = new URL(String(url), "http://127.0.0.1");
    location.pathname = parsed.pathname;
    location.search = parsed.search;
    location.hash = parsed.hash;
  }

  location.pathname = "/c/" + staleId;
  location.search = "?tab=chat";
  location.hash = "#reply";
  window.__TSR_ROUTER__ = { navigate: ({ to }) => routerCalls.push(to) };
  window.history = {
    state: { hostState: "keep" },
    replaceState: (state, title, url) => {
      replacements.push({ state, title, url });
      applyLocationURL(url);
    }
  };
  window.dispatchEvent = (event) => { popstateEvents.push(event.type); return true; };

  assert.strictEqual(
    synchronizeRenderedConversationRoute(conversationRouteSyncState, visibleId, location, null),
    "navigate",
    "the first rendered mismatch must navigate through the host router"
  );
  assert.deepStrictEqual(routerCalls, ["/c/" + visibleId + "?tab=chat#reply"], "the first navigate must preserve query and hash");
  synchronizeRenderedConversationRoute(conversationRouteSyncState, visibleId, location, null);
  synchronizeRenderedConversationRoute(conversationRouteSyncState, visibleId, location, null);
  assert.strictEqual(routerCalls.length, 1, "render ticks during the acknowledgement window must not duplicate navigate");

  await later(240);
  assert.strictEqual(routerCalls.length, 2, "an unacknowledged navigate must receive one bounded retry");
  assert.strictEqual(replacements.length, 1, "an unacknowledged retry must fall back exactly once");
  assert.strictEqual(replacements[0].url, "/c/" + visibleId + "?tab=chat#reply", "fallback must preserve query and hash");
  assert.deepStrictEqual(replacements[0].state, { hostState: "keep" }, "fallback must preserve host history state");
  assert.strictEqual(parseConversationId(location.pathname), visibleId, "the fallback URL must converge on the rendered conversation");
  assert.strictEqual(
    synchronizeRenderedConversationRoute(conversationRouteSyncState, visibleId, location, null),
    "none",
    "converged routes must not start another synchronization"
  );
  assert.strictEqual(replacements.length, 1, "converged routes must not repeat fallback");
  assert(!popstateEvents.includes("popstate"), "route acknowledgement must not synthesize popstate");
  console.log("INTEGRATED_IDENTITY_OK");
})().catch((error) => { console.error(error.stack || error); process.exit(1); });
`)
}

func TestConversationIdentity_RouteSyncDelayedSuccessAndObsoleteTargetCancellation(t *testing.T) {
	runIdentityCallChain(t, `
(async () => {
  const staleId = "11111111-1111-4111-8111-111111111111";
  const firstVisibleId = "22222222-2222-4222-8222-222222222222";
  const secondVisibleId = "33333333-3333-4333-8333-333333333333";
  const hostSetTimeout = require("timers").setTimeout;
  const hostClearTimeout = require("timers").clearTimeout;
  globalThis.setTimeout = hostSetTimeout;
  globalThis.clearTimeout = hostClearTimeout;
  const later = (ms) => new Promise((resolve) => hostSetTimeout(resolve, ms));
  const routerCalls = [];
  const replacements = [];
  let acceptedTarget = "";

  function applyLocationURL(url) {
    const parsed = new URL(String(url), "http://127.0.0.1");
    location.pathname = parsed.pathname;
    location.search = parsed.search;
    location.hash = parsed.hash;
  }

  location.pathname = "/c/" + staleId;
  location.search = "";
  location.hash = "";
  window.__TSR_ROUTER__ = {
    navigate: ({ to }) => {
      routerCalls.push(to);
      const target = to;
      hostSetTimeout(() => {
        if (acceptedTarget === target) applyLocationURL(target);
      }, 15);
    }
  };
  window.history = {
    state: { hostState: "keep" },
    replaceState: (state, title, url) => replacements.push({ state, title, url })
  };

  acceptedTarget = "/c/" + firstVisibleId;
  assert.strictEqual(
    synchronizeRenderedConversationRoute(conversationRouteSyncState, firstVisibleId, location, null),
    "navigate",
    "the first target must navigate"
  );
  synchronizeRenderedConversationRoute(conversationRouteSyncState, firstVisibleId, location, null);
  synchronizeRenderedConversationRoute(conversationRouteSyncState, firstVisibleId, location, null);
  assert.deepStrictEqual(routerCalls, ["/c/" + firstVisibleId], "repeated ticks must stay inside the first acknowledgement window");

  acceptedTarget = "/c/" + secondVisibleId;
  assert.strictEqual(
    synchronizeRenderedConversationRoute(conversationRouteSyncState, secondVisibleId, location, null),
    "navigate",
    "a changed rendered target must supersede the pending target"
  );
  synchronizeRenderedConversationRoute(conversationRouteSyncState, secondVisibleId, location, null);
  await later(120);

  assert.deepStrictEqual(routerCalls, ["/c/" + firstVisibleId, "/c/" + secondVisibleId], "the delayed second target must acknowledge without duplicate navigation");
  assert.strictEqual(parseConversationId(location.pathname), secondVisibleId, "delayed router success must converge on the newest target");
  assert.strictEqual(replacements.length, 0, "delayed router success must not trigger fallback");
  assert.strictEqual(
    synchronizeRenderedConversationRoute(conversationRouteSyncState, secondVisibleId, location, null),
    "none",
    "a delayed acknowledgement must clear pending synchronization"
  );
  console.log("INTEGRATED_IDENTITY_OK");
})().catch((error) => { console.error(error.stack || error); process.exit(1); });
`)
}

func TestConversationIdentity_RouteSyncExplicitIntentAndMissingRouterFallback(t *testing.T) {
	runIdentityCallChain(t, `
(async () => {
  const staleId = "44444444-4444-4444-8444-444444444444";
  const explicitId = "55555555-5555-4555-8555-555555555555";
  const disagreementId = "66666666-6666-4666-8666-666666666666";
  const fallbackId = "77777777-7777-4777-8777-777777777777";
  const hostSetTimeout = require("timers").setTimeout;
  const hostClearTimeout = require("timers").clearTimeout;
  globalThis.setTimeout = hostSetTimeout;
  globalThis.clearTimeout = hostClearTimeout;
  const later = (ms) => new Promise((resolve) => hostSetTimeout(resolve, ms));
  const routerCalls = [];
  const replacements = [];
  const popstateEvents = [];

  function applyLocationURL(url) {
    const parsed = new URL(String(url), "http://127.0.0.1");
    location.pathname = parsed.pathname;
    location.search = parsed.search;
    location.hash = parsed.hash;
  }
  function navigationElement(targetId) {
    const anchor = { href: "http://127.0.0.1/c/" + targetId };
    const element = {
      closest: (selector) => {
        const value = String(selector);
        if (value.includes("a[href]")) return anchor;
        if (value.includes("conversation-row-sidebar")) return element;
        return null;
      }
    };
    return element;
  }

  location.pathname = "/c/" + staleId;
  location.search = "";
  location.hash = "";
  window.__TSR_ROUTER__ = {
    navigate: ({ to }) => {
      routerCalls.push(to);
      hostSetTimeout(() => applyLocationURL(to), 10);
    }
  };
  window.history = {
    state: { hostState: "keep" },
    replaceState: (state, title, url) => {
      replacements.push({ state, title, url });
      applyLocationURL(url);
    }
  };
  window.dispatchEvent = (event) => { popstateEvents.push(event.type); return true; };

  const agreementIntent = createExplicitConversationNavigationIntent(navigationElement(explicitId), location, 1);
  assert.strictEqual(
    synchronizeRenderedConversationRoute(conversationRouteSyncState, explicitId, location, agreementIntent),
    "navigate",
    "an explicit target agreeing with the rendered ID must repair the stale URL"
  );
  synchronizeRenderedConversationRoute(conversationRouteSyncState, explicitId, location, agreementIntent);
  await later(100);
  assert.deepStrictEqual(routerCalls, ["/c/" + explicitId], "explicit target agreement must synchronize once");
  assert.strictEqual(replacements.length, 0, "acknowledged explicit target agreement must not fall back");

  location.pathname = "/c/" + staleId;
  location.search = "";
  location.hash = "";
  const callsBeforeDisagreement = routerCalls.length;
  assert.strictEqual(
    synchronizeRenderedConversationRoute(conversationRouteSyncState, disagreementId, location, agreementIntent),
    "none",
    "an explicit target disagreeing with the rendered ID must fence synchronization"
  );
  assert.strictEqual(routerCalls.length, callsBeforeDisagreement, "explicit target disagreement must not navigate");
  assert.strictEqual(replacements.length, 0, "explicit target disagreement must not fall back");

  location.pathname = "/c/" + staleId;
  location.search = "?tab=chat";
  location.hash = "#reply";
  window.__TSR_ROUTER__ = null;
  assert.strictEqual(
    synchronizeRenderedConversationRoute(conversationRouteSyncState, fallbackId, location, null),
    "replaceState",
    "a missing router must use the immediate safe history fallback"
  );
  assert.strictEqual(replacements.length, 1, "missing-router fallback must run exactly once");
  assert.strictEqual(replacements[0].url, "/c/" + fallbackId + "?tab=chat#reply", "missing-router fallback must preserve query and hash");
  assert.deepStrictEqual(replacements[0].state, { hostState: "keep" }, "missing-router fallback must preserve host state");
  synchronizeRenderedConversationRoute(conversationRouteSyncState, fallbackId, location, null);
  assert.strictEqual(replacements.length, 1, "missing-router fallback must not repeat after convergence");
  assert(!popstateEvents.includes("popstate"), "explicit-intent and fallback synchronization must not synthesize popstate");
  console.log("INTEGRATED_IDENTITY_OK");
})().catch((error) => { console.error(error.stack || error); process.exit(1); });
`)
}

func TestConversationIdentity_ExplicitIntentRouteAgreementAndBlankReset(t *testing.T) {
	runIdentityCallChain(t, `
(async () => {
  const oldId = "c23cddd7-1111-4222-8333-444444444444";
  const newId = "eef014a4-5555-4666-8777-888888888888";
  const staleUrlId = "99999999-aaaa-4bbb-8ccc-dddddddddddd";
  const disagreementViewId = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";
  const disagreementTargetId = "bbbbbbbb-cccc-4ddd-8eee-ffffffffffff";
  const routerCalls = [];
  const metricRequests = [];
  const popstateEvents = [];

  window.fetch = async (url) => {
    if (String(url).includes("/api/tokens/chat-metrics")) {
      metricRequests.push(new URL(url).searchParams.get("conversation_id"));
    }
    return { ok: true, json: async () => ({ success: true, pending_intents: [], user_prompt_count: 0, prompts: [], turns: [] }) };
  };
  window.__TSR_ROUTER__ = { navigate: ({ to }) => routerCalls.push(to) };
  window.dispatchEvent = (event) => { popstateEvents.push(event.type); return true; };

  const view = new NodeEl("div", "conversation-view");
  const inputBox = new NodeEl("div", "agent-input-box");
  inputBox.className = "no-focus-agent-input";
  inputBox.classList.add("no-focus-agent-input");
  const trigger = new NodeEl("button", "model-selector-trigger");
  inputBox.appendChild(trigger);
  view.appendChild(inputBox);
  document.body.appendChild(view);

  function setVisible(id) {
    view.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
    inputBox.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
  }
  function navigationElement(kind, targetId) {
    const anchor = targetId ? { href: "http://127.0.0.1/c/" + targetId } : null;
    const element = {
      closest: (selector) => {
        const value = String(selector);
        if (value.includes("a[href]")) return anchor;
        if (kind === "new" && value.includes("new-conversation-button")) return element;
        if (kind === "conversation" && value.includes("conversation-row-sidebar")) return element;
        if (kind === "conversation" && value.includes('a[href*="/c/"]')) return element;
        return null;
      }
    };
    return element;
  }

  location.pathname = "/c/" + oldId;
  location.search = "";
  location.hash = "";
  setVisible(oldId);
  window.__swissExplicitConversationNavigation = null;
  cachedTokensByConv[oldId] = 149000;
  cachedContextWindowByConv[oldId] = { used_tokens: 149000, max_tokens: 256000 };
  renderContextWindowProgressRing();
  let ring = document.getElementById("swiss-context-window-ring");
  assert(ring && /149K/.test(ring.getAttribute("title")), "fixture must paint the old numeric ring before New Conversation");
  metricRequests.length = 0;

  const newChat = new NodeEl("button", "new-conversation-button");
  document.body.appendChild(newChat);
  emitDocument("click", newChat, true);
  const newIntent = window.__swissExplicitConversationNavigation;
  assert(newIntent, "the real document click listener must install a New Conversation intent");
  assert.strictEqual(newIntent.kind, "new_conversation", "fixture must create a New Conversation intent");
  ring = document.getElementById("swiss-context-window-ring");
  assert.strictEqual(conversationMetricIdentity.conversationId, null, "New Conversation must invalidate identity immediately");
  assert.strictEqual(conversationMetricIdentity.phase, "blank", "New Conversation must enter blank identity immediately");
  assert.strictEqual(ring, null, "New Conversation must remove the stale numeric ring immediately");
  assert.deepStrictEqual(metricRequests, [], "a blank New Conversation at the stale URL must not fetch stale metrics");

  setVisible(newId);
  renderContextWindowProgressRing();
  assert.deepStrictEqual(routerCalls, ["/c/" + newId], "the first rendered New Conversation ID must synchronize once while URL remains old");
  renderContextWindowProgressRing();
  renderContextWindowProgressRing();
  assert.strictEqual(routerCalls.length, 1, "pending New Conversation synchronization must not loop on render ticks");

  routerCalls.length = 0;
  location.pathname = "/c/" + staleUrlId;
  setVisible(newId);
  window.__swissExplicitConversationNavigation = createExplicitConversationNavigationIntent(navigationElement("conversation", newId), location, 2);
  renderContextWindowProgressRing();
  assert.deepStrictEqual(routerCalls, ["/c/" + newId], "sidebar intent agreeing with the rendered ID must repair the stale URL exactly once");
  renderContextWindowProgressRing();
  renderContextWindowProgressRing();
  assert.strictEqual(routerCalls.length, 1, "target agreement must not repeat synchronization on render ticks");

  routerCalls.length = 0;
  location.pathname = "/c/" + staleUrlId;
  setVisible(disagreementViewId);
  window.__swissExplicitConversationNavigation = createExplicitConversationNavigationIntent(navigationElement("conversation", disagreementTargetId), location, 3);
  renderContextWindowProgressRing();
  renderContextWindowProgressRing();
  assert.strictEqual(routerCalls.length, 0, "explicit target disagreement must fence rendered-ID synchronization");
  assert(!popstateEvents.includes("popstate"), "explicit-intent route synchronization must not synthesize popstate");

  console.log("INTEGRATED_IDENTITY_OK");
})().catch((error) => { console.error(error.stack || error); process.exit(1); });
`)
}

func TestConversationIdentity_GeneratedNewConversationRingResetCallChain(t *testing.T) {
	runIdentityCallChain(t, `
(async () => {
  const oldId = "c23cddd7-1111-4222-8333-444444444444";
  const newId = "eef014a4-5555-4666-8777-888888888888";
  const metricRequests = [];
  const oldMetrics = deferred();
  const routerCalls = [];
  const hostSetTimeout = require("timers").setTimeout;
  const later = (ms) => new Promise(resolve => hostSetTimeout(resolve, ms));

  window.fetch = async (url) => {
    if (String(url).includes("/api/tokens/chat-metrics")) {
      const id = new URL(url).searchParams.get("conversation_id");
      metricRequests.push(id);
      if (id === oldId) return oldMetrics.promise;
      if (id === newId) {
        return {
          ok: true,
          json: async () => ({
            conversation_id: newId,
            success: true,
            turns: [{ context_tokens: 245000, input_tokens: 245000, output_tokens: 0 }],
            context_window: { used_tokens: 245000, max_tokens: 256000 }
          })
        };
      }
    }
    return {
      ok: true,
      json: async () => ({ success: true, pending_intents: [], user_prompt_count: 0, prompts: [], turns: [] })
    };
  };
  window.__TSR_ROUTER__ = { navigate: ({ to }) => routerCalls.push(to) };
  window.__swissExplicitConversationNavigation = null;

  const view = new NodeEl("div", "conversation-view");
  const inputBox = new NodeEl("div", "agent-input-box");
  inputBox.className = "no-focus-agent-input";
  inputBox.classList.add("no-focus-agent-input");
  const trigger = new NodeEl("button", "model-selector-trigger");
  inputBox.appendChild(trigger);
  view.appendChild(inputBox);
  document.body.appendChild(view);

  function setVisible(id) {
    view.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
    inputBox.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
  }

  location.pathname = "/c/" + oldId;
  location.search = "";
  location.hash = "";
  setVisible(oldId);
  cachedTokensByConv[oldId] = 245000;
  cachedContextWindowByConv[oldId] = { used_tokens: 245000, max_tokens: 256000 };
  renderContextWindowProgressRing();

  let ring = document.getElementById("swiss-context-window-ring");
  assert(ring && /96% \(245K\/256K active\)/.test(ring.getAttribute("title")), "fixture must paint the old numeric ring");
  assert.deepStrictEqual(metricRequests, [oldId], "fixture must leave an old-route metric request in flight");
  metricRequests.length = 0;

  const newChat = new NodeEl("button", "new-conversation-button");
  document.body.appendChild(newChat);
  emitDocument("click", newChat, true);

  assert(window.__swissExplicitConversationNavigation, "the real document click listener must install New Conversation intent");
  assert.strictEqual(conversationMetricIdentity.conversationId, null, "New Conversation must invalidate metric identity immediately");
  assert.strictEqual(conversationMetricIdentity.phase, "blank", "New Conversation must enter blank identity immediately");
  assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "New Conversation must remove the numeric ring synchronously");
  assert.strictEqual(document.querySelectorAll(".swiss-context-window-ring, .swiss-context-window-ring-sub").length, 0, "New Conversation must remove every context ring synchronously");
  assert.deepStrictEqual(metricRequests, [], "New Conversation must not fetch metrics for the old route");

  location.pathname = "/";
  location.search = "";
  setVisible(null);
  view.remove();
  await flush();
  await later(20);
  await flush();

  assert.strictEqual(conversationMetricIdentity.conversationId, null, "identity must stay blank while no conversation ID is rendered");
  assert.strictEqual(conversationMetricIdentity.phase, "blank", "identity must stay in the blank phase while no conversation ID is rendered");
  assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "no ring may appear without a rendered conversation identity");
  assert.deepStrictEqual(metricRequests, [], "no delayed render may fetch the prior route after New Conversation");

  oldMetrics.resolve({
    ok: true,
    json: async () => ({
      conversation_id: oldId,
      success: true,
      user_prompt_count: 1,
      prompts: [{ index: 0, text: "old prompt" }],
      turns: [{ context_tokens: 245000, input_tokens: 245000, output_tokens: 0 }],
      context_window: { used_tokens: 245000, max_tokens: 256000 }
    })
  });
  await flush();
  await later(20);
  await flush();

  assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "a delayed prior response must not repaint the ring");
  assert.strictEqual(document.querySelectorAll("[title*='245K']").length, 0, "a delayed prior response must not restore a numeric title");
  assert.strictEqual(conversationMetricIdentity.conversationId, null, "a delayed prior response must not restore metric identity");
  assert(!Object.prototype.hasOwnProperty.call(cachedTokensByConv, oldId), "New Conversation must discard the prior numeric cache");
  assert(!Object.prototype.hasOwnProperty.call(cachedContextWindowByConv, oldId), "New Conversation must discard the prior context-window cache");

  setVisible(newId);
  document.body.appendChild(view);
  renderContextWindowProgressRing();
  await flush();
  await later(20);
  await flush();

  ring = document.getElementById("swiss-context-window-ring");
  assert(ring, "a rendered conversation ID must mount the ring again");
  assert(/96% \(245K\/256K active\)/.test(ring.getAttribute("title")), "a rendered conversation ID must render fresh numeric metrics: " + ring.getAttribute("title") + " requests=" + JSON.stringify(metricRequests) + " identity=" + JSON.stringify(conversationMetricIdentity) + " tokens_cache=" + JSON.stringify(cachedTokensByConv[newId]) + " window_cache=" + JSON.stringify(cachedContextWindowByConv[newId]) + " generations=" + JSON.stringify(conversationMetricRequestGenerations));
  assert.strictEqual(conversationMetricIdentity.conversationId, newId, "a rendered conversation ID must resume metric identity");
  assert.deepStrictEqual(metricRequests, [newId], "the new conversation must fetch metrics exactly once");
  assert.deepStrictEqual(routerCalls, ["/c/" + newId], "New Conversation route synchronization must still run once");

  renderContextWindowProgressRing();
  await flush();
  assert.deepStrictEqual(metricRequests, [newId], "render ticks must not repeat the fresh metric fetch");
  console.log("INTEGRATED_IDENTITY_OK");
})().catch((error) => { console.error(error.stack || error); process.exit(1); });
`)
}

func TestConversationIdentity_CrossInstanceMetricResetEpochCallChain(t *testing.T) {
	runIdentityCallChain(t, `
(async () => {
  const oldId = "c23cddd7-1111-4222-8333-444444444444";
  const newId = "eef014a4-5555-4666-8777-888888888888";
  const metricRequests = [];
  const oldMetrics = deferred();
  const restoredMetrics = deferred();
  let oldMetricRequestCount = 0;
  const routerCalls = [];
  const hostSetTimeout = require("timers").setTimeout;
  const later = (ms) => new Promise(resolve => hostSetTimeout(resolve, ms));

  window.__swissConversationMetricResetEpoch = 0;
  window.__swissExplicitConversationNavigation = null;
  window.fetch = async (url) => {
    if (String(url).includes("/api/tokens/chat-metrics")) {
      const id = new URL(url).searchParams.get("conversation_id");
      metricRequests.push(id);
      if (id === oldId) {
        oldMetricRequestCount += 1;
        return oldMetricRequestCount === 1 ? oldMetrics.promise : restoredMetrics.promise;
      }
      if (id === newId) {
        return {
          ok: true,
          json: async () => ({
            conversation_id: newId,
            success: true,
            turns: [{ context_tokens: 187000, input_tokens: 187000, output_tokens: 0 }],
            context_window: { used_tokens: 187000, max_tokens: 256000 }
          })
        };
      }
    }
    return {
      ok: true,
      json: async () => ({ success: true, pending_intents: [], user_prompt_count: 0, prompts: [], turns: [] })
    };
  };
  window.__TSR_ROUTER__ = { navigate: ({ to }) => routerCalls.push(to) };

  const view = new NodeEl("div", "conversation-view");
  const inputBox = new NodeEl("div", "agent-input-box");
  inputBox.className = "no-focus-agent-input";
  inputBox.classList.add("no-focus-agent-input");
  const trigger = new NodeEl("button", "model-selector-trigger");
  inputBox.appendChild(trigger);
  view.appendChild(inputBox);
  document.body.appendChild(view);

  function setVisible(id) {
    view.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
    inputBox.__reactFiber = { memoizedProps: { conversationId: id }, return: null };
  }

  location.pathname = "/c/" + oldId;
  location.search = "";
  location.hash = "";
  setVisible(oldId);
  cachedTokensByConv[oldId] = 245000;
  cachedContextWindowByConv[oldId] = { used_tokens: 245000, max_tokens: 256000 };
  renderContextWindowProgressRing();

  let ring = document.getElementById("swiss-context-window-ring");
  assert(ring && /96% \(245K\/256K active\)/.test(ring.getAttribute("title")), "fixture must paint the old numeric ring");
  assert.deepStrictEqual(metricRequests, [oldId], "fixture must leave an old-route metric response pending");
  metricRequests.length = 0;

  window.__swissExplicitConversationNavigation = {
    version: 2,
    kind: "new_conversation",
    at: 1,
    sourceRoute: "/c/" + oldId,
    targetRoute: "",
    targetConversationId: null,
    route: "",
    conversationId: null,
    pending: true
  };

  // Another injected instance changed the page-scoped epoch. This instance did
  // not run the owning click listener's local cleanup.
  window.__swissConversationMetricResetEpoch += 1;
  renderContextWindowProgressRing();

  assert.strictEqual(conversationMetricIdentity.conversationId, null, "an unseen epoch must invalidate this instance's identity");
  assert.strictEqual(conversationMetricIdentity.phase, "blank", "an unseen epoch must suppress the stale rendered ID");
  assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "an unseen epoch must remove the stale ring before metrics render");
  assert.strictEqual(document.querySelectorAll(".swiss-context-window-ring, .swiss-context-window-ring-sub").length, 0, "an unseen epoch must remove every ring");
  assert(!Object.prototype.hasOwnProperty.call(cachedTokensByConv, oldId), "an unseen epoch must discard this instance's token cache");
  assert(!Object.prototype.hasOwnProperty.call(cachedContextWindowByConv, oldId), "an unseen epoch must discard this instance's context-window cache");

  oldMetrics.resolve({
    ok: true,
    json: async () => ({
      conversation_id: oldId,
      success: true,
      user_prompt_count: 1,
      prompts: [{ index: 0, text: "old prompt" }],
      turns: [{ context_tokens: 245000, input_tokens: 245000, output_tokens: 0 }],
      context_window: { used_tokens: 245000, max_tokens: 256000 }
    })
  });
  await flush();
  await later(20);
  await flush();

  assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "a delayed old response must not repaint the stale ring");
  assert.strictEqual(document.querySelectorAll("[title*='245K']").length, 0, "a delayed old response must not restore a numeric title");
  assert.strictEqual(conversationMetricIdentity.conversationId, null, "a delayed old response must not restore identity");

  // Boundary: A -> New Conversation pending -> A must resume A explicitly.
  metricRequests.length = 0;
  routerCalls.length = 0;
  location.pathname = "/c/" + newId;
  setVisible(oldId);
  const restoreTarget = {
    closest: (selector) => {
      const value = String(selector);
      if (value.includes("a[href]")) return { href: "http://127.0.0.1/c/" + oldId };
      if (value.includes("conversation-row-sidebar") || value.includes('a[href*="/c/"]')) return restoreTarget;
      return null;
    }
  };
  window.__swissExplicitConversationNavigation =
    createExplicitConversationNavigationIntent(restoreTarget, location, 2);
  assert.strictEqual(window.__swissExplicitConversationNavigation.kind, "conversation", "fixture must model explicit sidebar navigation");
  assert.strictEqual(window.__swissExplicitConversationNavigation.targetConversationId, oldId, "fixture must explicitly target A");
  renderContextWindowProgressRing();
  assert.deepStrictEqual(metricRequests, [oldId], "explicitly returning to A must issue a fresh metrics request");
  restoredMetrics.resolve({
    ok: true,
    json: async () => ({
      conversation_id: oldId,
      success: true,
      turns: [{ context_tokens: 149000, input_tokens: 149000, output_tokens: 0 }],
      context_window: { used_tokens: 149000, max_tokens: 256000 }
    })
  });
  await flush();
  await later(20);
  await flush();

  ring = document.getElementById("swiss-context-window-ring");
  assert(ring, "explicitly returning to A must remount the ring");
  assert(/58% \(149K\/256K active\)/.test(ring.getAttribute("title")), "explicitly returning to A must resume fresh metrics: " + ring.getAttribute("title"));
  assert.strictEqual(conversationMetricIdentity.conversationId, oldId, "explicitly returning to A must restore metric identity");
  assert.strictEqual(conversationMetricResetStaleId, null, "explicitly returning to A must clear the stale marker");
  assert.deepStrictEqual(metricRequests, [oldId], "explicitly returning to A must fetch A metrics once");
  assert.deepStrictEqual(routerCalls, ["/c/" + oldId], "explicitly returning to A must synchronize the route once");

  // The actual trusted New Conversation listener must advance the shared epoch.
  metricRequests.length = 0;
  routerCalls.length = 0;
  location.pathname = "/c/" + oldId;
  const epochBeforeClick = window.__swissConversationMetricResetEpoch;
  const newChat = new NodeEl("button", "new-conversation-button");
  document.body.appendChild(newChat);
  emitDocument("click", newChat, true);
  assert.strictEqual(window.__swissConversationMetricResetEpoch, epochBeforeClick + 1, "trusted New Conversation must increment the page-scoped epoch");
  assert.strictEqual(window.__swissExplicitConversationNavigation.kind, "new_conversation", "trusted New Conversation must install intent");
  assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "trusted New Conversation must remove the current ring");
  renderContextWindowProgressRing();
  assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "pending New Conversation must keep A suppressed");

  setVisible(newId);
  renderContextWindowProgressRing();
  await flush();
  await later(20);
  await flush();

  ring = document.getElementById("swiss-context-window-ring");
  assert(ring, "a new valid rendered ID must mount the ring after an epoch reset");
  assert(/73% \(187K\/256K active\)/.test(ring.getAttribute("title")), "a new valid ID must render fresh metrics: " + ring.getAttribute("title"));
  assert.strictEqual(conversationMetricIdentity.conversationId, newId, "a new valid ID must not stay permanently suppressed");
  assert.deepStrictEqual(metricRequests, [newId], "a new valid ID must fetch metrics exactly once");
  assert.deepStrictEqual(routerCalls, ["/c/" + newId], "a new valid ID must still synchronize the route exactly once");

  renderContextWindowProgressRing();
  await flush();
  assert.deepStrictEqual(metricRequests, [newId], "render after a new ID must not repeat the metric fetch");
  console.log("INTEGRATED_IDENTITY_OK");
})().catch((error) => { console.error(error.stack || error); process.exit(1); });
`)
}
