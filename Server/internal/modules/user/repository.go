package user

import (
	"context"
	"database/sql"
	"errors"
	"time"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	GetByID(ctx context.Context, id uint64) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	// GetAll is for the admin listing (GET /users) — deliberately does NOT
	// filter by is_active like GetByID does: an admin needs to see
	// deactivated accounts too, precisely so they can reactivate them.
	GetAll(ctx context.Context, limit, offset int) ([]*User, int, error)
	Create(ctx context.Context, user *User) (uint64, error)
	Update(ctx context.Context, user *User) error
	UpdateStatus(ctx context.Context, id uint64, isActive bool) error
}

type ProfileRepository interface {
	GetByUserID(ctx context.Context, userID uint64) (*UserProfile, error)
	Create(ctx context.Context, profile *UserProfile) error
	Update(ctx context.Context, profile *UserProfile) error
	UpdateLastOnline(ctx context.Context, userID uint64) error
	UpdateLastReadBook(ctx context.Context, userID uint64, bookID uint64) error
	IncrementBooksRead(ctx context.Context, userID uint64) error
	IncrementPagesRead(ctx context.Context, userID uint64, pages uint32) error
}

// LibraryRepository — moved from internal/modules/reading (see UserLibrary
// in model.go for why).
type LibraryRepository interface {
	GetByUserID(ctx context.Context, userID uint64, status string, limit, offset int) ([]*UserLibrary, int, error)
	GetEntry(ctx context.Context, userID, bookID uint64) (*UserLibrary, error)
	Upsert(ctx context.Context, entry *UserLibrary) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetByID(ctx context.Context, id uint64) (*User, error) {
	u := &User{}
	query := `SELECT * FROM users WHERE id = ? AND is_active = TRUE`

	err := r.db.GetContext(ctx, u, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, apperrors.Internal(err)
	}
	return u, nil
}

func (r *repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	query := `SELECT * FROM users WHERE email = ?`

	err := r.db.GetContext(ctx, u, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, apperrors.Internal(err)
	}
	return u, nil
}

func (r *repository) GetAll(ctx context.Context, limit, offset int) ([]*User, int, error) {
	var users []*User
	var total int

	countQuery := `SELECT COUNT(*) FROM users`
	if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT * FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?`
	if err := r.db.SelectContext(ctx, &users, query, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return users, total, nil
}

func (r *repository) Create(ctx context.Context, u *User) (uint64, error) {
	query := `
				INSERT INTO users (first_name, last_name, email, password, role)
				VALUES (:first_name, :last_name, :email, :password, :role)
			`

	result, err := r.db.NamedExecContext(ctx, query, u)
	if err != nil {
		return 0, apperrors.Internal(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, apperrors.Internal(err)
	}
	return uint64(id), nil
}

func (r *repository) Update(ctx context.Context, u *User) error {
	query := `
				UPDATE users
				SET first_name = :first_name,
					last_name = :last_name,
					email = :email
				WHERE id = :id
			`

	_, err := r.db.NamedExecContext(ctx, query, u)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *repository) UpdateStatus(ctx context.Context, id uint64, isActive bool) error {
	query := `UPDATE users SET is_active = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, isActive, id)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

type profileRepository struct {
	db *sqlx.DB
}

func NewProfileRepository(db *sqlx.DB) ProfileRepository {
	return &profileRepository{db: db}
}

func (r *profileRepository) GetByUserID(ctx context.Context, userID uint64) (*UserProfile, error) {
	profile := &UserProfile{}
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

func (r *profileRepository) Create(ctx context.Context, profile *UserProfile) error {
	query := `INSERT INTO users_profile (user_id) VALUES (:user_id)`
	_, err := r.db.NamedExecContext(ctx, query, profile)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *profileRepository) Update(ctx context.Context, profile *UserProfile) error {
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

func (r *profileRepository) UpdateLastOnline(ctx context.Context, userID uint64) error {
	query := `UPDATE users_profile SET last_online_at = ? WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *profileRepository) UpdateLastReadBook(ctx context.Context, userID uint64, bookID uint64) error {
	query := `UPDATE users_profile SET last_read_book_id = ? WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, bookID, userID)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *profileRepository) IncrementBooksRead(ctx context.Context, userID uint64) error {
	query := `UPDATE users_profile SET total_books_read = total_books_read + 1 WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *profileRepository) IncrementPagesRead(ctx context.Context, userID uint64, pages uint32) error {
	query := `UPDATE users_profile SET total_pages_read = total_pages_read + ? WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, pages, userID)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

// ── User Library ─────────────────────────────────────────

type libraryRepository struct {
	db *sqlx.DB
}

func NewLibraryRepository(db *sqlx.DB) LibraryRepository {
	return &libraryRepository{db: db}
}

func (r *libraryRepository) GetByUserID(ctx context.Context, userID uint64, status string, limit, offset int) ([]*UserLibrary, int, error) {
	var entries []*UserLibrary
	var total int

	countQuery := `SELECT COUNT(*) FROM user_library WHERE user_id = ?`
	countArgs := []any{userID}

	if status != "" {
		countQuery += ` AND status = ?`
		countArgs = append(countArgs, status)
	}

	if err := r.db.GetContext(ctx, &total, countQuery, countArgs...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT * FROM user_library WHERE user_id = ?`
	args := []any{userID}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	if err := r.db.SelectContext(ctx, &entries, query, args...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return entries, total, nil
}

func (r *libraryRepository) GetEntry(ctx context.Context, userID, bookID uint64) (*UserLibrary, error) {
	entry := &UserLibrary{}
	query := `SELECT * FROM user_library WHERE user_id = ? AND book_id = ?`

	err := r.db.GetContext(ctx, entry, query, userID, bookID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("library entry")
		}
		return nil, apperrors.Internal(err)
	}
	return entry, nil
}

func (r *libraryRepository) Upsert(ctx context.Context, entry *UserLibrary) error {
	query := `
		INSERT INTO user_library (user_id, book_id, status)
		VALUES (:user_id, :book_id, :status)
		ON DUPLICATE KEY UPDATE status = VALUES(status)
	`
	_, err := r.db.NamedExecContext(ctx, query, entry)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}
