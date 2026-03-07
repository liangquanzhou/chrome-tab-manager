# CTM — Resource Lifecycle

## 1. Purpose

这份文档定义 CTM 核心资源的**生命周期模型**。

它回答的是：

- 一个资源从哪里来
- 什么时候被保存
- 什么时候被组织
- 什么时候被同步
- 什么时候被恢复
- 什么时候被归档或删除

这份文档的目标是把 CTM 从“很多静态对象”变成“有生命周期的完整系统”。

---

## 2. Lifecycle Philosophy

CTM 的资源不是静态文件。

每一类资源都应被理解为：

1. **被发现**
2. **被捕获**
3. **被组织**
4. **被增强**
5. **被同步**
6. **被再次使用**
7. **被归档或删除**

---

## 3. Runtime Resource Lifecycle

## 3.1 Tab Lifecycle

### Flow

1. tab 出现在某个 target/window 中
2. 用户浏览、激活、关闭、分组、固定
3. tab 可能被 capture 进：
   - session
   - collection
   - workspace
4. tab 本身可以消失，但其价值可能被 library 保留下来

### Meaning

tab 是 runtime 资源，不是最终长期资产。

---

## 3.2 Group Lifecycle

### Flow

1. group 在运行时创建
2. group 承载一组相关 tabs
3. group 结构可能被 capture 进 session
4. group 在运行态消失后，其组织语义仍可能通过 session/workspace 留下

### Meaning

group 是 runtime 组织单元。

---

## 3.3 Window Lifecycle

### Flow

1. window 作为 tab 容器存在
2. 在 session capture 中成为结构单元
3. 在 restore 中重新出现

### Meaning

window 的长期价值主要在 capture / restore 语义里。

---

## 4. Library Resource Lifecycle

## 4.1 Session Lifecycle

### Creation

session 由 runtime capture 产生。

### Evolution

session 一般不是持续修改型对象，而是“按时间点保存的新版本”。

### Reuse

session 可被：

- preview
- restore
- attach to workspace
- export

### End state

session 可被：

- 保留
- 归档
- 删除

### Lifecycle summary

`runtime snapshot -> saved session -> preview/restore -> archive/delete`

---

## 4.2 Collection Lifecycle

### Creation

collection 可由：

- 手工创建
- 从 tabs 添加
- 从 bookmarks/search 结果中提取

### Evolution

collection 是可持续编辑型对象。

它会经历：

- add items
- remove items
- reorder / group mentally
- attach notes / tags

### Reuse

collection 可被：

- partial restore
- full restore
- export
- attach to workspace

### End state

collection 可被：

- 长期保留
- 合并进 workspace
- 删除

### Lifecycle summary

`create -> curate -> enrich -> restore/export -> archive/delete`

---

## 4.3 BookmarkMirror Lifecycle

### Creation

由原生 bookmark source 产生镜像。

### Evolution

mirror 会随着浏览器原生书签变化而更新。

### Reuse

mirror 可被：

- browse
- search
- attach to workspace
- export

### End state

mirror 本身不作为终极业务对象被“删除”，它随 source 更新。

### Lifecycle summary

`source sync -> mirror refresh -> search/browse/use`

---

## 4.4 BookmarkOverlay Lifecycle

### Creation

overlay 由用户对书签添加：

- tag
- note
- alias
- relationship

### Evolution

overlay 是持续编辑型对象。

### Reuse

overlay 可被：

- search
- workspace attach
- export
- cross-device sync

### End state

overlay 可被修改、移除、同步、冲突处理。

### Lifecycle summary

`bookmark selected -> overlay added -> search/workspace/sync -> update/remove`

---

## 4.5 Workspace Lifecycle

### Creation

workspace 由用户围绕某个任务创建。

### Evolution

workspace 是最持续演化的长期对象。

它会经历：

- attach sessions
- attach collections
- attach bookmarks
- add notes/tags
- update structure over time

### Reuse

workspace 可被：

- startup
- search
- sync
- resume

### End state

workspace 可被：

- archive
- retire
- delete

### Lifecycle summary

`create -> attach resources -> evolve -> startup/resume -> archive/delete`

---

## 5. Sync Lifecycle

## 5.1 CTM-Owned Resource Sync Lifecycle

对 session、collection、workspace、bookmark overlay 等资源，生命周期中应天然包含同步阶段。

### Flow

1. 本地创建或修改
2. 标记为待同步
3. 进入 syncing
4. 成功后变为 synced
5. 如失败则进入 failed / conflicted
6. 用户可继续本地使用
7. 之后再次同步或修复

### Lifecycle summary

`local change -> pending sync -> synced/conflicted -> repair/resume`

---

## 5.2 Native Bookmark Sync Lifecycle

原生书签同步不由 CTM 主导，但 CTM 需要参与其感知和利用。

### Flow

1. 用户或 Chrome 修改原生书签
2. Chrome / Google Sync 传播该变化
3. CTM 刷新 bookmark mirror
4. overlay 继续附着在相关书签上

### Lifecycle summary

`chrome bookmark change -> google sync -> CTM mirror refresh`

---

## 6. Cross-Resource Lifecycle Transitions

CTM 最有价值的地方，在于资源不是孤立的，而是能互相转化。

### Important transitions

- `Tab -> Session`
- `Tab -> Collection`
- `Bookmark -> Collection`
- `Bookmark -> Workspace`
- `Session -> Workspace`
- `Collection -> Workspace`
- `Search result -> Action`

这些转化路径越顺畅，CTM 的整体系统感越强。

---

## 7. Archive and Deletion

不是所有资源都应该直接删除。

从产品角度要承认三种终态：

1. **Active**
2. **Archived**
3. **Deleted**

### Active

当前还在使用或可快速恢复。

### Archived

不常用，但值得保留。

### Deleted

不再需要。

这对 workspace、session、collection 尤其重要。

---

## 8. Lifecycle by Resource Type

| Resource | More snapshot-like | More evolving |
|----------|--------------------|---------------|
| Tab | yes | no |
| Group | yes | no |
| Session | yes | limited |
| Collection | partly | yes |
| BookmarkMirror | no | refreshed |
| BookmarkOverlay | no | yes |
| Workspace | no | strongly yes |

这张表很重要，因为不同资源的 UI 和 sync 模式不应完全一样。

---

## 9. Lifecycle Red Flags

如果未来出现这些倾向，说明生命周期模型被做坏了：

- session 被当作长期反复编辑对象
- collection 不能从 tabs/bookmarks/search 自然生成
- workspace 不能随着资源长期演化
- overlay 不能独立同步
- sync 状态不进入资源生命周期

---

## 10. Final Lifecycle Statement

CTM 的资源生命周期应始终体现这条主线：

**discover -> capture -> organize -> enrich -> sync -> reuse -> archive/delete**

只有这样，CTM 才是一个完整的 browser workspace system，而不是一组分散的资源列表。
