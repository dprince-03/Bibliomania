# Infra branch plan (`feat/infra-docker-stack`)

This is the plan agreed on for the infra branch, kept for reference. It was
approved before implementation started, then adjusted twice mid-implementation
per follow-up instructions — see "Deviations from the original plan" at the
bottom for what actually changed and why.

## Context

Bibliotheca is expanding from "Go API + one React scaffold" into a real monorepo: a product web app, a marketing site, an admin dashboard (its own frontend + backend), and a mobile app, all fronted by nginx and backed by MySQL/Redis/Umami. The user had already carved out the target directory skeleton (`Client/web/{app,main}`, `Client/admin`, `Client/mobile`, `infra/nginx`) but left them empty except for a leftover Vite scaffold in `Client/web/main`. The existing `infra/docker/` compose setup was written back when the Go server was the only service and had since bit-rotted (paths assumed `Server/` as CWD, a corrupted env line, apk-on-Debian mismatch) — this branch both scaffolds the new apps and rebuilds the Docker orchestration to actually run everything together, in dev and prod, with a consistent `bibliotheca-<service>` image naming scheme.

CI pipelines were originally scoped out (later brought back in — see deviations). The Go server's existing build-break (import path / method-name mismatches, tracked in `Server/docs/Steps.md`) is a separate concern — this branch's Docker setup is correct, but the `server` container won't actually boot until that's fixed elsewhere.

**Language decision across every new JS/TS app: plain JavaScript, no TypeScript** — applies to `web/app`, `web/main`, `admin/web`, and `admin/api` alike.

**Desktop app**: Java (JavaFX), decided after an Electron vs. JavaFX comparison (Electron reuses the React UI/team skills across web+admin+desktop; JavaFX has no code/skill overlap with the rest of the stack but was the user's preference).

**Server restructure ("Structure by Feature")**: moving `Server/internal/{handlers,services,repository,models,dto}/<domain>.go` (grouped by layer) to `Server/internal/<domain>/{handler.go,service.go,repository.go,model.go,dto.go}` (grouped by feature) was raised during planning and deliberately deferred to its own follow-up branch, to avoid tangling an architecture change with this branch's unrelated diffs and with the still-open build-break fix. **Confirmed**: its own branch (`refactor/server-feature-structure`), immediately after this one merges — see `docs/TODO.md`.

## Branch

`feat/infra-docker-stack`

## 1. Scaffold the client apps

| App | Command (run inside target dir) | Notes |
|---|---|---|
| `Client/web/main` | delete leftover Vite files first, then `npx create-next-app@latest . --javascript --tailwind --eslint --app --src-dir --import-alias "@/*" --use-npm` | Marketing site. `output: 'export'` (static export). |
| `Client/web/app` | *(originally)* `npm create vite@latest . -- --template react` | Product app. **Changed mid-implementation to Next.js** — see deviations. |
| `Client/admin/web` | `npx create-next-app@latest . --javascript --tailwind --eslint --app --src-dir --import-alias "@/*" --use-npm` | Admin dashboard frontend. `output: 'standalone'` (SSR-capable). |
| `Client/admin/api` | `npx @nestjs/cli new . --package-manager npm --skip-git --language javascript` | Admin backend, forced to plain JavaScript. |
| `Client/mobile` | `flutter create --org com.bibliotheca --platforms=android,ios .` | Mobile app. |
| `Client/desktop` | Hand-written (Maven not installed locally) — `pom.xml` + `App.java`/`Launcher.java` matching the standard `javafx-archetype-simple` layout | Native desktop app, Java 21 LTS. |

Each app keeps its own `package.json`/`package-lock.json` (npm, no workspaces).

## 2. Dockerfiles — centralized under `infra/docker/<service>/`

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

## 3. Compose split — base + dev/prod overrides

Base `docker-compose.yml`: networks, volumes, every service's static shape, images named `bibliotheca-<service>`. Fixed bugs: `context`/volume paths that assumed `Server/` as CWD, a corrupted `MYSQL_ROOT_PASSWORD` env line, and a migrations-into-`docker-entrypoint-initdb.d` mount that would have undone itself (MySQL runs `*.sql` files alphabetically once — golang-migrate's paired `up`/`down` files would run back-to-back).

`docker-compose.dev.yml`: dev Dockerfiles, bind mounts + node_modules volumes, dev-only `adminer` + `mailpit`.

`docker-compose.prod.yml`: prod Dockerfiles, no bind mounts, no dev-only services, `restart: always`.

Run from the repo root with `--env-file .env` (not `--project-directory .` — that flag also changes how the compose files' relative paths resolve and breaks them, a real behavior discovered during implementation).

## 4. nginx — subdomain routing

`bibliotheca.local`, `app.bibliotheca.local`, `admin.bibliotheca.local` (+`/api/`), `api.bibliotheca.local`, `analytics.bibliotheca.local`, plus dev-only `adminer.bibliotheca.local` / `mail.bibliotheca.local`. Uses Docker's embedded DNS resolver (`resolver 127.0.0.11`) with `set $upstream ...; proxy_pass $upstream;` for lazy per-request hostname resolution — found during implementation that nginx otherwise resolves upstreams once at startup and refuses to boot if one isn't resolvable yet.

## 5. Env files

Root `.env.example` (compose-level: DB creds, Umami Postgres creds, Resend placeholder) + `Server/.env.example` (app-level, unchanged shape, added SMTP/Resend placeholders). The `server` container's DB_* environment is overridden from the root `.env`'s values (not left to `Server/.env`), so `mysql`'s init credentials and the app's connection credentials can never drift out of sync — found during implementation that they otherwise silently could.

## 6. Docs updates

Root `README.md`, `CLAUDE.md`, new `infra/README.md` — layout, setup, run commands, routes, known caveats.

## Verification

`docker compose ... config` (both dev and prod) to validate merged YAML/env substitution; individual `docker build` runs per service to catch real Dockerfile issues; a smoke test of `web-main`'s static export actually serving; confirming `server`'s build fails at exactly the pre-existing, already-documented `go build` bug and nowhere else.

---

## Deviations from the original plan

These happened after the plan above was approved, in response to follow-up instructions mid-implementation:

1. **`Client/web/app` changed from Vite+React to Next.js.** After scaffolding it as Vite+React per the approved plan, the user said "every website is in next.js" — re-scaffolded as Next.js (`output: 'standalone'`), matching `admin/web`. This shifted its dev/prod Dockerfiles and dev host port to the same shape as the other two Next.js apps instead of a Vite-specific pattern.
2. **`.github/` (CI/CD) added — not in the original plan's scope.** Added per an explicit follow-up ask: per-app CI (`<app>-ci.yml`, lint/build/test) and CD (`<app>-cd.yml`, push to `ghcr.io/dprince-03/bibliotheca-<service>` on merge to `main`; `mobile`/`desktop` upload build artifacts instead) for all 8 apps/services, a PR template, issue templates, and `dependabot.yml` covering every ecosystem in the repo.
3. **Dev host ports moved off their original defaults.** The plan's dev port choices (8080, 3000-3002, 4000, 3306, 6379, 80, 8081) turned out to collide with several other projects' Docker stacks already running on this machine (checked via `docker ps`). Reassigned every dev host-port to the `9080-9090` range via dedicated `*_HOST_PORT` env vars, decoupled from the containers' internal ports (which are unaffected and unchanged).
4. **Container naming was already correct** — a follow-up question asked "is the container named after the project," and checking confirmed every service already used `bibliotheca-<service>` container names plus a `name: bibliotheca` compose project name; no change was needed.
