
### Decisions to Challenge
- Decision 1 (Isolation via HOME/USERPROFILE): Unsound on Windows (APPDATA/LOCALAPPDATA remain un-scoped; os.Symlink requires elevated privileges, falling back to non-updating static copies) and fragile if Antigravity switches to app.getPath('userData') because PrepareIsolatedHome populates state in ~/.gemini/antigravity rather than the userData directory.
- Decision 2 (Mounting /v1/ and exempting from GUI session auth): Right decision in concept (external tools and CLI only possess the gateway bearer token AGY_LLM_GATEWAY_API_KEY, not internal GUI session tokens), but the implementation fails open when the token is empty and relays OAuth in isolation mode.
- Decision 3 (x-opencode-session derived from opening user prompt): Breaks immediately upon context truncation/windowing (the first message in context changes, mutating the hash mid-dialogue) and collides across concurrent conversations sharing common openings ("Hi", "Explain this").
- Decision 4 (GET /v1beta/models asymmetry): Indefensible and broken — in api_proxy mode /v1beta/models advertises native models, but /v1beta/models/{id}:streamGenerateContent returns 404 for all native models.
- Decision 5 (authorizeExternal empty PIN allowed): Unsafe — treats unconfigured external access as wide open while warning of increased ban risk, and compares PINs without constant-time equality.
- Decision 6 (Accidental OAuth relay in default mode): Violated — /v1/chat/completions relays native Gemini models through local Google OAuth in profile_isolation mode.
- Decision 7 (PrepareIsolatedHome warnings on credential failure): Dangerous — mirrors fail silently, launching the isolated app in an unauthenticated or corrupted state.

### Blockers

#### 1. Multi-turn tool execution completely broken on /v1/chat/completions
- File & Line:  pkg/custommodels/gateway.go#L1124-L1133 ,  pkg/custommodels/gateway.go#L792-L806 ,  pkg/custommodels/gateway.go#L1053-L1054 ,  pkg/custommodels/gateway.go#L1072 ,  pkg/custommodels/gateway.go#L1109
- What is wrong: openAIRequestToGemini assigns Name: message.ToolCallID to FunctionResponse.Name (violating Gemini wire protocol where Name must be the function name, e.g. read_file), and discards any non-JSON tool output because json.Unmarshal fails on plain text. geminiToOpenAIMessages fabricates synthetic IDs (swiss-call-%s-%d) disconnected from the client's tool_call_id, causing downstream OpenAI providers to reject subsequent turns with tool_call_id does not match. handleOpenAIChatCompletion hardcodes id: "call_swiss_0" and index: 0 for all tool calls in both streaming and non-streaming modes, corrupting parallel tool calls. Both streaming and non-streaming endpoints emit finish_reason: "stop" instead of "tool_calls".
- openAIRequestToGemini assigns Name: message.ToolCallID to FunctionResponse.Name (violating Gemini wire protocol where Name must be the function name, e.g. read_file), and discards any non-JSON tool output because json.Unmarshal fails on plain text.
- geminiToOpenAIMessages fabricates synthetic IDs (swiss-call-%s-%d) disconnected from the client's tool_call_id, causing downstream OpenAI providers to reject subsequent turns with tool_call_id does not match.
- handleOpenAIChatCompletion hardcodes id: "call_swiss_0" and index: 0 for all tool calls in both streaming and non-streaming modes, corrupting parallel tool calls.
- Both streaming and non-streaming endpoints emit finish_reason: "stop" instead of "tool_calls".
- Why it matters: Any tool-calling turn executed through /v1/chat/completions fails on turn 2 with protocol errors, drops CLI tool outputs, or stops agent execution because finish_reason is never "tool_calls".
- Smallest correct change: Preserve original tool call IDs in a request-scoped map; populate FunctionResponse.Name with the function name; wrap plain string outputs as map[string]interface{}{"output": text}; index tool calls dynamically; and emit finish_reason: "tool_calls" when calls are present.

#### 2. /v1beta/ generation in api_proxy mode unconditionally returns 404
- File & Line:  pkg/custommodels/gateway.go#L174-L185  (cf.  pkg/custommodels/gateway.go#L125-L138 )
- What is wrong: In api_proxy mode, handleListModels returns native Gemini models (models/gemini-2.0-flash). However, handleModelAction only searches g.enabledModels() (custom models) and immediately returns HTTP 404 unknown model for any native model.
- Why it matters: The Gemini CLI surface (AGY_LLM_GATEWAY_URL with Gemini protocol) cannot execute a single turn in api_proxy mode.
- Smallest correct change: In handleModelAction, check if target == nil && g.isNativeModel(r.Context(), modelID): route native models to g.runNativeGemini instead of returning 404.

#### 3. Native Gemini OAuth traffic relayed in default profile_isolation mode
- File & Line:  pkg/custommodels/gateway.go#L953-L955 ,  pkg/custommodels/gateway.go#L1001 ,  pkg/custommodels/native.go#L234-L239
- What is wrong: handleOpenAIListModels lists native Gemini models in all modes, and handleOpenAIChatCompletion executes them via runNativeGemini using ~/.gemini/oauth_creds.json even when Mode == IntegrationProfileIsolation.
- Why it matters: Violates the core product invariant: "never relay native Gemini traffic through the daemon in the default mode".
- Smallest correct change: In handleOpenAIListModels and handleOpenAIChatCompletion, gate native model inclusion and dispatch behind if g.store.IntegrationSettings().Mode == IntegrationAPIProxy. In profile_isolation mode, reject native models with HTTP 404 or 403.

#### 4. Fail-open gateway authentication
- File & Line:  pkg/custommodels/gateway.go#L84-L86
- What is wrong: authorize returns true when g.token == "".
- Why it matters: If Store.GatewayToken() encounters a read/write failure or uninitialized config, all gateway endpoints (/v1/* and /v1beta/*) become unauthenticated on the network.
- Smallest correct change: Change line 85 to return false.

#### 5. HTTP server WriteTimeout: 30s severs streaming LLM turns
- File & Line:  pkg/webgui/server.go#L624
- What is wrong: s.httpServer enforces WriteTimeout: 30 * time.Second. In Go's http.Server, WriteTimeout is an absolute deadline from request headers to completion that does not reset on chunked SSE writes.
- Why it matters: Any streaming generation or reasoning model (DeepSeek-R1, thinking models, large completions) taking longer than 30 seconds is killed mid-stream with i/o timeout.
- Smallest correct change: Set WriteTimeout: 0 on s.httpServer or clear write deadlines on streaming handlers using http.NewResponseController(w).SetWriteDeadline(time.Time{}).

#### 6. VS Code custom model mode has no inference path
- File & Line:  pkg/process/surfaces.go#L125-L128 ,  pkg/custommodels/proxy.go#L82-L85 ,  pkg/custommodels/proxy.go#L99-L102
- What is wrong: ApplyVSCodeModelSource sets --cloud_code_endpoint and --app_data_dir. However, app_model_overrides was removed, proxy.go passes fetchAvailableModels through verbatim to Google without custom models, and proxy.go passes unknown models through to Google Cloud Code.
- Why it matters: Violates the rule: "never present a switch that has no inference path behind it". Selecting "Custom models" for VS Code leaves the UI unable to pick custom models, and all turns route directly to Google.
- Smallest correct change: Either inject enabled custom models into fetchAvailableModels in proxy.go and map the active model, or remove the VS Code custom model toggle from the UI until implemented.

### Should Fix

#### 7. Subagents CLI delegation switch is an unimplemented UI facade
- File & Line:  frontend/src/components/ModelSourceTab.tsx#L190-L202 ,  pkg/process/surfaces.go#L38 ,  pkg/webgui/server.go#L3369
- What is wrong: profile.Subagents is saved in config but completely ignored by LaunchAntigravityProfile, LaunchVSCode, and the backend.
- Why it matters: Violates repository invariant #1 (Zero Artificial Previews).
- Smallest correct change: Remove the subagent segmented control from ModelSourceTab.tsx until subagent CLI delegation is actually wired into launcher process flags or environment.

#### 8. OpenCode legacy URL rewriting is broken in ResolveEndpoint
- File & Line:  pkg/custommodels/tester.go#L315-L318 ,  pkg/custommodels/tester.go#L393-L399
- What is wrong: Lines 315-317 return baseURL untouched if it ends with /chat/completions. If a legacy entry has https://opencode.ai/zen/go/v1/chat/completions, it is
---

## Disposition (what was fixed after the review)

| Finding | Status |
|---|---|
| B1 tool-call protocol (name/id/plain-text/finish_reason) | Fixed: `toolCallID`/`toolNameFromID` encode the function name in the id so echoes map back without server state; plain-text tool output wrapped as `{"output":…}`; per-call index/id; `finish_reason: "tool_calls"` in both stream and non-stream. Tests: `toolcall_test.go`. |
| B2 `/v1beta` generation 404 for native models in proxy mode | Fixed: `handleModelAction` routes native ids to `runNativeGemini` when `Mode == api_proxy`. |
| B3 native OAuth relayed in default mode | Fixed: native ids are listed and dispatched only in api_proxy mode on both surfaces; isolation mode 404s them. `TestOpenAISurfaceListsBothFamilies` now sets proxy mode. |
| B4 fail-open gateway auth | Fixed: `authorize` returns false when no token is configured. |
| B5 30s WriteTimeout severs SSE | Fixed: `WriteTimeout: 0` with a comment explaining net/http's absolute deadline. |
| B6 VS Code custom mode has no inference path | Gated: `ApplyVSCodeModelSource` refuses `custom` with an explicit message; the UI shows VS Code as Native only. |
| S7 subagents switch unimplemented | Removed from `ModelSourceTab`; backend field retained but inert until delegation exists. |
| S8 legacy `/zen/go/v1/chat/completions` passthrough | Fixed: any `/zen/go` path maps to the canonical inference endpoint; `/v1` base appends `/chat/completions`. |
| D5 PIN comparison / empty PIN | Constant-time comparison added. Empty PIN still means "token only" — deliberate, documented. |
| D7 silent credential mirror failures | Credentials (`oauth_creds.json`, `google_accounts.json`, `config`, `~/.config/antigravity`) are now fatal; other entries still warn. |
| D1 Windows APPDATA / symlink copies | Accepted with rationale: Gemini state comes from `os.homedir()`, and `--user-data-dir` covers Chromium, so APPDATA is not on the state path; copies are refreshed at every launch, so they are as fresh as the launch. |
| D3 session id derivation | Investigated live: the app sends no conversation id (headers/body captured), so the derivation stands. |

Verification after fixes: `go build ./...`, `go vet`, 25 packages green, frontend `tsc` clean and bundle rebuilt, daemon redeployed, gateway smoke test returns `post-review-ok`.
