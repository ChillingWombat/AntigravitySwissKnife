# Antigravity-ACP: Architecture Design Review & Forensic Attack

Target: `.rpiv/artifacts/design/antigravity-acp-architecture.md`  
Evaluated against: `/home/david/.local/bin/agy` (v1.4.5, Linux x86_64), `coder/acp-go-sdk` v0.13.5, and research dossiers (`reverse-pi-acp.md`, `reverse-antigravity-acp.md`, `acp-go-sdk.md`).

---

## 1. Answers to Section 7 Open Questions

### Question 1: Separate Go module vs. package in root module
* **Recommendation**: **Subpackage inside root module (`pkg/acp/`, `cmd/antigravity-acp/`). Do not create a separate `go.mod`.**
* **Reasoning**:
  1. Standalone binaries in Go do not require separate `go.mod` files. Running `go build -o bin/antigravity-acp ./cmd/antigravity-acp` from the repo root produces a completely self-contained binary with zero external runtime dependencies.
  2. A nested module inside the monorepo forces dual dependency trees, `replace` directives during local development, desynced Go toolchain versions, and prevents the main daemon (`cmd/swiss`) from importing ACP types or sharing SQLite session models directly.
* **Reasoning that would change it**: If and only if `antigravity-acp` is extracted into an independent public GitHub repository (e.g. `github.com/davids/antigravity-acp`) distributed via `go install github.com/...@latest` independently of `Antigravity Swiss Knife`, or if `coder/acp-go-sdk` introduces unresolvable protobuf/gRPC dependency pins that conflict with the root daemon.

---

### Question 2: CLI cancellation (kill-and-respawn vs in-band)
* **Recommendation**: **Kill-and-respawn via `SIGINT` (falling back to `SIGKILL` on timeout), paired with passing `--conversation <id>` on subsequent turns. In-band cancellation does not exist.**
* **Reasoning**:
  1. Live binary forensics on `/home/david/.local/bin/agy` v1.4.5 prove that sending `{"event":"cancel"}` on stdin outputs:
     ```text
     warning: ignoring unsupported stream input message event "cancel"
     ```
  2. Disassembly of the input event dispatcher at `0x947e0f2–0x947e5e4` confirms that only `event == "user"` (string length 4, ASCII `0x72657375`) is dispatched. Events `control_request` (length 15) and `control_response` (length 16) jump to `0x947e5ea` and explicitly return `stream input message event %q is not supported yet`. Any other event name is rejected or ignored.
  3. `SIGINT` triggers `agy`'s internal shutdown hook, which commits the WAL and flushes `~/.gemini/antigravity-cli/conversations/<id>.db`. Since the adapter captures `conversation_id` from the initial `init` event, respawning `agy --conversation <id>` restores the full trajectory.
* **Reasoning that would change it**: If Google ships an `agy` update that implements `control_request` or an in-band cancel event over `stream-json` stdin. (We should leave an internal hook for this, but the runtime driver must default to process signaling).

---

### Question 3: Should the app (CDP) surface ship in v1?
* **Recommendation**: **Cut CDP entirely. Do not ship `surface/app` in v1; ship CLI-only.**
* **Reasoning**:
  1. The repo's standing brief (`AGENTS.md`) strictly prohibits Potemkin facades and fragile DOM scraping: *"Manipulating DOM elements, button labels, or dropdowns without an active, verified data pipeline to the host application's runtime/IPC/network layer is an INTEGRITY VIOLATION."*
  2. CDP scraping cannot deliver token-by-token streaming, reasoning deltas (`agent_thought_chunk`), structured tool execution parameters (`tool_info`), or token usage accounting. It breaks whenever Google updates UI class names or DOM hierarchies in Antigravity 2.0.
  3. The local `agy` CLI uses the identical backend, account authentication, model catalog, and plugins as the desktop app, but provides a fully structured, strongly typed NDJSON stream. Shipping a bulletproof CLI adapter delivers immediate value without shipping fragile technical debt.
* **Reasoning that would change it**: If Antigravity 2.0 exposes a headless WebSocket or local Unix domain socket with structured JSON-RPC (e.g. its internal language server or the `agy_acp_server` socket at `127.0.0.1:35779`) rather than requiring DOM inspection.

---

### Question 4: Auth handling (`oauth_creds.json` vs explicit `authenticate`)
* **Recommendation**: **Transparently reuse existing `~/.gemini/` credentials. In `initialize`, advertise an `agent`-type `authMethod` (or empty `authMethods: []` if already authenticated). Return ACP error `-32000` (Auth Required) from `session/new` only if credentials are missing or expired.**
* **Reasoning**:
  1. `agy` manages OAuth tokens in `~/.gemini/` automatically. Forcing every editor session to execute an interactive `authenticate` handshake when the user is already signed in introduces gratuitous friction.
  2. Editors (Zed, JetBrains) expect local CLI agents to be immediately ready. If `~/.gemini/` credentials are valid, `session/new` succeeds immediately. If missing, returning `-32000` signals the editor to show the login prompt.
  3. The active account email can be surfaced passively in `_meta.account` or `session_info_update` without gating the handshake.
* **Reasoning that would change it**: If Google adds multi-account profile switching to `agy` (e.g. `--account <email>`), requiring the editor to choose an identity before launching the session.

---

### Question 5: `session/list` for the CLI (read SQLite vs adapter index)
* **Recommendation**: **Maintain an adapter-owned metadata index (`~/.config/antigravity-acp/sessions.json` or SQLite WAL) for ACP-created sessions. Read `agy`'s `conversations.db` only as a secondary read-only fallback.**
* **Reasoning**:
  1. As documented in `reverse-pi-acp.md` (issue #161), scanning the underlying agent's raw storage directory on every `session/list` pagination call is an $O(N^2)$ operation that freezes editor UI threads.
  2. As documented in `reverse-antigravity-acp.md`, reading `agy`'s private SQLite tables couples the adapter to reverse-engineered protobuf schemas (`steps.proto`) that change without notice.
  3. An adapter-owned JSON or SQLite index tracks `{sessionId, conversationId, cwd, title, createdAt, updatedAt}` atomically. It resolves `session/list` in <1 ms without touching Google's internal database.
* **Reasoning that would change it**: If `agy` adds a CLI command `agy conversations list --json` that outputs structured conversation summaries natively.

---

## 2. Practical Failure Modes in the Architecture

### 2.1 Stream-JSON Input Framing Assumption is Wrong
* **Design claim (line 72)**: *"write one NDJSON prompt line per turn."*
* **Failure in practice**: The design omits the payload schema. If you send `{"text": "..."}` or `{"event":"user", "message":{"text":"..."}}`, `agy` immediately fails the turn:
  ```json
  {"event":"result","result":{"status":"ERROR","error":"stream input \"user\" message has no content"}}
  ```
* **Corrected Claim**: The input JSON line **must** strictly match:
  ```json
  {"event":"user", "message":{"content":"<prompt text>"}}
  ```
  Alternatively, `content` may be a list of content blocks: `[{"type":"text", "text":"<prompt text>"}]`. Disassembly at `0x947d727` (`cmpl $0x74786574, (%rax)`) confirms `text` is the only supported block type. Any other block type triggers `stream input content block type %q is not supported (only "text")`.

### 2.2 Permission Handling is a Dead-End in `stream-json`
* **Design claim (line 75–76)**: *"if the CLI asks for approval, surface it as an ACP permission request when the stream exposes it..."*
* **Failure in practice**: **`agy`'s `stream-json` input loop cannot receive permission approvals.**  
  1. Forensic analysis of `0x947e0f2–0x947e5e4` proves that `agy` accepts *only* `event == "user"`. It has no handler for permission responses.
  2. When `agy` runs without `--dangerously-skip-permissions` in default `request-review` mode, any tool call that modifies files or executes commands (`write_file`, `replace_file_content`, `run_command`) invokes `ask_permission`. Because stdin cannot answer, `agy` deadlocks or immediately denies the tool call.
* **Corrected Claim**: The adapter **must** launch `agy` with `--dangerously-skip-permissions` for execution modes (`accept-edits`, `auto`), or with `--mode plan` for read-only runs. To honor ACP's permission model, permission gating must be performed **ahead of time by the ACP client/editor**, not delegated to `agy`'s interactive prompt.

### 2.3 Process Lifetime & Orphan Subprocesses
* **Design flaw**: The design plans for a long-lived `agy` process per session, but lacks process lifecycle supervision:
  1. **Orphan processes**: If Zed or Neovim crashes or terminates via `SIGKILL`, the child `agy` process is reparented to PID 1 and runs forever. In Go, the child must be launched with:
     ```go
     cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
     ```
  2. **Sudden child exit**: If `agy` exits due to memory limits, rate limiting (429), or a crash, `p.stdout` yields EOF. Without an exit monitor goroutine, `Prompt()` hangs waiting for a terminal `result` event that will never arrive (reproducing `pi-acp` issue #160).
  3. **Idle resource leak**: Leaving `agy` running indefinitely per ACP session consumes ~150 MB RSS per child plus file locks. The adapter needs an idle reaper (e.g. kill child after 20 minutes of inactivity; re-spawn with `--conversation <id>` on next turn).

### 2.4 Cancellation Protocol Laundering
* **Design flaw**: Line 73 states *"Cancellation kills the child, since the CLI has no cancel RPC."*
* **Failure in practice**:
  When `session/cancel` arrives, `acp-go-sdk` cancels the `Prompt` context. If the adapter kills the child and allows `exec.Cmd.Wait()` to return `err != nil`, Go's `toReqErr` transforms the error into JSON-RPC `-32800` ("Request cancelled").
  Per ACP v1 specification (`prompt-turn#cancellation`), **the agent must return a successful `acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil`**, NOT an RPC error. Returning an error causes editor clients to display red failure banners instead of normal turn cancellation.

### 2.5 Session ID vs Conversation ID Synchronization
* **Design flaw**: Line 72–73 states *"the id comes from the first response so it round-trips."*
* **Failure in practice**:
  In ACP, `session/new` must return a `sessionId` **immediately** in its synchronous response. The first `session/prompt` has not occurred yet.
  If the adapter waits until `session/prompt` to spawn `agy`, it must invent a synthetic `sessionId` during `session/new`. That creates two disjoint ID spaces (`acp_sess_...` vs `agy_conv_...`).
  Alternatively, if the adapter spawns `agy` during `session/new` to read the `init` event, `session/new` takes **6.5 to 7.5 seconds** (measured live), which risks timing out strict editors during startup.
* **Corrected Claim**: Generate a UUID `sessionId` in `session/new`, persist the record to `sessions.json`, and spawn `agy` lazily on the first prompt or eagerly in a background goroutine. The mapping must be durable across restarts.

### 2.6 Line Reader Buffer Limits
* **Design flaw**: The design does not specify stream reading mechanics.
* **Failure in practice**: Standard Go `bufio.Scanner` defaults to a max token buffer of 64 KB (and errors on lines > 1 MB). In `agy stream-json`, a `step_update` for a large file write or grep output can emit single NDJSON lines exceeding 2 MB. Using `bufio.Scanner` causes an unrecoverable `token too long` crash.
* **Corrected Claim**: The stream parser must use `bufio.Reader` with `ReadBytes('\n')` or `ReadSlice` into dynamic buffers, capable of handling payloads up to 16 MB.

### 2.7 Environment & Editor Packaging
* **Design flaw**: Assumes `agy` is located on `$PATH`.
* **Failure in practice**: Desktop GUI editors (e.g. Zed launched from a desktop shortcut or macOS dock) do NOT inherit shell environments defined in `~/.bashrc` or `~/.zshrc`. `exec.LookPath("agy")` fails with `ENOENT`.
* **Corrected Claim**: The adapter must resolve binaries in order:
  `$AGY_BIN` → `~/.local/bin/agy` → `/usr/local/bin/agy` → `$PATH`.

---

## 3. Evaluation of the Surface Seam

### The Proposed Seam (Line 53–64)
```go
type Surface interface {
    Name() string
    Start(ctx context.Context, opts SessionOptions) (sessionID string, err error)
    Send(ctx context.Context, sessionID, prompt string, emit func(Event) error) (StopReason, error)
    Cancel(ctx context.Context, sessionID string) error
    List(ctx context.Context) ([]SessionInfo, error)
    Close(sessionID string) error
}
```

### Why This Seam Fails
1. **False Symmetry & YAGNI**:
   - `surface/vscode` is dead on arrival (`ErrUnsupported`).
   - `surface/app` (CDP) is fundamentally different: it does not create session IDs, has no structured tool events, and has no `StopReason`.
   - Creating a 6-method generic interface to support one real implementation (`surface/cli`) and two stubs is premature abstraction.
2. **Inverted Flow via Callbacks**:
   `Send(..., emit func(Event) error)` uses a callback pattern. In Go, callbacks invert control, complicate error handling, and make goroutine cancellation treacherous. If `emit()` blocks because the client is slow, `Send()` blocks, stalling stdout consumption and backing up the child process pipe.
3. **Loss of Multi-Modal Prompts**:
   `prompt string` flattens the input. ACP v1 `session/prompt` provides `[]acp.ContentBlock` (supporting text, images, and embedded resource links). Stripping this to `prompt string` permanently cripples image inputs before they reach the driver.

### The Simpler, Idiomatic Seam
Follow `svkozak/pi-acp`: treat the adapter as a direct, dedicated CLI driver. If modularity for future backends is required, decouple the **Session Process** from the **Turn Stream**:

```go
// Driver spawns or attaches to an agent execution instance.
type Driver interface {
    Spawn(ctx context.Context, opts SessionConfig) (SessionProcess, error)
    Resume(ctx context.Context, conversationID string, opts SessionConfig) (SessionProcess, error)
}

// SessionProcess represents an active backend instance (e.g. agy subprocess).
type SessionProcess interface {
    ConversationID() string
    // Turn starts a turn and streams strongly typed events until closed.
    Turn(ctx context.Context, prompt PromptPayload) (<-chan SurfaceEvent, <-chan error)
    Cancel() error
    Close() error
}

type PromptPayload struct {
    Text   string
    Images []acp.ImageContent
}
```
**Benefits**:
- Idiomatic Go channel streaming (`<-chan SurfaceEvent`) allows clean `select` loops over `ctx.Done()`, client backpressure, and child errors.
- Process lifetime is explicitly tied to `SessionProcess`.
- `PromptPayload` retains multimodal data.

---

## 4. Verification of `agy` v1.4.5 Capabilities

Live verification was executed directly against `/home/david/.local/bin/agy`:

### 4.1 Does `stream-json` Actually Stream Intermediate Output?
* **VERDICT**: **YES. It streams granular intermediate chunks in real-time.**
* **Live Test Evidence**:
  Command: `agy --input-format stream-json --output-format stream-json --model gemini-3.8-flash-low --mode plan`
  Output timeline:
  - `T+6.56s`: `init` event emitted containing `conversation_id`, 60 available tools, and `permission_mode: "request-review"`.
  - `T+7.71s`: `step_update` for `user_input` marked `DONE`.
  - `T+18.08s`: `step_update` for tool calls emitted (`state: "ACTIVE"` then `state: "DONE"`) with full `tool_name` (`call_mcp_tool`), arguments (`{"query":"..."}`), and duration.
  - `T+22.47s` to `T+23.14s`: `step_update` for `agent_response` streamed live with incremental chunks (`text_delta`: "I have", " prepared th", "e plan in [plan.md]...", etc.) across 12 sequential active frames before marking `DONE`.
  - `T+23.41s`: Terminal `result` event emitted with full final response, token usage (`input: 7551, output: 306, cache_read: 72440`), and duration.

**Conclusion**: The design doc's core assumption that `agy` can stream intermediate progress without polling SQLite is **100% verified and true**.

---

### 4.2 Discrepancies Between Design Doc and Binary Reality

| Feature | Design Claim | Real Binary Behavior (`agy` v1.4.5) | Impact on Design |
|---|---|---|---|
| **Input Format** | Generic NDJSON line | Requires `{"event":"user", "message":{"content":"..."}}` | **High**: Non-matching payloads fail turn immediately. |
| **Content Blocks** | Unspecified | Only `type: "text"` supported (disassembly `0x947d727`) | **Medium**: Images cannot be passed directly via stdin content blocks. |
| **In-band Cancel** | Open question (§7.2) | **None**. Stdin ignores non-user events (`warning: ignoring unsupported stream input message event`) | **High**: Must cancel via `SIGINT` to child process. |
| **Permission Round-trip** | Forward to ACP client (§4.1) | **None**. Stdin does not accept permission responses | **Fatal**: Must use `--dangerously-skip-permissions` or `--mode plan`. |
| **Startup Time** | Fast process spawn | **~6.5 seconds** to first `init` event | **High**: Do not spawn synchronously during `session/new`. |
| **Event Types** | Assumed standard | `init`, `step_update`, `result` | **Positive**: Clean 3-event protocol, easy to map to ACP. |

---

## 5. Architectural Recommendations for Implementation

1. **Module Hierarchy**: Keep `antigravity-acp` inside the repo's root module under `cmd/antigravity-acp` and `pkg/acp/`.
2. **Single Driver Model**: Do not build a multi-surface abstraction for v1. Build a lean, hardened `AgyDriver` that speaks `stream-json` over stdio pipes.
3. **Lazy Subprocess Spawning**: Generate the ACP `sessionId` instantly in `session/new`, and spawn the `agy` process either in the background or upon the first `session/prompt` to keep `session/new` latency under 5ms.
4. **Child Process Safeguards**: Always set `Pdeathsig: SIGKILL` on Linux. Supervise `agy` stderr in a dedicated logger goroutine.
5. **Clean Cancellation**: Cancel prompts by sending `SIGINT` to the `agy` process, drain the pipe, and unconditionally return `PromptResponse{StopReason: StopReasonCancelled}, nil`.
6. **Execution Modes**: Expose `mode` in `configOptions` (`plan` vs `accept-edits`). For `accept-edits`, pass `--dangerously-skip-permissions` and rely on ACP client workspace trust.
