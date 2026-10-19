---
id: TASK-3351-002
title: "Verify the existing test suite passes under vitest 4.1.11"
task_type: testing
parent_review: TASK-REV-3351
feature_id: FEAT-3351
wave: 2
implementation_mode: direct
complexity: 2
dependencies:
  - TASK-3351-001
---

Run the existing web app test suite under the upgraded vitest 4.1.11 toolchain and confirm every previously passing test still passes with no new failures or errors introduced by the version change.

## The words of the request this task serves

List lines: 6, 7

The request says: "all existing tests must still pass". This task is the regression gate: execute the full existing vitest suite (the tests in apps/web/src/) under 4.1.11 and confirm zero new failures or errors compared to the pre-upgrade baseline.

## Files to Create

- _none_

## Files to Modify

- _none_

## Acceptance Criteria

- [ ] One check covers "all existing tests must": it finds the members itself when it runs, from apps/web/package.json:10 (`vitest run` discovers all test files in src/), and fails if it finds none.
- [ ] `npm --workspace sample-web run test` (vitest run) exits 0 with all tests passing under vitest 4.1.11.
- [ ] `npm --workspace sample-web run coverage` (vitest run --coverage) exits 0, confirming the @vitest/coverage-v8 4.1.11 integration works.
- [ ] No previously passing test fails after the upgrade (the set of tests that passed under 4.0.15 all still pass under 4.1.11).
- [ ] No new failures or errors are introduced by the version change.
- [ ] The existing test files (apps/web/src/App.test.tsx, apps/web/src/api.test.ts, apps/web/src/config.test.ts) are not modified.

## Implementation Notes

- This task creates no new files and modifies no files. Its deliverable is the verified green run.
- Run `npm --workspace sample-web run test` from the repository root. This invokes `vitest run` per apps/web/package.json:10.
- Run `npm --workspace sample-web run coverage` to exercise the @vitest/coverage-v8 4.1.11 path (scripts/coverage.sh:14 calls this).
- If any test fails, the failure is a regression caused by the version change and must be reported — do NOT edit the test to make it pass.
- The existing test suite is the baseline: apps/web/src/App.test.tsx, apps/web/src/api.test.ts, apps/web/src/config.test.ts. These files must remain untouched.
- The project's aggregate check (`bash scripts/check.sh`) also runs the web tests; running it confirms the upgrade integrates with the full native check pipeline.
- The coder must search the whole repository for further test files under apps/web/src/ before writing the check, and add any found to the verification. The vitest runner discovers all *.test.ts and *.test.tsx files in src/ automatically.