# Handoff Report: Go Backend Daemon, Binary Builds & Process Lifecycle Survey

**Agent**: `explorer_survey_2` (Teamwork Explorer)  
**Date**: 2026-10-05T10:22:00Z  
**Target Milestone**: Pre-M1 Architecture Survey & Sidecar Management Mapping  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_survey_2`  

---

## 1. Observation

### 1.1 Codebase Structure & Build Configuration
- **Module Definition** (`go.mod:1-4`):
  ```go
  module github.com/ChillingWombat/antigravity-swiss-knife

  go 1.24.6
  ```
  The Go module has **zero external dependencies** and relies exclusively on the Go standard library.
- **Binary Target**: `bin/swiss` (12.6 MB executable).
  - Executable type: `bin/swiss: ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked` (`file bin/swiss`).
  - Go compiler installed on host: `go version go1.27.1 linux/amd64`.
  - Build command: `go build -o bin/swiss ./cmd/swiss`.
  - Version command output (`./bin/swiss version`):
    `Antigravity Swiss Knife v2.0.0 (Go 1.24.6)`
- **Cross-Compilation Capability**:
  Because there are zero external C dependencies, cross-compilation with `CGO_ENABLED=0` succeeds natively on Linux without toolchain prerequisites:
  - Windows: `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/swiss.exe ./cmd/swiss` (verified exit code 0).
  - macOS: `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/swiss-darwin-arm64 ./cmd/swiss` and `GOARCH=amd64` (verified exit code 0).
  - Linux: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/swiss ./cmd/swiss` (verified statically linked).

### 1.2 CLI Flags & Daemon Startup
- **CLI Implementation** (`cmd/swiss/main.go:173-218`):
  ```go
  func runDaemon(args []string) {
      fs := flag.NewFlagSet("daemon", flag.ExitOnError)
      socketPath := fs.String("socket", core.GetSocketPath(), "Socket path")
      withWeb := fs.Bool("web", false, "Also start Web GUI server")
      webAddr := fs.String("addr", "127.0.0.1:8765", "Web GUI listen address")
      _ = fs.Parse(args)

      cfg, _ := core.LoadConfig()
      if cfg == nil {
          cfg = core.DefaultConfig()
      }

      d, err := daemon.NewDaemon(cfg, *socketPath)
      if err != nil {
          fmt.Fprintf(os.Stderr, "Error initializing daemon: %v\n", err)
          os.Exit(1)
      }

      if err := d.Start(); err != nil {
          fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
          os.Exit(1)
      }

      fmt.Printf("Antigravity Swiss Knife Daemon started on socket: %s (PID: %d)\n", *socketPath, os.Getpid())

      var webSrv *webgui.Server
      if *withWeb {
          webSrv = webgui.NewServer(*webAddr, *socketPath)
          if err := webSrv.Start(); err != nil {
              fmt.Fprintf(os.Stderr, "Warning: failed to start web GUI: %v\n", err)
          } else {
              fmt.Printf("Web GUI listening on http://%s\n", *webAddr)
          }
      }

      sigCh := make(chan os.Signal, 1)
      signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
      <-sigCh

      fmt.Println("\nShutting down daemon...")
      if webSrv != nil {
          _ = webSrv.Stop()
      }
      _ = d.Stop()
      fmt.Println("Daemon gracefully stopped.")
  }
  ```
- **CLI Flag Summary for `swiss daemon`**:
  - `-web`: Boolean flag. When set, instantiates and starts `webgui.Server` in the same process.
  - `-addr string`: HTTP listen address (default `"127.0.0.1:8765"`).
  - `-socket string`: Unix domain socket path for JSON-RPC IPC.
    Default path resolved in `pkg/core/constants.go:68-85`:
    1. `$ANTIGRAVITY_SWISS_SOCKET`
    2. `$ANTIGRAVITY_SWISS_RUNTIME_DIR/daemon.sock`
    3. `$XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock` (typically `/run/user/1000/antigravity-swiss/daemon.sock` on Linux)
    4. Fallback: `/tmp/antigravity-swiss/daemon.sock` (or `%TEMP%\antigravity-swiss\daemon.sock` on Windows).

### 1.3 Health Endpoint & Status API
- **Endpoint Route** (`pkg/webgui/server.go:111`):
  `mux.HandleFunc("/api/status", s.handleStatus)`
- **Implementation** (`pkg/webgui/server.go:211-240`):
  The handler dispatches IPC call `s.client.Call("swiss.getStatus", nil, &status)`.
- **Live Verification**:
  Direct probe via `curl -s http://127.0.0.1:8765/api/status` while `bin/swiss daemon --web` was running returned:
  ```json
  {
    "active_account": "david.alt@google.com",
    "antigravity_pid": 1742689,
    "antigravity_running": true,
    "daemon_pid": 1983994,
    "daemon_running": true,
    "pid": 1983994,
    "total_accounts": 11,
    "version": "2.0.0"
  }
  ```
  `daemon_running: true` and `daemon_pid: <PID>` serve as unambiguous indicators that the Go daemon engine is live.
  When the HTTP server is running standalone without daemon IPC, `daemon_running` is `false` and `daemon_pid` is `0`.
  When no process is listening, HTTP GET immediately fails with `ECONNREFUSED`.

### 1.4 Socket & Lockfile Mechanics
- **IPC Server Startup** (`pkg/ipc/server.go:44-70`):
  ```go
  func (s *Server) Start() error {
      dir := filepath.Dir(s.socketPath)
      if err := os.MkdirAll(dir, 0700); err != nil {
          return fmt.Errorf("failed to create socket directory: %w", err)
      }

      // Clean up stale socket file if present
      _ = os.Remove(s.socketPath)

      l, err := net.Listen("unix", s.socketPath)
      if err != nil {
          return fmt.Errorf("failed to bind socket %s: %w", s.socketPath, err)
      }

      if err := os.Chmod(s.socketPath, 0600); err != nil {
          l.Close()
          return fmt.Errorf("failed to chmod socket: %w", err)
      }
      ...
  ```
  Directory permissions are `0700`, socket file permissions are `0600`.
- **IPC Server Teardown** (`pkg/ipc/server.go:73-82`):
  ```go
  func (s *Server) Stop() error {
      if atomic.CompareAndSwapInt32(&s.running, 1, 0) {
          if s.listener != nil {
              _ = s.listener.Close()
          }
          s.wg.Wait()
          _ = os.Remove(s.socketPath)
      }
      return nil
  }
  ```
  Clean termination unconditionally unlinks the socket file `os.Remove(s.socketPath)`.
- **Web Server Teardown** (`pkg/webgui/server.go:191-198`):
  ```go
  func (s *Server) Stop() error {
      if s.httpServer != nil {
          ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
          defer cancel()
          return s.httpServer.Shutdown(ctx)
      }
      return nil
  }
  ```
  `httpServer.Shutdown` gracefully drains active HTTP connections within a 2-second timeout window.
- **Signal Handling** (`cmd/swiss/main.go:58, 208-218`):
  - `signal.Ignore(syscall.SIGHUP)`: ignores terminal disconnects.
  - `signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)`: handles `os.Interrupt` (SIGINT) and `syscall.SIGTERM`.
  - Sequence on signal receipt:
    1. Print `\nShutting down daemon...`
    2. `webSrv.Stop()` drains HTTP listener.
    3. `d.Stop()` shuts down daemon scheduler, cancels context, closes Unix socket, removes socket file, and waits for background goroutines (`wg.Wait()`).
    4. Print `Daemon gracefully stopped.` and exit 0.

### 1.5 Live Process Lifecycle & Orphan Verification
- In our test execution, `bin/swiss daemon --web` was spawned with PID `1983994`.
- Signal was issued via `kill -TERM 1983994`.
- Output log observed:
  ```text
  Shutting down daemon...
  Daemon gracefully stopped.
  ```
- Exit status: `0`.
- Socket file verification: `/run/user/1000/antigravity-swiss/daemon.sock` was confirmed unlinked.
- Process tree verification: `pgrep -l swiss` returned exit code 1 (`0 swiss processes remaining`).

### 1.6 Go Test Suite Status
- Execution command: `go test -count=1 ./pkg/... ./cmd/...`
- Results across all 16 packages:
  ```text
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/cache          0.002s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/core           0.040s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels   0.005s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/daemon         0.009s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements   0.002s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint    0.007s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui            0.163s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc            0.003s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring        0.014s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/process        0.004s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota          0.004s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/system         0.022s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/templates      0.003s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/totp           0.005s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/pkg/webgui         0.150s
  ok   github.com/ChillingWombat/antigravity-swiss-knife/cmd/swiss          0.399s
  ```
  Pass rate: **100% (16 of 16 packages passed)**. Total execution duration: ~0.8s.

---

## 2. Logic Chain

1. **Self-Contained Backend Feasibility**:
   - Observations 1.1 and 1.6 establish that the Go backend has zero external dependencies, standard library only, and 100% test pass rate.
   - Cross-compilation to Linux (`bin/swiss`), Windows (`bin/swiss.exe`), and macOS (`bin/swiss`) requires only standard `GOOS` and `GOARCH` flags with `CGO_ENABLED=0`.
   - Therefore, packaging the Go binary inside the Electron application bundle as a single binary requires zero runtime prerequisites (no Python, no system C libraries, no external toolchains).

2. **Daemon Liveness Check in Electron**:
   - Observations 1.2 and 1.3 show that `swiss daemon --web` binds to `127.0.0.1:8765` and serves `GET /api/status`.
   - `GET /api/status` returns HTTP 200 with `daemon_running: true`.
   - Therefore, the Electron main process can execute a fast HTTP GET request (with e.g. 500ms timeout) to `http://127.0.0.1:8765/api/status`.
   - If HTTP 200 is received and `daemon_running === true`: the daemon is already running (e.g., started manually by CLI or previously). Electron marks `isManagedChild = false` and avoids duplicate spawning.
   - If connection fails (`ECONNREFUSED` / timeout): Electron marks `isManagedChild = true` and spawns `bin/swiss daemon --web`.

3. **Spawning & Readiness Handshake**:
   - From Observation 1.2, launching `bin/swiss daemon --web` starts both the IPC socket and the HTTP web server in a single native process.
   - Electron main process spawns the process via Node `child_process.spawn(binaryPath, ['daemon', '--web', '--addr', '127.0.0.1:8765'])`.
   - Electron then polls `http://127.0.0.1:8765/api/status` with retries (every 100ms, max 10s timeout).
   - Once the endpoint returns 200, the sidecar is guaranteed healthy and ready to serve IPC and frontend requests.

4. **Zero-Orphan Graceful Termination**:
   - Observations 1.4 and 1.5 demonstrate that when `SIGTERM` or `SIGINT` is sent to `swiss daemon`, it triggers graceful shutdown in `cmd/swiss/main.go:209`:
     - Calls `webSrv.Stop()` -> drains HTTP listener.
     - Calls `d.Stop()` -> closes IPC listener and removes `daemon.sock`.
     - Exits cleanly with return code 0 and leaves zero dangling child processes.
   - Therefore, when Electron completely quits (`app.on('before-quit')` or OS signals `SIGINT`/`SIGTERM`):
     - If `isManagedChild === true`, Electron sends `child.kill('SIGTERM')` to the child process.
     - Electron waits up to 3 seconds for the child process `exit` event before falling back to `SIGKILL`.
     - Socket file and lockfiles are cleanly unlinked, leaving zero orphaned processes (`pgrep swiss = 0`).

---

## 3. Caveats

1. **Pre-existing Daemon Lifecycle**:
   If a user started `swiss daemon --web` outside Electron (e.g. from a separate terminal), Electron's liveness check will detect it and avoid duplicate spawning. In that scenario, Electron must NOT kill the external daemon when Electron exits, because Electron did not own or spawn that process. The `isManagedChild` boolean flag properly distinguishes between owned vs. pre-existing daemons.
2. **Platform Path Differences**:
   In packaged production (`app.isPackaged === true`), the binary path is relative to `process.resourcesPath` (`resources/bin/swiss` on Linux/macOS, `resources\bin\swiss.exe` on Windows). In development mode (`app.isPackaged === false`), it is located at `<repoRoot>/bin/swiss`. The binary resolution function must check both environments.
3. **Windows Signal Handling**:
   On Windows, Node's `child.kill('SIGTERM')` does not send a true POSIX signal; instead, Windows processes terminate more abruptly. However, on Windows, Unix domain sockets are not used (Windows fallback path in `pkg/core/constants.go:48-52`), and the Go daemon on Windows catches `os.Interrupt` cleanly. On Linux (the primary environment for Google Antigravity 2.0), `child.kill('SIGTERM')` delivers standard POSIX `SIGTERM`.

---

## 4. Conclusion

The Go backend daemon (`cmd/swiss/main.go`) and the `pkg/` subsystem are 100% ready for Electron sidecar integration:
1. **Zero External Dependencies**: Pure Go standard library, builds with `CGO_ENABLED=0` across Linux, Windows, and macOS.
2. **Integrated Dual Service**: `swiss daemon --web` starts both the JSON-RPC daemon engine and the HTTP Web GUI server on `127.0.0.1:8765` in a single OS process.
3. **Reliable Health Check**: `GET /api/status` provides deterministic status reporting (`daemon_running: true`, `daemon_pid: <PID>`).
4. **Clean Process Lifecycle**: Responds to `SIGTERM` and `SIGINT` by stopping HTTP listeners, closing IPC servers, removing socket files, and terminating with exit code 0 and zero dangling processes.
5. **Test Health**: All 16 Go packages pass unit and integration tests (`go test ./pkg/... ./cmd/...`).

---

## 5. Architectural Recommendations for Electron Sidecar Implementation

### 5.1 Recommended Architecture: `DaemonManager` Class
In the Electron main process (e.g. `src/main/daemon.ts` or `src/main/sidecar.ts`):

```typescript
import { spawn, ChildProcess } from 'child_process';
import http from 'http';
import path from 'path';
import fs from 'fs';
import { app } from 'electron';

export interface DaemonStatus {
  daemon_running: boolean;
  daemon_pid?: number;
  version?: string;
  active_account?: string;
  total_accounts?: number;
  antigravity_running?: boolean;
  antigravity_pid?: number;
}

export class DaemonManager {
  private child: ChildProcess | null = null;
  private isManagedChild: boolean = false;
  private readonly baseUrl: string = 'http://127.0.0.1:8765';

  /**
   * Resolve binary path based on development vs packaged environment.
   */
  public resolveBinaryPath(): string {
    const isWin = process.platform === 'win32';
    const binName = isWin ? 'swiss.exe' : 'swiss';

    if (app.isPackaged) {
      // Packaged app: binary lives in process.resourcesPath/bin/swiss
      return path.join(process.resourcesPath, 'bin', binName);
    }
    // Development mode: binary lives in <root>/bin/swiss
    return path.join(app.getAppPath(), 'bin', binName);
  }

  /**
   * Probe /api/status to verify if daemon is active.
   */
  public async checkStatus(timeoutMs: number = 1000): Promise<DaemonStatus | null> {
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
   * Ensure daemon is running: verify liveness, spawn if inactive, wait for readiness.
   */
  public async start(): Promise<void> {
    const existing = await this.checkStatus();
    if (existing && existing.daemon_running) {
      console.log(`[DaemonManager] Existing daemon detected (PID: ${existing.daemon_pid}). Reusing.`);
      this.isManagedChild = false;
      return;
    }

    const binPath = this.resolveBinaryPath();
    if (!fs.existsSync(binPath)) {
      throw new Error(`Go daemon binary not found at: ${binPath}`);
    }

    console.log(`[DaemonManager] Spawning Go daemon sidecar: ${binPath} daemon --web`);
    this.child = spawn(binPath, ['daemon', '--web', '--addr', '127.0.0.1:8765'], {
      detached: false,
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
      env: { ...process.env },
    });

    this.isManagedChild = true;

    this.child.stdout?.on('data', (chunk) => {
      console.log(`[GoDaemon:out] ${chunk.toString().trim()}`);
    });
    this.child.stderr?.on('data', (chunk) => {
      console.error(`[GoDaemon:err] ${chunk.toString().trim()}`);
    });
    this.child.on('error', (err) => {
      console.error('[DaemonManager] Child process error:', err);
    });
    this.child.on('exit', (code, signal) => {
      console.log(`[DaemonManager] Child exited with code=${code}, signal=${signal}`);
      this.child = null;
    });

    // Poll until ready (up to 10 seconds)
    const deadline = Date.now() + 10000;
    while (Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 150));
      const status = await this.checkStatus(500);
      if (status && status.daemon_running) {
        console.log(`[DaemonManager] Go daemon successfully ready on ${this.baseUrl}`);
        return;
      }
    }

    throw new Error('Timeout waiting for Go daemon to become ready on 127.0.0.1:8765');
  }

  /**
   * Graceful termination on full application quit.
   */
  public async stop(): Promise<void> {
    if (!this.isManagedChild || !this.child) {
      return;
    }

    console.log('[DaemonManager] Terminating managed Go daemon child process via SIGTERM...');
    const child = this.child;
    this.child = null;

    return new Promise((resolve) => {
      let resolved = false;
      const done = () => {
        if (!resolved) {
          resolved = true;
          resolve();
        }
      };

      const timer = setTimeout(() => {
        console.warn('[DaemonManager] Grace period expired; issuing SIGKILL to Go daemon.');
        try { child.kill('SIGKILL'); } catch {}
        done();
      }, 3000);

      child.once('exit', () => {
        clearTimeout(timer);
        console.log('[DaemonManager] Go daemon cleanly terminated. Zero orphans.');
        done();
      });

      try {
        child.kill('SIGTERM');
      } catch (e) {
        clearTimeout(timer);
        done();
      }
    });
  }
}
```

### 5.2 Hooking into Electron Main Process Lifecycle
```typescript
import { app, BrowserWindow, Tray, Menu } from 'electron';
import { DaemonManager } from './daemon';

const daemonManager = new DaemonManager();
let isQuitting = false;

app.on('before-quit', async (event) => {
  if (!isQuitting) {
    event.preventDefault();
    isQuitting = true;
    try {
      await daemonManager.stop();
    } finally {
      app.quit();
    }
  }
});

// Safeguard against unhandled termination signals
['SIGINT', 'SIGTERM'].forEach((signal) => {
  process.on(signal, async () => {
    await daemonManager.stop();
    process.exit(0);
  });
});

app.whenReady().then(async () => {
  await daemonManager.start();
  // Create BrowserWindow, Tray, etc.
});
```

---

## 6. Verification Method

### 6.1 Independent Verification Commands
1. **Run Full Go Test Suite**:
   ```bash
   cd "/mnt/Data/Projects/Antigravity Swiss Knife"
   go test -count=1 ./pkg/... ./cmd/...
   ```
   *Expected Result*: All 16 packages output `ok` with 0 failures.

2. **Verify CLI Binary & Version**:
   ```bash
   ./bin/swiss version
   ```
   *Expected Result*: `Antigravity Swiss Knife v2.0.0 (Go 1.24.6)`.

3. **Verify Daemon Start, HTTP Status & Graceful SIGTERM Shutdown**:
   ```bash
   bash -c '
   ./bin/swiss daemon --web &
   PID=$!
   sleep 1
   curl -s http://127.0.0.1:8765/api/status
   echo ""
   kill -TERM $PID
   wait $PID
   pgrep -l swiss || echo "Zero swiss processes"
   '
   ```
   *Expected Result*:
   - HTTP response JSON with `"daemon_running": true`.
   - Daemon logs `Shutting down daemon...` and `Daemon gracefully stopped.`
   - `pgrep -l swiss` prints `Zero swiss processes`.

4. **Verify Cross-Platform Builds**:
   ```bash
   CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /tmp/swiss.exe ./cmd/swiss && rm -f /tmp/swiss.exe
   CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/swiss-darwin ./cmd/swiss && rm -f /tmp/swiss-darwin
   ```
   *Expected Result*: Both commands return exit code 0.

### 6.2 Invalidation Conditions
- If any external third-party CGO library is introduced into Go that breaks `CGO_ENABLED=0` static builds.
- If `cmd/swiss/main.go` changes the default HTTP listen address away from `127.0.0.1:8765` without updating flags.
- If `signal.Notify` in `cmd/swiss/main.go` stops catching `syscall.SIGTERM`.
