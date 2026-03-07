# CTM — Acceptance and Quality Gates

## 1. Purpose

This document defines how to decide whether CTM is progressing correctly.

It combines:

- phase acceptance logic
- product review gates
- engineering quality gates
- cross-phase checks

The goal is to stop “lots of code exists” from being confused with “the product is actually solid.”

---

## 2. Acceptance Philosophy

A stage is only complete when all three are true:

1. the capability exists
2. it behaves correctly
3. it still fits the product model

So every stage should be checked at three levels:

- `Product fit`
- `Behavioral correctness`
- `Quality / test coverage`

---

## 3. Stage Exit Rules

## Stage 1 — Runtime

Must prove:

- live browser state is visible
- targets are scoped correctly
- tabs/groups actions work on the right target
- event flow is reliable enough to build on

## Stage 2 — Library

Must prove:

- sessions and collections are real CTM-owned resources
- capture and restore semantics are correct
- list vs get semantics are clear and stable

## Stage 3 — Bookmarks

Must prove:

- bookmarks are modeled as first-class resources
- bookmark source, mirror, and overlay are not confused
- bookmarks are ready to participate in search and workspace

## Stage 4 — Sync

Must prove:

- CTM-owned resources are sync-aware
- sync state is visible
- local-first still holds under failure

## Stage 5 — Search

Must prove:

- search is cross-resource
- results are actionable
- saved search is possible

## Stage 6 — Workspace

Must prove:

- workspace is not a renamed collection
- it can aggregate multiple resource types
- it supports startup / resume logic

## Stage 7 — Interaction

Must prove:

- CLI, TUI, and palette align semantically
- navigation reflects the product IA
- users can discover and complete core journeys

## Stage 8 — Power

Must prove:

- batch and advanced actions amplify existing value
- they do not create surface inconsistency

---

## 4. Product Review Gate

Every major change should be checked against these questions:

1. does this strengthen runtime + library together
2. does this preserve bookmarks as a first-class domain
3. does this improve or preserve search as a global entrypoint
4. does this reinforce workspace as a long-term center
5. does this preserve local-first behavior
6. does this keep sync visible and trustworthy
7. does this keep command surfaces aligned

If most answers are “no,” the product is drifting.

---

## 5. Architecture Review Gate

Every major change should also be checked against:

1. are browser-owned and CTM-owned data still clearly separated
2. are bookmark source, mirror, and overlay still separate
3. is sync still treated as a first-class layer
4. is workspace still above raw resources, not collapsing into them
5. is search still a shared layer, not duplicated per view

---

## 6. Interaction Review Gate

For CLI/TUI/palette behavior, always check:

1. is the action discoverable
2. does the same action mean the same thing across surfaces
3. can the user tell what resource and scope they are acting on
4. does the action move naturally within the IA
5. is there visible feedback for the state change

---

## 7. Testing Layers

CTM should be validated across four layers:

## 7.1 Domain tests

Check:

- object semantics
- lifecycle semantics
- sync/state transitions

## 7.2 Contract tests

Check:

- action shapes
- message correctness
- request/response guarantees

## 7.3 Interaction tests

Check:

- TUI navigation
- palette behavior
- search flows
- workspace flows

## 7.4 End-to-end tests

Check:

- live browser integration
- extension/daemon flow
- sync-visible flows
- restore/startup flows

---

## 8. Non-Negotiable Quality Gates

The following should never be skipped:

- contracts remain explicit
- bookmarks remain first-class
- sync state remains visible
- workspace remains distinct from collection/session
- search remains cross-resource
- local-first remains true

---

## 9. What “Done” Should Mean

“Done” should not mean:

- the code compiles
- the command exists
- the view renders

“Done” should mean:

- the behavior works
- the behavior is testable
- the behavior fits the product model
- the behavior can survive later expansion

---

## 10. Final Quality Statement

CTM is only healthy when:

- implementation is correct
- product shape is preserved
- long-term expansion remains possible

That is the quality bar for the whole project.
