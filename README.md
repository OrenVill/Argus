# Argus

Tier-1 log anomaly detection daemon: tail logs, mask and template with Drain, track state in Redis, and POST to webhooks only on real anomalies.

**Status:** specification and development environment only — the daemon is not implemented yet.

## Development environment

Cloud Agents use [`.cursor/environment.json`](.cursor/environment.json):

| Component | Purpose |
| --- | --- |
| Go 1.23 | Daemon implementation (spec allows Go or Rust; this repo targets Go) |
| Redis | Template state (`argus:<env>:<service>:<hash>`) |
| Port 9090 | Reserved for the future control API |
| Port 8765 | Reserved for a local webhook mock during development |

### Local commands

```bash
bash .cursor/install.sh
bash .cursor/start.sh
redis-cli ping
```

Example configuration: [`config/argus.yaml.example`](config/argus.yaml.example).

## Repositories

| Forge | Location | Visibility |
| --- | --- | --- |
| **Origin (Cursor)** | [oren/Argus](https://cursor.com/codebase/oren/Argus) | Private (personal Origin repos stay private) |
| **GitHub** | `https://github.com/<your-user>/Argus` after publish | Public (once created) |

Origin and GitHub can stay linked via **Sync from GitHub** so pushes to GitHub update the Origin mirror. See [Mirror a GitHub repository](https://cursor.com/docs/origin/mirror-github).
