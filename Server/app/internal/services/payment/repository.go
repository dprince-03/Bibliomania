// Package payment is payment-service: one-time book purchases (Postgres +
// Kafka) through Stripe Checkout or Paystack — picked per purchase by
// currency (provider.go). Starts hosted checkouts synchronously, learns the
// outcome from each provider's signed webhooks, and publishes payment.*
// events via the outbox.
package payment

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/events"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

//go:embed migrations/*.sql
var Migrations embed.FS

const (
	StatusPending  = "pending"
	StatusPaid     = "paid"
	StatusFailed   = "failed"
	StatusRefunded = "refunded"
)

type Purchase struct {
	ID                uint64    `db:"id"`
	PublicID          string    `db:"public_id"`
	UserID            uint64    `db:"user_id"`
	UserEmail         string    `db:"user_email"`
	BookID            uint64    `db:"book_id"`
	BookTitle         string    `db:"book_title"`
	AmountCents       int64     `db:"amount_cents"`
	Currency          string    `db:"currency"`
	Status            string    `db:"status"`
	Provider          string    `db:"provider"`
	ProviderReference *string   `db:"provider_reference"`
	ProviderPaymentID *string   `db:"provider_payment_id"`
	CheckoutURL       *string   `db:"checkout_url"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

type Repository interface {
	// Create inserts a pending purchase. ErrPendingExists if the user
	// already has an open checkout for this book (unique partial index).
	Create(ctx context.Context, p *Purchase) error
	GetPending(ctx context.Context, userID, bookID uint64) (*Purchase, error)
	SetReference(ctx context.Context, id uint64, reference, checkoutURL string) error
	MarkFailed(ctx context.Context, id uint64) error
	HasPaid(ctx context.Context, userID, bookID uint64) (bool, error)
	ListByUser(ctx context.Context, userID uint64, limit, offset int) ([]*Purchase, int, error)
	// SettleFromWebhook moves the pending purchase ev refers to into
	// ev.Status, deduping on (provider, ev.ID) and writing the outbox event,
	// all in one transaction. A "paid" outcome whose amount or currency
	// doesn't match the purchase is refused (ErrAmountMismatch) and the
	// purchase stays pending for manual review. Returns (nil, nil) for a
	// duplicate delivery, an unknown reference, or an already-settled
	// purchase.
	SettleFromWebhook(ctx context.Context, provider string, ev *WebhookEvent, event func(*Purchase) (events.Event, error)) (*Purchase, error)
}

// ErrAmountMismatch: the provider reports a payment that doesn't match
// what the purchase costs. Never grant the book on it.
var ErrAmountMismatch = errors.New("paid amount/currency does not match the purchase")

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

const purchaseColumns = `id, public_id, user_id, user_email, book_id, book_title, amount_cents, currency, status,
	provider, provider_reference, provider_payment_id, checkout_url, created_at, updated_at`

// ErrPendingExists: an open checkout for this user+book already exists.
var ErrPendingExists = errors.New("pending purchase exists")

func (r *repository) Create(ctx context.Context, p *Purchase) error {
	row := r.db.QueryRowxContext(ctx, `
		INSERT INTO book_purchases (public_id, user_id, user_email, book_id, book_title, amount_cents, currency, provider)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, status, created_at, updated_at`,
		p.PublicID, p.UserID, p.UserEmail, p.BookID, p.BookTitle, p.AmountCents, p.Currency, p.Provider)
	if err := row.Scan(&p.ID, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_purchases_pending_per_user_book" {
			return ErrPendingExists
		}
		return apperrors.Internal(err)
	}
	return nil
}

func (r *repository) GetPending(ctx context.Context, userID, bookID uint64) (*Purchase, error) {
	p := &Purchase{}
	err := r.db.GetContext(ctx, p, `SELECT `+purchaseColumns+` FROM book_purchases
		WHERE user_id = $1 AND book_id = $2 AND status = 'pending'`, userID, bookID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NotFound("pending purchase")
	}
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	return p, nil
}

func (r *repository) SetReference(ctx context.Context, id uint64, reference, checkoutURL string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE book_purchases SET provider_reference = $1, checkout_url = $2 WHERE id = $3`,
		reference, checkoutURL, id); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *repository) MarkFailed(ctx context.Context, id uint64) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE book_purchases SET status = 'failed' WHERE id = $1 AND status = 'pending'`, id); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *repository) HasPaid(ctx context.Context, userID, bookID uint64) (bool, error) {
	var n int
	if err := r.db.GetContext(ctx, &n, `
		SELECT COUNT(*) FROM book_purchases WHERE user_id = $1 AND book_id = $2 AND status = 'paid'`,
		userID, bookID); err != nil {
		return false, apperrors.Internal(err)
	}
	return n > 0, nil
}

func (r *repository) ListByUser(ctx context.Context, userID uint64, limit, offset int) ([]*Purchase, int, error) {
	purchases := []*Purchase{}
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM book_purchases WHERE user_id = $1`, userID); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	if err := r.db.SelectContext(ctx, &purchases, `
		SELECT `+purchaseColumns+` FROM book_purchases
		WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return purchases, total, nil
}

func (r *repository) SettleFromWebhook(ctx context.Context, provider string, ev *WebhookEvent, event func(*Purchase) (events.Event, error)) (*Purchase, error) {
	var settled *Purchase
	err := events.InTx(ctx, r.db, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO webhook_events (provider, id, type) VALUES ($1, $2, $3)
			ON CONFLICT (provider, id) DO NOTHING`, provider, ev.ID, ev.Type)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil // duplicate delivery
		}

		p := &Purchase{}
		err = tx.GetContext(ctx, p, `
			SELECT `+purchaseColumns+` FROM book_purchases
			WHERE provider = $1 AND provider_reference = $2 AND status = 'pending'
			FOR UPDATE`, provider, ev.Reference)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // unknown reference, or already settled
		}
		if err != nil {
			return err
		}

		if ev.Status == StatusPaid && (ev.AmountMinor != p.AmountCents || !strings.EqualFold(ev.Currency, p.Currency)) {
			return fmt.Errorf("%w: purchase %d costs %d %s, provider reports %d %s",
				ErrAmountMismatch, p.ID, p.AmountCents, p.Currency, ev.AmountMinor, ev.Currency)
		}

		if err := tx.GetContext(ctx, p, `
			UPDATE book_purchases
			SET status = $1, provider_payment_id = NULLIF($2, '')
			WHERE id = $3
			RETURNING `+purchaseColumns, ev.Status, ev.PaymentID, p.ID); err != nil {
			return err
		}

		e, err := event(p)
		if err != nil {
			return err
		}
		if err := events.AddToOutbox(ctx, tx, e); err != nil {
			return err
		}
		settled = p
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrAmountMismatch) {
			return nil, err
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// A second paid purchase of the same book (two checkouts
			// completed): the unique index refuses it. Needs a refund —
			// logged by the caller; automatic refunds are Step 27 scope.
			return nil, apperrors.Conflict("book already purchased")
		}
		return nil, apperrors.Internal(err)
	}
	return settled, nil
}
