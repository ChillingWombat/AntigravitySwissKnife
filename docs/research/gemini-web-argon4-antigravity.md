# Research Report: Using Gemini Web's Argon 4 Model in Antigravity & A2A/MCP Direct Messaging Feasibility

**Date**: 2026-10-30  
**Context**: `/mnt/Data/Projects/Antigravity Swiss Knife`  
**Referenced Skills**: `mp-research`, `david-wish`  
**Primary Sources Examined**:
- `pkg/bridge/a2a.go` & `pkg/bridge/a2a_protocol.go` (A2A 0.3.0 server implementation)
- `pkg/bridge/bridge.go` & `pkg/bridge/lead.go` (Gemini Bridge & MCP Streamable HTTP Gateway)
- `pkg/custommodels/gcp.go` & `pkg/custommodels/gateway.go` (Vertex AI & Custom Model Gateway)
- `docs/wiki/Remote-Connect-and-Bridge.md` & `docs/wiki/Custom-Models.md` (System documentation)
- Model Context Protocol (MCP) Specification (v2024-11-05) & Agent-to-Agent (A2A) Protocol Specification (v0.3.0)

---

## 1. Executive Summary & Core Findings

### Question 1: Can Antigravity send messages directly to Gemini Web over the existing MCP / A2A connection?
**Answer: No.** Direct outbound communication from Antigravity to Gemini Web cannot be achieved over the existing MCP or A2A bridge connection.

**Root Causes**:
1. **MCP Client/Server Topology**: In your current configuration, **Gemini Web is the MCP Client (Host)** and **Antigravity Swiss Knife is the MCP Server**. Network connections are initiated outbound from Google's infrastructure to your tunnel (`https://<tunnel>/api/bridge/mcp`). Gemini Web does not expose an open listening socket, public IP, or webhook endpoint for external servers to call.
2. **A2A Provider Asymmetry**: Swiss Knife implements the A2A 0.3.0 server specification (`/.well-known/agent-card.json`, `/a2a`), advertising itself as an agent capable of receiving work (`message/send`). Google Gemini Web does **not** host an A2A Agent Card or an A2A JSON-RPC listener; it cannot be targeted as an A2A agent.
3. **Lack of MCP Sampling**: While the MCP specification includes an optional `sampling/createMessage` primitive (allowing a server to request a completion from the client's LLM), **Google Gemini Web does not implement MCP Sampling**. It only acts as a tool caller (`tools/list`, `tools/call`).
4. **Session Model**: Gemini Web only generates completions in response to an end-user interaction within `gemini.google.com`.

---

### Question 2: What is the best way to use Gemini 4 Argon with Antigravity?
Depending on your primary goal (zero-cost subscription quota vs. native IDE workflow), there are two optimal approaches:

| Approach | Primary Benefit | Trade-off | How to Use |
| :--- | :--- | :--- | :--- |
| **Option A: Invert Flow (Gemini Web as Lead, Antigravity as Executor)** | **100% Free** with Gemini Advanced subscription; uses Web Argon 4 thinking, 1M+ context, and web grounding. | Conversation root sits in Gemini Web, not Antigravity desktop chat. | Drive tasks from `gemini.google.com` using the existing MCP/A2A tools (`a2a_assign_task`, `antigravity_dispatch_task`). |
| **Option B: Native Antigravity via Swiss Knife Custom Models (GCP Vertex AI)** | **Native IDE Experience**: Full Antigravity agent tools, diffs, subagents, and thinking level selector. | Billed per token via Google Cloud Platform ($2 in / $10 out / $0.10 cached per 1M tokens). | In Swiss Knife -> `Custom Models` -> Add GCP Model `gemini-4-argon` (location `global`). Appears directly in Antigravity model dropdown. |
| **Option C: Antigravity Native Account Quota** | Seamless native Antigravity experience at $0 additional cost. | Requires Google account tier with Argon access enabled. | Select `Gemini 4 Argon High` from Antigravity's model list (tracked via `quota_5h_argon` / `quota_weekly_argon`). |

---

## 2. Detailed Technical Breakdown: Protocol & Architecture Analysis

### 2.1 The Directional Asymmetry of MCP
In the Model Context Protocol:
```text
[ Gemini Web (gemini.google.com) ]   <--- MCP Host / Client
               |
               | (Outbound HTTPS POST / SSE over Cloudflare / Tailscale Tunnel)
               v
[ Swiss Knife Daemon (pkg/bridge) ]   <--- MCP Server
               |
               v
[ Antigravity Desktop / CLI ]
```
- **Gemini Web connects to Swiss Knife**: Gemini Web is configured with the tunnel endpoint URL (`https://<host>/api/bridge/mcp`) and bearer token.
- **Tools are pulled, not pushed**: Gemini Web calls `tools/list` on Swiss Knife to discover tools (`a2a_assign_task`, `antigravity_dispatch_task`, `run_command`, etc.). When the user in Gemini Web prompts the model, Gemini Web calls `tools/call`.
- **No Inbound Endpoint on Gemini Web**: There is no HTTP server, WebSocket server, or listener running on `gemini.google.com` that Swiss Knife could connect to.
- **Why MCP Sampling Does Not Solve This**:
  - MCP section *Client Capabilities: Sampling* (`sampling/createMessage`) allows an MCP server to request an LLM completion from the host client.
  - However, Gemini Web's MCP client implementation does **not** declare or support sampling capabilities.
  - Even on hosts that do support sampling, sampling can only be triggered *during an ongoing tool invocation* initiated by the client; a server cannot trigger sampling out of nowhere without an incoming turn.

---

### 2.2 The Directional Asymmetry of A2A (Agent-to-Agent Protocol)
In Swiss Knife's A2A 0.3.0 implementation (`pkg/bridge/a2a.go` and `pkg/bridge/a2a_protocol.go`):
- Swiss Knife serves the canonical discovery document at `/.well-known/agent-card.json` (`SpecAgentCard`).
- Swiss Knife handles inbound JSON-RPC 2.0 methods at `/a2a`:
  - `message/send`: Accepts a task from an external caller and dispatches it to Antigravity.
  - `message/stream`: Streams task execution progress via Server-Sent Events.
  - `tasks/get`, `tasks/cancel`, `tasks/resubscribe`.
- **Gemini Web is NOT an A2A Agent**: Google does not expose an A2A agent card or A2A message endpoints for Gemini Web chat sessions. The upgrade to A2A enables external orchestrators (like AutoGen, CrewAI, LangGraph, or custom agent scripts) to send tasks to Antigravity, but does not turn Gemini Web into an open RPC receiver.

---

### 2.3 Why Browser Automation (Scraping Gemini Web) is an Anti-Pattern
One might consider running a local proxy that controls an open browser tab on `gemini.google.com` via CDP or Puppeteer to type prompts and scrape output:
- **Fragile DOM**: Google continuously updates obfuscated class names and layout components on `gemini.google.com`.
- **High Latency**: Simulating keystrokes, waiting for streaming DOM updates, and extracting markdown introduces 10–25 seconds of latency per turn.
- **Bot Detection & Account Risk**: High-frequency automated queries against Google web properties frequently trigger CAPTCHAs, temporary account lockouts, or session invalidation.
- **No Function Calling**: Web UI responses are raw text or markdown; they lack the structured JSON schemas and protocol guarantees Antigravity needs for agentic file edits, terminal execution, and subagent coordination.

---

## 3. Recommended Architectures

### Architecture 1: The Inverted Workflow (Gemini Web as Lead, Antigravity as Worker)
This leverages your **existing** setup without writing any new code:

```text
+-----------------------------------------------------------------------------------+
|               Inverted Architecture: Gemini Web Lead & Messenger Mode             |
+-----------------------------------------------------------------------------------+
  [Developer]
       |
       | 1. Chat & architect in Gemini Web with Gemini 4 Argon
       v
  [Gemini Web (Argon 4)]
       |
       | 2. Call MCP / A2A Tool: a2a_assign_task(prompt)
       v  (Streamable HTTPS Tunnel: /api/bridge/mcp)
  [Swiss Knife LeadHandler / A2ATaskManager (pkg/bridge)]
       |
       | 3. CDP Injection into active Antigravity session
       v
  [Antigravity Desktop 2.0 / CLI]
       |
       | 4. Executes code, writes tests, runs compiler/linters
       v
  [Swiss Knife Reads Artifacts & Transcripts]
       |
       | 5. Returns final execution evidence & diffs to Gemini Web
       v
  [Gemini Web (Argon 4) Reviews & Answers Developer]
```

**How to activate**:
1. In Swiss Knife, navigate to **Remote Connect & Bridge** -> **Gemini**.
2. Select **Lead & Messenger Mode** (or **Hybrid Mode**).
3. Ensure the Remote Access tunnel is active (`quick`, `tailscale`, or `custom`).
4. In Gemini Web, instruct Gemini: *"Review the repository structure and implement the feature in Antigravity"*.
5. Gemini 4 Argon will automatically call `a2a_assign_task` or `antigravity_dispatch_task`, monitor progress, and report back the completed result.

---

### Architecture 2: Native Antigravity Integration via GCP Vertex AI (Custom Models)
If you want to work **inside Antigravity Desktop** as your primary IDE and select `Gemini 4 Argon` from the native dropdown:

```text
+-----------------------------------------------------------------------------------+
|               Native Custom Model Architecture: Antigravity Desktop               |
+-----------------------------------------------------------------------------------+
  [Antigravity Desktop 2.0]
       |
       | 1. Developer selects "Gemini 4 Argon High" in native model selector
       | 2. Sends GenerateContent request to Swiss Knife Gateway
       v
  [Swiss Knife Custom Model Gateway (pkg/custommodels/gateway.go)]
       |
       | 3. Authenticates with GCP Service Account Key
       v  (HTTPS REST: https://aiplatform.googleapis.com)
  [Google Cloud Platform Vertex AI: publishers/google/models/gemini-4-argon]
       |
       | 4. Returns streaming token generation with extended thinking
       v
  [Antigravity Desktop: Full Agent Loops, Tools, In-line Diffs & Subagents]
```

**Configuration steps**:
1. Open Antigravity Swiss Knife -> **Custom Models** (`/custom-models`).
2. Click **Add Model** -> Select **Google Cloud Vertex AI (GCP)**.
3. Configure the fields:
   - **Model Identifier**: `gemini-4-argon`
   - **Location / Region**: `global` *(Critical: as enforced by `BuildVertexAIEndpoint`, `gemini-4-argon` is only hosted in the global location without region prefix)*.
   - **GCP Project ID**: Your Google Cloud project ID.
   - **Service Account Key**: Paste your GCP Service Account JSON key (requires `roles/aiplatform.user`).
4. Test connection (asserts HTTP 200 OK and reports billing account status).
5. Open Antigravity Desktop: hover over the model selector dropdown to configure the thinking level (`Low`, `Medium`, `High`), and start coding directly with Argon 4.

---

## 4. Conclusion & Actionable Advice

1. **Do not attempt to build a reverse pipe from Antigravity to Gemini Web**: The protocols (MCP and A2A) and Gemini Web's infrastructure are fundamentally client-driven; attempting to send unsolicited prompts to Gemini Web from the outside violates the transport design.
2. **If your priority is zero cost (Gemini Web subscription)**: Use Gemini Web with the existing MCP/A2A bridge in **Lead & Messenger Mode**. You get Argon 4's massive reasoning capacity driving Antigravity's local terminal execution.
3. **If your priority is native IDE integration**: Configure `gemini-4-argon` via **Swiss Knife Custom Models (GCP Vertex AI)** using the `global` endpoint.
