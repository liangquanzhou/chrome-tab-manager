# CTM — Bookmarks Model

## 1. Purpose

这份文档定义 CTM 的**bookmarks 模型**。

它回答的是：

- 原生书签和 CTM 书签能力的关系是什么
- bookmarks 为什么必须是一等资源
- bookmark source、mirror、overlay 各自扮演什么角色

---

## 2. Why Bookmarks Matter

bookmarks 不是附属功能。

它们是：

- 长期知识入口
- 搜索的重要资源域
- workspace 的组成部分
- Google Sync 与 CTM 自有系统之间的桥梁

如果 bookmarks 只是“再加一个 view”，产品一定会做窄。

---

## 3. Three Bookmark Layers

CTM 应该从一开始就把 bookmarks 分成三层。

## 3.1 Bookmark Source

### Definition

浏览器原生书签树。

### Owner

- Chrome / Google account

### Role

- 提供原始数据
- 作为书签能力的上游来源

---

## 3.2 Bookmark Mirror

### Definition

CTM 对原生书签的本地镜像视图。

### Role

- 统一搜索
- 统一浏览
- 统一参与 workspace / collections / exports

### Why needed

因为 CTM 不能每次都把书签只当浏览器私有结构看待。

---

## 3.3 Bookmark Overlay

### Definition

CTM 对书签附加的本地增强层。

### Can contain

- tags
- notes
- aliases
- local grouping
- workspace relationships

### Owner

- CTM

### Sync target

- iCloud

---

## 4. Bookmark Source of Truth

bookmarks 需要双重真相结构：

### Native truth

- 原生书签结构
- 由 Chrome / Google Sync 维护

### CTM truth

- 对书签的增强解释和组织
- 由 CTM library 维护

这两者不能混成一层。

---

## 5. What Bookmark Features Belong to CTM

CTM 不需要取代 Chrome 书签系统，但应该补齐 Chrome 没做好的能力：

- tree browsing in terminal
- unified search
- tag / note / alias
- workspace attachment
- cross-resource linking
- export

---

## 6. Bookmarks in Search

bookmarks 必须天然进入统一搜索。

否则用户仍然要在：

- tabs
- collections
- bookmarks

几个地方分别找东西，CTM 的价值会大打折扣。

---

## 7. Bookmarks in Workspace

bookmarks 不是孤立资源。

它们应该能：

- 进入 workspace
- 与 session / collection 并列存在
- 承载 task-specific tags / notes

这样 workspace 才不是只由临时浏览状态组成。

---

## 8. Bookmarks in Sync

bookmarks 的同步必须分两部分理解。

### Native bookmark sync

- 由 Google / Chrome 负责

### Bookmark overlay sync

- 由 CTM + iCloud 负责

这意味着：

- 原生 bookmark 树不由 CTM 云端重新定义
- 但 CTM 对书签的增强能力要能跨设备同步

---

## 9. Bookmark User Journeys

典型用户路径应该包括：

### Journey A

Bookmark → search → open in tab

### Journey B

Bookmark → attach to workspace

### Journey C

Bookmark → add tag/note → become long-term knowledge asset

### Journey D

Bookmark set → export / collection / startup material

---

## 10. Bookmark Red Flags

如果未来出现这些设计，说明 bookmark 模型又被做窄了：

- bookmarks 只是一个只读列表
- bookmarks 不参与统一搜索
- bookmarks 不能附加 note/tag
- bookmarks 不能进入 workspace
- CTM 想接管原生书签同步本身

---

## 11. Final Bookmarks Statement

CTM 的 bookmarks 模型应始终是：

- 原生书签由 Chrome / Google 维护
- CTM 提供 mirror
- CTM 提供 overlay
- bookmarks 是 unified search 和 workspace 的一等资源

这四点缺一不可。
