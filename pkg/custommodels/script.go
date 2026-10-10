package custommodels

import (
	"encoding/json"
	"fmt"
)

// GenerateCustomModelsScript produces the JavaScript logic injected into Antigravity
// (Desktop Electron and VS Code extension webviews) to dynamically integrate custom models
// when the Swiss Knife daemon is running and model inference is functional.
func GenerateCustomModelsScript(cfg *Config) string {
	// Custom models must not be baked permanently into the offline Electron renderer.
	// Initial state starts empty; config is loaded dynamically only when the Swiss Knife daemon is running and inference is supported.
	cfgJSON := []byte(`{"version":"1.0.0","inference_supported":true,"models":[],"project_binds":{}}`)
	if cfg != nil {
		if cBytes, err := json.Marshal(cfg); err == nil {
			cfgJSON = cBytes
		}
	}

	return fmt.Sprintf(`
/* === Antigravity Swiss Knife: Custom Model Provider Integration === */
(() => {
  try {
    let customConfig = %s || { version: "1.0.0", inference_supported: true, models: [], project_binds: {} };
    if (!customConfig) customConfig = { version: "1.0.0", inference_supported: true, models: [], project_binds: {} };
    if (!customConfig.models) customConfig.models = [];
    if (!customConfig.project_binds) customConfig.project_binds = {};

    // Dynamic config loader: only queries the local daemon when running.
    // Permanent injection must NOT load or show custom models offline when the app is stopped.
    async function refreshCustomConfig() {
      try {
        if (typeof window !== "undefined" && window.fetch) {
          const res = await fetch("http://127.0.0.1:8765/api/custom_models");
          if (res.ok) {
            const loaded = await res.json();
            if (loaded) {
              customConfig = loaded;
              if (!customConfig.models) customConfig.models = [];
              if (!customConfig.project_binds) customConfig.project_binds = {};
              return;
            }
          }
        }
      } catch (_) {}
      customConfig = { version: "1.0.0", inference_supported: false, models: [], project_binds: {} };
    }

    refreshCustomConfig();
    setInterval(refreshCustomConfig, 5000);

    function getActiveProjectName() {
      // Detect current project name from document title, active sidebar row, or workspace header
      const selectedRow = document.querySelector('[data-testid="conversation-row-sidebar"][data-selected="true"]');
      if (selectedRow) {
        const pName = selectedRow.getAttribute("data-swiss-project");
        if (pName) return pName;
      }
      const projectHeader = document.querySelector('[data-project-card="true"]');
      if (projectHeader) {
        const pName = projectHeader.getAttribute("data-swiss-project") || projectHeader.textContent.trim();
        if (pName) return pName;
      }
      const wsTitle = document.querySelector('[data-testid="workspace-title"], [data-testid="project-name"]');
      if (wsTitle && wsTitle.textContent.trim()) {
        return wsTitle.textContent.trim();
      }
      const title = document.title || "";
      return title.split(" - ")[0].trim() || "Default Project";
    }

    function isCustomModelInferenceAvailable() {
      return Boolean(
        customConfig &&
        customConfig.inference_supported === true &&
        customConfig.models &&
        customConfig.models.some(m => m.enabled)
      );
    }

    // 1. Hook Model Selector Trigger Pill & Dropdown
    function updateModelSelector() {
      const trigger = document.querySelector('[data-testid="model-selector-trigger"]');
      const curProject = getActiveProjectName();

      // Custom model options must only be displayed in the UI if custom model inference is
      // actively supported and functional. If inference is not implemented/working, or if our app
      // is not running, the model selector must remain in its clean native state.
      if (!isCustomModelInferenceAvailable()) {
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
        const thinkingPill = document.querySelector("#swiss-thinking-level-pill");
        if (thinkingPill) thinkingPill.remove();

        const popover = document.querySelector('[role="menu"], [data-radix-popper-content-wrapper], .model-dropdown-menu');
        if (popover) {
          const customGrp = popover.querySelector(".swiss-custom-models-menu-group");
          if (customGrp) customGrp.remove();
          const customHeader = popover.querySelector('[data-testid="custom-models-header"]');
          if (customHeader) customHeader.remove();
          const nativeHeader = popover.querySelector('[data-testid="model-selector-header"]');
          if (nativeHeader && nativeHeader.textContent.trim() === "Native Model") {
            nativeHeader.textContent = "Model";
          }
        }
        return;
      }

      // Check if a custom model is explicitly bound to this project
      const boundModelId = customConfig?.project_binds?.[curProject];
      let activeCustomModel = null;
      if (boundModelId && boundModelId !== "native") {
        activeCustomModel = (customConfig?.models || []).find(m => m.id === boundModelId && m.enabled) || null;
      }

      // Update trigger pill text if custom model is bound
      if (trigger) {
        const label = trigger.querySelector("span") || trigger;
        if (activeCustomModel) {
          if (trigger.getAttribute("data-swiss-custom-bound") !== activeCustomModel.id) {
            trigger.setAttribute("data-swiss-custom-bound", activeCustomModel.id);
            const cleanActiveName = (activeCustomModel.display_name || "")
              .replace(/\s*\((Anthropic|OpenAI)\)/gi, "")
              .replace(/^(OpenAI|Anthropic)\s+/gi, "")
              .trim();
            label.textContent = cleanActiveName + " \u25be";
          }
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

        // Clean up any stale Gemini reasoning selector from DOM if present
        const staleGeminiSelector = document.querySelector("#swiss-gemini-reasoning-selector");
        if (staleGeminiSelector) {
          staleGeminiSelector.remove();
        }

        // Render / update thinking level switcher pill if custom model supports thinking
        let thinkingPill = document.querySelector("#swiss-thinking-level-pill");
        if (activeCustomModel && activeCustomModel.supports_thinking) {
          const curLevel = activeCustomModel.thinking_level || "high";
          const levels = (activeCustomModel.thinking_levels && activeCustomModel.thinking_levels.length > 0)
            ? activeCustomModel.thinking_levels
            : ["off", "low", "medium", "high"];
          const displayLevel = curLevel.charAt(0).toUpperCase() + curLevel.slice(1);

          if (!thinkingPill) {
            thinkingPill = document.createElement("button");
            thinkingPill.id = "swiss-thinking-level-pill";
            thinkingPill.setAttribute("type", "button");
            thinkingPill.style.cssText = "display: inline-flex; align-items: center; gap: 4px; margin-left: 6px; padding: 2px 9px; font-size: 11px; font-weight: 500; border-radius: 9999px; border: 1px solid var(--border, rgba(0,0,0,0.12)); background: var(--secondary, rgba(0,0,0,0.04)); color: var(--foreground, #101010); cursor: pointer; user-select: none; transition: all 0.15s ease; height: 22px; vertical-align: middle;";

            thinkingPill.onclick = (e) => {
              e.stopPropagation();
              e.preventDefault();
              const activeMod = (customConfig?.models || []).find(m => m.id === activeCustomModel.id);
              const cLevel = activeMod?.thinking_level || "high";
              const idx = levels.indexOf(cLevel);
              const nextIdx = (idx + 1) %% levels.length;
              const nextLevel = levels[nextIdx];
              if (activeMod) activeMod.thinking_level = nextLevel;
              activeCustomModel.thinking_level = nextLevel;
              const nextDisplay = nextLevel.charAt(0).toUpperCase() + nextLevel.slice(1);
              thinkingPill.innerHTML = '<span style="opacity: 0.8;">Thinking:</span> <b>' + nextDisplay + '</b> <span style="font-size: 9px; opacity: 0.6;">\\u25be</span>';
              fetch("http://127.0.0.1:8765/api/custom_models/thinking_level", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ model_id: activeCustomModel.id, level: nextLevel })
              }).catch(() => {});
            };

            if (trigger.nextSibling) {
              trigger.parentNode.insertBefore(thinkingPill, trigger.nextSibling);
            } else {
              trigger.parentNode.appendChild(thinkingPill);
            }
          }
          thinkingPill.innerHTML = '<span style="opacity: 0.8;">Thinking:</span> <b>' + displayLevel + '</b> <span style="font-size: 9px; opacity: 0.6;">\\u25be</span>';
          thinkingPill.title = "Click to switch thinking level (" + levels.join(" \\u2192 ") + ")";
        } else if (thinkingPill) {
          thinkingPill.remove();
        }
      }

      // Detect open dropdown menu
      const popover = document.querySelector('[role="menu"], [data-radix-popper-content-wrapper], .model-dropdown-menu');
      if (!popover || popover.__swissCustomGrouped) return;

      const menuItems = Array.from(popover.querySelectorAll('[role="menuitem"], button, .menu-item'));
      if (menuItems.length === 0) return;

      // Check if menu has native model names
      const isModelMenu = menuItems.some(it => {
        const txt = it.textContent || "";
        return txt.includes("Gemini") || txt.includes("Flash") || txt.includes("Claude") || txt.includes("Pro") || txt.includes("GPT-OSS") || txt.includes("Opus");
      });

      if (!isModelMenu) return;

      // Rename factory category header "Model" to "Native Model"
      const nativeHeader = popover.querySelector('[data-testid="model-selector-header"]') ||
        Array.from(popover.querySelectorAll('div')).find(d => {
          const t = (d.textContent || "").trim();
          return d.children.length === 0 && (t === "Model" || t === "Native Models");
        });
      if (nativeHeader && nativeHeader.textContent.trim() !== "Native Model") {
        nativeHeader.textContent = "Native Model";
      }

      // Rename existing custom category header to "Custom Model"
      const existingCustomHeader = popover.querySelector('[data-testid="custom-models-header"]');
      if (existingCustomHeader && existingCustomHeader.textContent.trim() !== "Custom Model") {
        existingCustomHeader.textContent = "Custom Model";
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
        if (low === "openrouter") return "OpenRouter";
        if (low === "groq") return "Groq";
        if (low === "mistral") return "Mistral";
        if (low === "deepseek") return "DeepSeek";
        return pt.charAt(0).toUpperCase() + pt.slice(1);
      }

      // Hook native items to unbind custom model when native model is chosen
      menuItems.forEach(it => {
        if (!it.__swissNativeBound) {
          it.__swissNativeBound = true;
          it.addEventListener("click", async () => {
            if (!customConfig.project_binds) customConfig.project_binds = {};
            customConfig.project_binds[curProject] = "native";
            if (trigger) {
              trigger.removeAttribute("data-swiss-custom-bound");
              trigger.removeAttribute("data-swiss-native-label");
            }
            const tp = document.querySelector("#swiss-thinking-level-pill");
            if (tp) tp.remove();
            try {
              await fetch("http://127.0.0.1:8765/api/custom_models/bind", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ project: curProject, model_id: "native" })
              });
            } catch (_) {}
          });
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
          customContainer.appendChild(separator);

          // Native Section Header matching factory "Model" header
          const customHeader = document.createElement("div");
          customHeader.setAttribute("data-testid", "custom-models-header");
          customHeader.className = "text-xs px-2 pt-1 pb-1 text-muted-foreground font-medium select-none";
          customHeader.textContent = "Custom Model";
          customHeader.style.cssText = "padding: 3px 8px 3px 8px; font-size: 12px; font-weight: 500; color: var(--muted-foreground, #71717a); user-select: none;";
          customContainer.appendChild(customHeader);

          customModels.forEach(m => {
            const isCurrentlyActive = (activeCustomModel && activeCustomModel.id === m.id);
            const itemEl = document.createElement("div");
            itemEl.setAttribute("role", "menuitem");
            itemEl.setAttribute("tabindex", "-1");
            itemEl.setAttribute("data-testid", "model-selector-custom-item");
            itemEl.className = "w-full pr-2 pl-2 text-left text-[13px] cursor-pointer outline-none no-focus-ring transition-colors select-none flex items-center rounded-md py-1 gap-1.5 text-secondary-foreground justify-between";
            itemEl.style.cssText = "padding: 4px 8px; font-size: 13px; font-family: inherit; cursor: pointer; display: flex; align-items: center; justify-content: space-between; border-radius: 6px; height: 26px; box-sizing: border-box; transition: background 0.1s, color 0.1s; color: var(--secondary-foreground, #334155); background: " + (isCurrentlyActive ? "var(--secondary, rgba(0,0,0,0.06))" : "transparent") + ";";

            itemEl.onmouseenter = () => {
              itemEl.style.backgroundColor = "var(--secondary, rgba(0, 0, 0, 0.06))";
              itemEl.style.color = "var(--foreground, #101010)";
            };
            itemEl.onmouseleave = () => {
              itemEl.style.backgroundColor = isCurrentlyActive ? "var(--secondary, rgba(0, 0, 0, 0.06))" : "transparent";
              itemEl.style.color = isCurrentlyActive ? "var(--foreground, #101010)" : "var(--secondary-foreground, #334155)";
            };

            let quotaText = "";
            if (m.quota_type === "cost_based" && m.total_budget > 0) {
              const pct = Math.round((m.prepaid_balance / m.total_budget) * 100);
              quotaText = "$" + m.prepaid_balance.toFixed(2) + " (" + pct + "%%)";
            } else if (m.quota_type === "quota_based" && m.quota_fraction != null) {
              quotaText = Math.round(m.quota_fraction * 100) + "%%";
            }

            const providerLabel = formatProvider(m.provider_type);

            const cleanDisplayName = (m.display_name || "")
              .replace(/\s*\((Anthropic|OpenAI)\)/gi, "")
              .replace(/^(OpenAI|Anthropic)\s+/gi, "")
              .trim();

            itemEl.innerHTML = '<span class="flex items-center gap-1.5 min-w-0 flex-1 truncate" style="display: flex; align-items: center; gap: 6px; min-width: 0; flex: 1; overflow: hidden;">' +
              '<span class="truncate text-xs text-left" style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; font-weight: 400; color: inherit;">' + cleanDisplayName + '</span>' +
              '</span>' +
              '<span style="display: flex; align-items: center; gap: 4px; flex-shrink: 0; padding-left: 8px;">' +
              (quotaText ? '<span style="font-size: 11px; color: var(--muted-foreground, #64748b); opacity: 0.75; font-weight: 400; white-space: nowrap;">' + quotaText + '</span>' : '') +
              (isCurrentlyActive ? '<span style="font-size: 12px; font-weight: 600; color: var(--foreground, #101010); margin-left: 2px;">\u2713</span>' : '') +
              '</span>';

            itemEl.onclick = async () => {
              if (!customConfig.project_binds) customConfig.project_binds = {};
              customConfig.project_binds[curProject] = m.id;

              if (trigger) {
                const label = trigger.querySelector("span") || trigger;
                if (!trigger.getAttribute("data-swiss-native-label")) {
                  trigger.setAttribute("data-swiss-native-label", label.textContent.trim());
                }
                label.textContent = cleanDisplayName + " \u25be";
                trigger.setAttribute("data-swiss-custom-bound", m.id);
              }
              try {
                await fetch("http://127.0.0.1:8765/api/custom_models/bind", {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ project: curProject, model_id: m.id })
                });
              } catch (_) {}
              popover.remove();
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

          if (m.quota_type === "cost_based") {
            if (m.total_budget > 0) {
              pct = Math.min(100, Math.max(0, Math.round((m.prepaid_balance / m.total_budget) * 100)));
              hasInfo = true;
              subtitle = "$" + m.prepaid_balance.toFixed(2) + " of $" + m.total_budget.toFixed(2) + " prepaid deposit left";
            } else {
              hasInfo = false;
              subtitle = "Cost-based (Untracked balance)";
            }
          } else if (m.quota_type === "quota_based") {
            if (m.quota_fraction != null) {
              pct = Math.min(100, Math.max(0, Math.round(m.quota_fraction * 100)));
              hasInfo = true;
              subtitle = pct + "%% rate limit quota remaining";
            } else {
              hasInfo = false;
              subtitle = "Quota-based (Untracked)";
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
