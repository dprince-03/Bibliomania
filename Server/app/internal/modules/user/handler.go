package user

import (
	"encoding/json"
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

// ── Profile ───────────────────────────────────────────────

// GetMe godoc
//
//	@Summary	Get my profile
//	@Tags		users
//	@Produce	json
//	@Success	200	{object}	utils.APIResponse{data=UserProfileResponse}
//	@Failure	401	{object}	utils.APIError	"missing/invalid token"
//	@Failure	404	{object}	utils.APIError	"account deactivated"
//	@Security	BearerAuth
//	@Router		/users/me [get]
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	resp, err := h.service.GetMe(r.Context(), userID)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "profile retrieved", resp)
}

// UpdateMe godoc
//
//	@Summary		Update my profile
//	@Description	Only touches profile fields (phone_number, bio, profile_picture) — no route changes first_name/last_name/email yet.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		UpdateProfileRequest	true	"Fields to update (all optional)"
//	@Success		200		{object}	utils.APIResponse{data=UserProfileResponse}
//	@Failure		400		{object}	utils.APIError	"invalid body"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Security		BearerAuth
//	@Router			/users/me [patch]
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.UpdateMe(r.Context(), userID, req)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "profile updated", resp)
}

// ── User Library ──────────────────────────────────────────

// GetLibrary godoc
//
//	@Summary	Get my library
//	@Tags		users
//	@Produce	json
//	@Param		status	query		string	false	"Filter to one status"	Enums(wishlist, to_read, reading, completed, dropped)
//	@Param		page	query		int		false	"Page number"			default(1)
//	@Param		limit	query		int		false	"Items per page"		default(10)
//	@Success	200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]LibraryEntryResponse}}
//	@Failure	401		{object}	utils.APIError	"missing/invalid token"
//	@Security	BearerAuth
//	@Router		/users/me/library [get]
func (h *Handler) GetLibrary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	status := r.URL.Query().Get("status")
	pg := utils.GetPagination(r)

	resp, err := h.service.GetLibrary(r.Context(), userID, status, pg)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "library retrieved", resp)
}

// UpdateLibraryStatus godoc
//
//	@Summary		Set a book's status on my library shelf
//	@Description	Upsert — calling it again for the same book updates the existing entry rather than creating a duplicate.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			bookId	path		int							true	"Book ID"
//	@Param			request	body		UpdateLibraryStatusRequest	true	"New status"
//	@Success		200		{object}	utils.APIResponse{data=LibraryEntryResponse}
//	@Failure		400		{object}	utils.APIError	"invalid book id/body"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		404		{object}	utils.APIError	"book not found"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Security		BearerAuth
//	@Router			/users/me/library/{bookId} [patch]
func (h *Handler) UpdateLibraryStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	bookID := utils.GetPathID(r, "bookId")
	if bookID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	var req UpdateLibraryStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.UpdateLibraryStatus(r.Context(), userID, bookID, req)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "library updated", resp)
}

// ── History ───────────────────────────────────────────────

// GetHistory godoc
//
//	@Summary		Get my reading history
//	@Description	Reading activity (from session data) — not the same as GET /borrows/my, which is checkout history.
//	@Tags			users
//	@Produce		json
//	@Param			page	query		int	false	"Page number"		default(1)
//	@Param			limit	query		int	false	"Items per page"	default(10)
//	@Success		200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]HistoryEntryResponse}}
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Security		BearerAuth
//	@Router			/users/me/history [get]
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	pg := utils.GetPagination(r)

	resp, err := h.service.GetHistory(r.Context(), userID, pg)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "history retrieved", resp)
}

// ── Admin ─────────────────────────────────────────────────

// GetAll godoc
//
//	@Summary		List every user
//	@Description	Includes deactivated accounts (unlike most other lookups), so an admin can find and reactivate one.
//	@Tags			users
//	@Produce		json
//	@Param			page	query		int	false	"Page number"		default(1)
//	@Param			limit	query		int	false	"Items per page"	default(10)
//	@Success		200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]UserResponse}}
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		403		{object}	utils.APIError	"admin only"
//	@Security		BearerAuth
//	@Router			/users [get]
func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	pg := utils.GetPagination(r)

	resp, err := h.service.GetAllUsers(r.Context(), pg)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "users retrieved", resp)
}

// UpdateStatus godoc
//
//	@Summary		Activate or deactivate a user
//	@Description	Deactivating immediately blocks the user's next login and any endpoint that filters to is_active=TRUE (e.g. their own GET /users/me starts 404ing, even with a still-valid token).
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int						true	"User ID"
//	@Param			request	body		UpdateUserStatusRequest	true	"New status"
//	@Success		200		{object}	utils.APIResponse
//	@Failure		400		{object}	utils.APIError	"invalid id/body"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		403		{object}	utils.APIError	"admin only"
//	@Security		BearerAuth
//	@Router			/users/{id}/status [patch]
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	targetID := utils.GetPathID(r, "id")
	if targetID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid user id", nil))
		return
	}

	var req UpdateUserStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.service.UpdateUserStatus(r.Context(), targetID, req); err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "user status updated", nil)
}
