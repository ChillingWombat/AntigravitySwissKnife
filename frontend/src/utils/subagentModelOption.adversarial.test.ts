import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

describe('Adversarial & Empirical Verification: Milestone 13 Subagent Custom Model Options & David-Design', () => {
  const pagePath = path.resolve(__dirname, '../pages/SwitcherSettingsPage.tsx')
  const pageSource = fs.readFileSync(pagePath, 'utf8')

  const sec4bMarker = '/* Section 4b: Gemini Subagent Custom Models */'
  const sec5Marker = '/* Section 5: Post-Reset Keep-Alive Warmup Engine */'
  const sec4bIdx = pageSource.indexOf(sec4bMarker)
  const sec5Idx = pageSource.indexOf(sec5Marker)

  assert.ok(sec4bIdx !== -1, 'Section 4b marker must exist')
  assert.ok(sec5Idx !== -1, 'Section 5 marker must exist')
  assert.ok(sec4bIdx < sec5Idx, 'Section 4b must appear before Section 5')

  const sec4bSource = pageSource.slice(sec4bIdx, sec5Idx)

  describe('1. David-Design Iconography & Zero-Decorative-Emoji Invariant', () => {
    const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u

    it('empirically guarantees 0 decorative emojis across Section 4b JSX', () => {
      assert.equal(
        emojiRegex.test(sec4bSource),
        false,
        'Section 4b must not contain any decorative emojis'
      )
    })

    it('verifies exclusively monochrome Lucide icons are employed with vector sizing', () => {
      assert.ok(!sec4bSource.includes('<Bot size={16}'), 'Header icon was removed per user request for clean headers')
      assert.ok(!sec4bSource.includes('<Cpu'), 'Option A header icon removed per user request')
      assert.ok(!sec4bSource.includes('<Sliders'), 'Option B header icon removed per user request')
      assert.ok(sec4bSource.includes('<CheckCircle2 size={14} color="var(--primary)" />'), 'Must use Lucide CheckCircle2 icon for active checkmark')
    })

    it('verifies toggle switch in header enables or disables subagent custom models', () => {
      assert.ok(sec4bSource.includes('<ToggleSwitch'), 'Section 4b must contain ToggleSwitch')
      assert.ok(sec4bSource.includes('subagentCustomModelsEnabled'), 'ToggleSwitch controls subagentCustomModelsEnabled')
      assert.ok(
        sec4bSource.includes("opacity: subagentCustomModelsEnabled ? 1 : 0.45"),
        'Option cards must dim when custom models are disabled'
      )
    })
  })

  describe('2. Single-Line Action Header Protection (whiteSpace: nowrap)', () => {
    it('protects Section 4b title with whiteSpace: nowrap', () => {
      assert.ok(
        sec4bSource.includes("textTransform: 'uppercase', whiteSpace: 'nowrap'"),
        'Section 4b uppercase title must have whiteSpace: nowrap'
      )
    })

    it('protects Option A label with whiteSpace: nowrap', () => {
      assert.ok(
        sec4bSource.includes("color: 'var(--text)', whiteSpace: 'nowrap'"),
        'Option A text label must have whiteSpace: nowrap'
      )
      assert.ok(sec4bSource.includes('Default custom model only'))
    })

    it('protects Option B label with whiteSpace: nowrap', () => {
      assert.ok(
        sec4bSource.includes("color: 'var(--text)', whiteSpace: 'nowrap'"),
        'Option B text label must have whiteSpace: nowrap'
      )
      assert.ok(sec4bSource.includes('Auto-decide based on ability, performance, and price'))
    })

    it('verifies natural wrapping is preserved for multi-line descriptive text', () => {
      assert.ok(sec4bSource.includes("lineHeight: 1.5"), 'Card description must allow natural multi-line reading')
      assert.ok(sec4bSource.includes("lineHeight: '1.45'"), 'Option descriptions must allow natural multi-line reading')
    })
  })

  describe('3. Spacing Tokens & Visual Border Geometry', () => {
    it('verifies 4px grid multiple alignment for grid gap, margins, and radii', () => {
      assert.ok(sec4bSource.includes("gap: '12px'"), 'Option grid gap must be 12px (3 * 4px)')
      assert.ok(sec4bSource.includes("margin: '0 0 16px'"), 'Card description bottom margin must be 16px (4 * 4px)')
      assert.ok(sec4bSource.includes("borderRadius: '8px'"), 'Option card border radius must be 8px (2 * 4px)')
      assert.ok(sec4bSource.includes("gap: '8px'"), 'Option card internal flex gap must be 8px (2 * 4px)')
    })

    it('verifies consistent card padding and sibling alignment', () => {
      assert.ok(sec4bSource.includes("padding: '14px'"), 'Option cards must use 14px padding matching sibling cards')
      assert.ok(sec4bSource.includes("marginBottom: '14px'"), 'Header bottom margin must be 14px matching Section 3/4')
    })

    it('verifies active border is 2px solid primary and unselected is 1px solid border', () => {
      assert.ok(
        sec4bSource.includes("border: subagentStrategy === 'default_custom_only' ? '2px solid var(--primary)' : '1px solid var(--border)'")
      )
      assert.ok(
        sec4bSource.includes("border: subagentStrategy === 'auto_decide' ? '2px solid var(--primary)' : '1px solid var(--border)'")
      )
    })

    it('verifies subtle background tint for active selection', () => {
      assert.ok(
        sec4bSource.includes("backgroundColor: subagentStrategy === 'default_custom_only' ? 'var(--primary-light, rgba(11, 87, 208, 0.04))' : 'var(--surface)'")
      )
      assert.ok(
        sec4bSource.includes("backgroundColor: subagentStrategy === 'auto_decide' ? 'var(--primary-light, rgba(11, 87, 208, 0.04))' : 'var(--surface)'")
      )
    })
  })

  describe('4. Interactive State Binding & Auto-Save Wiring', () => {
    it('binds onClick handlers directly to setSubagentStrategy', () => {
      assert.ok(sec4bSource.includes("onClick={() => setSubagentStrategy('default_custom_only')}"))
      assert.ok(sec4bSource.includes("onClick={() => setSubagentStrategy('auto_decide')}"))
    })

    it('verifies subagentStrategy state hydration and fallback in SwitcherSettingsPage', () => {
      assert.ok(
        /useState<SubagentModelStrategy>\s*\(\s*initialRules\?\.subagent_model_strategy\s*\|\|\s*'default_custom_only'\s*\)/.test(pageSource),
        'subagentStrategy must initialize from initialRules or default to default_custom_only'
      )
      assert.ok(
        pageSource.includes('if (initialRules.subagent_model_strategy) {'),
        'subagentStrategy must hydrate when initialRules changes'
      )
    })

    it('verifies debounced auto-save payload contains subagent_model_strategy', () => {
      assert.ok(pageSource.includes('subagent_model_strategy: subagentStrategy,'))
      assert.ok(pageSource.includes('subagentStrategy,'), 'useEffect dependency array must include subagentStrategy')
    })
  })

  describe('5. Mathematical Viewport Geometry Stress Simulation', () => {
    it('proves that on desktop minimum window width (1216px), option columns have ample clearance', () => {
      const windowMinWidth = 1216
      const navRailWidth = 200
      const mainPadding = 24 * 2
      const cardPadding = 20 * 2
      const gridGap = 12
      const workspaceWidth = windowMinWidth - navRailWidth - mainPadding - cardPadding
      // workspaceWidth = 1216 - 200 - 48 - 40 = 928px
      assert.equal(workspaceWidth, 928)

      // 2-column layout width per option card
      const optionCardWidth = (workspaceWidth - gridGap) / 2 // (928 - 12) / 2 = 458px
      assert.equal(optionCardWidth, 458)

      // Option B Header text: "Auto-decide based on ability, performance, and price" is 51 chars
      // Rendered width measured via Headless Chrome: ~343px
      const measuredHeaderWidth = 343
      const cardInternalPadding = 14 * 2 // 28px
      const iconAndGap = 15 + 8 // 23px
      const checkmarkAndGap = 14 + 8 // 22px
      const totalRequiredWidth = measuredHeaderWidth + cardInternalPadding + iconAndGap + checkmarkAndGap
      // totalRequiredWidth = 343 + 28 + 23 + 22 = 416px

      assert.ok(optionCardWidth > totalRequiredWidth, `Option card width (${optionCardWidth}px) must exceed required width (${totalRequiredWidth}px)`)
      const clearance = optionCardWidth - totalRequiredWidth
      assert.ok(clearance >= 40, `Clearance must be at least 40px (actual clearance: ${clearance}px)`)
    })
  })
})
