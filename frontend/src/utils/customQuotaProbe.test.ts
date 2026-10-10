import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {
  CUSTOM_QUOTA_LAYOUT_TOKENS,
  isCustomQuotaUrlValid,
  formatCustomQuotaStatusText,
  buildDraftModelForQuotaProbe,
  extractQuotaProbeFeedback,
} from './customQuotaProbe.ts'
import type { QuotaResult } from '../types.ts'

describe('Custom Models Quota Endpoint Fetch (Requirement 3.2)', () => {
  const rootSrcDir = path.resolve(import.meta.dirname, '..')
  const customModelsPagePath = path.join(rootSrcDir, 'pages', 'CustomModelsPage.tsx')

  describe('1. Layout Tokens & Invariants', () => {
    it('locks input width shrinking with flex: 1 alongside Fetch button', () => {
      assert.equal(CUSTOM_QUOTA_LAYOUT_TOKENS.inputFlex, 1)
      assert.equal(CUSTOM_QUOTA_LAYOUT_TOKENS.buttonHeight, '30px')
      assert.equal(CUSTOM_QUOTA_LAYOUT_TOKENS.statusMinHeight, '20px')
      assert.equal(CUSTOM_QUOTA_LAYOUT_TOKENS.whiteSpace, 'nowrap')
      assert.equal(CUSTOM_QUOTA_LAYOUT_TOKENS.borderWidth, '1px')
    })

    it('validates HTTP/HTTPS custom quota endpoint URLs correctly', () => {
      assert.ok(isCustomQuotaUrlValid('https://api.openai.com/v1/dashboard/billing/usage'))
      assert.ok(isCustomQuotaUrlValid('http://localhost:8080/v1/usage'))
      assert.ok(!isCustomQuotaUrlValid(''))
      assert.ok(!isCustomQuotaUrlValid('   '))
      assert.ok(!isCustomQuotaUrlValid('ftp://example.com/quota'))
      assert.ok(!isCustomQuotaUrlValid('not-a-url'))
    })
  })

  describe('2. Custom Quota Probe Logic & Feedback Extraction', () => {
    it('builds draft model with custom quota endpoint correctly', () => {
      const draft = buildDraftModelForQuotaProbe({
        name: 'claude-3-7-sonnet',
        providerType: 'anthropic',
        baseUrl: 'https://api.anthropic.com',
        apiKey: 'sk-ant-test',
        customQuotaEndpoint: 'https://api.anthropic.com/v1/usage',
      })

      assert.equal(draft.name, 'claude-3-7-sonnet')
      assert.equal(draft.provider_type, 'anthropic')
      assert.equal(draft.custom_quota_endpoint, 'https://api.anthropic.com/v1/usage')
      assert.equal(draft.base_url, 'https://api.anthropic.com')
      assert.equal(draft.api_key, 'sk-ant-test')
    })

    it('extracts success feedback with latency for balance response', () => {
      const quotaRes: QuotaResult = {
        quota_type: 'balance',
        balance_value: '$48.50',
        fraction: 0.85,
        has_percentage: true,
      }
      const feedback = extractQuotaProbeFeedback(quotaRes, 142)
      assert.equal(feedback.type, 'success')
      assert.equal(feedback.message, 'Balance: $48.50')
      assert.equal(feedback.latencyMs, 142)

      const formatted = formatCustomQuotaStatusText(feedback)
      assert.equal(formatted, 'Success (142ms): Balance: $48.50')
    })

    it('extracts success feedback with latency for token quota response', () => {
      const quotaRes: QuotaResult = {
        quota_type: 'quota',
        quota_value: '500,000 tokens',
        fraction: 0.5,
        has_percentage: true,
      }
      const feedback = extractQuotaProbeFeedback(quotaRes, 88)
      assert.equal(feedback.type, 'success')
      assert.equal(feedback.message, 'Quota: 500,000 tokens')
      assert.equal(feedback.latencyMs, 88)

      const formatted = formatCustomQuotaStatusText(feedback)
      assert.equal(formatted, 'Success (88ms): Quota: 500,000 tokens')
    })

    it('extracts error feedback when probe returns untracked/na', () => {
      const quotaRes: QuotaResult = {
        quota_type: 'na',
        fraction: null,
        has_percentage: false,
        message: 'Endpoint returned 404 Not Found',
      }
      const feedback = extractQuotaProbeFeedback(quotaRes, 310)
      assert.equal(feedback.type, 'error')
      assert.equal(feedback.message, 'Endpoint returned 404 Not Found')
      assert.equal(feedback.latencyMs, 310)

      const formatted = formatCustomQuotaStatusText(feedback)
      assert.equal(formatted, 'Failed (310ms): Endpoint returned 404 Not Found')
    })

    it('handles null or failed probe gracefully', () => {
      const feedback = extractQuotaProbeFeedback(null, 5000)
      assert.equal(feedback.type, 'error')
      assert.ok(feedback.message.includes('No response'))
      assert.equal(feedback.latencyMs, 5000)
    })
  })

  describe('3. CustomModelsPage.tsx Source Code Verification', () => {
    it('verifies shrunk input and Fetch button immediately adjacent in CustomModelsPage.tsx', () => {
      assert.ok(fs.existsSync(customModelsPagePath), 'CustomModelsPage.tsx must exist')
      const source = fs.readFileSync(customModelsPagePath, 'utf8')

      // Check flex layout with input flex: 1 and button adjacent
      assert.ok(source.includes("display: 'flex', gap: '8px', alignItems: 'center'"))
      assert.ok(source.includes('flex: 1'))
      assert.ok(source.includes('Custom Quota Endpoint (optional)'))
      assert.ok(source.includes('onClick={handleFetchCustomQuotaEndpoint}'))
      assert.ok(source.includes('<span>Fetch</span>'))
      assert.ok(source.includes('<span>Fetching...</span>'))
    })

    it('verifies real API connection to api.fetchCustomModelQuota', () => {
      const source = fs.readFileSync(customModelsPagePath, 'utf8')
      assert.ok(
        source.includes('api.fetchCustomModelQuota(draftModel)'),
        'CustomModelsPage must call api.fetchCustomModelQuota with the draft model'
      )
    })

    it('verifies inline feedback container with min-height eliminates disruptive layout shift', () => {
      const source = fs.readFileSync(customModelsPagePath, 'utf8')
      assert.ok(
        source.includes("minHeight: '20px'"),
        'CustomModelsPage must provide fixed min-height for inline status container'
      )
      assert.ok(source.includes('customQuotaFeedback'))
      assert.ok(source.includes('latencyMs'))
      assert.ok(source.includes('CheckCircle2'))
      assert.ok(source.includes('AlertCircle'))
    })

    it('verifies feedback reset on input change and modal reset', () => {
      const source = fs.readFileSync(customModelsPagePath, 'utf8')
      assert.ok(source.includes('if (customQuotaFeedback) setCustomQuotaFeedback(null)'))
      assert.ok(source.includes('setCustomQuotaFeedback(null)'))
    })
  })
})
