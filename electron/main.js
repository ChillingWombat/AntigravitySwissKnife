const { app, BrowserWindow, Tray, Menu, ipcMain, shell, Notification } = require('electron');
const path = require('path');
const http = require('http');
const fs = require('fs');
const { DaemonManager } = require('./daemon-manager');

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

  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 960,
    minHeight: 640,
    title: 'Antigravity Swiss Knife',
    icon: iconPath,
    show: !startMinimized,
    backgroundColor: '#131314', // Google Gemini dark surface token
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: false,
    },
  });

  // Intercept window close ('X') to minimize to system tray
  mainWindow.on('close', (event) => {
    if (!isQuitting) {
      event.preventDefault();
      mainWindow.hide();
      if (tray && process.platform === 'win32') {
        tray.displayBalloon({
          title: 'Antigravity Swiss Knife',
          content: 'Application minimized to system tray.',
        });
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

app.on('window-all-closed', () => {
  // On non-macOS, keep running in tray unless isQuitting
  if (process.platform === 'darwin' && isQuitting) {
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
  getMainWindow: () => mainWindow,
  getTray: () => tray,
  DAEMON_URL,
  DAEMON_PORT,
  DAEMON_HOST,
};
