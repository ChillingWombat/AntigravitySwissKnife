"""
Unit tests for antigravity_swiss.session: app_storage and sqlite_guard.
"""

import json
import os
from pathlib import Path
import sqlite3
import pytest

from antigravity_swiss.session.app_storage import (
    AppStorageManager,
    ConversationSessionState,
    WindowGeometry,
)
from antigravity_swiss.session.sqlite_guard import (
    CheckpointResult,
    SQLiteIntegrityGuard,
    WALStatus,
)


def test_app_storage_read_and_atomic_write(temp_dir):
    """Verify AppStorageManager reads safely and performs atomic writes."""
    storage_path = Path(temp_dir) / "app_storage.json"
    mgr = AppStorageManager(app_storage_path=storage_path)

    # Empty file reads as empty dict
    assert mgr.read_raw() == {}

    # Write dictionary
    items = {"key1": "val1", "key2": "val2"}
    mgr.write_raw(items)

    assert storage_path.exists()
    assert (storage_path.stat().st_mode & 0o777) == 0o644
    assert mgr.read_raw() == items

    # Update items with deletion
    mgr.update_items({"key1": "updated_val", "key2": None, "key3": "val3"})
    updated = mgr.read_raw()
    assert updated["key1"] == "updated_val"
    assert "key2" not in updated
    assert updated["key3"] == "val3"


def test_preserve_active_conversation_and_sqlite_sync(temp_dir):
    """Verify session preservation updates layout, index, login email, and DB timestamp."""
    app_storage = Path(temp_dir) / "app_storage.json"
    conv_db = Path(temp_dir) / "conversation_summaries.db"

    # Setup mock conversation_summaries.db
    con = sqlite3.connect(conv_db)
    con.execute("CREATE TABLE conversation_summaries (conversation_id TEXT PRIMARY KEY, last_modified_time INTEGER);")
    con.execute("INSERT INTO conversation_summaries VALUES ('convo-123', 1000);")
    con.commit()
    con.close()

    mgr = AppStorageManager(app_storage_path=app_storage, conv_summaries_db=conv_db)
    assert mgr.get_active_conversation_id() == "convo-123"

    # Preserve conversation
    mgr.preserve_active_conversation("convo-123", account_email="switched@example.com")

    # Verify storage contents
    storage_data = mgr.read_raw()
    layout_key = "antigravity-multi-conversation-layout-v3-convo-123"
    assert layout_key in storage_data
    layout_node = json.loads(storage_data[layout_key])
    assert layout_node["rootNode"]["cascadeId"] == "convo-123"
    assert storage_data["antigravity-multi-conversation-layout-v3-index"] == "[]"
    assert storage_data["jetski.onboarding.lastLoginUsername"] == "switched@example.com"

    # Verify timestamp in SQLite was updated
    con = sqlite3.connect(conv_db)
    cur = con.cursor()
    cur.execute("SELECT last_modified_time FROM conversation_summaries WHERE conversation_id = 'convo-123'")
    new_timestamp = cur.fetchone()[0]
    con.close()
    assert new_timestamp > 1000


def test_window_geometry_extraction(temp_dir):
    """Verify WindowGeometry parses dimensions and panel widths."""
    app_storage = Path(temp_dir) / "app_storage.json"
    storage_json = Path(temp_dir) / "storage.json"

    storage_json.write_text(json.dumps({
        "windowsState": {
            "lastActiveWindow": {
                "uiState": {"x": 50, "y": 100, "width": 1400, "height": 900, "mode": 1}
            }
        }
    }), encoding="utf-8")

    app_storage.write_text(json.dumps({
        "auxPaneWidth": "42.5",
        "inTabSidebarWidth": "180"
    }), encoding="utf-8")

    mgr = AppStorageManager(app_storage_path=app_storage, storage_json_path=storage_json)
    geom = mgr.get_window_geometry()
    assert geom.x == 50
    assert geom.y == 100
    assert geom.width == 1400
    assert geom.height == 900
    assert geom.mode == 1
    assert geom.aux_pane_width == 42.5
    assert geom.in_tab_sidebar_width == 180


def test_sqlite_integrity_guard_checkpoint_and_quick_check(temp_dir):
    """Verify SQLiteIntegrityGuard runs TRUNCATE checkpoint and quick_check verification."""
    db_file = Path(temp_dir) / "test_state.vscdb"
    con = sqlite3.connect(db_file)
    con.execute("PRAGMA journal_mode=WAL;")
    con.execute("CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT);")
    con.execute("INSERT INTO kv VALUES ('item1', 'data1');")
    con.commit()
    con.close()

    guard = SQLiteIntegrityGuard(config_dir=temp_dir, gemini_dir=temp_dir)

    wal_stat = guard.inspect_wal(db_file)
    assert wal_stat.db_path == db_file

    ckpt_res = guard.checkpoint_database(db_file, mode="TRUNCATE")
    assert ckpt_res.success is True
    assert ckpt_res.busy == 0
    assert ckpt_res.integrity_ok is True

    # Critical database discovery
    guard.gemini_dir = Path(temp_dir)
    critical = guard.get_critical_databases()
    assert isinstance(critical, list)
