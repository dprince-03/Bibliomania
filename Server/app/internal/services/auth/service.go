package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/events"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
	refreshtoken "github.com/dprince-03/Bibliomania/pkg/refreshToken"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

const serviceName = "auth-service"

type Service struct {
	db          *sqlx.DB
	accountRepo AccountRepository
	tokenRepo   TokenRepository
	jwtManager  *jwt.Manager
	refreshTTL  time.Duration
}

func NewService(db *sqlx.DB, accountRepo AccountRepository, tokenRepo TokenRepository, jwtManager *jwt.Manager, refreshTTL time.Duration) *Service {
	return &Service{
		db:          db,
		accountRepo: accountRepo,
		tokenRepo:   tokenRepo,
		jwtManager:  jwtManager,
		refreshTTL:  refreshTTL,
	}
}

// issue token helper( internal helper )
func (s *Service) issueTokens(ctx context.Context, a *Account) (*AuthResponse, error) {
	// generate access token
	accessToken, err := s.jwtManager.GenerateAccessToken(a.ID, a.Email, a.Role)
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	// generate refresh token
	rawRefreshed, hashedRefreshed, err := refreshtoken.Generate()
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	// Store hashed refresh tokens in DB
	rt := &RefreshToken{
		UserID:       a.ID,
		RefreshToken: hashedRefreshed,
		ExpiresAt:    time.Now().Add(s.refreshTTL),
	}
	if err := s.tokenRepo.Create(ctx, rt); err != nil {
		return nil, err
	}

	return &AuthResponse{
		User: UserResponse{
			ID:        a.ID,
			FirstName: a.FirstName,
			LastName:  a.LastName,
			Email:     a.Email,
			Role:      a.Role,
			IsActive:  a.IsActive,
		},
		Token: TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: rawRefreshed,
			// The access token's own TTL, not the refresh token's.
			ExpiresIn: int64(s.jwtManager.AccessTokenTTL().Seconds()),
		},
	}, nil
}

// -- Register
//
// The account row and its auth.user_registered event commit in one
// transaction (outbox), so user-service is guaranteed to eventually learn
// about every account that exists here — and never about one that doesn't.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	req.Email = utils.NormalizeEmail(req.Email)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	// Check if email already exists (the UNIQUE constraint below is what
	// actually guarantees it under concurrency; this gives the nicer error
	// for the common case).
	if existing, _ := s.accountRepo.GetByEmail(ctx, req.Email); existing != nil {
		return nil, apperrors.Conflict("email belongs to a user")
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	account := &Account{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Password:  hashedPassword,
		Role:      middleware.RoleMember,
		IsActive:  true,
	}

	err = events.InTx(ctx, s.db, func(tx *sqlx.Tx) error {
		id, err := s.accountRepo.Create(ctx, tx, account)
		if err != nil {
			return err
		}
		account.ID = id
		return addUserRegistered(ctx, tx, account)
	})
	if err != nil {
		if isDuplicateKey(err) {
			return nil, apperrors.Conflict("email belongs to a user")
		}
		return nil, err
	}

	return s.issueTokens(ctx, account)
}

// addUserRegistered writes the auth.user_registered event for a into tx's
// outbox. Shared with cmd/seed, which creates the admin account directly.
func addUserRegistered(ctx context.Context, tx *sqlx.Tx, a *Account) error {
	e, err := events.New(ctx, events.TypeUserRegistered, serviceName, utils.Uint64Key(a.ID), events.UserRegistered{
		UserID:    a.ID,
		Email:     a.Email,
		FirstName: a.FirstName,
		LastName:  a.LastName,
		Role:      a.Role,
		IsActive:  a.IsActive,
	})
	if err != nil {
		return err
	}
	return events.AddToOutbox(ctx, tx, e)
}

// CreateAccount inserts an account with an explicit role plus its
// auth.user_registered event — the seed tool's way to create the admin,
// since registration always produces a member.
func CreateAccount(ctx context.Context, db *sqlx.DB, a *Account) error {
	repo := NewAccountRepository(db)
	return events.InTx(ctx, db, func(tx *sqlx.Tx) error {
		id, err := repo.Create(ctx, tx, a)
		if err != nil {
			return err
		}
		a.ID = id
		return addUserRegistered(ctx, tx, a)
	})
}

func isDuplicateKey(err error) bool {
	var myErr *mysql.MySQLError
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Err != nil {
		err = appErr.Err
	}
	return errors.As(err, &myErr) && myErr.Number == 1062
}

// -- Login
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	req.Email = utils.NormalizeEmail(req.Email)
	a, err := s.accountRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.Unauthorized("Invalid email or password")
	}

	if !a.IsActive {
		return nil, apperrors.Unauthorized("Account is deactivated")
	}

	if !utils.CheckPassword(req.Password, a.Password) {
		return nil, apperrors.Unauthorized("Invalid email or password")
	}

	return s.issueTokens(ctx, a)
}

// -- Logout
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	hashed := refreshtoken.HashToken(rawToken)
	return s.tokenRepo.Revoke(ctx, hashed)
}

// -- Refresh Token
//
// Rotation is one-time use, enforced atomically in the database
// (TokenRepository.Consume): of two concurrent refreshes with the same
// token, exactly one succeeds. A token presented again after it was used is
// treated as stolen — the legitimate client already holds its successor —
// so every session for that user is revoked (refresh-token reuse detection,
// per the OAuth 2.0 Security BCP) and they must log in again.
func (s *Service) RefreshToken(ctx context.Context, rawToken string) (*AuthResponse, error) {
	hashed := refreshtoken.HashToken(rawToken)

	storedToken, err := s.tokenRepo.Consume(ctx, hashed)
	if errors.Is(err, ErrTokenReused) {
		if rerr := s.tokenRepo.RevokeAllByUserID(ctx, nil, storedToken.UserID); rerr != nil {
			return nil, rerr
		}
		slog.WarnContext(ctx, "refresh token reuse detected — all sessions revoked", "user_id", storedToken.UserID)
		return nil, apperrors.Unauthorized("refresh token already used, please login again")
	}
	if err != nil {
		return nil, apperrors.Unauthorized("invalid or expired refresh token")
	}

	if time.Now().After(storedToken.ExpiresAt) {
		return nil, apperrors.Unauthorized("refresh token expired, please login again")
	}

	// GetByID filters to active accounts, so a deactivated user's refresh
	// fails here even if their token predates the deactivation.
	a, err := s.accountRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, a)
}

// ── Event consumers ───────────────────────────────────────

// OnUserStatusChanged applies an admin (de)activation made in user-service
// to this service's copy — and on deactivation also revokes every refresh
// token, so the user is out as soon as their current access token expires.
func (s *Service) OnUserStatusChanged(ctx context.Context, e events.Event) error {
	var p events.UserStatusChanged
	if err := e.Decode(&p); err != nil {
		return err
	}
	return events.InTx(ctx, s.db, func(tx *sqlx.Tx) error {
		first, err := events.MarkProcessed(ctx, tx, e.ID)
		if err != nil || !first {
			return err
		}
		if err := s.accountRepo.UpdateStatus(ctx, tx, p.UserID, p.IsActive); err != nil {
			return err
		}
		if !p.IsActive {
			return s.tokenRepo.RevokeAllByUserID(ctx, tx, p.UserID)
		}
		return nil
	})
}
