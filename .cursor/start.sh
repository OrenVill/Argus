#!/usr/bin/env bash
set -euo pipefail

start_redis() {
  if redis-cli ping >/dev/null 2>&1; then
    return 0
  fi
  if command -v redis-server >/dev/null; then
    redis-server --daemonize yes --bind 127.0.0.1 --port 6379 \
      --save "" --appendonly no --loglevel notice 2>/dev/null || true
  fi
  if ! redis-cli ping >/dev/null 2>&1 && command -v service >/dev/null; then
    sudo service redis-server start 2>/dev/null || true
  fi
}

start_redis

for _ in $(seq 1 30); do
  if redis-cli ping 2>/dev/null | grep -q PONG; then
    echo "redis ready on 127.0.0.1:6379"
    exit 0
  fi
  sleep 0.2
done

echo "redis failed to become ready" >&2
exit 1
