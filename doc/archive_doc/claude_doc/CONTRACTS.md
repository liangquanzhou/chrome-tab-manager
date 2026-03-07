# CTM — Protocol & API Contracts

每个 daemon action 的 request/response 精确定义。
TUI/CLI 开发者必须按此文档编码，不可猜测 response shape。

## 通用规则

- 所有消息必须包含 `id`, `protocol_version`, `type`
- Request 必须包含 `action`
- Response 的 `id` 必须与 Request 的 `id` 一致
- Error 必须包含 `error.code` 和 `error.message`
- Payload 中不包含的字段表示该字段不适用，不要用 null

## Hello

Extension → Daemon 注册

```json
// Request (from extension via NM shim)
{
  "id": "hello_1",
  "protocol_version": 1,
  "type": "hello",
  "payload": {
    "channel": "stable",
    "extensionId": "abc123...",
    "instanceId": "uuid-v4",
    "userAgent": "Chrome/130.0...",
    "capabilities": ["tabs", "groups", "events"],
    "min_supported": 1
  }
}

// Response (from daemon)
{
  "id": "hello_1",
  "protocol_version": 1,
  "type": "response",
  "action": "hello",
  "payload": {
    "targetId": "target_1"
  }
}
```

## targets.list

列出当前在线的 targets。

```json
// Request
{ "action": "targets.list", "payload": {} }

// Response payload
{
  "targets": [
    {
      "targetId": "target_1",
      "channel": "stable",
      "label": "work",        // 用户自定义标签，可为 null
      "isDefault": true,
      "userAgent": "Chrome/130.0...",
      "connectedAt": 1709827200000
    }
  ]
}
```

## targets.default

设置默认 target。

```json
// Request
{ "action": "targets.default", "payload": { "targetId": "target_1" } }

// Response payload
{ "targetId": "target_1" }
```

## targets.clearDefault

清除默认 target。

```json
// Request
{ "action": "targets.clearDefault", "payload": {} }

// Response payload
{}
```

## targets.label

给 target 设置标签。

```json
// Request
{ "action": "targets.label", "payload": { "targetId": "target_1", "label": "work" } }

// Response payload
{ "targetId": "target_1", "label": "work" }
```

## tabs.list

列出指定 target 的所有 tabs。**转发到 extension**。

```json
// Request
{ "action": "tabs.list", "target": { "targetId": "target_1" }, "payload": {} }

// Response payload
{
  "tabs": [
    {
      "id": 12345,
      "windowId": 1,
      "index": 0,
      "title": "Google",
      "url": "https://google.com",
      "active": true,
      "pinned": false,
      "groupId": -1,
      "favIconUrl": "https://..."
    }
  ]
}
```

**注意**：`groupId: -1` 表示不在任何 group 中。

## tabs.open

打开新 tab 或切换到已有 tab。**转发到 extension**。

```json
// Request
{
  "action": "tabs.open",
  "target": { "targetId": "target_1" },
  "payload": {
    "url": "https://example.com",
    "active": true,
    "focus": true,           // 是否将 Chrome 窗口带到前台
    "deduplicate": true,     // 是否查找已打开的相同 URL
    "windowId": 1            // 可选，指定窗口
  }
}

// Response payload
{
  "tabId": 12346,
  "windowId": 1,
  "reused": false   // true 表示切换到已有 tab 而非新建
}
```

## tabs.close

关闭 tab。**转发到 extension**。

```json
// Request
{ "action": "tabs.close", "target": { "targetId": "target_1" }, "payload": { "tabId": 12345 } }

// Response payload
{}
```

## tabs.activate

激活 tab。**转发到 extension**。

```json
// Request
{
  "action": "tabs.activate",
  "target": { "targetId": "target_1" },
  "payload": { "tabId": 12345, "focus": true }
}

// Response payload
{}
```

## tabs.update

更新 tab 属性。**转发到 extension**。

```json
// Request
{
  "action": "tabs.update",
  "target": { "targetId": "target_1" },
  "payload": { "tabId": 12345, "pinned": true }
}

// Response payload
{}
```

## groups.list

列出 tab groups。**转发到 extension**。

```json
// Request
{ "action": "groups.list", "target": { "targetId": "target_1" }, "payload": {} }

// Response payload
{
  "groups": [
    {
      "id": 1,
      "title": "Work",
      "color": "blue",
      "collapsed": false,
      "windowId": 1
    }
  ]
}
```

## groups.create

创建 tab group。**转发到 extension**。

```json
// Request
{
  "action": "groups.create",
  "target": { "targetId": "target_1" },
  "payload": {
    "tabIds": [12345, 12346],
    "title": "Work",
    "color": "blue"
  }
}

// Response payload
{ "groupId": 2 }
```

## groups.update

更新 group 属性。**转发到 extension**。

```json
// Request
{
  "action": "groups.update",
  "target": { "targetId": "target_1" },
  "payload": {
    "groupId": 2,
    "title": "Updated",
    "color": "red",
    "collapsed": true
  }
}

// Response payload
{}
```

## groups.delete

删除（解散）group。**转发到 extension**。

```json
// Request
{ "action": "groups.delete", "target": { "targetId": "target_1" }, "payload": { "groupId": 2 } }

// Response payload
{}
```

## sessions.list

列出已保存的 sessions。**daemon 本地处理**。

```json
// Request
{ "action": "sessions.list", "payload": {} }

// Response payload — 返回 SUMMARY，不含 tabs 详情
{
  "sessions": [
    {
      "name": "work-afternoon",
      "tabCount": 15,
      "windowCount": 2,
      "groupCount": 3,
      "createdAt": "2026-03-07T10:00:00Z",
      "sourceTarget": "target_1"
    }
  ]
}
```

**重要**：list 返回 summary（`tabCount`），不返回完整 tabs 数组。要获取完整数据用 `sessions.get`。

## sessions.get

获取单个 session 的完整数据。**daemon 本地处理**。

```json
// Request
{ "action": "sessions.get", "payload": { "name": "work-afternoon" } }

// Response payload — 返回完整数据
{
  "session": {
    "name": "work-afternoon",
    "createdAt": "2026-03-07T10:00:00Z",
    "sourceTarget": "target_1",
    "windows": [
      {
        "tabs": [
          { "url": "https://...", "title": "...", "pinned": false, "active": true, "groupIndex": 0 }
        ]
      }
    ],
    "groups": [
      { "title": "Work", "color": "blue", "collapsed": false }
    ]
  }
}
```

## sessions.save

保存当前 target 的 tabs 为 session。**daemon + extension 配合**。

```json
// Request
{ "action": "sessions.save", "target": { "targetId": "target_1" }, "payload": { "name": "work-afternoon" } }

// Response payload
{ "name": "work-afternoon", "tabCount": 15, "windowCount": 2, "groupCount": 3 }
```

## sessions.restore

恢复 session 到指定 target。**daemon + extension 配合**。

```json
// Request
{ "action": "sessions.restore", "target": { "targetId": "target_1" }, "payload": { "name": "work-afternoon" } }

// Response payload
{ "windowsCreated": 2, "tabsOpened": 15, "tabsFailed": 0, "groupsCreated": 3 }
```

**恢复规则**（TS 版 v9 最终逻辑）：
1. 按 `windows[]` 逐个创建窗口（第一个 tab URL 作为窗口初始 URL）
2. 追加后续 tabs 到该窗口
3. 所有 tabs 就位后，按 `groups[]` 创建 tab groups
4. tab 打开失败 → 跳过继续，不阻塞
5. group 创建失败 → 记 warn，不阻塞

## sessions.delete

删除 session 文件。**daemon 本地处理**。

```json
// Request
{ "action": "sessions.delete", "payload": { "name": "work-afternoon" } }

// Response payload
{}
```

## collections.list

列出所有 collections。**daemon 本地处理**。

```json
// Request
{ "action": "collections.list", "payload": {} }

// Response payload — 返回 SUMMARY
{
  "collections": [
    {
      "name": "favorites",
      "itemCount": 8,
      "createdAt": "2026-03-07T10:00:00Z",
      "updatedAt": "2026-03-07T12:00:00Z"
    }
  ]
}
```

**重要**：list 返回 `itemCount`，不返回 `items[]`。要获取完整数据用 `collections.get`。

## collections.get

获取单个 collection 的完整数据。

```json
// Request
{ "action": "collections.get", "payload": { "name": "favorites" } }

// Response payload
{
  "collection": {
    "name": "favorites",
    "createdAt": "2026-03-07T10:00:00Z",
    "updatedAt": "2026-03-07T12:00:00Z",
    "items": [
      { "url": "https://...", "title": "Example", "groupLabel": "Work" }
    ]
  }
}
```

## collections.create / delete / addItems / removeItems / addFromTabs / restore

（格式同理，参考 sessions 的模式）

## subscribe

注册事件订阅。

```json
// Request
{ "action": "subscribe", "payload": { "patterns": ["tabs.*", "groups.*"] } }

// Response payload
{ "subscribed": true }
```

订阅后，匹配的事件会作为 `type: "event"` 推送：

```json
{
  "id": "evt_1",
  "protocol_version": 1,
  "type": "event",
  "action": "tabs.created",
  "payload": {
    "tab": { "id": 12347, "windowId": 1, "url": "...", "title": "..." },
    "_target": { "targetId": "target_1" }
  }
}
```

**重要**：每个 event 的 payload 中必须包含 `_target.targetId`，用于多 target 场景下的事件隔离。

## daemon.stop

停止 daemon 进程。

```json
// Request
{ "action": "daemon.stop", "payload": {} }

// Response payload（daemon 发完即关闭）
{ "stopping": true }
```

---

## Phase 7+ Actions: Bookmarks

### bookmarks.tree

获取完整书签树。**daemon 转发到 extension**。

```json
// Request
{ "action": "bookmarks.tree", "target": { "targetId": "target_1" }, "payload": {} }

// Response payload
{
  "tree": [
    {
      "id": "0",
      "title": "",
      "children": [
        {
          "id": "1",
          "title": "Bookmarks Bar",
          "children": [
            {
              "id": "42",
              "title": "GitHub",
              "url": "https://github.com",
              "dateAdded": 1709827200000
            },
            {
              "id": "43",
              "title": "Work",
              "children": [...]
            }
          ]
        }
      ]
    }
  ]
}
```

**注意**：书签是树结构。`children` 存在 = 文件夹，`url` 存在 = 书签。两者互斥。

### bookmarks.search

搜索书签。**daemon 转发到 extension**。

```json
// Request
{ "action": "bookmarks.search", "target": { "targetId": "target_1" }, "payload": { "query": "github" } }

// Response payload
{
  "bookmarks": [
    { "id": "42", "title": "GitHub", "url": "https://github.com", "parentId": "1", "dateAdded": 1709827200000 }
  ]
}
```

### bookmarks.get

获取单个书签节点。**daemon 转发到 extension**。

```json
// Request
{ "action": "bookmarks.get", "target": { "targetId": "target_1" }, "payload": { "id": "42" } }

// Response payload
{
  "bookmark": { "id": "42", "title": "GitHub", "url": "https://github.com", "parentId": "1", "dateAdded": 1709827200000 }
}
```

### bookmarks.mirror

将 Chrome 书签树镜像到 CTM 本地存储。**daemon 处理**：先 bookmarks.tree 获取，然后保存到本地。

```json
// Request
{ "action": "bookmarks.mirror", "target": { "targetId": "target_1" }, "payload": {} }

// Response payload
{ "nodeCount": 1523, "folderCount": 87, "mirroredAt": "2026-03-07T10:00:00Z" }
```

### bookmarks.overlay.set

给书签设置 CTM overlay（tag/note/alias）。**daemon 本地处理**，不修改 Chrome 书签。

```json
// Request
{
  "action": "bookmarks.overlay.set",
  "payload": {
    "bookmarkId": "42",
    "tags": ["work", "dev"],
    "note": "Main repo dashboard",
    "alias": "gh"
  }
}

// Response payload
{ "bookmarkId": "42", "tags": ["work", "dev"], "note": "Main repo dashboard", "alias": "gh" }
```

### bookmarks.overlay.get

获取书签的 CTM overlay。**daemon 本地处理**。

```json
// Request
{ "action": "bookmarks.overlay.get", "payload": { "bookmarkId": "42" } }

// Response payload
{ "bookmarkId": "42", "tags": ["work", "dev"], "note": "Main repo dashboard", "alias": "gh" }
```

### bookmarks.export

导出书签为 Markdown。**daemon 本地处理**。

```json
// Request
{ "action": "bookmarks.export", "payload": { "folderId": "1", "format": "markdown" } }

// Response payload
{ "content": "# Bookmarks Bar\n\n- [GitHub](https://github.com)\n- Work\n  - [Jira](https://...)\n" }
```

### 书签事件

Extension 监听 `chrome.bookmarks.onCreated/onChanged/onRemoved`，推送事件。

```json
// Event: bookmarks.created
{
  "type": "event",
  "action": "bookmarks.created",
  "payload": {
    "bookmark": { "id": "99", "title": "New Page", "url": "https://...", "parentId": "1" },
    "_target": { "targetId": "target_1" }
  }
}

// Event: bookmarks.changed
{
  "type": "event",
  "action": "bookmarks.changed",
  "payload": {
    "id": "42",
    "changes": { "title": "GitHub - Home" },
    "_target": { "targetId": "target_1" }
  }
}

// Event: bookmarks.removed
{
  "type": "event",
  "action": "bookmarks.removed",
  "payload": {
    "id": "42",
    "removeInfo": { "parentId": "1", "index": 3 },
    "_target": { "targetId": "target_1" }
  }
}
```

---

## Stage 5 Actions: Search

### search.query

跨资源统一搜索。**daemon 本地处理**。

```json
// Request
{
  "action": "search.query",
  "payload": {
    "query": "github",
    "mode": "global",
    "scopes": ["tabs", "sessions", "collections", "bookmarks", "workspaces"],
    "tags": [],
    "host": "",
    "limit": 50
  }
}

// Response payload
{
  "results": [
    { "kind": "tab", "id": "12345", "title": "GitHub", "url": "https://github.com", "matchField": "title", "score": 1.0 },
    { "kind": "bookmark", "id": "42", "title": "GitHub - Home", "url": "https://github.com", "matchField": "url", "score": 0.9 },
    { "kind": "collection", "id": "uuid-1", "title": "dev-tools", "matchField": "item.url", "score": 0.7 }
  ],
  "total": 3
}
```

**注意**：`scopes` 为空时搜索所有资源。搜索结果按 `score` 降序排列。

### search.saved.list

列出已保存的搜索。**daemon 本地处理**。

```json
// Request
{ "action": "search.saved.list", "payload": {} }

// Response payload
{
  "searches": [
    { "id": "ss_uuid_1", "name": "work-repos", "query": { "query": "github", "tags": ["work"] }, "createdAt": "...", "updatedAt": "..." }
  ]
}
```

### search.saved.create / search.saved.delete

```json
// Create
{ "action": "search.saved.create", "payload": { "name": "work-repos", "query": { "query": "github", "tags": ["work"] } } }
// Response: { "id": "ss_uuid_1", "name": "work-repos" }

// Delete
{ "action": "search.saved.delete", "payload": { "id": "ss_uuid_1" } }
// Response: {}
```

---

## Stage 6+ Actions: Workspace & Stage 4+ Actions: Sync

### workspace.list

列出所有 workspace。**daemon 本地处理**。

```json
// Request
{ "action": "workspace.list", "payload": {} }

// Response payload
{
  "workspaces": [
    {
      "id": "ws_uuid_1",
      "name": "frontend-project",
      "sessionCount": 2,
      "collectionCount": 3,
      "bookmarkFolderIds": ["43"],
      "createdAt": "2026-03-07T10:00:00Z",
      "updatedAt": "2026-03-07T12:00:00Z"
    }
  ]
}
```

### workspace.get

获取 workspace 完整数据。**daemon 本地处理**。

```json
// Request
{ "action": "workspace.get", "payload": { "id": "ws_uuid_1" } }

// Response payload
{
  "workspace": {
    "id": "ws_uuid_1",
    "name": "frontend-project",
    "description": "React migration project workspace",
    "sessions": ["morning-tabs", "afternoon-tabs"],
    "collections": ["ui-references", "api-docs", "design-inspo"],
    "bookmarkFolderIds": ["43"],
    "savedSearchIds": ["ss_uuid_1"],
    "tags": ["frontend", "active"],
    "notes": "React migration project",
    "status": "active",
    "defaultTarget": "target_1",
    "lastActiveAt": "2026-03-07T11:30:00Z",
    "createdAt": "2026-03-07T10:00:00Z",
    "updatedAt": "2026-03-07T12:00:00Z"
  }
}
```

### workspace.create / workspace.update / workspace.delete

```json
// Create
{ "action": "workspace.create", "payload": { "name": "frontend-project" } }
// Response: { "id": "ws_uuid_1", "name": "frontend-project" }

// Update
{ "action": "workspace.update", "payload": { "id": "ws_uuid_1", "sessions": ["morning-tabs"], "tags": ["frontend"] } }
// Response: {}

// Delete (需确认)
{ "action": "workspace.delete", "payload": { "id": "ws_uuid_1" } }
// Response: {}
```

### workspace.switch

切换到指定 workspace：关闭当前所有 tabs，恢复 workspace 关联的 session。**daemon + extension 配合**。

```json
// Request
{ "action": "workspace.switch", "target": { "targetId": "target_1" }, "payload": { "id": "ws_uuid_1" } }

// Response payload
{ "tabsClosed": 24, "windowsCreated": 2, "tabsOpened": 18, "tabsFailed": 0 }
```

### sync.status

获取 iCloud 同步状态。**daemon 本地处理**。

```json
// Request
{ "action": "sync.status", "payload": {} }

// Response payload
{
  "enabled": true,
  "syncDir": "~/Library/Mobile Documents/com~ctm/",
  "lastSync": "2026-03-07T12:00:00Z",
  "pendingChanges": 0,
  "conflicts": []
}
```

### sync.repair

修复/重建 iCloud 同步状态。**daemon 本地处理**。

```json
// Request
{ "action": "sync.repair", "payload": {} }

// Response payload
{ "reindexed": true, "objectCount": 42, "conflictsResolved": 1 }
```

---

## 通用数据模型补充（Phase 7+ 生效）

### 持久对象通用字段

从 Phase 1 开始，所有持久化对象（session, collection）必须包含：

```json
{
  "id": "uuid-v4",        // 稳定 ID，独立于 name（name 可改，id 不变）
  "name": "...",
  "createdAt": "ISO8601",
  "updatedAt": "ISO8601"
}
```

**设计理由**：iCloud 同步需要稳定 ID 和修改时间来做 last-write-wins 冲突检测。
从 Phase 1 就加入这些字段，避免后续数据迁移。

### BookmarkNode

```json
{
  "id": "chrome-bookmark-id",
  "title": "...",
  "url": "...",              // 仅书签有，文件夹无
  "parentId": "...",
  "dateAdded": 1709827200000,
  "children": [...]          // 仅文件夹有
}
```

### BookmarkOverlay（CTM 自有，不写回 Chrome）

```json
{
  "bookmarkId": "42",
  "tags": ["work", "dev"],
  "note": "...",
  "alias": "gh"
}
```

### Workspace

```json
{
  "id": "ws_uuid_1",
  "name": "frontend-project",
  "description": "...",
  "sessions": ["session-name-1", "session-name-2"],
  "collections": ["coll-name-1"],
  "bookmarkFolderIds": ["43", "87"],
  "savedSearchIds": ["ss_uuid_1"],
  "tags": ["frontend", "active"],
  "notes": "...",
  "status": "active",
  "defaultTarget": "target_1",
  "lastActiveAt": "ISO8601",
  "createdAt": "ISO8601",
  "updatedAt": "ISO8601"
}
```
