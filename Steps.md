```
✅ Step 1  — Scaffold: folder structure, go.mod, git init

✅ Step 2  — Config: .env, config.go, godotenv

✅ Step 3  — MySQL: connection, pooling, migrations (10 tables)
             users, authors, users_profile, books, borrows_records,
             refresh_tokens, book_authors, reading_sessions,
             user_library, bookmarks

✅ Step 4  — Redis: connection, cache interface, key helpers

⏳ Step 5  — Models, DTOs, Errors, Utils
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

⏳ Step 6  — Repository Layer
             One file per domain, pure DB queries, zero business logic
             user.go      → GetByID, GetByEmail, Create, Update
             author.go    → GetByID, GetAll, Create, Update, Delete
             book.go      → GetByID, GetAll, Create, Update, Delete, Search
             borrow.go    → GetByID, GetByUser, Create, Update (return)
             reading.go   → GetSession, UpsertSession, GetLibrary,
                            UpdateLibraryStatus, GetBookmarks, CreateBookmark
             token.go     → Create, GetByToken, Revoke, DeleteExpired

⏳ Step 7  — Authentication
             pkg/jwt/
               jwt.go          → GenerateAccessToken, ParseToken, ValidateClaims
             pkg/refreshtoken/
               refreshtoken.go → Generate, Hash, Verify
             internal/auth/
               service.go      → Register, Login, Logout, RefreshToken
             Routes:
               POST /api/v1/auth/register
               POST /api/v1/auth/login
               POST /api/v1/auth/logout
               POST /api/v1/auth/refresh

⏳ Step 8  — Core Middleware
             middleware/logger.go      → structured request/response logging
             middleware/cors.go        → allowed origins, headers, methods
             middleware/recovery.go    → catch panics → return clean 500
             middleware/auth.go        → validate JWT → inject user into context

⏳ Step 9  — RBAC Middleware
             middleware/rbac.go        → RoleRequired("admin", "librarian")
             Applied per route group:
               /admin/*   → admin only
               /staff/*   → admin + librarian
               /api/*     → any authenticated user

⏳ Step 10 — Rate Limiting Middleware
             middleware/rate_limiter.go
             Per-IP token bucket via golang.org/x/time/rate
             Configurable RPS + burst from config

⏳ Step 11 — Authors Module
             handlers/author.go   → CreateAuthor, GetAuthor, GetAll,
                                    UpdateAuthor, DeleteAuthor, GetBooksByAuthor
             services/author.go   → business logic + cache
             Routes:
               GET    /api/v1/authors
               GET    /api/v1/authors/:id
               GET    /api/v1/authors/:id/books
               POST   /api/v1/authors          [librarian, admin]
               PUT    /api/v1/authors/:id      [librarian, admin]
               DELETE /api/v1/authors/:id      [admin]

⏳ Step 12 — Books Module
             handlers/book.go     → CRUD + assign authors
             services/book.go     → business logic + cache
             Routes:
               GET    /api/v1/books
               GET    /api/v1/books/:id
               POST   /api/v1/books            [librarian, admin]
               PUT    /api/v1/books/:id        [librarian, admin]
               DELETE /api/v1/books/:id        [admin]
               POST   /api/v1/books/:id/authors [librarian, admin]

⏳ Step 13 — Search + Filtering + Pagination
             Full-text MySQL search across books + authors
             Filter by genre, year, format (digital/physical), author
             Redis caching of search results (TTL 2 min)
             Routes:
               GET /api/v1/search?q=&genre=&author=&year=&page=&limit=

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
             Request ID middleware (trace requests across logs)
             /health endpoint (checks DB + Redis liveness)
```