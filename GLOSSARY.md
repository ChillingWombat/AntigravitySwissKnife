# Domain Glossary

Canonical terminology for Antigravity Swiss Knife (`cmd/swiss`, `pkg/...`, `antigravity-acp`, `electron/`, `frontend/`).

## Core Entities

- **Host IDE (`Antigravity`)**: The Google Antigravity 2.0 desktop application (`~/.gemini/antigravity`) or CLI (`agy`) whose credentials, quotas, hardware profiles, and DOM enhancements are managed by Swiss Knife.
- **Companion Daemon (`swiss`)**: The local Go background process serving loopback HTTP (`127.0.0.1:8765`) and Unix domain / named-pipe JSON-RPC (`daemon.sock`), responsible for quota polling, account rotation, session revival, and ACP mesh coordination.
- **Session Token (`X-Session-Token`)**: Per-user 256-bit secret persisted at `0600` in `<ConfigDir>/session.token`, required on all `/api/*` requests outside the public bootstrap whitelist.
- **Account Fleet**: The set of Google and custom model credentials stored in the encrypted keyring (`pkg/keyring`), ranked by 5-hour and weekly quota fractions, plan tier (`Free`, `Plus`, `Pro`, `Pro - Trial`, `Ultra`), and cooldown state.
- **Hardware Profile (`DeviceProfile`)**: Per-account deterministic telemetry identifiers (`machine_id`, `sqm_id`, `dev_device_id`, `installation_id`) synchronized into host storage before account switches to maintain account isolation.
- **Cascade (`conversation_id`)**: A single Antigravity agent conversation trajectory stored in `conversation_summaries.db` and `conversations/<id>.db`.
- **Active Cascade Shield**: Protection rule in `pkg/cache/pruner.go` that prevents cache pruning from deleting brain/step/task artifacts belonging to conversations marked running or modified within the active staleness window, synchronized across processes via `.prune.lock`.
- **Revival Intent (`RevivalIntent`)**: Persisted state record capturing an interrupted cascade prior to an account switch or crash, used by `pkg/revival` to resume work via CDP or CLI after host relaunch.

## Protocols & Bridges

- **ACP Adapter (`antigravity-acp`)**: Standalone stdio JSON-RPC 2.0 server (`antigravity-acp/cmd/antigravity-acp`) implementing the Agent Client Protocol over either the Antigravity Language Server (`ls` Connect RPC surface) or the `agy` CLI (`cli` stream-json surface).
- **ACP Agent Mesh (`pkg/acpmesh`)**: Registry and protocol probe client that resolves built-in presets (`Google Antigravity 2.0`, `Antigravity ACP Adapter`, `Pi`, `Codex`, `Claude Code`, `OpenCode`, `Devin`, `Cursor`) and user overrides, executing live ACP `initialize` and Pi RPC `get_state` handshakes and prompt dispatch.
- **Custom Model Gateway (`pkg/custommodels/gateway.go`)**: Loopback translation proxy that exposes OpenAI, Anthropic, Gemini API, and Vertex AI GCP models to Antigravity as Gemini-compatible streaming endpoints while enforcing SSRF IP-pinning validation.
- **Remote & Channel Bridges (`pkg/bridge`)**: Authenticated tunnel and messaging relays (Remote Web Console, Gemini Bridge OAuth 2.0 / MCP, WhatsApp, Telegram, Discord Gateway v10) connecting external clients to local Antigravity sessions.
