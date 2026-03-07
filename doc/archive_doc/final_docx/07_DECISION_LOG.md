# CTM — Decision Log

This is the curated decision log for the unified planning set.

## D-001

**Decision:** CTM is a terminal-first browser workspace manager.  
**Why:** The product is larger than a tab/session utility.  
**Implication:** All planning must include runtime, library, bookmarks, search, workspace, sync, and power.

## D-002

**Decision:** Product definition is not split into separate v1/v2 worldviews.  
**Why:** Deferring bookmarks, search, sync, or workspace narrows the model too early.  
**Implication:** These areas must exist in the architecture from day one, even if implementation is staged.

## D-003

**Decision:** Runtime and library must coexist.  
**Why:** CTM needs both live browser control and durable long-term value.  
**Implication:** Tabs/groups and sessions/collections are both product core.

## D-004

**Decision:** Bookmarks are first-class.  
**Why:** They affect search, workspace, sync, and long-term knowledge organization.  
**Implication:** Bookmarks must have a top-level product presence and a distinct model.

## D-005

**Decision:** Sync uses a dual-track model.  
**Why:** Native bookmarks and CTM-owned data belong to different sync systems.  
**Implication:** Google/Chrome Sync handles native bookmarks; CTM + iCloud handle CTM-owned data.

## D-006

**Decision:** CTM must be local-first.  
**Why:** The product should remain usable offline and under sync failure.  
**Implication:** Cloud sync is enhancement, not precondition.

## D-007

**Decision:** Workspace is the long-term center.  
**Why:** Users manage work contexts, not isolated resources.  
**Implication:** Workspace must aggregate sessions, collections, bookmarks, tags, notes, and saved searches.

## D-008

**Decision:** Search is a product center.  
**Why:** Users should not have to guess resource type before searching.  
**Implication:** Search must span tabs, sessions, collections, bookmarks, and workspaces.

## D-009

**Decision:** The top-level product areas are fixed.  
**Why:** The IA must reflect the whole product.  
**Implication:** Targets, Tabs, Groups, Sessions, Collections, Bookmarks, Search, Workspaces, and Sync are the stable top-level areas.

## D-010

**Decision:** CLI, TUI, and command palette have different roles.  
**Why:** They are different command surfaces, not duplicate products.  
**Implication:** CLI = explicit automation, TUI = exploration/organization, palette = fast global shortcuts.

## D-011

**Decision:** Build order follows product skeleton, not feature count.  
**Why:** Stable layers reduce rework.  
**Implication:** Build in this order: runtime, library, bookmarks, sync, search, workspace, interaction, power.

## D-012

**Decision:** Bookmark modeling uses three layers.  
**Why:** Native source, CTM mirror, and CTM overlay are different responsibilities.  
**Implication:** Do not collapse source, mirror, and overlay into one object.

## D-013

**Decision:** Sync must be visible in the product.  
**Why:** Users must trust cross-device continuity.  
**Implication:** Sync is a top-level area, not hidden infrastructure.

## D-014

**Decision:** The extension set is part of the product, not a wishlist.  
**Why:** You explicitly want the full product, not a narrow tool.  
**Implication:** Bookmarks, search, workspace, sync, and power-user automation stay in-scope.

## D-015

**Decision:** CTM remains browser-centered.  
**Why:** It needs a clear product boundary.  
**Implication:** Notes/tags/workspaces serve browser-centered work, not a generic note-taking product.

## Maintenance Rule

If a future decision changes one of these, do not silently edit history.  
Add a new decision that explicitly supersedes the older one.
