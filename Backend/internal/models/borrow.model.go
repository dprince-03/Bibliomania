package models

import "time"

type BorrowRecord struct {
	ID         uint64     `db:"id"`
	UserID     uint64     `db:"user_id"`
	BookID     uint64     `db:"book_id"`
	BorrowedAt time.Time  `db:"borrowed_at"`
	DueAt      time.Time  `db:"due_at"`
	ReturnedAt *time.Time `db:"returned_at"`
	Status     string     `db:"status"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
}
