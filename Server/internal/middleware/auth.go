package middleware

import (
	"context"
	"net/http"
	"strings"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/utils"
	"github.com/dprince-03/Bibliotheca/pkg/jwt"
)

func AuthGuard(jwtManager *jwt.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				utils.Error(w, apperrors.Unauthorized("missing authorization header"))
				return
			}

			// 2. Check format: "Bearer <token>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				utils.Error(w, apperrors.Unauthorized("invalid authorization header format"))
				return
			}

			tokenStr := parts[1]
			if tokenStr == "" {
				utils.Error(w, apperrors.Unauthorized("missing token"))
				return
			}

			// 3. Parse and validate token
			claims, err := jwtManager.ParseAccessToken(tokenStr)
			if err != nil {
				utils.Error(w, apperrors.Unauthorized("invalid or expired token"))
				return
			}

			// 4. Inject user info into context
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
