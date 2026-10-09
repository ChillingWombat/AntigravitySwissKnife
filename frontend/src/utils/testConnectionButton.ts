import type { TestResult } from '../types.ts'

export const TEST_BUTTON_WIDTH_PX = 160
export const TEST_BUTTON_HEIGHT_PX = 32

export type TestConnectionButtonState = 'idle' | 'testing' | 'success' | 'error'
export type TestConnectionButtonIcon = 'zap' | 'spinner' | 'check' | 'alert'

export interface TestConnectionButtonInput {
  isTesting: boolean
  testResult: TestResult | null
  baseUrl: string
}

export interface TestConnectionButtonPresentation {
  state: TestConnectionButtonState
  label: string
  tooltip: string
  disabled: boolean
  icon: TestConnectionButtonIcon
  spinIcon: boolean
  style: {
    width: string
    minWidth: string
    maxWidth: string
    height: string
    backgroundColor: string
    color: string
    border: string
    fontWeight: number
  }
}

const LOCKED_GEOMETRY = {
  width: `${TEST_BUTTON_WIDTH_PX}px`,
  minWidth: `${TEST_BUTTON_WIDTH_PX}px`,
  maxWidth: `${TEST_BUTTON_WIDTH_PX}px`,
  height: `${TEST_BUTTON_HEIGHT_PX}px`,
} as const

export function sanitizeConnectionErrorMessage(msg?: string): string {
  if (!msg) return 'Connection test failed'
  const cleaned = msg
    .replace(/\bHTTP\s+[1-5]\d\d(?:\s+[A-Za-z]+)?\b/gi, (match) => {
      const parts = match.split(/\s+/)
      return parts.length > 2 ? parts.slice(2).join(' ') : ''
    })
    .replace(/\bwith\s+status(?:\s+code)?\s+[1-5]\d\d\b/gi, '')
    .replace(/\bstatus(?:\s+code)?\s+[1-5]\d\d\b/gi, '')
    .replace(/\b[1-5]\d\d\s+(?:OK|Unauthorized|Forbidden|Not Found|Internal Server Error|Bad Request|Failed)\b/gi, (match) => {
      return match.replace(/^[1-5]\d\d\s+/, '')
    })
    .replace(/\b[1-5]\d\d\b/g, '')
    .replace(/\(\s*\)/g, '')
    .replace(/\s*:\s*(?=:|$)/g, '')
    .replace(/\s{2,}/g, ' ')
    .trim()

  return cleaned || 'Connection test failed'
}

export function getTestConnectionButtonPresentation({
  isTesting,
  testResult,
  baseUrl,
}: TestConnectionButtonInput): TestConnectionButtonPresentation {
  const hasUrl = baseUrl.trim().length > 0

  if (isTesting) {
    return {
      state: 'testing',
      label: 'Testing...',
      tooltip: 'Testing model connection...',
      disabled: true,
      icon: 'spinner',
      spinIcon: true,
      style: {
        ...LOCKED_GEOMETRY,
        backgroundColor: 'var(--tonal)',
        color: 'var(--on-primary-container)',
        border: '1px solid transparent',
        fontWeight: 500,
      },
    }
  }

  if (testResult) {
    const isSuccess =
      testResult.success &&
      testResult.status_code >= 200 &&
      testResult.status_code < 300

    if (isSuccess) {
      const label = `OK (${testResult.latency_ms}ms)`
      const quotaSuffix = testResult.quota_result?.balance_value
        ? ` • Balance: ${testResult.quota_result.balance_value}`
        : testResult.quota_result?.quota_value
          ? ` • Quota: ${testResult.quota_result.quota_value}`
          : testResult.quota_result?.quota_type === 'quota'
            ? ' • Quota'
            : ''

      return {
        state: 'success',
        label,
        tooltip: `${label}${quotaSuffix} (Click to re-test)`,
        disabled: !hasUrl,
        icon: 'check',
        spinIcon: false,
        style: {
          ...LOCKED_GEOMETRY,
          backgroundColor: 'var(--green-bg)',
          color: 'var(--green)',
          border: '1px solid #ceead6',
          fontWeight: 600,
        },
      }
    }

    const label = 'Test Failed'
    const cleanMsg = sanitizeConnectionErrorMessage(testResult.message)

    return {
      state: 'error',
      label,
      tooltip: `Failed: ${cleanMsg} (Click to re-test)`,
      disabled: !hasUrl,
      icon: 'alert',
      spinIcon: false,
      style: {
        ...LOCKED_GEOMETRY,
        backgroundColor: 'var(--red-bg)',
        color: 'var(--red)',
        border: '1px solid #fad2cf',
        fontWeight: 600,
      },
    }
  }

  return {
    state: 'idle',
    label: 'Test Connection',
    tooltip: hasUrl
      ? 'Test connection to model endpoint'
      : 'Enter an Endpoint URL before testing',
    disabled: !hasUrl,
    icon: 'zap',
    spinIcon: false,
    style: {
      ...LOCKED_GEOMETRY,
      backgroundColor: 'var(--tonal)',
      color: 'var(--on-primary-container)',
      border: '1px solid transparent',
      fontWeight: 500,
    },
  }
}
