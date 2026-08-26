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

// GetAll godoc
//
//	@Summary		List every borrow record
//	@Description	Sweeps overdue records first, so status is always current as of this request.
//	@Tags			borrows
//	@Produce		json
//	@Param			page	query		int	false	"Page number"		default(1)
//	@Param			limit	query		int	false	"Items per page"	default(10)
//	@Success		200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]BorrowResponse}}
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		403		{object}	utils.APIError	"librarian/admin only"
//	@Security		BearerAuth
//	@Router			/borrows [get]
func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	pg := utils.GetPagination(r)

	resp, err := h.service.GetAll(r.Context(), pg)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "borrows retrieved", resp)
}

// GetMyBorrows godoc
//
//	@Summary	List my borrow records
//	@Tags		borrows
//	@Produce	json
//	@Param		page	query		int	false	"Page number"		default(1)
//	@Param		limit	query		int	false	"Items per page"	default(10)
//	@Success	200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]BorrowResponse}}
//	@Failure	401		{object}	utils.APIError	"missing/invalid token"
//	@Security	BearerAuth
//	@Router		/borrows/my [get]
func (h *Handler) GetMyBorrows(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	pg := utils.GetPagination(r)

	resp, err := h.service.GetMyBorrows(r.Context(), userID, pg)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "borrows retrieved", resp)
}

// Borrow godoc
//
//	@Summary		Borrow a book
//	@Description	409 if you already have an active borrow for this book. 400 if no copies are available. due_at is now + BORROW_LOAN_DAYS.
//	@Tags			borrows
//	@Accept			json
//	@Produce		json
//	@Param			request	body		BorrowRequest	true	"Book to borrow"
//	@Success		201		{object}	utils.APIResponse{data=BorrowResponse}
//	@Failure		400		{object}	utils.APIError	"invalid body, or no available copies"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		404		{object}	utils.APIError	"book not found"
//	@Failure		409		{object}	utils.APIError	"already have an active borrow for this book"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Security		BearerAuth
//	@Router			/borrows [post]
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
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusCreated, "book borrowed", resp)
}

// Return godoc
//
//	@Summary		Return a borrowed book
//	@Description	A member may only return their own borrow; librarian/admin may return anyone's (e.g. processing a physical return at a desk).
//	@Tags			borrows
//	@Produce		json
//	@Param			id	path		int	true	"Borrow ID"
//	@Success		200	{object}	utils.APIResponse
//	@Failure		400	{object}	utils.APIError	"invalid id"
//	@Failure		401	{object}	utils.APIError	"missing/invalid token"
//	@Failure		403	{object}	utils.APIError	"not your borrow, and not librarian/admin"
//	@Failure		404	{object}	utils.APIError	"borrow not found"
//	@Failure		409	{object}	utils.APIError	"already returned"
//	@Security		BearerAuth
//	@Router			/borrows/{id}/return [patch]
func (h *Handler) Return(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	role := middleware.GetUserRole(r.Context())
	borrowID := utils.GetPathID(r, "id")
	if borrowID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid borrow id", nil))
		return
	}

	if err := h.service.Return(r.Context(), userID, role, borrowID); err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "book returned", nil)
}
