# Antigravity language server API — can an ACP adapter drive conversations?

Date 2026-10-22. Antigravity 2.25.0 (desktop) + extension `google.google-antigravity-1.9.0`, Linux.
Every claim below has a command and its real output, or a file path.

## Verdict

**Yes.** The language server exposes a full **Connect (buf.build) RPC surface over loopback HTTP**,
guarded by one CSRF header. The whole conversation loop works from outside the UI. Live proof,
verbatim, against the running desktop instance:

```text
StartCascade            -> {"cascadeId":"c0a5d193-cc98-4d65-aa8b-12fd405961ca"}
SendUserCascadeMessage  -> {}                        (cascadeConfig.plannerConfig.planModel set)
GetCascadeTrajectorySteps -> {"type":"CORTEX_STEP_TYPE_PLANNER_RESPONSE",
                              "plannerResponse":{"response":"ACP_PROBE_OK", ...}}
```

On top of that the same binary ships an explicit headless surface: an `agentapi` subcommand
(`new-conversation` / `send-message` / `get-conversation-metadata`) and the `agy` CLI with
`-p --output-format stream-json`. One backend serves both surfaces: desktop app and VS Code
extension. DOM scraping is unnecessary.

## 1. Wire protocol

**Not REST.** One POST endpoint per method: `/<fq-service>/<Method>`, JSON or binary protobuf. The
server advertises what it accepts — `Accept-Post: application/connect+json, application/connect+proto,
application/grpc, application/grpc+json, application/grpc-web, ...` — on the `415` you get when
POSTing `application/json` to a server-streaming method.

Connect-JSON is the cheapest path: `application/json` for unary, `application/connect+json` with a
5-byte envelope (`0x00` + big-endian uint32 length + body) for streaming.

**Auth header: `x-codeium-csrf-token`.**

```bash
curl -s -i -X POST http://127.0.0.1:33103/exa.language_server_pb.LanguageServerService/GetStatus \
  -H "Content-Type: application/json" -d '{}'
```
```text
HTTP/1.1 401 Unauthorized
{"code":"unauthenticated","message":"unauthenticated: missing CSRF token"}
```

Health probe, no token needed:

```bash
curl -s http://127.0.0.1:33103/healthz
# {"instanceId":"4ca74bd4-152c-4a5c-900c-f5afc43ff5f4","status":"ok"}
```

The binary ships a skill doc telling its own agents to use exactly this shape
(`strings -n 8 /opt/Antigravity/resources/bin/language_server | sed -n '208726,208745p'`):

```text
: "${ANTIGRAVITY_LS_ADDRESS:=localhost:5387}"
lsrpc() {  # usage: lsrpc <MethodName> <json-body>
  curl -sS -X POST "http://${ANTIGRAVITY_LS_ADDRESS}/exa.language_server_pb.LanguageServerService/$1" \
    -H "Content-Type: application/json" -H "x-codeium-csrf-token: ${ANTIGRAVITY_CSRF_TOKEN}" -d "$2"
```

## 2. Port and CSRF discovery

Each process listens twice — one HTTPS/HTTP2 (UI + gRPC) socket, one plaintext HTTP socket. Desktop,
pid 1299142 (`tr '\0' ' ' < /proc/1299142/cmdline`):

```text
/opt/Antigravity/resources/bin/language_server --standalone --override_ide_name antigravity
  --subclient_type hub --override_ide_version 2.25.0 --override_user_agent_name antigravity
  --https_server_port 0 --csrf_token 600a5e3f-3f5a-40e9-8866-c88d67b7dcb8
  --app_data_dir antigravity --enable_sidecars
  --host_bridge_url=http://127.0.0.1:36991 --host_bridge_token=4c28e382...
```

`--https_server_port 0` means OS-assigned, so the port is only known from the **stderr banner**,
which Electron tees into a log:

```bash
grep -n "port at" ~/.config/Antigravity/logs/language_server.log | tail -2
```
```text
5: I1023 00:58:38 server.go:684] Language server listening on random port at 43191 for HTTPS (gRPC)
6: I1023 00:58:38 server.go:692] Language server listening on random port at 33103 for HTTP
```

Those match the live sockets (`ss -tlnp | grep language_server` → `127.0.0.1:43191 fd=20`,
`127.0.0.1:33103 fd=23`). `43191` refuses plaintext (`Client sent an HTTP request to an HTTPS
server`); `33103` is plaintext HTTP/1.1 + h2c. Both answer `/healthz` with the same `instanceId`.

CSRF token is in argv, readable same-user with no privileges:

```bash
tr '\0' ' ' < /proc/1299142/cmdline | grep -o 'csrf_token [^ ]*'   # 600a5e3f-...
curl -sk https://127.0.0.1:43191/ | grep -o '"csrfToken":"[^"]*"'  # same token, from the web UI
```

**No port file exists** here (`find ~/.gemini ~/.config -name 'ls_*.json'` → nothing). The binary
does carry the struct `{pid, httpsPort, httpPort, lspPort, lsVersion, csrfToken}` and the format
string `ls_%s.json`, so some launch mode writes one — not the `--standalone` / `--hub` modes seen.
**Log-line parsing is the reliable route today.** The VS Code extension does not need discovery: it
picks the port and passes `--hub-port`.

## 3. Service inventory

Extracted from the Go binary's embedded `FileDescriptorProto` blobs. Main file
`third_party/jetski/language_server_pb/language_server.proto`, package `exa.language_server_pb`,
**346 RPCs, 20 server-streaming**.

Descriptors are stored uncompressed: each `.proto` string is preceded by `0x0a <varint len>` and
decodes forward as `FileDescriptorProto` (companion script:
`.rpiv/artifacts/research/language-server-descriptor-extract.py`):

```python
def locate(name):                       # -> offsets of the 0x0a tag
    nb = name.encode(); out = []; i = 0
    while True:
        o = data.find(nb, i)
        if o < 0: return out
        i = o + 1
        for k in (1, 2, 3):
            ln, j = rv(data, o - k)
            if j == o and ln == len(nb) and data[o-k-1] == 0x0a: out.append(o - k - 1)
# python3 language-server-descriptor-extract.py \
#   third_party/jetski/language_server_pb/language_server.proto svc
```

Other services: `exa.extension_server_pb.ExtensionServerService` (56 LS→editor callbacks),
`exa.chat_client_server_pb.ChatClientServerService`, `exa.api_server_pb.ApiServerService`
(telemetry/upstream), `exa.remoting.RemotingService` (remote control),
`exa.host_bridge_pb.HostBridgeService` (3 RPCs the Electron main process serves to the LS),
`exa.conversation_pb` / `jetski.product.v1.ConversationService`, plus index/analytics/plugin/dev
services an adapter does not need.

### Conversation control

| RPC | In → Out | Purpose |
| --- | --- | --- |
| `StartCascade` | `StartCascadeRequest` → `{cascadeId, projectEnvInfo}` | create conversation |
| `SendUserCascadeMessage` | `SendUserCascadeMessageRequest` → `{}` | send user turn |
| `GetCascadeTrajectory` / `GetCascadeTrajectorySteps` | `{cascadeId[, stepOffset]}` → trajectory / steps | read |
| `GetAllCascadeTrajectories` / `GetConversationMetadata` | `{}` / `{conversationId}` | list / one |
| `WaitForConversationFullyIdle` | `{conversationId, inactivityTimeoutSeconds, stabilizationDurationSeconds}` → `{timedOut}` | turn completion |
| `CancelCascadeInvocation` | `{cascadeId, killBackgroundTasks, notifyParent}` → `{}` | interrupt |
| `ForceStopCascadeTree`, `CancelCascadeSteps` | `{cascadeId}` | harder stops |
| `HandleCascadeUserInteraction` | `{cascadeId, interaction}` → `{}` | answer permission prompts |
| `SendAllQueuedMessages`, `DeleteQueuedUserInputStep` | queue control | queued turns |
| `ForkConversation`, `DeleteCascadeTrajectory` | lifecycle | branch / delete |
| `RevertToCascadeStep`, `GetRevertPreview` | `{cascadeId, stepIndex}` | undo |
| `GetTurnDiff`, `GetTrajectoryFileDiffs`, `GetPatchAndCodeChange` | `{cascadeId}` | file mutations |
| `GetCascadeModelConfigData`, `WarmUpAgentProcess`, `GetResolvedCascadeConfig` | — | catalogue / latency |

### Key shapes (field numbers from the descriptor)

```text
StartCascadeRequest {
  CortexTrajectorySource source = 4;         // enum, REQUIRED
  CortexTrajectoryType trajectory_type = 5;  // enum
  repeated string workspace_uris = 8;        // file:/// URIs
  Model requested_model = 14;                // enum
  Metadata metadata = 1; string cascade_id = 7; string parent_conversation_id = 12;
  ProjectEnvironmentConfig project_env_config = 17; repeated string tags = 18; ... }
StartCascadeResponse { string cascade_id = 1; ProjectEnvironmentInfo project_env_info = 2; }

SendUserCascadeMessageRequest {
  string cascade_id = 1;                      // required
  repeated TextOrScopeItem items = 2;         // TextOrScopeItem { string text = 1; ContextScopeItem item = 2; }
  CascadeConfig cascade_config = 5;           // plannerConfig.planModel (enum) required in practice
  bool blocking = 8; repeated Step additional_steps = 9;
  MessageDeliveryStrategy delivery_strategy = 22; ... }
SendUserCascadeMessageResponse {}             // empty — payload arrives via trajectory

GetCascadeTrajectoryStepsRequest { string cascade_id = 1; uint32 step_offset = 2;
  ClientTrajectoryVerbosity verbosity = 3; bool disable_rehydration = 5; }
CancelCascadeInvocationRequest { string cascade_id = 1; bool kill_background_tasks = 2;
  bool notify_parent = 3; }
WaitForConversationFullyIdleRequest { string conversation_id = 1; int32 inactivity_timeout_seconds = 2;
  int32 stabilization_duration_seconds = 3; bool return_on_executor_error = 4; }
```

`exa.cortex_pb` supplies `CascadeConfig { planner_config, checkpoint_config, executor_config, ... }`
and `CascadePlannerConfig { Model plan_model = 1; ModelOrAlias requested_model = 15; string model_name = 28; ... }`.
**`Model` is an enum, not a message** — passing an object is rejected with `invalid value for enum
field planModel`. `Trajectory` = `{trajectory_id, cascade_id, repeated Step steps, ...}`; each `Step`
has `type` (`CORTEX_STEP_TYPE_USER_INPUT`, `_PLANNER_RESPONSE`, `_RUN_COMMAND`, `_CODE_ACTION`,
`_GREP_SEARCH`, `_ERROR_MESSAGE`, `_SYSTEM_MESSAGE`, …) plus a matching typed payload member.

## 4. Live proof against the running desktop instance

```bash
T=600a5e3f-3f5a-40e9-8866-c88d67b7dcb8
B=http://127.0.0.1:33103/exa.language_server_pb.LanguageServerService
H=(-H "Content-Type: application/json" -H "x-codeium-csrf-token: $T")
```

#### a. Health, status, model catalogue (PII redacted)

```text
$ curl ... $B/GetStatus          -> {}        $ curl ... $B/GetCapabilities
                                              -> {"supportsHookResultProtoBytes":true}
$ curl ... $B/GetUserStatus      -> {"userStatus":{"planStatus":{"planInfo":
   {"teamsTier":"TEAMS_TIER_PRO","planName":"Pro", ...}}, "cascadeModelConfigData":
   {"clientModelConfigs":[{"label":"Gemini 3.8 Flash (Medium)","modelId":"gemini-3.8-flash-medium",
    "modelOrAlias":{"model":"MODEL_PLACEHOLDER_M319"}}, ...]}}}
```

#### c. List conversations, read typed step events

```text
$ curl ... $B/GetAllCascadeTrajectories -d '{}'
{"trajectorySummaries":{"59bc9e12-30c0-4718-8afe-70237e616a92":{"stepCount":362,
 "status":"CASCADE_RUN_STATUS_IDLE","workspaces":[{"workspaceFolderAbsoluteUri":
 "file:///mnt/Data/Projects/Antigravity%20Swiss%20Knife","branchName":"main"}], ...}}}

$ curl ... $B/GetCascadeTrajectorySteps -d '{"cascadeId":"59bc9e12-...","stepOffset":0}'
{"steps":[{"type":"CORTEX_STEP_TYPE_SYSTEM_MESSAGE","status":"CORTEX_STEP_STATUS_DONE",
  "systemMessage":{"message":"[Message] timestamp=... content=<original_task>..."}}, ...]}
```

#### e. Create, drive a turn, read the answer — the decisive test

```text
$ curl ... $B/StartCascade -d '{"workspaceUris":["file:///tmp/acp-probe"]}'
400 {"code":"invalid_argument","message":"CortexTrajectorySource is unspecified"}

$ curl ... $B/StartCascade -d '{"workspaceUris":["file:///tmp/acp-probe"],
    "source":"CORTEX_TRAJECTORY_SOURCE_AGENT_API",
    "trajectoryType":"CORTEX_TRAJECTORY_TYPE_CASCADE"}'
200 {"cascadeId":"c0a5d193-cc98-4d65-aa8b-12fd405961ca"}

$ curl ... $B/SendUserCascadeMessage -d '{"cascadeId":"c0a5d193-...",
    "items":[{"text":"Reply with exactly: ACP_PROBE_OK"}]}'
200 {}     # turn then fails: "failed to construct executor: plan model not specified"

$ curl ... $B/SendUserCascadeMessage -d '{"cascadeId":"c0a5d193-...",
    "items":[{"text":"Reply with exactly: ACP_PROBE_OK"}],
    "cascadeConfig":{"plannerConfig":{"planModel":"MODEL_PLACEHOLDER_M319"}}}'
200 {}
$ curl ... $B/WaitForConversationFullyIdle -d '{"conversationId":"c0a5d193-...",
    "inactivityTimeoutSeconds":120,"stabilizationDurationSeconds":2}'          -> 200 {}
$ curl ... $B/GetCascadeTrajectorySteps -d '{"cascadeId":"c0a5d193-...","stepOffset":0}'
{"steps":[
 {"type":"CORTEX_STEP_TYPE_USER_INPUT","status":"CORTEX_STEP_STATUS_DONE",
  "userInput":{"items":[{"text":"Reply with exactly: ACP_PROBE_OK"}],
               "userConfig":{"plannerConfig":{"planModel":"MODEL_PLACEHOLDER_M319"}}}},
 {"type":"CORTEX_STEP_TYPE_PLANNER_RESPONSE","status":"CORTEX_STEP_STATUS_DONE",
  "plannerResponse":{"response":"ACP_PROBE_OK","modifiedResponse":"ACP_PROBE_OK", ...}}]}

$ curl ... $B/DeleteCascadeTrajectory -d '{"cascadeId":"c0a5d193-..."}'    # {} — cleaned up
```

The model returned exactly the requested string. `create → send → await → read` works from outside
the UI while the app runs, proving **a second client attaches** to the live instance.

#### f. Streaming transport

Server-streaming needs `application/connect+json` plus a request envelope (`0x00` + be32 length +
body); raw JSON gives `promised 5777794671 bytes in enveloped message`, no envelope header `415`.
With correct framing the stream opens and immediately reports:

```text
frame flags=0x02 len=114: {"error":{"code":"unknown","message":"reactive state is deprecated"}}
```

Transport works, but **the reactive-component streams are deprecated** (server string:
`reactive state is deprecated`). `StreamAgentStateUpdates` / `RequestAgentStatePageUpdate` exist as
the successor; their descriptors live outside `language_server.proto` and were not exercised.
**Polling `GetCascadeTrajectorySteps` is the proven, non-deprecated read path.**

## 5. VS Code extension — same binary, different launch

`~/.vscode/extensions/google.google-antigravity-1.9.0/extension.js` (4.1 MB, ships a sourcemap):

```js
let Nb = ["--hub", `--hub-port=${Ya}`, "--app_data_dir=antigravity", `--csrf_token=${this.csrfToken}`];
for (let pb of vscode.workspace.workspaceFolders ?? []) Nb.push(`--add-dir=${pb.uri.fsPath}`);
let w = `http://127.0.0.1:${Ya}`;   // Ya = antigravity.serverPort config, else ephemeral port
```

- **binary:** desktop `/opt/Antigravity/resources/bin/language_server`; extension cached by
  `acquireBinaryPath`, run as `~/.gemini/bin/agy`.
- **flags:** desktop `--standalone ... --subclient_type hub`; extension `--hub --hub-port=<n>` plus
  `--add-dir=<workspace folder>` ×N and `--enable_sidecars`/`--host_bridge_*` on the desktop side.
- **port:** desktop `--https_server_port 0`, read from the log; extension picks it
  (`antigravity.serverPort` or ephemeral) and uses `http://`.
- **ready probe:** desktop = stderr banner; extension = `GET /healthz` every 250 ms.

Live hub here — `agy --hub --hub-port=35595 --app_data_dir=antigravity --csrf_token=fe085a49-...`,
listening on `33791` (HTTPS) and `35595` (HTTP):

```text
$ curl -s http://127.0.0.1:35595/healthz
{"instanceId":"82547080-72d3-49d7-a1f4-190f03f86f27","status":"ok"}
$ curl -s -X POST http://127.0.0.1:35595/exa.language_server_pb.LanguageServerService/GetUserStatus \
    -H "Content-Type: application/json" -H "x-codeium-csrf-token: fe085a49-..." -d '{}'
200 {"userStatus":{"disableTelemetry":true,"planStatus":{...}}}     # same schema as desktop
```

Identical API, identical auth, separate process. `agy` is 213.4 MB vs the bundled server's 173.9 MB
— the CLI build wraps the same server plus the `agy` command tree. **One adapter, two surfaces:**
find a loopback listener + CSRF token, then run the same RPCs. Desktop port from the log line;
extension port from `--hub-port` / `antigravity.serverPort`, confirmed by `/healthz`.

## 6. The supported headless surface

The extension's own sidecar runtime calls `agentapi` rather than hand-rolling HTTP (`strings` →
bundled `backend.js`):

```js
const lsAddr = process.env.ANTIGRAVITY_LS_ADDRESS;
const csrf   = process.env.ANTIGRAVITY_CSRF_TOKEN;
// Prefer invoking the language server executable's `agentapi` subcommand directly…
const exe = process.env.ANTIGRAVITY_AGENTAPI_EXE;
const execArgs = exe ? ['agentapi', ...args] : args;
// sendMessage:             ['send-message', ...(title?['--title',t]:[]), conversationId, message]
// startConversation:       ['new-conversation','--model',model, message]
// getConversationMetadata: ['get-conversation-metadata', conversationId]
```

Proven on the live desktop instance:

```text
$ export ANTIGRAVITY_LS_ADDRESS=127.0.0.1:33103 ANTIGRAVITY_CSRF_TOKEN=600a5e3f-...
$ agy agentapi --help
Usage: agentapi <command> [args]
  get-conversation-metadata <conversation_id>
  new-conversation [--model=<flash_lite|flash|pro>] [--title=<title>] [--profile=<profile>] <prompt>
  send-message [--title=<title>] <recipient_id> <content>

$ agy agentapi get-conversation-metadata 59bc9e12-30c0-4718-8afe-70237e616a92
{ "response": { "conversationMetadata": { "metadata": {
    "workspaces": [{ "workspaceFolderAbsoluteUri":
       "file:///mnt/Data/Projects/Antigravity%20Swiss%20Knife", "branchName": "main" }],
    "createdAt": "2026-10-22T13:34:36.561035573Z", ...
```

`agy` is itself a headless agent CLI: `-p/--print`, `--output-format text|json|stream-json`,
`--input-format text|stream-json` (NDJSON, one turn per line), `--conversation <id>`,
`--mode accept-edits|auto|plan`, `--model`, `--project`, `--agent`, `--effort`, `--json-schema`,
`--sandbox`, `--dangerously-skip-permissions`; subcommands `agent(s)`, `mcp`, `models`, `plugin(s)`,
`remote-control`, `update`, `agentapi`.

That the RPC has a shipped CLI and an `agentapi` verb is the strongest signal it is *meant* for
programmatic use, not just UI plumbing.

## 7. Adapter design sketch

```text
 1. Discovery: scan /proc/*/cmdline for language_server|agy + csrf_token; --hub/--hub-port gives the
    port directly; --https_server_port 0 needs the log tail. Verify GET /healthz; cache {base, token}.
 2. Transport: POST <base>/exa.language_server_pb.LanguageServerService/<Method>, headers
    {Content-Type: application/json, x-codeium-csrf-token}. Unary only.
 3. session/new    -> StartCascade{source:"CORTEX_TRAJECTORY_SOURCE_AGENT_API",
                    trajectoryType:"CORTEX_TRAJECTORY_TYPE_CASCADE", workspaceUris:[...]} -> cascadeId
 4. session/prompt -> SendUserCascadeMessage{cascadeId, items:[{text}],
                      cascadeConfig:{plannerConfig:{planModel:"MODEL_PLACEHOLDER_M319"}}}
 5. session/update -> poll GetCascadeTrajectorySteps{cascadeId, stepOffset} on a cursor; map Step.type:
    USER_INPUT->user_message, PLANNER_RESPONSE->agent_message_chunk,
    RUN_COMMAND|GREP_SEARCH|CODE_ACTION->tool_call (+tool_call_update from status),
    ERROR_MESSAGE->error. Turn end = WaitForConversationFullyIdle.
 6. Permissions -> HandleCascadeUserInteraction{cascadeId, interaction}.
 7. Cancel      -> CancelCascadeInvocation{cascadeId, killBackgroundTasks:true}.
 8. Model select -> GetUserStatus.cascadeModelConfigData.clientModelConfigs (label, modelId,
    modelOrAlias.model); planModel wants the enum, not the modelId string.
 9. Resume       -> GetAllCascadeTrajectories / GetCascadeTrajectory.
10. Smoke test  -> shell out to `agy agentapi new-conversation|send-message`; same backend.
```

`SendUserCascadeMessage` returns `{}` immediately, so `session/prompt` must take its result from the
trajectory or `WaitForConversationFullyIdle`. `blocking: true` (field 8) exists, unverified.

## 8. Blockers and what is unproven

Nothing hard blocks it. Friction, ranked: (1) desktop port discovery is log-text parsing — no
`ls_*.json` appears in `--standalone`/`--hub` launches here, so add a configurable port override;
(2) the CSRF token lives in argv (same-user only, acceptable for a local daemon) and is minted fresh
per launch, so cache keyed on instance; (3) `cascadeConfig.plannerConfig.planModel` is mandatory —
missing it yields `failed to construct executor: plan model not specified`, and the catalogue
`modelId` (`gemini-3.8-flash-medium`) is not the enum (`MODEL_PLACEHOLDER_M319`), map
`modelOrAlias.model`; (4) reactive streams are deprecated, so updates are polled; (5) no published
protobuf package, but descriptors extract from the binary and Connect-JSON lets an adapter ship
`map[string]any` first; (6) no compatibility contract on these internal `exa.*` protos — everything
here is pinned to 2.25.0.

**Unproven / not verified:**

- `StreamAgentStateUpdates` and `RequestAgentStatePageUpdate` (descriptors outside
  `language_server.proto`; the replacement for the deprecated reactive streams).
- Whether `blocking: true` makes `SendUserCascadeMessage` synchronous.
- `HandleCascadeUserInteraction` against a real permission prompt (shape known, not exercised).
- `--remote-control` / `RemotingService.ConnectRemote` as a transport for a non-local adapter.
- Concurrent writers on one cascade: concurrent *reads* worked; writes went to a fresh cascade.
- `ls_<id>.json` port-file behaviour in launch modes other than `--standalone` / `--hub`.
