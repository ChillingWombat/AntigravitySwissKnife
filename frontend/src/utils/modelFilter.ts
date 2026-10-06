import type { CustomModel } from '../types'

export type ModelFilterOption = 'all' | 'enabled' | 'disabled'

export interface ModelFilterStats {
  total: number
  enabled: number
  disabled: number
}

/**
 * Computes model count statistics across total, enabled, and disabled states.
 */
export function computeModelFilterStats(models: CustomModel[] = []): ModelFilterStats {
  let enabled = 0
  let disabled = 0
  for (const m of models) {
    if (m.enabled) {
      enabled++
    } else {
      disabled++
    }
  }
  return {
    total: models.length,
    enabled,
    disabled,
  }
}

/**
 * Filters a list of custom models according to the selected filter option.
 *
 * @param models List of custom models
 * @param filter 'all' | 'enabled' | 'disabled'
 * @returns Filtered list of models
 */
export function filterModels(
  models: CustomModel[] = [],
  filter: ModelFilterOption = 'all'
): CustomModel[] {
  if (filter === 'enabled') {
    return models.filter((m) => m.enabled)
  }
  if (filter === 'disabled') {
    return models.filter((m) => !m.enabled)
  }
  return models
}

/**
 * Resolves the effective custom model ID according to the selection rules:
 * - If no models are enabled/available, returns empty string ("").
 * - If a selected default model exists in the available options, retains it.
 * - Otherwise, defaults to the first model in the list without any 'auto' option.
 */
export function resolveEffectiveCustomModel(
  availableModels: Array<{ id: string }>,
  selectedDefault?: string
): string {
  if (!availableModels || availableModels.length === 0) {
    return ''
  }
  const valid = availableModels.filter(
    (m) => m && typeof m.id === 'string' && m.id.trim() !== ''
  )
  if (valid.length === 0) {
    return ''
  }
  const trimmedSelected = (selectedDefault || '').trim()
  if (trimmedSelected) {
    const match = valid.find((m) => m.id.trim() === trimmedSelected)
    if (match) {
      return match.id.trim()
    }
  }
  return valid[0].id.trim()
}

