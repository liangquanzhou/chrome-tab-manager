# CTM — Change Policy

## 1. Purpose

This document defines how CTM should handle future requirement changes.

It exists for one reason:

**the owner expects the product to change later, and wants those changes to remain manageable.**

This document is written for agents.

---

## 2. Core Idea

Changing requirements is normal.  
Chaotic change is the problem.

So CTM should be changed through:

1. product-level clarification
2. domain/architecture update
3. implementation update
4. acceptance update

Not the other way around.

---

## 3. Change Categories

Every requested change should first be classified into one of these categories.

## 3.1 Product Scope Change

Examples:

- add a new top-level capability
- change the product definition
- change whether a domain is first-class

This is the highest-impact change type.

### Required docs to update first

- [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
- [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
- [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)

## 3.2 Domain Model Change

Examples:

- add a new core object
- split one object into two
- change resource ownership

### Required docs to update first

- [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
- [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)
- [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)

## 3.3 User Experience Change

Examples:

- add or remove top-level area
- change navigation model
- move actions between CLI/TUI/palette

### Required docs to update first

- [03_USER_EXPERIENCE_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/03_USER_EXPERIENCE_MODEL.md)
- [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)
- [claude_doc/TUI_GUIDELINE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/TUI_GUIDELINE.md)

## 3.4 Architecture / Contract Change

Examples:

- add new actions
- change message semantics
- add new runtime module

### Required docs to update first

- [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)
- [claude_doc/DESIGN.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/DESIGN.md)
- [claude_doc/CONTRACTS.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/CONTRACTS.md)

## 3.5 Delivery / Stage Change

Examples:

- reorder build stages
- change implementation priorities
- redefine completion milestones

### Required docs to update first

- [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
- [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)
- [claude_doc/PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/PLAN.md)
- [claude_doc/ACCEPTANCE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/ACCEPTANCE.md)

---

## 4. Required Change Sequence

Agents must apply changes in this order:

1. **clarify the requested change**
2. **identify the change category**
3. **update the right planning docs first**
4. **record or amend the decision**
5. **only then change implementation**
6. **update acceptance criteria**
7. **report residual impact**

This sequence is mandatory for major changes.

---

## 5. Decision Log Rule

If a change affects product direction, the agent must update:

- [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)

If the new decision replaces an older one, it must:

- explicitly mention which decision it supersedes
- explain why the change happened
- explain what the new implication is

Do not silently rewrite old meaning.

---

## 6. Change Safety Questions

Before implementing a significant change, agents must answer:

1. does this change product scope
2. does this change the domain model
3. does this change user navigation
4. does this change command surface responsibilities
5. does this change sync behavior
6. does this change acceptance criteria
7. will this make later changes easier or harder

If the change makes later changes harder, the agent should surface that explicitly.

---

## 7. Reversible vs Irreversible Changes

Agents should distinguish between:

### Reversible changes

- naming improvements
- UI placement changes
- extra commands
- additional views

### More expensive changes

- object identity changes
- sync model changes
- workspace semantics changes
- source-of-truth changes
- contract shape changes

Expensive changes require more documentation updates first.

---

## 8. Rule for a Non-Technical Owner

Because the owner is non-technical, agents must not rely on:

- implicit architecture knowledge
- code-reading as the main explanation
- “it’s obvious from implementation”

Agents must make changes legible through planning docs first.

That is how the owner keeps control without coding.

---

## 9. What To Do When a Request Seems Small But Changes the Model

Some changes sound small but are actually structural.

Examples:

- “can we just add bookmark tagging”
- “can we just sync this too”
- “can we just add another workspace behavior”

In such cases, the agent must:

1. explain that this is a model-level change
2. update the planning docs first
3. then implement

This prevents hidden architectural drift.

---

## 10. What To Do When a Request Truly Is Small

If a request does not change:

- product scope
- domain model
- navigation
- contracts
- sync behavior

then the agent may implement directly and update only the closest relevant doc if needed.

Not every small change needs a planning ceremony.

---

## 11. Product Coherence Rule

The most important standard for change is not:

- was this fast to code

It is:

- did this preserve a coherent product
- did this keep future expansion possible

That is the core protection against “后面做完了要改，那我就抓瞎”.

---

## 12. Final Change Statement

For CTM, change is expected.

The system should handle change by:

- updating product meaning first
- updating architecture second
- updating code third
- updating acceptance last

That is the safest way to let agents move fast without losing control.
