# Argus

Tier-1 log anomaly detection daemon: tail logs, mask and template with Drain, track state in Redis, and POST to webhooks only on real anomalies.

**Status:** v1 Tier-1 daemon implemented (ingest, pipeline, Redis detection, async webhooks, control API, CLI). See [`docs/SPEC.md`](docs/SPEC.md).

Product specification: [`docs/SPEC.md`](docs/SPEC.md) (implementation-ready v1.0).

Agent skills: [`.cursor/skills/`](.cursor/skills/) (e.g. `large-mission-development` for multi-track epics).

## Remotes

| Role | Forge | Remote name | URL |
| --- | --- | --- | --- |
| **Daily work (canonical)** | Origin | `origin` | [oren/Argus](https://cursor.com/codebase/oren/Argus) |
| **Public on GitHub** | GitHub | `github` | [OrenVill/Argus](https://github.com/OrenVill/Argus) |

Cloud Agents and local development use **Origin**. Push to **`github`** when you want the public repo updated. Origin personal repos stay private on Cursor.

### First-time machine setup (WSL)

```bash
# Install Origin CLI if needed, clone, start Redis for dev
bash scripts/clone-origin.sh ~/code/Argus
cd ~/code/Argus

# One-time: create public GitHub repo + add `github` remote (uses `gh`)
bash scripts/setup-github-remote.sh
git push origin main
bash scripts/push-github.sh
```

If `origin: command not found` after install:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

Origin CLI: [cursor.com/docs/origin/cli](https://cursor.com/docs/origin/cli)

### After each feature or agent session

```bash
git push origin main       # always first — canonical history on Origin
bash scripts/push-github.sh   # same branch to GitHub
```

Do **not** use **Sync from GitHub** on the codebase home if you want Origin to stay authoritative; that flow makes GitHub the source of truth.

## Development environment

Cloud Agents use [`.cursor/environment.json`](.cursor/environment.json):

| Component | Purpose |
| --- | --- |
| Go 1.23 | Daemon implementation (spec allows Go or Rust; this repo targets Go) |
| Redis | Template state (`argus:<env>:<service>:<hash>`) |
| Port 9090 | Control API (`control.listen`, default `127.0.0.1:9090`) |
| Port 8765 | Reserved for a local webhook mock during development |

### Local commands

```bash
bash .cursor/install.sh
bash .cursor/start.sh
redis-cli ping
```

After you **Save** the Cloud Agent environment, Redis should start automatically on each agent boot via `start`. If `docs/SPEC.md` is missing locally, run `git pull origin main` (ensure latest commits are on Origin).

Example configuration: [`config/argus.yaml.example`](config/argus.yaml.example).

### Run locally

```bash
export ALERT_WEBHOOK_URL=http://127.0.0.1:8765/hook
go build -o argus ./cmd/argus
./argus run -c config/argus.yaml.example   # set inputs.stdin or files for your environment
./argus grace start --service payment-svc --duration 30m -c config/argus.yaml.example
./argus template ignore --hash <hex> -c config/argus.yaml.example
```

Smoke (Redis + mock webhook pipeline): `bash scripts/argus-smoke.sh`
