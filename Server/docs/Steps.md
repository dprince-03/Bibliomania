> Status as of 2026-08-24: Steps 1-12 have code written for them, and
> `go build ./...` / `go vet ./...` succeed — see "Build history" at the
> bottom of this file for how it got there. `internal/modules/` is now
> organized **by feature** (`auth/`, `user/`, `catalog/`, `borrow/`,
> `reading/`), not by layer — see `docs/ARCHITECTURE.md` for the package
> map. Steps 13-20 have no code yet beyond what's noted below.

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

⏳ Step 14 — E-Library: Upload / Download / Offline Sync
             internal/modules/catalog/book_handler.go (extended)
             internal/modules/catalog/book_service.go  (extended)
             Upload PDF/EPUB → save to storage/books/{bookID}/
             Download with auth + role check + streaming
             Offline sync endpoint:
               PATCH /api/v1/reading/:bookId/sync
               → client sends progress + client_updated_at
               → server resolves conflict (last write wins)
             Routes:
               POST  /api/v1/books/:id/upload   [librarian, admin]
               GET   /api/v1/books/:id/download  [member, librarian, admin]
               PATCH /api/v1/reading/:bookId/sync

⏳ Step 15 — Reading Sessions + Progress + Bookmarks
             internal/modules/reading/handler.go   (new)
             internal/modules/reading/service.go   (new)
             Routes:
               GET   /api/v1/reading/:bookId/session
               PATCH /api/v1/reading/:bookId/progress
               GET   /api/v1/reading/:bookId/bookmarks
               POST  /api/v1/reading/:bookId/bookmarks
               DELETE /api/v1/reading/:bookId/bookmarks/:id

⏳ Step 16 — Borrowing System
             internal/modules/borrow/handler.go   (new)
             internal/modules/borrow/service.go   (new)
             Auto-detect overdue on fetch
             Decrements/increments available_copies atomically
             Routes:
               GET  /api/v1/borrows              [admin, librarian]
               GET  /api/v1/borrows/my           [member]
               POST /api/v1/borrows              [member]
               PATCH /api/v1/borrows/:id/return  [member, librarian, admin]

⏳ Step 17 — Member Management + User Library
             internal/modules/user/handler.go   (new)
             internal/modules/user/service.go   (new)
             Routes:
               GET   /api/v1/users/me
               PATCH /api/v1/users/me
               GET   /api/v1/users/me/library
               PATCH /api/v1/users/me/library/:bookId
               GET   /api/v1/users/me/history
               GET   /api/v1/users             [admin]
               PATCH /api/v1/users/:id/status  [admin]

⏳ Step 18 — Swagger / OpenAPI Docs
             Annotate all handlers with swaggo comments
             Auto-generate with: swag init -g cmd/api/main.go
             Served at: GET /swagger/*

⏳ Step 19 — Makefile + Docker Polish
             make run          → go run cmd/api/main.go
             make build        → go build -o bibliotheca
             make migrate-up   → run all pending migrations
             make migrate-down → rollback last migration
             make docker-up    → docker compose up --build -d
             make docker-down  → docker compose down
             make swagger      → swag init
             make seed         → run seed data (dev only)
             docker-compose.prod.yml → production overrides

⏳ Step 20 — Final Polish
             Global error handler (single point for all errors)
             Input validation via go-playground/validator
             Security headers middleware (HSTS, XSS, CSP, etc.)
             Graceful shutdown (drain connections on SIGTERM)
             Request ID middleware (trace requests across logs)  ← done early, Step 8
             /health endpoint (checks DB + Redis liveness)       ← endpoint exists, no Redis/DB check yet
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
