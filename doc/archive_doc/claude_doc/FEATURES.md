# CTM — Feature Implementation List

具体功能点清单。产品方向和能力域定义见 `codex_doc/CAPABILITY_MAP.md`，建设顺序见 `codex_doc/BUILD_ORDER.md`。

---

## Stage 1 — Runtime Foundation

### 基础架构

| # | 功能 | 说明 |
|---|------|------|
| F01 | 单二进制 | Go 编译为 `ctm`，cobra 子命令区分角色 |
| F02 | NDJSON 协议 | Unix socket 上的 NDJSON 消息协议，带版本协商 |
| F03 | 持久客户端 | client 支持 connect/request/subscribe/reconnect |
| F04 | NM Shim | Chrome Native Messaging 4-byte LE ↔ NDJSON 桥接 |
| F05 | Extension 嵌入 | Go embed 嵌入 extension，`ctm install` 解压 |

### Tab 管理

| # | 功能 | 说明 | CLI | TUI |
|---|------|------|-----|-----|
| F20 | Tab 列表 | 列出当前 target 所有 tabs | `ctm tabs list` | Tabs view |
| F21 | Tab 打开 | 打开新 tab（支持 deduplicate） | `ctm tabs open <url>` | `:open <url>` |
| F22 | Tab 关闭 | 关闭指定 tab（TUI 支持批量） | `ctm tabs close --tab-id` | `x` |
| F23 | Tab 激活 | 切换到指定 tab（可选 focus 窗口前台） | `ctm tabs activate --tab-id --focus` | `Enter` |
| F24 | Tab 更新 | 修改 tab 属性（pinned 等） | `ctm tabs update` | — |
| F25 | Tab 搜索/过滤 | 按 title/url 即时过滤 | — | `/` 搜索 |
| F26 | Tab 多选 | Space 切换选中，Ctrl-A 全选，u 清除 | — | TUI |
| F27 | Tab 复制 | y-chord：yy(URL) yn(title) yh(host) ym(markdown) | — | TUI |

### Tab Group 管理

| # | 功能 | 说明 | CLI | TUI |
|---|------|------|-----|-----|
| F30 | Group 列表 | 列出所有 tab groups | `ctm groups list` | Groups view |
| F31 | Group 创建 | 从选中 tabs 创建 group | `ctm groups create --title --tab-id` | `G` + title |
| F32 | Group 更新 | 修改 title/color/collapsed | `ctm groups update` | `e` |
| F33 | Group 删除 | 解散 group（tabs 保留） | `ctm groups delete` | — |
| F34 | Group 展开/折叠 | TUI 中展开查看组内 tabs | — | `Enter` |

### Target 管理

| # | 功能 | 说明 | CLI | TUI |
|---|------|------|-----|-----|
| F60 | Target 列表 | 列出在线 targets | `ctm targets list` | Targets view |
| F61 | Target 设为默认 | 设置默认 target | `ctm targets default <id>` | `d` |
| F62 | Target 清除默认 | 清除默认 target | `ctm targets clear-default` | — |
| F63 | Target 标签 | 给 target 设置标签 | `ctm targets label <id> <label>` | `e` |
| F64 | Target 自动选择 | 单 target 自动选；多 target + default 选 default | — | 自动 |

### 实时事件

| # | 功能 | 说明 |
|---|------|------|
| F70 | 事件订阅 | 客户端注册 pattern 匹配订阅 |
| F71 | Tab 事件 | tabs.created / tabs.removed / tabs.updated |
| F72 | Group 事件 | groups.created / groups.updated / groups.removed |
| F73 | 事件隔离 | 每个 event 含 _target.targetId，TUI 按 target 过滤 |
| F74 | 事件批量 | TUI 侧 150ms batch，避免高频刷新 |
| F75 | Extension debounce | tabs.onUpdated per-tab 300ms debounce |

---

## Stage 2 — Library Foundation

### Session 管理

| # | 功能 | 说明 | CLI | TUI |
|---|------|------|-----|-----|
| F40 | Session 保存 | 保存当前 target 所有 tabs + groups + windows | `ctm sessions save <name>` | `n` + name |
| F41 | Session 列表 | 列出已保存 sessions（summary） | `ctm sessions list` | Sessions view |
| F42 | Session 查看 | 获取 session 完整数据 | `ctm sessions get <name>` | `Enter` 预览 |
| F43 | Session 恢复 | 恢复到指定 target（多窗口 + groups） | `ctm sessions restore <name>` | `o` |
| F44 | Session 删除 | 删除 session 文件（D-D 确认） | `ctm sessions delete <name>` | `D` `D` |
| F200 | 自动 Session 保存 | 每 N 分钟自动保存为 `auto-<timestamp>`，保留最近 K 个 | 自动 | — |
| F202 | 崩溃恢复 | daemon 异常退出前自动保存当前 tabs | 自动 | — |
| F221 | 恢复预览 | restore 前预览 session 内容（tab 列表 + 与当前 tabs 对比） | — | TUI |
| F222 | Session 快速切换 | 关闭当前所有 tabs + 恢复另一个 session（一步操作） | `ctm sessions switch` | TUI |
| F201 | Session 差异对比 | 比较两个 session 的差异（新增/删除的 tabs） | CLI | TUI |

### Collection 管理

| # | 功能 | 说明 | CLI | TUI |
|---|------|------|-----|-----|
| F50 | Collection 创建 | 创建空 collection | `ctm collections create <name>` | `n` + name |
| F51 | Collection 列表 | 列出所有 collections（summary） | `ctm collections list` | Collections view |
| F52 | Collection 查看 | 获取 collection 完整内容 | `ctm collections get <name>` | `Enter` 展开 |
| F53 | Collection 添加 URL | 直接添加 URL | `ctm collections add <name> --url` | — |
| F54 | Collection 从 Tabs 添加 | 从当前 tabs 选择添加 | `ctm collections add --from-tabs` | `a` + picker |
| F55 | Collection 移除 Item | 从 collection 移除链接 | `ctm collections remove` | `D` `D` |
| F56 | Collection 恢复 | 打开 collection 中所有 URL | `ctm collections restore <name>` | `o` |
| F57 | Collection 删除 | 删除整个 collection（D-D 确认） | `ctm collections delete <name>` | `D` `D` |
| F208 | 批量打开 | 从 collection 中选择部分 items 打开（而非全部恢复） | CLI | TUI |

### 通用 Library 能力

| # | 功能 | 说明 |
|---|------|------|
| F215 | Markdown 导出 | 将 session/collection 导出为 Markdown 链接列表 |
| F223 | Undo / 操作历史 | 关闭 tab 后可撤回（daemon 缓存最近 N 次关闭操作） |
| F212 | 重复检测 | 自动检测并高亮重复的 URL，提供一键合并 |
| F11 | 数据迁移 | `ctm migrate` 从 TS 版导入 sessions/collections |

---

## Stage 3 — Bookmarks

| # | 功能 | 说明 | 优先级 |
|---|------|------|--------|
| F100 | 书签导入 | 从 Chrome 读取完整书签树（chrome.bookmarks.getTree） | 高 |
| F101 | 书签列表 | TUI 树形视图浏览书签（文件夹 + 书签） | 高 |
| F102 | 书签搜索 | 按 title/url/tag 搜索 | 高 |
| F103 | 书签创建 | 终端中添加书签到 Chrome | 中 |
| F104 | 书签删除 | 终端中删除 Chrome 书签 | 中 |
| F105 | 书签移动 | 终端中移动书签到不同文件夹 | 低 |
| F106 | 书签标签 | 给书签打 tag（本地 overlay，不修改 Chrome 数据） | 中 |
| F107 | 书签导出 | 导出为 HTML/JSON/Markdown | 中 |
| F108 | 书签实时同步 | Chrome 书签变更 → 事件推送 → TUI 更新 | 中 |
| F109 | BookmarkMirror | Chrome 书签树在 CTM 本地的可搜索镜像 | 高 |
| F110 | BookmarkOverlay | CTM 对书签的增强层（tags/notes/aliases），不写回 Chrome | 高 |

---

## Stage 4 — Sync

| # | 功能 | 说明 | 优先级 |
|---|------|------|--------|
| F150 | 本地持久化 | 所有持久对象有 UUID `id` + `createdAt` + `updatedAt` | 高 |
| F151 | iCloud 同步 | `~/Library/Mobile Documents/com~ctm/` 双向同步 | 高 |
| F152 | 冲突处理 | file-level last-write-wins + 冲突文件保留 | 高 |
| F153 | Session 同步 | sessions 同步到 iCloud | 高 |
| F154 | Collection 同步 | collections 同步到 iCloud | 高 |
| F155 | Overlay 同步 | bookmark overlay 同步到 iCloud | 中 |
| F156 | Sync 状态 | TUI Sync view 显示同步状态、冲突、最近同步时间 | 中 |
| F157 | Sync 修复 | `ctm sync repair` 重建索引、解决冲突 | 中 |
| F158 | Device 感知 | 帮助用户理解资源来自哪台机器 | 低 |

---

## Stage 5 — Search

| # | 功能 | 说明 | 优先级 |
|---|------|------|--------|
| F220 | 跨资源搜索 | 一次搜索 tabs/sessions/collections/bookmarks/workspaces | 高 |
| F225 | 搜索 by tag | 按 tag 过滤所有资源 | 高 |
| F226 | 搜索 by host/domain | 按域名聚合搜索结果 | 中 |
| F227 | SavedSearch | 保存常用查询定义，可重复执行 | 中 |
| F228 | Smart Collection | 基于查询规则自动生成的动态 collection | 低 |
| F229 | 搜索结果直接行动 | 从搜索结果一键 activate/restore/open | 高 |

---

## Stage 6 — Workspace

| # | 功能 | 说明 | 优先级 |
|---|------|------|--------|
| F205 | Workspace 创建 | 创建新 workspace | 高 |
| F230 | Workspace 关联资源 | 关联 sessions/collections/bookmark folders/tags | 高 |
| F231 | Workspace 启动 | 一键恢复 workspace 关联的 session + 打开 collections | 高 |
| F232 | Workspace 切换 | 关闭当前所有 tabs + 恢复目标 workspace | 高 |
| F233 | Workspace 模板 | 预定义 workspace 模板，一键创建 | 中 |
| F234 | Workspace 搜索 | 在 workspace 范围内搜索资源 | 中 |
| F235 | Workspace 元数据 | tags/notes/defaultTarget | 中 |

---

## Stage 7 — Interaction

### Daemon

| # | 功能 | 说明 |
|---|------|------|
| F04 | Daemon（actor 模式） | 单 goroutine Hub，零锁状态管理 |
| F05 | flock 进程单例 | 排他锁防止多实例 |
| F06 | 优雅关闭 | SIGTERM → 停止 accept → 等待 in-flight → 清理 socket |
| F09 | LaunchAgent | `ctm install` 安装 launchd plist，开机自启 |
| F10 | CLI auto-start | CLI/TUI 连接失败时自动启动 daemon |

### TUI

| # | 功能 | 说明 |
|---|------|------|
| F80 | 8+ 视图切换 | Tabs/Groups/Sessions/Collections/Bookmarks/Workspaces/Search/Sync |
| F81 | Vim 导航 | j/k/gg/G/Ctrl-D/Ctrl-U |
| F82 | 搜索过滤 | / 进入，即时过滤，Enter 保留，Esc 清除 |
| F83 | 命令面板 | : 进入，支持 target/open/save/restore/help/quit |
| F84 | 帮助覆盖层 | ? 打开，从 keymap.go 自动生成内容 |
| F85 | 三通道反馈 | Toast（3s 消失）、Error（Esc 清除）、ConfirmHint（非确认键清除） |
| F86 | y-chord（复制） | yy/yn/yh/ym/yg |
| F87 | z-chord（过滤） | zg/zu/zp/zw/zi |
| F88 | D-D chord（删除确认） | 持久数据删除二次确认 |
| F89 | 断线重连 UI | 显示 disconnected → 自动重连 → 恢复 |

### 分发

| # | 功能 | 说明 |
|---|------|------|
| F90 | GoReleaser | darwin-arm64 + darwin-amd64 |
| F91 | Homebrew tap | `brew install user/tap/ctm` |
| F92 | ctm install | 一键安装 NM manifest + LaunchAgent + extension |
| F93 | ctm install --check | 验证安装完整性 |
| F94 | ctm version | 显示版本号（goreleaser ldflags 注入） |
| F95 | ctm doctor | 诊断安装状态 |
| F120 | Chrome Beta 支持 | 第二个 NM manifest + target label 自动识别 |
| F130 | Extension 上架 | Chrome Web Store 发布 |

---

## Stage 8 — Power

| # | 功能 | 说明 | 优先级 |
|---|------|------|--------|
| F206 | 拖拽排序 | TUI 中用 Shift+j/k 移动 tab 顺序 | 中 |
| F209 | Session 模板 | 预定义 session 模板，一键启动 | 中 |
| F211 | Tab 挂起/冻结 | 挂起不活跃 tabs 节省内存（chrome.tabs.discard） | 中 |
| F213 | Tab 排序 | 按 title/url/domain/last-accessed 排序 | 中 |
| F214 | Domain 分组 | 自动按 domain 将 tabs 归入 group | 中 |
| F218 | Pin/Unpin 批量操作 | TUI 中批量 pin/unpin 选中 tabs | 中 |
| F240 | Automation hooks | 自定义触发器和动作 | 低 |
| F241 | Diagnostics / repair | 修复安装、同步、数据一致性 | 中 |
