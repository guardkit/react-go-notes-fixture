---
complexity: 5
dependencies:
- TASK-9018-001
feature_id: FEAT-9018
id: TASK-9018-002
implementation_mode: task-work
parent_review: TASK-REV-9018
status: design_approved
task_type: feature
title: Return the done flag from GET /api/notes
wave: 2
---

Read the new `done` column through the notes store and surface it on every note object returned by `GET /api/notes`, as a JSON boolean that is always present (never omitted when `false`), while leaving the existing fields and the endpoint path unchanged.

## The words of the request this task serves

List lines: 2

"GET /api/notes returns each note's done flag" — this task is the read surface. It also carries the parts of the request the scenarios pin at that surface: every note carries `done`, it is a boolean, it is `false` for notes never marked complete (including rows predating the migration), it is `true` for a note whose value is `true`, and the existing note fields and the endpoint path are unchanged.

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/store.go`
- `apps/api/internal/notes/handler.go`
- `apps/api/internal/notes/handler_test.go`
- `apps/api/internal/notes/contract_test.go`

## Acceptance Criteria

- [ ] The `Note` struct carries a `Done bool` field whose JSON tag emits `done` and is never omitted when `false` (no `omitempty`).
- [ ] `PGStore.List` selects `done` and populates `Note.Done` from the column added by `apps/api/migrations/0003_add_done.sql` (TASK-9018-001); `GET /api/notes` returns `done: false` for every note created before the migration.
- [ ] A note whose `done` value is `true` is returned with `done: true` (the just-inside bound of the boolean domain).
- [ ] No note in the `GET /api/notes` response carries a `done` value that is not a boolean.
- [ ] The existing note fields (`id`, `label`, `createdAt`) and the endpoint path `GET /api/notes` are unchanged; the change is additive.
- [ ] `TestContractRefusesForbiddenResponses` is updated so a `GET /api/notes` response missing `done` is refused by the contract validator, and a response carrying `done` is accepted — the contract (TASK-9018-003) is the source the validator loads.
- [ ] `handler_test.go` pins the wire shape of the list response to include `done` (extend the existing `TestCreatePreservesContextOwnerAndWireShape`-style exact-body assertions to the list path).
- [ ] One check covers "each note": it finds the members itself when it runs, from `apps/api/internal/notes/handler.go` (the `GET /api/notes` handler at `apps/api/internal/notes/handler.go:46`, which returns the note list via `store.List`), and fails if it finds none.
- [ ] All modified files pass project-configured lint/format checks with zero errors: `gofmt` and `go vet` (Go), `npm run lint` (ESLint) and Prettier (web), and `npm run contract:check` (contract drift) — the aggregate `bash scripts/check.sh` must exit 0.

## Implementation Notes

- The delivered surface is `mux.Handle("GET /api/notes", …)` in `apps/api/internal/notes/handler.go:46`, which calls `store.List(r.Context(), owner)`; the note shape is defined in `apps/api/internal/notes/store.go`. Add the field there and read the column in the `List` query.
- This task depends on TASK-9018-001: the `done` column must exist before `List` can select it. Do not select a column the migration has not added.
- Keep the change read-only with respect to the flag (ASSUM-004): no write path for setting `done` is in scope, so `Create` continues to insert without a `done` value and relies on the column default.
- The contract validator in `apps/api/internal/notes/contract_test.go` loads `contract/openapi.yaml` (the file TASK-9018-003 updates); coordinate the forbidden-response case so the validator bites on a missing `done` once the contract declares it required.
- The web journey in `tests/e2e/specs/journey.spec.ts` and the web view in `apps/web/src/App.tsx` must keep passing unchanged — the added field is additive and the existing client tolerates it (ASSUM-005). Do not change the browser journey to require `done`; the request does not ask the view to render it.
- The "each note" check must iterate the notes the handler returns at run time (the list `store.List` yields for the signed-in owner) rather than a hand-written list of note literals, so every note in the response is covered and an empty result fails the check. Before writing it, search the whole repository for further places that enumerate note objects (the handler, the store, the contract, the generated types, the web client and view) and add any found to the check's coverage.
- Run the project's own checks before finishing: `gofmt -l .` and `go vet ./...` in `apps/api`, `npm run lint` and Prettier for the web workspace, and `npm run contract:check` at the repo root (the aggregate `bash scripts/check.sh`).