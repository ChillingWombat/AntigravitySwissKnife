# Token Broker Threat Model

Issue: #361. Scope: the Electron renderer, preload bridge, Electron main process, loopback daemon, and bridge machine-control tools.

## Assets

- Daemon startup and session tokens.
- Host filesystem reads and writes, file deletion, terminal launch, IDE launch, and computer-use actions.
- Bridge configuration, outbound HTTP permissions, and provider credentials.
- Approval decisions and request arguments.

## Trust Boundaries

| Boundary | Trusted side | Untrusted side |
| --- | --- | --- |
| Renderer to Electron main | Main process reads the token and owns the HTTP client. | React UI, injected scripts, local XSS, and compromised extensions. |
| Electron main to daemon | Fixed loopback origin, allowlisted method/path, main-process auth header. | Renderer-supplied headers, URLs, redirects, and arbitrary request bodies. |
| Bridge to host tools | Explicit user allowlist and typed approval failures. | MCP callers and model-generated tool arguments. |
| Browser fallback to daemon | Unlock flow and scoped session token. | Hostile or unauthenticated pages. |

The renderer receives `window.swiss.request` only. It never receives a raw session-token getter, page-global token, session-storage token, token URL, or token-bearing error.

## Threats

### Local XSS

XSS can invoke every broker route available to the renderer and can alter the visible UI. It cannot read the daemon token from the renderer. Read routes remain exact-allowlisted. Every non-allowlisted `/api/*` POST one-shot asks the user through a native Electron dialog showing the exact method and path, with a bounded and redacted body preview. XSS can request a prompt but cannot silently approve it.

### Compromised browser extension

An extension with access to the Antigravity renderer has the same broker capability as local XSS and can also manipulate DOM state. It cannot call the native approval API directly or read the Electron main-process token. It can trigger prompt spam and impersonate the product UI around the prompt. Users must verify the native dialog's method and path rather than trusting renderer text.

### Hostile page or navigation

The broker pins requests to `http://127.0.0.1:8765`, rejects absolute and protocol-relative paths, strips caller authentication headers, and rejects redirects. Renderer fetch/XHR requests do not receive the startup token through the Electron session interceptor. A hostile page cannot turn the broker into an arbitrary-origin proxy.

### Replay and prompt reuse

Approval is one request only and is never cached. Tokens are sent in the `X-Session-Token` header, never in URLs, and are not logged. Replaying an approved body requires a new request and a new prompt. The daemon remains the final authorization check.

### Confused deputy

The renderer can ask the privileged main process to act. The broker limits the origin and route shape, and the native dialog binds the approval to the exact method and path. Machine tools in the Go bridge require an explicit entry in `allowed_tools`; `CallTool` returns `APPROVAL_REQUIRED` instead of treating an empty allowlist as unrestricted.

## Residual Risks

- A compromised Electron main process or preload script is outside this boundary and can read the token.
- Approval fatigue can cause a user to accept a dangerous command or path.
- Body previews redact known sensitive fields and opaque content, but arbitrary secrets can still appear in displayed path names or other short values.
- Existing non-machine mutation routes already allowlisted by the broker remain silent, one-shot user approval is required only for other mutating routes.
- The non-Electron browser fallback stores its unlocked session token in origin-scoped local storage. Local XSS in that browser mode can read it. The fallback now fails closed without a token and never puts the token in a URL.
- The daemon token is long-lived on disk; rotation and secure storage depend on the daemon installation.
- An extension or screen-control malware that can click the native dialog defeats user approval.
