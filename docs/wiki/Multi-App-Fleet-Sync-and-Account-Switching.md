# Multi-App Fleet Sync & Account Switching

Antigravity Swiss Knife provides cross-application account coordination, quota monitoring, and zero-loss credential rotation for developer workstations running multiple Google Antigravity environments.

---

## 1. Fleet Management Across 3 Antigravity Applications

Developers routinely run different flavors of Google Antigravity depending on their workflow:

1. **Antigravity Desktop 2.0 (`desktop`)**: The primary Electron-based graphical IDE.
2. **Antigravity CLI `agy` (`agy`)**: The terminal daemon and command-line execution harness.
3. **Antigravity Extension (`vscode`)**: The editor extension running inside VS Code or compatible forks.

Antigravity Swiss Knife automatically detects installed instances across all three application types, monitors their respective credential stores, and surfaces unified quota statistics in a single dashboard.

---

## 2. Fleet Switching Modes

The Switcher Settings module (`frontend/src/pages/SystemSettingsPage.tsx` and `pkg/switcher`) provides two operational modes for fleet synchronization:

```
[Switcher Settings]
Fleet Mode:
( ) Same account across all apps (synchronized fleet)
(*) Individual accounts per app (independent app isolation)
```

### 2.1 Shared Account Mode (`same_account`)
- Synchronizes a single active Google CloudCode account across all installed Antigravity applications.
- When an account switch occurs (manually via UI or automatically via quota exhaustion), the daemon updates the credentials across Desktop, CLI, and Extension simultaneously.
- If only one Antigravity application is installed on the host, the switcher UI automatically forces and greys out this toggle to prevent confusing configurations.

### 2.2 Individual Accounts Mode (`individual_accounts`)
- Decouples each installed application into an independent slot.
- For example, Desktop runs on Account A (e.g. Ultra tier for deep code generation), CLI `agy` runs on Account B (e.g. Pro tier for terminal workflows), and the Extension runs on Account C.
- The UI fleet table displays targeted switch controls for each app individually, or allows switching all apps at once.

---

## 3. Standby Account Selection Algorithm

When an active account hits rate limits or exhausts its 5-hour or weekly quota pool, the auto-switch engine selects a replacement account from the standby pool:

```
Candidate Accounts
       |
       v
Filter: Accounts with available 5h quota > threshold (e.g. 15%)
       |
       v
Filter: In individual mode, prioritize unassigned standby accounts
       | (exclude accounts actively in use by other running apps)
       |
       v
Score Candidates:
  Tier Weight: Enterprise (500) > Ultra (400) > Pro (300) > Edu (200) > Free (100)
  + Quota Availability (remaining % * 2)
  + Staleness Bonus (idle time since last switch)
       |
       v
Select Top-Scoring Standby Account
```

If all standby accounts are already occupied by other applications, the algorithm gracefully falls back to selecting the least-constrained shared account, ensuring tasks never crash due to exhausted quotas.

---

## 4. Offline Account Switching

When a user triggers an account switch while Antigravity is not running, conventional tools fail by attempting to relaunch non-existent windows or corrupting process handles.

Antigravity Swiss Knife implements clean offline switching:
1. **Process Detection**: Checks process tables for running host IDE instances (`len(mainPIDs) == 0`).
2. **Direct Disk Mutation**: Updates `activeEmail` and authentication tokens directly inside the OS keyring and configuration state stores.
3. **Suppressed Relaunch**: Completely bypasses window launch triggers, leaving the IDE closed until the user explicitly opens it.
4. **State Consistency**: Updates the internal daemon state so that when Antigravity is subsequently launched, it immediately boots with the target account active.

---

## 5. Background Reconcile Latch

Antigravity periodically executes background sync loops that attempt to reconcile local state with previously cached credentials. This can cause "credential flip-flop", where an account switch is immediately reverted by a lagging background process.

The daemon protects against this using a **Reconcile Latch**:
- When an account switch commits, a temporal lock (reconcile latch) activates for a calibrated grace period (30 seconds).
- Any incoming background reconcile payload that matches the old account is rejected.
- Only after the new account establishes its first authenticated request does the latch release, locking in the new account permanently.

---

## 6. Hardware Fingerprint Virtualization

Google Antigravity tracks device installations across several configuration files. If multiple accounts share identical machine identifiers on a single host, accounts can be flagged for abnormal concurrency.

The daemon isolates device profiles per account by virtualizing four hardware parameters:
- `machineid`: Operating system machine UUID.
- `.updaterId`: Auto-updater client identifier.
- `installation_id`: Desktop IDE installation GUID.
- `installation_uuid`: Global installation identifier stored in `antigravity_state.pbtxt`.

When rotating credentials, the daemon swaps the corresponding isolated hardware identifiers atomically, ensuring each configured account presents a consistent, dedicated identity profile.

---

## 7. 1-Token Keep-Alive Warmup Engine

Newly activated accounts or accounts that have been idle for multiple hours can experience cold-start latency (1500ms to 4000ms) on their first user prompt.

To eliminate this delay:
- Following an account switch, the daemon issues a minimal keep-alive warmup probe to the internal endpoint:
  ```http
  POST /v1internal:generateContent
  Content-Type: application/json

  {
    "model": "models/gemini-2.5-pro",
    "contents": [{"parts": [{"text": "ping"}]}],
    "generationConfig": {"maxOutputTokens": 1}
  }
  ```
- The request consumes exactly 1 token.
- Verifies authentication credentials, primes upstream TLS connections, establishes backend routing affinity, and updates the local quota cache before the developer types their next prompt.
