# Bibliotheca

📚 Bibliotheca
- A Library Management & E-Library System
- Built with Go, MySQL, Redis, Next.js, NestJS, Flutter, and JavaFX

## Monorepo layout

```
Bibliotheca/
├── Server/                 # Go REST API (net/http, MySQL, Redis)
├── Client/
│   ├── web/
│   │   ├── app/            # Next.js — the product app end-users use
│   │   └── main/           # Next.js (static export) — marketing/advertising site
│   ├── admin/
│   │   ├── web/            # Next.js — admin dashboard frontend
│   │   └── api/            # NestJS — admin dashboard backend
│   ├── mobile/             # Flutter — mobile app
│   └── desktop/            # JavaFX — native desktop app
└── infra/
    ├── docker/             # Dockerfiles + docker-compose (base + dev/prod overrides)
    └── nginx/              # Reverse-proxy config
```

Every JS/TS app uses plain JavaScript (no TypeScript) and its own `package.json` (npm, no shared workspace). See [infra/README.md](infra/README.md) for how to bring the whole stack up.

## Server (Go API)

```
Server/
├── cmd/
│   ├── api/            # Entry point (main.go)
│   └── key/            # JWT secret generator CLI (keyGen.go)
│
├── internal/
│   ├── modules/         # feature packages — one dir per business domain
│   │   ├── auth/        # Register/Login/Logout/Refresh — model, dto, repository, service, handler
│   │   ├── user/        # User/UserProfile — model, dto, repository (no service/handler yet — Step 17)
│   │   ├── catalog/     # Author + Book merged (real many-to-many via book_authors — see CLAUDE.md)
│   │   ├── borrow/      # BorrowRecord — model, dto, repository (no service/handler yet — Step 16)
│   │   └── reading/     # ReadingSession/UserLibrary/Bookmark — model, dto, repository (no service/handler yet — Step 15)
│   ├── cache/          # Cache interface, Redis implementation, key/TTL helpers
│   ├── config/         # Env loading & validation
│   ├── database/       # golang-migrate runner
│   ├── errors/         # AppError, typed HTTP errors
│   ├── middleware/     # auth, rbac, ownership, cors, logger, recovery, requestID, rate_limiter, security, chain
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
- All five `Client/` apps (`web/app`, `web/main`, `admin/web`, `admin/api`, `mobile`) are bare framework scaffolds — no product code written yet. `Client/desktop` (JavaFX) is hand-written scaffolding, not generated, since Maven isn't installed in the environment it was created in — see [Client/desktop/README.md](Client/desktop/README.md).

`go build ./...` and `go vet ./...` both succeed, and the server boots and serves real requests — see `Server/docs/Steps.md` → "Build history" if you want the story of what used to be broken here and how the feature-based restructure fixed it as a side effect.

## Client

Five apps, all bare scaffolds — no product code yet:
- **`web/app`** and **`admin/web`** — Next.js, `output: "standalone"` (SSR-capable)
- **`web/main`** — Next.js, `output: "export"` (static marketing site)
- **`admin/api`** — NestJS in plain-JavaScript mode (no compiled build step; runs via `babel-node`)
- **`mobile`** — Flutter (Android + iOS targets)
- **`desktop`** — JavaFX (Maven), hand-written scaffold — see its own README for build requirements

## Infra

`infra/docker/` + `infra/nginx/` hold the full Docker Compose setup — every app above, nginx, MySQL, Redis, Umami analytics (+ its own Postgres), and (dev-only) Adminer + Mailpit, split into a shared base compose file plus `dev`/`prod` overrides. See **[infra/README.md](infra/README.md)** for setup and the exact run commands — don't guess at `docker compose` flags here, the project-directory/env-file interaction is non-obvious and documented there.

## Getting started

**Whole stack (recommended):** see [infra/README.md](infra/README.md).

**Go API only, without Docker:**
```bash
cd Server
cp .env.example .env      # fill in DB/Redis/JWT values
go run ./cmd/key           # optional: generate a JWT secret
go run ./cmd/api           # needs MySQL + Redis reachable per your .env
```

See [Server/docs/project_setup.md](Server/docs/project_setup.md) for env vars and the migration/schema layout, and [Server/docs/Steps.md](Server/docs/Steps.md) for the build roadmap.

## Docs

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — system-wide overview: every app, data stores, request flow
- [docs/API.md](docs/API.md) — where to find API docs (points at the two below)
- [Server/docs/API.md](Server/docs/API.md) — the actual endpoint reference
- [Client/docs/API.md](Client/docs/API.md) — which client app calls what, base URLs per environment
- [docs/plan.md](docs/plan.md) — the infra branch's plan, as agreed and as actually implemented
- [docs/TODO.md](docs/TODO.md) — living task list, kept current
- [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) — branching, commit, and docs conventions
