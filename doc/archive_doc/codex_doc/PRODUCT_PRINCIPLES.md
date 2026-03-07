# CTM — Product Principles

## 1. Purpose

这份文档定义 CTM 的**产品原则**。

它回答的是：

- 这个产品必须始终坚持什么
- 哪些体验不能退化
- 哪些方向看起来合理，但实际上会把产品做偏

这份文档的目标是给后续所有 agent 和迭代一个稳定护栏。

---

## 2. Core Positioning

CTM 的核心定位始终是：

**terminal-first browser workspace manager**

这意味着它必须同时保持：

- browser-centered
- resource-oriented
- workspace-aware
- search-first
- sync-capable

其中任何一项被削弱，产品都会变形。

---

## 3. Product Principles

## Principle 1 — Terminal-first, not terminal-only gimmick

CTM 的价值不是“它跑在 terminal 里”，而是：

- terminal 里也能管理完整浏览器工作体系
- terminal 里也能做长期知识组织
- terminal 里也能跨设备继续工作

所以 terminal 是体验形态，不是唯一卖点。

---

## Principle 2 — Runtime and Library must coexist

CTM 不能只做 runtime control，也不能只做 library。

它必须同时支持：

- 现在浏览器里正在发生什么
- 什么值得被长期保存

这两条缺一不可。

---

## Principle 3 — Bookmarks are first-class

bookmarks 绝不是附属资源。

它们必须：

- 能被浏览
- 能被搜索
- 能被组织
- 能进入 workspace
- 能有 CTM 的增强层

如果 bookmarks 被边缘化，CTM 的长期知识能力就不完整。

---

## Principle 4 — Search is a product center, not a helper

搜索不是辅助功能。

它必须成为：

- 跨资源统一入口
- 结果即行动入口
- 连接 runtime、library、workspace 的桥梁

如果 search 退化成各页面自己的过滤框，产品会重新碎片化。

---

## Principle 5 — Workspace is the long-term center

session、collection、bookmark 都很重要，但长期中心对象应该是 workspace。

因为用户真正想管理的是：

- 某个项目
- 某个任务
- 某个主题上下文

而不是单个资源本身。

---

## Principle 6 — Sync must be visible and trustworthy

同步能力不能只是后台黑盒。

用户必须知道：

- 哪些资源同步了
- 哪些没同步
- 哪些有冲突
- 哪些需要修复

如果用户看不到 sync state，就不会真正信任产品。

---

## Principle 7 — Local-first always

CTM 必须先是本地可用的，再是云端增强的。

这意味着：

- 离线也能工作
- 同步失败不应阻断主流程
- 本地始终是可操作的真实产品

---

## Principle 8 — Cross-resource consistency matters more than feature count

一个功能做出来，不代表产品更完整。

真正重要的是：

- 它有没有进入统一搜索
- 它能不能进入 workspace
- 它的同步语义清不清楚
- 它在 CLI/TUI/palette 中是否一致

功能多但不一致，会让产品变别扭。

---

## Principle 9 — Power-user capability is part of the product

批量操作、导出、排序、去重、诊断，不是可有可无的小功能。

它们决定 CTM 能不能成为 daily driver。

所以 power features 不是“以后再说”，而是产品的一部分。

---

## Principle 10 — Every important state change must be legible

用户必须能理解：

- 发生了什么
- 改变了什么
- 作用在哪个 target / workspace / resource 上
- 结果是否已保存 / 同步 / 失败

不透明的状态变化会直接伤害信任。

---

## 4. Anti-Goals

CTM 不应该变成下面这些东西：

### Not a generic browser

CTM 不是要替代 Chrome 本身。

### Not a plain bookmark app

bookmarks 很重要，但 CTM 不是只做书签管理。

### Not a pure sync client

云同步重要，但 CTM 不是云盘前端。

### Not a disconnected collection of tools

CTM 不能变成：

- 一个 tabs 小工具
- 一个 sessions 小工具
- 一个 bookmarks 小工具

拼起来的工具箱。

它必须是完整系统。

---

## 5. Non-Negotiable Experience Rules

这些体验不允许退化：

1. 用户可以从 runtime 自然进入 library
2. 用户可以从 library 自然回到 runtime
3. bookmarks 永远可以进入 search 和 workspace
4. sync 状态必须可见
5. workspace 必须是长期上下文，而不是 collection 别名
6. command surfaces 不能语义漂移
7. 搜索结果必须可行动

---

## 6. Strategic Product Tests

以后每次判断 CTM 有没有跑偏，可以问这 7 个问题：

1. 这次迭代有没有加强 runtime 与 library 的连接？
2. bookmarks 有没有继续保持第一类资源地位？
3. search 有没有变得更像统一入口？
4. workspace 有没有更接近中心对象？
5. sync 有没有更透明、更可靠？
6. CLI/TUI/palette 的动作语义有没有更一致？
7. 用户是不是更容易跨设备继续工作了？

如果大多数答案是否定的，那说明虽然在“做功能”，但产品在偏航。

---

## 7. Product Red Flags

下面这些迹象说明 CTM 在退化：

- tabs/groups 功能越来越多，但 bookmarks/search/workspace 不动
- sync 只剩背景机制，没有可见体验
- workspace 被做成 collection 的别名
- CLI/TUI/palette 各有各的动作语义
- 功能越来越多，但无法串成真实用户路径

---

## 8. Final Product Statement

CTM 应始终坚持这条产品主线：

**control the browser, capture what matters, organize it into knowledge, search it globally, resume it as a workspace, and carry it across devices.**

只要这条主线还在，产品就不会跑偏。
