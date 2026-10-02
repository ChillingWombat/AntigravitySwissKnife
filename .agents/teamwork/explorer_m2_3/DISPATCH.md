## 2026-10-02T09:05:11Z

You are the M2 Rule Engine & Offline Mock Explorer for Antigravity Swiss Knife.

Read the authoritative requirements at:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/ORIGINAL_REQUEST.md
and project architecture at:
/mnt/Data/Projects/Antigravity Swiss Knife/PROJECT.md
and the upstream quota specification report at:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/spec_miner_quota_1/handoff.md

Your working directory is:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m2_3

Scope: Milestone 2 Features F09 (`F09_AUTO_SWITCH_RULE_ENGINE`), F26 (`F26_OFFLINE_MOCK_HARNESS`), and daemon IPC integration.
Investigate and design the exact implementation strategy for:
1. `antigravity_swiss/quota/rule_engine.py`:
   - Configurable threshold evaluation: comparing active model quota (`remainingFraction`) against configured threshold (e.g. 5% or 0.05).
   - Account eligibility selection: sorting accounts in `AccountVault` by available quota fraction across tiered models (Gemini 3.8 Flash, Flash Lite, Pro, Claude Sonnet), selecting the best account, and triggering `KeyringService.switch_account`.
   - Cooldown guardrails: preventing rapid thrashing between accounts if multiple accounts are low.
2. `tests/fixtures/mock_cloudcode_server.py`:
   - Pure Python in-process HTTP mock server simulating `cloudcode-pa.googleapis.com` responses:
     * Custom quota profiles (healthy 100%, depleted 2%, reset countdowns).
     * Simulating 1-token warmup ping responses.
     * Offline testability without external network access or burning real API quotas.
3. IPC & Daemon Integration:
   - Connecting `QuotaPoller` and `AutoSwitchRuleEngine` into `AsyncUnixSocketServer` and `SwissKnifeController`:
     * JSON-RPC methods: `quota.get_summary`, `quota.poll_now`, `rules.get_config`, `rules.set_config`.
     * Broadcasting IPC events: `notify.quota_updated`, `notify.account_switched`, `notify.warmup_triggered`.

Constraint: Rely exclusively on Antigravity desktop app's agent/account context rather than invoking any legacy agy CLI.
Deliver a structured implementation blueprint to:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m2_3/handoff.md
Follow Handoff Protocol. When complete, notify parent (11f1f26d-e61c-4e23-9c94-5ec9e98e06dd) via send_message.
