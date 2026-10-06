import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  filterModels,
  computeModelFilterStats,
  resolveEffectiveCustomModel,
} from './modelFilter.ts'
import type { CustomModel } from '../types.ts'

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
})

