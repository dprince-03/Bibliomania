# TODO

Living task list — update this as work lands or priorities change. Don't let
it go stale; a wrong TODO is worse than none. See `docs/plan.md` for how the
infra branch got here, and `Server/docs/Steps.md` for the Server-side
feature roadmap (Steps 13-20).

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
      `ghcr.io/dprince-03/bibliotheca-<service>` using the default
      `GITHUB_TOKEN` — this requires the repo's Settings → Actions → General
      → Workflow permissions to allow "Read and write permissions" (not the
      read-only default). Untested — verify on first real push to `main`.

## Decided

- [x] **Desktop app stays out of Docker.** Confirmed: JavaFX is a native GUI
      app, not a long-lived headless service — CI (`desktop-ci.yml`)
      verifies the build; that's sufficient. Revisit only if a concrete need
      for a headless/VNC demo mode comes up.
- [x] **Server "Structure by Feature" refactor** — done, on
      `refactor/server-feature-structure`. `Server/internal/` now groups by
      domain (`auth/`, `user/`, `catalog/`, `borrow/`, `reading/`) instead of
      by layer — see `docs/ARCHITECTURE.md` and `CLAUDE.md` for the package
      map, `Server/docs/Steps.md` → "Build history" for what got fixed along
      the way. As an inherent side effect (not separate work), this also
      fixed the Go server build break that used to block everything below —
      `go build ./...`/`go vet ./...` succeed and the server boots and
      serves real requests now.
- [x] **Hand-written API docs added**: `Server/docs/API.md` (endpoint
      reference), `Client/docs/API.md` (which client app calls what, base
      URLs per environment), `docs/API.md` (pointer to both). Deliberately
      not the full swaggo/OpenAPI pipeline — that stays Step 18's own
      branch, see `Server/docs/Steps.md`. Keep `Server/docs/API.md` in sync
      by hand until Step 18 replaces it with something generated.

## Infra follow-ups (see `infra/README.md` → "Known caveats")

- [ ] `admin-api` has no compiled build step (`nest new --language
      javascript` scaffolding runs via `babel-node` even in prod). Add a
      `babel index.js src --out-dir dist` build script once there's real
      code, then switch `Dockerfile.prod` to a proper multi-stage build
      running `node dist/main.js`.
- [ ] TLS/HTTPS for prod nginx (certbot or similar) — not set up.
- [ ] A full `docker compose up --build` (dev) has never been run
      end-to-end — only `config` validation and individual `docker build`
      runs per service. Now unblocked (server build break is fixed) — worth
      doing.
- [ ] Mobile/desktop CD currently uploads unsigned build artifacts only — no
      code-signing or app-store/distribution pipeline exists for either
      platform.

## Product code

- [x] `Server`: all 20 roadmap steps done (config/DB/Redis, auth, authors,
      books, search, e-library upload/download, reading sessions/sync/
      bookmarks, borrowing, user/member management, Swagger docs, Makefile +
      `make seed`, final polish — consolidated global error handler, real
      `/health` DB/Redis liveness check). See `Server/docs/Steps.md`.
- [ ] `Client/web/app` — the actual library product app (currently the bare
      Next.js template).
- [ ] `Client/web/main` — the marketing site (currently the bare Next.js
      template).
- [ ] `Client/admin/web` + `Client/admin/api` — the admin dashboard
      (currently bare templates on both sides).
- [ ] `Client/mobile` — the mobile app (currently `flutter create`'s default
      counter demo).
- [ ] `Client/desktop` — the desktop app (currently a single placeholder
      window).

## Platform vision — step-by-step build plan

Not started yet. Step numbers below match `Server/docs/Steps.md` exactly
(Steps 21-45 are listed there in this same order, with more implementation
detail per step) — this list is the cross-app view, since `Server/docs/Steps.md`
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
- [ ] **Step 27 — Payments/billing module.** Pick a marketplace payment
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

## Testing

- [ ] No tests exist anywhere in the repo yet (Server has zero `_test.go`
      files; every Client app has only its framework's default test
      scaffold, if any). Add tests alongside the product code above, not as
      a separate retrofitting pass.
