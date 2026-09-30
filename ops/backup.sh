#!/bin/sh
# Consistent database dump plus both upload trees. Run during a maintenance
# window (stop storefront writes) for a matching DB/files snapshot.
set -eu
umask 077
: "${POSTGRES_USER:?required}"
: "${POSTGRES_DB:?required}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env.prod}"
BACKUP_DIR="${BACKUP_DIR:-backups}"
mkdir -p "$BACKUP_DIR"
backup_path="$BACKUP_DIR/plantshop-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir "$backup_path"
compose() { docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"; }
compose exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc --no-owner --no-privileges > "$backup_path/database.dump"
compose exec -T backend tar -C /app/data/uploads -cf - . > "$backup_path/uploads.tar"
printf '%s\n' "$POSTGRES_DB" > "$backup_path/source-database.txt"
(cd "$backup_path" && sha256sum database.dump uploads.tar > SHA256SUMS)
printf 'Backup completed: %s\n' "$backup_path"
printf 'Keep an encrypted copy on another host and test restoration.\n'
