---
id: TASK-KRSR-001
title: Add bounded readiness wait to the Keycloak realm setup
task_type: feature
parent_review: TASK-REV-6AC9
feature_id: FEAT-6AC9
wave: 1
implementation_mode: task-work
complexity: 6
dependencies: []
---

Add a bounded readiness wait around the admin login and the first admin call in the realm setup, retrying only on 503 or a refused connection under a 2-minute total budget, failing straight away on any other error, and keeping the admin password off every command line and message.

## The words of the request this task serves

> Make the realm setup wait for Keycloak to be ready: retry the admin login and the first admin call only on 503 or a refused connection, give up after at most 2 minutes in total with a clear message saying Keycloak was not ready in time, fail straight away on any other error such as a wrong password, and never show the admin password on a command line or in any message.

## Files to Create

- `scripts/keycloak/lib/readiness.sh`

## Files to Modify

- `scripts/keycloak/configure-realm.inside.sh`

## Acceptance Criteria

- [ ] The admin login and the first admin call in `scripts/keycloak/configure-realm.inside.sh` retry on HTTP 503 Service Unavailable and on a refused connection, and on nothing else.
- [ ] A 500 Internal Server Error, a 401 Unauthorized (wrong password), or any other non-retryable outcome fails the script with a non-zero exit on the first attempt, reporting the status it received.
- [ ] The retry loop is bounded by a single 2-minute total wall-clock budget across all attempts; when the budget is exhausted the script exits non-zero with a clear message stating that Keycloak was not ready in time, naming the 2-minute budget and the admin endpoint probed.
- [ ] When Keycloak answers 503 (or refuses the connection) a few times and then serves successfully, the realm setup completes with a successful exit status and reports that more than one login attempt was made.
- [ ] The admin password is passed to the login request through a channel that never places it on a command line (the existing stdin transport in `scripts/keycloak-config.sh:36-43` is preserved), and the password never appears in the script's standard output or standard error, including in failure messages.
- [ ] The readiness wait covers only the admin login and the first admin call; subsequent realm-configuration calls are unchanged.
- [ ] The existing `k()` and `kf()` wrappers (`scripts/keycloak/configure-realm.inside.sh:10-11`) and the existing secrets-on-stdin transport (`scripts/keycloak-config.sh:36-43`) are preserved.
- [ ] One check covers "every deploy": it finds the members itself when it runs, from `scripts/check.sh:30`, and fails if it finds none.
- [ ] All modified files pass project-configured lint/format checks with zero errors (the shellcheck sweep in `scripts/check.sh:30` covers `scripts/keycloak/*.sh`).

## Implementation Notes

- Look at the existing login and first-admin-call sites in `scripts/keycloak/configure-realm.inside.sh` first and change only what is missing; do not rewrite the realm-configuration calls that already work.
- The retry helper lives in `scripts/keycloak/lib/readiness.sh` so the wait logic is testable in isolation; the inside script sources it and wraps only the login and first admin call.
- Back-off interval and growth policy are a deferred decision (ASSUM-002): pick a fixed short interval (e.g. 2 s) that fits comfortably inside the 2-minute budget and document it in the helper's header comment.
- The 2-minute budget is a single total wall-clock limit across all attempts, not a per-attempt timeout (ASSUM-003).
- Distinguish 503 from connection-refused from other transport failures (ASSUM-007); only the first two are retryable.
- Keep the admin password off argv and off every message (ASSUM-004, ASSUM-008): the existing `{ export ...; cat configure-realm.inside.sh; } | docker compose exec -T keycloak bash -s` transport in `scripts/keycloak-config.sh:36-43` already keeps secrets off the host command line — do not regress it, and never echo the password in retry diagnostics.
- A non-zero exit from this script is what stops the whole start-up sequence in the checks, local stack and deploy contexts (ASSUM-005); no change to the callers is needed.
- The shellcheck sweep at `scripts/check.sh:30` globs `scripts/*.sh scripts/keycloak/*.sh deploy/*.sh infra/postgres/*.sh` — it discovers its members at run time, so the new `scripts/keycloak/lib/readiness.sh` is covered automatically once it exists; search the whole repository for further deploy-context files before writing any check and add any found.
- Tests are delivered by TASK-KRSR-002 against a stand-in Keycloak (stub endpoint or command shim, ASSUM-006); this task must not require a live Keycloak to pass.