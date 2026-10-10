# Antigravity Swiss Knife — Agent Standing Brief & Engineering Protocol

## Repository Architecture & Technology Stack
- **Companion Daemon**: Headless Go service (`cmd/swiss`, `pkg/...`), loopback HTTP (127.0.0.1:8765), SQLite WAL (`~/.config/antigravity-swiss/swiss.db`), OS keyring integration.
- **Desktop UI**: Electron shell (`main.js`) + React 19 / TypeScript / Vite frontend (`frontend/src/...`), Tailwind CSS, Lucide icons.
- **Host Runtime Integration**: CDP automation, DOM script injection into Google Antigravity 2.0 app (`~/.gemini/antigravity`).

## Global Engineering & Design Invariants

### 1. Zero Artificial Previews & Real Functional Implementation
- **Strict Prohibition**: Never create artificial previews, mocked visual gadgets, or simulated dummy data pretending to have completed work.
- **Genuine Functionality**: Every feature must be backed by real APIs, real state, or actual daemon services.
- **Gadget Rule**: Do not add icons or tags to gadgets unless explicitly requested by David.

### 2. David-Design UI Standards
- **Mandatory Lucide Icons**: Always use **Lucide** for all icons and symbols (`lucide-react`, Lucide SVG). Strictly avoid decorative emojis and miscellaneous icon sets.
- **Zero Decorative Emojis**: Never use decorative emojis in headers, cards, buttons, badges, or gadgets.
- **Layout Invariants**: Single-line action labels (`white-space: nowrap`), flex-wrapping tag containers (`flex-wrap: wrap`), 1px subtle tokenized borders (`border-border`), label-left / switch-right for toggles.

### 3. Git & Remote Synchronization Contract
- Every lifecycle action (branch creation, ticket creation, PR creation, merge) must synchronize immediately with `origin`.
- Zero local-only drift: no unpushed commits on `main`. All promotions via GitHub PRs.

### 4. Code & Documentation Anti-AI Voice
- Reject bloated documentation, buzzwords (`delve`, `leverage`, `robust`), and formulaic `-ing` summaries.
- Follow `David-Humanizer`: direct, concise, technically dense, and human.
- Eliminate paranoid defensive code, redundant null checks, and empty try/catch blocks.

### 5. Autonomous Computer & Browser Use Execution
- When needed, automatically leverage computer use and browser use capabilities (`obscura`, `open-browser-use`, `open-computer-use`) proactively without waiting for user reminders.

