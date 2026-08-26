package router

import (
	"net/http"
	"time"

	"github.com/dprince-03/Bibliotheca/internal/config"
	"github.com/dprince-03/Bibliotheca/internal/middleware"
	"github.com/dprince-03/Bibliotheca/internal/modules/auth"
	"github.com/dprince-03/Bibliotheca/internal/modules/borrow"
	"github.com/dprince-03/Bibliotheca/internal/modules/catalog"
	"github.com/dprince-03/Bibliotheca/internal/modules/reading"
	"github.com/dprince-03/Bibliotheca/internal/modules/user"
	"github.com/dprince-03/Bibliotheca/pkg/jwt"

	httpSwagger "github.com/swaggo/http-swagger"
)

func New(
	cfg *config.Config,
	jwtManager *jwt.Manager,
	authHandler *auth.Handler,
	authorHandler *catalog.AuthorHandler,
	bookHandler *catalog.BookHandler,
	readingHandler *reading.Handler,
	borrowHandler *borrow.Handler,
	userHandler *user.Handler,
) http.Handler {
	mux := http.NewServeMux()

	// ── Rate limiter stores ────────────────────
	// Global: from config (default 100 rps, burst 200)
	globalStore := middleware.NewRateLimiterStore(
		cfg.RateLimitRPS,
		cfg.RateLimitBurst,
		5*time.Minute,
	)

	// Auth routes: strict — 10 attempts per minute, burst of 5
	// Protects against brute-force login attacks
	authStore := middleware.NewRateLimiterStore(
		10.0/60.0, // 10 requests per 60 seconds
		5,
		10*time.Minute,
	)

	// ── CORS ──────────────────────────────────
	corsMiddleware := middleware.CORS(middleware.DefaultCORSConfig())

	// ── Auth middleware helpers ────────────────
	requireAuth := middleware.AuthGuard(jwtManager)

	requireAdmin := func(h http.HandlerFunc) http.Handler {
		return requireAuth(middleware.RoleRequired(middleware.RoleAdmin)(h))
	}

	requireLibrarian := func(h http.HandlerFunc) http.Handler {
		return requireAuth(
			middleware.RoleRequired(middleware.RoleAdmin, middleware.RoleLibrarian)(h),
		)
	}

	requireMember := func(h http.HandlerFunc) http.Handler {
		return requireAuth(
			middleware.RoleRequired(
				middleware.RoleAdmin,
				middleware.RoleLibrarian,
				middleware.RoleMember,
			)(h),
		)
	}

	// ── Health (public, no rate limit) ────────
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"bibliotheca"}`))
	})

	// ── Swagger / OpenAPI docs (Step 18, public) ──
	// httpSwagger.Handler parses r.RequestURI itself to find the sub-path
	// (index.html, doc.json, static assets), so this trailing-slash prefix
	// route is enough — no http.StripPrefix needed.
	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// ── Auth (public, STRICT rate limit) ──────
	// Each auth route gets the strict limiter applied individually
	authRateLimit := middleware.RateLimit(authStore)

	mux.Handle("POST /api/v1/auth/register", authRateLimit(http.HandlerFunc(authHandler.Register)))
	mux.Handle("POST /api/v1/auth/login", authRateLimit(http.HandlerFunc(authHandler.Login)))
	mux.Handle("POST /api/v1/auth/logout", authRateLimit(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("POST /api/v1/auth/refresh", authRateLimit(http.HandlerFunc(authHandler.RefreshToken)))

	// ── Authors ───────────────────────────────
	mux.HandleFunc("GET /api/v1/authors", authorHandler.GetAll)
	mux.HandleFunc("GET /api/v1/authors/{id}", authorHandler.GetByID)
	mux.HandleFunc("GET /api/v1/authors/{id}/books", authorHandler.GetBooksByAuthor)
	mux.Handle("POST /api/v1/authors", requireLibrarian(authorHandler.Create))
	mux.Handle("PUT /api/v1/authors/{id}", requireLibrarian(authorHandler.Update))
	mux.Handle("DELETE /api/v1/authors/{id}", requireAdmin(authorHandler.Delete))

	// ── Books ─────────────────────────────────
	mux.HandleFunc("GET /api/v1/books",
		bookHandler.GetAll) // public
	mux.HandleFunc("GET /api/v1/books/{id}",
		bookHandler.GetByID) // public
	mux.HandleFunc("GET /api/v1/search",
		bookHandler.Search) // public
	mux.Handle("POST /api/v1/books",
		requireLibrarian(bookHandler.Create)) // librarian+
	mux.Handle("PUT /api/v1/books/{id}",
		requireLibrarian(bookHandler.Update)) // librarian+
	mux.Handle("DELETE /api/v1/books/{id}",
		requireAdmin(bookHandler.Delete)) // admin only
	mux.Handle("POST /api/v1/books/{id}/authors",
		requireLibrarian(bookHandler.AssignAuthor)) // librarian+
	mux.Handle("DELETE /api/v1/books/{id}/authors/{authorId}",
		requireLibrarian(bookHandler.RemoveAuthor))

	// ── E-Library (Step 14) ───────────────────
	mux.Handle("POST /api/v1/books/{id}/upload",
		requireLibrarian(bookHandler.Upload)) // librarian+
	mux.Handle("GET /api/v1/books/{id}/download",
		requireMember(bookHandler.Download)) // member+ (i.e. any authenticated user)

	// ── Reading (Step 14 + 15) ─────────────────
	mux.Handle("PATCH /api/v1/reading/{bookId}/sync",
		requireMember(readingHandler.Sync))
	mux.Handle("GET /api/v1/reading/{bookId}/session",
		requireMember(readingHandler.GetSession))
	mux.Handle("PATCH /api/v1/reading/{bookId}/progress",
		requireMember(readingHandler.UpdateProgress))
	mux.Handle("GET /api/v1/reading/{bookId}/bookmarks",
		requireMember(readingHandler.GetBookmarks))
	mux.Handle("POST /api/v1/reading/{bookId}/bookmarks",
		requireMember(readingHandler.CreateBookmark))
	mux.Handle("DELETE /api/v1/reading/{bookId}/bookmarks/{id}",
		requireMember(readingHandler.DeleteBookmark))

	// ── Borrows (Step 16) ──────────────────────
	mux.Handle("GET /api/v1/borrows",
		requireLibrarian(borrowHandler.GetAll)) // librarian, admin
	mux.Handle("GET /api/v1/borrows/my",
		requireMember(borrowHandler.GetMyBorrows))
	mux.Handle("POST /api/v1/borrows",
		requireMember(borrowHandler.Borrow))
	mux.Handle("PATCH /api/v1/borrows/{id}/return",
		requireMember(borrowHandler.Return)) // ownership enforced in the service (self, or librarian/admin)

	// ── Users (Step 17) ────────────────────────
	mux.Handle("GET /api/v1/users/me",
		requireMember(userHandler.GetMe))
	mux.Handle("PATCH /api/v1/users/me",
		requireMember(userHandler.UpdateMe))
	mux.Handle("GET /api/v1/users/me/library",
		requireMember(userHandler.GetLibrary))
	mux.Handle("PATCH /api/v1/users/me/library/{bookId}",
		requireMember(userHandler.UpdateLibraryStatus))
	mux.Handle("GET /api/v1/users/me/history",
		requireMember(userHandler.GetHistory))
	mux.Handle("GET /api/v1/users",
		requireAdmin(userHandler.GetAll))
	mux.Handle("PATCH /api/v1/users/{id}/status",
		requireAdmin(userHandler.UpdateStatus))

	// ── Global middleware (wraps everything) ──
	// Global rate limiter is part of the chain here
	return middleware.Chain(
		mux,
		middleware.RequestID,
		middleware.Logger,
		middleware.Recovery,
		corsMiddleware,
		middleware.SecurityHeaders,
		middleware.RateLimit(globalStore),
	)
}
