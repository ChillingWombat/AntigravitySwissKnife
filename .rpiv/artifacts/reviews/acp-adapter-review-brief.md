# Review brief: Antigravity-ACP adapter, mesh honesty, bridges, computer use

Review the code, not this summary. Be adversarial: name defects with file and line, say what
breaks in practice, and rank findings Blocker / Should fix / Consider. Do not restate what works.

## What was built

**New Go module `antigravity-acp/`** (separate `go.mod`, binary `antigravity-acp`):

- `surface/surface.go` — `Surface` interface (`Start`, `Send(emit)`, `Cancel`, `List`, `Close`),
  plus optional `ModelSelector` and `HistoryProvider`.
- `surface/ls/` — drives the Antigravity language server over Connect RPC:
  port from the server log, CSRF token from process argv (procfs/`ps`/CIM per OS).
  Methods used: `StartCascade`, `SendUserCascadeMessage`, `GetCascadeTrajectorySteps`,
  `CancelCascadeInvocation`, `GetAllCascadeTrajectories`. Polls steps every 400 ms.
- `surface/cli/` — drives `agy --output-format stream-json -p`, streams `step_update.text_delta`,
  resumes with `--conversation`, default mode `accept-edits` with `--dangerously-skip-permissions`.
- `acp/agent.go` — `coder/acp-go-sdk` v0.13.5 agent: `initialize`, `session/new`, `session/prompt`,
  `session/cancel`, `session/list`, `session/load`, `session/set_config_option`. Unsupported
  methods return `-32601`.

**Mesh (`pkg/webgui/acp_mesh.go`, `pkg/acpmesh/presets.go`)**
- `buildACPNode` reports `connected` only when `ProbeEndpoint` succeeds; a detected process with a
  silent endpoint reports `installed_idle`, latency `null`, handshake `never`, empty version/tools.
- `SimLatencyMS` removed everywhere.
- Added a stdio preset for our adapter; binary presence is the state.

**Bridges (`pkg/webgui/server.go`, `pkg/bridge/oauth.go`)**
- Channel webhook no longer returns `{success:true}`; it returns 501 because nothing is relayed.
- OAuth metadata CORS echoes an allowlisted origin instead of `*`.

**Computer use (`pkg/bridge/computeruse_*.go`, `direct.go`)**
- `click` and `type` implemented per OS: xdotool (Linux), osascript (macOS), user32 via PowerShell
  (Windows). Status lists the detected driver.

## Decisions to attack

1. Polling `GetCascadeTrajectorySteps` every 400 ms with a step offset, treating the first
   `CORTEX_STEP_TYPE_PLANNER_RESPONSE` as the end of the turn. What breaks with multi-step
   responses, tool calls, or a turn that emits several planner responses?
2. Session identity is the cascade id, and `Send` keeps offsets in memory. What happens after a
   restart, or when two clients drive the same cascade?
3. `session/load` replays only planner text. Is that enough for a client to resume safely?
4. The CLI surface spawns `agy` per turn and kills it to cancel. Correctness and orphan risk?
5. `accept-edits` plus `--dangerously-skip-permissions` as the default. Is the permission story
   defensible given the agent-API cascade emits no approval step?
6. Mesh: is `installed_idle` the right state, and does anything still imply a connection that does
   not exist?
7. Webhook returning 501: does that break WhatsApp's retry semantics, and what should it be?
8. Computer use: shell interpolation in the PowerShell/AppleScript strings, focus requirements,
   and what happens on Wayland.

## What to verify by running, not reading

- `cd antigravity-acp && go test ./...`
- `GOOS=darwin go build ./...`, `GOOS=windows go build ./...`
- Whether `GetCascadeTrajectorySteps` returns steps in a `steps` array or under `trajectory.steps`
  for the running server, and whether the first planner response really marks the end of a turn.

## Output

Write the review to `/mnt/Data/Projects/Antigravity Swiss Knife/.rpiv/artifacts/reviews/acp-adapter-review.md`.
Group findings as Blocker / Should fix / Consider, each with file:line, the failure mode, and the
smallest correct change. If a decision above is right, say so in one line and move on.
