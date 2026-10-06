# Project Plan: 1152×648 16:9 Window Geometry & Golden Ratio Layout Architecture

## Objective
Establish a fixed minimal non-maximized window size of 1152×648 px (strict 16:9 aspect ratio, 4-pixel aligned) for Antigravity Swiss Knife, and design layout zones, sections, and gadgets adhering as close to the golden ratio as possible with 4-pixel increment ceiling rounding for widths.

## Track Structure
### 1. Survey & Architecture Mapping (Phase 0)
- Dispatch 3 parallel Explorers to investigate current `electron/main.js`, `frontend/src/` layouts, styling system, components/gadgets, existing test infrastructure, and compile feature requirements.
- Merge Explorer findings into feature inventory and architecture layout in `PROJECT.md`.

### 2. Implementation Track
- **Milestone 1: Minimal Window Geometry & 16:9 Aspect Ratio Locking**
  - Scope: `electron/main.js`, window options (minWidth: 1152, minHeight: 648, width: 1152, height: 648), `mainWindow.setAspectRatio(16 / 9)`.
  - Gate: Explorer -> Worker -> 2 Reviewers -> 2 Challengers -> Forensic Auditor.
- **Milestone 2: Golden Ratio Layout Architecture & 4-Pixel Grid Alignment**
  - Scope: Layout token system in `frontend/src` (4px grid constants, golden ratio helpers, ceiling 4-increment step rule `W_major = ceil(W / phi)_4`), NavRail 220px (55×4), Header 72px (18×4), main content workspace 932×576 px (ratio 1.61806).
  - Gate: Explorer -> Worker -> 2 Reviewers -> 2 Challengers -> Forensic Auditor.
- **Milestone 3: Component & Gadget Sizing Compliance**
  - Scope: Cards, modals, gauges, interactive gadgets conforming to 4px spacing and golden-ratio derived dimensions; responsive behavior without horizontal overflow or clipping at 1152×648.
  - Gate: Explorer -> Worker -> 2 Reviewers -> 2 Challengers -> Forensic Auditor.
- **Milestone 4: Automated Verification Suite & Adversarial Hardening**
  - Scope: Programmatic test suite asserting 4px divisibility, width ceiling vs floor rounding proofs, TypeScript check, Vite build. Adversarial test suite with Challengers.
  - Gate: Explorer -> Worker -> 2 Reviewers -> 2 Challengers -> Forensic Auditor.

### 3. E2E Testing Track (Parallel)
- Requirement-driven opaque-box test runner validating window constraints and layout mathematics.
