import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

describe('Bridge Section & Multi-Channel Remote Chat Invariants', () => {
  const __dirname = path.dirname(fileURLToPath(import.meta.url))
  const navRailPath = path.resolve(__dirname, '../components/NavRail.tsx')
  const appPath = path.resolve(__dirname, '../App.tsx')
  const utilsPagePath = path.resolve(__dirname, '../pages/UtilitiesPage.tsx')
  const bridgePagePath = path.resolve(__dirname, '../pages/BridgePage.tsx')
  const remotePagePath = path.resolve(__dirname, '../pages/RemoteControlPage.tsx')
  const geminiPagePath = path.resolve(__dirname, '../pages/GeminiBridgePage.tsx')
  const waPagePath = path.resolve(__dirname, '../pages/WhatsAppBridgePage.tsx')
  const tgPagePath = path.resolve(__dirname, '../pages/TelegramBridgePage.tsx')
  const dsPagePath = path.resolve(__dirname, '../pages/DiscordBridgePage.tsx')

  const navContent = fs.readFileSync(navRailPath, 'utf8')
  const appContent = fs.readFileSync(appPath, 'utf8')
  const utilsContent = fs.readFileSync(utilsPagePath, 'utf8')
  const bridgeContent = fs.readFileSync(bridgePagePath, 'utf8')
  const remoteContent = fs.readFileSync(remotePagePath, 'utf8')
  const geminiContent = fs.readFileSync(geminiPagePath, 'utf8')
  const waContent = fs.readFileSync(waPagePath, 'utf8')
  const tgContent = fs.readFileSync(tgPagePath, 'utf8')
  const dsContent = fs.readFileSync(dsPagePath, 'utf8')

  const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u

  it('NavRail contains dedicated Remote Connect section button with Network Lucide icon', () => {
    assert.ok(navContent.includes('id="btnNavBridge"'))
    assert.ok(navContent.includes('onSelectTool(10)'))
    assert.ok(navContent.includes('<Network size={18}'))
    assert.ok(navContent.includes('Remote Connect'))
  })

  it('App.tsx contains 5 segmented tabs for Bridge section in exact required order', () => {
    assert.ok(appContent.includes("currentTool === 10"))
    assert.ok(appContent.includes("['Remote', 'Gemini', 'WhatsApp', 'Telegram', 'Discord']"))
    assert.ok(appContent.includes('<BridgePage'))
  })

  it('UtilitiesPage contains 3 tabs: Storage Manager, ACP Agent Mesh, Chat & Project Importer', () => {
    assert.ok(appContent.includes("['Storage Manager', 'ACP Agent Mesh', 'Chat & Project Importer']"))
    assert.ok(!utilsContent.includes('GeminiBridgePage'))
    assert.ok(utilsContent.includes('<BrainCachePage />'))
    assert.ok(utilsContent.includes('<ACPAgentMeshPage />'))
  })

  it('BridgePage integrates all 5 sub-pages', () => {
    assert.ok(bridgeContent.includes('<RemoteControlPage />'))
    assert.ok(bridgeContent.includes('<GeminiBridgePage />'))
    assert.ok(bridgeContent.includes('<WhatsAppBridgePage />'))
    assert.ok(bridgeContent.includes('<TelegramBridgePage />'))
    assert.ok(bridgeContent.includes('<DiscordBridgePage />'))
  })

  it('RemoteControlPage uses one operational remote-access control', () => {
    assert.strictEqual(emojiRegex.test(remoteContent), false, 'RemoteControlPage must not contain emojis')
    assert.ok(remoteContent.includes('api.getRemoteControlStatus'))
    assert.ok(remoteContent.includes('api.updateRemoteControlConfig'))
    assert.ok(remoteContent.includes('api.rotateRemotePIN'))
    assert.ok(remoteContent.includes('Security PIN'))
    assert.ok(remoteContent.includes('DevTools Port'))
    assert.ok(remoteContent.includes('google-card'))
    assert.ok(remoteContent.includes('Start Remote Access'))
    assert.strictEqual(remoteContent.includes('Enable Remote'), false, 'duplicate top-level enable toggle must be removed')
    assert.strictEqual(remoteContent.includes('Account-Switch Resilience Mode'), false, 'no-op resilience toggle must be removed')
    assert.ok(remoteContent.includes('tunnel runs in the Swiss daemon'))
    assert.strictEqual(remoteContent.includes('Host Antigravity PID'), false, 'developer PID gadget must be removed')
    assert.strictEqual(remoteContent.includes('Survives account rotations'), false, 'duplicate resilience gadget must be removed')
    assert.ok(remoteContent.includes('Remote Endpoint'))
    assert.strictEqual(remoteContent.includes('badge-chip'), false, 'RemoteControlPage tunnel cards must not contain badge tags')
    assert.strictEqual(remoteContent.includes('(Recommended)'), false, 'RemoteControlPage must not contain (Recommended) text')
  })

  it('WhatsAppBridgePage enforces David-Design and functional settings', () => {
    assert.strictEqual(emojiRegex.test(waContent), false, 'WhatsAppBridgePage must not contain emojis')
    assert.ok(waContent.includes('WhatsApp Agent Chat Bridge'))
    assert.ok(waContent.includes('api.getChannelsStatus'))
    assert.ok(waContent.includes('api.updateChannelsConfig'))
    assert.ok(waContent.includes("api.testChannel('whatsapp')"))
    assert.ok(waContent.includes('Allowed Phone Numbers'))
    assert.ok(waContent.includes('google-card'))
    assert.ok(waContent.includes('ToggleSwitch'))
    // Issue #119: the header status chip is removed — state lives in the toggle and status cards.
    assert.strictEqual(waContent.includes('badge-chip'), false, 'WhatsAppBridgePage header chip must be removed')
  })

  it('TelegramBridgePage enforces David-Design and functional settings', () => {
    assert.strictEqual(emojiRegex.test(tgContent), false, 'TelegramBridgePage must not contain emojis')
    assert.ok(tgContent.includes('Telegram Agent Chat Bridge'))
    assert.ok(tgContent.includes('api.getChannelsStatus'))
    assert.ok(tgContent.includes('Bot Token (from @BotFather)'))
    assert.ok(tgContent.includes("api.testChannel('telegram')"))
    assert.ok(tgContent.includes('Allowed Chat IDs'))
    assert.ok(tgContent.includes('google-card'))
    assert.ok(tgContent.includes('ToggleSwitch'))
    // Issue #119: the header status chip is removed — state lives in the toggle and status cards.
    assert.strictEqual(tgContent.includes('badge-chip'), false, 'TelegramBridgePage header chip must be removed')
  })

  it('DiscordBridgePage enforces David-Design and functional settings', () => {
    assert.strictEqual(emojiRegex.test(dsContent), false, 'DiscordBridgePage must not contain emojis')
    assert.ok(dsContent.includes('Discord Agent Chat Bridge'))
    assert.ok(dsContent.includes('Discord Bot Token'))
    assert.ok(dsContent.includes('Application ID'))
    assert.ok(dsContent.includes("api.testChannel('discord')"))
    assert.ok(dsContent.includes('Allowed Channel IDs'))
    assert.ok(dsContent.includes('google-card'))
    assert.ok(dsContent.includes('ToggleSwitch'))
    // Issue #119: the header status chip is removed — state lives in the toggle and status cards.
    assert.strictEqual(dsContent.includes('badge-chip'), false, 'DiscordBridgePage header chip must be removed')
  })

  it('GeminiBridgePage enforces David-Design and functional settings', () => {
    assert.strictEqual(emojiRegex.test(geminiContent), false, 'GeminiBridgePage must not contain emojis')
    assert.ok(geminiContent.includes('Gemini Agent Chat Bridge'))
    assert.ok(geminiContent.includes('api.getBridgeStatus'))
    assert.ok(geminiContent.includes('api.testBridgeConnection'))
    assert.ok(geminiContent.includes('google-card'))
    assert.ok(geminiContent.includes('badge-chip'))
    assert.strictEqual(geminiContent.includes('Enable Bridge'), false, 'duplicate top-level enable toggle must be removed')
  })

  it('RemoteControlPage includes tunnel diagnostics log viewer and action feedback banner', () => {
    assert.ok(remoteContent.includes('View Tunnel Diagnostics & Logs'), 'must contain tunnel diagnostics log viewer trigger')
    assert.ok(remoteContent.includes('Hide Connection Logs'), 'must contain hide log toggle text')
    assert.ok(remoteContent.includes('showTunnelLogs'), 'must have state for tunnel logs')
    assert.ok(remoteContent.includes('actionFeedback'), 'must have state for action feedback')
    assert.ok(remoteContent.includes('actionFeedback.text'), 'must render actionFeedback text')
    assert.ok(remoteContent.includes('status?.tunnel?.logs'), 'must display tunnel logs from status')
  })

  it('WhatsAppBridgePage, TelegramBridgePage, and DiscordBridgePage update status on test and fetch unredacted config', () => {
    for (const { name, content } of [
      { name: 'WhatsAppBridgePage', content: waContent },
      { name: 'TelegramBridgePage', content: tgContent },
      { name: 'DiscordBridgePage', content: dsContent },
    ]) {
      assert.ok(
        content.includes('api.getChannelsConfig(true)'),
        `${name} must fetch unredacted config with getChannelsConfig(true)`
      )
      assert.ok(
        content.includes('setStatus(res.status)'),
        `${name} must update status with setStatus(res.status) on test / save`
      )
    }
  })

  it('GeminiBridgePage merges rotated credentials and seeds custom tunnel URL', () => {
    assert.ok(geminiContent.includes('res.auth_token || res.status?.auth_token'))
    assert.ok(geminiContent.includes('res.pairing_pin || res.status?.pairing_pin'))
    assert.ok(geminiContent.includes('setCustomTunnelURL((prev) => prev || res.tunnel.gateway_url)'))
  })

  it('all 5 bridge pages eliminate banned AI buzzwords (utilize, supercharge, paradigm shift, delve)', () => {
    const pages = [
      { name: 'RemoteControlPage', content: remoteContent },
      { name: 'GeminiBridgePage', content: geminiContent },
      { name: 'WhatsAppBridgePage', content: waContent },
      { name: 'TelegramBridgePage', content: tgContent },
      { name: 'DiscordBridgePage', content: dsContent },
      { name: 'BridgePage', content: bridgeContent },
    ]
    const banned = ['utilize', 'supercharge', 'paradigm shift', 'delve']
    for (const page of pages) {
      for (const word of banned) {
        assert.strictEqual(
          page.content.toLowerCase().includes(word),
          false,
          `${page.name} must not contain AI buzzword "${word}"`
        )
      }
    }
  })
})
