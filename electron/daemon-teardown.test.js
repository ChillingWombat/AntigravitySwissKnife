const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');

const configDir = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-daemon-teardown-'));
process.env.ANTIGRAVITY_SWISS_CONFIG_DIR = configDir;
const tokenPath = path.join(configDir, 'session.token');
fs.writeFileSync(tokenPath, 'current-token', 'utf8');

const Module = require('node:module');
const noop = () => {};
const app = {
  name: 'Antigravity Swiss Test',
  isPackaged: false,
  commandLine: { appendSwitch: noop },
  on: () => app,
  once: () => app,
  whenReady: () => new Promise(() => {}),
  requestSingleInstanceLock: () => true,
  quit: noop,
  exit: noop,
  getPath: () => configDir,
  getAppPath: () => process.cwd(),
  getVersion: () => 'test',
  setName: noop,
  setAppUserModelId: noop,
};
const electronStub = {
  app,
  BrowserWindow: Object.assign(function BrowserWindow() {}, {
    getAllWindows: () => [],
    fromWebContents: () => null,
  }),
  Tray: function Tray() {},
  Menu: { buildFromTemplate: () => ({}), setApplicationMenu: noop },
  ipcMain: { handle: noop, on: noop, removeHandler: noop },
  shell: { openExternal: async () => {}, showItemInFolder: noop },
  Notification: function Notification() {},
  nativeImage: { createFromPath: () => ({ isEmpty: () => true }), createEmpty: () => ({}) },
  session: { defaultSession: { webRequest: { onBeforeSendHeaders: noop } } },
  clipboard: { readText: () => '', writeText: noop },
  screen: { getPrimaryDisplay: () => ({ workAreaSize: { width: 1, height: 1 } }), getAllDisplays: () => [] },
  dialog: { showErrorBox: noop },
};
electronStub.Notification.isSupported = () => false;

const originalLoad = Module._load;
Module._load = function loadWithElectronStub(request, parent, isMain) {
  if (request === 'electron') return electronStub;
  return originalLoad.call(this, request, parent, isMain);
};
let daemonClient;
try {
  daemonClient = require('./main');
} finally {
  Module._load = originalLoad;
}
const {
  requestDaemonJsonWithAuth,
  isNativeRestoreResponse,
  teardownDaemonForAppExit,
} = daemonClient;

const restoreResponse = { success: true, restored_native: true };

async function withServer(handler, run) {
  const server = http.createServer(handler);
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  const baseUrl = `http://127.0.0.1:${address.port}`;
  try {
    return await run(baseUrl);
  } finally {
    if (typeof server.closeAllConnections === 'function') {
      server.closeAllConnections();
    }
    await new Promise((resolve) => server.close(resolve));
  }
}

function respondJson(res, status, payload) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(payload));
}

test('daemon teardown authenticates and rejects failed restoration', async (t) => {
  await t.test('attaches the current daemon session token', async () => {
    let receivedToken = '';
    await withServer((req, res) => {
      receivedToken = req.headers['x-session-token'];
      respondJson(res, 200, restoreResponse);
    }, async (baseUrl) => {
      const payload = await requestDaemonJsonWithAuth('/api/custom_models/restore_native', {
        baseUrl,
        validateResponse: isNativeRestoreResponse,
      });
      assert.deepEqual(payload, restoreResponse);
    });
    assert.equal(receivedToken, 'current-token');
  });

  await t.test('refreshes the token once after a 401', async () => {
    let attempts = 0;
    const receivedTokens = [];
    await withServer((req, res) => {
      attempts += 1;
      receivedTokens.push(req.headers['x-session-token']);
      if (attempts === 1) {
        fs.writeFileSync(tokenPath, 'refreshed-token', 'utf8');
        respondJson(res, 401, { error: 'expired token' });
        return;
      }
      respondJson(res, 200, restoreResponse);
    }, async (baseUrl) => {
      const payload = await requestDaemonJsonWithAuth('/api/custom_models/restore_native', {
        baseUrl,
        validateResponse: isNativeRestoreResponse,
      });
      assert.deepEqual(payload, restoreResponse);
    });
    assert.deepEqual(receivedTokens, ['current-token', 'refreshed-token']);
  });

  await t.test('fails on a second 401 without another retry', async () => {
    let attempts = 0;
    await withServer((req, res) => {
      attempts += 1;
      respondJson(res, 401, { error: 'still unauthorized' });
    }, async (baseUrl) => {
      await assert.rejects(
        requestDaemonJsonWithAuth('/api/custom_models/restore_native', {
          baseUrl,
          authHeaders: () => ({ 'X-Session-Token': 'fixed-token' }),
        }),
        /401/
      );
    });
    assert.equal(attempts, 2);
  });

  await t.test('times out without resolving early', async () => {
    await withServer(() => {}, async (baseUrl) => {
      await assert.rejects(
        requestDaemonJsonWithAuth('/api/custom_models/restore_native', {
          baseUrl,
          timeoutMs: 40,
          authHeaders: () => ({ 'X-Session-Token': 'fixed-token' }),
        }),
        /timed out/
      );
    });
  });

  await t.test('rejects non-2xx responses', async () => {
    await withServer((req, res) => {
      respondJson(res, 500, { error: 'restore failed' });
    }, async (baseUrl) => {
      await assert.rejects(
        requestDaemonJsonWithAuth('/api/custom_models/restore_native', {
          baseUrl,
          authHeaders: () => ({ 'X-Session-Token': 'fixed-token' }),
        }),
        /500/
      );
    });
  });

  await t.test('rejects malformed JSON', async () => {
    await withServer((req, res) => {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end('{not-json');
    }, async (baseUrl) => {
      await assert.rejects(
        requestDaemonJsonWithAuth('/api/custom_models/restore_native', {
          baseUrl,
          authHeaders: () => ({ 'X-Session-Token': 'fixed-token' }),
        }),
        /invalid JSON/
      );
    });
  });

  await t.test('rejects a wrong restore contract', async () => {
    await withServer((req, res) => {
      respondJson(res, 200, { success: true, restored_native: false });
    }, async (baseUrl) => {
      await assert.rejects(
        requestDaemonJsonWithAuth('/api/custom_models/restore_native', {
          baseUrl,
          authHeaders: () => ({ 'X-Session-Token': 'fixed-token' }),
          validateResponse: isNativeRestoreResponse,
        }),
        /unexpected response contract/
      );
    });
  });

  await t.test('always stops the daemon and rethrows restoration failure', async () => {
    const restoreError = new Error('native restoration failed');
    let stopped = false;
    await assert.rejects(
      teardownDaemonForAppExit({
        mode: 'app',
        requestRestore: async () => { throw restoreError; },
        stopDaemon: async () => { stopped = true; },
        detachDaemon: () => { throw new Error('detach must not run in app mode'); },
      }),
      (err) => err === restoreError
    );
    assert.equal(stopped, true);
  });
});
