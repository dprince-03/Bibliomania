package main

import (
	"bibliotheca/internal/auth"
	"bibliotheca/internal/cache"
	"bibliotheca/internal/config"
	"bibliotheca/internal/database"
	"bibliotheca/internal/repository"
	"bibliotheca/internal/router"
	"bibliotheca/pkg/jwt"
	"bibliotheca/pkg/mysqlclient"
	"bibliotheca/pkg/redisclient"

	"context"
	"github.com/go-playground/validator/v10"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// ~~ Config ~~
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Bibliotheca starting on port %s [%s mode]\n", cfg.ServerPort, cfg.AppEnv)

	// ~~ MySql Database ~~
	db, err := mysqlclient.ConnectMySqlClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MySql client: %v", err)
	}
	defer db.Close()

	// ~~ Migration ~~
	if err := database.RunMigration(db, "./migrations"); err != nil {
		log.Fatalf("Migration error: %v", err)
	}

	// ~~ Redis ~~
	redisClient, err := redisclient.Connect(cfg)
	if err != nil {
		log.Fatalf("Redis error: %v", err)
	}
	defer redisClient.Close()

	// Cache Layer
	appCache := cache.NewRedisCache(redisClient, "bibliotheca")
	_ = appCache

	// ~~ Shared dependencies ~~
	validate := validator.New()
	jwtManager := jwt.NewManger(cfg.JWTSecret, cfg.AccessTokenTTL)

	// ~~ Repositories ~~
	userRepo := repository.NewUserRepository(db)
	profileRepo := repository.NewUserProfileRepository(db)
	tokenRepo := repository.NewTokenRepository(db)

	// ~~ Auth ~~
	authService := auth.NewService(
		userRepo,
		profileRepo,
		tokenRepo,
		jwtManager,
		cfg.RefreshTokenTTL,
	)
	authHandler := auth.NewHandler(authService, validate)

	// ~~ Routes ~~
	r := router.Routes(authHandler)

	// ~~ Server ~~
	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// ~~ Graceful Shutdown ~~
	go func() {
		log.Printf("Server is listening on port : %s\n", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server Error: %v", err)
		}
	}()

	// Block until we receive SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")

	// Give in-flight requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Forced shutdown: %v", err)
	}

	log.Println("Server stopped cleanly")
}
