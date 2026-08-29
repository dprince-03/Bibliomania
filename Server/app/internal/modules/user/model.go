package user

import (
	"time"

	"github.com/dprince-03/Bibliomania/internal/modules/catalog"
)

type User struct {
	ID        uint64    `db:"id"`
	FirstName string    `db:"first_name"`
	LastName  string    `db:"last_name"`
	Email     string    `db:"email"`
	Password  string    `db:"password"`
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
// completed, ...) — moved here from internal/modules/reading during Step 17,
// since it's conceptually a member-management concept ("my library") the
// roadmap itself groups with profile/history, not a reading-progress concept.
type UserLibrary struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	BookID    uint64    `db:"book_id"`
	Status    string    `db:"status"`
	AddedAt   time.Time `db:"added_at"`
	UpdatedAt time.Time `db:"updated_at"`

	// Populated via JOIN
	Book *catalog.Book `db:"-"`
}
