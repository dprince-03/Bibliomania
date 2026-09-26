# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Bibliomania is a Library Management & E-Library System, now a multi-app monorepo:
- **`Server/app`** — Go **microservices** (split from a modular monolith on 2026-09-25): one Go module, eight binaries in `cmd/` — `gateway` (GraphQL + versioned REST façade, the only thing clients reach) and seven services, `auth` (MySQL), `catalog` / `borrow` / `user` / `payment` (Postgres), `reading` (MongoDB) and `notification` (no DB). gRPC between services, events over RabbitMQ / Kafka / NATS JetStream, Redis cache, JWT auth, `net/http` (stdlib router, no framework), `sqlx`. Module path `github.com/dprince-03/Bibliomania` (matches the GitHub repo exactly, case included — that's what makes `go get`/module-proxy resolution work if this module is ever imported elsewhere), Go 1.25.
- **`Server/admin`** — NestJS, admin dashboard backend, scaffolded in **plain-JavaScript mode** (`nest new --language javascript`) — no compiled build step, runs via `babel-node` even in prod. Lives under `Server/` (not `Client/`) because it's a real backend, not a frontend — moved out of `Client/admin/api` for exactly that reason; see `docs/plan.md` for when/why.
- **`Client/web/app`** — Next.js, the product app end-users browse (`output: "standalone"`).
- **`Client/web/main`** — Next.js, the marketing/advertising site (`output: "export"`, static).
- **`Client/admin`** — Next.js, admin dashboard frontend (`output: "standalone"`).
- **`Client/mobile`** — Flutter (Android + iOS).
- **`Client/desktop`** — JavaFX (Maven), hand-written scaffold (Maven wasn't installed when this was created, so it wasn't generated via `mvn archetype:generate` — verify it still builds before trusting the pom.xml blindly).
- **`infra/docker/`** + **`infra/nginx/`** + **`infra/observability/`** — Docker Compose orchestration (base + dev/prod overrides) for the whole stack: services, databases (with plain-SQL init in `infra/docker/{postgres,mysql}/init/`), brokers, the Grafana stack, nginx, Umami, dev-only Adminer/Mailpit. See `infra/README.md`.
- **`infra/k8s/`** + **`infra/terraform/local/`** — the same stack on Kubernetes: Kustomize base + `local`/`prod` overlays, a kind cluster (Terraform or `make -C infra/k8s up`), Traefik ingress. Shares the SQL init and observability files with Compose. See `infra/k8s/README.md`.
- **`.github/workflows/`** — separate `<app>-ci.yml` + `<app>-cd.yml` per app, path-filtered; CD (push to `main` only) builds+pushes to `ghcr.io/dprince-03/bibliomania-<service>`. See `infra/README.md`'s "CI/CD" section.

**Every JS/TS app is plain JavaScript, no TypeScript**, each with its own independent `package.json` (npm, no shared workspace) — this was an explicit, deliberate choice; don't introduce TypeScript or a monorepo tool (pnpm workspaces, etc.) without checking first.

## Commands (all run from `Server/app/`)

`Server/app/Makefile` wraps the common ones. `.env` files are sourced at the shell level, not via Make's `include` — Make's comment character is `#`, and this repo's password values can contain a literal `#`, which `include` would silently truncate.

```bash
go build ./... && go vet ./... && go test ./...
make build                     # all eight binaries into bin/
make run SERVICE=catalog       # one service on the host (needs its infra — easiest with the Compose stack up)
make docker-up / docker-down / docker-logs   # whole dev stack
make seed                      # admin (via auth's DB + outbox) and sample catalog (via the gateway's REST API); stack must be up
../../infra/docker/smoke-test.sh   # 55-check end-to-end test of every cross-service path (needs curl + jq, seeded stack)
go test -tags integration ./...    # Testcontainers: real MySQL/Postgres/Mongo (needs Docker; DOCKER_HOST if Docker Desktop is off)
golangci-lint run ./...            # config: .golangci.yml (CI runs it, plus govulncheck/gosec/Trivy)
go run ./cmd/key               # generate a JWT secret

make proto      # proto/ → gen/ (buf + protoc-gen-go + protoc-gen-go-grpc — install commands in docs/project_setup.md)
make graphql    # internal/gateway/graph/schema.graphqls → generated.go (gqlgen is a go.mod tool)
make swagger    # regenerate internal/swaggerdocs/ from every service's handler annotations (served by the gateway)

migrate create -ext sql -dir internal/services/<svc>/migrations -seq <name>   # new migration for one service
```

Each service applies its own **embedded** migrations on boot — there's no shared `migrations/` directory any more. Unit tests cover the borrow Saga, API versioning, payments, idempotency, resilience, middleware, uploads and telemetry; `*/integration_test.go` (build tag `integration`) prove the concurrency guarantees and the reconciler against real databases (helpers in `internal/testsupport`); `infra/loadtest/load.js` is a k6 load test. Everything else is covered by the smoke test. Generated code (`gen/`, `internal/gateway/graph/generated.go`, `internal/swaggerdocs/`) is committed; CI checks the GraphQL output is current and runs `buf lint` / `buf breaking`.

Each `Client/` Node app (`web/app`, `web/main`, `admin`) has its own `package.json` and standard scripts (`npm run dev`/`build`/`lint`). `Server/admin` (NestJS) has the same shape, just no `build`/`lint` script (plain-JS mode has no compiled build step). `Client/mobile` is a standard Flutter project (`flutter run`/`flutter build`). `Client/desktop` needs Maven (`mvn javafx:run`) — not installed in every environment, don't assume it's there.

### Kubernetes (same stack, local kind cluster)

```bash
cd infra/terraform/local && terraform init && terraform apply   # or: make -C infra/k8s up
make -C infra/k8s up-ha        # replicated + meshed variant (overlays/local-ha; needs operators, ~10 GB RAM)
make -C infra/k8s images-go load-go   # rebuild only the Go images after a backend change
make -C infra/k8s seed smoke status down
kubectl kustomize --load-restrictor LoadRestrictionsNone infra/k8s/overlays/local   # render (shared files live outside the kustomization root)
```

Manifests are Kustomize (`infra/k8s/base` + `overlays/{local,local-ha,prod}` + opt-in `components/{ha,mesh,virus-scan}`); anything an overlay or component adds must be namespaced explicitly (`namespace: bibliomania` is repeated on purpose). Patches that add env vars referencing other vars (`$(VAR)`) must be JSON-patch appends — a strategic merge inserts them first and the reference doesn't expand. Quorum StatefulSets need `podManagementPolicy: Parallel`. Prod: Argo CD (`infra/k8s/argocd/`) + SealedSecrets (`make seal`); CD pins image tags in `overlays/prod` with a one-line edit — keep each `images:` entry on one line. Service names/ports match Compose so configs are shared. Gateway readiness uses `/livez`, never its aggregate `/health`.

### Docker / Compose (whole stack)

Run from the **repo root**, with `--env-file infra/docker/.env` (not
`--project-directory .` — see `infra/README.md` for why that flag breaks
path resolution here):
```bash
docker compose --env-file infra/docker/.env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml up --build   # dev
docker compose --env-file infra/docker/.env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.prod.yml up -d --build # prod
docker compose --env-file infra/docker/.env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml config         # validate only
```

## Architecture

Full reference: `Server/app/docs/plan.md` → "Microservices split — Server-side implications" (what each service owns and why, and every decision made while building it), `Server/app/docs/project_setup.md` (layout, env, databases), `Server/app/docs/API.md` (REST/GraphQL conventions, versioning, behaviour changes).

- **Layout**: `cmd/<service>/main.go` wires one service explicitly (no DI framework). Each service's code lives in `internal/services/<service>/` — model/dto/repository/service/handler, a `grpc.go` server where it has one, and `migrations/` embedded via `embed.FS`. Cross-cutting infra stays directly under `internal/`: `apiversion`, `cache`, `config`, `database`, `errors`, `events`, `gateway`, `grpcx`, `health`, `middleware`, `server`, `telemetry`, `utils`. `internal/errors.AppError` is the one error type — handlers translate it to HTTP, `grpcx` to gRPC codes and back (so a 404 stays a 404 across a hop). `utils.Success`/`utils.Error`/`utils.HandleError` write the JSON envelope.
- **Database per service — never reach into another service's database.** Cross-service data goes over gRPC (write-time correctness checks) or events (propagating facts). Display-only aggregation belongs in the **gateway**, never in a service calling another service to enrich its own response. There are no foreign keys across services; `user_id` is trusted because it comes from a verified JWT.
- **Every service verifies the JWT itself** (shared `JWT_SECRET`). The gateway forwards the bearer token; `grpcx` carries it as gRPC metadata and re-verifies it server-side, putting identity in ctx under the same keys `middleware.AuthGuard` uses — so `middleware.GetUserID(ctx)` works in HTTP and gRPC handlers alike. Never pass a bare user ID between services as proof of identity.
- **Events**: always through the **transactional outbox** (`events.AddToOutbox` inside the same tx as the business write, or `MongoOutbox.Add` in reading-service) — never publish directly after a commit. Event types and payloads are the shared contract in `internal/events/types.go`; changing a payload incompatibly means bumping its `CurrentVersions` entry. Consumers must be idempotent (`events.MarkProcessed` in the same tx) — every broker here is at-least-once. Broker per producer is fixed: auth/user/catalog → RabbitMQ, borrow/payment → Kafka, reading → NATS JetStream; one flow never crosses brokers.
- **`catalog`** — authors + books in one package (`author_*.go` / `book_*.go`), for the same reason as before: `BookResponse` embeds authors. E-library files go through `internal/storage` (SeaweedFS over the S3 protocol in Compose/k8s — self-hosted, no cloud account; `local` for bare `go run`); uploads are content-sniffed and optionally ClamAV-scanned. `BookRepository.Update` applies copy changes as a relative delta guarded by `>= 0` — never write an absolute `available_copies`. Owns Redis caching (`cache.ReadThrough`, which serves a stale copy when Postgres is down; invalidation by prefix via `Cache.DeletePrefix`; keys only in `internal/cache/keys.go`), Postgres full-text search (generated `search_vector` + pg_trgm), and the catalog half of the borrow Saga: `ReserveCopy`/`ReleaseCopy`, idempotent per reservation key (`copy_reservations`) — its `available_copies > 0` guard is the real concurrency safety.
- **`borrow`** — owns the orchestrated **Saga** (GetBook → ReserveCopy → create record + outbox event → compensate with ReleaseCopy; queued in `saga_compensations` if catalog is down). `Idempotency-Key` header makes `POST /borrows` retry-safe. Returns release the copy *before* marking returned. Title and email are **snapshotted** onto the record. Overdue detection runs both on reads and on a background sweeper (emits `borrow.borrow_overdue`).
- **`reading`** — MongoDB (single-node replica set, needed for transactions). Last-write-wins on `client_updated_at_ns` (integer nanoseconds — BSON dates are only millisecond precision). Validates the book against catalog only when a session/bookmark is *created*, never on progress updates. Owns `GET /users/me/history`.
- **`user`** — profiles, library shelf, admin user management. Its `users` table is a copy of auth's accounts (via `auth.user_registered`); `is_active` is written here and flows back to auth via `user.status_changed`. Reading counters are cached from `reading.book_completed`. `GET /users/me/library` is answered by the gateway (shelf here + live titles from catalog).
- **`auth`** — registration/login/logout/refresh; refresh tokens are hashed and rotated one-time-use via an atomic `Consume` — a reused token revokes all the user's sessions. Login never depends on another service. `/internal/v1/accounts` (X-Internal-Token = `INTERNAL_API_TOKEN`, never routed by the gateway) feeds user-service's reconciler, which repairs auth ↔ user drift hourly.
- **`payment`** — one-time purchases through a `Provider` interface (`provider.go`): Stripe and Paystack, chosen by the book's currency (`PAYSTACK_CURRENCIES` → Paystack, else Stripe). Webhooks at `/payments/webhook/{provider}`; never mark a purchase paid unless the provider-reported amount/currency matches (Paystack's is re-read from its Verify API). Live keys refused unless `PAYMENTS_LIVE_ENABLED=true`.
- **`gateway`** — REST routes are a proxy table in `internal/gateway/gateway.go`; add a new service route there. It also owns every edge concern: CORS, security headers, global + strict `/auth` rate limiting (Redis-backed, shared across replicas), `Idempotency-Key` on every mutating route (`internal/idempotency`), per-upstream circuit breakers + bulkheads (`guard.go`), API-version checks.
- **Resilience** (`internal/resilience`): every gRPC client (`grpcx.Dial`) has a circuit breaker that trips on infrastructure errors only; an open breaker returns `Unavailable` → HTTP 503 with `Retry-After`. Reads may degrade (stale cache, partial GraphQL); writes never pretend to succeed.
- **Request handling**: decode JSON bodies with `utils.DecodeJSON` (1 MB cap, unknown fields rejected) and validate with `utils.NewValidator()` (adds `https_url`). Records exposed to clients get a UUIDv7 `public_id` (`utils.NewPublicID`); handlers accept either ID form (`utils.GetPathRef`). GraphQL resolvers in `internal/gateway/graph/` fan out over gRPC.
- **API versioning**: REST by URL path (`/api/v1`, `internal/apiversion`, `GET /api/versions`, Deprecation/Sunset headers), gRPC by proto package (`bibliomania.<svc>.v1` — a breaking change means a `v2` package; `buf breaking` enforces it), events by `schema_version`, GraphQL by additive evolution + `@deprecated`. See `Server/app/docs/API.md` → "Versioning".
- **Middleware** is composed in `server.Middleware` (tracing, request ID, logging, recovery, metrics — `telemetry.HTTPMetrics` must stay innermost, see its comment) for every service; the gateway adds its edge chain on top. Route guards: `middleware.NewGuards(jwt).Member/Librarian/Admin(...)`. Roles (`admin`, `librarian`, `member`) are defined once in `middleware/rbac.go` — never compare against raw role strings elsewhere.
- **Observability**: log with `slog.*Context(ctx, ...)` so lines get `trace_id` (Grafana links Loki ↔ Tempo); every service exposes `/metrics` and `/health` (the gateway's `/health` covers the whole system). Alert rules live in `infra/observability/prometheus/alerts.yml` (shared by Compose and k8s); a new metric worth paging on gets a rule there. The OTel resource is schemaless on purpose — pinning a semconv schema URL crashes boot on the next SDK upgrade (`internal/telemetry` has a test).
- **Pagination** goes through `utils.GetPagination(r)` (HTTP) / `utils.NewPagination(page, limit)` (gRPC, GraphQL) → `utils.NewPaginatedResponse(items, total, page, limit)`.
- **Database connections** go through `pkg/postgresclient`, `pkg/mysqlclient`, `pkg/mongoclient` (and `pkg/redisclient`) — they retry while the database starts. `internal/database` only runs migrations.
- **Config** (`internal/config`) — one `Config` for all binaries; `.env` is optional (skipped in production); each `cmd/<service>` declares its must-haves with `cfg.Require(...)` — the only place required-field checks belong. HTTP port env var is `SERVER_PORT` (default `8080`), gRPC is `GRPC_PORT` (`9090`).
- Naming: no redundant prefixes once the package name carries the meaning — `catalog.AuthorService`/`BookService` (two services, need disambiguating), `borrow.Repository` (one per package).
- **Go toolchain**: go.mod says `go 1.25.5` (minimum language version — build with `GOTOOLCHAIN=local` so it isn't bumped); the images build with `golang:1.26-alpine` because the stdlib's security fixes come from the builder. When upgrading a dependency, check go.mod's `go` line didn't move.
- **Docker images** are named `bibliomania-<service>` (`bibliomania-gateway`, `bibliomania-catalog`, …, plus the client apps). All eight Go services share `infra/docker/app/Dockerfile.{dev,prod}` — the `SERVICE` build arg (prod) or the per-service air command (dev) picks the binary. Dockerfiles live under `infra/docker/<service>/` (nginx keeps its own in `infra/nginx/`); the base compose file declares `build.context` (+ `args.SERVICE`), the dev/prod overrides add `build.dockerfile`. The monolith's `app` Compose service is gone — the **gateway** took over its host port (9081) and its role behind `api.bibliomania.local`; the admin backend's service is `admin`.

### Roadmap and current state

`Server/app/docs/Steps.md` is the authoritative step-by-step roadmap (Steps 1-20 done as the monolith; Steps 21-45 for the platform vision — 🟡 27 and 37 partly built by the split) — check it before assuming something exists. The microservices split itself (not a numbered Step) is done: all seven services + gateway, verified end to end on the full Compose stack. Kubernetes (Kustomize + kind + Terraform) and Paystack were added the same day; a hardening pass followed on 2026-09-26 (concurrency, idempotency, resilience, HA, mesh, CI security, alerting, GitOps — `docs/CHANGELOG.md`, `Server/app/docs/plan.md` → "Hardening pass"). What's still open — unmigrated monolith data, `web/app` still on REST rather than GraphQL, no broker/database auth, no real cloud cluster, sharding (designed only) — is listed in `Server/app/docs/plan.md` → "Still open (after the build)" and `docs/TODO.md`.

## Git workflow for this repo

Each roadmap step was implemented on its own branch — one step per branch, not bundled. Branch naming: `feat/step-<N>-<short-slug>` (e.g. `feat/step-20-final-polish`). After a step lands, update the corresponding entry in `Server/app/docs/Steps.md` (status marker + notes) as part of that same branch/PR. All 20 Server steps are now done; this convention still applies to any future numbered Server step, and non-roadmap work (infra, refactors) gets its own descriptively-named branch outside this numbering, e.g. `feat/infra-docker-stack`, `refactor/server-feature-structure`, `refactor/server-microservices`.
