package middleware

import (
	"context"
	"net/http"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/utils"
)

// OwnershipChecker is a function that checks whether the requesting user
// owns the resource being accessed.
// Implement one per resource type (borrow record, bookmark, etc.)
type OwnershipChecker func(ctx context.Context, userID uint64, resourceID uint64) (bool, error)

// OwnershipRequired checks that the authenticated user owns the resource.
// getResourceID extracts the resource ID from the request (e.g. from URL path).
// MUST be used after AuthGuard.
func OwnershipRequired(
	checker OwnershipChecker,
	getResourceID func(r *http.Request) uint64,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			role := GetUserRole(r.Context())

			// Admins bypass ownership checks
			if role == RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			resourceID := getResourceID(r)

			owns, err := checker(r.Context(), userID, resourceID)
			if err != nil {
				utils.Error(w, apperrors.Internal(err))
				return
			}

			if !owns {
				utils.Error(w, apperrors.Forbidden(
					"you do not have permission to access this resource",
				))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
