package middleware

import (
	"log/slog"
	"net/http"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/utils"
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

				utils.Error(w, apperrors.Internal(nil))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
