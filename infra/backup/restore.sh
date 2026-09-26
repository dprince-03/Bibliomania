#!/usr/bin/env bash
# Restore a backup made by backup.sh into the running Compose stack.
#
#   infra/backup/restore.sh backups/<timestamp>
#
# DESTRUCTIVE: each database is replaced by the backup's contents. Stop the
# application services first so nothing writes mid-restore:
#   docker compose ... stop gateway auth catalog borrow reading user notification payment
# and start them again afterwards. Checksums are verified before anything
# is touched.
set -euo pipefail

IN=${1:?usage: restore.sh <backup-dir>}
cd "$(dirname "$0")/../.."
ENV_FILE=${ENV_FILE:-infra/docker/.env}
[[ -f "$ENV_FILE" ]] || ENV_FILE=infra/docker/.env.example
. infra/backup/load-env.sh
load_env "$ENV_FILE"

(cd "$IN" && sha256sum --quiet -c SHA256SUMS) || { echo "checksum mismatch — refusing to restore" >&2; exit 1; }

for db in catalog borrow user payment; do
  f="$IN/postgres-$db.dump"
  [[ -f "$f" ]] || continue
  # --clean --if-exists drops then recreates each object; --no-owner +
  # --role keeps objects owned by the service's role.
  role=$db; [[ $db == user ]] && role=users
  docker exec -i bibliomania-postgres pg_restore -U "${POSTGRES_USER:-postgres}" -d "bibliomania_$db" \
    --clean --if-exists --no-owner --role="$role" --single-transaction < "$f"
  echo "  restored postgres bibliomania_$db"
done

if [[ -f "$IN/mysql-auth.sql.gz" ]]; then
  gunzip -c "$IN/mysql-auth.sql.gz" | docker exec -i -e MYSQL_PWD="${AUTH_DB_ROOT_PASSWORD:-changeme}" \
    bibliomania-mysql mysql -uroot "${AUTH_DB_NAME:-bibliomania_auth}"
  echo "  restored mysql ${AUTH_DB_NAME:-bibliomania_auth}"
fi

if [[ -f "$IN/mongo-reading.archive.gz" ]]; then
  docker exec -i bibliomania-mongo mongorestore --quiet --drop --archive --gzip < "$IN/mongo-reading.archive.gz"
  echo "  restored mongo bibliomania_reading"
fi
echo "done — start the application services again"
