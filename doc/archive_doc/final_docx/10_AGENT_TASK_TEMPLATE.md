# CTM — Agent Task Template

## 1. Purpose

This template is for assigning work to an agent in a way that stays aligned with the CTM product model.

It is designed for a non-technical owner.

That means:

- the owner can describe the goal in plain language
- the agent is responsible for mapping that goal onto the planning set
- the task prompt should still be structured enough to avoid drift

Use this template whenever you ask an agent to:

- build a new feature
- refactor an area
- change product behavior
- add a new command or view
- change sync / search / workspace behavior

---

## 2. How To Use This Template

There are two layers:

1. **Owner layer**
   - short, plain-language request

2. **Agent expansion layer**
   - structured execution interpretation

As the owner, you only need to fill the owner layer if you want.

The agent must expand the rest before coding.

---

## 3. Owner Layer (Simple Version)

Copy this block when giving an agent a task:

```md
Task:

What I want:
- 

Why I want it:
- 

What should feel different after this is done:
- 

Anything I definitely want included:
- 

Anything I definitely do not want:
- 
```

This is enough for a first pass if the agent is disciplined.

---

## 4. Agent Expansion Layer (Required Before Coding)

After reading the owner request, the agent must rewrite the task into this structure.

```md
# Task Title

## 1. Requested Outcome
- What must exist or behave differently when this work is done

## 2. Product Area
- runtime / library / bookmarks / search / workspace / sync / power

## 3. Domain Objects Affected
- Which objects are touched

## 4. User Journey Affected
- Which primary or secondary journeys are changed

## 5. Command Surfaces Affected
- CLI / TUI / command palette

## 6. Source-of-Truth Impact
- Which truth boundary is touched

## 7. In Scope
- Exact items included in this task

## 8. Out of Scope
- Exact items deliberately excluded from this task

## 9. Docs To Read First
- final_docx docs
- deeper raw docs if needed

## 10. Docs That Must Be Updated
- Which docs must be changed if implementation changes meaning

## 11. Acceptance Criteria
- What must be true to call it done

## 12. Risks / Change Sensitivity
- What could accidentally narrow the product or break later expansion
```

The agent should not start implementation until this expansion is clear.

---

## 5. Required Product Classification

Every task must be classified into one primary product area:

- `Runtime`
- `Library`
- `Bookmarks`
- `Search`
- `Workspace`
- `Sync`
- `Power`

If the task crosses multiple areas, the agent must explicitly say so.

Example:

- Primary area: `Bookmarks`
- Secondary areas: `Search`, `Workspace`, `Sync`

This is important because many “small changes” are actually cross-domain changes.

---

## 6. Required Domain Check

For each task, the agent must explicitly identify:

- what objects are being created or changed
- what their source of truth is
- whether they must later be searchable
- whether they must later be sync-aware
- whether they must later attach to a workspace

If the agent cannot answer these, the task definition is not ready.

---

## 7. Required UX Check

For each task, the agent must say:

- where the user enters this behavior
- where the user sees the result
- what command surface owns the action
- how the user will discover it

This prevents “implemented but invisible” features.

---

## 8. Required Documentation Check

Before closing the task, the agent must say whether the task required updates to:

- [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
- [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
- [03_USER_EXPERIENCE_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/03_USER_EXPERIENCE_MODEL.md)
- [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)
- [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
- [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)
- [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)

If the answer is “none,” the agent should still explain why no planning doc changed.

---

## 9. Good Task Examples

## Example A — Add bookmark tagging

### Bad task

“给书签加 tag。”

### Better task

```md
Task:

What I want:
- I want bookmarks to support user-defined tags.

Why I want it:
- I want bookmarks to become part of long-term organization, not just a browser tree.

What should feel different after this is done:
- I can tag bookmarks and later search/filter/workspace-organize them through CTM.

Anything I definitely want included:
- Works with search
- Works with workspace
- Remains separate from native Chrome bookmark ownership

Anything I definitely do not want:
- Do not overwrite Chrome’s native bookmark model
```

## Example B — Improve sync visibility

### Better task

```md
Task:

What I want:
- I want a real sync status area that shows whether my CTM data is healthy across devices.

Why I want it:
- I need to trust the product before I rely on it as my main system.

What should feel different after this is done:
- I can understand whether data is synced, stale, conflicted, or broken.

Anything I definitely want included:
- Visible statuses
- Repair entrypoint
- Clear distinction between Google bookmark sync and CTM iCloud sync

Anything I definitely do not want:
- Do not hide sync again as background-only infrastructure
```

---

## 10. Bad Task Smells

If a task prompt looks like this, the agent should slow down:

- “just add ...”
- “just make it support ...”
- “just sync ...”
- “just add another view ...”

These often hide model-level changes.

When this happens, the agent should expand the task carefully before coding.

---

## 11. Completion Report Template

After implementation, the agent should report back in this format:

```md
## Completed
- 

## Product areas touched
- 

## Domain objects touched
- 

## Docs updated
- 

## Acceptance evidence
- 

## Remaining risks
- 
```

This makes it easier for the owner to understand progress without reading code.

---

## 12. Final Rule

The purpose of this template is not bureaucracy.

It is to make sure that when the owner changes direction later, the project still has enough structure that no one needs to “抓瞎”.
