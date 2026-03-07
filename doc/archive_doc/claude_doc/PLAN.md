# chrome-tab-tui — Go Rewrite Plan

## Background

TS/Bun 版本 (chrome-session-tui) 已完成 M0-M6 全部功能，包括 CLI、daemon、TUI (Ink v5)、实时事件、Collections。但分发依赖 bun 运行时，不适合 brew。

Go 重写目标：**单二进制，`brew install` 即用，零运行时依赖。**

## Architecture

```
Chrome Extension (JS, 不变)
    |
    | Chrome Native Messaging (stdin/stdout, 4-byte length prefix)
    |
ctm nm-shim (Go, 同一个二进制)
    |
    | NDJSON over Unix socket
    |
ctm daemon (Go, 长驻进程)
    |
    +--- ctm tui (Bubble Tea, 持久连接)
    +--- ctm tabs list (CLI, 一次性连接)
    +--- ctm sessions save ...
```

所有组件编译为同一个二进制 `ctm`，通过子命令区分角色。

## Binary & Config

- **Binary name**: `ctm` (chrome-tab-manager)
- **Config dir**: `~/.config/ctm/`
- **Socket**: `~/.config/ctm/daemon.sock`
- **Sessions**: `~/.config/ctm/sessions/<name>.json`
- **Collections**: `~/.config/ctm/collections/<name>.json`
- **NM manifest**: `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.ctm.native_host.json` (macOS)

与 TS 版本 (`chrome-session-tui`) 完全独立，不共享数据目录。

### 安全

- `~/.config/ctm/` 目录权限 `0700`，`daemon.sock` 权限 `0600`
- daemon 接受连接时可选校验 peer UID（macOS: `LOCAL_PEERCRED`），拒绝非当前用户连接

### 路径策略（XDG 合规）

运行时状态与持久数据分离：

| 类别 | macOS | Linux |
|------|-------|-------|
| 配置 + 持久数据 | `~/Library/Application Support/ctm/` 或 `~/.config/ctm/` | `$XDG_CONFIG_HOME/ctm/` |
| Socket + Lock | `~/.config/ctm/` (macOS 无 XDG_RUNTIME_DIR) | `$XDG_RUNTIME_DIR/ctm/` |

Phase 1 先只支持 macOS（统一 `~/.config/ctm/`），Linux 路径在需要时补充。

### Extension 分发策略

brew 只分发 `ctm` 二进制，extension 有三条路：

1. **`ctm install --extension`**：将 embed 在二进制中的 extension 解压到 `~/.config/ctm/extension/`，并输出 `--load-extension` 路径提示
2. **Chrome Web Store**（可选，v2 考虑）
3. **手动 Load unpacked**（开发模式）

Go 1.16+ `embed` 包可将 `extension/` 目录嵌入二进制。

## Tech Stack

| 用途 | 库 |
|------|-----|
| CLI 框架 | `github.com/spf13/cobra` |
| TUI 框架 | `github.com/charmbracelet/bubbletea` |
| TUI 样式 | `github.com/charmbracelet/lipgloss` |
| TUI 组件 | `github.com/charmbracelet/bubbles` |
| 剪贴板 | `github.com/atotto/clipboard` |
| 构建分发 | GoReleaser + homebrew-tap |

不引入其他外部依赖。JSON 编解码用标准库。

## Project Structure

```
chrome-tab-tui/
  go.mod
  main.go                     # cobra root command
  cmd/
    root.go                   # root cmd + global flags
    daemon.go                 # ctm daemon [--foreground]
    tui.go                    # ctm tui
    tabs.go                   # ctm tabs {list|close|activate|open}
    groups.go                 # ctm groups {list|create|update|delete}
    sessions.go               # ctm sessions {list|save|restore|delete|get}
    collections.go            # ctm collections {list|create|delete|add|remove|restore}
    targets.go                # ctm targets {list|default|label}
    nmshim.go                 # ctm nm-shim (Chrome 调用, 用户不直接用)
    install.go                # ctm install [--check]
    migrate.go                # ctm migrate [--from <path>]
    version.go                # ctm version
  internal/
    config/
      paths.go               # ConfigDir, SocketPath, SessionsDir, CollectionsDir
    protocol/
      messages.go             # Request, Response, Error, Event, Hello 结构体
      ndjson.go               # Reader (bufio.Scanner, 1MB buffer) + Writer (json.Marshal + \n)
      ids.go                  # MakeID()
    client/
      client.go               # Client: Connect, Request, Subscribe, Reconnect
    daemon/
      server.go               # Accept loop, connection handler, routing
      targets.go              # Target registry (Hub 内部状态，无锁)
      sessions.go             # Session file CRUD
      collections.go          # Collection file CRUD
      subscribe.go            # Subscriber set, pattern matching, fanout
      state.go                # DaemonState (defaultTarget, etc.)
    nmshim/
      shim.go                 # 4-byte length prefix codec, stdin/stdout <-> socket bridge
    bookmarks/                # Stage 3: bookmark tree + mirror + overlay
      mirror.go
      overlay.go
      search.go
      export.go
    search/                   # Stage 5: cross-resource search engine
      engine.go
      saved.go
    sync/                     # Stage 4: iCloud sync engine
      engine.go
      state.go
      device.go
    workspace/                # Stage 6: workspace model
      workspace.go
    tui/
      app.go                  # Root Model, Init, Update, View
      keymap.go               # key.Binding definitions, help.KeyMap interface
      state.go                # AppState struct, Msg types
      tabs.go                 # TabBrowser Model
      groups.go               # GroupBrowser Model
      sessions.go             # SessionBrowser Model
      collections.go          # CollectionBrowser Model
      targets.go              # TargetPicker Model
      header.go               # Header component
      statusbar.go            # StatusBar component (toast/error/confirmHint)
      help.go                 # HelpOverlay
      styles.go               # Lip Gloss style definitions
  extension/                  # Chrome Extension (JS, 从 TS 版复制)
    manifest.json
    service-worker.js
  Makefile
  .goreleaser.yml
  PLAN.md
```

## Protocol

NDJSON over Unix socket，与 TS 版格式类似但**不保证向后兼容**（全新项目）。

```json
{"id":"msg_1","protocol_version":1,"type":"request","action":"tabs.list","target":{"targetId":"t1"},"payload":{}}
{"id":"msg_1","protocol_version":1,"type":"response","action":"tabs.list","payload":{"tabs":[...]}}
{"id":"evt_1","protocol_version":1,"type":"event","action":"tabs.created","payload":{"tab":{...}}}
```

消息类型: `hello`, `request`, `response`, `error`, `event`

### Daemon Actions（完整列表）

| 分类 | Action | 说明 |
|------|--------|------|
| daemon | `daemon.stop` | 停止 daemon |
| targets | `targets.list` | 列出在线 targets |
| targets | `targets.default` | 设置默认 target |
| targets | `targets.clearDefault` | 清除默认 target |
| targets | `targets.label` | 给 target 设置标签 |
| tabs | `tabs.list` / `tabs.open` / `tabs.close` / `tabs.activate` / `tabs.update` | 转发到 extension |
| groups | `groups.list` / `groups.create` / `groups.update` / `groups.delete` | 转发到 extension |
| sessions | `sessions.list` / `sessions.get` / `sessions.save` / `sessions.restore` / `sessions.delete` | daemon 本地 CRUD（save/restore 需 extension 配合） |
| collections | `collections.list` / `collections.get` / `collections.create` / `collections.delete` | daemon 本地 CRUD |
| collections | `collections.addItems` / `collections.removeItems` / `collections.addFromTabs` / `collections.restore` | daemon 本地 + extension 配合 |
| events | `subscribe` | 注册事件订阅 |

### 日志策略

- daemon foreground 模式：日志输出到 stderr（`log.SetOutput(os.Stderr)`）
- launchd 模式：plist 配置 `StandardOutPath` / `StandardErrorPath` → `~/.config/ctm/daemon.log`
- 日志格式：`[daemon HH:MM:SS] message`（与 TS 版一致）
- 日志级别：默认 info，`--verbose` flag 开启 debug

### 版本协商

`hello` 握手阶段进行版本校验：

```json
{"type":"hello","payload":{"protocol_version":1,"min_supported":1,"capabilities":["tabs","groups","sessions","collections","events"]}}
```

- daemon 收到 hello 后比较 `protocol_version` 与自身支持范围
- 不兼容时返回 `error`（code: `PROTOCOL_MISMATCH`），附带可读的升级提示
- CLI/TUI/extension 各自独立发版，版本协商避免 brew 升级后二进制与 extension 错配

## Daemon Lifecycle

Go daemon 需要比 TS 版更健壮的生命周期管理：

### 并发模型

TS daemon 依赖 Node.js 单线程事件循环天然串行化全局状态。Go 版必须显式处理并发：

**方案：单 owner goroutine（actor 模式）**

```go
// daemon 核心状态由一个 goroutine 独占
type Hub struct {
    registerCh   chan *Connection
    unregisterCh chan *Connection
    requestCh    chan *Request
    eventCh      chan *Event
    // ... targets, pendingRequests, subscribers 全在这个 goroutine 内操作
}

func (h *Hub) Run(ctx context.Context) {
    for {
        select {
        case conn := <-h.registerCh: ...
        case conn := <-h.unregisterCh: ...
        case req := <-h.requestCh: ...
        case evt := <-h.eventCh: ...
        case <-ctx.Done(): return
        }
    }
}
```

- 每条连接一个读 goroutine，解析消息后发到 Hub channel
- Hub 内部零锁，所有可变状态在同一个 goroutine 中访问
- 文件 I/O（sessions/collections）走 `tmp + fsync + rename` 原子写入

### 进程单例

启动顺序：
1. 打开 lockfile（`~/.config/ctm/daemon.lock`），尝试 `flock` 排他锁
2. 获取锁成功 → 检查并清理残留 socket → 启动 listen
3. 获取锁失败 → 另一个 daemon 在运行，退出并提示
4. 进程崩溃时 flock 自动释放，比 PID file 可靠

### 优雅关闭
```go
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()
// 1. 停止 accept 新连接
// 2. 等待 in-flight 请求完成 (带 5s timeout)
// 3. 关闭所有 subscriber 连接
// 4. 删除 socket 文件
// 5. 释放 flock
```

### Daemon 生命周期管理

**macOS 主路径：launchd**

`ctm install` 同时安装：
- NM manifest → `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/`
- LaunchAgent plist → `~/Library/LaunchAgents/com.ctm.daemon.plist`

```xml
<plist version="1.0">
<dict>
  <key>Label</key><string>com.ctm.daemon</string>
  <key>ProgramArguments</key><array>
    <string>{{CTM_BIN_PATH}}</string>  <!-- ctm install 时动态替换为实际路径 -->
    <string>daemon</string>
    <string>--foreground</string>
  </array>
  <key>KeepAlive</key><true/>
  <key>RunAtLoad</key><true/>
  <key>StandardOutPath</key><string>{{CTM_CONFIG_DIR}}/daemon.log</string>
  <key>StandardErrorPath</key><string>{{CTM_CONFIG_DIR}}/daemon.log</string>
</dict>
</plist>
```

**Fallback：自动启动**

CLI/TUI 连接失败时的启动流程（防竞态）：
1. 尝试连接 socket → 失败
2. 尝试获取 lockfile → 成功则 fork daemon；失败则说明其他进程正在启动
3. 等待 socket 就绪（poll，最多 3s）
4. 重试连接

**手动启动**：`ctm daemon [--foreground]`

## TUI: TS → Go 映射

| TS (Ink/React) | Go (Bubble Tea) |
|---|---|
| `useReducer(reducer, state)` | `Model` struct + `Update(msg) (Model, Cmd)` |
| `Action` union type | `tea.Msg` interface implementations |
| `dispatch(action)` | 返回 `tea.Cmd` |
| `useInput(fn, {isActive})` | `Update` 中 `switch model.mode` + `key.Matches` |
| `useEffect` | `Init() tea.Cmd` + 返回 `tea.Cmd` chain |
| `PersistentClient` | `client.Client` goroutine + `chan tea.Msg` |
| 150ms event batch | `tea.Tick(150ms)` + buffer slice |
| React 组件 | 嵌套 Model (`tabsModel`, `sessionsModel` 等) |
| `ink-testing-library` | `teatest` package |
| `useState` (local) | Model 字段 |
| 多个 `useInput` 同时 active | **单个 `Update`，消除键冲突** |

### Bubble Tea 优势
- 单入口 `Update` 消除了 Ink 多 `useInput` handler 的键冲突问题
- 纯函数式更新，测试友好
- `teatest` 提供内置的 TUI 测试支持
- 大量 tab (1000+) 场景可用 list 组件的虚拟化

## Phased Implementation

### 与 BUILD_ORDER 的关系

codex_doc/BUILD_ORDER.md 定义了 8 个**领域 Stage**（Runtime → Library → Bookmarks → Sync → Search → Workspace → Interaction → Power），关注"先把哪个产品骨架立起来"。

本文的 Phase 是**实现顺序**，关注"先写哪段代码"。两者是互补关系：

- Phase 1-3 对应 Stage 1 + 7（Runtime foundation + 基础 Interaction）
- Phase 4 对应 Stage 7（CLI Interaction）
- Phase 5 对应 Stage 2 + 7（Library + TUI Interaction）
- Phase 6 对应 Stage 7（Distribution）
- Phase 7 对应 Stage 3（Bookmarks）
- Phase 8a 对应 Stage 4（Sync）
- Phase 8b 对应 Stage 5 + 6（Search + Workspace）

所有 Phase 的设计必须兼容 BUILD_ORDER 定义的领域骨架。

### Phase 1: 骨架 + 协议 + Client

```
go mod init, cobra setup
internal/config/paths.go
internal/protocol/messages.go, ndjson.go
internal/client/client.go (connect, request, reconnect)
```

验证: 编译通过, 单元测试协议编解码

### Phase 2: Daemon

```
internal/daemon/ 全部
cmd/daemon.go
```

核心: Unix socket server, target 管理, 请求路由 (extension → daemon → response)
存储: sessions + collections 文件 CRUD
事件: subscribe + fanout

验证: `ctm daemon` 启动, 用 TS extension 连接成功

### Phase 3: NM Shim + Install

```
internal/nmshim/shim.go
cmd/nmshim.go
cmd/install.go
```

NM shim: Chrome stdin/stdout (4-byte LE prefix) ↔ daemon Unix socket (NDJSON)
Install:
- 生成 NM host manifest（路径动态获取 `os.Executable()` 的 realpath，不指向 versioned Cellar 路径）
- 安装 LaunchAgent plist
- 解压 embed 的 extension 到 `~/.config/ctm/extension/`
- `ctm install --check` 验证安装状态

验证: Chrome Extension 通过 Go shim 连接 Go daemon

### Phase 4: CLI

```
cmd/tabs.go, groups.go, sessions.go, collections.go, targets.go
```

复用 `internal/client/`。输出格式: table (默认) + `--json` flag。

验证: `ctm tabs list`, `ctm sessions save work`, `ctm collections create fav`

### Phase 5: TUI

```
internal/tui/ 全部
cmd/tui.go
```

Bubble Tea 全功能移植:
- 5 个视图: Tabs, Groups, Sessions, Collections, Targets
- 键绑定: vim 导航, y-chord, z-filter, D-D confirm
- 三通道反馈: toast, error, confirmHint
- 实时事件订阅 + 批量更新
- 帮助覆盖层, 命令面板

验证: `ctm tui` 功能与 TS 版一致

### Phase 6: 分发

```
.goreleaser.yml
homebrew-tap repo
Makefile
```

- GoReleaser: darwin-arm64, darwin-amd64（v1 仅 macOS，Linux 后续按需补充）
- Homebrew tap: `brew tap user/tap && brew install ctm`
- `ctm install` post-install 自动注册 NM manifest
- `ctm version` 显示版本号 (由 goreleaser ldflags 注入)

验证: `brew install user/tap/ctm && ctm install && ctm tui`

## Extension Changes

Extension JS 代码基本不变，仅修改：

1. **NM host name**: `com.csm.native_host` → `com.ctm.native_host`
2. **Extension ID**: 新的 extension (或复用, 取决于是否上 Chrome Web Store)
3. **Protocol**: 如有协议调整，同步修改

Extension 放在 `extension/` 目录，通过 Go `embed` 嵌入二进制。`ctm install` 时解压到 `~/.config/ctm/extension/` 供 `--load-extension` 加载，或未来通过 Chrome Web Store 安装。

## Testing Strategy

| 层 | 方法 |
|----|------|
| protocol | 单元测试: encode/decode round-trip |
| daemon | 集成测试: in-process server + client |
| client | 单元测试: mock net.Conn |
| tui | `teatest`: 发送按键序列, 断言输出 |
| nmshim | 单元测试: bytes 编解码 + fuzz（4-byte length 边界） |
| E2E | smoke script: daemon + extension + TUI 联调，含断连/重连场景 |

`go test -race ./...` 覆盖所有包（`-race` 强制开启，CI 中必跑）。

Fuzz 测试重点：NM 4-byte length parser、NDJSON parser（畸形输入）。

## Data Migration

提供 `ctm migrate` 子命令，从 TS 版目录一次性导入：

```
ctm migrate [--from ~/.config/chrome-session-tui]
```

- 复制 `sessions/*.json` → `~/.config/ctm/sessions/`
- 复制 `collections/*.json` → `~/.config/ctm/collections/`
- JSON 格式兼容（TS 版格式直接可用）
- 导入后提示用户验证，不自动删除源文件

## Decisions (from Codex Review)

以下问题在审查中已明确：

1. **Binary name**: `ctm` — 已确认
2. **Daemon lifecycle**: macOS 以 launchd 为主路径，CLI auto-start 为 fallback
3. **Extension 分发**: embed 在二进制中 + `ctm install` 解压；Chrome Web Store 为 v2 目标
4. **多浏览器**: 显式 defer 到 v2，v1 只支持 Chrome Stable；不在 v1 协议里留半开放口
5. **Linux 支持**: Phase 6 先收窄为 darwin-only，Linux 路径（XDG_RUNTIME_DIR 等）在需要时补充

### Phase 7: Bookmarks + Knowledge Layer

```
extension: 添加 chrome.bookmarks 权限 + bookmark 事件监听
internal/bookmarks/ (new): bookmark tree 模型 + 本地镜像存储
internal/tui/bookmarks.go (new): Bookmarks tree view
cmd/bookmarks.go (new): ctm bookmarks {list|search|tree|get|tag|export}
```

实现要点：
- Extension: `chrome.bookmarks.getTree()` handler + `onCreated/onChanged/onRemoved` 事件监听
- Daemon: BookmarkMirror 本地 JSON 存储 + overlay CRUD
- TUI: tree view（展开/折叠/缩进）

领域设计见 `codex_doc/DOMAIN_MODEL.md`（BookmarkSource / BookmarkMirror / BookmarkOverlay）。

验证: `ctm bookmarks tree`, `ctm bookmarks search <keyword>`, TUI Bookmarks view

### Phase 8: Sync + Workspace

```
internal/sync/ (new): iCloud 同步引擎
internal/workspace/ (new): workspace 模型
cmd/workspace.go (new): ctm workspace {list|create|switch|delete}
cmd/sync.go (new): ctm sync {status|repair}
```

#### 8a: iCloud Sync

实现要点：
- 同步目录：`~/Library/Mobile Documents/com~ctm/`
- 同步策略：local-first + file-level last-write-wins + 冲突文件 `{id}.conflict.json`
- fsnotify 监听云端变更 → 增量同步
- TUI Sync view：显示同步状态、冲突、最近同步时间

Source of Truth 划分见 `codex_doc/PRODUCT_ARCHITECTURE.md` §3。

#### 8b: Search Layer（对应 BUILD_ORDER Stage 5）

```
internal/search/ (new): 跨资源搜索引擎 + SavedSearch 持久化
cmd/search.go (new): ctm search <query> [--scope tabs,sessions,bookmarks] [--tag work]
```

核心能力：
- 跨资源统一搜索（tabs + sessions + collections + bookmarks + workspaces）
- 按 tag/host/domain 过滤
- SavedSearch 持久化（可重复执行的查询）
- 搜索结果直接可行动（activate/restore/open）
- TUI Search view：统一搜索入口

验证: `ctm search github` 返回跨资源结果 + TUI Search view 完整工作流

#### 8c: Workspace

实现要点：
- 存储：`~/.config/ctm/workspaces/{id}.json`
- CLI: `ctm workspace {list|create|switch|delete}`
- `switch` 实现：先 `tabs.close` 全部当前 tabs，再 `sessions.restore` 关联 session
- TUI Workspaces view：列表 + 一键切换

Workspace 领域定义见 `codex_doc/DOMAIN_MODEL.md` §4.5。

验证: `ctm workspace switch` 全流程 + iCloud 同步到另一台 Mac

## Extension Changes

Extension JS 代码修改：

1. **NM host name**: `com.csm.native_host` → `com.ctm.native_host`
2. **Extension ID**: 新的 extension (或复用, 取决于是否上 Chrome Web Store)
3. **Protocol**: 如有协议调整，同步修改
4. **Phase 7 新增**：
   - `manifest.json` 添加 `"bookmarks"` 权限
   - `chrome.bookmarks.getTree()` handler
   - `chrome.bookmarks.onCreated/onChanged/onRemoved` 事件监听 → sendEvent

Extension 放在 `extension/` 目录，通过 Go `embed` 嵌入二进制。`ctm install` 时解压到 `~/.config/ctm/extension/` 供 `--load-extension` 加载，或未来通过 Chrome Web Store 安装。

## Open Questions (Remaining)

1. **Chrome Web Store**：何时上架？上架需要 extension ID 固定
2. **多浏览器适配层**：Chrome Beta / Brave / Edge 的 manifest 路径 + extension ID 映射
3. **Bookmarks tree view**：TUI 中用 tree widget 还是 indent list？Bubble Tea 生态的 tree 组件成熟度待调研
4. **iCloud 冲突**：file-level last-write-wins 是否够用？是否需要 field-level merge？
5. **Workspace startup**：切换 workspace 时是否自动关闭当前所有 tabs？还是保留并标记为"非当前 workspace"？
