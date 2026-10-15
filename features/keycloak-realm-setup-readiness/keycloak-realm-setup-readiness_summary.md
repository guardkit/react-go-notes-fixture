# Feature Spec Summary: Keycloak Realm Setup Readiness

**Stack**: generic
**Generated**: 2026-08-14T00:00:00Z
**Scenarios**: 14 total (1 smoke, 0 regression)
**Assumptions**: 10 total (0 high / 0 medium / 10 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification pins the bounded readiness wait added to the Keycloak realm setup path so that app start-up, the checks target, the local stack and every deploy stop failing on a Keycloak that reports itself healthy slightly before its admin login is usable. It covers retrying the admin login and the first admin call only on HTTP 503 Service Unavailable or a refused connection under a single 2-minute total wall-clock budget, failing immediately on any other outcome such as a wrong password, and keeping the admin password off every command line and message. The stack is undeclared and defaults to `generic`; no codebase scan or fleet-memory retrieval was performed in this headless run, so all inferred values are recorded as deferred low-confidence assumptions.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 4 |
| Boundary conditions (@boundary) | 4 |
| Negative cases (@negative) | 5 |
| Edge cases (@edge-case) | 3 |

## Deferred Items

None. All proposed scenarios were accepted in this headless run; no group was deferred.

## Open Assumptions (low confidence)

- ASSUM-001 — The readiness wait covers only the admin login and the first admin call; subsequent realm-configuration calls do not need their own readiness wait
- ASSUM-002 — The retry uses a back-off interval and growth policy within the 2-minute budget
- ASSUM-003 — The 2-minute budget is a single total wall-clock limit across all attempts, not a per-attempt timeout
- ASSUM-004 — Only HTTP 503 and a refused connection are retryable; every other HTTP status and every other transport error is terminal
- ASSUM-005 — The admin password is passed to the login request through a channel that never places it on a command line
- ASSUM-006 — The behaviour is tested against a stand-in Keycloak (stub endpoint or command shim) rather than a real instance
- ASSUM-007 — The HTTP client can distinguish an HTTP 503 response from a connection-refused failure and surface the status without the password appearing in the request line
- ASSUM-008 — The readiness wait is implemented inside the realm setup path so it applies uniformly to the checks, the local stack, and every deploy
- ASSUM-009 — The admin password is currently reachable by the login invocation in a form that could appear in argv or in output, and the fix removes that exposure
- ASSUM-010 — Failing the realm setup with a non-zero exit status is sufficient to stop the whole start-up sequence in the checks, local stack and deploy contexts

## Verifier routing (proposed)

- "Realm setup succeeds after Keycloak answers 503 a few times before it is ready" → toolchain
- "Realm setup succeeds after a refused connection before Keycloak is ready" → toolchain
- "Realm configuration content and ordering are unchanged once the admin endpoint answers" → toolchain
- "The admin password is never placed on a command line" → toolchain
- "A 503 response just inside the retryable set is retried" → toolchain
- "A 500 response just outside the retryable set is not retried" → toolchain
- "A Keycloak that becomes ready just inside the 2-minute budget is accepted" → toolchain
- "A Keycloak that never becomes ready is given up on within the 2-minute budget" → toolchain
- "A wrong admin password fails straight away with no retry" → toolchain
- "A non-transient transport failure fails straight away with no retry" → toolchain
- "The admin password is absent from all output of a failed run" → toolchain
- "A Keycloak that never becomes ready by refusing the connection is given up on within the budget" → toolchain
- "A non-transient failure is distinguishable from the not-ready-in-time timeout" → toolchain
- "The readiness wait applies to the checks, the local stack and every deploy" → toolchain

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Keycloak Realm Setup Readiness" --context features/keycloak-realm-setup-readiness/keycloak-realm-setup-readiness_summary.md
