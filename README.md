# Bibliomania

📚 Bibliomania
- A Library Management & E-Library System
- Built with Go microservices (gRPC, GraphQL, REST), MySQL, Postgres, MongoDB, Redis, RabbitMQ, Kafka, NATS, Next.js, NestJS, Flutter, and JavaFX — observed with OpenTelemetry, Prometheus, Loki, Tempo and Grafana

## Monorepo layout

```
Bibliomania/
├── Server/
│   ├── app/                # Go microservices: gateway + 7 services (one module)
│   └── admin/              # NestJS — admin dashboard backend
├── Client/
│   ├── web/
│   │   ├── app/            # Next.js — the product app end-users use
│   │   └── main/           # Next.js (static export) — marketing/advertising site
│   ├── admin/              # Next.js — admin dashboard frontend
│   ├── mobile/             # Flutter — mobile app
│   └── desktop/            # JavaFX — native desktop app
└── infra/
    ├── docker/             # Dockerfiles + docker-compose (base + dev/prod overrides)
    └── nginx/              # Reverse-proxy config
```

Both real backends live under `Server/` (`app` = the Go microservices, `admin` = the NestJS admin backend) — `Client/` holds only frontends now. `Server/admin` moved out of `Client/admin/api` for exactly that reason; see `docs/plan.md`.

Every JS/TS app uses plain JavaScript (no TypeScript) and its own `package.json` (npm, no shared workspace). See [infra/README.md](infra/README.md) for how to bring the whole stack up.

## Server (Go microservices)

`Server/app` is one Go module with eight binaries — a gateway and seven
services, each with its own database (split from a modular monolith on
2026-09-25; see `Server/app/docs/plan.md` → "Microservices split").

```
Server/app/
├── cmd/
│   ├── gateway/        # the only entry point for clients: REST /api/v1 façade + GraphQL
│   ├── auth/           # registration, login, refresh-token rotation          — MySQL, RabbitMQ
│   ├── catalog/        # authors, books, search, e-library files, copy reservation — Postgres, Redis, RabbitMQ, gRPC
│   ├── borrow/         # borrowing + the borrow Saga, overdue sweep           — Postgres, Kafka, gRPC
│   ├── reading/        # progress, offline sync, bookmarks, history           — MongoDB, NATS, gRPC
│   ├── user/           # profiles, library shelf, admin user management       — Postgres, RabbitMQ, NATS, gRPC
│   ├── notification/   # email from domain events                             — RabbitMQ, Kafka
│   ├── payment/        # one-time book purchases via Stripe or Paystack        — Postgres, Kafka
│   ├── seed/           # `make seed` — admin user + sample catalog, against a running stack
│   └── key/            # JWT secret generator
├── proto/              # gRPC contracts (bibliomania.<svc>.v1) → gen/ (generated, committed)
├── internal/
│   ├── services/<svc>/ # each service's model/dto/repository/service/handler/grpc + embedded migrations
│   ├── gateway/        # REST proxy table, aggregation, GraphQL (graph/, gqlgen)
│   ├── apiversion/     # REST API versioning (/api/versions, Deprecation/Sunset)
│   ├── events/         # event envelope + types, transactional outbox, RabbitMQ/Kafka/NATS adapters
│   ├── grpcx/          # gRPC server/client, JWT forwarding, error mapping
│   ├── telemetry/      # OpenTelemetry traces, JSON logs with trace_id, Prometheus metrics
│   ├── server/         # process runner: HTTP + gRPC + workers, graceful shutdown
│   └── cache/ config/ database/ errors/ health/ middleware/ utils/ swaggerdocs/
├── pkg/                # postgresclient, mysqlclient, mongoclient, redisclient, jwt, refreshToken
├── docs/               # project_setup.md, Steps.md, API.md, plan.md (see "Docs" below)
└── Makefile            # build/test/proto/graphql/swagger/seed/docker-up/…
```

### Implemented so far

All 20 roadmap steps (see `Server/app/docs/Steps.md`) — auth with refresh-token rotation, the author/book catalog (full-text + partial search, Redis-cached), e-library upload/download, reading sessions (offline sync with last-write-wins, bookmarks), borrowing, member management, Swagger docs, health checks — now running as microservices, with:

- the same REST API as before (every `/api/v1` path, through the gateway) plus a **GraphQL** API that fans out across services;
- an orchestrated **borrow Saga** with idempotent retries and compensation, a **transactional outbox** in every service, and idempotent event consumers across RabbitMQ, Kafka and NATS;
- **API versioning** for REST, GraphQL, gRPC and events;
- email notifications and a first slice of **payments** (Stripe or Paystack, chosen by currency);
- **observability**: traces, logs and metrics for every service and database/broker, in Grafana.

Runs on **Docker Compose** and on **Kubernetes** (`infra/k8s`: Kustomize, a kind cluster created by Terraform, Traefik ingress). The same end-to-end test (`infra/docker/smoke-test.sh`, 55 checks) passes against both — including the replicated `local-ha` cluster through failovers and the Linkerd mesh.

## Infra

`infra/docker/` + `infra/nginx/` + `infra/observability/` hold the full Docker Compose setup — every app and service above, their databases (MySQL, Postgres, MongoDB, Redis) and brokers (RabbitMQ, Kafka, NATS), the Grafana observability stack, nginx, Umami analytics (+ its own Postgres), and (dev-only) Adminer + Mailpit, split into a shared base compose file plus `dev`/`prod` overrides. See **[infra/README.md](infra/README.md)** for setup and the exact run commands — don't guess at `docker compose` flags here, the project-directory/env-file interaction is non-obvious and documented there.

## Getting started

**Whole stack (recommended):** see [infra/README.md](infra/README.md).

**Server only:** the services need their databases and brokers, so the
Compose stack is the practical way to run them:
```bash
cd Server/app
cp .env.example .env      # set JWT_SECRET (go run ./cmd/key generates one)
make docker-up            # whole dev stack; first start compiles each service — give it a minute
make seed                 # admin@bibliomania.local + sample catalog
../../infra/docker/smoke-test.sh   # optional: end-to-end check of every cross-service path
```
**On Kubernetes instead:** `cd infra/terraform/local && terraform init && terraform apply`
(or `make -C infra/k8s up`), then `make -C infra/k8s seed smoke` — see
[infra/k8s/README.md](infra/k8s/README.md).

With Compose, open `http://localhost:9081/swagger/index.html` (REST reference),
`http://localhost:9081/playground` (GraphQL) and `http://localhost:9095`
(Grafana).

See [Server/app/docs/project_setup.md](Server/app/docs/project_setup.md) for env vars and the migration/schema layout, and [Server/app/docs/Steps.md](Server/app/docs/Steps.md) for the build roadmap.

## Docs

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — system-wide overview: every app, data stores, request flow
- [docs/API.md](docs/API.md) — where to find API docs (points at the two below)
- [Server/app/docs/API.md](Server/app/docs/API.md) — API conventions, versioning, GraphQL, behaviour changes from the split (endpoint list: the Swagger UI)
- [Client/docs/API.md](Client/docs/API.md) — which client app calls what, base URLs per environment
- [docs/plan.md](docs/plan.md) — running log of major initiative plans (infra branch, platform vision), as agreed and (where implemented) as actually built
- [Server/app/docs/plan.md](Server/app/docs/plan.md) — the Server-side technical plan: the microservices split (built — what, how, every decision, what's still open) and the platform vision (Steps 21+)
- [docs/TODO.md](docs/TODO.md) — living task list, kept current
- [docs/CHANGELOG.md](docs/CHANGELOG.md) — dated record of every change and bug fix, including ones too small for `docs/plan.md`
- [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) — branching, commit, and docs conventions
