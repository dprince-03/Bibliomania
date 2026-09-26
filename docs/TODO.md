# TODO

Living task list — update this as work lands or priorities change. Don't let
it go stale; a wrong TODO is worse than none. See `docs/plan.md` for how the
infra branch got here, `Server/app/docs/Steps.md` for the Server-side
roadmap (Steps 1-20 done, 21-45 planned), `Client/docs/web-app-Steps.md`
for `Client/web/app`'s own roadmap (all 7 steps done), and `docs/CHANGELOG.md` for a dated record of
every change/fix, including ones too small to belong in any of the above.

## Known repo hygiene issue

- [x] `Client/admin/api/node_modules/` (~4,051 files) was tracked in git —
      root cause was a too-narrow `.gitignore` pattern (`*/node_modules`
      only matches one directory deep; `Client/admin/api/node_modules` is
      three deep) plus `admin/api` never having its own `.gitignore` at all.
      Fixed on `refactor/server-feature-structure`: root `.gitignore` now
      has a proper `node_modules/` pattern (matches any depth),
      `Client/admin/api/.gitignore` added, and the tracked files removed via
      `git rm -r --cached`.
      **Still open**: this only stops recurrence and cleans up going
      forward — the blob data is still in `main`'s already-pushed git
      history (from `feat/infra-docker-stack`'s merge commit). Fully
      purging it needs a history rewrite (`git filter-repo`/BFG) +
      force-push to `main`, which wasn't done here given how disruptive
      that is to a shared branch — do this deliberately, separately, if it
      matters enough to justify the disruption.

## Blocking / high priority

- [ ] **Confirm GHCR push permissions.** The `*-cd.yml` workflows push to
      `ghcr.io/dprince-03/bibliomania-<service>` using the default
      `GITHUB_TOKEN` — this requires the repo's Settings → Actions → General
      → Workflow permissions to allow "Read and write permissions" (not the
      read-only default). Untested — verify on first real push to `main`.

## Decided

- [x] **Desktop app stays out of Docker.** Confirmed: JavaFX is a native GUI
      app, not a long-lived headless service — CI (`desktop-ci.yml`)
      verifies the build; that's sufficient. Revisit only if a concrete need
      for a headless/VNC demo mode comes up.
- [x] **Server "Structure by Feature" refactor** — done, on
      `refactor/server-feature-structure`. `Server/app/internal/` now groups by
      domain (`auth/`, `user/`, `catalog/`, `borrow/`, `reading/`) instead of
      by layer — see `docs/ARCHITECTURE.md` and `.claude/CLAUDE.md` for the package
      map, `Server/app/docs/Steps.md` → "Build history" for what got fixed along
      the way. As an inherent side effect (not separate work), this also
      fixed the Go server build break that used to block everything below —
      `go build ./...`/`go vet ./...` succeed and the server boots and
      serves real requests now.
- [x] **Hand-written API docs added**: `Server/app/docs/API.md` (endpoint
      reference), `Client/docs/API.md` (which client app calls what, base
      URLs per environment), `docs/API.md` (pointer to both). Deliberately
      not the full swaggo/OpenAPI pipeline — that was Step 18's own
      branch. **Since landed**: Step 18 is done, the Swagger UI at
      `/swagger/*` is now the generated endpoint reference, and
      `Server/app/docs/API.md` was trimmed to cross-cutting conventions
      only (envelope, pagination, how to regenerate the spec).
- [x] **Server split into `Server/app` (Go) + `Server/admin` (NestJS)** —
      done, on `refactor/server-app-admin-split`. `Client/admin/api` moved to
      `Server/admin` since it's a real backend, not a frontend; `Client/admin/web`
      collapsed to `Client/admin` now that there's no sibling `api` to
      disambiguate from. Docker Compose services renamed to match
      (`server`→`app`, `admin-api`→`admin`). See `docs/plan.md`.

## Infra follow-ups (see `infra/README.md` → "Known caveats")

- [ ] `Server/admin` (Compose service `admin`, formerly `admin-api`) has no compiled build step (`nest new --language
      javascript` scaffolding runs via `babel-node` even in prod). Add a
      `babel index.js src --out-dir dist` build script once there's real
      code, then switch `Dockerfile.prod` to a proper multi-stage build
      running `node dist/main.js`.
- [ ] TLS/HTTPS for prod nginx (certbot or similar) — not set up.
- [ ] A full `docker compose up --build` (dev) of *every* container has
      still never been run in one go. The backend half — all eight Go
      services, databases, brokers and observability — was brought up and
      tested end to end on 2026-09-25 (microservices split); the Node
      clients, nginx and Umami weren't part of that run.
- [ ] Mobile/desktop CD currently uploads unsigned build artifacts only — no
      code-signing or app-store/distribution pipeline exists for either
      platform.

## Product code

- [x] `Server/app`: all 20 roadmap steps done (config/DB/Redis, auth, authors,
      books, search, e-library upload/download, reading sessions/sync/
      bookmarks, borrowing, user/member management, Swagger docs, Makefile +
      `make seed`, final polish — consolidated global error handler, real
      `/health` DB/Redis liveness check). See `Server/app/docs/Steps.md`.
- [x] `Client/web/app` — the actual library product app. All 7 steps of
      its own roadmap done (foundations, auth, catalog browsing, borrowing,
      profile & personal library, e-library reader via epub.js, polish) —
      see `Client/docs/web-app-Steps.md` / `web-app-plan.md`.
- [x] `Client/web/main` — the marketing site: Home/Features/About/Contact,
      live book/author counts from the API, "warm & literary" design, and a
      clearly-labeled "coming soon" section on About previewing the
      platform-vision direction without claiming unbuilt features are live.
- [ ] `Client/admin` + `Server/admin` — the admin dashboard
      (currently bare templates on both sides). **UI mock-up done
      2026-09-25** (7 screens: Dashboard, Books, Book edit, Authors, Users,
      Borrows, Curation queue):
      https://claude.ai/artifact/UFbdfib7sPxTj8NYebQY3P
- [ ] `Client/mobile` — the mobile app (currently `flutter create`'s default
      counter demo). **UI mock-up done 2026-09-25** (shared with desktop,
      below). Local SQLite for offline reading/catalog is decided (see
      `docs/plan.md`); borrowing stays online-only.
- [ ] `Client/desktop` — the desktop app (currently a single placeholder
      window). **UI mock-up done 2026-09-25** — mobile + desktop, 9
      screens: https://claude.ai/artifact/XMpA8hcKo8UEeez1onPnTK. Same
      SQLite offline decision as mobile.

## Platform vision — step-by-step build plan

Not started yet. Step numbers below match `Server/app/docs/Steps.md` exactly
(Steps 21-45 are listed there in this same order, with more implementation
detail per step) — this list is the cross-app view, since `Server/app/docs/Steps.md`
only covers the Server. See `docs/plan.md` → "Platform vision: authors,
libraries, readers" for the full business/product context. Dependency-ordered —
later items build on the schema/tenancy work earlier ones establish.

- [ ] **Step 21 — `Library`/`Branch` schema + multi-tenant migration** (new
      Server `library` module). Foundation everything below depends on —
      libraries become real multi-branch institutions, not a single global
      catalog.
- [ ] **Step 22 — Scope `librarian` RBAC to one library.** No more flat
      global librarian role — a librarian account manages exactly one
      library's branches/inventory/holds.
- [ ] **Step 23 — Redesign `borrow` into Hold + Fulfillment.** Status
      progression (`reserved` → `ready_for_pickup`/`out_for_delivery` →
      `active` → `returned`) and fulfillment methods (`pickup`/`locker`/
      `delivery` within a branch's radius/`mail` outside it); copies move
      from a global pool to per-branch ownership.
- [ ] **Step 24 — Library signup, verification, and manual admin-approval
      flow.**
- [ ] **Step 25 — Librarian curation/"Library Selected" badging workflow**
      for indie books — addresses libraries' real stated obstacle (judging
      quality without professional reviews).
- [ ] **Step 26 — Adopt BISAC Subject Headings** as the category taxonomy
      (many-to-many on `Book`), replacing the current single `Genre` string
      field.
- [ ] **Step 27 — Payments/billing module.** (First slice built as
      `payment-service`: one-time purchases via Stripe Checkout or
      Paystack, by currency. Licences, subscriptions and payout splits —
      Stripe Connect / Paystack Split — remain.) Pick a marketplace payment
      processor (open decision — see `docs/plan.md`'s open questions); build
      the three flows (flat $2,000/yr library license, recurring 85/15-split
      reader→library subscription, one-time 85/15-split reader→author
      purchase); perpetual "buy once, read for life" purchase entitlements;
      watermarking instead of DRM.
- [ ] **Step 28 — Reader subscription tiers** (Regular/Scholar/Premium
      Scholar) with concrete feature gates defined.
- [ ] **Step 29 — Content-moderation pipeline at upload** —
      AI/plagiarism-detection API call, honest AI-disclosure field, routing
      flagged content to the curation queue rather than auto-blocking.
- [ ] **Step 30 — Author analytics dashboard** — aggregation over existing
      `reading_sessions` data (completion rate, drop-off page, avg reading
      time); no new tracking needed.
- [ ] **Step 31 — Translation marketplace + Audiobook Studio** — author-side
      production tools (pure-AI / AI+human-post-edit / full-human-revenue-
      share translation; AI-narrated audiobook editions).
- [ ] **Step 32 — Reader-side Read Aloud + accessibility features** —
      on-demand TTS utility (distinct from Audiobook Studio) plus
      dyslexia-friendly font and adjustable size/spacing in the reading UI.
- [ ] **Step 33 — AI reading companion** — spoiler-safe chat scoped strictly
      to a book the reader already has access to, up to their current page.
- [ ] **Step 34 — Social/engagement layer** — follow, "currently reading"
      presence, series-completion nudges, book-club threads, gifting,
      referral perks.
- [ ] **Step 35 — Data collection & privacy policy** — GDPR-aligned data
      minimization stance, no ad-tracking; foundational for every step
      after it, not code-first.
- [ ] **Step 36 — Encryption baseline + MFA** — AES-256 at rest, TLS 1.3 in
      transit, RSA-2048/ECC key exchange; MFA required on library-admin and
      author-payout accounts.
- [ ] **Step 37 — Observability** — self-hosted OpenTelemetry + Prometheus +
      Grafana, threaded alongside the existing request-ID middleware.
      (Largely built with the microservices split — full Grafana stack,
      per-DB and per-broker exporters, services dashboard. Alerting and
      community dashboards remain.)
- [ ] **Step 38 — Third-party penetration test** — formal, paid, external;
      hard launch-blocker on Step 27 accepting real payments in production
      (may still be built/tested in dev beforehand).
- [ ] **Step 39 — Encrypted local storage (mobile + desktop)** —
      Readium-LCP-style encrypted offline cache, covering both readers'
      purchased/borrowed books and authors' unpublished drafts. Not a
      reintroduction of DRM on the entitlement itself — Step 27's purchase
      records stay permanent/portable regardless.
- [ ] **Step 40 — Note-taking** — highlights/notes alongside the existing
      bookmark model, synced across devices.
- [ ] **Step 41 — Virtual book club** — scheduling, live video/chat (external
      integration), polls, AI-generated discussion questions reusing Step
      33's reading companion; authors can join their own book's discussion
      natively.
- [ ] **Step 42 — Reading goals dashboard** — annual goal (Goodreads-style)
      plus a daily streak counter, off existing reading-session data.
- [ ] **Step 43 — Data export & account deletion** — GDPR portability/
      erasure flow.
- [ ] **Step 44 — Public status page** — externally-visible uptime, built
      off the existing `/health` signal.
- [ ] **Step 45 — Backup & disaster-recovery policy** — documented, tested
      DB backup/restore procedure.
- [ ] **Resume the `Client/web/main` marketing-site plan** once positioning
      reflects what's actually real, rather than the pre-vision "library
      management system" framing. (Not a numbered Server step — this is
      Client-side.)

## `Server/app` microservices split

Decided 2026-09-24/25, **built 2026-09-25** on
`refactor/server-microservices`. Decisions in `docs/plan.md`; what was
built and every implementation decision in `Server/app/docs/plan.md` →
"Microservices split — Server-side implications" → "Implementation".

- [x] Decide service boundaries, database per service, protocols
      (gRPC/GraphQL/REST), sync vs. async per operation, broker per
      service, and exact data ownership per relationship.
- [x] Research + decide `notification-service`, `payment-service`, and the
      platform layers (Terraform, Kubernetes, Grafana-stack observability,
      DB monitoring).
- [x] Interface contracts — now real: `Server/app/proto/` (catalog, borrow,
      reading, user; auth needs none — nothing calls it synchronously) and
      `Server/app/internal/gateway/graph/schema.graphqls`. REST is
      documented by the generated Swagger (the gateway serves it).
- [x] UI mock-ups for the surfaces without designs yet (admin dashboard,
      mobile + desktop) — links under "Product code" above.
- [x] Build order: split first, Steps 21-45 after.
- [x] Idempotency (`Idempotency-Key` on borrows, idempotent reserve/
      release, outbox + deduped consumers), service discovery (Compose
      DNS), GraphQL library (gqlgen, no federation).
- [x] Restructure into seven services + gateway, including the title
      snapshot on borrow/reading records.
- [x] API versioning across REST (URL path + deprecation headers), gRPC
      (proto packages, `buf breaking` in CI), events (`schema_version`),
      GraphQL (`@deprecated`).
- [x] Compose/infra: Postgres, MongoDB, Kafka, RabbitMQ, NATS, the Grafana
      stack + exporters; host ports in `infra/README.md` + `docs/PORTS.md`;
      CI/CD per service.
- [x] Verified end to end: `infra/docker/smoke-test.sh`, 55 checks passing
      on Compose and on the HA kind cluster (including after failovers and
      through the Linkerd mesh).
- [ ] **Move `web/app` onto GraphQL** — it still uses REST through the
      gateway, so the fan-out the gateway was built for isn't used yet.
- [ ] Decide whether the monolith's old dev data (`bibliomania` MySQL,
      `mysql_data` volume) needs migrating into the new databases — not
      migrated; a dev stack starts empty (`make seed`).
- [x] mTLS between services (Linkerd, `infra/k8s/components/mesh`; gRPC
      ports accept meshed clients only) — 2026-09-26.
- [ ] Security before any real deployment: auth on MongoDB (replica-set
      keyfile), Kafka and NATS.
- [x] Alerting: 15 Prometheus rules + Alertmanager (Mailpit in dev, Slack
      in prod) — 2026-09-26.
- [ ] Observability leftover (Step 37): import the exporters' community
      dashboards (IDs in `infra/README.md`).
- [x] Kubernetes (`infra/k8s`: Kustomize base + local/prod overlays) on a
      local kind cluster created by Terraform (`infra/terraform/local`),
      Traefik ingress.
- [x] Paystack alongside Stripe (by currency); DB connectors in
      `Server/app/pkg/`; database init as plain `.sql` in `infra/docker/`.
- [ ] A real cluster: pick a provider (DigitalOcean/Hetzner managed
      Kubernetes per the plan), swap Terraform's `kind_cluster` for it,
      finish `infra/k8s/overlays/prod` (domain, cert-manager, secrets
      management, metrics-server), and decide managed vs. in-cluster
      databases.
- [x] GitOps deploy: `app-cd.yml` → `promote` pins the Go images in
      `overlays/prod` to the commit SHA; the Argo CD Application
      (`infra/k8s/argocd/`) syncs it. Secrets via Sealed Secrets (`make
      seal`). Real sync is blocked on a production cluster and on this
      branch reaching `main`.
- [ ] Client images (web-app, web-main, admin-web, admin) aren't pinned by
      `promote` yet — only the eight Go services are.
- [ ] Re-run the host-port conflict check with Docker Desktop running (the
      2026-09-25 check could only see the system engine — `docs/PORTS.md`).

## Hardening pass (2026-09-26)

Details: `Server/app/docs/plan.md` → "Hardening pass", `docs/CHANGELOG.md`.

- [x] Concurrency fixes (refresh rotation, copy counts, pending checkouts)
      + Testcontainers integration tests proving them.
- [x] Input/upload validation, gateway-wide idempotency, circuit breakers
      and degraded reads, distributed rate limiting.
- [x] SeaweedFS object storage (catalog scales out), Kafka partitions, prod
      HPAs.
- [x] HA overlay (`local-ha`): CloudNativePG, MySQL replica, Mongo replica
      set, Kafka/NATS/RabbitMQ clusters — failover-tested. Backups
      (Compose script + nightly CronJob).
- [x] Public UUIDv7 IDs + the auth ↔ user reconciler.
- [x] CI security scans, golangci-lint, k6 load test, alerting, Linkerd,
      Sealed Secrets, Argo CD.
- [ ] Sharding — designed (plan.md), deliberately not built; revisit when a
      table outgrows one node.
- [ ] MySQL automatic failover (an operator such as Percona's) if auth's
      availability needs become stricter than a manually promoted standby.
- [ ] Decide the Go toolchain line: go.mod says 1.25.5 (minimum), images
      build with 1.26.x; Dependabot is already proposing 1.27.

## Testing

- [ ] Server has unit tests (borrow Saga, API versioning, payments,
      idempotency, resilience, middleware, uploads, telemetry …),
      Testcontainers integration tests (`go test -tags integration ./...`),
      the 55-check smoke test and a k6 load test (`infra/loadtest/`); much
      service logic still has no unit tests, and Client apps have only framework default scaffolds. Add
      tests alongside new product code, not as a separate retrofitting
      pass.
