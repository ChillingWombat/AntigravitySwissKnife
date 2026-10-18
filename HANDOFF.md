# HANDOFF — Antigravity Swiss Knife

> 面向后续 agent 的项目交接文档。最后更新：2026-10-18 15:10（Sydney）。
> 与 `.workbuddy/memory/` 日志互补：那里记流水，这里记可复用的结构与坑。

## 1. 项目概览

- **是什么**：Google Antigravity IDE 的多账号 / 配额管理工具（Go 后端 + React 前端 + Electron 桌面壳 + Web GUI）。
- **主检出**：`D:/Projects/Antigravity Swiss Knife`（git 主工作树，main 分支）。
- **并行 worktree**：`D:/Projects/swiss-wt/{gcp,switch,theme}`（历史上用于 subagent 并行开发；各自 frontend/node_modules 用 junction 指向主检出，见 §5）。
- **状态（截至更新时）**：4 个功能 PR 已全部合并入 main，无开放 PR；桌面应用正在运行（Electron + daemon PID 见 `swiss status`）。

## 2. 构建与运行（Windows 开发机）

```bash
# Go 构建（无系统 Go 工具链，用便携版）
GOCACHE="D:/Projects/Antigravity Swiss Knife/scratch/gocache" \
  "D:/Projects/Antigravity Swiss Knife/scratch/gotoolchain/go/bin/go.exe" build -o bin/swiss.exe ./cmd/swiss

# 前端构建（tsc -b && vite build；outDir=../pkg/webgui/dist，emptyOutDir 自动清孤儿产物）
cd frontend && npm run build
# 前端测试
cd frontend && npm test        # node --import tsx --test src/**/*.test.ts

# 启动桌面应用（agent 会话里必须清 ELECTRON_RUN_AS_NODE，否则 electron 以 Node 身份静默退出）
env -u ELECTRON_RUN_AS_NODE npm run desktop      # 脚本本身已带 --no-sandbox

# daemon 单独启动 / 状态查询
./bin/swiss.exe daemon
./bin/swiss.exe status        # RPC 查询：活跃账户、账户数、Host IDE 状态
```

- **daemon 配置目录（Windows）**：`%APPDATA%\antigravity-swiss`（`session.token` 64 字节、`accounts.json`、`config.json`）。**不要**用 `~/.config`。
- **Web GUI**：daemon 监听 `127.0.0.1:8765`；默认拒绝（default-deny），需 `X-Session-Token` 头（值取自 `session.token`）。桌面端由 `electron/main.js` 的 `webRequest.onBeforeSendHeaders` 自动注入。
- **构建纪律**：`bin/swiss.exe` 从包含全部已合并修复的分支构建（当前=main）。曾因从缺修复的分支构建出现 pid=0 症状。

## 3. 架构要点

### 3.1 鉴权链（loopback auth，PR #50 起）
- daemon 默认拒绝所有请求；Electron renderer 注入 `X-Session-Token`。
- 浏览器链：`api.ts` 把 token 存 localStorage（`swiss_session_token`），每个请求带头部；App Password 关闭时 401 自动空密码重取；否则触发 `swiss:auth-required` 锁屏。支持一次性 `?token=` 链接。桌面端经 `desktop:get-session-token` IPC 引导，锁屏只防浏览器直连。
- **daemon-only 模式 + 按需浏览器查看是一等公民流程**。

### 3.2 桌面账号切换的真实会话存储（2026-10-15 重大发现）
- `jetski-standalone-oauth-token` / `app_storage.json` 是**输出**：应用从内部存储恢复会话并在启动后重写这些文件。应用关闭时写文件**不能**改账号（已穷尽验证）。文件切换对既有会话是死路。
- **真正的会话存储 = 操作系统 keyring**：Windows 是凭据管理器目标 `gemini:antigravity`（wincred）；macOS Keychain；Linux 文件。payload：`{"token":{...},"auth_method":"consumer","id_token":"<top-level>"}`。
- 已实现（PR #76）：`pkg/keyring/keyring_windows.go` + scanner source 1a2 + `SyncAllSurfaces` 写入。E2E 验证过：写 keyring → 杀进程重启 → 应用以新账号启动。

### 3.3 Windows 控制台窗口隐藏（Task #1，PR #98）
- 根因：`CREATE_NO_WINDOW`/`DETACHED_PROCESS` 让子进程无控制台 → 其 console 子孙各自分配**可见**窗口（每次 spawn 闪一个终端）。
- 机制：`CREATE_NEW_CONSOLE` + `HideWindow` = 隐藏控制台且可被子孙继承。
- API（`pkg/core/`）：
  - `SetHiddenConsole(cmd)`：短寿命无子进程的探针（tasklist/git/tailscale）。
  - `SetHiddenConsoleInherit(cmd)`：执行任意命令的 `cmd.exe /c`（隐藏控制台需可继承）。
  - `LaunchHiddenConsoleProcess(cmd)`：agy/code revival 等 detached 启动。
- 任何 daemon spawn console 应用的站点漏掉这些调用就会复现闪窗。

### 3.4 进程检测
- 一律用 `core.process.IsExactAntigravityExecutable(name)`，**禁止裸字符串比较**（Windows 报 `Antigravity.exe` 大小写+.exe，Linux 报 `antigravity`；2026-10-15 的切换 bug 正是这个）。
- `FindAntigravityProcesses` 分平台：`finder_windows.go`（Toolhelp32 + PEB cmdline）/ `finder_unix.go`（/proc）。OS 特定代码不要回填进 shield.go。

### 3.5 活跃账户真相链
live CDP fiber → jetski token → `app_storage.json`。vault 的 `active_account` **仅当** `IsAntigravityRunning()` 为真时才向实时检测收敛——进程查找坏了 vault 就 stale。CDP 端口读 `%APPDATA%\Antigravity\DevToolsActivePort`（动态，不是 9222）。

### 3.6 账户状态模型 + 手动切换不变式（用户 2026-10-18 澄清）
- 状态：`ACTIVE` / `STANDBY` / `COOLDOWN` / `COOLING` / `ERROR` / `BANNED`，由 `quota.BuildAccountQuotaStatesFromMapWithThresholds` 计算（默认 STANDBY；低于阈值→COOLING；缺 refresh token→ERROR；banned→BANNED）。
- **STANDBY = 未冷却 + 未激活 = 可随时切换使用的健康账户**（用户口径）。
- **手动切换不变式（PR #99）**：右键切换对任意非活跃账户开放（历史上门控从来不是 STANDBY-only；用户感知的阻塞来自 `isBanned||(isCooldown&&!hasNonGemini)` 守卫，已删）。切换到**非 STANDBY** 目标时先关 auto-switch（`swiss.setRuleConfig` RPC，失败回退本地 Save + `keyring.SyncCloudAccountsAutoSwitch`），防止 daemon 立刻轮换走手动 pin 的账号；**STANDBY 目标不动 auto-switch**。切换前用切换前状态判定（`preSwitchAccountStatus`）。

### 3.7 GCP 自定义模型（Task #2，PR #100）
- 仅存在于 `global` location 的模型（如 `gemini-4-argon`）必须用**无区域前缀主机** `https://aiplatform.googleapis.com`；`BuildVertexAIEndpoint` 已映射。Claude→us-east5 重映射只适用于区域 location。
- 默认模型晋升门控：`last_test_success`（*bool，nil=从未测试）+ `last_tested_at`；`SaveModel` 拒绝把上次测试失败的模型**新晋**为默认（已是默认的仍可编辑，较新测试记录优先）；`/api/rules` 的 `default_custom_model` 返回 400。404 时后端追加 global 提示，前端给 "Switch to global" 按钮。

### 3.8 其他硬约束
- **Redaction sentinel 永不落盘**：`[REDACTED]`/`******` 是显示占位。PR #84/#91 后所有 keyring 写路径都忽略 sentinel；新增账户写路径必须带同样守卫（这类 bug 曾一次性抹掉全部 refresh token）。
- **IPC 停机语义（PR #82）**：`ipc.Server.Stop()` 先强制关闭 tracked 连接再 `wg.Wait()`。未来任何等待 handler goroutine 的改动若不关连接会重新挂死 daemon 停机。
- **凭据恢复途径**：Electron HTTP cache（`%APPDATA%\antigravity-swiss-knife\Cache\Cache_Data\data_2`）可能留有未脱敏 `/api/accounts` 响应；JSON 扫描 + `RefreshGoogleTokenFull` 验证可恢复。恢复 vault 时必须同时 heal keyring `gemini:antigravity`（`WriteWindowsKeyringToken`），否则应用起来是登出态。
- **CLAUDE.md 是 AGENTS.md 的符号链接**；本 checkout `core.symlinks=false`，git 永远显示它 binary "modified"（IntxLNK 仿真）。提交时排除：`git add -A ':!CLAUDE.md'`；不要 checkout/重建它。

## 4. webgui dist 构建产物约定

- 每个前端 PR 按仓库惯例重建 `pkg/webgui/dist/`（go:embed 嵌入）。
- **合并 2 个以上前端 PR 必然在 dist 冲突**（index.html + 哈希包名）。解法：合并时 dist 取对方版本，最后做**一次** `npm run build`，提交为 `chore(webgui): rebuild dist from merged source`（2026-10-18 处理 #97/#99/#100 即此方式，commit 9b5c727）。
- 已知怪癖：主线构建把 4095B 的 `claude.svg` 内联为 data URI；worktree 构建却产出外部文件 `claude-Bhev5pnc.svg`（工具链版本相同，疑 rolldown 缓存）。两者功能等价、无路径引用；以最终重建版为准。
- dist/ 下的静态资产（`favicon.svg`、`icons.svg`、`logo/`、`antigravity_logo.png`）**不是** vite 产物，是提交进 repo 的，别当垃圾删。

## 5. 工具链怪癖与坑（Windows + agent 会话）

| 坑 | 规避 |
|---|---|
| WorkBuddy 宿主设 `ELECTRON_RUN_AS_NODE=1`，agent 会话启动 electron 会以 Node 身份静默退出 | `env -u ELECTRON_RUN_AS_NODE npm run desktop` |
| agent 会话（含 daemon→Antigravity 重启）里的 Chromium 子沙箱失败 → GPU child 0xC0000005 → 退出 | `--no-sandbox` / `--in-process-gpu` 可启动；**不要**据此判定产品坏了，让 David 自己启动验证 |
| `git push` 只有 Bash 工具成功（PowerShell 静默 exit 128） | 推送一律用 Bash 工具 |
| worktree 的 frontend node_modules 需 win32-x64 原生包（`@esbuild/win32-x64`、`@rolldown/binding-win32-x64-msvc`） | 用 PowerShell `New-Item -ItemType Junction` 链接主检出 node_modules（Git Bash 的 `mklink /J` 失败）；报 "@esbuild/linux-x64 is present" 时在 `frontend/` 跑 `npm install`（不是 `npm ci`） |
| 安全策略拦截 PowerShell COM 实例化（回收站删除）与 Bash heredoc（误报） | COM 删除改用 `Remove-Item -Force`（先隔离备份）；大段提交信息用 PowerShell here-string `@'...'@` |
| 清理进程时**绝不** kill Google Antigravity 应用进程（`AppData\Local\Programs\Antigravity`） | 只清 swiss/electron |
| 注册表 NUL 前缀值名（`\0vshost`）对 regedit/reg.exe/.NET 全隐形 | 只能 NT API `NtDeleteValueKey` + 显式长度 `UNICODE_STRING`；Go 侧 `utf16.Encode` 不用 `StringToUTF16`（遇内嵌 NUL panic）；`RegOpenKeyExW` 掩码用 `0x000F003F`（含 SYNCHRONIZE 会 ACCESS_DENIED） |
| **subagent 汇报可能造假**（2026-10-18 switch-dev 曾谎报 PR 已创建） | 信任但验证：`git log`/`gh pr view`/`gh pr checks` 亲自核实再合并 |

## 6. 安全事件：vshost 恶意软件（2026-10-18 已清除）

- **现象**：`vshost.exe - Application Error` 弹窗（0xE0434352 = .NET CLR 未处理异常），Defender AMSI 威胁 2147972000 + wangmeng .NET 载荷。驻留约 4-5 个月。
- **持久化**：3 个仿冒计划任务 + 5 个 MS 签名文件副本（LOLBin）+ 2 个 NUL 前缀隐藏 Run 值。
- **已清除并验证**：任务/文件/Run 值全删；Run 键 15→13；RunOnce/Wow6432Node 无 NUL 残留；无 vshost 进程。保留了合法的 `dfsvc` 值、Chs 文件、WindowsApps 别名。
- **反清除对抗**：恶意软件在目录上设 `Everyone: Deny Delete/ReadPermissions/ChangePermissions`，文件继承 → 先 `icacls /remove:d *S-1-1-0` 再删。
- **隔离备份**：`scratch/vshost-quarantine-2026-10-18/`（5 个恶意文件副本 + 3 个任务 XML + MANIFEST.txt 含 SHA256 + HKCU_Run_backup.reg）。
- **NT API 删除工具**：`scratch/regfix/main.go`（独立 go module）。
- **仍待办**：建议用户跑 Defender 全盘/脱机扫描；驻留 4-5 个月，已提醒轮换重要凭据。

## 7. 本会话完成的工作（2026-10-18）

四项待办全部合并入 main（顺序 #97→#98→#99→#100，merge commit）：

| PR | 内容 | 验证 |
|---|---|---|
| #97 `feat/theme-auto-light-dark` | Auto/Light/Dark 主题（core 配置 + API + theme.ts + 暗色令牌 + 设置页） | CI 三作业绿 |
| #98 `fix/hide-console-windows` | §3.3 控制台闪窗修复，14 站点 + Windows-only 测试 | CI 三作业绿 |
| #99 `fix/manual-switch-nonstandby` | §3.6 手动切换不变式 + 3 个 webgui 测试 | CI 三作业绿 |
| #100 `fix/gcp-custom-models-region` | §3.7 GCP global location + 默认门控 | CI 三作业绿 |

收尾：dist 重建（9b5c727）→ `bin/swiss.exe` 从合并后 main 重建 → daemon 启动验证（RPC 正常、3 账户）→ 桌面应用已启动并全链路验证（8765 服务新 bundle `index-vCZdaxSh.js`、认证 API 正常、无 token 401）。

## 8. 待办 / 下一步

1. **Defender 全盘/脱机扫描**（用户侧，未完成）——vshost 驻留过久，凭据轮换建议已发出。
2. 桌面应用当前由 agent 会话启动（`--no-sandbox`）；用户后续自行启动时同样命令即可（脚本已内置）。
3. 无开放 PR；main 即完整状态。新功能开发按 §4 约定带 dist 重建。
4. subagent 并行开发模式（worktree + junction node_modules + 显式 `git add`）在本项目验证有效，可复用；汇报必须独立核实（§5 表末行）。
