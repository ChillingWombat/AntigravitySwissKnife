# Security, Compliance & Cache Management

Antigravity Swiss Knife enforces strict security boundaries to protect user credentials, preserve cloud compliance, and maintain local filesystem hygiene.

---

## 1. Google Terms of Service & Cloud Acceptable Use Alignment

Antigravity Swiss Knife is engineered to operate strictly within the bounds of personal engineering tooling:

- **Local-Only Tooling**: Operates exclusively as a local supervisor on the developer's workstation.
- **No Reverse Engineering of Proprietary Model Weights**: Does not extract, scrape, or reverse-engineer upstream neural network weights or hosted service backends.
- **Direct First-Party Endpoints**: All API requests originated by Antigravity 2.0 or the companion daemon travel directly to official Google Cloud endpoints (`cloudcode.googleapis.com`, `generativelanguage.googleapis.com`). No intermediary third-party relay or proxy servers are placed in the network path.
- **Adherence to Authentication Standards**: Authentication tokens are refreshed using official OAuth 2.0 grant flows and stored in operating system keyrings.

---

## 2. Zero-Proxy Architecture vs. External Proxy Risks

A common pattern in unofficial AI tooling is the deployment of local HTTP proxy servers (e.g. tools that act as MITM reverse proxies or rewrite request headers to spoof third-party clients).

### 2.1 Risks of External Proxies
1. **Severe Account Suspension Risk**: Google Cloud and identity infrastructure monitor request patterns. When traffic flows through unauthorized proxy frameworks that mangle user-agent strings or insert spoofed headers, upstream fraud detection flags the account for automated suspension.
2. **One-Time Appeal Policy**: Suspended Google Cloud accounts are typically permitted only a single manual appeal. An unsuccessful appeal leads to permanent domain or account revocation.
3. **Token Leakage**: External proxies frequently store plaintext bearer tokens in logs or unencrypted caches, exposing keys to other local processes or malware.

### 2.2 The Zero-Proxy Guarantee
Antigravity Swiss Knife fundamentally rejects this proxy model:
- The daemon does **not** run an outbound HTTP proxy.
- It does **not** intercept, rewrite, or MITM API requests in transit.
- It interacts with Antigravity through clean OS-level credential provisioning (updating keyring entries and local configuration records) and direct CDP webview injection.

---

## 3. 6-Probe Custom Model Security Auditor

When developers bridge custom models (OpenAI, Anthropic Claude, DeepSeek, or local LLMs) into their workspace, the security auditor (`pkg/custommodels/`) subjects the endpoint to six automated security probes before authorizing the connection:

| Probe | Name | Objective & Security Check |
| :--- | :--- | :--- |
| **Probe 1** | TLS Transport Validation | Enforces TLS 1.3 or modern TLS 1.2 with strict certificate verification; rejects unencrypted HTTP or invalid certificates for external domains. |
| **Probe 2** | Proxy Header Analysis | Inspects response headers to detect unauthorized intermediary caching, tracking proxies, or unexpected `Via` / `X-Forwarded-For` injection. |
| **Probe 3** | Prompt Injection Defense | Sends canary test inputs containing indirect prompt injection patterns to verify the remote endpoint does not echo sensitive system prompts. |
| **Probe 4** | Tool-Call Schema Integrity | Sends complex nested JSON tool definitions and asserts that the endpoint returns valid, parseable function call payloads without schema corruption. |
| **Probe 5** | Model Downgrade Canary | Sends a targeted reasoning challenge to detect covert model downgrade substitutions (e.g. an endpoint advertising a frontier model but serving a quantized small model). |
| **Probe 6** | Latency & Jitter Baseline | Measures round-trip latency variance across multiple tokens to ensure predictable interactive performance and identify unstable endpoints. |

---

## 4. Cache Auto-Pruner

Over extended development sessions, Antigravity generates gigabytes of cached conversation artifacts, temporary tool outputs, and language server logs.

The cache auto-pruner (`pkg/cache/`) organizes local storage into five distinct categories:

1. **`scratch`**: Temporary scratchpads and diff previews.
2. **`steps`**: Detailed step-by-step reasoning outputs and execution plans.
3. **`tasks`**: Background task logs, subagent execution traces, and stdout dumps.
4. **`conversations`**: Historical conversation JSON caches and summary records.
5. **`other`**: Language server temporary files and extension run caches.

### 4.1 Unlimited Defaults Policy
By default, all category size limits and retention periods are set to **Unlimited (`0`)**. Antigravity Swiss Knife will **never** automatically delete developer data or conversation history without explicit user configuration.

### 4.2 Configurable Thresholds & Background Cleanup
Developers can configure retention limits in System Settings (e.g. prune scratch files older than 14 days, or cap task logs to 500 MB). When configured:
- A background worker runs every 6 hours with low I/O priority (`ionice` idle class on Linux).
- Files exceeding age or size thresholds are safely unlinked.
- Active conversations and pinned revival files are strictly protected from pruning.
