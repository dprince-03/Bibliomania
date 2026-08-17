package middleware

import (
	apperrors "bibliotheca/internal/errors"
	"bibliotheca/internal/utils"
	"log/slog"
	"net/http"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID := GetRequestID(r.Context())

				slog.Error(
					"panic recovered",
					"request_id", requestID,
					"method", r.Method,
					"path", r.URL.Path,
					"error", err,
				)

				utils.ErrorResponse(w, apperrors.Internal(nil))
			}
		}()

		next.ServeHTTP(w, r)
	})
}