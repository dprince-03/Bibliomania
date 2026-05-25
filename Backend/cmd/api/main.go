package main

import (
	"bibliotheca/internal/cache"
	"bibliotheca/internal/config"
	"bibliotheca/internal/database"
	"bibliotheca/pkg/mysqlclient"
	"bibliotheca/pkg/redisclient"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	// "github.com/go-playground/validator/v10"
)

func main() {
	r := gin.Default()
	// validate := validator.New()
	// jwtSecret := cfg.JWTSecret
	
	// Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Sql
	db, err := mysqlclient.ConnectMySqlClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MySql client: %v", err)
	}
	defer db.Close()

	// Migration
	if err := database.RunMigration(db, "./migrations"); err != nil {
		log.Fatalf("Migration error: %v", err)
	}

	// Redis
	redisClient, err := redisclient.Connect(cfg)
	if err != nil {
		log.Fatalf("Redis error: %v", err)
	}
	defer redisClient.Close()

	// Cache Layer
	appCache := cache.NewRedisCache(redisClient, "bibliotheca")
	_ = appCache


	// URL
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Routes
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "root url is healthy",
		})
	})

	r.GET("/health-check", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "url is working",
		})
	})

	// Server
	log.Printf("Bibliotheca starting on port %s [%s mode]\n", cfg.ServerPort, cfg.AppEnv)
	server := fmt.Sprintf("0.0.0.0:%s", cfg.ServerPort)
	r.Run(server)
}