package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	apperrors "github.com/yourusername/bibliotheca/internal/errors"
	"github.com/yourusername/bibliotheca/internal/utils"
	"golang.org/x/time/rate"
)

// ipLimiter holds a rate limiter and the last time it was seen.
// lastSeen is used to clean up stale limiters from memory.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiterStore manages per-IP limiters with automatic cleanup.
type RateLimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	rps      rate.Limit // requests per second
	burst    int        // max burst size
	ttl      time.Duration // how long before an idle IP is evicted
}

func NewRateLimiterStore(rps float64, burst int, ttl time.Duration) *RateLimiterStore {
	store := &RateLimiterStore{
		limiters: make(map[string]*ipLimiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		ttl:      ttl,
	}

	// Background goroutine cleans up stale IP entries every minute
	// Prevents unbounded memory growth under sustained traffic
	go store.cleanupLoop()

	return store
}

// getLimiter returns the rate limiter for an IP, creating one if needed.
func (s *RateLimiterStore) getLimiter(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.limiters[ip]
	if !exists {
		limiter := rate.NewLimiter(s.rps, s.burst)
		s.limiters[ip] = &ipLimiter{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	entry.lastSeen = time.Now()
	return entry.limiter
}

// cleanupLoop removes IP entries that haven't been seen within the TTL.
func (s *RateLimiterStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		for ip, entry := range s.limiters {
			if time.Since(entry.lastSeen) > s.ttl {
				delete(s.limiters, ip)
			}
		}
		s.mu.Unlock()
	}
}

// RateLimit returns a middleware that enforces per-IP rate limiting.
func RateLimit(store *RateLimiterStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			limiter := store.getLimiter(ip)

			if !limiter.Allow() {
				// Set Retry-After header so clients know when to try again
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Limit", "see documentation")
				utils.Error(w, apperrors.TooManyRequests("rate limit exceeded, slow down"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitStrict returns a tighter rate limiter for sensitive routes.
// Use on: /auth/login, /auth/register, /auth/refresh
func RateLimitStrict(rps float64, burst int) func(http.Handler) http.Handler {
	store := NewRateLimiterStore(rps, burst, 10*time.Minute)
	return RateLimit(store)
}

// extractIP gets the real client IP, respecting proxies.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (set by load balancers / proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can be a comma-separated list — first IP is the client
		if ip, _, err := net.SplitHostPort(xff); err == nil {
			return ip
		}
		return xff
	}

	// Check X-Real-IP (set by Nginx)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}