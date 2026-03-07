# CTM — Start Here

## What This Folder Is

`final_docx/` is the curated, unified planning set for CTM.

It merges:

- the product-level thinking from `codex_doc/`
- the implementation-level planning from `claude_doc/`

The goal is simple:

- **you only need to read one set of docs to understand the project**

The older folders are still useful as raw source material, but this folder is the new working set.

---

## What CTM Is

CTM is a:

**terminal-first browser workspace manager**

It is not just:

- a tab manager
- a session saver
- a bookmark viewer

It must support, as one coherent product:

- runtime browser control
- long-term library management
- bookmarks
- unified search
- workspaces
- cloud sync
- power-user automation

---

## Reading Order

If you are a human owner and want the shortest path, read in this order:

1. [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
2. [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
3. [03_USER_EXPERIENCE_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/03_USER_EXPERIENCE_MODEL.md)
4. [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
5. [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)

If you are sending an agent to build code, the minimum reading set should be:

1. [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
2. [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
3. [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)
4. [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
5. [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)

---

## Document Set

- [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
  Product definition, principles, scope, and long-term positioning.

- [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
  Core domain objects, capability map, source-of-truth boundaries.

- [03_USER_EXPERIENCE_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/03_USER_EXPERIENCE_MODEL.md)
  Information architecture, navigation, command surfaces, and major user journeys.

- [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)
  Unified system structure, layers, runtime model, module boundaries.

- [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
  Build order, stages, deliverables, and what must be considered from day one.

- [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)
  Acceptance criteria, testing layers, review gates, and phase exit rules.

- [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)
  Stable product decisions that should not be re-argued each turn.

- [08_AGENT_EXECUTION_RULES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/08_AGENT_EXECUTION_RULES.md)
  Operating rules for agents so they do not silently narrow the product or skip planning updates.

- [09_CHANGE_POLICY.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/09_CHANGE_POLICY.md)
  Rules for how future requirement changes must be handled.

- [10_AGENT_TASK_TEMPLATE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/10_AGENT_TASK_TEMPLATE.md)
  Reusable template for giving implementation tasks to agents.

- [11_REVIEW_TEMPLATE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/11_REVIEW_TEMPLATE.md)
  Reusable template for reviewing completed work and deciding whether it is really done.

- [99_SOURCE_MAP.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/99_SOURCE_MAP.md)
  Where each unified doc came from in `claude_doc/` and `codex_doc/`.

---

## Single Sources of Truth

In this folder, these are the key source-of-truth documents:

- Product definition: [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
- Domain model: [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
- UX structure: [03_USER_EXPERIENCE_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/03_USER_EXPERIENCE_MODEL.md)
- System structure: [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)
- Delivery order: [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
- Completion standard: [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)
- Stable strategic choices: [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)
- Agent operating guardrails: [08_AGENT_EXECUTION_RULES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/08_AGENT_EXECUTION_RULES.md)
- Requirement change process: [09_CHANGE_POLICY.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/09_CHANGE_POLICY.md)

---

## How To Use This As a Beginner

Do not start from implementation details.

Use this sequence:

1. understand what the product is
2. understand what objects exist
3. understand how users move through it
4. understand the architecture shape
5. understand the build order
6. understand how to know something is done

If you keep that order, the project stays coherent.
