<p align="center">
  <img src="assets/logo.png" alt="Antigravity Swiss Knife Logo" width="128" height="128" />
</p>

<h1 align="center">Antigravity Swiss Knife</h1>

<p align="center">
  Open-source companion for Google Antigravity
</p>

<p align="center">
  <a href="#overview">Overview</a> •
  <a href="#key-features">Key Features</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#modules">Modules</a> •
  <a href="#lifecycle">Lifecycle</a> •
  <a href="#platform-support">Platform Support</a>
  <br />
  <a href="#installation">Installation</a> •
  <a href="#terms-of-service-alignment--safety-notice">ToS &amp; Safety</a> •
  <a href="#feedback--community">Feedback</a> •
  <a href="#thanks">Thanks</a> •
  <a href="#license">License</a>
</p>

---

## Overview

Antigravity Swiss Knife is a local engineering tool designed to enhance workflows in Google Antigravity 2.0. It provides multi-account quota monitoring, zero-loss credential rotation, custom model security auditing, in-chat token telemetry, and auxiliary development extensions. Pre-built standalone binaries are ready to download and run directly from [Releases](https://github.com/ChillingWombat/AntigravitySwissKnife/releases) with zero installation required, while building from source is fully supported for developers building locally.

> [!NOTE]
> **Active Development & Rapid Iteration**: Antigravity Swiss Knife is actively expanding with frequent releases. We iterate rapidly based on developer workflows—please feel free to report bugs, suggest new capabilities, or share feedback on [GitHub Issues](https://github.com/ChillingWombat/AntigravitySwissKnife/issues).

The project is built on three core technical principles:

- **Local-First Execution**: The daemon and supervisory GUI run entirely on the host system. No network proxies or intermediary servers are inserted between Antigravity and upstream endpoints.
- **Native Keyring Security**: Credentials remain managed inside the operating system's native secret storage (Linux Secret Service API via `libsecret`, macOS Keychain, and Windows Credential Manager).
- **Atomic Reversibility**: Runtime modifications generate pristine `.swiss.bak` snapshots, enabling clean one-click restoration to default system states.

---

## Key Features

<p align="center">
  <img src="assets/key_features.png" alt="Antigravity Swiss Knife Key Features" width="100%" />
</p>

---

## Architecture

The system operates across three decoupled tiers running exclusively on the host: an injected runtime layer inside Antigravity 2.0, a React 19 supervisory desktop interface, and a headless pure-Go daemon (`bin/swiss daemon`) communicating over local Unix domain sockets and loopback HTTP.

<p align="center">
  <img src="assets/architecture.png" alt="Antigravity Swiss Knife System Architecture" width="100%" />
</p>

All inter-process communications remain local to the machine, coordinating configuration updates via Chrome DevTools Protocol (CDP) and delegating secret management to operating system keyrings.

---

## Modules

The application is structured into eight functional subsystems:

<p align="center">
  <img src="assets/modules_overview.png" alt="Antigravity Swiss Knife Functional Modules" width="100%" />
</p>

Each subsystem operates as an independent module coordinated through the Go companion daemon and exposed via the supervisory GUI and CLI interfaces. Subsystems share state through the local SQLite store and communicate over the Unix domain socket interface.

---

## Lifecycle

Credential rotation and quota synchronizations execute without interrupting active coding sessions or restarting the Antigravity process:

<p align="center">
  <img src="assets/lifecycle_flow.png" alt="Account Switching Lifecycle" width="100%" />
</p>

The companion daemon acquires a process lock, retrieves the target credentials from the OS keyring, swaps hardware profile fingerprints (`machineid`, `.updaterId`, `installation_uuid`), and hot-reloads the Antigravity interface via Chrome DevTools Protocol (CDP). A 1-token probe verifies upstream quota readiness before returning control to the user.

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

### Pre-built Releases (Ready to Run)

Antigravity Swiss Knife does not require manual compilation or installation. Standalone, ready-to-use binaries are published directly under [Releases](https://github.com/ChillingWombat/AntigravitySwissKnife/releases):

- **Linux**: Standalone AppImage (`.AppImage`) and Debian package (`.deb`)
- **Windows**: Portable executable (`.exe`) and NSIS installer
- **macOS**: Standalone disk image (`.dmg`) and zip archive

Download the asset for your operating system and launch it immediately out of the box.

### Building from Source (Optional)

Developers who prefer to audit, modify, or compile the source code locally can build directly from this repository:

#### Prerequisites

- Go 1.22 or newer
- Node.js 20 or newer with npm
- Native OS secret store (`libsecret` on Linux, Keychain on macOS, Credential Manager on Windows)

#### Build Commands

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

## Production Releases & Multi-Branch Laptop Workflow

Antigravity Swiss Knife provides an automated cross-platform release pipeline that compiles and packages self-contained applications for all three operating systems directly into the gitignored `release/` folder:

```bash
# Package production releases for all 3 platforms
npm run release

# Or package for a specific operating system
npm run release:linux      # release/linux/ (AppImage/deb/unpacked)
npm run release:win        # release/windows/ (installer & unpacked .exe)
npm run release:mac        # release/macos/ (.app bundle & DMG)
```

### Running the Production App Concurrently with Active Agent Branches

A major problem when collaborating with autonomous coding agents is that switching Git branches (`git checkout` / `git switch`) during active development can break or corrupt a running desktop instance.

Because release builds are placed in `release/`, you can launch and run the compiled production application on your laptop **completely decoupled** from source code branches:

```bash
# Launch the production release app locally
npm run app:prod

# Or directly execute the universal launcher
bash release/run.sh
```

While the production app runs in the background or foreground, agents can pull, modify, switch, and test code on Git branches without interrupting your running session.

---

## Codebase Architecture & Clean Directory Layout

To ensure that end users and open-source consumers receive a clean, uncluttered repository, all development-only scratchpads, adversarial tests, and milestone logs are grouped into dedicated gitignored directories:

```
├── cmd/swiss/             # Go daemon CLI entrypoint
├── pkg/                   # Go core packages, handlers, daemons (*_test.go unit tests)
├── frontend/              # React 19 / TypeScript supervisory UI (*.test.ts unit tests)
├── electron/              # Desktop application runner & daemon supervisor
├── assets/                # Visual brand assets, architecture diagrams, theme SVGs
├── scripts/               # Production build, release, and worktree automation scripts
│   ├── build-release.js   # Automated 3-OS compiler and packager
│   └── worktree.sh        # Multi-agent Git worktree manager
├── release/               # [Gitignored] Compiled standalone apps (Linux, Windows, macOS)
│   ├── linux/             # Packaged Linux app & Go binary
│   ├── windows/           # Packaged Windows app & Go binary (.exe)
│   ├── macos/             # Packaged macOS app & Go binaries (arm64 + amd64)
│   ├── run.sh             # Universal OS launcher
│   └── README.md          # Release architecture documentation
└── dev/                   # [Gitignored] Development and testing resources
    ├── scratch/           # Ad-hoc investigation scripts, temporary test scripts
    ├── testing/           # Legacy test harnesses, fixtures, adversarial runners
    ├── milestones/        # Development milestone logs and checklist records
    ├── tools/             # Local profiling dumps and developer helpers
    └── README.md          # Development directory documentation
```

---

## Version Control Strategy for Multi-Agent Collaboration

When multiple agents or developers work on this repository simultaneously, sharing a single working tree causes frequent checkout conflicts and file collisions.

To isolate concurrent development, use **Git Worktrees**:

```bash
# List all active agent worktrees
./scripts/worktree.sh list

# Create a new isolated worktree for a feature branch
./scripts/worktree.sh create feature/my-feature

# Remove a worktree once work is merged
./scripts/worktree.sh remove feature/my-feature
```

Each worktree lives in `.worktrees/` (which is gitignored), has its own working directory, shares the central Git database, and automatically links `node_modules` so tests and builds run without dependency duplication.


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

## Feedback & Community

Antigravity Swiss Knife is under rapid, continuous development. We frequently release updates, improve reliability, and add new capabilities to support evolving developer workflows.

We welcome all community feedback:

- **Bug Reports**: If you experience an unexpected behavior, keyring issue, or platform regression, please open an issue on [GitHub Issues](https://github.com/ChillingWombat/AntigravitySwissKnife/issues).
- **Feature Suggestions**: Have ideas for new workspace modules, integrations, or usability refinements? We actively prioritize community requests.
-

- **Workflow Discussions**: Join the conversation to share how you use Antigravity Swiss Knife and suggest where friction can be eliminated.

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
