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
- Enables direct subagent continuation and context injection without restarting the entire desktop application when hot-swapping credentials.

---

## 5. Remote Control & Tunneling Architecture

Antigravity Swiss Knife provides an out-of-home remote control wrapper (`pkg/bridge/remote.go` and `pkg/bridge/tunnel.go`) that exposes Antigravity's interface across local networks, private Tailnets, and public HTTPS tunnels with persistent session re-binding across Google account rotations.

### 5.1 Remote Control Reverse Proxy (`127.0.0.1:8766` & `/remote`)
- **Loopback Service Binding**: The daemon starts a dedicated HTTP and WebSocket reverse proxy listener on `127.0.0.1:8766`.
- **Route Handling**:
  - `/remote/api/auth`: Validates the Security PIN and issues a session cookie (`swiss_remote_pin`).
  - `/remote/api/status`: Returns current CDP connection status, active Google account, connected clients, and tunnel diagnostics.
  - `/remote/api/screenshot`: Captures live viewport screenshots from Antigravity via CDP.
  - `/remote/api/conversation/*`: Dispatches new tasks or appends instructions to active sessions.
  - `/remote` & `/`: Renders the mobile-responsive entrance and console interface, or redirects authenticated clients to the active web app path.
- **Antigravity Web App Proxying**: When Antigravity Desktop's built-in `language_server` web application is running, authenticated incoming requests are reverse-proxied directly to the upstream IDE web origin, preserving streaming Connect-RPC calls, WebSockets, and terminal channels.
- **Live Account Synchronization**: The proxy injects a background script (`#swiss-remote-account-sync`) into the HTML response body before `</body>`. This client script queries Redux stores and `agentServiceResolver` every 2.5 seconds to reconcile active user email and name states when account switches occur on the host machine.

### 5.2 Dynamic CDP `language_server` Origin & CSRF Discovery
- **Dynamic Origin Resolution**: Antigravity's internal language server binds to an ephemeral loopback port using either HTTP or self-signed HTTPS. The daemon queries Chromium DevTools targets (`injector.FindDevToolsPort()` and `injector.GetPageTargets()`) via CDP to discover the active loopback origin (e.g., `https://127.0.0.1:<port>`) and current URL path.
- **CSRF Token Extraction**: Upstream RPC requests require a valid CSRF token header (`x-codeium-csrf-token`). The daemon fetches the root HTML of the detected language server origin and extracts the active token using regular expressions (`"csrfToken"\s*:\s*"([^"]+)"`).
- **Header Rewriting**: The reverse proxy `Director` updates the `Origin` and `Referer` headers to match the upstream language server origin and sets `x-codeium-csrf-token` on outgoing requests. Downstream response headers that would prevent framing or remote display (`Content-Security-Policy`, `Content-Security-Policy-Report-Only`, `Cross-Origin-Opener-Policy`, `X-Frame-Options`) are stripped automatically.

### 5.3 4-Minute Tunnel Keep-Alive Probes & Health Invalidation
- **Periodic Probing**: `TunnelManager` runs a background keep-alive loop with a 4-minute cadence (`DefaultKeepAliveInterval = 4 * time.Minute`).
- **Idle Timeout Prevention**: Probes dispatch lightweight HTTP GET requests through the public gateway URL with custom headers:
  ```http
  User-Agent: Antigravity-Swiss-Knife-KeepAlive/1.0
  X-Swiss-KeepAlive: 1
  ```
  This traffic prevents Cloudflare Quick Tunnel (TryCloudflare) sessions from terminating due to Cloudflare's 5-minute idle expiration window.
- **Health Invalidation & Auto-Stop**: If a keep-alive probe returns gateway or upstream error codes (HTTP 502, 503, 504, 521, 522, 523, 524, 530) or fails network dispatch, `invalidateTunnel` immediately kills the underlying tunnel process, clears public endpoint URLs from memory and configuration, transitions status to inactive, and triggers the `onAutoStop` handler to shut down the proxy listener.

### 5.4 Tailscale Funnel vs. Private Tailnet TCP Forwarder
The system supports two distinct network topologies when using Tailscale:
- **Private Tailnet TCP Forwarder**:
  - The daemon detects the local Tailscale binary (`LocateTailscale`), runs `tailscale ip -4`, and parses `tailscale status --json` for MagicDNS names.
  - Binds a dedicated TCP forwarder listener to the node's Tailscale IP address (`<tailscale-ip>:<forwardPort> -> 127.0.0.1:<localPort>`).
  - Allows authenticated devices on the operator's private Tailnet (such as mobile phones and remote laptops) to access the console via MagicDNS (`http://<magicdns>:<port>/remote`) or Tailscale IP (`http://<tailscale-ip>:<port>/remote`) with zero public internet exposure.
- **Tailscale Funnel**:
  - When operator permissions and node configurations permit, the daemon initiates background publishing via `tailscale funnel --bg` or `tailscale serve --bg`.
  - Routes inbound traffic through Tailscale's edge infrastructure to provide an encrypted public HTTPS endpoint (`https://<magicdns>/remote`).
  - If Funnel activation fails or node permissions are restricted, the manager falls back to the private Tailnet TCP forwarder.

### 5.5 Security PIN & App Access Password Verification
Two distinct cryptographic barriers guard remote and local operations:
- **Remote Control Security PIN**:
  - Remote HTTP and WebSocket endpoints verify incoming requests against the daemon's configured Security PIN (supporting 6-digit to 8-character verification tokens).
  - Clients can supply the PIN via the `X-Remote-PIN` header, `Authorization: Bearer <pin>`, the `?pin=` query parameter, or the `swiss_remote_pin` HTTP-only cookie.
  - Failed authentication attempts return `401 Unauthorized`.
  - Supports instant on-demand PIN rotation: generating a fresh PIN immediately clears all active client sessions and revokes existing authentication cookies.
- **App Access Password**:
  - Protects the desktop application GUI and prevents unauthorized local access on shared workstations.
  - Implements RFC 2898 PBKDF2-HMAC-SHA256 with 100,000 iterations and a cryptographically random 16-byte salt (`pbkdf2:sha256:100000:<salt>:<hash>`).
  - Enforces a minimum length of 6 characters and gates all sensitive UI surfaces, including account switching, keyring token exports, and configuration editing, behind a master lock screen.
