"""
SQLite WAL Inspector and TRUNCATE Checkpoint Integrity Runner.
==============================================================
Verifies zero dirty WAL pages and database integrity before Antigravity process relaunch.
"""

from __future__ import annotations

import logging
import os
import sqlite3
import time
from dataclasses import dataclass
from pathlib import Path

from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_ANTIGRAVITY_DATA_DIR,
)

logger = logging.getLogger(__name__)

DEFAULT_CONFIG_DIR = DEFAULT_ANTIGRAVITY_CONFIG_DIR
DEFAULT_GEMINI_DIR = DEFAULT_ANTIGRAVITY_DATA_DIR


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
        self.config_dir = Path(config_dir).expanduser().resolve()
        self.gemini_dir = Path(gemini_dir).expanduser().resolve()

    def get_critical_databases(self) -> list[Path]:
        """Returns all critical SQLite databases requiring flush verification."""
        is_testing = bool(os.environ.get("PYTEST_CURRENT_TEST") or os.environ.get("ANTIGRAVITY_SWISS_TESTING"))
        real_config = Path(DEFAULT_CONFIG_DIR).expanduser().resolve()
        real_gemini = Path(DEFAULT_GEMINI_DIR).expanduser().resolve()
        if is_testing and (self.config_dir == real_config or self.gemini_dir == real_gemini):
            logger.info("SAFETY SHIELD: Skipping SQLite database discovery on real host directory during test mode.")
            return []

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

        # 4. Active conversation databases (any with active .db-wal files or present in conversations/)
        convs_dir = self.gemini_dir / "conversations"
        if convs_dir.exists():
            for wal_file in convs_dir.glob("*.db-wal"):
                base_name = wal_file.name[:-4]  # strip -wal
                base_db = convs_dir / base_name
                if base_db.exists() and base_db not in dbs:
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
            row = cur.fetchone()
            busy = row[0] if row else 0
            log_frames = row[1] if row and len(row) > 1 else 0
            ckpt_frames = row[2] if row and len(row) > 2 else 0

            # Execute integrity check
            cur.execute("PRAGMA quick_check;")
            integrity_row = cur.fetchone()
            integrity_str = integrity_row[0] if integrity_row else "unknown"
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
