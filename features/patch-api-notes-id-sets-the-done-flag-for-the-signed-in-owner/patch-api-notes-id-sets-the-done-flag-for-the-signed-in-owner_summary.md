# Feature Spec Summary: PATCH /api/notes/{id} sets the done flag for the signed-in owner

**Stack**: generic
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 10 total (2 smoke, 0 regression)
**Assumptions**: 6 total (0 high / 0 medium / 6 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the single capability requested: PATCH /api/notes/{id} lets the signed-in owner set a note's done flag to true or false, returns the full updated note so the client can refresh without a follow-up GET, and answers 404 uniformly for a note that does not exist or belongs to someone else. The done flag is the sole mutable field; other note fields must remain unchanged. Authentication and note storage are treated as pre-existing platform capabilities, consistent with the supplied code where the signed-in principal is resolved from the bearer token's subject and every store call is scoped to the owner.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 2 |
| Negative cases (@negative) | 4 |
| Edge cases (@edge-case) | 2 |

## Deferred Items

None. All proposed groups were accepted in headless mode.

## Open Assumptions (low confidence)

- ASSUM-001 — A malformed or non-boolean done value is rejected with a 400 validation error (400 vs 422 undecided).
- ASSUM-002 — An unauthenticated request is refused with 401 before route handling (401 vs 403 undecided).
- ASSUM-003 — A PATCH to the collection path without an id is refused as invalid (invalid vs 404 undecided).
- ASSUM-004 — The returned updated note includes server-managed fields such as an updated timestamp.
- ASSUM-005 — Toggling the done flag does not modify server-managed fields such as the creation timestamp.
- ASSUM-006 — The 404 response body carries no information revealing whether the note exists.

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "PATCH /api/notes/{id} sets the done flag for the signed-in owner" --context features/patch-api-notes-id-sets-the-done-flag-for-the-signed-in-owner/patch-api-notes-id-sets-the-done-flag-for-the-signed-in-owner_summary.md

## Verifier routing (proposed)

- "Owner marks a note as done and receives the updated note" → hurl
- "Owner re-opens a completed note and receives the updated note" → hurl
- "Updating the done flag leaves the note's other fields unchanged" → hurl
- "The done flag accepts each valid boolean value" → hurl
- "A non-boolean done value is rejected" → hurl
- "Updating a note that does not exist answers 404" → hurl
- "Updating a note owned by someone else answers 404" → hurl
- "A missing note and another user's note are indistinguishable to the caller" → hurl
- "An unauthenticated request is refused before the update is applied" → hurl
- "A PATCH without a note id is refused" → hurl