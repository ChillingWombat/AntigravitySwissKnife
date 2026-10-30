# Antigravity Swiss Knife Code Review Handoff

Date: 2026-10-30

Repository: `ChillingWombat/AntigravitySwissKnife`

Tracker: [#368 Code review hardening roadmap](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/368)

Review baseline: `main` at `1d3ea71` plus the uncommitted A2A, custom-model, bridge, token-monitor, Electron, and frontend work present in the shared checkout.

## Current State

- The shared checkout is dirty on `main`. It contains 65 tracked file modifications, staged changes, and multiple untracked source and documentation files.
- `pkg/webgui/dist/index.html` references `assets/index-id7AbEuV.js`, which is untracked.
- Older generated bundles remain staged or unstaged for deletion.
- The new proxy lifecycle and A2A work exists only in the dirty working tree, not at `HEAD`.
- No production code was changed during review. Compile-only checks passed for the touched Go packages and the frontend TypeScript project.

Do not reset, clean, stash, or commit the shared checkout wholesale. Preserve the dirty tree before starting isolated work.

## Review Outcome

No Critical issue was proven. The review identified:

- 3 P0 issues that block merging or packaging the current dirty tree.
- 5 P1 architecture, security, performance, and retention issues.
- 2 P2 frontend and quality-modernization issues.

The Argon review was directionally correct on the major monolith, polling, cache, frontend-state, and workspace-bloat problems. It missed three defects in the uncommitted proxy lifecycle: destructive shell RC cleanup, concurrent transcript rewrites, and unauthenticated restoration with hardcoded success.

## Issue Graph

| Issue | Priority | Scope | Primary write area |
| --- | --- | --- | --- |
| [#358](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/358) | P0 | Make shell RC cleanup marker-scoped and non-destructive | `pkg/custommodels` |
| [#359](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/359) | P0 | Stop transcript rewrites from corrupting live logs | `pkg/custommodels` |
| [#360](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/360) | P0 | Authenticate native restoration and propagate failures | `electron/`, `pkg/webgui` |
| [#361](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/361) | P1 | Scope the daemon bearer token away from injected renderers | `electron/`, injected scripts, bridge |
| [#362](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/362) | P1 | Use one backend state owner for `daemon --web` | `cmd/swiss`, `pkg/daemon`, `pkg/webgui` |
| [#363](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/363) | P1 | Cache status scans and consolidate frontend polling | `pkg/daemon`, `pkg/webgui`, `frontend/src` |
| [#364](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/364) | P1 | Bound token caches and A2A task retention | `pkg/webgui/token_monitor.go`, `pkg/bridge/a2a.go` |
| [#365](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/365) | P1 | Bundle typed injected scripts and share one DOM scheduler | injected-script packages and build config |
| [#366](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/366) | P2 | Add typed navigation, resilient loading, and accessible dialogs | `frontend/src` |
| [#367](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/367) | P2 | Replace source-grep tests and normalize generated assets | frontend tests, CI, `pkg/webgui/dist` |

## Execution Order

1. Land `#358`, `#359`, and `#360` before any release or merge.
2. Decide and implement the authorization model in `#361`.
3. Complete state ownership in `#362`, then remove redundant same-process fallbacks.
4. Complete `#363`, `#364`, and `#365` as independent P1 slices.
5. Complete `#366` and `#367`.
6. Run one clean release verification and rebuild only after source changes are complete.

`#361` should precede the injected-runtime rewrite in `#365` because the bundle's bootstrap and credential model depend on the token-boundary decision.

`#367` must be last because it owns the canonical `pkg/webgui/dist` output.

## Preserve The Baseline

Before creating worker worktrees, export the complete dirty-tree state without changing the shared checkout:

```bash
cd "/mnt/Data/Projects/Antigravity Swiss Knife"
mkdir -p /tmp/swiss-review-baseline
git diff --binary HEAD > /tmp/swiss-review-baseline/tracked.patch
git ls-files --others --exclude-standard -z \
  | tar --null -T - -cf /tmp/swiss-review-baseline/untracked.tar
git status --short > /tmp/swiss-review-baseline/status.txt
```

Create the isolated worktree, apply the tracked patch, then extract the untracked files:

```bash
git fetch origin --prune
git worktree add -b feature/code-review-hardening \
  /tmp/swiss-review-hardening origin/main
git -C /tmp/swiss-review-hardening apply --3way \
  /tmp/swiss-review-baseline/tracked.patch
tar -C /tmp/swiss-review-hardening \
  -xf /tmp/swiss-review-baseline/untracked.tar
```

Verify the result before dispatching workers:

```bash
git -C /tmp/swiss-review-hardening status --short
go test ./pkg/custommodels ./pkg/webgui ./pkg/bridge -run '^$'
(cd /tmp/swiss-review-hardening/frontend && \
  npx tsc --noEmit -p tsconfig.app.json)
```

If patch application conflicts, stop and report the conflicting paths. Do not discard either side.

## Per-Issue Protocol

Each issue is a vertical slice:

- Add or update the failing behavioral test first.
- Prove the test fails for the expected reason before implementation.
- Make the smallest root-cause fix.
- Run only the targeted package tests and required compile checks.
- Run an independent review with the issue acceptance criteria and raw diff.
- Open a PR only after the issue-specific verification is green.

P0 branches must be based on the preserved hardening branch, not a clean `origin/main` checkout that lacks the reviewed changes.

Use disjoint worktrees for parallel issues. Do not let two workers edit the same files.

## Verification Matrix

| Issue | Required evidence |
| --- | --- |
| `#358` | Shell fixtures prove unrelated exports survive; legacy block migration is idempotent; file mode and atomic-write behavior are verified. |
| `#359` | Concurrent host-writer stress test proves no appended transcript lines are lost; proxy behavior remains available through the chosen sidecar or API. |
| `#360` | Tests cover valid token, stale token, 401 retry, timeout, non-2xx, malformed response, sync failure, and restart failure; one live exit test proves native mode. |
| `#361` | Injected code has no long-lived bearer token; privileged flows use scoped capabilities; threat-model tests cover an untrusted renderer. |
| `#362` | HTTP and RPC observe the same account, quota, custom-model, and enhancement state; constructor failures stop startup. |
| `#363` | Cache hit/TTL/invalidation tests pass; an idle trace shows no per-request process scans and no duplicate fleet/account fetch. |
| `#364` | Eviction tests cover missing files, capacity pressure, TTL, active streams, and retained task diagnostics. |
| `#365` | DOM harness proves one scheduler, bounded timers, hidden-document behavior, and module failure isolation; generated bundle is typechecked and linted. |
| `#366` | Reload/Back/Forward restore route and tab; partial failures retain good data; dialogs pass focus, Escape, label, and keyboard checks. |
| `#367` | Migrated tests assert rendered behavior; clean build produces a self-consistent asset set; every referenced asset exists and is tracked. |

## Targeted Commands

Compile checks already passed on the review baseline:

```bash
go test ./pkg/custommodels ./pkg/webgui ./pkg/bridge -run '^$'
(cd frontend && npx tsc --noEmit -p tsconfig.app.json)
```

Ticket work should add the issue-specific tests before using broader checks.

Final hardening verification, run only after all source tickets are merged:

```bash
go vet ./...
go test ./pkg/...
npm test --prefix frontend
npm test
npm run build
```

The final app verification must launch the rebuilt daemon and desktop shell, confirm `/healthz`, exercise account switching, enter and leave proxy mode, and prove the native endpoint is restored after exit.

## Argon Cross-Check

Confirmed:

- `pkg/webgui/server.go` and `pkg/webgui/token_monitor.go` total 10,356 lines.
- `pkg/webgui/server.go` contains 205 `HandleFunc` registrations plus three `Handle` registrations.
- `CustomModelsPage.tsx` contains 74 `useState` calls.
- The injected runtime contains five active whole-document observer sites and fourteen `setInterval` call sites.
- The frontend fetches fleet and account data redundantly every ten seconds.
- Token caches and the new A2A task manager lack eviction.
- `scratch/`, `release/`, and `bin/` account for approximately 10.9 GB.

Corrected or clarified:

- The source tree contains 222,948 lines including tests and 137,188 non-test lines, depending on measurement scope.
- Thirty of 71 frontend test files read implementation source; the report's "7 of 19" baseline is obsolete.
- `webgui.NewServer` has four `NewStore("")` calls and `daemon.NewDaemon` has three, plus separate vault and revival instances.
- The generated asset hash in the report is stale; the current HTML references untracked `index-id7AbEuV.js`.
- `validateJailedPath` intentionally permits unrestricted absolute-path reads. It is a high-risk capability boundary, not a traversal defect.

## Working-Tree Constraints

- Preserve all existing user and agent changes.
- Never use `git reset --hard`, `git checkout --`, or a broad clean command.
- Do not modify the shared checkout to implement multiple issues.
- Do not kill unrelated Antigravity, Electron, or daemon processes.
- Do not store credentials or token values in issues, documentation, logs, or test fixtures.
- Do not package the current generated bundle until `#367` completes.

## First Actions

1. Read [#368](https://github.com/ChillingWombat/AntigravitySwissKnife/issues/368) and the three P0 issues.
2. Export and preserve the dirty tree using the commands above.
3. Create isolated worktrees for `#358`, `#359`, and `#360`.
4. Write the failing regression tests before touching production code.
5. Update the tracker checklist as each issue passes review and merges.

