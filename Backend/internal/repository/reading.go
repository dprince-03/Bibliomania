package repository

import (
	"context"
	"database/sql"
	"errors"

	apperrors "bibliotheca/internal/errors"
	"bibliotheca/internal/models"
	"github.com/jmoiron/sqlx"
)

type readingSessionRepository struct {
	db *sqlx.DB
}

func NewReadingSessionRepository(db *sqlx.DB) ReadingSessionRepository {
	return &readingSessionRepository{db: db}
}

func (r *readingSessionRepository) GetByUserAndBook(ctx context.Context, userID, bookID uint64) (*models.ReadingSession, error) {
	session := &models.ReadingSession{}
	query := `SELECT * FROM reading_sessions WHERE user_id = ? AND book_id = ?`

	err := r.db.GetContext(ctx, session, query, userID, bookID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("reading session")
		}
		return nil, apperrors.Internal(err)
	}
	return session, nil
}

// Upsert creates or updates a reading session.
// On conflict (same user + book), updates progress only if client data is newer.
func (r *readingSessionRepository) Upsert(ctx context.Context, session *models.ReadingSession) error {
	query := `
		INSERT INTO reading_sessions
		    (user_id, book_id, current_page, total_pages, progress_pct,
		     current_chapter, is_completed, completed_at, client_updated_at)
		VALUES
		    (:user_id, :book_id, :current_page, :total_pages, :progress_pct,
		     :current_chapter, :is_completed, :completed_at, :client_updated_at)
		ON DUPLICATE KEY UPDATE
		    current_page     = IF(client_updated_at >= VALUES(client_updated_at), current_page,     VALUES(current_page)),
		    total_pages      = IF(client_updated_at >= VALUES(client_updated_at), total_pages,      VALUES(total_pages)),
		    progress_pct     = IF(client_updated_at >= VALUES(client_updated_at), progress_pct,     VALUES(progress_pct)),
		    current_chapter  = IF(client_updated_at >= VALUES(client_updated_at), current_chapter,  VALUES(current_chapter)),
		    is_completed     = IF(client_updated_at >= VALUES(client_updated_at), is_completed,     VALUES(is_completed)),
		    completed_at     = IF(client_updated_at >= VALUES(client_updated_at), completed_at,     VALUES(completed_at)),
		    client_updated_at = IF(client_updated_at >= VALUES(client_updated_at), client_updated_at, VALUES(client_updated_at))
	`
	_, err := r.db.NamedExecContext(ctx, query, session)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

// ── User Library ─────────────────────────────────────────

type userLibraryRepository struct {
	db *sqlx.DB
}

func NewUserLibraryRepository(db *sqlx.DB) UserLibraryRepository {
	return &userLibraryRepository{db: db}
}

func (r *userLibraryRepository) GetByUserID(ctx context.Context, userID uint64, status string, limit, offset int) ([]*models.UserLibrary, int, error) {
	var entries []*models.UserLibrary
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

func (r *userLibraryRepository) GetEntry(ctx context.Context, userID, bookID uint64) (*models.UserLibrary, error) {
	entry := &models.UserLibrary{}
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

func (r *userLibraryRepository) Upsert(ctx context.Context, entry *models.UserLibrary) error {
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

// ── Bookmarks ─────────────────────────────────────────────

type bookmarkRepository struct {
	db *sqlx.DB
}

func NewBookmarkRepository(db *sqlx.DB) BookmarkRepository {
	return &bookmarkRepository{db: db}
}

func (r *bookmarkRepository) GetByUserAndBook(ctx context.Context, userID, bookID uint64) ([]*models.Bookmark, error) {
	var bookmarks []*models.Bookmark
	query := `
		SELECT * FROM bookmarks
		WHERE user_id = ? AND book_id = ?
		ORDER BY page ASC
	`
	if err := r.db.SelectContext(ctx, &bookmarks, query, userID, bookID); err != nil {
		return nil, apperrors.Internal(err)
	}
	return bookmarks, nil
}

func (r *bookmarkRepository) GetByID(ctx context.Context, id uint64) (*models.Bookmark, error) {
	bookmark := &models.Bookmark{}
	query := `SELECT * FROM bookmarks WHERE id = ?`

	err := r.db.GetContext(ctx, bookmark, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("bookmark")
		}
		return nil, apperrors.Internal(err)
	}
	return bookmark, nil
}

func (r *bookmarkRepository) Create(ctx context.Context, bookmark *models.Bookmark) (uint64, error) {
	query := `
		INSERT INTO bookmarks (user_id, book_id, page, note, highlight, color)
		VALUES (:user_id, :book_id, :page, :note, :highlight, :color)
	`
	result, err := r.db.NamedExecContext(ctx, query, bookmark)
	if err != nil {
		return 0, apperrors.Internal(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, apperrors.Internal(err)
	}
	return uint64(id), nil
}

func (r *bookmarkRepository) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM bookmarks WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}
