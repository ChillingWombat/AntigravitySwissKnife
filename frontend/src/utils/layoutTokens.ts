/**
 * Golden Ratio Layout Architecture & 4-Pixel Grid Tokens
 *
 * Implements strict 4-pixel grid alignment and Golden Ratio (phi ~ 1.618034)
 * proportions for top-level workspace zones, cards, and modal dialogs.
 */

// 1. Base 4-Pixel Grid Constants
export const GRID_UNIT = 4
export const WINDOW_MIN_WIDTH = 1152 // 288 * 4 (Strict 16:9)
export const WINDOW_MIN_HEIGHT = 648 // 162 * 4 (Strict 16:9)
export const WINDOW_ASPECT_RATIO_W = 16
export const WINDOW_ASPECT_RATIO_H = 9
export const WINDOW_ASPECT_RATIO = 16 / 9 // 1.7777777777777777

// Top-Level Zone Dimensions
export const NAV_RAIL_WIDTH = 220 // 55 * 4
export const HEADER_HEIGHT = 72 // 18 * 4
export const WORKSPACE_MIN_WIDTH = 932 // 233 * 4 (WINDOW_MIN_WIDTH - NAV_RAIL_WIDTH)
export const WORKSPACE_MIN_HEIGHT = 576 // 144 * 4 (WINDOW_MIN_HEIGHT - HEADER_HEIGHT)
export const WORKSPACE_ASPECT_RATIO = 932 / 576 // 1.6180555555555556 (error < 0.00003 from PHI)

// Golden Ratio Constant
export const PHI = 1.618033988749895

// Spacing Scale (Integer multiples of 4 pixels)
export const SPACING = {
  XXS: 4,
  XS: 8,
  SM: 12,
  MD: 16,
  LG: 20,
  XL: 24,
  XXL: 28,
  XXXL: 32,
} as const

// 2. Grid & Rounding Helpers
export function snapToGrid4(val: number): number {
  return Math.round(val / 4) * 4
}

export function ceilToGrid4(val: number): number {
  return Math.ceil(val / 4) * 4
}

export function floorToGrid4(val: number): number {
  return Math.floor(val / 4) * 4
}

export function isGridAligned4(val: number): boolean {
  return Number.isInteger(val) && val % 4 === 0
}

// 3. Ceiling 4-Increment Step Rule: W_major = ceil(W / phi)_4
// Biases the partitioned ratio closer to 16:9 (1.7778 > 1.6180)
export function calcMajorWidthCeil4(containerWidth: number): number {
  return ceilToGrid4(containerWidth / PHI)
}

export function calcMinorWidth(containerWidth: number, majorWidth: number, gap = 0): number {
  return containerWidth - majorWidth - gap
}

export function calcGoldenSplit(containerWidth: number, gap = 0): { major: number; minor: number } {
  const available = containerWidth - gap
  const major = ceilToGrid4(available / PHI)
  const minor = available - major
  return { major, minor }
}

export function calcGoldenDimensionsFromHeight(height: number): {
  width: number
  height: number
  aspectRatio: number
} {
  const hSnapped = snapToGrid4(height)
  const wCeil4 = ceilToGrid4(hSnapped * PHI)
  return {
    width: wCeil4,
    height: hSnapped,
    aspectRatio: hSnapped === 0 ? 0 : wCeil4 / hSnapped,
  }
}

export function calcGoldenDimensionsFromWidth(width: number): {
  width: number
  height: number
  aspectRatio: number
} {
  const wSnapped = snapToGrid4(width)
  const hSnapped = snapToGrid4(wSnapped / PHI)
  return {
    width: wSnapped,
    height: hSnapped,
    aspectRatio: hSnapped === 0 ? 0 : wSnapped / hSnapped,
  }
}
