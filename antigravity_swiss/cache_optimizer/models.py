"""
Cache Optimizer Models & Dataclasses (Requirements R5, Features F12, F13, F14).
================================================================================
Defines cache categories, individual cache item descriptors, system breakdown,
pruning options and results, and prompt token bloat analysis structures.
"""

from __future__ import annotations

from dataclasses import dataclass, field
import datetime
from enum import Enum
from pathlib import Path
from typing import Any, Dict, List, Optional


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

    @property
    def task_count(self) -> int:
        return self.conversation_count

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
    prune_wal: bool = False
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
            "pruned_directories_count": self.pruned_directories_count,
            "categories_affected": self.categories_affected,
            "elapsed_seconds": round(self.elapsed_seconds, 3),
            "protected_active_id": self.protected_active_id,
            "protected_pinned_ids": self.protected_pinned_ids,
            "dry_run": self.dry_run,
            "databases_vacuumed": self.databases_vacuumed,
            "details": self.details[:50],
        }


@dataclass
class RedundancyMetrics:
    """Detailed breakdown of redundant token categories."""
    static_system_tokens: int = 0
    duplicated_schema_tokens: int = 0
    verbose_output_bloat_tokens: int = 0
    repeated_error_tokens: int = 0


@dataclass
class PromptCacheAnalysis:
    """Analysis of prompt token consumption and bloat within a conversation."""
    conversation_id: str
    total_steps: int
    turn_count: int
    total_prompt_tokens: int
    estimated_redundant_tokens: int
    potential_savings_fraction: float
    oversized_tool_outputs_count: int
    cached_prefix_potential_tokens: int
    tool_pruning_savings_tokens: int
    breakdown: RedundancyMetrics = field(default_factory=RedundancyMetrics)
    optimization_recommendations: List[str] = field(default_factory=list)

    # Backwards compatibility aliases
    @property
    def bloat_tokens(self) -> int:
        return self.estimated_redundant_tokens

    @property
    def savings_potential_fraction(self) -> float:
        return self.potential_savings_fraction

    def to_dict(self) -> Dict[str, Any]:
        return {
            "conversation_id": self.conversation_id,
            "total_steps": self.total_steps,
            "turn_count": self.turn_count,
            "total_prompt_tokens": self.total_prompt_tokens,
            "estimated_redundant_tokens": self.estimated_redundant_tokens,
            "potential_savings_fraction": round(self.potential_savings_fraction, 4),
            "potential_savings_percent": round(self.potential_savings_fraction * 100, 2),
            "oversized_tool_outputs_count": self.oversized_tool_outputs_count,
            "cached_prefix_potential_tokens": self.cached_prefix_potential_tokens,
            "tool_pruning_savings_tokens": self.tool_pruning_savings_tokens,
            "bloat_tokens": self.bloat_tokens,
            "savings_potential_fraction": self.savings_potential_fraction,
            "breakdown": {
                "static_system_tokens": self.breakdown.static_system_tokens,
                "duplicated_schema_tokens": self.breakdown.duplicated_schema_tokens,
                "verbose_output_bloat_tokens": self.breakdown.verbose_output_bloat_tokens,
                "repeated_error_tokens": self.breakdown.repeated_error_tokens,
            },
            "optimization_recommendations": self.optimization_recommendations,
        }


# Maintain backward compatibility alias
TokenBloatReport = PromptCacheAnalysis
