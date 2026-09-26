// notification-service: turns domain events into email. No database, no
// public API — consumes RabbitMQ (auth events) and Kafka (borrow and
// payment events). One service subscribing to several self-contained
// flows; no single flow crosses brokers.
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dprince-03/Bibliomania/internal/config"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/health"
	"github.com/dprince-03/Bibliomania/internal/server"
	"github.com/dprince-03/Bibliomania/internal/services/notification"
	"github.com/dprince-03/Bibliomania/internal/telemetry"
	"github.com/dprince-03/Bibliomania/pkg/redisclient"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load("notification-service")
	server.Must(err, "config error")
	server.Must(cfg.Require("RABBITMQ_URL", "KAFKA_BROKERS"), "config error")

	shutdown, err := telemetry.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	server.Must(err, "telemetry init failed")
	defer func() { _ = shutdown(context.Background()) }()

	rabbit, err := events.DialRabbit(ctx, cfg.RabbitMQURL, cfg.RabbitMQQueueType)
	server.Must(err, "rabbitmq error")
	defer rabbit.Close()

	events.SetReplicationFactor(cfg.KafkaReplicationFactor)
	server.Must(events.EnsureTopics(ctx, cfg.KafkaBrokers, events.TopicBorrow, events.TopicPayment), "kafka topic setup failed")

	redisClient, err := redisclient.Connect(fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort), cfg.RedisPassword, cfg.RedisDB)
	server.Must(err, "redis error")
	defer redisClient.Close()

	service := notification.NewService(
		notification.NewMailer(cfg.ResendAPIKey, cfg.SMTPHost, cfg.SMTPPort, cfg.MailFrom),
		notification.NewRedisSentLog(redisClient),
	)
	handler := events.Route(service.Handlers())

	mux := http.NewServeMux()
	server.Ops(mux, health.NewChecker(cfg.ServiceName).
		Add("rabbitmq", rabbit.Ping).
		Add("redis", func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }).
		Handle)

	err = server.Run(ctx, server.Service{
		Config:  cfg,
		Handler: mux,
		Workers: []server.Worker{
			func(ctx context.Context) {
				rabbit.Consume(ctx, "notification-service", []string{events.TypeUserRegistered}, handler)
			},
			func(ctx context.Context) {
				err := events.ConsumeKafka(ctx, cfg.KafkaBrokers, "notification-service",
					[]string{events.TopicBorrow, events.TopicPayment}, handler)
				server.Must(err, "kafka consumer failed")
			},
		},
	})
	server.Must(err, "server error")
}
