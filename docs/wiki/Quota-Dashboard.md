# Quota Dashboard

[Features](Features.md) / Quota Dashboard

The Quota Dashboard is the primary supervisory cockpit in Antigravity Swiss Knife. It surfaces real-time quota telemetry across multi-account fleets, monitors rolling rate-limit windows, and coordinates manual and automated credential rotations.

---

## 1. Architectural Role & Telemetry Ingestion

Antigravity communicates with Google CloudCode Pa endpoints (`cloudcode-pa.googleapis.com`) to query account tiers, model allowances, and token buckets. The dashboard queries `POST /v1internal:retrieveUserQuotaSummary` through the local background daemon.

The response payload contains quota buckets parsed into two primary rolling windows:
1. **5-Hour Rolling Burst Window**: Rate limit buckets governing short-term request density.
2. **7-Day Rolling Weekly Runway**: Aggregate token budget allocated per billing or trial cycle.

Each bucket returns `remainingFraction` (floating point between `0.0` and `1.0`), alongside an RFC 3339 `resetTime` timestamp. The dashboard converts fractions into percentage meters and computes live countdown timers until bucket restoration.

---

## 2. Circular Gauge & Horizontal Quota Visualizers

The dashboard presents quota metrics through two synchronized UI primitives:
- **Circular Gauges**: Positioned at the top of the view for the currently active account. The gauges track primary model buckets (Gemini 3.8 Flash, Gemini 3.8 Flash Lite, Gemini 3.8 Pro, and Claude Opus). Dual concentric SVG rings render 5-hour burst limits on the outer track and weekly reserves on the inner track.
- **Horizontal Quota Bars**: Displayed within each row of the fleet inventory table. These responsive bars display remaining fractions and hover tooltips revealing exact reset horizons, avoiding dedicated table column clutter.

---

## 3. Account Sorting & Rotation Heuristics

The fleet table provides four deterministic sorting modes via the top-bar Sort Selector:

### Auto Mode (Default Engine)
The autonomous scheduler maximizes continuous pair-programming sessions through multi-tier ranking:
1. **Active Identity Pinning**: The active runtime account pins to row 0.
2. **Account Priority Tier**: Accounts with `High` priority rank ahead of `Mid` and `Low` priority accounts.
3. **Health Validation**: Accounts in `NEEDS_REAUTH`, `ERROR`, or `BANNED` states move to the bottom tier.
4. **Quota Runway Scoring**: Standby candidates sort by available 5-hour percentage, followed by weekly reset horizons.
5. **Depleted Account Demotion**: When the active account exhausts its quota below the configured switch threshold, the engine rotates credentials to the row 1 successor and demotes the exhausted account to the bottom row.

### Manual Sort Modes
- **Account Identity**: Alphabetical sort by account alias, followed by email address.
- **5H Quota**: Ranked strictly by remaining 5-hour burst allowance in descending order.
- **Weekly Quota**: Ranked strictly by remaining weekly token budget in descending order.

---

## 4. Multi-App Fleet Targeting

Developers can direct account switches to specific local Antigravity installations using target application selectors:
- **Antigravity Desktop 2.0**: Electron host application running with default config directory `~/.config/Antigravity`.
- **Antigravity CLI (`agy`)**: Headless command-line tool using standalone credential keyrings.
- **Antigravity Code Extension**: VS Code extension operating under `~/.vscode/extensions`.

The dashboard supports two fleet synchronization models:
- **Shared Fleet Mode**: Account rotations apply atomically across all three installed applications simultaneously.
- **Per-App Fleet Mode**: Each application maintains an independent active account and rotation schedule.

---

## 5. Account Lifecycle States

The dashboard tracks five explicit lifecycle states:
- `ACTIVE`: The account currently bound to the target application's active session.
- `STANDBY`: A healthy account authenticated and eligible for automated rotation.
- `NEEDS_REAUTH`: An account with an expired refresh token or invalidated OAuth consent. Requires browser re-authentication.
- `ERROR`: An account returning upstream API errors during quota polling.
- `BANNED`: An account flagged or suspended upstream. The dashboard displays an alert advising removal or appeal to prevent rotation deadlocks.

---

## 6. Rotation Latch & Subagent Revival Integration

Initiating an account switch (manual or automated) arms the `pkg/revival` subsystem. The daemon captures the current conversation UUID (`cascadeId`) and subagent execution state from `app_storage.json`, terminates the Antigravity process cleanly, swaps keyring credentials and device fingerprints, relaunches the host process, and re-attaches to the active conversation via Chrome DevTools Protocol.
