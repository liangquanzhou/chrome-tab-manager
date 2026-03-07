# CTM — Acceptance Criteria

每个 Phase 的完成标准。AI 编码完成后必须逐项验证，全部通过才能进入下一 Phase。

## Phase 1: 骨架 + 协议 + Client

### 自动化测试（必须通过）

| # | 测试 | 命令 | 通过标准 |
|---|------|------|----------|
| 1.1 | 编译 | `go build ./...` | exit 0，无 warning |
| 1.2 | 单元测试 | `go test -race ./internal/config/ ./internal/protocol/` | exit 0 |
| 1.3 | Message round-trip | `go test -run TestMessageRoundTrip ./internal/protocol/` | Request/Response/Error/Event 编解码 round-trip 一致 |
| 1.4 | NDJSON Reader | `go test -run TestNDJSONReader ./internal/protocol/` | 多条消息、空行、截断行正确处理 |
| 1.5 | NDJSON Writer 线程安全 | `go test -race -run TestNDJSONWriter ./internal/protocol/` | 并发写入无 race |
| 1.6 | ID 生成唯一性 | `go test -run TestMakeID ./internal/protocol/` | 1000 个 ID 无重复 |
| 1.7 | Config 路径 | `go test -run TestPaths ./internal/config/` | 返回正确路径，环境变量覆盖生效 |
| 1.8 | Fuzz NDJSON | `go test -fuzz=FuzzNDJSON ./internal/protocol/ -fuzztime=30s` | 无 panic |
| 1.9 | Client Connect | `go test -run TestClientConnect ./internal/client/` | mock server 连接成功 |
| 1.10 | Client Request/Response | `go test -run TestClientRequest ./internal/client/` | 请求发送、响应匹配、超时返回错误 |

### 手动验证

| # | 验证项 | 通过标准 |
|---|--------|----------|
| 1.M1 | `go run . version` | 输出版本信息（dev） |
| 1.M2 | cobra help | `go run . --help` 显示子命令列表 |
| 1.M3 | 项目结构 | `internal/config/`, `internal/protocol/`, `internal/client/`, `cmd/` 目录存在 |

### 产出物

- [ ] `go.mod` + `go.sum`
- [ ] `cmd/root.go`, `cmd/version.go`
- [ ] `internal/config/paths.go` + `paths_test.go`
- [ ] `internal/protocol/messages.go`, `ndjson.go`, `ids.go` + 测试文件
- [ ] `internal/client/client.go` + `client_test.go`

---

## Phase 2: Daemon

### 自动化测试（必须通过）

| # | 测试 | 命令 | 通过标准 |
|---|------|------|----------|
| 2.1 | 编译 | `go build ./...` | exit 0 |
| 2.2 | 全量测试 | `go test -race ./...` | exit 0 |
| 2.3 | Hub target 注册 | 集成测试 | fake extension 发 hello → targets.list 返回该 target |
| 2.4 | Hub 请求路由 | 集成测试 | CLI 发 tabs.list → daemon 转发到 extension → 响应返回 CLI |
| 2.5 | Target 断连清理 | 集成测试 | extension 断开 → targets.list 不再包含该 target |
| 2.6 | Session CRUD | 集成测试 | save → list（返回 summary）→ get（返回 full）→ delete → list 为空 |
| 2.7 | Collection CRUD | 集成测试 | create → addItems → get（含 items）→ removeItems → delete |
| 2.8 | Subscribe + fanout | 集成测试 | subscriber 注册 → extension 发 event → subscriber 收到（含 _target） |
| 2.9 | 事件 pattern 过滤 | 集成测试 | subscribe `tabs.*` → 只收到 tabs 事件，不收到 groups 事件 |
| 2.10 | Flock 单例 | 集成测试 | 启动两个 daemon → 第二个报错退出 |
| 2.11 | 优雅关闭 | 集成测试 | SIGTERM → socket 文件删除，in-flight 请求完成或超时 |
| 2.12 | 原子写入 | 单元测试 | atomicWriteJSON 写入内容正确，中途 cancel 不留损坏文件 |
| 2.13 | Name 校验 | 单元测试 | `../evil`, 空字符串, 超长名 → 返回错误 |

### 手动验证

| # | 验证项 | 通过标准 |
|---|--------|----------|
| 2.M1 | Daemon 启动 | `ctm daemon --foreground` 启动，输出日志 |
| 2.M2 | Extension 连接 | TS 版 extension 通过 NM 连接到 Go daemon（需 Phase 3 的 shim，此阶段可用直接 socket 连接测试） |
| 2.M3 | daemon.stop | 发送 `daemon.stop` → daemon 退出 |

### 产出物

- [ ] `internal/daemon/server.go`, `targets.go`, `sessions.go`, `collections.go`, `subscribe.go`, `state.go`
- [ ] `cmd/daemon.go`
- [ ] 集成测试文件

---

## Phase 3: NM Shim + Install

### 自动化测试（必须通过）

| # | 测试 | 命令 | 通过标准 |
|---|------|------|----------|
| 3.1 | NM 帧编解码 | 单元测试 | 4-byte LE encode/decode round-trip |
| 3.2 | 大消息处理 | 单元测试 | 接近 1MB 的消息正确编解码 |
| 3.3 | 截断消息 | 单元测试 | 不完整帧 → 返回错误，不 panic |
| 3.4 | Fuzz NM | `go test -fuzz=FuzzNMFrame ./internal/nmshim/ -fuzztime=30s` | 无 panic |
| 3.5 | Bridge 双向转发 | 集成测试 | stdin → socket 和 socket → stdout 双向数据正确 |
| 3.6 | Install manifest 生成 | 单元测试 | 生成的 JSON 包含正确路径和 allowed_origins |

### 手动验证

| # | 验证项 | 通过标准 |
|---|--------|----------|
| 3.M1 | ctm install | 运行后 NM manifest 和 LaunchAgent plist 存在且内容正确 |
| 3.M2 | ctm install --check | 报告安装状态（manifest 存在、路径正确、extension 目录存在） |
| 3.M3 | Extension 全链路 | Chrome 加载 extension → extension 通过 Go NM shim 连接 Go daemon → `targets.list` 显示 target |
| 3.M4 | Extension 解压 | `~/.config/ctm/extension/manifest.json` 存在且内容正确 |

### 产出物

- [ ] `internal/nmshim/shim.go` + 测试文件
- [ ] `cmd/nmshim.go`, `cmd/install.go`
- [ ] `extension/` 目录（嵌入用）

---

## Phase 4: CLI

### 自动化测试（必须通过）

| # | 测试 | 命令 | 通过标准 |
|---|------|------|----------|
| 4.1 | 编译 | `go build ./...` | exit 0 |
| 4.2 | 全量测试 | `go test -race ./...` | exit 0 |

### 手动验证（需要 daemon + extension 运行）

| # | 验证项 | 命令 | 通过标准 |
|---|--------|------|----------|
| 4.M1 | Targets | `ctm targets list` | 显示在线 targets（table 格式） |
| 4.M2 | Targets JSON | `ctm targets list --json` | 有效 JSON |
| 4.M3 | Tabs list | `ctm tabs list` | 显示当前 tabs |
| 4.M4 | Tabs open | `ctm tabs open https://example.com` | 新 tab 打开 |
| 4.M5 | Tabs close | `ctm tabs close --tab-id <id>` | tab 关闭 |
| 4.M6 | Tabs activate | `ctm tabs activate --tab-id <id> --focus` | tab 激活 + 窗口前台 |
| 4.M7 | Groups list | `ctm groups list` | 显示 tab groups |
| 4.M8 | Groups create | `ctm groups create --title test --tab-id <id1> --tab-id <id2>` | group 创建 |
| 4.M9 | Sessions save | `ctm sessions save test-session` | 保存成功 |
| 4.M10 | Sessions list | `ctm sessions list` | 显示 test-session |
| 4.M11 | Sessions restore | `ctm sessions restore test-session` | tabs 恢复 |
| 4.M12 | Sessions delete | `ctm sessions delete test-session` | 删除成功 |
| 4.M13 | Collections create | `ctm collections create test-coll` | 创建成功 |
| 4.M14 | Collections add | `ctm collections add test-coll --url https://example.com --title Example` | 添加成功 |
| 4.M15 | Collections restore | `ctm collections restore test-coll` | tabs 打开 |
| 4.M16 | Collections delete | `ctm collections delete test-coll` | 删除成功 |
| 4.M17 | Auto-start daemon | 杀掉 daemon → `ctm tabs list` → daemon 自动启动并返回结果 |
| 4.M18 | Target 歧义 | 两个 target 在线，无 default → `ctm tabs list` → 报 TARGET_AMBIGUOUS |
| 4.M19 | Default target | `ctm targets default <id>` → `ctm tabs list` → 使用 default |

### 产出物

- [ ] `cmd/tabs.go`, `cmd/groups.go`, `cmd/sessions.go`, `cmd/collections.go`, `cmd/targets.go`

---

## Phase 5: TUI

### 自动化测试（必须通过）

| # | 测试 | 命令 | 通过标准 |
|---|------|------|----------|
| 5.1 | 编译 | `go build ./...` | exit 0 |
| 5.2 | 全量测试 | `go test -race ./...` | exit 0 |
| 5.3 | Boot 1 target | teatest | 自动选中 target，显示 tabs view |
| 5.4 | Boot N targets + default | teatest | 自动选中 default target |
| 5.5 | Boot N targets no default | teatest | 显示 target picker |
| 5.6 | View 切换 | teatest | Tab/1/2/3/4 切换视图，header 更新 |
| 5.7 | Cursor 导航 | teatest | j/k 移动 cursor，gg/G 跳转，Ctrl-D/U 翻页 |
| 5.8 | Filter | teatest | / 进入 filter，输入文本过滤，Enter 保留，Esc 清除 |
| 5.9 | Tab activate | teatest | Enter → 发送 tabs.activate 请求 |
| 5.10 | Tab close | teatest | x → 发送 tabs.close 请求 |
| 5.11 | Multi-select + close | teatest | Space 选中多个 → x → 批量关闭 |
| 5.12 | Create group | teatest | G → 输入 title → Enter → 发送 groups.create |
| 5.13 | Session save | teatest | n → 输入 name → Enter → 发送 sessions.save |
| 5.14 | Session restore | teatest | o → 发送 sessions.restore |
| 5.15 | Session delete D-D | teatest | D → D → 发送 sessions.delete；D → Esc → 取消 |
| 5.16 | Yank chord | teatest | y → y → 复制 URL 到剪贴板 |
| 5.17 | Z-filter chord | teatest | z → p → 只显示 pinned tabs |
| 5.18 | Help overlay | teatest | ? → 显示帮助 → Esc 关闭 |
| 5.19 | Command palette | teatest | : → 输入命令 → Enter 执行 |
| 5.20 | Cursor isolation | teatest | Tabs view cursor=5 → 切到 Sessions(3项) → cursor clamp 到 2 |
| 5.21 | Event batch | teatest | 模拟多个 event → 150ms 内聚合为一次更新 |
| 5.22 | Toast auto-clear | teatest | 触发 toast → 3s 后消失 |
| 5.23 | Error persistent | teatest | 触发 error → 保持显示 → Esc 清除 |
| 5.24 | Reconnect | teatest | 断开 mock server → TUI 显示 disconnected → 重连后恢复 |

### 手动验证（需要 daemon + extension 运行）

| # | 验证项 | 通过标准 |
|---|--------|----------|
| 5.M1 | TUI 启动 | `ctm tui` 正常启动，显示 tabs |
| 5.M2 | Tab 操作 | activate/close 在浏览器中生效 |
| 5.M3 | 实时事件 | 在浏览器中新开 tab → TUI 自动显示 |
| 5.M4 | 实时事件关闭 | 在浏览器中关闭 tab → TUI 自动移除 |
| 5.M5 | 多 target | 两个 target → target picker → 选择后显示对应 tabs |
| 5.M6 | 事件隔离 | 在 target A 的浏览器中操作 → 仅 target A 的 TUI 数据更新 |
| 5.M7 | 断线重连 | kill daemon → TUI 显示 disconnected → restart daemon → TUI 自动恢复 |
| 5.M8 | Session 全流程 | save → list → restore → delete，每步 TUI 反馈正确 |
| 5.M9 | Collection 全流程 | create → add from tabs → expand → restore → delete |
| 5.M10 | Help 准确性 | ? 显示的按键与实际行为一致 |
| 5.M11 | Status bar 准确性 | 每个 view 的 status bar 提示与实际可用操作一致 |

### 产出物

- [ ] `internal/tui/` 全部文件
- [ ] `cmd/tui.go`
- [ ] teatest 测试文件

---

## Phase 6: 分发

### 自动化测试（必须通过）

| # | 测试 | 命令 | 通过标准 |
|---|------|------|----------|
| 6.1 | GoReleaser dry-run | `goreleaser release --snapshot --clean` | 生成 darwin-arm64 + darwin-amd64 二进制 |
| 6.2 | 全量测试 | `go test -race ./...` | exit 0 |

### 手动验证

| # | 验证项 | 通过标准 |
|---|--------|----------|
| 6.M1 | Homebrew 安装 | `brew install user/tap/ctm` 成功 |
| 6.M2 | Post-install | `ctm install` → NM manifest + LaunchAgent + extension 全部就位 |
| 6.M3 | 全链路 | `ctm install && ctm tui` → 完整功能可用 |
| 6.M4 | Version | `ctm version` 显示正确版本号（非 dev） |
| 6.M5 | Upgrade | 新版本 `brew upgrade ctm` → `ctm install` → 功能正常 |

### 产出物

- [ ] `.goreleaser.yml`
- [ ] `Makefile`
- [ ] Homebrew tap 仓库

---

## Phase 7: Bookmarks + Knowledge Layer

### 自动化测试

| # | 测试 | 通过标准 |
|---|------|----------|
| 7.1 | 编译 + 全量测试 | `go test -race ./...` pass |
| 7.2 | BookmarkNode 树遍历 | 单元测试：递归遍历、搜索、路径计算 |
| 7.3 | BookmarkMirror 存储 | 单元测试：save/load round-trip |
| 7.4 | BookmarkOverlay CRUD | 单元测试：set/get/delete overlay |
| 7.5 | Markdown 导出 | 单元测试：树 → Markdown 格式正确 |
| 7.6 | bookmarks.tree action | 集成测试：extension 返回书签树 → daemon 转发 |
| 7.7 | bookmarks.search action | 集成测试：搜索关键词返回匹配书签 |
| 7.8 | bookmarks.mirror action | 集成测试：完整镜像保存到本地 |
| 7.9 | 书签事件 | 集成测试：Chrome 新增书签 → event 推送 → TUI 更新 |
| 7.10 | TUI Bookmarks view | teatest：树展开/折叠、搜索、tag 设置 |
| 7.11 | 跨资源搜索 | teatest：搜索同时返回 tabs + sessions + bookmarks 匹配 |

### 手动验证

| # | 验证项 | 通过标准 |
|---|--------|----------|
| 7.M1 | `ctm bookmarks tree` | 显示完整书签树（缩进格式） |
| 7.M2 | `ctm bookmarks search <keyword>` | 返回匹配书签 |
| 7.M3 | TUI Bookmarks view | 树形浏览、h/l 展开折叠、/ 搜索 |
| 7.M4 | Tag 设置 | TUI 中 `t` 设置 tag → overlay 持久化 |
| 7.M5 | Markdown 导出 | `ctm bookmarks export --folder "Bookmarks Bar"` 输出正确 |
| 7.M6 | 书签实时更新 | 在 Chrome 中添加书签 → TUI 自动显示 |

---

## Phase 8: Sync + Workspace

### 自动化测试

| # | 测试 | 通过标准 |
|---|------|----------|
| 8.1 | 编译 + 全量测试 | `go test -race ./...` pass |
| 8.2 | Workspace CRUD | 单元测试：create/update/delete/list round-trip |
| 8.3 | workspace.switch | 集成测试：关闭当前 tabs + 恢复 workspace session |
| 8.4 | SyncEngine 基础 | 单元测试：local → cloud 文件同步（mock 目录） |
| 8.5 | 冲突检测 | 单元测试：两端都修改 → 保留较新 + 生成 .conflict 文件 |
| 8.6 | sync.status | 集成测试：返回正确的同步状态 |
| 8.7 | sync.repair | 集成测试：重建索引后状态一致 |
| 8.8 | TUI Workspaces view | teatest：列表、展开、切换、创建、删除 |
| 8.9 | TUI Sync view | teatest：显示同步状态、冲突列表 |

### 手动验证

| # | 验证项 | 通过标准 |
|---|--------|----------|
| 8.M1 | Workspace 创建 | `ctm workspace create dev-project` 成功 |
| 8.M2 | Workspace 切换 | `ctm workspace switch dev-project` → 当前 tabs 关闭 + session 恢复 |
| 8.M3 | TUI Workspace view | 列出 workspaces，`o` 切换成功 |
| 8.M4 | iCloud 同步 | Mac A 保存 session → Mac B 执行 `ctm sync` → session 出现 |
| 8.M5 | 冲突处理 | 两台 Mac 同时修改同一 collection → 冲突文件生成 → Sync view 显示 |
| 8.M6 | 同步修复 | `ctm sync repair` → 状态恢复正常 |

---

## 跨 Phase 检查（每个 Phase 完成后都要做）

| # | 检查项 | 方法 |
|---|--------|------|
| X.1 | Race condition | `go test -race ./...` pass |
| X.2 | 编译警告 | `go vet ./...` 无警告 |
| X.3 | 无循环依赖 | `go mod graph` 中 internal 包无循环 |
| X.4 | 测试覆盖率 | `go test -coverprofile` ，protocol/config > 80%，daemon > 60% |
| X.5 | CONTRACTS 合规 | 新增 action 的 response 格式与 CONTRACTS.md 一致 |
| X.6 | LESSONS 检查项 | 本 Phase 涉及的 LESSONS.md 检查项全部勾选 |
