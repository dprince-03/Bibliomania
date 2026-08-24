# TODO

Living task list — update this as work lands or priorities change. Don't let
it go stale; a wrong TODO is worse than none. See `docs/plan.md` for how the
infra branch got here, and `Server/docs/Steps.md` for the Server-side
feature roadmap (Steps 13-20).

## Known repo hygiene issue

- [x] `Client/admin/api/node_modules/` (~4,051 files) was tracked in git —
      root cause was a too-narrow `.gitignore` pattern (`*/node_modules`
      only matches one directory deep; `Client/admin/api/node_modules` is
      three deep) plus `admin/api` never having its own `.gitignore` at all.
      Fixed on `refactor/server-feature-structure`: root `.gitignore` now
      has a proper `node_modules/` pattern (matches any depth),
      `Client/admin/api/.gitignore` added, and the tracked files removed via
      `git rm -r --cached`.
      **Still open**: this only stops recurrence and cleans up going
      forward — the blob data is still in `main`'s already-pushed git
      history (from `feat/infra-docker-stack`'s merge commit). Fully
      purging it needs a history rewrite (`git filter-repo`/BFG) +
      force-push to `main`, which wasn't done here given how disruptive
      that is to a shared branch — do this deliberately, separately, if it
      matters enough to justify the disruption.

## Blocking / high priority

- [ ] **Confirm GHCR push permissions.** The `*-cd.yml` workflows push to
      `ghcr.io/dprince-03/bibliotheca-<service>` using the default
      `GITHUB_TOKEN` — this requires the repo's Settings → Actions → General
      → Workflow permissions to allow "Read and write permissions" (not the
      read-only default). Untested — verify on first real push to `main`.

## Decided

- [x] **Desktop app stays out of Docker.** Confirmed: JavaFX is a native GUI
      app, not a long-lived headless service — CI (`desktop-ci.yml`)
      verifies the build; that's sufficient. Revisit only if a concrete need
      for a headless/VNC demo mode comes up.
- [x] **Server "Structure by Feature" refactor** — done, on
      `refactor/server-feature-structure`. `Server/internal/` now groups by
      domain (`auth/`, `user/`, `catalog/`, `borrow/`, `reading/`) instead of
      by layer — see `docs/ARCHITECTURE.md` and `CLAUDE.md` for the package
      map, `Server/docs/Steps.md` → "Build history" for what got fixed along
      the way. As an inherent side effect (not separate work), this also
      fixed the Go server build break that used to block everything below —
      `go build ./...`/`go vet ./...` succeed and the server boots and
      serves real requests now.
- [x] **Hand-written API docs added**: `Server/docs/API.md` (endpoint
      reference), `Client/docs/API.md` (which client app calls what, base
      URLs per environment), `docs/API.md` (pointer to both). Deliberately
      not the full swaggo/OpenAPI pipeline — that stays Step 18's own
      branch, see `Server/docs/Steps.md`. Keep `Server/docs/API.md` in sync
      by hand until Step 18 replaces it with something generated.

## Infra follow-ups (see `infra/README.md` → "Known caveats")

- [ ] `admin-api` has no compiled build step (`nest new --language
      javascript` scaffolding runs via `babel-node` even in prod). Add a
      `babel index.js src --out-dir dist` build script once there's real
      code, then switch `Dockerfile.prod` to a proper multi-stage build
      running `node dist/main.js`.
- [ ] TLS/HTTPS for prod nginx (certbot or similar) — not set up.
- [ ] A full `docker compose up --build` (dev) has never been run
      end-to-end — only `config` validation and individual `docker build`
      runs per service. Now unblocked (server build break is fixed) — worth
      doing.
- [ ] Mobile/desktop CD currently uploads unsigned build artifacts only — no
      code-signing or app-store/distribution pipeline exists for either
      platform.

## Product code (nothing built yet — all scaffolds)

- [ ] `Server`: Steps 13-20 (search/filtering completion, e-library
      upload/download, reading sessions, borrowing, user/member management,
      Swagger docs, Makefile) — see `Server/docs/Steps.md`.
- [ ] `Client/web/app` — the actual library product app (currently the bare
      Next.js template).
- [ ] `Client/web/main` — the marketing site (currently the bare Next.js
      template).
- [ ] `Client/admin/web` + `Client/admin/api` — the admin dashboard
      (currently bare templates on both sides).
- [ ] `Client/mobile` — the mobile app (currently `flutter create`'s default
      counter demo).
- [ ] `Client/desktop` — the desktop app (currently a single placeholder
      window).

## Testing

- [ ] No tests exist anywhere in the repo yet (Server has zero `_test.go`
      files; every Client app has only its framework's default test
      scaffold, if any). Add tests alongside the product code above, not as
      a separate retrofitting pass.
