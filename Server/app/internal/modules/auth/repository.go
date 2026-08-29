package auth

import (
	"context"
	"database/sql"
	"errors"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"

	"github.com/jmoiron/sqlx"
)

type TokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	GetByToken(ctx context.Context, token string) (*RefreshToken, error)
	Revoke(ctx context.Context, token string) error
	RevokeAllByUserID(ctx context.Context, userID uint64) error
	DeleteExpired(ctx context.Context) error
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

func (r *tokenRepository) Revoke(ctx context.Context, token string) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE WHERE refresh_token = ?`
	_, err := r.db.ExecContext(ctx, query, token)
	if err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (r *tokenRepository) RevokeAllByUserID(ctx context.Context, userID uint64) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = ?`
	_, err := r.db.ExecContext(ctx, query, userID)
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
