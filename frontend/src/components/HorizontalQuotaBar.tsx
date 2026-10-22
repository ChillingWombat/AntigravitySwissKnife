import React from 'react'
import { COMPONENT_TOKENS } from '../utils/layoutTokens'

interface HorizontalQuotaBarProps {
  fraction: number | null | undefined // 0.0 to 1.0, or null if untracked
  height?: number
  maxWidth?: number | string
  title?: string
  disabled?: boolean
  showPercentage?: boolean
}

export const HorizontalQuotaBar: React.FC<HorizontalQuotaBarProps> = ({
  fraction,
  height = COMPONENT_TOKENS.QUOTA_BAR_HEIGHT,
  maxWidth = 160,
  title,
  disabled = false,
  showPercentage = true,
}) => {
  const isUntracked = disabled || fraction === null || fraction === undefined || isNaN(fraction)
  const pct = isUntracked ? 0 : Math.max(0, Math.min(100, Math.round((fraction as number) * 100)))

  let barColor = '#137333' // Google green
  if (isUntracked) {
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
      {showPercentage !== false && (
        <span
          style={{
            fontSize: '11px',
            fontWeight: 700,
            color: isUntracked ? 'var(--text-muted)' : 'var(--text)',
            whiteSpace: 'nowrap',
          }}
        >
          {isUntracked ? 'N/A' : `${pct}%`}
        </span>
      )}
    </div>
  )
}
