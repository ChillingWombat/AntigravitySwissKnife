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
    this.isStoppingDaemon = false;
    this.isDetached = false;
    this.isTeardown = false;
    this.respawnTimer = null;
    this.crashTimestamps = [];
    this.maxRestarts = options.maxRestarts || 3;
    this.restartWindowMs = options.restartWindowMs || 30000;
    this.isHealthy = false;
    this.lastHealthyAt = 0;
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
   * Synchronously checks if a Swiss daemon is responding healthy on this.baseUrl via curl on Linux.
   */
  checkStatusSync(timeoutSec = 0.3) {
    if (process.platform !== 'linux') return false;
    try {
      const { execFileSync } = require('child_process');
      const out = execFileSync('curl', ['-s', '-m', String(timeoutSec), `${this.baseUrl}/health`], {
        encoding: 'utf8',
        timeout: Math.ceil(timeoutSec * 1000) + 200,
        stdio: ['ignore', 'pipe', 'ignore'],
      });
      const parsed = JSON.parse(out);
      return Boolean(parsed && parsed.status === 'ok' && parsed.daemon_running === true);
    } catch {
      return false;
    }
  }

  /**
   * Reaps unmanaged orphaned swiss daemon processes on Linux if no daemon is responding on HTTP.
   * Returns true if any orphaned process was signaled.
   * @param {object} [options]
   * @param {boolean} [options.force] Force reap even in test environment
   */
  reapOrphanedProcesses(options = {}) {
    if (process.platform !== 'linux') return false;
    const force = Boolean(options && options.force);
    if (process.env.NODE_ENV === 'test' && !force) return false;

    // Never signal a process if a Swiss daemon is already responding healthy on this.baseUrl
    if (this.isHealthy && (Date.now() - (this.lastHealthyAt || 0) < 3000)) {
      return false;
    }
    if (this.checkStatusSync && this.checkStatusSync(0.3)) {
      this.isHealthy = true;
      this.lastHealthyAt = Date.now();
      return false;
    }

    let signaled = false;
    try {
      const procEntries = fs.readdirSync('/proc');
      for (const entry of procEntries) {
        if (!/^\d+$/.test(entry)) continue;
        const pid = parseInt(entry, 10);
        if (pid === process.pid || pid <= 1) continue;
        if (this.child && pid === this.child.pid) continue;
        try {
          const cmdline = fs.readFileSync(`/proc/${pid}/cmdline`, 'utf8');
          if ((cmdline.includes('swiss') || cmdline.includes('antigravity-swiss')) && (cmdline.includes('daemon') || cmdline.includes('web'))) {
            // Check if process is a genuine orphan: check /proc/${pid}/status or /proc/${pid}/stat
            let ppid = null;
            try {
              const status = fs.readFileSync(`/proc/${pid}/status`, 'utf8');
              const match = status.match(/^PPid:\s*(\d+)/m);
              if (match) {
                ppid = parseInt(match[1], 10);
              }
            } catch (_) {}

            if (ppid === null) {
              try {
                const stat = fs.readFileSync(`/proc/${pid}/stat`, 'utf8');
                const lastParen = stat.lastIndexOf(')');
                if (lastParen !== -1) {
                  const rest = stat.slice(lastParen + 2).trim().split(/\s+/);
                  const parsedPpid = parseInt(rest[1], 10);
                  if (!Number.isNaN(parsedPpid)) {
                    ppid = parsedPpid;
                  }
                }
              } catch (_) {}
            }

            if (ppid === null) continue;

            // Only reap if ppid === 1 or if the parent process no longer exists
            let isOrphan = false;
            if (ppid === 1) {
              isOrphan = true;
            } else {
              try {
                process.kill(ppid, 0);
              } catch (killErr) {
                if (killErr.code === 'ESRCH') {
                  isOrphan = true;
                }
              }
              if (!isOrphan && !fs.existsSync(`/proc/${ppid}`)) {
                isOrphan = true;
              }
            }

            if (isOrphan) {
              console.log(`[DaemonManager] Reaping genuine unmanaged orphaned Go process (PID: ${pid}, PPID: ${ppid})...`);
              try {
                process.kill(pid, 'SIGTERM');
                signaled = true;
              } catch (_) {}
            }
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
      let req = null;
      let overallTimer = null;

      const finish = (value) => {
        if (settled) return;
        settled = true;
        if (overallTimer) clearTimeout(overallTimer);
        resolve(value);
      };

      overallTimer = setTimeout(() => {
        try {
          if (req) req.destroy();
        } catch (_) {}
        finish(null);
      }, timeoutMs + 300);
      overallTimer.unref();

      req = http.get(`${this.baseUrl}${pathname}`, { timeout: timeoutMs, headers }, (res) => {
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
    if (health && health.status === 'ok' && health.daemon_running === true && typeof health.version === 'string' && health.version.trim()) {
      this.isHealthy = true;
      this.lastHealthyAt = Date.now();
      return health;
    }

    const status = await this.requestJson('/api/status', timeoutMs, this.getAuthHeaders());
    if (status && status.daemon_running === true && typeof status.version === 'string' && status.version.trim()) {
      this.isHealthy = true;
      this.lastHealthyAt = Date.now();
      return status;
    }
    this.isHealthy = false;
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
    this.isStoppingDaemon = false;
    this.isDetached = false;
    this.isTeardown = false;
    if (this.respawnTimer) {
      clearTimeout(this.respawnTimer);
      this.respawnTimer = null;
    }

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
      this._handleChildExit(code, signal);
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
   * Handles child process exit with watchdog auto-restart behavior.
   */
  _handleChildExit(code, signal) {
    console.log(`[DaemonManager] Child process exited: code=${code}, signal=${signal}`);
    const wasManaged = this.isManagedChild;
    this.child = null;
    this.isManagedChild = false;
    this.isHealthy = false;

    // Watchdog / Auto-Restart:
    // If unexpected (code !== 0 or killed externally while app is running),
    // log a warning and schedule automatic respawn (this.start()) after 1000ms
    // with max retry count (e.g., 3 retries within 30s) so the sidecar never stays dead.
    const isIntentional = this.isStoppingDaemon || this.isDetached || this.isTeardown;
    if (!isIntentional && wasManaged) {
      const now = Date.now();
      this.crashTimestamps = this.crashTimestamps.filter((ts) => now - ts < this.restartWindowMs);

      if (this.crashTimestamps.length < this.maxRestarts) {
        this.crashTimestamps.push(now);
        const attempt = this.crashTimestamps.length;
        console.warn(`[DaemonManager] Unexpected daemon termination (code=${code}, signal=${signal}). Watchdog scheduling auto-restart (attempt ${attempt}/${this.maxRestarts}) in 1000ms...`);
        if (this.respawnTimer) clearTimeout(this.respawnTimer);
        this.respawnTimer = setTimeout(async () => {
          this.respawnTimer = null;
          if (this.isStoppingDaemon || this.isDetached || this.isTeardown) return;
          try {
            console.log('[DaemonManager] Watchdog respawning Go daemon sidecar...');
            await this.start();
          } catch (err) {
            console.error('[DaemonManager] Watchdog auto-restart failed:', err.message);
          }
        }, 1000);
        if (this.respawnTimer && typeof this.respawnTimer.unref === 'function') {
          this.respawnTimer.unref();
        }
      } else {
        console.error(`[DaemonManager] Daemon exceeded maximum auto-restart attempts (${this.maxRestarts} crashes in ${this.restartWindowMs / 1000}s). Watchdog will not respawn.`);
      }
    }
  }

  /**
   * Detaches the Go daemon child process so it continues running in background (Daemon Mode).
   */
  detach() {
    this.isDetached = true;
    if (this.respawnTimer) {
      clearTimeout(this.respawnTimer);
      this.respawnTimer = null;
    }
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
    this.isStoppingDaemon = true;
    this.isTeardown = true;
    if (this.respawnTimer) {
      clearTimeout(this.respawnTimer);
      this.respawnTimer = null;
    }
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
              try {
                if (process.platform !== 'win32') {
                  try { process.kill(-pid, 'SIGTERM'); } catch (_) { process.kill(pid, 'SIGTERM'); }
                } else {
                  process.kill(pid, 'SIGTERM');
                }
              } catch (_) {}
              const deadline = Date.now() + 2000;
              while (Date.now() < deadline) {
                await new Promise((r) => setTimeout(r, 100));
                const alive = await this.checkStatus(200);
                if (!alive) break;
              }
              // If still running after deadline, escalate to SIGKILL / taskkill
              try {
                process.kill(pid, 0); // check if alive
                console.warn(`[DaemonManager] Adopted daemon (PID: ${pid}) did not exit within 2000ms. Escalating to SIGKILL...`);
                if (process.platform !== 'win32') {
                  try { process.kill(-pid, 'SIGKILL'); } catch (_) { process.kill(pid, 'SIGKILL'); }
                } else {
                  try {
                    const { execSync } = require('child_process');
                    execSync(`taskkill /F /PID ${pid} /T`, { stdio: 'ignore' });
                  } catch (_) {
                    try { process.kill(pid, 'SIGKILL'); } catch (_) {}
                  }
                }
              } catch (_) {}
            } else {
              console.warn(`[DaemonManager] Refusing to signal PID ${pid}: not confirmed as swiss daemon`);
            }
          }
          if (process.platform === 'linux') {
            this.reapOrphanedProcesses();
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
      let killTimer = null;
      let hardLimitTimer = null;

      const finish = () => {
        if (!resolved) {
          resolved = true;
          if (killTimer) clearTimeout(killTimer);
          if (hardLimitTimer) clearTimeout(hardLimitTimer);
          resolve();
        }
      };

      // Hard limit timer: under NO circumstances will this promise take > 3000ms
      hardLimitTimer = setTimeout(() => {
        console.warn(`[DaemonManager] Hard stop limit reached (3000ms) for PID ${pid}. Forcing resolution.`);
        finish();
      }, 3000);
      hardLimitTimer.unref();

      // 2.5-second fallback to SIGKILL before hard limit
      killTimer = setTimeout(() => {
        console.warn(`[DaemonManager] Grace period expired (2500ms). Escalating to SIGKILL on PID ${pid}...`);
        try {
          if (process.platform !== 'win32') {
            try {
              process.kill(-pid, 'SIGKILL');
            } catch (_) {
              process.kill(pid, 'SIGKILL');
            }
          } else {
            try {
              const { execSync } = require('child_process');
              execSync(`taskkill /F /PID ${pid} /T`, { stdio: 'ignore' });
            } catch (_) {
              try { child.kill('SIGKILL'); } catch (_) {}
            }
          }
        } catch (err) {
          console.warn(`[DaemonManager] SIGKILL warning: ${err.message}`);
        }
      }, 2500);
      killTimer.unref();

      child.once('exit', (code, signal) => {
        console.log(`[DaemonManager] Go daemon child process (PID: ${pid}) exited cleanly: code=${code}, signal=${signal}. Zero orphans.`);
        finish();
      });

      try {
        if (process.platform !== 'win32') {
          try {
            process.kill(-pid, 'SIGTERM');
          } catch (_) {
            process.kill(pid, 'SIGTERM');
          }
        } else {
          child.kill('SIGTERM');
        }
      } catch (err) {
        console.warn(`[DaemonManager] SIGTERM error: ${err.message}`);
        finish();
      }
    });

    if (process.platform === 'linux') {
      this.reapOrphanedProcesses();
    }

    // Clean up socket file defensively
    this.cleanupSocket();
  }
}

module.exports = { DaemonManager };
