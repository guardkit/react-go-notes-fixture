---
id: TASK-KRSR-002
title: Add tests for the realm setup readiness wait
task_type: testing
parent_review: TASK-REV-6AC9
feature_id: FEAT-6AC9
wave: 2
implementation_mode: task-work
complexity: 4
dependencies:
  - TASK-KRSR-001
scaffolding_kind: tests
---

Add tests showing that the realm setup succeeds when Keycloak answers 503 a few times before it is ready, fails within the time limit when Keycloak never becomes ready, and fails at once on a wrong password.

## The words of the request this task serves

> Add tests showing that the setup succeeds when Keycloak answers 503 a few times before it is ready, fails within the time limit when Keycloak never becomes ready, and fails at once on a wrong password.

## Files to Create

- `tests/keycloak/test_configure_realm_readiness.sh`

## Files to Modify

- `scripts/check.sh`

## Acceptance Criteria

- [ ] A test stands in a Keycloak that answers the first few admin login attempts with 503 Service Unavailable and then serves successfully, and asserts the realm setup completes with a successful exit status and reports more than one login attempt.
- [ ] A test stands in a Keycloak that refuses the first few admin login connections and then serves successfully, and asserts the realm setup completes with a successful exit status and reports more than one login attempt.
- [ ] A test asserts that a 500 Internal Server Error fails on the first attempt with a non-zero exit, reports the 500 status, and reports only one login attempt.
- [ ] A test asserts that a 401 Unauthorized (wrong password) fails straight away with a non-zero exit, reports the 401 authentication failure, reports only one login attempt, and completes within 2 minutes.
- [ ] A test asserts that a Keycloak that answers 503 for the whole run is given up on within 2 minutes with a non-zero exit and a clear message naming the 2-minute budget and the admin endpoint probed.
- [ ] A test asserts that a Keycloak that refuses every connection for the whole run is given up on within 2 minutes with a non-zero exit and the same clear message.
- [ ] A test asserts that the admin password never appears in the realm setup's standard output or standard error, on both a successful run and a failed (401) run.
- [ ] A test asserts that a Keycloak that becomes ready just under 2 minutes is accepted and the elapsed time is at most 2 minutes.
- [ ] The new test file is wired into the aggregate check so it runs in CI (extend the shellcheck/`scripts/*.sh` sweep at `scripts/check.sh:30` or add an equivalent invocation).

## Implementation Notes

- Tests run against a stand-in Keycloak — a stub HTTP endpoint or a command shim placed on PATH (ASSUM-006) — never a live instance.
- The tests exercise the readiness helper and the inside script's login/first-admin-call path from TASK-KRSR-001; they depend on that task's deliverable.
- For the 2-minute-budget tests, make the budget injectable (e.g. an environment variable read by the helper) so the wall-clock tests run fast; the default remains 120 s.
- The test harness must not place the admin password on a command line either — pass it the same way the production path does.

## Seam Tests

The following seam test validates the integration contract with the producer task. Implement this test to verify the boundary before integration.

```python
"""Seam test: verify READINESS_BUDGET_SECONDS contract from TASK-KRSR-001."""
import pytest


@pytest.mark.seam
@pytest.mark.integration_contract("READINESS_BUDGET_SECONDS")
def test_readiness_budget_seconds_format():
    """Verify READINESS_BUDGET_SECONDS matches the expected format.

    Contract: integer number of seconds, default 120, parsed by the readiness helper in scripts/keycloak/lib/readiness.sh
    Producer: TASK-KRSR-001
    """
    # Producer side: get the artifact value
    value = ""  # e.g. os.environ.get("READINESS_BUDGET_SECONDS")

    # Consumer side: verify format matches contract
    assert value, "READINESS_BUDGET_SECONDS must not be empty"
    assert value.isdigit(), f"Expected an integer number of seconds, got: {value}"
```