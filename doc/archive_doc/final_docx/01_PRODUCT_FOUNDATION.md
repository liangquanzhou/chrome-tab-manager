# CTM — Product Foundation

## 1. Product Definition

CTM is a:

**terminal-first browser workspace manager**

That means CTM is not only about controlling a live browser.

It must unify:

- current browser runtime state
- long-term resource library
- bookmarks as knowledge assets
- unified search
- workspace organization
- cloud sync
- power-user workflows

This is the core definition of the product.

---

## 2. Product Scope

CTM must fully support these seven capability areas:

1. `Runtime`
2. `Library`
3. `Bookmarks`
4. `Search`
5. `Workspace`
6. `Sync`
7. `Power`

These are not optional side modules.

They are the product.

---

## 3. Core Product Promise

CTM should let a user:

1. control what is happening in the browser now
2. capture what matters for later
3. organize it into a durable knowledge library
4. find it again without guessing resource type
5. resume work through workspaces
6. carry that work across devices

In short:

**control -> capture -> organize -> search -> resume -> sync**

---

## 4. Product Principles

These principles should not drift.

### 4.1 Runtime and library must coexist

CTM cannot become only a live browser control tool.  
It also cannot become only a static library manager.

It must support both.

### 4.2 Bookmarks are first-class

Bookmarks are not a side view.

They must participate in:

- search
- workspaces
- metadata enrichment
- long-term organization

### 4.3 Search is a product center

Search is not just filtering inside one list.

It must be a cross-resource entrypoint over:

- tabs
- sessions
- collections
- bookmarks
- workspaces

### 4.4 Workspace is the long-term center

The long-term center of the product is not a tab, not a session, not a collection.

It is the workspace.

### 4.5 Local-first always

CTM must work locally first.

Cloud sync enhances the product, but local use cannot depend on the cloud being healthy.

### 4.6 Sync must be visible and trustworthy

Users must be able to understand:

- what is synced
- what is not
- what failed
- what conflicted

### 4.7 Power-user capability is part of the product

Automation, batch operations, export, dedupe, sorting, diagnostics:

these are not extra polish.  
They are part of the product’s value.

---

## 5. Source-of-Truth Model

CTM needs a clean truth model.

### Browser-owned truth

- live tabs
- live groups
- live windows
- native bookmark source

### CTM-owned truth

- sessions
- collections
- bookmark overlays
- workspaces
- tags
- notes
- saved searches

### Cloud role

Cloud should replicate truth, not redefine it.

So:

- Google / Chrome Sync handles native bookmarks
- CTM handles its own long-term library
- iCloud sync carries CTM-owned library across devices

---

## 6. What CTM Is Not

CTM should not drift into these shapes:

- a generic browser replacement
- a plain bookmark app
- a generic cloud sync client
- a disconnected toolbox of unrelated commands

CTM must remain:

**browser-centered, resource-oriented, workspace-aware**

---

## 7. Why This Product Is Bigger Than a Tab Tool

A tab tool only answers:

- what is open now
- how do I close, group, or switch tabs

CTM must answer larger questions:

- what have I already collected
- what belongs to this project
- where is that thing I saw before
- how do I continue this work tomorrow
- how do I continue this work on another machine

That is why bookmarks, search, workspace, and sync must be designed from the beginning.

---

## 8. Complete Product Shape

If CTM is built correctly, users should be able to start from any of these entrypoints:

- the browser runtime
- their saved library
- bookmarks
- unified search
- a workspace
- sync state

And still move naturally through the same product.

That is the real product shape.
