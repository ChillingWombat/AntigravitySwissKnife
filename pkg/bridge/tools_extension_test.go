package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestBridge(t *testing.T) *Service {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gemini_bridge.json")
	svc, err := NewService(path)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.mu.Lock()
	svc.config.AllowedTools = append(svc.config.AllowedTools,
		"local_run_command", "local_write_file", "local_computer_use",
		"local_http_request", "local_obsidian_write_note")
	svc.mu.Unlock()
	return svc
}

func TestHostAllowed(t *testing.T) {
	allow := []string{"api.calendar.example", "*.googleapis.com", "https://Hooks.Example.com/path"}
	cases := []struct {
		host string
		want bool
	}{
		{"api.calendar.example", true},
		{"API.Calendar.Example", true},
		{"www.googleapis.com", true},
		{"calendar.googleapis.com", true},
		{"googleapis.com", false}, // the wildcard needs a subdomain
		{"hooks.example.com", true},
		{"evil.example.com", false},
		{"api.calendar.example.evil.com", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := hostAllowed(tc.host, allow); got != tc.want {
			t.Errorf("hostAllowed(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestNormalizeAllowHost(t *testing.T) {
	cases := map[string]string{
		"  API.Example.com  ":           "api.example.com",
		"https://api.example.com/v1/x":  "api.example.com",
		"http://api.example.com:8080/p": "api.example.com:8080",
		"api.example.com.":              "api.example.com",
		"":                              "",
	}
	for in, want := range cases {
		if got := normalizeAllowHost(in); got != want {
			t.Errorf("normalizeAllowHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// A host that is not allowlisted must never be contacted in whitelist_only mode:
// the tool answers with an approval request instead, and the request is queued for the user.
func TestHTTPRequestRequiresApprovalForUnknownHost(t *testing.T) {
	svc := newTestBridge(t)
	if err := svc.SetHTTPAllowlistMode(HTTPAllowlistModeWhitelistOnly); err != nil {
		t.Fatalf("SetHTTPAllowlistMode: %v", err)
	}

	// A dead address: if the bridge tried to dial it, the call would fail instead
	// of returning an approval request.
	result, err := svc.HTTPRequest(map[string]interface{}{
		"url":    "http://127.0.0.1:1/calendar",
		"method": "POST",
		"reason": "read the user's calendar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("decode result: %v\n%s", err, result)
	}
	if payload["status"] != "approval_required" {
		t.Fatalf("expected approval_required, got %v", payload["status"])
	}
	if payload["request_sent"] != false {
		t.Errorf("no request may be sent before approval, got request_sent=%v", payload["request_sent"])
	}

	pending := svc.PendingHTTPApprovals()
	if len(pending) != 1 || pending[0].Host != "127.0.0.1:1" {
		t.Fatalf("expected one pending approval for the host, got %+v", pending)
	}
	if pending[0].Reason != "read the user's calendar" {
		t.Errorf("pending approval must carry the agent's reason, got %q", pending[0].Reason)
	}

	// Queueing twice must not duplicate the entry.
	if _, err := svc.HTTPRequest(map[string]interface{}{"url": "http://127.0.0.1:1/again"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(svc.PendingHTTPApprovals()); got != 1 {
		t.Errorf("pending approvals must be deduplicated per host, got %d", got)
	}
}

func TestHTTPRequestAfterApproval(t *testing.T) {
	svc := newTestBridge(t)
	if err := svc.SetHTTPAllowlistMode(HTTPAllowlistModeWhitelistOnly); err != nil {
		t.Fatalf("SetHTTPAllowlistMode: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Errorf("expected the caller's Authorization header, got %q", got)
		}
		w.Header().Set("X-Test", "ok")
		_, _ = w.Write([]byte(`{"events":[]}`))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")

	result, err := svc.HTTPRequest(map[string]interface{}{"url": server.URL + "/events"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "approval_required") {
		t.Fatalf("expected an approval request before the host is allowed, got %s", result)
	}

	if _, err := svc.AllowHTTPHost(host); err != nil {
		t.Fatalf("AllowHTTPHost: %v", err)
	}
	if got := svc.PendingHTTPApprovals(); len(got) != 0 {
		t.Errorf("approving a host must clear its pending request, got %+v", got)
	}

	result, err = svc.HTTPRequest(map[string]interface{}{
		"url":     server.URL + "/events",
		"method":  "POST",
		"body":    `{"title":"standup"}`,
		"headers": map[string]interface{}{"Authorization": "Bearer token-123"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("decode result: %v\n%s", err, result)
	}
	if payload["status"] != "ok" || payload["request_sent"] != true {
		t.Fatalf("expected a sent request, got %+v", payload)
	}
	if payload["http_status"].(float64) != 200 {
		t.Errorf("expected 200, got %v", payload["http_status"])
	}
	if body, _ := payload["body"].(string); body != `{"events":[]}` {
		t.Errorf("expected the response body, got %q", body)
	}
	if headers, ok := payload["headers"].(map[string]interface{}); !ok || headers["X-Test"] != "ok" {
		t.Errorf("expected response headers, got %v", payload["headers"])
	}
}

func TestHTTPAllowlistCommand(t *testing.T) {
	svc := newTestBridge(t)

	if _, err := svc.HTTPAllowlistCommand(map[string]interface{}{"action": "allow", "host": "api.example.com"}); err != nil {
		t.Fatalf("allow: %v", err)
	}
	list, err := svc.HTTPAllowlistCommand(map[string]interface{}{"action": "list"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(list, "api.example.com") {
		t.Fatalf("allowlist must contain the allowed host, got %s", list)
	}

	if _, err := svc.HTTPAllowlistCommand(map[string]interface{}{"action": "revoke", "host": "api.example.com"}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if hosts := svc.AllowlistHosts(); len(hosts) != 0 {
		t.Errorf("revoke must remove the host, got %v", hosts)
	}

	if _, err := svc.HTTPAllowlistCommand(map[string]interface{}{"action": "explode"}); err == nil {
		t.Error("an unknown allowlist action must error")
	}
	if _, err := svc.HTTPAllowlistCommand(map[string]interface{}{"action": "allow", "host": "  "}); err == nil {
		t.Error("allowing an empty host must error")
	}
}

func TestHTTPRequestRejectsNonHTTPSchemes(t *testing.T) {
	svc := newTestBridge(t)
	if _, err := svc.AllowHTTPHost("example.com"); err != nil {
		t.Fatalf("AllowHTTPHost: %v", err)
	}
	if _, err := svc.HTTPRequest(map[string]interface{}{"url": "file:///etc/passwd"}); err == nil {
		t.Fatal("file:// must be rejected")
	}
	if _, err := svc.HTTPRequest(map[string]interface{}{"url": "not a url"}); err == nil {
		t.Fatal("a url without a host must be rejected")
	}
}

// setupTestVault creates a vault under a temporary HOME so discovery finds it.
func setupTestVault(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	obsidianMu.Lock()
	obsidianCache = nil
	obsidianCachedAt = time.Time{}
	obsidianMu.Unlock()

	vault := filepath.Join(home, "Notes", "Brain")
	if err := os.MkdirAll(filepath.Join(vault, ".obsidian"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "Projects"), 0755); err != nil {
		t.Fatal(err)
	}
	notes := map[string]string{
		filepath.Join(vault, "Daily.md"):             "# Daily\n\nMet #alice about the launch plan.\nSee [[Launch Plan]] for details.\n",
		filepath.Join(vault, "Launch Plan.md"):       "# Launch Plan\n\nShip the bridge tools. #work\n",
		filepath.Join(vault, "Projects", "Retro.md"): "# Retro\n\nWe used [[Launch Plan]] and #alice helped.\n",
	}
	for path, content := range notes {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return vault
}

func TestObsidianListVaults(t *testing.T) {
	vault := setupTestVault(t)
	dh := NewDirectHandler()

	out, err := dh.ObsidianListVaults(map[string]interface{}{"refresh": true})
	if err != nil {
		t.Fatalf("ObsidianListVaults: %v", err)
	}
	var payload struct {
		Count  int `json:"count"`
		Vaults []struct {
			Path  string `json:"path"`
			Notes int    `json:"notes"`
		} `json:"vaults"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// The machine may hold real vaults too; the freshly created one must be listed
	// with its note count.
	found := false
	for _, listed := range payload.Vaults {
		if strings.HasSuffix(filepath.ToSlash(listed.Path), "/Notes/Brain") || strings.EqualFold(filepath.Clean(listed.Path), filepath.Clean(vault)) {
			found = true
			if listed.Notes != 3 {
				t.Errorf("expected 3 notes in the test vault, got %d", listed.Notes)
			}
			break
		}
	}
	if !found {
		t.Fatalf("the test vault must be listed, got %s", out)
	}
	if payload.Count != len(payload.Vaults) {
		t.Errorf("count %d must match the vault list length %d", payload.Count, len(payload.Vaults))
	}
}

func TestObsidianSearchAndNotes(t *testing.T) {
	vault := setupTestVault(t)
	dh := NewDirectHandler()

	// Body search.
	out, err := dh.ObsidianSearch(map[string]interface{}{"query": "launch plan"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "Launch Plan.md") {
		t.Errorf("body search must find the note, got %s", out)
	}

	// Tag query.
	out, err = dh.ObsidianSearch(map[string]interface{}{"query": "#alice"})
	if err != nil {
		t.Fatalf("tag search: %v", err)
	}
	if !strings.Contains(out, "Daily.md") || !strings.Contains(out, "Retro.md") {
		t.Errorf("#alice must match both notes, got %s", out)
	}
	if strings.Contains(out, "Launch Plan.md") {
		t.Errorf("the tag search must not match notes without the tag, got %s", out)
	}

	// Wikilink query.
	out, err = dh.ObsidianSearch(map[string]interface{}{"query": "[[Launch Plan]]"})
	if err != nil {
		t.Fatalf("wikilink search: %v", err)
	}
	if !strings.Contains(out, "Daily.md") || !strings.Contains(out, "Retro.md") {
		t.Errorf("wikilink search must match both referring notes, got %s", out)
	}

	// Read by vault-relative reference.
	out, err = dh.ObsidianReadNote(map[string]interface{}{"path": "Brain:Projects/Retro"})
	if err != nil {
		t.Fatalf("read by reference: %v", err)
	}
	if !strings.Contains(out, "We used [[Launch Plan]]") {
		t.Errorf("unexpected note content: %s", out)
	}

	// Write: append keeps the old content, overwrite replaces it.
	target := filepath.Join(vault, "New Note")
	if _, err := dh.ObsidianWriteNote(map[string]interface{}{"path": target, "content": "first line"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := dh.ObsidianWriteNote(map[string]interface{}{"path": target + ".md", "content": "second line", "mode": "append"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	data, err := os.ReadFile(target + ".md")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(data), "first line") || !strings.Contains(string(data), "second line") {
		t.Errorf("append must keep both lines, got %q", string(data))
	}
	if _, err := dh.ObsidianWriteNote(map[string]interface{}{"path": target, "content": "reset", "mode": "overwrite"}); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	data, _ = os.ReadFile(target + ".md")
	if strings.TrimSpace(string(data)) != "reset" {
		t.Errorf("overwrite must replace the note, got %q", string(data))
	}
	if _, err := dh.ObsidianWriteNote(map[string]interface{}{"path": target, "content": "x", "mode": "sideways"}); err == nil {
		t.Error("an unsupported write mode must error")
	}
}

func TestToolManifestExposesFullToolset(t *testing.T) {
	svc := newTestBridge(t)

	names := map[string]bool{}
	for _, tool := range svc.ToolManifest() {
		if name, ok := tool["name"].(string); ok {
			names[name] = true
		}
	}
	required := []string{
		"local_run_command", "local_read_file", "local_write_file", "local_list_directory",
		"local_search_files", "local_system_info", "local_computer_use",
		"local_http_request", "local_http_allowlist",
		"local_obsidian_list_vaults", "local_obsidian_search",
		"local_obsidian_read_note", "local_obsidian_write_note",
		"antigravity_session_model_list", "antigravity_session_model_get", "antigravity_session_model_set",
		"antigravity_lead_create_conversation", "antigravity_lead_await_response",
	}
	for _, name := range required {
		if !names[name] {
			t.Errorf("hybrid mode must expose %q", name)
		}
	}

	// Exposed tools are callable only after an explicit entry, except machine-control
	// tools, which remain listed for opt-in but are denied by default.
	allowed := map[string]bool{}
	for _, name := range svc.GetConfig().AllowedTools {
		allowed[name] = true
	}
	for name := range names {
		if name == "antigravity_dispatch_task" || name == "antigravity_list_sessions" {
			continue
		}
		if _, gated := machineControlTools[name]; gated {
			continue
		}
		if !allowed[name] {
			t.Errorf("tool %q is exposed but missing from allowed_tools", name)
		}
	}
	defaultAllowed := map[string]bool{}
	for _, name := range DefaultConfig().AllowedTools {
		defaultAllowed[name] = true
	}
	for name := range machineControlTools {
		if defaultAllowed[name] {
			t.Errorf("machine-control tool %q must not be allowed by default", name)
		}
	}
}

func TestHTTPRequestAlwaysAllowMode(t *testing.T) {
	svc := newTestBridge(t)
	if got := svc.HTTPAllowlistMode(); got != "always_allow" {
		t.Fatalf("expected default mode always_allow, got %s", got)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events":["meeting"]}`))
	}))
	defer server.Close()

	result, err := svc.HTTPRequest(map[string]interface{}{"url": server.URL + "/events"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "approval_required") {
		t.Fatalf("always_allow must not require approval, got %s", result)
	}
	if len(svc.PendingHTTPApprovals()) != 0 {
		t.Fatalf("always_allow must not create pending approvals")
	}
}

func TestHTTPAllowlistModePersistence(t *testing.T) {
	svc := newTestBridge(t)
	if got := svc.HTTPAllowlistMode(); got != "always_allow" {
		t.Fatalf("default mode should be always_allow, got %s", got)
	}
	if err := svc.SetHTTPAllowlistMode("whitelist_only"); err != nil {
		t.Fatalf("SetHTTPAllowlistMode: %v", err)
	}
	if got := svc.HTTPAllowlistMode(); got != "whitelist_only" {
		t.Fatalf("mode should be whitelist_only, got %s", got)
	}
	// Reload from disk
	svc2, err := NewService(svc.configPath)
	if err != nil {
		t.Fatalf("NewService reload: %v", err)
	}
	if got := svc2.HTTPAllowlistMode(); got != "whitelist_only" {
		t.Fatalf("reloaded mode should be whitelist_only, got %s", got)
	}
}
