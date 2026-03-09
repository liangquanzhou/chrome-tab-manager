# Codex Review V10

## 范围

这轮不是针对单个回答文件，也不是只看某一层。

我重新从顶层看了当前仓库的几条主线：

- build / test / coverage
- contract / registry / matrix / phase 文档
- CLI / daemon / extension 的实际链路
- install / doctor / release 配置

目标是回答两个问题：

1. 当前还有没有真实未闭环问题
2. 项目现在的主要风险到底是在代码层，还是在“真相源分叉”层

---

## 结论

**当前 CTM 的主风险已经不是“系统跑不起来”，而是“顶层真相源开始彼此分叉”。**

代码层本身比前几轮明显更稳：

- `go build ./...` 通过
- `go vet ./...` 通过
- `go test -count=1 ./...` 通过
- 全量覆盖率当前为 **75.6%**
- 包级覆盖率：
  - `ctm/cmd`: **56.5%**
  - `ctm/internal/daemon`: **89.4%**
  - `ctm/internal/tui`: **60.6%**

但从顶层看，现在最值得优先处理的不是继续堆功能，而是：

> **把 contract、registry、capability matrix、phase 文档重新收成同一个现实版本。**

---

## Findings

### Medium: `downloads.cancel` 当前有真实链路错误，CLI / contract 和 extension 对 payload 字段名不一致

这是这轮最明确的功能性问题。

当前三层定义不一致：

- CLI 发送的是 `downloadId`
- contract 文档也写的是 `downloadId`
- 但 extension `handleDownloadsCancel()` 实际要求的是 `id`

daemon 对 `downloads.cancel` 只是直接 forward，不做 payload 适配，所以这个不一致会直接落到运行时错误：

> `downloads.cancel: id is required`

这不是文档问题，而是当前用户路径会实际失败的问题。

证据：

- `cmd/downloads.go`：`connectAndRequest("downloads.cancel", map[string]any{"downloadId": id}, ...)`
- `doc/12_CONTRACTS.md`：`downloads.cancel` request 写的是 `downloadId`
- `extension/service-worker.js`：`handleDownloadsCancel()` 检查 `payload.id`

另外，这条链路当前也没有明显的命令级回归测试保护。

### Medium: `18_CURRENT_PHASE.md` 仍然明显过期，已经不能作为 current truth 使用

这份文档现在和当前仓库状态存在直接冲突。

最明显的几处：

- 还写着 `57` 个 action
- 还写着 `S=32 / P=16 / R=9`
- 还写着 `Groups view` 的 `Enter handler` 缺失
- 还写着 `daemon 30.6%` 覆盖率需提升

这些都和当前代码现实不一致。

这意味着 `18_CURRENT_PHASE.md` 现在更像历史快照，而不是当前阶段说明。

### Medium: `19_CAPABILITY_MATRIX.md` 依然存在结构性自相矛盾

这份文档比前几轮已经强很多，但还没有收口成可直接引用的 current-state 表。

我这轮重新计数 action 表格行，当前是：

- 总 action row: **62**
- `S=31`
- `P=19`
- `R=12`

但 summary 仍写：

- `S=32`
- `P=16`
- `R=9`

同时，`Coverage Gaps Summary` 里还保留了已经失效的描述，例如：

- `Groups: Enter 在 handleEnter 中无 ViewGroups case`

而当前代码里 `ViewGroups` 的 `handleEnter()` 分支已经存在。

也就是说，这份 matrix 现在不是“方向错”，而是“局部已准、汇总未收口”。

### Medium: `ActionRegistry`、`12_CONTRACTS.md`、`Capability Matrix`、实际 CLI 暴露面已经开始分叉

这是这轮最值得重视的顶层问题。

现在仓库里已经不止一个“真相源”：

- `internal/daemon/registry.go`
- `doc/12_CONTRACTS.md`
- `doc/19_CAPABILITY_MATRIX.md`
- 实际 CLI 命令面

但这四者并没有完全一致。

明确例子：

- `daemon.stop`
  - `registry.go` 记为 `CLISupported`
  - `19_CAPABILITY_MATRIX.md` 说可通过 `ctm daemon stop`
  - 但当前 `cmd/` 里并没有这个 CLI 子命令
  - `12_CONTRACTS.md` 反而写的是 `CLI: internal only`

- `bookmarks.get`
  - `registry.go` 记为 `CLISupported`
  - `12_CONTRACTS.md` 写的是 `CLI: internal only`
  - `19_CAPABILITY_MATRIX.md` 也写 CLI `X`
  - 实际 `cmd/bookmarks.go` 没有 `get`

- `bookmarks.overlay.set/get`
  - `registry.go` 记为 `CLISupported`
  - `12_CONTRACTS.md` 写 `CLI: internal only`
  - 实际 `cmd/` 里没有公开命令

- `downloads.cancel`
  - `19_CAPABILITY_MATRIX.md` 有
  - `12_CONTRACTS.md` 有
  - 实际 CLI 有
  - 但 `registry.go` 里甚至没有登记 `downloads.cancel`

- `history.delete`
  - matrix 有
  - extension 有
  - CLI 有
  - `registry.go` 没有

这说明一个很关键的事实：

> **项目已经开始建设 authoritative contract / action registry，但它们还没形成单一真相。**

这会直接影响：

- review 的准确性
- 能力边界判断
- CLI/TUI 的支持等级标注
- 后续自动校验和文档生成

### Low: `.goreleaser.yml` 的 Homebrew 配置仍然是模板占位值，分发路径还不是可直接发布状态

这不是运行时 bug，但它说明“分发 DONE”还不能按产品级标准理解。

当前 `.goreleaser.yml` 里仍是：

- `owner: user`
- `name: homebrew-tap`
- `homepage: https://github.com/user/chrome-tab-manager`

这说明 release 配置骨架在，但还不是可直接发布的真实配置。

如果后续要把 distribution 当正式能力收口，这块仍需要补真实 owner/repo/homepage 和对应 smoke。

---

## 已关闭 / 应降级的问题

这轮也确认了几条过去反复提到的问题，已经不应该再按旧说法继续写。

### 1. `12_CONTRACTS.md` 的 selector 说明这轮已经修正

当前文档开头已经明确写成：

> 当前唯一实现的 selector 是 `targetId`；`channel` / `label` 仅存在于 protocol struct 中，daemon 和 CLI 都未实现。

这意味着之前那条“contract 文档夸大 selector 能力”的 finding，这轮已经关闭。

### 2. bookmark mirror 不应再继续按“非 target-aware”写

这条技术债的旧表述已经过期。

当前 daemon 侧已经做了这些事：

- mirror 写入同时支持 `mirror_<targetId>.json`
- 读取时优先 per-target mirror，再 fallback 到 `mirror.json`
- `searchBookmarks()` 会遍历 per-target mirror 文件并做去重
- `bookmarks.mirror` contract 里也已经写了 per-target / fallback 行为

所以现在更准确的说法应该是：

> bookmark mirror 已经进入 target-aware 过渡实现，不再是“单全局 mirror.json”的旧状态。

它是否还需要进一步产品化，是后续演进问题；但“仍然不是 target-aware”这句已经不准了。

### 3. `doctor` 已经不是 `install --check` 的简单 alias

`doctor` 现在已经做最小 runtime health check：

- binary
- manifest
- LaunchAgent
- daemon socket reachability
- `targets.list`
- extension connection count

所以现在不能再把它描述成“只做文件存在性检查”。

---

## 当前项目的真实状态

我现在会这样定义当前 CTM：

> **代码内核已经进入 consolidation，项目治理层还没有进入 single-source-of-truth。**

这句话拆开就是：

### 代码层

- 主链路已成
- contract / doctor / exit code / typed error 都在继续收口
- 核心包测试仍然足够强，尤其是 `internal/daemon`
- 项目已经不是原型

### 项目层

- `Phase` 文档过期
- `Capability Matrix` 汇总不准
- `ActionRegistry` 还不能直接当 canonical inventory
- release 配置还没完全产品化

所以当前最该做的不是新增一层 UI 或再加一组 action，而是：

1. 修真实 bug：先修 `downloads.cancel`
2. 统一 action inventory：`registry` / `contracts` / `matrix` / CLI 面必须统一
3. 缩减真相源数量：明确谁是 canonical，谁由谁生成
4. 重新定义 `18_CURRENT_PHASE.md` 的职责：不要再让它维护易过期细节

---

## 建议的下一步顺序

### P0

1. 修 `downloads.cancel` payload 字段不一致
2. 给这条链路补 CLI + daemon/extension integration test

### P1

1. 统一 `ActionRegistry`
2. 用它反向校验 `12_CONTRACTS.md`
3. 用它反向校验 `19_CAPABILITY_MATRIX.md`

### P2

1. 把 `18_CURRENT_PHASE.md` 改成阶段说明 + 指向 canonical truth
2. 不再在 phase 文档里手写 action 计数、包覆盖率、具体 handler 缺失

### P3

1. 把 `.goreleaser.yml` 从模板值改成真实发布配置
2. 补 release smoke path

---

## 最后判断

**现在这个项目的主要问题，已经不是“代码没做完”，而是“系统已经长到必须有单一真相源”。**

如果继续在当前状态下叠加功能，会出现的不是立即崩掉，而是：

- review 越来越难对齐
- 文档越来越快过期
- 支持等级越来越说不清
- CLI/TUI/release 的边界越来越模糊

所以当前这轮最值得做的，不是“再加功能证明系统还能长”，而是：

> **把 action inventory、contract、capability matrix、phase 叙述收成一个版本。**

只有这一步做完，后面的扩展才会继续是低摩擦的。
