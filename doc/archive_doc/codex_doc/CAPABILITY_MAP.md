# CTM — Capability Map

## 1. Purpose

这份文档定义 CTM 的**完整产品能力地图**。

目标不是回答“先做哪个技术模块”，而是回答：

- 这个产品最终需要覆盖哪些能力面
- 每个能力属于哪一层
- 哪些能力是核心，不是附属
- 以后新增功能应该挂到哪里，避免产品越做越散

CTM 的完整定义是：

**CTM = terminal-first browser workspace manager**

它同时覆盖：

- 浏览器运行态控制
- 长期资源整理
- 书签管理
- 跨资源搜索
- 工作区组织
- 云同步
- 高级自动化

---

## 2. Top-Level Capability Areas

CTM 的完整产品能力分为 7 个一级能力域：

1. `Runtime`
2. `Library`
3. `Bookmarks`
4. `Search`
5. `Workspace`
6. `Sync`
7. `Power`

这 7 个能力域都属于产品本体。

---

## 3. Capability Tree

## 3.1 Runtime

Runtime 负责“当前浏览器正在发生什么”。

### Core capabilities

- Target discovery
- Target selection
- Default target
- Multiple online targets
- Window awareness
- Tab list / open / close / activate
- Tab pin / unpin
- Group list / create / update / dissolve
- Live browser events
- Runtime filtering

### User value

- 在 terminal 里直接控制浏览器
- 实时看见浏览器状态变化
- 在多 target 环境中安全操作

### Must support

- 多窗口
- 多 group
- 多 target
- 实时更新
- 断线重连

---

## 3.2 Library

Library 负责“什么值得长期保存和再次使用”。

### Core capabilities

- Session save
- Session preview
- Session restore
- Session delete
- Collection create
- Collection add/remove items
- Collection restore
- Partial restore
- Export as markdown / link list
- Duplicate detection
- Auto snapshot
- Crash recovery

### User value

- 把临时浏览状态变成长期资产
- 对链接和工作现场进行收集与恢复
- 把“浏览器当前状态”提升成“可复用资源”

### Must support

- 命名资源
- 可预览
- 可部分恢复
- 可导出
- 可删除

---

## 3.3 Bookmarks

Bookmarks 负责“原生书签资源 + CTM 对书签的增强”。

### Core capabilities

- Bookmark tree browse
- Bookmark search
- Bookmark mirror
- Bookmark import from Chrome source
- Bookmark export
- Bookmark tagging
- Bookmark notes
- Bookmark aliases
- Bookmark metadata overlay

### User value

- 不只是读取 Chrome 书签
- 而是把书签纳入 CTM 的长期知识体系

### Product rule

- Chrome / Google Sync 是原生书签的同步来源
- CTM 负责书签增强层，而不是替代 Chrome 书签系统

---

## 3.4 Search

Search 负责“跨资源统一检索”。

### Core capabilities

- Search tabs
- Search groups
- Search sessions
- Search collections
- Search bookmarks
- Search workspaces
- Search by title / url / host / tag / note
- Saved searches
- Smart collections

### User value

- 把“我记得它大概在哪里”变成“我能立刻找到它”
- 把 CTM 从管理工具提升成检索入口

### Product rule

- Search 必须是跨资源能力，不是每个 view 自己的孤立搜索框

---

## 3.5 Workspace

Workspace 负责“围绕任务组织资源”。

### Core capabilities

- Workspace create
- Workspace rename/delete
- Workspace attach session
- Workspace attach collection
- Workspace attach bookmarks
- Workspace attach notes/tags
- Workspace startup
- Workspace templates
- Workspace restore
- Workspace-level search

### User value

- 用户管理的不再只是 tab 或 session
- 而是完整的工作上下文

### Product rule

- Workspace 是长期中心对象
- Session / Collection / Bookmark 都是 workspace 的组成部分

---

## 3.6 Sync

Sync 负责“资源如何跨设备保持一致”。

### Core capabilities

- Local-first persistence
- Google-backed bookmark source
- iCloud sync for CTM-owned data
- Sync status
- Conflict handling
- Selective sync
- Sync repair / rebuild
- Device awareness

### User value

- 换设备还能继续同一个工作体系
- 不依赖单机状态
- 书签和自定义资源各走各的正确同步路径

### Product rule

- Bookmark source sync 与 CTM library sync 分开建模
- 不能把所有数据都绑到 Google Sync
- 也不能把原生书签硬搬成 CTM 自己唯一真相源

---

## 3.7 Power

Power 负责“高频用户和自动化能力”。

### Core capabilities

- Batch operations
- Bulk pin / unpin
- Bulk group actions
- Sort tabs
- Deduplicate
- Suspend / discard tabs
- Domain grouping
- Markdown export
- Automation hooks
- Diagnostics / doctor
- Repair commands

### User value

- 降低重复操作成本
- 让 CTM 真正成为 daily driver

### Product rule

- 这些能力不是“炫技功能”
- 它们是 power-user adoption 的关键

---

## 4. Capability by Product Layer

为了保证扩展性，每个能力都应落在明确的层里。

### 4.1 Browser Runtime Layer

属于这一层的能力：

- targets
- windows
- tabs
- groups
- bookmark source
- browser events

### 4.2 Library Layer

属于这一层的能力：

- sessions
- collections
- bookmark mirror
- tags
- notes
- exports
- restore strategies

### 4.3 Workspace Layer

属于这一层的能力：

- workspace objects
- workspace startup
- templates
- cross-resource organization

### 4.4 Sync Layer

属于这一层的能力：

- iCloud sync
- sync status
- conflict handling
- repair / reconcile

### 4.5 Interaction Layer

属于这一层的能力：

- CLI
- TUI
- command palette
- keymaps
- help
- search entrypoints

---

## 5. What Must Be First-Class from Day One

下面这些能力即使不在第一周全部做完，也必须从第一天就按一级能力来设计。

### Must be first-class

- Bookmarks
- Search
- Workspace
- Sync

原因：

- 这些能力会反过来影响数据模型
- 会影响导航结构
- 会影响长期产品定位
- 如果一开始不当一级能力，后面一定返工

---

## 6. Capability Relationships

这些能力之间不是平行孤岛，而是有明确关系。

### Runtime ↔ Library

- runtime 提供实时数据
- library 负责把有价值的状态沉淀下来

### Bookmarks ↔ Search

- bookmarks 是搜索的重要资源域
- bookmark overlay 会扩展搜索维度

### Library ↔ Workspace

- workspace 聚合 library 资源
- workspace 不是替代 library，而是上层组织结构

### Library ↔ Sync

- CTM-owned library 必须能独立同步
- sync 不能侵入业务定义，但必须从一开始存在

### Runtime ↔ Workspace

- workspace startup 会驱动 runtime restore
- runtime live state 也可能被吸收进 workspace capture

---

## 7. Product Navigation Implication

如果 CTM 的能力地图是完整的，那么最终交互入口不应该只有：

- tabs
- groups
- sessions
- collections

更完整的导航应包含：

- Targets
- Tabs
- Groups
- Sessions
- Collections
- Bookmarks
- Search
- Workspaces
- Sync

这不是要求一开始就把所有界面写完。

而是要求：

- 导航结构
- 命名方式
- 顶层世界观

从现在开始就为这些能力保留位置。

---

## 8. Priority of Strategic Expansion

如果按产品价值排序，最应该尽快形成完整闭环的是：

### Priority 1

- Bookmarks
- Search
- Sync
- Workspace

### Priority 2

- Partial restore
- Auto snapshot
- Crash recovery
- Duplicate detection
- Markdown export

### Priority 3

- Batch pin / unpin
- Domain grouping
- Sort / suspend
- Automation hooks
- Diagnostics

---

## 9. Red Flags

如果未来出现下面这些倾向，说明产品又在退化回“tab 工具”：

- 只继续加 tabs/groups 操作，不扩 library
- 把 bookmarks 当作单独附属功能
- 把 search 限制为单 view 过滤
- 不做 workspace
- 不做 sync status / conflict handling
- 把 power features 永远放进“以后再说”

---

## 10. Final Capability Statement

CTM 的完整产品能力不是：

- tab manager
- session saver
- bookmark viewer

而是这三者再加上：

- unified search
- workspace organization
- cloud sync
- power-user automation

一句话总结：

**CTM should fully support runtime control, long-term curation, bookmark intelligence, unified search, workspace orchestration, cloud sync, and power-user automation.**
