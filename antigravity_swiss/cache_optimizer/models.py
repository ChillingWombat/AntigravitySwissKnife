"""
Cache Optimizer Models & Dataclasses (Requirement R5).
======================================================
"""

from __future__ import annotations

from dataclasses import dataclass, field
import datetime
from typing import Any, Dict, List, Optional


@dataclass
class CacheCategoryUsage:
    """Usage stats for a specific sub-category of the cache."""
    category: str
    total_bytes: int
    file_count: int
    description: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return {
            "category": self.category,
            "total_bytes": self.total_bytes,
            "total_mb": round(self.total_bytes / (1024 * 1024), 2),
            "file_count": self.file_count,
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
    reclaimable_bytes: int = 0


@dataclass
class CacheBreakdown:
    """System-wide storage and cache breakdown for Antigravity."""
    brain_total_bytes: int
    conversations_total_bytes: int
    active_session_bytes: int
    reclaimable_bytes: int
    conversation_count: int
    categories: Dict[str, CacheCategoryUsage] = field(default_factory=dict)
    conversations: List[ConversationCacheSummary] = field(default_factory=list)

    @property
    def total_bytes(self) -> int:
        return self.brain_total_bytes + self.conversations_total_bytes

    def to_dict(self) -> Dict[str, Any]:
        return {
            "brain_total_bytes": self.brain_total_bytes,
            "brain_total_mb": round(self.brain_total_bytes / (1024 * 1024), 2),
            "conversations_total_bytes": self.conversations_total_bytes,
            "conversations_total_mb": round(self.conversations_total_bytes / (1024 * 1024), 2),
            "total_mb": round(self.total_bytes / (1024 * 1024), 2),
            "active_session_bytes": self.active_session_bytes,
            "reclaimable_bytes": self.reclaimable_bytes,
            "reclaimable_mb": round(self.reclaimable_bytes / (1024 * 1024), 2),
            "conversation_count": self.conversation_count,
            "categories": {k: v.to_dict() for k, v in self.categories.items()},
        }


@dataclass
class PruneOptions:
    """Configuration options for cleaning stale cache."""
    prune_scratch: bool = True
    prune_steps: bool = True
    prune_tasks: bool = True
    prune_wal: bool = False
    min_age_days: float = 3.0
    dry_run: bool = False


@dataclass
class PruneResult:
    """Outcome of a cache pruning operation."""
    reclaimed_bytes: int
    pruned_files_count: int
    pruned_directories_count: int
    protected_active_id: Optional[str]
    dry_run: bool
    details: List[str] = field(default_factory=list)

    def to_dict(self) -> Dict[str, Any]:
        return {
            "reclaimed_bytes": self.reclaimed_bytes,
            "reclaimed_mb": round(self.reclaimed_bytes / (1024 * 1024), 2),
            "pruned_files_count": self.pruned_files_count,
            "pruned_directories_count": self.pruned_directories_count,
            "protected_active_id": self.protected_active_id,
            "dry_run": self.dry_run,
            "details": self.details[:50],  # sample first 50 operations
        }
