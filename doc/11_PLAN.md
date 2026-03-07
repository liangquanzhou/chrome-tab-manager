# CTM — Go Rewrite Plan

## 1. Background

TS/Bun 版 (chrome-session-tui) 已完成 M0-M6 全部功能。Go 重写目标：**单二进制，`brew install` 即用，零运行时依赖。**

## 2. Architecture

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

所有组件编译为同一个二进制 `ctm`，cobra 子命令区分角色。

## 3. Binary & Config

- **Binary**: `ctm` (chrome-tab-manager)
- **Config**: `~/.config/ctm/`
- **Socket**: `~/.config/ctm/daemon.sock`
- **Sessions**: `~/.config/ctm/sessions/<name>.json`
- **Collections**: `~/.config/ctm/collections/<name>.json`
- **NM manifest**: `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.ctm.native_host.json`

安全：目录 `0700`，socket `0600`，可选 peer UID 校验。

Phase 1 先只支持 macOS（统一 `~/.config/ctm/`），Linux 路径在需要时补充。

## 4. Tech Stack

| 用途 | 库 |
|------|-----|
| CLI | `cobra` |
| TUI | `bubbletea` + `lipgloss` + `bubbles` |
| 剪贴板 | `atotto/clipboard` |
| 构建分发 | GoReleaser + homebrew-tap |

JSON 编解码用标准库。不引入其他外部依赖。

## 5. Project Structure

```
chrome-tab-tui/
  go.mod, main.go
  cmd/          root, daemon, tui, tabs, groups, sessions, collections,
                targets, nmshim, install, migrate, version
  internal/
    config/     paths.go
    protocol/   messages.go, ndjson.go, ids.go
    client/     client.go
    daemon/     server.go, targets.go, sessions.go, collections.go, subscribe.go, state.go
    nmshim/     shim.go
    bookmarks/  mirror.go, overlay.go, search.go, export.go     (Stage 3)
    search/     engine.go, saved.go                              (Stage 5)
    sync/       engine.go, state.go, device.go                   (Stage 4)
    workspace/  workspace.go                                     (Stage 6)
    tui/        app.go, keymap.go, state.go, tabs.go, groups.go,
                sessions.go, collections.go, targets.go,
                header.go, statusbar.go, help.go, styles.go
  extension/    manifest.json, service-worker.js
  Makefile, .goreleaser.yml
```

## 6. Protocol

NDJSON over Unix socket。消息类型: hello, request, response, error, event。
精确 request/response 定义见 `12_CONTRACTS.md`。

Hello 握手协商 protocol_version；不兼容返回 PROTOCOL_MISMATCH + 升级提示。

## 7. Daemon Lifecycle

详细设计见 `09_DESIGN.md` §2.4。

- **并发**：Actor 模式 (Hub)，单 goroutine 独占状态，零锁
- **单例**：flock 排他锁
- **优雅关闭**：signal.NotifyContext → 停 accept → 等 in-flight → 清理
- **macOS 主路径**：launchd (`ctm install` 安装 LaunchAgent plist)
- **Fallback**：CLI/TUI 连接失败时自动启动 daemon（防竞态 lockfile check）

## 8. Phased Implementation

### Phase ↔ BUILD_ORDER Stage Mapping

| Phase | Stage | 内容 |
|-------|-------|------|
| 1 | 1+7 | 骨架 + 协议 + Client |
| 2 | 1+7 | Daemon (Hub + target + sessions/collections + subscribe) |
| 3 | 1+7 | NM Shim + Install |
| 4 | 7 | CLI (table + --json) |
| 5 | 2+7 | TUI (5 views + vim + chords + feedback + events) |
| 6 | 7 | 分发 (GoReleaser + Homebrew) |
| 7 | 3 | Bookmarks + Knowledge Layer |
| 8a | 4 | iCloud Sync |
| 8b | 5+6 | Search + Workspace |

### Phase 1: 骨架 + 协议 + Client
`go mod init`, cobra, config/paths, protocol/messages+ndjson, client/client。
验证：编译通过，协议编解码单元测试。

### Phase 2: Daemon
daemon/ 全部 + cmd/daemon。Hub (actor), target 管理, sessions/collections CRUD, subscribe+fanout。
验证：`ctm daemon` 启动，extension 连接成功。

### Phase 3: NM Shim + Install
nmshim, cmd/nmshim, cmd/install。NM 桥接 + manifest 生成 + LaunchAgent + embed extension。
验证：Chrome Extension 通过 Go shim 连接 Go daemon。

### Phase 4: CLI
cmd/tabs, groups, sessions, collections, targets。复用 client/。Table + --json。
验证：`ctm tabs list`, `ctm sessions save work`。

### Phase 5: TUI
internal/tui/ 全部 + cmd/tui。5 views, vim 导航, chords, 三通道反馈, 实时事件。
验证：`ctm tui` 功能与 TS 版一致。

### Phase 6: 分发
.goreleaser.yml, homebrew-tap, Makefile。darwin-arm64 + darwin-amd64。
验证：`brew install user/tap/ctm && ctm install && ctm tui`。

### Phase 7: Bookmarks
Extension 添加 bookmarks 权限 + 事件监听。internal/bookmarks/. TUI Bookmarks view. CLI bookmarks commands.
验证：`ctm bookmarks tree`, TUI 树形浏览 + overlay tag。

### Phase 8: Sync + Search + Workspace
8a: iCloud sync engine + Sync view。
8b: Cross-resource search + SavedSearch + Workspace CRUD + switch。
验证：跨设备同步 + `ctm search github` + `ctm workspace switch`。

## 9. Extension Changes

Extension JS 基本不变，修改：
1. NM host name: `com.csm.native_host` → `com.ctm.native_host`
2. Extension ID: 新 extension 或复用
3. Phase 7: `manifest.json` 添加 `"bookmarks"` 权限 + bookmark 事件监听

通过 Go embed 嵌入二进制，`ctm install` 解压到 `~/.config/ctm/extension/`。

## 10. Data Migration

```
ctm migrate [--from ~/.config/chrome-session-tui]
```

复制 sessions/collections JSON。格式兼容。不自动删除源文件。

## 11. TUI: TS → Go Mapping

| TS (Ink/React) | Go (Bubble Tea) |
|---|---|
| `useReducer` | `Model` + `Update(msg)` |
| Action union | `tea.Msg` interface |
| `dispatch` | 返回 `tea.Cmd` |
| `useInput` | `Update` 中 `switch model.mode` + `key.Matches` |
| `PersistentClient` | goroutine + `chan tea.Msg` |
| 150ms event batch | `tea.Tick(150ms)` + buffer |
| React 组件 | 嵌套 Model |
| `ink-testing-library` | `teatest` |

Bubble Tea 优势：单入口 Update 消除 Ink 多 useInput handler 键冲突。

## 12. Confirmed Decisions

1. Binary name: `ctm`
2. Daemon lifecycle: macOS launchd 主路径，CLI auto-start fallback
3. Extension 分发: embed + `ctm install` 解压；Chrome Web Store 按需上架
4. 多浏览器: 完整建模，Phase 7+ 实现（当前只 Chrome Stable）
5. Linux: 完整建模，Phase 6 先 darwin-only，后续补充

## 13. Open Questions

以下问题均**不阻塞 Phase 1-4 开工**，可在对应 Phase 到来时决定：

1. Chrome Web Store 上架时机（Phase 6+ 决定）
2. 多浏览器适配层 Beta / Brave / Edge（Phase 7+ 决定）
3. TUI tree widget 成熟度（Phase 5 评估）
4. iCloud 冲突 file-level vs field-level merge（Phase 8a 决定）
5. Workspace switch 是否自动关闭当前所有 tabs（Phase 8b 决定）
