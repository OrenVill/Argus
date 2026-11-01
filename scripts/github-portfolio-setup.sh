#!/usr/bin/env bash
# One-time: create public GitHub repo (if missing) and add the `github` remote.
# Day-to-day work stays on Origin (`origin` remote). Run from repo root.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "Run this from inside the Argus git checkout." >&2
  exit 1
}
cd "$ROOT"

GITHUB_OWNER="${GITHUB_OWNER:-OrenVill}"
GITHUB_REPO="${GITHUB_REPO:-Argus}"
PUBLIC_URL="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}.git"

if ! command -v gh >/dev/null; then
  echo "Install GitHub CLI: https://cli.github.com/" >&2
  exit 1
fi

if [[ -z "${GH_TOKEN:-}" ]]; then
  export GH_TOKEN="$(gh auth token)"
fi

if git remote get-url github >/dev/null 2>&1; then
  echo "Remote github already exists: $(git remote get-url github)"
else
  if ! gh repo view "${GITHUB_OWNER}/${GITHUB_REPO}" >/dev/null 2>&1; then
    gh repo create "${GITHUB_OWNER}/${GITHUB_REPO}" --public \
      --description "Tier-1 log anomaly detection daemon (Drain + Redis + webhooks)"
    echo "Created public ${GITHUB_OWNER}/${GITHUB_REPO}"
  else
    echo "GitHub repo ${GITHUB_OWNER}/${GITHUB_REPO} already exists"
  fi

  git remote add github "${PUBLIC_URL}"
  echo "Added remote github -> ${PUBLIC_URL}"
fi

echo
echo "Next: push Origin main, then mirror to GitHub:"
echo "  git push origin main"
echo "  git push github main"
echo "Or: bash scripts/push-portfolio.sh"
