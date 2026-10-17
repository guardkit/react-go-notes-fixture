# Feature Spec Summary: PATCH /api/notes/{id} sets the done flag for the signed-in owner

**Stack**: generic
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 6 total (2 smoke, 0 regression)
**Assumptions**: 3 total (0 high / 0 medium / 3 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the single capability requested: PATCH /api/notes/{id} lets the signed-in owner set a note's done flag to true or false, returns the full updated note so the client can refresh without a follow-up GET, and answers 404 uniformly for a note that does not exist or belongs to someone else. The done flag is the sole mutable field; other note fields must remain unchanged. Authentication and note storage are treated as pre-existing platform capabilities, consistent with the supplied code where the signed-in principal is resolved from the bearer token's subject and every store call is scoped to the owner.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 1 |
| Negative cases (@negative) | 2 |
| Edge cases (@edge-case) | 0 |

## Deferred Items

None. All proposed groups were accepted in headless mode.

## Open Assumptions (low confidence)

- ASSUM-001 — The returned updated note includes server-managed fields such as an updated timestamp.
- ASSUM-002 — Toggling the done flag does not modify server-managed fields such as the creation timestamp.
- ASSUM-003 — The 404 response body carries no information revealing whether the note exists.

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "PATCH /api/notes/{id} sets the done flag for the signed-in owner" --context features/patch-api-notes-id-sets-the-done-flag-for-the-signed-in-owner/patch-api-notes-id-sets-the-done-flag-for-the-signed-in-owner_summary.md

## Verifier routing (proposed)

- "Owner marks a note as done and receives the updated note" → hurl
- "Owner re-opens a completed note and receives the updated note" → hurl
- "Updating the done flag leaves the note's other fields unchanged" → hurl
- "The done flag accepts each valid boolean value" → hurl
- "Updating a note that does not exist answers 404" → hurl
- "Updating a note owned by someone else answers 404" → hurl
