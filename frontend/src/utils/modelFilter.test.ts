import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {
  filterModels,
  filterFetchedModels,
  computeModelFilterStats,
  resolveEffectiveCustomModel,
} from './modelFilter.ts'
import type { CustomModel, ModelInfo } from '../types.ts'

const mockModels: CustomModel[] = [
  {
    id: 'm1',
    name: 'deepseek-v4-flash',
    display_name: 'DeepSeek V4.1 Flash',
    provider_type: 'custom',
    base_url: 'https://opencode.ai/zen/go',
    project_mappings: ['*'],
    enabled: true,
    is_default: true,
    quota_type: 'quota',
    prepaid_balance: 0,
    total_budget: 0,
    quota_fraction: 1.0,
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
  },
  {
    id: 'm2',
    name: 'claude-3-7-sonnet',
    display_name: 'Claude 3.7 Sonnet',
    provider_type: 'anthropic',
    base_url: 'https://api.anthropic.com',
    project_mappings: ['*'],
    enabled: false,
    is_default: false,
    quota_type: 'balance',
    prepaid_balance: 10,
    total_budget: 50,
    quota_fraction: 0.2,
    created_at: '2026-10-02T00:00:00Z',
    updated_at: '2026-10-02T00:00:00Z',
  },
  {
    id: 'm3',
    name: 'gemini-2-5-pro',
    display_name: 'Gemini 2.5 Pro',
    provider_type: 'gemini',
    base_url: 'https://generativelanguage.googleapis.com',
    project_mappings: ['*'],
    enabled: true,
    is_default: false,
    quota_type: 'quota',
    prepaid_balance: 0,
    total_budget: 0,
    quota_fraction: 0.85,
    created_at: '2026-10-03T00:00:00Z',
    updated_at: '2026-10-03T00:00:00Z',
  },
]

describe('modelFilter utility', () => {
  describe('filterModels', () => {
    it('returns all models when filter is "all"', () => {
      const result = filterModels(mockModels, 'all')
      assert.equal(result.length, 3)
      assert.deepEqual(
        result.map((m) => m.id),
        ['m1', 'm2', 'm3']
      )
    })

    it('returns only enabled models when filter is "enabled"', () => {
      const result = filterModels(mockModels, 'enabled')
      assert.equal(result.length, 2)
      assert.deepEqual(
        result.map((m) => m.id),
        ['m1', 'm3']
      )
      assert.ok(result.every((m) => m.enabled === true))
    })

    it('returns only disabled models when filter is "disabled"', () => {
      const result = filterModels(mockModels, 'disabled')
      assert.equal(result.length, 1)
      assert.deepEqual(
        result.map((m) => m.id),
        ['m2']
      )
      assert.ok(result.every((m) => m.enabled === false))
    })

    it('handles empty model array safely', () => {
      assert.deepEqual(filterModels([], 'all'), [])
      assert.deepEqual(filterModels([], 'enabled'), [])
      assert.deepEqual(filterModels([], 'disabled'), [])
    })

    it('defaults to "all" when filter parameter is omitted', () => {
      const result = filterModels(mockModels)
      assert.equal(result.length, 3)
    })
  })

  describe('computeModelFilterStats', () => {
    it('correctly tallies total, enabled, and disabled counts', () => {
      const stats = computeModelFilterStats(mockModels)
      assert.deepEqual(stats, {
        total: 3,
        enabled: 2,
        disabled: 1,
      })
    })

    it('returns zeroes for empty models list', () => {
      const stats = computeModelFilterStats([])
      assert.deepEqual(stats, {
        total: 0,
        enabled: 0,
        disabled: 0,
      })
    })
  })

  describe('resolveEffectiveCustomModel', () => {
    it('returns blank string when available custom models list is empty', () => {
      assert.equal(resolveEffectiveCustomModel([]), '')
      assert.equal(resolveEffectiveCustomModel([], 'm1'), '')
      assert.equal(resolveEffectiveCustomModel(undefined as any), '')
    })

    it('defaults to the first model in the list when no default is set or invalid', () => {
      const models = [{ id: 'custom-gpt4' }, { id: 'custom-claude' }]
      assert.equal(resolveEffectiveCustomModel(models), 'custom-gpt4')
      assert.equal(resolveEffectiveCustomModel(models, ''), 'custom-gpt4')
      assert.equal(resolveEffectiveCustomModel(models, 'non-existent-id'), 'custom-gpt4')
    })

    it('preserves the manually chosen model if it exists in the available list', () => {
      const models = [{ id: 'custom-gpt4' }, { id: 'custom-claude' }, { id: 'custom-gemini' }]
      assert.equal(resolveEffectiveCustomModel(models, 'custom-claude'), 'custom-claude')
      assert.equal(resolveEffectiveCustomModel(models, 'custom-gemini'), 'custom-gemini')
    })

    it('handles whitespace trimming and invalid/null array elements safely', () => {
      const models = [
        null as any,
        { id: '' },
        { id: '   ' },
        { id: 'custom-claude' },
        { id: 'custom-gemini' },
      ]
      assert.equal(resolveEffectiveCustomModel(models), 'custom-claude')
      assert.equal(resolveEffectiveCustomModel(models, '  custom-gemini  '), 'custom-gemini')
      assert.equal(resolveEffectiveCustomModel([null as any, { id: ' ' }]), '')
    })
  })

  describe('filterFetchedModels', () => {
    const fetched: ModelInfo[] = [
      {
        id: 'models/gemini-2.5-pro',
        display_name: 'Gemini 2.5 Pro',
        description: 'Advanced reasoning model with thinking capabilities',
      },
      {
        id: 'models/claude-3-7-sonnet@20250219',
        display_name: 'Claude 3.7 Sonnet',
        description: 'Hybrid reasoning and standard generation model',
      },
      {
        id: 'deepseek-chat',
        display_name: 'DeepSeek V3',
        description: 'High efficiency open weights model',
      },
    ]

    it('returns empty array when input models is empty or null', () => {
      assert.deepEqual(filterFetchedModels([], 'pro'), [])
      assert.deepEqual(filterFetchedModels(null as any, 'pro'), [])
    })

    it('returns all models when search query is empty or whitespace', () => {
      assert.equal(filterFetchedModels(fetched, '').length, 3)
      assert.equal(filterFetchedModels(fetched, '   \t  ').length, 3)
    })

    it('matches by cleaned id or raw id', () => {
      const byClean = filterFetchedModels(fetched, 'claude-3-7-sonnet')
      assert.equal(byClean.length, 1)
      assert.equal(byClean[0].display_name, 'Claude 3.7 Sonnet')
    })

    it('matches by display name or description case-insensitively', () => {
      const byDesc = filterFetchedModels(fetched, 'hybrid reasoning')
      assert.equal(byDesc.length, 1)
      assert.equal(byDesc[0].id, 'models/claude-3-7-sonnet@20250219')
    })

    it('supports multi-token queries matching across fields', () => {
      const multi = filterFetchedModels(fetched, 'gemini thinking')
      assert.equal(multi.length, 1)
      assert.equal(multi[0].id, 'models/gemini-2.5-pro')
    })

    it('returns empty array when query matches no model', () => {
      const results = filterFetchedModels(fetched, 'non-existent-model-xyz')
      assert.deepEqual(results, [])
    })

    it('filters models with parenthesized context annotations', () => {
      const annotatedList: ModelInfo[] = [
        { id: 'deepseek-v4-flash (1,048,576 ctx)', display_name: 'DeepSeek Flash' },
      ]
      const results = filterFetchedModels(annotatedList, 'deepseek-v4-flash')
      assert.equal(results.length, 1)
    })

    it('matches models with complex tags, ports, and version punctuation', () => {
      const taggedList: ModelInfo[] = [
        { id: 'deepseek-r1:14b', display_name: 'DeepSeek R1 14B' },
        { id: 'meta-llama/llama-3.1-70b-instruct', display_name: 'Llama 3.1 70B' },
        { id: 'claude-3-7-sonnet@20250219', display_name: 'Claude 3.7 Sonnet' },
        { id: 'qwen3-embedding:0.6b', display_name: 'Qwen3 Embedding:0.6b' },
        { id: 'all-minilm:latest', display_name: 'All Minilm:latest' },
      ]
      assert.equal(filterFetchedModels(taggedList, '14b').length, 1)
      assert.equal(filterFetchedModels(taggedList, 'meta-llama 70b').length, 1)
      assert.equal(filterFetchedModels(taggedList, '@20250219').length, 1)
      assert.equal(filterFetchedModels(taggedList, 'CLAUDE SONNET').length, 1)
      assert.equal(filterFetchedModels(taggedList, 'qwen 0.6b').length, 1)
      assert.equal(filterFetchedModels(taggedList, 'minilm').length, 1)
    })

    it('handles malformed or sparse model objects without throwing', () => {
      const sparse: ModelInfo[] = [
        null as any,
        { id: '' },
        { id: 'gpt-4o', display_name: undefined },
      ]
      assert.equal(filterFetchedModels(sparse, 'gpt').length, 1)
    })
  })

  describe('CustomModelsPage Fetched Models Dropdown Contract', () => {
    const pagePath = path.resolve(import.meta.dirname, '../pages/CustomModelsPage.tsx')
    const pageContent = fs.readFileSync(pagePath, 'utf8')
    const normalizedContent = pageContent.replace(/\s+/g, ' ')

    it('ensures dropdown stays open when user enters text in model name field', () => {
      assert.equal(
        /if\s*\(\s*val\.trim\(\)\s*\)\s*\{\s*setShowModelDropdown\(false\)/.test(normalizedContent),
        false,
        'CustomModelsPage must not dismiss dropdown when input is entered'
      )
      assert.ok(
        /if\s*\(\s*fetchedModels\.length\s*>\s*0\s*\)\s*\{\s*setShowModelDropdown\(true\)/.test(normalizedContent),
        'CustomModelsPage must ensure dropdown is shown when typing and fetched models exist'
      )
    })

    it('verifies dropdown options are filtered by user input query', () => {
      assert.ok(
        pageContent.includes('filterFetchedModels(fetchedModels, modelName)'),
        'CustomModelsPage must compute filteredFetchedModels using filterFetchedModels'
      )
      assert.ok(
        pageContent.includes('filteredFetchedModels.map('),
        'CustomModelsPage must render filtered list of fetched models'
      )
    })

    it('verifies dropdown header displays filtered count alongside total discovered count', () => {
      assert.ok(
        pageContent.includes('filteredFetchedModels.length} of ${fetchedModels.length'),
        'CustomModelsPage must display filtered count vs total in dropdown header'
      )
    })

    it('verifies dropdown displays empty state message when no models match query', () => {
      assert.ok(
        pageContent.includes('No models match'),
        'CustomModelsPage must display empty state message when filter yields zero matches'
      )
    })

    it('verifies clicking or focusing input re-opens dropdown even if input has existing text', () => {
      assert.equal(
        pageContent.includes('!modelName.trim() && fetchedModels.length > 0'),
        false,
        'CustomModelsPage must allow opening dropdown on click/focus regardless of whether modelName is set'
      )
    })

    it('verifies Escape key dismisses dropdown cleanly', () => {
      assert.ok(
        pageContent.includes("if (e.key === 'Escape')"),
        'CustomModelsPage must handle Escape key on model input'
      )
    })
  })
})



