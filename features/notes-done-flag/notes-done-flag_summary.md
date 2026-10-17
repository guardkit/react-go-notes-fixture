# Feature Spec Summary: Notes Done Flag

**Stack**: generic (stack undeclared in the request; noted explicitly rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 14 total (2 smoke, 0 regression)
**Assumptions**: 4 total (0 high / 0 medium / 4 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification pins the vertical slice that adds a boolean `done` flag to notes, expressed entirely through the one interface the request and context supply: the authenticated `GET /api/notes` listing. The migration that introduces the `done` column with a false default (backfilling existing rows to false) is pinned by its observable consequence — a pre-existing note comes back with `done: false` — rather than by a storage assertion; the listing exposes each note's `done` flag alongside its existing `id`, `label`, and `createdAt` fields, and the contract and generated web types are pinned through that same observable listing and the web code that reads it. The slice is read-only — no write path to set `done` is specified. The stack is undeclared (`generic`); the supplied repository evidence indicates a Go `net/http` API with `GET /api/notes`, an OpenAPI contract at `contract/openapi.yaml`, and TypeScript web types generated via `npm run contract:generate`, but no stack was declared in the request.

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

- ASSUM-001 — contract is OpenAPI declaring done as a required boolean property
- ASSUM-002 — web types are regenerated from the contract by the existing codegen step, not hand-edited
- ASSUM-003 — slice is read-only; setting done to true is out of scope
- ASSUM-004 — pre-migration notes are observable only through the listing; column-default behaviour pinned via the listing rather than a storage probe

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes Done Flag" --context features/notes-done-flag/notes-done-flag_summary.md

## Verifier routing (proposed)

- "Migration adds the done column defaulting to not-done" → hurl
- "Listing notes returns the done flag for a not-done note" → hurl
- "Listing notes returns the done flag for a done note" → hurl
- "The done flag is always present and never null in the listing" → hurl
- "Adding the done flag does not remove existing note fields" → hurl
- "Existing notes are backfilled to not-done by the migration" → hurl
- "The contract declares the done flag matching the implemented response" → hurl
- "Regenerated web types carry the done flag" → hurl
- "Regenerated web types type-check against existing web consumers" → hurl
- "A note object without a done flag is not accepted by the contract" → hurl
- "A note object with a null done flag is not accepted by the contract" → hurl
- "Contract validation fails when the response drifts from the contract" → hurl
- "A listing of many notes returns each note's own done flag" → hurl
- "The persistence mapping exposes the done flag to the application layer" → hurl
