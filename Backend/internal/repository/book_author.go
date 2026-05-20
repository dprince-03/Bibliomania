package repository

import (
	"context"

	apperrors "bibliotheca/internal/errors"
	"bibliotheca/internal/models"
	"github.com/jmoiron/sqlx"
)

type bookAuthorRepository struct {
	db *sqlx.DB
}

func NewBookAuthorRepository(db *sqlx.DB) BookAuthorRepository {
	return &bookAuthorRepository{db: db}
}

func (r *bookAuthorRepository) AssignAuthor(ctx context.Context, ba *models.BookAuthor) error {
	query := `
		INSERT INTO book_authors (book_id, author_id, role)
		VALUES (:book_id, :author_id, :role)
		ON DUPLICATE KEY UPDATE role = VALUES(role)
	`
	_, err := r.db.NamedExecContext(ctx, query, ba)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *bookAuthorRepository) RemoveAuthor(ctx context.Context, bookID, authorID uint64) error {
	query := `DELETE FROM book_authors WHERE book_id = ? AND author_id = ?`
	_, err := r.db.ExecContext(ctx, query, bookID, authorID)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *bookAuthorRepository) GetAuthorsByBookID(ctx context.Context, bookID uint64) ([]*models.Author, error) {
	var authors []*models.Author
	query := `
		SELECT a.*
		FROM authors a
		JOIN book_authors ba ON ba.author_id = a.id
		WHERE ba.book_id = ?
		ORDER BY ba.role ASC
	`
	if err := r.db.SelectContext(ctx, &authors, query, bookID); err != nil {
		return nil, apperrors.Internal(err)
	}
	return authors, nil
}

func (r *bookAuthorRepository) GetBooksByAuthorID(ctx context.Context, authorID uint64, limit, offset int) ([]*models.Book, int, error) {
	var books []*models.Book
	var total int

	countQuery := `SELECT COUNT(*) FROM book_authors WHERE author_id = ?`
	if err := r.db.GetContext(ctx, &total, countQuery, authorID); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `
		SELECT b.*
		FROM books b
		JOIN book_authors ba ON ba.book_id = b.id
		WHERE ba.author_id = ?
		ORDER BY b.created_at DESC
		LIMIT ? OFFSET ?
	`
	if err := r.db.SelectContext(ctx, &books, query, authorID, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return books, total, nil
}
