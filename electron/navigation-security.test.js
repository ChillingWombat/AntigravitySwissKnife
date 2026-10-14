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
