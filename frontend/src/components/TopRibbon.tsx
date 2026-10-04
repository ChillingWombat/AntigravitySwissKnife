import React from 'react'
import { User } from 'lucide-react'

interface TopRibbonProps {
  currentTab: number
  onSelectTab: (idx: number) => void
  activeAccount: string | null
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
        height: '56px',
        backgroundColor: '#ffffff',
        borderBottom: '1px solid var(--border)',
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

      {/* Google User Identity Chip on Right */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          backgroundColor: '#e8f0fe',
          border: '1px solid var(--primary-container)',
          borderRadius: '16px',
          padding: '6px 16px',
          fontSize: '12px',
          fontWeight: 600,
          color: 'var(--primary)',
        }}
      >
        <User size={14} color="var(--primary)" />
        <span>{activeAccount || 'No Active Account'}</span>
      </div>
    </header>
  )
}
