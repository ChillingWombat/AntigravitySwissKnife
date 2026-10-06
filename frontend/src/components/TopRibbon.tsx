import React from 'react'
import { CheckCircle2 } from 'lucide-react'

interface TopRibbonProps {
  currentTab: number
  onSelectTab: (idx: number) => void
  activeAccount?: string | null
}

const TABS = [
  'Dashboard',
  'Accounts & MFA',
  'Fingerprints',
  'Cache Manager',
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
          borderRadius: '20px',
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
                borderRadius: '16px',
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

      {/* Active Account Pill at the right end of the top tab switcher bar */}
      {activeAccount ? (
        <div
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: '7px',
            fontSize: '12px',
            fontWeight: 600,
            color: 'var(--primary)',
            backgroundColor: 'rgba(26, 115, 232, 0.08)',
            padding: '6px 14px',
            borderRadius: '16px',
            border: '1px solid rgba(26, 115, 232, 0.22)',
            maxWidth: '320px',
          }}
          title={`Active runtime account: ${activeAccount}`}
        >
          <CheckCircle2 size={14} color="#1a73e8" style={{ flexShrink: 0 }} />
          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            Active: {activeAccount}
          </span>
        </div>
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
