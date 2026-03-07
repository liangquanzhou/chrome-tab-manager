# CTM — Domain Model & Resource Lifecycle

## 1. Domain Overview

CTM 领域对象分三类：

- **Runtime Objects** — 浏览器当前在线状态
- **Library Objects** — CTM 自有长期资源
- **Sync Objects** — 跨设备同步与状态管理

## 2. Runtime Objects

### 2.1 Target

一个在线浏览器实例（Chrome profile / instance）。所有 runtime 操作的作用域边界。承载 windows / tabs / groups / bookmark source。

### 2.2 Window

某个 target 中的一个浏览器窗口。承载 tab 顺序和 group 上下文，是 session restore 的结构单元。

### 2.3 Tab

浏览器标签页。最基础的 runtime 资源，可被打开/关闭/激活/分组/固定/捕获进 session 或 collection。不是长期知识对象。

**生命周期**：出现 → 被浏览/操作 → 可能被 capture 进 session/collection/workspace → 关闭后价值可能被 library 保留。

### 2.4 Group

一组运行中的 tab。提供 runtime 组织结构。结构可被 capture 进 session。不等于 workspace。

**生命周期**：创建 → 承载 tabs → 可能被 capture 进 session → 消失后语义通过 session/workspace 留下。

### 2.5 BookmarkSource

浏览器原生书签树。Owner = Chrome / Google account。作为 BookmarkMirror 的输入来源。不是 CTM 最终真相源，但是 CTM 书签能力的重要上游。

## 3. Library Objects

### 3.1 Session

某个 target 在某一时刻的浏览器快照。含 windows / tabs / groups / target reference / metadata。是"时间点快照"，一般不持续编辑。

**生命周期**：`runtime snapshot → saved session → preview/restore → archive/delete`

### 3.2 Collection

用户手工整理的一组链接资源。可含 items / titles / urls / notes / tags。是"资源包"，可持续编辑。

**生命周期**：`create → curate → enrich → restore/export → archive/delete`

### 3.3 BookmarkMirror

Chrome 原生书签在 CTM 中的镜像视图。让书签能参与统一搜索、workspace 和 export。不是取代原生书签，是 CTM 对原生书签的可操作视图。

**生命周期**：`source sync → mirror refresh → search/browse/use`（随 source 更新，不独立删除）

### 3.4 BookmarkOverlay

CTM 对书签增加的本地增强层。可含 tags / notes / aliases / custom grouping / workspace relationships。Owner = CTM，sync target = iCloud。原生 bookmark 与本地 overlay 分开，Chrome/Google Sync 与 CTM library sync 互不污染。

**生命周期**：`bookmark selected → overlay added → search/workspace/sync → update/remove`

### 3.5 Workspace

围绕某个任务或主题组织的一组资源。可聚合 sessions / collections / bookmarks / tags / notes / saved searches / optional default target。长期中心对象，不是 collection 或 session 的别名。

**生命周期**：`create → attach resources → evolve → startup/resume → archive/delete`

- Session 回答"当时浏览器是什么状态"
- Collection 回答"我整理了哪些链接"
- Workspace 回答"这项工作由哪些资源组成"

### 3.6 Tag

跨资源分类标签。可附着到 bookmarks / collections / sessions / workspaces。提供统一分类和过滤能力。

### 3.7 Note

附着在资源上的文本说明。可附着到 bookmark / collection / session / workspace。把资源从"链接集合"提升成"可理解上下文"。

### 3.8 SavedSearch

可重复执行的查询定义。保存高频检索入口，可参与 workspace 和 smart collection。

## 4. Sync Objects

### 4.1 SyncAccount

一个同步来源或目标。如 Google-backed bookmark source 或 iCloud library sync target。标识不同同步体系。

### 4.2 SyncState

描述某个资源与某个同步目标之间关系的状态对象。表示已同步/冲突/待上传/待修复等。不混进核心业务对象内部，是附着在资源上的状态层。

### 4.3 Device

参与同步的客户端设备。帮助用户理解资源来源和冲突原因。

## 5. Ownership Model

| Category | Objects |
|----------|---------|
| Browser-owned | Target, Window, Tab, Group, BookmarkSource |
| CTM-owned | Session, Collection, BookmarkMirror, BookmarkOverlay, Workspace, Tag, Note, SavedSearch |
| Sync-owned state | SyncAccount, SyncState, Device |

## 6. Relationship Graph

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

## 7. Cross-Resource Lifecycle Transitions

CTM 最有价值的地方在于资源不孤立，能互相转化：

- Tab → Session / Collection
- Bookmark → Collection / Workspace
- Session → Workspace
- Collection → Workspace
- Search result → Action

## 8. Resource State Model

每个 CTM-owned 长期对象有三种终态：

- **Active** — 当前使用或可快速恢复
- **Archived** — 不常用但值得保留
- **Deleted** — 不再需要

| Resource | 偏快照 | 偏持续演化 |
|----------|--------|-----------|
| Tab / Group | yes | no |
| Session | yes | limited |
| Collection | partly | yes |
| BookmarkMirror | no | refreshed |
| BookmarkOverlay | no | yes |
| Workspace | no | strongly yes |

## 9. Required Cross-Cutting Properties

所有 CTM-owned 长期对象必须具备：

- Stable identity (UUID)
- Human-readable name
- Timestamps (createdAt, updatedAt)
- Searchability
- Exportability
- Syncability (id + timestamp 是 iCloud 同步的前提)

## 10. Object Roles in User Experience

| Category | Objects | 用户主要做什么 |
|----------|---------|---------------|
| Runtime-first | Target, Window, Tab, Group | 控制、浏览、实时切换 |
| Library-first | Session, Collection, BookmarkMirror, BookmarkOverlay | 保存、搜索、恢复、导出 |
| Context-first | Workspace, Tag, Note, SavedSearch | 组织、归纳、长期复用 |
| Infrastructure-first | SyncAccount, SyncState, Device | 理解同步状态和冲突 |

## 11. Expansion Rules

新增功能时必须先回答：

1. 它属于哪个对象？
2. 它改变哪个对象的角色？
3. 它的真相源在哪里？
4. 它是否需要参与搜索？
5. 它是否需要参与同步？
6. 它是否应该被 workspace 聚合？

回答不清楚 → 功能还没进入正确的领域模型。
