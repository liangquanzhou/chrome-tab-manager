# Codex Review V2

## Verdict

这轮要更直白一点：

- 当前仓库**不是没有测试**
- 但它离“每个功能点都有测试”还差得很远
- 如果现在有人说“测试已经比较完整”，这个判断是**站不住的**

我实际跑了覆盖率之后，结论非常明确：

- `go test -cover ./...` 可通过
- 总覆盖率只有 **34.8%**
- 覆盖高度不均衡
- 基础库层测试不错
- 用户真正会用到的命令层、恢复链路、TUI 运行时链路，测试明显不足

一句话判断：

**现在的测试状态更像“核心库和部分后端逻辑有单测”，不是“完整产品有系统测试保障”。**

---

## Scope of This Review

这份 `v2` 的目标不是重复 `v1`，而是把下面三件事一次性说透：

1. 当前单元测试到底覆盖到了哪一层
2. 哪些用户功能点实际上没有测试保护
3. 下一轮 agent 必须补哪些测试，才算进入“可持续开发”状态

这份 review 重点讨论：

- Go 单元测试
- 覆盖率
- 功能点到测试的映射
- 风险优先级
- 测试补齐清单

这份 review **不等于**真实 Chrome 浏览器 E2E 验收。

---

## Commands I Actually Ran

我实际执行了这些命令：

```bash
go test -cover ./...
go test -coverprofile=/tmp/ctm-cover2.out ./...
go tool cover -func=/tmp/ctm-cover2.out
rg -n '^func Test|^func Fuzz' internal cmd --glob '*_test.go'
```

我不是只看文件数量或只看 `*_test.go` 是否存在，而是同时对了：

- 每个包的覆盖率
- 每个关键函数是否真的被跑到
- 每个用户功能背后有没有相应测试

---

## Coverage Snapshot

当前覆盖率结果：

- `ctm`: `0.0%`
- `ctm/cmd`: `0.0%`
- `internal/bookmarks`: `96.2%`
- `internal/client`: `80.4%`
- `internal/config`: `88.5%`
- `internal/daemon`: `30.6%`
- `internal/nmshim`: `30.2%`
- `internal/protocol`: `88.9%`
- `internal/search`: `96.3%`
- `internal/sync`: `65.9%`
- `internal/tui`: `23.4%`
- `internal/workspace`: `100.0%`
- total: **`34.8%`**

这个结构非常说明问题：

- 底层纯逻辑库测得不错
- 用户入口层几乎没测
- 中间业务编排层测了一部分，但远不够
- TUI 有测试文件，但对真实运行时帮助还不够

---

## High-Level Judgment

### 现在测得比较好的部分

这些模块的测试可以算“实打实有价值”：

- `internal/protocol`
- `internal/config`
- `internal/bookmarks`
- `internal/search`
- `internal/client`
- `internal/workspace`

它们的共同特点是：

- 纯函数或弱副作用
- 输入输出边界明确
- 不依赖复杂多进程链路

### 现在测得不够的部分

这些才是产品真正危险的地方：

- `cmd/` 命令层
- `internal/daemon` 的大部分 handler
- `sessions.restore`
- `collections.restore`
- `internal/tui` 的运行时交互
- `internal/nmshim.Run`

也就是说：

**越接近“真实产品行为”的地方，测试越薄。**

---

## Findings

### High: CLI 层几乎完全裸奔

`ctm/cmd` 当前覆盖率是 **`0.0%`**。

这不是“偏低”，这是：

- 所有顶层用户入口都没有自动化单测保护
- 所有 flag 解析、参数校验、命令输出、错误路径都可能悄悄坏掉

当前没有测试保护的命令包括：

- `install`
- `daemon`
- `targets`
- `tabs`
- `groups`
- `sessions`
- `collections`
- `bookmarks`
- `search`
- `sync`
- `workspaces`
- `tui`
- `version`
- `nm-shim`

这意味着什么？

- 你改一个 `cobra` flag，没人会第一时间发现行为漂了
- 你改一个默认 target 逻辑，可能 build 绿、库测试绿，但 CLI 已经坏了
- 你改输出格式，后面 TUI / shell script / agent workflow 都可能被悄悄破坏

这是当前测试体系最大的结构性缺口。

---

### High: daemon 只有“骨架测试”，大量业务 handler 没被覆盖

`internal/daemon` 当前覆盖率是 **`30.6%`**。

这不是说 daemon 完全没测。  
事实上它测了几条重要主线：

- target 注册
- request routing
- session CRUD 的一部分
- collection CRUD 的一部分
- subscribe / fanout
- singleton flock
- name validation

这些测试主要在：

- `internal/daemon/hub_test.go`
- `internal/daemon/storage_test.go`

但更关键的问题是：  
大量真正承接产品功能的 handler 还是 **0%**。

完全没被跑到的核心 handler 包括：

- `internal/daemon/bookmarks_handler.go`
  - `handleBookmarksTree`
  - `handleBookmarksSearch`
  - `handleBookmarksGet`
  - `handleBookmarksMirror`
  - `handleBookmarksOverlaySet`
  - `handleBookmarksOverlayGet`
  - `handleBookmarksExport`
- `internal/daemon/search_handler.go`
  - `handleSearchQuery`
  - `searchSessions`
  - `searchCollections`
  - `searchWorkspaces`
  - `searchBookmarks`
  - `searchTabs`
  - `handleSearchSavedList`
  - `handleSearchSavedCreate`
  - `handleSearchSavedDelete`
- `internal/daemon/workspace_handler.go`
  - `handleWorkspaceList`
  - `handleWorkspaceGet`
  - `handleWorkspaceCreate`
  - `handleWorkspaceUpdate`
  - `handleWorkspaceDelete`
  - `handleWorkspaceSwitch`
- `internal/daemon/sync_handler.go`
  - `handleSyncStatus`
  - `handleSyncRepair`
  - `syncLocalDir`

这意味着：

- 文档里写了很多能力
- 代码里也写了很多 handler
- 但测试层面，这些能力几乎还没进入“受保护状态”

如果现在继续大量往上加功能，不先补这些 handler 测试，后面维护成本会快速上升。

---

### High: restore 是产品核心链路，但 restore 相关测试明显不够

这是最需要单独强调的点。

从产品角度看，下面这些不是“附属功能”，而是产品核心价值：

- `sessions.save`
- `sessions.restore`
- `collections.restore`
- `workspace.switch`

但目前覆盖率显示：

- `internal/daemon/sessions.go: handleSessionsRestore` -> `0.0%`
- `internal/daemon/collections.go: handleCollectionsRestore` -> `0.0%`

这意味着当前测试没有真正兜住这些关键风险：

- 恢复时目标 target 是否选对
- 恢复时 payload 格式是否正确
- 恢复错误是否能结构化返回
- 恢复时 partial failure 怎么处理
- 空 session / 空 collection 怎么处理
- restore 到 extension 的转发链是否正确

也就是说：

**最像“产品”的功能，现在还没有被单元测试真正保护。**

---

### Medium: TUI 有测试文件，但运行时主路径大面积未覆盖

`internal/tui` 当前覆盖率是 **`23.4%`**。

这个数字容易误导人。因为它会让人产生一种错觉：

- “TUI 已经开始有测试了”

这句话只说一半是对的。

对的部分：

- `internal/tui/app_test.go` 已经有不少测试
- 一些状态逻辑确实覆盖到了

已经测到的内容包括：

- `ViewState.visibleCount`
- `ViewState.clampCursor`
- `ViewState.realIndex`
- `nextView`
- `moveCursor`
- `gg / G`
- filter
- `z` filter
- `matchesFilter`
- selection helpers
- `truncate`
- `extractHost`
- `flattenBookmarkTree`
- `parsePayload`

但真正危险的点是：

`internal/tui/app.go` 里这些关键运行时函数几乎都是 **0%**：

- `Init`
- `Update`
- `View`
- `handleKey`
- `handleFilterKey`
- `handleCommandKey`
- `handleNameInputKey`
- `handleYankKey`
- `handleConfirmDeleteKey`
- `connectCmd`
- `waitForEvent`
- `refreshCurrentView`
- `applyRefresh`
- `targetSelector`
- `doRequest`
- `handleEnter`
- `closeTabs`
- `handleRestore`
- `saveSession`
- `createCollection`
- `setDefaultTarget`
- `showToast`
- `executeCommand`
- `renderHeader`
- `renderContent`
- `renderItem`
- `renderStatusBar`
- `renderHelp`

所以更准确的描述应该是：

- TUI **有一批 state/helper 单测**
- 但 **真实交互路径几乎没测**

这两者不能混为一谈。

---

### Medium: client 基础不错，但 reconnect 这种关键韧性路径没测

`internal/client` 覆盖率是 **`80.4%`**，这算不错。

它已经测了：

- connect
- request/response
- timeout
- not connected
- error response
- events
- disconnect
- close

但是：

- `Reconnect` 当前是 **`0.0%`**

这意味着：

- 平时 happy path 看起来稳
- 断线重连这种真正影响产品体验的韧性路径，没有被证明

如果这个项目以后是 daemon + nm-shim + extension + TUI 的长期运行模型，`Reconnect` 不应该是未覆盖状态。

---

### Medium: multi-target 设计已经有了，但 default/label/error path 还没真正测透

`internal/daemon/hub.go` 里和 multi-target 相关的一些核心分支覆盖率仍然偏低或为 0：

- `handleTargetsDefault` -> `0.0%`
- `handleTargetsClearDefault` -> `0.0%`
- `handleTargetsLabel` -> `0.0%`
- `targetErrorCode` -> `0.0%`
- `resolveTarget` -> `28.6%`

这代表什么？

- 代码里已经有多 target 世界观
- 但默认 target、清除默认、label、target 错误码这些“产品真正可见”的行为，还没有被系统证明

这类功能如果没有测试，后面最容易出现的情况是：

- 功能大致能跑
- 但边界行为一直漂
- 用户偶尔遇到奇怪错误
- agent 每轮改动都要重新猜

---

### Medium: nm-shim 只测了 frame 编解码，没测主运行流程

`internal/nmshim` 当前覆盖率是 **`30.2%`**。

已测：

- `ReadNMFrame`
- `WriteNMFrame`
- large message / truncated / zero length / too large
- fuzz

没测：

- `Run` -> `0.0%`

这意味着：

- framing 规则测了
- 但真正的 shim 生命周期、stdin/stdout 桥接、daemon socket 代理主流程没有测

如果以后浏览器联调不稳，这里会是一个很典型的“库测试都绿，但真实链路有问题”的来源。

---

### Medium: sync 核心引擎测了，但 daemon 对外暴露面没测

`internal/sync` 是 `65.9%`，本身不算差。

说明：

- 同步引擎核心逻辑有一定测试

但对产品来说，用户不会直接调用 `sync.NewSyncEngine()`，而是会走：

- daemon handler
- CLI 命令
- TUI view

而这几层目前都没有形成完整保护链。

所以当前可以说：

- “sync 内核有测试”
- 不能说“sync 功能整体有测试”

---

### Low: 覆盖率高不等于功能完整

`internal/workspace` 是 `100%`，`internal/bookmarks` 和 `internal/search` 也都很高。

这当然是好事。  
但不能把它误读成：

- bookmarks 功能整体成熟
- workspace 功能整体成熟
- search 功能整体成熟

更准确地说：

- 这些包里的纯逻辑函数测得比较干净
- 但从 CLI/daemon/TUI 到真实产品功能的整条链路，还没有同等程度的保障

---

## Feature-by-Feature Test Reality

下面这张表更接近你真正关心的问题：

“每个功能点，到底有没有测试？”

### 1. Install / Bootstrap

功能点：

- `install`
- `install --check`
- manifest 路径生成
- LaunchAgent 路径生成
- extension id 注入

当前状态：

- 基本没有命令层测试
- `cmd/install.go` 是 `0%`

结论：

- **没有真正自动化保护**

建议：

- 命令测试
- golden output 测试
- 临时目录下的文件生成测试

### 2. Daemon lifecycle

功能点：

- `daemon` 启动
- socket 创建
- lock file
- 单实例
- stop

当前状态：

- `Server.Start`、`acquireLock` 有部分测试
- `Hub.Stop` / `handleDaemonStop` 仍未覆盖

结论：

- **部分有测，但不完整**

### 3. Targets

功能点：

- target 注册
- `targets list`
- default target
- clear default
- label
- ambiguous target
- target-not-found

当前状态：

- target 注册、list 有测试
- default/clear/label/error path 没测透

结论：

- **只测了半条链**

### 4. Tabs

功能点：

- `tabs list`
- `tabs open`
- `tabs close`
- `tabs activate`

当前状态：

- daemon forwarding 有主线测试
- CLI 没测
- TUI action 没测
- extension/browser E2E 不属于单元测试

结论：

- **有一部分中间层测试，但没有产品级保护**

### 5. Groups

功能点：

- `groups list`
- `groups create`

当前状态：

- 主要还是依赖 routing 层间接覆盖
- 命令层和 TUI 层无系统测试

结论：

- **测试不足**

### 6. Sessions

功能点：

- `list`
- `get`
- `save`
- `restore`
- `delete`

当前状态：

- CRUD 部分路径有测
- `restore` 是关键缺口

结论：

- **save/list/get/delete 勉强有些保护**
- **restore 还不够**

### 7. Collections

功能点：

- `list`
- `get`
- `create`
- `delete`
- `add`
- `restore`

当前状态：

- CRUD 一部分有测试
- `restore` 没测

结论：

- **和 sessions 一样，restore 是明显短板**

### 8. Bookmarks

功能点：

- tree
- search
- get
- mirror
- overlay
- export

当前状态：

- `internal/bookmarks` 纯逻辑测得很好
- daemon handler 完全没测
- CLI 没测

结论：

- **底层逻辑强，产品路径弱**

### 9. Search

功能点：

- query
- host match
- saved search list/create/delete
- unified search across resources

当前状态：

- `internal/search` 纯逻辑强
- daemon search handler 几乎 0

结论：

- **搜索算法有测，搜索产品功能没测**

### 10. Sync

功能点：

- status
- repair
- sync to cloud
- sync from cloud

当前状态：

- engine 侧中等偏好
- daemon/CLI 暴露面很弱

结论：

- **中间层不错，入口层不足**

### 11. Workspaces

功能点：

- list
- get
- create
- update
- delete
- switch

当前状态：

- `internal/workspace` 的 pure model 是 100%
- daemon workspace handler 是 0%
- CLI 也是 0%

结论：

- **模型层强，真实功能层弱**

### 12. TUI

功能点：

- init
- connect
- subscribe
- event refresh
- multi-view navigation
- filter
- command mode
- yank
- delete confirm
- enter actions
- render
- status/help consistency

当前状态：

- helper/state 测了一些
- 真实交互循环没测
- render 没测
- event loop 没测
- action dispatch 没测

结论：

- **现在不能说“TUI 有完整测试”**

---

## What This Means in Practice

如果现在继续只看 `go test` 是绿的，就会出现一种很危险的错觉：

- 代码看起来稳定
- 实际上只有底层库稳定
- 越靠近真实用户行为，越缺保护

最容易出问题的地方就是：

- 改一个 CLI flag
- 改一个 target 选择逻辑
- 改一个 restore payload
- 改一个 TUI 交互键
- 改一个 daemon handler 分支

然后：

- build 还是绿
- 大部分测试还是绿
- 但真实功能已经悄悄漂了

---

## Concrete Unit Test Work That Still Needs To Be Done

下面这部分是这份 review 最重要的内容。  
不是泛泛地说“要补测试”，而是明确到**应该补什么测试**。

### P0: 先补用户入口和核心恢复链路

#### 1. `cmd` 层测试

至少要补这些：

- `cmd/install_test.go`
  - `install --check`
  - 缺 extension id
  - 有 extension id
  - manifest/LaunchAgent 缺失时输出
- `cmd/targets_test.go`
  - `targets list`
  - `default`
  - `clear-default`
  - `label`
- `cmd/tabs_test.go`
  - `list`
  - `open`
  - `close`
  - `activate`
- `cmd/sessions_test.go`
  - `list`
  - `get`
  - `save`
  - `restore`
  - `delete`
- `cmd/collections_test.go`
  - `list`
  - `get`
  - `create`
  - `delete`
  - `add`
  - `restore`
- `cmd/bookmarks_test.go`
  - `tree`
  - `search`
  - `mirror`
  - `export`
- `cmd/search_test.go`
  - unified search
  - saved search list
- `cmd/sync_test.go`
  - `status`
  - `repair`
- `cmd/workspace_test.go`
  - `list`
  - `get`
  - `create`
  - `delete`
  - `switch`

这里不需要一开始就上全量真 socket。  
先用 fake client / stub response 也比完全没有强。

#### 2. restore 测试

必须补：

- `internal/daemon/sessions_restore_test.go`
  - restore 正常路径
  - target 缺失
  - extension 返回错误
  - 空 windows/tabs/groups
  - payload 格式错误
- `internal/daemon/collections_restore_test.go`
  - restore 正常路径
  - collection 不存在
  - item 为空
  - extension 错误

这是当前最像“产品价值核心”的测试缺口。

---

### P1: 补 multi-target 和业务 handler

#### 3. target 行为测试

补这些 daemon 测试：

- `handleTargetsDefault`
- `handleTargetsClearDefault`
- `handleTargetsLabel`
- `resolveTarget`
  - explicit target
  - default target
  - single target fallback
  - ambiguous target
  - offline target
- `targetErrorCode`

#### 4. bookmarks handler 测试

补这些：

- `bookmarks.tree`
- `bookmarks.search`
- `bookmarks.get`
- `bookmarks.mirror`
- `bookmarks.overlay.set`
- `bookmarks.overlay.get`
- `bookmarks.export`

#### 5. search handler 测试

补这些：

- search across sessions
- search across collections
- search across workspaces
- search across bookmarks
- search across tabs
- saved search list/create/delete

#### 6. workspace handler 测试

补这些：

- list
- get
- create
- update
- delete
- switch

#### 7. sync handler 测试

补这些：

- `sync.status`
- `sync.repair`
- local sync dir traversal
- handler error mapping

---

### P2: 补 TUI 真实交互测试

当前 `internal/tui/app_test.go` 不能删，但必须升级。

至少补这些：

- `Init` 连接命令触发
- `Update` 对 key 的真实处理
- `Update` 对 event 的处理
- `View` 基本 render smoke
- `handleEnter` 在各 view 的主操作
- `handleRestore`
- `saveSession`
- `createCollection`
- `setDefaultTarget`
- `executeCommand`
- `waitForEvent`
- `refreshCurrentView`

建议做法：

- 保留现有纯逻辑测试
- 再加一层 Bubble Tea model 交互测试
- 至少覆盖：
  - tabs view
  - sessions view
  - collections view
  - targets view

否则现在的 TUI 测试只能说明：

- “一些 helper 正常”

不能说明：

- “TUI 真的能稳定工作”

---

### P2: 补 nm-shim 主流程测试

至少补：

- `Run` happy path
- daemon socket 连接失败
- stdin 读错误
- stdout 写错误
- 双向转发
- shutdown path

---

## Suggested Testing Layers

为了避免 agent 下轮继续乱补测试，我建议直接按 4 层来建：

### Layer 1: Pure unit tests

适用：

- config
- protocol
- bookmarks
- search
- workspace
- sync internals
- TUI state helpers

### Layer 2: Handler tests

适用：

- daemon handlers

方式：

- fake storage
- fake extension conn
- request/response assertions

### Layer 3: Command tests

适用：

- `cmd/*`

方式：

- execute Cobra command
- capture stdout/stderr
- fake client
- fake config dir

### Layer 4: TUI interaction tests

适用：

- Bubble Tea app

方式：

- key input
- event injection
- state assertion
- render smoke

---

## What Not To Misread

下面这些结论是错的，不要再让 agent 这么汇报：

- “有很多测试，所以产品已经比较稳了”
- “TUI 有测试了，所以 TUI 风险不高”
- “bookmarks/search/workspace 覆盖率高，所以相关功能没问题”
- “daemon 有测试了，所以 handler 层差不多”

更准确的说法应该是：

- 底层纯逻辑测试不错
- 中层业务编排测试部分覆盖
- 产品入口层和交互层仍然明显不足

---

## Retest Criteria For The Next Round

下轮如果 agent 说“测试补齐了”，我会看这些：

1. `ctm/cmd` 不再是 `0.0%`
2. `internal/daemon` 至少提升到一个更像样的水平，重点不是总数，而是 handler 不再大面积 `0%`
3. `sessions.restore` 和 `collections.restore` 不再是 `0%`
4. `internal/tui` 不再只是 helper 测试，`Update/View/Init` 至少有一部分被实际跑到
5. `internal/nmshim.Run` 不再是 `0%`
6. 新测试不是只堆 happy path，要包含错误路径

---

## Bottom Line

如果只回答你最关心的那个问题：

### 单元测试覆盖度怎么样？

**一般，不算高，而且结构失衡。**

### 每个功能点都有测试吗？

**没有。差得还比较明显。**

### 现在最危险的缺口在哪？

按优先级排：

1. `cmd` 层没有测试
2. `restore` 核心链路没测透
3. daemon 业务 handler 大面积未覆盖
4. TUI 真实交互没测
5. multi-target 边界没测透

### 当前能不能继续开发？

能。  
但如果不先补上面这些测试，后面开发会越来越依赖人工回归，改一次飘一次。

### 我建议下一轮最应该做什么？

不是再加新功能。  
而是：

- 先补 `cmd` 测试
- 再补 `restore` 测试
- 再补 daemon handler 测试
- 最后补 TUI 交互测试

这才是当前最值钱的投入。
