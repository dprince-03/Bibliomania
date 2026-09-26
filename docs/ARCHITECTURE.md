# Architecture

High-level map of the system. For implementation detail on any one piece,
follow the links rather than duplicating it here.

## Apps

| App             | Path                 | Tech                          | Role                                                                                                                                                                            |
| --------------- | -------------------- | ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| gateway + 7 services | `Server/app/` | Go microservices (one module, `cmd/<service>`) | The API every client talks to, via the **gateway** (REST `/api/v1` + GraphQL). Behind it: auth, catalog, borrow, reading, user, notification, payment — see "Server/app: microservices" below and [`.claude/CLAUDE.md`](../.claude/CLAUDE.md). |
| web/app         | `Client/web/app/`  | Next.js (JS)                  | The product — the library app end-users browse/borrow/read from.                                                                                                               |
| web/main        | `Client/web/main/` | Next.js (JS), static export   | Marketing/advertising site. No server-side logic — pure static HTML/CSS/JS.                                                                                                    |
| admin           | `Client/admin/`    | Next.js (JS)                  | Admin dashboard UI.                                                                                                                                                             |
| admin (backend) | `Server/admin/`    | NestJS (JS)                   | Admin-only backend — separate from the Go services; calls them through the gateway's REST API (`GATEWAY_URL`). Lives under `Server/`, not `Client/`, because it's a real backend — see `docs/plan.md`. |
| mobile          | `Client/mobile/`   | Flutter                       | Mobile app, talks to the gateway directly.                                                                                                                                      |
| desktop         | `Client/desktop/`  | JavaFX                        | Native desktop app, talks to the gateway directly. Not containerized — see [`infra/README.md`](../infra/README.md).                                                           |

All JS/TS apps are plain JavaScript (no TypeScript), each with an independent
`package.json` — no shared workspace/monorepo tool.

## Server/app: microservices

Split from a modular monolith on 2026-09-25. Each service owns its data
outright — no service can reach another's database:

| Service | Database | Publishes on | Talks to (sync) | Owns |
| --- | --- | --- | --- | --- |
| **gateway** | — | — | every service (gRPC + REST proxy) | the edge: REST `/api/v1` façade, `/graphql`, versioning, CORS, rate limits |
| auth | MySQL | RabbitMQ | — | accounts' credentials, refresh tokens |
| catalog | Postgres (+ Redis cache, SeaweedFS files) | RabbitMQ | — | authors, books, search, e-library files, copy reservations |
| borrow | Postgres | Kafka | catalog (gRPC) | borrow records + the borrow Saga |
| reading | MongoDB | NATS JetStream | catalog (gRPC) | reading sessions, offline sync, bookmarks, history |
| user | Postgres | RabbitMQ | catalog (gRPC) | profiles, library shelf, admin user management |
| notification | none | — | — | email (consumes RabbitMQ + Kafka) |
| payment | Postgres | Kafka | catalog (gRPC), Stripe, Paystack | book purchases |

- **Sync**: gRPC between services (contracts in `Server/app/proto/`),
  only for write-time correctness checks (does this book exist? reserve a
  copy). Display aggregation happens in the gateway, never inside a
  service.
- **Async**: domain events via a transactional outbox in every service →
  its broker → idempotent consumers (e.g. `auth.user_registered` creates the
  user-service profile; `reading.book_completed` updates reading counters;
  `borrow.borrow_overdue` triggers an email).
- **Identity**: every service verifies the JWT itself; the gateway and gRPC
  forward the caller's token.
- **Resilience**: circuit breakers and bulkheads on every gRPC client and
  gateway upstream (fail fast with 503 + `Retry-After`); catalog reads fall
  back to a stale cached copy; the gateway makes every mutating route
  retry-safe with `Idempotency-Key` (Redis) and rate-limits per IP across
  replicas.
- **Consistency across services**: events keep user-service's account copy
  in sync with auth's; an hourly reconciler repairs any drift. Clients see
  UUIDv7 `public_id`s, not sequential database IDs.

Full reasoning, every decision, and what's still open:
[`Server/app/docs/plan.md`](../Server/app/docs/plan.md) → "Microservices
split — Server-side implications".

## Why a separate admin backend instead of extending the Go services

The admin dashboard has its own NestJS backend (`Server/admin/`) rather
than admin endpoints bolted onto the gateway. Before the split it shared
the Go app's MySQL database, with an unenforced "read-mostly" convention;
the split removed that problem — there's no shared application database
any more, so the admin backend calls the services through the gateway's
REST API like any other client (it gets `GATEWAY_URL`). Admin-only routes
the services already expose (`GET /users`, `PATCH /users/{id}/status`,
`GET /borrows`, catalog writes) are role-guarded in the owning service.

## Data stores

- **One database per service**: MySQL (`bibliomania_auth`), Postgres
  (`bibliomania_catalog`, `_borrow`, `_user`, `_payment` — one server, one
  role each), MongoDB (`bibliomania_reading`, single-node replica set).
  Each service runs its own embedded migrations on boot. On Kubernetes,
  `components/ha` replicates all of them (CloudNativePG, a MySQL replica, a
  3-member Mongo replica set); sharding is designed but deliberately not
  built (`Server/app/docs/plan.md` → "Hardening pass").
- **Redis** — catalog's cache, the gateway's idempotency records and rate
  limits, notification's sent-log. Never a source of truth.
- **SeaweedFS** — e-library files (self-hosted object storage speaking the
  S3 protocol; no cloud account). Backups: `infra/backup/`.
- **Brokers** — RabbitMQ, Kafka, NATS JetStream (see the table above).
- **Umami's Postgres** (`umami-postgres`) — belongs entirely to Umami
  analytics, separate from the application's `postgres`.
- **Observability** — Prometheus (metrics + alert rules → Alertmanager),
  Loki (logs), Tempo (traces), Grafana on top.

## Network / request flow (dev)

```
                          ┌──────────┐
   browser ─────────────▶ │  nginx   │  (subdomain routing, see infra/README.md)
                          └────┬─────┘
            ┌──────────────────┼───────────────┬─────────────┬──────────────┐
            ▼                  ▼               ▼             ▼              ▼
        web-main           web-app         admin-web       admin     (api.*) gateway
     (static export)    (Next.js SSR)   (Next.js SSR)    (NestJS)   REST /api/v1 + /graphql
                               │                             │              │
                               └──── REST ───────────────────┴──────▶ gateway
                                                                            │  REST proxy /
                                                                            │  gRPC fan-out
     ┌──────────────┬──────────────┬──────────────┬──────────────┬──────────┴───┐
     ▼              ▼              ▼              ▼              ▼              ▼
   auth          catalog ◀──gRPC── borrow       reading        user         payment
  (MySQL)     (Postgres+Redis)   (Postgres)    (MongoDB)    (Postgres)     (Postgres)
     │              │              │              │              │              │
     └─ RabbitMQ ───┴──────────────┼── Kafka ─────┼── NATS ──────┘ (+RabbitMQ)──┘ Kafka
                                   ▼              ▼
                             notification      user (counters)
                               (email)

   mobile / desktop ─────────────────────────────────────────────────▶ gateway
   (bypass nginx — native apps, not browser clients)
```

`mobile` and `desktop` are native apps, not browser clients — they call the
gateway directly rather than going through nginx's subdomain routing
(which exists for browser-based hostname resolution). No service other than
the gateway is reachable from outside the Compose network.

## Deployment topology

Dev and prod are the same service graph, different Docker Compose overrides
(`docker-compose.dev.yml` vs `docker-compose.prod.yml` on top of the shared
`docker-compose.yml` base) — see `infra/README.md` for the full breakdown of
what differs (air hot reload + bind mounts vs built images, dev-only
Adminer/Mailpit, host port publishing). The same service graph also runs on
**Kubernetes** (`infra/k8s`: Kustomize base + local/prod overlays), locally
on a kind cluster that Terraform (`infra/terraform/local`) creates, with
Traefik as the ingress controller. The `local-ha` overlay adds replication
for every stateful piece and a Linkerd mesh (mTLS between services).
Production is set up for GitOps — Argo CD syncs `overlays/prod` from `main`,
CD pins image tags, secrets are committed as SealedSecrets — but there's no
production host or cloud cluster yet; `infra/k8s/README.md` → "Production"
lists what one needs.

## What's built vs. not

- **Built**: `Server/app` (all 20 roadmap steps, now as seven
  microservices + a gateway, with full observability), `Client/web/app` (all 7
  steps of its own roadmap — auth, catalog, borrowing, personal library,
  epub.js reader; see `Client/docs/web-app-Steps.md`), `Client/web/main`
  (marketing site).
- **Still bare scaffolds**: `Client/admin` + `Server/admin`,
  `Client/mobile`, `Client/desktop`. UI mock-ups exist for all of them
  (links in `docs/TODO.md` → "Product code").

See `docs/TODO.md` for the current punch list.

## Planned: platform-vision repositioning

A separate, larger change is planned but mostly not started: multi-branch
library tenancy, payments/billing (subscriptions + one-time purchases +
library licensing — one-time purchases exist as payment-service's first
slice), curation, and several AI-assisted features (moderation,
translation, audiobooks, a reading companion). This will add code to the
owning services (or new services) and reshape parts of the data model
described above (e.g. physical
book copies moving from a global pool to per-branch ownership). See
[`docs/plan.md`](plan.md) → "Platform vision" for the full context and
[`Server/app/docs/plan.md`](../Server/app/docs/plan.md) for the technical plan.

## Microservices split — done

Built 2026-09-25, hardened 2026-09-26 (concurrency fixes, idempotency,
resilience, HA, mesh, CI security scans, alerting, GitOps — see
`docs/CHANGELOG.md`). Remaining follow-ups — `web/app` onto GraphQL,
migrating old monolith data, broker/database auth, a real cloud cluster —
are in `docs/TODO.md`.
