package bridge

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNormalizeAgentBackend(t *testing.T) {
	cases := map[string]string{
		"":                AgentAuto,
		"auto":            AgentAuto,
		"Automatic":       AgentAuto,
		"app":             AgentApp,
		"GUI":             AgentApp,
		"desktop":         AgentApp,
		"antigravity-2":   AgentApp,
		"cli":             AgentCLI,
		"agy":             AgentCLI,
		"antigravity_cli": AgentCLI,
		"nonsense":        "",
	}
	for in, want := range cases {
		if got := normalizeAgentBackend(in); got != want {
			t.Errorf("normalizeAgentBackend(%q) = %q, want %q", in, got, want)
		}
	}
}

// writeFakeCLI installs a stand-in `agy` that answers the two shapes the bridge
// parses: `models` and a print turn with --output-format json.
func writeFakeCLI(t *testing.T, conversationID string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agy")
	script := `#!/bin/sh
if [ "$1" = "models" ]; then
  printf 'gemini-4-argon-high\tGemini 4 Argon (High)\ngemini-3.8-flash-low\tGemini 3.8 Flash (Low)\n'
  exit 0
fi
printf '{"conversation_id":"` + conversationID + `","status":"SUCCESS","response":"pong","duration_seconds":1.5,"num_turns":1,"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}\n'
`
	if runtime.GOOS == "windows" {
		path = filepath.Join(dir, "agy.cmd")
		script = "@echo off\r\n" +
			"if \"%~1\"==\"models\" goto models\r\n" +
			"echo {\"conversation_id\":\"" + conversationID + "\",\"status\":\"SUCCESS\",\"response\":\"pong\",\"duration_seconds\":1.5,\"num_turns\":1,\"usage\":{\"input_tokens\":10,\"output_tokens\":2,\"total_tokens\":12}}\r\n" +
			"goto :eof\r\n" +
			":models\r\n" +
			"echo gemini-4-argon-high\tGemini 4 Argon (High)\r\n" +
			"echo gemini-3.8-flash-low\tGemini 3.8 Flash (Low)\r\n" +
			"exit /b 0\r\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake cli: %v", err)
	}
	return path
}

func TestCLIHandler_CreateConversationParsesResult(t *testing.T) {
	h := NewCLIHandler()
	h.SetPath(writeFakeCLI(t, "11111111-2222-3333-4444-555555555555"))

	out, err := h.CreateConversation(map[string]interface{}{"prompt": "say pong"})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if payload["agent"] != "cli" {
		t.Errorf("expected agent cli, got %v", payload["agent"])
	}
	if payload["conversation_id"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("expected the CLI conversation id, got %v", payload["conversation_id"])
	}
	if !strings.Contains(payload["response"].(string), "pong") {
		t.Errorf("expected the response text, got %v", payload["response"])
	}
	if payload["status"] != "success" {
		t.Errorf("expected a lowercased success status, got %v", payload["status"])
	}
}

func TestCLIHandler_ModelSelection(t *testing.T) {
	h := NewCLIHandler()
	h.SetPath(writeFakeCLI(t, "conv-1"))

	models, err := h.ListModels()
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 || models[0].Slug == "" || models[0].Label == "" {
		t.Fatalf("expected the parsed models table, got %+v", models)
	}

	slug, err := h.resolveModelSlug("Gemini 3.8 Flash (Low)")
	if err != nil || slug != "gemini-3.8-flash-low" {
		t.Fatalf("label must resolve to its slug, got %q / %v", slug, err)
	}
	slug, err = h.resolveModelSlug("gemini-4-argon-high")
	if err != nil || slug != "gemini-4-argon-high" {
		t.Fatalf("slug must resolve to itself, got %q / %v", slug, err)
	}
	if _, err := h.resolveModelSlug("no-such-model"); err == nil {
		t.Error("an unknown model must be rejected with the available list")
	}

	// A stored model applies to the next turn of that conversation.
	if err := h.setModel("conv-1", "gemini-3.8-flash-low"); err != nil {
		t.Fatalf("setModel: %v", err)
	}
	if got := h.modelFor("conv-1"); got != "gemini-3.8-flash-low" {
		t.Errorf("expected the stored model, got %q", got)
	}
	if err := h.setModel("", "gemini-4-argon-high"); err != nil {
		t.Fatalf("setModel default: %v", err)
	}
	if got := h.modelFor("some-other-conv"); got != "gemini-4-argon-high" {
		t.Errorf("expected the default model to apply, got %q", got)
	}
}

func TestCLIHandler_RejectsConcurrentTurnOnSameConversation(t *testing.T) {
	h := NewCLIHandler()
	// A CLI that blocks until the test releases it.
	dir := t.TempDir()
	release := filepath.Join(dir, "release")
	path := filepath.Join(dir, "agy")
	script := `#!/bin/sh
if [ "$1" = "models" ]; then printf 'm-1\tM One\n'; exit 0; fi
while [ ! -f "` + release + `" ]; do sleep 0.05; done
printf '{"conversation_id":"conv-busy","status":"SUCCESS","response":"done"}\n'
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	h.SetPath(path)

	done := make(chan struct{})
	go func() {
		_, _ = h.SendMessage(map[string]interface{}{"message": "first", "conversation_id": "conv-busy"})
		close(done)
	}()

	// Wait until the run is registered.
	deadline := 0
	for {
		h.mu.Lock()
		busy := len(h.runs) > 0
		h.mu.Unlock()
		if busy || deadline > 200 {
			break
		}
		deadline++
		time.Sleep(20 * time.Millisecond)
	}

	if _, err := h.SendMessage(map[string]interface{}{"message": "second", "conversation_id": "conv-busy"}); err == nil {
		t.Error("a second turn on the same conversation must be refused while one is running")
	}

	_ = os.WriteFile(release, []byte("go"), 0644)
	<-done
}

func TestResolvedAgentPreference(t *testing.T) {
	svc := newTestBridge(t)

	// No CLI available: auto resolves to the app.
	svc.cliHdl = NewCLIHandler()
	svc.cliHdl.SetPath("")
	if got := svc.ResolvedAgent(); got != AgentApp {
		t.Errorf("without a CLI, auto must resolve to app, got %q", got)
	}

	// CLI available: auto prefers it (cheaper than the GUI).
	svc.cliHdl.SetPath(writeFakeCLI(t, "conv-auto"))
	if got := svc.ResolvedAgent(); got != AgentCLI {
		t.Errorf("with a CLI installed, auto must prefer cli, got %q", got)
	}

	// Explicit app wins even when the CLI exists.
	if _, err := svc.SetAgentBackend(AgentApp); err != nil {
		t.Fatalf("SetAgentBackend(app): %v", err)
	}
	if got := svc.ResolvedAgent(); got != AgentApp {
		t.Errorf("app must be honoured, got %q", got)
	}

	// Explicit cli is honoured.
	if _, err := svc.SetAgentBackend(AgentCLI); err != nil {
		t.Fatalf("SetAgentBackend(cli): %v", err)
	}
	if got := svc.ResolvedAgent(); got != AgentCLI {
		t.Errorf("cli must be honoured, got %q", got)
	}

	// Unknown values are rejected.
	if _, err := svc.SetAgentBackend("teleport"); err == nil {
		t.Error("an unknown backend must be rejected")
	}

	// Selecting the CLI without a binary is refused instead of silently ignored.
	svc.cliHdl.SetPath("")
	if _, err := svc.SetAgentBackend(AgentCLI); err == nil {
		t.Error("selecting cli without the binary must fail with a clear error")
	}
}

func TestUnifiedModelSurfaceIncludesCustomModels(t *testing.T) {
	svc := newTestBridge(t)
	svc.cliHdl.SetPath(writeFakeCLI(t, "conv-models"))
	svc.SetCustomModelSources(func() []CustomModelRef {
		return []CustomModelRef{{ID: "deepseek-v4", Name: "deepseek-v4.1-flash", Label: "DeepSeek V4.1 Flash", Provider: "openai"}}
	}, nil)

	svc.SetGatewayTokenSource(func() string { return "gw-token" })

	// CLI list: native options plus custom ones, both selectable now that the
	// local LLM gateway serves custom models to the CLI.
	out, err := svc.SessionModelListCLI(map[string]interface{}{})
	if err != nil {
		t.Fatalf("SessionModelListCLI: %v", err)
	}
	var payload struct {
		Options []struct {
			Kind              string `json:"kind"`
			ID                string `json:"id"`
			Agent             string `json:"agent"`
			UnavailableReason string `json:"unavailable_reason"`
		} `json:"options"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var sawNative, sawCustom bool
	for _, option := range payload.Options {
		switch option.Kind {
		case "native":
			sawNative = true
		case "custom":
			sawCustom = true
			if option.UnavailableReason != "" {
				t.Errorf("custom models are gateway-served and must be available, got %q", option.UnavailableReason)
			}
		}
	}
	if !sawNative || !sawCustom {
		t.Fatalf("expected native and custom entries in one list, got %s", out)
	}

	// Selecting a custom model on the CLI backend is accepted and remembered: the
	// next turn is pointed at the local gateway.
	setOut, err := svc.SessionModelSetCLI(map[string]interface{}{"model": "DeepSeek V4.1 Flash", "conversation_id": "conv-custom"})
	if err != nil {
		t.Fatalf("custom model must be selectable on the CLI backend: %v", err)
	}
	if !strings.Contains(setOut, "deepseek-v4") || !strings.Contains(setOut, "custom") {
		t.Errorf("expected the custom model id in the result, got %s", setOut)
	}
	if got := svc.cliHdl.CurrentModel("conv-custom"); got != "deepseek-v4" {
		t.Errorf("expected the custom model to be remembered, got %q", got)
	}

	// Without a gateway token the same call must fail loudly instead of running a
	// native model by accident.
	svc.SetGatewayTokenSource(func() string { return "" })
	if _, err := svc.SessionModelSetCLI(map[string]interface{}{"model": "DeepSeek Flash"}); err == nil {
		t.Error("a custom model without a gateway token must be refused")
	}

	// A native model is accepted and remembered.
	out, err = svc.SessionModelSetCLI(map[string]interface{}{"model": "Gemini 3.8 Flash (Low)", "conversation_id": "conv-models"})
	if err != nil {
		t.Fatalf("SessionModelSetCLI: %v", err)
	}
	if !strings.Contains(out, "gemini-3.8-flash-low") {
		t.Errorf("expected the resolved slug in the result, got %s", out)
	}
	if got := svc.cliHdl.CurrentModel("conv-models"); got != "gemini-3.8-flash-low" {
		t.Errorf("expected the model to be remembered, got %q", got)
	}
}

func TestToolManifestExposesAgentBackendTool(t *testing.T) {
	svc := newTestBridge(t)
	names := map[string]bool{}
	for _, tool := range svc.ToolManifest() {
		if name, ok := tool["name"].(string); ok {
			names[name] = true
		}
	}
	if !names["antigravity_bridge_set_agent"] {
		t.Error("the backend selector tool must be advertised")
	}
	allowed := false
	for _, name := range svc.GetConfig().AllowedTools {
		if name == "antigravity_bridge_set_agent" {
			allowed = true
		}
	}
	if !allowed {
		t.Error("the backend selector tool must be in allowed_tools")
	}
}

// Regression: a completed turn must not leave a stale "running" marker, or every
// later turn on that conversation is refused as concurrent.
func TestCLIHandler_SequentialTurnsOnSameConversation(t *testing.T) {
	h := NewCLIHandler()
	h.SetPath(writeFakeCLI(t, "conv-seq"))

	if _, err := h.SendMessage(map[string]interface{}{"message": "first", "conversation_id": "conv-seq"}); err != nil {
		t.Fatalf("first turn: %v", err)
	}
	if _, err := h.SendMessage(map[string]interface{}{"message": "second", "conversation_id": "conv-seq"}); err != nil {
		t.Fatalf("second turn must run after the first finished: %v", err)
	}
	h.mu.Lock()
	leftover := len(h.runs)
	h.mu.Unlock()
	if leftover != 0 {
		t.Errorf("completed runs must be cleared, %d left", leftover)
	}
}

// Regression: the CLI index stores last_modified_time as TEXT; scanning it as a
// number silently produced an empty conversation list.
func TestCLIHandler_ListConversationsReadsTextTimestamps(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := filepath.Join(home, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "conversation_summaries.db")
	db, err := sqlOpenForTest(dbPath)
	if err != nil {
		t.Fatalf("open test index: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE conversation_summaries (
		conversation_id TEXT PRIMARY KEY, title TEXT, preview TEXT,
		step_count INTEGER, last_modified_time TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	rows := [][5]interface{}{
		{"older-conv", "Older", "first", 3, "2026-10-20 01:00:00.0+00:00"},
		{"newer-conv", "Newer", "second", 7, "2026-10-22 03:19:50.399569329+00:00"},
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO conversation_summaries VALUES (?,?,?,?,?)`, r[0], r[1], r[2], r[3], r[4]); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	_ = db.Close()

	h := NewCLIHandler()
	out, err := h.ListConversations(map[string]interface{}{"limit": 5.0})
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	var payload struct {
		Count         int `json:"count"`
		Conversations []struct {
			ConversationID string `json:"conversation_id"`
			Title          string `json:"title"`
			StepCount      int    `json:"step_count"`
			LastModified   string `json:"last_modified"`
		} `json:"conversations"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if payload.Count != 2 || len(payload.Conversations) != 2 {
		t.Fatalf("expected both conversations, got %s", out)
	}
	if payload.Conversations[0].ConversationID != "newer-conv" {
		t.Errorf("expected newest first, got %s", payload.Conversations[0].ConversationID)
	}
	if payload.Conversations[0].LastModified == "" || payload.Conversations[0].StepCount != 7 {
		t.Errorf("expected the text timestamp and step count to survive, got %+v", payload.Conversations[0])
	}
}

func sqlOpenForTest(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

// Caller-supplied argv values must never be parsed as a CLI flag.
func TestCLIHandler_RejectsFlagLikeArguments(t *testing.T) {
	h := NewCLIHandler()
	h.SetPath(writeFakeCLI(t, "conv-flags"))

	for name, call := range map[string]func() error{
		"model flag": func() error {
			_, err := h.CreateConversation(map[string]interface{}{"prompt": "hi", "model": "--dangerously-skip-permissions"})
			return err
		},
		"conversation flag": func() error {
			_, err := h.SendMessage(map[string]interface{}{"message": "hi", "conversation_id": "--version"})
			return err
		},
		"effort flag": func() error {
			_, err := h.SendMessage(map[string]interface{}{"message": "hi", "conversation_id": "conv-flags", "effort": "-x"})
			return err
		},
	} {
		if err := call(); err == nil {
			t.Errorf("%s: expected rejection of a flag-like value", name)
		}
	}

	// A missing or non-absolute binary is refused before any spawn.
	h.SetPath("agy-not-a-path")
	if _, err := h.CreateConversation(map[string]interface{}{"prompt": "hi"}); err == nil {
		t.Error("an unverified binary path must be refused")
	}
}

// The CLI's env vars must point at the gateway for a custom model and be absent
// for native ones.
func TestCLIGatewayEnvResolution(t *testing.T) {
	svc, err := NewService(t.TempDir() + "/bridge.json")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.SetCustomModelSources(func() []CustomModelRef {
		return []CustomModelRef{{ID: "deepseek-test", Name: "deepseek-chat", Label: "DeepSeek Test"}}
	}, nil)
	svc.SetGatewayTokenSource(func() string { return "gw-token" })

	env, canonical, ok := svc.cliGatewayEnv("deepseek-test")
	if !ok {
		t.Fatal("expected a gateway environment for a custom model")
	}
	if canonical != "deepseek-test" {
		t.Errorf("expected the canonical gateway id, got %q", canonical)
	}
	if env["AGY_LLM_GATEWAY_URL"] == "" || env["AGY_LLM_GATEWAY_API_KEY"] != "gw-token" {
		t.Errorf("unexpected gateway env: %+v", env)
	}
	if env["AGY_LLM_GATEWAY_MODELS"] != "deepseek-test" {
		t.Errorf("expected the comma-separated model list, got %q", env["AGY_LLM_GATEWAY_MODELS"])
	}
	if _, _, ok := svc.cliGatewayEnv("gemini-4-argon-high"); ok {
		t.Error("native models must not be routed through the gateway")
	}
}
