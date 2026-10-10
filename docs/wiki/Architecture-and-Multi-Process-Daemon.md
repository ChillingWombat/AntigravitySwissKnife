# Architecture & Multi-Process Daemon

Antigravity Swiss Knife uses a three-tier local architecture to separate runtime hooks, supervisory controls, and background operations. This design isolates heavy backend operations like quota polling, hardware profile virtualization, and file caching from the desktop user interface.

---

## 1. Three-Tier Architectural Overview

The system decomposes into three decoupled tiers:

```
+------------------------------------------------------------------------+
|                     Host Runtime (Antigravity 2.0)                     |
|  - persistent_script.js (DOM controls, TPS telemetry, turn jumps)      |
|  - persistent_styles.css (visual tokens, responsive density)           |
+------------------------------------------------------------------------+
                               | (Chrome DevTools Protocol / CDP)
+------------------------------------------------------------------------+
|             Supervisory Desktop Interface (Electron & React 19)        |
|  - Vite, Tailwind CSS, Lucide icons, Base UI primitives                |
|  - Tray process, window manager, daemon lifecycle supervisor          |
+------------------------------------------------------------------------+
                               | (Unix Domain Socket / Loopback HTTP)
+------------------------------------------------------------------------+
|               Companion Daemon Binary (bin/swiss daemon)               |
|  - Pure Go, compiled natively, zero external runtime dependencies      |
|  - Quota poller, keyring adapter, cache pruner, revival manager        |
+------------------------------------------------------------------------+
                               |
                               v
            Operating System Keyring & SQLite Storage
```

### 1.1 Injected Host Runtime (Tier 1)
The injected runtime layer integrates into the Antigravity 2.0 Electron renderer process:
- `persistent_script.js`: Injected into the Antigravity chat webview. Implements rapid turn-jump navigation, collapsible tool execution output cards, real-time tokens-per-second (TPS) calculation, and subagent state inspection.
- `persistent_styles.css`: Applies clean density tokens and visual overrides without breaking upstream IDE layout semantics.
- Injection is coordinated by the supervisory interface or the daemon via the Chrome DevTools Protocol (CDP) port.

### 1.2 Supervisory Desktop Interface (Tier 2)
The desktop frontend provides system observability and manual management:
- Built with Electron and React 19 using Vite and Tailwind CSS.
- Strictly adheres to the David-Design design system: clean functional surfaces, zero decorative emojis, monochrome Lucide icons, and 1px tokenized borders.
- Communicates with Tier 3 through typed IPC clients, automatically supervising the daemon process lifecycle (`electron/daemon-manager.js`).
- Manages the native system tray menu via `electron/tray-account-menu.js`, displaying active account badges and one-click quota-switching menus.

### 1.3 Companion Daemon (Tier 3)
A standalone pure-Go binary (`bin/swiss daemon`) running headless on the host:
- Compiled natively with Go toolchains, requiring no Python, Node.js, or external runtimes at runtime.
- Executes background operations: periodic quota polling, keyring interaction via native OS APIs, hardware fingerprint isolation, SQLite database maintenance, and cache auto-pruning.
- Exposes a low-latency IPC interface for the desktop interface and CLI utilities.

---

## 2. Inter-Process Communication (IPC)

The daemon coordinates with desktop applications, CLI utilities, and external agent nodes through two local communication channels:

### 2.1 Unix Domain Sockets (Linux & macOS)
- Primary transport on POSIX systems:
  - Standard socket path: `$XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock`
  - Fallback path when `$XDG_RUNTIME_DIR` is unset: `/tmp/antigravity-swiss-$UID/daemon.sock`
- Socket permissions are strictly restricted to the current user (`0600`), preventing local privilege escalation or unauthenticated cross-user reads.

### 2.2 Loopback HTTP REST / SSE Interface
- Standard loopback address: `127.0.0.1:8765`
- Implemented in `pkg/webgui/server.go`.
- Serves REST endpoints for account switching, quota inspection, and system status.
- Uses Server-Sent Events (SSE) on `/api/events` for real-time telemetry streaming (active turn metrics, quota updates, switch notifications).
- Explicitly binds only to `127.0.0.1` and verifies `Host` headers to prevent DNS rebinding attacks.

---

## 3. SQLite Write-Ahead Logging (WAL) Integrity

Antigravity stores IDE settings, workspace metadata, and conversation histories in SQLite databases:
- `state.vscdb`: Primary key-value state store for Antigravity and VS Code forks.
- `conversation_summaries.db`: Conversation index and turn cache.

Direct external writes to active SQLite files risk database corruption if the host IDE is holding database locks. Antigravity Swiss Knife implements WAL-safe access protocols in `pkg/system/storage.go`:

1. **Busy Timeout Configuration**: Opens connections with `PRAGMA busy_timeout = 5000;` to wait gracefully for host IDE transactions to clear.
2. **WAL Journal Mode Enforcement**: Uses `PRAGMA journal_mode = WAL;` to enable concurrent reads without blocking writer threads.
3. **Pristine Snapshotting**: Before executing any schema mutation or key update, the file is duplicated to `<filename>.swiss.bak` with permissions preserved.
4. **Clean Transaction Boundaries**: All atomic key updates commit within explicit transactions (`BEGIN IMMEDIATE ... COMMIT;`).

---

## 4. Chrome DevTools Protocol (CDP) Live Patching

When Antigravity starts with remote debugging enabled (`--remote-debugging-port`), the companion daemon communicates with the Chromium runtime:

- Connects to the local debugging endpoint (e.g. `http://127.0.0.1:<port>/json`).
- Locates the active IDE workbench and agent webview target IDs.
- Injects runtime script modifications into the page context via `Runtime.evaluate`.
- Inspects React Fiber nodes to read conversation identifiers and turn execution states without invasive disk polling.
- Enables seamless subagent continuation and context injection without restarting the entire desktop application when hot-swapping credentials.
