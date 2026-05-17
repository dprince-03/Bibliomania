package models

import "time"

type ReadingSession struct {
	ID              uint64     `db:"id"`
	UserID          uint64     `db:"user_id"`
	BookID          uint64     `db:"book_id"`
	CurrentPage     uint32     `db:"current_page"`
	TotalPages      uint32     `db:"total_pages"`
	ProgressPct     float64    `db:"progress_pct"`
	CurrentChapter  *string    `db:"current_chapter"`
	StartedAt       time.Time  `db:"started_at"`
	LastReadAt      time.Time  `db:"last_read_at"`
	CompletedAt     *time.Time `db:"completed_at"`
	IsCompleted     bool       `db:"is_completed"`
	ClientUpdatedAt *time.Time `db:"client_updated_at"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

type UserLibrary struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	BookID    uint64    `db:"book_id"`
	Status    string    `db:"status"`
	AddedAt   time.Time `db:"added_at"`
	UpdatedAt time.Time `db:"updated_at"`

	// Populated via JOIN
	Book *Book `db:"-"`
}

type Bookmark struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	BookID    uint64    `db:"book_id"`
	Page      uint32    `db:"page"`
	Note      *string   `db:"note"`
	Highlight *string   `db:"highlight"`
	Color     string    `db:"color"`
	CreatedAt time.Time `db:"created_at"`
}
