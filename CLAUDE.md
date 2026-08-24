# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Bibliotheca is a Library Management & E-Library System, now a multi-app monorepo:
- **`Server/`** — Go REST API. Module name `bibliotheca`, Go 1.25, MySQL via `sqlx`, Redis for caching, JWT for auth, `net/http` (stdlib router, no framework).
- **`Client/web/app`** — Next.js, the product app end-users browse (`output: "standalone"`).
- **`Client/web/main`** — Next.js, the marketing/advertising site (`output: "export"`, static).
- **`Client/admin/web`** — Next.js, admin dashboard frontend (`output: "standalone"`).
- **`Client/admin/api`** — NestJS, admin dashboard backend, scaffolded in **plain-JavaScript mode** (`nest new --language javascript`) — no compiled build step, runs via `babel-node` even in prod.
- **`Client/mobile`** — Flutter (Android + iOS).
- **`Client/desktop`** — JavaFX (Maven), hand-written scaffold (Maven wasn't installed when this was created, so it wasn't generated via `mvn archetype:generate` — verify it still builds before trusting the pom.xml blindly).
- **`infra/docker/`** + **`infra/nginx/`** — Docker Compose orchestration (base + dev/prod overrides) for the whole stack, plus MySQL, Redis, Umami analytics (+ its own Postgres), nginx, and dev-only Adminer/Mailpit. See `infra/README.md`.
- **`.github/workflows/`** — separate `<app>-ci.yml` + `<app>-cd.yml` per app, path-filtered; CD (push to `main` only) builds+pushes to `ghcr.io/dprince-03/bibliotheca-<service>`. See `infra/README.md`'s "CI/CD" section.

**Every JS/TS app is plain JavaScript, no TypeScript**, each with its own independent `package.json` (npm, no shared workspace) — this was an explicit, deliberate choice; don't introduce TypeScript or a monorepo tool (pnpm workspaces, etc.) without checking first.

## Commands (all run from `Server/`)

```bash
go build ./...              # currently fails — see "Current build issues" below
go vet ./...
go run ./cmd/api             # start the API (loads Server/.env via godotenv)
go run ./cmd/key              # generate a JWT secret
air                            # hot reload during dev (config: Server/.air.toml)

migrate create -ext sql -dir migrations -seq <name>   # new migration pair
migrate -path ./migrations -database "mysql://user:pass@tcp(localhost:3306)/bibliotheca" up
```

There are no tests in the repo yet and no Makefile — don't assume `make test`/`make run` work.

Each `Client/` Node app (`web/app`, `web/main`, `admin/web`, `admin/api`) has its own `package.json` and standard scripts (`npm run dev`/`build`/`lint`) but no product code beyond its framework's default template. `Client/mobile` is a standard Flutter project (`flutter run`/`flutter build`). `Client/desktop` needs Maven (`mvn javafx:run`) — not installed in every environment, don't assume it's there.

### Docker / Compose (whole stack)

Run from the **repo root**, with `--env-file .env` (not `--project-directory .` — see `infra/README.md` for why that flag breaks path resolution here):
```bash
docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml up --build   # dev
docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.prod.yml up -d --build # prod
docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml config         # validate only
```

## Architecture

Layered, one direction of dependency: `router` → `middleware chain` → `handlers` → `services` → `repository` → `models`, wired together explicitly in `cmd/api/main.go` (no DI framework). `dto` structs shape request/response bodies; `internal/errors.AppError` is the one error type handlers translate to HTTP status codes.

- **`internal/auth`** is a self-contained vertical slice (service.go + handler.go) rather than living split across `handlers/`/`services/` like the other domains — Register/Login/Logout/RefreshToken. Refresh tokens are randomly generated, hashed before storage, and rotated (one-time use) on every refresh.
- **`internal/services/*`** hold business logic and own Redis caching: read-through on GET (check cache key → miss → query repo → populate cache), explicit invalidation on writes (`invalidate*Cache` helpers called after Create/Update/Delete). Cache keys/TTLs are centralized in `internal/cache/keys.go` — always add new cache keys there rather than inlining `fmt.Sprintf` elsewhere (existing list-cache invalidation does this inconsistently; don't copy that pattern for new code).
- **`internal/repository/repository.go`** is the single file declaring every repository interface (`UserRepository`, `AuthorRepository`, `BookRepository`, `BorrowRepository`, `ReadingSessionRepository`, `UserLibraryRepository`, `BookmarkRepository`, `TokenRepository`, ...); each domain then gets its own implementation file (`author.go`, `book.go`, `book_author.go`, etc.) with a `sqlx`-backed struct. When adding a repository method, declare it on the interface here first.
- **Middleware** (`internal/middleware/`) is composed once in `router.New` via `middleware.Chain(...)`, applied outermost-to-innermost in the order passed to `Chain`. Route-specific middleware (auth guard, RBAC, per-route rate limits) is applied by wrapping individual handlers when routes are registered, not through the global chain. Roles (`admin`, `librarian`, `member`) are defined once in `middleware/rbac.go` — never compare against raw role strings elsewhere. `AuthGuard` must run before `RoleRequired`/`OwnershipRequired`/`SelfOrAdmin`, since they read user ID/role/email out of request context that only `AuthGuard` populates.
- **Rate limiting** is per-IP token-bucket (`golang.org/x/time/rate`) with two separate `RateLimiterStore` instances: a loose global one and a strict one applied individually to each `/api/v1/auth/*` route to blunt brute-force attempts.
- **Pagination** goes through `utils.GetPagination(r)` → `utils.Pagination{Page, Limit, Offset}` → `utils.NewPaginatedResponse(items, total, page, limit)`; keep new list endpoints consistent with this rather than inventing ad hoc paging.
- **Config** (`internal/config`) loads `.env` via godotenv (skipped when `APP_ENV=production`) with typed getters and fallback defaults; `Config.validate()` is the only place required-field checks belong. The server port env var is `SERVER_PORT` (default `8080`) — it used to incorrectly read `PORT`, silently ignoring the documented `.env` setting; fixed, don't reintroduce that mismatch.
- **Docker images** are named `bibliotheca-<service>` (e.g. `bibliotheca-server`, `bibliotheca-web-app`), declared in `infra/docker/docker-compose.yml`'s base file. Dockerfiles live centralized under `infra/docker/<service>/Dockerfile.{dev,prod}` (nginx is the one exception, keeping its Dockerfiles in `infra/nginx/` alongside its conf files) — the base compose file declares `build.context` only; `docker-compose.dev.yml`/`docker-compose.prod.yml` each add the matching `build.dockerfile`.

### Roadmap and current state

`Server/docs/Steps.md` is the authoritative step-by-step build roadmap (Steps 1-20) and tracks what's actually implemented vs. planned — check it before assuming a module exists. `Server/docs/project_setup.md` documents `.env` vars and the migration/schema layout. As of the last audit: Steps 1-12 have code, Steps 13-20 do not (borrow, reading-session, and user/member modules currently have only models/DTOs/repository interfaces — no handlers or services).

### Current build issues

`go build ./...` fails right now from an in-progress refactor, not a design problem — details are in `Server/docs/Steps.md` under "Known build issues":
1. Import path split: some files still import `github.com/yourusername/bibliotheca/...`, others correctly use `bibliotheca/...` (the real module path). Every new/edited file should use `bibliotheca/...`.
2. `internal/services/author.go` and `book.go` call repository methods (`GetByID`, `GetAll`, `Create`, `Update`, `Delete`) that don't match the interface method names actually declared in `internal/repository/repository.go` and implemented in `author.go`/`book.go` (`GetAuthorByID`, `GetAllAuthors`, `CreateAuthor`, ... / `GetBookByID`, `GetAllBooks`, `CreateBook`, `UpdateBook`, `DeleteBook`).
3. `router.New(...)`'s signature and `main.go`'s call site disagree, and the router body references out-of-scope variables plus a duplicated dead `/health` block.
4. `main.go` calls `database.RunMigrations` (plural); the exported function is `database.RunMigration` (singular).

## Git workflow for this repo

Each roadmap step (currently Steps 13-20) is implemented on its own branch — one step per branch, not bundled. Branch naming: `feat/step-<N>-<short-slug>` (e.g. `feat/step-13-search-filtering`). After a step lands, update the corresponding entry in `Server/docs/Steps.md` (status marker + notes) as part of that same branch/PR. Non-roadmap work (infra, refactors) gets its own descriptively-named branch outside this numbering, e.g. `feat/infra-docker-stack`.

**Planned follow-up, not yet started**: restructuring `Server/internal/{handlers,services,repository,models,dto}/<domain>.go` (grouped by layer) into `Server/internal/<domain>/{handler.go,service.go,repository.go,model.go,dto.go}` (grouped by feature) — deliberately kept out of the infra branch and out of the Steps 13-20 work to avoid tangling an architecture change with unrelated diffs. If you're picking this up, do it as its own branch (e.g. `refactor/server-feature-structure`), probably after the existing build-break fix lands (restructuring code that doesn't compile yet is harder to verify).
