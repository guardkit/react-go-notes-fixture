# Feature Spec Summary: Notes Done Flag

**Stack**: generic (no stack signal supplied; `stack` parameter unset, so `generic` is used rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 18 total (3 smoke, 3 regression)
**Assumptions**: 10 total (0 high / 0 medium / 10 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification pins the addition of a `done` boolean to notes across three layers: a new additive, reversible migration that adds a non-nullable `done` column defaulting to false (so pre-existing rows read as not-done without a backfill), the `GET /api/notes` read surface that reports `done` on every note object (never omitted when false), and the contract plus regenerated web types that declare the same required boolean. It is read-only with respect to the flag — no client-facing write path is in scope — and the cross-layer agreement gate (response payload, contract, generated types) is covered as a regression. The input is well specified for the named surfaces; the open questions in the input (write path, single-note reads, filtering/sorting, migration strategy, type-commit policy) are carried as low-confidence assumptions rather than invented requirements.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 10 |
| Boundary conditions (@boundary) | 3 |
| Negative cases (@negative) | 3 |
| Edge cases (@edge-case) | 3 |

## Deferred Items

None. No scenario groups were deferred; every proposed scenario is included.

## Open Assumptions (low confidence)

All 10 assumptions are low confidence and unconfirmed (headless `--auto` mode). They need human verification before acceptance:

- ASSUM-001 — Notes are persisted in a relational database addressed by a single notes table.
- ASSUM-002 — GET /api/notes already exists and returns a collection of note objects.
- ASSUM-003 — Web types are machine-generated from the contract by an existing codegen step.
- ASSUM-004 — done is a two-valued boolean completion state, not a richer lifecycle.
- ASSUM-005 — The change is read-only with respect to the flag (no write path in scope).
- ASSUM-006 — The notes table is small enough for a single straightforward migration.
- ASSUM-007 — Adding a response field is backward compatible for existing consumers.
- ASSUM-008 — Contract and generated web types are versioned and deployed with the API change.
- ASSUM-009 — GET /api/notes requires a valid access token (unauthorised access refused).
- ASSUM-010 — The done flag appears only on GET /api/notes, not on single-note reads.

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes Done Flag" --context features/notes-done-flag/notes-done-flag_summary.md

## Verifier routing (proposed)

Advisory only; `/feature-plan` writes the authoritative per-scenario stamp map. Titles are quoted verbatim from the `.feature` file.

- "Applying the migration adds a done column defaulting to false" → toolchain
- "Pre-existing notes read as not-done after the migration" → toolchain
- "Reverting the migration removes the done column without data loss" → toolchain
- "The migration is a single ordered schema change" → toolchain
- "The notes list reports the done flag for every note" → hurl
- "A note that has never been marked complete is returned with done false" → hurl
- "Existing note fields and the endpoint path are unchanged" → hurl
- "The contract declares done as a required boolean on the note schema" → toolchain
- "The generated web note type carries the done boolean" → toolchain
- "Regenerating from the committed contract yields no diff" → toolchain
- "A note marked complete is returned with done true" → hurl
- "A non-boolean done value is not accepted by the schema" → toolchain
- "An owner with no notes receives an empty list" → hurl
- "An anonymous request to the notes list is refused" → hurl
- "A request with a forged token is refused" → hurl
- "The response payload validates against the updated contract" → toolchain
- "A divergence between response, contract and types fails the check" → toolchain
- "The end-to-end regression covers new and pre-existing notes" → toolchain
