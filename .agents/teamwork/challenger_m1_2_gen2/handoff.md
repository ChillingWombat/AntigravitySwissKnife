# M1 IPC & Process Lifecycle Adversarial Challenge Handoff Report

**Agent**: `challenger_m1_2_gen2` (M1 IPC & Process Challenger)  
**Date**: 2026-10-01T08:35:00Z  
**Parent Agent**: `parent` (`11f1f26d-e61c-4e23-9c94-5ec9e98e06dd`)  
**Scope Delivered**: Adversarial Stress Testing of Unix Domain Socket IPC and Process Lifecycle (Milestone 1, Features F04, F25)  
**Verdict**: **`REQUEST_CHANGES`**  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/challenger_m1_2_gen2`

---

## 1. Observation

### 1.1 Stress Test Suite Execution Results
All four predecessor stress test suites (`stress_socket_disconnects.py`, `stress_concurrent_clients.py`, `stress_malformed_and_overruns.py`, `stress_process_and_locks.py`) were executed directly in the project environment. All four suites failed or hung, revealing 4 distinct bugs in production code and test design flaws:

| Stress Suite Script | Target Scope | Execution Command | Result | Failure Mechanism |
|---|---|---|---|---|
| `stress_socket_disconnects.py` | Abrupt RST disconnects, large payload broadcasts | `python3 .agents/teamwork/challenger_m1_2/stress_socket_disconnects.py` | **HANG** (Timed out / Canceled after 4m) | Hung on Test 3 during 1MB broadcast due to client buffer overflow + server drain deadlock |
| `stress_concurrent_clients.py` | 60 concurrent clients, pub-sub storm, 1,200 RPCs | `python3 .agents/teamwork/challenger_m1_2/stress_concurrent_clients.py` | **FAILED** (`exit code 1`) | `assert server.client_count == NUM_CLIENTS` failed immediately (`server.client_count == 0`) due to missing event loop yield |
| `stress_malformed_and_overruns.py` | Syntax errors, binary noise, 11MB overruns | `python3 .agents/teamwork/challenger_m1_2/stress_malformed_and_overruns.py` | **FAILED** (`exit code 1`) | `TimeoutError: timed out` at `sock.recv(4096)` caused by blocking socket calls in asyncio event loop thread; unhandled `UnicodeDecodeError` in server |
| `stress_process_and_locks.py` | SingletonLock, dead PIDs, SIGKILL escalation | `python3 .agents/teamwork/challenger_m1_2/stress_process_and_locks.py` | **FAILED** (`exit code 1`) | `assert lock_mgr.lock_file.exists()` failed at line 152; `SingletonLock` dangling symlinks return `False` on `Path.exists()` |

---

### 1.2 Verbatim Observations & Empirical Bug Proofs

#### Bug #1: `AsyncDaemonClient` 64KB Buffer Limit Overrun Crash (Critical)
- **Location**: `antigravity_swiss/ipc/socket_client.py:63`
- **Code**:
  ```python
  self._reader, self._writer = await asyncio.open_unix_connection(str(self.socket_path))
  ```
- **Observation**:
  `asyncio.open_unix_connection` defaults to `limit=65536` (64KB). While the server in `antigravity_swiss/ipc/socket_server.py:120` specifies `limit=MAX_FRAME_SIZE` (10MB), `AsyncDaemonClient` omits the `limit` argument.
- **Empirical Proof**:
  Executed test sending a 100KB payload to `AsyncDaemonClient`:
  ```bash
  python3 -c '
  import asyncio, tempfile
  from pathlib import Path
  from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer
  from antigravity_swiss.ipc.socket_client import AsyncDaemonClient

  async def test_overrun():
      with tempfile.TemporaryDirectory() as td:
          sock = Path(td) / "test.sock"
          server = AsyncUnixSocketServer(sock)
          @server.register("get_big")
          def get_big():
              return "A" * 100000  # 100KB payload
          await server.start()
          client = AsyncDaemonClient(sock)
          await client.connect()
          try:
              res = await client.call("get_big", timeout=2.0)
          except Exception as e:
              print("CLIENT FAILED AS EXPECTED:", type(e), e)
          finally:
              await client.close()
              await server.stop()
  asyncio.run(test_overrun())
  '
  ```
  **Output**:
  ```text
  CLIENT FAILED AS EXPECTED: <class 'antigravity_swiss.core.errors.IPCError'> RPC method 'get_big' timed out after 2.0s
  asyncio.exceptions.LimitOverrunError: Separator is found, but chunk is longer than limit
  ValueError: Separator is found, but chunk is longer than limit
  ```

---

#### Bug #2: `AsyncUnixSocketServer.broadcast_event` Head-of-Line Blocking & Indefinite Deadlock (Critical)
- **Location**: `antigravity_swiss/ipc/socket_server.py:173-180`
- **Code**:
  ```python
  for writer in list(self._clients):
      try:
          writer.write(msg)
          await writer.drain()
          sent += 1
      except (ConnectionResetError, BrokenPipeError, OSError):
          dead_clients.append(writer)
  ```
- **Observation**:
  1. `await writer.drain()` has no timeout (`asyncio.wait_for`).
  2. Iteration across `self._clients` is synchronous and sequential.
  3. If any single client stops reading, has a full kernel buffer, or experiences high socket backpressure, `writer.drain()` hangs indefinitely.
  4. This completely halts `broadcast_event`, preventing ANY subsequent clients from receiving events.
- **Empirical Proof**:
  Executed test connecting one unreading client and broadcasting 200KB events:
  ```bash
  python3 -c '
  import asyncio, tempfile, socket
  from pathlib import Path
  from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer

  async def test_hang():
      with tempfile.TemporaryDirectory() as td:
          sock_path = Path(td) / "test.sock"
          server = AsyncUnixSocketServer(sock_path)
          await server.start()
          s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
          s.connect(str(sock_path))
          await asyncio.sleep(0.05)
          try:
              for i in range(20):
                  print(f"Broadcasting #{i}...")
                  await asyncio.wait_for(
                      server.broadcast_event("test.event", {"data": "X" * 200000}),
                      timeout=1.0
                  )
          except asyncio.TimeoutError:
              print("CONFIRMED BUG: broadcast_event HUNG on writer.drain() due to slow/unreading client!")
          finally:
              s.close()
              await server.stop()
  asyncio.run(test_hang())
  '
  ```
  **Output**:
  ```text
  Broadcasting #0...
  Broadcasting #1...
  CONFIRMED BUG: broadcast_event HUNG on writer.drain() due to slow/unreading client!
  ```

---

#### Bug #3: `ProcessLifecycleManager.relaunch()` 5.0-Second Delay on Symlink Existence (High)
- **Location**: `antigravity_swiss/process/lifecycle.py:204`
- **Code**:
  ```python
  # Startup verification (poll up to 5s)
  deadline = time.monotonic() + 5.0
  while time.monotonic() < deadline:
      if proc.poll() is not None:
          raise ProcessLaunchError(
              f"Antigravity exited prematurely with return code {proc.returncode}"
          )
      if self.lock_manager.lock_file.exists():
          break
      time.sleep(0.2)
  ```
- **Observation**:
  `lock_manager.lock_file` is `~/.config/Antigravity/SingletonLock`, which Electron creates as a symlink pointing to `<hostname>-<PID>`.
  Because `<hostname>-<PID>` does not exist as a physical file on the filesystem, `Path.exists()` follows the symlink and evaluates to `False`.
  Consequently, `self.lock_manager.lock_file.exists()` is **ALWAYS `False`**.
  `relaunch()` never breaks early; every relaunch suffers an artificial 5.0-second delay.
- **Empirical Proof**:
  Executed test launching a process that creates `SingletonLock` immediately:
  ```bash
  python3 -c '
  import os, time, tempfile
  from pathlib import Path
  from antigravity_swiss.process.lifecycle import ProcessLifecycleManager
  from antigravity_swiss.process.lock_manager import SingletonLockManager

  with tempfile.TemporaryDirectory() as td:
      config_dir = Path(td)
      lock_mgr = SingletonLockManager(config_dir)
      script = config_dir / "fake_antigravity.sh"
      script.write_text(f"#!/bin/sh\nln -s \"testhost-$$\" \"{config_dir}/SingletonLock\"\nsleep 10\n")
      script.chmod(0o755)
      mgr = ProcessLifecycleManager(antigravity_bin=script, lock_manager=lock_mgr)
      t0 = time.monotonic()
      pid = mgr.relaunch()
      elapsed = time.monotonic() - t0
      print(f"Relaunch elapsed time: {elapsed:.2f}s")
      os.kill(pid, 9)
  '
  ```
  **Output**:
  ```text
  Relaunch elapsed time: 5.41s
  ```
  Even though the lock was created in < 0.02s, `relaunch()` took 5.41 seconds.

---

#### Bug #4: `SingletonLockManager` and `ProcessLifecycleManager` Zombie Process Blindness (High)
- **Location**: `antigravity_swiss/process/lock_manager.py:83-94` and `antigravity_swiss/process/lifecycle.py:124-128`
- **Code**:
  ```python
  if pid is not None:
      proc_path = Path(f"/proc/{pid}")
      if proc_path.exists():
          is_alive = True
          try:
              cmdline = (proc_path / "cmdline").read_bytes().decode("utf-8", errors="ignore")
              if "antigravity" in cmdline:
                  is_antigravity = True
  ...
  is_orphaned = not (is_alive and is_antigravity)
  ```
- **Observation**:
  In Linux, terminated processes enter state `Z (zombie)` until reaped by their parent via `wait()`.
  While a zombie, `/proc/<pid>` exists and `/proc/<pid>/cmdline` can still be read.
  `SingletonLockManager.inspect_lock()` only checks `proc_path.exists()`. It does not inspect `/proc/<pid>/status` for `State: Z (zombie)`.
  As a result:
  1. A dead zombie process is reported as `is_alive = True` and `is_antigravity = True`.
  2. `is_orphaned` is set to `False`.
  3. `cleanup_orphaned_locks()` refuses to delete the stale lock file, logging:
     `"Cannot cleanup locks: PID <pid> is actively running Antigravity."`
  4. In `terminate_gracefully()`, `Path(f"/proc/{pid}").exists()` remains true for zombie children, causing unnecessary SIGKILL escalation.
- **Empirical Proof**:
  Executed test killing an Antigravity process without immediate parent reaping:
  ```bash
  python3 -c '
  import subprocess, sys
  from pathlib import Path
  proc = subprocess.Popen([sys.executable, "-c", "# antigravity\nimport time; time.sleep(10)"])
  proc.kill()
  print("/proc/<pid> exists?", Path(f"/proc/{proc.pid}").exists())
  for line in Path(f"/proc/{proc.pid}/status").read_text().splitlines():
      if line.startswith("State:"):
          print(line)
  proc.wait()
  '
  ```
  **Output**:
  ```text
  /proc/<pid> exists? True
  State:	Z (zombie)
  ```

---

#### Bug #5: `AsyncUnixSocketServer` Silent Connection Drop on Non-UTF8 / Binary Input (Medium)
- **Location**: `antigravity_swiss/ipc/socket_server.py:211, 221-224`
- **Code**:
  ```python
  line_str = line.decode("utf-8").strip()
  ```
- **Observation**:
  Binary frames (e.g. `b"\x00\x01\x02\xff\xfe\xfd\n"`) trigger `UnicodeDecodeError`.
  This is unhandled before `_process_raw_line()`, falling into `except Exception as exc: logger.error(...)`, which logs a stack trace and closes the connection without sending a JSON-RPC 2.0 `-32700` Parse Error.
- **Empirical Proof**:
  ```bash
  python3 -c '
  import asyncio, tempfile
  from pathlib import Path
  from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer

  async def test_binary():
      with tempfile.TemporaryDirectory() as td:
          sock_path = Path(td) / "test.sock"
          server = AsyncUnixSocketServer(sock_path)
          await server.start()
          reader, writer = await asyncio.open_unix_connection(str(sock_path))
          writer.write(b"\x00\x01\x02\xff\xfe\xfd\n")
          await writer.drain()
          resp = await reader.read(4096)
          print("Response received:", repr(resp))
          await server.stop()
  asyncio.run(test_binary())
  '
  ```
  **Output**:
  ```text
  UnicodeDecodeError: 'utf-8' codec can't decode byte 0xff in position 3: invalid start byte
  Response received: b''
  ```

---

## 2. Logic Chain

1. **Premise 1 (IPC Scalability & Reliability)**:
   In `PROJECT.md § Architecture`, the Unix Domain Socket IPC connects GUI, CLI, and daemon. The GUI continuously receives `notify.quota_updated` and `notify.account_switched` events.
   - *Logic Step*: Under Bug #1, any large JSON-RPC response or broadcast (> 64KB) immediately breaks `AsyncDaemonClient` with `LimitOverrunError`.
   - *Logic Step*: Under Bug #2, if any connected client lags or disconnects without an immediate FIN/RST, `broadcast_event` deadlocks indefinitely on `writer.drain()`, freezing the daemon's event notification loop.
   - *Conclusion*: IPC fails reliability and concurrency requirements under multi-client or large-payload conditions.

2. **Premise 2 (Process Lifecycle & Account Switch Latency)**:
   In `PROJECT.md § F04`, process termination and relaunch must be fast, zero-loss, and reliable during account switches.
   - *Logic Step*: Under Bug #3, `relaunch()` checks `lock_file.exists()` on a symlink, which always evaluates to `False`.
   - *Logic Step*: Every account switch incurs a guaranteed 5.0s hang before `relaunch()` returns.
   - *Logic Step*: Under Bug #4, if a terminated Antigravity process becomes a zombie prior to parent cleanup, `inspect_lock()` treats it as actively running and refuses to clean up `SingletonLock`, which can block subsequent process launches.
   - *Conclusion*: Process lifecycle management contains severe latency penalties and race conditions around lock sanitization.

3. **Premise 3 (Specification Conformance)**:
   JSON-RPC 2.0 requires code `-32700` (`Parse error`) when invalid bytes or malformed frames are received.
   - *Logic Step*: Under Bug #5, non-UTF8 sequences trigger an unhandled `UnicodeDecodeError` that aborts the socket with 0 bytes response.
   - *Conclusion*: IPC server violates RFC specification error handling for non-UTF8 inputs.

---

## 3. Caveats

1. **Predecessor Test Flaws**:
   The predecessor stress scripts in `.agents/teamwork/challenger_m1_2/` were themselves buggy (blocking calls in event loops, missing `asyncio.sleep` yields to allow socket callbacks to register). However, once these harness artifacts were isolated, the underlying production code bugs (Bugs 1-5) were empirically reproduced in clean, minimal scripts.
2. **PySide6 UI**:
   The PySide6 UI client was not tested with live Qt event loops, as testing focused strictly on headless daemon Unix domain sockets and process management per Milestone 1 scope.
3. **No Implementation Changes Made**:
   In strict accordance with the Empirical Challenger role constraints ("Review-only — do NOT modify implementation code"), no production files in `antigravity_swiss/` were altered.

---

## 4. Conclusion & Required Actions

### Verdict: **`REQUEST_CHANGES`**

Milestone 1 Core IPC & Process Lifecycle cannot be approved until the following 5 concrete remediation items are addressed by the implementer:

1. **Fix `AsyncDaemonClient.connect()` Frame Limit**:
   In `antigravity_swiss/ipc/socket_client.py:63`, pass `limit=MAX_FRAME_SIZE` to `asyncio.open_unix_connection(str(self.socket_path), limit=MAX_FRAME_SIZE)`.
2. **Harden `AsyncUnixSocketServer.broadcast_event()` Against Slow Clients**:
   In `antigravity_swiss/ipc/socket_server.py:173-180`:
   - Wrap `writer.drain()` in a timeout: `await asyncio.wait_for(writer.drain(), timeout=0.5)`.
   - Handle `asyncio.TimeoutError` by discarding the delinquent client and pruning it from `self._clients`.
   - Alternatively, broadcast to all clients concurrently using `asyncio.gather(*[...], return_exceptions=True)`.
3. **Fix Symlink Existence Check in `ProcessLifecycleManager.relaunch()`**:
   In `antigravity_swiss/process/lifecycle.py:204`, replace `if self.lock_manager.lock_file.exists():` with `if self.lock_manager.lock_file.is_symlink() or os.path.lexists(self.lock_manager.lock_file):`.
4. **Add Zombie State Detection in `SingletonLockManager` and `ProcessLifecycleManager`**:
   In `antigravity_swiss/process/lock_manager.py:83-94`:
   Check `/proc/{pid}/status` for `State: Z (zombie)`. If state is zombie, set `is_alive = False` and `is_orphaned = True`.
   In `antigravity_swiss/process/lifecycle.py:124-128`, exit poll loop early if process state is zombie.
5. **Handle Non-UTF8 Decoding in `AsyncUnixSocketServer._handle_client()`**:
   In `antigravity_swiss/ipc/socket_server.py:211`, wrap `line.decode("utf-8")` in a `try...except UnicodeDecodeError:`. Upon decode failure, return a `-32700` ParseError frame: `json.dumps({"jsonrpc": "2.0", "error": ParseError("Invalid UTF-8 encoding").to_rpc_error(), "id": None})`.

---

## 5. Verification Method

Once the implementer applies the fixes, verify using the following commands:

### 5.1 Verify Large Payload (>64KB) Handling in Client
```bash
python3 -c '
import asyncio, tempfile
from pathlib import Path
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer
from antigravity_swiss.ipc.socket_client import AsyncDaemonClient

async def run():
    with tempfile.TemporaryDirectory() as td:
        sock = Path(td) / "test.sock"
        server = AsyncUnixSocketServer(sock)
        server.register("big", lambda: "A" * 100000)
        await server.start()
        client = AsyncDaemonClient(sock)
        await client.connect()
        res = await client.call("big")
        assert len(res) == 100000
        await client.close()
        await server.stop()
        print("VERIFIED: 100KB payload roundtrip passed.")
asyncio.run(run())
'
```

### 5.2 Verify Broadcast Does Not Hang on Slow/Unreading Clients
```bash
python3 -c '
import asyncio, tempfile, socket
from pathlib import Path
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer

async def run():
    with tempfile.TemporaryDirectory() as td:
        sock = Path(td) / "test.sock"
        server = AsyncUnixSocketServer(sock)
        await server.start()
        s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        s.connect(str(sock))
        await asyncio.sleep(0.05)
        for i in range(10):
            await asyncio.wait_for(server.broadcast_event("test", {"d": "X" * 200000}), timeout=1.0)
        s.close()
        await server.stop()
        print("VERIFIED: broadcast_event resilient against unreading socket.")
asyncio.run(run())
'
```

### 5.3 Verify `relaunch()` Returns Immediately (< 1.0s) Upon Lock Symlink Creation
```bash
python3 -c '
import os, time, tempfile
from pathlib import Path
from antigravity_swiss.process.lifecycle import ProcessLifecycleManager
from antigravity_swiss.process.lock_manager import SingletonLockManager

with tempfile.TemporaryDirectory() as td:
    config_dir = Path(td)
    lock_mgr = SingletonLockManager(config_dir)
    script = config_dir / "fake.sh"
    script.write_text(f"#!/bin/sh\nln -s \"host-$$\" \"{config_dir}/SingletonLock\"\nsleep 10\n")
    script.chmod(0o755)
    mgr = ProcessLifecycleManager(antigravity_bin=script, lock_manager=lock_mgr)
    t0 = time.monotonic()
    pid = mgr.relaunch()
    elapsed = time.monotonic() - t0
    os.kill(pid, 9)
    assert elapsed < 1.5, f"Relaunch took too long: {elapsed}s"
    print(f"VERIFIED: Relaunch detected symlink in {elapsed:.2f}s (< 1.5s).")
'
```
