---
id: TASK-8037-001
title: Render the done flag in the notes list
task_type: feature
parent_review: TASK-REV-8037
feature_id: FEAT-8037
wave: 1
implementation_mode: task-work
complexity: 4
dependencies: []
---

Render each note's done flag in the web app's notes list so that a done note's row carries a "Done" label and visually differentiated styling, while a not-done note's row keeps the standard presentation with no "Done" label.

## The words of the request this task serves

List lines: 1, 2

This task exists to satisfy "Show each note's done flag in the web app's notes list, with done notes clearly marked as done." It owns the delivered surface — the notes list rows the person sees — for every approved scenario: the per-row "Done" label, the differentiated done styling, the not-done default (including unset/null treated as not-done), the never-leak rule, and the unchanged title/ordering/total.

## Files to Create

- `apps/web/src/done-flag.test.tsx`

## Files to Modify

- `apps/web/src/App.tsx`

## Acceptance Criteria

- [ ] A note whose `done` is `true` renders a row containing a "Done" textual label and a row class/style that differs from a not-done row (the ASSUM-003 treatment: textual label plus differentiated styling).
- [ ] A note whose `done` is `false` renders no "Done" label and retains the standard row styling.
- [ ] A note whose `done` is absent/null is treated as not-done and rendered in the not-done presentation (no "Done" label, standard styling) — never hidden or omitted.
- [ ] The done treatment never leaks onto a not-done row: a not-done row is not styled differently from another not-done row and carries no "Done" label.
- [ ] The non-color textual cue ("Done") is present on every done row and absent from every not-done row, so the two states stay mutually distinguishable without relying on colour alone.
- [ ] One check covers "each note": it finds the members itself when it runs, from `contract/openapi.yaml`, and fails if it finds none.
- [ ] Showing the done flag does not alter other list behaviour: each note's title text is unchanged, notes appear in the same order as before, and the "N notes" total above the list is unchanged.
- [ ] A list containing only not-done notes shows every row in the not-done presentation with no row hidden or omitted.
- [ ] The existing done-flag delivery is kept and asserted at the surface: the row reflects the `done` value the `GET /api/notes` response carries (the existing proof at `apps/api/internal/notes/done_test.go:58` and `tests/e2e/specs/notes-done.spec.ts:15`), so the UI change does not depend on any API change.
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- The done flag already arrives on every note: the contract declares it a required boolean (`contract/openapi.yaml`, `Note.done`), the Go handler answers it (`apps/api/internal/notes/handler.go:22`), the store reads it back (`apps/api/internal/notes/store.go`), and the web type carries it (`apps/web/src/api-types.ts`, `apps/web/src/api.ts`). Do not change the API, the contract, or the store — this is a display-layer change only (ASSUM-001).
- Before writing the "each note" check, search the whole repository for further members of the set the request names ("each note") and add any found to the check. The descriptor listed 24 candidate files but only 24 of 53 matching files were listed (`listed_all: false`), so 29 matching files were not shown — search beyond the listed candidates.
- The "each note" check should derive its members from the route/contract source (`contract/openapi.yaml`, the `listNotes` operation and `Note` schema) so a note added later is covered without editing the check.
- Keep the done treatment legible in a single scan (ASSUM-003): pair the "Done" label with a clearly differentiated row style (e.g. a distinct class on the `listitem`), and do not reorder or group done notes (ASSUM-004) — done notes stay in their current list position.
- Treat unset/null `done` as not-done (ASSUM-005); the API already answers `false` for such rows, but the renderer must not crash or show an "unknown" state if the field is missing.
- The web unit suite lives at `apps/web/src/App.test.tsx` (outside the declared `test_roots`); add the new done-flag unit checks in `apps/web/src/done-flag.test.tsx` and run them with the project's web test command. Do not assert that any method raises `NotImplementedError` or that a list is empty for behaviour a later task in this feature implements.
- Run the existing web tests (`apps/web/src/App.test.tsx`) and the existing e2e done-flag spec (`tests/e2e/specs/notes-done.spec.ts`) as part of this task's verification to confirm no regression in title, ordering, or total.