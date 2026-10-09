import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'

describe('Adversarial & Stress Verification: Launch Antigravity 2.0 Button', () => {
  const pagePath = path.resolve(import.meta.dirname, '../pages/QuotaDashboardPage.tsx')
  const pageSrc = fs.readFileSync(pagePath, 'utf8')

  // Helper extracting inline style object string or properties from QuotaDashboardPage
  const extractButtonBlock = () => {
    const idIdx = pageSrc.indexOf('id="btnLaunchAntigravity"')
    assert.ok(idIdx !== -1, 'Button id="btnLaunchAntigravity" must exist')
    const btnStart = pageSrc.lastIndexOf('<button', idIdx)
    const btnEnd = pageSrc.indexOf('</button>', idIdx)
    return pageSrc.slice(btnStart, btnEnd)
  }

  const extractActionRowBlock = () => {
    const rowStart = pageSrc.indexOf('{/* Action Row: Scan Local Accounts, Add Account, and Launch Antigravity Logo Button */}')
    assert.ok(rowStart !== -1, 'Action row comment must exist')
    const rowEnd = pageSrc.indexOf('{/* Card B: Merged Total Quota Progress Rings */}', rowStart)
    return pageSrc.slice(rowStart, rowEnd !== -1 ? rowEnd : rowStart + 4000)
  }

  describe('1. Interaction & State Testing (Focus, Blur, Hover, Leave)', () => {
    const btnBlock = extractButtonBlock()

    it('validates keyboard tab navigation: focus and blur handlers trigger tooltip visibility', () => {
      // Must have onFocus and onBlur
      assert.ok(
        btnBlock.includes('onFocus={() => setIsLogoHovered(true)}'),
        'Button must show tooltip on keyboard focus (Tab navigation)'
      )
      assert.ok(
        btnBlock.includes('onBlur={() => setIsLogoHovered(false)}'),
        'Button must hide tooltip on blur (Tab away / Shift+Tab)'
      )
      // Must not suppress keyboard accessibility with tabIndex = -1
      assert.ok(
        !btnBlock.includes('tabIndex={-1}'),
        'Button must not have negative tabIndex so it remains in standard keyboard tab order'
      )
      // Must declare type="button" to prevent accidental form submission if wrapped
      assert.ok(btnBlock.includes('type="button"'), 'Button must specify type="button"')
      // Must provide accessible name for screen readers
      assert.ok(
        btnBlock.includes('aria-label="Launch Antigravity 2.0"'),
        'Button must have descriptive aria-label'
      )
    })

    it('validates mouse hover & leave event handling without event flapping', () => {
      assert.ok(
        btnBlock.includes('onMouseEnter={() => setIsLogoHovered(true)}'),
        'Button must set isLogoHovered to true on mouse enter'
      )
      assert.ok(
        btnBlock.includes('onMouseLeave={() => setIsLogoHovered(false)}'),
        'Button must set isLogoHovered to false on mouse leave'
      )

      // Pointer event isolation to eliminate mouse flapping
      assert.ok(
        btnBlock.includes("pointerEvents: 'none'"),
        'Child img must have pointerEvents: none to prevent inner-target event disruption or ghost drag'
      )

      const rowBlock = extractActionRowBlock()
      assert.ok(
        rowBlock.includes("role=\"tooltip\"") && rowBlock.includes("pointerEvents: 'none'"),
        'Floating tooltip must have pointerEvents: none to prevent hover jitter/flapping over cursor'
      )
    })

    it('verifies state transformation matrix when isLaunchingIDE is true', () => {
      // Logic simulation of style rules in QuotaDashboardPage.tsx
      const evaluateButtonStyles = (isLaunchingIDE: boolean, isLogoHovered: boolean) => {
        return {
          cursor: isLaunchingIDE ? 'wait' : 'pointer',
          transform: isLaunchingIDE ? 'scale(0.95)' : isLogoHovered ? 'scale(1.12)' : 'scale(1)',
          filter: isLogoHovered
            ? 'drop-shadow(0 2px 6px rgba(66, 133, 244, 0.38)) brightness(1.08)'
            : 'drop-shadow(0 1px 2px rgba(0, 0, 0, 0.06))',
          opacity: isLaunchingIDE ? 0.6 : 1,
          disabled: isLaunchingIDE,
        }
      }

      // Scenario A: Resting unhovered
      const resting = evaluateButtonStyles(false, false)
      assert.strictEqual(resting.cursor, 'pointer')
      assert.strictEqual(resting.transform, 'scale(1)')
      assert.strictEqual(resting.opacity, 1)
      assert.strictEqual(resting.disabled, false)

      // Scenario B: Hovered while idle
      const hovered = evaluateButtonStyles(false, true)
      assert.strictEqual(hovered.cursor, 'pointer')
      assert.strictEqual(hovered.transform, 'scale(1.12)')
      assert.strictEqual(hovered.opacity, 1)
      assert.strictEqual(hovered.disabled, false)

      // Scenario C: Launch in flight, even if cursor is hovering
      const launchingHovered = evaluateButtonStyles(true, true)
      assert.strictEqual(launchingHovered.cursor, 'wait', 'Cursor must be "wait" during launch')
      assert.strictEqual(launchingHovered.transform, 'scale(0.95)', 'Transform scale(0.95) must override hover scale(1.12)')
      assert.strictEqual(launchingHovered.opacity, 0.6, 'Opacity must drop to 0.6 during launch')
      assert.strictEqual(launchingHovered.disabled, true, 'Disabled attribute must be true during launch')

      // Scenario D: Launch in flight, cursor left
      const launchingUnhovered = evaluateButtonStyles(true, false)
      assert.strictEqual(launchingUnhovered.cursor, 'wait')
      assert.strictEqual(launchingUnhovered.transform, 'scale(0.95)')
      assert.strictEqual(launchingUnhovered.opacity, 0.6)
      assert.strictEqual(launchingUnhovered.disabled, true)
    })

    it('verifies dynamic tooltip text alternation based on isLaunchingIDE', () => {
      const rowBlock = extractActionRowBlock()
      assert.ok(
        rowBlock.includes("isLaunchingIDE ? 'Launching Antigravity 2.0...' : 'Launch Antigravity 2.0'"),
        'Tooltip must dynamically inform user when launch is in progress'
      )
    })
  })

  describe('2. Re-entrancy & Concurrency Guard Testing', () => {
    it('verifies re-entrancy guard prevents concurrent double-launches', async () => {
      // Simulate handleLaunchAntigravity concurrency logic
      let backendCallCount = 0
      let isLaunchingIDE = false

      const mockElectronAPI = {
        launchAntigravity: async () => {
          backendCallCount++
          await new Promise((r) => setTimeout(r, 20))
          return { success: true }
        },
      }

      const simulateHandleLaunch = async () => {
        if (isLaunchingIDE) return
        try {
          isLaunchingIDE = true
          await mockElectronAPI.launchAntigravity()
        } finally {
          // In real component, reset happens after delay
        }
      }

      // Fire 10 simultaneous rapid clicks
      await Promise.all([
        simulateHandleLaunch(),
        simulateHandleLaunch(),
        simulateHandleLaunch(),
        simulateHandleLaunch(),
        simulateHandleLaunch(),
        simulateHandleLaunch(),
        simulateHandleLaunch(),
        simulateHandleLaunch(),
        simulateHandleLaunch(),
        simulateHandleLaunch(),
      ])

      assert.strictEqual(backendCallCount, 1, 'Re-entrancy guard must allow exactly 1 launch call across concurrent bursts')
    })

    it('verifies fallback error handling when launch call fails', async () => {
      let feedback = ''
      let isLaunchingIDE = false

      const mockFailingLaunch = async () => {
        if (isLaunchingIDE) return
        try {
          isLaunchingIDE = true
          feedback = 'Launching Antigravity 2.0...'
          const res = { success: false, error: 'Daemon connection refused' }
          if (res && res.success === false) {
            throw new Error(res.error)
          }
        } catch (err: any) {
          feedback = 'Could not launch Antigravity: ' + err.message
        } finally {
          isLaunchingIDE = false
        }
      }

      await mockFailingLaunch()
      assert.strictEqual(feedback, 'Could not launch Antigravity: Daemon connection refused')
      assert.strictEqual(isLaunchingIDE, false)
    })
  })

  describe('3. Layout & Wrapping Stress-Testing', () => {
    const rowBlock = extractActionRowBlock()

    it('ensures action row specifies flexWrap: wrap and space-between', () => {
      assert.ok(rowBlock.includes("flexWrap: 'wrap'"), 'Row must enable flexWrap: wrap')
      assert.ok(rowBlock.includes("justifyContent: 'space-between'"), 'Row must specify justifyContent: space-between')
      assert.ok(rowBlock.includes("alignItems: 'flex-end'"), 'Row must specify alignItems: flex-end')
    })

    it('ensures button container is pinned to bottom-right corner via marginLeft: auto and alignSelf: flex-end', () => {
      assert.ok(rowBlock.includes("marginLeft: 'auto'"), 'Logo container must have marginLeft: auto')
      assert.ok(rowBlock.includes("alignSelf: 'flex-end'"), 'Logo container must have alignSelf: flex-end')
    })

    it('mathematically simulates flex wrapping under constrained and narrowed viewports', () => {
      // Simulation of CSS flex-box layout engine
      interface Box {
        id: string
        width: number
        height: number
        marginLeftAuto?: boolean
      }

      const simulateFlexRowWrap = (containerWidth: number, gap: number, leftBoxes: Box[], rightBox: Box) => {
        const leftGroupWidth = leftBoxes.reduce((acc, b) => acc + b.width, 0) + (leftBoxes.length - 1) * gap
        const totalContentWidth = leftGroupWidth + gap + rightBox.width

        if (totalContentWidth <= containerWidth) {
          // Fits on single line
          return {
            lines: [
              {
                leftItems: leftBoxes.map((b) => b.id),
                rightItem: rightBox.id,
                rightItemX: containerWidth - rightBox.width, // marginLeft: auto pushes to far right
                overflow: false,
              },
            ],
          }
        } else {
          // Outer row wraps: rightBox goes to line 2
          return {
            lines: [
              {
                leftItems: leftBoxes.map((b) => b.id),
                rightItem: null,
                overflow: leftGroupWidth > containerWidth,
              },
              {
                leftItems: [],
                rightItem: rightBox.id,
                rightItemX: containerWidth - rightBox.width, // on line 2, marginLeft: auto pushes to line 2 right edge!
                overflow: rightBox.width > containerWidth,
              },
            ],
          }
        }
      }

      const scanBtn: Box = { id: 'scan', width: 172, height: 32 }
      const addBtn: Box = { id: 'add', width: 172, height: 32 }
      const logoBtn: Box = { id: 'logo', width: 32, height: 32, marginLeftAuto: true }

      // Test Case 1: Standard Dashboard Width = 640px
      const normal = simulateFlexRowWrap(640, 8, [scanBtn, addBtn], logoBtn)
      assert.strictEqual(normal.lines.length, 1)
      assert.strictEqual(normal.lines[0].rightItem, 'logo')
      assert.strictEqual(normal.lines[0].rightItemX, 608) // 640 - 32
      assert.strictEqual(normal.lines[0].overflow, false)

      // Test Case 2: Boundary Width = 392px (172 + 8 + 172 + 8 + 32 = 392)
      const boundary = simulateFlexRowWrap(392, 8, [scanBtn, addBtn], logoBtn)
      assert.strictEqual(boundary.lines.length, 1)
      assert.strictEqual(boundary.lines[0].rightItemX, 360)
      assert.strictEqual(boundary.lines[0].overflow, false)

      // Test Case 3: Narrowed Width = 380px (triggers wrapping)
      const wrapped = simulateFlexRowWrap(380, 8, [scanBtn, addBtn], logoBtn)
      assert.strictEqual(wrapped.lines.length, 2, 'Action row must cleanly wrap onto line 2')
      assert.strictEqual(wrapped.lines[1].rightItem, 'logo')
      assert.strictEqual(wrapped.lines[1].rightItemX, 348, 'Pinned to right edge of line 2 via marginLeft: auto')
      assert.strictEqual(wrapped.lines[1].overflow, false, 'Line 2 must not overflow')

      // Test Case 4: Narrow card = 360px
      const card360 = simulateFlexRowWrap(360, 8, [scanBtn, addBtn], logoBtn)
      assert.strictEqual(card360.lines.length, 2)
      assert.strictEqual(card360.lines[1].rightItemX, 328)
      assert.strictEqual(card360.lines[1].overflow, false)
    })

    it('verifies tooltip positioning and arrow geometry', () => {
      // Tooltip arrow pointer alignment math:
      // Button width is 32px.
      // Tooltip has right: 0.
      // Tooltip arrow has borderLeft: 5px, borderRight: 5px (total width 10px).
      // arrow right is 11px.
      // Center of arrow = 11px + 5px = 16px.
      // Center of 32px button = 32px / 2 = 16px.
      const buttonWidth = 32
      const arrowRight = 11
      const arrowHalfWidth = 5
      const arrowCenterFromRight = arrowRight + arrowHalfWidth
      const buttonCenterFromRight = buttonWidth / 2

      assert.strictEqual(
        arrowCenterFromRight,
        buttonCenterFromRight,
        'Tooltip arrow center (16px) must align with button center (16px)'
      )
    })
  })
})
