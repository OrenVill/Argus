#!/usr/bin/env bash
# Push current branch to Origin (source of truth), then to public GitHub (portfolio).
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "Run this from inside the Argus git checkout." >&2
  exit 1
}
cd "$ROOT"

BRANCH="${1:-$(git branch --show-current)}"

if ! git remote get-url origin | grep -q origin.cursor.com; then
  echo "Expected primary remote 'origin' to point at Origin (origin.cursor.com)." >&2
  exit 1
fi

if ! git remote get-url github >/dev/null 2>&1; then
  echo "Missing remote 'github'. Run: bash scripts/github-portfolio-setup.sh" >&2
  exit 1
fi

echo "Pushing ${BRANCH} to Origin..."
git push origin "${BRANCH}"

echo "Pushing ${BRANCH} to GitHub (public portfolio)..."
git push github "${BRANCH}"

echo "Done. Portfolio: https://github.com/${GITHUB_OWNER:-OrenVill}/${GITHUB_REPO:-Argus}"
