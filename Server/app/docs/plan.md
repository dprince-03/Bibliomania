# Server plan: platform vision → Server implications (Steps 21+)

`Server/app/docs/Steps.md` covers the original roadmap — Steps 1-20, all complete.
This file is its technical companion for what comes next: the Server-side
consequences of the platform-vision decisions recorded in the root
[`docs/plan.md`](../../../docs/plan.md) → "Platform vision: authors, libraries,
readers". Read that section first for the *why*; this file is scoped to the
*what* — which Server modules and schema changes each decision implies.

**Nothing here is implemented yet.** Section numbers below match
`Server/app/docs/Steps.md`'s Step 21-45 entries exactly (same order, same
scope) — that file has the terser step-by-step summary; this one has the
fuller reasoning behind each. Each section becomes its own
`feat/step-<N>-<slug>` branch later, one at a time, per the existing
convention in `.claude/CLAUDE.md` / `docs/CONTRIBUTING.md`.

**Separately**, `Server/app` has been split from a modular monolith into
microservices (decided 2026-09-24/25, **built 2026-09-25** on
`refactor/server-microservices`, not a numbered Step) — see "Microservices
split — Server-side implications" near the bottom of this file. That
changes *where* several Steps below land: Step 27's first slice now exists
as `payment-service`, Step 37 is largely done, and every Step's "new
module" is now "new code in the owning service" (or a new service). Read it
before starting any of them.

## Step 21 — New `library` module

- `Library` and `Branch` models — a `Library` has many `Branch`es (address,
  delivery radius, its own physical inventory), matching the confirmed
  multi-tenant, multi-branch decision (a county/city system is one `Library`
  with several `Branch`es, not one account per building).
- Repository/service/handler skeleton, following the existing feature-package
  shape (`internal/modules/library/`).
- Foundation step — Steps 22 onward (RBAC scoping, per-branch copies,
  verification) all depend on this landing first.

## Step 22 — Scope `librarian` RBAC to one library

- `librarian` is currently a flat global role
  (`internal/middleware/rbac.go`) — needs a `library_id` scope so a librarian
  account can only act on their own library's branches/inventory/holds.
- Touches `AuthGuard`/`RoleRequired`'s context plumbing, not just the role
  list — the middleware needs to know *which* library a request's librarian
  belongs to, not just that they hold the role.

## Step 23 — Redesign `borrow`: Hold + Fulfillment

- `BorrowRecord` schema changes: expand `Status` beyond the current
  `active`/`returned` shape to `reserved` → `ready_for_pickup` /
  `out_for_delivery` → `active` → `returned`; add a `FulfillmentMethod`
  (`pickup` / `locker` / `delivery` / `mail`).
- Physical copies move from `catalog.Book`'s global `TotalCopies`/
  `AvailableCopies` to per-`Branch` ownership — a copy belongs to a branch,
  not a platform-wide pool.
- Delivery eligibility is a distance check between the reader's address and
  the branch's service radius; readers outside it fall back to `mail`
  fulfillment rather than being blocked.
- The existing atomic `WHERE available_copies > 0` reservation pattern
  (`Server/app/internal/modules/borrow`, see `.claude/CLAUDE.md`) still applies — just
  scoped to a branch's copy count instead of a global one.

## Step 24 — Library signup, verification, admin approval

- Collect legal institution name, branch address(es), an official
  registration/license number, a supporting document upload, and an
  official-domain admin email for the primary librarian account (not a free
  consumer address).
- Gate live status behind manual admin approval before the library can
  receive real holds or payments.
- Financial-grade verification (tax ID, bank account) is largely handled for
  free by whichever payment processor is chosen in Step 27, since onboarding
  a payee already requires it — don't duplicate that compliance work here.

## Step 25 — Curation: "Library Selected" badging

- A join table inside `catalog` — `library_id`, `book_id`, `badged_by`,
  `badged_at` — letting a librarian badge an indie book without the library
  ever owning or hosting it. Digital catalog entries stay global and
  library-agnostic; curation is metadata on top, not a copy/ownership record.
- Directly answers libraries' real stated obstacle to indie acquisition
  (judging quality without professional reviews).

## Step 26 — Category model: adopt BISAC Subject Headings

- `catalog.Book.Genre` (currently a single string field) gets superseded by
  a many-to-many relation to BISAC Subject Heading codes — a book can carry
  more than one (industry norm: up to ~3). Needs a `BookCategory` join table
  and, likely, a small seeded/reference table of valid BISAC codes rather
  than free-text genre strings.

## Step 27 — Payments/billing module

- New `internal/modules/billing/` package.
- `LibraryLicense` — the flat $2,000/year platform license (platform-to-
  library direction, not a split).
- `ReaderSubscription` — recurring monthly charge, tiered (Regular/Scholar/
  Premium Scholar, see Step 28), 85/15 split to library/platform.
- `BookPurchase` — one-time charge, 85/15 split to author/platform, and
  **must be a permanent entitlement record** independent of subscription
  status — "buy once, read for life" cannot be revoked by a lapsed library
  subscription, so this table can't be keyed through `ReaderSubscription`.
- Payout/ledger tracking for authors and libraries.
- **Explicitly dependent on an external payment-processor SDK decision not
  yet made** (Stripe Connect or a regional equivalent — see root
  `docs/plan.md`'s open questions). Don't build a hand-rolled ledger that
  duplicates what a marketplace-payments processor already does.
- File delivery for a purchased book reuses the existing e-library download
  flow (`catalog.BookService.GetDownloadPath`, Step 14) — gate it on "has a
  `BookPurchase` record or the book is free," not a new delivery mechanism.
- **Launch gate**: must not accept real payments in production until Step
  38's third-party penetration test passes (may still be built and tested
  in dev before then).

## Step 28 — Reader subscription tiers

- Regular / Scholar / Premium Scholar, built on `billing.ReaderSubscription`
  from Step 27.
- Concrete feature gates not yet finalized — directional ideas in root
  `docs/plan.md` (borrow limits, priority holds, cross-library access,
  discounted delivery, early curated-title access).

## Step 29 — Content-moderation pipeline

- A content-score + AI-disclosure field on `Book`, populated by a call to an
  external plagiarism/AI-detection API at upload time.
- Score feeds the Step 25 curation queue (flagged-but-undisclosed content
  gets priority librarian review) and discovery ranking — never an automatic
  reject; human judgment stays final, matching how real publisher pipelines
  use these tools.

## Step 30 — Author analytics dashboard

- Aggregation endpoint(s) over existing `reading_sessions` data (completion
  rate, drop-off page, average reading time) — no new tracking needed, the
  raw signal (`current_page`, timestamps) already exists.

## Step 31 — Translation marketplace + Audiobook Studio

- `Translation` — a `Book` has many translated editions, each with its own
  file, language, translation tier (`ai` / `ai_plus_human_edit` / `human`),
  and status; revenue-split terms vary by tier per root `docs/plan.md`'s
  monetization model.
- `AudiobookAsset` — a `Book` has many audio editions (one per voice/
  language), each an AI-narration job result, stored/delivered like the
  existing e-library file.

## Step 32 — Reader-side Read Aloud + accessibility

- On-demand, real-time TTS playback for whatever the reader is currently
  viewing, on any book they have legitimate access to — distinct from Step
  31's produced, sellable Audiobook Studio editions.
- Dyslexia-friendly font toggle and adjustable size/line-spacing in the
  reading UI.
- Mostly a Client-side feature; Server's role is the same thin
  authorization check described in Step 33.

## Step 33 — AI reading companion

- Spoiler-safe chat scoped strictly to the text of a book the reader already
  has legitimate access to, up to their current page — no outside knowledge,
  no hallucinated plot.
- Server exposes a thin authorization check (purchase/subscription/free) —
  the chat logic itself is likely a thin proxy to an LLM API, not new domain
  logic. Deliberately not a writing tool.

## Step 34 — Social/engagement layer

- New `internal/modules/social/` package.
- `Follow` (reader → author), `Comment`, `Rating` — the engagement layer
  currently entirely absent. `reading.SessionRepository` already tracks who's
  reading what; a `Follow` table sits naturally alongside it for
  new-release notifications.

## Step 35 — Data collection & privacy policy

- Foundational, not code-first: define what's collected, why, and the
  GDPR-aligned stance (data minimization, no ad-tracking) that Steps 36
  onward and the billing module (Step 27) need to already respect, rather
  than retrofit later.

## Step 36 — Encryption baseline + MFA

- AES-256 for data at rest, TLS 1.3 in transit, RSA-2048/ECC for key
  exchange — applied across existing sensitive data (addresses from Steps
  21/24, payment records from Step 27), not just new tables going forward.
- MFA required on library-admin and author-payout accounts — the single
  most common 2026 compliance-audit failure point industry-wide, cheap to
  require from the start and expensive to retrofit.

## Step 37 — Observability: OpenTelemetry + Prometheus + Grafana

- Self-hosted (decided over a managed service like Datadog/Grafana Cloud,
  given the project is pre-revenue).
- Threads `trace_id`/`span_id` alongside the existing `request_id`
  (`middleware/requestID.go`, Step 8) rather than replacing it — traces and
  metrics export to Prometheus, visualized in Grafana; existing structured
  logging (`log/slog`-based) stays as-is.

## Step 38 — Third-party penetration test

- A formal, paid, external penetration test — decided as a **hard launch
  blocker on Step 27** (payments), not a strict build-order dependency:
  Step 27 can be built and tested in dev before this, but must not accept
  real payments in production until this passes.

## Step 39 — Encrypted local storage (mobile + desktop)

- Readium LCP-style model: the on-device offline cache is encrypted at
  rest, keyed to device+account, decrypted only in-memory inside the
  reading app itself.
- Explicitly **not** a re-introduction of hard DRM on the entitlement
  itself — Step 27's `BookPurchase` stays a permanent, portable record
  regardless. This is offline-cache hygiene on mobile/desktop specifically
  (no local file to protect in a browser-based web reader), covering two
  distinct cases: readers' purchased/borrowed books, and authors'
  unpublished drafts/manuscripts pre-release.

## Step 40 — Note-taking (highlights + notes)

- New `Note`/`Highlight` model alongside `reading.Bookmark` — synced across
  devices the same way reading progress already is via the existing sync
  infrastructure (Steps 14/15).

## Step 41 — Virtual book club

- Extends `internal/modules/social/` (Step 34) rather than a new module:
  scheduling + reminders, live video/chat (an external video-call
  integration, following the same "don't hand-roll it" logic as Step 27's
  payment processor), polls, and AI-generated discussion questions reusing
  Step 33's reading-companion infrastructure instead of a separate build.
- Differentiator over existing dedicated book-club apps: since authors are
  already first-class accounts on this platform, any author can join a
  discussion of their own book natively — no external outreach needed,
  unlike platforms where "author joins the call" is a special, hard-won
  feature.

## Step 42 — Reading goals dashboard

- New field(s) on the reader's profile: an annual goal (Goodreads Reading
  Challenge-style — familiar, proven) plus a daily streak counter (the gap
  Goodreads itself doesn't cover, which newer habit-focused apps win on).
- Computed off existing `reading_sessions` timestamps — no new tracking
  needed, same "the data already exists" pattern as Step 30's author
  analytics dashboard.

## Step 43 — Data export & account deletion

- GDPR portability/erasure: one flow to export a reader's notes,
  highlights, and reading history; a real account-deletion path. Cheap to
  build now, expensive to retrofit once an audit flags its absence.

## Step 44 — Public status page

- Externally-visible uptime page built off the existing `/health` signal
  (Step 20, `internal/health`) rather than a new liveness mechanism —
  libraries paying Step 27's $2,000/year license will reasonably expect
  visible uptime, not just an internal check.

## Step 45 — Backup & disaster-recovery policy

- A documented and actually-tested database backup/restore procedure. More
  an operational runbook than new application code, but load-bearing once
  real institutional and financial data lives in this database, not just
  demo data.

## Microservices split — Server-side implications

**Status: implemented 2026-09-25** (branch `refactor/server-microservices`),
all seven services + gateway at once, carved out of `Server/app` in place
(the monolith no longer exists). Verified end to end on the full Compose
stack — see "Implementation" at the end of this section. The *why* and the
full decision trail live in root [`docs/plan.md`](../../../docs/plan.md)'s
"`Server/app` microservices split" sections; the contracts now live in
`proto/` (gRPC) and `internal/gateway/graph/schema.graphqls` (GraphQL). The
subsections below were written before the build and are kept as the plan
of record; "Implementation" records what was actually built and every
decision made along the way.
Driven by wanting hands-on distributed-systems experience, not by a
measured production need.

### Service map

The five existing `internal/modules/` packages each become their own
service with their own database, plus two new ones:

| Service | From today's | Database | Event broker |
| --- | --- | --- | --- |
| `auth-service` | `modules/auth` | MySQL | RabbitMQ |
| `catalog-service` | `modules/catalog` | Postgres | RabbitMQ |
| `borrow-service` | `modules/borrow` | Postgres | Kafka |
| `reading-service` | `modules/reading` | MongoDB | NATS |
| `user-service` | `modules/user` | Postgres | RabbitMQ |
| `notification-service` | new (consumes `RESEND_API_KEY`, currently unused) | none | consumes RabbitMQ + Kafka |
| `payment-service` | new (Step 27's billing module) | Postgres | Kafka |

Redis stays a cache (plus ephemeral broadcasts), never a system of record.
All five services split at once, not incrementally. Suggested sequencing:
get "a user borrows a book" (`auth` → `catalog` → `borrow` → `user`)
working end-to-end first.

### Communication

- **gRPC** between services only; **GraphQL gateway** in front for
  `web/app` (display aggregation/fan-out); **REST** stays for
  `Client/admin`, health checks, and webhooks.
- **`POST /borrow`** becomes a synchronous orchestrated Saga:
  `borrow-service` calls `catalog-service`'s `ReserveCopy` (gRPC), creates
  its record, and calls `ReleaseCopy` to compensate if that fails. This
  replaces today's single-DB `DecrementAvailableCopies` +
  insert. `BookBorrowed` is published async afterwards.
- **`auth` keeps its own copy** of email/password hash/role, synced from
  `user` via events, so login never depends on `user-service` being up.

### Code dependencies that change

Today's in-process cross-module calls, and what replaces each:

- `borrow` → `catalog.BookRepository` (existence check, copy decrement,
  title lookup) → gRPC `ReserveCopy`/`ReleaseCopy` at write time; title
  **snapshotted onto the borrow row at creation**. Note this is a behavior
  change: today `borrow.Service.enrichBorrows`
  (`internal/modules/borrow/service.go`) looks the title up live, per
  record, on every read. `BorrowRecord` has no title column yet.
- `reading` → `catalog` (book-existence check) → validate once at
  session/bookmark creation only, never on progress updates; title
  snapshotted at creation.
- `user` → `reading.SessionRepository` (powers `GET /users/me/history`) →
  **removed**; the GraphQL gateway fans out to `reading-service` directly.
- `user` → `catalog` (for `UserLibrary.Book`) → validated at add-time;
  display resolved live by the gateway (a wishlist should show the current
  title, unlike a borrow's historical one).
- `user_profiles.total_books_read`/`total_pages_read` → owned by
  `reading-service`; `user-service` keeps an eventually-consistent cached
  copy updated from a book-completed event.
- `user_id` foreign keys everywhere → no cross-service check at all; a
  valid JWT already proves the user exists.

Net result: `borrow` and `reading` each depend on `catalog` only;
`user-service` depends on neither `catalog` nor `reading`.

### Impact on Steps 21-45 above

- **Step 27** (payments/billing) → becomes `payment-service` (Postgres +
  Kafka, Stripe Connect), not an `internal/modules/billing/` package.
- **Step 37** (observability) → expands to the full Grafana stack
  (Prometheus + Loki + Tempo + Grafana), OpenTelemetry in every service,
  plus per-DB and per-broker Prometheus exporters. This also answers the
  split's own "how is a request traced across services" question.
- Email notifications (overdue, hold-ready, welcome), implied by several
  Steps, belong to `notification-service`.
- **Platform layers** (not Steps yet): Terraform (Docker provider locally
  first, then DigitalOcean/Hetzner), Kubernetes via `kind`/`k3d` locally
  before a managed cluster.

### Still open (as planned, before the build)

Resolved by the build — kept for the record, see "Implementation" below:
build order (split first, then Steps 21-45), idempotency, service
discovery, the GraphQL library, and the remaining contracts.

### Implementation (2026-09-25)

**Layout.** One Go module, eight binaries (`cmd/<service>`), each service's
code in `internal/services/<service>/` with its own embedded migrations;
shared plumbing in `internal/{events,grpcx,telemetry,server,database,
apiversion}`. One module rather than eight keeps shared code (errors,
middleware, event contracts) in one place; the services are still
separately built, deployed and scaled (one image each, `bibliomania-<svc>`).
Full map: [`project_setup.md`](project_setup.md).

**Decisions made while building** (none of these were settled in the plan):

- **Build order**: split first, Steps 21-45 after, in the new services.
- **Consistency: transactional outbox everywhere.** Every service that
  publishes writes its event into an `outbox_events` table/collection *in
  the same local transaction* as the business row; a relay publishes it
  afterwards. No "row committed, event lost" window. MongoDB runs as a
  single-node replica set purely so reading-service gets multi-document
  transactions for this.
- **Consumers are idempotent**: SQL consumers record each event ID in
  `processed_events` in the same transaction as their write; every
  broker delivers at-least-once. Failed messages retry 3× in-process, then
  dead-letter (`<queue>.dlq` / `<topic>.dlq` / JetStream `Term`), counted in
  `events_handled_total{outcome="dead_letter"}`.
- **Idempotency**: `POST /borrows` takes an `Idempotency-Key` header
  (unique per user in `borrow_records`); catalog's `ReserveCopy`/
  `ReleaseCopy` are idempotent per reservation key (`copy_reservations`).
- **Borrow Saga compensation**: a failed borrow releases its copy inline;
  if catalog is unreachable the release is queued in `saga_compensations`
  and retried with backoff. `ReserveCopy` timing out (unknown outcome) also
  triggers a release — safe because release is idempotent. Returns release
  *first*, then mark the record returned, so a half-done return can simply
  be retried. Unit-tested in `internal/services/borrow/service_test.go`.
- **Identity**: auth-service creates accounts and owns credentials;
  user-service gets a copy via `auth.user_registered`. `is_active` is the
  one identity field written in user-service (admin action) and flows back
  via `user.status_changed`, which also revokes refresh tokens. Every
  service verifies the JWT itself (shared `JWT_SECRET`); the gateway
  forwards the bearer token, and gRPC carries it as metadata — no service
  trusts a bare user ID from another.
- **user-service does call catalog-service** (`GetBook`) when a book is
  added to a shelf — a write-time correctness check, which rule 1 of the
  data-ownership decision allows. The plan's "user-service has zero
  dependencies on catalog" was about *display*; that part holds.
- **Service discovery**: Compose DNS (`catalog:9090`). gRPC clients connect
  lazily and retry `UNAVAILABLE`, so start order doesn't matter.
- **GraphQL**: gqlgen (Go), schema-first, a single gateway schema — no
  federation. The library field batches catalog lookups
  (`GetBooksByIds`); other nested fields resolve per item.
- **REST kept, via the gateway**: every existing `/api/v1` path still works
  unchanged — the gateway reverse-proxies to the owning service, and
  aggregates the one route that spans two (`GET /users/me/library`).
  `web/app` needed only its internal base URL changed.
- **API versioning**: URL-path for REST (`internal/apiversion`), proto
  package for gRPC, `schema_version` per event type, `@deprecated` for
  GraphQL — see [`API.md`](API.md) → "Versioning".
- **Databases**: the four Postgres databases share one server with one
  role each; MySQL now holds only auth's database (a fresh
  `auth_mysql_data` volume — the old one's migration version table would
  have clashed).
- **Payments**: payment-service ships a first slice of Step 27 — one-time
  book purchase via Stripe Checkout (Paystack added in the follow-up),
  signed webhooks, purchase ledger, events. Books got optional
  `price_cents`/`currency`. Refuses live keys unless
  `PAYMENTS_LIVE_ENABLED=true` (the Step 38 launch gate).
- **Observability** (most of Step 37): OpenTelemetry traces to Tempo,
  JSON logs with `trace_id` shipped by Alloy to Loki, Prometheus metrics
  from every service plus exporters for MySQL, Postgres, MongoDB, Redis,
  Kafka, NATS and RabbitMQ's built-in endpoint, and a provisioned Grafana
  "Bibliomania — Services" dashboard. Grafana links logs ↔ traces.

**Bugs fixed on the way** (all pre-existing in the monolith): book updates
never persisted the recalculated `available_copies`; list/search cache
invalidation only cleared three hard-coded page keys; CORS didn't allow
`PATCH` (which the app uses) and sent a date string as
`Access-Control-Max-Age`; `total_books_read`/`total_pages_read` were never
incremented; `redisclient` used an invalid `%m` format verb; the Makefile's
`docker-up`/`docker-down` pointed at a root `.env` that no longer exists.

**Verification.** `go build`/`go vet`/`go test` clean; the full stack
(35 containers) brought up with Compose and exercised by
`infra/docker/smoke-test.sh` — 41 checks across every cross-service path
(Saga + idempotent replay + release-first return, gRPC fan-out, all
three brokers, eventual-consistency reads, GraphQL, versioning,
deactivation propagation), all passing; Prometheus scraping all 18
targets; a borrow request traced across gateway → borrow → catalog in
Tempo; log lines in Loki carrying the same `trace_id`.

### Follow-up (2026-09-25, later the same day)

Requested by the user after the build:

- **Database connectors moved to `pkg/`** (`postgresclient`,
  `mysqlclient`, `mongoclient`, alongside `redisclient`), with shared retry
  and pool logic in `pkg/sqlretry`. `internal/database` now only runs
  migrations.
- **Database init as `.sql`** in `infra/docker/{postgres,mysql}/init/`.
  Postgres roles and databases no longer come from a shell script;
  passwords are read with psql `\getenv`, so no secret is written in SQL.
  MySQL got persisted server settings. Tables still come only from each
  service's migrations.
- **Paystack alongside Stripe**, chosen by currency (see
  [`API.md`](API.md) → "Payments"). A `Provider` interface
  (`internal/services/payment/provider.go`) covers both. Paystack webhooks
  are HMAC-SHA512 verified and then re-checked with Paystack's Verify API.
  Any paid amount or currency that doesn't match the purchase is refused.
  Schema changes went in a new migration (000003: provider-neutral
  columns, `webhook_events`). Tested with unit tests plus an end-to-end run
  against a fake Paystack API on the Compose network.
- **Kubernetes** (`infra/k8s`, Kustomize base + local/prod overlays) on
  **kind**, with **Terraform** (`infra/terraform/local`) creating the
  cluster and **Traefik** as the ingress controller. The user asked which
  option was best; the reasoning is in `infra/k8s/README.md` → "Why these
  tools". Side effects in the code: gRPC clients use round-robin load
  balancing (headless `*-grpc` Services), the prod image runs as a numeric
  UID (10001) so `runAsNonRoot` can verify it, and the Loki per-container
  label is now `app` in both environments.

### Hardening pass (2026-09-26)

Answers to the user's review questions, built one after another. The full
list of changes and bugs is in `docs/CHANGELOG.md` → 2026-09-26. These are
the decisions and why:

- **Concurrency: let the database arbitrate.** Every race fixed here is a
  single conditional SQL statement, not an application-level lock:
  - refresh rotation: `UPDATE … WHERE revoked = FALSE`;
  - copy counts: a relative delta, guarded by `>= 0`;
  - pending checkouts: a unique partial index.
  Locks in Go wouldn't hold across replicas; these do. Each fix has a
  Testcontainers test that fires 15–40 concurrent requests at a real
  database.
- **Idempotency at the edge.** One gateway middleware covers every
  mutating route, storing the full response in Redis for 24h and keyed by
  caller + route + key. The service-level guarantees stay underneath it
  (borrow's `Idempotency-Key`, catalog's per-key reservations, deduped
  consumers). It fails open if Redis is down: a missing replay is better
  than refusing writes. Multipart uploads are skipped (the body can be
  hundreds of MB).
- **Fallback when a dependency is down.**
  - Circuit breakers trip on infrastructure errors only (Unavailable,
    DeadlineExceeded, …), never on 4xx/NotFound.
  - Reads degrade: stale cache, a partial GraphQL result.
  - Writes fail fast with 503 + `Retry-After`. A write is never faked.
- **Storage.** Files moved to object storage so catalog is stateless.
  SeaweedFS was picked because MinIO stopped shipping community images.
  "S3" is only the protocol — no cloud account is involved, and any
  S3-protocol store can replace it.
- **Replication.**
  - Every stateful piece gets its standard HA form: CloudNativePG for
    Postgres (automatic failover), a GTID replica for MySQL (hot standby,
    manual promotion), a replica set for Mongo, KRaft quorum for Kafka,
    a JetStream cluster for NATS, and quorum queues for RabbitMQ.
  - Measured: Postgres failover in 17s, after lowering CNPG's
    `smartShutdownTimeout`; the broker quorums lose one node without the
    smoke test noticing.
  - MySQL has no automatic failover: auth's writes are small and rare, and
    an operator (Percona) is the next step if that changes.
- **Sharding: designed, deliberately not built.** Nothing here comes close
  to one node's limits (the largest table is borrow records). Sharding now
  would add cross-shard queries, resharding and operational weight for no
  gain. Replication (above) covers availability and read scale. When a
  table does outgrow a node:
  - **Key**: `user_id` (hashed) for the per-user data — borrows, reading
    sessions, bookmarks, library, purchases. Nearly every query is "this
    user's …", so it stays on one shard.
  - **Catalog isn't sharded**: it's small and read-heavy. It scales with
    read replicas plus the Redis cache.
  - **How**: Postgres via Citus (distribute by `user_id`; the per-service
    databases make that one decision per service), Mongo via native
    sharding on `{user_id: "hashed"}`.
  - **Before then**: the public UUIDv7 IDs are what make it possible —
    clients never see the sequential per-database IDs, which wouldn't be
    unique across shards. Admin-wide lists ("all borrows") would become
    scatter-gather queries or move to an analytics store.
- **IDs.** Internal keys stay sequential `BIGINT` (index locality, small
  foreign keys). Clients get UUIDv7 (`public_id`): not guessable, and time-
  ordered, so the index stays healthy. The REST handlers accept both forms
  during the transition.
- **Reconciling IDs across services.** user-service copies auth's accounts
  from events, which are reliable through the outbox, but a restore or a
  manual edit can still make them drift. The reconciler compares both
  sides and repairs each field from its owner. Orphans are flagged, never
  deleted: a user with no auth account needs a human to decide.
- **Load balancing.** gRPC keeps one long-lived HTTP/2 connection per
  client, so a normal Kubernetes Service (per-connection balancing) would
  pin each caller to one pod. The clients already avoid that with
  client-side round-robin over headless `<svc>-grpc` Services, but DNS
  only reveals new pods when a client re-resolves, so a freshly scaled-up
  replica waits for traffic. Linkerd adds per-request, latency-aware
  (EWMA) balancing with live endpoint updates, plus mTLS. Its scope is
  service-to-service only (databases and brokers are skipped — no mTLS to
  gain, and server-speaks-first protocols stall behind protocol
  detection). HTTP balancing at the edge stays Traefik's job.
- **Mesh enforcement** is per port: gRPC (9090) accepts only meshed
  identities. HTTP ports stay open to Traefik, Prometheus and kubelet
  probes, none of which are in the mesh.
- **Secrets.** Sealed Secrets over SOPS: an encrypted `SealedSecret` is a
  plain manifest, so Argo CD applies it with no plugin, and only the
  target cluster can decrypt it.
- **GitOps.** Argo CD watches `overlays/prod` on `main`. CD pins image tags
  with a one-line edit per service — `kustomize edit set image` would
  rewrite and reflow the whole file. A merge is a deploy; a revert is a
  rollback.
- **Toolchain.** go.mod keeps `go 1.25.5` as the minimum *language*
  version; images build with Go 1.26.x. The standard library is compiled
  into the binaries, so its security fixes come from the builder image,
  and Go 1.25 no longer gets them.

### Still open (after the build)

- **Existing data isn't migrated.** The monolith's `bibliomania` MySQL
  database is left untouched but unused; a dev stack starts empty (`make
  seed`). A one-off migration tool is possible if any real data matters.
- **`web/app` still uses REST**, not GraphQL. Moving its data layer to
  `/graphql` (where the gateway's fan-out actually pays off) is Client-side
  work, not started.
- **mTLS between services: done on Kubernetes** (Linkerd,
  `components/mesh`). Compose has no mesh. MongoDB, Kafka and NATS still
  run without auth — fine while only the gateway is reachable from
  outside, required before a real deployment.
- ~~notification-service can send a duplicate email~~ — fixed 2026-09-26
  with a Redis sent-log. A duplicate can still slip through only if Redis
  is down, since sending then fails open.
- **Kafka consumer-group rebalance after a restart** delays delivery by up
  to the session timeout (~45s). Late, not lost.
- Payment scope beyond one-time purchases (licences, subscriptions,
  Stripe Connect splits, refunds) stays Step 27.
- Terraform + Kubernetes: a local kind cluster is built by Terraform
  (see "Follow-up"). A real cloud provider and cluster isn't set up yet;
  `infra/k8s/overlays/prod` lists what that needs.
- Step 37 leftover: importing the community dashboards for each exporter
  (Grafana.com IDs: MySQL 7362, Postgres 9628, MongoDB 2583, Redis 763,
  Kafka 7589, RabbitMQ 10991, NATS 2279). Alerting is done (2026-09-26).
- MySQL (auth) has no automatic failover (hot standby only; see "Hardening
  pass").
- Argo CD has only passed a server-side dry run. A real sync needs this
  branch merged to `main`, plus a production cluster.

## Verification

Each step above gets designed and verified individually as its own roadmap
step when it's actually built — this file states intent and shape, not a
tested implementation. Update `Server/app/docs/Steps.md`'s matching entry (⏳ → ✅
with real verification notes, mirroring how Steps 1-20 are documented) as
each step actually ships, and trim or update this file's corresponding
section rather than leaving it describing something already shipped.
