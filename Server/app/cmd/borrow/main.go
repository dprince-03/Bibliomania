// borrow-service: borrowing, returns, overdue detection, and the
// orchestrated borrow Saga against catalog-service. Postgres + Kafka.
package main

import (
	"context"
	"net/http"
	"time"

	borrowv1 "github.com/dprince-03/Bibliomania/gen/borrow/v1"
	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	"github.com/dprince-03/Bibliomania/internal/config"
	"github.com/dprince-03/Bibliomania/internal/database"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/health"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/server"
	"github.com/dprince-03/Bibliomania/internal/services/borrow"
	"github.com/dprince-03/Bibliomania/internal/telemetry"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
	"github.com/dprince-03/Bibliomania/pkg/postgresclient"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load("borrow-service")
	server.Must(err, "config error")
	server.Must(cfg.Require("DATABASE_URL", "JWT_SECRET", "KAFKA_BROKERS", "CATALOG_GRPC_ADDR"), "config error")

	shutdown, err := telemetry.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	server.Must(err, "telemetry init failed")
	defer func() { _ = shutdown(context.Background()) }()

	db, err := postgresclient.Connect(ctx, cfg.DatabaseURL)
	server.Must(err, "database error")
	defer db.Close()
	server.Must(database.Migrate(db, database.DriverPostgres, borrow.Migrations), "migration error")

	events.SetReplicationFactor(cfg.KafkaReplicationFactor)
	server.Must(events.EnsureTopics(ctx, cfg.KafkaBrokers, events.TopicBorrow), "kafka topic setup failed")
	kafka, err := events.NewKafkaPublisher(cfg.KafkaBrokers, events.TopicBorrow)
	server.Must(err, "kafka error")
	defer kafka.Close()

	catalogConn, err := grpcx.Dial(cfg.CatalogGRPCAddr)
	server.Must(err, "catalog client error")
	defer catalogConn.Close()

	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	service := borrow.NewService(borrow.NewRepository(db), borrow.NewCatalogClient(catalogv1.NewCatalogServiceClient(catalogConn)), cfg.BorrowLoanDays)

	mux := http.NewServeMux()
	borrow.NewHandler(service, utils.NewValidator()).Routes(mux, middleware.NewGuards(jwtManager))
	server.Ops(mux, health.NewChecker(cfg.ServiceName).
		Add("database", db.PingContext).
		Add("kafka", kafka.Ping).
		Handle)

	grpcServer := grpcx.NewServer(jwtManager, cfg.IsProduction())
	borrowv1.RegisterBorrowServiceServer(grpcServer, borrow.NewGRPCServer(service))

	err = server.Run(ctx, server.Service{
		Config:  cfg,
		Handler: mux,
		GRPC:    grpcServer,
		Workers: []server.Worker{
			func(ctx context.Context) { events.RunRelay(ctx, db, kafka, 500*time.Millisecond) },
			func(ctx context.Context) { service.RunCompensator(ctx, 5*time.Second) },
			func(ctx context.Context) { service.RunOverdueSweeper(ctx, cfg.OverdueSweepInterval) },
		},
	})
	server.Must(err, "server error")
}
