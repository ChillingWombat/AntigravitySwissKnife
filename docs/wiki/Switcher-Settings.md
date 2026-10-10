# Switcher Settings

[Features](Features.md) / Switcher Settings

The Switcher Settings console defines the policies, thresholds, polling cadences, and model hierarchies that govern automated account rotation in Antigravity Swiss Knife.

---

## 1. Auto-Switch Quota Thresholds

The auto-switcher monitors remaining quotas and triggers credential rotations before rate limit exhaustion disrupts active tasks:
- **5-Hour Quota Threshold**: The percentage floor (e.g., 5%) for the 5-hour rolling burst window. When the active account's 5-hour fraction falls below this number, the scheduler evaluates standby successors.
- **Weekly Quota Threshold**: The percentage floor (e.g., 5%) for the aggregate weekly runway. Protects long-running projects from starving an entire account of weekly budget.

---

## 2. Rotation Strategies

The switch strategy determines how standby accounts are prioritized during rotation:
- **Balanced (Default)**: Distributes request volume evenly across all healthy accounts. Standby candidates are weighted by available quota percentage to balance burn rates across the fleet.
- **Greedy**: Drains high-priority accounts down to the configured threshold before touching secondary accounts. Maximizes token efficiency on primary paid tiers.
- **Sequential**: Rotates strictly in user-defined fleet table order, cycling back to the start once all accounts reach their rotation threshold.

---

## 3. Polling Cadences & Anti-Thundering Jitter

Polling upstream Google CloudCode Pa endpoints requires balanced timing:
- **Active Polling Interval**: The frequency (default: 120 seconds) at which the active account queries quota status. Fast enough to track rapid token burn while avoiding rate limit flags.
- **Standby Polling Interval**: The frequency (default: 900 seconds) at which standby accounts check quota recovery.
- **Standby Random Jitter**: A randomized offset (default: $\pm$ 30 seconds) added to standby poll timers. Desynchronizes requests across large account fleets to prevent bursty network patterns.
- **Dynamic Quota Refresh**: An adaptive poller that shortens polling intervals as an account approaches its reset timestamp, then switches to low-frequency polling once quotas are restored.

---

## 4. Reset Horizon Warmup Engine

Upstream quota buckets do not always reset exactly at the calculated timestamp without incoming traffic. Swiss Knife implements an automated 1-token keep-alive warmup mechanism:
- When a depleted account reaches its scheduled `resetTime`, the daemon dispatches a minimal generation request (`POST /v1internal:generateContent` with `maxOutputTokens: 1`).
- This keep-alive probe activates Google's internal bucket recalculation routines, refreshing quota counters immediately.
- Includes HTTP Date header drift correction to align workstation system clocks with upstream Google Cloud servers.

---

## 5. Multi-App Synchronization

Controls how credential rotations interact with different local Antigravity environments:
- **Shared Mode**: A rotation event updates credentials and device profiles across Antigravity Desktop 2.0, the CLI (`agy`), and the VS Code Extension concurrently.
- **Isolated Per-App Mode**: Each application maintains an independent active account. A switch triggered in the desktop app leaves CLI background tasks running on their current account undisturbed.

---

## 6. Model Source Hierarchy & Subagent Routing

Defines the default fallback chain when routing queries across native models and external providers:
1. **Model Source Hierarchy Ordering**: Drag-and-drop prioritization between Gemini Native Models, Custom Models (BYOK), and Non-Gemini Native Models (Claude / GPT).
2. **Default Gemini Model**: Target baseline model (e.g., `gemini-3.8-flash-high`).
3. **Subagent Custom Model Offloading**: When enabled, delegates autonomous subagent tasks to configured custom models, preserving expensive orchestrator quotas for primary pair-programming prompts.
