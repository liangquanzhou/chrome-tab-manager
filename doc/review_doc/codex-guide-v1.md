# Codex Guide V1

## 目的

这不是一份 review 结论补充，而是一份**指导下一步迭代的工程意见**。

目标是回答 4 个问题：

1. 项目当前处在什么状态
2. 下一阶段最值得投入的方向是什么
3. 应该按什么顺序推进
4. 每一阶段做完以后，怎么判断“是真的收口了”

文档基于 **2026-03-08** 当前仓库状态整理。

---

## 一句话判断

CTM 现在已经不是 demo，也不是只会跑 happy path 的原型。

它已经具备：

- 清晰的单二进制架构
- 已打通的 Chrome Extension → Native Messaging → daemon → CLI/TUI 主链路
- 比较扎实的测试基础
- 足够明确的领域边界（targets / tabs / groups / sessions / collections / bookmarks / search / workspace / sync）

但它还没有完全到“产品层已经收口、可以无顾虑继续横向加功能”的阶段。

**下一步最重要的，不是继续堆新 feature，而是把已有能力的底层语义、交互表面、索引/存储层、文档状态统一起来。**

换句话说：

- 核心系统已经成型
- 产品表面还没有完全成型
- 基础设施已经开始成为下一阶段的约束条件

---

## 当前状态总览

## 1. 已经比较完整的部分

### 1.1 核心架构

主链路已经闭环：

- `ctm` 单二进制承担 CLI / daemon / nm-shim
- Chrome Extension 通过 Native Messaging 接入
- daemon 通过 Unix socket 服务 CLI / TUI
- Hub 作为中心路由和状态协调点

这条链路现在不是概念设计，而是已经落到代码和测试里。

### 1.2 领域切分

当前代码结构和协议结构是清楚的：

- `internal/protocol` 负责消息模型
- `internal/client` 负责 daemon 连接与请求
- `internal/daemon` 负责动作路由、target 管理、持久对象 handler
- `internal/tui` 负责交互层
- `internal/bookmarks` / `internal/search` / `internal/sync` / `internal/workspace` 负责各领域逻辑

这意味着：

- 新增一个资源域并不需要重写架构
- 新增一个 action 的改动位置相对可预测
- 代码库已经具备“长期维护”的基本形态

### 1.3 测试基础

当前测试状态已经明显高于常见个人工具项目：

- `go vet ./...` 可通过
- `go build ./...` 可通过
- `go test -count=1 -race ./...` 可通过
- `go test -count=1 ./... -coverprofile=cover.out` 可通过

最近一次实测覆盖率：

- 总覆盖率：`87.4%`
- `internal/daemon`：`91.7%`
- `internal/tui`：`81.6%`
- `ctm/cmd`：`67.2%`

这说明项目当前的主要风险已经不是“完全没测试”，而是“某些新增交互路径或系统边界尚未完全测试到位”。

### 1.4 主功能骨架

下面这些能力已经不只是“建模”，而是基本可用：

- target 发现 / 选择 / default
- tabs list / open / close / activate / update
- groups list / create / update / delete
- session save / list / get / restore / delete
- collection create / list / get / add / restore / delete
- bookmarks tree / search / mirror / overlay / export
- cross-resource search
- workspace CRUD / switch
- sync status / repair
- TUI 多视图交互

从“系统骨架是否成立”这个角度看，项目已经跨过最危险的阶段。

---

## 2. 仍然不够完整的部分

### 2.1 产品表面不完全一致

底层能力和上层入口并不总是同步。

典型例子：

- daemon / extension 已支持 `bookmarks.create`、`bookmarks.update`、`bookmarks.remove`
- 但 CLI 目前只暴露 `tree/search/mirror/export`
- TUI 中有一部分 bookmarks 删除能力，但整体 bookmark 管理还没有形成完整一致的产品面

另一个典型例子：

- Workspaces 视图 help 文案里提示了 `n create new`、`D D delete`
- 但 TUI 正常按键分支里 `n` 只接到了 Sessions / Collections，没有真正接到 Workspace create

这类问题不会让架构崩，但会明显降低“产品完成度”的感知。

### 2.2 文档状态已经开始落后于实现

文档体系本身设计得很好，但有部分文档不再代表当前现实。

最明显的是：

- `18_CURRENT_PHASE.md` 仍停留在早期 phase 描述

当项目进入第二阶段以后，文档如果继续滞后，会产生两个问题：

1. 新开发时容易误判哪些是已完成、哪些是规划项
2. review 会开始花时间纠正文档而不是发现真实问题

### 2.3 搜索层仍是早期实现，不适合继续无限扩

现在的 `search.query` 是一个可用的实现，但不是可长期承载大规模数据的实现。

当前行为大致是：

- 扫 sessions 目录
- 扫 collections 目录
- 扫 workspaces 目录
- 读 bookmark mirror
- 对 tabs 向 extension 拉一次实时数据
- 汇总后排序截断

这在当前规模下是合理的，但存在天然上限：

- 数据量大时性能会明显下降
- 每次搜索成本偏高
- 搜索结果质量和元数据丰富度会被文件结构限制
- 后续做 saved search / smart collection / workspace search / action ranking 会越来越别扭

### 2.4 bookmark mirror 的多 target 语义还没收口

这是当前最明确、也最值得优先解决的真实技术债之一。

当前模型是：

- 写 mirror 时记录了 `TargetID`
- 读 mirror 时仍固定读单个 `mirror.json`

这意味着：

- 当前单实例使用基本没问题
- 但系统一旦真的进入多 target 常态使用，bookmark 数据语义就不再闭合

这是“当前能用，但下一阶段一定要处理”的典型问题。

### 2.5 sync 还是工程底座，不是成熟产品能力

当前 sync 更像：

- 本地 JSON 文件目录
- 与 cloud 目录之间的双向复制
- 基于 modtime 的 pending/conflict 判断
- local-wins repair

这套东西适合 Phase 1/2 的演进式实现，但还不足以承载更强的产品承诺。

现在不适合把它包装成“跨设备同步已经成熟”的能力。

### 2.6 分发和安装还不是真正的产品级完成

当前已经有：

- `.goreleaser.yml`
- `Makefile`
- `ctm install`
- Native Messaging manifest 写入
- LaunchAgent 写入

但它距离“用户拿到二进制就能完整装好”还有距离。

尤其是：

- extension 还不是 embed + 解压的一步式安装路径
- install 更像“宿主侧安装”，不是完整的产品安装器

所以它现在是“可工程使用”，还不是“完整交付体验”。

---

## 当前项目的正确定位

我建议把当前 CTM 定位成：

> 一个已经成型的、本地优先的、Chrome-first 的浏览器工作流系统内核  
> 而不是“已经全产品化完成的 browser workspace manager”

这个定位很重要，因为它会直接决定下一步的决策方式。

如果把它误判成“产品已经基本完成”，接下来就很容易发生：

- 继续加 power features
- 继续加浏览器支持
- 继续扩 workspace / sync / automation
- 但底层索引、存储语义、交互一致性、安装分发都还没收口

那样项目会很快变成“功能很多，但越来越难维护”。

---

## 下一步迭代的总原则

## 原则 1：从“做出功能”转向“统一系统”

前一阶段已经证明：这个项目可以把功能做出来。

下一阶段重点应该变成：

- 底层语义是否统一
- CLI / TUI / daemon / extension 是否一致
- 文档是否反映当前实现
- 新功能是否建立在可扩展底座上

## 原则 2：优先补基础设施，不优先扩能力面

短期内不建议优先投入：

- 新 power features
- 新浏览器适配
- 更复杂的 automation hooks
- 花哨但不改变系统约束的 TUI 美化

短期内建议优先投入：

- 搜索索引层
- per-target bookmark mirror 语义
- Workspace / Bookmarks / Saved Search 的表面收口
- 安装与文档收口

## 原则 3：每加一个入口，都要检查三层一致性

新增或补齐能力时，不要只看某一层是否实现。

至少同时检查：

1. daemon action 是否存在且 contract 清楚
2. CLI / TUI 是否至少有一个自然入口
3. 文档与 help 是否准确

现在项目里最常见的不完整感，恰恰来自这三层没有一起推进。

---

## 建议的下一步迭代主题

我建议把下一轮迭代定义为：

## Iteration Theme

**“收口现有能力，建立下一阶段可扩展底座”**

这轮不追求 feature count 继续上升，而追求：

- 系统更一致
- 表面更完整
- 基础层更可扩

---

## 推荐迭代顺序

下面是我建议的推进顺序。

顺序很重要，因为前面的工作会决定后面的成本。

## Phase A：先做“真实状态收口”

### 目标

把“代码当前有什么”与“文档、help、交互上说自己有什么”统一起来。

### 建议动作

1. 更新 `18_CURRENT_PHASE.md`
2. 对 `11_PLAN.md` / `13_ACCEPTANCE.md` 做一次 reality check
3. 做一张 capability matrix

建议用一张表列清：

- daemon 已实现
- CLI 已暴露
- TUI 已暴露
- Extension 已支持
- 测试是否覆盖
- 是否可视为当前 phase 已完成

### 为什么优先做这个

因为如果真实状态不清楚，后面的每次迭代都会浪费一部分精力在“重新发现现状”上。

### 验收标准

- 当前 phase 文档和代码状态一致
- 不再存在 help 写了但入口没接上的明显项
- 团队可以用文档快速回答“这个能力现在到底算已完成还是未完成”

---

## Phase B：收口产品表面

### 目标

把已经存在的底层能力真正贯通到 CLI / TUI。

### 应优先收口的面

#### 1. Workspace 表面闭环

当前 workspace 在 daemon 侧已经具备 CRUD + switch。

下一步应该补齐：

- TUI create / delete 的真实按键行为
- 必要时补 `workspace.get` / `workspace.update` 的交互入口
- help / status / toast 文案与实际行为保持一致

#### 2. Bookmark 管理表面闭环

当前 bookmarks 不是只读系统了，底层已经支持 create / update / remove。

建议明确选择一条路线：

- 要么正式把 bookmarks 写操作提升为当前 phase 能力，并补 CLI/TUI 入口
- 要么暂时下沉，不在交互层暴露，并在文档中明确“当前只支持 mirror + overlay”

不要继续维持“底层能写，上层像不能写”的模糊状态。

#### 3. Saved Search 表面闭环

当前 daemon 已支持 `search.saved.create/delete/list`，但 CLI 只暴露了 `list`。

建议至少补：

- CLI create
- CLI delete
- 必要时在 TUI / command palette 中给一个最小入口

### 为什么这一阶段重要

因为用户感知到的“完整性”，很大程度上不是来自底层模型，而是来自“同一能力能不能自然地走通”。

### 验收标准

- Workspace / Bookmarks / Saved Search 不再出现“底层支持、表层缺位”的主要断层
- TUI help 与真实行为一致
- CLI 不再明显漏掉已经实现的主能力

---

## Phase C：升级搜索和镜像底座

### 目标

把搜索系统从“可用的全量扫描实现”升级为“可以承接下一阶段功能扩展的底座”。

### 应优先做的两件事

#### 1. 引入本地索引层

不要再让 `search.query` 长期依赖全量扫盘。

建议最小方案：

- 每个资源域在写入时同步维护一个轻量 index
- index 可先做成 JSON，不必一开始就引入数据库
- bookmark mirror 在更新时同步产出扁平索引
- search.query 默认查 index，必要时再补实时源

最关键的是先建立“index 是一等对象”的概念。

#### 2. 把 bookmark mirror 改成 per-target 模型

建议直接升级成：

- `mirror_<targetId>.json`
- 或 target 子目录模型

同时明确：

- CLI/TUI 在多 target 下默认读哪个 mirror
- 没有 mirror 时是否自动拉取
- 搜索 bookmarks 时 target 选择如何生效

### 这一阶段完成后的收益

- 搜索成本下降
- 多 target 语义闭环
- 之后做 workspace search / saved search / smart collection 的成本显著下降

### 验收标准

- `search.query` 不再依赖每次全量读所有资源文件
- bookmarks mirror 读写都按 target 闭环
- 有 dedicated regression tests 覆盖 multi-target bookmark/search 语义

---

## Phase D：把重操作和状态迁移做稳

### 目标

处理那些当前“能用，但会在规模变大时开始影响体验”的路径。

### 建议优先项

#### 1. restore / workspace.switch 节流

当前 session restore / workspace switch 真正的压力主要来自浏览器一次性打开大量 tabs。

建议补：

- tab open throttle
- 大 restore 阈值提示
- 分批 restore 或后台 restore

#### 2. heavy job queue

建议为这些任务建立统一串行队列：

- bookmark mirror rebuild
- search index rebuild
- workspace switch
- 大 restore

这样能减少高峰期的资源竞争，也让后续状态提示更清晰。

#### 3. 操作结果语义补强

特别是 best-effort 类操作，建议逐步增加：

- warnings
- skipped count
- partial failure reason

这会比简单的 success / error 更适合后期产品化。

### 验收标准

- 大 restore 不会造成明显卡顿
- 重操作不会互相打架
- 用户能看懂一次操作到底是成功、部分成功、还是跳过

---

## Phase E：再来收 sync 和 distribution

### 目标

把“已经有底座”的 sync / install 推到更接近产品级。

### Sync 建议

当前不要着急加更多 sync feature，先补模型：

- 明确 sync object boundary
- 明确以文件为单位还是对象为单位冲突
- 引入 sync metadata version
- 明确 last-sync 基准而不只是依赖 modtime 差

等这些清楚以后，再谈：

- iCloud
- device awareness
- 更细粒度冲突处理

### Distribution / Install 建议

优先从“工程安装”变成“产品安装”：

- extension 资源 embed 到二进制
- `ctm install` 负责解压 extension、写 manifest、写 LaunchAgent
- `ctm install --check` 扩充为真正的 doctor-style 检查入口

### 验收标准

- install 成为真正的一步式主路径
- sync 从“目录复制引擎”升级为“明确语义的数据同步层”

---

## 不建议现在优先做的事

下面这些事情现在都不是最优先。

## 1. 不要优先扩浏览器矩阵

现在先把 Chrome-first 模型做稳，比扩 Brave / Edge / Arc 更值。

## 2. 不要优先加 Power Layer

像 automation hooks、复杂 tab sort、更多批处理命令，这些都应该排在底层索引和语义收口之后。

## 3. 不要急着上数据库

当前 JSON 持久化仍然是合理方案。

问题不在于“没数据库”，而在于“缺 index / 语义层 / 多 target 模型”。

只有当：

- 搜索索引复杂度继续上升
- 需要事务性更强的对象关系
- sync metadata 变复杂

再考虑 SQLite 一类方案才合理。

## 4. 不要把 TUI 视觉优化当主任务

当前 TUI 已经到了可以用、也有一定风格的阶段。

下一步主要缺的是：

- 行为一致性
- 帮助与实际对齐
- 交互路径覆盖

不是继续改配色或布局细节。

---

## 推荐的下一轮迭代拆分

如果你要把下一轮迭代拆成 2 个小里程碑，我建议这样拆。

## 里程碑 1：表面收口

主题：

**“让当前能力真的完整可用”**

包含：

- phase / acceptance / capability 文档 reality check
- Workspace TUI create/delete 接通
- Bookmark 管理能力做产品决策并统一入口
- Saved Search CLI create/delete 补齐
- help / keymap / command palette / toast 全量对齐

完成后应该达到：

- 产品表面不再出现明显假按钮 / 假提示 / 半暴露能力

## 里程碑 2：底层升级

主题：

**“为下一阶段扩展准备底座”**

包含：

- bookmark mirror per-target
- search index 第一版
- restore / switch 节流
- heavy job queue
- sync metadata 和 install 路径梳理

完成后应该达到：

- 再往上加 workspace/search/sync feature 时，不会明显被当前底座拖住

---

## 下一轮迭代的验收门槛

我建议下一轮不要只看“代码能跑”，而要加 4 类验收标准。

## 1. 真实状态验收

- 文档与代码状态一致
- capability matrix 可回答当前系统边界

## 2. 表面一致性验收

- help 里写出来的快捷键都真的可用
- CLI / TUI / daemon 的核心能力不再明显错位

## 3. 语义一致性验收

- multi-target bookmark mirror 闭环
- workspace / search / bookmarks 的 target 行为可解释

## 4. 扩展性验收

- 搜索不再依赖全量扫盘
- 大 restore / switch 不会让系统明显变卡
- 新增一个 resource action 的接入路径清晰、成本可控

---

## 作为 owner 的决策建议

如果你只想知道“下一步应该怎么管方向”，我建议是：

### 1. 不要把项目定义成“继续疯狂长功能”

现在最值钱的是收口，不是发散。

### 2. 把下一轮目标明确写成“系统统一”

这样 review、开发、验收都会更聚焦。

### 3. 每加一个功能，都要求同时回答 5 个问题

1. daemon contract 是什么
2. CLI 是否暴露
3. TUI 是否暴露
4. target / sync / search 语义是否清楚
5. 文档和测试是否同步更新

### 4. 把“文档真实度”当作产品质量的一部分

对这个项目来说，文档不是附属品。

现在代码规模和能力域已经足够多，如果文档失真，迭代成本会稳定上升。

---

## 最终建议

CTM 当前最正确的下一步，不是“再证明它还能做更多功能”，而是：

> 把已经做出来的能力，整理成一个真正一致、可持续扩展、可继续产品化的系统。

这轮如果做对了，后面无论是：

- 更强的 Workspace
- 更成熟的 Search
- 更可信的 Sync
- 更完整的 Distribution
- 更丰富的 Power Layer

都会明显更顺。

如果这轮跳过，直接继续加功能，项目短期会更热闹，但中期会开始变重、变散、变难维护。

所以我对下一步的核心建议只有一句：

**先收口，再扩张。**
