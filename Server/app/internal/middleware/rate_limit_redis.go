package middleware

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

// DistributedRateLimit limits requests per client IP with state in Redis
// (GCRA via redis_rate), so every gateway replica shares one budget — the
// in-memory RateLimiterStore gives each replica its own, and N replicas
// would allow N times the intended rate.
//
// rps/burst mirror RateLimiterStore's meaning. name separates independent
// limits (the global one, the strict /auth one) in Redis.
//
// If Redis is unreachable the request is checked against fallback (an
// in-memory limiter) instead — degraded to per-replica limits, but never
// unlimited. Responses carry RateLimit-* headers either way.
func DistributedRateLimit(rdb *redis.Client, name string, rps float64, burst int, fallback *RateLimiterStore) func(http.Handler) http.Handler {
	limiter := redis_rate.NewLimiter(rdb)
	limit := toLimit(rps, burst)
	var degraded atomic.Bool

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)

			ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
			res, err := limiter.Allow(ctx, "gw:rl:"+name+":"+ip, limit)
			cancel()

			if err != nil {
				if !degraded.Swap(true) {
					slog.WarnContext(r.Context(), "rate limiter falling back to per-replica limits (Redis unavailable)", "limit", name, "error", err)
				}
				if !fallback.getLimiter(ip).Allow() {
					tooMany(w, time.Second)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			if degraded.Swap(false) {
				slog.InfoContext(r.Context(), "rate limiter back on shared Redis limits", "limit", name)
			}

			w.Header().Set("RateLimit-Limit", strconv.Itoa(limit.Burst))
			w.Header().Set("RateLimit-Remaining", strconv.Itoa(res.Remaining))
			if res.Allowed == 0 {
				tooMany(w, res.RetryAfter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// toLimit converts "rps with burst" into redis_rate's rate-per-period form,
// keeping sub-1-rps limits (the strict /auth one is 10 per minute) exact.
func toLimit(rps float64, burst int) redis_rate.Limit {
	if rps >= 1 {
		return redis_rate.Limit{Rate: int(math.Round(rps)), Burst: burst, Period: time.Second}
	}
	perMinute := int(math.Round(rps * 60))
	return redis_rate.Limit{Rate: max(perMinute, 1), Burst: burst, Period: time.Minute}
}

func tooMany(w http.ResponseWriter, retryAfter time.Duration) {
	secs := int(math.Ceil(retryAfter.Seconds()))
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	utils.Error(w, apperrors.TooManyRequests("rate limit exceeded, slow down"))
}
