package borrow

import (
	"net/http"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/middleware"
	"github.com/dprince-03/Bibliomania/internal/utils"

	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service  *Service
	validate *validator.Validate
}

func NewHandler(service *Service, validate *validator.Validate) *Handler {
	return &Handler{service: service, validate: validate}
}

func (h *Handler) Routes(mux *http.ServeMux, guard middleware.Guards) {
	mux.Handle("GET /api/v1/borrows", guard.Librarian(h.GetAll))
	mux.Handle("GET /api/v1/borrows/my", guard.Member(h.GetMyBorrows))
	mux.Handle("POST /api/v1/borrows", guard.Member(h.Borrow))
	// Ownership (self, or librarian/admin) is enforced in the service.
	mux.Handle("PATCH /api/v1/borrows/{id}/return", guard.Member(h.Return))
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

	items, total, err := h.service.GetMyBorrows(r.Context(), userID, pg)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "borrows retrieved", utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit))
}

// Borrow godoc
//
//	@Summary		Borrow a book
//	@Description	409 if you already have an active or overdue borrow for this book. 400 if no copies are available. due_at is now + BORROW_LOAN_DAYS.
//	@Description	Send an Idempotency-Key header to make retries safe: a repeat request with the same key returns the original borrow (200) instead of borrowing again (201).
//	@Tags			borrows
//	@Accept			json
//	@Produce		json
//	@Param			request			body		BorrowRequest	true	"Book to borrow"
//	@Param			Idempotency-Key	header		string			false	"Client-chosen key (e.g. a UUID) that makes retries safe"
//	@Success		201				{object}	utils.APIResponse{data=BorrowResponse}
//	@Success		200				{object}	utils.APIResponse{data=BorrowResponse}	"replayed: this key already borrowed"
//	@Failure		400				{object}	utils.APIError							"invalid body"
//	@Failure		401				{object}	utils.APIError							"missing/invalid token"
//	@Failure		404				{object}	utils.APIError							"book not found"
//	@Failure		409				{object}	utils.APIError							"no available copies, or already have an active borrow for this book"
//	@Failure		422				{object}	utils.APIError							"validation failed"
//	@Security		BearerAuth
//	@Router			/borrows [post]
func (h *Handler) Borrow(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req BorrowRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.HandleError(w, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	key := r.Header.Get("Idempotency-Key")
	if len(key) > 100 {
		utils.Error(w, apperrors.BadRequest("Idempotency-Key must be at most 100 characters", nil))
		return
	}

	resp, created, err := h.service.Borrow(r.Context(), userID, middleware.GetUserEmail(r.Context()), req, key)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	if !created {
		utils.Success(w, http.StatusOK, "book already borrowed with this idempotency key", resp)
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
//	@Param			id	path		string	true	"Borrow id — the numeric id or the public_id UUID"
//	@Success		200	{object}	utils.APIResponse{data=BorrowResponse}
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
	ref, ok := utils.GetPathRef(r, "id")
	if !ok {
		utils.Error(w, apperrors.BadRequest("invalid borrow id (numeric id or public UUID)", nil))
		return
	}

	var resp *BorrowResponse
	var err error
	if ref.PublicID != "" {
		resp, err = h.service.ReturnByPublicID(r.Context(), userID, role, ref.PublicID)
	} else {
		resp, err = h.service.Return(r.Context(), userID, role, ref.ID)
	}
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "book returned", resp)
}
