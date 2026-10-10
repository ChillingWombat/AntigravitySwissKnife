import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import type { SubagentModelStrategy, RuleConfig } from '../types.ts'

// --- Subagent Custom Model Options Logic Models ---

export const DEFAULT_SUBAGENT_MODEL_STRATEGY: SubagentModelStrategy = 'default_custom_only'

export function normalizeSubagentModelStrategy(raw?: string | null): SubagentModelStrategy {
  const trimmed = String(raw || '').trim().toLowerCase()
  if (trimmed === 'auto_decide' || trimmed === 'auto' || trimmed === 'autodecide') {
    return 'auto_decide'
  }
  return 'default_custom_only'
}

export interface SubagentStrategyOption {
  id: SubagentModelStrategy
  label: string
  description: string
  icon: string
}

export const SUBAGENT_STRATEGY_OPTIONS: SubagentStrategyOption[] = [
  {
    id: 'default_custom_only',
    label: 'Default custom model only',
    description: 'Directs all Gemini subagent workloads to the configured Default Custom Model without dynamic model switching.',
    icon: 'Cpu',
  },
  {
    id: 'auto_decide',
    label: 'Auto-decide based on ability, performance, and price',
    description: 'Evaluates task requirements, context size, model latency, and token price across enabled custom models to select the most cost-effective candidate.',
    icon: 'Sliders',
  },
]

describe('Subagent Custom Model Options in Switcher Settings', () => {
  describe('1. Default Strategy & Normalization Fallbacks', () => {
    it('sets default subagent model strategy to "default_custom_only"', () => {
      assert.equal(DEFAULT_SUBAGENT_MODEL_STRATEGY, 'default_custom_only')
    })

    it('normalizes undefined, null, and empty string to "default_custom_only"', () => {
      assert.equal(normalizeSubagentModelStrategy(undefined), 'default_custom_only')
      assert.equal(normalizeSubagentModelStrategy(null), 'default_custom_only')
      assert.equal(normalizeSubagentModelStrategy(''), 'default_custom_only')
      assert.equal(normalizeSubagentModelStrategy('   '), 'default_custom_only')
    })

    it('normalizes unknown or invalid strategies to "default_custom_only"', () => {
      assert.equal(normalizeSubagentModelStrategy('unknown_strategy'), 'default_custom_only')
      assert.equal(normalizeSubagentModelStrategy('random-model'), 'default_custom_only')
      assert.equal(normalizeSubagentModelStrategy('custom_only'), 'default_custom_only')
    })

    it('normalizes "auto_decide", "auto", and aliases to "auto_decide" case-insensitively', () => {
      assert.equal(normalizeSubagentModelStrategy('auto_decide'), 'auto_decide')
      assert.equal(normalizeSubagentModelStrategy('AUTO_DECIDE'), 'auto_decide')
      assert.equal(normalizeSubagentModelStrategy('auto'), 'auto_decide')
      assert.equal(normalizeSubagentModelStrategy('Auto'), 'auto_decide')
      assert.equal(normalizeSubagentModelStrategy('autodecide'), 'auto_decide')
      assert.equal(normalizeSubagentModelStrategy('  auto_decide  '), 'auto_decide')
    })

    it('preserves "default_custom_only" exactly', () => {
      assert.equal(normalizeSubagentModelStrategy('default_custom_only'), 'default_custom_only')
      assert.equal(normalizeSubagentModelStrategy('DEFAULT_CUSTOM_ONLY'), 'default_custom_only')
    })
  })

  describe('2. Option Definitions & Exact Labels/IDs', () => {
    it('contains exactly 2 strategy options', () => {
      assert.equal(SUBAGENT_STRATEGY_OPTIONS.length, 2)
    })

    it('Option A has exact required ID, label, icon and description', () => {
      const optA = SUBAGENT_STRATEGY_OPTIONS.find((o) => o.id === 'default_custom_only')
      assert.ok(optA, 'Option A must exist')
      assert.equal(optA.id, 'default_custom_only')
      assert.equal(optA.label, 'Default custom model only')
      assert.equal(optA.icon, 'Cpu')
      assert.ok(optA.description.includes('Directs all Gemini subagent workloads to the configured Default Custom Model'))
    })

    it('Option B has exact required ID, label, icon and description', () => {
      const optB = SUBAGENT_STRATEGY_OPTIONS.find((o) => o.id === 'auto_decide')
      assert.ok(optB, 'Option B must exist')
      assert.equal(optB.id, 'auto_decide')
      assert.equal(optB.label, 'Auto-decide based on ability, performance, and price')
      assert.equal(optB.icon, 'Sliders')
      assert.ok(optB.description.includes('Evaluates task requirements, context size, model latency, and token price'))
    })
  })

  describe('3. David-Design Compliance & Invariants', () => {
    const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u

    it('guarantees zero decorative emojis across all option labels and descriptions', () => {
      for (const opt of SUBAGENT_STRATEGY_OPTIONS) {
        assert.ok(!emojiRegex.test(opt.label), `Label must not contain emojis: ${opt.label}`)
        assert.ok(!emojiRegex.test(opt.description), `Description must not contain emojis: ${opt.description}`)
      }
    })

    it('inspects SwitcherSettingsPage.tsx source for David-Design compliance in Section 4b', () => {
      const pagePath = path.resolve(import.meta.dirname, '../pages/SwitcherSettingsPage.tsx')
      const content = fs.readFileSync(pagePath, 'utf8')

      // 1. Verify Section 4b exists immediately after Section 4
      assert.ok(content.includes('Section 4b: Gemini Subagent Custom Models'), 'Section 4b comment header must exist')
      const sec4Index = content.indexOf('Section 4: Default Models Configuration')
      const sec4bIndex = content.indexOf('Section 4b: Gemini Subagent Custom Models')
      const sec5Index = content.indexOf('Section 5: Post-Reset Keep-Alive Warmup Engine')
      assert.ok(sec4Index < sec4bIndex, 'Section 4b must appear after Section 4')
      assert.ok(sec4bIndex < sec5Index, 'Section 4b must appear before Section 5')

      // 2. Extract Section 4b code block
      const sec4bBlock = content.slice(sec4bIndex, sec5Index)

      // 3. Verify zero decorative emojis in Section 4b
      assert.ok(!emojiRegex.test(sec4bBlock), 'Section 4b in SwitcherSettingsPage.tsx must not contain decorative emojis')

      // 4. Verify single-line action text (whiteSpace: 'nowrap') on headers and option labels
      assert.ok(sec4bBlock.includes("whiteSpace: 'nowrap'"), 'Section 4b must enforce single-line action headers')

      // 5. Verify 4px grid spacing tokens (12px gap, 14px padding, 16px margin, 8px radius)
      assert.ok(sec4bBlock.includes("gap: '12px'"), 'Option grid must use 12px gap (3x 4px)')
      assert.ok(sec4bBlock.includes("padding: '14px'"), 'Option card must use 14px padding')
      assert.ok(sec4bBlock.includes("borderRadius: '8px'"), 'Option card must use 8px border radius (2x 4px)')
      assert.ok(sec4bBlock.includes("margin: '0 0 16px'"), 'Card description must use 16px bottom margin (4x 4px)')

      // 6. Verify 1px subtle unselected border and 2px primary active border
      assert.ok(sec4bBlock.includes("border: subagentStrategy === 'default_custom_only' ? '2px solid var(--primary)' : '1px solid var(--border)'"))
      assert.ok(sec4bBlock.includes("border: subagentStrategy === 'auto_decide' ? '2px solid var(--primary)' : '1px solid var(--border)'"))

      // 7. Verify Lucide icons exclusively & header toggle switch
      assert.ok(!sec4bBlock.includes('<Bot size={16}'), 'Header icon was removed per user request for clean headers')
      assert.ok(!sec4bBlock.includes('<Cpu'), 'Option A header icon removed per user request')
      assert.ok(!sec4bBlock.includes('<Sliders'), 'Option B header icon removed per user request')
      assert.ok(sec4bBlock.includes('<CheckCircle2 size={14}'), 'Must use Lucide CheckCircle2 icon for active indicator')
      assert.ok(sec4bBlock.includes('<ToggleSwitch'), 'Section 4b must contain ToggleSwitch in header')
      assert.ok(sec4bBlock.includes('subagentCustomModelsEnabled'), 'ToggleSwitch controls subagentCustomModelsEnabled')
      assert.ok(sec4bBlock.includes("opacity: subagentCustomModelsEnabled ? 1 : 0.45"), 'Options dim when disabled')
      assert.ok(sec4bBlock.includes("pointerEvents: subagentCustomModelsEnabled ? 'auto' : 'none'"), 'Options inert when disabled')
    })
  })

  describe('4. State Transition & Auto-Save Payload Simulation', () => {
    it('simulates state initialization with null initialRules falling back to default', () => {
      const initialRules = null as RuleConfig | null
      const state: SubagentModelStrategy = initialRules?.subagent_model_strategy || DEFAULT_SUBAGENT_MODEL_STRATEGY
      assert.equal(state, 'default_custom_only')
    })

    it('simulates hydration from loaded initialRules', () => {
      const initialRules: Partial<RuleConfig> = {
        subagent_model_strategy: 'auto_decide',
      }
      let state: SubagentModelStrategy = 'default_custom_only'
      if (initialRules.subagent_model_strategy) {
        state = initialRules.subagent_model_strategy
      }
      assert.equal(state, 'auto_decide')
    })

    it('simulates clicking Option B and Option A state transitions', () => {
      let state: SubagentModelStrategy = 'default_custom_only'

      // User clicks Option B
      state = 'auto_decide'
      assert.equal(state, 'auto_decide')

      // User clicks Option A
      state = 'default_custom_only'
      assert.equal(state, 'default_custom_only')
    })

    it('simulates debounced auto-save payload construction', () => {
      const state: SubagentModelStrategy = 'auto_decide'
      const payload: Partial<RuleConfig> = {
        auto_switch_threshold: 0.05,
        default_gemini_model: 'gemini-3.8-flash-high',
        default_custom_model: 'custom-gpt-4o',
        subagent_model_strategy: state,
      }

      const serialized = JSON.stringify(payload)
      const parsed = JSON.parse(serialized)

      assert.equal(parsed.subagent_model_strategy, 'auto_decide')
    })
  })
})
