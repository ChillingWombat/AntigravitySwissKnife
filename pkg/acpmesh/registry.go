package acpmesh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// DefaultACPRegistryURL is the official CDN URL for the ACP registry.
const DefaultACPRegistryURL = "https://cdn.agentclientprotocol.com/registry/v1/latest/registry.json"

// RemoteACPRegistry models the top-level registry document.
type RemoteACPRegistry struct {
	Version string           `json:"version"`
	Agents  []RemoteACPAgent `json:"agents"`
}

// RemoteACPAgent models one agent entry in the remote ACP registry.
type RemoteACPAgent struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description,omitempty"`
	Website      string                 `json:"website,omitempty"`
	Distribution map[string]interface{} `json:"distribution,omitempty"`
}

// FetchRemoteRegistry downloads and parses the ACP registry from registryURL.
func FetchRemoteRegistry(ctx context.Context, registryURL string) (*RemoteACPRegistry, error) {
	if strings.TrimSpace(registryURL) == "" {
		registryURL = DefaultACPRegistryURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create registry request: %w", err)
	}
	req.Header.Set("User-Agent", "antigravity-swiss-knife/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB safety cap
	if err != nil {
		return nil, fmt.Errorf("read registry body: %w", err)
	}

	var reg RemoteACPRegistry
	if err := json.Unmarshal(body, &reg); err != nil {
		return nil, fmt.Errorf("decode registry json: %w", err)
	}
	return &reg, nil
}

// MatchRegistryAgentToPreset finds the matching local preset ID for a remote agent.
func MatchRegistryAgentToPreset(reg RemoteACPAgent) (string, bool) {
	regID := strings.ToLower(strings.TrimSpace(reg.ID))
	switch regID {
	case "antigravity-acp", "agent-antigravity-acp", "antigravity":
		return PresetAntigravityACP, true
	case "claude-acp", "claude-code", "agent-claude-code":
		return PresetClaudeCode, true
	case "codex-acp", "codex", "agent-codex":
		return PresetCodex, true
	case "cursor", "agent-cursor":
		return PresetCursor, true
	case "devin", "agent-devin":
		return PresetDevin, true
	case "opencode", "opencode-v2", "agent-opencode":
		return PresetOpenCode, true
	case "pi-acp", "pi", "agent-pi":
		return PresetPi, true
	case "deepseek", "deepseek-harness", "agent-deepseek-harness":
		return PresetDeepSeek, true
	default:
		for _, p := range Presets() {
			if strings.EqualFold(p.ID, regID) || strings.EqualFold(strings.TrimPrefix(p.ID, "agent-"), regID) {
				return p.ID, true
			}
		}
		return "", false
	}
}

// SyncRegistry fetches the ACP registry and updates preset settings within overrides.
func SyncRegistry(ctx context.Context, registryURL string, currentOverrides []core.ACPAgentConfig) ([]core.ACPAgentConfig, int, error) {
	reg, err := FetchRemoteRegistry(ctx, registryURL)
	if err != nil {
		return nil, 0, err
	}

	overridesMap := make(map[string]core.ACPAgentConfig, len(currentOverrides))
	for _, o := range currentOverrides {
		overridesMap[strings.TrimSpace(o.ID)] = o
	}

	updatedCount := 0
	for _, regAgent := range reg.Agents {
		presetID, matched := MatchRegistryAgentToPreset(regAgent)
		if !matched {
			continue
		}

		version := strings.TrimSpace(regAgent.Version)
		if version != "" && !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		existing, exists := overridesMap[presetID]
		if !exists {
			existing = core.ACPAgentConfig{
				ID: presetID,
			}
		}

		changed := false
		if version != "" && existing.Version != version {
			existing.Version = version
			changed = true
		}

		if changed || !exists {
			overridesMap[presetID] = existing
			updatedCount++
		}
	}

	// Rebuild overrides keeping presets first in standard order, then custom agents
	presets := Presets()
	result := make([]core.ACPAgentConfig, 0, len(overridesMap))
	handled := make(map[string]bool, len(overridesMap))

	for _, p := range presets {
		if o, ok := overridesMap[p.ID]; ok {
			result = append(result, o)
			handled[p.ID] = true
		}
	}

	for _, o := range currentOverrides {
		if !handled[o.ID] {
			if override, ok := overridesMap[o.ID]; ok {
				result = append(result, override)
			} else {
				result = append(result, o)
			}
			handled[o.ID] = true
		}
	}

	return result, updatedCount, nil
}
