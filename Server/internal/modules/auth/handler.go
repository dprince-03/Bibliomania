package auth

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/utils"

	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service  *Service
	validate *validator.Validate
}

func NewHandler(service *Service, validate *validator.Validate) *Handler {
	return &Handler{
		service:  service,
		validate: validate,
	}
}

// Register godoc
//
//	@Summary		Register a new member
//	@Description	Creates a member account (role is always "member") and returns access + refresh tokens
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RegisterRequest	true	"Registration details"
//	@Success		201		{object}	utils.APIResponse{data=AuthResponse}
//	@Failure		400		{object}	utils.APIError	"invalid body"
//	@Failure		409		{object}	utils.APIError	"email already registered"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Router			/auth/register [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Register(r.Context(), req)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			utils.Error(w, appErr)
			return
		}
		utils.Error(w, apperrors.Internal(err))
		return
	}

	utils.Success(w, http.StatusCreated, "registration successful", resp)
}

// Login godoc
//
//	@Summary		Log in
//	@Description	Verifies credentials and returns access + refresh tokens
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequest	true	"Credentials"
//	@Success		200		{object}	utils.APIResponse{data=AuthResponse}
//	@Failure		400		{object}	utils.APIError	"invalid body"
//	@Failure		401		{object}	utils.APIError	"wrong credentials or deactivated account"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Router			/auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Login(r.Context(), req)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			utils.Error(w, appErr)
			return
		}
		utils.Error(w, apperrors.Internal(err))
		return
	}

	utils.Success(w, http.StatusOK, "login successful", resp)
}

// Logout godoc
//
//	@Summary		Log out
//	@Description	Revokes the given refresh token (does not touch the still-live access token — it just expires naturally)
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RefreshTokenRequest	true	"Refresh token to revoke"
//	@Success		200		{object}	utils.APIResponse
//	@Failure		400		{object}	utils.APIError	"invalid body"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Router			/auth/logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	err := h.service.Logout(r.Context(), req.RefreshToken)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			utils.Error(w, appErr)
			return
		}
		utils.Error(w, apperrors.Internal(err))
		return
	}

	utils.Success(w, http.StatusOK, "Logout successful", nil)
}

// RefreshToken godoc
//
//	@Summary		Refresh tokens
//	@Description	Rotates the refresh token (one-time use — the old one is revoked) and returns a new access + refresh token pair
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RefreshTokenRequest	true	"Current refresh token"
//	@Success		200		{object}	utils.APIResponse{data=AuthResponse}
//	@Failure		400		{object}	utils.APIError	"invalid body"
//	@Failure		401		{object}	utils.APIError	"invalid, expired, or already-used refresh token"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Router			/auth/refresh [post]
func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			utils.Error(w, appErr)
			return
		}
		utils.Error(w, apperrors.Internal(err))
		return
	}

	utils.Success(w, http.StatusOK, "Token refreshed successfully", resp)
}
