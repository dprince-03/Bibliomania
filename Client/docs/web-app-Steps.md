> Status as of 2026-08-29: **Steps 1-4 done, Steps 5-7 not started.** This is `Client/web/app`'s own roadmap — Steps 1-7 below, numbered fresh from 1. It is a **separate roadmap from the Server's** (`Server/app/docs/Steps.md`, Steps 1-45) — don't confuse the two. See [`web-app-plan.md`](web-app-plan.md) for the full architecture reasoning (auth strategy, `proxy.js` naming, API base URLs, visual identity) behind the steps below. One step, one branch — same discipline the Server roadmap uses.

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

✅ Step 3 — Catalog browsing
             Home page (src/app/page.js) rebuilt as the real catalog: a
             plain GET search form (no client JS — SearchForm.js submits
             to / with q/genre/format query params, always resetting to
             page 1), paginated book listing (GET /api/v1/books when no
             filters, GET /api/v1/search when any are present), rendered
             via new BookCard/Pagination components.
             Book detail page (src/app/books/[id], GET /api/v1/books/{id}):
             title, linked authors, genre, format (physical/digital via
             is_digital), copy availability; 404s via next/navigation's
             notFound() on a real 404 from the API, caught by checking
             ApiError.status. No borrow/read actions yet — those are
             Steps 4/6.
             Author browsing (src/app/authors, GET /api/v1/authors) and
             author detail page (src/app/authors/[id], GET
             /api/v1/authors/{id} + GET /api/v1/authors/{id}/books, its
             own pagination) via new AuthorCard component.
             New src/app/not-found.js styled to match the app (blackletter
             logo/palette) instead of Next's generic 404, since notFound()
             is now actually used.
             Navbar gained real links (Catalog, Authors) now that these
             pages exist — no proxy.js gating on any of this, matching the
             API's own public GET routes.

             Found and fixed a real bug while verifying search: MySQL's
             InnoDB FULLTEXT index caches newly-inserted rows in memory
             (innodb_ft_cache_size) and only merges them into the on-disk
             index on a size threshold or an explicit OPTIMIZE TABLE, not
             immediately on insert — so GET /api/v1/search found nothing
             for books that had just been seeded, even searching their
             exact title. Confirmed directly in MySQL (MATCH/AGAINST
             scored 0 for every row until OPTIMIZE TABLE books; scored
             correctly after). Fixed in Server/app/cmd/seed/main.go:
             seedCatalog now reports whether it actually inserted new
             rows, and main() runs OPTIMIZE TABLE books once after a real
             seed (skipped entirely on an already-seeded database).
             Verified end-to-end against the real Go API: catalog listing
             showed all 3 seeded books; title search ("Hobbit"), author-
             name search ("Herbert" → Dune), and genre filter (Fantasy
             correctly excluding Dune) all returned correct results after
             the fix; book detail page showed real author/genre/
             availability data and a nonexistent book id correctly 404'd;
             author listing and author detail (with their books,
             paginated) both showed real seeded data.

✅ Step 4 — Borrowing
             New src/app/actions/borrow.js Server Actions (borrowAction,
             returnAction), routed through lib/api.js (unlike auth.js's raw
             fetch — these endpoints are auth-required and benefit from its
             Authorization header + transparent refresh-and-retry):
               POST /api/v1/borrows
               PATCH /api/v1/borrows/{id}/return
             src/app/books/[id]/page.js's placeholder text replaced with a
             real BorrowForm (new src/components/BorrowForm.js, a client
             component using useActionState): shown only when a session
             cookie is present (getRefreshToken(), same check Navbar
             already uses), disabled with an inline message when
             book.available_copies is 0 for a physical book — reflects the
             real availability from Step 3's book detail fetch, not a
             client-side guess. A signed-out visitor sees a "Sign in to
             borrow this book" prompt instead of the form.
             New src/app/borrows/page.js ("My borrows"): paginated
             GET /api/v1/borrows/my, listing each borrow's book title
             (linked to its detail page), borrowed/due/returned dates, and
             a status badge (active/overdue/returned — colors keyed
             directly off the API's own status string, no client-side
             overdue math). New src/components/ReturnForm.js (client
             component) renders a Return button per row, hidden once
             status is "returned".
             proxy.js's protectedRoutes gained /borrows (member-only,
             same as /account). Navbar gained a "My Borrows" link, shown
             next to the Account button whenever a session is present.
             Verified end-to-end against the real Go API (not mocked, via
             the same curl-driven progressive-enhancement Server Action
             flow used in Step 2): registered a fresh user, borrowed The
             Hobbit (id 7, 3 copies) from its real rendered detail page —
             confirmed the redirect to /borrows, confirmed available_copies
             dropped to 2/3 via a direct API call, confirmed the borrows
             list showed the correct due date (borrowed_at + 14 days) and
             an "active" badge. Attempted to borrow the same book again and
             confirmed the API's 409 ("you already have an active borrow
             for this book") rendered inline via useActionState instead of
             crashing or redirecting. Submitted the real Return form and
             confirmed the badge flipped to "returned", the Return button
             disappeared, and available_copies restored to 3/3. Confirmed
             an unauthenticated request to /borrows redirected to /login
             (proxy.js) and that the signed-out book detail page rendered
             the sign-in prompt instead of the borrow form.

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
