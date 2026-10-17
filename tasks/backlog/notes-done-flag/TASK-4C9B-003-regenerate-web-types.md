---
id: TASK-4C9B-003
title: "Regenerate the web types from the updated contract"
task_type: feature
parent_review: TASK-REV-4C9B
feature_id: FEAT-4C9B
wave: 3
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-4C9B-002
---

Regenerate `apps/web/src/api-types.ts` from the updated contract via the existing codegen step, so the frontend `Note` type carries a `done` boolean property and existing web code type-checks against it.

## The words of the request this task serves

List lines: 4

> "and the generated web types updated to match"

The web types are generated from the contract by the existing codegen step and committed (ASSUM-002); this task runs that step so the regenerated `Note` type carries `done` and the web consumers of `GET /api/notes` type-check against it.

## Files to Create

- _none_

## Files to Modify

- `apps/web/src/api-types.ts`

## Acceptance Criteria

- `apps/web/src/api-types.ts` is regenerated from `contract/openapi.yaml` by the existing codegen step (`npm run contract:generate`, driven by `tools/contract/generate.mjs`) and committed — not hand-edited (ASSUM-002).
- The regenerated `Note` type carries a `done` boolean property, so the completion state is available to web code that reads a listed note from `GET /api/notes`.
- The regenerated types type-check against existing web code that consumes `GET /api/notes` responses: `apps/web/src/api.ts` (`listNotes`), `apps/web/src/App.tsx`, and their tests compile under strict TypeScript with no changes beyond the regenerated file.
- `npm run contract:check` passes (regeneration produces no drift against the committed file).
- All modified files pass project-configured lint/format checks with zero errors (`tsc`, ESLint, Prettier via `scripts/check.sh`).

## Implementation Notes

- The codegen step is `npm run contract:generate` (`README.md:34`); the tool lives at `tools/contract/generate.mjs`. Run it and commit the regenerated `apps/web/src/api-types.ts`.
- `apps/web/src/api-types.ts` is generated — never hand-edit it; the drift check (`npm run contract:check`) fails on divergence between the committed file and a fresh regeneration.
- Existing web consumers to keep compiling: `apps/web/src/api.ts` (`listNotes` at the `/api/notes` fetch, `apps/web/src/api.ts:60` area) and `apps/web/src/App.tsx` (the signed-in notes view). The additive `done` property must not break them; if a consumer destructures note fields exhaustively, adjust only what the additive change requires.
- Run the existing web checks first (`npm run typecheck`, `npm run lint`, the web unit tests) before and after regeneration to confirm the only delta is the regenerated file.