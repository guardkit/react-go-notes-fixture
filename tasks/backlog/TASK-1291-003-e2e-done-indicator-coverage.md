---
id: TASK-1291-003
title: Add end-to-end coverage for the done indicator
task_type: testing
parent_review: TASK-REV-1291
feature_id: FEAT-1291
wave: 3
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-1291-001
  - TASK-1291-002
scaffolding_kind: tests
---

Add Playwright end-to-end coverage that the notes list shows each note's done flag with done notes clearly marked as done.

## The words of the request this task serves

_none — this is a tests task_

## Files to Create

- `tests/e2e/specs/done-indicator.spec.ts`

## Files to Modify

- `tests/e2e/specs/helpers.ts`

## Acceptance Criteria

- [ ] An end-to-end test loads the notes list and asserts each row presents its note's done status without opening the note.
- [ ] An end-to-end test asserts a done note shows the indicator in its done state and an open note shows the not-done state.
- [ ] An end-to-end test asserts a done note is visually distinguished by more than one channel (a "Done" label plus a row-level treatment).
- [ ] An end-to-end test asserts the indicator appears in a consistent position across all rows and that a mixed list renders both states without ambiguity.
- [ ] An end-to-end test asserts viewing the list does not change any note's done flag.
- [ ] An end-to-end test asserts the indicator does not displace or truncate the title, preview, and timestamp.
- [ ] The new tests pass against the implementation delivered by TASK-1291-001 and TASK-1291-002.

## Implementation Notes

- This task only writes tests; it names no web address or capability the request did not name.
- Place the new spec under the declared test root `tests/e2e/specs/` and reuse the existing helpers in `tests/e2e/specs/helpers.ts` (extend them only as needed for the done indicator).
- Run the existing `tests/e2e/specs/journey.spec.ts` and `tests/e2e/specs/smoke.spec.ts` first to confirm the baseline is green before adding the new spec; "run the existing tests first" is a step in this task, not a separate task.
- Do not modify the existing journey or smoke specs unless the request requires it; keep the project's own committed checks green.