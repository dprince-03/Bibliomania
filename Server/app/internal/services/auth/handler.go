package auth

import (
	"net/http"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/utils"

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

// Routes registers auth-service's REST routes. Rate limiting (the strict
// per-IP limiter on every /auth route) is applied at the gateway, the one
// edge every client request crosses.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", h.Register)
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.RefreshToken)
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
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.HandleError(w, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Register(r.Context(), req)
	if err != nil {
		utils.HandleError(w, err)
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
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.HandleError(w, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Login(r.Context(), req)
	if err != nil {
		utils.HandleError(w, err)
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
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.HandleError(w, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	if err := h.service.Logout(r.Context(), req.RefreshToken); err != nil {
		utils.HandleError(w, err)
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
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.HandleError(w, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "Token refreshed successfully", resp)
}
