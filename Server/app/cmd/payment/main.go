// payment-service: one-time book purchases via Stripe Checkout or Paystack
// (chosen by currency). Postgres + Kafka. Refuses to start with live keys
// unless PAYMENTS_LIVE_ENABLED=true (Step 38 launch gate).
package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	"github.com/dprince-03/Bibliomania/internal/config"
	"github.com/dprince-03/Bibliomania/internal/database"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/health"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/server"
	"github.com/dprince-03/Bibliomania/internal/services/payment"
	"github.com/dprince-03/Bibliomania/internal/telemetry"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
	"github.com/dprince-03/Bibliomania/pkg/postgresclient"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load("payment-service")
	server.Must(err, "config error")
	server.Must(cfg.Require("DATABASE_URL", "JWT_SECRET", "KAFKA_BROKERS", "CATALOG_GRPC_ADDR"), "config error")
	server.Must(payment.CheckLiveKeyGate(cfg.PaymentsLiveEnabled, cfg.StripeSecretKey, cfg.PaystackSecretKey), "refusing to start")

	shutdown, err := telemetry.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	server.Must(err, "telemetry init failed")
	defer func() { _ = shutdown(context.Background()) }()

	providers := payment.NewRouter(
		payment.NewStripe(cfg.StripeSecretKey, cfg.StripeWebhookSecret),
		payment.NewPaystack(cfg.PaystackSecretKey, cfg.PaystackBaseURL, cfg.PaystackCurrencies),
	)
	if cfg.StripeSecretKey == "" {
		slog.Warn("STRIPE_SECRET_KEY unset — Stripe checkout disabled")
	}
	if cfg.PaystackSecretKey == "" {
		slog.Warn("PAYSTACK_SECRET_KEY unset — Paystack checkout disabled")
	}

	db, err := postgresclient.Connect(ctx, cfg.DatabaseURL)
	server.Must(err, "database error")
	defer db.Close()
	server.Must(database.Migrate(db, database.DriverPostgres, payment.Migrations), "migration error")

	events.SetReplicationFactor(cfg.KafkaReplicationFactor)
	server.Must(events.EnsureTopics(ctx, cfg.KafkaBrokers, events.TopicPayment), "kafka topic setup failed")
	kafka, err := events.NewKafkaPublisher(cfg.KafkaBrokers, events.TopicPayment)
	server.Must(err, "kafka error")
	defer kafka.Close()

	catalogConn, err := grpcx.Dial(cfg.CatalogGRPCAddr)
	server.Must(err, "catalog client error")
	defer catalogConn.Close()

	service := payment.NewService(payment.NewRepository(db), catalogv1.NewCatalogServiceClient(catalogConn),
		providers, cfg.PaymentSuccessURL, cfg.PaymentCancelURL)

	mux := http.NewServeMux()
	payment.NewHandler(service, utils.NewValidator()).Routes(mux, middleware.NewGuards(jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)))
	server.Ops(mux, health.NewChecker(cfg.ServiceName).
		Add("database", db.PingContext).
		Add("kafka", kafka.Ping).
		Handle)

	err = server.Run(ctx, server.Service{
		Config:  cfg,
		Handler: mux,
		Workers: []server.Worker{
			func(ctx context.Context) { events.RunRelay(ctx, db, kafka, 500*time.Millisecond) },
		},
	})
	server.Must(err, "server error")
}
