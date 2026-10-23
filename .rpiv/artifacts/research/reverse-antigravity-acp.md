# Reverse-engineering report: `shubzkothekar/antigravity-acp`

Target: https://github.com/shubzkothekar/antigravity-acp @ `d10aa13` (v1.2.0, 2026-09-27).
Method: full clone + source read, plus live probes against the locally installed `agy` (v1.4.5)
and a real conversation SQLite DB. Every claim below carries a path or commit.

## 0. Headline

| Question | Answer |
|---|---|
| Role | ACP **agent/server** (implements the agent side of ACP). One-way bridge, not a proxy. |
| Attaches to | `agy` CLI **only**. Zero code touches the desktop app, the VS Code extension, or CDP. |
| Mechanism | Spawns `agy ... -p <prompt>` as a **stdio subprocess**, then **polls the SQLite conversation DB** for progress. It does not read agy's stdout during a turn. |
| Language | TypeScript on Bun. 6.0k LOC src + 1.9k generated protobuf, ~28 test files. Not Go. |
| Legal status | README + issue #3 declare it a **Google ToS violation** risk (account suspension). |

---

## 1. What the project actually does

It is an **ACP agent** — the server side. Editors (Zed, Paseo, Neovim clients) connect to it and
drive it; it then drives `agy`. There is no ACP client behavior toward Antigravity and no
Antigravity-to-Antigravity proxying.

Direction of control is strictly inbound:

```text
ACP client  --ndjson/stdio-->  agy-acp  --subprocess argv-->  agy CLI  -->  conversation SQLite DB
ACP client  <--ndjson/stdio--  agy-acp  <--200ms poll-------  (same DB)
```

Evidence: `README.md` ("It lets any ACP-compatible editor drive `agy`"), `src/acp/server.ts:45-55`
(server connects the SDK's `agent()`), `package.json` (`bin: agy-acp`).

**The load-bearing insight**: it is not a protocol bridge at all. `agy -p` is a black box whose
stdout is discarded (`stdout: "ignore"`, `src/agy/process.ts:108-113`). All ACP `session/update`
traffic is *synthesized from agy's private database*. The project is a **SQLite-to-ACP translator**
with a subprocess launcher attached.

---

## 2. Which Antigravity surface, and by what mechanism

`agy` CLI only, via stdio subprocess + filesystem reads.

- Binary resolution: next-to-executable → `$AGY_BIN` → `$PATH` (`src/agy/binary.ts:29-39`).
- Auto-download: pinned release table, `AGY_VERSION = "1.2.12"` from
  `google-antigravity/antigravity-cli`, six platform archives with hardcoded SHA-256
  (`src/agy/installer.ts:12-56`).
- Spawn: `Bun.spawn([binary, ...args], { cwd, stdin:"ignore", stdout:"ignore", stderr:"pipe" })`
  (`src/agy/process.ts:103-114`).
- Arg vector per turn (`src/agy/process.ts:77-99`):
  `--add-dir <cwd> [--add-dir <extra>…] [--conversation <id>] [--model <id>]`
  `--mode plan|accept-edits --dangerously-skip-permissions [--sandbox]`
  `--print-timeout 0 [<AGY_EXTRA_ARGS>] -p <prompt>`
- Progress source: `~/.gemini/antigravity-cli/conversations/<conversationId>.db`, override via
  `$AGY_CONVERSATIONS_DIR` (`src/constants/index.ts:6-10`). Polled every 200 ms
  (`POLL_INTERVAL_MS`, `src/constants/index.ts:22`; loop at `src/acp/adapter.ts:145-166`).

I verified the DB contract against a real local file. `pragma table_info(steps)` returns exactly
the columns the reader selects — `idx, step_type, status, step_payload, error_details,
permissions, task_details` — matching `SELECT_ROWS` in `src/conversation/database.ts:21-23`.
Observed `step_type` values in that DB: 14 (user input), 15 (agent text), 101 (stop hook).

**No CDP, no unix socket, no HTTP, no LS RPC, no Electron, no VS Code extension host.** The words
`cdp`, `websocket`, `extension` do not appear in `src/`.

---

## 3. The ACP implementation

### Transport
NDJSON over stdio, via `@agentclientprotocol/sdk` v1. `ndJsonStream(stdoutWritable(),
Bun.stdin.stream())` (`src/acp/server.ts:45`). stdout is reserved for the wire; `console.log` is
remapped to stderr at startup (`index.ts:21-24`).

### Methods registered (`src/acp/server.ts:60-118`)

| Method | Handler | Notes |
|---|---|---|
| `initialize` | `agent.ts:120-150` | advertises `loadSession`, `promptCapabilities.image` + `embeddedContext`, session caps `list/delete/resume/close/additionalDirectories`, `auth.logout` |
| `authenticate` / `logout` | `agent.ts:153-166` | no-ops; agy owns OAuth |
| `session/new` | `agent.ts:168-178` | random UUID, returns `configOptions` |
| `session/load` | `agent.ts:180-217` | replays full history from the DB |
| `session/resume` | `agent.ts:219-244` | re-attaches client, re-announces commands/options |
| `session/list`, `session/delete`, `session/close` | `agent.ts:246-288` | `close` cancels in-flight work |
| `session/prompt` | `agent.ts:290-345` | the whole turn |
| `session/set_config_option` | `agent.ts:352-395` | `model`, `mode`, `sandbox` |
| `session/set_mode`, `session/set_model` | `server.ts:100-113` | legacy aliases for Paseo (#14) |
| `session/cancel` (notification) | `agent.ts:347-350` | |
| `models/list`, `resources/list`, `prompts/list`, `tools/list` | `server.ts:114-121` | **not ACP spec**; last three return empty arrays (`agent.ts:397-407`) |

### Session ↔ conversation mapping

ACP `sessionId` is `crypto.randomUUID()` (`src/acp/sessions.ts:18`) and is *unrelated* to the agy
conversation id. The binding is stored in `Session` (`src/types/session.ts:2-21`) and persisted to
`~/.agy-acp/sessions.json` (`src/constants/index.ts:13-16`, `src/store/sessionStore.ts`).

First prompt binds the pair by **directory snapshot diffing**:

1. snapshot `*.db` stems before spawn (`src/conversation/scan.ts:7-19`);
2. spawn agy, then look for new stems each poll;
3. if **more than one** new DB appears, refuse to bind and log a warning
   (`src/conversation/scan.ts:26-41`).

That refusal is a real, acknowledged race — two agy runs started within one poll window on the
same machine silently detach the session. PR #4 (closed) tried to fix it with a cross-process
SQLite lock; the shipped code instead just gives up. The repo also carries `lastStepIdx` as a
high-water mark so replays and subsequent turns don't re-emit steps.

### `session/update` variants actually emitted

Grepping `sessionUpdate: "` across `src/` yields exactly seven:
`agent_message_chunk`, `user_message_chunk`, `tool_call`, `tool_call_update`,
`session_info_update`, `available_commands_update`, `config_option_update`.

Absent: `agent_thought_chunk`, `plan`, `usage_update`, `current_mode_update`. Reasoning is folded
into a synthetic `tool_call` with `kind:"think"` built from the conversation title's second
paragraph (`src/updates/title.ts:24-39`, `src/conversation/translator.ts:177-193`) — a hack that
only fires when agy happens to emit a type-23 title step.

### Tool call flow

**One direction only: agy → client, for display.** There is no path for a client to supply tools,
and no path for agy to call back into the client.

Step type → builder dispatch (`src/conversation/updates.ts:62-135`):

```text
14 user prompt      → user_message_chunk
15 agent text       → agent_message_chunk
23 title            → session_info_update (+ "Think" tool_call)
5  file edit        → tool_call kind=edit
8/9 view_file/list  → tool_call kind=read
7/33 grep/web search→ tool_call kind=search
21 run_command      → tool_call kind=execute
31 read_url_content → tool_call kind=fetch
127 invoke_subagent → tool_call other
138 ask_question    → tool_call other
17, 132             → routed by tool name (mixed bag; 17 = "artifact wrappers")
90/98/101           → dropped (lifecycle noise)
default             → routed by tool name, else skipped
```

Tool args come from `step_payload.toolRun.call.rawInputJson` — a *JSON string inside* a protobuf
field, parsed separately (`src/updates/utils.ts:15-25`). Tool outputs come from a hand-decoded
protobuf field 140 (`src/conversation/columns.ts`, #15). Status enum mapping is
`2→in_progress, 6/7→failed, else completed` (`src/updates/utils.ts:43-55`).

`ask_question` (type 138) and permission rows are rendered as **inert text content blocks**
(`src/updates/tools/question.ts:36-68`, `src/updates/utils.ts:76-79`). The user cannot answer them
through ACP; the question is already answered or abandoned inside agy.

### Permissions — deliberately absent

`agy` is **always** launched with `--dangerously-skip-permissions` (`src/agy/process.ts:89-91`,
comment: "there is no interactive terminal for the user to approve tool calls"). PR #7 made this
unconditional and removed the "Skip Permissions" mode toggle.

Consequently `session/request_permission` is **never sent**. `AcpClient.requestPermission`
(`src/acp/client.ts:17-27`) is dead code — grep finds only the definition, no call site. What the
client sees instead is a `tool_call` whose content includes `"Permission requested: command (…)"`
after the fact.

### Cancellation

`session/cancel` → `SIGINT` to the child (`child.kill()` on Windows, `SIGINT` elsewhere,
`src/acp/adapter.ts:75-85`), chosen so agy flushes its DB. Turn returns `stopReason: "cancelled"`.

---

## 4. Language, dependencies, packaging, install

- **TypeScript 5**, ESM, Bun runtime (`package.json`, `CLAUDE.md`).
- Two runtime deps only: `@agentclientprotocol/sdk ^1.0.0`, `@bufbuild/protobuf ^2.12.1`.
  Dev: `@biomejs/biome`, `@types/bun`.
- Bun-native throughout — `bun:sqlite`, `Bun.spawn`, `Bun.stdin/stdout`, `Bun.write`. No
  `better-sqlite3`, no protobufjs, no node stream shims (README "Caching & performance"). PRs #6
  and #11 (Node/tsx + `better-sqlite3` support) were **closed unmerged**; main is Bun-only.
- Packaging: `bun build --compile` SEA targets for 6 platform/arch combos (`package.json`
  `build:*`), published on `v*` tag by `.github/workflows/release.yml`, plus an opt-in Homebrew tap
  step gated on `vars.HOMEBREW_TAP_REPO` (#12).
- npm: `antigravity-acp@1.2.0`, `bin` points at `index.ts` (needs Bun present); `postinstall`
  downloads agy into `bin/`.
- Releases: v1.0.0 (2026-06-29), v1.1.0, v1.2.0 — six binaries each, confirmed via Releases API.
- Install paths for a user: `bun run index.ts` from source, `npx antigravity-acp`, a downloaded
  SEA binary, or Homebrew. Point the editor at the binary as a stdio agent.

---

## 5. Acknowledged failure modes and limitations

From README caveats, CHANGELOG, and issue/PR bodies:

| # | Problem | Evidence |
|---|---|---|
| ToS | Third-party editor driving `agy` on a personal Antigravity OAuth login is named by Google's FAQ as a ToS violation; account suspension risk | README warning, issue #3, ACP-registry PR 414 comment |
| 13 | `Cannot find module 'zod/v4'` — SDK declares zod only as a peer dep; Bun 1.3.x / `--legacy-peer-deps` / pnpm without auto-install-peers all break | issue #13 (open), PR #29 (open) |
| 17 | `E2BIG` when prompt > 128 KB arg limit → 64 KB threshold, offload to temp file | PR #17, `src/agy/process.ts:8-28` |
| 16 | agy's print-mode timeout aborted long turns; fixed with `--print-timeout 0` | PR #16 |
| 22/23 | Quota exhaustion (429) hung the client for minutes with no message; agy retries internally. Now detected by scanning `ERROR_MESSAGE` (step_type 17) rows and killing the child if reset > 60 s | PR #22/#23, `src/acp/adapter.ts:18-52` |
| 19 | Tool calls emitted `in_progress` never closed → clients treat turn as truncated (25 of 57 tool calls in one session) | PR #19, `src/conversation/translator.ts:120-147` |
| 15 | Step type 132 not routed; protobuf field 140 tool output not decoded | PR #15 |
| 14 | Paseo got `-32601` on legacy `session/set_mode`/`set_model` | PR #14 |
| 20 | No `usage_update` (no context tracking) | issue #20 (open), PR #26 (open) |
| 27 | No `session/fork` — agy has none, so PR clones the `.db` under a new id and relies on UUID length equality to keep protobuf length prefixes valid | PR #27 (open) |
| 28 | `mcpServers` from `session/new` silently dropped; PR writes a temp plugin folder because agy has no per-run MCP flag | PR #28 (open) |
| — | Narration noise ("I will …" lines) filtered by string prefix heuristic | `src/narration/index.ts` |
| — | Replay cache validated only by `(mtime, size)`; a same-size rewrite is missed | `src/conversation/replay.ts:64-77` |

Structural limitations the authors do not state but the code shows:

1. **Schema fragility.** The protobuf schema is reverse-engineered. `src/gen/steps.ts` is generated
   from `steps.proto`, but **that `.proto` is not in the repo**. `columns.ts:6-8` admits the
   hand-written proto for `permissions` "was inaccurate" and was replaced with manual decoding.
   Step-type numbers are magic constants with no upstream source.
2. **Pinned binary drifts.** `AGY_VERSION = "1.2.12"` with hardcoded checksums. Locally installed
   `agy` is **1.4.5**; `agy models` there lists `gemini-4-argon-*`, `gemini-3.8-flash-*`,
   `claude-opus-5-5-*` — none of which appear in `DEFAULT_MODELS`
   (`src/acp/agent.ts:55-58`, still `gemini-3.6-flash-medium` / `claude-opus-4-6-thinking`).
3. **One process per prompt.** ~5 s of agy startup per turn (`agy models` alone is ~5 s, noted in
   `src/agy/process.ts:36`), plus DB-scan binding.
4. **200 ms polling latency floor** and a race where two agy runs in the same window break binding.
5. **No permission gate.** Every turn runs with `--dangerously-skip-permissions`. An ACP client
   that expects to approve tool calls gets none, silently.
6. **No streaming fidelity.** Text arrives in DB-flush-sized chunks; reasoning is unmodeled;
   `ask_question` and permissions are lossy text.

---

## 6. What we copy, what we avoid

### Critical context discovered during this review

Two facts that change the plan for a Go Antigravity-ACP:

**(a) Google ships an official ACP server.** `agentclientprotocol/registry/antigravity-acp/agent.json`
declares `"id": "antigravity-acp"`, `"authors": ["Google LLC"]`, `"version": "1.4.1"`,
`"license": "proprietary"`, distributed from `dl.google.com/agy-extensions/releases/...` as
`agy_acp_server` (a Go binary — confirmed: `.par` here is an ELF Go executable, and `--help` prints
Go `absl`-style flags `--[no]debug`, `--[no]notices`). It is already installed on this machine at
`~/.local/bin/agy_acp_server` with state at `~/.gemini/antigravity-acp/acp_token.json`. Registry PR
#414 (shubzkothekar's) was **closed unmerged** with the note "the official ACP adapter is now live."

**(b) `agy` now has a native NDJSON streaming protocol.** Verified live on v1.4.5:

```bash
agy --input-format stream-json --output-format stream-json --model gemini-3.8-flash-low
```

one long-lived process, two turns fed as `{"event":"user","message":{...}}` lines, emitting:

```json
{"event":"init","conversation_id":"…","init":{"model","cwd","tools":[…60 tools…],"permission_mode"}}
{"event":"step_update","step_update":{"conversation_id","step_index","state":"ACTIVE|DONE","step_type":"user_input|agent_response","text_delta":"…"}}
{"event":"step_update",…,"duration_seconds":…,"usage":{"input_tokens","output_tokens","thinking_tokens","cache_read_tokens","total_tokens"}}
{"event":"result","result":{"conversation_id","status":"SUCCESS|ERROR","response","duration_seconds","num_turns","usage"}}
```

`--print-timeout` now defaults to `0s`, and `--effort`, `--agent`, `--project`, `--json-schema`,
`--disable-slash-commands` all exist. `init.tools` includes `ask_permission` and
`ask_custom_permission`, so permission negotiation is a first-class tool call, not a side channel.

### Copy

1. **The `sessions.json` binding model.** ACP session id → backend conversation id, plus
   `lastStepIdx` high-water mark, atomic temp-file+rename writes serialized through an in-process
   promise chain (`src/store/sessionStore.ts`). Straightforward to port to Go with a mutex and
   `os.Rename`.
2. **Legacy method aliases.** Registering `session/set_mode` and `session/set_model` as aliases of
   `session/set_config_option` (`src/acp/server.ts:100-113`) is cheap and unblocks real clients.
   Same for the non-spec `models/list` (#7) used by non-editor harnesses.
3. **Post-response notification ordering.** `available_commands_update` → `config_option_update`
   sent ~50 ms after `session/new|load|resume` so the client's mode/model pickers populate
   (`src/acp/agent.ts:437-465`). The `setTimeout` is ugly; the ordering requirement is real.
4. **Quota-error surfacing.** Detect `RESOURCE_EXHAUSTED`/429, stop the child, and return a real
   error with reset time instead of hanging (`src/acp/adapter.ts:18-52`). Any ACP front end needs
   this.
5. **Config-option shape.** `model` (category `model`, select), `mode` (category `mode`, select),
   `sandbox` (category `security`, boolean) with per-session persistence
   (`src/acp/agent.ts:474-535`). Good ACP citizenry.
6. **The tool-call lifecycle fix.** When a step is observed non-terminal, remember it and re-emit
   as `tool_call_update` on the terminal transition (`src/conversation/translator.ts:120-147`).
   Whatever event source we use, the invariant holds: a non-terminal `tool_call` must be closed.
7. **Tool-name → `ToolKind` and title derivation.** `src/updates/utils.ts:179-205` and the
   per-tool builders (`execute.ts`, `read.ts`, `search.ts`, `edit.ts`) show which agy tool names
   matter and how to render them. Reusable mapping table even if our transport differs.
8. **Temp-file prompt offload with cleanup in `finally`** (`src/acp/adapter.ts:63,196-205`) —
   useful fallback for oversized `-p` args.

### Avoid

1. **DB polling as the event source.** This is the central design error. It costs a 200 ms latency
   floor, a `(mtime, size)` cache, a whole replay cache (`replay.ts`), a directory-snapshot binding
   race (`scan.ts`), and dependence on a private schema. `agy --output-format stream-json` gives
   the same data with sub-second latency and stable field names. Polling only as a *last-resort
   fallback* if we must support agy < the version that added stream-json.
2. **Reverse-engineered protobuf without the `.proto`.** `src/gen/steps.ts` cannot be regenerated,
   and `columns.ts` already admits a wrong guess. Do not inherit this debt.
3. **One process per prompt.** ~5 s startup per turn. `--input-format stream-json` keeps one
   process alive for N turns — verified above. Combine with `session/cancel` mapping to a control
   message rather than `SIGINT`.
4. **`--dangerously-skip-permissions` unconditionally.** It defeats the point of ACP permission
   requests and is a live footgun. `init.permission_mode` and the `ask_permission` tool give us a
   real channel; use it, and default to a mode that prompts.
5. **Directory-snapshot binding with "refuse if >1 new DB".** Replace with the `conversation_id`
   from the `init` event — it arrives before any work.
6. **Faking reasoning as a `tool_call kind:"think"`** parsed out of a title string.
7. **String-prefix narration filtering** (`"I will"`, `"I'll"`, `"I’ll"`).
8. **Hardcoded binary version + checksums + model fallback list.** Stale within weeks (we have
   1.4.5 vs their pinned 1.2.12; their default models no longer exist). Discover at runtime, cache
   with a TTL, and degrade honestly.
9. **Bun-only with no Node fallback** (two closed PRs tried and failed). Irrelevant to Go, but the
   lesson — don't couple the runtime to the packaging — applies.
10. **Claiming `promptCapabilities.image`** by writing a base64 blob to `/tmp` and instructing the
    model to `view_file` it (`src/acp/agent.ts:540-560`, #24). It works, but it leaks user
    attachments to the filesystem and depends on prompt compliance.

### Implication for our Go Antigravity-ACP

`shubzkothekar/antigravity-acp` occupies only the `agy` CLI lane, and that lane is now closed:
Google ships `agy_acp_server` officially and `agy` speaks native stream-json. Rebuilding it in Go
is a strictly worse version of a shipped Google artifact, against a documented ToS violation.

The unclaimed surface — and the reason this project is worth building — is everything
`antigravity-acp` does not touch: **Antigravity 2.0 desktop app** (Electron/CDP) and the **VS Code
extension** (extension host / LS RPC). That is where a Go daemon (`cmd/swiss`, loopback HTTP,
SQLite) has a defensible role: a single supervisor that can attach to whichever Antigravity surface
is running and present one ACP agent to outside editors, with `agy` handled by delegating to the
official `agy_acp_server` rather than reimplementing it.

Concretely:

- ACP server core in Go (stdio NDJSON, `initialize`/`session.*`/`session/prompt`/`session/cancel`),
  with the copied items in §6 above.
- `agy` surface: **supervise the official `agy_acp_server`** or drive `agy --input-format
  stream-json` directly — do not poll SQLite, do not pin 1.2.12.
- Desktop app surface: CDP against the Electron renderer, which is a genuinely different problem
  from anything in this repo and has no prior art here to copy.
- Permission handling: real `session/request_permission` round-trips, defaulting to a prompting
  mode. This is the single biggest behavioral improvement available over the reference.
