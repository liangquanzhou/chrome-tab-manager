# CTM — User Experience Model

## 1. Top-Level Areas

CTM should expose these top-level areas:

- Targets
- Tabs
- Groups
- Sessions
- Collections
- Bookmarks
- Search
- Workspaces
- Sync

This is the full user-facing structure of the product.

---

## 2. Area Roles

### Runtime entry

- Targets
- Tabs
- Groups

These answer:

- what is happening in the browser now
- where am I acting

### Library entry

- Sessions
- Collections
- Bookmarks

These answer:

- what have I saved
- what have I curated
- what knowledge assets do I already have

### Global entry

- Search
- Workspaces
- Sync

These answer:

- where is the thing I want
- what project am I in
- is my system healthy and synced

---

## 3. Navigation Model

CTM navigation should support three kinds of users:

### Runtime-first users

They start with live browser state:

- Targets
- Tabs
- Groups

### Library-first users

They start with stored resources:

- Sessions
- Collections
- Bookmarks

### Goal-first users

They start with:

- Search
- Workspaces

This is important: the product must feel natural from all three entrypoints.

---

## 4. Command Surfaces

CTM has three command surfaces.

## 4.1 CLI

Best for:

- explicit actions
- automation
- export/import
- diagnostics
- scripts

## 4.2 TUI

Best for:

- browsing
- exploring
- selecting
- organizing
- multi-step workflows

## 4.3 Command Palette

Best for:

- fast global actions inside TUI
- jumping across resource areas
- lightweight command execution

The palette should not become a second CLI.

---

## 5. Major User Journeys

## 5.1 Capture research

Tabs -> Groups -> Session / Collection -> Workspace

### Meaning

Turn live browsing into long-term assets.

## 5.2 Resume work

Workspace -> Session / Collection / Bookmark -> Restore/Open -> Tabs

### Meaning

Return to a work context, not just a browser window.

## 5.3 Find something vaguely remembered

Search -> mixed results -> action

### Meaning

Search is the unified entrypoint when the user does not know the resource type.

## 5.4 Curate a reusable pack

Tabs / Bookmarks / Search -> Collection -> Export / Restore / Attach to Workspace

### Meaning

Collections are curated reusable packages, not snapshots.

## 5.5 Use bookmarks as knowledge

Bookmarks -> Tag / Note / Attach -> Workspace / Search

### Meaning

Bookmarks become part of the knowledge system.

## 5.6 Continue on another device

Sync -> Workspaces / Sessions / Collections -> Resume

### Meaning

The product supports continuity, not just storage.

---

## 6. UX Rules

These user experience rules should remain stable:

1. users can move from runtime to library naturally
2. users can move from library back to runtime naturally
3. bookmarks can enter search and workspaces naturally
4. search results must be actionable
5. sync must be visible
6. workspaces must feel like contexts, not static folders

---

## 7. UX Red Flags

The product is drifting if:

- search becomes only a local filter
- bookmarks are treated like a side panel
- workspaces are just renamed collections
- sync disappears into the background
- CLI, TUI, and palette develop different action meanings

---

## 8. UX Summary

The full CTM experience should feel like:

**browse now, save what matters, find it later, re-enter it as a workspace, and continue it anywhere.**
