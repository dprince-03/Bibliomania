package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/grpcx"

	"github.com/jmoiron/sqlx"
)

type BookRepository interface {
	GetByID(ctx context.Context, id uint64) (*Book, error)
	GetByIDs(ctx context.Context, ids []uint64) ([]*Book, error)
	GetAll(ctx context.Context, limit, offset int) ([]*Book, int, error)
	Create(ctx context.Context, book *Book) (uint64, error)
	// Update writes book's editable fields. available_copies is never
	// written as an absolute value (a concurrent ReserveCopy/ReleaseCopy
	// would be lost): it moves by copiesDelta, atomically, and the update
	// is refused (ErrCopiesOnLoan) if that would take it below zero — i.e.
	// total_copies below the number currently on loan.
	Update(ctx context.Context, book *Book, copiesDelta int) error
	Delete(ctx context.Context, id uint64) error
	UpdateFilePath(ctx context.Context, id uint64, path string, size int64, format string) error
	Search(ctx context.Context, params BookSearchParams, limit, offset int) ([]*Book, int, error)

	// ReserveCopy/ReleaseCopy must run inside a transaction (WithTx) — the
	// reservation row, the copy count and the outbox event move together.
	ReserveCopy(ctx context.Context, bookID uint64, key string) (reserved bool, err error)
	ReleaseCopy(ctx context.Context, key string) (bookID uint64, released bool, err error)

	WithTx(tx *sqlx.Tx) BookRepository
}

type bookRepository struct {
	q sqlx.ExtContext
}

func NewBookRepository(db *sqlx.DB) BookRepository {
	return &bookRepository{q: db}
}

func (r *bookRepository) WithTx(tx *sqlx.Tx) BookRepository {
	return &bookRepository{q: tx}
}

func (r *bookRepository) GetByID(ctx context.Context, id uint64) (*Book, error) {
	book := &Book{}
	query := `SELECT ` + bookColumns + ` FROM books b WHERE b.id = $1`

	err := sqlx.GetContext(ctx, r.q, book, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("book")
		}
		return nil, apperrors.Internal(err)
	}
	return book, nil
}

func (r *bookRepository) GetByIDs(ctx context.Context, ids []uint64) ([]*Book, error) {
	var books []*Book
	if len(ids) == 0 {
		return books, nil
	}
	query, args, err := sqlx.In(`SELECT `+bookColumns+` FROM books b WHERE b.id IN (?)`, ids)
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	if err := sqlx.SelectContext(ctx, r.q, &books, r.q.Rebind(query), args...); err != nil {
		return nil, apperrors.Internal(err)
	}
	return books, nil
}

func (r *bookRepository) GetAll(ctx context.Context, limit, offset int) ([]*Book, int, error) {
	var books []*Book
	var total int

	if err := sqlx.GetContext(ctx, r.q, &total, `SELECT COUNT(*) FROM books`); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT ` + bookColumns + ` FROM books b ORDER BY b.created_at DESC LIMIT $1 OFFSET $2`
	if err := sqlx.SelectContext(ctx, r.q, &books, query, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return books, total, nil
}

func (r *bookRepository) Create(ctx context.Context, book *Book) (uint64, error) {
	query := `
		INSERT INTO books (title, isbn, genre, description, cover_image, published_year,
		                   total_copies, available_copies, is_digital, price_cents, currency)
		VALUES (:title, :isbn, :genre, :description, :cover_image, :published_year,
		        :total_copies, :available_copies, :is_digital, :price_cents, :currency)
		RETURNING id
	`
	return insertReturningID(ctx, r.q, query, book)
}

func (r *bookRepository) Update(ctx context.Context, book *Book, copiesDelta int) error {
	var available int
	err := sqlx.GetContext(ctx, r.q, &available, `
		UPDATE books
		SET title = $1,
			genre = $2,
			description = $3,
			cover_image = $4,
			published_year = $5,
			total_copies = total_copies + $6,
			available_copies = available_copies + $6,
			is_digital = $7,
			price_cents = $8,
			currency = $9
		WHERE id = $10 AND available_copies + $6 >= 0
		RETURNING available_copies`,
		book.Title, book.Genre, book.Description, book.CoverImage, book.PublishedYear,
		copiesDelta, book.IsDigital, book.PriceCents, book.Currency, book.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCopiesOnLoan
	}
	if err != nil {
		return apperrors.Internal(err)
	}
	book.AvailableCopies = available
	return nil
}

// ErrCopiesOnLoan: total_copies can't drop below the copies currently borrowed.
var ErrCopiesOnLoan = apperrors.Conflict("cannot reduce total_copies below the number of copies currently on loan")

func (r *bookRepository) Delete(ctx context.Context, id uint64) error {
	if _, err := r.q.ExecContext(ctx, `DELETE FROM books WHERE id = $1`, id); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *bookRepository) UpdateFilePath(ctx context.Context, id uint64, path string, size int64, format string) error {
	query := `
		UPDATE books
		SET file_path = $1, file_size_bytes = $2, file_format = $3, is_digital = TRUE
		WHERE id = $4
	`
	if _, err := r.q.ExecContext(ctx, query, path, size, format, id); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

// ReserveCopy is the atomic "take a copy" step of the borrow Saga, made
// idempotent by key:
//   - first call with a key: record the reservation and decrement
//     available_copies (only if > 0 — the real concurrency guard);
//   - a retry with the same key: report the original outcome, touch nothing.
//
// Returns grpcx.ErrNoCopies when the book exists but has none left.
func (r *bookRepository) ReserveCopy(ctx context.Context, bookID uint64, key string) (bool, error) {
	res, err := r.q.ExecContext(ctx, `
		INSERT INTO copy_reservations (idempotency_key, book_id, status)
		VALUES ($1, $2, 'reserved')
		ON CONFLICT (idempotency_key) DO NOTHING
	`, key, bookID)
	if err != nil {
		return false, apperrors.Internal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Key seen before — idempotent replay.
		var existing struct {
			BookID uint64 `db:"book_id"`
			Status string `db:"status"`
		}
		if err := sqlx.GetContext(ctx, r.q, &existing,
			`SELECT book_id, status FROM copy_reservations WHERE idempotency_key = $1`, key); err != nil {
			return false, apperrors.Internal(err)
		}
		if existing.BookID != bookID {
			return false, apperrors.BadRequest("idempotency key already used for a different book", nil)
		}
		return existing.Status == "reserved", nil
	}

	res, err = r.q.ExecContext(ctx, `
		UPDATE books
		SET available_copies = available_copies - 1
		WHERE id = $1 AND available_copies > 0
	`, bookID)
	if err != nil {
		return false, apperrors.Internal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Either no such book or no copies — the caller rolls back, which
		// also discards the reservation row inserted above.
		if _, err := r.GetByID(ctx, bookID); err != nil {
			return false, err
		}
		return false, grpcx.ErrNoCopies
	}
	return true, nil
}

// ReleaseCopy gives back the copy held by key, at most once. Unknown or
// already-released keys are a successful no-op (released=false).
func (r *bookRepository) ReleaseCopy(ctx context.Context, key string) (uint64, bool, error) {
	var bookID uint64
	err := sqlx.GetContext(ctx, r.q, &bookID, `
		UPDATE copy_reservations
		SET status = 'released'
		WHERE idempotency_key = $1 AND status = 'reserved'
		RETURNING book_id
	`, key)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, apperrors.Internal(err)
	}

	if _, err := r.q.ExecContext(ctx, `
		UPDATE books
		SET available_copies = available_copies + 1
		WHERE id = $1 AND available_copies < total_copies
	`, bookID); err != nil {
		return 0, false, apperrors.Internal(err)
	}
	return bookID, true, nil
}

// Search filters books by free-text query (Postgres full-text search on
// title/description, OR a partial title/author-name match via pg_trgm-backed
// ILIKE), plus exact-match genre/format/author/year filters. The author
// join is always present — needed both for the free-text author-name match
// and the author_id filter — but GROUP BY keeps one row per book.
func (r *bookRepository) Search(ctx context.Context, params BookSearchParams, limit, offset int) ([]*Book, int, error) {
	var books []*Book
	var total int

	conditions := []string{}
	args := []any{}

	if params.Query != "" {
		conditions = append(conditions,
			"(b.search_vector @@ websearch_to_tsquery('simple', ?) OR b.title ILIKE ? OR a.first_name ILIKE ? OR a.last_name ILIKE ?)")
		like := "%" + escapeLike(params.Query) + "%"
		args = append(args, params.Query, like, like, like)
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
	if err := sqlx.GetContext(ctx, r.q, &total, r.q.Rebind(countSQL), args...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	searchSQL := fmt.Sprintf(
		"SELECT %s %s %s GROUP BY b.id ORDER BY b.created_at DESC LIMIT ? OFFSET ?",
		bookColumns, fromClause, whereClause,
	)
	args = append(args, limit, offset)
	if err := sqlx.SelectContext(ctx, r.q, &books, r.q.Rebind(searchSQL), args...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	return books, total, nil
}

// escapeLike stops user input's % and _ acting as ILIKE wildcards.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
