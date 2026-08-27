# API Reference

Covers the API as it exists today (Steps 1-20). Planned additions from the
platform-vision repositioning — libraries/branches, payments/subscriptions,
curation, translation, audiobooks, etc. — aren't part of the API yet; see
[`Server/docs/plan.md`](plan.md) for what's planned and root
[`docs/plan.md`](../../docs/plan.md) for the business context.

**The authoritative, up-to-date endpoint reference is the generated Swagger UI**, not this file:

```bash
go run ./cmd/api
# then open http://localhost:8080/swagger/index.html
```

Every route, request/response shape, and auth requirement lives there, generated directly from the handler annotations (see `Server/docs/Steps.md` → Step 18) — it can't drift out of sync with the code the way a hand-written list eventually would. This file only covers the cross-cutting conventions that don't belong on any single endpoint.

**Regenerating** after changing a handler's annotations:

```bash
cd Server && make swagger
# equivalent to:
#   swag init -g cmd/api/main.go --output internal/swaggerdocs --parseInternal --parseDependency
#   swag fmt -g cmd/api/main.go
```

`internal/swaggerdocs/` (the generated `docs.go` + `swagger.json` + `swagger.yaml`) is committed, not gitignored — `cmd/api/main.go` blank-imports `docs.go` for its `init()` side effect, so it needs to be present for `go build`/`go run` to work.

## Conventions

All routes are prefixed `/api/v1` except `/health` and `/swagger/*`. `GET /health` pings the database and Redis with a 2s timeout and returns `200 {"status":"ok","database":"ok","cache":"ok"}` when both are reachable, or `503 {"status":"degraded",...}` naming whichever one failed. All other responses are JSON with the shared envelope:

```json
// success
{ "success": true, "message": "...", "data": { ... } }

// error
{ "success": false, "error": "...", "code": 404 }
```

`code` matches the HTTP status. Paginated list endpoints wrap `data` as:

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

Authenticated routes expect `Authorization: Bearer <access_token>` (obtained from `/auth/login` or `/auth/register`) — in Swagger UI, click "Authorize" and paste the token to try these interactively.
