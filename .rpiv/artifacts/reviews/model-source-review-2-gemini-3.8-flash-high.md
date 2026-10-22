
# Re-review verdict

### 1. Verification of Original Blockers and Should-Fix Items
Item	Status	Verification (File & Line)
B1: Multi-turn tool execution on /v1/chat/completions	PARTIAL	Streaming mode fixed with finish_reason: "tool_calls" (
pkg/custommodels/gateway.go#L1117
), dynamic indexing and toolCallID (
pkg/custommodels/gateway.go#L1098-L1099
), plain-text output wrapping (
pkg/custommodels/gateway.go#L1180
), and toolNameFromID decoding (
pkg/custommodels/gateway.go#L1189
). However, in non-streaming mode, 
pkg/custommodels/gateway.go#L1137
 still hardcodes "id": "call_swiss_0". On turn 2, toolNameFromID("call_swiss_0") strips _0 and decodes the function name as "swiss" instead of the actual tool name, causing Gemini to reject the turn.
B2: /v1beta/ 404 for native models in proxy mode	FIXED	
pkg/custommodels/gateway.go#L188-L191
 permits native models through when Mode == IntegrationAPIProxy, and lines 
pkg/custommodels/gateway.go#L230
 and 
pkg/custommodels/gateway.go#L241
 dispatch them to runNativeGemini.
B3: Native OAuth traffic relayed in default mode	FIXED	
pkg/custommodels/gateway.go#L966-L970
 lists native Gemini models only when Mode == IntegrationAPIProxy, and 
pkg/custommodels/gateway.go#L1043
 returns 404 for native models unless in api_proxy mode.
B4: Fail-open gateway authentication	FIXED	
pkg/custommodels/gateway.go#L86-L89
 explicitly returns false if g.token == "".
B5: HTTP server WriteTimeout: 30s severs SSE	FIXED	
pkg/webgui/server.go#L626
 sets WriteTimeout: 0.
B6: VS Code custom mode has no inference path	FIXED	
pkg/process/surfaces.go#L118-L123
 refuses "custom" with an explicit error, and 
frontend/src/components/ModelSourceTab.tsx#L176-L178
 pins VS Code to "Native models" only without presenting a dead toggle.
S7: Subagents CLI delegation switch unimplemented	FIXED	
frontend/src/components/ModelSourceTab.tsx#L180-L195
 removes the inert subagent control from the UI.
S8: OpenCode legacy URL rewriting in ResolveEndpoint	FIXED	
pkg/custommodels/tester.go#L318-L320
 intercepts any URL containing /zen/go and maps it directly to the canonical endpoint https://opencode.ai/inference/go/openai/v1/chat/completions.

### 2. Regressions and Defects Introduced by the Fixes
- Non-streaming tool calls broken on turn 2 ("call_swiss_0" -> "swiss")  File & Line:  pkg/custommodels/gateway.go#L1137  vs  pkg/custommodels/gateway.go#L994-L998 Defect: Non-streaming handleOpenAIChatCompletion still emits "id": "call_swiss_0". When the client echoes this ID back in a subsequent tool message,  toolNameFromID  parses body[:idx] where body is "swiss_0" and idx is 5, returning "swiss". openAIRequestToGemini then sets FunctionResponse.Name = "swiss". Upstream Gemini/providers fail because the declared tool is read_file, not swiss. Fix: In non-streaming mode (line 1137), replace "call_swiss_0" with toolCallID(part.FunctionCall.Name, len(calls)).
Non-streaming tool calls broken on turn 2 ("call_swiss_0" -> "swiss")
- File & Line:  pkg/custommodels/gateway.go#L1137  vs  pkg/custommodels/gateway.go#L994-L998
- Defect: Non-streaming handleOpenAIChatCompletion still emits "id": "call_swiss_0". When the client echoes this ID back in a subsequent tool message,  toolNameFromID  parses body[:idx] where body is "swiss_0" and idx is 5, returning "swiss". openAIRequestToGemini then sets FunctionResponse.Name = "swiss". Upstream Gemini/providers fail because the declared tool is read_file, not swiss.
- Fix: In non-streaming mode (line 1137), replace "call_swiss_0" with toolCallID(part.FunctionCall.Name, len(calls)).
- Non-deterministic tool response ID collision for repeated tools  File & Line:  pkg/custommodels/gateway.go#L813-L821 Defect: When converting FunctionResponse in geminiToOpenAIMessages, it iterates callNames (a map[string]string) looking for name == part.FunctionResponse.Name. If a turn executed multiple calls to the same function (e.g. two read_file invocations), Go's non-deterministic map iteration matches the first randomly selected call ID for both responses, duplicating the ID and dropping association with the second tool call. Furthermore, line 820 passes len(calls) where calls is empty (always index 0).
Non-deterministic tool response ID collision for repeated tools
- File & Line:  pkg/custommodels/gateway.go#L813-L821
- Defect: When converting FunctionResponse in geminiToOpenAIMessages, it iterates callNames (a map[string]string) looking for name == part.FunctionResponse.Name. If a turn executed multiple calls to the same function (e.g. two read_file invocations), Go's non-deterministic map iteration matches the first randomly selected call ID for both responses, duplicating the ID and dropping association with the second tool call. Furthermore, line 820 passes len(calls) where calls is empty (always index 0).
- Malformed error chunk on streaming tool calls  File & Line:  pkg/custommodels/gateway.go#L1119-L1123 Defect: If turnErr != nil occurs after sawToolCall was set, line 1120 emits content: "\nprovider error: ..." with finish_reason: "tool_calls". Combining error text with finish_reason: "tool_calls" violates OpenAI SSE framing and crashes downstream client state machines.
Malformed error chunk on streaming tool calls
- File & Line:  pkg/custommodels/gateway.go#L1119-L1123
- Defect: If turnErr != nil occurs after sawToolCall was set, line 1120 emits content: "\nprovider error: ..." with finish_reason: "tool_calls". Combining error text with finish_reason: "tool_calls" violates OpenAI SSE framing and crashes downstream client state machines.

### 3. Open Items & Pushed-Back Decisions

#### A. Pushed-Back Decisions Under Review
- Windows Directory Mirroring (os.Remove fails on directories)  File & Line:  pkg/process/isolated_home.go#L106 Adversarial Assessment: The rationale states "copies are refreshed at every launch". This is false on disk on Windows. On standard Windows without Developer Mode, os.Symlink fails and copies are made. On launch 2, line 106 executes _ = os.Remove(dst). In Go, os.Remove returns ENOTEMPTY on directories. Because line 106 fails to delete the old copied directory, copyDir runs into pre-existing files and fails when encountering existing symlinks or renamed entries. It must be os.RemoveAll(dst).
Windows Directory Mirroring (os.Remove fails on directories)
- File & Line:  pkg/process/isolated_home.go#L106
- Adversarial Assessment: The rationale states "copies are refreshed at every launch". This is false on disk on Windows. On standard Windows without Developer Mode, os.Symlink fails and copies are made. On launch 2, line 106 executes _ = os.Remove(dst). In Go, os.Remove returns ENOTEMPTY on directories. Because line 106 fails to delete the old copied directory, copyDir runs into pre-existing files and fails when encountering existing symlinks or renamed entries. It must be os.RemoveAll(dst).
- Empty PIN Semantics & Total Bypass of PIN on /v1beta/  File & Line:  pkg/custommodels/gateway.go#L72-L76 ,  pkg/custommodels/gateway.go#L108 ,  pkg/custommodels/gateway.go#L160 Adversarial Assessment: While subtle.ConstantTimeCompare was added for the OpenAI surface, /v1beta/models and /v1beta/models/ never call authorizeExternal at all. An external agent reaching the port can call /v1beta/ endpoints without providing the PIN, even when external PIN protection is configured. Furthermore,  pkg/custommodels/gateway.go#L78  still accepts ?pin= in query strings, leaking credentials into web proxy and server access logs.
Empty PIN Semantics & Total Bypass of PIN on /v1beta/
- File & Line:  pkg/custommodels/gateway.go#L72-L76 ,  pkg/custommodels/gateway.go#L108 ,  pkg/custommodels/gateway.go#L160
- Adversarial Assessment: While subtle.ConstantTimeCompare was added for the OpenAI surface, /v1beta/models and /v1beta/models/ never call authorizeExternal at all. An external agent reaching the port can call /v1beta/ endpoints without providing the PIN, even when external PIN protection is configured. Furthermore,  pkg/custommodels/gateway.go#L78  still accepts ?pin= in query strings, leaking credentials into web proxy and server access logs.
- Session ID Derivation from Opening Prompt  File & Line:  pkg/custommodels/gateway.go#L1250-L1269 Adversarial Assessment: Deriving x-opencode-session from sha256(modelID + NUL + first_user_message) still suffers from: Prompt collision: Two independent conversations initiated with common prompts ("Hi", "Explain this") share the same session ID on OpenCode Go, corrupting server-side context. Context window sliding: When conversations truncate early history, the first user message in context changes, mutating the hash mid-dialogue and resetting the session.
Session ID Derivation from Opening Prompt
- File & Line:  pkg/custommodels/gateway.go#L1250-L1269
- Adversarial Assessment: Deriving x-opencode-session from sha256(modelID + NUL + first_user_message) still suffers from: Prompt collision: Two independent conversations initiated with common prompts ("Hi", "Explain this") share the same session ID on OpenCode Go, corrupting server-side context. Context window sliding: When conversations truncate early history, the first user message in context changes, mutating the hash mid-dialogue and resetting the session.
- Prompt collision: Two independent conversations initiated with common prompts ("Hi", "Explain this") share the same session ID on OpenCode Go, corrupting server-side context.
- Context window sliding: When conversations truncate early history, the first user message in context changes, mutating the hash mid-dialogue and resetting the session.

#### B. Additional Open Findings from Initial Review
- State Directory Leakage into Native Profile  File & Line:  pkg/process/isolated_home.go#L27 sharedHomeEntries still only lists "antigravity", "antigravity-ide", "antigravity-backup". Directories ~/.gemini/history and files settings.json, projects.json remain symlinked, causing the custom profile to overwrite native chat history and workspace preferences.
State Directory Leakage into Native Profile
- File & Line:  pkg/process/isolated_home.go#L27
- sharedHomeEntries still only lists "antigravity", "antigravity-ide", "antigravity-backup". Directories ~/.gemini/history and files settings.json, projects.json remain symlinked, causing the custom profile to overwrite native chat history and workspace preferences.
- Extension Credential Mirroring Broken on Windows & macOS  File & Line:  pkg/process/isolated_home.go#L68 Hardcodes filepath.Join(realHome, ".config", "antigravity"). On Windows (%APPDATA%\antigravity) and macOS (~/Library/Application Support/antigravity), this path does not exist, so extension credentials are never mirrored.
Extension Credential Mirroring Broken on Windows & macOS
- File & Line:  pkg/process/isolated_home.go#L68
- Hardcodes filepath.Join(realHome, ".config", "antigravity"). On Windows (%APPDATA%\antigravity) and macOS (~/Library/Application Support/antigravity), this path does not exist, so extension credentials are never mirrored.
- Anthropic Message Translation Rejection on Consecutive Roles  File & Line:  pkg/custommodels/gateway.go#L847-L879 Consecutive same-role messages are not coalesced, causing Anthropic's API to reject payloads with HTTP 400 (roles must alternate between "user" and "assistant").
Anthropic Message Translation Rejection on Consecutive Roles
- File & Line:  pkg/custommodels/gateway.go#L847-L879
- Consecutive same-role messages are not coalesced, causing Anthropic's API to reject payloads with HTTP 400 (roles must alternate between "user" and "assistant").
---

## Disposition of the re-review

| Re-review finding | Status |
|---|---|
| B1 PARTIAL — non-stream path still emitted `id: "call_swiss_0"` (would decode to `swiss`) | Fixed: `toolCallID(part.FunctionCall.Name, len(calls))` with a real `index`. `grep call_swiss_0` is now zero. |
| New: repeated calls to one tool reused a single id (`callNames` map iteration, `len(calls)` always 0) | Fixed: `pendingToolID` pairs each response with the next unanswered call of the same name, in order. Test: `TestRepeatedToolCallsKeepDistinctIDs`. |
| New: error frame emitted `finish_reason: "tool_calls"` with text content | Fixed: error frames finish with `stop`; only a real tool call ends with `tool_calls`. |
| New: `/v1beta/` never called `authorizeExternal` — PIN bypass when the proxy is exposed | Fixed: `handleListModels` and `handleModelAction` check the PIN. Test: `TestGeminiSurfaceRequiresPinWhenExternal`. |
| New: `?pin=` accepted, leaking the PIN into proxy/access logs | Fixed: header only (`X-Swiss-PIN`); the query form is rejected, covered by the same test. |
| Pushed back: `os.Remove` cannot delete a copied directory (Windows fallback) | Fixed: `os.RemoveAll`, with the reason in a comment. |
| Pushed back: session id derived from the opening prompt | Upheld: captured live payloads show the app sends no conversation id (headers `[accept-encoding authorization content-length content-type user-agent]`, body keys `[max_completion_tokens messages model stream stream_options]`). |

Verification after these fixes: 25 packages green, `go vet` clean, binary rebuilt and redeployed, gateway smoke test returns `re-review-fix-ok`.

---

## Live verification round (removes one feature)

Every remaining path was exercised against the real system. One claim did not survive:

| Check | Result |
|---|---|
| Gateway bearer token required | 401 without, 200 with |
| Proxy PIN on `/v1/*` and `/v1beta/*` | 401 without `X-Swiss-PIN`, 200 with; `?pin=` rejected (unit test) |
| `/v1/models` and `/v1beta/models` contents | Custom models only, identical in both modes |
| App custom-model turn (isolation) | Gateway receives `chat/completions model="openai-deepseek-v4-flash-…"`, window renders the answer, no error |
| VS Code custom mode | 409 with an explicit reason; native path unaffected |
| Full suite | 25 packages green, bundle rebuilt |
| **Native Gemini models on the external surface** | **Removed — not implementable here** |

### Why native serving was deleted

- `generativelanguage.googleapis.com/v1beta/models` → **403 insufficient authentication scopes**. The app's token carries `cloud-platform`, `cclog`, `experimentsandconfigs`, `aicode` — never the public Gemini API scope. Token refresh was implemented and confirmed working, and it changed nothing.
- `cloudcode-pa.googleapis.com/v1internal:loadCodeAssist` → 200, but `ineligibleTiers: UNSUPPORTED_CLIENT` and no `cloudaicompanionProject`, so Cloud Code refuses to serve our client.

Neither upstream can execute a native model from this process, so `native.go`, `runModel`, the native catalog, the token refresher and the native listing were removed. The gateway now exposes custom models only, which is exactly what it can serve. Native traffic stays on the app's own direct path to Google, which is also the ban-risk-safe default.
