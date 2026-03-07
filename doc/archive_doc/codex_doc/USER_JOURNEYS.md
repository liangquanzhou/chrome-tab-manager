# CTM — User Journeys

## 1. Purpose

这份文档定义 CTM 的**典型用户路径**。

它回答的是：

- 用户到底会怎么使用 CTM
- 完整产品的价值如何在真实路径中体现
- runtime、library、bookmarks、search、workspace、sync 是如何串起来的

这份文档的目标是防止产品只剩“功能列表”，却没有真实使用闭环。

---

## 2. Journey Philosophy

CTM 的用户路径不应该只围绕“操作 tab”展开。

完整产品的路径应覆盖三类目标：

1. **处理当前浏览器状态**
2. **沉淀长期资源**
3. **重新进入某个工作上下文**

因此，用户路径必须天然跨越：

- Runtime
- Library
- Search
- Workspace
- Sync

---

## 3. Primary Journeys

## 3.1 Journey A — Research Capture

### User intent

“我现在在查一个主题，开了很多 tab，我想把它们沉淀下来。”

### Typical flow

1. 用户在 `Tabs` 中浏览当前打开内容
2. 把部分 tabs 分成 groups
3. 把当前状态保存为 `Session`
4. 把一部分值得长期保留的链接存成 `Collection`
5. 把已有相关 `Bookmarks` 加进同一个 `Workspace`
6. 给这个 workspace 加 tag / note

### Product value

- 临时浏览变成长期资产
- 浏览状态不再只是“一堆当前打开的页”

---

## 3.2 Journey B — Resume Work

### User intent

“我今天想继续昨天那个项目。”

### Typical flow

1. 用户从 `Workspaces` 进入目标项目
2. 看到相关的 sessions / collections / bookmarks
3. 选择 `workspace startup`
4. 恢复相关 session，打开需要的 collection 项目
5. 回到 `Tabs` / `Groups` 继续工作

### Product value

- 从“重新找资源”变成“直接回到上下文”

---

## 3.3 Journey C — Find Something I Remember Vaguely

### User intent

“我记得之前见过一个链接，但忘了它是在 tab、session、collection 还是 bookmark 里。”

### Typical flow

1. 用户进入 `Search`
2. 搜索 title / host / keyword / tag
3. 在统一结果中看到：
   - 可能是 live tab
   - 可能是 bookmark
   - 可能是 collection item
   - 可能是某个 workspace 内资源
4. 直接对结果执行动作：
   - activate
   - open
   - restore
   - attach to workspace

### Product value

- Search 成为统一入口
- 用户不必先猜资源类型

---

## 3.4 Journey D — Curate a Resource Pack

### User intent

“我想把某个主题的关键链接整理成一个资源包。”

### Typical flow

1. 用户从 `Tabs`、`Bookmarks`、`Search` 中筛出相关资源
2. 把资源加入 `Collection`
3. 给 collection 起名、补 note
4. 以后可以：
   - partial restore
   - markdown export
   - attach to workspace

### Product value

- collection 成为长期复用资源，而不是临时列表

---

## 3.5 Journey E — Browser Bookmarks as Knowledge Base

### User intent

“我 Chrome 里已经有很多书签，但我想把它们真正组织起来。”

### Typical flow

1. 用户进入 `Bookmarks`
2. 浏览书签树或直接搜索
3. 给某些 bookmarks 添加：
   - tag
   - note
   - alias
4. 将部分书签加入 workspace
5. 以后从 `Search` 或 `Workspace` 重新找到它们

### Product value

- 原生书签从静态列表变成可组织的长期知识

---

## 3.6 Journey F — Cross-Device Continuity

### User intent

“我换一台 Mac，也想继续刚才的工作。”

### Typical flow

1. 用户在设备 A 上创建或修改：
   - sessions
   - collections
   - workspaces
   - bookmark overlays
2. CTM 通过 `iCloud` 同步 CTM-owned library
3. 在设备 B 上，用户打开 CTM
4. 进入 `Sync` 看状态正常
5. 再进入 `Workspaces` 或 `Search`
6. 继续工作

### Product value

- CTM 从单机工具升级为跨设备工作系统

---

## 3.7 Journey G — Live Browser Control

### User intent

“我现在就想快速把浏览器收拾干净、切组、切 tab、执行批量动作。”

### Typical flow

1. 用户在 `Tabs` 中过滤当前内容
2. 多选 tabs
3. 批量：
   - close
   - group
   - pin/unpin
   - add to collection
4. 切到 `Groups` 调整结构
5. 必要时保存成 session

### Product value

- CTM 既是长期系统，也是即时控制台

---

## 4. Secondary Journeys

## 4.1 Journey H — Review / Archive

### User intent

“我想整理历史资源，删掉不再需要的内容。”

### Typical flow

1. 用户浏览 sessions / collections / workspaces
2. 预览内容
3. 删除旧资源或归档
4. 保留真正重要的长期资产

---

## 4.2 Journey I — Export / Share

### User intent

“我想把这套资料导出给自己或别人。”

### Typical flow

1. 用户在 session / collection / workspace 中选择资源
2. 导出成 markdown / link list
3. 在外部工具中继续使用

---

## 4.3 Journey J — Diagnose Problems

### User intent

“为什么这台机器上没同步到，或者为什么浏览器没连上？”

### Typical flow

1. 用户进入 `Sync`
2. 查看：
   - sync health
   - failed items
   - conflict states
3. 必要时进入诊断 / repair 路径

---

## 5. Journey Coverage by Product Area

为了保证产品完整性，每个顶层区域都应该服务于至少一种关键用户路径。

| Area | Supports journeys |
|------|-------------------|
| Targets | G, F, J |
| Tabs | A, G |
| Groups | A, G |
| Sessions | A, B, H, I |
| Collections | A, D, H, I |
| Bookmarks | C, E |
| Search | C, E, B |
| Workspaces | A, B, E, F |
| Sync | F, J |

---

## 6. Journey-Based Product Test

以后判断 CTM 是不是在往完整产品走，不要只看功能数，而要看下面这些问题：

### Question 1

用户能不能从当前浏览状态自然沉淀出长期资源？

### Question 2

用户能不能从长期资源自然回到运行态？

### Question 3

用户能不能不先猜资源类型，直接搜索到想要的东西？

### Question 4

用户能不能跨设备继续同一个 workspace？

### Question 5

用户能不能把 bookmarks 纳入长期知识体系，而不是只读树结构？

---

## 7. Journey Red Flags

如果未来产品表现出下面这些倾向，说明用户路径开始断裂：

- 从 tabs 很难沉淀到 session / collection / workspace
- search 不能直接对结果执行动作
- bookmarks 很难进入 workspace
- sync 只是后台机制，不支持用户完成跨设备续接
- workspace 只是静态分组，不能 startup / resume

---

## 8. Final Journey Statement

CTM 的完整产品体验应该围绕这条主线：

**capture → organize → search → resume → sync → continue**

只要这条主线成立，CTM 就不是“很多功能的集合”，而是一个真正可持续使用的工作系统。
