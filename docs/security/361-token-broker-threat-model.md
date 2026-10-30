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

The renderer receives `window.swiss.request` only. It never receives a raw session-token getter, page-global token, token URL, or token-bearing error. Browser fallback tokens stay in memory, and startup purges the legacy `swiss_session_token` key from both local storage and session storage.

## Threats

### Local XSS

XSS can invoke every broker route available to the renderer and can alter the visible UI. It cannot read the daemon token from the renderer. Safe and privileged routes are explicit method-and-path tables. Unknown routes fail with `ROUTE_NOT_ALLOWED` before the broker reads a token or calls the daemon. Privileged POST routes ask through a native Electron dialog showing the exact method and path, with a bounded and redacted body preview. XSS can request a prompt but cannot silently approve it.

### Compromised browser extension

An extension with access to the Antigravity renderer has the same broker capability as local XSS and can also manipulate DOM state. It cannot call the native approval API directly or read the Electron main-process token. It can trigger prompt spam and impersonate the product UI around the prompt. Users must verify the native dialog's method and path rather than trusting renderer text.

### Hostile page or navigation

The broker pins requests to `http://127.0.0.1:8765`, rejects absolute and protocol-relative paths, strips caller authentication headers, and rejects redirects. The session interceptor attaches the startup token only to top-level daemon document and static loads. Renderer fetch/XHR requests and daemon `/api` subframe navigations do not receive the token; subframe navigations to that API are cancelled. A hostile page cannot turn the broker into an arbitrary-origin proxy.

### Replay and prompt reuse

Approval is one request only and is never cached. Tokens are sent in the `X-Session-Token` header, never in URLs, and are not logged. Replaying an approved body requires a new request and a new prompt. The daemon remains the final authorization check.

### Confused deputy

The renderer can ask the privileged main process to act. The broker limits the origin and exact route, and the native dialog binds the approval to the method and path. Filesystem, Obsidian, computer-use, HTTP, file-write, and command tools in the Go bridge require an explicit entry in `allowed_tools`; new defaults omit them, and `CallTool` returns `APPROVAL_REQUIRED` instead of treating an empty allowlist as unrestricted. Existing explicit allowlist entries remain intact.

### File read and list

`/api/files/read` and `/api/files/list` are privileged GET routes. Each request requires a fresh native approval that shows the canonical filesystem path. The broker decodes path values repeatedly until stable, allowing at most ten passes; malformed encoding or a value still changing after ten passes is denied. It rejects missing, relative, NUL-containing, and traversal-containing fully decoded path values. It resolves symlinks when the target exists and compares both lexical and resolved paths against the Swiss daemon configuration directory. Paths inside that directory and files named `session.token`, `session.pid`, or `credentials*` are denied before the broker reads the daemon token or calls the daemon; the native dialog states the denial reason.

## Residual Risks

- A compromised Electron main process or preload script is outside this boundary and can read the token.
- Approval fatigue can cause a user to accept a dangerous command or path.
- Body previews redact known sensitive fields and opaque content, but arbitrary secrets can still appear in displayed path names or other short values.
- Safe broker mutation routes remain silent; privileged GET file reads/lists and privileged POST routes require one-shot native approval.
- A filesystem race can replace a checked path after approval and before the daemon opens it. The daemon still accepts sensitive reads when called directly with a valid session token.
- The non-Electron browser fallback keeps its unlocked session token only in page memory, so reloads require another unlock. Local XSS in that browser mode can still read the in-memory token during the unlocked session.
- The daemon token is long-lived on disk; rotation and secure storage depend on the daemon installation.
- An extension or screen-control malware that can click the native dialog defeats user approval.
