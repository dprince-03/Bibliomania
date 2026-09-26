// Package borrow is borrow-service: borrowing and returns (Postgres +
// Kafka). Owns the orchestrated borrow Saga — ReserveCopy on
// catalog-service, create the record, ReleaseCopy to compensate — so the
// gateway calls one service and that service owns the cross-service
// coordination. Publishes borrow.* events to Kafka via the outbox.
package borrow

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
