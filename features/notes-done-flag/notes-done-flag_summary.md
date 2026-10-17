# Feature Spec Summary: Notes Done Flag

**Stack**: generic (stack undeclared in the request; noted explicitly rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 14 total (2 smoke, 0 regression)
**Assumptions**: 4 total (0 high / 0 medium / 4 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification pins the vertical slice that adds a boolean `done` flag to notes: a migration that introduces the column with a false default (backfilling existing rows to false), the `GET /api/notes` response exposing each note's `done` flag alongside its existing fields, the API contract declaring `done` as a required boolean matching the implemented response, and the web types regenerated from that contract. The slice is read-only — no write path to set `done` is specified. The stack is undeclared (`generic`); the supplied repository evidence indicates a Go `net/http` API, an OpenAPI contract at `contract/openapi.yaml`, and TypeScript web types generated via `npm run contract:generate`, but no stack was declared in the request.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 5 |
| Boundary conditions (@boundary) | 4 |
| Negative cases (@negative) | 3 |
| Edge cases (@edge-case) | 2 |

## Deferred Items

None. No scenario groups were deferred (headless mode proposes all scenarios at once).

## Open Assumptions (low confidence)

- ASSUM-001 — done column is non-nullable boolean with server-side false default (backfill strategy)
- ASSUM-002 — contract is OpenAPI declaring done as a required boolean property
- ASSUM-003 — web types are regenerated from the contract by the existing codegen step, not hand-edited
- ASSUM-004 — slice is read-only; setting done to true is out of scope

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes Done Flag" --context features/notes-done-flag/notes-done-flag_summary.md

## Verifier routing (proposed)

- "Migration adds the done column defaulting to not-done" → toolchain
- "Listing notes returns the done flag for a not-done note" → hurl
- "Listing notes returns the done flag for a done note" → hurl
- "The done flag is always present and never null in the listing" → hurl
- "Adding the done flag does not remove existing note fields" → hurl
- "Existing notes are backfilled to not-done by the migration" → toolchain
- "The contract declares the done flag matching the implemented response" → toolchain
- "Regenerated web types carry the done flag" → toolchain
- "Regenerated web types type-check against existing web consumers" → toolchain
- "A note object without a done flag is not accepted by the contract" → toolchain
- "A note object with a null done flag is not accepted by the contract" → toolchain
- "Contract validation fails when the response drifts from the contract" → toolchain
- "A listing of many notes returns each note's own done flag" → hurl
- "The persistence mapping exposes the done flag to the application layer" → toolchain