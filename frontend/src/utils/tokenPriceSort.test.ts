import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import type { ModelPricingRecord } from '../types.ts'
import {
  sortPricingModels,
  getGeminiSelectorRank,
  isModelDefaultNative,
  isModelDefaultCustom,
  normalizeModelIdentifier,
  GEMINI_SELECTOR_ORDER,
} from './tokenPriceSort.ts'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

describe('Token Price Table Sorting & Edit Rate Modal (Milestone 3 / R3)', () => {
  const sampleNativeModels: ModelPricingRecord[] = [
    {
      internal_id: 1,
      canonical_id: 'gemini-3.1-pro',
      model_id: 'gemini-3.1-pro',
      model_name: 'Gemini 3.1 Pro',
      name: 'Gemini 3.1 Pro',
      provider: 'Google',
      classification: 'native',
      input_price_per_m: 1.25,
      cached_input_price_per_m: 0.3125,
      output_price_per_m: 5.0,
      source: 'provider',
      updated_at: '2026-10-10 12:00',
    },
    {
      internal_id: 2,
      canonical_id: 'gemini-3.6-flash',
      model_id: 'gemini-3.6-flash',
      model_name: 'Gemini 3.6 Flash',
      name: 'Gemini 3.6 Flash',
      provider: 'Google',
      classification: 'native',
      input_price_per_m: 0.075,
      cached_input_price_per_m: 0.01875,
      output_price_per_m: 0.30,
      source: 'provider',
      updated_at: '2026-10-10 12:00',
    },
    {
      internal_id: 3,
      canonical_id: 'gemini-3.8-flash',
      model_id: 'gemini-3.8-flash',
      model_name: 'Gemini 3.8 Flash',
      name: 'Gemini 3.8 Flash',
      provider: 'Google',
      classification: 'native',
      input_price_per_m: 0.15,
      cached_input_price_per_m: 0.0375,
      output_price_per_m: 0.60,
      source: 'provider',
      updated_at: '2026-10-10 12:00',
    },
    {
      internal_id: 4,
      canonical_id: 'gemini-3.7-flash',
      model_id: 'gemini-3.7-flash',
      model_name: 'Gemini 3.7 Flash',
      name: 'Gemini 3.7 Flash',
      provider: 'Google',
      classification: 'native',
      input_price_per_m: 0.15,
      cached_input_price_per_m: 0.0375,
      output_price_per_m: 0.60,
      source: 'provider',
      updated_at: '2026-10-10 12:00',
    },
  ]

  const sampleCustomModels: ModelPricingRecord[] = [
    {
      internal_id: 20,
      canonical_id: 'deepseek-v3',
      model_id: 'deepseek-v3',
      custom_model_id: 'custom-deepseek',
      model_name: 'DeepSeek V3',
      name: 'DeepSeek V3',
      provider: 'DeepSeek',
      classification: 'custom',
      input_price_per_m: 0.14,
      cached_input_price_per_m: 0.014,
      output_price_per_m: 0.28,
      source: 'provider',
      updated_at: '2026-10-10 12:00',
    },
    {
      internal_id: 10,
      canonical_id: 'gpt-4o',
      model_id: 'gpt-4o',
      custom_model_id: 'custom-gpt4o',
      model_name: 'GPT-4o',
      name: 'GPT-4o',
      provider: 'OpenAI',
      classification: 'custom',
      input_price_per_m: 2.50,
      cached_input_price_per_m: 1.25,
      output_price_per_m: 10.0,
      source: 'provider',
      updated_at: '2026-10-10 12:00',
    },
    {
      internal_id: 15,
      canonical_id: 'claude-3-7-sonnet',
      model_id: 'claude-3-7-sonnet',
      custom_model_id: 'custom-claude37',
      model_name: 'Claude 3.7 Sonnet',
      name: 'Claude 3.7 Sonnet',
      provider: 'Anthropic',
      classification: 'custom',
      input_price_per_m: 3.0,
      cached_input_price_per_m: 0.30,
      output_price_per_m: 15.0,
      source: 'provider',
      updated_at: '2026-10-10 12:00',
    },
  ]

  describe('1. Model Sorting Order Specification', () => {
    it('orders Gemini Native models first according to Antigravity selector sequence', () => {
      const sorted = sortPricingModels(sampleNativeModels)
      const ids = sorted.map((m) => m.canonical_id)
      assert.deepEqual(ids, [
        'gemini-3.8-flash',
        'gemini-3.7-flash',
        'gemini-3.6-flash',
        'gemini-3.1-pro',
      ])
    })

    it('places all Native models before Custom models', () => {
      const mixed = [...sampleCustomModels, ...sampleNativeModels]
      const sorted = sortPricingModels(mixed)

      // First 4 must be native
      for (let i = 0; i < 4; i++) {
        assert.equal(sorted[i].classification, 'native', `Item ${i} should be native`)
      }
      // Last 3 must be custom
      for (let i = 4; i < sorted.length; i++) {
        assert.equal(sorted[i].classification, 'custom', `Item ${i} should be custom`)
      }
    })

    it('orders Custom models by ascending internal_id', () => {
      const sorted = sortPricingModels(sampleCustomModels)
      const internalIds = sorted.map((m) => m.internal_id)
      assert.deepEqual(internalIds, [10, 15, 20])
    })

    it('correctly sorts outdated native models within the custom category by internal_id', () => {
      const outdated: ModelPricingRecord = {
        internal_id: 12,
        canonical_id: 'gemini-1.5-flash',
        model_id: 'gemini-1.5-flash',
        model_name: 'Gemini 1.5 Flash',
        name: 'Gemini 1.5 Flash',
        provider: 'Google',
        classification: 'custom',
        is_outdated_native: true,
        input_price_per_m: 0.075,
        cached_input_price_per_m: 0.01875,
        output_price_per_m: 0.30,
        source: 'manual',
        updated_at: '2026-10-10 12:00',
      }
      const sorted = sortPricingModels([...sampleCustomModels, outdated])
      const internalIds = sorted.map((m) => m.internal_id)
      assert.deepEqual(internalIds, [10, 12, 15, 20])
    })
  })

  describe('2. Chosen Default Model Pinning', () => {
    it('pins the chosen default Gemini model to the top of the Native section', () => {
      // Choose gemini-3.7-flash as default
      const sorted = sortPricingModels(sampleNativeModels, {
        defaultGeminiModel: 'gemini-3.7-flash',
      })
      assert.equal(sorted[0].canonical_id, 'gemini-3.7-flash')
      // Remaining native models follow selector order
      assert.equal(sorted[1].canonical_id, 'gemini-3.8-flash')
      assert.equal(sorted[2].canonical_id, 'gemini-3.6-flash')
      assert.equal(sorted[3].canonical_id, 'gemini-3.1-pro')
    })

    it('pins chosen default Gemini model when specified with thinking level suffix', () => {
      const sorted = sortPricingModels(sampleNativeModels, {
        defaultGeminiModel: 'gemini-3.7-flash-high',
      })
      assert.equal(sorted[0].canonical_id, 'gemini-3.7-flash')
    })

    it('pins native model with is_default property to the top', () => {
      const modelsWithDefault = sampleNativeModels.map((m) =>
        m.canonical_id === 'gemini-3.6-flash' ? { ...m, is_default: true } : m
      )
      const sorted = sortPricingModels(modelsWithDefault)
      assert.equal(sorted[0].canonical_id, 'gemini-3.6-flash')
    })

    it('pins the chosen default Custom model to the top of the Custom section', () => {
      // Choose deepseek-v3 (internal_id 20) as default
      const mixed = [...sampleCustomModels, ...sampleNativeModels]
      const sorted = sortPricingModels(mixed, {
        defaultCustomModel: 'custom-deepseek',
      })

      // Native section remains on top
      assert.equal(sorted[0].classification, 'native')
      assert.equal(sorted[3].classification, 'native')

      // First custom item (index 4) should be DeepSeek V3 despite having highest internal_id (20)
      assert.equal(sorted[4].classification, 'custom')
      assert.equal(sorted[4].canonical_id, 'deepseek-v3')

      // Subsequent custom models are ordered by ascending internal_id
      assert.equal(sorted[5].canonical_id, 'gpt-4o') // internal_id 10
      assert.equal(sorted[6].canonical_id, 'claude-3-7-sonnet') // internal_id 15
    })

    it('pins custom model with is_default property to the top of the Custom section', () => {
      const customWithDefault = sampleCustomModels.map((m) =>
        m.canonical_id === 'claude-3-7-sonnet' ? { ...m, is_default: true } : m
      )
      const sorted = sortPricingModels(customWithDefault)
      assert.equal(sorted[0].canonical_id, 'claude-3-7-sonnet')
      assert.equal(sorted[1].canonical_id, 'gpt-4o')
      assert.equal(sorted[2].canonical_id, 'deepseek-v3')
    })

    it('pins both default Native and default Custom models simultaneously in their respective sections', () => {
      const mixed = [...sampleCustomModels, ...sampleNativeModels]
      const sorted = sortPricingModels(mixed, {
        defaultGeminiModel: 'gemini-3.1-pro',
        defaultCustomModel: 'custom-deepseek',
      })

      // Native pinned: gemini-3.1-pro is at index 0
      assert.equal(sorted[0].canonical_id, 'gemini-3.1-pro')
      assert.equal(sorted[0].classification, 'native')

      // Custom pinned: deepseek-v3 is at index 4
      assert.equal(sorted[4].canonical_id, 'deepseek-v3')
      assert.equal(sorted[4].classification, 'custom')
    })
  })

  describe('3. Normalization & Helper Invariants', () => {
    it('verifies GEMINI_SELECTOR_ORDER defines models in Antigravity selector sequence', () => {
      assert.deepEqual(GEMINI_SELECTOR_ORDER, [
        'gemini-3.8-flash',
        'gemini-3.7-flash',
        'gemini-3.6-flash',
        'gemini-3.1-pro',
      ])
    })

    it('verifies getGeminiSelectorRank returns correct 0-based rank for selector models', () => {
      assert.equal(getGeminiSelectorRank(sampleNativeModels[2]), 0) // gemini-3.8-flash
      assert.equal(getGeminiSelectorRank(sampleNativeModels[3]), 1) // gemini-3.7-flash
      assert.equal(getGeminiSelectorRank(sampleNativeModels[1]), 2) // gemini-3.6-flash
      assert.equal(getGeminiSelectorRank(sampleNativeModels[0]), 3) // gemini-3.1-pro

      const nonGeminiNative: ModelPricingRecord = {
        ...sampleNativeModels[0],
        canonical_id: 'claude-sonnet-4-6',
        model_name: 'Claude Sonnet 4.6',
        provider: 'Anthropic',
      }
      assert.equal(getGeminiSelectorRank(nonGeminiNative), 200)
    })

    it('verifies isModelDefaultNative detects default native model accurately', () => {
      assert.equal(isModelDefaultNative(sampleNativeModels[2], 'gemini-3.8-flash-high'), true)
      assert.equal(isModelDefaultNative(sampleNativeModels[3], 'gemini-3.8-flash-high'), false)
      assert.equal(isModelDefaultNative(sampleNativeModels[3], 'gemini-3.7-flash'), true)
      assert.equal(isModelDefaultNative(sampleCustomModels[0], 'gemini-3.8-flash'), false)
    })

    it('verifies isModelDefaultCustom detects default custom model accurately', () => {
      assert.equal(isModelDefaultCustom(sampleCustomModels[0], 'custom-deepseek'), true)
      assert.equal(isModelDefaultCustom(sampleCustomModels[0], 'deepseek-v3'), true)
      assert.equal(isModelDefaultCustom(sampleCustomModels[1], 'custom-deepseek'), false)
      assert.equal(isModelDefaultCustom(sampleNativeModels[0], 'custom-deepseek'), false)
    })

    it('normalizes model identifiers with reasoning levels and whitespace', () => {
      assert.equal(normalizeModelIdentifier('gemini-3.8-flash-high'), 'gemini-3.8-flash')
      assert.equal(normalizeModelIdentifier('gemini-3.7-flash-medium'), 'gemini-3.7-flash')
      assert.equal(normalizeModelIdentifier('gemini-3.6-flash-low'), 'gemini-3.6-flash')
      assert.equal(normalizeModelIdentifier('gemini-pro-agent'), 'gemini-3.1-pro')
      assert.equal(normalizeModelIdentifier('Gemini 3.8 Flash'), 'gemini-3.8-flash')
    })

    it('handles empty, null, or undefined model lists immutably', () => {
      const empty = sortPricingModels([])
      assert.deepEqual(empty, [])

      const original = [...sampleNativeModels]
      const sorted = sortPricingModels(original)
      assert.notEqual(sorted, original) // new reference
      assert.equal(original[0].canonical_id, 'gemini-3.1-pro') // original unchanged
    })
  })

  describe('4. TokenMonitorPage Source Code & UI Structural Verification', () => {
    const pagePath = path.resolve(__dirname, '../pages/TokenMonitorPage.tsx')
    const source = fs.readFileSync(pagePath, 'utf8')

    it('verifies Metric Card displays "Generation Speed" and "76.2 TPS"', () => {
      assert.match(source, /Generation Speed/, 'Card header must be Generation Speed')
      assert.match(source, /76\.2\s*(?:<span[^>]*>)?TPS/, 'Card must display 76.2 TPS')
      assert.doesNotMatch(source, /Average Generation Speed/, 'Old header Average Generation Speed must be removed')
    })

    it('verifies subscript subtitles are removed from all 4 metric cards', () => {
      assert.doesNotMatch(source, /Prompt caching saved/, 'Card 1 subtitle must be removed')
      assert.doesNotMatch(source, /prompt tokens cached/, 'Card 2 subtitle must be removed')
      assert.doesNotMatch(source, /Measured across.*active model/, 'Card 3 subtitle must be removed')
      assert.doesNotMatch(source, /Across \{projectBreakdowns\.length\} active project/, 'Card 4 subtitle must be removed')
    })

    it('verifies table row Action column only contains "Edit Rate" button (Delete button removed)', () => {
      // In the table row mapping section, find Action column td
      const actionTdMatch = source.match(/<td[^>]*textAlign:\s*'center'[^>]*>[\s\S]*?Edit Rate[\s\S]*?<\/td>/g)
      assert.ok(actionTdMatch && actionTdMatch.length > 0, 'Must have Action column td')

      // Ensure that in the pricing table rows, Delete button is not present
      const tableBodyMatch = source.match(/<tbody>[\s\S]*?<\/tbody>/)
      assert.ok(tableBodyMatch, 'Must find table body')
      const tableBody = tableBodyMatch[0]
      assert.doesNotMatch(tableBody, /<Trash2/, 'Table row Action column must not have Trash2 / Delete button')
      assert.doesNotMatch(tableBody, /<span>Delete<\/span>/, 'Table row Action column must not have Delete button')
    })

    it('verifies Delete button is relocated to bottom-left of Edit Rate modal for custom models', () => {
      // Find modal footer
      assert.match(source, /justifyContent:\s*'space-between'/, 'Modal footer must use space-between for bottom-left alignment')
      assert.match(source, /editingPricing\.classification === 'custom'/, 'Delete button must be conditional on custom classification')
      assert.match(source, /handleDeletePricingModel\(editingPricing\)/, 'Clicking Delete in modal must call handleDeletePricingModel')
    })

    it('verifies sortedPricingList is utilized for the pricing table and preview', () => {
      assert.match(source, /sortedPricingList\.map/, 'Table must map over sortedPricingList')
    })

    it('verifies Metric Card 1 displays "Total Cost" when unitMode is usd', () => {
      assert.match(source, /unitMode === 'usd' \? 'Total Cost' : 'Total Tokens'/, 'Card 1 must label Total Cost for usd and Total Tokens for tokens')
      assert.doesNotMatch(source, /Total Cost \(Spend\)/, 'Old label Total Cost (Spend) must be removed')
    })

    it('verifies Input/Output Ratio metric card is present before Cached Input Ratio', () => {
      assert.match(source, /Input\/Output Ratio/, 'Must have Input/Output Ratio card')
      const ioIdx = source.indexOf('Input/Output Ratio')
      const cachedIdx = source.indexOf('Cached Input Ratio')
      assert.ok(ioIdx !== -1 && cachedIdx !== -1 && ioIdx < cachedIdx, 'Input/Output Ratio card must appear before Cached Input Ratio card')
      assert.match(source, /Total Input Cost:/, 'Must contain Total Input Cost tooltip')
      assert.match(source, /Total Output Cost:/, 'Must contain Total Output Cost tooltip')
      assert.match(source, /Total Input Token:/, 'Must contain Total Input Token tooltip')
      assert.match(source, /Total Output Token:/, 'Must contain Total Output Token tooltip')
    })

    it('verifies Y-Axis rail with 5 tick levels is present on the Usage Trend chart', () => {
      assert.match(source, /formatYTick/, 'Must define formatYTick function')
      assert.match(source, /\[1\.0,\s*0\.75,\s*0\.5,\s*0\.25,\s*0\.0\]\.map/, 'Must map over 5 tick levels')
    })

    it('verifies Models and Projects gadgets have explainers removed and fixed height 340px with internal scroll', () => {
      assert.doesNotMatch(source, /Canonicalized across thinking levels/, 'Models explainer text must be removed')
      assert.doesNotMatch(source, /Across \{projectBreakdowns\.length\} Project/, 'Projects explainer text must be removed')
      assert.match(source, /height:\s*'340px'[\s\S]*?overflowY:\s*'auto'/, 'Gadgets must specify height 340px and overflowY auto')
    })
  })
})
