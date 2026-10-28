import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {
  ACP_NODE_IDS,
  ACP_PRODUCT_NAMES,
  ACP_CARD_LAYOUT_TOKENS,
  isAcpGridAligned,
  hasDecorativeEmojis,
  getAcpStatusPresentation,
} from './acpPresentation.ts'

describe('ACP Agent Mesh & Official Brand SVG Icons (Requirement 3.1)', () => {
  const rootSrcDir = path.resolve(import.meta.dirname, '..')
  const acpPagePath = path.join(rootSrcDir, 'pages', 'ACPAgentMeshPage.tsx')
  const appPath = path.join(rootSrcDir, 'App.tsx')
  const brandIconsPath = path.join(rootSrcDir, 'components', 'AcpBrandIcons.tsx')

  it('verifies all 10 canonical ACP mesh node IDs have official product names and IDs', () => {
    assert.equal(ACP_NODE_IDS.length, 10)

    const expectedNodes = [
      { id: 'agent-antigravity', name: 'Antigravity 2.0', compName: 'AntigravityLogoIcon' },
      { id: 'agent-antigravity-cli', name: 'Antigravity CLI', compName: 'AntigravityCliIcon' },
      { id: 'agent-antigravity-vscode', name: 'Antigravity VS Code Extension', compName: 'AntigravityVSCodeIcon' },
      { id: 'agent-devin', name: 'Devin', compName: 'DevinLogoIcon' },
      { id: 'agent-opencode', name: 'OpenCode', compName: 'OpenCodeLogoIcon' },
      { id: 'agent-deepseek-harness', name: 'DeepSeek Harness', compName: 'DeepSeekLogoIcon' },
      { id: 'agent-pi', name: 'Pi', compName: 'PiLogoIcon' },
      { id: 'agent-codex', name: 'Codex', compName: 'CodexLogoIcon' },
      { id: 'agent-claude-code', name: 'Claude Code', compName: 'ClaudeCodeLogoIcon' },
      { id: 'agent-cursor', name: 'Cursor', compName: 'CursorLogoIcon' },
    ]

    const brandContent = fs.readFileSync(brandIconsPath, 'utf8')

    for (const node of expectedNodes) {
      assert.ok(ACP_NODE_IDS.includes(node.id as any), `Missing node ID in ACP_NODE_IDS: ${node.id}`)
      assert.equal(ACP_PRODUCT_NAMES[node.id as keyof typeof ACP_PRODUCT_NAMES], node.name)
      assert.ok(
        brandContent.includes(`export const ${node.compName}`),
        `AcpBrandIcons.tsx must export component ${node.compName} for ${node.name}`
      )
      assert.ok(
        brandContent.includes(`case '${node.id}':`),
        `getAcpBrandIcon must handle node ID '${node.id}' in AcpBrandIcons.tsx`
      )
    }
  })

  it('verifies AcpBrandIcons.tsx contains official brand logos with zero emojis', () => {
    const brandContent = fs.readFileSync(brandIconsPath, 'utf8')
    assert.ok(!hasDecorativeEmojis(brandContent), 'AcpBrandIcons.tsx must not contain emojis')
    assert.ok(brandContent.includes('assets/logo/'), 'AcpBrandIcons.tsx must reference real logos from assets folder')
    assert.ok(brandContent.includes('AntigravityLogoIcon'))
    assert.ok(brandContent.includes('AntigravityCliIcon'))
    assert.ok(brandContent.includes('AntigravityVSCodeIcon'))
    assert.ok(brandContent.includes('DevinLogoIcon'))
    assert.ok(brandContent.includes('OpenCodeLogoIcon'))
    assert.ok(brandContent.includes('DeepSeekLogoIcon'))
    assert.ok(brandContent.includes('PiLogoIcon'))
    assert.ok(brandContent.includes('CodexLogoIcon'))
    assert.ok(brandContent.includes('ClaudeCodeLogoIcon'))
    assert.ok(brandContent.includes('CursorLogoIcon'))
    assert.ok(brandContent.includes('GoogleCloudLogoIcon'))
    assert.ok(brandContent.includes('getAcpBrandIcon'))
    assert.ok(brandContent.includes('default:'))
  })

  it('verifies ACPAgentMeshPage.tsx adheres to David-Design (4px grid, single-line text nowrap, 0 emojis)', () => {
    assert.ok(fs.existsSync(acpPagePath), 'ACPAgentMeshPage.tsx must exist')
    const pageContent = fs.readFileSync(acpPagePath, 'utf8')

    // Zero decorative emojis
    assert.ok(!hasDecorativeEmojis(pageContent), 'ACPAgentMeshPage.tsx must not contain emojis')

    // Lucide icons
    assert.ok(pageContent.includes("from 'lucide-react'"), 'ACPAgentMeshPage must import Lucide icons')
    assert.ok(pageContent.includes('getAcpBrandIcon'), 'ACPAgentMeshPage must use getAcpBrandIcon for agent avatars')

    // Single-line action labels (white-space: nowrap)
    assert.ok(pageContent.includes('whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace'))
    assert.ok(pageContent.includes('Ping'))

    // Real API integration
    assert.ok(pageContent.includes('api.getAcpMesh()'), 'ACPAgentMeshPage must query real api.getAcpMesh')

    // 4px grid tokens
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.cardPadding))
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.cardBorderRadius))
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.headerMarginBottom))
    assert.ok(isAcpGridAligned(ACP_CARD_LAYOUT_TOKENS.avatarSize))
  })

  it('verifies App.tsx renders ACPAgentMeshPage for tool 11', () => {
    assert.ok(fs.existsSync(appPath), 'App.tsx must exist')
    const appContent = fs.readFileSync(appPath, 'utf8')

    assert.ok(
      appContent.includes("import { ACPAgentMeshPage } from './pages/ACPAgentMeshPage'"),
      'App must import ACPAgentMeshPage'
    )
    assert.ok(
      appContent.includes('currentTool === 11') && appContent.includes('<ACPAgentMeshPage />'),
      'App must render <ACPAgentMeshPage /> for tool 11'
    )
  })

  it('validates status badge presentation conforms to white-space: nowrap with zero emojis', () => {
    const statuses = ['active_hosting', 'connected', 'listening', 'idle', 'unreachable']
    for (const st of statuses) {
      const pres = getAcpStatusPresentation(st)
      assert.equal(pres.whiteSpace, 'nowrap')
      assert.ok(!hasDecorativeEmojis(pres.label))
    }
  })
})
