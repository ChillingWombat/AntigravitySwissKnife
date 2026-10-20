# Remote Connect & Multi-Channel Agent Bridges

[Features](Features.md) / Remote Connect & Bridge

The Remote Connect subsystem (Nav Rail `10`) provides secure remote access to Antigravity 2.0 and connects external chat platforms directly to local agent workflows. The top ribbon provides five functional tabs: `Remote`, `Gemini`, `WhatsApp`, `Telegram`, and `Discord`.

---

## 1. Remote Control (`/remote`) Architecture

The Remote Control view provides complete browser-based remote operation of the active Antigravity session on mobile devices, tablets, and remote workstations without requiring physical desktop access.

```text
+---------------------------------------------------------------------------------+
|                       Remote Control Architecture (/remote)                     |
+---------------------------------------------------------------------------------+
  [Remote Browser / Mobile Device]
                |
                v  (HTTPS / WSS Tunnel)
  [Cloudflare Quick Tunnel / Tailscale Funnel / Tailnet / Custom Domain]
                |
                v  (Loopback HTTP: 127.0.0.1:8766)
  [Swiss Knife Remote Proxy (pkg/bridge/remote.go)]
        |-- Live Upstream Discovery via Chrome DevTools Protocol (CDP)
        |-- Live CSRF Token Re-binding ("x-codeium-csrf-token")
        |-- React Fiber Account Sync Injection ("userStatus/setUserStatus")
        |-- Official Antigravity Multi-Color Favicon Replacement (/favicon.svg, /favicon.ico)
        |-- 8-Character Alphanumeric PIN / App Password Gate
                |
                v  (Loopback HTTP / Connect-RPC)
  [Antigravity 2.0 language_server Web App (exa.language_server_pb.*)]
```

### Upstream Discovery via Chrome DevTools Protocol (CDP)
Antigravity Desktop runs an internal web application managed by its `language_server` process. When remote access is enabled, the Swiss Knife daemon queries active Electron targets via CDP on loopback port `127.0.0.1`. It identifies the running web application origin and registers it as the upstream target for the reverse proxy.

### Real Antigravity 2.0 Web App Reverse Proxy
Rather than rendering simulated gadgets or synthetic mockups, the proxy forwards all incoming traffic to the real Antigravity web app running on loopback:
- **Static Assets & Scripts**: Serves the authentic client bundle directly from the upstream `language_server`.
- **Connect-RPC Endpoints**: Proxies streaming RPC requests (`/exa.language_server_pb.LanguageServerService/*`) with zero response buffering (`FlushInterval: -1`), keeping server-sent events, token streams, and agent progress indicators responsive.
- **Fallback Console**: If Antigravity is closed or CDP is detached, the proxy falls back to an integrated management interface providing screenshot streaming, conversation switching, and task dispatching.

### Official Antigravity Entrance Screen
When unauthenticated clients open `/remote`, the proxy serves an entrance screen matching the official Antigravity aesthetic:
- Native Antigravity multi-color SVG insignia and radial color glow backdrop.
- Obfuscated PIN entry input with an eye toggle for visibility.
- Direct JSON authentication via `/remote/api/auth` setting an encrypted session cookie (`swiss_remote_pin`).

### Official Antigravity Favicon Replacement
Remote Connect (`/remote` and proxied `language_server` web pages) replaces the upstream internal gift favicon with the official Google Antigravity multi-color delta SVG icon (`/favicon.svg` and `/favicon.ico`). Both the unauthenticated entrance screen and proxied application routes serve the official icon, preventing external browser tabs, bookmarks, and mobile home-screen web clips from loading development placeholder assets.

### Live CSRF Token Re-binding Across Restarts
Antigravity's upstream `language_server` generates a dynamic CSRF token on startup and verifies it on every Connect-RPC call via the `x-codeium-csrf-token` header:
- The reverse proxy scans incoming HTML payloads with a regex matcher (`"csrfToken"\s*:\s*"([^"]+)"`) to extract the current token.
- Outgoing RPC requests are automatically rewritten with the fresh upstream token.
- When `language_server` restarts during background sleep cycles or crashes, the proxy captures the new token on the next response, preventing `403 Forbidden` dropouts.

### Live Redux Account Identity Sync Across Switches
When switching accounts in Swiss Knife, the backend updates OAuth credentials and restarts local language servers. To ensure remote browser sessions reflect the new account without requiring manual reloads, the proxy injects an account coordination script (`swiss-remote-account-sync`) into the HTML body:
- **Fiber Traversal**: Every 2.5 seconds, on window focus, and on document visibility transitions, the script traverses the React Fiber tree from `#root` to locate `userStatusProvider`, `agentServiceResolver`, or the Redux `store`.
- **Status Retrieval**: Queries `getUserStatus()` from the resolved agent service client.
- **Redux Dispatch**: When a change in user email or plan tier is detected, it dispatches:
  ```javascript
  store.dispatch({ type: "userStatus/setUserStatus", payload: nextUserStatus });
  ```
- This updates account badges, quota meters, and model selectors across all connected remote devices immediately.

---

## 2. Four Remote Control Tunnel Modes

Remote Control supports four network tunnel architectures:

| Tunnel Mode | Identifier | Network Scope | Endpoint Format | Configuration Requirements |
| :--- | :--- | :--- | :--- | :--- |
| **Cloudflare Quick Tunnel** | `quick` | Public Internet | `https://*.trycloudflare.com/remote` | Zero configuration. Requires `cloudflared` binary on host. |
| **Tailscale Funnel** | `tailscale` | Public Internet | `https://*.ts.net/remote` | Requires active Tailscale connection and Funnel permission. |
| **Tailscale Private Tailnet** | `tailnet` | Private Encrypted Mesh | `http://100.x.y.z:8766/remote` or MagicDNS | Private to authorized tailnet nodes. Binds local TCP forwarder. |
| **Custom Domain** | `custom` | Custom Public / Private | `https://your-domain.com/remote` | User-managed reverse proxy. Requires valid `https://` endpoint. |

### Cloudflare Quick Tunnel (`quick`)
Spawns an on-demand `cloudflared` process with command line:
```bash
cloudflared tunnel --no-autoupdate --url http://127.0.0.1:8766
```
Parses process stdout for registered `trycloudflare.com` hostnames and exposes the endpoint without needing a Cloudflare account, custom domain, or open router ports.

### Tailscale Funnel (`tailscale`)
Invokes `tailscale funnel --bg http://127.0.0.1:8766` to route public HTTPS requests through Tailscale's edge infrastructure directly to the local daemon port.

### Tailscale Private Tailnet (`tailnet`)
Designed for strict private access without exposing any ports to the public internet:
1. Queries the machine's Tailscale IPv4 address (`100.x.y.z`) and MagicDNS hostname via `tailscale ip -4` and `tailscale status --json`.
2. Starts a TCP forwarder listening on `<Tailscale-IP>:<localPort>` that pipes connections directly to `127.0.0.1:<localPort>`.
3. Only authenticated devices joined to the user's private tailnet can reach the endpoint.

### Custom Domain (`custom`)
Allows routing through pre-existing enterprise reverse proxies (Nginx, Caddy, Cloudflare Zero Trust, Traefik). The endpoint validator enforces the following constraints:
- Must use the `https://` scheme.
- Must include a valid hostname.
- Rejects loopback addresses (`127.0.0.1`, `localhost`, `::1`) to prevent dead configurations.

---

## 3. Four-Minute Keep-Alive & Auto-Stop Protection

Public tunnel gateways drop idle TCP connections if no packets traverse the route. Cloudflare Quick Tunnels terminate connections after 5 minutes (300 seconds) of inactivity.

```text
+---------------------------------------------------------------------------------+
|                  4-Minute Tunnel Keep-Alive & Auto-Stop Cycle                   |
+---------------------------------------------------------------------------------+
  [Every 4 Minutes: TunnelManager.ProbeAndKeepAlive()]
         |
         v
  Send HTTP GET to Public Gateway URL with header:
  "X-Swiss-KeepAlive: 1"
         |
         +--> [HTTP 200 OK / 401 Unauthorized] -> Tunnel alive; reset keep-alive timer
         |
         +--> [502 / 503 / 504 / 521-524 / 530 or Unreachable]
                     |
                     v
              [invalidateTunnel()]
                     |-- Kill tunnel process (cloudflared / tailscale)
                     |-- Close loopback HTTP listener
                     |-- Reset Active state to false
                     |-- Log exact failure cause in tunnel history
```

### Probing Mechanism
- The `TunnelManager` runs a background ticker every 4 minutes (`DefaultKeepAliveInterval = 4 * time.Minute`).
- Dispatches a lightweight HTTP GET request to the active public gateway URL with header:
  ```http
  X-Swiss-KeepAlive: 1
  ```
- Local endpoints acknowledge this header and return immediate `200 OK` status without loading full web assets.

### Automated Fault Invalidation & Auto-Stop
If the probe detects network failure or receives upstream Cloudflare / gateway error codes:
- `502 Bad Gateway`
- `503 Service Unavailable`
- `504 Gateway Timeout`
- `521 Web Server Is Down`
- `522 Connection Timed Out`
- `523 Origin Is Unreachable`
- `524 A Timeout Occurred`
- `530 Cloudflare Error`

The daemon immediately calls `invalidateTunnel(reason)`. This kills the tunnel subprocess, closes the HTTP listener, sets `config.Enabled = false`, and records the diagnostic failure in the log stream. This ensures stale or orphaned tunnel URLs do not linger on the host.

---

## 4. Security PIN, Session Revocation & Brute-Force Defense

Remote access is guarded by four defense layers:

### 8-Character Alphanumeric Security PIN
Generated via `GeneratePairingPIN()` using cryptographically secure random bytes (`crypto/rand`). Characters are sampled across a 62-character keyspace (`0-9`, `A-Z`, `a-z`) with guaranteed digit and letter distribution. Validated via `IsAlphanumericPIN8()`.

### Instant Session Revocation (`Rotate PIN`)
Clicking **Rotate PIN** generates a new 8-character PIN, saves it to `remote_control.json`, and flushes the in-memory client session table (`activeClients`). Any client currently connected is locked out on their next request and redirected to the entrance screen.

### Master App Password Toggle (`Use App Password`)
When enabled, remote clients can authenticate using either the 8-character PIN or the master PBKDF2-HMAC-SHA256 Application Access Password configured in System Settings.

### Brute-Force Rate Limiting
The authentication validator tracks failed attempts by client IP (resolving `X-Forwarded-For`, `X-Real-IP`, and remote socket addresses):
- **Threshold**: 5 failed PIN attempts within a 5-minute window.
- **Action**: Locks out the client IP for 15 minutes.
- **Audit Log**: Records timestamp, IP, authorization method, and access result in a circular 50-entry audit buffer.

---

## 5. Gemini Bridge (Streamable HTTP MCP)

The **Gemini** tab connects Gemini Web, Gemini Advanced, and external orchestrators to Antigravity via the Model Context Protocol (MCP) streamable HTTP transport (`/api/bridge/mcp`).

### Tunnel Modes
Gemini Bridge supports three public HTTPS tunnel modes: `Cloudflare Quick Tunnel` (`quick`), `Tailscale Funnel` (`tailscale`), and `Custom Domain` (`custom`).

### Three Operating Modes
- **Hybrid**: Inspects incoming tool calls. Tasks targeting Antigravity conversations or project tasks route to Antigravity via CDP; local filesystem, shell, and desktop commands execute directly on the host machine.
- **Lead**: Treats Antigravity as the primary agent lead. All prompts and instructions are dispatched into Antigravity conversations.
- **Direct**: Operates as a direct host actuator without interacting with Antigravity chat sessions.

### Registered MCP Tools
- **Host Actuation**: `run_command` (with timeout and directory targeting), `read_file`, `write_file`, `list_directory`, `system_info`, `computer_use` (native screen capture using `grim`, `scrot`, `import`, or Windows GDI).
- **Agent Orchestration**: `dispatch_task`, `send_message`, `conversation_status`, `list_conversations`, `get_screenshot`.

### Built-in OAuth 2.0 / 2.1 MCP Authorization Server (RFC 9728, RFC 8414, RFC 7591, PKCE)
To support zero-configuration MCP connection setup from Google Gemini App and Web, Gemini Bridge implements an RFC-compliant OAuth 2.0 / 2.1 authorization server with Proof Key for Code Exchange (PKCE).

```text
+---------------------------------------------------------------------------------+
|                  OAuth 2.0 / 2.1 Authorization Server Architecture              |
+---------------------------------------------------------------------------------+
  [Google Gemini / MCP Client]
         |
         | 1. POST /api/bridge/mcp (Unauthenticated)
         v
  [Gemini Bridge HTTP Gateway]
         |
         | 2. 401 Unauthorized + WWW-Authenticate (RFC 9728 resource_metadata)
         v
  [Client Queries Discovery Endpoints]
         |-- GET /.well-known/oauth-protected-resource
         |-- GET /.well-known/oauth-authorization-server (or /openid-configuration)
         v
  [Dynamic Client Registration / Pre-Configured Client ID]
         |-- POST /oauth/register (RFC 7591 -> client_id & client_secret)
         |-- Or uses explicit client_id "antigravity-gemini-bridge"
         v
  [Interactive Authorization Consent]
         |-- Browser opens GET /oauth/authorize?response_type=code&code_challenge=...
         |-- User verifies with 8-character Pairing PIN or App Access Password
         |-- POST /oauth/authorize -> 302 Redirect with single-use ?code=...
         v
  [Token Exchange (PKCE Verification)]
         |-- POST /oauth/token (grant_type=authorization_code + code_verifier)
         |-- Verifies SHA-256 S256 hash or plain code_verifier
         v
  [Authenticated MCP Session]
         |-- POST /api/bridge/mcp with "Authorization: Bearer <access_token>" -> 200 OK
```

#### RFC 9728 Protected Resource Metadata & 401 Challenge
When Gemini connects to `/api/bridge/mcp` without credentials, the daemon returns HTTP `401 Unauthorized` with the RFC 9728 challenge header:
```http
WWW-Authenticate: Bearer realm="Antigravity Swiss Knife MCP", resource_metadata="https://<host>/.well-known/oauth-protected-resource"
```
Gemini parses `resource_metadata` and queries the metadata endpoint:
- **Endpoint**: `GET /.well-known/oauth-protected-resource`
- **Payload**:
  ```json
  {
    "resource": "https://<host>/api/bridge/mcp",
    "authorization_servers": ["https://<host>"],
    "bearer_methods_supported": ["header", "body", "query"],
    "scopes_supported": ["mcp", "gemini", "claudeai", "openid", "profile"]
  }
  ```

#### RFC 8414 Authorization Server Metadata & OpenID Discovery
Gemini resolves the authorization server configuration from standard discovery paths:
- **Endpoints**: `GET /.well-known/oauth-authorization-server` and `GET /.well-known/openid-configuration`
- **Payload**:
  ```json
  {
    "issuer": "https://<host>",
    "authorization_endpoint": "https://<host>/oauth/authorize",
    "token_endpoint": "https://<host>/oauth/token",
    "registration_endpoint": "https://<host>/oauth/register",
    "response_types_supported": ["code"],
    "grant_types_supported": ["authorization_code", "refresh_token", "client_credentials"],
    "token_endpoint_auth_methods_supported": ["client_secret_post", "client_secret_basic", "none"],
    "code_challenge_methods_supported": ["S256", "plain"],
    "scopes_supported": ["mcp", "gemini", "claudeai", "openid", "profile"]
  }
  ```

#### RFC 7591 Dynamic Client Registration
Clients supporting dynamic onboarding register client metadata automatically:
- **Endpoint**: `POST /oauth/register`
- **Request**:
  ```json
  {
    "client_name": "Google Gemini Client",
    "redirect_uris": ["https://gemini.google.com/auth/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "scope": "mcp"
  }
  ```
- **Response**: Returns HTTP `201 Created` with `client_id` (`antigravity-gemini-bridge`), `client_secret` (the active daemon token), registered redirect URIs, and allowed grant types.

#### Interactive Authorization Consent (`/oauth/authorize`)
Gemini directs the user's browser to the authorization endpoint:
- **`GET /oauth/authorize`**: Displays an Antigravity-styled consent screen detailing the requested permissions (`scope=mcp`). Prompts the operator for the 8-character Pairing PIN or the master App Access Password (if `Use App Password` is toggled).
- **Fast-Path URL Parameter**: Accepts `?pin=<pin>` or `?password=<pass>` for automated authorization, returning an immediate `302 Found` redirection without rendering the form.
- **`POST /oauth/authorize`**: Validates the PIN or password against `VerifyCandidatePIN()`. On match, generates a single-use 48-character hexadecimal authorization code valid for 10 minutes, stores the client's PKCE `code_challenge` and `redirect_uri`, and redirects to `redirect_uri?code=<code>&state=<state>`. Rejects invalid inputs with HTTP `401 Unauthorized`.

#### Token Exchange (`POST /oauth/token`)
Exchanges credentials or authorization codes for Bearer access tokens:
- **`authorization_code` Grant**: Validates the single-use code, enforces `redirect_uri` matching, and verifies PKCE `code_verifier` against the stored `code_challenge` via SHA-256 (`S256`) or `plain`. Once consumed, codes cannot be replayed.
- **`refresh_token` Grant**: Exchanges a valid refresh token for an active Bearer token.
- **`client_credentials` Grant**: Authenticates via form body, JSON payload, or HTTP Basic Auth (`Authorization: Basic <base64>`) using the `client_id` and `client_secret` (or Pairing PIN / App Access Password).
- **Token Lifetime**: Returned tokens specify `expires_in: 31536000` (1 year) with Bearer token type.

#### Gemini Bridge UI Credentials & Manual Setup
For clients or extensions requiring manual configuration rather than automatic discovery, the Gemini Bridge UI displays explicit credentials ready for Google Gemini's *Additional settings*:
- **OAuth Client ID**: `antigravity-gemini-bridge`
- **OAuth Client Secret**: The active daemon Bearer Token (can be rotated anytime with **Rotate PIN**)
- **Pairing PIN**: 8-character alphanumeric code for interactive consent, or the master App Access Password when toggled.

---

## 6. External Chat Agent Bridges

Antigravity Swiss Knife includes three chat channel bridges for communicating with Antigravity from messaging apps:

```text
+---------------------------------------------------------------------------------+
|                       Multi-Channel Chat Bridges Architecture                   |
+---------------------------------------------------------------------------------+
  [WhatsApp Client]   --> [Meta Graph API v21.0 / Webhook] --+
  [Telegram Client]   --> [Telegram Bot API (getUpdates)]   --+--> [ChannelsManager]
  [Discord Client]    --> [Discord Bot Gateway / REST]       --+          |
                                                                          v
                                                             [Antigravity Lead Dispatch]
```

### WhatsApp Bridge
- **Modes**: `cloud_api` (Meta WhatsApp Business Platform Graph API v21.0), `mcp` (Model Context Protocol connector), and `webhook` (custom webhook endpoints).
- **Credentials**: Phone Number ID, Permanent System User Access Token, and Webhook Verification Token.
- **Sender Whitelisting**: `allowed_numbers` array filters inbound messages, ignoring unauthorized contacts.

### Telegram Bridge
- **Bot Token**: Authenticates bots created through `@BotFather`.
- **Modes**: Long-polling (`getUpdates`) or incoming webhook notifications.
- **Access Control**: `allowed_chat_ids` restricts command execution to specific Telegram user IDs or group chats.
- **Live Verification**: Queries `https://api.telegram.org/bot<token>/getMe` to verify the bot username and connection state.

### Discord Bridge
- **Bot Token & Application ID**: Connects via Discord REST API v10 (`users/@me`).
- **Channel & User Filters**: `allowed_channels` and `allowed_users` whitelist authorized server channels and Discord accounts.
- **Formatting**: Strips Discord markdown mentions before passing prompt instructions to Antigravity.
