# CTM -- 终端优先的浏览器工作区管理器

CTM 是一个单二进制 Go 程序，从终端完整控制 Chrome。通过 Chrome Extension + Native Messaging + Unix Socket 管道连接，实现对标签页、分组、会话、收藏集、书签、工作区、搜索和跨设备同步的全面管理 -- 无需离开终端。

## 功能亮点

### 浏览器运行态控制
- 列出、打开、关闭、激活、固定、静音、移动标签页
- 创建和管理标签页分组
- 截图和提取页面文字
- 多目标支持（同时控制多个 Chrome 实例）

### 长期资源管理
- **Sessions（会话）** -- 保存和恢复完整的浏览器快照（窗口、标签页、分组）
- **Collections（收藏集）** -- 策展和整理链接集合，便于复用
- **Bookmarks（书签）** -- 镜像 Chrome 书签，通过 overlay 层添加标签/备注/别名

### 搜索和工作区
- **Search（搜索）** -- 跨资源统一搜索，覆盖标签页、会话、收藏集、书签和工作区
- **Workspaces（工作区）** -- 组织长期任务上下文，聚合会话、收藏集和书签

### 同步和诊断
- **Sync（同步）** -- 基于 iCloud 的跨设备同步，支持状态可见和冲突解决
- **Doctor（诊断）** -- 完整的安装、守护进程和扩展连通性健康检查

## 架构

```
Chrome Extension (JS)
    |
    | Chrome Native Messaging (stdin/stdout, 4 字节长度前缀)
    |
ctm nm-shim (Go)
    |
    | NDJSON over Unix socket
    |
ctm daemon (Go, 长驻进程, Hub actor 模式)
    |
    +--- ctm tui   (Bubble Tea, 持久连接)
    +--- ctm cli   (Cobra, 一次性连接)
```

所有组件编译为单一 `ctm` 二进制文件，通过 Cobra 子命令区分角色。

### 模块结构

```
cmd/                  CLI 入口（Cobra 命令）
internal/
  config/             路径解析和常量（叶节点）
  protocol/           消息类型、NDJSON 编解码、ID 生成（叶节点）
  client/             守护进程连接、请求/响应、重连
  daemon/             Hub actor、路由、持久化、socket 服务
  nmshim/             Chrome NM 4 字节帧 <-> NDJSON 桥接
  tui/                Bubble Tea TUI（11 个视图）
  bookmarks/          书签镜像、overlay、导出、搜索
  search/             跨资源搜索引擎、保存的搜索
  sync/               iCloud 同步引擎、冲突解决
  workspace/          工作区聚合和启动
```

## 环境要求

- **Go 1.24+**（或下载预编译 release）
- **macOS**（LaunchAgent 自动启动守护进程；Linux 可构建但无自动启动）
- **Google Chrome** 或 Chrome Beta
- **CTM Chrome Extension**（作为未打包扩展加载）

## 快速开始

### 1. 构建和安装

```bash
git clone <repo-url> && cd chrome-tab-manager
make build            # 生成 ./ctm 二进制文件
make install          # 安装 LaunchAgent 用于守护进程自动启动
```

### 2. 连接 Chrome Extension

在 `chrome://extensions` 中找到扩展 ID，然后注册 Native Messaging 主机：

```bash
ctm install --extension-id=<your-extension-id>
```

### 3. 启动守护进程

LaunchAgent 会自动启动守护进程。手动启动：

```bash
ctm daemon --foreground
```

### 4. 验证安装

```bash
ctm doctor
```

### 5. 开始使用

```bash
ctm tabs list
ctm tui
```

## CLI 用法

```bash
# 标签页
ctm tabs list [--json]
ctm tabs open <url> [--active] [--deduplicate]
ctm tabs close <tabId>
ctm tabs activate <tabId> [--focus]
ctm tabs mute <tabId>
ctm tabs pin <tabId>
ctm tabs move <tabId> [--window=N] [--index=N]
ctm tabs text <tabId>
ctm tabs capture [tabId] [-o file.png]

# 分组
ctm groups list [--json]

# 会话
ctm sessions list [--json]
ctm sessions save <name>
ctm sessions get <name>
ctm sessions restore <name>
ctm sessions delete <name>

# 收藏集
ctm collections list [--json]
ctm collections create <name>
ctm collections get <name>
ctm collections add <name> <url> [title]
ctm collections restore <name>
ctm collections delete <name>

# 书签
ctm bookmarks tree [--depth=N]
ctm bookmarks search <query>
ctm bookmarks export [--format=md|json]

# 搜索
ctm search <query> [--scope=tabs,sessions,...]

# 工作区
ctm workspaces list
ctm workspaces create <name>
ctm workspaces switch <name>

# 目标
ctm targets list [--json]
ctm targets default [targetId]

# 系统
ctm daemon [--foreground]
ctm install [--extension-id=ID] [--check]
ctm doctor [--extension-id=ID]
ctm tui
ctm version
```

所有资源命令支持 `--target=<id>` 来选择特定浏览器实例。

## TUI 快捷键

### 全局

| 快捷键      | 操作         |
|------------|-------------|
| `q`        | 退出         |
| `Esc`      | 取消/关闭     |
| `?`        | 帮助         |
| `:`        | 命令模式      |
| `/`        | 过滤         |
| `r`        | 刷新         |
| `Tab`      | 切换视图      |
| `1`-`9`    | 跳转视图      |

### 导航

| 快捷键         | 操作   |
|---------------|--------|
| `j` / `Down`  | 下移   |
| `k` / `Up`    | 上移   |
| `gg`          | 到顶部  |
| `G`           | 到底部  |
| `Space`       | 选中   |
| `Enter`       | 执行   |

### Tabs（标签页）

| 快捷键 | 操作            |
|-------|----------------|
| `x`   | 关闭标签页       |
| `m`   | 静音/取消静音    |
| `p`   | 固定/取消固定    |
| `v`   | 切换预览        |
| `s`   | 截图（外部打开）  |
| `M`   | 移动到窗口      |
| `A`   | 添加到收藏集     |
| `n`   | 分组选中项       |
| `y.`  | 复制            |

### Sessions（会话）

| 快捷键   | 操作   |
|---------|--------|
| `o`     | 恢复   |
| `n`     | 新建   |
| `x x`   | 删除   |

### Collections（收藏集）

| 快捷键    | 操作          |
|----------|--------------|
| `o`      | 恢复          |
| `n`      | 新建          |
| `e`      | 重命名        |
| `x`      | 移除项目       |
| `x x`    | 删除收藏集     |
| `J`/`K`  | 移动项目       |

### Bookmarks（书签）

| 快捷键         | 操作       |
|---------------|-----------|
| `a`           | 添加书签    |
| `E`           | 导出       |
| `l` / `Right` | 展开文件夹  |
| `h` / `Left`  | 折叠       |
| `D D`         | 删除       |
| `zM`          | 全部折叠    |
| `zR`          | 全部展开    |

### Workspaces（工作区）

| 快捷键  | 操作   |
|--------|--------|
| `o`    | 切换   |
| `n`    | 新建   |
| `e`    | 改名   |
| `D D`  | 删除   |

## 开发

### 构建

```bash
make build                    # 构建二进制
make all                      # vet + test + build
```

### 测试

```bash
make test                     # go test -race ./...
go test -race ./internal/protocol/
go test -race ./internal/config/

# Fuzz 测试
go test -fuzz=FuzzNDJSON ./internal/protocol/ -fuzztime=30s
go test -fuzz=FuzzNMFrame ./internal/nmshim/ -fuzztime=30s
```

### 检查

```bash
make lint                     # go vet
```

### 发布

使用 [GoReleaser](https://goreleaser.com/) 构建 release。支持平台：darwin/amd64, darwin/arm64, linux/amd64, linux/arm64。

```bash
make release-dry              # 本地快照构建
goreleaser release             # 基于 tag 的正式发布
```

## 项目结构

| 目录          | 说明                                              |
|--------------|--------------------------------------------------|
| `cmd/`       | CLI 命令（Cobra）                                  |
| `internal/`  | 所有 Go 包（config, protocol, client, daemon, tui, ...） |
| `doc/`       | 设计文档和规格说明                                   |
| `extension/` | Chrome Extension（Manifest V3）                    |
| `Makefile`   | 构建、测试和发布目标                                  |

## 许可证

MIT
