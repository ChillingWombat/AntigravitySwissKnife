"""
Prompt Cache Optimizer & Context Bloat Analyzer (Feature F16).
==============================================================
Analyzes transcript logs to measure prompt token bloat from oversized tool outputs,
repetitive error dumps, and redundant system messages.
"""

from __future__ import annotations

from dataclasses import dataclass
import json
import logging
from pathlib import Path
from typing import Dict, List, Optional

logger = logging.getLogger("antigravity_swiss.cache_optimizer.prompt_cache")


@dataclass
class TokenBloatReport:
    """Analysis of token bloat and redundant context within a conversation."""
    conversation_id: str
    total_steps: int
    estimated_total_tokens: int
    bloat_tokens: int
    oversized_tool_outputs_count: int
    savings_potential_fraction: float


class PromptCacheOptimizer:
    """Analyzes transcript JSONL logs to compute token consumption and identify bloat."""

    @staticmethod
    def estimate_tokens(text: str) -> int:
        """Heuristic token estimator (~4 chars per token for English & code)."""
        if not text:
            return 0
        return max(1, len(text) // 4)

    def analyze_transcript(self, transcript_path: Path | str) -> Optional[TokenBloatReport]:
        """Analyzes a transcript.jsonl file for context bloat."""
        path = Path(transcript_path).expanduser().resolve()
        if not path.exists():
            return None

        total_steps = 0
        total_chars = 0
        bloat_chars = 0
        oversized_outputs = 0

        conv_id = path.parent.parent.parent.name

        try:
            with open(path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    try:
                        step = json.loads(line)
                    except json.JSONDecodeError:
                        continue

                    total_steps += 1
                    content = str(step.get("content") or "")
                    thinking = str(step.get("thinking") or "")
                    total_chars += len(content) + len(thinking)

                    # Inspect step outputs for bloat (> 8KB tool output)
                    if len(content) > 8192:
                        oversized_outputs += 1
                        bloat_chars += (len(content) - 4096)
        except Exception as exc:
            logger.error("Failed to analyze transcript %s: %s", path, exc)
            return None

        est_tokens = self.estimate_tokens("a" * total_chars)
        bloat_tokens = self.estimate_tokens("a" * bloat_chars)
        savings_frac = (bloat_tokens / est_tokens) if est_tokens > 0 else 0.0

        return TokenBloatReport(
            conversation_id=conv_id,
            total_steps=total_steps,
            estimated_total_tokens=est_tokens,
            bloat_tokens=bloat_tokens,
            oversized_tool_outputs_count=oversized_outputs,
            savings_potential_fraction=round(savings_frac, 3),
        )
