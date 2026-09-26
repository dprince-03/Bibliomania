package middleware

import (
	"net/http"

	"github.com/dprince-03/Bibliomania/pkg/jwt"
)

// Guards bundles the AuthGuard + RoleRequired combinations every service's
// routes use, replacing the helper closures the old monolith router built
// once for itself.
type Guards struct {
	auth func(http.Handler) http.Handler
}

func NewGuards(jwtManager *jwt.Manager) Guards {
	return Guards{auth: AuthGuard(jwtManager)}
}

func (g Guards) Admin(h http.HandlerFunc) http.Handler {
	return g.auth(RoleRequired(RoleAdmin)(h))
}

func (g Guards) Librarian(h http.HandlerFunc) http.Handler {
	return g.auth(RoleRequired(RoleAdmin, RoleLibrarian)(h))
}

// Member is any authenticated user.
func (g Guards) Member(h http.HandlerFunc) http.Handler {
	return g.auth(RoleRequired(RoleAdmin, RoleLibrarian, RoleMember)(h))
}
