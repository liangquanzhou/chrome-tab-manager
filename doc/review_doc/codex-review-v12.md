# Codex Review V12

## 结论

你这次指出的时序问题是成立的。

我重新按当前文件状态复核后，之前那几条关于：

- `18_CURRENT_PHASE.md` 还写旧计数 / 旧覆盖率
- `19_CAPABILITY_MATRIX.md` 还保留旧 summary / 旧 Groups Enter 描述

都已经**不再成立**。

当前仓库状态更准确的判断是：

> **代码和 contract 层已经明显收口；剩余问题主要是少量文档细节和发布配置，不再是系统性失真。**

---

## 当前 Findings

### Low: `19_CAPABILITY_MATRIX.md` 里 `daemon.stop` 的 CLI 描述仍与当前实现不一致

当前 matrix 仍写：

- `daemon.stop | ... | CLI = V`
- `daemon.stop 通过 ctm daemon stop 或内部请求触发`

但当前 `cmd/daemon.go` 实际没有 `stop` 子命令。

这条和当前 `12_CONTRACTS.md`、`ActionRegistry` 的 `CLI: internal` 判断也不一致。

这是当前还能确认的一条真实文档错位，但已经是**局部**问题，不是上一轮那种整份文档过期。

### Low: `.goreleaser.yml` 仍然是模板化发布配置

当前 Homebrew 配置还是占位值：

- `owner: user`
- `name: homebrew-tap`
- `homepage: https://github.com/user/chrome-tab-manager`

这不影响本地运行和主链路，但说明 release/distribution 还没完全产品化。

---

## 已关闭的旧判断

这轮明确确认，以下判断已经不应继续保留：

### 1. `18_CURRENT_PHASE.md` 旧计数 / 旧覆盖率 / 旧 handler 缺失

当前文件已经重写成 pointer-style 阶段文档，不再手写这些易漂移细节。

### 2. `19_CAPABILITY_MATRIX.md` 的旧 summary 计数

当前 summary 已更新为：

- `S=31`
- `P=19`
- `R=12`

与 action 表计数一致。

### 3. `downloads.cancel` payload mismatch

当前 CLI 已改为发送 `id`，和 extension / contract 对齐，这条真实 bug 已关闭。

### 4. `ActionRegistry` 大面积分叉

`registry.go` 这轮已经明显同步过，和 `12_CONTRACTS.md` / 实际 CLI 面的对齐程度比之前高很多。

现在剩余的不一致主要是少量 matrix 文案，而不是 registry 整体失真。

---

## 当前项目状态

我现在会这样描述当前 CTM：

> **主链路、contract、CLI 行为、typed error、doctor 都已经进入收口态；剩余问题以文档尾差和发布配置为主。**

这意味着：

- 现在不应该再按“系统性不稳定”来评价项目
- 也不应该再沿用几轮前的 old finding
- 下一步更像 polishing / reconciliation，而不是大规模补洞

---

## 建议

### P1

修掉 `19_CAPABILITY_MATRIX.md` 里 `daemon.stop` 这一条 CLI 描述，让 matrix 和实际 `cmd/` / contract / registry 完全一致。

### P2

把 `.goreleaser.yml` 从模板配置改成真实仓库配置，并补最小 release smoke。

---

## 最后判断

如果只给一句最新判断：

> **你说的时序问题判断是对的；我当前全盘复核后，不再认为 phase/matrix 还有大面积过期。剩余问题已经降到局部文档对齐和 release 配置层面。**
