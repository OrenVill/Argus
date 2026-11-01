#!/usr/bin/env bash
set -euo pipefail

if ! redis-cli ping >/dev/null 2>&1; then
  sudo service redis-server start
fi

for _ in $(seq 1 30); do
  if redis-cli ping | grep -q PONG; then
    echo "redis ready on 127.0.0.1:6379"
    exit 0
  fi
  sleep 0.2
done

echo "redis failed to become ready" >&2
exit 1
