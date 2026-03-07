# CTM — Protocol & API Contracts

每个 daemon action 的 request/response 精确定义。
TUI/CLI 开发者必须按此文档编码，不可猜测 response shape。

## General Rules

- 所有消息必须包含 `id`, `protocol_version`, `type`
- Request 必须包含 `action`
- Response 的 `id` 必须与 Request 一致
- Error 必须包含 `error.code` 和 `error.message`
- 不包含的字段表示不适用，不要用 null
- 所有持久对象必须有 `id` (UUID), `createdAt`, `updatedAt`

## Hello

Extension → Daemon 注册。

```json
// Request (extension via NM shim)
{ "type": "hello", "payload": {
    "channel": "stable", "extensionId": "abc123...", "instanceId": "uuid-v4",
    "userAgent": "Chrome/130.0...",
    "capabilities": ["tabs", "groups", "events"],
    "min_supported": 1
}}

// Response
{ "type": "response", "action": "hello", "payload": { "targetId": "target_1" }}
```

## targets.list

```json
// Response payload
{ "targets": [{
    "targetId": "target_1", "channel": "stable", "label": "work",
    "isDefault": true, "userAgent": "Chrome/130.0...", "connectedAt": 1709827200000
}]}
```

## targets.default / targets.clearDefault / targets.label

```json
// targets.default
{ "action": "targets.default", "payload": { "targetId": "target_1" }}
// Response: { "targetId": "target_1" }

// targets.clearDefault
{ "action": "targets.clearDefault", "payload": {}}
// Response: {}

// targets.label
{ "action": "targets.label", "payload": { "targetId": "target_1", "label": "work" }}
// Response: { "targetId": "target_1", "label": "work" }
```

## tabs.list (→ extension)

```json
// Response payload
{ "tabs": [{
    "id": 12345, "windowId": 1, "index": 0,
    "title": "Google", "url": "https://google.com",
    "active": true, "pinned": false, "groupId": -1, "favIconUrl": "https://..."
}]}
```

`groupId: -1` = 不在任何 group。

## tabs.open (→ extension)

```json
// Request payload
{ "url": "https://example.com", "active": true, "focus": true,
  "deduplicate": true, "windowId": 1 }

// Response payload
{ "tabId": 12346, "windowId": 1, "reused": false }
```

## tabs.close / tabs.activate / tabs.update (→ extension)

```json
// tabs.close（单个）
{ "payload": { "tabId": 12345 }}  // Response: {}
// tabs.close（批量）— TUI 多选关闭时逐个发 tabs.close 请求，不使用数组形式

// tabs.activate
{ "payload": { "tabId": 12345, "focus": true }}  // Response: {}

// tabs.update
{ "payload": { "tabId": 12345, "pinned": true }}  // Response: {}
```

## groups.list / create / update / delete (→ extension)

```json
// groups.list response
{ "groups": [{ "id": 1, "title": "Work", "color": "blue", "collapsed": false, "windowId": 1 }]}

// groups.create
{ "payload": { "tabIds": [12345, 12346], "title": "Work", "color": "blue" }}
// Response: { "groupId": 2 }

// groups.update
{ "payload": { "groupId": 2, "title": "Updated", "color": "red", "collapsed": true }}
// Response: {}

// groups.delete
{ "payload": { "groupId": 2 }}  // Response: {}
```

## sessions.list (daemon local)

**返回 SUMMARY，不含 tabs**。要获取完整数据用 sessions.get。

```json
// Response payload
{ "sessions": [{
    "name": "work-afternoon", "tabCount": 15, "windowCount": 2, "groupCount": 3,
    "createdAt": "2026-03-07T10:00:00Z", "sourceTarget": "target_1"
}]}
```

## sessions.get (daemon local)

```json
// Request: { "payload": { "name": "work-afternoon" }}
// Response payload — 完整数据
{ "session": {
    "name": "work-afternoon", "createdAt": "2026-03-07T10:00:00Z",
    "sourceTarget": "target_1",
    "windows": [{ "tabs": [
        { "url": "https://...", "title": "...", "pinned": false, "active": true, "groupIndex": 0 }
    ]}],
    "groups": [{ "title": "Work", "color": "blue", "collapsed": false }]
}}
```

## sessions.save (daemon + extension)

```json
// Request: { "target": {...}, "payload": { "name": "work-afternoon" }}
// Response: { "name": "work-afternoon", "tabCount": 15, "windowCount": 2, "groupCount": 3 }
```

## sessions.restore (daemon + extension)

```json
// Response: { "windowsCreated": 2, "tabsOpened": 15, "tabsFailed": 0, "groupsCreated": 3 }
```

**恢复规则** (TS v9 最终逻辑)：
1. 按 `windows[]` 逐个创建窗口
2. 追加后续 tabs
3. **所有 tabs 就位后**按 `groups[]` 创建 groups
4. tab 失败 → 跳过，不阻塞
5. group 失败 → warn，不阻塞

## sessions.delete (daemon local)

```json
// Request: { "payload": { "name": "work-afternoon" }}
// Response: {}
```

## collections.list (daemon local)

**返回 SUMMARY (itemCount)**，不返回 items[]。用 collections.get 获取完整数据。

```json
{ "collections": [{
    "name": "favorites", "itemCount": 8,
    "createdAt": "2026-03-07T10:00:00Z", "updatedAt": "2026-03-07T12:00:00Z"
}]}
```

## collections.get (daemon local)

```json
// Request: { "payload": { "name": "favorites" }}
// Response payload — 完整数据
{ "collection": {
    "name": "favorites", "createdAt": "...", "updatedAt": "...",
    "items": [{ "url": "https://...", "title": "Example", "groupLabel": "Work" }]
}}
```

## collections.create (daemon local)

```json
// Request: { "payload": { "name": "favorites" }}
// Response: { "name": "favorites", "createdAt": "..." }
```

## collections.delete (daemon local)

```json
// Request: { "payload": { "name": "favorites" }}
// Response: {}
```

## collections.addItems (daemon local)

```json
// Request: { "payload": { "name": "favorites", "items": [
//   { "url": "https://...", "title": "Example", "groupLabel": "Work" }
// ]}}
// Response: { "name": "favorites", "itemCount": 9 }
```

## collections.removeItems (daemon local)

```json
// Request: { "payload": { "name": "favorites", "urls": ["https://..."] }}
// Response: { "name": "favorites", "itemCount": 7 }
```

## collections.addFromTabs (daemon + extension)

```json
// Request: { "target": {...}, "payload": { "name": "favorites", "tabIds": [12345, 12346] }}
// Response: { "name": "favorites", "itemCount": 10, "added": 2 }
```

## collections.restore (daemon + extension)

```json
// Request: { "target": {...}, "payload": { "name": "favorites" }}
// Response: { "tabsOpened": 8, "tabsFailed": 0 }
```

## subscribe

```json
// Request: { "payload": { "patterns": ["tabs.*", "groups.*"] }}
// Response: { "subscribed": true }
```

Event 推送格式：
```json
{ "type": "event", "action": "tabs.created",
  "payload": { "tab": {...}, "_target": { "targetId": "target_1" }}}
```

**重要**：每个 event payload 必须含 `_target.targetId`。

## daemon.stop

```json
// Response (daemon 发完即关闭): { "stopping": true }
```

---

## Phase 7+: Bookmarks

### bookmarks.tree (→ extension)
```json
// Response: { "tree": [{ "id": "0", "title": "", "children": [...] }]}
```
`children` 存在 = 文件夹，`url` 存在 = 书签，两者互斥。

### bookmarks.search (→ extension)
```json
// Request: { "payload": { "query": "github" }}
// Response: { "bookmarks": [{ "id": "42", "title": "GitHub", "url": "...", "parentId": "1", "dateAdded": ... }]}
```

### bookmarks.get (→ extension)
```json
// Response: { "bookmark": { "id": "42", "title": "GitHub", "url": "...", ... }}
```

### bookmarks.mirror (daemon)
```json
// Response: { "nodeCount": 1523, "folderCount": 87, "mirroredAt": "..." }
```

### bookmarks.overlay.set / overlay.get (daemon local)
```json
// overlay.set request
{ "payload": { "bookmarkId": "42", "tags": ["work"], "note": "...", "alias": "gh" }}
// Response: same fields

// overlay.get request
{ "payload": { "bookmarkId": "42" }}
// Response: same fields
```

### bookmarks.export (daemon local)
```json
// Request: { "payload": { "folderId": "1", "format": "markdown" }}
// Response: { "content": "# Bookmarks Bar\n\n- [GitHub](https://...)..." }
```

### Bookmark Events
```json
// bookmarks.created
{ "action": "bookmarks.created", "payload": { "bookmark": {...}, "_target": {...} }}

// bookmarks.changed
{ "action": "bookmarks.changed", "payload": { "id": "42", "changes": { "title": "..." }, "_target": {...} }}

// bookmarks.removed
{ "action": "bookmarks.removed", "payload": { "id": "42", "removeInfo": {...}, "_target": {...} }}
```

---

## Stage 5: Search

### search.query (daemon local)
```json
// Request
{ "payload": { "query": "github", "mode": "global",
    "scopes": ["tabs", "sessions", "collections", "bookmarks", "workspaces"],
    "tags": [], "host": "", "limit": 50 }}

// Response
{ "results": [
    { "kind": "tab", "id": "12345", "title": "GitHub", "url": "...", "matchField": "title", "score": 1.0 },
    { "kind": "bookmark", "id": "42", "title": "GitHub", "url": "...", "matchField": "url", "score": 0.9 }
], "total": 3 }
```

`scopes` 空 = 搜索所有。结果按 `score` 降序。

### search.saved.list / create / delete (daemon local)
```json
// list response
{ "searches": [{ "id": "ss_uuid_1", "name": "work-repos", "query": {...}, "createdAt": "...", "updatedAt": "..." }]}

// create
{ "payload": { "name": "work-repos", "query": { "query": "github", "tags": ["work"] } }}
// Response: { "id": "ss_uuid_1", "name": "work-repos" }

// delete: { "payload": { "id": "ss_uuid_1" }} → {}
```

---

## Stage 6+: Workspace & Stage 4+: Sync

### workspace.list / get / create / update / delete (daemon local)

```json
// workspace.list response
{ "workspaces": [{
    "id": "ws_uuid_1", "name": "frontend-project",
    "sessionCount": 2, "collectionCount": 3, "bookmarkFolderIds": ["43"],
    "createdAt": "...", "updatedAt": "..."
}]}

// workspace.get response
{ "workspace": {
    "id": "ws_uuid_1", "name": "frontend-project",
    "description": "React migration project workspace",
    "sessions": ["morning-tabs", "afternoon-tabs"],
    "collections": ["ui-references", "api-docs"],
    "bookmarkFolderIds": ["43"],
    "savedSearchIds": ["ss_uuid_1"],
    "tags": ["frontend", "active"],
    "notes": "React migration project",
    "status": "active",
    "defaultTarget": "target_1",
    "lastActiveAt": "2026-03-07T11:30:00Z",
    "createdAt": "...", "updatedAt": "..."
}}

// workspace.create: { "payload": { "name": "..." }} → { "id": "ws_uuid_1", "name": "..." }
// workspace.update: { "payload": { "id": "...", "sessions": [...], "tags": [...] }} → {}
// workspace.delete: { "payload": { "id": "..." }} → {}
```

### workspace.switch (daemon + extension)
```json
// Response: { "tabsClosed": 24, "windowsCreated": 2, "tabsOpened": 18, "tabsFailed": 0 }
```

### sync.status / sync.repair (daemon local)
```json
// sync.status response
{ "enabled": true, "syncDir": "~/Library/Mobile Documents/com~ctm/",
  "lastSync": "...", "pendingChanges": 0, "conflicts": [] }

// sync.repair response
{ "reindexed": true, "objectCount": 42, "conflictsResolved": 1 }
```
