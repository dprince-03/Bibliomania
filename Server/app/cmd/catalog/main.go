// catalog-service: authors, books, search, e-library files, and the
// ReserveCopy/ReleaseCopy half of the borrow Saga. Postgres + Redis +
// RabbitMQ. Serves REST (via the gateway) and gRPC (internal).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	catalogv1 "github.com/dprince-03/Bibliomania/gen/catalog/v1"
	"github.com/dprince-03/Bibliomania/internal/cache"
	"github.com/dprince-03/Bibliomania/internal/config"
	"github.com/dprince-03/Bibliomania/internal/database"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/grpcx"
	"github.com/dprince-03/Bibliomania/internal/health"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/server"
	"github.com/dprince-03/Bibliomania/internal/services/catalog"
	"github.com/dprince-03/Bibliomania/internal/storage"
	"github.com/dprince-03/Bibliomania/internal/telemetry"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/clamav"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
	"github.com/dprince-03/Bibliomania/pkg/postgresclient"
	"github.com/dprince-03/Bibliomania/pkg/redisclient"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load("catalog-service")
	server.Must(err, "config error")
	server.Must(cfg.Require("DATABASE_URL", "JWT_SECRET", "RABBITMQ_URL"), "config error")

	shutdown, err := telemetry.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
	server.Must(err, "telemetry init failed")
	defer func() { _ = shutdown(context.Background()) }()

	db, err := postgresclient.Connect(ctx, cfg.DatabaseURL)
	server.Must(err, "database error")
	defer db.Close()
	server.Must(database.Migrate(db, database.DriverPostgres, catalog.Migrations), "migration error")

	redisClient, err := redisclient.Connect(fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort), cfg.RedisPassword, cfg.RedisDB)
	server.Must(err, "redis error")
	defer redisClient.Close()
	appCache := cache.NewRedisCache(redisClient, "bibliomania")

	var store storage.Store
	switch cfg.StorageBackend {
	case "s3":
		server.Must(cfg.Require("S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY"), "config error")
		store, err = storage.NewS3(ctx, storage.S3Config{
			Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey,
			Bucket: cfg.S3Bucket, UseSSL: cfg.S3UseSSL, Region: cfg.S3Region,
		})
	default:
		store, err = storage.NewLocal(cfg.StoragePath)
	}
	server.Must(err, "storage error")

	// Virus scanning is optional; when configured it fails closed.
	var scanner catalog.Scanner
	checks := health.NewChecker(cfg.ServiceName)
	if cfg.ClamAVAddr != "" {
		clam := clamav.New(cfg.ClamAVAddr)
		scanner = clam
		checks.Add("virus-scanner", clam.Ping)
	} else {
		slog.Warn("CLAMAV_ADDR unset — uploads are not virus-scanned")
	}

	rabbit, err := events.DialRabbit(ctx, cfg.RabbitMQURL, cfg.RabbitMQQueueType)
	server.Must(err, "rabbitmq error")
	defer rabbit.Close()

	validate := utils.NewValidator()
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)

	authorRepo := catalog.NewAuthorRepository(db)
	bookRepo := catalog.NewBookRepository(db)
	bookAuthorRepo := catalog.NewBookAuthorRepository(db)

	authorService := catalog.NewAuthorService(authorRepo, bookAuthorRepo, appCache)
	bookService := catalog.NewBookService(db, bookRepo, authorRepo, bookAuthorRepo, appCache, store, scanner, cfg.MaxUploadSizeMB)

	// Public reads from a replica when one is configured (HA setup).
	if cfg.ReadDatabaseURL != "" {
		readDB, err := postgresclient.Connect(ctx, cfg.ReadDatabaseURL)
		server.Must(err, "read replica error")
		defer readDB.Close()
		reads := catalog.ReadReplicas{
			Books:       catalog.NewBookRepository(readDB),
			Authors:     catalog.NewAuthorRepository(readDB),
			BookAuthors: catalog.NewBookAuthorRepository(readDB),
		}
		bookService.WithReadReplicas(reads)
		authorService.WithReadReplicas(reads)
		checks.Add("read-replica", readDB.PingContext)
		slog.Info("catalog reads routed to replica")
	}

	guards := middleware.NewGuards(jwtManager)
	mux := http.NewServeMux()
	catalog.NewAuthorHandler(authorService, validate).Routes(mux, guards)
	catalog.NewBookHandler(bookService, validate, cfg.MaxUploadSizeMB).Routes(mux, guards)
	server.Ops(mux, checks.
		Add("database", db.PingContext).
		Add("cache", appCache.Ping).
		Add("rabbitmq", rabbit.Ping).
		Add("storage", store.Ping).
		Handle)

	grpcServer := grpcx.NewServer(jwtManager, cfg.IsProduction())
	catalogv1.RegisterCatalogServiceServer(grpcServer, catalog.NewGRPCServer(bookService))

	err = server.Run(ctx, server.Service{
		Config:  cfg,
		Handler: mux,
		GRPC:    grpcServer,
		Workers: []server.Worker{
			func(ctx context.Context) { events.RunRelay(ctx, db, rabbit, 500*time.Millisecond) },
		},
	})
	server.Must(err, "server error")
}
