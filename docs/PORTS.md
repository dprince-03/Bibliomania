# Port assignments & conflict check

Bibliomania's dev host ports are defined in [`infra/README.md`](../infra/README.md)
(the source of truth for the actual `*_HOST_PORT`/`DB_PORT`/`REDIS_PORT` env
vars) and `infra/docker/.env.example`. This file is a periodic audit log — a record
of checking that assignment against every *other* project's
containers/processes running on this development machine, since this
machine runs several unrelated Docker stacks at once. Re-run the check
below whenever a new project starts claiming ports, or before assuming the
current range is still safe — don't just trust the last recorded result.

## Current assignment (mirrors `infra/README.md`)

| Service | Env var | Port |
|---|---|---|
| nginx | `NGINX_HOST_PORT` | 9080 |
| gateway (was the Go API `app`) | `SERVER_HOST_PORT` | 9081 |
| web-main | `WEB_MAIN_HOST_PORT` | 9082 |
| web-app | `WEB_APP_HOST_PORT` | 9083 |
| admin-web | `ADMIN_WEB_HOST_PORT` | 9084 |
| admin (NestJS) | `ADMIN_API_HOST_PORT` | 9085 |
| MySQL (auth-service) | `DB_PORT` | 9086 |
| Redis | `REDIS_PORT` | 9087 |
| Adminer (dev only) | `ADMINER_HOST_PORT` | 9088 |
| Mailpit web UI (dev only) | `MAILPIT_WEB_PORT` | 9089 |
| Mailpit SMTP (dev only) | `MAILPIT_SMTP_PORT` | 9090 |
| Postgres (catalog/borrow/user/payment) | `POSTGRES_HOST_PORT` | 9091 |
| MongoDB (reading-service) | `MONGO_HOST_PORT` | 9092 |
| RabbitMQ management UI | `RABBITMQ_UI_HOST_PORT` | 9093 |
| NATS monitoring | `NATS_MONITOR_HOST_PORT` | 9094 |
| Grafana | `GRAFANA_HOST_PORT` | 9095 |
| Prometheus | `PROMETHEUS_HOST_PORT` | 9096 |
| kind cluster ingress (Traefik) HTTP — Kubernetes only | `http_host_port` (Terraform) / `kind/cluster.yaml` | 9097 |
| kind cluster ingress (Traefik) HTTPS — Kubernetes only | `https_host_port` / `kind/cluster.yaml` | 9098 |
| SeaweedFS S3-protocol API (dev only) | `S3_HOST_PORT` | 9099 |
| Alertmanager (dev only) | `ALERTMANAGER_HOST_PORT` | 9101 |

The seven services behind the gateway, Kafka, Loki, Tempo and the
exporters publish no host ports at all.

If any of these ever need to change, update `infra/docker/.env.example` and
`infra/README.md`'s table together — this file only tracks verification,
it isn't itself a config source.

## How to check for conflicts

```bash
# Every port currently listening on this machine, Docker or bare process:
{ docker ps --format '{{.Ports}}' | grep -oE '0\.0\.0\.0:[0-9]+' | grep -oE '[0-9]+$'; \
  ss -ltn | awk 'NR>1{print $4}' | grep -oE '[0-9]+$'; } | sort -n | uniq
```

Cross-reference the output against the table above. Any overlap means a
port needs reassigning in both `infra/docker/.env.example` and `infra/README.md`.

## Verification log

### 2026-08-29 — no conflicts

Checked against every other project running on this machine at the time:
`aens-*` (nginx, client, admin, worker, api, umami + its Postgres, redis),
`newshub-*` (client, server, mysql), `nexus_*` (client, umami + its
Postgres, api, worker, redis), `lms-*` (api, mysql, redis),
`so-good-catering-dev-*` (api, client, worker, postgres, redis), and a
couple of unnamed `docker-*` compose projects (server, client, postgres) —
plus every bare (non-Docker) host process listening at the time.

**Result: the entire 9080-9090 range was free.** Nearest neighbors were
`8080`/`8081` (newshub, so-good-catering) and `9100` (unrelated, not
Docker) — a clear gap on either side of Bibliomania's range.

Noted but out of scope for this check: bare (non-Docker) processes were
found listening on `3000`, `3004`, and `3005` — not part of any
Bibliomania compose service, possibly stray leftover dev-server processes
from earlier work. Worth a manual look if `npm run dev` for another
project unexpectedly fails to bind.

### 2026-09-25 — 9091-9096 added for the microservices split (partial check)

Claimed 9091-9096 for the new dev tooling ports above. **Only a partial
check was possible**: Docker Desktop (where the other projects' stacks in
the 2026-08-29 entry run) wasn't running at the time, so only bare host
processes and the system Docker engine were visible. Of those, nothing
listened in 9080-9099 except the known bare process on `9100`. The
2026-08-29 check had also found no other project in the 909x range.
**Re-run the check with Docker Desktop up** before relying on 9091-9096.

### 2026-09-25 — 9097-9098 added for the local Kubernetes cluster

The kind cluster (`infra/k8s/kind/cluster.yaml`, `infra/terraform/local`)
publishes its ingress on 9097/9098. The same partial check as above applies
(Docker Desktop wasn't running); nothing else listened there. The
`make -C infra/k8s seed/smoke` targets also open short-lived port-forwards on
19081/19086/19089 while they run.

### 2026-09-26 — 9099 (SeaweedFS) and 9101 (Alertmanager)

9099 publishes SeaweedFS's S3-protocol API in dev (the file store that
replaced catalog's local volume); 9101 publishes Alertmanager's UI in dev.
9100 was skipped: the known bare host process still listens there. Same
partial check as above — system Docker engine and bare processes only,
Docker Desktop not running.

