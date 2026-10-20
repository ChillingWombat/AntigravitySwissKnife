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
    assert.ok(content.includes('<KeyRound'))
    assert.ok(content.includes('<Bot'))
    assert.ok(content.includes('<Layers'))
    assert.ok(content.includes('<Terminal'))
  })

  it('implements zero-setup remote tunneling controls without decorative tags', () => {
    assert.ok(content.includes('api.startBridgeTunnel'))
    assert.ok(content.includes('api.stopBridgeTunnel'))
    assert.ok(content.includes('Cloudflare Quick Tunnel'))
    assert.ok(content.includes('Tailscale Funnel'))
    assert.ok(content.includes('Custom Domain'))
    assert.strictEqual(content.includes('Quick Tunnel (Recommended)'), false)
    assert.strictEqual(content.includes('Tailscale Network'), false)
    assert.strictEqual(content.includes('Custom HTTPS Tunnel'), false)
    assert.strictEqual(content.includes('Zero Signup'), false)
    assert.strictEqual(content.includes('badge-chip badge-blue'), false)
    assert.strictEqual(content.includes('Reverse Proxy'), false)
  })

  it('uses tunnel lifecycle as the only operational bridge control', () => {
    assert.ok(content.includes('Verify Bridge'))
    assert.ok(content.includes('Gateway Status'))
    assert.ok(content.includes('Operating Mode'))
    assert.ok(content.includes('Remote Internet Pipe'))
    assert.strictEqual(content.includes('Enable Bridge'), false)
    assert.strictEqual(content.includes('handleToggleEnable'), false)
    assert.strictEqual(content.includes('api.updateBridgeConfig'), false)
    assert.strictEqual(content.includes('Desktop Surface'), false)
    assert.strictEqual(content.includes('Account Shield Active'), false)
  })

  it('reports verification failures and does not render fabricated credentials', () => {
    assert.ok(content.includes('isError: !res?.success'))
    assert.strictEqual(content.includes("'582914'"), false)
    assert.strictEqual(content.includes("'swiss-gemini-auth-token'"), false)
  })

  it('provides pairing PIN and Bearer token display with rotation', () => {
    assert.ok(content.includes('api.rotateBridgeToken'))
    assert.ok(content.includes('Pairing PIN'))
    assert.ok(content.includes('status?.pairing_pin'))
    assert.ok(content.includes('status?.auth_token'))
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
})
