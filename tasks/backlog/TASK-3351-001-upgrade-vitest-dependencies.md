---
id: TASK-3351-001
title: "Upgrade vitest and @vitest/coverage-v8 to 4.1.11 in apps/web manifest and lockfile"
task_type: feature
parent_review: TASK-REV-3351
feature_id: FEAT-3351
wave: 1
implementation_mode: direct
complexity: 3
dependencies: []
---

Upgrade the two test-tool devDependencies in apps/web/package.json from 4.0.15 to exactly 4.1.11 and regenerate the root package-lock.json so every resolved entry for vitest and @vitest/coverage-v8 reads 4.1.11 with no 4.0.15 residue.

## The words of the request this task serves

List lines: 1, 2, 3, 4, 5, 6, 7

The request says: "upgrade the test tools vitest and @vitest/coverage-v8 from 4.0.15 to 4.1.11 in apps/web/package.json and package-lock.json, because 4.0.15 has a critical security advisory (GHSA-5xrq-8626-4rwp)". This task is the edit itself: both devDependency entries pinned to exactly 4.1.11 in the manifest, the lockfile regenerated to resolve 4.1.11 everywhere, and the vulnerable 4.0.15 eliminated from the resolved tree so the audit no longer reports the advisory.

## Files to Create

- _none_

## Files to Modify

- `apps/web/package.json`
- `package-lock.json`

## Acceptance Criteria

- [ ] The devDependency entry for vitest in apps/web/package.json reads exactly "4.1.11" (not a caret range, not a higher 4.x).
- [ ] The devDependency entry for @vitest/coverage-v8 in apps/web/package.json reads exactly "4.1.11" (not a caret range, not a higher 4.x).
- [ ] The root package-lock.json resolves vitest at version 4.1.11 (the "version" field of the node_modules/vitest entry is "4.1.11").
- [ ] The root package-lock.json resolves @vitest/coverage-v8 at version 4.1.11 (the "version" field of the node_modules/@vitest/coverage-v8 entry is "4.1.11").
- [ ] No resolved entry for vitest or @vitest/coverage-v8 in package-lock.json remains at 4.0.15 (grep for "4.0.15" in the lockfile returns no vitest or coverage-v8 entries).
- [ ] Neither tool is installed at a version higher than 4.1.11.
- [ ] A security audit against the web app dependency tree does not report advisory GHSA-5xrq-8626-4rwp against vitest or @vitest/coverage-v8.
- [ ] `npm ci --ignore-scripts --no-audit --no-fund` succeeds (the lockfile and manifest are consistent; scripts/install.sh's hash guard is satisfied).
- [ ] All modified files pass project-configured lint/format checks with zero errors.
- [ ] The existing test suite passes under the upgraded test tools: `npm --workspace sample-web run test` (vitest run) exits 0 with all tests passing under vitest 4.1.11.
- [ ] No previously passing test fails after the upgrade (the set of tests that passed under 4.0.15 all still pass under 4.1.11).

## Implementation Notes

- Edit apps/web/package.json lines 22 and 29: change `"@vitest/coverage-v8": "4.0.15"` and `"vitest": "4.0.15"` to `"4.1.11"` (exact pins, no caret).
- After editing the manifest, run `npm install --package-lock-only` (or `npm install`) at the repository root to regenerate package-lock.json. The workspace root is the repo root (package.json workspaces include apps/web).
- The lockfile currently pins @vitest/coverage-v8 at 4.0.15 at package-lock.json:2058-2061 and vitest at 4.0.15 elsewhere in the lockfile; both must update to 4.1.11.
- scripts/install.sh (line 9) hashes package-lock.json before `npm ci` and refuses a mismatch afterward, so the lockfile must be fully consistent with the manifest before committing.
- Do NOT change any other dependency version. Only vitest and @vitest/coverage-v8 move.
- The existing test suite (apps/web/src/*.test.ts, apps/web/src/*.test.tsx) must not be edited; this task changes only the two dependency files.
- After the install, verify with `npm ls vitest @vitest/coverage-v8 --workspace sample-web` that both resolve to exactly 4.1.11.