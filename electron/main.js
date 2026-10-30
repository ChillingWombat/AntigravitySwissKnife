const { app, BrowserWindow, Tray, Menu, ipcMain, shell, Notification, nativeImage, session, clipboard, screen, dialog } = require('electron');
const path = require('path');
const { sendTrayNavigation, TRAY_NAVIGATION_TARGETS, buildTrayApplicationMenu } = require('./tray-navigation');
const http = require('http');
const fs = require('fs');
const os = require('os');
const { DaemonManager } = require('./daemon-manager');
const { createDaemonBroker, shouldAttachDaemonSessionHeader } = require('./daemon-broker');
const {
  isAccountExcludedFromSwitchMenu,
  rankAndFilterAccountsForTray,
  buildSwitchMenuItems,
  formatQuotaPercentage,
  formatQuotaInfo,
  resolveActiveAccount,
  resolveActiveAccountAlias,
  buildTrayMenuTemplate,
} = require('./tray-account-menu');

// Windows taskbar/notification identity: must match build.appId in package.json
if (app && typeof app.setAppUserModelId === 'function') {
  app.setAppUserModelId('com.antigravity.swiss-knife');
}

// Linux desktop configuration: use native Wayland when available and disable hardware GL
// scanout buffers unless explicitly requested (prevents Intel i915 execbuf ENOMEM and XWayland unmap stalls).
if (process.platform === 'linux' && app) {
  if (typeof app.disableHardwareAcceleration === 'function' && process.env.ANTIGRAVITY_SWISS_ENABLE_GPU !== '1') {
    app.disableHardwareAcceleration();
  }
  if (app.commandLine) {
    if (process.env.ELECTRON_NO_SANDBOX === '1') {
      app.commandLine.appendSwitch('no-sandbox');
      app.commandLine.appendSwitch('disable-setuid-sandbox');
    }
    app.commandLine.appendSwitch('ozone-platform-hint', 'auto');
  }
}

// Desktop background & close behavior settings (defaults to ON: minimize to system tray on window close)
function getCloseToTraySetting() {
  try {
    const configDir = getSwissConfigDir();
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
  return true; // Default: true (ON) - minimize to system tray when closed
}

function setCloseToTraySetting(enabled) {
  try {
    const configDir = getSwissConfigDir();
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

// Desktop runtime mode setting ('app' | 'daemon', defaults to 'app')
function getRuntimeModeSetting() {
  try {
    const configDir = getSwissConfigDir();
    const settingsPath = path.join(configDir, 'desktop_settings.json');
    if (fs.existsSync(settingsPath)) {
      const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
      if (data.runtime_mode === 'daemon' || data.runtime_mode === 'app') {
        return data.runtime_mode;
      }
    }
  } catch (err) {
    console.warn('[Settings] Failed to read desktop_settings.json runtime_mode:', err.message);
  }
  return 'app'; // Default: 'app' (App Only mode)
}

function setRuntimeModeSetting(mode) {
  try {
    const configDir = getSwissConfigDir();
    if (!fs.existsSync(configDir)) {
      fs.mkdirSync(configDir, { recursive: true });
    }
    const settingsPath = path.join(configDir, 'desktop_settings.json');
    let data = {};
    if (fs.existsSync(settingsPath)) {
      try { data = JSON.parse(fs.readFileSync(settingsPath, 'utf8')); } catch (_) {}
    }
    data.runtime_mode = mode === 'daemon' ? 'daemon' : 'app';
    fs.writeFileSync(settingsPath, JSON.stringify(data, null, 2), 'utf8');
    return true;
  } catch (err) {
    console.error('[Settings] Failed to write desktop_settings.json runtime_mode:', err.message);
    return false;
  }
}

let teardownPromise = null;

// Teardown routing depending on runtime mode
async function teardownDaemonForAppExit(options = {}) {
  if (teardownPromise) {
    return teardownPromise;
  }
  teardownPromise = (async () => {
    isStoppingDaemon = true;
    const mode = options.mode || getRuntimeModeSetting();
    const requestRestore = options.requestRestore || (() => requestDaemonJsonWithAuth(
      '/api/custom_models/restore_native',
      { validateResponse: isNativeRestoreResponse }
    ));
    const stopDaemon = options.stopDaemon || (() => daemonManager.stop({ force: true }));
    const detachDaemon = options.detachDaemon || (() => daemonManager.detach());
    const failures = [];

    console.log(`[App] Teardown daemon for app exit. Runtime mode: ${mode}`);
    try {
      await requestRestore();
    } catch (err) {
      console.warn('[App] Failed to restore native Antigravity mode:', err.message || err);
      failures.push(err);
    }

    try {
      if (mode === 'daemon') {
        await detachDaemon();
      } else {
        await stopDaemon();
      }
    } catch (err) {
      console.warn('[App] Failed to stop daemon during app exit:', err.message || err);
      failures.push(err);
    }

    if (failures.length === 1) {
      throw failures[0];
    }
    if (failures.length > 1) {
      throw new AggregateError(failures, 'daemon teardown failed');
    }
  })();
  return teardownPromise;
}

// Port configuration
const DAEMON_PORT = 8765;
const DAEMON_HOST = '127.0.0.1';
const DAEMON_URL = `http://${DAEMON_HOST}:${DAEMON_PORT}`;
let trustedConnectingFallbackUrl = null;

// Helper: verify that target URL belongs strictly to the local daemon origin (P1-16)
// Helper: verify that an IPC event originates from our active main window and trusted daemon origin
function isTrustedSender(event, targetWindow = mainWindow) {
  if (!targetWindow || (typeof targetWindow.isDestroyed === 'function' && targetWindow.isDestroyed())) {
    return false;
  }
  if (!event || !event.sender || event.sender !== targetWindow.webContents) {
    return false;
  }
  if (event.senderFrame) {
    if (event.senderFrame.parent) return false;
    const mainFrame = event.sender.mainFrame;
    if (mainFrame && (
      event.senderFrame !== mainFrame ||
      event.senderFrame.routingId !== mainFrame.routingId
    )) {
      return false;
    }
  }
  return isTrustedDaemonOrigin(event.senderFrame?.url);
}

function isTrustedConnectingFallbackUrl(targetUrl) {
  return Boolean(trustedConnectingFallbackUrl) && targetUrl === trustedConnectingFallbackUrl;
}

function isTrustedDaemonOrigin(targetUrl, daemonBase = DAEMON_URL) {
  if (!targetUrl || typeof targetUrl !== 'string') return false;
  if (isTrustedConnectingFallbackUrl(targetUrl)) return true;
  try {
    const parsed = new URL(targetUrl);
    const expected = new URL(daemonBase);
    return parsed.origin === expected.origin;
  } catch {
    return false;
  }
}

// Helper: ensure external URLs opened via shell.openExternal strictly use http: or https: (P1-16)
function isSafeExternalUrl(targetUrl) {
  if (!targetUrl || typeof targetUrl !== 'string') return false;
  try {
    const parsed = new URL(targetUrl);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

// Daemon session token auth: the Go daemon enforces default-deny loopback auth (P0-50)
// and accepts the startup token persisted in session.token. The desktop renderer
// authenticates by attaching that token as X-Session-Token on every daemon-origin request.
let cachedSessionToken = null;
let cachedSessionTokenAt = 0;

function getSwissConfigDir() {
  if (process.env.ANTIGRAVITY_SWISS_CONFIG_DIR) {
    return process.env.ANTIGRAVITY_SWISS_CONFIG_DIR;
  }
  const home = os.homedir();
  if (process.platform === 'win32') {
    return path.join(process.env.APPDATA || path.join(home, 'AppData', 'Roaming'), 'antigravity-swiss');
  }
  if (process.platform === 'darwin') {
    return path.join(home, 'Library', 'Application Support', 'antigravity-swiss');
  }
  return path.join(process.env.XDG_CONFIG_HOME || path.join(home, '.config'), 'antigravity-swiss');
}

const daemonBroker = createDaemonBroker({
  getToken: () => getDaemonSessionToken(),
  requestApproval: async ({ method, path, bodyPreview, resolvedPath, denialReason }) => {
    const { dialog } = require('electron');
    const owner = BrowserWindow.getFocusedWindow() || BrowserWindow.getAllWindows()[0] || null;
    const denied = Boolean(denialReason);
    const sensitiveRead = method === 'GET';
    const detail = [
      resolvedPath ? `Resolved path:\n${resolvedPath}` : '',
      denied ? `Denied: ${denialReason}` : '',
      !resolvedPath && !denied
        ? sensitiveRead
          ? 'This request reads sensitive data from this machine.'
          : 'This request can change files or control applications on this machine.'
        : '',
      bodyPreview ? `Body preview:\n${bodyPreview}` : '',
    ].filter(Boolean).join('\n\n');
    const result = await dialog.showMessageBox(owner, {
      type: 'warning',
      buttons: denied ? ['OK'] : ['Deny', 'Approve'],
      defaultId: 0,
      cancelId: 0,
      noLink: true,
      title: denied
        ? 'File access denied'
        : resolvedPath
          ? 'Approve file access?'
          : sensitiveRead
            ? 'Approve sensitive read?'
            : 'Approve machine control?',
      message: `${method} ${path}`,
      detail,
    });
    return !denied && result.response === 1;
  },
});

const {
  installLegacySessionTokenCleanup,
  isDaemonApiUrl,
  shouldCancelDaemonSubFrameNavigation,
} = require('./daemon-broker');
installLegacySessionTokenCleanup(app);

function getDaemonSessionToken(maxAgeMs = 3000) {
  const now = Date.now();
  if (cachedSessionToken && now - cachedSessionTokenAt < maxAgeMs) {
    return cachedSessionToken;
  }
  try {
    const token = fs.readFileSync(path.join(getSwissConfigDir(), 'session.token'), 'utf8').trim();
    if (token) {
      cachedSessionToken = token;
      cachedSessionTokenAt = now;
      return token;
    }
  } catch (err) {
    // Token file missing (daemon first boot in progress): fall back to last known token.
  }
  return cachedSessionToken || '';
}

function attachDaemonAuthHeaders() {
  if (!session || !session.defaultSession || !session.defaultSession.webRequest) {
    return;
  }
  const filter = { urls: [`${DAEMON_URL}/*`] };
  session.defaultSession.webRequest.onBeforeRequest(filter, (details, callback) => {
    callback({ cancel: shouldCancelDaemonSubFrameNavigation(details) });
  });
  session.defaultSession.webRequest.onBeforeSendHeaders(filter, (details, callback) => {
    const requestHeaders = details.requestHeaders || {};
    for (const key of Object.keys(requestHeaders)) {
      if (key.toLowerCase() === 'x-session-token') delete requestHeaders[key];
    }
    if (!shouldAttachDaemonSessionHeader(details)) {
      callback({ requestHeaders });
      return;
    }
    // Document/static loads need the startup token; renderer XHR/fetch must use
    // the allowlisted IPC broker instead of receiving this credential implicitly.
    const token = getDaemonSessionToken();
    if (token) {
      requestHeaders['X-Session-Token'] = token;
    }
    callback({ requestHeaders });
  });
}

// Main-process requests (tray, launch, probes) run outside the renderer session,
// so the webRequest hook above never sees them. Attach the token explicitly.
function daemonAuthHeaders(extra = {}, maxAgeMs = 3000) {
  const headers = { ...extra };
  const token = getDaemonSessionToken(maxAgeMs);
  if (token) {
    headers['X-Session-Token'] = token;
  }
  return headers;
}

function isNativeRestoreResponse(payload) {
  return Boolean(payload) &&
    typeof payload === 'object' &&
    payload.success === true &&
    payload.restored_native === true;
}

async function requestDaemonJsonWithAuth(path, options = {}) {
  const baseUrl = options.baseUrl || DAEMON_URL;
  const timeoutMs = options.timeoutMs ?? 2000;
  const authHeaders = options.authHeaders || daemonAuthHeaders;
  const requestUrl = new URL(path, baseUrl).toString();

  const send = async (headers) => {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
      return await fetch(requestUrl, {
        method: 'POST',
        headers,
        signal: controller.signal,
      });
    } catch (err) {
      if (controller.signal.aborted) {
        throw new Error(`daemon request timed out after ${timeoutMs}ms`);
      }
      throw err;
    } finally {
      clearTimeout(timer);
    }
  };

  let response = await send(authHeaders());
  if (response.status === 401) {
    response = await send(authHeaders({}, 0));
  }
  if (response.status === 401) {
    throw new Error('daemon authentication failed after token refresh (401)');
  }
  if (!response.ok) {
    const detail = await response.text();
    throw new Error(`daemon responded ${response.status}${detail ? `: ${detail.slice(0, 200)}` : ''}`);
  }

  let payload;
  try {
    payload = await response.json();
  } catch (_) {
    throw new Error('daemon returned invalid JSON');
  }
  if (typeof options.validateResponse === 'function' && !options.validateResponse(payload)) {
    throw new Error('daemon returned unexpected response contract');
  }
  return payload;
}

// Module State
let mainWindow = null;
let tray = null;
let activeTrayContextMenu = null;
let trayMenuInterval = null;
let lastTrayMenuSignature = null;
let trayMenuRevision = 0;
let isQuitting = false;
let isStoppingDaemon = false;
let isTearingDown = false;
let teardownCompleted = false;

function isMainWindowUsable(win = mainWindow) {
  if (!win) return false;
  if (typeof win.isDestroyed === 'function' && win.isDestroyed()) return false;
  if (win.webContents) {
    if (typeof win.webContents.isDestroyed === 'function' && win.webContents.isDestroyed()) return false;
    if (typeof win.webContents.isCrashed === 'function' && win.webContents.isCrashed()) return false;
  }
  return true;
}

async function showOrCreateMainWindow() {
  if (!isMainWindowUsable(mainWindow)) {
    mainWindow = null;
    await createWindow({ forceShow: true });
    return mainWindow;
  }
  try {
    if (typeof mainWindow.isMinimized === 'function' && mainWindow.isMinimized()) {
      mainWindow.restore();
    }
    if (typeof mainWindow.show === 'function') {
      mainWindow.show();
    }
    if (process.platform !== 'linux' && typeof mainWindow.setAlwaysOnTop === 'function') {
      mainWindow.setAlwaysOnTop(true);
      setTimeout(() => {
        try {
          if (mainWindow && typeof mainWindow.setAlwaysOnTop === 'function' && !mainWindow.isDestroyed()) {
            mainWindow.setAlwaysOnTop(false);
          }
        } catch (_) {}
      }, 150).unref();
    }
    if (typeof mainWindow.moveTop === 'function') {
      mainWindow.moveTop();
    }
    if (typeof mainWindow.focus === 'function') {
      mainWindow.focus();
    }
    if (process.platform === 'linux') {
      activateLinuxWindowOnDesktop(1);
    }
  } catch (err) {
    console.warn('[Window] Failed to reveal existing window, recreating:', err.message);
    try {
      if (mainWindow && typeof mainWindow.destroy === 'function' && !mainWindow.isDestroyed()) {
        mainWindow.destroy();
      }
    } catch (_) {}
    mainWindow = null;
    await createWindow({ forceShow: true });
  }
  return mainWindow;
}

async function toggleOrShowMainWindow() {
  if (isMainWindowUsable(mainWindow) && typeof mainWindow.isVisible === 'function' && mainWindow.isVisible() && !mainWindow.isMinimized()) {
    if (process.platform === 'linux') {
      mainWindow.close();
    } else {
      mainWindow.hide();
    }
    return;
  }
  await showOrCreateMainWindow();
}

function requestAppQuit() {
  isQuitting = true;
  if (trayMenuInterval) {
    clearInterval(trayMenuInterval);
    trayMenuInterval = null;
  }
  if (app && typeof app.quit === 'function') {
    app.quit();
  }
}

// Sidecar Daemon Manager instance
const daemonManager = new DaemonManager({
  port: DAEMON_PORT,
  host: DAEMON_HOST,
  baseUrl: DAEMON_URL,
  getAuthHeaders: () => daemonAuthHeaders(),
});

// Liveness probe helper
function probeDaemonStatus(timeoutMs = 1500) {
  return daemonManager.checkStatus(timeoutMs);
}

function loadConnectingFallback(win) {
  const fallbackUrl = `data:text/html;charset=utf-8,${encodeURIComponent(buildConnectingFallbackHtml())}`;
  trustedConnectingFallbackUrl = fallbackUrl;
  return win.loadURL(fallbackUrl).catch(() => {});
}

function buildConnectingFallbackHtml() {
  return `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Antigravity Swiss Knife - Connecting</title>
  <style>
    body { background: #131314; color: #e3e3e3; font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100vh; margin: 0; }
    h2 { margin-bottom: 8px; font-weight: 500; }
    p { color: #8e918f; font-size: 14px; margin-bottom: 24px; }
    .actions { display: flex; gap: 12px; align-items: center; }
    button { background: #1a73e8; color: white; border: none; padding: 8px 20px; border-radius: 6px; font-size: 14px; cursor: pointer; }
    button:hover { background: #1557b0; }
    button:disabled { background: #3c4043; cursor: not-allowed; }
    #quitBtn { background: #282a2d; color: #c4c7c5; border: 1px solid #3c4043; }
    #quitBtn:hover { background: #35373b; color: #e3e3e3; }
  </style>
</head>
<body>
  <h2>Connecting to Daemon...</h2>
  <p>The companion service is starting or unreachable on port 8765.</p>
  <div class="actions">
    <button id="retryBtn" type="button" onclick="checkAndRetry()">Retry Connection</button>
    <button id="quitBtn" type="button" onclick="window.electronAPI?.quitApp?.()">Quit Application</button>
  </div>
  <script>
    const retryButton = document.getElementById("retryBtn");
    let retryInFlight = false;

    async function checkAndRetry() {
      if (retryInFlight) return;
      retryInFlight = true;
      if (retryButton) retryButton.disabled = true;
      try {
        const result = await window.electronAPI?.retryDaemonConnection?.();
        if (result?.success) return;
      } catch (_) {}
      retryInFlight = false;
      if (retryButton) retryButton.disabled = false;
    }

    checkAndRetry();
    setInterval(checkAndRetry, 1500);
  </script>
</body>
</html>`;
}

async function recoverDaemonConnection(options = {}) {
  const manager = options.manager || daemonManager;
  const targetWindow = options.window === undefined ? mainWindow : options.window;

  try {
    await manager.start();
    const status = await manager.checkStatus(1500);
    if (!status || status.daemon_running !== true) {
      return { success: false, error: 'Swiss daemon did not pass its health contract' };
    }
    if (!targetWindow || (typeof targetWindow.isDestroyed === 'function' && targetWindow.isDestroyed())) {
      return { success: false, error: 'Application window is unavailable' };
    }
    await targetWindow.loadURL(DAEMON_URL);
    return { success: true, status };
  } catch (err) {
    return { success: false, error: err.message };
  }
}

// Single instance lock
if (app && typeof app.requestSingleInstanceLock === 'function') {
  const gotTheLock = app.requestSingleInstanceLock();
  if (!gotTheLock) {
    console.log('[Electron] Another instance is already running. Quitting.');
    app.quit();
    process.exit(0);
  } else {
    app.on('second-instance', async (_event, _commandLine, _workingDirectory) => {
      await showOrCreateMainWindow();
    });
  }
}

function getIconPath() {
  const iconPath = path.join(__dirname, '..', 'assets', 'logo.png');
  if (fs.existsSync(iconPath)) {
    return iconPath;
  }
  return undefined;
}

function getWindowIcon() {
  // Windows prefers a multi-size .ico for window/taskbar icons
  if (process.platform === 'win32') {
    const icoPath = path.join(__dirname, '..', 'assets', 'icons', 'icon.ico');
    if (fs.existsSync(icoPath)) {
      return icoPath;
    }
  }
  const iconPath = path.join(__dirname, '..', 'assets', 'icons', 'icon_256x256.png');
  if (fs.existsSync(iconPath)) {
    return iconPath;
  }
  return getIconPath();
}

function getTrayIcon() {
  const isMac = process.platform === 'darwin';
  const isWin = process.platform === 'win32';

  // Notification-area icons need a transparent background: the app icon's white
  // squircle either disappears on the white overflow flyout or reads as a blank
  // tile, which is why the tray icon looked small even after the taskbar icon
  // was fixed. tray_*.png carry only the knife glyph on transparency.
  //
  // Windows 11 draws tray icons at 24 px (30 px at 125 % DPI, 36 px at 150 %),
  // so hand it the 32 px glyph instead of a 16 px bitmap that would be upscaled.
  const trayFile = isWin
    ? path.join(__dirname, '..', 'assets', 'icons', 'tray_32x32.png')
    : path.join(__dirname, '..', 'assets', 'icons', 'tray_24x24.png');

  let img;
  if (fs.existsSync(trayFile)) {
    img = nativeImage.createFromPath(trayFile);
  }
  if (!img || img.isEmpty()) {
    const icoPath = path.join(__dirname, '..', 'assets', 'icons', 'icon.ico');
    if (fs.existsSync(icoPath)) {
      img = nativeImage.createFromPath(icoPath);
    }
  }
  if (!img || img.isEmpty()) {
    const rawPath = getIconPath();
    if (rawPath) {
      img = nativeImage.createFromPath(rawPath);
    }
  }

  if (!img || img.isEmpty()) {
    return null;
  }

  // Windows: return the native image untouched — resizing it to 16 px would
  // reintroduce the blur the 32 px glyph just fixed.
  if (isWin) {
    return img;
  }

  // macOS status item icons are 18x18 template images (avoids covering half
  // the top bar); Linux tray icons are conventionally 22x22.
  if (isMac) {
    const resized = img.resize({ width: 18, height: 18 });
    if (typeof resized.setTemplateImage === 'function') {
      resized.setTemplateImage(true);
    }
    return resized;
  }
  return img.resize({ width: 22, height: 22 });
}

function switchAccountViaHttp(email) {
  try {
    const req = http.request(`${DAEMON_URL}/api/switch`, {
      method: 'POST',
      headers: daemonAuthHeaders({ 'Content-Type': 'application/json' }),
      timeout: 3000,
    }, (res) => {
      let body = '';
      res.on('data', (c) => { body += c; });
      res.on('end', () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          console.log(`[Tray] Switched to account ${email} successfully`);
        } else {
          console.error(`[Tray] Switch account failed (status ${res.statusCode}): ${body}`);
        }
        setTimeout(updateTrayMenu, 500);
      });
    });
    req.on('error', (err) => {
      console.error('[Tray] Switch account request error:', err);
    });
    req.write(JSON.stringify({ email }));
    req.end();
  } catch (err) {
    console.error('[Tray] Switch account failed:', err);
  }
}

function computeTrayMenuSignature({ activeAlias, quotaInfo, accountsCount, switchMenuItems }) {
  const itemsSig = Array.isArray(switchMenuItems)
    ? switchMenuItems.map((item) => `${item.label || ''}:${Boolean(item.enabled)}`).sort().join('|')
    : '';
  return `${activeAlias || ''}::${quotaInfo || ''}::${accountsCount || 0}::${itemsSig}`;
}

function applyTrayMenuState({
  activeAlias = 'Not Logged In',
  quotaInfo = '5H: 0% | 7D: 0%',
  switchMenuItems = [],
  accountsCount = 0,
  force = false,
} = {}) {
  if (!tray || (typeof tray.isDestroyed === 'function' && tray.isDestroyed())) {
    return false;
  }

  const signature = computeTrayMenuSignature({ activeAlias, quotaInfo, accountsCount, switchMenuItems });
  if (!force && signature === lastTrayMenuSignature) {
    return false;
  }

  const { menu: contextMenu } = buildTrayApplicationMenu({
    Menu,
    buildTemplate: buildTrayMenuTemplate,
    activeAlias,
    quotaInfo,
    switchMenuItems,
    accountsCount,
    showOrCreateMainWindow,
    getMainWindow: () => mainWindow,
    requestAppQuit,
  });
  activeTrayContextMenu = contextMenu;
  tray.setContextMenu(contextMenu);
  lastTrayMenuSignature = signature;
  trayMenuRevision += 1;

  // On Linux SNI hosts (e.g. GNOME status-tray OverflowButton), updating ToolTip
  // with an em-dash suffix ("Antigravity Swiss Knife — ...") preserves the normalized
  // appId/display name ("Antigravity Swiss Knife") while guaranteeing a PropertiesChanged
  // signal on org.kde.StatusNotifierItem so cached DBusMenu item IDs are invalidated.
  if (typeof tray.setToolTip === 'function') {
    if (process.platform === 'linux') {
      tray.setToolTip(`Antigravity Swiss Knife — ${activeAlias} (${quotaInfo}) [${trayMenuRevision}]`);
    } else {
      tray.setToolTip('Antigravity Swiss Knife');
    }
  }

  return true;
}

let isUpdatingTrayMenu = false;

async function fetchTrayMenuData() {
  let activeAccount = 'Not Logged In';
  let accountsList = [];

  try {
    const status = await probeDaemonStatus(800);
    if (status && status.active_account) {
      activeAccount = status.active_account;
    }
  } catch {}

  // Fetch accounts from fleet quota endpoint (includes real-time quota state, error status, and ranking)
  let fleetFetched = false;
  try {
    const fleetRes = await new Promise((resolve) => {
      let resolved = false;
      let req = null;
      const finish = (val) => {
        if (!resolved) {
          resolved = true;
          resolve(val);
        }
      };
      const guardTimer = setTimeout(() => {
        try { if (req) req.destroy(); } catch (_) {}
        finish(null);
      }, 1200);
      guardTimer.unref();

      req = http.get(`${DAEMON_URL}/api/quota/fleet`, { timeout: 800, headers: daemonAuthHeaders() }, (res) => {
        let body = '';
        res.on('data', (c) => { body += c; });
        res.on('end', () => {
          clearTimeout(guardTimer);
          try { finish(JSON.parse(body)); } catch { finish(null); }
        });
        res.on('error', () => {
          clearTimeout(guardTimer);
          finish(null);
        });
      });
      req.on('timeout', () => {
        clearTimeout(guardTimer);
        req.destroy();
        finish(null);
      });
      req.on('error', () => {
        clearTimeout(guardTimer);
        finish(null);
      });
    });

    if (fleetRes && Array.isArray(fleetRes.accounts)) {
      accountsList = fleetRes.accounts;
      if (fleetRes.active_account) {
        activeAccount = fleetRes.active_account;
      }
      fleetFetched = true;
    }
  } catch {}

  // Fallback to /api/accounts if fleet endpoint was unavailable or returned empty
  if (!fleetFetched || accountsList.length === 0) {
    try {
      const accRes = await new Promise((resolve) => {
        let resolved = false;
        let req = null;
        const finish = (val) => {
          if (!resolved) {
            resolved = true;
            resolve(val);
          }
        };
        const guardTimer = setTimeout(() => {
          try { if (req) req.destroy(); } catch (_) {}
          finish([]);
        }, 1200);
        guardTimer.unref();

        req = http.get(`${DAEMON_URL}/api/accounts`, { timeout: 800, headers: daemonAuthHeaders() }, (res) => {
          let body = '';
          res.on('data', (c) => { body += c; });
          res.on('end', () => {
            clearTimeout(guardTimer);
            try { finish(JSON.parse(body)); } catch { finish([]); }
          });
          res.on('error', () => {
            clearTimeout(guardTimer);
            finish([]);
          });
        });
        req.on('timeout', () => {
          clearTimeout(guardTimer);
          req.destroy();
          finish([]);
        });
        req.on('error', () => {
          clearTimeout(guardTimer);
          finish([]);
        });
      });
      if (Array.isArray(accRes)) {
        accountsList = accRes;
      } else if (accRes && Array.isArray(accRes.accounts)) {
        accountsList = accRes.accounts;
      }
    } catch {}
  }

  let activeAcc = resolveActiveAccount(accountsList, activeAccount);
  if (activeAcc && activeAcc.email && (activeAccount === 'Not Logged In' || !activeAccount)) {
    activeAccount = activeAcc.email;
  }
  const activeAlias = resolveActiveAccountAlias(accountsList, activeAccount);

  // If active account has no quota data yet, try quick query to /api/quota
  if (activeAccount && activeAccount !== 'Not Logged In' && (!activeAcc || activeAcc.quota_5h_current === undefined || activeAcc.quota_weekly === undefined)) {
    try {
      const singleQuota = await new Promise((resolve) => {
        let resolved = false;
        let req = null;
        const finish = (val) => {
          if (!resolved) {
            resolved = true;
            resolve(val);
          }
        };
        const guardTimer = setTimeout(() => {
          try { if (req) req.destroy(); } catch (_) {}
          finish(null);
        }, 800);
        guardTimer.unref();

        req = http.get(`${DAEMON_URL}/api/quota?email=${encodeURIComponent(activeAccount)}`, { timeout: 400, headers: daemonAuthHeaders() }, (res) => {
          let body = '';
          res.on('data', (c) => { body += c; });
          res.on('end', () => {
            clearTimeout(guardTimer);
            try { finish(JSON.parse(body)); } catch { finish(null); }
          });
          res.on('error', () => {
            clearTimeout(guardTimer);
            finish(null);
          });
        });
        req.on('timeout', () => {
          clearTimeout(guardTimer);
          req.destroy();
          finish(null);
        });
        req.on('error', () => {
          clearTimeout(guardTimer);
          finish(null);
        });
      });
      if (singleQuota) {
        if (!activeAcc) {
          activeAcc = { email: activeAccount };
        }
        if (typeof singleQuota.quota_5h_fraction === 'number') {
          activeAcc.quota_5h_current = singleQuota.quota_5h_fraction;
        } else if (typeof singleQuota.quota_5h_current === 'number') {
          activeAcc.quota_5h_current = singleQuota.quota_5h_current;
        } else if (typeof singleQuota.min_fraction === 'number') {
          activeAcc.quota_5h_current = singleQuota.min_fraction;
        }

        if (typeof singleQuota.quota_weekly_fraction === 'number') {
          activeAcc.quota_weekly = singleQuota.quota_weekly_fraction;
        } else if (typeof singleQuota.quota_weekly === 'number') {
          activeAcc.quota_weekly = singleQuota.quota_weekly;
        }
        if (typeof singleQuota.quota_5h_argon === 'number') {
          activeAcc.quota_5h_argon = singleQuota.quota_5h_argon;
          activeAcc.reset_horizon_5h_argon_text = singleQuota.reset_horizon_5h_argon_text;
        }
        if (typeof singleQuota.quota_weekly_argon === 'number') {
          activeAcc.quota_weekly_argon = singleQuota.quota_weekly_argon;
          activeAcc.reset_horizon_weekly_argon_text = singleQuota.reset_horizon_weekly_argon_text;
        }
      }
    } catch {}
  }

  const quotaText = formatQuotaInfo(activeAcc);
  const switchMenuItems = buildSwitchMenuItems(accountsList, activeAccount, (email) => {
    switchAccountViaHttp(email);
  }, 0.10, 0.05, fleetFetched);

  return {
    activeAlias,
    quotaInfo: quotaText,
    switchMenuItems,
    accountsCount: accountsList.length,
  };
}

async function updateTrayMenu(options = {}) {
  if (isUpdatingTrayMenu || isQuitting || isTearingDown) return;
  if (!tray || (typeof tray.isDestroyed === 'function' && tray.isDestroyed())) {
    return;
  }
  isUpdatingTrayMenu = true;
  try {
    const state = await fetchTrayMenuData();
    applyTrayMenuState({
      ...state,
      force: Boolean(options && options.force),
    });
  } catch (err) {
    console.error('[Tray] Error updating tray menu:', err);
  } finally {
    isUpdatingTrayMenu = false;
  }
}

async function createTray() {
  const trayIcon = getTrayIcon();
  if (!trayIcon) return;

  try {
    let initialState = {
      activeAlias: 'Not Logged In',
      quotaInfo: '5H: 0% | 7D: 0%',
      switchMenuItems: [],
      accountsCount: 0,
    };
    try {
      initialState = await fetchTrayMenuData();
    } catch (_) {}

    tray = new Tray(trayIcon);
    tray.setToolTip('Antigravity Swiss Knife');

    // Attach the live context menu synchronously right after Tray construction
    // so StatusNotifierItem registers with revision 1 already populated.
    applyTrayMenuState({
      ...initialState,
      force: true,
    });

    tray.on('click', async () => {
      await toggleOrShowMainWindow();
    });

    tray.on('double-click', async () => {
      await toggleOrShowMainWindow();
    });

    // Periodically refresh tray state
    if (trayMenuInterval) {
      clearInterval(trayMenuInterval);
    }
    trayMenuInterval = setInterval(updateTrayMenu, 15000);
  } catch (err) {
    console.warn('[Tray] Could not create system tray icon:', err.message);
  }
}

async function createWindow(options = {}) {
  const windowIcon = getWindowIcon();
  if (process.platform === 'darwin' && app.dock && windowIcon) {
    try {
      const docImg = nativeImage.createFromPath(windowIcon);
      if (!docImg.isEmpty()) {
        const size = docImg.getSize();
        const insetWidth = Math.round(size.width * 0.82);
        const insetHeight = Math.round(size.height * 0.82);
        const resized = docImg.resize({ width: insetWidth, height: insetHeight });
        app.dock.setIcon(resized);
      } else {
        app.dock.setIcon(windowIcon);
      }
    } catch (_) {
      try { app.dock.setIcon(windowIcon); } catch (__) {}
    }
  }
  const forceShow = Boolean(options && options.forceShow);
  const startMinimized = !forceShow && (
    process.argv.includes('--minimized') ||
    process.argv.includes('--hidden') ||
    (app && typeof app.getLoginItemSettings === 'function' && app.getLoginItemSettings().wasOpenedAsHidden)
  );

  // Disable default application menu bar
  Menu.setApplicationMenu(null);

  let initialX;
  let initialY;
  try {
    if (screen && typeof screen.getPrimaryDisplay === 'function') {
      const primaryDisplay = screen.getPrimaryDisplay();
      if (primaryDisplay && primaryDisplay.workArea) {
        const { x, y, width, height } = primaryDisplay.workArea;
        initialX = Math.round(x + (width - 1216) / 2);
        initialY = Math.round(y + (height - 684) / 2);
      }
    }
  } catch (_) {}

  mainWindow = new BrowserWindow({
    width: 1216,
    height: 684,
    minWidth: 1216,
    minHeight: 684,
    ...(typeof initialX === 'number' && typeof initialY === 'number'
      ? { x: initialX, y: initialY }
      : { center: true }),
    title: 'Antigravity Swiss Knife',
    icon: windowIcon,
    show: !startMinimized,
    autoHideMenuBar: true,
    backgroundColor: '#131314', // Google Gemini dark surface token
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: true,
    },
  });

  mainWindow.setMenu(null);

  let linuxActivationInFlight = false;
  const activateLinuxWindowOnDesktop = (attempt = 1) => {
    if (process.platform !== 'linux') return;
    if (!mainWindow || (typeof mainWindow.isDestroyed === 'function' && mainWindow.isDestroyed())) {
      linuxActivationInFlight = false;
      return;
    }
    if (attempt === 1) {
      if (linuxActivationInFlight) return;
      linuxActivationInFlight = true;
    }
    try {
      const { execFile } = require('child_process');
      execFile(
        'gdbus',
        [
          'call',
          '--session',
          '--dest',
          'org.gnome.Shell',
          '--object-path',
          '/org/gnome/Shell/Extensions/Windows',
          '--method',
          'org.gnome.Shell.Extensions.Windows.List',
        ],
        { timeout: 1000 },
        (err, stdout) => {
          let target = null;
          if (!err && stdout) {
            const match = stdout.match(/\('(\[.*\])',\)/s);
            if (match) {
              try {
                const windows = JSON.parse(match[1]);
                target = windows.find(
                  (w) =>
                    w.pid === process.pid ||
                    (typeof w.wm_class === 'string' && w.wm_class.toLowerCase() === 'antigravity-swiss-knife')
                );
              } catch (_) {}
            }
          }

          if (!target || !target.id) {
            if (attempt < 12) {
              const retryTimer = setTimeout(() => activateLinuxWindowOnDesktop(attempt + 1), 150);
              if (retryTimer && typeof retryTimer.unref === 'function') {
                retryTimer.unref();
              }
            } else {
              linuxActivationInFlight = false;
            }
            return;
          }

          const winIdStr = String(target.id);
          const callWindowsMethod = (methodName, cb) => {
            execFile(
              'gdbus',
              [
                'call',
                '--session',
                '--dest',
                'org.gnome.Shell',
                '--object-path',
                '/org/gnome/Shell/Extensions/Windows',
                '--method',
                `org.gnome.Shell.Extensions.Windows.${methodName}`,
                winIdStr,
              ],
              { timeout: 1000 },
              (mErr, mOut) => {
                if (typeof cb === 'function') cb(mErr, mOut);
              }
            );
          };

          callWindowsMethod('Unminimize', () => {
            callWindowsMethod('MakeAbove', () => {
              callWindowsMethod('Activate', () => {
                callWindowsMethod('Details', (dErr, dOut) => {
                  let isFocusedAndMapped = false;
                  if (!dErr && dOut) {
                    const dMatch = dOut.match(/\('(\{.*\})',\)/s);
                    if (dMatch) {
                      try {
                        const details = JSON.parse(dMatch[1]);
                        isFocusedAndMapped = details.focus === true && Number(details.width) > 0 && Number(details.height) > 0;
                      } catch (_) {}
                    }
                  }

                  if (isFocusedAndMapped) {
                    const unmakeTimer = setTimeout(() => {
                      callWindowsMethod('UnmakeAbove', () => {
                        linuxActivationInFlight = false;
                      });
                    }, 150);
                    if (unmakeTimer && typeof unmakeTimer.unref === 'function') {
                      unmakeTimer.unref();
                    }
                    return;
                  }

                  if (attempt < 6) {
                    const retryTimer = setTimeout(() => activateLinuxWindowOnDesktop(attempt + 1), 180);
                    if (retryTimer && typeof retryTimer.unref === 'function') {
                      retryTimer.unref();
                    }
                    return;
                  }

                  // If Mutter focus-stealing prevention withheld focus on a background
                  // DBus tray callback, keep MakeAbove active so the window remains
                  // visibly on top of maximized windows until the user focuses/interacts with it.
                  linuxActivationInFlight = false;
                  if (mainWindow && typeof mainWindow.once === 'function') {
                    const clearAboveOnInteract = () => {
                      const clearTimer = setTimeout(() => {
                        callWindowsMethod('UnmakeAbove');
                      }, 150);
                      if (clearTimer && typeof clearTimer.unref === 'function') {
                        clearTimer.unref();
                      }
                    };
                    mainWindow.once('focus', clearAboveOnInteract);
                    mainWindow.once('blur', clearAboveOnInteract);
                  }
                });
              });
            });
          });
        }
      );
    } catch (_) {
      linuxActivationInFlight = false;
    }
  };

  const revealMainWindow = () => {
    if (!mainWindow || (typeof mainWindow.isDestroyed === 'function' && mainWindow.isDestroyed())) return;
    try {
      if (typeof mainWindow.isMinimized === 'function' && mainWindow.isMinimized()) {
        mainWindow.restore();
      }
      if (typeof mainWindow.show === 'function') {
        mainWindow.show();
      }
      if (process.platform !== 'linux' && typeof mainWindow.setAlwaysOnTop === 'function') {
        mainWindow.setAlwaysOnTop(true);
        const timer = setTimeout(() => {
          try {
            if (mainWindow && (typeof mainWindow.isDestroyed !== 'function' || !mainWindow.isDestroyed())) {
              mainWindow.setAlwaysOnTop(false);
            }
          } catch (_) {}
        }, 150);
        if (timer && typeof timer.unref === 'function') {
          timer.unref();
        }
      }
      if (typeof mainWindow.moveTop === 'function') {
        mainWindow.moveTop();
      }
      if (typeof mainWindow.focus === 'function') {
        mainWindow.focus();
      }
      activateLinuxWindowOnDesktop(1);
    } catch (_) {}
  };

  if (!startMinimized) {
    revealMainWindow();
    if (typeof mainWindow.once === 'function') {
      mainWindow.once('ready-to-show', revealMainWindow);
    }
    if (mainWindow.webContents && typeof mainWindow.webContents.once === 'function') {
      mainWindow.webContents.once('did-finish-load', revealMainWindow);
    }
  }

  const isTilingWM = process.platform === 'linux' && Boolean(
    process.env.SWAYSOCK ||
    process.env.I3SOCK ||
    (process.env.XDG_CURRENT_DESKTOP && ['i3', 'sway', 'bspwm', 'xmonad', 'hyprland'].some((wm) => process.env.XDG_CURRENT_DESKTOP.toLowerCase().includes(wm)))
  );

  if (!isTilingWM) {
    // Lock 16:9 aspect ratio for windowed resizing
    mainWindow.setAspectRatio(16 / 9);

    // Manage aspect ratio locking across window states
    mainWindow.on('maximize', () => mainWindow.setAspectRatio(0));
    mainWindow.on('unmaximize', () => mainWindow.setAspectRatio(16 / 9));
    mainWindow.on('enter-full-screen', () => mainWindow.setAspectRatio(0));
    mainWindow.on('leave-full-screen', () => mainWindow.setAspectRatio(16 / 9));
  }

  // Intercept window close ('X'): default is to minimize to system tray unless close_to_tray is disabled
  mainWindow.on('close', (event) => {
    if (teardownCompleted || isTearingDown || isQuitting) return;
    const closeToTray = getCloseToTraySetting();
    const hasTray = Boolean(tray && (typeof tray.isDestroyed !== 'function' || !tray.isDestroyed()));
    if (closeToTray && hasTray) {
      if (process.platform === 'linux') {
        // On Linux (XWayland / GNOME Wayland), intercepting WM_DELETE_WINDOW with
        // mainWindow.hide() disposes the compositor window actor while EGL swaps
        // buffers, crashing the GPU process and leaving a dead X11 window handle.
        // Destroy the BrowserWindow cleanly on the next tick; window-all-closed
        // keeps the tray process alive and showOrCreateMainWindow() recreates the
        // window on demand.
        event.preventDefault();
        const winToDestroy = mainWindow;
        mainWindow = null;
        setImmediate(() => {
          try {
            if (winToDestroy && typeof winToDestroy.isDestroyed === 'function' && !winToDestroy.isDestroyed()) {
              winToDestroy.destroy();
            }
          } catch (_) {}
        });
        return;
      }
      event.preventDefault();
      mainWindow.hide();
      if (tray && process.platform === 'win32') {
        tray.displayBalloon({
          title: 'Antigravity Swiss Knife',
          content: 'Application minimized to system tray.',
        });
      }
    } else {
      requestAppQuit();
    }
  });

  // Security: Prevent window from navigating away from local daemon origin (P1-16)
  mainWindow.webContents.on('will-navigate', (event, url) => {
    if (!isTrustedDaemonOrigin(url)) {
      event.preventDefault();
      if (isSafeExternalUrl(url)) {
        shell.openExternal(url);
      }
    }
  });

  // Security: Prevent subframes from navigating away from local daemon origin (P1-16)
  mainWindow.webContents.on('will-frame-navigate', (event) => {
    if (event.url && (!isTrustedDaemonOrigin(event.url) || isDaemonApiUrl(event.url))) {
      event.preventDefault();
    }
  });

  // Security: Deny unauthorized webview tag creation (P1-16)
  mainWindow.webContents.on('will-attach-webview', (event) => {
    event.preventDefault();
  });

  // Open external links in default browser (strictly http: and https:) (P1-16)
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (isSafeExternalUrl(url)) {
      shell.openExternal(url);
    }
    return { action: 'deny' };
  });

  const showConnectingFallback = () => {
    if (!mainWindow || mainWindow.isDestroyed()) return;
    loadConnectingFallback(mainWindow);
  };

  let backgroundRetryTimer = null;
  const startBackgroundDaemonProbe = () => {
    if (backgroundRetryTimer) return;
    backgroundRetryTimer = setInterval(async () => {
      if (!mainWindow || mainWindow.isDestroyed()) {
        clearInterval(backgroundRetryTimer);
        backgroundRetryTimer = null;
        return;
      }
      const status = await probeDaemonStatus(600);
      if (status) {
        clearInterval(backgroundRetryTimer);
        backgroundRetryTimer = null;
        const currentUrl = mainWindow.webContents.getURL();
        if (!currentUrl.startsWith(DAEMON_URL)) {
          console.log('[Window] Daemon became healthy; loading daemon URL...');
          mainWindow.loadURL(DAEMON_URL).catch(() => {});
        }
      }
    }, 2000);
  };

  const loadDaemonUrlWithRetry = async (maxRetries = 10, delayMs = 1000) => {
    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      if (!mainWindow || mainWindow.isDestroyed()) return;
      try {
        await mainWindow.loadURL(DAEMON_URL);
        if (backgroundRetryTimer) {
          clearInterval(backgroundRetryTimer);
          backgroundRetryTimer = null;
        }
        if (!startMinimized) {
          revealMainWindow();
        }
        return;
      } catch (err) {
        if (attempt === maxRetries) {
          console.error('[Window] Exhausted retries loading daemon URL:', err.message);
          showConnectingFallback();
          startBackgroundDaemonProbe();
        } else {
          await new Promise((r) => setTimeout(r, delayMs));
        }
      }
    }
  };
  loadDaemonUrlWithRetry();

  mainWindow.webContents.on('did-fail-load', (_event, errorCode, _errorDescription, validatedURL) => {
    if (isTrustedDaemonOrigin(validatedURL) && !validatedURL.startsWith('data:')) {
      console.warn(`[Window] Navigation to ${validatedURL} failed (code ${errorCode}). Showing connecting fallback.`);
      showConnectingFallback();
      startBackgroundDaemonProbe();
    }
  });

  mainWindow.on('closed', () => {
    if (backgroundRetryTimer) {
      clearInterval(backgroundRetryTimer);
      backgroundRetryTimer = null;
    }
    mainWindow = null;
  });
}

function getLinuxAutostartPath() {
  return path.join(os.homedir(), '.config', 'autostart', 'antigravity-swiss-knife.desktop');
}

function getLinuxAutostartEnabled() {
  return fs.existsSync(getLinuxAutostartPath());
}

function setLinuxAutostartEnabled(enabled) {
  const autostartFile = getLinuxAutostartPath();
  const autostartDir = path.dirname(autostartFile);
  if (enabled) {
    if (!fs.existsSync(autostartDir)) {
      fs.mkdirSync(autostartDir, { recursive: true });
    }
    let binPath = process.execPath;
    const installedPath = path.join(os.homedir(), '.local', 'share', 'antigravity-swiss-knife', 'app', 'antigravity-swiss-knife');
    if (fs.existsSync(installedPath)) {
      binPath = installedPath;
    }
    const iconPath = path.join(os.homedir(), '.local', 'share', 'antigravity-swiss-knife', 'app', 'resources', 'assets', 'logo.png');
    const content = `[Desktop Entry]
Type=Application
Version=1.0
Name=Antigravity Swiss Knife
Comment=Google Antigravity Desktop Companion
Exec="${binPath}" --minimized
Icon=${fs.existsSync(iconPath) ? iconPath : 'antigravity-swiss-knife'}
Terminal=false
Categories=Development;
StartupNotify=false
X-GNOME-Autostart-enabled=true
`;
    fs.writeFileSync(autostartFile, content, 'utf8');
    return true;
  } else {
    if (fs.existsSync(autostartFile)) {
      try {
        fs.unlinkSync(autostartFile);
      } catch (_) {}
    }
    return false;
  }
}

// Setup IPC handlers
function registerIpcHandlers() {
  ipcMain.handle('desktop:get-startup-setting', async (event) => {
    if (!isTrustedSender(event)) {
      return { openAtLogin: false, openAsHidden: false, platform: process.platform };
    }
    try {
      if (process.platform === 'linux') {
        const enabled = getLinuxAutostartEnabled();
        return {
          openAtLogin: enabled,
          openAsHidden: true,
          platform: 'linux',
        };
      }
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

  ipcMain.handle('desktop:set-startup-setting', async (event, enabled) => {
    if (!isTrustedSender(event)) {
      return { success: false, error: 'Untrusted IPC sender' };
    }
    try {
      const isEnabled = typeof enabled === 'object' && enabled !== null
        ? Boolean(enabled.enabled)
        : Boolean(enabled);

      if (process.platform === 'linux') {
        const active = setLinuxAutostartEnabled(isEnabled);
        return { success: true, openAtLogin: active };
      }

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

  ipcMain.handle('desktop:get-close-to-tray-setting', async (event) => {
    if (!isTrustedSender(event)) {
      return { closeToTray: false };
    }
    return { closeToTray: getCloseToTraySetting() };
  });

  ipcMain.handle('desktop:set-close-to-tray-setting', async (event, enabled) => {
    if (!isTrustedSender(event)) {
      return { success: false, closeToTray: false };
    }
    const isEnabled = typeof enabled === 'object' && enabled !== null
      ? Boolean(enabled.enabled)
      : Boolean(enabled);
    const success = setCloseToTraySetting(isEnabled);
    return { success, closeToTray: isEnabled };
  });

  ipcMain.handle('desktop:get-runtime-mode-setting', async (event) => {
    if (!isTrustedSender(event)) {
      return { runtimeMode: 'app' };
    }
    return { runtimeMode: getRuntimeModeSetting() };
  });

  ipcMain.handle('desktop:set-runtime-mode-setting', async (event, mode) => {
    if (!isTrustedSender(event)) {
      return { success: false, runtimeMode: 'app' };
    }
    const targetMode = typeof mode === 'object' && mode !== null
      ? (mode.runtimeMode || mode.runtime_mode || 'app')
      : (mode === 'daemon' ? 'daemon' : 'app');
    const success = setRuntimeModeSetting(targetMode);
    return { success, runtimeMode: targetMode };
  });

  ipcMain.handle('desktop:start-daemon', async (event) => {
    if (!isTrustedSender(event)) {
      return { success: false, error: 'Untrusted IPC sender' };
    }
    try {
      console.log('[IPC] Manual Start Daemon requested...');
      await daemonManager.start();
      const status = await daemonManager.checkStatus(2000);
      return { success: true, status };
    } catch (err) {
      console.error('[IPC] start-daemon failed:', err);
      return { success: false, error: err.message };
    }
  });

  ipcMain.handle('desktop:swiss-daemon-request', async (event, request) => {
    if (!isTrustedSender(event)) {
      return { error: { code: 'UNTRUSTED_SENDER', message: 'Untrusted IPC sender' } };
    }
    try {
      return await daemonBroker.request(request);
    } catch (error) {
      const code = error && typeof error.code === 'string' ? error.code : 'DAEMON_BROKER_ERROR';
      const message = error && typeof error.message === 'string' ? error.message : 'Daemon broker request failed';
      return { error: { code, message } };
    }
  });

  ipcMain.handle('desktop:retry-daemon-connection', async (event) => {
    if (!isTrustedSender(event)) {
      return { success: false, error: 'Untrusted recovery request' };
    }
    return recoverDaemonConnection();
  });

  ipcMain.on('desktop:quit-app', (event) => {
    if (!isTrustedSender(event)) return;
    console.log('[App] Quit requested via IPC');
    requestAppQuit();
  });

  ipcMain.handle('desktop:notify', async (event, params) => {
    if (!isTrustedSender(event)) {
      return false;
    }
    if (Notification.isSupported()) {
      const rawTitle = typeof params?.title === 'string' ? params.title.trim() : '';
      const rawBody = typeof params?.body === 'string' ? params.body.trim() : '';
      const title = `[Antigravity Swiss Knife] ${rawTitle.slice(0, 100)}`;
      const body = rawBody.slice(0, 300);
      new Notification({ title, body, icon: getIconPath() }).show();
      return true;
    }
    return false;
  });

  ipcMain.handle('desktop:open-external', async (event, url) => {
    if (!isTrustedSender(event)) {
      return false;
    }
    try {
      if (typeof url !== 'string') return false;
      const parsed = new URL(url);
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
        console.warn(`[IPC] desktop:open-external rejected non-http(s) scheme: ${parsed.protocol}`);
        return false;
      }
      await shell.openExternal(url);
      return true;
    } catch {
      return false;
    }
  });

  ipcMain.handle('desktop:write-clipboard', async (event, text) => {
    if (!isTrustedSender(event)) {
      return false;
    }
    if (typeof text !== 'string' || text.length === 0 || text.length > 16384) {
      return false;
    }
    try {
      clipboard.writeText(text);
      return true;
    } catch {
      return false;
    }
  });

  ipcMain.handle('desktop:focus-window', async (event) => {
    if (!isTrustedSender(event)) {
      return false;
    }
    await showOrCreateMainWindow();
    return true;
  });

  ipcMain.handle('desktop:select-path', async (event, options) => {
    if (!isTrustedSender(event)) {
      return null;
    }
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

  ipcMain.handle('desktop:launch-antigravity', async (event) => {
    if (!isTrustedSender(event)) {
      return { success: false, error: 'Untrusted IPC sender' };
    }
    try {
      let res = await fetch(`${DAEMON_URL}/api/desktop/launch`, {
        method: 'POST',
        headers: daemonAuthHeaders(),
      });
      if (res.status === 401) {
        // Token rotated since the last read; re-read the file and retry once.
        res = await fetch(`${DAEMON_URL}/api/desktop/launch`, {
          method: 'POST',
          headers: daemonAuthHeaders({}, 0),
        });
      }
      if (!res.ok) {
        const text = await res.text();
        throw new Error(`daemon responded ${res.status}: ${text.slice(0, 200)}`);
      }
      return await res.json();
    } catch (err) {
      console.warn('[Electron] Failed to trigger relaunch via daemon, attempting fallback:', err.message);
      try {
        const { spawn } = require('child_process');
        let cmd = 'antigravity';
        let args = [];
        if (process.platform === 'darwin') {
          cmd = 'open';
          args = ['-a', 'Antigravity'];
        } else if (process.platform === 'linux') {
          const candidates = [
            '/opt/Antigravity/antigravity',
            '/usr/bin/antigravity',
            '/usr/local/bin/antigravity',
            path.join(os.homedir(), '.local', 'bin', 'antigravity'),
            path.join(os.homedir(), '.antigravity', 'bin', 'antigravity'),
          ];
          for (const cand of candidates) {
            if (fs.existsSync(cand)) {
              cmd = cand;
              break;
            }
          }
        }
        const child = spawn(cmd, args, { detached: true, stdio: 'ignore', windowsHide: true });
        child.unref();
        return { success: true, message: 'Antigravity process launched' };
      } catch (spawnErr) {
        return { success: false, error: spawnErr.message };
      }
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
      if (minWidth !== 1216 || minHeight !== 684) {
        console.error(`[E2E-TEST] Minimum size mismatch! Expected 1216x684, got: ${minWidth}x${minHeight}`);
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

      // Verify current window dimensions (either exact 1216x684 or valid 16:9 4px-aligned multiple)
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

      console.log('[E2E-TEST] Window geometry verified: 1216x684 (16:9 aspect ratio, 4px aligned)');
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
      setCloseToTraySetting(false);
      if (getCloseToTraySetting() !== false) {
        console.error('[E2E-TEST] Failed to set closeToTray to false');
        process.exit(1);
      }
      setCloseToTraySetting(true);
      if (getCloseToTraySetting() !== true) {
        console.error('[E2E-TEST] Failed to set closeToTray to true');
        process.exit(1);
      }
      setCloseToTraySetting(initClose);
      console.log('[E2E-TEST] CloseToTray IPC and persistence verified: OK');

      // Test runtimeMode setting persistence & IPC logic
      console.log('[E2E-TEST] Testing runtimeMode setting...');
      const initMode = getRuntimeModeSetting();
      console.log('[E2E-TEST] Initial runtimeMode:', initMode);
      setRuntimeModeSetting('daemon');
      if (getRuntimeModeSetting() !== 'daemon') {
        console.error('[E2E-TEST] Failed to set runtimeMode to daemon');
        process.exit(1);
      }
      setRuntimeModeSetting('app');
      if (getRuntimeModeSetting() !== 'app') {
        console.error('[E2E-TEST] Failed to set runtimeMode to app');
        process.exit(1);
      }
      setRuntimeModeSetting(initMode);
      console.log('[E2E-TEST] RuntimeMode IPC and persistence verified: OK');

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
if (app && typeof app.whenReady === 'function') {
  app.whenReady().then(async () => {
    Menu.setApplicationMenu(null);
    registerIpcHandlers();
    attachDaemonAuthHeaders();
    try {
      await daemonManager.start();
    } catch (err) {
      console.error('[App] Failed to start Go daemon sidecar:', err);
    }
    await createTray();
    const startHiddenOnBoot = (
      process.argv.includes('--minimized') ||
      process.argv.includes('--hidden') ||
      (app && typeof app.getLoginItemSettings === 'function' && app.getLoginItemSettings().wasOpenedAsHidden)
    ) && process.env.TEST_DESKTOP_E2E !== '1';

    if (!(process.platform === 'linux' && startHiddenOnBoot)) {
      await createWindow();
    }

    app.on('activate', async () => {
      await showOrCreateMainWindow();
    });

    if (process.env.TEST_DESKTOP_E2E === '1') {
      runE2eVerification();
    }
  });
}

// Full application quit hook
if (app && typeof app.on === 'function') {
  app.on('before-quit', async (event) => {
    isQuitting = true;
    if (teardownCompleted) {
      return;
    }
    event.preventDefault();
    if (isTearingDown) {
      return;
    }
    isTearingDown = true;

    if (trayMenuInterval) {
      clearInterval(trayMenuInterval);
      trayMenuInterval = null;
    }

    // Safety fallback: ensure process never hangs indefinitely on quit
    setTimeout(() => {
      console.warn('[App] Hard quit fallback triggered');
      app.exit(0);
    }, 5500).unref();

    try {
      await Promise.race([
        teardownDaemonForAppExit(),
        new Promise((_, reject) =>
          setTimeout(() => reject(new Error('Teardown timed out after 4500ms')), 4500)
        ),
      ]);
    } catch (err) {
      const detail = err && err.message ? err.message : String(err);
      console.warn('[App] Warning during daemon stop on before-quit:', detail);
      try {
        if (dialog && typeof dialog.showErrorBox === 'function') {
          dialog.showErrorBox('Antigravity Swiss exit cleanup failed', detail);
        }
      } catch (dialogErr) {
        console.error('[App] Failed to show teardown error:', dialogErr.message || dialogErr);
      }
    } finally {
      teardownCompleted = true;
      try {
        if (mainWindow && typeof mainWindow.isDestroyed === 'function' && !mainWindow.isDestroyed()) {
          mainWindow.destroy();
        }
      } catch (_) {}
      try {
        if (tray && typeof tray.isDestroyed === 'function' && !tray.isDestroyed()) {
          tray.destroy();
        }
      } catch (_) {}
    }

    if (app && typeof app.exit === 'function') {
      app.exit(0);
    } else {
      app.quit();
    }
  });

  app.on('window-all-closed', () => {
    if (isQuitting || isTearingDown || teardownCompleted) {
      return;
    }
    const closeToTray = getCloseToTraySetting();
    const hasTray = Boolean(tray && (typeof tray.isDestroyed !== 'function' || !tray.isDestroyed()));
    if (!closeToTray || !hasTray) {
      requestAppQuit();
    }
  });
}

// Operating System termination signal handlers
const handleExitSignal = async (signal) => {
  console.log(`[App] Received OS signal ${signal}. Initiating graceful teardown...`);
  if (isStoppingDaemon) return;
  isStoppingDaemon = true;
  isQuitting = true;
  try {
    await teardownDaemonForAppExit();
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
    await teardownDaemonForAppExit();
  } catch {}
  process.exit(1);
});

// Module Exports
module.exports = {
  daemonManager,
  DaemonManager,
  probeDaemonStatus,
  buildConnectingFallbackHtml,
  recoverDaemonConnection,
  createWindow,
  showOrCreateMainWindow,
  toggleOrShowMainWindow,
  isMainWindowUsable,
  requestAppQuit,
  createTray,
  updateTrayMenu,
  computeTrayMenuSignature,
  applyTrayMenuState,
  registerIpcHandlers,
  getSwissConfigDir,
  getCloseToTraySetting,
  setCloseToTraySetting,
  getRuntimeModeSetting,
  setRuntimeModeSetting,
  teardownDaemonForAppExit,
  requestDaemonJsonWithAuth,
  isNativeRestoreResponse,
  isAccountExcludedFromSwitchMenu,
  rankAndFilterAccountsForTray,
  buildSwitchMenuItems,
  formatQuotaPercentage,
  formatQuotaInfo,
  resolveActiveAccount,
  resolveActiveAccountAlias,
  buildTrayMenuTemplate,
  switchAccountViaHttp,
  getMainWindow: () => mainWindow,
  getTray: () => tray,
  DAEMON_URL,
  DAEMON_PORT,
  DAEMON_HOST,
  isTrustedDaemonOrigin,
  isSafeExternalUrl,
  isTrustedSender,
  loadConnectingFallback,
};
