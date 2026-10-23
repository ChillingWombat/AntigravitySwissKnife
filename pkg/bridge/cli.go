package bridge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	_ "modernc.org/sqlite"
)

// The Antigravity CLI ("agy") runs one non-interactive turn per invocation:
//
//	agy -p "<prompt>" --output-format json [--conversation <id>] [--model <slug>]
//
// and answers with a single JSON object carrying conversation_id, status,
// response and usage. This handler gives the bridge the same surface the desktop
// app path has, minus the UI-only actions (compact is a slash command, stop kills
// the running process). Turns on one conversation are serialised here because the
// CLI itself does not reject concurrent runs on the same conversation.

const (
	cliTurnTimeoutDefault = 180 * time.Second
	cliTurnTimeoutMax     = 900 * time.Second
	cliResponseLimit      = 200 * 1024
	// cliDefaultModelKey stores the model used when a conversation pins none.
	cliDefaultModelKey = "__default__"
)

// cliTurnResult mirrors the CLI's --output-format json payload.
type cliTurnResult struct {
	ConversationID string  `json:"conversation_id"`
	Status         string  `json:"status"`
	Response       string  `json:"response"`
	Error          string  `json:"error"`
	DurationSec    float64 `json:"duration_seconds"`
	NumTurns       int     `json:"num_turns"`
	// Model is the backend model this turn actually used. Not part of the CLI's
	// JSON payload, so it is excluded from (de)serialisation.
	Model string `json:"-"`
	Usage struct {
		InputTokens     int64 `json:"input_tokens"`
		OutputTokens    int64 `json:"output_tokens"`
		ThinkingTokens  int64 `json:"thinking_tokens"`
		CacheReadTokens int64 `json:"cache_read_tokens"`
		TotalTokens     int64 `json:"total_tokens"`
	} `json:"usage"`
}

// cliRun tracks one in-flight CLI turn so stop/status/await can see it.
type cliRun struct {
	conversationID string
	createdConvID  string
	model          string
	startedAt      time.Time
	cmd            *exec.Cmd
	done           chan struct{}
	result         *cliTurnResult
	err            error
}

// CLIHandler drives the Antigravity CLI binary.
type CLIHandler struct {
	mu       sync.Mutex
	cliPath  string
	runs     map[string]*cliRun // conversation id -> in-flight turn
	models   []cliModel
	modelsAt time.Time
	lastUsed map[string]string // conversation id -> model slug last passed
	cwd      string
	// gatewayEnv returns the AGY_LLM_GATEWAY_* environment for a custom model id,
	// or ok=false when the model is not a custom model.
	gatewayEnv func(model string) (map[string]string, string, bool)
}

// cliModel is one entry of `agy models`.
type cliModel struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

// NewCLIHandler creates a CLI handler and resolves the binary once.
func NewCLIHandler() *CLIHandler {
	return &CLIHandler{
		cliPath:  resolveAgyCLIPath(),
		runs:     map[string]*cliRun{},
		lastUsed: map[string]string{},
	}
}

// resolveAgyCLIPath finds the Antigravity CLI binary.
func resolveAgyCLIPath() string {
	if p, err := exec.LookPath("agy"); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, candidate := range []string{
			filepath.Join(home, ".local", "bin", "agy"),
			filepath.Join(home, ".local", "bin", "agy.exe"),
		} {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

// Available reports whether the CLI binary was found.
func (h *CLIHandler) Available() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cliPath != ""
}

// Path returns the resolved binary path.
func (h *CLIHandler) Path() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cliPath
}

// SetPath overrides the binary (tests).
func (h *CLIHandler) SetPath(path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cliPath = path
}

// SetGatewayEnv wires the local LLM gateway: when it recognises a custom model id
// it returns the env vars that point the CLI at the gateway.
func (h *CLIHandler) SetGatewayEnv(provider func(model string) (map[string]string, string, bool)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.gatewayEnv = provider
}

// SetWorkingDir pins the directory every turn runs in. The CLI uses the process
// CWD as the conversation workspace, so it is always set explicitly.
func (h *CLIHandler) SetWorkingDir(dir string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cwd = dir
}

func (h *CLIHandler) workDir() string {
	h.mu.Lock()
	cwd := h.cwd
	h.mu.Unlock()
	if cwd != "" {
		return cwd
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	home, _ := os.UserHomeDir()
	return home
}

// ListModels returns the models `agy models` advertises (cached for a minute).
func (h *CLIHandler) ListModels() ([]cliModel, error) {
	h.mu.Lock()
	if len(h.models) > 0 && time.Since(h.modelsAt) < time.Minute {
		models := append([]cliModel{}, h.models...)
		h.mu.Unlock()
		return models, nil
	}
	h.mu.Unlock()
	bin, err := h.resolveBinary()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "models")
	core.SetHiddenConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("agy models failed: %w", err)
	}
	models := []cliModel{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		slug, label, found := strings.Cut(line, "\t")
		slug = strings.TrimSpace(slug)
		if slug == "" {
			continue
		}
		if !found {
			label = slug
		}
		models = append(models, cliModel{Slug: slug, Label: strings.TrimSpace(label)})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Label < models[j].Label })

	h.mu.Lock()
	h.models = models
	h.modelsAt = time.Now()
	h.mu.Unlock()
	return models, nil
}

// resolveModelSlug maps a label, slug or value to an accepted --model argument.
func (h *CLIHandler) resolveModelSlug(raw string) (string, error) {
	wanted := strings.ToLower(strings.TrimSpace(raw))
	if wanted == "" {
		return "", nil
	}
	models, err := h.ListModels()
	if err != nil {
		return "", err
	}
	for _, m := range models {
		if strings.ToLower(m.Slug) == wanted {
			return m.Slug, nil
		}
	}
	for _, m := range models {
		if strings.ToLower(m.Label) == wanted {
			return m.Slug, nil
		}
	}
	for _, m := range models {
		if strings.Contains(strings.ToLower(m.Label), wanted) {
			return m.Slug, nil
		}
	}
	available := make([]string, 0, len(models))
	for _, m := range models {
		available = append(available, m.Label+" ("+m.Slug+")")
	}
	return "", fmt.Errorf("unknown CLI model %q; available: %s", raw, strings.Join(available, ", "))
}

// validateFlagValue rejects argv values that would be parsed as a CLI flag or
// that contain a NUL byte. The bridge builds argv itself (no shell), so this is
// the only place caller-supplied strings reach the process.
func validateFlagValue(name, value string) error {
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("%s contains a NUL byte", name)
	}
	if strings.HasPrefix(strings.TrimSpace(value), "-") {
		return fmt.Errorf("%s must not start with '-': %q", name, value)
	}
	return nil
}

// resolveBinary returns the verified CLI binary path. The path never comes from
// tool arguments: it is resolved once from PATH or ~/.local/bin.
func (h *CLIHandler) resolveBinary() (string, error) {
	h.mu.Lock()
	bin := h.cliPath
	h.mu.Unlock()
	if bin == "" {
		return "", fmt.Errorf("Antigravity CLI not found: install it or add agy to PATH")
	}
	if !filepath.IsAbs(bin) {
		return "", fmt.Errorf("Antigravity CLI path must be absolute, got %q", bin)
	}
	info, err := os.Stat(bin)
	if err != nil {
		return "", fmt.Errorf("Antigravity CLI not executable: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("Antigravity CLI path points at a directory: %s", bin)
	}
	return bin, nil
}

// runTurn executes one CLI turn and waits for it.
// runTurn keeps the historical signature; it runs in the daemon's working directory.
func (h *CLIHandler) runTurn(prompt, conversationID, model, effort string, timeout time.Duration) (*cliTurnResult, error) {
	return h.runTurnIn(prompt, conversationID, model, effort, "", timeout)
}

// runTurnIn runs one turn in the given directory. cwd is the workspace a caller asked
// for, which is how a channel's default project reaches the CLI.
func (h *CLIHandler) runTurnIn(prompt, conversationID, model, effort, cwd string, timeout time.Duration) (*cliTurnResult, error) {
	bin, err := h.resolveBinary()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	for name, value := range map[string]string{"conversation_id": conversationID, "model": model, "effort": effort} {
		if err := validateFlagValue(name, value); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(model) == "" {
		model = h.modelFor(conversationID)
	}

	// A custom model is served by the local LLM gateway: point the CLI at it
	// instead of resolving a native slug.
	h.mu.Lock()
	gatewayLookup := h.gatewayEnv
	h.mu.Unlock()
	var gatewayEnv map[string]string
	if gatewayLookup != nil {
		if env, canonical, ok := gatewayLookup(model); ok && len(env) > 0 {
			gatewayEnv = env
			// The CLI keys on gateway model ids, not display labels.
			if canonical != "" {
				model = canonical
			}
		}
	}
	if gatewayEnv == nil {
		// Native model: resolve the label against `agy models`.
		resolved, resolveErr := h.resolveModelSlug(model)
		if resolveErr != nil {
			return nil, resolveErr
		}
		model = resolved
	}

	args := []string{"-p", prompt, "--output-format", "json"}
	if conversationID != "" {
		args = append(args, "--conversation", conversationID)
	}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	if strings.TrimSpace(effort) != "" {
		args = append(args, "--effort", effort)
	}
	// Print mode never blocks on stdin, but an explicit approval mode keeps
	// tool use deterministic for an unattended bridge.
	args = append(args, "--dangerously-skip-permissions")

	if timeout <= 0 {
		timeout = cliTurnTimeoutDefault
	}
	if timeout > cliTurnTimeoutMax {
		timeout = cliTurnTimeoutMax
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = h.workDir()
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	if len(gatewayEnv) > 0 {
		env := os.Environ()
		for key, value := range gatewayEnv {
			env = append(env, key+"="+value)
		}
		cmd.Env = env
	}
	core.SetHiddenConsole(cmd)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	run := &cliRun{
		conversationID: conversationID,
		model:          model,
		startedAt:      time.Now(),
		cmd:            cmd,
		done:           make(chan struct{}),
	}
	key := conversationID
	if key == "" {
		key = "__pending__"
	}
	h.mu.Lock()
	if _, busy := h.runs[key]; busy {
		h.mu.Unlock()
		return nil, fmt.Errorf("a turn is already running for conversation %s; wait for it or stop it first", conversationID)
	}
	h.runs[key] = run
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.runs, key)
		if run.createdConvID != "" {
			delete(h.runs, run.createdConvID)
		}
		h.mu.Unlock()
		close(run.done)
	}()

	out, runErr := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		run.err = fmt.Errorf("CLI turn timed out after %s", timeout)
		return nil, run.err
	}

	result := &cliTurnResult{}
	trimmed := strings.TrimSpace(string(out))
	if trimmed != "" {
		// The CLI emits one JSON object. Prefer the whole payload; fall back to the
		// last line that parses so a stray banner or log line cannot break it.
		if jsonErr := json.Unmarshal([]byte(trimmed), result); jsonErr != nil {
			for _, line := range reversedLines(trimmed) {
				if lineErr := json.Unmarshal([]byte(line), result); lineErr == nil {
					break
				}
			}
		}
	}
	if result.Status == "ERROR" || result.Error != "" {
		msg := result.Error
		if msg == "" {
			msg = strings.TrimSpace(stderr.String())
		}
		if msg == "" {
			msg = "CLI reported an error"
		}
		run.err = fmt.Errorf("%s", msg)
		return nil, run.err
	}
	if runErr != nil && result.Response == "" {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = runErr.Error()
		}
		run.err = fmt.Errorf("agy failed: %s", msg)
		return nil, run.err
	}

	result.Model = model
	run.result = result
	if result.ConversationID != "" {
		h.mu.Lock()
		if model != "" {
			h.lastUsed[result.ConversationID] = model
		}
		// Remember the conversation this run created so a follow-up can address it.
		run.createdConvID = result.ConversationID
		h.mu.Unlock()
	}
	return result, nil
}

func reversedLines(s string) []string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// CreateConversation runs a first turn and stores it as the active session.
func (h *CLIHandler) CreateConversation(args map[string]interface{}) (string, error) {
	prompt := argString(args, "prompt", "")
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}
	model := argString(args, "model", "")
	timeout := secondsArg(args, "timeout_seconds", cliTurnTimeoutDefault)
	started := time.Now()
	result, err := h.runTurnIn(prompt, "", model, argString(args, "effort", ""), argString(args, "cwd", ""), timeout)
	if err != nil {
		return "", err
	}
	payload := map[string]interface{}{
		"agent":            "cli",
		"status":           strings.ToLower(result.Status),
		"conversation_id":  result.ConversationID,
		"model":            modelOrUnknown(result.Model),
		"response":         truncateRunes(result.Response, cliResponseLimit),
		"duration_seconds": result.DurationSec,
		"num_turns":        result.NumTurns,
		"usage":            result.Usage,
		"elapsed_seconds":  time.Since(started).Seconds(),
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

// SendMessage runs a follow-up turn on an existing conversation.
func (h *CLIHandler) SendMessage(args map[string]interface{}) (string, error) {
	message := argString(args, "message", "")
	if message == "" {
		return "", fmt.Errorf("message is required")
	}
	convID := argString(args, "conversation_id", "")
	if convID == "" {
		h.mu.Lock()
		for id := range h.lastUsed {
			convID = id
		}
		h.mu.Unlock()
	}
	if convID == "" {
		return "", fmt.Errorf("conversation_id is required: run antigravity_lead_create_conversation first")
	}
	model := argString(args, "model", "")
	timeout := secondsArg(args, "timeout_seconds", cliTurnTimeoutDefault)
	result, err := h.runTurn(message, convID, model, argString(args, "effort", ""), timeout)
	if err != nil {
		return "", err
	}
	payload := map[string]interface{}{
		"agent":           "cli",
		"status":          strings.ToLower(result.Status),
		"conversation_id": result.ConversationID,
		"model":           modelOrUnknown(result.Model),
		"response":        truncateRunes(result.Response, cliResponseLimit),
		"num_turns":       result.NumTurns,
		"usage":           result.Usage,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

// GetStatus reports whether a CLI turn is running.
func (h *CLIHandler) GetStatus(args map[string]interface{}) (string, error) {
	convID := argString(args, "conversation_id", "")
	h.mu.Lock()
	defer h.mu.Unlock()
	payload := map[string]interface{}{"agent": "cli", "status": "idle", "running": false}
	for id, run := range h.runs {
		if convID != "" && id != convID {
			continue
		}
		payload["status"] = "in_progress"
		payload["running"] = true
		payload["conversation_id"] = id
		payload["elapsed_seconds"] = time.Since(run.startedAt).Seconds()
		break
	}
	if convID != "" {
		payload["conversation_id"] = convID
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

// AwaitResponse waits for the in-flight turn on a conversation.
func (h *CLIHandler) AwaitResponse(args map[string]interface{}) (string, error) {
	convID := argString(args, "conversation_id", "")
	timeout := secondsArg(args, "timeout_seconds", 60*time.Second)
	deadline := time.Now().Add(timeout)

	for {
		h.mu.Lock()
		var run *cliRun
		for id, candidate := range h.runs {
			if convID == "" || id == convID {
				run = candidate
				convID = id
				break
			}
		}
		h.mu.Unlock()

		if run == nil {
			return "", fmt.Errorf("no in-flight CLI turn to wait for; the last turn already completed")
		}

		select {
		case <-run.done:
			payload := map[string]interface{}{"agent": "cli", "conversation_id": convID, "status": "completed"}
			if run.result != nil {
				payload["response"] = truncateRunes(run.result.Response, cliResponseLimit)
				payload["usage"] = run.result.Usage
				payload["duration_seconds"] = run.result.DurationSec
			}
			if run.err != nil {
				payload["status"] = "error"
				payload["error"] = run.err.Error()
			}
			data, _ := json.MarshalIndent(payload, "", "  ")
			return string(data), nil
		case <-time.After(250 * time.Millisecond):
			if time.Now().After(deadline) {
				payload := map[string]interface{}{
					"agent":           "cli",
					"conversation_id": convID,
					"status":          "in_progress",
					"elapsed_seconds": time.Since(run.startedAt).Seconds(),
					"waited_seconds":  timeout.Seconds(),
				}
				data, _ := json.MarshalIndent(payload, "", "  ")
				return string(data), nil
			}
		}
	}
}

// StopConversation kills the running CLI turn.
func (h *CLIHandler) StopConversation(args map[string]interface{}) (string, error) {
	convID := argString(args, "conversation_id", "")
	h.mu.Lock()
	var run *cliRun
	var id string
	for candidateID, candidate := range h.runs {
		if convID == "" || candidateID == convID {
			run = candidate
			id = candidateID
			break
		}
	}
	h.mu.Unlock()
	if run == nil || run.cmd == nil || run.cmd.Process == nil {
		return "", fmt.Errorf("no in-flight CLI turn found")
	}
	if err := run.cmd.Process.Kill(); err != nil {
		return "", fmt.Errorf("stop CLI turn: %w", err)
	}
	payload := map[string]interface{}{"agent": "cli", "status": "stopped", "conversation_id": id}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

// CompactConversation asks the CLI to compact a conversation via its slash command.
func (h *CLIHandler) CompactConversation(args map[string]interface{}) (string, error) {
	convID := argString(args, "conversation_id", "")
	if convID == "" {
		return "", fmt.Errorf("conversation_id is required for CLI compaction")
	}
	result, err := h.runTurn("/compact", convID, argString(args, "model", ""), "", cliTurnTimeoutDefault)
	if err != nil {
		return "", err
	}
	payload := map[string]interface{}{
		"agent":           "cli",
		"status":          "compacted",
		"conversation_id": result.ConversationID,
		"response":        truncateRunes(result.Response, 2000),
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

// ListConversations reads the CLI's conversation index.
func (h *CLIHandler) ListConversations(args map[string]interface{}) (string, error) {
	limit := 10
	if v, ok := args["limit"].(float64); ok && int(v) > 0 {
		limit = int(v)
	}
	if limit > 100 {
		limit = 100
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	dbPath := filepath.Join(home, ".gemini", "antigravity-cli", "conversation_summaries.db")
	if _, statErr := os.Stat(dbPath); statErr != nil {
		return "", fmt.Errorf("CLI conversation index not found at %s: run a CLI turn first", dbPath)
	}
	db, openErr := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if openErr != nil {
		return "", fmt.Errorf("open CLI conversation index: %w", openErr)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT conversation_id, COALESCE(title, ''), COALESCE(preview, ''), COALESCE(step_count, 0), COALESCE(last_modified_time, 0)
		FROM conversation_summaries ORDER BY last_modified_time DESC LIMIT ?`, limit)
	if err != nil {
		return "", fmt.Errorf("query CLI conversation index: %w", err)
	}
	defer rows.Close()

	type entry struct {
		ConversationID string `json:"conversation_id"`
		Title          string `json:"title"`
		Preview        string `json:"preview,omitempty"`
		StepCount      int    `json:"step_count"`
		LastModified   string `json:"last_modified"`
	}
	entries := []entry{}
	for rows.Next() {
		var e entry
		// last_modified_time is stored as an RFC3339-ish string, not a number.
		if scanErr := rows.Scan(&e.ConversationID, &e.Title, &e.Preview, &e.StepCount, &e.LastModified); scanErr == nil {
			entries = append(entries, e)
		} else {
			return "", fmt.Errorf("read CLI conversation row: %w", scanErr)
		}
	}
	payload := map[string]interface{}{
		"agent":         "cli",
		"status":        "ok",
		"count":         len(entries),
		"conversations": entries,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

// setModel records the model the next turn for a conversation should pass. An
// empty conversation id sets the default for conversations that do not pin one.
func (h *CLIHandler) setModel(conversationID, slug string) error {
	key := conversationID
	if key == "" {
		key = cliDefaultModelKey
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if strings.TrimSpace(slug) == "" {
		delete(h.lastUsed, key)
		return nil
	}
	h.lastUsed[key] = slug
	return nil
}

// modelFor returns the model to pass for a conversation, honouring setModel.
func (h *CLIHandler) modelFor(conversationID string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conversationID != "" {
		if model, ok := h.lastUsed[conversationID]; ok {
			return model
		}
	}
	return h.lastUsed[cliDefaultModelKey]
}

// Models exposes the CLI model list for the unified model tools.
func (h *CLIHandler) Models() ([]cliModel, error) { return h.ListModels() }

// CurrentModel reports the model last passed for a conversation.
func (h *CLIHandler) CurrentModel(conversationID string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastUsed[conversationID]
}

func secondsArg(args map[string]interface{}, key string, fallback time.Duration) time.Duration {
	if v, ok := args[key].(float64); ok && v > 0 {
		return time.Duration(v * float64(time.Second))
	}
	return fallback
}

func modelOrUnknown(model string) string {
	if strings.TrimSpace(model) == "" {
		return "default"
	}
	return model
}
