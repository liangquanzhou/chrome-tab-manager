# CTM — Command Surfaces

## 1. Purpose

这份文档定义 CTM 的**动作表面模型**。

它回答的是：

- 哪些动作应该属于 CLI
- 哪些动作应该属于 TUI
- 哪些动作应该属于 command palette
- 同一个能力在不同表面如何协作，而不是互相打架

这份文档的目标是防止产品出现：

- CLI 什么都能做但不好用
- TUI 什么都塞进去但越来越重
- command palette 变成第二套半成品产品

---

## 2. Surface Philosophy

CTM 不只有一个交互面，而是三种主要 command surface：

1. `CLI`
2. `TUI`
3. `Command Palette`

它们不是重复实现，而应分工明确。

### 基本原则

- **CLI** 负责可脚本化、显式、可组合的动作
- **TUI** 负责探索、浏览、选择、批量交互
- **Command Palette** 负责高频、跨域、即时命令入口

---

## 3. CLI Surface

CLI 是 CTM 的显式控制面。

### 最适合 CLI 的动作

- 明确的资源操作
- 批量参数化操作
- 机器可调用动作
- 导出 / 导入 / 诊断
- 自动化脚本接入

### Typical CLI strengths

- `ctm tabs open <url>`
- `ctm sessions save <name>`
- `ctm collections restore <name>`
- `ctm search query "<term>"`
- `ctm workspaces startup <name>`
- `ctm sync status`

### CLI role in product

- 可自动化
- 可复现
- 可复制到脚本和 shell 历史里

### CLI should be the primary surface for

- install / check / doctor
- export / import
- sync repair
- batch operations
- structured output

---

## 4. TUI Surface

TUI 是 CTM 的探索与组织面。

### 最适合 TUI 的动作

- 浏览大量资源
- 多选
- 过滤
- 预览
- 交互式整理
- 上下文切换

### Typical TUI strengths

- 在 tabs 里筛选并批量处理
- 在 groups 里查看结构
- 在 sessions 里预览并恢复
- 在 bookmarks 树里探索
- 在 workspaces 里进入上下文

### TUI role in product

- 提供高密度信息界面
- 帮助用户在不知道精确命令时完成操作
- 承担主要的“组织”任务

### TUI should be the primary surface for

- resource browsing
- selection workflows
- multi-step curation
- workspace-oriented navigation

---

## 5. Command Palette Surface

Command Palette 是 TUI 内的全局快捷入口。

它不是独立产品，也不是完整 CLI 替代品。

### 最适合 palette 的动作

- 高频全局动作
- 跨资源域跳转
- 轻量命令执行
- 不适合绑专门热键的动作

### Typical palette strengths

- `open <url>`
- `target`
- `save <name>`
- `restore <name>`
- `workspace <name>`
- `search <term>`
- `help`
- `quit`

### Palette role in product

- 缩短“我知道我要做什么”到“我立刻执行”的路径
- 作为全局入口，而不是资源管理主界面

### Palette should not become

- 第二套完整 CLI
- 第二套隐藏交互系统
- 与 keymap 平行的完整产品

---

## 6. Surface-by-Capability Mapping

下面这张表定义不同能力更适合落在哪个表面。

| Capability | Primary surface | Secondary surface |
|------------|-----------------|------------------|
| Runtime tab control | TUI | CLI |
| Group management | TUI | CLI |
| Session save/restore | CLI + TUI | Palette |
| Collection curation | TUI | CLI |
| Bookmark browse | TUI | CLI |
| Bookmark search | Search/TUI | CLI |
| Unified search | TUI + Palette | CLI |
| Workspace startup | TUI + CLI | Palette |
| Sync status | TUI + CLI | — |
| Sync repair | CLI | TUI |
| Export | CLI | TUI |
| Diagnostics | CLI | TUI |

---

## 7. What Each Surface Should Optimize For

## 7.1 CLI optimizes for

- precision
- scripting
- reproducibility
- automation
- structured output

## 7.2 TUI optimizes for

- discoverability
- exploration
- context
- multi-selection
- high-density visibility

## 7.3 Command Palette optimizes for

- speed
- cross-domain jump
- command recall
- low-friction access

---

## 8. Surface Boundaries

为了防止功能越做越乱，必须明确边界。

### CLI boundary

CLI 不需要承担：

- 大量视觉探索
- 复杂树形浏览体验
- 重交互选择器

### TUI boundary

TUI 不需要承担：

- 所有高级批处理脚本入口
- 全部安装/诊断/修复细节
- 过多低频长命令输入

### Palette boundary

Palette 不需要承担：

- 完整的批量资源管理
- 深层编辑流程
- 复杂多步 wizard

---

## 9. Surface Consistency Rules

为了让三个表面不割裂，CTM 应始终遵守这些规则：

### Rule 1

同一个核心动作在不同 surface 上应有同样的业务含义。

### Rule 2

CLI 和 TUI 应共享同一套资源命名空间：

- targets
- tabs
- groups
- sessions
- collections
- bookmarks
- workspaces
- sync

### Rule 3

Palette 应复用已有动作模型，而不是发明新的隐式语义。

### Rule 4

TUI 中的高频动作可以有快捷键，但必须与 palette 和 CLI 的能力边界对齐。

---

## 10. Surface Entry Points

完整产品中，三种 surface 的典型入口应是：

### CLI entry

- 用户已经知道自己要做什么
- 需要脚本化 / 可复制 / 可自动化

### TUI entry

- 用户知道自己要处理哪类资源
- 但需要浏览、选择、对比、整理

### Palette entry

- 用户在 TUI 里，知道自己要执行一个全局动作
- 希望不离开当前上下文

---

## 11. Examples of Proper Distribution

### Example A — Open a URL

- CLI: `ctm tabs open <url>`
- TUI: palette 中 `open <url>`
- TUI hotkey: 可有，但不必成为主路径

### Example B — Restore a workspace

- CLI: `ctm workspaces startup <name>`
- TUI: Workspaces view 里选择并启动
- Palette: `workspace <name>` 或 `startup <name>`

### Example C — Diagnose sync problem

- CLI: `ctm sync doctor`
- TUI: Sync view 里查看状态
- Palette: 不应承载复杂诊断本身，只可跳转到 sync view

---

## 12. Surface Red Flags

如果未来出现下面这些情况，说明 command surfaces 开始失控：

- palette 开始复制完整 CLI
- TUI 承担太多安装/修复命令
- CLI 里塞了大量本该在 TUI 中浏览完成的交互
- 同一个动作在三个 surface 上名字和语义都不一致

---

## 13. Final Surface Statement

CTM 的 command surfaces 应始终分工明确：

- **CLI** = explicit control and automation
- **TUI** = exploration and organization
- **Command Palette** = global shortcut layer inside TUI

三个表面共同构成完整产品，但各自承担不同的产品职责。
