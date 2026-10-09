export type ModelSourceKey = 'gemini' | 'custom' | 'non_gemini' | 'credits'

export const MODEL_SOURCE_LABELS: Record<ModelSourceKey, string> = {
  gemini: 'Gemini Native Models',
  custom: 'Custom Models',
  non_gemini: 'Non-Gemini Native Models',
  credits: 'AI Credits',
}

const MODEL_SOURCE_ORDER: readonly ModelSourceKey[] = ['gemini', 'custom', 'non_gemini', 'credits']

const LEGACY_KEY_MAP: Record<string, ModelSourceKey> = {
  custom_model: 'custom',
  ai_credits: 'credits',
}

export function normalizeModelSourceHierarchy(
  raw: readonly string[] | null | undefined
): ModelSourceKey[] {
  const result: ModelSourceKey[] = []
  for (const item of raw ?? []) {
    const key = (MODEL_SOURCE_ORDER as readonly string[]).includes(item)
      ? (item as ModelSourceKey)
      : LEGACY_KEY_MAP[item]
    if (key && !result.includes(key)) {
      result.push(key)
    }
  }
  for (const key of MODEL_SOURCE_ORDER) {
    if (!result.includes(key)) {
      result.push(key)
    }
  }
  return result
}
