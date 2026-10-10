/**
 * Custom Quota Endpoint Probing & Presentation Utility
 * Adheres strictly to David-Design (4px grid, single-line action labels, 1px tokenized borders, zero disruptive layout shift).
 */

import type { CustomModel, QuotaResult } from '../types'

export interface CustomQuotaStatus {
  type: 'loading' | 'success' | 'error'
  message: string
  latencyMs?: number
}

export const CUSTOM_QUOTA_LAYOUT_TOKENS = {
  inputFlex: 1,
  buttonHeight: '30px',
  buttonPadding: '0 12px',
  buttonFontSize: '12px',
  buttonBorderRadius: '4px',
  statusMinHeight: '20px',
  statusFontSize: '11px',
  borderWidth: '1px',
  whiteSpace: 'nowrap' as const,
}

export function isCustomQuotaUrlValid(url: string): boolean {
  if (!url || typeof url !== 'string') return false
  const trimmed = url.trim()
  if (trimmed.length < 8) return false
  return trimmed.startsWith('http://') || trimmed.startsWith('https://')
}

export function formatCustomQuotaStatusText(status: CustomQuotaStatus): string {
  if (status.type === 'loading') {
    return status.message || 'Probing endpoint...'
  }
  const latencyStr = status.latencyMs !== undefined ? ` (${status.latencyMs}ms)` : ''
  if (status.type === 'success') {
    return `Success${latencyStr}: ${status.message}`
  }
  return `Failed${latencyStr}: ${status.message}`
}

export function buildDraftModelForQuotaProbe(params: {
  id?: string
  name: string
  displayName?: string
  providerType: CustomModel['provider_type']
  baseUrl: string
  apiKey?: string
  customQuotaEndpoint: string
  quotaType?: CustomModel['quota_type']
  isDefault?: boolean
  enabled?: boolean
}): CustomModel {
  const ep = params.customQuotaEndpoint.trim()
  return {
    id: params.id || 'draft-quota',
    name: params.name.trim() || 'custom-model',
    display_name: params.displayName?.trim() || params.name.trim() || 'Custom Model',
    provider_type: params.providerType || 'custom',
    base_url: params.baseUrl.trim() || ep,
    api_key: params.apiKey?.trim() || '',
    custom_quota_endpoint: ep,
    project_mappings: ['*'],
    quota_type: params.quotaType || 'na',
    prepaid_balance: 0,
    total_budget: 0,
    quota_fraction: null,
    is_default: !!params.isDefault,
    enabled: params.enabled !== undefined ? params.enabled : true,
  }
}

export function extractQuotaProbeFeedback(res: QuotaResult | null | undefined, latencyMs: number): CustomQuotaStatus {
  if (!res) {
    return {
      type: 'error',
      message: 'No response from quota probe.',
      latencyMs,
    }
  }

  const isSuccess =
    res.quota_type !== 'na' ||
    !!res.balance_value ||
    !!res.quota_value ||
    (res.fraction !== null && res.fraction !== undefined)

  if (isSuccess) {
    const summary = res.balance_value
      ? `Balance: ${res.balance_value}`
      : res.quota_value
      ? `Quota: ${res.quota_value}`
      : res.message || 'Usage info discovered.'
    return {
      type: 'success',
      message: summary,
      latencyMs,
    }
  }

  return {
    type: 'error',
    message: res.message || 'No quota or balance metrics discovered at endpoint.',
    latencyMs,
  }
}
