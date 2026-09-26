package catalog

import (
	"context"
	"database/sql"
	"errors"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"

	"github.com/jmoiron/sqlx"
)

type AuthorRepository interface {
	GetByID(ctx context.Context, id uint64) (*Author, error)
	GetAll(ctx context.Context, limit, offset int) ([]*Author, int, error)
	Create(ctx context.Context, author *Author) (uint64, error)
	Update(ctx context.Context, author *Author) error
	Delete(ctx context.Context, id uint64) error
	// WithTx returns a copy of the repository bound to tx, so its writes
	// commit together with an outbox event.
	WithTx(tx *sqlx.Tx) AuthorRepository
}

// authorRepository runs against either the pool or a transaction —
// sqlx.ExtContext covers both.
type authorRepository struct {
	q sqlx.ExtContext
}

func NewAuthorRepository(db *sqlx.DB) AuthorRepository {
	return &authorRepository{q: db}
}

func (r *authorRepository) WithTx(tx *sqlx.Tx) AuthorRepository {
	return &authorRepository{q: tx}
}

const authorColumns = `id, first_name, last_name, middle_name, image, date_of_birth, biography, phone, email, created_at, updated_at`

func (r *authorRepository) GetByID(ctx context.Context, id uint64) (*Author, error) {
	author := &Author{}
	query := `SELECT ` + authorColumns + ` FROM authors WHERE id = $1`

	err := sqlx.GetContext(ctx, r.q, author, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("author")
		}
		return nil, apperrors.Internal(err)
	}
	return author, nil
}

func (r *authorRepository) GetAll(ctx context.Context, limit, offset int) ([]*Author, int, error) {
	var authors []*Author
	var total int

	if err := sqlx.GetContext(ctx, r.q, &total, `SELECT COUNT(*) FROM authors`); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT ` + authorColumns + ` FROM authors ORDER BY first_name ASC LIMIT $1 OFFSET $2`
	if err := sqlx.SelectContext(ctx, r.q, &authors, query, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return authors, total, nil
}

func (r *authorRepository) Create(ctx context.Context, author *Author) (uint64, error) {
	query := `
		INSERT INTO authors (first_name, last_name, middle_name, image, date_of_birth, biography, phone, email)
		VALUES (:first_name, :last_name, :middle_name, :image, :date_of_birth, :biography, :phone, :email)
		RETURNING id
	`
	return insertReturningID(ctx, r.q, query, author)
}

func (r *authorRepository) Update(ctx context.Context, author *Author) error {
	query := `
		UPDATE authors
		SET first_name = :first_name,
			last_name = :last_name,
			middle_name = :middle_name,
			image = :image,
			date_of_birth = :date_of_birth,
			biography = :biography,
			phone = :phone,
			email = :email
		WHERE id = :id
	`
	if _, err := sqlx.NamedExecContext(ctx, r.q, query, author); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *authorRepository) Delete(ctx context.Context, id uint64) error {
	if _, err := r.q.ExecContext(ctx, `DELETE FROM authors WHERE id = $1`, id); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

// insertReturningID runs a named INSERT ... RETURNING id — Postgres has no
// LastInsertId.
func insertReturningID(ctx context.Context, q sqlx.ExtContext, query string, arg any) (uint64, error) {
	bound, args, err := sqlx.Named(query, arg)
	if err != nil {
		return 0, apperrors.Internal(err)
	}
	var id uint64
	if err := sqlx.GetContext(ctx, q, &id, q.Rebind(bound), args...); err != nil {
		return 0, apperrors.Internal(err)
	}
	return id, nil
}
