// Package postgresclient connects catalog-, borrow-, user- and
// payment-service to their Postgres databases (pgx driver via database/sql,
// wrapped in sqlx).
package postgresclient

import (
	"context"

	"github.com/dprince-03/Bibliomania/pkg/sqlretry"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
	"github.com/jmoiron/sqlx"
)

// DriverName is the database/sql driver name — also what sqlx uses to pick
// $1-style bind variables, and what internal/database.Migrate expects.
const DriverName = "pgx"

// Connect opens a pool from a postgres:// URL, e.g.
// postgres://catalog:pw@postgres:5432/bibliomania_catalog?sslmode=disable.
func Connect(ctx context.Context, url string) (*sqlx.DB, error) {
	return sqlretry.Connect(ctx, DriverName, url)
}
