"""
Safe Parsing, Atomic Replacement, and Session Preservation for app_storage.json.
===============================================================================
Preserves active conversation cascadeId, layout nodes, aux-pane-session tabs,
and jetski.onboarding.lastLoginUsername across Antigravity application relaunches.
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

from antigravity_swiss.core.constants import (
    APP_STORAGE_JSON_NAME,
    CONVERSATION_SUMMARIES_DB_NAME,
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_ANTIGRAVITY_DATA_DIR,
)

logger = logging.getLogger(__name__)

DEFAULT_APP_STORAGE = DEFAULT_ANTIGRAVITY_CONFIG_DIR / APP_STORAGE_JSON_NAME
DEFAULT_STORAGE_JSON = DEFAULT_ANTIGRAVITY_CONFIG_DIR / "User" / "globalStorage" / "storage.json"
DEFAULT_CONV_SUMMARIES_DB = DEFAULT_ANTIGRAVITY_DATA_DIR / CONVERSATION_SUMMARIES_DB_NAME

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
        self.app_storage_path = Path(app_storage_path).expanduser().resolve()
        self.storage_json_path = Path(storage_json_path).expanduser().resolve()
        self.conv_summaries_db = Path(conv_summaries_db).expanduser().resolve()

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
            try:
                os.chmod(self.app_storage_path, 0o644)
            except OSError:
                pass
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
        is_testing = bool(os.environ.get("ANTIGRAVITY_SWISS_TESTING") or os.environ.get("PYTEST_CURRENT_TEST"))
        is_host_db = self.conv_summaries_db == DEFAULT_CONV_SUMMARIES_DB.resolve()

        if self.conv_summaries_db.exists() and not (is_testing and is_host_db):
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

    def get_active_cascade_id(self) -> str | None:
        """Alias for get_active_conversation_id."""
        return self.get_active_conversation_id()

    def get_pinned_conversation_ids(self) -> list[str]:
        """Reads pinned conversation IDs from app_storage.json (pinned_conversations_order)."""
        storage = self.read_raw()
        raw = storage.get("pinned_conversations_order")
        if not raw:
            return []
        if isinstance(raw, list):
            return [str(x) for x in raw]
        if isinstance(raw, str):
            try:
                parsed = json.loads(raw)
                if isinstance(parsed, list):
                    return [str(x) for x in parsed]
            except Exception:
                pass
        return []

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

        # 4. Populate and preserve aux pane session tabs
        for key in (AUX_PANE_KEY, AUX_PANE_V2_KEY):
            aux_data: dict[str, Any] = {"conversationPanes": {}}
            if key in storage:
                try:
                    loaded = json.loads(storage[key])
                    if isinstance(loaded, dict):
                        aux_data = loaded
                except (json.JSONDecodeError, TypeError):
                    pass
            panes = aux_data.setdefault("conversationPanes", {})
            if cascade_id not in panes or not isinstance(panes.get(cascade_id), dict):
                panes[cascade_id] = {
                    "tabs": [
                        {"id": f"artifact__{cascade_id}", "content": {"type": "artifactView"}},
                        {"id": f"file__{cascade_id}", "content": {"type": "fileView"}},
                    ],
                    "activeTabId": f"artifact__{cascade_id}",
                    "isPaneOpen": True,
                }
            storage[key] = json.dumps(aux_data)

        self.write_raw(storage)

        # 5. Touch last_modified_time in conversation_summaries.db
        self.touch_conversation_summary(cascade_id)

    def touch_conversation_summary(self, cascade_id: str) -> bool:
        """Updates last_modified_time for cascade_id to current UTC timestamp."""
        if not self.conv_summaries_db.exists():
            return False
        try:
            con = sqlite3.connect(self.conv_summaries_db, timeout=5.0)
            now_epoch = int(datetime.now(timezone.utc).timestamp() * 1000)
            cur = con.cursor()
            cur.execute(
                "UPDATE conversation_summaries SET last_modified_time = ? "
                "WHERE conversation_id = ?",
                (now_epoch, cascade_id),
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
