# Research Report: Antigravity Gateway Mode Model Picker Root-Cause Analysis

**Target File**: `/opt/Antigravity/resources/bin/language_server`  
**Webview Asset**: `main.js` (embedded zip in `language_server` at offset `0x916f9f0`)  
**VS Code Extension**: `google.google-antigravity-1.9.0`  
**Date**: 2026-10-23  

---

## Executive Summary

When running Antigravity in gateway mode (`AGY_LLM_GATEWAY_URL`, `AGY_LLM_GATEWAY_API_KEY`, `AGY_LLM_GATEWAY_MODELS`), the model chip in the chat input bar successfully displays the gateway model ID, and chat turns execute correctly through the gateway. However, clicking the model chip reveals a dropdown menu rendering `"No models available"`.

The root cause is a **direct compile-time naming mismatch between the Go backend and the React webview frontend**:

1. In gateway mode, `language_server` builds a model sort object hardcoded with `name: "Enterprise Gateway Models"`.
2. The React webview's model selector hook (`k5b` in `main.js:11735`) hardcodes a filter looking exclusively for `c.name.toLowerCase() === "recommended"`.
3. Because `"Enterprise Gateway Models"` does not match `"recommended"`, `k5b` discards all options and returns an empty list (`[]`).
4. The empty options list triggers the dropdown empty state: `<p data-testid="model-selector-empty">No models available</p>`.
5. The model chip itself avoids this filter because `gMa()` resolves the active selection using `modelSorts.at(0)?.groups.at(0)?.options.at(0)`, which grabs the first gateway model regardless of the sort name.

Pure environment variable configuration **cannot** resolve this under `AGY_LLM_GATEWAY_URL` because the sort name `"Enterprise Gateway Models"` is a compiled static string in `language_server`. However, the issue can be resolved via:
- A **5-byte binary patch** in `language_server` redirecting the sort name pointer to the existing `"Recommended"` string literal.
- A **frontend patch** in `main.js` relaxing `k5b` to accept `a[0]` or `"enterprise gateway models"`.
- An **environment proxy pattern** setting `CLOUD_CODE_URL` instead of `AGY_LLM_GATEWAY_URL`.

---

## 1. What Feeds the Model Picker in Gateway Mode

### 1.1 Language Server Gateway Discovery Pipeline

In `/opt/Antigravity/resources/bin/language_server` (ELF 64-bit x86-64, Go 1.20+):

1. **Authentication Mode Branching**:
   - Location: `google3/third_party/jetski/language_server/code_assist_client/codeassistclient.(*CodeAssistClient).fetchAvailableModels` (`0x6625f2f` - `0x6626480`).
   - String check at `0x6625fb8` - `0x6625fca`:
     - Compares auth mode string against `"gemini_api_key"` (len 14, `0x6625f74`) and `"gateway"` (len 7, `0x6625fb2`).
   - When `AGY_LLM_GATEWAY_URL` is set, auth mode is `"gateway"`.
   - At `0x662644a`, `fetchAvailableModels` calls `google3/third_party/jetski/language_server/code_assist_client/codeassistclient.buildGatewayAvailableModels` (`0x66244c0` - `0x6625ea0`).

2. **Environment Variable Ingestion in `buildGatewayAvailableModels`**:
   `buildGatewayAvailableModels` makes exactly four calls to `os.Getenv` (`0x4aeda80`):
   - `0x66245fa`: `os.Getenv("AGY_LLM_GATEWAY_MODELS")` (string at `0x41e18ce`).
     - Split with comma delimiter `b","` (at `0x479c870`) via `strings.genSplit` (`0x4b03860`).
     - Each item is trimmed via `strings.TrimSpace` (`0x4b06c80`).
     - Items are deduped into a map and collected into model lists.
   - `0x6625400`: `os.Getenv("AGY_LLM_GATEWAY_TIER_FLASH_LITE")` (string at `0x424becd`).
   - `0x66254b3`: `os.Getenv("AGY_LLM_GATEWAY_TIER_FLASH")` (string at `0x421bf8e`).
   - `0x66255a4`: `os.Getenv("AGY_LLM_GATEWAY_TIER_PRO")` (string at `0x4204910`).
   - If tier variables are omitted, it calls `google3/third_party/jetski/gateway/gateway.InferTieredModelConfig` (`0x6dcc9a0`) to heuristically assign flash, flash-lite, and pro tiers from the model names.

3. **`FetchAvailableModelsResponse` Construction**:
   `buildGatewayAvailableModels` constructs a `FetchAvailableModelsResponse` protobuf message:
   - `Models`: Map of model configs created for each model in `AGY_LLM_GATEWAY_MODELS`.
   - `TieredModelIds`: Populated with `flash_lite`, `flash`, and `pro`.
   - `DefaultAgentModelId`: Set to the resolved default model.
   - `AgentModelSorts`: Constructed at `0x662526d` - `0x662527c`:
     ```assembly
     662526d: movq $0x19, 0x10(%rax)        # Length 25 (0x19)
     6625275: lea  -0x2414cc9(%rip), %rdx   # 0x42105b3 -> "Enterprise Gateway Models"
     662527c: mov  %rdx, 0x8(%rax)          # AgentModelSort.Name
     ...
     66252b4: movq $0x7, 0x10(%rax)         # Length 7
     66252bc: lea  -0x2563ad3(%rip), %rdx   # 0x40c17f0 -> "Gateway"
     66252c3: mov  %rdx, 0x8(%rax)          # Group.GroupName
     ```

   In contrast, the standard Google Cloud Code path (`buildDefaultAvailableModels` at `0x6623ca0`) writes:
   ```assembly
   6624057: movq $0xb, 0x10(%rax)          # Length 11 (0x0b)
   662405f: lea  -0x25087ae(%rip), %rdx   # 0x411b8b8 -> "Recommended"
   6624066: mov  %rdx, 0x8(%rax)          # AgentModelSort.Name = "Recommended"
   ```

4. **RPC Propagation to Frontend**:
   - `google3/third_party/jetski/language_server/code_assist_client/codeassistclient.processModelSorts` (`0x6627ec0`) maps `AgentModelSorts` into `ClientModelSorts` within `CascadeModelConfigData`.
   - The language server serves `CascadeModelConfigData` via ConnectRPC method `LanguageServerService/GetUserStatus` (handled by `language_server.(*Server).GetUserStatus` at `0x725dec0`).
   - The webview runs a polling loop in `[UserStatusSync]` (`main.js:13917`) calling `e.getUserStatus()` every 10 minutes (and on mount), updating the React root state with `userStatus.cascadeModelConfigData`.

---

### 1.2 Webview Frontend Handling

The webview assets (`index.html`, `main.js`, CSS) are served directly by `language_server`'s embedded HTTP bundle server (`bundleAssetServer.ServeHTTP` at `0x726c980`, reading from the embedded zip at file offset `0x916f9f0`). Both the Antigravity desktop app and the VS Code extension (`extension.js:27420` `renderJetskiIframe`) load this identical webview.

In `main.js`:

1. **State Conversion in `gMa` / `pmc` (`main.js:2804248` & `13075`)**:
   ```javascript
   // Parse individual model configurations
   v = (0, A.useMemo)(() => [
     ...(b?.cascadeModelConfigData?.clientModelConfigs || [])
       .map(Ck)
       .filter(J => J !== null)
       .filter(J => !J.disabled)
   ], [b?.cascadeModelConfigData]);

   // Build sorted groups:
   x = (0, A.useMemo)(() => {
     var J = b?.cascadeModelConfigData,
         L = J?.clientModelConfigs || [],
         N = J?.clientModelSorts || [];
     J = L.map(Ck).filter(O => O !== null).filter(O => !O.disabled);
     L = dma(L, N);
     return L.length > 0 ? L : cma(J);
   }, [b?.cascadeModelConfigData]);
   ```
   `dma(clientModelConfigs, clientModelSorts)` (`main.js:1870125`) iterates through `clientModelSorts` (`N`), matching model labels to parsed configs:
   - Output `x` is:
     `[{ name: "Enterprise Gateway Models", groups: [{ groupName: "Gateway", options: [model1, model2, ...] }] }]`.

2. **Why the Model Chip Shows the Gateway Model ID**:
   `currentModelConfig` is resolved by selector `y(c, l)` (`main.js:2804248`):
   ```javascript
   y = (0, A.useCallback)((J, L) => {
     if (v.length !== 0 || u.length !== 0) {
       ...
       return J !== void 0 && J !== I.Model.GOOGLE_GEMINI_INTERNAL_BYOM && (J = Dk(J, v))
         ? J
         : x.at(0)?.groups.at(0)?.options.at(0) ?? u.at(0);
     }
   }, [v, u, x]);
   ```
   Since `x.at(0)` is `"Enterprise Gateway Models"`, `x.at(0).groups.at(0).options.at(0)` extracts the first gateway model. The trigger chip component `p5b` (`main.js:11735`) renders `selectedModel.label`, successfully printing the gateway model ID on the chip.

3. **Why the Dropdown Renders "No models available"**:
   When the dropdown is opened, `o5b` invokes `l5b(a.modelSorts)` -> `k5b(a.modelSorts)` (`main.js:11735`):
   ```javascript
   const k5b = a => (0, A.useMemo)(() => {
     var b = a.find(c => c.name.toLowerCase() === "recommended");
     return b && b.groups?.length
       ? { groupName: "Recommended", options: b.groups[0]?.options || [] }
       : { groupName: "Recommended", options: [] };
   }, [a]);
   ```
   `k5b` strictly filters for `c.name.toLowerCase() === "recommended"`.
   - `a` contains only `{ name: "Enterprise Gateway Models", groups: [...] }`.
   - `a.find(...)` returns `undefined`.
   - `k5b` returns `{ groupName: "Recommended", options: [] }`.
   - The dropdown list component `y5b` (`main.js:11760`) checks:
     ```javascript
     q = a.some(r => r.options.length > 0);
     ...
     q ? a.map(r => r.options.length > 0 && A.createElement(x5b, { key: r.groupName, group: r }))
       : A.createElement("p", { "data-testid": "model-selector-empty", className: "text-xs opacity-50 px-2 py-1" }, "No models available")
     ```
   - Since `options` is empty, `q` evaluates to `false`.
   - The UI unconditionally renders `<p data-testid="model-selector-empty">No models available</p>`.

---

## 2. Feasibility of Pure Environment Variable Configuration

### Verdict: **Impossible under pure `AGY_LLM_GATEWAY_URL` mode alone**

Disassembly of `buildGatewayAvailableModels` (`0x66244c0` - `0x6625ea0`) proves:
- Only four environment variables are checked: `AGY_LLM_GATEWAY_MODELS`, `AGY_LLM_GATEWAY_TIER_FLASH_LITE`, `AGY_LLM_GATEWAY_TIER_FLASH`, and `AGY_LLM_GATEWAY_TIER_PRO`.
- The sort name `"Enterprise Gateway Models"` is passed via immediate RIP-relative pointer `0x42105b3` and immediate length `0x19` (25). No environment variable, flag, or configuration file can override this sort name.
- Setting `AGY_LLM_GATEWAY_TIER_*` only populates the internal `TieredModelIds` protobuf message (used for tier-based subagent routing), but has zero effect on `AgentModelSorts.Name`.
- If `clientModelSorts` is empty, the frontend's built-in fallback `cma(B)` creates `{ name: "All", groups: ... }`, which also fails `k5b`'s `"recommended"` filter.

---

## 3. Concrete Solutions & Smallest Patches

### Solution A: The 5-Byte Binary Patch in `language_server` (Recommended for Gateway Mode)

Change the compiled sort name in `buildGatewayAvailableModels` from `"Enterprise Gateway Models"` to `"Recommended"`.

- **Target File**: `/opt/Antigravity/resources/bin/language_server`
- **Virtual Address**: `0x662526d`
- **File Offset**: `0x662526d` (Segment 01 LOAD R E, offset == vaddr)

#### Assembly Before & After:
```assembly
# BEFORE
0x662526d: 48 c7 40 10 19 00 00 00   movq $0x19, 0x10(%rax)       # Length 25
0x6625275: 48 8d 15 37 b3 be fd      lea  -0x2414cc9(%rip), %rdx  # Points to 0x42105b3 ("Enterprise Gateway Models")

# AFTER
0x662526d: 48 c7 40 10 0b 00 00 00   movq $0x0b, 0x10(%rax)       # Length 11
0x6625275: 48 8d 15 3c 66 af fd      lea  -0x25099c4(%rip), %rdx  # Points to 0x411b8b8 ("Recommended")
```

#### Exact Byte Patch:
- At file offset `0x6625271`: `0x19` -> `0x0b` (1 byte)
- At file offset `0x6625278`: `37 b3 be fd` -> `3c 66 af fd` (4 bytes)

#### Python Patch Script:
```python
with open("/opt/Antigravity/resources/bin/language_server", "r+b") as f:
    f.seek(0x662526d)
    chunk = f.read(15)
    assert chunk == bytes.fromhex("48c7401019000000488d1537b3befd"), f"Unexpected bytes: {chunk.hex()}"
    f.seek(0x662526d)
    f.write(bytes.fromhex("48c740100b000000488d153c66affd"))
print("Successfully patched language_server: sort name is now 'Recommended'")
```

With this patch, `buildGatewayAvailableModels` outputs `AgentModelSorts[0].Name = "Recommended"`. When `main.js:k5b` runs, `a.find(c => c.name.toLowerCase() === "recommended")` succeeds, and all models listed in `AGY_LLM_GATEWAY_MODELS` render in the dropdown menu.

---

### Solution B: Webview JavaScript Patch (Non-Binary Alternative)

Modify `main.js` inside the web bundle served by `language_server` (or via MITM proxy / unpacked bundle):

- **Target File**: `main.js` (inside embedded zip at file offset `0x916f9f0`, or served directory)
- **Target Location**: Function `k5b` (`main.js:11735`)
- **Original Code**:
  ```javascript
  const k5b = a => (0, A.useMemo)(() => {
    var b = a.find(c => c.name.toLowerCase() === "recommended");
    return b && b.groups?.length
      ? { groupName: "Recommended", options: b.groups[0]?.options || [] }
      : { groupName: "Recommended", options: [] };
  }, [a]);
  ```
- **Patched Code**:
  ```javascript
  const k5b = a => (0, A.useMemo)(() => {
    var b = a.find(c => c.name.toLowerCase() === "recommended" || c.name.toLowerCase() === "enterprise gateway models") || a[0];
    return b && b.groups?.length
      ? { groupName: b.name, options: b.groups[0]?.options || [] }
      : { groupName: "Recommended", options: [] };
  }, [a]);
  ```

---

### Solution C: Architecture-Level Alternative via `CLOUD_CODE_URL` (Zero Patches)

If binary modification is undesirable, Antigravity Swiss Knife can supply custom models by avoiding the `AGY_LLM_GATEWAY_URL` gateway path entirely and using the Cloud Code proxy seam:

1. Do **not** set `AGY_LLM_GATEWAY_URL`.
2. Launch the desktop app or VS Code extension with:
   ```bash
   CLOUD_CODE_URL="http://127.0.0.1:8765"
   ```
3. The Swiss Knife daemon at `127.0.0.1:8765` intercepts `POST /v1internal:fetchAvailableModels` and returns:
   ```json
   {
     "models": {
       "deepseek-chat": {
         "displayName": "DeepSeek V3"
       },
       "deepseek-reasoner": {
         "displayName": "DeepSeek R1"
       }
     },
     "agentModelSorts": [
       {
         "name": "Recommended",
         "groups": [
           {
             "groupName": "Custom Models",
             "modelLabels": [
               "DeepSeek V3",
               "DeepSeek R1"
             ]
           }
         ]
       }
     ],
     "defaultAgentModelId": "deepseek-chat"
   }
   ```
4. Because the sort name returned by the proxy is `"Recommended"`, `k5b` accepts it without any binary or frontend modifications.
