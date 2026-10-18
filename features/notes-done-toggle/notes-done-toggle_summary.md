# Feature Spec Summary: Notes Done-Toggle (Web App)

**Stack**: generic (stack undeclared in the request; recorded as generic rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 13 total (4 smoke, 0 regression)
**Assumptions**: 6 total (0 high / 0 medium / 6 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the in-list completion control on the web app's notes list: a tick control per note row that flips a note's done/not-done state, saves it through the existing PATCH /api/notes/{id} endpoint carrying the done flag, reverts and signals failure when the save does not succeed, and renders the persisted state after a reload from the backend's stored note data. It spans the three features of the slice (FEAT-PO-001 tick control, FEAT-PO-002 persistence, FEAT-PO-003 reload) as one vertical slice. The backend PATCH endpoint and the notes list read path are treated as existing surfaces; their exact payload shapes are carried as low-confidence assumptions rather than invented.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 7 |
| Boundary conditions (@boundary) | 2 |
| Negative cases (@negative) | 3 |
| Edge cases (@edge-case) | 1 |

## Deferred Items

None. No scenario groups were deferred; all proposed scenarios are included.

## Open Assumptions (low confidence)

- ASSUM-001 — done state represented as a boolean done flag (field name/type not stated in the request)
- ASSUM-002 — done note visually distinguished by struck-through or dimmed title
- ASSUM-003 — optimistic tick updating reconciled to the server-confirmed state
- ASSUM-004 — failed save indicated by an inline failure message on the note row
- ASSUM-005 — notes list read path returns the done flag for each note
- ASSUM-006 — PATCH /api/notes/{id} body carries the done flag alone and is accepted

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes Done-Toggle (Web App)" --context features/notes-done-toggle/notes_done_toggle_summary.md

## Verifier routing (proposed)

- "Ticking a note in the list marks it as done" → playwright
- "Unticking a done note in the list returns it to not done" → playwright
- "The completion control is operable without opening the note detail view" → playwright
- "Ticking a note saves the done state through PATCH /api/notes/{id}" → hurl
- "Unticking a note saves the not-done state through PATCH /api/notes/{id}" → hurl
- "A failed save reverts the tick control and shows a failure indication" → playwright
- "A note ticked as done appears ticked after a reload" → playwright
- "A note returned to not done appears unticked after a reload" → playwright
- "The completion control accepts only the two binary states" → toolchain
- "Re-applying the same done flag answers the same note" → hurl
- "Saving a note that does not exist answers one indistinguishable not-found" → hurl
- "Saving the done flag leaves the note's other fields untouched" → hurl
- "An unauthenticated save is challenged rather applied" → hurl
