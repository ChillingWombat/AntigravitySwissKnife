import type { CustomModel, ModelInfo } from '../types'
import { extractCleanModelId } from './modelExtraction.ts'

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

/**
 * Filters fetched models by user input query.
 * Matches case-insensitively against raw ID, cleaned ID, display name, and description.
 * Supports whitespace-separated multi-token search queries.
 *
 * @param models List of fetched ModelInfo objects
 * @param query Search query from user input
 * @returns Filtered list of ModelInfo objects
 */
export function filterFetchedModels(
  models: ModelInfo[] = [],
  query: string = ''
): ModelInfo[] {
  if (!models || models.length === 0) return []
  const trimmed = (query || '').trim().toLowerCase()
  if (!trimmed) return models

  const tokens = trimmed.split(/\s+/).filter(Boolean)
  if (tokens.length === 0) return models

  return models.filter((m) => {
    if (!m) return false
    const rawId = (m.id || '').toLowerCase()
    const cleanId = extractCleanModelId(m.id || '').toLowerCase()
    const displayName = (m.display_name || '').toLowerCase()
    const description = (m.description || '').toLowerCase()

    const combined = `${rawId} ${cleanId} ${displayName} ${description}`

    return tokens.every((token) => combined.includes(token))
  })
}


