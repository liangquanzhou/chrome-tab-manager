# CTM — Implementation Plan

## 1. Planning Strategy

Implementation should follow product skeleton, not feature accumulation.

That means:

- first define the layers and object boundaries
- then build them in an order that avoids rework

---

## 2. Recommended Build Order

## Stage 1 — Runtime Foundation

Build first:

- targets
- windows
- tabs
- groups
- live event flow

Why:

- everything else eventually consumes runtime state

Stage exit condition:

- the system can reliably understand and act on live browser state

---

## Stage 2 — Library Foundation

Build next:

- sessions
- collections
- capture / restore
- preview / delete / export basics

Why:

- CTM must become more than a runtime tool as early as possible

Stage exit condition:

- live state can be turned into durable CTM-owned resources

---

## Stage 3 — Bookmarks as First-Class

Build next:

- bookmark source integration
- bookmark mirror
- bookmark overlay model
- bookmark browse/search basics

Why:

- bookmarks affect search, workspace, and sync architecture

Stage exit condition:

- bookmarks are part of the product model, not a future add-on

---

## Stage 4 — Sync Foundation

Build next:

- local-first persistence model
- iCloud sync model for CTM-owned data
- visible sync states
- conflict model

Why:

- sync changes object identity, metadata, and lifecycle assumptions

Stage exit condition:

- the product has a real sync model, even if the UX is still simple

---

## Stage 5 — Search Layer

Build next:

- cross-resource search
- saved searches
- basic smart collection logic

Why:

- search depends on all major resource types existing

Stage exit condition:

- users can find resources without first guessing resource type

---

## Stage 6 — Workspace Layer

Build next:

- workspace creation
- workspace attachments
- workspace startup / resume
- workspace search scope

Why:

- workspace is the long-term center, but it should be built on top of stable resources

Stage exit condition:

- users can organize and resume work as contexts, not just resource lists

---

## Stage 7 — Interaction System

Build next:

- CLI structure
- TUI structure
- command palette
- help / feedback / navigation consistency

Why:

- interaction should expose a stable model, not invent one first

Stage exit condition:

- CLI, TUI, and palette all reflect the same product semantics

---

## Stage 8 — Power Layer

Build last:

- batch actions
- sorting
- dedupe
- export variants
- automation hooks
- diagnostics / repair

Why:

- these matter a lot, but they should amplify a stable system

Stage exit condition:

- CTM feels like a daily driver, not just a correct system

---

## 3. What Must Be Considered From Day One

Even before implementation reaches the later stages, these concerns must be considered immediately:

- bookmarks are first-class
- search is global
- workspace is central
- sync is visible
- local-first is non-negotiable
- resource identities are stable
- overlays are separate from browser-native data

---

## 4. What Can Be Implemented Later Without Breaking Direction

These can come later without changing the product worldview:

- advanced automation hooks
- richer export formats
- tab sorting variants
- duplicate detection UX polish
- more power-user shortcuts

They still matter, but they do not redefine the architecture.

---

## 5. What Should Not Be Delayed

These should not be pushed out of the model:

- bookmarks
- iCloud sync
- workspace
- search

If they are treated as “future extras,” later stages will force redesign.

---

## 6. Practical Stage Mapping

If you want a simple operational view:

- `Runtime` gives CTM live control
- `Library` gives CTM durable value
- `Bookmarks` gives CTM knowledge depth
- `Sync` gives CTM cross-device continuity
- `Search` gives CTM a unified entrypoint
- `Workspace` gives CTM long-term center of gravity
- `Interaction` makes all of that usable
- `Power` makes all of that efficient

That is the right order of importance.
