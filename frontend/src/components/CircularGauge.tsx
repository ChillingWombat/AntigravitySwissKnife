import React from 'react'

interface CircularGaugeProps {
  percentage: number | null // 0 to 100, or null if untracked/no quota
  title: string
  size?: number
  strokeWidth?: number
  emptyGrey?: boolean
}

export const CircularGauge: React.FC<CircularGaugeProps> = ({
  percentage,
  title,
  size = 130,
  strokeWidth = 11,
  emptyGrey = false,
}) => {
  const radius = (size - strokeWidth) / 2
  const circumference = 2 * Math.PI * radius

  const hasValue = percentage !== null && !emptyGrey && percentage >= 0
  const clamped = hasValue ? Math.max(0, Math.min(100, Math.round(percentage as number))) : 0
  const strokeDashoffset = circumference - (clamped / 100) * circumference

  // Color selection based on health
  let strokeColor = '#137333' // Google green
  if (clamped < 20) {
    strokeColor = '#b3261e' // Red
  } else if (clamped < 50) {
    strokeColor = '#b06000' // Yellow/Orange
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center' }}>
      <div style={{ position: 'relative', width: size, height: size }}>
        <svg width={size} height={size} style={{ transform: 'rotate(-90deg)' }}>
          {/* Background track (Grey) */}
          <circle
            cx={size / 2}
            cy={size / 2}
            r={radius}
            fill="none"
            stroke="#e5e9f0"
            strokeWidth={strokeWidth}
          />
          {/* Active Progress - Only rendered when tracking info exists */}
          {hasValue && (
            <circle
              cx={size / 2}
              cy={size / 2}
              r={radius}
              fill="none"
              stroke={strokeColor}
              strokeWidth={strokeWidth}
              strokeDasharray={circumference}
              strokeDashoffset={strokeDashoffset}
              strokeLinecap="round"
              style={{ transition: 'stroke-dashoffset 0.6s cubic-bezier(0.4, 0, 0.2, 1)' }}
            />
          )}
        </svg>

        {/* Centered Percentage or N/A */}
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <span
            style={{
              fontSize: hasValue ? (size <= 80 ? '15px' : '26px') : (size <= 80 ? '12px' : '20px'),
              fontWeight: 700,
              color: hasValue ? 'var(--text)' : 'var(--text-muted)',
              letterSpacing: hasValue ? 'normal' : '0.5px',
            }}
          >
            {hasValue ? `${clamped}%` : 'N/A'}
          </span>
        </div>
      </div>

      <div style={{ marginTop: size <= 80 ? '4px' : '12px' }}>
        <div style={{ fontSize: size <= 80 ? '10px' : '12px', fontWeight: 600, color: 'var(--text)', whiteSpace: 'nowrap' }}>
          {title}
        </div>
      </div>
    </div>
  )
}
