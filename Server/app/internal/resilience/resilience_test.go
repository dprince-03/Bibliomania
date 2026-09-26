package resilience

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

var errDown = errors.New("connection refused")
var errNotFound = errors.New("not found")

func isInfra(err error) bool { return errors.Is(err, errDown) }

func TestBreakerOpensOnInfraFailuresOnly(t *testing.T) {
	b := NewBreaker("test-dep", isInfra)

	// Business errors never trip it.
	for i := 0; i < 20; i++ {
		_ = b.Do(func() error { return errNotFound })
	}
	if b.State() != "closed" {
		t.Fatalf("404s tripped the breaker: %s", b.State())
	}

	// 5 consecutive infrastructure failures open it…
	for i := 0; i < 5; i++ {
		_ = b.Do(func() error { return errDown })
	}
	if b.State() != "open" {
		t.Fatalf("want open, got %s", b.State())
	}
	// …and while open, calls fail fast without running.
	ran := false
	if err := b.Do(func() error { ran = true; return nil }); !errors.Is(err, ErrOpen) || ran {
		t.Fatalf("open breaker must not call through: err=%v ran=%v", err, ran)
	}
}

func TestBulkheadCapsConcurrency(t *testing.T) {
	bh := NewBulkhead("test-bh", 2, 20*time.Millisecond)
	ctx := context.Background()
	r1, _ := bh.Acquire(ctx)
	r2, _ := bh.Acquire(ctx)
	if _, err := bh.Acquire(ctx); !errors.Is(err, ErrBulkheadFull) {
		t.Fatalf("third call should be rejected, got %v", err)
	}
	r1()
	r3, err := bh.Acquire(ctx)
	if err != nil {
		t.Fatalf("slot freed, should acquire: %v", err)
	}
	r2()
	r3()

	// Under load: never more than 2 inside.
	var mu sync.Mutex
	inside, peak := 0, 0
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := bh.Acquire(ctx)
			if err != nil {
				return
			}
			mu.Lock()
			inside++
			peak = max(peak, inside)
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			inside--
			mu.Unlock()
			release()
		}()
	}
	wg.Wait()
	if peak > 2 {
		t.Fatalf("peak concurrency %d exceeded the bulkhead of 2", peak)
	}
}
