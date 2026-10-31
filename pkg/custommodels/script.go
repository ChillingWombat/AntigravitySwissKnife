package custommodels

import (
	"encoding/json"
	"fmt"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/rendererbroker"
)

// GenerateCustomModelsScript produces the JavaScript logic injected into Antigravity
// (Desktop Electron and VS Code extension webviews) to dynamically integrate custom models
// when the Swiss Knife daemon is running and model inference is functional.
func GenerateCustomModelsScript(cfg *Config) string {
	// Custom models must not be baked permanently into the offline Electron renderer.
	// Initial state starts empty; config is loaded dynamically only when the Swiss Knife daemon is running and inference is supported.
	cfgJSON := []byte(`{"version":"1.0.0","inference_supported":true,"gemini_quota_exhausted":false,"integration":{"mode":"profile_isolation","apps":{"antigravity":{"source":"native","subagents":"app"},"vscode":{"source":"native","subagents":"app"}}},"models":[]}`)
	if cfg != nil {
		cfgCopy := *cfg
		cfgCopy.InferenceSupported = true
		if cfgCopy.Integration.Mode == "" {
			cfgCopy.Integration.Mode = IntegrationProfileIsolation
		}
		if cfgCopy.Integration.Apps == nil {
			cfgCopy.Integration.Apps = map[string]AppProfile{
				"antigravity": {Source: ModelSourceNative, Subagents: SubagentsApp},
				"vscode":      {Source: ModelSourceNative, Subagents: SubagentsApp},
			}
		}
		if cBytes, err := json.Marshal(&cfgCopy); err == nil {
			cfgJSON = cBytes
		}
	}

	return rendererbroker.FetchScript + fmt.Sprintf(`
/* === Antigravity Swiss Knife: Custom Model Provider Integration === */
(() => {
  try {
    let customConfig = %s;
    if (!customConfig.models) customConfig.models = [];
    if (!customConfig.integration) {
      customConfig.integration = {
        mode: "profile_isolation",
        apps: {
          antigravity: { source: "native", subagents: "app" },
          vscode: { source: "native", subagents: "app" }
        }
      };
    }

    function getEffectiveCatalogMode() {
      const mode = (customConfig && customConfig.integration && customConfig.integration.mode) || "profile_isolation";
      if (mode === "api_proxy") {
        return "hybrid";
      }
      let profile = "";
      if (typeof window !== "undefined" && window.__SWISS_WINDOW_PROFILE__) {
        profile = window.__SWISS_WINDOW_PROFILE__;
      } else if (
        typeof process !== "undefined" &&
        ((process.env && (process.env.AGY_LLM_GATEWAY_URL || (process.env.HOME && process.env.HOME.includes("antigravity-swiss-custom")))) ||
         (Array.isArray(process.argv) && process.argv.some(a => typeof a === "string" && a.includes("antigravity-swiss-custom"))))
      ) {
        profile = "custom";
      } else {
        profile = (customConfig && customConfig.integration && customConfig.integration.apps && customConfig.integration.apps.antigravity && customConfig.integration.apps.antigravity.source) || "native";
      }

      if (profile === "custom") {
        return "custom_only";
      }
      return "native_only";
    }

    // Auth headers provider matching daemon session authentication
    function getSwissAuthHeaders(extra = {}) {
      const headers = Object.assign({ "Content-Type": "application/json" }, extra);
      return headers;
    }

    // Dynamic config loader: only queries the local daemon when running.
    // Permanent injection must NOT load or show custom models offline when the app is stopped.
    async function refreshCustomConfig() {
      try {
        if (typeof window !== "undefined" && window.fetch) {
          const res = await fetch("http://127.0.0.1:8765/api/custom_models", {
            headers: getSwissAuthHeaders(),
            signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(2000) : undefined
          });
          if (res.ok) {
            const loaded = await res.json();
            if (loaded) {
              customConfig = loaded;
              if (!customConfig.models) customConfig.models = [];
              if (!customConfig.integration) {
                customConfig.integration = {
                  mode: "profile_isolation",
                  apps: {
                    antigravity: { source: "native", subagents: "app" },
                    vscode: { source: "native", subagents: "app" }
                  }
                };
              }
              return;
            }
          }
        }
      } catch (_) {}
      const savedIntegration = customConfig && customConfig.integration;
      customConfig = {
        version: "1.0.0",
        inference_supported: false,
        models: [],
        integration: savedIntegration || {
          mode: "profile_isolation",
          apps: {
            antigravity: { source: "native", subagents: "app" },
            vscode: { source: "native", subagents: "app" }
          }
        }
      };
    }

    refreshCustomConfig();
    setInterval(refreshCustomConfig, 5000);

    function isCustomModelInferenceAvailable() {
      return Boolean(
        customConfig &&
        customConfig.inference_supported === true &&
        customConfig.models &&
        customConfig.models.some(m => m.enabled)
      );
    }

    // Factory Antigravity 2.0 SVG icons (CheckIcon and ChevronRightIcon from workbench main.js)
    const FACTORY_CHECK_SVG = '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 -960 960 960" fill="currentColor" class="shrink-0" data-testid="menu-item-check" style="flex-shrink: 0;"><path d="M382-339.38L727.54-684.92q8.92-8.92 20.88-9.12t21.27,9.12T779-663.54t-9.31,21.38l-362.38,363Q396.46-268.31 382-268.31t-25.31-10.85l-167-167q-8.92-8.92-8.81-21.19t9.42-21.58t21.38-9.31t21.38,9.31L382-339.38Z"/></svg>';
    const FACTORY_CHEVRON_RIGHT_SVG = '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 -960 960 960" fill="currentColor" class="shrink-0" style="flex-shrink: 0;"><path d="M517.85-480l-184-184L376-706.15L602.15-480L376-253.85L333.85-296l184-184Z"/></svg>';

    const EFFORT_LABELS = {
      low: "Low",
      medium: "Medium",
      high: "High",
      xhigh: "Extra",
      max: "Max"
    };

    function formatEffortLabel(lvl) {
      if (!lvl) return "";
      const key = String(lvl).trim().toLowerCase();
      if (EFFORT_LABELS[key]) return EFFORT_LABELS[key];
      return key.charAt(0).toUpperCase() + key.slice(1);
    }

    function cleanModelDisplayName(raw) {
      return (raw || "")
        .replace(/\s*\((Anthropic|OpenAI|GCP Vertex|GCP|Google Cloud)\)/gi, "")
        .replace(/^(OpenAI|Anthropic|Google Cloud|GCP)\s+/gi, "")
        .trim();
    }

    function getModelThinkingLevels(m) {
      if (!m || m.supports_thinking !== true) return [];
      const supported = ["low", "medium", "high"];
      const hasRawLevels = Array.isArray(m.thinking_levels) && m.thinking_levels.length > 0;
      if (!hasRawLevels) return supported.slice();
      const levels = m.thinking_levels
        .map(l => String(l).trim().toLowerCase())
        .filter(l => supported.includes(l));
      return Array.from(new Set(levels));
    }

    function getActiveThinkingLevel(m, levels) {
      const lvls = levels || getModelThinkingLevels(m);
      if (!lvls || lvls.length === 0) return "";
      const cur = (m && m.thinking_level ? String(m.thinking_level) : "").trim().toLowerCase();
      if (cur && cur !== "off" && lvls.includes(cur)) return cur;
      return lvls.includes("high") ? "high" : lvls[lvls.length - 1];
    }

    function renderTriggerModelLabel(trigger, modelObj) {
      if (!trigger || !modelObj) return;
      const label = trigger.querySelector("span") || trigger;
      const cleanName = cleanModelDisplayName(modelObj.display_name || modelObj.name);
      const levels = getModelThinkingLevels(modelObj);
      const activeLvl = getActiveThinkingLevel(modelObj, levels);
      const effortText = activeLvl ? formatEffortLabel(activeLvl) : "";
      const sig = modelObj.id + ":" + activeLvl;
      const accessibleLabel = effortText ? cleanName + " (" + effortText + ")" : cleanName;
      if (trigger.getAttribute("aria-label") !== accessibleLabel) {
        trigger.setAttribute("aria-label", accessibleLabel);
      }
      if (trigger.getAttribute("data-swiss-custom-bound") !== sig || !(label.textContent || "").includes(cleanName)) {
        trigger.setAttribute("data-swiss-custom-bound", sig);
        label.textContent = cleanName;
        if (effortText) {
          const effortSpan = document.createElement("span");
          effortSpan.className = "opacity-70";
          effortSpan.textContent = " " + effortText;
          label.appendChild(effortSpan);
        }
      }
    }

    let submenuCloseTimer = null;

    function clearSubmenuCloseTimer() {
      if (submenuCloseTimer) {
        clearTimeout(submenuCloseTimer);
        submenuCloseTimer = null;
      }
    }

    function removeCustomThinkingSubmenu() {
      clearSubmenuCloseTimer();
      const existing = document.getElementById("swiss-custom-thinking-submenu");
      if (existing) existing.remove();
      document.querySelectorAll('[data-testid="model-selector-custom-item"]').forEach(el => {
        if (el.__swissSyncHoverState) el.__swissSyncHoverState(false);
      });
    }

    function clearSessionCustomModel() {
      if (typeof window !== "undefined") {
        window.__swiss_session_custom_model_id = null;
        window.__swiss_native_model_selected = true;
      }
      customConfig.active_model_id = "";
      const trigger = document.querySelector('[data-testid="model-selector-trigger"]');
      if (trigger) {
        const savedNative = trigger.getAttribute("data-swiss-native-label");
        trigger.removeAttribute("data-swiss-custom-bound");
        trigger.removeAttribute("data-swiss-native-label");
        const label = trigger.querySelector("span") || trigger;
        if (savedNative && label) {
          label.textContent = savedNative;
        }
        try {
          const k = Object.keys(trigger).find(key => key.startsWith("__reactFiber"));
          if (k && trigger[k]) {
            const ch = trigger[k].memoizedProps?.children;
            const collect = (node) => {
              if (!node) return "";
              if (typeof node === "string") return node;
              if (typeof node === "number") return String(node);
              if (Array.isArray(node)) return node.map(collect).join("");
              if (node.props?.children) return collect(node.props.children);
              return "";
            };
            const nativeText = collect(ch?.[0]);
            if (nativeText && label) label.textContent = nativeText;
          }
        } catch (_) {}
      }
      removeCustomThinkingSubmenu();
      fetch("http://127.0.0.1:8765/api/custom_models/active", {
        method: "POST",
        headers: getSwissAuthHeaders(),
        body: JSON.stringify({ model_id: "" })
      }).catch(() => {});
    }

    if (!window.__swissNativeModelCaptureBound) {
      window.__swissNativeModelCaptureBound = true;
      document.addEventListener("click", (event) => {
        const target = event.target;
        if (!(target instanceof Element)) return;
        if (target.closest(".swiss-custom-models-menu-group")) return;
        if (target.closest("#swiss-custom-thinking-submenu")) return;
        if (target.closest('[data-testid="model-selector-custom-item"]')) return;
        if (target.closest('[data-testid="model-selector-trigger"]')) return;
        const popover = getModelSelectorPopover();
        const nativeOption = target.closest(
          '[data-testid="model-selector-effort-option"], ' +
          '[data-testid="model-selector-context-window-option"], ' +
          '[data-testid="model-selector-item"], ' +
          '[data-testid="model-selector-effort-group"]'
        );
        const panelItem = target.closest('[data-testid="model-selector-panel"] [role="menuitem"]:not([data-testid="model-selector-custom-item"])');
        const popoverItem = (popover && popover.contains(target)) ? target.closest('[role="menuitem"]:not([data-testid="model-selector-custom-item"]), [role="menuitemradio"]') : null;
        const clickedItem = nativeOption || panelItem || popoverItem;
        if (!clickedItem || (clickedItem.textContent || "").trim() === "View Usage") return;
        clearSessionCustomModel();
      }, true);
    }

    // The host menus (Projects panel display options, conversation menus, ...) are
    // also [role="menu"], so the custom-model group may only be attached to the
    // model selector's own popover. Anchor on the model-selector testids; never
    // fall back to "the first open menu" on the page.
    function getModelSelectorPopover() {
      const panel = document.querySelector('[data-testid="model-selector-panel"]');
      const anchor = panel || document.querySelector(
        '[data-testid="model-selector-item"], ' +
        '[data-testid="model-selector-header"], ' +
        '[data-testid="model-selector-effort-group"], ' +
        '[data-testid="model-selector-context-window-option"]'
      );
      if (!anchor) return null;
      return anchor.closest('[role="menu"]:not(#swiss-custom-thinking-submenu), [data-radix-popper-content-wrapper], .model-dropdown-menu, .bg-card') || panel || null;
    }

    // 1. Hook Model Selector Trigger Pill & Dropdown
    // Bridge hook: the Gemini Bridge daemon selects a custom model without
    // clicking the dropdown. Same effect as a user pick: session id, app-wide
    // active model, and the model pill label.
    if (typeof window !== "undefined") {
      window.__swissSetCustomModel = function (modelID) {
        const wanted = String(modelID || "").trim();
        if (!wanted) return { success: false, reason: "model_id_required" };
        const models = customConfig.models || [];
        const model = models.find(m => m && m.enabled && (m.id === wanted || m.name === wanted));
        if (!model) return { success: false, reason: "unknown_or_disabled_model" };
        window.__swiss_session_custom_model_id = model.id;
        window.__swiss_native_model_selected = false;
        customConfig.active_model_id = model.id;
        const trigger = document.querySelector('[data-testid="model-selector-trigger"]');
        if (trigger) {
          const label = trigger.querySelector("span") || trigger;
          if (!trigger.getAttribute("data-swiss-native-label") && label) {
            trigger.setAttribute("data-swiss-native-label", (label.textContent || "").trim());
          }
          renderTriggerModelLabel(trigger, model);
        }
        removeCustomThinkingSubmenu();
        fetch("http://127.0.0.1:8765/api/custom_models/active", {
          method: "POST",
          headers: getSwissAuthHeaders(),
          body: JSON.stringify({ model_id: model.id })
        }).catch(() => {});
        return { success: true, model_id: model.id, label: model.display_name || model.name || model.id };
      };
    }

    function updateModelSelectorTrigger(trigger, activeCustomModel) {
      if (!trigger) return;
      const catalogMode = getEffectiveCatalogMode();
      const label = trigger.querySelector("span") || trigger;

      if (catalogMode === "native_only") {
        if (trigger.hasAttribute("data-swiss-custom-bound")) {
          trigger.removeAttribute("data-swiss-custom-bound");
          trigger.removeAttribute("data-swiss-native-label");
          try {
            const k = Object.keys(trigger).find(key => key.startsWith("__reactFiber"));
            if (k && trigger[k]) {
              const ch = trigger[k].memoizedProps?.children;
              const collect = (node) => {
                if (!node) return "";
                if (typeof node === "string") return node;
                if (typeof node === "number") return String(node);
                if (Array.isArray(node)) return node.map(collect).join("");
                if (node.props?.children) return collect(node.props.children);
                return "";
              };
              const nativeText = collect(ch?.[0]);
              if (nativeText) label.textContent = nativeText;
            }
          } catch (_) {}
        }
        return;
      }

      if (catalogMode === "custom_only") {
        let targetModel = activeCustomModel;
        if (!targetModel) {
          const selectable = customConfig.models || [];
          const defId = customConfig.default_custom_model;
          if (defId) {
            targetModel = selectable.find(m => (m.id === defId || m.name === defId) && m.enabled);
          }
          if (!targetModel) {
            targetModel = selectable.find(m => m.is_default && m.enabled) || selectable.find(m => m.enabled);
          }
        }
        if (targetModel) {
          renderTriggerModelLabel(trigger, targetModel);
        }
        return;
      }

      // Hybrid mode
      if (activeCustomModel) {
        renderTriggerModelLabel(trigger, activeCustomModel);
      } else if (trigger.hasAttribute("data-swiss-custom-bound")) {
        trigger.removeAttribute("data-swiss-custom-bound");
        trigger.removeAttribute("data-swiss-native-label");
        try {
          const k = Object.keys(trigger).find(key => key.startsWith("__reactFiber"));
          if (k && trigger[k]) {
            const ch = trigger[k].memoizedProps?.children;
            const collect = (node) => {
              if (!node) return "";
              if (typeof node === "string") return node;
              if (typeof node === "number") return String(node);
              if (Array.isArray(node)) return node.map(collect).join("");
              if (node.props?.children) return collect(node.props.children);
              return "";
            };
            const nativeText = collect(ch?.[0]);
            if (nativeText) label.textContent = nativeText;
          }
        } catch (_) {}
      }
    }

    function updateModelSelector() {
      const trigger = document.querySelector('[data-testid="model-selector-trigger"]');

      // Always remove any legacy external thinking pill in the composer bar
      const legacyThinkingPill = document.querySelector("#swiss-thinking-level-pill");
      if (legacyThinkingPill) legacyThinkingPill.remove();

      // Custom model options must only be displayed in the UI if custom model inference is
      // actively supported and functional. If inference is not implemented/working, or if our app
      // is not running, the model selector must remain in its clean native state.
      if (!isCustomModelInferenceAvailable()) {
        removeCustomThinkingSubmenu();
        if (trigger && trigger.hasAttribute("data-swiss-custom-bound")) {
          trigger.removeAttribute("data-swiss-custom-bound");
          trigger.removeAttribute("data-swiss-native-label");
          try {
            const k = Object.keys(trigger).find(key => key.startsWith("__reactFiber"));
            if (k && trigger[k]) {
              const ch = trigger[k].memoizedProps?.children;
              const collect = (node) => {
                if (!node) return "";
                if (typeof node === "string") return node;
                if (typeof node === "number") return String(node);
                if (Array.isArray(node)) return node.map(collect).join("");
                if (node.props?.children) return collect(node.props.children);
                return "";
              };
              const nativeText = collect(ch?.[0]);
              const label = trigger.querySelector("span") || trigger;
              if (nativeText && label) label.textContent = nativeText;
            }
          } catch (_) {}
        }

        const popover = getModelSelectorPopover();
        if (popover) {
          const customGrp = popover.querySelector(".swiss-custom-models-menu-group");
          if (customGrp) customGrp.remove();
          const customHeader = popover.querySelector('[data-testid="custom-models-header"]');
          if (customHeader) customHeader.remove();
          const nativeHeader = popover.querySelector('[data-testid="model-selector-header"]');
          if (nativeHeader && nativeHeader.textContent.trim() === "Native Model") {
            nativeHeader.textContent = "Model";
          }
          popover.querySelectorAll('[data-testid="model-selector-item"], [data-testid="model-selector-effort-group"], [data-testid="model-selector-context-window-option"]').forEach(el => {
            el.style.display = "";
          });
        }
        return;
      }

      const catalogMode = getEffectiveCatalogMode();
      if (catalogMode === "native_only") {
        removeCustomThinkingSubmenu();
        const popover = getModelSelectorPopover();
        if (popover) {
          const existingCustomGroup = popover.querySelector(".swiss-custom-models-menu-group");
          if (existingCustomGroup) existingCustomGroup.remove();
          const customHeader = popover.querySelector('[data-testid="custom-models-header"]');
          if (customHeader) customHeader.remove();

          const nativeHeader = popover.querySelector('[data-testid="model-selector-header"]') ||
            Array.from(popover.querySelectorAll('div')).find(d => {
              const t = (d.textContent || "").trim();
              return d.children.length === 0 && (t === "Model" || t === "Native Model" || t === "Native Models");
            });
          if (nativeHeader && nativeHeader.textContent.trim() === "Native Model") {
            nativeHeader.textContent = "Model";
          }
          popover.querySelectorAll('[data-testid="model-selector-item"], [data-testid="model-selector-effort-group"], [data-testid="model-selector-context-window-option"]').forEach(el => {
            el.style.display = "";
          });
        }
        if (trigger && (trigger.hasAttribute("data-swiss-custom-bound") || (typeof window !== "undefined" && window.__swiss_session_custom_model_id))) {
          clearSessionCustomModel();
        }
        return;
      }

      // Determine activeCustomModel. A model is never bound to a project: the pick
      // is per session, falling back to the app-wide active model, and only then to
      // the quota-exhaustion backup. If the user explicitly clicked a native model,
      // respect that selection and do not auto-override with custom model fallback.
      let activeCustomModel = null;
      const selectable = customConfig.models || [];
      const isNativeSelected = (typeof window !== "undefined" && window.__swiss_native_model_selected === true);
      if (isNativeSelected) {
        activeCustomModel = null;
      } else {
        if (typeof window !== "undefined" && window.__swiss_session_custom_model_id) {
          activeCustomModel = selectable.find(m => m.id === window.__swiss_session_custom_model_id && m.enabled) || null;
        }
        if (!activeCustomModel && customConfig.active_model_id) {
          activeCustomModel = selectable.find(m => m.id === customConfig.active_model_id && m.enabled) || null;
        }
        if (!activeCustomModel && (customConfig.gemini_quota_exhausted === true || catalogMode === "custom_only")) {
          // Gemini quota is exhausted or in custom_only mode: fall back to the configured backup model.
          const defId = customConfig.default_custom_model;
          if (defId) {
            activeCustomModel = selectable.find(m => (m.id === defId || m.name === defId) && m.enabled) || null;
          }
          if (!activeCustomModel) {
            activeCustomModel = selectable.find(m => m.is_default && m.enabled) || null;
          }
          if (!activeCustomModel) {
            activeCustomModel = selectable.find(m => m.enabled) || null;
          }
        }
      }

      updateModelSelectorTrigger(trigger, activeCustomModel);

      // Clean up any stale Gemini reasoning selector from DOM if present
      const staleGeminiSelector = document.querySelector("#swiss-gemini-reasoning-selector");
      if (staleGeminiSelector) {
        staleGeminiSelector.remove();
      }

      // Detect open model selector dropdown. Other host menus are [role="menu"]
      // too, so the lookup is anchored on the model selector itself.
      const popover = getModelSelectorPopover();
      if (!popover) {
        removeCustomThinkingSubmenu();
        return;
      }

      const menuItems = Array.from(popover.querySelectorAll('[role="menuitem"], button, .menu-item'));
      if (menuItems.length === 0) return;

      // Check if menu has native model names
      const isModelMenu = menuItems.some(it => {
        const txt = it.textContent || "";
        return txt.includes("Gemini") || txt.includes("Flash") || txt.includes("Claude") || txt.includes("Pro") || txt.includes("GPT-OSS") || txt.includes("Opus") || it.matches?.('[data-testid="model-selector-custom-item"]');
      });

      if (!isModelMenu) return;

      if (catalogMode === "custom_only") {
        popover.querySelectorAll('[data-testid="model-selector-item"]').forEach(el => {
          // Do not hide custom items which have data-testid="model-selector-custom-item"
          if (!el.matches('[data-testid="model-selector-custom-item"]')) {
            el.style.display = "none";
          }
        });
        // Also hide native effort groups and context window options
        popover.querySelectorAll('[data-testid="model-selector-effort-group"], [data-testid="model-selector-context-window-option"]').forEach(el => {
          el.style.display = "none";
        });
      } else if (catalogMode === "hybrid") {
        popover.querySelectorAll('[data-testid="model-selector-item"], [data-testid="model-selector-effort-group"], [data-testid="model-selector-context-window-option"]').forEach(el => {
          el.style.display = "";
        });
      }

      // Rename factory category header "Model" to "Native Model"
      const nativeHeader = popover.querySelector('[data-testid="model-selector-header"]') ||
        Array.from(popover.querySelectorAll('div')).find(d => {
          const t = (d.textContent || "").trim();
          return d.children.length === 0 && (t === "Model" || t === "Native Model" || t === "Native Models");
        });
      if (nativeHeader) {
        if (catalogMode === "custom_only") {
          nativeHeader.style.display = "none";
        } else if (catalogMode === "hybrid") {
          nativeHeader.style.display = "";
          if (nativeHeader.textContent.trim() !== "Native Model") {
            nativeHeader.textContent = "Native Model";
          }
        } else {
          nativeHeader.style.display = "";
          if (nativeHeader.textContent.trim() === "Native Model") {
            nativeHeader.textContent = "Model";
          }
        }
      }

      // Rename existing custom category header to "Custom Model"
      const existingCustomHeader = popover.querySelector('[data-testid="custom-models-header"]');
      if (existingCustomHeader) {
        if (catalogMode === "custom_only") {
          if (existingCustomHeader.textContent.trim() !== "Model") {
            existingCustomHeader.textContent = "Model";
          }
        } else if (existingCustomHeader.textContent.trim() !== "Custom Model") {
          existingCustomHeader.textContent = "Custom Model";
        }
      }

      const existingSeparator = popover.querySelector('.swiss-custom-models-menu-group > [role="separator"]');
      if (existingSeparator) {
        existingSeparator.style.display = (catalogMode === "custom_only") ? "none" : "";
      }

      if (popover.__swissCustomGrouped) return;
      popover.__swissCustomGrouped = true;

      function formatProvider(pt) {
        if (!pt) return "Custom";
        const low = pt.toLowerCase();
        if (low === "openai") return "OpenAI";
        if (low === "anthropic") return "Anthropic";
        if (low === "ollama") return "Local";
        if (low === "gemini") return "Gemini";
        if (low === "gcp") return "Google Cloud";
        if (low === "openrouter") return "OpenRouter";
        if (low === "groq") return "Groq";
        if (low === "mistral") return "Mistral";
        if (low === "deepseek") return "DeepSeek";
        return pt.charAt(0).toUpperCase() + pt.slice(1);
      }

      // Hook native items to unbind custom model when native model is chosen
      menuItems.forEach(it => {
        if (it.closest(".swiss-custom-models-menu-group")) return;
        const nativeChecks = new Set();
        const labeledCheck = it.querySelector('[data-testid="menu-item-check"]');
        if (labeledCheck) nativeChecks.add(labeledCheck);
        it.querySelectorAll("svg").forEach(svg => {
          const path = svg.querySelector("path");
          const pathData = path ? path.getAttribute("d") || "" : "";
          if ((svg.innerHTML || "").includes("M382-339.38") || pathData.includes("M382-339.38")) {
            nativeChecks.add(svg);
          }
        });
        nativeChecks.forEach(check => {
          check.style.display = activeCustomModel ? "none" : "";
        });
        if (!it.__swissNativeBound && !(it.textContent || "").trim().includes("View Usage")) {
          it.__swissNativeBound = true;
          it.addEventListener("mouseenter", () => {
            removeCustomThinkingSubmenu();
          });
          it.addEventListener("click", async () => {
            clearSessionCustomModel();
          }, true);
        }
      });

      // Find bottom action if present (e.g. "View Usage" / "Select Model Ctrl+/")
      let bottomAction = null;
      for (let i = menuItems.length - 1; i >= 0; i--) {
        const t = (menuItems[i].textContent || "").trim();
        if (t.includes("View Usage") || t.includes("Select Model") || t.includes("Ctrl+/") || t.includes("Add Model")) {
          bottomAction = menuItems[i];
          break;
        }
      }

      const parentEl = menuItems[0]?.parentElement;
      if (parentEl) {
        const customModels = (customConfig?.models || []).filter(m => m.enabled);
        if (customModels.length > 0) {
          // Create "Custom Model" container
          const customContainer = document.createElement("div");
          customContainer.className = "swiss-custom-models-menu-group";

          // Subtle native divider
          const separator = document.createElement("div");
          separator.setAttribute("role", "separator");
          separator.style.cssText = "height: 1px; background: var(--border, rgba(0, 0, 0, 0.08)); margin: 4px -4px 3px -4px;";
          if (catalogMode === "custom_only") {
            separator.style.display = "none";
          }
          customContainer.appendChild(separator);

          // Native Section Header matching factory "Model" header
          const customHeader = document.createElement("div");
          customHeader.setAttribute("data-testid", "custom-models-header");
          customHeader.className = "text-xs px-2 pt-1 pb-1 text-muted-foreground font-medium select-none";
          if (catalogMode === "custom_only") {
            customHeader.textContent = "Model";
          } else {
            customHeader.textContent = "Custom Model";
          }
          customHeader.style.cssText = "padding: 3px 8px 3px 8px; font-size: 12px; font-weight: 500; color: var(--muted-foreground, #71717a); user-select: none;";
          customHeader.addEventListener("mouseenter", () => {
            removeCustomThinkingSubmenu();
          });
          customContainer.appendChild(customHeader);

          customModels.forEach(m => {
            const isCurrentlyActive = (activeCustomModel && activeCustomModel.id === m.id);
            const levels = getModelThinkingLevels(m);
            const supportsThinking = m.supports_thinking === true && levels.length > 0;
            const canChangeThinking = levels.length > 1;
            const curLevel = getActiveThinkingLevel(m, levels);
            const curLevelLabel = formatEffortLabel(curLevel);

            const itemEl = document.createElement("div");
            itemEl.setAttribute("role", "menuitem");
            itemEl.setAttribute("tabindex", "-1");
            itemEl.setAttribute("data-testid", "model-selector-custom-item");
            itemEl.className = "w-full pr-2 pl-2 text-left text-[13px] cursor-pointer outline-none no-focus-ring transition-colors select-none flex items-center rounded-md py-1 gap-1.5 text-secondary-foreground justify-between";
            itemEl.style.cssText = "padding: 4px 8px; font-size: 13px; font-family: inherit; cursor: pointer; display: flex; align-items: center; justify-content: space-between; border-radius: 6px; height: 26px; box-sizing: border-box; transition: background 0.1s, color 0.1s; color: var(--secondary-foreground, #334155); background: " + (isCurrentlyActive ? "var(--secondary, rgba(0,0,0,0.06))" : "transparent") + ";";

            let quotaText = "";
            const qType = (m.quota_type || "").toLowerCase();
            if ((qType === "balance" || qType === "cost_based") && (m.balance_value || (m.total_budget > 0 && m.prepaid_balance > 0))) {
              if (m.balance_value) {
                quotaText = m.balance_value;
              } else {
                const pct = Math.round((m.prepaid_balance / m.total_budget) * 100);
                quotaText = "$" + m.prepaid_balance.toFixed(2) + " (" + pct + "%%)";
              }
            } else if ((qType === "quota" || qType === "quota_based") && (m.quota_value || m.quota_fraction != null)) {
              if (m.quota_value) {
                quotaText = m.quota_value;
              } else if (m.quota_fraction != null) {
                quotaText = Math.round(m.quota_fraction * 100) + "%%";
              }
            }

            const cleanDisplayName = cleanModelDisplayName(m.display_name || m.name);

            let trailingIconHTML = "";
            if (canChangeThinking) {
              if (isCurrentlyActive) {
                trailingIconHTML =
                  '<span class="swiss-custom-check-wrap" style="display: flex; align-items: center; justify-content: center; width: 14px; height: 14px;">' + FACTORY_CHECK_SVG + '</span>' +
                  '<span class="swiss-custom-chevron-wrap" role="button" aria-label="Change thinking level" title="Change thinking level" style="display: none; align-items: center; justify-content: center; width: 16px; height: 16px; opacity: 0.5; border-radius: 4px; cursor: pointer; flex-shrink: 0; transition: opacity 0.1s;">' + FACTORY_CHEVRON_RIGHT_SVG + '</span>';
              } else {
                trailingIconHTML =
                  '<span class="swiss-custom-chevron-wrap" role="button" aria-label="Change thinking level" title="Change thinking level" style="display: none; align-items: center; justify-content: center; width: 16px; height: 16px; opacity: 0.5; border-radius: 4px; cursor: pointer; flex-shrink: 0; transition: opacity 0.1s;">' + FACTORY_CHEVRON_RIGHT_SVG + '</span>';
              }
            } else if (isCurrentlyActive) {
              trailingIconHTML =
                '<span class="swiss-custom-check-wrap" style="display: flex; align-items: center; justify-content: center; width: 14px; height: 14px;">' + FACTORY_CHECK_SVG + '</span>';
            }

            itemEl.innerHTML = '<span class="flex items-center gap-1.5 min-w-0 flex-1 truncate" style="display: flex; align-items: center; gap: 6px; min-width: 0; flex: 1; overflow: hidden;">' +
              '<span class="truncate text-xs text-left" style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; font-weight: 400; color: inherit;">' + cleanDisplayName + '</span>' +
              (supportsThinking ? '<span class="shrink-0 opacity-70 text-xs" data-testid="custom-model-effort-label" style="flex-shrink: 0; opacity: 0.7; font-size: 12px;">' + curLevelLabel + '</span>' : '') +
              '</span>' +
              '<span style="display: flex; align-items: center; gap: 6px; flex-shrink: 0; padding-left: 8px; margin-left: auto;">' +
              (quotaText ? '<span style="font-size: 11px; color: var(--muted-foreground, #64748b); opacity: 0.75; font-weight: 400; white-space: nowrap;">' + quotaText + '</span>' : '') +
              trailingIconHTML +
              '</span>';

            function syncHoverState(hovered) {
              itemEl.style.backgroundColor = (hovered || isCurrentlyActive) ? "var(--secondary, rgba(0, 0, 0, 0.06))" : "transparent";
              itemEl.style.color = (hovered || isCurrentlyActive) ? "var(--foreground, #101010)" : "var(--secondary-foreground, #334155)";
              if (canChangeThinking) {
                const chk = itemEl.querySelector(".swiss-custom-check-wrap");
                const chv = itemEl.querySelector(".swiss-custom-chevron-wrap");
                if (chk) chk.style.display = hovered ? "none" : (isCurrentlyActive ? "flex" : "none");
                if (chv) chv.style.display = hovered ? "flex" : "none";
              }
            }
            itemEl.__swissSyncHoverState = syncHoverState;

            async function selectCustomModelWithLevel(chosenLevel) {
              if (typeof window !== "undefined") {
                window.__swiss_session_custom_model_id = m.id;
                window.__swiss_native_model_selected = false;
              }
              if (chosenLevel) {
                m.thinking_level = chosenLevel;
              }
              customConfig.active_model_id = m.id;

              if (trigger) {
                const label = trigger.querySelector("span") || trigger;
                if (!trigger.getAttribute("data-swiss-native-label")) {
                  trigger.setAttribute("data-swiss-native-label", label.textContent.trim());
                }
                renderTriggerModelLabel(trigger, m);
              }
              removeCustomThinkingSubmenu();
              document.body.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
              popover.remove();

              try {
                if (chosenLevel) {
                  await fetch("http://127.0.0.1:8765/api/custom_models/thinking_level", {
                    method: "POST",
                    headers: getSwissAuthHeaders(),
                    body: JSON.stringify({ model_id: m.id, level: chosenLevel })
                  });
                }
                await fetch("http://127.0.0.1:8765/api/custom_models/active", {
                  method: "POST",
                  headers: getSwissAuthHeaders(),
                  body: JSON.stringify({ model_id: m.id })
                });
              } catch (_) {}
            }

            function openThinkingSubmenu() {
              clearSubmenuCloseTimer();
              const curSubmenu = document.getElementById("swiss-custom-thinking-submenu");
              if (curSubmenu && curSubmenu.__sourceItem === itemEl) {
                syncHoverState(true);
                return;
              }
              removeCustomThinkingSubmenu();
              syncHoverState(true);
              const submenu = document.createElement("div");
              submenu.id = "swiss-custom-thinking-submenu";
              submenu.__sourceItem = itemEl;
              submenu.setAttribute("role", "menu");
              submenu.className = "bg-card text-foreground rounded-[10px] border border-border shadow-md outline-none no-focus-ring min-w-40 scrollbar-none flex flex-col p-1 gap-px";
              const menuCard = getModelSelectorPopover() || popover;
              const menuRect = menuCard.getBoundingClientRect();
              const itemRect = itemEl.getBoundingClientRect();
              const targetLeft = (menuRect.right + 165 > window.innerWidth) ? Math.max(4, Math.round(menuRect.left - 162)) : Math.round(menuRect.right);
              submenu.style.cssText = "position: fixed; z-index: 6001; min-width: 160px; padding: 4px; gap: 1px; display: flex; flex-direction: column; border-radius: 10px; border: 1px solid var(--border, rgba(0,0,0,0.08)); background: var(--card, #ffffff); color: var(--foreground, #101010); box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1); left: " + targetLeft + "px; top: " + Math.round(itemRect.top - 5) + "px;";

              levels.forEach(lvl => {
                const isSelectedLvl = (getActiveThinkingLevel(m, levels) === lvl);
                const optEl = document.createElement("div");
                optEl.setAttribute("role", "menuitemradio");
                optEl.setAttribute("aria-checked", isSelectedLvl ? "true" : "false");
                optEl.setAttribute("data-testid", "model-selector-effort-option");
                optEl.setAttribute("data-effort", lvl);
                optEl.className = "group w-full pr-2 pl-2 text-left text-[13px] cursor-pointer outline-none no-focus-ring transition-colors select-none flex items-center justify-between rounded-md py-1 gap-1.5";
                optEl.style.cssText = "padding: 4px 8px; font-size: 13px; font-family: inherit; cursor: pointer; display: flex; align-items: center; justify-content: space-between; border-radius: 6px; height: 26px; box-sizing: border-box; transition: background 0.1s, color 0.1s; color: " + (isSelectedLvl ? "var(--foreground, #101010)" : "var(--secondary-foreground, #334155)") + "; background: transparent;";
                optEl.innerHTML =
                  '<span class="min-w-0 truncate">' + formatEffortLabel(lvl) + '</span>' +
                  '<span style="width: 14px; height: 14px; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">' +
                  (isSelectedLvl ? FACTORY_CHECK_SVG : '') +
                  '</span>';
                optEl.onmouseenter = () => {
                  optEl.style.backgroundColor = "var(--secondary, rgba(0, 0, 0, 0.06))";
                  optEl.style.color = "var(--foreground, #101010)";
                };
                optEl.onmouseleave = () => {
                  optEl.style.backgroundColor = "transparent";
                  optEl.style.color = isSelectedLvl ? "var(--foreground, #101010)" : "var(--secondary-foreground, #334155)";
                };
                optEl.onclick = (e) => {
                  e.stopPropagation();
                  selectCustomModelWithLevel(lvl);
                };
                submenu.appendChild(optEl);
              });

              submenu.onmouseenter = () => {
                clearSubmenuCloseTimer();
                syncHoverState(true);
              };
              submenu.onmouseleave = (e) => {
                if (e.relatedTarget && itemEl.contains(e.relatedTarget)) return;
                clearSubmenuCloseTimer();
                submenuCloseTimer = setTimeout(() => {
                  removeCustomThinkingSubmenu();
                }, 150);
              };
              document.body.appendChild(submenu);
            }

            const chevronWrap = itemEl.querySelector(".swiss-custom-chevron-wrap");
            if (chevronWrap) {
              chevronWrap.onmouseenter = (e) => {
                e.stopPropagation();
                clearSubmenuCloseTimer();
                chevronWrap.style.opacity = "1";
                openThinkingSubmenu();
              };
              chevronWrap.onmouseleave = () => {
                chevronWrap.style.opacity = "0.5";
              };
              chevronWrap.onclick = (e) => {
                e.stopPropagation();
                clearSubmenuCloseTimer();
                openThinkingSubmenu();
              };
            }

            itemEl.onmouseenter = () => {
              clearSubmenuCloseTimer();
              if (canChangeThinking) {
                const curSubmenu = document.getElementById("swiss-custom-thinking-submenu");
                if (curSubmenu && curSubmenu.__sourceItem === itemEl) {
                  syncHoverState(true);
                  return;
                }
                openThinkingSubmenu();
              } else {
                removeCustomThinkingSubmenu();
                syncHoverState(true);
              }
            };
            itemEl.onmouseleave = (e) => {
              const curSubmenu = document.getElementById("swiss-custom-thinking-submenu");
              if (curSubmenu && curSubmenu.__sourceItem === itemEl) {
                if (e.relatedTarget && (curSubmenu.contains(e.relatedTarget) || itemEl.contains(e.relatedTarget))) {
                  return;
                }
                clearSubmenuCloseTimer();
                submenuCloseTimer = setTimeout(() => {
                  removeCustomThinkingSubmenu();
                }, 150);
                return;
              }
              syncHoverState(false);
            };

            itemEl.onclick = async (e) => {
              if (e && e.target && e.target.closest && e.target.closest(".swiss-custom-chevron-wrap")) {
                return;
              }
              await selectCustomModelWithLevel(supportsThinking ? curLevel : "");
            };

            customContainer.appendChild(itemEl);
          });

          if (bottomAction && bottomAction.parentElement === parentEl) {
            parentEl.insertBefore(customContainer, bottomAction);
          } else {
            parentEl.appendChild(customContainer);
          }
        }
      }
    }

    // 2. Hook "Models & Usage" Settings Page with Progress Rings matching native Google AI card styling
    function updateModelsAndUsagePage() {
      // Fast-path: Only check if settings modal or tab container is actually present
      const settingsContainer = document.querySelector(".settings-tab-content, [role='dialog'], [data-testid='models-usage-container']");
      if (!settingsContainer) return;

      // Look for Models & Usage view container within settings
      const headings = Array.from(settingsContainer.querySelectorAll("h1, h2, h3, div"));
      const modelsPageHeader = headings.find(h => {
        const t = (h.textContent || "").trim();
        return t === "Models & Usage" || t === "Models" || t.includes("Model Quotas");
      });

      if (!modelsPageHeader) return;

      // Find cards container
      const cardsContainer = settingsContainer.querySelector(".model-quota-cards, .quota-grid, [data-testid='models-usage-container']") ||
        modelsPageHeader.closest(".settings-tab-content") ||
        modelsPageHeader.parentElement;

      if (!cardsContainer) return;

      if (!isCustomModelInferenceAvailable()) {
        const existingSection = cardsContainer.querySelector("#swiss-custom-models-section");
        if (existingSection) existingSection.remove();
        return;
      }

      const models = (customConfig?.models || []).filter(m => m.enabled);
      const configKey = JSON.stringify(models);

      let section = cardsContainer.querySelector("#swiss-custom-models-section");
      if (!section) {
        section = document.createElement("div");
        section.id = "swiss-custom-models-section";
        section.style.cssText = "margin-top: 24px; width: 100%%;";
        cardsContainer.appendChild(section);
      } else if (section.__lastConfigKey === configKey) {
        return;
      }
      section.__lastConfigKey = configKey;

      let html = '<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px;">' +
        '<div style="font-size: 13px; font-weight: 500; color: var(--foreground, #101010);">Custom Model</div>' +
        '<span style="font-size: 11px; color: var(--muted-foreground, #64748b); font-weight: 400; background: var(--secondary, rgba(0,0,0,0.06)); padding: 2px 8px; border-radius: 9999px;">Custom Endpoints</span>' +
        '</div>';

      html += '<div style="background: var(--card, #ffffff); border: 1px solid var(--border, rgba(0,0,0,0.08)); border-radius: 10px; overflow: hidden; font-family: inherit;">';

      if (models.length === 0) {
        html += '<div style="padding: 16px; color: var(--muted-foreground, #64748b); font-size: 12px; font-style: italic;">No custom models configured. Add endpoints in Swiss Knife.</div>';
      } else {
        models.forEach((m, idx) => {
          let pct = 0;
          let hasInfo = false;
          let subtitle = "API quota";

          const qType = (m.quota_type || "").toLowerCase();
          if (qType === "balance" || qType === "cost_based") {
            if (m.quota_fraction != null) {
              pct = Math.min(100, Math.max(0, Math.round(m.quota_fraction * 100)));
              hasInfo = true;
              subtitle = (m.balance_value ? m.balance_value + " • " : "") + pct + "%% balance left";
            } else if (m.total_budget > 0) {
              pct = Math.min(100, Math.max(0, Math.round((m.prepaid_balance / m.total_budget) * 100)));
              hasInfo = true;
              subtitle = "$" + m.prepaid_balance.toFixed(2) + " of $" + m.total_budget.toFixed(2) + " prepaid deposit left";
            } else if (m.prepaid_balance > 0) {
              pct = 100;
              hasInfo = true;
              subtitle = "$" + m.prepaid_balance.toFixed(2) + " prepaid deposit left";
            } else if (m.balance_value) {
              const num = parseFloat(m.balance_value.replace(/[^0-9.]/g, ""));
              if (!isNaN(num) && num > 0) {
                pct = 100;
                hasInfo = true;
                subtitle = "Balance: " + m.balance_value + " \u2022 100%% balance left";
              } else {
                hasInfo = false;
                subtitle = "Balance: " + m.balance_value;
              }
            } else {
              hasInfo = false;
              subtitle = "Balance (Untracked)";
            }
          } else if (qType === "quota" || qType === "quota_based") {
            if (m.quota_fraction != null) {
              pct = Math.min(100, Math.max(0, Math.round(m.quota_fraction * 100)));
              hasInfo = true;
              subtitle = (m.quota_value ? m.quota_value + " • " : "") + pct + "%% rate limit quota remaining";
            } else if (m.quota_value) {
              hasInfo = false;
              subtitle = "Quota: " + m.quota_value;
            } else {
              hasInfo = false;
              subtitle = "Quota (Untracked)";
            }
          } else {
            hasInfo = false;
            subtitle = "Local / Pay-as-you-go API (Untracked)";
          }

          const size = 32;
          const stroke = 3;
          const radius = (size - stroke) / 2;
          const circumference = 2 * Math.PI * radius;
          const offset = hasInfo ? circumference - (pct / 100) * circumference : circumference;

          const ringHTML = '<svg width="' + size + '" height="' + size + '" style="transform: rotate(-90deg); flex-shrink: 0;">' +
            '<circle cx="' + (size/2) + '" cy="' + (size/2) + '" r="' + radius + '" fill="none" stroke="var(--border, rgba(0,0,0,0.08))" stroke-width="' + stroke + '"/>' +
            (hasInfo ? '<circle cx="' + (size/2) + '" cy="' + (size/2) + '" r="' + radius + '" fill="none" stroke="var(--foreground, #101010)" stroke-width="' + stroke + '" stroke-dasharray="' + circumference + '" stroke-dashoffset="' + offset + '" stroke-linecap="round"/>' : '') +
            '</svg>';

          const centerText = hasInfo ? pct + "%%" : "N/A";
          const borderStyle = idx < models.length - 1 ? "border-bottom: 1px solid var(--border, rgba(0,0,0,0.06));" : "";
          const provLabel = formatProvider(m.provider_type);

          html += '<div style="padding: 12px 16px; display: flex; align-items: center; justify-content: space-between; ' + borderStyle + '">' +
            '<div>' +
            '<div style="display: flex; align-items: center; gap: 8px;">' +
            '<span style="font-size: 13px; font-weight: 500; color: var(--foreground, #101010);">' + m.display_name + '</span>' +
            '<span style="font-size: 11px; font-weight: 400; padding: 1px 6px; border-radius: 9999px; background: var(--secondary, rgba(0,0,0,0.06)); color: var(--muted-foreground, #64748b);">' + provLabel + '</span>' +
            '</div>' +
            '<div style="font-size: 12px; color: var(--muted-foreground, #64748b); margin-top: 2px;">' + subtitle + '</div>' +
            '</div>' +
            '<div style="display: flex; align-items: center; gap: 10px;">' +
            '<span style="font-size: 13px; font-weight: 500; color: var(--foreground, #101010);">' + centerText + '</span>' +
            ringHTML +
            '</div>' +
            '</div>';
        });
      }

      html += '</div>';
      section.innerHTML = html;
    }

    let scheduledRaf = null;
    let isUpdatingCustomModels = false;

    function runCustomModelHooks() {
      if (isUpdatingCustomModels) return;
      isUpdatingCustomModels = true;
      try {
        updateModelSelector();
        updateModelsAndUsagePage();
      } finally {
        setTimeout(() => { isUpdatingCustomModels = false; }, 32);
      }
    }

    if (window.__swissCustomModelsObserverInstance) {
      window.__swissCustomModelsObserverInstance.disconnect();
    }
    window.__swissCustomModelsObserverInstance = new MutationObserver(() => {
      if (isUpdatingCustomModels) return;
      if (scheduledRaf) return;
      scheduledRaf = requestAnimationFrame(() => {
        scheduledRaf = null;
        runCustomModelHooks();
      });
    });
    window.__swissCustomModelsObserverInstance.observe(document.body, { childList: true, subtree: true });

    runCustomModelHooks();
  } catch (err) {
    console.warn("[SwissKnife] Custom model hook exception:", err);
  }
})();
`, string(cfgJSON))
}
