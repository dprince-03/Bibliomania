package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	AppEnv       string
	ServerPort   string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// JWT
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// Storage
	StoragePath     string
	MaxUploadSizeMB int64

	// Rate Limiting
	RateLimitRPS   float64
	RateLimitBurst int
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		i, err := strconv.Atoi(val)
		if err == nil {
			return i
		}
		log.Printf("Warning: invalid int for %s, using default %d\n", key, fallback)
	}
	return fallback
}

func getEnvAsFloat(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		f, err := strconv.ParseFloat(val, 64)
		if err == nil {
			return f
		}
		log.Printf("Warning: invalid float for %s, using default %f\n", key, fallback)
	}
	return fallback
}

// validate ensures critical config values are present.
func (c *Config) validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.DBPassword == "" && c.AppEnv == "production" {
		return fmt.Errorf("DB_PASSWORD is required in production")
	}
	return nil
}

// IsDevelopment is a convenience helper used in middleware & logging.
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

// IsProduction is a convenience helper.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func Load() (*Config, error) {
	if os.Getenv("APP_ENV") != "production" {
		err := godotenv.Load()
		if err != nil {
			return nil, fmt.Errorf("Failed to load .env file")
		}
	}

	cfg := &Config{
		// Server
		AppEnv:       getEnv("APP_ENV", "development"),
		ServerPort:   getEnv("PORT", "5000"),
		ReadTimeout:  time.Duration(getEnvAsInt("SERVER_READ_TIMEOUT", 10)) * time.Second,
		WriteTimeout: time.Duration(getEnvAsInt("SERVER_WRITE_TIMEOUT", 10)) * time.Second,

		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "bibliotheca"),

		// Redis
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),

		// JWT
		JWTSecret:       getEnv("JWT_SECRET", ""),
		AccessTokenTTL:  time.Duration(getEnvAsInt("JWT_ACCESS_TOKEN_TTL", 15)) * time.Minute,
		RefreshTokenTTL: time.Duration(getEnvAsInt("JWT_REFRESH_TOKEN_TTL", 10080)) * time.Minute,

		// Storage
		StoragePath:     getEnv("STORAGE_PATH", "./storage"),
		MaxUploadSizeMB: int64(getEnvAsInt("MAX_UPLOAD_SIZE_MB", 50)),

		// Rate Limiting
		RateLimitRPS:   getEnvAsFloat("RATE_LIMIT_RPS", 100),
		RateLimitBurst: getEnvAsInt("RATE_LIMIT_BURST", 200),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}
