# Interface contracts

> **Superseded 2026-09-25.** These were the mock-up-phase drafts. The real
> contracts now live in the code and are what to read and edit:
>
> - gRPC: `Server/app/proto/<service>/v1/*.proto` (catalog, borrow,
>   reading, user) → generated into `Server/app/gen/`
> - GraphQL: `Server/app/internal/gateway/graph/schema.graphqls`
> - REST: the generated Swagger spec the gateway serves at `/swagger/`
>
> The drafts below are kept unedited as a record of the mock-up phase. The
> one open inconsistency noted at the bottom was resolved in the real
> schema: `BorrowRecord` and `ReadingHistoryEntry` now expose both
> `bookTitle` (the snapshot) and a nullable live `book`.

Mock-up phase, not implementation — these describe the shape of the
interfaces decided in [`docs/plan.md`](../plan.md)'s microservices-split
sections, so the restructuring work (queued after the mock-up phase) has
something concrete to build against instead of starting from scratch.
Nothing here is generated/compiled yet — no `protoc`, no codegen — these
are the contracts themselves, for review, not running code.

## What's here

- **`catalog.proto`** — the one gRPC contract with real cross-service
  *correctness* dependencies: `borrow-service`'s reserve/release Saga and
  `reading-service`'s existence check both call this, and nothing else.
  Written first because it's the interface that encodes an actual design
  decision (the Saga, idempotency keys), not just a mechanical translation
  of an existing REST endpoint.
- **`gateway.graphql`** — the GraphQL gateway schema `web/app` will talk
  to instead of five separate services' REST APIs. This is where every
  display-aggregation case from the data-ownership resolution lives:
  `Book.myBorrowStatus`, `Book.myLibraryStatus`, and `myHistory` are the
  three concrete fan-outs that motivated having a gateway at all.

## Known gap between these contracts and today's code

The data-ownership decision says `borrow` and `reading` **snapshot** the
book title onto their own rows at creation. Today's
`borrow.Service.enrichBorrows` doesn't do that. It looks up the title live,
once per record, on every read, and `BorrowRecord` has no title column. Snapshotting is
new behavior the restructure has to add.

**Open inconsistency in `gateway.graphql`:** `BorrowRecord` exposes
`book: Book!`, which the gateway would resolve live through `catalog`. It
has no snapshotted `bookTitle` field, so the schema doesn't yet reflect the
snapshot decision. Either add a `bookTitle: String!` (snapshot) alongside
or instead of `book`, or record a decision that borrows resolve live after
all. Not changed here without that decision.

## What's still missing (mechanical, not design-heavy)

Each of `auth`, `borrow`, `reading`, and `user` needs its own small proto
file exposing what the gateway calls internally — `borrow-service`'s
`BorrowBook`/`ReturnBook`/`GetMyBorrows`, `user-service`'s
`GetProfile`/`UpdateLibraryStatus`/`GetLibrary`, `reading-service`'s
`UpdateProgress`/`CreateBookmark`/`GetHistory`, `auth-service`'s
`Login`/`Register`/`Refresh`. Unlike `catalog.proto`, none of these
involve a new design decision — they're each a direct translation of an
endpoint that already exists and works in `Server/app` today (see
`Server/app/internal/modules/*/handler.go`), just moved from an HTTP
handler signature to a gRPC one. Left for a follow-up pass rather than
included here, since there's no open question to resolve in writing them.

Also not yet started: OpenAPI specs for the REST surfaces that stay REST
(`Client/admin`'s calls into each service, health checks) — same
"mechanical, not design-heavy" reasoning applies.
