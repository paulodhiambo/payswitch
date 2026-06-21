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
# Usage:
#   source /opt/payswitch/.env
#   bash /opt/payswitch/deploy/docker/pre-deploy.sh
# =============================================================================

set -euo pipefail

STACK_NAME="payswitch"
DEPLOY_DIR="/opt/payswitch"
MIGRATIONS_DIR="${DEPLOY_DIR}/migrations/postgres"

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
# Writes ca-cert.pem, server-cert.pem, server-key.pem into the certs volume.
# Skipped if ca-cert.pem already exists (allows cert rotation by removing the file).
echo "--> Running certgen..."
docker run --rm \
  --mount "type=volume,src=${STACK_NAME}_certs,dst=/certs" \
  --entrypoint sh \
  "${GHCR_REPO}/certgen:${IMAGE_TAG}" \
  -c '[ -f /certs/ca-cert.pem ] && echo "Certs already present, skipping." || /service'

# ── 5. Create overlay network (idempotent) ────────────────────────────────────
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

# ── 6. Run database migrations ────────────────────────────────────────────────
# Runs a one-shot postgres client container on the overlay network.
# Services on the overlay (postgres) are reachable by name.
# Migrations are applied in alphabetical order; each script is idempotent
# (uses IF NOT EXISTS / ON CONFLICT DO NOTHING patterns).
echo "--> Running database migrations..."
docker run --rm \
  --network "${NETWORK_NAME}" \
  --volume "${MIGRATIONS_DIR}:/schema:ro" \
  --env PGPASSWORD=switch \
  postgres:16-alpine \
  sh -c '
    echo "Waiting for postgres..."
    until pg_isready -h postgres -U switch -q; do sleep 2; done
    echo "Applying migrations..."
    for f in $(ls /schema/*.sql | sort); do
      echo "  applying $f..."
      psql -h postgres -U switch -d switch -f "$f" -q
    done
    echo "Migrations complete."
  '

# ── 7. Docker secret: dashboard_users ────────────────────────────────────────
# htpasswd-format credentials for the Traefik dashboard basic-auth middleware.
# Uses APR1 (MD5-based) — compatible with all Traefik versions.
# To rotate: docker secret rm dashboard_users && re-run this script.
SECRET_NAME="dashboard_users"
if ! docker secret ls --format '{{.Name}}' | grep -q "^${SECRET_NAME}$"; then
  echo "--> Creating Docker secret: ${SECRET_NAME}..."
  HASHED_PW=$(openssl passwd -apr1 "${TRAEFIK_DASHBOARD_PASSWORD}")
  printf '%s:%s\n' "${TRAEFIK_DASHBOARD_USER}" "${HASHED_PW}" \
    | docker secret create "${SECRET_NAME}" -
  echo "    Dashboard user: ${TRAEFIK_DASHBOARD_USER}"
else
  echo "--> Secret ${SECRET_NAME} already exists — skipping."
  echo "    To rotate credentials: docker secret rm ${SECRET_NAME} && re-run."
fi

# ── 8. Deploy / update the Swarm stack ───────────────────────────────────────
echo "--> Deploying stack '${STACK_NAME}' with image tag '${IMAGE_TAG}'..."
GHCR_REPO="${GHCR_REPO}" \
IMAGE_TAG="${IMAGE_TAG}" \
CSRF_SECRET="${CSRF_SECRET}" \
  docker stack deploy \
    --with-registry-auth \
    --prune \
    --resolve-image always \
    -c "${DEPLOY_DIR}/deploy/docker/stack.yaml" \
    "${STACK_NAME}"

echo "==> Deploy complete!"
echo ""
echo "    Portal:    http://<VPS_IP>:8000/portal/"
echo "    Dashboard: http://<VPS_IP>:8080/dashboard/"
echo "    Uptrace:   http://<VPS_IP>:14318"
echo ""
echo "    Services:  docker service ls"
echo "    Logs:      docker service logs payswitch_gateway -f"
