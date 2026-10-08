import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  GRID_UNIT,
  WINDOW_MIN_WIDTH,
  WINDOW_MIN_HEIGHT,
  WINDOW_ASPECT_RATIO,
  NAV_RAIL_WIDTH,
  HEADER_HEIGHT,
  WORKSPACE_MIN_WIDTH,
  WORKSPACE_MIN_HEIGHT,
  WORKSPACE_ASPECT_RATIO,
  WORKSPACE_PADDING_X,
  WORKSPACE_CONTENT_MIN_WIDTH,
  TABLE_MIN_WIDTH,
  COMPONENT_TOKENS,
  PHI,
  SPACING,
  snapToGrid4,
  ceilToGrid4,
  floorToGrid4,
  isGridAligned4,
  calcMajorWidthCeil4,
  calcMinorWidth,
  calcGoldenSplit,
  calcGoldenDimensionsFromHeight,
  calcGoldenDimensionsFromWidth,
} from './layoutTokens.ts'

describe('Layout Tokens & Golden Ratio Math', () => {
  it('enforces 4-pixel grid divisibility on base window and zone constants', () => {
    assert.equal(GRID_UNIT, 4)
    assert.ok(isGridAligned4(WINDOW_MIN_WIDTH), 'WINDOW_MIN_WIDTH must be divisible by 4')
    assert.ok(isGridAligned4(WINDOW_MIN_HEIGHT), 'WINDOW_MIN_HEIGHT must be divisible by 4')
    assert.ok(isGridAligned4(NAV_RAIL_WIDTH), 'NAV_RAIL_WIDTH must be divisible by 4')
    assert.ok(isGridAligned4(HEADER_HEIGHT), 'HEADER_HEIGHT must be divisible by 4')
    assert.ok(isGridAligned4(WORKSPACE_MIN_WIDTH), 'WORKSPACE_MIN_WIDTH must be divisible by 4')
    assert.ok(isGridAligned4(WORKSPACE_MIN_HEIGHT), 'WORKSPACE_MIN_HEIGHT must be divisible by 4')

    assert.equal(WINDOW_MIN_WIDTH % 4, 0)
    assert.equal(WINDOW_MIN_HEIGHT % 4, 0)
    assert.equal(NAV_RAIL_WIDTH % 4, 0)
    assert.equal(HEADER_HEIGHT % 4, 0)
    assert.equal(WORKSPACE_MIN_WIDTH % 4, 0)
    assert.equal(WORKSPACE_MIN_HEIGHT % 4, 0)
  })

  it('verifies strict 16:9 window aspect ratio', () => {
    assert.equal(WINDOW_MIN_WIDTH, 1216)
    assert.equal(WINDOW_MIN_HEIGHT, 684)
    assert.equal(WINDOW_MIN_WIDTH / WINDOW_MIN_HEIGHT, 16 / 9)
    assert.equal(WINDOW_ASPECT_RATIO, 16 / 9)
  })

  it('verifies workspace dimensions aligned to 4-pixel grid', () => {
    assert.equal(WORKSPACE_MIN_WIDTH, WINDOW_MIN_WIDTH - NAV_RAIL_WIDTH)
    assert.equal(WORKSPACE_MIN_HEIGHT, WINDOW_MIN_HEIGHT - HEADER_HEIGHT)
    assert.equal(WORKSPACE_MIN_WIDTH, 1016)
    assert.equal(WORKSPACE_MIN_HEIGHT, 612)
    assert.equal(WORKSPACE_ASPECT_RATIO, 1016 / 612)
    assert.ok(isGridAligned4(WORKSPACE_MIN_WIDTH))
    assert.ok(isGridAligned4(WORKSPACE_MIN_HEIGHT))
  })

  it('validates SPACING scale tokens are all multiples of 4', () => {
    for (const [key, val] of Object.entries(SPACING)) {
      assert.ok(
        isGridAligned4(val),
        `Spacing ${key} (${val}px) must be an integer multiple of 4`
      )
    }
  })

  it('rounds accurately using snapToGrid4, ceilToGrid4, and floorToGrid4', () => {
    assert.equal(snapToGrid4(0), 0)
    assert.equal(snapToGrid4(2), 4) // Math.round(2/4)*4 = 4
    assert.equal(snapToGrid4(1.9), 0)
    assert.equal(snapToGrid4(5), 4)
    assert.equal(snapToGrid4(7), 8)
    assert.equal(snapToGrid4(129), 128)
    assert.equal(snapToGrid4(130), 132)

    assert.equal(ceilToGrid4(1), 4)
    assert.equal(ceilToGrid4(4), 4)
    assert.equal(ceilToGrid4(4.1), 8)
    assert.equal(ceilToGrid4(7), 8)

    assert.equal(floorToGrid4(3.9), 0)
    assert.equal(floorToGrid4(4), 4)
    assert.equal(floorToGrid4(7), 4)
    assert.equal(floorToGrid4(8), 8)
  })

  it('calculates major and minor split widths using ceiling 4-increment step rule', () => {
    const split = calcGoldenSplit(WORKSPACE_MIN_WIDTH)
    assert.ok(isGridAligned4(split.major), 'Major width must be 4-pixel aligned')
    assert.ok(isGridAligned4(split.minor), 'Minor width must be 4-pixel aligned')
    assert.equal(split.major + split.minor, WORKSPACE_MIN_WIDTH)

    // 1016 / PHI = 627.922... -> ceilToGrid4 gives 628
    assert.equal(calcMajorWidthCeil4(WORKSPACE_MIN_WIDTH), 628)
    assert.equal(split.major, 628)
    assert.equal(split.minor, 388)

    // With gap = 16 (available: 1000 -> 1000 / PHI = 618.033... -> ceilToGrid4 gives 620)
    const splitWithGap = calcGoldenSplit(WORKSPACE_MIN_WIDTH, 16)
    assert.ok(isGridAligned4(splitWithGap.major))
    assert.ok(isGridAligned4(splitWithGap.minor))
    assert.equal(splitWithGap.major + splitWithGap.minor + 16, WORKSPACE_MIN_WIDTH)

    // Direct invocation of calcMinorWidth
    const minorCalculated = calcMinorWidth(WORKSPACE_MIN_WIDTH, split.major)
    assert.equal(minorCalculated, 388)
    const minorWithGapCalculated = calcMinorWidth(WORKSPACE_MIN_WIDTH, splitWithGap.major, 16)
    assert.equal(minorWithGapCalculated, splitWithGap.minor)
  })

  it('verifies ceiling 4-increment rule biases ratios closer to 16:9 than floor rounding', () => {
    const TARGET_16_9 = 16 / 9
    const testHeights = [100, 160, 200, 240, 300, 320, 360, 400, 500, 576, 648]

    for (const h of testHeights) {
      const exactWidth = h * PHI
      const ceilW = ceilToGrid4(exactWidth)
      const floorW = floorToGrid4(exactWidth)

      const ceilRatio = ceilW / h
      const floorRatio = floorW / h

      const ceilDelta = Math.abs(ceilRatio - TARGET_16_9)
      const floorDelta = Math.abs(floorRatio - TARGET_16_9)

      assert.ok(
        ceilDelta <= floorDelta,
        `For height ${h}: ceil delta (${ceilDelta}) must be <= floor delta (${floorDelta}) relative to 16:9`
      )
    }
  })

  it('computes golden dimensions from height and width snapped to 4px', () => {
    const dimH = calcGoldenDimensionsFromHeight(200)
    assert.ok(isGridAligned4(dimH.width))
    assert.ok(isGridAligned4(dimH.height))
    assert.equal(dimH.height, 200)
    assert.equal(dimH.width, ceilToGrid4(200 * PHI)) // 200 * 1.618034 = 323.6 -> 324
    assert.equal(dimH.width, 324)

    const dimW = calcGoldenDimensionsFromWidth(500)
    assert.ok(isGridAligned4(dimW.width))
    assert.ok(isGridAligned4(dimW.height))
  })

  it('validates COMPONENT_TOKENS are all positive multiples of 4', () => {
    for (const [key, val] of Object.entries(COMPONENT_TOKENS)) {
      assert.ok(
        isGridAligned4(val),
        `Component token ${key} (${val}px) must be an integer multiple of 4`
      )
      assert.ok(val > 0, `Component token ${key} must be positive`)
    }
  })

  it('verifies viewport budgeting and table width constraints', () => {
    assert.ok(isGridAligned4(WORKSPACE_PADDING_X), 'WORKSPACE_PADDING_X must be divisible by 4')
    assert.ok(isGridAligned4(TABLE_MIN_WIDTH), 'TABLE_MIN_WIDTH must be divisible by 4')
    assert.equal(WORKSPACE_CONTENT_MIN_WIDTH, WORKSPACE_MIN_WIDTH - 2 * WORKSPACE_PADDING_X)
    assert.equal(WORKSPACE_CONTENT_MIN_WIDTH, 968)
    assert.equal(TABLE_MIN_WIDTH, 880)
    assert.ok(
      TABLE_MIN_WIDTH <= WORKSPACE_CONTENT_MIN_WIDTH,
      `TABLE_MIN_WIDTH (${TABLE_MIN_WIDTH}) must fit within WORKSPACE_CONTENT_MIN_WIDTH (${WORKSPACE_CONTENT_MIN_WIDTH}) to eliminate horizontal scrollbar`
    )
  })
})
