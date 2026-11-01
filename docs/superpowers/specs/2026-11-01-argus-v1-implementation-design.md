# Argus v1 — Implementation design

**Status:** draft for mission execution  
**Product spec:** [`docs/SPEC.md`](../../SPEC.md) (authoritative behavior)  
**Mission scope:** **Full v1 (A)** — all sections of the product spec before mission close.

## Decisions (brainstorm)

| Topic | Choice | Rationale |
| --- | --- | --- |
| Scope | Full v1 per `docs/SPEC.md` | Single shippable sidecar daemon |
| Drain | In-repo `internal/drain` | Exact depth, Jaccard, `<*>`, hash input alignment |
| Redis Lua | `//go:embed` under `internal/redis/scripts/` | Reviewable, testable, one atomic script per template event |
| File ingest | `nxadm/tail` + inode/dev reopen + `fsnotify` for glob dirs | Rotate-safe (rename, copytruncate, new files in glob) |
| Stdin ingest | Dedicated reader, large `bufio`, line-delimited | Sidecar-friendly |
| Config | Typed struct + `yaml.v3` + `${ENV}` expansion, validate at startup | Fail fast; no silent defaults |
| CLI | `spf13/cobra` subcommands on one binary | `run`, `grace`, `template` per spec §10 |
| HTTP control | `chi` router, std `net/http`, optional bearer middleware | Clear routes for §8 |
| Redis client | `redis/go-redis/v9` | Pipelines where safe; Lua for template hash |
| Webhook | Bounded async queue + worker pool, retry backoff | Non-blocking hot path |
| Logging | `log/slog` JSON/text | Operational clarity |
| Metrics | `expvar` counters (drops, lines, webhooks) + 1/sec stderr on drop | §9 drop policy |
| Hash | `cespare/xxhash/v2` over `{env}\0{service}\0{template}` | §4.2 |
| Quality gate | `make quality:pr` (lint + unit + integration + bench smoke) | Cloud mission verifier |

## Architecture approaches considered

1. **Layered monolith (chosen)** — Single `argus` binary; `internal/*` packages with narrow interfaces. Matches spec deployment and minimizes ops surface.
2. **Ingest plugins** — Dynamic sources; rejected for v1 (no spec requirement, delays hardening).
3. **Split control/data processes** — Rejected; spec assumes one daemon.

## Package layout

```text
cmd/argus/main.go                 # cobra root
internal/config/                  # load, env expand, validate
internal/meta/                    # env, service, image_tag
internal/mask/                    # built-in + user patterns
internal/drain/                   # Drain tree per (env, service)
internal/hash/                    # template hash
internal/level/                   # JSON field + regex normalize
internal/ingest/                  # file tail, stdin, fan-in
internal/pipeline/                # mask → drain → hash → level → redis eval
internal/redis/                   # client, grace keys, embed scripts
internal/redis/scripts/           # process_line.lua (atomic state machine)
internal/webhook/                 # queue, workers, payload, retries
internal/control/                 # HTTP API §8
internal/daemon/                  # workers, channel, lifecycle
internal/cli/                     # cobra commands
test/integration/                 # Redis + Lua table tests
test/golden/                      # webhook JSON
testdata/logs/                    # sample lines for drain/mask tests
Makefile
go.mod
```

## Data flow

```text
ingest → [bounded channel] → worker pool
  → mask → drain → hash → parse level
  → EVAL process_line.lua (grace key lookup inside or passed in)
  → if anomaly: enqueue webhook job
webhook workers → POST with idempotency header
control API → status / grace / healthz (parallel goroutine)
```

## Redis Lua contract

- **One script** `process_line.lua` performs all §5 transitions for a template hash key.
- **Arguments:** env, service, template_hash, template text, level, now RFC3339, masked sample, grace deadline (or flag), detection config snapshots (regression duration, surge multipliers JSON), ttl seconds, sample max.
- **Returns:** JSON `{ "fire_webhook": bool, "anomaly_type", "count", "reported_count", "surge_multiplier", "sample_lines", ... }` for Go to POST.
- **Grace key:** `argus:{env}:grace:{service}` read inside script via `KEYS` avoided — pass grace deadline string from Go (Go reads grace key once per line or caches with short TTL); prefer **Go reads grace, passes as ARG** to keep script key count stable.
- **Regression:** use previous `last_seen` before update (spec §5.4).
- **Surge / regression tier reset:** §5.5 regression sets `last_surge_tier=0` on REGRESSION webhook path in script.

## Ingest reliability

- Per matched file: tail goroutine, track `(dev, inode, size)`.
- On **rename/truncate/copytruncate:** reopen per tail best practices.
- **Glob directories:** periodic rescan + `fsnotify` to pick up new files under `inputs.files` globs.
- **Backpressure:** bounded channel; on drop increment `expvar` and rate-limited stderr warning (1/s).

## Webhook delivery

- Payload validated against `docs/schemas/webhook-v1.json` in tests.
- Header `X-Argus-Idempotency-Key` per §7.1.
- Retries 1s, 4s, 16s; failures logged; never block ingest workers.

## Control API

- `POST /api/v1/templates/{hash}/status` — drives §5.6 (`new` deletes hash).
- `POST /api/v1/deploy/grace-window` — sets grace Redis key with TTL buffer §6.
- `GET /healthz` — unauthenticated.

## Testing strategy (implementation)

| Layer | Location | Covers |
| --- | --- | --- |
| Unit | `internal/*/*_test.go` | mask, drain, hash, level |
| Integration | `test/integration/redis_*_test.go` | Lua table cases: NEW, grace, ignored, surge tiers, regression |
| Golden | `test/golden/webhook_*` | JSON shape |
| Bench | `internal/pipeline/*_bench_test.go` | hot path budget signal |

Integration tests require Redis (`127.0.0.1:6379`); skip if unavailable in short mode, required in `quality:pr`.

## Mission tracks (≤3 parallel)

| Track | ID | Owns | Branch prefix | Merge order |
| --- | --- | --- | --- | --- |
| **A** | normalization | `go.mod`, Makefile skeleton, config, mask, drain, hash, level, unit tests | `cursor/argus-v1-track-a-*-8966` | **1st** |
| **B** | redis-detection | `internal/redis/**`, Lua, grace keys, integration tests | `cursor/argus-v1-track-b-*-8966` | **2nd** (after A interfaces stable) |
| **C** | runtime | ingest, pipeline, daemon, webhook, control, cli, golden, wire-up | `cursor/argus-v1-track-c-*-8966` | **3rd** |

**Parallel rule:** A and B may run in parallel once shared types are defined in A’s first commit (`internal/meta`, hash string format). B must not import ingest/webhook. C starts after A+B merged to integration branch or sequentially merge A→B→main then C rebases.

**Serialized merges to `main`:** A → B → C only.

## Interfaces between tracks

```go
// internal/meta/meta.go
type Meta struct { Env, Service, ImageTag string }

// internal/hash/hash.go
func TemplateHash(m meta.Meta, template string) string

// internal/drain/drain.go
type Cluster struct { Template string }
func (f *Forest) Match(line string) Cluster

// internal/redis/store.go
type LineResult struct { FireWebhook bool; AnomalyType string; ... }
func (s *Store) ProcessLine(ctx context.Context, m meta.Meta, in ProcessInput) (LineResult, error)
```

Track A owns signatures; B implements `Store`; C owns `pipeline` calling A+B.

## Non-goals for implementers

- Kubernetes log input (reserved key only).
- Prometheus exporter (expvar sufficient v1).
- Changing product semantics vs `docs/SPEC.md`.

## Cloud agent rules

- Sync `origin/main` before each track branch.
- Never commit implementation on `main`.
- Run `make quality:pr` before opening track PR.
- No `BYPASS_AUTH` in tests or config.
