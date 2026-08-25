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

	"github.com/dprince-03/Bibliotheca/internal/cache"
	"github.com/dprince-03/Bibliotheca/internal/config"
	"github.com/dprince-03/Bibliotheca/internal/database"
	"github.com/dprince-03/Bibliotheca/internal/modules/auth"
	"github.com/dprince-03/Bibliotheca/internal/modules/borrow"
	"github.com/dprince-03/Bibliotheca/internal/modules/catalog"
	"github.com/dprince-03/Bibliotheca/internal/modules/reading"
	"github.com/dprince-03/Bibliotheca/internal/modules/user"
	"github.com/dprince-03/Bibliotheca/internal/router"
	"github.com/dprince-03/Bibliotheca/pkg/jwt"
	"github.com/dprince-03/Bibliotheca/pkg/mysqlclient"
	"github.com/dprince-03/Bibliotheca/pkg/redisclient"
)

func main() {
	// ── Config ────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	log.Printf("Bibliotheca starting on port %s [%s mode]\n", cfg.ServerPort, cfg.AppEnv)

	// ── Database ──────────────────────────────
	db, err := mysqlclient.ConnectMySqlClient(cfg)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	if err := database.RunMigration(db, "./migrations"); err != nil {
		log.Fatalf("Migration error: %v", err)
	}

	// ── Redis ─────────────────────────────────
	redisClient, err := redisclient.Connect(cfg)
	if err != nil {
		log.Fatalf("Redis error: %v", err)
	}
	defer redisClient.Close()

	appCache := cache.NewRedisCache(redisClient, "bibliotheca")

	// ── Shared ────────────────────────────────
	validate := validator.New()
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL)

	// ── Repositories ──────────────────────────
	userRepo := user.NewRepository(db)
	profileRepo := user.NewProfileRepository(db)
	tokenRepo := auth.NewTokenRepository(db)
	authorRepo := catalog.NewAuthorRepository(db)
	bookRepo := catalog.NewBookRepository(db)
	bookAuthorRepo := catalog.NewBookAuthorRepository(db)
	readingSessionRepo := reading.NewSessionRepository(db)
	bookmarkRepo := reading.NewBookmarkRepository(db)
	borrowRepo := borrow.NewRepository(db)

	// ── Services ──────────────────────────────
	authService := auth.NewService(userRepo, profileRepo, tokenRepo, jwtManager, cfg.RefreshTokenTTL)
	authorService := catalog.NewAuthorService(authorRepo, bookAuthorRepo, appCache)
	bookService := catalog.NewBookService(bookRepo, authorRepo, bookAuthorRepo, appCache, cfg.StoragePath, cfg.MaxUploadSizeMB)
	readingService := reading.NewService(readingSessionRepo, bookmarkRepo, bookRepo)
	borrowService := borrow.NewService(borrowRepo, bookRepo, cfg.BorrowLoanDays)

	// ── Handlers ──────────────────────────────
	authHandler := auth.NewHandler(authService, validate)
	authorHandler := catalog.NewAuthorHandler(authorService, validate)
	bookHandler := catalog.NewBookHandler(bookService, validate, cfg.MaxUploadSizeMB)
	readingHandler := reading.NewHandler(readingService, validate)
	borrowHandler := borrow.NewHandler(borrowService, validate)

	// ── Router ────────────────────────────────
	r := router.New(cfg, jwtManager, authHandler, authorHandler, bookHandler, readingHandler, borrowHandler)

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
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Forced shutdown: %v", err)
	}

	log.Println("Server stopped cleanly")
}
