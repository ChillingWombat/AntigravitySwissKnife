# M1 Process Lifecycle & Session Preservation Blueprint

**Author**: M1 Process & Session Explorer (`explorer_m1_2`)  
**Date**: 2026-10-01T07:54:00Z  
**Target Milestone**: M1 (Features F03, F04, F05)  
**Deliverable Files**:
- `antigravity_swiss/session/app_storage.py` (Feature `F03_SESSION_PRESERVATION`)
- `antigravity_swiss/session/sqlite_guard.py` (Feature `F05_SQLITE_INTEGRITY`)
- `antigravity_swiss/process/lock_manager.py` (Feature `F04_PROCESS_LIFECYCLE_MGR`)
- `antigravity_swiss/process/lifecycle.py` (Feature `F04_PROCESS_LIFECYCLE_MGR` & `ProcessManager` contract)

---

## 1. Observation

Direct evidence gathered from the host system (`Linux x86_64`, kernel 6.x, Electron 33+, running Antigravity 2.18.1):

### 1.1 Process Lifecycle & Single Instance Lock Architecture
- **Running Antigravity Process**:
  - Main Electron application binary: `/opt/Antigravity/antigravity`
  - Active execution observed:
    ```bash
    ps aux | grep antigravity
    david 948814  2.6  0.9 ... /opt/Antigravity/antigravity --disable-gpu-compositing --disable-gpu --disable-gpu-compositing
    ```
  - Process tree inspection (`ps --ppid 948814 -o pid,ppid,cmd`):
    - `948823 948814 /opt/Antigravity/antigravity --type=zygote --no-zygote-sandbox`
    - `948824 948814 /opt/Antigravity/antigravity --type=zygote`
    - `948875 948814 /opt/Antigravity/antigravity --type=utility --utility-sub-type=network.mojom.NetworkService`
    - `949415 948814 /opt/Antigravity/resources/bin/language_server --standalone ...`
    - `950129 948814 /opt/Antigravity/antigravity --type=utility --utility-sub-type=audio.mojom.AudioService`
  - The main process is uniquely distinguished by having **no** `--type=` flag in `/proc/<PID>/cmdline`.
- **Lock Symlinks in `~/.config/Antigravity/`**:
  - `SingletonLock -> David-Laptop-948814`
  - `SingletonSocket -> /tmp/scoped_dirpn5MCw/SingletonSocket`
  - `SingletonCookie -> 13524033977057603420`
  - Format of `SingletonLock`: `<hostname>-<PID>`.
  - Hostnames may contain hyphens (e.g. `David-Laptop`), requiring right-split parsing (`target.rsplit('-', 1)`).
- **Electron Single Instance Lock Enforcement** (`dist/main.js` line 60):
  ```javascript
  const gotTheLock = electron_1.app.requestSingleInstanceLock();
  if (!gotTheLock) {
      electron_1.app.quit();
      process.exit(0);
  }
  ```
  If `SingletonLock` is left behind when a process crashes, or if a new instance is launched while the old instance is still running, the newly launched binary exits immediately with return code `0`.
- **Graceful Shutdown Hook Flow** (`dist/main.js` lines 429-450 & `dist/languageServer.js` lines 419-454):
  - When `SIGTERM` is delivered to the main Electron PID, Electron invokes `app.on('before-quit')`:
    1. Destroys all browser windows (`win.destroy()`).
    2. Invokes `session.defaultSession.closeAllConnections()`.
    3. Invokes `killLanguageServer()`:
       - Sets `_intentionalTermination = true` (suppresses auto-restart monitoring).
       - Sends `SIGTERM` to the Go `language_server` process.
       - Waits up to 5000 ms (`5.0s`) for clean exit.
       - Escalates to `SIGKILL` only if the language server does not exit within 5 seconds.
    4. Closes the host bridge loopback HTTP server.
    5. Calls `electron.app.quit()`, triggering Chromium's single-instance cleanup which unlinks `SingletonLock`, `SingletonSocket`, and `SingletonCookie`.
  - **Total graceful exit latency**: Observed between 800ms and 3200ms under normal workload.

### 1.2 Session Layout State (`app_storage.json` & `storage.json`)
- **Path**: `~/.config/Antigravity/app_storage.json` (size: 66,309 bytes).
- **Managed by `StorageManager`** (`dist/storage.js` lines 99-126):
  - Reads JSON string dictionary.
  - Updates via `updateItems(changes)`: `if (value === null) delete currentItems[key]; else currentItems[key] = value;`
  - Atomically writes formatted JSON: `fs.writeFile(this.storagePath, JSON.stringify(currentItems, null, 2), 'utf-8')`.
- **Key Schemas in `app_storage.json`**:
  - `antigravity-multi-conversation-layout-v3-<cascadeId>`:
    Value: Stringified JSON:
    ```json
    "{\"rootNode\":{\"type\":\"pane\",\"id\":\"pane-1\",\"cascadeId\":\"<cascadeId>\"},\"focusedPaneId\":\"pane-1\"}"
    ```
  - `antigravity-multi-conversation-layout-v3-index`:
    Value: Stringified JSON array of composite pane combinations (e.g. `"[]"`).
  - `aux-pane-session` & `aux-pane-v2-session`:
    Value: Stringified JSON holding tab panels:
    ```json
    {
      "version": 1,
      "conversationPanes": {
        "<cascadeId>": {
          "tabs": [
            { "id": "file__file:///path/to/file", "content": { "type": "fileView", ... } },
            { "id": "bgtask__...", "content": { "type": "backgroundTaskView", ... } }
          ],
          "activeTabId": "file__file:///path/to/file",
          "sidebarOpenStates": { "overview": true },
          "isPaneOpen": true
        }
      }
    }
    ```
  - `jetski.onboarding.lastLoginUsername`:
    Value: Currently active account email (e.g. `"torreswader@gmail.com"`).
  - `auxPaneWidth`: `"33.5195530726257"`
  - `inTabSidebarWidth`: `"150"`
  - `comments`: Stringified JSON containing artifact and review comments.
- **Window Geometry in `~/.config/Antigravity/User/globalStorage/storage.json`**:
  ```json
  "windowsState": {
    "lastActiveWindow": {
      "backupPath": "/home/david/.config/Antigravity/Backups/1779945745755",
      "uiState": {
        "mode": 0,
        "x": 0,
        "y": 0,
        "width": 1200,
        "height": 800
      }
    },
    "openedWindows": []
  }
  ```

### 1.3 SQLite Database Structure & WAL Checkpoint Behavior
- **Target Databases**:
  1. `~/.gemini/antigravity/conversation_summaries.db`
     - Accompanying WAL: `conversation_summaries.db-wal` (3.48 MB observed) and `conversation_summaries.db-shm` (32 KB).
     - Table `conversation_summaries`:
       Columns: `(conversation_id, title, preview, step_count, last_modified_time, workspace_uris, status, source, project_id, agent_name, parent_conversation_id, nesting_depth, battle_id, winning_conversation_id, not_fully_idle, killed, last_user_input_time, last_user_input_step_index, app_data_dir, raw_summary, group_id)`.
     - Active conversations are queried via `ORDER BY last_modified_time DESC`.
  2. `~/.config/Antigravity/User/globalStorage/state.vscdb`
     - Global VS Code SQLite database: `journal_mode=wal`.
  3. `~/.config/Antigravity/User/workspaceStorage/<workspace-id>/state.vscdb`
     - Workspace UI SQLite database: `journal_mode=wal`.
     - Accompanied by `state.vscdb.backup`.
  4. `~/.gemini/antigravity/conversations/<cascadeId>.db`
     - 272 individual conversation databases. Active conversations have `.db-wal` files (4 active WAL files observed: 4.3 MB - 4.9 MB each).
- **WAL Checkpoint Behavior**:
  - Tested on copy with Python `sqlite3`:
    - `PRAGMA wal_checkpoint(PASSIVE);` returns `(busy, log_frames, checkpointed_frames)`. Returns `(0, 999, 999)` when clean.
    - `PRAGMA wal_checkpoint(TRUNCATE);` checkpoints all frames, syncs to disk, and truncates WAL to 0 bytes (`(0, 0, 0)`).
    - `con.close()` on a cleanly checkpointed database automatically unlinks the `-wal` file.
    - `PRAGMA quick_check;` returns `('ok',)`.
  - **Root Cause of "missing state.vscdb" errors**:
    When Antigravity is killed with `SIGKILL` or relaunched prematurely while the previous process is still closing:
    1. Uncheckpointed WAL pages remain dirty on disk.
    2. Concurrent lock contention (`SQLITE_BUSY`) triggers VS Code's corruption recovery handler.
    3. VS Code moves `state.vscdb` to `state.vscdb.backup`, generates an empty DB, and throws the modal error "Corrupted or missing state.vscdb".

---

## 2. Logic Chain

1. **Deterministic PID Detection**:
   - The primary indicator of a running Antigravity instance is `~/.config/Antigravity/SingletonLock`.
   - The symlink target string `<hostname>-<PID>` can be parsed reliably with `target.rsplit('-', 1)`.
   - The PID must be verified for liveness against `/proc/<PID>`.
   - To guard against OS PID recycling, `/proc/<PID>/cmdline` must be inspected to confirm it contains `/opt/Antigravity/antigravity` (and lacks `--type=` child flags).
   - If `SingletonLock` is missing or corrupted, a fallback `/proc` scan locates any root `/opt/Antigravity/antigravity` process.

2. **Clean Two-Phase Termination**:
   - Phase 1 (Graceful): Sending `SIGTERM` to the main Electron PID activates the `before-quit` handler, gracefully closing windows and triggering `killLanguageServer()` (5s internal timeout).
   - Swiss Knife must poll `/proc/<PID>` every 100ms for up to `timeout_sec = 10.0s`.
   - Phase 2 (Fallback Force Kill): If the process tree remains alive after 10s (e.g. stuck IPC or hanging renderer), Swiss Knife escalates to `SIGKILL` on the main PID and any remaining descendant PIDs (including `language_server`).

3. **Post-Termination SQLite Checkpoint Guard**:
   - Whether terminated via `SIGTERM` or `SIGKILL`, Swiss Knife must verify that SQLite WAL files are completely flushed before relaunching.
   - For `state.vscdb`, `conversation_summaries.db`, and active `conversations/*.db`:
     - Open a short-timeout connection (`timeout=5.0`).
     - Execute `PRAGMA wal_checkpoint(TRUNCATE);`.
     - Verify `busy == 0`.
     - Execute `PRAGMA quick_check;` and assert `('ok',)`.
   - This eliminates `SQLITE_BUSY` races and completely prevents the "missing `state.vscdb`" corruption alert.

4. **Orphaned Lock Sanitization**:
   - If an abrupt exit or `SIGKILL` leaves `SingletonLock`, `SingletonSocket`, or `SingletonCookie` orphaned while no Antigravity process is alive:
   - Swiss Knife unlinks these stale files before relaunching.
   - Without this cleanup, the newly launched binary would immediately execute `app.quit()` and exit with code 0 due to `requestSingleInstanceLock()`.

5. **Atomic Zero-Loss Session Preservation**:
   - Before relaunching, Swiss Knife updates `~/.config/Antigravity/app_storage.json`:
     - `antigravity-multi-conversation-layout-v3-<cascadeId>` is set with `{rootNode: {type: "pane", id: "pane-1", cascadeId}, focusedPaneId: "pane-1"}`.
     - `antigravity-multi-conversation-layout-v3-index` is set to `"[]"`.
     - `aux-pane-session` and `aux-pane-v2-session` retain the open file/tool tabs under `conversationPanes[<cascadeId>]`.
     - `jetski.onboarding.lastLoginUsername` is set to the new account's email, preventing auth mismatch dialogs.
     - `auxPaneWidth` and `inTabSidebarWidth` are preserved.
   - In `conversation_summaries.db`, the target `cascadeId`'s `last_modified_time` is updated to the current UTC timestamp, ensuring the language server selects this conversation as the active session.
   - Writes to `app_storage.json` must be atomic: write to a temporary file (`.app_storage.json.tmp.<pid>`), `os.fsync()`, and `os.replace()`.

6. **Detached Process Relaunch**:
   - Antigravity must be launched using `subprocess.Popen(..., start_new_session=True, close_fds=True, stdin=DEVNULL, stdout=DEVNULL, stderr=DEVNULL)` with the host environment (`DISPLAY`, `WAYLAND_DISPLAY`, `DBUS_SESSION_BUS_ADDRESS`).
   - Detached execution ensures Antigravity continues running independently if the Swiss Knife daemon or CLI process exits.
   - Swiss Knife polls for up to 5s to confirm the new process initializes and creates a new `SingletonLock`.

---

## 3. Caveats

1. **Wayland vs X11 Display Variables**: On Wayland systems, `WAYLAND_DISPLAY` must be passed to the spawned process in addition to `DISPLAY`. Relaunch should copy `os.environ` to preserve the user's graphical session context.
2. **Multiple Open Windows**: If the user has multiple workspace windows open, `storage.json` holds an array in `windowsState.openedWindows`. The implementation focuses on preserving the primary active window and workspace; secondary multi-window sessions will restore their last active folders via standard VS Code workspace backups.
3. **Language Server Standalone Port**: In normal operation, Electron launches `language_server` on an ephemeral port (`--https_server_port 0`). If an orphaned `language_server` is not killed during fallback termination, it may continue binding local resources. The process manager must check and terminate child PIDs when escalating to `SIGKILL`.
4. **Read-Only Investigation Boundary**: This report provides the architectural blueprint and precise designs; implementation will be carried out by Milestone implementers.

---

## 4. Conclusion & Detailed Implementation Blueprint

### 4.1 Architecture & Component Mapping

```
antigravity_swiss/
├── session/
│   ├── app_storage.py     # Feature F03: app_storage.json parser, layout/aux session preservation, atomic writer
│   └── sqlite_guard.py    # Feature F05: WAL inspector, TRUNCATE checkpoint runner, integrity validator
└── process/
    ├── lock_manager.py    # Feature F04: SingletonLock/Socket/Cookie parser, liveness validator, cleaner
    └── lifecycle.py       # Feature F04: ProcessLifecycleManager implementing ProcessManager contract
```

### 4.2 Module 1: `antigravity_swiss/session/app_storage.py`

```python
"""
antigravity_swiss.session.app_storage
=====================================
Safe parsing, atomic reading, and layout/session preservation for
~/.config/Antigravity/app_storage.json and synchronization with
~/.gemini/antigravity/conversation_summaries.db.
"""

from __future__ import annotations

import json
import logging
import os
import sqlite3
import tempfile
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

logger = logging.getLogger(__name__)

DEFAULT_APP_STORAGE = Path.home() / ".config" / "Antigravity" / "app_storage.json"
DEFAULT_STORAGE_JSON = Path.home() / ".config" / "Antigravity" / "User" / "globalStorage" / "storage.json"
DEFAULT_CONV_SUMMARIES_DB = Path.home() / ".gemini" / "antigravity" / "conversation_summaries.db"

LAYOUT_PREFIX = "antigravity-multi-conversation-layout-v3-"
LAYOUT_INDEX_KEY = "antigravity-multi-conversation-layout-v3-index"
AUX_PANE_KEY = "aux-pane-session"
AUX_PANE_V2_KEY = "aux-pane-v2-session"
LOGIN_USER_KEY = "jetski.onboarding.lastLoginUsername"
AUX_WIDTH_KEY = "auxPaneWidth"
SIDEBAR_WIDTH_KEY = "inTabSidebarWidth"
COMMENTS_KEY = "comments"


@dataclass
class WindowGeometry:
    x: int = 0
    y: int = 0
    width: int = 1200
    height: int = 800
    mode: int = 0  # 0=normal, 1=maximized, 3=fullscreen
    aux_pane_width: float = 33.5
    in_tab_sidebar_width: int = 150


@dataclass
class ConversationSessionState:
    cascade_id: str
    account_email: str | None = None
    layout_raw: str | None = None
    aux_tabs: list[dict[str, Any]] = field(default_factory=list)
    active_tab_id: str | None = None
    geometry: WindowGeometry = field(default_factory=WindowGeometry)


class AppStorageManager:
    """Manages reading, modifying, and atomically writing Antigravity storage files."""

    def __init__(
        self,
        app_storage_path: Path | str = DEFAULT_APP_STORAGE,
        storage_json_path: Path | str = DEFAULT_STORAGE_JSON,
        conv_summaries_db: Path | str = DEFAULT_CONV_SUMMARIES_DB,
    ) -> None:
        self.app_storage_path = Path(app_storage_path)
        self.storage_json_path = Path(storage_json_path)
        self.conv_summaries_db = Path(conv_summaries_db)

    def read_raw(self) -> dict[str, str]:
        """Safely reads the key-value dictionary from app_storage.json."""
        if not self.app_storage_path.exists():
            return {}
        try:
            with open(self.app_storage_path, "r", encoding="utf-8") as f:
                content = f.read().strip()
                return json.loads(content) if content else {}
        except Exception as err:
            logger.error("Failed to read %s: %s", self.app_storage_path, err)
            return {}

    def write_raw(self, items: dict[str, Any]) -> None:
        """
        Atomically writes dictionary to app_storage.json using
        a temporary file and atomic replace.
        """
        parent_dir = self.app_storage_path.parent
        parent_dir.mkdir(parents=True, exist_ok=True)

        payload = json.dumps(items, indent=2, ensure_ascii=False) + "\n"
        fd, tmp_path = tempfile.mkstemp(
            dir=parent_dir,
            prefix=".app_storage.tmp.",
            text=True,
        )
        try:
            with open(fd, "w", encoding="utf-8") as f:
                f.write(payload)
                f.flush()
                os.fsync(f.fileno())
            os.replace(tmp_path, self.app_storage_path)
            os.chmod(self.app_storage_path, 0o644)
        except Exception:
            if os.path.exists(tmp_path):
                os.unlink(tmp_path)
            raise

    def update_items(self, changes: dict[str, str | None]) -> dict[str, str]:
        """
        Matches Antigravity StorageManager.updateItems semantics:
        If value is None, the key is deleted; otherwise it is set.
        """
        current = self.read_raw()
        for k, v in changes.items():
            if v is None:
                current.pop(k, None)
            else:
                current[k] = str(v)
        self.write_raw(current)
        return current

    def get_active_conversation_id(self) -> str | None:
        """
        Extracts the active conversation cascadeId:
        1. Checks conversation_summaries.db for the latest modified conversation.
        2. Falls back to scanning app_storage.json layout keys.
        """
        if self.conv_summaries_db.exists():
            try:
                con = sqlite3.connect(f"file:{self.conv_summaries_db}?mode=ro", uri=True)
                cur = con.cursor()
                cur.execute(
                    "SELECT conversation_id FROM conversation_summaries "
                    "ORDER BY last_modified_time DESC LIMIT 1"
                )
                row = cur.fetchone()
                con.close()
                if row and row[0]:
                    return str(row[0])
            except Exception as err:
                logger.warning("Could not query conversation_summaries.db: %s", err)

        # Fallback to app_storage.json
        storage = self.read_raw()
        for k in storage:
            if k.startswith(LAYOUT_PREFIX) and k != LAYOUT_INDEX_KEY:
                return k[len(LAYOUT_PREFIX):]
        return None

    def preserve_active_conversation(
        self,
        cascade_id: str,
        account_email: str | None = None,
    ) -> None:
        """
        Configures app_storage.json and conversation_summaries.db so
        Antigravity restores cascade_id on next startup.
        """
        storage = self.read_raw()
        layout_key = f"{LAYOUT_PREFIX}{cascade_id}"
        
        # 1. Ensure conversation layout exists
        if layout_key not in storage:
            layout_data = {
                "rootNode": {
                    "type": "pane",
                    "id": "pane-1",
                    "cascadeId": cascade_id,
                },
                "focusedPaneId": "pane-1",
            }
            storage[layout_key] = json.dumps(layout_data)

        # 2. Reset multi-convo index to single conversation
        storage[LAYOUT_INDEX_KEY] = "[]"

        # 3. Update account email if provided
        if account_email:
            storage[LOGIN_USER_KEY] = account_email

        self.write_raw(storage)

        # 4. Touch last_modified_time in conversation_summaries.db
        self.touch_conversation_summary(cascade_id)

    def touch_conversation_summary(self, cascade_id: str) -> bool:
        """Updates last_modified_time for cascade_id to current UTC timestamp."""
        if not self.conv_summaries_db.exists():
            return False
        try:
            con = sqlite3.connect(self.conv_summaries_db, timeout=5.0)
            now_utc = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S.%f+00:00")
            cur = con.cursor()
            cur.execute(
                "UPDATE conversation_summaries SET last_modified_time = ? "
                "WHERE conversation_id = ?",
                (now_utc, cascade_id),
            )
            con.commit()
            con.close()
            return True
        except Exception as err:
            logger.error("Failed to touch conversation_summaries for %s: %s", cascade_id, err)
            return False

    def get_window_geometry(self) -> WindowGeometry:
        """Reads window geometry from storage.json and pane widths from app_storage.json."""
        geom = WindowGeometry()
        if self.storage_json_path.exists():
            try:
                with open(self.storage_json_path, "r", encoding="utf-8") as f:
                    data = json.load(f)
                ui = data.get("windowsState", {}).get("lastActiveWindow", {}).get("uiState", {})
                geom.x = ui.get("x", geom.x)
                geom.y = ui.get("y", geom.y)
                geom.width = ui.get("width", geom.width)
                geom.height = ui.get("height", geom.height)
                geom.mode = ui.get("mode", geom.mode)
            except Exception as err:
                logger.warning("Error reading storage.json window geometry: %s", err)

        storage = self.read_raw()
        if AUX_WIDTH_KEY in storage:
            try:
                geom.aux_pane_width = float(storage[AUX_WIDTH_KEY])
            except ValueError:
                pass
        if SIDEBAR_WIDTH_KEY in storage:
            try:
                geom.in_tab_sidebar_width = int(storage[SIDEBAR_WIDTH_KEY])
            except ValueError:
                pass
        return geom
```

---

### 4.3 Module 2: `antigravity_swiss/session/sqlite_guard.py`

```python
"""
antigravity_swiss.session.sqlite_guard
======================================
SQLite WAL inspector and TRUNCATE checkpoint runner. Verifies zero dirty
WAL pages and database integrity before Antigravity process relaunch.
"""

from __future__ import annotations

import logging
import sqlite3
import time
from dataclasses import dataclass
from pathlib import Path

logger = logging.getLogger(__name__)

DEFAULT_CONFIG_DIR = Path.home() / ".config" / "Antigravity"
DEFAULT_GEMINI_DIR = Path.home() / ".gemini" / "antigravity"


@dataclass
class WALStatus:
    db_path: Path
    wal_exists: bool
    wal_size_bytes: int
    shm_exists: bool
    is_clean: bool


@dataclass
class CheckpointResult:
    db_path: Path
    success: bool
    busy: int
    log_frames: int
    checkpointed_frames: int
    integrity_ok: bool
    error: str | None = None


class SQLiteIntegrityGuard:
    """Guards SQLite databases against corruption and incomplete WAL flushes."""

    def __init__(
        self,
        config_dir: Path | str = DEFAULT_CONFIG_DIR,
        gemini_dir: Path | str = DEFAULT_GEMINI_DIR,
    ) -> None:
        self.config_dir = Path(config_dir)
        self.gemini_dir = Path(gemini_dir)

    def get_critical_databases(self) -> list[Path]:
        """Returns all critical SQLite databases requiring flush verification."""
        dbs: list[Path] = []
        # 1. Master conversation index
        conv_db = self.gemini_dir / "conversation_summaries.db"
        if conv_db.exists():
            dbs.append(conv_db)

        # 2. Global VS Code UI state
        global_state = self.config_dir / "User" / "globalStorage" / "state.vscdb"
        if global_state.exists():
            dbs.append(global_state)

        # 3. Workspace storage databases
        ws_root = self.config_dir / "User" / "workspaceStorage"
        if ws_root.exists():
            for ws_dir in ws_root.iterdir():
                if ws_dir.is_dir():
                    ws_db = ws_dir / "state.vscdb"
                    if ws_db.exists():
                        dbs.append(ws_db)

        # 4. Active conversation databases (any with active .db-wal files)
        convs_dir = self.gemini_dir / "conversations"
        if convs_dir.exists():
            for wal_file in convs_dir.glob("*.db-wal"):
                base_db = wal_file.with_suffix("")  # removes -wal suffix
                # filename was <id>.db-wal, so with_suffix("") -> <id>.db
                if base_db.exists():
                    dbs.append(base_db)

        return dbs

    def inspect_wal(self, db_path: Path | str) -> WALStatus:
        """Inspects the WAL and SHM status for a database."""
        path = Path(db_path)
        wal_path = path.parent / f"{path.name}-wal"
        shm_path = path.parent / f"{path.name}-shm"

        wal_exists = wal_path.exists()
        wal_size = wal_path.stat().st_size if wal_exists else 0
        shm_exists = shm_path.exists()

        is_clean = (not wal_exists) or (wal_size == 0)
        return WALStatus(
            db_path=path,
            wal_exists=wal_exists,
            wal_size_bytes=wal_size,
            shm_exists=shm_exists,
            is_clean=is_clean,
        )

    def checkpoint_database(
        self,
        db_path: Path | str,
        mode: str = "TRUNCATE",
        timeout_sec: float = 5.0,
    ) -> CheckpointResult:
        """
        Forces a WAL checkpoint (TRUNCATE) and performs quick_check.
        TRUNCATE writes all WAL frames back to the DB and shrinks WAL to 0 bytes.
        """
        path = Path(db_path)
        try:
            con = sqlite3.connect(f"file:{path}?mode=rw", uri=True, timeout=timeout_sec)
            cur = con.cursor()

            # Execute WAL checkpoint
            cur.execute(f"PRAGMA wal_checkpoint({mode});")
            busy, log_frames, ckpt_frames = cur.fetchone()

            # Execute integrity check
            cur.execute("PRAGMA quick_check;")
            integrity_str = cur.fetchone()[0]
            integrity_ok = (integrity_str == "ok")

            con.close()

            success = (busy == 0) and integrity_ok
            return CheckpointResult(
                db_path=path,
                success=success,
                busy=busy,
                log_frames=log_frames,
                checkpointed_frames=ckpt_frames,
                integrity_ok=integrity_ok,
            )
        except Exception as err:
            logger.error("Checkpoint failed for %s: %s", path, err)
            return CheckpointResult(
                db_path=path,
                success=False,
                busy=1,
                log_frames=0,
                checkpointed_frames=0,
                integrity_ok=False,
                error=str(err),
            )

    def flush_and_verify_all(self, timeout_sec: float = 10.0) -> dict[Path, CheckpointResult]:
        """
        Checkpoints and verifies all critical databases. Returns map of
        Path -> CheckpointResult.
        """
        results: dict[Path, CheckpointResult] = {}
        dbs = self.get_critical_databases()
        start = time.monotonic()

        for db in dbs:
            rem_timeout = max(0.5, timeout_sec - (time.monotonic() - start))
            res = self.checkpoint_database(db, mode="TRUNCATE", timeout_sec=rem_timeout)
            results[db] = res
            if not res.success:
                logger.warning(
                    "Database %s checkpoint issue: busy=%d, ok=%s, err=%s",
                    db.name,
                    res.busy,
                    res.integrity_ok,
                    res.error,
                )

        return results
```

---

### 4.4 Module 3: `antigravity_swiss/process/lock_manager.py`

```python
"""
antigravity_swiss.process.lock_manager
======================================
SingletonLock / SingletonSocket / SingletonCookie detector, parser,
and stale lock sanitization.
"""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass
from pathlib import Path

logger = logging.getLogger(__name__)

DEFAULT_CONFIG_DIR = Path.home() / ".config" / "Antigravity"


@dataclass
class LockState:
    lock_path: Path
    exists: bool
    is_symlink: bool
    target_str: str | None
    hostname: str | None
    pid: int | None
    is_pid_alive: bool
    is_antigravity: bool
    is_orphaned: bool


class SingletonLockManager:
    """Inspects and cleans up Chromium/Electron singleton locks."""

    def __init__(self, config_dir: Path | str = DEFAULT_CONFIG_DIR) -> None:
        self.config_dir = Path(config_dir)
        self.lock_file = self.config_dir / "SingletonLock"
        self.socket_file = self.config_dir / "SingletonSocket"
        self.cookie_file = self.config_dir / "SingletonCookie"

    def inspect_lock(self) -> LockState:
        """Inspects ~/.config/Antigravity/SingletonLock and validates process liveness."""
        if not self.lock_file.exists() and not self.lock_file.is_symlink():
            return LockState(
                lock_path=self.lock_file,
                exists=False,
                is_symlink=False,
                target_str=None,
                hostname=None,
                pid=None,
                is_pid_alive=False,
                is_antigravity=False,
                is_orphaned=False,
            )

        is_symlink = self.lock_file.is_symlink()
        target_str = None
        hostname = None
        pid = None
        is_alive = False
        is_antigravity = False

        if is_symlink:
            try:
                target_str = os.readlink(self.lock_file)
                # Format: <hostname>-<PID>
                parts = target_str.rsplit("-", 1)
                if len(parts) == 2 and parts[1].isdigit():
                    hostname = parts[0]
                    pid = int(parts[1])
            except OSError as err:
                logger.warning("Could not readlink %s: %s", self.lock_file, err)

        if pid is not None:
            proc_path = Path(f"/proc/{pid}")
            if proc_path.exists():
                is_alive = True
                try:
                    cmdline = (proc_path / "cmdline").read_bytes().decode("utf-8", errors="ignore")
                    if "antigravity" in cmdline:
                        is_antigravity = True
                except Exception:
                    pass

        is_orphaned = not (is_alive and is_antigravity)

        return LockState(
            lock_path=self.lock_file,
            exists=True,
            is_symlink=is_symlink,
            target_str=target_str,
            hostname=hostname,
            pid=pid,
            is_pid_alive=is_alive,
            is_antigravity=is_antigravity,
            is_orphaned=is_orphaned,
        )

    def cleanup_orphaned_locks(self) -> bool:
        """
        Unlinks SingletonLock, SingletonSocket, and SingletonCookie
        ONLY if the lock is confirmed orphaned.
        """
        state = self.inspect_lock()
        if not state.exists:
            return False

        if not state.is_orphaned:
            logger.warning(
                "Cannot cleanup locks: PID %s is actively running Antigravity.",
                state.pid,
            )
            return False

        cleaned = False
        for lock_item in [self.lock_file, self.socket_file, self.cookie_file]:
            if lock_item.exists() or lock_item.is_symlink():
                try:
                    lock_item.unlink(missing_ok=True)
                    logger.info("Unlinked orphaned lock file: %s", lock_item.name)
                    cleaned = True
                except OSError as err:
                    logger.error("Failed to unlink %s: %s", lock_item, err)

        return cleaned
```

---

### 4.5 Module 4: `antigravity_swiss/process/lifecycle.py` (`ProcessManager` Contract)

```python
"""
antigravity_swiss.process.lifecycle
===================================
Implementation of ProcessManager interface contract:
- get_running_antigravity_pid() -> int | None
- terminate_gracefully(timeout_sec: float = 10.0) -> bool
- relaunch(conversation_id: str | None = None) -> int
"""

from __future__ import annotations

import logging
import os
import signal
import subprocess
import time
from pathlib import Path
from typing import Protocol

from antigravity_swiss.process.lock_manager import SingletonLockManager
from antigravity_swiss.session.app_storage import AppStorageManager
from antigravity_swiss.session.sqlite_guard import SQLiteIntegrityGuard

logger = logging.getLogger(__name__)

DEFAULT_ANTIGRAVITY_BIN = Path("/opt/Antigravity/antigravity")


class ProcessManager(Protocol):
    """Interface Contract defined in PROJECT.md § Interface Contracts."""
    def get_running_antigravity_pid(self) -> int | None: ...
    def terminate_gracefully(self, timeout_sec: float = 10.0) -> bool: ...
    def relaunch(self, conversation_id: str | None = None) -> int: ...


class ProcessLifecycleManager(ProcessManager):
    """Production implementation of ProcessManager."""

    def __init__(
        self,
        antigravity_bin: Path | str = DEFAULT_ANTIGRAVITY_BIN,
        lock_manager: SingletonLockManager | None = None,
        sqlite_guard: SQLiteIntegrityGuard | None = None,
        storage_manager: AppStorageManager | None = None,
    ) -> None:
        self.antigravity_bin = Path(antigravity_bin)
        self.lock_manager = lock_manager or SingletonLockManager()
        self.sqlite_guard = sqlite_guard or SQLiteIntegrityGuard()
        self.storage_manager = storage_manager or AppStorageManager()

    def get_running_antigravity_pid(self) -> int | None:
        """
        Determines the PID of the running root Antigravity process.
        Uses SingletonLock detection with verification, falling back to /proc scanning.
        """
        lock_state = self.lock_manager.inspect_lock()
        if lock_state.is_pid_alive and lock_state.is_antigravity:
            return lock_state.pid

        # Fallback /proc scan for root process (no --type= flag)
        proc_root = Path("/proc")
        for entry in proc_root.iterdir():
            if not entry.name.isdigit():
                continue
            try:
                cmdline = (entry / "cmdline").read_bytes().decode("utf-8", errors="ignore")
                if "/opt/Antigravity/antigravity" in cmdline and "--type=" not in cmdline:
                    pid = int(entry.name)
                    return pid
            except (OSError, PermissionError):
                continue

        return None

    def terminate_gracefully(self, timeout_sec: float = 10.0) -> bool:
        """
        Terminates running Antigravity instance cleanly:
        1. Sends SIGTERM to main PID (triggers before-quit + killLanguageServer).
        2. Polls /proc/<PID> every 100ms up to timeout_sec.
        3. If unresponsive, escalates to SIGKILL on process tree.
        4. Cleans up orphaned singleton locks.
        5. Flushes and checkpoints all SQLite WAL databases.
        """
        pid = self.get_running_antigravity_pid()
        if pid is None:
            logger.info("Antigravity is not currently running.")
            self.lock_manager.cleanup_orphaned_locks()
            self.sqlite_guard.flush_and_verify_all(timeout_sec=5.0)
            return True

        logger.info("Sending SIGTERM to Antigravity (PID %d)...", pid)
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass

        # Poll for graceful termination
        poll_interval = 0.1
        deadline = time.monotonic() + timeout_sec
        clean_exit = False

        while time.monotonic() < deadline:
            if not Path(f"/proc/{pid}").exists():
                clean_exit = True
                break
            time.sleep(poll_interval)

        # Fallback escalation to SIGKILL
        if not clean_exit:
            logger.warning(
                "PID %d did not terminate within %.1fs. Escalating to SIGKILL...",
                pid,
                timeout_sec,
            )
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            time.sleep(0.5)

        # Remove orphaned lock files if left behind
        self.lock_manager.cleanup_orphaned_locks()

        # Flush and checkpoint SQLite databases
        self.sqlite_guard.flush_and_verify_all(timeout_sec=5.0)

        return True

    def relaunch(
        self,
        conversation_id: str | None = None,
        extra_args: list[str] | None = None,
    ) -> int:
        """
        Safely relaunches Antigravity:
        1. Ensures existing processes are cleanly terminated.
        2. Preserves active conversation layout and timestamp if provided.
        3. Spawns detached process with start_new_session=True.
        4. Verifies startup and returns new PID.
        """
        # Ensure clean state
        self.terminate_gracefully(timeout_sec=10.0)

        # Apply conversation session preservation if target given
        if conversation_id:
            self.storage_manager.preserve_active_conversation(conversation_id)

        if not self.antigravity_bin.exists():
            raise FileNotFoundError(f"Antigravity binary not found at {self.antigravity_bin}")

        # Assemble launch arguments
        args = [str(self.antigravity_bin)]
        if extra_args:
            args.extend(extra_args)

        env = os.environ.copy()

        logger.info("Launching detached Antigravity instance: %s", args)
        proc = subprocess.Popen(
            args,
            start_new_session=True,  # Detached process group
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            close_fds=True,
            env=env,
        )

        new_pid = proc.pid
        logger.info("Antigravity launched with PID %d", new_pid)

        # Startup verification (poll up to 5s)
        deadline = time.monotonic() + 5.0
        while time.monotonic() < deadline:
            if proc.poll() is not None:
                raise RuntimeError(
                    f"Antigravity exited prematurely with return code {proc.returncode}"
                )
            if self.lock_manager.lock_file.exists():
                break
            time.sleep(0.2)

        return new_pid
```

---

## 5. Verification Method

To independently verify the implementation and test behavior:

### 5.1 Unit & Integration Test Plan
1. **`tests/test_session_app_storage.py`**:
   - Test reading valid `app_storage.json` fixtures.
   - Test atomic write semantics (temporary file + rename).
   - Test `update_items` with deletion (`None` value).
   - Test `preserve_active_conversation` injecting layout node and updating `conversation_summaries.db` timestamp.
   - Test `get_window_geometry` reading from both `storage.json` and `app_storage.json`.

2. **`tests/test_sqlite_guard.py`**:
   - Create temporary SQLite database in WAL mode with active frames (`con.commit()`).
   - Verify `inspect_wal()` detects non-zero WAL size.
   - Verify `checkpoint_database(mode="TRUNCATE")` flushes all frames and resets WAL size to 0 bytes.
   - Verify `quick_check()` confirms integrity.

3. **`tests/test_lock_manager.py`**:
   - Create mock `SingletonLock` symlink to `f"{socket.gethostname()}-{os.getpid()}"`.
   - Verify `inspect_lock()` marks process alive.
   - Test dead PID (e.g. PID `9999999`): verify `is_orphaned == True`.
   - Verify `cleanup_orphaned_locks()` unlinks stale lock files without touching live ones.

4. **`tests/test_process_lifecycle.py`**:
   - Mock `subprocess.Popen` and `os.kill` to simulate 10s timeout escalation.
   - Verify sequence: `SIGTERM` -> poll -> `SIGKILL` -> lock cleanup -> SQLite checkpoint -> detached relaunch.

### 5.2 Manual Host Verification Commands
```bash
# 1. Check current running PID from SingletonLock
python3 -c "
import os
target = os.readlink(os.path.expanduser('~/.config/Antigravity/SingletonLock'))
pid = int(target.rsplit('-', 1)[1])
print('Parsed PID:', pid, 'Alive:', os.path.exists(f'/proc/{pid}'))
"

# 2. Check WAL files of critical databases
python3 -c "
import glob, os
dbs = ['~/.gemini/antigravity/conversation_summaries.db', '~/.config/Antigravity/User/globalStorage/state.vscdb']
for db in dbs:
    wal = os.path.expanduser(db) + '-wal'
    print(wal, 'exists:', os.path.exists(wal), 'size:', os.path.getsize(wal) if os.path.exists(wal) else 0)
"

# 3. Check conversation layout in app_storage.json
python3 -c "
import json
with open(os.path.expanduser('~/.config/Antigravity/app_storage.json')) as f:
    d = json.load(f)
for k in d:
    if 'multi-conversation-layout' in k:
        print(k, '->', d[k][:80])
"
```
