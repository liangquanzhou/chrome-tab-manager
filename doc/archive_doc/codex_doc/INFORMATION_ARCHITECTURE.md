# CTM — Information Architecture

## 1. Purpose

这份文档定义 CTM 的**信息架构**。

它回答的是：

- 完整产品有哪些顶层区域
- 这些区域之间是什么层级关系
- 用户从哪里进入、看到什么、再去哪里
- runtime、library、bookmarks、search、workspace、sync 这些能力如何被组织成一个统一产品

这份文档不讨论技术实现，只讨论产品结构。

---

## 2. IA Principles

CTM 的信息架构必须满足这几个原则：

1. **按资源域组织，而不是按技术模块组织**
2. **搜索和 workspace 必须是顶层入口**
3. **runtime 与 library 必须同时可见**
4. **sync 必须可见，不可藏起来**
5. **用户始终能回答：我现在在看哪类资源**

---

## 3. Top-Level Product Map

CTM 的完整顶层信息架构应包含 9 个一级区域：

1. `Targets`
2. `Tabs`
3. `Groups`
4. `Sessions`
5. `Collections`
6. `Bookmarks`
7. `Search`
8. `Workspaces`
9. `Sync`

这 9 个区域共同组成完整产品。

---

## 4. IA by Product Layer

## 4.1 Runtime Layer

负责当前浏览器运行态。

### Areas

- Targets
- Tabs
- Groups

### Questions this layer answers

- 我现在连接到哪个浏览器上下文？
- 当前有哪些 live tabs / groups / windows？
- 我可以立刻控制什么？

---

## 4.2 Library Layer

负责长期资源。

### Areas

- Sessions
- Collections
- Bookmarks

### Questions this layer answers

- 哪些东西值得长期保存？
- 我之前整理过什么？
- 我能恢复什么？
- 我已经积累了哪些长期知识资源？

---

## 4.3 Global Layer

负责跨资源入口和高层组织。

### Areas

- Search
- Workspaces
- Sync

### Questions this layer answers

- 我想找某个东西，它在哪？
- 我现在在处理哪个任务上下文？
- 我的资源现在是否安全、是否同步正常？

---

## 5. Structural Hierarchy

从信息架构角度，CTM 可以理解成三层：

### Level 1 — Global Entry

- Search
- Workspaces

### Level 2 — Resource Domains

- Tabs
- Groups
- Sessions
- Collections
- Bookmarks

### Level 3 — Context / Infrastructure

- Targets
- Sync

这三层的关系是：

- `Global Entry` 负责回答“我要做什么”
- `Resource Domains` 负责回答“我要处理哪类资源”
- `Context / Infrastructure` 负责回答“这件事发生在哪、是否可靠”

---

## 6. Area Definitions

## 6.1 Targets

### Function

浏览器上下文入口。

### Contains

- online targets
- default target
- target labels
- target status

### Primary actions

- select
- set default
- rename / label

### IA role

- 安全作用域入口

---

## 6.2 Tabs

### Function

当前 live browser content 入口。

### Contains

- live tabs
- filters
- selection state
- current runtime actions

### Primary actions

- activate
- close
- group
- add to collection
- capture into session

### IA role

- 实时工作台

---

## 6.3 Groups

### Function

运行态结构化组织入口。

### Contains

- live groups
- grouped tabs
- group structure

### Primary actions

- expand
- edit
- dissolve
- attach tabs

### IA role

- 运行态组织层

---

## 6.4 Sessions

### Function

浏览器快照资产入口。

### Contains

- session summaries
- session previews
- restoreable structures

### Primary actions

- save
- preview
- restore
- export
- archive / delete

### IA role

- 时间点快照资产层

---

## 6.5 Collections

### Function

人工 curated 资源入口。

### Contains

- collections
- collection items
- notes / tags

### Primary actions

- create
- add items
- remove items
- restore all / partial
- export

### IA role

- 资源包层

---

## 6.6 Bookmarks

### Function

原生书签与 CTM 书签增强层入口。

### Contains

- bookmark tree
- bookmark mirror
- bookmark overlay
- bookmark metadata

### Primary actions

- browse
- search
- open
- tag / note
- attach to workspace

### IA role

- 长期知识资源层

---

## 6.7 Search

### Function

跨资源统一入口。

### Contains

- global queries
- mixed resource results
- saved searches
- smart collection entrypoints

### Primary actions

- search
- filter by resource type
- jump to result
- act on result
- save search

### IA role

- 横向主入口

---

## 6.8 Workspaces

### Function

任务上下文入口。

### Contains

- workspaces
- attached sessions
- attached collections
- attached bookmarks
- tags / notes / saved searches

### Primary actions

- enter workspace
- startup / resume
- attach resources
- search within
- archive

### IA role

- 长期产品中心层

---

## 6.9 Sync

### Function

同步状态与可靠性入口。

### Contains

- sync health
- last sync
- device context
- failed/conflicted resources

### Primary actions

- inspect
- retry
- repair
- rebuild

### IA role

- 基础设施可见性层

---

## 7. Cross-Area Relationships

这些区域在信息架构上不是孤立的。

### Runtime → Library

- Tabs / Groups 可以流向 Sessions / Collections

### Bookmarks → Workspace

- Bookmarks 可以进入 Workspace

### Search → Everything

- Search 可以进入任何资源域

### Sync → Library / Workspace / Bookmarks Overlay

- Sync 主要面向 CTM-owned resources

### Workspaces ↔ Resource Domains

- Workspace 聚合 Sessions / Collections / Bookmarks

---

## 8. Primary Navigation Routes

CTM 信息架构应天然支持这些主要跳转路径：

### Route A

Targets → Tabs → Sessions

### Route B

Tabs → Collections

### Route C

Bookmarks → Workspaces

### Route D

Search → Tabs / Sessions / Collections / Bookmarks / Workspaces

### Route E

Workspaces → Startup → Tabs / Groups

### Route F

Sync → Workspaces / Sessions / Collections

---

## 9. Entry Point Strategy

不同用户应能从不同入口进入同一个完整产品。

### Runtime-first users

最常从：

- Targets
- Tabs

进入

### Library-first users

最常从：

- Sessions
- Collections
- Bookmarks

进入

### Goal-first users

最常从：

- Search
- Workspaces

进入

### Reliability-first users

最常从：

- Sync

进入

---

## 10. TUI IA Implication

TUI 不应只是“几个列表页”。

它的顶层导航结构应能承载：

- Runtime areas
- Library areas
- Global areas

也就是说，TUI 的未来 IA 至少应自然容纳：

- Targets
- Tabs
- Groups
- Sessions
- Collections
- Bookmarks
- Search
- Workspaces
- Sync

---

## 11. CLI IA Implication

CLI 也应映射同样的信息架构。

最终命名空间应接近：

- `ctm targets`
- `ctm tabs`
- `ctm groups`
- `ctm sessions`
- `ctm collections`
- `ctm bookmarks`
- `ctm search`
- `ctm workspaces`
- `ctm sync`

这保证 CLI 和 TUI 共享同一套产品世界观。

---

## 12. IA Red Flags

如果未来出现这些迹象，说明信息架构开始退化：

- 顶层入口只剩 tabs/groups/sessions/collections
- bookmarks 被放到二级页面
- search 不是顶层入口
- workspace 不是顶层入口
- sync 被藏到“设置/诊断”下面

---

## 13. Final IA Statement

CTM 的信息架构应始终围绕这三个中心展开：

- **当前浏览器状态**
- **长期资源知识库**
- **工作区与跨设备连续性**

所有顶层区域都应服务于这三个中心，而不是互相割裂。
