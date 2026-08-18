package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/yourusername/bibliotheca/internal/auth"
	"github.com/yourusername/bibliotheca/internal/cache"
	"github.com/yourusername/bibliotheca/internal/config"
	"github.com/yourusername/bibliotheca/internal/database"
	"github.com/yourusername/bibliotheca/internal/handlers"
	"github.com/yourusername/bibliotheca/internal/repository"
	"github.com/yourusername/bibliotheca/internal/router"
	"github.com/yourusername/bibliotheca/internal/services"
	"github.com/yourusername/bibliotheca/pkg/jwt"
	"github.com/yourusername/bibliotheca/pkg/mysqlclient"
	"github.com/yourusername/bibliotheca/pkg/redisclient"
)

func main() {
	// ── Config ────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Config error: %v", err)
	}
	log.Printf("📚 Bibliotheca starting on port %s [%s mode]\n", cfg.ServerPort, cfg.AppEnv)

	// ── Database ──────────────────────────────
	db, err := mysqlclient.Connect(cfg)
	if err != nil {
		log.Fatalf("❌ Database error: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, "./migrations"); err != nil {
		log.Fatalf("❌ Migration error: %v", err)
	}

	// ── Redis ─────────────────────────────────
	redisClient, err := redisclient.Connect(cfg)
	if err != nil {
		log.Fatalf("❌ Redis error: %v", err)
	}
	defer redisClient.Close()

	appCache := cache.NewRedisCache(redisClient, "bibliotheca")

	// ── Shared ────────────────────────────────
	validate   := validator.New()
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)

	// ── Repositories ──────────────────────────
	userRepo       := repository.NewUserRepository(db)
	profileRepo    := repository.NewUserProfileRepository(db)
	tokenRepo      := repository.NewTokenRepository(db)
	authorRepo     := repository.NewAuthorRepository(db)
	bookRepo       := repository.NewBookRepository(db)
	bookAuthorRepo := repository.NewBookAuthorRepository(db)

	// ── Services ──────────────────────────────
	authService   := auth.NewService(userRepo, profileRepo, tokenRepo, jwtManager, cfg.RefreshTokenTTL)
	authorService := services.NewAuthorService(authorRepo, bookAuthorRepo, appCache)
	bookService   := services.NewBookService(bookRepo, authorRepo, bookAuthorRepo, appCache)

	// ── Handlers ──────────────────────────────
	authHandler   := auth.NewHandler(authService, validate)
	authorHandler := handlers.NewAuthorHandler(authorService, validate)
	bookHandler   := handlers.NewBookHandler(bookService, validate)

	// ── Router ────────────────────────────────
	r := router.New(cfg, jwtManager, authHandler, authorHandler, bookHandler)

	// ── Server ────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// ── Graceful shutdown ─────────────────────
	go func() {
		log.Printf("🚀 Server listening on :%s\n", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⏳ Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("❌ Forced shutdown: %v", err)
	}

	log.Println("✅ Server stopped cleanly")
}