import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

describe('Gemini Bridge Remote Tunneling, Auth & Modes Contract', () => {
  const __dirname = path.dirname(fileURLToPath(import.meta.url))
  const pagePath = path.resolve(__dirname, '../pages/GeminiBridgePage.tsx')
  const content = fs.readFileSync(pagePath, 'utf8')

  it('strictly contains 0 decorative unicode emojis', () => {
    const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
    assert.strictEqual(
      emojiRegex.test(content),
      false,
      'GeminiBridgePage must not contain decorative emojis'
    )
  })

  it('exclusively utilizes Lucide icons for UI symbols', () => {
    assert.ok(content.includes("from 'lucide-react'"))
    assert.ok(content.includes('<Radio'))
    assert.ok(content.includes('<Bot'))
    assert.ok(content.includes('<Layers'))
    assert.ok(content.includes('<Terminal'))
  })

  it('implements zero-setup remote tunneling controls without decorative tags', () => {
    assert.ok(content.includes('Access Mode'))
    assert.strictEqual(content.includes('Remote Internet Access'), false)
    assert.strictEqual(content.includes('Publishes the local port 8765 MCP endpoint over public HTTPS'), false)
    assert.ok(content.includes('api.startBridgeTunnel'))
    assert.ok(content.includes('api.stopBridgeTunnel'))
    assert.ok(content.includes('Cloudflare Quick Tunnel'))
    assert.ok(content.includes('Tailscale Funnel'))
    assert.ok(content.includes('Custom Domain'))
    assert.strictEqual(content.includes('Tailscale Private Tailnet'), false)
    assert.strictEqual(content.includes('Quick Tunnel (Recommended)'), false)
    assert.strictEqual(content.includes('Tailscale Network'), false)
    assert.strictEqual(content.includes('Custom HTTPS Tunnel'), false)
    assert.strictEqual(content.includes('Zero Signup'), false)
    assert.strictEqual(content.includes('badge-chip badge-blue'), false)
    assert.strictEqual(content.includes('Reverse Proxy'), false)
  })

  it('uses tunnel lifecycle as the only operational bridge control', () => {
    assert.ok(content.includes('Enable Remote Access'))
    assert.ok(content.includes('ariaLabel="Enable Remote Access"'))
    assert.ok(content.includes('Gateway Status'))
    assert.ok(content.includes('Operating Mode'))
    assert.strictEqual(content.includes('Verify Bridge'), false, 'verify button must be replaced by the header toggle')
    assert.strictEqual(content.includes('Remote Internet Pipe'), false, 'remote pipe metric card must be removed')
    assert.strictEqual(content.includes('Start Remote Access'), false, 'start tunnel button must be replaced by the header toggle')
    assert.strictEqual(content.includes('Stop Remote Access'), false, 'stop tunnel button must be replaced by the header toggle')
    assert.strictEqual(content.includes('Enable Bridge'), false)
    assert.strictEqual(content.includes('handleToggleEnable'), false)
    assert.strictEqual(content.includes('api.updateBridgeConfig'), false)
    assert.strictEqual(content.includes('Desktop Surface'), false)
    assert.strictEqual(content.includes('Account Shield Active'), false)
  })

  it('reports request failures and does not render fabricated credentials', () => {
    assert.ok(content.includes('isError: true'))
    assert.strictEqual(content.includes('api.testBridgeConnection'), false)
    assert.strictEqual(content.includes("'582914'"), false)
    assert.strictEqual(content.includes("'swiss-gemini-auth-token'"), false)
  })

  it('provides pairing PIN and Bearer token display with rotation', () => {
    assert.ok(content.includes('api.rotateBridgeToken'))
    assert.ok(content.includes('Pairing PIN'))
    assert.ok(content.includes('Use App Password'))
    assert.ok(content.includes('ToggleSwitch'))
    assert.ok(content.includes('Rotate PIN'))
    assert.strictEqual(content.includes('Copy PIN'), false, 'Copy PIN button must be removed from Pairing PIN gadget')
    assert.ok(content.includes('status?.pairing_pin'))
    assert.ok(content.includes('status?.auth_token'))
    assert.ok(content.includes("gridTemplateColumns: '2fr 1fr'"))

    const endpointIndex = content.indexOf('Active Endpoint')
    const pairingPinIndex = content.indexOf('Pairing PIN (Quick Setup)')
    const credentialsIndex = content.indexOf('Authentication & Pairing Credentials')
    assert.ok(endpointIndex >= 0)
    assert.ok(pairingPinIndex > endpointIndex, 'Pairing PIN must be next to Active Endpoint in Card 1')
    assert.ok(pairingPinIndex < credentialsIndex, 'Pairing PIN gadget must be removed from the credentials card')
  })

  it('renders OAuth 2.0 guidance and OAuth Client ID credentials', () => {
    assert.ok(content.includes('OAuth Client ID'))
    assert.ok(content.includes('antigravity-gemini-bridge'))
    assert.ok(content.includes('OAuth 2.0'))
    assert.ok(content.includes('status?.oauth_client_id'))
    assert.ok(content.includes('Copy Client ID'))
    assert.ok(content.includes("'AGY Bridge':"))
    assert.ok(content.includes('OAuth Client Secret / Bearer Token'))
    assert.ok(content.includes('Streamable HTTPS'))
  })

  it('provides mode switching between Hybrid, Lead/Messenger, and Direct Actuator', () => {
    assert.ok(content.includes('api.setBridgeMode'))
    assert.ok(content.includes('Hybrid Mode (Recommended)'))
    assert.ok(content.includes('Lead & Messenger Mode'))
    assert.ok(content.includes('Direct Actuator Mode'))
  })

  it('includes active Antigravity lead session monitor with response forwarding', () => {
    assert.ok(content.includes('active_lead_session'))
    assert.ok(content.includes('Forwarded Antigravity Response Preview'))
  })

  it('removes explanatory model context protocol tools gadget (deferred to documentation)', () => {
    assert.strictEqual(content.includes('Exposed Model Context Protocol Tools'), false)
    for (const tool of [
      'antigravity_lead_create_conversation',
      'antigravity_lead_compact_conversation',
      'antigravity_lead_stop_conversation',
      'antigravity_fleet_get_quota',
      'antigravity_fleet_switch_account',
      'local_search_files',
    ]) {
      assert.strictEqual(content.includes(tool), false, `${tool} should not be in explanatory UI gadget`)
    }

    assert.strictEqual(content.includes('Test Remote Task Dispatch'), false)
    assert.strictEqual(content.includes('Surface Connectivity Matrix'), false)
    assert.strictEqual(content.includes('How to direct Gemini in chat:'), false)
  })

  it('avoids banned AI buzzwords and enforces concise technical copy', () => {
    const bannedBuzzwords = ['supercharge', 'delve', 'paradigm shift', 'synergy', 'game-changer']
    for (const word of bannedBuzzwords) {
      assert.strictEqual(
        content.toLowerCase().includes(word),
        false,
        `GeminiBridgePage must not contain banned buzzword "${word}"`
      )
    }
  })

  it('provides Outbound HTTP Allowlist mode switch between Always Allow and Whitelist Only without slop subtitle', () => {
    assert.ok(content.includes('Outbound HTTP Allowlist'))
    assert.ok(content.includes('Always Allow'))
    assert.ok(content.includes('Whitelist Only'))
    assert.ok(content.includes('api.setBridgeHTTPAllowlistMode'))
    assert.ok(content.includes("allowlistMode === 'always_allow'"))
    assert.ok(content.includes('All outbound HTTP requests and domains are permitted without restriction.'))
    // Anti-slop check: remove the explanatory subtitle
    assert.strictEqual(
      content.includes('Agents can call external APIs (calendars, REST, webhooks) only on these hosts.'),
      false,
      'Explanatory subtitle under Outbound HTTP Allowlist header must be removed'
    )
  })
})
