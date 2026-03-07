# CTM — Modular Design Specification

## 1. Design Principles

本项目遵循以下原则。每条原则后附具体约束，AI 编码时必须检查。

### 1.1 Single Responsibility（单一职责）

每个 Go package 只做一件事：

| Package | 职责 | 不做什么 |
|---------|------|----------|
| `config` | 路径解析 | 不读写文件内容 |
| `protocol` | 消息编解码 | 不知道 socket/network |
| `client` | 连接 daemon、发请求 | 不知道 TUI/CLI |
| `daemon` | 状态管理、路由、持久化 | 不知道 Chrome API 细节 |
| `nmshim` | 帧格式转换 | 无业务逻辑 |
| `tui` | 用户交互、渲染 | 不直接操作文件系统 |

### 1.2 Dependency Inversion（依赖反转）

**依赖方向**：高层模块依赖抽象，不依赖具体实现。

```
cmd/ → imports → internal/tui, internal/client, internal/daemon, internal/config
internal/tui → imports → internal/protocol, internal/config
internal/client → imports → internal/protocol, internal/config
internal/daemon → imports → internal/protocol, internal/config
internal/nmshim → imports → internal/protocol, internal/config
internal/protocol → imports → nothing (leaf)
internal/config → imports → nothing (leaf)
```

**禁止循环依赖**。`protocol` 和 `config` 是纯叶节点。

### 1.3 Interface Segregation（接口隔离）

不做一个巨型 Client interface，拆成小接口：

```go
// 只请求
type Requester interface {
    Request(ctx context.Context, action string, payload any, target *TargetSelector) (*Response, error)
}

// 只订阅
type Subscriber interface {
    Subscribe(ctx context.Context, patterns []string) (<-chan *Event, error)
}

// 只写消息（daemon 内部用）
type MessageWriter interface {
    WriteMessage(msg *Message) error
}
```

### 1.4 Fail Fast（快速失败）

- Protocol version 不兼容 → 立即断开，不降级
- Socket 连接失败 → 立即返回错误，由调用方决定重试
- JSON 解析失败 → 记日志 + 跳过该消息，不 panic
- Extension 10s 无响应 → 超时错误返回给客户端

### 1.5 Principle of Least Surprise（最少惊讶）

- `Esc` 永远是取消/返回，不会修改状态
- `Enter` 永远是主操作
- 删除持久数据（session/collection）需要二次确认（D-D）
- 关闭 tab（临时资源）不需要确认

### 1.6 YAGNI（不做不需要的事）

当前阶段明确不做（但在设计中已预留位置）：
- 数据库持久化（JSON 文件足够）
- 插件系统
- 自己实现 Google OAuth（Chrome Sync 已处理）
- 高级 profile 推断

以下能力已在架构中定义，按 BUILD_ORDER 的 Stage 顺序实现：
- 多浏览器支持（Stage 7: Interaction）
- iCloud sync（Stage 4: Sync）
- Chrome Web Store（Stage 7: Interaction）
- Linux 路径（按需补充）

### 1.7 Defense in Depth（纵深防御）

安全不靠单一措施：
- 目录权限 `0700` + socket 权限 `0600` + peer UID 校验
- Protocol version 字段 + hello 握手校验 + capability negotiation
- flock 单例 + stale socket 探测 + auto-start 竞态锁

### 1.8 Separation of Concerns（关注点分离）

分三层，每层独立可测：

```
Transport (socket, NDJSON, NM framing)
    ↕
Protocol (message types, routing, correlation)
    ↕
Business Logic (tabs, groups, sessions, collections)
```

---

## 2. Package Dependency Graph

```
                         cmd/
                  /   |    |    \    \
                 v    v    v     v    v
              tui  client daemon nmshim
               |     |     |      |
               v     v     v      v
            protocol protocol protocol
               |     |     |      |
               v     v     v      v
            config  config config config

          daemon 内部依赖:
            daemon → bookmarks, search, sync, workspace (Stage 3-6)
            bookmarks, search, sync, workspace → protocol, config (叶节点)
```

规则：
- 箭头方向 = import 方向
- `protocol` 和 `config` 是纯叶节点，不 import 任何 internal 包
- `tui` 通过 interface 依赖 `client`，不直接 import `daemon`
- `cmd/` 是组装层，负责连接各模块
- Stage 3-6 新增的 `bookmarks`、`search`、`sync`、`workspace` 包只依赖 `protocol` 和 `config`
- `daemon` 在 Hub 路由中调用这些包的函数

---

## 3. Module Specifications

### 3.1 `internal/config`

**职责**：路径解析 + 常量定义

```go
package config

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

**设计约束**：
- 所有路径函数是纯函数（可被 test 覆盖通过环境变量或参数）
- 测试时通过 `CTM_CONFIG_DIR` 环境变量覆盖根目录
- 不做文件 I/O，只计算路径

### 3.2 `internal/protocol`

**职责**：消息类型定义 + NDJSON 编解码 + ID 生成

```go
// --- 消息类型 ---
type MessageType string
const (
    TypeHello    MessageType = "hello"
    TypeRequest  MessageType = "request"
    TypeResponse MessageType = "response"
    TypeError    MessageType = "error"
    TypeEvent    MessageType = "event"
)

type Message struct {
    ID              string          `json:"id"`
    ProtocolVersion int             `json:"protocol_version"`
    Type            MessageType     `json:"type"`
    Action          string          `json:"action,omitempty"`
    Target          *TargetSelector `json:"target,omitempty"`
    Payload         json.RawMessage `json:"payload,omitempty"`
    Error           *ErrorBody      `json:"error,omitempty"`
}

type TargetSelector struct {
    TargetID string `json:"targetId,omitempty"`
}

type ErrorBody struct {
    Code    ErrorCode `json:"code"`
    Message string    `json:"message"`
    Details any       `json:"details,omitempty"`
}

// --- Error Codes ---
type ErrorCode string
const (
    ErrDaemonUnavailable     ErrorCode = "DAEMON_UNAVAILABLE"
    ErrTargetOffline         ErrorCode = "TARGET_OFFLINE"
    ErrTargetAmbiguous       ErrorCode = "TARGET_AMBIGUOUS"
    ErrExtensionNotConnected ErrorCode = "EXTENSION_NOT_CONNECTED"
    ErrChromeAPIError        ErrorCode = "CHROME_API_ERROR"
    ErrInstallationInvalid   ErrorCode = "INSTALLATION_INVALID"
    ErrProtocolMismatch      ErrorCode = "PROTOCOL_MISMATCH"
    ErrTimeout               ErrorCode = "TIMEOUT"
    ErrUnknownAction         ErrorCode = "UNKNOWN_ACTION"
    ErrInvalidPayload        ErrorCode = "INVALID_PAYLOAD"
)

// --- NDJSON Reader/Writer ---
type Reader struct { scanner *bufio.Scanner }
func NewReader(r io.Reader) *Reader   // scanner buffer 1MB
func (r *Reader) Read() (*Message, error)

type Writer struct { w io.Writer; mu sync.Mutex }
func NewWriter(w io.Writer) *Writer
func (w *Writer) Write(msg *Message) error  // json.Marshal + \n, mutex 保护

// --- ID 生成 ---
func MakeID() string  // "msg_" + timestamp + counter
```

**设计约束**：
- `Payload` 用 `json.RawMessage`，延迟解析到业务层
- Writer 必须线程安全（多个 goroutine 可能同时写同一个 conn）
- Reader 不需要线程安全（每个 conn 只有一个读 goroutine）

### 3.3 `internal/client`

**职责**：连接 daemon、发送请求、订阅事件、自动重连

```go
type Client struct {
    socketPath string
    conn       net.Conn
    reader     *protocol.Reader
    writer     *protocol.Writer
    pending    map[string]chan *protocol.Message  // request ID → response channel
    events     chan *protocol.Message
    // ...
}

// 公开接口
func New(socketPath string) *Client
func (c *Client) Connect(ctx context.Context) error
func (c *Client) Close() error
func (c *Client) Request(ctx context.Context, action string, payload any, target *protocol.TargetSelector) (*protocol.Message, error)
func (c *Client) Subscribe(ctx context.Context, patterns []string) (<-chan *protocol.Message, error)
func (c *Client) Connected() bool
```

**内部机制**：
- `Connect` 后启动一个读 goroutine，分发 response/event
- `Request` 发送消息 + 等待对应 ID 的 response（带 timeout）
- `Subscribe` 发送 subscribe 请求，后续 events 通过 channel 推送
- 断线重连：指数退避（1s, 2s, 4s... 上限 30s），重连后自动重新 subscribe

**状态机**：

```
Disconnected → Connecting → Connected → Disconnected
                    ↑                        |
                    └────── (auto-reconnect) ─┘
```

### 3.4 `internal/daemon`

**职责**：Socket server + Hub（状态管理）+ 存储

#### 3.4.1 Server

```go
func Start(ctx context.Context) error
// 1. EnsureDirs()
// 2. 获取 flock
// 3. 清理 stale socket
// 4. net.Listen("unix", socketPath)
// 5. chmod 0600
// 6. 启动 Hub goroutine
// 7. Accept loop: 每个 conn 启动一个读 goroutine
// 8. ctx.Done() → 优雅关闭
```

#### 3.4.2 Hub（actor 模式，核心）

```go
type Hub struct {
    // 全部状态，仅在 Run goroutine 内访问
    targets      map[string]*Target
    pending      map[string]*PendingRequest
    subscribers  []*Subscriber
    defaultTarget string
    targetCounter int

    // channels（外部 → Hub）
    registerCh   chan *Conn
    unregisterCh chan *Conn
    messageCh    chan *IncomingMessage
    doneCh       chan struct{}
}

func (h *Hub) Run(ctx context.Context)  // 唯一访问状态的 goroutine
```

**消息路由规则**：

```
收到消息 → 判断 type:
  hello   → 注册 target（分配 ID, 记录 channel/extensionId）
  request → 判断 action:
    daemon.*     → Hub 直接处理
    targets.*    → Hub 直接处理
    sessions.*   → Hub 处理（文件 I/O）
    collections.* → Hub 处理（文件 I/O）
    subscribe    → 注册 subscriber
    tabs.*/groups.* → 转发到 target extension
  response/error → 匹配 pending request，转发给发起方
  event → fanout 到匹配的 subscribers
```

#### 3.4.3 Storage（sessions + collections）

```go
// 原子写入：tmp file → fsync → rename
func atomicWriteJSON(path string, data any) error

// Session CRUD
func (h *Hub) handleSessionSave(conn *Conn, req *protocol.Message)
func (h *Hub) handleSessionList(conn *Conn, req *protocol.Message)
func (h *Hub) handleSessionGet(conn *Conn, req *protocol.Message)
func (h *Hub) handleSessionDelete(conn *Conn, req *protocol.Message)
func (h *Hub) handleSessionRestore(conn *Conn, req *protocol.Message)

// Collection CRUD（同理）
```

**数据文件格式**：见 CONTRACTS.md

### 3.5 `internal/nmshim`

**职责**：Chrome stdin/stdout (4-byte LE) ↔ daemon socket (NDJSON)

```go
func Run(ctx context.Context, socketPath string) error
// 1. 连接 daemon socket
// 2. 启动两个 goroutine:
//    stdin → readNativeMessage → writeNDJSON(socket)
//    socket → readNDJSON → writeNativeMessage(stdout)
// 3. 任一方断开 → 退出

func readNativeMessage(r io.Reader) ([]byte, error)   // 4-byte LE length + JSON
func writeNativeMessage(w io.Writer, data []byte) error // 4-byte LE length + JSON
```

**设计约束**：
- stdout 只用于协议消息，诊断日志只写 stderr
- 消息大小上限 1MB（Chrome Native Messaging 限制）
- 长度字段用 byte 计算，不是 string 长度

### 3.6 `internal/tui`

**职责**：Bubble Tea TUI，5 个视图 + 全局键绑定 + 三通道反馈

#### 3.6.1 App Model（Root）

```go
type App struct {
    client     *client.Client
    state      AppState
    tabs       TabsModel
    groups     GroupsModel
    sessions   SessionsModel
    collections CollectionsModel
    targets    TargetsModel
    header     HeaderModel
    statusbar  StatusBarModel
    help       HelpModel
    keymap     KeyMap
}

func (a App) Init() tea.Cmd
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (a App) View() string
```

#### 3.6.2 State

```go
type AppState struct {
    View             ViewType
    SelectedTargetID string
    ConnectionStatus ConnectionStatus
    Filter           string
    InputMode        InputMode
    Loading          bool
    Error            string
    Toast            *Toast
    ConfirmHint      string
    EventBuffer      []protocol.Message  // 150ms batch
}

type ViewType int
const (
    ViewTargets ViewType = iota
    ViewTabs
    ViewGroups
    ViewSessions
    ViewCollections
)

type InputMode int
const (
    ModeNormal InputMode = iota
    ModeFilter
    ModeCommand
    ModeHelp
    ModeGroupTitle
    ModeSessionName
    ModeCollectionName
    ModeCollectionPicker
    ModeYank       // y- chord
    ModeZFilter    // z- chord
    ModeConfirmDelete // D- chord
)
```

#### 3.6.3 Keymap（Single Source of Truth）

```go
// keymap.go 是键绑定的唯一注册表
// Help overlay 和 StatusBar 都从这里生成
// 不允许在 view 代码中硬编码键绑定描述

type KeyBinding struct {
    Key      key.Binding
    ActionID string
    Label    string
    Scope    Scope  // Global, Tabs, Groups, Sessions, Collections
}

var AllBindings = []KeyBinding{...}  // 全部键绑定

func BindingsForScope(scope Scope) []KeyBinding  // 按 scope 过滤
func BindingsForView(view ViewType) []KeyBinding  // 当前 view 的绑定
```

---

## 4. Error Handling Strategy

### 4.1 Error 分层

```
Chrome API Error (extension 内部)
    ↓ 包装为 ErrorBody{code: CHROME_API_ERROR}
Protocol Error (NDJSON 解析失败)
    ↓ 记日志，跳过消息
Transport Error (socket 断开)
    ↓ client 触发重连；TUI 显示 disconnected
Business Error (session 不存在)
    ↓ 返回 ErrorBody{code: ..., message: "..."}
User Error (参数缺失)
    ↓ CLI 打印 usage；TUI 显示 toast
```

### 4.2 Error 传播规则

| 来源 | 处理方 | 动作 |
|------|--------|------|
| Extension → daemon | daemon | 原样转发给 CLI/TUI |
| daemon → CLI | CLI | 打印 error message + exit 1 |
| daemon → TUI | TUI | 显示 error bar（persistent，Esc 清除） |
| socket 断开 | client | 触发重连，TUI 显示 toast |
| JSON 解析失败 | protocol.Reader | 记 warn 日志，跳过，不断连 |

### 4.3 Error 不应该做的事

- 不 panic（除非真正不可恢复）
- 不吞错误（每个 error 要么处理要么传播）
- 不在 error message 里暴露内部路径

---

## 5. State Machines

### 5.1 Daemon Connection State (per connection)

```
New → HelloReceived → Identified(target) → Active → Closed
 |                                                     ↑
 └── (no hello within 5s) → Timeout → Closed ─────────┘
```

### 5.2 Client Connection State

```
Disconnected → Connecting → Connected → Disconnected
                  ↑            |              |
                  |            | (socket err) |
                  └────────────┘              |
                  ↑                           |
                  └─── (backoff timer) ───────┘
```

### 5.3 TUI Input Mode State

```
Normal ──/──→ Filter ──Enter──→ Normal (keep filter)
   |                  ──Esc───→ Normal (clear filter)
   |──:──→ Command ──Enter──→ Normal (execute)
   |                 ──Esc───→ Normal
   |──?──→ Help ────Esc/q/──→ Normal
   |──y──→ Yank ────y/n/h/m──→ Normal (copy + toast)
   |               ──other──→ Normal (cancel)
   |──z──→ ZFilter ──g/u/p/w──→ Normal (apply filter)
   |                ──other──→ Normal (cancel)
   |──D──→ ConfirmDelete ──D──→ Normal (delete + toast)
   |                     ──other──→ Normal (cancel)
   |──G──→ GroupTitle ──Enter──→ Normal (create group)
   |                   ──Esc───→ Normal
```

---

## 6. Timeout & Retry Budget

端到端超时：CLI/TUI 发出请求 → 收到响应

```
CLI/TUI ──(socket connect: 3s)──→ daemon
daemon  ──(forward to ext: 0ms)──→ extension
extension ──(Chrome API: ~5s)───→ daemon
daemon  ──(response: 0ms)──────→ CLI/TUI
                              Total budget: ~10s
```

| 操作 | 超时 | 重试 |
|------|------|------|
| Socket connect | 3s | 不重试，返回错误 |
| Extension response | 10s | 不重试（daemon 侧 timer） |
| Auto-start daemon | 3s poll | 最多 1 次 |
| Client reconnect | 指数退避 1-30s | 无限重试 |
| TUI event batch | 150ms | N/A（聚合，不重试） |

---

## 7. Testing Strategy (Detailed)

### 7.1 测试金字塔

```
        /  E2E smoke  \         少，慢，需要 Chrome
       / integration   \        中等，需要 socket
      /   unit tests    \       多，快，纯函数
```

### 7.2 Per-Module 测试策略

| Module | 测试类型 | 依赖 | 关键用例 |
|--------|----------|------|----------|
| `config` | 单元 | 无 | 路径计算、环境变量覆盖 |
| `protocol` | 单元 + fuzz | 无 | round-trip encode/decode、畸形输入 |
| `client` | 单元 | mock net.Conn | connect/request/timeout/reconnect |
| `daemon` | 集成 | temp socket | Hub 路由、target 注册、session CRUD、event fanout |
| `nmshim` | 单元 + fuzz | 无 | 4-byte LE framing、大消息、截断消息 |
| `tui` | teatest | mock client | 按键序列 → 断言输出 |
| E2E | smoke script | Chrome + extension | daemon + shim + extension 全链路 |

### 7.3 可测性设计要求

```go
// 所有 socket 路径必须可注入
func NewClient(socketPath string) *Client  // ✓ 可测
// 不能 func NewClient() *Client { path := config.SocketPath() }  // ✗ 不可测

// daemon 也一样
func StartDaemon(ctx context.Context, cfg DaemonConfig) error  // ✓
type DaemonConfig struct {
    SocketPath     string
    SessionsDir    string
    CollectionsDir string
}
```

### 7.4 CI 必跑

```bash
go test -race -count=1 ./...
go test -fuzz=FuzzNDJSON ./internal/protocol/ -fuzztime=30s
go test -fuzz=FuzzNMFrame ./internal/nmshim/ -fuzztime=30s
```

---

## 8. Stage 3-6 模块设计

领域对象定义见 `codex_doc/DOMAIN_MODEL.md`，能力域见 `codex_doc/CAPABILITY_MAP.md`。
以下是 Go 实现层面的模块设计。Phase 1 的设计约束必须兼容它们。

### 8.1 `internal/bookmarks`（Stage 3）

**职责**：BookmarkMirror 存储 + BookmarkOverlay CRUD + 搜索 + 导出

```go
// 书签节点（树结构）
type BookmarkNode struct {
    ID        string          `json:"id"`
    Title     string          `json:"title"`
    URL       string          `json:"url,omitempty"`       // 有 URL = 书签，无 URL = 文件夹
    ParentID  string          `json:"parentId,omitempty"`
    DateAdded int64           `json:"dateAdded,omitempty"`
    Children  []*BookmarkNode `json:"children,omitempty"`  // 有 Children = 文件夹
}

// CTM overlay（不写回 Chrome）
type BookmarkOverlay struct {
    BookmarkID string   `json:"bookmarkId"`
    Tags       []string `json:"tags,omitempty"`
    Note       string   `json:"note,omitempty"`
    Alias      string   `json:"alias,omitempty"`
}

// 本地镜像
type BookmarkMirror struct {
    Tree       []*BookmarkNode   `json:"tree"`
    MirroredAt string            `json:"mirroredAt"`
    TargetID   string            `json:"targetId"`
}

// 公开接口
func SaveMirror(dir string, mirror *BookmarkMirror) error
func LoadMirror(dir string, targetID string) (*BookmarkMirror, error)
func SaveOverlay(dir string, overlay *BookmarkOverlay) error
func LoadOverlay(dir string, bookmarkID string) (*BookmarkOverlay, error)
func SearchMirror(mirror *BookmarkMirror, query string) []*BookmarkNode
func ExportMarkdown(node *BookmarkNode, depth int) string
```

### 8.2 `internal/sync`（Stage 4）

**职责**：iCloud 文件同步 + 冲突检测

```go
type SyncEngine struct {
    localDir  string   // ~/.config/ctm/
    cloudDir  string   // ~/Library/Mobile Documents/com~ctm/
}

func (s *SyncEngine) Sync(ctx context.Context) (*SyncResult, error)
func (s *SyncEngine) Status() (*SyncStatus, error)
func (s *SyncEngine) Repair(ctx context.Context) error
func (s *SyncEngine) Watch(ctx context.Context) error   // fsnotify 监听云端变更

type SyncResult struct {
    Uploaded   int
    Downloaded int
    Conflicts  int
}

type SyncStatus struct {
    Enabled        bool
    SyncDir        string
    LastSync       string
    PendingChanges int
    Conflicts      []ConflictInfo
}
```

### 8.3 `internal/workspace`（Stage 6）

**职责**：Workspace CRUD + 关联资源管理

```go
// Workspace — 领域定义见 codex_doc/WORKSPACE_MODEL.md
type Workspace struct {
    ID               string   `json:"id"`
    Name             string   `json:"name"`
    Description      string   `json:"description,omitempty"`
    Sessions         []string `json:"sessions"`
    Collections      []string `json:"collections"`
    BookmarkFolderIDs []string `json:"bookmarkFolderIds"`
    SavedSearchIDs   []string `json:"savedSearchIds,omitempty"`
    Tags             []string `json:"tags,omitempty"`
    Notes            string   `json:"notes,omitempty"`
    Status           string   `json:"status,omitempty"`     // "active", "archived"
    DefaultTarget    string   `json:"defaultTarget,omitempty"`
    LastActiveAt     string   `json:"lastActiveAt,omitempty"`
    CreatedAt        string   `json:"createdAt"`
    UpdatedAt        string   `json:"updatedAt"`
}

func SaveWorkspace(dir string, ws *Workspace) error
func LoadWorkspace(dir string, id string) (*Workspace, error)
func ListWorkspaces(dir string) ([]*WorkspaceSummary, error)
func DeleteWorkspace(dir string, id string) error
```

### 8.4 Phase 1 必须满足的设计约束（为 Phase 7-8 铺路）

```go
// 1. 存储函数必须接受目录参数
func atomicWriteJSON(dir, name string, data any) error  // ✓

// 2. Hub action 路由必须是 map/switch，不是 if-else 链
// Phase 7 加 bookmarks.* 只需注册新 handler

// 3. TUI ViewType 用 iota，Phase 7 追加 ViewBookmarks / ViewWorkspaces / ViewSync
type ViewType int
const (
    ViewTargets ViewType = iota
    ViewTabs
    ViewGroups
    ViewSessions
    ViewCollections
    // Phase 7+:
    // ViewBookmarks
    // ViewWorkspaces
    // ViewSync
)

// 4. 所有持久对象必须有 id (UUID) + createdAt + updatedAt
// 这是 iCloud 同步的前提条件

// 5. NM manifest 路径和 extension ID 通过 config 包获取，不硬编码
// Phase 7 加多浏览器只需扩展 config

// 6. Extension capability 协商，Phase 7 加 "bookmarks" capability
```

### 8.5 `internal/search`（Stage 5）

**职责**：跨资源搜索 + SavedSearch 持久化

```go
// 搜索请求（维度见 codex_doc/SEARCH_MODEL.md §4）
type SearchQuery struct {
    Query   string   `json:"query"`
    Mode    string   `json:"mode,omitempty"`   // "quick", "global", "saved"; 默认 "global"
    Scopes  []string `json:"scopes,omitempty"` // tabs, sessions, collections, bookmarks, workspaces; 空=全部
    Tags    []string `json:"tags,omitempty"`
    Host    string   `json:"host,omitempty"`   // 按域名过滤
    Limit   int      `json:"limit,omitempty"`
}

// 搜索结果
type SearchResult struct {
    Kind       string `json:"kind"`       // "tab", "session", "collection", "bookmark", "workspace"
    ID         string `json:"id"`
    Title      string `json:"title"`
    URL        string `json:"url,omitempty"`
    MatchField string `json:"matchField"` // title, url, host, tag, note, alias
    Score      float64 `json:"score"`
}

// SavedSearch（可重复执行的查询定义）
type SavedSearch struct {
    ID        string      `json:"id"`
    Name      string      `json:"name"`
    Query     SearchQuery `json:"query"`
    CreatedAt string      `json:"createdAt"`
    UpdatedAt string      `json:"updatedAt"`
}

// 公开接口
func Search(query SearchQuery, sources SearchSources) ([]SearchResult, error)
func SaveSearch(dir string, ss *SavedSearch) error
func LoadSavedSearches(dir string) ([]*SavedSearch, error)
func DeleteSavedSearch(dir string, id string) error
```

### 8.6 Sync 相关 Go 类型（Stage 4 实现时使用）

领域概念见 `codex_doc/DOMAIN_MODEL.md` §5（SyncAccount / SyncState / Device）。

```go
type SyncAccount struct {
    ID       string `json:"id"`
    Kind     string `json:"kind"`     // "google", "icloud"
    Label    string `json:"label"`
    Enabled  bool   `json:"enabled"`
}

// SyncState — 状态值见 codex_doc/SYNC_MODEL.md §8
type SyncState struct {
    ResourceID   string `json:"resourceId"`
    ResourceKind string `json:"resourceKind"` // "session", "collection", "workspace", "overlay", "saved_search"
    SyncDomain   string `json:"syncDomain"`   // "library", "knowledge", "system"
    SyncedAt     string `json:"syncedAt,omitempty"`
    Status       string `json:"status"` // "local_only", "syncing", "synced", "stale", "conflicted", "failed", "disabled"
    ConflictFile string `json:"conflictFile,omitempty"`
}

type Device struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Platform string `json:"platform"` // "darwin", "linux"
    LastSeen string `json:"lastSeen"`
}
```

### 8.7 不做的东西

- 除 iCloud 外的 Cloud sync — YAGNI
- 数据库后端 — JSON 文件足够
- 插件系统 — 过度设计
- 自己实现 Google OAuth — Chrome Sync 已处理
