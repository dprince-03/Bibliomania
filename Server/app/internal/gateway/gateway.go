// Package gateway is the single edge every client request crosses
// (nginx → gateway → services). It serves:
//
//   - POST /graphql — the GraphQL API for web/app (internal/gateway/graph);
//   - /api/{version}/... — the REST API, reverse-proxied path-for-path to
//     the service that owns each route, so REST clients (web/app today,
//     Client/admin, mobile, desktop) keep one base URL and the exact
//     pre-split contract;
//   - GET /api/v1/users/me/library — the one REST route the gateway
//     answers itself, because it aggregates two services;
//   - /api/versions, /health (every service's), /metrics, /swagger/.
//
// Edge concerns live here and only here: CORS, security headers, the
// global per-IP rate limit, the strict /auth rate limit, API-version
// validation.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	userv1 "github.com/dprince-03/Bibliomania/gen/user/v1"
	"github.com/dprince-03/Bibliomania/internal/apiversion"
	"github.com/dprince-03/Bibliomania/internal/config"
	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/gateway/graph"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/health"
	"github.com/dprince-03/Bibliomania/internal/idempotency"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/resilience"
	"github.com/dprince-03/Bibliomania/internal/server"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/jwt"

	"github.com/redis/go-redis/v9"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// ServedVersions are the REST API versions this build serves, oldest first.
// Adding "v2" here is the switch that starts routing /api/v2/... (the
// services must register their v2 routes too).
var ServedVersions = []string{"v1"}

type Deps struct {
	Config *config.Config
	JWT    *jwt.Manager
	// Redis backs the idempotency middleware and the distributed rate
	// limiter; nil disables idempotency (tests / bare runs).
	Redis      *redis.Client
	Resolver   *graph.Resolver
	Versions   *apiversion.Registry
	HTTPClient *http.Client
}

// route maps a path pattern (net/http ServeMux syntax) to the service
// that owns it.
type route struct {
	pattern string
	target  string
	strict  bool // strict auth rate limit
}

func routes(cfg *config.Config) []route {
	return []route{
		{"/api/v1/auth/", cfg.AuthHTTPURL, true},
		{"/api/v1/authors", cfg.CatalogHTTPURL, false},
		{"/api/v1/authors/", cfg.CatalogHTTPURL, false},
		{"/api/v1/books", cfg.CatalogHTTPURL, false},
		{"/api/v1/books/", cfg.CatalogHTTPURL, false},
		{"/api/v1/search", cfg.CatalogHTTPURL, false},
		{"/api/v1/borrows", cfg.BorrowHTTPURL, false},
		{"/api/v1/borrows/", cfg.BorrowHTTPURL, false},
		{"/api/v1/reading/", cfg.ReadingHTTPURL, false},
		{"/api/v1/users/me/history", cfg.ReadingHTTPURL, false},
		{"/api/v1/users", cfg.UserHTTPURL, false},
		{"/api/v1/users/", cfg.UserHTTPURL, false},
		{"/api/v1/payments/", cfg.PaymentHTTPURL, false},
	}
}

// NewHandler builds the gateway's full HTTP handler (routes + edge
// middleware). internal/server adds the standard tracing/logging/metrics
// chain around it.
func NewHandler(d Deps) (http.Handler, error) {
	cfg := d.Config
	mux := http.NewServeMux()

	// Rate limits: shared across gateway replicas through Redis, with the
	// in-memory stores as the fallback if Redis is down (or not configured).
	globalStore := middleware.NewRateLimiterStore(cfg.RateLimitRPS, cfg.RateLimitBurst, 5*time.Minute)
	// Auth routes: strict — 10 attempts per minute, burst of 5, to blunt
	// brute-force attempts.
	authStore := middleware.NewRateLimiterStore(10.0/60.0, 5, 10*time.Minute)
	globalLimit := middleware.RateLimit(globalStore)
	authLimit := middleware.RateLimit(authStore)
	if d.Redis != nil {
		globalLimit = middleware.DistributedRateLimit(d.Redis, "global", cfg.RateLimitRPS, cfg.RateLimitBurst, globalStore)
		authLimit = middleware.DistributedRateLimit(d.Redis, "auth", 10.0/60.0, 5, authStore)
	}

	// One guarded transport (breaker + bulkhead) per upstream service,
	// shared by all of that service's routes.
	upstream := otelhttp.NewTransport(http.DefaultTransport)
	transports := map[string]*guardedTransport{}
	for _, rt := range routes(cfg) {
		if transports[rt.target] == nil {
			transports[rt.target] = newGuardedTransport(rt.target, upstream)
		}
		p, err := newProxy(rt.target, transports[rt.target])
		if err != nil {
			return nil, err
		}
		var h http.Handler = p
		if rt.strict {
			h = authLimit(h)
		}
		mux.Handle(rt.pattern, h)
	}

	guards := middleware.NewGuards(d.JWT)
	mux.Handle("GET /api/v1/users/me/library", guards.Member(libraryHandler(d.Resolver)))
	mux.HandleFunc("GET /api/versions", d.Versions.Handle)

	// Anything else under /api/v1 isn't a route.
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		utils.Error(w, apperrors.NotFound("route"))
	})

	// GraphQL
	gql := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: d.Resolver}))
	gql.AddTransport(transport.Options{})
	gql.AddTransport(transport.GET{})
	gql.AddTransport(transport.POST{})
	gql.Use(extension.FixedComplexityLimit(300))
	gql.Use(graph.DepthLimit{Max: 8}) // deepest real query today is 4
	if !cfg.IsProduction() {
		gql.Use(extension.Introspection{})
		mux.Handle("GET /playground", playground.Handler("Bibliomania GraphQL", "/graphql"))
	}
	gql.SetErrorPresenter(graph.ErrorPresenter)
	mux.Handle("/graphql", optionalAuth(d.JWT)(gql))

	// Swagger (generated from every service's handler annotations).
	mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	// Health covers the whole system: the gateway is only "ok" if every
	// service behind it is.
	checker := health.NewChecker(cfg.ServiceName)
	for name, base := range map[string]string{
		"auth-service": cfg.AuthHTTPURL, "catalog-service": cfg.CatalogHTTPURL,
		"borrow-service": cfg.BorrowHTTPURL, "reading-service": cfg.ReadingHTTPURL,
		"user-service": cfg.UserHTTPURL, "payment-service": cfg.PaymentHTTPURL,
	} {
		checker.Add(name, health.HTTPCheck(d.HTTPClient, base+"/health"))
	}
	server.Ops(mux, checker.Handle)

	chain := []func(http.Handler) http.Handler{
		middleware.CORS(middleware.DefaultCORSConfig()),
		middleware.SecurityHeaders,
		globalLimit,
		d.Versions.Middleware,
	}
	if d.Redis != nil {
		chain = append(chain, idempotency.New(d.Redis, callerOf(d.JWT)).Handler)
	}
	return middleware.Chain(mux, chain...), nil
}

// callerOf scopes idempotency keys: the JWT subject for authenticated
// requests, "anon:<ip>" otherwise (register/login), so callers never share
// a key space.
func callerOf(jwtManager *jwt.Manager) idempotency.CallerFunc {
	return func(r *http.Request) string {
		if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			if claims, err := jwtManager.ParseAccessToken(token); err == nil {
				return fmt.Sprintf("user:%d", claims.UserID)
			}
		}
		return "anon:" + middleware.ClientIP(r)
	}
}

// newProxy forwards the request unchanged (same path, headers, body) to
// target. The caller's Authorization header goes along — each service
// verifies the JWT itself. Streams bodies both ways, so uploads and
// book downloads aren't buffered in the gateway.
func newProxy(target string, rt http.RoundTripper) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(u)
			pr.Out.Host = pr.In.Host
			pr.SetXForwarded()
		},
		Transport:     rt,
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			switch {
			case errors.Is(err, resilience.ErrOpen):
				// Failing fast: the service has been failing; don't wait on it.
				w.Header().Set("Retry-After", "10")
				utils.Error(w, &apperrors.AppError{Code: http.StatusServiceUnavailable, Message: "service temporarily unavailable, retry shortly"})
			case errors.Is(err, resilience.ErrBulkheadFull):
				w.Header().Set("Retry-After", "1")
				utils.Error(w, &apperrors.AppError{Code: http.StatusServiceUnavailable, Message: "service is overloaded, retry shortly"})
			default:
				slog.ErrorContext(r.Context(), "upstream unreachable", "target", target, "path", r.URL.Path, "error", err)
				utils.Error(w, &apperrors.AppError{Code: http.StatusBadGateway, Message: "upstream service unavailable"})
			}
		},
	}, nil
}

// optionalAuth verifies a bearer token if one is sent (401 if it's
// invalid, so the client knows to refresh) and lets anonymous requests
// through — GraphQL resolvers decide per field what needs a user.
func optionalAuth(jwtManager *jwt.Manager) func(http.Handler) http.Handler {
	guard := middleware.AuthGuard(jwtManager)
	return func(next http.Handler) http.Handler {
		guarded := guard(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
				return
			}
			guarded.ServeHTTP(w, r)
		})
	}
}

// libraryEntry is GET /api/v1/users/me/library's item shape — identical to
// the pre-split LibraryEntryResponse.
type libraryEntry struct {
	BookID    uint64    `json:"book_id"`
	BookTitle string    `json:"book_title"`
	Status    string    `json:"status"`
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// libraryHandler godoc
//
//	@Summary		Get my library
//	@Description	Aggregated by the gateway: shelf entries from user-service, titles resolved live (one batch call) from catalog-service. If catalog-service is down the shelf is still returned, with empty titles and an X-Degraded: catalog-service header.
//	@Tags			users
//	@Produce		json
//	@Param			status	query		string	false	"Filter to one status"	Enums(wishlist, to_read, reading, completed, dropped)
//	@Param			page	query		int		false	"Page number"			default(1)
//	@Param			limit	query		int		false	"Items per page"		default(10)
//	@Success		200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]libraryEntry}}
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Security		BearerAuth
//	@Router			/users/me/library [get]
func libraryHandler(res *graph.Resolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		pg := utils.GetPagination(r)

		shelf, err := res.User.ListLibrary(ctx, &userv1.ListLibraryRequest{
			Status: r.URL.Query().Get("status"), Page: int32(pg.Page), Limit: int32(pg.Limit),
		})
		if err != nil {
			utils.HandleError(w, grpcx.FromStatus(err))
			return
		}

		// Graceful degradation: the shelf is user-service's data and still
		// worth returning if catalog-service is down — just without titles,
		// flagged so the client can say so.
		message := "library retrieved"
		titles, err := bookTitles(ctx, res.Catalog, shelf.GetEntries())
		if err != nil {
			slog.WarnContext(ctx, "catalog unavailable — library returned without titles", "error", err)
			w.Header().Set("X-Degraded", "catalog-service")
			message = "library retrieved (book titles temporarily unavailable)"
			titles = map[uint64]string{}
		}

		items := make([]libraryEntry, 0, len(shelf.GetEntries()))
		for _, e := range shelf.GetEntries() {
			items = append(items, libraryEntry{
				BookID:    e.GetBookId(),
				BookTitle: titles[e.GetBookId()],
				Status:    e.GetStatus(),
				AddedAt:   e.GetAddedAt().AsTime(),
				UpdatedAt: e.GetUpdatedAt().AsTime(),
			})
		}
		utils.Success(w, http.StatusOK, message,
			utils.NewPaginatedResponse(items, int(shelf.GetTotalCount()), pg.Page, pg.Limit))
	}
}

func bookTitles(ctx context.Context, catalog catalogv1.CatalogServiceClient, entries []*userv1.LibraryEntry) (map[uint64]string, error) {
	titles := map[uint64]string{}
	if len(entries) == 0 {
		return titles, nil
	}
	ids := make([]uint64, len(entries))
	for i, e := range entries {
		ids[i] = e.GetBookId()
	}
	books, err := catalog.GetBooksByIds(ctx, &catalogv1.GetBooksByIdsRequest{BookIds: ids})
	if err != nil {
		return nil, grpcx.FromStatus(err)
	}
	for _, b := range books.GetBooks() {
		titles[b.GetId()] = b.GetTitle()
	}
	return titles, nil
}
