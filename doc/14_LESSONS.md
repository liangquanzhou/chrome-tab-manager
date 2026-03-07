# CTM — Lessons from TS Version

TS 版 9 轮 Codex review + 完整 M0-M6 开发提炼的教训。Go 重写必须参考。

## 1. Event Flow（4 轮才确认）

**问题**：事件通道 4 轮审查才稳定。"谁发送、谁转发、谁消费"没提前写清。

**教训**：
- 完整链路图：`Chrome API → extension → NM → daemon → subscriber`
- 每个 event payload 必须含 `_target.targetId`（多 target 事件隔离）
- Extension `tabs.onUpdated` per-tab 300ms debounce
- Daemon fanout 只发给匹配 pattern 的 subscribers

**Go 检查项**：
- [ ] Hub.Run eventCh fanout 带 pattern 匹配
- [ ] event 消息含 `_target` 字段
- [ ] TUI 侧 150ms batch

## 2. API Contract（list vs get 陷阱）

**问题**：`sessions.list` 返回 summary (tabCount)，TUI 曾直接用 list 渲染详情导致空白。

**教训**：list = summary (counts)，get = full data (arrays)。这是刻意设计。

**Go 检查项**：
- [ ] 12_CONTRACTS.md 每个 action response 精确定义
- [ ] Daemon handler 严格按 12_CONTRACTS.md 返回
- [ ] TUI list view 用 list 数据，detail/expand 用 get 数据

## 3. Multi-Window Session Restore

**问题**：多窗口 + tab group 恢复顺序经历多次重写。

**正确逻辑** (TS v9 最终版)：
1. 按 `windows[]` 逐个创建窗口（第一个 tab URL 作窗口初始 URL）
2. 追加后续 tabs
3. **所有 tabs 就位后**再创建 groups（Chrome API 限制）
4. Tab 失败 → 跳过；Group 失败 → warn
5. 返回 `tabsFailed` 和 `groupsCreated` 计数

**Go 检查项**：
- [ ] Restore handler 按 windows → tabs → groups 严格顺序
- [ ] 返回 windowsCreated, tabsOpened, tabsFailed, groupsCreated
- [ ] 单 tab 失败不中断整个 restore

## 4. 键冲突（Ink 特有，Go 已解决）

**问题**：Ink 多组件同时 `useInput` 导致全局键和局部键冲突。

**Go 优势**：Bubble Tea 单入口 `Update` 天然消除。但父 Model 必须先判断 mode 再传递 key 给子 Model。

**Go 检查项**：
- [ ] App.Update 先检查 InputMode，再路由 key
- [ ] 全局键在 mode != Normal 时正确屏蔽

## 5. Cursor Isolation

**问题**：切换 view 后 cursor 越界。

**教训**：每个 view 独立 cursor/selection。切换 view 时 clamp 不重置。

**Go 检查项**：
- [ ] 每个 view Model 独立 cursor 和 selection
- [ ] View 切换时 `min(cursor, len(items)-1)`
- [ ] 数据刷新时 clamp cursor

## 6. 断线重连后状态恢复

**问题**：重连后事件订阅丢失，target 可能变。

**教训**：重连后 1) 重新 subscribe 2) 重新 targets.list 3) 验证 selectedTargetId

**Go 检查项**：
- [ ] Client reconnect 后自动重新 subscribe
- [ ] TUI 收到 reconnect 后刷新 targets
- [ ] selectedTargetId 失效时切到 target picker

## 7. Status Bar 与实际行为不一致

**教训**：Status bar、help overlay、实际键绑定必须从同一数据源生成。

**Go 检查项**：
- [ ] keymap.go 是唯一键绑定注册表
- [ ] StatusBar 和 HelpOverlay 从 `BindingsForView()` 生成
- [ ] 新增/修改键绑定只改 keymap.go

## 8. 三通道反馈

**最终方案**：Toast (3s自动消) / Error (Esc清除) / ConfirmHint (非确认键清除)

**Go 检查项**：
- [ ] AppState 有 Toast、Error、ConfirmHint 三个独立字段
- [ ] Toast 用 tea.Tick 自动清除
- [ ] ConfirmHint 在非 D 键时清除

## 9. Chord 模式

**最终方案**：`y`(yank) / `z`(filter) / `D`(confirm delete) 前缀。2s 超时自动取消。

**Go 检查项**：
- [ ] InputMode 含 ModeYank、ModeZFilter、ModeConfirmDelete
- [ ] Chord mode 有超时自动取消
- [ ] 进入 chord 时 StatusBar 显示后续键提示

## 10. Name Validation（路径遍历防御）

**教训**：name 做 allowlist `^[a-zA-Z0-9_-]+$`，不允许空/超长(128)。Daemon 侧校验。

**Go 检查项**：
- [ ] Daemon handler 收 name 后先 validate
- [ ] 正则 allowlist
- [ ] `filepath.Join` + 检查结果在预期目录内

## 11. 原子文件写入

**教训**：tmp file → 写内容 → fsync → rename。中间 crash 只丢 tmp。

**Go 检查项**：
- [ ] `atomicWriteJSON` 按 tmp → fsync → rename
- [ ] tmp 在同一目录下
- [ ] 下次启动清理残留 `.tmp`

## 12. Extension 与 Daemon 版本错配

**教训**：hello 握手协商 protocol_version。不兼容返回 PROTOCOL_MISMATCH + 升级提示。

**Go 检查项**：
- [ ] Hello handler 检查 protocol_version
- [ ] PROTOCOL_MISMATCH 含 "请运行 ctm install 更新" 提示
- [ ] `ctm install --check` 验证 extension 版本
