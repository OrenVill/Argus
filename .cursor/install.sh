#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

go version
redis-server --version

if [[ -f go.mod ]]; then
  go mod download
  go mod verify
fi

if [[ -f Makefile ]]; then
  make -n test >/dev/null 2>&1 || true
fi

echo "Argus dev dependencies ready."
