# CTM — Workspace Model

领域定义见 `02_DOMAIN.md` §3.5。产品原则见 `01_PRODUCT.md` P5。

## 1. Definition

**围绕某个任务、项目或主题组织起来的一组浏览器相关资源。**

不是单一 session，不是单一 collection，不是单一书签文件夹。是这些资源的上层组织单位。

## 2. What Workspace Contains

- sessions
- collections
- bookmarks / bookmark overlays
- tags / notes
- saved searches
- optional default target context

Workspace 既是"资源容器"，也是"任务上下文"。

## 3. Workspace vs Others

| | Session | Collection | Workspace |
|-|---------|------------|-----------|
| 本质 | 时间点快照 | 人工整理的链接包 | 长期任务上下文 |
| 偏向 | 恢复现场 | 资源集合 | 持续演化 |
| 回答 | "当时浏览器是什么状态" | "我整理了哪些链接" | "这项工作由哪些资源组成" |

## 4. Workspace Responsibilities

| 责任 | 说明 |
|------|------|
| Organization | 不同类型资源挂到同一任务上下文 |
| Entry Point | 用户从 workspace 进入工作系统，不从某个 tab 开始 |
| Startup | 启动或恢复相关资源 |
| Search Scope | workspace 是天然搜索范围 |
| Long-Term Continuity | 参与同步，跨设备连续工作的核心对象 |

## 5. Workspace Lifecycle

`create → name → attach resources → evolve → search within → startup/restore → archive/delete`

Workspace 不是一次性快照，是长期存在的上下文。

## 6. Workspace Startup

用户重新进入任务时，workspace startup = 重新进入工作上下文：
- 打开相关 session
- 打开相关 collection
- 打开相关 bookmark subset
- 恢复特定资源组合

## 7. Workspace Templates

为重复性工作流提供快速入口（research / weekly review / project onboarding）。

## 8. Workspace Search

workspace 不只被搜索到，也是搜索范围：
- 搜 workspace 内全部资源
- 搜 workspace 内的 bookmarks / sessions / collections
- 搜 workspace 内的 tags / notes

## 9. Workspace Metadata

- name / description
- tags / notes
- status (active / archived)
- priority
- last active time
- default target

## 10. Workspace and Sync

workspace definition 必须进入 iCloud sync。否则多设备只有同步过去的 session/collection，没有"这些资源属于哪个工作上下文"的结构。

## 11. Red Flags

- workspace 只是 collection 别名
- workspace 不能聚合 bookmarks
- workspace 不能参与搜索
- workspace 不能 startup
- workspace 不参与同步
