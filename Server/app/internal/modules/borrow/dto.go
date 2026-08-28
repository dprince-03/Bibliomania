package borrow

import "time"

type BorrowRequest struct {
	BookID uint64 `json:"book_id" validate:"required"`
}

type BorrowResponse struct {
	ID         uint64     `json:"id"`
	BookID     uint64     `json:"book_id"`
	BookTitle  string     `json:"book_title"`
	BorrowedAt time.Time  `json:"borrowed_at"`
	DueAt      time.Time  `json:"due_at"`
	ReturnedAt *time.Time `json:"returned_at,omitempty"`
	Status     string     `json:"status"`
}
