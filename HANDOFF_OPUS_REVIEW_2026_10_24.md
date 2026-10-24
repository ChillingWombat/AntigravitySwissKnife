# Opus 5.5 Handoff — Full Codebase Review, Security Hardening & Optimization (2026-10-24)

> **Purpose**: Self-contained handoff for an **Opus 5.5** agent resuming this review & optimization wave from another session/harness after Opus 5.5 quota exhaustion in conversation `c20fad1b-91bc-4686-8e93-31acfd1657e2`.
>
> **Companion Files**:
> - Original Review Artifact: `/home/david/.gemini/antigravity/brain/c20fad1b-91bc-4686-8e93-31acfd1657e2/review.md`
> - Brain Handoff Artifact: `/home/david/.gemini/antigravity/brain/c20fad1b-91bc-4686-8e93-31acfd1657e2/handoff.md`
> - Workspace Checkpoint: `.agents/state/checkpoint.md`
> - Project Standing Brief: `AGENTS.md` & `HANDOFF.md`

---

## 1. Context & Architecture of This Wave

1. **Initial Baseline Assessment (`main @ 39e1e84`)**:
   - Size: ~208k LOC across 664 tracked files (Go companion daemon in `cmd/swiss` + `pkg/...`, Electron main/preload in `electron/...`, React 19 + TypeScript + Vite frontend in `frontend/src/...`).
   - `go build ./...`, `go vet ./...`, and `cd frontend && npx tsc -b --noEmit` passed cleanly at baseline.
   - `staticcheck ./...` had **78 findings** (including dead code `U1000`, unread assignments `SA4006`, deprecated APIs `SA1019`, and simplifications `S1011`/`S1017`/`S1039`/`SA4016`/`SA6005`; plus `ST1005` capitalized error strings which were intentionally excluded because UI/tests assert on them).
   - `cd frontend && npx oxlint` had **97 warnings** (`25 no-unused-vars`, `10 react/immutability`, `7 react/purity`, `5 react/no-children-prop`, `5 react-hooks/exhaustive-deps`, `1 react/refs`, `1 no-useless-escape`, etc.).
   - `~/go/bin/govulncheck ./...` reported **13 reachable stdlib CVEs** in `go1.27.1` (`GO-2026-6599` through `GO-2026-6809` in `net/http`, `crypto/tls`, `crypto/x509`, `html/template`, `net/textproto`), all fixed in `go1.27.2` / `go1.27.3`.
   - Baseline tests:
     - Electron (`node --test electron/*.test.js`): PASS.
     - Frontend (`cd frontend && npm test`): 735/735 PASS.
     - Go (`go test ./pkg/...`): all packages passed **except** `pkg/acpmesh` (`TestProbeAgentHandshakeRealPi`), which failed because `probePiRPC` timed out and fell back to `pi-acp` (`stdio-acp` instead of `pi-rpc`).

2. **Why Flash Workers Need Opus 5.5 Audit**:
   - Per David's instruction, Opus 5.5 performed the architectural & security review (`review.md`) and delegated the bounded implementation work across **5 parallel `DeepCoder` Flash workers** with strictly disjoint file write scopes.
   - All 5 Flash workers have landed their Layer 0 code & test changes in the working tree (`54+ files modified`) and are completing their `DeepCoderImprovementWorker` passes.
   - **Because Flash models can introduce subtle edge-case regressions or formatting noise**, the resuming **Opus 5.5** agent must independently audit each worker's diff against the checklist in **Section 3** before building, relaunching the daemon/app, and opening the PR.

---

## 2. Complete Findings Matrix (`review.md` → Root Cause → Worker Fix)

### 2.1 Security Findings (`S1`–`S9`)

| ID | Severity | Finding & Root Cause | Assigned Worker & Exact Fix Landed |
|---|---|---|---|
| **S1** | **High** | **`getClientIP` XFF Brute-Force Lockout Bypass** (`pkg/bridge/auth.go:239-257`): `getClientIP(r)` unconditionally trusted `X-Forwarded-For[0]` and `X-Real-IP` from any peer. An attacker could rotate `X-Forwarded-For` on every request to bypass the 5-attempt / 15-minute IP lockout on both Bridge auth and `/remote` PIN auth. | **W1 (`pkg/bridge`)**: Rewrote `getClientIP` (`pkg/bridge/auth.go:244-275`) to parse host from `r.RemoteAddr` first. Proxy headers (`Cf-Connecting-Ip` → **rightmost** non-empty `X-Forwarded-For` → `X-Real-IP`) are consulted **only** when `net.ParseIP(host).IsLoopback()` (traffic arriving via local cloudflared/tailscale tunnel). Added tests in `pkg/bridge/auth_test.go` and updated `pkg/bridge/challenger_m4_test.go:161-168`. |
| **S2** | **High** | **OAuth Endpoints Bypass PIN Brute-Force Lockout** (`pkg/bridge/oauth.go:694-760`, `919-930`): `ServeOAuthAuthorize` (GET `?pin=` fast-path and POST form) and `ServeOAuthToken` (`refresh_token` / `client_credentials` grants) called `VerifyCandidatePIN` directly without consulting or updating `AuthManager`'s `lockedUntil`/`failedAttempts` map, allowing unlimited PIN/App-Password brute-forcing via `/oauth/authorize` or `/oauth/token`. | **W1 (`pkg/bridge` — Final Pipeline Complete)**: Extracted shared lockout helpers (`isLockedLocked`, `recordFailureLocked`, `recordSuccessLocked`) and added `(*AuthManager).VerifyPIN(r, candidate) (ok, locked bool)` in `pkg/bridge/auth.go:278-353` (accepts PIN, App Password, or `am.token`). Wired `ServeOAuthAuthorize` (unconditionally on GET even when `pin == ""` so locked IPs get 429 before rendering the form, without incrementing failures on empty `pin`, plus POST) and `ServeOAuthToken` (`refresh_token`/`client_credentials`) to `s.authMgr.VerifyPIN`. Added regression tests in `pkg/bridge/oauth_test.go:709-726` and new `pkg/bridge/remote_test.go`. |
| **S3** | **Medium** | **OAuth `authorization_code` Grant Missing Checks** (`pkg/bridge/oauth.go:836-908`): (a) `clientID` was parsed from body/form/BasicAuth and never checked against `entry.ClientID` (flagged by `staticcheck SA4006`); (b) `if entry.RedirectURI != "" && redirectURI != ""` allowed omitting `redirect_uri` in the token request even when `authorize` bound one; (c) unknown `code_challenge_method` fell through to plain-text comparison instead of being rejected. | **W1 (`pkg/bridge`)**: In `pkg/bridge/oauth.go:881-945`: (a) if `entry.ClientID != ""` and `clientID != entry.ClientID` → 400 `invalid_grant` (`"client_id mismatch"`); (b) if `entry.RedirectURI != ""` and `redirectURI != entry.RedirectURI` → 400 `invalid_grant`; (c) PKCE accepts only `"S256"` (case-insensitive) or `"plain"`/`""`, returning 400 `invalid_request` otherwise. Added unit tests in `pkg/bridge/oauth_test.go`. |
| **S4** | **Medium** | **CORS Echoes External Origins with `Allow-Credentials: true`** (`pkg/webgui/server.go:727-754`): For paths matching `isExternalOriginAllowedPath` (`/remote`, `/mcp`, `/oauth`, webhooks), `corsMiddleware` echoed any caller `Origin` **together with** `Access-Control-Allow-Credentials: true` and omitted `Vary: Origin`. Because `/remote` authenticates via the `swiss_remote_pin` cookie, any website could make credentialed cross-origin requests to `/remote/*`. | **W2 (`pkg/webgui`)**: Updated `corsMiddleware` in `pkg/webgui/server.go:734-750` to always set `Vary: Origin` when `Origin` is present, and only emit `Access-Control-Allow-Credentials: true` when `isAllowedLoopbackOrigin(origin)` is true. External origins on public/bridge paths still get `Access-Control-Allow-Origin` without `Allow-Credentials`. |
| **S5** | **Medium** | **Fail-Open `core.LoadConfig` on Corrupt JSON** (`pkg/core/config.go:201-204`): On `json.Unmarshal` error, `LoadConfig()` returned `DefaultConfig(), nil` (`AppPasswordEnabled: false`). If `config.json` became truncated/corrupted on disk, `/api/auth/unlock` (`pkg/webgui/server.go:3956`) saw `err == nil && !cfg.AppPasswordEnabled` and minted a valid 24h session token with no password. | **W2 (`pkg/core`)**: Updated `pkg/core/config.go:203` to return `DefaultConfig(), fmt.Errorf("parse %s: %w", cfgPath, err)` on unmarshal failure so `handleAuthUnlock` fails closed with HTTP 500. Added `TestLoadConfig_CorruptJSON_ReturnsError` in `pkg/core/config_test.go:455-470`. |
| **S6** | **Low** | **Non-Constant-Time Secret Comparisons**: Token, PIN, and PKCE comparisons in `pkg/bridge/auth.go`, `pkg/bridge/oauth.go`, and `pkg/webgui/auth_middleware.go` used raw `==`. | **W1 & W2**: Added `secureCompare` (`subtle.ConstantTimeCompare`) in `pkg/bridge/auth.go:125-128`, `pkg/bridge/oauth.go`, and `pkg/webgui/auth_middleware.go:205,226`. |
| **S7** | **Low** | **Unconditional `InsecureSkipVerify` in Remote Proxy** (`pkg/bridge/remote.go:346-400`): `resolveUpstreamCSRF` and `proxyToAntigravityWebApp` disabled TLS verification for all origins, including non-loopback `SetWebOriginOverride` URLs. | **W1 (`pkg/bridge`)**: Split into `remoteLoopbackTransport` (`InsecureSkipVerify: true` only when upstream host is loopback `127.0.0.1`/`localhost`/`::1`) and `remoteVerifiedTransport` (standard TLS verification) in `pkg/bridge/remote.go:345-385`. |
| **S8** | **Low** | **Inconsistent Electron IPC Sender Validation** (`electron/main.js:838-1010`): Only 3 of 15 `ipcMain.handle` channels checked `event.sender === mainWindow.webContents`; `desktop:get-session-token` (which returns the daemon auth token) and 11 others had no sender or frame origin check. | **W4 (`electron`)**: Added `isTrustedSender(event, targetWindow = mainWindow)` in `electron/main.js:128-140` checking window liveness, `event.sender === targetWindow.webContents`, and `isTrustedDaemonOrigin(event.senderFrame?.url)`. Enforced across all 15 `ipcMain.handle` handlers and tested in `electron/navigation-security.test.js:254-308`. |
| **S9** | **Low** | **Missing `Secure` Flag on `swiss_remote_pin` Cookie Over HTTPS Tunnel** (`pkg/bridge/remote.go:969-999`): Cookie lacked `Secure: true` when served over HTTPS / Cloudflare tunnel. | **W1 (`pkg/bridge`)**: Set `Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")` in both `handlePINAuth` and `handleLogout` (`pkg/bridge/remote.go:1035,1055`). |

---

### 2.2 Resource Leaks & Performance Findings (`P1`–`P6`)

| ID | Finding & Root Cause | Assigned Worker & Exact Fix Landed |
|---|---|---|
| **P1** | **Per-Call `http.Transport` Leak in Language Server Quota Client** (`pkg/quota/language_server.go:361-405`): `newLanguageServerHTTPClient()` allocated a brand-new `*http.Transport` (with no `IdleConnTimeout`) on every quota RPC call, leaking idle keep-alive connections/goroutines and repeating TLS handshakes on every poll. | **W3 (`pkg/quota`)**: Converted `newLanguageServerHTTPClient()` (`pkg/quota/language_server.go:391-419`) to return a `sync.Once` package-level singleton `*http.Client` with `IdleConnTimeout: 30 * time.Second` and `MaxIdleConnsPerHost: 4`. |
| **P2** | **Per-Request `http.Transport` Allocation in Remote ReverseProxy & CSRF Probe** (`pkg/bridge/remote.go:346-400`): Both `resolveUpstreamCSRF` and `proxyToAntigravityWebApp` allocated a new `*http.Transport` on every single HTTP request to `/remote/*`, and `ReverseProxy` used deprecated `Director` (`SA1019`). | **W1 (`pkg/bridge`)**: Shared package-level transports (`remoteLoopbackTransport`, `remoteVerifiedTransport`) with `IdleConnTimeout: 90 * time.Second` and migrated `ReverseProxy` from `Director` to `Rewrite` (`pkg/bridge/remote.go:345-465`). |
| **P3** | **Auth Middleware Hot-Path Disk Read & 100k-Round PBKDF2 on Stale Session Tokens** (`pkg/webgui/auth_middleware.go:164-230`): (a) Every unauthenticated request read and parsed `config.json` from disk even when no credentials were sent; (b) when a browser/renderer sent a stale 64-hex session token from a previous daemon run, `validateRequestAuth` fell through to `cfg.VerifyAppPassword(candidate)`, running 100,000 rounds of PBKDF2-SHA256 on every polled request. | **W2 (`pkg/webgui` — Final Pipeline Complete)**: In `pkg/webgui/auth_middleware.go:51-250`: consolidated `s.authMu.RLock()` into a single critical section, and updated the early exit guard before `core.LoadConfig()` to `if appPasswordCandidate == "" && (candidate == "" || isSessionTokenShaped(candidate)) { return false }` — skipping both the `config.json` disk read and the 100k-round PBKDF2 when `candidate` is empty or 64-hex session-token shaped (`X-App-Password` / `?password=` still always checks `VerifyAppPassword`). Added `pkg/webgui/cors_and_auth_hotpath_test.go` (`TestCORS_VaryAndCredentials` including `OPTIONS` preflight, `TestAuthHotPath_LazyConfigLoad`, `TestAuthUnlock_CorruptConfig_FailsClosed`). All 22 Go packages pass `go test -count=1 ./pkg/...`. |
| **P5** | **Unbounded HTTP Clients (`http.DefaultClient` / `http.PostForm`)**: `pkg/bridge/channels_discord.go:123` and `pkg/keyring/google_oauth.go:535` used `http.DefaultClient` with no timeout. | **W1 & W3**: Added `discordHTTPClient` (`Timeout: 15s`) in `pkg/bridge/channels_discord.go:121-126` and `googleOAuthHTTPClient` (`Timeout: 30s`) in `pkg/keyring/google_oauth.go:34,537`. |
| **P6** | **Unbounded `s.sessionTokens` Map Growth** (`pkg/webgui/server.go:134-145`, `auth_middleware.go:199-204`): Expired 24h session tokens were never deleted from `s.sessionTokens`. | **W2 (`pkg/webgui`)**: Deletes expired token on lookup in `validateRequestAuth` (`pkg/webgui/auth_middleware.go:213-223`) and sweeps all expired entries inside `createSessionToken` (`pkg/webgui/server.go:140-145`). |

---

### 2.3 Correctness, Test Failure & Hygiene Findings

| Item | Location | Fix Landed |
|---|---|---|
| **Baseline Test Failure: `TestProbeAgentHandshakeRealPi`** | `pkg/acpmesh/handshake.go`, `handshake_test.go` | **W5 (`pkg/acpmesh` — Final Pipeline Complete)**: `probePiRPC` previously forced `context.WithTimeout(ctx, 4*time.Second)` and spawned `pi` without `--no-extensions --no-mcp --no-skills` (causing `pi` to start 21 MCP servers and `pi-lens-lsp`, taking >4s and falling back to `pi-acp`). Split into `probePiRPC` / `probePiRPCOnce` passing `--no-extensions --no-mcp --no-skills` first (dropping real Pi RPC probe latency from >4s to **0.17–0.29s** and CPU by ~40x, with fallback to basic flags for older `pi` binaries), added `probeContext(ctx)` (honors longer caller deadline, raises default timeout 4s → 8s), appended `(pi-rpc fallback: ...)` diagnostic to `result.Summary`, and replaced hardcoded `~/.config/nvm/versions/node/v24.16.0/bin/pi-acp` with SemVer 2.0.0-compliant (`v24.16.0 > v24.16.0-rc.1`), deduplicated glob discovery across `$NVM_DIR`, `~/.config/nvm`, and `~/.nvm`. All 25 test suites pass in 0.62s. |
| **React Prop Mutation (`account.* = ...`)** | `frontend/src/components/AccountDetailModal.tsx:130-725`, `frontend/src/utils/accountPresentation.test.ts:210` | **W4 (`frontend` — Final Pipeline Complete)**: Removed direct mutations to `account.plan_tier`, `account.error_message`, and `account.status`; updated `isAccountInError`, `isQuotaConnected`, `st`, `errorAlert`, and `renderPlanTierBadge` to read exclusively from local state (`currentPlanTier`, `liveErrorMessage`, `currentStatus`) so post-refresh recovery transitions cleanly out of `ERROR` state; updated `accountPresentation.test.ts` assertion without comment facades. |
| **Render-Time `window.*`, `ref.current` & TDZ Fixes** | `frontend/src/pages/GitHubWorkspacePage.tsx:85-185`, `QuotaDashboardPage.tsx:148-154`, `AppEnhancementsPage.tsx:209-248` | **W4 (`frontend` — Final Pipeline Complete)**: Wrapped `navigateToConversation` in `useCallback` and moved `window.navigateToConversation` / `window.jumpToConversation` assignment into `useEffect` with unmount cleanup; moved `isVerifyingRef.current = isVerifyingErrorAccount` into `useEffect`; declared `loadConfig` before mount `useEffect` in `AppEnhancementsPage.tsx` (eliminating all 10 `react/immutability` warnings). |
| **All Non-`ST1005` `staticcheck` Findings** | `pkg/bridge`, `pkg/webgui`, `pkg/quota`, `pkg/keyring`, `pkg/custommodels`, `pkg/github`, `pkg/gui`, `pkg/system`, `pkg/importer` | **W1, W2, W3 (W3 Final Pipeline Complete)**: Removed dead functions/consts/fields (`U1000`), replaced deprecated `strings.Title` and `filepath.HasPrefix` (`SA1019`) and `ReverseProxy.Director` (`SA1019`), fixed unread assignments (`SA4006` — including explicit `*cacheP` context-caching rate assertions at `pkg/custommodels/custommodels_test.go:799,805,831`, `pName` assertions in `pkg/importer/importer_test.go:28-40`, and `tier != PlanTierFree` in `pkg/quota/client_test.go:195`), and applied `S1011`, `S1017`, `S1039`, `SA4016`, `SA6005`. |
| **Frontend `oxlint` Cleanup** | `frontend/src/**` | **W4 (`frontend`)**: Reduced `oxlint` warnings from **97 → 55** (0 errors), eliminating all `no-unused-vars` (25), `react/no-children-prop` (5), `no-useless-escape` (1), `react/immutability` (9), `react/refs` (1), and `useContextMenuPlacement.ts` `exhaustive-deps` (1). |

---

## 3. Critical Opus 5.5 Audit Checklist (Verify Flash Workers' Code!)

When you resume as **Opus 5.5**, do **not** blindly trust the Flash workers' reports. Run `git diff` and inspect each of these potential pitfalls:

1. **Check `pkg/core/config.go` Line Endings & Callers of `core.LoadConfig()`**:
   - `pkg/core/config.go` had CRLF line endings; W2 ran `gofmt` which converted the file to LF (`179 insertions(+), 179 deletions(-)`). Restore CRLF on unchanged lines (or check `.gitattributes`) so the diff on `pkg/core/config.go` is strictly the 1-line `fmt.Errorf("parse %s: %w", cfgPath, err)` change.
   - Verify no caller of `core.LoadConfig()` across `pkg/...` breaks when a corrupt `config.json` returns `DefaultConfig(), nonNilErr` (note that `LoadConfig()` still returns a non-nil `DefaultConfig()` alongside the error, so callers that do `cfg, _ := core.LoadConfig()` continue to get `DefaultConfig()`, while security-sensitive callers like `handleAuthUnlock` check `if err != nil` and fail closed).
2. **Audit `isTrustedSender` in `electron/main.js`**:
   - W4 added `isTrustedSender(event, targetWindow = mainWindow)` requiring `isTrustedDaemonOrigin(event.senderFrame?.url)`.
   - **Critical check**: When the daemon is temporarily unreachable on startup, `electron/main.js` loads a `data:text/html;charset=utf-8,...` fallback page (`showConnectingFallback`) whose "Retry Connection" button calls `desktop:retry-daemon-connection` via preload IPC! Verify that `isTrustedDaemonOrigin` or `isTrustedSender` accepts the connecting fallback `data:` URL (or that `desktop:retry-daemon-connection` allows `mainWindow.webContents` when on the fallback page). If not, clicking "Retry Connection" on the fallback screen would be rejected!
3. **Audit `isSessionTokenShaped` in `pkg/webgui/auth_middleware.go`**:
   - W2 skipped `cfg.VerifyAppPassword(candidate)` when `candidate` is 64 lowercase hex chars (`isSessionTokenShaped`).
   - Confirm that `appPasswordCandidate` (`X-App-Password` header / `?password=` query param) is **always** checked against `cfg.VerifyAppPassword(appPasswordCandidate)` regardless of shape, and only `candidate` (`Bearer` / `X-Session-Token` / `?token=`) skips PBKDF2 when 64-hex shaped.
4. **Audit `VerifyPIN` & Lockout in `pkg/bridge/auth.go` and `pkg/bridge/oauth.go`**:
   - In `ServeOAuthToken` (`refresh_token` / `client_credentials` branch), `candidate` can be either the raw bridge auth token (`s.GetRawAuthToken()`) OR the pairing PIN / App Password.
   - **Critical check**: Verify that `s.authMgr.VerifyPIN(r, candidate)` (or `ServeOAuthToken`) still accepts `s.GetRawAuthToken()` (which is the `refresh_token` issued by the `authorization_code` grant at `oauth.go:913`!) as well as the PIN/App Password! If W1 replaced `candidate == s.GetRawAuthToken() || VerifyCandidatePIN(...)` with *only* a PIN check, then `grant_type=refresh_token` with the token returned by `authorization_code` will fail with 401! Check `pkg/bridge/oauth.go` lines 940–965 and `pkg/bridge/auth.go` `VerifyPIN` immediately.
5. **Audit `Rewrite` Migration in `pkg/bridge/remote.go`**:
   - In `httputil.ReverseProxy`, when migrating from `Director` to `Rewrite(pr *httputil.ProxyRequest)`:
     - Verify `Rewrite` sets `pr.Out.URL.Scheme`, `pr.Out.URL.Host`, `pr.Out.Host`, `Origin`, `Referer`, `x-codeium-csrf-token`, and `Accept-Encoding` identically on `pr.Out`.
6. **Orchestrator-Owned Root / CI / Toolchain Updates (Not Yet Applied)**:
   - Update `.github/workflows/race.yml` (lines 30, 66, 95) and `.github/workflows/release.yml` (line 88) from `go-version: '1.24.x'` / `'1.23.x'` to `go-version-file: 'go.mod'`.
   - Check if `go1.27.3` toolchain is available (`GOTOOLCHAIN=go1.27.3+auto go env GOVERSION`) to eliminate the 13 `govulncheck` stdlib vulnerabilities.
   - Note that `pkg/webgui/dist/` currently has dirty changes from W2/W4 running builds; run a single clean `npm run build` at the very end after all source audits pass (`HANDOFF.md` §4).

---

## 4. Exact Commands for Resuming Opus 5.5 Agent

```bash
# 1. Inspect subagent completion & current diff
cd "/mnt/Data/Projects/Antigravity Swiss Knife"
git status --short
git diff --stat

# 2. Audit the 5 critical checkpoints in Section 3 above:
git diff pkg/bridge/auth.go pkg/bridge/oauth.go pkg/bridge/remote.go
git diff pkg/webgui/server.go pkg/webgui/auth_middleware.go pkg/core/config.go
git diff electron/main.js frontend/src/components/AccountDetailModal.tsx

# 3. Run full static analysis & test suites across Go, Frontend, and Electron:
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./... 2>&1 | grep -v ST1005
(cd frontend && npx tsc -b --noEmit && npx oxlint)
npm test

# 4. Rebuild production bundles & binaries (Mandatory per AGENTS.md §6):
npm run build

# 5. Relaunch active daemon / desktop app if running (NEVER kill Google Antigravity IDE) & sync PR to origin (AGENTS.md §3 & §6).
```

## 5. Stage 0 & Baseline Preservation (Completed)

- Preserved the 58-file dirty working tree baseline on `feature/opus-review-hardening` at `0c32139`, under Spec `#314` and Ticket `#315`.
- Removed the `pkg/core/config.go` CRLF and whitespace diff churn. The fail-closed unmarshal error at `pkg/core/config.go:203` is now the only change: `1 insertion(+), 1 deletion(-)`.
- `P4` was a numbering gap between `P3` and `P5` in the earlier findings table. No finding, worker assignment, or fix was omitted.

## 6. Second-Wave Deep Review Findings & Fixes (Tickets #316-#322)

### 6.1 S10 / P7 - Bridge Auth and OAuth Token Separation (`#316`)

Files: `pkg/bridge/auth.go`, `pkg/bridge/oauth.go`.

- Moved `VerifyCandidatePIN` (`core.LoadConfig()` and PBKDF2) outside `am.mu`, then ordered the constant-time token comparison before PBKDF2.
- `VerifyCandidatePIN` now returns `false` when `useAppPassword=true` and `core.LoadConfig()` fails. A corrupt config no longer falls back to the 6-digit PIN.
- Canonicalized proxy and remote IPs with `net.ParseIP`. Added expiry sweeps and a 1024-IP cap to `failedAttempts` and `lockedUntil`.
- Added `VerifyInteractivePIN` for `/oauth/authorize`. Raw bearer tokens (`am.token`) are rejected in URL query and form PIN fields. `/oauth/token` still accepts `am.token` for `refresh_token` and `client_credentials`.

### 6.2 S11 / P8 - WebGUI Authentication Hot Path (`#317`)

Files: `pkg/webgui/auth_middleware.go`, `pkg/webgui/server.go`.

- Added stat-validated (`modTime` and `size`) in-memory caching to `GetLocalSessionToken()`. A renderer with a stale 64-hex token no longer causes a disk read on every poll.
- Added a 5-attempts/sec per-IP rate limit before the legacy non-hex Bearer password fallback. This stops PBKDF2 CPU amplification.
- Changed `createSessionToken()` to return `(string, error)`. A `crypto/rand.Read` failure now returns HTTP 500 on unlock paths instead of minting an all-zero token.
- CORS middleware now preserves existing `Vary` headers and does not duplicate `Origin`.

### 6.3 C1 / P9 - ACPMesh Timeout and Quota Diagnostics (`#318`)

Files: `pkg/acpmesh/handshake.go`, `pkg/quota/client.go`.

- `PollAccountLiveQuota` now preserves post-refresh `FetchProjectAndTier` errors in diagnostics instead of discarding them with `_`.
- Bounded overall `probePiHandshake` execution with a 12s default ceiling.
- Deduplicated `resolvePiACPExecutable()`.
- Broadened `isUnsupportedPiRPCFlagError` to cover 6 CLI flag rejection forms.

### 6.4 S12 - Electron Subframe and Connecting Fallback IPC (`#319`)

Files: `electron/main.js`, `electron/navigation-security.test.js`.

- `isTrustedSender` now verifies `mainFrame` identity and routing, so subframes cannot call privileged IPC handlers.
- Replaced blanket `data:text/html` trust with an exact match against the generated connecting fallback URL.

### 6.5 C2 / P10 - Frontend Polling and AbortSignal Lifecycle (`#320`)

Files: `frontend/src/App.tsx`, `frontend/src/api.ts`.

- Added the `isPollingRef` single-flight guard and hidden-tab pause/resume to the 10s status and account polling loop. A poll longer than 10s is no longer repeatedly self-aborted by GET deduplication in `api.ts`.
- Removed caller `AbortSignal` listeners in `apiFetch` `finally` blocks. Long-lived signals no longer accumulate listeners.

### 6.6 C3 - Frontend Async Races and Render Purity (`#321`)

- Guarded `AccountDetailModal` live-quota fetches and 1s async TOTP generation against stale account or secret switches and unmounts. Restored the strict `getAccountHeaderDisplay` assertion in `accountPresentation.test.ts`.
- Coordinated `SystemSettingsPage` `runtimeMode` initialization and awaited `navigator.clipboard.writeText` before showing the copied state.
- Fixed `exhaustive-deps` and render-time `Date.now()` purity warnings in `GitHubWorkspacePage.tsx` and `CustomModelsPage.tsx`.
- Added capture-phase `scroll` repositioning to `useContextMenuPlacement.ts`.

### 6.7 CI1 / UI1 - CI Gates and UI Copy (`#322`)

- Updated `.github/workflows/race.yml` and `.github/workflows/release.yml` from Go `1.24.x` / `1.23.x` to `1.26.x`, matching `go.mod`.
- Scoped `contents: write` to `publish-release` only. Removed silent failure suppression (`|| true` and `if-no-files-found: warn`) from `release.yml`.
- Removed remaining tutorial microcopy from `AppEnhancementsPage`, `ExtensionsPage`, `ACPAgentMeshPage`, `RemoteControlPage`, and `WhatsAppBridgePage`.
