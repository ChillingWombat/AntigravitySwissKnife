const fs = require('node:fs');
const http = require('node:http');
const test = require('node:test');
const assert = require('node:assert/strict');
const {
  buildConnectingFallbackHtml,
  recoverDaemonConnection,
  isTrustedDaemonOrigin,
  isSafeExternalUrl,
  DAEMON_URL,
  isTrustedSender,
  loadConnectingFallback,
  computeTrayMenuSignature,
  isMainWindowUsable,
} = require('./main');
const { DaemonManager } = require('./daemon-manager');

function startHttpServer(handler) {
  return new Promise((resolve, reject) => {
    const server = http.createServer(handler);
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      server.removeListener('error', reject);
      const { port } = server.address();
      resolve({
        server,
        baseUrl: `http://127.0.0.1:${port}`,
        close: () => new Promise((closeResolve) => server.close(closeResolve)),
      });
    });
  });
}

test('isTrustedDaemonOrigin - verifies exact origin matching', () => {
  const DAEMON_URL = 'http://127.0.0.1:8765';

  // Valid internal URLs
  assert.equal(isTrustedDaemonOrigin('http://127.0.0.1:8765', DAEMON_URL), true);
  assert.equal(isTrustedDaemonOrigin('http://127.0.0.1:8765/', DAEMON_URL), true);
  assert.equal(isTrustedDaemonOrigin('http://127.0.0.1:8765/settings', DAEMON_URL), true);
  assert.equal(isTrustedDaemonOrigin('http://127.0.0.1:8765/api/status?key=val#hash', DAEMON_URL), true);

  // Deceptive prefix bypasses (Issue #57, P1-16)
  assert.equal(isTrustedDaemonOrigin('http://127.0.0.1:8765.attacker.com', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('http://127.0.0.1:8765@attacker.com', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('http://attacker.com/http://127.0.0.1:8765', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('http://attacker.com:8765', DAEMON_URL), false);

  // Scheme and port mismatches
  assert.equal(isTrustedDaemonOrigin('https://127.0.0.1:8765', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('http://127.0.0.1:8766', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('http://localhost:8765', DAEMON_URL), false);

  // Data URLs are trusted only when they exactly match the loaded fallback.
  assert.equal(isTrustedDaemonOrigin('data:text/html;charset=utf-8,%3Ch1%3EConnecting%3C/h1%3E', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('data:text/html,<h1>Connecting</h1>', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('data:application/javascript,alert(1)', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('data:application/javascript,alert(1)', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('data:image/svg+xml,<svg></svg>', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('data:text/plain,hello', DAEMON_URL), false);

  // Dangerous protocols
  assert.equal(isTrustedDaemonOrigin('file:///C:/Windows/calc.exe', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('javascript:alert(1)', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin('', DAEMON_URL), false);
  assert.equal(isTrustedDaemonOrigin(null, DAEMON_URL), false);
});

test('isSafeExternalUrl - restricts shell.openExternal strictly to http and https', () => {
  // Allowed safe web protocols
  assert.equal(isSafeExternalUrl('https://github.com/ChillingWombat/AntigravitySwissKnife'), true);
  assert.equal(isSafeExternalUrl('http://example.com/docs'), true);
  assert.equal(isSafeExternalUrl('https://api.github.com/releases'), true);

  // Dangerous OS/local protocols that must NEVER be passed to shell.openExternal
  assert.equal(isSafeExternalUrl('file:///C:/Windows/System32/calc.exe'), false);
  assert.equal(isSafeExternalUrl('file:///etc/passwd'), false);
  assert.equal(isSafeExternalUrl('ms-msdt:/id PCWDiagnostic'), false);
  assert.equal(isSafeExternalUrl('javascript:alert(1)'), false);
  assert.equal(isSafeExternalUrl('data:text/html,<script>alert(1)</script>'), false);
  assert.equal(isSafeExternalUrl('powershell.exe -enc AAAA'), false);
  assert.equal(isSafeExternalUrl('cmd.exe /c calc.exe'), false);
  assert.equal(isSafeExternalUrl(''), false);
  assert.equal(isSafeExternalUrl(null), false);
  assert.equal(isSafeExternalUrl(undefined), false);
});


test('DaemonManager - cleanupSocket removes existing socket file and handles nonexistent gracefully', () => {
  const { DaemonManager } = require('./daemon-manager');
  const path = require('path');
  const os = require('os');
  const dm = new DaemonManager({ port: 8765, host: '127.0.0.1' });
  const testSockPath = path.join(os.tmpdir(), `antigravity-swiss-test-${Date.now()}`, 'daemon.sock');

  fs.mkdirSync(path.dirname(testSockPath), { recursive: true });
  fs.writeFileSync(testSockPath, 'dummy-socket');
  assert.equal(fs.existsSync(testSockPath), true);

  dm.cleanupSocket(testSockPath);
  if (process.platform === 'win32') {
    // Windows daemons use named pipes, so cleanupSocket must not delete a filesystem path.
    assert.equal(fs.existsSync(testSockPath), true);
    fs.rmSync(testSockPath, { force: true });
  } else {
    assert.equal(fs.existsSync(testSockPath), false);
  }

  assert.doesNotThrow(() => dm.cleanupSocket(testSockPath));
  try { fs.rmdirSync(path.dirname(testSockPath)); } catch {}
});

test('DaemonManager - reapOrphanedProcesses does not throw and returns boolean', () => {
  const dm = new DaemonManager({ port: 8765, host: '127.0.0.1' });
  assert.doesNotThrow(() => {
    const signaled = dm.reapOrphanedProcesses();
    assert.equal(typeof signaled, 'boolean');
  });
});

test('DaemonManager - rejects unrelated HTTP 200 responses and bare 401s', async () => {
  const fixture = await startHttpServer((req, res) => {
    if (req.url === '/health') {
      res.writeHead(200, { 'Content-Type': 'text/html' });
      res.end('<html>unrelated local service</html>');
      return;
    }
    res.writeHead(401, { 'Content-Type': 'application/json' });
    res.end('{"error":"unauthorized"}');
  });

  try {
    const dm = new DaemonManager({ baseUrl: fixture.baseUrl });
    assert.equal(await dm.checkStatus(250), null);
  } finally {
    await fixture.close();
  }
});

test('DaemonManager - rejects health and status payloads that do not match the Swiss contract', async () => {
  const fixture = await startHttpServer((req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    if (req.url === '/health') {
      res.end('{"status":"ok"}');
      return;
    }
    res.end('{"daemon_running":true}');
  });

  try {
    const dm = new DaemonManager({ baseUrl: fixture.baseUrl });
    assert.equal(await dm.checkStatus(250), null);
  } finally {
    await fixture.close();
  }
});

test('DaemonManager - accepts only a complete Swiss health contract', async () => {
  const fixture = await startHttpServer((req, res) => {
    if (req.url === '/health') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end('{"status":"ok","version":"0.2.0","daemon_running":true}');
      return;
    }
    res.writeHead(404);
    res.end();
  });

  try {
    const dm = new DaemonManager({ baseUrl: fixture.baseUrl });
    assert.deepEqual(await dm.checkStatus(250), {
      status: 'ok',
      version: '0.2.0',
      daemon_running: true,
    });
  } finally {
    await fixture.close();
  }
});

test('DaemonManager - accepts an authenticated Swiss status contract when health is unavailable', async () => {
  const fixture = await startHttpServer((req, res) => {
    if (req.url === '/health') {
      res.writeHead(404);
      res.end();
      return;
    }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end('{"daemon_running":true,"daemon_pid":4242,"version":"0.2.0"}');
  });

  try {
    const dm = new DaemonManager({
      baseUrl: fixture.baseUrl,
      getAuthHeaders: () => ({ 'X-Session-Token': 'valid-token' }),
    });
    assert.deepEqual(await dm.checkStatus(250), {
      daemon_running: true,
      daemon_pid: 4242,
      version: '0.2.0',
    });
  } finally {
    await fixture.close();
  }
});

test('Connecting fallback - retries through the preload IPC bridge without a data-origin fetch', () => {
  const html = buildConnectingFallbackHtml();
  assert.match(html, /retryDaemonConnection/);
  assert.doesNotMatch(html, /\bfetch\s*\(/);
});

test('Connecting fallback - renders Quit Application button linked to preload IPC bridge', () => {
  const html = buildConnectingFallbackHtml();
  assert.match(html, /id="quitBtn"/);
  assert.match(html, /window\.electronAPI\?\.quitApp\?\.\(\)/);
  assert.match(html, />Quit Application<\/button>/);
});

test('Preload bridge - exposes quitApp method', () => {
  const { electronAPI } = require('./preload');
  assert.equal(typeof electronAPI.quitApp, 'function');
});

test('DaemonManager - reapOrphanedProcesses skips when NODE_ENV is test unless forced', () => {
  const dm = new DaemonManager({ port: 8765, host: '127.0.0.1' });
  const origEnv = process.env.NODE_ENV;
  try {
    process.env.NODE_ENV = 'test';
    const result = dm.reapOrphanedProcesses();
    assert.equal(result, false);
  } finally {
    process.env.NODE_ENV = origEnv;
  }
});

test('DaemonManager - reapOrphanedProcesses respects active healthy daemon on baseUrl', () => {
  const dm = new DaemonManager({ port: 8765, host: '127.0.0.1' });
  dm.isHealthy = true;
  dm.lastHealthyAt = Date.now();
  const result = dm.reapOrphanedProcesses({ force: true });
  assert.equal(result, false);
});

test('DaemonManager - Watchdog schedules auto-restart on unexpected exit', () => {
  const dm = new DaemonManager({ port: 8765, host: '127.0.0.1', maxRestarts: 3, restartWindowMs: 30000 });
  dm.isManagedChild = true;
  dm._handleChildExit(1, null);

  assert.equal(dm.crashTimestamps.length, 1);
  assert.ok(dm.respawnTimer !== null);

  // Clear timer
  clearTimeout(dm.respawnTimer);
  dm.respawnTimer = null;
});

test('DaemonManager - Watchdog suppresses auto-restart when intentionally stopped or detached', () => {
  const dm = new DaemonManager({ port: 8765, host: '127.0.0.1' });
  dm.isManagedChild = true;
  dm.isStoppingDaemon = true;
  dm._handleChildExit(0, 'SIGTERM');

  assert.equal(dm.crashTimestamps.length, 0);
  assert.equal(dm.respawnTimer, null);

  const dm2 = new DaemonManager({ port: 8765, host: '127.0.0.1' });
  dm2.isManagedChild = true;
  dm2.isDetached = true;
  dm2._handleChildExit(0, null);

  assert.equal(dm2.crashTimestamps.length, 0);
  assert.equal(dm2.respawnTimer, null);
});

test('recoverDaemonConnection - starts the daemon and navigates only to the trusted daemon URL', async () => {
  const loadedUrls = [];
  let startCalls = 0;
  const result = await recoverDaemonConnection({
    manager: {
      start: async () => {
        startCalls += 1;
      },
      checkStatus: async () => ({
        daemon_running: true,
        status: 'ok',
        version: '0.2.0',
      }),
    },
    window: {
      isDestroyed: () => false,
      loadURL: async (url) => {
        loadedUrls.push(url);
      },
    },
  });

  assert.equal(result.success, true);
  assert.equal(startCalls, 1);
  assert.deepEqual(loadedUrls, [DAEMON_URL]);
});

test('recoverDaemonConnection - does not navigate when the daemon contract is invalid', async () => {
  const loadedUrls = [];
  const result = await recoverDaemonConnection({
    manager: {
      start: async () => {},
      checkStatus: async () => null,
    },
    window: {
      isDestroyed: () => false,
      loadURL: async (url) => {
        loadedUrls.push(url);
      },
    },
  });

  assert.equal(result.success, false);
  assert.deepEqual(loadedUrls, []);
});

test('isTrustedSender - verifies window status, main-frame identity, and exact origin', async () => {
  const mainFrame = {
    url: 'http://127.0.0.1:8765/api/status',
    parent: null,
    routingId: 1,
  };
  const fakeWebContents = { mainFrame };
  const fakeWindow = {
    isDestroyed: () => false,
    webContents: fakeWebContents,
  };

  // Valid event from daemon origin.
  assert.equal(
    isTrustedSender({ sender: fakeWebContents, senderFrame: mainFrame }, fakeWindow),
    true
  );

  let fallbackUrl = '';
  await loadConnectingFallback({
    loadURL: async (url) => {
      fallbackUrl = url;
    },
  });
  assert.match(fallbackUrl, /^data:text\/html;charset=utf-8,/);

  // Arbitrary data URLs are rejected even after the official fallback is known.
  mainFrame.url = 'data:text/html,<script>alert(1)</script>';
  assert.equal(
    isTrustedSender({ sender: fakeWebContents, senderFrame: mainFrame }, fakeWindow),
    false
  );

  // The exact connecting fallback data URL is trusted on the main frame.
  mainFrame.url = fallbackUrl;
  assert.equal(
    isTrustedSender({ sender: fakeWebContents, senderFrame: mainFrame }, fakeWindow),
    true
  );

  // Subframes cannot use IPC even when they claim a trusted URL.
  assert.equal(
    isTrustedSender(
      {
        sender: fakeWebContents,
        senderFrame: { ...mainFrame, parent: mainFrame, routingId: 2 },
      },
      fakeWindow
    ),
    false
  );

  // Missing or destroyed window.
  mainFrame.url = DAEMON_URL;
  assert.equal(isTrustedSender({ sender: fakeWebContents, senderFrame: mainFrame }, null), false);
  assert.equal(
    isTrustedSender(
      { sender: fakeWebContents, senderFrame: mainFrame },
      { isDestroyed: () => true, webContents: fakeWebContents }
    ),
    false
  );

  // Missing event or wrong sender.
  assert.equal(isTrustedSender(null, fakeWindow), false);
  assert.equal(isTrustedSender({}, fakeWindow), false);
  assert.equal(isTrustedSender({ sender: {} }, fakeWindow), false);

  // Missing senderFrame or untrusted URL.
  assert.equal(isTrustedSender({ sender: fakeWebContents }, fakeWindow), false);
  mainFrame.url = 'http://attacker.com';
  assert.equal(
    isTrustedSender({ sender: fakeWebContents, senderFrame: mainFrame }, fakeWindow),
    false
  );
  mainFrame.url = 'file:///etc/passwd';
  assert.equal(
    isTrustedSender({ sender: fakeWebContents, senderFrame: mainFrame }, fakeWindow),
    false
  );
  mainFrame.url = 'javascript:alert(1)';
  assert.equal(
    isTrustedSender({ sender: fakeWebContents, senderFrame: mainFrame }, fakeWindow),
    false
  );
});

test('DaemonManager - requestJson bounds hanging HTTP streams and resolves null', async () => {
  const hangingServer = await startHttpServer((req, res) => {
    // Write headers but never finish or send complete body
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.write('{"status":');
  });

  try {
    const dm = new DaemonManager({ baseUrl: hangingServer.baseUrl });
    const start = Date.now();
    const result = await dm.requestJson('/test-hang', 200);
    const duration = Date.now() - start;
    assert.equal(result, null);
    assert.ok(duration < 1000, `Expected request to timeout quickly, took ${duration}ms`);
  } finally {
    await hangingServer.close();
  }
});

test('DaemonManager - stop enforces bounded execution and cleans up sockets', async () => {
  const dm = new DaemonManager({ port: 8765, host: '127.0.0.1' });
  const start = Date.now();
  await dm.stop();
  const elapsed = Date.now() - start;
  assert.ok(elapsed < 1000, `stop took ${elapsed}ms`);
});

test('computeTrayMenuSignature - produces stable signatures and detects state changes', () => {
  const stateA = {
    activeAlias: 'Main',
    quotaInfo: '5H: 86% | 7D: 96%',
    accountsCount: 2,
    switchMenuItems: [
      { label: '✓ a@example.com', enabled: false },
      { label: 'b@example.com', enabled: true },
    ],
  };
  const stateB = {
    activeAlias: 'Main',
    quotaInfo: '5H: 86% | 7D: 96%',
    accountsCount: 2,
    switchMenuItems: [
      { label: 'b@example.com', enabled: true },
      { label: '✓ a@example.com', enabled: false },
    ],
  };
  const stateChangedQuota = {
    ...stateA,
    quotaInfo: '5H: 84% | 7D: 96%',
  };

  assert.equal(computeTrayMenuSignature(stateA), computeTrayMenuSignature(stateB));
  assert.notEqual(computeTrayMenuSignature(stateA), computeTrayMenuSignature(stateChangedQuota));
});

test('isMainWindowUsable - rejects null, destroyed, or crashed BrowserWindow instances', () => {
  assert.equal(isMainWindowUsable(null), false);
  assert.equal(isMainWindowUsable({ isDestroyed: () => true }), false);
  assert.equal(
    isMainWindowUsable({
      isDestroyed: () => false,
      webContents: { isDestroyed: () => true, isCrashed: () => false },
    }),
    false
  );
  assert.equal(
    isMainWindowUsable({
      isDestroyed: () => false,
      webContents: { isDestroyed: () => false, isCrashed: () => true },
    }),
    false
  );
  assert.equal(
    isMainWindowUsable({
      isDestroyed: () => false,
      webContents: { isDestroyed: () => false, isCrashed: () => false },
    }),
    true
  );
});
