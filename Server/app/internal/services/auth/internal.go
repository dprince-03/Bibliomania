package auth

import (
	"crypto/subtle"
	"net/http"
	"strconv"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/utils"
)

// AccountRecord is the identity data other services reconcile against —
// never the password hash.
type AccountRecord struct {
	ID        uint64 `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
}

// InternalRoutes registers service-internal endpoints: not under /api, so
// the gateway (which only proxies /api/v1 paths) never exposes them, and
// guarded by a shared token as a second lock. Disabled if token is empty.
func (h *Handler) InternalRoutes(mux *http.ServeMux, token string) {
	if token == "" {
		return
	}
	mux.Handle("GET /internal/v1/accounts", requireInternalToken(token, http.HandlerFunc(h.listAccounts)))
}

func requireInternalToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Token")), []byte(token)) != 1 {
			utils.Error(w, apperrors.Unauthorized("internal endpoint"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// listAccounts pages accounts by id: ?after_id=0&limit=500.
func (h *Handler) listAccounts(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseUint(r.URL.Query().Get("after_id"), 10, 64)
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 1000 {
		limit = 500
	}
	accounts, err := h.service.accountRepo.ListAfter(r.Context(), after, limit)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	out := make([]AccountRecord, len(accounts))
	for i, a := range accounts {
		out[i] = AccountRecord{ID: a.ID, Email: a.Email, FirstName: a.FirstName, LastName: a.LastName, Role: a.Role, IsActive: a.IsActive}
	}
	utils.Success(w, http.StatusOK, "accounts", out)
}
