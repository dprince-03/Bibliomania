package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	UserRoleKey  contextKey = "user_role"
	UserEmailKey contextKey = "user_email"
	TokenKey     contextKey = "token"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use existing request ID from upstream proxy if present
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Inject into response headers so clients can trace requests
		w.Header().Set("X-Request-ID", requestID)
		// ...and onto the request itself, so the gateway's reverse proxy
		// forwards the same ID to whichever service handles the call.
		r.Header.Set("X-Request-ID", requestID)

		// Inject into context so handlers and logs can read it
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID pulls the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetUserID pulls the authenticated user ID from context.
func GetUserID(ctx context.Context) uint64 {
	if id, ok := ctx.Value(UserIDKey).(uint64); ok {
		return id
	}
	return 0
}

// GetUserRole pulls the authenticated user role from context.
func GetUserRole(ctx context.Context) string {
	if role, ok := ctx.Value(UserRoleKey).(string); ok {
		return role
	}
	return ""
}

// GetUserEmail pulls the authenticated user's email from context.
func GetUserEmail(ctx context.Context) string {
	if email, ok := ctx.Value(UserEmailKey).(string); ok {
		return email
	}
	return ""
}

// GetToken pulls the caller's raw bearer token from context (set by
// AuthGuard over HTTP, or by internal/grpcx over gRPC).
func GetToken(ctx context.Context) string {
	if t, ok := ctx.Value(TokenKey).(string); ok {
		return t
	}
	return ""
}

// WithRequestID stores a request ID in ctx — used by internal/grpcx to carry
// the ID across a gRPC hop.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// WithIdentity stores an already-verified caller in ctx. Used by
// internal/grpcx after it verifies a forwarded JWT, so gRPC handlers read
// identity through the same GetUserID/GetUserRole helpers as HTTP handlers.
func WithIdentity(ctx context.Context, userID uint64, role, email, token string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, UserRoleKey, role)
	ctx = context.WithValue(ctx, UserEmailKey, email)
	return context.WithValue(ctx, TokenKey, token)
}
