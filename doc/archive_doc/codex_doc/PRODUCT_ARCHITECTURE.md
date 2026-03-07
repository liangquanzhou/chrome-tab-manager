# CTM — Product Architecture

## 1. Product Definition

CTM 不是一个“tab manager”而已。

更准确的定义是：

**CTM 是一个 terminal-first 的 browser workspace manager。**

它同时处理三类事情：

1. **控制浏览器当前运行态**
   - targets
   - windows
   - tabs
   - groups

2. **维护用户自己的长期知识库**
   - sessions
   - collections
   - bookmarks mirror
   - tags / notes / metadata
   - workspaces

3. **把这些长期数据同步到云端**
   - Chrome / Google Sync 负责浏览器原生书签
   - CTM 自己负责 library 数据
   - iCloud 负责 CTM library 的云同步

这意味着：

- CTM 不只是“看当前开了哪些 tab”
- CTM 也不是“替代 Chrome Sync”
- CTM 的定位是把浏览器运行态、长期组织、跨设备同步统一到一个 terminal-first 产品里

---

## 2. Core Product Model

从产品角度，CTM 有四层。

### 2.1 Browser Runtime Layer

这一层处理在线浏览器状态。

核心对象：

- `Target`
  - 一个在线浏览器实例
  - 可以是不同 profile，也可以是未来不同 browser/channel
- `Window`
- `Tab`
- `Group`
- `BookmarkSource`
  - 来自 Chrome 的原生书签树

这一层的特点：

- 数据是“活的”
- 变化频繁
- 可以被 CTM 控制
- 不是 CTM 自己的最终持久化来源

### 2.2 Library Layer

这一层处理 CTM 自己拥有的数据。

核心对象：

- `Session`
  - 某一时刻的浏览器快照
- `Collection`
  - 用户手工整理的一组链接
- `BookmarkMirror`
  - Chrome 书签在 CTM 中的可搜索镜像
- `Tag`
- `Note`
- `Workspace`
  - 聚合多个资源的高层对象

这一层的特点：

- 数据是长期存在的
- 可以被搜索、组织、导出、同步
- 可以承载 Chrome 原生模型里没有的扩展能力

### 2.3 Sync Layer

这一层处理“什么跟谁同步”。

CTM 应该从一开始就采用双轨同步模型：

- **Google / Chrome Sync**
  - 负责 Chrome 原生书签的跨设备同步
  - CTM 不重造这部分轮子

- **iCloud Sync**
  - 负责 CTM 自己的数据同步
  - 包括 sessions、collections、workspace、tags、notes、bookmark overlay

也就是：

- `bookmarks source of truth` 在 Chrome / Google
- `library source of truth` 在 CTM
- `cloud sync for CTM-owned data` 在 iCloud

### 2.4 Interaction Layer

这一层是用户直接感知到的产品表面。

包括：

- CLI
- TUI
- command palette
- keymap / workflow / search

这一层不应该自己发明业务模型，而应该消费上面三层。

这样未来加：

- bookmark tree view
- workspace view
- search view
- sync status view

都不会推翻整体产品。

---

## 3. Source of Truth

为了保证扩展性，CTM 需要一开始就明确“谁是真相源”。

### 3.1 Live Browser State

这些数据的真相源在浏览器：

- tabs
- groups
- windows
- 当前活动状态
- Chrome 原生书签树

CTM 对它们做两件事：

- 控制
- 镜像

### 3.2 CTM-Owned Library State

这些数据的真相源在 CTM：

- sessions
- collections
- workspace 定义
- tags
- notes
- bookmark overlay
- 用户自定义组织关系

这些数据不能绑死在 Chrome Sync 上。

### 3.3 Cloud State

云端只承担复制与同步，不应该重新定义业务真相。

因此：

- Google Sync = 浏览器原生书签的上游同步体系
- iCloud = CTM library 的同步体系

---

## 4. Why Bookmarks Must Be First-Class

bookmarks 不能只是“以后再加一个 view”。

它们会影响：

- 数据模型
- 搜索模型
- 同步模型
- workspace 模型
- 交互模型

原因很简单：

- bookmarks 是树结构，不是平面列表
- bookmarks 属于长期知识，不是临时运行态
- bookmarks 需要被搜索、标记、归类、导出
- bookmarks 会同时与 Google Sync 和 CTM library 发生关系

所以在产品层面，CTM 应该把 bookmarks 定义成：

- **浏览器原生资源**
- **可被镜像进 CTM library 的核心对象**
- **未来 workspace 的组成部分**

---

## 5. Why iCloud Must Be First-Class

iCloud 也不能只当“以后顺手加个同步”。

只要你要：

- 多设备使用
- 跨机器继续同一个 workspace
- collections / sessions / tags / notes 不丢

那同步层就必须一开始进入架构。

这不代表一开始就把所有同步细节都写完。

但产品模型必须从第一天就默认：

- 每个持久对象有稳定 ID
- 每个持久对象有修改时间
- 每个持久对象能被同步
- 同步失败不影响本地使用
- 云同步只是增强，不是唯一存储

---

## 6. Workspace-Centered Future

如果 CTM 只停留在 tab/session 管理，它的上限会比较低。

更合理的长期中心对象是：

`Workspace`

Workspace 不是简单文件夹，而是一个工作上下文：

- 一组 live tabs
- 一组 saved sessions
- 一组 collections
- 一组 related bookmarks
- 用户的 tags / notes / labels
- 可选的默认 target / browser context

这样 CTM 的长期产品路径就很清楚：

- Runtime control 解决“现在在浏览器里做什么”
- Library 解决“长期保留什么”
- Workspace 解决“这些资源如何围绕一个任务组织起来”

---

## 7. Capability Map

为了避免以后产品边界变乱，能力应按下面几类组织。

### 7.1 Runtime Control

- list/open/close/activate tabs
- create/update/delete groups
- focus target/window
- live event stream
- realtime filtering

### 7.2 Capture & Restore

- save session
- restore session
- partial restore
- restore into current/new window
- crash recovery
- auto snapshot

### 7.3 Curation

- create collections
- add from tabs
- add raw URLs
- deduplicate
- batch open
- markdown export

### 7.4 Knowledge Layer

- bookmark mirror
- bookmark tree browsing
- cross-resource search
- tags
- notes
- saved filters
- resource relationships

### 7.5 Workspace Layer

- create workspace
- attach sessions/collections/bookmarks
- workspace search
- workspace restore/startup
- workspace-specific metadata

### 7.6 Sync Layer

- local-first persistence
- Google-backed bookmark source
- iCloud library sync
- conflict handling
- sync status
- sync repair / reindex

---

## 8. Recommended Top-Level Domain Objects

为了让未来扩展不推翻现有系统，建议从一开始就在产品模型上承认这些对象：

- `Target`
- `Window`
- `Tab`
- `Group`
- `BookmarkNode`
- `Session`
- `Collection`
- `Workspace`
- `Tag`
- `Note`
- `SavedSearch`
- `SyncAccount`
- `SyncState`

不是说一开始全部实现。

而是说：

- 文档
- 命名
- 数据边界
- UI 导航

都要默认这些对象将来会存在。

---

## 9. Product Navigation Model

如果按完整产品来设计，CTM 的最终 TUI/CLI 结构应该不是只有 tabs/groups/sessions。

更完整的导航模型应该包含：

- Targets
- Tabs
- Groups
- Sessions
- Collections
- Bookmarks
- Search
- Workspaces
- Sync

其中：

- `Tabs/Groups` 更偏实时操作
- `Sessions/Collections/Bookmarks` 更偏长期整理
- `Search/Workspaces` 更偏高层入口
- `Sync` 更偏状态与诊断

这会比单纯的“tab manager”更像一个完整产品。

---

## 10. Non-Goals

为了保持产品清晰，CTM 不应该变成这些东西：

- 一个通用浏览器
- 一个纯 bookmark manager
- 一个云盘客户端
- 一个笔记系统的完全替代品

CTM 的边界应保持为：

**browser-centered workspace management**

也就是：

- 一切围绕浏览器工作流
- 但不被浏览器当前运行态绑死

---

## 11. Product Expansion Opportunities

基于这套架构，后面最值得做的扩展有这些。

### 11.1 High-Value Extensions

- **Bookmarks as first-class citizens**
  - 书签树浏览
  - 书签搜索
  - 书签导出
  - 书签 overlay（tag/note/alias）

- **Cross-resource search**
  - 一次搜索 tabs / sessions / collections / bookmarks / workspace

- **Workspace startup**
  - 一键进入某个工作上下文
  - 自动恢复相关 session / collection / bookmark set

- **Partial restore**
  - 只恢复一部分 tabs / groups / collection items

- **Duplicate detection**
  - 发现重复 tab / bookmark / collection item

- **Markdown export**
  - session / collection / workspace 导出成 markdown

### 11.2 Organization Extensions

- tags
- notes
- aliases
- saved searches
- domain-based grouping
- smart collections
- workspace templates

### 11.3 Sync Extensions

- iCloud sync status view
- conflict resolution UI
- repair / rebuild local mirror
- device awareness
- selective sync

### 11.4 Power-User Extensions

- automation hooks
- batch commands
- import/export pipelines
- browser diagnostics
- pinned/unpinned bulk actions
- tab sorting
- tab suspend/discard

---

## 12. Final Product Positioning

一句话总结：

**CTM = terminal-first browser workspace manager**

它不是只管理 tabs，也不是只同步 bookmarks。

它应该同时具备：

- 浏览器实时控制能力
- 长期知识整理能力
- 云同步能力
- 工作区组织能力

而这四件事里：

- bookmarks 应该从第一天就是第一类资源
- iCloud 应该从第一天就是第一类同步目标
- workspace 应该从第一天就是未来中心对象

只要这个产品定义不变，底层实现可以逐步演进，但整体方向不会跑偏。
