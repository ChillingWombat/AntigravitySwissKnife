---
date: 2026-10-22T18:58:59+1100
author: David
commit: b5ed9e0
branch: main
repository: Antigravity Swiss Knife
topic: Custom Model Integration Across Antigravity Surfaces Feature Implementation
tags: [custom-models, model-gateway, profile-isolation, api-proxy, gemini-bridge, agy-cli, antigravity-2.0, vscode-extension]
status: complete
last_updated: 2026-10-22T18:58:59+1100
last_updated_by: David
type: feature_development
---

# Handoff: real custom models on the CLI + Model Source settings/launchers

## Task(s)

Goal: custom (BYO-provider) models must genuinely work — not labelled — on the Antigravity CLI, the Antigravity 2.0 app, and the VS Code extension, and Gemini models must keep working natively without interference or ban risk.

Status by surface:

| Surface | Status |
|---|---|
| Antigravity CLI | **Done, verified live.** `agy` turn answered by DeepSeek through the daemon gateway (`deepseek-was-here`, 4.6 s). Native turns keep zero gateway traffic. |
| App + VS Code — settings backend | **Done.** Integration settings persisted, validated, served. |
| App + VS Code — launchers | **Done.** Isolation-gated Antigravity launch; VS Code `serverArgs` writer. |
| App + VS Code — Model Source tab UI | **Done.** `ModelSourceTab.tsx`, wired into `CustomModelsPage` (`pageView`), bundle rebuilt, `tsc` clean. |
| Proxy mode protocol layer | **In progress** — PIN gate landed; Gemini-model conversion pending. |

Ground rules David set (do not violate):
- Never expose both model types in one catalog inside a single app instance (structural: the gateway seam *replaces* the catalog).
- Never relay native Gemini traffic through the daemon in the default mode (ban risk). Native model calls must go straight to Google.
- No model-slot overriding (retired: `app_model_overrides` is deleted from config, store, proxy and tests).
- Emojis are banned; Lucide icons only; follow David-Design tokens (see `AGENTS.md`).

## Decisions and evidence

1. **CLI custom models = native BYOM seam.** `agy` honours `AGY_LLM_GATEWAY_URL`, `_API_KEY`, `_MODELS` (comma ids), `_WIRE_PROTOCOL` (`openai` or default gemini/vertex). Discovery `GET {url}/v1/models` (openai) or `/v1beta/models?pageSize=1000` (vertex). Inference `POST /v1/chat/completions` or `/v1beta/models/{id}:streamGenerateContent?alt=sse`. **SSE streaming is mandatory**; non-streamed replies are consumed but yield empty output/errors.
2. **The language server honours the same env vars** (Tier 3 gateway discovery), verified on the real `language_server` binary — that is how the app gets first-class custom entries. No state file, no RPC (`JetboxWriteState` is WebSocket-only; HTTP 404s), no `main.js` patch.
3. **`CLOUD_CODE_URL` also works** for both surfaces (overrides the hardcoded `--cloud_code_endpoint`), verified with a stub (102 requests) and on the real process. Kept as fallback seam.
4. **Injecting ids into `fetchAvailableModels` does not work** — the renderer curates its id set client-side; unknown ids are discarded. Hence catalog-level seams only.
5. **Profile isolation verdict:** VS Code needs **no patch** — `antigravity.serverArgs` is appended last (Go flag last-wins) so `["--app_data_dir=antigravity-swiss-custom","--cloud_code_endpoint=<gateway>"]` isolates and repoints; workspace settings override user settings and arrays are replaced wholesale. Antigravity 2.0 **does** need a one-expression `dist/paths.js` patch: `getAppDataDirName()` uses `app.getName()`, so every window writes `~/.gemini/antigravity` (state, `conversation_summaries.db` + wal, trajectory reindex — confirmed corruption risk). Patch: `return path.basename(app.getPath('userData'))`. Auth is global (`~/.config/antigravity/plugin-secrets.json`), so isolated windows stay signed in.

## What is in the tree now

- `pkg/custommodels/gateway.go` — daemon model gateway: Gemini surface (`/v1beta/models`, `:generateContent`, `:streamGenerateContent`) **and** OpenAI surface (`/v1/models`, `/v1/chat/completions`); provider translation for OpenAI-compatible, Anthropic and Gemini-API; **tool calling both directions**; bearer auth via `Store.GatewayToken()`.
- `pkg/custommodels/proxy.go` — Cloud Code proxy (fallback seam): verbatim passthrough, custom-model translation with the **wrapped** envelope `{"response":{…},"traceId":…,"metadata":{}}` the LS requires (bare frames are silently rejected → retry storm), no interception of non-generation endpoints. Dispatched ahead of the mux by `cloudCodeDispatch` because ServeMux cannot match `/v1internal:method`. Auth-exempt paths in `pkg/webgui/auth_middleware.go`: `/v1beta*`, `/v1internal*`.
- `pkg/custommodels/models.go` — `Config.Integration` (`IntegrationSettings{Mode, Apps, Proxy}`), constants, `DefaultIntegrationSettings()` = profile isolation + native + subagents in own app. `gateway_token`, `app_model_overrides` **removed**.
- `pkg/custommodels/store.go` — `GatewayToken()`, `RotateGatewayToken()`, `IntegrationSettings()` (per-field defaults), `SetIntegrationSettings()` (validates mode/source/subagents/scope), `SetActiveModel()`, `GetConfig` echoes `Integration`.
- `pkg/bridge/cli.go` — CLI backend: `runTurn`, per-conversation serialisation, JSON parsing (whole-payload first, then last-line), model resolution **inside** `runTurn`, gateway env injected only for custom models, canonical gateway id rewrite, absolute-binary + flag-like-arg validation (`validateFlagValue`, `resolveBinary`).
- `pkg/bridge/backend.go` — `agentBackend` interface, `appBackend`/`cliBackend`, `resolveAgentBackend(requested, cliReady)`, `cliGatewayEnv(model)` → `(env, canonicalID, ok)`, `SetGatewayTokenSource`.
- `pkg/bridge/model_surface.go` — unified model list (`kind: native|custom`), CLI custom selection, gateway token helper.
- `pkg/process/surfaces.go` — `IsolatedProfileDir`, `AppDataDirIsolationApplied()`, `LaunchAntigravityProfile(source, gatewayURL, key, shield)` (refuses custom launch until asar isolation is applied), `VSCodeSettingsPath()`, `ApplyVSCodeModelSource(source, gatewayURL)`, `LaunchVSCode()`.
- `pkg/webgui/server.go` — `/api/custom_models/active`, `/integration` (GET/POST), `/launch` (POST `{app}`), `/app_proxy` (POST `{action}`), `cloudCodeDispatch`, `loopbackAddr`; `/api/custom_models/bind` and `/app_override` deleted.
- Tests: `pkg/custommodels/gateway_test.go` (7), `proxy_test.go` (6 → 4 after override removal), `pkg/bridge/backend_test.go` (agent resolution, CLI parsing/model/flags/sequential turns/list timestamps, unified surface, gateway env). Suites green at last run: 1756 Go tests / 26 packages, 737 frontend.

## Remaining work, in order

1. **Model Source tab UI** in the Custom Models section (`frontend/src/pages/CustomModelsPage.tsx`, new tab beside the model list; helpers in `frontend/src/api.ts`).
   - Mode cards: **Profile Isolation** (default) / **API Proxy** — orange `AlertTriangle` + "Potential Ban Risk" beside the API Proxy header.
   - Profile Isolation: two rows `Antigravity 2.0` and `VS Code`, each `Native | Custom` segmented control + launch button (`POST /api/custom_models/launch {app}`); when Custom, an extra row `Subagents: (•) Within its own app ( ) Antigravity CLI`, default own app.
   - API Proxy: scope segmented `Antigravity apps only` (default) / `Also expose to external agents`; external reveals endpoint (`GET /integration` → `endpoint`), optional PIN (masked), and a red `AlertCircle` + "Increased Ban Risk"; protocol blocks listing Gemini (`/v1beta/models`, Gemini models only, no conversion) and OpenAI (`/v1/models`, `/v1/chat/completions`, Gemini calls converted to the Gemini envelope).
   - Persist with `POST /api/custom_models/integration`.
2. **Proxy protocol layer** (`pkg/custommodels/proxy.go` or a sibling): when scope is external, require PIN on the external surface; Gemini-format listing exposes Gemini models only; OpenAI-format accepts Gemini model ids and converts the request into the Gemini/Cloud Code envelope (`{project, requestId, request:{contents,systemInstruction,generationConfig,sessionId}, model, userAgent, requestType:"agent"}`) forwarded with the on-disk OAuth token (`~/.gemini/oauth_creds.json`), returning OpenAI SSE. Add tests for conversion, PIN gating, and per-protocol model filtering.
3. **Isolation patch helper** (`pkg/process`): idempotent `paths.js` patch (backup, repack via asar), `AppDataDirIsolationApplied()` already detects the unpatched state; re-check after Antigravity updates. Only then can the Antigravity custom launch pass its gate.
4. **Verification**: native window untouched (no gateway log lines), custom window answering from DeepSeek, VS Code custom window listing custom models, external proxy call with/without PIN.

## Verification recipes (used in this session)

- Gateway discovery: `curl -s "http://127.0.0.1:8765/v1beta/models?pageSize=1000" -H "Authorization: Bearer $(python3 -c "import json,os;print(json.load(open(os.path.expanduser('~/.config/antigravity-swiss/custom_models.json')))['gateway_token'])")"`.
- Real CLI turn through the bridge: `POST /api/bridge/mcp` → `tools/call antigravity_lead_create_conversation {"prompt":"Reply with exactly: deepseek-was-here","model":"DeepSeek Flash"}` with the bridge auth token from `~/.config/antigravity-swiss/gemini_bridge.json`.
- Traffic path proof: `grep -i "llm-gateway" <daemon log>`; native turns must show zero new lines.
- Bridge tool list: `tools/list` → 30 tools, including `antigravity_bridge_set_agent` and `antigravity_session_model_*`.

## Gotchas

- Running the packaged app: kill the old Electron instance first (`pkill -f antigravity-swiss-knife.bin`) or it exits with "Another instance is already running"; copy `bin/swiss` into `~/.local/share/antigravity-swiss-knife/app/resources/bin/swiss`; launch with `setsid` for a standalone daemon.
- Two windows: never launch a custom Antigravity window before the asar isolation patch; the launcher refuses, do not bypass it.
- `agy` rejects display labels when a gateway is active — always pass the gateway model **id** (the bridge rewrites labels to ids).
- The LS's SSE envelope for the app path is wrapped; the CLI's is bare. Mixing them hangs the app.

## Memories written this session (mem0, project "Antigravity Swiss Knife")

- CLI BYOM contract (`AGY_LLM_GATEWAY_*`, SSE requirements).
- `CLOUD_CODE_URL` seam details (plain HTTP, envelope, Bearer, streaming, respawn).
- App BYOM dormancy (`supportsCustomModels` gate, `custom_models_config` schema, pinned Gemini scheme).
- Bridge backend abstraction + unified model surface + project-binding removal.
- Gateway implementation (CLI real custom models).
- Agreed integration design (Profile Isolation vs API Proxy, alerts, defaults).
- Backend settings step, launcher step, profile-isolation verdict.
