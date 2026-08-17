package middleware

import "net/http"

// Chain applies middleware in the order provided.
// Usage: Chain(handler, Logger, Recovery, CORS(...))
// Executes left to right: Logger → Recovery → CORS → handler
func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// Apply in reverse so execution order matches declaration order
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}