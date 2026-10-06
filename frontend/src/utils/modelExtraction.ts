/**
 * Model Identifier and Metadata Extraction Utilities
 * Supports multi-provider format parsing (OpenAI, Anthropic, Gemini, DeepSeek, SiliconFlow, Ollama, etc.)
 */

/**
 * Extracts a clean model identifier from a raw string or fetched model label.
 * Strips:
 * - Parenthesized context annotations, e.g. " (1,048,576 ctx)"
 * - Bracketed annotations, e.g. "[Thinking]", "[default]"
 * - Gemini "models/" prefix
 * - Emoji decorations
 *
 * @param raw Raw string or label
 * @returns Clean, trimmed model identifier
 */
export function extractCleanModelId(raw: string): string {
  if (!raw) return ''

  let s = raw.trim()

  // 1. Strip Gemini API "models/" prefix
  if (s.startsWith('models/')) {
    s = s.substring('models/'.length)
  }

  // 2. Remove emojis (e.g. 🧠, ⚡, ✏️)
  s = s.replace(/[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/gu, '')

  // 3. Strip bracketed annotations like [Thinking], [default]
  s = s.replace(/\[[^\]]*\]/g, '')

  // 4. Strip parenthesized annotations like (1,048,576 ctx), (128k context), (free)
  s = s.replace(/\([^)]*\)/g, '')

  // 5. Trim and collapse multiple spaces
  s = s.replace(/\s+/g, ' ').trim()

  return s
}

/**
 * Extracts context window length in tokens from model metadata or label string.
 *
 * @param raw Raw string that might contain context info, e.g. "(1,048,576 ctx)" or "(128k)"
 * @param modelObj Optional model object containing numeric context window fields
 * @returns Number of tokens or null if not detected
 */
export function extractContextWindow(raw: string, modelObj?: any): number | null {
  // 1. Check direct numeric properties on model object
  if (modelObj && typeof modelObj === 'object') {
    const directNum =
      modelObj.context_window ??
      modelObj.inputTokenLimit ??
      modelObj.context_length ??
      modelObj.max_context_tokens
    if (typeof directNum === 'number' && directNum > 0) {
      return directNum
    }
  }

  if (!raw) return null

  // 2. Look for explicit formatted numbers inside parentheses, e.g. (1,048,576 ctx)
  const numCommaMatch = raw.match(/\(\s*([0-9]{1,3}(?:,[0-9]{3})+)\s*(?:ctx|tokens|context)?/i)
  if (numCommaMatch) {
    const parsed = parseInt(numCommaMatch[1].replace(/,/g, ''), 10)
    if (!isNaN(parsed) && parsed > 0) return parsed
  }

  // 3. Look for plain numbers inside parentheses, e.g. (128000 ctx)
  const plainNumMatch = raw.match(/\(\s*([0-9]{4,10})\s*(?:ctx|tokens|context)?/i)
  if (plainNumMatch) {
    const parsed = parseInt(plainNumMatch[1], 10)
    if (!isNaN(parsed) && parsed > 0) return parsed
  }

  // 4. Look for multiplier patterns like (128k), (200k), (32k), (1m), (2M)
  const multiplierMatch = raw.match(/\(\s*([0-9]+(?:\.[0-9]+)?)\s*([km])\s*(?:ctx|tokens|context)?/i)
  if (multiplierMatch) {
    const val = parseFloat(multiplierMatch[1])
    const unit = multiplierMatch[2].toLowerCase()
    if (!isNaN(val) && val > 0) {
      if (unit === 'k') {
        return Math.round(val * 1024)
      } else if (unit === 'm') {
        return Math.round(val * 1024 * 1024)
      }
    }
  }

  return null
}

/**
 * Detects available thinking / reasoning levels for a given model.
 *
 * @param modelId Model identifier string
 * @param modelObj Optional model object containing supports_thinking or thinking_levels
 * @returns Array of thinking levels (e.g. ['low', 'medium', 'high']) or empty array if non-reasoning
 */
export function detectThinkingLevels(modelId: string, modelObj?: any): string[] {
  if (modelObj && typeof modelObj === 'object') {
    if (Array.isArray(modelObj.thinking_levels) && modelObj.thinking_levels.length > 0) {
      return modelObj.thinking_levels.filter((l: string) => l.toLowerCase() !== 'off')
    }
    if (modelObj.supports_thinking) {
      return ['low', 'medium', 'high']
    }
  }

  const lower = (modelId || '').toLowerCase()
  if (
    lower.includes('o1') ||
    lower.includes('o3') ||
    lower.includes('r1') ||
    lower.includes('reasoner') ||
    lower.includes('thinking') ||
    lower.includes('claude-3-7') ||
    lower.includes('claude-opus-4-6') ||
    lower.includes('claude-4-6') ||
    lower.includes('gemini-2.5') ||
    lower.includes('gemini-3.8') ||
    lower.includes('gemini-3.1') ||
    lower.includes('qwq')
  ) {
    return ['low', 'medium', 'high']
  }

  return []
}
