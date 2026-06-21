#!/usr/bin/env bash
# =============================================================================
# pre-deploy.sh — PaySwitch Swarm pre-deploy init script
#
# Run this on the VPS before (or as part of) every deploy.
# It is idempotent: safe to re-run on updates.
#
# Required environment variables (set in /opt/payswitch/.env):
#   GHCR_REPO                  e.g. ghcr.io/your-org/payswitch
#   IMAGE_TAG                  git SHA of the build to deploy
#   CSRF_SECRET                hex-64 string  (openssl rand -hex 32)
#   TRAEFIK_DASHBOARD_USER     dashboard login username
#   TRAEFIK_DASHBOARD_PASSWORD dashboard login password (plain text — hashed here)
#   VPS_GHCR_TOKEN             GitHub PAT with read:packages scope
#   GITHUB_ACTOR               GitHub username (for GHCR login)
#
# Order of operations (critical — do NOT reorder):
#   1. GHCR login
#   2. Init Docker Swarm
#   3. Create named volumes
#   4. Run certgen (standalone container — only needs the certs volume)
#   5. Create overlay network
#   6. Create Docker secrets
#   7. Deploy / update the stack  ← postgres starts HERE
#   8. Wait for postgres to be healthy inside the running stack
#   9. Run migrations (one-shot container on the overlay network)
# =============================================================================

set -euo pipefail

STACK_NAME="payswitch"
DEPLOY_DIR="/opt/payswitch"
MIGRATIONS_DIR="${DEPLOY_DIR}/migrations/postgres"
POSTGRES_WAIT_TIMEOUT=120   # seconds to wait for postgres to become ready

# ── Validate required vars ────────────────────────────────────────────────────
: "${GHCR_REPO:?GHCR_REPO is required}"
: "${IMAGE_TAG:?IMAGE_TAG is required}"
: "${CSRF_SECRET:?CSRF_SECRET is required}"
: "${TRAEFIK_DASHBOARD_USER:?TRAEFIK_DASHBOARD_USER is required}"
: "${TRAEFIK_DASHBOARD_PASSWORD:?TRAEFIK_DASHBOARD_PASSWORD is required}"
: "${VPS_GHCR_TOKEN:?VPS_GHCR_TOKEN is required}"
: "${GITHUB_ACTOR:?GITHUB_ACTOR is required}"

echo "==> PaySwitch pre-deploy  (stack: ${STACK_NAME}, tag: ${IMAGE_TAG})"

# ── 1. Authenticate to GHCR ───────────────────────────────────────────────────
echo "--> Logging in to GHCR..."
echo "${VPS_GHCR_TOKEN}" | docker login ghcr.io -u "${GITHUB_ACTOR}" --password-stdin

# ── 2. Init Docker Swarm (idempotent) ─────────────────────────────────────────
SWARM_STATE=$(docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || echo inactive)
if [ "${SWARM_STATE}" != "active" ]; then
  echo "--> Initialising Docker Swarm..."
  docker swarm init --advertise-addr "$(hostname -I | awk '{print $1}')"
else
  echo "--> Swarm already active."
fi

# ── 3. Create named volumes (idempotent) ──────────────────────────────────────
for vol in certs postgres_data clickhouse_data; do
  FULL="${STACK_NAME}_${vol}"
  if ! docker volume ls --format '{{.Name}}' | grep -q "^${FULL}$"; then
    echo "--> Creating volume ${FULL}..."
    docker volume create "${FULL}"
  fi
done

# ── 4. Generate TLS certificates (idempotent) ─────────────────────────────────
# Standalone docker run — only needs the certs volume, no network required.
# Skipped if ca-cert.pem already exists (rotate by removing the file).
echo "--> Running certgen..."
docker run --rm \
  --mount "type=volume,src=${STACK_NAME}_certs,dst=/certs" \
  --entrypoint sh \
  "${GHCR_REPO}/certgen:${IMAGE_TAG}" \
  -c '[ -f /certs/ca-cert.pem ] && echo "Certs already present, skipping." || /service'

# ── 5. Create overlay network (idempotent) ────────────────────────────────────
# Must be created before stack deploy so Traefik and the migration container
# can both join it. Docker Swarm will adopt it when the stack deploys.
NETWORK_NAME="${STACK_NAME}_net"
if ! docker network ls --format '{{.Name}}' | grep -q "^${NETWORK_NAME}$"; then
  echo "--> Creating overlay network ${NETWORK_NAME}..."
  docker network create \
    --driver overlay \
    --attachable \
    --label "com.docker.stack.namespace=${STACK_NAME}" \
    "${NETWORK_NAME}"
else
  echo "--> Network ${NETWORK_NAME} already exists."
fi

# ── 6. Docker secret: dashboard_users ────────────────────────────────────────
# Must exist before stack deploy (Traefik reads it at startup).
SECRET_NAME="dashboard_users"
if ! docker secret ls --format '{{.Name}}' | grep -q "^${SECRET_NAME}$"; then
  echo "--> Creating Docker secret: ${SECRET_NAME}..."
  HASHED_PW=$(openssl passwd -apr1 "${TRAEFIK_DASHBOARD_PASSWORD}")
  printf '%s:%s\n' "${TRAEFIK_DASHBOARD_USER}" "${HASHED_PW}" \
    | docker secret create "${SECRET_NAME}" -
  echo "    Dashboard user: ${TRAEFIK_DASHBOARD_USER}"
else
  echo "--> Secret ${SECRET_NAME} already exists — skipping."
  echo "    To rotate: docker secret rm ${SECRET_NAME} && re-run."
fi

# ── 7. Deploy / update the Swarm stack ───────────────────────────────────────
# This starts postgres (and all other services). Migrations run AFTER this.
echo "--> Deploying stack '${STACK_NAME}' (image: ${IMAGE_TAG})..."
GHCR_REPO="${GHCR_REPO}" \
IMAGE_TAG="${IMAGE_TAG}" \
CSRF_SECRET="${CSRF_SECRET}" \
  docker stack deploy \
    --with-registry-auth \
    --prune \
    --resolve-image always \
    -c "${DEPLOY_DIR}/deploy/docker/stack.yaml" \
    "${STACK_NAME}"

# ── 8. Wait for postgres to be healthy ────────────────────────────────────────
# Postgres runs as a Swarm service now. Poll until a container is up and
# pg_isready succeeds. This is the gate before running migrations.
echo "--> Waiting for postgres to be ready (timeout: ${POSTGRES_WAIT_TIMEOUT}s)..."
ELAPSED=0
while true; do
  # Find a running container belonging to the postgres service
  PG_CONTAINER=$(docker ps --filter "name=${STACK_NAME}_postgres" -q 2>/dev/null | head -1)
  if [ -n "${PG_CONTAINER}" ] && docker exec "${PG_CONTAINER}" pg_isready -U switch -q 2>/dev/null; then
    echo "    Postgres is ready."
    break
  fi
  if [ "${ELAPSED}" -ge "${POSTGRES_WAIT_TIMEOUT}" ]; then
    echo "ERROR: Timed out waiting for postgres after ${POSTGRES_WAIT_TIMEOUT}s"
    echo "       Check service logs: docker service logs ${STACK_NAME}_postgres"
    exit 1
  fi
  sleep 5
  ELAPSED=$((ELAPSED + 5))
  echo "    Still waiting... (${ELAPSED}s)"
done

# ── 9. Run database migrations ────────────────────────────────────────────────
# One-shot container on the overlay network. Postgres is confirmed healthy above.
# Applied in alphabetical order; scripts must be idempotent (IF NOT EXISTS etc.).
echo "--> Running database migrations..."
docker run --rm \
  --network "${NETWORK_NAME}" \
  --volume "${MIGRATIONS_DIR}:/schema:ro" \
  --env PGPASSWORD=switch \
  postgres:16-alpine \
  sh -c '
    for f in $(ls /schema/*.sql | sort); do
      echo "  applying $(basename $f)..."
      psql -h postgres -U switch -d switch -f "$f" -q
    done
    echo "Migrations complete."
  '

echo ""
echo "==> Deploy complete!"
echo "    Portal:    http://<VPS_IP>:8000/portal/"
echo "    Dashboard: http://<VPS_IP>:8080/dashboard/"
echo "    Uptrace:   http://<VPS_IP>:14318"
echo ""
echo "    Services:  docker service ls"
echo "    Logs:      docker service logs ${STACK_NAME}_gateway -f"
