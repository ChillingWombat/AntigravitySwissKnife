const fs = require('node:fs');
const test = require('node:test');
const assert = require('node:assert/strict');
const { isTrustedDaemonOrigin, isSafeExternalUrl } = require('./main');

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
  assert.equal(fs.existsSync(testSockPath), false);

  assert.doesNotThrow(() => dm.cleanupSocket(testSockPath));
  try { fs.rmdirSync(path.dirname(testSockPath)); } catch {}
});

test('DaemonManager - reapOrphanedProcesses does not throw and returns boolean', () => {
  const { DaemonManager } = require('./daemon-manager');
  const dm = new DaemonManager({ port: 8765, host: '127.0.0.1' });
  assert.doesNotThrow(() => {
    const signaled = dm.reapOrphanedProcesses();
    assert.equal(typeof signaled, 'boolean');
  });
});
