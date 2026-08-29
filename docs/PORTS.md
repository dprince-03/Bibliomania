# Port assignments & conflict check

Bibliomania's dev host ports are defined in [`infra/README.md`](../infra/README.md)
(the source of truth for the actual `*_HOST_PORT`/`DB_PORT`/`REDIS_PORT` env
vars) and root `.env.example`. This file is a periodic audit log — a record
of checking that assignment against every *other* project's
containers/processes running on this development machine, since this
machine runs several unrelated Docker stacks at once. Re-run the check
below whenever a new project starts claiming ports, or before assuming the
current range is still safe — don't just trust the last recorded result.

## Current assignment (mirrors `infra/README.md`)

| Service | Env var | Port |
|---|---|---|
| nginx | `NGINX_HOST_PORT` | 9080 |
| Go API (`app`) | `SERVER_HOST_PORT` | 9081 |
| web-main | `WEB_MAIN_HOST_PORT` | 9082 |
| web-app | `WEB_APP_HOST_PORT` | 9083 |
| admin-web | `ADMIN_WEB_HOST_PORT` | 9084 |
| admin (NestJS) | `ADMIN_API_HOST_PORT` | 9085 |
| MySQL | `DB_PORT` | 9086 |
| Redis | `REDIS_PORT` | 9087 |
| Adminer (dev only) | `ADMINER_HOST_PORT` | 9088 |
| Mailpit web UI (dev only) | `MAILPIT_WEB_PORT` | 9089 |
| Mailpit SMTP (dev only) | `MAILPIT_SMTP_PORT` | 9090 |

If any of these ever need to change, update `.env.example` and
`infra/README.md`'s table together — this file only tracks verification,
it isn't itself a config source.

## How to check for conflicts

```bash
# Every port currently listening on this machine, Docker or bare process:
{ docker ps --format '{{.Ports}}' | grep -oE '0\.0\.0\.0:[0-9]+' | grep -oE '[0-9]+$'; \
  ss -ltn | awk 'NR>1{print $4}' | grep -oE '[0-9]+$'; } | sort -n | uniq
```

Cross-reference the output against the table above. Any overlap means a
port needs reassigning in both `.env.example` and `infra/README.md`.

## Verification log

### 2026-08-29 — no conflicts

Checked against every other project running on this machine at the time:
`aens-*` (nginx, client, admin, worker, api, umami + its Postgres, redis),
`newshub-*` (client, server, mysql), `nexus_*` (client, umami + its
Postgres, api, worker, redis), `lms-*` (api, mysql, redis),
`so-good-catering-dev-*` (api, client, worker, postgres, redis), and a
couple of unnamed `docker-*` compose projects (server, client, postgres) —
plus every bare (non-Docker) host process listening at the time.

**Result: the entire 9080-9090 range was free.** Nearest neighbors were
`8080`/`8081` (newshub, so-good-catering) and `9100` (unrelated, not
Docker) — a clear gap on either side of Bibliomania's range.

Noted but out of scope for this check: bare (non-Docker) processes were
found listening on `3000`, `3004`, and `3005` — not part of any
Bibliomania compose service, possibly stray leftover dev-server processes
from earlier work. Worth a manual look if `npm run dev` for another
project unexpectedly fails to bind.
