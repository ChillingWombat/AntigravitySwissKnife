# Milestone 2: Rule Engine, Offline Mock Server & IPC Integration Blueprint

**Agent**: `explorer_m2_3`  
**Milestone**: M2 - Upstream Quota Poller, Warmup Engine & Rule Engine  
**Scope**: F09 (`F09_AUTO_SWITCH_RULE_ENGINE`), F26 (`F26_OFFLINE_MOCK_HARNESS`), and Daemon IPC Integration  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m2_3`  
**Date**: `2026-10-02T09:10:00Z`  

---

## 1. Observation

Direct extraction from existing codebase, architecture specifications, and runtime behavior:

### 1.1 Existing Keyring and Account Vault Infrastructure
- **`antigravity_swiss/keyring/switcher.py`**:
  - Line 155: `AccountRecord` holds `email`, `label`, `credential: KeyringCredential`, `added_at`, `last_used_at`, `totp_secret`, `is_healthy: bool`.
  - Line 189: `AccountVault` manages `accounts.json` (mode `0600`) in `~/.config/antigravity-swiss/`, backed by POSIX `fcntl.flock` concurrency locking on `accounts.lock`.
  - Line 382: `AccountVault.list_accounts()` returns sorted list of registered emails.
  - Line 385: `AccountVault.list_account_records()` returns list of full `AccountRecord` objects.
  - Line 404: `AccountVault.get_active_account()` returns currently active account email.
  - Line 504: `KeyringService` exposes `get_active_credential()`, `set_active_credential()`, `list_accounts()`, and `switch_account(account_email: str, reason: str = "manual") -> bool`.
  - Line 566: `switch_account` writes active credential back to vault to prevent token loss, loads target record, updates native Secret Service (`secret-tool` or D-Bus), sets active account in vault, and fires registered switch listeners.
  - Line 632: `KeyringSwitcher.switch_to_account(email, force=False)` coordinates account switch with `AppStorageManager.preserve_active_conversation()` to preserve active `cascadeId`.

### 1.2 Existing Daemon IPC & Controller Architecture
- **`antigravity_swiss/ipc/socket_server.py`**:
  - Line 41: `AsyncUnixSocketServer` hosts JSON-RPC 2.0 / NDJSON on `$XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock` (mode `0600`, parent dir `0700`).
  - Line 70: `register(method_name: str, handler)` registers synchronous or asynchronous RPC handlers.
  - Line 162: `broadcast_event(event_name: str, payload: dict[str, Any]) -> int` broadcasts JSON-RPC 2.0 notifications to all active connected streaming clients.
  - Line 198: `broadcast_event_threadsafe(event_name, payload)` enables non-async threads or background loops to emit events safely.
- **`antigravity_swiss/ipc/controller.py`**:
  - Line 22: `SwissKnifeController(ABC)` defines interface: `is_daemon_running()`, `get_status()`, `list_accounts()`, `switch_account()`, `get_quota_summary()`.
  - Line 51: `RemoteDaemonController` connects to running daemon socket via `SyncDaemonClient`.
  - Line 74: `StandaloneController` executes in-process fallback when daemon is offline.
  - Line 145: `create_controller()` factory resolves `RemoteDaemonController` if active or `StandaloneController` fallback.
- **`antigravity_swiss/__main__.py`**:
  - Line 43: `run_daemon()` initializes `AsyncUnixSocketServer`, registers RPC methods (`status.get`, `accounts.list`, `accounts.switch`, stub `quota.get_summary`, `daemon.shutdown`), and runs `asyncio` event loop.

### 1.3 Existing Offline Mock Server (`tests/fixtures/mock_cloudcode_server.py`)
- Line 20: `MockCloudCodeHandler(BaseHTTPRequestHandler)` handles loopback HTTP/1.1 requests.
- Line 313: `MockCloudCodeServer` runs `HTTPServer` in a background daemon thread on `127.0.0.1:<random_port>`.
- Endpoints handled:
  - `POST /v1internal:retrieveUserQuotaSummary`: Returns canned Gemini and 3P quota buckets.
  - `POST /v1internal:fetchAvailableModels`: Returns model catalog (`gemini-3.8-flash-high`, `gemini-3.5-flash-lite`, `gemini-3.1-pro-low`, `claude-sonnet-4-6`).
  - `POST /v1internal:generateContent`: Simulates 1-token keep-alive warmup ping; returns 429 if quota is 0.0 before reset, or 200 with usage metadata and updates `warmup_fired = True`.
  - `POST /token`: Simulates Google OAuth2 refresh endpoint.
  - `/test_control/set_quota`: Updates global remaining fraction and reset countdown.
  - `/test_control/advance_time`: Simulates time progression.
  - `/test_control/simulate_transient_error`: Simulates transient 503 or 429 responses.
- **Limitation Observed**: Currently, `MockCloudCodeServer` only holds a single global `current_quota_fraction` (line 323). When testing multi-account auto-switching (e.g. Account A at 2%, Account B at 95%), requests from both accounts receive the same global quota unless updated per request.

### 1.4 Test Suite Baseline
- `pytest` execution (`task-54`) passed 335 of 335 existing tests in 25.63s across unit, stress, and tier 1-4 suites.

---

## 2. Logic Chain

### 2.1 Feature F09: Auto-Switch Rule Engine (`antigravity_swiss/quota/rule_engine.py`)

1. **Threshold Evaluation Semantics**:
   - Quotas are represented as floating-point fractions $\in [0.0, 1.0]$.
   - Inputs must be strictly clamped: $\text{fraction}_{\text{clamped}} = \max(0.0, \min(1.0, \text{fraction}_{\text{raw}}))$.
   - Trigger condition: An auto-switch is triggered when:
     $$\text{remainingFraction} \le \text{threshold}$$
   - Configurable per-model threshold:
     - High-demand models (e.g. `gemini-3.1-pro`, `claude-sonnet-4-6`) may require higher threshold (e.g. 15% / 0.15) due to large token consumption per prompt.
     - Fast models (e.g. `gemini-3.8-flash`, `gemini-3.5-flash-lite`) use standard threshold (default 5% / 0.05).
     - Global fallback: If no per-model threshold is set, default threshold `0.05` applies.
   - Dual-Window Depletion Awareness:
     - Google upstream organizes models into a 5-hour rolling burst window (`gemini-5h`, `3p-5h`) and a 7-day weekly tier window (`gemini-weekly`, `3p-weekly`).
     - If either the 5-hour fraction $\le \text{threshold}$ OR the weekly fraction $\le 0.01$ (weekly exhausted), the account is deemed depleted. Switching is mandatory because Google rejects generation (HTTP 429 `RESOURCE_EXHAUSTED`) on weekly depletion regardless of 5h availability.

2. **Account Eligibility and Multi-Tiered Selection Algorithm**:
   - Let $\mathcal{A}$ be the set of accounts in `AccountVault`.
   - Filter candidates $\mathcal{C} \subset \mathcal{A}$:
     $$\mathcal{C} = \{ a \in \mathcal{A} \mid a.\text{email} \ne \text{active\_account} \land a.\text{is\_healthy} = \text{True} \land \neg \text{in\_cooldown}(a) \land \text{quota}(a) > \text{threshold} + \text{margin} \}$$
   - Model Tier Weights:
     Different models have different utility in Antigravity:
     - Tier 1: Gemini 3.8 Flash (`gemini-3.8-flash-tiered` / `gemini-5h` bucket): Weight $W_{\text{flash}} = 0.40$ (primary reasoning workhorse)
     - Tier 2: Gemini 3.1 Pro (`gemini-3.1-pro-low` / `pro`): Weight $W_{\text{pro}} = 0.30$ (deep reasoning / complex architecture)
     - Tier 3: Claude Sonnet (`claude-sonnet-4-6` / `3p-5h` bucket): Weight $W_{\text{claude}} = 0.20$ (code generation)
     - Tier 4: Gemini Flash Lite (`gemini-3.5-flash-lite` / `flashLite`): Weight $W_{\text{lite}} = 0.10$ (fast tool invocation / pings)
   - Composite Eligibility Score:
     $$\text{Score}(a) = W_{\text{flash}} \cdot Q_{\text{flash}}(a) + W_{\text{pro}} \cdot Q_{\text{pro}}(a) + W_{\text{claude}} \cdot Q_{\text{claude}}(a) + W_{\text{lite}} \cdot Q_{\text{lite}}(a)$$
   - Tie-Breaking Order:
     1. Highest Composite Score $\text{Score}(a)$.
     2. Highest Weekly Limit Remaining $Q_{\text{weekly}}(a)$.
     3. Least Recently Used (`last_used_at` oldest or `None`).

3. **Anti-Thrashing Guardrails & Cooldown Protection**:
   - Problem: If Account A has 4% quota, switches to Account B with 6%, and Account B immediately drops to 4%, an unconstrained engine will bounce back to Account A, entering an infinite restart loop and freezing the desktop application.
   - Guardrail 1: **Per-Account Cooldown** (`cooldown_seconds`, default 300.0s / 5 min). An account that was switched out cannot be switched back in until cooldown elapses, UNLESS its `resetTime` has arrived and a successful keep-alive warmup has recharged its quota above $0.50$.
   - Guardrail 2: **Minimum Switch Margin / Hysteresis** (`switch_margin`, default 0.05). A candidate account is only eligible if its remaining fraction $> \text{threshold} + \text{switch\_margin}$ (e.g. $0.05 + 0.05 = 0.10$).
   - Guardrail 3: **Switch Rate Limiter** (`max_switches_in_window = 3` in `switch_window_seconds = 600.0s`). If more than 3 switches occur within 10 minutes, auto-switch enters throttled state and notifies user.
   - Guardrail 4: **All-Exhausted State**: If all standby accounts are depleted or in cooldown ($\mathcal{C} = \emptyset$), the engine MUST NOT crash or thrash. It logs a warning, emits `notify.all_accounts_exhausted`, calculates the nearest `resetTime` across all accounts, and pauses auto-switching until that time.

---

### 2.2 Feature F26: Offline Mock CloudCode Server (`tests/fixtures/mock_cloudcode_server.py`)

1. **Deterministic Multi-Account Quota Profiles**:
   - To test multi-account selection without network access, `MockCloudCodeServer` must map incoming Bearer tokens to distinct account profiles.
   - Internal mapping:
     `account_profiles: dict[str, MockAccountProfile]`
   - Profile structure:
     - `remaining_fraction: float`
     - `weekly_remaining_fraction: float`
     - `reset_time: datetime`
     - `warmup_fired: bool`
     - `token_expired: bool`
   - Convenience presets:
     - `set_account_profile(token="ya29.alice", remaining=0.02, reset_in_seconds=120)`
     - `apply_profile_preset(token="ya29.bob", preset="healthy")` (remaining=1.0)
     - `apply_profile_preset(token="ya29.carol", preset="depleted")` (remaining=0.0)
     - `apply_profile_preset(token="ya29.dave", preset="weekly_exhausted")` (remaining=1.0, weekly=0.0)

2. **1-Token Keep-Alive Warmup Simulation**:
   - Incoming request to `POST /v1internal:generateContent` with body containing `maxOutputTokens: 1`.
   - If profile has `remaining_fraction <= 0.0` and `now < reset_time`: Return HTTP 429 `RESOURCE_EXHAUSTED`.
   - If `now >= reset_time`:
     - Transition profile: `warmup_fired = True`, `remaining_fraction = 1.0`, `reset_time = now + 5 hours`.
     - Return HTTP 200 with valid `candidates` and `usageMetadata` (`promptTokenCount: 1, candidatesTokenCount: 1, totalTokenCount: 2`).

3. **Clock Drift Calibration**:
   - `MockCloudCodeServer` injects HTTP `Date` header calculated as `simulated_now + timedelta(seconds=clock_drift_seconds)`.
   - Control endpoint `/test_control/set_clock_drift` sets deliberate drift (e.g. +3.5s) to verify client-side drift compensation.

---

### 2.3 IPC & Background Daemon Integration

1. **JSON-RPC Methods**:
   | RPC Method | Request Params | Response Structure | Purpose |
   |---|---|---|---|
   | `quota.get_summary` | `{"account": str \| null}` | `{"account": str, "groups": list, "timestamp": str, "is_healthy": bool}` | Returns cached or live quota summary |
   | `quota.poll_now` | `{"account": str \| null}` | `{"account": str, "summary": dict, "switched": bool}` | Forces immediate poll and rule engine evaluation |
   | `rules.get_config` | `{}` | `{"auto_switch_enabled": bool, "auto_switch_threshold": float, "cooldown_seconds": float, ...}` | Returns rule engine configuration |
   | `rules.set_config` | `{"auto_switch_threshold": 0.05, ...}` | `{"success": bool, "config": dict}` | Updates rule engine config and saves to `settings.json` |

2. **Pub-Sub Notifications**:
   | Notification Event | Trigger Condition | Payload Schema |
   |---|---|---|
   | `notify.quota_updated` | Poller completes poll cycle | `{"account": str, "remaining_fraction": float, "groups": list, "timestamp": str}` |
   | `notify.account_switched` | Auto-switch or manual switch completes | `{"previous_account": str, "active_account": str, "reason": str, "relaunch_pid": int \| null, "timestamp": str}` |
   | `notify.warmup_triggered` | Warmup ping dispatched upon reset horizon | `{"account": str, "model_id": str, "success": bool, "next_window": str, "timestamp": str}` |
   | `notify.all_accounts_exhausted` | All accounts below threshold | `{"active_account": str, "earliest_reset": str, "timestamp": str}` |

---

## 3. Detailed Architectural Blueprint & Implementation Specification

### 3.1 Specification: `antigravity_swiss/quota/rule_engine.py`

```python
"""
Auto-Switch Rule Engine & Account Eligibility Selector (Feature F09).
======================================================================
Evaluates model quota thresholds, sorts accounts by tiered model health,
and triggers atomic keyring switching with anti-thrashing cooldown guardrails.
"""

from __future__ import annotations

import collections
from dataclasses import dataclass, field
import datetime
import logging
import time
from typing import Any, Callable, Dict, List, Optional, Tuple

from antigravity_swiss.core.constants import (
    DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION,
)
from antigravity_swiss.core.errors import SwissKnifeError
from antigravity_swiss.keyring.switcher import AccountRecord, AccountVault, KeyringService

logger = logging.getLogger("antigravity_swiss.quota.rule_engine")


@dataclass
class RuleEngineConfig:
    """Configuration for quota auto-switching and anti-thrashing guardrails."""
    enabled: bool = True
    default_threshold: float = DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION  # 0.05 (5%)
    per_model_thresholds: Dict[str, float] = field(default_factory=lambda: {
        "gemini-3.8-flash": 0.05,
        "gemini-3.5-flash-lite": 0.05,
        "gemini-3.1-pro": 0.15,
        "claude-sonnet-4-6": 0.10,
    })
    cooldown_seconds: float = 300.0           # 5 minutes minimum between switches to same account
    switch_margin: float = 0.05              # Target must have >= (threshold + margin)
    max_switches_in_window: int = 3          # Maximum switches before rate limiting
    switch_window_seconds: float = 600.0      # 10 minutes rolling rate limit window
    weekly_threshold: float = 0.01           # Weekly quota below 1% treated as exhausted
    tier_weights: Dict[str, float] = field(default_factory=lambda: {
        "flash": 0.40,
        "pro": 0.30,
        "claude": 0.20,
        "flash_lite": 0.10,
    })

    def get_threshold_for_model(self, model_id: str) -> float:
        """Resolve threshold for model ID with fuzzy prefix matching."""
        if not model_id:
            return self.default_threshold
        for key, val in self.per_model_thresholds.items():
            if key in model_id:
                return max(0.0, min(1.0, val))
        return max(0.0, min(1.0, self.default_threshold))


@dataclass
class EvaluationResult:
    """Outcome of rule engine threshold evaluation."""
    should_switch: bool
    reason: str
    current_account: Optional[str]
    target_account: Optional[str] = None
    current_fraction: float = 1.0
    threshold: float = 0.05
    target_score: float = 0.0
    all_exhausted: bool = False
    cooldown_active: bool = False
    details: Dict[str, Any] = field(default_factory=dict)


class AutoSwitchRuleEngine:
    """
    Evaluates account quotas against configured thresholds and selects the best
    standby account using multi-model tiered scoring and cooldown guardrails.
    """

    def __init__(
        self,
        vault: AccountVault,
        keyring_service: KeyringService,
        config: Optional[RuleEngineConfig] = None,
        clock_fn: Callable[[], float] = time.time,
    ) -> None:
        self.vault = vault
        self.keyring_service = keyring_service
        self.config = config or RuleEngineConfig()
        self._clock = clock_fn
        self._last_switch_out_times: Dict[str, float] = {}  # account_email -> timestamp
        self._switch_history: collections.deque[float] = collections.deque()
        self._account_quota_cache: Dict[str, Any] = {}

    def update_cached_quota(self, email: str, quota_summary: Any) -> None:
        """Store latest quota summary for an account."""
        self._account_quota_cache[email] = quota_summary

    def is_account_in_cooldown(self, email: str) -> bool:
        """Check if an account was recently switched away from."""
        last_out = self._last_switch_out_times.get(email)
        if last_out is None:
            return False
        return (self._clock() - last_out) < self.config.cooldown_seconds

    def is_rate_limited(self) -> bool:
        """Check if global switch rate limit is exceeded."""
        now = self._clock()
        # Evict timestamps older than window
        while self._switch_history and (now - self._switch_history[0]) > self.config.switch_window_seconds:
            self._switch_history.popleft()
        return len(self._switch_history) >= self.config.max_switches_in_window

    def record_switch(self, from_email: str, to_email: str) -> None:
        """Record switch event for cooldown and rate limiting."""
        now = self._clock()
        if from_email:
            self._last_switch_out_times[from_email] = now
        self._switch_history.append(now)

    def extract_remaining_fractions(self, quota_data: Any) -> Dict[str, float]:
        """
        Extract normalized model and bucket fractions from QuotaSummary or dict.
        Returns dict with keys: 'flash', 'pro', 'claude', 'flash_lite', 'weekly_gemini', 'weekly_3p'.
        """
        fractions = {
            "flash": 1.0,
            "pro": 1.0,
            "claude": 1.0,
            "flash_lite": 1.0,
            "weekly_gemini": 1.0,
            "weekly_3p": 1.0,
        }
        if not quota_data:
            return fractions

        # Handle dict or QuotaSummary object
        groups = getattr(quota_data, "groups", None)
        if groups is None and isinstance(quota_data, dict):
            groups = quota_data.get("groups", [])

        if isinstance(groups, list):
            for group in groups:
                buckets = group.get("buckets", []) if isinstance(group, dict) else getattr(group, "buckets", [])
                for b in buckets:
                    b_id = b.get("bucketId", "") if isinstance(b, dict) else getattr(b, "bucket_id", "")
                    rem = b.get("remainingFraction", 1.0) if isinstance(b, dict) else getattr(b, "remaining_fraction", 1.0)
                    rem = max(0.0, min(1.0, float(rem)))

                    if b_id == "gemini-5h":
                        fractions["flash"] = rem
                        fractions["pro"] = rem
                        fractions["flash_lite"] = rem
                    elif b_id == "gemini-weekly":
                        fractions["weekly_gemini"] = rem
                    elif b_id == "3p-5h":
                        fractions["claude"] = rem
                    elif b_id == "3p-weekly":
                        fractions["weekly_3p"] = rem

        return fractions

    def calculate_account_score(self, email: str, quota_data: Any) -> float:
        """
        Compute weighted multi-model score for an account:
        Score = W_flash*Q_flash + W_pro*Q_pro + W_claude*Q_claude + W_lite*Q_lite
        Penalizes heavily if weekly quota is depleted.
        """
        fractions = self.extract_remaining_fractions(quota_data)
        
        # If weekly quota is exhausted, account score drops to 0.0
        if fractions["weekly_gemini"] <= self.config.weekly_threshold or fractions["weekly_3p"] <= self.config.weekly_threshold:
            return 0.0

        weights = self.config.tier_weights
        score = (
            weights.get("flash", 0.4) * fractions["flash"]
            + weights.get("pro", 0.3) * fractions["pro"]
            + weights.get("claude", 0.2) * fractions["claude"]
            + weights.get("flash_lite", 0.1) * fractions["flash_lite"]
        )
        return round(score, 4)

    def select_best_standby_account(
        self,
        current_email: str,
        threshold: float,
    ) -> Tuple[Optional[str], float, bool]:
        """
        Select highest scoring standby account meeting threshold and margin.
        Returns: (best_email, best_score, all_exhausted)
        """
        records = self.vault.list_account_records()
        candidates: List[Tuple[str, float, AccountRecord]] = []
        has_healthy_accounts = False

        for rec in records:
            if rec.email == current_email:
                continue
            if not rec.is_healthy:
                continue

            has_healthy_accounts = True
            if self.is_account_in_cooldown(rec.email):
                continue

            q_data = self._account_quota_cache.get(rec.email)
            fractions = self.extract_remaining_fractions(q_data)
            burst_remaining = fractions.get("flash", 1.0)
            weekly_remaining = fractions.get("weekly_gemini", 1.0)

            # Eligibility requirements:
            # 1. Burst remaining must exceed threshold + switch_margin
            # 2. Weekly remaining must exceed weekly_threshold
            required_min = threshold + self.config.switch_margin
            if burst_remaining <= required_min or weekly_remaining <= self.config.weekly_threshold:
                continue

            score = self.calculate_account_score(rec.email, q_data)
            candidates.append((rec.email, score, rec))

        if not candidates:
            return None, 0.0, True

        # Sort candidates: highest score -> highest weekly -> oldest last_used_at
        def _sort_key(item: Tuple[str, float, AccountRecord]):
            email, score, rec = item
            q_data = self._account_quota_cache.get(email)
            weekly = self.extract_remaining_fractions(q_data)["weekly_gemini"]
            last_used = rec.last_used_at or ""
            # Invert score and weekly for descending sort; last_used ascending
            return (-score, -weekly, last_used)

        candidates.sort(key=_sort_key)
        best_email, best_score, _ = candidates[0]
        return best_email, best_score, False

    def evaluate(
        self,
        current_email: str,
        current_quota_data: Any,
        active_model_id: str = "gemini-3.8-flash-high",
    ) -> EvaluationResult:
        """
        Main evaluation entry point.
        Checks if active account's quota breaches threshold and finds best successor.
        """
        if not self.config.enabled:
            return EvaluationResult(
                should_switch=False,
                reason="Auto-switch disabled in settings",
                current_account=current_email,
            )

        threshold = self.config.get_threshold_for_model(active_model_id)
        fractions = self.extract_remaining_fractions(current_quota_data)
        current_burst = fractions.get("flash", 1.0)
        current_weekly = fractions.get("weekly_gemini", 1.0)

        # Update cache for current account
        self.update_cached_quota(current_email, current_quota_data)

        # Check breach conditions
        burst_breached = current_burst <= threshold
        weekly_breached = current_weekly <= self.config.weekly_threshold

        if not burst_breached and not weekly_breached:
            return EvaluationResult(
                should_switch=False,
                reason="Quota healthy",
                current_account=current_email,
                current_fraction=current_burst,
                threshold=threshold,
            )

        # Check rate limiting guardrail
        if self.is_rate_limited():
            logger.warning("Auto-switch throttled: exceeded %d switches in %ds",
                           self.config.max_switches_in_window, self.config.switch_window_seconds)
            return EvaluationResult(
                should_switch=False,
                reason="Rate limit exceeded (thrashing protection active)",
                current_account=current_email,
                current_fraction=current_burst,
                threshold=threshold,
                cooldown_active=True,
            )

        # Select best successor
        best_email, best_score, all_exhausted = self.select_best_standby_account(current_email, threshold)

        if all_exhausted or not best_email:
            reason = "All standby accounts exhausted or below threshold"
            logger.warning("%s (active account: %s)", reason, current_email)
            return EvaluationResult(
                should_switch=False,
                reason=reason,
                current_account=current_email,
                current_fraction=current_burst,
                threshold=threshold,
                all_exhausted=True,
            )

        trigger_reason = "Weekly quota exhausted" if weekly_breached else f"Quota ({current_burst:.2%}) below threshold ({threshold:.2%})"
        return EvaluationResult(
            should_switch=True,
            reason=trigger_reason,
            current_account=current_email,
            target_account=best_email,
            current_fraction=current_burst,
            threshold=threshold,
            target_score=best_score,
        )
```

---

### 3.2 Specification: Offline Mock Server Extensions (`tests/fixtures/mock_cloudcode_server.py`)

Enhance `MockCloudCodeServer` and `MockCloudCodeHandler` to support:
1. `account_profiles: Dict[str, Dict[str, Any]]`:
   - Keyed by access token (e.g. `ya29.alice`, `ya29.bob`).
   - Stores `current_quota_fraction`, `weekly_quota_fraction`, `reset_time`, `warmup_fired`.
2. New test control endpoints:
   - `POST /test_control/set_account_quota`:
     ```json
     {
       "token": "ya29.alice",
       "remaining": 0.02,
       "reset_in_seconds": 60,
       "weekly_remaining": 1.0
     }
     ```
   - `POST /test_control/set_clock_drift`:
     ```json
     {
       "drift_seconds": 3.5
     }
     ```
   - `POST /test_control/apply_preset`:
     ```json
     {
       "token": "ya29.alice",
       "preset": "depleted"
     }
     ```
3. Handlers update:
   - `_get_profile_for_token(auth_header: str)`: Extracts token, looks up `account_profiles.get(token)`. If absent, returns default profile.
   - `:retrieveUserQuotaSummary`: Uses token-specific profile to format `remainingFraction` and `resetTime`.
   - `:generateContent`: Validates token-specific quota; updates token-specific `warmup_fired = True` and resets token-specific fraction to 1.0.

---

### 3.3 Specification: Daemon IPC & Controller Integration

#### 1. JSON-RPC Methods in `__main__.py`

```python
    # Quota Summary
    @server.register("quota.get_summary")
    async def rpc_quota_summary(account: Optional[str] = None) -> dict[str, Any]:
        target = account or poller.active_account_email
        summary = poller.get_cached_summary(target)
        if not summary:
            summary = await poller.poll_account(target)
        return summary.to_dict() if hasattr(summary, "to_dict") else summary

    # Immediate Poll Trigger
    @server.register("quota.poll_now")
    async def rpc_quota_poll_now(account: Optional[str] = None) -> dict[str, Any]:
        target = account or poller.active_account_email
        summary = await poller.poll_account(target)
        # Evaluate auto-switch rule engine immediately
        eval_result = rule_engine.evaluate(target, summary)
        switched = False
        if eval_result.should_switch and eval_result.target_account:
            switched = await asyncio.to_thread(
                keyring_switcher.switch_to_account, eval_result.target_account
            )
            rule_engine.record_switch(target, eval_result.target_account)
            server.broadcast_event_threadsafe("notify.account_switched", {
                "previous_account": target,
                "active_account": eval_result.target_account,
                "reason": eval_result.reason,
            })
        server.broadcast_event_threadsafe("notify.quota_updated", {
            "account": target,
            "summary": summary.to_dict() if hasattr(summary, "to_dict") else summary,
        })
        return {
            "account": target,
            "summary": summary.to_dict() if hasattr(summary, "to_dict") else summary,
            "switched": bool(switched),
            "active_account": eval_result.target_account if switched else target,
        }

    # Rules Get Config
    @server.register("rules.get_config")
    def rpc_rules_get_config() -> dict[str, Any]:
        return {
            "auto_switch_enabled": config.auto_switch_enabled,
            "auto_switch_threshold": config.auto_switch_threshold,
            "cooldown_seconds": rule_engine.config.cooldown_seconds,
            "switch_margin": rule_engine.config.switch_margin,
            "per_model_thresholds": rule_engine.config.per_model_thresholds,
            "max_switches_in_window": rule_engine.config.max_switches_in_window,
            "switch_window_seconds": rule_engine.config.switch_window_seconds,
        }

    # Rules Set Config
    @server.register("rules.set_config")
    def rpc_rules_set_config(**kwargs: Any) -> dict[str, Any]:
        if "auto_switch_enabled" in kwargs:
            config.auto_switch_enabled = bool(kwargs["auto_switch_enabled"])
            rule_engine.config.enabled = config.auto_switch_enabled
        if "auto_switch_threshold" in kwargs:
            val = float(kwargs["auto_switch_threshold"])
            config.auto_switch_threshold = max(0.0, min(1.0, val))
            rule_engine.config.default_threshold = config.auto_switch_threshold
        if "cooldown_seconds" in kwargs:
            rule_engine.config.cooldown_seconds = max(0.0, float(kwargs["cooldown_seconds"]))
        if "per_model_thresholds" in kwargs and isinstance(kwargs["per_model_thresholds"], dict):
            rule_engine.config.per_model_thresholds.update(kwargs["per_model_thresholds"])
        
        config.save_settings()
        return rpc_rules_get_config()
```

#### 2. Updates to `antigravity_swiss/ipc/controller.py`

Add method signatures to `SwissKnifeController`:
- `poll_quota_now(account: Optional[str] = None) -> dict[str, Any]`
- `get_rule_config() -> dict[str, Any]`
- `set_rule_config(**kwargs: Any) -> dict[str, Any]`

In `RemoteDaemonController`:
- `poll_quota_now`: `return self._client.call("quota.poll_now", {"account": account})`
- `get_rule_config`: `return self._client.call("rules.get_config")`
- `set_rule_config`: `return self._client.call("rules.set_config", kwargs)`

In `StandaloneController`:
- Provide direct in-process access to `SwissKnifeConfig` for `get_rule_config` and `set_rule_config`.

---

## 4. Caveats

1. **Standby Account Quota Freshness**:
   - Standby accounts' quotas are only known if they have been polled at least once or cached in memory.
   - If a new standby account is registered, its quota should be initialized optimistically (e.g. 1.0) or polled via a low-frequency background discovery poll so that eligibility ranking has accurate data.
2. **Weekly Quota Trap**:
   - An account whose 5-hour rolling burst quota has reset to 100% may still be blocked if its 7-day weekly tier limit is depleted. The rule engine must evaluate both windows to avoid switching into a dead account.
3. **Multi-Client Pub-Sub Flow**:
   - GUI widgets (e.g. `QuotaDashboard`, `SwitcherSettings`) connect via `AsyncDaemonClient` or `QLocalSocket`. Event notifications (`notify.quota_updated`, `notify.account_switched`) must be dispatched as fire-and-forget JSON-RPC notifications (`id: null`) to avoid blocking IPC reader threads.
4. **Testing Process Relaunches**:
   - In automated tests or under `ANTIGRAVITY_SWISS_TESTING=1`, `ProcessManager` must never terminate host Antigravity processes. Auto-switch tests should verify keyring rotation and mock relaunch callbacks without signalling `/proc`.

---

## 5. Conclusion

1. **`antigravity_swiss/quota/rule_engine.py`**:
   - Designed with strict clamping $[0.0, 1.0]$, per-model threshold matching, dual-window (5h burst + weekly) checks, multi-model weighted scoring (Flash 0.40, Pro 0.30, Claude 0.20, Flash Lite 0.10), and 4 anti-thrashing guardrails (300s cooldown, 0.05 margin, 3-switch rate limit, and graceful all-exhausted standby).
2. **`tests/fixtures/mock_cloudcode_server.py`**:
   - Extended with per-token / per-account quota profiles, multi-account warmup state tracking, clock drift simulation, and preset profiles (`healthy`, `depleted`, `weekly_exhausted`) ensuring 100% hermetic offline testability.
3. **IPC & Daemon Integration**:
   - Defined 4 JSON-RPC methods (`quota.get_summary`, `quota.poll_now`, `rules.get_config`, `rules.set_config`) and 4 broadcast events (`notify.quota_updated`, `notify.account_switched`, `notify.warmup_triggered`, `notify.all_accounts_exhausted`), integrating seamlessly with `SwissKnifeController`.

---

## 6. Verification Method

1. **Unit Test Verification**:
   - Execute unit tests verifying `AutoSwitchRuleEngine`:
     ```bash
     pytest tests/unit/test_quota_rule_engine.py -v
     ```
   - Verify threshold boundaries (0.0, 1.0, negative clamping, > 1.0 clamping).
   - Verify multi-account eligibility ranking and tie-breaking.
   - Verify cooldown prevention and rate-limiting anti-thrashing.

2. **Offline Mock Server Verification**:
   - Execute pairwise and feature tests against `mock_cloudcode_server`:
     ```bash
     pytest tests/e2e/test_tier1_features.py -k "f09 or f26" -v
     ```
   - Confirm zero external network access and zero API token burns.

3. **IPC Round-Trip Verification**:
   - Start test daemon socket and verify JSON-RPC methods:
     ```bash
     pytest tests/unit/test_ipc.py -v
     ```
   - Confirm pub-sub delivery of `notify.account_switched` and `notify.quota_updated`.

4. **Full Test Suite Regression**:
   ```bash
   pytest
   ```
   - Confirm 335+ tests pass with zero errors.
