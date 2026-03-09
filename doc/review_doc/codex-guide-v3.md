# Codex Guide V3

## 目的

这份文档是在 `codex-guide-v2.md` 基础上的一次**顶层复核版**。

它不再重复“这个项目已经具备哪些能力”，而是直接回答三个更重要的问题：

1. 上一轮指出的关键问题，是不是真的都改了
2. 现在项目处在什么阶段，应该怎样定义当前状态
3. 下一轮迭代，应该继续扩功能，还是先把系统和产品边界彻底收口

---

## 一句话结论

**代码层的大头，基本已经改掉了；但项目层、规格层、发布层，还不能说“都改完了”。**

更准确地说：

- 之前最明显的两个代码级 Medium 已基本关闭
- CTM 现在确实进入了 consolidation，而不是继续救火
- 但当前还没有进入“可宣称全面收口”的状态

原因不是底层 handler 还缺一大片，而是：

> **项目的“真实现状”还没有被顶层文档和支持边界稳定地定义下来。**

这意味着，当前阶段最重要的工作不再是“再补几个 action”，而是：

> **把当前已经形成的系统，收口成一个有清晰真相来源、明确支持边界、可持续发布的产品层。**

---

## 这次复核确认了什么

### 1. 上一轮两个具体代码问题，已经基本修掉

#### 1.1 TUI help / keymap / handler 的大面积失真，已经明显收口

这块是这轮最重要的正向变化之一。

当前 `internal/tui/keymap.go` 里各 view 的 help，已经比之前保守和真实得多：

- Tabs 不再宣称 `G` / `a` 这类未接通动作
- Groups 只声明 `Enter expand/collapse`
- Targets 只声明 `Enter activate`、`d set default`
- Bookmarks / Workspaces / History 的 help 也与当前 handler 更接近

同时，`internal/tui/app.go` 的 `handleEnter()` 已经明确接上：

- `ViewGroups`
- `ViewTargets`
- `ViewBookmarks`
- `ViewHistory`
- `ViewWorkspaces`
- `ViewSessions`
- `ViewCollections`

这说明项目已经从“交互面成片超前于实现”回到了“多数 surface 基本可信”的状态。

#### 1.2 安装路径不再被 `--extension-id` 卡死

`cmd/install.go` 现在已经明确区分了两种安装模式：

- `ctm install`
  只安装 LaunchAgent
- `ctm install --extension-id=...`
  安装 LaunchAgent + Native Messaging manifest

并且 `ctm install --check` 在不传 `--extension-id` 时也能正常工作。

这意味着上一轮那个“GoReleaser / Homebrew 路径被 install 参数契约卡住”的问题，已经不再成立。

---

## 当前项目应该怎样重新定义

我对当前 CTM 的判断，比 `guide v2` 更明确：

> **CTM 现在已经是一个系统内核成型、用户表面基本可用、但产品规格尚未完全收口的本地浏览器工作流系统。**

这个判断有三层意思：

### 1. 它已经不是原型

当前仓库已经具备：

- daemon / hub 中心结构
- extension / native messaging 桥接
- CLI / TUI 双入口
- sessions / collections / bookmarks / workspace / search / sync / history 等多个资源域
- 稳定的自动化测试基础

我本轮实跑：

- `go vet ./...`
- `go build ./...`
- `go test -count=1 ./...`
- `go test -count=1 ./... -coverprofile=cover.out`

都通过。当前这次全量覆盖率跑出来是 **78.1%**。  
补充看包级覆盖率，本轮得到：

- `ctm/cmd`: **55.8%**
- `ctm/internal/daemon`: **89.6%**
- `ctm/internal/tui`: **64.1%**

这不是“功能脚本集”，也不是“只够演示的实验项目”。

### 2. 它也还不是完全产品化完成

现在真正没收口的，不是“能不能做”，而是“哪些已经正式支持、哪些只是部分支持、哪些只是底层预留”。

这件事如果不被定义清楚，项目会进入一种很典型的第二阶段风险：

- 代码能力越来越多
- 用户入口越来越多
- 文档越来越多
- 但“当前正式承诺的产品表面”反而越来越模糊

### 3. 它已经到了应该用“治理”而不是“堆功能”来推进的阶段

换句话说，CTM 下一轮的主问题不是 capability growth，而是 **truth alignment**。

也就是：

- 代码现实
- capability matrix
- current phase
- help / keymap
- install / release narrative

必须有一个统一、稳定、可验证的版本。

---

## 现在还没改完的，不是哪里

如果只看代码层，这一轮确实比前几轮收得多。

但如果从顶层看，当前仍然有三个关键缺口没有完全闭环。

### 1. 顶层“现状真相”文档仍然滞后，而且互相打架

这是我这轮最明确保留的判断。

`doc/18_CURRENT_PHASE.md` 仍然包含明显过期信息，例如：

- 还写着 `Groups view` 的 `Enter handler` 缺失
- 还写着 `daemon 30.6%` 需提升

这些都已经与当前代码现实不一致。

更重要的是，`doc/19_CAPABILITY_MATRIX.md` 虽然比之前更接近当前代码，但它自己的汇总区也没有完全收口：

- Summary 里写 `S=32 / P=16 / R=9`
- 但我按表格 action 行重新计数，实际是 **62** 个 action row：
  - `S=31`
  - `P=19`
  - `R=12`
- `Coverage Gaps Summary` 里也还保留了 “Groups Enter 无 ViewGroups case” 这种已经失效的描述

这件事的影响，不是“文档有点旧”这么简单。

因为当前项目已经进入 consolidation，`18_CURRENT_PHASE.md` 和 `19_CAPABILITY_MATRIX.md` 这两份文档，实际上已经开始承担：

- 阶段判断
- 支持边界
- 下一轮优先级
- 对外叙述依据

如果这两份文档不准，整个项目的“当前状态”就会被持续误报。

### 2. “支持等级”已经提出了，但还没有真正变成项目纪律

`S / P / R` 这个框架本身是对的，而且应该保留。

问题在于，项目还没有完全进入“按支持等级推进”的工作方式。

目前仍然能看到这样的残留倾向：

- 很多底层 action 已存在
- 一部分在 CLI 有入口
- 一部分在 TUI 有入口
- 一部分只有测试或 only-daemon handler
- 但这些能力还没有被稳定地整理成“正式支持面”和“预留能力面”

这会带来一个长期风险：

> **系统越强，用户越难知道当前真正被支持的路径是什么。**

下一轮如果继续横向加 action，而不先稳定 S/P/R 的语义，matrix 会继续膨胀，收口成本会继续上升。

### 3. 发布路径现在能跑，但还没到“发布即产品承诺”的级别

`install` 的主要契约问题已经修掉了，这很好。

但“能跑”不等于“发布层已经完成”。

当前发布面仍然需要回答更严格的问题：

- 安装后的 extension pairing 是否被稳定验证
- `install --check` 是否要验证 manifest 内容，而不只是文件存在
- release artifact、Homebrew、LaunchAgent、NM manifest、extension 加载说明，是否构成同一个可信路径
- 哪些是“内部可用”，哪些是“对外正式支持”，是否在文档里有一致说法

也就是说，发布层已经有骨架，但还没有完全成为第一公民。

---

## 所以，到底是不是“都改了”

我的判断是：

**没有。**

但必须分清楚“没改完”的性质。

### 已经改掉的部分

- TUI help / handler 的大面积错配
- `install` 被 `--extension-id` 契约卡死的问题
- 许多前几轮 review 提到的底层 handler / 测试覆盖缺口

### 还没彻底改完的部分

- `Current Phase` 文档没有追上代码现实
- `Capability Matrix` 的 summary / gap summary 没完全对齐自己的 action 表
- “支持等级”还没完全变成发布、文档、迭代决策的统一依据
- 项目仍然需要把“收口轮”从代码修补，推进到规格收口与发布收口

所以更准确的说法不是：

> “所有问题都修完了”

而应该是：

> “代码层最主要的问题已经修到位，项目层最主要的问题变成了定义和维护当前真相。” 

---

## 下一轮应该怎么做

我建议下一轮不要再把重点放在“再扩几条能力线”，而是按下面顺序推进。

## Priority 1: 统一真相源

把 `doc/19_CAPABILITY_MATRIX.md` 真正变成**唯一当前能力真相表**，但前提是先修准。

要做的事：

- 逐行校正 action table、Summary、Coverage Gaps Summary 三处内容
- 把 `18_CURRENT_PHASE.md` 改成“阶段说明 + 本轮目标 + 指向 matrix”，而不是再重复写一遍易过期细节
- 停止在 phase 文档里手写易漂移的覆盖率数字，改成“见最新 test artifact / latest review”

验收标准：

- `18_CURRENT_PHASE.md` 不再包含与代码冲突的动作描述
- `19_CAPABILITY_MATRIX.md` 的表格行数、S/P/R 汇总、gap summary 三者一致
- 以后 review 时，能直接以 matrix 作为 current-state 依据

## Priority 2: 把 S / P / R 变成工程纪律

不是所有 P 都应该补成 S，也不是所有 R 都值得继续保留。

下一轮应该做的是“做决策”，而不是“把所有空位都补满”。

建议：

- 只挑最高价值的 P 做晋级
- 其余 P 明确保持 partial，不伪装成完整能力
- 对低价值 R 做去留决策，必要时降为内部 API，不再进入产品叙述

推荐先看的 P：

- Search 全系
- Downloads
- Bookmarks create/export
- Targets clearDefault / label
- Sync repair

验收标准：

- 每个 P action 都有明确去向：升 S / 保 P / 降内部
- 发布文档和 help 不再暗示超出当前支持等级的用户路径

## Priority 3: 收口发布路径

当前 `install` 已经从“有契约冲突”进到“可继续产品化”的阶段。

下一步应该做的是把发布层从“能跑”升级成“可信”。

建议：

- 加一条 release smoke path
  - fresh install
  - `ctm install`
  - `ctm install --extension-id=...`
  - `ctm install --check`
  - extension connect
  - `tabs.list` / `targets.list`
- 明确区分“内部运行链路已验证”和“正式对外支持路径”
- 如果 Homebrew 继续保留，就把它当正式入口维护，而不是半成品入口

验收标准：

- 新机器上的安装和连通路径可复现
- 发布文档与实际命令行为一致
- 失败时能快速定位在 LaunchAgent、NM manifest、extension、daemon 哪一层

## Priority 4: 回到真正的平台问题

当前最值得做的平台型工作，仍然是之前 `guide v2` 里那几项，而不是新增资源域：

- 搜索索引化
- bookmark mirror 的 target-aware 语义
- 大操作的节流/队列化
- sync 的语义边界明确化

这些问题不会像 handler 缺失那样立即显眼，但它们决定了 CTM 下一阶段能否继续稳定扩展。

---

## 对当前阶段的最终判断

我建议把当前 CTM 的阶段定义成下面这句：

> **Post-Phase-8 consolidation，不再以“功能已实现”为完成标准，而以“支持边界已定义、表面一致、发布可信”为完成标准。**

这比简单说“Phase 都做完了”更接近当前现实。

因为当前真正最需要解决的，不是功能存在性，而是：

- 当前能力的正式边界
- 文档和代码之间的单一真相
- 项目到底准备承诺什么

---

## 最后的建议

如果只问一句“下一轮最该做什么”，我的答案是：

> **不要继续优先扩功能，先把 current truth 定义清楚。**

更具体一点，就是：

1. 先修正并冻结 `18_CURRENT_PHASE.md` 和 `19_CAPABILITY_MATRIX.md`
2. 再基于 S/P/R 决定哪些能力要正式支持
3. 再把 install / release 路径收口成可信主路径
4. 最后才进入下一轮新的平台扩展

`guide v2` 的核心结论是：**下一轮最重要的不是扩展系统，而是定义系统。**

这次 `guide v3` 的更新版结论是：

> **代码层已经基本收口；接下来真正决定项目质量的，是能不能把“当前真相”和“正式支持边界”定义成一个稳定版本。**
