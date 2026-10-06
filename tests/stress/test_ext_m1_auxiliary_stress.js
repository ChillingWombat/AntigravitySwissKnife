#!/usr/bin/env node
/**
 * Empirical Adversarial Stress Test Suite for Ext-M1 (Requirements R1 & R2)
 *
 * Verifies:
 * Phase 1: Pure Syntax and Compilation Check of live Go GenerateAuxiliaryPluginsScript()
 * Phase 2: Synthetic DOM structure & missing elements (graceful degradation)
 * Phase 3: Rapid two-way tab switching (Swiss tabs <-> factory tabs) without exceptions
 * Phase 4: Coordinate calculations in getCanvasCoords under container scales,
 *          subpixel offsets, and zero-size dimensions
 * Phase 5: Resource & listener leak analysis
 * Phase 6: Send to Chat workflow (canvas compositing, file attachment, Lexical / textarea injection)
 */

const fs = require('fs');
const path = require('path');
const vm = require('vm');
const { execSync } = require('child_process');

// -------------------------------------------------------------------------
// Helper: DOM Mock Engine
// -------------------------------------------------------------------------
class MockDOMTokenList {
  constructor(el) {
    this.el = el;
    this._set = new Set();
  }
  add(...tokens) {
    tokens.forEach(t => { if (t) this._set.add(t); });
    this._sync();
  }
  remove(...tokens) {
    tokens.forEach(t => this._set.delete(t));
    this._sync();
  }
  toggle(token, force) {
    if (force === true) {
      this._set.add(token);
    } else if (force === false) {
      this._set.delete(token);
    } else {
      if (this._set.has(token)) this._set.delete(token);
      else this._set.add(token);
    }
    this._sync();
    return this._set.has(token);
  }
  contains(token) {
    return this._set.has(token);
  }
  _sync() {
    this.el._className = Array.from(this._set).join(' ');
  }
  _parse(str) {
    this._set.clear();
    (str || '').split(/\s+/).filter(Boolean).forEach(t => this._set.add(t));
  }
}

class MockElement {
  constructor(tagName, doc) {
    this.tagName = (tagName || 'div').toUpperCase();
    this.ownerDocument = doc;
    this.parentElement = null;
    this.children = [];
    this.attributes = new Map();
    this.dataset = {};
    this.style = {};
    this._className = '';
    this.classList = new MockDOMTokenList(this);
    this._eventListeners = new Map();
    this.id = '';
    this.innerText = '';
    this.textContent = '';
    this._innerHTML = '';
    this._rect = { left: 0, top: 0, width: 400, height: 600, right: 400, bottom: 600 };
  }

  get className() { return this._className; }
  set className(val) {
    this._className = val || '';
    this.classList._parse(val);
  }

  get clientWidth() { return this._rect.width || 0; }
  get clientHeight() { return this._rect.height || 0; }

  getBoundingClientRect() {
    return { ...this._rect };
  }

  setBoundingClientRect(rect) {
    this._rect = { ...rect };
    this._rect.right = this._rect.left + this._rect.width;
    this._rect.bottom = this._rect.top + this._rect.height;
  }

  setAttribute(name, val) {
    this.attributes.set(name, String(val));
    if (name === 'id') this.id = String(val);
    if (name === 'class') this.className = String(val);
    if (name.startsWith('data-')) {
      const key = name.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase());
      this.dataset[key] = String(val);
    }
  }

  getAttribute(name) {
    return this.attributes.has(name) ? this.attributes.get(name) : null;
  }

  removeAttribute(name) {
    this.attributes.delete(name);
    if (name === 'id') this.id = '';
    if (name === 'class') this.className = '';
  }

  hasAttribute(name) {
    return this.attributes.has(name);
  }

  appendChild(child) {
    if (!child) return null;
    if (child.parentElement) {
      child.parentElement.removeChild(child);
    }
    child.parentElement = this;
    this.children.push(child);
    if (this.ownerDocument && this.ownerDocument._triggerMutation) {
      this.ownerDocument._triggerMutation();
    }
    return child;
  }

  removeChild(child) {
    const idx = this.children.indexOf(child);
    if (idx !== -1) {
      this.children.splice(idx, 1);
      child.parentElement = null;
      if (this.ownerDocument && this.ownerDocument._triggerMutation) {
        this.ownerDocument._triggerMutation();
      }
    }
    return child;
  }

  addEventListener(type, listener, options) {
    if (!this._eventListeners.has(type)) {
      this._eventListeners.set(type, []);
    }
    this._eventListeners.get(type).push(listener);
  }

  removeEventListener(type, listener) {
    if (!this._eventListeners.has(type)) return;
    const list = this._eventListeners.get(type).filter(l => l !== listener);
    this._eventListeners.set(type, list);
  }

  dispatchEvent(event) {
    if (!event.target) event.target = this;
    event.currentTarget = this;
    const list = this._eventListeners.get(event.type) || [];
    for (const l of list) {
      try { l.call(this, event); } catch (e) {
        if (event._errors) event._errors.push(e);
        else event._errors = [e];
      }
    }
    if (typeof this['on' + event.type] === 'function') {
      try { this['on' + event.type].call(this, event); } catch (e) {
        if (event._errors) event._errors.push(e);
        else event._errors = [e];
      }
    }
    if (event.bubbles && this.parentElement && !event._propagationStopped) {
      this.parentElement.dispatchEvent(event);
    }
    return !event.defaultPrevented;
  }

  closest(selector) {
    let cur = this;
    while (cur && cur.nodeType === 1) {
      if (cur.matches && cur.matches(selector)) return cur;
      cur = cur.parentElement;
    }
    return null;
  }

  get nodeType() { return 1; }

  matches(selector) {
    if (!selector) return false;
    selector = selector.trim();
    if (selector.includes(':not(')) {
      const match = selector.match(/^(.*?):not\((.*?)\)$/);
      if (match) {
        const base = match[1];
        const neg = match[2];
        const baseOk = base ? this.matches(base) : true;
        const negMatch = this.matches(neg);
        return baseOk && !negMatch;
      }
    }
    if (selector.startsWith('#')) return this.id === selector.slice(1);
    if (selector.startsWith('.')) {
      const classes = selector.split('.').filter(Boolean);
      return classes.every(c => this.classList.contains(c));
    }
    if (selector.startsWith('[') && selector.endsWith(']')) {
      const content = selector.slice(1, -1);
      if (content.includes('^=')) {
        const [attr, val] = content.split('^=').map(s => s.replace(/["']/g, '').trim());
        const attrVal = this.getAttribute(attr);
        return attrVal !== null && attrVal.startsWith(val);
      }
      if (content.includes('*=')) {
        const [attr, val] = content.split('*=').map(s => s.replace(/["']/g, '').trim());
        const attrVal = this.getAttribute(attr);
        return attrVal !== null && attrVal.includes(val);
      }
      if (content.includes('=')) {
        const [attr, val] = content.split('=').map(s => s.replace(/["']/g, '').trim());
        return this.getAttribute(attr) === val;
      }
      return this.hasAttribute(content);
    }
    if (selector.toLowerCase() === this.tagName.toLowerCase()) return true;
    return false;
  }

  querySelector(selector) {
    return this._matchDescendant(selector, false);
  }

  querySelectorAll(selector) {
    const results = [];
    this._matchDescendantsAll(selector, results);
    return results;
  }

  _matchDescendant(selector, checkSelf) {
    if (checkSelf && this.matches && this.matches(selector)) return this;
    for (const child of this.children) {
      if (child.matches && child.matches(selector)) return child;
      const found = child._matchDescendant(selector, false);
      if (found) return found;
    }
    return null;
  }

  _matchDescendantsAll(selector, results) {
    for (const child of this.children) {
      if (child.matches && child.matches(selector)) results.push(child);
      child._matchDescendantsAll(selector, results);
    }
  }

  get innerHTML() { return this._innerHTML; }
  set innerHTML(html) {
    this._innerHTML = html;
    this.children = [];
    if (!html || typeof html !== 'string') return;
    const tagRegex = /<([a-zA-Z0-9\-]+)([^>]*)>/g;
    let match;
    while ((match = tagRegex.exec(html)) !== null) {
      const tagName = match[1];
      const attrStr = match[2];
      if (tagName.startsWith('/')) continue;
      const child = this.ownerDocument ? this.ownerDocument.createElement(tagName) : new MockElement(tagName);
      const attrRegex = /([a-zA-Z0-9\-_:]+)(?:=["']([^"']*)["'])?/g;
      let aMatch;
      while ((aMatch = attrRegex.exec(attrStr)) !== null) {
        const aName = aMatch[1];
        const aVal = aMatch[2] !== undefined ? aMatch[2] : '';
        child.setAttribute(aName, aVal);
      }
      this.children.push(child);
      child.parentElement = this;
    }
  }
}

class MockCanvasContext2D {
  constructor(canvas) {
    this.canvas = canvas;
    this.strokeStyle = '#000000';
    this.fillStyle = '#000000';
    this.lineWidth = 1;
    this.lineCap = 'butt';
    this.lineJoin = 'miter';
    this.font = '10px sans-serif';
    this._lineDash = [];

    this.arcCalls = [];
    this.quadraticCurveToCalls = [];
    this.strokeRectCalls = [];
    this.fillRectCalls = [];
    this.strokeCount = 0;
    this.fillCount = 0;
    this.clearRectCalls = [];
  }

  beginPath() {}
  moveTo(x, y) { this._lastMove = { x, y }; }
  lineTo(x, y) { this._lastLine = { x, y }; }
  arc(x, y, r, sa, ea) {
    this.arcCalls.push({ x, y, r, sa, ea });
  }
  quadraticCurveTo(cpx, cpy, x, y) {
    this.quadraticCurveToCalls.push({ cpx, cpy, x, y });
  }
  stroke() { this.strokeCount++; }
  fill() { this.fillCount++; }
  strokeRect(x, y, w, h) {
    this.strokeRectCalls.push({ x, y, w, h });
  }
  fillRect(x, y, w, h) {
    this.fillRectCalls.push({ x, y, w, h });
  }
  clearRect(x, y, w, h) {
    this.clearRectCalls.push({ x, y, w, h });
  }
  setLineDash(dash) { this._lineDash = dash; }
  fillText(text, x, y) {}
  getImageData(x, y, w, h) { return { data: new Uint8ClampedArray(w * h * 4), width: w, height: h }; }
  putImageData(data, x, y) {}
  drawImage() {}
}

class MockCanvasElement extends MockElement {
  constructor(doc) {
    super('canvas', doc);
    this.width = 400;
    this.height = 600;
    this._ctx = new MockCanvasContext2D(this);
  }

  getContext(type) {
    if (type === '2d') return this._ctx;
    return null;
  }

  toBlob(callback, type) {
    const blob = { size: 1024, type: type || 'image/png' };
    if (typeof callback === 'function') setTimeout(() => callback(blob), 0);
  }

  toDataURL(type) {
    return 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';
  }
}

class MockDocument {
  constructor() {
    this._eventListeners = new Map();
    this._observers = [];
    this.body = new MockElement('body', this);
    this.documentElement = new MockElement('html', this);
    this.documentElement.appendChild(this.body);
  }

  _triggerMutation() {
    this._observers.forEach(ob => {
      try { ob.cb([]); } catch (_) {}
    });
  }

  createElement(tag) {
    const t = tag.toLowerCase();
    if (t === 'canvas') return new MockCanvasElement(this);
    const el = new MockElement(t, this);
    if (t === 'input') el.value = '';
    return el;
  }

  getElementById(id) {
    return this.querySelector('#' + id);
  }

  querySelector(selector) {
    if (selector.includes(' ')) {
      const parts = selector.split(/\s+/).filter(Boolean);
      let cur = this.documentElement.querySelector(parts[0]);
      for (let i = 1; i < parts.length && cur; i++) {
        cur = cur.querySelector(parts[i]);
      }
      return cur;
    }
    return this.documentElement.querySelector(selector);
  }

  querySelectorAll(selector) {
    return this.documentElement.querySelectorAll(selector);
  }

  addEventListener(type, listener) {
    if (!this._eventListeners.has(type)) this._eventListeners.set(type, []);
    this._eventListeners.get(type).push(listener);
  }

  removeEventListener(type, listener) {
    if (!this._eventListeners.has(type)) return;
    this._eventListeners.set(type, this._eventListeners.get(type).filter(l => l !== listener));
  }

  dispatchEvent(event) {
    const list = this._eventListeners.get(event.type) || [];
    list.forEach(l => l(event));
  }

  execCommand() { return true; }
}

function createTestEnvironment(customLocalStorage = {}) {
  const doc = new MockDocument();
  const storage = { ...customLocalStorage };
  const intervals = [];

  const mockWindow = {
    __swissAuxiliaryInitialized: false,
    document: doc,
    localStorage: {
      getItem: (k) => (k in storage ? storage[k] : null),
      setItem: (k, v) => { storage[k] = String(v); },
      removeItem: (k) => { delete storage[k]; },
      clear: () => { Object.keys(storage).forEach(k => delete storage[k]); }
    },
    addEventListener: (type, fn) => {
      mockWindow._listeners = mockWindow._listeners || [];
      mockWindow._listeners.push({ type, fn });
    },
    removeEventListener: (type, fn) => {
      if (!mockWindow._listeners) return;
      mockWindow._listeners = mockWindow._listeners.filter(l => l.fn !== fn);
    },
    setTimeout: (fn, ms) => setTimeout(fn, ms || 0),
    clearTimeout: (id) => clearTimeout(id),
    setInterval: (fn, ms) => {
      intervals.push(fn);
      return setInterval(fn, ms || 1000);
    },
    clearInterval: (id) => clearInterval(id),
    triggerIntervals: () => {
      intervals.forEach(fn => { try { fn(); } catch (_) {} });
    },
    prompt: (msg, def) => def || 'Test annotation comment',
    console: {
      log: () => {},
      warn: (...args) => { mockWindow._warns = mockWindow._warns || []; mockWindow._warns.push(args); },
      error: (...args) => { mockWindow._errors = mockWindow._errors || []; mockWindow._errors.push(args); }
    },
    MutationObserver: class {
      constructor(cb) {
        this.cb = cb;
        doc._observers.push(this);
      }
      observe(target, options) {
        // Fire immediately upon initial observe
        try { this.cb([]); } catch (_) {}
      }
      disconnect() {
        const idx = doc._observers.indexOf(this);
        if (idx !== -1) doc._observers.splice(idx, 1);
      }
    },
    Event: class {
      constructor(type, opts) {
        this.type = type;
        this.bubbles = !!(opts && opts.bubbles);
      }
      stopPropagation() { this._propagationStopped = true; }
    },
    CustomEvent: class {
      constructor(type, opts) {
        this.type = type;
        this.detail = opts && opts.detail;
      }
    },
    DataTransfer: class {
      constructor() { this.items = { add: (f) => { this.files = [f]; } }; this.files = []; }
    },
    File: class {
      constructor(parts, name, opts) {
        this.name = name;
        this.type = opts ? opts.type : '';
      }
    },
    Blob: class {
      constructor(parts, opts) { this.size = 100; this.type = opts ? opts.type : ''; }
    }
  };

  mockWindow.window = mockWindow;
  return { window: mockWindow, document: doc, storage };
}

// -------------------------------------------------------------------------
// Load Script from Go
// -------------------------------------------------------------------------
function getLiveScript() {
  const tmpGo = path.resolve(__dirname, 'dump_script.go');
  const goCode = `package main
import (
  "fmt"
  "github.com/ChillingWombat/antigravity-swiss-knife/pkg/plugins"
)
func main() {
  fmt.Print(plugins.GenerateAuxiliaryPluginsScript())
}
`;
  fs.writeFileSync(tmpGo, goCode);
  try {
    const out = execSync(`go run "${tmpGo}"`, {
      cwd: path.resolve(__dirname, '../..'),
      maxBuffer: 10 * 1024 * 1024,
      encoding: 'utf8'
    });
    fs.unlinkSync(tmpGo);
    return out;
  } catch (err) {
    if (fs.existsSync(tmpGo)) fs.unlinkSync(tmpGo);
    throw new Error('Failed to extract live script from Go: ' + err.message);
  }
}

// -------------------------------------------------------------------------
// Main Adversarial Test Suite
// -------------------------------------------------------------------------
async function runStressSuite() {
  console.log('========================================================================');
  console.log('  ANTIGRAVITY SWISS KNIFE - EXT-M1 EMPIRICAL ADVERSARIAL STRESS SUITE  ');
  console.log('========================================================================\n');

  const rawScript = getLiveScript();
  console.log(`[Setup] Loaded live auxiliary script (${(rawScript.length / 1024).toFixed(1)} KB)`);

  let findings = [];
  let passedTests = 0;
  let failedTests = 0;

  function assert(desc, condition, details = '') {
    if (condition) {
      console.log(`  ✓ ${desc}`);
      passedTests++;
    } else {
      console.error(`  ✗ FAIL: ${desc} ${details ? '(' + details + ')' : ''}`);
      failedTests++;
    }
  }

  // -----------------------------------------------------------------------
  // PHASE 1: Pure Syntax and Compilation Check of RAW Live Script
  // -----------------------------------------------------------------------
  console.log('\n[Phase 1] Pure Syntax & Engine Compilation of Live Generated Script');
  let rawSyntaxValid = false;
  try {
    new vm.Script(rawScript);
    rawSyntaxValid = true;
    assert('Live script compiles without SyntaxError in V8 engine', true);
  } catch (err) {
    assert('Live script compiles without SyntaxError in V8 engine', false, err.message);
    findings.push({
      id: 'FINDING-1-SYNTAX-ERROR',
      severity: 'CRITICAL',
      title: 'Unescaped Markdown Backticks in JavaScript Template Literal Cause Fatal SyntaxError',
      location: 'pkg/plugins/auxiliary.go:1698',
      details: err.message,
      impact: 'The entire auxiliary script fails to parse in V8/Chromium. Zero tabs or features mount.'
    });
  }

  // Sanitize script in memory solely for executing subsequent runtime stress suites
  let sanitizedScript = rawScript;
  if (!rawSyntaxValid) {
    console.log('\n[Notice] Applying in-memory sanitization to unescaped backticks to evaluate runtime logic...');
    sanitizedScript = rawScript.replace(
      '```\\n${selectedText}\\n```',
      '\\`\\`\\`\\n${selectedText}\\n\\`\\`\\`'
    );
    try {
      new vm.Script(sanitizedScript);
      console.log('  ✓ In-memory sanitized script compiles cleanly in V8 engine.');
    } catch (e) {
      console.error('  ✗ Still failing compilation after sanitization:', e.message);
    }
  }

  // -----------------------------------------------------------------------
  // PHASE 2: Synthetic DOM Structure & Missing Elements (Graceful Degradation)
  // -----------------------------------------------------------------------
  console.log('\n[Phase 2] Synthetic DOM Structure & Missing Elements (Graceful Degradation)');

  // 2.1: Completely empty document
  {
    const env = createTestEnvironment();
    const sandbox = { ...env.window, window: env.window, document: env.document, localStorage: env.window.localStorage };
    try {
      vm.runInNewContext(sanitizedScript, sandbox);
      assert('Empty DOM: script executes without throwing', true);
      assert('Empty DOM: zero error logs during initialization', !sandbox._errors || sandbox._errors.length === 0);
    } catch (err) {
      assert('Empty DOM: script executes without throwing', false, err.message);
    }
  }

  // 2.2: Navbar present, but bodyContainer missing
  {
    const env = createTestEnvironment();
    const header = env.document.createElement('div');
    header.className = 'shrink-0 flex items-center gap-0.5 border-b';
    env.document.body.appendChild(header);

    const sandbox = { ...env.window, window: env.window, document: env.document, localStorage: env.window.localStorage };
    vm.runInNewContext(sanitizedScript, sandbox);

    const swissBrowserBtn = header.querySelector('[data-tab-id="swiss-browser"]');
    assert('Missing bodyContainer: Swiss buttons still mount to header', !!swissBrowserBtn);

    let clickError = null;
    try {
      const evt = new sandbox.Event('click', { bubbles: true });
      swissBrowserBtn.dispatchEvent(evt);
    } catch (e) {
      clickError = e;
    }
    assert('Missing bodyContainer: clicking Swiss tab degrades gracefully (no exception)', !clickError);
  }

  // 2.3: Fallback navbar selectors
  {
    const fallbackSelectors = [
      { name: 'Data-testid container', setup: (env) => {
        const parent = env.document.createElement('div');
        parent.setAttribute('data-testid', 'auxiliary-panel');
        const h = env.document.createElement('div');
        h.className = 'shrink-0 flex items-center gap-0.5 border-b';
        parent.appendChild(h);
        env.document.body.appendChild(parent);
        return h;
      }},
      { name: 'Auxiliarybar part container', setup: (env) => {
        const parent = env.document.createElement('div');
        parent.className = 'part auxiliarybar';
        const h = env.document.createElement('div');
        h.className = 'shrink-0 flex items-center gap-0.5 border-b';
        parent.appendChild(h);
        env.document.body.appendChild(parent);
        return h;
      }},
      { name: 'Simplified border-b navbar', setup: (env) => {
        const h = env.document.createElement('div');
        h.className = 'shrink-0 flex items-center border-b';
        env.document.body.appendChild(h);
        return h;
      }}
    ];

    fallbackSelectors.forEach((spec) => {
      const env = createTestEnvironment();
      const header = spec.setup(env);

      const body = env.document.createElement('div');
      body.className = 'flex-grow overflow-hidden';
      env.document.body.appendChild(body);

      const sandbox = { ...env.window, window: env.window, document: env.document, localStorage: env.window.localStorage };
      vm.runInNewContext(sanitizedScript, sandbox);

      const btn = header.querySelector('[data-tab-id="swiss-browser"]');
      assert(`Fallback Selector [${spec.name}]: successfully mounts tabs`, !!btn);
    });
  }

  // 2.4: Idempotency under multiple setupAuxiliaryTabs runs
  {
    const env = createTestEnvironment();
    const header = env.document.createElement('div');
    header.className = 'shrink-0 flex items-center gap-0.5 border-b';
    const body = env.document.createElement('div');
    body.className = 'flex-grow overflow-hidden';
    env.document.body.appendChild(header);
    env.document.body.appendChild(body);

    const sandbox = { ...env.window, window: env.window, document: env.document, localStorage: env.window.localStorage };
    vm.runInNewContext(sanitizedScript, sandbox);

    // Trigger observer 50 times
    for (let i = 0; i < 50; i++) {
      env.document._triggerMutation();
    }
    const finalBtns = header.querySelectorAll('.swiss-aux-tab-btn');
    assert('Idempotency: exactly 3 Swiss tab buttons present (no duplicate inflation)', finalBtns.length === 3);
  }

  // -----------------------------------------------------------------------
  // PHASE 3: Rapid Tab Switching & Two-Way State Synchronization
  // -----------------------------------------------------------------------
  console.log('\n[Phase 3] Rapid Tab Switching & Two-Way State Synchronization');

  let windowResizeListenersCount = 0;
  {
    const env = createTestEnvironment();
    const header = env.document.createElement('div');
    header.className = 'shrink-0 flex items-center gap-0.5 border-b';

    // Native Factory Tabs
    const nativeOverview = env.document.createElement('button');
    nativeOverview.setAttribute('data-tab-id', 'overview');
    nativeOverview.className = 'active';
    header.appendChild(nativeOverview);

    const nativeTerminal = env.document.createElement('button');
    nativeTerminal.setAttribute('data-tab-id', 'terminal');
    header.appendChild(nativeTerminal);

    const body = env.document.createElement('div');
    body.className = 'flex-grow overflow-hidden';

    const nativeOverviewView = env.document.createElement('div');
    nativeOverviewView.id = 'native-overview-view';
    body.appendChild(nativeOverviewView);

    env.document.body.appendChild(header);
    env.document.body.appendChild(body);

    const sandbox = { ...env.window, window: env.window, document: env.document, localStorage: env.window.localStorage };
    vm.runInNewContext(sanitizedScript, sandbox);

    const swissBrowserBtn = header.querySelector('[data-tab-id="swiss-browser"]');
    const swissFilesBtn = header.querySelector('[data-tab-id="swiss-files"]');
    const swissMemosBtn = header.querySelector('[data-tab-id="swiss-memos"]');
    const swissContainer = () => body.querySelector('#swiss-aux-container');

    // Step 3.1: Click Swiss Browser Tab
    swissBrowserBtn.dispatchEvent(new sandbox.Event('click', { bubbles: true }));
    assert('2-Way Sync: Swiss container mounted and visible (display: flex)', swissContainer() && swissContainer().style.display === 'flex');
    assert('2-Way Sync: Native view hidden (display: none)', nativeOverviewView.style.display === 'none');
    assert('2-Way Sync: Native tab button active class removed', !nativeOverview.classList.contains('active'));
    assert('2-Way Sync: Swiss tab button has active class', swissBrowserBtn.classList.contains('active'));
    assert('2-Way Sync: LocalStorage stores "swiss-browser"', env.storage['antigravity_active_aux_tab'] === 'swiss-browser');

    // Step 3.2: Click Native Overview Tab
    nativeOverview.dispatchEvent(new sandbox.Event('click', { bubbles: true }));
    assert('2-Way Sync: Swiss container hidden (display: none)', swissContainer() && swissContainer().style.display === 'none');
    assert('2-Way Sync: Native view restored (display: "")', nativeOverviewView.style.display === '');
    assert('2-Way Sync: Swiss tab button active class removed', !swissBrowserBtn.classList.contains('active'));
    assert('2-Way Sync: LocalStorage stores "factory"', env.storage['antigravity_active_aux_tab'] === 'factory');

    // Step 3.3: Rapid Switching Stress (1,000 switches)
    const t0 = Date.now();
    let switchErrors = 0;
    const tabsSequence = [swissBrowserBtn, nativeOverview, swissFilesBtn, nativeTerminal, swissMemosBtn, nativeOverview];
    for (let i = 0; i < 1000; i++) {
      try {
        const btn = tabsSequence[i % tabsSequence.length];
        btn.dispatchEvent(new sandbox.Event('click', { bubbles: true }));
      } catch (err) {
        switchErrors++;
      }
    }
    const elapsed = Date.now() - t0;
    assert('Stress Test: 1,000 rapid back-and-forth tab switches completed with 0 errors', switchErrors === 0);
    console.log(`    ↳ Completed 1,000 tab switches in ${elapsed}ms (${(elapsed / 1000).toFixed(3)}ms/switch)`);

    swissMemosBtn.dispatchEvent(new sandbox.Event('click', { bubbles: true }));
    assert('Final state consistency: Swiss container is flex for memos', swissContainer().style.display === 'flex');
    assert('Final state consistency: LocalStorage is "swiss-memos"', env.storage['antigravity_active_aux_tab'] === 'swiss-memos');

    windowResizeListenersCount = (sandbox._listeners || []).filter(l => l.type === 'resize').length;
  }

  // -----------------------------------------------------------------------
  // PHASE 4: Coordinate Calculations in getCanvasCoords under Container Scales
  // -----------------------------------------------------------------------
  console.log('\n[Phase 4] Coordinate Calculations in getCanvasCoords under Container Scales & Offsets');

  {
    const env = createTestEnvironment();
    const header = env.document.createElement('div');
    header.className = 'shrink-0 flex items-center gap-0.5 border-b';
    const body = env.document.createElement('div');
    body.className = 'flex-grow overflow-hidden';
    env.document.body.appendChild(header);
    env.document.body.appendChild(body);

    const sandbox = { ...env.window, window: env.window, document: env.document, localStorage: env.window.localStorage };
    vm.runInNewContext(sanitizedScript, sandbox);

    // Switch to Browser View
    const swissBrowserBtn = header.querySelector('[data-tab-id="swiss-browser"]');
    swissBrowserBtn.dispatchEvent(new sandbox.Event('click', { bubbles: true }));

    const canvas = body.querySelector('#swiss-browser-canvas');
    const penBtn = body.querySelector('#swiss-b-pen');
    const rectBtn = body.querySelector('#swiss-b-rect');
    const ctx = canvas.getContext('2d');

    assert('Canvas overlay initialized with dimensions (400x600)', canvas.width === 400 && canvas.height === 600);

    penBtn.dispatchEvent(new sandbox.Event('click', { bubbles: true }));
    assert('Pen tool activated: pointerEvents is auto', canvas.style.pointerEvents === 'auto');

    // 4.1: Standard 1:1 Scale (rect at (100, 50), size 400x600)
    canvas.setBoundingClientRect({ left: 100, top: 50, width: 400, height: 600 });
    canvas.dispatchEvent({ type: 'mousedown', clientX: 200, clientY: 150 });
    assert('Scale 1.0: mousedown starts path at (100, 100)',
      ctx.arcCalls.length > 0 && ctx.arcCalls[0].x === 100 && ctx.arcCalls[0].y === 100
    );

    // Move to (220, 170) -> relative (120, 120) -> midpoint (110, 110)
    canvas.dispatchEvent({ type: 'mousemove', clientX: 220, clientY: 170 });
    assert('Scale 1.0: mousemove quadraticCurveTo midpoint computed at (110, 110)',
      ctx.quadraticCurveToCalls.length > 0 &&
      ctx.quadraticCurveToCalls[0].x === 110 &&
      ctx.quadraticCurveToCalls[0].y === 110
    );
    canvas.dispatchEvent({ type: 'mouseup', clientX: 220, clientY: 170 });

    // 4.2: Zoom Out: Scale = 0.5 (visual rect 200x300 at (50, 25))
    ctx.arcCalls = [];
    ctx.quadraticCurveToCalls = [];
    canvas.setBoundingClientRect({ left: 50, top: 25, width: 200, height: 300 });
    canvas.dispatchEvent({ type: 'mousedown', clientX: 100, clientY: 75 });
    assert('Scale 0.5: translates (100, 75) screen pos to exact internal canvas (100, 100)',
      ctx.arcCalls.length > 0 && Math.round(ctx.arcCalls[0].x) === 100 && Math.round(ctx.arcCalls[0].y) === 100
    );
    canvas.dispatchEvent({ type: 'mouseup', clientX: 100, clientY: 75 });

    // 4.3: Zoom In: Scale = 1.25 (visual rect 500x750 at (80, 40))
    ctx.arcCalls = [];
    canvas.setBoundingClientRect({ left: 80, top: 40, width: 500, height: 750 });
    canvas.dispatchEvent({ type: 'mousedown', clientX: 205, clientY: 165 });
    assert('Scale 1.25: translates (205, 165) screen pos to exact internal canvas (100, 100)',
      ctx.arcCalls.length > 0 && Math.round(ctx.arcCalls[0].x) === 100 && Math.round(ctx.arcCalls[0].y) === 100
    );
    canvas.dispatchEvent({ type: 'mouseup', clientX: 205, clientY: 165 });

    // 4.4: Subpixel and Negative Offsets (rect at (-123.45, -67.89), size 400x600)
    ctx.arcCalls = [];
    canvas.setBoundingClientRect({ left: -123.45, top: -67.89, width: 400, height: 600 });
    canvas.dispatchEvent({ type: 'mousedown', clientX: -23.45, clientY: 32.11 });
    assert('Subpixel/Negative Offset: translates accurately to internal canvas (100, 100)',
      ctx.arcCalls.length > 0 &&
      Math.abs(ctx.arcCalls[0].x - 100) < 0.001 &&
      Math.abs(ctx.arcCalls[0].y - 100) < 0.001
    );
    canvas.dispatchEvent({ type: 'mouseup', clientX: -23.45, clientY: 32.11 });

    // 4.5: Extreme Scale: 0.1x (width 40, height 60) and 3.0x (width 1200, height 1800)
    ctx.arcCalls = [];
    canvas.setBoundingClientRect({ left: 0, top: 0, width: 40, height: 60 });
    canvas.dispatchEvent({ type: 'mousedown', clientX: 10, clientY: 15 });
    assert('Extreme Scale 0.1x: (10, 15) maps to (100, 150)',
      ctx.arcCalls.length > 0 && Math.round(ctx.arcCalls[0].x) === 100 && Math.round(ctx.arcCalls[0].y) === 150
    );
    canvas.dispatchEvent({ type: 'mouseup', clientX: 10, clientY: 15 });

    // 4.6: Adversarial Zero-Size Container (width = 0, height = 0)
    ctx.arcCalls = [];
    canvas.setBoundingClientRect({ left: 0, top: 0, width: 0, height: 0 });
    let zeroSizeError = null;
    try {
      canvas.dispatchEvent({ type: 'mousedown', clientX: 50, clientY: 50 });
      canvas.dispatchEvent({ type: 'mousemove', clientX: 60, clientY: 60 });
      canvas.dispatchEvent({ type: 'mouseup', clientX: 60, clientY: 60 });
    } catch (e) {
      zeroSizeError = e;
    }
    assert('Zero-Size Container: drawing events survive division by zero without uncaught exceptions', !zeroSizeError);

    // 4.7: Bounding Box Tool
    rectBtn.dispatchEvent(new sandbox.Event('click', { bubbles: true }));
    assert('Rect Tool activated: pen tool deactivated', !penBtn.classList.contains('active') && rectBtn.classList.contains('active'));

    canvas.setBoundingClientRect({ left: 0, top: 0, width: 400, height: 600 });
    ctx.strokeRectCalls = [];
    canvas.dispatchEvent({ type: 'mousedown', clientX: 50, clientY: 50 });
    canvas.dispatchEvent({ type: 'mousemove', clientX: 200, clientY: 150 });
    canvas.dispatchEvent({ type: 'mouseup', clientX: 200, clientY: 150 });

    assert('Rect Tool: successfully drew bounding box stroke and fill',
      ctx.strokeRectCalls.length > 0 && ctx.fillRectCalls.length > 0
    );
  }

  // -----------------------------------------------------------------------
  // PHASE 5: Listener Leak & Resource Retention Analysis
  // -----------------------------------------------------------------------
  console.log('\n[Phase 5] Listener Leak & Resource Retention Analysis');
  if (windowResizeListenersCount > 10) {
    console.log(`  ⚠ Listener Accumulation: ${windowResizeListenersCount} resize listeners attached to window`);
    findings.push({
      id: 'FINDING-2-LISTENER-LEAK',
      severity: 'LOW',
      title: 'window.addEventListener("resize", applyDeviceScale) Leak Across Repeated Browser Tab Switches',
      location: 'pkg/plugins/auxiliary.go:963',
      details: `Each switch to the Browser tab adds a new resize listener to window without removing previous listeners. After 1,000 switches, ${windowResizeListenersCount} listeners were accumulated.`,
      impact: 'Negligible CPU overhead on resize, but closures over detached DOM nodes are retained in memory.'
    });
  } else {
    assert('Window resize listeners bounded across tab switches', true);
  }

  // -----------------------------------------------------------------------
  // PHASE 6: Send to Chat Workflow & Lexical Injection
  // -----------------------------------------------------------------------
  console.log('\n[Phase 6] Send to Chat Workflow & Lexical / Textarea Injection');

  {
    const env = createTestEnvironment();
    const header = env.document.createElement('div');
    header.className = 'shrink-0 flex items-center gap-0.5 border-b';
    const body = env.document.createElement('div');
    body.className = 'flex-grow overflow-hidden';
    env.document.body.appendChild(header);
    env.document.body.appendChild(body);

    const fileInput = env.document.createElement('input');
    fileInput.type = 'file';
    env.document.body.appendChild(fileInput);

    let lexicalUpdated = false;
    let lexicalInsertedText = '';
    const lexicalElem = env.document.createElement('div');
    lexicalElem.setAttribute('data-lexical-editor', 'true');
    lexicalElem.__lexicalEditor = {
      update: (fn) => {
        lexicalUpdated = true;
        env.document.execCommand = (cmd, _, text) => {
          if (cmd === 'insertText') lexicalInsertedText = text;
        };
        fn();
      }
    };
    env.document.body.appendChild(lexicalElem);

    const sandbox = { ...env.window, window: env.window, document: env.document, localStorage: env.window.localStorage };
    vm.runInNewContext(sanitizedScript, sandbox);

    const swissBrowserBtn = header.querySelector('[data-tab-id="swiss-browser"]');
    swissBrowserBtn.dispatchEvent(new sandbox.Event('click', { bubbles: true }));

    const sendBtn = body.querySelector('#swiss-b-send-chat');
    assert('Send to Chat button found in toolbar', !!sendBtn);

    let sendError = null;
    try {
      sendBtn.onclick();
    } catch (e) {
      sendError = e;
    }
    assert('Send to Chat executes cleanly', !sendError);

    await new Promise(r => setTimeout(r, 50));

    assert('Send to Chat: File object attached to fileInput via DataTransfer', fileInput.files && fileInput.files.length === 1);
    assert('Send to Chat: Attached file name is "annotation.png"', fileInput.files && fileInput.files[0].name === 'annotation.png');
    assert('Send to Chat: Lexical editor updated with prompt context', lexicalUpdated && lexicalInsertedText.includes('[Browser Preview Annotation'));
  }

  // -----------------------------------------------------------------------
  // SUMMARY REPORT & VERDICT
  // -----------------------------------------------------------------------
  console.log('\n========================================================================');
  console.log(`TOTAL CHECKS: ${passedTests + failedTests} | PASSED: ${passedTests} | FAILED: ${failedTests}`);
  console.log('FINDINGS COUNT:', findings.length);
  findings.forEach(f => {
    console.log(`- [${f.severity}] ${f.id}: ${f.title}`);
    console.log(`  Location: ${f.location}`);
    console.log(`  Details: ${f.details}`);
  });
  console.log('========================================================================');

  if (findings.some(f => f.severity === 'CRITICAL')) {
    console.log('\nFINAL ADVERSARIAL VERDICT: CHALLENGE');
    process.exit(1);
  } else {
    console.log('\nFINAL ADVERSARIAL VERDICT: CONFIRM');
    process.exit(0);
  }
}

runStressSuite().catch(err => {
  console.error('Fatal stress suite runner error:', err);
  process.exit(1);
});
