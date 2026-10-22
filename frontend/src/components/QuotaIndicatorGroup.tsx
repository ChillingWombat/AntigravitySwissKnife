import React from 'react'
import { CircularGauge } from './CircularGauge'
import { HorizontalQuotaBar } from './HorizontalQuotaBar'
import type { QuotaBucket } from '../types'

interface QuotaIndicatorGroupProps {
  /** Every quota window reported by the provider (5H / 7D / Monthly ...). */
  buckets: QuotaBucket[]
  /** Ring size used when there is exactly one quota window. */
  size?: number
  strokeWidth?: number
  /** Fallback ring label when the provider only reports a single scalar quota. */
  singleTitle?: string
}

/**
 * Renders one progress indicator per fetched quota (#127).
 *
 * Rendering rule (user-specified): a single quota stays a ring; with two or
 * more quotas the widget switches to labelled progress bars — one bar per
 * quota — so every window (5H, 7D, Monthly ...) is visible at once.
 */
export const QuotaIndicatorGroup: React.FC<QuotaIndicatorGroupProps> = ({
  buckets,
  size = 88,
  strokeWidth = 8,
  singleTitle,
}) => {
  const tracked = buckets.filter((b) => b && b.label)

  if (tracked.length <= 1) {
    const only = tracked[0]
    const hasValue = only?.fraction !== null && only?.fraction !== undefined && !isNaN(only.fraction as number)
    const pct = hasValue ? Math.max(0, Math.min(100, Math.round((only!.fraction as number) * 100))) : null
    const title = only?.value
      ? `${only.label}: ${only.value}`
      : singleTitle || only?.label || 'N/A'
    return <CircularGauge percentage={pct} emptyGrey={!hasValue} title={title} size={size} strokeWidth={strokeWidth} />
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: '8px',
        width: '176px',
        flexShrink: 0,
      }}
    >
      {tracked.map((b) => {
        const hasFraction = b.fraction !== null && b.fraction !== undefined && !isNaN(b.fraction as number)
        let valueText: string
        if (hasFraction) {
          const pct = Math.max(0, Math.min(100, Math.round((b.fraction as number) * 100)))
          if (b.value && b.value.trim().length > 0) {
            const trimmed = b.value.trim()
            valueText = trimmed.toLowerCase().endsWith('left') ? trimmed : `${trimmed} left`
          } else {
            valueText = `${pct}% left`
          }
        } else {
          valueText = b.value || 'N/A'
        }

        return (
          <div key={b.label} style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: '6px',
                fontSize: '10px',
                fontWeight: 600,
                color: 'var(--text-muted)',
                whiteSpace: 'nowrap',
              }}
            >
              <span>{b.label}</span>
              <span style={{ color: 'var(--text)' }}>{valueText}</span>
            </div>
            <HorizontalQuotaBar
              fraction={b.fraction}
              height={6}
              maxWidth="100%"
              showPercentage={false}
              title={b.message || `${b.label}: ${valueText}`}
            />
          </div>
        )
      })}
    </div>
  )
}
