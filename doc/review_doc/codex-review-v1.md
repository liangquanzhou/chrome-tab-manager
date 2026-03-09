# Codex Review V1

## Verdict

当前仓库已经不是“只搭了个空架子”的状态。

我实际验证到：

- `go build ./...` 通过
- `go test -race ./...` 通过
- `go run . version` 通过
- `go run . --help` 通过
- 基础层已有一批真实单元测试

但我不会把这一轮判成“已经可以稳定进入真实浏览器联调 / 日常使用”的状态。  
原因不是代码量不够，而是有几条**最关键的产品链路**还没有闭环：

1. 安装链路还不能产出真实可用的 Chrome Native Messaging 配置
2. 多 target 设计已经进入协议和 daemon，但 CLI/TUI 还没真正用起来
3. TUI 的实时事件订阅没有真正接进 Bubble Tea 主循环
4. TUI 的 help / keymap 比实际行为更完整，存在“文档承诺 > 代码能力”

一句话判断：

- **基础层不错**
- **产品轮廓已经出来了**
- **但还没到可以放心继续往上堆功能的状态**

---

## Scope of This Review

这份 review 关注三件事：

1. 当前代码到底做到了什么程度
2. 有没有单元测试、哪些层有、哪些层没有
3. 目前最影响继续开发效率和产品信心的问题是什么

这份 review **不是**浏览器 E2E 验收。  
我没有在这轮里做真实 Chrome 扩展联调，只做了：

- 代码结构检查
- 本地编译/测试
- CLI 基本运行验证
- 文档与实现一致性检查

---

## What I Checked

### 目录与文件

我查看了这些区域：

- 仓库顶层
- `cmd/`
- `internal/`
- `doc/`
- `doc/review_doc/`

### 实际执行的命令

我实际跑了这些命令：

```bash
find ../chrome-tab-manager -maxdepth 2 -type f | sort | sed -n '1,240p'
find ../chrome-tab-manager/internal -maxdepth 2 -type f | sort
find ../chrome-tab-manager -type f -name '*_test.go' | sort

go build ./...
go test -race ./...

go run . version
go run . --help
go run . install --check
go run . tabs list
```

### 重点阅读的代码

- `cmd/install.go`
- `cmd/helpers.go`
- `cmd/tabs.go`
- `cmd/targets.go`
- `cmd/workspace.go`
- `internal/client/client.go`
- `internal/daemon/hub.go`
- `internal/protocol/messages.go`
- `internal/protocol/ndjson.go`
- `internal/tui/app.go`
- `internal/tui/keymap.go`
- `internal/tui/state.go`

### 重点对照的文档

- `doc/04_INTERACTION.md`
- `doc/09_DESIGN.md`
- `doc/10_TUI.md`
- `doc/11_PLAN.md`
- `doc/12_CONTRACTS.md`
- `doc/13_ACCEPTANCE.md`

---

## Execution Evidence

### Build

```bash
go build ./...
```

结果：

- 通过

### Tests

```bash
go test -race ./...
```

结果摘要：

- `ctm` / `cmd`：无测试文件
- `internal/bookmarks`：通过
- `internal/client`：通过
- `internal/config`：通过
- `internal/daemon`：通过
- `internal/nmshim`：通过
- `internal/protocol`：通过
- `internal/search`：通过
- `internal/sync`：通过
- `internal/tui`：无测试文件
- `internal/workspace`：通过

### CLI smoke

```bash
go run . version
```

结果：

- 输出 `ctm dev`

```bash
go run . --help
```

结果：

- 根命令可运行
- 顶层命令包括：
  - `bookmarks`
  - `collections`
  - `daemon`
  - `groups`
  - `install`
  - `search`
  - `sessions`
  - `sync`
  - `tabs`
  - `targets`
  - `tui`
  - `workspace`

### Install check

```bash
go run . install --check
```

结果：

```text
Binary: /Users/didi/Library/Caches/go-build/.../ctm
missing NM Manifest: not found
missing LaunchAgent: not found
```

这说明：

- `install --check` 能跑
- 但当前机器上尚未安装
- 更重要的是，生成路径和 manifest 逻辑仍需继续收口

### CLI runtime behavior

```bash
go run . tabs list
```

结果：

```text
Error: cannot connect to daemon (is it running?): connect /Users/didi/.config/ctm/daemon.sock: dial unix ... no such file or directory
```

这本身不算 bug。  
但它也暴露出：**当前 CLI 还没有实现文档承诺的 auto-start fallback**。

---

## Test Baseline

## 已存在的测试

仓库里**有单元测试**，而且不算少。不是“没有测试”。

当前有测试的包：

- `internal/bookmarks`
- `internal/client`
- `internal/config`
- `internal/daemon`
- `internal/nmshim`
- `internal/protocol`
- `internal/search`
- `internal/sync`
- `internal/workspace`

对应测试文件：

- `internal/bookmarks/bookmarks_test.go`
- `internal/client/client_test.go`
- `internal/config/paths_test.go`
- `internal/daemon/hub_test.go`
- `internal/daemon/storage_test.go`
- `internal/nmshim/shim_test.go`
- `internal/protocol/ids_test.go`
- `internal/protocol/messages_test.go`
- `internal/protocol/ndjson_test.go`
- `internal/protocol/ndjson_fuzz_test.go`
- `internal/search/search_test.go`
- `internal/sync/sync_test.go`
- `internal/workspace/workspace_test.go`

## 缺失的测试

当前没有这些测试：

- `internal/tui` 没有任何 `*_test.go`
- `cmd/` 没有测试

这和文档里 `Phase 5` 的承诺不一致：

- `doc/13_ACCEPTANCE.md:92-108`

文档里明确写了 TUI 要有：

- `teatest`
- boot 测试
- 视图切换测试
- `gg/G`
- filter
- tab activate/close
- yank / z-filter
- reconnect

但现在 `internal/tui` 目录里没有任何测试文件。

## 当前测试覆盖面的真实含义

可以这样理解：

- **基础层（protocol/client/daemon/storage）**：已经有可信测试
- **产品交互层（TUI）**：基本还是手工 smoke 状态

所以：

- 你可以相信底层现在不是纯拍脑袋写的
- 但不能因为 `go test -race ./...` 绿了，就默认 TUI 体验也可靠

---

## Findings

### High — `ctm install` 还不能生成真实可用的 Native Messaging 安装结果

`cmd/install.go` 目前仍然把 `allowed_origins` 写成占位符：

- `cmd/install.go:49-55`

```go
manifest := map[string]any{
    "name":            "com.ctm.native_host",
    "description":     "CTM Native Messaging Host",
    "path":            exe,
    "type":            "stdio",
    "allowed_origins": []string{"chrome-extension://EXTENSION_ID/"},
}
```

这意味着：

- 生成出来的 NM manifest 不能直接给真实 extension 用
- `ctm install` 现在更像“把两个文件写到磁盘上”，不是完整安装

此外，当前安装链还缺这些关键环节：

- extension 资源在哪里
- extension ID 从哪里来
- extension ID 和 `allowed_origins` 怎么对齐
- `ctm install` 是否负责把 extension 解到固定位置

而当前仓库里也没有顶层 `extension/` 目录。

影响：

- 浏览器端真实联调会卡在安装阶段
- 就算 daemon 和 nmshim 再完整，Chrome 也未必能真正连上

结论：

- 这是当前最实际的 blocker 之一

---

### High — 多 target 模型在协议层存在，但在 CLI/TUI 里基本还没有真正生效

协议和 daemon 明显已经按多 target 设计：

- `internal/protocol/messages.go:42-46`
- `internal/daemon/hub.go:376-401`

而且 daemon 的 target 解析逻辑并不差：

- 指定 target → 用它
- 有 default → 用 default
- 只有一个 target → 自动用
- 多个 target 无 default → 歧义错误

但上层调用几乎都没有传 target：

- `cmd/helpers.go:16-24`
- `cmd/tabs.go`
- `cmd/groups.go`
- `cmd/sessions.go`
- `cmd/collections.go`
- `cmd/bookmarks.go`
- `cmd/search.go`
- `cmd/workspace.go`
- `cmd/sync.go`

这些命令最终几乎都是：

```go
connectAndRequest(..., nil)
```

TUI 也类似：

- `internal/tui/app.go:56` 有 `selectedTarget`
- `internal/tui/app.go:618-623` 可以在 Targets view 选 target
- 但真正请求还是：
  - `internal/tui/app.go:508`
  - `internal/tui/app.go:594`

即：

```go
a.client.Request(..., nil)
```

这说明目前 target 的状态是：

- UI 有
- 协议有
- daemon 有
- 但请求层没有真正把它用起来

影响：

- 多 target 场景目前不可真正信赖
- target picker 更像是“产品意图已进入 UI”，不是闭环能力

结论：

- 这不是小瑕疵，是一条产品骨架没接完

---

### High — TUI 的实时事件订阅没有进入 Bubble Tea 主循环，实时刷新目前并未闭环

TUI 的 `connectCmd()` 会调用订阅：

- `internal/tui/app.go:457-478`

但收到 event 后，代码只是：

```go
for evt := range eventCh {
    _ = evt
}
```

见：

- `internal/tui/app.go:464-473`

同时：

- `eventMsg` 类型已经定义了：`internal/tui/app.go:24`
- `Update()` 也准备处理 `eventMsg`：`internal/tui/app.go:101-103`

所以这里很明显是：

- 设计准备好了
- 代码没有真正接到消息循环里

这会直接导致：

- TUI 不会因为真实的 tabs/groups 事件自动刷新
- 文档里对 Phase 5 的“实时事件”承诺没有兑现

对应文档：

- `doc/13_ACCEPTANCE.md:111-115`

结论：

- 这是 TUI 最关键的一个集成缺口

---

### High — CLI 文档承诺了 auto-start fallback，但当前实现并没有

文档里明确写了：

- `doc/11_PLAN.md:89-90`
- `doc/11_PLAN.md:176`
- `doc/13_ACCEPTANCE.md:86`

也就是：

- macOS 主路径：launchd
- fallback：CLI/TUI 连接失败时自动启动 daemon
- Phase 4 验收包含 `kill daemon -> ctm tabs list -> 自动启动`

但 `cmd/helpers.go` 现在只是：

- `cmd/helpers.go:16-24`

```go
if err := c.Connect(ctx); err != nil {
    return nil, fmt.Errorf("cannot connect to daemon (is it running?): %w", err)
}
```

我实际跑：

```bash
go run . tabs list
```

得到的也是直接失败，而不是 autostart。

这说明：

- 文档承诺存在
- CLI 行为还没跟上

影响：

- 第一轮使用体验会明显不顺
- 也会削弱你后面对“brew install 即用”的信心

---

### Medium — TUI 的 help / keymap / status hints 比真实实现更完整，存在“写了但没做完”的动作

`internal/tui/keymap.go` 明显在往完整交互模型靠：

- `gg`
- `G`
- `Space`
- `Enter`
- `y·`
- `z·`
- 各 view 的动作键

见：

- `internal/tui/keymap.go:13-94`

但实际实现只覆盖了其中一部分。

最明确的例子：

- `z` filter 在文档和 keymap 里存在
- 实际实现是 placeholder

见：

- `internal/tui/app.go:420-424`

```go
// Simplified z-filter: placeholder for Phase 5
return a, nil
```

类似地，keymap 里写的很多动作目前还只是“声明态”，并没有在 `handleKey` 或后续 handler 里形成完整行为闭环。

影响：

- Help 会误导用户
- Status bar 会展示不能真正依赖的动作
- 后续继续开发时，很容易误以为某些交互已经完成

结论：

- 这类问题不挡编译，但非常影响 TUI 可信度

---

### Medium — TUI 现在不是“完全没做”，但仍更像单文件原型而不是稳定交互层

`internal/tui/app.go` 现在已经很长，而且承担了太多职责：

- 连接
- 订阅
- 刷新
- 行为
- 过滤
- 渲染
- 帮助
- 状态栏
- 命令面板

虽然文档里设计的是更模块化的 TUI，但当前实现主要集中在一个文件里。

这本身不一定错，但对未来变更有明显风险：

- 你后面改交互会容易牵一发而动全身
- 很多行为只能靠读 `app.go` 发现
- TUI 测试缺席时，这种结构更容易积累微妙回归

这条我放 `Medium`，不是因为现在就必须大重构，而是因为：

- 如果继续往这个文件里堆功能
- 以后改 `Bookmarks / Workspaces / Sync` 的交互会越来越痛

---

### Medium — 协议错误码语义没有守住，target 相关错误被统一折叠成 `TARGET_AMBIGUOUS`

协议里定义了多种错误码：

- `TARGET_OFFLINE`
- `TARGET_AMBIGUOUS`
- `EXTENSION_NOT_CONNECTED`

见：

- `internal/protocol/messages.go:19-29`

但 `forwardToExtension()` 里只要 `resolveTarget()` 出错，就统一发：

- `internal/daemon/hub.go:345-349`

```go
sendError(..., protocol.ErrTargetAmbiguous, err.Error())
```

而 `resolveTarget()` 明明区分了：

- target 不存在
- 没有 target
- 多 target 歧义

见：

- `internal/daemon/hub.go:376-401`

影响：

- 上层错误语义不准
- 后续 CLI/TUI 文案很难精确
- 测试对协议契约的信心变弱

---

### Medium — 文档与命令命名空间有一处不一致：`workspaces` vs `workspace`

交互文档里定义的 CLI 顶层命名空间是：

- `ctm workspaces ...`

见：

- `doc/04_INTERACTION.md:84-96`

实际命令却是：

- `cmd/workspace.go:14-17`

```go
Use: "workspace"
```

这不是功能 bug，但它破坏了“文档是单一真相源”这件事。

影响：

- owner 看文档和实际 CLI 会产生落差
- agent 以后继续扩展时容易复制这种不一致

---

### Medium — `Phase 5` 文档承诺了 TUI 测试，但当前完全没有 `internal/tui` 测试文件

文档说：

- `doc/13_ACCEPTANCE.md:94-108`

当前现实是：

- `internal/tui/` 没有任何 `*_test.go`
- `go test -race ./...` 输出里是 `? ctm/internal/tui [no test files]`

这条我单独列出来，是因为它不只是“缺个测试”：

- 你最在意的交互手感
- 恰恰在当前最缺测试的层里

如果不尽快补：

- 以后每轮改 TUI 都会继续靠主观 smoke
- “看起来能用，但总觉得别扭”的问题会反复出现

---

### Low — `install --check` 现在显示的是 Go build cache 里的临时可执行路径

我实际跑：

```bash
go run . install --check
```

得到的 `Binary:` 是：

- `/Users/didi/Library/Caches/go-build/.../ctm`

这本身是 `go run` 的正常现象，但它提醒了一个产品风险：

- 当前安装逻辑对“真实发布二进制路径”和“开发时临时运行路径”还没有区分策略

如果这个问题不单独处理：

- 很容易把 build cache 路径写进安装配置
- 后面会出现“为什么重开之后失效”的问题

这条现在不阻塞继续写代码，但很值得在正式装机前收口。

---

## What Is Good

这轮也有不少明确值得肯定的地方：

### 1. 基础层已经比较像正式工程，不像随便堆出来的脚本项目

可以看到这些层已经都进入工程化状态：

- `config`
- `protocol`
- `client`
- `daemon`
- `nmshim`

而且大多有测试支撑。

### 2. 产品范围没有被偷偷缩回“tab 小工具”

命令面已经覆盖：

- `bookmarks`
- `search`
- `sync`
- `workspace`

说明产品世界观至少已经进入代码层，不是只停留在文档里。

### 3. CLI 顶层轮廓已经比较完整

虽然细节还没完全打通，但从 `go run . --help` 看，主资源域已经基本都有入口了。

### 4. daemon 的核心设计方向是对的

从 `internal/daemon/hub.go` 来看：

- target 注册
- request 路由
- event fanout
- pending response

这条主干方向没有跑偏。

### 5. 协议层设计是有意识的

`protocol_version`、`TargetSelector`、`ErrorCode` 这些都已经进入协议层，不是临时凑的。

---

## Subsystem Status Snapshot

这是我对当前子系统状态的高层判断。

| 子系统 | 状态 | 说明 |
|---|---|---|
| `config` | 稳 | 路径和目录逻辑已有测试 |
| `protocol` | 稳 | 编解码和 fuzz 已有 |
| `client` | 稳 | mock server 测试比较完整 |
| `daemon` | 中等偏稳 | Hub 路由和 CRUD 已有测试，但上层集成还没全部闭环 |
| `nmshim` | 稳 | framing 基本靠谱 |
| `CLI` | 中等 | 命令面全，但 autostart / target-aware 还没收口 |
| `TUI` | 弱 | 有实现，但关键链路和测试都没闭环 |
| `install/distribution` | 弱 | 目前还不能算真实可用安装链 |
| `bookmarks/search/sync/workspace` | 早期可用态 | 已进入代码，但还需要真实产品联调验证 |

---

## Recommended Fix Order

如果只按“最值钱、最能减少未来返工”的顺序修，我建议是：

### 1. 先修安装链路

至少收口这些问题：

- extension ID 的来源
- `allowed_origins`
- extension 资源路径
- `ctm install` 是否负责部署 extension
- 开发路径和发布路径的区别

这是浏览器联调的前提。

### 2. 再修 target-aware 请求

至少做到：

- CLI 支持 `--target`
- TUI 的 `selectedTarget` 真正进入请求
- default target 和 target picker 都变成真实行为

这条修完，多 target 才不只是“未来文档”。

### 3. 再修 TUI 事件链路

优先收口：

- subscribe → Bubble Tea 消息循环
- tabs/groups 事件刷新
- 断线 / 重连 / reconnect 提示

### 4. 再对齐 TUI keymap 与实现

要么：

- 实现 keymap 宣称的动作

要么：

- 把 help / status / keymap 下调到当前真实能力

不要长期维持“展示层比行为层更完整”。

### 5. 再补 TUI 自动化测试

我建议第一批就补：

- boot
- view switch
- `gg / G`
- filter
- target select
- tab activate / close
- session save / restore
- help overlay

---

## Suggested Acceptance Gates Before Real E2E

在进入真实浏览器 E2E 之前，我建议至少满足这几条：

1. `ctm install` 生成的安装结果不再含占位符  
2. `tabs/groups/sessions/collections` 都能带 target selector  
3. TUI 的 event loop 真正接通  
4. `internal/tui` 至少出现第一批交互测试  
5. CLI 的 autostart 行为和文档一致，或者文档先下调

---

## Direct Answers to Your Questions

### 1. 有没有单元测试？

**有，而且基础层不少。**

不是“没有测试”，而是：

- **基础层测试已有**
- **交互层测试明显缺**

### 2. 现在能不能继续开发？

**可以。**

但更准确地说：

- 可以继续做
- 不适合盲目继续堆功能

最值得先做的是把上面那 3-5 条主链路收口，不然越往后越容易返工。

### 3. 当前最该担心什么？

不是搜索、书签、workspace 这些未来功能本身。  
现在最该担心的是：

- 安装链路没闭环
- target 模型没真接上
- TUI 的实时和测试没闭环

这三件事如果不先修，后面很多“功能看起来有了”的东西，都会继续停留在半成品状态。

---

## Final Assessment

我对当前这轮代码的最终判断是：

- **不是失败**
- **不是空壳**
- **也还不是一个可以放心长期迭代的稳定基线**

它现在更像：

> 基础层已经站住了，产品层轮廓也出现了，但最关键的安装、target routing、TUI event loop 还没收口。

如果你要我给一个简短结论：

- **值得继续**
- **但先别急着往上堆更多 feature**
- **先把安装、target、TUI 实时和 TUI 测试补齐**

这会比继续加新命令更值钱。

---

## Phase Judgment

如果按文档里的 Phase 来判断，我的看法是：

- **Phase 1（骨架 + 协议 + Client）**：已过
- **Phase 2（Daemon）**：大体已过，至少基础路由和存储层已经具备
- **Phase 3（NM Shim + Install）**：`nmshim` 基本成形，但 `install` 还不能签收
- **Phase 4（CLI）**：命令面已铺开，但 `auto-start` 和 target-aware 还未收口
- **Phase 5（TUI）**：已有原型实现，但不能按文档验收标准签收
- **Phase 7/8 的能力域**：已经部分进入代码，但还不应被视为产品级完成

更准确地说：

- 当前仓库最像是 **“Phase 4.5”**
- 也就是：
  - 基础层已经超过最初骨架阶段
  - CLI 已经具备产品轮廓
  - TUI 已经开始成形
  - 但安装 / target / 事件 / TUI 测试还没有闭环

---

## What Is Actually Implemented Now

这部分是给 agent 和 owner 对齐“现在真实做到哪”的，不再靠猜。

### 已经明显实现的东西

- 协议消息模型
- NDJSON 编解码
- Native Messaging frame 编解码
- Unix socket client
- daemon Hub 路由
- sessions / collections 的本地存储与 CRUD
- bookmarks 基础操作
- search 基础匹配与 saved search 框架
- sync 基础目录同步和状态
- workspace 基础模型和 CRUD
- CLI 顶层命令面
- TUI 基础屏幕、键位、视图切换、过滤、保存/恢复/删除部分动作

### 还只是“部分实现”的东西

- install 完整链路
- target-aware routing
- TUI 实时事件刷新
- TUI 的完整 keymap 执行闭环
- TUI 的自动化测试
- 多 target 真实可用性

### 还不能假设完成的东西

- 真正的 Chrome 扩展联调
- 浏览器端真实 event 端到端
- 安装后即用体验
- 多 target 隔离体验
- TUI 和文档承诺的一致性

---

## Module-by-Module Review

### `cmd/`

优点：

- 顶层命令面已经比较完整
- 参数和输出风格整体统一
- `version` / `help` / 基本子命令都能跑

问题：

- `install` 只是半成品
- `workspace` 命名和文档不一致
- 没有 `--target`
- 没有 CLI auto-start fallback
- 没有命令层测试

建议：

- 先收口 target、install、autostart
- 再考虑命令层 golden/smoke tests

### `internal/config`

优点：

- 路径函数清晰
- 测试已存在
- `EnsureDirs()` 已有

风险：

- `SyncDir()` 目前只是本地路径选择，不等于真正 iCloud 产品行为完成

结论：

- 这层目前比较稳

### `internal/protocol`

优点：

- `Message` / `TargetSelector` / `ErrorCode` 已经成型
- NDJSON 读写和 fuzz 已有
- 方向正确

问题：

- 错误码虽然定义完整，但上层没有严格按语义使用

结论：

- 协议层设计是好的，当前主要问题在调用方

### `internal/client`

优点：

- mock server 测试完整度不错
- Request/Response、timeout、disconnect 都有覆盖

问题：

- event 通道虽有，但上层 TUI 没正确消费
- reconnect 逻辑存在，但产品层未完整利用

结论：

- 这层本身比 TUI 更可信

### `internal/daemon`

优点：

- Hub、pending、subscriber、storage 都进入了真实工程状态
- CRUD + 路由 + pattern fanout 已有测试

问题：

- 错误码映射不够精确
- target routing 虽存在，但上层还未充分利用

结论：

- daemon 是当前最像“可继续扩展基础”的一层

### `internal/nmshim`

优点：

- framing 和桥接逻辑有测试

结论：

- 基本靠谱
- 当前主要问题不是 shim，而是 install/extension 链路

### `internal/tui`

优点：

- 已经不是空白
- 视图、模式、键位、渲染、toast/error/confirm hint 都开始成型

问题：

- 事件链没接通
- keymap 与行为不完全一致
- target 选择没有真正影响请求
- 没有测试
- 单文件过大，后续变更风险高

结论：

- 这是当前最需要补强的一层

### `internal/bookmarks / search / sync / workspace`

优点：

- 已经进入实现，不只是文档承诺
- 都有一些单元测试

风险：

- 这些模块“有代码”不等于“产品闭环”
- 如果上层 interaction 和 install 没先收口，这几层暂时还发挥不出完整价值

结论：

- 可以继续保留，但不应抢在安装/target/TUI 基础问题之前成为主战场

---

## Document vs Code Mismatches

这部分很重要，因为你现在是文档驱动开发。

### 1. `workspaces` vs `workspace`

文档：

- `doc/04_INTERACTION.md:94`

```text
ctm workspaces ...
```

代码：

- `cmd/workspace.go:15`

```go
Use: "workspace"
```

建议：

- 统一成一套，不要长期双写法并存

### 2. CLI auto-start

文档：

- `doc/11_PLAN.md:89-90`
- `doc/11_PLAN.md:176`
- `doc/13_ACCEPTANCE.md:86`

代码：

- `cmd/helpers.go:21-23`

只会直接报错，不会 auto-start。

### 3. TUI Phase 5 testing

文档：

- `doc/13_ACCEPTANCE.md:94-108`

现实：

- `internal/tui/` 没有测试文件

### 4. TUI z-filter / richer actions

文档：

- `doc/10_TUI.md`

现实：

- `internal/tui/app.go:420-424`

还是 placeholder。

### 5. 完整安装链

文档：

- `doc/11_PLAN.md`

现实：

- `cmd/install.go` 仍依赖 placeholder `EXTENSION_ID`
- 仓库中未见真实 `extension/` 资源目录

---

## Hidden Risks If You Ignore This Review

如果现在直接继续堆功能，不先修这轮指出的问题，后面最可能发生的是：

### 1. 功能很多，但第一步就用不起来

因为：

- install 不完整
- daemon 不自动起来
- extension 不一定能连

结果：

- 你会看到很多命令
- 但真实体验是“每一步都要人工补环境”

### 2. 多 target 会变成产品级回归源

因为：

- UI 有 target
- 协议有 target
- 但请求没带 target

结果：

- 后面一旦真接 stable / beta / profile
- bug 会非常难查

### 3. TUI 会越来越“看起来像成品，其实行为不稳”

因为：

- keymap 在变多
- help 在变多
- 但没有自动化测试
- 事件链也还没闭环

结果：

- 每轮迭代都可能引入新的别扭感

### 4. 你会误以为 bookmarks/search/workspace/sync 已经“做完一半”

但实际上：

- 这些子系统在没有稳定 interaction + install + routing 的情况下
- 还没办法转化成可靠产品体验

---

## Concrete Fix List for Claude / Next Agent

下面这份可以直接当执行清单。

### P0 — 必须先修

#### P0.1 安装链路闭环

目标：

- `ctm install` 产出真实可用的 NM manifest
- 不再出现 `EXTENSION_ID` 占位符

至少要完成：

- 定义真实 extension ID 来源
- 写入正确 `allowed_origins`
- 明确 extension 资源路径
- 明确 `install` 对 extension 的职责

验收：

- `ctm install --check` 能报告完整安装状态
- manifest 不含 placeholder

#### P0.2 target-aware 请求闭环

目标：

- CLI 和 TUI 发请求时能真正指定 target

至少要完成：

- CLI 增加 `--target`
- TUI 的 `selectedTarget` 真正进入 request
- default target 行为和文档一致

验收：

- 代码里关键请求不再统一传 `nil`
- target 选择不再只是显示态

#### P0.3 TUI 实时事件接入主循环

目标：

- subscribe 到的 event 真能更新 UI

至少要完成：

- 把 `eventCh` 接入 Bubble Tea 消息循环
- `tabs.* / groups.*` 事件触发刷新

验收：

- 浏览器事件发生后 TUI 自动更新

### P1 — 紧接着做

#### P1.1 TUI keymap 与实现对齐

目标：

- help/status/keymap 不再超前于实现

做法二选一：

- 实现当前展示的动作
- 或临时下调提示文案

#### P1.2 CLI autostart fallback

目标：

- 文档和实现对齐

验收：

- daemon 不在时，CLI 能按设计自动拉起

#### P1.3 错误码语义收口

目标：

- `TARGET_OFFLINE`、`TARGET_AMBIGUOUS`、`EXTENSION_NOT_CONNECTED` 各归各位

### P2 — 再做

#### P2.1 TUI 自动化测试

第一批建议：

- boot
- view switch
- `gg / G`
- filter
- target select
- tab activate
- tab close
- session save / restore
- help overlay

#### P2.2 命名空间收口

目标：

- `workspace` / `workspaces` 统一

#### P2.3 TUI 结构拆分

目标：

- 逐步把 `internal/tui/app.go` 的职责拆开
- 不要继续往一个文件里堆所有交互

---

## Retest Checklist After Fixes

修完后，下一轮 review 最少应该重新验证这些：

```bash
go build ./...
go test -race ./...

go run . install --check
go run . tabs list
go run . targets list
go run . tui
```

如果已经接上真实浏览器，再补：

- `ctm install`
- 扩展加载
- `ctm tabs list`
- `ctm tabs open`
- `ctm tabs activate`
- `ctm sessions save`
- `ctm tui` 实时刷新

---

## What Not To Work On Yet

在 P0/P1 没收口前，我不建议把时间优先花在这些地方：

- 更丰富的 search ranking
- 更复杂的 workspace UX
- 更完整的 bookmark overlay 细节
- 更细的 sync 策略
- 更多 CLI 子命令打磨

原因不是这些不重要，而是：

- 当前真正卡住产品可信度的，不是“功能不够多”
- 而是“最关键链路还没闭环”

---

## Questions the Owner Can Ask Next Time

下次你不想读代码时，可以直接问 agent 这几句：

1. `ctm install` 现在是不是已经不再写 placeholder 了？
2. TUI 里选中的 target 会不会真的影响请求？
3. 浏览器里新开一个 tab，TUI 会不会自己刷新？
4. 当前哪些按键在 help 里有，但按下去其实还没实现？
5. 现在 `internal/tui` 有没有测试了？
6. 现在 CLI 还会不会在 daemon 不在时直接报错？
7. 这轮修的是“产品闭环”，还是只是又加了几个 feature？

---

## Bottom Line

如果我把这份 review 再压成一句最重要的话，就是：

> 当前代码已经值得继续，但继续的正确方向不是再加功能，而是先把安装、target、TUI 实时和 TUI 测试这四条主链路收口。

这四条一旦收口，后面再做 bookmarks/search/workspace/sync，产品会顺很多。  
如果不先收口，后面做得越多，返工和“看起来有、实际上不稳”的问题只会越多。
