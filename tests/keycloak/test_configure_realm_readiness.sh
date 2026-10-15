#!/usr/bin/env bash
# shellcheck shell=bash
#
# TASK-KRSR-002: the bounded readiness wait of the Keycloak realm setup,
# tested against a stand-in Keycloak (ASSUM-006) - a command shim placed in
# front of kcadm by the KCADM override in
# scripts/keycloak/configure-realm.inside.sh. No live instance, no container.
#
# The realm setup is driven exactly the way scripts/keycloak-config.sh drives
# it: the readiness helper and the inside script go down one stdin stream to
# `bash -s`, and the secrets travel inside that stream as `export` lines. The
# admin password therefore never appears on a command line here either, and
# the child's environment is not used for it.
#
# Budget (ASSUM-003): READINESS_BUDGET_SECONDS and
# READINESS_RETRY_INTERVAL_SECONDS are injected per test so the wall-clock
# cases stay fast; the helper's own default stays 120 s. One test drives a
# fake clock (PATH shims for `date` and `sleep`) so the "just under 2 minutes"
# case is exercised at the real 120 s boundary without waiting for it.
set -uo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readiness="$root/scripts/keycloak/lib/readiness.sh"
inside="$root/scripts/keycloak/configure-realm.inside.sh"
for needed in "$readiness" "$inside"; do
  [[ -f "$needed" ]] || { echo "missing the deliverable under test: $needed" >&2; exit 1; }
done

work="$(mktemp -d)"
state="$work/state"
stub="$work/kcadm-stub.sh"
mkdir -p "$state" "$work/shim"
trap 'rm -rf "$work"' EXIT

# The stand-in endpoint. The login probes <server>/realms/master/protocol/
# openid-connect/token and the first admin call <server>/admin/realms.
stub_server="http://127.0.0.1:18080/auth"
test_realm="notes-readiness-test"

# Secrets of this run. They are only ever handed to the setup through the
# stdin stream, never through env or argv of the setup process.
admin_password="KcAdminZq7TgRm2Xb9Lk4Nf8Vd3Hs6J"
user_password="NotesTestZp4Wq8Jt2Ry6Xc1Bm5Kd9Lf"

# Deterministic ids the stand-in serves for "already exists" answers.
stub_client_id="11111111-1111-4111-8111-111111111111"
stub_mapper_id="22222222-2222-4222-8222-222222222222"
stub_alice_id="33333333-3333-4333-8333-333333333333"
stub_bob_id="44444444-4444-4444-8444-444444444444"
fake_start=1000

checks=0
failures=0
failed_list=()

ok() {
  printf '  ok   %s\n' "$1"
  checks=$((checks + 1))
}
bad() {
  printf '  FAIL %s\n' "$1"
  checks=$((checks + 1))
  failures=$((failures + 1))
  failed_list+=("$1")
}
# One check each. `[[ ... ]]` is a reserved word, so it cannot be handed to a
# helper as a command; the comparisons live in these predicates instead.
assert_zero() {
  local desc="$1" status="$2"
  if [[ "$status" == "0" ]]; then ok "$desc"; else bad "$desc (exit status $status)"; fi
}
assert_nonzero() {
  local desc="$1" status="$2"
  if [[ "$status" != "0" ]]; then ok "$desc (exit status $status)"; else bad "$desc (exit status 0)"; fi
}
# assert_num <description> <left> <op> <right>
assert_num() {
  local desc="$1" left="$2" op="$3" right="$4" hit=no
  case "$op" in
  -eq) [[ "$left" == "$right" ]] && hit=yes ;;
  -ne) [[ "$left" != "$right" ]] && hit=yes ;;
  -gt) ((left > right)) && hit=yes ;;
  -ge) ((left >= right)) && hit=yes ;;
  -le) ((left <= right)) && hit=yes ;;
  esac
  if [[ "$hit" == "yes" ]]; then ok "$desc"; else bad "$desc ($left $op $right is false)"; fi
}
# assert_has <description> <text> <substring>
assert_has() {
  if [[ "$2" == *"$3"* ]]; then ok "$1"; else bad "$1 -- not in the output: $3"; fi
}
# assert_hasnt <description> <text> <substring never allowed to appear>
assert_hasnt() {
  if [[ "$2" != *"$3"* ]]; then ok "$1"; else bad "$1 -- the secret value was printed"; fi
}
# assert_file_has <description> <file> <fixed string>
assert_file_has() {
  if grep -qF -e "$3" -- "$2"; then ok "$1"; else bad "$1 -- not in $2: $3"; fi
}

# --- the stand-in Keycloak -------------------------------------------------
cat >"$stub" <<'STUB'
#!/usr/bin/env bash
# shellcheck shell=bash
# Stand-in for /opt/keycloak/bin/kcadm.sh (ASSUM-006). KCADM points the realm
# setup at this file, so the readiness helper talks to it instead of Keycloak.
# KC_STUB_MODE picks the behaviour:
#   503_then_ok:<n>     n login attempts answer HTTP 503, then it serves
#   refused_then_ok:<n> n login connections are refused, then it serves
#   500                 every login answers HTTP 500
#   401                 every login answers HTTP 401 (wrong password)
#   503_always          every login answers HTTP 503
#   refused_always      every login connection is refused
#   ready_at:<s>        answers 503 until <s> seconds of the (fake) clock
#                       have passed, then serves
set -uo pipefail

mode="${KC_STUB_MODE:?}"
state="${KC_STUB_STATE:?}"
server="${KCADM_SERVER:-http://127.0.0.1:18080/auth}"
token_url="$server/realms/master/protocol/openid-connect/token"
attempts_file="$state/login-attempts"

note() { printf '%s\n' "$*" >>"$state/calls.log"; }

bump() {
  local n=0
  [[ -f "$attempts_file" ]] && n="$(<"$attempts_file")"
  n=$((n + 1))
  printf '%s' "$n" >"$attempts_file"
  printf '%s' "$n"
}

# A chatty kcadm echoes what it was asked to do, password included. Nothing
# the realm setup prints may carry it, so the harness checks for it there.
echo_argv() {
  printf 'stub kcadm argv: %s\n' "$*" >>"$state/calls.log"
  printf 'stub kcadm argv: %s\n' "$*" >&2
}

fail_http() {
  printf 'Call "%s" returned HTTP %s\n' "$token_url" "$1" >&2
  printf '%s\n' "$2" >&2
  exit 1
}

fail_refused() {
  printf 'Call "%s" failed: java.net.ConnectException: Connection refused\n' "$token_url" >&2
  exit 1
}

login() {
  local attempt n now
  attempt="$(bump)"
  note "login attempt $attempt"
  echo_argv "$@"
  case "$mode" in
  503_then_ok:*)
    n="${mode#503_then_ok:}"
    ((attempt > n)) || fail_http 503 'Service Unavailable'
    ;;
  refused_then_ok:*)
    n="${mode#refused_then_ok:}"
    ((attempt > n)) || fail_refused
    ;;
  500)
    fail_http 500 'Internal Server Error'
    ;;
  401)
    fail_http 401 '{"error":"invalid_grant","error_description":"Invalid user credentials"}'
    ;;
  503_always)
    fail_http 503 'Service Unavailable'
    ;;
  refused_always)
    fail_refused
    ;;
  ready_at:*)
    n="${mode#ready_at:}"
    now="$(<"$KC_FAKE_CLOCK")"
    ((now - $(<"$state/clock-start") >= n)) || fail_http 503 'Service Unavailable'
    ;;
  *)
    printf 'stub kcadm: unknown mode %s\n' "$mode" >&2
    exit 1
    ;;
  esac
  printf 'stub kcadm: admin login succeeded on attempt %s\n' "$attempt" >&2
}

case "${1:-}" in
config)
  login "$@"
  exit 0
  ;;
get)
  case "${2:-}" in
  realms) printf 'master\n%s\n' "${NOTES_REALM:-notes}" ;;
  clients) printf '%s\n' "${STUB_CLIENT_ID:?}" ;;
  clients/*) printf '%s,notes-api-audience\n' "${STUB_MAPPER_ID:?}" ;;
  users)
    name=""
    for arg in "$@"; do
      [[ "$arg" == username=* ]] && name="${arg#username=}"
    done
    case "$name" in
    alice) printf '%s\n' "${STUB_ALICE_ID:?}" ;;
    bob) printf '%s\n' "${STUB_BOB_ID:?}" ;;
    *) printf '\n' ;;
    esac
    ;;
  *) printf '\n' ;;
  esac
  exit 0
  ;;
create | update)
  note "$*"
  exit 0
  ;;
*)
  note "unhandled: $*"
  exit 0
  ;;
esac
STUB
chmod +x "$stub"

# The fake clock: `date +%s` and `sleep` for the setup process only, so a run
# can walk a whole 2-minute budget in milliseconds.
cat >"$work/shim/date" <<'SHIM'
#!/usr/bin/env bash
if [[ $# -eq 1 && "$1" == "+%s" ]]; then
  printf '%s\n' "$(<"$KC_FAKE_CLOCK")"
else
  exec /bin/date "$@"
fi
SHIM
cat >"$work/shim/sleep" <<'SHIM'
#!/usr/bin/env bash
seconds="${1:-0}"
case "$seconds" in '' | *[!0-9]*) seconds=0 ;; esac
printf '%s' "$(( $(<"$KC_FAKE_CLOCK") + seconds ))" >"$KC_FAKE_CLOCK"
SHIM
chmod +x "$work/shim/date" "$work/shim/sleep"

# --- driving the realm setup ------------------------------------------------
# run_setup <stub mode> [VAR=VALUE ...]. Sets setup_status, setup_out,
# setup_err, setup_elapsed (wall seconds) and stub_attempts.
use_fake_clock=0
run_setup() {
  local mode="$1"
  shift
  printf '0' >"$state/login-attempts"
  : >"$state/calls.log"
  local -a envp=(
    -u READINESS_BUDGET_SECONDS
    -u READINESS_RETRY_INTERVAL_SECONDS
    "KCADM=$stub"
    "KCADM_SERVER=$stub_server"
    "KC_STUB_MODE=$mode"
    "KC_STUB_STATE=$state"
    "STUB_CLIENT_ID=$stub_client_id"
    "STUB_MAPPER_ID=$stub_mapper_id"
    "STUB_ALICE_ID=$stub_alice_id"
    "STUB_BOB_ID=$stub_bob_id"
  )
  if ((use_fake_clock)); then
    printf '%s' "$fake_start" >"$work/clock"
    printf '%s' "$fake_start" >"$state/clock-start"
    envp+=("KC_FAKE_CLOCK=$work/clock" "PATH=$work/shim:$PATH")
  fi
  envp+=("$@")
  local started="$SECONDS"
  {
    # Exactly the stdin transport of scripts/keycloak-config.sh: the secrets
    # first, then the helper, then the script body that uses them.
    printf 'export NOTES_REALM=%q\n' "$test_realm"
    printf 'export NOTES_CLIENT_ID=%q\n' notes-web
    printf 'export NOTES_AUDIENCE=%q\n' notes-api
    printf 'export NOTES_REDIRECTS_JSON=%q\n' '["http://127.0.0.1:41001/*"]'
    printf 'export KEYCLOAK_ADMIN_PASSWORD=%q\n' "$admin_password"
    printf 'export NOTES_TEST_PASSWORD=%q\n' "$user_password"
    cat "$readiness"
    cat "$inside"
  } | env "${envp[@]}" bash -s >"$work/out" 2>"$work/err"
  setup_status=$?
  setup_elapsed=$((SECONDS - started))
  setup_out="$(<"$work/out")"
  setup_err="$(<"$work/err")"
  stub_attempts="$(<"$state/login-attempts")"
  use_fake_clock=0
}

# The attempt count the helper reported for the admin login.
reported_login_attempts() {
  if [[ "$setup_err" =~ admin\ login\ ready\ on\ attempt\ ([0-9]+) ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  elif [[ "$setup_err" =~ admin\ login\ failed.*after\ ([0-9]+)\ attempt ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  else
    printf '0'
  fi
}

# The seconds the helper reported for the admin login's readiness.
reported_login_seconds() {
  if [[ "$setup_err" =~ ready\ on\ attempt\ [0-9]+,\ ([0-9]+)s\ into ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  else
    printf '0'
  fi
}

user_lines() { grep -c '^user-id ' <<<"$setup_out"; }

# --- the tests ---------------------------------------------------------------
ac001_503_then_ready() {
  run_setup "503_then_ok:3" "READINESS_RETRY_INTERVAL_SECONDS=1"
  local reported
  reported="$(reported_login_attempts)"
  assert_zero "AC-001 realm setup exits 0 after three 503s" "$setup_status"
  assert_num "AC-001 realm setup configured both test users" "$(user_lines)" -eq 2
  assert_num "AC-001 more than one login attempt reported (got $reported)" "$reported" -gt 1
  assert_num "AC-001 the stand-in really was called more than once (got $stub_attempts)" "$stub_attempts" -gt 1
  assert_has "AC-001 each retry was reported as a 503" "$setup_err" "got HTTP 503 Service Unavailable"
}

ac002_refused_then_ready() {
  run_setup "refused_then_ok:3" "READINESS_RETRY_INTERVAL_SECONDS=1"
  local reported
  reported="$(reported_login_attempts)"
  assert_zero "AC-002 realm setup exits 0 after three refused connections" "$setup_status"
  assert_num "AC-002 realm setup configured both test users" "$(user_lines)" -eq 2
  assert_num "AC-002 more than one login attempt reported (got $reported)" "$reported" -gt 1
  assert_num "AC-002 the stand-in really was called more than once (got $stub_attempts)" "$stub_attempts" -gt 1
  assert_has "AC-002 each retry was reported as a refused connection" "$setup_err" "got connection refused"
}

ac003_500_fails_at_once() {
  run_setup "500"
  local reported
  reported="$(reported_login_attempts)"
  assert_nonzero "AC-003 a 500 exits non-zero" "$setup_status"
  assert_has "AC-003 the 500 status is reported" "$setup_err" "HTTP 500"
  assert_has "AC-003 the reason is named" "$setup_err" "Internal Server Error"
  assert_num "AC-003 it is not retried: one attempt reported (got $reported)" "$reported" -eq 1
  assert_num "AC-003 the stand-in was called exactly once (got $stub_attempts)" "$stub_attempts" -eq 1
  assert_num "AC-003 no users were configured" "$(user_lines)" -eq 0
}

ac004_wrong_password_fails_at_once() {
  run_setup "401"
  local reported
  reported="$(reported_login_attempts)"
  assert_nonzero "AC-004 a wrong password exits non-zero" "$setup_status"
  assert_has "AC-004 the 401 status is reported" "$setup_err" "HTTP 401"
  assert_has "AC-004 the authentication failure is named" "$setup_err" "Unauthorized - authentication failed"
  assert_num "AC-004 it is not retried: one attempt reported (got $reported)" "$reported" -eq 1
  assert_num "AC-004 the stand-in was called exactly once (got $stub_attempts)" "$stub_attempts" -eq 1
  assert_num "AC-004 it fails straight away, within 2 minutes (${setup_elapsed}s)" "$setup_elapsed" -le 120
}

ac005_never_ready_503() {
  run_setup "503_always" "READINESS_BUDGET_SECONDS=3" "READINESS_RETRY_INTERVAL_SECONDS=1"
  assert_nonzero "AC-005 a Keycloak stuck on 503 exits non-zero" "$setup_status"
  assert_num "AC-005 it is given up on within 2 minutes (${setup_elapsed}s)" "$setup_elapsed" -le 120
  assert_num "AC-005 the give-up fits the injected budget (${setup_elapsed}s)" "$setup_elapsed" -le 6
  assert_has "AC-005 the message says Keycloak was not ready in time" "$setup_err" "not ready in time"
  assert_has "AC-005 the message names the 2-minute budget" "$setup_err" "2-minute budget"
  assert_has "AC-005 the message names the admin endpoint probed" \
    "$setup_err" "$stub_server/realms/master/protocol/openid-connect/token"
  assert_has "AC-005 the last failure is reported as a 503" "$setup_err" "last failure: HTTP 503 Service Unavailable"
  assert_num "AC-005 it kept trying until the budget ran out (got $stub_attempts attempts)" "$stub_attempts" -gt 1
}

ac006_never_ready_refused() {
  run_setup "refused_always" "READINESS_BUDGET_SECONDS=3" "READINESS_RETRY_INTERVAL_SECONDS=1"
  assert_nonzero "AC-006 a Keycloak that refuses every connection exits non-zero" "$setup_status"
  assert_num "AC-006 it is given up on within 2 minutes (${setup_elapsed}s)" "$setup_elapsed" -le 120
  assert_num "AC-006 the give-up fits the injected budget (${setup_elapsed}s)" "$setup_elapsed" -le 6
  assert_has "AC-006 the message says Keycloak was not ready in time" "$setup_err" "not ready in time"
  assert_has "AC-006 the message names the 2-minute budget" "$setup_err" "2-minute budget"
  assert_has "AC-006 the message names the admin endpoint probed" \
    "$setup_err" "$stub_server/realms/master/protocol/openid-connect/token"
  assert_has "AC-006 the last failure is reported as a refused connection" \
    "$setup_err" "last failure: connection refused"
}

ac007_password_never_printed() {
  run_setup "503_then_ok:1" "READINESS_RETRY_INTERVAL_SECONDS=1"
  assert_zero "AC-007 the successful run exits 0" "$setup_status"
  assert_hasnt "AC-007 no admin password on the successful run's stdout" "$setup_out" "$admin_password"
  assert_hasnt "AC-007 no user password on the successful run's stdout" "$setup_out" "$user_password"
  assert_hasnt "AC-007 no admin password on the successful run's stderr" "$setup_err" "$admin_password"
  assert_hasnt "AC-007 no user password on the successful run's stderr" "$setup_err" "$user_password"
  run_setup "401"
  assert_nonzero "AC-007 the failed run exits non-zero" "$setup_status"
  assert_hasnt "AC-007 no admin password on the failed run's stdout" "$setup_out" "$admin_password"
  assert_hasnt "AC-007 no user password on the failed run's stdout" "$setup_out" "$user_password"
  assert_hasnt "AC-007 no admin password on the failed run's stderr" "$setup_err" "$admin_password"
  assert_hasnt "AC-007 no user password on the failed run's stderr" "$setup_err" "$user_password"
  # The stand-in did receive the password on its own command line, the way
  # the production login does, so the checks above are not vacuous.
  assert_file_has "AC-007 the stand-in login carried a --password argument" "$state/calls.log" "--password"
}

ac008_ready_just_under_2_minutes() {
  use_fake_clock=1
  run_setup "ready_at:118"
  local reported
  reported="$(reported_login_seconds)"
  assert_zero "AC-008 Keycloak ready just under 2 minutes is accepted" "$setup_status"
  assert_num "AC-008 realm setup configured both test users" "$(user_lines)" -eq 2
  assert_num "AC-008 the readiness took most of the budget (${reported}s of 120s)" "$reported" -ge 100
  assert_num "AC-008 the elapsed time is at most 2 minutes (${reported}s)" "$reported" -le 120
  assert_num "AC-008 the run itself took ${setup_elapsed}s of wall clock" "$setup_elapsed" -le 120
}

ac009_wired_into_the_check() {
  assert_file_has "AC-009 scripts/check.sh runs this file" \
    "$root/scripts/check.sh" "tests/keycloak/test_configure_realm_readiness.sh"
  assert_file_has "AC-009 scripts/check.sh sweeps this file's shell syntax too" \
    "$root/scripts/check.sh" "tests/keycloak/*.sh"
}

readiness_budget_contract() {
  # Seam: READINESS_BUDGET_SECONDS is an integer number of seconds, default
  # 120, read by scripts/keycloak/lib/readiness.sh (TASK-KRSR-001).
  assert_file_has "seam the default budget is 120 s" "$readiness" "READINESS_BUDGET_SECONDS:-120"
  run_setup "503_always" "READINESS_BUDGET_SECONDS=not-a-number"
  assert_nonzero "seam a non-integer budget is refused" "$setup_status"
  assert_has "seam a non-integer budget is explained" "$setup_err" "must be whole numbers of seconds"
}

printf 'keycloak realm setup readiness tests\n'
for test_case in readiness_budget_contract ac001_503_then_ready ac002_refused_then_ready \
  ac003_500_fails_at_once ac004_wrong_password_fails_at_once ac005_never_ready_503 \
  ac006_never_ready_refused ac007_password_never_printed \
  ac008_ready_just_under_2_minutes ac009_wired_into_the_check; do
  printf '* %s\n' "$test_case"
  "$test_case"
done

printf '%d checks passed\n' "$checks"
if ((failures > 0)); then
  printf '%d checks failed\n' "$failures"
  for failure in "${failed_list[@]}"; do
    printf '  - %s\n' "$failure"
  done
  exit 1
fi
printf '0 checks failed\n'
