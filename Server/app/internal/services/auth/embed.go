// Package auth is auth-service: registration, login, logout and refresh-
// token rotation. Owns the `accounts` credential copy and refresh tokens
// (MySQL). Publishes auth.user_registered (RabbitMQ, via the outbox);
// consumes user.status_changed so a deactivated account can't log in.
package auth

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
