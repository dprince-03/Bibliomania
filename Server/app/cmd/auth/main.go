// auth-service: registration, login, logout, refresh-token rotation.
// MySQL + RabbitMQ. REST only (no gRPC server — nothing calls auth
// synchronously; login never depends on another service).
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/dprince-03/Bibliomania/internal/config"
	"github.com/dprince-03/Bibliomania/internal/database"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/health"
	"github.com/dprince-03/Bibliomania/internal/server"
	"github.com/dprince-03/Bibliomania/internal/services/auth"
	"github.com/dprince-03/Bibliomania/internal/telemetry"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
	"github.com/dprince-03/Bibliomania/pkg/mysqlclient"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load("auth-service")
	server.Must(err, "config error")
	server.Must(cfg.Require("DATABASE_URL", "JWT_SECRET", "RABBITMQ_URL"), "config error")

	shutdown, err := telemetry.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	server.Must(err, "telemetry init failed")
	defer func() { _ = shutdown(context.Background()) }()

	db, err := mysqlclient.Connect(ctx, cfg.DatabaseURL)
	server.Must(err, "database error")
	defer db.Close()
	server.Must(database.Migrate(db, database.DriverMySQL, auth.Migrations), "migration error")

	rabbit, err := events.DialRabbit(ctx, cfg.RabbitMQURL, cfg.RabbitMQQueueType)
	server.Must(err, "rabbitmq error")
	defer rabbit.Close()

	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	service := auth.NewService(db, auth.NewAccountRepository(db), auth.NewTokenRepository(db), jwtManager, cfg.RefreshTokenTTL)
	handler := auth.NewHandler(service, utils.NewValidator())

	mux := http.NewServeMux()
	handler.Routes(mux)
	handler.InternalRoutes(mux, cfg.InternalAPIToken)
	server.Ops(mux, health.NewChecker(cfg.ServiceName).
		Add("database", db.PingContext).
		Add("rabbitmq", rabbit.Ping).
		Handle)

	err = server.Run(ctx, server.Service{
		Config:  cfg,
		Handler: mux,
		Workers: []server.Worker{
			func(ctx context.Context) { events.RunRelay(ctx, db, rabbit, 500*time.Millisecond) },
			func(ctx context.Context) {
				rabbit.Consume(ctx, "auth-service", []string{events.TypeUserStatusChanged}, service.OnUserStatusChanged)
			},
		},
	})
	server.Must(err, "server error")
}
