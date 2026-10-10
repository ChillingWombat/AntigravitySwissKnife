/**
 * ACP (Agent Client Protocol) Presentation & Layout Utility
 * Adheres strictly to David-Design (4px grid, single-line action text nowrap, zero decorative emojis).
 */

export const ACP_NODE_IDS = [
  'agent-antigravity',
  'agent-antigravity-cli',
  'agent-devin',
  'agent-opencode',
  'agent-deepseek-harness',
  'agent-pi',
  'agent-codex',
  'agent-claude-code',
  'agent-cursor',
] as const

export type AcpNodeId = typeof ACP_NODE_IDS[number]

export const ACP_NODE_ICONS: Record<AcpNodeId, string> = {
  'agent-antigravity': 'Monitor',
  'agent-antigravity-cli': 'Terminal',
  'agent-devin': 'Workflow',
  'agent-opencode': 'Code2',
  'agent-deepseek-harness': 'Cpu',
  'agent-pi': 'Radio',
  'agent-codex': 'TerminalSquare',
  'agent-claude-code': 'Bot',
  'agent-cursor': 'Edit3',
}

export const ACP_PRODUCT_NAMES: Record<AcpNodeId, string> = {
  'agent-antigravity': 'Antigravity 2.0',
  'agent-antigravity-cli': 'Antigravity CLI',
  'agent-devin': 'Devin',
  'agent-opencode': 'OpenCode',
  'agent-deepseek-harness': 'DeepSeek Harness',
  'agent-pi': 'Pi',
  'agent-codex': 'Codex',
  'agent-claude-code': 'Claude Code',
  'agent-cursor': 'Cursor',
}

export const ACP_CARD_LAYOUT_TOKENS = {
  cardPadding: '16px', // 4 * 4px
  cardBorderRadius: '8px', // 2 * 4px
  cardBorderWidth: '1px',
  cardShadow: '0 1px 2px rgba(0,0,0,0.04)',
  headerMarginBottom: '8px', // 2 * 4px
  avatarSize: '28px', // 7 * 4px
  avatarBorderRadius: '6px',
  bodyGap: '4px', // 1 * 4px
  tagGap: '4px', // 1 * 4px
  actionButtonHeight: '28px', // 7 * 4px
  actionButtonPadding: '4px 10px',
  actionButtonFontSize: '11.5px',
  actionButtonBorderRadius: '6px',
  whiteSpace: 'nowrap' as const,
}

export function isAcpGridAligned(valPx: number | string): boolean {
  const num = typeof valPx === 'number' ? valPx : parseInt(valPx, 10)
  return !isNaN(num) && num % 4 === 0
}

export function hasDecorativeEmojis(text: string): boolean {
  const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}\u{1FA00}-\u{1FAFF}]/u
  return emojiRegex.test(text)
}

export function getAgentNodeIconName(id: string): string {
  if (id in ACP_NODE_ICONS) {
    return ACP_NODE_ICONS[id as AcpNodeId]
  }
  return 'Bot'
}

export function getAcpStatusPresentation(status: string) {
  switch (status) {
    case 'active_hosting':
      return {
        bg: '#e6f4ea',
        color: '#137333',
        label: 'ACTIVE HOSTING',
        whiteSpace: 'nowrap' as const,
      }
    case 'connected':
      return {
        bg: '#e8f0fe',
        color: 'var(--primary)',
        label: 'CONNECTED',
        whiteSpace: 'nowrap' as const,
      }
    case 'listening':
      return {
        bg: '#fef7e0',
        color: '#b06000',
        label: 'LISTENING',
        whiteSpace: 'nowrap' as const,
      }
    case 'idle':
      return {
        bg: '#f1f3f4',
        color: '#5f6368',
        label: 'IDLE',
        whiteSpace: 'nowrap' as const,
      }
    case 'unreachable':
    default:
      return {
        bg: '#fce8e6',
        color: '#d93025',
        label: 'UNREACHABLE',
        whiteSpace: 'nowrap' as const,
      }
  }
}
