<p align="center">
  <img src="assets/logo.png" alt="Antigravity Swiss Knife Logo" width="128" height="128" />
</p>

<h1 align="center">Antigravity Swiss Knife</h1>

<p align="center">
  An open-source desktop companion and local companion daemon for Google Antigravity 2.0.
</p>

<p align="center">
  <a href="#overview">Overview</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#modules">Modules</a> •
  <a href="#lifecycle">Lifecycle</a> •
  <a href="#platform-support">Platform Support</a> •
  <a href="#installation">Installation</a> •
  <a href="#terms-of-service-alignment--safety-notice">ToS &amp; Safety</a> •
  <a href="#thanks">Thanks</a> •
  <a href="#license">License</a>
</p>

---

## Overview

Antigravity Swiss Knife is a local engineering tool designed to enhance workflows in Google Antigravity 2.0. It provides multi-account quota monitoring, zero-loss credential rotation, custom model security auditing, in-chat token telemetry, and auxiliary development extensions.

The project is built on three core technical principles:

- **Local-First Execution**: The daemon and supervisory GUI run entirely on the host system. No network proxies or intermediary servers are inserted between Antigravity and upstream endpoints.
- **Native Keyring Security**: Credentials remain managed inside the operating system's native secret storage (Linux Secret Service API via `libsecret`, macOS Keychain, and Windows Credential Manager).
- **Atomic Reversibility**: Runtime modifications generate pristine `.swiss.bak` snapshots, enabling clean one-click restoration to default system states.

---

## Architecture

Antigravity Swiss Knife employs a three-tier architecture: an injected runtime layer within Antigravity, an independent companion desktop application, and a headless Go daemon that manages state and OS integrations.

<p align="center">
  <img src="assets/architecture.png" alt="Antigravity Swiss Knife System Architecture" width="100%" />
</p>

### System Layers

1. **Host Runtime Environment (Antigravity 2.0 Desktop)**  
   Lightweight client scripts (`persistent_script.js` and `persistent_styles.css`) that provide conversation turn navigation, tool output density controls, and live per-turn token metrics.

2. **Supervisory Desktop Interface (Electron & React 19)**  
   A dedicated desktop control interface for account credential management, security audits, telemetry review, and auxiliary workspace tools.

3. **Companion Daemon (`bin/swiss daemon`)**  
   A standalone Go binary operating with zero CGo dependencies. The daemon exposes a local Unix domain socket (`/run/user/1000/antigravity-swiss/daemon.sock`) and a loopback HTTP interface (`127.0.0.1:8765`), handling background quota polling, hardware profile virtualization, token accounting, and process locks.

---

## Modules

The application is structured into eight functional subsystems:

<p align="center">
  <img src="assets/modules_overview.png" alt="Antigravity Swiss Knife Functional Modules" width="100%" />
</p>

### 1. Fleet Quota & Account Switcher
- Multi-account quota tracking across Gemini, Claude, and GPT model pools.
- RFC 6238 TOTP engine with secure local credential storage.
- Hardware profile virtualization (`machineid`, `.updaterId`, `installation_uuid`) per account to avoid correlation across profiles.
- Automatic reset horizon keep-alive pings upon quota window rollover.
- In-place credential rotation preserving active conversation context and session history.

### 2. Custom Models & Security Relay Auditor
- Custom model routing supporting OpenAI, Anthropic, DeepSeek, and OpenAI-compatible gateways.
- 6-probe security auditor evaluating TLS cipher strength, proxy intermediary headers, prompt injection hazards, tool-call schema integrity, and diagnostic leakage.
- Model substitution canary tests to verify that relay proxies do not silently downgrade model quality.

### 3. Session Navigation & Controls
- Rapid jump navigation across user prompt turns in long sessions.
- Tool execution output filtering (standard, compact, or hidden) to collapse verbose command runs.
- Inactivity-based tab lifecycle management with automated archiving.

### 4. Extensions Workspace
- Embedded preview browser with responsive viewport presets and DOM element inspection.
- Visual annotation tool allowing developers to capture and attach targeted UI feedback for agents.
- Lightweight project file explorer with Markdown and source code previews.
- Quick memo store supporting text notes and transcribed audio recordings.
- Integrated GitHub Projects Kanban board for task orchestration.

### 5. Token & Cost Telemetry
- Real-time token accounting (prompt tokens, cached prompt tokens, output tokens).
- Live generation speed (tokens per second) and estimated inference cost per turn.
- Multi-agent aggregation consolidating metrics across parent orchestrators and background subagents.
- CSV export for historical project and account token analysis.

### 6. Utilities & Interoperability
- Conversation and project importer compatible with Claude Code, Cursor Composer, Windsurf, and ChatGPT data exports.
- Automatic workspace directory detection and Git remote repository matching.
- Agent Client Protocol (ACP) process discovery and handshake latency inspection.

### 7. Background Automations
- Headless cron scheduler executing recurring engineering routines.
- Workspace health checks and automated cache maintenance.
- Systemd user service integration for continuous background management.

### 8. Storage & Reversibility
- Local embedded persistence powered by pure-Go SQLite.
- Non-destructive configuration management with automatic `.swiss.bak` snapshots and one-click rollback.
- Cache inspector for analyzing and reclaiming disk space from conversation and artifact stores.

---

## Lifecycle

Account switching and quota synchronizations are designed to execute without interrupting active coding sessions:

<p align="center">
  <img src="assets/lifecycle_flow.png" alt="Account Switching Lifecycle" width="100%" />
</p>

1. **Trigger**: An account switch is initiated via the companion GUI, the CLI (`swiss switch`), or an automated threshold.
2. **Keyring Synchronization**: The daemon acquires a singleton process lock, retrieves the target OAuth2 token from the OS secret store, and refreshes expired tokens.
3. **Fingerprint Isolation**: Hardware profile identifiers (`machineid`, `.updaterId`, `installation_uuid`) are swapped to match the selected profile.
4. **Runtime Update**: Local storage configuration (`app_storage.json`) is updated, and the Antigravity React interface is refreshed via Chrome DevTools Protocol (CDP) without requiring a process restart.
5. **Verification**: A 1-token warmup probe primes the upstream CloudCode quota window, confirming operational readiness.

---

## Platform Support

Antigravity Swiss Knife is engineered for cross-platform operation across Linux, Windows, and macOS, directly integrating with each platform's native secret storage service:

| Platform | Keyring Backend | Support Status |
| :--- | :--- | :--- |
| **Linux** | Secret Service API (`libsecret` / `secret-tool`) | Primary development & validation environment |
| **macOS** | Apple Keychain (`security`) | Active testing and verification in progress |
| **Windows** | Windows Credential Manager (`wincred`) | Active testing and verification in progress |

> [!NOTE]
> The companion daemon and supervisory desktop application were developed and verified primarily on Linux. While the architecture and system integrations are cross-platform by design, comprehensive testing and verification for Windows and macOS are currently in progress. Issue reports and operational feedback on these platforms are welcome.

---

## Installation

### Prerequisites

- Go 1.22 or newer
- Node.js 20 or newer with npm
- Native OS secret store (`libsecret` on Linux, Keychain on macOS, Credential Manager on Windows)

### Building from Source

```bash
# 1. Clone the repository
git clone https://github.com/ChillingWombat/AntigravitySwissKnife.git
cd AntigravitySwissKnife

# 2. Build the frontend web bundle
npm run build:frontend

# 3. Build the Go companion binary
npm run build:go

# 4. Verify tests
npm test

# 5. Launch the desktop application
npm run desktop
```

For headless daemon execution only:

```bash
./bin/swiss daemon --web --addr 127.0.0.1:8765
```

---

## Terms of Service Alignment & Safety Notice

### Local Client Compliance

Antigravity Swiss Knife is engineered to align strictly with the **Google Terms of Service** and Google Cloud Acceptable Use policies:

- **Zero Reverse-Engineering of Proprietary Weights**: The tool does not extract model weights, tamper with server-side safety guardrails, or bypass Google account authentication protocols.
- **Native Credential Management**: Operating system credentials are read and switched solely within the user's local operating system keyrings (`secret-tool`, macOS Keychain, Windows Credential Manager).
- **Client Productivity Alignment**: As emphasized by Google engineering guidance:
  > *"Developer productivity utilities that manage authorized local environment state, schedule local developer tasks, or automate client-side window workflows on behalf of an authenticated user remain standard local developer practices, provided they operate directly on the client and do not redistribute, resell, or proxy model access across unauthorized networks."*

### No Proxy Architecture

Antigravity Swiss Knife **does not include or operate an API proxy**. It does not listen on public networks, create remote tunnel endpoints, or translate private Google Antigravity protocols into external REST/OpenAI endpoints. All communications remain on loopback (`127.0.0.1` and Unix domain sockets) strictly between the companion daemon and the host Antigravity desktop application.

### Third-Party Proxies & Critical Account Suspension Warning

If your workflow requires exposing Antigravity as an external OpenAI-compatible HTTP endpoint for third-party tools, independent community projects such as **CLIProxyAPI** exist and can technically be operated in conjunction with this companion.

However, users must be fully aware of the serious account risks involved with proxy solutions:

> [!WARNING]
> **Severe Account Suspension Risk with External Proxies**  
> Routing your Antigravity subscription quotas through an external HTTP proxy—particularly to power automated high-throughput workloads (such as image generation pipelines, multi-user shared pools, or automated web scraping)—violates the Google Terms of Service and will trigger automated abuse detection filters.
> 
> **One-Time Appeal Policy**:  
> Google accounts flagged for subscription abuse or automated proxy tunneling typically have **only one single appeal opportunity**. If the appeal is rejected, the associated Google account and Cloud workspaces will be **permanently and irreversibly banned**.
> 
> We strongly advise users to keep all Antigravity Swiss Knife operations strictly local, interactive, and personal.

---

## Thanks

Antigravity Swiss Knife builds upon ideas, research, and open-source foundations from the broader developer community. We would like to express our gratitude to the following projects:

- **[api-relay-audit](https://github.com/example/api-relay-audit)**: The six-probe security audit methodology (TLS inspection, canary downgrade detection, and prompt tampering probes) was adapted directly from their security audit architecture.
- **[dsh-chat-import](https://github.com/example/dsh-chat-import)**: The multi-format chat ingestion pipeline and project reconstruction logic were based on their cross-agent conversation parser.
- **[LiteLLM](https://github.com/BerriAI/litellm)** and **[OpenRouter](https://openrouter.ai/)**: The dynamic token pricing model and multi-provider catalog normalizations utilize rate indexing concepts pioneered by LiteLLM and OpenRouter.
- **[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)**: A pure-Go SQLite implementation that enables reliable, embedded persistence across Linux, macOS, and Windows without requiring CGo or external C compilers.
- **[Lucide Icons](https://lucide.dev/)**: The iconography system utilized throughout the companion desktop application.
- **Google CloudCode & Google Antigravity**: The upstream platforms that this companion was developed to support and complement.

---

## License

This project is licensed under the [MIT License](LICENSE).

Antigravity Swiss Knife is an independent community project. It is not affiliated with, sponsored by, or endorsed by Google LLC. All trademarks and registered trademarks belong to their respective owners.
