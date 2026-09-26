#!/usr/bin/env bash
# Back up every service database of the Compose stack (Step 45).
#
#   infra/backup/backup.sh [output-dir]        # default: backups/<UTC timestamp>/
#
# One native-format dump per database, taken inside the running database
# containers (no client tools needed on the host):
#   postgres-<db>.dump   pg_dump custom format (compressed; restore with pg_restore)
#   mysql-auth.sql.gz    mysqldump --single-transaction (consistent, no locks)
#   mongo-reading.archive.gz  mongodump --archive --gzip
#   SHA256SUMS           checksums, verified by restore.sh
# Object storage (e-library files) is backed up separately — see README.
set -euo pipefail

cd "$(dirname "$0")/../.."
ENV_FILE=${ENV_FILE:-infra/docker/.env}
[[ -f "$ENV_FILE" ]] || ENV_FILE=infra/docker/.env.example
. infra/backup/load-env.sh
load_env "$ENV_FILE"

OUT=${1:-backups/$(date -u +%Y%m%dT%H%M%SZ)}
mkdir -p "$OUT"
echo "backing up to $OUT"

for db in catalog borrow user payment; do
  docker exec bibliomania-postgres pg_dump -U "${POSTGRES_USER:-postgres}" -Fc "bibliomania_$db" > "$OUT/postgres-$db.dump"
  echo "  postgres bibliomania_$db  $(du -h "$OUT/postgres-$db.dump" | cut -f1)"
done

docker exec -e MYSQL_PWD="${AUTH_DB_ROOT_PASSWORD:-changeme}" bibliomania-mysql \
  mysqldump -uroot --single-transaction --routines --triggers --set-gtid-purged=OFF "${AUTH_DB_NAME:-bibliomania_auth}" \
  | gzip > "$OUT/mysql-auth.sql.gz"
echo "  mysql ${AUTH_DB_NAME:-bibliomania_auth}  $(du -h "$OUT/mysql-auth.sql.gz" | cut -f1)"

docker exec bibliomania-mongo mongodump --quiet --db bibliomania_reading --archive --gzip > "$OUT/mongo-reading.archive.gz"
echo "  mongo bibliomania_reading  $(du -h "$OUT/mongo-reading.archive.gz" | cut -f1)"

(cd "$OUT" && sha256sum -- * > SHA256SUMS)
echo "done"
