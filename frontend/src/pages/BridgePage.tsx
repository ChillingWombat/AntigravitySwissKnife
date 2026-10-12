import React, { useState } from 'react'
import { RemoteControlPage } from './RemoteControlPage'
import { GeminiBridgePage } from './GeminiBridgePage'
import { WhatsAppBridgePage } from './WhatsAppBridgePage'
import { TelegramBridgePage } from './TelegramBridgePage'
import { DiscordBridgePage } from './DiscordBridgePage'

interface BridgePageProps {
  initialTab?: number
  activeTab?: number
  onTabChange?: (tab: number) => void
}

export const BridgePage: React.FC<BridgePageProps> = ({
  initialTab = 0,
  activeTab: controlledActiveTab,
  onTabChange: _onTabChange,
}) => {
  const [internalActiveTab] = useState<number>(initialTab)
  const [socialSubTab, setSocialSubTab] = useState<'telegram' | 'discord'>('telegram')
  const activeTab = controlledActiveTab !== undefined ? controlledActiveTab : internalActiveTab

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Tab 0: Antigravity Remote Control Wrapper (Account-Switch Resilient) */}
      {activeTab === 0 && <RemoteControlPage />}

      {/* Tab 1: Gemini Bridge (Two-Way Agent Streamable HTTP MCP) */}
      {activeTab === 1 && <GeminiBridgePage />}

      {/* Tab 2: WhatsApp Agent Chat Bridge */}
      {activeTab === 2 && <WhatsAppBridgePage />}

      {/* Tab 3: Telegram & Discord Agent Chat Bridges */}
      {activeTab === 3 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              backgroundColor: 'var(--tonal)',
              borderRadius: '8px',
              padding: '3px',
              gap: '2px',
              width: 'fit-content',
            }}
          >
            <button
              onClick={() => setSocialSubTab('telegram')}
              style={{
                borderRadius: '6px',
                padding: '6px 16px',
                fontSize: '12px',
                fontWeight: socialSubTab === 'telegram' ? 600 : 500,
                color: socialSubTab === 'telegram' ? 'var(--primary)' : 'var(--text-muted)',
                backgroundColor: socialSubTab === 'telegram' ? '#ffffff' : 'transparent',
                boxShadow: socialSubTab === 'telegram' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                border: 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                transition: 'all 0.15s ease',
              }}
            >
              Telegram Bridge
            </button>
            <button
              onClick={() => setSocialSubTab('discord')}
              style={{
                borderRadius: '6px',
                padding: '6px 16px',
                fontSize: '12px',
                fontWeight: socialSubTab === 'discord' ? 600 : 500,
                color: socialSubTab === 'discord' ? 'var(--primary)' : 'var(--text-muted)',
                backgroundColor: socialSubTab === 'discord' ? '#ffffff' : 'transparent',
                boxShadow: socialSubTab === 'discord' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                border: 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                transition: 'all 0.15s ease',
              }}
            >
              Discord Bridge
            </button>
          </div>
          {socialSubTab === 'telegram' ? <TelegramBridgePage /> : <DiscordBridgePage />}
        </div>
      )}
    </div>
  )
}

export default BridgePage
