# CTM — TUI Interaction Guideline

交互设计的唯一来源（source of truth）。
目标：让 Go/Bubble Tea 版 TUI 从第一天就有清晰、可测试的交互契约。

改编自 TS 版 TUI_GUIDELINE_V1.md，适配 Bubble Tea 架构。

## 1. 交互不变量

非协商条件。任何代码修改不得违反以下规则（除非先更新本文档）：

1. **Esc** 永远是取消/返回/清除，不修改任何持久状态
2. **Enter** 永远是当前焦点项的主操作
3. **Space** 永远是选中/取消选中切换
4. **q** 在 Normal mode 退出应用；在 overlay/input 模式中无效（用 Esc）
5. 全局视图切换键不抢占局部操作的最有价值键位
6. 持久化数据的删除必须有二次确认（D-D chord）
7. 每次状态变更必须有可见反馈：行变化、计数变化、toast、error bar
8. Header 始终显示当前 target
9. 用户始终能回答三个问题："我在哪？什么被选中？Enter 会做什么？"
10. Header、Status bar、Help overlay、实际行为必须一致（单数据源 keymap.go）
11. **Cursor isolation**：数据更新只影响对应 view 的 cursor/selection

## 2. View 架构

### 2.1 顶级 View（v1: 5 个，v2+: 8 个）

| View | 主要功能 | Phase |
|------|----------|-------|
| Targets | 选择运行时 target，设置 default，编辑 label | P5 |
| Tabs | 浏览/搜索/选择/激活/关闭/分组/收藏 tabs | P5 |
| Groups | 查看 tab groups，展开/折叠，重命名/重色/解组 | P5 |
| Sessions | 保存/预览/恢复/删除 sessions | P5 |
| Collections | 浏览/创建/展开/恢复/删除 collections | P5 |
| Bookmarks | 树形浏览/搜索/tag/export Chrome 书签 | P7 |
| Workspaces | 创建/切换/管理 workspace | P8 |
| Sync | 同步状态/冲突/修复 | P8 |

### 2.2 视图切换键

| 键 | 作用 |
|----|------|
| `Tab` | 下一个视图（循环：Tabs → Groups → Sessions → Collections → Bookmarks → ...） |
| `Shift-Tab` | 上一个视图 |
| `` ` `` | 切到 Targets（低频操作，放在非数字键） |
| `1` | Tabs |
| `2` | Groups |
| `3` | Sessions |
| `4` | Collections |
| `5` | Bookmarks（Phase 7+） |
| `6` | Workspaces（Phase 8+） |
| `7` | Sync（Phase 8+） |

**设计理由**：
- Targets 极少切换，`` ` `` 避免浪费数字键位且避免误按
- 数字键直达，Tab/Shift-Tab 顺序切换，两种模式覆盖不同使用习惯
- 释放 `t`/`g`/`s`/`c` 给局部操作

## 3. 全局键

Normal mode 下任何 view 都生效（overlay/input 打开时不生效）：

| 键 | 功能 |
|----|------|
| `q` | 退出应用 |
| `Esc` | 关闭 overlay / 取消操作 / 清除 error / 返回上一层 |
| `?` | 打开帮助 overlay |
| `:` | 打开命令面板 |
| `/` | 进入搜索/过滤模式 |
| `Tab` / `Shift-Tab` | 切换视图 |
| `1`-`4`, `` ` `` | 直达视图 |
| `r` | 刷新当前视图 |

## 4. 列表导航（所有列表通用）

| 键 | 功能 |
|----|------|
| `j` / `↓` | 下移 |
| `k` / `↑` | 上移 |
| `gg` / `Home` | 跳到顶部 |
| `G` / `End` | 跳到底部（注意：Tabs view 中 `G` 是 create group，用 `End` 替代） |
| `Ctrl-D` | 下翻半页 |
| `Ctrl-U` | 上翻半页 |
| `Space` | 切换选中 |
| `Enter` | 主操作 |

**Bubble Tea 实现注意**：
- `gg` 是两次 `g`，需要实现简单的 key sequence detection
- 方案：第一个 `g` 设置 `pendingG = true`，200ms 内再按 `g` → 跳顶；超时或按其他键 → 取消

## 5. View 局部键

### 5.1 Tabs View

| 键 | 功能 | 前提条件 |
|----|------|----------|
| `Enter` | 激活当前 tab | 有焦点 tab |
| `x` | 关闭焦点 tab 或已选 tabs | 有焦点或选中 |
| `G` | 从选中 tabs 创建 group → 弹出 title 输入 | 有选中 tabs |
| `a` | 将选中 tabs 添加到 collection → 弹出 picker | 有选中 tabs |
| `o` | 打开 URL 详情 / 预览 | 有焦点 tab |
| `n` | 打开新 tab（命令面板 `:open <url>`） | — |
| `u` | 清除所有选中 | 有选中 |
| `Ctrl-A` | 全选 | — |

### 5.2 Groups View

| 键 | 功能 | 前提条件 |
|----|------|----------|
| `Enter` | 展开/折叠 group；在子 tab 上则激活该 tab | — |
| `e` | 编辑 group title/color | 焦点在 group 上 |
| `u` | 解组选中 tabs 或焦点子 tab | 有选中或焦点 |
| `a` | 将选中 tabs 加入焦点 group | 有选中 tabs |

### 5.3 Sessions View

| 键 | 功能 | 前提条件 |
|----|------|----------|
| `Enter` | 预览/检查 session 内容 | 有焦点 session |
| `o` | 恢复 session 到当前 target | 有焦点 session |
| `n` | 保存当前 target 为新 session → 弹出 name 输入 | target 已选 |
| `D` | 删除 session（D-D 确认） | 有焦点 session |

### 5.4 Collections View

| 键 | 功能 | 前提条件 |
|----|------|----------|
| `Enter` | 展开/折叠 collection | 有焦点 |
| `o` | 在子项上打开单个 URL；在 collection 上恢复全部 | — |
| `n` | 创建新 collection → 弹出 name 输入 | — |
| `D` | 删除 collection 或选中 item（D-D 确认） | 有焦点 |

### 5.5 Targets View

| 键 | 功能 | 前提条件 |
|----|------|----------|
| `Enter` | 激活 target 并跳转到 Tabs view | 有焦点 target |
| `d` | 设置焦点 target 为 default | 有焦点 target |
| `e` | 编辑 target label | 有焦点 target |

## 6. Chord 键族

### 6.1 `y` — Yank/Copy 族

进入 `ModeYank` 后，按下一个键完成操作：

| 序列 | 功能 |
|------|------|
| `y` `y` | 复制焦点 URL |
| `y` `n` | 复制焦点 display name（tab title / session name / collection name） |
| `y` `h` | 复制焦点 host/domain |
| `y` `m` | 复制焦点为 Markdown 链接 `[title](url)` |
| `y` `g` | 复制当前 group/session/collection 为 Markdown 链接列表 |

**行为规则**：
- 按 `y` → StatusBar 显示 `[y]url [n]ame [h]ost [m]arkdown [g]roup`
- 按合法后续键 → 复制到剪贴板 + Toast "Copied: ..."
- 按非法键或 2s 超时 → 取消，回到 Normal，无反馈
- Esc → 取消

### 6.2 `z` — Filter 族

进入 `ModeZFilter` 后，按下一个键切换过滤器：

| 序列 | 功能 |
|------|------|
| `z` `g` | 只显示已分组 tabs |
| `z` `u` | 只显示未分组 tabs |
| `z` `p` | 只显示已固定 tabs |
| `z` `w` | 只显示当前窗口 tabs |
| `z` `i` | 隐藏/显示内部页面（chrome://, extension pages） |

**行为规则**：
- 按 `z` → StatusBar 显示 `[g]rouped [u]ngrouped [p]inned [w]indow [i]nternal`
- 按合法键 → 应用过滤 + Toast "Filter: pinned only"
- 再按同一个 `z`+键 → 取消该过滤
- 过滤激活时 header 显示过滤标签

### 6.3 `D` — Confirm Delete

| 序列 | 功能 |
|------|------|
| `D` `D` | 确认删除（session/collection） |

**行为规则**：
- 第一个 `D` → StatusBar 显示 `Press D again to delete "<name>"`（ConfirmHint 通道）
- 第二个 `D`（2s 内）→ 执行删除 + Toast "Deleted: ..."
- 按其他键或 2s 超时 → 取消，清除 ConfirmHint
- 仅在 Sessions/Collections view 生效

## 7. Input Modes 状态机

```
Normal
  ├─ /  → Filter   ─── Enter → Normal (keep filter)
  │                 ─── Esc   → Normal (clear filter)
  ├─ :  → Command  ─── Enter → Normal (execute)
  │                 ─── Esc   → Normal
  ├─ ?  → Help     ─── Esc/q/?  → Normal
  ├─ y  → Yank     ─── y/n/h/m/g → Normal (copy + toast)
  │                 ─── other/timeout → Normal
  ├─ z  → ZFilter  ─── g/u/p/w/i → Normal (apply filter)
  │                 ─── other/timeout → Normal
  ├─ D  → ConfirmDelete ─── D → Normal (delete + toast)
  │                      ─── other/timeout → Normal
  ├─ G  → GroupTitle ─── Enter → Normal (create group)
  │                   ─── Esc   → Normal
  ├─ n  → NameInput  ─── Enter → Normal (save/create)
  │   (session/collection)  Esc → Normal
  └─ a  → CollectionPicker ─── Enter → Normal (add to collection)
                            ─── Esc   → Normal
```

**Bubble Tea 实现**：
- `InputMode` 是 `AppState` 的字段
- `App.Update` 最外层 switch on `InputMode`
- 非 Normal mode 时，全局键（q/1-4/Tab 等）不传递
- 每个 mode 对应一个 `handleXxxInput(msg tea.KeyMsg) (tea.Model, tea.Cmd)` 方法

## 8. 三通道反馈系统

| 通道 | 用途 | 清除方式 | 位置 |
|------|------|----------|------|
| Toast | 操作成功反馈 | 3s 自动消失（`tea.Tick`） | StatusBar 右侧 |
| Error | 持久错误 | 用户按 `Esc` | StatusBar 全宽，红色背景 |
| ConfirmHint | 确认提示（D-D） | 按非确认键或 2s 超时 | StatusBar 中央 |

**优先级**（当多通道同时有内容时）：
1. Error（最高优先级，遮盖其他）
2. ConfirmHint
3. Toast
4. 默认 action hints

## 9. 搜索与过滤

### 9.1 `/` 搜索模式

- 进入：按 `/`
- 输入框出现在列表上方
- 每次按键即时过滤（不等 Enter）
- Enter：保留过滤，退出搜索框，cursor 回到列表
- Esc：清除过滤，退出搜索框

### 9.2 搜索范围

| View | 搜索字段 |
|------|----------|
| Tabs | title, url |
| Groups | group title |
| Sessions | session name |
| Collections | collection name, 展开后搜 item title/url |

### 9.3 搜索后导航

- `n` / `N`：在搜索结果中跳转到下一个/上一个匹配
- 注意：Sessions view 中 `n` 是 "new session"，此时搜索导航用 `Ctrl-N`/`Ctrl-P`

## 10. 命令面板（`:` 模式）

初始命令集：

| 命令 | 功能 |
|------|------|
| `:target` | 切到 Targets view |
| `:target default` | 设置当前 target 为 default |
| `:open <url>` | 打开新 tab |
| `:save <name>` | 保存当前 target 为 session |
| `:restore <name>` | 恢复 session |
| `:help` | 打开帮助 |
| `:quit` / `:q` | 退出 |

命令面板支持前缀匹配和 Tab 补全。

## 11. Help Overlay

按 `?` 打开全屏帮助覆盖层，显示：

1. **全局键**
2. **当前 view 的局部键**
3. **Chord 族**（y-, z-, D-D）
4. **当前状态**：target、view、filter、connection status

内容从 `keymap.go` 的 `AllBindings` 自动生成，不硬编码。

## 12. Status Bar 规则

- 只显示当前 view + 当前 mode 中**实际可用**的操作
- 选中计数 > 0 时，显示 `N selected`
- Loading 时显示 spinner（用 bubbles 的 spinner 组件）
- 反映焦点行类型：如 Groups view 中焦点在 group 行 vs tab 行，Enter 的含义不同

## 13. 屏幕布局

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
|   12350         npm - package search                  |
+------------------------------------------------------+
| [Enter]activate [x]close [G]roup  1 selected  | OK    |  StatusBar
+------------------------------------------------------+
```

Header 固定 2 行，StatusBar 固定 1 行，中间区域自适应。

## 14. Review Gate

每次 TUI 变更必须同时更新：

1. 本文档的交互契约
2. `keymap.go` 的键绑定
3. Help overlay 内容（自动从 keymap.go 生成）
4. Status bar 文本（自动从 keymap.go 生成）
5. teatest 测试

五项缺一不可。

### 14.1 Review 问题清单

1. 这个键是否可被发现（help/status bar 中有显示）？
2. 同一个键在其他 view 中的含义是否一致？
3. 当前 mode 是否对用户明显？
4. 用户能否用 Esc 退出？
5. 该操作在多 target 场景下是否安全？
6. 破坏性操作是否有确认步骤？
7. Header、status bar、help overlay 是否都更新了？

## 15. Phase 7+ View 契约

### 15.1 Bookmarks View（Phase 7）

书签是树结构，与其他 view 的扁平列表不同。

| 键 | 功能 | 前提条件 |
|----|------|----------|
| `Enter` | 展开/折叠文件夹；在书签上打开 URL | — |
| `/` | 搜索书签（跨 title/url/tag） | — |
| `t` | 给焦点书签设置 tag | 有焦点 |
| `n` | 给焦点书签添加 note | 有焦点 |
| `a` | 将焦点书签添加到 collection | 有焦点 |
| `y` `y` | 复制书签 URL | 有焦点书签 |
| `y` `m` | 复制为 Markdown 链接 | 有焦点书签 |
| `y` `g` | 复制文件夹为 Markdown 链接列表 | 有焦点文件夹 |
| `r` | 重新从 Chrome 镜像书签树 | — |

**树形导航补充**：
- `h` / `←` — 折叠当前文件夹或跳到父节点
- `l` / `→` — 展开当前文件夹或进入第一个子节点

### 15.2 Workspaces View（Phase 8）

| 键 | 功能 | 前提条件 |
|----|------|----------|
| `Enter` | 展开 workspace（显示关联的 sessions/collections/bookmarks） | 有焦点 |
| `o` | 切换到 workspace（关闭当前 tabs + 恢复） | 有焦点 workspace |
| `n` | 创建新 workspace → 弹出 name 输入 | — |
| `e` | 编辑 workspace（修改关联资源） | 有焦点 workspace |
| `D` | 删除 workspace（D-D 确认） | 有焦点 workspace |
| `a` | 将当前 session/collection 关联到 workspace | 有焦点 workspace |

### 15.3 Sync View（Phase 8）

| 键 | 功能 | 前提条件 |
|----|------|----------|
| `Enter` | 查看冲突详情 | 有冲突项 |
| `o` | 修复/解决冲突 | 有冲突项 |
| `r` | 手动触发同步 | — |
| `R` | 修复/重建同步状态 | — |

## 16. Anti-Patterns

不要做这些事：

- 同一个键在不同 view 中一个是导航、一个是修改，且没有可见的 mode 切换
- 持久删除用小写单键且无确认
- 隐藏快捷键（不在 help 或 status bar 中显示）
- 新增键绑定但不更新测试和文档
- 在非焦点 view 的数据更新中修改 cursor/selection（cursor isolation 违反）
- 在 view 代码中硬编码按键描述字符串（必须从 keymap.go 生成）
