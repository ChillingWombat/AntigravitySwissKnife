package acpmesh

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func testEnv(goos string) Env {
	return Env{
		GOOS:         goos,
		Home:         "/home/tester",
		RunUserDir:   "/run/user/1000",
		LocalAppData: `C:\Users\tester\AppData\Local`,
	}
}

func findResolved(t *testing.T, agents []Resolved, id string) Resolved {
	t.Helper()
	for _, a := range agents {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("agent %q not present in resolved registry", id)
	return Resolved{}
}

func TestDefaultEndpointPerOS(t *testing.T) {
	cases := []struct {
		name     string
		goos     string
		presetID string
		want     string
	}{
		{
			name:     "antigravity acp stdio has empty endpoint on linux",
			goos:     "linux",
			presetID: PresetAntigravityACP,
			want:     "",
		},
		{
			name:     "antigravity acp stdio has empty endpoint on darwin",
			goos:     "darwin",
			presetID: PresetAntigravityACP,
			want:     "",
		},
		{
			name:     "antigravity acp stdio has empty endpoint on windows",
			goos:     "windows",
			presetID: PresetAntigravityACP,
			want:     "",
		},
		{
			name:     "devin linux socket",
			goos:     "linux",
			presetID: PresetDevin,
			want:     "unix:///run/user/1000/devin-acp.sock",
		},
		{
			name:     "devin windows pipe",
			goos:     "windows",
			presetID: PresetDevin,
			want:     `\\.\pipe\devin-acp`,
		},
		{
			name:     "tcp preset unchanged on windows",
			goos:     "windows",
			presetID: PresetOpenCode,
			want:     "127.0.0.1:4096",
		},
		{
			name:     "tcp preset unchanged on darwin",
			goos:     "darwin",
			presetID: PresetCodex,
			want:     "127.0.0.1:48080",
		},
		{
			name:     "tcp preset unchanged on linux",
			goos:     "linux",
			presetID: PresetClaudeCode,
			want:     "127.0.0.1:45124",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			preset, ok := PresetByID(tc.presetID)
			if !ok {
				t.Fatalf("preset %q not found", tc.presetID)
			}
			if got := preset.DefaultEndpoint(testEnv(tc.goos)); got != tc.want {
				t.Fatalf("DefaultEndpoint(%s) = %q, want %q", tc.goos, got, tc.want)
			}
		})
	}
}

func TestDefaultBinaryPathPerOS(t *testing.T) {
	cases := []struct {
		name     string
		goos     string
		presetID string
		want     string
	}{
		{"antigravity acp windows", "windows", PresetAntigravityACP, "/home/tester/bin/antigravity-acp.exe"},
		{"antigravity acp linux", "linux", PresetAntigravityACP, "/home/tester/.local/bin/antigravity-acp"},
		{"antigravity acp darwin", "darwin", PresetAntigravityACP, "/home/tester/.local/bin/antigravity-acp"},
		{"devin windows falls back to historical path", "windows", PresetDevin, "/usr/bin/devin-desktop"},
		{"cursor windows", "windows", PresetCursor, "C:/Users/tester/AppData/Local/Programs/cursor/Cursor.exe"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			preset, _ := PresetByID(tc.presetID)
			if got := preset.DefaultBinaryPath(testEnv(tc.goos)); got != tc.want {
				t.Fatalf("DefaultBinaryPath(%s) = %q, want %q", tc.goos, got, tc.want)
			}
		})
	}
}

func TestUnifiedAntigravityACPPreset(t *testing.T) {
	presets := Presets()
	if len(presets) == 0 || presets[0].ID != PresetAntigravityACP {
		t.Fatalf("PresetAntigravityACP must be the first preset, got %+v", presets)
	}

	for _, p := range presets {
		if p.ID == "agent-antigravity-cli" || p.ID == "agent-antigravity-vscode" {
			t.Fatalf("obsolete preset %s must be removed", p.ID)
		}
		if p.SocketName == "antigravity-acp.sock" {
			t.Fatal("fictional antigravity-acp.sock must be removed")
		}
	}

	preset, ok := PresetByID(PresetAntigravityACP)
	if !ok {
		t.Fatal("Antigravity ACP Adapter preset is missing")
	}
	if preset.Name != "Antigravity ACP Adapter" {
		t.Fatalf("preset name = %q, want 'Antigravity ACP Adapter'", preset.Name)
	}
	if preset.BinaryLinux != "~/.local/bin/antigravity-acp" {
		t.Fatalf("preset binary = %q", preset.BinaryLinux)
	}
	if preset.DefaultEndpoint(testEnv("linux")) != "" {
		t.Fatalf("expected empty endpoint for stdio preset, got %q", preset.DefaultEndpoint(testEnv("linux")))
	}
}

func TestExpandPathExpandsUserProfile(t *testing.T) {
	env := testEnv("windows")

	t.Setenv("USERPROFILE", "")
	if got := expandPath("%USERPROFILE%/bin/antigravity-acp.exe", env); got != "/home/tester/bin/antigravity-acp.exe" {
		t.Fatalf("USERPROFILE fallback expansion = %q", got)
	}

	t.Setenv("USERPROFILE", `C:\Users\tester`)
	if got := expandPath("%USERPROFILE%/bin/antigravity-acp.exe", env); got != "C:/Users/tester/bin/antigravity-acp.exe" {
		t.Fatalf("USERPROFILE env expansion = %q", got)
	}
}

func TestMergePresetWithoutOverrideUsesOSDefault(t *testing.T) {
	agents := Merge(nil, testEnv("windows"))
	if len(agents) != len(Presets()) {
		t.Fatalf("expected %d presets, got %d", len(Presets()), len(agents))
	}

	antigravity := findResolved(t, agents, PresetAntigravityACP)
	if !antigravity.OSDefault {
		t.Fatal("preset without override must be flagged OSDefault")
	}
	if antigravity.Endpoint != "" {
		t.Fatalf("unexpected endpoint %q, expected empty for stdio", antigravity.Endpoint)
	}
	if antigravity.Custom {
		t.Fatal("preset must not be flagged Custom")
	}
	if len(antigravity.Tools) != 5 {
		t.Fatalf("expected the historical 5 tools, got %v", antigravity.Tools)
	}
}

func TestMergeOverrideWinsOverOSDefault(t *testing.T) {
	overrides := []core.ACPAgentConfig{
		{ID: PresetAntigravityACP, Endpoint: "127.0.0.1:9911", BinaryPath: "/custom/antigravity-acp", Name: "Antigravity Custom"},
	}
	agents := Merge(overrides, testEnv("linux"))
	antigravity := findResolved(t, agents, PresetAntigravityACP)

	if antigravity.Endpoint != "127.0.0.1:9911" {
		t.Fatalf("override endpoint not applied: %q", antigravity.Endpoint)
	}
	if antigravity.OSDefault {
		t.Fatal("override endpoint must not be flagged OSDefault")
	}
	if antigravity.BinaryPath != "/custom/antigravity-acp" {
		t.Fatalf("override binary path not applied: %q", antigravity.BinaryPath)
	}
	if antigravity.Name != "Antigravity Custom" {
		t.Fatalf("override name not applied: %q", antigravity.Name)
	}
	// Fields the override leaves empty must fall back to the preset.
	if antigravity.Type != "Desktop IDE & Agent Core (ACP)" {
		t.Fatalf("empty override type must keep the preset type, got %q", antigravity.Type)
	}
	if antigravity.Version != "v1.2.0-draft" {
		t.Fatalf("empty override version must keep the preset version, got %q", antigravity.Version)
	}
}

func TestMergeCustomAgentsAppended(t *testing.T) {
	overrides := []core.ACPAgentConfig{
		{ID: "custom-my-agent", Name: "My Agent", Endpoint: "127.0.0.1:7000", Custom: true, Tools: []string{"shell"}},
		{ID: "custom-no-endpoint", Name: "Broken", Custom: true},
		{ID: "agent-removed-in-future", Endpoint: "127.0.0.1:7001"},
	}
	agents := Merge(overrides, testEnv("linux"))

	if len(agents) != len(Presets())+1 {
		t.Fatalf("expected presets + 1 custom agent, got %d", len(agents))
	}

	last := agents[len(agents)-1]
	if last.ID != "custom-my-agent" || !last.Custom {
		t.Fatalf("custom agent must be appended last, got %+v", last)
	}
	if last.Endpoint != "127.0.0.1:7000" || last.OSDefault {
		t.Fatalf("custom endpoint must be used verbatim: %+v", last)
	}
	if last.Type != "Custom Agent" {
		t.Fatalf("custom agent without type must get a fallback type, got %q", last.Type)
	}
	if last.Tools == nil {
		t.Fatal("custom agent tools must be an empty slice, not nil (JSON null breaks the UI)")
	}

	for _, a := range agents {
		if a.ID == "custom-no-endpoint" {
			t.Fatal("custom agent without endpoint must be skipped")
		}
		if a.ID == "agent-removed-in-future" {
			t.Fatal("unknown non-custom id must be ignored")
		}
	}
}

func TestMergePresetToolsOverride(t *testing.T) {
	overrides := []core.ACPAgentConfig{{ID: PresetPi, Tools: []string{"only_tool"}}}
	agents := Merge(overrides, testEnv("linux"))
	pi := findResolved(t, agents, PresetPi)
	if len(pi.Tools) != 1 || pi.Tools[0] != "only_tool" {
		t.Fatalf("override tools not applied: %v", pi.Tools)
	}
}

func TestValidateEndpoint(t *testing.T) {
	valid := []string{
		"unix:///run/user/1000/antigravity-acp.sock",
		`\\.\pipe\antigravity-acp`,
		"npipe://antigravity-acp",
		"127.0.0.1:4096",
		"localhost:4096",
		"[::1]:4096",
	}
	for _, endpoint := range valid {
		if err := ValidateEndpoint(endpoint); err != nil {
			t.Fatalf("expected %q to be valid: %v", endpoint, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"unix://",
		`\\.\pipe\`,
		"npipe://",
		"127.0.0.1",
		"127.0.0.1:0",
		"127.0.0.1:70000",
		"127.0.0.1:abc",
		":4096",
		"http://127.0.0.1:4096",
	}
	for _, endpoint := range invalid {
		if err := ValidateEndpoint(endpoint); err == nil {
			t.Fatalf("expected %q to be rejected", endpoint)
		}
	}
}

func TestNormalizeAgentInput(t *testing.T) {
	taken := map[string]bool{}
	for _, p := range Presets() {
		taken[p.ID] = true
	}

	t.Run("preset override is not custom", func(t *testing.T) {
		entry, custom, err := NormalizeAgentInput(AgentInput{
			ID:       PresetDevin,
			Name:     "Devin",
			Endpoint: "127.0.0.1:9000",
			Tools:    []string{" a ", "", "b", "a"},
		}, taken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if custom {
			t.Fatal("preset override must not be custom")
		}
		if entry.Custom {
			t.Fatal("persisted entry must not be flagged custom")
		}
		if len(entry.Tools) != 2 || entry.Tools[0] != "a" || entry.Tools[1] != "b" {
			t.Fatalf("tools not normalised: %v", entry.Tools)
		}
	})

	t.Run("unknown id creates custom agent", func(t *testing.T) {
		entry, custom, err := NormalizeAgentInput(AgentInput{
			Name:     "My Local Agent",
			Endpoint: "127.0.0.1:9001",
		}, taken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !custom || !entry.Custom {
			t.Fatal("new agent must be flagged custom")
		}
		if entry.ID != "custom-my-local-agent" {
			t.Fatalf("unexpected generated id %q", entry.ID)
		}
	})

	t.Run("custom id collision gets a suffix", func(t *testing.T) {
		ids := map[string]bool{"custom-my-local-agent": true}
		if got := GenerateCustomID("My Local Agent", ids); got != "custom-my-local-agent-2" {
			t.Fatalf("unexpected id %q", got)
		}
	})

	t.Run("editing an existing custom agent keeps its id", func(t *testing.T) {
		ids := map[string]bool{"custom-my-local-agent": true}
		entry, custom, err := NormalizeAgentInput(AgentInput{
			ID:       "custom-my-local-agent",
			Name:     "My Local Agent",
			Endpoint: "127.0.0.1:9002",
		}, ids)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !custom {
			t.Fatal("custom agent must stay custom")
		}
		if entry.ID != "custom-my-local-agent" {
			t.Fatalf("editing must not mint a new id, got %q", entry.ID)
		}
	})

	t.Run("new agent requires a name", func(t *testing.T) {
		if _, _, err := NormalizeAgentInput(AgentInput{Endpoint: "127.0.0.1:9002"}, taken); err == nil {
			t.Fatal("expected an error for a nameless new agent")
		}
	})

	t.Run("preset Antigravity ACP override with empty or stdio endpoint succeeds", func(t *testing.T) {
		entry, custom, err := NormalizeAgentInput(AgentInput{
			ID:         PresetAntigravityACP,
			Name:       "Antigravity ACP Adapter",
			Endpoint:   "",
			BinaryPath: "~/.local/bin/antigravity-acp",
			Tools:      []string{"read_file", "write_file"},
		}, taken)
		if err != nil {
			t.Fatalf("unexpected error for empty endpoint on PresetAntigravityACP: %v", err)
		}
		if custom {
			t.Fatal("preset override must not be custom")
		}
		if entry.Endpoint != "" {
			t.Fatalf("expected empty endpoint, got %q", entry.Endpoint)
		}

		// Also test explicit "stdio" string input
		entryStdio, _, errStdio := NormalizeAgentInput(AgentInput{
			ID:       PresetAntigravityACP,
			Endpoint: "stdio",
		}, taken)
		if errStdio != nil {
			t.Fatalf("unexpected error for stdio endpoint: %v", errStdio)
		}
		if entryStdio.Endpoint != "" {
			t.Fatalf("expected empty endpoint when stdio is passed, got %q", entryStdio.Endpoint)
		}
	})

	t.Run("legacy agent-antigravity id maps to PresetAntigravityACP", func(t *testing.T) {
		entry, custom, err := NormalizeAgentInput(AgentInput{
			ID:       "agent-antigravity",
			Endpoint: "",
		}, taken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if custom {
			t.Fatal("legacy preset id must not be custom")
		}
		if entry.ID != PresetAntigravityACP {
			t.Fatalf("expected ID %q, got %q", PresetAntigravityACP, entry.ID)
		}

		p, ok := PresetByID("agent-antigravity")
		if !ok || p.ID != PresetAntigravityACP {
			t.Fatalf("PresetByID for legacy id failed: got %+v, ok=%v", p, ok)
		}
	})

	t.Run("bad endpoint rejected", func(t *testing.T) {
		if _, _, err := NormalizeAgentInput(AgentInput{Name: "X", Endpoint: "not-an-endpoint"}, taken); err == nil {
			t.Fatal("expected an error for an invalid endpoint")
		}
	})
}

func TestRemoveAgentSemantics(t *testing.T) {
	overrides := []core.ACPAgentConfig{
		{ID: PresetDevin, Endpoint: "127.0.0.1:9000"},
		{ID: "custom-one", Name: "One", Endpoint: "127.0.0.1:9001", Custom: true},
	}

	t.Run("preset override removal resets to default", func(t *testing.T) {
		updated, removed := RemoveAgent(overrides, PresetDevin)
		if !removed {
			t.Fatal("expected the preset override to be removed")
		}
		agents := Merge(updated, testEnv("linux"))
		devin := findResolved(t, agents, PresetDevin)
		if !devin.OSDefault {
			t.Fatal("preset must fall back to its OS default after removal")
		}
		if devin.Endpoint != "unix:///run/user/1000/devin-acp.sock" {
			t.Fatalf("unexpected endpoint after reset: %q", devin.Endpoint)
		}
	})

	t.Run("custom removal deletes the node", func(t *testing.T) {
		updated, removed := RemoveAgent(overrides, "custom-one")
		if !removed {
			t.Fatal("expected the custom agent to be removed")
		}
		for _, a := range updated {
			if a.ID == "custom-one" {
				t.Fatal("custom agent still present")
			}
		}
	})

	t.Run("unknown id reports no change", func(t *testing.T) {
		if _, removed := RemoveAgent(overrides, "custom-nope"); removed {
			t.Fatal("expected removed=false for an unknown id")
		}
	})
}

func TestUpsertAgent(t *testing.T) {
	initial := []core.ACPAgentConfig{{ID: PresetDevin, Endpoint: "127.0.0.1:1"}}
	updated := UpsertAgent(initial, core.ACPAgentConfig{ID: PresetDevin, Endpoint: "127.0.0.1:2"})
	if len(updated) != 1 || updated[0].Endpoint != "127.0.0.1:2" {
		t.Fatalf("upsert must replace in place: %+v", updated)
	}

	appended := UpsertAgent(updated, core.ACPAgentConfig{ID: "custom-x", Custom: true, Endpoint: "127.0.0.1:3"})
	if len(appended) != 2 || appended[1].ID != "custom-x" {
		t.Fatalf("upsert must append new ids: %+v", appended)
	}
}

func TestBuildCandidates(t *testing.T) {
	candidates := BuildCandidates(
		"127.0.0.1:4096",
		"127.0.0.1:4096",
		[]int{4096, 35779, 0, 70000},
		[]string{"/run/user/1000/antigravity-acp.sock", "/run/user/1000/antigravity-acp.sock"},
	)

	byEndpoint := make(map[string]Candidate, len(candidates))
	for _, c := range candidates {
		byEndpoint[c.Endpoint] = c
	}

	if len(byEndpoint) != len(candidates) {
		t.Fatal("candidates must be de-duplicated")
	}
	if len(candidates) != 3 {
		t.Fatalf("expected configured + port + socket candidates, got %d: %+v", len(candidates), candidates)
	}
	if _, ok := byEndpoint["127.0.0.1:35779"]; !ok {
		t.Fatal("observed listening port missing")
	}
	if _, ok := byEndpoint["unix:///run/user/1000/antigravity-acp.sock"]; !ok {
		t.Fatal("observed socket path missing")
	}
	for _, c := range candidates {
		if c.Endpoint == "" || c.Kind == "" || c.Note == "" {
			t.Fatalf("candidate must be fully described: %+v", c)
		}
		if c.Reachable {
			t.Fatal("BuildCandidates must not claim reachability before probing")
		}
	}
}

func TestBuildCandidatesNoFabrication(t *testing.T) {
	candidates := BuildCandidates("", "", nil, nil)
	if len(candidates) != 0 {
		t.Fatalf("no observations must yield no candidates, got %+v", candidates)
	}
}

func TestProbeEndpointUnreachable(t *testing.T) {
	reachable, latency := ProbeEndpoint("127.0.0.1:1", 30*time.Millisecond)
	if reachable || latency != 0 {
		t.Fatalf("expected an unreachable result, got reachable=%v latency=%v", reachable, latency)
	}
}

func TestProbeEndpointInvalidPipeOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipes are probed for real on Windows")
	}
	if reachable, _ := ProbeEndpoint(`\\.\pipe\definitely-not-a-real-pipe`, 10*time.Millisecond); reachable {
		t.Fatal("named pipes must not report reachable on non-Windows hosts")
	}
}

func TestACPAgentConfigJSONRoundTrip(t *testing.T) {
	raw := `{"id":"custom-x","name":"X","endpoint":"127.0.0.1:9000","acp_version":"v1","tools":["a"],"custom":true,"unknown_field":"ignored"}`
	var entry core.ACPAgentConfig
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if entry.ID != "custom-x" || !entry.Custom || entry.Endpoint != "127.0.0.1:9000" {
		t.Fatalf("unexpected entry: %+v", entry)
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(encoded), `"custom":true`) {
		t.Fatalf("custom flag missing from %s", encoded)
	}
}
