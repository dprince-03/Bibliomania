# Contributing

## Branching

One concern per branch — a roadmap step, an infra change, a refactor — not
bundled together. Two naming patterns:
- Roadmap steps: `feat/step-<N>-<short-slug>` (e.g.
  `feat/step-13-search-filtering`). The Server roadmap
  (`Server/app/docs/Steps.md`) has Steps 1-20 done and 21-45 planned.
  Client apps keep their own separately-numbered roadmaps (e.g.
  `Client/docs/web-app-Steps.md`, Steps 1-7). Don't mix up the two
  numberings.
- Everything else (infra, refactors, cross-cutting work):
  a descriptive name in the same spirit (e.g. `feat/infra-docker-stack`,
  `refactor/server-feature-structure`).

After a roadmap step lands, update its entry in that roadmap's `Steps.md`
(status marker + notes) as part of that same branch/PR.

## Before opening a PR

Run the checks CI will run, locally, first — see each app's own CI workflow
in `.github/workflows/<app>-ci.yml` for the exact commands (they're kept
deliberately simple: `go vet`/`build`/`test`, `npm run lint`/`build`,
`flutter analyze`/`test`, `mvn package`). A PR that fails CI on something you
could have caught locally wastes a review cycle.

## Commit messages

Describe *why*, not just *what* — the diff already shows what changed.
Conventional prefixes (`feat:`, `fix:`, `refactor:`, `docs:`) aren't
enforced, but keep the first line short and the body focused on intent and
tradeoffs, not a restatement of the diff.

## Docs discipline

- **Every change and bug fix gets a dated line in `docs/CHANGELOG.md`**,
  newest first, including ones too small for `docs/plan.md` or a
  `Steps.md`. Nothing should exist only in chat history or a commit
  message.
- Update `docs/TODO.md` when you finish something on it or discover new
  work — it's meant to stay current, not describe a snapshot from whenever
  it was last touched.
- If you change something documented in `.claude/CLAUDE.md`, `infra/README.md`, or
  `docs/ARCHITECTURE.md`, update that doc in the same PR. Stale
  architecture docs are worse than none, because they're actively
  misleading.
- New apps, services, or major structural changes get a line in
  `docs/ARCHITECTURE.md`, not just a mention buried in a PR description.
- Major initiative plans (an infra branch, the platform-vision repositioning,
  etc.) live in `docs/plan.md` as a running log — add a new `##` section
  there rather than a one-off file. Server-side technical plans that follow
  from one of those live in `Server/app/docs/plan.md`. As each planned piece
  actually lands, update the relevant section (or move it into
  `Server/app/docs/Steps.md` once it's a real, numbered step) instead of leaving
  the plan doc describing something already shipped. `docs/plan.md` is
  append-only: add a new dated section instead of rewriting an older one.
  If an older entry turns out to be wrong, strike it through and add a
  dated correction rather than silently editing it.
- Draft interface contracts (`.proto`, GraphQL SDL, OpenAPI) for planned
  services live in `docs/interfaces/`. Keep them in step with the
  data-ownership decisions in `docs/plan.md`.

## CI/CD

Every app has its own `<app>-ci.yml` (runs on every PR/push touching that
app's path) and `<app>-cd.yml` (runs on push to `main` only — builds and
pushes to GHCR, or uploads a build artifact for mobile/desktop). See
`infra/README.md`'s "CI/CD" section for the full list and what each does.
Dependabot (`.github/dependabot.yml`) opens weekly update PRs for every
ecosystem in the repo — review and merge these like any other PR, don't
let them pile up.
