#!/bin/sh
# Restore only to a NEW database and a NEW local upload staging directory.
# No running application's database or upload volume is overwritten.
set -eu
umask 077
: "${POSTGRES_USER:?required}"
: "${BACKUP_PATH:?required}"
: "${RESTORE_DB:?new database name required}"
: "${RESTORE_UPLOAD_DIR:?new empty directory required}"
case "$RESTORE_DB" in *[!a-zA-Z0-9_]*|'') echo 'Invalid database name' >&2; exit 1;; esac
[ ! -e "$RESTORE_UPLOAD_DIR" ] || { echo 'Upload destination already exists' >&2; exit 1; }
(cd "$BACKUP_PATH" && sha256sum -c SHA256SUMS)
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env.prod}"
compose() { docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"; }
# createdb fails if the name already exists. Never drop or reuse a database.
compose exec -T postgres createdb -U "$POSTGRES_USER" "$RESTORE_DB"
compose exec -T postgres pg_restore -U "$POSTGRES_USER" -d "$RESTORE_DB" --no-owner --no-privileges --single-transaction < "$BACKUP_PATH/database.dump"
mkdir -p "$RESTORE_UPLOAD_DIR"
# Only restore trusted backups; tar contains the upload root and articles/.
tar -xf "$BACKUP_PATH/uploads.tar" -C "$RESTORE_UPLOAD_DIR"
printf 'Restored new database %s and staged uploads at %s.\n' "$RESTORE_DB" "$RESTORE_UPLOAD_DIR"
printf 'Verify counts, migration history and images before changing deployment configuration.\n'
