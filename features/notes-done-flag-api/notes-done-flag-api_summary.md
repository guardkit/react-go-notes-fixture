# Feature Spec Summary: Notes Done-Flag API

**Stack**: generic
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 8 total (2 smoke, 0 regression)
**Assumptions**: 4 total (0 high / 0 medium / 4 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the PATCH /api/notes/{id} endpoint that lets a signed-in owner set a note's done flag to true or false, returning the updated note on success. It pins the ownership-scoped 404 rule (missing and foreign notes are indistinguishable), the boolean-only constraint on the done flag, idempotency of repeated updates, and the refusal of unauthenticated requests. The stack is undeclared and treated as generic; the supplied repository code is a Go HTTP handler with a bearer-token authenticator and string note ids, which informs the observable interface facts used in the steps.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 1 |
| Negative cases (@negative) | 3 |
| Edge cases (@edge-case) | 1 |

## Deferred Items

None. All groups were proposed and accepted in headless mode.

## Open Assumptions (low confidence)

- ASSUM-001: An unauthenticated PATCH request is answered with 401 sign-in required
- ASSUM-002: The updated-note response includes at minimum the note's id and done state
- ASSUM-003: A successful update returns HTTP 200 with the updated note as the JSON response body
- ASSUM-004: Note ids are opaque strings; no specific format (integer, UUID) is enforced

## Verifier routing (proposed)

- "Owner sets a note's done flag to true" → hurl
- "Owner sets a note's done flag to false" → hurl
- "Re-sending the same done value yields the same updated note" → hurl
- "The done flag accepts each boolean value" → hurl
- "A note id that does not exist is answered with 404" → hurl
- "A note belonging to another user is answered with 404" → hurl
- "An unauthenticated request is refused before any note mutation" → hurl
- "The updated note representation carries its identity and new done state" → hurl

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes Done-Flag API" --context features/notes-done-flag-api/notes-done-flag-api_summary.md
