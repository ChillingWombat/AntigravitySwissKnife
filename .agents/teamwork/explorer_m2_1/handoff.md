# Milestone 2 Quota Poller & Model Catalog Explorer Blueprint

**Agent**: `explorer_m2_1`  
**Milestone**: M2 - Upstream Quota Poller & Model Catalog Fetcher (Features F06, F07)  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m2_1`  
**Target Handoff**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m2_1/handoff.md`  
**Timestamp**: `2026-10-02T09:10:30Z`  

---

## 1. Observation

Direct extraction from authoritative specifications (`.agents/teamwork/spec_miner_quota_1/handoff.md`), codebase files (`PROJECT.md`, `antigravity_swiss/core/`, `antigravity_swiss/keyring/`, `antigravity_swiss/ipc/`), mock fixtures (`tests/fixtures/mock_cloudcode_server.py`), and local environment inspection:

### 1.1 Python Runtime & Dependency Environment
- **Python Version**: `Python 3.12.0` (at `/usr/bin/python3`).
- **HTTP Libraries Available**:
  - `urllib.request`, `urllib.error`, `urllib.parse`, `http.client`: **AVAILABLE** (Standard Library).
  - `aiohttp`, `httpx`, `requests`: **NOT INSTALLED** (`ImportError`).
  - **Conclusion**: All network communication must use Python standard library (`urllib.request` / `urllib.error` / `http.client`). In asynchronous contexts (such as the daemon event loop), requests must execute via `asyncio.to_thread` to ensure non-blocking I/O without introducing external dependencies.
- **Test Runner**: `/usr/bin/pytest` (pytest 9.0.2). Current baseline: 24 unit tests and 299 e2e tests passing cleanly.

### 1.2 Upstream CloudCode Protocol & Endpoints
From upstream binary reverse-engineering in `spec_miner_quota_1/handoff.md`:
- **Base URL**: `https://cloudcode-pa.googleapis.com` (configurable for testing to `http://127.0.0.1:<port>`).
- **Endpoints**:
  1. `POST /v1internal:retrieveUserQuotaSummary`:
     - Request Headers:
       - `Authorization: Bearer <access_token>`
       - `Content-Type: application/json`
       - `User-Agent: antigravity/2.18.1 linux/amd64`
     - Request Body: `{"project": ""}`
     - Response: JSON with `groups` containing `buckets` (`bucketId`, `displayName`, `window`, `remainingFraction`, `resetTime`, `description`), and root `description`.
  2. `POST /v1internal:fetchAvailableModels`:
     - Request Headers: Same as above.
     - Request Body: `{"project": ""}`
     - Response: JSON with `models` mapping model IDs to `ModelDetails` (`displayName`, `supportsImages`, `supportsThinking`, `maxTokens`, `maxOutputTokens`, `quotaInfo`), `defaultAgentModelId`, and `tieredModelIds` (`flashLite`, `flash`, `pro`).
  3. `POST /token` or `https://oauth2.googleapis.com/token`:
     - Request Body (JSON or Form-URL-Encoded):
       `client_id`, `client_secret`, `refresh_token`, `grant_type="refresh_token"`
     - Response: `{"access_token": "ya29...", "expires_in": 3600, "token_type": "Bearer"}`.
- **Server Clock Drift Observation**:
  Upstream responses include RFC 7231 / HTTP `Date` header: e.g., `Thu, 01 Oct 2026 05:04:53 GMT`. Clock drift between Google server and local host may range up to several seconds:
  $$\Delta t = t_{\text{Google}} - t_{\text{local}}$$

### 1.3 Keyring Integration Context
- Secret Service attributes: `service=gemini`, `username=antigravity`.
- Stored JSON structure has `token` containing `access_token`, `refresh_token`, `expiry`, and `token_type`, plus `id_token` and `auth_method`.
- Keyring classes in `antigravity_swiss.keyring.switcher`:
  - `KeyringCredential`: models credentials, serializes/deserializes Antigravity JSON.
  - `KeyringService`: reads/writes active credentials in Secret Service (`get_active_credential()`, `set_active_credential()`).
  - `AccountVault`: stores multi-account credentials in `~/.config/antigravity-swiss/accounts.json` with file locking.

---

## 2. Logic Chain

1. **Module Separation**:
   To satisfy single-responsibility and maintain testability, M2 quota logic must be decomposed into three files under `antigravity_swiss/quota/`:
   - `models.py`: Immutable data models, deserializers, and serializers for buckets, groups, summaries, and model catalogs.
   - `client.py`: Low-level standard library HTTP client handling endpoints, headers, clock drift tracking, token refresh, and error mapping.
   - `poller.py`: High-level background daemon service managing polling loops, dynamic token resolution, in-memory caching, exponential backoff with jitter, and event notifications.

2. **Standard Library Networking Strategy (`urllib` + `asyncio.to_thread`)**:
   - Because `aiohttp` and `httpx` are not installed, and third-party dependencies are restricted, `urllib.request` provides zero-dependency compatibility.
   - To avoid blocking the daemon's `asyncio` event loop during network requests, `client.py` provides synchronous methods (`retrieve_user_quota_summary_sync`, `fetch_available_models_sync`) and async wrappers (`retrieve_user_quota_summary`, `fetch_available_models`) executed via `asyncio.to_thread`.
   - Timeout handling uses `socket.setdefaulttimeout` or explicit `timeout` parameter in `urllib.request.urlopen(req, timeout=...)`.

3. **Data Model Architecture (`models.py`)**:
   - `ModelQuotaBucket`: Dataclass capturing `bucket_id`, `display_name`, `window` ("5h" | "weekly"), `remaining_fraction` (0.0 to 1.0), `reset_time` (UTC datetime), `description`, `disabled`, and `remaining_amount`.
     - Provides `seconds_until_reset(now)` and `is_exhausted(threshold)`.
     - Supports `to_dict()` and `from_dict()` with ISO 8601 string formatting.
   - `QuotaSummaryGroup`: Dataclass capturing group `display_name` ("Gemini Models", "Claude and GPT models"), `description`, and `buckets: list[ModelQuotaBucket]`.
   - `QuotaSummary`: Dataclass containing `groups: list[QuotaSummaryGroup]`, `description`, `timestamp`, `active_account`, and `server_time_drift_seconds`.
     - Convenience accessors: `.gemini_5h`, `.gemini_weekly`, `.p3_5h`, `.p3_weekly`, `.lowest_remaining_fraction`, `.is_exhausted(threshold)`.
   - `ModelDetails`: Dataclass capturing capabilities (`supports_images`, `supports_thinking`, `max_tokens`, `max_output_tokens`) and inline `quota_info` (`remaining_fraction`, `reset_time`).
   - `TieredModelConfig`: Dataclass capturing `flash_lite: list[str]`, `flash: list[str]`, `pro: list[str]`.
   - `ModelCatalog`: Dataclass storing `models: dict[str, ModelDetails]`, `default_agent_model_id`, `tiered_model_ids: TieredModelConfig`, and `timestamp`.

4. **Error Taxonomy & Exception Handling**:
   - `QuotaError` (from `core.errors`) is the base class.
   - Specific exceptions:
     - `QuotaAuthExpiredError`: HTTP 401 unauthenticated, triggers token refresh.
     - `QuotaRateLimitError`: HTTP 429 rate limit or quota exhaustion.
     - `QuotaUnavailableError`: HTTP 503 / 502 / 504 transient gateway outage.
     - `QuotaNetworkError`: DNS, connection refusal, or socket timeout.
   - All errors map cleanly to JSON-RPC 2.0 error codes (`-32030` to `-32034`).

5. **Token Lifecycle & Refresh Flow**:
   - **Proactive Refresh**: `poller.py` inspects `credential.expiry`. If token expires within 120 seconds, it refreshes the token before making the quota call.
   - **Reactive Refresh**: If an API call receives HTTP 401, `poller.py` catches `QuotaAuthExpiredError`, requests a new access token via `client.refresh_access_token(cred.refresh_token)`, updates `KeyringService.set_active_credential()`, updates `AccountVault`, and immediately retries the quota request.

6. **Caching, Jitter & Exponential Backoff**:
   - In-memory cache stores the latest `QuotaSummary` with configurable TTL (default 15s). Repeated calls to `poll_summary()` within TTL return cached data without hitting Google.
   - Polling loop runs at `poll_interval_sec` (default 30s for GUI, 60s for daemon) with randomized jitter ($\pm 1.5$s) to prevent harmonic network spikes.
   - On transient errors (429, 503, network failure), the loop enters exponential backoff:
     $$\text{backoff} = \min(60.0, 2.0 \times 1.5^{\text{consecutive\_errors}}) + \text{Uniform}(0.2, 1.5)$$

7. **IPC Daemon Wiring**:
   - Daemon registers RPC methods:
     - `quota.get_summary` -> calls `poller.poll_summary()`, returns `summary.to_dict()`.
     - `models.get_catalog` -> calls `poller.fetch_models()`, returns `catalog.to_dict()`.
   - When quota updates, poller triggers callback `server.broadcast_event_threadsafe("notify.quota_updated", {"summary": summary.to_dict()})`.

---

## 3. Detailed Implementation Blueprint

### 3.1 `antigravity_swiss/quota/models.py`

```python
"""
Data structures for Quota Summary, Quota Buckets, and Model Catalog.
===================================================================
Conforms strictly to PROJECT.md § Interface Contracts and Google CloudCode Protobuf specs.
"""

from __future__ import annotations

from dataclasses import asdict, dataclass, field
import datetime
from typing import Any, Dict, List, Optional


def parse_rfc3339_timestamp(ts: Optional[str]) -> Optional[datetime.datetime]:
    """Parse RFC 3339 / ISO 8601 UTC timestamp string to datetime.datetime."""
    if not ts or not isinstance(ts, str):
        return None
    try:
        clean_ts = ts.replace("Z", "+00:00")
        dt = datetime.datetime.fromisoformat(clean_ts)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=datetime.timezone.utc)
        return dt
    except (ValueError, TypeError):
        return None


def format_rfc3339_timestamp(dt: Optional[datetime.datetime]) -> Optional[str]:
    """Format datetime to RFC 3339 UTC string."""
    if dt is None:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=datetime.timezone.utc)
    return dt.astimezone(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


@dataclass
class ModelQuotaBucket:
    """
    Representation of an individual quota bucket (e.g. gemini-5h, gemini-weekly).
    Protobuf: google.internal.cloud.code.v1internal.QuotaSummaryBucket
    """
    bucket_id: str
    display_name: str
    window: str  # "5h" | "weekly"
    remaining_fraction: float  # 0.0 to 1.0
    reset_time: Optional[datetime.datetime] = None
    description: str = ""
    disabled: bool = False
    remaining_amount: Optional[int] = None

    def seconds_until_reset(self, reference_time: Optional[datetime.datetime] = None) -> Optional[float]:
        """Calculate seconds remaining until quota resets."""
        if self.reset_time is None:
            return None
        now = reference_time or datetime.datetime.now(datetime.timezone.utc)
        if now.tzinfo is None:
            now = now.replace(tzinfo=datetime.timezone.utc)
        return max(0.0, (self.reset_time - now).total_seconds())

    def is_exhausted(self, threshold_fraction: float = 0.05) -> bool:
        """Check if bucket quota is below or equal to threshold."""
        return self.remaining_fraction <= threshold_fraction

    def to_dict(self) -> Dict[str, Any]:
        return {
            "bucketId": self.bucket_id,
            "displayName": self.display_name,
            "window": self.window,
            "remainingFraction": float(self.remaining_fraction),
            "resetTime": format_rfc3339_timestamp(self.reset_time),
            "description": self.description,
            "disabled": self.disabled,
            "remainingAmount": self.remaining_amount,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> ModelQuotaBucket:
        bucket_id = str(data.get("bucketId") or data.get("bucket_id") or "")
        display_name = str(data.get("displayName") or data.get("display_name") or bucket_id)
        window = str(data.get("window") or "")
        raw_frac = data.get("remainingFraction") if data.get("remainingFraction") is not None else data.get("remaining_fraction", 1.0)
        remaining_fraction = float(raw_frac)
        reset_time = parse_rfc3339_timestamp(data.get("resetTime") or data.get("reset_time"))
        description = str(data.get("description") or "")
        disabled = bool(data.get("disabled", False))
        remaining_amount = data.get("remainingAmount") or data.get("remaining_amount")
        if remaining_amount is not None:
            remaining_amount = int(remaining_amount)

        return cls(
            bucket_id=bucket_id,
            display_name=display_name,
            window=window,
            remaining_fraction=remaining_fraction,
            reset_time=reset_time,
            description=description,
            disabled=disabled,
            remaining_amount=remaining_amount,
        )


@dataclass
class QuotaSummaryGroup:
    """
    Logical grouping of quota buckets (e.g. 'Gemini Models' or 'Claude and GPT models').
    Protobuf: google.internal.cloud.code.v1internal.QuotaSummaryGroup
    """
    display_name: str
    description: str = ""
    buckets: List[ModelQuotaBucket] = field(default_factory=list)

    def get_bucket(self, bucket_id: str) -> Optional[ModelQuotaBucket]:
        for b in self.buckets:
            if b.bucket_id == bucket_id:
                return b
        return None

    def get_5h_bucket(self) -> Optional[ModelQuotaBucket]:
        for b in self.buckets:
            if b.window == "5h" or b.bucket_id.endswith("-5h"):
                return b
        return None

    def get_weekly_bucket(self) -> Optional[ModelQuotaBucket]:
        for b in self.buckets:
            if b.window == "weekly" or b.bucket_id.endswith("-weekly"):
                return b
        return None

    def to_dict(self) -> Dict[str, Any]:
        return {
            "displayName": self.display_name,
            "description": self.description,
            "buckets": [b.to_dict() for b in self.buckets],
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> QuotaSummaryGroup:
        display_name = str(data.get("displayName") or data.get("display_name") or "")
        description = str(data.get("description") or "")
        raw_buckets = data.get("buckets") or []
        buckets = [ModelQuotaBucket.from_dict(b) for b in raw_buckets if isinstance(b, dict)]
        return cls(display_name=display_name, description=description, buckets=buckets)


@dataclass
class QuotaSummary:
    """
    Top-level quota summary response structure.
    Protobuf: google.internal.cloud.code.v1internal.RetrieveUserQuotaSummaryResponse
    """
    groups: List[QuotaSummaryGroup] = field(default_factory=list)
    description: str = ""
    timestamp: datetime.datetime = field(default_factory=lambda: datetime.datetime.now(datetime.timezone.utc))
    active_account: Optional[str] = None
    server_time_drift_seconds: float = 0.0

    @property
    def gemini_group(self) -> Optional[QuotaSummaryGroup]:
        for g in self.groups:
            if "Gemini" in g.display_name:
                return g
        return None

    @property
    def claude_3p_group(self) -> Optional[QuotaSummaryGroup]:
        for g in self.groups:
            if "Claude" in g.display_name or "3p" in g.display_name.lower():
                return g
        return None

    @property
    def gemini_5h(self) -> Optional[ModelQuotaBucket]:
        g = self.gemini_group
        return g.get_5h_bucket() if g else self.get_bucket("gemini-5h")

    @property
    def gemini_weekly(self) -> Optional[ModelQuotaBucket]:
        g = self.gemini_group
        return g.get_weekly_bucket() if g else self.get_bucket("gemini-weekly")

    @property
    def p3_5h(self) -> Optional[ModelQuotaBucket]:
        g = self.claude_3p_group
        return g.get_5h_bucket() if g else self.get_bucket("3p-5h")

    @property
    def p3_weekly(self) -> Optional[ModelQuotaBucket]:
        g = self.claude_3p_group
        return g.get_weekly_bucket() if g else self.get_bucket("3p-weekly")

    def get_bucket(self, bucket_id: str) -> Optional[ModelQuotaBucket]:
        for g in self.groups:
            b = g.get_bucket(bucket_id)
            if b:
                return b
        return None

    def lowest_remaining_fraction(self, group_name: Optional[str] = None) -> float:
        """Find the minimum remaining fraction across all active buckets."""
        candidates = []
        for g in self.groups:
            if group_name and group_name.lower() not in g.display_name.lower():
                continue
            for b in g.buckets:
                if not b.disabled:
                    candidates.append(b.remaining_fraction)
        return min(candidates) if candidates else 1.0

    def is_exhausted(self, threshold_fraction: float = 0.05, group_name: Optional[str] = None) -> bool:
        """Check if any tracked bucket is below or equal to threshold."""
        return self.lowest_remaining_fraction(group_name) <= threshold_fraction

    def to_dict(self) -> Dict[str, Any]:
        return {
            "groups": [g.to_dict() for g in self.groups],
            "description": self.description,
            "timestamp": format_rfc3339_timestamp(self.timestamp),
            "activeAccount": self.active_account,
            "serverTimeDriftSeconds": self.server_time_drift_seconds,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> QuotaSummary:
        raw_groups = data.get("groups") or []
        groups = [QuotaSummaryGroup.from_dict(g) for g in raw_groups if isinstance(g, dict)]
        description = str(data.get("description") or "")
        ts = parse_rfc3339_timestamp(data.get("timestamp")) or datetime.datetime.now(datetime.timezone.utc)
        active_account = data.get("activeAccount") or data.get("active_account")
        drift = float(data.get("serverTimeDriftSeconds") or data.get("server_time_drift_seconds") or 0.0)

        return cls(
            groups=groups,
            description=description,
            timestamp=ts,
            active_account=active_account,
            server_time_drift_seconds=drift,
        )


@dataclass
class ModelDetails:
    """
    Capabilities and details for a single model.
    Protobuf: google.internal.cloud.code.v1internal.ModelDetails
    """
    model_id: str
    display_name: str
    supports_images: bool = False
    supports_thinking: bool = False
    thinking_budget: int = 0
    min_thinking_budget: int = 0
    recommended: bool = False
    max_tokens: int = 0
    max_output_tokens: int = 0
    tokenizer_type: str = ""
    beta_warning_message: str = ""
    beta: bool = False
    disabled: bool = False
    description: str = ""
    remaining_fraction: Optional[float] = None
    reset_time: Optional[datetime.datetime] = None

    def to_dict(self) -> Dict[str, Any]:
        res = {
            "modelId": self.model_id,
            "displayName": self.display_name,
            "supportsImages": self.supports_images,
            "supportsThinking": self.supports_thinking,
            "thinkingBudget": self.thinking_budget,
            "minThinkingBudget": self.min_thinking_budget,
            "recommended": self.recommended,
            "maxTokens": self.max_tokens,
            "maxOutputTokens": self.max_output_tokens,
            "tokenizerType": self.tokenizer_type,
            "betaWarningMessage": self.beta_warning_message,
            "beta": self.beta,
            "disabled": self.disabled,
            "description": self.description,
        }
        if self.remaining_fraction is not None or self.reset_time is not None:
            res["quotaInfo"] = {
                "remainingFraction": self.remaining_fraction,
                "resetTime": format_rfc3339_timestamp(self.reset_time),
            }
        return res

    @classmethod
    def from_dict(cls, model_id: str, data: Dict[str, Any]) -> ModelDetails:
        quota_info = data.get("quotaInfo") or data.get("quota_info") or {}
        rem_frac = quota_info.get("remainingFraction") if isinstance(quota_info, dict) else None
        res_time = parse_rfc3339_timestamp(quota_info.get("resetTime")) if isinstance(quota_info, dict) else None

        return cls(
            model_id=model_id,
            display_name=str(data.get("displayName") or model_id),
            supports_images=bool(data.get("supportsImages", False)),
            supports_thinking=bool(data.get("supportsThinking", False)),
            thinking_budget=int(data.get("thinkingBudget", 0)),
            min_thinking_budget=int(data.get("minThinkingBudget", 0)),
            recommended=bool(data.get("recommended", False)),
            max_tokens=int(data.get("maxTokens", 0)),
            max_output_tokens=int(data.get("maxOutputTokens", 0)),
            tokenizer_type=str(data.get("tokenizerType", "")),
            beta_warning_message=str(data.get("betaWarningMessage", "")),
            beta=bool(data.get("beta", False)),
            disabled=bool(data.get("disabled", False)),
            description=str(data.get("description", "")),
            remaining_fraction=float(rem_frac) if rem_frac is not None else None,
            reset_time=res_time,
        )


@dataclass
class TieredModelConfig:
    """
    Tiered model classification for adaptive fallback.
    Protobuf: google.internal.cloud.code.v1internal.TieredModelConfig
    """
    flash_lite: List[str] = field(default_factory=list)
    flash: List[str] = field(default_factory=list)
    pro: List[str] = field(default_factory=list)

    def to_dict(self) -> Dict[str, List[str]]:
        return {
            "flashLite": list(self.flash_lite),
            "flash": list(self.flash),
            "pro": list(self.pro),
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> TieredModelConfig:
        return cls(
            flash_lite=list(data.get("flashLite") or data.get("flash_lite") or []),
            flash=list(data.get("flash") or []),
            pro=list(data.get("pro") or []),
        )


@dataclass
class ModelCatalog:
    """
    Complete available model catalog returned from fetchAvailableModels.
    Protobuf: google.internal.cloud.code.v1internal.FetchAvailableModelsResponse
    """
    models: Dict[str, ModelDetails] = field(default_factory=dict)
    default_agent_model_id: str = "gemini-3.8-flash-high"
    tiered_model_ids: TieredModelConfig = field(default_factory=TieredModelConfig)
    timestamp: datetime.datetime = field(default_factory=lambda: datetime.datetime.now(datetime.timezone.utc))

    def get_model(self, model_id: str) -> Optional[ModelDetails]:
        return self.models.get(model_id)

    def to_dict(self) -> Dict[str, Any]:
        return {
            "models": {k: v.to_dict() for k, v in self.models.items()},
            "defaultAgentModelId": self.default_agent_model_id,
            "tieredModelIds": self.tiered_model_ids.to_dict(),
            "timestamp": format_rfc3339_timestamp(self.timestamp),
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> ModelCatalog:
        raw_models = data.get("models") or {}
        models = {}
        if isinstance(raw_models, dict):
            for m_id, m_data in raw_models.items():
                if isinstance(m_data, dict):
                    models[m_id] = ModelDetails.from_dict(m_id, m_data)

        default_agent = str(data.get("defaultAgentModelId") or "gemini-3.8-flash-high")
        tiered = TieredModelConfig.from_dict(data.get("tieredModelIds") or {})
        ts = parse_rfc3339_timestamp(data.get("timestamp")) or datetime.datetime.now(datetime.timezone.utc)

        return cls(
            models=models,
            default_agent_model_id=default_agent,
            tiered_model_ids=tiered,
            timestamp=ts,
        )
```

---

### 3.2 `antigravity_swiss/quota/client.py`

```python
"""
Pure Python Standard Library Client for Google CloudCode & OAuth Endpoints.
==========================================================================
Directly interacts with:
- POST /v1internal:retrieveUserQuotaSummary
- POST /v1internal:fetchAvailableModels
- POST /v1internal:generateContent
- POST /token (OAuth2 refresh)

Zero third-party dependencies: strictly uses urllib.request / urllib.error / http.client.
Provides synchronous methods and non-blocking asyncio wrappers via asyncio.to_thread.
"""

from __future__ import annotations

import asyncio
import datetime
import email.utils
import json
import logging
import socket
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, Optional, Tuple

from antigravity_swiss.core.constants import (
    GOOGLE_DEFAULT_CLIENT_ID,
    GOOGLE_DEFAULT_CLIENT_SECRET,
    GOOGLE_OAUTH_TOKEN_URL,
    GOOGLE_USER_AGENT,
)
from antigravity_swiss.core.errors import (
    QuotaAuthExpiredError,
    QuotaError,
    QuotaNetworkError,
)

logger = logging.getLogger("antigravity_swiss.quota.client")

DEFAULT_CLOUDCODE_URL = "https://cloudcode-pa.googleapis.com"
DEFAULT_TIMEOUT_SEC = 15.0


class QuotaRateLimitError(QuotaError):
    """Raised on HTTP 429 RESOURCE_EXHAUSTED."""
    def __init__(self, message: str = "Quota exhausted or rate limit reached", data: Any = None):
        super().__init__(message, code=-32033, data=data)


class QuotaUnavailableError(QuotaError):
    """Raised on HTTP 503 / 502 / 504 UNAVAILABLE."""
    def __init__(self, message: str = "Google CloudCode service temporarily unavailable", data: Any = None):
        super().__init__(message, code=-32034, data=data)


class CloudCodeClient:
    """
    Standard library HTTP client communicating with Google CloudCode endpoints.
    """

    def __init__(
        self,
        base_url: str = DEFAULT_CLOUDCODE_URL,
        token_url: str = GOOGLE_OAUTH_TOKEN_URL,
        user_agent: str = f"{GOOGLE_USER_AGENT} linux/amd64",
        timeout: float = DEFAULT_TIMEOUT_SEC,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.token_url = token_url
        self.user_agent = user_agent
        self.timeout = timeout
        self.last_clock_drift_seconds: float = 0.0

    def _parse_http_date_drift(self, date_header: Optional[str]) -> float:
        """Parse HTTP Date response header and calculate clock drift (server - local)."""
        if not date_header:
            return 0.0
        try:
            parsed_dt = email.utils.parsedate_to_datetime(date_header)
            if parsed_dt.tzinfo is None:
                parsed_dt = parsed_dt.replace(tzinfo=datetime.timezone.utc)
            now = datetime.datetime.now(datetime.timezone.utc)
            drift = (parsed_dt - now).total_seconds()
            self.last_clock_drift_seconds = drift
            return drift
        except Exception as exc:
            logger.debug("Could not parse HTTP Date header '%s': %s", date_header, exc)
            return 0.0

    def _execute_request_sync(
        self,
        url: str,
        payload_dict: Dict[str, Any],
        access_token: Optional[str] = None,
        extra_headers: Optional[Dict[str, str]] = None,
    ) -> Tuple[Dict[str, Any], float]:
        """
        Synchronous HTTP POST request execution with strict error mapping.
        Returns: (response_json_dict, clock_drift_seconds)
        """
        body_bytes = json.dumps(payload_dict).encode("utf-8")
        headers = {
            "Content-Type": "application/json",
            "User-Agent": self.user_agent,
            "Content-Length": str(len(body_bytes)),
        }
        if access_token:
            headers["Authorization"] = f"Bearer {access_token}"
        if extra_headers:
            headers.update(extra_headers)

        req = urllib.request.Request(url, data=body_bytes, headers=headers, method="POST")

        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                status_code = resp.status
                raw_body = resp.read()
                date_header = resp.headers.get("Date")
                drift = self._parse_http_date_drift(date_header)

                try:
                    data = json.loads(raw_body.decode("utf-8")) if raw_body else {}
                except json.JSONDecodeError as jde:
                    raise QuotaError(f"Malformed JSON response from {url}: {jde}") from jde

                return data, drift

        except urllib.error.HTTPError as he:
            date_header = he.headers.get("Date") if hasattr(he, "headers") else None
            drift = self._parse_http_date_drift(date_header)
            raw_err = he.read().decode("utf-8", errors="replace") if hasattr(he, "read") else ""
            err_json = {}
            try:
                err_json = json.loads(raw_err)
            except Exception:
                err_json = {"raw": raw_err}

            msg = err_json.get("error", {}).get("message", raw_err) if isinstance(err_json.get("error"), dict) else raw_err

            if he.code == 401:
                raise QuotaAuthExpiredError(f"HTTP 401 Unauthenticated: {msg}", data=err_json) from he
            elif he.code == 429:
                raise QuotaRateLimitError(f"HTTP 429 Resource Exhausted: {msg}", data=err_json) from he
            elif he.code in (502, 503, 504):
                raise QuotaUnavailableError(f"HTTP {he.code} Unavailable: {msg}", data=err_json) from he
            else:
                raise QuotaError(f"HTTP {he.code} Error from {url}: {msg}", code=-32030, data=err_json) from he

        except (urllib.error.URLError, socket.timeout, TimeoutError, OSError) as ne:
            raise QuotaNetworkError(f"Network error connecting to {url}: {ne}") from ne

    # --- Synchronous API Methods ---

    def retrieve_user_quota_summary_sync(
        self,
        access_token: str,
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        """Call POST /v1internal:retrieveUserQuotaSummary synchronously."""
        url = f"{self.base_url}/v1internal:retrieveUserQuotaSummary"
        return self._execute_request_sync(url, {"project": project}, access_token=access_token)

    def fetch_available_models_sync(
        self,
        access_token: str,
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        """Call POST /v1internal:fetchAvailableModels synchronously."""
        url = f"{self.base_url}/v1internal:fetchAvailableModels"
        return self._execute_request_sync(url, {"project": project}, access_token=access_token)

    def generate_content_warmup_sync(
        self,
        access_token: str,
        model: str = "gemini-3.5-flash-lite",
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        """Call POST /v1internal:generateContent with 1-token prompt synchronously."""
        url = f"{self.base_url}/v1internal:generateContent"
        payload = {
            "project": project,
            "model": model,
            "request": {
                "contents": [{"role": "user", "parts": [{"text": " "}]}],
                "generationConfig": {"maxOutputTokens": 1, "temperature": 0.0},
            },
        }
        return self._execute_request_sync(url, payload, access_token=access_token)

    def refresh_access_token_sync(
        self,
        refresh_token: str,
        client_id: Optional[str] = None,
        client_secret: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Execute OAuth2 token refresh synchronously against token endpoint."""
        cid = client_id or GOOGLE_DEFAULT_CLIENT_ID
        csec = client_secret or GOOGLE_DEFAULT_CLIENT_SECRET

        target_url = self.token_url
        if "127.0.0.1" in self.base_url or "localhost" in self.base_url:
            target_url = f"{self.base_url}/token"

        payload = {
            "client_id": cid,
            "client_secret": csec,
            "refresh_token": refresh_token,
            "grant_type": "refresh_token",
        }
        data, _ = self._execute_request_sync(target_url, payload)
        if "access_token" not in data:
            raise QuotaAuthExpiredError("OAuth token refresh response did not contain access_token", data=data)
        return data

    # --- Asynchronous Wrappers (Non-blocking via asyncio.to_thread) ---

    async def retrieve_user_quota_summary(
        self,
        access_token: str,
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        return await asyncio.to_thread(self.retrieve_user_quota_summary_sync, access_token, project)

    async def fetch_available_models(
        self,
        access_token: str,
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        return await asyncio.to_thread(self.fetch_available_models_sync, access_token, project)

    async def generate_content_warmup(
        self,
        access_token: str,
        model: str = "gemini-3.5-flash-lite",
        project: str = "",
    ) -> Tuple[Dict[str, Any], float]:
        return await asyncio.to_thread(self.generate_content_warmup_sync, access_token, model, project)

    async def refresh_access_token(
        self,
        refresh_token: str,
        client_id: Optional[str] = None,
        client_secret: Optional[str] = None,
    ) -> Dict[str, Any]:
        return await asyncio.to_thread(self.refresh_access_token_sync, refresh_token, client_id, client_secret)
```

---

### 3.3 `antigravity_swiss/quota/poller.py`

```python
"""
Background Quota Polling Engine & Model Catalog Manager.
========================================================
Orchestrates CloudCodeClient, KeyringService, and AccountVault.
Implements:
- Proactive & reactive OAuth token refresh
- In-memory caching with TTL
- Exponential backoff with jitter on errors
- Thread-safe pub-sub notifications to daemon & GUI
"""

from __future__ import annotations

import asyncio
import datetime
import logging
import random
import time
from typing import Any, Callable, Dict, Optional

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.constants import DEFAULT_POLLING_INTERVAL_SECONDS
from antigravity_swiss.core.errors import (
    KeyringError,
    QuotaAuthExpiredError,
    QuotaError,
)
from antigravity_swiss.keyring.switcher import AccountVault, KeyringCredential, KeyringService
from antigravity_swiss.quota.client import CloudCodeClient
from antigravity_swiss.quota.models import ModelCatalog, QuotaSummary

logger = logging.getLogger("antigravity_swiss.quota.poller")

DEFAULT_CACHE_TTL_SEC = 15.0
DEFAULT_MODEL_REFRESH_INTERVAL_SEC = 900.0  # 15 minutes


class QuotaPoller:
    """
    Periodic background poller fetching live quota fractions and model catalogs.
    """

    def __init__(
        self,
        config: Optional[SwissKnifeConfig] = None,
        client: Optional[CloudCodeClient] = None,
        keyring_service: Optional[KeyringService] = None,
        vault: Optional[AccountVault] = None,
        poll_interval_sec: Optional[float] = None,
        cache_ttl_sec: float = DEFAULT_CACHE_TTL_SEC,
        model_refresh_interval_sec: float = DEFAULT_MODEL_REFRESH_INTERVAL_SEC,
        on_quota_updated: Optional[Callable[[QuotaSummary], Any]] = None,
    ) -> None:
        from antigravity_swiss.core.config import SwissKnifeConfig
        self.config = config or SwissKnifeConfig.load()
        self.client = client or CloudCodeClient()
        self.vault = vault or AccountVault(config_path=self.config.accounts_file)
        self.keyring_service = keyring_service or KeyringService(vault=self.vault)
        self.poll_interval_sec = poll_interval_sec or self.config.poll_interval_sec or DEFAULT_POLLING_INTERVAL_SECONDS
        self.cache_ttl_sec = cache_ttl_sec
        self.model_refresh_interval_sec = model_refresh_interval_sec
        self.on_quota_updated = on_quota_updated

        # Poller state
        self._is_running: bool = False
        self._task: Optional[asyncio.Task] = None
        self._lock = asyncio.Lock()
        self._cached_summary: Optional[QuotaSummary] = None
        self._cached_summary_time: float = 0.0
        self._cached_catalog: Optional[ModelCatalog] = None
        self._cached_catalog_time: float = 0.0
        self._consecutive_errors: int = 0
        self._last_error: Optional[Exception] = None

    @property
    def is_running(self) -> bool:
        return self._is_running

    @property
    def cached_summary(self) -> Optional[QuotaSummary]:
        return self._cached_summary

    @property
    def cached_catalog(self) -> Optional[ModelCatalog]:
        return self._cached_catalog

    async def start(self) -> None:
        """Start background polling task."""
        if self._is_running:
            return
        self._is_running = True
        self._task = asyncio.create_task(self._poll_loop())
        logger.info("Quota poller background task started (interval=%.1fs)", self.poll_interval_sec)

    async def stop(self) -> None:
        """Stop background polling task cleanly."""
        if not self._is_running:
            return
        self._is_running = False
        if self._task and not self._task.done():
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        self._task = None
        logger.info("Quota poller stopped cleanly.")

    async def _poll_loop(self) -> None:
        """Main periodic polling loop with exponential backoff and jitter."""
        while self._is_running:
            try:
                summary = await self.poll_summary(force=True)
                self._consecutive_errors = 0
                self._last_error = None
                # Add slight random jitter (+/- 1.5s) to avoid harmonic network synchronization
                jitter = random.uniform(-1.5, 1.5)
                sleep_sec = max(5.0, self.poll_interval_sec + jitter)
            except asyncio.CancelledError:
                break
            except Exception as exc:
                self._consecutive_errors += 1
                self._last_error = exc
                backoff = min(60.0, 2.0 * (1.5 ** min(self._consecutive_errors, 6)))
                jitter = random.uniform(0.2, 1.5)
                sleep_sec = backoff + jitter
                logger.warning(
                    "Quota poller error (failure #%d): %s; backing off for %.1fs",
                    self._consecutive_errors,
                    exc,
                    sleep_sec,
                )

            try:
                await asyncio.sleep(sleep_sec)
            except asyncio.CancelledError:
                break

    async def _ensure_valid_token(self, cred: KeyringCredential, account_email: Optional[str] = None) -> KeyringCredential:
        """
        Inspect token expiration. If expired or expiring within 120s, refresh token.
        """
        now = datetime.datetime.now(datetime.timezone.utc)
        needs_refresh = False

        if not cred.access_token:
            needs_refresh = True
        elif cred.expiry:
            try:
                exp_dt = datetime.datetime.fromisoformat(cred.expiry.replace("Z", "+00:00"))
                if exp_dt.tzinfo is None:
                    exp_dt = exp_dt.replace(tzinfo=datetime.timezone.utc)
                if (exp_dt - now).total_seconds() < 120.0:
                    needs_refresh = True
            except Exception:
                pass

        if not needs_refresh:
            return cred

        if not cred.refresh_token:
            logger.warning("Token expired but no refresh_token present for %s", account_email)
            return cred

        logger.info("Proactively refreshing access token for %s", account_email or "active account")
        new_token_data = await self.client.refresh_access_token(cred.refresh_token)
        new_access_token = new_token_data["access_token"]
        expires_in = int(new_token_data.get("expires_in", 3600))
        new_expiry = (now + datetime.timedelta(seconds=expires_in)).isoformat()

        updated_cred = KeyringCredential(
            access_token=new_access_token,
            refresh_token=cred.refresh_token,
            token_type=new_token_data.get("token_type", "Bearer"),
            expiry=new_expiry,
            auth_method=cred.auth_method,
            id_token=cred.id_token,
        )

        # Update active Secret Service keyring if refreshing active account
        active_email = self.vault.get_active_account()
        if account_email is None or account_email == active_email:
            self.keyring_service.set_active_credential(updated_cred)

        if account_email:
            rec = self.vault.get_account(account_email)
            self.vault.add_or_update_account(
                email=account_email,
                credential=updated_cred,
                label=rec.label if rec else "",
            )

        return updated_cred

    async def poll_summary(self, force: bool = False, email: Optional[str] = None) -> QuotaSummary:
        """
        Fetch quota summary for the specified account (or active account).
        Uses cache if within cache_ttl_sec and force is False.
        """
        async with self._lock:
            now_mono = time.monotonic()
            if not force and not email and self._cached_summary and (now_mono - self._cached_summary_time < self.cache_ttl_sec):
                return self._cached_summary

            # 1. Resolve credential
            if email:
                rec = self.vault.get_account(email)
                if not rec:
                    raise KeyringError(f"Account '{email}' not found in vault")
                cred = rec.credential
                target_email = email
            else:
                cred = self.keyring_service.get_active_credential()
                target_email = self.vault.get_active_account() or cred.extract_email_from_id_token()

            # 2. Check token freshness
            cred = await self._ensure_valid_token(cred, target_email)

            # 3. Query CloudCode API with reactive 401 retry
            try:
                raw_data, drift = await self.client.retrieve_user_quota_summary(cred.access_token)
            except QuotaAuthExpiredError:
                if cred.refresh_token:
                    logger.info("HTTP 401 received; executing reactive token refresh")
                    cred = await self._ensure_valid_token(
                        KeyringCredential(
                            access_token="",
                            refresh_token=cred.refresh_token,
                            token_type=cred.token_type,
                            expiry="",
                            auth_method=cred.auth_method,
                            id_token=cred.id_token,
                        ),
                        target_email,
                    )
                    raw_data, drift = await self.client.retrieve_user_quota_summary(cred.access_token)
                else:
                    raise

            # 4. Parse model structure
            summary = QuotaSummary.from_dict(raw_data)
            summary.active_account = target_email
            summary.server_time_drift_seconds = drift

            # 5. Cache result if querying active account
            if not email:
                self._cached_summary = summary
                self._cached_summary_time = now_mono

            # 6. Notify callbacks
            if self.on_quota_updated and not email:
                try:
                    res = self.on_quota_updated(summary)
                    if asyncio.iscoroutine(res):
                        await res
                except Exception as exc:
                    logger.warning("Error in on_quota_updated callback: %s", exc)

            return summary

    async def fetch_models(self, force: bool = False, email: Optional[str] = None) -> ModelCatalog:
        """
        Fetch available models and capabilities.
        Uses cache if within model_refresh_interval_sec and force is False.
        """
        async with self._lock:
            now_mono = time.monotonic()
            if not force and not email and self._cached_catalog and (now_mono - self._cached_catalog_time < self.model_refresh_interval_sec):
                return self._cached_catalog

            if email:
                rec = self.vault.get_account(email)
                if not rec:
                    raise KeyringError(f"Account '{email}' not found in vault")
                cred = rec.credential
            else:
                cred = self.keyring_service.get_active_credential()

            cred = await self._ensure_valid_token(cred, email)

            try:
                raw_data, _ = await self.client.fetch_available_models(cred.access_token)
            except QuotaAuthExpiredError:
                if cred.refresh_token:
                    cred = await self._ensure_valid_token(
                        KeyringCredential(
                            access_token="",
                            refresh_token=cred.refresh_token,
                            token_type=cred.token_type,
                            expiry="",
                            auth_method=cred.auth_method,
                            id_token=cred.id_token,
                        ),
                        email,
                    )
                    raw_data, _ = await self.client.fetch_available_models(cred.access_token)
                else:
                    raise

            catalog = ModelCatalog.from_dict(raw_data)
            if not email:
                self._cached_catalog = catalog
                self._cached_catalog_time = now_mono

            return catalog
```

---

## 4. Caveats

1. **Clock Drift Between Local System and Upstream CloudCode**:
   - The system clock may differ from Google's servers by several seconds.
   - The client parses the HTTP RFC 7231 `Date` header on every response to calculate $\Delta t = t_{\text{Google}} - t_{\text{local}}$. The warmup engine in F08 and quota gauge reset countdowns in F18 must add $\Delta t$ to all scheduled countdowns to avoid sending keep-alive pings before Google's batch reconciler resets.
2. **Third-Party Claude & GPT Buckets vs Gemini Buckets**:
   - Upstream separates Gemini models (`gemini-5h`, `gemini-weekly`) from third-party models (`3p-5h`, `3p-weekly`).
   - Depletion of `3p-5h` does NOT deplete `gemini-5h`. The rule engine in F09 and UI in F18 must track both pools independently.
3. **Consumer Account Project ID**:
   - Standard consumer accounts authenticate with `auth_method: "consumer"` and pass `{"project": ""}` in the request body. Enterprise GCP or Cloud Workstations may require a project ID string. Defaulting `project=""` while allowing an override covers both scenarios.
4. **Daemon Integration & IPC Event Broadcasting**:
   - When running inside `AsyncUnixSocketServer` in `__main__.py`, `QuotaPoller` must broadcast `notify.quota_updated` on every successful poll so connected GUI clients instantly receive updated gauges without polling over IPC.
5. **No Agnostic agy CLI Invocation**:
   - All token acquisitions strictly interface with `KeyringService` (Linux Secret Service `secret-tool`), adhering to the fundamental constraint of operating natively in the Antigravity desktop app's agent/account context.

---

## 5. Conclusion

Milestone 2 features F06 (`F06_QUOTA_SUMMARY_POLLER`) and F07 (`F07_MODEL_CATALOG_FETCHER`) have a fully specified, zero-dependency implementation blueprint:
1. `antigravity_swiss/quota/models.py` implements complete, typed dataclasses (`ModelQuotaBucket`, `QuotaSummaryGroup`, `QuotaSummary`, `ModelDetails`, `TieredModelConfig`, `ModelCatalog`) with bi-directional JSON serialization and rich property accessors.
2. `antigravity_swiss/quota/client.py` implements a pure standard library client (`urllib.request`) communicating with `https://cloudcode-pa.googleapis.com` endpoints, extracting HTTP `Date` drift, handling token refresh, and cleanly wrapping blocking calls with `asyncio.to_thread`.
3. `antigravity_swiss/quota/poller.py` coordinates dynamic credential acquisition, proactive/reactive token refresh, caching with TTL, exponential backoff with jitter on errors, and pub-sub notifications for the daemon and UI.

---

## 6. Verification Method

To independently verify the implementation:

1. **Unit Test Suite Execution**:
   Run `pytest` against unit tests:
   ```bash
   /usr/bin/pytest tests/unit -v
   ```
2. **Mock Server Integration Tests**:
   Verify against `MockCloudCodeServer` (which emulates `retrieveUserQuotaSummary`, `fetchAvailableModels`, token refresh, clock drift, and transient 503 errors):
   ```bash
   /usr/bin/pytest tests/e2e/test_tier1_features.py -k "f06 or f07" -v
   ```
3. **Standalone Quota Poll Verification**:
   ```bash
   python3 -c "
   import asyncio
   from antigravity_swiss.quota.client import CloudCodeClient
   from antigravity_swiss.quota.models import QuotaSummary
   from tests.fixtures.mock_cloudcode_server import MockCloudCodeServer

   async def run():
       srv = MockCloudCodeServer()
       url = srv.start()
       try:
           client = CloudCodeClient(base_url=url)
           data, drift = await client.retrieve_user_quota_summary('ya29.test')
           summary = QuotaSummary.from_dict(data)
           assert summary.gemini_5h.window == '5h'
           assert summary.gemini_5h.remaining_fraction == 0.85
           print('SUCCESS: Verified quota models and client over mock server.')
       finally:
           srv.stop()

   asyncio.run(run())
   "
   ```
4. **Full Test Suite Integrity**:
   Verify all existing 323+ tests continue to pass without regression:
   ```bash
   /usr/bin/pytest tests/unit tests/e2e -q
   ```
