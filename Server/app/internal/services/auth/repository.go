package auth

import (
	"context"
	"database/sql"
	"errors"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"

	"github.com/jmoiron/sqlx"
)

type AccountRepository interface {
	// GetByID filters to active accounts, like the old user.Repository did.
	GetByID(ctx context.Context, id uint64) (*Account, error)
	GetByEmail(ctx context.Context, email string) (*Account, error)
	// Create runs inside tx so the account row and its
	// auth.user_registered outbox event commit together.
	Create(ctx context.Context, tx *sqlx.Tx, a *Account) (uint64, error)
	UpdateStatus(ctx context.Context, tx *sqlx.Tx, id uint64, isActive bool) error
	// ListAfter pages every account (active or not) by ascending id — for
	// user-service's reconciler.
	ListAfter(ctx context.Context, afterID uint64, limit int) ([]*Account, error)
}

type TokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	GetByToken(ctx context.Context, token string) (*RefreshToken, error)
	// Consume atomically revokes an unrevoked token and returns it — the
	// one-time-use guarantee of refresh rotation. Exactly one of any number
	// of concurrent callers gets the token; the rest get ErrTokenReused
	// (the token exists but was already used) or Unauthorized (unknown).
	Consume(ctx context.Context, token string) (*RefreshToken, error)
	Revoke(ctx context.Context, token string) error
	// RevokeAllByUserID runs in tx when given one, else directly.
	RevokeAllByUserID(ctx context.Context, tx *sqlx.Tx, userID uint64) error
	DeleteExpired(ctx context.Context) error
}

type accountRepository struct {
	db *sqlx.DB
}

func NewAccountRepository(db *sqlx.DB) AccountRepository {
	return &accountRepository{db: db}
}

func (r *accountRepository) GetByID(ctx context.Context, id uint64) (*Account, error) {
	a := &Account{}
	query := `SELECT * FROM accounts WHERE id = ? AND is_active = TRUE`

	err := r.db.GetContext(ctx, a, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, apperrors.Internal(err)
	}
	return a, nil
}

func (r *accountRepository) GetByEmail(ctx context.Context, email string) (*Account, error) {
	a := &Account{}
	query := `SELECT * FROM accounts WHERE email = ?`

	err := r.db.GetContext(ctx, a, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, apperrors.Internal(err)
	}
	return a, nil
}

func (r *accountRepository) Create(ctx context.Context, tx *sqlx.Tx, a *Account) (uint64, error) {
	query := `
		INSERT INTO accounts (first_name, last_name, email, password, role, is_active)
		VALUES (:first_name, :last_name, :email, :password, :role, :is_active)
	`
	result, err := tx.NamedExecContext(ctx, query, a)
	if err != nil {
		return 0, apperrors.Internal(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, apperrors.Internal(err)
	}
	return uint64(id), nil
}

func (r *accountRepository) UpdateStatus(ctx context.Context, tx *sqlx.Tx, id uint64, isActive bool) error {
	query := `UPDATE accounts SET is_active = ? WHERE id = ?`
	if _, err := tx.ExecContext(ctx, query, isActive, id); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *accountRepository) ListAfter(ctx context.Context, afterID uint64, limit int) ([]*Account, error) {
	accounts := []*Account{}
	if err := r.db.SelectContext(ctx, &accounts,
		`SELECT * FROM accounts WHERE id > ? ORDER BY id LIMIT ?`, afterID, limit); err != nil {
		return nil, apperrors.Internal(err)
	}
	return accounts, nil
}

type tokenRepository struct {
	db *sqlx.DB
}

func NewTokenRepository(db *sqlx.DB) TokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) Create(ctx context.Context, token *RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, refresh_token, expires_at)
		VALUES (:user_id, :refresh_token, :expires_at)
	`
	_, err := r.db.NamedExecContext(ctx, query, token)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *tokenRepository) GetByToken(ctx context.Context, token string) (*RefreshToken, error) {
	rt := &RefreshToken{}
	query := `SELECT * FROM refresh_tokens WHERE refresh_token = ? AND revoked = FALSE`

	err := r.db.GetContext(ctx, rt, query, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.Unauthorized("invalid or expired refresh token")
		}
		return nil, apperrors.Internal(err)
	}
	return rt, nil
}

// ErrTokenReused: a refresh token was presented after it had already been
// rotated — the signature of a stolen token. See Service.RefreshToken.
var ErrTokenReused = errors.New("refresh token reused")

func (r *tokenRepository) Consume(ctx context.Context, token string) (*RefreshToken, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE refresh_token = ? AND revoked = FALSE`, token)
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	rt := &RefreshToken{}
	if err := r.db.GetContext(ctx, rt, `SELECT * FROM refresh_tokens WHERE refresh_token = ?`, token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.Unauthorized("invalid or expired refresh token")
		}
		return nil, apperrors.Internal(err)
	}
	if n == 0 {
		return rt, ErrTokenReused
	}
	return rt, nil
}

func (r *tokenRepository) Revoke(ctx context.Context, token string) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE WHERE refresh_token = ?`
	_, err := r.db.ExecContext(ctx, query, token)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *tokenRepository) RevokeAllByUserID(ctx context.Context, tx *sqlx.Tx, userID uint64) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = ?`
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, userID)
	} else {
		_, err = r.db.ExecContext(ctx, query, userID)
	}
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *tokenRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW() OR revoked = TRUE`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}
