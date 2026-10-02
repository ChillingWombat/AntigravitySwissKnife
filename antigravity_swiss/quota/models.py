"""
Data structures for Quota Summary, Quota Buckets, and Model Catalog.
===================================================================
Conforms strictly to PROJECT.md § Interface Contracts and Google CloudCode Protobuf specs:
- google.internal.cloud.code.v1internal.QuotaSummaryBucket
- google.internal.cloud.code.v1internal.QuotaSummaryGroup
- google.internal.cloud.code.v1internal.RetrieveUserQuotaSummaryResponse
- google.internal.cloud.code.v1internal.ModelDetails
- google.internal.cloud.code.v1internal.TieredModelConfig
- google.internal.cloud.code.v1internal.FetchAvailableModelsResponse
"""

from __future__ import annotations

from dataclasses import dataclass, field
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
        return dt.astimezone(datetime.timezone.utc)
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

    def __post_init__(self) -> None:
        # Clamp remaining_fraction to [0.0, 1.0]
        try:
            self.remaining_fraction = max(0.0, min(1.0, float(self.remaining_fraction)))
        except (ValueError, TypeError):
            self.remaining_fraction = 1.0

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
        try:
            remaining_fraction = float(raw_frac)
        except (ValueError, TypeError):
            remaining_fraction = 1.0
        reset_time = parse_rfc3339_timestamp(data.get("resetTime") or data.get("reset_time"))
        description = str(data.get("description") or "")
        disabled = bool(data.get("disabled", False))
        remaining_amount = data.get("remainingAmount") or data.get("remaining_amount")
        if remaining_amount is not None:
            try:
                remaining_amount = int(remaining_amount)
            except (ValueError, TypeError):
                remaining_amount = None

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

    def __post_init__(self) -> None:
        if self.remaining_fraction is not None:
            try:
                self.remaining_fraction = max(0.0, min(1.0, float(self.remaining_fraction)))
            except (ValueError, TypeError):
                self.remaining_fraction = None

    def to_dict(self) -> Dict[str, Any]:
        res: Dict[str, Any] = {
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
