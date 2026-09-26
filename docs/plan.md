# Project Plans

A running log of major initiative plans for Bibliomania, kept for reference —
each as agreed on and, where implementation has happened, as actually
implemented. Newest/most relevant entries are added as new `##` sections;
older ones stay put as historical record rather than getting overwritten.

## Infra branch plan (`feat/infra-docker-stack`)

This is the plan agreed on for the infra branch, kept for reference. It was
approved before implementation started, then adjusted twice mid-implementation
per follow-up instructions — see "Deviations from the original plan" at the
bottom for what actually changed and why.

### Context

Bibliomania is expanding from "Go API + one React scaffold" into a real monorepo: a product web app, a marketing site, an admin dashboard (its own frontend + backend), and a mobile app, all fronted by nginx and backed by MySQL/Redis/Umami. The user had already carved out the target directory skeleton (`Client/web/{app,main}`, `Client/admin`, `Client/mobile`, `infra/nginx`) but left them empty except for a leftover Vite scaffold in `Client/web/main`. The existing `infra/docker/` compose setup was written back when the Go server was the only service and had since bit-rotted (paths assumed `Server/` as CWD, a corrupted env line, apk-on-Debian mismatch) — this branch both scaffolds the new apps and rebuilds the Docker orchestration to actually run everything together, in dev and prod, with a consistent `bibliomania-<service>` image naming scheme.

CI pipelines were originally scoped out (later brought back in — see deviations). The Go server's existing build-break (import path / method-name mismatches, tracked in `Server/app/docs/Steps.md`) is a separate concern — this branch's Docker setup is correct, but the `server` container won't actually boot until that's fixed elsewhere.

**Language decision across every new JS/TS app: plain JavaScript, no TypeScript** — applies to `web/app`, `web/main`, `admin/web`, and `admin/api` alike.

**Desktop app**: Java (JavaFX), decided after an Electron vs. JavaFX comparison (Electron reuses the React UI/team skills across web+admin+desktop; JavaFX has no code/skill overlap with the rest of the stack but was the user's preference).

**Server restructure ("Structure by Feature")**: moving `Server/internal/{handlers,services,repository,models,dto}/<domain>.go` (grouped by layer) to `Server/internal/<domain>/{handler.go,service.go,repository.go,model.go,dto.go}` (grouped by feature) was raised during planning and deliberately deferred to its own follow-up branch, to avoid tangling an architecture change with this branch's unrelated diffs and with the still-open build-break fix. **Confirmed**: its own branch (`refactor/server-feature-structure`), immediately after this one merges — see `docs/TODO.md`.

### Branch

`feat/infra-docker-stack`

### 1. Scaffold the client apps

| App | Command (run inside target dir) | Notes |
|---|---|---|
| `Client/web/main` | delete leftover Vite files first, then `npx create-next-app@latest . --javascript --tailwind --eslint --app --src-dir --import-alias "@/*" --use-npm` | Marketing site. `output: 'export'` (static export). |
| `Client/web/app` | *(originally)* `npm create vite@latest . -- --template react` | Product app. **Changed mid-implementation to Next.js** — see deviations. |
| `Client/admin/web` | `npx create-next-app@latest . --javascript --tailwind --eslint --app --src-dir --import-alias "@/*" --use-npm` | Admin dashboard frontend. `output: 'standalone'` (SSR-capable). |
| `Client/admin/api` | `npx @nestjs/cli new . --package-manager npm --skip-git --language javascript` | Admin backend, forced to plain JavaScript. |
| `Client/mobile` | `flutter create --org com.bibliomania --platforms=android,ios .` | Mobile app. |
| `Client/desktop` | Hand-written (Maven not installed locally) — `pom.xml` + `App.java`/`Launcher.java` matching the standard `javafx-archetype-simple` layout | Native desktop app, Java 21 LTS. |

Each app keeps its own `package.json`/`package-lock.json` (npm, no workspaces).

### 2. Dockerfiles — centralized under `infra/docker/<service>/`

```
infra/docker/
├── server/Dockerfile.dev, Dockerfile.prod      # moved from infra/docker/ root, bugs fixed
├── web-app/Dockerfile.dev, Dockerfile.prod
├── web-main/Dockerfile.dev, Dockerfile.prod
├── admin-web/Dockerfile.dev, Dockerfile.prod
├── admin-api/Dockerfile.dev, Dockerfile.prod
├── docker-compose.yml
├── docker-compose.dev.yml
└── docker-compose.prod.yml
```

No Dockerfile/compose service for `mobile` or `desktop` — both are locally-installed native apps, not long-lived services (CI builds them instead, see `.github/workflows/`).

Fixes applied to the pre-existing `server` Dockerfiles: `golang:X.Y-alpine` consistently (was mixing Debian `golang:X.Y` with `apk add`, which doesn't exist there), Go version aligned to `go.mod`'s `1.25.5`, prod's final stage switched from a full `golang` image to plain `alpine:3.20` (the binary is static, `CGO_ENABLED=0` — no Go toolchain needed at runtime).

### 3. Compose split — base + dev/prod overrides

Base `docker-compose.yml`: networks, volumes, every service's static shape, images named `bibliomania-<service>`. Fixed bugs: `context`/volume paths that assumed `Server/` as CWD, a corrupted `MYSQL_ROOT_PASSWORD` env line, and a migrations-into-`docker-entrypoint-initdb.d` mount that would have undone itself (MySQL runs `*.sql` files alphabetically once — golang-migrate's paired `up`/`down` files would run back-to-back).

`docker-compose.dev.yml`: dev Dockerfiles, bind mounts + node_modules volumes, dev-only `adminer` + `mailpit`.

`docker-compose.prod.yml`: prod Dockerfiles, no bind mounts, no dev-only services, `restart: always`.

Run from the repo root with `--env-file infra/docker/.env` (not `--project-directory .` — that flag also changes how the compose files' relative paths resolve and breaks them, a real behavior discovered during implementation).

### 4. nginx — subdomain routing

`bibliomania.local`, `app.bibliomania.local`, `admin.bibliomania.local` (+`/api/`), `api.bibliomania.local`, `analytics.bibliomania.local`, plus dev-only `adminer.bibliomania.local` / `mail.bibliomania.local`. Uses Docker's embedded DNS resolver (`resolver 127.0.0.11`) with `set $upstream ...; proxy_pass $upstream;` for lazy per-request hostname resolution — found during implementation that nginx otherwise resolves upstreams once at startup and refuses to boot if one isn't resolvable yet.

### 5. Env files

`infra/docker/.env.example` (compose-level: DB creds, Umami Postgres creds, Resend placeholder) + `Server/.env.example` (app-level, unchanged shape, added SMTP/Resend placeholders). The `server` container's DB_* environment is overridden from `infra/docker/.env`'s values (not left to `Server/.env`), so `mysql`'s init credentials and the app's connection credentials can never drift out of sync — found during implementation that they otherwise silently could.

### 6. Docs updates

Root `README.md`, `.claude/CLAUDE.md`, new `infra/README.md` — layout, setup, run commands, routes, known caveats.

### Verification

`docker compose ... config` (both dev and prod) to validate merged YAML/env substitution; individual `docker build` runs per service to catch real Dockerfile issues; a smoke test of `web-main`'s static export actually serving; confirming `server`'s build fails at exactly the pre-existing, already-documented `go build` bug and nowhere else.

### Deviations from the original plan

These happened after the plan above was approved, in response to follow-up instructions mid-implementation:

1. **`Client/web/app` changed from Vite+React to Next.js.** After scaffolding it as Vite+React per the approved plan, the user said "every website is in next.js" — re-scaffolded as Next.js (`output: 'standalone'`), matching `admin/web`. This shifted its dev/prod Dockerfiles and dev host port to the same shape as the other two Next.js apps instead of a Vite-specific pattern.
2. **`.github/` (CI/CD) added — not in the original plan's scope.** Added per an explicit follow-up ask: per-app CI (`<app>-ci.yml`, lint/build/test) and CD (`<app>-cd.yml`, push to `ghcr.io/dprince-03/bibliomania-<service>` on merge to `main`; `mobile`/`desktop` upload build artifacts instead) for all 8 apps/services, a PR template, issue templates, and `dependabot.yml` covering every ecosystem in the repo.
3. **Dev host ports moved off their original defaults.** The plan's dev port choices (8080, 3000-3002, 4000, 3306, 6379, 80, 8081) turned out to collide with several other projects' Docker stacks already running on this machine (checked via `docker ps`). Reassigned every dev host-port to the `9080-9090` range via dedicated `*_HOST_PORT` env vars, decoupled from the containers' internal ports (which are unaffected and unchanged).
4. **Container naming was already correct** — a follow-up question asked "is the container named after the project," and checking confirmed every service already used `bibliomania-<service>` container names plus a `name: bibliomania` compose project name; no change was needed.

## Platform vision: authors, libraries, readers (in progress)

**Status: vision agreed, implementation not started.** See [`Server/app/docs/plan.md`](../Server/app/docs/plan.md) for the Server-side technical plan this implies, and [`docs/TODO.md`](TODO.md) for the step-by-step build checklist. This section is the business/product-facing record of a long research and decision conversation, kept here so it isn't lost to chat history.

### Context

Building `Client/web/main` (the marketing site) surfaced that it couldn't be written honestly without first knowing what Bibliomania actually *is*. That question turned into a repositioning: from "a library circulation system" to a three-sided platform — authors, libraries, and readers — built to bridge authors and readers directly, eliminate printing cost as a barrier to publishing, and fix the #1 problem indie authors report (discoverability — 78% cite it as their single biggest challenge, per 2026 self-publishing survey data).

### Mission

Bridge the gap between authors and their readers/fans, eliminate the cost of printing as a barrier to publishing, and give indie books real distribution — through direct reader discovery *and* real library partnerships (the way OverDrive/Libby-style library distribution already measurably drives reads and discoverability for indie titles today).

### Three sides of the platform

- **Authors** — publish digital-first, no print run, no upfront cost. Paid via a 15% platform commission on sales.
- **Libraries** — real institutions (public, school, private), potentially multi-branch, that manage physical inventory/holds/delivery *and* curate/feature indie digital work for their patrons.
- **Readers** — subscribe to a library of their choice for access/borrowing, and separately buy individual author books outright ("buy once, read for life").

### Confirmed architectural decisions

- **Libraries are multi-tenant and multi-branch** (decided over a single-global-institution alternative): a `Library` account can have several physical `Branch` locations underneath it, matching how real county/city library systems actually operate — one system, many buildings. Each branch has its own address, delivery radius, and physical inventory. A `librarian` account belongs to exactly one library and can only manage that library's branches/inventory/holds — never another library's.
- **The digital/indie catalog stays global and library-agnostic** — no owner, no scarcity, not tied to any one library's inventory. A library can *feature*/curate an indie title without ever hosting or owning it.
- **`borrow` is not being removed — it's being redesigned as Hold + Fulfillment**, not a due-date circulation simulator. The original ask was for a way to "book a book down for your arrival, or have it delivered as long as you're within reach" — a real, long-standing library service pattern (curbside pickup, community pickup lockers, and "Books by Mail" for patrons outside a delivery radius), not a new invention:
  - Status progression: `reserved` (held for arrival) → `ready_for_pickup` / `out_for_delivery` → `active` → `returned`.
  - Fulfillment methods: `pickup` (branch or a community locker), `delivery` (courier, gated by the branch's service radius), `mail` (fallback for readers outside the radius).
- **Library verification**, before an account can go live and start collecting subscription payments: collect legal institution name, physical address(es) per branch, an official registration/license number, a supporting document upload, and an official domain email for the primary librarian-admin (not a free consumer address) — then a manual admin approval gate. Financial-grade verification (tax ID, bank account) is largely handled for free by whichever marketplace payment processor is used to pay libraries out, since that onboarding already requires it — deliberately not duplicating that compliance work in-house.
- **Librarian curation/vetting workflow — build now, not deferred.** Librarians can review and badge indie submissions ("Library Selected"). This directly answers libraries' real, researched objection to indie acquisition (57% of surveyed librarians say judging quality without professional reviews is the main obstacle) and gives authors real institutional distribution, not just algorithmic ranking.
- **Categories use BISAC Subject Headings**, the free North American book-industry-standard taxonomy (tree-structured, e.g. `FICTION → Romance → Time Travel`), instead of a bespoke genre list — authors publishing elsewhere already have codes they can reuse. A book can carry more than one code (industry norm is up to ~3), so this is a many-to-many relation on `Book`, not a single field.

### Monetization model

- **Authors**: 15% platform commission per book sold. **"Buy once, read for life"** — a one-time purchase that survives even if the reader later cancels a library subscription; a perpetual entitlement, not a lease. (Consistent with real marketplace norms — Babelcube's translator-brokering fee is also 15%.)
- **Libraries**: pay the platform a flat **$2,000/year license fee** to operate on Bibliomania. Libraries then sell their own reader subscriptions in three tiers (**Regular / Scholar / Premium Scholar**) — the platform takes **15% of that subscription revenue, per subscribing reader, per month** (a separate, recurring cut, distinct from the flat annual license).
- **Readers**: pay (a) a monthly subscription to whichever library they register under, at their chosen tier, and (b) one-time purchases for any author book that isn't free.
- **Payments infrastructure**: recommend marketplace split-payment infrastructure (Stripe Connect, or a regional equivalent) over hand-rolled fund movement — it auto-splits each charge between platform/seller/library and handles payouts, built for exactly this three-party shape. Three distinct flows, not one generic "payment": the flat library license (platform-to-library direction), the recurring reader→library subscription (85/15 auto-split), and the one-time reader→author book purchase (85/15 auto-split, perpetual entitlement).
- **DRM stance**: DRM-free, to make "read for life" real (perpetual, any-device ownership) — paired with **watermarking** (invisibly embedding the buyer's identity) instead of hard DRM, for forensic traceability against leaks without breaking the ownership promise.
- **Tier differentiation** (suggested, not finalized): Regular = standard borrowing/holds at one library; Scholar = larger concurrent-borrow limits, priority holds, research/export tools; Premium Scholar = cross-library access, discounted/included delivery, early access to newly curated indie titles.

### Feature ideas by theme

**Discovery & trust**
- Content-moderation pipeline: run uploads through an AI/plagiarism-detection API, store the score (don't auto-block), combine with an honest AI-disclosure field at upload, and route flagged-but-undisclosed content to the librarian curation queue for human review — matches how real publishers (Elsevier, Springer, IEEE via iThenticate) actually use these tools: one signal feeding human judgment, not an automatic gate. Not a hypothetical problem — AI-generated titles are already an estimated ~31% of new bestseller-list entrants industry-wide as of 2026.
- Trending/rising and new-release discovery feeds, driven by real read/borrow counts already logged.
- Public, non-authenticated, SEO-crawlable book/author pages — readers sharing links *is* the discovery/marketing engine.

**Author tools**
- Self-serve publish flow (today, upload is admin/librarian-gated only).
- **Audiobook Studio**: author picks an AI voice (or several, for dialogue) and generates a distributable, sellable audiobook edition — AI narration costs ~$8–$99/book vs. $1,200–$2,800 for human narration. Optional human-narrator marketplace later for character-heavy fiction where AI still loses.
- **Translation marketplace**, three tiers on the same 15%-brokerage model as book sales (mirrors Babelcube): pure AI (near-free, fine for non-fiction), AI draft + human post-edit (30–50% cheaper than full human, right fit for fiction), full human literary translation on a revenue-share with zero upfront author cost.
- Author analytics dashboard (completion rate, drop-off page, average reading time) — buildable almost for free, since reading-session progress data already captures the raw signal.
- Payout transparency, bundle/box-set pricing, gifting, referral perks.

**Reader experience**
- **Read Aloud** (distinct from Audiobook Studio): a personal, on-demand, real-time text-to-speech utility for whatever the reader is currently viewing, on *any* book they have legitimate access to — not a sellable product. Free default voice, premium natural-AI-voice tier as an upsell.
- **AI reading companion**: a spoiler-safe chat scoped strictly to the text of a book the reader already has access to, up to their current page — no outside knowledge, no hallucinated plot. Deliberately *not* a writing tool — extending it that direction would undercut the content-moderation defense above. Open question: whether this needs a separate author-compensation model (some platforms pay authors when a chatbot "uses" their book) or is fair as a value-add on a book the reader already paid for.
- Accessibility: dyslexia-friendly font toggle, adjustable size/line-spacing, screen-reader-clean output — an underserved market (10–15% of people have a print disability; under 10% of publications are accessible), not just a compliance checkbox.
- "Currently reading" social presence (opt-in), series-completion nudges (off existing progress data), book-club/discussion threads.

**Library / physical logistics**
- Hold + Fulfillment as described above.
- Bulk/institutional seat licensing (e.g. a school buying 200 Scholar seats at a bulk rate) — a natural extension of the tier model, a real revenue lever since many libraries here will be school libraries.

**Cross-cutting**
- Compounding AI pipeline: a translated edition can get its own AI-narrated audiobook in that language automatically — one author's book reaching a new language *and* a new format at near-zero marginal cost.

### Explicitly open questions

Not silently resolved — flagged for a real decision later:
- Payment processor choice and base currency/localization strategy.
- Exact feature-gating per reader tier (Regular/Scholar/Premium Scholar) — directional ideas only above.
- The AI reading companion's author-compensation stance.
- Whether/when a human-narrator marketplace supplements AI narration.

## `Server/app` microservices split — for hands-on distributed-systems experience

**Status: decided, design not finalized, implementation not started.**
A "no code" conversation, kept here so it isn't lost to chat history.

### Context

`Server/app` today is a single Go binary/process, feature-organized
internally (`internal/modules/auth`, `catalog`, `borrow`, `reading`,
`user`, each owning its own model/repo/service/handler) but sharing one
MySQL database and one Redis instance — a modular monolith, not
microservices, by every standard definition (single deployable unit,
shared database, in-process calls between domains, no independent
scaling or failure isolation between domains).

### Mission

Not driven by an observed production need (no scaling bottleneck, no team
split, no measured pain point exists today) — driven by wanting real,
hands-on experience building and operating an actual distributed system,
using Bibliomania as the vehicle rather than a toy tutorial with two fake
services.

### Confirmed decisions

- **Split all five domains at once** (`auth`, `catalog`, `borrow`,
  `reading`, `user`), not incrementally one service at a time. Explicitly
  chosen over the more conventional "extract one service, prove it,
  learn from it" advice, since there are no production stakes to protect
  here and the goal is the experience of coordinating five things at
  once, not risk minimization.
- **Database-per-service, with genuine polyglot persistence** — MySQL,
  Postgres, Redis, and MongoDB all represented across the five services
  (not one database technology reused five times). Directional fit
  discussed: `reading` (sessions/bookmarks/progress) is the strongest
  MongoDB candidate — document-shaped, high write volume, no real
  relational-integrity needs — versus `auth`/`catalog`/`borrow`, which
  are relational and stay MySQL/Postgres. Redis remains a cache, not a
  system of record for anything.
- **One suggested (not yet committed) sequencing note**: even building all
  five in parallel, get one path fully working end-to-end first (e.g.
  "a user borrows a book," which touches `auth`, `catalog`, `borrow`, and
  `user`) before polishing the rest — a learning-sequencing suggestion,
  not a scope change from "all at once."

### Explicitly open questions (as of the first conversation)

Not silently resolved — flagged for a real decision before implementation
starts:
- **Sync HTTP vs. event-driven** (a message broker — Kafka, RabbitMQ, or
  NATS) for how the five services talk to each other. This is the single
  biggest remaining decision — it decides whether the project is teaching
  "circuit breakers and retries" or "eventual consistency and event
  sourcing."
- **Exact data ownership per service** — which tables/fields move where,
  and what happens to data that currently lives in one MySQL table but
  conceptually spans two future services (e.g. `borrow_records`
  referencing both `users` and `books` directly via foreign key today).
- **Cross-service consistency strategy** for operations that are currently
  one MySQL transaction (e.g. "decrement available copies and create the
  borrow record") once they span two databases — Saga/compensating
  actions vs. an outbox+event pattern vs. accepted eventual consistency.
- **Idempotency, tracing, and service discovery approach** — not yet
  decided how retries are made safe, how a request is correlated across
  five processes' logs, or whether Docker Compose's built-in DNS is
  sufficient (likely yes, at this scale) versus a real service registry.

**Update, 2026-09-24 20:38 WAT — the sync/event-driven, consistency-
strategy, and data-ownership-per-service questions above are substantially
resolved by the follow-up conversation below.** Left in place rather than
deleted, per this doc's own "older entries stay put" convention — see the
new section below for what was actually decided and why, and the trimmed
open-questions list at the very end of that section for what's still
genuinely unresolved.

## `Server/app` microservices split — follow-up (2026-09-24 20:38 WAT)

Continuation of the conversation above, same "no code" discussion, kept as
its own dated section rather than edited into the original one — new
ground covered, nothing above changed or removed.

### Further confirmed decisions

- **Database per service — genuine polyglot persistence, one database
  technology per service, chosen for that service's own data shape:**
  - `auth` → **MySQL** — relational, needs real constraints (unique
    email) and ACID transactions (refresh-token rotation).
  - `catalog`, `borrow`, `user` → **Postgres** — all three relational,
    need real joins/constraints; `catalog` specifically benefits from
    Postgres's full-text search (`tsvector`, `pg_trgm`, and `pgvector`
    later if the AI-discovery ideas above ever happen) over MySQL's
    FULLTEXT.
  - `reading` (sessions/bookmarks/progress) → **MongoDB** — document-
    shaped, high write volume, no real relational-integrity needs within
    the domain. One of the genuinely good real-world MongoDB fits, not a
    case of reaching for it because it's novel.
  - **Redis** stays a cache for whichever service needs it, plus one
    narrow additional job (see brokers below) — never a system of record.
  - **The hard consequence, called out on purpose**: `borrow`'s atomic
    "reserve a copy" is currently one MySQL transaction
    (`available_copies` decrement + borrow-record insert, same DB). Once
    `catalog` (owns `available_copies`) and `borrow` have separate
    databases, that's two network calls to two services instead — see
    the Saga decision below for how this is actually handled.

- **`Client/desktop` and `Client/mobile` get local SQLite for offline
  activities.** This isn't a new mechanism — `reading_sessions
  .client_updated_at` is already `DATETIME(6)` specifically for
  last-write-wins offline-sync conflict resolution (built during the
  original Server roadmap, never consumed by an actual offline client
  until now). Reading progress/bookmarks and cached catalog browsing are
  offline-safe (additive/mergeable, or read-only). **`borrow` is
  deliberately excluded from offline support** — it allocates a scarce
  resource (a physical copy), and two offline devices "borrowing" the
  same last copy can't be reconciled after the fact without someone
  losing a book they thought they had. Borrowing requires connectivity,
  or becomes a "pending, subject to server confirmation" request rather
  than a real offline mutation.

- **Three communication protocols, each a distinct job, not overlapping
  choices:**
  - **gRPC** — internal service-to-service calls only, never client-facing.
  - **GraphQL** — a gateway/BFF layer in front of the five services,
    serving `web/app` specifically. This is the concrete reason it earns
    a place here: a book detail view needs data from `catalog` (the
    book), `borrow` (have you borrowed it), and `user` (is it on your
    shelf) — one GraphQL query resolved by fanning out to three services
    beats the frontend making three separate REST calls itself. Same
    pattern GitHub/Netflix use GraphQL for in front of their own
    microservices.
  - **REST** — stays for what doesn't need aggregation: `Client/admin`
    (simpler, more tabular), health checks, webhooks, anything external.

- **Sync vs. async, mapped per operation, not chosen globally:**
  - Reads (`GET /books`, `/search`, `/authors`, login/register, and
    `user`'s history endpoint once it's a real cross-service call to
    `reading`) — **sync**, always. Nobody logs in or reads a catalog page
    "eventually."
  - `POST /borrow`'s actual reservation — **synchronous orchestrated
    Saga**, not event-driven: `borrow-service` calls `catalog-service`'s
    reserve endpoint directly (gRPC), gets an immediate yes/no, creates
    the borrow record, and calls a compensating "release copy" endpoint
    back on `catalog-service` if that second step fails. Chosen over a
    fully choreographed async version because the browser is waiting for
    an immediate answer either way — going fully event-driven here would
    mean either blocking on an event round-trip anyway, or a
    `202 Accepted` + polling/WebSocket UI state on the frontend, which is
    a real feature cost not worth paying for a first version. A fully
    choreographed version (no sync call anywhere in the borrow flow) is a
    legitimate stretch goal once this works, not the starting point.
  - After a successful borrow — **async**: publish `BookBorrowed`, let
    downstream consumers (future waitlist notifications, analytics,
    cached availability views) react on their own time.
  - **`auth` deliberately duplicates minimal identity data** (email,
    password hash, role) rather than calling `user-service` synchronously
    on every login, even though "user" conceptually seems to own that
    data — login is the single most latency/availability-critical path
    in the system, and shouldn't go down because an unrelated service is
    having a bad day. Kept in sync via the event bus, not a live
    dependency.

- **Message brokers: Kafka + RabbitMQ + NATS, explicitly for hands-on
  exposure to each one — not a "best tool for the job" architectural
  claim.** Unlike the databases, these three don't differ by data shape —
  they all solve the identical problem (reliable message delivery), so
  there's no domain-shape reason to prefer one over another the way there
  is for MySQL vs. Mongo. The one hard rule: **a single business flow
  never crosses between brokers** — bridging two message systems just to
  talk to yourself is a real anti-pattern with no payoff, not a learning
  opportunity. Mapping (each service's own event stream lives on exactly
  one broker):
  - `catalog` → **RabbitMQ** — classic queue semantics, flexible routing,
    the on-ramp broker for straightforward discrete events (`BookAdded`,
    `CopyReserved`, `CopyReleased`).
  - `borrow` → **Kafka** — the domain where Kafka's actual headline
    feature (a durable, replayable log) earns its keep: borrowing history
    is exactly the kind of thing worth reconstructing by replaying events
    or feeding into analytics later.
  - `reading` → **NATS** — high-frequency, low-stakes-per-message
    (progress updates firing constantly while reading), precisely NATS's
    design point; mirrors `reading` already being the lightweight,
    high-throughput service via MongoDB too.
  - `auth` + `user` → also **RabbitMQ**, alongside `catalog`, rather than
    inventing a 4th/5th dedicated broker — `auth`'s events
    (`UserRegistered`, credentials changed) are exactly what
    `user`-service needs to react to (create the corresponding profile),
    and neither publishes often enough to justify owning a broker.
  - **Redis** — the one legitimate multi-broker case, but for a genuinely
    different job: ephemeral, no-persistence broadcasts (live
    notifications, cache-invalidation pings), never a peer to the other
    three for durable domain events.

### Explicitly open questions (remaining as of 2026-09-24 20:38 WAT)

Not silently resolved — flagged for a real decision before implementation
starts:
- **Exact data ownership per service** — which tables/fields move where,
  and what happens to data that currently lives in one MySQL table but
  conceptually spans two future services (e.g. `borrow_records`
  referencing both `users` and `books` directly via foreign key today).
  **Resolved 2026-09-25 09:56 WAT** — see the new dated section at the
  bottom of this file.
- **Idempotency and tracing approach** — not yet decided how retries are
  made safe (a timed-out borrow request retried shouldn't create two
  borrow records), or how a request is correlated across five processes'
  logs (a real distributed-tracing setup, e.g. OpenTelemetry, vs. just a
  shared request ID threaded through logs).
- **Service discovery** — Docker Compose's built-in DNS is likely
  sufficient at this scale; a real service registry only matters once
  instances come and go dynamically (e.g. in Kubernetes), not decided
  either way yet.
- **GraphQL gateway implementation** — which library/approach federates
  the three underlying services' schemas, not yet chosen.

## `Server/app` microservices split — additional services & platform layers (2026-09-25 09:18 WAT)

Continuation of the same conversation, own dated section per this doc's
"older entries stay put" convention — nothing above changed. User asked
for research into email, payment, Terraform, Kubernetes, monitoring, and
DB monitoring, plus a sequencing note: restructure the existing codebase
and mock up every interface next, doing whatever doesn't need code first.

### Two new domain services (join the original five)

- **`notification-service`** — email (and later SMS/push), purely an
  event consumer, no public API of its own. Reacts to `UserRegistered`
  (from `auth`), `BorrowOverdue`/`HoldReady` (from `borrow`), etc.
  Deliberately **no dedicated database** — its "history" is a log/audit
  trail, not a queryable business entity, so it leans on the observability
  stack (below) instead of a Postgres table just to remember what it sent.
  Not every service in a "database per service" architecture needs to own
  a database. Consumes from **multiple brokers** (RabbitMQ for
  `auth`/`user`/`catalog` events, Kafka for `borrow`) — this doesn't
  violate the "never cross brokers mid-flow" rule from the last section,
  since each event it reacts to is already a complete, independent flow;
  it's one service subscribing to several self-contained flows, not one
  flow spanning two brokers.
  Already half-planned: `RESEND_API_KEY` exists in `.env.example`, unused,
  with Mailpit standing in for dev — this service is what would finally
  consume it.
- **`payment-service`** — Stripe Connect integration for the three-way
  split payments already scoped in the platform-vision section above
  (library license, reader→library subscription, reader→author purchase).
  Owns transaction/subscription records, initiates checkout sessions
  synchronously, reacts to Stripe's own webhooks (an external event-driven
  integration outside our control). **Database: Postgres** — money needs
  ACID and auditability, same reasoning as `auth`. **Broker: Kafka**,
  alongside `borrow` — a financial event log is one of Kafka's textbook
  use cases, arguably a better fit than borrowing history, since replay
  matters for money in a way it doesn't for a welcome email.

### Platform layers — infrastructure every service sits on, not services themselves

- **Terraform.** Complementary to Docker Compose, not competing —
  Compose orchestrates containers on a machine you already have;
  Terraform provisions the machine/managed-services themselves.
  Sequencing recommendation: start against Terraform's own **Docker
  provider** locally, to learn the real plan/apply/state workflow without
  cloud cost or a cloud IAM model at the same time. Move to a real
  provider once that's comfortable — DigitalOcean or Hetzner over
  AWS/GCP for a personal learning project specifically for cost; DO's
  managed Kubernetes (DOKS) is a much gentler and cheaper on-ramp than
  EKS/GKE.
- **Kubernetes.** The natural ceiling Docker Compose hits once seven
  services each want independent scaling, rolling deploys, and
  self-healing. Recommendation: a **local cluster** (`kind` or `k3d`,
  both Kubernetes-in-Docker) before a paid managed cluster — Terraform can
  provision a real one later once kubectl itself is comfortable.
- **Monitoring/observability: the Grafana stack** — Prometheus (metrics),
  Loki (logs), Tempo (traces), Grafana (dashboards + alerting). One
  coherent, modern, well-documented family instead of unrelated tools
  stitched together. Instrument every service with **OpenTelemetry**
  (vendor-neutral, swappable backend) so a single request's trace threads
  through all seven services — this is the concrete answer to the
  "idempotency and tracing approach" open question above.
- **DB monitoring — not a separate system, the same stack.** Prometheus
  has a purpose-built exporter per database already in the split
  (`mysqld_exporter`, `postgres_exporter`, `mongodb_exporter`,
  `redis_exporter`), all feeding the same Grafana instance, with
  pre-built community dashboards for each. Worth pairing with DB-native
  tools too: Postgres's `pg_stat_statements` and MySQL's slow-query log,
  for catching bad queries the metrics alone won't surface. The three
  brokers (Kafka, RabbitMQ, NATS) get the same treatment — each has its
  own Prometheus exporter too, so observability covers the whole system,
  not just the app services.

### Sequencing agreed

1. Research above (this section) — done.
2. Restructure the existing codebase into the service boundaries decided
   so far — needs real code, queued after the mock-up phase below rather
   than done blind.
3. Mock up every interface — **both** UI mock-ups and interface contracts
   (OpenAPI for REST, `.proto` stubs for the gRPC inter-service calls, a
   GraphQL SDL schema for the gateway) — agreed, not yet started.
   **Dependency flagged, not yet resolved**: meaningful interface
   contracts need the "exact data ownership per service" open question
   (above) answered first — you can't write a real `.proto` for
   `catalog-service` without knowing exactly which fields it owns once
   `borrow_records`' cross-service foreign keys are untangled. UI
   mock-ups are less tightly coupled to that and could reasonably start
   first.

## `Server/app` microservices split — exact data ownership per service, resolved (2026-09-25 09:56 WAT)

Continuation of the same conversation, own dated section per this doc's
"older entries stay put" convention — nothing above changed. Resolves the
"exact data ownership per service" open question flagged in the
2026-09-24 20:38 WAT section above.

### The governing principle

Two rules that make the individual answers below fall out naturally
rather than being six separate ad hoc calls:

1. **Cross-service calls for a write's correctness happen synchronously,
   at write time, inside the service doing the writing.** Cross-service
   data needed only to make a *response* look complete is resolved at the
   **GraphQL gateway** (fan-out), never by one service calling another
   purely to enrich its own output. This keeps each service's real
   dependency count to the one or two others it actually needs for
   business logic, not "everyone calls everyone."
2. **A valid JWT already proves the user it names exists.** `auth`/the
   gateway checked that before the request reached any other service, so
   `user_id` foreign keys never need a cross-service existence check —
   that entire category of validation is free.

### Resolved, per relationship

- **`borrow_records.user_id`** — no cross-service call, ever (JWT already
  proved it).
- **`borrow_records.book_id`** (+ the `available_copies` decrement) —
  synchronous validate-and-reserve at write time, via the orchestrated
  Saga already decided. For display (the title in "my borrows"): **snapshot
  the title once, at creation**, stored locally in `borrow-service`'s own
  row — not a cache needing invalidation, a historical record (the same
  reason an e-commerce order keeps the product name as it was at purchase
  even if the product's renamed later). ~~`enrichBorrows` already does
  exactly this today; keep the behavior.~~ **Correction (2026-09-25):**
  it doesn't — `borrow.Service.enrichBorrows` looks the title up live via
  `bookRepo.GetByID`, once per record, on every read, and `BorrowRecord`
  has no title column. Snapshotting is new behavior the split must add,
  not existing behavior to keep.
- **`reading_sessions`/`bookmarks`.`user_id`** — no cross-service call
  (JWT).
- **`reading_sessions`/`bookmarks`.`book_id`** — validate existence
  **once, at session/bookmark creation**, not on every progress update.
  `reading` is the highest-frequency service (NATS) — revalidating on
  every progress tick would put a synchronous cross-service call on a hot
  path for a check that only matters once, and an invalid `book_id` here
  is a client bug, not an inventory risk the way `borrow`'s is. Title
  display: snapshot at creation, same reasoning as `borrow`.
- **`user_library.book_id`** — validate at add-time. Display is the one
  exception to the snapshot pattern: **resolve the title live**, via the
  GraphQL gateway fanning out to `catalog` when rendering "My Library" —
  a wishlist entry is an ongoing "I want this," not a historical record,
  so it should reflect the book's current title if it's ever changed.
  `user-service` itself never needs to know `catalog` exists for this.
- **`user_profiles.total_books_read` / `.total_pages_read`** — ownership
  **moves to `reading-service` entirely**, not duplicated on `user`'s
  profile row as a source of truth. `reading-service` publishes an event
  when a book is completed; `user-service` keeps a cheap,
  eventually-consistent cached copy on the profile. A few seconds of
  staleness on a "books read" counter is an acceptable trade for not
  coupling profile-page uptime to `reading-service`'s.
- **`GET /users/me/history`** — resolved at the **GraphQL gateway**,
  fanning out directly to `reading-service` — not a new `user-service` →
  `reading-service` dependency. Same reasoning as the library shelf: pure
  display aggregation for `web/app` is the gateway's job, not something to
  recreate inside a service that doesn't otherwise need it.

### The resulting dependency graph

`borrow` and `reading` each end up with exactly **one** real cross-service
dependency (`catalog`, for write-time validation) and nothing else.
`user-service` ends up with **zero** direct dependencies on `catalog` or
`reading` — everything it used to reach into `reading`'s data for is
absorbed by the gateway instead. Meaningfully simpler than "every service
calls every service it references," and wasn't obvious until each
relationship was worked through individually rather than assumed.

### Unblocks

Interface-contract mock-ups (the `.proto`/OpenAPI/GraphQL-SDL side of the
mock-up phase) can now proceed — each service's actual field list and its
one real dependency (if any) are specified above.

## `Server/app` microservices split — built (2026-09-25)

Own dated section per this doc's "older entries stay put" convention —
nothing above changed. The user asked to "do the restructure", then chose:
**everything in one pass** (all seven services, four database engines,
three brokers, gateway, observability), **carve up `Server/app` in place**
(the monolith no longer exists as a binary), **gqlgen** for the gateway,
and **API versioning** across every interface (added mid-build). This
answered the open "build order" question: split first, Steps 21-45 after.

What was built, every implementation decision, and what's still open is
recorded in one place rather than duplicated here:
[`Server/app/docs/plan.md`](../Server/app/docs/plan.md) → "Microservices
split — Server-side implications" → "Implementation" and "Still open
(after the build)". The short version of the decisions this doc's earlier
sections left open:

- **Cross-service consistency** → transactional outbox in every service
  (MongoDB runs as a single-node replica set to get transactions), and
  idempotent consumers.
- **Idempotency** → `Idempotency-Key` on borrows; `ReserveCopy`/
  `ReleaseCopy` idempotent per reservation key; queued, retried Saga
  compensation.
- **Tracing** → OpenTelemetry end to end, including through event
  envelopes; logs carry `trace_id`.
- **Service discovery** → Compose DNS.
- **GraphQL** → gqlgen, one schema, no federation.
- **Versioning** → REST by URL path with Deprecation/Sunset headers, gRPC by
  proto package (`buf breaking` in CI), events by `schema_version`, GraphQL
  by `@deprecated`.

One refinement of the 2026-09-25 09:56 data-ownership section: user-service
*does* call catalog-service, once, to validate a book being added to a
shelf. That's a write-time correctness check, which that section's own rule
1 allows. Its "zero dependencies" claim was about display, and that part
holds.

Verified on the full dev stack with `infra/docker/smoke-test.sh` (41
checks across every cross-service path, all passing), plus Prometheus,
Tempo and Loki checks.

## Follow-up: Kubernetes, Paystack, SQL init, `pkg/` clients (2026-09-25)

Own dated section, same convention. The user pointed out four things the
build hadn't covered. Three of them had ambiguous scope, so I asked:

- **"sql in infra/docker"** → database init as plain `.sql`
  (`infra/docker/{postgres,mysql}/init/`), replacing the Postgres shell
  script. Passwords are read from env with psql `\getenv`. Tables still
  come only from each service's migrations.
- **Paystack** → alongside Stripe, picked by the book's currency (NGN/GHS/
  ZAR/KES → Paystack). Webhooks are HMAC-verified and then re-checked
  against Paystack's Verify API; a mismatched amount never grants a book.
- **Kubernetes** → the user asked "which is best? use that". Chosen:
  Kustomize (base + local/prod overlays) on kind, with Terraform creating
  the cluster and Traefik as ingress. ingress-nginx was retired in March
  2026, and a Helm-based stack would lean on Bitnami charts, whose images
  went behind a paid tier in 2025. Reasoning in `infra/k8s/README.md`.
- **Postgres client in `Server/app/pkg`** → `pkg/postgresclient`, with
  `mysqlclient` and `mongoclient` alongside it for consistency.

Verified: `terraform apply` builds the whole stack on kind (33 pods), and
the same 42-check smoke test passes there and on Compose (55 checks after
the 2026-09-26 hardening pass, also on the replicated `local-ha` cluster). A Paystack
purchase ran end to end against a fake Paystack API. Details and the bugs
found along the way: `docs/CHANGELOG.md` and `Server/app/docs/plan.md` →
"Follow-up".
