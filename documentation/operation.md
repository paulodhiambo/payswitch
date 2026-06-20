# Payment Switch Operations Manual

This guide outlines how to access, authenticate, and monitor the payment switch services running in the local Docker Compose environment.

---

## 1. Summary of Services & Endpoints

| Service / Tool | Access URL / Port | Auth Mechanism | Description |
| :--- | :--- | :--- | :--- |
| **Kong API Gateway (mTLS)** | `https://localhost:8443` | Client SSL Certificates (mTLS) | Secure entrypoint for Bank-to-Switch payment requests |
| **Participant Portal** | `http://localhost:8000/portal/` | Authentik Session Cookie + TOTP | Web UI for switch operators and banks |
| **Authentik Admin Panel** | `http://localhost:9000` | Username / Password | Identity provider administration |
| **Grafana Dashboard** | `http://localhost:3000` | Anonymous (Admin enabled) | Visualizes metrics and OTLP tracing |
| **Prometheus Server** | `http://localhost:9090` | None | Centralized metrics scraper and query engine |
| **Direct Portal BFF API** | `http://localhost:8090` | Cookie (bypass gateway for dev) | Developer REST endpoint for portal API |
| **Postgres Database** | `localhost:5432` | Username/Password | Orchestrator transaction ledger and state |
| **Redpanda Console** | `localhost:9092` | None | Kafka/Redpanda broker endpoint |
| **Redis Cache** | `localhost:6379` | None | Lookup and session cache store |

---

## 2. Accessing the Bank Payment API (mTLS)

The Gateway exposes the secure payment processing API on port `8443` through Kong. Kong terminates mTLS and validates client certificates against the dev CA.

### A. Pre-requisite: Client Certificates
Dev certificates are generated automatically by the `certgen` container on initial start. You can copy the generated client keys out of the docker volume using:
```bash
# Copy client certificates for bank-a
docker compose cp certgen:/certs/client-bank-a-cert.pem .
docker compose cp certgen:/certs/client-bank-a-key.pem .
```

### B. Submitting a Payment Request (JSON)
Submit a payment instruction using `curl`. An `Idempotency-Key` header is required to protect against duplicate submissions.
```bash
curl -k --cert client-bank-a-cert.pem --key client-bank-a-key.pem \
  https://localhost:8443/payments \
  -H 'Idempotency-Key: payment-id-12345' \
  -H 'Content-Type: application/json' \
  -d '{
    "end_to_end_id": "E2E-99881",
    "destination_bic": "BANKDEFF",
    "dest_account": "ACC-B",
    "amount": 5000,
    "currency": "KES",
    "debtor_name": "Alice Smith",
    "creditor_name": "Bob Jones",
    "remittance_info": "Invoice #4920"
  }'
```

---

## 3. Participant Portal & Authentik (MFA Login)

The Participant Portal is a React SPA proxied by Kong on port `8000` and protected by Authentik forward-authentication.

*   **Portal URL**: `http://localhost:8000/portal/`

### A. Bootstrapped Dev Accounts
You can sign in using one of the following bootstrapped credentials (defined in [portal-application.yaml](file:///Users/paul/GolandProjects/switch/deploy/authentik/blueprints/portal-application.yaml)):

| Role / Scope | Username | Password | Attribute |
| :--- | :--- | :--- | :--- |
| **Switch Administrator** | `switch-admin` | `switchadmin` | Global access |
| **Switch Operator** | `switch-ops` | `switchops` | Global monitoring |
| **Bank Administrator** | `bank-admin` | `bankadmin` | Scoped to Bank A |
| **Bank Viewer** | `bank-viewer` | `bankviewer` | Scoped to Bank A |

### B. Authentik Admin Panel
To manage users, groups, application provider details, or custom header property mappings:
*   **URL**: `http://localhost:9000/if/admin/`
*   **Username**: `akadmin` (or `admin@switch.local`)
*   **Password**: `switchadmin`

---

## 4. Monitoring & Observability (Grafana, Prometheus & Tempo)

The stack integrates full metrics collection (Prometheus) and distributed tracing (OpenTelemetry/Tempo).

### A. Grafana
*   **URL**: `http://localhost:3000`
*   **Authentication**: None (configured for anonymous admin access).
*   **Dashboard**: A pre-configured **Payment Switch Overview** dashboard is loaded automatically. It visualizes:
    *   Active payments in flight (live gauges)
    *   Cumulative payment counts categorized by status
    *   Saga steps throughput (steps processed/min)
    *   HTTP traffic rates and p99/p90 latency quantiles

### B. Distributed Tracing (Tempo)
To trace payment sagas across the microservice orchestration flow:
1.  Open Grafana and go to **Explore** (left navigation).
2.  Select **Tempo** from the datasource dropdown.
3.  Choose **Search** to list trace spans. You can filter by tags such as:
    *   `service.name = "gateway"`
    *   `step` (e.g. `reserve_source`, `screen`, `credit_destination`)
4.  Each payment saga run aggregates all 10 steps (including downstream gRPC microservice calls) under a single trace ID, making it easy to isolate latency bottlenecks or exceptions.

### C. Prometheus
Prometheus runs as a background scraping service on port `9090`:
*   **URL**: `http://localhost:9090`
*   **Scraping**: Automatically queries metrics from every service container on port `9095` every 5 seconds.
*   **Ad-hoc Queries**: You can query custom metrics directly via PromQL (e.g., `rate(saga_steps_total[1m])`).

### D. Direct Service Metrics Endpoints
For debugging, each container exposes raw Prometheus metrics on internal port `9095`. These are forwarded to distinct host ports:
*   **Gateway**: `http://localhost:9095/metrics`
*   **Compliance Service**: `http://localhost:9096/metrics`
*   **Lookup Service**: `http://localhost:9097/metrics`
*   **Settlement Service**: `http://localhost:9098/metrics`
*   **Quoting Service**: `http://localhost:9099/metrics`
*   **Notification Service**: `http://localhost:9100/metrics`
*   **Routing Service**: `http://localhost:9101/metrics`
*   **Reconciliation Service**: `http://localhost:9102/metrics`

---

## 5. Troubleshooting & Operations

### A. Rebuilding and Restarting
If you modify configuration files or source code, rebuild the binaries and restart the containers to clear cached layers:
```bash
docker compose -f deploy/docker/compose.yaml down
docker compose -f deploy/docker/compose.yaml up --build
```

### B. Checking Database State
To inspect payments, audit entries, or outbox transactions directly in Postgres:
```bash
docker compose exec postgres psql -U switch -d switch
```
Useful SQL queries:
*   `SELECT id, uetr, status, amount FROM payments;` (view payments and their terminal state)
*   `SELECT * FROM outbox_events;` (view transactional outbox events waiting to be dispatched to Redpanda)
