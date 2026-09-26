# API Reference

Since the microservices split (2026-09-25), every client talks to **one
gateway** (`cmd/gateway`), which exposes three things:

| Surface | Path | For |
| --- | --- | --- |
| REST | `/api/v1/...` | Every existing client (`web/app`, `web/main`, mobile, desktop, `Server/admin`). Same paths, request/response shapes and envelope as before the split — the gateway reverse-proxies each route to the service that owns it. |
| GraphQL | `POST /graphql` (`GET /playground` outside production) | `web/app`'s intended future API: one query fans out to several services (e.g. a book + your borrow of it + its status on your shelf). Schema: `internal/gateway/graph/schema.graphqls`. |
| Ops | `/health`, `/metrics`, `/swagger/`, `/api/versions` | Health of the whole system, Prometheus metrics, docs, version discovery. |

Services also talk to each other over **gRPC** (contracts in `proto/`) and
**events** (RabbitMQ / Kafka / NATS) — internal only, never client-facing.
See [`plan.md`](plan.md) → "Microservices split" for which service owns
what.

**The authoritative REST endpoint reference is the generated Swagger UI**, not this file:

```bash
make docker-up          # from Server/app — the whole stack
# then open http://localhost:9081/swagger/index.html (the gateway)
```

Every route, request/response shape, and auth requirement lives there,
generated from the handler annotations in every service (the gateway serves
one combined spec). This file only covers the conventions that don't belong
on any single endpoint.

**Regenerating** after changing any handler's annotations:

```bash
cd Server/app && make swagger
# equivalent to:
#   swag init -g cmd/gateway/main.go --output internal/swaggerdocs --parseInternal --parseDependency
#   swag fmt -g cmd/gateway/main.go
```

`internal/swaggerdocs/` is committed — `cmd/gateway/main.go` blank-imports
it. `make graphql` regenerates the GraphQL gateway after a schema edit;
`make proto` regenerates `gen/` after a `.proto` edit.

## Versioning

Every interface is versioned, each in the way that fits it:

| Interface | How it's versioned | A breaking change means |
| --- | --- | --- |
| REST | URL path: `/api/v1/...` | A new `/api/v2/...` served alongside v1, v1 deprecated with a sunset date, removed after it. |
| GraphQL | Not versioned by URL — the schema evolves additively | Add the new field/argument, mark the old one `@deprecated(reason: "...")`, remove it once clients have moved. |
| gRPC | Proto package: `bibliomania.<service>.v1` | A new `v2` package; `buf breaking` in CI (`app-ci.yml`) rejects incompatible edits to a v1 package. |
| Events | `schema_version` field on every event envelope (`internal/events/types.go` → `CurrentVersions`) | Bump that type's version; the producer publishes both until consumers handle the new one. Consumers reject unknown versions (dead-letter) instead of misreading them. |

**What counts as breaking** (REST and GraphQL alike): removing or renaming a
field or route, changing a field's type or meaning, making an optional
input required, changing what a status code means. Adding an optional
field, a new route, or a new enum value clients can ignore is **not**
breaking and ships in place.

### REST version mechanics (gateway, `internal/apiversion`)

- `GET /api/versions` lists what's served:

  ```json
  { "success": true, "data": { "current": "v1",
    "versions": [ { "version": "v1", "status": "current", "base_path": "/api/v1" } ] } }
  ```

- Every `/api/v1/...` response carries `API-Version: v1`.
- A request for a version that isn't served (`/api/v2/...` today, or a path
  with no version) gets a JSON `404` naming the supported ones — never a
  silent fallback:

  ```json
  { "success": false, "error": "unsupported API version \"v2\"", "code": 404, "supported_versions": ["v1"] }
  ```

- Deprecating a version is configuration, not code:
  `API_DEPRECATED_VERSIONS=v1:2027-01-01:2027-07-01` (version :
  deprecated-on : sunset-on) on the gateway. Its responses then carry
  `Deprecation: @<unix time>` (RFC 9745), `Sunset: <HTTP date>` (RFC 8594)
  and `Link: </api/v2>; rel="successor-version"`.
- Adding a version: services register the new `/api/v2/...` routes next to
  the v1 ones, the gateway gets `"v2"` appended to `gateway.ServedVersions`
  and v2 routes in its proxy table.

## Conventions

All REST routes are prefixed `/api/v1` except `/health`, `/metrics`,
`/swagger/*` and `/api/versions`. All REST responses are JSON with the
shared envelope:

```json
// success
{ "success": true, "message": "...", "data": { ... } }

// error
{ "success": false, "error": "...", "code": 404 }
```

`code` matches the HTTP status. `502` means the gateway couldn't reach the
service behind a route; `503` means a service couldn't reach one it depends
on (e.g. borrow-service → catalog-service). Paginated list endpoints wrap
`data` as:

```json
{
  "items": [ ... ],
  "total_count": 0,
  "page": 1,
  "limit": 10,
  "total_pages": 0,
  "has_next_page": false,
  "has_previous_page": false
}
```

Pagination query params: `?page=1&limit=10` (both optional, on every list endpoint).

Authenticated routes expect `Authorization: Bearer <access_token>`
(obtained from `/auth/login` or `/auth/register`) — in Swagger UI, click
"Authorize" and paste the token. Every service verifies the token itself;
the gateway just forwards it.

`GET /health` on the gateway returns `200 {"status":"ok"}` only if every
service behind it reports healthy, `503 {"status":"degraded"}` otherwise,
with a per-service `checks` map. Each service's own `/health` (internal)
checks its database, cache and brokers the same way.

### Request limits and validation

- JSON bodies: at most **1 MB** (`413`), no unknown fields (`400`), no
  trailing data after the object. URL fields (`cover_image`, …) must be
  `https://`. Emails are trimmed and lower-cased.
- Search: `q` ≤ 200 characters, `genre` ≤ 100 (`400`).
- GraphQL: query depth ≤ 8.
- Uploads (`POST /books/{id}/upload`): the file's *content* must be a
  real PDF or EPUB and must match its extension. An `.html` renamed to
  `.pdf` gets `400`. When virus scanning is enabled, an infected file
  gets `422`, and a scanner that can't be reached gets `503` (fail
  closed).
- Downloads support `Range` (`206`) and are served with
  `X-Content-Type-Options: nosniff`.

### Idempotency

Every mutating REST route (`POST`/`PUT`/`PATCH`/`DELETE`) behind the
gateway accepts an optional `Idempotency-Key` header (≤100 chars, e.g. a
UUID). Send it when a retry must not repeat the action, e.g. after a
timeout where you didn't see the response.

- **Same key, same body** (within 24h): the stored response is returned
  as-is, with `Idempotent-Replayed: true`. Nothing runs a second time.
- **Same key, different body**: `422`.
- **Same key, first request still running**: `409` — retry shortly.
- Keys are scoped per caller (user, or IP when anonymous) and per route.
  5xx responses aren't stored, so a retry after a server error does run
  again. Multipart uploads aren't covered.

`POST /borrows` additionally keeps its own per-user key on the borrow
record (`201` the first time, `200` on a replay). The GraphQL
`borrowBook(bookId, idempotencyKey)` mutation has the same semantics.

### IDs

Borrow records and purchases carry a **`public_id`**: a UUIDv7, not
guessable, and time-ordered. Prefer it in URLs and in anything you store:
`PATCH /borrows/{id}/return` accepts either the numeric `id` or the
`public_id`. Numeric IDs remain for compatibility.

### Rate limits

Per client IP, shared across gateway replicas. The global default is 100
req/s with a burst of 200; `/auth/*` allows 5 per burst, refilling at 10
per minute. Responses carry `RateLimit-Limit`, `RateLimit-Remaining`
and `RateLimit-Reset`; exceeding a limit gets `429` with `Retry-After`.

### When a service is down

- `503` with `Retry-After`: a circuit breaker is open (the dependency
  failed repeatedly) or the service is saturated. Retry after the given
  delay. Writes always fail rather than pretend to succeed.
- Catalog reads (`GET /books`, `/books/{id}`, search) keep working from
  a cached copy while catalog's database is unreachable.
- `GET /users/me/library` without catalog-service: the entries come back
  without live titles, with the header `X-Degraded: catalog-service`. In
  GraphQL, `myLibrary` returns the entries plus an error on the missing
  `book` fields.

### GraphQL errors

Errors carry a machine-readable `extensions.code` (`UNAUTHENTICATED`,
`FORBIDDEN`, `NOT_FOUND`, `BAD_USER_INPUT`, `CONFLICT`,
`SERVICE_UNAVAILABLE`, `INTERNAL_SERVER_ERROR`) plus the equivalent HTTP
`extensions.status`. Fields that need a user (`me`, `my*`, mutations) fail
with `UNAUTHENTICATED` when no token is sent; per-user fields on public
types (`Book.myBorrowStatus`, `Book.myLibraryStatus`) are just `null` for
anonymous callers. Sending an *invalid* token gets an HTTP `401` for the
whole request, so the client knows to refresh.

## Behaviour changes from the split

Deliberate, and small — the REST contract is otherwise unchanged:

- `GET /borrows`, `/borrows/my`: `book_title` is the title **when the book
  was borrowed** (snapshot), not today's title.
- `GET /users/me/history`: served by reading-service; `book_title` is the
  title when the reading session started.
- `GET /users/me/library`: `book_title` is resolved live (the gateway asks
  catalog-service), so a wishlist shows the current title.
- `POST /borrows`: `409` also when you have an **overdue** (not just
  active) borrow of the same book.
- `PATCH /borrows/{id}/return` now returns the updated borrow in `data`
  (previously `null`), and `{id}` may be the `public_id`.
- `POST /borrows` with no copies left: **`409`** (was `400`), since the
  request is valid and the book's state is what conflicts (2026-09-26).
- `PATCH /books/{id}` lowering `total_copies` below the number currently on
  loan: `409`.
- `POST /auth/refresh` with a refresh token that was **already used**:
  `401`, **and every session for that user is revoked**. A reused token
  means it was copied (OAuth reuse detection), so the user must log in
  again. Each refresh token works exactly once.
- `POST /payments/checkout` again for a book with an open checkout
  returns that same checkout (same `checkout_url`) instead of starting a
  second payment.
- `GET /reading/{bookId}/session` and `.../bookmarks` no longer check that
  the book exists — they just return your (empty) data. Creating a session
  or bookmark still validates the book.
- `GET /users/me` can `404` for a few milliseconds right after
  registration, until user-service applies the `auth.user_registered`
  event. Deactivating a user reaches auth-service the same way (normally
  within a second) and also revokes their refresh tokens.
- `total_books_read` / `total_pages_read` on `GET /users/me` now actually
  update (they were never incremented before the split): user-service
  bumps them when reading-service reports a completed book.
- New: `POST /payments/checkout`, `GET /payments/my`, `POST
  /payments/webhook/{stripe|paystack}` (payment-service), and optional
  `price_cents` + `currency` on books — see "Payments" below.

## Payments

One-time book purchases through **Stripe** or **Paystack**, chosen per
purchase by the book's currency: `PAYSTACK_CURRENCIES` (default NGN, GHS,
ZAR, KES) go to Paystack, anything else to Stripe. `POST /payments/checkout`
takes `{"book_id": 1}` plus an optional `"provider": "stripe"|"paystack"` to
force one (it must still accept the currency), and returns
`{purchase_id, public_id, provider, checkout_url}` — redirect the reader
there. A repeat within the session lifetime returns the same checkout; a
concurrent duplicate that's still starting gets `409`.

- `price_cents` is in the currency's **minor unit** (cents, kobo, pesewas)
  for both providers. A book with no price isn't for sale (`400`).
- A provider with no secret key configured is disabled; if no configured
  provider takes a book's currency, checkout returns `400`.
- The purchase is `pending` until the provider's **webhook** confirms it.
  Register `…/api/v1/payments/webhook/stripe` and
  `…/api/v1/payments/webhook/paystack` in each dashboard. Stripe is
  verified with `Stripe-Signature` against `STRIPE_WEBHOOK_SECRET`.
  Paystack is verified with `x-paystack-signature` (HMAC-SHA512 with the
  secret key) and then re-checked against Paystack's Verify API, so the
  amount and status come from Paystack itself, not the webhook body.
  Deliveries are deduped per provider.
- A "paid" outcome whose amount or currency doesn't match the purchase
  never grants the book: the purchase stays `pending` and it's logged for
  manual review. The same book can't be paid for twice (`409` at checkout,
  enforced by a unique index).
- Live keys (`sk_live_…`, for either provider) make payment-service refuse
  to start unless `PAYMENTS_LIVE_ENABLED=true` (the Step 38 launch gate).
