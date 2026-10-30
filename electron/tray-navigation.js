'use strict'

const TRAY_NAVIGATION_TARGETS = Object.freeze({
  dashboard: 'switcher/dashboard',
  systemSettings: 'system-settings',
  utilitiesArchives: 'utilities/archives',
})

function sendTrayNavigation(mainWindow, target) {
  const webContents = mainWindow && mainWindow.webContents
  if (!webContents || typeof webContents.send !== 'function') return false
  webContents.send('desktop:navigate', target)
  return true
}

module.exports = {
  TRAY_NAVIGATION_TARGETS,
  sendTrayNavigation,
}
