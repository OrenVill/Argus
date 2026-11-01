#!/usr/bin/env bash
# Push current branch to Origin, then to GitHub.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "Run this from inside the Argus git checkout." >&2
  exit 1
}
cd "$ROOT"

BRANCH="${1:-$(git branch --show-current)}"
GITHUB_OWNER="${GITHUB_OWNER:-OrenVill}"
GITHUB_REPO="${GITHUB_REPO:-Argus}"

if ! git remote get-url origin | grep -q origin.cursor.com; then
  echo "Expected primary remote 'origin' to point at Origin (origin.cursor.com)." >&2
  exit 1
fi

if ! git remote get-url github >/dev/null 2>&1; then
  echo "Missing remote 'github'. Run: bash scripts/setup-github-remote.sh" >&2
  exit 1
fi

echo "Pushing ${BRANCH} to Origin..."
git push origin "${BRANCH}"

echo "Pushing ${BRANCH} to GitHub..."
git push github "${BRANCH}"

echo "Done. https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}"
