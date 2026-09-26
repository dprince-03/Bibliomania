// Package config loads every service's settings from the environment.
//
// There is one Config struct for all eight binaries (seven services + the
// gateway) rather than one per service: each container only gets the env
// vars it actually uses (see infra/docker/docker-compose.yml), and each
// cmd/<service>/main.go calls Require for the keys it can't run without.
// Keeping one struct avoids eight near-identical loaders drifting apart.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Service identity — used for logs, traces, metrics and event `source`.
	ServiceName string

	// Server
	AppEnv       string
	ServerPort   string // HTTP (REST + /health + /metrics)
	GRPCPort     string // internal gRPC, never published to the host
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Databases — each service gets exactly one, via its own DSN.
	DatabaseURL string // MySQL DSN (auth) or postgres:// URL (catalog/borrow/user/payment)
	// Optional read-only replica pool for catalog's public reads; empty =
	// read from DatabaseURL.
	ReadDatabaseURL string
	MongoURL        string // reading
	MongoDB         string

	// Redis (catalog cache)
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// JWT — verified independently by every service and the gateway.
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// Storage (catalog's e-library uploads). STORAGE_BACKEND=local keeps
	// files under StoragePath (bare `go run`); s3 uses an S3-compatible
	// object store (SeaweedFS in Compose/Kubernetes) so catalog can scale.
	StorageBackend  string
	StoragePath     string
	S3Endpoint      string
	S3AccessKey     string
	S3SecretKey     string
	S3Bucket        string
	S3UseSSL        bool
	S3Region        string
	MaxUploadSizeMB int64
	// clamd host:port for virus-scanning uploads; empty = no scanning.
	ClamAVAddr string

	// Rate limiting (gateway only — it's the single edge)
	RateLimitRPS   float64
	RateLimitBurst int

	// Borrowing
	BorrowLoanDays       int
	OverdueSweepInterval time.Duration

	// Message brokers
	RabbitMQURL  string
	KafkaBrokers []string
	// Replication factor for topics this service creates: 1 for the
	// single-broker dev cluster, 3 for the HA cluster (infra/k8s ha).
	KafkaReplicationFactor int
	NATSURL                string
	// JetStream stream replicas (1 single node; 3 for the HA cluster).
	NATSStreamReplicas int
	// RabbitMQ queue type for consumer queues: "classic" (default) or
	// "quorum" (Raft-replicated across cluster nodes — the HA setting).
	RabbitMQQueueType string

	// Internal gRPC addresses (host:port on the Compose network)
	CatalogGRPCAddr string
	BorrowGRPCAddr  string
	ReadingGRPCAddr string
	UserGRPCAddr    string

	// Internal HTTP base URLs — the gateway's REST façade proxies to these.
	AuthHTTPURL    string
	CatalogHTTPURL string
	BorrowHTTPURL  string
	ReadingHTTPURL string
	UserHTTPURL    string
	PaymentHTTPURL string

	// Shared secret for service-internal endpoints never exposed through the
	// gateway (auth's /internal/v1/accounts, used by user-service's
	// reconciler). Empty disables those endpoints.
	InternalAPIToken string
	// How often user-service reconciles its user copies against auth's
	// accounts; 0 disables.
	ReconcileInterval time.Duration

	// Observability — empty disables trace export (propagation still works).
	OTLPEndpoint string

	// Email (notification-service). RESEND_API_KEY wins over SMTP when set.
	SMTPHost     string
	SMTPPort     string
	MailFrom     string
	ResendAPIKey string

	// Payments (payment-service)
	StripeSecretKey     string
	StripeWebhookSecret string
	PaystackSecretKey   string
	PaystackBaseURL     string
	PaystackCurrencies  []string // currencies routed to Paystack
	// Step 38 launch gate: live payment keys (Stripe or Paystack) are
	// refused unless this is set explicitly — see Server/app/docs/plan.md,
	// Step 27/38.
	PaymentsLiveEnabled bool
	PaymentSuccessURL   string
	PaymentCancelURL    string

	// API versioning (gateway) — see internal/apiversion. Comma-separated
	// "version:deprecatedOn[:sunsetOn]" entries (dates YYYY-MM-DD), e.g.
	// "v1:2027-01-01:2027-07-01". Empty = nothing deprecated.
	APIDeprecations string
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

func getEnvAsBool(key string, fallback bool) bool {
	if val := os.Getenv(key); val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
		log.Printf("Warning: invalid bool for %s, using default %t\n", key, fallback)
	}
	return fallback
}

func getEnvAsList(key string, fallback []string) []string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	var out []string
	for _, part := range strings.Split(val, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Require fails if any of the given env keys is unset. Each cmd/<service>
// declares its own must-haves here — this is the only place required-field
// checks belong.
func (c *Config) Require(keys ...string) error {
	var missing []string
	for _, k := range keys {
		if os.Getenv(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: missing required env: %s", c.ServiceName, strings.Join(missing, ", "))
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

// Load reads .env (outside production, and only if present — inside Docker
// the env comes from Compose, not a file) and returns the config for the
// named service.
func Load(serviceName string) (*Config, error) {
	if os.Getenv("APP_ENV") != "production" {
		if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	cfg := &Config{
		ServiceName: serviceName,

		// Server
		AppEnv:       getEnv("APP_ENV", "development"),
		ServerPort:   getEnv("SERVER_PORT", "8080"),
		GRPCPort:     getEnv("GRPC_PORT", "9090"),
		ReadTimeout:  time.Duration(getEnvAsInt("SERVER_READ_TIMEOUT", 10)) * time.Second,
		WriteTimeout: time.Duration(getEnvAsInt("SERVER_WRITE_TIMEOUT", 10)) * time.Second,

		// Databases
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		ReadDatabaseURL: getEnv("READ_DATABASE_URL", ""),
		MongoURL:        getEnv("MONGO_URL", "mongodb://localhost:27017"),
		MongoDB:         getEnv("MONGO_DB", "bibliomania_reading"),

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
		StorageBackend:  getEnv("STORAGE_BACKEND", "local"),
		StoragePath:     getEnv("STORAGE_PATH", "./storage"),
		S3Endpoint:      getEnv("S3_ENDPOINT", ""),
		S3AccessKey:     getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:     getEnv("S3_SECRET_KEY", ""),
		S3Bucket:        getEnv("S3_BUCKET", "bibliomania-books"),
		S3UseSSL:        getEnvAsBool("S3_USE_SSL", false),
		S3Region:        getEnv("S3_REGION", "us-east-1"),
		MaxUploadSizeMB: int64(getEnvAsInt("MAX_UPLOAD_SIZE_MB", 50)),
		ClamAVAddr:      getEnv("CLAMAV_ADDR", ""),

		// Rate Limiting
		RateLimitRPS:   getEnvAsFloat("RATE_LIMIT_RPS", 100),
		RateLimitBurst: getEnvAsInt("RATE_LIMIT_BURST", 200),

		// Borrowing
		BorrowLoanDays:       getEnvAsInt("BORROW_LOAN_DAYS", 14),
		OverdueSweepInterval: time.Duration(getEnvAsInt("OVERDUE_SWEEP_INTERVAL_SECONDS", 300)) * time.Second,

		// Brokers
		RabbitMQURL:            getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		KafkaBrokers:           getEnvAsList("KAFKA_BROKERS", []string{"localhost:9092"}),
		KafkaReplicationFactor: getEnvAsInt("KAFKA_REPLICATION_FACTOR", 1),
		NATSURL:                getEnv("NATS_URL", "nats://localhost:4222"),
		NATSStreamReplicas:     getEnvAsInt("NATS_STREAM_REPLICAS", 1),
		RabbitMQQueueType:      getEnv("RABBITMQ_QUEUE_TYPE", "classic"),

		// gRPC
		CatalogGRPCAddr: getEnv("CATALOG_GRPC_ADDR", "localhost:9091"),
		BorrowGRPCAddr:  getEnv("BORROW_GRPC_ADDR", "localhost:9092"),
		ReadingGRPCAddr: getEnv("READING_GRPC_ADDR", "localhost:9093"),
		UserGRPCAddr:    getEnv("USER_GRPC_ADDR", "localhost:9094"),

		// Internal HTTP
		AuthHTTPURL:    getEnv("AUTH_HTTP_URL", "http://localhost:8081"),
		CatalogHTTPURL: getEnv("CATALOG_HTTP_URL", "http://localhost:8082"),
		BorrowHTTPURL:  getEnv("BORROW_HTTP_URL", "http://localhost:8083"),
		ReadingHTTPURL: getEnv("READING_HTTP_URL", "http://localhost:8084"),
		UserHTTPURL:    getEnv("USER_HTTP_URL", "http://localhost:8085"),
		PaymentHTTPURL: getEnv("PAYMENT_HTTP_URL", "http://localhost:8087"),

		OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),

		InternalAPIToken:  getEnv("INTERNAL_API_TOKEN", ""),
		ReconcileInterval: time.Duration(getEnvAsInt("RECONCILE_INTERVAL_MINUTES", 60)) * time.Minute,

		// Email
		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnv("SMTP_PORT", "1025"),
		MailFrom:     getEnv("MAIL_FROM", "Bibliomania <no-reply@bibliomania.local>"),
		ResendAPIKey: getEnv("RESEND_API_KEY", ""),

		// Payments
		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		PaystackSecretKey:   getEnv("PAYSTACK_SECRET_KEY", ""),
		PaystackBaseURL:     getEnv("PAYSTACK_BASE_URL", "https://api.paystack.co"),
		PaystackCurrencies:  getEnvAsList("PAYSTACK_CURRENCIES", []string{"NGN", "GHS", "ZAR", "KES"}),
		PaymentsLiveEnabled: getEnvAsBool("PAYMENTS_LIVE_ENABLED", false),
		PaymentSuccessURL:   getEnv("PAYMENT_SUCCESS_URL", "http://app.bibliomania.local/payments/success"),
		PaymentCancelURL:    getEnv("PAYMENT_CANCEL_URL", "http://app.bibliomania.local/payments/cancel"),

		APIDeprecations: getEnv("API_DEPRECATED_VERSIONS", ""),
	}

	if cfg.DBPasswordMissingInProd() {
		return nil, fmt.Errorf("%s: DATABASE_URL must carry a password in production", serviceName)
	}

	return cfg, nil
}

// DBPasswordMissingInProd catches the one misconfiguration the old
// DB_PASSWORD check guarded against, now that credentials live inside a DSN.
func (c *Config) DBPasswordMissingInProd() bool {
	if !c.IsProduction() || c.DatabaseURL == "" {
		return false
	}
	// Both DSN shapes put credentials before '@': "user:pass@..." / "postgres://user:pass@...".
	creds, _, found := strings.Cut(strings.TrimPrefix(c.DatabaseURL, "postgres://"), "@")
	if !found {
		return true
	}
	_, pass, _ := strings.Cut(creds, ":")
	return pass == ""
}
