package idempotency

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setup(t *testing.T, handler http.HandlerFunc) (http.Handler, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mw := New(rdb, func(r *http.Request) string { return r.Header.Get("X-User") })
	return mw.Handler(handler), mr
}

func do(h http.Handler, method, key, user, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/v1/things", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set(Header, key)
	}
	r.Header.Set("X-User", user)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestReplaysFirstResponse(t *testing.T) {
	var calls atomic.Int32
	h, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"n":` + string('0'+n) + `,"echo":` + string(body) + `}`))
	})

	first := do(h, http.MethodPost, "k1", "u1", `{"a":1}`)
	second := do(h, http.MethodPost, "k1", "u1", `{"a":1}`)
	if calls.Load() != 1 {
		t.Fatalf("handler ran %d times", calls.Load())
	}
	if second.Code != http.StatusCreated || second.Body.String() != first.Body.String() || second.Header().Get(ReplayedHeader) != "true" {
		t.Fatalf("replay mismatch: %d %q vs %q", second.Code, second.Body.String(), first.Body.String())
	}

	// Different user, same key: independent.
	do(h, http.MethodPost, "k1", "u2", `{"a":1}`)
	if calls.Load() != 2 {
		t.Fatalf("keys must be scoped per caller, calls=%d", calls.Load())
	}
	// No key: never deduplicated.
	do(h, http.MethodPost, "", "u1", `{"a":1}`)
	do(h, http.MethodPost, "", "u1", `{"a":1}`)
	if calls.Load() != 4 {
		t.Fatalf("keyless requests must always run, calls=%d", calls.Load())
	}
}

func TestKeyReuseWithDifferentBody(t *testing.T) {
	h, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	do(h, http.MethodPost, "k", "u", `{"a":1}`)
	if w := do(h, http.MethodPost, "k", "u", `{"a":2}`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422", w.Code)
	}
}

func TestServerErrorsAreNotStored(t *testing.T) {
	var calls atomic.Int32
	h, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
	do(h, http.MethodPost, "k", "u", `{}`)
	if w := do(h, http.MethodPost, "k", "u", `{}`); w.Code != http.StatusCreated || calls.Load() != 2 {
		t.Fatalf("retry after 5xx should run again: code=%d calls=%d", w.Code, calls.Load())
	}
}

func TestConcurrentDuplicatesRunOnce(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	h, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-release
		w.WriteHeader(http.StatusCreated)
	})

	codes := make(chan int, 10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- do(h, http.MethodPost, "same", "u", `{}`).Code
		}()
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	close(codes)

	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if calls.Load() != 1 || counts[http.StatusCreated] != 1 || counts[http.StatusConflict] != 9 {
		t.Fatalf("calls=%d codes=%v — want exactly one execution, nine 409s", calls.Load(), counts)
	}
}

func TestRedisDownFailsOpen(t *testing.T) {
	var calls atomic.Int32
	h, mr := setup(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusCreated) })
	mr.Close()
	if w := do(h, http.MethodPost, "k", "u", `{}`); w.Code != http.StatusCreated || calls.Load() != 1 {
		t.Fatalf("should run unprotected when Redis is down: %d", w.Code)
	}
}
