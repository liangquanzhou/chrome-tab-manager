# CTM — TUI Interaction Guideline

本文件定义 **稳定的 TUI 交互约束**。

它不再手维护完整键位表；易变键位应以代码和运行时 help 为准。

Live sources:

- Key bindings / help surface: `internal/tui/keymap.go`
- TUI state machine / handlers: `internal/tui/app.go`
- 顶层交互模型与用户旅程: `04_INTERACTION.md`
- 历史完整 TUI 规范: `doc/archive/10_TUI_FULL.md`

---

## 1. Interaction Invariants

以下是不应轻易改变的交互契约：

1. `Esc` = 取消 / 返回 / 清除，不隐式修改持久状态
2. `Enter` = 当前焦点项的主操作
3. `Space` = 选中 / 取消选中切换
4. 持久化删除必须通过二次确认 chord（`D D` 或 `x x`）
5. 每次状态变更必须有可见反馈
6. 用户始终能回答：我在哪、当前选中了什么、`Enter` 会做什么
7. Header / Status / Help / 实际行为必须一致
8. 数据刷新只影响对应 view 的 cursor / selection（cursor isolation）
9. View 切换不应破坏其他 view 的局部状态

---

## 2. Scope Boundary

TUI 负责：

- 浏览大量资源
- 交互式筛选与整理
- 预览与上下文切换
- 高价值快捷操作

TUI 不负责：

- 完整脚本化批处理
- 安装与诊断的全部细节
- 重新定义 CLI / contract 语义

---

## 3. Views

当前 top-level views 由代码与 `19_CAPABILITY_MATRIX.md` 共同反映。

TUI 设计上分三类：

| View Class | Examples | 主要目标 |
|------------|----------|----------|
| Runtime | Targets, Tabs, Groups | 处理当前在线浏览器状态 |
| Library | Sessions, Collections, Bookmarks | 浏览与整理长期资源 |
| Global / Infra | Workspaces, Search, Sync, History, Downloads | 跨资源入口与系统状态 |

要求：

- 每个 view 有明确主操作
- 每个 view 的 destructive action 必须有确认
- 每个 view 的 empty state / error state / loading state 必须可辨认

---

## 4. Input Modes

TUI 的稳定模式边界应保持清晰：

| Mode | 目的 | 退出方式 |
|------|------|----------|
| Normal | 浏览与执行主操作 | 切换到其他 mode 或 quit |
| Filter | view-local filter / search input | `Enter` / `Esc` |
| Command | `:` command palette style command | `Enter` / `Esc` |
| NameInput | 命名、URL、label 等轻输入 | `Enter` / `Esc` |
| Help | 查看帮助 | `Esc` / `q` / `?` |
| Yank / Confirm / Z-like chord modes | 短暂二段式动作 | 第二键 / 超时 / `Esc` |

要求：

- mode 切换优先于普通 view key 处理
- chord mode 必须有超时和可见提示
- overlay / input mode 中，全局退出键不应误伤持久状态

---

## 5. Feedback Model

TUI 必须保持三通道反馈：

1. **Error** — 明确失败原因，可清除
2. **Confirm / Hint** — 当前 chord 或待确认动作
3. **Toast / Status** — 成功反馈、轻提示、状态变化

优先级：

`Error > ConfirmHint > Toast > Normal Status`

---

## 6. Layout Rules

- Header 持续显示当前 view 与 target 上下文
- Main content 聚焦单一资源域，不同时塞多个重交互区域
- Status bar 不承载隐藏业务语义，只显示状态与反馈
- Preview / detail panel 属于增强信息，不应抢走主列表的定位清晰度

---

## 7. Data Ownership

TUI 不是业务真相源。

- 业务语义来自 `12_CONTRACTS.md`
- 当前 capability 完整度来自 `19_CAPABILITY_MATRIX.md`
- TUI 只消费 daemon contract，不发明自己的业务模型

因此：

- 当 key binding 改变时，优先改 `keymap.go`
- 当动作语义改变时，优先改 `12_CONTRACTS.md`
- 当 surface 支持等级改变时，优先改 `19_CAPABILITY_MATRIX.md`

---

## 8. Review Gate

任何较大的 TUI 改动，至少回答：

1. 是否改变了 `Esc` / `Enter` / delete confirm 语义？
2. 是否破坏 Header / Help / 实际行为一致性？
3. 是否影响 cursor isolation？
4. 是否引入了新的 mode 或新的隐式状态？
5. 这个变化属于 TUI 表面，还是其实在改 contract / product model？

如果答案落在后两类，应同步回到 `04_INTERACTION.md`、`12_CONTRACTS.md` 或 `19_CAPABILITY_MATRIX.md`。
