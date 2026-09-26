# Bibliomania infra

Docker Compose orchestration for the whole stack — 35 containers in dev:

- **Application**: the gateway + seven Go services (auth, catalog, borrow,
  reading, user, notification, payment), three web frontends, the admin
  backend.
- **Data**: MySQL (auth), Postgres (catalog, borrow, user, payment — one
  database each), MongoDB (reading), Redis (catalog's cache, the gateway's
  idempotency records and shared rate limits), SeaweedFS (e-library files —
  self-hosted object storage speaking the S3 protocol; no cloud account).
- **Brokers**: RabbitMQ, Kafka (KRaft, no ZooKeeper), NATS JetStream.
- **Observability**: Prometheus (+ alert rules), Alertmanager, Loki, Tempo,
  Grafana, Grafana Alloy (log shipping), and exporters for every database
  and broker.
- **Edge / other**: nginx, Umami analytics (+ its own Postgres), and
  dev-only Adminer + Mailpit. ClamAV (upload virus scanning) is opt-in:
  `--profile scan`.

Payments: payment-service talks to Stripe and/or Paystack — set
`STRIPE_*` / `PAYSTACK_*` in `infra/docker/.env` (test keys only; see
`Server/app/docs/API.md` → "Payments").

What each service owns and why: `Server/app/docs/plan.md` → "Microservices
split". **The same stack on Kubernetes** (kind locally, via Terraform or
`make -C infra/k8s up`): [`infra/k8s/README.md`](k8s/README.md).

## Layout

```text
infra/
├── docker/
│   ├── docker-compose.yml       # base: every service's static shape, networks, volumes
│   ├── docker-compose.dev.yml   # dev override: air hot reload, bind mounts, host ports, dev tooling
│   ├── docker-compose.prod.yml  # prod override: built images only, no host ports but nginx
│   ├── app/Dockerfile.{dev,prod}  # ALL eight Go services (SERVICE build arg / air command picks one)
│   ├── web-app/ web-main/ admin-web/ admin/   Dockerfile.{dev,prod}
│   ├── postgres/init/*.sql             # roles + databases (passwords via \getenv) + extensions
│   ├── mysql/init/*.sql                # auth MySQL server settings (slow-query log, digests)
│   ├── rabbitmq/enabled_plugins        # management UI + Prometheus endpoint
│   └── smoke-test.sh                   # end-to-end test of every cross-service path (55 checks)
├── backup/                             # backup.sh / restore.sh for Compose — see backup/README.md
├── loadtest/load.js                    # k6 load test (browse + borrow/read/return), plain JS
├── k8s/                                # Kubernetes: Kustomize base + overlays, kind config — see k8s/README.md
├── terraform/local/                    # kind cluster + Traefik + app, declaratively
├── observability/                      # shared by Compose and Kubernetes
│   ├── prometheus/prometheus.yml       # scrape targets (services + exporters)
│   ├── prometheus/alerts.yml           # alert rules (availability, errors, latency, breakers, events, reconciler)
│   ├── alertmanager/alertmanager.yml   # routing: Mailpit in dev (prod: Slack, see infra/k8s/overlays/prod)
│   ├── loki/ tempo/ alloy/             # log store, trace store, log shipper
│   └── grafana/                        # provisioned datasources + "Bibliomania — Services" dashboard
└── nginx/
    ├── Dockerfile.{dev,prod}
    └── conf.d/default.{dev,prod}.conf
```

No Dockerfile/compose service exists for `Client/mobile` (Flutter) or
`Client/desktop` (JavaFX) — both are locally-installed native apps, not
long-lived services.

## Setup

1. Copy env files:

   ```bash
   cp infra/docker/.env.example infra/docker/.env  # compose-level: DB/broker credentials, ports, Stripe, Grafana
   cp Server/app/.env.example Server/app/.env      # shared Go service config — above all JWT_SECRET
   ```

2. **`infra/docker/.env` is the source of truth for credentials** — the
   compose file builds each service's `DATABASE_URL`/`RABBITMQ_URL` from the
   same values it passes to the database/broker containers, so they can't
   disagree. Database setup is plain SQL run **on a volume's first boot
   only**: `postgres/init/*.sql` creates the four roles + databases
   (passwords read from the container env with psql `\getenv`) and
   extensions; `mysql/init/*.sql` sets persisted server settings. Changing a
   password later means `ALTER ROLE`/`ALTER USER` too.
3. Add these to `/etc/hosts` (dev only — the subdomain routing needs them):

   ```text
   127.0.0.1 bibliomania.local app.bibliomania.local admin.bibliomania.local api.bibliomania.local analytics.bibliomania.local grafana.bibliomania.local rabbitmq.bibliomania.local adminer.bibliomania.local mail.bibliomania.local
   ```

**Coming from the monolith stack?** The auth database uses a new volume
(`auth_mysql_data`), so the old `mysql_data` volume — the monolith's
`bibliomania` database — is simply no longer mounted. Its data isn't
migrated (see `Server/app/docs/plan.md` → "Still open"); `docker volume rm
bibliomania_mysql_data` once you don't need it.

## Run

All commands run from the **repo root**, pointing `--env-file` at
`infra/docker/.env` (where the file actually lives). Don't use
`--project-directory .` instead — that also changes how build contexts and
`env_file:` paths inside the compose files resolve (they're written relative
to `infra/docker/`, the compose files' own directory) and breaks them.

```bash
# Dev (or: cd Server/app && make docker-up)
docker compose --env-file infra/docker/.env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml up --build

# Prod
docker compose --env-file infra/docker/.env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.prod.yml up -d --build

# Validate config without starting anything (catches path/env mistakes early)
docker compose --env-file infra/docker/.env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml config

# Seed, then test every cross-service path end to end (needs curl + jq)
(cd Server/app && make seed) && infra/docker/smoke-test.sh
```

Dev hot reload: all eight Go containers bind-mount `Server/app` and run
air with their own build target, so an edit rebuilds and restarts each
service (they share one Go build cache volume). On first `up` each service
compiles before it answers — give `/health` a minute.

## Routes (dev)

| Subdomain | Service | Container port |
| --- | --- | --- |
| `bibliomania.local` | web-main (marketing) | 3000 (dev) / 80 (prod, static via nginx-in-container) |
| `app.bibliomania.local` | web-app (product app) | 3000 |
| `admin.bibliomania.local` | admin-web | 3000 |
| `admin.bibliomania.local/api/` | admin | 4000 |
| `api.bibliomania.local` | **gateway** — REST `/api/v1`, `/graphql`, `/swagger/` | 8080 |
| `analytics.bibliomania.local` | umami | 3000 |
| `grafana.bibliomania.local` | grafana | 3000 |
| `rabbitmq.bibliomania.local` (dev only) | RabbitMQ management UI | 15672 |
| `adminer.bibliomania.local` (dev only) | adminer | 8080 |
| `mail.bibliomania.local` (dev only) | mailpit | 8025 |

The seven services are **not** routed by nginx or published to the host —
the gateway is the only way in (plus each service's internal `/health`).

Dev also publishes host ports for local tooling, in the `9080-9101` range (skipping 9097/9098, the kind cluster, and 9100) —
deliberately not the usual `3000`/`5432`/`8080`/`6379`/etc. defaults, because
this machine runs several other projects' Docker stacks that already claim
those. Override via `infra/docker/.env` on a collision — see
[`docs/PORTS.md`](../docs/PORTS.md) for the conflict-check method and the
verification log:

| Service | Host port (env var, default) |
| --- | --- |
| nginx | `NGINX_HOST_PORT`, 9080 |
| **gateway** (was the monolith `app`) | `SERVER_HOST_PORT`, 9081 |
| web-main | `WEB_MAIN_HOST_PORT`, 9082 |
| web-app | `WEB_APP_HOST_PORT`, 9083 |
| admin-web | `ADMIN_WEB_HOST_PORT`, 9084 |
| admin | `ADMIN_API_HOST_PORT`, 9085 |
| mysql (auth) | `DB_PORT`, 9086 |
| redis | `REDIS_PORT`, 9087 |
| adminer | `ADMINER_HOST_PORT`, 9088 |
| mailpit (web UI) | `MAILPIT_WEB_PORT`, 9089 |
| mailpit (SMTP) | `MAILPIT_SMTP_PORT`, 9090 |
| postgres | `POSTGRES_HOST_PORT`, 9091 |
| mongo (use `?directConnection=true` from the host) | `MONGO_HOST_PORT`, 9092 |
| RabbitMQ management UI | `RABBITMQ_UI_HOST_PORT`, 9093 |
| NATS monitoring | `NATS_MONITOR_HOST_PORT`, 9094 |
| grafana | `GRAFANA_HOST_PORT`, 9095 |
| prometheus | `PROMETHEUS_HOST_PORT`, 9096 |
| seaweedfs (S3-protocol API) | `S3_HOST_PORT`, 9099 |
| alertmanager | `ALERTMANAGER_HOST_PORT`, 9101 |

These are only the *host*-side ports — container-to-container traffic (e.g.
nginx → `gateway:8080`, borrow → `catalog:9090`) uses internal ports
regardless. Prod publishes only nginx's port 80 (and reserves 443 for the
TLS follow-up).

## Observability

Open Grafana (`:9095` or `grafana.bibliomania.local`, login from
`GRAFANA_ADMIN_*`):

- **Dashboards → Bibliomania → "Bibliomania — Services"**: request rate,
  errors and p95 latency per service (REST and gRPC), and events published
  / consumed / dead-lettered.
- **Explore → Tempo**: a trace follows one request through the gateway, the
  services it calls over gRPC and, via the event envelope, the consumers
  that later handle its events. Tempo's service graph shows who calls whom.
- **Explore → Loki**: `{service="borrow-service"}` etc. Lines carry
  `trace_id`; click it to jump to the trace (and from a trace, to its logs).
- **Alerts**: 15 rules in `observability/prometheus/alerts.yml` (service
  or database down, 5xx and gRPC error rates, p95 latency, open circuit
  breakers, dead-lettered events, outbox publish failures, consumer lag,
  reconciler drift). Alertmanager (`:9101` in dev) emails them to Mailpit
  (`:9089`); a critical alert silences the same service's warnings.
  Validate with `promtool check rules` / `amtool check-config`.
- Database and broker exporters are all scraped (Prometheus → Status →
  Targets). Their community dashboards aren't provisioned yet — import by
  ID: MySQL 7362, Postgres 9628, MongoDB 2583, Redis 763, Kafka 7589,
  RabbitMQ 10991, NATS 2279.

## One Postgres for the app, another for Umami — don't mix them up

`postgres` hosts the four service databases (`bibliomania_catalog`,
`_borrow`, `_user`, `_payment`, one role each). `umami-postgres` belongs
entirely to Umami analytics — different credentials (`UMAMI_POSTGRES_*`),
different volume. Never point Umami at the application's Postgres or vice
versa.

## CI/CD

`.github/workflows/` has a separate `<app>-ci.yml` + `<app>-cd.yml` pair per app (app, web-app, web-main, admin-web, admin, nginx, mobile, desktop — 16 files total), each path-filtered so only the changed app's workflows run:

- **`*-ci.yml`** (every push/PR touching that app): lint/build/test using the app's own tooling (`go vet`/`build`/`test`, `npm run lint`/`build`, `flutter analyze`/`test`, `mvn package`). `app-ci.yml` also checks the generated GraphQL code is current and runs `buf lint` + `buf breaking` (against `main`) on the gRPC contracts, plus:
  - **lint** — golangci-lint (`Server/app/.golangci.yml`: standard linters + gosec, bodyclose, misspell, gofmt);
  - **integration** — `go test -tags integration` with Testcontainers (real MySQL/Postgres/Mongo per test);
  - **security** — govulncheck (fails only on vulnerabilities the code actually reaches), gosec (SARIF → the repo's Security tab), and a Trivy scan of the repo (dependency CVEs, committed secrets, Dockerfile/Kubernetes/Terraform misconfigurations). These run on the Go toolchain the images ship with (1.26.x), since standard-library fixes come from the toolchain, not go.mod.
- **`*-cd.yml`** (push to `main` only, not PRs, and not gated on the CI workflow passing — they run independently): Docker-image services build their prod Dockerfile and push to `ghcr.io/dprince-03/bibliomania-<service>:latest` + `:<sha>`. `app-cd.yml` is a matrix over the eight Go services (`bibliomania-gateway`, `-auth`, `-catalog`, `-borrow`, `-reading`, `-user`, `-notification`, `-payment`): build → **Trivy image scan** (a fixable HIGH/CRITICAL CVE stops the push) → push. Its **`promote`** job then pins those images in `infra/k8s/overlays/prod` to the commit SHA and commits that; Argo CD deploys it (`infra/k8s/README.md` → "GitOps"). `mobile-cd.yml`/`desktop-cd.yml` instead upload a build artifact (unsigned release APK / packaged jar) — there's no app-store or code-signing pipeline set up.

`.github/dependabot.yml` covers every ecosystem in the repo (gomod, npm ×4, pub, maven, docker, github-actions) on a weekly schedule.

## Known caveats

- **No auth on MongoDB, Kafka or NATS.** Fine while only nginx is exposed
  in prod; must be closed before a real deployment (MongoDB needs a
  replica-set keyfile to enable auth). mTLS between services exists on
  Kubernetes only (Linkerd); Compose has no mesh.
- `alloy` mounts the Docker socket read-only to read container logs —
  standard for log shippers, but it is host-level access.
- In **Compose**, every database and broker is single-node — a volume loss
  loses data since the last backup (`infra/backup/`). Replicated versions of
  all of them are in the Kubernetes `local-ha` overlay
  (`infra/k8s/components/ha`).
- `Server/admin` was scaffolded by `nest new --language javascript`, which has no
  compiled build step — it runs via `babel-node` even in the "prod" image, so
  `Dockerfile.prod` keeps devDependencies instead of `npm ci --omit=dev`. Once
  there's real code in this API, add a proper Babel build step
  (`babel index.js src --out-dir dist`) and switch to a `node dist/main.js`
  multi-stage build. It now gets `GATEWAY_URL` — it should call the
  services through the gateway, since there's no shared database any more.
- TLS/HTTPS for prod nginx (certbot or similar) is not set up — follow-up work.
- Migrations run inside each service on boot (embedded in the binary), never
  via a database `docker-entrypoint-initdb.d` mount: that entrypoint runs
  every `*.sql` file alphabetically once, so golang-migrate's paired
  `..._up.sql`/`..._down.sql` files would run back-to-back and undo each
  other. The only init script is Postgres's role/database creation.
