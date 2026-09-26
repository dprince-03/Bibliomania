// gateway: the single edge in front of the seven services — GraphQL for
// web/app, the versioned REST API (reverse-proxied to each owning service),
// Swagger UI, and whole-system /health.
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	borrowv1 "github.com/dprince-03/Bibliomania/gen/borrow/v1"
	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	readingv1 "github.com/dprince-03/Bibliomania/gen/reading/v1"
	userv1 "github.com/dprince-03/Bibliomania/gen/user/v1"
	"github.com/dprince-03/Bibliomania/internal/apiversion"
	"github.com/dprince-03/Bibliomania/internal/config"
	"github.com/dprince-03/Bibliomania/internal/gateway"
	"github.com/dprince-03/Bibliomania/internal/gateway/graph"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/server"
	"github.com/dprince-03/Bibliomania/internal/telemetry"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
	"github.com/dprince-03/Bibliomania/pkg/redisclient"

	"google.golang.org/grpc"

	_ "github.com/dprince-03/Bibliomania/internal/swaggerdocs"
)

//	@title			Bibliomania API
//	@version		1.0
//	@description	Library Management & E-Library System — REST API for authentication, the book catalog, borrowing, reading progress, member management and purchases.
//	@description	Served by the gateway, which routes each path to the microservice that owns it. All endpoints except /health and /auth/* return the shared JSON envelope: {"success", "message"|"error", "data"|"code"}.
//	@description	Versioned by URL path (/api/v1). GET /api/versions lists served versions; a deprecated version's responses carry Deprecation/Sunset headers. A GraphQL API is also available at POST /graphql.

//	@contact.name	Bibliomania
//	@contact.url	https://github.com/dprince-03/Bibliomania

//	@license.name	MIT

//	@host		localhost:8080
//	@BasePath	/api/v1

//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and the JWT access token (obtained from /auth/login or /auth/register).

func main() {
	ctx := context.Background()

	cfg, err := config.Load("gateway")
	server.Must(err, "config error")
	server.Must(cfg.Require("JWT_SECRET", "CATALOG_GRPC_ADDR", "BORROW_GRPC_ADDR", "READING_GRPC_ADDR", "USER_GRPC_ADDR"), "config error")

	shutdown, err := telemetry.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	server.Must(err, "telemetry init failed")
	defer func() { _ = shutdown(context.Background()) }()

	dial := func(addr string) *grpc.ClientConn {
		conn, err := grpcx.Dial(addr)
		server.Must(err, "grpc client error")
		return conn
	}
	catalogConn, borrowConn := dial(cfg.CatalogGRPCAddr), dial(cfg.BorrowGRPCAddr)
	readingConn, userConn := dial(cfg.ReadingGRPCAddr), dial(cfg.UserGRPCAddr)
	defer catalogConn.Close()
	defer borrowConn.Close()
	defer readingConn.Close()
	defer userConn.Close()

	// Redis: idempotency records + distributed rate limiting.
	redisClient, err := redisclient.Connect(fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort), cfg.RedisPassword, cfg.RedisDB)
	server.Must(err, "redis error")
	defer redisClient.Close()

	versions, err := apiversion.NewRegistry(gateway.ServedVersions, cfg.APIDeprecations)
	server.Must(err, "api version config error")

	handler, err := gateway.NewHandler(gateway.Deps{
		Config:   cfg,
		JWT:      jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL),
		Redis:    redisClient,
		Versions: versions,
		Resolver: &graph.Resolver{
			Catalog: catalogv1.NewCatalogServiceClient(catalogConn),
			Borrow:  borrowv1.NewBorrowServiceClient(borrowConn),
			Reading: readingv1.NewReadingServiceClient(readingConn),
			User:    userv1.NewUserServiceClient(userConn),
		},
		HTTPClient: &http.Client{Timeout: 3 * time.Second},
	})
	server.Must(err, "gateway setup failed")

	server.Must(server.Run(ctx, server.Service{Config: cfg, Handler: handler}), "server error")
}
