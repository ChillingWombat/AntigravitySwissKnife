// Package acpmesh resolves the ACP agent registry shown on the Utilities >
// ACP Agent Mesh page.
//
// The package combines three inputs:
//
//  1. the built-in presets (a fixed list of agents the page has always listed),
//  2. the user's persisted overrides for those presets, and
//  3. custom agents the user added.
//
// IMPORTANT: nothing here asserts that any third-party application ships an ACP
// server natively. The presets only carry conventional/observational defaults
// (a plausible endpoint and binary location per OS). Every field is meant to be
// overridden by the user, which is why the resolved registry always reports
// whether an endpoint came from the OS default or from the user.
package acpmesh

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// Preset identifiers. They are stable because persisted overrides are keyed by
// them.
const (
	PresetAntigravity    = "agent-antigravity"
	PresetAntigravityCLI = "agent-antigravity-cli"
	PresetDevin          = "agent-devin"
	PresetOpenCode       = "agent-opencode"
	PresetDeepSeek       = "agent-deepseek-harness"
	PresetPi             = "agent-pi"
	PresetCodex          = "agent-codex"
	PresetClaudeCode     = "agent-claude-code"
	PresetCursor         = "agent-cursor"
)

// Env carries the host-specific inputs used to compute OS defaults. It is a
// plain struct rather than direct runtime.GOOS / os.Getenv calls so the default
// selection logic can be exercised for every OS from any host.
type Env struct {
	GOOS         string
	Home         string
	RunUserDir   string
	LocalAppData string
	// AntigravityBinary is an optional pre-resolved Antigravity executable
	// (core.GetAntigravityBinaryPath) that already knows the per-OS layout.
	AntigravityBinary string
}

// CurrentEnv snapshots the host environment.
func CurrentEnv() Env {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	env := Env{
		GOOS:              runtime.GOOS,
		Home:              home,
		LocalAppData:      os.Getenv("LOCALAPPDATA"),
		AntigravityBinary: core.GetAntigravityBinaryPath(),
	}
	if runtime.GOOS != "windows" {
		runUserDir := fmt.Sprintf("/run/user/%d", os.Getuid())
		if _, err := os.Stat(runUserDir); err != nil {
			// Containers and CI images often lack the per-user runtime dir.
			runUserDir = "/run/user/1000"
		}
		env.RunUserDir = runUserDir
	}
	return env
}

// Preset describes one built-in agent entry.
type Preset struct {
	ID      string
	Name    string
	Type    string
	Version string
	Tools   []string
	// SocketName is the conventional unix socket file name / Windows named
	// pipe base name. Empty for TCP presets.
	SocketName string
	// MacAppSupportDir is the directory name used for the macOS
	// ~/Library/Application Support/<dir>/<socket> assumption.
	MacAppSupportDir string
	// TCPAddr is the conventional loopback address for TCP presets.
	TCPAddr string
	// Binary paths per OS. "~/" and "%LOCALAPPDATA%/" prefixes are expanded.
	// An empty per-OS value falls back to BinaryLinux (the historical preset
	// value) rather than inventing a path.
	BinaryLinux   string
	BinaryDarwin  string
	BinaryWindows string
	// StatusWhenRunning is the status reported for a running process whose
	// endpoint does not answer.
	StatusWhenRunning string
}

// Presets returns the built-in registry in display order. Tool lists and names
// are unchanged from the historical hardcoded page.
func Presets() []Preset {
	return []Preset{
		{
			ID:      PresetAntigravity,
			Name:    "Google Antigravity 2.0",
			Type:    "Desktop IDE & Agent Core",
			Version: "v1.2.0-draft",
			Tools:   []string{"read_file", "write_file", "terminal", "mcp_proxy", "subagent_invoke"},
			// Assumption: the desktop IDE exposes its ACP endpoint as a
			// per-user socket on Linux and a named pipe on Windows.
			SocketName:        "antigravity-acp.sock",
			MacAppSupportDir:  "Antigravity",
			BinaryLinux:       "/opt/Antigravity/antigravity",
			BinaryDarwin:      "/Applications/Antigravity.app/Contents/MacOS/Antigravity",
			BinaryWindows:     "%LOCALAPPDATA%/Programs/Antigravity/Antigravity.exe",
			StatusWhenRunning: "active_hosting",
		},
		{
			ID:                PresetAntigravityCLI,
			Name:              "Antigravity CLI (agy)",
			Type:              "Terminal Agent Daemon",
			Version:           "v1.2.0-draft",
			Tools:             []string{"terminal", "bash", "file_ops", "headless_prompt", "subagent_mesh"},
			TCPAddr:           "127.0.0.1:35779",
			BinaryLinux:       "~/.local/bin/agy",
			BinaryWindows:     "~/.local/bin/agy.exe",
			StatusWhenRunning: "connected",
		},
		{
			ID:      PresetDevin,
			Name:    "Devin",
			Type:    "Autonomous Developer IDE",
			Version: "v1.2.0",
			Tools:   []string{"cascade_tools", "terminal", "diff_apply", "browser_eval"},
			// No verified Windows/macOS binary location, so those hosts keep
			// the historical Linux path instead of a guess.
			SocketName:        "devin-acp.sock",
			MacAppSupportDir:  "Devin",
			BinaryLinux:       "/usr/bin/devin-desktop",
			StatusWhenRunning: "connected",
		},
		{
			ID:                PresetOpenCode,
			Name:              "OpenCode",
			Type:              "Editor & Agent Sidecar",
			Version:           "v1.2.0",
			Tools:             []string{"read_file", "write_file", "bash", "grep", "lsp_bridge", "subagent_spawn"},
			TCPAddr:           "127.0.0.1:4096",
			BinaryLinux:       "~/.local/bin/opencode",
			BinaryWindows:     "~/.local/bin/opencode.exe",
			StatusWhenRunning: "connected",
		},
		{
			ID:                PresetDeepSeek,
			Name:              "DeepSeek Harness",
			Type:              "Terminal Agent Harness",
			Version:           "v1.1.8",
			Tools:             []string{"command_exec", "file_ops", "git_apply", "reasoning_stream"},
			TCPAddr:           "127.0.0.1:43880",
			BinaryLinux:       "~/.local/bin/dsh",
			BinaryWindows:     "~/.local/bin/dsh.exe",
			StatusWhenRunning: "connected",
		},
		{
			ID:                PresetPi,
			Name:              "Pi",
			Type:              "Pi Agent Core",
			Version:           "v1.1.2",
			Tools:             []string{"chat_eval", "python_repl", "knowledge_query"},
			TCPAddr:           "127.0.0.1:41888",
			BinaryLinux:       "~/.local/bin/pi",
			BinaryWindows:     "~/.local/bin/pi.exe",
			StatusWhenRunning: "connected",
		},
		{
			ID:      PresetCodex,
			Name:    "Codex",
			Type:    "Codex Agent CLI",
			Version: "v1.2.0-draft",
			Tools:   []string{"sandbox_exec", "git_apply", "review", "web_search"},
			// The historical preset pinned an nvm-managed path; it is left
			// untouched because no cross-platform location is known.
			TCPAddr:           "127.0.0.1:48080",
			BinaryLinux:       "~/.config/nvm/versions/node/v24.16.0/bin/codex",
			StatusWhenRunning: "connected",
		},
		{
			ID:                PresetClaudeCode,
			Name:              "Claude Code CLI",
			Type:              "Terminal Agent Daemon",
			Version:           "v1.1.4",
			Tools:             []string{"bash", "glob", "grep", "file_edit"},
			TCPAddr:           "127.0.0.1:45124",
			BinaryLinux:       "~/.local/bin/claude",
			BinaryWindows:     "~/.local/bin/claude.exe",
			StatusWhenRunning: "connected",
		},
		{
			ID:      PresetCursor,
			Name:    "Cursor Editor Agent",
			Type:    "Editor Sidecar",
			Version: "v1.0.8",
			Tools:   []string{"lsp_diagnostics", "symbol_search", "diff_apply"},
			// Conventional install locations for the Cursor editor.
			TCPAddr:           "127.0.0.1:49200",
			BinaryLinux:       "/opt/Cursor/cursor",
			BinaryDarwin:      "/Applications/Cursor.app/Contents/MacOS/Cursor",
			BinaryWindows:     "%LOCALAPPDATA%/Programs/cursor/Cursor.exe",
			StatusWhenRunning: "connected",
		},
	}
}

// PresetByID returns the preset with the given identifier.
func PresetByID(id string) (Preset, bool) {
	for _, p := range Presets() {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// DefaultEndpoint returns the conventional endpoint for the preset on env.GOOS.
//
// macOS assumption: socket-based apps are expected to place their socket under
// ~/Library/Application Support/<app>/<name>-acp.sock, mirroring the Linux
// /run/user/<uid>/ layout. Windows has no unix sockets, so socket presets map to
// a named pipe (\\.\pipe\<name>-acp). TCP presets keep their historical ports on
// every OS.
func (p Preset) DefaultEndpoint(env Env) string {
	if p.TCPAddr != "" {
		return p.TCPAddr
	}
	if p.SocketName == "" {
		return ""
	}
	switch env.GOOS {
	case "windows":
		return `\\.\pipe\` + strings.TrimSuffix(p.SocketName, ".sock")
	case "darwin":
		dir := p.MacAppSupportDir
		if dir == "" {
			dir = p.Name
		}
		home := toSlash(env.Home)
		if home == "" {
			return "unix:///tmp/" + p.SocketName
		}
		return "unix://" + path.Join(home, "Library", "Application Support", dir, p.SocketName)
	default:
		dir := env.RunUserDir
		if dir == "" {
			dir = "/run/user/1000"
		}
		return "unix://" + path.Join(toSlash(dir), p.SocketName)
	}
}

// DefaultBinaryPath returns the conventional executable location for the preset
// on env.GOOS.
func (p Preset) DefaultBinaryPath(env Env) string {
	if p.ID == PresetAntigravity && env.AntigravityBinary != "" {
		return toSlash(env.AntigravityBinary)
	}
	raw := p.BinaryLinux
	switch env.GOOS {
	case "windows":
		if p.BinaryWindows != "" {
			raw = p.BinaryWindows
		}
	case "darwin":
		if p.BinaryDarwin != "" {
			raw = p.BinaryDarwin
		}
	}
	return expandPath(raw, env)
}

// expandPath expands the "~/" and "%LOCALAPPDATA%/" prefixes and normalises
// separators to forward slashes. These values are displayed and persisted, never
// executed by this package, so a stable representation is preferable to
// host-dependent separators.
func expandPath(raw string, env Env) string {
	raw = toSlash(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(raw, "~/"):
		home := toSlash(env.Home)
		if home == "" {
			return raw
		}
		return path.Join(home, strings.TrimPrefix(raw, "~/"))
	case strings.HasPrefix(raw, "%LOCALAPPDATA%/"):
		base := toSlash(env.LocalAppData)
		if base == "" {
			return raw
		}
		return path.Join(base, strings.TrimPrefix(raw, "%LOCALAPPDATA%/"))
	}
	return raw
}

func toSlash(s string) string {
	return strings.ReplaceAll(s, `\`, "/")
}

// Resolved is one registry entry after merging presets, overrides and custom
// agents. It contains no process state.
type Resolved struct {
	ID                string
	Name              string
	Type              string
	BinaryPath        string
	Endpoint          string
	Version           string
	Tools             []string
	Custom            bool
	OSDefault         bool
	StatusWhenRunning string
}

// Merge combines the built-in presets with the persisted agent configuration.
//
// Rules:
//   - a non-empty override field wins over the preset default;
//   - an override without an endpoint keeps the OS default endpoint and is
//     flagged OSDefault;
//   - entries flagged Custom are appended after the presets, in stored order;
//   - unknown non-custom ids (e.g. a preset removed in a later release) are
//     ignored rather than surfacing a card with no defaults.
func Merge(overrides []core.ACPAgentConfig, env Env) []Resolved {
	byID := make(map[string]core.ACPAgentConfig, len(overrides))
	for _, o := range overrides {
		id := strings.TrimSpace(o.ID)
		if id == "" {
			continue
		}
		byID[id] = o
	}

	presets := Presets()
	out := make([]Resolved, 0, len(presets)+len(overrides))
	seen := make(map[string]bool, len(presets))

	for _, p := range presets {
		o := byID[p.ID]
		seen[p.ID] = true

		r := Resolved{
			ID:                p.ID,
			Name:              firstNonEmpty(o.Name, p.Name),
			Type:              firstNonEmpty(o.Type, p.Type),
			BinaryPath:        firstNonEmpty(o.BinaryPath, p.DefaultBinaryPath(env)),
			Version:           firstNonEmpty(o.Version, p.Version),
			Tools:             append([]string(nil), p.Tools...),
			StatusWhenRunning: p.StatusWhenRunning,
		}
		if len(o.Tools) > 0 {
			r.Tools = append([]string(nil), o.Tools...)
		}
		if endpoint := strings.TrimSpace(o.Endpoint); endpoint != "" {
			r.Endpoint = endpoint
		} else {
			r.Endpoint = p.DefaultEndpoint(env)
			r.OSDefault = true
		}
		out = append(out, r)
	}

	for _, o := range overrides {
		id := strings.TrimSpace(o.ID)
		if id == "" || seen[id] || !o.Custom {
			continue
		}
		endpoint := strings.TrimSpace(o.Endpoint)
		if endpoint == "" {
			// A custom agent without an endpoint is not addressable; skip it
			// instead of rendering a permanently broken card.
			continue
		}
		seen[id] = true
		out = append(out, Resolved{
			ID:         id,
			Name:       firstNonEmpty(o.Name, id),
			Type:       firstNonEmpty(o.Type, "Custom Agent"),
			BinaryPath: strings.TrimSpace(o.BinaryPath),
			Endpoint:   endpoint,
			Version:    firstNonEmpty(o.Version, "unknown"),
			// Keep the slice non-nil so the API always serialises [] rather than
			// null; the frontend reads .length unconditionally.
			Tools:             append([]string{}, o.Tools...),
			Custom:            true,
			StatusWhenRunning: "connected",
		})
	}

	return out
}

// AgentInput is the request body accepted by POST /api/utilities/acp/agents.
type AgentInput struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	BinaryPath string   `json:"binary_path"`
	Endpoint   string   `json:"endpoint"`
	Version    string   `json:"acp_version"`
	Tools      []string `json:"tools"`
}

// NormalizeAgentInput validates an editor submission and converts it into a
// persisted entry. existingIDs must contain the preset ids plus every stored id.
//
// The returned bool reports whether the entry is a custom agent (true) or a
// preset override (false).
func NormalizeAgentInput(in AgentInput, existingIDs map[string]bool) (core.ACPAgentConfig, bool, error) {
	name := strings.TrimSpace(in.Name)
	endpoint := strings.TrimSpace(in.Endpoint)

	if err := ValidateEndpoint(endpoint); err != nil {
		return core.ACPAgentConfig{}, false, err
	}

	id := strings.TrimSpace(in.ID)
	_, isPreset := PresetByID(id)
	custom := !isPreset
	if custom && name == "" {
		return core.ACPAgentConfig{}, false, fmt.Errorf("name is required when adding a new agent")
	}
	// A known custom id is an edit: keep it. An empty or unknown id creates a
	// fresh custom agent with a generated identifier.
	if custom && (id == "" || !existingIDs[id]) {
		id = GenerateCustomID(name, existingIDs)
	}

	return core.ACPAgentConfig{
		ID:         id,
		Name:       name,
		Type:       strings.TrimSpace(in.Type),
		BinaryPath: strings.TrimSpace(in.BinaryPath),
		Endpoint:   endpoint,
		Version:    strings.TrimSpace(in.Version),
		Tools:      NormalizeTools(in.Tools),
		Custom:     custom,
	}, custom, nil
}

// NormalizeTools trims entries, drops empties and de-duplicates while keeping
// the submitted order.
func NormalizeTools(tools []string) []string {
	if len(tools) == 0 {
		return nil
	}
	out := make([]string, 0, len(tools))
	seen := make(map[string]bool, len(tools))
	for _, t := range tools {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// GenerateCustomID builds a stable "custom-<slug>" identifier that does not
// collide with taken.
func GenerateCustomID(name string, taken map[string]bool) string {
	base := "custom-" + Slugify(name)
	if base == "custom-" {
		base = "custom-agent"
	}
	id := base
	for i := 2; taken[id]; i++ {
		id = base + "-" + strconv.Itoa(i)
	}
	return id
}

// Slugify lowercases a display name and keeps only [a-z0-9], collapsing every
// other run into a single dash.
func Slugify(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// RemoveAgent drops the stored entry for id. For a preset id this resets the
// preset to its OS default; for a custom id it removes the agent entirely. The
// bool reports whether an entry was removed.
func RemoveAgent(overrides []core.ACPAgentConfig, id string) ([]core.ACPAgentConfig, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return overrides, false
	}
	out := make([]core.ACPAgentConfig, 0, len(overrides))
	removed := false
	for _, o := range overrides {
		if strings.TrimSpace(o.ID) == id {
			removed = true
			continue
		}
		out = append(out, o)
	}
	return out, removed
}

// UpsertAgent replaces the stored entry with the same id or appends it.
func UpsertAgent(overrides []core.ACPAgentConfig, entry core.ACPAgentConfig) []core.ACPAgentConfig {
	out := make([]core.ACPAgentConfig, 0, len(overrides)+1)
	replaced := false
	for _, o := range overrides {
		if strings.TrimSpace(o.ID) == entry.ID {
			out = append(out, entry)
			replaced = true
			continue
		}
		out = append(out, o)
	}
	if !replaced {
		out = append(out, entry)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
