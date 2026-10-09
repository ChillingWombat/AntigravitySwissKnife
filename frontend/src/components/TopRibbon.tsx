import React from 'react'

interface TopRibbonProps {
  currentTab: number
  onSelectTab: (idx: number) => void
  activeAccount?: string | null
}

const TABS = [
  'Dashboard',
  'Fingerprints',
  'Settings',
]

export const TopRibbon: React.FC<TopRibbonProps> = ({
  currentTab,
  onSelectTab,
  activeAccount,
}) => {
  return (
    <header
      style={{
        height: '72px',
        backgroundColor: '#ffffff',
        borderBottom: '1px solid var(--border)',
        boxSizing: 'border-box',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 24px',
        flexShrink: 0,
      }}
    >
      {/* Google AI Studio Segmented Pill Track */}
      <div
        style={{
          display: 'flex',
          backgroundColor: 'var(--tonal)',
          borderRadius: '8px',
          padding: '3px',
          gap: '2px',
        }}
      >
        {TABS.map((tab, idx) => {
          const isActive = currentTab === idx
          return (
            <button
              key={tab}
              onClick={() => onSelectTab(idx)}
              style={{
                borderRadius: '6px',
                padding: '6px 16px',
                fontSize: '12px',
                fontWeight: isActive ? 600 : 500,
                color: isActive ? 'var(--primary)' : 'var(--text-muted)',
                backgroundColor: isActive ? '#ffffff' : 'transparent',
                boxShadow: isActive ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                border: 'none',
              }}
            >
              {tab}
            </button>
          )
        })}
      </div>

      {/* Active account as plain text at the right end of the top tab switcher bar */}
      {activeAccount ? (
        <span
          style={{
            display: 'inline-block',
            fontSize: '13px',
            fontWeight: 500,
            color: 'var(--primary)',
            border: '1px solid var(--primary)',
            borderRadius: '6px',
            padding: '4px 10px',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            maxWidth: '320px',
          }}
          title={`Active runtime account: ${activeAccount}`}
        >
          Current Account: {activeAccount}
        </span>
      ) : (
        <div
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: '6px',
            fontSize: '12px',
            fontWeight: 500,
            color: 'var(--text-muted)',
            backgroundColor: 'var(--tonal)',
            padding: '6px 14px',
            borderRadius: '16px',
          }}
        >
          <span>No Active Account</span>
        </div>
      )}
    </header>
  )
}
