# API Reference (frontend view)

`web/app`'s own build plan is [`web-app-plan.md`](web-app-plan.md) +
[`web-app-Steps.md`](web-app-Steps.md) — auth strategy, API base URLs, and
the step-by-step roadmap for actually consuming what's described below.

The endpoint list, request/response shapes, and auth model live in the
generated Swagger UI (`http://localhost:9081/swagger/index.html` — the gateway — when the
server is running — see `Server/app/docs/API.md`) — don't duplicate them here;
this file only covers what's specific to consuming that API *from* the
apps in this `Client/` directory. `web/main` has real API integration
(client-side book/author counts on its home page) — every other app below
is still a bare framework scaffold with no API integration code yet, so
this file is here to make sure work starts against the right base URL from
day one.

## Which app calls what

| App | Talks to | How |
|---|---|---|
| `web/app` | the gateway (`Server/app`, `cmd/gateway`) | REST `/api/v1` today (server-side, via `API_INTERNAL_URL=http://gateway:8080` in Docker). The gateway also serves `POST /graphql`, built for this app — moving `web/app`'s data layer onto it is open work (`docs/TODO.md`). |
| `web/main` | the gateway | Directly, client-side only — a small public-endpoint fetch for live book/author counts on the home page (`GET /api/v1/books`, `GET /api/v1/authors`), nothing else. |
| `admin` | `Server/admin`, not the gateway directly | The admin dashboard's own backend — see `docs/ARCHITECTURE.md` → "Why a separate admin backend". |
| `Server/admin` | the gateway's REST API (`GATEWAY_URL`) | Since the microservices split there is no shared application database to connect to — each service owns its own. |
| `mobile` | the gateway | REST, same as `web/app`. |
| `desktop` | the gateway | REST, same as `web/app`. |

Clients only ever talk to the **gateway**: the seven services behind it
aren't reachable from outside the Compose network. Every pre-split REST path
still works unchanged (the gateway proxies each to the service that owns
it); the handful of deliberate behaviour changes are listed in
`Server/app/docs/API.md` → "Behaviour changes from the split".

## Base URLs

Values come from `infra/docker/.env` (see `infra/docker/.env.example` and
`infra/README.md` → "Routes (dev)"). Never hardcode these in app code — read from an env var
per app's own framework convention (e.g. `NEXT_PUBLIC_API_URL` for the
Next.js apps).

| Environment | Gateway base URL |
|---|---|
| Dev (via `infra/docker/docker-compose.dev.yml` + nginx) | `http://api.bibliomania.local` |
| Dev (gateway's published port) | `http://localhost:9081` (`$SERVER_HOST_PORT` — see `infra/README.md`) |
| Dev, inside the Compose network (server-side code) | `http://gateway:8080` |
| Prod | Not yet deployed anywhere — no real domain assigned yet. |

`admin`'s equivalent (its own backend, `Server/admin`, not the gateway) is
`http://admin.bibliomania.local/api` in dev — see `ADMIN_WEB_API_URL` in
`infra/docker/.env.example`.

## Auth

Every non-public API route (REST or GraphQL) expects `Authorization: Bearer <access_token>`
(see `Server/app/docs/API.md` → "Conventions"). Access tokens expire in 15 minutes by
default (`JWT_ACCESS_TOKEN_TTL`) — any client integration needs to handle
refresh via `POST /auth/refresh` before that, not just react to a `401`.
