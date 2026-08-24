package router

import (
	"net/http"
	"time"

	"github.com/dprince-03/Bibliotheca/internal/config"
	"github.com/dprince-03/Bibliotheca/internal/middleware"
	"github.com/dprince-03/Bibliotheca/internal/modules/auth"
	"github.com/dprince-03/Bibliotheca/internal/modules/catalog"
	"github.com/dprince-03/Bibliotheca/pkg/jwt"
)

func New(
	cfg *config.Config,
	jwtManager *jwt.Manager,
	authHandler *auth.Handler,
	authorHandler *catalog.AuthorHandler,
	bookHandler *catalog.BookHandler,
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

	_, _ = requireAdmin, requireMember

	// ── Health (public, no rate limit) ────────
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"bibliotheca"}`))
	})

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

	// ── Future route groups (added each step) ──
	//
	// Search (Step 13):
	//   GET    /api/v1/search             → public
	//
	// E-Library (Step 14):
	//   POST   /api/v1/books/:id/upload   → requireLibrarian
	//   GET    /api/v1/books/:id/download → requireMember
	//
	// Reading (Step 15):
	//   GET    /api/v1/reading/:bookId/session    → requireMember
	//   PATCH  /api/v1/reading/:bookId/progress   → requireMember
	//   PATCH  /api/v1/reading/:bookId/sync       → requireMember
	//   GET    /api/v1/reading/:bookId/bookmarks  → requireMember
	//   POST   /api/v1/reading/:bookId/bookmarks  → requireMember
	//   DELETE /api/v1/reading/:bookId/bookmarks/:id → requireMember
	//
	// Borrows (Step 16):
	//   GET    /api/v1/borrows            → requireLibrarian
	//   GET    /api/v1/borrows/my         → requireMember
	//   POST   /api/v1/borrows            → requireMember
	//   PATCH  /api/v1/borrows/:id/return → requireMember
	//
	// Users (Step 17):
	//   GET    /api/v1/users/me           → requireMember
	//   PATCH  /api/v1/users/me          → requireMember
	//   GET    /api/v1/users/me/library   → requireMember
	//   PATCH  /api/v1/users/me/library/:bookId → requireMember
	//   GET    /api/v1/users/me/history   → requireMember
	//   GET    /api/v1/users             → requireAdmin
	//   PATCH  /api/v1/users/:id/status  → requireAdmin

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
