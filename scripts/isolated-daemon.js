'use strict';

const net = require('net');
const path = require('path');
const fs = require('fs');
const os = require('os');
const http = require('http');
const { spawn } = require('child_process');

const FORBIDDEN_PORT = 8765;

/**
 * Allocates an ephemeral free TCP port on 127.0.0.1.
 * Guaranteed to never return port 8765 (reserved for live host application).
 */
function getFreePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.listen(0, '127.0.0.1', () => {
      const port = srv.address().port;
      srv.close((err) => {
        if (err) return reject(err);
        if (port === FORBIDDEN_PORT) {
          return resolve(getFreePort());
        }
        resolve(port);
      });
    });
    srv.on('error', reject);
  });
}

/**
 * Resolves the path to the swiss daemon executable.
 */
function resolveDaemonBinary(customPath) {
  if (customPath && fs.existsSync(customPath)) {
    return customPath;
  }
  const isWin = process.platform === 'win32';
  const binName = isWin ? 'swiss.exe' : 'swiss';
  const localBin = path.join(__dirname, '..', 'bin', binName);
  if (fs.existsSync(localBin)) {
    return localBin;
  }
  return null;
}

/**
 * Probes the /health unauthenticated endpoint of the daemon.
 */
function probeHealth(port) {
  return new Promise((resolve, reject) => {
    const req = http.get(
      {
        host: '127.0.0.1',
        port,
        path: '/health',
        timeout: 1000,
      },
      (res) => {
        let body = '';
        res.on('data', (chunk) => {
          body += chunk;
        });
        res.on('end', () => {
          try {
            resolve(JSON.parse(body));
          } catch (e) {
            reject(new Error(`Non-JSON response from daemon on port ${port}: ${body}`));
          }
        });
      }
    );
    req.on('error', reject);
    req.on('timeout', () => {
      req.destroy();
      reject(new Error(`Timeout probing port ${port}`));
    });
  });
}

/**
 * Probes the /api/status authenticated endpoint of the daemon.
 */
function probeStatus(port, token) {
  return new Promise((resolve, reject) => {
    const headers = {};
    if (token) {
      headers['X-Session-Token'] = token;
    }
    const req = http.get(
      {
        host: '127.0.0.1',
        port,
        path: '/api/status',
        headers,
        timeout: 1000,
      },
      (res) => {
        let body = '';
        res.on('data', (chunk) => {
          body += chunk;
        });
        res.on('end', () => {
          try {
            resolve(JSON.parse(body));
          } catch (e) {
            reject(new Error(`Non-JSON response from daemon on port ${port}: ${body}`));
          }
        });
      }
    );
    req.on('error', reject);
    req.on('timeout', () => {
      req.destroy();
      reject(new Error(`Timeout probing port ${port}`));
    });
  });
}

/**
 * Reads the session.token file from a config directory if it exists.
 */
function readSessionToken(configDir) {
  try {
    const tokenPath = path.join(configDir, 'session.token');
    if (fs.existsSync(tokenPath)) {
      return fs.readFileSync(tokenPath, 'utf8').trim();
    }
  } catch (_) {}
  return null;
}

/**
 * Polls the daemon until it answers HTTP 200 on /health with valid status.
 */
async function waitForHealth(port, timeoutMs = 10000) {
  const start = Date.now();
  let lastErr = null;
  while (Date.now() - start < timeoutMs) {
    try {
      const status = await probeHealth(port);
      if (status && (status.daemon_running || status.daemon_pid)) {
        return status;
      }
    } catch (err) {
      lastErr = err;
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`Timed out waiting for isolated daemon on port ${port} after ${timeoutMs}ms. Last error: ${lastErr?.message}`);
}

/**
 * Spawns an isolated companion daemon guaranteed to not interfere with port 8765
 * or host IDE configuration.
 *
 * @param {Object} options
 * @param {number} [options.port] Custom port (must not be 8765)
 * @param {string} [options.configDir] Custom config directory
 * @param {string} [options.binaryPath] Custom path to swiss binary
 * @param {number} [options.timeoutMs] Health check timeout in ms
 * @returns {Promise<{port: number, baseUrl: string, socketPath: string, configDir: string, sessionToken: string, stop: () => Promise<void>}>}
 */
async function startIsolatedDaemon(options = {}) {
  let port = options.port;
  if (port === FORBIDDEN_PORT) {
    throw new Error(`Port ${FORBIDDEN_PORT} is strictly forbidden for isolated testing. Port ${FORBIDDEN_PORT} is reserved for the live desktop application.`);
  }
  if (!port) {
    port = await getFreePort();
  }

  let isCreatedConfigDir = false;
  let configDir = options.configDir;
  if (!configDir) {
    configDir = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-isolated-'));
    isCreatedConfigDir = true;
  }

  const socketPath = options.socketPath || path.join(configDir, 'swiss.sock');
  const binary = resolveDaemonBinary(options.binaryPath);
  if (!binary) {
    throw new Error('swiss daemon binary not found. Build it with `npm run build:go` or pass options.binaryPath.');
  }

  // Critical invariants:
  // 1. ANTIGRAVITY_TEST_MODE: disables startLanguageServerURLSync() so test daemon
  //    never mutates the live Antigravity host language server cloud code URL.
  // 2. ANTIGRAVITY_SWISS_CONFIG_DIR: isolates all database, token, and json config.
  const env = {
    ...process.env,
    ANTIGRAVITY_TEST_MODE: '1',
    ANTIGRAVITY_TESTING: '1',
    ANTIGRAVITY_SWISS_CONFIG_DIR: configDir,
    XDG_CONFIG_HOME: configDir,
    ...(options.env || {}),
  };

  const args = [
    'daemon',
    '--socket', socketPath,
    '--web',
    '--addr', `127.0.0.1:${port}`,
  ];

  const child = spawn(binary, args, {
    env,
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  const logs = [];
  child.stdout.on('data', (d) => logs.push(d.toString()));
  child.stderr.on('data', (d) => logs.push(d.toString()));

  let hasStopped = false;
  const stop = async () => {
    if (hasStopped) return;
    hasStopped = true;

    if (child && !child.killed) {
      child.kill('SIGTERM');
      await new Promise((resolve) => {
        const timer = setTimeout(() => {
          try { child.kill('SIGKILL'); } catch (_) {}
          resolve();
        }, 3000);
        child.on('exit', () => {
          clearTimeout(timer);
          resolve();
        });
      });
    }

    if (isCreatedConfigDir && fs.existsSync(configDir)) {
      try {
        fs.rmSync(configDir, { recursive: true, force: true });
      } catch (_) {}
    }
  };

  try {
    const health = await waitForHealth(port, options.timeoutMs || 10000);
    const sessionToken = readSessionToken(configDir);
    return {
      port,
      baseUrl: `http://127.0.0.1:${port}`,
      socketPath,
      configDir,
      sessionToken,
      health,
      stop,
      getLogs: () => logs.join(''),
    };
  } catch (err) {
    await stop();
    throw new Error(`Failed to start isolated daemon on port ${port}: ${err.message}\nLogs:\n${logs.join('')}`);
  }
}

module.exports = {
  FORBIDDEN_PORT,
  getFreePort,
  resolveDaemonBinary,
  probeHealth,
  probeStatus,
  readSessionToken,
  waitForHealth,
  startIsolatedDaemon,
};
