package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

const pingTimeout = 2 * time.Second

// Checker pings the database and cache the API depends on so /health reports
// real liveness instead of just "the process is running".
type Checker struct {
	db    *sqlx.DB
	redis *redis.Client
}

func NewChecker(db *sqlx.DB, redisClient *redis.Client) *Checker {
	return &Checker{db: db, redis: redisClient}
}

type status struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Database string `json:"database"`
	Cache    string `json:"cache"`
}

// Handle godoc
//
//	@Summary		Liveness check
//	@Description	Pings the database and Redis; returns 200 "ok" only if both are reachable, 503 "degraded" otherwise
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	status
//	@Failure		503	{object}	status
//	@Router			/health [get]
func (c *Checker) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
	defer cancel()

	resp := status{Status: "ok", Service: "bibliotheca", Database: "ok", Cache: "ok"}
	healthy := true

	if err := c.db.PingContext(ctx); err != nil {
		resp.Database = "unreachable"
		healthy = false
	}

	if err := c.redis.Ping(ctx).Err(); err != nil {
		resp.Cache = "unreachable"
		healthy = false
	}

	statusCode := http.StatusOK
	if !healthy {
		resp.Status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}
