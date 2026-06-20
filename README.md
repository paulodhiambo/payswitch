# Payment Switch

A real-time payment switch built in Go using a hexagonal (ports-and-adapters)
architecture with a hand-rolled saga orchestration pattern. It accepts
ISO20022-style payment instructions, runs them through validation,
compliance screening, fund reservation, and commit — with automatic
compensation on failure and a transactional outbox for reliable event
publishing.

## Table of Contents

- [Implementation Status](#implementation-status)
- [Architecture](#architecture)
- [Payment Flow](#payment-flow)
- [Services](#services)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
- [API](#api)
- [Protobuf Services](#protobuf-services)
- [Project Layout](#project-layout)
- [Observability](#observability)
- [Kubernetes](#kubernetes)
- [CI](#ci)
- [Design Decisions](#design-decisions)
- [Known Limitations / Roadmap](#known-limitations--roadmap)
- [Contributing](#contributing)
- [License](#license)

---

## Implementation Status

| Component | Status |
|---|---|
| Gateway, saga engine (Validate/Screen/Reserve/Commit), outbox, sweeper | Implemented |
| `compliance-service`, `lookup-service`, `settlement-service` | Implemented as standalone gRPC services |
| `quoting-service`, `notification-service` | Implemented as standalone gRPC services, **not yet called by the saga** — see [Known Limitations](#known-limitations--roadmap) |
| `audit-service` | Implemented (Kafka consumer, logs lifecycle events) |
| `internal/participant`, `internal/certificate` | Implemented as libraries used by the gateway's mTLS middleware, not separate deployables |
| `internal/reconciliation`, `internal/routing` | **Stubs** — no logic yet |

If you're picking this up to extend it, this table is the actual source of
truth on what's load-bearing versus scaffolding.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Gateway (REST API :8080)                                │
│  ├── POST /payments   (submit payment → saga)           │
│  ├── GET  /payments/{id}  (status lookup)               │
│  └── GET  /healthz   (k8s probe)                        │
├─────────────────────────────────────────────────────────┤
│  Saga Orchestrator (in-process)                         │
│  ├── ValidateStep    (input validation)                 │
│  ├── ScreenStep      (compliance/AML → gRPC)           │
│  ├── ReserveStep     (hold funds, 5min TTL)             │
│  └── CommitStep      (credit destination)               │
├─────────────────────────────────────────────────────────┤
│  Outbox Relay (Kafka)       │  Sweeper (reservation TTL)│
└─────────────────────────────────────────────────────────┘

Microservices:
  compliance-service  (gRPC)  — sanctions/AML screening        [called by saga]
  lookup-service      (gRPC)  — BIC resolution + Redis cache    [standalone]
  settlement-service  (gRPC)  — net settlement windows + ScyllaDB ledger  [standalone]
  quoting-service     (gRPC)  — FX quote generation             [standalone, not yet in saga]
  notification-service(gRPC)  — participant notification dispatch [standalone, not yet in saga]
  audit-service       (Kafka) — event log consumer
```

mTLS authentication and participant/certificate resolution happen in the
Gateway layer (`pkg/middleware`, `internal/participant`,
`internal/certificate`) — see [Design Decisions](#design-decisions).

> Note: the saga's four steps are the only ones currently in the critical
> path of `POST /payments`. `lookup-service`, `settlement-service`,
> `quoting-service`, and `notification-service` run as independent,
> callable gRPC services, but only `compliance-service` is actually invoked
> during payment submission today.

---

## Payment Flow

```
POST /payments
  │
  ▼
RECEIVED ──► VALIDATED ──► SCREENED ──► RESERVED ──► COMMITTED
                                              │
                                          (5min TTL)
                                              │
                                          sweeper ──► ABORTED
                                              (compensation)
```

All state transitions write an outbox event atomically within the same
Postgres transaction (no dual-write problem). Failed steps trigger
compensation in reverse order.

`POST /payments` currently executes the saga **synchronously** — the HTTP
response doesn't return until the payment reaches `COMMITTED` or
`ABORTED`. This is a deliberate v1 simplification; see
[Design Decisions](#design-decisions) for the trade-off and when to revisit
it.

---

## Services

| Binary | Ports (default) | Dependencies | Description |
|--------|-------|--------------|-------------|
| `gateway` | HTTP `:8080`, metrics `:9095` | Postgres, Kafka, Redis | REST API gateway, saga engine, outbox relay, reservation sweeper |
| `compliance-service` | gRPC `:9090`, metrics `:9095` | — | AML/sanctions screening stub |
| `lookup-service` | gRPC `:9090`, metrics `:9095` | Redis (optional) | BIC-to-bank resolution with cache-aside |
| `settlement-service` | gRPC `:9090`, metrics `:9095` | ScyllaDB (optional) | Net settlement windows, ledger audit trail |
| `quoting-service` | gRPC `:9090`, metrics `:9095` | — | FX quote generation with fee, 30s TTL |
| `notification-service` | gRPC `:9090`, metrics `:9095` | — | Webhook/push dispatch (stub, logs to stdout) |
| `audit-service` | — | Kafka | Payment event consumer, logs lifecycle events |
| `certgen` | — (CLI) | — | Dev mTLS certificate generator |

> **Port note:** every service binary defaults its gRPC listener to
> `GRPC_ADDR=:9090`. That's fine when each runs in its own container/pod,
> but if you run several on one machine for local dev, you must override
> `GRPC_ADDR` per process so they don't collide — see
> [Quick Start](#quick-start) for the exact commands. The gateway's
> `COMPLIANCE_ADDR` / `LOOKUP_ADDR` / `SETTLEMENT_ADDR` settings are its
> *client-side* addresses and must match wherever each downstream service
> actually ends up listening.

---

## Configuration

All config via environment variables (see [pkg/config/config.go](pkg/config/config.go)):

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8080` | Gateway HTTP listen address |
| `GRPC_ADDR` | `:9090` | gRPC listen address — same default for every service binary; override per-process for local multi-service runs |
| `POSTGRES_DSN` | `postgres://switch:switch@localhost:5432/switch?sslmode=disable` | Postgres connection |
| `REDIS_ADDR` | `""` | Redis address (disabled if empty) |
| `KAFKA_BROKERS` | `""` | Kafka/Redpanda broker list (disabled if empty) |
| `OTLP_ENDPOINT` | `""` | OpenTelemetry OTLP gRPC endpoint |
| `METRICS_ADDR` | `:9095` | Prometheus `/metrics` HTTP address |
| `SCYLLA_HOSTS` | `""` | ScyllaDB hosts (disabled if empty) |
| `SCYLLA_KEYSPACE` | `switch` | ScyllaDB keyspace |
| `COMPLIANCE_ADDR` | `localhost:9091` | Gateway's client address for compliance-service |
| `LOOKUP_ADDR` | `localhost:9092` | Gateway's client address for lookup-service |
| `SETTLEMENT_ADDR` | `localhost:9093` | Gateway's client address for settlement-service |
| `TLS_CERT_FILE` | `""` | TLS cert path (plain HTTP if empty) |
| `TLS_KEY_FILE` | `""` | TLS key path |
| `TLS_CA_FILE` | `""` | CA cert for mTLS client verification |

A binary only reads the variables it actually uses — `lookup-service`
ignores `POSTGRES_DSN`, for instance. There's no per-service config
validation that warns you about irrelevant vars being set, so don't take
an unused variable in your `.env` as a sign something's misconfigured.

---

## Quick Start

### Prerequisites

- Go 1.26+
- Docker Desktop (for integration tests)
- [k6](https://k6.io/) (for load tests, optional)

### Run unit tests

```bash
make test
```

### Run integration tests (requires Docker)

```bash
make test-integration
```

Starts Postgres 16, Redpanda v24.3.4, and Redis 7 via testcontainers-go.

### Build all binaries

```bash
make build
```

Produces `bin/gateway`, `bin/compliance-service`, `bin/lookup-service`,
`bin/settlement-service`, `bin/quoting-service`,
`bin/notification-service`, `bin/audit-service`, `bin/certgen`.

### Run a working local stack

A bare `gateway` process alone **cannot** complete a payment — `ScreenStep`
makes a gRPC call to `compliance-service`, and without it listening on
`COMPLIANCE_ADDR` (`localhost:9091` by default), submission fails at the
`SCREENED` stage. Minimum viable local stack:

```bash
# 1. Start Postgres
docker run -d --name postgres -e POSTGRES_USER=switch -e POSTGRES_PASSWORD=switch \
  -e POSTGRES_DB=switch -p 5432:5432 postgres:16-alpine

# 2. Apply schema
export POSTGRES_DSN="postgres://switch:switch@localhost:5432/switch?sslmode=disable"
psql "$POSTGRES_DSN" < migrations/postgres/0001_init.sql

# 3. Start compliance-service on the port the gateway expects by default
GRPC_ADDR=:9091 go run ./cmd/compliance-service &

# 4. Start the gateway
go run ./cmd/gateway
```

For the full stack (BIC lookup, settlement, quotes, notifications), start
the remaining services the same way, each on its own port matching the
gateway's `*_ADDR` config:

```bash
GRPC_ADDR=:9092 go run ./cmd/lookup-service &
GRPC_ADDR=:9093 go run ./cmd/settlement-service &
GRPC_ADDR=:9094 go run ./cmd/quoting-service &
GRPC_ADDR=:9095 go run ./cmd/notification-service &   # NB: pick a free port — :9095 is also the default metrics port, override METRICS_ADDR too if running both on one host
```

Remember: `quoting-service` and `notification-service` will be running and
reachable, but nothing in the saga calls them yet (see
[Implementation Status](#implementation-status)).

### Generate dev mTLS certificates

```bash
go run ./cmd/certgen
```

Produces `ca.pem`, `server.pem`, `server-key.pem`, `client.pem`, `client-key.pem`.

### Load tests

```bash
make load-smoke    # baseline correctness at low concurrency
make load-stress   # 100 VUs — find the breaking point
make load-soak     # 30 VUs / 10 min — sustained run, watch for leaks/drift
```

---

## API

### Headers

| Header | Required | Description |
|---|---|---|
| `Idempotency-Key` | Yes | Client-generated key. A retried request with the same key returns the original response instead of reprocessing the payment. |

In production, the gateway terminates mTLS (`TLS_CA_FILE`) and resolves
the caller's `participant_id` from the client certificate thumbprint
(`internal/participant`, `internal/certificate`). Locally, leave the
`TLS_*` vars unset to run over plain HTTP.

### POST /payments

```json
{
  "end_to_end_id": "e2e-123",
  "source_bic": "BANKUS33",
  "destination_bic": "BANKDEFF",
  "source_account": "ACC-A",
  "dest_account": "ACC-B",
  "amount": 10000,
  "currency": "USD"
}
```

Response `201` (committed):
```json
{
  "id": "pay_abc123",
  "end_to_end_id": "e2e-123",
  "status": "COMMITTED",
  "source_bic": "BANKUS33",
  "destination_bic": "BANKDEFF",
  "amount": 10000,
  "currency": "USD",
  "created_at": "2026-06-20T10:00:00Z"
}
```

On a failed/aborted payment, the same resource shape is returned with
`"status": "ABORTED"`. Whether the response includes a reason field (e.g.
`abort_reason`) depends on the gateway's response DTO in
`internal/gateway` — confirm against the code and document the exact
shape here once verified, rather than relying on this README's word for
it.

### GET /payments/{id}

Response `200` with the same payment object structure.

Full OpenAPI spec at [api/openapi/gateway.yaml](api/openapi/gateway.yaml).

---

## Protobuf Services

All gRPC definitions in [api/proto/](api/proto/):

| Service | RPCs | File |
|---------|------|------|
| `Compliance` | `Screen` | [compliance.proto](api/proto/compliance.proto) |
| `Lookup` | `ResolveBIC`, `SearchBanks` | [lookup.proto](api/proto/lookup.proto) |
| `Quoting` | `GetQuote`, `AcceptQuote` | [quoting.proto](api/proto/quoting.proto) |
| `Notification` | `Notify` | [notification.proto](api/proto/notification.proto) |
| `Settlement` | `SubmitForSettlement`, `SettleWindow`, `GetNetPositions` | [settlement.proto](api/proto/settlement.proto) |
| `Orchestrator` | `SubmitPayment`, `GetPaymentStatus`, `ListPayments`, `CompensatePayment` | [orchestrator.proto](api/proto/orchestrator.proto) |

Regenerate stubs:

```bash
make proto
```

---

## Project Layout

```
├── api/               OpenAPI spec + protobuf definitions + generated stubs
├── cmd/               Service entrypoints (8 binaries)
├── deploy/            Dockerfile + Kubernetes manifests (kustomize)
├── internal/          Hexagonal architecture
│   ├── gateway/           REST HTTP handler
│   ├── orchestrator/      Saga engine, steps, sweeper, sqlc repo
│   ├── compliance/        Screening service + gRPC adapter
│   ├── lookup/            BIC resolution + gRPC adapter
│   ├── settlement/        Net settlement engine + gRPC adapter
│   ├── quoting/           Quote generation + gRPC adapter
│   ├── notification/      Notification dispatch + gRPC adapter
│   ├── audit/             Kafka event consumer
│   ├── participant/       Registry for participants/certificates
│   ├── certificate/       Certificate-based registration
│   ├── reconciliation/    Stub
│   └── routing/           Stub
├── migrations/        SQL + CQL schemas
├── pkg/               Shared libraries
│   ├── cache/         Redis client
│   ├── config/        Environment config (viper)
│   ├── crypto/        ISO20022 signature verification + replay protection
│   ├── eventbus/      Kafka producer/consumer
│   ├── iso20022/      XML structs for pacs.008 / pacs.002
│   ├── ledger/        ScyllaDB audit store
│   ├── metrics/       Prometheus instrumentation
│   ├── middleware/    HTTP middleware (idempotency, mTLS)
│   ├── outbox/        Transactional outbox writer + relay
│   ├── resilience/    Circuit breaker + retry
│   └── telemetry/     OpenTelemetry tracing + structured logging
└── test/
    ├── integration/   Docker-backed integration tests
    └── load/          k6 load test scripts
```

---

## Observability

- **Metrics**: every binary exposes Prometheus metrics on `METRICS_ADDR`
  (`:9095` by default) — scrape each service independently.
- **Tracing**: set `OTLP_ENDPOINT` to ship spans to an OpenTelemetry
  collector. Tag spans with `end_to_end_id` when adding instrumentation to
  new code paths, so a payment's full saga is traceable as one trace.
- **Logs**: structured logging via `pkg/telemetry` — correlate with
  `end_to_end_id`, not the internal payment UUID, since that's what
  participants reference when they ask about a specific payment.

---

## Kubernetes

Deploy to k8s (requires kustomize):

```bash
kubectl apply -k deploy/k8s/
```

Creates **7 Deployments** (`gateway` + the 6 microservices — `certgen` is
a dev-only CLI, not deployed), **6 Services** exposing inbound ports
(`audit-service` is a pure Kafka consumer with no inbound traffic, so it
doesn't need one), plus Postgres/Redis/Redpanda StatefulSets, an HPA, a
KEDA ScaledObject (Kafka-lag-based autoscaling), and NetworkPolicies in the
`payment-switch` namespace.

> The previous version of this README claimed 10 Deployments / 9
> Services — that didn't match the 8 documented binaries (7 deployable +
> `certgen`). Counts above are derived from the binaries actually listed
> in this doc; reconcile against `deploy/k8s/` directly if the manifests
> have since diverged.

---

## CI

GitHub Actions workflow (`.github/workflows/ci.yaml`):

1. **lint** — `go vet` + staticcheck
2. **test** — unit tests with `-race` + build all binaries
3. **test-integration** — Docker-backed integration tests
4. **docker** — Buildx matrix build + push to GHCR (main branch only)

---

## Design Decisions

- **Money** as `int64` minor units (cents, not floats)
- **Ports-and-adapters** layout for testability and swappable infrastructure
- **Hand-rolled saga** with per-step compensation (Temporal as future upgrade path)
- **Transactional outbox** for reliable event publishing (no dual-write)
- **`FOR UPDATE SKIP LOCKED`** for sweeper and outbox relay concurrency
- **sqlc** for type-safe Postgres queries (no hand-written ORM)
- **mTLS** for gateway authentication via client certificate thumbprints
- **Synchronous saga execution**: `POST /payments` blocks until
  `COMMITTED`/`ABORTED` rather than returning `202 Accepted` and making the
  client poll. Chosen for simplicity in v1. Revisit if/when `quoting-service`
  or `notification-service` get wired into the critical path — an extra
  network hop or two inside a synchronous HTTP request is a different
  latency budget than today's 4-step saga.

---

## Known Limitations / Roadmap

- `internal/reconciliation` and `internal/routing` are stubs with no logic.
- `quoting-service` and `notification-service` run as real gRPC services
  but aren't called anywhere in the current saga — either wire them in as
  additional steps (`QuoteStep`, `NotifyStep`) or be explicit in docs/specs
  that they're not yet part of the payment lifecycle.
- The synchronous saga model means a slow downstream gRPC call (e.g.
  compliance) adds directly to `POST /payments` latency. Fine at current
  scale; reconsider if average screening latency grows or more steps are
  added to the path.
- No documented account-lookup or routing endpoint yet — `source_bic`/
  `destination_bic` are supplied directly by the caller rather than
  resolved by the switch, even though `lookup-service` exists.

---

## Contributing

### Branch naming

- `feat/<description>` — new features
- `fix/<description>` — bug fixes
- `docs/<description>` — documentation only
- `chore/<description>` — CI, tooling, refactors with no behaviour change

Use hyphens as separators (e.g. `feat/add-settlement-step`).

### PR workflow

1. Create a branch from `main`.
2. Make your changes.
3. Run the required checks locally:

   ```bash
   make vet          # go vet ./...
   make test         # unit tests with -race
   ```

4. If your changes touch integration-testable code, also run:

   ```bash
   make test-integration   # requires Docker
   ```

5. Open a PR against `main` with a clear description of what changed and
   why. The CI pipeline (`lint` → `test` → `test-integration`) must pass
   before merging.

### Required checks for merge

| Check | What it runs | Fail = block merge |
|-------|-------------|--------------------|
| Lint  | `go vet ./...` + `staticcheck` | Yes |
| Test  | `go test ./cmd/... ./internal/... ./pkg/... -race -count=1` | Yes |
| Build | `make build` (all 11 binaries) | Yes |
| Integration | `go test ./test/integration/... -race -timeout=180s` | Yes |
| Proto | Generated stubs are up to date with `.proto` files | Reviewer discretion |

### Code owners

A `CODEOWNERS` file at `.github/CODEOWNERS` defines who must review
changes to sensitive paths:

```
# Default owners (everyone on the team)
* @team-leads

# Saga orchestration — incorrect state transitions could lose money
/internal/orchestrator/  @core-payments

# Transactional outbox — data loss risk
/pkg/outbox/  @core-payments

# ISO20022 / crypto — regulatory correctness
/pkg/iso20022/  @compliance-lead
/pkg/crypto/    @compliance-lead

# ScyllaDB ledger — audit trail integrity
/pkg/ledger/  @core-payments

# Kubernetes manifests — production reliability
/deploy/k8s/  @platform-team

# gRPC + REST specs — API contract
/api/  @api-owners
```

Add or adjust the team aliases to match your GitHub org.

## License

MIT. See [LICENSE](LICENSE).