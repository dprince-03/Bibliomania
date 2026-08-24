package borrow

import (
	"context"
	"database/sql"
	"errors"
	"time"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	GetByID(ctx context.Context, id uint64) (*BorrowRecord, error)
	GetAllByUserID(ctx context.Context, userID uint64, limit, offset int) ([]*BorrowRecord, int, error)
	GetAll(ctx context.Context, limit, offset int) ([]*BorrowRecord, int, error)
	Create(ctx context.Context, record *BorrowRecord) (uint64, error)
	MarkReturned(ctx context.Context, id uint64) error
	UpdateOverdue(ctx context.Context) error
	HasActiveBorrow(ctx context.Context, userID, bookID uint64) (bool, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetByID(ctx context.Context, id uint64) (*BorrowRecord, error) {
	record := &BorrowRecord{}
	query := `SELECT * FROM borrows_records WHERE id = ?`

	err := r.db.GetContext(ctx, record, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("borrow record")
		}
		return nil, apperrors.Internal(err)
	}
	return record, nil
}

func (r *repository) GetAllByUserID(ctx context.Context, userID uint64, limit, offset int) ([]*BorrowRecord, int, error) {
	var records []*BorrowRecord
	var total int

	countQuery := `SELECT COUNT(*) FROM borrows_records WHERE user_id = ?`
	if err := r.db.GetContext(ctx, &total, countQuery, userID); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `
		SELECT * FROM borrows_records
		WHERE user_id = ?
		ORDER BY borrowed_at DESC
		LIMIT ? OFFSET ?
	`
	if err := r.db.SelectContext(ctx, &records, query, userID, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return records, total, nil
}

func (r *repository) GetAll(ctx context.Context, limit, offset int) ([]*BorrowRecord, int, error) {
	var records []*BorrowRecord
	var total int

	countQuery := `SELECT COUNT(*) FROM borrows_records`
	if err := r.db.GetContext(ctx, &total, countQuery); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT * FROM borrows_records ORDER BY borrowed_at DESC LIMIT ? OFFSET ?`
	if err := r.db.SelectContext(ctx, &records, query, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return records, total, nil
}

func (r *repository) Create(ctx context.Context, record *BorrowRecord) (uint64, error) {
	query := `
		INSERT INTO borrows_records (user_id, book_id, due_at)
		VALUES (:user_id, :book_id, :due_at)
	`
	result, err := r.db.NamedExecContext(ctx, query, record)
	if err != nil {
		return 0, apperrors.Internal(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, apperrors.Internal(err)
	}
	return uint64(id), nil
}

func (r *repository) MarkReturned(ctx context.Context, id uint64) error {
	query := `
		UPDATE borrows_records
		SET status      = 'returned',
		    returned_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

// UpdateOverdue marks all active borrows past their due date as overdue.
// Run this on a schedule (cron) or on every borrow fetch.
func (r *repository) UpdateOverdue(ctx context.Context) error {
	query := `
		UPDATE borrows_records
		SET status = 'overdue'
		WHERE status = 'active' AND due_at < NOW()
	`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *repository) HasActiveBorrow(ctx context.Context, userID, bookID uint64) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM borrows_records
		WHERE user_id = ? AND book_id = ? AND status = 'active'
	`
	if err := r.db.GetContext(ctx, &count, query, userID, bookID); err != nil {
		return false, apperrors.Internal(err)
	}
	return count > 0, nil
}
