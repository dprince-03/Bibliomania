> Status as of 2026-08-26: **all 20 roadmap steps are done.** `go build ./...` /
> `go vet ./...` succeed — see "Build history" at the bottom of this file
> for how it got there. `internal/modules/` is organized **by feature**
> (`auth/`, `user/`, `catalog/`, `borrow/`, `reading/`), not by layer — see
> `docs/ARCHITECTURE.md` for the package map. Step 20 (final hardening —
> consolidated global error handler, real DB+Redis `/health` check) was the
> last one to land, closing out this original roadmap.
>
> **This isn't the end of Server work** — a platform-vision repositioning
> (multi-branch libraries, payments/commissions, curation, AI features,
> security/observability, and reader/community features) was agreed after
> Step 20 landed. **Steps 21-45 are listed below, ⏳ not started**, in the
> same dependency order as `Server/app/docs/plan.md`'s fuller technical writeup
> (start with Step 21 — everything from Step 22 onward depends on it; Step
> 38 is a hard launch-blocker on Step 27, not a strict build-order
> dependency). Root [`docs/plan.md`](../../../docs/plan.md) has the full
> business/product context these steps implement.

```
✅ Step 1  — Scaffold: folder structure, go.mod, git init

✅ Step 2  — Config: .env, config.go, godotenv

✅ Step 3  — MySQL: connection, pooling, migrations (10 tables)
             users, authors, users_profile, books, borrows_records,
             refresh_tokens, book_authors, reading_sessions,
             user_library, bookmarks

✅ Step 4  — Redis: connection, cache interface, key helpers

✅ Step 5  — Models, DTOs, Errors, Utils
             Now grouped by feature (see "Build history" → restructure), not
             in their own top-level folders:
               internal/modules/auth/model.go, dto.go       → RefreshToken; Register/Login/Refresh/Token/AuthResponse
               internal/modules/user/model.go, dto.go       → User, UserProfile; UserResponse, UpdateProfileRequest, ...
               internal/modules/catalog/author_model.go, author_dto.go, book_model.go, book_dto.go, book_author_model.go
               internal/modules/borrow/model.go, dto.go     → BorrowRecord; BorrowRequest, BorrowResponse
               internal/modules/reading/model.go, dto.go    → ReadingSession, UserLibrary, Bookmark; their DTOs
             errors/ and utils/ stay shared/top-level (not feature-specific):
               errors.go       → AppError, typed HTTP errors (404, 401, 403, 422)
               response.utils.go → Success/Error JSON response formatters
               pagination.utils.go → Page, limit, offset helpers
               file.utils.go   → File type/size validation helpers
               hash.utils.go   → bcrypt password hash/compare

✅ Step 6  — Repository Layer
             One file per domain, pure DB queries, zero business logic.
             Now lives inside each feature package (see Step 5) with
             consistent GetByID/GetAll/Create/Update/Delete-style names —
             the earlier method-name mismatch between interface and caller
             is resolved as part of the restructure:
             internal/modules/user/repository.go     → Repository, ProfileRepository
             internal/modules/catalog/author_repository.go, book_repository.go, book_author_repository.go
             internal/modules/borrow/repository.go   → Repository
             internal/modules/reading/repository.go  → SessionRepository, LibraryRepository, BookmarkRepository
             internal/modules/auth/repository.go     → TokenRepository

✅ Step 7  — Authentication
             pkg/jwt/
               jwt.go          → GenerateAccessToken, ParseAccessToken, NewManager (was misnamed NewManger)
             pkg/refreshToken/
               refreshToken.go → Generate, HashToken
             internal/modules/auth/
               service.go      → Register, Login, Logout, RefreshToken
               handler.go      → HTTP handlers for the routes below
             Routes:
               POST /api/v1/auth/register
               POST /api/v1/auth/login
               POST /api/v1/auth/logout
               POST /api/v1/auth/refresh

             Bug fix (found while building Client/web/app's Step 2 — see
             Client/docs/web-app-Steps.md): issueTokens() in
             auth/service.go returned TokenResponse.ExpiresIn using the
             *refresh* token's TTL (604800s) for every register/login/
             refresh response, not the access token's (900s) — the JWT
             itself was always correctly minted with a 15-minute expiry
             via jwt.Manager, only the reported expires_in was wrong.
             Fixed by adding jwt.Manager.AccessTokenTTL() and using it in
             issueTokens instead of s.refreshTTL. Caught by a frontend
             client testing the real response shape, not by re-reading
             this code — a reminder that "the API works" isn't fully
             verified until something outside the Server actually
             consumes every field it returns.

✅ Step 8  — Core Middleware
             middleware/logger.go      → structured request/response logging
             middleware/cors.go        → allowed origins, headers, methods
             middleware/recovery.go    → catch panics → return clean 500
             middleware/auth.go        → validate JWT → inject user into context
             middleware/requestID.go   → generates/propagates a request ID (not originally planned until Step 20)
             middleware/security.go    → security headers (also pulled forward from Step 20)

✅ Step 9  — RBAC + Ownership Middleware
             middleware/rbac.go        → RoleRequired(admin, librarian, member), SelfOrAdmin
             middleware/ownership.go   → OwnershipRequired (generic resource-owner check, admin bypass)
             Roles: admin / librarian / member (constants in rbac.go)

✅ Step 10 — Rate Limiting Middleware
             middleware/rate_limiter.go
             Per-IP token bucket via golang.org/x/time/rate, with idle-IP cleanup goroutine
             Configurable RPS + burst from config; a stricter store is used for auth routes

✅ Step 11 — Authors Module
             internal/modules/catalog/author_handler.go → GetAll, GetByID, GetBooksByAuthor, Create, Update, Delete
             internal/modules/catalog/author_service.go → business logic + Redis cache
             Routes:
               GET    /api/v1/authors
               GET    /api/v1/authors/:id
               GET    /api/v1/authors/:id/books
               POST   /api/v1/authors          [librarian, admin]
               PUT    /api/v1/authors/:id      [librarian, admin]
               DELETE /api/v1/authors/:id      [admin]

✅ Step 12 — Books Module (Search folded in early, see Step 13)
             internal/modules/catalog/book_handler.go → CRUD + assign/remove authors + search
             internal/modules/catalog/book_service.go → business logic + Redis cache
             Routes:
               GET    /api/v1/books
               GET    /api/v1/books/:id
               GET    /api/v1/search
               POST   /api/v1/books            [librarian, admin]
               PUT    /api/v1/books/:id        [librarian, admin]
               DELETE /api/v1/books/:id        [admin]
               POST   /api/v1/books/:id/authors [librarian, admin]
               DELETE /api/v1/books/:id/authors/:authorId [librarian, admin]

✅ Step 13 — Search + Filtering + Pagination
             internal/modules/catalog/book_repository.go → Search(ctx, BookSearchParams, limit, offset)
             internal/modules/catalog/book_dto.go        → BookSearchParams (Query, Genre, Format, AuthorID, Year)
             `q` matches book title/description (MySQL FULLTEXT, idx_books_search)
             OR author first/last name (LEFT JOIN book_authors + authors) —
             e.g. q=Rowling finds her books even with no title/description match.
             genre/format(digital|physical)/author(id)/year are exact-match filters,
             independent of `q` and of each other.
             Redis caching of search results (TTL 2 min, key includes every filter —
             cache.KeySearchBooks in internal/cache/keys.go)
             Verified end-to-end: created 2 authors + 2 books, exercised every filter
             and both invalid-input cases (bad format, non-numeric author), all correct.
             Route (unchanged from Step 12): GET /api/v1/search?q=&genre=&format=&author=&year=&page=&limit=

✅ Step 14 — E-Library: Upload / Download / Offline Sync
             internal/modules/catalog/book_handler.go, book_service.go (extended)
               Upload validates format (pdf/epub only — file_format column is
               ENUM('pdf','epub'), so anything else is rejected before it
               ever reaches the DB) and size (MAX_UPLOAD_SIZE_MB), streams to
               {STORAGE_PATH}/books/{bookID}/{sanitized filename}, and stores
               a path RELATIVE to STORAGE_PATH in the DB (so a differently
               configured STORAGE_PATH between environments doesn't orphan
               existing records). MaxBytesReader caps the request body before
               ParseMultipartForm runs, so an oversized upload is rejected
               before being buffered.
               Download uses http.ServeFile (range requests, content-type
               sniffing) with Content-Disposition: attachment.
             internal/modules/reading/service.go, handler.go, dto.go (new —
               first code in this package beyond repository-only; scoped to
               just Sync for this step, Step 15 extends the same Service)
               Sync verifies the book exists (clean 404 instead of a raw FK
               violation), computes progress_pct/is_completed server-side
               (the client only sends current_page/total_pages), and
               delegates conflict resolution entirely to
               SessionRepository.Upsert's SQL (last write wins, compared by
               client_updated_at) — the service re-fetches afterward so the
               response always reflects the authoritative merged state, not
               necessarily what the client just sent.
               Fixed along the way: UpdateProgressRequest.CurrentPage had a
               `required` validator tag, which go-playground/validator
               treats as failing on the zero value — meaning "start reading
               at page 0" was impossible. Removed; added `required` to
               ClientUpdatedAt instead, since the whole conflict-resolution
               mechanism depends on it being present.
             Routes:
               POST  /api/v1/books/:id/upload   [librarian, admin]
               GET   /api/v1/books/:id/download  [member, librarian, admin] (i.e. any authenticated user)
               PATCH /api/v1/reading/:bookId/sync [member, librarian, admin]
             Verified end-to-end against a live server + MySQL + Redis: wrong
             file type → 400, no auth → 401/404 cases, successful upload
             flips is_digital/file_format, download streams correct content
             with correct headers, and — the important one — sync's
             last-write-wins actually works (a stale update with an older
             client_updated_at is correctly discarded, not applied).
             `Server/app/storage/` added to root .gitignore (uploaded content,
             never committed).

✅ Step 15 — Reading Sessions + Progress + Bookmarks
             internal/modules/reading/handler.go, service.go extended (not
             recreated — Step 14 already started them with Sync).
             internal/modules/reading/dto.go gained ProgressUpdateRequest —
             deliberately a separate type from Sync's UpdateProgressRequest:
             the progress endpoint is always-online (no client_updated_at
             from the caller; the server stamps "now" itself), Sync is for
             offline clients replaying their own clock.
             Both endpoints share one upsertProgress(...) helper for the
             progress_pct/is_completed computation and the Upsert call —
             they only differ in whose timestamp feeds the conflict check.
             Bookmarks: ownership enforced in the service (DeleteBookmark
             checks both user_id and book_id against the URL, no admin
             bypass — a bookmark is a personal note, not shared library data).
             Real bug found via this step's own rapid-fire progress testing:
             `reading_sessions.client_updated_at` was plain DATETIME
             (1-second resolution). Two updates landing in the same second
             got equal, truncated timestamps, and the Upsert SQL's `>=`
             comparison means ties favor the OLD row — a legitimate second
             update was silently dropped. This wasn't hit by Step 14's Sync
             (tested once, not back-to-back) but Step 15's UpdateProgress
             (meant for frequent online calls) hit it immediately. Fixed via
             migration 000011: column is now DATETIME(6) (microsecond
             precision) — verified by reproducing the exact rapid-fire
             sequence before and after.
             Routes:
               GET   /api/v1/reading/:bookId/session
               PATCH /api/v1/reading/:bookId/progress
               GET   /api/v1/reading/:bookId/bookmarks
               POST  /api/v1/reading/:bookId/bookmarks
               DELETE /api/v1/reading/:bookId/bookmarks/:id
             Verified end-to-end (2 real registered users, not synthetic
             tokens, specifically to exercise the reading_sessions FK):
             session/progress/bookmark happy paths, page-0 acceptance, the
             rapid-fire fix above, validation errors (missing page, bad
             color, missing total_pages), 404s for nonexistent books, and —
             the one that matters most — a second user cannot delete or see
             the first user's bookmarks (403, and confirmed not deleted).

✅ Step 16 — Borrowing System
             internal/modules/borrow/handler.go, service.go (new)
             internal/config: new BorrowLoanDays (BORROW_LOAN_DAYS, default
             14) — loan period length, added since nothing like it existed.
             Auto-detect overdue on fetch: GetAll/GetMyBorrows both sweep
             borrowRepo.UpdateOverdue before reading — a read-time sweep,
             since there's no cron/worker in this codebase (verified: a
             borrow created with a due date already in the past shows
             status="active" right after Borrow, then "overdue" on the very
             next GET).
             Decrements/increments available_copies atomically:
             DecrementAvailableCopies IS the concurrency-safe check-and-update
             (WHERE available_copies > 0) — HasActiveBorrow is a separate
             business-rule guard (no double-borrowing the same book), not
             the safety mechanism. If Create fails after the decrement
             succeeds, the decrement is compensated (incremented back) so a
             failed borrow can't leak a copy.
             Return ownership: self, or librarian/admin (staff processing a
             return on someone's behalf) — enforced in the service, not
             route middleware, matching how reading/bookmark ownership was
             done in Step 15. Returning an already-returned borrow is a 409,
             not a silent no-op (double-return would double-increment
             available_copies).
             Routes:
               GET  /api/v1/borrows              [admin, librarian]
               GET  /api/v1/borrows/my           [member]
               POST /api/v1/borrows              [member]
               PATCH /api/v1/borrows/:id/return  [member, librarian, admin]
             Verified end-to-end (3 real registered users + admin): full
             borrow→return lifecycle, copies dropping to 0 across 2 borrows
             on a 2-copy book then a 3rd borrower correctly rejected (400),
             double-borrow rejected (409), cross-user return rejected (403),
             double-return rejected (409), staff-on-behalf return succeeds,
             and the overdue auto-detection above.

✅ Step 17 — Member Management + User Library
             internal/modules/user/handler.go, service.go (new)
             UserLibrary + LibraryRepository moved here from
             internal/modules/reading (Step 15 had put them there since the
             old models/reading.model.go pre-restructure file happened to
             group ReadingSession/UserLibrary/Bookmark together — but this
             step's own name, "Member Management + User Library", says
             where it actually belongs; nothing consumed LibraryRepository
             yet, so the move was free). user/model.go picked up a
             dependency on catalog (for UserLibrary.Book), same pattern as
             reading → catalog already established.
             internal/modules/user/repository.go gained Repository.GetAll —
             deliberately does NOT filter by is_active like GetByID does:
             an admin needs to see deactivated accounts too, precisely to
             reactivate them.
             internal/modules/reading/repository.go gained
             SessionRepository.GetAllByUserID, added specifically to power
             history below — ReadingSession itself stays owned by reading,
             user.Service just reads through it (one-directional
             user → reading dependency, no cycle).
             GET /users/me/history is reading activity (session data), a
             different concept from GET /borrows/my (Step 16 — physical/
             digital checkout) — the two were never meant to be the same
             list, both named "history"-ish for different reasons.
             Routes:
               GET   /api/v1/users/me
               PATCH /api/v1/users/me
               GET   /api/v1/users/me/library
               PATCH /api/v1/users/me/library/:bookId
               GET   /api/v1/users/me/history
               GET   /api/v1/users             [admin]
               PATCH /api/v1/users/:id/status  [admin]
             Verified end-to-end: profile get/update, library upsert
             (wishlist → reading transition on the same book, not a
             duplicate row), status-filtered library listing, history
             reflecting real synced reading progress, admin-only listing
             (403 for a member), and — the interesting one — deactivating a
             user via PATCH /users/:id/status immediately blocks both future
             login AND their still-valid token's own GetMe (GetByID's
             is_active filter), confirmed reactivation undoes both.

✅ Step 18 — Swagger / OpenAPI Docs
             All 37 handler methods annotated with swaggo comments (auth 4,
             catalog/author 6, catalog/book 10, reading 6, borrow 4, user 7)
             — every route in Server/app/docs/API.md now also has a live,
             interactive counterpart. General API info (title, BasePath
             /api/v1, BearerAuth security scheme) lives in cmd/api/main.go
             above func main().
             Generated via: swag init -g cmd/api/main.go --output
             internal/swaggerdocs --parseInternal --parseDependency
             (--parseInternal needed since every model lives under
             internal/, which swag skips by default; --parseDependency
             needed for utils.APIResponse/PaginatedResponse/APIError, which
             live in a different package than the handlers referencing them).
             The generated internal/swaggerdocs/ (docs.go + swagger.json +
             swagger.yaml) is committed, not gitignored — cmd/api/main.go
             blank-imports docs.go for its init() side effect (registering
             the spec), so `go build`/`go run` need it present. Whenever a
             handler's annotations change, regenerate with `make swagger`
             (Step 19 added the Makefile after this step landed).
             Served at GET /swagger/* via github.com/swaggo/http-swagger —
             registered as a single "GET /swagger/" prefix route; the
             handler parses r.RequestURI itself to find index.html/doc.json/
             assets, so no http.StripPrefix is needed.
             Verified: GET /swagger/ redirects to index.html (200), GET
             /swagger/doc.json is valid JSON with the correct title/
             basePath/37 operations/BearerAuth security definition, and spot
             checks (nested response schemas via allOf, the multipart file
             param on /books/{id}/upload, per-route security requirements)
             all matched the actual handler behavior.

✅ Step 19 — Makefile + Docker Polish
             Server/app/Makefile: run, build (→ bin/bibliotheca, gitignored),
             vet, fmt, migrate-up, migrate-down, docker-up, docker-down,
             swagger, seed.
             migrate-up/migrate-down deliberately do NOT use Make's own
             `include .env` — Make's comment character is `#`, and this
             repo's real DB_PASSWORD/REDIS_PASSWORD values contain a literal
             `#` (e.g. "@Dev_prince#2003"), which `include` would silently
             truncate. Instead each recipe sources .env as a shell script
             (`. ./.env` inside the recipe's shell), where `#` only starts a
             comment at the beginning of a word — confirmed this actually
             matters by testing against the real .env, not a sanitized one.
             docker-up/docker-down wrap the existing dev compose command
             from infra/README.md with paths relative to Server/app/
             (`../../.env`, `../../infra/docker/...`) — docker-compose.prod.yml (already built
             during the infra branch) is intentionally not wrapped here,
             since prod is a deploy-host concern, not a local dev
             convenience.
             cmd/seed/main.go (new): seeds one admin user
             (admin@bibliotheca.local) and 3 sample author+book pairs.
             Coarse-grained idempotency (skip admin seeding if that email
             exists; skip catalog seeding if any author exists at all)
             rather than per-row upserts — meant for a fresh dev database,
             not repeated runs against one already in use.
             Update (found while building Client/web/app's Step 3 —
             catalog search): after a real seed, runs `OPTIMIZE TABLE
             books` once. MySQL's InnoDB FULLTEXT index caches newly-
             inserted rows in memory and only merges them into the
             on-disk index on a size threshold or an explicit OPTIMIZE,
             not immediately on insert — without this, GET
             /api/v1/search found nothing for the very books just seeded,
             even searching their exact title. Confirmed directly in
             MySQL (MATCH/AGAINST scored 0 for every row until OPTIMIZE
             TABLE; scored correctly after).
             Verified end-to-end: `make migrate-up` against the real Server/app/.env
             (proving the `#`-in-password handling actually works, not just
             in theory), `make seed` twice (second run correctly skipped
             both admin and catalog), seeded books/login visible via the
             live API, `make build` produces a real binary, `make swagger`
             regenerates without diffing, and both docker-compose
             (dev + prod) configs resolve cleanly with Server/app/-relative paths.

✅ Step 20 — Final Polish
             Of the six items originally listed here, four were already done
             by earlier steps (input validation, security headers middleware,
             graceful shutdown, request ID middleware — done early at Step 8).
             This step did the two genuinely remaining ones:
             1. Global error handler: every handler
             (auth/catalog[author,book]/reading/borrow/user) had its own
             private `handleError(w, err)` method — five were byte-for-byte
             identical copies, and auth/handler.go had the same
             type-switch-on-*apperrors.AppError logic inlined 4 times instead
             of even a local helper. Replaced all of it with one exported
             `utils.HandleError(w, err)` in utils/response.utils.go; deleted
             the six now-redundant call sites' worth of duplicated logic.
             2. /health now actually checks liveness instead of returning a
             static `{"status":"ok"}`: new internal/health package
             (health.Checker, wrapping *sqlx.DB + *redis.Client) pings both
             with a 2s timeout and returns 200 `{"status":"ok","database":"ok","cache":"ok"}`
             when both are reachable, or 503
             `{"status":"degraded","database":"...","cache":"..."}` naming
             which one failed otherwise. Wired into main.go (constructed
             alongside db/redisClient, which were already in scope) and
             router.go (one new `healthChecker *health.Checker` param,
             replacing the static closure) — router.New is now at 9 params,
             judged acceptable since this is the final roadmap step and no
             more handlers get added after this.
             Verified end-to-end against the live server (MySQL + Redis both
             running locally): `/health` returned 200/ok with both checks ok;
             stopped the Redis container (`docker stop nexus_redis`) and
             confirmed `/health` returned 503 with `"cache":"unreachable"`
             while `"database":"ok"` stayed correct; restarted Redis and
             confirmed it recovered to 200/ok. Also re-verified the
             consolidated error handler on real request paths: a 404 from
             `GET /api/v1/books/999999` and a 422 from an invalid
             `/auth/register` body both still returned the correct shared
             envelope shape. `go build ./...`, `go vet ./...`, and `gofmt -l .`
             are all clean, and `make swagger` regenerated
             internal/swaggerdocs/ with the new `/health` annotation
             (confirmed present in swagger.json) without needing any other
             handler's annotations touched.

---------------------------------------------------------------------
Steps 21+ — platform-vision repositioning (not started; see plan.md)
---------------------------------------------------------------------

⏳ Step 21 — Library/Branch module (multi-tenant foundation)
             New internal/modules/library/ package.
             model.go   → Library, Branch (address, delivery radius)
             Everything from Step 22 onward depends on this landing first —
             physical copies, holds, and librarian scoping all become
             per-branch instead of global once this exists.
             See Server/app/docs/plan.md → "New library module".

⏳ Step 22 — Scope librarian role to one library
             middleware/rbac.go: librarian gains a library_id scope —
             a librarian account manages exactly one library's
             branches/inventory/holds, never another's.
             Touches AuthGuard/RoleRequired's context plumbing.

⏳ Step 23 — Redesign borrow: Hold + Fulfillment
             internal/modules/borrow/model.go: Status expands to
             reserved → ready_for_pickup/out_for_delivery → active →
             returned; new FulfillmentMethod (pickup/locker/delivery/mail).
             catalog.Book's global TotalCopies/AvailableCopies moves to
             per-Branch ownership. Delivery eligibility = distance check
             against the branch's radius; outside it, fall back to mail.

⏳ Step 24 — Library signup, verification, admin approval
             Collect legal name, branch address(es), registration/license
             number, a supporting document, and an official-domain admin
             email at signup; gate live status behind manual admin
             approval before a library can receive holds or payments.

⏳ Step 25 — Curation: "Library Selected" badging
             New join table in catalog: library_id, book_id, badged_by,
             badged_at. Librarians badge indie titles without the library
             ever owning/hosting them — addresses libraries' real stated
             obstacle (judging quality without professional reviews).

⏳ Step 26 — Category model: adopt BISAC Subject Headings
             catalog.Book.Genre (single string) superseded by a
             many-to-many BookCategory relation against BISAC codes
             (industry norm: up to ~3 per book), replacing the bespoke
             genre string.

⏳ Step 27 — Payments/billing module
             New internal/modules/billing/ package.
             LibraryLicense   → flat $2,000/yr, platform-to-library
             ReaderSubscription → recurring monthly, tiered, 85/15 split
             BookPurchase     → one-time, 85/15 split, PERMANENT entitlement
             (must outlive a lapsed ReaderSubscription — "buy once, read
             for life" cannot be keyed through the subscription record)
             Depends on an external payment-processor SDK decision not
             yet made (Stripe Connect or regional equivalent) — see
             plan.md's open questions before starting this step.
             LAUNCH GATE: must not accept real payments in production
             until Step 38's third-party penetration test passes — may
             still be built/tested in dev before then.

⏳ Step 28 — Reader subscription tiers
             Regular / Scholar / Premium Scholar — concrete feature gates
             (borrow limits, priority holds, cross-library access, etc.)
             to be finalized against billing.ReaderSubscription from
             Step 27.

⏳ Step 29 — Content-moderation pipeline
             Content-score + AI-disclosure field on Book, populated by an
             external plagiarism/AI-detection API call at upload. Score
             feeds the Step 25 curation queue and discovery ranking —
             never an automatic reject, human judgment stays final.

⏳ Step 30 — Author analytics dashboard
             Aggregation endpoint(s) over existing reading_sessions data
             (completion rate, drop-off page, avg reading time) — no new
             tracking needed, the raw signal already exists.

⏳ Step 31 — Translation marketplace + Audiobook Studio
             Translation  → Book has-many translated editions (file,
             language, tier: ai/ai_plus_human_edit/human, status)
             AudiobookAsset → Book has-many audio editions (one per
             voice/language), delivered like the existing e-library file.

⏳ Step 32 — Reader-side Read Aloud + accessibility
             On-demand TTS playback for any book the reader has access
             to (distinct from Step 31's produced Audiobook Studio
             editions) + dyslexia-friendly font and adjustable
             size/spacing in the reading UI. Mostly Client-side; Server's
             role is the access-check endpoint (see Step 33).

⏳ Step 33 — AI reading companion
             Spoiler-safe chat scoped strictly to a book the reader
             already has legitimate access to, up to their current page.
             Server exposes a thin authorization check (purchase/
             subscription/free) — the chat logic itself is likely a
             thin proxy to an LLM API, not new domain logic.

⏳ Step 34 — Social/engagement layer
             New internal/modules/social/ package.
             Follow (reader → author), Comment, Rating — sits naturally
             alongside reading.SessionRepository for new-release
             notifications.

⏳ Step 35 — Data collection & privacy policy
             Foundational, not code-first: define what's collected, why,
             and the GDPR-aligned stance (data minimization, no
             ad-tracking) that Steps 36+ and the billing module (Step 27)
             need to already respect, not retrofit.

⏳ Step 36 — Encryption baseline + MFA
             AES-256 at rest, TLS 1.3 in transit, RSA-2048/ECC for key
             exchange — applied across existing tables (esp. addresses
             from Step 21/24, payment records from Step 27).
             MFA required on library-admin and author-payout accounts —
             the single most common 2026 compliance-audit failure point.

⏳ Step 37 — Observability: OpenTelemetry + Prometheus + Grafana
             Self-hosted for now. Threads trace_id/span_id alongside the
             existing request_id (middleware/requestID.go, Step 8) rather
             than replacing it. Metrics → Prometheus, dashboards → Grafana,
             logs stay on the existing log/slog-based structured logger.

⏳ Step 38 — Third-party penetration test (gates Step 27 going live)
             Formal, paid, external pen test. Decided as a hard launch
             blocker: Step 27 (payments) may be built and tested in dev
             before this, but must not accept real payments in production
             until this passes.

⏳ Step 39 — Encrypted local storage (mobile + desktop)
             Readium LCP-style: on-device cache encrypted at rest, keyed
             to device+account, decrypted only in-memory inside the
             reading app. Distinct from the DRM-free *entitlement* (Step
             27's BookPurchase stays permanent/portable) — this is
             offline-cache hygiene, not a licensing restriction. Covers
             both purchased/borrowed books (readers) and authors'
             unpublished drafts/manuscripts pre-release.

⏳ Step 40 — Note-taking (highlights + notes)
             New model alongside reading.Bookmark — Note/Highlight,
             synced across devices the same way reading progress already
             is.

⏳ Step 41 — Virtual book club
             Extends internal/modules/social/ (Step 34): scheduling +
             reminders, live video/chat (external video-call integration,
             like Step 27's payment processor — not hand-rolled), polls,
             AI-generated discussion questions reusing Step 33's
             reading-companion infra. Differentiator over existing book-
             club apps: since authors are first-class accounts here, any
             author can join a discussion of their own book natively, no
             external outreach needed.

⏳ Step 42 — Reading goals dashboard
             New field(s) on user's profile: an annual goal (Goodreads-
             Reading-Challenge-style) plus a daily streak counter (the
             gap most existing reading-challenge tools, including
             Goodreads itself, don't cover). Computed off existing
             reading_sessions timestamps — no new tracking needed.

⏳ Step 43 — Data export & account deletion
             GDPR portability/erasure: export notes/highlights/history in
             one flow, plus a real account-deletion path. Cheap now,
             expensive to retrofit once an audit flags its absence.

⏳ Step 44 — Public status page
             Externally-visible uptime page off the existing /health
             signal (Step 20) — libraries paying the Step 27 annual
             license will want visible uptime, not just an internal check.

⏳ Step 45 — Backup & disaster-recovery policy
             Documented (and tested) DB backup/restore procedure — an
             operational runbook as much as code, but now load-bearing
             given real institutional and financial data lives in this
             database, not just demo data.
```

## Build history

As of **2026-08-24**, `go build ./...` and `go vet ./...` both succeed, and
`go run ./cmd/api` boots and serves real requests (verified: `/health`,
`GET /api/v1/books`, `GET /api/v1/authors`, `POST /api/v1/auth/register`,
RBAC-gated `POST /api/v1/authors` correctly 401s unauthenticated, and
`GET /api/v1/authors/:id/books` — the one path that most exercises the
`catalog` package's internal author↔book cross-references). This was fixed
as a side effect of the `refactor/server-feature-structure` branch (see
`docs/ARCHITECTURE.md` for the resulting package layout), not as separate
targeted bug-fixing — the bugs below all lived in files that branch rewrote
anyway. For posterity, everything that was actually wrong:

1. **Import path split** — `go.mod` module was the bare `bibliotheca`, but
   several files imported via the old `github.com/yourusername/bibliotheca/...`
   path. Fixed at the time: every file used the bare `bibliotheca/...`.
   **Since updated again**: the module path is now
   `github.com/dprince-03/Bibliotheca` (matching the actual GitHub repo,
   case included), so `go get`/module-proxy resolution works correctly if
   this module is ever imported from elsewhere — every internal import uses
   that full path now, not the bare `bibliotheca/...` shown above.
2. **Repository/service method-name mismatch** — the old
   `internal/repository/repository.go` defined `GetAuthorByID`,
   `GetAllAuthors`, `CreateAuthor`, `GetBookByID`, etc., but the services
   calling them expected the shorter `GetByID`, `GetAll`, `Create`. Fixed:
   every feature package's repository interface now uses the short,
   non-stuttering names its own service actually calls.
3. `internal/router/router.go`'s `New(...)` signature didn't match how
   `main.go` called it, and the body referenced out-of-scope variables plus
   a duplicated dead `/health` block. Fixed: signature corrected, dead code
   removed.
4. `cmd/api/main.go` called `database.RunMigrations` (plural); the exported
   function is `database.RunMigration` (singular). Fixed.
5. **Found during the restructure, not previously documented**:
   `pkg/jwt.NewManger` was a typo for `NewManager` (fixed at the source, one
   caller); `main.go` called `mysqlclient.Connect`, but the actual exported
   function is `mysqlclient.ConnectMySqlClient` (fixed the call site);
   `internal/utils` had two competing response-helper names in use across
   the codebase (`SuccessResponse`/`ErrorResponse` vs `Success`/`Error`) —
   standardized on the shorter `Success`/`Error` (the majority convention)
   and updated the 3 call sites using the old names; `golang.org/x/time/rate`
   was imported but missing from `go.sum` (`go get` + `go mod tidy` fixed
   it).
