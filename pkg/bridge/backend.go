package bridge

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Agent backends. The bridge can drive either the Antigravity 2.0 desktop app
// (over the devtools protocol) or the Antigravity CLI. The user picks which one
// with Config.AgentBackend; "auto" prefers the CLI because a short-lived process
// costs less than keeping the GUI on the critical path.
const (
	AgentAuto    = "auto"
	AgentApp     = "app"
	AgentCLI     = "cli"
	agentUnknown = ""
)

// normalizeAgentBackend maps any accepted spelling onto a backend constant.
func normalizeAgentBackend(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "cli", "agy", "antigravity_cli", "antigravity-cli":
		return AgentCLI
	case "app", "gui", "desktop", "antigravity", "antigravity_2", "antigravity-2":
		return AgentApp
	case "", "auto", "automatic", "default":
		return AgentAuto
	}
	return ""
}

// agentBackend is the surface every lead / session-model tool needs. Both the
// desktop-app handler and the CLI handler satisfy it, so ToolManifest and
// CallTool stay backend-agnostic.
type agentBackend interface {
	Name() string
	CreateConversation(args map[string]interface{}) (string, error)
	SendMessage(args map[string]interface{}) (string, error)
	GetStatus(args map[string]interface{}) (string, error)
	AwaitResponse(args map[string]interface{}) (string, error)
	ListConversations(args map[string]interface{}) (string, error)
	CompactConversation(args map[string]interface{}) (string, error)
	StopConversation(args map[string]interface{}) (string, error)
	ListModels(args map[string]interface{}) (string, error)
	GetModel(args map[string]interface{}) (string, error)
	SetModel(args map[string]interface{}) (string, error)
}

// appBackend adapts the desktop-app handlers to the backend interface.
type appBackend struct {
	svc  *Service
	lead *LeadHandler
}

func (b *appBackend) Name() string { return AgentApp }

func (b *appBackend) CreateConversation(args map[string]interface{}) (string, error) {
	return b.lead.CreateConversation(args)
}

func (b *appBackend) SendMessage(args map[string]interface{}) (string, error) {
	return b.lead.SendMessage(args)
}

func (b *appBackend) GetStatus(args map[string]interface{}) (string, error) {
	return b.lead.GetStatus(args)
}

func (b *appBackend) AwaitResponse(args map[string]interface{}) (string, error) {
	return b.lead.AwaitResponse(args)
}

func (b *appBackend) ListConversations(args map[string]interface{}) (string, error) {
	return b.lead.ListConversations(args)
}

func (b *appBackend) CompactConversation(args map[string]interface{}) (string, error) {
	return b.lead.CompactConversation(args)
}

func (b *appBackend) StopConversation(args map[string]interface{}) (string, error) {
	return b.lead.StopConversation(args)
}

func (b *appBackend) ListModels(args map[string]interface{}) (string, error) {
	return b.svc.SessionModelList(args)
}

func (b *appBackend) GetModel(args map[string]interface{}) (string, error) {
	return b.svc.SessionModelGet(args)
}

func (b *appBackend) SetModel(args map[string]interface{}) (string, error) {
	return b.svc.SessionModelSet(args)
}

// cliBackend adapts the CLI handler to the backend interface.
type cliBackend struct {
	svc *Service
	cli *CLIHandler
}

func (b *cliBackend) Name() string { return AgentCLI }

func (b *cliBackend) CreateConversation(args map[string]interface{}) (string, error) {
	return b.cli.CreateConversation(args)
}

func (b *cliBackend) SendMessage(args map[string]interface{}) (string, error) {
	return b.cli.SendMessage(args)
}

func (b *cliBackend) GetStatus(args map[string]interface{}) (string, error) {
	return b.cli.GetStatus(args)
}

func (b *cliBackend) AwaitResponse(args map[string]interface{}) (string, error) {
	return b.cli.AwaitResponse(args)
}

func (b *cliBackend) ListConversations(args map[string]interface{}) (string, error) {
	return b.cli.ListConversations(args)
}

func (b *cliBackend) CompactConversation(args map[string]interface{}) (string, error) {
	return b.cli.CompactConversation(args)
}

func (b *cliBackend) StopConversation(args map[string]interface{}) (string, error) {
	return b.cli.StopConversation(args)
}

func (b *cliBackend) ListModels(args map[string]interface{}) (string, error) {
	return b.svc.SessionModelListCLI(args)
}

func (b *cliBackend) GetModel(args map[string]interface{}) (string, error) {
	return b.svc.SessionModelGetCLI(args)
}

func (b *cliBackend) SetModel(args map[string]interface{}) (string, error) {
	return b.svc.SessionModelSetCLI(args)
}

// RequestedAgent returns the configured backend preference.
func (s *Service) RequestedAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeAgentBackend(s.config.AgentBackend)
}

// resolveAgentBackend maps a preference plus CLI availability onto the backend
// that will actually run. Pure so callers already holding a lock can use it.
func resolveAgentBackend(requested string, cliReady bool) string {
	switch normalizeAgentBackend(requested) {
	case AgentApp:
		return AgentApp
	case AgentCLI:
		if cliReady {
			return AgentCLI
		}
		// Explicitly requested but missing: fall back rather than failing every
		// tool call, and let GetStatus report the mismatch.
		return AgentApp
	}
	if cliReady {
		return AgentCLI
	}
	return AgentApp
}

// ResolvedAgent returns which backend a lead / model tool will actually use,
// considering availability. "auto" prefers the CLI, per configuration policy.
func (s *Service) ResolvedAgent() string {
	s.mu.RLock()
	requested := s.config.AgentBackend
	s.mu.RUnlock()
	return resolveAgentBackend(requested, s.cliAvailable())
}

// cliAvailable reports whether the CLI binary was found.
func (s *Service) cliAvailable() bool {
	return s.cliHdl != nil && s.cliHdl.Available()
}

// cliGatewayEnv points the CLI at the local LLM gateway when the requested model
// is a Swiss Knife custom model. Native models return ok=false and keep using
// Google directly.
func (s *Service) cliGatewayEnv(modelID string) (map[string]string, string, bool) {
	trimmed := strings.TrimSpace(modelID)
	if trimmed == "" {
		return nil, "", false
	}
	refs := s.customModelList()
	if len(refs) == 0 {
		return nil, "", false
	}
	canonical := ""
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
		if strings.EqualFold(ref.ID, trimmed) || strings.EqualFold(ref.Name, trimmed) || strings.EqualFold(ref.Label, trimmed) {
			canonical = ref.ID
		}
	}
	if canonical == "" {
		return nil, "", false
	}
	token := s.gatewayToken()
	if token == "" {
		return nil, "", false
	}
	port := s.GetConfig().Port
	if port <= 0 {
		port = 8765
	}
	return map[string]string{
		"AGY_LLM_GATEWAY_URL":     fmt.Sprintf("http://127.0.0.1:%d", port),
		"AGY_LLM_GATEWAY_API_KEY": token,
		"AGY_LLM_GATEWAY_MODELS":  strings.Join(ids, ","),
	}, canonical, true
}

// agent returns the resolved backend implementation.
func (s *Service) agent() agentBackend {
	if s.ResolvedAgent() == AgentCLI {
		return &cliBackend{svc: s, cli: s.cliHdl}
	}
	return &appBackend{svc: s, lead: s.leadHdl}
}

// SetAgentBackend stores the preferred backend ("auto", "app" or "cli").
func (s *Service) SetAgentBackend(value string) (string, error) {
	normalized := normalizeAgentBackend(value)
	if normalized == agentUnknown {
		return "", fmt.Errorf("unsupported agent backend %q: use auto, app or cli", value)
	}
	if normalized == AgentCLI && (s.cliHdl == nil || !s.cliHdl.Available()) {
		return "", fmt.Errorf("Antigravity CLI not found: install it or add agy to PATH before selecting it")
	}
	s.mu.Lock()
	s.config.AgentBackend = normalized
	err := s.saveConfig()
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	return normalized, nil
}

// CustomModelRef is one Swiss Knife custom model as the bridge sees it.
type CustomModelRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	Provider string `json:"provider,omitempty"`
}

// customModelProvider lists enabled custom models. Replaced in tests.
type customModelProvider func() []CustomModelRef

// customModelActivator selects a custom model in the desktop app. Replaced in tests.
type customModelActivator func(modelID string) error

func marshalPayload(payload map[string]interface{}) string {
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data)
}
