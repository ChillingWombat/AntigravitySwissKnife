import React from 'react'
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
  const activeTab = controlledActiveTab !== undefined ? controlledActiveTab : initialTab

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Tab 0: Antigravity Remote Control Wrapper (Account-Switch Resilient) */}
      {activeTab === 0 && <RemoteControlPage />}

      {/* Tab 1: Gemini Bridge (Two-Way Agent Streamable HTTP MCP) */}
      {activeTab === 1 && <GeminiBridgePage />}

      {/* Tab 2: WhatsApp Agent Chat Bridge */}
      {activeTab === 2 && <WhatsAppBridgePage />}

      {/* Tab 3: Telegram Agent Chat Bridge */}
      {activeTab === 3 && <TelegramBridgePage />}

      {/* Tab 4: Discord Agent Chat Bridge */}
      {activeTab === 4 && <DiscordBridgePage />}
    </div>
  )
}

export default BridgePage
