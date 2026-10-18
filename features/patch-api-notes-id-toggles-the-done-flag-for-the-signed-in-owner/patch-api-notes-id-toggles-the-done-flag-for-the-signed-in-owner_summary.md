# Feature Spec Summary: PATCH /api/notes/{id} toggles the done flag for the signed-in owner

**Stack**: generic
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 7 total (4 smoke, 0 regression)
**Assumptions**: 3 total (0 high / 0 medium / 3 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the new PATCH /api/notes/{id} endpoint that lets a signed-in owner toggle a note's done flag to true or false, returning the full updated note on success. It pins the owner-scoped lookup that answers an indistinguishable 404 for both nonexistent and foreign notes (preventing existence leakage) and the partial update that mutates only the done flag while leaving other attributes untouched. The stack is undeclared and treated as generic.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 0 |
| Negative cases (@negative) | 2 |
| Edge cases (@edge-case) | 2 |

## Deferred Items

None. All proposed scenarios are included; no groups were deferred.

## Open Assumptions (low confidence)

- ASSUM-001 — Returned note includes id, label, and done (vs timestamps)
- ASSUM-002 — Success status is 200 (vs 204)
- ASSUM-003 — Note id is a string (vs numeric)

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "PATCH /api/notes/{id} toggles the done flag for the signed-in owner" --context features/patch-api-notes-id-toggles-the-done-flag-for-the-signed-in-owner/patch-api-notes-id-toggles-the-done-flag-for-the-signed-in-owner_summary.md

## Verifier routing (proposed)

- "Owner marks a note done and receives the updated note" → hurl
- "Owner marks a note not done and receives the updated note" → hurl
- "Only the done flag is mutated by the update" → hurl
- "Updating a note that does not exist answers 404" → hurl
- "Updating another user's note answers 404" → hurl
- "Setting done to its current value succeeds idempotently" → hurl
- "The toggled done flag persists for a later read" → toolchain