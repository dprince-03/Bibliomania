package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
)

// StaleTTL is how long a last-known-good copy of every cached read is kept
// for use while the database is down.
const StaleTTL = 24 * time.Hour

const stalePrefix = "stale:"

// ReadThrough returns key's cached value, or calls load and caches the
// result for ttl. Alongside the normal entry it keeps a long-lived stale
// copy; if load fails with an infrastructure error (not a 4xx such as
// "not found") and a stale copy exists, that copy is served instead —
// catalog browsing keeps working through a database outage, slightly out
// of date. A broken cache is never fatal: it just means a load.
func ReadThrough[T any](ctx context.Context, c Cache, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	var zero T
	if raw, err := c.Get(ctx, key); err == nil {
		var v T
		if json.Unmarshal([]byte(raw), &v) == nil {
			return v, nil
		}
	}

	v, err := load()
	if err != nil {
		if !isInfraError(err) {
			return zero, err
		}
		if raw, serr := c.Get(ctx, stalePrefix+key); serr == nil {
			var stale T
			if json.Unmarshal([]byte(raw), &stale) == nil {
				slog.WarnContext(ctx, "database unavailable — serving stale cached copy", "key", key, "error", err)
				return stale, nil
			}
		}
		return zero, err
	}

	if data, merr := json.Marshal(v); merr == nil {
		_ = c.Set(ctx, key, data, ttl)
		_ = c.Set(ctx, stalePrefix+key, data, StaleTTL)
	}
	return v, nil
}

func isInfraError(err error) bool {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Code >= http.StatusInternalServerError
	}
	return true
}
