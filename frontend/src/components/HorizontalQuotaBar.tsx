import React from 'react'
import { COMPONENT_TOKENS } from '../utils/layoutTokens'

interface HorizontalQuotaBarProps {
  fraction: number // 0.0 to 1.0
  height?: number
  maxWidth?: number | string
  title?: string
  disabled?: boolean
}

export const HorizontalQuotaBar: React.FC<HorizontalQuotaBarProps> = ({
  fraction,
  height = COMPONENT_TOKENS.QUOTA_BAR_HEIGHT,
  maxWidth = 160,
  title,
  disabled = false,
}) => {
  const pct = disabled ? 0 : Math.max(0, Math.min(100, Math.round(fraction * 100)))

  let barColor = '#137333' // Google green
  if (disabled) {
    barColor = '#9aa0a6'
  } else if (pct < 20) {
    barColor = '#b3261e' // Exhausted red
  } else if (pct < 50) {
    barColor = '#b06000' // Warning yellow
  }

  return (
    <div
      title={title}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: `${COMPONENT_TOKENS.QUOTA_BAR_GAP}px`,
        width: '100%',
        maxWidth: maxWidth,
      }}
    >
      <div
        style={{
          flex: 1,
          height: height,
          backgroundColor: '#e5e9f0',
          borderRadius: height / 2,
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            height: '100%',
            width: `${pct}%`,
            backgroundColor: barColor,
            borderRadius: height / 2,
            transition: 'width 0.4s ease',
          }}
        />
      </div>
      <span
        style={{
          fontSize: '11px',
          fontWeight: 700,
          color: disabled ? 'var(--text-muted)' : 'var(--text)',
          whiteSpace: 'nowrap',
        }}
      >
        {disabled ? '--%' : `${pct}%`}
      </span>
    </div>
  )
}
