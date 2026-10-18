# Feature Spec Summary: Notes Done-Flag API

**Stack**: generic
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 11 total (2 smoke, 0 regression)
**Assumptions**: 6 total (0 high / 0 medium / 6 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the PATCH /api/notes/{id} endpoint that lets a signed-in owner set a note's done flag to true or false, returning the updated note on success. It pins the ownership-scoped 404 rule (missing and foreign notes are indistinguishable), the boolean-only constraint on the done flag, idempotency of repeated updates, and the refusal of unauthenticated requests. The stack is undeclared and treated as generic; the supplied repository code is a Go HTTP handler with a bearer-token authenticator and string note ids, which informs the observable interface facts used in the steps.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 2 |
| Negative cases (@negative) | 5 |
| Edge cases (@edge-case) | 2 |

## Deferred Items

None. All groups were proposed and accepted in headless mode.

## Open Assumptions (low confidence)

- ASSUM-001: A non-boolean done value in the request body is rejected with a 400 validation error
- ASSUM-002: An unauthenticated PATCH request is answered with 401 sign-in required
- ASSUM-003: A syntactically malformed {id} is answered with 404 rather than a validation error
- ASSUM-004: The updated-note response includes at minimum the note's id and done state
- ASSUM-005: A successful update returns HTTP 200 with the updated note as the JSON response body
- ASSUM-006: Note ids are opaque strings; no specific format (integer, UUID) is enforced

## Verifier routing (proposed)

- "Owner sets a note's done flag to true" → hurl
- "Owner sets a note's done flag to false" → hurl
- "Re-sending the same done value yields the same updated note" → hurl
- "The done flag accepts each boolean value" → hurl
- "A non-boolean done value is rejected" → hurl
- "A note id that does not exist is answered with 404" → hurl
- "A note belonging to another user is answered with 404" → hurl
- "The missing-note and foreign-note responses are indistinguishable" → hurl
- "An unauthenticated request is refused before any note mutation" → hurl
- "A syntactically malformed note id is answered with 404" → hurl
- "The updated note representation carries its identity and new done state" → hurl

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes Done-Flag API" --context features/notes-done-flag-api/notes-done-flag-api_summary.md
