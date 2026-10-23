# Antigravity-ACP Adapter, Mesh Honesty, Bridges & Computer Use: Adversarial Code Review

## Verification Commands & Live Runtime Inspection

### 1. `go test ./...` in `antigravity-acp/`
```text
=== RUN   TestSendStreamsTextDeltas
--- PASS: TestSendStreamsTextDeltas (0.00s)
=== RUN   TestSendReportsFailureStatus
--- PASS: TestSendReportsFailureStatus (0.00s)
=== RUN   TestConversationResumeID
--- PASS: TestConversationResumeID (0.00s)
PASS
ok  	github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface/cli	0.010s
=== RUN   TestStartSendsCascadeSource
--- PASS: TestStartSendsCascadeSource (0.00s)
=== RUN   TestSendStreamsPlannerResponseOnce
--- PASS: TestSendStreamsPlannerResponseOnce (0.40s)
=== RUN   TestListReadsTrajectories
--- PASS: TestListReadsTrajectories (0.00s)
=== RUN   TestSetModelChangesPlannerModel
--- PASS: TestSetModelChangesPlannerModel (0.40s)
PASS
ok  	github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface/ls	0.807s
```
**Observation**: Tests pass only because test suites use toy mocks with a 2-step trajectory (`USER_INPUT` -> single `PLANNER_RESPONSE` with no tool calls). Real trajectories completely violate these assumptions.

### 2. Cross-Compilation Builds
- `GOOS=darwin go build ./...`: Succeeded with exit code 0 (no compile errors).
- `GOOS=windows go build ./...`: Succeeded with exit code 0 (no compile errors).

### 3. Live Language Server RPC Inspection (`GetCascadeTrajectorySteps`)
Queried the live Language Server (PID 1299142, port 33103):
- **Envelope structure**: Steps are returned directly at the top level as `{"steps": [...]}`. There is no `{"trajectory": {"steps": [...]}}` wrapper.
- **Multi-step turn execution**: In trajectory `f820d758-17cc-4bf3-9eba-558affaadf87`, a single user prompt generated **126 steps**.
- **Planner response occurrences**: `CORTEX_STEP_TYPE_PLANNER_RESPONSE` appeared 31 times during that single turn:
  - Steps 1, 3, 5, etc.: emitted `toolCalls` and no `response` text (`resp_len = -1`).
  - Steps 57, 96, 109: emitted intermediate commentary (`response` text length 161, 81, 73) **and** `toolCalls`.
  - Step 125: final answer (`response` length 2733).
- **Error message payload**: Step `CORTEX_STEP_TYPE_ERROR_MESSAGE` in trajectory `f526ca76-e60f-4233-bc61-007b03103369` contains:
  `{"errorMessage":{"error":{"userErrorMessage":"...","shortError":"...","fullError":"..."}}}`.
  There is **no** `errorMessage.message` field in the server protocol.

---

## Blockers

### 1. Premature Turn Completion & Dropped Steps in Language Server Surface
- **File & Line**: `antigravity-acp/surface/ls/ls.go:198-202, 162-164`
- **Failure mode in practice**: A turn in Cascade routinely runs multiple steps (up to 120+ steps) and emits intermediate thoughts/text alongside tool calls (e.g. step 57 in trajectory `f820d758` emitted text of length 161 and executed a tool). Because `pollSteps` marks `done = true` immediately upon encountering any non-empty `st.PlannerResponse.Response`, `Send` returns `surface.StopEndTurn` prematurely. All subsequent steps (steps 58 through 125), subagents, tool outputs, and the final response are dropped and never delivered to the ACP client. The cascade continues running on the host while the ACP client believes the turn is complete, causing prompt collisions on subsequent turns.
- **Smallest correct change**: Do not treat `PlannerResponse.Response != ""` as completion. Check trajectory status via `GetAllCascadeTrajectories` (or check step completion / `latestExecutorTerminationReason`) and only terminate the turn when the trajectory status transitions from `CASCADE_RUN_STATUS_RUNNING` to `CASCADE_RUN_STATUS_IDLE`.

### 2. Error Message Schema Mismatch Causes 10-Minute Polling Hang
- **File & Line**: `antigravity-acp/surface/ls/ls.go:115-117, 203-206`
- **Failure mode in practice**: `ls.go` defines `errorMessage` as `struct { Message string "json:\"message\"" }` and checks `st.ErrorMessage.Message != ""`. In the actual Language Server protocol, the payload is nested under `error`: `errorMessage.error.shortError` / `errorMessage.error.userErrorMessage`. Consequently, `st.ErrorMessage.Message` is always `""`. When an error step occurs (e.g. invalid model, context overflow, executor failure), `ls.go` fails to detect it, never sets `done = true`, and hangs in a 400ms polling loop until `turnTimeout` (10 minutes) expires, returning `turn did not finish within 10m0s`.
- **Smallest correct change**: Update `step` struct to decode `ErrorMessage`:
  ```go
  ErrorMessage *struct {
      Error struct {
          UserErrorMessage string `json:"userErrorMessage"`
          ShortError       string `json:"shortError"`
      } `json:"error"`
      Message string `json:"message"`
  } `json:"errorMessage"`
  ```
  And check `st.ErrorMessage.Error.ShortError != ""` or `st.ErrorMessage.Error.UserErrorMessage != ""`.

### 3. In-Memory Step Offset Causes Stale History Replay & Instant Turn Exit After Restart
- **File & Line**: `antigravity-acp/surface/ls/ls.go:40, 171-174, 210-212`
- **Failure mode in practice**: `s.offsets` is an ephemeral in-memory map initialized to 0. When `antigravity-acp` is restarted (or when invoked per session over stdio), `offsets[sessionID]` is 0. When a client prompts an existing session, `pollSteps` queries `stepOffset: 0`. It immediately reads step 1 from historical turns, finds the old `PlannerResponse`, emits the old answer from days ago as the response to the new prompt, sets `done = true`, and returns `StopEndTurn` within 400ms. The actual answer to the new prompt is never delivered.
- **Smallest correct change**: When `offsets[sessionID]` is uninitialized, seed it before the turn by calling `GetAllCascadeTrajectories` and setting `s.offsets[sessionID] = summary.StepCount`, or record the step index of the newly sent `CORTEX_STEP_TYPE_USER_INPUT`.

### 4. Silent Error Swallowing in ACP Agent Prompt Handler
- **File & Line**: `antigravity-acp/acp/agent.go:78-84`
- **Failure mode in practice**: In `Agent.Prompt`, the callback passed to `surface.Send` discards any event where `ev.Kind != surface.EventText`:
  ```go
  if ev.Kind != surface.EventText || ev.Text == "" {
      return nil
  }
  ```
  When the surface emits `surface.EventError` (from an error step), `agent.go` returns `nil` without notifying the client. `Send` returns `StopEndTurn`, and `Agent.Prompt` returns `acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil`. The ACP client is informed that the turn succeeded normally with an empty message, silently masking backend failures.
- **Smallest correct change**: If `ev.Kind == surface.EventError`, stream an error message or return an RPC error from `Prompt`.

### 5. CLI Surface Never Persists Conversation ID Across Turns (Multi-Turn Broken)
- **File & Line**: `antigravity-acp/surface/cli/cli.go:41-50, 73-86, 149-156`
- **Failure mode in practice**: `cli.Start` mints an ID formatted as `"cli-" + hex(buf)`. `cli.Send` extracts the conversation ID using `conversationFromSession(sessionID)`, which looks for a colon (`strings.Index(sessionID, ":")`). However:
  1. `cli.Start` never includes a colon.
  2. `cli.Send` never records the conversation ID returned in the `init` or `result` JSON events into `s.sessions`.
  3. `switch ev.Event` in `cli.Send` does not even have a `case "init"`.
  Therefore, every turn executes `agy` without `--conversation`. Multi-turn conversations are completely broken; every prompt starts a brand-new detached session with zero memory of previous turns.
- **Smallest correct change**: In `Surface`, add a mutex-guarded map `convIDs map[string]string`. Handle `case "init"` in `cli.Send` to store `ev.Init.ConversationID` (or parse `ev.StepUpdate.ConversationID`), and pass `--conversation convID` on subsequent turns.

### 6. Command Injection & Syntax Invalidation in Windows Computer Use
- **File & Line**: `pkg/bridge/computeruse_windows.go:23-28`
- **Failure mode in practice**:
  1. `performType` uses `fmt.Sprintf` to interpolate raw `text` into a single-quoted PowerShell string:
     `script := fmt.Sprintf("... [System.Windows.Forms.SendKeys]::SendWait('%s')", text)`.
     If `text` contains `'` (e.g. `"don't"`), the PowerShell command terminates with a parse error. If `text` contains `'); Start-Process calc.exe; #`, arbitrary PowerShell commands execute on the host with the daemon's privileges.
  2. .NET `SendKeys.SendWait` treats `+`, `^`, `%`, `~`, `(`, `)`, `{`, `}` as modifier commands (`+` = Shift, `%` = Alt, etc.). Typing unescaped strings like `"50%"` or `"func(x)"` throws a runtime exception or leaves keyboard modifiers stuck down.
- **Smallest correct change**: Pass text via standard input or environment variables rather than command-line interpolation. For SendKeys, escape reserved characters by wrapping them in braces (`{%}`, `{+}`, `{(}`, `{)}`), or use Win32 `SendInput` via a helper script.

---

## Should Fix

### 1. CLI Surface Ignores Requested Working Directory (`Cwd`)
- **File & Line**: `antigravity-acp/surface/cli/cli.go:87`
- **Failure mode in practice**: `cli.Start` receives `SessionOptions{Cwd: ...}` and saves it in `s.sessions`. However, `cli.Send` creates `exec.CommandContext(ctx, s.Bin, args...)` and never sets `cmd.Dir`. `agy` runs in the current working directory of the `antigravity-acp` process rather than the client workspace directory. Files modified or created by the agent end up in the wrong directory.
- **Smallest correct change**: Set `cmd.Dir = s.sessions[sessionID].Cwd` if non-empty before calling `cmd.Start()`.

### 2. CLI Surface `Cancel` is a No-Op and Fails to Terminate Process Groups
- **File & Line**: `antigravity-acp/surface/cli/cli.go:87, 143`
- **Failure mode in practice**:
  1. `cli.Cancel` is a stub: `func (s *Surface) Cancel(ctx context.Context, sessionID string) error { return nil }`. Client-initiated cancellations via `session/cancel` do nothing; `agy` keeps running.
  2. When `ctx` is canceled during `Send`, `cmd := exec.CommandContext` sends `SIGKILL` only to the parent `agy` process PID. Any child processes spawned by `agy` (compilers, test runners, dev servers) are reparented to PID 1 and continue running as orphans.
- **Smallest correct change**: Track active `*exec.Cmd` per `sessionID`. On Unix, set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and kill `-cmd.Process.Pid` in `Cancel`.

### 3. WhatsApp Webhook Returning 501 Triggers 7-Day Meta Retry Storm
- **File & Line**: `pkg/webgui/server.go:7017`
- **Failure mode in practice**: In `handleBridgeChannelsWebhook`, returning HTTP 501 (`StatusNotImplemented`) on POST requests violates Meta WhatsApp Cloud API webhook semantics. Meta treats all 5xx status codes as transient delivery errors and retries delivery with exponential backoff for up to 7 days. This creates a continuous request flood in daemon logs and causes Meta to automatically deactivate the webhook endpoint due to excessive error rates.
- **Smallest correct change**: Return HTTP 200 OK with `{"status":"dropped","reason":"relay_not_configured"}` so Meta registers successful delivery and ceases retries, or return 404/410 if the bridge is disabled.

### 4. AppleScript Syntax Error & Newline Failure in macOS Computer Use
- **File & Line**: `pkg/bridge/computeruse_darwin.go:14, 21-34`
- **Failure mode in practice**:
  1. In `performClick`, `click at {x:%d, y:%d}` is invalid AppleScript syntax. `System Events` expects a list `{x, y}`, not a record `{x:..., y:...}`. Furthermore, `click at` on `System Events` without a process target fails on modern macOS versions.
  2. In `performType`, passing text with raw newlines (`\n`) to `osascript -e` fails to compile because AppleScript string literals cannot span unescaped newlines.
- **Smallest correct change**: In `performClick`, use `{x, y}` or use a lightweight native helper. In `performType`, replace `\n` with `return` keystrokes (`key code 36`).

### 5. macOS Screenshot Missing in Direct Computer Use Actuator
- **File & Line**: `pkg/bridge/direct.go:327-365`
- **Failure mode in practice**: `DirectHandler.ComputerUse` handles Linux CLI utilities (`grim`, `scrot`, `import`) and Windows (`windows-screen-capture` via PowerShell), but has no case for macOS (`darwin`). Calling `action == "screenshot"` on macOS always returns `"screenshot capture not configured: no active display or capture utility available"`.
- **Smallest correct change**: Add a case for `runtime.GOOS == "darwin"` invoking `/usr/sbin/screencapture -x <outPath>`.

### 6. Windows `%USERPROFILE%` Environment Variable Ignored by ACP Mesh Expander
- **File & Line**: `pkg/webgui/acp_mesh.go:282-291`
- **Failure mode in practice**: `PresetAntigravityACP` defines `BinaryWindows: "%USERPROFILE%/bin/antigravity-acp.exe"`. In `acp_mesh.go`, `expandHome` only checks for `strings.HasPrefix(path, "~")` and does not expand Windows `%USERPROFILE%` or `%LOCALAPPDATA%`. On Windows hosts, `os.Stat` fails on the unexpanded string, causing `antigravity-acp` to permanently report `status: "not_found"` instead of `status: "installed_idle"`.
- **Smallest correct change**: Expand `%...%` environment variables via `os.ExpandEnv` or `filepath.Clean(os.ExpandEnv(path))`.

### 7. History Replay Drops User Prompts and Leaves Step Offset at Zero
- **File & Line**: `antigravity-acp/surface/ls/ls.go:254-275, acp/agent.go:130-155`
- **Failure mode in practice**: `History()` filters solely for `st.PlannerResponse.Response != ""`. User prompts (`CORTEX_STEP_TYPE_USER_INPUT`) and tool operations are discarded. Replaying this history emits a stream of assistant messages with zero user questions, leaving the ACP client with a broken transcript. Furthermore, `History()` does not update `s.offsets[sessionID]`. If a client calls `session/load` followed by `session/prompt`, the offset is still 0, triggering the Decision 2 stale replay bug.
- **Smallest correct change**: Replay both user input and planner response messages with proper role attribution, and advance `s.offsets[sessionID]` to `len(steps)`.

### 8. LS Surface `List` Omits Conversation Title
- **File & Line**: `antigravity-acp/surface/ls/ls.go:232-249`
- **Failure mode in practice**: The `summary` struct in `ls.List` only decodes `StepCount`, `LastModifiedTime`, `CreatedTime`, and `Status`. It does not decode the `summary` field (which contains conversation titles like `"ACP Adapter Code Review"`). Consequently, `info.Title` is always empty, and `session/list` responses return uninformative bare UUIDs to ACP clients.
- **Smallest correct change**: Add `Summary string "json:\"summary\""` to the decoded struct and assign `info.Title = summary.Summary`.

---

## Consider

### 1. Wayland Display Server Input Incompatibility & Focus Stealing
- **File & Line**: `pkg/bridge/computeruse_linux.go:12-33`
- **Failure mode in practice**: `xdotool` interacts only with X11 and XWayland clients. Under native Wayland sessions (default on modern GNOME and KDE), `xdotool` cannot simulate input on native Wayland windows due to compositor security isolation. Additionally, `performType` types into whatever window holds system focus; without explicit window targeting or raising, typing can leak into arbitrary background applications.
- **Smallest correct change**: Check `$XDG_SESSION_TYPE`. If `wayland`, fallback to `ydotool` or return a clear error indicating Wayland input emulation constraints.

### 2. PowerShell `Add-Type` Compilation Overhead on Windows Mouse Clicks
- **File & Line**: `pkg/bridge/computeruse_windows.go:13-21`
- **Failure mode in practice**: `performClick` executes `Add-Type -TypeDefinition ...` via PowerShell on every single click. Invoking the C# compiler (`csc.exe`) in an ephemeral PowerShell process introduces a 1.5–2.5 second latency per mouse click.
- **Smallest correct change**: Precompile a standalone C helper binary or invoke user32 APIs via a persistent script/daemon.

### 3. Unchecked Default of `--dangerously-skip-permissions` in CLI Surface
- **File & Line**: `antigravity-acp/surface/cli/cli.go:80-82`
- **Failure mode in practice**: The CLI surface silently injects `--dangerously-skip-permissions` whenever `--mode` is not `"plan"`. In an ACP context, an external client driving `antigravity-acp` can perform destructive filesystem and shell actions without any confirmation or warning displayed to the user.
- **Smallest correct change**: Default `--mode` to `"plan"` unless explicitly configured by the client, or surface permission requests via ACP's `session/request_permission`.

### 4. CORS Allowlist Loopback Gaps
- **File & Line**: `pkg/bridge/oauth.go:141-146`
- **Failure mode in practice**: `setOAuthCORS` allows `http://localhost:` and `http://127.0.0.1:`. If a local service connects without an explicit port (`http://localhost` on port 80), or via `https://localhost`, or `[::1]`, CORS validation rejects the origin.
- **Smallest correct change**: Use `isLoopbackHost(origin)` to validate loopback origins uniformly.

### 5. Mesh Registry Top-Level Status Always "online"
- **File & Line**: `pkg/webgui/acp_mesh.go:313`
- **Failure mode in practice**: `buildACPMeshResponse` always returns `"status": "online"`, even when zero mesh nodes are reachable or active.
- **Smallest correct change**: Set `"status": "degraded"` or `"offline"` when no reachable nodes exist.

---

## Decisions Evaluated

1. **Polling `GetCascadeTrajectorySteps` every 400 ms with step offset, treating first `PLANNER_RESPONSE` as end-of-turn**:
   **FLAWED**: Turns are multi-step and emit intermediate text before tool calls; stopping at the first planner response truncates turns, drops subsequent tool steps and answers, and leaves the server running in the background.
2. **Session identity is cascade id, keeping offsets in memory**:
   **FLAWED**: Resets offset to 0 on restart or fresh stdio invocation, immediately replaying day-old historical answers to new prompts and exiting in 400ms without reading the actual response.
3. **`session/load` replays only planner text**:
   **INSUFFICIENT**: Omits user questions, breaks conversational context in the client transcript, and fails to advance the offset pointer for subsequent prompts.
4. **CLI surface spawns `agy` per turn and kills it to cancel**:
   **FLAWED**: Fails to pass `--conversation` on subsequent turns (conversation context is forgotten), ignores `Cwd`, leaves `Cancel` as an empty no-op, and risks process group orphans on kill.
5. **`accept-edits` plus `--dangerously-skip-permissions` as default**:
   **INDEFENSIBLE AS DEFAULT**: Silently removes all confirmation gates for arbitrary shell/filesystem mutations without client approval or user consent.
6. **Mesh `installed_idle` state**:
   `installed_idle` is the right state for unprobed stdio binaries or inactive endpoints, but `expandHome` fails to expand `%USERPROFILE%` on Windows.
7. **Webhook returning 501**:
   **FLAWED**: Meta treats 501 as a transient failure and initiates an exponential retry storm lasting up to 7 days, eventually disabling the webhook.
8. **Computer use string interpolation and Wayland**:
   **SEVERELY FLAWED**: PowerShell string interpolation is vulnerable to command injection via single quotes; SendKeys syntax is unescaped; AppleScript syntax is invalid on modern macOS; Wayland input silently fails under `xdotool`.
