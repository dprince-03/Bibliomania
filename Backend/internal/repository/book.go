package repository

import (
	apperrors "bibliotheca/internal/errors"
	"bibliotheca/internal/models"
	"context"
	"database/sql"
	"errors"

	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
)

type bookRepository struct {
	db *sqlx.DB
}

func NewBookRepository(db *sqlx.DB) BookRepository {
	return &bookRepository{
		db: db,
	}
}

func (r *bookRepository) GetBookByID(ctx context.Context, id uint64) (*models.Book, error) {
	book := &models.Book{}
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

func (r *bookRepository) GetAllBooks(ctx context.Context, limit, offset int) ([]*models.Book, int, error) {
	var books []*models.Book
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

func (r *bookRepository) CreateBook(ctx context.Context, book *models.Book) (uint64, error) {
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

func (r *bookRepository) UpdateBook(ctx context.Context, book *models.Book) error {
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

func (r *bookRepository) DeleteBook(ctx context.Context, id uint64) error {
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

func (r *bookRepository) Search(ctx context.Context, query, genre string, year, limit, offset int) ([]*models.Book, int, error) {
	var books []*models.Book
	var total int

	// Build query dynamically based on filters provided
	conditions := []string{}
	args := []any{}

	if query != "" {
		conditions = append(conditions, "MATCH(title, description) AGAINST(? IN BOOLEAN MODE)")
		args = append(args, query)
	}
	if genre != "" {
		conditions = append(conditions, "genre = ?")
		args = append(args, genre)
	}
	if year > 0 {
		conditions = append(conditions, "published_year = ?")
		args = append(args, year)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM books %s", whereClause)
	if err := r.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	searchSQL := fmt.Sprintf(
		"SELECT * FROM books %s ORDER BY created_at DESC LIMIT ? OFFSET ?",
		whereClause,
	)
	args = append(args, limit, offset)
	if err := r.db.SelectContext(ctx, &books, searchSQL, args...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	return books, total, nil
}
