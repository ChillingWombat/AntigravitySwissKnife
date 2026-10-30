import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  ACP_NODE_IDS,
  ACP_NODE_ICONS,
  ACP_PRODUCT_NAMES,
  ACP_CARD_LAYOUT_TOKENS,
  isAcpGridAligned,
  hasDecorativeEmojis,
  getAgentNodeIconName,
  getAcpStatusPresentation,
  getAcpStatusCategory,
  sortAcpAgents,
} from './acpPresentation.ts'

describe('ACP Presentation & Layout Specifications (Milestone 7)', () => {
  it('contains all 10 canonical ACP mesh node IDs', () => {
    assert.equal(ACP_NODE_IDS.length, 10)
    const expected = [
      'agent-antigravity',
      'agent-antigravity-cli',
      'agent-antigravity-vscode',
      'agent-devin',
      'agent-opencode',
      'agent-deepseek-harness',
      'agent-pi',
      'agent-codex',
      'agent-claude-code',
      'agent-cursor',
    ]
    for (const id of expected) {
      assert.ok(ACP_NODE_IDS.includes(id as any), `Missing expected node ID: ${id}`)
    }
  })

  it('verifies that all 10 node IDs map to valid Lucide icons', () => {
    const validLucideIcons = new Set([
      'Monitor',
      'Terminal',
      'Workflow',
      'Code2',
      'Cpu',
      'Radio',
      'TerminalSquare',
      'Bot',
      'Edit3',
      'Activity',
    ])

    for (const id of ACP_NODE_IDS) {
      const icon = ACP_NODE_ICONS[id]
      assert.ok(icon, `Node ${id} must have an icon defined`)
      assert.ok(validLucideIcons.has(icon), `Node ${id} icon '${icon}' must be an approved Lucide icon`)
      assert.equal(getAgentNodeIconName(id), icon)
    }

    // Specific mapping verification
    assert.equal(getAgentNodeIconName('agent-antigravity'), 'Monitor')
    assert.equal(getAgentNodeIconName('agent-antigravity-cli'), 'Terminal')
    assert.equal(getAgentNodeIconName('agent-antigravity-vscode'), 'Code2')
    assert.equal(getAgentNodeIconName('agent-devin'), 'Workflow')
    assert.equal(getAgentNodeIconName('agent-opencode'), 'Code2')
    assert.equal(getAgentNodeIconName('agent-deepseek-harness'), 'Cpu')
    assert.equal(getAgentNodeIconName('agent-pi'), 'Radio')
    assert.equal(getAgentNodeIconName('agent-codex'), 'TerminalSquare')
    assert.equal(getAgentNodeIconName('agent-claude-code'), 'Bot')
    assert.equal(getAgentNodeIconName('agent-cursor'), 'Edit3')
    assert.equal(ACP_PRODUCT_NAMES['agent-antigravity-vscode'], 'Antigravity VS Code Extension')
  })

  it('enforces 4px grid geometry on card layout tokens (David-Design)', () => {
    assert.equal(ACP_CARD_LAYOUT_TOKENS.cardPadding, '16px') // 4 * 4px
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.cardPadding))

    assert.equal(ACP_CARD_LAYOUT_TOKENS.cardBorderRadius, '8px') // 2 * 4px
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.cardBorderRadius))

    assert.equal(ACP_CARD_LAYOUT_TOKENS.headerMarginBottom, '8px') // 2 * 4px
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.headerMarginBottom))

    assert.equal(ACP_CARD_LAYOUT_TOKENS.avatarSize, '28px') // 7 * 4px
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.avatarSize))

    assert.equal(ACP_CARD_LAYOUT_TOKENS.bodyGap, '4px') // 1 * 4px
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.bodyGap))

    assert.equal(ACP_CARD_LAYOUT_TOKENS.tagGap, '4px') // 1 * 4px
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.tagGap))

    assert.equal(ACP_CARD_LAYOUT_TOKENS.actionButtonHeight, '28px') // 7 * 4px
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.actionButtonHeight))
  })

  it('mandates whiteSpace: nowrap on action buttons and status badges', () => {
    assert.equal(ACP_CARD_LAYOUT_TOKENS.whiteSpace, 'nowrap')

    const statuses = ['active_hosting', 'connected', 'listening', 'idle', 'unreachable']
    for (const status of statuses) {
      const pres = getAcpStatusPresentation(status)
      assert.equal(pres.whiteSpace, 'nowrap')
      assert.ok(pres.label.length > 0)
      assert.ok(!hasDecorativeEmojis(pres.label))
    }
  })

  it('enforces zero decorative emojis across all node names and presentation metadata', () => {
    for (const id of ACP_NODE_IDS) {
      assert.ok(!hasDecorativeEmojis(id), `Node ID ${id} must not contain emojis`)
      const icon = ACP_NODE_ICONS[id]
      assert.ok(!hasDecorativeEmojis(icon), `Icon ${icon} must not contain emojis`)
    }

    assert.ok(hasDecorativeEmojis('🚀 Rocket') === true)
    assert.ok(hasDecorativeEmojis('Ping All ACP Nodes') === false)
    assert.ok(hasDecorativeEmojis('Google Antigravity 2.0') === false)
    assert.ok(hasDecorativeEmojis('Devin') === false)
  })

  it('confirms Windsurf Cascade is renamed to Devin and legacy ID is not in canonical list', () => {
    assert.ok(!ACP_NODE_IDS.includes('agent-windsurf' as any))
    assert.ok(ACP_NODE_IDS.includes('agent-devin'))
    assert.equal(getAgentNodeIconName('agent-devin'), 'Workflow')
  })

  it('correctly categorizes ACP statuses into connected > installed > notfound', () => {
    assert.equal(getAcpStatusCategory('connected'), 'connected')
    assert.equal(getAcpStatusCategory('active_hosting'), 'connected')
    assert.equal(getAcpStatusCategory('listening'), 'connected')

    assert.equal(getAcpStatusCategory('installed_idle'), 'installed')
    assert.equal(getAcpStatusCategory('installed'), 'installed')

    assert.equal(getAcpStatusCategory('not_found'), 'notfound')
    assert.equal(getAcpStatusCategory('unreachable'), 'notfound')
    assert.equal(getAcpStatusCategory('idle'), 'notfound')
    assert.equal(getAcpStatusCategory(''), 'notfound')
    assert.equal(getAcpStatusCategory('unknown_xyz'), 'notfound')
  })

  it('sorts ACP agent cards in the order of connected > installed > notfound, then alphabetically', () => {
    const fleet = [
      { id: 'agent-devin', name: 'Devin', status: 'not_found' },
      { id: 'agent-opencode', name: 'OpenCode', status: 'not_found' },
      { id: 'agent-deepseek-harness', name: 'DeepSeek Harness', status: 'not_found' },
      { id: 'agent-pi', name: 'Pi', status: 'connected' },
      { id: 'agent-codex', name: 'Codex', status: 'installed_idle' },
      { id: 'agent-claude-code', name: 'Claude Code CLI', status: 'not_found' },
      { id: 'agent-cursor', name: 'Cursor Editor Agent', status: 'not_found' },
    ]

    const sorted = sortAcpAgents(fleet)
    const sortedNames = sorted.map((a) => a.name)

    assert.deepEqual(sortedNames, [
      // 1. Connected
      'Pi',
      // 2. Installed
      'Codex',
      // 3. Not Found (alphabetical)
      'Claude Code CLI',
      'Cursor Editor Agent',
      'DeepSeek Harness',
      'Devin',
      'OpenCode',
    ])
  })

  it('sorts multiple agents alphabetically within the same status category', () => {
    const agents = [
      { id: '1', name: 'Z-Connected', status: 'connected' },
      { id: '2', name: 'A-Connected', status: 'connected' },
      { id: '3', name: 'Z-Installed', status: 'installed_idle' },
      { id: '4', name: 'A-Installed', status: 'installed_idle' },
      { id: '5', name: 'Z-NotFound', status: 'not_found' },
      { id: '6', name: 'A-NotFound', status: 'not_found' },
    ]

    const sorted = sortAcpAgents(agents)
    assert.deepEqual(sorted.map((a) => a.name), [
      'A-Connected',
      'Z-Connected',
      'A-Installed',
      'Z-Installed',
      'A-NotFound',
      'Z-NotFound',
    ])
  })
})
