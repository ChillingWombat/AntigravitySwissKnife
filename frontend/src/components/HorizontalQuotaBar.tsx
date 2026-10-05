import React from 'react'

interface HorizontalQuotaBarProps {
  fraction: number // 0.0 to 1.0
  height?: number
  maxWidth?: number | string
  title?: string
}

export const HorizontalQuotaBar: React.FC<HorizontalQuotaBarProps> = ({
  fraction,
  height = 9,
  maxWidth = 160,
  title,
}) => {
  const pct = Math.max(0, Math.min(100, Math.round(fraction * 100)))

  let barColor = '#137333' // Google green
  if (pct < 20) {
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
        gap: '6px',
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
          color: 'var(--text)',
          whiteSpace: 'nowrap',
        }}
      >
        {pct}%
      </span>
    </div>
  )
}
