package catalog

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/utils"

	"github.com/go-playground/validator/v10"
)

type AuthorHandler struct {
	service  *AuthorService
	validate *validator.Validate
}

func NewAuthorHandler(service *AuthorService, validate *validator.Validate) *AuthorHandler {
	return &AuthorHandler{service: service, validate: validate}
}

func (h *AuthorHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	pg := utils.GetPagination(r)

	resp, err := h.service.GetAll(r.Context(), pg)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "authors retrieved", resp)
}

func (h *AuthorHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid author id", nil))
		return
	}

	resp, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "author retrieved", resp)
}

func (h *AuthorHandler) GetBooksByAuthor(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid author id", nil))
		return
	}

	pg := utils.GetPagination(r)

	resp, err := h.service.GetBooksByAuthor(r.Context(), id, pg)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "books retrieved", resp)
}

func (h *AuthorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateAuthorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Create(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusCreated, "author created", resp)
}

func (h *AuthorHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid author id", nil))
		return
	}

	var req UpdateAuthorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	resp, err := h.service.Update(r.Context(), id, req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "author updated", resp)
}

func (h *AuthorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid author id", nil))
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "author deleted", nil)
}

// handleError is a local helper — converts any error type to the right HTTP response
func (h *AuthorHandler) handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		utils.Error(w, appErr)
		return
	}
	utils.Error(w, apperrors.Internal(err))
}
