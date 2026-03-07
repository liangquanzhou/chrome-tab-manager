# CTM — Decision Log

## 1. Purpose

这份文档记录 CTM 已经明确做出的**关键产品决策**。

它的作用是：

- 防止后续 agent 重复争论已经定过的方向
- 让产品世界观保持连续
- 为未来决策提供上下文

本日志应持续追加，不应频繁重写历史结论。

---

## 2. Decision Format

每条决策包含：

- `Decision`
- `Status`
- `Why`
- `Implication`

---

## 3. Decisions

## D-001 — Product Definition

### Decision

CTM 的产品定义是：

**terminal-first browser workspace manager**

### Status

Accepted

### Why

它不只是 tab manager，也不只是 session saver 或 bookmark viewer。

### Implication

所有后续设计必须同时考虑：

- runtime
- library
- bookmarks
- search
- workspace
- sync
- power features

---

## D-002 — Product Is Not Split into v1/v2 at the Definition Level

### Decision

产品定义不再按 `v1/v2` 切碎，而是一次性按完整产品来建模。

### Status

Accepted

### Why

bookmarks、workspace、sync 这些能力如果被推迟到“以后再说”，会导致前面的模型做窄。

### Implication

即使实现顺序有先后，这些能力也必须从第一天进入架构和领域模型。

---

## D-003 — Runtime and Library Must Coexist

### Decision

CTM 必须同时包含：

- 浏览器实时控制能力
- 长期资源管理能力

### Status

Accepted

### Why

只有 runtime 没有 library，会退化成浏览器控制台。  
只有 library 没有 runtime，会失去对当前浏览工作流的掌控。

### Implication

Tabs / Groups 和 Sessions / Collections 必须都属于产品本体。

---

## D-004 — Bookmarks Are First-Class

### Decision

bookmarks 是第一类资源，不是附属功能。

### Status

Accepted

### Why

bookmarks 会影响搜索、workspace、同步和长期知识体系。

### Implication

产品中必须长期保留：

- Bookmarks top-level area
- bookmark mirror
- bookmark overlay

---

## D-005 — Dual-Track Sync

### Decision

CTM 采用双轨同步模型：

- Chrome / Google Sync 负责原生书签
- CTM + iCloud 负责 CTM-owned library

### Status

Accepted

### Why

原生书签与 CTM 自定义资源属于不同数据域，不能绑死在同一个同步体系里。

### Implication

不能把 sessions / collections / workspaces 直接塞进 Google Sync。  
也不能让 CTM 试图重造原生书签同步。

---

## D-006 — Local-First Is Non-Negotiable

### Decision

CTM 必须是 local-first 产品。

### Status

Accepted

### Why

terminal 工具必须在离线、弱网、同步失败时仍然可用。

### Implication

云同步是增强，不是前提。  
同步失败不应阻断主要工作流。

---

## D-007 — Workspace Is the Long-Term Center

### Decision

workspace 是长期中心对象，不是 session 或 collection 的别名。

### Status

Accepted

### Why

用户长期管理的是任务上下文，而不是单一资源。

### Implication

workspace 必须能聚合：

- sessions
- collections
- bookmarks
- notes / tags / saved searches

---

## D-008 — Search Is a Product Center

### Decision

search 是跨资源统一入口，不只是某个 view 的过滤框。

### Status

Accepted

### Why

完整产品必须允许用户在不先猜资源类型的情况下找到资源。

### Implication

search 必须覆盖：

- tabs
- sessions
- collections
- bookmarks
- workspaces

---

## D-009 — Top-Level Product Areas

### Decision

CTM 的顶层区域固定为：

- Targets
- Tabs
- Groups
- Sessions
- Collections
- Bookmarks
- Search
- Workspaces
- Sync

### Status

Accepted

### Why

这 9 个区域共同覆盖 runtime、library、global 三层结构。

### Implication

如果未来某次设计把 bookmarks/search/workspaces/sync 从顶层拿掉，说明产品正在退化。

---

## D-010 — Command Surfaces Have Different Roles

### Decision

CLI、TUI、Command Palette 不是重复产品，而是不同动作表面。

### Status

Accepted

### Why

不同动作适合不同交互面。

### Implication

- CLI = explicit control / automation
- TUI = exploration / organization
- Palette = fast global shortcut layer

---

## D-011 — Build Order Follows Product Skeleton, Not Feature Count

### Decision

建设顺序按能力骨架，而不是按功能零碎堆叠。

### Status

Accepted

### Why

先立骨架，后填实现，才能减少返工。

### Implication

建设顺序固定为：

1. Runtime
2. Library
3. Bookmarks
4. Sync
5. Search
6. Workspace
7. Interaction
8. Power

---

## D-012 — Bookmark Model Has Three Layers

### Decision

bookmarks 按三层建模：

- BookmarkSource
- BookmarkMirror
- BookmarkOverlay

### Status

Accepted

### Why

原生书签、可搜索镜像、CTM 增强层是三种不同职责。

### Implication

不能把原生 bookmark 树与 CTM tag/note/alias 混成一个对象。

---

## D-013 — Sync Must Be Visible

### Decision

sync 不是后台黑盒，必须有独立产品入口。

### Status

Accepted

### Why

如果用户看不到 sync state，就不会信任跨设备连续性。

### Implication

`Sync` 必须是顶层区域，不可藏进“设置”或“诊断”。

---

## D-014 — Product Must Support the Full Extension Set

### Decision

下面这些能力都属于产品本体，不是可有可无的附加模块：

- bookmarks
- search
- workspace
- sync
- power-user automation

### Status

Accepted

### Why

用户已经明确要求完整产品，而不是只做一个 tab/session 工具。

### Implication

后续设计与实现都不应再把这些能力降级成“以后可选”。

---

## D-015 — CTM Should Remain Browser-Centered

### Decision

CTM 始终是 browser-centered workspace product，不扩展成泛笔记工具或泛知识库。

### Status

Accepted

### Why

产品要有清晰边界。

### Implication

虽然会有 notes、tags、workspace，但它们都应围绕浏览器资源服务，而不是发展成独立笔记系统。

---

## 4. Ongoing Rule

以后新增高层产品决策时：

- 直接追加到本文件
- 不要覆盖已接受决策
- 如果确实推翻旧决策，应新增一条 superseding decision，并标记旧条目被替代

---

## 5. Final Decision Statement

这份日志的作用，是让 CTM 后续不管由谁来实现，都始终沿着同一条产品主线推进：

**control the browser, build a long-term resource library, unify search, organize workspaces, and sync CTM-owned knowledge across devices.**
