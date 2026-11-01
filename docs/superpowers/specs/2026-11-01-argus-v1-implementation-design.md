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

| Track | ID | Owns | Branch name | Merge order |
| --- | --- | --- | --- | --- |
| **A** | normalization | See **Track manifest** below | `cursor/argus-v1-track-a-normalization-8966` | **1st** |
| **B** | redis-detection | See **Track manifest** below | `cursor/argus-v1-track-b-redis-8966` | **2nd** (after **A** on `main`) |
| **C** | runtime | See **Track manifest** below | `cursor/argus-v1-track-c-runtime-8966` | **3rd** (after **A+B** on `main`) |

### Track manifest (ownership + gates)

| Track | Owned paths | Forbidden paths (do not edit) | Quality gate before PR |
| --- | --- | --- | --- |
| **A** | `go.mod`, `Makefile`, `.golangci.yml`, `cmd/argus/main.go` (stub only), `internal/config/**`, `internal/meta/**`, `internal/mask/**`, `internal/drain/**`, `internal/hash/**`, `internal/level/**`, `config/argus.yaml.example` (shape only), `testdata/logs/**` | `internal/redis/**`, `internal/ingest/**`, `internal/pipeline/**`, `internal/daemon/**`, `internal/webhook/**`, `internal/control/**`, `internal/cli/**` (beyond what stub needs), `test/integration/**`, `test/golden/**`, `docs/schemas/**` | `make lint` + `go test ./internal/config/... ./internal/mask/... ./internal/drain/... ./internal/hash/... ./internal/level/...` |
| **B** | `internal/redis/**` (incl. `scripts/process_line.lua`), `test/integration/redis_*_test.go` | `internal/ingest/**`, `internal/pipeline/**`, `internal/daemon/**`, `internal/webhook/**`, `internal/control/**`, `internal/cli/**`, `test/golden/**`; do not change A package APIs without A owner | `make quality:pr` (requires Redis; see Makefile) |
| **C** | `internal/ingest/**`, `internal/pipeline/**`, `internal/daemon/**`, `internal/webhook/**`, `internal/control/**`, `internal/cli/**`, `test/golden/**`, `README.md` (implementation status), optional `scripts/*smoke*` | `internal/mask/**`, `internal/drain/**`, `internal/hash/**`, `internal/level/**`, `internal/redis/scripts/**` (behavior changes go through B) | `make quality:pr` |

**Parallel development:** At most **two** implementer branches at once (**A** then **B**, or **B** rebased on **A**’s branch before **A** merges — optional early start). **Merge to integration is always serialized:** **A → B → C** only onto `cursor/argus-v1-integration-8966`; never merge B or C before their dependencies are on **integration**. **Do not merge track PRs to `main`.** **C** must not open until **A** and **B** are on integration.

**Release:** After all tracks and `project-reviewer` on integration, open a single PR **integration → `main`**.

**Rebase rule:** Before opening a track PR, sync `origin/cursor/argus-v1-integration-8966`, rebase the track branch, run that track’s quality gate.

## Interfaces between tracks

Track **A** owns type signatures and behavior for normalization; track **B** owns Redis/Lua and exposes `Store`; track **C** wires ingest → pipeline → webhook/control/CLI. Cross-track changes to these signatures require orchestrator approval.

```go
// internal/meta/meta.go — Track A
type Meta struct { Env, Service, ImageTag string }

// internal/config — Track A: exported Config loaded from YAML (C reads, does not fork schema)
// internal/hash/hash.go — Track A
func TemplateHash(m meta.Meta, template string) string // hex lowercase xxHash64

// internal/drain/drain.go — Track A
type Cluster struct { Template string }
func NewForest(m meta.Meta, depth int, simThreshold float64) *Forest
func (f *Forest) Match(line string) Cluster

// internal/level — Track A
func Parse(line string, cfg config.LogLevelConfig) string // DEBUG|INFO|WARN|ERROR|FATAL

// internal/redis/store.go — Track B
type ProcessInput struct {
	TemplateHash, Template, Level, NowRFC3339, MaskedSample string
	GraceDeadlineRFC3339 string // empty if no active grace
	RegressionThreshold  time.Duration
	SurgeMultipliers     []int
	TTLSeconds, SampleMax int
}
type LineResult struct {
	FireWebhook bool
	AnomalyType string // NEW_SIGNATURE | FREQUENCY_SPIKE | REGRESSION
	Count, ReportedCount int
	SurgeMultiplier int
	SampleLines []string
	FirstSeen, LastSeen string
}
func (s *Store) ProcessLine(ctx context.Context, m meta.Meta, in ProcessInput) (LineResult, error)

// Control-facing — Track B (HTTP lives in C)
func (s *Store) SetTemplateStatus(ctx context.Context, m meta.Meta, hash, status string) error // known|ignored|new
func (s *Store) SetGraceDeadline(ctx context.Context, m meta.Meta, service string, deadline time.Time, ttl time.Duration) error
```

**Module path:** `github.com/OrenVill/Argus` (matches public GitHub remote in README).

## Non-goals for implementers

- Kubernetes log input (reserved key only).
- Prometheus exporter (expvar sufficient v1).
- Changing product semantics vs `docs/SPEC.md`.

## Cloud agent rules

- Sync `origin/cursor/argus-v1-integration-8966` before each track branch (and `origin/main` only when cutting integration from main).
- Never commit implementation on `main` or push track work directly to `main`.
- Track PR **base branch:** `cursor/argus-v1-integration-8966`.
- Run `make quality:pr` before opening track PR.
- No `BYPASS_AUTH` in tests or config.
- **Models:** implementers `composer-2.5-fast`; track PR review `cursor-grok-4.6-high`; mission close `project-reviewer` on **integration** with `claude-opus-5-5-high`, then release PR to `main` (see `docs/plans/progress.md`).
