import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  MODEL_SOURCE_LABELS,
  normalizeModelSourceHierarchy,
} from './modelSourceHierarchy.ts'

describe('modelSourceHierarchy utility', () => {
  describe('MODEL_SOURCE_LABELS', () => {
    it('labels every canonical key', () => {
      assert.deepEqual(MODEL_SOURCE_LABELS, {
        gemini: 'Gemini Native Models',
        custom: 'Custom Models',
        non_gemini: 'Non-Gemini Native Models',
        credits: 'AI Credits',
      })
    })
  })

  describe('normalizeModelSourceHierarchy', () => {
    it('returns the default order for undefined input', () => {
      assert.deepEqual(normalizeModelSourceHierarchy(undefined), [
        'gemini',
        'custom',
        'non_gemini',
        'credits',
      ])
    })

    it('returns the default order for null input', () => {
      assert.deepEqual(normalizeModelSourceHierarchy(null), [
        'gemini',
        'custom',
        'non_gemini',
        'credits',
      ])
    })

    it('returns the default order for an empty array', () => {
      assert.deepEqual(normalizeModelSourceHierarchy([]), [
        'gemini',
        'custom',
        'non_gemini',
        'credits',
      ])
    })

    it('maps legacy keys to canonical keys', () => {
      assert.deepEqual(
        normalizeModelSourceHierarchy(['custom_model', 'ai_credits', 'gemini', 'non_gemini']),
        ['custom', 'credits', 'gemini', 'non_gemini']
      )
    })

    it('drops unknown keys', () => {
      assert.deepEqual(
        normalizeModelSourceHierarchy(['gemini', 'bogus_source', 'custom']),
        ['gemini', 'custom', 'non_gemini', 'credits']
      )
    })

    it('dedupes keys keeping the first occurrence', () => {
      assert.deepEqual(
        normalizeModelSourceHierarchy(['custom', 'gemini', 'custom_model', 'credits', 'non_gemini', 'credits']),
        ['custom', 'gemini', 'credits', 'non_gemini']
      )
    })

    it('appends missing keys in default order', () => {
      assert.deepEqual(normalizeModelSourceHierarchy(['credits']), [
        'credits',
        'gemini',
        'custom',
        'non_gemini',
      ])
    })

    it('preserves a fully-specified canonical order', () => {
      assert.deepEqual(
        normalizeModelSourceHierarchy(['non_gemini', 'credits', 'custom', 'gemini']),
        ['non_gemini', 'credits', 'custom', 'gemini']
      )
    })
  })
})
