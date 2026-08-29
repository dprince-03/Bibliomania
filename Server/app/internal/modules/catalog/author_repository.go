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
}

type authorRepository struct {
	db *sqlx.DB
}

func NewAuthorRepository(db *sqlx.DB) AuthorRepository {
	return &authorRepository{
		db: db,
	}
}

func (r *authorRepository) GetByID(ctx context.Context, id uint64) (*Author, error) {
	author := &Author{}
	query := `SELECT * FROM authors WHERE id = ?`

	err := r.db.GetContext(ctx, author, query, id)
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

	countQuery := `SELECT COUNT(*) FROM authors`
	if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT * FROM authors ORDER BY first_name ASC LIMIT ? OFFSET ?`
	if err := r.db.SelectContext(ctx, &authors, query, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return authors, total, nil
}

func (r *authorRepository) Create(ctx context.Context, author *Author) (uint64, error) {
	query := `
				INSERT INTO authors (first_name, last_name, middle_name, image, date_of_birth, biography, phone, email)
				VALUES (:first_name, :last_name, :middle_name, :image, :date_of_birth, :biography, :phone, :email)
			`

	result, err := r.db.NamedExecContext(ctx, query, author)
	if err != nil {
		return 0, apperrors.Internal(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, apperrors.Internal(err)
	}
	return uint64(id), nil
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

	_, err := r.db.NamedExecContext(ctx, query, author)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *authorRepository) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM authors WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}
