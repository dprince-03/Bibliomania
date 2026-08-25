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

// Sync handles PATCH /api/v1/reading/{bookId}/sync — the client sends its
// local progress plus when it was last updated, and the server resolves any
// conflict against what it already has (see Service.Sync).
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

func (h *Handler) handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		utils.Error(w, appErr)
		return
	}
	utils.Error(w, apperrors.Internal(err))
}
