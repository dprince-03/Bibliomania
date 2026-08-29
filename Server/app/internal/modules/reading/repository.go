package reading

import (
	"context"
	"database/sql"
	"errors"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"

	"github.com/jmoiron/sqlx"
)

type SessionRepository interface {
	GetByUserAndBook(ctx context.Context, userID, bookID uint64) (*ReadingSession, error)
	// GetAllByUserID powers GET /users/me/history (see internal/modules/user) —
	// declared here, not there, since ReadingSession itself stays owned by
	// this package; user.Service just reads through it.
	GetAllByUserID(ctx context.Context, userID uint64, limit, offset int) ([]*ReadingSession, int, error)
	Upsert(ctx context.Context, session *ReadingSession) error
}

type BookmarkRepository interface {
	GetByUserAndBook(ctx context.Context, userID, bookID uint64) ([]*Bookmark, error)
	GetByID(ctx context.Context, id uint64) (*Bookmark, error)
	Create(ctx context.Context, bookmark *Bookmark) (uint64, error)
	Delete(ctx context.Context, id uint64) error
}

// ── Reading Sessions ──────────────────────────────────────

type sessionRepository struct {
	db *sqlx.DB
}

func NewSessionRepository(db *sqlx.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) GetByUserAndBook(ctx context.Context, userID, bookID uint64) (*ReadingSession, error) {
	session := &ReadingSession{}
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

func (r *sessionRepository) GetAllByUserID(ctx context.Context, userID uint64, limit, offset int) ([]*ReadingSession, int, error) {
	var sessions []*ReadingSession
	var total int

	countQuery := `SELECT COUNT(*) FROM reading_sessions WHERE user_id = ?`
	if err := r.db.GetContext(ctx, &total, countQuery, userID); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `
		SELECT * FROM reading_sessions
		WHERE user_id = ?
		ORDER BY last_read_at DESC
		LIMIT ? OFFSET ?
	`
	if err := r.db.SelectContext(ctx, &sessions, query, userID, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return sessions, total, nil
}

// Upsert creates or updates a reading session.
// On conflict (same user + book), updates progress only if client data is newer.
func (r *sessionRepository) Upsert(ctx context.Context, session *ReadingSession) error {
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

// ── Bookmarks ─────────────────────────────────────────────

type bookmarkRepository struct {
	db *sqlx.DB
}

func NewBookmarkRepository(db *sqlx.DB) BookmarkRepository {
	return &bookmarkRepository{db: db}
}

func (r *bookmarkRepository) GetByUserAndBook(ctx context.Context, userID, bookID uint64) ([]*Bookmark, error) {
	var bookmarks []*Bookmark
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

func (r *bookmarkRepository) GetByID(ctx context.Context, id uint64) (*Bookmark, error) {
	bookmark := &Bookmark{}
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

func (r *bookmarkRepository) Create(ctx context.Context, bookmark *Bookmark) (uint64, error) {
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
