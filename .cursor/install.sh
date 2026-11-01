#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

go version

if ! command -v redis-server >/dev/null; then
  sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq redis-server
fi
redis-server --version

if [[ -f go.mod ]]; then
  go mod download
  go mod verify
fi

if ! command -v golangci-lint >/dev/null; then
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
fi

if [[ -f Makefile ]]; then
  make -n test >/dev/null 2>&1 || true
fi

echo "Argus dev dependencies ready."
