import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { clampContextMenuPosition } from './contextMenuPosition.ts'

const viewport = { width: 1000, height: 800 }
const menu = { width: 220, height: 260 }

describe('clampContextMenuPosition', () => {
  it('keeps the menu at the pointer when it fits', () => {
    const p = clampContextMenuPosition({ x: 100, y: 120 }, menu, viewport)
    assert.equal(p.left, 100)
    assert.equal(p.top, 120)
    assert.equal(p.flippedX, false)
    assert.equal(p.flippedY, false)
  })

  it('flips above the pointer for a bottom-row right click', () => {
    const p = clampContextMenuPosition({ x: 300, y: 760 }, menu, viewport)
    assert.equal(p.top, 760 - 260)
    assert.equal(p.flippedY, true)
    assert.equal(p.left, 300)
  })

  it('flips left of the pointer near the right edge', () => {
    const p = clampContextMenuPosition({ x: 980, y: 100 }, menu, viewport)
    assert.equal(p.left, 980 - 220)
    assert.equal(p.flippedX, true)
  })

  it('clamps to the viewport when the menu is taller than the space on both sides', () => {
    const tall = { width: 220, height: 780 }
    const p = clampContextMenuPosition({ x: 100, y: 400 }, tall, viewport)
    // Flipping is impossible; the menu hugs the bottom edge with the 8px margin.
    assert.equal(p.top, viewport.height - tall.height - 8)
    assert.ok(p.top >= 8)
    assert.ok(p.top + tall.height <= viewport.height)
  })

  it('never places the menu at negative coordinates', () => {
    const p = clampContextMenuPosition({ x: 2, y: 2 }, menu, viewport)
    assert.equal(p.left, 8)
    assert.equal(p.top, 8)
  })

  it('works in a narrow viewport where flipping is impossible', () => {
    const narrow = { width: 300, height: 400 }
    const p = clampContextMenuPosition({ x: 280, y: 380 }, menu, narrow)
    assert.ok(p.left >= 8 && p.left + menu.width <= narrow.width)
    assert.ok(p.top >= 8 && p.top + menu.height <= narrow.height)
  })
})
