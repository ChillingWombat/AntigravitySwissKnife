import type { ModelPricingRecord } from '../types.ts'

export const GEMINI_SELECTOR_ORDER = [
  'gemini-3.8-flash',
  'gemini-3.7-flash',
  'gemini-3.6-flash',
  'gemini-3.1-pro',
] as const

export interface SortPricingOptions {
  defaultGeminiModel?: string | null
  defaultCustomModel?: string | null
}

export function normalizeModelIdentifier(idOrName?: string | null): string {
  if (!idOrName) return ''
  let s = idOrName.toLowerCase().trim()
  s = s.replace(/[\s_]+/g, '-')
  const suffixes = ['-high', '-medium', '-low', '-off', '-thinking', '-tiered', '-agent']
  for (const suf of suffixes) {
    if (s.endsWith(suf)) {
      s = s.slice(0, -suf.length)
      break
    }
  }
  if (s === 'gemini-pro' || s === 'gemini-3-1-pro') return 'gemini-3.1-pro'
  if (s === 'gemini-3-flash') return 'gemini-3.6-flash'
  return s
}

export function getGeminiSelectorRank(pr: ModelPricingRecord): number {
  const normId = normalizeModelIdentifier(pr.canonical_id || pr.model_id)
  const normName = normalizeModelIdentifier(pr.model_name || pr.name)

  for (let i = 0; i < GEMINI_SELECTOR_ORDER.length; i++) {
    const target = GEMINI_SELECTOR_ORDER[i]
    if (
      normId === target ||
      normId.includes(target) ||
      normName === target ||
      normName.includes(target)
    ) {
      return i
    }
  }

  const rawId = (pr.canonical_id || pr.model_id || '').toLowerCase()
  const rawName = (pr.model_name || pr.name || '').toLowerCase()
  const rawProv = (pr.provider || '').toLowerCase()
  if (
    rawId.includes('gemini') ||
    rawName.includes('gemini') ||
    rawProv.includes('google') ||
    rawProv.includes('gemini')
  ) {
    return 100
  }

  return 200
}

export function isModelDefaultNative(
  pr: ModelPricingRecord,
  defaultGeminiModel?: string | null
): boolean {
  if (pr.classification !== 'native') return false
  if ((pr as any).is_default === true) return true
  if (!defaultGeminiModel || !defaultGeminiModel.trim()) return false

  const target = normalizeModelIdentifier(defaultGeminiModel)
  const normId = normalizeModelIdentifier(pr.canonical_id || pr.model_id)
  const normName = normalizeModelIdentifier(pr.model_name || pr.name)

  return (
    normId === target ||
    normId.includes(target) ||
    target.includes(normId) ||
    normName === target ||
    normName.includes(target)
  )
}

export function isModelDefaultCustom(
  pr: ModelPricingRecord,
  defaultCustomModel?: string | null
): boolean {
  if (pr.classification !== 'custom') return false
  if ((pr as any).is_default === true) return true
  if (!defaultCustomModel || !defaultCustomModel.trim()) return false

  const target = defaultCustomModel.trim().toLowerCase()
  const customId = (pr.custom_model_id || '').toLowerCase()
  const modelId = (pr.model_id || '').toLowerCase()
  const canonicalId = (pr.canonical_id || '').toLowerCase()
  const name = (pr.name || '').toLowerCase()
  const modelName = (pr.model_name || '').toLowerCase()
  const internalId = String(pr.internal_id ?? '')

  return (
    customId === target ||
    modelId === target ||
    canonicalId === target ||
    name === target ||
    modelName === target ||
    internalId === target
  )
}

export function sortPricingModels(
  records: ModelPricingRecord[],
  options?: SortPricingOptions
): ModelPricingRecord[] {
  const defaultGemini = options?.defaultGeminiModel
  const defaultCustom = options?.defaultCustomModel

  return [...records].sort((a, b) => {
    const aIsNative = a.classification === 'native'
    const bIsNative = b.classification === 'native'

    if (aIsNative && !bIsNative) return -1
    if (!aIsNative && bIsNative) return 1

    if (aIsNative && bIsNative) {
      const aDef = isModelDefaultNative(a, defaultGemini)
      const bDef = isModelDefaultNative(b, defaultGemini)
      if (aDef && !bDef) return -1
      if (!aDef && bDef) return 1

      const aRank = getGeminiSelectorRank(a)
      const bRank = getGeminiSelectorRank(b)
      if (aRank !== bRank) return aRank - bRank

      return (a.internal_id || 0) - (b.internal_id || 0)
    }

    const aDef = isModelDefaultCustom(a, defaultCustom)
    const bDef = isModelDefaultCustom(b, defaultCustom)
    if (aDef && !bDef) return -1
    if (!aDef && bDef) return 1

    const aId = a.internal_id ?? 0
    const bId = b.internal_id ?? 0
    if (aId !== bId) return aId - bId

    const aName = a.model_name || a.name || ''
    const bName = b.model_name || b.name || ''
    return aName.localeCompare(bName)
  })
}
