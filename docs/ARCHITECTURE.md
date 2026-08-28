# Architecture

High-level map of the system. For implementation detail on any one piece,
follow the links rather than duplicating it here.

## Apps

| App | Path | Tech | Role |
|---|---|---|---|
| app | `Server/app/` | Go, `net/http`, MySQL, Redis | The one API every client talks to. See [`CLAUDE.md`](../CLAUDE.md) for its internal architecture. |
| web/app | `Client/web/app/` | Next.js (JS) | The product — the library app end-users browse/borrow/read from. |
| web/main | `Client/web/main/` | Next.js (JS), static export | Marketing/advertising site. No server-side logic — pure static HTML/CSS/JS. |
| admin | `Client/admin/` | Next.js (JS) | Admin dashboard UI. |
| admin (backend) | `Server/admin/` | NestJS (JS) | Admin-only backend — separate from the Go app; talks to the same MySQL database. Lives under `Server/`, not `Client/`, because it's a real backend — see `docs/plan.md`. |
| mobile | `Client/mobile/` | Flutter | Mobile app, talks to the Go app directly. |
| desktop | `Client/desktop/` | JavaFX | Native desktop app, talks to the Go app directly. Not containerized — see [`infra/README.md`](../infra/README.md). |

All JS/TS apps are plain JavaScript (no TypeScript), each with an independent
`package.json` — no shared workspace/monorepo tool.

## Why a separate admin backend instead of extending the Go app

The admin dashboard has its own NestJS backend (`Server/admin/`) rather than
new endpoints on the Go app (`Server/app/`). This means two backends share
one MySQL database — the admin backend should treat the schema
`Server/app/migrations/` owns as read-mostly where possible, and any schema
change needs to work for both. There's no formal contract enforcing this yet
(see `docs/TODO.md`); be deliberate about what the admin backend writes
directly versus what it should instead call the Go app's API for.

## Data stores

- **MySQL** (`mysql` container) — the application's primary database.
  Migrations live in `Server/app/migrations/`, run automatically by the Go
  app on boot (`database.RunMigration`), not by a MySQL init-script mount.
- **Redis** (`redis` container) — caching only (see `Server/app/internal/cache/`),
  not a source of truth. Nothing should read from Redis as its only copy of
  data.
- **Umami's Postgres** (`umami-postgres` container) — belongs entirely to
  Umami analytics. Completely separate from the app's MySQL — different
  engine, different credentials, different volume. Never point Umami at the
  app's MySQL instance.

## Network / request flow (dev)

```
                          ┌──────────┐
   browser/app ────────▶  │  nginx   │  (subdomain routing, see infra/README.md)
                          └────┬─────┘
              ┌────────────────┼────────────────┬──────────────┬───────────┐
              ▼                ▼                 ▼              ▼           ▼
         web-main          web-app          admin-web        admin         app
      (static export)   (Next.js SSR)     (Next.js SSR)    (NestJS)     (Go API)
                                                 │               │           │
                                                 └───────────────┴─────┬─────┘
                                                                       ▼
                                                                    mysql
                                                                       │
                                                                    redis
                                                                  (cache only)

   mobile / desktop ──────────────────────────────────────────────▶  app
   (bypass nginx entirely — no browser, hit the API directly)
```

`mobile` and `desktop` are native apps, not browser clients — they call the
`app` service directly rather than going through nginx's subdomain routing
(which exists for browser-based hostname resolution).

## Deployment topology

Dev and prod are the same service graph, different Docker Compose overrides
(`docker-compose.dev.yml` vs `docker-compose.prod.yml` on top of the shared
`docker-compose.yml` base) — see `infra/README.md` for the full breakdown of
what differs (bind mounts vs built images, dev-only Adminer/Mailpit, host
port publishing). There's no dedicated prod host set up yet; `docker-compose.prod.yml`
assumes one will exist, not this development machine.

## What's not built yet

Every Client app above is a bare framework scaffold with no product code.
See `docs/TODO.md` for the current punch list.

## Planned: platform-vision repositioning

A separate, larger change is planned but not started: multi-branch library
tenancy, a payments/billing module (subscriptions + one-time purchases +
library licensing), curation, and several AI-assisted features (moderation,
translation, audiobooks, a reading companion). This will add new Server
modules and reshape parts of the data model described above (e.g. physical
book copies moving from a global pool to per-branch ownership). See
[`docs/plan.md`](plan.md) → "Platform vision" for the full context and
[`Server/app/docs/plan.md`](../Server/app/docs/plan.md) for the technical plan.
