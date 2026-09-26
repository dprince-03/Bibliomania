// Package health serves GET /health for every service: 200 "ok" only if
// each dependency that service registered answers, 503 "degraded"
// otherwise — real liveness, not just "the process is running".
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const pingTimeout = 2 * time.Second

// Check pings one dependency.
type Check func(ctx context.Context) error

// Checker holds a service's named dependency checks (database, cache,
// broker, downstream services...).
type Checker struct {
	service string
	checks  map[string]Check
}

func NewChecker(service string) *Checker {
	return &Checker{service: service, checks: map[string]Check{}}
}

// Add registers a named check; returns the Checker for chaining.
func (c *Checker) Add(name string, check Check) *Checker {
	c.checks[name] = check
	return c
}

type status struct {
	Status  string            `json:"status"`
	Service string            `json:"service"`
	Checks  map[string]string `json:"checks"`
}

// Handle godoc
//
//	@Summary		Liveness check
//	@Description	Pings every dependency of the service answering (database, cache, broker, downstream services); returns 200 "ok" only if all are reachable, 503 "degraded" otherwise. The gateway's /health also checks every service behind it.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	status
//	@Failure		503	{object}	status
//	@Router			/health [get]
func (c *Checker) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
	defer cancel()

	resp := status{Status: "ok", Service: c.service, Checks: map[string]string{}}
	var mu sync.Mutex
	var wg sync.WaitGroup
	healthy := true

	for name, check := range c.checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := "ok"
			if err := check(ctx); err != nil {
				result = "unreachable"
			}
			mu.Lock()
			defer mu.Unlock()
			resp.Checks[name] = result
			if result != "ok" {
				healthy = false
			}
		}()
	}
	wg.Wait()

	statusCode := http.StatusOK
	if !healthy {
		resp.Status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// HTTPCheck returns a Check that GETs url and expects a 2xx.
func HTTPCheck(client *http.Client, url string) Check {
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			return &httpStatusError{code: resp.StatusCode}
		}
		return nil
	}
}

type httpStatusError struct{ code int }

func (e *httpStatusError) Error() string { return http.StatusText(e.code) }
