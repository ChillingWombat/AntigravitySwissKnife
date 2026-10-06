# Survey Report: SQLite Cross-Agent Chat Importer (R5) & Pipeline Integration, Build & Tests (R8)

**Agent**: `explorer_ext_survey_3` (teamwork_preview_explorer)  
**Date**: 2026-10-05  
**Authoritative Reference**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/ORIGINAL_REQUEST.md` (Timestamp: `2026-10-05T22:09:01Z`)  
**Target Scope**:  
- **R5**: Real SQLite Cross-Agent Chat & Project Importer  
- **R8**: Full Pipeline Integration, Build & Tests  

---

## 1. Executive Summary

This investigation provides a comprehensive architectural survey and concrete technical specification for implementing **Requirement R5** (Native SQLite Cross-Agent Chat & Project Importer) and **Requirement R8** (Full Pipeline Integration, Build & Tests) within Antigravity Swiss Knife.

### Core Discoveries:
1. **Antigravity Dual-Storage Architecture**: Antigravity 2.0 does not store conversation state in a single file. It uses a three-tier storage model:
   - Global catalog: `~/.gemini/antigravity/conversation_summaries.db` (`conversation_summaries` table, 21 columns).
   - Per-session database: `~/.gemini/antigravity/conversations/<conversation_id>.db` (`trajectory_meta` and `steps` tables).
   - Session transcript log: `~/.gemini/antigravity/brain/<conversation_id>/.system_generated/logs/transcript.jsonl`.
2. **Project Identity in Antigravity**: Antigravity groups conversations in its UI sidebar using a UUID string (`project_id`), tracked globally in `/home/david/.config/Antigravity/app_storage.json` under `projectsOrder`. The legacy prototype (`scripts/agent_importer.py`) mistakenly wrote directory basename strings (e.g. `ddeng.me`), which caused imported conversations to fail to associate with existing workspace projects in the Antigravity UI.
3. **Legacy Python Workaround Elimination**: The current codebase still spawns Python 3 subprocesses in three separate locations:
   - `pkg/webgui/server.go` (lines 1937, 1977): runs `python3 scripts/agent_importer.py ...` for scanning and importing.
   - `pkg/gui/archive.go` (line 663): runs `python3 -c "import sqlite3..."` for query and archiving.
   - `pkg/gui/store.go` (line 663): runs `python3 -c "import sqlite3..."` for project activity statistics.
   All of these can be completely replaced by a pure Go SQLite engine (`modernc.org/sqlite`), eliminating all external Python runtime dependencies to fulfill R1/R5.
4. **Styler Script Generator Fragmentation**: In `pkg/gui/styler.go`, `GenerateScript(cfg)` merges base scripts, custom models, and enhancements, but **omits** `plugins.GenerateAuxiliaryPluginsScript()`. That script was only appended in `GenerateScriptWithCustomModels()`. Consequently, standard `GenerateScript()` calls and sync operations drop the Browser Preview, File Explorer, Quick Memos, and in-chat Telemetry Badges.
5. **Test Suite Status**:
   - `npm run build` in `frontend/` builds cleanly in 505ms.
   - `go test ./pkg/plugins` currently fails due to two CSS class selector mismatches in `pkg/plugins/auxiliary_test.go` (`.swiss-aux-btn-group` vs `.swiss-browser-btn`, `.swiss-annotation-canvas` vs `.swiss-browser-canvas-overlay`).
   - `cmd/swiss/main_test.go` lacks test coverage for `swiss patch sync` and `swiss patch status`.
   - New Go test coverage is needed for the R5 chat importer parsers and SQLite writer.

---

## 2. Requirement R5: Real SQLite Cross-Agent Chat & Project Importer

### 2.1 Antigravity Native Storage Architecture

Direct inspection of the user's live Antigravity installation revealed the complete schema and storage contracts.

#### A. Master Catalog: `~/.gemini/antigravity/conversation_summaries.db`
Verified SQLite schema of `conversation_summaries`:
```sql
CREATE TABLE `conversation_summaries` (
    `conversation_id` text,
    `title` text NOT NULL DEFAULT "",
    `preview` text NOT NULL DEFAULT "",
    `step_count` integer NOT NULL DEFAULT 0,
    `last_modified_time` datetime NOT NULL,
    `workspace_uris` text NOT NULL,
    `status` text NOT NULL DEFAULT "",
    `source` text NOT NULL DEFAULT "",
    `project_id` text NOT NULL DEFAULT "",
    `agent_name` text NOT NULL DEFAULT "",
    `parent_conversation_id` text NOT NULL DEFAULT "",
    `nesting_depth` integer NOT NULL DEFAULT 0,
    `battle_id` text NOT NULL DEFAULT "",
    `winning_conversation_id` text NOT NULL DEFAULT "",
    `not_fully_idle` numeric NOT NULL DEFAULT false,
    `killed` numeric NOT NULL DEFAULT false,
    `last_user_input_time` datetime NOT NULL,
    `last_user_input_step_index` integer NOT NULL DEFAULT -1,
    `app_data_dir` text NOT NULL DEFAULT "",
    `raw_summary` blob,
    `group_id` text NOT NULL DEFAULT "",
    PRIMARY KEY (`conversation_id`)
);
```

**Key Column Contracts**:
- `conversation_id`: Canonical UUID string (e.g. `14c45a98-0c4d-4216-adc0-235ce48d5c5a`).
- `title`: Display title in the sidebar (e.g. `[Claude Code] Fix auth issue`).
- `preview`: Initial snippet of the first user message.
- `step_count`: Total number of conversation steps.
- `last_modified_time`: RFC3339 formatted UTC string (e.g. `2026-10-05T19:52:14.388264Z`).
- `workspace_uris`: JSON array of URL-encoded `file://` URIs (e.g. `["file:///mnt/Data/Projects/Antigravity%20Swiss%20Knife"]`).
- `status`: `"DONE"` (for imported sessions) or `"CASCADE_RUN_STATUS_RUNNING"`.
- `source`: Import source tag (e.g. `"CLAUDE_CODE_IMPORT"`, `"CHATGPT_IMPORT"`, `"GENERIC_IMPORT"`).
- `project_id`: The project UUID matching `projectsOrder` in `app_storage.json` (e.g. `"0a37d3b4-aa56-48d4-9762-e4f1c1098ec4"`). **If this matches an existing project, Antigravity places the conversation inside that project's section in the sidebar.**
- `agent_name`: Name displayed for the agent (e.g. `"Claude Code"`, `"ChatGPT"`, `"DeepSeek Harness"`).
- `last_user_input_time`: RFC3339 formatted UTC string.
- `last_user_input_step_index`: Index of the last user turn (defaults to 0 for initial prompt).

#### B. Per-Session Database: `~/.gemini/antigravity/conversations/<conversation_id>.db`
Verified SQLite schema:
```sql
CREATE TABLE `trajectory_meta` (
    `trajectory_id` text,
    `cascade_id` text,
    `trajectory_type` integer,
    `source` integer,
    PRIMARY KEY (`trajectory_id`)
);

CREATE TABLE `steps` (
    `idx` integer,
    `step_type` integer NOT NULL DEFAULT 0,
    `status` integer NOT NULL DEFAULT 0,
    `has_subtrajectory` numeric NOT NULL DEFAULT false,
    `metadata` blob,
    `error_details` blob,
    `permissions` blob,
    `task_details` blob,
    `render_info` blob,
    `step_payload` blob,
    `step_format` integer NOT NULL DEFAULT 0,
    PRIMARY KEY (`idx`)
);

CREATE TABLE `gen_metadata` (`idx` integer, `data` blob, `size` integer NOT NULL DEFAULT 0, PRIMARY KEY (`idx`));
CREATE TABLE `executor_metadata` (`idx` integer, `data` blob, PRIMARY KEY (`idx`));
CREATE TABLE `parent_references` (`idx` integer, `data` blob, PRIMARY KEY (`idx`));
CREATE TABLE `trajectory_metadata_blob` (`id` text DEFAULT "main", `data` blob, PRIMARY KEY (`id`));
CREATE TABLE `battle_mode_infos` (`idx` integer, `data` blob, PRIMARY KEY (`idx`));
```

**Key Data Contracts**:
- `trajectory_meta`:
  - `trajectory_id`: New UUIDv4.
  - `cascade_id`: Matches `conversation_id`.
  - `trajectory_type`: `4` (standard cascade trajectory).
  - `source`: `1`.
- `steps`:
  - `idx`: `0, 1, 2, ...`
  - `step_type`:
    - `14`: `USER_INPUT` (explicit user query)
    - `15`: `PLANNER_RESPONSE` (model assistant response)
    - `132`: `TOOL_CALL` / tool execution
  - `status`: `5` (COMPLETED / DONE)
  - `has_subtrajectory`: `0`
  - `step_payload`: UTF-8 JSON or serialized representation of the step content
  - `step_format`: `0`

#### C. Session Transcript Log: `~/.gemini/antigravity/brain/<conversation_id>/.system_generated/logs/transcript.jsonl`
Each line is a JSON object:
```json
{"step_index": 0, "source": "USER_EXPLICIT", "type": "USER_INPUT", "status": "DONE", "created_at": "2026-10-05T22:00:00Z", "content": "..."}
{"step_index": 1, "source": "MODEL", "type": "PLANNER_RESPONSE", "status": "DONE", "created_at": "2026-10-05T22:00:05Z", "content": "..."}
```

### 2.2 Workspace Auto-Matching with `app_storage.json`

Antigravity stores its desktop UI configuration in `/home/david/.config/Antigravity/app_storage.json`.
- `projectsOrder`: Stored as a JSON string containing an array of project UUIDs:
  `["0a37d3b4-aa56-48d4-9762-e4f1c1098ec4", "c3cf1430-c1ee-40c9-8eda-371606369bea", ...]`
- By querying `conversation_summaries.db`:
  `SELECT project_id, workspace_uris FROM conversation_summaries GROUP BY project_id, workspace_uris`
  we map each project UUID to its actual workspace filesystem path (e.g. `0a37d3b4-aa56-48d4-9762-e4f1c1098ec4` -> `/mnt/Data/Projects/Antigravity Swiss Knife`).

**4-Tier Matching Algorithm**:
Given a detected path $P$ from an imported session:
1. **Tier 1 (Exact Match)**: If $\text{norm}(P) == \text{norm}(W_i)$ for project $i$, return `(project_id_i, "exact")`.
2. **Tier 2 (Subpath Match)**: If $P$ is a subfolder of $W_i$, or $W_i$ is a subfolder of $P$, return `(project_id_i, "heuristic")`.
3. **Tier 3 (Basename Match)**: If $\text{basename}(P) == \text{basename}(W_i)$, return `(project_id_i, "heuristic")`.
4. **Tier 4 (Fallback Modes)**:
   - `auto`: Match to currently active project or first project in `projectsOrder`.
   - `create-new`: Generate a new project UUID and append to `projectsOrder`.
   - `standalone`: Assign empty string `""` or standalone marker.
   - `single-dedicated`: Assign dedicated group `imported-shared-archive`.
   - `source-dedicated`: Assign `imported-<source>`.

### 2.3 Real Session Parsers

#### 1. Claude Code Parser (`~/.claude/transcripts/*.jsonl`)
- Location: Scans `/home/david/.claude/transcripts/` for `ses_*.jsonl` files (verified 16+ active transcripts present).
- Record format:
  - `{"type": "user", "timestamp": "...", "content": "..."}`: User prompt turn.
  - `{"type": "tool_use", "timestamp": "...", "tool_name": "...", "tool_input": {...}}`: Tool invocation turn.
  - `{"type": "tool_result", "timestamp": "...", "tool_name": "...", "tool_output": "..."}`: Tool result turn.
- Title derivation: Content of the first user message (up to 75 characters) prefixed by `[Claude Code]`.
- Token estimation: File size divided by 4 or message count multiplied by 150.

#### 2. ChatGPT Export Parser (`conversations.json`)
- Format: Array of JSON objects exported by OpenAI.
- Structure:
  - `id`: Conversation ID.
  - `title`: Conversation title.
  - `create_time`: Float Unix timestamp.
  - `mapping`: Dict of nodes indexed by node ID.
    - Each node contains `message`:
      - `author.role`: `"user"` | `"assistant"` | `"system"`
      - `content.parts`: Array of strings
      - `create_time`: Float Unix timestamp
    - Node traversal: Walk node tree starting from the root or iterate chronologically by `create_time`.
- Title derivation: `[ChatGPT] ` + conversation title.

#### 3. Raw JSON Transcript Parser
- Flexible ingestion supporting:
  - Top-level message array: `[{"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}]`
  - Object with messages: `{"title": "...", "messages": [...]}`
  - Line-delimited JSON (JSONL).
  - Handles diverse key variations: `role`/`author`/`speaker`, `content`/`text`/`message`.

### 2.4 Pure Go SQLite Engine (`modernc.org/sqlite`)

- `modernc.org/sqlite` is 100% pure Go (transpiled C via `ccgo`).
- Works natively with `CGO_ENABLED=0` (verified in current environment).
- Zero external runtime dependencies; zero C compiler required.
- Implements standard `database/sql` driver (`"sqlite"`).
- Concurrency and lock handling:
  ```go
  db, err := sql.Open("sqlite", dbPath)
  _, _ = db.Exec("PRAGMA journal_mode = WAL;")
  _, _ = db.Exec("PRAGMA busy_timeout = 5000;")
  ```

---

## 3. Requirement R8: Full Pipeline Integration, Build & Tests

### 3.1 Styler Script Generation Pipeline (`pkg/gui/styler.go`)

#### The Problem:
In `pkg/gui/styler.go`:
```go
// Line 294:
func GenerateScript(cfg *Config) string {
    baseScript := ...
    customScript := custommodels.GenerateCustomModelsScript(cmCfg)
    enhScript := enhancements.GenerateEnhancementsScript(enhCfg)
    return baseScript + ";\n\n" + customScript + ";\n\n" + enhScript + ";"
}
```
`GenerateScript(cfg)` **does not call** `plugins.GenerateAuxiliaryPluginsScript()`.
`plugins.GenerateAuxiliaryPluginsScript()` was only called in `GenerateScriptWithCustomModels()`:
```go
// Line 978:
func GenerateScriptWithCustomModels(cfg *Config, cmCfg *custommodels.Config) string {
    base := GenerateScript(cfg)
    ...
    pluginsScript := plugins.GenerateAuxiliaryPluginsScript()
    ...
}
```
When `swiss patch sync` is executed, it calls `store.SyncPersistentFiles()` (`pkg/gui/store.go:149`), which calls `writePersistentFiles()`.
If any code calls `GenerateScript(cfg)`, the auxiliary plugins (Browser, Files, Memos, In-Chat Telemetry) are completely dropped from `persistent_script.js`.

#### The Solution:
Unify all script generators into `GenerateScript(cfg)` in `pkg/gui/styler.go`:
```go
func GenerateScript(cfg *Config) string {
    baseScript := generateBaseStylerScript(cfg)

    var cmCfg *custommodels.Config
    if cmStore, err := custommodels.NewStore(""); err == nil {
        c := cmStore.GetConfig()
        cmCfg = &c
    }
    customScript := custommodels.GenerateCustomModelsScript(cmCfg)

    var enhCfg *enhancements.EnhancementsConfig
    if enhStore, err := enhancements.NewStore(""); err == nil {
        c := enhStore.GetConfig()
        enhCfg = &c
    }
    enhScript := enhancements.GenerateEnhancementsScript(enhCfg)
    pluginsScript := plugins.GenerateAuxiliaryPluginsScript()

    var parts []string
    if baseScript != "" { parts = append(parts, baseScript) }
    if customScript != "" { parts = append(parts, customScript) }
    if enhScript != "" { parts = append(parts, enhScript) }
    if pluginsScript != "" { parts = append(parts, pluginsScript) }

    return strings.Join(parts, ";\n\n") + ";"
}
```
`GenerateScriptWithCustomModels(cfg, cmCfg)` then simply overrides `cmCfg` and delegates to the same bundled pipeline.

### 3.2 CLI Command Pipeline: `swiss patch sync`

In `cmd/swiss/main.go` (line 831):
```go
case "sync":
    if err := store.SyncPersistentFiles(); err != nil {
        fmt.Fprintf(os.Stderr, "Sync failed: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("Successfully synced persistent_styles.css and persistent_script.js")
```
`store.SyncPersistentFiles()` writes:
- `~/.config/antigravity-swiss/persistent_styles.css` via `GenerateCSS(cfg)` (which includes `plugins.GenerateAuxiliaryPluginsCSS()`).
- `~/.config/antigravity-swiss/persistent_script.js` via `GenerateScript(cfg)` (which now bundles all 4 scripts).

### 3.3 Build and Test Harnesses Audit

| Target | Command | Current Status | Issues & Fixes Required |
|:---|:---|:---:|:---|
| **Frontend Build** | `npm run build` (in `frontend/`) | **GREEN** | Build succeeds in 505ms (`vite v8.3.2`). |
| **Go Core Suite** | `go test ./pkg/core ./pkg/cache ...` | **GREEN** | All 15 existing packages pass cleanly. |
| **Go Plugins** | `go test ./pkg/plugins` | **FAIL** | Selector assertion mismatch in `pkg/plugins/auxiliary_test.go` lines 16, 22. Expects `.swiss-aux-btn-group` and `.swiss-annotation-canvas`, but `auxiliary.go` uses `.swiss-browser-btn` and `.swiss-browser-canvas-overlay`. Add aliases or align test. |
| **Go CLI Tests** | `go test ./cmd/swiss` | **PASS** (Partial) | Tests `version`, `help`, `status`, `quota`. Needs tests for `patch status` and `patch sync`. |
| **Go Importer Tests** | `go test ./pkg/importer` | **MISSING** | New test suite needed to verify Claude Code parsing, ChatGPT parsing, raw JSON parsing, and SQLite write. |

### 3.4 Documentation Needs in `README.md`

`README.md` needs updates to accurately describe:
1. **R5 Native SQLite Cross-Agent Chat Importer**:
   - Explanation of direct SQLite conversion into `conversation_summaries.db` and per-conversation `<id>.db`.
   - Workspace auto-matching with `app_storage.json` `projectsOrder`.
   - Supported sources: Claude Code transcripts, ChatGPT JSON export, raw JSON.
2. **R8 Persistent Desktop Script Bundling**:
   - `swiss patch sync` command and how it generates `persistent_script.js`.
   - Full bundle breakdown: Base Styler + Custom Models Injection + UI Enhancements + Auxiliary Panel Plugins.
3. **Auxiliary Panel Plugins Guide**:
   - Live Browser Preview with canvas annotations and "Send to Chat".
   - Auxiliary File Explorer with code editor and mutation endpoints.
   - Quick Memos with WebM/Opus audio recording.
   - In-Chat Token & TPS telemetry badge.

---

## 4. Implementation Blueprint for Engineering Team

### Step 1: Add `modernc.org/sqlite` to `go.mod`
Run:
```bash
go get modernc.org/sqlite@v1.60.1
```
This enables zero-CGO SQLite operations across Linux, Windows, and macOS.

### Step 2: Implement `pkg/importer` Package
Create:
- `pkg/importer/types.go`: Data structures for candidates, conversations, steps.
- `pkg/importer/claude.go`: Real parser for `~/.claude/transcripts/*.jsonl`.
- `pkg/importer/chatgpt.go`: Real parser for ChatGPT `conversations.json`.
- `pkg/importer/raw_json.go`: Real parser for generic JSON/JSONL transcripts.
- `pkg/importer/matcher.go`: Auto-matches detected paths against `app_storage.json` `projectsOrder` and known workspace URIs from `conversation_summaries.db`.
- `pkg/importer/writer.go`: Direct SQLite writer:
  - Inserts row into `~/.gemini/antigravity/conversation_summaries.db`.
  - Creates `~/.gemini/antigravity/conversations/<id>.db` with `trajectory_meta` and `steps`.
  - Creates `~/.gemini/antigravity/brain/<id>/.system_generated/logs/transcript.jsonl`.
- `pkg/importer/importer_test.go`: 100% test coverage for all parsers and SQLite writes.

### Step 3: Wire `pkg/importer` into `pkg/webgui/server.go`
In `pkg/webgui/server.go`:
- In `handleUtilitiesImportScan`: Call `importer.ScanCandidates(source)`.
- In `handleUtilitiesImport`: Call `importer.ExecuteImport(candidateIDs, source, mode)`.
- Delete `getAgentImporterScriptPath()` and all `exec.Command("python3", ...)` calls.

### Step 4: Unify Script Generation in `pkg/gui/styler.go`
- Update `GenerateScript(cfg)` to integrate `plugins.GenerateAuxiliaryPluginsScript()`.
- Add test in `pkg/gui/gui_test.go` asserting `GenerateScript(cfg)` contains identifiers from all four sub-scripts (`setupAuxiliaryTabs`, `swiss-browser-view`, `custommodels`, `promptJump`).

### Step 5: Fix `pkg/plugins/auxiliary_test.go`
- Update `auxiliary_test.go` or add alias classes in `auxiliary.go` so `go test ./pkg/plugins` passes green.

### Step 6: Add CLI Tests in `cmd/swiss/main_test.go`
- Add tests for `go run . patch status --json` and `go run . patch sync`.

### Step 7: Update `README.md`
- Document R5 and R8 features, CLI usage, and persistence architecture.
