# CTM — TUI Interaction Guideline

交互设计的唯一来源 (source of truth)。Bubble Tea 架构。

## 1. Interaction Invariants

非协商条件。违反前必须先更新本文档：

1. **Esc** = 取消/返回/清除，不修改持久状态
2. **Enter** = 当前焦点项的主操作
3. **Space** = 选中/取消选中切换
4. **q** 在 Normal mode 退出；overlay/input mode 中无效
5. 全局视图切换键不抢占局部最有价值键位
6. 持久化数据删除必须 D-D 二次确认
7. 每次状态变更必须有可见反馈
8. Header 始终显示当前 target
9. 用户始终能回答："我在哪？什么被选中？Enter 会做什么？"
10. Header、Status bar、Help overlay、实际行为一致（单数据源 keymap.go）
11. **Cursor isolation**：数据更新只影响对应 view 的 cursor/selection

## 2. Views

### Phase 5: 5 Views

| View | 主要功能 |
|------|----------|
| Targets | 选择 target，设 default，编辑 label |
| Tabs | 浏览/搜索/选择/激活/关闭/分组/收藏 |
| Groups | 查看/展开/折叠/重命名/重色/解组 |
| Sessions | 保存/预览/恢复/删除 |
| Collections | 浏览/创建/展开/恢复/删除 |

### Phase 7+: 3 Additional Views

| View | 主要功能 |
|------|----------|
| Bookmarks | 树形浏览/搜索/tag/export |
| Workspaces | 创建/切换/管理 |
| Sync | 同步状态/冲突/修复 |

### View Switching

| 键 | 作用 |
|----|------|
| `Tab` / `Shift-Tab` | 下一个/上一个视图（循环） |
| `` ` `` | 切到 Targets |
| `1`-`4` | Tabs / Groups / Sessions / Collections |
| `5`-`7` | Bookmarks / Workspaces / Sync (Phase 7+) |

## 3. Global Keys (Normal mode)

| 键 | 功能 |
|----|------|
| `q` | 退出 |
| `Esc` | 关闭 overlay / 取消 / 清除 error |
| `?` | 帮助 overlay |
| `:` | 命令面板 |
| `/` | 搜索/过滤 |
| `r` | 刷新 |

## 4. List Navigation (通用)

| 键 | 功能 |
|----|------|
| `j`/`↓`, `k`/`↑` | 下移/上移 |
| `gg`/`Home`, `G`/`End` | 跳顶/跳底 |
| `Ctrl-D`/`Ctrl-U` | 下/上翻半页 |
| `Space` | 切换选中 |
| `Enter` | 主操作 |

`gg` 实现：第一个 `g` 设 `pendingG=true`，200ms 内再按 `g` → 跳顶；超时或其他键 → 取消。

## 5. View-Specific Keys

### Tabs View
| 键 | 功能 | 前提 |
|----|------|------|
| `Enter` | 激活 tab | 有焦点 |
| `x` | 关闭焦点/选中 tabs | — |
| `G` | 创建 group → title 输入 | 有选中 |
| `a` | 添加到 collection → picker | 有选中 |
| `o` | URL 详情/预览 | 有焦点 |
| `n` | 新 tab (`:open <url>`) | — |
| `u` | 清除选中 | 有选中 |
| `Ctrl-A` | 全选 | — |

### Groups View
| 键 | 功能 |
|----|------|
| `Enter` | 展开/折叠 group；子 tab 上激活 |
| `e` | 编辑 title/color |
| `u` | 解组 |
| `a` | 将选中 tabs 加入 group |

### Sessions View
| 键 | 功能 |
|----|------|
| `Enter` | 预览 session |
| `o` | 恢复 session |
| `n` | 保存新 session → name 输入 |
| `D` | 删除 (D-D 确认) |

### Collections View
| 键 | 功能 |
|----|------|
| `Enter` | 展开/折叠 |
| `o` | 子项打开 URL；collection 恢复全部 |
| `n` | 创建新 collection → name 输入 |
| `D` | 删除 (D-D 确认) |

### Targets View
| 键 | 功能 |
|----|------|
| `Enter` | 激活 target → 跳到 Tabs |
| `d` | 设为 default |
| `e` | 编辑 label |

### Bookmarks View (Phase 7)
| 键 | 功能 |
|----|------|
| `Enter` | 展开/折叠文件夹；书签上打开 URL |
| `h`/`←`, `l`/`→` | 折叠/展开（树导航） |
| `/` | 搜索 (title/url/tag) |
| `t` | 设置 tag |
| `n` | 添加 note |
| `a` | 添加到 collection |
| `r` | 重新镜像 |

### Workspaces View (Phase 8)
| 键 | 功能 |
|----|------|
| `Enter` | 展开 workspace |
| `o` | 切换到 workspace |
| `n` | 创建新 workspace |
| `e` | 编辑关联资源 |
| `D` | 删除 (D-D) |
| `a` | 关联 session/collection |

### Sync View (Phase 8)
| 键 | 功能 |
|----|------|
| `Enter` | 冲突详情 |
| `o` | 修复/解决冲突 |
| `r` | 手动触发同步 |
| `R` | 修复/重建 |

## 6. Chord Keys

### `y` — Yank/Copy
| 序列 | 功能 |
|------|------|
| `y` `y` | 复制 URL |
| `y` `n` | 复制 display name |
| `y` `h` | 复制 host/domain |
| `y` `m` | 复制 Markdown 链接 |
| `y` `g` | 复制 group/session/collection 为 Markdown 列表 |

按 `y` → StatusBar 显示后续键提示。合法键 → 复制 + Toast。非法键/2s 超时 → 取消。

### `z` — Filter
| 序列 | 功能 |
|------|------|
| `z` `g` | 已分组 tabs |
| `z` `u` | 未分组 tabs |
| `z` `p` | 已固定 tabs |
| `z` `w` | 当前窗口 tabs |
| `z` `i` | 隐藏/显示内部页面 |

再按同一组合 → 取消过滤。过滤激活时 header 显示标签。

### `D` — Confirm Delete
`D` → StatusBar 显示 `Press D again to delete "<name>"` → `D`(2s 内) → 执行删除 + Toast。非确认键/超时 → 取消。仅 Sessions/Collections/Workspaces view。

## 7. Input Modes State Machine

```
Normal
  ├─ /  → Filter ─── Enter → Normal (keep filter) / Esc → Normal (clear)
  ├─ :  → Command ─── Enter → Normal (execute) / Esc → Normal
  ├─ ?  → Help ─── Esc/q/? → Normal
  ├─ y  → Yank ─── y/n/h/m/g → Normal (copy) / other/timeout → Normal
  ├─ z  → ZFilter ─── g/u/p/w/i → Normal (filter) / other/timeout → Normal
  ├─ D  → ConfirmDelete ─── D → Normal (delete) / other/timeout → Normal
  ├─ G  → GroupTitle ─── Enter → Normal (create) / Esc → Normal
  ├─ n  → NameInput ─── Enter → Normal (save) / Esc → Normal
  └─ a  → CollectionPicker ─── Enter → Normal (add) / Esc → Normal
```

Bubble Tea 实现：`InputMode` 是 `AppState` 字段。`App.Update` 最外层 switch on `InputMode`。非 Normal mode 时全局键不传递。

## 8. Three-Channel Feedback

| Channel | 用途 | 清除方式 | 位置 |
|---------|------|----------|------|
| **Toast** | 操作成功 | 3s `tea.Tick` 自动消失 | StatusBar 右侧 |
| **Error** | 持久错误 | 用户 Esc | StatusBar 全宽红色 |
| **ConfirmHint** | D-D 确认提示 | 非确认键/2s 超时 | StatusBar 中央 |

优先级：Error > ConfirmHint > Toast > 默认 action hints。

## 9. Search & Filter

- `/` 进入搜索 → 输入框出现在列表上方 → 每次按键即时过滤
- Enter → 保留过滤，cursor 回到列表
- Esc → 清除过滤

| View | 搜索字段 |
|------|----------|
| Tabs | title, url |
| Groups | group title |
| Sessions | session name |
| Collections | collection name, 展开后 item title/url |

## 10. Command Palette (`:` mode)

| 命令 | 功能 |
|------|------|
| `:target` | 切到 Targets view |
| `:target default` | 设当前 target 为 default |
| `:open <url>` | 打开新 tab |
| `:save <name>` | 保存 session |
| `:restore <name>` | 恢复 session |
| `:help` | 帮助 |
| `:quit` / `:q` | 退出 |

支持前缀匹配和 Tab 补全。

## 11. Screen Layout

```
+------------------------------------------------------+
| CTM  [1]Tabs [2]Groups [3]Sessions [4]Collections  * |  Header
|                          target: work (stable)        |
+------------------------------------------------------+
| > filter text_                                        |  Filter (visible when active)
+------------------------------------------------------+
|   12345 [Work]  Getting Started with React...         |
|   12346 [Work]  React Hooks Documentation             |
| > 12347         GitHub - user/repo/pull/42            |  > = cursor
| x 12349         Stack Overflow - How to...            |  x = selected
+------------------------------------------------------+
| [Enter]activate [x]close [G]roup  1 selected  | OK    |  StatusBar
+------------------------------------------------------+
```

Header 固定 2 行，StatusBar 固定 1 行，中间自适应。

## 12. Review Gate

每次 TUI 变更必须同时更新：

1. 本文档的交互契约
2. `keymap.go` 键绑定
3. Help overlay 内容（从 keymap.go 自动生成）
4. Status bar 文本（从 keymap.go 自动生成）
5. teatest 测试

### Review Questions

1. 这个键是否可被发现（help/status bar 中显示）？
2. 同一键在其他 view 含义是否一致？
3. 当前 mode 是否对用户明显？
4. 用户能否用 Esc 退出？
5. 该操作在多 target 下是否安全？
6. 破坏性操作是否有确认步骤？
7. Header、status bar、help overlay 是否都更新了？

## 13. Anti-Patterns

- 同一键在不同 view 一个导航一个修改，无可见 mode 切换
- 持久删除用小写单键且无确认
- 隐藏快捷键（不在 help/status bar 中）
- 新增键绑定不更新测试和文档
- 非焦点 view 的数据更新修改 cursor/selection
- view 代码中硬编码按键描述（必须从 keymap.go 生成）
