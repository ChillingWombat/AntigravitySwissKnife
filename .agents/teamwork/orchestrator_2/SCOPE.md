# Scope: Antigravity Swiss Knife Extensions Delivery (R1–R8)

## Architecture

The Antigravity Swiss Knife Extensions provide production-ready integrations for Google Antigravity 2.0 (Electron desktop app at `/opt/Antigravity/antigravity`):
1. **Auxiliary Panel Tab Injector Engine**: Dynamically mounts custom tabs (Browser, Files, Memos) into Antigravity's right navbar `.shrink-0.flex.items-center.gap-0.5.border-b` with two-way state synchronization with factory tabs (overview, review, terminal) and view container `#swiss-aux-container` inside `.flex-grow.overflow-hidden`.
2. **Live Browser Preview & Visual Canvas Annotation**: Embedded `<webview>` loading local development servers (e.g. `localhost:5173`, `localhost:3000`) or remote URLs with toolbar, Bézier-smoothed red pen (#ea4335, 3px), bounding box drag tool, DOM element selector, "Send to Chat" file attachment into composer `input[type="file"]` & Lexical editor injection, and mobile device frames (iPhone 16 Pro, Pixel 9, iPad) with touch emulation.
3. **Auxiliary File Explorer & Mutation Endpoints**: Pure Go backend filesystem endpoints in `pkg/webgui/server.go` (`/api/files/write`, `/api/files/rename`, `/api/files/delete`, `/api/files/reveal`, `/api/files/terminal`), dynamic path fallback, breadcrumbs, search, tree navigation, context menus, syntax-highlighted code editor, and Markdown WYSIWYG/preview toggle.
4. **Real 6-Probe Custom Models API Relay Security Auditor**: Active HTTP test probes replacing cosmetic stubs: Transport Security (TLS port fix), Origin Lineage, Active Model Canary (reasoning benchmark prompt), Prompt Echo & System Integrity, Tool Call Schema Preservation, and Error & Credential Leakage (active invalid param test). Visual MD3 risk meter (0–100, A+ to F).
5. **Real SQLite Cross-Agent Chat & Project Importer**: Pure Go SQLite (`modernc.org/sqlite`, `CGO_ENABLED=0`, zero Python) parser for Claude Code (`~/.claude/transcripts/*.jsonl`), ChatGPT JSON export, and raw JSON transcripts. Writes directly into `~/.gemini/antigravity/conversation_summaries.db`, `conversations/<id>.db`, and `transcript.jsonl`. Auto-matches workspace directories to `app_storage.json` (`projectsOrder`).
6. **Real In-Chat Token & TPS Telemetry**: Real log parsing of `transcript.jsonl`, child subagent token aggregation via `conversation_summaries.db`, and Google Material telemetry badge injection below assistant turns in chat DOM.
7. **Quick Memos with Real Audio Recording**: WebM/Opus audio recording via `MediaRecorder`, Base64 encoding into `~/.config/antigravity-swiss/memos.json`, waveform canvas scrubbing, and drag-and-drop file attachment into chat composer.
8. **Pipeline Integration, Build & Tests**: Unified `GenerateScript(cfg)` in `pkg/gui/styler.go` bundling all generators into `persistent_script.js`. 100% green tests (`go test ./...`), clean frontend build (`npm run build`), CLI sync (`swiss patch sync`), and complete documentation.

---

## Feature Inventory

| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F27 | `F27_AUX_TAB_INJECTOR` | Injects Browser, Files, Memos tabs into `.shrink-0.flex.items-center.gap-0.5.border-b` (`data-tab-id="swiss-*"`) and `#swiss-aux-container` inside `.flex-grow.overflow-hidden` with two-way state sync. | Ext-M1 | ORIGINAL_REQUEST §R1 |
| F28 | `F28_BROWSER_PREVIEW_CANVAS` | Embedded `<webview>` loading local dev servers/remote URLs with toolbar, Bézier red pen (#ea4335, 3px), bounding box, DOM element selector, "Send to Chat" file & Lexical injection, and mobile device frames. | Ext-M1 | ORIGINAL_REQUEST §R2 |
| F29 | `F29_FILE_MUTATION_ENDPOINTS_EXPLORER` | Go filesystem mutation endpoints (`/api/files/write`, `rename`, `delete`, `reveal`, `terminal`), breadcrumbs, search, tree, syntax-highlighted code editor, and Markdown preview. | Ext-M2 | ORIGINAL_REQUEST §R3 |
| F30 | `F30_SECURITY_AUDITOR_6_PROBES` | Active 6-probe API relay security auditor (TLS port fix, Origin Lineage, Canary, Echo, Tool Call Schema, Credential Leakage) and MD3 risk meter (0–100, A+ to F). | Ext-M3 | ORIGINAL_REQUEST §R4 |
| F31 | `F31_SQLITE_CHAT_IMPORTER` | Pure Go SQLite importer for Claude Code, ChatGPT, and raw JSON, writing directly into `conversation_summaries.db` and `conversations/<id>.db` with `app_storage.json` project matching. | Ext-M4 | ORIGINAL_REQUEST §R5 |
| F32 | `F32_IN_CHAT_TOKEN_TELEMETRY` | Exact token metrics and TPS from `transcript.jsonl`, child subagent token aggregation via SQLite, and Material badge injection below assistant turns in chat DOM. | Ext-M5 | ORIGINAL_REQUEST §R6 |
| F33 | `F33_QUICK_MEMOS_AUDIO` | Text and WebM/Opus audio memos stored in local config, waveform canvas scrubbing, and drag-and-drop into chat composer. | Ext-M5 | ORIGINAL_REQUEST §R7 |
| F34 | `F34_PIPELINE_SYNC_BUILD_TESTS` | Unified `GenerateScript(cfg)` in `styler.go`, `swiss patch sync`, 100% test coverage (`go test ./...`), clean frontend build (`npm run build`), and `README.md` documentation. | Ext-M6 | ORIGINAL_REQUEST §R8 |

---

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| Ext-M1 | Auxiliary Panel Tab Injector Engine & Browser Preview with Annotation Canvas | Features F27, F28 (R1, R2). In `pkg/plugins/auxiliary.go` and `frontend/`: exact DOM injection, two-way factory tab sync, `<webview>` embedding, Bézier red drawing canvas, DOM element selector, "Send to Chat" Lexical & File injection, responsive frames. | none | IN_PROGRESS |
| Ext-M2 | Auxiliary File Explorer with Real Mutation Endpoints & Editors | Feature F29 (R3). In `pkg/webgui/server.go`, `pkg/plugins/auxiliary.go`, `frontend/`: `/api/files/write`, `/api/files/rename`, `/api/files/delete`, `/api/files/reveal`, `/api/files/terminal`, syntax-highlighted code editor, Markdown preview, 100% tests in `server_test.go`. | Ext-M1 | PLANNED |
| Ext-M3 | Real 6-Probe Custom Models API Relay Security Auditor | Feature F30 (R4). In `pkg/custommodels/` and `frontend/src/utils/securityAudit.ts`, `SecurityReportModal.tsx`: TLS port dialer fix, multi-provider payloads, active invalid param test for Probe 6, removal of cosmetic stubs, MD3 risk meter gauge (0–100, A+ to F), `CustomModel` persistence fields, unit tests in `auditor_test.go`. | none | PLANNED |
| Ext-M4 | Native Pure Go SQLite Cross-Agent Chat & Project Importer | Feature F31 (R5). In `pkg/importer/`: pure Go SQLite (`modernc.org/sqlite`, `CGO_ENABLED=0`), parsers for Claude Code (`~/.claude/transcripts/*.jsonl`), ChatGPT JSON, raw JSON, direct write to `conversation_summaries.db` & `conversations/<id>.db`, auto-matching `app_storage.json` `projectsOrder`, zero Python subprocesses, unit tests in `importer_test.go`. | none | PLANNED |
| Ext-M5 | In-Chat Token & TPS Telemetry & Quick Memos with Real Audio | Features F32, F33 (R6, R7). Real `transcript.jsonl` log parsing in Go, subagent token aggregation via SQLite, Google Material badges below assistant turns, WebM/Opus audio recording Base64 encoding, waveform scrubbing, drag-and-drop into chat composer. | Ext-M1, Ext-M4 | PLANNED |
| Ext-M6 | Full Pipeline Integration, Styler Script Generator, Build & Tests | Feature F34 (R8). Unify `GenerateScript(cfg)` in `styler.go` to bundle all generators into `persistent_script.js`, fix `pkg/plugins/auxiliary_test.go`, add CLI tests in `cmd/swiss/main_test.go`, update `README.md`, ensure 100% test coverage (`go test ./...`) and clean frontend build. | Ext-M1, Ext-M2, Ext-M3, Ext-M4, Ext-M5 | PLANNED |

---

## Interface Contracts

### 1. Tab Injector ↔ Host Antigravity Navbar
- **Host Tab Bar**: `.shrink-0.flex.items-center.gap-0.5.border-b`
- **Injected Tab Attributes**: `data-tab-id="swiss-browser"`, `data-tab-id="swiss-files"`, `data-tab-id="swiss-memos"`
- **Host View Container**: `.flex-grow.overflow-hidden`
- **Injected Container**: `#swiss-aux-container` (nested inside `.flex-grow.overflow-hidden`)
- **Factory Tabs**: `overview`, `review`, `terminal` (hidden when Swiss tab is active; restored when factory tab is clicked)

### 2. Live Browser Preview ↔ Chat Composer
- **Canvas Screenshot Attachment**: Synthesized `File` blob dispatched via `DataTransfer` to `document.querySelector('input[type="file"]')`
- **Lexical Editor Injection**: Updates `document.querySelector('.lexical-editor').__lexicalEditor` via Lexical state transition or resilient fallback to composer textarea

### 3. Filesystem Endpoints (`pkg/webgui/server.go`)
- `POST /api/files/write`: `{"path": string, "content": string}` -> `{"success": true, "bytes": int}`
- `POST /api/files/rename`: `{"old_path": string, "new_path": string}` -> `{"success": true}`
- `POST /api/files/delete`: `{"path": string, "recursive": bool}` -> `{"success": true}`
- `POST /api/files/reveal`: `{"path": string}` -> `{"success": true}` (xdg-open on Linux, open on macOS, explorer on Windows)
- `POST /api/files/terminal`: `{"path": string}` -> `{"success": true}` (spawn terminal emulator in directory)

### 4. 6-Probe Security Auditor (`pkg/custommodels/auditor.go`)
- Probes:
  1. `probeTransportSecurity`: TLS certificate validation on port 443
  2. `probeOriginLineage`: Proxy & intermediary detection headers
  3. `probeActiveModelCanary`: Reasoning canary verifying genuine model response
  4. `probePromptEchoIntegrity`: System prompt echo canary
  5. `probeToolCallSchemaPreservation`: Nested tool call parameter preservation
  6. `probeCredentialLeakage`: Invalid parameter injection testing for stack traces & key leaks
- Metric: Score 0–100, Letter Grade A+ to F, Material Design 3 risk meter gauge

### 5. Pure Go SQLite Importer (`pkg/importer/`)
- Driver: `modernc.org/sqlite`
- Targets:
  - `~/.gemini/antigravity/conversation_summaries.db`: `conversation_summaries`
  - `~/.gemini/antigravity/conversations/<id>.db`: `trajectory_meta`, `steps`
  - `~/.gemini/antigravity/brain/<id>/.system_generated/logs/transcript.jsonl`
- Project Matching: Matches workspace directory to `app_storage.json` `projectsOrder` and existing project summaries.

### 6. Pipeline Bundling (`pkg/gui/styler.go`)
- `GenerateScript(cfg)` must return:
  `baseScript + ";\n\n" + customScript + ";\n\n" + enhScript + ";\n\n" + pluginsScript + ";"`
