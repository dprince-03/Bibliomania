# Backups and restore (Step 45)

Every service database is dumped in its engine's native format, with a
`SHA256SUMS` file that restores verify before touching anything.

| File | Database | Tool |
| --- | --- | --- |
| `postgres-{catalog,borrow,user,payment}.dump` | the four Postgres databases | `pg_dump -Fc` (restore: `pg_restore`) |
| `mysql-auth.sql.gz` | auth (MySQL) | `mysqldump --single-transaction` — consistent, no table locks |
| `mongo-reading.archive.gz` | reading (MongoDB) | `mongodump --archive --gzip` |
| `SHA256SUMS` | — | checksums of the above |

## Compose

```bash
infra/backup/backup.sh                    # → backups/<UTC timestamp>/ (git-ignored)
infra/backup/restore.sh backups/<stamp>   # DESTRUCTIVE: replaces each database
```

Both scripts run the dump/restore tools inside the running database
containers, so no client tools are needed on the host. They read
credentials from `infra/docker/.env` (falling back to `.env.example`) via
`load-env.sh`. That file parses `.env` **as data**: sourcing it as shell
breaks on values such as `MAIL_FROM=Bibliomania <no-reply@…>`, where the
`<` is a redirect.

Before restoring, stop the application services so nothing writes
mid-restore, and start them again afterwards:

```bash
docker compose --env-file infra/docker/.env -f infra/docker/docker-compose.yml \
  -f infra/docker/docker-compose.dev.yml stop gateway auth catalog borrow reading user notification payment
```

Postgres restores use `--clean --if-exists --no-owner --role=<service role>`,
so objects stay owned by each service's own role.

## Kubernetes

The `db-backup` CronJob (`infra/k8s/base/data/backup.yaml`) runs nightly
at 02:00 (cluster time). It produces the same files, uploads them with rclone to the
bucket `bibliomania-backups` on the cluster's own SeaweedFS, checks
every file landed intact (`rclone check`), and keeps the newest 7.

```bash
kubectl -n bibliomania create job --from=cronjob/db-backup db-backup-manual   # run one now
kubectl -n bibliomania logs job/db-backup-manual -c upload
```

To restore from it, download a backup set and restore with the engines'
own tools against the primary. This example uses `rclone` locally, with a
port-forward to `seaweedfs:8333` and the same `S3_*` credentials:

```bash
rclone copy store:bibliomania-backups/<stamp> ./restore-<stamp>
(cd restore-<stamp> && sha256sum -c SHA256SUMS)
```

## E-library files (object storage)

The database dumps don't include uploaded book files; those live in
SeaweedFS (bucket `bibliomania-books`). Copy that bucket to a second
location on a schedule:

```bash
rclone sync store:bibliomania-books <another-remote>:bibliomania-books
```

`<another-remote>` can be any rclone remote: another S3-protocol store, a
disk, and so on.

## Test restores

A backup you haven't restored is a guess. The scripts here were checked by
backing up the Compose stack, wiping it, restoring, and comparing row
counts (2026-09-25/26). Repeat that after schema changes.
