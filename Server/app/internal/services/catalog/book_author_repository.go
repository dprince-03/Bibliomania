package catalog

import (
	"context"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"

	"github.com/jmoiron/sqlx"
)

type BookAuthorRepository interface {
	AssignAuthor(ctx context.Context, ba *BookAuthor) error
	RemoveAuthor(ctx context.Context, bookID, authorID uint64) error
	GetAuthorsByBookID(ctx context.Context, bookID uint64) ([]*Author, error)
	// GetAuthorsByBookIDs loads authors for many books in one query — the
	// old per-book loop was an N+1.
	GetAuthorsByBookIDs(ctx context.Context, bookIDs []uint64) (map[uint64][]*Author, error)
	GetBooksByAuthorID(ctx context.Context, authorID uint64, limit, offset int) ([]*Book, int, error)
	WithTx(tx *sqlx.Tx) BookAuthorRepository
}

type bookAuthorRepository struct {
	q sqlx.ExtContext
}

func NewBookAuthorRepository(db *sqlx.DB) BookAuthorRepository {
	return &bookAuthorRepository{q: db}
}

func (r *bookAuthorRepository) WithTx(tx *sqlx.Tx) BookAuthorRepository {
	return &bookAuthorRepository{q: tx}
}

func (r *bookAuthorRepository) AssignAuthor(ctx context.Context, ba *BookAuthor) error {
	query := `
		INSERT INTO book_authors (book_id, author_id, role)
		VALUES (:book_id, :author_id, :role)
		ON CONFLICT (book_id, author_id) DO UPDATE SET role = EXCLUDED.role
	`
	if _, err := sqlx.NamedExecContext(ctx, r.q, query, ba); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *bookAuthorRepository) RemoveAuthor(ctx context.Context, bookID, authorID uint64) error {
	query := `DELETE FROM book_authors WHERE book_id = $1 AND author_id = $2`
	if _, err := r.q.ExecContext(ctx, query, bookID, authorID); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *bookAuthorRepository) GetAuthorsByBookID(ctx context.Context, bookID uint64) ([]*Author, error) {
	byBook, err := r.GetAuthorsByBookIDs(ctx, []uint64{bookID})
	if err != nil {
		return nil, err
	}
	return byBook[bookID], nil
}

type authorWithBook struct {
	Author
	BookID uint64 `db:"book_id"`
}

func (r *bookAuthorRepository) GetAuthorsByBookIDs(ctx context.Context, bookIDs []uint64) (map[uint64][]*Author, error) {
	out := make(map[uint64][]*Author, len(bookIDs))
	if len(bookIDs) == 0 {
		return out, nil
	}

	query, args, err := sqlx.In(`
		SELECT a.id, a.first_name, a.last_name, a.middle_name, a.image, a.date_of_birth,
		       a.biography, a.phone, a.email, a.created_at, a.updated_at, ba.book_id
		FROM authors a
		JOIN book_authors ba ON ba.author_id = a.id
		WHERE ba.book_id IN (?)
		ORDER BY ba.book_id, ba.role ASC
	`, bookIDs)
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	var rows []authorWithBook
	if err := sqlx.SelectContext(ctx, r.q, &rows, r.q.Rebind(query), args...); err != nil {
		return nil, apperrors.Internal(err)
	}
	for i := range rows {
		a := rows[i].Author
		out[rows[i].BookID] = append(out[rows[i].BookID], &a)
	}
	return out, nil
}

func (r *bookAuthorRepository) GetBooksByAuthorID(ctx context.Context, authorID uint64, limit, offset int) ([]*Book, int, error) {
	var books []*Book
	var total int

	countQuery := `SELECT COUNT(*) FROM book_authors WHERE author_id = $1`
	if err := sqlx.GetContext(ctx, r.q, &total, countQuery, authorID); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `
		SELECT ` + bookColumns + `
		FROM books b
		JOIN book_authors ba ON ba.book_id = b.id
		WHERE ba.author_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3
	`
	if err := sqlx.SelectContext(ctx, r.q, &books, query, authorID, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return books, total, nil
}
