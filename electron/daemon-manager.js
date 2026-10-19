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
    this.socketPath = options.socketPath || null;
    this.getAuthHeaders = typeof options.getAuthHeaders === 'function' ? options.getAuthHeaders : () => ({});
    this.child = null;
    this.isManagedChild = false;
    this.logBuffer = [];
    this.maxLogLines = 50;
    this.startPromise = null;
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
    let appDir = (app && typeof app.getAppPath === 'function')
      ? app.getAppPath()
      : path.resolve(__dirname, '..');
    if (appDir.endsWith('.asar') || appDir.includes('.asar')) {
      appDir = process.resourcesPath || path.dirname(appDir);
    }
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
    if (this.socketPath) {
      return this.socketPath;
    }
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
   * Reaps unmanaged orphaned swiss daemon processes on Linux if no daemon is responding on HTTP.
   * Returns true if any orphaned process was signaled.
   */
  reapOrphanedProcesses() {
    if (process.platform !== 'linux') return false;
    let signaled = false;
    try {
      const procEntries = fs.readdirSync('/proc');
      for (const entry of procEntries) {
        if (!/^\d+$/.test(entry)) continue;
        const pid = parseInt(entry, 10);
        if (pid === process.pid || pid <= 1) continue;
        try {
          const cmdline = fs.readFileSync(`/proc/${pid}/cmdline`, 'utf8');
          if ((cmdline.includes('swiss') || cmdline.includes('antigravity-swiss')) && cmdline.includes('daemon')) {
            console.log(`[DaemonManager] Reaping unmanaged orphaned Go daemon process (PID: ${pid})...`);
            try {
              process.kill(pid, 'SIGTERM');
              signaled = true;
            } catch (_) {}
          }
        } catch (_) {}
      }
    } catch (_) {}
    return signaled;
  }

  /**
   * Cleans up stale Unix domain socket file if it exists.
  * @param {string} [customPath] Optional explicit socket path (e.g. for testing)
   */
  cleanupSocket(customPath = null) {
    if (process.platform === 'win32') return;
    try {
      const sockPath = customPath || this.getSocketPath();
      if (fs.existsSync(sockPath)) {
        fs.rmSync(sockPath, { force: true });
        console.log(`[DaemonManager] Cleaned up socket file: ${sockPath}`);
      }
    } catch (err) {
      console.warn(`[DaemonManager] Note on socket cleanup: ${err.message}`);
    }
  }

  /**
   * Reads a bounded JSON object from a daemon endpoint.
   */
  requestJson(pathname, timeoutMs = 1000, headers = {}) {
    return new Promise((resolve) => {
      let settled = false;
      const finish = (value) => {
        if (settled) return;
        settled = true;
        resolve(value);
      };

      const req = http.get(`${this.baseUrl}${pathname}`, { timeout: timeoutMs, headers }, (res) => {
        const contentType = String(res.headers['content-type'] || '').toLowerCase();
        if (res.statusCode !== 200 || !contentType.includes('application/json')) {
          res.resume();
          finish(null);
          return;
        }

        let data = '';
        res.setEncoding('utf8');
        res.on('data', (chunk) => {
          data += chunk;
          if (data.length > 64 * 1024) {
            res.destroy();
            finish(null);
          }
        });
        res.on('end', () => {
          try {
            const parsed = JSON.parse(data);
            finish(parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : null);
          } catch {
            finish(null);
          }
        });
        res.on('error', () => finish(null));
      });

      req.on('error', () => finish(null));
      req.on('timeout', () => {
        req.destroy();
        finish(null);
      });
    });
  }

  /**
   * Checks daemon identity by validating /health, then /api/status.
   * A 401 is not identity: both contracts require Swiss JSON fields.
   */
  async checkStatus(timeoutMs = 1000) {
    const health = await this.requestJson('/health', timeoutMs);
    if (health && health.status === 'ok' && typeof health.version === 'string' && health.version.trim()) {
      return { ...health, daemon_running: true };
    }

    const status = await this.requestJson('/api/status', timeoutMs, this.getAuthHeaders());
    if (status && status.daemon_running === true && typeof status.version === 'string' && status.version.trim()) {
      return status;
    }
    return null;
  }

  /**
   * Ensures the daemon is running: reuses existing external daemon if active,
   * otherwise spawns the binary and polls until ready.
   */
  async start() {
    if (this.startPromise) {
      return this.startPromise;
    }
    this.startPromise = this._startInternal();
    try {
      return await this.startPromise;
    } finally {
      this.startPromise = null;
    }
  }

  async _startInternal() {
    // 1. External Daemon Check: probe /api/status verifying HTTP 200 and status.daemon_running === true
    const existing = await this.checkStatus(800);
    if (existing && existing.daemon_running === true) {
      console.log(`[DaemonManager] Existing Go daemon detected (PID: ${existing.daemon_pid || existing.pid}). Reusing external daemon.`);
      this.isManagedChild = false;
      return;
    }

    // Reap unmanaged orphaned Go daemon processes before spawning a new sidecar
    if (this.reapOrphanedProcesses()) {
      await new Promise((r) => setTimeout(r, 200));
    }
    // Clean up any stale Unix domain socket file defensively
    this.cleanupSocket();

    // 2. Resolve binary
    const binPath = this.resolveBinaryPath();
    if (!fs.existsSync(binPath) && binPath.includes(path.sep)) {
      throw new Error(`Go daemon binary not found at '${binPath}'. Build with 'go build -o bin/swiss ./cmd/swiss'.`);
    }

    console.log(`[DaemonManager] Spawning Go daemon sidecar: ${binPath} daemon --web --addr ${this.host}:${this.port}`);

    // 3. Spawn child process
    let runCwd = path.resolve(__dirname, '..');
    if (app && app.isPackaged) {
      try {
        runCwd = (typeof app.getPath === 'function' ? app.getPath('userData') : null) || os.homedir();
      } catch {
        runCwd = os.homedir();
      }
    } else if (app && typeof app.getAppPath === 'function') {
      const p = app.getAppPath();
      runCwd = (p.endsWith('.asar') || p.includes('.asar')) ? (process.resourcesPath || path.dirname(p)) : p;
    }
    if (runCwd.startsWith('/tmp/.mount_') || process.env.APPIMAGE) {
      runCwd = os.homedir();
    }
    if (!fs.existsSync(runCwd)) {
      try {
        fs.mkdirSync(runCwd, { recursive: true });
      } catch {}
    }

    this.child = spawn(binPath, ['daemon', '--web', '--addr', `${this.host}:${this.port}`], {
      cwd: runCwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      detached: true, // Independent process group for clean detach in Daemon Mode
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
   * Detaches the Go daemon child process so it continues running in background (Daemon Mode).
   */
  detach() {
    if (!this.child) {
      console.log('[DaemonManager] No managed child daemon to detach.');
      return;
    }

    const child = this.child;
    const pid = child.pid;
    console.log(`[DaemonManager] Detaching Go daemon child process (PID: ${pid}) for background persistence...`);

    child.removeAllListeners('exit');
    child.removeAllListeners('error');
    if (child.stdout) {
      try { child.stdout.destroy(); } catch (_) {}
    }
    if (child.stderr) {
      try { child.stderr.destroy(); } catch (_) {}
    }

    try {
      child.unref();
    } catch (err) {
      console.warn(`[DaemonManager] child.unref() warning: ${err.message}`);
    }

    this.child = null;
    this.isManagedChild = false;
    console.log(`[DaemonManager] Go daemon (PID: ${pid}) detached successfully in background.`);
  }

  /**
   * Graceful termination on full application exit.
   */
  async stop(options = {}) {
    const force = Boolean(options && options.force);
    if (!this.isManagedChild || !this.child) {
      if (force && process.env.TEST_DESKTOP_E2E !== '1') {
        try {
          const status = await this.checkStatus(600);
          const pid = status?.daemon_pid || status?.pid;
          if (pid && typeof pid === 'number' && pid > 1 && pid !== process.pid) {
            let isSwissProcess = true;
            if (process.platform === 'linux') {
              try {
                const cmdline = fs.readFileSync(`/proc/${pid}/cmdline`, 'utf8');
                isSwissProcess = cmdline.includes('swiss') || cmdline.includes('antigravity');
              } catch (_) {
                isSwissProcess = false;
              }
            }
            if (isSwissProcess) {
              console.log(`[DaemonManager] Terminating adopted background Go daemon (PID: ${pid}) via SIGTERM...`);
              try { process.kill(pid, 'SIGTERM'); } catch (_) {}
              const deadline = Date.now() + 2000;
              while (Date.now() < deadline) {
                await new Promise((r) => setTimeout(r, 100));
                const alive = await this.checkStatus(200);
                if (!alive) break;
              }
            } else {
              console.warn(`[DaemonManager] Refusing to signal PID ${pid}: not confirmed as swiss daemon`);
            }
          }
        } catch (err) {
          console.warn('[DaemonManager] Note stopping adopted daemon:', err.message);
        }
        this.cleanupSocket();
      } else {
        console.log('[DaemonManager] No managed child daemon to terminate (external daemon preserved).');
      }
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
