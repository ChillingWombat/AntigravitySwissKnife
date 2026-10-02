"""
Hermetic mock for Antigravity 2.0 Linux filesystem hierarchy.
Reconstructs exact ~/.config/Antigravity and ~/.gemini/antigravity structures,
verifying 36-byte UUID schemas, pbtxt formatting, app_storage.json, and SQLite WAL files.
"""

import json
import os
import sqlite3
import uuid
from typing import Any, Dict, List, Optional


class MockAntigravityFs:
    """Manages an isolated Antigravity filesystem tree within a temporary directory."""

    def __init__(self, root_dir: str):
        self.root_dir = os.path.abspath(root_dir)
        self.home_dir = os.path.join(self.root_dir, "home")
        self.config_antigravity_dir = os.path.join(self.home_dir, ".config", "Antigravity")
        self.gemini_antigravity_dir = os.path.join(self.home_dir, ".gemini", "antigravity")
        self.conversations_dir = os.path.join(self.gemini_antigravity_dir, "conversations")
        self.brain_dir = os.path.join(self.gemini_antigravity_dir, "brain")
        self.global_storage_dir = os.path.join(self.config_antigravity_dir, "User", "globalStorage")
        
        # Identity and active session defaults
        self.active_cascade_id = str(uuid.uuid4())
        self.active_machine_id = str(uuid.uuid4())
        self.active_updater_id = str(uuid.uuid4())
        self.active_installation_id = str(uuid.uuid4())
        self.active_installation_uuid = str(uuid.uuid4())
        self.active_account_email = "primary-user@gmail.com"

    def setup(self) -> None:
        """Creates the full directory hierarchy and initializes baseline files."""
        os.makedirs(self.config_antigravity_dir, exist_ok=True)
        os.makedirs(self.gemini_antigravity_dir, exist_ok=True)
        os.makedirs(self.conversations_dir, exist_ok=True)
        os.makedirs(self.brain_dir, exist_ok=True)
        os.makedirs(self.global_storage_dir, exist_ok=True)

        self.write_machine_id(self.active_machine_id)
        self.write_updater_id(self.active_updater_id)
        self.write_installation_id(self.active_installation_id)
        self.write_pbtxt(self.active_installation_uuid)
        self.write_app_storage(self.active_cascade_id, self.active_account_email)
        self.init_sqlite_state_vscdb()
        self.init_sqlite_conversation_summaries()
        self.create_conversation_data(self.active_cascade_id, title="Active Main Project")

    # --- Fingerprint File Handlers ---

    def get_machine_id_path(self) -> str:
        return os.path.join(self.config_antigravity_dir, "machineid")

    def get_updater_id_path(self) -> str:
        return os.path.join(self.config_antigravity_dir, ".updaterId")

    def get_installation_id_path(self) -> str:
        return os.path.join(self.gemini_antigravity_dir, "installation_id")

    def get_pbtxt_path(self) -> str:
        return os.path.join(self.gemini_antigravity_dir, "antigravity_state.pbtxt")

    def write_machine_id(self, val: str) -> None:
        # Strictly no newline, exactly 36 bytes
        path = self.get_machine_id_path()
        with open(path, "wb") as f:
            f.write(val.strip().encode("ascii"))

    def read_machine_id(self) -> str:
        with open(self.get_machine_id_path(), "rb") as f:
            return f.read().decode("ascii")

    def write_updater_id(self, val: str) -> None:
        path = self.get_updater_id_path()
        with open(path, "wb") as f:
            f.write(val.strip().encode("ascii"))

    def read_updater_id(self) -> str:
        with open(self.get_updater_id_path(), "rb") as f:
            return f.read().decode("ascii")

    def write_installation_id(self, val: str) -> None:
        path = self.get_installation_id_path()
        with open(path, "wb") as f:
            f.write(val.strip().encode("ascii"))

    def read_installation_id(self) -> str:
        with open(self.get_installation_id_path(), "rb") as f:
            return f.read().decode("ascii")

    def write_pbtxt(self, installation_uuid: str, include_onboarding: bool = True) -> None:
        path = self.get_pbtxt_path()
        content = ""
        if include_onboarding:
            content += (
                "post_onboarding: {\n"
                "  completed_steps: POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME\n"
                "  completed_steps: POST_ONBOARDING_STEP_TYPE_USAGE_MODE\n"
                "  completed_steps: POST_ONBOARDING_STEP_TYPE_AGENT_CONFIGURATION\n"
                "  completed_steps: POST_ONBOARDING_STEP_TYPE_ADD_WORKSPACE\n"
                "}\n"
                "seen_nuxs: { uids: 27 uids: 26 }\n"
                "agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED\n"
                "last_selected_agent_model: MODEL_PLACEHOLDER_M318\n"
                "migrate_convos_into_projects: MIGRATION_STATUS_COMPLETED\n"
            )
        content += f'installation_uuid: "{installation_uuid}"\n'
        content += (
            "migrate_retroactive_projects: RETROACTIVE_MIGRATION_STATUS_COMPLETED_UNNECESSARY\n"
            "migrations: { key: 2 value: MIGRATION_STATUS_COMPLETED }\n"
        )
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)

    def read_pbtxt(self) -> str:
        with open(self.get_pbtxt_path(), "r", encoding="utf-8") as f:
            return f.read()

    # --- Session & app_storage.json Handlers ---

    def get_app_storage_path(self) -> str:
        return os.path.join(self.config_antigravity_dir, "app_storage.json")

    def write_app_storage(self, cascade_id: str, email: str, focused_pane: str = "pane-1") -> None:
        path = self.get_app_storage_path()
        layout_obj = {
            "rootNode": {
                "type": "pane",
                "id": focused_pane,
                "cascadeId": cascade_id
            },
            "focusedPaneId": focused_pane
        }
        aux_pane_obj = {
            "conversationPanes": {
                cascade_id: {
                    "tabs": [
                        {"id": f"artifact__{cascade_id}", "content": {"type": "artifactView"}},
                        {"id": f"file__{cascade_id}", "content": {"type": "fileView"}}
                    ],
                    "activeTabId": f"artifact__{cascade_id}",
                    "isPaneOpen": True
                }
            }
        }
        data = {
            f"antigravity-multi-conversation-layout-v3-{cascade_id}": json.dumps(layout_obj),
            "antigravity-multi-conversation-layout-v3-index": json.dumps([cascade_id]),
            "aux-pane-session": json.dumps(aux_pane_obj),
            "aux-pane-v2-session": json.dumps(aux_pane_obj),
            "jetski.onboarding.lastLoginUsername": email,
            "new-convo-last-selected-project": str(uuid.uuid4())
        }
        with open(path, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=2)

    def read_app_storage(self) -> Dict[str, Any]:
        with open(self.get_app_storage_path(), "r", encoding="utf-8") as f:
            return json.load(f)

    # --- SQLite Database Initializers ---

    def get_state_vscdb_path(self) -> str:
        return os.path.join(self.global_storage_dir, "state.vscdb")

    def init_sqlite_state_vscdb(self) -> None:
        db_path = self.get_state_vscdb_path()
        conn = sqlite3.connect(db_path)
        cur = conn.cursor()
        cur.execute("PRAGMA journal_mode = WAL;")
        cur.execute("CREATE TABLE IF NOT EXISTS ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB);")
        cur.execute("INSERT OR REPLACE INTO ItemTable VALUES ('telemetry.machineId', X'616263');")
        conn.commit()
        conn.close()

    def get_conversation_summaries_path(self) -> str:
        return os.path.join(self.gemini_antigravity_dir, "conversation_summaries.db")

    def init_sqlite_conversation_summaries(self) -> None:
        db_path = self.get_conversation_summaries_path()
        conn = sqlite3.connect(db_path)
        cur = conn.cursor()
        cur.execute("PRAGMA journal_mode = WAL;")
        cur.execute("""
            CREATE TABLE IF NOT EXISTS conversation_summaries (
                conversation_id TEXT PRIMARY KEY,
                title TEXT,
                preview TEXT,
                step_count INTEGER,
                last_modified_time INTEGER,
                workspace_uris TEXT,
                status TEXT,
                source TEXT,
                project_id TEXT,
                agent_name TEXT,
                parent_conversation_id TEXT,
                nesting_depth INTEGER,
                battle_id TEXT,
                winning_conversation_id TEXT,
                not_fully_idle INTEGER,
                killed INTEGER,
                last_user_input_time INTEGER,
                last_user_input_step_index INTEGER,
                app_data_dir TEXT,
                raw_summary TEXT,
                group_id TEXT
            );
        """)
        conn.commit()
        conn.close()

    def create_conversation_data(self, cascade_id: str, title: str = "Test Task", size_kb: int = 10) -> None:
        """Populates conversations/<cascadeId>.db, entry in summaries, and brain directory."""
        # 1. Update conversation_summaries.db
        conn = sqlite3.connect(self.get_conversation_summaries_path())
        cur = conn.cursor()
        import time
        now = int(time.time() * 1000)
        cur.execute(
            """INSERT OR REPLACE INTO conversation_summaries 
               (conversation_id, title, preview, step_count, last_modified_time, status)
               VALUES (?, ?, ?, ?, ?, ?)""",
            (cascade_id, title, f"Preview for {title}", 10, now, "COMPLETED")
        )
        conn.commit()
        conn.close()

        # 2. Create conversations/<cascadeId>.db
        convo_db_path = os.path.join(self.conversations_dir, f"{cascade_id}.db")
        cconn = sqlite3.connect(convo_db_path)
        ccur = cconn.cursor()
        ccur.execute("CREATE TABLE IF NOT EXISTS trajectory_meta (trajectory_id TEXT, cascade_id TEXT);")
        ccur.execute("CREATE TABLE IF NOT EXISTS steps (idx INTEGER, step_type TEXT, payload BLOB);")
        dummy_data = b"x" * (size_kb * 1024)
        ccur.execute("INSERT INTO steps VALUES (1, 'user_input', ?);", (dummy_data,))
        cconn.commit()
        cconn.close()

        # 3. Create brain/<cascadeId>/ scratch and logs
        task_brain_dir = os.path.join(self.brain_dir, cascade_id)
        scratch_dir = os.path.join(task_brain_dir, "scratch")
        steps_dir = os.path.join(task_brain_dir, ".system_generated", "steps")
        os.makedirs(scratch_dir, exist_ok=True)
        os.makedirs(steps_dir, exist_ok=True)

        with open(os.path.join(scratch_dir, "scratchpad.txt"), "w") as f:
            f.write("Scratch content " * 100)
        with open(os.path.join(steps_dir, "output.txt"), "w") as f:
            f.write("Step output log " * 50)
        with open(os.path.join(task_brain_dir, "screenshot.png"), "wb") as f:
            f.write(b"\x89PNG\r\n\x1a\n" + b"\x00" * 500)
