# Reverse-engineering report: `svkozak/pi-acp`

Source: `git clone https://github.com/svkozak/pi-acp` @ `04d0a15d6bb42bb65cce5c9caedcdcb2cdcb6905` (v0.0.34, 157 commits).
ACE MCP `rea` server exposes only mobile/native artifact inspectors (APK, PE, Flutter, JEB) — no JS/source-repo analyzer — so evidence below is from the cloned tree, the ACP SDK tarball (`@agentclientprotocol/sdk@0.26.0`), and the GitHub issues API.

Purpose: inform `Antigravity-ACP` (Go), which must expose the Antigravity 2.0 app, `agy` CLI and the VS Code extension over ACP.

---

## 0. TL;DR

`pi-acp` is a **thin agent-side ACP server** that is a *transport adapter*, not an agent. It speaks ACP JSON-RPC 2.0 over stdio to the editor and speaks pi's own newline-delimited JSON RPC over stdio to a spawned `pi --mode rpc` child. All intelligence, tools, credentials, sessions and persistence stay in pi; the adapter's only job is event/format translation plus a handful of headless slash commands.

Codebase is small and legible: **4,427 LOC across 20 files** in `src/`.

| Layer | Files | LOC |
|---|---|---|
| ACP protocol surface | `src/acp/agent.ts` | 1655 |
| ACP session/turn state machine | `src/acp/session.ts` | 1140 |
| pi RPC subprocess | `src/pi-rpc/process.ts` | 432 |
| pi session file scanning | `src/acp/pi-sessions.ts` | 341 |
| Pure translators | `src/acp/translate/*.ts`, `slash-commands.ts`, `pi-commands.ts`, `pi-settings.ts`, `auth*.ts`, `paths.ts`, `session-store.ts` | ~800 |
| Entry | `src/index.ts` | 82 |

---

## 1. Architecture: agent side only

**Agent side (ACP server).** `PiAcpAgent implements ACPAgent` (`src/acp/agent.ts:225`) is registered as the agent handler; no client-side implementation exists (no `fs/*`, no `terminal/*` calls anywhere in the tree).

**Entry point:** `src/index.ts:60` — `new AgentSideConnection(conn => new PiAcpAgent(conn), stream)`. Before that, `src/index.ts:9` short-circuits: if `argv` contains `--terminal-login` the process never starts an ACP loop; it `spawnSync`s `pi` with inherited stdio and exits with pi's status (the "Terminal Auth" escape hatch).

Three-layer split, mirrored in `AGENTS.md`:

```
editor ──ACP JSON-RPC/stdio──> src/index.ts ──> PiAcpAgent (src/acp/agent.ts)
                                                    │
                                            SessionManager (src/acp/session.ts)
                                                    │  one PiRpcProcess per ACP session
                                            PiRpcProcess (src/pi-rpc/process.ts)
                                                    │  NDJSON/stdio
                                            pi --mode rpc  (child process)
```

The rule "keep ACP handling in `src/acp/*`, pi transport in `src/pi-rpc/*`, translation as small pure functions" is stated in `AGENTS.md` and actually holds — `src/acp/translate/{prompt,pi-messages,pi-tools,bash}.ts` are pure and unit-tested (`test/unit/*`).

---

## 2. Exact protocol behaviour

### 2.1 Transport and framing

- ACP over **stdio, newline-delimited JSON** (`application/json` lines, one JSON-RPC 2.0 message per line). No `Content-Length` framing. Implemented by the SDK's `ndJsonStream` (`sdk/dist/stream.js`), which splits on `\n`, trims, `JSON.parse`s each line, and silently logs+skips unparseable lines.
- Naming trap in `src/index.ts:59`: pi-acp calls `ndJsonStream(input, output)` with its *own* variable names, but the SDK signature is `ndJsonStream(output, input)` — arg 1 is the **sink** (a `WritableStream` writing to `process.stdout`), arg 2 is the **source** (`ReadableStream` fed from `process.stdin`). Correct at runtime, inverted by name.
- `stdout` writes are defensive: destroyed/non-writable stdout resolves instead of throwing (`src/index.ts:27-39`), so a vanished editor does not crash the agent.
- Shutdown: `stdin end`/`close`, `SIGINT`, `SIGTERM` → best-effort `agent.dispose()` → `sessions.disposeAll()` → `SIGTERM` every child.

### 2.2 JSON-RPC methods implemented

Registered by the SDK switch (`sdk/dist/acp.js`) against `AGENT_METHODS` (`sdk/dist/schema/index.js`); `PROTOCOL_VERSION = 1`.

| Method | Handler | Behaviour |
|---|---|---|
| `initialize` | `agent.ts:239` | always replies `protocolVersion: 1` (coerces anything else to 1), `agentInfo`, `authMethods`, capabilities |
| `authenticate` | `agent.ts:443` | **no-op success** — auth is out-of-band via `--terminal-login` |
| `session/new` | `agent.ts:274` | spawn a fresh `pi --mode rpc`, derive sessionId from pi, return config |
| `session/load` | `agent.ts:932` | respawn pi against a stored session file, replay history as notifications |
| `session/prompt` | `agent.ts:449` | prompt turn; intercepts built-in slash commands |
| `session/cancel` | `agent.ts:897` | notification; clears queue + `{type:"abort"}` |
| `session/list` | `agent.ts:903` | scans pi's `*.jsonl` session files, 50/page numeric-offset cursor |
| `session/delete` | `agent.ts:1121` | unlinks the pi session file, idempotent |
| `session/set_mode` | `agent.ts:1150` | maps modeId → pi thinking level |
| `session/set_config_option` | `agent.ts:1170` | `model` / `thought_level` |
| `session/set_model` | **not routed** | `unstable_setSessionModel` (`agent.ts:1147`) is dead code: SDK 0.26.0 has no `session/set_model` in `AGENT_METHODS` |

Not implemented: `providers/*`, `session/fork`, `session/resume`, `session/close`, `logout`, `nes/*`, `document/*`, `mcp/message` (tool-side).

### 2.3 `initialize` — capability advertisement

```ts
protocolVersion: requested === 1 ? 1 : 1            // agent.ts:241-247
agentInfo: { name, title: 'pi ACP adapter', version } // read from nearest package.json
authMethods: getAuthMethods({ supportsTerminalAuthMeta:
  params.clientCapabilities?._meta?['terminal-auth'] === true })
agentCapabilities: {
  loadSession: true,
  mcpCapabilities: { http: false, sse: false },
  promptCapabilities: { image: true, audio: false,
    embeddedContext: process.env.PI_ACP_ENABLE_EMBEDDED_CONTEXT === 'true' },
  sessionCapabilities: { list: {}, delete: {} }     // marked UNSTABLE in code
}
```

Notes worth copying: (a) capability flags are derived from the *client's* advertised `_meta`, not hardcoded; (b) `embeddedContext` is off by default and gated by an env var; (c) `image: true` is honest — images are forwarded as base64 to pi.

### 2.4 `session/new`

1. Rejects non-absolute `cwd` (`invalidParams`). `cwd` becomes the default listing filter.
2. Loads file slash commands from `~/.pi/agent/prompts/**/*.md` and `<cwd>/.pi/prompts/**/*.md`; reads `enableSkillCommands` from merged pi settings.
3. `SessionManager.create` → `PiRpcProcess.spawn({cwd})` → `pi --mode rpc --no-themes` (`process.ts:184`).
4. Reads `get_state()` for `sessionId`/`sessionFile`; **the ACP sessionId is pi's own sessionId** (`session.ts:238`). Falls back to `crypto.randomUUID()`.
5. Persists `{sessionId, cwd, sessionFile}` into `~/.pi/pi-acp/session-map.json` (write-through on every upsert).
6. Fetches `get_state` + `get_available_models` **in parallel** to cut startup latency.
7. **Auth gate:** if the models list is empty → `RequestError.authRequired({authMethods}, 'Configure an API key…')`; spawn failures → `internalError({code})`. Failed sessions are torn down *and their pi session file unlinked* (`cleanupFailedNewSession`).
8. `closeAllExcept(newSessionId)` (`agent.ts:384`) — policy: **one live pi child per ACP connection** (one editor window = one connection).
9. Response: `{ sessionId, configOptions, models, modes, _meta.piAcp.startupInfo }`. `configOptions` is the current schema; `models`/`modes` are legacy fields that strict clients drop (the schema for `NewSessionResponse` has no `models`).
10. **Deferred notifications** via `setTimeout(…, 0)` (`agent.ts:406`, `:961`): `usage_update` then `available_commands_update`. The comment is explicit — Zed ignores `session/update` for a sessionId it has not yet learned from the response. This ordering constraint is real and must be reproduced.

### 2.5 `session/prompt`

```
promptToPiMessage(blocks) -> { message, images }        // translate/prompt.ts
if (text starts with '/' && no images) -> built-in command path, return end_turn
else -> session.prompt(message, images, titleHint)
```

- `promptToPiMessage`: `text` concatenated verbatim; `resource_link` → `\n[Context] <uri>`; `image` → base64 passthrough (data URL prefix stripped); `resource` (text) → `\n[Embedded Context] <uri> (<mime>)\n<text>`; blob → size note only; `audio` → explicit "not supported" marker rather than silent drop.
- Built-ins handled entirely adapter-side with no model call: `compact`, `autocompact`, `session` (stats), `name`, `steering`, `follow-up`, `changelog` (reads installed pi `CHANGELOG.md`, 20 000-char cap), `export` (guards against pi's uncorrelated parse error on empty session files, then emits a `resource_link` to the HTML).
- File-based commands are *not* handled here: pi RPC mode disables slash expansion, so `expandSlashCommand` runs inside `session.prompt` (`session.ts:370`).
- **Turn completion does not use the RPC reply.** `proc.prompt()` only acknowledges acceptance. The turn resolves on pi's `agent_settled` event, because retry/compaction/queued continuations emit multiple `turn_end` + `agent_end` before settling (`session.ts:956-967`, comment at `session.ts:308-312`).
- Concurrency: a second prompt while one is in flight is **queued client-side** (`turnQueue`), with queue depth published as `session_info_update._meta.piAcp.queueDepth` (the code notes Zed does not render it).
- `stopReason`: pi errors map to `cancelled` if `cancelRequested` else `end_turn` — ACP has no `error` stop reason. This loses token-limit and compaction-abort semantics (open issues #147, #148).

### 2.6 Streaming `session/update`

35 emit sites (`this.conn.sessionUpdate`), one `requestPermission`. Kinds emitted:

| Kind | Source |
|---|---|
| `agent_message_chunk` | `message_update.assistantMessageEvent.text_delta`; also auto-retry/compaction status lines, startup info, slash-command output |
| `agent_thought_chunk` | `thinking_delta` (a real thought stream exists despite the README) |
| `user_message_chunk` | `session/load` replay only |
| `tool_call` / `tool_call_update` | `toolcall_start|delta|end`, `tool_execution_start|update|end` |
| `session_info_update` | title changes + queue metadata |
| `available_commands_update` | after `session/new` / `load` |
| `usage_update` | pi `get_session_stats().contextUsage` |
| `current_mode_update`, `config_option_update` | model/thinking changes |

**Ordering guarantee:** `emit()` chains every notification onto a single `lastEmit` promise (`session.ts:477`), so updates are serialized per session. `settleTurn()` awaits `publishContextUsage()` → `flushEmits()` *before* resolving the `session/prompt` request (`session.ts:501-521`). Notification errors are swallowed so prompt completion never depends on client health.

**Tool-call projection details worth copying:**
- Status is monotonic: `currentToolCalls` tracks `pending|in_progress` and the code never downgrades back to `pending` when a late `toolcall_delta` arrives.
- Locations are resolved against session `cwd` (relative → absolute) so editors can follow along.
- For `edit`, the pre-edit file is snapshotted at `tool_execution_start`; on success the new contents are re-read and emitted as a **structured ACP diff** `{type:'diff', path, oldText, newText}`. Line number is inferred from a *unique* `oldText` match in the snapshot (returns undefined on ambiguity).
- Bash is special-cased: `kind:'execute'`, `content:[{type:'terminal', terminalId: toolCallId}]` plus `_meta.terminal_info / terminal_output / terminal_exit` (`translate/bash.ts`). This is a **Zed display convention** — the agent never called `terminal/create`, so it is not spec-compliant terminal delegation. Output deltas are computed by prefix-diffing snapshots.

### 2.7 `session/cancel` and permission requests

Cancel (`agent.ts:897` → `session.ts:405`):
1. `cancelRequested = true`; queued turns are drained and resolved `'cancelled'`.
2. `proc.abort()` → `{type:'abort'}` RPC to pi.
3. The in-flight `session/prompt` resolves only when pi emits `agent_settled` afterwards. **If pi never emits it, the ACP request hangs** — this is not covered by a timer (issue #160).

Permissions: **tool calls are never permission-gated.** pi runs autonomously. `conn.requestPermission` is used in exactly one place (`session.ts:911`, inside `requestExtensionPermission`) to service pi **extension UI requests**:

- `extension_ui_request` with `method: 'select'` → one `allow_once` option per pi option, `optionId: choice-<index>`.
- `method: 'confirm'` → `[{yes, allow_once}, {no, reject_once}]`.
- `method: 'input' | 'editor'` → unsupported: emits a chat notice and replies `{id, cancelled: true}`.
- `method: 'notify'` → chat chunk with `_meta.piAcp.notify.level`, then `{cancelled: true}`.
- Any permission RPC exception is converted to `{id, cancelled: true}` so pi is never left blocked.
- Synthetic `toolCall` for the prompt is `{toolCallId: 'pi-ui-<id>', title, kind: 'other', status: 'pending', rawInput}`.

---

## 3. Connection to the underlying agent

**Subprocess, per ACP session: `pi --mode rpc --no-themes`** (`src/pi-rpc/process.ts:184`). No SDK, no socket, no in-process embedding. `AGENTS.md` states it plainly: *"1 ACP session ↔ 1 pi subprocess"* because pi RPC mode is effectively single-session.

Wire protocol (pi's, not ACP): newline-delimited JSON on the child's stdin/stdout.

- Outbound: `{type, id, ...}` command objects. Commands used: `prompt`, `abort`, `get_state`, `get_available_models`, `set_model`, `get_available_thinking_levels`, `set_thinking_level`, `set_follow_up_mode`, `set_steering_mode`, `compact`, `set_auto_compaction`, `get_session_stats`, `set_session_name`, `export_html`, `switch_session`, `get_messages`, `get_commands`, `extension_ui_response` (`process.ts:35-56`).
- Inbound: `{type:'response', id, success, data, error}` correlates to a pending request via `id` (a `crypto.randomUUID()`); **everything else is broadcast to event handlers** — i.e. the two channels are demultiplexed by the presence of `type === 'response'`.
- Non-JSON stdout lines are captured as a "prelude" (ANSI-stripped) rather than fatal — pi prints human-readable startup info there.
- Child `exit`/`error` rejects **all** pending requests with `pi process exited (code=…, signal=…)`.
- Spawn is gated on the child's `spawn` event so `ENOENT`/`EACCES` surface as `PiRpcSpawnError` with a code, instead of later EPIPE noise.
- Auxiliary requests (`get_session_stats`) use a 1 s timeout (`SESSION_STATS_TIMEOUT_MS`), with `timer.unref()` and idempotent `drop()` so a late response cannot double-settle a promise.

### Session ↔ conversation mapping and cross-turn state

| Concern | Mechanism | Evidence |
|---|---|---|
| 1 ACP session = 1 conversation | 1 child process per session; child owns the transcript | `session.ts:220`, `AGENTS.md` |
| Session identity | ACP `sessionId` **is** pi's session id, read from `get_state()` | `session.ts:238` |
| Durable mapping | `~/.pi/pi-acp/session-map.json` — `{version:1, sessions:{id:{sessionId,cwd,sessionFile,updatedAt}}}` | `paths.ts`, `session-store.ts` |
| Transcript store | pi's own `~/.pi/agent/sessions/**/*.jsonl` (or `PI_CODING_AGENT_DIR`, or `settings.sessionDir`) | `pi-sessions.ts:14-50` |
| Resuming | new child spawned with `--session <sessionFile>`, so pi rehydrates | `process.ts:186`, `agent.ts:207` |
| History replay | `session/load` calls `get_messages()` and re-emits synthetic `user_message_chunk`, `agent_message_chunk`, `tool_call`/`tool_call_update` (bash replayed with terminal meta) | `agent.ts:977-1090` |
| In-adapter state | `SessionManager.sessions` map, `restoringSessions` (dedupes concurrent loads), per-session turn queue, tool-call status map, bash output snapshots, pending startup info, `cancelRequested` | `session.ts:290-330` |
| Cross-restart durability | session files + session map; adapter keeps **no transcript in memory** | — |

Consequences visible in code: a client can `session/list` after restart, `session/load` an old session, and the same session is `/resume`-able in the plain `pi` CLI — that bidirectional resumability is the single strongest design idea here.

Session-store writes are `writeFileSync(JSON.stringify(...))` directly with no temp-file + rename, so a crash mid-write corrupts the map (it degrades to `{version:1, sessions:{}}`, i.e. silently forgets all sessions).

---

## 4. Auth and configuration

**Credentials are never touched by the adapter.** pi owns provider keys/OAuth (`~/.pi/agent/…`); the adapter's auth story is: detect failure, tell the client to run terminal login.

- `authMethods` (`src/acp/auth.ts`): single method `pi_terminal_login`, `type: 'terminal'`, `args: ['--terminal-login']`, `env: {}`, **plus** `_meta['terminal-auth'] = {command, args:[…, '--terminal-login'], label}` for Zed's Authenticate banner. Launch spec is derived from `process.argv` (reuses `node /path/dist/index.js`), else falls back to bare `pi-acp`.
- Failure detection is **substring matching** on error text (`auth-required.ts`): `api key`, `apikey`, `missing key`, `no key`, `not configured`, `unauthorized`, `authentication`, `permission denied`, `forbidden`, `401`, `403` → `RequestError.authRequired({authMethods}, …)`. Also triggered when pi reports zero available models right after spawn.
- `authenticate` is a no-op (`agent.ts:443`).

Environment variables (complete set in `src/`):

| Var | Effect |
|---|---|
| `PI_ACP_PI_COMMAND` | override the pi executable (all 3 spawn sites) |
| `PI_ACP_ENABLE_EMBEDDED_CONTEXT` | `'true'` → advertise `promptCapabilities.embeddedContext` |
| `PI_CODING_AGENT_DIR` | pi config root (default `~/.pi/agent`); affects settings, sessions, skills |
| `HOME` | used raw for `~/.pi/agent/prompts`, `~/.pi/agent/extensions`, `~/.agents/skills` |

No CLI flags except `--terminal-login`.

Config files read:
- `<agentDir>/settings.json` + `<cwd>/.pi/settings.json`, **deep-merged, project wins** (`pi-settings.ts`). Only `enableSkillCommands` (and legacy `skills.enableSkillCommands`) and `quietStartup` (legacy `quietStartUp`) are consumed.
- `settings.sessionDir`, `settings.packages` (used to render extension list in the startup banner).
- `<agentDir>/prompts/**/*.md`, `<cwd>/.pi/prompts/**/*.md` — YAML frontmatter `description:`, fallback to first line, 60 chars.
- `<agentDir>/skills/**`, `~/.agents/skills/**`, `<cwd>/.pi/skills/**`, `<cwd>/AGENTS.md` — for the startup banner only.
- `~/.pi/pi-acp/session-map.json` — adapter-owned.
- Installed pi `CHANGELOG.md` (via `which pi` → realpath → package root, or `npm root -g`) for `/changelog`.

Blocking subprocess calls on the `session/new` path: `pi --version` and `npm view @earendil-works/pi-coding-agent version` (800 ms timeout) to build the "New version available" notice. Synchronous `spawnSync` inside a request handler is a latency smell.

---

## 5. Packaging and launch

- `package.json`: `"bin": {"pi-acp": "dist/index.js"}`, `"type": "module"`, `"files": ["dist"]`, `engines.node >= 20` (README says Node 22+ — inconsistent), `publishConfig.provenance: true`.
- Runtime deps: `@agentclientprotocol/sdk ^0.26.0`, `cross-spawn ^7`, `zod ^3.25` (peer of the SDK). Build: `tsup` (`tsup.config.ts`: ESM, node22 target, sourcemap, `banner.js = '#!/usr/bin/env node'`, no minify/splitting). Tests: `node --import tsx --test test/**/*.test.ts` (node:test, no jest/vitest).
- CI: `.github/workflows/npm-publish.yml`, `github-release.yml`, and a dedicated `windows-session-path.yml` that runs exactly two test files (`pi-rpc-session-path`, `pi-command`) on `windows-latest` — evidence that Windows paths/launchers were a recurring failure.
- Smoke harness: `scripts/smoke-acp.mjs` builds, spawns `node dist/index.js`, writes raw JSON-RPC lines (`initialize`, `session/new`, `session/prompt`) and kills on prompt resolution. Ten more `scripts/smoke-*.mjs` cover compact/export/modes/queue/session/changelog/load.
- Editor wiring (README): ACP registry entry (`"type": "registry"`), `npx -y pi-acp`, global `pi-acp`, or `node /path/dist/index.js` under `agent_servers` in Zed's `settings.json`, with `env` for the two `PI_ACP_*` vars.

---

## 6. Known limitations

README "Limitations" (verbatim intent):
- No ACP filesystem delegation (`fs/*`), no terminal delegation (`terminal/*`) — pi does local I/O.
- `mcpServers` accepted and stored in session state but **not forwarded** to pi.
- Assistant streaming as `agent_message_chunk` "no separate thought stream" — **stale**: `agent_thought_chunk` is emitted (`session.ts:700`).
- Queue is client-side, approximating pi's `one-at-a-time`.
- pi extension-provided slash commands not supported; `/model`, `/clear` intentionally not implemented.

From the live issue list (`api.github.com/repos/svkozak/pi-acp/issues`, open at time of writing):

| # | Issue | Maps to code |
|---|---|---|
| 160 | in-flight `session/prompt` never resolves when the pi child exits mid-turn | completion depends on `agent_settled`; no exit watchdog |
| 152 | `session/new`/`load` close unrelated sessions, leaving their prompts unresolved | `closeAllExcept` at `agent.ts:384`, `:967` |
| 161 | `session/list` is O(N²) — every page rescans and re-reads all session files | `listPiSessions()` per request, no cache |
| 148 | mid-turn compaction / aborted tool call silently ends as `end_turn` | `stopReason` fallback in `agent.ts:889` |
| 147 / 135 / 136 | pi token-limit stops and prompt errors not preserved / propagated | same fallback; ACP has no error stop reason |
| 154 | turn stalls at tool-call boundary: no tool results, timeouts never fire | no per-tool timeout anywhere |
| 153 | `session/new` `mcpServers` silently dropped | `mcpServers` stored only |
| 138 | JetBrains Air looks for `category: "mode"`; `thought_level` breaks mode changes | `buildConfigOptions` |
| 140 | pi extension commands absent from `available_commands_update` | `includeExtensionCommands: false` hardcoded |
| 141 | project trust not handled in non-interactive RPC | no `--approve` equivalent |
| 167 | several paths ignore `PI_CODING_AGENT_DIR`, always create `~/.pi` | `prompts`/`extensions` use raw `HOME` (`agent.ts:1568+`); `paths.ts` hardcodes `~/.pi/pi-acp` |
| 132 | Windows: session file path truncated at spaces → sessions silently share a file | session path passed as argv string |
| 165 | Windows: backslashes in startup banner get mangled by Markdown | banner assembly |

Also dead/vestigial: `unstable_setSessionModel` (no such method in SDK 0.26), legacy `models`/`modes` response fields, `queueDepth` metadata the code itself annotates as invisible in Zed.

---

## 7. What to copy for a Go implementation

### Copy these

1. **Two adapters, one thin middle.** Keep `acp/` (protocol), `agrpc/` (upstream transport), `translate/` (pure functions). The pure translators carry most of the test value; pi-acp's `test/unit/pi-messages.test.ts`, `pi-tools.test.ts`, `prompt-to-pi-message.test.ts` are the model.
2. **Session id = upstream session id.** Enables `session/list`, `session/load`, `session/delete`, and cross-tool resume for free instead of inventing an id space plus a translation table.
3. **Durable, tiny, boring session map.** A JSON file `{sessionId → cwd, upstreamSessionFile}` is all the adapter needs. Do it with temp-file + `rename` (pi-acp does not).
4. **Turn completion driven by a terminal upstream event, not by request ack.** The ack-only RPC reply vs. "multi-turn retry/compaction before settle" distinction is the crux of correct ACP prompt semantics.
5. **Single serialized notification chain per session**, flushed *before* the `session/prompt` response. Ordering is user-visible.
6. **Defer session-scoped notifications past the `session/new` response.** A `time.AfterFunc(0, …)` / goroutine hand-off is needed because clients drop updates for unknown session ids.
7. **Monotonic tool-call status** and a tool-call id → status map; never regress `in_progress` → `pending`.
8. **Snapshot-before-mutation to emit structured ACP diffs**, with unique-`oldText` line inference for the location. Cheap, and the single biggest editor-UX win.
9. **`requestPermission` only for upstream UI prompts**, with a mandatory "reply cancelled on any error" path so the child is never left blocked.
10. **Capability negotiation from the client's own advertisement** (`clientCapabilities._meta`) and env-gated optional caps (`embeddedContext`), not hardcoded `true`.
11. **Honest media translation**: text passthrough, resource → annotated text, image passthrough, audio → explicit unsupported marker instead of a silent drop.
12. **Spawn errors as typed errors with `code`**, surfaced as `internalError({code})` / `authRequired` so the editor can show an actionable message. Verified against the `spawn` event, not the first write.
13. **Auth failure → `RequestError.authRequired` + `authMethods`**, so the client offers a login affordance. For Antigravity, prefer a real readiness probe (does `agy`/the app respond?) over pi-acp's substring matching, but keep the *shape* of the error.
14. **Smoke script as a first-class artifact.** A 60-line raw-stdio script (`initialize` → `session/new` → `session/prompt` → assert chunks) is worth more than most of the unit tests for regression detection.

### Avoid these

1. **No watchdog on turn lifetime.** pi-acp hangs forever if the child dies or settles silently (issue #160). In Go: a per-turn `context.Context` + child-exit watcher that resolves the prompt with `cancelled`/error, plus a per-tool timeout (issue #154).
2. **Do not close sibling sessions to enforce "one live child".** It strands in-flight prompts (issue #152). Refcount sessions, or let idle children be reaped — don't kill live turns.
3. **Do not count on `agent_settled` being emitted after `abort`.** Make cancel resolve the pending turn itself (with a short grace window for late updates).
4. **Avoid `O(N)` session rescans per `session/list` page** (issue #161). Cache file headers by mtime+size in Go.
5. **Do not advertise capabilities you do not implement.** pi-acp's own README drift on the thought stream shows how this rots. Keep one source of truth from which `initialize`, the docs, and the tests are generated.
6. **Do not emulate a terminal you never created.** `content:[{type:'terminal', terminalId}]` + `_meta.terminal_*` works in Zed only; a strict client will show a dangling terminal id. If Antigravity-ACP shells out, either do real `terminal/*` delegation or use plain `content` text blocks.
7. **Do not swallow `stopReason` semantics.** Map upstream token-limit / abort-after-tool-failure distinctly (`max_tokens` exists in ACP) rather than collapsing everything to `end_turn` (issues #147/#148).
8. **Do not run `spawnSync` (`which`, `npm view`, version probes) inside request handlers.** pi-acp does this on `session/new` and in `/changelog`; move to a cached background probe.
9. **Do not invent non-schema response fields.** `models`/`modes` on `NewSessionResponse` are silently stripped by strict clients — piggyback on `configOptions` or `_meta`.
10. **Do not ship dead RPC handlers.** `unstable_setSessionModel` rots silently when the SDK drops the method; add a route test per method name.
11. **Windows argv plumbing needs its own CI lane.** pi-acp's dedicated workflow plus issues #132/#165 are the evidence; a session path with a space silently corrupting sessions is the worst class of bug because it looks like data loss later.
12. **Avoid `process.exit()` on write-error paths inside library code.** pi-acp calls `process.exit(0)` from a stdout error handler (`index.ts:75`) — acceptable for a CLI, hostile in a reusable package (PR #157 exists precisely to decouple the library from the CLI).
13. **Right-size the capability surface.** pi-acp's `AGENTS.md` MVP constraints (no fs/terminal delegation, ignore `mcpServers`) are honest, but silently *accepting* `mcpServers` and dropping them (issue #153) is worse than rejecting. For Antigravity, the analogous decisions are: which of app / `agy` / VS Code extension is authoritative per session — decide and document, don't accept-and-drop.

### Go-specific notes

- Framing: newline-delimited JSON. Use `json.Decoder` on the child's stdout (it tolerates no trailing newline issues) but you must tolerate non-JSON prelude lines — decode into `json.RawMessage` and fall back to logging the raw line. Guard with `bufio.Scanner` buffer growth if the upstream emits long JSON lines.
- You need **two independent runtimes**: a newline-delimited JSON-RPC 2.0 loop for ACP (stdlib, `encoding/json` + your own dispatch table keyed by method string) and a second one for the upstream channel. Do not reuse one connection type for both; the two protocols differ (`session/update` notifications vs `type:response` correlation).
- Request/response correlation: `map[string]chan response` guarded by a mutex, `id` = UUID; reject-all-on-exit.
- Per-session goroutine topology that mirrors pi-acp's proven shape: 1 reader goroutine (split into responses vs events), 1 serialized writer goroutine (channel + mutex) so notifications can't interleave, 1 event loop that owns tool-call state.
- Keep `_meta` payloads in a typed struct per namespace (`piAcp`, `terminal_info`) rather than `map[string]any` — the codebase's `any`-heavy event handling (`Record<string, unknown>` everywhere) is its main maintainability cost.

---

## Appendix: evidence map

| Claim | Evidence |
|---|---|
| Agent-side only, stdio NDJSON | `src/index.ts:41-63`, `sdk/dist/stream.js` |
| `PROTOCOL_VERSION = 1` | `sdk/dist/schema/index.js:43` |
| Method table | `sdk/dist/acp.js` dispatch switch; handlers at `src/acp/agent.ts:239-1190` |
| Turn settles on `agent_settled` | `src/acp/session.ts:956-970`, comment `:308-312` |
| Notification serialization | `src/acp/session.ts:477-500`, `:515-525` |
| Deferred post-`new` notifications | `src/acp/agent.ts:406-441` (`setTimeout(…,0)` + comment) |
| One child per connection | `src/acp/agent.ts:384`, `src/acp/session.ts:203` |
| Permission only for extension UI | `src/acp/session.ts:911-1010` |
| Session map schema | `src/acp/session-store.ts:6-20`, `src/acp/paths.ts` |
| Session id from pi | `src/acp/session.ts:238` |
| pi RPC command set | `src/pi-rpc/process.ts:35-56` |
| Auth methods | `src/acp/auth.ts`, `src/acp/auth-required.ts` |
| Env vars | `grep process.env src/` → `PI_ACP_PI_COMMAND`, `PI_ACP_ENABLE_EMBEDDED_CONTEXT`, `PI_CODING_AGENT_DIR`, `HOME` |
| Build/publish | `package.json`, `tsup.config.ts`, `.github/workflows/*.yml` |
| Limitations | `README.md` ("Limitations"), GitHub issues #132–#167 |
