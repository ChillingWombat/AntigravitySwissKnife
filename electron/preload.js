const { contextBridge, ipcRenderer } = require('electron');

const electronAPI = {
  isElectron: true,
  getStartupSetting: () => ipcRenderer.invoke('desktop:get-startup-setting'),
  setStartupSetting: (enabled) => ipcRenderer.invoke('desktop:set-startup-setting', enabled),
  getCloseToTraySetting: () => ipcRenderer.invoke('desktop:get-close-to-tray-setting'),
  setCloseToTraySetting: (enabled) => ipcRenderer.invoke('desktop:set-close-to-tray-setting', enabled),
  getRuntimeModeSetting: () => ipcRenderer.invoke('desktop:get-runtime-mode-setting'),
  setRuntimeModeSetting: (mode) => ipcRenderer.invoke('desktop:set-runtime-mode-setting', mode),
  startDaemon: () => ipcRenderer.invoke('desktop:start-daemon'),
  onNavigate: (callback) => {
    if (typeof callback === 'function') {
      const subscription = (_event, toolIndex) => callback(toolIndex);
      ipcRenderer.on('desktop:navigate', subscription);
      return () => ipcRenderer.removeListener('desktop:navigate', subscription);
    }
    return () => {};
  },
  sendNotification: (title, body) => ipcRenderer.invoke('desktop:notify', { title, body }),
  openExternal: (url) => ipcRenderer.invoke('desktop:open-external', url),
  selectPath: (options) => ipcRenderer.invoke('desktop:select-path', options),
  launchAntigravity: () => ipcRenderer.invoke('desktop:launch-antigravity'),
};

if (process.contextIsolated) {
  contextBridge.exposeInMainWorld('electronAPI', electronAPI);
} else {
  window.electronAPI = electronAPI;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { electronAPI };
}
