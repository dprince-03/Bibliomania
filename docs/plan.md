# Project Plans

A running log of major initiative plans for Bibliotheca, kept for reference —
each as agreed on and, where implementation has happened, as actually
implemented. Newest/most relevant entries are added as new `##` sections;
older ones stay put as historical record rather than getting overwritten.

## Infra branch plan (`feat/infra-docker-stack`)

This is the plan agreed on for the infra branch, kept for reference. It was
approved before implementation started, then adjusted twice mid-implementation
per follow-up instructions — see "Deviations from the original plan" at the
bottom for what actually changed and why.

### Context

Bibliotheca is expanding from "Go API + one React scaffold" into a real monorepo: a product web app, a marketing site, an admin dashboard (its own frontend + backend), and a mobile app, all fronted by nginx and backed by MySQL/Redis/Umami. The user had already carved out the target directory skeleton (`Client/web/{app,main}`, `Client/admin`, `Client/mobile`, `infra/nginx`) but left them empty except for a leftover Vite scaffold in `Client/web/main`. The existing `infra/docker/` compose setup was written back when the Go server was the only service and had since bit-rotted (paths assumed `Server/` as CWD, a corrupted env line, apk-on-Debian mismatch) — this branch both scaffolds the new apps and rebuilds the Docker orchestration to actually run everything together, in dev and prod, with a consistent `bibliotheca-<service>` image naming scheme.

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
| `Client/mobile` | `flutter create --org com.bibliotheca --platforms=android,ios .` | Mobile app. |
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

Base `docker-compose.yml`: networks, volumes, every service's static shape, images named `bibliotheca-<service>`. Fixed bugs: `context`/volume paths that assumed `Server/` as CWD, a corrupted `MYSQL_ROOT_PASSWORD` env line, and a migrations-into-`docker-entrypoint-initdb.d` mount that would have undone itself (MySQL runs `*.sql` files alphabetically once — golang-migrate's paired `up`/`down` files would run back-to-back).

`docker-compose.dev.yml`: dev Dockerfiles, bind mounts + node_modules volumes, dev-only `adminer` + `mailpit`.

`docker-compose.prod.yml`: prod Dockerfiles, no bind mounts, no dev-only services, `restart: always`.

Run from the repo root with `--env-file .env` (not `--project-directory .` — that flag also changes how the compose files' relative paths resolve and breaks them, a real behavior discovered during implementation).

### 4. nginx — subdomain routing

`bibliotheca.local`, `app.bibliotheca.local`, `admin.bibliotheca.local` (+`/api/`), `api.bibliotheca.local`, `analytics.bibliotheca.local`, plus dev-only `adminer.bibliotheca.local` / `mail.bibliotheca.local`. Uses Docker's embedded DNS resolver (`resolver 127.0.0.11`) with `set $upstream ...; proxy_pass $upstream;` for lazy per-request hostname resolution — found during implementation that nginx otherwise resolves upstreams once at startup and refuses to boot if one isn't resolvable yet.

### 5. Env files

Root `.env.example` (compose-level: DB creds, Umami Postgres creds, Resend placeholder) + `Server/.env.example` (app-level, unchanged shape, added SMTP/Resend placeholders). The `server` container's DB_* environment is overridden from the root `.env`'s values (not left to `Server/.env`), so `mysql`'s init credentials and the app's connection credentials can never drift out of sync — found during implementation that they otherwise silently could.

### 6. Docs updates

Root `README.md`, `CLAUDE.md`, new `infra/README.md` — layout, setup, run commands, routes, known caveats.

### Verification

`docker compose ... config` (both dev and prod) to validate merged YAML/env substitution; individual `docker build` runs per service to catch real Dockerfile issues; a smoke test of `web-main`'s static export actually serving; confirming `server`'s build fails at exactly the pre-existing, already-documented `go build` bug and nowhere else.

### Deviations from the original plan

These happened after the plan above was approved, in response to follow-up instructions mid-implementation:

1. **`Client/web/app` changed from Vite+React to Next.js.** After scaffolding it as Vite+React per the approved plan, the user said "every website is in next.js" — re-scaffolded as Next.js (`output: 'standalone'`), matching `admin/web`. This shifted its dev/prod Dockerfiles and dev host port to the same shape as the other two Next.js apps instead of a Vite-specific pattern.
2. **`.github/` (CI/CD) added — not in the original plan's scope.** Added per an explicit follow-up ask: per-app CI (`<app>-ci.yml`, lint/build/test) and CD (`<app>-cd.yml`, push to `ghcr.io/dprince-03/bibliotheca-<service>` on merge to `main`; `mobile`/`desktop` upload build artifacts instead) for all 8 apps/services, a PR template, issue templates, and `dependabot.yml` covering every ecosystem in the repo.
3. **Dev host ports moved off their original defaults.** The plan's dev port choices (8080, 3000-3002, 4000, 3306, 6379, 80, 8081) turned out to collide with several other projects' Docker stacks already running on this machine (checked via `docker ps`). Reassigned every dev host-port to the `9080-9090` range via dedicated `*_HOST_PORT` env vars, decoupled from the containers' internal ports (which are unaffected and unchanged).
4. **Container naming was already correct** — a follow-up question asked "is the container named after the project," and checking confirmed every service already used `bibliotheca-<service>` container names plus a `name: bibliotheca` compose project name; no change was needed.

## Platform vision: authors, libraries, readers (in progress)

**Status: vision agreed, implementation not started.** See [`Server/app/docs/plan.md`](../Server/app/docs/plan.md) for the Server-side technical plan this implies, and [`docs/TODO.md`](TODO.md) for the step-by-step build checklist. This section is the business/product-facing record of a long research and decision conversation, kept here so it isn't lost to chat history.

### Context

Building `Client/web/main` (the marketing site) surfaced that it couldn't be written honestly without first knowing what Bibliotheca actually *is*. That question turned into a repositioning: from "a library circulation system" to a three-sided platform — authors, libraries, and readers — built to bridge authors and readers directly, eliminate printing cost as a barrier to publishing, and fix the #1 problem indie authors report (discoverability — 78% cite it as their single biggest challenge, per 2026 self-publishing survey data).

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
- **Libraries**: pay the platform a flat **$2,000/year license fee** to operate on Bibliotheca. Libraries then sell their own reader subscriptions in three tiers (**Regular / Scholar / Premium Scholar**) — the platform takes **15% of that subscription revenue, per subscribing reader, per month** (a separate, recurring cut, distinct from the flat annual license).
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
