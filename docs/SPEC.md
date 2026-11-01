# Argus — Product Specification (v1.0)

Argus is a lightweight, zero-cloud **Tier 1** log observer: it tails log streams, masks high-cardinality tokens, groups lines into templates with a Drain parser, tracks template state in **Redis**, and POSTs to external webhooks only when deterministic anomaly rules fire. Tier 2 (LLM agents, Slack, issue trackers) stays out of process.

**Implementation target for this repository:** Go 1.23+, local Redis, single binary + optional sidecar deployment.

---

## 1. Goals and non-goals

### Goals

- Sub-millisecond **per-line** evaluation on the hot path (mask → template → Redis).
- **No raw unmasked** log content in Redis or outbound webhooks.
- **Deterministic** anomaly detection (no ML in Tier 1).
- Sidecar / daemon / stdin-friendly operation (Kubernetes, Docker Compose, edge).
- Manual **ignore** and **deploy grace** controls via HTTP API and CLI.

### Non-goals (v1)

- Log storage or search (Elasticsearch, Loki, etc.).
- In-process root-cause analysis or generative alerts.
- Multi-tenant SaaS control plane.
- Guaranteed exactly-once webhook delivery (at-least-once with idempotency hints).

---

## 2. Architecture

```text
[ App logs: file | stdin | container stdout ]
        │
        ▼
┌───────────────────────────────────────┐
│  Argus daemon (out of request path)   │
│  • mask → drain → hash                │
│  • Redis state machine (Lua)          │
│  • async webhook worker pool          │
│  • HTTP control API (:9090)           │
└───────────────┬───────────────────────┘
                │
     ┌──────────┴──────────┐
     ▼                     ▼
 [ Redis ]          [ Webhook URL ]
 (state only)       (Tier 2 agents)
```

| Tier | Role | Latency |
| --- | --- | --- |
| **Tier 1 (Argus)** | Parse, count, rule eval | Hot path &lt; 1 ms/line (p99 target) |
| **Tier 2 (webhook consumer)** | Investigation, tickets, chat | Async; not on log path |

**Deployment:** one Argus process (or replica set sharing Redis) per **environment + service** pair configured in `argus.yaml`. Replicas must share Redis for consistent counts; grace-window deadlines are stored in Redis (see §6.3).

---

## 3. Inputs and metadata

Each process is configured with fixed metadata (not inferred from log text):

| Field | Source | Required |
| --- | --- | --- |
| `env` | config / `ARGUS_ENV` | yes |
| `service` | config / `ARGUS_SERVICE` | yes |
| `image_tag` | config / `ARGUS_IMAGE_TAG` | no (omit from webhook if empty) |

**Log sources** (at least one enabled):

| Source | Config key | Behavior |
| --- | --- | --- |
| File tail | `inputs.files[]` | Follow paths; rotate-safe (inode-aware) |
| Stdin | `inputs.stdin: true` | Line-delimited stream |
| Future | `inputs.kubernetes` | Out of v1 scope; reserved key |

**Log level** (for grace-window severity rules):

1. If `log_level.json_field` is set, parse line as JSON when possible and read that field.
2. Else apply `log_level.regex` (default: `(?i)\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PANIC)\b`).
3. Normalize `WARNING` → `WARN`, `PANIC` → `FATAL`.
4. If unknown, treat as **INFO** for grace-window purposes only (still alert on NEW_SIGNATURE for ERROR/FATAL lines when level is known).

---

## 4. Pipeline

### 4.1 Pre-processing and masking

Apply **built-in** masks first (order fixed), then user `masking_patterns` from config. Replacement tokens:

| Pattern | Token |
| --- | --- |
| UUID / GUID | `<UUID>` |
| IPv4 / IPv6 (+ optional port) | `<IP>` |
| ISO-8601, common date/time, Unix epochs | `<TS>` |
| `0x` hex pointers | `<HEX>` |
| Email-like, `Bearer …`, basic auth patterns | `<PII>` |
| Integer / float metrics | `<NUM>` |

Masking runs on the **full line** before Drain tokenization. The **masked line** is the only form stored as samples or sent in webhooks.

### 4.2 Drain templating

- **Algorithm:** fixed-depth Drain tree per `(env, service)` in memory.
- **`drain_depth`:** max tree depth (default 4).
- **`sim_threshold`:** minimum Jaccard similarity to merge into an existing cluster (default 0.5); if no cluster matches, create a new template.
- **Wildcard:** tokens that differ within a cluster become `<*>` in the template string.
- **Template hash:** 64-bit **xxHash64** (hex lowercase), digest input:

  ```text
  {env}\0{service}\0{template_string}
  ```

### 4.3 Redis data model

**Key:** `{key_prefix}{env}:{service}:{template_hash}`  
Example: `argus:prod:payment-svc:a1b2c3d4e5f67890`

**Type:** Redis **HASH**

| Field | Type | Description |
| --- | --- | --- |
| `status` | string | `known` or `ignored` only (`new` is never stored) |
| `first_seen` | string | RFC3339 UTC |
| `last_seen` | string | RFC3339 UTC |
| `count` | integer string | Total occurrences |
| `reported_count` | integer string | `count` at last webhook for this template |
| `last_surge_tier` | integer string | Highest surge multiplier tier already alerted (0, 10, 100, 1000) |
| `template` | string | Masked template text |
| `level` | string | Last seen level (enum) |
| `samples` | string | JSON array of up to `sample_lines_max` masked lines |

**TTL:** on every update, `EXPIRE key ttl_days * 86400`.

**Concurrency:** all read-modify-write for a template MUST run in a single **Lua script** (atomic).

---

## 5. Anomaly detection state machine

```text
                    masked line + hash
                            │
                            ▼
                   ┌────────────────┐
                   │  Redis HASH    │
                   │  exists?       │
                   └───────┬────────┘
                           │
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
      [ no record ]   status=known    status=ignored
           │               │               │
           │               │               └── increment count only
           │               │
           ▼               ▼
    see §5.1 NEW      see §5.2 KNOWN
```

### 5.1 First sighting (no Redis record)

| Condition | Action |
| --- | --- |
| Deploy grace active **and** level ∈ {DEBUG, INFO} | Create hash: `status=known`, `count=1`, no webhook |
| Deploy grace active **and** level ∈ {WARN, ERROR, FATAL} | Create hash, fire **NEW_SIGNATURE**, set `reported_count=1`, `last_surge_tier=0` |
| No grace | Fire **NEW_SIGNATURE**, create `status=known`, `count=1`, `reported_count=1` |

After handling, set `first_seen`, `last_seen`, `template`, `level`, append masked sample.

### 5.2 Known template

1. Increment `count`.
2. Update `last_seen`, `level`, append sample (trim to max).
3. If deploy grace active and level ∈ {DEBUG, INFO}: **stop** (no further rules).
4. Evaluate **regression** (§5.3) using **previous** `last_seen` before writing the new timestamp.
5. Else evaluate **surge** (§5.4) against `count` and `reported_count` / `last_surge_tier`.
6. If a rule fires: POST webhook, set `reported_count = count`, update `last_surge_tier` when surge.

### 5.3 Ignored template

Increment `count`, update `last_seen` and samples. **Never** webhook.

### 5.4 Regression rule

On a **known** template, when processing a new line:

```text
if (now - parse(last_seen)) >= regression_threshold   // default 24h
   → anomaly_type = REGRESSION
```

Use the stored `last_seen` from before this event; then set `last_seen = now`.

### 5.5 Surge rule

Configured multipliers: `[10, 100, 1000]` (strictly increasing).

For each tier `M` greater than `last_surge_tier`:

```text
if count >= reported_count * M   (and reported_count >= 1)
   → fire FREQUENCY_SPIKE (include M in payload metadata)
   → set last_surge_tier = M, reported_count = count
   → stop after first fired tier in this event
```

Do not re-fire the same tier until `reported_count` is reset by a webhook-fired event that sets a new baseline (regression does not reset tiers unless specified: **regression resets `last_surge_tier` to 0** when REGRESSION webhook fires).

### 5.6 Status API semantics

| API value | Meaning |
| --- | --- |
| `known` | Normal tracking (default after first line) |
| `ignored` | Count only, no webhooks |
| `new` | **Admin reset:** delete hash; next line behaves as first sighting |

---

## 6. Deploy grace window

**Purpose:** suppress NEW_SIGNATURE noise for benign levels during rollouts.

| Trigger | Mechanism |
| --- | --- |
| HTTP `POST /api/v1/deploy/grace-window` | Sets grace deadline for `service` |
| CLI `argus grace start --service … --duration 30m` | Same |

**Storage:** Redis key `argus:{env}:grace:{service}` → RFC3339 deadline (TTL = duration + 1h buffer).

**While `now < deadline`:**

- NEW templates at DEBUG/INFO → `known` without webhook.
- NEW at WARN+ → webhook immediately.
- KNOWN templates at DEBUG/INFO → no surge/regression webhooks.

---

## 7. Webhooks

### 7.1 Delivery

- **Method:** POST JSON to `webhook.url`.
- **Timeout:** configurable (default 5s).
- **Retries:** 3 attempts, exponential backoff (1s, 4s, 16s); failures logged, not blocking ingest.
- **Idempotency header:** `X-Argus-Idempotency-Key: {env}:{service}:{hash}:{anomaly_type}:{count}`

### 7.2 Payload

Required fields:

| Field | Type | Notes |
| --- | --- | --- |
| `env` | string | |
| `service` | string | |
| `template_hash` | string | hex xxHash64 |
| `template` | string | masked |
| `level` | string | DEBUG, INFO, WARN, ERROR, FATAL |
| `anomaly_type` | string | NEW_SIGNATURE, FREQUENCY_SPIKE, REGRESSION |
| `count` | integer | |
| `reported_count` | integer | prior baseline when relevant |
| `sample_lines` | string[] | masked only |
| `first_seen` | string | RFC3339 |
| `last_seen` | string | RFC3339 |

Optional: `image_tag`, `surge_multiplier` (int, for FREQUENCY_SPIKE).

Schema: `docs/schemas/webhook-v1.json`.

---

## 8. Control API

- **Listen:** `control.listen` (default `127.0.0.1:9090`).
- **Auth:** optional `control.bearer_token`; if set, require `Authorization: Bearer …`.
- **Endpoints:**

| Method | Path | Body | Effect |
| --- | --- | --- | --- |
| POST | `/api/v1/templates/{hash}/status` | `{"status":"known"\|"ignored"\|"new"}` | §5.6 |
| POST | `/api/v1/deploy/grace-window` | `{"service":"…","duration":"30m"}` | §6 |
| GET | `/healthz` | — | `200 ok` (no auth) |

---

## 9. Configuration

Canonical example: `config/argus.yaml.example`. Environment variable expansion supported for secrets (webhook URL).

| Section | Purpose |
| --- | --- |
| `daemon.workers` | Goroutines consuming line queue |
| `daemon.buffer_capacity` | Bounded channel size; drop policy: count metric + stderr sample (1/sec max) |
| `redis.*` | Connection, prefix, TTL |
| `normalization.*` | Drain + extra masks |
| `detection.*` | Grace, regression, surge list |
| `webhook.*` | URL, timeout |
| `control.*` | Listen address, token |
| `inputs.*` | File list, stdin |
| `meta.*` | env, service, image_tag |
| `log_level.*` | JSON field + regex |

---

## 10. CLI (v1)

```text
argus run -c argus.yaml          # start daemon
argus grace start --service S --duration 30m
argus template ignore --hash H
argus template reset --hash H    # same as status=new
```

---

## 11. Non-functional requirements

| Metric | Target |
| --- | --- |
| Throughput | ≥ 10,000 lines/sec per CPU core (benchmarked with Redis on localhost) |
| Hot-path latency | p99 &lt; 1 ms from line read to Redis ACK (excluding webhook) |
| RSS | &lt; 50 MB for binary + bounded Drain forest per service |
| Privacy | No unmasked lines in Redis or webhook JSON |

**Redis optimization:** pipeline non-dependent updates where safe; template evaluation remains Lua-atomic per hash.

---

## 12. Testing strategy (v1)

- Unit: masking, Drain clustering, hash stability.
- Integration: Redis Lua transitions (table-driven cases for NEW, surge tiers, regression, grace, ignored).
- Golden files for webhook JSON.

---

## 13. Revision history

| Version | Changes |
| --- | --- |
| 1.0 (draft) | Initial product brief |
| **1.0 (this doc)** | Resolved Redis shape, surge latching, regression timing, grace storage, log level extraction, `new` status semantics, sample masking, Go implementation target |
