const electron = require('electron');
const app = electron.app || null;
const path = require('path');
const http = require('http');
const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');

/**
 * DaemonManager supervises the Go daemon sidecar lifecycle:
 * - Resolves binary path across dev and packaged modes.
 * - Detects pre-existing daemons via HTTP GET /api/status.
 * - Spawns bin/swiss daemon --web if not running.
 * - Polls every 150ms up to 10s timeout until HTTP 200 with status.daemon_running === true.
 * - Gracefully terminates child process on full app exit (SIGTERM -> 3s SIGKILL).
 * - Defensively unlinks Unix socket file.
 * - Never terminates external pre-existing daemons (isManagedChild = false).
 */
class DaemonManager {
  constructor(options = {}) {
    this.port = options.port || 8765;
    this.host = options.host || '127.0.0.1';
    this.baseUrl = options.baseUrl || `http://${this.host}:${this.port}`;
    this.child = null;
    this.isManagedChild = false;
    this.logBuffer = [];
    this.maxLogLines = 50;
  }

  /**
   * Resolves the Go binary executable path for dev vs packaged production mode.
   */
  resolveBinaryPath() {
    const isWin = process.platform === 'win32';
    const binName = isWin ? 'swiss.exe' : 'swiss';

    // 1. Packaged application: process.resourcesPath/bin/swiss
    if (app && app.isPackaged) {
      const packagedPath = path.join(process.resourcesPath, 'bin', binName);
      if (fs.existsSync(packagedPath)) {
        this.ensureExecutable(packagedPath);
        return packagedPath;
      }
    }

    // 2. Development mode: <projectRoot>/bin/swiss
    const appDir = (app && typeof app.getAppPath === 'function')
      ? app.getAppPath()
      : path.resolve(__dirname, '..');
    const devPath = path.join(appDir, 'bin', binName);
    if (fs.existsSync(devPath)) {
      this.ensureExecutable(devPath);
      return devPath;
    }

    // 3. Fallback relative to __dirname
    const relativePath = path.resolve(__dirname, '..', 'bin', binName);
    if (fs.existsSync(relativePath)) {
      this.ensureExecutable(relativePath);
      return relativePath;
    }

    // 4. Custom environment variable override
    if (process.env.SWISS_BIN_PATH && fs.existsSync(process.env.SWISS_BIN_PATH)) {
      this.ensureExecutable(process.env.SWISS_BIN_PATH);
      return process.env.SWISS_BIN_PATH;
    }

    // 5. Fallback to system PATH
    return binName;
  }

  /**
   * Ensures binary has executable permissions on POSIX systems.
   */
  ensureExecutable(filePath) {
    if (process.platform === 'win32') return;
    try {
      fs.accessSync(filePath, fs.constants.X_OK);
    } catch {
      try {
        fs.chmodSync(filePath, 0o755);
      } catch (err) {
        console.warn(`[DaemonManager] Warning: could not chmod +x ${filePath}: ${err.message}`);
      }
    }
  }

  /**
   * Resolves the Unix socket path used by the Go daemon.
   */
  getSocketPath() {
    if (process.env.ANTIGRAVITY_SWISS_SOCKET) {
      return process.env.ANTIGRAVITY_SWISS_SOCKET;
    }
    if (process.env.ANTIGRAVITY_SWISS_RUNTIME_DIR) {
      return path.join(process.env.ANTIGRAVITY_SWISS_RUNTIME_DIR, 'daemon.sock');
    }
    if (process.env.XDG_RUNTIME_DIR) {
      return path.join(process.env.XDG_RUNTIME_DIR, 'antigravity-swiss', 'daemon.sock');
    }
    return path.join(os.tmpdir(), 'antigravity-swiss', 'daemon.sock');
  }

  /**
   * Cleans up stale Unix domain socket file if it exists.
   */
  cleanupSocket() {
    if (process.platform === 'win32') return;
    try {
      const sockPath = this.getSocketPath();
      if (fs.existsSync(sockPath)) {
        fs.unlinkSync(sockPath);
        console.log(`[DaemonManager] Cleaned up socket file: ${sockPath}`);
      }
    } catch (err) {
      console.warn(`[DaemonManager] Note on socket cleanup: ${err.message}`);
    }
  }

  /**
   * Checks daemon liveness by probing GET /api/status.
   */
  checkStatus(timeoutMs = 1000) {
    return new Promise((resolve) => {
      const req = http.get(`${this.baseUrl}/api/status`, { timeout: timeoutMs }, (res) => {
        if (res.statusCode !== 200) {
          resolve(null);
          return;
        }
        let data = '';
        res.on('data', (chunk) => { data += chunk; });
        res.on('end', () => {
          try {
            const parsed = JSON.parse(data);
            resolve(parsed);
          } catch {
            resolve(null);
          }
        });
      });

      req.on('error', () => resolve(null));
      req.on('timeout', () => {
        req.destroy();
        resolve(null);
      });
    });
  }

  /**
   * Ensures the daemon is running: reuses existing external daemon if active,
   * otherwise spawns the binary and polls until ready.
   */
  async start() {
    // 1. External Daemon Check: probe /api/status verifying HTTP 200 and status.daemon_running === true
    const existing = await this.checkStatus(800);
    if (existing && existing.daemon_running === true) {
      console.log(`[DaemonManager] Existing Go daemon detected (PID: ${existing.daemon_pid || existing.pid}). Reusing external daemon.`);
      this.isManagedChild = false;
      return;
    }

    // 2. Resolve binary
    const binPath = this.resolveBinaryPath();
    if (!fs.existsSync(binPath) && binPath.includes(path.sep)) {
      throw new Error(`Go daemon binary not found at '${binPath}'. Build with 'go build -o bin/swiss ./cmd/swiss'.`);
    }

    console.log(`[DaemonManager] Spawning Go daemon sidecar: ${binPath} daemon --web --addr ${this.host}:${this.port}`);

    // 3. Spawn child process
    let runCwd = path.resolve(__dirname, '..');
    if (app && app.isPackaged) {
      runCwd = process.resourcesPath;
    } else if (app && typeof app.getAppPath === 'function') {
      const appPath = app.getAppPath();
      try {
        runCwd = fs.statSync(appPath).isDirectory() ? appPath : path.dirname(appPath);
      } catch {
        runCwd = path.dirname(appPath);
      }
    }

    this.child = spawn(binPath, ['daemon', '--web', '--addr', `${this.host}:${this.port}`], {
      cwd: runCwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      detached: false, // Ensures child process group is tied to Electron
      windowsHide: true,
      env: { ...process.env },
    });

    this.isManagedChild = true;

    // 4. Capture logs and stream
    this.child.stdout.on('data', (data) => {
      const str = data.toString();
      this.appendLog(`[out] ${str.trim()}`);
      process.stdout.write(`[swiss-daemon] ${str}`);
    });

    this.child.stderr.on('data', (data) => {
      const str = data.toString();
      this.appendLog(`[err] ${str.trim()}`);
      process.stderr.write(`[swiss-daemon-err] ${str}`);
    });

    this.child.on('error', (err) => {
      console.error('[DaemonManager] Child process error:', err);
    });

    this.child.on('exit', (code, signal) => {
      console.log(`[DaemonManager] Child process exited: code=${code}, signal=${signal}`);
      this.child = null;
      this.isManagedChild = false;
    });

    // 5. Poll every 150ms up to 10s timeout until HTTP 200 with status.daemon_running === true
    const deadline = Date.now() + 10000;
    while (Date.now() < deadline) {
      if (!this.child) {
        throw new Error(`[DaemonManager] Go daemon process exited unexpectedly during startup.\nLogs:\n${this.logBuffer.join('\n')}`);
      }
      await new Promise((r) => setTimeout(r, 150));
      const status = await this.checkStatus(400);
      if (status && status.daemon_running === true) {
        console.log(`[DaemonManager] Go daemon ready and healthy on ${this.baseUrl} (PID: ${status.daemon_pid || this.child.pid})`);
        return;
      }
    }

    // 6. Handle timeout
    await this.stop();
    throw new Error(`[DaemonManager] Timed out waiting for Go daemon to become healthy on ${this.baseUrl}/api/status.\nLogs:\n${this.logBuffer.join('\n')}`);
  }

  /**
   * Appends a log line to the rolling log buffer.
   */
  appendLog(line) {
    this.logBuffer.push(line);
    if (this.logBuffer.length > this.maxLogLines) {
      this.logBuffer.shift();
    }
  }

  /**
   * Graceful termination on full application exit.
   */
  async stop() {
    if (!this.isManagedChild || !this.child) {
      console.log('[DaemonManager] No managed child daemon to terminate (external daemon preserved).');
      return;
    }

    const child = this.child;
    const pid = child.pid;
    this.child = null;
    this.isManagedChild = false;

    console.log(`[DaemonManager] Terminating managed Go daemon child process (PID: ${pid}) via SIGTERM...`);

    await new Promise((resolve) => {
      let resolved = false;
      const finish = () => {
        if (!resolved) {
          resolved = true;
          resolve();
        }
      };

      // 3-second fallback to SIGKILL
      const killTimer = setTimeout(() => {
        console.warn(`[DaemonManager] Grace period expired (3000ms). Escalating to SIGKILL on PID ${pid}...`);
        try {
          child.kill('SIGKILL');
        } catch (err) {
          console.warn(`[DaemonManager] SIGKILL warning: ${err.message}`);
        }
        finish();
      }, 3000);

      child.once('exit', (code, signal) => {
        clearTimeout(killTimer);
        console.log(`[DaemonManager] Go daemon child process (PID: ${pid}) exited cleanly: code=${code}, signal=${signal}. Zero orphans.`);
        finish();
      });

      try {
        child.kill('SIGTERM');
      } catch (err) {
        clearTimeout(killTimer);
        console.warn(`[DaemonManager] SIGTERM error: ${err.message}`);
        finish();
      }
    });

    // Clean up socket file defensively
    this.cleanupSocket();
  }
}

module.exports = { DaemonManager };
