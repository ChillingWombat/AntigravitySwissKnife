import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  getTestConnectionButtonPresentation,
  TEST_BUTTON_WIDTH_PX,
  TEST_BUTTON_HEIGHT_PX,
} from './testConnectionButton.ts'
import type { TestResult } from '../types.ts'

describe('testConnectionButton utility', () => {
  it('locks dimensions to 160x32px (multiples of 4px grid unit)', () => {
    assert.equal(TEST_BUTTON_WIDTH_PX, 160)
    assert.equal(TEST_BUTTON_HEIGHT_PX, 32)
    assert.equal(TEST_BUTTON_WIDTH_PX % 4, 0)
    assert.equal(TEST_BUTTON_HEIGHT_PX % 4, 0)
  })

  it('returns idle presentation when not testing and no previous result', () => {
    const pres = getTestConnectionButtonPresentation({
      isTesting: false,
      testResult: null,
      baseUrl: 'https://api.openai.com/v1',
    })

    assert.equal(pres.state, 'idle')
    assert.equal(pres.icon, 'zap')
    assert.equal(pres.spinIcon, false)
    assert.equal(pres.label, 'Test Connection')
    assert.equal(pres.disabled, false)
    assert.equal(pres.style.width, '160px')
    assert.equal(pres.style.minWidth, '160px')
    assert.equal(pres.style.maxWidth, '160px')
    assert.equal(pres.style.height, '32px')
    assert.equal(pres.style.backgroundColor, 'var(--tonal)')
    assert.equal(pres.style.color, 'var(--on-primary-container)')
  })

  it('disables button in idle state when baseUrl is empty or whitespace', () => {
    const pres = getTestConnectionButtonPresentation({
      isTesting: false,
      testResult: null,
      baseUrl: '   ',
    })

    assert.equal(pres.state, 'idle')
    assert.equal(pres.disabled, true)
  })

  it('returns testing presentation with spinner and disabled state while request is in flight', () => {
    const prevResult: TestResult = {
      success: true,
      status_code: 200,
      latency_ms: 420,
      message: 'OK',
      endpoint: 'https://api.openai.com/v1',
    }
    const pres = getTestConnectionButtonPresentation({
      isTesting: true,
      testResult: prevResult,
      baseUrl: 'https://api.openai.com/v1',
    })

    assert.equal(pres.state, 'testing')
    assert.equal(pres.icon, 'spinner')
    assert.equal(pres.spinIcon, true)
    assert.equal(pres.label, 'Testing...')
    assert.equal(pres.disabled, true)
    assert.equal(pres.style.width, '160px')
    assert.equal(pres.style.minWidth, '160px')
    assert.equal(pres.style.maxWidth, '160px')
    assert.equal(pres.style.height, '32px')
  })

  it('returns success presentation with inline status, latency, green styling, and remains clickable for re-testing', () => {
    const result: TestResult = {
      success: true,
      status_code: 200,
      latency_ms: 1876,
      message: 'OK',
      endpoint: 'https://api.openai.com/v1',
      quota_result: {
        quota_type: 'balance',
        balance_value: '$14.20',
        fraction: 0.71,
        has_percentage: true,
      },
    }
    const pres = getTestConnectionButtonPresentation({
      isTesting: false,
      testResult: result,
      baseUrl: 'https://api.openai.com/v1',
    })

    assert.equal(pres.state, 'success')
    assert.equal(pres.icon, 'check')
    assert.equal(pres.spinIcon, false)
    assert.equal(pres.label, '200 OK (1876ms)')
    assert.equal(pres.disabled, false)
    assert.equal(pres.style.backgroundColor, 'var(--green-bg)')
    assert.equal(pres.style.color, 'var(--green)')
    assert.equal(pres.style.width, '160px')
    assert.equal(pres.style.minWidth, '160px')
    assert.equal(pres.style.maxWidth, '160px')
    assert.equal(pres.style.height, '32px')
    assert.ok(pres.tooltip.includes('200 OK (1876ms)'))
    assert.ok(pres.tooltip.includes('Balance: $14.20'))
    assert.ok(pres.tooltip.toLowerCase().includes('re-test'))
  })

  it('returns error presentation with status code or Test Failed, red styling, and remains clickable for re-testing', () => {
    const httpError: TestResult = {
      success: false,
      status_code: 401,
      latency_ms: 120,
      message: 'Unauthorized API key',
      endpoint: 'https://api.openai.com/v1',
    }
    const presHttp = getTestConnectionButtonPresentation({
      isTesting: false,
      testResult: httpError,
      baseUrl: 'https://api.openai.com/v1',
    })

    assert.equal(presHttp.state, 'error')
    assert.equal(presHttp.icon, 'alert')
    assert.equal(presHttp.spinIcon, false)
    assert.equal(presHttp.label, '401 Failed')
    assert.equal(presHttp.disabled, false)
    assert.equal(presHttp.style.backgroundColor, 'var(--red-bg)')
    assert.equal(presHttp.style.color, 'var(--red)')
    assert.equal(presHttp.style.width, '160px')
    assert.equal(presHttp.style.minWidth, '160px')
    assert.equal(presHttp.style.maxWidth, '160px')
    assert.equal(presHttp.style.height, '32px')
    assert.ok(presHttp.tooltip.includes('Unauthorized API key'))

    const networkError: TestResult = {
      success: false,
      status_code: 0,
      latency_ms: 0,
      message: 'Connection refused',
      endpoint: 'https://api.openai.com/v1',
    }
    const presNet = getTestConnectionButtonPresentation({
      isTesting: false,
      testResult: networkError,
      baseUrl: 'https://api.openai.com/v1',
    })

    assert.equal(presNet.state, 'error')
    assert.equal(presNet.label, 'Test Failed')
    assert.equal(presNet.disabled, false)
    assert.ok(presNet.tooltip.includes('Connection refused'))
  })
})
