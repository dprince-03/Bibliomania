// user-service: profiles, the personal library shelf, admin user
// management. Postgres + RabbitMQ (publish user.status_changed, consume
// auth.user_registered) + NATS JetStream (consume reading.*).
package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	userv1 "github.com/dprince-03/Bibliomania/gen/user/v1"
	"github.com/dprince-03/Bibliomania/internal/config"
	"github.com/dprince-03/Bibliomania/internal/database"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/health"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/server"
	"github.com/dprince-03/Bibliomania/internal/services/user"
	"github.com/dprince-03/Bibliomania/internal/telemetry"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
	"github.com/dprince-03/Bibliomania/pkg/postgresclient"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load("user-service")
	server.Must(err, "config error")
	server.Must(cfg.Require("DATABASE_URL", "JWT_SECRET", "RABBITMQ_URL", "NATS_URL", "CATALOG_GRPC_ADDR"), "config error")

	shutdown, err := telemetry.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	server.Must(err, "telemetry init failed")
	defer func() { _ = shutdown(context.Background()) }()

	db, err := postgresclient.Connect(ctx, cfg.DatabaseURL)
	server.Must(err, "database error")
	defer db.Close()
	server.Must(database.Migrate(db, database.DriverPostgres, user.Migrations), "migration error")

	rabbit, err := events.DialRabbit(ctx, cfg.RabbitMQURL, cfg.RabbitMQQueueType)
	server.Must(err, "rabbitmq error")
	defer rabbit.Close()

	nc, err := events.DialNATS(ctx, cfg.NATSURL, cfg.NATSStreamReplicas)
	server.Must(err, "nats error")
	defer nc.Close()

	catalogConn, err := grpcx.Dial(cfg.CatalogGRPCAddr)
	server.Must(err, "catalog client error")
	defer catalogConn.Close()

	validate := utils.NewValidator()
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	service := user.NewService(
		user.NewRepository(db),
		user.NewProfileRepository(db),
		user.NewLibraryRepository(db),
		user.NewCatalogClient(catalogv1.NewCatalogServiceClient(catalogConn)),
	)

	mux := http.NewServeMux()
	user.NewHandler(service, validate).Routes(mux, middleware.NewGuards(jwtManager))
	server.Ops(mux, health.NewChecker(cfg.ServiceName).
		Add("database", db.PingContext).
		Add("rabbitmq", rabbit.Ping).
		Add("nats", nc.Ping).
		Handle)

	grpcServer := grpcx.NewServer(jwtManager, cfg.IsProduction())
	userv1.RegisterUserServiceServer(grpcServer, user.NewGRPCServer(service, validate.Struct))

	workers := []server.Worker{}
	if cfg.ReconcileInterval > 0 && cfg.InternalAPIToken != "" {
		reconciler := user.NewReconciler(db, user.NewAuthAccountSource(cfg.AuthHTTPURL, cfg.InternalAPIToken))
		workers = append(workers, func(ctx context.Context) { reconciler.Run(ctx, cfg.ReconcileInterval) })
	} else {
		slog.Warn("account reconciliation disabled (needs INTERNAL_API_TOKEN and RECONCILE_INTERVAL_MINUTES > 0)")
	}

	err = server.Run(ctx, server.Service{
		Config:  cfg,
		Handler: mux,
		GRPC:    grpcServer,
		Workers: append(workers,
			func(ctx context.Context) { events.RunRelay(ctx, db, rabbit, 500*time.Millisecond) },
			func(ctx context.Context) {
				rabbit.Consume(ctx, "user-service", []string{events.TypeUserRegistered}, service.OnUserRegistered)
			},
			func(ctx context.Context) {
				err := nc.Consume(ctx, "user-service",
					[]string{events.TypeBookCompleted, events.TypeProgressUpdated},
					events.Route(map[string]events.Handler{
						events.TypeBookCompleted:   service.OnBookCompleted,
						events.TypeProgressUpdated: service.OnProgressUpdated,
					}))
				server.Must(err, "nats consumer failed")
			},
		),
	})
	server.Must(err, "server error")
}
