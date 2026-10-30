'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const { TRAY_NAVIGATION_TARGETS, sendTrayNavigation, buildTrayApplicationMenu } = require('./tray-navigation')
const { buildTrayMenuTemplate } = require('./tray-account-menu')

function findMenuItem(items, matcher) {
  for (const item of items || []) {
    if (matcher(item)) return item
    const children = Array.isArray(item.submenu) ? item.submenu : item.submenu && item.submenu.items
    const nested = findMenuItem(children, matcher)
    if (nested) return nested
  }
  return null
}

test('tray navigation uses semantic targets and one IPC payload shape', () => {
  const sent = []
  const mainWindow = {
    webContents: {
      send(...args) {
        sent.push(args)
      },
    },
  }

  assert.equal(TRAY_NAVIGATION_TARGETS.systemSettings, 'system-settings')
  assert.equal(TRAY_NAVIGATION_TARGETS.utilitiesArchives, 'utilities/archives')
  assert.equal(sendTrayNavigation(mainWindow, TRAY_NAVIGATION_TARGETS.systemSettings), true)
  assert.equal(sendTrayNavigation(mainWindow, TRAY_NAVIGATION_TARGETS.utilitiesArchives), true)
  assert.deepEqual(sent, [
    ['desktop:navigate', 'system-settings'],
    ['desktop:navigate', 'utilities/archives'],
  ])
})

test('production tray template wires System Settings to the active window', async () => {
  const sent = []
  let showCalls = 0
  let builtTemplate = null
  const mainWindow = {
    webContents: {
      isLoading: () => false,
      send(...args) {
        sent.push(args)
      },
    },
  }
  const Menu = {
    buildFromTemplate(template) {
      builtTemplate = template
      return { template }
    },
  }

  const { menu } = buildTrayApplicationMenu({
    Menu,
    buildTemplate: buildTrayMenuTemplate,
    activeAlias: 'route-test@example.com',
    quotaInfo: '5H: 100% | 7D: 100%',
    switchMenuItems: [],
    accountsCount: 0,
    showOrCreateMainWindow: async () => { showCalls += 1 },
    getMainWindow: () => mainWindow,
    requestAppQuit: () => {},
  })

  assert.ok(builtTemplate)
  assert.equal(menu.template, builtTemplate)
  const item = findMenuItem(builtTemplate, (candidate) => String(candidate.label || '').includes('System Settings'))
  assert.ok(item, 'System Settings tray item must exist')
  await item.click()
  assert.equal(showCalls, 1)
  assert.deepEqual(sent, [['desktop:navigate', TRAY_NAVIGATION_TARGETS.systemSettings]])
})

test('tray System Settings waits for a loading window before navigating', async () => {
  const sent = []
  let finishLoad = null
  const mainWindow = {
    webContents: {
      isLoading: () => true,
      once(event, callback) {
        assert.equal(event, 'did-finish-load')
        finishLoad = callback
      },
      send(...args) {
        sent.push(args)
      },
    },
  }
  const Menu = {
    buildFromTemplate(template) {
      return { template }
    },
  }

  const { menu } = buildTrayApplicationMenu({
    Menu,
    buildTemplate: buildTrayMenuTemplate,
    activeAlias: 'Not Logged In',
    quotaInfo: '5H: 0% | 7D: 0%',
    switchMenuItems: [],
    accountsCount: 0,
    showOrCreateMainWindow: async () => {},
    getMainWindow: () => mainWindow,
    requestAppQuit: () => {},
  })

  const item = findMenuItem(menu.template, (candidate) => String(candidate.label || '').includes('System Settings'))
  assert.ok(item, 'System Settings tray item must exist')
  await item.click()
  assert.deepEqual(sent, [])
  assert.equal(typeof finishLoad, 'function')
  finishLoad()
  assert.deepEqual(sent, [['desktop:navigate', TRAY_NAVIGATION_TARGETS.systemSettings]])
})
