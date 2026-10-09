# Active Conversation Continuation & Subagent Revival

When switching accounts or restarting Antigravity 2.0 to apply configuration changes, running developer conversations and subagent execution workflows were historically severed. The developer had to manually re-navigate to the previous conversation, inspect lost progress, and re-issue prompts.

The `pkg/revival` subsystem eliminates this disruption by capturing execution state pre-switch and automatically resuming conversations and subagents post-relaunch.

---

## 1. Problem Statement

Rotating credentials while an autonomous agent or subagent is running presents three technical failure modes:
1. **Context Severance**: The IDE reloads or restarts into an empty default workbench (`/`), losing the active thread URL (`/c/<conversationId>`).
2. **Subagent Task Termination**: Background subagent workers executing multi-file edits or terminal commands are aborted mid-turn when process credentials change.
3. **Manual Re-prompting Friction**: The user must inspect whether the previous subagent succeeded, paste recent logs, and instruct the agent to continue.

`pkg/revival` solves this with an atomic state machine:

```
[Pre-Switch Phase]
1. SessionDetector queries active IDE via CDP / Fiber tree
2. Extracts Conversation ID, Subagent ID, active files, pending goal
3. Persists RevivalIntent to disk (90-second TTL)
4. Pins active layout in app_storage.json

[Switch & Restart Phase]
5. Credentials and hardware profiles are swapped atomically
6. Host IDE process terminates cleanly and restarts

[Post-Relaunch Phase]
7. SessionDetector polls CDP until IDE workbench and agent webview are ready
8. Verifies RevivalIntent has not expired (TTL < 90s, retries < 2)
9. Injects continuation trigger into active webview context
10. RevivalIntent marked completed and cleared from disk
```

---

## 2. Pre-Switch State Capture: `SessionDetector`

Before the daemon initiates an account switch or triggers an IDE reload, `SessionDetector` inspects the host runtime:

- **Active Conversation ID (`cascadeId`)**: Retrieved from the React Fiber state tree of the chat pane or by querying active webview URL routes.
- **Active Subagent Status**: Checks whether any child agent task is in `running` or `waiting_tool` status.
- **Active Task Context**: Captures the current subagent objective, recently executed tool actions, and pending instructions.
- **Layout Route**: Records the precise view state (e.g. `/c/e32ab3c3-b41d-426e-896a-df3feeb9821e`).

If no conversation is actively in flight, the revival cycle is safely skipped.

---

## 3. Revival Intent Lifecycle & Store

Captured state is serialized into a `RevivalIntent` struct and persisted to disk via `pkg/revival/store.go`:

```go
type RevivalIntent struct {
    ID             string    `json:"id"`
    ConversationID string    `json:"conversation_id"`
    SubagentID     string    `json:"subagent_id,omitempty"`
    Prompt         string    `json:"prompt,omitempty"`
    TargetApp      string    `json:"target_app"`
    CreatedAt      time.Time `json:"created_at"`
    ExpiresAt      time.Time `json:"expires_at"`
    RetryCount     int       `json:"retry_count"`
    Status         string    `json:"status"` // pending, in_progress, completed, expired
}
```

### 3.1 Strict Safety Parameters
- **90-Second Time-To-Live (TTL)**: A `RevivalIntent` expires 90 seconds after creation. If the host IDE takes longer than 90 seconds to restart (e.g. user manually postpones startup), the intent is discarded to prevent stale, unexpected prompt injections.
- **2 Retry Limit**: The daemon attempts continuation injection a maximum of 2 times. If the webview fails to acknowledge the injection or throws an error, the intent is abandoned to prevent infinite retry loops.
- **Atomic Disk Serialization**: The intent store writes to a dedicated JSON file in the application state directory, using atomic file renames (`rename` syscall) to prevent partial reads.

---

## 4. Layout Pinning via `app_storage.json`

When Antigravity boots, its window manager inspects `app_storage.json` to decide which conversation route to restore.

To prevent the IDE from reverting to an empty landing screen:
1. `pkg/revival` parses `app_storage.json`.
2. Locates the active route configuration key (`lastActiveConversation` or `activeWorkspaceRoute`).
3. Updates the route to `/c/<conversationId>` corresponding to the captured intent.
4. Atomically flushes the file before the new IDE process launches.

This guarantees that the moment the IDE window renders, the exact previous conversation thread is already visible on screen.

---

## 5. Post-Relaunch CDP Prompt Injection

Once the restarted IDE opens:
1. The daemon's background monitor detects the new process and connects to its CDP debugging port.
2. Polls the webview DOM until the conversation input container and subagent execution status elements are fully mounted.
3. Injects a continuation event into the webview execution context:
   - For interactive conversations: signals the agent to resume execution from the last pending step with full knowledge of the account refresh.
   - For autonomous subagents: triggers the next task evaluation turn without requiring manual user intervention.
4. Increments `RetryCount` upon dispatch and marks the intent `completed` once the webview acknowledges receipt.

---

## 6. Verification and Stress Testing

The revival pipeline is tested against adverse conditions in `pkg/revival/revival_test.go` and `pkg/revival/stress_test.go`:
- **Expired Intent Rejection**: Confirms intents older than 90 seconds are ignored without injecting prompts.
- **Max Retry Eviction**: Confirms intents with `RetryCount >= 2` transition to `failed` and do not recur.
- **Corrupted Store Recovery**: Verifies malformed JSON on disk is quarantined without crashing the daemon.
- **Rapid Switch Resiliency**: Validates back-to-back account switches correctly cancel previous pending intents and retain only the latest valid state.
