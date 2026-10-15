# shellcheck shell=bash
# Bounded readiness wait for the Keycloak realm setup (TASK-KRSR-001).
#
# Sourced by scripts/keycloak/configure-realm.inside.sh, which wraps only the
# admin login and the first admin call in it; the realm-configuration calls
# that follow are left exactly as they were. The helper is function
# definitions only, so scripts/keycloak-config.sh can send it down the same
# stdin stream as the script body (the keycloak image has no copy of the
# repository).
#
# Budget (ASSUM-003): READINESS_BUDGET_SECONDS - a whole number, default 120 -
# is ONE total wall-clock limit across every attempt of this run's readiness
# wait, never a per-attempt timeout. The login and the first admin call share
# that one deadline: the wait cannot outlast 2 minutes in total.
# Interval (ASSUM-002): a fixed READINESS_RETRY_INTERVAL_SECONDS - a whole
# number, default 2 - between attempts, with no growth. 2 s divides the
# 2-minute budget evenly, so the deadline arithmetic stays obvious.
#
# Retryable (ASSUM-007): HTTP 503 Service Unavailable, and a refused
# connection. Everything else - a 401 from a wrong password, a 500, any other
# status, any other transport failure - fails on the first attempt, reporting
# the status it received.
#
# The admin password (ASSUM-004, ASSUM-008) arrives through the stdin
# transport in scripts/keycloak-config.sh, is never echoed by this file, and
# is redacted from every diagnostic before anything is printed.
#
# Usage (from the sourcing script, which supplies k() and kc_server):
#   login_with_readiness <stdout_path>
#   kc_wait_ready <what> <endpoint> <stdout_path> [--] <command> [args...]
# Exit 0 puts the command's stdout in <stdout_path>; exit 1 explains itself on
# standard error. Diagnostics go to standard error only, because the caller's
# standard output is parsed for "user-id" lines.
#
# Globals set by a failed attempt: KC_WAIT_STATUS (the HTTP status seen, empty
# when there was none), KC_WAIT_RETRY (yes|no), KC_WAIT_DETAIL (one line).
# KC_WAIT_STARTED and KC_WAIT_DEADLINE carry the one budget across the calls:
# the first kc_wait_ready of a run fixes the deadline and the rest keep it.
KC_WAIT_STATUS=""
KC_WAIT_RETRY=no
KC_WAIT_DETAIL=""
KC_WAIT_STARTED=""
KC_WAIT_DEADLINE=""

# This run's secrets, one per line. Only the names are ever printed.
kc_secret_values() {
  local name
  for name in KEYCLOAK_ADMIN_PASSWORD NOTES_TEST_PASSWORD; do
    if [[ -n "${!name:-}" ]]; then
      printf '%s\n' "${!name}"
    fi
  done
}

# Any text with this run's secrets replaced. Every diagnostic goes through
# here, so a command that echoes its own arguments cannot leak the password.
kc_redact_secrets() {
  local text=$1 secret
  while IFS= read -r secret; do
    if [[ -n "$secret" ]]; then
      text="${text//"$secret"/"[redacted]"}"
    fi
  done < <(kc_secret_values)
  printf '%s' "$text"
}

# One line of context out of a command's own output, redacted.
kc_first_line() {
  local text
  text="$(kc_redact_secrets "$1")"
  text="${text//$'\r'/}"
  text="${text#"${text%%[![:space:]]*}"}"
  printf '%s' "${text%%$'\n'*}"
}

kc_status_reason() {
  case "$1" in
  400) printf '%s' "Bad Request" ;;
  401) printf '%s' "Unauthorized - authentication failed (check KEYCLOAK_ADMIN_PASSWORD)" ;;
  403) printf '%s' "Forbidden" ;;
  404) printf '%s' "Not Found" ;;
  500) printf '%s' "Internal Server Error" ;;
  502) printf '%s' "Bad Gateway" ;;
  503) printf '%s' "Service Unavailable" ;;
  504) printf '%s' "Gateway Timeout" ;;
  *) printf '%s' "HTTP status $1" ;;
  esac
}

# Classify one failed attempt into KC_WAIT_STATUS / KC_WAIT_RETRY /
# KC_WAIT_DETAIL. The status comes from the command's own words: kcadm says
# `Call "<url>" returned HTTP 503`, so an "HTTP <code>", a "<code> <reason>"
# or a "status=<code>" is the authority. With no status anywhere, a refused
# connection (or curl's exit 7) is the other retryable outcome; anything else
# is a plain failure.
kc_classify_attempt() {
  local rc=$1 text=$2 lower code="" reason=""
  local http_re='http[[:space:]]+([0-9]{3})'
  local word_re='(^|[^0-9])([0-9]{3})[[:space:]]+(service[[:space:]]unavailable|unauthorized|internal[[:space:]]server[[:space:]]error|bad[[:space:]]request|not[[:space:]]found|forbidden|bad[[:space:]]gateway|gateway[[:space:]]timeout)'
  local form_re='status[[:space:]]*[=:][[:space:]]*([0-9]{3})'
  local refused_re='connection[[:space:]]refused|connectexception|econnrefused'
  KC_WAIT_STATUS=""
  KC_WAIT_RETRY=no
  lower="${text,,}"
  if [[ "$lower" =~ $http_re ]]; then
    code="${BASH_REMATCH[1]}"
  elif [[ "$lower" =~ $word_re ]]; then
    code="${BASH_REMATCH[2]}"
  elif [[ "$lower" =~ $form_re ]]; then
    code="${BASH_REMATCH[1]}"
  fi
  if [[ -n "$code" ]]; then
    KC_WAIT_STATUS="$code"
    reason="$(kc_status_reason "$code")"
    KC_WAIT_DETAIL="HTTP $code $reason"
    if [[ "$code" == "503" ]]; then
      KC_WAIT_RETRY=yes
    fi
  elif [[ "$lower" =~ $refused_re ]] || [[ "$rc" == "7" ]]; then
    KC_WAIT_DETAIL="connection refused"
    KC_WAIT_RETRY=yes
  else
    KC_WAIT_DETAIL="no HTTP status; $(kc_first_line "$text") (exit $rc)"
  fi
}

# The single give-up message: it names the 2-minute budget and the admin
# endpoint probed, and never the password.
kc_not_ready_in_time() {
  local what=$1 endpoint=$2 budget=$3 attempts=$4 elapsed=$5 detail=$6 budget_note
  if [[ "$budget" == "120" ]]; then
    budget_note="2 minutes, 120s total"
  else
    budget_note="2 minutes; READINESS_BUDGET_SECONDS in force: ${budget}s"
  fi
  printf 'keycloak: Keycloak was not ready in time: %s did not succeed at %s within the 2-minute budget (%s; %s attempt(s), %ss elapsed); last failure: %s\n' \
    "$what" "$endpoint" "$budget_note" "$attempts" "$elapsed" "$(kc_redact_secrets "$detail")" >&2
}

# kc_wait_ready <what> <endpoint> <stdout_path> [--] <command> [args...]
# Retry while the outcome is retryable, give up when the one total budget runs
# out, and fail straight away on anything else.
kc_wait_ready() {
  local what=$1 endpoint=$2 out=$3
  shift 3
  if [[ "${1:-}" == "--" ]]; then
    shift
  fi
  local budget="${READINESS_BUDGET_SECONDS:-120}"
  local interval="${READINESS_RETRY_INTERVAL_SECONDS:-2}"
  if [[ ! "$budget" =~ ^[0-9]+$ ]] || [[ ! "$interval" =~ ^[0-9]+$ ]] || [[ "$interval" == "0" ]]; then
    printf 'keycloak: READINESS_BUDGET_SECONDS (%s) and READINESS_RETRY_INTERVAL_SECONDS (%s) must be whole numbers of seconds\n' \
      "$budget" "$interval" >&2
    return 1
  fi
  local err now nap attempt=0 rc text deadline
  # One budget for the whole readiness wait: the first call of this run fixes
  # the deadline, and every later call keeps it (ASSUM-003).
  if [[ -z "${KC_WAIT_DEADLINE:-}" ]]; then
    KC_WAIT_STARTED="$(date +%s)"
    KC_WAIT_DEADLINE=$((KC_WAIT_STARTED + budget))
  fi
  started=$KC_WAIT_STARTED
  deadline=$KC_WAIT_DEADLINE
  err="$(mktemp "${TMPDIR:-/tmp}/kc-readiness-XXXXXX" 2>/dev/null)" ||
    err="/tmp/kc-readiness-$$"
  while true; do
    attempt=$((attempt + 1))
    rc=0
    "$@" >"$out" 2>"$err" || rc=$?
    if [[ "$rc" == "0" ]]; then
      if ((attempt > 1)); then
        printf 'keycloak: %s ready on attempt %d, %ss into the 2-minute budget\n' \
          "$what" "$attempt" "$(($(date +%s) - started))" >&2
      fi
      rm -f "$err"
      return 0
    fi
    text="$(<"$err")"
    kc_classify_attempt "$rc" "$text"
    if [[ "$KC_WAIT_RETRY" != "yes" ]]; then
      printf 'keycloak: %s failed at %s: %s (not retryable, failing straight away after %d attempt(s))\n' \
        "$what" "$endpoint" "$KC_WAIT_DETAIL" "$attempt" >&2
      rm -f "$err"
      return 1
    fi
    now="$(date +%s)"
    if ((now >= deadline)); then
      kc_not_ready_in_time "$what" "$endpoint" "$budget" "$attempt" \
        "$((now - started))" "$KC_WAIT_DETAIL"
      rm -f "$err"
      return 1
    fi
    nap=$((deadline - now))
    if ((nap > interval)); then
      nap=$interval
    fi
    if ((nap < 1)); then
      nap=1
    fi
    printf 'keycloak: waiting for Keycloak: %s attempt %d got %s; retrying in %ds (%ds left of the 2-minute budget)\n' \
      "$what" "$attempt" "$KC_WAIT_DETAIL" "$nap" "$((deadline - now))" >&2
    sleep "$nap"
  done
}

# The admin login, waited for. The password reaches the login through the
# stdin transport of scripts/keycloak-config.sh - never a host command line -
# and is redacted from everything kc_wait_ready prints.
login_with_readiness() {
  kc_wait_ready "admin login" \
    "$kc_server/realms/master/protocol/openid-connect/token" "$1" -- \
    k config credentials --server "$kc_server" --realm master --user admin \
    --password "$KEYCLOAK_ADMIN_PASSWORD"
}
