# Server plan: platform vision → Server implications (Steps 21+)

`Server/docs/Steps.md` covers the original roadmap — Steps 1-20, all complete.
This file is its technical companion for what comes next: the Server-side
consequences of the platform-vision decisions recorded in the root
[`docs/plan.md`](../../docs/plan.md) → "Platform vision: authors, libraries,
readers". Read that section first for the *why*; this file is scoped to the
*what* — which Server modules and schema changes each decision implies.

**Nothing here is implemented yet.** Section numbers below match
`Server/docs/Steps.md`'s Step 21-45 entries exactly (same order, same
scope) — that file has the terser step-by-step summary; this one has the
fuller reasoning behind each. Each section becomes its own
`feat/step-<N>-<slug>` branch later, one at a time, per the existing
convention in `CLAUDE.md` / `docs/CONTRIBUTING.md`.

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
  (`Server/internal/modules/borrow`, see `CLAUDE.md`) still applies — just
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

## Verification

Each step above gets designed and verified individually as its own roadmap
step when it's actually built — this file states intent and shape, not a
tested implementation. Update `Server/docs/Steps.md`'s matching entry (⏳ → ✅
with real verification notes, mirroring how Steps 1-20 are documented) as
each step actually ships, and trim or update this file's corresponding
section rather than leaving it describing something already shipped.
