import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

describe('Milestone 11: Cache Auto-Pruner Defaults & Styling', () => {
  const apiPath = path.resolve(__dirname, '../api.ts')
  const pagePath = path.resolve(__dirname, '../pages/BrainCachePage.tsx')

  const apiSource = fs.readFileSync(apiPath, 'utf8')
  const pageSource = fs.readFileSync(pagePath, 'utf8')

  describe('1. Frontend API Signature Defaults (api.ts)', () => {
    it('sets default arguments to 0 (Unlimited) for scanCache', () => {
      assert.ok(
        apiSource.includes('scanCache: (days = 0, maxSizeGB = 0)'),
        'api.scanCache must default to days = 0, maxSizeGB = 0'
      )
    })

    it('sets default arguments to 0 (Unlimited) for pruneCache', () => {
      assert.ok(
        apiSource.includes('pruneCache: (days = 0, maxSizeGB = 0)'),
        'api.pruneCache must default to days = 0, maxSizeGB = 0'
      )
    })
  })

  describe('2. BrainCachePage State Initialization & Invariants', () => {
    it('initializes pruneDays state hook to 0 (Unlimited)', () => {
      assert.ok(
        pageSource.includes('const [pruneDays, setPruneDays] = useState<number>(0)'),
        'pruneDays must initialize to 0'
      )
    })

    it('initializes maxSizeGB state hook to 0 (Unlimited)', () => {
      assert.ok(
        pageSource.includes('const [maxSizeGB, setMaxSizeGB] = useState<number>(0)'),
        'maxSizeGB must initialize to 0'
      )
    })

    it('config loader uses 0 as fallback and accepts max_size_gb >= 0', () => {
      assert.ok(
        pageSource.includes('let initialDays = 0'),
        'initialDays fallback must be 0'
      )
      assert.ok(
        pageSource.includes('let initialSize = 0'),
        'initialSize fallback must be 0'
      )
      assert.ok(
        pageSource.includes('cfg.max_size_gb >= 0'),
        'config loader must accept cfg.max_size_gb >= 0 without clamping 0 to 5'
      )
    })

    it('handleMaxSizeGBChange accepts 0 and safe guards non-negative numbers', () => {
      assert.ok(
        pageSource.includes('val >= 0 ? val : 0'),
        'handleMaxSizeGBChange must accept val >= 0 and normalize to 0'
      )
      assert.ok(
        pageSource.includes('saveCacheConfig({ max_size_gb: safeVal })'),
        'handleMaxSizeGBChange must save safeVal'
      )
    })
  })

  describe('3. Size Limit Input: min={0} and Greyed out "0"', () => {
    it('constrains size limit input with min={0} allowing Unlimited 0', () => {
      assert.ok(
        pageSource.includes('min={0}'),
        'Size limit input must have min={0}'
      )
      assert.ok(
        !pageSource.includes('min={0.5}'),
        'Size limit input must not restrict min to 0.5'
      )
    })

    it('applies conditional styling to grey out the "0" when size is 0', () => {
      assert.ok(
        pageSource.includes("color: maxSizeGB === 0 ? 'var(--text-subtle)' : 'var(--text)'"),
        'Size limit input must use var(--text-subtle) when maxSizeGB === 0'
      )
      assert.ok(
        pageSource.includes('fontWeight: maxSizeGB === 0 ? 500 : 600'),
        'Size limit input must use lighter font weight 500 when maxSizeGB === 0'
      )
    })

    it('renders inline (Unlimited) indicator when maxSizeGB is 0', () => {
      assert.ok(
        pageSource.includes('{maxSizeGB === 0 && ('),
        'Inline indicator conditional must be present'
      )
      assert.ok(
        pageSource.includes('(Unlimited)'),
        'Inline indicator must display (Unlimited)'
      )
      assert.ok(
        pageSource.includes("color: 'var(--text-subtle)'"),
        'Inline indicator must be styled with var(--text-subtle)'
      )
    })

    it('sets accessible title attribute reflecting 0 (Unlimited)', () => {
      assert.ok(
        pageSource.includes("title={maxSizeGB === 0 ? '0 (Unlimited)' : `${maxSizeGB} GB`}"),
        'Size limit input must have informative title tooltip'
      )
    })
  })

  describe('4. Confirmation Modal Messaging for Unlimited Retentions', () => {
    it('adjusts confirmation dialog copy when both retention and size are Unlimited', () => {
      assert.ok(
        pageSource.includes('pruneDays === 0 && maxSizeGB === 0'),
        'Confirmation modal must handle case where both days and size are 0'
      )
      assert.ok(
        pageSource.includes('Both age retention and size limit are set to <strong>Unlimited</strong>'),
        'Confirmation modal must clearly state both age retention and size limit are Unlimited'
      )
    })
  })

  describe('5. David-Design Compliance & Zero Decorative Emojis', () => {
    it('strictly contains zero decorative emojis', () => {
      const emojiRegex = /[\p{Extended_Pictographic}]/gu
      const matches = pageSource.match(emojiRegex)
      assert.deepEqual(matches, null, `Found forbidden decorative emojis: ${matches?.join(', ')}`)
    })

    it('enforces single-line action text (whiteSpace: nowrap) on buttons and controls', () => {
      assert.ok(
        pageSource.includes("whiteSpace: 'nowrap'"),
        'BrainCachePage must enforce whiteSpace: nowrap for David-Design single-line action items'
      )
    })

    it('uses exclusive monochrome Lucide icons', () => {
      assert.ok(pageSource.includes("from 'lucide-react'"))
      assert.ok(!pageSource.includes('@fortawesome'))
      assert.ok(!pageSource.includes('react-icons'))
    })
  })
})
