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

function buildTrayApplicationMenu({
  Menu,
  buildTemplate,
  activeAlias,
  quotaInfo,
  switchMenuItems,
  accountsCount,
  showOrCreateMainWindow,
  getMainWindow,
  requestAppQuit,
}) {
  const template = buildTemplate({
    activeAlias,
    quotaInfo,
    switchMenuItems,
    accountsCount,
    onOpenDashboard: async () => {
      await showOrCreateMainWindow()
    },
    onSystemSettings: async () => {
      await showOrCreateMainWindow()
      const mainWindow = getMainWindow()
      if (!mainWindow || (typeof mainWindow.isDestroyed === 'function' && mainWindow.isDestroyed())) return

      const webContents = mainWindow.webContents
      const navigate = () => sendTrayNavigation(mainWindow, TRAY_NAVIGATION_TARGETS.systemSettings)
      if (webContents && typeof webContents.isLoading === 'function' && webContents.isLoading()) {
        if (typeof webContents.once === 'function') webContents.once('did-finish-load', navigate)
        return
      }
      if (webContents) navigate()
    },
    onQuit: () => {
      requestAppQuit()
    },
  })

  return { template, menu: Menu.buildFromTemplate(template) }
}

module.exports = {
  TRAY_NAVIGATION_TARGETS,
  sendTrayNavigation,
  buildTrayApplicationMenu,
}
