package user

import (
	"context"
	"database/sql"
	"errors"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/events"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	GetByID(ctx context.Context, id uint64) (*User, error)
	// GetAll is for the admin listing (GET /users) — deliberately does NOT
	// filter by is_active like GetByID does: an admin needs to see
	// deactivated accounts too, precisely so they can reactivate them.
	GetAll(ctx context.Context, limit, offset int) ([]*User, int, error)
	// UpdateStatus changes is_active and records user.status_changed in the
	// outbox, in one transaction. NotFound if the user doesn't exist.
	UpdateStatus(ctx context.Context, id uint64, isActive bool, event events.Event) error

	// Event-driven writes — each dedupes on the event ID (processed_events)
	// so a redelivered message is a no-op.
	ApplyUserRegistered(ctx context.Context, eventID string, u *User) error
	ApplyBookCompleted(ctx context.Context, eventID string, userID uint64, pages uint32) (applied bool, err error)
	ApplyProgress(ctx context.Context, userID, bookID uint64) error
}

type ProfileRepository interface {
	GetByUserID(ctx context.Context, userID uint64) (*UserProfile, error)
	Update(ctx context.Context, profile *UserProfile) error
}

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

const userColumns = `id, first_name, last_name, email, role, is_active, created_at, updated_at`

func (r *repository) GetByID(ctx context.Context, id uint64) (*User, error) {
	u := &User{}
	query := `SELECT ` + userColumns + ` FROM users WHERE id = $1 AND is_active = TRUE`

	if err := r.db.GetContext(ctx, u, query, id); err != nil {
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

	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM users`); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT ` + userColumns + ` FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	if err := r.db.SelectContext(ctx, &users, query, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return users, total, nil
}

func (r *repository) UpdateStatus(ctx context.Context, id uint64, isActive bool, event events.Event) error {
	err := events.InTx(ctx, r.db, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET is_active = $1 WHERE id = $2`, isActive, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apperrors.NotFound("user")
		}
		return events.AddToOutbox(ctx, tx, event)
	})
	return wrap(err)
}

func (r *repository) ApplyUserRegistered(ctx context.Context, eventID string, u *User) error {
	err := events.InTx(ctx, r.db, func(tx *sqlx.Tx) error {
		first, err := events.MarkProcessed(ctx, tx, eventID)
		if err != nil || !first {
			return err
		}
		if _, err := tx.NamedExecContext(ctx, `
			INSERT INTO users (id, first_name, last_name, email, role, is_active)
			VALUES (:id, :first_name, :last_name, :email, :role, :is_active)
			ON CONFLICT (id) DO NOTHING`, u); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO users_profile (user_id) VALUES ($1)
			ON CONFLICT (user_id) DO NOTHING`, u.ID)
		return err
	})
	return wrap(err)
}

func (r *repository) ApplyBookCompleted(ctx context.Context, eventID string, userID uint64, pages uint32) (bool, error) {
	applied := false
	err := events.InTx(ctx, r.db, func(tx *sqlx.Tx) error {
		first, err := events.MarkProcessed(ctx, tx, eventID)
		if err != nil || !first {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE users_profile
			SET total_books_read = total_books_read + 1,
			    total_pages_read = total_pages_read + $1
			WHERE user_id = $2`, pages, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			// The profile hasn't been replicated from auth yet. Fail (and
			// roll back the processed marker) so the broker redelivers.
			return errors.New("profile not found yet")
		}
		applied = true
		return nil
	})
	return applied, wrap(err)
}

func (r *repository) ApplyProgress(ctx context.Context, userID, bookID uint64) error {
	// Naturally idempotent — no dedupe needed.
	_, err := r.db.ExecContext(ctx, `
		UPDATE users_profile
		SET last_read_book_id = $1, last_online_at = now()
		WHERE user_id = $2`, bookID, userID)
	return wrap(err)
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperrors.Internal(err)
}

// ── Profile ───────────────────────────────────────────────

type profileRepository struct {
	db *sqlx.DB
}

func NewProfileRepository(db *sqlx.DB) ProfileRepository {
	return &profileRepository{db: db}
}

const profileColumns = `id, user_id, phone_number, bio, profile_picture, last_online_at, last_read_book_id,
	total_books_read, total_pages_read, created_at, updated_at`

func (r *profileRepository) GetByUserID(ctx context.Context, userID uint64) (*UserProfile, error) {
	profile := &UserProfile{}
	query := `SELECT ` + profileColumns + ` FROM users_profile WHERE user_id = $1`

	if err := r.db.GetContext(ctx, profile, query, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("profile")
		}
		return nil, apperrors.Internal(err)
	}
	return profile, nil
}

func (r *profileRepository) Update(ctx context.Context, profile *UserProfile) error {
	query := `
		UPDATE users_profile
		SET phone_number = :phone_number,
			bio = :bio,
			profile_picture = :profile_picture
		WHERE user_id = :user_id
	`
	if _, err := r.db.NamedExecContext(ctx, query, profile); err != nil {
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

const libraryColumns = `id, user_id, book_id, status, added_at, updated_at`

func (r *libraryRepository) GetByUserID(ctx context.Context, userID uint64, status string, limit, offset int) ([]*UserLibrary, int, error) {
	entries := []*UserLibrary{}
	var total int

	where := `WHERE user_id = ?`
	args := []any{userID}
	if status != "" {
		where += ` AND status = ?`
		args = append(args, status)
	}

	if err := r.db.GetContext(ctx, &total, r.db.Rebind(`SELECT COUNT(*) FROM user_library `+where), args...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := r.db.Rebind(`SELECT ` + libraryColumns + ` FROM user_library ` + where + ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`)
	if err := r.db.SelectContext(ctx, &entries, query, append(args, limit, offset)...); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return entries, total, nil
}

func (r *libraryRepository) GetEntry(ctx context.Context, userID, bookID uint64) (*UserLibrary, error) {
	entry := &UserLibrary{}
	query := `SELECT ` + libraryColumns + ` FROM user_library WHERE user_id = $1 AND book_id = $2`

	if err := r.db.GetContext(ctx, entry, query, userID, bookID); err != nil {
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
		ON CONFLICT (user_id, book_id) DO UPDATE SET status = EXCLUDED.status
	`
	if _, err := r.db.NamedExecContext(ctx, query, entry); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}
