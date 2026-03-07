# CTM — Interaction Model

导航、信息架构、Command Surfaces、用户路径的唯一定义。

## 1. Nine Top-Level Areas

| # | Area | Lane | 用户在这里做什么 |
|---|------|------|-----------------|
| 1 | **Targets** | Runtime | 选择浏览器上下文、设 default、理解动作作用域 |
| 2 | **Tabs** | Runtime | 浏览/过滤/激活/关闭/分组/收藏当前 tabs |
| 3 | **Groups** | Runtime | 查看/调整 tab group 结构 |
| 4 | **Sessions** | Library | 保存/预览/恢复/删除浏览器快照 |
| 5 | **Collections** | Library | 收集/整理/恢复链接资源包 |
| 6 | **Bookmarks** | Library | 浏览书签树/搜索/tag/note/进入 workspace |
| 7 | **Search** | Global | 跨资源统一搜索，结果即行动入口 |
| 8 | **Workspaces** | Global | 进入任务上下文，startup/resume 工作环境 |
| 9 | **Sync** | Global | 查看同步状态/冲突/修复 |

## 2. Navigation Hierarchy

| Level | Areas | 回答的问题 |
|-------|-------|-----------|
| **Level 1: Global Entry** | Search, Workspaces | "我要做什么" |
| **Level 2: Resource Domains** | Tabs, Groups, Sessions, Collections, Bookmarks | "我要处理哪类资源" |
| **Level 3: Context/Infrastructure** | Targets, Sync | "这件事在哪做、是否可靠" |

## 3. Three Navigation Lanes

| Lane | Areas | 特征 |
|------|-------|------|
| **Runtime** | Targets, Tabs, Groups | 实时操作、数据活跃、变化频繁 |
| **Library** | Sessions, Collections, Bookmarks | 长期保存、可搜索/恢复/导出 |
| **Global** | Search, Workspaces, Sync | 跨资源入口、高层组织、基础设施 |

TUI 未来形态不应只有 4 个 tab 页，应具备资源域切换 + 全局搜索入口 + workspace 入口 + sync 状态入口。

## 4. Cross-Area Relationships

- **Runtime → Library**: Tabs / Groups 流向 Sessions / Collections
- **Bookmarks → Workspace**: Bookmarks 进入 Workspace
- **Search → Everything**: Search 进入任何资源域
- **Sync → Library / Workspace / Overlay**: Sync 面向 CTM-owned resources
- **Workspaces ↔ Resource Domains**: Workspace 聚合 Sessions / Collections / Bookmarks

## 5. Three Command Surfaces

| Surface | 最适合 | 优化目标 |
|---------|--------|----------|
| **CLI** | 明确的资源操作、批量参数化、机器调用、导出/导入/诊断、脚本自动化 | precision, scripting, reproducibility |
| **TUI** | 浏览大量资源、多选、过滤、预览、交互式整理、上下文切换 | discoverability, exploration, context |
| **Command Palette** | 高频全局动作、跨域跳转、轻量命令执行 | speed, cross-domain jump, low friction |

### Surface-by-Capability Mapping

| Capability | Primary | Secondary |
|------------|---------|-----------|
| Runtime tab control | TUI | CLI |
| Group management | TUI | CLI |
| Session save/restore | CLI + TUI | Palette |
| Collection curation | TUI | CLI |
| Bookmark browse | TUI | CLI |
| Unified search | TUI + Palette | CLI |
| Workspace startup | TUI + CLI | Palette |
| Sync status | TUI + CLI | — |
| Sync repair | CLI | TUI |
| Export | CLI | TUI |
| Diagnostics | CLI | TUI |

### Surface Boundaries

- **CLI 不做**：大量视觉探索、复杂树形浏览、重交互选择器
- **TUI 不做**：所有高级批处理脚本、全部安装/诊断细节
- **Palette 不做**：完整批量资源管理、深层编辑流程、多步 wizard

### Consistency Rules

1. 同一核心动作在不同 surface 有同样业务含义
2. CLI 和 TUI 共享同一套资源命名空间（targets / tabs / groups / sessions / collections / bookmarks / workspaces / sync）
3. Palette 复用已有动作模型，不发明新语义
4. TUI 高频动作可有快捷键，但与 palette 和 CLI 能力边界对齐

## 6. CLI Navigation

CLI 顶层命名空间：

```
ctm targets ...
ctm tabs ...
ctm groups ...
ctm sessions ...
ctm collections ...
ctm bookmarks ...
ctm search ...
ctm workspaces ...
ctm sync ...
```

与 TUI 共享同一套产品世界观。

## 7. Primary User Journeys

### Journey A — Research Capture
**"我查了很多东西，想沉淀下来"**

Tabs → Groups → Session/Collection → Workspace → tag/note

### Journey B — Resume Work
**"我要继续昨天那个项目"**

Workspaces → 查看关联资源 → startup → Session restore → Tabs/Groups

### Journey C — Find Something Vaguely Remembered
**"我记得之前见过，忘了在哪"**

Search → 统一结果(tab/bookmark/collection/workspace) → 直接行动(activate/open/restore/attach)

### Journey D — Curate a Resource Pack
**"把某主题的关键链接整理成资源包"**

Tabs/Bookmarks/Search → 筛选 → Collection → name/note → partial restore / export / workspace attach

### Journey E — Bookmarks as Knowledge Base
**"把 Chrome 书签真正组织起来"**

Bookmarks → 浏览/搜索 → tag/note/alias → 加入 Workspace → 从 Search 或 Workspace 重新找到

### Journey F — Cross-Device Continuity
**"换一台 Mac，继续工作"**

设备 A 创建/修改 sessions/collections/workspaces → iCloud 同步 → 设备 B 打开 CTM → Sync 状态正常 → Workspaces/Search → 继续工作

### Journey G — Live Browser Control
**"快速收拾浏览器、批量操作"**

Tabs → 过滤/多选 → 批量 close/group/pin → Groups 调整 → 保存 session

### Journey H — Review / Archive
**"整理历史资源"**

Sessions/Collections/Workspaces → 预览 → 删除或归档 → 保留重要资产

### Journey I — Export / Share
**"导出资料给自己或别人"**

Session/Collection/Workspace → 导出 markdown/link list → 外部使用

### Journey J — Diagnose Problems
**"为什么没同步 / 为什么没连上"**

Sync → 查看 health/failed/conflict → 修复/repair

## 8. Journey Coverage

| Area | Supports Journeys |
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

## 9. Entry Point Strategy

| 用户类型 | 最常从哪进入 |
|---------|-------------|
| Runtime-first | Targets, Tabs |
| Library-first | Sessions, Collections, Bookmarks |
| Goal-first | Search, Workspaces |
| Reliability-first | Sync |

## 10. Navigation Red Flags

- Search 只剩某个列表的过滤框
- Workspaces 被放在次级菜单
- Bookmarks 变成附属页
- Sync 没有独立入口
- Tabs/Groups 占满主界面，library 能力被边缘化
- palette 复制完整 CLI
- TUI 承担太多安装/修复命令
- 同一动作在三个 surface 上名字和语义不一致
