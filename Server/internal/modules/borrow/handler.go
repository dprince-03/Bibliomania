package borrow

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/middleware"
	"github.com/dprince-03/Bibliotheca/internal/utils"

	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service  *Service
	validate *validator.Validate
}

func NewHandler(service *Service, validate *validator.Validate) *Handler {
	return &Handler{service: service, validate: validate}
}

// GetAll handles GET /api/v1/borrows — admin/librarian only.
func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	pg := utils.GetPagination(r)

	resp, err := h.service.GetAll(r.Context(), pg)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "borrows retrieved", resp)
}

// GetMyBorrows handles GET /api/v1/borrows/my.
func (h *Handler) GetMyBorrows(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	pg := utils.GetPagination(r)

	resp, err := h.service.GetMyBorrows(r.Context(), userID, pg)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "borrows retrieved", resp)
}

// Borrow handles POST /api/v1/borrows.
func (h *Handler) Borrow(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req BorrowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Borrow(r.Context(), userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusCreated, "book borrowed", resp)
}

// Return handles PATCH /api/v1/borrows/{id}/return.
func (h *Handler) Return(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	role := middleware.GetUserRole(r.Context())
	borrowID := utils.GetPathID(r, "id")
	if borrowID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid borrow id", nil))
		return
	}

	if err := h.service.Return(r.Context(), userID, role, borrowID); err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "book returned", nil)
}

func (h *Handler) handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		utils.Error(w, appErr)
		return
	}
	utils.Error(w, apperrors.Internal(err))
}
