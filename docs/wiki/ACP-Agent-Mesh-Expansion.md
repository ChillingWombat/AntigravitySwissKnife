# ACP Agent Mesh Expansion

The Agent Client Protocol (ACP) provides a standardized, local-first communication layer enabling AI coding agents to discover each other, negotiate capabilities, and delegate execution tasks on the developer's workstation.

Antigravity Swiss Knife acts as a local coordinator and telemetry observer for all registered ACP agent nodes.

---

## 1. Agent Client Protocol (ACP) Overview

ACP operates over local Unix domain sockets, named pipes, and loopback TCP ports on the developer's machine:
- **Zero Cloud Dependence**: Agent discovery and message routing occur strictly on `localhost`.
- **Lightweight JSON-RPC 2.0 Framing**: All requests, responses, and events use structured JSON payloads with standard `id`, `method`, and `params` fields.
- **Dynamic Tool Registration**: Agents expose their tool surfaces (e.g. bash runners, file editors, AST parsers) to peer agents in the mesh.
- **Latency Telemetry**: The daemon periodically samples peer responsiveness to detect hung agent runtimes before dispatching tasks.

---

## 2. The 9 Registered ACP Agent Nodes

The mesh registry (`pkg/webgui/server.go` and `frontend/src/utils/acpPresentation.ts`) tracks nine distinct agent environments:

| Node ID | Agent Name | Node Type | Primary Interface | Protocol Role |
| :--- | :--- | :--- | :--- | :--- |
| `agent-antigravity` | Google Antigravity 2.0 | GUI Editor | Desktop IDE Window | Primary Orchestrator & Chat Agent |
| `agent-antigravity-cli` | Antigravity CLI (`agy`) | CLI Daemon | Terminal Process | Headless Agent Execution & CI Runner |
| `agent-devin` | Devin (formerly Windsurf Cascade) | GUI Editor | Desktop IDE Window | Autonomous Workspace Engineering |
| `agent-opencode` | OpenCode | GUI Sidecar | Electron Webview | Open Model Sidecar & Extension |
| `agent-deepseek-harness` | DeepSeek Harness | CLI Process | Terminal TTY | Terminal Reasoning Engine |
| `agent-pi` | Pi | CLI Daemon | Local Service Port | Autonomous Math & Logic Specialist |
| `agent-codex` | Codex | CLI Process | Shell Subprocess | Scripting & Tool Automation CLI |
| `agent-claude-code` | Claude Code CLI | CLI Process | Terminal TTY | Terminal Codebase Refactoring Agent |
| `agent-cursor` | Cursor Editor Agent | GUI Editor | VS Code Fork | Inline Code Completion & Diff Agent |

---

## 3. GUI vs CLI Node Separation

The frontend dashboard (`UtilitiesPage.tsx`) cleanly categorizes nodes to reflect their operational properties:

### 3.1 GUI Editor Nodes
- **Characteristics**: Render graphical user interfaces, maintain open project windows, and interact via Electron webviews or native Chromium instances.
- **Visual Design**: Presented with distinct card badges, window state indicators (`focused`, `minimized`, `background`), and CDP attachment status.
- **Nodes**: Antigravity 2.0, Devin, OpenCode, Cursor.

### 3.2 CLI / Terminal Nodes
- **Characteristics**: Execute headless in terminal emulators or background daemon pools, with no UI rendering overhead.
- **Visual Design**: Presented with monospaced PID badges, active TTY indicators, memory consumption metrics, and stdout/stderr stream health.
- **Nodes**: Antigravity CLI (`agy`), DeepSeek Harness, Pi, Codex, Claude Code.

---

## 4. Handshake Protocol & Capability Exchange

When a new agent node starts or joins the mesh, it initiates a standard four-way handshake:

```
Agent Node                                     Antigravity Swiss Knife
    |                                                     |
    | ------------ 1. ACP_HELLO (node metadata) ---------> |
    |                                                     |
    | <----------- 2. ACP_HELLO_ACK (session ID) -------- |
    |                                                     |
    | ------------ 3. ACP_REGISTER_TOOLS (tool schema) --> |
    |                                                     |
    | <----------- 4. ACP_MESH_TOPOLOGY (peer list) ------ |
    |                                                     |
    | <=== (Periodic 15s Heartbeat Ping / Pong) ========> |
```

### 4.1 Step 1: `ACP_HELLO`
The connecting node broadcasts its identifier, version, and socket address:
```json
{
  "jsonrpc": "2.0",
  "method": "acp.hello",
  "params": {
    "node_id": "agent-claude-code",
    "name": "Claude Code CLI",
    "version": "1.0.4",
    "kind": "cli",
    "listen_address": "127.0.0.1:41920"
  },
  "id": 1
}
```

### 4.2 Step 2: `ACP_HELLO_ACK`
The daemon validates the node identity, assigns a session token, and establishes a ping schedule:
```json
{
  "jsonrpc": "2.0",
  "result": {
    "session_id": "sess_8f29d102",
    "heartbeat_interval_ms": 15000,
    "daemon_version": "0.1.0"
  },
  "id": 1
}
```

### 4.3 Step 3: Tool Capability Registration
The agent node registers available callable functions so other agents or the supervisory interface can invoke them:
```json
{
  "jsonrpc": "2.0",
  "method": "acp.register_tools",
  "params": {
    "session_id": "sess_8f29d102",
    "tools": [
      {
        "name": "run_bash_command",
        "description": "Execute arbitrary shell command in workspace",
        "parameters": {"type": "object", "properties": {"command": {"type": "string"}}}
      }
    ]
  },
  "id": 2
}
```

### 4.4 Step 4: Health Monitoring & Latency Inspection
The daemon samples latency every 15 seconds by measuring round-trip time for `acp.ping`. Latencies are displayed in the UI:
- **Healthy**: < 5ms (green badge)
- **Degraded**: 5ms – 50ms (amber badge)
- **Unresponsive**: > 50ms or missed 2 consecutive pings (red badge, marked offline)
