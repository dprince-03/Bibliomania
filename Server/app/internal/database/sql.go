// Package database applies a service's embedded migrations on boot.
// Connecting lives in pkg/postgresclient, pkg/mysqlclient and
// pkg/mongoclient. No service ever connects to another service's database —
// cross-service data goes over gRPC or events.
package database

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
)

// Driver names — the same values pkg/mysqlclient and pkg/postgresclient
// register, repeated here so migration callers don't import both.
const (
	DriverMySQL    = "mysql"
	DriverPostgres = "pgx"
)

// Migrate applies a service's embedded migrations (its own migrations/
// directory, compiled into the binary via embed.FS — so the prod image
// needs no migrations folder next to it).
func Migrate(db *sqlx.DB, driver string, migrations fs.FS) error {
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("reading embedded migrations: %w", err)
	}

	var dbDriver database.Driver
	switch driver {
	case DriverMySQL:
		dbDriver, err = migratemysql.WithInstance(db.DB, &migratemysql.Config{})
	case DriverPostgres:
		dbDriver, err = migratepgx.WithInstance(db.DB, &migratepgx.Config{})
	default:
		return fmt.Errorf("unsupported migration driver %q", driver)
	}
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, driver, dbDriver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("migrations already up to date")
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}
	slog.Info("migrations applied")
	return nil
}
