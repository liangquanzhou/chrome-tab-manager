# CTM — Workspace Model

## 1. Purpose

这份文档定义 CTM 的**workspace 模型**。

它回答的是：

- workspace 到底是什么
- workspace 和 session / collection / bookmark 的区别是什么
- workspace 为什么是长期中心对象
- workspace 在产品中承担什么作用

---

## 2. Workspace Definition

Workspace 是：

**围绕某个任务、项目或主题组织起来的一组浏览器相关资源。**

它不是：

- 单一 session
- 单一 collection
- 单一书签文件夹

它是这些资源的上层组织单位。

---

## 3. Why Workspace Exists

如果没有 workspace，CTM 只能做到：

- 控制当前 tab
- 保存某个快照
- 整理某组链接

但用户真正长期管理的是：

- 某个项目
- 某个研究主题
- 某个任务上下文

workspace 的存在，就是把这些分散资源变成一个可进入、可搜索、可恢复的整体。

---

## 4. What a Workspace Can Contain

workspace 可以聚合：

- sessions
- collections
- bookmarks
- bookmark overlays
- tags
- notes
- saved searches
- optional target preference

也就是说：

workspace 是“资源容器”，但不只是容器，它还是“任务上下文”。

---

## 5. Workspace vs Session

### Session

- 是某一时刻的浏览器快照
- 偏时间点
- 偏恢复现场

### Workspace

- 是某个任务的长期组织单位
- 偏主题 / 项目
- 偏持续演化

一句话：

- session 回答“当时浏览器是什么状态”
- workspace 回答“这项工作由哪些资源组成”

---

## 6. Workspace vs Collection

### Collection

- 是手工整理的一组链接
- 更像资源包

### Workspace

- 是更高层的工作上下文
- 可以包含多个 collections

一句话：

- collection 是资源集合
- workspace 是组织这些资源集合的上层单位

---

## 7. Workspace vs Bookmarks

### Bookmarks

- 代表长期链接资源
- 可以属于多个上下文

### Workspace

- 代表具体任务上下文
- 书签只是其组成部分之一

一句话：

- 书签是材料
- workspace 是项目

---

## 8. Workspace Responsibilities

workspace 在产品中至少承担这几个责任：

### 8.1 Organization

把不同类型资源挂到同一任务上下文下。

### 8.2 Entry Point

用户可以从 workspace 进入整个工作系统，而不是从某个 tab 开始。

### 8.3 Startup

workspace 应支持启动或恢复相关资源。

### 8.4 Search Scope

workspace 应该成为一个天然搜索范围。

### 8.5 Long-Term Continuity

workspace 应参与同步，成为跨设备连续工作的核心对象。

---

## 9. Workspace Lifecycle

从产品角度，一个 workspace 的生命周期至少包括：

1. create
2. name
3. attach resources
4. evolve over time
5. search within
6. startup / restore
7. archive / delete

这意味着：

workspace 不是一次性快照，而是长期存在的上下文。

---

## 10. Workspace Startup

workspace 真正有价值的时刻，是用户重新进入一个任务时。

因此 workspace 应自然支持：

- 打开相关 session
- 打开相关 collection
- 打开相关 bookmark subset
- 恢复特定资源组合

也就是：

workspace startup = 重新进入工作上下文

---

## 11. Workspace Templates

当 workspace 模型稳定后，模板会自然出现。

### Template value

- 为重复性工作流提供快速入口
- 让“开工”变成可复用动作

### Example

- research template
- weekly review template
- project onboarding template

---

## 12. Workspace Search

workspace 不只是被搜索到，也应该成为搜索范围本身。

### Means

- 搜某个 workspace 内全部资源
- 搜某个 workspace 内的 bookmarks / sessions / collections
- 搜某个 workspace 内的 tags / notes

这能让 workspace 成为真正可操作的上下文，而不是静态分组。

---

## 13. Workspace Metadata

workspace 天然适合承载高层元数据。

例如：

- name
- description
- tags
- notes
- status
- priority
- last active time

这些元数据会让 workspace 比 collection/session 更像“项目”。

---

## 14. Workspace and Sync

workspace 必须是同步的一等对象。

否则多设备场景下，用户得到的只会是：

- 一堆同步过去的 session
- 一堆同步过去的 collection

却没有“这些资源属于哪个工作上下文”的结构。

因此：

- workspace definition 必须进入 iCloud sync
- workspace 相关的 note/tag/saved search 也必须进入 iCloud sync

---

## 15. Workspace Red Flags

如果未来出现这些情况，说明 workspace 模型被做窄了：

- workspace 只是 collection 的别名
- workspace 不能聚合 bookmarks
- workspace 不能参与搜索
- workspace 不能 startup
- workspace 不参与同步

---

## 16. Final Workspace Statement

CTM 的 workspace 应始终被定义为：

**长期、可搜索、可恢复、可同步的任务上下文。**

它是 CTM 从“资源管理工具”升级为“工作系统”的关键对象。
