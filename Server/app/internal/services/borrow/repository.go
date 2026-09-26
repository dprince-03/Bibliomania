package borrow

import (
	"context"
	"database/sql"
	"errors"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/events"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

// EventFor builds the outbox event for a record once its ID is known.
type EventFor func(r *BorrowRecord) (events.Event, error)

type Repository interface {
	GetByID(ctx context.Context, id uint64) (*BorrowRecord, error)
	GetByPublicID(ctx context.Context, publicID string) (*BorrowRecord, error)
	GetByIdempotencyKey(ctx context.Context, userID uint64, key string) (*BorrowRecord, error)
	GetOpenByUserAndBook(ctx context.Context, userID, bookID uint64) (*BorrowRecord, error)
	GetAllByUserID(ctx context.Context, userID uint64, limit, offset int) ([]*BorrowRecord, int, error)
	GetAll(ctx context.Context, limit, offset int) ([]*BorrowRecord, int, error)
	HasOpenBorrow(ctx context.Context, userID, bookID uint64) (bool, error)

	// The writes below each commit the row change and its outbox event in
	// one transaction.
	Create(ctx context.Context, record *BorrowRecord, event EventFor) (uint64, error)
	MarkReturned(ctx context.Context, record *BorrowRecord, event EventFor) error
	// MarkOverdue flips every active borrow past its due date to overdue
	// and emits borrow.borrow_overdue for each (notification-service emails
	// the member). Returns how many it flipped.
	MarkOverdue(ctx context.Context, event EventFor) (int, error)

	// Saga compensations that couldn't run inline — see migration 000002.
	AddCompensation(ctx context.Context, reservationKey string, bookID uint64, cause error) error
	DueCompensations(ctx context.Context, limit int) ([]Compensation, error)
	CompensationFailed(ctx context.Context, reservationKey string, cause error) error
	CompensationDone(ctx context.Context, reservationKey string) error
}

type Compensation struct {
	ReservationKey string `db:"reservation_key"`
	BookID         uint64 `db:"book_id"`
	Attempts       int    `db:"attempts"`
}

// ErrOpenBorrowExists is returned by Create when the partial unique index
// uq_borrows_open_per_user_book rejects a concurrent duplicate borrow.
var ErrOpenBorrowExists = apperrors.Conflict("you already have an active borrow for this book")

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

const recordColumns = `id, public_id, user_id, user_email, book_id, book_title, reservation_key, idempotency_key,
	borrowed_at, due_at, returned_at, status, created_at, updated_at`

func (r *repository) get(ctx context.Context, query string, args ...any) (*BorrowRecord, error) {
	record := &BorrowRecord{}
	if err := r.db.GetContext(ctx, record, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("borrow record")
		}
		return nil, apperrors.Internal(err)
	}
	return record, nil
}

func (r *repository) GetByID(ctx context.Context, id uint64) (*BorrowRecord, error) {
	return r.get(ctx, `SELECT `+recordColumns+` FROM borrow_records WHERE id = $1`, id)
}

func (r *repository) GetByPublicID(ctx context.Context, publicID string) (*BorrowRecord, error) {
	return r.get(ctx, `SELECT `+recordColumns+` FROM borrow_records WHERE public_id = $1`, publicID)
}

func (r *repository) GetByIdempotencyKey(ctx context.Context, userID uint64, key string) (*BorrowRecord, error) {
	return r.get(ctx, `SELECT `+recordColumns+` FROM borrow_records WHERE user_id = $1 AND idempotency_key = $2`, userID, key)
}

func (r *repository) GetOpenByUserAndBook(ctx context.Context, userID, bookID uint64) (*BorrowRecord, error) {
	return r.get(ctx, `SELECT `+recordColumns+` FROM borrow_records
		WHERE user_id = $1 AND book_id = $2 AND status IN ('active', 'overdue')`, userID, bookID)
}

func (r *repository) GetAllByUserID(ctx context.Context, userID uint64, limit, offset int) ([]*BorrowRecord, int, error) {
	var records []*BorrowRecord
	var total int

	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM borrow_records WHERE user_id = $1`, userID); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT ` + recordColumns + ` FROM borrow_records
		WHERE user_id = $1 ORDER BY borrowed_at DESC LIMIT $2 OFFSET $3`
	if err := r.db.SelectContext(ctx, &records, query, userID, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return records, total, nil
}

func (r *repository) GetAll(ctx context.Context, limit, offset int) ([]*BorrowRecord, int, error) {
	var records []*BorrowRecord
	var total int

	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM borrow_records`); err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	query := `SELECT ` + recordColumns + ` FROM borrow_records ORDER BY borrowed_at DESC LIMIT $1 OFFSET $2`
	if err := r.db.SelectContext(ctx, &records, query, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return records, total, nil
}

func (r *repository) HasOpenBorrow(ctx context.Context, userID, bookID uint64) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM borrow_records
		WHERE user_id = $1 AND book_id = $2 AND status IN ('active', 'overdue')`
	if err := r.db.GetContext(ctx, &count, query, userID, bookID); err != nil {
		return false, apperrors.Internal(err)
	}
	return count > 0, nil
}

func (r *repository) Create(ctx context.Context, record *BorrowRecord, event EventFor) (uint64, error) {
	err := events.InTx(ctx, r.db, func(tx *sqlx.Tx) error {
		row := tx.QueryRowxContext(ctx, `
			INSERT INTO borrow_records
			    (public_id, user_id, user_email, book_id, book_title, reservation_key, idempotency_key, due_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id, borrowed_at, status`,
			record.PublicID, record.UserID, record.UserEmail, record.BookID, record.BookTitle,
			record.ReservationKey, record.IdempotencyKey, record.DueAt)
		if err := row.Scan(&record.ID, &record.BorrowedAt, &record.Status); err != nil {
			return err
		}
		return addEvent(ctx, tx, record, event)
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, ErrOpenBorrowExists
		}
		return 0, apperrors.Internal(err)
	}
	return record.ID, nil
}

func (r *repository) MarkReturned(ctx context.Context, record *BorrowRecord, event EventFor) error {
	err := events.InTx(ctx, r.db, func(tx *sqlx.Tx) error {
		row := tx.QueryRowxContext(ctx, `
			UPDATE borrow_records
			SET status = 'returned', returned_at = now()
			WHERE id = $1 AND status <> 'returned'
			RETURNING returned_at, status`, record.ID)
		if err := row.Scan(&record.ReturnedAt, &record.Status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperrors.Conflict("this borrow has already been returned")
			}
			return err
		}
		return addEvent(ctx, tx, record, event)
	})
	var appErr *apperrors.AppError
	if err != nil && !errors.As(err, &appErr) {
		return apperrors.Internal(err)
	}
	return err
}

func (r *repository) MarkOverdue(ctx context.Context, event EventFor) (int, error) {
	var flipped int
	err := events.InTx(ctx, r.db, func(tx *sqlx.Tx) error {
		var records []*BorrowRecord
		if err := tx.SelectContext(ctx, &records, `
			UPDATE borrow_records
			SET status = 'overdue'
			WHERE status = 'active' AND due_at < now()
			RETURNING `+recordColumns); err != nil {
			return err
		}
		for _, rec := range records {
			if err := addEvent(ctx, tx, rec, event); err != nil {
				return err
			}
		}
		flipped = len(records)
		return nil
	})
	if err != nil {
		return 0, apperrors.Internal(err)
	}
	return flipped, nil
}

func addEvent(ctx context.Context, tx *sqlx.Tx, record *BorrowRecord, event EventFor) error {
	e, err := event(record)
	if err != nil {
		return err
	}
	return events.AddToOutbox(ctx, tx, e)
}

func (r *repository) AddCompensation(ctx context.Context, reservationKey string, bookID uint64, cause error) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO saga_compensations (reservation_key, book_id, last_error)
		VALUES ($1, $2, $3)
		ON CONFLICT (reservation_key) DO NOTHING`, reservationKey, bookID, errString(cause))
	return err
}

func (r *repository) DueCompensations(ctx context.Context, limit int) ([]Compensation, error) {
	var out []Compensation
	err := r.db.SelectContext(ctx, &out, `
		SELECT reservation_key, book_id, attempts FROM saga_compensations
		WHERE next_attempt_at <= now()
		ORDER BY next_attempt_at
		LIMIT $1`, limit)
	return out, err
}

func (r *repository) CompensationFailed(ctx context.Context, reservationKey string, cause error) error {
	// Exponential backoff, capped at 10 minutes.
	_, err := r.db.ExecContext(ctx, `
		UPDATE saga_compensations
		SET attempts = attempts + 1,
		    last_error = $2,
		    next_attempt_at = now() + LEAST(interval '10 minutes', interval '5 seconds' * power(2, attempts))
		WHERE reservation_key = $1`, reservationKey, errString(cause))
	return err
}

func (r *repository) CompensationDone(ctx context.Context, reservationKey string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM saga_compensations WHERE reservation_key = $1`, reservationKey)
	return err
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// dueAt is split out so the loan length rule lives in one place.
func dueAt(from time.Time, loanDays int) time.Time {
	return from.AddDate(0, 0, loanDays)
}
