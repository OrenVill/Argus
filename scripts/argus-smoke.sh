#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! redis-cli ping 2>/dev/null | grep -q PONG; then
  echo "Redis required for smoke test" >&2
  exit 1
fi

export ARGUS_TEST_REDIS=1
go test ./test/golden/... -count=1 -tags=integration -run TestRuntimeSmokePipelineWebhookRedis
echo "smoke ok"
