---
id: TASK-NLTC-002
title: Add unit tests for the total note count display
task_type: testing
parent_review: TASK-REV-8BFC
feature_id: FEAT-8BFC
wave: 2
implementation_mode: direct
complexity: 2
dependencies:
  - TASK-NLTC-001
scaffolding_kind: tests
---

Add unit tests for the notes list view covering the total note count display, its derivation from the list's collection, and its placement above the list.

## The words of the request this task serves

_none — this is a tests task_

## Files to Create

- `apps/web/src/App.test.tsx`

## Files to Modify

- _none_

## Acceptance Criteria

- [ ] A unit test renders the notes list view with a collection of notes and asserts the view displays the total number of notes as "N notes" (e.g. "12 notes" for 12 notes).
- [ ] A unit test asserts the displayed total equals the number of notes in the collection passed to the view (the count is sourced from the same collection that renders the list).
- [ ] A unit test asserts the total number of notes is not displayed as part of any individual note in the list.

## Implementation Notes

- This task tests the surface delivered by TASK-NLTC-001 in `apps/web/src/App.tsx`; run it after that task lands.
- Use the existing web test setup (vitest + React Testing Library, consistent with `apps/web/src/api.test.ts` and `apps/web/src/config.test.ts`).
- Stub `listNotes` to return a fixed collection so the count is deterministic.