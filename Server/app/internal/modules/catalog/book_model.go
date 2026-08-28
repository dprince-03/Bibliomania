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

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	// Populated via JOIN — not a DB column
	Authors []Author `db:"-"`
}
