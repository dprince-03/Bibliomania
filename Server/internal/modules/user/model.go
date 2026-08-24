package user

import "time"

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
