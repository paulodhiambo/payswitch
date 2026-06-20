# Payment Switch

A real-time payment switch built in Go using a hexagonal (ports-and-adapters) architecture with a hand-rolled saga orchestration pattern.

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
  compliance-service  (gRPC :9090)  — sanctions/AML screening
  lookup-service      (gRPC :9090)  — BIC resolution + Redis cache
  settlement-service  (gRPC :9090)  — net settlement windows + ScyllaDB ledger
  quoting-service     (gRPC :9090)  — FX quote generation
  notification-service(gRPC :9090)  — participant notification dispatch
  audit-service       (Kafka)       — event log consumer
```

### Payment Flow

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

All state transitions write an outbox event atomically within the same Postgres transaction (no dual-write problem). Failed steps trigger compensation in reverse order.

## Services

| Binary | Ports | Dependencies | Description |
|--------|-------|--------------|-------------|
| `gateway` | HTTP `:8080`, metrics `:9095` | Postgres, Kafka, Redis | REST API gateway, saga engine, outbox relay, reservation sweeper |
| `compliance-service` | gRPC `:9090`, metrics `:9095` | — | AML/sanctions screening stub |
| `lookup-service` | gRPC `:9090`, metrics `:9095` | Redis (optional) | BIC-to-bank resolution with cache-aside |
| `settlement-service` | gRPC `:9090`, metrics `:9095` | ScyllaDB (optional) | Net settlement windows, ledger audit trail |
| `quoting-service` | gRPC `:9090`, metrics `:9095` | — | FX quote generation with fee, 30s TTL |
| `notification-service` | gRPC `:9090`, metrics `:9095` | — | Webhook/push dispatch (stub, logs to stdout) |
| `audit-service` | — | Kafka | Payment event consumer, logs lifecycle events |
| `certgen` | — (CLI) | — | Dev mTLS certificate generator |

## Configuration

All config via environment variables (see [pkg/config/config.go](pkg/config/config.go)):

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8080` | Gateway HTTP listen address |
| `GRPC_ADDR` | `:9090` | gRPC listen address (all services) |
| `POSTGRES_DSN` | `postgres://switch:switch@localhost:5432/switch?sslmode=disable` | Postgres connection |
| `REDIS_ADDR` | `""` | Redis address (disabled if empty) |
| `KAFKA_BROKERS` | `""` | Kafka/Redpanda broker list (disabled if empty) |
| `OTLP_ENDPOINT` | `""` | OpenTelemetry OTLP gRPC endpoint |
| `METRICS_ADDR` | `:9095` | Prometheus `/metrics` HTTP address |
| `SCYLLA_HOSTS` | `""` | ScyllaDB hosts (disabled if empty) |
| `SCYLLA_KEYSPACE` | `switch` | ScyllaDB keyspace |
| `COMPLIANCE_ADDR` | `localhost:9091` | Compliance service gRPC address |
| `LOOKUP_ADDR` | `localhost:9092` | Lookup service gRPC address |
| `SETTLEMENT_ADDR` | `localhost:9093` | Settlement service gRPC address |
| `TLS_CERT_FILE` | `""` | TLS cert path (plain HTTP if empty) |
| `TLS_KEY_FILE` | `""` | TLS key path |
| `TLS_CA_FILE` | `""` | CA cert for mTLS client verification |

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

Produces `bin/gateway`, `bin/compliance-service`, `bin/lookup-service`, `bin/settlement-service`, `bin/quoting-service`, `bin/notification-service`, `bin/audit-service`, `bin/certgen`.

### Run the gateway locally

```bash
# Start infrastructure
docker run -d --name postgres -e POSTGRES_USER=switch -e POSTGRES_PASSWORD=switch -e POSTGRES_DB=switch -p 5432:5432 postgres:16-alpine

# Apply schema
psql "$POSTGRES_DSN" < migrations/postgres/0001_init.sql

# Start gateway
export POSTGRES_DSN="postgres://switch:switch@localhost:5432/switch?sslmode=disable"
go run ./cmd/gateway
```

### Generate dev mTLS certificates

```bash
go run ./cmd/certgen
```

Produces `ca.pem`, `server.pem`, `server-key.pem`, `client.pem`, `client-key.pem`.

### Load tests

```bash
make load-smoke    # quick smoke test
make load-stress   # 100 VUs stress test
make load-soak     # 30 VUs / 10 min soak test
```

## API

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

Response `201`:
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

### GET /payments/{id}

Response `200` with same payment object structure.

Full OpenAPI spec at [api/openapi/gateway.yaml](api/openapi/gateway.yaml).

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

## Project Layout

```
├── api/               OpenAPI spec + protobuf definitions + generated stubs
├── cmd/               Service entrypoints (11 binaries)
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
│   ├── middleware/     HTTP middleware (idempotency, mTLS)
│   ├── outbox/        Transactional outbox writer + relay
│   ├── resilience/    Circuit breaker + retry
│   └── telemetry/     OpenTelemetry tracing + structured logging
└── test/
    ├── integration/   Docker-backed integration tests
    └── load/          k6 load test scripts
```

## Kubernetes

Deploy to k8s (requires kustomize):

```bash
kubectl apply -k deploy/k8s/
```

Creates 10 Deployments, 9 Services, Postgres/Redis/Redpanda StatefulSets, HPA, KEDA ScaledObject (Kafka lag-based autoscaling), and NetworkPolicies in the `payment-switch` namespace.

## CI

GitHub Actions workflow (`.github/workflows/ci.yaml`):

1. **lint** — `go vet` + staticcheck
2. **test** — unit tests with `-race` + build all binaries
3. **test-integration** — Docker-backed integration tests
4. **docker** — Buildx matrix build + push to GHCR (main branch only)

## Design Decisions

- **Money** as `int64` minor units (cents, not floats)
- **Ports-and-adapters** layout for testability and swappable infrastructure
- **Hand-rolled saga** with per-step compensation (Temporal as future upgrade path)
- **Transactional outbox** for reliable event publishing (no dual-write)
- **`FOR UPDATE SKIP LOCKED`** for sweeper and outbox relay concurrency
- **sqlc** for type-safe Postgres queries (no hand-written ORM)
- **mTLS** for gateway authentication via client certificate thumbprints
