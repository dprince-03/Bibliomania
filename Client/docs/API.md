# API Reference (frontend view)

The endpoint list, request/response shapes, and auth model live in the
generated Swagger UI (`http://localhost:8080/swagger/index.html` when the
server is running — see `Server/docs/API.md`) — don't duplicate them here;
this file only covers what's specific to consuming that API *from* the
apps in this `Client/` directory. None of the apps below have any API
integration code yet (all are bare framework scaffolds) — this is here so
that work starts against the right base URL from day one.

## Which app calls what

| App | Talks to | How |
|---|---|---|
| `web/app` | Server (Go API) | Directly — the product app is the primary consumer of every endpoint in `Server/docs/API.md`. |
| `web/main` | Nothing | Static marketing site, no API calls. |
| `admin/web` | `admin/api`, not Server directly | The admin dashboard's own backend fronts Server's data — see `docs/ARCHITECTURE.md` → "Why a separate admin/api instead of extending Server". |
| `admin/api` | Server's MySQL database directly, and potentially Server's API for some operations | Not yet decided which — see `docs/ARCHITECTURE.md` for the caveat about two backends sharing one database. |
| `mobile` | Server (Go API) | Directly, same as `web/app`. |
| `desktop` | Server (Go API) | Directly, same as `web/app`. |

## Base URLs

Values come from the root `.env` (see `.env.example` and `infra/README.md`
→ "Routes (dev)"). Never hardcode these in app code — read from an env var
per app's own framework convention (e.g. `NEXT_PUBLIC_API_URL` for the
Next.js apps).

| Environment | Server base URL |
|---|---|
| Dev (via `infra/docker/docker-compose.dev.yml` + nginx) | `http://api.bibliotheca.local` |
| Dev (Go API run directly, no Docker) | `http://localhost:8080` (or `$SERVER_HOST_PORT` if you changed it — see `infra/README.md`) |
| Prod | Not yet deployed anywhere — no real domain assigned yet. |

`admin/web`'s equivalent (its own backend, not Server) is
`http://admin.bibliotheca.local/api` in dev — see `ADMIN_WEB_API_URL` in the
root `.env.example`.

## Auth

Every non-public Server route expects `Authorization: Bearer <access_token>`
(see `Server/docs/API.md` → "Auth"). Access tokens expire in 15 minutes by
default (`JWT_ACCESS_TOKEN_TTL`) — any client integration needs to handle
refresh via `POST /auth/refresh` before that, not just react to a `401`.
