'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { TRAY_NAVIGATION_TARGETS, sendTrayNavigation } = require('./tray-navigation')

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

test('main-process tray menu no longer sends raw navigation indices', () => {
  const source = fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8')
  assert.match(source, /sendTrayNavigation\(mainWindow, TRAY_NAVIGATION_TARGETS\.systemSettings\)/)
  assert.doesNotMatch(source, /desktop:navigate',\s*(?:1|2|6)\b/)
})
