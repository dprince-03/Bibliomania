# Bibliotheca infra

Docker Compose orchestration for the whole stack: the Go API, three web
frontends, the admin backend, MySQL, Redis, Umami analytics, nginx, and
(dev-only) Adminer + Mailpit.

## Layout

```
infra/
├── docker/
│   ├── docker-compose.yml       # base: networks, volumes, every service's static shape
│   ├── docker-compose.dev.yml   # dev override: hot reload, bind mounts, dev-only tooling
│   ├── docker-compose.prod.yml  # prod override: built images only, no bind mounts
│   ├── server/Dockerfile.{dev,prod}
│   ├── web-app/Dockerfile.{dev,prod}
│   ├── web-main/Dockerfile.{dev,prod}
│   ├── admin-web/Dockerfile.{dev,prod}
│   └── admin-api/Dockerfile.{dev,prod}
└── nginx/
    ├── Dockerfile.{dev,prod}
    └── conf.d/default.{dev,prod}.conf
```

No Dockerfile/compose service exists for `Client/mobile` (Flutter) or
`Client/desktop` (JavaFX) — both are locally-installed native apps, not
long-lived services. CI build/packaging images for either are future work.

## Setup

1. Copy env files:
   ```bash
   cp .env.example .env               # repo root — compose-level vars
   cp Server/.env.example Server/.env # app-level vars (JWT secret, timeouts, etc.)
   ```
2. **`DB_NAME`/`DB_USER`/`DB_PASSWORD` in the root `.env` are the source of
   truth** — `docker-compose.yml` overrides `server`'s environment with these
   same values so it can never disagree with what `mysql` was initialized
   with. You don't need to hand-sync `Server/.env`'s copies of those three
   keys for Docker use; they only matter when running the API outside Docker.
3. Add these to `/etc/hosts` (dev only — the subdomain routing needs them):
   ```
   127.0.0.1 bibliotheca.local app.bibliotheca.local admin.bibliotheca.local api.bibliotheca.local analytics.bibliotheca.local adminer.bibliotheca.local mail.bibliotheca.local
   ```

## Run

All commands run from the **repo root**. Use `--env-file .env`, not
`--project-directory .` — the latter also changes how build contexts and
`env_file:` paths inside the compose files resolve (they're written relative
to `infra/docker/`, the compose files' own directory) and breaks them.

```bash
# Dev
docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml up --build

# Prod
docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.prod.yml up -d --build

# Validate config without starting anything (catches path/env mistakes early)
docker compose --env-file .env -f infra/docker/docker-compose.yml -f infra/docker/docker-compose.dev.yml config
```

## Routes (dev)

| Subdomain | Service | Container port |
|---|---|---|
| `bibliotheca.local` | web-main (marketing) | 3000 (dev) / 80 (prod, static via nginx-in-container) |
| `app.bibliotheca.local` | web-app (product app) | 3000 |
| `admin.bibliotheca.local` | admin-web | 3000 |
| `admin.bibliotheca.local/api/` | admin-api | 4000 |
| `api.bibliotheca.local` | server (Go API) | 8080 |
| `analytics.bibliotheca.local` | umami | 3000 |
| `adminer.bibliotheca.local` (dev only) | adminer | 8080 |
| `mail.bibliotheca.local` (dev only) | mailpit | 8025 |

Dev also publishes host ports for local tooling, in the `9080-9090` range —
deliberately not the usual `3000`/`5000`/`8080`/`6379`/etc. defaults, because
this machine runs several other projects' Docker stacks that already claim
those (checked via `docker ps` when these were picked; override via `.env`
if a future project collides with one of these instead):

| Service | Host port (env var, default) |
|---|---|
| nginx | `NGINX_HOST_PORT`, 9080 |
| server | `SERVER_HOST_PORT`, 9081 |
| web-main | `WEB_MAIN_HOST_PORT`, 9082 |
| web-app | `WEB_APP_HOST_PORT`, 9083 |
| admin-web | `ADMIN_WEB_HOST_PORT`, 9084 |
| admin-api | `ADMIN_API_HOST_PORT`, 9085 |
| mysql | `DB_PORT`, 9086 |
| redis | `REDIS_PORT`, 9087 |
| adminer | `ADMINER_HOST_PORT`, 9088 |
| mailpit (web UI) | `MAILPIT_WEB_PORT`, 9089 |
| mailpit (SMTP) | `MAILPIT_SMTP_PORT`, 9090 |

These are only the *host*-side ports — container-to-container traffic (e.g.
nginx → `server:8080`) uses the container's internal port regardless, so
changing a host port here never requires touching nginx config or any
service's internal `PORT`/`SERVER_PORT` env var. Prod publishes only nginx's
port 80 (and reserves 443 for the TLS follow-up) — everything else is
reachable solely inside `bibliotheca_network`, so prod has no equivalent
conflict risk (assuming a dedicated deployment host).

## Two Postgres/MySQL instances — don't mix them up

`umami` needs its **own** Postgres (`umami-postgres`), completely separate
from the app's MySQL (`mysql`). They use different engines, different
credentials (`UMAMI_POSTGRES_*` vs `DB_*`), and different volumes
(`umami_postgres_data` vs `mysql_data`). Never point Umami at the app's
MySQL — it can't use it anyway (wrong engine), but the naming is close enough
to warrant the callout.

## CI/CD

`.github/workflows/` has a separate `<app>-ci.yml` + `<app>-cd.yml` pair per app (server, web-app, web-main, admin-web, admin-api, nginx, mobile, desktop — 16 files total), each path-filtered so only the changed app's workflows run:
- **`*-ci.yml`** (every push/PR touching that app): lint/build/test using the app's own tooling (`go vet`/`build`/`test`, `npm run lint`/`build`, `flutter analyze`/`test`, `mvn package`).
- **`*-cd.yml`** (push to `main` only, not PRs, and not gated on the CI workflow passing — they run independently): the 6 Docker-image services build their prod Dockerfile and push to `ghcr.io/dprince-03/bibliotheca-<service>:latest` + `:<sha>`. `mobile-cd.yml`/`desktop-cd.yml` instead upload a build artifact (unsigned release APK / packaged jar) — there's no app-store or code-signing pipeline set up, so treat those as downloadable builds, not real releases.

`.github/dependabot.yml` covers every ecosystem in the repo (gomod, npm ×4, pub, maven, docker, github-actions) on a weekly schedule.

## Known caveats

- The Go API's build is currently broken for unrelated reasons (tracked in
  `Server/docs/Steps.md` under "Known build issues" — an import-path split
  and a repository/service method-name mismatch). The `server` container's
  Docker setup here is correct, but it won't actually boot until that's fixed.
- `admin-api` was scaffolded by `nest new --language javascript`, which has no
  compiled build step — it runs via `babel-node` even in the "prod" image, so
  `Dockerfile.prod` keeps devDependencies instead of `npm ci --omit=dev`. Once
  there's real code in this API, add a proper Babel build step
  (`babel index.js src --out-dir dist`) and switch to a `node dist/main.js`
  multi-stage build.
- TLS/HTTPS for prod nginx (certbot or similar) is not set up — follow-up work.
- Migrations run via `cmd/api/main.go`'s `database.RunMigration` call on
  server boot, not via a MySQL `docker-entrypoint-initdb.d` mount (an earlier
  version of this compose file did that, which is wrong: MySQL's init
  entrypoint runs every `*.sql` file alphabetically once, so golang-migrate's
  paired `..._up.sql`/`..._down.sql` files would run back-to-back and undo
  each other).
