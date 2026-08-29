package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"

	"github.com/jmoiron/sqlx"
)

type BookRepository interface {
	GetByID(ctx context.Context, id uint64) (*Book, error)
	GetAll(ctx context.Context, limit, offset int) ([]*Book, int, error)
	Create(ctx context.Context, book *Book) (uint64, error)
	Update(ctx context.Context, book *Book) error
	Delete(ctx context.Context, id uint64) error
	UpdateFilePath(ctx context.Context, id uint64, path string, size int64, format string) error
	DecrementAvailableCopies(ctx context.Context, id uint64) error
	IncrementAvailableCopies(ctx context.Context, id uint64) error
	Search(ctx context.Context, params BookSearchParams, limit, offset int) ([]*Book, int, error)
}

type bookRepository struct {
	db *sqlx.DB
}

func NewBookRepository(db *sqlx.DB) BookRepository {
	return &bookRepository{
		db: db,
	}
}

func (r *bookRepository) GetByID(ctx context.Context, id uint64) (*Book, error) {
	book := &Book{}
	query := `SELECT * FROM books WHERE id = ?`

	err := r.db.GetContext(ctx, book, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("book")
		}
		return nil, apperrors.Internal(err)
	}
	return book, nil
}

func (r *bookRepository) GetAll(ctx context.Context, limit, offset int) ([]*Book, int, error) {
	var books []*Book
	var total int

	countQuery := `SELECT COUNT(*) FROM books`
	if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT * FROM books ORDER BY created_at DESC LIMIT ? OFFSET ?`
	if err := r.db.SelectContext(ctx, &books, query, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return books, total, nil
}

func (r *bookRepository) Create(ctx context.Context, book *Book) (uint64, error) {
	query := `
				INSERT INTO books (title, isbn, genre, description, cover_image, published_year, total_copies, available_copies, is_digital)
				VALUES (:title, :isbn, :genre, :description, :cover_image, :published_year, :total_copies, :available_copies, :is_digital)
			`

	result, err := r.db.NamedExecContext(ctx, query, book)
	if err != nil {
		return 0, apperrors.Internal(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, apperrors.Internal(err)
	}
	return uint64(id), nil
}

func (r *bookRepository) Update(ctx context.Context, book *Book) error {
	query := `
				UPDATE books
				SET title = :title,
					genre = :genre,
					description = :description,
					cover_image = :cover_image,
					published_year = :published_year,
					total_copies = :total_copies,
					is_digital = :is_digital
				WHERE id = :id
			`

	_, err := r.db.NamedExecContext(ctx, query, book)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *bookRepository) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM books WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *bookRepository) UpdateFilePath(ctx context.Context, id uint64, path string, size int64, format string) error {
	query := `
				UPDATE books
				SET file_path = ?, file_size_bytes = ?, file_format = ?, is_digital = TRUE
				WHERE id = ?
			`

	_, err := r.db.ExecContext(ctx, query, path, size, format, id)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *bookRepository) DecrementAvailableCopies(ctx context.Context, id uint64) error {
	// Only decrement if copies are available — atomic check + update
	query := `
		UPDATE books
		SET available_copies = available_copies - 1
		WHERE id = ? AND available_copies > 0
	`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return apperrors.Internal(err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return apperrors.Internal(err)
	}
	if rows == 0 {
		return apperrors.BadRequest("no available copies for this book", nil)
	}
	return nil
}

func (r *bookRepository) IncrementAvailableCopies(ctx context.Context, id uint64) error {
	query := `
		UPDATE books
		SET available_copies = available_copies + 1
		WHERE id = ? AND available_copies < total_copies
	`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

// Search filters books by free-text query (matches title/description via
// MySQL full-text search, OR author name), plus exact-match genre/format/
// author/year filters. The author join is always present — needed both for
// the free-text author-name match and the author_id filter — but GROUP BY
// keeps one row per book even when a book has several matching authors.
func (r *bookRepository) Search(ctx context.Context, params BookSearchParams, limit, offset int) ([]*Book, int, error) {
	var books []*Book
	var total int

	conditions := []string{}
	args := []any{}

	if params.Query != "" {
		conditions = append(conditions,
			"(MATCH(b.title, b.description) AGAINST(? IN BOOLEAN MODE) OR a.first_name LIKE ? OR a.last_name LIKE ?)")
		like := "%" + params.Query + "%"
		args = append(args, params.Query, like, like)
	}
	if params.Genre != "" {
		conditions = append(conditions, "b.genre = ?")
		args = append(args, params.Genre)
	}
	switch params.Format {
	case "digital":
		conditions = append(conditions, "b.is_digital = TRUE")
	case "physical":
		conditions = append(conditions, "b.is_digital = FALSE")
	}
	if params.AuthorID > 0 {
		conditions = append(conditions, "ba.author_id = ?")
		args = append(args, params.AuthorID)
	}
	if params.Year > 0 {
		conditions = append(conditions, "b.published_year = ?")
		args = append(args, params.Year)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	const fromClause = `
		FROM books b
		LEFT JOIN book_authors ba ON ba.book_id = b.id
		LEFT JOIN authors a ON a.id = ba.author_id
	`

	countSQL := fmt.Sprintf("SELECT COUNT(DISTINCT b.id) %s %s", fromClause, whereClause)
	if err := r.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	searchSQL := fmt.Sprintf(
		"SELECT b.* %s %s GROUP BY b.id ORDER BY b.created_at DESC LIMIT ? OFFSET ?",
		fromClause, whereClause,
	)
	args = append(args, limit, offset)
	if err := r.db.SelectContext(ctx, &books, searchSQL, args...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	return books, total, nil
}
