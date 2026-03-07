# CTM — Lessons from TS Version

从 chrome-session-tui 9 轮 Codex review + 完整 M0-M6 开发过程中提炼的教训。
Go 重写时必须参考，避免重蹈覆辙。

## 1. Event Flow（4 轮才确认）

**问题**：TS 版事件通道经历 4 轮审查才最终稳定。根本原因是"谁发送、谁转发、谁消费"没有在动手前写清楚。

**教训**：
- 事件流向必须画出完整链路图：`Chrome API → extension → NM → daemon → subscriber`
- 每个 event payload 必须包含 `_target.targetId`，否则多 target 场景事件会混淆
- extension 侧 `chrome.tabs.onUpdated` 需要 per-tab 300ms debounce，否则 URL 导航期间会产生大量无意义事件
- daemon 侧 fanout 只发给匹配 pattern 的 subscribers，不广播全量

**Go 重写检查项**：
- [ ] Hub.Run 中 `eventCh` 的 fanout 逻辑是否带 pattern 匹配
- [ ] event 消息结构体是否包含 `_target` 字段
- [ ] TUI 侧 150ms batch 是否实现

## 2. API Contract（list vs get 的 itemCount 陷阱）

**问题**：`sessions.list` 和 `collections.list` 返回 SUMMARY（含 `tabCount`/`itemCount`），不含完整数组。TUI 曾直接用 list 返回值渲染详情，导致空白。

**教训**：
- list 返回 summary（counts），get 返回 full data（arrays）—— 这是刻意的设计，不是 bug
- AI 编码时如果不知道这个约定，会反复猜测 response shape
- 所有 API contract 必须写进 CONTRACTS.md，编码前先读

**Go 重写检查项**：
- [ ] CONTRACTS.md 中每个 action 的 response payload 是否精确定义
- [ ] daemon handler 是否严格按 CONTRACTS.md 返回
- [ ] TUI 中 list view 用 list 数据，detail/expand 用 get 数据

## 3. Multi-Window Session Restore（从未完全解决）

**问题**：TS 版 session restore 经历多次重写，核心难点是多窗口 + tab group 的恢复顺序。

**正确的恢复逻辑**（TS v9 最终版）：
1. 按 `windows[]` 逐个创建窗口（第一个 tab 的 URL 作为窗口初始 URL）
2. 追加后续 tabs 到该窗口
3. **所有 tabs 就位后**，再按 `groups[]` 创建 tab groups
4. tab 打开失败 → 跳过，不阻塞
5. group 创建失败 → 记 warn，不阻塞

**教训**：
- group 必须在 tab 全部创建完成后才能建立（Chrome API 限制：`chrome.tabs.group` 需要已存在的 tabId）
- 不要尝试并行创建 tab + group，这会导致竞态
- restore 结果要返回 `tabsFailed` 和 `groupsCreated` 计数，让用户知道是否有遗漏

**Go 重写检查项**：
- [ ] session restore handler 是否按 windows → tabs → groups 严格顺序执行
- [ ] 是否返回 `windowsCreated`, `tabsOpened`, `tabsFailed`, `groupsCreated`
- [ ] 单个 tab 失败是否跳过而非中断整个 restore

## 4. 多 useInput Handler 键冲突（Ink 特有，Go 已解决）

**问题**：Ink/React 中多个组件同时注册 `useInput`，导致全局键和局部键冲突。例如 `g` 在 TabBrowser 中是 "create group"，同时在 App 中是 "switch to groups view"。

**教训**：
- 根本原因是 Ink 的 `useInput` 没有优先级或排他机制
- TS 版通过 `isActive` flag 和 mode 判断来 workaround，但逻辑分散

**Go 优势**：
- Bubble Tea 的单入口 `Update(msg)` 天然消除了这个问题
- 但要注意：嵌套 Model 的 `Update` 调用链中，父 Model 必须先判断 mode，再决定是否传递 key 给子 Model

**Go 重写检查项**：
- [ ] App.Update 中是否先检查 InputMode，然后才路由 key 到子 Model
- [ ] 全局键（q, Esc, ?, :, /）是否在 mode != Normal 时被正确屏蔽

## 5. Cursor Isolation（跨 View 的 Cursor 污染）

**问题**：切换 view 后 cursor 位置异常。例如在 Tabs view 选中第 10 项，切到 Sessions view（只有 3 项），cursor 越界。

**教训**：
- Reducer action 更新某个 view 的数据时，只能修改该 view 的 cursor/selection
- 切换 view 时不重置 cursor（保留用户位置），但要 clamp 到合法范围

**Go 重写检查项**：
- [ ] 每个 view Model 独立维护自己的 cursor 和 selection
- [ ] view 切换时 clamp cursor（`min(cursor, len(items)-1)`）
- [ ] 数据刷新（SET_TABS 等）时 clamp cursor

## 6. 断线重连后的状态恢复

**问题**：TUI 断线重连后，事件订阅丢失，需要重新 subscribe。但 target 可能已变（daemon 重启导致 targetId 重新分配）。

**教训**：
- 重连后必须：1. 重新 subscribe  2. 重新获取 targets.list  3. 验证 selectedTargetId 是否仍有效
- 如果 selectedTargetId 不在新的 targets 列表中，回退到 target picker

**Go 重写检查项**：
- [ ] Client reconnect 后是否自动重新 subscribe
- [ ] TUI 收到 reconnect 信号后是否刷新 targets
- [ ] selectedTargetId 失效时是否切换到 target picker

## 7. Status Bar 与实际行为不一致

**问题**：status bar 显示的快捷键与实际行为不一致，是审查中反复出现的问题。

**教训**：
- Status bar、help overlay、实际键绑定必须从同一个数据源生成
- 不允许在 view 代码中硬编码按键描述字符串

**Go 重写检查项**：
- [ ] keymap.go 是唯一的键绑定注册表
- [ ] StatusBar 和 HelpOverlay 都从 `BindingsForView()` 生成
- [ ] 新增/修改键绑定时只改 keymap.go，不改 view 代码

## 8. 三通道反馈（从混乱到清晰）

**问题**：早期 TUI 只有一个 error 状态，导致成功提示和错误提示混用同一个位置。

**最终方案**：三通道分离
- **Toast**：操作成功反馈，3 秒自动消失（如 "Copied URL"）
- **Error**：持久错误，需要用户按 Esc 清除（如 "Target offline"）
- **ConfirmHint**：确认提示，按下非确认键时自动清除（如 "Press D again to delete"）

**Go 重写检查项**：
- [ ] AppState 是否有 Toast、Error、ConfirmHint 三个独立字段
- [ ] Toast 是否用 tea.Tick 实现自动清除
- [ ] ConfirmHint 是否在非 D 按键时清除

## 9. Chord 模式（y-, z-, D-D）

**问题**：单键快捷键不够用，但要避免 Ctrl 组合键（终端兼容性差）。

**最终方案**：chord prefix
- `y` → 进入 yank 模式 → `y`/`n`/`h`/`m` 完成操作
- `z` → 进入 filter 模式 → `g`/`u`/`p`/`w` 应用过滤
- `D` → 进入 confirm delete 模式 → `D` 确认删除

**教训**：
- chord 超时：如果进入 chord mode 后 2s 无操作，自动取消
- chord mode 下按任何非法键 → 取消并回到 Normal（不报错）
- 进入 chord mode 时 status bar 要显示可用的后续键

**Go 重写检查项**：
- [ ] InputMode 枚举是否包含 ModeYank、ModeZFilter、ModeConfirmDelete
- [ ] chord mode 是否有超时自动取消
- [ ] 进入 chord 时 StatusBar 是否显示后续键提示

## 10. Name Validation（路径遍历防御）

**问题**：session/collection name 直接拼成文件路径，如果 name 包含 `../` 或 `/`，可能写到预期目录之外。

**教训**：
- name 必须做 allowlist 校验：`^[a-zA-Z0-9_-]+$`
- 不允许空 name、不允许超长 name（128 字符上限）
- daemon 侧校验，不信任客户端

**Go 重写检查项**：
- [ ] daemon handler 收到 name 后是否先 validate
- [ ] 是否用正则 allowlist 而非 blocklist
- [ ] 文件路径拼接是否用 `filepath.Join` 并检查结果在预期目录内

## 11. 原子文件写入

**问题**：直接写文件 + crash = 文件损坏。TS 版早期有用户报告 session 文件变成空文件。

**教训**：
- 写入流程：tmp file → 写内容 → fsync → rename to target
- rename 是原子的（POSIX 保证），中间 crash 只会丢 tmp 文件
- Go 标准库的 `os.Rename` 满足这个需求

**Go 重写检查项**：
- [ ] `atomicWriteJSON` 是否按 tmp → fsync → rename 实现
- [ ] tmp 文件是否在同一目录下（跨文件系统 rename 会失败）
- [ ] 是否处理了 tmp 文件的清理（如果 rename 前 crash，下次启动时清理 `.tmp` 文件）

## 12. Extension 与 Daemon 版本错配

**问题**：extension 和 daemon 独立更新时，协议不兼容导致诡异错误。

**教训**：
- hello 握手中必须协商 protocol_version
- 不兼容时 daemon 应返回明确的 `PROTOCOL_MISMATCH` 错误，附带人类可读的升级提示
- `ctm install --check` 应该检测 extension 版本与 binary 版本是否匹配

**Go 重写检查项**：
- [ ] hello handler 是否检查 protocol_version
- [ ] PROTOCOL_MISMATCH error 是否包含 "请运行 ctm install 更新" 类似提示
- [ ] `ctm install --check` 是否验证 extension manifest 中的版本号
