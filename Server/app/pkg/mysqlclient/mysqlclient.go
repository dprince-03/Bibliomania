// Package mysqlclient connects auth-service to its MySQL database.
package mysqlclient

import (
	"context"
	"strings"

	"github.com/dprince-03/Bibliomania/pkg/sqlretry"

	_ "github.com/go-sql-driver/mysql" // registers the "mysql" driver
	"github.com/jmoiron/sqlx"
)

const DriverName = "mysql"

// Connect opens a pool from a go-sql-driver DSN, e.g.
// auth:pw@tcp(mysql:3306)/bibliomania_auth. parseTime (scan DATETIME into
// time.Time) and multiStatements (a migration file may hold several
// statements) are added if the DSN doesn't set them.
func Connect(ctx context.Context, dsn string) (*sqlx.DB, error) {
	return sqlretry.Connect(ctx, DriverName, withDefaults(dsn))
}

func withDefaults(dsn string) string {
	for _, param := range []string{"parseTime=true", "multiStatements=true"} {
		name, _, _ := strings.Cut(param, "=")
		if strings.Contains(dsn, name+"=") {
			continue
		}
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + param
	}
	return dsn
}
