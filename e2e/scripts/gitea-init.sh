#!/usr/bin/env bash
set -euo pipefail

# gitea-init.sh — Creates the admin user via Gitea CLI and waits for readiness.
#
# This script is intentionally minimal: it only creates the admin user, which is the
# only operation that must use the Gitea CLI (docker exec).  All other setup
# (PAT creation, org, repo, branch) is handled in Go via e2e/bootstrap/gitea.go
# for consistency, type-safety, and a single canonical definition of required scopes.
#
# After this script exits successfully:
#   - Admin user is reachable via HTTP
#   - Go code (BootstrapGitea) can create the PAT, test-org, and test-repo

GITEA_ADMIN_USER="${GITEA_ADMIN_USER:-gitadmin}"
GITEA_ADMIN_PASSWORD="${GITEA_ADMIN_PASSWORD:-admin-password}"
GITEA_ADMIN_EMAIL="${GITEA_ADMIN_EMAIL:-admin@local.dev}"
GITEA_URL="${GITEA_URL:-http://localhost:3000}"

# Detect the actual Gitea container name (it may change after docker compose down/up)
GITEA_CONTAINER=$(docker compose ps --format '{{.Name}}' 2>/dev/null | grep gitea | head -1)
if [ -z "${GITEA_CONTAINER}" ]; then
  # Fallback to default name
  GITEA_CONTAINER="e2e-gitea-1"
fi
echo "Using Gitea container: ${GITEA_CONTAINER}"

# Wait for Gitea to be ready
echo "Waiting for Gitea to be ready..."
until curl -sf "${GITEA_URL}/api/v1/version" >/dev/null 2>&1; do
  echo "Waiting for Gitea..."
  sleep 2
done

echo "Gitea is ready."

# Create the admin user if needed and wait for it to become available.
echo "Creating admin user '${GITEA_ADMIN_USER}'..."
if ! docker exec -u git "${GITEA_CONTAINER}" /usr/local/bin/gitea admin user create \
  --config /data/gitea/conf/app.ini \
  --admin \
  --username "${GITEA_ADMIN_USER}" \
  --password "${GITEA_ADMIN_PASSWORD}" \
  --email "${GITEA_ADMIN_EMAIL}" \
  --must-change-password=false; then
  echo "WARNING: admin user creation command returned non-zero; continuing to verify availability"
fi

echo "Waiting for admin user '${GITEA_ADMIN_USER}' to be available..."
for i in $(seq 1 30); do
  if curl -sf -u "${GITEA_ADMIN_USER}:${GITEA_ADMIN_PASSWORD}" "${GITEA_URL}/api/v1/user" >/dev/null 2>&1; then
    echo "Admin user '${GITEA_ADMIN_USER}' is available."
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: admin user '${GITEA_ADMIN_USER}' never became available" >&2
    exit 1
  fi
  sleep 2
done

echo "Gitea admin user bootstrap completed successfully."
echo "Admin user '${GITEA_ADMIN_USER}' is ready at ${GITEA_URL}."
echo "Remaining setup (PAT, org, repo) will be handled by the Go bootstrap code."
