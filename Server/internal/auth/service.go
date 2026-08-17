package auth

import (
	"bibliotheca/internal/dto"
	apperrors "bibliotheca/internal/errors"
	"bibliotheca/internal/models"
	"bibliotheca/internal/repository"
	"bibliotheca/internal/utils"
	"bibliotheca/pkg/jwt"
	refreshtoken "bibliotheca/pkg/refreshToken"
	"context"
	"time"
)

type Service struct {
	userRepo    repository.UserRepository
	profileRepo repository.UserProfileRepository
	tokenRepo   repository.TokenRepository
	jwtManager  *jwt.Manager
	refreshTTL  time.Duration
}

func NewService(userRepo repository.UserRepository, profileRepo repository.UserProfileRepository, tokenRepo repository.TokenRepository, jwtManager *jwt.Manager, refreshTTL time.Duration) *Service {
	return &Service{
		userRepo:    userRepo,
		profileRepo: profileRepo,
		tokenRepo:   tokenRepo,
		jwtManager:  jwtManager,
		refreshTTL:  refreshTTL,
	}
}

// issue token helper( internal helper )
func (s *Service) issueTokens(ctx context.Context, user *models.User) (*dto.AuthResponse, error) {
	// generate access token
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	// generate refresh token
	rawRefreshed, hashedRefreshed, err := refreshtoken.Generate()
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	// Store hashed refresh tokens in DB
	rt := &models.RefreshToken{
		UserID:       user.ID,
		RefreshToken: hashedRefreshed,
		ExpiresAt:    time.Now().Add(s.refreshTTL),
	}
	if err := s.tokenRepo.Create(ctx, rt); err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		User: dto.UserResponse{
			ID:        user.ID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Role:      user.Role,
			IsActive:  user.IsActive,
		},
		Token: dto.TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: rawRefreshed,
			ExpiresIn:    int64(s.refreshTTL.Seconds()),
		},
	}, nil
}

// -- Register
func (s *Service) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
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
	user := &models.User{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Password:  hashedPassword,
		Role:      "member",
	}

	userID, err := s.userRepo.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	user.ID = userID

	// Create empty profile for new user
	profile := &models.UserProfile{
		UserID: userID,
	}
	if err := s.profileRepo.CreateUserProfile(ctx, profile); err != nil {
		return nil, err
	}

	// Issue token
	return s.issueTokens(ctx, user)
}

// -- Login
func (s *Service) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	// Find user by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.Unauthorized("Invalid email or password")
	}

	// Check account is active
	if !user.IsActive {
		return nil, apperrors.Unauthorized("Account is deactivated")
	}

	// Verify password
	if  !utils.CheckPassword(req.Password, user.Password) {
		return nil, apperrors.Unauthorized("Invalid email or password")
	}

	// Issue tokens
	return s.issueTokens(ctx, user)
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
func (s *Service) RefreshToken(ctx context.Context, rawToken string) (*dto.AuthResponse, error) {
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
	user, err := s.userRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, err
	}

	// Issue refresh token
	return s.issueTokens(ctx, user)
}