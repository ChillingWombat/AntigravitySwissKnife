'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const {
  FORBIDDEN_PORT,
  getFreePort,
  resolveDaemonBinary,
  startIsolatedDaemon,
  probeHealth,
  probeStatus,
} = require('./isolated-daemon');

function ensureBinaryExists() {
  const binary = resolveDaemonBinary();
  if (binary && fs.existsSync(binary)) {
    return binary;
  }
  try {
    const isWin = process.platform === 'win32';
    const binName = isWin ? 'swiss.exe' : 'swiss';
    const target = path.join(__dirname, '..', 'bin', binName);
    fs.mkdirSync(path.join(__dirname, '..', 'bin'), { recursive: true });
    execSync(`go build -o "${target}" ./cmd/swiss`, {
      cwd: path.join(__dirname, '..'),
      stdio: 'ignore',
    });
    if (fs.existsSync(target)) {
      return target;
    }
  } catch (_) {}
  return null;
}

test('getFreePort returns a non-zero port that is never 8765', async () => {
  const port = await getFreePort();
  assert.ok(typeof port === 'number');
  assert.ok(port > 1024 && port < 65536);
  assert.notEqual(port, FORBIDDEN_PORT);
});

test('startIsolatedDaemon rejects port 8765 to prevent conversation interference', async () => {
  await assert.rejects(
    async () => {
      await startIsolatedDaemon({ port: 8765 });
    },
    {
      message: /Port 8765 is strictly forbidden for isolated testing/,
    }
  );
});

test('startIsolatedDaemon boots on ephemeral port with isolated config and cleans up', async (t) => {
  const binary = ensureBinaryExists();
  if (!binary) {
    t.skip('swiss daemon binary not found and go compiler unavailable');
    return;
  }
  const daemon = await startIsolatedDaemon({ binaryPath: binary });
  assert.ok(daemon.port > 0);
  assert.notEqual(daemon.port, FORBIDDEN_PORT);
  assert.ok(fs.existsSync(daemon.configDir));
  assert.ok(daemon.sessionToken && daemon.sessionToken.length > 10);

  // Verify unauthenticated /health on the ephemeral port
  const health = await probeHealth(daemon.port);
  assert.equal(health.status, 'ok');
  assert.equal(health.daemon_running, true);
  assert.ok(typeof health.daemon_pid === 'number');

  // Verify authenticated /api/status on the ephemeral port
  const status = await probeStatus(daemon.port, daemon.sessionToken);
  assert.equal(status.daemon_running, true);
  assert.equal(status.daemon_pid, health.daemon_pid);

  // Stop daemon
  await daemon.stop();

  // Verify daemon process is gone and port is closed
  await assert.rejects(async () => {
    await probeHealth(daemon.port);
  });

  // Verify temp directory was cleaned up
  assert.equal(fs.existsSync(daemon.configDir), false);
});
