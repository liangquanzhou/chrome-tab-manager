# CTM — Agent Execution Rules

## 1. Purpose

This document is written for agents, not for end users.

Its goal is to make CTM implementation safer under iterative development, especially when:

- the owner is non-technical
- requirements evolve over time
- multiple agents may work on the project
- the product is intentionally broad

Agents should treat this file as an operating policy.

---

## 2. Primary Rule

**Do not guess product direction from code alone.**

Always derive direction from the planning set first.

The codebase is an implementation snapshot.  
The docs define what the product is supposed to become.

---

## 3. Required Reading Order Before Coding

Before starting any meaningful implementation, the agent must read in this order:

1. [00_START_HERE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/00_START_HERE.md)
2. [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
3. [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
4. [03_USER_EXPERIENCE_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/03_USER_EXPERIENCE_MODEL.md)
5. [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)
6. [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
7. [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)
8. [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)

If the work touches deeper implementation specifics, then also read:

- [claude_doc/CONTRACTS.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/CONTRACTS.md)
- [claude_doc/DESIGN.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/DESIGN.md)
- [claude_doc/TUI_GUIDELINE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/TUI_GUIDELINE.md)
- [claude_doc/ACCEPTANCE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/ACCEPTANCE.md)

---

## 4. Product Intent Rules

Agents must preserve these assumptions at all times:

### Rule 1

CTM is a full product, not a small tab utility.

### Rule 2

Bookmarks, search, workspace, sync, and power-user automation are in-scope product areas.

### Rule 3

The product owner is non-technical and mainly provides demand and product direction.

### Rule 4

The docs exist to reduce future change cost.  
Agents must prefer designs that remain editable later.

### Rule 5

Do not optimize for “fastest short-term coding” if it narrows the future product model.

---

## 5. What Agents Must Never Do

Agents must not:

- silently downgrade bookmarks to a side feature
- silently treat search as only local filtering
- silently collapse workspace into collection/session
- silently hide sync as a background-only mechanism
- silently redefine product scope based on current partial code
- silently assume the owner accepts a narrower product because implementation is hard

If something is hard, the agent may stage implementation.  
It may not silently shrink the product.

---

## 6. What Agents Must Explicitly Protect

Agents must explicitly protect:

- source-of-truth boundaries
- object identities
- workspace centrality
- local-first behavior
- sync visibility
- cross-resource searchability
- consistent command surfaces

These are not optional polish items.

---

## 7. Mandatory Design Questions Before Any Major Change

Before implementing a meaningful feature or refactor, the agent must answer:

1. Which product area does this belong to:
   - runtime
   - library
   - bookmarks
   - search
   - workspace
   - sync
   - power

2. Which domain objects does it affect?

3. What is the source of truth for the affected data?

4. Will this need to be searchable later?

5. Will this need to be sync-aware later?

6. Should this be attachable to a workspace later?

7. Which command surfaces need to expose it:
   - CLI
   - TUI
   - palette

If the agent cannot answer these clearly, it should not rush into coding.

---

## 8. Mandatory Documentation Updates

When a major feature is implemented or changed, the agent must update the relevant planning docs.

### Product-level changes

Update at least one of:

- [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
- [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
- [03_USER_EXPERIENCE_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/03_USER_EXPERIENCE_MODEL.md)
- [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)

### Architecture-level changes

Update at least one of:

- [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)
- [claude_doc/DESIGN.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/DESIGN.md)
- [claude_doc/CONTRACTS.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/CONTRACTS.md)

### Delivery/testing changes

Update at least one of:

- [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
- [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)
- [claude_doc/ACCEPTANCE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/ACCEPTANCE.md)

---

## 9. Coding Style for a Changing Product

Because the owner expects future changes, the agent should optimize for:

- clear separation of domains
- explicit contracts
- strong naming
- isolated responsibilities
- reversible choices
- low surprise for future maintainers

The agent should avoid:

- hidden coupling
- implicit semantics
- view-driven business logic
- over-optimized abstractions that make change harder

---

## 10. Staging Rule

If implementation must be staged, the agent should do this:

1. preserve the final product shape in docs and data model
2. deliver a smaller implementation slice
3. clearly mark what is stubbed, missing, or delayed
4. avoid pretending the smaller slice is the whole product

Staging is acceptable.  
Silent product shrinkage is not.

---

## 11. Acceptance Rule

An implementation is not complete just because:

- code compiles
- a command exists
- a view renders

It is only complete when:

- behavior works
- acceptance criteria are satisfied
- the product model is still coherent
- future extensions remain possible

---

## 12. Communication Rule

When reporting progress, agents should always make clear:

- what changed
- which product area it belongs to
- whether the docs were updated
- what is still missing
- what future change risk remains

This is especially important because the owner is not using code-level reasoning to judge progress.

---

## 13. Final Rule

If there is a conflict between:

- short-term convenience
- long-term product coherence

the agent should prefer long-term product coherence unless explicitly told otherwise.
