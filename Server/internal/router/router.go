package router

import (
	"bibliotheca/internal/auth"
	"net/http"
)

func Routes(
	authHandler *auth.Handler,
) http.Handler {
	mux := http.NewServeMux()

	// ~~ Health check endpoint ~~
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{ "status": "ok", "service": "Bibliotheca Backend" }`))
	})

	// ~~ Auth endpoints ~~
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("POST /api/v1/auth/refresh", authHandler.RefreshToken)
	
	return mux
}