package models

import "time"

type RefreshToken struct {
	ID           uint64    `db:"id"`
	UserID       uint64    `db:"user_id"`
	RefreshToken string    `db:"refresh_token"`
	ExpiresAt    time.Time `db:"expires_at"`
	Revoked      bool      `db:"revoked"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}
