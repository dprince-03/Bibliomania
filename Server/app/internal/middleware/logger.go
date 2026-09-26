package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader records the final status. 1xx statuses (e.g. the "100
// Continue" a reverse proxy relays for Expect: 100-continue uploads) are
// informational — passed through without counting as "written", otherwise
// the real status that follows would be swallowed and the client would get
// an implicit 200 with an error body.
func (rw *responseWriter) WriteHeader(code int) {
	if code >= 100 && code < 200 {
		rw.ResponseWriter.WriteHeader(code)
		return
	}
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := newResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		requestID := GetRequestID(r.Context())

		// Choose log level based on status code
		switch {
		case wrapped.statusCode >= 500:
			slog.ErrorContext(r.Context(),
				"request completed",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
			)
		case wrapped.statusCode >= 400:
			slog.WarnContext(r.Context(),
				"request completed",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
			)
		default:
			slog.InfoContext(r.Context(),
				"request completed",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
			)
		}
	})
}
