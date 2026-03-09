# Codex Review V9

## Verdict

当前代码能编译，测试也能过：

- `go build ./...` 通过
- `go test -count=1 ./...` 通过

所以这轮我看到的问题，不是“代码已经坏了”，而是**产品表面契约**和**安装分发路径**仍有两处真实不一致。

---

## Findings

### Medium: TUI help/keymap 仍然明显超前于真实实现，多个 view 在“写了支持，但实际按下去不是那个行为”

这不是单个快捷键写错，而是一个成片的交互契约失真。

#### 1. Tabs view

`bindingsForView(ViewTabs)` 仍然宣称：

- `G` = `group (sel)`
- `a` = `add to collection`

但当前正常模式里：

- `G` 实际被绑定成“跳到底部”导航：[app.go:294](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/app.go#L294)
- 没有对应的 `a` 正常模式 handler

证据：

- [keymap.go:39](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/keymap.go#L39)
- [app.go:294](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/app.go#L294)

#### 2. Groups view

`bindingsForView(ViewGroups)` 仍然宣称：

- `Enter` = `expand/collapse`
- `e` = `edit`
- `u` = `ungroup`

但当前：

- `handleEnter()` 没有 `ViewGroups` 分支，[app.go:1001](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/app.go#L1001) 之后直接结束
- `u` 在正常模式里是全局 `clearSelection()`，不是 group ungroup：[app.go:310](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/app.go#L310)
- 没有 `e` 对应的正常模式 handler

证据：

- [keymap.go:54](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/keymap.go#L54)
- [app.go:310](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/app.go#L310)
- [app.go:1001](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/app.go#L1001)

#### 3. Targets view

`bindingsForView(ViewTargets)` 仍然宣称：

- `e` = `edit label`

但当前只有 `d` 对应 `targets.default`，没有 `targets.label` 的 TUI 入口。

证据：

- [keymap.go:74](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/keymap.go#L74)
- [app.go:394](/Users/didi/ai_projects/chrome-tab-manager/internal/tui/app.go#L394)

这类问题的危险不在于 panic，而在于：

- 用户看到 help 后会误以为功能可用
- 实际按下去要么没反应，要么执行的是别的动作
- 当前测试主要覆盖“能渲染、能切 view、部分 action 能跑”，没有把这些 help-advertised action 当作契约来验证

所以这条我认为是一个真实的 Medium finding，而不是文案小问题。

### Medium: Homebrew 分发路径按当前配置是坏的，`post_install` 会直接撞上 `install` 的 `--extension-id` 强校验

当前 `.goreleaser.yml` 里的 brew 配置是：

- `post_install: system "#{bin}/ctm", "install"`

但 `ctm install` 入口一开始就要求：

- `--extension-id` 必填

也就是说，按当前配置生成的 Homebrew formula 在 `post_install` 阶段会直接失败。

证据：

- [.goreleaser.yml:50](/Users/didi/ai_projects/chrome-tab-manager/.goreleaser.yml#L50)
- [install.go:20](/Users/didi/ai_projects/chrome-tab-manager/cmd/install.go#L20)

更糟的是，这不是偶然疏漏，而是当前测试也把这个行为固定下来了：

- `install --check` 无 `--extension-id` 也应失败：[cmd_test.go:428](/Users/didi/ai_projects/chrome-tab-manager/cmd/cmd_test.go#L428)

我本地直接执行 `./ctm install --check`，当前结果确实是报：

```text
--extension-id is required
```

所以这条不是“未来可能有问题”，而是**当前 release path 自相矛盾**。

---

## Bottom Line

这轮我没有新的 High finding。  
但有两个不建议忽略的 Medium：

1. TUI help/keymap 与真实行为仍有成片失真
2. Homebrew `post_install` 按当前配置跑不通

如果只问我“当前项目还能不能继续迭代”，答案当然是可以。  
但这两条都已经不是抽象技术债，而是会直接影响用户使用感知和分发可信度的实际问题。
