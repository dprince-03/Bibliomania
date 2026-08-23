# Bibliotheca

📚 Bibliotheca
- A Library Management & E-Library System
- Built with Go, MySQL, Redis, and React (Vite)

## Monorepo layout

```
Bibliotheca/
├── Server/            # Go REST API (net/http, MySQL, Redis)
├── Client/            # React + Vite frontend — early scaffold, not yet built out
└── infra/docker/      # Dockerfiles + docker-compose for the stack
```

## Server (Go API)

```
Server/
├── cmd/
│   ├── api/            # Entry point (main.go)
│   └── key/            # JWT secret generator CLI (keyGen.go)
│
├── internal/
│   ├── auth/           # Register/Login/Logout/Refresh — service.go + handler.go
│   ├── cache/          # Cache interface, Redis implementation, key/TTL helpers
│   ├── config/         # Env loading & validation
│   ├── database/       # golang-migrate runner
│   ├── dto/            # Request/response structs (auth, book, author, borrow, reading, user)
│   ├── errors/         # AppError, typed HTTP errors
│   ├── handlers/       # HTTP handlers — currently: author.go, book.go
│   ├── middleware/     # auth, rbac, ownership, cors, logger, recovery, requestID, rate_limiter, security, chain
│   ├── models/         # DB structs (user, author, book, borrow, reading, token)
│   ├── repository/     # Raw DB queries, one file per domain
│   ├── services/       # Business logic + caching — currently: author.go, book.go, mapper.go
│   └── utils/          # response helpers, pagination, file validation, hashing
│
├── pkg/
│   ├── jwt/            # Access token generate/parse/validate
│   ├── mysqlclient/    # MySQL connection + pooling
│   ├── redisclient/    # Redis connection + pooling
│   └── refreshToken/   # Refresh token generate/hash/verify
│
├── migrations/         # 10 golang-migrate SQL files (see docs/project_setup.md)
├── docs/               # project_setup.md, Steps.md (build/roadmap notes)
├── .air.toml           # Hot reload (air)
└── .env.example        # Config template
```

### Implemented so far
- Config loading (`.env` via godotenv) with dev/prod validation
- MySQL connection pooling + migration runner (10 tables)
- Redis connection + cache abstraction with centralized TTL/key helpers
- JWT-based auth: register, login, logout, refresh-token rotation (hashed refresh tokens stored in DB)
- Authors module: CRUD + list books by author, Redis-cached
- Books module: CRUD, author assignment, search/filter, Redis-cached
- Middleware stack: request ID, structured logging, panic recovery, CORS, security headers, per-IP token-bucket rate limiting (strict limiter on auth routes), RBAC (`admin` / `librarian` / `member`), resource-ownership checks

### Not yet implemented
- Borrowing system, reading sessions/bookmarks, and user/member management have **models, DTOs, and repository interfaces only** — no handlers or services yet
- File upload/download for the e-library (PDF/EPUB) is not wired up
- Swagger/OpenAPI docs, Makefile, and full Docker polish are pending
- `Client/` is still the unmodified Vite + React template — no app code written yet

### Known issues (current branch does not build)
`go build ./...` currently fails. The codebase is mid-refactor and has two families of inconsistencies to resolve before it compiles:
- **Import path split**: `go.mod` declares the module as `bibliotheca`, but `cmd/api/main.go`, `internal/router/router.go`, `internal/services/*.go`, `internal/handlers/*.go`, and `internal/middleware/{rbac,ownership,rate_limiter}.go` still import via the old path `github.com/yourusername/bibliotheca/...`, while the rest of the codebase (`internal/auth`, `internal/repository`, `internal/utils`, `internal/middleware/auth.go`) uses the correct `bibliotheca/...` path.
- **Repository/service method-name mismatch**: `internal/repository/repository.go` defines interfaces with methods like `GetAuthorByID`, `GetAllAuthors`, `CreateAuthor`, `GetBookByID`, `GetAllBooks`, `CreateBook`, `UpdateBook`, `DeleteBook` (and the concrete `authorRepository`/`bookRepository` implement exactly those), but `internal/services/author.go` and `internal/services/book.go` call the shorter `GetByID`, `GetAll`, `Create`, `Update`, `Delete` — those methods don't exist on the interfaces.
- `internal/router/router.go`'s `New(...)` signature only accepts `(jwtManager, authHandler)`, but `cmd/api/main.go` calls it with 5 arguments (`cfg, jwtManager, authHandler, authorHandler, bookHandler`); the router body also references `cfg`, `authorHandler`, and `bookHandler` that aren't in scope, and has a duplicated/dead "health" route block.
- `cmd/api/main.go` calls `database.RunMigrations(...)` (plural); the actual exported function is `database.RunMigration(...)` (singular).

None of this affects understanding the design — it reads as an in-progress refactor (repository method names and import paths were changed in one place but not propagated everywhere) rather than a design problem.

## Client (React + Vite)

Default `create-vite` React template — routing, API client, and UI have not been started yet. See [Client/README.md](Client/README.md).

## Infra

`infra/docker/` holds `Dockerfile.dev`, `Dockerfile.prod`, and `docker-compose.yml` for running the API alongside MySQL and Redis.

## Getting started

```bash
cd Server
cp .env.example .env      # fill in DB/Redis/JWT values
go run ./cmd/key           # optional: generate a JWT secret
go run ./cmd/api           # once the build issues above are fixed
```

See [Server/docs/project_setup.md](Server/docs/project_setup.md) for env vars and the migration/schema layout, and [Server/docs/Steps.md](Server/docs/Steps.md) for the build roadmap.
