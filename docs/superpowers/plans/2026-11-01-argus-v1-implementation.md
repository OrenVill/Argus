# Argus v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the full Argus v1 daemon in Go per [`docs/SPEC.md`](../../SPEC.md): ingest, mask, Drain, Redis Lua anomaly machine, webhooks, control API, and CLI.

**Architecture:** Layered monolith with three mission tracks (normalization, redis-detection, runtime); serialized merges onto **`cursor/argus-v1-integration-8966`**, then one PR to `main`. Hot path stays in worker goroutines; webhooks async.

**Tech Stack:** Go 1.23+, redis/go-redis/v9, cobra, chi, yaml.v3, xxhash, nxadm/tail, fsnotify, embedded Lua.

## Global Constraints

- Go **1.23+**; single binary `argus`.
- **No unmasked** log content in Redis or webhook JSON.
- Template hash: **xxHash64** hex lowercase on `{env}\0{service}\0{template}`.
- Redis template updates: **single Lua script** atomic RMW per hash.
- Deploy grace in Redis key `argus:{env}:grace:{service}`.
- Control API default `127.0.0.1:9090`; webhook timeout default **5s**; retries **1s, 4s, 16s**.
- Surge multipliers **[10, 100, 1000]**; regression default **24h**.
- Cloud agents: **never** `BYPASS_AUTH`.
- **File ownership / forbidden paths / per-track gates:** authoritative table in [implementation design spec § Mission tracks](../specs/2026-11-01-argus-v1-implementation-design.md#mission-tracks-3-parallel).

---

## File structure (target)

See [`docs/superpowers/specs/2026-11-01-argus-v1-implementation-design.md`](../specs/2026-11-01-argus-v1-implementation-design.md) for package layout.

---

## Track A — Normalization (merge 1)

**Branch:** `cursor/argus-v1-track-a-normalization-8966`  
**Owner:** backend implementer cloud agent

### Task A1: Module scaffold

- [ ] Create `go.mod` module `github.com/OrenVill/Argus`
- [ ] Add `Makefile` targets: `test`, `test-integration`, `lint`, `bench`, `quality:pr` (`quality:pr` runs full suite including integration when Redis is up; Track A PR gate is lint + unit scope below until B lands integration targets)
- [ ] Add `.golangci.yml` sensible defaults
- [ ] `cmd/argus/main.go` stub calling cli root (empty run ok)

### Task A2: Meta + config

- [ ] `internal/meta` — `Env`, `Service`, `ImageTag` (stable for B/C)
- [ ] `internal/config` load `config/argus.yaml.example` shape
- [ ] Env var expansion for `${VAR}`
- [ ] Validation: required meta, redis, webhook url when run mode (defer strict webhook for tests via build tag if needed)

### Task A3: Masking

- [ ] Built-in masks in fixed order per spec §4.1
- [ ] User `masking_patterns` appended
- [ ] Table tests with golden masked outputs

### Task A4: Drain

- [ ] `internal/drain` forest keyed by `(env, service)`
- [ ] `drain_depth`, `sim_threshold`, `<*>` wildcard
- [ ] Unit tests for cluster stability and new templates

### Task A5: Hash + level

- [ ] `internal/hash` xxHash64 hex
- [ ] `internal/level` JSON field + regex + normalization WARNING→WARN, PANIC→FATAL, default INFO

**Track A exit:** `go test ./internal/config/... ./internal/meta/... ./internal/mask/... ./internal/drain/... ./internal/hash/... ./internal/level/...` pass; `make lint` pass. Do **not** edit forbidden paths (see design spec track manifest).

---

## Track B — Redis detection (merge 2)

**Branch:** `cursor/argus-v1-track-b-redis-8966`  
**Depends on:** Track **A** merged to **integration** (meta, hash, config types). Optional: branch early off A’s PR and rebase before merge — still merge **A** before **B** onto integration.

### Task B1: Redis client wrapper

- [ ] `internal/redis/client.go` connect, key prefix helper
- [ ] Grace key set/get with TTL = duration + 1h buffer

### Task B2: Lua script

- [ ] `internal/redis/scripts/process_line.lua` — full §5 state machine
- [ ] `go:embed` + `Store.ProcessLine` wrapper
- [ ] `Store.SetTemplateStatus` / `SetGraceDeadline` for §5.6 and §6 (HTTP routes wired in Track C only)

### Task B3: Integration tests

- [ ] Table-driven cases in `test/integration/`:
  - NEW no grace / grace DEBUG / grace WARN+
  - ignored increment only
  - surge tiers 10/100/1000 latching
  - regression after threshold; tier reset on regression
- [ ] Use real Redis; build tag or env `ARGUS_TEST_REDIS=1`

**Track B exit:** integration tests pass with Redis; `make quality:pr` on track branch.

---

## Track C — Runtime surface (merge 3)

**Branch:** `cursor/argus-v1-track-c-runtime-8966`  
**Depends on:** Tracks A + B on `main`

### Task C1: Ingest

- [ ] File tail with glob + fsnotify + inode reopen
- [ ] Stdin optional
- [ ] Fan-in to bounded channel

### Task C2: Pipeline + daemon

- [ ] `internal/pipeline` wire mask→drain→hash→level→redis
- [ ] `internal/daemon` worker pool, drop metrics
- [ ] `argus run -c` starts ingest + workers + control + webhook pool

### Task C3: Webhook

- [ ] Payload struct matching `docs/schemas/webhook-v1.json`
- [ ] Worker pool, retries, idempotency header
- [ ] Golden JSON tests

### Task C4: Control + CLI

- [ ] HTTP routes §8 with optional bearer
- [ ] Cobra: `run`, `grace start`, `template ignore`, `template reset`

### Task C5: End-to-end smoke

- [ ] Script or test: feed lines stdin → Redis state → mock webhook on `:8765`
- [ ] Update README status from "not implemented"

**Track C exit:** `make quality:pr` green; manual smoke with `redis-cli` + webhook listener.

---

## Orchestration

| Step | Action |
| --- | --- |
| 0 | Integration branch `cursor/argus-v1-integration-8966` from `main`; all mission PRs target it |
| 1 | Merge design spec + plan PR → **integration** |
| 2 | Cloud agent Track A → PR → **integration** → `quality:pr` gate |
| 3 | Cloud agent Track B → PR → **integration** |
| 4 | Cloud agent Track C → PR → **integration** |
| 5 | `project-reviewer` on **integration** (`claude-opus-5-5-high`) |
| 6 | Single release PR **integration → `main`** |

Progress log: [`docs/plans/progress.md`](../../plans/progress.md)

---

## Verification commands

```bash
bash .cursor/install.sh
bash .cursor/start.sh
redis-cli ping
make quality:pr
go test ./... -count=1
```
