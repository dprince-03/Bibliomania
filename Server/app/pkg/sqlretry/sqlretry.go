// Package sqlretry opens a database/sql pool with sqlx, retrying while the
// database is still starting (docker compose up, a pod scheduled before its
// StatefulSet is ready), and applies the pool settings every service uses.
// Shared by pkg/postgresclient and pkg/mysqlclient.
package sqlretry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	attempts = 15
	backoff  = 2 * time.Second
)

// Connect opens and pings a pool for driverName, retrying for ~30s.
func Connect(ctx context.Context, driverName, dsn string) (*sqlx.DB, error) {
	var db *sqlx.DB
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		db, err = sqlx.ConnectContext(ctx, driverName, dsn)
		if err == nil {
			break
		}
		slog.Warn("database not ready, retrying", "driver", driverName, "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", driverName, err)
	}

	db.SetMaxOpenConns(25)                 // max open connections to DB
	db.SetMaxIdleConns(10)                 // max idle connections kept in pool
	db.SetConnMaxLifetime(5 * time.Minute) // recycle connections every 5 min
	db.SetConnMaxIdleTime(2 * time.Minute) // close idle connections after 2 min

	slog.Info("database connected", "driver", driverName)
	return db, nil
}
