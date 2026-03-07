# CTM — Sync Model

产品原则见 `01_PRODUCT.md` P6/P7。Source of Truth 见 `01_PRODUCT.md` §3。

## 1. Sync Philosophy

**local-first, cloud-enhanced product**

- 本地始终可用
- 云同步是增强，不是生存前提
- 同步失败不让核心工作流失效

## 2. Dual-Track Sync

| Track | Covers | Source of Truth | Cloud | CTM Role |
|-------|--------|-----------------|-------|----------|
| **A: Browser-Native** | 原生书签 | Chrome / Google | Google Sync | 读取 / 镜像 / 增强，不替代 |
| **B: CTM Library** | sessions, collections, workspaces, tags, notes, overlays, saved searches | CTM local | iCloud | 完全负责 |

## 3. What Syncs Where

| Data Type | Source of Truth | Cloud Path |
|-----------|-----------------|------------|
| Native bookmarks | Chrome / Google | Google Sync |
| BookmarkMirror | CTM local | optional via iCloud |
| BookmarkOverlay | CTM local | iCloud |
| Sessions | CTM local | iCloud |
| Collections | CTM local | iCloud |
| Workspaces | CTM local | iCloud |
| Tags / Notes | CTM local | iCloud |
| Saved searches | CTM local | iCloud |
| Runtime state | Browser live | 不直接同步 |

## 4. Three Sync Domains

| Domain | Covers |
|--------|--------|
| **Library** | sessions, collections, workspaces |
| **Knowledge** | bookmark overlays, tags, notes, aliases, saved searches |
| **System** | sync state, device metadata, last sync markers |

## 5. iCloud Role

1. **Cross-device continuity** — 多台 Mac 间继续同一套工作体系
2. **Cloud persistence** — 长期资源不依赖单机
3. **Conflict-aware replication** — 多端修改时给出可解释状态

## 6. Seven Sync States

| State | 含义 |
|-------|------|
| `local_only` | 仅本地，未同步 |
| `syncing` | 正在同步中 |
| `synced` | 已同步完成 |
| `stale` | 云端有更新，本地未拉取 |
| `conflicted` | 多端修改冲突 |
| `failed` | 同步失败 |
| `disabled` | 同步已禁用 |

这些状态在产品中可感知，不只给系统看。

## 7. Sync Rules

1. 本地优先，本地始终可读可写
2. 云端同步异步增强，不阻塞主要工作流
3. 同步冲突显式可见，不悄悄吞掉
4. 原生书签与 CTM overlay 分离，不互相覆盖
5. 用户能知道：已同步/未同步/有冲突/需修复

## 8. Conflict Model

三种冲突处理思路：
1. 保留最新版本 (last-write-wins)
2. 保留两个版本 (conflict file)
3. 标记为待人工处理

产品世界观里"冲突是可见状态"。

## 9. Device Model

`Device` 帮助回答：
- 这条资源是哪台机器创建的
- 哪次修改来自哪台设备
- 为什么发生冲突
- 哪台机器落后了

## 10. Offline Behavior

离线时可以继续：保存 session、创建 collection、编辑 workspace/tags/notes、浏览 bookmark mirror。

恢复在线后：送 CTM-owned changes 到 iCloud、刷新 mirror/sync state。

## 11. Sync Visibility

产品中必须有明确 sync 入口，显示：
- overall sync health
- last successful sync
- failed / conflicted resources
- current device
- connected cloud status

## 12. Sync-Aware from Day One

即使不全部实现，以下对象必须从第一天具备 sync-aware 身份（UUID + timestamps）：

Session, Collection, Workspace, BookmarkOverlay, Tag, Note, SavedSearch

## 13. Red Flags

- 想把 sessions/collections 塞进 Google Sync
- 想让 runtime live state 成为云端真相源
- 想把 bookmark overlay 直接写回原生 bookmark 结构
- 没有独立 sync status / conflict 概念
- 同步失败后主要工作流不可用
