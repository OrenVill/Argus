#!/usr/bin/env bash
# Bootstrap: install Origin CLI (if needed), clone oren/Argus, install dev deps.
set -euo pipefail

TARGET="${1:-Argus}"
ORIGIN_REPO="${ORIGIN_REPO:-oren/Argus}"

if ! command -v origin >/dev/null; then
  curl -fsSL https://downloads.cursor.com/origin/install.sh | sh
  export PATH="${HOME}/.local/bin:${PATH}"
fi

if [[ ":${PATH}:" != *":${HOME}/.local/bin:"* ]]; then
  echo 'Add to ~/.bashrc: export PATH="$HOME/.local/bin:$PATH"' >&2
fi

origin auth status >/dev/null 2>&1 || origin auth login

if [[ -d "${TARGET}/.git" ]]; then
  echo "Directory ${TARGET} already exists; skipping clone." >&2
else
  origin repo clone "${ORIGIN_REPO}" "${TARGET}"
fi

cd "${TARGET}"
bash .cursor/install.sh
bash .cursor/start.sh

echo "Clone ready. To add GitHub: bash scripts/setup-github-remote.sh"
