# M3 Brain Cache Inspector & Pruner Architecture Blueprint
**Features**: F12 (`F12_BRAIN_CACHE_INSPECTOR`) & F13 (`F13_BRAIN_CACHE_PRUNER`)  
**Working Folder**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m3_2/`  
**Date**: 2026-10-02  

---

## 1. Observation

### 1.1 Real Host Filesystem Inspection (`~/.gemini/antigravity/` and `~/.config/Antigravity/`)
Direct measurements from host directories revealed a massive cache footprint totaling over **6.7 GB**:

```bash
$ du -sh ~/.gemini/antigravity/brain ~/.gemini/antigravity/conversations ~/.gemini/antigravity/conversation_summaries.db* ~/.config/Antigravity/app_storage.json
4.1G    /home/david/.gemini/antigravity/brain
2.6G    /home/david/.gemini/antigravity/conversations
496K    /home/david/.gemini/antigravity/conversation_summaries.db
32K     /home/david/.gemini/antigravity/conversation_summaries.db-shm
4.0M    /home/david/.gemini/antigravity/conversation_summaries.db-wal
72K     /home/david/.config/Antigravity/app_storage.json
```

Detailed file categorization of `~/.gemini/antigravity/brain/` (361 directories):
- **Screenshots / Media**: **1,718.17 MB** (2,799 files) — Stored in per-conversation `<conv_id>/.user_uploaded/media_*.png` and global `brain/tempmediaStorage/media_*.png`.
- **Task Scratchpads**: **905.97 MB** (39,202 files) — Stored in per-conversation `<conv_id>/scratch/*` and global `~/.gemini/antigravity/scratch/`.
- **Conversation Transcripts**: **831.03 MB** (5,307 files) — Stored in `<conv_id>/.system_generated/logs/transcript.jsonl`, `transcript_full.jsonl`, and `chunks/`.
- **Tool Execution Logs / Tasks**: **186.37 MB** (1,999 files) — Stored in `<conv_id>/.system_generated/tasks/task-*.log`.
- **Step Outputs**: **79.65 MB** (30,056 files) — Stored in `<conv_id>/.system_generated/steps/<step_id>/output.txt`.
- **Other Brain Files**: **29.97 MB** (3,924 files) — Stored in `<conv_id>/.system_generated/messages/*` and miscellaneous checkpoints.

Detailed inspection of `~/.gemini/antigravity/conversations/`:
- Contains **430 SQLite files** (`<conversation_id>.db`, `<conversation_id>.db-wal`, `<conversation_id>.db-shm`).
- Several individual conversation databases exceed 90 MB and 180 MB (e.g. `19c06e44-26ed-40f9-8262-565d0a6b3e60.db` is 181.8 MB).
- Table schema in per-conversation databases: `trajectory_meta`, `steps`, `gen_metadata`, `executor_metadata`, `parent_references`, `trajectory_metadata_blob`, `battle_mode_infos`.
- SQLite freelist inspection shows unused pages that can be compacted via `VACUUM;`.

Master Conversation Summary Database (`~/.gemini/antigravity/conversation_summaries.db`):
- Single master table `conversation_summaries` (360 rows).
- Key columns: `conversation_id`, `title`, `preview`, `step_count`, `last_modified_time`, `workspace_uris`, `status`, `last_user_input_time`.

### 1.2 Session Layout & Pinned Sessions in `app_storage.json`
Direct inspection of `/home/david/.config/Antigravity/app_storage.json`:
- Conversation layouts are keyed as `antigravity-multi-conversation-layout-v3-<cascadeId>`.
- Open pane layouts: `aux-pane-session` and `aux-pane-v2-session`.
- Pinned sessions: stored in key `pinned_conversations_order` as a JSON array of conversation IDs (e.g., `[]` or `["<conv_id_1>", ...]`).

### 1.3 Codebase Inspection & Naming Gaps in `antigravity_swiss/`
1. `antigravity_swiss/cache_optimizer/inspector.py`:
   - Line 47: Calls `self.app_storage.get_active_cascade_id()`.
   - In `antigravity_swiss/session/app_storage.py` (line 132), the method is named `get_active_conversation_id()`. Because line 48 had `except Exception: return None`, calling `get_active_cascade_id()` silently fails and returns `None` unless passed explicitly.
2. `antigravity_swiss/cache_optimizer/models.py`:
   - Lacks the explicit `CacheCategory` enum requested by the dispatch requirement.
   - Lacks the `CacheItem` dataclass requested by the dispatch requirement.
   - `CacheBreakdown` lacks `oldest_timestamp`, `newest_timestamp`, and `item_count`.
   - `PruneResult` lacks `bytes_freed`, `files_deleted`, `categories_affected`, and `elapsed_seconds` (currently uses `reclaimed_bytes`, `pruned_files_count`, `pruned_directories_count`).
3. Class naming:
   - Current modules name them `CacheInspector` and `CachePruner`.
   - Requirement specifies `BrainCacheInspector` and `BrainCachePruner`.
   - We must provide `BrainCacheInspector` and `BrainCachePruner` as the primary classes with `CacheInspector = BrainCacheInspector` and `CachePruner = BrainCachePruner` aliases for 100% backwards compatibility.
4. IPC Interface Compliance (`PROJECT.md § Interface Contracts` item 4):
   - Socket server (`socket_server.py`) has `register_quota_handlers` but lacks `register_cache_handlers`.
   - `SwissKnifeController` in `controller.py` lacks `get_cache_breakdown()` and `prune_cache()`.

---

## 2. Logic Chain

```
[Observation 1.1: 6.7GB cache with 1.7GB screenshots, 906MB scratch, 831MB transcripts, 2.6GB SQLite DBs]
    │
    ▼
[Logic Step 1: Brain cache bloat is heavily concentrated in media screenshots, task scratchpads, tool execution outputs, and unvacuumed SQLite databases]
    │
    ▼
[Observation 1.2: Pinned sessions are in pinned_conversations_order; active session is cascadeId]
    │
    ▼
[Logic Step 2: Safe retention policy must unconditionally protect:
   a) Active conversation (cascadeId)
   b) Pinned conversations (pinned_conversations_order)
   c) Permanent conversation transcripts (transcript.jsonl / transcript_full.jsonl)
   d) Host data protection shield when ANTIGRAVITY_SWISS_TESTING=1]
    │
    ▼
[Observation 1.3: Discrepancy between get_active_cascade_id and get_active_conversation_id, plus missing CacheCategory enum, CacheItem, oldest/newest timestamps, bytes_freed, categories_affected, elapsed_seconds, and IPC methods]
    │
    ▼
[Logic Step 3: Upgrade models.py, inspector.py, pruner.py to full specification while maintaining dual-name backward compatibility for existing tests and UI]
    │
    ▼
[Logic Step 4: Wire cache.get_breakdown and cache.prune to socket_server.py and controller.py per PROJECT.md § Interface Contracts]
```

---

## 3. Caveats

1. **Host Data Protection**: During automated testing, `ANTIGRAVITY_SWISS_TESTING=1` must strictly prevent any pruning or vacuuming of the real host user directories (`/home/david/.gemini/antigravity` and `/home/david/.config/Antigravity`). The pruner must enforce `dry_run = True` if the target paths match host defaults.
2. **SQLite Locking During VACUUM**: Running `VACUUM` on active conversation databases will fail or block if the Antigravity application has an exclusive lock. Therefore, `BrainCachePruner` must skip active databases and wrap `VACUUM` operations in `try/except (sqlite3.OperationalError, sqlite3.DatabaseError)` with bounded timeouts.
3. **Permanent Conversation Records**: Transcripts (`.system_generated/logs/transcript*.jsonl`) represent the user's historical prompt and response history. They should NEVER be deleted by standard stale cache pruning, even for inactive conversations. Only transient artifacts (scratch, steps, task logs, unreferenced temp media) should be pruned.
4. **No Host Process Signals**: Consistent with project safety constraints, cache inspection and pruning never scans `/proc` or issues POSIX signals to running host processes.

---

## 4. Conclusion & Complete Implementation Architecture

### 4.1 Architecture of `antigravity_swiss/cache_optimizer/`

```
antigravity_swiss/cache_optimizer/
├── __init__.py           # Exports BrainCacheInspector, BrainCachePruner, CacheCategory, CacheItem, CacheBreakdown, PruneResult
├── models.py             # CacheCategory enum, CacheItem, CacheCategoryUsage, ConversationCacheSummary, CacheBreakdown, PruneOptions, PruneResult
├── inspector.py          # BrainCacheInspector (and CacheInspector alias)
├── pruner.py             # BrainCachePruner (and CachePruner alias)
└── prompt_cache.py       # PromptCacheOptimizer & TokenBloatReport (F14)
```

---

### 4.2 Exact Specification for `models.py`

```python
from __future__ import annotations
from dataclasses import dataclass, field
import datetime
from enum import Enum
from pathlib import Path
from typing import Any, Dict, List, Optional, Set

class CacheCategory(str, Enum):
    """Enumeration of cache item categories across brain/ and conversations/."""
    SCREENSHOTS = "screenshots"      # .user_uploaded/*, tempmediaStorage/*, media_*.png
    SCRATCHPADS = "scratchpads"      # scratch/* temporary task scripts & files
    TOOL_LOGS = "tool_logs"          # .system_generated/tasks/task-*.log
    STEP_OUTPUTS = "step_outputs"    # .system_generated/steps/*/output.txt
    TRANSCRIPTS = "transcripts"      # .system_generated/logs/transcript*.jsonl
    DATABASES = "databases"          # conversations/*.db, conversation_summaries.db
    OTHER = "other"                  # .system_generated/messages/*, checkpoints, etc.

@dataclass
class CacheItem:
    """Represents a discrete inspectable file or resource within the cache."""
    path: Path
    category: CacheCategory
    size_bytes: int
    mtime: datetime.datetime
    conversation_id: Optional[str] = None
    is_active: bool = False
    is_pinned: bool = False
    is_prunable: bool = False

@dataclass
class CacheCategoryUsage:
    """Usage statistics for a specific category."""
    category: str
    total_bytes: int
    file_count: int
    reclaimable_bytes: int = 0
    oldest_timestamp: Optional[datetime.datetime] = None
    newest_timestamp: Optional[datetime.datetime] = None
    description: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return {
            "category": self.category,
            "total_bytes": self.total_bytes,
            "total_mb": round(self.total_bytes / (1024 * 1024), 2),
            "file_count": self.file_count,
            "reclaimable_bytes": self.reclaimable_bytes,
            "reclaimable_mb": round(self.reclaimable_bytes / (1024 * 1024), 2),
            "oldest_timestamp": self.oldest_timestamp.isoformat() if self.oldest_timestamp else None,
            "newest_timestamp": self.newest_timestamp.isoformat() if self.newest_timestamp else None,
            "description": self.description,
        }

@dataclass
class ConversationCacheSummary:
    """Storage footprint of a single conversation brain directory and DB."""
    conversation_id: str
    last_modified: datetime.datetime
    brain_bytes: int
    db_bytes: int
    is_active: bool = False
    is_pinned: bool = False
    reclaimable_bytes: int = 0

@dataclass
class CacheBreakdown:
    """System-wide storage and cache breakdown for Antigravity."""
    brain_total_bytes: int
    conversations_total_bytes: int
    active_session_bytes: int
    reclaimable_bytes: int
    conversation_count: int
    item_count: int = 0
    oldest_timestamp: Optional[datetime.datetime] = None
    newest_timestamp: Optional[datetime.datetime] = None
    categories: Dict[str, CacheCategoryUsage] = field(default_factory=dict)
    conversations: List[ConversationCacheSummary] = field(default_factory=list)

    @property
    def total_bytes(self) -> int:
        return self.brain_total_bytes + self.conversations_total_bytes

    def to_dict(self) -> Dict[str, Any]:
        return {
            "total_bytes": self.total_bytes,
            "total_mb": round(self.total_bytes / (1024 * 1024), 2),
            "brain_total_bytes": self.brain_total_bytes,
            "brain_total_mb": round(self.brain_total_bytes / (1024 * 1024), 2),
            "conversations_total_bytes": self.conversations_total_bytes,
            "conversations_total_mb": round(self.conversations_total_bytes / (1024 * 1024), 2),
            "active_session_bytes": self.active_session_bytes,
            "active_session_mb": round(self.active_session_bytes / (1024 * 1024), 2),
            "reclaimable_bytes": self.reclaimable_bytes,
            "reclaimable_mb": round(self.reclaimable_bytes / (1024 * 1024), 2),
            "conversation_count": self.conversation_count,
            "item_count": self.item_count,
            "oldest_timestamp": self.oldest_timestamp.isoformat() if self.oldest_timestamp else None,
            "newest_timestamp": self.newest_timestamp.isoformat() if self.newest_timestamp else None,
            "categories": {k: v.to_dict() for k, v in self.categories.items()},
            "conversations": [
                {
                    "conversation_id": c.conversation_id,
                    "last_modified": c.last_modified.isoformat(),
                    "brain_bytes": c.brain_bytes,
                    "db_bytes": c.db_bytes,
                    "is_active": c.is_active,
                    "is_pinned": c.is_pinned,
                    "reclaimable_bytes": c.reclaimable_bytes,
                }
                for c in self.conversations[:100]
            ],
        }

@dataclass
class PruneOptions:
    """Configuration options for cleaning stale cache."""
    prune_tasks: bool = True
    prune_screenshots: bool = True
    prune_scratch: bool = True
    prune_steps: bool = True
    vacuum_databases: bool = True
    min_age_days: float = 3.0
    dry_run: bool = False
    target_categories: Optional[List[CacheCategory]] = None

@dataclass
class PruneResult:
    """Outcome of a cache pruning operation conforming to PROJECT.md."""
    bytes_freed: int
    files_deleted: int
    categories_affected: List[str]
    elapsed_seconds: float
    protected_active_id: Optional[str] = None
    protected_pinned_ids: List[str] = field(default_factory=list)
    dry_run: bool = False
    databases_vacuumed: int = 0
    details: List[str] = field(default_factory=list)

    # Backwards compatibility properties
    @property
    def reclaimed_bytes(self) -> int:
        return self.bytes_freed

    @property
    def pruned_files_count(self) -> int:
        return self.files_deleted

    @property
    def pruned_directories_count(self) -> int:
        return len([d for d in self.details if "directory" in d.lower() or "scratch" in d.lower()])

    def to_dict(self) -> Dict[str, Any]:
        return {
            "bytes_freed": self.bytes_freed,
            "bytes_freed_mb": round(self.bytes_freed / (1024 * 1024), 2),
            "reclaimed_bytes": self.bytes_freed,
            "files_deleted": self.files_deleted,
            "pruned_files_count": self.files_deleted,
            "categories_affected": self.categories_affected,
            "elapsed_seconds": round(self.elapsed_seconds, 3),
            "protected_active_id": self.protected_active_id,
            "protected_pinned_ids": self.protected_pinned_ids,
            "dry_run": self.dry_run,
            "databases_vacuumed": self.databases_vacuumed,
            "details": self.details[:50],
        }
```

---

### 4.3 Exact Specification for `inspector.py` (`BrainCacheInspector`)

Key responsibilities:
1. **Active & Pinned Discovery**:
   - Reads active `cascadeId` via `app_storage.get_active_conversation_id()`.
   - Reads `pinned_conversations_order` list from `app_storage.json`.
   - Returns `set[str]` of protected session IDs.
2. **Deep Scanning**:
   - `brain/`: Iterates over conversation directories and `tempmediaStorage`.
   - Categorizes each file according to path patterns:
     * `media_*.png`, `.user_uploaded`, `tempmediaStorage`, images -> `CacheCategory.SCREENSHOTS`
     * `scratch/` -> `CacheCategory.SCRATCHPADS`
     * `.system_generated/tasks/`, `task-*.log` -> `CacheCategory.TOOL_LOGS`
     * `.system_generated/steps/` -> `CacheCategory.STEP_OUTPUTS`
     * `.system_generated/logs/` -> `CacheCategory.TRANSCRIPTS`
     * Other -> `CacheCategory.OTHER`
   - `conversations/` & `conversation_summaries.db`:
     * `.db`, `.db-wal`, `.db-shm` -> `CacheCategory.DATABASES`
3. **Metrics Aggregation**:
   - Tracks `oldest_timestamp`, `newest_timestamp`, and `item_count` globally and per-category.
   - Calculates `reclaimable_bytes`: non-active, non-pinned scratch, steps, tasks, screenshots older than cutoff (or all non-protected prunable categories).
   - Generates `CacheBreakdown` with backwards-compatible key mappings (`steps`, `scratch`, `tasks`, `logs`, `messages`, `artifacts`, `databases`, `screenshots`, `step_outputs`, `tool_logs`, `scratchpads`, `transcripts`).

---

### 4.4 Exact Specification for `pruner.py` (`BrainCachePruner`)

Key responsibilities:
1. **Safety Shield (`_is_host_environment_protected`)**:
   - If `ANTIGRAVITY_SWISS_TESTING=1` or `PYTEST_CURRENT_TEST` is set AND `data_dir` or `config_dir` matches real host paths (`~/.gemini/antigravity` or `~/.config/Antigravity`), dry run is unconditionally forced (`dry_run = True`).
2. **Unconditional Safe Retention**:
   - NEVER prune active `cascadeId` conversation.
   - NEVER prune pinned conversations from `pinned_conversations_order`.
   - NEVER prune conversation transcripts (`transcript.jsonl`, `transcript_full.jsonl`).
3. **Pruning Execution**:
   - `opts.prune_screenshots`: Unlink `.user_uploaded/*.png`, `tempmediaStorage/*.png`, scratch media images older than cutoff.
   - `opts.prune_scratch`: Remove `scratch/` folders and files in stale conversations older than cutoff.
   - `opts.prune_steps`: Remove `.system_generated/steps/` directories and logs older than cutoff.
   - `opts.prune_tasks`: Remove `.system_generated/tasks/*.log` older than cutoff.
   - `opts.vacuum_databases`:
     * For eligible SQLite databases (`conversations/*.db` of non-active conversations, `conversation_summaries.db`):
     * If dry run: query `PRAGMA freelist_count;` * `PRAGMA page_size;` and add to estimated freed bytes.
     * If live prune:
       ```python
       sz_before = db_path.stat().st_size
       con = sqlite3.connect(db_path, timeout=5.0)
       try:
           con.execute("PRAGMA wal_checkpoint(TRUNCATE);")
           con.execute("VACUUM;")
           con.commit()
       finally:
           con.close()
       sz_after = db_path.stat().st_size
       freed = max(0, sz_before - sz_after)
       bytes_freed += freed
       ```
4. **Returns `PruneResult`**:
   - `bytes_freed`, `files_deleted`, `categories_affected`, `elapsed_seconds`, `protected_active_id`, `protected_pinned_ids`, `dry_run`, `databases_vacuumed`, `details`.

---

### 4.5 Exact Specification for IPC Integration

#### 1. In `antigravity_swiss/ipc/socket_server.py`:
Add method `register_cache_handlers`:
```python
def register_cache_handlers(self, inspector: Any, pruner: Any) -> None:
    """Wire RPC methods: cache.get_breakdown, cache.prune."""
    @self.register("cache.get_breakdown")
    async def rpc_cache_breakdown() -> dict[str, Any]:
        breakdown = await asyncio.to_thread(inspector.scan_breakdown)
        return breakdown.to_dict()

    @self.register("cache.prune")
    async def rpc_cache_prune(options: dict[str, Any] | None = None) -> dict[str, Any]:
        from antigravity_swiss.cache_optimizer.models import PruneOptions
        opts = PruneOptions(**(options or {}))
        result = await asyncio.to_thread(pruner.prune, options=opts)
        return result.to_dict()
```

#### 2. In `antigravity_swiss/ipc/controller.py`:
Add methods to `SwissKnifeController`:
```python
@abstractmethod
def get_cache_breakdown(self) -> dict[str, Any]:
    """Retrieve categorized cache breakdown and reclaimable estimates."""
    ...

@abstractmethod
def prune_cache(self, options: dict[str, Any] | None = None) -> dict[str, Any]:
    """Execute cache pruning and return PruneResult dictionary."""
    ...
```
Implement in `RemoteDaemonController`:
```python
def get_cache_breakdown(self) -> dict[str, Any]:
    return self._client.call("cache.get_breakdown")

def prune_cache(self, options: dict[str, Any] | None = None) -> dict[str, Any]:
    return self._client.call("cache.prune", {"options": options or {}})
```
Implement in `StandaloneController`:
```python
def get_cache_breakdown(self) -> dict[str, Any]:
    from antigravity_swiss.cache_optimizer.inspector import BrainCacheInspector
    inspector = BrainCacheInspector(
        data_dir=self.config.antigravity_data_dir,
        config_dir=self.config.antigravity_config_dir,
    )
    return inspector.scan_breakdown().to_dict()

def prune_cache(self, options: dict[str, Any] | None = None) -> dict[str, Any]:
    from antigravity_swiss.cache_optimizer.inspector import BrainCacheInspector
    from antigravity_swiss.cache_optimizer.models import PruneOptions
    from antigravity_swiss.cache_optimizer.pruner import BrainCachePruner
    pruner = BrainCachePruner(
        data_dir=self.config.antigravity_data_dir,
        config_dir=self.config.antigravity_config_dir,
    )
    opts = PruneOptions(**(options or {}))
    return pruner.prune(options=opts).to_dict()
```

---

## 5. Verification Method

### 5.1 Test Strategy & Test Suite Requirements
Test file: `tests/unit/test_cache_optimizer.py`
The test suite must cover:
1. **Model Validation**:
   - `CacheCategory` enum values (`screenshots`, `scratchpads`, `tool_logs`, `step_outputs`, `transcripts`, `databases`, `other`).
   - `CacheItem` initialization and attribute types.
   - `CacheBreakdown` serialization with `oldest_timestamp`, `newest_timestamp`, `item_count`, and category mappings.
   - `PruneResult` properties (`bytes_freed`, `files_deleted`, `categories_affected`, `elapsed_seconds`, backwards-compatible `reclaimed_bytes`, `pruned_files_count`).
2. **Deep Inspection (`BrainCacheInspector`)**:
   - Categorization of mock screenshots (`.user_uploaded/media_*.png`, `tempmediaStorage/media_*.png`).
   - Categorization of task scratchpads (`scratch/*`).
   - Categorization of tool execution logs (`.system_generated/tasks/task-*.log`).
   - Categorization of step outputs (`.system_generated/steps/*/output.txt`).
   - Categorization of conversation transcripts (`.system_generated/logs/transcript.jsonl`).
   - Categorization of SQLite databases (`conversations/*.db`, `conversation_summaries.db`).
   - Detection of active conversation via `app_storage.json` layout / `cascadeId`.
   - Detection of pinned conversations via `pinned_conversations_order` in `app_storage.json`.
   - Computation of `reclaimable_bytes` for non-active, non-pinned items.
3. **Safe Pruning (`BrainCachePruner`)**:
   - **Dry run**: files remain intact, `bytes_freed` calculated, `dry_run=True`.
   - **Active conversation shield**: active conversation's scratch, step outputs, screenshots, and logs remain completely intact.
   - **Pinned session shield**: pinned conversation's files remain completely intact.
   - **Permanent transcripts preservation**: transcripts in stale conversations are never deleted.
   - **Stale cache removal**: scratch, steps, tasks, and screenshots older than cutoff are unlinked.
   - **SQLite VACUUM**: mock SQLite database with dummy tables and deleted rows successfully compacted via `VACUUM;` with reduction in file size counted towards `bytes_freed` and `databases_vacuumed`.
   - **Host Data Shield**: if configured pointing to default host directories under `ANTIGRAVITY_SWISS_TESTING=1`, pruner forces dry run and touches no files.
4. **IPC Integration Verification**:
   - Test JSON-RPC execution of `cache.get_breakdown` and `cache.prune`.
   - Test standalone controller methods `get_cache_breakdown` and `prune_cache`.

### 5.2 Verification Commands
```bash
ANTIGRAVITY_SWISS_TESTING=1 pytest tests/unit/test_cache_optimizer.py -v
ANTIGRAVITY_SWISS_TESTING=1 pytest tests/unit/ -q
```

### 5.3 Invalidation Conditions
- Any test fails or throws `AttributeError: 'AppStorageManager' object has no attribute 'get_active_cascade_id'`.
- Real host files in `/home/david/.gemini/antigravity` are deleted or mutated during test runs.
- Transcripts or active conversation artifacts are pruned during a prune operation.
- IPC methods `cache.get_breakdown` or `cache.prune` return error status or malformed payloads.

---
**Explorer Verdict**: Investigation complete, architectural blueprint verified against live filesystem layout, ready for Implementer.
