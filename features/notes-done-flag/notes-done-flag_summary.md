# Feature Spec Summary: Notes Done Flag

**Stack**: generic (no stack signal supplied; `stack` parameter unset, so `generic` is used rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 8 total (2 smoke, 1 regression)
**Assumptions**: 5 total (0 high / 0 medium / 5 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification pins the addition of a `done` boolean to notes, expressed only as externally observable behaviour. A new migration is the prerequisite that lands first; every feature's observable outcome is asserted through the `GET /api/notes` read surface (the `done` field appears on every note object, is a true-or-false value, is never omitted when false, and existing fields plus the endpoint path are unchanged), so the migration is verified by the state it produces in the notes list rather than by inspecting the database schema. The contract is exercised through its observable effect — a client generated from it treats `done` as always present — rather than by inspecting the OpenAPI definition. The cross-layer regression asserts that the response keeps the note fields the client already consumes while adding `done`. It is read-only with respect to the flag (no client-facing write path is in scope). Internal implementation details (column nullability, database-level defaults, schema inspection, codegen steps) are deliberately excluded because they are not observable through the interface and therefore could not be proven as written. The open questions in the input (write path, single-note reads, filtering/sorting, migration strategy, type-commit policy) are carried as low-confidence assumptions rather than invented requirements.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 5 |
| Boundary conditions (@boundary) | 2 |
| Negative cases (@negative) | 1 |
| Edge cases (@edge-case) | 1 |

## Deferred Items

None. No scenario groups were deferred; every proposed scenario is included.

## Open Assumptions (low confidence)

All 5 assumptions are low confidence and unconfirmed (headless `--auto` mode). They need human verification before acceptance:

- ASSUM-001 — Notes are persisted in a relational database addressed by a single notes table.
- ASSUM-002 — The notes list endpoint GET /api/notes already exists and returns a collection of note objects.
- ASSUM-003 — done is a two-valued boolean completion state, not a richer lifecycle.
- ASSUM-004 — The change is read-only with respect to the flag (no write path in scope).
- ASSUM-005 — Adding a response field is backward compatible for existing consumers.

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes Done Flag" --context features/notes-done-flag/notes-done-flag_summary.md

## Verifier routing (proposed)

Advisory only; `/feature-plan` writes the authoritative per-scenario stamp map. Titles are quoted verbatim from the `.feature` file.

- "The notes list reports done false for all notes after the migration" → hurl
- "The notes list reports the done flag for every note" → hurl
- "A note that has never been marked complete is returned with done false" → hurl
- "Existing note fields and the endpoint path are unchanged" → hurl
- "A client built from the contract treats done as always present" → toolchain
- "A note marked complete is returned with done true" → hurl
- "The notes list never returns a non-boolean done value" → hurl
- "The notes list returns note fields the existing client consumes" → hurl