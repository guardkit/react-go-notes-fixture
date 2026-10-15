---
id: TASK-NLTC-003
title: Add end-to-end test for the total note count
task_type: testing
parent_review: TASK-REV-8BFC
feature_id: FEAT-8BFC
wave: 2
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-NLTC-001
scaffolding_kind: tests
---

Add an end-to-end test that the notes list view shows the total number of notes above the list against the running stack.

## The words of the request this task serves

_none — this is a tests task_

## Files to Create

- _none_

## Files to Modify

- `tests/e2e/specs/journey.spec.ts`

## Acceptance Criteria

- [ ] An end-to-end test signs in, observes the notes list, and asserts the total number of notes is displayed above the notes list in the format "N notes".
- [ ] The test asserts the displayed total matches the number of notes actually shown in the list (sourced from the same collection).

## Implementation Notes

- This task tests the surface delivered by TASK-NLTC-001; run it after that task lands.
- Extend the existing Playwright journey in `tests/e2e/specs/journey.spec.ts`, reusing its helpers (`tests/e2e/specs/helpers.ts`) and the existing sign-in/seed flow.
- Assert against the rendered list region and the summary region above it; do not assert the count inside a note item.