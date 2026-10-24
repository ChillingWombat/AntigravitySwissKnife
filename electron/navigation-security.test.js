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

  // Fallback connecting screens (data:text/html)
  assert.equal(isTrustedDaemonOrigin('data:text/html;charset=utf-8,%3Ch1%3EConnecting%3C/h1%3E', DAEMON_URL), true);
  assert.equal(isTrustedDaemonOrigin('data:text/html,<h1>Connecting</h1>', DAEMON_URL), true);

  // Dangerous non-HTML data URIs must NOT be trusted
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

test('isTrustedSender - verifies window status, sender identity, and frame origin', () => {
  const fakeWebContents = {};
  const fakeWindow = {
    isDestroyed: () => false,
    webContents: fakeWebContents,
  };

  // Valid event from daemon origin
  assert.equal(
    isTrustedSender(
      { sender: fakeWebContents, senderFrame: { url: 'http://127.0.0.1:8765/api/status' } },
      fakeWindow
    ),
    true
  );

  // Valid event from connecting fallback data URL
  assert.equal(
    isTrustedSender(
      { sender: fakeWebContents, senderFrame: { url: 'data:text/html;charset=utf-8,Connecting...' } },
      fakeWindow
    ),
    true
  );

  // Missing or destroyed window
  assert.equal(isTrustedSender({ sender: fakeWebContents, senderFrame: { url: DAEMON_URL } }, null), false);
  assert.equal(
    isTrustedSender(
      { sender: fakeWebContents, senderFrame: { url: DAEMON_URL } },
      { isDestroyed: () => true, webContents: fakeWebContents }
    ),
    false
  );

  // Missing event or wrong sender
  assert.equal(isTrustedSender(null, fakeWindow), false);
  assert.equal(isTrustedSender({}, fakeWindow), false);
  assert.equal(isTrustedSender({ sender: {} }, fakeWindow), false);

  // Missing senderFrame or untrusted URL
  assert.equal(isTrustedSender({ sender: fakeWebContents }, fakeWindow), false);
  assert.equal(
    isTrustedSender({ sender: fakeWebContents, senderFrame: { url: 'http://attacker.com' } }, fakeWindow),
    false
  );
  assert.equal(
    isTrustedSender({ sender: fakeWebContents, senderFrame: { url: 'file:///etc/passwd' } }, fakeWindow),
    false
  );
  assert.equal(
    isTrustedSender({ sender: fakeWebContents, senderFrame: { url: 'javascript:alert(1)' } }, fakeWindow),
    false
  );
});
