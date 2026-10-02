"""
Antigravity Swiss Knife Quota Tracking, Polling & Auto-Switch Rule Engine.
==========================================================================
Provides real-time model quota tracking, CloudCode REST client, background polling,
and intelligent account rotation rule engine.
"""

from antigravity_swiss.quota.client import (
    CloudCodeClient,
    QuotaRateLimitError,
    QuotaUnavailableError,
)
from antigravity_swiss.quota.models import (
    ModelCatalog,
    ModelDetails,
    ModelQuotaBucket,
    QuotaSummary,
    QuotaSummaryGroup,
    TieredModelConfig,
    format_rfc3339_timestamp,
    parse_rfc3339_timestamp,
)
from antigravity_swiss.quota.poller import QuotaPoller
from antigravity_swiss.quota.rule_engine import (
    AutoSwitchRuleEngine,
    EvaluationResult,
    RuleEngineConfig,
)

__all__ = [
    "ModelQuotaBucket",
    "QuotaSummaryGroup",
    "QuotaSummary",
    "ModelDetails",
    "TieredModelConfig",
    "ModelCatalog",
    "parse_rfc3339_timestamp",
    "format_rfc3339_timestamp",
    "CloudCodeClient",
    "QuotaRateLimitError",
    "QuotaUnavailableError",
    "QuotaPoller",
    "RuleEngineConfig",
    "EvaluationResult",
    "AutoSwitchRuleEngine",
]
