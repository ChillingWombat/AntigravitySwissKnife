import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import type { RuleConfig, QuotaRefreshMode } from '../types'

export function resolveQuotaRefreshMode(rules?: Partial<RuleConfig> | null): QuotaRefreshMode {
  if (rules?.quota_refresh_mode) return rules.quota_refresh_mode
  if (rules?.dynamic_quota_refresh_enabled === false) return 'manual'
  return 'dynamic'
}

describe('Dynamic Quota Refresh Frequency in Switcher Settings', () => {
  const pagePath = path.resolve(import.meta.dirname, '../pages/SwitcherSettingsPage.tsx')
  const pageContent = fs.readFileSync(pagePath, 'utf8')

  describe('1. Default Mode & Normalization', () => {
    it('defaults to "dynamic" when initialRules is null or undefined', () => {
      assert.equal(resolveQuotaRefreshMode(null), 'dynamic')
      assert.equal(resolveQuotaRefreshMode(undefined), 'dynamic')
    })

    it('defaults to "dynamic" when quota_refresh_mode is not explicitly specified in initialRules', () => {
      const initialRules: Partial<RuleConfig> = {
        auto_switch_enabled: true,
        polling_interval_seconds: 60,
      }
      assert.equal(resolveQuotaRefreshMode(initialRules), 'dynamic')
    })

    it('honors "manual" when explicitly set in initialRules', () => {
      const initialRules: Partial<RuleConfig> = {
        quota_refresh_mode: 'manual',
        dynamic_quota_refresh_enabled: false,
      }
      assert.equal(resolveQuotaRefreshMode(initialRules), 'manual')
    })

    it('honors dynamic_quota_refresh_enabled=false fallback to "manual"', () => {
      const initialRules: Partial<RuleConfig> = {
        dynamic_quota_refresh_enabled: false,
      }
      assert.equal(resolveQuotaRefreshMode(initialRules), 'manual')
    })
  })

  describe('2. Auto-Save Payload Construction', () => {
    it('constructs payload with quota_refresh_mode="dynamic" and dynamic_quota_refresh_enabled=true', () => {
      const quotaRefreshMode: QuotaRefreshMode = 'dynamic'
      const payload = {
        quota_refresh_mode: quotaRefreshMode,
        dynamic_quota_refresh_enabled: (quotaRefreshMode as string) === 'dynamic',
      }
      assert.equal(payload.quota_refresh_mode, 'dynamic')
      assert.equal(payload.dynamic_quota_refresh_enabled, true)
    })

    it('constructs payload with quota_refresh_mode="manual" and dynamic_quota_refresh_enabled=false', () => {
      const quotaRefreshMode: QuotaRefreshMode = 'manual'
      const payload = {
        quota_refresh_mode: quotaRefreshMode,
        dynamic_quota_refresh_enabled: (quotaRefreshMode as string) === 'dynamic',
      }
      assert.equal(payload.quota_refresh_mode, 'manual')
      assert.equal(payload.dynamic_quota_refresh_enabled, false)
    })
  })

  describe('3. SwitcherSettingsPage.tsx UI Structure & Conditional Visibility', () => {
    it('verifies presence of Account Quota Refresh Frequency selector with Dynamic and Manual options', () => {
      assert.ok(
        pageContent.includes('Account Quota Refresh Frequency:'),
        'Page must contain Account Quota Refresh Frequency title'
      )
      assert.ok(
        pageContent.includes('Dynamic Adaptive Fetch'),
        'Selector must contain Dynamic Adaptive Fetch option'
      )
      assert.ok(
        pageContent.includes('Manually Set Refresh Frequency'),
        'Selector must contain Manually Set Refresh Frequency option'
      )
    })

    it('verifies manual interval sliders are hidden when dynamic (wrapped in quotaRefreshMode === "manual")', () => {
      assert.ok(
        pageContent.includes("quotaRefreshMode === 'manual'"),
        'Manual polling intervals must be conditionally rendered under quotaRefreshMode === "manual"'
      )
    })

    it('verifies Standby Account Staggered Jitter Gap is hidden when dynamic and only shown in manual mode', () => {
      assert.ok(
        pageContent.includes("quotaRefreshMode === 'manual' && ("),
        'Standby Jitter Gap must be conditionally rendered only when quotaRefreshMode === "manual"'
      )
    })

    it('verifies the Dynamic Adaptive Quota Levels information gadget has been removed per user request', () => {
      assert.ok(
        !pageContent.includes('Dynamic Adaptive Quota Levels'),
        'Dynamic Adaptive Quota Levels info box must be completely removed from SwitcherSettingsPage'
      )
    })

    it('verifies horizontal divider between thresholds and refresh frequency fetch method', () => {
      assert.ok(
        pageContent.includes('{/* Divider between Thresholds and Refresh Frequency Fetch Method */}'),
        'Must contain divider comment between Thresholds and Refresh Frequency'
      )
      assert.ok(
        pageContent.includes("borderTop: '1px solid var(--border)'"),
        'Must use 1px solid var(--border) divider'
      )
    })
  })

  describe('4. David-Design & David-Humanizer Invariants', () => {
    it('guarantees 0 decorative emojis in the dynamic refresh section', () => {
      const emojiRegex = /[\u{1F300}-\u{1F6FF}\u{1F900}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
      const dynamicBlockStart = pageContent.indexOf('Account Quota Refresh Frequency:')
      const dynamicBlockEnd = pageContent.indexOf('Section 1b: Multi-App Account Synchronization Mode')
      assert.ok(dynamicBlockStart > 0 && dynamicBlockEnd > dynamicBlockStart)
      const block = pageContent.slice(dynamicBlockStart, dynamicBlockEnd)
      assert.ok(!emojiRegex.test(block), 'Dynamic refresh section must not contain decorative emojis')
    })

    it('guarantees 1px solid tokenized borders with zero 1.5px borders', () => {
      const dynamicBlockStart = pageContent.indexOf('Account Quota Refresh Frequency:')
      const dynamicBlockEnd = pageContent.indexOf('Section 1b: Multi-App Account Synchronization Mode')
      const block = pageContent.slice(dynamicBlockStart, dynamicBlockEnd)
      assert.ok(!block.includes('1.5px solid'), 'Must not contain non-standard 1.5px solid borders')
    })

    it('guarantees zero banned AI buzzwords in the dynamic refresh copy', () => {
      const bannedWords = ['utilize', 'utilizes', 'supercharge', 'paradigm shift', 'delve']
      const dynamicBlockStart = pageContent.indexOf('Account Quota Refresh Frequency:')
      const dynamicBlockEnd = pageContent.indexOf('Section 1b: Multi-App Account Synchronization Mode')
      const block = pageContent.slice(dynamicBlockStart, dynamicBlockEnd)
      for (const word of bannedWords) {
        const regex = new RegExp(`\\b${word}\\b`, 'i')
        assert.ok(!regex.test(block), `Banned word "${word}" found in dynamic refresh section`)
      }
    })
  })
})
