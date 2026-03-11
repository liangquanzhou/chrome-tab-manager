# CTM -- Terminal-First Browser Workspace Manager

[中文文档](README_CN.md)

CTM is a single-binary Go program that controls Chrome from the terminal.
It connects via a Chrome Extension + Native Messaging + Unix Socket pipeline,
giving you full command over tabs, groups, sessions, collections, bookmarks,
workspaces, search, and cross-device sync -- all without leaving the terminal.

## Feature Highlights

### Runtime Control
- List, open, close, activate, pin, mute, and move tabs
- Create and manage tab groups
- Capture screenshots and extract page text
- Multi-target support (control multiple Chrome instances)

### Library Management
- **Sessions** -- save and restore complete browser snapshots (windows, tabs, groups)
- **Collections** -- curate and organize link bundles for reuse
- **Bookmarks** -- mirror Chrome bookmarks, add tags/notes/aliases via overlay layer

### Search and Workspaces
- **Search** -- cross-resource unified search across tabs, sessions, collections, bookmarks, and workspaces
- **Workspaces** -- organize long-lived task contexts that aggregate sessions, collections, and bookmarks

### Sync and Diagnostics
- **Sync** -- iCloud-based cross-device sync for CTM-owned resources with visible status and conflict resolution
- **Doctor** -- full health check of installation, daemon, and extension connectivity

## Architecture

```
Chrome Extension (JS)
    |
    | Chrome Native Messaging (stdin/stdout, 4-byte length prefix)
    |
ctm nm-shim (Go)
    |
    | NDJSON over Unix socket
    |
ctm daemon (Go, long-running, Hub actor pattern)
    |
    +--- ctm tui   (Bubble Tea, persistent connection)
    +--- ctm cli   (Cobra, one-shot connection)
```

All components compile into a single `ctm` binary. Cobra subcommands select the role.

### Module Layout

```
cmd/                  CLI entry points (Cobra commands)
internal/
  config/             Path resolution and constants (leaf)
  protocol/           Message types, NDJSON codec, ID generation (leaf)
  client/             Daemon connection, request/response, reconnect
  daemon/             Hub actor, routing, persistence, socket server
  nmshim/             Chrome NM 4-byte framing <-> NDJSON bridge
  tui/                Bubble Tea TUI (11 views)
  bookmarks/          Bookmark mirror, overlay, export, search
  search/             Cross-resource search engine, saved searches
  sync/               iCloud sync engine, conflict resolution
  workspace/          Workspace aggregation and startup
```

## Prerequisites

- **Go 1.24+** (or download a prebuilt release)
- **macOS** (LaunchAgent for daemon auto-start; Linux builds available without auto-start)
- **Google Chrome** or Chrome Beta
- **CTM Chrome Extension** (loaded as unpacked extension)

## Quick Start

### 1. Build and install

```bash
git clone <repo-url> && cd chrome-tab-manager
make build            # produces ./ctm binary
make install          # installs LaunchAgent for daemon auto-start
```

### 2. Connect the Chrome Extension

Find your extension ID at `chrome://extensions`, then register the Native Messaging host:

```bash
ctm install --extension-id=<your-extension-id>
```

### 3. Start the daemon

The LaunchAgent starts the daemon automatically. To run manually:

```bash
ctm daemon --foreground
```

### 4. Verify the setup

```bash
ctm doctor
```

### 5. Use it

```bash
ctm tabs list
ctm tui
```

## CLI Usage

```bash
# Tabs
ctm tabs list [--json]
ctm tabs open <url> [--active] [--deduplicate]
ctm tabs close <tabId>
ctm tabs activate <tabId> [--focus]
ctm tabs mute <tabId>
ctm tabs pin <tabId>
ctm tabs move <tabId> [--window=N] [--index=N]
ctm tabs text <tabId>
ctm tabs capture [tabId] [-o file.png]

# Groups
ctm groups list [--json]

# Sessions
ctm sessions list [--json]
ctm sessions save <name>
ctm sessions get <name>
ctm sessions restore <name>
ctm sessions delete <name>

# Collections
ctm collections list [--json]
ctm collections create <name>
ctm collections get <name>
ctm collections add <name> <url> [title]
ctm collections restore <name>
ctm collections delete <name>

# Bookmarks
ctm bookmarks tree [--depth=N]
ctm bookmarks search <query>
ctm bookmarks export [--format=md|json]

# Search
ctm search <query> [--scope=tabs,sessions,...]

# Workspaces
ctm workspaces list
ctm workspaces create <name>
ctm workspaces switch <name>

# Targets
ctm targets list [--json]
ctm targets default [targetId]

# System
ctm daemon [--foreground]
ctm install [--extension-id=ID] [--check]
ctm doctor [--extension-id=ID]
ctm tui
ctm version
```

All resource commands accept `--target=<id>` to select a specific browser instance.

## TUI Key Bindings

### Global

| Key       | Action         |
|-----------|----------------|
| `q`       | Quit           |
| `Esc`     | Cancel / close |
| `?`       | Help           |
| `:`       | Command mode   |
| `/`       | Filter         |
| `r`       | Refresh        |
| `Tab`     | Next view      |
| `1`-`9`   | Switch view    |

### Navigation

| Key       | Action   |
|-----------|----------|
| `j` / `Down`  | Down     |
| `k` / `Up`    | Up       |
| `gg`      | Top      |
| `G`       | Bottom   |
| `Space`   | Select   |
| `Enter`   | Action   |

### Tabs View

| Key   | Action             |
|-------|--------------------|
| `x`   | Close tab          |
| `m`   | Mute / unmute      |
| `p`   | Pin / unpin        |
| `v`   | Toggle preview     |
| `M`   | Move to window     |
| `A`   | Add to collection  |
| `n`   | Group selected     |
| `y.`  | Yank / copy        |

### Sessions View

| Key     | Action     |
|---------|------------|
| `o`     | Restore    |
| `n`     | Save new   |
| `x x`   | Delete     |

### Collections View

| Key     | Action            |
|---------|-------------------|
| `o`     | Restore           |
| `n`     | Create new        |
| `e`     | Rename            |
| `x`     | Remove item       |
| `x x`   | Delete collection |
| `J`/`K` | Move item         |

### Bookmarks View

| Key       | Action           |
|-----------|------------------|
| `a`       | Add bookmark     |
| `E`       | Export           |
| `l` / `Right` | Expand folder |
| `h` / `Left`  | Collapse      |
| `D D`     | Delete           |
| `zM`      | Fold all         |
| `zR`      | Unfold all       |

### Workspaces View

| Key   | Action     |
|-------|------------|
| `o`   | Switch     |
| `n`   | Create new |
| `e`   | Edit name  |
| `D D` | Delete     |

## Development

### Build

```bash
make build                    # build binary
make all                      # vet + test + build
```

### Test

```bash
make test                     # go test -race ./...
go test -race ./internal/protocol/
go test -race ./internal/config/

# Fuzz testing
go test -fuzz=FuzzNDJSON ./internal/protocol/ -fuzztime=30s
go test -fuzz=FuzzNMFrame ./internal/nmshim/ -fuzztime=30s
```

### Lint

```bash
make lint                     # go vet
```

### Release

Releases are built with [GoReleaser](https://goreleaser.com/). Supported platforms: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64.

```bash
make release-dry              # local snapshot build
goreleaser release             # tag-based release
```

## Project Structure

| Directory     | Description                                       |
|---------------|---------------------------------------------------|
| `cmd/`        | CLI commands (Cobra)                              |
| `internal/`   | All Go packages (config, protocol, client, daemon, tui, ...) |
| `doc/`        | Design documents and specifications               |
| `Makefile`    | Build, test, and release targets                  |

## License

MIT
