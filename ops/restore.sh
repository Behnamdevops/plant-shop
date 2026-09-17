#!/bin/sh
# Restore the production Plant Shop Postgres database from a plain SQL dump.
#
# Reads all configuration from the environment; contains no credentials.
#
# Required environment variables:
#   POSTGRES_USER     database user (matches .env.prod POSTGRES_USER)
#   POSTGRES_DB       database name (matches .env.prod POSTGRES_DB)
#   RESTORE_FILE      path to the dump produced by ops/backup.sh
#
# Optional:
#   RESTORE_FORCE     set to exactly "yes" to skip the interactive warning.
#
# Usage:
#   ./ops/restore.sh
#
# WARNING: this REPLACES the current contents of the target database.
# Stop the backend first so no writes race the restore:
#   docker compose -f docker-compose.prod.yml stop backend
set -eu

: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"
: "${RESTORE_FILE:?RESTORE_FILE is required}"

if [ ! -f "${RESTORE_FILE}" ]; then
  echo "restore file not found: ${RESTORE_FILE}" >&2
  exit 1
fi

if [ "${RESTORE_FORCE:-}" != "yes" ]; then
  echo "This will OVERWRITE database '${POSTGRES_DB}' with ${RESTORE_FILE}."
  echo "Set RESTORE_FORCE=yes to proceed without this prompt."
  printf 'Type the database name to confirm: '
  read -r CONFIRM
  if [ "${CONFIRM}" != "${POSTGRES_DB}" ]; then
    echo "aborted" >&2
    exit 1
  fi
fi

docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" \
  < "${RESTORE_FILE}"

echo "restore finished. Verify with the SELECT checks in OPERATIONS.md"
echo "(row counts on orders/products/users and schema_migrations history),"
echo "then start the backend again:"
echo "  docker compose -f docker-compose.prod.yml up -d backend"
