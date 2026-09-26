package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func hit(h http.Handler, ip string) int {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = ip + ":1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

// Two "replicas" (two middleware instances) on one Redis share one budget.
func TestDistributedLimitSharedAcrossReplicas(t *testing.T) {
	mr := miniredis.RunT(t)
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	newReplica := func() http.Handler {
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		return DistributedRateLimit(rdb, "t", 1, 3, NewRateLimiterStore(1, 3, time.Minute))(ok)
	}
	a, b := newReplica(), newReplica()

	allowed := 0
	for i := 0; i < 6; i++ {
		h := a
		if i%2 == 1 {
			h = b
		}
		if hit(h, "10.0.0.1") == 200 {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("allowed %d across two replicas, want the shared burst of 3", allowed)
	}
	if hit(a, "10.0.0.2") != 200 {
		t.Fatal("a different IP has its own budget")
	}
}

func TestDistributedLimitFallsBackWhenRedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	h := DistributedRateLimit(rdb, "t", 1, 2, NewRateLimiterStore(1, 2, time.Minute))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	mr.Close()

	codes := []int{hit(h, "1.1.1.1"), hit(h, "1.1.1.1"), hit(h, "1.1.1.1")}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != 429 {
		t.Fatalf("fallback should still limit (burst 2): %v", codes)
	}
}

func TestToLimitKeepsSubSecondRates(t *testing.T) {
	l := toLimit(10.0/60.0, 5)
	if l.Rate != 10 || l.Period != time.Minute || l.Burst != 5 {
		t.Fatalf("10/min limit converted wrongly: %+v", l)
	}
}
