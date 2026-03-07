# CTM — Acceptance Criteria

每个 Phase 的完成标准。全部通过才能进入下一 Phase。

## Phase 1: 骨架 + 协议 + Client

### 自动化测试

| # | 测试 | 通过标准 |
|---|------|----------|
| 1.1 | `go build ./...` | exit 0，无 warning |
| 1.2 | `go test -race ./internal/config/ ./internal/protocol/ ./internal/client/` | exit 0 |
| 1.3 | Message round-trip | Request/Response/Error/Event 编解码一致 |
| 1.4 | NDJSON Reader | 多条消息、空行、截断行正确处理 |
| 1.5 | NDJSON Writer 线程安全 | `-race` 并发写入无 race |
| 1.6 | ID 唯一性 | 1000 个 ID 无重复 |
| 1.7 | Config 路径 | 返回正确路径，环境变量覆盖生效 |
| 1.8 | Fuzz NDJSON | `-fuzztime=30s` 无 panic |
| 1.9 | Client Connect | mock server 连接成功 |
| 1.10 | Client Request/Response | 请求发送、响应匹配、超时返回错误 |

### 手动验证
- `go run . version` 输出版本信息
- `go run . --help` 显示子命令列表
- `internal/config/`, `internal/protocol/`, `internal/client/`, `cmd/` 目录存在

---

## Phase 2: Daemon

### 自动化测试

| # | 测试 | 通过标准 |
|---|------|----------|
| 2.1-2.2 | 编译 + 全量 | exit 0 |
| 2.3 | Hub target 注册 | fake extension hello → targets.list 返回 |
| 2.4 | Hub 请求路由 | CLI → daemon → extension → response 返回 |
| 2.5 | Target 断连清理 | extension 断开 → targets.list 不含该 target |
| 2.6 | Session CRUD | save → list(summary) → get(full) → delete → list 为空 |
| 2.7 | Collection CRUD | create → addItems → get(含 items) → removeItems → delete |
| 2.8 | Subscribe + fanout | subscriber → extension event → subscriber 收到(含 _target) |
| 2.9 | 事件 pattern 过滤 | subscribe `tabs.*` → 只收 tabs，不收 groups |
| 2.10 | Flock 单例 | 两个 daemon → 第二个报错 |
| 2.11 | 优雅关闭 | SIGTERM → socket 删除，in-flight 完成 |
| 2.12 | 原子写入 | atomicWriteJSON 正确，中途 cancel 不留损坏文件 |
| 2.13 | Name 校验 | `../evil`、空、超长 → 错误 |

### 手动验证
- `ctm daemon --foreground` 启动输出日志
- `daemon.stop` → daemon 退出

---

## Phase 3: NM Shim + Install

### 自动化测试

| # | 测试 | 通过标准 |
|---|------|----------|
| 3.1 | NM 帧编解码 | 4-byte LE round-trip |
| 3.2 | 大消息 | ~1MB 正确处理 |
| 3.3 | 截断消息 | 不完整帧 → 错误，不 panic |
| 3.4 | Fuzz NM | `-fuzztime=30s` 无 panic |
| 3.5 | Bridge 双向转发 | stdin→socket, socket→stdout 正确 |
| 3.6 | Manifest 生成 | JSON 含正确路径和 allowed_origins |

### 手动验证
- `ctm install` → NM manifest + LaunchAgent plist 正确
- `ctm install --check` → 报告安装状态
- Chrome Extension 通过 Go shim 连接 Go daemon
- `~/.config/ctm/extension/manifest.json` 存在

---

## Phase 4: CLI

### 手动验证（需 daemon + extension）

| # | 命令 | 通过标准 |
|---|------|----------|
| 4.M1-M2 | `ctm targets list [--json]` | 显示 targets |
| 4.M3-M6 | `ctm tabs list/open/close/activate` | 操作生效 |
| 4.M7-M8 | `ctm groups list/create` | 操作生效 |
| 4.M9-M12 | `ctm sessions save/list/restore/delete` | 全流程 |
| 4.M13-M16 | `ctm collections create/add/restore/delete` | 全流程 |
| 4.M17 | Auto-start | kill daemon → `ctm tabs list` → 自动启动 |
| 4.M18 | Target 歧义 | 两 target 无 default → TARGET_AMBIGUOUS |
| 4.M19 | Default target | 设 default → 自动使用 |

---

## Phase 5: TUI

### 自动化测试 (teatest)

| # | 测试 | 通过标准 |
|---|------|----------|
| 5.3-5.5 | Boot (1 target / N+default / N no default) | 自动选/显示 picker |
| 5.6 | View 切换 | Tab/1/2/3/4 切换，header 更新 |
| 5.7 | Cursor 导航 | j/k/gg/G/Ctrl-D/U |
| 5.8 | Filter | / 进入，输入过滤，Enter 保留，Esc 清除 |
| 5.9-5.11 | Tab activate/close/multi-select+close | 正确请求 |
| 5.12-5.15 | Group create / Session save/restore/delete(D-D) | 正确请求 |
| 5.16-5.17 | Yank chord / Z-filter | y→y 复制 / z→p 过滤 pinned |
| 5.18-5.19 | Help overlay / Command palette | ?/: 工作 |
| 5.20 | Cursor isolation | Tabs cursor=5 → Sessions(3项) → clamp |
| 5.21-5.23 | Event batch / Toast auto-clear / Error persistent | 时序正确 |
| 5.24 | Reconnect | 断开 → disconnected → 重连恢复 |

### 手动验证
- `ctm tui` 启动显示 tabs
- activate/close 在浏览器生效
- 实时事件：浏览器新开/关闭 tab → TUI 自动更新
- 多 target：picker → 选择 → 事件隔离
- 断线重连：kill daemon → TUI disconnected → restart → 恢复
- Session/Collection 全流程
- Help 和 Status bar 准确性

---

## Phase 6: 分发

- GoReleaser dry-run 生成 darwin-arm64 + darwin-amd64
- `brew install user/tap/ctm` 成功
- `ctm install` → NM + LaunchAgent + extension 就位
- `ctm version` 显示正确版本号
- `brew upgrade` → `ctm install` → 功能正常

---

## Phase 7: Bookmarks

| # | 测试 | 通过标准 |
|---|------|----------|
| 7.2-7.5 | BookmarkNode 遍历 / Mirror 存储 / Overlay CRUD / Markdown 导出 | 单元 |
| 7.6-7.8 | bookmarks.tree/search/mirror action | 集成 |
| 7.9 | 书签事件 | Chrome 新增 → event → TUI 更新 |
| 7.10 | TUI Bookmarks view | 树展开/折叠/搜索/tag |
| 7.11 | 跨资源搜索 | 同时返回 tabs + sessions + bookmarks |

手动：`ctm bookmarks tree/search`, TUI 树形浏览, tag 设置, markdown 导出, 书签实时更新。

---

## Phase 8: Sync + Search + Workspace

| # | 测试 | 通过标准 |
|---|------|----------|
| 8.2 | Workspace CRUD | create/update/delete/list round-trip |
| 8.3 | workspace.switch | 关闭 tabs + 恢复 session |
| 8.4-8.5 | SyncEngine + 冲突检测 | mock 目录同步 + conflict file |
| 8.6-8.7 | sync.status/repair | 正确状态 |
| 8.8-8.9 | TUI Workspaces/Sync view | 列表/切换/创建/删除 + 状态/冲突 |

手动：workspace 创建/切换, iCloud 跨 Mac 同步, 冲突处理, `ctm sync repair`。

---

## Cross-Phase Checks（每个 Phase 完成后）

| # | 检查项 |
|---|--------|
| X.1 | `go test -race ./...` pass |
| X.2 | `go vet ./...` 无警告 |
| X.3 | `go mod graph` internal 包无循环 |
| X.4 | 覆盖率：protocol/config > 80%，daemon > 60% |
| X.5 | 新 action response 格式与 12_CONTRACTS.md 一致 |
| X.6 | 14_LESSONS.md 涉及的检查项全部勾选 |
