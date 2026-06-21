# Payment Switch Operations Manual

This guide outlines how to access, authenticate, and monitor the payment switch services running in the local Docker Compose environment.

---

## 1. Summary of Services & Endpoints

| Service / Tool | Access URL / Port | Auth Mechanism | Description |
| :--- | :--- | :--- | :--- |
| **Traefik API Gateway (mTLS)** | `https://localhost:8443` | Client SSL Certificates (mTLS) | Secure entrypoint for Bank-to-Switch payment requests |
| **Traefik Dashboard** | `http://localhost:8080` | None | Admin dashboard to inspect routes and services (when enabled) |
| **Participant Portal** | `http://localhost:8000/portal/` | Auth Service Cookie | Web UI for switch operators and banks |
| **Uptrace Dashboard** | `http://localhost:14318` | None | Unified OTLP tracing and metrics dashboard |
| **Direct Portal BFF API** | `http://localhost:8090` | Cookie (bypass gateway for dev) | Developer REST endpoint for portal API |
| **Postgres Database** | `localhost:5432` | Username/Password | Orchestrator transaction ledger and state |
| **Redpanda Console** | `localhost:9092` | None | Kafka/Redpanda broker endpoint |
| **Redis Cache** | `localhost:6379` | None | Lookup and session cache store |

---

## 2. Accessing the Bank Payment API (mTLS)

The Gateway exposes the secure payment processing API on port `8443` through Traefik. Traefik terminates mTLS and validates client certificates against the dev CA.

### A. Pre-requisite: Client Certificates
Dev certificates are generated automatically by the `certgen` container on initial start. You can copy the generated client keys out of the docker volume using:
```bash
# Copy client certificates for bank-a
docker compose cp certgen:/certs/client-bank-a-cert.pem .
docker compose cp certgen:/certs/client-bank-a-key.pem .
```

### B. Submitting a Payment Request (JSON)
Submit a payment instruction using `curl`:
```bash
curl -k --cert client-bank-a-cert.pem --key client-bank-a-key.pem \
  https://localhost:8443/payments \
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

The response is `202 Accepted` with a payment ID and `RECEIVED` status. The
saga runs asynchronously; the final result (pacs.002 XML) is delivered via
callback to the bank's registered `callback_url`. If no callback URL is
configured, poll `GET /payments/{id}` for terminal status.

---

### 3. Participant Portal & Go Auth Service

The Participant Portal is a React SPA proxied by Traefik on port `8000` and protected by the custom Go-based `auth-service` forward-authentication.

*   **Portal URL**: `http://localhost:8000/portal/`

### A. Bootstrapped Dev Accounts
You can sign in using one of the following bootstrapped credentials:

| Role / Scope | Username | Password | Attribute |
| :--- | :--- | :--- | :--- |
| **Switch Administrator** | `switch-admin` | `switchadmin` | Global access |
| **Switch Operator** | `switch-ops` | `switchops` | Global monitoring |
| **Bank Administrator** | `bank-admin` | `bankadmin` | Scoped to Bank A |
| **Bank Viewer** | `bank-viewer` | `bankviewer` | Scoped to Bank A |

---

## 4. Monitoring & Observability (Uptrace)

The stack integrates full OTLP tracing and metrics collection in a unified dashboard via **Uptrace**.

### A. Uptrace
*   **URL**: `http://localhost:14318`
*   **Authentication**: None.
*   **Default Project**: Telemetry is automatically routed to Project 1 (`Default Project`).
*   **Distributed Tracing**:
    *   Navigate to the **Traces** tab to search and view payment saga transaction runs.
    *   Each payment saga aggregates all steps (e.g. `reserve_source`, `screen`, `credit_destination`) under a single trace ID, making it easy to isolate latency bottlenecks or exceptions.
*   **Metrics**:
    *   Go to the **Metrics** tab to view standard system metrics and custom transaction rates, latencies, and saga step throughput.

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


```shell
# Quick sanity check
k6 run test/load/smoke.js

# Ramp-up stress test (~3.5 min)
k6 run test/load/gateway.js

# Sustained soak test (~11 min)
k6 run test/load/soak.js

```