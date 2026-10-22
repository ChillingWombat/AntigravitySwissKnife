# Review brief — custom model integration (Model Source)

Review the implementation below. You have file access; read the code, don't trust this summary.
Be adversarial: name concrete defects, wrong assumptions, missing tests, and any place where the
code claims more than it does. Rank findings by severity. Do not restate what works.

## What the feature is meant to do

Let a user run real bring-your-own-provider models (DeepSeek via OpenCode Go, DeepSeek direct,
Gemini via GCP) inside the Antigravity surfaces, in two explicit modes, without disturbing normal
Gemini usage:

1. **Profile isolation** (default) — each app runs on its own state with one catalog. Native
   windows stay untouched; a custom-model window runs separately.
2. **API proxy** (opt-in, warned as ban risk) — one catalog serves both model families.

Hard rules from the product owner: never override or hijack a Gemini model slot; never relay
native Gemini traffic through the daemon in the default mode; never present a switch that has no
inference path behind it.

## Implemented surface

Settings and model store:
- `pkg/custommodels/models.go` — `Config.Integration` (`IntegrationSettings{Mode, Apps, Proxy}`),
  constants, `DefaultIntegrationSettings()`. `app_model_overrides` was retired and removed.
- `pkg/custommodels/store.go` — `GatewayToken()`, `IntegrationSettings()` with per-field defaults,
  `SetIntegrationSettings()` validation, `ListModels()`.
- `pkg/webgui/server.go` — `/api/custom_models/{active,integration,launch,app_proxy}`,
  `loopbackAddr`, `gatewayModelIDs`.

Model gateway (two wire surfaces, one store):
- `pkg/custommodels/gateway.go` — Gemini surface (`/v1beta/models`, `:streamGenerateContent`) and
  OpenAI surface (`/v1/models`, `/v1/chat/completions`); `authorize` (bearer) and
  `authorizeExternal` (PIN when the integration scope is external); provider translation for
  OpenAI-compatible, Anthropic and Gemini-key; tool calls both directions; `applyProviderHeaders`
  and `conversationSeed` for OpenCode Go's `x-opencode-session` requirement.
- `pkg/custommodels/native.go` — native Gemini models on the OpenAI surface: OAuth token from
  `~/.gemini/oauth_creds.json`, 5-minute catalog cache, `isNativeModel`, `runNativeGemini`
  (`:streamGenerateContent?alt=sse`), `runModel` dispatch between custom and native.
- `pkg/custommodels/tester.go` — `ResolveEndpoint`, including an OpenCode rule hoisted above the
  provider switch that maps every legacy shape to
  `https://opencode.ai/inference/go/openai/v1/chat/completions`.

Cloud Code proxy (fallback seam):
- `pkg/custommodels/proxy.go` — verbatim pass-through, custom model translation with the wrapped
  `{"response":…,"traceId":…,"metadata":{}}` envelope the language server requires, dispatch ahead
  of the mux because ServeMux cannot match `/v1internal:method`.

Surfaces / launchers:
- `pkg/process/surfaces.go` — `LaunchAntigravityProfile(source, gatewayURL, gatewayKey,
  gatewayModels, shield)`: native launches plainly; custom prepares an isolated HOME, adds
  `--user-data-dir`, and sets `AGY_LLM_GATEWAY_{URL,MODELS,API_KEY,WIRE_PROTOCOL}` plus
  `CLOUD_CODE_URL`. `ApplyVSCodeModelSource` writes `antigravity.serverArgs`; `LaunchVSCode`.
- `pkg/process/isolated_home.go` — `IsolatedHomeDir`, `PrepareIsolatedHome` (links every `~/.gemini`
  entry except the state dirs, keeps state private), `linkOrCopy`, `copyDir`, `homeEnvOverride`.

UI:
- `frontend/src/components/ModelSourceTab.tsx` — mode cards, per-app Native/Custom rows with launch
  buttons, conditional subagents row, proxy scope + PIN + risk alerts, both protocol blocks.
- `frontend/src/pages/CustomModelsPage.tsx` — `pageView` switch between the model list and Model
  Source; `frontend/src/api.ts` — the three new calls.

Debug seam:
- `SWISS_SURFACE_TRACE=1` logs non-GUI request paths and, for `/v1/chat/completions`, header names,
  top-level body keys and history shape (never values).

## Decisions to challenge specifically

1. **Isolation is achieved by scoping `HOME`/`USERPROFILE`**, because `dist/paths.js` derives state
   from `os.homedir()/.gemini/<app name>`. The asar-patch approach was built and then deleted.
   Is the env approach sound on Windows in particular, and what breaks if the app later switches to
   `app.getPath('userData')` for that path?
2. **The app executes turns through `/v1/chat/completions`**, not through `CLOUD_CODE_URL`; the
   surface was previously unmounted, which made every custom-model turn fail with a session 401.
   Is mounting `/v1/` and exempting it from GUI session auth (gateway enforces its own bearer) the
   right trust boundary?
3. **`x-opencode-session` is derived** from `sha256(modelID + NUL + first user message)[:32]` because
   the app sends no conversation id. Point out where this breaks.
4. **The `GET /v1beta/models` surface switches to native-only when `Mode == api_proxy`**, while the
   OpenAI surface always lists both families. Is that asymmetry defensible or confusing?
5. **`authorizeExternal` only checks the PIN when scope is `external`**, and treats an empty PIN as
   "allowed". Is that safe?
6. **Native Gemini calls bypass the daemon entirely** in the default path (only the proxy mode
   relays) — verify no code path accidentally relays OAuth traffic.
7. **`PrepareIsolatedHome` links credentials** (`~/.gemini/oauth_creds.json`,
   `~/.config/antigravity`) into the isolated home and warns instead of failing when a single entry
   cannot be mirrored. What can silently break there?

## Known gaps you should verify rather than assume

- The external proxy surface (PIN + Gemini-model conversion through `/v1/chat/completions`) has
  never been exercised against Google with real credentials.
- VS Code custom-model mode is written but untested end to end.
- `pkg/custommodels` has tests for the gateway, native dispatch, per-protocol listing, provider
  headers and endpoint resolution; coverage of `proxy.go` and `isolated_home.go` is thinner.

## How to report

Group findings as Blocker / Should fix / Consider. For each: file and line, what is wrong, why it
matters, and the smallest correct change. If a decision above is right, say so briefly and move on.
