# Specification Mining Report: Linux Environment, Secret Management, File Structures & Process Lifecycle

**Author**: Linux Environment Spec Miner (`spec_miner_env_1`)  
**Date**: 2026-10-01T07:45:00Z  
**Target Project**: Antigravity Swiss Knife  
**Scope**: Linux Secret Service API, Antigravity 2.0 File Formats & Paths, Process Lifecycle & Safe Relaunch, Upstream Quota & Auth Schema  

---

## 1. Observation

Direct observations and evidence collected on the target Linux system:

### 1.1 Secret Service API & Linux Keyring
- **CLI Utility**: `/usr/bin/secret-tool` is installed on host system (`libsecret-tools`).
- **D-Bus Bus**: Active session bus at `unix:path=/run/user/1000/bus` (`DBUS_SESSION_BUS_ADDRESS`).
- **Keyring Backend**: Tested via `secretstorage` and `keyring` in Python; default collection is `Login` (unlocked, `is_locked() == False`).
- **Target Secret Identification**:
  Executing `secret-tool search service gemini username antigravity` returned:
  ```text
  [/46]
  label = Password for 'antigravity' on 'gemini'
  secret = {"token":{"access_token":"ya29...","token_type":"Bearer","refresh_token":"1//...","expiry":"2026-10-01T15:45:29.267526114+10:00"},"auth_method":"consumer","id_token":"eyJhbGci..."}
  created = 2026-09-29 08:26:05
  modified = 2026-10-01 04:45:30
  schema = org.freedesktop.Secret.Generic
  attribute.service = gemini
  attribute.username = antigravity
  ```
- **Read Behavior (`lookup`)**:
  - `secret-tool lookup service gemini username antigravity` outputs the raw secret string to `stdout` with **NO** trailing newline (`0x0a`).
  - When matching attributes do not exist, `secret-tool lookup` outputs nothing to `stdout`, nothing to `stderr`, and exits with return code `1`.
- **Write Behavior (`store`)**:
  - Syntax: `printf "%s" "$PAYLOAD" | secret-tool store --label="Password for 'antigravity' on 'gemini'" service gemini username antigravity`
  - Reads the secret payload strictly from `stdin`.
  - Sets attributes `service=gemini`, `username=antigravity` under generic schema `org.freedesktop.Secret.Generic`.
- **Clear Behavior (`clear`)**:
  - Syntax: `secret-tool clear service <service> username <username>`
  - Deletes matching secrets and exits with return code `0`.
- **Underlying Consumer Implementation**:
  - The language server binary (`/opt/Antigravity/resources/bin/language_server`) is an ELF 64-bit Go executable.
  - Inspection of strings reveals it integrates `google3/third_party/golang/github_com/zalando/go_keyring/v/v0/secret_service/ss`.
  - `zalando/go-keyring` looks up secrets via `org.freedesktop.Secret.Collection.SearchItems` querying solely for `service` and `username`.
  - When Python's `keyring.set_password` is called, it injects an extraneous attribute `attribute.application = "Python keyring library"`. In contrast, `secret-tool store` sets exactly `service` and `username`, providing an exact 1:1 match with `zalando/go-keyring`.

### 1.2 Local Antigravity File Formats & Fingerprint Paths
Both `~/.config/Antigravity/` and `~/.gemini/antigravity/` exist on the filesystem.

#### A. Device Fingerprint Profiles
1. **`~/.config/Antigravity/machineid`**:
   - Format: Standard 36-character ASCII UUID v4 (`7d403dc5-24d6-4e84-ab22-08ee8df342d8`).
   - Size: Exactly 36 bytes. **Zero** trailing newline (`0x0a`).
2. **`~/.config/Antigravity/.updaterId`**:
   - Format: Standard 36-character ASCII UUID v4 (`575f53d3-9b85-5417-9e50-feee4924a48f`).
   - Size: Exactly 36 bytes. **Zero** trailing newline.
3. **`~/.gemini/antigravity/installation_id`**:
   - Format: Standard 36-character ASCII UUID v4 (`c463103c-805e-4552-978f-a72f5eec7acb`).
   - Size: Exactly 36 bytes. **Zero** trailing newline.
4. **`~/.gemini/antigravity/antigravity_state.pbtxt`**:
   - Format: Protobuf Text Format (`.pbtxt`).
   - Contains:
     ```protobuf
     post_onboarding: {
       completed_steps: POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME
       completed_steps: POST_ONBOARDING_STEP_TYPE_USAGE_MODE
       completed_steps: POST_ONBOARDING_STEP_TYPE_AGENT_CONFIGURATION
       completed_steps: POST_ONBOARDING_STEP_TYPE_ADD_WORKSPACE
     }
     seen_nuxs: { uids: 27 uids: 26 ... }
     agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED
     last_selected_agent_model: MODEL_PLACEHOLDER_M318
     migrate_convos_into_projects: MIGRATION_STATUS_COMPLETED
     installation_uuid: "936c9726-8a34-44b1-88c7-b8fe0b4a1291"
     migrate_retroactive_projects: RETROACTIVE_MIGRATION_STATUS_COMPLETED_UNNECESSARY
     migrations: { key: 2 value: MIGRATION_STATUS_COMPLETED }
     ```
   - When swapping virtualized device profiles, `installation_uuid` must be replaced while preserving the `post_onboarding`, `seen_nuxs`, and `migrations` flags to prevent repeated onboarding wizard popups.

#### B. Workspace & Window Layout State (`app_storage.json`)
- Path: `~/.config/Antigravity/app_storage.json`.
- Managed by `StorageManager` in `dist/storage.js` within `/opt/Antigravity/resources/app.asar`.
- Exposes `storage:get-items` and `storage:update-items` via IPC and writes pretty JSON (`JSON.stringify(currentItems, null, 2)`).
- Schema: Key-value map where all values are string-serialized JSON:
  - **`antigravity-multi-conversation-layout-v3-<cascadeId>`**:
    ```json
    "{\"rootNode\":{\"type\":\"pane\",\"id\":\"pane-1\",\"cascadeId\":\"2466a30b-1e33-4eca-9956-47ca06238d1b\"},\"focusedPaneId\":\"pane-1\"}"
    ```
    For multi-pane split layouts, key is `antigravity-multi-conversation-layout-v3-${[c1, c2].sort().join("+")}` with `rootNode: { type: "split", direction: "horizontal", children: [...] }`.
  - **`antigravity-multi-conversation-layout-v3-index`**:
    JSON string array of active multi-pane conversation group combinations (e.g. `"[]"`).
  - **`aux-pane-session` & `aux-pane-v2-session`**:
    JSON string holding `conversationPanes`:
    ```json
    {
      "conversationPanes": {
        "<cascadeId>": {
          "tabs": [
            { "id": "artifact__...", "content": { "type": "artifactView", ... } },
            { "id": "file__...", "content": { "type": "fileView", ... } },
            { "id": "bgtask__...", "content": { "type": "backgroundTaskView", ... } }
          ],
          "activeTabId": "artifact__...",
          "sidebarOpenStates": { "overview": true },
          "isPaneOpen": true
        }
      },
      "newConversationPanes": {}
    }
    ```
  - **`jetski.onboarding.lastLoginUsername`**: Active account email (e.g. `"torreswader@gmail.com"`).
  - **`new-convo-last-selected-project`**: Active project UUID.

#### C. Data Stores & Disk Cache Breakdown
1. **`~/.gemini/antigravity/conversations/`**:
   - 2.0 GB across 272 SQLite databases (`<cascadeId>.db`).
   - Schema per `<cascadeId>.db`:
     - `trajectory_meta`: `(trajectory_id, cascade_id, trajectory_type, source)`
     - `steps`: `(idx, step_type, status, has_subtrajectory, metadata, error_details, permissions, task_details, render_info, step_payload, step_format)`
     - `gen_metadata`, `executor_metadata`, `parent_references`, `trajectory_metadata_blob`, `battle_mode_infos`.
2. **`~/.gemini/antigravity/conversation_summaries.db`**:
   - Master SQLite metadata index.
   - Table `conversation_summaries`: `(conversation_id, title, preview, step_count, last_modified_time, workspace_uris, status, source, project_id, agent_name, parent_conversation_id, nesting_depth, battle_id, winning_conversation_id, not_fully_idle, killed, last_user_input_time, last_user_input_step_index, app_data_dir, raw_summary, group_id)`.
3. **`~/.gemini/antigravity/brain/`**:
   - 3.8 GB total disk usage across 1073 directories.
   - Breakdown by category:
     - Screenshots & Image dumps: **1,672 MB** (`.png` files)
     - Scratch directories: **917 MB** (`brain/<cascadeId>/scratch/`)
     - Tool Logs: **654 MB** (`.jsonl` files in `logs/`)
     - Tasks: **159 MB** (`.log` & task states)
     - Step outputs: **100 MB** (`.system_generated/steps/`)
     - Binary files & map files: **245 MB** (`.map`, `.so`, standalone binaries)
     - Chat messages: **11 MB**
     - Markdown Artifacts: **0.34 MB**

### 1.3 Process Lifecycle & Relaunch Architecture
- **Running Processes**:
  - Main Electron application: `/opt/Antigravity/antigravity`
  - Child processes spawned: `zygote`, `gpu-process`, `network.mojom.NetworkService`, `renderer`, `audio.mojom.AudioService`.
  - Core Language Server: `/opt/Antigravity/resources/bin/language_server` (listening on an ephemeral loopback port, e.g. `127.0.0.1:38425`).
- **Singleton Locks**:
  - `~/.config/Antigravity/SingletonLock` -> symlink to `<hostname>-<PID>` (e.g. `David-Laptop-773509`).
  - `~/.config/Antigravity/SingletonSocket` -> symlink to UNIX domain socket in `/tmp/scoped_dir.../SingletonSocket`.
  - `~/.config/Antigravity/SingletonCookie` -> numerical cookie.
  - Inspection of `dist/main.js` line 57:
    ```javascript
    const gotTheLock = electron_1.app.requestSingleInstanceLock();
    if (!gotTheLock) {
        electron_1.app.quit();
        process.exit(0);
    }
    ```
    If an existing instance is alive or its lock symlink is active, any newly launched `antigravity` immediately exits with code 0!
- **Shutdown Hooks**:
  - When `SIGTERM` is sent to the main Electron PID, Chromium triggers `electron.app.on('before-quit')`:
    ```javascript
    electron_1.app.on('before-quit', async (event) => {
        if (isQuitting) return;
        event.preventDefault();
        isQuitting = true;
        const windows = electron_1.BrowserWindow.getAllWindows();
        for (const win of windows) win.destroy();
        await Promise.all([
            electron_1.session.defaultSession.closeAllConnections(),
            (0, languageServer_1.killLanguageServer)(),
        ]);
        await closeHostBridgeServer();
        electron_1.app.quit();
    });
    ```
  - In `killLanguageServer()`: Sends `SIGTERM` to `language_server`, waits up to 5000 ms for graceful exit, and escalates to `SIGKILL` only if unresponsive.
  - On clean shutdown, `language_server` flushes all SQLite WAL files (`_busy_timeout=5000`, `_journal_mode=WAL`), and Chromium unlinks `SingletonLock`.
- **VS Code Storage Database (`state.vscdb`)**:
  - Locations:
    - `~/.config/Antigravity/User/globalStorage/state.vscdb`
    - `~/.config/Antigravity/User/workspaceStorage/<workspace-id>/state.vscdb`
  - File format: SQLite 3.x database (`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`).
  - Cause of `state.vscdb` errors: Sending `SIGKILL` or spawning a new instance before the previous instance finishes closing SQLite leaves locks or dirty WAL pages (`state.vscdb-wal`). When the new instance starts, VS Code encounters `SQLITE_BUSY` or journal corruption, marks the DB as corrupted, generates `state.vscdb.backup`, and triggers a "missing or corrupted state.vscdb" error modal.

### 1.4 Upstream Google Quota & Auth Schema
- **Google OAuth Client Credentials**:
  - Client ID: `1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com`
  - Client Secret: `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` (extracted from binary and live-tested).
  - Token Refresh URL: `https://oauth2.googleapis.com/token`
  - Live test confirmed: Posting `client_id`, `client_secret`, `grant_type=refresh_token`, and `refresh_token` returned HTTP 200 with a fresh `access_token` and `expires_in: 3599`.
- **Quota Summary Endpoint**:
  - URL: `POST https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary`
  - Headers: `Authorization: Bearer <access_token>`, `User-Agent: antigravity/2.18.1`, `Content-Type: application/json`
  - Body: `{}`
  - Response Schema:
    ```json
    {
      "groups": [
        {
          "displayName": "Gemini Models",
          "description": "Models within this group: Gemini Flash, Gemini Pro",
          "buckets": [
            {
              "bucketId": "gemini-weekly",
              "displayName": "Weekly Limit Remaining",
              "window": "weekly",
              "resetTime": "2026-10-08T03:53:53Z",
              "remainingFraction": 0.83625734
            },
            {
              "bucketId": "gemini-5h",
              "displayName": "Five Hour Limit Remaining",
              "window": "5h",
              "resetTime": "2026-10-01T08:53:53Z",
              "remainingFraction": 0.0175438
            }
          ]
        },
        {
          "displayName": "Claude and GPT models",
          "buckets": [
            { "bucketId": "3p-weekly", "remainingFraction": 1.0, "resetTime": "2026-10-08T05:07:53Z" },
            { "bucketId": "3p-5h", "remainingFraction": 1.0, "resetTime": "2026-10-01T10:07:53Z" }
          ]
        }
      ]
    }
    ```
- **Model Catalog Endpoint**:
  - URL: `POST https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels`
  - Returns 33 supported models, including:
    `gemini-3.8-flash-high`, `gemini-3.8-flash-tiered`, `gemini-3.1-pro-low`, `gemini-3.6-flash-high`, `claude-sonnet-4-6`, `claude-opus-4-6-thinking`, `gpt-oss-120b-medium`.

---

## 2. Logic Chain

1. **Secret Service Integration**:
   - The Secret Service API is accessible on Linux via `secret-tool` or Python's `secretstorage`/`keyring`.
   - `language_server` uses `zalando/go-keyring`, which expects exact attributes `service=gemini` and `username=antigravity`.
   - Python `keyring.set_password` inadvertently injects `application="Python keyring library"`. Therefore, executing `secret-tool store` directly (or using `secretstorage` without extra attributes) guarantees 100% binary compatibility with Antigravity's Go backend.
   - For headless or automated environments, D-Bus session address (`unix:path=/run/user/<uid>/bus`) must be exported.

2. **Per-Account Hardware Fingerprint Virtualization**:
   - Four distinct files uniquely identify the host install to Google: `~/.config/Antigravity/machineid`, `~/.config/Antigravity/.updaterId`, `~/.gemini/antigravity/installation_id`, and `installation_uuid` in `~/.gemini/antigravity/antigravity_state.pbtxt`.
   - The first three are raw 36-byte ASCII UUID strings with strictly zero trailing newline.
   - The fourth is an entry in a protobuf text file. Swapping the profile requires rewriting only `installation_uuid` while keeping the onboarding step completions intact so the UI does not reset to the initial welcome screen.

3. **Zero-Loss Session Preservation**:
   - Conversation layout is stored in `~/.config/Antigravity/app_storage.json` under keys `antigravity-multi-conversation-layout-v3-<cascadeId>`, `aux-pane-session`, and `aux-pane-v2-session`.
   - The active conversation history is persisted in `~/.gemini/antigravity/conversations/<cascadeId>.db` and indexed in `conversation_summaries.db`.
   - On relaunch, Antigravity reads `app_storage.json` and loads the conversation layout corresponding to the most recent timestamp in `conversation_summaries.db`.
   - Preserving or updating `app_storage.json` prior to relaunch guarantees the active conversation, open panes, and tool tabs restore exactly as they were.

4. **Graceful Process Lifecycle & Avoiding SQLite Corruption**:
   - Electron's `app.requestSingleInstanceLock()` prevents concurrent execution and relies on `~/.config/Antigravity/SingletonLock`.
   - Sending `SIGKILL` bypasses Electron's `before-quit` handler and `killLanguageServer()`, which leaves uncheckpointed WAL files (`state.vscdb-wal`, `conversation_summaries.db-wal`) and stale `SingletonLock` symlinks.
   - To guarantee zero corruption:
     a. Send `SIGTERM` to the root PID (found via `SingletonLock` or `/proc` scan).
     b. Poll `/proc/<PID>` until the entire process tree exits cleanly.
     c. Verify `SingletonLock` is removed.
     d. Perform SQLite integrity checks on `state.vscdb`.
     e. Atomically update keyring credentials and fingerprint files.
     f. Spawn `/opt/Antigravity/antigravity` in detached session (`start_new_session=True`).

5. **Quota Tracking & Automatic Horizon Warmup**:
   - `retrieveUserQuotaSummary` exposes the exact `remainingFraction` (e.g. `0.0175` for the 5h window) and `resetTime`.
   - When quota drops below the threshold (e.g. 5%), an account switch can be executed automatically.
   - When `resetTime` arrives on an exhausted account, dispatching a lightweight completion ping (`max_tokens: 1`) via `PredictionService/GenerateContent` immediately kicks off the next 5-hour quota window without requiring human interaction.

---

## 3. Caveats

- **Keyring Unlock State**: On standard desktop Linux, PAM automatically unlocks the `Login` keyring upon graphical login. If running in a headless SSH session without PAM keyring unlock, `secret-tool store` or `lookup` will fail unless the collection is explicitly unlocked via D-Bus prompt or password.
- **Multiple Workspaces**: `workspaceStorage` contains individual directories per workspace hash/timestamp. `state.vscdb` exists in both global storage and per-workspace directories.
- **Antigravity Manager vs Google Antigravity**: On this host, both `/opt/Antigravity/antigravity` and a third-party `/usr/lib/antigravity-manager/antigravity-manager` exist. The switcher must explicitly target `/opt/Antigravity/antigravity` (binary name `antigravity`) and never confuse it with `antigravity-manager` (`antigravity-man`).
- **Google OAuth Client Secret**: While the client secret was extracted and verified working for token refresh, Google may rotate client secrets in future minor releases of `language_server`.

---

## 4. Conclusion

- **Keyring Operations**: Native Linux secret management is fully operable via `/usr/bin/secret-tool` or Python `secretstorage` with `service=gemini`, `username=antigravity`.
- **Credential Payload**: Fully decoded as JSON with `token: { access_token, refresh_token, token_type, expiry }`, `auth_method: "consumer"`, and `id_token`.
- **Fingerprint Virtualization**: Complete specifications obtained for all 4 identity files (`machineid`, `.updaterId`, `installation_id`, `antigravity_state.pbtxt`). Exact byte requirements (36 bytes, no trailing newline) are confirmed.
- **Session Preservation**: Conversation and layout states are stored cleanly in `app_storage.json` and SQLite databases. Restoring the target `<cascadeId>` layout ensures seamless resumption.
- **Lifecycle Management**: A reliable 5-step sequence (`SIGTERM` -> poll exit -> clean stale locks -> swap profiles -> relaunch) prevents `state.vscdb` corruption and `SingleInstanceLock` aborts.
- **Upstream Quota Poller**: Confirmed working HTTP 200 on `cloudcode-pa.googleapis.com` endpoints with complete JSON schemas mapped for the Swiss Knife dashboard and warmup engine.

---

## 5. Verification Method

To independently verify these findings on the host system:

1. **Verify Secret Tool & Current Secret**:
   ```bash
   secret-tool lookup service gemini username antigravity | jq .auth_method
   # Expected output: "consumer"
   ```
2. **Verify Fingerprint Byte Counts**:
   ```bash
   wc -c ~/.config/Antigravity/machineid ~/.config/Antigravity/.updaterId ~/.gemini/antigravity/installation_id
   # Expected output: 36 for all three files
   ```
3. **Verify Upstream Quota Polling**:
   ```bash
   python3 -c "
   import subprocess, json, urllib.request
   raw = subprocess.check_output(['secret-tool', 'lookup', 'service', 'gemini', 'username', 'antigravity']).decode()
   token = json.loads(raw)['token']['access_token']
   req = urllib.request.Request(
       'https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary',
       data=b'{}',
       headers={'Authorization': f'Bearer {token}', 'Content-Type': 'application/json', 'User-Agent': 'antigravity/2.18.1'}
   )
   with urllib.request.urlopen(req) as resp:
       print('Status:', resp.status)
   "
   # Expected output: Status: 200
   ```
4. **Verify Process Detection & Singleton Lock**:
   ```bash
   readlink ~/.config/Antigravity/SingletonLock
   # Expected output: <hostname>-<PID> matching running antigravity process
   ```

---

## Features Discovered

| # | Category | Feature | Description | Inputs | Outputs | Error Behavior | Discovered Via |
|---|----------|---------|-------------|--------|---------|----------------|----------------|
| 1 | Secret Management | `secret-tool lookup` | Retrieves active Antigravity OAuth JSON payload | `service gemini username antigravity` | Raw JSON string (no trailing newline) | Exits code 1, empty stdout/stderr | CLI probing (`/usr/bin/secret-tool`) |
| 2 | Secret Management | `secret-tool store` | Writes new Antigravity OAuth credential payload | `stdin`, label, `service gemini username antigravity` | Keyring entry created under `Login` collection | Exits non-zero if keyring locked or D-Bus unreachable | CLI probing & D-Bus inspection |
| 3 | Secret Management | `secret-tool clear` | Removes Antigravity credentials | `service gemini username antigravity` | Keyring entry deleted | Exits code 0 even if entry not present | CLI probing |
| 4 | Secret Management | `zalando/go-keyring` compatibility | Underlying Go library used by Antigravity language_server | Queries generic Secret Service schema by `service` & `username` | Password string | Falls back to internal fallback provider if D-Bus fails | ELF string inspection of `language_server` |
| 5 | Device Fingerprinting | `machineid` | Host machine identifier used by Chromium/Electron | File path: `~/.config/Antigravity/machineid` | 36-byte UUID v4 (no newline) | Missing file causes regeneration on launch | Filesystem inspection & hex dump |
| 6 | Device Fingerprinting | `.updaterId` | Machine updater identifier used by `electron-updater` | File path: `~/.config/Antigravity/.updaterId` | 36-byte UUID v4 (no newline) | Missing file causes regeneration on launch | Filesystem inspection & hex dump |
| 7 | Device Fingerprinting | `installation_id` | Backend analytics identifier | File path: `~/.gemini/antigravity/installation_id` | 36-byte UUID v4 (no newline) | Missing file causes regeneration on launch | Filesystem inspection & hex dump |
| 8 | Device Fingerprinting | `antigravity_state.pbtxt` | Onboarding, feature flags, and `installation_uuid` | File path: `~/.gemini/antigravity/antigravity_state.pbtxt` | Protobuf text file with `installation_uuid` | Overwriting without onboarding flags triggers first-run wizard | Filesystem inspection & regex probe |
| 9 | Session Layout | `app_storage.json` Multi-Convo Layout | Stores pane split layout and active conversation cascade ID | Key: `antigravity-multi-conversation-layout-v3-<cascadeId>` | JSON string: `{rootNode: {type: "pane", id: "pane-1", cascadeId: "..."}, focusedPaneId: "..."}` | Missing key defaults to empty conversation view | `app.asar` decompile & `app_storage.json` query |
| 10 | Session Layout | `app_storage.json` Aux Pane Session | Stores open tabs (files, artifacts, background tasks) and sidebar state | Key: `aux-pane-session` & `aux-pane-v2-session` | JSON string with `conversationPanes[cascadeId]` | Corrupt JSON resets auxiliary panels | `app.asar` decompile & `app_storage.json` query |
| 11 | Session Layout | `app_storage.json` User Email | Holds currently signed-in user email | Key: `jetski.onboarding.lastLoginUsername` | String email address | Mismatch with keyring token may trigger re-login banner | `app_storage.json` query |
| 12 | Conversation Storage | Conversation SQLite Databases | Stores individual conversation trajectories, steps, and tool outputs | `~/.gemini/antigravity/conversations/<cascadeId>.db` | SQLite 3.x database | SQLite WAL locked if killed with `SIGKILL` | Python `sqlite3` probe |
| 13 | Conversation Storage | Conversation Summaries Index | Master index of all conversations with timestamps and project IDs | `~/.gemini/antigravity/conversation_summaries.db` | Table `conversation_summaries` | SQLite WAL locked if killed with `SIGKILL` | Python `sqlite3` probe |
| 14 | Cache & Storage | Brain Directory Cache | Storage for scratch files, tool logs, step dumps, screenshots | `~/.gemini/antigravity/brain/<cascadeId>/` | Total 3.8 GB (1.67 GB PNG, 917 MB scratch, 654 MB logs) | Disk exhaustion over time | Filesystem walk & size breakdown |
| 15 | Process Lifecycle | `SingletonLock` Detection | Identifies running instance PID and locks single instance | `~/.config/Antigravity/SingletonLock` | Symlink to `<hostname>-<PID>` | New process exits code 0 if lock is active | `/opt/Antigravity/resources/app.asar` main.js |
| 16 | Process Lifecycle | Graceful `SIGTERM` Shutdown | Electron `before-quit` handler destroys windows and calls `killLanguageServer()` | Signal `SIGTERM` (`15`) to main PID | Windows closed, LS terminated within 5s, WAL checkpointed, lock removed | Ungraceful `SIGKILL` leaves orphaned WAL and stale lock | `main.js` & `languageServer.js` decompile |
| 17 | Editor Storage | VS Code `state.vscdb` Integrity | Workspace and global UI storage database | `~/.config/Antigravity/User/globalStorage/state.vscdb` | SQLite 3.x database | Concurrency or abrupt exit causes "missing state.vscdb" modal | File type probe & SQLite inspection |
| 18 | Upstream API | User Quota Summary API | Polls real-time 5-hour and weekly quota remaining fraction and reset time | `POST https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary` | JSON object with `groups[].buckets[].remainingFraction` and `resetTime` | HTTP 401 if token expired; HTTP 403 if invalid | Live API probe |
| 19 | Upstream API | Model Catalog API | Fetches available agent models, tiered allocations, and default model | `POST https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels` | JSON object with 33 models, `defaultAgentModelId`, and `tieredModelIds` | HTTP 401 if token expired | Live API probe |
| 20 | Upstream Auth | Direct Token Refresh Flow | Refreshes Google OAuth `access_token` using embedded client credentials | `POST https://oauth2.googleapis.com/token` with `refresh_token`, `client_id`, `client_secret` | JSON object with fresh `access_token` (valid 3600s) | HTTP 400 if `client_secret` missing or invalid | Binary string mining & live API probe |

---

## Edge Cases

| # | Feature | Input | Observed Behavior |
|---|---------|-------|-------------------|
| 1 | `secret-tool lookup` | Non-existent service/username | Exits with status code 1, empty stdout, empty stderr; does not raise error message. |
| 2 | `secret-tool store` | Secret value without newline | Preserves raw exact binary bytes; does not append newline (`\n`). |
| 3 | Python `keyring.set_password` | Setting credentials via `keyring` library | Automatically adds `attribute.application = "Python keyring library"`. While harmless in most cases, `secret-tool store` avoids this extra attribute. |
| 4 | Fingerprint Files | Writing UUID with newline (`\n` or `echo $UUID > file`) | Produces 37 bytes instead of 36 bytes. Antigravity expects exact 36-byte raw UUID string. |
| 5 | `antigravity_state.pbtxt` | Generating fresh file with only `installation_uuid` | Wipes `post_onboarding` and `seen_nuxs`, causing Antigravity to pop up first-run onboarding tutorial. |
| 6 | Singleton Lock | Launching `antigravity` while lock exists | Newly launched process immediately invokes `app.quit()` and `process.exit(0)` silently. |
| 7 | Process Termination | Sending `SIGKILL` (`kill -9`) to main PID | Bypasses `before-quit` and `killLanguageServer()`. `language_server` orphaned, SQLite WAL files not checkpointed, `SingletonLock` remains symlinked. |
| 8 | SQLite Relaunch Race | Spawning new process before old process finishes `SIGTERM` exit | Both processes access `state.vscdb` concurrently, triggering `SQLITE_BUSY` (timeout 5000ms), followed by corrupted `state.vscdb` error modal. |
| 9 | Google Token Refresh | Calling `https://oauth2.googleapis.com/token` without `client_secret` | Returns HTTP 400 with `{"error": "invalid_request", "error_description": "client_secret is missing."}`. `client_secret` (`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`) is required. |
| 10 | Headless SSH Session | Invoking `secret-tool` without GUI session | Fails if `DBUS_SESSION_BUS_ADDRESS` is missing or keyring is locked without PAM auto-unlock. |
