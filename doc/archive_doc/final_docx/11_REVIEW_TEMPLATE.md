# CTM — Review Template

## 1. Purpose

This template is for reviewing work after an agent says something is done.

It is meant to answer:

- what changed
- whether it really matches the product model
- whether it is actually done
- what still feels risky

This template is written for both:

- agents who self-review
- reviewers who independently verify work

---

## 2. Review Philosophy

A good review does not only ask:

- “does the code run?”

It must also ask:

- “does this still fit CTM?”
- “did this preserve future extensibility?”
- “did this keep the product coherent?”

---

## 3. Review Output Format

Use this exact structure when reviewing meaningful work:

```md
## Verdict
- accepted / conditionally accepted / not accepted

## What changed
- 

## Product areas touched
- 

## Domain objects touched
- 

## Findings
- ordered by severity

## Acceptance evidence
- commands run
- behaviors observed
- docs updated

## Open questions
- 

## Residual risks
- 

## Next actions
- 
```

This keeps reviews readable even for a non-technical owner.

---

## 4. Mandatory Review Dimensions

Every serious review should check all of these.

## 4.1 Product Fit

Ask:

- does this strengthen or weaken the product definition
- did any first-class area get silently downgraded
- does this still support the full CTM shape

## 4.2 Domain Fit

Ask:

- were the right objects changed
- was source-of-truth preserved
- were overlays kept separate from native data
- did the resource relationships remain coherent

## 4.3 UX Fit

Ask:

- is the new behavior discoverable
- does it fit navigation and command surfaces
- does the result feel like part of the same product

## 4.4 Sync/Search/Workspace Fit

Ask:

- is this searchable if it should be
- is this sync-aware if it should be
- can this participate in workspace if it should

## 4.5 Change Safety

Ask:

- did this make future changes easier or harder
- did it introduce hidden coupling
- did it quietly narrow future options

---

## 5. Severity Guide

When writing findings, use this meaning:

### High

Blocks acceptance because it breaks:

- product direction
- core behavior
- source-of-truth integrity
- future expandability

### Medium

Does not fully block, but leaves meaningful risk in:

- UX consistency
- domain consistency
- review confidence
- growth path

### Low

Cleanup, polish, alignment, or documentation gaps that do not change the main outcome.

---

## 6. Acceptance Evidence Checklist

A review should capture evidence from three levels:

### Level 1 — Product evidence

- which product area changed
- which user journey improved
- whether planning docs were updated

### Level 2 — Behavioral evidence

- what commands were run
- what flows were exercised
- what user-visible result happened

### Level 3 — Quality evidence

- tests
- contracts
- acceptance checks
- known gaps

---

## 7. Documentation Review Checklist

A proper review should explicitly check whether the right docs changed.

### Product-level docs

- [01_PRODUCT_FOUNDATION.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/01_PRODUCT_FOUNDATION.md)
- [02_DOMAIN_AND_CAPABILITIES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/02_DOMAIN_AND_CAPABILITIES.md)
- [03_USER_EXPERIENCE_MODEL.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/03_USER_EXPERIENCE_MODEL.md)
- [07_DECISION_LOG.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/07_DECISION_LOG.md)

### Execution docs

- [04_SYSTEM_ARCHITECTURE.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/04_SYSTEM_ARCHITECTURE.md)
- [05_IMPLEMENTATION_PLAN.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/05_IMPLEMENTATION_PLAN.md)
- [06_ACCEPTANCE_AND_QUALITY_GATES.md](/Users/didi/ai_projects/chrome-tab-tui/final_docx/06_ACCEPTANCE_AND_QUALITY_GATES.md)

### Raw deep docs when relevant

- [claude_doc/CONTRACTS.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/CONTRACTS.md)
- [claude_doc/DESIGN.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/DESIGN.md)
- [claude_doc/TUI_GUIDELINE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/TUI_GUIDELINE.md)
- [claude_doc/ACCEPTANCE.md](/Users/didi/ai_projects/chrome-tab-tui/claude_doc/ACCEPTANCE.md)

---

## 8. Review Questions for a Non-Technical Owner

If you are not reading code, you can still ask the reviewer these questions:

1. What part of the product got better?
2. Which core objects changed?
3. Did this touch bookmarks, search, workspace, or sync?
4. Did the docs get updated before or with the code?
5. What would be hard to change later because of this work?
6. Is this truly done, or only partially done?
7. What should be tested next?

These questions are enough to catch most false “done” claims.

---

## 9. Review Smells

Be careful if a review sounds like this:

- “code compiles so it’s fine”
- “the command exists now”
- “we can fix the docs later”
- “workspace/search/sync not touched yet but it’s okay”
- “it’s probably fine for now”

These usually mean the work is not truly integrated into the product.

---

## 10. Suggested Closing Statements

When closing a review, use one of these:

### Accepted

The work is behaviorally correct, aligned with the product model, and safe to build on.

### Conditionally accepted

The main behavior is good, but a small set of follow-up issues should be fixed before building heavily on top of it.

### Not accepted

The implementation may exist, but it is not yet safe or aligned enough to treat as a stable base.

---

## 11. Final Rule

The review process exists to protect the owner from a very common failure mode:

**lots of implementation progress, but no stable product.**

If the review cannot explain why the work still fits the CTM product model, the work is not done enough.
