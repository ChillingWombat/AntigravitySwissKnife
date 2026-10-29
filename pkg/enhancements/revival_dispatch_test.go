package enhancements

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// revivalHandlerMarkers bound the injected auto-revival handler inside the
// generated script. Extracting the exact slice keeps this test honest: if the
// handler is restructured, the markers move and the test fails loudly instead of
// silently asserting nothing.
const (
	revivalHandlerStart = "/* === Active Conversation Auto-Revival & Continuation Handler === */"
	revivalHandlerEnd   = "window.__swissCheckRevival = checkAndExecuteConversationRevival;"
)

// extractRevivalHandler returns the auto-revival handler slice of the script.
func extractRevivalHandler(t *testing.T, script string) string {
	t.Helper()
	start := strings.Index(script, revivalHandlerStart)
	if start < 0 {
		t.Fatalf("revival handler marker %q not found in the generated script", revivalHandlerStart)
	}
	end := strings.Index(script[start:], revivalHandlerEnd)
	if end < 0 {
		t.Fatalf("revival handler export marker %q not found after the handler start", revivalHandlerEnd)
	}
	return script[start : start+end+len(revivalHandlerEnd)]
}

// TestRevivalDispatch_OnePromptPerInterruptedConversation drives the injected
// in-page auto-revival handler against a multi-intent payload and asserts the
// property the screenshot exposed: every interrupted conversation receives
// EXACTLY ONE prompt dispatch, even across repeated passes and even when the Go
// CDP trigger already dispatched some of the same intents.
func TestRevivalDispatch_OnePromptPerInterruptedConversation(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())
	body := extractRevivalHandler(t, script)

	convA := "a0000000-0000-4000-8000-00000000000a"
	convB := "b0000000-0000-4000-8000-00000000000b"
	convC := "c0000000-0000-4000-8000-00000000000c"

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal revival handler: %v", err)
	}

	harness := fmt.Sprintf(`const vm = require('vm');

const CONV_A = %q, CONV_B = %q, CONV_C = %q;
const PROMPT = 'Please continue ongoing tasks and subagents.';
const HANDLER = %s;

function makeIntent(conv, id, now) {
  return {
    intent_id: id, root_conversation_id: conv, cascade_id: conv,
    trigger_prompt: PROMPT, prompt: PROMPT,
    created_at: new Date(now).toISOString(), timestamp: now,
    status: 'pending', resumed: false, ttl_seconds: 600, max_attempts: 2
  };
}

function makeStorage() {
  const data = new Map();
  return {
    getItem: (k) => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => data.set(k, String(v)),
    removeItem: (k) => data.delete(k),
    clear: () => data.clear()
  };
}

function boot(intents, options = {}) {
  const sends = [];
  const acks = [];
  let editorText = options.initialEditorText || '';
  let servedIntents = intents.map(it => Object.assign({}, it));

  // Deterministic clock: the handler throttles on Date.now() (1.5s check floor)
  // and on setTimeout (80ms send retries). Both are injected so the test asserts
  // real dispatch logic without paying real wall-clock time.
  const clock = { now: options.now || Date.now() };
  class FakeDate extends Date {
    constructor(...args) { if (args.length === 0) { super(clock.now); } else { super(...args); } }
    static now() { return clock.now; }
  }
  const fastTimeout = (fn) => setTimeout(fn, 0);

  class Event { constructor(t, o = {}) { this.type = t; this.bubbles = !!o.bubbles; this.cancelable = !!o.cancelable; } }
  class CustomEvent extends Event {}
  class InputEvent extends Event { constructor(t, o = {}) { super(t, o); this.data = o.data; this.inputType = o.inputType; } }
  class KeyboardEvent extends Event { constructor(t, o = {}) { super(t, o); this.key = o.key; this.keyCode = o.keyCode; } }
  class MouseEvent extends Event {}
  class PointerEvent extends MouseEvent {}

  const editor = {
    isContentEditable: true,
    get innerText() { return editorText; },
    set innerText(v) { editorText = v; },
    get value() { return editorText; },
    set value(v) { editorText = v; },
    focus: () => {},
    dispatchEvent: (e) => {
      if (e.type === 'beforeinput' && e.data) editorText = e.data;
      if (e.type === 'beforeinput' && e.inputType === 'deleteContentBackward') editorText = '';
      return true;
    },
    closest: () => null,
    parentElement: null
  };

  const sendBtn = {
    disabled: false, type: 'button',
    getAttribute: (a) => (a === 'data-testid' ? 'send-button' : null),
    removeAttribute: () => {},
    dispatchEvent: () => true,
    click: () => { sends.push({ text: editorText }); editorText = ''; }
  };

  let currentPath = '/c/' + (options.routeConv || CONV_A);
  const navigate = (to) => { currentPath = String(to).split(/[?#]/)[0]; };
  const navLinks = {};
  for (const c of [CONV_A, CONV_B, CONV_C]) {
    navLinks[c] = { click: () => navigate('/c/' + c), tagName: 'A' };
  }

  const doc = {
    location: {
      get pathname() { return currentPath; },
      set pathname(v) { currentPath = v; },
      assign: (to) => navigate(to),
      href: '/'
    },
    querySelector: (sel) => {
      if (sel.indexOf('conversation-view') !== -1) return { tagName: 'DIV' };
      if (sel.indexOf('Send now') !== -1 || sel.indexOf('send-button-pending') !== -1 || sel.indexOf('cancel') !== -1 ||
          sel.indexOf('stop') !== -1 || sel.indexOf('queued') !== -1 || sel.indexOf('interaction-continue') !== -1) return null;
      if (sel.indexOf('contenteditable') !== -1 || sel.indexOf('Message input') !== -1 || sel.indexOf('agent-input-box') !== -1) return editor;
      if (sel.indexOf('send-button') !== -1 || sel.indexOf('send-tooltip') !== -1 || sel.indexOf('Send') !== -1) return sendBtn;
      const linkMatch = sel.match(/^a\[href\*="([^"]+)"\]$/);
      if (linkMatch) return navLinks[linkMatch[1]] || null;
      return null;
    },
    querySelectorAll: (sel) => (sel === 'button' ? [sendBtn] : []),
    execCommand: (cmd, ui, val) => { if (cmd === 'insertText') editorText = val; if (cmd === 'delete') editorText = ''; return true; },
    getSelection: () => ({ removeAllRanges: () => {}, addRange: () => {} }),
    createRange: () => ({ selectNodeContents: () => {} })
  };

  const localStorage = makeStorage();
  const sessionStorage = makeStorage();

  const win = {
    location: doc.location,
    __TSR_ROUTER__: { navigate: ({ to }) => navigate(to) },
    document: doc,
    getSelection: doc.getSelection,
    __swissDispatchedIntents: options.dispatchedIntents || {},
    height: 0,
    innerWidth: 1200,
    __swissPendingContinuations: intents.map(it => Object.assign({}, it)),
    fetch: async (url, opts) => {
      if (url.indexOf('/api/conversations/status') !== -1) {
        return { ok: true, json: async () => ({ success: true, pending_intents: servedIntents.map(it => Object.assign({}, it)) }) };
      }
      if (url.indexOf('/api/conversations/ack') !== -1) {
        const payload = JSON.parse((opts && opts.body) || '{}');
        acks.push(payload);
        const key = payload.intent_id || '';
        servedIntents = servedIntents.filter(it => (key ? it.intent_id !== key : it.root_conversation_id !== payload.cascade_id));
        return { ok: true, json: async () => ({ success: true }) };
      }
      return { ok: false, json: async () => ({}) };
    },
    postMessage: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    getSelection: doc.getSelection
  };
  doc.addEventListener = () => {};
  doc.removeEventListener = () => {};

  const sandbox = {
    window: win, document: doc, localStorage, sessionStorage,
    console, setTimeout: fastTimeout, clearTimeout, setInterval: () => 0, clearInterval: () => {},
    Event, CustomEvent, InputEvent, KeyboardEvent, MouseEvent, PointerEvent,
    Promise, JSON, Array, Object, Map, Boolean, String, Number, Math, Date: FakeDate,
    encodeURIComponent, decodeURIComponent, fetch: win.fetch, RegExp, Error, isNaN,
    requestAnimationFrame: (fn) => setTimeout(fn, 0), MutationObserver: class { observe() {} disconnect() {} }
  };
  vm.createContext(sandbox);
  vm.runInContext(HANDLER, sandbox);

  // flush drains the handler's pending timer callbacks. The handler types the
  // prompt, then finishes the send in a fire-and-forget async loop; without
  // draining, the assertion would run before the send loop even starts.
  // Timers are immediate, so a bounded microtask drain is enough to let the
  // send loop finish; 400 iterations made the whole suite needlessly slow.
  const flush = async () => {
    for (let i = 0; i < 40; i++) {
      await new Promise(r => setTimeout(r, 0));
    }
  };

  return {
    win, doc, editor, sends, acks, clock,
    check: async () => { await win.__swissCheckRevival(); await flush(); }
  };
}

// pass advances the injected clock past the handler's 1.5s check floor between
// passes, mirroring the page's 2.5s interval without real waiting.
async function pass(session, rounds, stopWhen) {
  for (let i = 0; i < rounds; i++) {
    await session.check();
    session.clock.now += 2000;
    if (stopWhen && stopWhen(session)) return;
  }
}

async function run() {
  const T0 = Date.UTC(2026, 0, 1, 12, 0, 0);

  // Case 1: three interrupted conversations -> exactly three prompts, three scoped acks.
  {
    const s = boot([
      makeIntent(CONV_A, 'intent-1-' + CONV_A, T0),
      makeIntent(CONV_B, 'intent-2-' + CONV_B, T0),
      makeIntent(CONV_C, 'intent-3-' + CONV_C, T0)
    ], { routeConv: CONV_A, now: T0 });
    await pass(s, 40, (x) => x.sends.length >= 3 && x.acks.length >= 3);

    if (s.sends.length !== 3) {
      throw new Error('expected exactly 3 sends (one per interrupted conversation), got ' + s.sends.length + ': ' + JSON.stringify(s.sends));
    }
    for (const send of s.sends) {
      if (send.text !== PROMPT) throw new Error('unexpected prompt text dispatched: ' + JSON.stringify(send.text));
    }
    if (s.acks.length !== 3) throw new Error('expected exactly 3 acks, got ' + s.acks.length + ': ' + JSON.stringify(s.acks));
    const keys = s.acks.map(a => a.intent_id).filter(Boolean);
    if (new Set(keys).size !== 3) throw new Error('acks must be scoped to distinct intent ids, got ' + JSON.stringify(keys));

    // No further prompt once everything was acked.
    await pass(s, 20);
    if (s.sends.length !== 3) throw new Error('post-ack passes re-dispatched a prompt: ' + s.sends.length);
  }

  // Case 2: intents the Go CDP trigger already dispatched are never re-sent.
  {
    const s = boot([
      makeIntent(CONV_A, 'intent-1-' + CONV_A, T0),
      makeIntent(CONV_B, 'intent-2-' + CONV_B, T0)
    ], { routeConv: CONV_A, now: T0, dispatchedIntents: { ['intent-1-' + CONV_A]: T0 } });
    await pass(s, 20, (x) => x.sends.length >= 1);
    await pass(s, 20);

    if (s.sends.length !== 1) {
      throw new Error('an intent already dispatched by the Go trigger must not be re-sent; expected 1 send, got ' + s.sends.length);
    }
  }

  // Case 3: the daemon re-publishing an already-sent intent adds no prompt.
  {
    const s = boot([makeIntent(CONV_A, 'intent-1-' + CONV_A, T0)], { routeConv: CONV_A, now: T0 });
    await pass(s, 20, (x) => x.sends.length >= 1);
    if (s.sends.length !== 1) throw new Error('expected 1 send, got ' + s.sends.length);
    await pass(s, 20);
    if (s.sends.length !== 1) throw new Error('repeated pass re-dispatched an already-sent intent: ' + s.sends.length);
  }

  // Case 4: a stale resume prompt left unsent in the composer is cleared, never sent.
  {
    const s = boot([makeIntent(CONV_A, 'intent-1-' + CONV_A, T0)], {
      routeConv: CONV_A,
      now: T0,
      dispatchedIntents: { ['intent-1-' + CONV_A]: T0 },
      initialEditorText: PROMPT
    });
    await pass(s, 20);
    if (s.sends.length !== 0) throw new Error('a fully dispatched intent must never be sent, got ' + s.sends.length);
    if ((s.editor.innerText || '').trim().length !== 0) {
      throw new Error('stale resume prompt left unsent in the composer was not cleared: ' + JSON.stringify(s.editor.innerText));
    }
  }

  // Case 5: a genuine user draft is untouched.
  {
    const s = boot([makeIntent(CONV_A, 'intent-1-' + CONV_A, T0)], { routeConv: CONV_A, now: T0, initialEditorText: 'my own unpublished question' });
    await pass(s, 10);
    if (s.sends.length !== 0) throw new Error('user draft protection broken: ' + s.sends.length + ' sends');
    if (s.editor.innerText !== 'my own unpublished question') {
      throw new Error('user draft was modified: ' + JSON.stringify(s.editor.innerText));
    }
  }

  console.log('All multi-intent dispatch assertions passed');
}

run().catch(err => { console.error(err && err.stack ? err.stack : err); process.exit(1); });
`, convA, convB, convC, string(encoded))

	scriptPath := filepath.Join(t.TempDir(), "multi_intent_dispatch.js")
	if err := os.WriteFile(scriptPath, []byte(harness), 0644); err != nil {
		t.Fatalf("failed to write harness: %v", err)
	}

	out, runErr := exec.Command("node", scriptPath).CombinedOutput()
	if runErr != nil {
		t.Fatalf("multi-intent dispatch harness failed: %v\nOutput:\n%s", runErr, string(out))
	}
	t.Logf("dispatch harness output: %s", strings.TrimSpace(string(out)))
}
