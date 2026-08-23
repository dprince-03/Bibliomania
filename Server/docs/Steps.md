> Status as of 2026-08-23: Steps 1-12 have code written for them, but the
> project does not currently build — see "Known build issues" at the bottom
> of this file. Steps 13-20 have no code yet beyond what's noted below.

```
✅ Step 1  — Scaffold: folder structure, go.mod, git init

✅ Step 2  — Config: .env, config.go, godotenv

✅ Step 3  — MySQL: connection, pooling, migrations (10 tables)
             users, authors, users_profile, books, borrows_records,
             refresh_tokens, book_authors, reading_sessions,
             user_library, bookmarks

✅ Step 4  — Redis: connection, cache interface, key helpers

✅ Step 5  — Models, DTOs, Errors, Utils
             models/
               user.go         → User, UserProfile structs
               author.go       → Author struct
               book.go         → Book, BookAuthor structs
               borrow.go       → BorrowRecord struct
               reading.go      → ReadingSession, UserLibrary, Bookmark structs
               token.go        → RefreshToken struct
             dto/
               auth.go         → RegisterRequest, LoginRequest, TokenResponse
               book.go         → CreateBookRequest, UpdateBookRequest, BookResponse
               author.go       → CreateAuthorRequest, AuthorResponse
               borrow.go       → BorrowRequest, BorrowResponse
               user.go         → UpdateProfileRequest, UserResponse
               reading.go      → UpdateProgressRequest, BookmarkRequest
             errors/
               errors.go       → AppError, typed HTTP errors (404, 401, 403, 422)
             utils/
               response.go     → JSON success/error response formatters
               pagination.go   → Page, limit, offset helpers
               file.go         → File type/size validation helpers
               hash.go         → bcrypt password hash/compare

✅ Step 6  — Repository Layer (⚠️ method names drifted from callers — see below)
             One file per domain, pure DB queries, zero business logic
             user.go         → GetByID, GetByEmail, CreateUser, UpdateUser, UpdateStatus
             user_profile.go → GetByUserID, CreateUserProfile, UpdateUserProfile, ...
             author.go       → GetAuthorByID, GetAllAuthors, CreateAuthor, UpdateAuthor, DeleteAuthor
             book.go         → GetBookByID, GetAllBooks, CreateBook, UpdateBook, DeleteBook, Search, ...
             book_author.go  → AssignAuthor, RemoveAuthor, GetAuthorsByBookID, GetBooksByAuthorID
             borrow.go       → GetByID, GetAllByUserID, GetAll, Create, MarkReturned, UpdateOverdue, HasActiveBorrow
             reading.go      → session/library/bookmark repos (interfaces only, no implementation file yet)
             token.go        → Create, GetByToken, Revoke, RevokeAllByUserID, DeleteExpired

✅ Step 7  — Authentication
             pkg/jwt/
               jwt.go          → GenerateAccessToken, ParseAccessToken
             pkg/refreshToken/
               refreshToken.go → Generate, HashToken
             internal/auth/
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
             handlers/author.go   → GetAll, GetByID, GetBooksByAuthor, Create, Update, Delete
             services/author.go   → business logic + Redis cache
             Routes:
               GET    /api/v1/authors
               GET    /api/v1/authors/:id
               GET    /api/v1/authors/:id/books
               POST   /api/v1/authors          [librarian, admin]
               PUT    /api/v1/authors/:id      [librarian, admin]
               DELETE /api/v1/authors/:id      [admin]

✅ Step 12 — Books Module (Search folded in early, see Step 13)
             handlers/book.go     → CRUD + assign/remove authors + search
             services/book.go     → business logic + Redis cache
             Routes:
               GET    /api/v1/books
               GET    /api/v1/books/:id
               GET    /api/v1/search
               POST   /api/v1/books            [librarian, admin]
               PUT    /api/v1/books/:id        [librarian, admin]
               DELETE /api/v1/books/:id        [admin]
               POST   /api/v1/books/:id/authors [librarian, admin]
               DELETE /api/v1/books/:id/authors/:authorId [librarian, admin]

⏳ Step 13 — Search + Filtering + Pagination (partially done — see Step 12)
             ✅ Basic search by query/genre/year exists in BookService.Search
             ⏳ Full-text MySQL search, author filter, and format filter not yet added
             ✅ Redis caching of search results (TTL 2 min)

⏳ Step 14 — E-Library: Upload / Download / Offline Sync
             handlers/book.go (extended)
             services/book.go  (extended)
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
             handlers/reading.go
             services/reading.go
             Routes:
               GET   /api/v1/reading/:bookId/session
               PATCH /api/v1/reading/:bookId/progress
               GET   /api/v1/reading/:bookId/bookmarks
               POST  /api/v1/reading/:bookId/bookmarks
               DELETE /api/v1/reading/:bookId/bookmarks/:id

⏳ Step 16 — Borrowing System
             handlers/borrow.go
             services/borrow.go
             Auto-detect overdue on fetch
             Decrements/increments available_copies atomically
             Routes:
               GET  /api/v1/borrows              [admin, librarian]
               GET  /api/v1/borrows/my           [member]
               POST /api/v1/borrows              [member]
               PATCH /api/v1/borrows/:id/return  [member, librarian, admin]

⏳ Step 17 — Member Management + User Library
             handlers/user.go
             services/user.go
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

## Known build issues (as of 2026-08-23)

`go build ./...` currently fails. Two things need reconciling — this reads as
an in-progress refactor, not a design problem:

1. **Import path split** — `go.mod` module is `bibliotheca`, but
   `cmd/api/main.go`, `internal/router/router.go`, `internal/services/*.go`,
   `internal/handlers/*.go`, and `internal/middleware/{rbac,ownership,rate_limiter}.go`
   still import via the old `github.com/yourusername/bibliotheca/...` path.
   Everything else (`internal/auth`, `internal/repository`, `internal/utils`,
   `internal/middleware/auth.go`) already uses the correct `bibliotheca/...` path.
2. **Repository/service method-name mismatch** — `internal/repository/repository.go`
   defines (and the concrete repos implement) `GetAuthorByID`, `GetAllAuthors`,
   `CreateAuthor`, `GetBookByID`, `GetAllBooks`, `CreateBook`, `UpdateBook`,
   `DeleteBook`, but `internal/services/author.go` / `book.go` call the shorter
   `GetByID`, `GetAll`, `Create`, `Update`, `Delete` — those don't exist on the
   interfaces.
3. `internal/router/router.go`'s `New(...)` only takes `(jwtManager, authHandler)`,
   but `main.go` calls it with 5 args; the router body also references `cfg`,
   `authorHandler`, `bookHandler` that aren't parameters, and has a duplicated
   dead `/health` block.
4. `cmd/api/main.go` calls `database.RunMigrations` (plural); the exported
   function is `database.RunMigration` (singular).
