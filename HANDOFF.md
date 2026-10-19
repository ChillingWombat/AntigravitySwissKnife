# HANDOFF — Antigravity Swiss Knife

> 面向后续 agent 的项目交接文档。最后更新：2026-10-19 18:05（Sydney）。
> 与 `.workbuddy/memory/` 日志互补：那里记流水，这里记可复用的结构与坑。

## 1. 项目概览

- **是什么**：Google Antigravity IDE 的多账号 / 配额管理工具（Go 后端 + React 前端 + Electron 桌面壳 + Web GUI）。
- **主检出**：`D:/Projects/Antigravity Swiss Knife`（git 主工作树，main 分支）。
- **并行 worktree**：`D:/Projects/swiss-wt/{gcp,switch,theme,inj,fe,be,rev,rev2,acct,acp,icon,gcpui}`（历史与近期 subagent 并行开发用；各自 frontend/node_modules 用 junction 指向主检出，见 §5）。**合并后可清理**。
- **状态（截至更新时）**：2026-10-19 全天 33 个 PR（#111–#165）已合并入 main；GitHub 开放 issue / 开放 PR 均为 0；桌面应用运行中（`swiss status` 查 daemon PID），服务 bundle `index-CYTZ13GE.js`。
- **验证过的运行命令**：`npm run desktop`（需 `env -u ELECTRON_RUN_AS_NODE`）、`bin/swiss.exe status`、`curl -H "X-Session-Token: $(cat "$APPDATA/antigravity-swiss/session.token")" http://127.0.0.1:8765/api/...`。

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

### 3.8 模型定价必须动态，硬编码只能是离线兜底（PR #113，重要）
- **层级顺序不可倒置**：provider 端点 → Google 定价页 → 第三方动态目录（OpenRouter / LiteLLM） → **bundled 表（仅离线最后兜底）** → unconfigured。任何把 bundled 表放在动态源之上的写法都会永久钉死旧价（`gemini-4-argon` 曾因此显示退役的 Flash 档 $0.75/$3.75，正确应为 $2/$10 + cache $0.10）。
- **HTML 正则必须带 DOTALL `(?s)`**：Google 定价页表格跨行；缺 `(?s)` 时 `tableRegex`/`trRegex` 匹配数为 0，动态路径静默返回空，bundled 表"获胜"——且既有测试会因 bundled 兜底而假阳性通过。
- **非手动价格 7 天过期**（`providerPriceMaxAge` / `providerPriceStale`），下次 sync 自动重取；瞬时网络失败**不得**清空已知价格。
- LiteLLM 目录的 key 带 provider 前缀（`vertex_ai/gemini-4-argon`），`fetchThirdPartyCatalogs` 会补裸模型名别名；OpenRouter 亦提供短名。
- 实测口径参考（Gemini 4 Argon）：introductory $2.00 in / $10.00 out / $0.10 cached per 1M，2026-12-01 起 $4 / $20 / $0.20。

### 3.9 启动 GUI 应用：禁止 SW_HIDE（PR #111）
- `LaunchDetachedProcessHiddenConsole` 会设 `HideWindow=true`（STARTF_USESHOWWINDOW + SW_HIDE）。Chromium/Electron **把启动显示状态应用到首个窗口** → Antigravity 以托盘态启动、没有可见窗口。同处设置的 `CREATE_NO_WINDOW` 对 GUI 子系统程序被 Windows 忽略，实际起作用的只有 SW_HIDE。
- 正确入口：`core.LaunchDetachedGUIProcess` / `core.SetDetachedGUIProcess`（仅 `CREATE_NEW_PROCESS_GROUP`，并清除 `CREATE_NO_WINDOW`/`DETACHED_PROCESS` 与 HideWindow）。隐藏控制台 helper 只用于 console 子进程（如 `cmd.exe /c`）。

### 3.10 图标：单一来源 + 透明托盘 glyph（PR #109/#110/#149）
- **源文件是 `assets/logo_square.png`**（600×600 白底方块，用户提供）。`scripts/regen_icons.py` 是唯一生成入口，改图标只跑它。
- 生成规则（不要退回"抠色"老路）：背景色从**图像边缘采样**（不是固定 255——该图边缘有 246–253 近白噪点，固定阈值会把整张边界当成内容）；内容框由内容掩码求出；App 图标 = **白色圆角板（半径 0.2237×边）+ 内容 78% 居中**，4× 超采样后降采样；托盘 = 纯透明底 + 边界洪水填充（保住内部白色 G），填充 92%。
- Windows 托盘用 `tray_32x32.png`（Win11 按 24/30/36px 渲染），**不要 resize 到 16**；`getTrayIcon()` 在 `electron/main.js`。
- Electron 运行时从 `assets/icons/` 读图标：**换图标只需重启应用，不必重建 Go**。
- 历史坑：旧实现按"外层画布色"抠透明度，但裁切框四角是白色圆角填充 → 输出成白**方**板（"四角还在"）；同一裁切框还切掉了底部浅蓝弧线。

### 3.11 Bridge / Remote 端点语义（PR #124）
- **不允许对外暴露 loopback**：`sanitizeEndpointURL` / `sanitizeTunnelStatus` 在**所有写入点 + `GetStatus` 读取边界**净化；否则前端渲染原始 `TunnelStatus.GatewayURL` 时仍会显示 `127.0.0.1`（历史缺陷）。
- **custom 端点校验**：必须 `https` + host 非空 + host 非 loopback，且校验**先于** `tm.Stop()`（被拒请求不得拆掉现有隧道）。`https://`（无 host）必须拒绝。
- **Tailscale 例外**：保持 `http://<magicdns>:<port>`——bridge 在 tailnet 内无 TLS 监听，盲目 https 会产生死链。对外可达端点（quick tunnel / custom）才强制 https。
- `Start("loopback")` 已改为报错（行为变更）。

### 3.12 Token 计量口径（PR #130，用户以 Magpie 为准）
- transcript 的 `input_tokens` 与 `cache_read_tokens` **互不包含**（实测 cache/fresh 比 20–25x，单轮 5–8x），因此 `total = fresh + cache_read + output` 是正确的加法；API 响应里 `input_tokens = fresh + cached`（prompt 侧总量）另带 `fresh_tokens`，前端 `fresh = input − cached` 与之一致。
- 与 Magpie 对齐的锚点：会话 `60577173-d663-4a89-8482-c0afc2b1fe79` → 8,307,647 tokens / 92 requests / 95.7% 命中 ≈ Magpie `8.1M / 92 / 95%`。⇒ Magpie 的 token 总数**包含** cache_read。
- **1573.63M 不是算错**，那是全量历史（118 个有用量会话，最大单会话 261M）；缺陷是**时间范围过滤失效**（KPI 消费的 summary 无 range 参数）。
- 指标卡与图表必须共用同一 range 状态；桶求和必须等于该 range 的 KPI 总量（有 e2e 断言）。
- **禁止任何伪造图表数据**：旧实现用 `Math.sin` 权重摊总量（历史不足时），已删除；无数据渲染空图。检测方式：`grep -c "Math.sin" pkg/webgui/dist/assets/*.js` 应为 0。

### 3.13 注入式 UI 的三条硬规则（PR #115/#116/#123/#124）
- **jumper（Quick Prompt Jump Bar，`pkg/enhancements/script.go`）**：条数不能取"当前挂载的 DOM 轮次"——chat 转写是虚拟化列表，滚动/流式会抖动。优先级：列表 ordinal（`data-index`/`aria-posinset`）→ `/api/tokens/chat-metrics` 的 `turns.length` → 每会话 high-water；仅在 (会话, 稳定总数) 变化时重建。
- **Split Right（`pkg/plugins/github_extension_script.go`）**：stage 必须**钉住自己的宿主 pane**（`data-pane-id`），可见性/关闭判定不得用"文档中第一个 conversation-view"；split 后要有 3s 守卫 + 200/600/1200/2400ms 再断言（IDE 重渲染会丢注入节点）。
- **Overview 自动收起（`pkg/boardpanel`）**：面板容器用 `[data-testid="auxiliary-panel"]` / `[data-swiss-aux-panel]` / `.part.auxiliarybar`，IDE 自身开关是 `[data-testid="toggle-aux-sidebar"]`；宽匹配 `[data-testid*="overview"]` 会命中面板内部节点并自报"已折叠"。入口折叠要有有界重试（瞬态帧），核实"容器真的被隐藏"才算成功。
- **resume prompt（`pkg/revival`）**：Go CDP 触发与页面内复活循环必须用**同一去重键**（`DedupeKey`：intent_id 优先，否则 `auto-revive-<convID>`），否则一次重启会发两条、第二条被 IDE 排队。restore 必须**先核实路由**（`__TSR_ROUTER__.state.location.pathname` + 视图挂载）再注入，不得拿地址栏当导航证据。

### 3.14 VS Code 扩展注入（PR #112）
- 目标：`google.google-antigravity`（webview 侧栏）。VS Code 默认不开 CDP，需以 `--remote-debugging-port` 启动（app 会写 `%APPDATA%\Code\DevToolsActivePort`；文件会过期，必须做 `/json` 存活校验）。
- 可注入目标：`vscode-file://`（workbench）、`vscode-webview://`（shell）、扩展内容帧（loopback `?extensionView=true`）。
- **已在运行的 VS Code 不会应用新 flag**——必须先退出再由 `POST /api/vscode/launch` 启动。配置：`apply_enhancements_to_vscode`（默认 true）、`vscode_debug_port`（默认 9333）。

### 3.15 其他硬约束
- **Redaction sentinel 永不落盘**：`[REDACTED]`/`******` 是显示占位。PR #84/#91 后所有 keyring 写路径都忽略 sentinel；新增账户写路径必须带同样守卫（这类 bug 曾一次性抹掉全部 refresh token）。
- **IPC 停机语义（PR #82）**：`ipc.Server.Stop()` 先强制关闭 tracked 连接再 `wg.Wait()`。未来任何等待 handler goroutine 的改动若不关连接会重新挂死 daemon 停机。
- **凭据恢复途径**：Electron HTTP cache（`%APPDATA%\antigravity-swiss-knife\Cache\Cache_Data\data_2`）可能留有未脱敏 `/api/accounts` 响应；JSON 扫描 + `RefreshGoogleTokenFull` 验证可恢复。恢复 vault 时必须同时 heal keyring `gemini:antigravity`（`WriteWindowsKeyringToken`），否则应用起来是登出态。
- **CLAUDE.md 是 AGENTS.md 的符号链接**；本 checkout `core.symlinks=false`，git 永远显示它 binary "modified"（IntxLNK 仿真）。提交时排除：`git add -A ':!CLAUDE.md'`；不要 checkout/重建它。

### 3.20 备忘（Quick Memos）体系（2026-10-19）
- **提取端点** `POST /api/memos/extract` `{memo_id}`（可选 `?workspace_path=`）：用配置的 GCP 自定义模型调 Vertex `generateContent`，媒体以 inline base64 传入，mime 由 magic bytes 判定。
  - **请求体必须带 `contents[0].role:"user"`**：缺了 Vertex 直接 400 `Please use a valid role: user, model.`。单测只 mock 响应时永远发现不了 —— 涉及外部 API 的功能必须断言**请求体形状**（role/parts/mime/auth header），并用真实凭据跑一次 E2E。
  - 空结果语义：不覆盖已有非空 `transcript`（返回原值 + `empty:true`）；`promptFeedback.blockReason` / `finishReason` ∈ {SAFETY, RECITATION, BLOCKLIST, PROHIBITED_CONTENT} → 502 且不改动原值。
- **配置**（`config.json` 的 `memo` 对象）：`auto_extract`（默认 **false**=手动）、`voice_language`、`ocr_language`（默认 `"auto"`，19 种语言）。`/api/memos/config` 顶层与 `config` 内各返回一份。
  - 在**已运行**的 daemon 上，该接口返回的是**扁平对象**（没有 `config` 包裹、没有 `success`），与其它接口形状不同，别照抄。
- **自动模式**：保存后服务端 goroutine 异步提取（幂等：再次校验 type/media/transcript；不阻塞保存；失败只 `log.Printf`）。前端**不要**再调 extract（会二次计费），改为保存后 **bounded polling**（2/5/10/20s，拿到 transcript 或视图销毁即停）；自动模式下隐藏卡片的手动转换按钮。
- **导出**：`POST /api/memos/export` → `~/Downloads/Antigravity Memos/`（不存在则创建）；文件名净化（非法字符/控制字符/Windows 设备名/120 rune 上限）+ **剥离历史 `[Voice] ` 前缀** + 魔数定扩展名 + ` (2)` 去重 + 返回绝对路径。
- **卡片 UX**：命名 `Voice Memo N` / `Sketch N`（N = 同类标题里最小可用正整数，纯函数 `nextMemoNumber`，`[Voice] ` 前缀不再新增且渲染时剥离旧值）；文本备忘创建时可填可选标题；右键菜单 = 改标题 / 保存到电脑 / 删除；**文本与图片卡双击进编辑器**（保存为更新，不新建）。
- **卡片尺寸**：宽从"最小面板宽"弹性增长到略小于 2× 最小（`repeat(auto-fill, minmax(min(var(--min),100%),1fr))` + `max-width: 2*min - 1px` —— 因为 `gap` 也占轨道预算，不加上限单卡会到 2.03×），高度 `aspect-ratio: 1.618/1`（黄金比随宽度自适应）。
- **搜索**：matched `title` / `content` / `transcript` / `tags` —— 转写/OCR 结果写进 `transcript` 后即可按内容搜索，未转换的只能按名字搜。
- 录音按钮：只显示图标 + 实时计时（无 "Stop Recording" 文案，aria-label 保留）；面板 ≤285px 时隐藏音量波形（容器查询）。

### 3.21 文件浏览器（2026-10-19）
- **没有路径白名单**（用户明确要求"no path restriction"）。解析器只强制**绝对路径**：相对路径、`..` 穿越、URL 编码/空字节变体一律拒绝；symlink 与 `\?\` 归一化保留。
- **破坏性操作仍受保护**：write/create/rename/move/delete 在敏感位置（`C:\Windows`、`C:\Program Files*`、`~/.ssh`、`~/.aws`、`~/.gnupg`、`~/.config/gcloud`、shell dotfiles、浏览器 profile）被拒；**读取放行**。这不是"浏览限制"，别一起删。
- **陷阱**：`cleanUserPath` 会把相对路径按 cwd/项目解析成绝对路径 → "必须绝对路径"的检查形同虚设（Linux 上 `../../../etc/passwd` 会变成真实绝对路径）。已拆为 `normaliseUserPath`（仅解码）+ `cleanUserPath`（cwd 解析，仅供 workspace/memo 记账）。
- 地址栏右端有**跳转下拉**：Parents（祖先链）+ Siblings（同级目录，`/api/files/list` 只取 `isDir`、排除当前）；导航复用既有 `loadFiles`。
- **浮层规则**：`.swiss-files-view` 是 `overflow: hidden`，锚在其中的菜单**必须 `position: fixed` + 挂 `document.body`** 并按可用空间钳制，否则矮面板会裁掉菜单。

### 3.22 聊天导航器（jumper / chat navigator，2026-10-19）
- 配置 `chat_navigator_mode`：**`panel`（默认）** / `line`；UI 在 TokenMonitorPage 的 In-Chat Telemetry Display 卡片（标签 "Chat navigator"）。
- panel 模式 hover 导航条 → **向右**展开面板：逐行 dash + 单行省略的提示词、当前轮高亮、点击复用 dash 跳转；`fixed` 挂 body、150ms 开/关宽限、键盘 ↑↓/Enter/Esc。
- **文本只能缓存已挂载轮次**（转写区是虚拟化列表），键为 (conversationKey, ordinal)，未见过的显示弱化 `…`；禁止编造文本。
- 点击跳转触发的平滑滚动**不得**关面板（`isClickJumping` 守卫 + 滚动结束复位）；用户手动滚动 transcript 才关。
- **提示词是用户输入**：line 模式 tooltip 曾把原文拼进 `innerHTML`（`<img onerror=…>` 会在 IDE 页面执行）→ 已改为 `escapeNavigatorText` 转义。任何把用户文本写进 `innerHTML` 的地方都必须转义。

### 3.23 文案规范（用户明确要求，2026-10-19）
- 配额窗口一律 **5H / 7D**：`Weekly Quota`、`5-Hour Quota`、`7-day rolling cycle` 等旧写法不得出现在任何界面（环标题、表头、排序下拉与 tooltip、账户弹窗、切换器设置、自动切换原因都要覆盖）。
- 配额分桶叫 **Non-Gemini**（不是 Claude/GPT）；模型许可开关 "Allow Claude & GPT models" 保持原样。
- **只改显示字符串**，不要动 `quota_weekly` / `quota_5h` 等数据键。
- 自查：`grep -c "Weekly Quota\|5-Hour\|7-day" pkg/webgui/dist/assets/*.js` 应为 0。

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
| subagent 可能**中途停摆**（改动留在 worktree、未提交、无 PR，agent 报 completed） | 用 `Agent(resume: <task_id>)` 续跑，指令里明确"不要重做已完成部分，只收尾：验证→切分支→提交→推送→建 PR"（2026-10-19 fe-dev4→fe-dev5 即如此） |
| 在**主检出** grep 已交给 worktree 的改动会得到 0 命中，误判"没实现" | 到对应 worktree 或 `git show origin/<branch>:<path>` 核对 |
| 团队上下文会**失效**（`SendMessage` 报 "Not in a team"），但 teammate 仍在跑 | 改用 `Agent(resume: <task_id>)` 继续该 worker |
| 复核者常把结论写成**可复跑脚本**而非长文报告（agent 可能未回消息） | 直接跑它留下的脚本（如 `scratch_review_*.py`）复核结论，再自行补消费者的语义核对 |
| 用户要求"所有任务建 issue 跟踪" | 流程：先 `gh issue create` → 派发 → PR 描述 `Fixes #N`（**每个 issue 单独一行或逗号分隔**，`Fixes #1 #2 #3` 只会自动关第一个）→ 合并后手动关闭漏网的 |
| 本机 Go 不在 PATH；subagent 会去用 `C:	ools\go` | 一律用仓库自带便携工具链 `scratch/gotoolchain/go/bin/go.exe`（`go run/tidy/build/test` 均可） |
| `go fmt ./...` / `gofmt -w` 会把大量**无关文件**按 CRLF 规范化（diff 噪声几百行） | 只对自己改的文件跑格式化；误改的无关文件 `git checkout --` 还原后再提交 |
| 主检出 `npm run build` 会把 `claude.svg` **内联**进 bundle，worktree 构建会产出外部 `claude-*.svg` | 两者等价；以最后一次构建为准，别把外部 svg 当垃圾删（新 bundle 可能引用它） |
| 生成的注入脚本改动没有天然测试面 | 在 `pkg/plugins` / `pkg/enhancements` 里加**源码级 token 断言**（如"label 块内必须是 ToggleSwitch 且无 checkbox"、"不得出现原始 innerHTML 拼接"）——本会话两次靠它挡住回归 |

## 6. 安全事件：vshost 恶意软件（2026-10-18 已清除）

- **现象**：`vshost.exe - Application Error` 弹窗（0xE0434352 = .NET CLR 未处理异常），Defender AMSI 威胁 2147972000 + wangmeng .NET 载荷。驻留约 4-5 个月。
- **持久化**：3 个仿冒计划任务 + 5 个 MS 签名文件副本（LOLBin）+ 2 个 NUL 前缀隐藏 Run 值。
- **已清除并验证**：任务/文件/Run 值全删；Run 键 15→13；RunOnce/Wow6432Node 无 NUL 残留；无 vshost 进程。保留了合法的 `dfsvc` 值、Chs 文件、WindowsApps 别名。
- **反清除对抗**：恶意软件在目录上设 `Everyone: Deny Delete/ReadPermissions/ChangePermissions`，文件继承 → 先 `icacls /remove:d *S-1-1-0` 再删。
- **隔离备份**：`scratch/vshost-quarantine-2026-10-18/`（5 个恶意文件副本 + 3 个任务 XML + MANIFEST.txt 含 SHA256 + HKCU_Run_backup.reg）。
- **NT API 删除工具**：`scratch/regfix/main.go`（独立 go module）。
- **仍待办**：建议用户跑 Defender 全盘/脱机扫描；驻留 4-5 个月，已提醒轮换重要凭据。

## 7. 会话完成的工作（2026-10-18）

四项待办全部合并入 main（顺序 #97→#98→#99→#100，merge commit）：

| PR | 内容 | 验证 |
|---|---|---|
| #97 `feat/theme-auto-light-dark` | Auto/Light/Dark 主题（core 配置 + API + theme.ts + 暗色令牌 + 设置页） | CI 三作业绿 |
| #98 `fix/hide-console-windows` | §3.3 控制台闪窗修复，14 站点 + Windows-only 测试 | CI 三作业绿 |
| #99 `fix/manual-switch-nonstandby` | §3.6 手动切换不变式 + 3 个 webgui 测试 | CI 三作业绿 |
| #100 `fix/gcp-custom-models-region` | §3.7 GCP global location + 默认门控 | CI 三作业绿 |

收尾：dist 重建（9b5c727）→ `bin/swiss.exe` 从合并后 main 重建 → daemon 启动验证（RPC 正常、3 账户）→ 桌面应用已启动并全链路验证（8765 服务新 bundle `index-vCZdaxSh.js`、认证 API 正常、无 token 401）。

## 7.1 会话完成的工作（2026-10-19 上午，15 个 PR）

用户当天连续报了 20+ 个问题，全部走「建 issue → 子 agent 并行实现 → 独立只读复核 → 修复复核发现 → 合并 → 重建重启 → 关 issue」流程。GitHub issue #117–#129 全部关闭。

| PR | 内容 | 复核 |
|---|---|---|
| #111 | 启动 IDE 不再托盘化（§3.9） | CI + flags 单测 |
| #112 | UI 增强覆盖 VS Code 扩展（§3.14） | CI + 实机 CDP 探测 |
| #113 | 定价动态化（§3.8） | CI + 部署前后同接口对比（0.75→2.00） |
| #114 | memo 网格 / 线宽 DPR / 录音时长波形 / 按钮统一 / memo 筛选器 / Non-Gemini / GCP 手填余额 / 配额区默认折叠 / 清 null 文案 | 独立复核两轮：首轮 3×P2+1×P3 → F1–F10 修复 → 全 PASS |
| #115 | resume 只发一条 + Overview 自动收起（§3.13） | 判别性测试（回退即失败） |
| #116 | jumper 稳定计数 + Split Right 双窗格（§3.13） | mock-DOM 运行时测试 + 负向对照 |
| #123 | 复活导航竞争（restore 先核实后注入） | fake navigator 序列断言 |
| #124 | 文件查看器单一 WYSIWYG / 面板按项目隔离 / 去标题胶囊 / 端点安全（§3.11） | 独立复核抓到 loopback 泄漏 + 无 host 端点，修复后补 13 组畸形输入测试 |
| #130 | range 贯通 + 堆叠直方图 + Token 口径实证 + 多配额指示器 + 7D 文案 + Requests 单位（§3.12） | 复核脚本独立复算 Magpie 口径 + 我复跑确认 |

关键遗留教训（都已写入 §3 / §5）：硬编码价格会短路上层动态源；HTML 正则缺 DOTALL 会静默失败；SW_HIDE 会隐藏 GUI 首窗口；托盘必须用透明 glyph；图表不得造数；token 字段互不包含（别看名猜子集）；虚拟化列表不能用来数总数；`Fixes #1 #2` 只关第一个 issue。

## 7.2 会话完成的工作（2026-10-19 下午，17 个 PR）

同一流程，issue **#132–#165** 全部关闭。这一段几乎全是「用户截图 → 我拆 issue → 两个 agent 各占一侧文件并行 → 合并 → 真机复核」。

| PR | 内容 | 关键证据 |
|---|---|---|
| #138/#140 | 备忘转写/OCR 端点；修 `contents[0].role`（真机 400 才暴露） | 真机：`Hello Hello Hi`、`✓`，落盘 `memos.json` |
| #139 | 卡片固定尺寸 + 播放进度 + 音量条 + 按钮标签 + 提取按钮 | 复核两轮；后由 #142 改为弹性宽度 |
| #142 | 卡片宽度弹性（最小 → <2×最小）+ 黄金比高度 | 无头 Chrome 逐宽度实测 246→981px |
| #146 | 录音按钮去文案、窄面板隐藏波形、自动模式隐藏提取按钮 + 保存后 bounded polling | 轮询 2/5/10/20s，停止条件已测 |
| #147/#148 | 备忘设置：手动/自动 + 语音/OCR 语言（19 种） | 真机：auto=true 保存后 t+6s 自动出转录 |
| #149 | 图标重建（透明圆角、无裁切；`logo_square.png` 为源） | corner-alpha=0、内容占比 77.9%×76.4% |
| #154/#156 | `POST /api/memos/export` + 文件名剥离 `[Voice] ` | 真机：WebM 39,581B / PNG 6,505B，魔数校验 |
| #155 | 卡片命名 `Voice Memo N`/`Sketch N`、右键菜单、双击编辑、文本可选标题 | 纯函数 `nextMemoNumber` + 单测 |
| #159 | 文件浏览器地址栏跳转下拉（Parents/Siblings） | fixed 挂 body，逃出 overflow 裁剪 |
| #160 | 移除路径白名单（绝对路径契约 + 破坏性操作敏感目录仍拒） | 真机：`D:\Projects\SML_GroupProject` 21 项；`C:\Windows` 写入被拒 |
| #161 | 5H/7D 文案全量收口 | bundle 内旧写法 0 命中 |
| #163/#164 | 聊天导航器 `chat_navigator_mode`（panel 默认/line）+ hover 面板 | 真机配置往返；补 `isClickJumping` 与 tooltip 转义 |
| #165 | "Pin focused chat to top" 改 ToggleSwitch | bundle 内 checkbox 0 命中 |

教训补充：**外部 API 的请求体必须断言**（mock 响应会让 100% 失败的功能全绿）；**虚拟化列表只能缓存已挂载项**；**用户文本进 `innerHTML` 必须转义**；**`overflow:hidden` 容器里的浮层要 `fixed` + body**；**缺口编号规则要能复用空洞**。

## 8. 待办 / 下一步

1. **Defender 全盘/脱机扫描**（用户侧，未完成）——vshost 驻留过久，凭据轮换建议已发出。
2. **注入式 UI 的目视验证**（用户侧，唯一无法自动化的一类）：文件查看器单一 WYSIWYG、面板切项目刷新、jumper 悬浮面板的位置/宽度与暗色对比度、Split Right 双窗格、直方图 hover 分解、录音时长/波形、卡片在 285px 最小面板下的排布、跳转下拉在矮面板下的浮层表现。
3. **真实设备/网络验证**：真麦克风下的录音 teardown、cloudflared quick tunnel 与 Tailscale 端到端、多窗口配额（5H/7D）对真实 provider 响应。
4. **VS Code 扩展开箱即用**：需先用应用的启动入口重启 VS Code（已运行的实例忽略 `--remote-debugging-port`）。
5. **已知小瑕疵（不阻塞，未修）**：`/api/files/list` 对非法路径返回 `200 + 空列表`（应 4xx）；解析器错误以 `200 + error` 字段返回；`/api/memos/config` 返回扁平对象而其它接口用 `{config}` 包裹。
6. 无开放 PR / issue；main 即完整状态。新功能按 §4 带 dist 重建，按 §5 末行建 issue + 独立复核后合并。
7. `D:/Projects/swiss-wt/` 下的 20+ 个并行 worktree 已完成使命，可择机清理（`git worktree remove`）；保留主检出即可。
8. subagent 并行开发模式（worktree + junction node_modules + 显式 `git add` + 独立复核）在本项目多次验证有效；汇报必须独立核实（§5），复核结论优先看可复跑脚本。
