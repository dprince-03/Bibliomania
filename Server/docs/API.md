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

### `GET /search?q=&genre=&format=&author=&year=&page=&limit=` — public
Same paginated `BookResponse` shape as `/books`. All params are optional and
independent of each other:
- `q` — free text. Matches book title/description (MySQL full-text) **or**
  author first/last name — e.g. `q=Rowling` finds her books even if that
  word appears nowhere in the title or description.
- `genre` — exact match.
- `format` — `digital` or `physical` (maps to `is_digital`); any other value
  is a `400`.
- `author` — an author **ID** (not a name — use `q` for name search); a
  non-numeric value is a `400`.
- `year` — exact match on `published_year`.

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

### `POST /books/{id}/upload` — `librarian`, `admin`
`multipart/form-data` with a single field named `file`. Only `.pdf`/`.epub`
are accepted (the DB column is `ENUM('pdf','epub')` — anything else, even
formats `ValidateFileFormat` otherwise recognizes like `.mobi`/`.txt`, is a
`400`), and only up to `MAX_UPLOAD_SIZE_MB`. On success, sets
`is_digital: true` and returns the updated `BookResponse` (now including
`file_format`). Re-uploading replaces the previous file.

### `GET /books/{id}/download` — any authenticated user
Streams the file with `Content-Disposition: attachment`. `404` if the book
has no uploaded file yet.

## Reading

### `PATCH /reading/{bookId}/sync` — any authenticated user
Creates or updates the caller's own reading session for a book (there's no
`user_id` in the body — it comes from the token). Designed for offline
clients: send whatever the client has locally, including when it was last
updated there.
```json
{ "current_page": 42, "total_pages": 300, "current_chapter": "Ch. 5", "client_updated_at": "2026-08-25T02:49:00Z" }
```
`client_updated_at` is required. If the server already has a session with a
**newer** `client_updated_at` than what you send, your update is silently
discarded and the response reflects the server's existing (newer) state —
this is the "last write wins" conflict resolution, and it means the response
may not match what you just sent. `progress_pct` and `is_completed` are
computed server-side from `current_page`/`total_pages`, not accepted as
input.
```json
{ "book_id": 1, "current_page": 42, "total_pages": 300, "progress_pct": 14.0, "current_chapter": "Ch. 5", "is_completed": false, "last_read_at": "2026-08-25T02:49:00Z" }
```

### `GET /reading/{bookId}/session` — any authenticated user
Returns the caller's own `ReadingSessionResponse` for the book (same shape as
`sync`'s response). `404` if they haven't started reading it yet.

### `PATCH /reading/{bookId}/progress` — any authenticated user
The plain, always-online counterpart to `sync` — no `client_updated_at` in
the request (the server stamps "now" itself, which always beats whatever an
offline client might `sync` in later with an older timestamp):
```json
{ "current_page": 42, "total_pages": 300, "current_chapter": "Ch. 5" }
```
Same response shape as `sync`. `current_page: 0` is valid (starting a book).

### `GET /reading/{bookId}/bookmarks` — any authenticated user
Returns the caller's own bookmarks for the book as a plain array (not
paginated — bookmark counts per book are expected to be small). Bookmarks
are private to each user; this never returns another user's bookmarks for
the same book.
```json
[{ "id": 1, "book_id": 1, "page": 42, "note": "interesting part", "highlight": null, "color": "yellow" }]
```

### `POST /reading/{bookId}/bookmarks` — any authenticated user
```json
{ "page": 42, "note": "interesting part", "color": "yellow" }
```
`page` is required. `color` is one of `yellow`, `green`, `blue`, `pink`,
`purple` (or omitted).

### `DELETE /reading/{bookId}/bookmarks/{id}` — owner only
`403` if the bookmark belongs to someone else, or doesn't belong to the
`bookId` in the URL — no admin override (a bookmark is a personal note, not
shared library data).

## Borrows

### `GET /borrows` — `librarian`, `admin`
Every borrow record in the system, paginated. Sweeps overdue records first
(any `active` record past its `due_at` becomes `overdue`), so `status` here
is always current as of this request, not just as of whenever it was
originally computed.

### `GET /borrows/my` — any authenticated user
Same shape, filtered to the caller's own borrows. Same overdue sweep.

Both return `BorrowResponse` items:
```json
{ "id": 1, "book_id": 5, "book_title": "The Hobbit", "borrowed_at": "2026-08-25T04:16:38Z", "due_at": "2026-09-08T04:16:38Z", "returned_at": null, "status": "active" }
```
`status` is one of `active`, `returned`, `overdue`.

### `POST /borrows` — any authenticated user
```json
{ "book_id": 5 }
```
`409` if you already have an active (unreturned) borrow for this book. `400`
if the book has no available copies. On success, `due_at` is `now +
BORROW_LOAN_DAYS` (14 by default) and the book's `available_copies` drops by
one.

### `PATCH /borrows/{id}/return` — owner, or `librarian`/`admin`
No body. A member may only return their own borrow; librarian/admin may
return anyone's (e.g. processing a physical return at a desk). `403` for
anyone else. `409` if it's already been returned. On success,
`available_copies` goes back up by one.

## Users

### `GET /users/me` — any authenticated user
```json
{ "id": 9, "first_name": "Fiona", "last_name": "Franklin", "email": "fiona@example.com", "role": "member", "is_active": true, "phone_number": null, "bio": null, "profile_picture": null, "last_online_at": null, "total_books_read": 0, "total_pages_read": 0 }
```

### `PATCH /users/me` — any authenticated user
```json
{ "phone_number": "+15551234567", "bio": "I love reading" }
```
Only touches profile fields (`phone_number`, `bio`, `profile_picture`) — no
route changes `first_name`/`last_name`/`email` yet. Same response shape as
`GET /users/me`.

### `GET /users/me/library?status=` — any authenticated user
The caller's personal book shelf, paginated. `status` optionally filters to
one value (see below).
```json
[{ "book_id": 6, "book_title": "Dune", "status": "reading", "added_at": "2026-08-25T12:00:14Z", "updated_at": "2026-08-25T12:00:14Z" }]
```

### `PATCH /users/me/library/{bookId}` — any authenticated user
```json
{ "status": "reading" }
```
`status` is one of `wishlist`, `to_read`, `reading`, `completed`, `dropped`.
Upsert — calling it again for the same book updates the existing entry
rather than creating a duplicate. `404` if the book doesn't exist.

### `GET /users/me/history` — any authenticated user
The caller's reading activity (from `internal/modules/reading`'s session
data) — **not** the same thing as `GET /borrows/my`, which is physical/
digital checkout history. A book only appears here once there's been a
`sync` or `progress` call for it.
```json
[{ "book_id": 6, "book_title": "Dune", "current_page": 30, "total_pages": 250, "progress_pct": 12.0, "is_completed": false, "last_read_at": "2026-08-25T12:00:38Z" }]
```

### `GET /users` — `admin`
Every user, paginated — **including deactivated accounts** (unlike most
other lookups in this API, which exclude them), precisely so an admin can
find and reactivate one.
```json
{ "id": 9, "first_name": "Fiona", "last_name": "Franklin", "email": "fiona@example.com", "role": "member", "is_active": true }
```

### `PATCH /users/{id}/status` — `admin`
```json
{ "is_active": false }
```
Deactivating a user immediately blocks their next login (`401`) **and**
invalidates their already-issued access token for any endpoint that reads
the user record (e.g. `GET /users/me` starts returning `404` even though
the token itself hasn't expired) — because those lookups filter to
`is_active = TRUE`. It does not revoke the refresh token itself, but a
revoked/deactivated account can't successfully refresh either, since
`Login`/`RefreshToken` both re-check `is_active`.

## `GET /health` — public, no auth, no rate limit
```json
{ "status": "ok", "service": "bibliotheca" }
```
Does not currently check DB/Redis liveness (see `Server/docs/Steps.md` → Step 20).

## Not yet built

Steps 18-20 (Swagger/OpenAPI generation, Makefile/Docker polish, final
hardening like a real `/health` DB/Redis check) are tooling and polish, not
new endpoints — see `Server/docs/Steps.md` for what each covers.
