# Antigravity Swiss Knife — Potential Issues Review

Date: 2026-10-13
Scope: `pkg/revival`, `pkg/daemon`, `pkg/process`, `pkg/core`, `pkg/keyring`, `pkg/quota`, `pkg/fingerprint`, `pkg/cache`, `pkg/custommodels`, `pkg/webgui`, `pkg/ipc`, `pkg/vault`, `pkg/system`, `electron/`, `frontend/src`
Method: static code read + live state check (CDP port 38047, `/api/conversations/status`, `conversation_summaries.db`). Severity: Critical / High / Medium / Low.

Live note (2026-10-13 03:11 AEDT): active conversation `a8b23bc4-...` was `isGenerating=true` with `2 subagents running` — healthy, no stall. The issues below are latent defects found in code, not all currently firing.

---

## Summary counts

- Critical: 10 (unauthenticated API + credential/file/RCE sinks, revival dedupe/cooldown/stale-flag defects)
- High: ~25 (process-kill races, sync atomicity, quota misclassification, prune safety, BYOK leak, Electron sandbox/bridge)
- Medium / Low: ~40 (see sections)

---

## 1. Conversation revival (`pkg/revival`, `pkg/gui/injector.go`)

### Critical

1. **Duplicate prompt dispatch — dedupe key unique per call** `pkg/revival/cdp_trigger.go:71,106-110`
   `intentKey = "cdp-<id>-<unixmilli>"` is fresh on every call, so `window.__swissDispatchedIntents[intentKey]` never hits across retries. Outer Go loop (`cdp_trigger.go:287-328`) re-executes the script on every target every 500ms until `success==true`. Same prompt injected + send clicked N times. Queued-duplicate cleanup (`cdp_trigger.go:270-278`) matches one hardcoded string only.

2. **Navigation race: `RestoreConversationPath` goroutine vs `TriggerDesktopContinuation`** `pkg/revival/engine.go:162-177`, `pkg/gui/injector.go:888-901`
   Fire-and-forget 25s restore runs concurrent with 30s trigger. Both mutate SPA router (`history.replaceState` / `location.assign` / link click). `replaceState` changes URL without loading data, so the editor found may belong to the previous conversation. Wrong-conversation injection possible.

3. **Per-conversation cooldown is permanent, not 10 min** `pkg/revival/engine.go:284-299`
   Comment says "within the last 10 minutes" but code is `convRevivedRecently := targetRevivedAt != nil` with no time check. `revivedMap` entries never expire. After one revive/ack, that conversation never re-synthesizes for the process lifetime. Unbounded map growth.

4. **Stale `RUNNING` flags have no freshness check** `pkg/revival/detector.go:180-183,197`, `pkg/revival/detector.go:297-320`
   `WHERE not_fully_idle != 0 OR status='CASCADE_RUN_STATUS_RUNNING'` with no `last_modified_time` recency filter. `checkTrajectoryRunning` checks `steps.status == 2` with no timestamp read. Crash/kill residue reports `NeedsRevival=true` forever. Observed live: 5 depth-2 subagents (`07928807…`, `c8210d02…`, `a1d49b78…`, `8c820128…`, `159efb91…`) stuck `RUNNING` under `IDLE` parents.

5. **`Attempts` increment lost; `MaxAttempts` never enforced** `pkg/revival/engine.go:129-132`, `pkg/revival/store.go:168-185`
   `intent.Attempts++` mutates memory copy; `UpdateStatus` reloads from disk and saves status only. No path checks `Attempts >= MaxAttempts`. Retries until 600s TTL. `MaxAttempts` 2 (`engine.go:74,263`) vs 3 (`engine.go:317`) is decorative.

### High

6. **Store Load-then-Save TOCTOU across processes** `pkg/revival/store.go:168-185`
   `Store.mu` is per-instance only. Daemon + CLI share the file. Concurrent revive/ack interleave; last-writer-wins; `ReviveConversation` (`engine.go:268`) overwrites any pending intent.

7. **`GetRevivalStatus` synthesizes intents without persisting** `pkg/revival/engine.go:303-321`
   In-memory `auto-revive-<conv>` only. Two concurrent status pollers each execute revival → double CDP injection. No mutex around detect → synthesize → execute.

8. **Failed intent wedges system until TTL** `pkg/revival/engine.go:303,131`
   Synthesis requires `(pending == nil || pending.Resumed)`. Failure leaves `Status=Failed, Resumed=false` on disk. No retry/backoff; operator waits 600s or deletes file manually.

9. **CDP broadcast to all matching pages** `pkg/revival/cdp_trigger.go:290-325`
   Every localhost target gets injected/clicked; first `success` wins but all already fired. Two Antigravity windows → both send.

10. **CDP target filter wrong in both directions** `pkg/revival/cdp_trigger.go:295-300`, `pkg/gui/injector.go:146-151,922`
    Prefix allow-list (`127.0.0.1`/`localhost`) misses `file://`/`app://`/custom Electron schemes → false timeout (`cdp_trigger.go:330`). Any unrelated local dev page on loopback passes and receives the script. `Type == "page"` drops webview/iframe editors.

11. **`CapturePreSwitchState` clears the cooldown it depends on** `pkg/revival/engine.go:91-98`
    `delete(revivedMap, ...)` right after persisting intent re-arms the same conversation immediately.

12. **Unconditional `AcknowledgeContinuation` side effects** `pkg/revival/engine.go:334-358`
    `lastRevivedAt = now` set before `LoadIntent`; empty `cascadeID` marks empty-key entry and clears whatever intent is on disk (`engine.go:353`). Unrelated ack suppresses legitimate revivals (45s global + permanent per-conv).

### Medium

13. **Live-CDP vs DB priority inconsistent** `pkg/revival/detector.go:43-60,101-127`
    `Detect` prefers live CDP; `DetectForID` never consults it. Stale renderer URL overrides newer DB state. `CaptureActiveConversationPath` (`injector.go:805-826`) returns first valid path, not focused window.

14. **Success recorded before async navigation completes** `pkg/revival/engine.go:135-150,167-171`
    `lastRevivedAt`/`ClearIntent` set while restore goroutine may still fail. Idle conversations (`prompt == ""`) return nil having done nothing but are marked `Revived`.

15. **TTL/attempt constants drift** `pkg/revival/store.go:60-62,136-139`, `pkg/revival/engine.go:74,263,317`
    Save defaults TTL 600; Load defaults 90 for `ttl <= 0`. Legacy files expire early.

16. **`LoadIntent` mutates files under read lock** `pkg/revival/store.go:101-153`
    `RLock` held while `os.Remove` on invalid/expired intents. Expiry path needs write lock.

17. **In-page 3.5s send poll vs 6s CDP deadline** `pkg/revival/cdp_trigger.go:235-267`, `pkg/gui/injector.go:168-173`
    Slow renderer trips Go deadline; retry uses new `intentKey` while prior script may still click send → duplicate counted as timeout.

18. **`fallbackToAppStorage` nondeterministic** `pkg/revival/detector.go:364-397`
    First valid pinned ID or `candidates[0]` from map iteration (random order). Months-old pinned conversation can resolve as "active" when DB is locked.

### Low

19. **Status aliases internal state; `Success` latches true** `pkg/revival/engine.go:284-285,323-330`
    Returns internal `*time.Time` pointer; callers can mutate. `Success` true forever after first revive.

20. **Port scan on every `Detect`** `pkg/revival/detector.go:44`, `pkg/gui/injector.go:79-126`
    `isHostEnv` effectively always true → file reads + 9-port probe on nearly every status call, including CLI polls.

---

## 2. Account switch + process lifecycle (`pkg/daemon/daemon.go`, `pkg/process/shield.go`, `pkg/core`, `pkg/keyring/sync.go`)

### High

21. **Only main PIDs signaled; Electron children survive** `pkg/process/shield.go:332-343,354-380`
    `mainPIDs` excludes `--type=` children, zygote, `language_server`. Survivors hold `SingletonSocket`, `DevToolsActivePort`, SQLite WAL handles while new instance launches (`shield.go:421-443`). `KillProcessGroup` (`pkg/core/kill_unix.go:10-12`) exists but unused on this path; detached setsid groups (`pkg/core/detach_unix.go:11-16`) would evade it anyway.

22. **`language_server` gets ~0.5s then SIGKILL** `pkg/process/shield.go:383-410`
    5×100ms poll after SIGTERM while LS may checkpoint SQLite. SIGKILL mid-checkpoint orphans WAL/SHM. Comment at `shield.go:359` admits Electron waits up to 5s for LS.

23. **Sync-then-kill has no barrier; 200ms sleep is a guess** `pkg/daemon/daemon.go:571-594,1756-1767,1814-1825`
    `SyncAllSurfaces` then `go { sleep 200ms; RelaunchHostIDE() }`. `secret-tool store` fork (`pkg/keyring/sync.go:172-184`) can exceed 200ms on locked keyring → kill lands mid-write. Concurrent manual + auto-switch both pass `relaunchMu` for kills but nothing serializes the preceding `SyncAllSurfaces` calls.

24. **Singleton files unlinked while orphans may hold socket** `pkg/process/shield.go:414-418,277-282`
    Removes `SingletonLock`/`SingletonSocket`/`SingletonCookie`/`DevToolsActivePort` after only main-PID death confirmed. Old + new instances can both claim singleton; new main may exit as "second instance" or both write `state.vscdb`.

25. **`SyncAllSurfaces` is 8 best-effort writes, errors discarded** `pkg/keyring/sync.go:681-707`
    secret-service → desktop token → app_storage → hardware profile → vscdb → CLI token → google_accounts → oauth_creds → cloud DB, every step `_ =`. Mixed-account states possible (desktop=B, CLI=A; tokens=B with fingerprint=A). No rollback or verify pass. `syncSingleVscdb` (`sync.go:613-658`) swallows lock failures (`_, _ =` at 641,648,654) yet switch reports success → next launch hydrates old-account session.

26. **Token files non-atomic truncate-write** `pkg/keyring/sync.go:339,361,399,447`
    Bare `os.WriteFile` for desktop/CLI/google/oauth tokens. SIGKILL mid-write → 0-byte token → logged-out launch. Only `SyncAppStorageLoginUser` (`223-228`) and vault/fingerprint saves use tmp+rename.

27. **Switch decision uses fresh active + stale standby data** `pkg/daemon/daemon.go:1708`, `pkg/quota/calculator.go:448-454`
    Standbys fall back to disk cache refreshed on slow timer. Successor elected on hours-old 5h fractions → immediate switch-back → ping-pong.

28. **12s reconcile latch vs multi-second cold start** `pkg/keyring/store.go:1493-1501`
    Latch expires while IDE still reports pre-switch user but `IsAntigravityRunning==true`; `ReconcileActiveAccount` flips vault back (`1527-1569`), undoing the switch. Sustained flap.

### Medium

29. **PID-reuse kill in wait loops** `pkg/process/shield.go:360-372,375-380,390-409`
    Liveness = `/proc/<pid>` existence (`494-507`). Recycled PID inside 6.5s window → `Kill()` hits wrong process. No cmdline revalidation.

30. **`conversation_summaries.db` read during kill window** `pkg/gui/injector.go:769-799`
    `busy_timeout=2000`, no `query_only`; silent empty fallback degrades to stale pinned paths.

31. **`cloud_accounts.db` via forked `python3` per switch** `pkg/keyring/cloud_accounts.go:110-124,413-424,443-454`
    External interpreter in critical path; lock contention surfaces as silent no-ops (`except: pass`).

32. **Fresh-token / vault split-brain** `pkg/keyring/sync.go:37-96,687`
    `EnsureFreshAccessToken` mutates memory only; vault persisted later (`daemon.go:577,1754,1812`). Crash between → disk newer than vault or vice versa.

33. **Store mutex held across network + fork I/O** `pkg/keyring/store.go:1474-1475,1512`
    Write lock across `SyncAllSurfaces` (8s HTTPS + `secret-tool` fork) + double `save()`. Slow refresh stalls all readers.

34. **`app_storage.json` lost-update between writers** `pkg/keyring/sync.go:190-229`, `pkg/gui/injector.go:665-690`
    Two unlocked read-modify-write paths (`.tmp` vs `.conv.tmp`). Switch racing revival capture drops `lastLoginUsername` or `last_conversation_path`. Neither pins which account the path belongs to.

35. **Config/vault split-brain on failed activation** `pkg/daemon/daemon.go:1731-1740`
    `Config.SetActiveAppAccount` persisted before `Keyring.SetActiveAccount` verified. Failed second `SetActiveAccount` leaves config=B, vault=A.

36. **`updateAccount`/`removeAccount` relaunch without revival intent** `pkg/daemon/daemon.go:394-412,443-459`
    Unlike switch handlers (`565-568`, `1746-1749`), no `CapturePreSwitchState`. Restoration depends on best-effort CDP only.

37. **Revival fires on no-relaunch path** `pkg/daemon/daemon.go:581-585,1768-1772,1826-1830`
    `ExecutePostRelaunchRevival` launched even when `shouldRelaunch==false`; CLI path spawns `agy --conversation … --continue` (`engine.go:202-203`) while user may be working.

38. **Stale dwell + sharing fallback in individual mode** `pkg/daemon/daemon.go:1677,1742-1744`, `pkg/quota/calculator.go:1364-1383`
    `activeDwellSec` computed once per tick but `lastSwitchTime` updated mid-tick per app. Phase 2 excludes only current app, not accounts assigned earlier in same tick → two depleted apps converge on same successor.

---

## 3. Web API + IPC security (`pkg/webgui/server.go`, `pkg/ipc`, `pkg/system/diagnostics.go`, `pkg/vault`)

### Critical

39. **Entire `/api/*` unauthenticated on TCP 127.0.0.1:8765** `pkg/webgui/server.go:374,380-385`
    Only `corsMiddleware`; zero `checkAuth`/`requireAuth` hits. `handleAuthUnlock` (`3060-3068`) returns `{"success":true}` with no session token. Any local process or loopback-capable page can call all handlers. IPC socket is correctly `0600` (`pkg/ipc/server.go:62-71`) but webgui bypasses it with direct-store fallbacks.

40. **Credential exfiltration endpoints** `pkg/webgui/server.go:516-528,717-735,925-983,2953-3030,5626-5645`
    `handleAccounts` → `store.ListAccounts()` returns decrypted `AccessToken`/`RefreshToken` (`pkg/keyring/store.go:699-713`, serialized `pkg/keyring/models.go:7-8,28-29`). `handleAccountsExport` includes password/TOTP/refresh/access/oauth tokens + `download=1`. OAuth start/exchange echo tokens in body. TOTP GET returns live code for any `?email=`. `handleBridgeTokenRotate` returns fresh `auth_token` + `pairing_pin`, undermining the one authed endpoint (`5515-5540`).

41. **Arbitrary file read/write/delete/rename/copy** `pkg/webgui/server.go:4225-4291,4568-5030`
    `cleanUserPath` only trims/cleans — no jail, no symlink resolution. Read (1MB cap only), write (`MkdirAll 0755` + `WriteFile 0644`), `RemoveAll` on attacker list, rename/copy/move. `copyPathRecursive` (`4801-4854`) recreates symlinks, unbounded recursion, and `OpenFile(... |0644)` widens permissions.

42. **RCE / arbitrary process spawn** `pkg/webgui/server.go:5095-5219,5053-5093,5031-5051,3207-3238,2421-2441,3319-3356`
    `handleFilesOpenIDE` default branch (`5147-5149`) uses `targetIDE` verbatim → `exec.Command(fields[0], ...)` with attacker `dir` (`5205-5210`); persisted via `SetPreferredIDE` (`5261-5294`) = stored injection. Terminal spawn, `xdg-open` on attacker path, unauthenticated `handleDaemonStart`, `handleAppReleaseUpgrade` (network download + apply), `handleSystemFactoryReset`, `handleCachePrune`.

43. **CORS allows any site** `pkg/webgui/server.go:413-437,439-470,5515-5522`
    Returns true for `""`/`"null"`/`"*"` (`441-443`), reflects any origin containing `127.0.0.1`/`localhost` (`459-460`), answers all OPTIONS 204, sets `Allow-Private-Network: true` + reflected `Allow-Origin` with `Allow-Credentials: true`. `/api/bridge/mcp` sets `Allow-Origin: *` with bearer auth. Localhost-only becomes any-website-you-visit.

### High

44. **`SetAppPath`/`SetAccountOverride` accept arbitrary executables** `pkg/core/config.go:255-288`, `pkg/webgui/server.go:3240-3297`
    No existence/absolute-path validation. Executed at `pkg/system/detector.go:244`, `pkg/bridge/bridge.go:293`, `server.go:3743-3821`. Unauthenticated reach.

45. **No rate limiting / request-size limits** `pkg/webgui/server.go` (global)
    Unbounded `Decode`/`ReadAll` (`747`, `5556`), 120s OAuth context per request (`2958`), unbounded SSE hijack (`5542-5553`), per-call `pgrep` forks. `handleAuthUnlock` has no backoff (6-char minimum, `pkg/core/crypto.go:147-155`).

46. **Diagnostics exfiltrates secrets to public GitHub URL** `pkg/system/diagnostics.go:42-49,225-307`
    Redacts only bearer/`gh*`/AIza/`sk-`/emails/paths — not `refresh_token`, `access_token`, `totp_secret`, `id_token`, `client_secret`. Full body embedded in `issue_url` query string (`302-307`) despite "Non-sensitive guaranteed" (`297`). Unauthenticated (`3402-3424`).

47. **Vault path traversal + world-readable copies + destructive sync** `pkg/vault/vault.go:317-402,159-314`
    `cid` only trims `.db`, then `Join(vaultDir, cid+".db")` — `/` and `..` escape. `linkOrCopy` falls back to `os.Create` (0666 & umask); `MkdirAll 0755`. Sync deletes on 60s mtime heuristic (`191-196,221-224`) — clock skew → silent loss. Exposed as POST.

48. **TOCTOU on Stat-then-use paths** `pkg/webgui/server.go:4246-4290,4588-4599,4090-4104,4863-4866`, `pkg/vault/vault.go:325-356`
    Symlink swap between check and use. `desktop_settings.json` read-modify-write has no lock + `0644` (`3192-3199`).

49. **Raw error strings leak internals** `pkg/webgui/server.go:524,537,567,583,612,683,829,1131,1184,1410,3219,3419,4707,4795,5607`
    Absolute paths, socket state, store errors to any localhost caller.

### Medium

50. **Lazy-init data races** `pkg/webgui/server.go:5446-5794` (`bridgeService`), `2349-2381` (detector/release manager), `2444-2451`, `2928-2935`
    Pointer init without mutex; concurrent first-hits race. Confirm with `-race` under parallel load.

51. **Overly permissive modes on sensitive-adjacent data** `pkg/webgui/server.go:4129-4141,3198,4099,4706,4792`
    Memos/vault/file-copies `0644`/`0755` vs correctly strict `config.json 0600` (`pkg/core/config.go:207`), key `0600` (`pkg/core/crypto.go:78`), socket `0600`. Workspace paths + conversation content readable by other local users.

52. **IPC framing fragility** `pkg/ipc/server.go:111-179`, `pkg/ipc/client.go:37-95`
    Newline-delimited JSON, single `ReadBytes`, fixed deadline; oversized result trips deadline. No `SO_PEERCRED` — same-UID only via perms.

---

## 4. Quota, warmup, fingerprint, cache, custom models

### High

53. **Claude/GPT default 1.0 forces false Pro upgrade** `pkg/quota/client.go:758-767,1107-1113`
    `LiveQuotaBreakdown` inits `Quota5hClaudeGPT/QuotaWeeklyClaudeGPT=1.0`; groups path only overwrites when bucket exists. `> 0` check then always true → `Free`/`ProTrial` → `Pro` with `HasClaudeGPTRights=false`. Init to 0.0 + seen-flags.

54. **Flat-bucket fallback misclassifies windows** `pkg/quota/client.go:822-852,793-814`
    `Contains(modelId,"weekly")` else 5h. New-format IDs without window token counted as 5h; weekly stays default 1.0 → false-healthy weekly, wrong switch/reset text.

55. **Warmup ignition: 6 sequential blocking calls, no ctx** `pkg/quota/warmup.go:173-211`, `pkg/daemon/daemon.go:1925-1952,2004-2030`
    6 endpoints × 10s serially (up to ~60s) inside fleet refresh loops. One slow endpoint stalls all accounts. No cancellation on shutdown. Refresh failure still probes with stale token (401s × 6).

56. **Ignition state memory-only; restart → re-ignition storm** `pkg/daemon/daemon.go:54-55,2056-2086,2022-2030`
    `ignitedAccounts` never persisted; every healthy standby re-ignites (generateContent + 2 polls each) after restart, no jitter.

57. **Prune `MaxSizeGB` deletes fresh files** `pkg/cache/pruner.go:100-150`
    With `MinAgeDays==0`, all category matches land in `remainingPrunable`; size-cap loop deletes oldest-first regardless of age. Cap measured over `BrainDir` only while inspector reports brain+conversations (`inspector.go:140`).

58. **BYOK secrets on plaintext GET + key in URL** `pkg/webgui/server.go:2454-2464`, `pkg/custommodels/store.go:96-131`, `pkg/custommodels/tester.go:97-118,149-176`
    `GET /api/custom_models` serializes `APIKey`/`GCPServiceAccountKey` verbatim. Gemini test appends `?key=SECRET` and echoes full URL in `TestResult.Endpoint`.

### Medium

59. **`remainingFraction` unvalidated** `pkg/quota/client.go:681,687,794,811,828-834`
    Raw float (incl. NaN/Inf, <0, >1) stored; only downstream ranking clamps. Clamp + NaN guard at parse.

60. **`resetTime` parse errors swallowed; zero = "Ready"** `pkg/quota/client.go:776-778,825-827`, `pkg/quota/models.go:30-33`
    `time.Parse` error ignored → zero `ResetTime` → `"Ready"`, `ResetSeconds=0`, overestimated availability. No Date-header skew correction (`client.go:573` uses local `time.Now()` only).

61. **5h-fraction picker returns arbitrary/weekly fraction** `pkg/quota/dynamic_frequency.go:206-233`
    First non-`"weekly"` model by API order (unstable; may be tab/autocomplete bucket). Exhausted account (`Quota5hFraction==0`) falls into scan; weekly-only returns weekly as 5h → pristine 5m interval instead of critical 30s. NaN falls through to least-frequent tier (`113-148`).

62. **Post-reset path triples quota cost** `pkg/daemon/daemon.go:1925-1937,2004-2016`
    Verify-poll + generateContent + post-poll every cycle while `shouldIgniteAccount`. Case 2 (`2022-2030`) re-ignites until `ResetSeconds5h<17700`.

63. **Probe uses wrong model/project** `pkg/quota/warmup.go:44-53,168-177,214-237`
    Hardcodes `gemini-3-flash` vs catalog default `gemini-3.8-flash-high` (`catalog.go:101`); always default `aicode-consumers` project, never account's discovered project. Stale ID risks 404 ignition failure.

64. **Fingerprint store case-sensitive; sync swallows errors** `pkg/fingerprint/store.go:203-259`, `pkg/keyring/sync.go:233-298`
    `User@x.com` vs `user@x.com` → duplicate hardware profiles; normalize at entry. All hardware-ID writes `_ =`; pbtxt rename `_ =`; non-`IsNotExist` reads leave empty content and return nil. Daemon believes sync succeeded with stale IDs.

65. **pbtxt updater naive text match** `pkg/keyring/sync.go:260-294`
    Only trimmed `installation_uuid:` prefix lines; misses spacing/quoting/binary variants; appends duplicates. Never cross-checks `installation_id` file vs pbtxt.

66. **No referenced-artifact protection in prune** `pkg/cache/pruner.go:54-121`
    Path-segment + mtime only. "Active cascade shield" (`85-89`) is substring match (over/under-shields) and pruner never walks `ConversationsDir`. `Inspector.SafeToDelete=true` unconditional (`inspector.go:65`). Fractional-day cutoff truncates (`inspector.go:68-72`, `pruner.go:66-70`): sub-hour cutoffs collapse (20min → 0 → everything qualifies).

67. **BYOK keys to arbitrary endpoints, plaintext at rest** `pkg/custommodels/quota.go:713-767`, `pkg/custommodels/models.go:44,47`, `pkg/custommodels/store.go:75-86`
    `APIKey` attached to user-supplied `CustomQuotaEndpoint` + auto-probed suffixes — typo/squat exfiltrates key. `custom_models.json` plaintext (`0600` correct but unencrypted, no keyring). Dual `Authorization` + `x-api-key` headers double leak surface; redaction misses `AIza`/GCP JSON.

### Low

68. **Anti-downgrade masks real downgrades** `pkg/quota/client.go:1158-1164`
    `PlanTier` preserved on upstream Free/empty. Genuine Pro→Free never surfaces.

69. **Strict lowercase-only UUID validation** `pkg/fingerprint/profile.go:11-13,76-90`
    Rejects valid uppercase RFC 4122 IDs → unnecessary regeneration.

70. **Hardware IDs `0644`; hardcoded home paths** `pkg/keyring/sync.go:238-258,302-318`
    World-readable IDs vs `device_profiles.json 0600`. `~/.config/Antigravity` via `os.UserHomeDir()` diverges from `core.GetAntigravityHostConfigDir()`. No cross-process lock (last-writer-wins).

71. **Magic ignition numbers** `pkg/daemon/daemon.go:2048-2075`
    `Quota5hFraction<0.95`, `ResetSeconds5h>=17700`, `now+4h55m` undocumented/inconsistent; unstarted timer surfacing as 0/`Ready` never ignites or ignites spuriously.

---

## 5. Electron + frontend (`electron/`, `frontend/src`)

### High

72. **Renderer bridge + disabled sandbox** `electron/main.js:372-377,19-22`, `electron/preload.js:1-24`
    `sandbox: false` + unconditional Linux `--no-sandbox` while preload exposes `launchAntigravity`/process spawn, `openExternal`, `selectPath`. Page loaded from loopback daemon (`main.js:435`). Any daemon XSS / loopback MITM = full bridge. Fix: `sandbox: true`, drop `no-sandbox`, scope bridge to validated actions.

73. **Unauthenticated daemon trust + PID kill** `electron/daemon-manager.js:164-169,301-317`
    Reuses any responder on 8765 with `daemon_running:true` (port squattable). `process.kill(pid)` uses PID from same unauthenticated `/api/status`.

74. **`desktop:open-external` zero validation** `electron/main.js:530-537`
    Any renderer string → `shell.openExternal`. Daemon data forwarded unchecked (`SystemSettingsPage.tsx:516-548` issue_url, `GitHubWorkspacePage.tsx:1758` `href` allows `javascript:`, `1563,2209` `window.open` from daemon URLs). Allowlist `http(s)` in main + validate before `href`/`window.open`.

75. **Close-to-tray with no tray = orphaned hidden app** `electron/main.js:38,319-321`
    Default `close_to_tray=true`; close hides window, but if icon missing handler returns silently — no way to restore, daemon keeps running headless.

76. **Double teardown race on close** `electron/main.js:392-416,729-757`
    `close` path lacks the `isStoppingDaemon` guard used by `before-quit`/`window-all-closed`. Concurrent `stop({force:true})` overlap.

77. **Fingerprint null-slice crash** `frontend/src/.../FingerprintsPage.tsx:256,266,35-53`
    `prof.machine_id.slice(...)` / `prof.installation_id.slice(...)` with no null guard → whole page crash on partial profile (contract issue #37). `loadProfiles` never re-runs on `accounts` prop change → stale rows.

### Medium

78. **Aspect-ratio lock edge cases** `electron/main.js:362-389,606-637`
    `setAspectRatio(16/9)` + fixed 1216×684 min breaks on small/tiling-WM/fractional-scale displays. Only maximize/fullscreen release lock; snapped/tiled states keep it. E2E exact-4px assertions brittle under decorations/scale.

79. **Single-shot load, no offline state** `electron/main.js:435-444`, `frontend/src/api.ts:39-46`, `frontend/src/App.tsx:110`
    One retry after 1500ms, no `did-fail-load` handler/error page; daemon poll up to 10s (`daemon-manager.js:234`) guarantees blank window on slow start. `api.ts` has no timeout; `App.tsx` 10s poll with no backoff.

80. **Refresh storm / stuck spinner** `frontend/src/.../QuotaDashboardPage.tsx:204-222`
    `for(;;)` polls `/api/quota/fleet` every 1.5s up to 120s (~80 hits), no abort on daemon-down; `.catch(()=>{})` swallows errors → permanent `isRefreshing`.

81. **Gauge contract masking (#36/#38)** `frontend/src/.../QuotaDashboardPage.tsx:408-413`, `types.ts:178-180`, `tray-account-menu.js:50-54`, `HorizontalQuotaBar.tsx:19`, `CircularGauge.tsx:22`
    Fallback chains (`??`) hide missing engine fields; missing quota defaults to `1.0` (renders 100% healthy) in two places; NaN renders as `NaN%` vs `N/A` inconsistently. Use `null` = unknown everywhere.

82. **Ranking divergence** `electron/tray-account-menu.js:55-56`, `frontend/src/.../accountRanking.ts:224-270`, `QuotaDashboardPage.tsx:1099`
    Tray hardcodes `0.5/0.35` soonness (ignores `reset_seconds`); frontend parses horizon text with mode scores. "Next" badge uses `index===1`, not `rankStandbyAccounts()` — wrong when active row isn't index 0 or individual-app mode on.

83. **Tray menu race + fire-and-forget switch** `electron/main.js:347,178-272`, `switchAccountViaHttp:156-176`
    15s interval, no overlap guard; 3 sequential HTTP calls (~2s) guarantee overlap on slow daemon → stale menu. Switch ignores HTTP status, no error UI.

84. **`desktop:start-daemon` TOCTOU spawn** `electron/main.js:510-520`, `electron/daemon-manager.js:162-250`
    No concurrency guard; double-click spawns two children on same port.

85. **Unvalidated CSS/color into persistent injection** `frontend/src/.../AppEnhancementsPage.tsx:610-628,270-290`
    Free-text hex stored and injected into Antigravity CSS/JS. `#fff; background:url(http://evil)`-class values need frontend regex + daemon-side escaping. Same for `project_colors`.

86. **DOM selector injection via conversation_id** `frontend/src/.../GitHubWorkspacePage.tsx:107-108,148`
    Daemon `conversation_id` interpolated into `querySelector` (outside try/catch) and `pushState('/c/'+id)`. Validate `^[A-Za-z0-9_-]+$` first.

87. **TOTP accepts weak secrets, 1s drift** `frontend/src/utils/totp.ts:39-48`, `AccountDetailModal.tsx:244-267`
    Invalid base32 chars skipped instead of rejected (typo → wrong key silently); 2-char secret accepted. 1s poll lags boundary; no ±1-step window. Dual normalizers can drift.

### Low

88. **Notification spoofing** `electron/main.js:522-528` — arbitrary title/body to OS notifications.
89. **PATH-hijack launch fallback** `electron/main.js:562-586` — bare `spawn('antigravity')` via PATH; prefer absolute candidates.
90. **Magic-index tray navigation** `frontend/src/App.tsx:96-108` — `10→7, 1→0`, no bounds check; stale-status bounce (`114-121,409-413`). Use named routes.
91. **Single ErrorBoundary** `frontend/src/App.tsx:369` — one crash blanks main view; per-page boundaries needed.
92. **Raw daemon errors rendered verbatim** `frontend/src/api.ts:42-43`, `QuotaDashboardPage.tsx:445,490` — sanitize to user strings.
93. **`onNavigate` listener leak** `electron/preload.js:12-19`, `frontend/src/App.tsx:96` — never unsubscribed; leaks on remount/HMR.
94. **OAuth paste auto-exchange** `frontend/src/.../AccountDetailModal.tsx:1080-1086` — any pasted `code=` fires token exchange; require submit.
95. **Clipboard without fallback** `AccountDetailModal.tsx:269-274` — unhandled rejection in non-secure context.

---

## Fix priority (highest leverage first)

1. Auth + CORS + secret redaction on webgui (issues 39-43, 58): bearer token on all `/api/*`, deny-by-default CORS, strip tokens/TOTP from list/export/GET, jail file APIs with `EvalSymlinks` + prefix check + `O_NOFOLLOW` + body caps, allow-list exec binaries.
2. Revival correctness (1-5, 8): stable idempotency key per intent (not per millisecond), persist synthesized intents, enforce `Attempts`/`TTL`, add `last_modified` freshness to RUNNING detection, expire `revivedMap` (10 min) instead of permanent suppression.
3. Switch atomicity + kill safety (21-26): barrier-gated kill (no 200ms guess), full process-tree termination + socket-release check before unlink/relaunch, tmp+rename for all token writes, error-returning `SyncAllSurfaces` with verify pass.
4. Quota/warmup (53-56, 62): zero-init Claude/GPT fractions, window-token classification fix, clamp/validate fractions and `resetTime`, persist ignition state, single-flight probe with ctx.
5. Prune + fingerprint + BYOK (57, 64-67): size-cap respects age, reference-aware prune, case-normalized emails, error-returning hardware sync, redact BYOK on read, header-only key auth.
6. Electron/frontend (72-77, 80-84): `sandbox: true`, validated `openExternal`, tray-close fallback, teardown gate, null-safe fingerprint rendering, bounded refresh with errors, shared ranking module.

---

## 6. Detailed verification & testing (2026-10-13)

Existing suites all green — they pass but do not cover the defects above (mocks replace CDP/clock/HTTP):

| Suite | Result |
|---|---|
| `go test ./pkg/revival/...` | 28 PASS, 0 FAIL |
| `go test ./pkg/quota/...` | 57 PASS, 0 FAIL |
| `go test ./pkg/cache/... ./pkg/fingerprint/... ./pkg/custommodels/...` | 39 PASS, 0 FAIL |
| `npm test --prefix frontend` | 556 tests / 173 suites, 0 FAIL |
| `node --test electron/tray-account-menu.test.js` | 39 tests / 9 suites, 0 FAIL |
| `go vet ./pkg/revival/... ./pkg/webgui/... ./pkg/daemon/...` | clean, 0 diagnostics |
| TOTP RFC 6238 vectors (`totp.ts`) | correct truncation; full 32-char secret at t=59 → `287082` matches RFC; short 16-char secret is a different key (`263420`), not an implementation bug |

### Runtime proofs (live daemon 127.0.0.1:8765, repro scripts in `/tmp/opencode/`)

| # | Claim | Proof | Status |
|---|---|---|---|
| 3 | Per-conv cooldown permanent | `engine.go:296` is `convRevivedRecently := targetRevivedAt != nil` — no `Since()` check, while comment promises 10 min; global path uses `Since<45s` | CONFIRMED (code) |
| 5 | `Attempts` increment lost | `repro_revival.go`: Save → `Attempts++` → `UpdateStatus(Failed)` → reload gives `got=0 want=1` | CONFIRMED (runtime) |
| 8 | Failed intent wedges synthesis | `engine.go:303` requires `pending==nil \|\| pending.Resumed`; failed intent stays `Failed`+unresumed until 600s TTL | CONFIRMED (code) |
| 40 | `/api/accounts` leaks tokens, no auth | `curl /api/accounts` → 200, 16 accounts, keys include `access_token`, `refresh_token`, `id_token` | CONFIRMED (runtime) |
| 43 | CORS bypass | `Origin: null` → `Allow-Origin: *`; `Origin: http://127.0.0.1.evil.com` → reflected with `Allow-Credentials: true` | CONFIRMED (runtime) |
| 41 | Arbitrary file read | `GET /api/files/read?path=/etc/hostname` → `{"success":true,"content":"David-Laptop\n",...}` | CONFIRMED (runtime) |
| 42 | `open_ide` arbitrary binary | `POST /api/files/open_ide {"ide":"echo-pwned-marker"}` → reaches exec path (`success:false` only because binary absent, no allow-list rejection) | CONFIRMED (runtime) |
| 64 | Fingerprint email case duplicates | `repro_cache_fp.go`: `GetOrCreateProfile("User@Example.com")` vs `"user@example.com"` → different `MachineID` | CONFIRMED (runtime) |
| 66 | Fractional-day cutoff truncates | 20 min = 0.0139 d → `time.Duration(0.0139*24)*time.Hour` = `0s` → everything qualifies | CONFIRMED (runtime) |
| 57 | Size-cap deletes fresh files | `pruner.go:100-118` + `130-150`: with `MinAgeDays==0` all category matches enter `remainingPrunable`, then oldest-first deleted regardless of age | CONFIRMED (code) |
| 87 | TOTP silently skips bad chars | `totp.ts:40-41`: `if (val === -1) continue`; only empty result returns null — typo'd secrets and 2-char secrets accepted | CONFIRMED (code) |
| 53-54 | Quota Claude/GPT 1.0 default + window misclassification | `client.go:758-767,1107-1113` (`> 0` always true → false Pro) and `822-852` (`Contains "weekly"` else 5h) | CONFIRMED (code) |

### Coverage gaps in existing tests (why green suites miss these)

- Revival tests mock `ScriptExecutor`/`TargetFinder` — never exercise unique-`intentKey` dedupe, multi-target broadcast, or `RestoreConversationPath` vs trigger concurrency.
- No test backdates `revivedMap` entries past 10 min, increments `Attempts` through `UpdateStatus`, or leaves a `Failed` intent and calls `GetRevivalStatus` again.
- No test feeds a flat bucket list without `weekly` in any `modelId`, or Claude/GPT-absent groups, then asserts `PlanTier`.
- Cache tests never combine `MinAgeDays=0` with `MaxSizeGB>0` and fresh files; fingerprint tests never vary email case.
- No webgui test asserts auth requirement, CORS deny, secret redaction, or path jailing (tests run against store directly).
- Frontend tests mock `fetch`; `QuotaDashboardPage` 1.5s×120 poll loop, ranking divergence, and `index===1` "Next" badge are unasserted.
