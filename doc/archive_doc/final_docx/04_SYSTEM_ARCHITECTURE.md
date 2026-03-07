# CTM — System Architecture

## 1. Architecture Goal

The architecture must support the full product definition, not just the first few commands.

That means the system has to support:

- runtime browser control
- CTM-owned long-term library
- bookmarks as a first-class domain
- search and workspace layers
- visible sync

---

## 2. Core Runtime Chain

At the highest level, CTM runs through this chain:

`Chrome Extension -> Native Messaging -> CTM NM Shim -> CTM Daemon -> CLI/TUI clients`

This is the live runtime path.

### High-level roles

- **Extension**
  Talks to Chrome APIs and emits live browser state/events.

- **NM Shim**
  Bridges Chrome Native Messaging and CTM’s internal daemon channel.

- **Daemon**
  Central runtime coordinator and library owner.

- **Client layer**
  CLI and TUI consume daemon capabilities.

---

## 3. Product Layers

The system should be understood in five layers.

## 3.1 Browser Runtime Layer

Handles:

- targets
- windows
- tabs
- groups
- bookmark source
- runtime events

## 3.2 Library Layer

Handles:

- sessions
- collections
- bookmark mirror
- bookmark overlay
- notes / tags / saved searches

## 3.3 Workspace Layer

Handles:

- workspace definitions
- resource attachments
- workspace startup / resume

## 3.4 Sync Layer

Handles:

- iCloud-backed CTM data sync
- sync state
- conflict state
- repair / rebuild

## 3.5 Interaction Layer

Handles:

- CLI
- TUI
- command palette
- help / navigation / action presentation

---

## 4. Ownership Model

This boundary should remain stable.

### Browser-owned

- live tabs
- live groups
- live windows
- native bookmarks

### CTM-owned

- sessions
- collections
- bookmark overlays
- workspaces
- tags / notes / saved searches

### Sync-owned metadata

- sync status
- device context
- conflict state

---

## 5. Architectural Backbone

The real architectural backbone is:

1. `Runtime`
2. `Library`
3. `Bookmarks`
4. `Sync`
5. `Search`
6. `Workspace`
7. `Interaction`
8. `Power`

This is the same order the product should be built around.

---

## 6. What Must Be Designed Early

Even if not all code is written immediately, these architectural concerns must be present from day one:

- bookmark layering
- iCloud sync boundary
- stable resource identities
- workspace as a first-class long-term object
- cross-resource search
- command surface consistency

If these are delayed, later implementation will force redesign.

---

## 7. Module-Level Direction

At a high level, the implementation side should remain separated into:

- config / paths
- protocol / contracts
- client transport
- daemon runtime
- bookmarks domain
- sync domain
- workspace domain
- search domain
- TUI / CLI interaction

The point is not the exact package names.

The point is to preserve clear product boundaries in code.

---

## 8. Runtime vs Long-Term Data

The most important architectural separation is:

### Runtime state

- live
- temporary
- browser-owned

### Long-term state

- durable
- organized
- searchable
- syncable
- CTM-owned

This distinction protects the product from collapsing into a mere browser controller.

---

## 9. Search and Workspace Position

Search and Workspace should sit above raw resources.

They are not lower-level storage features.

They are organizing layers that consume:

- runtime
- sessions
- collections
- bookmarks
- sync state

That is why they should not be treated as minor add-ons.

---

## 10. Architecture Red Flags

The architecture is drifting if:

- bookmarks are modeled only as a browser view
- sync is bolted on after storage is already fixed
- workspace is just another collection type
- search is implemented separately inside each view
- TUI behavior starts defining product semantics instead of consuming them

---

## 11. Architecture Summary

The system should be designed as:

**a runtime-aware, library-owning, bookmark-integrated, sync-visible, workspace-centered terminal product**

That is the architectural target.
