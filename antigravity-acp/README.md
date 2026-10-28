# antigravity-acp

Serves an Antigravity surface to any ACP client (Zed, Neovim, our own daemon) over stdio.

## Surfaces

- `ls` — the Antigravity language server, the runtime behind the desktop app and the
  VS Code extension. Talks Connect RPC over loopback HTTP. Port comes from the server
  log, the CSRF token from the process command line (procfs on Linux, `ps` on macOS,
  CIM on Windows).
- `cli` — the `agy` binary in `--output-format stream-json` mode. Used when no server
  is running, and the only option on machines without the desktop app.

`--surface auto` (default) prefers `ls`, falls back to `cli`.

## What is honest about it

- Tool calls execute without an approval prompt. The agent-API cascade
  (`CORTEX_TRAJECTORY_SOURCE_AGENT_API`) edits files and runs commands on its own; the
  server emits no interaction step to answer. Clients that need approvals must gate
  the request themselves or use a read-only mode.
- `session/close`, `session/resume`, `session/fork` answer `-32601`. They are not
  implemented and are not advertised.
- `session/list` and `session/load` are real: summaries come from the server, and load
  replays recorded planner responses before returning.

## Run

```sh
go build ./cmd/antigravity-acp
./antigravity-acp --surface ls        # or cli, or auto
```

Point an editor at that command. stdout carries the protocol; logs go to stderr.

To exercise an existing conversation through the language-server surface:

```sh
go run ./cmd/acp-probe \
  --server "/tmp/antigravity-acp --surface ls" \
  --session <conversation-id> \
  --cwd "/path/to/project" \
  --prompt "Status check"
```

## Tests

```sh
go test ./...
```

The `ls` tests run against a fake Connect server that asserts the CSRF header, cascade
source, workspace URI, planner streaming, cancel flag and trajectory listing. The `cli`
tests replay a recorded `agy` stream.
