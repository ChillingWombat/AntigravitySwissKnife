const { app, BrowserWindow, Tray, Menu, ipcMain, shell, Notification } = require('electron');
const path = require('path');
const http = require('http');
const fs = require('fs');
const os = require('os');
const { DaemonManager } = require('./daemon-manager');

// Desktop background & close behavior settings (defaults to OFF: quit app & stop daemon on window close)
function getCloseToTraySetting() {
  try {
    const configDir = path.join(os.homedir(), '.config', 'antigravity-swiss');
    const settingsPath = path.join(configDir, 'desktop_settings.json');
    if (fs.existsSync(settingsPath)) {
      const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
      if (typeof data.close_to_tray === 'boolean') {
        return data.close_to_tray;
      }
    }
  } catch (err) {
    console.warn('[Settings] Failed to read desktop_settings.json:', err.message);
  }
  return false; // Default: false (OFF) - do not run in background when closed
}

function setCloseToTraySetting(enabled) {
  try {
    const configDir = path.join(os.homedir(), '.config', 'antigravity-swiss');
    if (!fs.existsSync(configDir)) {
      fs.mkdirSync(configDir, { recursive: true });
    }
    const settingsPath = path.join(configDir, 'desktop_settings.json');
    let data = {};
    if (fs.existsSync(settingsPath)) {
      try { data = JSON.parse(fs.readFileSync(settingsPath, 'utf8')); } catch (_) {}
    }
    data.close_to_tray = Boolean(enabled);
    fs.writeFileSync(settingsPath, JSON.stringify(data, null, 2), 'utf8');
    return true;
  } catch (err) {
    console.error('[Settings] Failed to write desktop_settings.json:', err.message);
    return false;
  }
}

// Port configuration
const DAEMON_PORT = 8765;
const DAEMON_HOST = '127.0.0.1';
const DAEMON_URL = `http://${DAEMON_HOST}:${DAEMON_PORT}`;

// Module State
let mainWindow = null;
let tray = null;
let isQuitting = false;
let isStoppingDaemon = false;

// Sidecar Daemon Manager instance
const daemonManager = new DaemonManager({
  port: DAEMON_PORT,
  host: DAEMON_HOST,
  baseUrl: DAEMON_URL,
});

// Liveness probe helper
function probeDaemonStatus(timeoutMs = 1500) {
  return daemonManager.checkStatus(timeoutMs);
}

// Single instance lock
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  console.log('[Electron] Another instance is already running. Quitting.');
  app.quit();
  process.exit(0);
} else {
  app.on('second-instance', (_event, _commandLine, _workingDirectory) => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      if (!mainWindow.isVisible()) mainWindow.show();
      mainWindow.focus();
    }
  });
}

function getIconPath() {
  const iconPath = path.join(__dirname, '..', 'assets', 'logo.png');
  if (fs.existsSync(iconPath)) {
    return iconPath;
  }
  return undefined;
}

async function updateTrayMenu() {
  if (!tray) return;

  let activeAccount = 'Not Logged In';
  let accountsList = [];

  try {
    const status = await probeDaemonStatus(800);
    if (status && status.active_account) {
      activeAccount = status.active_account;
    }
  } catch {}

  // Fetch accounts for quick switch
  try {
    const accRes = await new Promise((resolve) => {
      http.get(`${DAEMON_URL}/api/accounts`, { timeout: 800 }, (res) => {
        let body = '';
        res.on('data', (c) => { body += c; });
        res.on('end', () => {
          try { resolve(JSON.parse(body)); } catch { resolve([]); }
        });
      }).on('error', () => resolve([]));
    });
    if (Array.isArray(accRes)) {
      accountsList = accRes;
    } else if (accRes && Array.isArray(accRes.accounts)) {
      accountsList = accRes.accounts;
    }
  } catch {}

  const switchMenuItems = accountsList.map((acc) => {
    const email = acc.email || acc;
    const isCurrent = email === activeAccount;
    return {
      label: isCurrent ? `✓ ${email}` : email,
      enabled: !isCurrent,
      click: async () => {
        try {
          const req = http.request(`${DAEMON_URL}/api/switch`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            timeout: 2000,
          });
          req.write(JSON.stringify({ email }));
          req.end();
          setTimeout(updateTrayMenu, 1000);
        } catch (err) {
          console.error('[Tray] Switch account failed:', err);
        }
      },
    };
  });

  const contextMenu = Menu.buildFromTemplate([
    {
      label: 'Antigravity Swiss Knife',
      enabled: false,
    },
    {
      label: 'Open Dashboard',
      click: () => {
        if (mainWindow) {
          mainWindow.show();
          mainWindow.focus();
        }
      },
    },
    { type: 'separator' },
    {
      label: `Active: ${activeAccount}`,
      enabled: false,
    },
    {
      label: 'Quick Account Switch',
      submenu: switchMenuItems.length > 0 ? switchMenuItems : [{ label: 'No accounts configured', enabled: false }],
    },
    { type: 'separator' },
    {
      label: 'System Settings',
      click: () => {
        if (mainWindow) {
          mainWindow.show();
          mainWindow.focus();
          mainWindow.webContents.send('desktop:navigate', 2);
        }
      },
    },
    { type: 'separator' },
    {
      label: 'Quit Antigravity Swiss Knife',
      click: async () => {
        isQuitting = true;
        await daemonManager.stop();
        app.quit();
      },
    },
  ]);

  tray.setContextMenu(contextMenu);
}

function createTray() {
  const iconPath = getIconPath();
  if (!iconPath) return;

  try {
    tray = new Tray(iconPath);
    tray.setToolTip('Antigravity Swiss Knife');

    tray.on('click', () => {
      if (mainWindow) {
        if (mainWindow.isVisible()) {
          mainWindow.focus();
        } else {
          mainWindow.show();
          mainWindow.focus();
        }
      }
    });

    tray.on('double-click', () => {
      if (mainWindow) {
        mainWindow.show();
        mainWindow.focus();
      }
    });

    updateTrayMenu();
    // Periodically refresh tray state
    setInterval(updateTrayMenu, 15000);
  } catch (err) {
    console.warn('[Tray] Could not create system tray icon:', err.message);
  }
}

async function createWindow() {
  const iconPath = getIconPath();
  const startMinimized = process.argv.includes('--minimized') ||
                         process.argv.includes('--hidden') ||
                         app.getLoginItemSettings().wasOpenedAsHidden;

  // Disable default application menu bar
  Menu.setApplicationMenu(null);

  mainWindow = new BrowserWindow({
    width: 1152,
    height: 648,
    minWidth: 1152,
    minHeight: 648,
    title: 'Antigravity Swiss Knife',
    icon: iconPath,
    show: !startMinimized,
    autoHideMenuBar: true,
    backgroundColor: '#131314', // Google Gemini dark surface token
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: false,
    },
  });

  mainWindow.setMenu(null);

  // Lock 16:9 aspect ratio for windowed resizing
  mainWindow.setAspectRatio(16 / 9);

  // Manage aspect ratio locking across window states
  mainWindow.on('maximize', () => mainWindow.setAspectRatio(0));
  mainWindow.on('unmaximize', () => mainWindow.setAspectRatio(16 / 9));
  mainWindow.on('enter-full-screen', () => mainWindow.setAspectRatio(0));
  mainWindow.on('leave-full-screen', () => mainWindow.setAspectRatio(16 / 9));

  // Intercept window close ('X'): default is to quit app & stop daemon unless close_to_tray is manually enabled
  mainWindow.on('close', async (event) => {
    if (!isQuitting) {
      const closeToTray = getCloseToTraySetting();
      if (closeToTray) {
        event.preventDefault();
        mainWindow.hide();
        if (tray && process.platform === 'win32') {
          tray.displayBalloon({
            title: 'Antigravity Swiss Knife',
            content: 'Application minimized to system tray.',
          });
        }
      } else {
        event.preventDefault();
        isQuitting = true;
        try {
          await daemonManager.stop();
        } catch (err) {
          console.error('[App] Error stopping daemon on window close:', err);
        } finally {
          app.quit();
        }
      }
    }
  });

  // Security: Prevent window from navigating away from local daemon
  mainWindow.webContents.on('will-navigate', (event, url) => {
    if (!url.startsWith(DAEMON_URL)) {
      event.preventDefault();
      shell.openExternal(url);
    }
  });

  // Open external links in default browser
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (url.startsWith('http:') || url.startsWith('https:')) {
      shell.openExternal(url);
    }
    return { action: 'deny' };
  });

  // Load backend web GUI URL with resilient retry
  mainWindow.loadURL(DAEMON_URL).catch(() => {
    console.log('[Window] Waiting for daemon URL and retrying loadURL...');
    setTimeout(() => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.loadURL(DAEMON_URL).catch((err) => {
          console.error('[Window] Failed to load URL:', err.message);
        });
      }
    }, 1500);
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

// Setup IPC handlers
function registerIpcHandlers() {
  ipcMain.handle('desktop:get-startup-setting', async () => {
    try {
      const settings = app.getLoginItemSettings();
      return {
        openAtLogin: settings.openAtLogin,
        openAsHidden: settings.openAsHidden,
        platform: process.platform,
      };
    } catch (err) {
      console.error('[IPC] getLoginItemSettings error:', err);
      return { openAtLogin: false, openAsHidden: false, platform: process.platform };
    }
  });

  ipcMain.handle('desktop:set-startup-setting', async (_event, enabled) => {
    try {
      const isEnabled = typeof enabled === 'object' && enabled !== null
        ? Boolean(enabled.enabled)
        : Boolean(enabled);

      app.setLoginItemSettings({
        openAtLogin: isEnabled,
        openAsHidden: true,
        args: isEnabled ? ['--minimized'] : [],
      });
      const updated = app.getLoginItemSettings();
      return { success: true, openAtLogin: updated.openAtLogin };
    } catch (err) {
      console.error('[IPC] setLoginItemSettings error:', err);
      return { success: false, error: err.message };
    }
  });

  ipcMain.handle('desktop:get-close-to-tray-setting', async () => {
    return { closeToTray: getCloseToTraySetting() };
  });

  ipcMain.handle('desktop:set-close-to-tray-setting', async (_event, enabled) => {
    const isEnabled = typeof enabled === 'object' && enabled !== null
      ? Boolean(enabled.enabled)
      : Boolean(enabled);
    const success = setCloseToTraySetting(isEnabled);
    return { success, closeToTray: isEnabled };
  });

  ipcMain.handle('desktop:notify', async (_event, { title, body }) => {
    if (Notification.isSupported()) {
      new Notification({ title, body, icon: getIconPath() }).show();
      return true;
    }
    return false;
  });

  ipcMain.handle('desktop:open-external', async (_event, url) => {
    try {
      await shell.openExternal(url);
      return true;
    } catch {
      return false;
    }
  });

  ipcMain.handle('desktop:select-path', async (_event, options) => {
    try {
      const { dialog } = require('electron');
      const win = mainWindow || null;
      const res = await dialog.showOpenDialog(win, {
        properties: options?.directory ? ['openDirectory'] : ['openFile'],
        title: options?.title || 'Select Path',
      });
      if (!res.canceled && res.filePaths && res.filePaths.length > 0) {
        return res.filePaths[0];
      }
      return null;
    } catch (err) {
      console.error('Failed to open dialog:', err);
      return null;
    }
  });
}

// E2E Verification Routine
async function runE2eVerification() {
  setTimeout(async () => {
    try {
      console.log('[E2E-TEST] Verifying main window...');
      const title = mainWindow ? mainWindow.getTitle() : '';
      console.log('[E2E-TEST] Window title:', title);
      if (!title || !title.includes('Antigravity Swiss Knife')) {
        console.error('[E2E-TEST] Title mismatch! Expected "Antigravity Swiss Knife", got:', title);
        process.exit(1);
      }

      console.log('[E2E-TEST] Verifying window geometry and aspect ratio...');
      const [curWidth, curHeight] = mainWindow.getSize();
      const [minWidth, minHeight] = mainWindow.getMinimumSize();
      console.log(`[E2E-TEST] Window size: ${curWidth}x${curHeight}, Minimum size: ${minWidth}x${minHeight}`);

      // Verify minimum dimensions
      if (minWidth !== 1152 || minHeight !== 648) {
        console.error(`[E2E-TEST] Minimum size mismatch! Expected 1152x648, got: ${minWidth}x${minHeight}`);
        process.exit(1);
      }
      if (minWidth % 4 !== 0 || minHeight % 4 !== 0) {
        console.error(`[E2E-TEST] Minimum dimensions are not 4px aligned: ${minWidth}x${minHeight}`);
        process.exit(1);
      }
      if (Math.abs((minWidth / minHeight) - (16 / 9)) >= 0.0001) {
        console.error(`[E2E-TEST] Minimum aspect ratio is not 16:9: ${minWidth / minHeight}`);
        process.exit(1);
      }

      // Verify current window dimensions (either exact 1152x648 or valid 16:9 4px-aligned multiple)
      if (curWidth % 4 !== 0 || curHeight % 4 !== 0) {
        console.error(`[E2E-TEST] Current dimensions are not 4px aligned: ${curWidth}x${curHeight}`);
        process.exit(1);
      }
      if (Math.abs((curWidth / curHeight) - (16 / 9)) >= 0.0001) {
        console.error(`[E2E-TEST] Current aspect ratio is not 16:9: ${curWidth / curHeight}`);
        process.exit(1);
      }
      if (curWidth < minWidth || curHeight < minHeight) {
        console.error(`[E2E-TEST] Current dimensions (${curWidth}x${curHeight}) smaller than minimum (${minWidth}x${minHeight})!`);
        process.exit(1);
      }

      console.log('[E2E-TEST] Window geometry verified: 1152x648 (16:9 aspect ratio, 4px aligned)');
      console.log('[E2E-TEST] Probing API status on', DAEMON_URL);
      const status = await probeDaemonStatus(2000);
      console.log('[E2E-TEST] API status result:', (status && status.daemon_running) ? 'OK' : 'FAILED');
      if (!status || !status.daemon_running) {
        console.error('[E2E-TEST] Could not connect to daemon API or daemon_running is not true! Status:', status);
        process.exit(1);
      }
      console.log('[E2E-TEST] Testing IPC getLoginItemSettings...');
      const settings = app.getLoginItemSettings();
      console.log('[E2E-TEST] LoginItemSettings:', JSON.stringify(settings));

      // Test startup setting IPC normalization
      app.setLoginItemSettings({ openAtLogin: true, openAsHidden: true });
      const setTest = app.getLoginItemSettings();
      console.log('[E2E-TEST] Set startup setting test result:', setTest.openAtLogin);
      // Restore default
      app.setLoginItemSettings({ openAtLogin: false, openAsHidden: false });

      // Test closeToTray setting persistence & IPC logic
      console.log('[E2E-TEST] Testing closeToTray setting...');
      const initClose = getCloseToTraySetting();
      console.log('[E2E-TEST] Initial closeToTray:', initClose);
      setCloseToTraySetting(true);
      if (getCloseToTraySetting() !== true) {
        console.error('[E2E-TEST] Failed to set closeToTray to true');
        process.exit(1);
      }
      setCloseToTraySetting(false);
      if (getCloseToTraySetting() !== false) {
        console.error('[E2E-TEST] Failed to set closeToTray to false');
        process.exit(1);
      }
      console.log('[E2E-TEST] CloseToTray IPC and persistence verified: OK');

      console.log('[E2E-TEST] All E2E desktop assertions passed! Initiating graceful shutdown...');
      isQuitting = true;
      await daemonManager.stop();
      app.quit();
    } catch (err) {
      console.error('[E2E-TEST] Error in verification routine:', err);
      process.exit(1);
    }
  }, 2500);
}

// App lifecycle
app.whenReady().then(async () => {
  Menu.setApplicationMenu(null);
  registerIpcHandlers();
  try {
    await daemonManager.start();
  } catch (err) {
    console.error('[App] Failed to start Go daemon sidecar:', err);
  }
  createTray();
  await createWindow();

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    } else if (mainWindow) {
      mainWindow.show();
    }
  });

  if (process.env.TEST_DESKTOP_E2E === '1') {
    runE2eVerification();
  }
});

// Full application quit hook
app.on('before-quit', async (event) => {
  if (!isQuitting) {
    event.preventDefault();
    isQuitting = true;
    if (!isStoppingDaemon) {
      isStoppingDaemon = true;
      try {
        await daemonManager.stop();
      } catch (err) {
        console.error('[App] Error during daemon stop on before-quit:', err);
      } finally {
        app.quit();
      }
    }
  }
});

app.on('window-all-closed', async () => {
  const closeToTray = getCloseToTraySetting();
  if (!closeToTray || isQuitting || process.platform === 'darwin') {
    if (!isStoppingDaemon) {
      isStoppingDaemon = true;
      try {
        await daemonManager.stop();
      } catch (err) {}
    }
    app.quit();
  }
});

// Operating System termination signal handlers
const handleExitSignal = async (signal) => {
  console.log(`[App] Received OS signal ${signal}. Initiating graceful teardown...`);
  if (isStoppingDaemon) return;
  isStoppingDaemon = true;
  isQuitting = true;
  try {
    await daemonManager.stop();
  } catch (err) {
    console.error(`[App] Error stopping daemon on ${signal}:`, err);
  } finally {
    process.exit(0);
  }
};

process.on('SIGINT', () => handleExitSignal('SIGINT'));
process.on('SIGTERM', () => handleExitSignal('SIGTERM'));
if (process.platform !== 'win32') {
  process.on('SIGHUP', () => handleExitSignal('SIGHUP'));
}

// Uncaught error handler
process.on('uncaughtException', async (err) => {
  console.error('[App] Uncaught exception in main process:', err);
  try {
    await daemonManager.stop();
  } catch {}
  process.exit(1);
});

// Module Exports
module.exports = {
  daemonManager,
  DaemonManager,
  probeDaemonStatus,
  createWindow,
  createTray,
  updateTrayMenu,
  registerIpcHandlers,
  getCloseToTraySetting,
  setCloseToTraySetting,
  getMainWindow: () => mainWindow,
  getTray: () => tray,
  DAEMON_URL,
  DAEMON_PORT,
  DAEMON_HOST,
};
