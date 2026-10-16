---
id: TASK-96BB-001
title: "Show the note count in the browser tab title"
task_type: feature
parent_review: TASK-REV-96BB
feature_id: FEAT-96BB
wave: 1
implementation_mode: task-work
complexity: 4
dependencies: []
---

Make the browser tab title of the notes web app show the current note count in the exact format "Notes (N)", kept live with the note list, changing only the web app and its tests.

## The words of the request this task serves

List lines: 1

> "Show the number of notes in the browser tab title, for example \"Notes (3)\". Change only the web app and its tests."

This task is the whole of the requested behaviour: the tab title carries the count on load, stays correct as notes are added and deleted without a reload, and never diverges from the visible list. It is the delivered surface (the browser tab title) for every scenario of the feature. The task also extends the web app's own tests — both the unit tests in `apps/web/src/App.test.tsx` and the end-to-end journey in `tests/e2e/specs/journey.spec.ts` — to assert the tab-title count at the surfaces the person uses.

## Files to Create

- _none_

## Files to Modify

- `apps/web/src/App.tsx`
- `apps/web/src/App.test.tsx`
- `tests/e2e/specs/journey.spec.ts`

## Acceptance Criteria

- [ ] When the web app loads with 3 notes in its state, the browser tab title (`document.title`) is exactly "Notes (3)".
- [ ] When the web app loads with 0 notes in its state, the browser tab title is exactly "Notes (0)".
- [ ] When the user adds a note while the title reads "Notes (2)", the title becomes "Notes (3)" without a page reload.
- [ ] When the user deletes the last note while the title reads "Notes (1)", the title becomes "Notes (0)" and does not still show "Notes (1)".
- [ ] When the user adds a note while 4 notes are shown, the title becomes "Notes (5)" and matches the number of notes shown in the note list.
- [ ] For any note count N, the title is exactly "Notes (N)" — verified for 0, 1, 3 and 100 — with the literal prefix "Notes" and no other text.
- [ ] When the user adds a note and then deletes a note, the title settles on the correct count ("Notes (2)" from a starting count of 2), never an intermediate one.
- [ ] The title is derived from the same client-side note state that renders the note list, so it can never diverge from the visible list.
- [ ] No backend, API, or storage change is made: the count comes from state the web app already holds (the same collection that populates the "N notes" total above the list).
- [ ] The end-to-end journey (`tests/e2e/specs/journey.spec.ts`) asserts the browser tab title is exactly "Notes (N)" at the real browser surface (`page.title()`) for the note counts the journey already exercises, and that the title updates without a page reload when a note is added or deleted.
- [ ] The existing total-above-list assertions (`expectTotalAboveList`, `tests/e2e/specs/journey.spec.ts:46-75`) still pass unchanged.
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- The note count is already in client-side state: `apps/web/src/App.tsx` renders the note list and the "N notes" total above it from the same collection (see the `totalAboveTheList` helper in `apps/web/src/App.test.tsx:438-448` and the e2e `expectTotalAboveList` helper in `tests/e2e/specs/journey.spec.ts:46-75`). Reuse that state; do not add a fetch, endpoint, or storage read to compute the count (ASSUM-001).
- Set the title from a `useEffect` keyed on the notes collection so every add/delete re-renders the title in the same commit as the list — this is what keeps the title from diverging and what makes the rapid add-then-delete settle on the final count rather than an intermediate one.
- Format exactly `Notes (${count})` with the literal prefix "Notes", including `Notes (0)` for the empty state (ASSUM-003). Do not append the count to an existing app-specific title; the title must contain only this format (the @negative scenario).
- `document.title` is the surface (ASSUM-002). The web app's unit tests use a hand-rolled DOM shim in `apps/web/src/App.test.tsx` (the `FElement`/`FDocument` types around lines 56-60 and 225-240); if that shim has no `title` property, extend the shim so the test can assert `document.title` — that is a test-infrastructure change inside the web app's own tests, which the request permits.
- The existing "N notes" total above the list is a different surface and must keep working unchanged; this task adds the tab title alongside it, not in place of it.
- In `tests/e2e/specs/journey.spec.ts`, use Playwright's `page.title()` to assert the exact string `Notes (${expected})` (not a substring) alongside the existing `expectTotalAboveList` calls, so the @negative "no other text" guarantee holds at the browser surface too. Reuse the journey's existing helpers and fixtures; do not add a new spec file or a new test root.
- Run the existing web unit tests (`apps/web/src/App.test.tsx`) and the e2e journey before and after; the change must not turn any existing assertion red.
- Scope guard: touch nothing under `apps/api/`, `contract/`, or `infra/` in this task — the request says "change only the web app and its tests".