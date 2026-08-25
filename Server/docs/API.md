# API Reference

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

All routes are prefixed `/api/v1` except `/health` and `/swagger/*`. All responses are JSON with the shared envelope:

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
