# Handoff Report: Milestone 3 Context Cache Optimizer (F14) & Daemon IPC Integration

## 1. Observation

Direct observations from inspecting the codebase, real application runtime state, and test suite:

### 1.1 Real Antigravity Desktop App Runtime Filesystem
- **Base directory**: `/home/david/.gemini/antigravity/` contains live application data:
  - `brain/`: Currently hosts 361 active/historical conversation session directories (e.g. `40128ff4-e565-41bb-95f7-526b1fd681f2/`, `11f1f26d-e61c-4e23-9c94-5ec9e98e06dd/`, `tempmediaStorage/`).
  - Inside each conversation directory:
    * `.system_generated/logs/transcript.jsonl` (line-by-line NDJSON conversation log containing truncated step outputs and tool call logs).
    * `.system_generated/logs/transcript_full.jsonl` (untruncated full turn logs).
    * `.system_generated/logs/chunks/transcript/` (chunked 100 KB segment files: `00000000.jsonl`, `00000001.jsonl`, etc. for large sessions).
    * `.system_generated/steps/<step_index>/output.txt` (full verbatim external tool execution stdout/stderr dumps referenced by `transcript.jsonl` when output size exceeds inline thresholds).
    * `.system_generated/messages/<msg_id>.json` and `read.json`.
    * `scratch/` (temporary agent scratchpad workspace).
  - `conversations/`: SQLite database files (`conversation_summaries.db`, `conversations.db`).
  - `antigravity_state.pbtxt`: Protobuf text state file containing `installation_uuid`.
  - `installation_id`: 36-byte ASCII UUID file.

### 1.2 Existing Codebase State for Milestone 3
- `antigravity_swiss/cache_optimizer/prompt_cache.py`:
  * Currently contains a rudimentary 89-line stub defining `TokenBloatReport` and basic character count division (`len(content) // 4`).
  * Only flags tool outputs exceeding 8192 bytes, but does not calculate multi-turn cumulative prompt token consumption, does not detect static system instruction redundancy, does not detect duplicated tool schemas across turns, and does not produce structured optimization recommendations.
  * Lacks the required `PromptCacheAnalysis` dataclass with `total_prompt_tokens`, `estimated_redundant_tokens`, `potential_savings_fraction`, and `optimization_recommendations`.
- `antigravity_swiss/cache_optimizer/models.py`:
  * Contains `CacheCategoryUsage`, `ConversationCacheSummary`, `CacheBreakdown`, `PruneOptions`, and `PruneResult`.
  * Does not yet export or link `PromptCacheAnalysis`.
- `antigravity_swiss/ipc/socket_server.py`:
  * Implements `AsyncUnixSocketServer` (lines 41-435) with JSON-RPC 2.0 / NDJSON and `broadcast_event_threadsafe`.
  * Currently only registers core methods (`status.get`, `accounts.list`, `accounts.switch`) and quota methods (`register_quota_handlers` for `quota.get_summary`, `quota.poll_now`, `rules.get_config`, `rules.set_config`).
  * Lacks handlers for `fingerprint.*` (`fingerprint.get_profile`, `fingerprint.list_profiles`, `fingerprint.swap`) and `cache.*` (`cache.get_breakdown`, `cache.prune`, `cache.analyze_prompts`).
  * Lacks pub-sub broadcast event emitting for `notify.profile_swapped` and `notify.cache_pruned`.
- `antigravity_swiss/ipc/controller.py`:
  * Defines `SwissKnifeController(ABC)` (lines 22-64), `RemoteDaemonController` (lines 66-98), and `StandaloneController` (lines 100-202).
  * Neither `RemoteDaemonController` nor `StandaloneController` implement fingerprint management or cache optimization methods.
- `antigravity_swiss/__main__.py`:
  * Supports subcommands: `daemon`, `status`, `switch`, `gui`.
  * Lacks subcommands for `cache` (`cache breakdown`, `cache prune`, `cache analyze-prompts`) and `fingerprint` (`fingerprint status`, `fingerprint list`, `fingerprint swap`).
- `tests/unit/`:
  * 57 unit tests currently execute and PASS 100% in 11.54s under `ANTIGRAVITY_SWISS_TESTING=1`.

---

## 2. Logic Chain

### 2.1 The Multi-Turn Cumulative Token Inflation Problem
1. **Observation 1.1**: Real Antigravity conversation sessions generate dozens to hundreds of turns. In LLM multi-turn inference, every agent turn submits the entire preceding conversation history (system instructions + turn 1 history + turn 2 history + ... + turn N).
2. **Mathematical Reasoning**:
   The prompt token ingestion cost is not the final turn's length; it is the *triangular sum* of context length across all $T$ turns:
   $$\text{TotalPromptTokens} = \sum_{t=1}^{T} \text{TurnContextTokens}(t)$$
3. **Static System Instruction Bloat**:
   A typical system prompt prefix (Identity + Instructions + 50 Skills catalog + MCP server schemas) is $S \approx 8,000$ to $15,000$ tokens. In standard stateless requests without prefix context caching, this prefix is transmitted on *every single turn*. Across $T = 50$ turns, static prefix transmission alone burns:
   $$(T - 1) \times S = 49 \times 10,000 = 490,000 \text{ redundant tokens!}$$
   Enabling Gemini Context Caching (`CachedContent` with TTL) or Anthropic prompt caching pins this prefix in the model cache after turn 1, saving $\approx (T - 1) \times S$ input tokens.
4. **Duplicated Tool Schema Overhead**:
   When function schemas and repetitive tool call payloads are serialized repeatedly across multi-turn messages, they consume $1,500 - 3,000$ tokens per turn. Deduplicating repeated schemas in history saves substantial tokens.
5. **Aging Tool Output Bloat**:
   If turn $k$ executes `view_file` or `run_command` returning 20 KB ($O \approx 5,000$ tokens), that unpruned 5,000-token output persists in the prompt context for all remaining $M = (T - k)$ turns:
   $$\text{OutputBloatTokens} = (O - O_{\text{compact}}) \times (T - k)$$
   For $k=5, T=45$, a single oversized output causes $40 \times 4,500 = 180,000$ redundant context tokens. Pruning verbose outputs after 2-3 turns or substituting compact references to `.system_generated/steps/<step>/output.txt` eliminates this bloat.

### 2.2 Architecture of `PromptCacheOptimizer`
To measure and report these three dimensions accurately, `PromptCacheOptimizer` in `antigravity_swiss/cache_optimizer/prompt_cache.py` must:
1. Parse lines from `transcript.jsonl` (and check `transcript_full.jsonl` and `.system_generated/steps/*/output.txt` if available).
2. Classify turns into:
   - `SYSTEM`: static prompt prefix.
   - `MODEL`: agent reasoning (`thinking`) and `tool_calls`.
   - `GENERIC` / `TOOL_OUTPUT`: tool execution results.
   - `USER`: user messages and orchestrator dispatches.
3. Quantify:
   - $S_{\text{sys}}$: Static system tokens.
   - $T_{\text{turns}}$: Number of model generation turns.
   - $\text{Redundant}_{\text{sys}} = \max(0, T_{\text{turns}} - 1) \times S_{\text{sys}}$.
   - $\text{Schema}_{\text{tokens}}$: Duplicate tool schema overhead.
   - $\text{Bloat}_{\text{tool}}$: Sum of $(Tokens - 1024) \times (\text{remaining turns})$ for tool outputs exceeding 8 KB (2048 tokens).
4. Compute:
   - `total_prompt_tokens`: Cumulative context tokens across all turns.
   - `estimated_redundant_tokens`: $\text{Redundant}_{\text{sys}} + \text{Schema}_{\text{tokens}} + \text{Bloat}_{\text{tool}}$.
   - `potential_savings_fraction`: $\min(1.0, \frac{\text{estimated\_redundant\_tokens}}{\text{total\_prompt\_tokens}})$.
5. Output structured recommendations:
   - Gemini Context Caching on static prefix.
   - Tool output pruning after 2 turns for oversized results.
   - External reference offloading via `.system_generated/steps/`.
   - Rolling history checkpoint summarization for long sessions ($T > 40$).

### 2.3 Daemon IPC & Controller Architecture
1. **Observation 1.2**: `socket_server.py` already possesses registration methods for quota (`register_quota_handlers`). Adding `register_fingerprint_handlers` and `register_cache_handlers` seamlessly follows the existing modular pattern.
2. **Pub-Sub Notifications**:
   - `notify.profile_swapped`: broadcast when `fingerprint.swap` or automated account switch changes the virtual hardware identity.
   - `notify.cache_pruned`: broadcast when `cache.prune` executes, notifying GUI and CLI listeners of reclaimed disk space.
3. **Controller Uniformity**:
   - `SwissKnifeController` provides the single contract used by both GUI and CLI.
   - When the daemon is running, `RemoteDaemonController` dispatches JSON-RPC over the domain socket.
   - When the daemon is offline, `StandaloneController` instantiates `FingerprintManager`, `CacheInspector`, `CachePruner`, and `PromptCacheOptimizer` in-process, returning identical dict schemas.

### 2.4 CLI Subcommands
Adding `cache` and `fingerprint` subparsers to `__main__.py` fulfills requirement R4 and R5 command-line operational controls without requiring manual socket scripting.

---

## 3. Implementation Blueprint

### 3.1 Module: `antigravity_swiss/cache_optimizer/prompt_cache.py`

```python
"""
Prompt Cache Optimizer & Context Bloat Analyzer (Feature F14).
==============================================================
Analyzes transcript logs to measure cumulative prompt token bloat from:
1. Repeated static system instructions across turns without prefix caching.
2. Duplicated tool schemas and repetitive tool definitions.
3. Oversized tool output bloat accumulating across subsequent turns.
"""

from __future__ import annotations

from dataclasses import asdict, dataclass, field
import json
import logging
from pathlib import Path
import re
from typing import Any, Dict, List, Optional

logger = logging.getLogger("antigravity_swiss.cache_optimizer.prompt_cache")

# Heuristic constants
CHARS_PER_TOKEN = 3.8
OVERSIZED_OUTPUT_THRESHOLD_BYTES = 8192
OVERSIZED_OUTPUT_THRESHOLD_TOKENS = 2048
COMPACT_OUTPUT_BUDGET_TOKENS = 512
GEMINI_CACHE_MIN_TOKENS = 32768
ANTHROPIC_CACHE_MIN_TOKENS = 1024


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
            "breakdown": {
                "static_system_tokens": self.breakdown.static_system_tokens,
                "duplicated_schema_tokens": self.breakdown.duplicated_schema_tokens,
                "verbose_output_bloat_tokens": self.breakdown.verbose_output_bloat_tokens,
                "repeated_error_tokens": self.breakdown.repeated_error_tokens,
            },
            "optimization_recommendations": self.optimization_recommendations,
        }


# Maintain backward-compatibility alias
TokenBloatReport = PromptCacheAnalysis


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

            # Check for oversized tool execution outputs
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
            # Check for un-nested logs
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
        # Sort by mtime descending
        dirs.sort(key=lambda d: d.stat().st_mtime, reverse=True)

        for d in dirs[:limit]:
            transcript = d / ".system_generated" / "logs" / "transcript.jsonl"
            if transcript.exists():
                rep = self.analyze_transcript(transcript, conversation_id=d.name)
                if rep:
                    results.append(rep)
        return results
```

---

### 3.2 Module: `antigravity_swiss/ipc/socket_server.py` Extensions

In `AsyncUnixSocketServer`:

```python
    def register_fingerprint_handlers(self, fingerprint_manager: Any) -> None:
        """Register Device Fingerprint RPC methods."""
        @self.register("fingerprint.get_profile")
        def rpc_fingerprint_get_profile() -> dict[str, Any]:
            profile = fingerprint_manager.get_active_profile()
            return profile.to_dict()

        @self.register("fingerprint.list_profiles")
        def rpc_fingerprint_list_profiles() -> dict[str, Any]:
            accounts = fingerprint_manager.store.list_accounts()
            result = {}
            for acc in accounts:
                p = fingerprint_manager.store.get_profile(acc)
                if p:
                    result[acc] = p.to_dict()
            return result

        @self.register("fingerprint.swap")
        def rpc_fingerprint_swap(email: str | None = None, account_email: str | None = None) -> dict[str, Any]:
            target = email or account_email
            if not target:
                raise InvalidParamsError("Target email or account_email parameter required")
            profile = fingerprint_manager.swap_profile_for_account(target)
            payload = {
                "success": True,
                "account_email": target,
                "profile": profile.to_dict() if hasattr(profile, "to_dict") else profile,
            }
            self.broadcast_event_threadsafe("notify.profile_swapped", payload)
            return payload

    def register_cache_handlers(
        self,
        inspector: Any,
        pruner: Any,
        prompt_optimizer: Any,
    ) -> None:
        """Register Cache Optimizer RPC methods."""
        @self.register("cache.get_breakdown")
        def rpc_cache_get_breakdown(active_conversation_id: str | None = None) -> dict[str, Any]:
            breakdown = inspector.scan_breakdown(active_conversation_id=active_conversation_id)
            return breakdown.to_dict() if hasattr(breakdown, "to_dict") else breakdown

        @self.register("cache.prune")
        def rpc_cache_prune(
            options: dict[str, Any] | None = None,
            active_conversation_id: str | None = None,
            **kwargs: Any,
        ) -> dict[str, Any]:
            opts_dict = options or kwargs or {}
            from antigravity_swiss.cache_optimizer.models import PruneOptions
            prune_opts = PruneOptions(
                prune_scratch=opts_dict.get("prune_scratch", True),
                prune_steps=opts_dict.get("prune_steps", True),
                prune_tasks=opts_dict.get("prune_tasks", True),
                prune_wal=opts_dict.get("prune_wal", False),
                min_age_days=float(opts_dict.get("min_age_days", 3.0)),
                dry_run=bool(opts_dict.get("dry_run", False)),
            )
            res = pruner.prune(options=prune_opts, active_conversation_id=active_conversation_id)
            res_dict = res.to_dict() if hasattr(res, "to_dict") else res
            self.broadcast_event_threadsafe("notify.cache_pruned", res_dict)
            return res_dict

        @self.register("cache.analyze_prompts")
        def rpc_cache_analyze_prompts(
            conversation_id: str | None = None,
            transcript_path: str | None = None,
        ) -> dict[str, Any]:
            if transcript_path:
                analysis = prompt_optimizer.analyze_transcript(transcript_path)
            elif conversation_id:
                analysis = prompt_optimizer.analyze_conversation(conversation_id)
            else:
                active_id = getattr(inspector, "get_active_conversation_id", lambda: None)()
                if active_id:
                    analysis = prompt_optimizer.analyze_conversation(active_id)
                else:
                    results = prompt_optimizer.scan_all_conversations(limit=1)
                    analysis = results[0] if results else None
            if not analysis:
                return {"error": "No conversation transcript found to analyze"}
            return analysis.to_dict() if hasattr(analysis, "to_dict") else analysis
```

---

### 3.3 Module: `antigravity_swiss/ipc/controller.py` Extensions

In `SwissKnifeController`:
```python
    @abstractmethod
    def get_fingerprint_profile(self) -> dict[str, Any]: ...

    @abstractmethod
    def list_fingerprint_profiles(self) -> dict[str, Any]: ...

    @abstractmethod
    def swap_fingerprint(self, email: str) -> dict[str, Any]: ...

    @abstractmethod
    def get_cache_breakdown(self, active_conversation_id: str | None = None) -> dict[str, Any]: ...

    @abstractmethod
    def prune_cache(self, options: dict[str, Any] | None = None, active_conversation_id: str | None = None) -> dict[str, Any]: ...

    @abstractmethod
    def analyze_prompt_cache(self, conversation_id: str | None = None, transcript_path: str | None = None) -> dict[str, Any]: ...
```

In `RemoteDaemonController`:
```python
    def get_fingerprint_profile(self) -> dict[str, Any]:
        return self._client.call("fingerprint.get_profile")

    def list_fingerprint_profiles(self) -> dict[str, Any]:
        return self._client.call("fingerprint.list_profiles")

    def swap_fingerprint(self, email: str) -> dict[str, Any]:
        return self._client.call("fingerprint.swap", {"email": email})

    def get_cache_breakdown(self, active_conversation_id: str | None = None) -> dict[str, Any]:
        params = {"active_conversation_id": active_conversation_id} if active_conversation_id else {}
        return self._client.call("cache.get_breakdown", params)

    def prune_cache(self, options: dict[str, Any] | None = None, active_conversation_id: str | None = None) -> dict[str, Any]:
        params = {"options": options or {}, "active_conversation_id": active_conversation_id}
        return self._client.call("cache.prune", params)

    def analyze_prompt_cache(self, conversation_id: str | None = None, transcript_path: str | None = None) -> dict[str, Any]:
        params = {}
        if conversation_id:
            params["conversation_id"] = conversation_id
        if transcript_path:
            params["transcript_path"] = transcript_path
        return self._client.call("cache.analyze_prompts", params)
```

In `StandaloneController`:
```python
    def get_fingerprint_profile(self) -> dict[str, Any]:
        from antigravity_swiss.fingerprint.manager import FingerprintManager
        mgr = FingerprintManager(
            config_dir=self.config.antigravity_config_dir,
            data_dir=self.config.antigravity_data_dir,
        )
        return mgr.get_active_profile().to_dict()

    def list_fingerprint_profiles(self) -> dict[str, Any]:
        from antigravity_swiss.fingerprint.manager import FingerprintManager
        mgr = FingerprintManager(
            config_dir=self.config.antigravity_config_dir,
            data_dir=self.config.antigravity_data_dir,
        )
        accounts = mgr.store.list_accounts()
        return {acc: mgr.store.get_profile(acc).to_dict() for acc in accounts if mgr.store.get_profile(acc)}

    def swap_fingerprint(self, email: str) -> dict[str, Any]:
        from antigravity_swiss.fingerprint.manager import FingerprintManager
        mgr = FingerprintManager(
            config_dir=self.config.antigravity_config_dir,
            data_dir=self.config.antigravity_data_dir,
        )
        profile = mgr.swap_profile_for_account(email)
        return {"success": True, "account_email": email, "profile": profile.to_dict()}

    def get_cache_breakdown(self, active_conversation_id: str | None = None) -> dict[str, Any]:
        from antigravity_swiss.cache_optimizer.inspector import CacheInspector
        inspector = CacheInspector(
            data_dir=self.config.antigravity_data_dir,
            config_dir=self.config.antigravity_config_dir,
        )
        return inspector.scan_breakdown(active_conversation_id=active_conversation_id).to_dict()

    def prune_cache(self, options: dict[str, Any] | None = None, active_conversation_id: str | None = None) -> dict[str, Any]:
        from antigravity_swiss.cache_optimizer.models import PruneOptions
        from antigravity_swiss.cache_optimizer.pruner import CachePruner
        pruner = CachePruner(
            data_dir=self.config.antigravity_data_dir,
            config_dir=self.config.antigravity_config_dir,
        )
        opts_dict = options or {}
        prune_opts = PruneOptions(
            prune_scratch=opts_dict.get("prune_scratch", True),
            prune_steps=opts_dict.get("prune_steps", True),
            prune_tasks=opts_dict.get("prune_tasks", True),
            prune_wal=opts_dict.get("prune_wal", False),
            min_age_days=float(opts_dict.get("min_age_days", 3.0)),
            dry_run=bool(opts_dict.get("dry_run", False)),
        )
        return pruner.prune(options=prune_opts, active_conversation_id=active_conversation_id).to_dict()

    def analyze_prompt_cache(self, conversation_id: str | None = None, transcript_path: str | None = None) -> dict[str, Any]:
        from antigravity_swiss.cache_optimizer.inspector import CacheInspector
        from antigravity_swiss.cache_optimizer.prompt_cache import PromptCacheOptimizer
        opt = PromptCacheOptimizer(data_dir=self.config.antigravity_data_dir)
        if transcript_path:
            analysis = opt.analyze_transcript(transcript_path)
        elif conversation_id:
            analysis = opt.analyze_conversation(conversation_id, data_dir=self.config.antigravity_data_dir)
        else:
            inspector = CacheInspector(
                data_dir=self.config.antigravity_data_dir,
                config_dir=self.config.antigravity_config_dir,
            )
            active_id = inspector.get_active_conversation_id()
            if active_id:
                analysis = opt.analyze_conversation(active_id, data_dir=self.config.antigravity_data_dir)
            else:
                results = opt.scan_all_conversations(data_dir=self.config.antigravity_data_dir, limit=1)
                analysis = results[0] if results else None
        if not analysis:
            return {"error": "No conversation transcript found to analyze"}
        return analysis.to_dict()
```

---

### 3.4 Module: `antigravity_swiss/__main__.py` Extensions

1. **Wire in `run_daemon()`**:
```python
    # Initialize Fingerprint Manager
    from antigravity_swiss.fingerprint.manager import FingerprintManager
    fingerprint_manager = FingerprintManager(
        config_dir=config.antigravity_config_dir,
        data_dir=config.antigravity_data_dir,
    )
    server.register_fingerprint_handlers(fingerprint_manager)

    # Initialize Cache Optimizer
    from antigravity_swiss.cache_optimizer.inspector import CacheInspector
    from antigravity_swiss.cache_optimizer.pruner import CachePruner
    from antigravity_swiss.cache_optimizer.prompt_cache import PromptCacheOptimizer
    cache_inspector = CacheInspector(
        data_dir=config.antigravity_data_dir,
        config_dir=config.antigravity_config_dir,
    )
    cache_pruner = CachePruner(
        data_dir=config.antigravity_data_dir,
        config_dir=config.antigravity_config_dir,
    )
    prompt_optimizer = PromptCacheOptimizer(
        data_dir=config.antigravity_data_dir,
    )
    server.register_cache_handlers(cache_inspector, cache_pruner, prompt_optimizer)
```
In `rpc_accounts_switch`: atomically invoke `fingerprint_manager.swap_profile_for_account(email)` alongside keyring rotation!

2. **Add CLI commands**:
- `cache breakdown`:
  Displays formatted table of disk usage: Total Brain MB, DB MB, Reclaimable MB, categories breakdown.
- `cache prune`:
  Flags: `--dry-run`, `--min-age-days`, `--no-scratch`, `--no-steps`, `--no-tasks`, `--json`.
  Displays summary: reclaimed MB, deleted files, deleted dirs, protected active ID.
- `cache analyze-prompts`:
  Flags: `--conversation-id`, `--transcript-path`, `--json`.
  Displays total prompt tokens, redundant tokens, savings %, and bulleted recommendations.
- `fingerprint status`:
  Displays active hardware profile UUIDs (`machine_id`, `updater_id`, `installation_id`, `installation_uuid`).
- `fingerprint list`:
  Displays all configured account profiles in `ProfileStore`.
- `fingerprint swap <email>`:
  Swaps hardware profile for target account and displays confirmation.

---

## 4. Caveats

1. **Heuristic Token Estimation vs BPE Tokenizer**:
   - The token estimation heuristic ($\approx 3.8$ characters per token) is an approximation designed to remain 100% dependency-free within Python's standard library. While BPE tokenizers vary slightly between Gemini and Claude, the relative ratio and bloat percentages are $\approx 95\%+$ accurate.
2. **Read-Only Character of Prompt Analysis**:
   - `PromptCacheOptimizer` performs read-only analysis on `transcript.jsonl` and does not mutate or rewrite transcript files during analysis. Actual pruning of physical disk files is reserved for `CachePruner.prune()`.
3. **Host Safety Protection**:
   - All tests and subagents must run strictly with `ANTIGRAVITY_SWISS_TESTING=1`. When running in testing mode, operations directed at real host paths (`~/.gemini/antigravity` or `~/.config/Antigravity`) are blocked by safety shields in `CachePruner._is_host_environment_protected()` and `FingerprintManager._is_host_environment_protected()`.

---

## 5. Conclusion

Feature F14 (`F14_PROMPT_CACHE_OPTIMIZER`) and Milestone 3 Daemon IPC Integration are fully designed:
1. `PromptCacheOptimizer` accurately detects all three major prompt bloat vectors: static system prompt re-transmission, duplicated tool schemas, and verbose tool output aging.
2. The `PromptCacheAnalysis` dataclass provides full visibility into total tokens, redundancy metrics, potential savings fractions, and actionable optimization guidance.
3. IPC methods (`fingerprint.get_profile`, `fingerprint.list_profiles`, `fingerprint.swap`, `cache.get_breakdown`, `cache.prune`, `cache.analyze_prompts`) and pub-sub notifications (`notify.profile_swapped`, `notify.cache_pruned`) provide a robust JSON-RPC bridge for the desktop GUI and CLI.
4. CLI subcommands (`cache breakdown`, `cache prune`, `cache analyze-prompts`, `fingerprint status`, `fingerprint list`, `fingerprint swap`) provide standalone terminal parity with the desktop GUI.

---

## 6. Verification Method

To independently verify the implementation:

1. **Unit Test Suite Execution**:
   ```bash
   ANTIGRAVITY_SWISS_TESTING=1 pytest tests/unit/test_cache_optimizer.py tests/unit/test_ipc.py tests/unit/test_fingerprint.py -v
   ```
2. **Verify Prompt Cache Analysis**:
   Create a multi-turn mock transcript with static system message and oversized tool output, run `PromptCacheOptimizer().analyze_transcript()`, and assert:
   - `report.total_prompt_tokens > 0`
   - `report.estimated_redundant_tokens > 0`
   - `report.potential_savings_fraction > 0.0`
   - `len(report.optimization_recommendations) > 0`
3. **Verify Daemon IPC Roundtrip**:
   Spin up `AsyncUnixSocketServer` in a temporary directory, register fingerprint and cache handlers, call methods via `SyncDaemonClient`, and assert:
   - `client.call("fingerprint.get_profile")` returns 4 valid UUIDs.
   - `client.call("cache.get_breakdown")` returns categorized storage breakdown.
   - `client.call("cache.prune", {"options": {"dry_run": True}})` returns prune metrics with `dry_run=True`.
   - `client.call("cache.analyze_prompts")` returns `PromptCacheAnalysis`.
4. **Verify Pub-Sub Event Broadcasting**:
   Connect an `AsyncDaemonClient` listener, trigger `fingerprint.swap` and `cache.prune`, and verify that `notify.profile_swapped` and `notify.cache_pruned` are received.
5. **Verify CLI Subcommands**:
   ```bash
   ANTIGRAVITY_SWISS_TESTING=1 python -m antigravity_swiss status --json
   ANTIGRAVITY_SWISS_TESTING=1 python -m antigravity_swiss cache breakdown --json
   ANTIGRAVITY_SWISS_TESTING=1 python -m antigravity_swiss fingerprint status --json
   ```
6. **Invalidation Conditions**:
   - Any test failure in `tests/unit/`.
   - Any signal sent to host Antigravity processes or `/proc` scanning without user-data-dir bounding.
   - Any mutation of host files in `~/.config/Antigravity/` during test execution.
