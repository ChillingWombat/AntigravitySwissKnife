# Master Plan — Antigravity Swiss Knife

## Objective
Build, verify, and deliver Antigravity Swiss Knife — a native standalone desktop multi-tool and companion daemon for Google Antigravity 2.0 fulfilling R1 to R5 with strict Google Gemini Material 3 dark styling and 100% native Linux integration.

## Phase 0: Survey & Scope Mapping
- Dispatch 3 Explorers / Spec Miners in parallel:
  1. Survey local Linux environment: Secret Service / `secret-tool`, `~/.config/Antigravity/` and `~/.gemini/antigravity/` directory structures, `machineid`, `.updaterId`, `installation_id`, `antigravity_state.pbtxt`, `app_storage.json`.
  2. Survey upstream Quota API specifications: `cloudcode-pa.googleapis.com` endpoints (`fetchAvailableModels`, `retrieveUserQuotaSummary`), token handling, payload structures, 1-token keep-alive ping mechanics.
  3. Survey UI architecture & tech stack options: Material Design 3 dark theme implementation (#131314 surface, #1e1f20 card, #8ab4f8 accent), fixed left panel, top ribbon, TOTP RFC 6238 engine with countdown rings, desktop packaging.
- Merge findings into `PROJECT.md` Feature Inventory and Interface Contracts.

## Phase 1: Dual Track Launch
- **E2E Testing Track**: Dispatches dedicated test architect / test writers to build opaque-box test infrastructure and Tiers 1-4 test cases covering all inventoried features. Publishes `TEST_READY.md`.
- **Implementation Track**: Executes sequential modular milestones with strict quality gates (Explorer -> Worker -> Reviewers -> Challengers -> Forensic Auditor).

## Phase 2: Implementation Milestones
- **M1: Core Keyring Switcher & Process Lifecycle Engine (R1)**:
  - Secret Service wrapper (`secret-tool` / libsecret `service=gemini`, `username=antigravity`).
  - Session preservation (`app_storage.json` layout, `cascadeId`).
  - Clean process termination and relaunch.
- **M2: Upstream Quota Poller & Reset Horizon Warmup (R3)**:
  - OAuth token integration.
  - Periodic polling of `fetchAvailableModels` & `retrieveUserQuotaSummary`.
  - Fraction & reset countdown tracking.
  - Automated 1-token keep-alive warmup at resetTime.
- **M3: Per-Account Device Fingerprint Virtualizer (R4)**:
  - Generation and isolation of `machineid`, `.updaterId`, `installation_id`, and `installation_uuid` in `antigravity_state.pbtxt`.
  - Atomic profile swapping alongside keyring switches.
- **M4: Brain & Context Cache Optimizer (R5)**:
  - Disk inspection for `~/.gemini/antigravity/brain/` and `~/.gemini/antigravity/conversations/`.
  - Safe cleanup routines for stale tasks and transcripts without affecting active sessions.
  - Prompt token reduction / optimization utilities.
- **M5: Gemini M3 Desktop Application & Multi-Page GUI (R2)**:
  - Google Gemini Material 3 Dark theme (#131314, #1e1f20, #8ab4f8).
  - Fixed left panel navigation rail (Account Switcher, Tools Marketplace, Settings).
  - Account Switcher top ribbon with 5 tabs: Quota Dashboard, Accounts & MFA Vault (RFC 6238 TOTP engine with visual countdown rings), Device Fingerprints, Brain Cache Manager, Switcher Settings.
  - System tray icon and desktop integration.

## Phase 3: Final Acceptance & Hardening
- **Phase 1 of Final Milestone**: Run 100% of E2E test suite (Tiers 1-4) across all modules.
- **Phase 2 of Final Milestone**: Adversarial coverage hardening (Tier 5) with Challengers and Forensic Auditor until clean verdict and complete test passage.
