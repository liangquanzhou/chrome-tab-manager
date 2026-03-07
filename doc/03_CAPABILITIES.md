# CTM — Capabilities, Build Order & Features

## 1. Seven Capability Areas

| # | Area | 职责 | 首日必须是一等能力 |
|---|------|------|--------------------|
| 1 | **Runtime** | 当前浏览器状态控制 | — |
| 2 | **Library** | 长期资源保存与复用 | — |
| 3 | **Bookmarks** | 原生书签 + CTM 增强 | yes |
| 4 | **Sync** | 跨设备资源同步 | yes |
| 5 | **Search** | 跨资源统一检索 | yes |
| 6 | **Workspace** | 围绕任务组织资源 | yes |
| 7 | **Power** | 高频用户与自动化 | — |

## 2. Capability Details

### 2.1 Runtime
- Target discovery / selection / default / multi-target
- Window awareness
- Tab list / open / close / activate / pin / unpin
- Group list / create / update / dissolve
- Live browser events + runtime filtering

### 2.2 Library
- Session save / preview / restore / delete
- Collection create / add / remove / restore
- Partial restore
- Export (markdown / link list)
- Duplicate detection, auto snapshot, crash recovery

### 2.3 Bookmarks
- Bookmark tree browse / search
- BookmarkMirror (本地可搜索镜像)
- BookmarkOverlay (tags / notes / aliases)
- Bookmark export
- Chrome 是原生书签同步来源，CTM 负责增强层

### 2.4 Sync
- Local-first persistence
- Google-backed bookmark source (Track A)
- iCloud sync for CTM-owned data (Track B)
- Sync status / conflict handling / selective sync
- Sync repair / rebuild / device awareness

### 2.5 Search
- Cross-resource search (tabs / sessions / collections / bookmarks / workspaces)
- Search by title / url / host / tag / note / alias
- Saved searches + smart collections
- Search result actions (activate / restore / open / attach)

### 2.6 Workspace
- Create / rename / delete workspace
- Attach sessions / collections / bookmarks / notes / tags / saved searches
- Workspace startup / restore / templates
- Workspace-level search

### 2.7 Power
- Batch operations (close / pin / unpin / group)
- Tab sort / deduplicate / suspend / discard
- Domain grouping
- Markdown export
- Automation hooks
- Diagnostics / doctor / repair

## 3. Capability Relationships

```
Runtime ←→ Library     runtime 提供实时数据，library 沉淀有价值状态
Bookmarks ←→ Search    bookmarks 是搜索重要资源域，overlay 扩展搜索维度
Library ←→ Workspace   workspace 聚合 library 资源
Library ←→ Sync        CTM-owned library 独立同步
Runtime ←→ Workspace   workspace startup 驱动 runtime restore
```

## 4. Build Order (8 Stages)

**原则**：先定领域，再定交互。先定对象，再定动作。先定同步边界，再定云功能。

| Stage | Name | 完成标志 |
|-------|------|----------|
| 1 | **Runtime Foundation** | 能稳定回答：哪些 target 在线、每个 target 有什么 tabs/groups、状态变化能否实时反映、多 target 作用域是否清晰 |
| 2 | **Library Foundation** | 能区分 session(快照) vs collection(人工整理)，capture/restore/export 边界清楚 |
| 3 | **Bookmarks** | 原生书签和 CTM 增强层关系明确，bookmarks 参与 search 和 workspace |
| 4 | **Sync Foundation** | 哪些数据跟 Google 同步、哪些跟 iCloud 同步、sync 失败时产品仍可用 |
| 5 | **Search Layer** | 搜索跨资源、bookmarks/sessions/collections 统一进入搜索、结果可直接行动 |
| 6 | **Workspace Layer** | workspace 聚合哪些资源、与 session/collection/bookmark 边界清楚、startup 是自然用户路径 |
| 7 | **Interaction System** | 用户知道自己在哪、能从任意资源域快速执行下一步、search/workspace/sync 有自然入口 |
| 8 | **Power Layer** | 核心对象和交互系统都已稳定，power features 不会把产品做散 |

**Phase ↔ Stage 映射**见 `11_PLAN.md` §Phased Implementation。

## 5. Feature List

### Stage 1 — Runtime Foundation

#### 基础架构
| # | Feature | 说明 |
|---|---------|------|
| F01 | 单二进制 | Go 编译为 `ctm`，cobra 子命令区分角色 |
| F02 | NDJSON 协议 | Unix socket NDJSON + 版本协商 |
| F03 | 持久客户端 | connect / request / subscribe / reconnect |
| F04 | NM Shim | Chrome NM 4-byte LE ↔ NDJSON 桥接 |
| F05 | Extension 嵌入 | Go embed + `ctm install` 解压 |

#### Tab 管理
| # | Feature | CLI | TUI |
|---|---------|-----|-----|
| F20 | Tab 列表 | `ctm tabs list` | Tabs view |
| F21 | Tab 打开 (deduplicate) | `ctm tabs open <url>` | `:open <url>` |
| F22 | Tab 关闭 (批量) | `ctm tabs close --tab-id` | `x` |
| F23 | Tab 激活 (focus) | `ctm tabs activate --tab-id --focus` | `Enter` |
| F24 | Tab 更新 (pinned) | `ctm tabs update` | — |
| F25 | Tab 搜索/过滤 | — | `/` |
| F26 | Tab 多选 | — | Space / Ctrl-A / u |
| F27 | Tab 复制 (y-chord) | — | yy/yn/yh/ym/yg |

#### Group 管理
| # | Feature | CLI | TUI |
|---|---------|-----|-----|
| F30 | Group 列表 | `ctm groups list` | Groups view |
| F31 | Group 创建 | `ctm groups create --title --tab-id` | `G` + title |
| F32 | Group 更新 | `ctm groups update` | `e` |
| F33 | Group 删除 | `ctm groups delete` | — |
| F34 | Group 展开/折叠 | — | `Enter` |

#### Target 管理
| # | Feature | CLI | TUI |
|---|---------|-----|-----|
| F60 | Target 列表 | `ctm targets list` | Targets view |
| F61 | Target 设为默认 | `ctm targets default <id>` | `d` |
| F62 | Target 清除默认 | `ctm targets clear-default` | — |
| F63 | Target 标签 | `ctm targets label <id> <label>` | `e` |
| F64 | Target 自动选择 | — | 自动 |

#### 实时事件
| # | Feature | 说明 |
|---|---------|------|
| F70 | 事件订阅 | pattern 匹配订阅 |
| F71 | Tab 事件 | tabs.created / removed / updated |
| F72 | Group 事件 | groups.created / updated / removed |
| F73 | 事件隔离 | 每个 event 含 `_target.targetId` |
| F74 | TUI 事件批量 | 150ms batch |
| F75 | Extension debounce | tabs.onUpdated per-tab 300ms |

### Stage 2 — Library Foundation

#### Session 管理
| # | Feature | CLI | TUI |
|---|---------|-----|-----|
| F40 | Session 保存 | `ctm sessions save <name>` | `n` + name |
| F41 | Session 列表 (summary) | `ctm sessions list` | Sessions view |
| F42 | Session 查看 (full) | `ctm sessions get <name>` | `Enter` 预览 |
| F43 | Session 恢复 (多窗口+groups) | `ctm sessions restore <name>` | `o` |
| F44 | Session 删除 (D-D 确认) | `ctm sessions delete <name>` | `D` `D` |
| F200 | 自动保存 | 每 N 分钟 `auto-<timestamp>` | — |
| F202 | 崩溃恢复 | daemon 异常退出前自动保存 | — |
| F222 | Session 快速切换 | `ctm sessions switch` | TUI |

#### Collection 管理
| # | Feature | CLI | TUI |
|---|---------|-----|-----|
| F50 | Collection 创建 | `ctm collections create <name>` | `n` + name |
| F51 | Collection 列表 (summary) | `ctm collections list` | Collections view |
| F52 | Collection 查看 (full) | `ctm collections get <name>` | `Enter` 展开 |
| F53 | Collection 添加 URL | `ctm collections add --url` | — |
| F54 | Collection 从 Tabs 添加 | `ctm collections add --from-tabs` | `a` + picker |
| F55 | Collection 移除 Item | `ctm collections remove` | `D` `D` |
| F56 | Collection 恢复 | `ctm collections restore <name>` | `o` |
| F57 | Collection 删除 (D-D) | `ctm collections delete <name>` | `D` `D` |

#### 通用 Library 能力
| # | Feature |
|---|---------|
| F215 | Markdown 导出 (session / collection) |
| F212 | 重复检测 + 一键合并 |
| F11 | `ctm migrate` 从 TS 版导入 |

### Stage 3 — Bookmarks

| # | Feature | 优先级 |
|---|---------|--------|
| F100 | 书签导入 (chrome.bookmarks.getTree) | 高 |
| F101 | TUI 树形视图 | 高 |
| F102 | 书签搜索 (title/url/tag) | 高 |
| F106 | 书签标签 (overlay) | 中 |
| F107 | 书签导出 (HTML/JSON/Markdown) | 中 |
| F108 | 书签实时同步 (事件推送) | 中 |
| F109 | BookmarkMirror 本地镜像 | 高 |
| F110 | BookmarkOverlay (tags/notes/aliases) | 高 |

### Stage 4 — Sync

| # | Feature | 优先级 |
|---|---------|--------|
| F150 | 持久对象 UUID + timestamps | 高 |
| F151 | iCloud 双向同步 | 高 |
| F152 | 冲突处理 (last-write-wins + conflict file) | 高 |
| F156 | TUI Sync view | 中 |
| F157 | `ctm sync repair` | 中 |
| F158 | Device 感知 | 低 |

### Stage 5 — Search

| # | Feature | 优先级 |
|---|---------|--------|
| F220 | 跨资源搜索 | 高 |
| F225 | 搜索 by tag | 高 |
| F226 | 搜索 by host/domain | 中 |
| F227 | SavedSearch | 中 |
| F228 | Smart Collection | 低 |
| F229 | 搜索结果直接行动 | 高 |

### Stage 6 — Workspace

| # | Feature | 优先级 |
|---|---------|--------|
| F205 | Workspace 创建 | 高 |
| F230 | Workspace 关联资源 | 高 |
| F231 | Workspace 启动 | 高 |
| F232 | Workspace 切换 | 高 |
| F233 | Workspace 模板 | 中 |
| F234 | Workspace 搜索 | 中 |

### Stage 7 — Interaction

| # | Feature |
|---|---------|
| F80 | 8+ 视图切换 |
| F81-F89 | Vim 导航 / 搜索过滤 / 命令面板 / 帮助覆盖层 / 三通道反馈 / chord 键 / 断线重连 UI |
| F90-F95 | GoReleaser / Homebrew tap / ctm install / ctm version / ctm doctor |

### Stage 8 — Power

| # | Feature | 优先级 |
|---|---------|--------|
| F206 | 拖拽排序 (Shift+j/k) | 中 |
| F211 | Tab 挂起/冻结 | 中 |
| F213 | Tab 排序 | 中 |
| F214 | Domain 分组 | 中 |
| F218 | 批量 Pin/Unpin | 中 |
| F240 | Automation hooks | 低 |
| F241 | Diagnostics / repair | 中 |

## 6. Anti-Patterns

如果出现以下倾向，产品正在退化回"tab 工具"：

- 只加 tabs/groups 操作，不扩 library
- bookmarks 当附属功能
- search 限制为单 view 过滤
- 不做 workspace
- 不做 sync status / conflict handling
- power features 永远"以后再说"
