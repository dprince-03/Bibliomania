package catalog

import "time"

type Author struct {
	ID          uint64     `db:"id"`
	FirstName   string     `db:"first_name"`
	LastName    string     `db:"last_name"`
	MiddleName  *string    `db:"middle_name"`
	Image       *string    `db:"image"`
	DateOfBirth *time.Time `db:"date_of_birth"`
	Biography   *string    `db:"biography"`
	Phone       *string    `db:"phone"`
	Email       *string    `db:"email"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}
