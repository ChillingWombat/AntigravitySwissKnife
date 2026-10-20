# Custom Models

[Features](Features.md) / Custom Models

The Custom Models module provides a Bring-Your-Own-Key (BYOK) gateway, allowing developers to connect third-party LLMs and local runtimes directly to Antigravity without using network proxies.

---

## 1. Zero-Proxy Provider Architecture

Antigravity Swiss Knife never deploys an HTTP MITM proxy or external server to route model traffic. Instead, it writes validated provider configurations directly into Antigravity's local model registry files (`custom_models.json`). The host application dispatches API requests straight from the developer workstation to upstream endpoints.

### Supported API Standards
- **Anthropic Protocol**: Native Anthropic API (`/v1/messages`) supporting Claude 3.5 Sonnet, Claude 3 Opus, and prompt caching headers.
- **OpenAI Protocol**: Standard chat completions API (`/v1/chat/completions`) compatible with OpenAI GPT-4o, DeepSeek V3/R1, Mistral, and OpenRouter.
- **Google AI Studio Protocol**: Direct API key authentication against `generativelanguage.googleapis.com/v1beta`.
- **Local Runtime Endpoints**: Local LLMs running via Ollama, vLLM, or LiteRT on `http://localhost:11434` or custom ports, enabling 100% offline agent operation.

---

## 2. Six-Probe Automated Security Audit

To protect developer API keys from leaking to untrusted endpoints or unencrypted networks, the Custom Models manager runs a 6-probe security audit whenever a new model endpoint is added or tested:

```text
+------------------------------------------------------------------------+
|                      6-Probe Model Security Suite                      |
+------------------------------------------------------------------------+
  [Probe 1: TLS Enforcement]       -> Rejects insecure HTTP schemes (except localhost)
  [Probe 2: Query Param Leakage]   -> Confirms API keys are never embedded in URL query strings
  [Probe 3: Header Authorization]  -> Asserts standard Authorization/x-api-key bearer headers
  [Probe 4: CORS Posture]          -> Validates endpoint origin boundaries
  [Probe 5: Payload Sanitization]  -> Probes request body stripping of local workstation paths
  [Probe 6: Log Token Masking]     -> Asserts token stripping in local debug output
```

The UI displays clear status badges (`SECURE`, `WARNING`, or `UNSAFE`) and provides an interactive Security Report modal detailing each probe outcome.

---

## 3. Metadata Extraction & Model Parameter Detection

When configuring a custom model, the engine probes the endpoint to automatically detect operational parameters:
- **Clean Model Identifier**: Parses vendor-specific prefixes and resolves canonized model identifiers.
- **Context Window Sizing**: Probes context limits (e.g., 128k, 200k, 1M, or 2M tokens) and updates tokenizer budgets accordingly.
- **Thinking / Reasoning Support**: Detects support for extended reasoning tokens (e.g., Anthropic thinking budgets, DeepSeek R1 reasoning output).
- **Custom Quota Endpoint**: Accepts an optional secondary endpoint to poll real-time dollar balances or credit burn rates for pay-per-token API providers.

---

## 4. Subagent Delegation & Quota Preservation

Developers can designate specific custom models to handle autonomous subagent runs:
- Primary pair-programming prompts continue using high-capability Gemini models via official quotas.
- Resource-intensive background subagents (such as unit test generators, codebase indexers, or documentation writers) are delegated to fast, cost-effective custom endpoints (such as DeepSeek V3 or local Ollama instances).
- Prevents subagent background loops from exhausting main account 5-hour burst quotas.

---

## 5. In-Selector Thinking Level Flyout Submenu

Custom Models and GCP Vertex AI models supporting extended reasoning (such as `Gemini 4 Argon`, `Claude Opus 5.5`, and `DeepSeek Flash`) configure thinking budgets directly inside Antigravity's native model selector dropdown menu, eliminating the need for detached external composer pills.

### Flyout Menu Architecture
- **In-Selector Attachment**: When hovering over a custom or Vertex AI model row supporting reasoning, the injected runtime mounts `#swiss-custom-thinking-submenu` adjacent to Antigravity's native dropdown (`[role="menu"]:not(#swiss-custom-thinking-submenu)`).
- **Thinking Levels**: Renders radio options (`data-testid="model-selector-effort-option"`) for supported levels (`Low`, `Medium`, `High`, `Extra`, `Max`). Selecting a level dispatches an asynchronous update to `/api/custom_models/thinking_level` and binds the model configuration to the active project via `/api/custom_models/bind`.
- **Factory Asset Parity**: Submenu rows and parent indicators render with Antigravity 2.0's exact factory vector assets:
  - `CheckIcon`: Factory SVG (`viewBox="0 -960 960 960"`) indicating the active thinking level.
  - `ChevronRightIcon`: Factory SVG path indicating nested submenu availability on compatible models.

### Trigger Label Formatting
When a custom model with an active thinking level is selected, the model selector trigger button updates its label to match native Gemini formatting:
```html
<span class="opacity-70"> High</span>
```
The resulting trigger displays `<Model Name> <Level>` (for example, `Gemini 4 Argon High` or `DeepSeek Flash Medium`), maintaining consistent visual hierarchy across native and external providers.

### Navigation Protection & Lifecycle Cleanup
- **Hover-Tunnel Protection**: A 120ms debounce timer prevents premature submenu destruction when the pointer traverses the boundary between the parent model row and the flyout container.
- **Viewport Collision Flipping**: If the parent menu sits near the right edge of the screen (`menuRect.right + 165 > window.innerWidth`), the flyout automatically flips to render on the left side (`menuRect.left - 162`), preventing off-screen overflow.
- **Event Cleanup**: The submenu closes immediately when the user presses `Escape`, clicks outside the menu popover, selects a native Gemini model, or unmounts the model selector dropdown.
