# Review brief: channel relay, backend choice, default project

Review the code, not this summary. Findings as Blocker / Should fix / Consider, each with
file:line, the failure mode in practice, and the smallest correct change. Do not restate what works.

## What changed

**Relay (`pkg/bridge/channels_relay.go`, `pkg/bridge/channels_telegram.go`, `pkg/bridge/httpjson.go`)**
- `ParseWhatsAppInbound` keeps text messages, ignores status callbacks and non-text payloads.
- `ParseTelegramInbound` keeps `message.text`, ignores edits/joins/callbacks.
- `MarkMessageSeen` dedupes by message id with a 48h in-memory window.
- `Service.AnswerChannelMessage(text, project)` runs one turn on the configured backend
  (`s.agent().CreateConversation` with `{"prompt":…, "cwd": project}`) and extracts reply text from
  `response` / `text` / `message`, falling back to the raw JSON.
- `postJSON` treats any non-2xx as an error carrying the response snippet.

**Endpoints (`pkg/webgui/server.go`, `pkg/webgui/auth_middleware.go`)**
- WhatsApp webhook: validate `verify_token`, parse, dedupe, ack `{"accepted":n}` immediately,
  relay in a goroutine, reply through `graph.facebook.com/v21.0/<phone_id>/messages`.
- Telegram webhook `/api/bridge/channels/telegram/webhook`: token in the query must equal the bot
  token, same ack-then-relay shape, reply through `sendMessage`.
- Both are auth-exempt in the middleware because the platform cannot present a session token.

**Settings (`pkg/bridge/channels.go`, `pkg/bridge/backend.go`, frontend)**
- `ChannelsConfig.AgentBackend` (cli | app | empty = bridge default) and `DefaultProject`.
- `AgentACP` was added and then removed: `agy_acp_server` keeps its own conversation store, so it
  cannot continue the user's projects. `normalizeAgentBackend("acp")` now returns "".
- One shared `ChannelRoutingSettings` component renders both fields on all three channel pages.

## Attack these

1. `AnswerChannelMessage` parses the backend's JSON and guesses keys. What does the CLI backend
   actually return for `CreateConversation`, and does the fallback leak raw JSON to a user's chat?
2. Dedupe is a process-global map with no persistence: restart, or two daemon instances, will
   replay. Is in-memory acceptable, and what is the smallest correct fix?
3. The relay goroutine has no timeout and no concurrency limit. What happens with five messages at
   once, and does a stuck turn leak?
4. The Telegram token check compares the query string against the bot token. Is that defensible,
   and what leaks through logs and proxies?
5. `DefaultProject` is passed as `cwd` without validation. What happens with a relative path, a
   missing directory, or a path the agent cannot write?
6. Removing `AgentACP` left `Config.AgentBackend` values in existing user configs that now
   normalise to "". Is the fallback behaviour right, and is it visible to the user?
7. Webhook acks with 200 before the turn runs. If the turn later fails, the sender never learns.
   What is the right signal, given WhatsApp retries on non-2xx?
8. Inbound Telegram/Discord: only Telegram is wired. Is the Discord gap stated anywhere the user
   can see it, or does the UI imply all three work?

## Verify by running

- `cd "…/Antigravity Swiss Knife" && go test ./pkg/bridge/ ./pkg/webgui/ -count=1`
- `curl` the Telegram endpoint with a wrong token and confirm 403.

## Output

Write to `/mnt/Data/Projects/Antigravity Swiss Knife/.rpiv/artifacts/reviews/channel-relay-review.md`.
