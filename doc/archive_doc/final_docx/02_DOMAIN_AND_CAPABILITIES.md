# CTM — Domain and Capabilities

## 1. Domain Overview

CTM has three major object families:

1. `Runtime objects`
2. `Library objects`
3. `Sync objects`

This is the real domain backbone of the product.

---

## 2. Runtime Objects

Runtime objects represent live browser state.

### Core runtime objects

- `Target`
- `Window`
- `Tab`
- `Group`
- `BookmarkSource`

### Runtime meaning

- `Target` is the live browser context boundary
- `Window` organizes tabs at runtime
- `Tab` is the smallest live working unit
- `Group` is runtime organization
- `BookmarkSource` is the browser-native bookmark tree

Runtime objects are:

- live
- change frequently
- browser-owned

---

## 3. Library Objects

Library objects represent long-term CTM-owned assets.

### Core library objects

- `Session`
- `Collection`
- `BookmarkMirror`
- `BookmarkOverlay`
- `Workspace`
- `Tag`
- `Note`
- `SavedSearch`

### Library meaning

- `Session` = snapshot of browser state
- `Collection` = curated set of resources
- `BookmarkMirror` = searchable CTM mirror of native bookmarks
- `BookmarkOverlay` = CTM metadata layer on top of bookmarks
- `Workspace` = long-term task context

Library objects are:

- durable
- searchable
- organizable
- syncable

---

## 4. Sync Objects

Sync objects explain cross-device continuity.

### Core sync objects

- `SyncAccount`
- `SyncState`
- `Device`

### Sync meaning

- `SyncAccount` identifies a sync source or target
- `SyncState` explains whether a resource is synced, stale, conflicted, or failed
- `Device` gives cross-machine context

---

## 5. Capability Areas

CTM’s full capability map has seven top-level areas.

## 5.1 Runtime

Capabilities:

- target discovery
- target selection
- tabs control
- groups control
- windows awareness
- live browser events
- runtime filtering

## 5.2 Library

Capabilities:

- session save / preview / restore / delete
- collection create / edit / restore / delete
- partial restore
- capture and export
- duplicate detection

## 5.3 Bookmarks

Capabilities:

- bookmark tree browse
- bookmark search
- bookmark mirror
- bookmark overlay
- bookmark export

## 5.4 Search

Capabilities:

- cross-resource search
- saved searches
- smart collections
- actionable search results

## 5.5 Workspace

Capabilities:

- workspace create / update / delete
- attach resources
- startup / resume
- search within workspace
- templates

## 5.6 Sync

Capabilities:

- local-first persistence
- Google-backed bookmark source
- iCloud sync for CTM-owned data
- conflict handling
- sync status
- repair / rebuild

## 5.7 Power

Capabilities:

- batch actions
- pin/unpin in bulk
- sorting
- dedupe
- suspend/discard
- export
- diagnostics
- automation hooks

---

## 6. Source-of-Truth Matrix

| Resource | Primary truth |
|----------|---------------|
| Tabs / Groups / Windows | Browser runtime |
| Native bookmarks | Chrome / Google Sync |
| Sessions | CTM |
| Collections | CTM |
| Bookmark mirror | CTM local mirror |
| Bookmark overlay | CTM |
| Workspaces | CTM |
| Tags / Notes / Saved searches | CTM |
| Sync status | CTM sync layer |

This boundary is critical.

---

## 7. Cross-Resource Relationships

The value of CTM comes from transitions between resources.

### Important transitions

- `Tab -> Session`
- `Tab -> Collection`
- `Bookmark -> Collection`
- `Bookmark -> Workspace`
- `Session -> Workspace`
- `Collection -> Workspace`
- `Search result -> Action`

If these transitions are weak, the product becomes fragmented.

---

## 8. What Must Be First-Class From Day One

Even if implementation is staged, these areas must be first-class in the model from the beginning:

- bookmarks
- search
- workspace
- sync

If any of them is treated as “later add-on,” the model will narrow too early.

---

## 9. Product Center of Gravity

The short-term center of CTM is runtime.

The long-term center of CTM is workspace.

The connective tissue between them is:

- library
- bookmarks
- search
- sync

That is the correct mental model for the whole product.
