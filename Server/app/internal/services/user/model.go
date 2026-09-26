// Package user is user-service: profiles, the personal library shelf, and
// admin user management (Postgres). Holds a replicated copy of each
// account (from auth.user_registered) and cached reading counters (from
// reading.book_completed). Publishes user.status_changed (RabbitMQ) when
// an admin (de)activates an account.
//
// No direct dependency on reading-service at all, and on catalog-service
// only to validate a book when it's added to the shelf — displaying
// library titles and reading history is the gateway's job.
package user

import (
	"embed"
	"time"
)

//go:embed migrations/*.sql
var Migrations embed.FS

type User struct {
	ID        uint64    `db:"id"`
	FirstName string    `db:"first_name"`
	LastName  string    `db:"last_name"`
	Email     string    `db:"email"`
	Role      string    `db:"role"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type UserProfile struct {
	ID             uint64     `db:"id"`
	UserID         uint64     `db:"user_id"`
	PhoneNumber    *string    `db:"phone_number"`
	Bio            *string    `db:"bio"`
	ProfilePicture *string    `db:"profile_picture"`
	LastOnlineAt   *time.Time `db:"last_online_at"`
	LastReadBookID *uint64    `db:"last_read_book_id"`
	TotalBooksRead uint32     `db:"total_books_read"`
	TotalPagesRead uint32     `db:"total_pages_read"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// UserLibrary is a member's personal book-status shelf (wishlist, reading,
// completed, ...). Book IDs only — titles are resolved live by the gateway.
type UserLibrary struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	BookID    uint64    `db:"book_id"`
	Status    string    `db:"status"`
	AddedAt   time.Time `db:"added_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
