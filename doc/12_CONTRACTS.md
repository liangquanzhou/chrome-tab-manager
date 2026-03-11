# CTM — Protocol & API Contracts (Authoritative)

> **This document is the single source of truth for every daemon action's wire format.**
> TUI/CLI/extension developers MUST implement exactly what is specified here. Do not guess.

## Conventions

- **Selector**: The only currently implemented target selector is `targetId`. The `TargetSelector` struct in protocol also declares `channel` and `label` fields, but neither the daemon's `resolveTarget()` nor the CLI implements them — they are reserved for future use. Resolution order: explicit `targetId` → default target → single-target fallback → ambiguous error.
- **`--json` output**: Emits `payload`-only JSON (no envelope with `id`, `type`, etc.).
- **Exit codes**: `0` = success, `1` = usage/validation (cobra), `2` = daemon unavailable / installation invalid, `3` = target resolution error, `4` = action execution error / timeout / chrome API failure.
- **Error codes**: `DAEMON_UNAVAILABLE`, `TARGET_OFFLINE`, `TARGET_AMBIGUOUS`, `EXTENSION_NOT_CONNECTED`, `CHROME_API_ERROR`, `TIMEOUT`, `UNKNOWN_ACTION`, `INVALID_PAYLOAD`, `INSTALLATION_INVALID`, `PROTOCOL_MISMATCH`.

## General Envelope Rules

- All messages contain `id` (UUID v4), `protocol_version` (currently `1`), `type` (`hello` | `request` | `response` | `error` | `event`).
- Request messages include `action` (string) and optionally `target` (TargetSelector) and `payload` (object).
- Response `id` MUST match the request `id`.
- Error messages carry `error.code` (ErrorCode) and `error.message` (string).
- Omitted fields mean "not applicable"; never use `null`.
- All persistent objects carry `createdAt` and `updatedAt` (RFC 3339).

## Layer Definitions

- **Forward**: daemon transparently forwards request to extension via NM, returns extension's response.
- **Local**: daemon handles entirely from local state/files; no extension involvement.
- **Hybrid**: daemon orchestrates multi-step operations involving both local state and extension calls.

---

## hello

- Layer: local
- Target: disallowed
- CLI: internal only
- Request:
  - channel: string, required, browser release channel (e.g. "stable")
  - extensionId: string, required, Chrome extension ID
  - instanceId: string, required, UUID v4 unique to this connection
  - userAgent: string, required, browser user agent string
  - capabilities: string[], required, supported API domains (e.g. ["tabs","groups","bookmarks","history","downloads","windows","events"])
  - min_supported: int, required, minimum protocol version the extension supports
- Response:
  - targetId: string, assigned target identifier (e.g. "target_1")
- Errors:
  - INVALID_PAYLOAD: malformed hello payload
- Partial Failure: N/A
- Idempotency: not idempotent; each hello allocates a new targetId
- Notes: Sent by extension on NM port connect. First registered target is auto-set as default.

---

## daemon.stop

- Layer: local
- Target: disallowed
- CLI: internal only
- Request:
  - (no payload fields)
- Response:
  - stopping: bool, always `true`
- Errors: none
- Partial Failure: N/A
- Idempotency: idempotent (subsequent calls are no-ops since daemon is already shutting down)
- Notes: Daemon sends the response, then immediately stops its event loop.

---

## subscribe

- Layer: local
- Target: disallowed
- CLI: internal only
- Request:
  - patterns: string[], required, glob patterns for event actions to subscribe to (e.g. ["tabs.*", "groups.*", "*"])
- Response:
  - subscribed: bool, always `true`
- Errors:
  - INVALID_PAYLOAD: malformed subscribe payload
- Partial Failure: N/A
- Idempotency: not idempotent; each call adds a new subscription (duplicates accumulate)
- Notes: Events are pushed as `type: "event"` messages. Every event payload includes `_target.targetId`. Pattern `*` matches all actions; `domain.*` matches all actions in that domain.

---

## targets.list

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - targets: TargetInfo[], list of connected targets
    - targetId: string
    - channel: string
    - label: string
    - isDefault: bool
    - userAgent: string
    - connectedAt: int (Unix milliseconds)
- Errors: none
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: Returns a snapshot of currently connected extension targets.

---

## targets.default

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - targetId: string, required, target to set as default
- Response:
  - targetId: string, the target that was set as default
- Errors:
  - INVALID_PAYLOAD: malformed payload
  - TARGET_OFFLINE: specified targetId not found among connected targets
- Partial Failure: N/A
- Idempotency: idempotent (setting the same default twice is a no-op)
- Notes: N/A

---

## targets.clearDefault

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - (empty object `{}`)
- Errors: none
- Partial Failure: N/A
- Idempotency: idempotent
- Notes: After clearing, target resolution falls back to "only one connected" or fails with TARGET_AMBIGUOUS.

---

## targets.label

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - targetId: string, required
  - label: string, required, human-readable label
- Response:
  - targetId: string
  - label: string
- Errors:
  - INVALID_PAYLOAD: malformed payload
  - TARGET_OFFLINE: specified targetId not found
- Partial Failure: N/A
- Idempotency: idempotent (setting the same label is a no-op)
- Notes: Labels are in-memory only; lost on daemon restart.

---

## tabs.list

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - tabs: Tab[], all tabs across all windows
    - id: int, Chrome tab ID
    - windowId: int
    - index: int, position within window
    - title: string
    - url: string
    - active: bool
    - pinned: bool
    - muted: bool
    - groupId: int (-1 = not in any group)
    - favIconUrl: string
- Errors:
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED: no target available
  - TIMEOUT: extension did not respond within 10s
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: `groupId: -1` means the tab is not in any group.

---

## tabs.open

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - url: string, required, URL to open
  - active: bool, optional (default true), make the tab active
  - focus: bool, optional (default false), focus the browser window
  - deduplicate: bool, optional (default false), reuse existing tab with same URL
  - windowId: int, optional, target window ID
- Response:
  - tabId: int, Chrome tab ID of the opened/reused tab
  - windowId: int
  - reused: bool, true if an existing tab was activated instead of creating new
- Errors:
  - INVALID_PAYLOAD: url is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: Chrome API failure
- Partial Failure: N/A
- Idempotency: not idempotent without `deduplicate: true`; with dedup, idempotent for the same URL
- Notes: When `deduplicate: true`, searches for existing tab with matching URL before creating.

---

## tabs.close

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - tabId: int, required, Chrome tab ID to close
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: tabId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: tab not found or cannot be closed
- Partial Failure: N/A
- Idempotency: idempotent (closing an already-closed tab returns CHROME_API_ERROR)
- Notes: TUI multi-select close sends individual `tabs.close` per tab, not batch.

---

## tabs.activate

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - tabId: int, required
  - focus: bool, optional (default false), also focus the browser window
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: tabId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: tab not found
- Partial Failure: N/A
- Idempotency: idempotent
- Notes: N/A

---

## tabs.update

- Layer: forward
- Target: required
- CLI: supported (via `tabs pin`, which uses `tabs.pin` action instead)
- Request:
  - tabId: int, required
  - pinned: bool, optional, new pinned state
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: tabId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: idempotent
- Notes: Currently only supports `pinned` property. Additional properties can be added later.

---

## tabs.mute

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - tabId: int, required
  - muted: bool, optional, explicit mute state; if omitted, toggles current state
- Response:
  - tabId: int
  - muted: bool, resulting mute state
- Errors:
  - INVALID_PAYLOAD: tabId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: tab not found
- Partial Failure: N/A
- Idempotency: idempotent when `muted` is explicitly set; toggle mode is not idempotent
- Notes: N/A

---

## tabs.pin

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - tabId: int, required
  - pinned: bool, optional, explicit pin state; if omitted, toggles current state
- Response:
  - tabId: int
  - pinned: bool, resulting pin state
- Errors:
  - INVALID_PAYLOAD: tabId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: tab not found
- Partial Failure: N/A
- Idempotency: idempotent when `pinned` is explicitly set; toggle mode is not idempotent
- Notes: N/A

---

## tabs.move

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - tabId: int, required
  - windowId: int, optional, target window ID
  - index: int, optional (default -1), target position (-1 = end)
- Response:
  - tabId: int
  - windowId: int, actual window after move
  - index: int, actual position after move
- Errors:
  - INVALID_PAYLOAD: tabId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: tab or window not found
- Partial Failure: N/A
- Idempotency: idempotent (moving to the same position is a no-op)
- Notes: N/A

---

## tabs.getText

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - tabId: int, required
- Response:
  - text: string, visible text content of the page (max 8KB)
- Errors:
  - INVALID_PAYLOAD: tabId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: cannot inject content script (e.g. chrome:// pages)
- Partial Failure: returns error message in `text` field prefixed with "(cannot access page: ...)" when content script injection fails
- Idempotency: read-only, safe
- Notes: Uses `chrome.scripting.executeScript` to extract `innerText` from page body. Falls back to `fetch` + HTML stripping when script injection fails. Output truncated to 8000 characters.

---

## tabs.capture

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - tabId: int, optional, tab to capture (activates it first if provided)
  - windowId: int, optional, window to capture (current window if omitted)
  - format: string, optional (default "png"), "png" or "jpeg"
  - quality: int, optional, JPEG quality 0-100
- Response:
  - dataUrl: string, base64-encoded data URL of the screenshot
- Errors:
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: `captureVisibleTab` failed and debugger fallback also failed
- Partial Failure: N/A
- Idempotency: read-only (but may activate a tab as side effect when `tabId` is provided)
- Notes: Primary method is `chrome.debugger` CDP `Page.captureScreenshot` (does not steal window focus). Falls back to `chrome.tabs.captureVisibleTab` if debugger attach fails. Fallback activates the tab and waits 150ms before capture. TUI `s` key captures and opens in external viewer.

---

## windows.list

- Layer: forward
- Target: required
- CLI: internal only
- Request:
  - (no payload fields)
- Response:
  - windows: Window[]
    - id: int
    - focused: bool
    - state: string ("normal", "minimized", "maximized", "fullscreen")
    - type: string ("normal", "popup", "panel", "app", "devtools")
    - width: int
    - height: int
    - top: int
    - left: int
- Errors:
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: Does not populate tab lists (`populate: false`).

---

## windows.create

- Layer: forward
- Target: required
- CLI: internal only
- Request:
  - url: string, optional, URL to open in the new window
  - focused: bool, optional
  - state: string, optional ("normal", "minimized", "maximized", "fullscreen")
- Response:
  - windowId: int
  - state: string
- Errors:
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR
- Partial Failure: N/A
- Idempotency: not idempotent (each call creates a new window)
- Notes: N/A

---

## windows.close

- Layer: forward
- Target: required
- CLI: internal only
- Request:
  - windowId: int, required
- Response:
  - closed: bool, always `true`
- Errors:
  - INVALID_PAYLOAD: windowId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: window not found
- Partial Failure: N/A
- Idempotency: idempotent (closing already-closed window returns CHROME_API_ERROR)
- Notes: N/A

---

## windows.focus

- Layer: forward
- Target: required
- CLI: internal only
- Request:
  - (no payload fields)
- Response:
  - focused: bool, always `true`
- Errors:
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: idempotent
- Notes: Focuses the last-focused Chrome window (brings browser to front). Used internally by `sessions.restore`, `collections.restore`, and `workspace.switch` after opening tabs.

---

## groups.list

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - groups: Group[]
    - id: int, Chrome group ID
    - title: string
    - color: string (grey, blue, red, yellow, green, pink, purple, cyan, orange)
    - collapsed: bool
    - windowId: int
- Errors:
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: N/A

---

## groups.create

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - tabIds: int[], required, tab IDs to include in the group
  - title: string, optional, group title
  - color: string, optional, group color
- Response:
  - groupId: int, Chrome group ID
- Errors:
  - INVALID_PAYLOAD: tabIds is empty or missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: one or more tabs not found
- Partial Failure: N/A (all-or-nothing at Chrome API level)
- Idempotency: not idempotent (creates a new group each time)
- Notes: N/A

---

## groups.update

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - groupId: int, required
  - title: string, optional
  - color: string, optional
  - collapsed: bool, optional
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: groupId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: group not found
- Partial Failure: N/A
- Idempotency: idempotent
- Notes: N/A

---

## groups.delete

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - groupId: int, required
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: groupId is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: group not found
- Partial Failure: N/A
- Idempotency: idempotent
- Notes: Implementation ungroups all tabs in the group (Chrome auto-deletes empty groups).

---

## sessions.list

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - sessions: SessionSummary[], **summary only** (no `tabs[]`)
    - name: string
    - tabCount: int
    - windowCount: int
    - groupCount: int
    - createdAt: string (RFC 3339)
    - sourceTarget: string
- Errors:
  - INVALID_PAYLOAD: cannot read sessions directory
- Partial Failure: corrupt session files are silently skipped
- Idempotency: read-only, safe
- Notes: Use `sessions.get` for full data including tabs.

---

## sessions.get

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - name: string, required, session name (validated: `^[a-zA-Z0-9_-]+$`, max 128 chars)
- Response:
  - session: Session (full data)
    - name: string
    - createdAt: string (RFC 3339)
    - sourceTarget: string
    - windows: SessionWindow[]
      - tabs: SessionTab[]
        - url: string
        - title: string
        - pinned: bool
        - active: bool
        - groupIndex: int (-1 = not in any group)
    - groups: SessionGroup[]
      - title: string
      - color: string
      - collapsed: bool
- Errors:
  - INVALID_PAYLOAD: name empty, invalid characters, or session not found
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: `groupIndex` maps to the index in the `groups[]` array.

---

## sessions.save

- Layer: hybrid
- Target: required
- CLI: supported
- Request:
  - name: string, required, session name (validated: `^[a-zA-Z0-9_-]+$`, max 128 chars)
- Response:
  - name: string
  - tabCount: int
  - windowCount: int
  - groupCount: int
  - createdAt: string (RFC 3339)
  - sourceTarget: string
- Errors:
  - INVALID_PAYLOAD: name empty or invalid characters
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED: no target for tab/group queries
  - TIMEOUT: extension did not respond to tabs.list or groups.list
- Partial Failure: N/A (if either tabs.list or groups.list fails, the entire save fails)
- Idempotency: not idempotent (overwrites any existing session with the same name)
- Notes: Queries `tabs.list` and `groups.list` from extension, builds session file, writes atomically (tmp -> fsync -> rename). Updates search index.

---

## sessions.restore

- Layer: hybrid
- Target: required
- CLI: supported
- Request:
  - name: string, required
- Response:
  - windowsCreated: int
  - tabsOpened: int
  - tabsFailed: int
  - groupsCreated: int
- Errors:
  - INVALID_PAYLOAD: name invalid or session not found
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
- Partial Failure: tab failures are counted in `tabsFailed` but do not block other tabs or groups. Group failures are silently skipped.
- Idempotency: not idempotent (each call opens new tabs/windows)
- Notes: Restore order: windows -> tabs (throttled: batches of 5, 200ms delay) -> groups. Routed through heavy job queue to serialize expensive operations. Focuses browser window after restore.

---

## sessions.delete

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - name: string, required
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: name invalid or delete failed (file not found)
- Partial Failure: N/A
- Idempotency: idempotent conceptually (deleting non-existent session returns error)
- Notes: Removes from search index.

---

## collections.list

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - collections: CollectionSummary[], **summary only** (no `items[]`)
    - name: string
    - itemCount: int
    - createdAt: string (RFC 3339)
    - updatedAt: string (RFC 3339)
- Errors:
  - INVALID_PAYLOAD: cannot read collections directory
- Partial Failure: corrupt collection files are silently skipped
- Idempotency: read-only, safe
- Notes: Use `collections.get` for full data including items.

---

## collections.get

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - name: string, required (validated: `^[a-zA-Z0-9_-]+$`, max 128 chars)
- Response:
  - collection: Collection (full data)
    - name: string
    - createdAt: string (RFC 3339)
    - updatedAt: string (RFC 3339)
    - items: CollectionItem[]
      - url: string
      - title: string
      - groupLabel: string (optional)
- Errors:
  - INVALID_PAYLOAD: name invalid or collection not found
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: N/A

---

## collections.create

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - name: string, required (validated: `^[a-zA-Z0-9_-]+$`, max 128 chars)
- Response:
  - name: string
  - createdAt: string (RFC 3339)
- Errors:
  - INVALID_PAYLOAD: name empty, invalid characters, or write failed
- Partial Failure: N/A
- Idempotency: not idempotent (overwrites existing collection with same name)
- Notes: Creates empty collection. Writes atomically. Updates search index.

---

## collections.delete

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - name: string, required
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: name invalid or delete failed
- Partial Failure: N/A
- Idempotency: idempotent conceptually (deleting non-existent returns error)
- Notes: Removes from search index.

---

## collections.addItems

- Layer: local
- Target: disallowed
- CLI: supported (via `collections add --url --title`)
- Request:
  - name: string, required
  - items: CollectionItem[], required
    - url: string
    - title: string
    - groupLabel: string (optional)
- Response:
  - name: string
  - itemCount: int, total items after addition
- Errors:
  - INVALID_PAYLOAD: name invalid, collection not found, or write failed
- Partial Failure: N/A (all items added atomically)
- Idempotency: not idempotent (duplicate items are appended)
- Notes: Appends items to existing collection. Updates `updatedAt`. Writes atomically. Updates search index.

---

## collections.removeItems

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - name: string, required
  - urls: string[], required, URLs to remove
- Response:
  - name: string
  - itemCount: int, total items after removal
- Errors:
  - INVALID_PAYLOAD: name invalid, collection not found, or write failed
- Partial Failure: N/A (URLs not found in collection are silently ignored)
- Idempotency: idempotent (removing already-removed URLs is a no-op)
- Notes: Removes all items whose URL matches any in the `urls` list. Updates search index.

---

## collections.rename

- Layer: local
- Target: disallowed
- CLI: internal
- Request:
  - name: string, required, current collection name
  - newName: string, required, new collection name
- Response:
  - name: string, the new name
- Errors:
  - INVALID_PAYLOAD: name invalid, collection not found, new name already exists, or write failed
- Idempotency: not idempotent (newName must differ from name; same-name errors with "already exists")
- Notes: Renames the JSON file on disk. TUI guards against same-name input. Updates search index.

---

## collections.reorder

- Layer: local
- Target: disallowed
- CLI: internal
- Request:
  - name: string, required
  - fromIndex: int, required, source position (0-based)
  - toIndex: int, required, destination position (0-based)
- Response:
  - name: string
  - itemCount: int
- Errors:
  - INVALID_PAYLOAD: name invalid, collection not found, index out of range, or write failed
- Idempotency: not idempotent (repeated calls keep moving)
- Notes: Moves a single item within the collection. Updates search index.

---

## collections.restore

- Layer: hybrid
- Target: required
- CLI: supported
- Request:
  - name: string, required
- Response:
  - tabsOpened: int
  - tabsFailed: int
- Errors:
  - INVALID_PAYLOAD: name invalid or collection not found
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
- Partial Failure: tab failures counted in `tabsFailed`, do not block other tabs
- Idempotency: not idempotent (each call opens new tabs)
- Notes: Opens each collection item as an inactive tab via `tabs.open`. Focuses browser window after restore.

---

## bookmarks.tree

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - tree: BookmarkNode[], full Chrome bookmarks tree
    - id: string
    - title: string
    - url: string (present = bookmark)
    - children: BookmarkNode[] (present = folder)
    - parentId: string
    - dateAdded: int (timestamp)
- Errors:
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: `children` present = folder, `url` present = bookmark, mutually exclusive. Daemon also updates local mirror in background after forwarding response.

---

## bookmarks.search

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - query: string, required, search text
- Response:
  - bookmarks: Bookmark[]
    - id: string
    - title: string
    - url: string
    - parentId: string
    - dateAdded: int
- Errors:
  - INVALID_PAYLOAD: query is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: N/A

---

## bookmarks.get

- Layer: forward
- Target: required
- CLI: internal only
- Request:
  - id: string, required, Chrome bookmark ID
- Response:
  - bookmark: Bookmark
    - id: string
    - title: string
    - url: string
    - parentId: string
    - dateAdded: int
- Errors:
  - INVALID_PAYLOAD: id is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: bookmark not found
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: N/A

---

## bookmarks.create

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - parentId: string, optional, parent folder ID (defaults to "Other Bookmarks" if omitted)
  - title: string, optional
  - url: string, optional (omit for folder)
  - index: int, optional, position within parent
- Response:
  - bookmark: Bookmark
    - id: string
    - title: string
    - url: string
    - parentId: string
    - dateAdded: int
- Errors:
  - INVALID_PAYLOAD: invalid parameters
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: parent folder not found
- Partial Failure: N/A
- Idempotency: not idempotent (creates new bookmark each time)
- Notes: Omitting `url` creates a folder.

---

## bookmarks.update

- Layer: forward
- Target: required
- CLI: internal only
- Request:
  - id: string, required, Chrome bookmark ID
  - title: string, optional
  - url: string, optional
- Response:
  - bookmark: Bookmark
    - id: string
    - title: string
    - url: string
    - parentId: string
    - dateAdded: int
- Errors:
  - INVALID_PAYLOAD: id is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: bookmark not found
- Partial Failure: N/A
- Idempotency: idempotent
- Notes: N/A

---

## bookmarks.remove

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - id: string, required, Chrome bookmark or folder ID
- Response:
  - removed: bool, always `true`
- Errors:
  - INVALID_PAYLOAD: id is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: bookmark not found or is a root node
- Partial Failure: N/A
- Idempotency: idempotent (removing already-removed returns CHROME_API_ERROR)
- Notes: Uses `chrome.bookmarks.removeTree` first (handles non-empty folders), falls back to `chrome.bookmarks.remove`.

---

## bookmarks.move

- Layer: forward
- Target: required
- CLI: internal only
- Request:
  - id: string, required, Chrome bookmark ID
  - parentId: string, optional, new parent folder ID
  - index: int, optional, position within new parent
- Response:
  - bookmark: Bookmark
    - id: string
    - title: string
    - url: string
    - parentId: string
- Errors:
  - INVALID_PAYLOAD: id is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: bookmark or target folder not found
- Partial Failure: N/A
- Idempotency: idempotent (moving to same location is a no-op)
- Notes: N/A

---

## bookmarks.mirror

- Layer: hybrid
- Target: optional
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - nodeCount: int, total bookmark nodes
  - folderCount: int, total folders
  - mirroredAt: string (RFC 3339)
  - targetId: string, source target
- Errors:
  - INVALID_PAYLOAD: failed to parse bookmark tree
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED: no target available and no cached mirror
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: idempotent (re-running produces fresh mirror)
- Notes: Returns cached mirror if available (checks per-target `mirror_<targetId>.json` first, then `mirror.json`). If no cache, fetches `bookmarks.tree` from extension and saves. Routed through heavy job queue when fetching from extension. Writes both per-target and default mirror files.

---

## bookmarks.overlay.set

- Layer: local
- Target: disallowed
- CLI: internal only
- Request:
  - bookmarkId: string, required (path-safe validated)
  - tags: string[], required
  - note: string, required
  - alias: string, required
- Response:
  - bookmarkId: string
  - tags: string[]
  - note: string
  - alias: string
- Errors:
  - INVALID_PAYLOAD: bookmarkId invalid or write failed
- Partial Failure: N/A
- Idempotency: idempotent (overwrites existing overlay)
- Notes: CTM-local metadata overlay for Chrome bookmarks. Stored as `<bookmarkId>.json` in overlays directory.

---

## bookmarks.overlay.get

- Layer: local
- Target: disallowed
- CLI: internal only
- Request:
  - bookmarkId: string, required (path-safe validated)
- Response:
  - bookmarkId: string
  - tags: string[]
  - note: string
  - alias: string
- Errors:
  - INVALID_PAYLOAD: bookmarkId invalid
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: Returns empty overlay (empty tags, empty note/alias) if no overlay file exists.

---

## bookmarks.export

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - folderId: string, optional, export subtree under this folder (entire tree if omitted)
  - format: string, optional (default "markdown"), export format
  - targetId: string, optional, prefer mirror from this target
- Response:
  - content: string, exported content (Markdown format)
- Errors:
  - INVALID_PAYLOAD: no bookmark mirror available, or folderId not found in mirror
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: Reads from local mirror file. Requires prior `bookmarks.mirror` to populate cache.

---

## history.search

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - query: string, required, search text (empty string allowed for recent history)
  - maxResults: int, optional (default 100)
  - startTime: int, optional, start time in ms since epoch
  - endTime: int, optional, end time in ms since epoch
- Response:
  - history: HistoryItem[]
    - id: string
    - url: string
    - title: string
    - lastVisitTime: float64 (ms since epoch)
    - visitCount: int
    - typedCount: int
- Errors:
  - INVALID_PAYLOAD: query field missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: N/A

---

## history.delete

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - url: string, required, URL to delete from history
- Response:
  - deleted: bool, always `true`
- Errors:
  - INVALID_PAYLOAD: url is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: idempotent
- Notes: N/A

---

## downloads.list

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - query: string, optional, filename regex filter
  - limit: int, optional (default 50)
- Response:
  - downloads: DownloadItem[]
    - id: int
    - filename: string
    - url: string
    - state: string ("in_progress", "interrupted", "complete")
    - totalBytes: int
    - bytesReceived: int
    - startTime: string (ISO)
    - endTime: string (ISO, empty if in progress)
    - mime: string
    - danger: string (default "safe")
- Errors:
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: Results ordered by start time descending.

---

## downloads.cancel

- Layer: forward
- Target: required
- CLI: supported
- Request:
  - id: int, required, download ID (matches extension's expected field name)
- Response:
  - cancelled: bool, always `true`
- Errors:
  - INVALID_PAYLOAD: id is missing
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
  - TIMEOUT
  - CHROME_API_ERROR: download not found or already completed
- Partial Failure: N/A
- Idempotency: idempotent (cancelling already-cancelled returns CHROME_API_ERROR)
- Notes: N/A

---

## search.query

- Layer: hybrid
- Target: optional
- CLI: supported
- Request:
  - query: string, required, search text
  - mode: string, optional (default "global")
  - scopes: string[], optional, limit to specific scopes (tabs, sessions, collections, bookmarks, workspaces); empty = all
  - tags: string[], optional, filter by tags
  - host: string, optional, filter by URL host
  - limit: int, optional (default 50)
- Response:
  - results: SearchResult[], sorted by score descending
    - kind: string ("tab", "session", "collection", "bookmark", "workspace")
    - id: string
    - title: string
    - url: string (empty for sessions/workspaces)
    - matchField: string ("title", "url", "name", "tag")
    - score: float64, relevance score
  - total: int, number of results returned
- Errors:
  - INVALID_PAYLOAD: malformed search payload
- Partial Failure: scopes that fail (e.g. tabs scope when no target is connected) are silently excluded from results
- Idempotency: read-only, safe
- Notes: Searches across multiple backends: tabs (live from extension), sessions/collections/workspaces (from search index or file scanning), bookmarks (from local mirror). Target is only needed when `tabs` scope is included.

---

## search.saved.list

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - searches: SavedSearch[]
    - id: string (prefixed with "ss_")
    - name: string
    - query: SearchQuery
    - createdAt: string (RFC 3339)
    - updatedAt: string (RFC 3339)
- Errors:
  - INVALID_PAYLOAD: cannot read saved searches directory
- Partial Failure: corrupt files are silently skipped
- Idempotency: read-only, safe
- Notes: N/A

---

## search.saved.create

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - name: string, required
  - query: SearchQuery, required
- Response:
  - id: string, generated ID (prefixed with "ss_")
  - name: string
- Errors:
  - INVALID_PAYLOAD: name is empty or write failed
- Partial Failure: N/A
- Idempotency: not idempotent (each call creates a new saved search with unique ID)
- Notes: N/A

---

## search.saved.delete

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - id: string, required (must start with "ss_", path-safe validated)
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: id invalid or delete failed
- Partial Failure: N/A
- Idempotency: idempotent conceptually (deleting non-existent returns error)
- Notes: N/A

---

## workspace.list

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - workspaces: WorkspaceSummary[]
    - id: string (prefixed with "ws_")
    - name: string
    - sessionCount: int
    - collectionCount: int
    - createdAt: string (RFC 3339)
    - updatedAt: string (RFC 3339)
- Errors:
  - INVALID_PAYLOAD: cannot read workspaces directory
- Partial Failure: corrupt files are silently skipped
- Idempotency: read-only, safe
- Notes: N/A

---

## workspace.get

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - id: string, required (validated: `^[a-zA-Z0-9_-]+$`, max 128 chars)
- Response:
  - workspace: Workspace (full data)
    - id: string
    - name: string
    - description: string
    - sessions: string[] (session names)
    - collections: string[] (collection names)
    - bookmarkFolderIds: string[]
    - savedSearchIds: string[]
    - tags: string[]
    - notes: string
    - status: string ("active", "archived")
    - defaultTarget: string
    - lastActiveAt: string (RFC 3339)
    - createdAt: string (RFC 3339)
    - updatedAt: string (RFC 3339)
- Errors:
  - INVALID_PAYLOAD: id invalid or workspace not found
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: N/A

---

## workspace.create

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - name: string, required
  - description: string, optional
  - sessions: string[], optional
  - collections: string[], optional
  - tags: string[], optional
- Response:
  - id: string, generated workspace ID
  - name: string
- Errors:
  - INVALID_PAYLOAD: name is empty or write failed
- Partial Failure: N/A
- Idempotency: not idempotent (each call creates a new workspace with unique ID)
- Notes: Status defaults to "active". Empty arrays are stored as `[]` not `null`. Updates search index.

---

## workspace.update

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - id: string, required
  - name: string, optional (pointer semantics: omit to keep, set to change)
  - description: string, optional
  - sessions: string[], optional
  - collections: string[], optional
  - bookmarkFolderIds: string[], optional
  - savedSearchIds: string[], optional
  - tags: string[], optional
  - notes: string, optional
  - status: string, optional
  - defaultTarget: string, optional
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: id invalid or workspace not found or write failed
- Partial Failure: N/A
- Idempotency: idempotent
- Notes: Only provided fields are updated; omitted fields are unchanged. Updates `updatedAt`. Updates search index.

---

## workspace.delete

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - id: string, required
- Response:
  - (empty object `{}`)
- Errors:
  - INVALID_PAYLOAD: id invalid or delete failed
- Partial Failure: N/A
- Idempotency: idempotent conceptually (deleting non-existent returns error)
- Notes: Removes from search index. Does NOT delete associated sessions or collections.

---

## workspace.switch

- Layer: hybrid
- Target: required
- CLI: supported
- Request:
  - id: string, required
- Response:
  - tabsClosed: int, tabs closed from previous state
  - windowsCreated: int
  - tabsOpened: int
  - tabsFailed: int
- Errors:
  - INVALID_PAYLOAD: id invalid or workspace not found
  - TARGET_OFFLINE / EXTENSION_NOT_CONNECTED
- Partial Failure: Best-effort semantics throughout:
  - `tabs.list` failure (extension disconnect): skips closing old tabs, continues with restore
  - Session file missing/corrupt: skips restore (`tabsOpened=0, tabsFailed=0`)
  - Individual tab open failures: counted in `tabsFailed`, do not block others
  - Group creation failures: silently skipped
  - `lastActiveAt` is always updated regardless of sub-step failures
- Idempotency: not idempotent (each call closes and reopens tabs)
- Notes: Steps: (1) close all current tabs, (2) restore first session in workspace (throttled: batches of 5, 200ms delay), (3) create groups. Routed through heavy job queue. Updates workspace `lastActiveAt` and `updatedAt`.

---

## sync.status

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - enabled: bool, whether cloud directory exists
  - syncDir: string, cloud directory path
  - lastSync: string (RFC 3339), last sync timestamp (empty if never synced)
  - pendingChanges: int, number of files needing sync
  - conflicts: string[], list of conflicted file paths
  - metaVersion: int, sync metadata version counter
  - deviceId: string, persistent device identifier
  - objectCount: int, number of tracked objects
- Errors:
  - INVALID_PAYLOAD: cannot stat or read cloud directory
- Partial Failure: N/A
- Idempotency: read-only, safe
- Notes: Uses checksum-based comparison when metadata available, falls back to modtime comparison.

---

## sync.repair

- Layer: local
- Target: disallowed
- CLI: supported
- Request:
  - (no payload fields)
- Response:
  - reindexed: bool, always `true`
  - objectCount: int, total objects after reindex
  - conflictsResolved: int, number of conflicts resolved (local wins)
- Errors:
  - INVALID_PAYLOAD: cannot access cloud or local directories
- Partial Failure: individual conflict resolution failures are silently skipped
- Idempotency: idempotent
- Notes: Conflict resolution strategy: last-write-wins (local copy overwrites cloud). Rebuilds sync metadata from scratch.

---

## Events (pushed to subscribers)

Events are delivered as `type: "event"` messages. Every event payload includes `_target.targetId`.

### Tab Events

- **tabs.created**: `{ tab: Tab, _target: {...} }`
- **tabs.removed**: `{ tabId: int, removeInfo: { windowId: int, isWindowClosing: bool }, _target: {...} }`
- **tabs.updated**: `{ tabId: int, changeInfo: {...}, tab: Tab, _target: {...} }`
- **tabs.activated**: `{ tabId: int, windowId: int, _target: {...} }`
- **tabs.moved**: `{ tabId: int, moveInfo: { windowId: int, fromIndex: int, toIndex: int }, _target: {...} }`

### Bookmark Events

- **bookmarks.created**: `{ id: string, bookmark: Bookmark, _target: {...} }`
- **bookmarks.changed**: `{ id: string, changes: { title?: string, url?: string }, _target: {...} }`
- **bookmarks.removed**: `{ id: string, removeInfo: { parentId: string, index: int }, _target: {...} }`
