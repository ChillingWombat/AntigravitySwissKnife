package bridge

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
)

// The model tools present ONE list to Gemini: Antigravity's native models and the
// Swiss Knife custom models, tagged by kind. A native pick goes through
// Antigravity's own selector (or `--model` on the CLI); a custom pick activates
// the Swiss Knife custom model for the session. Nothing is bound to a project.

// modelOption is one selectable model as returned to the client.
type modelOption struct {
	Kind              string      `json:"kind"` // "native" | "custom"
	Label             string      `json:"label"`
	ID                string      `json:"id,omitempty"`    // custom model id, or the CLI slug
	Value             interface{} `json:"value,omitempty"` // native numeric id
	Provider          string      `json:"provider,omitempty"`
	Disabled          bool        `json:"disabled"`
	Selected          bool        `json:"selected"`
	Remaining         *float64    `json:"remaining_fraction,omitempty"`
	Effort            string      `json:"effort,omitempty"`
	Agent             string      `json:"agent"` // backend that can serve this option
	UnavailableReason string      `json:"unavailable_reason,omitempty"`
}

// defaultCustomModels loads the enabled custom models from the daemon store.
func defaultCustomModels() []CustomModelRef {
	store, err := custommodels.NewStore("")
	if err != nil || store == nil {
		return nil
	}
	cfg := store.GetConfig()
	refs := make([]CustomModelRef, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		if !m.Enabled {
			continue
		}
		label := m.DisplayName
		if label == "" {
			label = m.Name
		}
		refs = append(refs, CustomModelRef{
			ID:       m.ID,
			Name:     m.Name,
			Label:    label,
			Provider: string(m.ProviderType),
		})
	}
	return refs
}

// gatewayToken returns the local LLM gateway bearer token, or "" when the store
// is unavailable. Tests replace it through SetGatewayTokenSource.
func (s *Service) gatewayToken() string {
	s.mu.RLock()
	custom := s.gatewayTokenFn
	s.mu.RUnlock()
	if custom != nil {
		return custom()
	}
	store, err := custommodels.NewStore("")
	if err != nil || store == nil {
		return ""
	}
	return store.GatewayToken()
}

// SetGatewayTokenSource overrides where the gateway bearer token comes from.
func (s *Service) SetGatewayTokenSource(provider func() string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gatewayTokenFn = provider
}

// activeCustomModelID reports the custom model the daemon has marked active.
func (s *Service) activeCustomModelID() string {
	store, err := custommodels.NewStore("")
	if err != nil || store == nil {
		return ""
	}
	return strings.TrimSpace(store.GetConfig().ActiveModelID)
}

// customModelList returns enabled custom models through the injectable provider.
func (s *Service) customModelList() []CustomModelRef {
	provider := s.customModels
	if provider == nil {
		provider = defaultCustomModels
	}
	return provider()
}

// SetCustomModelSources overrides how custom models are listed and selected.
// Used by tests and by the desktop app path.
func (s *Service) SetCustomModelSources(provider customModelProvider, activator customModelActivator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customModels = provider
	s.activateCustom = activator
}

// activateCustomModelInApp selects a custom model inside the running app, so the
// model pill and the next turn both use it.
func (s *Service) activateCustomModelInApp(modelID string) error {
	if s.activateCustom != nil {
		return s.activateCustom(modelID)
	}
	modelJSON, _ := json.Marshal(modelID)
	result, err := s.runPageScript(fmt.Sprintf(`(() => {
		if (typeof window.__swissSetCustomModel !== "function") {
			return { success: false, reason: "custom_model_selector_unavailable" };
		}
		return window.__swissSetCustomModel(%s);
	})()`, string(modelJSON)))
	if err != nil {
		return err
	}
	if success, ok := result["success"].(bool); !ok || !success {
		reason, _ := result["reason"].(string)
		if reason == "" {
			reason = "the app did not accept the custom model"
		}
		return fmt.Errorf("%s", reason)
	}
	return nil
}

// SessionModelListCLI lists the models the CLI can drive.
func (s *Service) SessionModelListCLI(args map[string]interface{}) (string, error) {
	if s.cliHdl == nil || !s.cliHdl.Available() {
		return "", fmt.Errorf("Antigravity CLI not found: install it or add agy to PATH")
	}
	models, err := s.cliHdl.ListModels()
	if err != nil {
		return "", err
	}
	conversation := argString(args, "conversation_id", "")
	current := s.cliHdl.CurrentModel(conversation)

	options := make([]modelOption, 0, len(models))
	for _, m := range models {
		options = append(options, modelOption{
			Kind:     "native",
			Label:    m.Label,
			ID:       m.Slug,
			Selected: current != "" && m.Slug == current,
			Agent:    AgentCLI,
		})
	}
	// Custom models are served to the CLI by the local LLM gateway
	// (AGY_LLM_GATEWAY_URL), so they are selectable here too.
	for _, ref := range s.customModelList() {
		options = append(options, modelOption{
			Kind:     "custom",
			Label:    ref.Label,
			ID:       ref.ID,
			Provider: ref.Provider,
			Agent:    AgentCLI,
			Selected: current != "" && strings.EqualFold(current, ref.ID),
		})
	}
	return marshalPayload(map[string]interface{}{
		"status":       "ok",
		"agent":        AgentCLI,
		"conversation": conversation,
		"count":        len(options),
		"options":      options,
	}), nil
}

// SessionModelGetCLI reports the CLI model used for the next turn.
func (s *Service) SessionModelGetCLI(args map[string]interface{}) (string, error) {
	if s.cliHdl == nil || !s.cliHdl.Available() {
		return "", fmt.Errorf("Antigravity CLI not found: install it or add agy to PATH")
	}
	conversation := argString(args, "conversation_id", "")
	current := s.cliHdl.CurrentModel(conversation)
	model := map[string]interface{}{"kind": "native", "id": current, "label": current}
	if current == "" {
		model = map[string]interface{}{"kind": "default", "id": "", "label": "Antigravity CLI default"}
	}
	return marshalPayload(map[string]interface{}{
		"status":       "ok",
		"agent":        AgentCLI,
		"conversation": conversation,
		"model":        model,
	}), nil
}

// SessionModelSetCLI records the model the next CLI turn will pass as --model.
func (s *Service) SessionModelSetCLI(args map[string]interface{}) (string, error) {
	if s.cliHdl == nil || !s.cliHdl.Available() {
		return "", fmt.Errorf("Antigravity CLI not found: install it or add agy to PATH")
	}
	requested := argString(args, "model", "")
	if requested == "" {
		requested = argString(args, "label", "")
	}
	conversation := argString(args, "conversation_id", "")

	// Custom models are served by the local LLM gateway: remember the id and the
	// next CLI turn runs against it.
	for _, ref := range s.customModelList() {
		if !modelRefMatches(ref, requested, args) {
			continue
		}
		if token := s.gatewayToken(); token == "" {
			return "", fmt.Errorf("custom model %q needs the local LLM gateway, which is not ready yet: retry after the daemon finishes starting", ref.Label)
		}
		if err := s.cliHdl.setModel(conversation, ref.ID); err != nil {
			return "", err
		}
		return marshalPayload(map[string]interface{}{
			"status":       "ok",
			"agent":        AgentCLI,
			"conversation": conversation,
			"model":        map[string]interface{}{"kind": "custom", "id": ref.ID, "label": ref.Label},
			"applies_to":   "the next CLI turn, served by the local LLM gateway",
		}), nil
	}

	slug, err := s.cliHdl.resolveModelSlug(requested)
	if err != nil {
		// Allow an explicit numeric/effort-only request to fall back to the list.
		if effort := argString(args, "effort", ""); effort != "" {
			models, listErr := s.cliHdl.ListModels()
			if listErr == nil {
				for _, m := range models {
					if strings.EqualFold(strings.TrimSpace(effort), effortSuffix(m.Slug)) {
						slug = m.Slug
						break
					}
				}
			}
		}
		if slug == "" {
			return "", err
		}
	}
	if err := s.cliHdl.setModel(conversation, slug); err != nil {
		return "", err
	}
	return marshalPayload(map[string]interface{}{
		"status":       "ok",
		"agent":        AgentCLI,
		"conversation": conversation,
		"model":        map[string]interface{}{"kind": "native", "id": slug, "label": slug},
		"applies_to":   "the next CLI turn",
	}), nil
}

// effortSuffix returns "high"/"medium"/... from a model slug.
func effortSuffix(slug string) string {
	lower := strings.ToLower(slug)
	for _, level := range []string{"xhigh", "max", "high", "medium", "low"} {
		if strings.HasSuffix(lower, "-"+level) {
			return level
		}
	}
	return ""
}

// modelRefMatches reports whether a request refers to this custom model.
func modelRefMatches(ref CustomModelRef, requested string, args map[string]interface{}) bool {
	wanted := strings.ToLower(strings.TrimSpace(requested))
	if wanted != "" {
		if wanted == strings.ToLower(ref.ID) || wanted == strings.ToLower(ref.Name) || wanted == strings.ToLower(ref.Label) {
			return true
		}
	}
	if customID := argString(args, "custom_model_id", ""); customID != "" {
		return strings.EqualFold(customID, ref.ID)
	}
	return false
}

// unifiedModelOptions builds the single list Gemini sees for the app backend:
// native models from Antigravity plus enabled Swiss Knife custom models.
func (s *Service) unifiedModelOptions(nativeOptions []interface{}, current map[string]interface{}) []modelOption {
	options := make([]modelOption, 0, len(nativeOptions)+4)
	for _, raw := range nativeOptions {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		option := modelOption{Kind: "native", Agent: AgentApp}
		option.Label, _ = entry["label"].(string)
		option.Value = entry["value"]
		option.Disabled, _ = entry["disabled"].(bool)
		option.Selected, _ = entry["selected"].(bool)
		if remaining, ok := entry["remaining_fraction"].(float64); ok {
			value := remaining
			option.Remaining = &value
		}
		options = append(options, option)
	}

	customCurrent := ""
	if current != nil {
		if id, ok := current["custom_model_id"].(string); ok {
			customCurrent = id
		}
	}
	for _, ref := range s.customModelList() {
		options = append(options, modelOption{
			Kind:     "custom",
			Label:    ref.Label,
			ID:       ref.ID,
			Provider: ref.Provider,
			Selected: customCurrent != "" && strings.EqualFold(customCurrent, ref.ID),
			Agent:    AgentApp,
		})
	}
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Kind != options[j].Kind {
			return options[i].Kind == "native"
		}
		return false
	})
	return options
}
