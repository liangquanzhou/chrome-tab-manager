# CTM — Current Phase

## Status: Post-Phase 8 — Consolidation Round (收口轮)

Phase 1-8 全部完成。当前处于**收口轮 (Consolidation Round)**：daemon contract + CLI 路径做稳，统一真相源。

### Phase 完成回顾

| Phase | 内容 | 状态 |
|-------|------|------|
| 1 | 骨架 + 协议 + Client | DONE |
| 2 | Daemon (Hub actor + target + sessions/collections + subscribe) | DONE |
| 3 | NM Shim + Install | DONE |
| 4 | CLI (tabs/groups/sessions/collections/targets) | DONE |
| 5 | TUI (Bubble Tea, 9 views, vim nav, chords, 三通道反馈) | DONE |
| 6 | 分发 (GoReleaser + Makefile) | DONE |
| 7 | Bookmarks (mirror/overlay/export/tree/search/create/remove) | DONE |
| 8 | Search + Workspace + Sync | DONE |

### 收口轮已完成

- daemon contract 收口：`12_CONTRACTS.md` 为 authoritative contract（9 项模板）
- typed error 全面覆盖：sentinel error 扩展到全部 daemon 错误路径
- Action registry：`internal/daemon/registry.go` 为 canonical action inventory
- CLI 行为统一：exit code 0-4 分层、`--json` payload-only
- Doctor 运行时诊断：socket → daemon request → extension → manifest 内容校验

### Canonical 真相源

**以下来源为权威，不要在本文档中重复其内容：**

| 信息 | 权威来源 |
|------|----------|
| Action 列表与 CLI 暴露 | `internal/daemon/registry.go` |
| Action wire format | `doc/12_CONTRACTS.md` |
| 支持等级 (S/P/R) | `doc/19_CAPABILITY_MATRIX.md` |
| 测试覆盖率 | `go test -cover ./...`（不手写，随时运行获取） |
| 当前代码规模 | `find internal cmd -name '*.go' | wc -l`（不手写） |

### 待推进

- **P -> S 提升**: 将高价值的 P 级 action 补齐用户路径（详见 `19_CAPABILITY_MATRIX.md`）
- **Release 产品化**: `.goreleaser.yml` 从模板值改为真实发布配置
