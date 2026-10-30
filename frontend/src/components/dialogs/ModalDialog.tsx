import React, { useCallback, useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'

export interface ModalDialogProps {
  open: boolean
  title: string
  description?: string
  onClose: () => void
  initialFocusRef?: React.RefObject<HTMLElement | null>
  size?: 'sm' | 'md' | 'lg'
  children: React.ReactNode
}

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

const SIZE_WIDTH = {
  sm: '420px',
  md: '600px',
  lg: '760px',
} as const

const visuallyHidden: React.CSSProperties = {
  position: 'absolute',
  width: '1px',
  height: '1px',
  padding: 0,
  margin: '-1px',
  overflow: 'hidden',
  clip: 'rect(0, 0, 0, 0)',
  whiteSpace: 'nowrap',
  border: 0,
}

let bodyScrollLockCount = 0
let bodyOverflowBeforeLock: string | null = null

const acquireBodyScrollLock = () => {
  if (bodyScrollLockCount === 0) {
    bodyOverflowBeforeLock = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  }
  bodyScrollLockCount += 1
}

const releaseBodyScrollLock = () => {
  if (bodyScrollLockCount === 0) return
  bodyScrollLockCount -= 1
  if (bodyScrollLockCount === 0 && bodyOverflowBeforeLock !== null) {
    document.body.style.overflow = bodyOverflowBeforeLock
    bodyOverflowBeforeLock = null
  }
}

const getFocusableElements = (dialog: HTMLDialogElement): HTMLElement[] =>
  Array.from(dialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    (element) => element.tabIndex >= 0 && !element.hasAttribute('disabled')
  )

export const ModalDialog: React.FC<ModalDialogProps> = ({
  open,
  title,
  description,
  onClose,
  initialFocusRef,
  size = 'md',
  children,
}) => {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const closeRequestedRef = useRef(false)
  const onCloseRef = useRef(onClose)
  const titleId = useId()
  const descriptionId = useId()

  useEffect(() => {
    onCloseRef.current = onClose
  }, [onClose])

  const requestClose = useCallback(() => {
    if (closeRequestedRef.current) return
    closeRequestedRef.current = true
    onCloseRef.current()
  }, [])

  useEffect(() => {
    if (!open) return
    const dialog = dialogRef.current
    if (!dialog || typeof document === 'undefined') return

    closeRequestedRef.current = false
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    acquireBodyScrollLock()

    if (!dialog.hasAttribute('open')) {
      if (typeof dialog.showModal === 'function') {
        try {
          dialog.showModal()
        } catch {
          dialog.setAttribute('open', '')
        }
      } else {
        dialog.setAttribute('open', '')
      }
    }

    const focusTarget = initialFocusRef?.current ?? getFocusableElements(dialog)[0] ?? dialog
    focusTarget.focus()

    return () => {
      releaseBodyScrollLock()
      if (dialog.hasAttribute('open')) {
        if (typeof dialog.close === 'function') {
          try {
            dialog.close()
          } catch {
            dialog.removeAttribute('open')
          }
        } else {
          dialog.removeAttribute('open')
        }
      }
      if (opener?.isConnected) opener.focus()
    }
  }, [initialFocusRef, open])

  const handleKeyDown = (event: React.KeyboardEvent<HTMLDialogElement>) => {
    const dialog = dialogRef.current
    if (!dialog) return

    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      requestClose()
      return
    }

    if (event.key !== 'Tab') return

    const focusable = getFocusableElements(dialog)
    if (focusable.length === 0) {
      event.preventDefault()
      dialog.focus()
      return
    }

    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    const activeElement = document.activeElement

    if (event.shiftKey && (activeElement === first || !dialog.contains(activeElement))) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && (activeElement === last || !dialog.contains(activeElement))) {
      event.preventDefault()
      first.focus()
    }
  }

  const handleCancel = (event: React.SyntheticEvent<HTMLDialogElement>) => {
    event.preventDefault()
    requestClose()
  }

  if (!open || typeof document === 'undefined') return null

  return createPortal(
    <dialog
      ref={dialogRef}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      aria-describedby={description ? descriptionId : undefined}
      tabIndex={-1}
      data-modal-size={size}
      className="modal-dialog"
      onCancel={handleCancel}
      onKeyDown={handleKeyDown}
      style={{
        position: 'fixed',
        inset: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: '100vw',
        height: '100dvh',
        maxWidth: 'none',
        maxHeight: 'none',
        margin: 0,
        padding: '20px',
        border: 0,
        background: 'transparent',
        color: 'inherit',
        overflow: 'auto',
        boxSizing: 'border-box',
      }}
    >
      <div
        aria-hidden="true"
        className="modal-dialog-backdrop"
        style={{
          position: 'fixed',
          inset: 0,
          background: 'rgba(0, 0, 0, 0.45)',
          backdropFilter: 'blur(3px)',
        }}
      />
      <h2 id={titleId} style={visuallyHidden}>
        {title}
      </h2>
      {description && (
        <p id={descriptionId} style={visuallyHidden}>
          {description}
        </p>
      )}
      <div
        className="modal-dialog-frame"
        style={{
          position: 'relative',
          width: SIZE_WIDTH[size],
          maxWidth: '94vw',
          maxHeight: '90vh',
        }}
      >
        {children}
      </div>
    </dialog>,
    document.body
  )
}
