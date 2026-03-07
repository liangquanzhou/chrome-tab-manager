# CTM — Domain Model

## 1. Purpose

这份文档定义 CTM 的**领域模型**。

它回答的是：

- CTM 到底有哪些核心对象
- 这些对象之间是什么关系
- 哪些对象属于浏览器，哪些对象属于 CTM 自己
- 哪些对象要参与同步

这份文档的作用不是约束底层代码结构，而是防止产品在实现过程中退化成“很多命令拼在一起”。

---

## 2. Domain Overview

CTM 的领域模型分成三类对象：

1. **Runtime Objects**
   - 浏览器当前在线状态

2. **Library Objects**
   - CTM 自己长期维护的资源

3. **Sync Objects**
   - 用于跨设备同步和状态管理的对象

---

## 3. Runtime Objects

## 3.1 Target

### Definition

一个在线浏览器实例。

### Represents

- 一个运行中的 Chrome profile / instance
- 一个用户当前可以操作的目标环境

### Responsibilities

- 作为所有 runtime 操作的作用域
- 承载 windows / tabs / groups / bookmark source

### Key ideas

- Target 是 runtime 的顶层边界
- 所有 tab/group/window 的 live identity 都只在某个 target 内有效

---

## 3.2 Window

### Definition

一个浏览器窗口。

### Represents

- 某个 target 中的一个运行中窗口

### Responsibilities

- 承载 tab 顺序
- 承载 group 所在上下文
- 作为 session restore 的结构单元

---

## 3.3 Tab

### Definition

一个浏览器标签页。

### Represents

- 某个目标 URL 的当前 live instance

### Responsibilities

- 被打开、关闭、激活、分组、固定
- 被捕获为 session / collection / workspace 的组成部分

### Key ideas

- Tab 是最基础的 runtime 资源
- 但它本身不是长期知识对象

---

## 3.4 Group

### Definition

一组运行中的 tab。

### Represents

- 浏览器当前的 tab group 结构

### Responsibilities

- 提供运行态组织结构
- 作为 session / workspace 恢复的重要语义单元

### Key ideas

- Group 是 runtime 组织，不等于 workspace

---

## 3.5 BookmarkSource

### Definition

浏览器原生书签树。

### Represents

- 由 Chrome 维护、由 Google 账号同步的书签体系

### Responsibilities

- 提供原始 bookmark 数据
- 作为 BookmarkMirror 的输入来源

### Key ideas

- 这是浏览器原生数据，不是 CTM 自己的最终真相源
- 但它是 CTM 书签能力的重要上游

---

## 4. Library Objects

## 4.1 Session

### Definition

某个 target 在某一时刻的浏览器快照。

### Contains

- windows
- tabs
- groups
- target reference
- metadata

### Responsibilities

- 保存工作现场
- 恢复工作现场
- 参与 workspace 组织

### Key ideas

- Session 是“快照”
- 它记录的是某一刻浏览器运行态的结构化表示

---

## 4.2 Collection

### Definition

用户手工整理的一组链接资源。

### Contains

- items
- titles
- urls
- optional notes / tags

### Responsibilities

- 收集链接
- 复用链接集合
- 恢复全部或部分内容

### Key ideas

- Collection 不等于 session
- Session 更像状态快照
- Collection 更像人工整理的资源包

---

## 4.3 BookmarkMirror

### Definition

Chrome 原生书签在 CTM 中的镜像视图。

### Contains

- bookmark tree snapshot
- searchable fields
- local overlay references

### Responsibilities

- 为搜索和组织提供统一入口
- 让书签能参与 workspace / search / export

### Key ideas

- Mirror 不是取代原生书签
- Mirror 是 CTM 对原生书签的可操作视图

---

## 4.4 BookmarkOverlay

### Definition

CTM 对书签增加的本地增强层。

### Contains

- tags
- notes
- aliases
- custom grouping
- local metadata

### Responsibilities

- 让书签具备原生 Chrome 不提供的能力

### Key ideas

- 原生 bookmark 与本地 overlay 必须分开
- 这样 Chrome / Google Sync 与 CTM library sync 才不会互相污染

---

## 4.5 Workspace

### Definition

围绕某个任务或主题组织的一组资源。

### Can contain

- sessions
- collections
- bookmarks
- tags
- notes
- optional default target context

### Responsibilities

- 成为用户长期工作的核心单位
- 把不同类型资源组织成一个可恢复、可搜索、可同步的上下文

### Key ideas

- Workspace 是长期中心对象
- 不是“另一个 collection”
- 也不是“另一个 session”

---

## 4.6 Tag

### Definition

跨资源的分类标签。

### Can apply to

- bookmarks
- collections
- sessions
- workspaces
- future resources

### Responsibilities

- 提供统一分类和过滤能力

---

## 4.7 Note

### Definition

附着在资源上的文本说明。

### Can apply to

- bookmark
- collection
- session
- workspace

### Responsibilities

- 提供人类语义
- 把资源从“链接集合”提升成“可理解上下文”

---

## 4.8 SavedSearch

### Definition

可重复执行的查询定义。

### Responsibilities

- 保存用户常用检索入口
- 支持 smart collection / workspace workflows

---

## 5. Sync Objects

## 5.1 SyncAccount

### Definition

一个同步来源或同步目标。

### Examples

- Google-backed bookmark source
- iCloud library sync target

### Responsibilities

- 标识不同同步体系
- 为资源同步状态提供上下文

---

## 5.2 SyncState

### Definition

描述某个资源与某个同步目标之间关系的状态对象。

### Responsibilities

- 表示是否已同步
- 表示是否冲突
- 表示是否待上传 / 待合并 / 待修复

### Key ideas

- 同步状态不应混进核心业务对象内部定义
- 应该是附着在资源上的状态层

---

## 5.3 Device

### Definition

参与同步的一个客户端设备。

### Responsibilities

- 帮助用户理解资源来自哪台机器
- 支持同步诊断与冲突分析

---

## 6. Ownership Model

为了避免产品边界混乱，CTM 必须明确对象所有权。

### Browser-owned

- Target
- Window
- Tab
- Group
- BookmarkSource

### CTM-owned

- Session
- Collection
- BookmarkMirror
- BookmarkOverlay
- Workspace
- Tag
- Note
- SavedSearch

### Sync-owned state

- SyncAccount
- SyncState
- Device

---

## 7. Source of Truth Matrix

## 7.1 Browser Runtime Truth

真相源在浏览器：

- live tabs
- live groups
- live windows
- active state
- native bookmarks

## 7.2 CTM Library Truth

真相源在 CTM：

- sessions
- collections
- workspace definitions
- tags
- notes
- bookmark overlay
- saved searches

## 7.3 Cloud Truth

云端不定义业务对象，只承担同步：

- Google Sync 同步浏览器原生书签
- iCloud 同步 CTM 自己的 library 数据

---

## 8. Relationship Graph

可以用下面这张高层图理解对象关系：

```text
Target
├─ Window
│  └─ Tab
│     └─ Group
└─ BookmarkSource

Session  ───── captures ─────> Window / Tab / Group state
Collection ─── contains ─────> Link items
BookmarkMirror ─ mirrors ────> BookmarkSource
BookmarkOverlay ─ enriches ──> BookmarkMirror / BookmarkSource

Workspace
├─ Session
├─ Collection
├─ BookmarkMirror / BookmarkOverlay
├─ Tag
└─ Note

SavedSearch ─ queries ───────> Session / Collection / Bookmark / Workspace
SyncState ─ attaches to ─────> CTM-owned resources
```

---

## 9. Required Cross-Cutting Properties

如果 CTM 要支持完整产品能力，那么长期对象都应具备这些共同属性：

- stable identity
- human-readable name
- timestamps
- searchability
- exportability
- syncability

这不要求所有对象的内部实现都完全一样。

但要求：

- 产品层面默认它们都能参与长期管理
- 不是一次性的临时结构

---

## 10. Object Roles in User Experience

不同对象在用户体验中的角色不同。

### Runtime-first objects

- Target
- Window
- Tab
- Group

用户主要用它们来：

- 控制
- 浏览
- 实时切换

### Library-first objects

- Session
- Collection
- BookmarkMirror
- BookmarkOverlay

用户主要用它们来：

- 保存
- 搜索
- 恢复
- 导出

### Context-first objects

- Workspace
- Tag
- Note
- SavedSearch

用户主要用它们来：

- 组织
- 归纳
- 长期复用

### Infrastructure-first objects

- SyncAccount
- SyncState
- Device

用户主要通过它们理解：

- 是否同步成功
- 资源来自哪里
- 为什么发生冲突

---

## 11. Expansion Rules

以后新增功能时，必须先回答：

1. 它属于哪个对象？
2. 它改变哪个对象的角色？
3. 它的真相源在哪里？
4. 它是否需要参与搜索？
5. 它是否需要参与同步？
6. 它是否应该被 workspace 聚合？

如果回答不清楚，说明功能还没进入正确的领域模型。

---

## 12. Final Domain Statement

CTM 的领域模型不是围绕“tab”展开，而是围绕“browser-centered work resources”展开。

其中：

- runtime objects 负责当前状态
- library objects 负责长期资产
- workspace 负责高层组织
- sync objects 负责跨设备连续性

只要这个领域模型稳定，后面的实现方式可以演化，但产品不会退化成一个只会开关 tab 的工具。
