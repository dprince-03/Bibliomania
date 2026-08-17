package repository

import (
	apperrors "bibliotheca/internal/errors"
	"bibliotheca/internal/models"
	"context"
	"database/sql"
	"time"

	"errors"

	"github.com/jmoiron/sqlx"
)

type userProfileRepository struct {
	db *sqlx.DB
}

func NewUserProfileRepository(db *sqlx.DB) UserProfileRepository {
	return &userProfileRepository{
		db: db,
	}
}

func (r *userProfileRepository) GetByUserID(ctx context.Context, userID uint64) (*models.UserProfile, error) {
	profile := &models.UserProfile{}
	query := `SELECT * FROM users_profile WHERE user_id = ?`

	err := r.db.GetContext(ctx, profile, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("profile")
		}
		return nil, apperrors.Internal(err)
	}
	return profile, nil
}

func (r *userProfileRepository) CreateUserProfile(ctx context.Context, profile *models.UserProfile) error {
	query := `INSERT INTO users_profile (user_id) VALUES (:user_id)`
	_, err := r.db.NamedExecContext(ctx, query, profile)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *userProfileRepository) UpdateUserProfile(ctx context.Context, profile *models.UserProfile) error {
	query := `
				UPDATE users_profile
				SET phone_number = :phone_number,
					bio = :bio,
					profile_picture = :profile_picture
				WHERE user_id = :user_id
			`

	_, err := r.db.NamedExecContext(ctx, query, profile)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *userProfileRepository) UpdateLastOnline(ctx context.Context, userID uint64) error {
	query := `UPDATE users_profile SET last_online_at = ? WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *userProfileRepository) UpdateLastReadBook(ctx context.Context, userID uint64, bookID uint64) error {
	query := `UPDATE users_profile SET last_read_book_id = ? WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, bookID, userID)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *userProfileRepository) IncrementBooksRead(ctx context.Context, userID uint64) error {
	query := `UPDATE users_profile SET total_books_read = total_books_read + 1 WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *userProfileRepository) IncrementPagesRead(ctx context.Context, userID uint64, pages uint32) error {
	query := `UPDATE users_profile SET total_pages_read = total_pages_read + ? WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, pages, userID)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}
