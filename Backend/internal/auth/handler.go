package auth

import (
	"bibliotheca/internal/dto"
	apperrors "bibliotheca/internal/errors"
	"bibliotheca/internal/utils"
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service *Service
	validate *validator.Validate
}

func NewHandler(service *Service, validate *validator.Validate) *Handler {
	return &Handler{
		service: service,
		validate: validate,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.ErrorResponse(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Register(r.Context(), req)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			utils.ErrorResponse(w, appErr)
			return
		}
		utils.ErrorResponse(w, apperrors.Internal(err))
		return
	}

	utils.SuccessResponse(w, http.StatusCreated, "registration successful", resp)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.ErrorResponse(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Login(r.Context(), req)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			utils.ErrorResponse(w, appErr)
			return
		}
		utils.ErrorResponse(w, apperrors.Internal(err))
		return
	}

	utils.SuccessResponse(w, http.StatusOK, "login successful", resp)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.ErrorResponse(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	err := h.service.Logout(r.Context(), req.RefreshToken)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			utils.ErrorResponse(w, appErr)
			return
		}
		utils.ErrorResponse(w, apperrors.Internal(err))
		return
	}

	utils.SuccessResponse(w, http.StatusOK, "Logout successful", nil)
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.ErrorResponse(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			utils.ErrorResponse(w, appErr)
			return
		}
		utils.ErrorResponse(w, apperrors.Internal(err))
		return
	}

	utils.SuccessResponse(w, http.StatusOK, "Token refreshed successfully", resp)
}