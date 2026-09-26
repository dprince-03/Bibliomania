// reading-service: reading progress (online + offline sync), bookmarks,
// reading history. MongoDB (single-node replica set, for transactions) +
// NATS JetStream.
package main

import (
	"context"
	"net/http"
	"time"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	readingv1 "github.com/dprince-03/Bibliomania/gen/reading/v1"
	"github.com/dprince-03/Bibliomania/internal/config"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/health"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/server"
	"github.com/dprince-03/Bibliomania/internal/services/reading"
	"github.com/dprince-03/Bibliomania/internal/telemetry"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
	"github.com/dprince-03/Bibliomania/pkg/mongoclient"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load("reading-service")
	server.Must(err, "config error")
	server.Must(cfg.Require("MONGO_URL", "JWT_SECRET", "NATS_URL", "CATALOG_GRPC_ADDR"), "config error")

	shutdown, err := telemetry.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	server.Must(err, "telemetry init failed")
	defer func() { _ = shutdown(context.Background()) }()

	mongoClient, err := mongoclient.Connect(ctx, cfg.MongoURL)
	server.Must(err, "database error")
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()
	db := mongoClient.Database(cfg.MongoDB)

	outbox := events.NewMongoOutbox(db)
	server.Must(reading.EnsureIndexes(ctx, db), "index setup failed")
	server.Must(outbox.EnsureIndexes(ctx), "outbox index setup failed")

	nc, err := events.DialNATS(ctx, cfg.NATSURL, cfg.NATSStreamReplicas)
	server.Must(err, "nats error")
	defer nc.Close()

	catalogConn, err := grpcx.Dial(cfg.CatalogGRPCAddr)
	server.Must(err, "catalog client error")
	defer catalogConn.Close()

	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	store := reading.NewStore(mongoClient, db, outbox)
	service := reading.NewService(store, reading.NewCatalogClient(catalogv1.NewCatalogServiceClient(catalogConn)))

	mux := http.NewServeMux()
	reading.NewHandler(service, utils.NewValidator()).Routes(mux, middleware.NewGuards(jwtManager))
	server.Ops(mux, health.NewChecker(cfg.ServiceName).
		Add("database", store.Ping).
		Add("nats", nc.Ping).
		Handle)

	grpcServer := grpcx.NewServer(jwtManager, cfg.IsProduction())
	readingv1.RegisterReadingServiceServer(grpcServer, reading.NewGRPCServer(service))

	err = server.Run(ctx, server.Service{
		Config:  cfg,
		Handler: mux,
		GRPC:    grpcServer,
		Workers: []server.Worker{
			func(ctx context.Context) { outbox.RunRelay(ctx, nc, 200*time.Millisecond) },
		},
	})
	server.Must(err, "server error")
}
