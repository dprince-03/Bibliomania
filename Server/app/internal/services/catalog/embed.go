// Package catalog is catalog-service: authors, books, search, e-library
// upload/download (Postgres + Redis cache + local file storage). Also the
// gRPC server other services call — most importantly ReserveCopy/
// ReleaseCopy, the catalog half of the borrow Saga. Publishes
// catalog.* events to RabbitMQ via the outbox.
//
// Still merges author + book in one package (author_*.go / book_*.go),
// for the same reason as before the split: BookResponse embeds authors and
// AuthorService.GetBooksByAuthor returns books.
package catalog

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
