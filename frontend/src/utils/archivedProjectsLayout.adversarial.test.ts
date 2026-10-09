import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Adversarial & Empirical Challenge: Archived Projects Visual Geometry & David-Design Adherence (Milestone 10)', () => {
  const pagePath = path.resolve(import.meta.dirname, '../pages/ArchivedProjectsPage.tsx')
  const testPath = path.resolve(import.meta.dirname, './archivedProjectsLayout.test.ts')
  const pageSrc = fs.readFileSync(pagePath, 'utf8')
  const testSrc = fs.readFileSync(testPath, 'utf8')

  describe('1. Card Header Top Bar Excision & Residual Markup Audit', () => {
    it('verifies complete removal of legacy card header container', () => {
      // The old top bar was a flex container with letterSpacing 0.8px, uppercase title, dropdown, and refresh button
      assert.equal(
        pageSrc.includes("letterSpacing: '0.8px'"),
        false,
        'Legacy header letterSpacing: 0.8px must not exist'
      )
      assert.equal(
        pageSrc.includes('Archived Projects\n          </div>'),
        false,
        'Legacy uppercase Archived Projects title in card header must not exist'
      )
    })

    it('verifies zero residual state or handler leaks from excised top bar', () => {
      // Neither state declarations nor references must remain
      const forbiddenTokens = [
        'activeProjects',
        'selectedToArchive',
        'handleArchiveSelect',
        'getGUIProjects',
        'ChevronDown',
      ]

      for (const token of forbiddenTokens) {
        assert.equal(
          pageSrc.includes(token),
          false,
          `Forbidden legacy token "${token}" must not appear anywhere in ArchivedProjectsPage.tsx`
        )
      }
    })

    it('verifies that google-card has no intermediate header elements before search or content', () => {
      const cardStartIndex = pageSrc.indexOf('className="google-card"')
      assert.ok(cardStartIndex !== -1, 'google-card element must exist')

      // Look at the opening structure of google-card
      const cardHeaderSection = pageSrc.slice(cardStartIndex, cardStartIndex + 300)
      assert.ok(
        cardHeaderSection.includes('{archived.length > 0 && ('),
        'google-card must directly open with conditional search row, not a card header'
      )
      assert.equal(
        cardHeaderSection.includes('Card Header'),
        false,
        'No residual "Card Header" comment or markup inside google-card'
      )
    })
  })

  describe('2. 4px Grid Alignment & Dimensional Token Invariant Audit', () => {
    it('verifies search container and input tokens strictly match David-Design specifications', () => {
      // Search container styling
      const searchContainerMatch = pageSrc.match(/{\/\* Archived Projects Card \*\/}[\s\S]*?{archived\.length > 0 && \(\s*<div\s*style={{([\s\S]*?)}}/)
      assert.ok(searchContainerMatch, 'Search container style block must exist')
      const searchContainerStyle = searchContainerMatch[1]

      assert.ok(
        searchContainerStyle.includes("padding: '16px 20px'"),
        'Search container padding must be 16px 20px'
      )
      assert.ok(
        searchContainerStyle.includes("borderBottom: '1px solid var(--border)'"),
        'Search container borderBottom must be 1px solid var(--border)'
      )

      // Search input wrapper
      assert.ok(
        pageSrc.includes("maxWidth: '384px'"),
        'Search input wrapper maxWidth must be 384px (96 * 4px)'
      )

      // Search input element
      const inputMatch = pageSrc.match(/<input[\s\S]*?style={{([\s\S]*?)}}/)
      assert.ok(inputMatch, 'Search input style block must exist')
      const inputStyle = inputMatch[1]

      assert.ok(inputStyle.includes("height: '36px'"), 'Search input height must be 36px (9 * 4px)')
      assert.ok(inputStyle.includes("padding: '0 12px 0 36px'"), 'Search input padding must be 0 12px 0 36px (all 4px multiples)')
      assert.ok(inputStyle.includes("borderRadius: '8px'"), 'Search input borderRadius must be 8px (2 * 4px)')
      assert.ok(inputStyle.includes("border: '1px solid var(--border)'"), 'Search input border must be 1px solid var(--border)')
    })

    it('verifies search icon sizing and positioning geometry', () => {
      const searchIconMatch = pageSrc.match(/<Search[\s\S]*?style={{([\s\S]*?)}}/)
      assert.ok(searchIconMatch, 'Search icon must exist with style block')
      const searchIconBlock = pageSrc.slice(pageSrc.indexOf('<Search'), pageSrc.indexOf('/>', pageSrc.indexOf('<Search')))

      assert.ok(searchIconBlock.includes('size={16}'), 'Search icon size must be 16 (4 * 4px)')
      assert.ok(searchIconBlock.includes("left: '12px'"), 'Search icon left must be 12px (3 * 4px)')
      assert.ok(searchIconBlock.includes("top: '10px'"), 'Search icon top must be 10px (centered in 36px input: (36-16)/2 = 10px)')
    })

    it('verifies conversation count badge dimensional tokens', () => {
      const badgeMatch = pageSrc.match(/{\/\* Conversation Count \*\/}[\s\S]*?<div\s*style={{([\s\S]*?)}}/)
      assert.ok(badgeMatch, 'Conversation count badge must exist')
      const badgeStyle = badgeMatch[1]

      assert.ok(badgeStyle.includes("padding: '2px 8px'"), 'Badge padding must be 2px 8px (8px is 2 * 4px)')
      assert.ok(badgeStyle.includes("gap: '6px'"), 'Badge gap must be 6px')
      assert.ok(badgeStyle.includes("borderRadius: '12px'"), 'Badge borderRadius must be 12px (3 * 4px)')
    })

    it('verifies that no legacy unaligned tokens remain across the file', () => {
      const legacyTokens = [
        "padding: '14px 20px'",
        "height: '38px'",
        "maxWidth: '380px'",
        "padding: '3px 9px'",
        "top: '11px'",
        'size={15}',
        "gap: '5px'",
      ]

      for (const token of legacyTokens) {
        assert.equal(
          pageSrc.includes(token),
          false,
          `Legacy unaligned token "${token}" must be completely excised`
        )
      }
    })
  })

  describe('3. Single-Line Action Text (whiteSpace: nowrap) Audit', () => {
    it('verifies whiteSpace: nowrap on all table headers in thead', () => {
      const theadMatch = pageSrc.match(/<thead>([\s\S]*?)<\/thead>/)
      assert.ok(theadMatch, 'thead must exist')
      const thead = theadMatch[1]

      // Extract all th elements
      const thMatches = thead.match(/<th[\s\S]*?<\/th>/g) || []
      assert.equal(thMatches.length, 4, 'Must have exactly 4 table headers')

      for (const th of thMatches) {
        assert.ok(
          th.includes("whiteSpace: 'nowrap'"),
          `Table header must enforce whiteSpace: nowrap. Found: ${th}`
        )
      }
    })

    it('verifies whiteSpace: nowrap on all table row action buttons', () => {
      const actionsCellMatch = pageSrc.match(/{\/\* Actions \*\/}[\s\S]*?<\/td>/)
      assert.ok(actionsCellMatch, 'Actions td must exist')
      const actionsCell = actionsCellMatch[0]

      const buttons = actionsCell.match(/<button[\s\S]*?<\/button>/g) || []
      assert.equal(buttons.length, 3, 'Must have exactly 3 action buttons (Settings, Restore, Delete)')

      for (const btn of buttons) {
        assert.ok(
          btn.includes("whiteSpace: 'nowrap'"),
          `Action button must enforce whiteSpace: nowrap. Found: ${btn.slice(0, 150)}`
        )
      }
    })

    it('verifies whiteSpace: nowrap on conversation badge and metadata spans', () => {
      const badgeCellMatch = pageSrc.match(/{\/\* Conversation Count \*\/}[\s\S]*?<\/td>/)
      assert.ok(badgeCellMatch, 'Conversation count td must exist')
      assert.ok(
        badgeCellMatch[0].includes("whiteSpace: 'nowrap'"),
        'Conversation count badge must enforce whiteSpace: nowrap'
      )

      const activeTimeCellMatch = pageSrc.match(/{\/\* Relative Last Active Time[\s\S]*?<\/td>/)
      assert.ok(activeTimeCellMatch, 'Relative active time td must exist')
      assert.ok(
        activeTimeCellMatch[0].includes("whiteSpace: 'nowrap'"),
        'Relative active time span must enforce whiteSpace: nowrap'
      )

      const pathSpanMatch = pageSrc.match(/<Folder size=\{11\} \/>\s*<span[\s\S]*?<\/span>/)
      assert.ok(pathSpanMatch, 'Folder path span must exist')
      assert.ok(
        pathSpanMatch[0].includes("whiteSpace: 'nowrap'"),
        'Folder path span must enforce whiteSpace: nowrap'
      )
    })
  })

  describe('4. Zero Decorative Emojis Comprehensive Audit', () => {
    // Broad emoji ranges covering Emoticons, Dingbats, Transport, Pictographs, Supplemental Symbols, Flags, etc.
    const emojiRegex = /[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}\u{FE00}-\u{FE0F}\u{1F000}-\u{1F02F}\u{1F0A0}-\u{1F0FF}]/gu

    it('verifies zero decorative emojis in ArchivedProjectsPage.tsx', () => {
      const matches = pageSrc.match(emojiRegex) || []
      assert.equal(
        matches.length,
        0,
        `ArchivedProjectsPage.tsx must contain 0 emojis; found: ${JSON.stringify(matches)}`
      )
    })

    it('verifies zero decorative emojis in archivedProjectsLayout.test.ts', () => {
      // Exclude unicode regex escape sequences in the test itself
      const sanitizedTestSrc = testSrc.replace(/\\u\{[0-9A-Fa-f]+\}/g, '')
      const matches = sanitizedTestSrc.match(emojiRegex) || []
      assert.equal(
        matches.length,
        0,
        `archivedProjectsLayout.test.ts must contain 0 emojis; found: ${JSON.stringify(matches)}`
      )
    })
  })

  describe('5. Adversarial UI Stress & State Simulation', () => {
    it('verifies that long folder URIs are defensively constrained with overflow: hidden, textOverflow: ellipsis, and maxWidth', () => {
      const folderSnippetMatch = pageSrc.match(/<Folder size=\{11\} \/>[\s\S]*?<\/span>/)
      assert.ok(folderSnippetMatch, 'Folder span snippet must exist')
      const snippet = folderSnippetMatch[0]

      assert.ok(snippet.includes("overflow: 'hidden'"), 'Path span must have overflow: hidden')
      assert.ok(snippet.includes("textOverflow: 'ellipsis'"), 'Path span must have textOverflow: ellipsis')
      assert.ok(snippet.includes("whiteSpace: 'nowrap'"), 'Path span must have whiteSpace: nowrap')
      assert.ok(snippet.includes("maxWidth: '340px'"), 'Path span must have constrained maxWidth')
    })

    it('verifies that empty and loading states cleanly preserve 4px grid spacing and zero emoji rules', () => {
      // Loading state
      assert.ok(pageSrc.includes("padding: '48px'"), 'Loading container padding is 48px (12 * 4px)')
      assert.ok(pageSrc.includes('RefreshCw'), 'Uses Lucide RefreshCw instead of cartoon emoji')

      // Empty state
      assert.ok(pageSrc.includes("padding: '48px 24px'"), 'Empty state padding is 48px 24px (12 * 4px, 6 * 4px)')
      assert.ok(pageSrc.includes("width: '48px'"), 'Empty state icon circle is 48px (12 * 4px)')
      assert.ok(pageSrc.includes("height: '48px'"), 'Empty state icon circle is 48px (12 * 4px)')
      assert.ok(pageSrc.includes('Archive'), 'Uses Lucide Archive icon instead of 🗄️ or 📦 emoji')
    })
  })
})
