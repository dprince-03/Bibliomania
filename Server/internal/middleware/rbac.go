package middleware

import (
	"net/http"
	"slices"

	apperrors "github.com/yourusername/bibliotheca/internal/errors"
	"github.com/yourusername/bibliotheca/internal/utils"
)

// Role constants — single source of truth for all roles in the app.
// Never use raw strings like "admin" anywhere outside this file.
const (
	RoleAdmin     = "admin"
	RoleMember    = "member"
	RoleLibrarian = "librarian"
)

// RoleRequired returns a middleware that only allows requests from
// users whose role is in the allowed list.
// MUST be used after AuthGuard — it reads user role from context.
func RoleRequired(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := GetUserRole(r.Context())

			if userRole == "" {
				utils.Error(w, apperrors.Unauthorized("authentication required"))
				return
			}

			if !slices.Contains(roles, userRole) {
				utils.Error(w, apperrors.Forbidden(
					"you do not have permission to access this resource",
				))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SelfOrAdmin allows access if the requesting user is the resource owner
// OR has the admin role. Used for endpoints like PATCH /users/:id
// where a user can edit their own profile but admins can edit anyone's.
func SelfOrAdmin(getUserID func(r *http.Request) uint64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctxUserID := GetUserID(r.Context())
			ctxRole := GetUserRole(r.Context())

			resourceUserID := getUserID(r)

			isSelf := ctxUserID == resourceUserID
			isAdmin := ctxRole == RoleAdmin

			if !isSelf && !isAdmin {
				utils.Error(w, apperrors.Forbidden(
					"you do not have permission to access this resource",
				))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}