# Antigravity-ACP: architecture

Status: draft, for review before implementation.
Scope: a Go binary that exposes Antigravity surfaces to other editors/agents over ACP v1.

## 1. What it is

An ACP **agent** (server side). Editors connect over stdio, speak ACP, and drive Antigravity through us.
We are not an ACP client and not a proxy between two Antigravity instances.

```
editor ── ACP/NDJSON over stdio ──> antigravity-acp ──> surface backend ──> Antigravity
```

Non-goals: multiplexing several editors in one process (one process per client connection),
replacing Antigravity's own UI, or proxying Gemini traffic.

## 2. Why we differ from the two reference projects

`shubzkothekar/antigravity-acp` runs `agy -p <prompt>`, throws away stdout, and reads the private
SQLite conversation database every 200 ms to synthesize updates. It only covers the CLI and it
depends on a schema Google can change without notice.

`svkozak/pi-acp` is the opposite extreme: a thin translator between ACP and its agent's own NDJSON
RPC, 4.4k LOC, no database reads. Its layering is the one to copy.

`agy` v1.4.5 supports `--input-format stream-json --output-format stream-json`, and the help text
states it runs one turn per input line. That gives real streaming without touching Google's
private tables, so we spawn the CLI and translate its NDJSON.

## 3. Process and module layout

Separate Go module `antigravity-acp` inside this repository (`antigravity-acp/`), own `go.mod`,
own binary. The daemon talks to it later; it must also run standalone for Zed/Neovim users.

```
antigravity-acp/
  cmd/antigravity-acp/main.go     flags, surface selection, stdio wiring
  acp/agent.go                    Agent implementation, capability advertisement
  acp/session.go                  session registry, turn sequencing, cancellation
  acp/translate/                  pure functions: surface events -> acp.SessionUpdate
  surface/surface.go              Surface interface (see 4)
  surface/cli/                    agy stream-json backend
  surface/app/                    desktop app backend (CDP)
  surface/vscode/                 VS Code backend, or an explicit unsupported error
```

Dependency rule: `acp/` never imports `surface/*` directly except through the interface;
`surface/*` never imports ACP types beyond `translate`. Translation functions stay pure and tested.

## 4. The Surface seam

```go
type Surface interface {
    Name() string
    // Start opens or resumes a conversation and returns its stable id.
    Start(ctx context.Context, opts SessionOptions) (sessionID string, err error)
    // Send runs one turn. Events are delivered in order until the turn ends or ctx is cancelled.
    Send(ctx context.Context, sessionID, prompt string, emit func(Event) error) (StopReason, error)
    Cancel(ctx context.Context, sessionID string) error
    List(ctx context.Context) ([]SessionInfo, error)
    Close(sessionID string) error
}
```

`Event` is the adapter's own type (text delta, reasoning delta, tool start/update, plan, error).
`translate` maps it to `acp.SessionUpdate`. One implementation per surface.

### 4.1 `agy` CLI (first class)

Spawn `agy --add-dir <cwd> --conversation <id> --input-format stream-json --output-format stream-json`
and write one NDJSON prompt line per turn. `--conversation` gives session continuity; the id comes
from the first response so it round-trips. Cancellation kills the child, since the CLI has no
cancel RPC. Permission handling: we do not pass `--dangerously-skip-permissions` by default; if the
CLI asks for approval, surface it as an ACP permission request when the stream exposes it, otherwise
run in `--mode plan` and report the limitation honestly.

### 4.2 Desktop app (second)

Attach over CDP, which we already drive: read the DevTools port from the app's userData, attach to
the page, and post into its composer. Text deltas come from observing the response DOM. This is
fragile by nature and must be marked as such in capabilities (experimental), never presented as
equivalent to the CLI backend.

### 4.3 VS Code extension (blocked)

The extension host spawns its own language server with its own environment; editor settings cannot
be handed a model catalog or an ACP endpoint. Until we find a real attach point, `surface/vscode`
returns `ErrUnsupported` with a message naming the reason. It does not pretend to work.

## 5. ACP behaviour

Methods: `initialize`, `authenticate` (delegates to Google account state, or no-op success when the
surface needs no auth), `session/new`, `session/prompt`, `session/cancel`, `session/list`,
`session/load`, plus `session/set_mode` and `session/set_config_option` for model/effort.

Unsupported methods return `-32601` rather than empty successes: `session/close`, `session/resume`,
`session/fork`, `logout`.

Capabilities: `loadSession: true` for the CLI surface, `promptCapabilities` text and image where the
surface supports them, `mcpCapabilities: false` until we actually forward MCP servers.

Stop reasons: map turn end to `end_turn`, cancellation to `cancelled`, provider refusal to
`refusal`, length cut to `max_tokens`. No invented reasons.

Permissions: when a surface exposes approval prompts, forward them as `session/request_permission`
and honour the client's answer. When a surface cannot ask, state it in the session's config rather
than auto-approving silently.

## 6. Testing

- Translation tests: recorded surface NDJSON fixtures → expected `acp.SessionUpdate` sequences.
- Session tests: two sequential prompts on one session keep one surface conversation; cancel
  mid-turn ends with `cancelled` and no orphan child processes.
- Protocol tests: drive the built binary over stdio with `ClientSideConnection`, assert
  `initialize` capabilities, `session/new`, streamed updates, permission round-trip.
- Process hygiene: every test asserts no surviving `agy` children.
- Manual: Zed and Neovim over stdio against the real CLI.

## 7. Open questions for review

1. Is a separate Go module the right call, or a package inside the main module that the daemon
   imports? A module keeps the binary standalone but duplicates version bumps.
2. CLI cancellation: kill-and-respawn loses the conversation unless `--conversation` is passed on
   the next turn. Is that acceptable, or should the adapter prefer in-band cancellation if the
   stream-json protocol has a cancel message? (Needs verification against the CLI.)
3. Should the app (CDP) surface ship at all in v1, given it is DOM-scraping, or wait until the app
   exposes a real agent API?
4. Auth: reuse `~/.gemini/oauth_creds.json` silently, or require an explicit `authenticate` call so
   the editor shows which Google account is in use?
5. `session/list` for the CLI: read `agy`'s own conversation store, or keep our own index?
