---
id: TASK-8037-002
title: Verify done-flag visibility at the browser surface
task_type: testing
parent_review: TASK-REV-8037
feature_id: FEAT-8037
wave: 2
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-8037-001
---

Add the browser-level (playwright) checks that prove the done flag is visible in the notes list at the surface the person uses: done rows carry the "Done" label and differentiated styling, not-done rows do not, and the distinction survives a single scan.

## The words of the request this task serves

List lines: 1, 2

This task verifies, at the browser surface the person actually uses, the same request the building task delivers: "Show each note's done flag in the web app's notes list, with done notes clearly marked as done." It exercises the rows TASK-8037-001 renders.

## Files to Create

- `tests/e2e/specs/notes-done-visibility.spec.ts`

## Files to Modify

- `tests/e2e/specs/helpers.ts`

## Acceptance Criteria

- [ ] A browser test loads the notes list with a done note and a not-done note and asserts the done row displays a "Done" label and is styled differently from the not-done row.
- [ ] A browser test asserts the not-done row displays no "Done" label and retains the standard row styling.
- [ ] A browser test asserts the done treatment never appears on a not-done row and the non-color "Done" cue is present only on done rows.
- [ ] A browser test asserts a list of only not-done notes shows every row in the not-done presentation with no row hidden or omitted.
- [ ] The new spec reuses the existing e2e helpers (`tests/e2e/specs/helpers.ts`) for sign-in and note creation, and follows the `chromium-notes-done` project pattern already declared in `tests/e2e/playwright.config.ts`.
- [ ] The existing `tests/e2e/specs/notes-done.spec.ts` (which asserts the flag at the `GET /api/notes` boundary) is kept and still passes; this task adds the UI-surface assertions on top of it rather than replacing it.

## Implementation Notes

- This task builds on the surface TASK-8037-001 delivers; it must not run before that task's rendering exists.
- Place the new spec under `tests/e2e/specs/` so it is picked up by the existing playwright config and the declared `tests/e2e` test root.
- Use the existing `listitem` role assertions (as in `tests/e2e/specs/notes-done.spec.ts:76`) and assert on the "Done" label text plus the differentiated row class/style introduced by TASK-8037-001.
- Keep the test hermetic: create the done/not-done notes through the app's own flow or the seeded fixtures the existing specs use; do not depend on ambient environment values.