# CTM — Product Definition

## 1. What Is CTM

**CTM = terminal-first browser workspace manager**

它同时处理三件事：

1. **控制浏览器当前运行态** — targets, windows, tabs, groups
2. **维护用户长期知识库** — sessions, collections, bookmarks, tags, notes, workspaces
3. **把长期数据同步到云端** — Chrome/Google Sync 负责原生书签，CTM + iCloud 负责 CTM 自有资源

## 2. Four-Layer Product Model

| Layer | 职责 | 核心对象 |
|-------|------|----------|
| **Browser Runtime** | 在线浏览器状态 | Target, Window, Tab, Group, BookmarkSource |
| **Library** | CTM 自有长期数据 | Session, Collection, BookmarkMirror, BookmarkOverlay, Tag, Note, SavedSearch |
| **Sync** | 跨设备同步 | SyncAccount, SyncState, Device |
| **Interaction** | 用户交互面 | CLI, TUI, Command Palette |

Interaction Layer 消费上面三层，不发明业务模型。

## 3. Source of Truth

| 数据 | 真相源 | 云端路径 |
|------|--------|----------|
| tabs / groups / windows / active state | 浏览器 | 不直接同步 |
| 原生书签树 | Chrome / Google account | Google Sync |
| sessions / collections / workspaces | CTM 本地 | iCloud |
| bookmark overlay / tags / notes / saved searches | CTM 本地 | iCloud |
| sync state / device metadata | CTM 本地 | iCloud |

云端只承担复制与同步，不重新定义业务真相。

## 4. Non-Goals

CTM 不应该变成：

- 通用浏览器
- 纯 bookmark manager
- 云盘客户端
- 笔记系统的替代品
- 一组松散拼凑的小工具

产品边界：**browser-centered workspace management**。一切围绕浏览器工作流，但不被浏览器当前运行态绑死。

## 5. Product Principles

| # | 原则 | 要点 |
|---|------|------|
| P1 | Terminal-first, not terminal-only gimmick | terminal 是体验形态，不是唯一卖点 |
| P2 | Runtime + Library 共存 | 只有 runtime → 控制台；只有 library → 失去实时掌控 |
| P3 | Bookmarks 是一等资源 | 能浏览/搜索/组织/进入 workspace/有增强层 |
| P4 | Search 是产品中心 | 跨资源统一入口，结果即行动入口 |
| P5 | Workspace 是长期中心 | 管理的是任务上下文，不是单一资源 |
| P6 | Sync 必须可见可信 | 用户能看到同步/冲突/修复状态 |
| P7 | Local-first | 离线可用，同步失败不阻断主流程 |
| P8 | 跨资源一致性 > 功能数量 | 功能多但不一致，产品会别扭 |
| P9 | Power-user 能力是产品本体 | 批量/导出/去重/诊断决定能否成为 daily driver |
| P10 | 状态变更必须可读 | 发生了什么、改了什么、结果是否保存/同步/失败 |

## 6. Non-Negotiable Experience Rules

1. 用户可以从 runtime 自然进入 library
2. 用户可以从 library 自然回到 runtime
3. bookmarks 永远可以进入 search 和 workspace
4. sync 状态必须可见
5. workspace 必须是长期上下文，不是 collection 别名
6. command surfaces（CLI/TUI/palette）不能语义漂移
7. 搜索结果必须可行动

## 7. Decision Log

| # | Decision | Status |
|---|----------|--------|
| D-001 | 产品定义 = terminal-first browser workspace manager | Accepted |
| D-002 | 不按 v1/v2 切碎产品定义，一次性完整建模 | Accepted |
| D-003 | Runtime + Library 必须共存 | Accepted |
| D-004 | Bookmarks 是第一类资源 | Accepted |
| D-005 | 双轨同步：Chrome/Google + CTM/iCloud | Accepted |
| D-006 | Local-first 不可协商 | Accepted |
| D-007 | Workspace 是长期中心对象 | Accepted |
| D-008 | Search 是产品中心 | Accepted |
| D-009 | 顶层区域 = Targets/Tabs/Groups/Sessions/Collections/Bookmarks/Search/Workspaces/Sync | Accepted |
| D-010 | CLI = explicit control, TUI = exploration, Palette = fast shortcut | Accepted |
| D-011 | 建设按能力骨架排序，不按功能零碎堆叠 | Accepted |
| D-012 | Bookmark 三层模型：Source / Mirror / Overlay | Accepted |
| D-013 | Sync 必须有独立产品入口 | Accepted |
| D-014 | bookmarks/search/workspace/sync/power 都属于产品本体 | Accepted |
| D-015 | CTM 始终以浏览器为中心，不扩展成泛笔记/泛知识库 | Accepted |

追加规则：新决策直接追加，不覆盖已接受决策。如推翻旧决策，新增 superseding 条目并标记旧条目。

## 8. Strategic Product Tests

每次迭代问这 7 个问题：

1. runtime 与 library 的连接加强了吗？
2. bookmarks 保持第一类资源地位了吗？
3. search 更像统一入口了吗？
4. workspace 更接近中心对象了吗？
5. sync 更透明可靠了吗？
6. CLI/TUI/palette 的动作语义更一致了吗？
7. 用户更容易跨设备继续工作了吗？

大多数答案为否 → 产品在偏航。

## 9. Product Red Flags

- tabs/groups 功能越来越多，但 bookmarks/search/workspace 不动
- sync 只剩背景机制，没有可见体验
- workspace 被做成 collection 别名
- CLI/TUI/palette 各有各的动作语义
- 功能越来越多，但无法串成真实用户路径
- bookmarks 被放到二级页面
- search 不是顶层入口

## 10. Final Statement

**control the browser, capture what matters, organize it into knowledge, search it globally, resume it as a workspace, and carry it across devices.**
