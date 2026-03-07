# CTM — Navigation Model

## 1. Purpose

这份文档定义 CTM 的**顶层导航模型**。

它回答的是：

- 用户进入产品后，应该先看到什么
- 顶层入口应该有哪些
- runtime、library、bookmarks、search、workspace、sync 这些能力如何在 UI 中被自然访问
- CLI 和 TUI 的导航世界观如何保持一致

这份文档只谈产品层导航，不谈技术实现。

---

## 2. Navigation Principles

CTM 的导航必须符合这几个原则：

1. **用户始终知道自己在哪个资源域**
2. **任何核心资源都不能成为隐藏能力**
3. **搜索是全局入口，不是某个页面的小功能**
4. **workspace 是高层组织入口，不是附属列表**
5. **sync 必须可见，不可藏在诊断角落**

---

## 3. Top-Level Product Areas

CTM 的完整导航应包含 9 个顶层区域：

1. `Targets`
2. `Tabs`
3. `Groups`
4. `Sessions`
5. `Collections`
6. `Bookmarks`
7. `Search`
8. `Workspaces`
9. `Sync`

它们不是平级功能堆积，而是三类入口：

### Runtime Entry

- Targets
- Tabs
- Groups

### Library Entry

- Sessions
- Collections
- Bookmarks

### High-Level Entry

- Search
- Workspaces
- Sync

---

## 4. What Each Top-Level Area Means

## 4.1 Targets

Targets 是“我正在操作哪个浏览器上下文”的入口。

### 用户在这里做什么

- 看当前有哪些 target 在线
- 选择默认 target
- 进入某个 target 的浏览器世界
- 理解当前动作会作用到哪

### 角色

- 安全边界入口
- runtime 范围入口

---

## 4.2 Tabs

Tabs 是“我当前正在处理哪些网页”的入口。

### 用户在这里做什么

- 浏览当前 tab
- 过滤 / 搜索当前打开内容
- 关闭 / 激活 / 分组 / 收藏
- 从运行态快速沉淀到 collection 或 session

### 角色

- 最常用的实时操作入口

---

## 4.3 Groups

Groups 是“运行态组织结构”的入口。

### 用户在这里做什么

- 查看当前 tab group
- 调整 group
- 浏览组内内容
- 把运行中的结构变成可理解上下文

### 角色

- runtime 组织入口

---

## 4.4 Sessions

Sessions 是“快照式长期保存”的入口。

### 用户在这里做什么

- 保存当前工作现场
- 预览过去快照
- 恢复一整套浏览状态
- 删除或导出快照

### 角色

- capture / restore 的长期资产入口

---

## 4.5 Collections

Collections 是“人工整理资源包”的入口。

### 用户在这里做什么

- 手工收集链接
- 从 tabs 抽取资源
- 部分恢复内容
- 对长期资源进行轻量整理

### 角色

- 手工 curated resources 入口

---

## 4.6 Bookmarks

Bookmarks 是“原生书签 + CTM 增强”的入口。

### 用户在这里做什么

- 浏览书签树
- 搜索书签
- 给书签增加 CTM 元数据
- 把书签纳入 workspace 或 collection

### 角色

- 长期知识入口
- 连接 Google Sync 与 CTM library 的桥梁

---

## 4.7 Search

Search 不是“某个列表上的输入框”，而是一个顶层入口。

### 用户在这里做什么

- 跨资源统一搜索
- 搜 tabs / sessions / collections / bookmarks / workspaces
- 保存搜索
- 把搜索结果转成行动

### 角色

- 全局入口
- 资源间跳转入口

---

## 4.8 Workspaces

Workspaces 是“我正在做什么任务”的入口。

### 用户在这里做什么

- 进入某个工作上下文
- 看到与该任务相关的 session / collection / bookmark / note
- 启动或恢复一整套工作环境

### 角色

- 产品高层中心入口

---

## 4.9 Sync

Sync 是“这些资源现在是否安全、是否跨设备存在”的入口。

### 用户在这里做什么

- 看同步状态
- 看最近同步结果
- 理解冲突
- 手动修复或重建

### 角色

- 基础设施可见性入口
- 跨设备连续性入口

---

## 5. Default Navigation Hierarchy

CTM 的默认导航层级应该这样理解：

### Level 1: Product-wide Entry

- Search
- Workspaces

这是最接近“我要做什么”的入口。

### Level 2: Resource Domains

- Tabs
- Groups
- Sessions
- Collections
- Bookmarks

这是最接近“我要处理哪类资源”的入口。

### Level 3: Context / Infrastructure

- Targets
- Sync

这是最接近“这件事在哪做、现在是否可靠”的入口。

---

## 6. TUI Navigation Model

TUI 不应该只围绕实时浏览器列表来设计。

更完整的顶层导航模型应该默认存在这几块：

- Runtime lane
- Library lane
- Global lane

### Runtime lane

- Targets
- Tabs
- Groups

### Library lane

- Sessions
- Collections
- Bookmarks

### Global lane

- Search
- Workspaces
- Sync

### Product implication

这意味着 TUI 的未来形态不应只是 4 个 tab 页，而应具备：

- 资源域切换
- 全局搜索入口
- workspace 入口
- sync 状态入口

---

## 7. CLI Navigation Model

CLI 也应该和 TUI 使用同一套导航世界观。

因此 CLI 的顶层命名空间最终应接近：

- `ctm targets ...`
- `ctm tabs ...`
- `ctm groups ...`
- `ctm sessions ...`
- `ctm collections ...`
- `ctm bookmarks ...`
- `ctm search ...`
- `ctm workspaces ...`
- `ctm sync ...`

这样用户在 CLI 和 TUI 之间切换时，心智模型不会断裂。

---

## 8. Primary User Journeys

顶层导航必须支持下面这些典型路径。

### Journey A — 从当前浏览状态沉淀长期资产

Tabs → Groups → Session/Collection → Workspace

### Journey B — 从长期知识找到资源并恢复运行态

Search / Bookmarks / Collections / Workspaces → Restore/Open → Tabs

### Journey C — 切换工作上下文

Workspace → Startup/Restore → Runtime

### Journey D — 跨设备继续工作

Sync → Workspace / Sessions / Collections → Restore

### Journey E — 从书签进入更高层组织

Bookmarks → Tag/Note/Attach → Workspace

---

## 9. Navigation Red Flags

如果未来出现下面这些迹象，说明导航模型开始退化：

- Search 只剩某个列表上的过滤框
- Workspaces 被放在次级菜单里
- Bookmarks 变成一个附属页
- Sync 没有独立入口
- Tabs/Groups 占满主界面，而 library 能力被边缘化

---

## 10. Final Navigation Statement

CTM 的导航模型应该始终体现这句话：

**用户既可以从“当前浏览器状态”进入，也可以从“长期资源”和“工作区”进入。**

这三种入口都必须是产品的一等入口，而不是谁附属于谁。
