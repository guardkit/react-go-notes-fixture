# Feature Spec Summary: PATCH /api/notes/{id} toggles the done flag for the signed-in owner

**Stack**: generic
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 14 total (4 smoke, 0 regression)
**Assumptions**: 10 total (0 high / 0 medium / 10 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the new PATCH /api/notes/{id} endpoint that lets a signed-in owner toggle a note's done flag to true or false, returning the full updated note on success. It pins the owner-scoped lookup that answers an indistinguishable 404 for both nonexistent and foreign notes (preventing existence leakage), the strict boolean validation of the done field, and the rejection of payloads that mutate fields other than done. The stack is undeclared and treated as generic.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 0 |
| Negative cases (@negative) | 6 |
| Edge cases (@edge-case) | 5 |

## Deferred Items

None. All proposed scenarios are included; no groups were deferred.

## Open Assumptions (low confidence)

- ASSUM-001 — Unauthenticated PATCH refused with 401 (vs 403)
- ASSUM-002 — Malformed JSON body refused with 400 (vs 422)
- ASSUM-003 — Non-boolean done value refused with 400 (vs 422)
- ASSUM-004 — Body mutating other fields refused with 400 (vs 422)
- ASSUM-005 — Empty JSON body refused with 400 (vs accepted as no-op)
- ASSUM-006 — Returned note includes id, label, and done (vs timestamps)
- ASSUM-007 — Success status is 200 (vs 204)
- ASSUM-008 — done field is required in the body (vs optional no-op)
- ASSUM-009 — Only done is permitted in the body (vs future extensibility)
- ASSUM-010 — Note id is a string (vs numeric)

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "PATCH /api/notes/{id} toggles the done flag for the signed-in owner" --context features/patch-api-notes-id-toggles-the-done-flag-for-the-signed-in-owner/patch-api-notes-id-toggles-the-done-flag-for-the-signed-in-owner_summary.md

## Verifier routing (proposed)

- "Owner marks a note done and receives the updated note" → hurl
- "Owner marks a note not done and receives the updated note" → hurl
- "Only the done flag is mutated by the update" → hurl
- "Updating a note that does not exist answers 404" → hurl
- "Updating another user's note answers 404" → hurl
- "Missing and foreign notes yield the same refusal" → hurl
- "A non-boolean done value is rejected" → hurl
- "A body attempting to change fields other than done is rejected" → hurl
- "An empty body is rejected" → hurl
- "Setting done to its current value succeeds idempotently" → hurl
- "The toggled done flag persists for a later read" → toolchain
- "A rejected payload does not partially mutate the note" → toolchain
- "An unauthenticated update is refused" → hurl
- "A malformed JSON body is refused" → hurl