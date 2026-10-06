import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  extractCleanModelId,
  extractContextWindow,
  detectThinkingLevels,
} from './modelExtraction.ts'

describe('modelExtraction utility', () => {
  describe('extractCleanModelId', () => {
    it('strips context annotations in parentheses', () => {
      assert.equal(
        extractCleanModelId('deepseek-v4-flash (1,048,576 ctx)'),
        'deepseek-v4-flash'
      )
      assert.equal(
        extractCleanModelId('gpt-4o (128k context)'),
        'gpt-4o'
      )
      assert.equal(
        extractCleanModelId('claude-3-7-sonnet-20250219 (200k tokens)'),
        'claude-3-7-sonnet-20250219'
      )
    })

    it('strips Gemini models/ prefix', () => {
      assert.equal(
        extractCleanModelId('models/gemini-2.5-pro'),
        'gemini-2.5-pro'
      )
      assert.equal(
        extractCleanModelId('models/gemini-2.5-flash (1,048,576 ctx)'),
        'gemini-2.5-flash'
      )
    })

    it('strips thinking/reasoning badges and bracket tags', () => {
      assert.equal(
        extractCleanModelId('deepseek-v4-flash 🧠 [Thinking] (1,048,576 ctx)'),
        'deepseek-v4-flash'
      )
      assert.equal(
        extractCleanModelId('gpt-4o [default]'),
        'gpt-4o'
      )
    })

    it('preserves provider model namespaces while trimming whitespace', () => {
      assert.equal(
        extractCleanModelId('  deepseek-ai/DeepSeek-V3  '),
        'deepseek-ai/DeepSeek-V3'
      )
      assert.equal(
        extractCleanModelId('deepseek-ai/DeepSeek-R1 (64k ctx)'),
        'deepseek-ai/DeepSeek-R1'
      )
    })
  })

  describe('extractContextWindow', () => {
    it('prefers numeric properties from model metadata object', () => {
      assert.equal(extractContextWindow('', { context_window: 1048576 }), 1048576)
      assert.equal(extractContextWindow('', { inputTokenLimit: 2097152 }), 2097152)
      assert.equal(extractContextWindow('', { context_length: 65536 }), 65536)
    })

    it('extracts formatted token counts from string annotations', () => {
      assert.equal(extractContextWindow('model (1,048,576 ctx)'), 1048576)
      assert.equal(extractContextWindow('model (128000 ctx)'), 128000)
    })

    it('extracts k and m multipliers from string annotations', () => {
      assert.equal(extractContextWindow('model (128k context)'), 131072)
      assert.equal(extractContextWindow('model (200k)'), 204800)
      assert.equal(extractContextWindow('model (32k)'), 32768)
      assert.equal(extractContextWindow('model (1m ctx)'), 1048576)
      assert.equal(extractContextWindow('model (2M ctx)'), 2097152)
    })

    it('returns null when no context window information is present', () => {
      assert.equal(extractContextWindow('plain-model-id'), null)
      assert.equal(extractContextWindow(''), null)
    })
  })

  describe('detectThinkingLevels', () => {
    it('returns levels from model info if provided', () => {
      const levels = detectThinkingLevels('custom-model', {
        supports_thinking: true,
        thinking_levels: ['low', 'medium', 'high'],
      })
      assert.deepEqual(levels, ['low', 'medium', 'high'])
    })

    it('infers thinking levels for reasoning model families', () => {
      assert.deepEqual(detectThinkingLevels('deepseek-r1'), ['low', 'medium', 'high'])
      assert.deepEqual(detectThinkingLevels('o1-preview'), ['low', 'medium', 'high'])
      assert.deepEqual(detectThinkingLevels('o3-mini'), ['low', 'medium', 'high'])
      assert.deepEqual(detectThinkingLevels('claude-3-7-sonnet'), ['low', 'medium', 'high'])
      assert.deepEqual(detectThinkingLevels('claude-opus-4-6'), ['low', 'medium', 'high'])
      assert.deepEqual(detectThinkingLevels('gemini-2.5-pro'), ['low', 'medium', 'high'])
      assert.deepEqual(detectThinkingLevels('gemini-3.8-flash'), ['low', 'medium', 'high'])
    })

    it('returns empty array for non-reasoning models without thinking support', () => {
      assert.deepEqual(detectThinkingLevels('gpt-4o-mini'), [])
      assert.deepEqual(detectThinkingLevels('claude-3-5-haiku'), [])
    })
  })
})
