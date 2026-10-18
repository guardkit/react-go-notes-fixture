---
id: TASK-4384-003
title: Document PATCH /api/notes/{id} in the OpenAPI contract
task_type: declarative
parent_review: TASK-REV-4384
feature_id: FEAT-4384
wave: 3
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-4384-002
---

Document the new PATCH /api/notes/{id} operation in the OpenAPI contract so the contract validator accepts the handler's responses and the generated web types carry the toggle.

## The words of the request this task serves

List lines: 1, 3, 4, 5

This task records the delivered surface — "Add PATCH /api/notes/{id} ... it returns the updated note, and answers 404 for a note that does not exist or belongs to someone else" — in the contract the validator and the web client are generated from.

## Files to Create

- _none_

## Files to Modify

- `contract/openapi.yaml`
- `apps/web/src/api-types.ts`

## Acceptance Criteria

- [ ] `contract/openapi.yaml` declares a PATCH operation on `/api/notes/{id}` with a bearer security requirement.
- [ ] The operation's request body is an object carrying only the `done` boolean (the write slice stays label-only; `NewNote` is unchanged).
- [ ] The operation declares a 200 response returning the full updated note (id, label, created_at, done) and a 404 response whose body does not distinguish a missing note from a foreign note.
- [ ] The operation declares the 401 sign-in-required response consistent with the existing operations.
- [ ] The contract validator (`apps/api/internal/notes/contract_test.go`) accepts the handler's actual responses for the new route, and `TestContractKeepsTheDoneFlagOffTheWritePathAndTheEnvelope` still passes (the `NewNote` and `NoteList` schemas are unchanged).
- [ ] `apps/web/src/api-types.ts` is regenerated from the updated contract so the generated types include the PATCH operation.

## Implementation Notes

- Regenerate `apps/web/src/api-types.ts` from `contract/openapi.yaml` using the existing generator (`tools/contract/generate.mjs`) rather than hand-editing the generated file.
- The 404 response schema must be the same shape for both the missing and foreign cases so the contract itself cannot express an existence leak.
- Do not add a `done` property to the `NewNote` component or a `done` field to the `NoteList` envelope — those are pinned by `apps/api/internal/notes/contract_done_test.go:72`.