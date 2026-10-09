import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  ACP_NODE_IDS,
  ACP_NODE_ICONS,
  ACP_CARD_LAYOUT_TOKENS,
  isAcpGridAligned,
  hasDecorativeEmojis,
  getAgentNodeIconName,
  getAcpStatusPresentation,
} from './acpPresentation.ts'

describe('ACP Presentation & Layout Specifications (Milestone 7)', () => {
  it('contains all 9 canonical ACP mesh node IDs', () => {
    assert.equal(ACP_NODE_IDS.length, 9)
    const expected = [
      'agent-antigravity',
      'agent-antigravity-cli',
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

  it('verifies that all 9 node IDs map to valid Lucide icons', () => {
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
    assert.equal(getAgentNodeIconName('agent-devin'), 'Workflow')
    assert.equal(getAgentNodeIconName('agent-opencode'), 'Code2')
    assert.equal(getAgentNodeIconName('agent-deepseek-harness'), 'Cpu')
    assert.equal(getAgentNodeIconName('agent-pi'), 'Radio')
    assert.equal(getAgentNodeIconName('agent-codex'), 'TerminalSquare')
    assert.equal(getAgentNodeIconName('agent-claude-code'), 'Bot')
    assert.equal(getAgentNodeIconName('agent-cursor'), 'Edit3')
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
})
