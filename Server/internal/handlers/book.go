package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/yourusername/bibliotheca/internal/dto"
	apperrors "github.com/yourusername/bibliotheca/internal/errors"
	"github.com/yourusername/bibliotheca/internal/services"
	"github.com/yourusername/bibliotheca/internal/utils"
)

type BookHandler struct {
	service  *services.BookService
	validate *validator.Validate
}

func NewBookHandler(service *services.BookService, validate *validator.Validate) *BookHandler {
	return &BookHandler{service: service, validate: validate}
}

func (h *BookHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	pg := utils.GetPagination(r)

	resp, err := h.service.GetAll(r.Context(), pg)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "books retrieved", resp)
}

func (h *BookHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	resp, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "book retrieved", resp)
}

func (h *BookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateBookRequest
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

	utils.Success(w, http.StatusCreated, "book created", resp)
}

func (h *BookHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	var req dto.UpdateBookRequest
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

	utils.Success(w, http.StatusOK, "book updated", resp)
}

func (h *BookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "book deleted", nil)
}

func (h *BookHandler) AssignAuthor(w http.ResponseWriter, r *http.Request) {
	bookID := utils.GetPathID(r, "id")
	if bookID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	var req dto.AssignAuthorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, apperrors.BadRequest("invalid request body", err))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.Error(w, apperrors.UnprocessableEntity(err.Error()))
		return
	}

	if err := h.service.AssignAuthor(r.Context(), bookID, req); err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "author assigned to book", nil)
}

func (h *BookHandler) RemoveAuthor(w http.ResponseWriter, r *http.Request) {
	bookID  := utils.GetPathID(r, "id")
	authorID := utils.GetPathID(r, "authorId")

	if bookID == 0 || authorID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book or author id", nil))
		return
	}

	if err := h.service.RemoveAuthor(r.Context(), bookID, authorID); err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "author removed from book", nil)
}

func (h *BookHandler) Search(w http.ResponseWriter, r *http.Request) {
	query  := r.URL.Query().Get("q")
	genre  := r.URL.Query().Get("genre")
	yearStr := r.URL.Query().Get("year")
	pg    := utils.GetPagination(r)

	var year int
	if yearStr != "" {
		y, err := strconv.Atoi(yearStr)
		if err != nil {
			utils.Error(w, apperrors.BadRequest("invalid year parameter", err))
			return
		}
		year = y
	}

	resp, err := h.service.Search(r.Context(), query, genre, year, pg)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "search results", resp)
}

func (h *BookHandler) handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		utils.Error(w, appErr)
		return
	}
	utils.Error(w, apperrors.Internal(err))
}