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
  const waPagePath = path.resolve(__dirname, '../pages/WhatsAppBridgePage.tsx')
  const tgPagePath = path.resolve(__dirname, '../pages/TelegramBridgePage.tsx')
  const dsPagePath = path.resolve(__dirname, '../pages/DiscordBridgePage.tsx')

  const navContent = fs.readFileSync(navRailPath, 'utf8')
  const appContent = fs.readFileSync(appPath, 'utf8')
  const utilsContent = fs.readFileSync(utilsPagePath, 'utf8')
  const bridgeContent = fs.readFileSync(bridgePagePath, 'utf8')
  const remoteContent = fs.readFileSync(remotePagePath, 'utf8')
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

  it('App.tsx contains 4 segmented tabs for Bridge section in exact required order', () => {
    assert.ok(appContent.includes("currentTool === 10"))
    assert.ok(appContent.includes("['Remote', 'Gemini', 'WhatsApp', 'Telegram & Discord']"))
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

  it('RemoteControlPage enforces David-Design and account-switch resilience', () => {
    assert.strictEqual(emojiRegex.test(remoteContent), false, 'RemoteControlPage must not contain emojis')
    assert.ok(remoteContent.includes('Account-Switch Resilience Mode'))
    assert.ok(remoteContent.includes('api.getRemoteControlStatus'))
    assert.ok(remoteContent.includes('api.updateRemoteControlConfig'))
    assert.ok(remoteContent.includes('api.rotateRemotePIN'))
    assert.ok(remoteContent.includes('Security PIN'))
    assert.ok(remoteContent.includes('DevTools Port'))
  })

  it('WhatsAppBridgePage enforces David-Design and functional settings', () => {
    assert.strictEqual(emojiRegex.test(waContent), false, 'WhatsAppBridgePage must not contain emojis')
    assert.ok(waContent.includes('WhatsApp Agent Chat Bridge'))
    assert.ok(waContent.includes('api.getChannelsStatus'))
    assert.ok(waContent.includes('api.updateChannelsConfig'))
    assert.ok(waContent.includes("api.testChannel('whatsapp')"))
    assert.ok(waContent.includes('Allowed Phone Numbers'))
  })

  it('TelegramBridgePage enforces David-Design and functional settings', () => {
    assert.strictEqual(emojiRegex.test(tgContent), false, 'TelegramBridgePage must not contain emojis')
    assert.ok(tgContent.includes('Telegram Agent Chat Bridge'))
    assert.ok(waContent.includes('api.getChannelsStatus'))
    assert.ok(tgContent.includes('Bot Token (from @BotFather)'))
    assert.ok(tgContent.includes("api.testChannel('telegram')"))
    assert.ok(tgContent.includes('Allowed Chat IDs'))
  })

  it('DiscordBridgePage enforces David-Design and functional settings', () => {
    assert.strictEqual(emojiRegex.test(dsContent), false, 'DiscordBridgePage must not contain emojis')
    assert.ok(dsContent.includes('Discord Agent Chat Bridge'))
    assert.ok(dsContent.includes('Discord Bot Token'))
    assert.ok(dsContent.includes('Application ID'))
    assert.ok(dsContent.includes("api.testChannel('discord')"))
    assert.ok(dsContent.includes('Allowed Channel IDs'))
  })

  it('all 4 new bridge pages eliminate banned AI buzzwords (utilize, supercharge, paradigm shift, delve)', () => {
    const pages = [
      { name: 'RemoteControlPage', content: remoteContent },
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
