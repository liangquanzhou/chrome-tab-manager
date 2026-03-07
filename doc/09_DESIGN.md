# CTM — Modular Design Specification

领域对象定义见 `02_DOMAIN.md`。能力域和 Stage 定义见 `03_CAPABILITIES.md`。

## 1. Design Principles

### 1.1 Single Responsibility

| Package | 职责 | 不做什么 |
|---------|------|----------|
| `config` | 路径解析 | 不读写文件内容 |
| `protocol` | 消息编解码 | 不知道 socket/network |
| `client` | 连接 daemon、发请求 | 不知道 TUI/CLI |
| `daemon` | 状态管理、路由、持久化 | 不知道 Chrome API 细节 |
| `nmshim` | 帧格式转换 | 无业务逻辑 |
| `tui` | 用户交互、渲染 | 不直接操作文件系统 |

### 1.2 Dependency Direction

```
cmd/ → imports → internal/tui, internal/client, internal/daemon, internal/config
internal/tui → imports → internal/protocol, internal/config
internal/client → imports → internal/protocol, internal/config
internal/daemon → imports → internal/protocol, internal/config
internal/nmshim → imports → internal/protocol, internal/config
internal/protocol → imports → nothing (leaf)
internal/config → imports → nothing (leaf)

daemon 内部依赖:
  daemon → bookmarks, search, sync, workspace (Stage 3-6)
  bookmarks, search, sync, workspace → protocol, config (叶节点)
```

禁止循环依赖。`protocol` 和 `config` 是纯叶节点。

### 1.3 Interface Segregation

```go
type Requester interface {
    Request(ctx context.Context, action string, payload any, target *TargetSelector) (*Response, error)
}
type Subscriber interface {
    Subscribe(ctx context.Context, patterns []string) (<-chan *Event, error)
}
type MessageWriter interface {
    WriteMessage(msg *Message) error
}
```

### 1.4 Fail Fast

- Protocol version 不兼容 → 立即断开
- Socket 连接失败 → 立即返回错误
- JSON 解析失败 → 记日志 + 跳过，不 panic
- Extension 10s 无响应 → 超时错误

### 1.5 Least Surprise

- `Esc` = 取消/返回，不修改状态
- `Enter` = 主操作
- 删除持久数据需二次确认 (D-D)
- 关闭 tab（临时资源）不需确认

### 1.6 YAGNI

当前不做（但已预留位置）：数据库持久化、插件系统、Google OAuth、高级 profile 推断。

完整建模、分阶段实现：iCloud sync (Stage 4)、多浏览器 (Stage 7) 已在架构中预留位置。

### 1.7 Defense in Depth

- 目录权限 `0700` + socket `0600` + peer UID 校验
- Protocol version + hello 握手 + capability negotiation
- flock 单例 + stale socket 探测 + auto-start 竞态锁

### 1.8 Separation of Concerns

三层独立可测：
```
Transport (socket, NDJSON, NM framing)
    ↕
Protocol (message types, routing, correlation)
    ↕
Business Logic (tabs, groups, sessions, collections)
```

## 2. Module Specifications

### 2.1 `internal/config`

路径解析 + 常量定义。所有路径函数是纯函数，测试时通过 `CTM_CONFIG_DIR` 环境变量覆盖。

```go
func ConfigDir() string         // ~/.config/ctm/
func SocketPath() string        // ~/.config/ctm/daemon.sock
func LockPath() string          // ~/.config/ctm/daemon.lock
func SessionsDir() string       // ~/.config/ctm/sessions/
func CollectionsDir() string    // ~/.config/ctm/collections/
func BookmarksDir() string      // ~/.config/ctm/bookmarks/
func OverlaysDir() string       // ~/.config/ctm/overlays/
func WorkspacesDir() string     // ~/.config/ctm/workspaces/
func SavedSearchesDir() string  // ~/.config/ctm/searches/
func SyncDir() string           // ~/Library/Mobile Documents/com~ctm/
func ExtensionDir() string      // ~/.config/ctm/extension/
func LogPath() string           // ~/.config/ctm/daemon.log
func EnsureDirs() error         // 创建所有必要目录，权限 0700
```

### 2.2 `internal/protocol`

消息类型定义 + NDJSON 编解码 + ID 生成。

```go
type MessageType string  // "hello", "request", "response", "error", "event"

type Message struct {
    ID              string          `json:"id"`
    ProtocolVersion int             `json:"protocol_version"`
    Type            MessageType     `json:"type"`
    Action          string          `json:"action,omitempty"`
    Target          *TargetSelector `json:"target,omitempty"`
    Payload         json.RawMessage `json:"payload,omitempty"`
    Error           *ErrorBody      `json:"error,omitempty"`
}

type ErrorCode string  // DAEMON_UNAVAILABLE, TARGET_OFFLINE, TARGET_AMBIGUOUS,
                       // EXTENSION_NOT_CONNECTED, CHROME_API_ERROR, INSTALLATION_INVALID,
                       // PROTOCOL_MISMATCH, TIMEOUT, UNKNOWN_ACTION, INVALID_PAYLOAD

// NDJSON: Reader (bufio.Scanner 1MB buffer) + Writer (json.Marshal + \n, mutex 保护)
// Payload 用 json.RawMessage 延迟解析。Writer 线程安全，Reader 不需要。
func MakeID() string  // "msg_" + timestamp + counter
```

### 2.3 `internal/client`

连接 daemon、发送请求、订阅事件、自动重连。

```go
func New(socketPath string) *Client
func (c *Client) Connect(ctx context.Context) error
func (c *Client) Close() error
func (c *Client) Request(ctx context.Context, action string, payload any, target *protocol.TargetSelector) (*protocol.Message, error)
func (c *Client) Subscribe(ctx context.Context, patterns []string) (<-chan *protocol.Message, error)
func (c *Client) Connected() bool
```

内部：读 goroutine 分发 response/event；Request 带 timeout；断线指数退避重连 (1s-30s)。

```
Disconnected → Connecting → Connected → Disconnected
                    ↑                        |
                    └────── (auto-reconnect) ─┘
```

### 2.4 `internal/daemon`

Socket server + Hub (actor 模式) + 存储。

**Hub**：单 goroutine 独占所有可变状态，零锁。

```go
type Hub struct {
    targets, pending, subscribers, defaultTarget, targetCounter
    registerCh, unregisterCh, messageCh, doneCh  // channels
}
func (h *Hub) Run(ctx context.Context)
```

**消息路由**：
```
hello    → 注册 target
request  → daemon.*/targets.* → Hub 直接处理
           sessions.*/collections.* → Hub 处理 (文件 I/O)
           subscribe → 注册 subscriber
           tabs.*/groups.* → 转发到 target extension
response/error → 匹配 pending request，转发给发起方
event    → fanout 到匹配的 subscribers
```

**存储**：原子写入 `tmp file → fsync → rename`。

### 2.5 `internal/nmshim`

Chrome stdin/stdout (4-byte LE) ↔ daemon socket (NDJSON)。stdout 只用于协议消息，诊断日志写 stderr。消息上限 1MB。

### 2.6 `internal/tui`

Bubble Tea TUI。视图架构和键绑定定义见 `10_TUI.md`。

```go
type App struct {
    client, state AppState
    tabs, groups, sessions, collections, targets  // sub-models
    header, statusbar, help, keymap
}
```

### 2.7 Stage 3-6 Modules

#### `internal/bookmarks` (Stage 3)

```go
type BookmarkNode struct { ID, Title, URL, ParentID string; DateAdded int64; Children []*BookmarkNode }
type BookmarkOverlay struct { BookmarkID string; Tags []string; Note, Alias string }
type BookmarkMirror struct { Tree []*BookmarkNode; MirroredAt, TargetID string }
```

#### `internal/search` (Stage 5)

```go
type SearchQuery struct { Query, Mode string; Scopes, Tags []string; Host string; Limit int }
type SearchResult struct { Kind, ID, Title, URL, MatchField string; Score float64 }
type SavedSearch struct { ID, Name string; Query SearchQuery; CreatedAt, UpdatedAt string }
```

#### `internal/sync` (Stage 4)

```go
type SyncEngine struct { localDir, cloudDir string }
type SyncAccount struct { ID, Kind, Label string; Enabled bool }
type SyncState struct { ResourceID, ResourceKind, SyncDomain, Status string; ... }
type Device struct { ID, Name, Platform, LastSeen string }
```

#### `internal/workspace` (Stage 6)

```go
type Workspace struct {
    ID, Name, Description string
    Sessions, Collections, BookmarkFolderIDs, SavedSearchIDs, Tags []string
    Notes, Status, DefaultTarget, LastActiveAt, CreatedAt, UpdatedAt string
}
```

## 3. Error Handling

### Error 分层

```
Chrome API Error → ErrorBody{CHROME_API_ERROR}
Protocol Error   → 记日志，跳过消息
Transport Error  → client 重连；TUI 显示 disconnected
Business Error   → ErrorBody{code, message}
User Error       → CLI 打印 usage；TUI 显示 toast
```

### Error 传播规则

| 来源 → 处理方 | 动作 |
|---------------|------|
| Extension → daemon | 原样转发 |
| daemon → CLI | 打印 + exit 1 |
| daemon → TUI | error bar (Esc 清除) |
| socket 断开 | 重连 + toast |
| JSON 解析失败 | warn 日志 + 跳过 |

不 panic、不吞错误、不暴露内部路径。

## 4. State Machines

### Daemon Connection (per connection)
```
New → HelloReceived → Identified(target) → Active → Closed
 |                                                     ↑
 └── (no hello within 5s) → Timeout → Closed ─────────┘
```

### TUI Input Mode
见 `10_TUI.md` §7。

## 5. Timeout & Retry Budget

| 操作 | 超时 | 重试 |
|------|------|------|
| Socket connect | 3s | 不重试 |
| Extension response | 10s | 不重试 (daemon 侧 timer) |
| Auto-start daemon | 3s poll | 最多 1 次 |
| Client reconnect | 指数退避 1-30s | 无限重试 |
| TUI event batch | 150ms | N/A |

## 6. Testing Strategy

### 测试金字塔
```
        /  E2E smoke  \         少，慢，需 Chrome
       / integration   \        中等，需 socket
      /   unit tests    \       多，快，纯函数
```

### Per-Module

| Module | 类型 | 关键用例 |
|--------|------|----------|
| `config` | 单元 | 路径计算、环境变量覆盖 |
| `protocol` | 单元 + fuzz | round-trip、畸形输入 |
| `client` | 单元 | connect/request/timeout/reconnect |
| `daemon` | 集成 | Hub 路由、target 注册、session CRUD、event fanout |
| `nmshim` | 单元 + fuzz | 4-byte LE framing、大消息 |
| `tui` | teatest | 按键序列 → 断言输出 |
| E2E | smoke script | daemon + shim + extension 全链路 |

### 可测性要求

所有 socket 路径必须可注入（参数或环境变量）。CI 必跑：

```bash
go test -race -count=1 ./...
go test -fuzz=FuzzNDJSON ./internal/protocol/ -fuzztime=30s
go test -fuzz=FuzzNMFrame ./internal/nmshim/ -fuzztime=30s
```

## 7. Phase 1 Design Constraints (为 Stage 3-6 铺路)

```go
// 1. 存储函数接受目录参数
func atomicWriteJSON(dir, name string, data any) error

// 2. Hub action 路由用 map/switch，不用 if-else 链
// 加 bookmarks.* 只需注册新 handler

// 3. TUI ViewType 用 iota，追加 ViewBookmarks / ViewWorkspaces / ViewSync

// 4. 所有持久对象有 id (UUID) + createdAt + updatedAt

// 5. NM manifest 路径和 extension ID 通过 config 包获取

// 6. Extension capability 协商，未来加 "bookmarks" capability
```
