# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Bibliotheca is a Library Management & E-Library System, now a multi-app monorepo:
- **`Server/`** — Go REST API. Module path `github.com/dprince-03/Bibliotheca` (matches the GitHub repo exactly, case included — that's what makes `go get`/module-proxy resolution work if this module is ever imported elsewhere), Go 1.25, MySQL via `sqlx`, Redis for caching, JWT for auth, `net/http` (stdlib router, no framework).
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

`Server/Makefile` (Step 19) wraps the common ones — `make run`/`build`/`vet`/`fmt`/`migrate-up`/`migrate-down`/`docker-up`/`docker-down`/`swagger`/`seed`. `migrate-up`/`migrate-down` source `Server/.env` at the shell level rather than via Make's `include`, deliberately — Make's own comment character is `#`, and this repo's `DB_PASSWORD`/`REDIS_PASSWORD` values contain a literal `#`, which `include` would silently truncate.

```bash
go build ./...
go vet ./...
go run ./cmd/api             # start the API (loads Server/.env via godotenv)
go run ./cmd/key              # generate a JWT secret
go run ./cmd/seed             # insert sample authors/books + an admin user (make seed) — skips if data already looks present
air                            # hot reload during dev (config: Server/.air.toml)

migrate create -ext sql -dir migrations -seq <name>   # new migration pair
migrate -path ./migrations -database "mysql://user:pass@tcp(localhost:3306)/bibliotheca" up

# After changing a handler's swaggo comments, regenerate internal/swaggerdocs/ (committed, see Server/docs/API.md):
swag init -g cmd/api/main.go --output internal/swaggerdocs --parseInternal --parseDependency
swag fmt -g cmd/api/main.go
```

There are no tests in the repo yet. `Server/Makefile` has `run`/`build`/`vet`/`fmt`/`migrate-up`/`migrate-down`/`docker-up`/`docker-down`/`swagger`/`seed` — no `make test` (nothing to test yet).

Each `Client/` Node app (`web/app`, `web/main`, `admin/web`, `admin/api`) has its own `package.json` and standard scripts (`npm run dev`/`build`/`lint`) but no product code beyond its framework's default template. `Client/mobile` is a standard Flutter project (`flutter run`/`flutter build`). `Client/desktop` needs Maven (`mvn javafx:run`) — not installed in every environment, don't assume it's there.

### Docker / Compose (whole stack)

Run from the **repo root**, with `--env-file .env` (not `--project-directory .` — see `infra/README.md` for why that flag breaks path resolution here):
```bash
docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml up --build   # dev
docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.prod.yml up -d --build # prod
docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml config         # validate only
```

## Architecture

`Server/internal/modules/` is organized **by feature**, not by layer: each business domain owns a package holding its own model/dto/repository/service/handler, wired together explicitly in `cmd/api/main.go` (no DI framework). Cross-cutting infra (`cache`, `config`, `database`, `errors`, `middleware`, `router`, `utils`) stays directly under `internal/`, not inside `modules/`. `internal/errors.AppError` is the one error type handlers translate to HTTP status codes; `internal/utils.Success`/`utils.Error` write the JSON envelope.

- **`internal/modules/auth`** — Register/Login/Logout/RefreshToken. Owns its own `RefreshToken` model, its DTOs, and `TokenRepository`. Refresh tokens are randomly generated, hashed before storage, and rotated (one-time use) on every refresh. Depends on `internal/modules/user` for the `User` type and its repositories.
- **`internal/modules/user`** — `User`/`UserProfile` models, their DTOs, `Repository` + `ProfileRepository`, plus profile/library/history endpoints and admin user management. Also owns `UserLibrary`/`LibraryRepository` (moved from `reading` in Step 17 — it's a member-management concept, not a reading-progress one) and depends on `catalog` (for `UserLibrary.Book`) and `reading` (reads through `SessionRepository` to power `GET /users/me/history`, a different concept from `borrow`'s checkout history). `Repository.GetAll` deliberately skips the `is_active` filter `GetByID` uses, so admins can see (and reactivate) deactivated accounts.
- **`internal/modules/catalog`** — merges **author + book** into one package (`author_*.go` / `book_*.go` files within it), deliberately not split further: `BookResponse` embeds authors and `AuthorService.GetBooksByAuthor` returns books, a genuine many-to-many via the `book_authors` junction that would otherwise force either an import cycle or duplicated response DTOs between separate `author`/`book` packages. Owns Redis caching for both: read-through on GET (check cache key → miss → query repo → populate cache), explicit invalidation on writes (`invalidate*Cache` helpers called after Create/Update/Delete). Cache keys/TTLs are centralized in `internal/cache/keys.go` — always add new cache keys there rather than inlining `fmt.Sprintf` elsewhere (existing list-cache invalidation does this inconsistently; don't copy that pattern for new code).
- **`internal/modules/borrow`** — borrowing/returns. `DecrementAvailableCopies`'s atomic `WHERE available_copies > 0` is the actual concurrency-safe reservation; `HasActiveBorrow` is a separate business-rule guard (no double-borrowing), not the safety mechanism. Overdue detection is a read-time sweep (`UpdateOverdue`, called before every list read) — there's no cron/worker in this codebase.
- **`internal/modules/reading`** — reading sessions, offline sync, and bookmarks; depends on `catalog` (book-existence checks; `Sync`/`UpdateProgress` share one `upsertProgress` helper, differing only in whose clock feeds the last-write-wins comparison). `reading_sessions.client_updated_at` is `DATETIME(6)` (see migration 000011) — plain `DATETIME`'s 1-second resolution let two same-second updates collide and silently drop a write, since the conflict SQL's `>=` favors the existing row on a tie.
- Naming convention across all of the above: no redundant prefixes once the package name carries the meaning — `catalog.AuthorService`/`BookService` (two services, need disambiguating), but `user.Repository`/`borrow.Repository` (one repo per package, no prefix needed). Entity/DTO struct names (`Author`, `Book`, `BorrowRecord`, `AuthorResponse`, ...) are unchanged from before the restructure.
- When adding a repository method, declare it on that domain's own `Repository` interface first — there's no longer a single central file listing every interface.
- **Middleware** (`internal/middleware/`) is composed once in `router.New` via `middleware.Chain(...)`, applied outermost-to-innermost in the order passed to `Chain`. Route-specific middleware (auth guard, RBAC, per-route rate limits) is applied by wrapping individual handlers when routes are registered, not through the global chain. Roles (`admin`, `librarian`, `member`) are defined once in `middleware/rbac.go` — never compare against raw role strings elsewhere. `AuthGuard` must run before `RoleRequired`/`OwnershipRequired`/`SelfOrAdmin`, since they read user ID/role/email out of request context that only `AuthGuard` populates.
- **Rate limiting** is per-IP token-bucket (`golang.org/x/time/rate`) with two separate `RateLimiterStore` instances: a loose global one and a strict one applied individually to each `/api/v1/auth/*` route to blunt brute-force attempts.
- **Pagination** goes through `utils.GetPagination(r)` → `utils.Pagination{Page, Limit, Offset}` → `utils.NewPaginatedResponse(items, total, page, limit)`; keep new list endpoints consistent with this rather than inventing ad hoc paging.
- **Config** (`internal/config`) loads `.env` via godotenv (skipped when `APP_ENV=production`) with typed getters and fallback defaults; `Config.validate()` is the only place required-field checks belong. The server port env var is `SERVER_PORT` (default `8080`) — it used to incorrectly read `PORT`, silently ignoring the documented `.env` setting; fixed, don't reintroduce that mismatch.
- **Docker images** are named `bibliotheca-<service>` (e.g. `bibliotheca-server`, `bibliotheca-web-app`), declared in `infra/docker/docker-compose.yml`'s base file. Dockerfiles live centralized under `infra/docker/<service>/Dockerfile.{dev,prod}` (nginx is the one exception, keeping its Dockerfiles in `infra/nginx/` alongside its conf files) — the base compose file declares `build.context` only; `docker-compose.dev.yml`/`docker-compose.prod.yml` each add the matching `build.dockerfile`.

### Roadmap and current state

`Server/docs/Steps.md` is the authoritative step-by-step build roadmap (Steps 1-20) and tracks what's actually implemented vs. planned — check it before assuming a module exists. `Server/docs/project_setup.md` documents `.env` vars and the migration/schema layout. As of the last audit: **all 20 steps are done** — every feature step, Swagger/OpenAPI docs at `GET /swagger/*`, the Makefile, and Step 20's final hardening (one consolidated `utils.HandleError` global error handler, and a real `/health` endpoint checking DB + Redis liveness via `internal/health`) — and `go build ./...`/`go vet ./...` succeed. The Server roadmap is complete; remaining work in the monorepo is on the `Client/` apps and infra follow-ups (see `docs/TODO.md`).

## Git workflow for this repo

Each roadmap step was implemented on its own branch — one step per branch, not bundled. Branch naming: `feat/step-<N>-<short-slug>` (e.g. `feat/step-20-final-polish`). After a step lands, update the corresponding entry in `Server/docs/Steps.md` (status marker + notes) as part of that same branch/PR. All 20 Server steps are now done; this convention still applies to any future numbered Server step, and non-roadmap work (infra, refactors) gets its own descriptively-named branch outside this numbering, e.g. `feat/infra-docker-stack`, `refactor/server-feature-structure`.
