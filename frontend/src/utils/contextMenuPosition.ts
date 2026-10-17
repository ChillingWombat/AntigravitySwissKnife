export interface MenuPoint {
  x: number
  y: number
}

export interface MenuSize {
  width: number
  height: number
}

export interface MenuViewport {
  width: number
  height: number
}

export interface MenuPlacement {
  top: number
  left: number
  flippedX: boolean
  flippedY: boolean
}

/**
 * Places a floating menu so it always stays inside the viewport.
 *
 * Preference order: open at the pointer, flip to the opposite side of the
 * pointer when the menu would overflow, and finally clamp to the viewport
 * edges (covers menus taller/wider than the available space).
 */
export function clampContextMenuPosition(
  point: MenuPoint,
  size: MenuSize,
  viewport: MenuViewport,
  margin = 8
): MenuPlacement {
  const maxLeft = Math.max(margin, viewport.width - size.width - margin)
  const maxTop = Math.max(margin, viewport.height - size.height - margin)

  let left = point.x
  let top = point.y
  let flippedX = false
  let flippedY = false

  if (left + size.width + margin > viewport.width) {
    const flippedLeft = point.x - size.width
    if (flippedLeft >= margin) {
      left = flippedLeft
      flippedX = true
    } else {
      left = maxLeft
    }
  }
  if (top + size.height + margin > viewport.height) {
    const flippedTop = point.y - size.height
    if (flippedTop >= margin) {
      top = flippedTop
      flippedY = true
    } else {
      top = maxTop
    }
  }

  left = Math.min(Math.max(left, margin), maxLeft)
  top = Math.min(Math.max(top, margin), maxTop)

  return { top, left, flippedX, flippedY }
}
