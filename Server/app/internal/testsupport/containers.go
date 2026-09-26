//go:build integration

// Package testsupport starts real, throwaway databases for integration
// tests (Testcontainers): `go test -tags integration ./...`. Needs Docker.
package testsupport

import (
	"context"
	"io/fs"
	"testing"
	"time"

	"github.com/dprince-03/Bibliomania/internal/database"
	"github.com/dprince-03/Bibliomania/pkg/mongoclient"
	"github.com/dprince-03/Bibliomania/pkg/mysqlclient"
	"github.com/dprince-03/Bibliomania/pkg/postgresclient"

	"github.com/jmoiron/sqlx"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Postgres starts postgres:16-alpine, applies migrations, returns a pool.
func Postgres(t *testing.T, migrations fs.FS) *sqlx.DB {
	t.Helper()
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("test"), tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("starting postgres: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := postgresclient.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(db, database.DriverPostgres, migrations); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return db
}

// MySQL starts mysql:8.4, applies migrations, returns a pool.
func MySQL(t *testing.T, migrations fs.FS) *sqlx.DB {
	t.Helper()
	ctx := context.Background()
	c, err := tcmysql.Run(ctx, "mysql:8.4",
		tcmysql.WithDatabase("test"), tcmysql.WithUsername("test"), tcmysql.WithPassword("test"))
	if err != nil {
		t.Fatalf("starting mysql: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	db, err := mysqlclient.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(db, database.DriverMySQL, migrations); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return db
}

// MongoReplicaSet starts a single-node mongo:7 replica set (transactions
// need one) and returns a connected client.
func MongoReplicaSet(t *testing.T) *mongo.Client {
	t.Helper()
	ctx := context.Background()
	c, err := tcmongo.Run(ctx, "mongo:7", tcmongo.WithReplicaSet("rs0"))
	if err != nil {
		t.Fatalf("starting mongo: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	uri, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client, err := mongoclient.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return client
}
