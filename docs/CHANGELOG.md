# Changelog

A dated, reverse-chronological record of every change and bug fix —
including the small ones that don't warrant their own entry in
[`docs/plan.md`](plan.md) (major initiative plans/decisions) or a
`Steps.md` roadmap file (structured feature work). If it happened, it gets
a line here. Newest at the top.

## 2026-09-26

Hardening pass on the microservices build, still on
`refactor/server-microservices`. The user had asked about concurrency,
idempotency, validation, downtime fallback, replication/sharding, scaling,
load balancing, UUIDs, reconciling mismatched IDs and DevOps tooling, then
said "do all one after the other so nothing gets left out", and later
"continue and do everything". Design notes: `Server/app/docs/plan.md` →
"Hardening pass (2026-09-26)".

- **Concurrency**
  - Refresh-token rotation is atomic (`TokenRepository.Consume`, a
    conditional `UPDATE`): of N concurrent refreshes with one token,
    exactly one wins. A reused token revokes every session for that user
    (OAuth reuse detection).
  - `BookRepository.Update` applies a copies *delta* in SQL, guarded by
    `available_copies + delta >= 0` (`409 ErrCopiesOnLoan`). Before, it
    wrote an absolute `available_copies` from a stale read, losing any
    borrow or return racing with a librarian's edit.
  - payment: at most one pending checkout per user and book (unique
    partial index, migration 000004). Retries get the same checkout back.
- **Validation**
  - `utils.DecodeJSON` everywhere: 1 MB cap (413), unknown fields rejected,
    no trailing data. URL fields must be `https`. Emails are normalised.
    Search terms are length-capped. GraphQL query depth is capped at 8.
  - Uploads: content is sniffed (a real `%PDF-` header, or a real EPUB
    zip) and must match the extension. Keys are random UUIDv7s. An
    optional ClamAV scan fails closed. Downloads use `http.ServeContent`
    (range requests) with `nosniff`.
- **Idempotency**
  - `Idempotency-Key` on every mutating gateway route, backed by Redis.
    Same key and body replays the stored response (`Idempotent-Replayed:
    true`); a different body gets 422; one still in flight gets 409.
  - notification-service dedupes sends through a Redis sent-log.
- **Downtime fallback**
  - Circuit breakers and bulkheads (`internal/resilience`) on every gRPC
    client and every gateway upstream: fast 503 with `Retry-After`
    instead of piling up.
  - Catalog reads serve a stale cached copy when Postgres is unreachable
    (`cache.ReadThrough`).
  - The library view degrades with `X-Degraded: catalog-service` (REST) or
    a partial result plus an error (GraphQL) instead of failing outright.
- **Rate limiting** works across gateway replicas (Redis GCRA,
  `RateLimit-*` headers), falling back to in-memory if Redis is down.
- **Scaling**
  - E-library files moved from a local volume to **SeaweedFS**, a
    self-hosted store speaking the S3 protocol (no cloud account — MinIO
    no longer publishes community images). catalog can now run more
    than one replica.
  - Kafka topics have 6 partitions. prod has HPAs for 9 Deployments.
- **Replication and backups**
  - `infra/k8s/components/ha`: CloudNativePG 3-instance Postgres, MySQL
    GTID primary plus replica, 3-member Mongo replica set, 3 KRaft Kafka
    brokers, 3-node NATS JetStream, RabbitMQ Cluster Operator (quorum
    queues). catalog reads from Postgres replicas (`READ_DATABASE_URL`).
  - Local overlay `local-ha` (`make -C infra/k8s up-ha`); prod includes
    it.
  - Backups: `infra/backup/backup.sh` / `restore.sh` (Compose) and a
    nightly `db-backup` CronJob. Both are checksum-verified and keep 7.
    The CronJob uploads with **rclone** (it used the `amazon/aws-cli`
    image before; nothing here uses AWS).
  - Sharding: a written design only, no code. See plan.md.
- **IDs**
  - Public UUIDv7 `public_id` on borrow records and purchases; REST
    accepts either ID form.
  - An auth ↔ user **reconciler** (user-service, hourly plus 30s after
    boot, one replica at a time via an advisory lock). It repairs missing
    users and identity drift, re-publishes `is_active` drift, and flags
    orphans. It reads auth's internal accounts endpoint using
    `INTERNAL_API_TOKEN`, which is now set in Compose, k8s and the
    `.env` examples.
- **DevOps**
  - CI: golangci-lint (`Server/app/.golangci.yml`), govulncheck, gosec
    (SARIF), a Trivy repo scan, `go test -race`, and Testcontainers
    integration tests (`-tags integration`). CD Trivy-scans each image
    before pushing and has a GitOps `promote` job.
  - Alerting: 15 Prometheus rules (`infra/observability/prometheus/alerts.yml`)
    and Alertmanager (Mailpit in dev; Slack in prod through a mounted
    Secret). Dev host port 9101.
  - A k6 load test (`infra/loadtest/load.js`).
  - Linkerd mTLS (`components/mesh`): gRPC ports accept meshed clients
    only. It also adds latency-aware per-request gRPC balancing that sees
    new pods immediately; DNS round-robin only sees them on re-resolve.
  - Sealed Secrets (`make seal`) and an Argo CD Application
    (`infra/k8s/argocd/`).
  - `make images-go` / `load-go` rebuild just the Go services.
- **Dependencies**
  - grpc 1.83.2, otel 1.44, pgx 5.9.2, mongo-driver 2.4.2, amqp091
    1.13.0, x/net, x/text. These clear every reachable vulnerability
    govulncheck found; go.mod stays `go 1.25.5`.
  - Images build with `golang:1.26-alpine` (1.26.8) on `alpine:3.22`.
    Go 1.25 no longer gets security fixes, and the standard library is
    compiled into the binaries.
  - RabbitMQ 4.2 everywhere (the operator's startup probe needs it).

**Bugs found and fixed.** The first four were already there; the rest were
found while verifying:
- The Redis cache never hit: values were JSON-encoded twice, so every read
  failed to decode.
- The gateway returned the wrong status after a `100 Continue` (the logging
  and idempotency wrappers took the 1xx as the final status). A 413 went out
  as 200.
- The refresh-token race and the copy-count race above.
- `backup.sh` sourced `.env` as shell. `MAIL_FROM=... <...>` is a
  redirect, so it failed. `infra/backup/load-env.sh` now parses `.env`
  as data (the Makefile's `seed` uses it too). While testing this, an
  unguarded test harness wiped the test databases on the system Docker
  engine (seeded test data only); the harness is guarded now.
- **Started by my OTel upgrade:** every service crashed at boot with a
  "conflicting Schema URL" (semconv 1.37 vs the SDK's 1.41). The resource
  is now schemaless, and `internal/telemetry` has a regression test.
- HA: Kafka, NATS and Mongo deadlocked at one pod (OrderedReady; a quorum
  member can't be Ready alone). They now use `podManagementPolicy:
  Parallel`.
- HA: the RabbitMQ 4.1 pods were killed in a loop: the operator's
  startup probe endpoint doesn't exist in 4.1 (404).
- HA: catalog's `READ_DATABASE_URL` sent the literal text
  `$(CATALOG_DB_PASSWORD)` as its password. The strategic merge put it
  ahead of the variable it references; it's a JSON-patch append now.
- HA: **MySQL replication had never been set up.** `--super-read-only`
  blocked the entrypoint's own init, and both init scripts used a bare
  `mysql` that couldn't find the init server's socket. The scripts now
  use `docker_process_sql`; read-only is persisted after setup; the
  replica doesn't create the database itself; its readiness check logs
  in for real (`mysqladmin ping` passes even when access is denied).
- HA: Postgres failover took about 3 minutes. CloudNativePG's smart
  shutdown waited for our pooled connections, with no primary in the
  meantime. `smartShutdownTimeout: 15` brought it to 17 seconds.
- `kind load` fails on multi-arch images. `make preload` falls back to a
  single-platform `ctr import`.
- "No copies left" returned 400. It's 409 now: the request is fine, the
  book's state conflicts.

**Verified**
- `go build` / `vet`, unit tests with `-race`, integration tests against
  real MySQL, Postgres and Mongo, golangci-lint and gosec: 0 issues.
  govulncheck: no reachable vulnerabilities with the 1.26.8 toolchain.
- Smoke test (55 checks) on Compose and on the `local-ha` kind cluster:
  after the HA fixes, after killing the Postgres primary plus one member
  each of Kafka, Mongo, NATS and RabbitMQ, after a second Postgres
  failover, and through the Linkerd mesh.
- A pod outside the mesh calling catalog's gRPC port gets
  `PermissionDenied`. MySQL replication was checked with a live write.
  catalog reads reach a Postgres replica. The rclone backup verified 7
  files. The reconciler ran on boot.
- Sealed Secrets round-trip on kind. The Argo CD Application passes a
  server-side dry run; a real sync needs this branch on `main`.
- k6: every threshold met (reads p95 1.8 ms, writes p95 14 ms, no rate
  limiting). Alerting end to end: stopping a service produced a
  `ServiceDown` email in Mailpit about 3 minutes later.

## 2026-09-25

- **Follow-up to the microservices split** (user: "you didn't do the
  postgres in the server/pkg, sql in infra/docker, add paystack to the
  payment, add kubernetes to infra/"):
  - **`Server/app/pkg/postgresclient`** (plus `mysqlclient` restored and
    `mongoclient` new, all sharing `pkg/sqlretry`). Every service connects
    through `pkg/`; `internal/database` only migrates.
  - **Database init as SQL**: `infra/docker/postgres/init/01-roles-and-databases.sql`
    and `02-extensions.sql` replace the shell script; passwords come via
    psql `\getenv`. `infra/docker/mysql/init/01-server-settings.sql` sets
    persisted MySQL settings (slow-query log, statement digests, strict
    `sql_mode`). Verified on fresh volumes.
  - **Paystack** alongside Stripe behind a `Provider` interface, routed by
    currency (`PAYSTACK_CURRENCIES`, default NGN/GHS/ZAR/KES). Webhooks now
    go to `/payments/webhook/{stripe|paystack}` (replacing
    `/payments/webhook`). Paystack's HMAC-SHA512 signature is checked, then
    the transaction is re-read from its Verify API. Paid amounts are checked
    against the purchase. Migration 000003 makes the columns
    provider-neutral. There are unit tests, plus an end-to-end run against
    a fake Paystack API:
    - checkout routed to Paystack;
    - a forged webhook gets 400, a signed one 200, and a duplicate is a
      no-op;
    - status comes from the Verify API even when the webhook body lies
      about the amount;
    - buying the same book again gets 409;
    - the receipt email arrives via Kafka.
  - **Kubernetes**: `infra/k8s` (Kustomize base + `local`/`prod` overlays,
    about 93 objects), `infra/k8s/kind/` (cluster and Traefik values) and
    `infra/terraform/local` (kind + Traefik + app). Traefik replaces the
    retired ingress-nginx. `terraform apply` brought up all 33 pods, all
    8 ingress hostnames route, Prometheus discovers every pod, and the
    smoke test passes 42/42 in-cluster.
  - **Code changes for Kubernetes**: round-robin gRPC load balancing
    (headless `*-grpc` Services), the prod image runs as numeric UID 10001
    (so `runAsNonRoot` works), and every service has a new `/livez`.
  - **Bugs caught by the cluster run**:
    - Overlay-added objects (the Secret, Mailpit, prod's HPA/PDBs) landed
      in `default` because the base's namespace doesn't cover them; both
      overlays now set it.
    - Generated-secret hash suffixes weren't rewritten into the base's
      references; the Secret now has a fixed name.
    - Traefik's Service stayed `LoadBalancer` (the chart wants
      `service.spec.type`), which hung Helm on kind.
    - **Design fix**: the gateway's readiness probe used its whole-system
      `/health`, so one unhealthy service would have taken the entire API
      out of rotation. It now uses `/livez`; liveness everywhere is
      `/livez`.
  - Loki's per-container label is now `app` in both Compose and
    Kubernetes. The smoke test's payments checks no longer assume no
    provider is configured (now 42 checks). Host ports 9097/9098 (kind
    ingress) were added to `docs/PORTS.md`. Secrets and Terraform state are
    git-ignored.
  - Docs: `infra/k8s/README.md` and `infra/terraform/local/README.md` are
    new; updated `Server/app/docs/{API,project_setup,plan}.md`,
    `.claude/CLAUDE.md`, `README.md`, `infra/README.md`,
    `docs/{ARCHITECTURE,TODO,PORTS,plan}.md`.

- **`Server/app` split into microservices** (branch
  `refactor/server-microservices`). The user asked for "everything", carved
  in place, with gqlgen and API versioning. The modular monolith became a
  gateway plus seven services (auth, catalog, borrow, reading, user,
  notification, payment), each with its own database: MySQL, Postgres ×4
  and MongoDB. They talk over gRPC (`proto/`) and RabbitMQ / Kafka / NATS
  JetStream, with a transactional outbox and idempotent consumers
  throughout. There's a borrow Saga with idempotency keys and queued
  compensation, a GraphQL gateway that keeps the whole `/api/v1` REST API
  working, and a first slice of Stripe payments. Observability covers the
  full Grafana stack plus exporters for every database and broker. Compose,
  Dockerfiles, nginx and CI/CD (one image per service, `buf lint` +
  `breaking`) were rewritten. Details and every decision:
  `Server/app/docs/plan.md` → "Microservices split — Server-side
  implications" → "Implementation"; root `docs/plan.md` has a dated
  summary.
- **API versioning added across every interface** (requested mid-build).
  REST uses URL paths, with `GET /api/versions`, a JSON 404 for unsupported
  versions, and RFC 9745/8594 Deprecation/Sunset headers driven by
  `API_DEPRECATED_VERSIONS`. gRPC uses proto packages, with `buf breaking`
  in CI. Events carry `schema_version` and consumers reject unknown
  versions. GraphQL evolves additively with `@deprecated`. Documented in
  `Server/app/docs/API.md` → "Versioning".
- **Verified end to end**: the backend stack (Go services, databases,
  brokers, observability) ran on the system Docker engine. Docker Desktop
  wasn't running. New `infra/docker/smoke-test.sh` passes all 41 checks.
  Prometheus scrapes all 18 targets, a request is traced across
  gateway → borrow → catalog, and logs carry the same `trace_id`. `web/app`
  was run against the gateway and renders live data unchanged. The first
  unit tests in the repo cover the borrow Saga, versioning and the payments
  launch gate.
- **Bugs found while building and testing:**
  - OpenTelemetry semconv version mismatch: every service refused to start.
  - A stale offline sync returned 500 instead of being discarded. The
    duplicate-key "loser" detection aborted the MongoDB transaction; it now
    does an explicit existence check.
  - Request log lines lacked `trace_id` because slog was called without the
    request context.
- **Pre-existing monolith bugs fixed along the way:**
  - Book updates never persisted the adjusted `available_copies`.
  - Cache invalidation only cleared three hard-coded list-page keys.
  - CORS rejected `PATCH` and sent a date string as `Access-Control-Max-Age`.
  - Reading counters were never incremented.
  - `redisclient` used an invalid `%m` verb.
  - The Makefile's docker targets pointed at the removed root `.env`.
- **Client + docs follow-through:** `web/app`'s local API fallback and
  `.env.example` now point at the gateway (`localhost:9081`). Compose's
  `API_INTERNAL_URL` is `http://gateway:8080`. `Server/admin` gets
  `GATEWAY_URL`, since there's no shared database any more. Updated
  `.claude/CLAUDE.md`, `README.md`, `infra/README.md`,
  `docs/{ARCHITECTURE,API,TODO,PORTS,plan}.md`, `docs/interfaces/README.md`
  (drafts marked superseded) and `Client/docs/API.md`, plus all four
  `Server/app/docs/` files.
- **Host ports 9091-9096 claimed** for Postgres, MongoDB, RabbitMQ UI, NATS
  monitoring, Grafana and Prometheus. The conflict check was only partial
  (Docker Desktop not running); logged in `docs/PORTS.md`, re-check listed
  in `docs/TODO.md`.
- **Not done:**
  - The monolith's existing dev data isn't migrated.
  - `web/app` still uses REST, not GraphQL.
  - No mTLS, and no auth on MongoDB, Kafka or NATS.
  - No Terraform or Kubernetes.
  - All tracked in `docs/TODO.md`.

- **Brought the rest of `docs/` up to date** (flagged by the user: several
  docs hadn't been touched since the web-app roadmap, the rename, or the
  microservices-split decisions):
  - `TODO.md`: `Client/web/app` marked done (all 7 steps, previously
    still "bare Next.js template"). The Step 18 note now says it landed.
    `admin-api` became `Server/admin`/`admin`. Added the mock-up Artifact
    links to the admin/mobile/desktop items, notes on Steps 27/37, and a
    new "`Server/app` microservices split" checklist (done decisions,
    remaining contracts, open decisions, restructure, infra/ports).
  - `ARCHITECTURE.md`: replaced "every Client app is a bare scaffold" with
    what's actually built vs. not. Added a "Planned: microservices split"
    section (service/DB/broker table), noting that the diagram and data
    stores still describe the current system.
  - `CONTRIBUTING.md`: dropped the stale "currently Steps 13-20" and
    covered the separate Client roadmaps. Added the `CHANGELOG.md` rule,
    `plan.md`'s append-only/strike-through-correction convention, and
    `docs/interfaces/`.
  - `API.md`: pointer to `docs/interfaces/`, clearly marked as planned,
    not live.
  - `PORTS.md`: noted that the split's new containers have no ports
    assigned yet and must be conflict-checked first.
  - `interfaces/README.md`: documented that `enrichBorrows` doesn't
    snapshot titles today. Flagged an open inconsistency:
    `gateway.graphql`'s `BorrowRecord` resolves `book` live, with no
    snapshotted title field. It needs a decision, so the schema is
    unchanged.
  - `README.md`: `Server/app/docs/plan.md` link description now mentions
    the split.

- **Server docs caught up with the microservices-split decisions** — they
  had only been recorded in root `docs/plan.md`, leaving
  `Server/app/docs/` silent about a change that redraws its whole module
  layout (flagged by the user). Added a "Microservices split — Server-side
  implications" section to `Server/app/docs/plan.md` (service/DB/broker
  map, which in-process cross-module calls get replaced by what, impact on
  Steps 27/37, open questions), plus pointers from its intro and from
  `Server/app/docs/Steps.md`'s status block. Surfaced one new open
  question: build order between the split and Steps 21-45.
- **Corrected a factual error in root `docs/plan.md`** (data-ownership
  section): it claimed `borrow.Service.enrichBorrows` already snapshots
  book titles. It actually does a live `bookRepo.GetByID` per record on
  every read (an N+1), and `BorrowRecord` has no title column. Struck
  through in place with a dated correction rather than silently rewritten.

- **Mobile & desktop UI mock-ups added** — a second Design canvas Artifact
  (the admin dashboard above was scoped separately and stayed in its own
  canvas), 9 screens: `Client/mobile` (Catalog, Book detail, My Library,
  My Borrows, Reader, Account — touch-first, bottom tab bar) and
  `Client/desktop` (Library, Reader, Sync & storage — windowed, sidebar
  nav). Unlike the admin dashboard's own deliberately utilitarian look,
  these carry `web/app`'s actual established brand (gold/navy/royal-blue
  palette, Cinzel/EB Garamond, blackletter wordmark) since they're the
  same product on different platforms, not a distinct tool. The mobile
  Reader and desktop Sync & Storage screens are the two that concretely
  visualize the SQLite-offline decision from `docs/plan.md` — a
  "Downloaded / synced 2 min ago" indicator and a local-storage/pending-
  sync breakdown, respectively — rather than leaving that decision as
  text with nothing to point at. This was flagged as a gap by the user
  (only the admin mock-up had been delivered against the three identified
  new-interface surfaces) and completed in the same sitting.

- **Mock-up phase started for the microservices split** (see `docs/plan.md`
  for the full decisions this builds on). Two deliverables:
  - **Interface contracts**: new `docs/interfaces/` — `catalog.proto` (the
    one gRPC contract with real cross-service correctness dependencies:
    `ReserveCopy`/`ReleaseCopy`/`GetBook`/`GetBooksByIds`, used by
    `borrow-service` and `reading-service`), `gateway.graphql` (the
    GraphQL gateway schema `web/app` will talk to — `Book.myBorrowStatus`/
    `.myLibraryStatus` and `myHistory` are the concrete display-aggregation
    fan-outs from the data-ownership decision), and a `README.md` noting
    what's mechanically left (per-service proto files for `auth`/`borrow`/
    `reading`/`user`, REST OpenAPI specs) as a follow-up, not started here.
  - **UI mock-up**: a 7-screen admin dashboard design (Dashboard, Books,
    Book edit, Authors, Users, Borrows, Curation queue), published as a
    Design canvas Artifact — `Client/admin` is still a bare scaffold with
    no screens designed, unlike `web/app`/`web/main` which already have
    real, built, working UI and didn't need mock-ups. Curation queue is a
    deliberate teaser of the platform-vision "Library Selected" badging
    workflow (`docs/plan.md`'s platform-vision section), not yet a real
    backend feature.

## 2026-09-24

- **Started this changelog.** User asked for every plan, decision, change,
  and bug fix to be documented from now on, with nothing left to only live
  in chat history. Plans/decisions still go in `docs/plan.md`; structured
  feature work still goes in each app's own `Steps.md`; this file is the
  new home for everything else, including changes too small to justify a
  `docs/plan.md` section.

## 2026-08-29

- **Moved `.env.example` from the repo root to `infra/docker/.env.example`**
  — it configures the Docker Compose stack specifically (not the whole
  repo), so it now lives beside the compose files that actually consume
  it, matching how `Server/app/.env.example` already sits inside
  `Server/app/`. Updated every doc that referenced the old root path/command
  (`.claude/CLAUDE.md`, `infra/README.md`, `Client/docs/API.md`,
  `docs/PORTS.md`, `docs/plan.md`) to `--env-file infra/docker/.env`.
  Verified both dev and prod Compose overlays still resolve variables
  correctly from the new location.
- **Moved `CLAUDE.md` to `.claude/CLAUDE.md`** (`git mv`, history
  preserved) at the user's request. Fixed the one relative link this broke
  (`docs/ARCHITECTURE.md`) and updated every other prose mention of the
  file across `README.md`, `docs/CONTRIBUTING.md`, `docs/plan.md`,
  `docs/TODO.md`, `Server/app/docs/plan.md`,
  `.github/pull_request_template.md`, and both dev/prod compose files.
  Flagged one open question for the user: unconfirmed whether Claude Code
  auto-loads project memory from `.claude/CLAUDE.md` the same way it does
  from a root-level `CLAUDE.md`.
- **Removed the root `.env` file** created earlier the same day to test the
  Docker stack — user didn't want a general one sitting around. Left
  `Server/app/.env` (the real, pre-existing per-service dev config)
  untouched.
- **Fixed a Redis config-parse crash in `infra/docker/docker-compose.yml`.**
  The `redis` service's `command:` was written as a folded YAML string
  (`command: >`); with `REDIS_PASSWORD` empty, `--requirepass
  ${REDIS_PASSWORD:-}` collapsed to nothing, shifting every flag after it
  by one slot — Redis received `--requirepass --appendonly yes` and failed
  to parse `--appendonly` as a password value. Converted `command:` to
  explicit YAML list form, where each argument is its own list entry
  regardless of whether a variable resolves empty. Verified by starting
  just the `redis` service and confirming `(healthy)`. Pre-existing bug,
  unrelated to the rename below — only surfaced because the freshly-created
  root `.env` (see above) had no `REDIS_PASSWORD` set, unlike whatever the
  user had been using before.
- **Fixed a Docker build failure: `air-verse/air@latest` requires Go
  ≥1.26.** `infra/docker/app/Dockerfile.dev` installs `air` for hot reload
  on top of `golang:1.25.5-alpine`; `air`'s `@latest` had drifted to
  v1.67.4, which needs a newer Go toolchain than the pinned base image
  ships. Pinned to `air@v1.66.1` (the newest release confirmed to still
  install cleanly on Go 1.25.5, checked by testing several tags directly
  against the `golang:1.25.5-alpine` image). Verified the full `app` build
  stage completes end-to-end after the pin. Pre-existing bug, unrelated to
  the rename below — `@latest` was always going to break eventually,
  independent of anything else changing.
- **Renamed the project from Bibliotheca to Bibliomania**, end to end, per
  the user's decision. Scope, in order:
  - GitHub repository renamed via `gh repo rename` (`dprince-03/Bibliotheca`
    → `dprince-03/Bibliomania`); git remote `origin` updated automatically.
  - Go module path `github.com/dprince-03/Bibliotheca` →
    `.../Bibliomania` rewritten across every `.go` file and `go.mod`;
    JWT issuer claim, health-check service name, Redis cache namespace,
    seed admin email, and `DB_NAME` default all renamed too. Swagger docs
    regenerated via `swag init` (not hand-edited) so they stay a faithful
    reflection of source. `go build ./...`/`go vet ./...` clean throughout.
  - Docker image/container names, the Compose network name
    (`bibliomania_network`), and both nginx configs renamed; `docker
    compose config` validated clean for dev and prod.
  - All 8 `*-cd.yml` CI workflows' `ghcr.io/dprince-03/bibliotheca-*` image
    push targets renamed to `bibliomania-*`.
  - `web/app` and `web/main` — logo wordmark, page titles/metadata, footer
    copyright, and marketing copy renamed; both apps' `lint`/`build` clean
    afterward.
  - Desktop (JavaFX) Java package `com.bibliotheca.desktop` →
    `com.bibliomania.desktop` (files moved via `git mv`); mobile (Flutter)
    Kotlin package, Android `namespace`/`applicationId`, and iOS bundle
    identifier all renamed to `com.bibliomania.mobile`.
  - Every doc (`CLAUDE.md`, root `README.md`, everything under `docs/`)
    renamed throughout. A full `git ls-files | grep -i bibliotheca` across
    the tracked repo came back empty afterward.
  - **Deliberately left alone**: `Server/app/.env` (gitignored, real local
    credentials) still points at a MySQL database literally named
    `bibliotheca` — renaming a live database is a data operation, not a
    text rename, and was flagged to the user rather than done silently.
    The pre-existing seeded admin login (`admin@bibliotheca.local`) still
    works; re-running `make seed` now creates an *additional*
    `admin@bibliomania.local` account rather than recognizing the old one.
