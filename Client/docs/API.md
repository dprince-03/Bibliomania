# API Reference (frontend view)

The endpoint list, request/response shapes, and auth model live in the
generated Swagger UI (`http://localhost:8080/swagger/index.html` when the
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
| `web/app` | the Go API (`Server/app`) | Directly — the product app is the primary consumer of every endpoint in `Server/app/docs/API.md`. |
| `web/main` | the Go API (`Server/app`) | Directly, client-side only — a small public-endpoint fetch for live book/author counts on the home page (`GET /api/v1/books`, `GET /api/v1/authors`), nothing else. |
| `admin` | `Server/admin`, not the Go API directly | The admin dashboard's own backend fronts the Go app's data — see `docs/ARCHITECTURE.md` → "Why a separate admin backend instead of extending the Go app". |
| `Server/admin` | The Go app's MySQL database directly, and potentially its API for some operations | Not yet decided which — see `docs/ARCHITECTURE.md` for the caveat about two backends sharing one database. |
| `mobile` | the Go API (`Server/app`) | Directly, same as `web/app`. |
| `desktop` | the Go API (`Server/app`) | Directly, same as `web/app`. |

## Base URLs

Values come from the root `.env` (see `.env.example` and `infra/README.md`
→ "Routes (dev)"). Never hardcode these in app code — read from an env var
per app's own framework convention (e.g. `NEXT_PUBLIC_API_URL` for the
Next.js apps).

| Environment | Go API base URL |
|---|---|
| Dev (via `infra/docker/docker-compose.dev.yml` + nginx) | `http://api.bibliotheca.local` |
| Dev (Go API run directly, no Docker) | `http://localhost:8080` (or `$SERVER_HOST_PORT` if you changed it — see `infra/README.md`) |
| Prod | Not yet deployed anywhere — no real domain assigned yet. |

`admin`'s equivalent (its own backend, `Server/admin`, not the Go app) is
`http://admin.bibliotheca.local/api` in dev — see `ADMIN_WEB_API_URL` in the
root `.env.example`.

## Auth

Every non-public Go API route expects `Authorization: Bearer <access_token>`
(see `Server/app/docs/API.md` → "Auth"). Access tokens expire in 15 minutes by
default (`JWT_ACCESS_TOKEN_TTL`) — any client integration needs to handle
refresh via `POST /auth/refresh` before that, not just react to a `401`.
