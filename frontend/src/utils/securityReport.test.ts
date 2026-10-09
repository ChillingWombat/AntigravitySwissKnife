import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  resolveSecurityGrade,
  getRiskPresentation,
  formatSecurityReportMarkdown,
} from './securityAuditReport.ts'
import { sanitizeConnectionErrorMessage } from './testConnectionButton.ts'
import type { SecurityAuditReport } from '../types.ts'

describe('SecurityReportModal & Audit Report Logic', () => {
  describe('resolveSecurityGrade', () => {
    it('preserves explicitly provided security grades', () => {
      assert.equal(resolveSecurityGrade(80, 'A+'), 'A+')
      assert.equal(resolveSecurityGrade(10, 'B'), 'B')
      assert.equal(resolveSecurityGrade(50, '  A  '), 'A')
    })

    it('computes correct grade bands from risk scores', () => {
      assert.equal(resolveSecurityGrade(0), 'A+')
      assert.equal(resolveSecurityGrade(5), 'A+')
      assert.equal(resolveSecurityGrade(6), 'A')
      assert.equal(resolveSecurityGrade(15), 'A')
      assert.equal(resolveSecurityGrade(16), 'B')
      assert.equal(resolveSecurityGrade(30), 'B')
      assert.equal(resolveSecurityGrade(31), 'C')
      assert.equal(resolveSecurityGrade(50), 'C')
      assert.equal(resolveSecurityGrade(51), 'D')
      assert.equal(resolveSecurityGrade(70), 'D')
      assert.equal(resolveSecurityGrade(71), 'F')
      assert.equal(resolveSecurityGrade(100), 'F')
    })

    it('handles undefined, null, or NaN risk scores gracefully', () => {
      assert.equal(resolveSecurityGrade(undefined), 'A+')
      assert.equal(resolveSecurityGrade(null), 'A+')
      assert.equal(resolveSecurityGrade(NaN), 'A+')
    })
  })

  describe('getRiskPresentation', () => {
    it('handles low risk level', () => {
      const pres = getRiskPresentation('low')
      assert.equal(pres.isLow, true)
      assert.equal(pres.isMed, false)
      assert.equal(pres.isHighOrCrit, false)
      assert.equal(pres.statusColor, 'var(--green)')
      assert.equal(pres.riskBadgeClass, 'badge-green')
    })

    it('handles medium risk level', () => {
      const pres = getRiskPresentation('medium')
      assert.equal(pres.isLow, false)
      assert.equal(pres.isMed, true)
      assert.equal(pres.isHighOrCrit, false)
      assert.equal(pres.statusColor, 'var(--yellow)')
      assert.equal(pres.riskBadgeClass, 'badge-yellow')
    })

    it('handles high risk level as critical danger tier', () => {
      const pres = getRiskPresentation('high')
      assert.equal(pres.isLow, false)
      assert.equal(pres.isMed, false)
      assert.equal(pres.isHighOrCrit, true)
      assert.equal(pres.statusColor, 'var(--red)')
      assert.equal(pres.riskBadgeClass, 'badge-red')
    })

    it('handles critical risk level as critical danger tier', () => {
      const pres = getRiskPresentation('critical')
      assert.equal(pres.isLow, false)
      assert.equal(pres.isMed, false)
      assert.equal(pres.isHighOrCrit, true)
      assert.equal(pres.statusColor, 'var(--red)')
      assert.equal(pres.riskBadgeClass, 'badge-red')
    })

    it('handles whitespace, mixed case, and unknown inputs safely', () => {
      const presCase = getRiskPresentation('  CrItIcAl  ')
      assert.equal(presCase.isHighOrCrit, true)

      const presUnknown = getRiskPresentation('unknown')
      assert.equal(presUnknown.isLow, false)
      assert.equal(presUnknown.isMed, false)
      assert.equal(presUnknown.isHighOrCrit, false)
      assert.equal(presUnknown.riskBadgeClass, 'badge-neutral')
    })
  })

  describe('formatSecurityReportMarkdown', () => {
    const sampleReport: SecurityAuditReport = {
      model_id: 'deepseek-v3',
      endpoint: 'https://api.deepseek.com/v1',
      provider_type: 'openai',
      audited_at: '2026-10-09T04:26:00Z',
      risk_score: 12,
      risk_level: 'low',
      summary: 'Audit passed cleanly. Verified transport and valid proxy lineage.',
      probes: [
        {
          id: 'tls',
          name: 'TLS Encryption',
          category: 'Transport',
          description: 'TLS check',
          status: 'passed',
          details: 'Valid TLS 1.3 certificate',
          evidence: 'Strict HTTPS enforced',
        },
        {
          id: 'headers',
          name: 'Header Lineage',
          category: 'Integrity',
          description: 'Proxy header inspection',
          status: 'passed',
          details: 'No suspicious relay headers detected',
        },
      ],
      recommendations: [
        'Enforce key rotation every 90 days.',
      ],
    }

    it('produces structured markdown report with zero decorative emojis', () => {
      const md = formatSecurityReportMarkdown(sampleReport)
      assert.ok(md.includes('# Security Audit Report'))
      assert.ok(md.includes('**Model:** deepseek-v3'))
      assert.ok(md.includes('**Endpoint:** https://api.deepseek.com/v1'))
      assert.ok(md.includes('**Protocol:** openai'))
      assert.ok(md.includes('**Risk Level:** LOW (Grade A, Score 12/100)'))
      assert.ok(md.includes('## Summary'))
      assert.ok(md.includes(sampleReport.summary))
      assert.ok(md.includes('## Security Checks (2/2 Passed)'))
      assert.ok(md.includes('- [PASSED] **TLS Encryption** (Transport): Valid TLS 1.3 certificate'))
      assert.ok(md.includes('Evidence: `Strict HTTPS enforced`'))
      assert.ok(md.includes('1. Enforce key rotation every 90 days.'))

      // Strict David-Design: zero emojis in generated markdown
      const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
      assert.equal(emojiRegex.test(md), false, 'Report markdown must not contain emojis')
    })

    it('handles empty probes and recommendations without throwing', () => {
      const emptyReport: SecurityAuditReport = {
        model_id: 'minimal-model',
        endpoint: 'https://api.example.com',
        provider_type: 'openai',
        audited_at: '',
        risk_score: 95,
        risk_level: 'critical',
        summary: '',
        probes: [],
        recommendations: [],
      }

      const md = formatSecurityReportMarkdown(emptyReport)
      assert.ok(md.includes('**Risk Level:** CRITICAL (Grade F, Score 95/100)'))
      assert.ok(md.includes('No probe details recorded') || md.includes('## Security Checks (0/0 Passed)'))
    })
  })

  describe('sanitizeConnectionErrorMessage', () => {
    it('removes HTTP 200 / status codes from messages', () => {
      assert.equal(
        sanitizeConnectionErrorMessage('HTTP 200 OK: Everything is fine'),
        'OK: Everything is fine'
      )
      assert.equal(
        sanitizeConnectionErrorMessage('Provider responded with 200 OK in 120ms'),
        'Provider responded with OK in 120ms'
      )
    })

    it('removes HTTP error status codes like 401, 404, 500', () => {
      assert.equal(
        sanitizeConnectionErrorMessage('Authentication failed (401 Unauthorized)'),
        'Authentication failed (Unauthorized)'
      )
      assert.equal(
        sanitizeConnectionErrorMessage('Request failed with status code 500'),
        'Request failed'
      )
      assert.equal(
        sanitizeConnectionErrorMessage('Endpoint returned 404 Not Found at /v1/chat'),
        'Endpoint returned Not Found at /v1/chat'
      )
    })

    it('handles empty or undefined message safely', () => {
      assert.equal(sanitizeConnectionErrorMessage(''), 'Connection test failed')
      assert.equal(sanitizeConnectionErrorMessage(undefined), 'Connection test failed')
    })
  })
})
