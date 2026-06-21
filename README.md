# Payment Switch

[![CI](https://github.com/paulodhiambo/payswitch/actions/workflows/ci.yaml/badge.svg)](https://github.com/paulodhiambo/payswitch/actions/workflows/ci.yaml)

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
- [Participant Portal](#participant-portal)
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
|---|---|---|
| Gateway + ISO 20022 (pacs.008/pacs.002), saga engine (10 steps), outbox, sweeper | Implemented |
| `compliance-service`, `lookup-service`, `settlement-service` | Implemented as standalone gRPC services |
| `quoting-service`, `notification-service` | Implemented as standalone gRPC services, wired into saga |
| `reconciliation-service`, `routing-service` | Implemented as standalone gRPC services with logic |
| `audit-service` | Implemented (Kafka consumer, logs lifecycle events) |
| `internal/participant`, `internal/certificate` | Implemented as libraries used by the gateway's mTLS middleware, not separate deployables |
| `portal-api` (BFF for Participant Portal) | Designed — spec at `api/openapi/portal.yaml`, architecture at `frontend.md` |

If you're picking this up to extend it, this table is the actual source of
truth on what's load-bearing versus scaffolding.

---

## Architecture

The system has two separate entry points with distinct auth mechanisms:

```
 Bank systems (mTLS)                   Human operators (browser)
         │                                       │
         ▼                                       ▼
 ┌───────────────────┐            ┌──────────────────────────┐
  │  Traefik API Gateway │            │  Authentik Outpost        │
  │  (mTLS :8443)     │            │  (session cookie + TOTP)  │
  │  • client cert    │            │  • validates session      │
  │  • rate limiting  │            │  • injects identity hdrs  │
  │  • TLS client     │            │  • redirects to login     │
  │    cert header    │            └──────────────┬────────────┘
 └────────┬──────────┘                           │
          │                                      ▼
          ▼                            ┌──────────────────────┐
 ┌─────────────────────────────┐       │  portal-api  (BFF)   │
 │  Gateway (REST :8080)       │       │  • transactions view │
 │  • POST /payments (ISO 20022│       │  • bank management   │
 │    JSON or pacs.008 XML)    │       │  • user management   │
 │  • GET  /payments/{id}      │       │  • audit log         │
 │  • GET  /healthz            │       └──────────────────────┘
 └──────────┬──────────────────┘
            │
            ▼
 ┌──────────────────────────────────────────────────────────┐
 │  Saga Orchestrator (in-process, 10 sequential steps)     │
 │  ├── ValidateStep    (input validation)                  │
 │  ├── LookupStep      (BIC resolution → gRPC)             │
 │  ├── RouteStep       (route selection → gRPC)            │
 │  ├── QuoteStep       (FX + fee quote → gRPC)             │
 │  ├── ScreenStep      (compliance/AML → gRPC)             │
 │  ├── ReserveStep     (hold funds, 5 min TTL)             │
 │  ├── CommitStep      (credit destination)                │
 │  ├── SettleStep      (net settlement → gRPC, optional)   │
 │  ├── RecordReconciliationStep (record for audit → gRPC)  │
 │  └── NotifyStep      (webhook/push → gRPC)               │
 ├──────────────────────────────────────────────────────────┤
 │  Outbox Relay (Kafka)      │  Sweeper (reservation TTL)  │
 └──────────────────────────────────────────────────────────┘

Microservices (all gRPC unless noted):
  compliance-service    — sanctions/AML screening        [called by saga]
  lookup-service        — BIC resolution + Redis cache   [called by saga]
  settlement-service    — net settlement + ScyllaDB ledger [optional step]
  quoting-service       — FX quote generation            [called by saga]
  notification-service  — participant notification        [called by saga]
  reconciliation-service — payment record matching       [called by saga]
  routing-service       — route + fee lookup             [called by saga]
  audit-service (Kafka) — event log consumer
```

mTLS authentication and participant/certificate resolution happen in the
Gateway layer (`pkg/middleware`, `internal/participant`,
`internal/certificate`) — see [Design Decisions](#design-decisions).

> Note: the saga runs 10 steps in sequence for every payment submission.
> `settlement-service` is optional — the `SettleStep` is only added when
> `SETTLEMENT_ADDR` is configured. All other steps use the in-process
> implementation unless the corresponding `*_ADDR` env var points to a
> remote gRPC service.

---

## Payment Flow

```
POST /payments
  │
  ▼
RECEIVED ──► VALIDATED ──► QUOTED ──► SCREENED ──► RESERVED ──► COMMITTED
                                                          │            │
                                                      (5min TTL)    notify
                                                          │
                                                     sweeper ──► ABORTED
                                                          (compensation)
```

All state transitions write an outbox event atomically within the same
Postgres transaction (no dual-write problem). Failed steps trigger
compensation in reverse order.

`POST /payments` returns **202 Accepted** immediately after basic validation
and persistence. The saga runs in a detached background goroutine and
delivers the final status via a **pacs.002 XML callback** to the originating
bank's registered callback URL. Callback delivery retries up to 3 times with
exponential backoff (1s, 3s, 9s). If no callback URL is configured, the
status is available via `GET /payments/{id}`.

---

## Services

| Binary | Ports (default) | Dependencies | Description |
|--------|-------|--------------|-------------|
| `gateway` | HTTP `:8080`, metrics `:9095` | Postgres, Kafka, Redis | REST/ISO 20022 API gateway, saga engine, outbox relay, reservation sweeper |
| `compliance-service` | gRPC `:9090`, metrics `:9095` | — | AML/sanctions screening stub |
| `lookup-service` | gRPC `:9090`, metrics `:9095` | Redis (optional) | BIC-to-bank resolution with cache-aside |
| `settlement-service` | gRPC `:9090`, metrics `:9095` | ScyllaDB (optional) | Net settlement windows, ledger audit trail |
| `quoting-service` | gRPC `:9090`, metrics `:9095` | — | FX quote generation with fee, 30s TTL |
| `notification-service` | gRPC `:9090`, metrics `:9095` | — | Webhook/push dispatch (stub, logs to stdout) |
| `reconciliation-service` | gRPC `:9090`, metrics `:9095` | — | Payment record matching |
| `routing-service` | gRPC `:9090`, metrics `:9095` | — | Route and fee lookup |
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
| `OTLP_ENDPOINT` | `""` | OpenTelemetry OTLP gRPC endpoint (traces, metrics, and logs) |
| `CALLBACK_TIMEOUT` | `10s` | Timeout for pacs.002 callback delivery requests |
| `METRICS_ADDR` | `:9095` | Prometheus `/metrics` HTTP address |
| `SCYLLA_HOSTS` | `""` | ScyllaDB hosts (disabled if empty) |
| `SCYLLA_KEYSPACE` | `switch` | ScyllaDB keyspace |
| `COMPLIANCE_ADDR` | `localhost:9091` | Gateway's client address for compliance-service |
| `LOOKUP_ADDR` | `localhost:9092` | Gateway's client address for lookup-service |
| `SETTLEMENT_ADDR` | `localhost:9093` | Gateway's client address for settlement-service |
| `QUOTING_ADDR` | `localhost:9094` | Gateway's client address for quoting-service (empty = in-process) |
| `NOTIFICATION_ADDR` | `""` | Gateway's client address for notification-service (empty = in-process) |
| `TLS_CERT_FILE` | `""` | *(deprecated — mTLS now handled by Traefik)* |
| `TLS_KEY_FILE` | `""` | *(deprecated — mTLS now handled by Traefik)* |
| `TLS_CA_FILE` | `""` | *(deprecated — mTLS now handled by Traefik)* |

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

The quickest way to run the full stack is:

```bash
# 1. Generate dev certificates
go run ./cmd/certgen

# 2. Start Postgres, apply schema
docker run -d --name postgres -e POSTGRES_USER=switch -e POSTGRES_PASSWORD=switch \
  -e POSTGRES_DB=switch -p 5432:5432 postgres:16-alpine
export POSTGRES_DSN="postgres://switch:switch@localhost:5432/switch?sslmode=disable"
psql "$POSTGRES_DSN" < migrations/postgres/0001_init.sql
psql "$POSTGRES_DSN" < migrations/postgres/0002_iso20022.sql

# 3. Start compliance-service on the port the gateway expects by default
GRPC_ADDR=:9091 go run ./cmd/compliance-service &

# 4. Start the gateway (plain HTTP, internal)
go run ./cmd/gateway

# 5. (Optional) Start Traefik for mTLS — or test directly on :8080 without auth
```

For the full stack (BIC lookup, settlement, quotes, notifications), start
the remaining services the same way, each on its own port matching the
gateway's `*_ADDR` config:

```bash
GRPC_ADDR=:9092 go run ./cmd/lookup-service &
GRPC_ADDR=:9093 go run ./cmd/settlement-service &
GRPC_ADDR=:9094 go run ./cmd/quoting-service &
GRPC_ADDR=:9095 go run ./cmd/notification-service &
```

Or use Docker Compose for the complete setup including Traefik mTLS:

```bash
go run ./cmd/certgen    # generates ca-*, server-*, client-* certs
cp *.pem deploy/docker/certs/
docker compose -f deploy/docker/compose.yaml up --build
```

This starts Postgres, Redis, Redpanda, Traefik (:8443 with mTLS), and all
microservices. Test with:

```bash
curl -k --cert client-bank-a-cert.pem --key client-bank-a-key.pem \
  https://localhost:8443/payments \
  -H 'Content-Type: application/json' \
  -d '{"end_to_end_id":"e2e-1","destination_bic":"BANKDEFF","dest_account":"ACC-B","amount":100,"currency":"USD"}'
```

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

In production, Traefik terminates mTLS, verifies the client certificate
against the CA, and injects the client certificate Subject CN as
`X-Forwarded-Tls-Client-Cert-Info`. The gateway parses the `CN=` field
from the header and resolves the participant ID via the participant
registry.  Locally (without Traefik), the gateway falls back to a dev-mode
participant (`bank-a`).

### Content negotiation

The gateway speaks both JSON and ISO 20022 XML:

| `Content-Type` | `Accept` | Behaviour |
|---|---|---|
| `application/json` | `application/json` (default) | JSON in, JSON out |
| `application/xml` | `application/json` | Parse pacs.008 XML, return JSON |
| `application/json` | `application/xml` | JSON in, return pacs.002 XML |
| `application/xml` | `application/xml` | Parse pacs.008, return pacs.002 |

### POST /payments

Minimal JSON request:

```json
{
  "end_to_end_id": "e2e-123",
  "destination_bic": "BANKDEFF",
  "dest_account": "ACC-B",
  "amount": 10000,
  "currency": "USD"
}
```

Optional ISO 20022 enrichment fields (auto-generated if omitted):

| Field | Default | Description |
|---|---|---|
| `uetr` | auto (UUID4) | SWIFT gpi Unique End-to-End Transaction Reference |
| `instruction_id` | auto (UUID4) | Instruction identification |
| `charge_bearer` | `"SLEV"` | `DEBT`, `CRED`, `SHAR`, or `SLEV` |
| `settlement_date` | today | ISO date (`YYYY-MM-DD`) |
| `debtor_name` | `""` | Originating party name |
| `creditor_name` | `""` | Beneficiary party name |
| `purpose_code` | `""` | ISO 20022 purpose code (e.g. `SALA`) |
| `remittance_info` | `""` | Unstructured remittance information |

Response `202` (accepted):

```json
{
  "id": "b1f2e9...",
  "uetr": "550e8400-e29b-41d4-a716-446655440000",
  "end_to_end_id": "e2e-123",
  "status": "RECEIVED",
  "iso_status": "RCVD",
  "amount": 10000,
  "currency": "USD",
  "created_at": "2026-06-20T10:00:00Z"
}
```

The saga runs asynchronously. The final status is delivered via a **pacs.002
XML callback** to the bank's registered callback URL (stored in the
`bank_api_configs` table with [`callback_url`](migrations/postgres/0008_callback_url.sql)).
If no callback URL is configured, poll `GET /payments/{id}` for the terminal
status.

`iso_status` is the pacs.002 `TxSts` code derived from the domain status:

| Domain status | `iso_status` |
|---|---|
| `RECEIVED` | `RCVD` |
| `VALIDATED` / `QUOTED` / `SCREENED` / `RESERVED` | `PDNG` |
| `COMMITTED` / `SETTLED` | `ACCP` |
| `ABORTED` | `RJCT` |

On a failed/aborted payment the same shape is returned with
`"status": "ABORTED"` and `"iso_status": "RJCT"`.

### GET /payments/{id}

Response `200` with the same payment object structure.

Full OpenAPI spec at [api/openapi/gateway.yaml](api/openapi/gateway.yaml).

---

## Participant Portal

A browser-based portal for switch administrators and bank staff. It is a
separate concern from the payment gateway — it authenticates *people*, not
systems.

| Hostname | Auth layer | Auth mechanism |
|---|---|---|
| `api.payment-switch.example.com` | Traefik (mTLS) | Client certificate |
| `portal.payment-switch.example.com` | Authentik Outpost | Session cookie + TOTP MFA |

**Authentik** acts as both IdP and reverse proxy for the portal. Its outpost
intercepts every browser request, validates the session cookie, runs the
login + TOTP flow for unauthenticated users, and injects identity headers
(`X-authentik-uid`, `X-User-Role`, `X-Participant-Id`) before forwarding to
`portal-api`. The SPA makes ordinary HTTP requests — no token library needed.

**Roles**: `SWITCH_ADMIN`, `SWITCH_OPS`, `BANK_ADMIN`, `BANK_OPERATOR`, `BANK_VIEWER`.
`BANK_*` roles are scoped to their own participant; the server enforces this
via `X-Participant-Id` — frontend RBAC checks are UX only.

**Portal API** (`cmd/portal-api`) is a Go BFF that reads from a separate
CQRS read projection (Postgres) maintained by `portal-projector` (a Kafka
consumer). It never queries the orchestrator's transactional database directly.

The SPA uses **TanStack Query** with `refetchInterval` for live-feeling
transaction views — no WebSocket.

- Architecture & auth flow: [`frontend.md`](frontend.md)
- OpenAPI spec: [`api/openapi/portal.yaml`](api/openapi/portal.yaml)
- Authentik blueprint: [`deploy/authentik/portal-blueprint.yaml`](deploy/authentik/portal-blueprint.yaml)

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
├── api/
│   ├── openapi/
│   │   ├── gateway.yaml       Payment gateway REST API (ISO 20022, bank systems)
│   │   └── portal.yaml        Participant Portal BFF API (human operators)
│   └── proto/                 Protobuf definitions + generated gRPC stubs
├── cmd/                       Service entrypoints (one main.go per binary)
├── deploy/
│   ├── authentik/             Authentik blueprint YAML (portal auth config)
│   ├── docker/                Docker Compose + Dockerfiles
│   └── k8s/                   Kubernetes manifests (kustomize)
├── frontend.md                Portal frontend & auth architecture
├── internal/
│   ├── gateway/               REST/ISO 20022 HTTP handler + content negotiation
│   ├── orchestrator/          Saga engine, steps, sweeper, sqlc repo
│   ├── compliance/            Screening service + gRPC adapter
│   ├── lookup/                BIC resolution + gRPC adapter
│   ├── settlement/            Net settlement engine + gRPC adapter
│   ├── quoting/               Quote generation + gRPC adapter
│   ├── notification/          Notification dispatch + gRPC adapter
│   ├── reconciliation/        Payment record matching + gRPC adapter
│   ├── routing/               Route + fee lookup + gRPC adapter
│   ├── audit/                 Kafka event consumer
│   ├── participant/           Registry for participants/certificates
│   └── certificate/           Certificate-based registration
├── migrations/                SQL + CQL schemas
│   └── postgres/
│       ├── 0001_init.sql              Core payment schema & outbox
│       ├── 0002_iso20022.sql          ISO 20022 enrichment columns
│       ├── 0003_portal.sql            Portal CQRS read projection
│       ├── 0004_seed.sql              Dev seed data
│       ├── 0005_route_fields.sql      Routing table columns
│       ├── 0006_bank_api_config.sql   Bank API config (base URLs)
│       ├── 0007_bank_api_seed.sql     Bank API dev seed
│       ├── 0008_callback_url.sql      Callback URL per bank
│       ├── 0009_bank_api_urls.sql     Bank API URL config
│       └── 0010_sync_payments_trigger.sql  Portal sync trigger
├── pkg/
│   ├── cache/                 Redis client
│   ├── config/                Environment config (viper)
│   ├── crypto/                ISO 20022 signature verification + replay protection
│   ├── eventbus/              Kafka producer/consumer
│   ├── iso20022/              pacs.008 / pacs.002 XML structs + domain helpers
│   ├── ledger/                ScyllaDB audit store
│   ├── metrics/               Prometheus instrumentation
│   ├── middleware/            HTTP middleware (idempotency, mTLS participant resolver)
│   ├── outbox/                Transactional outbox writer + relay
│   ├── resilience/            Circuit breaker + retry
│   └── telemetry/             OpenTelemetry tracing + structured logging
└── test/
    ├── integration/           Docker-backed integration tests (Testcontainers)
    └── load/                  k6 load test scripts
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
  When `OTLP_ENDPOINT` is configured, logs are exported via OTel as well
  (visible in Uptrace's Logs tab alongside traces and metrics).

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
5. **deploy** — rsyncs repo to VPS and runs `docker compose up --build -d`

---

## Design Decisions

- **Money** as `int64` minor units (cents, not floats)
- **Ports-and-adapters** layout for testability and swappable infrastructure
- **Hand-rolled saga** with per-step compensation (Temporal as future upgrade path)
- **Transactional outbox** for reliable event publishing (no dual-write)
- **`FOR UPDATE SKIP LOCKED`** for sweeper and outbox relay concurrency
- **sqlc** for type-safe Postgres queries (no hand-written ORM)
- **mTLS** terminated at Traefik — client certificate Subject CN is injected
  via the `X-Forwarded-Tls-Client-Cert-Info` header; the gateway parses `CN=`
  to resolve the participant identity.
- **Async saga execution**: `POST /payments` returns `202 Accepted`
  immediately after basic validation and persistence. The saga runs in a
  background goroutine and delivers the final status via pacs.002 callback
  to the bank's registered callback URL. Polling via `GET /payments/{id}`
  is available as a fallback.

---

## Known Limitations / Roadmap

- **Rate limiting**: built-in rate limiting is configured per-route with
  local (in-memory) counters. No Redis-backed distributed rate limiting or
  per-participant quotas are set up yet.
- **mTLS CA management**: client CA rotation and revocation are manual
  (no OCSP stapling or CRL distribution).
- **Circuit breaker / retry**: gRPC client calls in saga steps have no
  resilience wrapping today. `pkg/resilience` exists but is not wired into
  any caller. A downstream service outage blocks the entire saga step.
- **Reconciliation auto-match**: `RecordReconciliationStep` records each
  payment, but no automated background reconciliation job compares records
  against external statements yet.
- **Routing data / admin API**: Routes are seeded with defaults and loaded
  at startup. There is no admin API to manage routes dynamically or persist
  them to a database.
- **Saga goroutine isolation**: The saga runs in a background goroutine
  within the gateway process. If the gateway is restarted, in-flight sagas
  are lost. Consider an external saga executor (Temporal, camunda) for
  production resilience.
- **No account-lookup or routing endpoint** — `source_bic`/`destination_bic`
  are supplied directly by the caller rather than resolved by the switch,
  even though `lookup-service` and `routing-service` exist.

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