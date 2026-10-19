# Feature Spec Summary: Web App Vitest Security Upgrade (4.0.15 → 4.1.11)

**Stack**: generic (no stack signal declared; noted rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 7 total (4 smoke, 0 regression)
**Assumptions**: 3 total (0 high / 0 medium / 3 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the dependency upgrade of `vitest` and `@vitest/coverage-v8` in the web app workspace (`apps/web`) from 4.0.15 to exactly 4.1.11, across both `apps/web/package.json` and the root `package-lock.json`, remediating critical security advisory GHSA-5xrq-8626-4rwp. It also covers the regression gate: the full existing test suite must run green under 4.1.11 with no new failures or errors. No application behaviour changes are in scope; only test tooling versions change.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 4 |
| Boundary conditions (@boundary) | 2 |
| Negative cases (@negative) | 2 |
| Edge cases (@edge-case) | 0 |

## Deferred Items

None. All four groups were proposed and included (headless propose-never-elicit mode).

## Open Assumptions (low confidence)

All 3 assumptions are low confidence and deferred, awaiting the per-assumption approval pass (FEAT-SPL-003):

- ASSUM-001 — Target version pinned exactly at 4.1.11 vs a caret range
- ASSUM-002 — A passing-test baseline under 4.0.15 exists for comparison
- ASSUM-003 — Moving to 4.1.11 is sufficient closure; no interim compensating controls

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Web App Vitest Security Upgrade (4.0.15 → 4.1.11)" --context features/web-app-vitest-security-upgrade-4-0-15-4-1-11/web-app-vitest-security-upgrade-4-0-15-4-1-11_summary.md

## Verifier routing (proposed)

- "Both test tools are pinned to 4.1.11 in the web app manifest" → toolchain
- "The lockfile resolves the upgraded test tools with no lingering 4.0.15 entries" → toolchain
- "The security audit no longer reports the critical advisory against the test tools" → toolchain
- "The existing test suite passes under the upgraded test tools" → toolchain
- "The resolved test tool versions are exactly 4.1.11 and not a higher 4.x release" → toolchain
- "The vulnerable 4.0.15 version does not survive anywhere in the resolved tree" → toolchain
- "No previously passing test fails after the upgrade" → toolchain
