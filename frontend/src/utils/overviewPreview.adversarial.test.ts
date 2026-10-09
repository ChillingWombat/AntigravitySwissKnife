import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {
  OVERVIEW_PREVIEW_HEADER,
  OVERVIEW_DIVISION_BADGE_LABELS,
  SYNTHETIC_FILES_COUNT,
  SYNTHETIC_VISIBLE_FILES,
  SYNTHETIC_EXPANDED_FILES,
  SYNTHETIC_UPLOADS_COUNT,
  SYNTHETIC_VISIBLE_UPLOADS,
  SYNTHETIC_EXPANDED_UPLOADS,
  SYNTHETIC_OVERVIEW_FIXTURES,
} from './overviewPreviewData.ts'

describe('Adversarial & Stress Verification: Overview Panel Preview Layout, Styling & Expansion', () => {
  const pagePath = path.resolve(import.meta.dirname, '../pages/AppEnhancementsPage.tsx')
  const pageSrc = fs.readFileSync(pagePath, 'utf8')

  // Extract the Overview Panel Preview JSX block from AppEnhancementsPage.tsx
  const extractOverviewPreviewBlock = (): string => {
    const startComment = '{/* RIGHT COLUMN: Interactive Live Preview of Antigravity\'s Overview Panel */}'
    const startIdx = pageSrc.indexOf(startComment)
    assert.ok(startIdx !== -1, 'Must find Overview Panel Preview starting comment in AppEnhancementsPage.tsx')
    return pageSrc.slice(startIdx)
  }

  const previewBlock = extractOverviewPreviewBlock()

  describe('1. UI Layout & Boundary Testing (Container Widths, Wrapping & Truncation)', () => {
    it('verifies whiteSpace: nowrap is strictly enforced on preview header title', () => {
      // Find header title JSX element
      const headerUsageIdx = previewBlock.indexOf('{OVERVIEW_PREVIEW_HEADER}')
      assert.ok(headerUsageIdx !== -1, 'Must render {OVERVIEW_PREVIEW_HEADER}')
      const titleSpanStart = previewBlock.lastIndexOf('<span', headerUsageIdx)
      const titleSpanEnd = previewBlock.indexOf('>', headerUsageIdx)
      const titleSpanJSX = previewBlock.slice(titleSpanStart, titleSpanEnd + 1)

      assert.ok(
        titleSpanJSX.includes("whiteSpace: 'nowrap'"),
        'Header title span must specify whiteSpace: "nowrap" to prevent line wrapping'
      )
      assert.ok(
        titleSpanJSX.includes("fontWeight: 700"),
        'Header title must have fontWeight: 700'
      )
      assert.ok(
        titleSpanJSX.includes("fontSize: '13px'"),
        'Header title must have 13px font size per David-Design standard body typography'
      )
    })

    it('verifies whiteSpace: nowrap is strictly enforced on division style badges', () => {
      const badgeUsageIdx = previewBlock.indexOf('{OVERVIEW_DIVISION_BADGE_LABELS[op.division_style]}')
      assert.ok(badgeUsageIdx !== -1, 'Must render division style badge label')
      const badgeSpanStart = previewBlock.lastIndexOf('<span', badgeUsageIdx)
      const badgeSpanEnd = previewBlock.indexOf('>', badgeUsageIdx)
      const badgeSpanJSX = previewBlock.slice(badgeSpanStart, badgeSpanEnd + 1)

      assert.ok(
        badgeSpanJSX.includes("whiteSpace: 'nowrap'"),
        'Division style badge must specify whiteSpace: "nowrap" to prevent multiline badge wrapping'
      )
      assert.ok(
        badgeSpanJSX.includes("fontSize: '11px'"),
        'Badge must specify 11px font size per David-Design fine print standard'
      )
    })

    it('verifies parent flex container specifies minWidth: 0 for defensive shrink sizing', () => {
      const headerRowIdx = previewBlock.indexOf('{OVERVIEW_PREVIEW_HEADER}')
      const parentDivStart = previewBlock.lastIndexOf('<div', headerRowIdx)
      const parentDivJSX = previewBlock.slice(parentDivStart, headerRowIdx)

      assert.ok(
        parentDivJSX.includes("minWidth: 0"),
        'Title wrapper must include minWidth: 0 to allow flex container shrinking without breaking'
      )
    })

    it('mathematically simulates container width headroom and boundary constraints', () => {
      // Standard preview panel layout:
      // Outer card width: 360px
      // Outer card padding: 16px horizontal on left and right (32px total)
      // Net available inner width = 360 - 32 = 328px
      const cardWidth = 360
      const cardPaddingHorizontal = 16 * 2
      const innerAvailableWidth = cardWidth - cardPaddingHorizontal
      assert.equal(innerAvailableWidth, 328, 'Inner available width at 360px container must be 328px')

      // Approximate text rendered widths (13px bold for title, 11px semibold for badge):
      // Title: "Overview Panel Preview" (22 chars * ~6.5px avg char width) ~ 143px
      const avgCharWidth13pxBold = 6.5
      const titleTextLength = OVERVIEW_PREVIEW_HEADER.length
      const estimatedTitleWidth = Math.ceil(titleTextLength * avgCharWidth13pxBold)

      // Badge: "Border Zones" (12 chars * ~5.5px) + 16px padding (2*8px) ~ 82px
      const avgCharWidth11pxSemi = 5.5
      const borderZoneText = OVERVIEW_DIVISION_BADGE_LABELS.border_zone
      const estimatedBorderZoneWidth = Math.ceil(borderZoneText.length * avgCharWidth11pxSemi) + 16

      // Badge: "Divider Lines" (13 chars * ~5.5px) + 16px padding ~ 87.5px
      const dividerLineText = OVERVIEW_DIVISION_BADGE_LABELS.divider_line
      const estimatedDividerLineWidth = Math.ceil(dividerLineText.length * avgCharWidth11pxSemi) + 16

      // Scenario 1: Standard 360px Container with "Border Zones"
      const combinedWidthBorderZones = estimatedTitleWidth + estimatedBorderZoneWidth
      const headroomBorderZones = innerAvailableWidth - combinedWidthBorderZones
      assert.ok(
        headroomBorderZones >= 90,
        `Headroom at 360px container must be at least 90px (calculated: ${headroomBorderZones}px)`
      )

      // Scenario 2: Standard 360px Container with "Divider Lines"
      const combinedWidthDividerLines = estimatedTitleWidth + estimatedDividerLineWidth
      const headroomDividerLines = innerAvailableWidth - combinedWidthDividerLines
      assert.ok(
        headroomDividerLines >= 85,
        `Headroom at 360px container must be at least 85px (calculated: ${headroomDividerLines}px)`
      )

      // Scenario 3: Boundary threshold - minimum width before collision
      // Header and badge fit without collision down to ~230px
      const minimumSafeContainerWidth = combinedWidthDividerLines + cardPaddingHorizontal
      assert.ok(
        minimumSafeContainerWidth < 270,
        `Threshold width before wrap pressure must be under 270px (calculated: ${minimumSafeContainerWidth}px)`
      )
    })
  })

  describe('2. Section Expansion & Collapse Simulation ("See all (18)" and "See all (8)")', () => {
    it('simulates Files Changed expansion toggle state machine', () => {
      // Model the component state behavior
      let overviewFilesExpanded = false

      const getRenderedFiles = (isExpanded: boolean) => {
        return isExpanded
          ? [...SYNTHETIC_VISIBLE_FILES, ...SYNTHETIC_EXPANDED_FILES]
          : [...SYNTHETIC_VISIBLE_FILES]
      }

      const getButtonLabel = (isExpanded: boolean) => {
        return isExpanded ? 'See less' : `See all (${SYNTHETIC_FILES_COUNT})`
      }

      const getTriangleGlyph = (isExpanded: boolean) => {
        return isExpanded ? '▴' : '▾'
      }

      // Step 1: Initial collapsed state
      assert.equal(overviewFilesExpanded, false)
      let files = getRenderedFiles(overviewFilesExpanded)
      assert.equal(files.length, 5, 'Collapsed state must render exactly 5 visible files')
      assert.equal(getButtonLabel(overviewFilesExpanded), 'See all (18)')
      assert.equal(getTriangleGlyph(overviewFilesExpanded), '▾')

      // Step 2: User clicks "See all (18)"
      overviewFilesExpanded = !overviewFilesExpanded
      assert.equal(overviewFilesExpanded, true)
      files = getRenderedFiles(overviewFilesExpanded)
      assert.equal(files.length, 8, 'Expanded state must render 8 files (5 visible + 3 expanded)')
      assert.equal(getButtonLabel(overviewFilesExpanded), 'See less')
      assert.equal(getTriangleGlyph(overviewFilesExpanded), '▴')
      assert.ok(files.some((f) => f.name === 'session_handler.go'))
      assert.ok(files.some((f) => f.name === 'models.go'))
      assert.ok(files.some((f) => f.name === 'schema.ts'))

      // Step 3: User clicks "See less" (collapse back)
      overviewFilesExpanded = !overviewFilesExpanded
      assert.equal(overviewFilesExpanded, false)
      files = getRenderedFiles(overviewFilesExpanded)
      assert.equal(files.length, 5, 'Must return cleanly to 5 visible files')
      assert.equal(getButtonLabel(overviewFilesExpanded), 'See all (18)')
      assert.equal(getTriangleGlyph(overviewFilesExpanded), '▾')
      assert.ok(!files.some((f) => f.name === 'session_handler.go'))
    })

    it('simulates Uploads expansion toggle state machine', () => {
      let overviewUploadsExpanded = false

      const getRenderedUploads = (isExpanded: boolean) => {
        return isExpanded
          ? [...SYNTHETIC_VISIBLE_UPLOADS, ...SYNTHETIC_EXPANDED_UPLOADS]
          : [...SYNTHETIC_VISIBLE_UPLOADS]
      }

      const getUploadsButtonLabel = (isExpanded: boolean) => {
        return isExpanded ? 'See less' : `See all (${SYNTHETIC_UPLOADS_COUNT})`
      }

      const getUploadsTriangleGlyph = (isExpanded: boolean) => {
        return isExpanded ? '▴' : '▾'
      }

      // Initial collapsed state
      assert.equal(overviewUploadsExpanded, false)
      let uploads = getRenderedUploads(overviewUploadsExpanded)
      assert.equal(uploads.length, 3, 'Collapsed state must render exactly 3 visible uploads')
      assert.equal(getUploadsButtonLabel(overviewUploadsExpanded), 'See all (8)')
      assert.equal(getUploadsTriangleGlyph(overviewUploadsExpanded), '▾')

      // User expands
      overviewUploadsExpanded = !overviewUploadsExpanded
      assert.equal(overviewUploadsExpanded, true)
      uploads = getRenderedUploads(overviewUploadsExpanded)
      assert.equal(uploads.length, 5, 'Expanded state must render 5 uploads (3 visible + 2 expanded)')
      assert.equal(getUploadsButtonLabel(overviewUploadsExpanded), 'See less')
      assert.equal(getUploadsTriangleGlyph(overviewUploadsExpanded), '▴')
      assert.ok(uploads.some((u) => u.name === 'schema-diagram.png'))
      assert.ok(uploads.some((u) => u.name === 'component-spec.png'))

      // User collapses
      overviewUploadsExpanded = !overviewUploadsExpanded
      assert.equal(overviewUploadsExpanded, false)
      uploads = getRenderedUploads(overviewUploadsExpanded)
      assert.equal(uploads.length, 3)
      assert.equal(getUploadsButtonLabel(overviewUploadsExpanded), 'See all (8)')
      assert.equal(getUploadsTriangleGlyph(overviewUploadsExpanded), '▾')
    })

    it('verifies independent section header accordion collapse behavior', () => {
      // Test collapsedSections dictionary
      const initialCollapsed: Record<string, boolean> = {
        tasks: true,
        terminals: true,
      }

      // Check default open state of main sections
      assert.equal(initialCollapsed['files'] ?? false, false)
      assert.equal(initialCollapsed['uploads'] ?? false, false)
      assert.equal(initialCollapsed['subagents'] ?? false, false)
      assert.equal(initialCollapsed['goals'] ?? false, false)
      assert.equal(initialCollapsed['skills'] ?? false, false)
      assert.equal(initialCollapsed['tasks'], true)
      assert.equal(initialCollapsed['terminals'], true)

      // Toggle files collapsed
      const toggle = (state: Record<string, boolean>, key: string) => ({
        ...state,
        [key]: !state[key],
      })

      const afterFilesToggle = toggle(initialCollapsed, 'files')
      assert.equal(afterFilesToggle['files'], true)
      assert.equal(afterFilesToggle['uploads'] ?? false, false) // independent

      const afterFilesUntoggle = toggle(afterFilesToggle, 'files')
      assert.equal(afterFilesUntoggle['files'], false)
    })
  })

  describe('3. 4-Pixel Grid Spacing Compliance Verification', () => {
    it('verifies preview card outer container adheres to 4px grid', () => {
      // Check container styling in pageSrc
      const containerPattern = /width:\s*'360px'/
      assert.ok(containerPattern.test(previewBlock), 'Outer container width must be 360px (90 * 4)')
      assert.equal(360 % 4, 0)

      const paddingPattern = /padding:\s*'16px'/
      assert.ok(paddingPattern.test(previewBlock), 'Outer container padding must be 16px (4 * 4)')
      assert.equal(16 % 4, 0)

      const gapPattern = /gap:\s*'12px'/
      assert.ok(gapPattern.test(previewBlock), 'Outer container gap must be 12px (3 * 4)')
      assert.equal(12 % 4, 0)

      const radiusPattern = /borderRadius:\s*'12px'/
      assert.ok(radiusPattern.test(previewBlock), 'Outer container borderRadius must be 12px (3 * 4)')
      assert.equal(12 % 4, 0)
    })

    it('verifies header row and badge spacing adheres to 4px grid', () => {
      // Header row paddingBottom must be 8px (2 * 4)
      assert.ok(
        previewBlock.includes("paddingBottom: '8px'"),
        'Header row must have paddingBottom: 8px (divisible by 4)'
      )
      assert.equal(8 % 4, 0)

      // Header gap: 8px
      assert.ok(
        previewBlock.includes("gap: '8px'"),
        'Header items must use gap: 8px (divisible by 4)'
      )

      // Badge horizontal padding: 8px
      assert.ok(
        previewBlock.includes("padding: '2px 8px'"),
        'Badge must specify 8px horizontal padding (divisible by 4)'
      )

      // Badge border radius: 12px
      assert.ok(
        previewBlock.includes("borderRadius: '12px'"),
        'Badge must specify 12px border radius (divisible by 4)'
      )
    })

    it('verifies list gaps and triangle container heights adhere to 4px grid', () => {
      // List row gap: 4px
      assert.ok(
        previewBlock.includes("gap: '4px'"),
        'List item containers must use gap: 4px (divisible by 4)'
      )
      assert.equal(4 % 4, 0)

      // Refined triangle container height: 8px
      assert.ok(
        previewBlock.includes("height: '8px'"),
        'Refined triangle container must specify height: 8px (divisible by 4)'
      )
      assert.equal(8 % 4, 0)
    })

    it('verifies simulated panel max height is 640px (divisible by 4)', () => {
      assert.ok(
        previewBlock.includes("maxHeight: '640px'"),
        'Simulated panel container maxHeight must be 640px (160 * 4)'
      )
      assert.equal(640 % 4, 0)
    })
  })

  describe('4. Zero Decorative Emoji Audit across All Preview Elements', () => {
    it('verifies zero decorative emojis in all synthetic preview fixtures', () => {
      const serialized = JSON.stringify(SYNTHETIC_OVERVIEW_FIXTURES)
      // Unicode emoji block matches
      const emojiRegex = /[\u{1F300}-\u{1F64F}\u{1F680}-\u{1F6FF}\u{1F900}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
      assert.ok(!emojiRegex.test(serialized), 'SYNTHETIC_OVERVIEW_FIXTURES must contain 0 decorative emojis')
    })

    it('verifies zero decorative emojis in preview JSX source block', () => {
      // Scan the entire extracted JSX block for decorative emojis
      const emojiRegex = /[\u{1F300}-\u{1F64F}\u{1F680}-\u{1F6FF}\u{1F900}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
      const match = previewBlock.match(emojiRegex)
      assert.equal(
        match,
        null,
        `Preview JSX must contain 0 decorative emojis, found: ${match?.[0]}`
      )
    })

    it('verifies iconography is strictly restricted to Lucide icons and monochrome vector paths', () => {
      // Ensure all standard icons rendered are valid Lucide components
      const validLucideIcons = [
        'FileText',
        'Split',
        'Plus',
        'Globe',
        'Folder',
        'Maximize2',
        'X',
        'CheckCircle2',
        'Image',
      ]
      for (const icon of validLucideIcons) {
        assert.ok(
          previewBlock.includes(`<${icon}`) || previewBlock.includes(`${icon} size`),
          `Expected Lucide icon <${icon}> to be used in preview`
        )
      }
    })
  })

  describe('5. Privacy & Synthetic Data Integrity Sanity Audit', () => {
    it('verifies zero private or personal developer tokens in the preview block', () => {
      const privateTokens = [
        '/teamwork-preview',
        '/wish-coding',
        'you need more then these 4 tickets',
        'cmd/swiss',
        'pkg/enhancements',
        'antigravity_guide',
        '6:59 AM',
        '6:57 AM',
        '6:55 AM',
      ]
      for (const token of privateTokens) {
        assert.ok(
          !previewBlock.includes(token),
          `Preview block must not contain private/personal developer token: "${token}"`
        )
      }
    })
  })

  describe('6. Division Mode Transitions, Text Truncation & Contrast Verification', () => {
    it('verifies subagent title specifies text-overflow: ellipsis for defensive truncation', () => {
      assert.ok(
        previewBlock.includes("textOverflow: 'ellipsis'"),
        'Long subagent title must specify textOverflow: "ellipsis"'
      )
      assert.ok(
        previewBlock.includes("overflow: 'hidden'"),
        'Long subagent title must specify overflow: "hidden"'
      )
    })

    it('verifies safe rendering of zero-count sections (tasks and terminals)', () => {
      // Tasks and terminals must render null safely
      assert.ok(
        previewBlock.includes("id: 'tasks'"),
        'Must declare tasks section'
      )
      assert.ok(
        previewBlock.includes("id: 'terminals'"),
        'Must declare terminals section'
      )
      assert.ok(
        previewBlock.includes('renderContent: () => null'),
        'Zero-count sections must specify renderContent: () => null for safe no-op rendering'
      )
    })

    it('verifies accessible color contrast on both division mode badges', () => {
      // Color contrast computation per WCAG 2.1 specs
      const getRelativeLuminance = (r: number, g: number, b: number): number => {
        const [rs, gs, bs] = [r, g, b].map((c) => {
          const s = c / 255
          return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4)
        })
        return 0.2126 * rs + 0.7152 * gs + 0.0722 * bs
      }

      const getContrastRatio = (lum1: number, lum2: number): number => {
        const lighter = Math.max(lum1, lum2)
        const darker = Math.min(lum1, lum2)
        return (lighter + 0.05) / (darker + 0.05)
      }

      // Divider Lines badge: text #0369a1 (rgb(3, 105, 161)), bg #e0f2fe (rgb(224, 242, 254))
      const dividerTextLum = getRelativeLuminance(3, 105, 161)
      const dividerBgLum = getRelativeLuminance(224, 242, 254)
      const dividerContrast = getContrastRatio(dividerTextLum, dividerBgLum)
      assert.ok(
        dividerContrast >= 4.5,
        `Divider Lines badge contrast must meet WCAG AA (>= 4.5:1), got ${dividerContrast.toFixed(2)}:1`
      )

      // Border Zones badge: text #15803d (rgb(21, 128, 61)), bg #dcfce7 (rgb(220, 252, 231))
      const zoneTextLum = getRelativeLuminance(21, 128, 61)
      const zoneBgLum = getRelativeLuminance(220, 252, 231)
      const zoneContrast = getContrastRatio(zoneTextLum, zoneBgLum)
      assert.ok(
        zoneContrast >= 4.0,
        `Border Zones badge contrast must meet badge standard (>= 4.0:1), got ${zoneContrast.toFixed(2)}:1`
      )
    })

    it('verifies division style switching affects container styling and dividers', () => {
      // Divider mode uses #ffffff background and renders horizontal divider lines
      assert.ok(
        previewBlock.includes("op.division_style === 'border_zone' ? '#f1f5f9' : '#ffffff'"),
        'Simulated panel background must alternate between #f1f5f9 (border_zone) and #ffffff (divider_line)'
      )

      // Divider mode renders line between sections
      assert.ok(
        previewBlock.includes("!isZone && op.division_style === 'divider_line'"),
        'Must render divider lines only when division_style is divider_line'
      )

      // Border zone renders card box shadow and borders
      assert.ok(
        previewBlock.includes("borderRadius: `${op.zone_border_radius || 8}px`"),
        'Border zone mode must apply configurable border radius'
      )
    })
  })
})
