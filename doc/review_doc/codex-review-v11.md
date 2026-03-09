# Codex Review V11

## 范围

这轮是对 `v10` 的纠偏复核。

我重点重新确认了三类判断：

1. 上一轮提到的真实 bug 现在是否还成立
2. `registry` / `contracts` / `matrix` / CLI 实际暴露面现在是否已经对齐
3. 当前剩下的问题到底还在代码层，还是已经主要收缩到顶层文档和发布层

---

## 结论

**当前主风险已经进一步收缩到“顶层文档与发布层没收口”，不是新的功能性崩点。**

和 `v10` 相比，这轮最重要的变化是：

- `downloads.cancel` 的 CLI/extension payload mismatch 已经修掉
- `ActionRegistry` 已明显朝当前 CLI/contract 对齐
- `12_CONTRACTS.md` 继续变强，已经比 `matrix` 和 `phase` 更接近 current truth

所以当前更准确的说法是：

> **代码层继续在收口；真正落后的，主要是 `18_CURRENT_PHASE.md`、`19_CAPABILITY_MATRIX.md` 和发布配置。**

---

## Findings

### Medium: `18_CURRENT_PHASE.md` 仍然明显过期，不能作为 current truth 使用

这份文档仍然保留多条已经失效的信息：

- 还写着 `57` 个 action
- 还写着 `S=32 / P=16 / R=9`
- 还写着 `Groups view` 的 `Enter handler` 缺失
- 还写着 `daemon 30.6%` 覆盖率需提升

这些都和当前仓库状态冲突。

现在这份文档更像历史快照，不适合再承担“当前阶段说明”。

### Medium: `19_CAPABILITY_MATRIX.md` 仍未收口成可直接引用的 canonical truth

这份文档目前仍有三类问题：

1. **summary 计数不对**
   - 我按 action 表格重数，当前是 `62` 行
   - 实际分布是 `S=31 / P=19 / R=12`
   - 但 summary 还写着 `32 / 16 / 9`

2. **过期描述仍在**
   - 还保留 `Groups: Enter 在 handleEnter 中无 ViewGroups case`

3. **与当前 contract / registry / CLI 面仍有冲突**
   - 最明显的是 `daemon.stop`
   - matrix 里仍写成 CLI `V`，并说明可通过 `ctm daemon stop`
   - 但当前 `cmd/` 实际并没有这个子命令
   - 与 `12_CONTRACTS.md` / `registry.go` 当前的 `CLI: internal` 判断不一致

这意味着：

> matrix 现在还不能直接拿来当 authoritative inventory。

### Low: `.goreleaser.yml` 仍然是模板化发布配置，分发路径还不是可直接发布状态

当前 Homebrew 配置仍然是占位值：

- `owner: user`
- `name: homebrew-tap`
- `homepage: https://github.com/user/chrome-tab-manager`

这说明 release 骨架有了，但 distribution 还不是一条真正产品化的主路径。

---

## 已关闭 / 应撤销的旧判断

这轮有几条上一轮判断已经不应继续保留。

### 1. `downloads.cancel` payload mismatch 已关闭

当前 `cmd/downloads.go` 已经改成发送 `id`，与 extension 和 contract 对齐。

所以这条已经不再是当前 bug。

### 2. `ActionRegistry` 大面积分叉 的说法要收紧

`registry.go` 这轮已经明显同步过：

- `daemon.stop` → `CLIInternal`
- `bookmarks.get` / `bookmarks.overlay.*` → `CLIInternal`
- `downloads.cancel`、`history.delete` 等也已经入表

现在剩余的分叉主要集中在：

- `matrix` 还没追上
- `phase` 更没追上

而不是 `registry` 还在大面积乱写。

### 3. `12_CONTRACTS.md` 现在是当前最接近真相的顶层文档

它已经明确：

- selector 只有 `targetId` 已实现
- `--json` 是 payload-only
- exit code 约定
- `downloads.cancel` 请求字段是 `id`

并且 `tabs.getText` / `tabs.capture` / `windows.focus` 等前几轮缺失条目也已经补入。

所以接下来更合理的方向，不是继续怀疑 `12_CONTRACTS.md`，而是：

> **让 `matrix` 和 `phase` 反过来向它收口。**

---

## 当前项目状态

我现在会这样描述当前 CTM：

> **代码内核继续收口，顶层叙事仍滞后。**

拆开来看：

### 代码层

- `go build ./...` 通过
- `go vet ./...` 通过
- `go test -count=1 ./...` 通过
- `cmd` / `daemon` / `protocol` 的关键回归通过
- `downloads.cancel` 这类链路错误也在被修掉

### 文档/治理层

- `12_CONTRACTS.md` 变强
- `registry.go` 变强
- `19_CAPABILITY_MATRIX.md` 仍未同步完成
- `18_CURRENT_PHASE.md` 仍明显过期
- release 配置仍未产品化

所以当前阶段最值钱的工作，已经不是继续补 action，而是：

1. 让 `12_CONTRACTS.md` 成为 canonical truth
2. 用 `registry.go` 和实际 CLI 命令面校正 `19_CAPABILITY_MATRIX.md`
3. 把 `18_CURRENT_PHASE.md` 改成“阶段说明 + 指向 canonical truth”，不再维护易漂移细节
4. 把 `.goreleaser.yml` 从模板变成真实发布配置

---

## 最后判断

如果只用一句话概括当前状态：

> **代码已经比文档跑得快了。**

现在真正需要追的，不是新的复杂 feature，而是把：

- contracts
- registry
- matrix
- phase
- release config

收成一个版本。

只有做到这一步，项目才算从“功能很多的系统”进入“边界清晰的产品”。
