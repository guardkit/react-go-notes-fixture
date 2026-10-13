#!/usr/bin/env bash
# Builds what the images copy: the web bundle and a static API binary for the
# Docker engine's architecture, with the declared, pinned toolchain. The image
# builds then need no network beyond their pinned base images.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/require-tools.sh
source scripts/lib.sh
require_command docker
arch="$(docker version --format '{{.Server.Arch}}')"
[[ "$arch" =~ ^(amd64|arm64)$ ]] || die "unsupported Docker engine architecture: $arch"
npm --workspace sample-web run build
(
  cd apps/api
  mkdir -p bin
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o bin/notes-api ./cmd/api
)
