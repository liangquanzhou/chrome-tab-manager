# CTM — Sync Model

## 1. Purpose

这份文档定义 CTM 的**同步模型**。

它回答的是：

- 哪些数据由 Google / Chrome 负责同步
- 哪些数据由 CTM 自己负责同步
- iCloud 在产品里承担什么角色
- 本地、云端、外部来源之间的关系是什么

这份文档的目标是保证：

- 同步能力从第一天就是产品骨架
- 但不会把产品绑死在某一个同步提供方上

---

## 2. Sync Philosophy

CTM 应该是一个：

**local-first, cloud-enhanced product**

意思是：

- 本地始终可用
- 云同步是增强，不是生存前提
- 同步失败不应让核心工作流失效

---

## 3. Dual-Track Sync

CTM 从一开始就采用双轨同步模型。

## 3.1 Track A — Browser-Native Sync

这一轨由浏览器自己负责。

### Covers

- native bookmarks

### Source of truth

- Chrome / Google account

### CTM role

- 读取
- 镜像
- 增强
- 不替代其原生同步体系

---

## 3.2 Track B — CTM Library Sync

这一轨由 CTM 自己负责。

### Covers

- sessions
- collections
- workspace definitions
- tags
- notes
- bookmark overlays
- saved searches
- sync metadata

### Source of truth

- CTM local library

### Cloud target

- iCloud

---

## 4. What Syncs Where

为了避免混乱，CTM 必须从产品层明确下面这张表。

| Data Type | Source of Truth | Cloud Path |
|-----------|-----------------|------------|
| Native bookmarks | Chrome / Google | Google Sync |
| Bookmark mirror | CTM local mirror | optional via iCloud |
| Bookmark overlay | CTM local library | iCloud |
| Sessions | CTM local library | iCloud |
| Collections | CTM local library | iCloud |
| Workspaces | CTM local library | iCloud |
| Tags / Notes | CTM local library | iCloud |
| Saved searches | CTM local library | iCloud |
| Runtime state | Browser live state | 不直接同步 |

---

## 5. iCloud Role

iCloud 在 CTM 中不是“附加备份”。

它应该承担三个角色：

1. **Cross-device continuity**
   - 多台 Mac 之间继续同一套工作体系

2. **Cloud persistence**
   - 让 CTM 长期资源不依赖单机

3. **Conflict-aware replication**
   - 当多端修改发生时，给出可解释状态

---

## 6. Sync Domains

不同资源需要按不同域同步，而不是混成一坨。

### 6.1 Library Domain

同步内容：

- sessions
- collections
- workspaces

### 6.2 Knowledge Domain

同步内容：

- bookmark overlays
- tags
- notes
- aliases
- saved searches

### 6.3 System Domain

同步内容：

- sync state
- device metadata
- last sync markers

---

## 7. Sync Rules

CTM 的同步规则应在产品层固定为：

### Rule 1

本地优先，本地始终可读可写。

### Rule 2

云端同步是异步增强，不阻塞主要工作流。

### Rule 3

同步冲突必须显式可见，不能悄悄吞掉。

### Rule 4

原生书签与 CTM overlay 分离，不互相覆盖。

### Rule 5

用户必须能知道：

- 哪些数据已经同步
- 哪些还没同步
- 哪些有冲突
- 哪些需要修复

---

## 8. Sync States

从产品体验角度，CTM 的资源至少应有这些同步状态：

- `local-only`
- `syncing`
- `synced`
- `stale`
- `conflicted`
- `failed`
- `disabled`

这些状态不只是给系统看，也应该在产品中可感知。

---

## 9. Device Model

同步不是抽象发生的，而是发生在设备之间。

因此产品上应承认 `Device` 的存在。

### Device helps answer

- 这条 session 是哪台机器创建的
- 哪次修改来自哪台设备
- 为什么会发生冲突
- 哪台机器落后了

---

## 10. Conflict Model

冲突不是异常情况，而是同步产品的常态能力之一。

CTM 需要从高层就接受：

- 两台设备可能同时修改同一 workspace
- bookmark overlay 可能和本地镜像版本不一致
- collection/saved search 可能同时被改

所以产品层至少要存在三种冲突处理思路：

1. 保留最新版本
2. 保留两个版本
3. 标记为待人工处理

这不要求一开始就做复杂合并 UI。

但要求产品世界观里有“冲突是可见状态”。

---

## 11. Sync Visibility

Sync 不能隐藏在后台。

产品中必须有一个明确的 sync 入口，让用户看到：

- overall sync health
- last successful sync
- failed resources
- conflicted resources
- current device
- connected cloud status

---

## 12. Offline Behavior

CTM 必须默认支持离线工作。

### Offline means

- 可以继续保存 session
- 可以继续创建 collection
- 可以继续编辑 workspace / tags / notes
- 可以继续浏览 bookmark mirror

### When back online

- 再把 CTM-owned changes 送到 iCloud
- 再刷新 mirror / sync state

---

## 13. What Must Be Sync-Aware from Day One

即使一开始没有把所有同步流程都做满，下面这些对象也必须从第一天具备 sync-aware 身份：

- Session
- Collection
- Workspace
- BookmarkOverlay
- Tag
- Note
- SavedSearch

---

## 14. Sync Red Flags

如果未来出现这些设计倾向，说明同步模型开始偏了：

- 想把 sessions/collections 直接塞进 Google Sync
- 想让 runtime live state 成为云端真相源
- 想把 bookmark overlay 直接写回原生 bookmark 结构
- 没有独立的 sync status / conflict 概念
- 同步失败后让主要工作流不可用

---

## 15. Final Sync Statement

CTM 的同步模型应始终是：

- Google Sync 负责原生书签
- CTM 本地库负责产品自己的长期资源
- iCloud 负责 CTM 自有资源的云同步
- 本地优先，云端增强，冲突可见

这四点必须同时成立。
