---
id: TASK-9018-003
title: Update the contract and regenerate the web types
task_type: declarative
parent_review: TASK-REV-9018
feature_id: FEAT-9018
wave: 3
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-9018-002
---

Declare `done` as a required boolean on the note schema in `contract/openapi.yaml`, then regenerate `apps/web/src/api-types.ts` from it so a client built from the contract treats `done` as always present.

## The words of the request this task serves

List lines: 3

"with the contract and the generated web types updated to match" — this task updates the contract and regenerates the committed web types from it. It also carries the scenario that a client generated from the updated contract exposes `done` as a field that is always present.

## Files to Create

- _none_

## Files to Modify

- `contract/openapi.yaml`
- `apps/web/src/api-types.ts`

## Acceptance Criteria

- [ ] `contract/openapi.yaml` declares `done` on the note schema as a boolean that is **required** (present in the schema's `required` list), so a generated client treats it as always present.
- [ ] `apps/web/src/api-types.ts` is regenerated from the updated contract via `npm run contract:generate` and committed, so its `Note` type carries `done` as a non-optional boolean.
- [ ] `npm run contract:check` passes (no drift between the committed `api-types.ts` and the contract).
- [ ] The contract remains valid OpenAPI (the `contract_test.go` loader's `doc.Validate` still passes).
- [ ] All modified files pass project-configured lint/format checks with zero errors: `gofmt` and `go vet` (Go), `npm run lint` (ESLint) and Prettier (web), and `npm run contract:check` (contract drift) — the aggregate `bash scripts/check.sh` must exit 0.

## Implementation Notes

- `contract/openapi.yaml` is the source of the notes API contract; `apps/web/src/api-types.ts` is generated from it by `tools/contract/generate.mjs` (`npm run contract:generate`) and committed (README:34). Never hand-edit `api-types.ts` — regenerate it.
- This task depends on TASK-9018-002: the contract must describe the `done` field the read surface actually emits, so the contract and the handler land in agreement.
- Mark `done` required rather than optional: the scenarios pin that the field is never omitted when `false`, and the "client built from the contract treats done as always present" scenario requires the generated type to expose it unconditionally.
- The forbidden-response validator in `apps/api/internal/notes/contract_test.go` (updated in TASK-9018-002) loads this contract; a `GET /api/notes` response missing `done` must be refused once `done` is required here.
- Run the project's own checks before finishing: `gofmt -l .` and `go vet ./...` in `apps/api`, `npm run lint` and Prettier for the web workspace, and `npm run contract:check` at the repo root (the aggregate `bash scripts/check.sh`).