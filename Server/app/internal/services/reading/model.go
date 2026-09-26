// Package reading is reading-service: reading progress (online updates and
// offline sync), bookmarks, and reading history (MongoDB + NATS JetStream).
// Validates a book against catalog-service once, when a session or
// bookmark is created, and snapshots its title then — never on the
// high-frequency progress-update path. Publishes reading.* events.
package reading

import "time"

type ReadingSession struct {
	UserID         uint64     `bson:"user_id"`
	BookID         uint64     `bson:"book_id"`
	BookTitle      string     `bson:"book_title"`
	CurrentPage    uint32     `bson:"current_page"`
	TotalPages     uint32     `bson:"total_pages"`
	ProgressPct    float64    `bson:"progress_pct"`
	CurrentChapter *string    `bson:"current_chapter"`
	StartedAt      time.Time  `bson:"started_at"`
	LastReadAt     time.Time  `bson:"last_read_at"`
	CompletedAt    *time.Time `bson:"completed_at"`
	IsCompleted    bool       `bson:"is_completed"`
	// The last-write-wins clock, as Unix nanoseconds. Not a BSON date:
	// those only resolve to milliseconds, and two updates in the same
	// instant must not tie (the MySQL version needed DATETIME(6) for the
	// same reason — see the pre-split migration 000011).
	ClientUpdatedAtNs int64     `bson:"client_updated_at_ns"`
	CreatedAt         time.Time `bson:"created_at"`
	UpdatedAt         time.Time `bson:"updated_at"`
}

type Bookmark struct {
	// Sequential numeric IDs (from the counters collection), not ObjectIDs,
	// so the REST API's /bookmarks/{id} stays numeric as before the split.
	ID        uint64    `bson:"_id"`
	UserID    uint64    `bson:"user_id"`
	BookID    uint64    `bson:"book_id"`
	Page      uint32    `bson:"page"`
	Note      *string   `bson:"note"`
	Highlight *string   `bson:"highlight"`
	Color     string    `bson:"color"`
	CreatedAt time.Time `bson:"created_at"`
}
