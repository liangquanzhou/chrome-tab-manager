# Capability Matrix

Current state of each feature across daemon, extension, CLI, and TUI layers.

Generated from code audit on 2026-03-08. Support levels added on 2026-03-08. Last reconciled on 2026-03-10.

## Legend

### 实现状态
- **V** = implemented and working
- **T** = partial (see notes)
- **X** = not implemented
- **-** = not applicable for this layer

### 支持等级
- **S** (Supported) = 正式支持 -- 有完整用户路径 (daemon handler + CLI/TUI 入口), 可作为产品承诺
- **P** (Partial) = 部分支持 -- 底层存在但用户路径不完整 (缺 handler/入口/行为不一致)
- **R** (Reserved) = 预留/实验 -- 底层预留或实验性, 不算当前产品承诺

---

## Tabs

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| tabs.list | V(fwd) | V | V | V(2) | V | S |
| tabs.open | V(fwd) | V | V | V(:open) | V | S |
| tabs.close | V(fwd) | V | V | V(x) | V | S |
| tabs.activate | V(fwd) | V | V | V(Enter) | V | S |
| tabs.update | V(fwd) | V | X | X | - | R |
| tabs.mute | V(fwd) | V | V | V(m) | - | S |
| tabs.pin | V(fwd) | V | V | V(p) | - | S |
| tabs.move | V(fwd) | V | V | V(M) | - | S |
| tabs.getText | V(fwd) | V | V | V(v) | - | S |
| tabs.capture | V(fwd) | V | V | V(v) | - | S |

Notes:
- `tabs.update` 在 extension 中只处理 `pinned`，CLI/TUI 使用更具体的 `tabs.pin` 代替 -> R
- TUI tab preview `v` 循环三种模式：info / text(getText) / screenshot(capture)
- TUI `P` 键启动 w3m/lynx 预览
- TUI `M` 键移动 tab 到指定 window
- TUI `A` 键将选中 tab(s) 添加到 collection
- CLI `tabs text <id>` 提取页面文本；`tabs capture <id> -o file.png` 截图

## Groups

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| groups.list | V(fwd) | V | V | V(3) | V | S |
| groups.create | V(fwd) | V | V | V(n) | - | S |
| groups.update | V(fwd) | V | V | V(Enter) | - | S |
| groups.delete | V(fwd) | V | V | V(DD) | - | S |

Notes:
- TUI `n` 创建分组，`DD` 删除分组
- CLI `groups update <id> --title --color`，`groups delete <id>`
- 4 个 action 均有 daemon + CLI + TUI -> 全部 S

## Sessions

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| sessions.list | V | - | V | V(4) | V | S |
| sessions.get | V | - | V | V(Enter) | V | S |
| sessions.save | V | V* | V | V(n) | V | S |
| sessions.restore | V | V* | V | V(o) | V | S |
| sessions.delete | V | - | V | V(DD) | V | S |

Notes:
- * sessions.save 需要 extension 提供 tabs.list + groups.list 数据；sessions.restore 通过 extension tabs.open + groups.create 执行
- Daemon 使用 atomicWriteJSON 持久化
- 所有 5 个 action 均有完整的 daemon + CLI + TUI 路径 -> 全部 S

## Collections

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| collections.list | V | - | V | V(5) | V | S |
| collections.get | V | - | V | V(Enter) | V | S |
| collections.create | V | - | V | V(n) | V | S |
| collections.addItems | V | - | V | V(A) | V | S |
| collections.removeItems | V | - | V | V(x) | V | S |
| collections.restore | V | V* | V | V(o) | V | S |
| collections.delete | V | - | V | V(DD) | V | S |

Notes:
- * collections.restore 通过 extension tabs.open 执行
- TUI `A` 键（在 Tabs 视图）将选中 tab(s) 添加到指定 collection
- TUI `x` 键（在展开的 collection 嵌套 tab 上）移除单个 item
- CLI `collections remove-items <name> --urls <url1,url2,...>`
- 7 个 action 均有 daemon + CLI + TUI -> 全部 S

## Targets

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| targets.list | V | - | V | V(1) | V | S |
| targets.default | V | - | V | V(d) | V | S |
| targets.clearDefault | V | - | V | V(c) | V | S |
| targets.label | V | - | V | V(l) | V | S |

Notes:
- targets 状态在 Hub actor 内存管理，不持久化
- 4 个 action 全部 S

## Bookmarks

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| bookmarks.tree | V | V | V | V(6) | V | S |
| bookmarks.search | V(fwd) | V | V | V(/) | V | S |
| bookmarks.get | V(fwd) | V | X | X | - | R |
| bookmarks.mirror | V | V* | V | V(r) | V | S |
| bookmarks.export | V | - | V | V(E) | V | S |
| bookmarks.create | V(fwd) | V | V | V(a) | V | S |
| bookmarks.update | V(fwd) | V | X | X | - | R |
| bookmarks.remove | V(fwd) | V | V | V(DD) | V | S |
| bookmarks.move | V(fwd) | V | X | X | - | R |
| bookmarks.overlay.set | V | - | X | X | V | R |
| bookmarks.overlay.get | V | - | X | X | V | R |

Notes:
- * bookmarks.mirror 优先读本地缓存，缓存不存在则请求 extension bookmarks.tree 后缓存
- bookmarks.tree 同时更新本地 mirror
- TUI bookmarks view 支持 tree fold/unfold (Enter, zM, zR)
- Extension 发送 bookmarks.created / bookmarks.changed / bookmarks.removed 事件
- `bookmarks.get` / `bookmarks.update` / `bookmarks.move` 无 CLI 无 TUI -> R
- `bookmarks.overlay.set` / `bookmarks.overlay.get` 仅 daemon handler + 测试 -> R

## Workspaces

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| workspace.list | V | - | V | V(7) | V | S |
| workspace.get | V | - | V | V(Enter) | V | S |
| workspace.create | V | - | V | V(n) | V | S |
| workspace.update | V | - | V | V(e) | V | S |
| workspace.delete | V | - | V | V(DD) | V | S |
| workspace.switch | V | V* | V | V(o) | V | S |

Notes:
- * workspace.switch 通过 extension 执行：先 tabs.list + tabs.close 关闭所有 tab，再 tabs.open + groups.create 恢复首个 session
- TUI Enter = inspect (显示 workspace 详情)，`o` = switch，`e` = 编辑名称
- CLI `workspace update <id> --name --description --status`
- 6 个 action 全部 S

## Search

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| search.query | V | T* | V | V(/) | V | S |
| search.saved.list | V | - | V | V(0) | V | S |
| search.saved.create | V | - | V | V(n) | V | S |
| search.saved.delete | V | - | V | V(DD) | V | S |

Notes:
- * search.query 在 daemon 内跨资源搜索：tabs（通过 extension tabs.list）、sessions、collections、bookmarks（通过 mirror 文件）、workspaces
- TUI Search view (按 `0`) 支持 `/` 触发跨资源搜索，默认显示 saved searches 列表
- 搜索范围可通过 `--scope` 限制
- 4 个 action 均有 daemon + CLI + TUI view -> S

## History

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| history.search | V(fwd) | V | V | V(9) | - | S |
| history.delete | V(fwd) | V | V | V(DD) | - | S |

Notes:
- TUI History view 是第 9 个 view，按 `9` 切换
- TUI Enter 打开历史记录 URL
- 完整路径: daemon(fwd) + CLI + TUI -> S

## Downloads

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| downloads.list | V(fwd) | V | V | V(Tab) | - | S |
| downloads.cancel | V(fwd) | V | V | V(x) | - | S |

Notes:
- TUI Downloads view 通过 Tab 键循环切换到达，`x` 取消下载 -> S

## Windows

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| windows.list | V(fwd) | V | X | X | - | R |
| windows.create | V(fwd) | V | X | X | - | R |
| windows.close | V(fwd) | V | X | X | - | R |
| windows.focus | V(fwd) | V | X | X | - | R |

Notes:
- Daemon 转发到 extension，但无 CLI 命令和 TUI 操作 -> R
- sessions.restore / workspace.switch 内部使用 windows 能力

## Sync

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| sync.status | V | - | V | V(8) | V | S |
| sync.repair | V | - | V | V(R) | V | S |

Notes:
- TUI Sync view (按 `8`) 显示同步状态，`r` 刷新，`R` 执行 repair
- iCloud sync engine 对 sessions/collections/workspaces 目录做文件级同步
- 2 个 action 全部 S

## Daemon Control

| Action | Daemon | Extension | CLI | TUI | Tests | Level |
|--------|--------|-----------|-----|-----|-------|-------|
| daemon.stop | V | - | X | X | V | R |
| subscribe | V | - | - | V | V | S |

Notes:
- daemon.stop 仅通过 daemon 内部请求触发；无 CLI 子命令、无 TUI 入口 -> R
- TUI 启动时自动 subscribe `tabs.*`, `groups.*`, `bookmarks.*` -> S

## Events (Extension -> Daemon -> TUI)

| Event | Extension | Daemon | TUI |
|-------|-----------|--------|-----|
| tabs.created | V | V(fanout) | V(refresh) |
| tabs.removed | V | V(fanout) | V(refresh) |
| tabs.updated | V | V(fanout) | V(refresh) |
| tabs.activated | V | V(fanout) | V(refresh) |
| tabs.moved | V | V(fanout) | V(refresh) |
| bookmarks.created | V | V(fanout) | V(refresh) |
| bookmarks.changed | V | V(fanout) | V(refresh) |
| bookmarks.removed | V | V(fanout) | V(refresh) |

---

## Support Level Summary

### S (Supported) -- 52 actions
完整用户路径，可作为产品承诺:
- **Tabs**: list, open, close, activate, mute, pin, move, getText, capture (9)
- **Groups**: list, create, update, delete (4)
- **Sessions**: list, get, save, restore, delete (5)
- **Collections**: list, get, create, addItems, removeItems, restore, delete (7)
- **Targets**: list, default, clearDefault, label (4)
- **Bookmarks**: tree, search, mirror, create, remove, export (6)
- **Workspaces**: list, get, create, update, delete, switch (6)
- **Search**: query, saved.list, saved.create, saved.delete (4)
- **Sync**: status, repair (2)
- **History**: search, delete (2)
- **Downloads**: list, cancel (2)
- **Daemon**: subscribe (1)
- **Events**: 8 event types (全部 S，不计入 action 总数)

### P (Partial) -- 0 actions
无

### R (Reserved) -- 11 actions
底层预留或实验性:
- **Tabs**: update (被 pin/mute 替代) (1)
- **Bookmarks**: get, update, move, overlay.set, overlay.get (仅 daemon handler) (5)
- **Windows**: list, create, close, focus (内部管道，被 sessions.restore / workspace.switch 使用) (4)
- **Daemon**: stop (仅内部请求，无 CLI/TUI) (1)

---

## Coverage Gaps Summary

### CLI 缺失命令 (有 daemon handler 但无 CLI)
- `windows list`, `windows create`, `windows close`, `windows focus`
- `bookmarks update`, `bookmarks move`, `bookmarks get`
- `bookmarks overlay set`, `bookmarks overlay get`

### TUI 缺失操作 (有 daemon handler / CLI 但无 TUI handler)
- Bookmarks: overlay 无 TUI 操作

### 测试缺失
- tabs.mute, tabs.pin, tabs.move, tabs.getText, tabs.capture: 无单元测试
- groups.create, groups.update, groups.delete: 无单元测试
- bookmarks.update, bookmarks.move: 无单元测试
- history.search, history.delete: 无单元测试
- downloads.list, downloads.cancel: 无单元测试
- windows.list, windows.create, windows.close, windows.focus: 无单元测试
- collections.addItems, collections.removeItems: 无单元测试 (daemon handler 级)
