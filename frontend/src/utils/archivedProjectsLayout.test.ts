import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Archived Projects Gadget Top Bar Removal & Layout Invariants (Milestone 10)', () => {
  const pagePath = path.resolve(import.meta.dirname, '../pages/ArchivedProjectsPage.tsx')
  const pageSrc = fs.readFileSync(pagePath, 'utf8')

  describe('1. Card Header Top Bar Excision', () => {
    it('does not contain the card header top bar with uppercase Archived Projects title', () => {
      // The old top bar combined 16px 20px padding with an uppercase title label
      assert.equal(
        pageSrc.includes('letterSpacing: \'0.8px\''),
        false,
        'Card header letterSpacing: 0.8px must be excised'
      )
      assert.equal(
        pageSrc.includes('textTransform: \'uppercase\',\n            }}\n          >\n            Archived Projects'),
        false,
        'Redundant uppercase "Archived Projects" header text in card header must be removed'
      )
    })

    it('does not contain the active project archive select dropdown', () => {
      assert.equal(
        pageSrc.includes('+ Archive an active project...'),
        false,
        'Active project archive dropdown must be completely excised'
      )
      assert.equal(
        pageSrc.includes('ChevronDown'),
        false,
        'ChevronDown icon import and usage must be removed'
      )
      assert.equal(
        pageSrc.includes('selectedToArchive'),
        false,
        'selectedToArchive state must be removed'
      )
      assert.equal(
        pageSrc.includes('handleArchiveSelect'),
        false,
        'handleArchiveSelect handler callback must be removed'
      )
      assert.equal(
        pageSrc.includes('activeProjects'),
        false,
        'activeProjects state must be removed'
      )
    })

    it('does not fetch getGUIProjects in loadData', () => {
      assert.equal(
        pageSrc.includes('getGUIProjects'),
        false,
        'api.getGUIProjects() call in loadData must be removed'
      )
    })

    it('does not contain the standalone Refresh button from the excised header', () => {
      assert.equal(
        pageSrc.includes('title="Refresh archived projects list"'),
        false,
        'Excised top-bar refresh button must not be present'
      )
    })
  })

  describe('2. Promoted Search Filter Row & 4px-Grid Compliance', () => {
    it('promotes the search filter row as the direct top element inside google-card', () => {
      const cardIdx = pageSrc.indexOf('className="google-card"')
      assert.ok(cardIdx !== -1, 'google-card container must exist')

      const searchFilterIdx = pageSrc.indexOf('placeholder="Filter archived projects by name or path..."')
      assert.ok(searchFilterIdx !== -1, 'Search filter input must exist')
      assert.ok(searchFilterIdx > cardIdx, 'Search filter must be inside google-card')

      // Ensure no intermediate header element exists between google-card and search filter
      const intermediateSnippet = pageSrc.slice(cardIdx, searchFilterIdx)
      assert.equal(
        intermediateSnippet.includes('Archived Projects'),
        false,
        'No card header title between google-card start and search filter'
      )
      assert.equal(
        intermediateSnippet.includes('Archive an active project'),
        false,
        'No archive dropdown between google-card start and search filter'
      )
    })

    it('strictly complies with the 4-pixel grid for search container and input tokens', () => {
      // Container padding must be exactly 16px 20px with 1px borderBottom
      assert.ok(
        pageSrc.includes("padding: '16px 20px'"),
        'Search container must use 16px 20px padding (divisible by 4)'
      )
      assert.ok(
        pageSrc.includes("borderBottom: '1px solid var(--border)'"),
        'Search container must have 1px solid var(--border) divider'
      )

      // Must NOT contain old non-4px grid values
      assert.equal(
        pageSrc.includes("padding: '14px 20px'"),
        false,
        'Non-4px grid padding 14px 20px must be removed'
      )
      assert.equal(
        pageSrc.includes("height: '38px'"),
        false,
        'Non-4px grid height 38px must be replaced with 36px'
      )

      // Search input width, height, and border tokens
      assert.ok(
        pageSrc.includes("maxWidth: '384px'"),
        'Search container must have maxWidth: 384px (divisible by 4)'
      )
      assert.ok(
        pageSrc.includes("height: '36px'"),
        'Search input must have height: 36px (divisible by 4)'
      )
      assert.ok(
        pageSrc.includes("padding: '0 12px 0 36px'"),
        'Search input padding must be 0 12px 0 36px (all divisible by 4)'
      )
      assert.ok(
        pageSrc.includes("borderRadius: '8px'"),
        'Search input borderRadius must be 8px (divisible by 4)'
      )
      assert.ok(
        pageSrc.includes("border: '1px solid var(--border)'"),
        'Search input border must use tokenized var(--border)'
      )
      assert.ok(
        pageSrc.includes("background: 'var(--surface, #ffffff)'"),
        'Search input background must use tokenized surface'
      )
      assert.ok(
        pageSrc.includes("color: 'var(--text)'"),
        'Search input color must use tokenized text'
      )
    })

    it('positions the search icon strictly aligned to 4px grid', () => {
      assert.ok(
        pageSrc.includes("size={16}"),
        'Search icon size must be 16px (divisible by 4)'
      )
      assert.ok(
        pageSrc.includes("color=\"var(--text-muted)\""),
        'Search icon color must use var(--text-muted)'
      )
      assert.ok(
        pageSrc.includes("top: '10px'"),
        'Search icon top position must be 10px'
      )
      assert.ok(
        pageSrc.includes("left: '12px'"),
        'Search icon left position must be 12px (divisible by 4)'
      )
    })
  })

  describe('3. David-Design Standards: Single-Line Action Text & Button Geometry', () => {
    it('enforces whiteSpace: nowrap on all table headers', () => {
      const theadMatch = pageSrc.match(/<thead>([\s\S]*?)<\/thead>/)
      assert.ok(theadMatch, 'Table header thead element must exist')
      const theadContent = theadMatch[1]

      const headers = ['Project Name', 'Last Conversation', 'Conversations', 'Actions']
      for (const header of headers) {
        assert.ok(theadContent.includes(header), `Header "${header}" must exist`)
      }

      // Count occurrences of whiteSpace: 'nowrap' in thead
      const nowrapMatches = theadContent.match(/whiteSpace:\s*'nowrap'/g)
      assert.ok(nowrapMatches && nowrapMatches.length >= 4, 'All 4 table headers must declare whiteSpace: nowrap')
    })

    it('enforces whiteSpace: nowrap on action buttons (Settings, Restore, Delete)', () => {
      const actionsCellMatch = pageSrc.match(/{\/\* Actions \*\/}[\s\S]*?<\/td>/)
      assert.ok(actionsCellMatch, 'Actions table cell must exist')
      const actionsCell = actionsCellMatch[0]

      assert.ok(actionsCell.includes('Settings'), 'Settings action button must exist')
      assert.ok(actionsCell.includes('Restore'), 'Restore action button must exist')
      assert.ok(actionsCell.includes('Delete'), 'Delete action button must exist')

      const nowrapInActions = actionsCell.match(/whiteSpace:\s*'nowrap'/g)
      assert.ok(
        nowrapInActions && nowrapInActions.length >= 3,
        'Settings, Restore, and Delete action buttons must declare whiteSpace: nowrap'
      )
    })

    it('enforces 4px grid padding and whiteSpace: nowrap on relative active timestamp and conversation badge', () => {
      // Relative active timestamp
      const relativeTimeCell = pageSrc.match(/{\/\* Relative Last Active Time[\s\S]*?<\/td>/)
      assert.ok(relativeTimeCell, 'Relative last active time cell must exist')
      assert.ok(
        relativeTimeCell[0].includes("whiteSpace: 'nowrap'"),
        'Relative time span must declare whiteSpace: nowrap'
      )

      // Conversation count badge
      const convBadgeCell = pageSrc.match(/{\/\* Conversation Count \*\/}[\s\S]*?<\/td>/)
      assert.ok(convBadgeCell, 'Conversation count cell must exist')
      assert.ok(
        convBadgeCell[0].includes("padding: '2px 8px'"),
        'Conversation count badge must use 4px-grid compliant padding: 2px 8px'
      )
      assert.ok(
        convBadgeCell[0].includes("whiteSpace: 'nowrap'"),
        'Conversation count badge must declare whiteSpace: nowrap'
      )
      assert.equal(
        convBadgeCell[0].includes("padding: '3px 9px'"),
        false,
        'Old non-4px grid padding: 3px 9px must be removed'
      )
    })

    it('verifies zero decorative emojis across the entire page file', () => {
      const emojiRegex = /[\u{1F300}-\u{1F9FF}]/gu
      const matches = pageSrc.match(emojiRegex) || []
      assert.equal(matches.length, 0, `Zero decorative emojis permitted; found: ${matches.join(', ')}`)
    })
  })

  describe('4. Functional Continuity & Essential UI State Resilience', () => {
    it('preserves loading state with spinner', () => {
      assert.ok(pageSrc.includes('Loading archived projects...'), 'Loading message preserved')
      assert.ok(pageSrc.includes('<RefreshCw size={24} className="spin"'), 'RefreshCw spinner preserved')
    })

    it('preserves empty state message and guidance', () => {
      assert.ok(pageSrc.includes('No matching archived projects'), 'Search empty state preserved')
      assert.ok(pageSrc.includes('No archived projects'), 'General empty state preserved')
      assert.ok(pageSrc.includes('To archive a project and remove it from your sidebar'), 'Helpful guidance preserved')
    })

    it('preserves permanent delete confirmation modal and handlers', () => {
      assert.ok(pageSrc.includes('Confirm Project Deletion'), 'Delete modal header preserved')
      assert.ok(pageSrc.includes('handleDelete'), 'handleDelete handler preserved')
      assert.ok(pageSrc.includes('handleRestore'), 'handleRestore handler preserved')
      assert.ok(pageSrc.includes('handleOpenSettings'), 'handleOpenSettings handler preserved')
    })
  })
})
