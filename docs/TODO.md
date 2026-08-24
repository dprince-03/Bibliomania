# TODO

Living task list — update this as work lands or priorities change. Don't let
it go stale; a wrong TODO is worse than none. See `docs/plan.md` for how the
infra branch got here, and `Server/docs/Steps.md` for the Server-side
feature roadmap (Steps 13-20).

## Blocking / high priority

- [ ] **Fix the Go server build break** (`Server/docs/Steps.md` → "Known build
      issues"): import-path split (`github.com/yourusername/bibliotheca/...`
      vs `bibliotheca/...`), repository/service method-name mismatch,
      `router.New(...)` signature mismatch, `RunMigrations` vs `RunMigration`.
      Nothing that depends on the server actually running (full `docker
      compose up`, real end-to-end testing of any app against the API) can
      happen until this lands.
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
- [ ] **Server "Structure by Feature" refactor** — move
      `Server/internal/{handlers,services,repository,models,dto}/<domain>.go`
      (grouped by layer) to `Server/internal/<domain>/{handler,service,
      repository,model,dto}.go` (grouped by feature). **Next up**: its own
      branch (`refactor/server-feature-structure`), right after
      `feat/infra-docker-stack` merges — deliberately not bundled with this
      branch's unrelated infra/CI work.

## Infra follow-ups (see `infra/README.md` → "Known caveats")

- [ ] `admin-api` has no compiled build step (`nest new --language
      javascript` scaffolding runs via `babel-node` even in prod). Add a
      `babel index.js src --out-dir dist` build script once there's real
      code, then switch `Dockerfile.prod` to a proper multi-stage build
      running `node dist/main.js`.
- [ ] TLS/HTTPS for prod nginx (certbot or similar) — not set up.
- [ ] A full `docker compose up --build` (dev) has never been run
      end-to-end — only `config` validation and individual `docker build`
      runs per service. Do this once the server build-break is fixed.
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
