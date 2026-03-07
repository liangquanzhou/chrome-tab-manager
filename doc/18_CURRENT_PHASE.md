# CTM — Current Phase

## Phase 1: 骨架 + 协议 + Client

### 目标

建立 Go 项目骨架，实现协议编解码和 client 连接层。完成后应该能编译、跑通协议测试、通过 mock server 验证 client 连接。

### 当前只做

- `go mod init` + cobra CLI 骨架（root, version, daemon, tui 子命令）
- `internal/config/` — 路径解析（ConfigDir, SocketPath, SessionsDir, CollectionsDir）
- `internal/protocol/` — 消息类型定义、NDJSON 读写、ID 生成
- `internal/client/` — 连接 daemon、发请求、收响应、超时处理
- 单元测试：协议 round-trip、NDJSON 多条/空行/截断、fuzz、ID 唯一性、config 路径、client 连接

### 明确不做

- Daemon 实现（Phase 2）
- NM Shim（Phase 3）
- CLI 命令（Phase 4）
- TUI（Phase 5）
- 分发（Phase 6）
- 任何 Stage 3-8 的功能（Bookmarks / Sync / Search / Workspace / Power）
- Chrome Extension 修改

### Exit Criteria

完整测试表和手动验证清单见 `13_ACCEPTANCE.md` Phase 1 部分（测试 1.1-1.10 + 手动验证 3 项）。

### 设计约束（为后续 Phase 预留）

即使 Phase 1 不实现后续功能，设计必须满足：
- 持久对象预留 UUID + `createdAt` + `updatedAt` 字段
- 消息路由可扩展（action string 不硬编码 switch）
- ViewType 用 iota，预留 8 个视图位
- protocol_version 字段存在，用于未来 hello 握手
