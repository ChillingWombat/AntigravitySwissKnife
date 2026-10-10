# Token Monitor

[Features](Features.md) / Token Monitor

The Token Monitor provides real-time telemetry, context accounting, and cost tracking for developer sessions across all connected LLMs and subagents.

---

## 1. Context Accounting & Metrics Pipeline

During pair-programming sessions, the host application emits request and response metadata. The Token Monitor intercepts this stream via local IPC and logs each invocation to an internal SQLite WAL database (`token_telemetry.db`).

The telemetry engine records seven primary metrics:
1. **Total Tokens**: Sum of all processed input, cached context, and completion tokens.
2. **Input Tokens**: Uncached prompt and history tokens transmitted in the request payload.
3. **Cached Input Tokens**: Prompt tokens served from upstream context caches (e.g., Gemini context caching or Anthropic prompt caching).
4. **Output Tokens**: Generated model completion tokens and reasoning steps.
5. **Total Cost (USD)**: Dollar valuation calculated against the active model pricing registry.
6. **Cached Savings (USD)**: Dollar discount achieved via prompt cache hits versus full-price input tokens.
7. **Generation Velocity (TPS)**: Live tokens-per-second streaming throughput.

---

## 2. Visualizer Modes & Historical Windows

Developers can inspect telemetry across two visual units:
- **USD Valuation Mode**: Displays financial spend and prompt cache savings in US Dollars.
- **Token Volume Mode**: Displays raw token numbers formatted with standard SI suffixes (`k`, `M`).

Both modes support filtering across four time horizons: 24 Hours, 7 Days, 30 Days, and All Time.

---

## 3. Dynamic Model Pricing Registry

Cost calculations rely on an extensible pricing table (`pricing.json`) mapping rates per million tokens ($/1M tokens) across three dimensions:
- **Input Rate**: Cost per 1M un-cached input tokens.
- **Cached Input Rate**: Cost per 1M cached prompt tokens (typically 75% to 90% cheaper).
- **Output Rate**: Cost per 1M generated output tokens.

The pricing table comes pre-populated with official Google Cloud and Anthropic pricing, but allows full manual customization for custom BYOK models, discounted enterprise endpoints, or free local models ($0.00).

---

## 4. Subagent Aggregation Simulator

Autonomous pair-programming frequently spawns background child agents, creating multiplicative token burn. The Token Monitor includes an interactive Multi-Agent Simulator:
- Configures orchestrator base context size (e.g., 4,200 tokens).
- Adjusts concurrent subagent count (e.g., 2 to 8 subagents).
- Simulates average child agent turn size (e.g., 6,500 tokens).
- Models context cache hit ratios (e.g., 65% hit rate).
- Displays projected total token burn and dollar expenditure before launching large automated sweeps.

---

## 5. Telemetry Log Stream

The Telemetry & Logs tab provides a tabular stream of raw API calls:
- Timestamp (RFC 3339).
- Model identifier and plan tier.
- Prompt vs completion token breakdown.
- Context cache status.
- Round-trip latency and effective tokens-per-second streaming speed.

---

## 6. Forward Reference: Skills vs Harness Breakdown

In upcoming releases, the Token Monitor will expand to disclose granular breakdowns distinguishing tokens consumed by the agent harness (system framing, MCP tool schemas, conversation history) versus tokens used for actual skill execution (code edits, terminal commands, browser actions). See the [Roadmap](Roadmap.md) for architectural details on this planned capability.
