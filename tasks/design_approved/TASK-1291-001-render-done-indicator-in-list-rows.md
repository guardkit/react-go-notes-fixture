---
complexity: 5
dependencies: []
feature_id: FEAT-1291
id: TASK-1291-001
implementation_mode: task-work
parent_review: TASK-REV-1291
status: design_approved
task_type: feature
title: Render the done indicator in every notes-list row
wave: 1
---

Render each note's done flag in the web app's notes list so the user reads it without opening the note, with done notes clearly marked as done.

## The words of the request this task serves

List lines: 1, 2, 9, 10, 11, 12, 13, 14, 15, 16, 17

The part of the request this task exists to satisfy: "Show each note's done flag in the web app's notes list" — the list shows every row's done status so the user reads it without opening the note; "with done notes clearly marked as done" — a done note shows its indicator in the done state, an open note shows the not-done state, done notes are visually distinguished by more than one channel (check mark + "Done" label + row-level treatment), the indicator sits in a consistent position across all rows, a note with no explicit done value is treated as not done, the row reflects the current stored flag after a refresh, paginated rows carry the status on every page, a mixed list renders both states side by side without ambiguity, viewing the list does not mutate any note's done flag (read-only), the indicator does not displace or truncate existing row content, and the row layout holds at the narrowest supported viewport.

## Files to Create

- _none_

## Files to Modify

- `apps/web/src/App.tsx`

## Acceptance Criteria

- [ ] Every row in the notes list presents that note's done status for the user to read without opening the note.
- [ ] A note whose done flag is set shows the indicator in its done state; a note whose done flag is not set shows the indicator in its not-done state.
- [ ] A done note is visually distinguished from open notes by more than one channel: an explicit done affordance with a "Done" label plus a row-level treatment (muted title weight or strikethrough) that reads differently from an open title.
- [ ] The done indicator appears in the same position relative to the note title on every row.
- [ ] A note with no explicit done value is treated as not done rather than rendering an unknown or broken state.
- [ ] The row's done value reflects the note's current stored flag after a refresh, including flags changed elsewhere since the last load.
- [ ] Paginated rows carry the done status on every page, not just the first.
- [ ] A mixed list renders done and open rows side by side without ambiguity about which row is which.
- [ ] Viewing the list does not change any note's done flag (the list is read-only with respect to the flag).
- [ ] The done indicator does not displace or truncate the existing title, preview, and timestamp.
- [ ] The row layout holds at the narrowest supported viewport, with the done indicator and the rest of the row intact.
- [ ] One check covers "each note": it finds the members itself when it runs, from `contract/openapi.yaml`, and fails if it finds none.
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- The done flag is already on the list payload: the migration `0003_add_done.sql` adds the column, the store returns it on every listed note, the contract declares it, and the generated `Note` type carries it as a required boolean. Read the flag from the existing `Note` object the list already receives from `listNotes` in `apps/web/src/api.ts`; do not add a backend change or a per-note follow-up request.
- The existing web tests already pin the payload contract — `apps/web/src/api.test.ts` "keeps the completion flag of a well-formed note" (line 48 of shown code: `expect(parseNote({ ...note, done: true }).done).toBe(true);`) — so keep that behaviour; do not write a test that contradicts it.
- The existing API tests pin that the listing carries the flag on every note — `apps/api/internal/notes/handler_test.go` `doneFlag` function (line 492 of shown code refuses a listing that omits the flag) — so the backend already returns `done` on each note.
- Render the indicator inside the existing row markup in `apps/web/src/App.tsx` (which calls `listNotes(token).then(...)` at line 24 of the file) so it coexists with the title, preview, and timestamp without displacing them. Keep the indicator in a fixed position (e.g. a leading status cell) so the eye can scan a column of statuses.
- The done treatment combines a check mark plus a "Done" label with a row-level title treatment (muted weight or strikethrough). Absent done value defaults to not-done.
- The list is read-only with respect to the flag: rendering must not mutate or write the done value.
- The coder must search the whole repository for further members of "each note" before writing the set check, and add any found to the check; 43 files matched the phrase, only 24 were listed among candidates, so the candidate list is not the whole set (19 matching files were not listed).
- Run the existing web unit tests (`apps/web/src/App.test.tsx`, `apps/web/src/api.test.ts`) as part of this task's verification; they are the project's own checks for the list and the payload.