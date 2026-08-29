> Status as of 2026-08-29: **Steps 1-2 done, Steps 3-7 not started.** This is `Client/web/app`'s own roadmap — Steps 1-7 below, numbered fresh from 1. It is a **separate roadmap from the Server's** (`Server/app/docs/Steps.md`, Steps 1-45) — don't confuse the two. See [`web-app-plan.md`](web-app-plan.md) for the full architecture reasoning (auth strategy, `proxy.js` naming, API base URLs, visual identity) behind the steps below. One step, one branch — same discipline the Server roadmap uses.

```
✅ Step 1 — Foundations
             Layout shell (src/app/layout.js), the visual identity from
             web-app-plan.md: gold/navy/royal-blue palette as CSS custom
             properties (light via prefers-color-scheme default, dark
             override), three next/font/google fonts (UnifrakturMaguntia
             for the wordmark only, Cinzel for headings, EB Garamond for
             body/UI).
             Shared components: Container, Button, Navbar, Footer,
             Logo (blackletter wordmark). Navbar/Footer deliberately
             minimal — no nav links yet, since no other pages exist until
             Step 2+.
             lib/api.js: server-only fetch wrapper — attaches
             Authorization: Bearer <token> from the session cookie (a
             no-op until Step 2 adds it), unwraps the
             {success,message,data} envelope into a typed ApiError on
             failure. No auth logic yet — proved against the real public
             GET /api/v1/books endpoint from the home page.
             Env wiring: new API_INTERNAL_URL (server-only, defaults to
             http://app:8080 in Docker / http://localhost:8080 bare)
             alongside the existing NEXT_PUBLIC_API_URL
             (WEB_APP_API_URL in docker-compose.yml) — added to
             docker-compose.yml's web-app environment block and to a new
             Client/web/app/.env.example.
             Verified end-to-end: go build/vet clean; npm run lint and
             npm run build both clean (home route correctly renders as
             ƒ/Dynamic, since lib/api.js's cookies() call forces dynamic
             rendering); ran the real Go API + `next dev` together and
             confirmed the home page rendered "3 books in the catalog" —
             the real seeded count, fetched server-side through the new
             wrapper — with all three fonts loading (blackletter logo,
             Cinzel/Garamond elsewhere) and the palette applied in both
             light and dark.

✅ Step 2 — Auth
             Register/login/logout pages (src/app/register,src/app/login),
             each posting to a Server Action in src/app/actions/auth.js
             that calls:
               POST /api/v1/auth/register
               POST /api/v1/auth/login
               POST /api/v1/auth/logout
             On success, sets access_token + refresh_token as httpOnly
             cookies (secure in production only — plain http on localhost
             can't carry a Secure cookie, sameSite: lax) via the new
             src/lib/session.js, per web-app-plan.md — the access token's
             cookie Max-Age matches JWT_ACCESS_TOKEN_TTL (15 min default),
             the refresh token's matches JWT_REFRESH_TOKEN_TTL (7 days
             default, hardcoded client-side since the token response
             doesn't carry it).
             Found and fixed a real bug in the Go API while wiring this
             up: Server/app/internal/modules/auth's issueTokens() returned
             ExpiresIn using the *refresh* token's TTL (604800s) for every
             auth response, not the access token's (900s) — the JWT itself
             was always correctly minted with a 15-minute expiry, only the
             reported expires_in (and therefore this cookie's Max-Age) was
             wrong. Fixed by adding jwt.Manager.AccessTokenTTL() and using
             it in issueTokens instead of s.refreshTTL. Caught by testing
             the real registration response, not by reading the code.
             lib/api.js gains the transparent-refresh behavior: on a 401
             from a request that carried an access token, POST
             /api/v1/auth/refresh once, re-set both cookies from the
             rotated response, retry the original request once before
             giving up. A shared in-flight refresh promise (module-level,
             per server process) avoids two concurrent 401s both racing to
             use the same one-time-use refresh token.
             proxy.js (NOT middleware.js — this Next.js version renamed
             it, see web-app-plan.md): optimistic redirect to /login for
             protected routes when no session cookie is present. Reads
             the cookie only, no API round-trip inside the proxy itself.
             Also redirects an already-authenticated visitor away from
             /login and /register.
             Minimal src/app/account page added (not the real Step 5
             profile UI, just a temporary GET /api/v1/users/me display) to
             prove proxy.js + lib/api.js's Authorization attachment work
             together end-to-end; Navbar now shows Account/Sign-in based
             on session presence.
             Verified end-to-end against the real Go API (not mocked):
             registered a real user via the actual rendered form (curl
             submitting the real progressively-enhanced multipart POST,
             hidden Server Action fields included) — confirmed correct
             access/refresh cookie Max-Age values, confirmed
             GET /account showed the real registered user's data,
             confirmed proxy.js redirected an unauthenticated /account
             request to /login and an authenticated /login request to /,
             confirmed a wrong-password login attempt surfaced "Invalid
             email or password" inline via useActionState instead of
             crashing, confirmed logout cleared the session and
             subsequent /account requests redirected to /login again, and
             confirmed a fresh login after logout worked.

⏳ Step 3 — Catalog browsing
             Home/catalog page: paginated book listing
             (GET /api/v1/books, utils.Pagination-shaped response),
             search (GET /api/v1/search).
             Book detail page (GET /api/v1/books/{id}): title, authors,
             genre, format (physical/digital via is_digital), copy
             availability.
             Author browsing (GET /api/v1/authors) and author detail page
             (GET /api/v1/authors/{id}, GET /api/v1/authors/{id}/books).
             Public/unauthenticated where the API allows it (these are
             the catalog's public GET routes) — no proxy.js gating here.

⏳ Step 4 — Borrowing
             "My borrows" list (GET /api/v1/borrows/my).
             Borrow a book from its detail page
             (POST /api/v1/borrows) — reflects the real available_copies
             count from Step 3's book detail fetch, not a client-side
             guess.
             Return a book (PATCH /api/v1/borrows/{id}/return).
             Due-date and overdue display sourced directly from the
             borrow record the API returns (no client-side date math
             duplicating the server's overdue sweep).

⏳ Step 5 — Profile & personal library
             Profile view/edit (GET /api/v1/users/me,
             PATCH /api/v1/users/me).
             Personal library shelf (GET /api/v1/users/me/library,
             PATCH /api/v1/users/me/library/{bookId} for status updates).
             Reading history (GET /api/v1/users/me/history).

⏳ Step 6 — E-library reading experience
             Download flow (GET /api/v1/books/{id}/download — any
             authenticated user per the API's own annotation).
             An actual in-browser reader — the biggest single UI lift in
             this roadmap, format-dependent rendering (PDF/EPUB); library
             choice and exact approach decided when this step starts,
             not locked in here.
             Reading-progress sync (PATCH /api/v1/reading/{bookId}/sync,
             GET /api/v1/reading/{bookId}/session,
             PATCH /api/v1/reading/{bookId}/progress).
             Bookmarks CRUD (GET/POST /api/v1/reading/{bookId}/bookmarks,
             DELETE /api/v1/reading/{bookId}/bookmarks/{id}).
             This step's own light/dark/sepia reading-theme toggle,
             scoped to the reader view (see web-app-plan.md — deliberately
             not an app-wide switcher built earlier than this).

⏳ Step 7 — Polish
             Loading/error-state audit across Steps 1-6 (every
             fetch/Server Action needs a real pending + error UI, not
             just the happy path).
             Responsive/accessibility pass.
             Deliberately left lighter/open-ended until Steps 1-6 exist
             to actually audit — mirrors how the Server's own Step 20
             worked (most of it turned out already done by earlier
             steps).
```

## Out of scope for this roadmap

Librarian/admin-only endpoints — author/book write endpoints
(`POST`/`PUT`/`DELETE /api/v1/authors`, `/books`, the book-authors junction
endpoints, and `POST /api/v1/books/{id}/upload`), all-borrows listing
(`GET /api/v1/borrows`), and user-management endpoints
(`GET /api/v1/users`, `PATCH /api/v1/users/{id}/status`) — belong to
`Client/admin` + `Server/admin`, not `web/app`. None of the steps above
touch them.
