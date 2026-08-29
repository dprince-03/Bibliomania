# `Client/web/app` plan

**Status: Steps 1-3 (Foundations, Auth, Catalog browsing) done, Steps 4-7 not started.** See [`web-app-Steps.md`](web-app-Steps.md) for the step-by-step build checklist — one step, one branch, same discipline as `Server/app/docs/Steps.md`. This file covers the *why* and the cross-cutting architecture decisions; the Steps file covers the *what*, in order.

## Context

`Client/web/app` is the actual product — the primary consumer of nearly every member-facing endpoint the Go API already exposes (Steps 1-20 of `Server/app/docs/Steps.md`, all complete), and it's what the marketing site's (`Client/web/main`) "Open the App" button already points at. Right now it's still the bare `create-next-app` scaffold. Rather than building it as one large branch, it's split into discrete features, each built and verified on its own branch before the next starts — the same convention already used for the Server roadmap.

## Mission

Turn the real, already-working Go API into something a member actually uses: search the catalog, borrow a book, read an e-book, track a personal shelf and reading history — the experience the marketing site is already selling.

## Scope

Everything the Go API's member-facing surface supports is in scope eventually: auth, catalog browsing/search, borrowing, profile/personal library/reading history, and the full e-library reading experience (download, in-browser reader, progress sync, bookmarks). It's just sequenced across steps instead of built in one pass — see `web-app-Steps.md`.

Explicitly **not** in scope here: anything librarian/admin-only (author/book write endpoints, all-borrows listing, user-management/status endpoints) — those belong to `Client/admin` + `Server/admin`, not this app.

## Architecture decisions

### Auth: httpOnly-cookie session, not client-side JWT storage

The Go API issues a 15-minute access token plus a 7-day, one-time-use rotating refresh token (`Server/app/internal/modules/auth`). Storing either in `localStorage` would expose them to XSS and push refresh-token-rotation race handling into browser JS. `Client/web/app`'s `next.config.mjs` already sets `output: "standalone"`, which means this app has a real Node server at runtime (unlike `web-main`'s static export) — so login/register/refresh run as **Server Actions** that call the Go API directly and set the returned tokens as httpOnly cookies (`secure`, `sameSite: lax`). The browser never sees the raw JWT.

This is Next.js's own documented recommended pattern for this exact situation, confirmed against this installed version's own docs (`node_modules/next/dist/docs/01-app/02-guides/authentication.md`) rather than assumed from general knowledge — worth calling out because...

### This Next.js version has real breaking changes from what you'd expect

`Client/web/app/AGENTS.md` warns of this directly, and it's concretely true here: **the middleware convention is renamed to `proxy.js`/`proxy.ts`** in this version — not `middleware.js`. Route protection (optimistic redirect-to-login on protected routes, reading only the session cookie, no DB/API round-trip) goes in `proxy.js`. Anyone building against this app should re-check `node_modules/next/dist/docs/` before assuming a convention, per that same AGENTS.md note.

### A server-only API client wrapper centralizes the hard parts

One module (`lib/api.js`) is responsible for: attaching `Authorization: Bearer <access_token>` from the cookie to every Go API call, unwrapping the `{success, message, data}` response envelope, and transparently calling `POST /auth/refresh` + re-setting cookies on a `401` before retrying the original request once. This keeps the rotating-refresh-token dance in one place instead of scattered across every Server Action and Route Handler that touches the API.

### Two API base URLs, not one

- `API_INTERNAL_URL` (server-only, no `NEXT_PUBLIC_` prefix so it's never shipped to the browser) — used by the API client wrapper above. Defaults to `http://app:8080` inside Docker (the Go API's Compose service, reachable by container name on `bibliotheca_network`), falls back to `http://localhost:8080` for bare `npm run dev` outside Docker.
- `NEXT_PUBLIC_API_URL` (already wired in `infra/docker/docker-compose.yml` as `WEB_APP_API_URL`) — stays available for any direct, unauthenticated client-side calls a later step might want (e.g. anonymous catalog browsing before login, mirroring the pattern `web-main` already uses for its live stats strip).

These need to stay distinct: browsers can't resolve Docker-internal service names like `app`, so client-side code needs the public `api.bibliotheca.local` hostname, while server-side code run *inside* the Docker network can (and should, for reliability) skip the extra nginx hop.

### No shared workspace

Confirmed repo-wide convention: every Client app has its own independent `package.json`, no monorepo tool. Components (`Container`, `Button`, `Navbar`, `Footer`, etc.) get built fresh in `web/app`, not imported from `web-main` or `admin`, even where conceptually similar.

## Visual identity

A regal "royal library / illuminated manuscript" look — deliberately distinct from `web-main`'s warm/parchment "warm & literary" identity, since this is a daily-use tool, not a one-time-visit marketing page. Palette and fonts below get set up once, in Step 1, and used throughout.

### Colors

Every color has one job — mixed on purpose, not just listed:

| Role | Light mode | Dark mode |
|---|---|---|
| Background | White | Dark Blue `#0a1633` |
| Surface (cards) | pale warm tan (from Light Brown) `#f2e6d4` | lighter navy `#142a52` |
| Text | Dark Blue `#0d1b3e` | warm off-white `#f2ede1` |
| Muted text / soft accent | — | Light Blue `#9db4d9` |
| Border | muted Light Brown `#c9a878` | royal-blue-tinted, muted |
| Primary accent (CTAs, highlights) | Gold `#b8860b` | brighter Gold `#d4af37` |
| Structural accent (links, nav) | Royal Blue `#2648b0` | Light Blue `#9db4d9` |
| Alert accent (overdue, errors — sparing use) | Red `#8c2f2f` | brighter Red `#c0484a` |

White and Dark Blue anchor the two modes; Gold is the one primary accent tying everything together; Royal Blue carries structure; Light Blue softens dark mode; Red is reserved for warnings, never decorative; Light Brown warms up light mode without duplicating `web-main`'s parchment tone.

### Fonts — three tiers

True old-English/blackletter type is illegible at body-text size, and this app exists for extended reading — so it's scoped to exactly one place:

- **Logo only**: `UnifrakturMaguntia` (real blackletter) — the "Bibliotheca" wordmark, nowhere else.
- **Headings**: `Cinzel` — an engraved, Roman-capital display serif. Regal, still fully legible at heading sizes.
- **Body/UI text**: `EB Garamond` — a genuinely old but comfortably readable-at-length serif, for both UI chrome and (later) actual book-reading content.

All three via `next/font/google`, set up once in Step 1.

### Themes

Light/dark via `prefers-color-scheme`, set up in Step 1 — the same mechanism `web-main` and `admin` already use, not a new pattern. A real manual light/dark/**sepia** toggle is deliberately deferred to Step 6 (the e-library reader) specifically, since that's where readers actually want it (reading at night) — not built as a premature app-wide theme switcher before there's much content to switch a theme on.

## Verification discipline

Each step is built and verified against the real, running Go API — not just `next build` succeeding — before being considered done, matching the standard already set for every completed Server step.
