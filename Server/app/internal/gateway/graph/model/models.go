// Package model holds the GraphQL types the gateway builds by hand. Fields
// in the schema that are missing here (Book.myBorrowStatus,
// BorrowRecord.book, LibraryEntry.book, ...) get their own resolvers —
// that's where the cross-service fan-out happens, and only when a query
// actually asks for them.
package model

type Book struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Isbn            string    `json:"isbn"`
	Genre           string    `json:"genre"`
	Description     *string   `json:"description,omitempty"`
	CoverImage      *string   `json:"coverImage,omitempty"`
	PublishedYear   *int      `json:"publishedYear,omitempty"`
	IsDigital       bool      `json:"isDigital"`
	FileFormat      *string   `json:"fileFormat,omitempty"`
	Authors         []*Author `json:"authors"`
	TotalCopies     int       `json:"totalCopies"`
	AvailableCopies int       `json:"availableCopies"`
	PriceCents      *int      `json:"priceCents,omitempty"`
	Currency        *string   `json:"currency,omitempty"`
}

type BorrowRecord struct {
	ID         string       `json:"id"`
	PublicID   string       `json:"publicId"`
	BookID     string       `json:"bookId"`
	BookTitle  string       `json:"bookTitle"`
	BorrowedAt string       `json:"borrowedAt"`
	DueAt      string       `json:"dueAt"`
	ReturnedAt *string      `json:"returnedAt,omitempty"`
	Status     BorrowStatus `json:"status"`
}

type LibraryEntry struct {
	BookID    string        `json:"bookId"`
	Status    LibraryStatus `json:"status"`
	AddedAt   string        `json:"addedAt"`
	UpdatedAt string        `json:"updatedAt"`
	// Prefetched by myLibrary in one batch call (GetBooksByIds), so the
	// book resolver doesn't do one catalog call per entry.
	Prefetched *Book `json:"-"`
	// Set when prefetching ran and this book wasn't found (deleted).
	Missing bool `json:"-"`
}

type ReadingHistoryEntry struct {
	BookID      string  `json:"bookId"`
	BookTitle   string  `json:"bookTitle"`
	CurrentPage int     `json:"currentPage"`
	TotalPages  int     `json:"totalPages"`
	ProgressPct float64 `json:"progressPct"`
	IsCompleted bool    `json:"isCompleted"`
	LastReadAt  string  `json:"lastReadAt"`
}
