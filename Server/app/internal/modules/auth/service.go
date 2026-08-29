package auth

import (
	"context"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/modules/user"
	"github.com/dprince-03/Bibliomania/internal/utils"
	"github.com/dprince-03/Bibliomania/pkg/jwt"
	refreshtoken "github.com/dprince-03/Bibliomania/pkg/refreshToken"
)

type Service struct {
	userRepo    user.Repository
	profileRepo user.ProfileRepository
	tokenRepo   TokenRepository
	jwtManager  *jwt.Manager
	refreshTTL  time.Duration
}

func NewService(userRepo user.Repository, profileRepo user.ProfileRepository, tokenRepo TokenRepository, jwtManager *jwt.Manager, refreshTTL time.Duration) *Service {
	return &Service{
		userRepo:    userRepo,
		profileRepo: profileRepo,
		tokenRepo:   tokenRepo,
		jwtManager:  jwtManager,
		refreshTTL:  refreshTTL,
	}
}

// issue token helper( internal helper )
func (s *Service) issueTokens(ctx context.Context, u *user.User) (*AuthResponse, error) {
	// generate access token
	accessToken, err := s.jwtManager.GenerateAccessToken(u.ID, u.Email, u.Role)
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
		UserID:       u.ID,
		RefreshToken: hashedRefreshed,
		ExpiresAt:    time.Now().Add(s.refreshTTL),
	}
	if err := s.tokenRepo.Create(ctx, rt); err != nil {
		return nil, err
	}

	return &AuthResponse{
		User: user.UserResponse{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Email:     u.Email,
			Role:      u.Role,
			IsActive:  u.IsActive,
		},
		Token: TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: rawRefreshed,
			// The access token's own TTL, not the refresh token's — this
			// previously reused s.refreshTTL by mistake, so ExpiresIn (and
			// anything relying on it, e.g. a client setting its access-token
			// cookie's Max-Age) reported 7 days instead of the real 15
			// minutes the JWT itself actually expires in.
			ExpiresIn: int64(s.jwtManager.AccessTokenTTL().Seconds()),
		},
	}, nil
}

// -- Register
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	// Check if email already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if existingUser != nil {
		return nil, apperrors.Conflict("email belongs to a user")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	// Create User
	newUser := &user.User{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Password:  hashedPassword,
		Role:      "member",
	}

	userID, err := s.userRepo.Create(ctx, newUser)
	if err != nil {
		return nil, err
	}
	newUser.ID = userID

	// Create empty profile for new user
	profile := &user.UserProfile{
		UserID: userID,
	}
	if err := s.profileRepo.Create(ctx, profile); err != nil {
		return nil, err
	}

	// Issue token
	return s.issueTokens(ctx, newUser)
}

// -- Login
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	// Find user by email
	u, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.Unauthorized("Invalid email or password")
	}

	// Check account is active
	if !u.IsActive {
		return nil, apperrors.Unauthorized("Account is deactivated")
	}

	// Verify password
	if !utils.CheckPassword(req.Password, u.Password) {
		return nil, apperrors.Unauthorized("Invalid email or password")
	}

	// Issue tokens
	return s.issueTokens(ctx, u)
}

// -- Logout
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	// Hash the incoming token to match what's stored in DB
	hashed := refreshtoken.HashToken(rawToken)

	if err := s.tokenRepo.Revoke(ctx, hashed); err != nil {
		return err
	}
	return nil
}

// -- Refresh Token
func (s *Service) RefreshToken(ctx context.Context, rawToken string) (*AuthResponse, error) {
	// Hash token for DB lookup
	hashed := refreshtoken.HashToken(rawToken)

	// find token in DB
	storedToken, err := s.tokenRepo.GetByToken(ctx, hashed)
	if err != nil {
		return nil, apperrors.Unauthorized("invalied or expired refresh token")
	}

	// check expiry
	if time.Now().After(storedToken.ExpiresAt) {
		// Clean it up
		_ = s.tokenRepo.Revoke(ctx, hashed)
		return nil, apperrors.Unauthorized("refresh token expired, please login again")
	}

	// Revoke old token (rotation — one-time use)
	if err := s.tokenRepo.Revoke(ctx, hashed); err != nil {
		return nil, err
	}

	// Get user
	u, err := s.userRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, err
	}

	// Issue refresh token
	return s.issueTokens(ctx, u)
}
