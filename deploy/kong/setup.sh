#!/bin/bash
set -euo pipefail

KONG_ADMIN="http://kong:8001"

echo "Waiting for Kong Admin API..."
for i in $(seq 1 30); do
  if curl -sf -o /dev/null "$KONG_ADMIN/status" 2>/dev/null; then
    break
  fi
  sleep 2
done

echo "Kong ready — seeding configuration..."

# ---------------------------------------------------------------------------
# Payment Service — mTLS gateway on :8443
# ---------------------------------------------------------------------------
curl -sf -X PUT "$KONG_ADMIN/services/payment-service" \
  -H "Content-Type: application/json" \
  -d '{"host":"gateway","port":8080,"protocol":"http","read_timeout":120000,"write_timeout":120000,"connect_timeout":10000}'

curl -sf -X PUT "$KONG_ADMIN/routes/payments" \
  -H "Content-Type: application/json" \
  -d '{"name":"payments","paths":["/payments"],"strip_path":false,"methods":["POST","GET"],"service":{"name":"payment-service"}}'

curl -sf -X POST "$KONG_ADMIN/routes/payments/plugins" \
  -H "Content-Type: application/json" \
  -d '{"name":"post-function","config":{"access":["local dn = ngx.var.ssl_client_s_dn; if dn then kong.service.request.set_header(\"X-Participant-Id\", dn) end"]}}' || true

curl -sf -X POST "$KONG_ADMIN/routes/payments/plugins" \
  -H "Content-Type: application/json" \
  -d '{"name":"rate-limiting","config":{"minute":2000000,"policy":"local"}}' || true

curl -sf -X PUT "$KONG_ADMIN/routes/health" \
  -H "Content-Type: application/json" \
  -d '{"name":"health","paths":["/healthz"],"strip_path":false,"methods":["GET"],"service":{"name":"payment-service"}}'

# ---------------------------------------------------------------------------
# Portal API Service — behind forward-auth on :8000
# ---------------------------------------------------------------------------
curl -sf -X PUT "$KONG_ADMIN/services/portal-api-service" \
  -H "Content-Type: application/json" \
  -d '{"host":"portal-api","port":8080,"protocol":"http"}'

curl -sf -X PUT "$KONG_ADMIN/routes/portal-api" \
  -H "Content-Type: application/json" \
  -d '{"name":"portal-api","paths":["/portal/api"],"strip_path":false,"protocols":["http"],"service":{"name":"portal-api-service"}}'

curl -sf -X POST "$KONG_ADMIN/routes/portal-api/plugins" \
  -H "Content-Type: application/json" \
  -d '{"name":"pre-function","config":{"access":["local proto = kong.request.get_forwarded_scheme() or \"http\"; local host = kong.request.get_forwarded_host() or \"localhost:8000\"; local path = kong.request.get_path(); local qs = kong.request.get_raw_query(); local url = proto .. \"://\" .. host .. path; if qs and qs ~= \"\" then url = url .. \"?\" .. qs end; kong.service.request.set_header(\"X-Original-URL\", url)"]}}' || true

curl -sf -X POST "$KONG_ADMIN/routes/portal-api/plugins" \
  -H "Content-Type: application/json" \
  -d '{"name":"forward-auth","config":{"http_method":"GET","address":"http://auth-service:9000/auth","trust_response_headers":["X-authentik-uid","X-authentik-username","X-authentik-name","X-authentik-email","X-authentik-groups","X-User-Role","X-Participant-Id"]}}' || true

curl -sf -X POST "$KONG_ADMIN/routes/portal-api/plugins" \
  -H "Content-Type: application/json" \
  -d '{"name":"rate-limiting","config":{"minute":300,"policy":"local"}}' || true

# ---------------------------------------------------------------------------
# Portal SPA Service — static files, also behind forward-auth
# ---------------------------------------------------------------------------
curl -sf -X PUT "$KONG_ADMIN/services/portal-spa-service" \
  -H "Content-Type: application/json" \
  -d '{"host":"portal-api","port":8080,"protocol":"http"}'

curl -sf -X PUT "$KONG_ADMIN/routes/portal-spa" \
  -H "Content-Type: application/json" \
  -d '{"name":"portal-spa","paths":["/portal"],"strip_path":false,"protocols":["http"],"service":{"name":"portal-spa-service"}}'

curl -sf -X POST "$KONG_ADMIN/routes/portal-spa/plugins" \
  -H "Content-Type: application/json" \
  -d '{"name":"pre-function","config":{"access":["local proto = kong.request.get_forwarded_scheme() or \"http\"; local host = kong.request.get_forwarded_host() or \"localhost:8000\"; local path = kong.request.get_path(); local qs = kong.request.get_raw_query(); local url = proto .. \"://\" .. host .. path; if qs and qs ~= \"\" then url = url .. \"?\" .. qs end; kong.service.request.set_header(\"X-Original-URL\", url)"]}}' || true

curl -sf -X POST "$KONG_ADMIN/routes/portal-spa/plugins" \
  -H "Content-Type: application/json" \
  -d '{"name":"forward-auth","config":{"http_method":"GET","address":"http://auth-service:9000/auth","trust_response_headers":["X-authentik-uid","X-authentik-username","X-authentik-name","X-authentik-email","X-authentik-groups","X-User-Role","X-Participant-Id"]}}' || true

# ---------------------------------------------------------------------------
# Auth Service — unauthenticated login/logout
# ---------------------------------------------------------------------------
curl -sf -X PUT "$KONG_ADMIN/services/auth-service" \
  -H "Content-Type: application/json" \
  -d '{"host":"auth-service","port":9000,"protocol":"http"}'

curl -sf -X PUT "$KONG_ADMIN/routes/auth-routes" \
  -H "Content-Type: application/json" \
  -d '{"name":"auth-routes","paths":["/login","/logout"],"strip_path":false,"protocols":["http"],"service":{"name":"auth-service"}}'

# ---------------------------------------------------------------------------
# Global OpenTelemetry Plugin — exports traces to Uptrace
# ---------------------------------------------------------------------------
echo "Configuring OpenTelemetry plugin..."
curl -sf -X POST "$KONG_ADMIN/plugins" \
  -H "Content-Type: application/json" \
  -d '{"name":"opentelemetry","config":{"endpoint":"uptrace:14317","headers":{"uptrace-dsn":"http://token123@uptrace:14318/1"},"resource_attributes":{"service.name":"kong"}}}' || true

echo "Kong seeding complete!"
