package reading

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

// Sync godoc
//
//	@Summary		Sync offline reading progress
//	@Description	For offline clients: send local progress plus when it was last updated there. If the server already has a newer client_updated_at, this update is silently discarded and the response reflects the server's existing (newer) state — last write wins.
//	@Tags			reading
//	@Accept			json
//	@Produce		json
//	@Param			bookId	path		int						true	"Book ID"
//	@Param			request	body		UpdateProgressRequest	true	"Local progress + client clock"
//	@Success		200		{object}	utils.APIResponse{data=ReadingSessionResponse}
//	@Failure		400		{object}	utils.APIError	"invalid book id/body"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		404		{object}	utils.APIError	"book not found"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Security		BearerAuth
//	@Router			/reading/{bookId}/sync [patch]
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	bookID := utils.GetPathID(r, "bookId")
	if bookID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	var req UpdateProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Sync(r.Context(), userID, bookID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "reading progress synced", resp)
}

// GetSession godoc
//
//	@Summary	Get my reading session for a book
//	@Tags		reading
//	@Produce	json
//	@Param		bookId	path		int	true	"Book ID"
//	@Success	200		{object}	utils.APIResponse{data=ReadingSessionResponse}
//	@Failure	400		{object}	utils.APIError	"invalid book id"
//	@Failure	401		{object}	utils.APIError	"missing/invalid token"
//	@Failure	404		{object}	utils.APIError	"book not found, or no session started yet"
//	@Security	BearerAuth
//	@Router		/reading/{bookId}/session [get]
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	bookID := utils.GetPathID(r, "bookId")
	if bookID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	resp, err := h.service.GetSession(r.Context(), userID, bookID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "reading session retrieved", resp)
}

// UpdateProgress godoc
//
//	@Summary		Update reading progress (online)
//	@Description	The plain, always-online counterpart to sync — no client_updated_at; the server stamps "now", which always beats an offline client syncing in later with an older timestamp.
//	@Tags			reading
//	@Accept			json
//	@Produce		json
//	@Param			bookId	path		int						true	"Book ID"
//	@Param			request	body		ProgressUpdateRequest	true	"Current progress"
//	@Success		200		{object}	utils.APIResponse{data=ReadingSessionResponse}
//	@Failure		400		{object}	utils.APIError	"invalid book id/body"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		404		{object}	utils.APIError	"book not found"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Security		BearerAuth
//	@Router			/reading/{bookId}/progress [patch]
func (h *Handler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	bookID := utils.GetPathID(r, "bookId")
	if bookID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	var req ProgressUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.UpdateProgress(r.Context(), userID, bookID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "reading progress updated", resp)
}

// GetBookmarks godoc
//
//	@Summary		List my bookmarks for a book
//	@Description	Bookmarks are private per user — this never returns another user's bookmarks for the same book.
//	@Tags			reading
//	@Produce		json
//	@Param			bookId	path		int	true	"Book ID"
//	@Success		200		{object}	utils.APIResponse{data=[]BookmarkResponse}
//	@Failure		400		{object}	utils.APIError	"invalid book id"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		404		{object}	utils.APIError	"book not found"
//	@Security		BearerAuth
//	@Router			/reading/{bookId}/bookmarks [get]
func (h *Handler) GetBookmarks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	bookID := utils.GetPathID(r, "bookId")
	if bookID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	resp, err := h.service.GetBookmarks(r.Context(), userID, bookID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "bookmarks retrieved", resp)
}

// CreateBookmark godoc
//
//	@Summary	Create a bookmark
//	@Tags		reading
//	@Accept		json
//	@Produce	json
//	@Param		bookId	path		int				true	"Book ID"
//	@Param		request	body		BookmarkRequest	true	"Bookmark details"
//	@Success	201		{object}	utils.APIResponse{data=BookmarkResponse}
//	@Failure	400		{object}	utils.APIError	"invalid book id/body"
//	@Failure	401		{object}	utils.APIError	"missing/invalid token"
//	@Failure	404		{object}	utils.APIError	"book not found"
//	@Failure	422		{object}	utils.APIError	"validation failed"
//	@Security	BearerAuth
//	@Router		/reading/{bookId}/bookmarks [post]
func (h *Handler) CreateBookmark(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	bookID := utils.GetPathID(r, "bookId")
	if bookID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	var req BookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.CreateBookmark(r.Context(), userID, bookID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusCreated, "bookmark created", resp)
}

// DeleteBookmark godoc
//
//	@Summary		Delete a bookmark
//	@Description	403 if the bookmark belongs to someone else or a different book — no admin override, a bookmark is a personal note.
//	@Tags			reading
//	@Produce		json
//	@Param			bookId	path		int	true	"Book ID"
//	@Param			id		path		int	true	"Bookmark ID"
//	@Success		200		{object}	utils.APIResponse
//	@Failure		400		{object}	utils.APIError	"invalid ids"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		403		{object}	utils.APIError	"not your bookmark"
//	@Failure		404		{object}	utils.APIError	"bookmark not found"
//	@Security		BearerAuth
//	@Router			/reading/{bookId}/bookmarks/{id} [delete]
func (h *Handler) DeleteBookmark(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	bookID := utils.GetPathID(r, "bookId")
	bookmarkID := utils.GetPathID(r, "id")
	if bookID == 0 || bookmarkID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book or bookmark id", nil))
		return
	}

	if err := h.service.DeleteBookmark(r.Context(), userID, bookID, bookmarkID); err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "bookmark deleted", nil)
}

func (h *Handler) handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		utils.Error(w, appErr)
		return
	}
	utils.Error(w, apperrors.Internal(err))
}
