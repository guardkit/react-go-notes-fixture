# shellcheck shell=bash
# Sourced helpers shared by the stack scripts. Bash only; no project state.

die() {
  echo "$*" >&2
  exit 1
}

require_command() {
  local name
  for name in "$@"; do
    command -v "$name" >/dev/null || die "$name is required"
  done
}

# Docker CLI with the compose plugin; nothing is started here.
require_docker() {
  require_command docker curl
  docker compose version >/dev/null 2>&1 || die "docker compose is required"
}

# Every stack command needs an explicit project; the default name is refused
# so a script can never act on someone else's stack by accident.
require_project() {
  [[ -n "${COMPOSE_PROJECT_NAME:-}" ]] || die "COMPOSE_PROJECT_NAME is required"
  [[ "$COMPOSE_PROJECT_NAME" =~ ^[a-z0-9][a-z0-9_-]{0,62}$ ]] ||
    die "COMPOSE_PROJECT_NAME must be lower-case letters, digits, '-' or '_'"
}

# Random letters and digits from the kernel's generator.
rand_token() {
  local length="$1" out=""
  while ((${#out} < length)); do
    out+="$(head -c 256 /dev/urandom | LC_ALL=C tr -dc 'A-Za-z0-9')"
  done
  printf '%s' "${out:0:length}"
}

rand_hex() {
  local length="$1" out=""
  while ((${#out} < length)); do
    out+="$(head -c 64 /dev/urandom | LC_ALL=C tr -dc 'a-f0-9')"
  done
  printf '%s' "${out:0:length}"
}

# A loopback port nothing answers on. Compose's bind still fails loudly if
# another process takes it in between.
free_port() {
  local attempt port
  for attempt in $(seq 1 50); do
    port=$((20000 + $(od -An -N2 -tu2 /dev/urandom | tr -d ' ') % 20000))
    if ! (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null; then
      printf '%s' "$port"
      return 0
    fi
  done
  die "no free loopback port found after $attempt attempts"
}

# Values interpolated into JSON or the proxy's runtime settings stay plain.
require_plain() {
  local name="$1" value="${!1:-}"
  [[ -n "$value" ]] || die "$name is required"
  [[ "$value" != *[\"\\\`{}]* && "$value" != *$'\n'* ]] ||
    die "$name must not contain quotes, backslashes, braces or newlines"
}

# Curl body of a URL, failing on HTTP errors; bounded per attempt.
fetch() {
  curl -fsS --max-time 5 "$1"
}
