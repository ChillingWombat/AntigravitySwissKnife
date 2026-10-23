# ACP agent in Go — `github.com/coder/acp-go-sdk` v0.13.5

Goal: an ACP server in Go adapting Google Antigravity (desktop app, `agy` CLI, VS Code
extension) so external editors/agents can drive it. Everything here is read from real module
source and the live spec, then verified by compiling and running code. Where spec and SDK code
disagree, the code wins; the disagreements are in §7.

Verified: `go get github.com/coder/acp-go-sdk@latest` → **v0.13.5**, cache
`/home/david/go/pkg/mod/github.com/coder/acp-go-sdk@v0.13.5`. SDK schema pin `schema/version` =
`0.13.5`, `schema/meta.json` → `"version": 1`. Upstream snapshot: agentclientprotocol.com v1
docs + release **schema-v1.26.0** (2026-10-16). Toolchain Go 1.27.1/linux. The skeleton in §6
was built and driven by a real `ClientSideConnection` over stdio pipes: initialize →
session/new → streamed `agent_message_chunk`/`plan`/`tool_call` → `session/request_permission`
round-trip → `tool_call_update` → `stopReason=end_turn`.

## 1. Package layout

One flat package `acp` at module root; no subpackages except `example/*` and `cmd/generate`.

| File | Lines | Contents |
| --- | --- | --- | --- |
| `agent.go` | 33 | `AgentSideConnection`, `NewAgentSideConnection`, `Done()`, `SetLogger()` |
| `agent_gen.go` | 520 | generated inbound dispatch `(*AgentSideConnection).handle` + outbound typed client calls |
| `client.go` / `client_gen.go` | 27 / 311 | mirror image for the client side |
| `connection.go` | 947 | JSON-RPC 2.0 line-delimited transport, notification queue, cancellation |
| `types_gen.go` | 9494 | all request/response/union types + `Agent`, `AgentLoader`, `Client` interfaces |
| `helpers.go` / `helpers_gen.go` | 259 / 151 | `TextBlock`, `UpdateAgentMessageText`, `StartToolCall`, `UpdateToolCall`, `ToolDiffContent`, `Ptr[T]` |
| `errors.go` / `extensions.go` | 82 / 103 | `RequestError`, `NewMethodNotFound`, `toReqErr`; `_`-prefixed extension methods, `CallExtension` / `NotifyExtension` |
| `constants_gen.go` | 56 | method-name consts, `ProtocolVersionNumber = 1` |

No transport package, no server loop, no config struct — the SDK is a codec plus a connection.

### What a server must implement

`Agent` (`types_gen.go:9302`) — 11 methods, **not capability-gated** (the 12th wire method,
`session/load`, is interface-gated):

```go
Authenticate | Initialize | Logout | Cancel | CloseSession | ListSessions
NewSession | Prompt | ResumeSession | SetSessionConfigOption | SetSessionMode
// LoadSession lives on AgentLoader (types_gen.go:9357)
```

Consequences:

- You must implement `CloseSession`/`ListSessions`/`ResumeSession` even when the capability is
  unadvertised: return `acp.NewMethodNotFound(...)` → client gets `-32601`, which is what the
  spec wants. `session/load` is the only interface-gated one (`loader, ok :=
  a.agent.(AgentLoader)` in `agent_gen.go`).
- `AgentExperimental` (`types_gen.go:9367`) carries every unstable method (`document/didOpen`,
  `nes/*`, `providers/*`, `session/fork`, `session/delete`). Dispatch type-asserts per method;
  don't implement it unless you serve those.
- There is no separate session handler or permission handler. The connection *is* the handler:
  you implement `Agent`, and you call back into the client through the `*AgentSideConnection`.

Client side mirrors: `Client` — 9 methods, `ReadTextFile`, `WriteTextFile`, `RequestPermission`,
`SessionUpdate`, `CreateTerminal`, `KillTerminal`, `TerminalOutput`, `ReleaseTerminal`,
`WaitForTerminalExit` (`types_gen.go:9431`) — plus `ClientExperimental` (elicitation,
`mcp/connect`, `mcp/disconnect`, `types_gen.go:9465`). `README.md:54` tells you to implement
`acp.ClientTerminal`; **that interface does not exist in v0.13.5** (terminal methods are on
`Client`). Trust the code.

## 2. Transport wiring

```go
func NewAgentSideConnection(agent Agent, peerInput io.Writer, peerOutput io.Reader) *AgentSideConnection

asc := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin) // stdio
asc := acp.NewAgentSideConnection(agent, nc, nc)              // net.Conn satisfies both: pass twice

// in-memory (what the verification harness uses)
c2aR, c2aW := io.Pipe() // agent reads c2aR, client writes c2aW
a2cR, a2cW := io.Pipe() // client reads a2cR, agent writes a2cW
asc := acp.NewAgentSideConnection(agent, a2cW, c2aR)
csc := acp.NewClientSideConnection(client, c2aW, a2cR)
```

(`agent.go:19`.) Direction matters: **`peerInput` is where you write**, **`peerOutput` is where
you read**. Swapping them deadlocks silently.

There is no `net.Listener`/`http` transport in the SDK; stdio is the transport the spec defines,
everything else is your own `io.Writer`/`io.Reader` pair.

`Done() <-chan struct{}` closes when the read loop exits (peer closed stdin, stream error, fatal
framing/queue fault); `SetLogger(*slog.Logger)` routes connection diagnostics only. The whole
main is: build the connection, keep the pointer (`a.conn = conn` — the SDK will not inject it
back into you), then `<-conn.Done()`. There is no `SetAgentConnection` hook:
`example/agent/main.go:145` defines its own and calls it at line 412 — duck typing, not an
interface.

## 3. Lifecycle as the code implements it

`initialize` → `session/new` → `session/prompt` (streaming `session/update`, optional
`session/request_permission`) → `session/prompt` response carrying `stopReason`;
`session/cancel` arrives as a notification mid-turn.

**initialize.** Dispatch unmarshals, calls generated `Validate()`, then `agent.Initialize`. The
SDK does **no** version negotiation — compare `p.ProtocolVersion` against
`acp.ProtocolVersionNumber` yourself. Spec says reply with your latest on mismatch and let the
client disconnect; the skeleton returns `-32602` instead (single-version bridge).

**session/new.** `NewSessionRequest.Validate()` requires `cwd` and **non-nil** `mcpServers`.
Return any unique `acp.SessionId`; store `cwd`, `additionalDirectories`, `mcpServers` yourself.

**session/prompt.** `agent_gen.go` wraps the call:

```go
reqCtx, cancel = context.WithCancel(ctx)
a.mu.Lock()
if prev, ok := a.sessionCancels[string(p.SessionId)]; ok { prev() }  // one live turn per session
a.sessionCancels[string(p.SessionId)] = cancel
a.mu.Unlock()
resp, err := a.agent.Prompt(reqCtx, p)
```

So `Prompt`'s ctx is already per-session cancellable, a second prompt cancels the first, and
`session/cancel` (`agent_gen.go`, `AgentMethodSessionCancel`) cancels that ctx *and then* calls
`agent.Cancel`. Abort work by watching `ctx.Done()` inside `Prompt`; `Cancel` is bookkeeping.

**Cancellation, measured** (blocking agent, real pipes): returning
`PromptResponse{StopReason: acp.StopReasonCancelled}, nil` gives the client a normal
`stopReason=cancelled`; returning `ctx.Err()` instead gives
`RequestError{Code:-32800,"Request cancelled"}` via `toReqErr`. The spec
(`prompt-turn#cancellation`) demands the first: "Agents **MUST** catch these errors and return
the semantically meaningful `cancelled` stop reason". The SDK allows both.

## 4. Protocol v1 shapes

From `constants_gen.go`, cross-checked against released `schema.json` (v1.26.0).

Client → agent **requests**:

| method | required params | result |
| --- | --- | --- | --- |
| `initialize` | `protocolVersion` | `protocolVersion`, `agentCapabilities`, `authMethods`, `agentInfo` |
| `authenticate` | `methodId` | `{}` |
| `logout` | — | `{}` |
| `session/new` | `cwd`, `mcpServers` | `sessionId`, `modes?`, `configOptions?` |
| `session/load` | `sessionId`, `cwd`, `mcpServers` | `{}` after replaying history via `session/update` |
| `session/resume` | `sessionId`, `cwd` | `{}`, no replay |
| `session/close` | `sessionId` | `{}` (must cancel in-flight work first) |
| `session/list` | — (`cursor?`, `cwd?`) | `sessions[]`, `nextCursor?` |
| `session/prompt` | `sessionId`, `prompt[]` | `stopReason`, `usage?` |
| `session/set_mode` | `sessionId`, `modeId` | `{}` |
| `session/set_config_option` | `sessionId`, `configId`, `value` | `configOptions` |

Client → agent **notification**: `session/cancel{sessionId}`. Agent → client **requests**:
`session/request_permission{sessionId, toolCall, options}`, `fs/read_text_file`,
`fs/write_text_file`, `terminal/create|output|wait_for_exit|kill|release`, `elicitation/create`.
Agent → client **notifications**: `session/update{sessionId, update}`, `elicitation/complete`.

**Capabilities.** All optional; omitted = unsupported = don't call. Agent
(`AgentCapabilities`, `types_gen.go:32`): `loadSession` bool; `promptCapabilities{image, audio,
embeddedContext}`; `mcpCapabilities{http, sse}` (stdio mandatory, implicit);
`sessionCapabilities{list, delete, resume, close, additionalDirectories}` — each an object where
`{}` means supported, absent/null means not; `auth{logout}`. Client (`ClientCapabilities`):
`fs{readTextFile, writeTextFile}`, `terminal`, `auth{terminal}`, `elicitation`, and in the
newest schema `session.{configOptions, compaction, notices}`. Baseline every agent must
support: `session/new`, `session/prompt`, `session/cancel`, `session/update`, plus
`ContentBlock::Text` and `ResourceLink` prompts.

**Update variants** (`SessionUpdate` union, `types_gen.go:5259`; discriminators read off the
generated `marshalJSON`):

| `sessionUpdate` | Go field | payload |
| --- | --- | --- | --- |
| `user_message_chunk`, `agent_message_chunk`, `agent_thought_chunk` | `.UserMessageChunk` / `.AgentMessageChunk` / `.AgentThoughtChunk` | `content`, `messageId?` |
| `tool_call` | `.ToolCall` | `toolCallId`+`title` required; `kind`, `status`, `content`, `locations`, `rawInput/rawOutput` |
| `tool_call_update` | `.ToolCallUpdate` | only `toolCallId` required; `title`/`kind`/`status` are `*T` so "unchanged" is expressible |
| `plan` | `.Plan` | `entries[]{content, priority, status}` — full replacement every time |
| `available_commands_update` | `.AvailableCommandsUpdate` | `availableCommands[]` |
| `current_mode_update` | `.CurrentModeUpdate` | `currentModeId` |
| `config_option_update` | `.ConfigOptionUpdate` | `configOptions[]` |
| `session_info_update` | `.SessionInfoUpdate` | `title?`, `updatedAt?` |
| `plan_update`, `plan_removed`, `usage_update` | `.PlanUpdate`, `.PlanRemoved`, `.UsageUpdate` | UNSTABLE in this SDK |

Exactly one union field may be set: `SessionUpdate.Validate()` errors on `count != 1`, and
`MarshalJSON` returns `{}` when none is set — a malformed notification the peer will drop.
Always build updates through the helpers.

**Permission flow.** `RequestPermission(ctx, RequestPermissionRequest)` →
`{Outcome}` where `Outcome.Selected.OptionId` or `Outcome.Cancelled`. Options are
`{optionId, name, kind}` with kind ∈ `allow_once|allow_always|reject_once|reject_always`. On a
cancelled turn the client **MUST** answer `cancelled`, so an agent seeing either `err != nil`
or `Outcome.Cancelled` must return `StopReasonCancelled`, never an error. Both-nil outcome is a
protocol violation — treat it as failure.

**Errors.** `-32700` parse, `-32600` invalid request, `-32601` method not found, `-32602`
invalid params, `-32603` internal, `-32800` request cancelled, `-32000` auth required, `-32002`
resource not found. Return `*acp.RequestError` to control the code — `toReqErr` turns **any**
other error into `-32603`, so a bare `fmt.Errorf("session not found")` reads as an internal
crash. Notifications never get responses; handler errors there are logged, except unhandled
`_`-prefixed extension notifications, dropped silently per spec (`connection.go:handleInbound`).

## 5. MCP servers and tools: what actually moves

1. **The client hands MCP servers to the agent.** `session/new`, `session/load`,
   `session/resume` all carry `mcpServers []McpServer`: stdio `{name, command, args,
   env[]{name,value}}` (mandatory for all agents), `{type:"http", name, url, headers[]}` (needs
   `mcpCapabilities.http`), `{type:"sse", ...}` (MCP-deprecated). The agent dials them and uses
   their tools as its own.
2. **There is no agent → client tool advertisement.** `InitializeResponse` has no field for
   servers, tools, or tool schemas. The only agent → client tooling surface is *reporting
   execution*: `tool_call` / `tool_call_update` with `toolCallId`, `title`, `kind`,
   `rawInput`, `rawOutput`, `content`, `locations`. Those are progress reports; a client cannot
   invoke them.
3. **Client tool calls never reach the agent.** Client → agent is the 11 requests in §4 plus
   `session/cancel`; no invoke-tool method exists. Tools run **agent-side, always**. What the
   client contributes is *environment*, not tools: `fs/*`, `terminal/*`, `elicitation/create`,
   gated by its own capabilities — the agent calls them to borrow the editor's fs and shells.
4. **Giving a client's tools to the model** is therefore indirect: the client ships an MCP
   server config (commonly its own) in `session/new` and the agent dials it — "Clients MAY use
   this ability to provide tools directly to the underlying language model by including their
   own MCP server."
5. **ACP-over-ACP MCP is SDK-only:** `McpServer.Acp` (`type:"acp"`, `id`), `McpCapabilities.Acp`,
   `mcp/connect`, `mcp/disconnect`, `mcp/message` live in `schema/meta.unstable.json`; released
   v1.26 has no `acp` MCP variant and no `mcp/*` methods. Not for interop.

Structural consequence for the Antigravity bridge: ACP cannot expose Antigravity's tools to a
foreign client as callable tools. What you expose is the *agent role* — prompts in, streamed
`agent_message_chunk`/`tool_call`/`plan` out — borrowing the client's fs/terminal when useful.

## 6. Minimal runnable server skeleton

Compiled against v0.13.5, driven end-to-end by a real client over stdio pipes. To test without
an editor: `exec.Command("./agy-acp")` + `StdinPipe()`/`StdoutPipe()` →
`acp.NewClientSideConnection(myClient, stdin, stdout)` → `Initialize`, `NewSession`, `Prompt`.
Pass `McpServers: []acp.McpServer{}` — empty slice, not nil (§9). `gofmt` will expand the one-line
stubs below; they are compacted here only to keep this listing readable.

```go
// Command agy-acp is a minimal, runnable ACP agent over stdio.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"sync"

	acp "github.com/coder/acp-go-sdk"
)

type agent struct {
	conn     *acp.AgentSideConnection // set by main; this is how you push updates
	mu       sync.Mutex
	sessions map[string]bool
}

var _ acp.Agent = (*agent)(nil)
var _ acp.AgentLoader = (*agent)(nil)

func (a *agent) Initialize(_ context.Context, p acp.InitializeRequest) (acp.InitializeResponse, error) {
	if p.ProtocolVersion != acp.ProtocolVersionNumber {
		return acp.InitializeResponse{}, acp.NewInvalidParams(map[string]any{"error": "unsupported protocolVersion"})
	}
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentInfo:       &acp.Implementation{Name: "agy-acp", Version: "0.1.0"},
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession:        true,
			PromptCapabilities: acp.PromptCapabilities{EmbeddedContext: true},
			McpCapabilities:    acp.McpCapabilities{}, // only advertise transports you actually dial
			SessionCapabilities: acp.SessionCapabilities{
				Close:  &acp.SessionCloseCapabilities{},
				Resume: &acp.SessionResumeCapabilities{},
			},
		},
		AuthMethods: []acp.AuthMethod{}, // never nil: SDK marshals nil as "authMethods": null
	}, nil
}

func (a *agent) NewSession(_ context.Context, p acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	id := newID()
	a.mu.Lock()
	a.sessions[id] = true
	a.mu.Unlock()
	// p.Cwd and p.McpServers are yours to honour here; dial MCP or drop the mcpCapabilities claim.
	return acp.NewSessionResponse{SessionId: acp.SessionId(id)}, nil
}

// LoadSession must replay the whole history as session/update before returning.
func (a *agent) LoadSession(ctx context.Context, p acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	a.mu.Lock()
	known := a.sessions[string(p.SessionId)]
	a.mu.Unlock()
	if !known {
		return acp.LoadSessionResponse{}, acp.NewInvalidParams(map[string]any{"error": "unknown session"})
	}
	err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: p.SessionId, Update: acp.UpdateUserMessageText("(replayed) earlier turn"),
	})
	return acp.LoadSessionResponse{}, err
}

// ResumeSession resumes without replaying history (unlike LoadSession).
func (a *agent) ResumeSession(context.Context, acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) { return acp.ResumeSessionResponse{}, nil }

func (a *agent) CloseSession(_ context.Context, p acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	a.mu.Lock()
	delete(a.sessions, string(p.SessionId))
	a.mu.Unlock()
	return acp.CloseSessionResponse{}, nil
}

// Everything below is capability-gated on the wire but mandatory in Go: answer -32601.
func (a *agent) ListSessions(context.Context, acp.ListSessionsRequest) (acp.ListSessionsResponse, error) { return acp.ListSessionsResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionList) }

func (a *agent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) { return acp.SetSessionModeResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetMode) }

func (a *agent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) { return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetConfigOption) }

func (a *agent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) { return acp.AuthenticateResponse{}, acp.NewInvalidParams(map[string]any{"error": "no auth methods"}) }

func (a *agent) Logout(context.Context, acp.LogoutRequest) (acp.LogoutResponse, error) { return acp.LogoutResponse{}, acp.NewMethodNotFound(acp.AgentMethodLogout) }

// Cancel runs after the SDK already cancelled the ctx handed to Prompt; that ctx is the abort signal.
func (a *agent) Cancel(context.Context, acp.CancelNotification) error { return nil }

func (a *agent) Prompt(ctx context.Context, p acp.PromptRequest) (acp.PromptResponse, error) {
	var text string
	for _, b := range p.Prompt {
		if b.Text != nil {
			text += b.Text.Text
		}
	}
	upd := func(u acp.SessionUpdate) error {
		return a.conn.SessionUpdate(ctx, acp.SessionNotification{SessionId: p.SessionId, Update: u})
	}
	if err := upd(acp.UpdateAgentMessageText("on it: " + text)); err != nil {
		return acp.PromptResponse{}, err
	}
	if err := upd(acp.UpdatePlan(
		acp.PlanEntry{Content: "inspect workspace", Priority: acp.PlanEntryPriorityHigh, Status: acp.PlanEntryStatusInProgress},
		acp.PlanEntry{Content: "apply edit", Priority: acp.PlanEntryPriorityMedium, Status: acp.PlanEntryStatusPending},
	)); err != nil {
		return acp.PromptResponse{}, err
	}

	const callID = acp.ToolCallId("call_1")
	if err := upd(acp.StartEditToolCall(callID, "Edit config", "/tmp/config.json", "{}")); err != nil {
		return acp.PromptResponse{}, err
	}
	perm, err := a.conn.RequestPermission(ctx, acp.RequestPermissionRequest{
		SessionId: p.SessionId,
		ToolCall:  acp.ToolCallUpdate{ToolCallId: callID, Title: acp.Ptr("Edit config")},
		Options: []acp.PermissionOption{
			{OptionId: "allow-once", Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce},
			{OptionId: "reject-once", Name: "Reject", Kind: acp.PermissionOptionKindRejectOnce},
		},
	})
	if err != nil {
		if ctx.Err() != nil { // cancellation must never surface as a JSON-RPC error
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
		}
		return acp.PromptResponse{}, err
	}
	if perm.Outcome.Cancelled != nil {
		return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
	}
	if perm.Outcome.Selected == nil || perm.Outcome.Selected.OptionId != "allow-once" {
		return acp.PromptResponse{StopReason: acp.StopReasonRefusal}, nil
	}

	if err := upd(acp.UpdateToolCall(callID, acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
		acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolDiffContent("/tmp/config.json", "{}\n", "")}))); err != nil {
		return acp.PromptResponse{}, err
	}
	if err := upd(acp.UpdateAgentMessageText("done.")); err != nil {
		return acp.PromptResponse{}, err
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func newID() string {
	var b [12]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	return "sess_" + hex.EncodeToString(b[:])
}

func main() {
	a := &agent{sessions: map[string]bool{}}
	// peerInput (2nd arg) is what we write to; peerOutput (3rd) is what we read from.
	conn := acp.NewAgentSideConnection(a, os.Stdout, os.Stdin)
	conn.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	a.conn = conn
	<-conn.Done() // peer closed the stream
}
```

## 7. SDK vs spec drift (trust the code)

| Topic | Spec (v1.26) | SDK v0.13.5 | Rule |
| --- | --- | --- | --- | --- |
| `notice`, `compaction_update`, `compaction_summary_chunk` | defined | **absent** from `SessionUpdate` | not expressible without `_meta`/extension |
| `usage_update` | stable | present, marked `UNSTABLE` | usable, treat as unstable |
| `plan_update`, `plan_removed` | defined | `UNSTABLE` | prefer plain `plan` |
| `name` on `tool_call`, `messageId` on prompts | defined | absent / unstable-only | fold the name into `title`; ignore `messageId` |
| MCP `acp` variant, `mcp/*` methods | absent | unstable-only | don't rely on it |
| `session/fork`, `session/delete` | defined | `AgentExperimental` only | avoid |
| `Agent` interface size | baseline = new/prompt/cancel/update | 11 mandatory methods | stub with `NewMethodNotFound` |
| client interface naming | — | `README.md` names a nonexistent `acp.ClientTerminal` | use `acp.Client` |
| `$/cancel_request` | receiver "free to ignore" | implemented and *sent* on outbound ctx cancel | peer answers the original request |

Both advertise `protocolVersion: 1`, so none of this is detectable through the version integer.
Feature detection must go through capabilities and `_meta` only.

## 8. Authentication

Declared in `InitializeResponse.AuthMethods []AuthMethod`, a union on `type` (custom
`UnmarshalJSON` at `types_gen.go:458` — set exactly one pointer, no validation catches two):

- absent / `"agent"` → `AuthMethod.Agent`: the agent logs in itself; client calls
  `authenticate{methodId}` and gets `{}`.
- `"env_var"` → `AuthMethod.EnvVar` (SDK `AuthMethodEnvVarInline`): client collects values into
  process env at launch. Unstable.
- `"terminal"` → `AuthMethod.Terminal`: the **client** re-launches your binary with the
  method's `args` + `env`, waits for exit 0, reconnects and re-initializes. Needs client
  capability `auth.terminal: true`, and must **not** be passed to `authenticate`.

`logout` is gated on `agentCapabilities.auth.logout` (`{}` = supported), params `{}`; clients
`SHOULD` expect active sessions to fail with auth errors afterwards. For the bridge: one
`agent`-type method that triggers `agy auth` internally (or `-32000` from `session/new` until a
token exists), plus `auth.logout` only if you can really clear the credential.
`AuthMethods: []acp.AuthMethod{}` when nothing is needed — empty slice, never nil.

## 9. Traps

**Framing.** Line-delimited JSON, one message per line, UTF-8, no embedded newlines
(`connection.go:receive`). Reader is `bufio.Scanner`, 1 MB initial / **10 MB max**; one larger
message kills the connection — never put a whole file or giant diff in a single `content`
block. Blank lines are skipped, and a line that fails `json.Unmarshal` is logged and **skipped**,
so a buggy peer silently loses messages.

**stdout hygiene.** Nothing but ACP frames on stdout — no `fmt.Println`, `log.Print`, or library
banner, or the stream is corrupt. Send logs to `os.Stderr` (skeleton does). #1 first-run failure.

**Request vs notification.** `id` present = request, answered exactly once; `id` absent =
notification, never answered (`handleInbound` responds only when `req.ID != nil`). Inbound
requests get one goroutine each (`connection.go:receive`) so your `Agent` methods run
**concurrently** — mutex session state. Notifications are processed **sequentially** through a
bounded queue, so update order is preserved.

**Notification queue overflow.** Capacity 1024 (`defaultMaxQueuedNotifications`); fall further
behind and the connection is **closed** with `errNotificationQueueOverflow`
(`connection.go:443`). A client that blocks inside `SessionUpdate` kills its own connection.

**Response barrier (a feature).** `SendRequest` returns only after every notification enqueued
before it is handled (`waitNotificationsUpTo`); `Prompt()` thus sees all prior updates. No flush.

**Outbound cancellation.** Cancelling the ctx of any outbound call (`SessionUpdate`,
`RequestPermission`, `fs/*`, `terminal/*`) sends `$/cancel_request` with that request id and
abandons the wait (`connection.go:waitForResponse`); the peer must still answer the original
request (partial result or `-32800`). Pending cancels cap at 1024, dropped silently.

**Backpressure.** One writer mutex (`writeMu`), no write deadline: if the peer stops reading,
the next `SessionUpdate` blocks forever → blocks `Prompt` → blocks the turn. Bound your producer,
check `ctx.Done()`/`conn.Done()` before long writes, never call `SessionUpdate` from a goroutine
you cannot abandon.

**Ordering.** `session/cancel` cancels `Prompt`'s ctx and then calls `Cancel`; return promptly,
and flush outputs before returning — the spec forbids updates *after* the `session/prompt`
response.

**Nil vs empty slices.** Required-array fields have no `omitempty`. Verified:
`json.Marshal(acp.NewSessionRequest{Cwd:"/tmp"})` → `{"cwd":"/tmp","mcpServers":null}`, and
`NewSessionRequest.Validate()` rejects nil (`"mcpServers is required"`). An SDK client that
forgets `McpServers: []acp.McpServer{}` gets `-32602` from any conforming agent. Same class of
bug for `AuthMethods` and `Prompt`.

**Zero-valued capability objects still marshal.** `InitializeResponse{ProtocolVersion:1}` →
`"agentCapabilities":{"auth":{},"mcpCapabilities":{},"promptCapabilities":{},"sessionCapabilities":{}}`
plus `"authMethods":[]`. Harmless today, but it *does* advertise an empty `sessionCapabilities`.

**Error laundering.** Anything not a `*acp.RequestError` becomes `-32603`; use
`NewInvalidParams`, `NewAuthRequired`, `NewRequestCancelled`, `NewMethodNotFound` deliberately.

**Unstable interfaces.** `AgentExperimental` dispatches by type assertion: implement it and
panic in a method you never meant to serve, and a stray call becomes a connection failure.

## 10. Shape for the Antigravity adapter

1. One `Agent` impl, one `AgentSideConnection` on stdio (`cmd/agy-acp`, spawned by the client).
   No listener, no handshake beyond `initialize`.
2. `Initialize`: `loadSession` only if you can replay Antigravity history; `embeddedContext` if
   you accept `resource` blocks; empty `mcpCapabilities` unless you really spawn MCP servers;
   `sessionCapabilities.close` if the bridge can tear down an `agy` session.
3. `NewSession`: map `cwd` (+ `additionalDirectories` if advertised) onto the Antigravity
   workspace, attach the runtime (CDP or `agy` CLI), return your own session handle.
4. `Prompt`: translate `ContentBlock[]` into a host turn; stream host output as
   `agent_message_chunk`, reasoning as `agent_thought_chunk`, host actions as `tool_call` →
   `tool_call_update` (with `kind` and `locations` for follow-along), multi-step work as `plan`
   (full replacement each time). Sensitive actions gate on `session/request_permission`
   (`allow_once`/`reject_once`) mapped onto the host's own approval path — no second approval UI.
5. `Cancel`/ctx cancellation aborts the host turn and still returns `StopReasonCancelled`, never
   an error. Logs go to stderr or the daemon's SQLite log, never stdout.
