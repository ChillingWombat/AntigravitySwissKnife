package bridge

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// Session model tools drive Antigravity's own model selector state for the
// conversation that is open right now. The app keeps the choice in a React
// context (selectedModel + modelOptions), so the bridge reads and sets it through
// that context instead of clicking the dropdown: no focus stealing, no race with
// the popover, and the selection is the same one the next turn is sent with.

// sessionModelContextScript locates the model context in the page and exposes
// list/get/set helpers as window.__swissSessionModel. Idempotent: re-evaluating
// it re-installs the helpers.
const sessionModelContextScript = `(() => {
  const readContext = () => {
    const rootEl = document.getElementById('root') || document.body;
    const containerKey = Object.keys(rootEl).find(k => k.startsWith('__reactContainer$')) || Object.keys(rootEl).find(k => k.startsWith('__reactFiber$'));
    if (!containerKey) return null;
    let start = rootEl[containerKey];
    if (start && start.stateNode && start.stateNode.current) start = start.stateNode.current;
    else if (start && start.current) start = start.current;
    const seen = new Set();
    const stack = [start];
    let visited = 0;
    while (stack.length && visited < 60000) {
      const fiber = stack.pop();
      if (!fiber || seen.has(fiber)) continue;
      seen.add(fiber);
      visited++;
      const value = fiber.memoizedProps && fiber.memoizedProps.value;
      if (value && typeof value === 'object' && value.selectedModel && typeof value.setSelectedModel === 'function') {
        return value;
      }
      if (fiber.child) stack.push(fiber.child);
      if (fiber.sibling) stack.push(fiber.sibling);
    }
    return null;
  };

  const flattenOptions = (ctx) => {
    const out = [];
    const push = (opt) => {
      if (!opt || typeof opt !== 'object') return;
      if (opt.options && Array.isArray(opt.options)) { opt.options.forEach(push); return; }
      if (opt.options && typeof opt.options === 'object') { Object.values(opt.options).forEach(push); return; }
      if (opt.label !== undefined && opt.value !== undefined) out.push(opt);
    };
    if (Array.isArray(ctx.modelOptions)) ctx.modelOptions.forEach(push);
    if (Array.isArray(ctx.modelSorts)) {
      ctx.modelSorts.forEach(sort => {
        if (!sort) return;
        if (Array.isArray(sort.groups)) sort.groups.forEach(g => { if (g && Array.isArray(g.options)) g.options.forEach(push); });
        if (Array.isArray(sort.options)) sort.options.forEach(push);
      });
    }
    const seen = new Set();
    return out.filter(o => {
      const key = String(o.value) + '|' + String(o.label);
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  };

  const summarize = (opt) => {
    if (!opt) return null;
    const quota = opt.quotaInfo || null;
    return {
      label: opt.label === undefined ? null : String(opt.label),
      value: opt.value === undefined ? null : opt.value,
      disabled: Boolean(opt.disabled),
      remaining_fraction: quota && typeof quota.remainingFraction === 'number' ? quota.remainingFraction : null,
      tag: opt.tagTitle || null
    };
  };

  window.__swissSessionModel = {
    available() { return Boolean(readContext()); },
    current() {
      const ctx = readContext();
      if (!ctx) return null;
      return summarize(ctx.selectedModel);
    },
    list() {
      const ctx = readContext();
      if (!ctx) return null;
      const current = summarize(ctx.selectedModel);
      return flattenOptions(ctx).map(o => {
        const item = summarize(o);
        item.selected = Boolean(current && String(current.value) === String(item.value));
        return item;
      });
    },
    async apply(label, value, effort) {
      const result = window.__swissSessionModel.set(label, value, effort);
      if (!result || !result.success) return result;
      const wanted = result.chosen ? result.chosen.value : null;
      // React commits the selection on the next render, so poll for the new value
      // instead of reading the context in the same tick.
      for (let attempt = 0; attempt < 15; attempt++) {
        await new Promise(r => setTimeout(r, 120));
        const current = window.__swissSessionModel.current();
        if (current && wanted !== null && String(current.value) === String(wanted)) {
          result.after = current;
          result.applied = true;
          return result;
        }
      }
      result.after = window.__swissSessionModel.current();
      result.applied = false;
      return result;
    },
    set(label, value, effort) {
      const ctx = readContext();
      if (!ctx) return { success: false, reason: 'model_selector_not_found' };
      const options = flattenOptions(ctx);
      const wantedLabel = (label === null || label === undefined) ? '' : String(label).trim().toLowerCase();
      const wantedEffort = (effort === null || effort === undefined) ? '' : String(effort).trim().toLowerCase();
      let chosen = null;
      if (value !== null && value !== undefined && String(value).trim() !== '') {
        chosen = options.find(o => String(o.value) === String(value).trim()) || null;
      }
      if (!chosen && wantedLabel) {
        chosen = options.find(o => String(o.label || '').trim().toLowerCase() === wantedLabel) || null;
      }
      if (!chosen && wantedLabel) {
        const partial = options.filter(o => String(o.label || '').trim().toLowerCase().includes(wantedLabel));
        if (wantedEffort) {
          chosen = partial.find(o => String(o.label || '').toLowerCase().includes('(' + wantedEffort + ')')) || null;
        }
        if (!chosen) chosen = partial[0] || null;
      }
      if (!chosen) {
        return {
          success: false,
          reason: 'no_matching_model',
          requested: { label: label || null, value: value === undefined ? null : value, effort: effort || null },
          options: flattenOptions(ctx).map(summarize)
        };
      }
      if (chosen.disabled) {
        return { success: false, reason: 'model_disabled', model: summarize(chosen) };
      }
      const before = summarize(ctx.selectedModel);
      const chosenSummary = summarize(chosen);
      ctx.setSelectedModel(chosen);
      return {
        success: true,
        requested: { label: label || null, value: value === undefined ? null : value, effort: effort || null },
        before,
        chosen: chosenSummary
      };
    }
  };
  return { installed: true, available: window.__swissSessionModel.available(), current: window.__swissSessionModel.current() };
})()`

// activePageTarget returns the Antigravity conversation page target.
func (s *Service) activePageTarget() (int, gui.DevToolsTarget, error) {
	injector := s.injector
	if injector == nil {
		return 0, gui.DevToolsTarget{}, fmt.Errorf("DevTools injector is not configured")
	}
	port, err := injector.FindDevToolsPort()
	if err != nil || port <= 0 {
		return 0, gui.DevToolsTarget{}, fmt.Errorf("could not connect to the running Antigravity desktop app: %w", err)
	}
	pages, err := injector.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		return 0, gui.DevToolsTarget{}, fmt.Errorf("no Antigravity window target found: %w", err)
	}
	target := pages[0]
	for _, page := range pages {
		if strings.HasPrefix(page.URL, "https://127.0.0.1:") || strings.HasPrefix(page.URL, "http://127.0.0.1:") ||
			strings.HasPrefix(page.URL, "https://localhost:") || strings.HasPrefix(page.URL, "http://localhost:") {
			target = page
			break
		}
	}
	return port, target, nil
}

// runPageScript evaluates one expression in the active Antigravity conversation
// page.
func (s *Service) runPageScript(expression string) (map[string]interface{}, error) {
	_, target, err := s.activePageTarget()
	if err != nil {
		return nil, err
	}
	return s.injector.ExecuteScript(target.WebSocketDebuggerURL, expression)
}

// runSessionModelScript installs the helpers, then evaluates one expression that
// uses them.
func (s *Service) runSessionModelScript(expression string) (map[string]interface{}, error) {
	_, target, err := s.activePageTarget()
	if err != nil {
		return nil, err
	}
	if _, err := s.injector.ExecuteScript(target.WebSocketDebuggerURL, sessionModelContextScript); err != nil {
		return nil, fmt.Errorf("locate the Antigravity model selector: %w", err)
	}
	result, err := s.injector.ExecuteScript(target.WebSocketDebuggerURL, expression)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SessionModelGet reports the model the open conversation will use next turn.
func (s *Service) SessionModelGet(args map[string]interface{}) (string, error) {
	result, err := s.runSessionModelScript(`(() => {
		if (!window.__swissSessionModel) return { available: false };
		const current = window.__swissSessionModel.current();
		return { available: window.__swissSessionModel.available(), current, conversation: (location.pathname.match(/\/c\/([^/?#]+)/) || [null, null])[1] };
	})()`)
	if err != nil {
		return "", err
	}
	if available, ok := result["available"].(bool); ok && !available {
		return "", fmt.Errorf("Antigravity's model selector is not mounted in the active window")
	}
	payload := map[string]interface{}{
		"status":       "ok",
		"agent":        AgentApp,
		"conversation": result["conversation"],
		"model":        result["current"],
	}
	if active := s.activeCustomModelID(); active != "" {
		payload["active_custom_model"] = active
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

// SessionModelList reports every model the open conversation can switch to.
func (s *Service) SessionModelList(args map[string]interface{}) (string, error) {
	result, err := s.runSessionModelScript(`(() => {
		if (!window.__swissSessionModel) return { available: false };
		return { available: window.__swissSessionModel.available(), options: window.__swissSessionModel.list(), conversation: (location.pathname.match(/\/c\/([^/?#]+)/) || [null, null])[1] };
	})()`)
	if err != nil {
		return "", err
	}
	if available, ok := result["available"].(bool); ok && !available {
		return "", fmt.Errorf("Antigravity's model selector is not mounted in the active window")
	}
	nativeOptions, _ := result["options"].([]interface{})
	merged := s.unifiedModelOptions(nativeOptions, nil)
	payload := map[string]interface{}{
		"status":       "ok",
		"agent":        AgentApp,
		"conversation": result["conversation"],
		"count":        len(merged),
		"options":      merged,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data), nil
}

// SessionModelSet switches the model for the open conversation. The request may
// name a native Antigravity model or a Swiss Knife custom model; both live in one
// list, so Gemini never needs to know which surface serves which model.
func (s *Service) SessionModelSet(args map[string]interface{}) (string, error) {
	label := argString(args, "model", "")
	if label == "" {
		label = argString(args, "label", "")
	}
	if customID := argString(args, "custom_model_id", ""); customID != "" && label == "" {
		label = customID
	}

	// Custom models: activate through the Swiss Knife selector.
	for _, ref := range s.customModelList() {
		if !modelRefMatches(ref, label, args) {
			continue
		}
		if err := s.activateCustomModelInApp(ref.ID); err != nil {
			return "", fmt.Errorf("activate custom model %q: %w", ref.Label, err)
		}
		return marshalPayload(map[string]interface{}{
			"status":  "ok",
			"agent":   AgentApp,
			"model":   map[string]interface{}{"kind": "custom", "id": ref.ID, "label": ref.Label},
			"applied": true,
			"scope":   "this session",
		}), nil
	}
	effort := argString(args, "effort", "")
	var numeric interface{}
	if raw, ok := args["value"]; ok && raw != nil {
		switch v := raw.(type) {
		case float64:
			numeric = int64(v)
		case string:
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				if parsed, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
					numeric = parsed
				} else {
					numeric = trimmed
				}
			}
		default:
			numeric = fmt.Sprint(v)
		}
	}
	if label == "" && numeric == nil {
		return "", fmt.Errorf("model is required: pass a label such as \"Gemini 3.8 Flash\" or a numeric value from antigravity_session_model_list")
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"label":  label,
		"value":  numeric,
		"effort": effort,
	})
	var request map[string]interface{}
	_ = json.Unmarshal(payloadBytes, &request)
	labelJSON, _ := json.Marshal(request["label"])
	valueJSON, _ := json.Marshal(request["value"])
	effortJSON, _ := json.Marshal(request["effort"])
	expression := fmt.Sprintf(`(async () => {
		if (!window.__swissSessionModel) return { success: false, reason: 'model_selector_not_found' };
		return await window.__swissSessionModel.apply(%s, %s, %s);
	})()`, string(labelJSON), string(valueJSON), string(effortJSON))

	result, err := s.runSessionModelScript(expression)
	if err != nil {
		return "", err
	}
	if success, ok := result["success"].(bool); !ok || !success {
		reason, _ := result["reason"].(string)
		if reason == "" {
			reason = "model switch failed"
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return string(data), fmt.Errorf("%s", reason)
	}
	if applied, ok := result["applied"].(bool); ok && !applied {
		data, _ := json.MarshalIndent(result, "", "  ")
		return string(data), fmt.Errorf("the model selector did not accept the requested model")
	}
	result["status"] = "ok"
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data), nil
}
