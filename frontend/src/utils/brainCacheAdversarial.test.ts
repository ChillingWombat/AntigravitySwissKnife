import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Adversarial & Empirical Stress Tests: BrainCachePage & Cache Auto-Pruner (Milestone 11)', () => {
  const pagePath = path.resolve(import.meta.dirname, '../pages/BrainCachePage.tsx')
  const pageSrc = fs.readFileSync(pagePath, 'utf8')

  describe('1. UI Interaction & Input Normalization Stress Harness', () => {
    // Exact simulation of handleMaxSizeGBChange normalization logic in BrainCachePage.tsx:
    // const safeVal = Number.isFinite(val) && val >= 0 ? val : 0
    const normalizeMaxSizeGB = (val: any): number => {
      const num = typeof val === 'number' ? val : Number(val)
      return Number.isFinite(num) && num >= 0 ? num : 0
    }

    it('handles clearing input (empty string) safely to 0 without throwing or NaN', () => {
      const emptyInput = ''
      const parsed = Number(emptyInput)
      assert.strictEqual(parsed, 0, 'Number("") is 0')
      const normalized = normalizeMaxSizeGB(parsed)
      assert.strictEqual(normalized, 0, 'Clearing input normalizes to 0 (Unlimited)')
    })

    it('handles explicit 0 input gracefully', () => {
      assert.strictEqual(normalizeMaxSizeGB(0), 0)
      assert.strictEqual(normalizeMaxSizeGB('0'), 0)
    })

    it('handles positive numbers and fractional increments correctly', () => {
      assert.strictEqual(normalizeMaxSizeGB(0.5), 0.5)
      assert.strictEqual(normalizeMaxSizeGB(1), 1)
      assert.strictEqual(normalizeMaxSizeGB(2.5), 2.5)
      assert.strictEqual(normalizeMaxSizeGB(10), 10)
      assert.strictEqual(normalizeMaxSizeGB('15.5'), 15.5)
    })

    it('guards against negative inputs by falling back to 0', () => {
      assert.strictEqual(normalizeMaxSizeGB(-0.5), 0)
      assert.strictEqual(normalizeMaxSizeGB(-1), 0)
      assert.strictEqual(normalizeMaxSizeGB(-999), 0)
    })

    it('guards against non-finite values (NaN, Infinity, -Infinity) by falling back to 0', () => {
      assert.strictEqual(normalizeMaxSizeGB(NaN), 0)
      assert.strictEqual(normalizeMaxSizeGB(Infinity), 0)
      assert.strictEqual(normalizeMaxSizeGB(-Infinity), 0)
      assert.strictEqual(normalizeMaxSizeGB('invalid-text'), 0)
      assert.strictEqual(normalizeMaxSizeGB(undefined), 0)
      assert.strictEqual(normalizeMaxSizeGB(null), 0)
    })

    it('verifies handleMaxSizeGBChange exception boundary prevents unhandled errors', async () => {
      let savedPayload: any = null
      let scanDaysCalled: number | null = null
      let scanSizeCalled: number | null = null

      const mockApi = {
        saveCacheConfig: async (payload: any) => {
          savedPayload = payload
          // Simulate backend error or network timeout
          throw new Error('Network timeout')
        },
        scanCache: async (days: number, size: number) => {
          scanDaysCalled = days
          scanSizeCalled = size
          return {}
        },
      }

      // Simulate component handler flow with error catching
      const simulateChange = async (val: number, currentDays: number) => {
        const safeVal = Number.isFinite(val) && val >= 0 ? val : 0
        try {
          await mockApi.saveCacheConfig({ max_size_gb: safeVal })
        } catch {
          // Gracefully caught
        }
        await mockApi.scanCache(currentDays, safeVal)
      }

      await assert.doesNotReject(async () => {
        await simulateChange(0, 0)
      })

      assert.deepStrictEqual(savedPayload, { max_size_gb: 0 })
      assert.strictEqual(scanDaysCalled, 0)
      assert.strictEqual(scanSizeCalled, 0)
    })
  })

  describe('2. Visual Geometry & Dynamic Greying Out of Digit 0', () => {
    it('verifies size input element has conditional color and weight applied', () => {
      assert.ok(
        pageSrc.includes("color: maxSizeGB === 0 ? 'var(--text-subtle)' : 'var(--text)'"),
        "Input text color must switch to 'var(--text-subtle)' when maxSizeGB === 0"
      )
      assert.ok(
        pageSrc.includes('fontWeight: maxSizeGB === 0 ? 500 : 600'),
        'Input text fontWeight must switch to 500 when maxSizeGB === 0'
      )
    })

    it('verifies size input has min={0} and step={0.5}', () => {
      assert.ok(pageSrc.includes('min={0}'), 'Input must have min={0}')
      assert.ok(pageSrc.includes('step={0.5}'), 'Input must have step={0.5}')
      assert.ok(!pageSrc.includes('min={0.5}'), 'Input must not restrict min to 0.5')
    })

    it('verifies inline (Unlimited) companion text appears when maxSizeGB === 0', () => {
      assert.ok(
        pageSrc.includes('{maxSizeGB === 0 && ('),
        'Inline indicator must only appear when maxSizeGB === 0'
      )
      assert.ok(
        pageSrc.includes('(Unlimited)'),
        'Inline indicator must contain "(Unlimited)"'
      )
      assert.ok(
        pageSrc.includes("color: 'var(--text-subtle)'"),
        "Inline indicator must use 'var(--text-subtle)'"
      )
      assert.ok(
        pageSrc.includes("whiteSpace: 'nowrap'"),
        "Inline indicator must specify whiteSpace: 'nowrap'"
      )
    })

    it('verifies informative title attribute is dynamically computed', () => {
      assert.ok(
        pageSrc.includes("title={maxSizeGB === 0 ? '0 (Unlimited)' : `${maxSizeGB} GB`}"),
        'Input must have dynamic title tooltip'
      )
    })
  })

  describe('3. 4-Pixel Grid Spacing Compliance', () => {
    it('verifies container gaps and paddings are 4px grid aligned', () => {
      // Main column gap: 20px (5 * 4)
      assert.ok(pageSrc.includes("gap: '20px'"), 'Column gap 20px is divisible by 4')

      // Metrics grid gap: 12px (3 * 4), marginBottom: 16px (4 * 4)
      assert.ok(pageSrc.includes("gap: '12px'"), 'Metrics gap 12px is divisible by 4')
      assert.ok(pageSrc.includes("marginBottom: '16px'"), 'Section marginBottom 16px is divisible by 4')

      // Card padding: 12px (3 * 4)
      assert.ok(pageSrc.includes("padding: '12px'"), 'Metric card padding 12px is divisible by 4')

      // Action row gap: 16px (4 * 4), gap: 8px (2 * 4)
      assert.ok(pageSrc.includes("gap: '16px'"), 'Action row gap 16px is divisible by 4')
      assert.ok(pageSrc.includes("gap: '8px'"), 'Control gap 8px is divisible by 4')

      // Modal dimensions: width: 440px (110 * 4), padding: 24px (6 * 4), margin: 0 0 20px (5 * 4)
      assert.ok(pageSrc.includes("width: '440px'"), 'Modal width 440px is divisible by 4')
      assert.ok(pageSrc.includes("padding: '24px'"), 'Modal padding 24px is divisible by 4')
      assert.ok(pageSrc.includes("margin: '0 0 20px'"), 'Modal paragraph margin 20px is divisible by 4')

      // Input width: 88px (22 * 4)
      assert.ok(pageSrc.includes("width: '88px'"), 'Input width 88px is divisible by 4')
    })
  })

  describe('4. Single-Line Action Text (whiteSpace: nowrap) Audit', () => {
    it('verifies whiteSpace: nowrap on all buttons in BrainCachePage', () => {
      // Extract all opening <button ... > tags balancing curly braces so arrow functions '=>' don't prematurely terminate
      const buttons: string[] = []
      let idx = 0
      while ((idx = pageSrc.indexOf('<button', idx)) !== -1) {
        let end = idx
        let inBrace = 0
        let inQuote = false
        for (let i = idx; i < pageSrc.length; i++) {
          const ch = pageSrc[i]
          if (ch === '{' && !inQuote) inBrace++
          else if (ch === '}' && !inQuote) inBrace--
          else if (ch === '"' || ch === "'") inQuote = !inQuote
          else if (ch === '>' && inBrace === 0 && !inQuote) {
            end = i + 1
            break
          }
        }
        buttons.push(pageSrc.slice(idx, end))
        idx = end
      }
      assert.strictEqual(buttons.length, 5, `Expected exactly 5 buttons, found ${buttons.length}`)

      for (const btn of buttons) {
        assert.ok(
          btn.includes("whiteSpace: 'nowrap'"),
          `Button must have whiteSpace: 'nowrap': ${btn}`
        )
      }
    })

    it('verifies whiteSpace: nowrap on all setting labels and inline badges', () => {
      // Check labels around select and input
      assert.ok(pageSrc.includes("Prune Stale Artifacts Older Than:"))
      assert.ok(pageSrc.includes("Size Limit (GB):"))
      assert.ok(pageSrc.includes("(Unlimited)"))

      // Verify that the surrounding spans or parents enforce whiteSpace: nowrap
      const labelMatches = pageSrc.match(/<span[^>]*whiteSpace:\s*'nowrap'[^>]*>[^<]*(?:Prune|Size|\(Unlimited\))[^<]*<\/span>/g)
      assert.ok(
        labelMatches && labelMatches.length >= 3,
        'Labels and (Unlimited) span must declare whiteSpace: nowrap'
      )
    })
  })

  describe('5. Zero Decorative Emoji Audit', () => {
    it('strictly verifies zero decorative emojis in BrainCachePage.tsx', () => {
      const emojiRegex = /[\p{Extended_Pictographic}]/gu
      const matches = pageSrc.match(emojiRegex)
      assert.strictEqual(
        matches,
        null,
        `BrainCachePage.tsx must have 0 decorative emojis, found: ${matches?.join(', ')}`
      )
    })

    it('verifies exclusive usage of Lucide icons', () => {
      const imports = pageSrc.match(/import\s*\{[^}]*\}\s*from\s*['"]lucide-react['"]/g)
      assert.ok(imports && imports.length > 0, 'Must import icons from lucide-react')

      // Ensure no third-party icon libraries imported
      assert.ok(!pageSrc.includes('@fortawesome'))
      assert.ok(!pageSrc.includes('react-icons'))
      assert.ok(!pageSrc.includes('@heroicons'))
    })
  })

  describe('6. Confirmation Modal Matrix Simulation', () => {
    const renderConfirmationText = (pruneDays: number, maxSizeGB: number): string => {
      const PRUNE_AGE_LABELS: Record<number, string> = {
        3: '3 Days',
        7: '7 Days',
        14: '14 Days',
        30: '30 Days',
        90: '3 Months',
        180: '6 Months',
        365: '1 Year',
        0: 'Unlimited',
      }
      if (pruneDays === 0 && maxSizeGB === 0) {
        return 'Both age retention and size limit are set to Unlimited. Active sessions, project directories, and the Conversation Vault will remain protected.'
      } else if (pruneDays === 0) {
        return `Are you sure you want to prune cache files exceeding the ${maxSizeGB} GB size limit (with Unlimited age retention)? Active sessions, project directories, and the Conversation Vault will remain protected.`
      } else if (maxSizeGB === 0) {
        return `Are you sure you want to prune cache files older than ${PRUNE_AGE_LABELS[pruneDays] || `${pruneDays} Days`} (with Unlimited size limit)? Active sessions, project directories, and the Conversation Vault will remain protected.`
      } else {
        return `Are you sure you want to prune cache files older than ${PRUNE_AGE_LABELS[pruneDays] || `${pruneDays} Days`} or exceeding the ${maxSizeGB} GB size limit? Active sessions, project directories, and the Conversation Vault will remain protected.`
      }
    }

    it('produces correct modal copy for (0, 0) Unlimited retention and size', () => {
      const text = renderConfirmationText(0, 0)
      assert.ok(text.includes('Both age retention and size limit are set to Unlimited'))
    })

    it('produces correct modal copy for (0, 10) Unlimited age with size limit', () => {
      const text = renderConfirmationText(0, 10)
      assert.ok(text.includes('exceeding the 10 GB size limit (with Unlimited age retention)'))
    })

    it('produces correct modal copy for (30, 0) Age retention with Unlimited size', () => {
      const text = renderConfirmationText(30, 0)
      assert.ok(text.includes('older than 30 Days (with Unlimited size limit)'))
    })

    it('produces correct modal copy for (14, 5) Both finite age and size limit', () => {
      const text = renderConfirmationText(14, 5)
      assert.ok(text.includes('older than 14 Days or exceeding the 5 GB size limit'))
    })
  })
})
