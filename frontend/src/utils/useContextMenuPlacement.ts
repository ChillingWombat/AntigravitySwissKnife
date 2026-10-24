import { useLayoutEffect, useRef, useState } from 'react'
import { clampContextMenuPosition } from './contextMenuPosition'

interface MenuPoint {
  x: number
  y: number
}

interface MenuCoordinates {
  top: number
  left: number
}

/**
 * Measures a rendered menu and returns viewport-clamped coordinates for it.
 * Render the menu at the returned position with `visibility: hidden` until
 * `menuPosition` is non-null so the first paint happens at the final spot.
 */
export function useContextMenuPlacement(open: boolean, point: MenuPoint | null) {
  const menuRef = useRef<HTMLDivElement | null>(null)
  const [menuPosition, setMenuPosition] = useState<MenuCoordinates | null>(null)
  const x = point?.x ?? 0
  const y = point?.y ?? 0
  const hasPoint = point !== null

  useLayoutEffect(() => {
    if (!open || !hasPoint || !menuRef.current) {
      setMenuPosition(null)
      return
    }

    const measure = () => {
      const el = menuRef.current
      if (!el) return
      const rect = el.getBoundingClientRect()
      const placed = clampContextMenuPosition(
        { x, y },
        { width: rect.width, height: rect.height },
        { width: window.innerWidth, height: window.innerHeight }
      )
      setMenuPosition({ top: placed.top, left: placed.left })
    }

    measure()
    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
  }, [open, hasPoint, x, y])

  return { menuRef, menuPosition }
}
