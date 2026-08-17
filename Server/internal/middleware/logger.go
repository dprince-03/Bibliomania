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

func (rw *responseWriter) WriteHeader(code int) {
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
			slog.Error(
				"request completed",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
			)
		case wrapped.statusCode >= 400:
			slog.Warn(
				"request completed",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
			)
		default:
			slog.Info(
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
