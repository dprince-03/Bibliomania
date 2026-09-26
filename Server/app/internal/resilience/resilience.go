// Package resilience holds the failure-isolation primitives shared by
// service-to-service calls (internal/grpcx) and the gateway's REST proxy:
//
//   - Breaker: a circuit breaker (sony/gobreaker). After enough consecutive
//     infrastructure failures to one dependency it "opens" and fails every
//     call instantly for a cool-down, then lets a few trial calls through
//     ("half-open") and closes again if they succeed. Callers get a fast,
//     clear 503 instead of each waiting out a timeout against a dead
//     service — and the dead service gets room to recover.
//   - Bulkhead: caps concurrent in-flight calls to one dependency, so a slow
//     upstream can tie up at most its own share of goroutines/connections,
//     not the whole caller.
//
// Both export state as Prometheus metrics (breaker_state,
// bulkhead_rejected_total).
package resilience

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sony/gobreaker/v2"
)

var (
	// ErrOpen: the breaker is open — the call wasn't attempted.
	ErrOpen = errors.New("circuit breaker open: dependency is failing, not attempted")
	// ErrBulkheadFull: too many calls already in flight to this dependency.
	ErrBulkheadFull = errors.New("bulkhead full: too many concurrent calls to this dependency")

	breakerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "breaker_state",
		Help: "Circuit breaker state per dependency: 0 closed, 1 half-open, 2 open.",
	}, []string{"dependency"})
	breakerTransitions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "breaker_transitions_total",
		Help: "Circuit breaker state changes per dependency and new state.",
	}, []string{"dependency", "to"})
	bulkheadRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bulkhead_rejected_total",
		Help: "Calls rejected because the dependency's bulkhead was full.",
	}, []string{"dependency"})
)

// Breaker wraps gobreaker with the policy every dependency uses.
type Breaker struct {
	cb *gobreaker.CircuitBreaker[any]
}

// BreakerPolicy: trip after 5 consecutive failures, or ≥50% failures over
// ≥20 calls in a 30s window; stay open 10s; allow 3 trial calls half-open.
// isFailure decides what counts — only infrastructure failures should
// (a 404 or a validation error means the dependency is healthy).
func NewBreaker(dependency string, isFailure func(error) bool) *Breaker {
	breakerState.WithLabelValues(dependency).Set(0)
	return &Breaker{cb: gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        dependency,
		MaxRequests: 3,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= 5 ||
				(c.Requests >= 20 && float64(c.TotalFailures)/float64(c.Requests) >= 0.5)
		},
		IsSuccessful: func(err error) bool { return err == nil || !isFailure(err) },
		OnStateChange: func(name string, from, to gobreaker.State) {
			breakerState.WithLabelValues(name).Set(float64(to))
			breakerTransitions.WithLabelValues(name, to.String()).Inc()
			slog.Warn("circuit breaker state change", "dependency", name, "from", from.String(), "to", to.String())
		},
	})}
}

// Do runs fn through the breaker. Returns ErrOpen (wrapped) without calling
// fn while open; otherwise fn's own error.
func (b *Breaker) Do(fn func() error) error {
	_, err := b.cb.Execute(func() (any, error) { return nil, fn() })
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return ErrOpen
	}
	return err
}

// State for tests/diagnostics.
func (b *Breaker) State() string { return b.cb.State().String() }

// Bulkhead is a counting semaphore with a short wait: a call queues for up
// to wait, then is rejected rather than piling up behind a slow dependency.
type Bulkhead struct {
	name  string
	slots chan struct{}
	wait  time.Duration
}

func NewBulkhead(dependency string, maxConcurrent int, wait time.Duration) *Bulkhead {
	return &Bulkhead{name: dependency, slots: make(chan struct{}, maxConcurrent), wait: wait}
}

// Acquire takes a slot (release it with the returned func) or fails with
// ErrBulkheadFull.
func (b *Bulkhead) Acquire(ctx context.Context) (func(), error) {
	select {
	case b.slots <- struct{}{}:
		return func() { <-b.slots }, nil
	default:
	}
	t := time.NewTimer(b.wait)
	defer t.Stop()
	select {
	case b.slots <- struct{}{}:
		return func() { <-b.slots }, nil
	case <-t.C:
		bulkheadRejected.WithLabelValues(b.name).Inc()
		return nil, ErrBulkheadFull
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
