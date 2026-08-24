# API Reference

Hand-written reference for the endpoints that actually exist today. This is
**not** generated — see `Server/docs/Steps.md` → Step 18 for the planned
swaggo/OpenAPI pipeline (live-generated docs + `/swagger/*` UI), which is
deliberately out of scope for now. When Step 18 lands, this file should be
replaced by (or reduced to a pointer to) the generated spec — don't let both
drift out of sync.

All routes are prefixed `/api/v1` except `/health`. All responses are JSON
with the shared envelope:

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

Pagination query params: `?page=1&limit=10` (both optional).

## Auth

None of these require a token. The strict rate limiter (10 req/min per IP)
applies to all four.

### `POST /auth/register`
```json
// request
{ "first_name": "Ada", "last_name": "Lovelace", "email": "ada@example.com", "password": "at-least-8-chars" }
```
Returns `201` with an `AuthResponse` (see below). New users are created with `role: "member"`.

### `POST /auth/login`
```json
{ "email": "ada@example.com", "password": "..." }
```
Returns `200` with an `AuthResponse`.

### `POST /auth/logout`
```json
{ "refresh_token": "..." }
```
Revokes the given refresh token. Returns `200`, no data.

### `POST /auth/refresh`
```json
{ "refresh_token": "..." }
```
Rotates the refresh token (old one is revoked, one-time use) and returns a new `AuthResponse`.

**`AuthResponse` shape:**
```json
{
  "user": { "id": 1, "first_name": "Ada", "last_name": "Lovelace", "email": "ada@example.com", "role": "member", "is_active": true },
  "token": { "access_token": "...", "refresh_token": "...", "expires_in": 604800 }
}
```
Send the access token as `Authorization: Bearer <access_token>` on every route below marked with a role.

## Authors

### `GET /authors` — public
Paginated list.

### `GET /authors/{id}` — public
```json
{ "id": 1, "first_name": "J.R.R.", "last_name": "Tolkien", "middle_name": null, "image": null, "date_of_birth": "1892-01-03T00:00:00Z", "biography": null, "email": null }
```

### `GET /authors/{id}/books` — public
Paginated list of that author's `BookResponse`s (see Books below) — each book's own `authors` field is omitted here (already implied by context).

### `POST /authors` — `librarian`, `admin`
```json
{
  "first_name": "J.R.R.", "last_name": "Tolkien",
  "middle_name": null, "image": null,
  "date_of_birth": "1892-01-03",
  "biography": null, "phone": null, "email": null
}
```
Only `first_name`/`last_name` are required. `date_of_birth` is `YYYY-MM-DD`.

### `PUT /authors/{id}` — `librarian`, `admin`
Same shape as create, all fields optional (partial update).

### `DELETE /authors/{id}` — `admin`

## Books

### `GET /books` — public
Paginated list of `BookResponse`.

### `GET /books/{id}` — public
```json
{
  "id": 1, "title": "The Hobbit", "isbn": "9780547928227", "genre": "Fantasy",
  "description": null, "cover_image": null, "published_year": 1937,
  "total_copies": 3, "available_copies": 2, "is_digital": false, "file_format": null,
  "authors": [ { "id": 1, "first_name": "J.R.R.", "last_name": "Tolkien", "...": "..." } ]
}
```

### `GET /search?q=&genre=&year=&page=&limit=` — public
Same paginated `BookResponse` shape as `/books`. `q` does a MySQL full-text
match against title+description; `genre` and `year` are exact-match filters.
Author and format filters are not implemented yet (Step 13 is partial — see
`Server/docs/Steps.md`).

### `POST /books` — `librarian`, `admin`
```json
{
  "title": "The Hobbit", "isbn": "9780547928227", "genre": "Fantasy",
  "description": null, "cover_image": null, "published_year": 1937,
  "total_copies": 3, "is_digital": false,
  "author_ids": [1], "author_roles": ["primary"]
}
```
`author_ids` is required (min 1) — every author must already exist.
`author_roles` is optional and positional (index-matched to `author_ids`);
missing entries default to `"primary"`. `available_copies` is set equal to
`total_copies` on creation, not accepted as input.

### `PUT /books/{id}` — `librarian`, `admin`
Same shape as create minus `author_ids`/`author_roles` (use the assign/remove
endpoints below for those), all fields optional. Changing `total_copies`
adjusts `available_copies` by the same delta.

### `DELETE /books/{id}` — `admin`

### `POST /books/{id}/authors` — `librarian`, `admin`
```json
{ "author_id": 2, "role": "co-author" }
```
`role` is one of `primary`, `co-author`, `editor`, `illustrator`.

### `DELETE /books/{id}/authors/{authorId}` — `librarian`, `admin`
Refuses (`400`) if it would leave the book with zero authors.

## `GET /health` — public, no auth, no rate limit
```json
{ "status": "ok", "service": "bibliotheca" }
```
Does not currently check DB/Redis liveness (see `Server/docs/Steps.md` → Step 20).

## Not implemented yet

Borrowing, reading sessions/bookmarks, user/member management, and e-library
upload/download have no routes yet — see `Server/docs/Steps.md` (Steps 14-17)
for the planned shape of each.
