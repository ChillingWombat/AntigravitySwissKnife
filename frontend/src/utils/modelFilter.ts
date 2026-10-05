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
