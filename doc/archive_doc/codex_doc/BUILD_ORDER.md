# CTM — Build Order

## 1. Purpose

这份文档定义 CTM 在“完整产品一次性设计”的前提下，**最不容易返工的建设顺序**。

它回答的不是：

- 先写哪个文件
- 先做哪个命令
- 先接哪个库

而是：

- 先把哪几个产品骨架立起来
- 哪些能力必须最早进入世界观
- 哪些能力可以后做实现，但现在必须留位置

---

## 2. Guiding Principle

CTM 不是线性堆功能的工具。

它应该按“能力骨架”建设，而不是按“一个个命令”建设。

正确顺序应该优先保证：

1. 产品世界观不返工
2. 核心对象不返工
3. 同步边界不返工
4. 导航结构不返工
5. 交互模型不返工

所以建设顺序必须围绕这几个问题：

- 浏览器实时态怎么定义
- 长期资源怎么定义
- 书签怎么进入核心模型
- 云同步怎么进入核心模型
- 工作区怎么成为长期中心

---

## 3. Recommended Build Order

## Stage 1 — Runtime Foundation

先把“浏览器当前运行态”这一层立起来。

### 目标

建立 CTM 对在线浏览器世界的完整理解。

### 要覆盖的能力

- Targets
- Windows
- Tabs
- Groups
- Runtime events
- Target selection
- Default target

### 为什么必须最先做

因为 CTM 所有能力最终都要落回浏览器运行态：

- session 是从 runtime capture 来的
- collection 常常从 tabs 派生
- workspace startup 最终会驱动 runtime restore
- search 也需要 runtime 资源参与

### 完成标志

当你能稳定回答这些问题时，Stage 1 才算立住：

- 现在有哪些 target 在线
- 每个 target 里有哪些 tabs/groups/windows
- 当前状态变化能不能实时反映
- 多 target 下作用域是否清晰

---

## Stage 2 — Library Foundation

在 runtime 之后，立刻把“长期资源层”建起来。

### 目标

把运行态沉淀成长期可管理资源。

### 要覆盖的能力

- Sessions
- Collections
- Capture
- Restore
- Preview
- Delete
- Export

### 为什么必须第二个做

因为 CTM 的价值不只是“控制当下”，而是“沉淀下来并可复用”。

如果没有 library，CTM 只是一个实时控制工具。

### 完成标志

当你能稳定回答这些问题时，Stage 2 才算立住：

- 什么是 session，什么是 collection
- 哪些资源是快照，哪些资源是人工整理
- capture / restore / export 的对象边界是否清楚

---

## Stage 3 — Bookmarks as First-Class Resource

第三步就要把 bookmarks 提升成第一类资源。

### 目标

让 bookmarks 从“浏览器附属能力”变成 CTM 产品核心的一部分。

### 要覆盖的能力

- Bookmark source
- Bookmark mirror
- Bookmark tree
- Bookmark search
- Bookmark overlay
- Bookmark export

### 为什么必须这么早

因为 bookmarks 会影响：

- 数据模型
- 搜索模型
- workspace 模型
- 同步模型

如果你把它拖到后面，后面做的一切搜索、workspace、sync 都会返工。

### 完成标志

当你能稳定回答这些问题时，Stage 3 才算立住：

- 原生书签和 CTM 书签增强层是什么关系
- bookmarks 如何参与 search
- bookmarks 如何参与 workspace

---

## Stage 4 — Sync Foundation

第四步就该把同步层作为产品骨架建进去。

### 目标

让 CTM 从单机工具变成长期可持续使用的产品。

### 要覆盖的能力

- Local-first persistence
- Google-backed bookmark source
- iCloud sync for CTM-owned library
- Sync state
- Sync status
- Conflict model
- Repair / rebuild model

### 为什么必须早做

因为 sync 不是“以后再接一层云”。

它会反过来影响：

- 对象 identity
- metadata
- 修改时间
- 删除语义
- 冲突处理

这些如果不提前进入架构，后面 library 和 workspace 都要重做。

### 完成标志

当你能稳定回答这些问题时，Stage 4 才算立住：

- 哪些数据跟 Google 同步
- 哪些数据跟 iCloud 同步
- 哪些数据的真相源在 CTM 本地
- sync 失败时产品是否仍然可用

---

## Stage 5 — Search Layer

在 runtime/library/bookmarks/sync 都有位置之后，再把搜索作为横向能力建起来。

### 目标

让 CTM 成为“统一入口”，不是一堆独立 view。

### 要覆盖的能力

- Cross-resource search
- Search tabs
- Search sessions
- Search collections
- Search bookmarks
- Saved search
- Smart collection

### 为什么放在这里

因为 search 本质上依赖对象模型已经存在。

如果资源域没建好，search 只能退化成每个列表各搜各的。

### 完成标志

当你能稳定回答这些问题时，Stage 5 才算立住：

- 搜索是否跨资源
- bookmarks / sessions / collections 是否统一进入搜索入口
- search 结果是否可以直接转成行动

---

## Stage 6 — Workspace Layer

第六步才把 workspace 真正立成中心对象。

### 目标

让 CTM 从资源管理系统升级为工作上下文系统。

### 要覆盖的能力

- Workspace create
- Workspace attach resources
- Workspace startup
- Workspace restore
- Workspace template
- Workspace search
- Workspace metadata

### 为什么不是第一步

因为 workspace 不是基础资源，它是上层聚合结构。

它必须站在：

- runtime
- library
- bookmarks
- sync
- search

这些基础都成立之后，才会稳定。

### 完成标志

当你能稳定回答这些问题时，Stage 6 才算立住：

- workspace 究竟聚合哪些资源
- workspace 与 session/collection/bookmark 的边界是否清楚
- workspace startup 是否是自然的用户路径

---

## Stage 7 — Interaction System

第七步才是把 CLI/TUI 做成统一、稳定、可扩展的交互表面。

### 目标

让复杂能力以稳定、可学、可测试的方式暴露给用户。

### 要覆盖的能力

- CLI navigation
- TUI navigation
- Search entrypoints
- Keymap
- Command palette
- Help system
- Feedback system
- Diagnostics surface

### 为什么放在这里

因为 Interaction layer 应该消费已有的产品模型，而不是倒逼产品模型。

### 完成标志

当你能稳定回答这些问题时，Stage 7 才算立住：

- 用户是否知道自己在哪里
- 用户能否从任意资源域快速执行下一步动作
- 搜索、workspace、sync 是否都已经有自然入口

---

## Stage 8 — Power Layer

最后再把 power-user 能力系统化。

### 目标

让 CTM 变成 daily driver，而不是“能用但不够爽”的工具。

### 要覆盖的能力

- Batch actions
- Pin/unpin bulk actions
- Sort
- Deduplicate
- Suspend/discard
- Domain grouping
- Markdown export
- Automation hooks
- Diagnostics / doctor
- Repair workflows

### 为什么放在最后

因为 power feature 的前提是：

- 核心对象已稳定
- 交互系统已稳定
- 搜索和 workspace 已存在

否则这些高级功能只会继续把产品做散。

---

## 4. The Real Backbone

如果把完整 CTM 压缩成真正需要先立住的 5 个骨架，就是：

1. Runtime
2. Library
3. Bookmarks
4. Sync
5. Workspace + Search

Interaction 和 Power 是建立在这五个骨架上的产品表面。

也就是说：

- **先定领域，再定交互**
- **先定对象，再定动作**
- **先定同步边界，再定云功能**

---

## 5. What Must Be Considered from Day One

即使不是第一阶段完整实现，下面这些也必须从第一天进入架构：

- Bookmarks
- iCloud sync
- Search
- Workspace
- Stable identities
- Metadata
- Conflict awareness

因为这些能力一旦后加，最容易引发全局返工。

---

## 6. What Can Be Implemented Later Without Breaking Direction

下面这些可以后做实现，但现在不必先打满：

- automation hooks
- tab sorting
- duplicate detection
- domain grouping
- markdown export
- crash recovery
- partial restore UX
- smart collections
- workspace templates

它们重要，但不会像 bookmarks / sync / workspace 那样改变产品世界观。

---

## 7. Anti-Patterns

如果按错顺序，最容易掉进这些坑：

- 先做大量 TUI 细节，再发现对象模型不够宽
- 先做 sessions/collections，再把 bookmarks 当外挂
- 先做单机数据，再后来硬接 iCloud
- 把 search 做成每个 view 各自过滤
- 把 workspace 推到很后面，最后只能成为 collection 的别名

---

## 8. Final Recommendation

如果你要的是“完整产品一次性定义好，再交给 agents 去实现”，最稳的建设顺序是：

1. Runtime foundation
2. Library foundation
3. Bookmarks as first-class
4. Sync foundation
5. Search layer
6. Workspace layer
7. Interaction system
8. Power layer

一句话总结：

**先把产品的世界观和骨架立稳，再让实现逐层填满；不要先把交互和命令做满，再回头补领域模型。**
