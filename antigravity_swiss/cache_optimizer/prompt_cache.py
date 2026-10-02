"""
Prompt Cache Optimizer & Context Bloat Analyzer (Feature F14).
==============================================================
Analyzes transcript logs to measure cumulative prompt token bloat from:
1. Repeated static system instructions across turns without prefix caching.
2. Duplicated tool schemas and repetitive tool definitions.
3. Oversized tool output bloat accumulating across subsequent turns.
"""

from __future__ import annotations

import json
import logging
from pathlib import Path
import re
from typing import Any, Dict, List, Optional

from antigravity_swiss.cache_optimizer.models import (
    PromptCacheAnalysis,
    RedundancyMetrics,
    TokenBloatReport,
)

logger = logging.getLogger("antigravity_swiss.cache_optimizer.prompt_cache")

# Heuristic constants
CHARS_PER_TOKEN = 3.8
OVERSIZED_OUTPUT_THRESHOLD_BYTES = 8192
OVERSIZED_OUTPUT_THRESHOLD_TOKENS = 2048
COMPACT_OUTPUT_BUDGET_TOKENS = 512
GEMINI_CACHE_MIN_TOKENS = 32768
ANTHROPIC_CACHE_MIN_TOKENS = 1024


class PromptCacheOptimizer:
    """Analyzes transcript JSONL logs to compute token consumption and identify redundancy."""

    def __init__(self, data_dir: Optional[Path | str] = None) -> None:
        self.data_dir = Path(data_dir).expanduser().resolve() if data_dir else None

    @staticmethod
    def estimate_tokens(text: str) -> int:
        """Heuristic token estimator (~3.8 chars per token for English text and code)."""
        if not text:
            return 0
        return max(1, int(len(text) / CHARS_PER_TOKEN))

    def analyze_transcript(
        self,
        transcript_path: Path | str,
        conversation_id: Optional[str] = None,
    ) -> Optional[PromptCacheAnalysis]:
        """Analyzes a transcript.jsonl file for context bloat and token reduction opportunities."""
        path = Path(transcript_path).expanduser().resolve()
        if not path.exists():
            return None

        conv_id = conversation_id
        if not conv_id:
            # Extract UUID from path hierarchy: brain/<conv_id>/.system_generated/logs/transcript.jsonl
            parts = path.parts
            for i, p in enumerate(parts):
                if p == "brain" and i + 1 < len(parts):
                    conv_id = parts[i + 1]
                    break
            if not conv_id:
                conv_id = path.parent.parent.parent.name if len(path.parents) >= 3 else path.stem

        steps: List[Dict[str, Any]] = []
        try:
            with open(path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    try:
                        step_data = json.loads(line)
                        steps.append(step_data)
                    except json.JSONDecodeError:
                        continue
        except Exception as exc:
            logger.error("Failed to read transcript %s: %s", path, exc)
            return None

        if not steps:
            return PromptCacheAnalysis(
                conversation_id=conv_id,
                total_steps=0,
                turn_count=0,
                total_prompt_tokens=0,
                estimated_redundant_tokens=0,
                potential_savings_fraction=0.0,
                oversized_tool_outputs_count=0,
                cached_prefix_potential_tokens=0,
                tool_pruning_savings_tokens=0,
                breakdown=RedundancyMetrics(),
                optimization_recommendations=["Transcript is empty."],
            )

        # 1. Parse steps and categorize content
        system_chars = 0
        total_model_turns = 0
        step_token_counts: List[int] = []
        oversized_outputs = 0
        tool_output_bloat_tokens = 0
        tool_pruning_savings = 0
        duplicated_schema_tokens = 0
        known_tool_names: set[str] = set()

        # Check for step output directory
        steps_dir = path.parent.parent / "steps"

        for idx, step in enumerate(steps):
            source = str(step.get("source") or "").upper()
            stype = str(step.get("type") or "").upper()
            content = str(step.get("content") or "")
            thinking = str(step.get("thinking") or "")
            tool_calls = step.get("tool_calls") or []

            # If content points to external file and file exists, inspect real size
            if "output.txt" in content and steps_dir.exists():
                step_idx = step.get("step_index", idx)
                ext_file = steps_dir / str(step_idx) / "output.txt"
                if ext_file.exists():
                    try:
                        content = ext_file.read_text(encoding="utf-8", errors="ignore")
                    except OSError:
                        pass

            step_tokens = self.estimate_tokens(content) + self.estimate_tokens(thinking)

            # Record system instruction size
            if source == "SYSTEM" or stype == "SYSTEM_MESSAGE" or idx == 0:
                system_chars += len(content)

            # Record model turns
            if source == "MODEL" or stype == "PLANNER_RESPONSE":
                total_model_turns += 1
                for tc in tool_calls:
                    if isinstance(tc, dict):
                        tname = tc.get("name", "")
                        if tname in known_tool_names:
                            # Repeated tool call schema/signature bloat
                            duplicated_schema_tokens += self.estimate_tokens(str(tc))
                        known_tool_names.add(tname)

            # Check for oversized tool execution outputs (>8KB)
            if stype == "GENERIC" or "output.txt" in content or len(content) > OVERSIZED_OUTPUT_THRESHOLD_BYTES:
                if len(content) > OVERSIZED_OUTPUT_THRESHOLD_BYTES:
                    oversized_outputs += 1
                    excess_tokens = max(0, step_tokens - COMPACT_OUTPUT_BUDGET_TOKENS)
                    # Remaining turns that have to carry this output
                    remaining_turns = max(1, len(steps) - idx)
                    turn_factor = max(1, remaining_turns // 2)
                    bloat = excess_tokens * turn_factor
                    tool_output_bloat_tokens += bloat
                    tool_pruning_savings += excess_tokens

            step_token_counts.append(step_tokens)

        # 2. Cumulative Prompt Tokens Calculation (Triangular Sum across turns)
        static_system_tokens = self.estimate_tokens("a" * system_chars)
        turn_count = max(1, total_model_turns)

        # Static prefix cache potential: repeated across (turn_count - 1) turns
        cached_prefix_potential = (turn_count - 1) * static_system_tokens if turn_count > 1 else 0

        # Cumulative context token estimation
        cumulative_tokens = 0
        current_context = static_system_tokens
        for t_tokens in step_token_counts:
            current_context += t_tokens
            cumulative_tokens += current_context

        total_prompt_tokens = max(cumulative_tokens, static_system_tokens * turn_count)
        estimated_redundant_tokens = (
            cached_prefix_potential +
            tool_output_bloat_tokens +
            duplicated_schema_tokens
        )

        savings_frac = (estimated_redundant_tokens / total_prompt_tokens) if total_prompt_tokens > 0 else 0.0
        savings_frac = min(0.95, max(0.0, savings_frac))

        # 3. Formulate Recommendations
        recommendations: List[str] = []
        if static_system_tokens >= ANTHROPIC_CACHE_MIN_TOKENS and turn_count > 3:
            recommendations.append(
                f"Enable Prompt/Context Prefix Caching for static system instructions "
                f"({static_system_tokens:,} tokens). Potential saving: {cached_prefix_potential:,} tokens across {turn_count} turns."
            )
        if oversized_outputs > 0:
            recommendations.append(
                f"Prune or summarize verbose tool execution outputs older than 3 turns "
                f"({oversized_outputs} oversized outputs detected). Potential cumulative saving: {tool_output_bloat_tokens:,} tokens."
            )
        if duplicated_schema_tokens > 1000:
            recommendations.append(
                f"Deduplicate repeated tool call signatures and schemas in turn history "
                f"(est. {duplicated_schema_tokens:,} redundant schema tokens)."
            )
        if turn_count > 30:
            recommendations.append(
                f"Session length is high ({turn_count} turns). Apply rolling checkpoint summarization to bound context accumulation."
            )
        if not recommendations:
            recommendations.append("Context is compact and within optimal operational thresholds.")

        metrics = RedundancyMetrics(
            static_system_tokens=cached_prefix_potential,
            duplicated_schema_tokens=duplicated_schema_tokens,
            verbose_output_bloat_tokens=tool_output_bloat_tokens,
            repeated_error_tokens=0,
        )

        return PromptCacheAnalysis(
            conversation_id=conv_id,
            total_steps=len(steps),
            turn_count=turn_count,
            total_prompt_tokens=total_prompt_tokens,
            estimated_redundant_tokens=estimated_redundant_tokens,
            potential_savings_fraction=round(savings_frac, 4),
            oversized_tool_outputs_count=oversized_outputs,
            cached_prefix_potential_tokens=cached_prefix_potential,
            tool_pruning_savings_tokens=tool_pruning_savings,
            breakdown=metrics,
            optimization_recommendations=recommendations,
        )

    def analyze_conversation(
        self,
        conversation_id: str,
        data_dir: Optional[Path | str] = None,
    ) -> Optional[PromptCacheAnalysis]:
        """Analyzes a conversation given its UUID."""
        base = Path(data_dir).expanduser().resolve() if data_dir else self.data_dir
        if not base:
            from antigravity_swiss.core.constants import DEFAULT_ANTIGRAVITY_DATA_DIR
            base = Path(DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()

        transcript_path = base / "brain" / conversation_id / ".system_generated" / "logs" / "transcript.jsonl"
        if not transcript_path.exists():
            transcript_path = base / "brain" / conversation_id / "transcript.jsonl"
        return self.analyze_transcript(transcript_path, conversation_id=conversation_id)

    def scan_all_conversations(
        self,
        data_dir: Optional[Path | str] = None,
        limit: int = 20,
    ) -> List[PromptCacheAnalysis]:
        """Scans recent conversation sessions in brain/ and produces analyses."""
        base = Path(data_dir).expanduser().resolve() if data_dir else self.data_dir
        if not base:
            from antigravity_swiss.core.constants import DEFAULT_ANTIGRAVITY_DATA_DIR
            base = Path(DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()

        brain_dir = base / "brain"
        results: List[PromptCacheAnalysis] = []
        if not brain_dir.exists():
            return results

        dirs = [d for d in brain_dir.iterdir() if d.is_dir() and d.name != "tempmediaStorage"]
        dirs.sort(key=lambda d: d.stat().st_mtime, reverse=True)

        for d in dirs[:limit]:
            transcript = d / ".system_generated" / "logs" / "transcript.jsonl"
            if transcript.exists():
                rep = self.analyze_transcript(transcript, conversation_id=d.name)
                if rep:
                    results.append(rep)
        return results
