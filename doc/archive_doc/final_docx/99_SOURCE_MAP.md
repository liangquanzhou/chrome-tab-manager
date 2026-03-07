# CTM — Source Map

## Purpose

This file explains how the old raw docs were consolidated into `final_docx/`.

The goal of `final_docx/` is not to preserve every document one-to-one.

It is to give you a cleaner working set.

---

## Source Folders

Raw product-heavy source:

- [codex_doc](/Users/didi/ai_projects/chrome-tab-tui/codex_doc)

Raw implementation-heavy source:

- [claude_doc](/Users/didi/ai_projects/chrome-tab-tui/claude_doc)

---

## Mapping

### [00_START_HERE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/00_START_HERE.md)

Built from:

- overall structure of `codex_doc/`
- overall structure of `claude_doc/`

### [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)

Built mainly from:

- [PRODUCT_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/PRODUCT_ARCHITECTURE.md)
- [PRODUCT_PRINCIPLES.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/PRODUCT_PRINCIPLES.md)
- [DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/DECISION_LOG.md)

### [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)

Built mainly from:

- [DOMAIN_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/DOMAIN_MODEL.md)
- [CAPABILITY_MAP.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/CAPABILITY_MAP.md)

### [03_USER_EXPERIENCE_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/03_USER_EXPERIENCE_MODEL.md)

Built mainly from:

- [INFORMATION_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/INFORMATION_ARCHITECTURE.md)
- [NAVIGATION_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/NAVIGATION_MODEL.md)
- [COMMAND_SURFACES.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/COMMAND_SURFACES.md)
- [USER_JOURNEYS.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/USER_JOURNEYS.md)

### [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)

Built mainly from:

- [PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/PLAN.md)
- [DESIGN.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/DESIGN.md)
- [PRODUCT_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/PRODUCT_ARCHITECTURE.md)

### [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)

Built mainly from:

- [BUILD_ORDER.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/BUILD_ORDER.md)
- [PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/PLAN.md)
- [FEATURES.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/FEATURES.md)

### [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)

Built mainly from:

- [ACCEPTANCE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/ACCEPTANCE.md)
- [TUI_GUIDELINE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/TUI_GUIDELINE.md)
- [LESSONS.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/LESSONS.md)

### [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)

Built mainly from:

- [DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/codex_doc/DECISION_LOG.md)

### [08_AGENT_EXECUTION_RULES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/08_AGENT_EXECUTION_RULES.md)

Built as a curated execution policy layer on top of:

- [00_START_HERE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/00_START_HERE.md)
- [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
- [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)

### [09_CHANGE_POLICY.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/09_CHANGE_POLICY.md)

Built as a curated change-management layer on top of:

- [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)
- [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
- [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)

### [10_AGENT_TASK_TEMPLATE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/10_AGENT_TASK_TEMPLATE.md)

Built as a new practical template for assigning work to agents using the unified planning set.

### [11_REVIEW_TEMPLATE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/11_REVIEW_TEMPLATE.md)

Built as a new practical template for structured review and acceptance of completed work.

### [99_SOURCE_MAP.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/99_SOURCE_MAP.md)

Built for:

- explaining this consolidation

---

## What Stayed in Raw Source Folders

Some documents remain better as raw reference rather than re-written summaries:

- [CONTRACTS.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/CONTRACTS.md)
- [DESIGN.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/DESIGN.md)
- [TUI_GUIDELINE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/TUI_GUIDELINE.md)

These are still worth keeping in `claude_doc/` as deeper implementation references.

---

## Recommended Working Rule

Use `final_docx/` as the main planning set.

Only open `claude_doc/` or `codex_doc/` when:

- you need deeper implementation detail
- you want to trace why a unified summary says something
- an agent needs detailed contracts or design specifics
