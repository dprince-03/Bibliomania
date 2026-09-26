package catalog

import "time"

type Book struct {
	ID              uint64  `db:"id"`
	Title           string  `db:"title"`
	ISBN            string  `db:"isbn"`
	Genre           string  `db:"genre"`
	Description     *string `db:"description"`
	CoverImage      *string `db:"cover_image"`
	PublishedYear   *int    `db:"published_year"`
	TotalCopies     int     `db:"total_copies"`
	AvailableCopies int     `db:"available_copies"`

	// E-Library
	FilePath      *string `db:"file_path"`
	FileSizeBytes *int64  `db:"file_size_bytes"`
	FileFormat    *string `db:"file_format"`
	IsDigital     bool    `db:"is_digital"`

	// Sale price (payment-service). Nil = not for sale.
	PriceCents *int64  `db:"price_cents"`
	Currency   *string `db:"currency"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	// Populated via JOIN — not a DB column
	Authors []Author `db:"-"`
}

// bookColumns is every real column (search_vector deliberately excluded —
// it's a generated tsvector with no Go field). Used instead of SELECT *.
const bookColumns = `b.id, b.title, b.isbn, b.genre, b.description, b.cover_image, b.published_year,
	b.total_copies, b.available_copies, b.file_path, b.file_size_bytes, b.file_format, b.is_digital,
	b.price_cents, b.currency, b.created_at, b.updated_at`
