# Architectural Evaluation & Consensus: VS Code Dual Catalog & Subagent Delegation

**Date:** 2026-10-22  
**Participants:** Reviewer (Gemini 3.8 Flash High) & Implementer (Pi Agent, DeepSeek V4.1 Flash)  
**Topic:** Technical Evaluation, Core Corrections & Consensus Plan for Deferred Model Source Features  

---

## 1. Executive Summary & Consensus Verdict

- **Goal 1 (VS Code Custom Catalog):**
  - **Approach 1A is dead.** `pkg/custommodels/proxy.go` contains a stale header comment from an earlier draft, but the actual implementation (lines 78–85) already documents that `fetchAvailableModels` discards uncurated model IDs (empirically verified against Antigravity 2.25.0).
  - **The decisive discovery:** Curation is mode-dependent. Cloud Code (`CLOUD_CODE_URL`) curates and drops unknown IDs, whereas Gateway mode (`AGY_LLM_GATEWAY_*`) is **trusting** and renders gateway IDs in the UI.
  - **Approach 1B (Gateway Mode)** is the real path. It can be tested with a zero-code env-var launch test.
  - **High-Priority Bug Uncovered:** VS Code currently hardcodes `--app_data_dir=antigravity` and forces `HOME` to `Ba.homedir()`, writing directly into `~/.gemini/antigravity` (`conversation_summaries.db`, `-wal`, `-shm`). This presents a live state corruption hazard across windows. Passing `--app_data_dir=antigravity-swiss-custom` via `serverArgs` (last-wins) is an immediate required fix.
- **Goal 2 (Subagent Delegation):**
  - **Approach 2A (Rule-only prompt steering) is rejected as a mechanism.** It fights hardcoded system prompts, lacks structured lifecycle ownership, and depends on empty `agy agents` lists.
  - **The real technical constraint:** `invoke_subagent` in-process is enum-restricted to `["inherit","flash_lite","flash","pro"]`. Out-of-process execution is the **mandatory mechanical precondition** for custom-model subagents to bypass Google quota.
  - **Approach 2B (`delegate_subagent` MCP tool) is accepted.** All core components (`CLIHandler`, `gatewayEnv`, MCP server) already exist in `pkg/bridge/`. Upgraded to use `--input-format stream-json`, `--output-format stream-json`, and `--conversation <id>` for continuity.

---

## 2. Agreed Action Plan & Implementation Order

| # | Priority | Item | Description | Effort |
|---|---|---|---|---|
| 1 | P0 (Hygiene) | Clean stale docs | Remove false header comment in `proxy.go:14-22`; fix refusal message in `surfaces.go:126`. | 15 min |
| 2 | P0 (Stability) | Isolate VS Code State | Add `--app_data_dir=antigravity-swiss-custom` via `antigravity.serverArgs` in `ApplyVSCodeModelSource`. Resolves live `conversation_summaries.db` corruption. | ~1 h |
| 3 | P1 (Spike) | Env-Var Catalog Test | Launch VS Code with `AGY_LLM_GATEWAY_*` env vars to verify if the webview dropdown renders the gateway catalog. Settles 1B permanently with zero code. | 30 min |
| 4 | P1 (Feature) | `delegate_subagent` MCP Tool | Wire `delegate_subagent(role, prompt, model_id, workspace_mode)` over `CLIHandler` + `gatewayEnv` with stream-json framing and `--conversation` tracking. | 1–2 days |
| 5 | P2 (Visibility)| Token Monitor Tracking | Surface delegated workers in Token Monitor / dashboard views using `agent_tracker` role mappings. | ~1 day |
| 6 | P2 (UI Gate) | Restore Subagent Toggle | Re-introduce toggle in `ModelSourceTab.tsx` only after Step 5, labelled honestly: *Subagent Budget: Google Subscription (In-Process) / BYOK Custom (Worker Process)*. | ~half day |
| 7 | P3 (Launcher) | VS Code Custom Launch | If Step 3 passes, wire `LaunchVSCode` with gateway env quartet and `serverArgs`. | ~half day |
