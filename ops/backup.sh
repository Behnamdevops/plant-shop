#!/bin/sh
# Dump the production Plant Shop Postgres database to a timestamped file.
#
# Reads all configuration from the environment; contains no credentials.
#
# Required environment variables:
#   POSTGRES_USER     database user (matches .env.prod POSTGRES_USER)
#   POSTGRES_DB       database name (matches .env.prod POSTGRES_DB)
#   BACKUP_DIR        local directory to write the dump into (default ./backups)
#
# Optional:
#   COMPOSE_FILE      compose project to exec against (default: docker-compose.prod.yml)
#   COMPOSE_PROJECT_NAME / --env-file must match the running production stack.
#
# Usage:
#   ./ops/backup.sh                          # plain SQL dump
#
# IMPORTANT: the dump file is written to the local disk of the host running
# this script. Copy it OFF that host immediately (e.g. to an object store
# or another machine over scp) — see OPERATIONS.md "Backups" for storage,
# encryption, access control, and retention requirements.
set -eu

: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"
BACKUP_DIR="${BACKUP_DIR:-backups}"
mkdir -p "${BACKUP_DIR}"

OUT="${BACKUP_DIR}/${POSTGRES_DB}-$(date -u +%Y%m%dT%H%M%SZ).sql"

# -F p  plain SQL (restorable with psql, human-readable)
# --no-owner --no-privileges: restore cleanly into a fresh cluster without
#                             assuming the same role names.
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" --no-owner --no-privileges \
  > "${OUT}"

echo "backup written: ${OUT}"
echo "verify it (see OPERATIONS.md) and copy it off this host now."
