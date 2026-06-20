# Payment Switch — Golang Implementation Architecture

This translates the refined architecture into concrete Go decisions: project
layout, interfaces, and code for the pieces that are easy to get wrong
(outbox, saga, idempotency, TTL sweep, resilience).

> Code below is illustrative, not a finished PR — it's close to
> compilable and shows the actual pattern, but elides some helper types and
> error handling for brevity. Treat it as a strong starting skeleton.

---

## 0. Technology Decisions

| Concern | Choice | Why |
|---|---|---|
| External (bank-facing) API | REST/JSON + ISO20022 XML over HTTPS, mTLS | Banks integrate via ISO20022; REST keeps onboarding simple |
| Internal service-to-service | gRPC | Type-safe, fast, idiomatic for Go-to-Go calls |
| Async events | Kafka via `segmentio/kafka-go` | Pure Go, no cgo, matches the event-bus decision from the architecture doc |
| Config/participant store | PostgreSQL via `pgx/v5` + `sqlc` | Compile-time-checked SQL, no ORM magic |
| Ledger store | ScyllaDB via `gocql` | High write throughput, explicit partition control |
| Cache | Redis via `go-redis/v9` | Cache-aside for lookup/routing, idempotency keys, nonces |
| Saga orchestration | Hand-rolled state machine + outbox (default); Temporal as upgrade path | Outbox = fewer deps now; Temporal = less code once saga complexity grows |
| Resilience | `sony/gobreaker` + `avast/retry-go` | Circuit breaker + backoff retries |
| Observability | `otel-go` + Prometheus + `uber-go/zap` | Standard, well-supported Go stack |
| Config | `spf13/viper` | Env-driven config, no custom parsing |
| Testing | `testify` + `testcontainers-go` + `uber-go/mock` | Real integration tests against real Postgres/Kafka/Redis |
| Module structure | Single Go module at repo root | Simpler CI and dependency management; split into multiple modules only if services genuinely diverge in dependency versions later |

---

## 1. Repository Layout

```text
payment-switch/
├── cmd/
│   ├── gateway/main.go
│   ├── lookup-service/main.go
│   ├── routing-service/main.go
│   ├── quoting-service/main.go
│   ├── compliance-service/main.go
│   ├── orchestrator-service/main.go
│   ├── settlement-service/main.go
│   ├── reconciliation-service/main.go
│   ├── audit-service/main.go
│   ├── notification-service/main.go
│   └── certificate-service/main.go
├── internal/
│   ├── gateway/
│   ├── lookup/
│   ├── routing/
│   ├── quoting/
│   ├── compliance/
│   ├── orchestrator/
│   │   ├── domain/          # Payment, Reservation — pure types, no deps
│   │   ├── ports/            # interfaces the domain depends on
│   │   ├── saga/              # saga steps + runner
│   │   ├── db/                # sqlc-generated code
│   │   └── sweep/             # TTL sweeper
│   ├── settlement/
│   ├── reconciliation/
│   ├── audit/
│   ├── notification/
│   └── certificate/
├── pkg/                        # shared libraries, importable by any service
│   ├── eventbus/                # Kafka producer/consumer wrappers
│   ├── outbox/                  # transactional outbox primitives
│   ├── cache/                   # Redis client wrapper
│   ├── crypto/                  # signing, verification, nonce store
│   ├── resilience/               # circuit breaker, retry helpers
│   ├── telemetry/                 # otel, metrics, logging setup
│   ├── middleware/                 # mTLS, idempotency (HTTP)
│   └── iso20022/                    # XML types + (un)marshalling
├── api/
│   ├── proto/                  # gRPC service definitions (.proto)
│   └── openapi/                # REST API spec for the gateway
├── migrations/
│   ├── postgres/
│   └── scylla/
├── deploy/
│   ├── docker/
│   └── k8s/
├── go.mod
└── Makefile
```

`internal/<service>/domain` has zero external dependencies. `ports`
defines the interfaces the domain logic needs (repository, publisher,
cache). Concrete adapters (`pgx`, `gocql`, `kafka-go`) live in
`pkg/` and get wired together only in each service's `main.go`. This is
standard ports-and-adapters — it's what makes the saga logic in §8
unit-testable without a database.

---

## 2. Domain Model

```go
// internal/orchestrator/domain/payment.go
package domain

import "time"

type PaymentStatus string

const (
	StatusReceived  PaymentStatus = "RECEIVED"
	StatusValidated PaymentStatus = "VALIDATED"
	StatusQuoted    PaymentStatus = "QUOTED"
	StatusScreened  PaymentStatus = "SCREENED"
	StatusReserved  PaymentStatus = "RESERVED"
	StatusCommitted PaymentStatus = "COMMITTED"
	StatusAborted   PaymentStatus = "ABORTED"
)

type Payment struct {
	ID             string // internal UUID
	EndToEndID     string // ISO20022 EndToEndId — the idempotency key
	SourceBIC      string
	DestinationBIC string
	SourceAccount  string
	DestAccount    string
	Amount         int64  // minor units (cents) — never use float for money
	Currency       string // ISO 4217
	Status         PaymentStatus
	QuoteID        *string
	ReservedAt     *time.Time
	ExpiresAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Reservation struct {
	PaymentID     string
	SourceAccount string
	Amount        int64
	Status        string // RESERVED | COMMITTED | ABORTED
	ReservedAt    time.Time
	ExpiresAt     time.Time
}
```

Money is `int64` minor units throughout — float64 anywhere near an amount
field is a bug waiting to happen.

---

## 3. Core Interfaces (Ports)

```go
// internal/orchestrator/ports/ports.go
package ports

import (
	"context"
	"time"

	"payment-switch/internal/orchestrator/domain"
)

type PaymentRepository interface {
	Create(ctx context.Context, p *domain.Payment) error
	UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) error
	GetByEndToEndID(ctx context.Context, e2eID string) (*domain.Payment, error)
	FindExpiredReservations(ctx context.Context, before time.Time) ([]domain.Reservation, error)
}

type ComplianceClient interface {
	Screen(ctx context.Context, p *domain.Payment) (ScreenResult, error)
}

type ScreenResult struct {
	Cleared bool
	Reason  string
}

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
}
```

Orchestrator logic depends only on these. `internal/orchestrator/db`
(generated by `sqlc`) implements `PaymentRepository` against Postgres;
tests implement it with an in-memory fake.

---

## 4. API Gateway Middleware

**mTLS → participant identity:**

```go
// pkg/middleware/mtls.go
package middleware

import (
	"context"
	"crypto/x509"
	"net/http"
)

type ctxKey string

const ParticipantCtxKey ctxKey = "participant_id"

type CertRegistry interface {
	// Resolve checks thumbprint match, expiry, and revocation (OCSP/CRL cache).
	Resolve(ctx context.Context, cert *x509.Certificate) (participantID string, err error)
}

func ExtractParticipant(registry CertRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				http.Error(w, "client certificate required", http.StatusUnauthorized)
				return
			}
			participantID, err := registry.Resolve(r.Context(), r.TLS.PeerCertificates[0])
			if err != nil {
				http.Error(w, "unknown or revoked certificate", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ParticipantCtxKey, participantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

Set up the listener with `tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}` —
standard library, no third-party TLS dependency needed.

**Idempotency dedup:**

```go
// pkg/middleware/idempotency.go
package middleware

import (
	"net/http"
	"time"

	"payment-switch/pkg/cache"
)

func Idempotency(c cache.Cache, ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			participantID := r.Context().Value(ParticipantCtxKey).(string)
			idemKey := r.Header.Get("Idempotency-Key")
			if idemKey == "" {
				http.Error(w, "Idempotency-Key header required", http.StatusBadRequest)
				return
			}
			key := "idem:" + participantID + ":" + idemKey

			ok, err := c.SetNX(r.Context(), key, "processing", ttl)
			if err != nil {
				http.Error(w, "cache error", http.StatusInternalServerError)
				return
			}
			if !ok {
				// Either a duplicate in flight, or a completed request. A full
				// implementation stores the final response under this key once
				// the handler completes and serves it here instead of erroring.
				http.Error(w, "duplicate request — see cached result", http.StatusConflict)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

---

## 5. Persistence Layer

**Postgres schema + sqlc:**

```sql
-- migrations/postgres/0001_payments.sql
CREATE TABLE payment (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    end_to_end_id   TEXT NOT NULL,
    source_bic      TEXT NOT NULL,
    destination_bic TEXT NOT NULL,
    source_account  TEXT NOT NULL,
    dest_account    TEXT NOT NULL,
    amount          BIGINT NOT NULL,
    currency        CHAR(3) NOT NULL,
    status          TEXT NOT NULL,
    reserved_at     TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_bic, end_to_end_id)
);

CREATE INDEX idx_payment_expires_at ON payment (expires_at) WHERE status = 'RESERVED';
```

```sql
-- internal/orchestrator/db/queries.sql
-- name: CreatePayment :one
INSERT INTO payment (end_to_end_id, source_bic, destination_bic, source_account,
                      dest_account, amount, currency, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'RECEIVED')
RETURNING *;

-- name: FindExpiredReservations :many
SELECT * FROM payment
WHERE status = 'RESERVED' AND expires_at < $1
FOR UPDATE SKIP LOCKED;
```

`sqlc` generates type-safe Go from these — query bugs are caught at build
time, no ORM reflection. `FOR UPDATE SKIP LOCKED` lets multiple sweeper
instances run concurrently without double-processing the same reservation.

**ScyllaDB ledger** (partitioned by participant + day to keep partitions bounded):

```sql
-- migrations/scylla/0001_ledger.cql
CREATE TABLE ledger.payment_events (
    participant_id  text,
    date_bucket     text,   -- yyyy-mm-dd
    payment_id      uuid,
    event_type      text,
    payload         text,
    created_at      timestamp,
    PRIMARY KEY ((participant_id, date_bucket), payment_id, created_at)
) WITH CLUSTERING ORDER BY (payment_id ASC, created_at ASC);
```

```go
// pkg/ledger/scylla.go
package ledger

import "github.com/gocql/gocql"

func NewSession(hosts []string, keyspace string) (*gocql.Session, error) {
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.LocalQuorum
	return cluster.CreateSession()
}
```

**Redis cache:**

```go
// pkg/cache/redis.go
package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct{ rdb *redis.Client }

func New(addr string) *Cache {
	return &Cache{rdb: redis.NewClient(&redis.Options{Addr: addr})}
}

func (c *Cache) SetNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, val, ttl).Result()
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result() // redis.Nil error if missing — check with errors.Is
}
```

---

## 6. Outbox Pattern

```sql
CREATE TABLE outbox (
    id           BIGSERIAL PRIMARY KEY,
    topic        TEXT NOT NULL,
    msg_key      TEXT NOT NULL,
    payload      JSONB NOT NULL,
    status       TEXT NOT NULL DEFAULT 'PENDING',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_pending ON outbox (id) WHERE status = 'PENDING';
```

```go
// pkg/outbox/outbox.go
package outbox

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// Write must run inside the same transaction as the business-state update
// it accompanies. It never touches Kafka directly — that's the relay's job.
func Write(ctx context.Context, tx pgx.Tx, topic, key string, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO outbox (topic, msg_key, payload) VALUES ($1, $2, $3)`,
		topic, key, payload)
	return err
}
```

```go
// pkg/outbox/relay.go
package outbox

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"payment-switch/pkg/eventbus"
)

// Relay polls the outbox table and publishes pending rows to Kafka. Safe to
// run multiple instances concurrently — FOR UPDATE SKIP LOCKED prevents
// double-publishing the same row.
type Relay struct {
	db        *pgxpool.Pool
	publisher *eventbus.Producer
}

func (r *Relay) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.flushBatch(ctx) // log error; next tick retries
		}
	}
}

func (r *Relay) flushBatch(ctx context.Context) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, topic, msg_key, payload FROM outbox
		WHERE status = 'PENDING' ORDER BY id LIMIT 500
		FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return err
	}

	var ids []int64
	for rows.Next() {
		var id int64
		var topic, key string
		var payload []byte
		if err := rows.Scan(&id, &topic, &key, &payload); err != nil {
			return err
		}
		if err := r.publisher.Publish(ctx, topic, key, payload); err != nil {
			return err // nothing marked published; whole batch retries next tick
		}
		ids = append(ids, id)
	}
	rows.Close()

	if len(ids) > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE outbox SET status = 'PUBLISHED', published_at = now() WHERE id = ANY($1)`,
			ids); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
```

Start with this polling relay. Swap in Debezium (CDC directly on the
outbox table → Kafka) only once the polling interval becomes a measurable
latency bottleneck — don't pay that operational complexity upfront.

---

## 7. Saga Orchestrator

```go
// internal/orchestrator/saga/saga.go
package saga

import (
	"context"

	"payment-switch/internal/orchestrator/domain"
)

// Step is one stage of the payment saga. Compensate must be idempotent and
// safe to call even if Execute only partially succeeded.
type Step interface {
	Name() string
	Execute(ctx context.Context, p *domain.Payment) error
	Compensate(ctx context.Context, p *domain.Payment) error
}

type Saga struct{ steps []Step }

func New(steps ...Step) *Saga { return &Saga{steps: steps} }

func (s *Saga) Run(ctx context.Context, p *domain.Payment) error {
	completed := make([]Step, 0, len(s.steps))
	for _, step := range s.steps {
		if err := step.Execute(ctx, p); err != nil {
			s.rollback(ctx, p, completed)
			return err
		}
		completed = append(completed, step)
	}
	return nil
}

func (s *Saga) rollback(ctx context.Context, p *domain.Payment, completed []Step) {
	for i := len(completed) - 1; i >= 0; i-- {
		_ = completed[i].Compensate(ctx, p) // failures here need retry/DLQ, not silent drop
	}
}
```

```go
// internal/orchestrator/saga/steps.go
package saga

import (
	"context"

	"payment-switch/internal/orchestrator/domain"
)

type ReserveSourceStep struct{ Repo PaymentRepo }

func (s ReserveSourceStep) Name() string { return "reserve_source" }
func (s ReserveSourceStep) Execute(ctx context.Context, p *domain.Payment) error {
	return s.Repo.ReserveWithTTL(ctx, p.ID, p.Amount, ttlFor(p))
}
func (s ReserveSourceStep) Compensate(ctx context.Context, p *domain.Payment) error {
	return s.Repo.ReleaseReservation(ctx, p.ID)
}

type CreditDestinationStep struct{ Bank BankClient }

func (s CreditDestinationStep) Name() string { return "credit_destination" }
func (s CreditDestinationStep) Execute(ctx context.Context, p *domain.Payment) error {
	return s.Bank.Credit(ctx, p.DestinationBIC, p.DestAccount, p.Amount)
}
func (s CreditDestinationStep) Compensate(ctx context.Context, p *domain.Payment) error {
	return s.Bank.ReverseCredit(ctx, p.DestinationBIC, p.DestAccount, p.Amount)
}
```

Each step's `Execute`/`Compensate` should write to the outbox (§6) within
the same DB transaction as its state change. The saga drives state
transitions; the outbox guarantees those transitions are durably published.

**Alternative: Temporal.** Hand-rolling keeps the dependency footprint
small and is enough for a handful of steps. Once the number of
compensation paths and retry/timeout rules grows, [Temporal](https://temporal.io)'s
Go SDK turns the saga above into a workflow function with automatic retry,
timeout, and history replay built in — at the cost of running and
operating the Temporal server. Reasonable path: start hand-rolled, migrate
once saga *complexity* (not transaction volume) becomes the actual pain
point.

---

## 8. Reservation TTL Sweep

```go
// internal/orchestrator/sweep/sweep.go
package sweep

import (
	"context"
	"time"
)

type Sweeper struct {
	repo PaymentRepo
	saga SagaRunner
}

func (s *Sweeper) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expired, err := s.repo.FindExpiredReservations(ctx, time.Now())
			if err != nil {
				continue
			}
			for _, r := range expired {
				_ = s.saga.Compensate(ctx, r.PaymentID) // triggers ABORT + outbox event
			}
		}
	}
}
```

---

## 9. Event Bus (Kafka)

```go
// pkg/eventbus/producer.go
package eventbus

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type Producer struct{ writer *kafka.Writer }

func NewProducer(brokers []string) *Producer {
	return &Producer{writer: &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.Hash{}, // key-based partitioning keeps one payment's events ordered
		RequiredAcks: kafka.RequireAll,
	}}
}

func (p *Producer) Publish(ctx context.Context, topic, key string, payload []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{Topic: topic, Key: []byte(key), Value: payload})
}
```

```go
// pkg/eventbus/consumer.go
package eventbus

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type Handler func(ctx context.Context, msg kafka.Message) error

func Consume(ctx context.Context, brokers []string, topic, group string, h Handler) error {
	r := kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, Topic: topic, GroupID: group})
	defer r.Close()

	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			return err
		}
		if err := h(ctx, msg); err != nil {
			continue // route to a DLQ topic in a real handler, don't crash-loop
		}
		if err := r.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}
```

Start with plain JSON payloads. Add Avro + a schema registry
(`riferrei/srclient`) once payload size or cross-service schema
compatibility becomes an actual problem — not before.

---

## 10. Signature Verification & Replay Protection

```go
// pkg/crypto/verify.go
package crypto

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"time"
)

var ErrReplay = errors.New("replayed message")

type NonceStore interface {
	SetNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error)
}

func VerifySignature(cert *x509.Certificate, canonicalXML, signature []byte) error {
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("unsupported public key type")
	}
	digest := sha256.Sum256(canonicalXML)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], signature)
}

func CheckReplay(ctx context.Context, store NonceStore, participantID, nonce string, ts time.Time) error {
	if time.Since(ts) > 5*time.Minute || time.Until(ts) > time.Minute {
		return errors.New("timestamp out of window")
	}
	ok, err := store.SetNX(ctx, "nonce:"+participantID+":"+nonce, "1", 10*time.Minute)
	if err != nil {
		return err
	}
	if !ok {
		return ErrReplay
	}
	return nil
}
```

mTLS itself needs no third-party library —
`tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}`
from the standard library is sufficient.

---

## 11. Resilience

```go
// pkg/resilience/breaker.go
package resilience

import (
	"time"

	"github.com/sony/gobreaker"
)

func NewBankBreaker(bankBIC string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "bank-" + bankBIC,
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     15 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures > 5 },
	})
}
```

```go
// pkg/resilience/retry.go
package resilience

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"
)

func WithBackoff(ctx context.Context, fn func() error) error {
	return retry.Do(fn,
		retry.Context(ctx),
		retry.Attempts(3),
		retry.Delay(100*time.Millisecond),
		retry.DelayType(retry.BackOffDelay),
	)
}
```

**Bulkhead**: give each downstream bank's HTTP client its own
`http.Transport` with a bounded `MaxConnsPerHost`, instead of a shared
global client — a slow bank then can't starve calls to every other bank.

---

## 12. Observability

```go
// pkg/telemetry/telemetry.go
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"
)

func InitTracer(ctx context.Context, otlpEndpoint string) (*trace.TracerProvider, error) {
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(otlpEndpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	tp := trace.NewTracerProvider(trace.WithBatcher(exp))
	otel.SetTracerProvider(tp)
	return tp, nil
}
```

Tag every span with `end_to_end_id` as an attribute — the correlation key
across gateway → orchestrator → settlement → audit. `prometheus/client_golang`'s
`promhttp.Handler()` on `/metrics` is enough to start; `uber-go/zap` with
`end_to_end_id` on every payment-related log line.

---

## 13. Configuration

```go
// pkg/config/config.go
package config

import "github.com/spf13/viper"

type Config struct {
	HTTPAddr     string   `mapstructure:"HTTP_ADDR"`
	PostgresDSN  string   `mapstructure:"POSTGRES_DSN"`
	ScyllaHosts  []string `mapstructure:"SCYLLA_HOSTS"`
	RedisAddr    string   `mapstructure:"REDIS_ADDR"`
	KafkaBrokers []string `mapstructure:"KAFKA_BROKERS"`
	OTLPEndpoint string   `mapstructure:"OTLP_ENDPOINT"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	return &c, nil
}
```

No DI framework needed at this size — wire concrete dependencies by hand in
each `cmd/<service>/main.go`. Reach for `google/wire` only if constructor
wiring genuinely becomes unwieldy; it rarely does here.

---

## 14. Testing Strategy

```go
// internal/orchestrator/saga/saga_test.go
func TestSaga_CompensatesOnFailure(t *testing.T) {
	failing := &mockStep{name: "credit_destination", err: errors.New("bank timeout")}
	reserve := &mockStep{name: "reserve_source"}

	s := saga.New(reserve, failing)
	err := s.Run(context.Background(), &domain.Payment{})

	require.Error(t, err)
	require.True(t, reserve.compensated, "reserve step should have been compensated")
}
```

```go
// integration test against a real Postgres via testcontainers-go
func TestOutboxRelay_PublishesAndMarksPublished(t *testing.T) {
	ctx := context.Background()
	pgC, dsn := startTestPostgres(t, ctx)
	defer pgC.Terminate(ctx)

	pool, _ := pgxpool.New(ctx, dsn)
	runMigrations(t, pool)
	// write an outbox row, run relay.flushBatch, assert status = PUBLISHED
}
```

- **Unit tests**: saga steps, signature verification, state transitions —
  mocked interfaces generated from `ports` via `uber-go/mock`.
- **Integration tests** (`testcontainers-go`): real Postgres, Kafka, Redis
  for the outbox relay, repositories, and the full reserve→commit→settle
  path.
- **Contract tests**: golden-file round-trips of real `pacs.008`/`pacs.002`
  XML samples through the `iso20022` package's (un)marshalling.

---

## 15. Build & Deployment

```dockerfile
# deploy/docker/Dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE
RUN CGO_ENABLED=0 go build -o /out/service ./cmd/${SERVICE}

FROM alpine:3.19
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]
```

```makefile
# Makefile (excerpt)
build-%:
	docker build --build-arg SERVICE=$* -t payment-switch/$*:dev -f deploy/docker/Dockerfile .

test:
	go test ./... -race -count=1

sqlc:
	sqlc generate
```

Kubernetes: one `Deployment` + `Service` per `cmd/` binary.
`HorizontalPodAutoscaler` on CPU for stateless services; for
`orchestrator-service` and `settlement-service` specifically, scale on
Kafka consumer lag (via KEDA) since that's the metric that actually tracks
backlog under load. `NetworkPolicy` restricting direct DB access to only
the owning service's pods.

---

## 16. Recommended Libraries

| Concern | Library |
|---|---|
| HTTP routing | `go-chi/chi` |
| gRPC | `google.golang.org/grpc` + protobuf |
| Postgres | `jackc/pgx/v5` + `sqlc-dev/sqlc` |
| Scylla/Cassandra | `gocql/gocql` |
| Redis | `redis/go-redis/v9` |
| Kafka | `segmentio/kafka-go` |
| Schema registry (later) | `riferrei/srclient` |
| Circuit breaker | `sony/gobreaker` |
| Retry/backoff | `avast/retry-go` |
| Tracing/metrics | `go.opentelemetry.io/otel`, `prometheus/client_golang` |
| Logging | `uber-go/zap` |
| Config | `spf13/viper` |
| Testing | `stretchr/testify`, `testcontainers-go`, `uber-go/mock` |
| Durable workflow (optional, later) | `go.temporal.io/sdk` |

---

## 17. Suggested Build Order (walking skeleton first)

1. `gateway` + `orchestrator-service` against Postgres only, no Kafka yet —
   hardcode a single bank pair, get
   `RECEIVED→VALIDATED→RESERVED→COMMITTED` working end-to-end
   synchronously.
2. Add the outbox table + relay; replace direct calls with event
   publishing. Add `audit-service` consuming `payment.*` topics.
3. Add reservation TTL + sweeper. Add `compliance-service` as a stub that
   always clears (wire real screening logic later).
4. Add `participant-service` + `certificate-service` + mTLS at the
   gateway, replacing the hardcoded bank pair.
5. Add `lookup-service` + `routing-service` + Redis cache.
6. Add `settlement-service` (net settlement engine) +
   `reconciliation-service`.
7. Add tracing/metrics once there are enough moving parts for it to pay
   off — earlier than this, there's nothing interesting to observe yet.

This gets a real payment moving through the saga in step 1, before any
surrounding infrastructure exists — validate the core state machine before
layering on the operational concerns from the architecture refinement.