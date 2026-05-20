package repository

import (
	apperrors "bibliotheca/internal/errors"
	"bibliotheca/internal/models"
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{
		db: db,
	}
}

func (r *userRepository) GetByID(ctx context.Context, id uint64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT * FROM users WHERE id = ? AND is_active = TRUE`

	err := r.db.GetContext(ctx, user, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, apperrors.Internal(err)
	}
	return user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT * FROM users WHERE email = ?`

	err := r.db.GetContext(ctx, user, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, apperrors.Internal(err)
	}
	return user, nil
}

func (r *userRepository) CreateUser(ctx context.Context, user *models.User) (uint64, error) {
	query := `
				INSERT INTO users (first_name, last_name, email, password, role)
				VALUES (:first_name, :last_name, :email, :password, :role)
			`

	result, err := r.db.NamedExecContext(ctx, query, user)
	if err != nil {
		return 0, apperrors.Internal(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, apperrors.Internal(err)
	}
	return uint64(id), nil
}

func (r *userRepository) UpdateUser(ctx context.Context, user *models.User) error {
	query := `
				UPDATE users
				SET first_name = :first_name,
					last_name = :last_name,
					email = :email
				WHERE id = :id
			`

	_, err := r.db.NamedExecContext(ctx, query, user)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *userRepository) UpdateStatus(ctx context.Context, id uint64, isActive bool) error {
	query := `UPDATE users SET is_active = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, isActive, id)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}
