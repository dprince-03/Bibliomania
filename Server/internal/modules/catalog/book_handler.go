package catalog

import (
	"encoding/json"
	"net/http"
	"strconv"

	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/utils"

	"github.com/go-playground/validator/v10"
)

type BookHandler struct {
	service         *BookService
	validate        *validator.Validate
	maxUploadSizeMB int64
}

func NewBookHandler(service *BookService, validate *validator.Validate, maxUploadSizeMB int64) *BookHandler {
	return &BookHandler{service: service, validate: validate, maxUploadSizeMB: maxUploadSizeMB}
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
	var req CreateBookRequest
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

	var req UpdateBookRequest
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

	var req AssignAuthorRequest
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
	bookID := utils.GetPathID(r, "id")
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

// Upload handles POST /api/v1/books/{id}/upload (multipart/form-data, field
// name "file"). MaxBytesReader caps the whole request body up front, before
// ParseMultipartForm buffers anything — an oversized upload is rejected
// early rather than after being fully read into memory/disk.
func (h *BookHandler) Upload(w http.ResponseWriter, r *http.Request) {
	bookID := utils.GetPathID(r, "id")
	if bookID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	maxBytes := h.maxUploadSizeMB * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.Error(w, apperrors.BadRequest("file too large or invalid multipart form", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.Error(w, apperrors.BadRequest("a 'file' field is required", err))
		return
	}
	defer file.Close()

	resp, err := h.service.UploadFile(r.Context(), bookID, header.Filename, header.Size, file)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "file uploaded", resp)
}

// Download handles GET /api/v1/books/{id}/download. Uses http.ServeFile so
// range requests and content-type sniffing work for large PDF/EPUB files.
func (h *BookHandler) Download(w http.ResponseWriter, r *http.Request) {
	bookID := utils.GetPathID(r, "id")
	if bookID == 0 {
		utils.Error(w, apperrors.BadRequest("invalid book id", nil))
		return
	}

	absPath, filename, err := h.service.GetDownloadPath(r.Context(), bookID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	http.ServeFile(w, r, absPath)
}

func (h *BookHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pg := utils.GetPagination(r)

	params := BookSearchParams{
		Query: q.Get("q"),
		Genre: q.Get("genre"),
	}

	if format := q.Get("format"); format != "" {
		if format != "digital" && format != "physical" {
			utils.Error(w, apperrors.BadRequest("invalid format parameter, must be 'digital' or 'physical'", nil))
			return
		}
		params.Format = format
	}

	if authorStr := q.Get("author"); authorStr != "" {
		authorID, err := strconv.ParseUint(authorStr, 10, 64)
		if err != nil {
			utils.Error(w, apperrors.BadRequest("invalid author parameter", err))
			return
		}
		params.AuthorID = authorID
	}

	if yearStr := q.Get("year"); yearStr != "" {
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			utils.Error(w, apperrors.BadRequest("invalid year parameter", err))
			return
		}
		params.Year = year
	}

	resp, err := h.service.Search(r.Context(), params, pg)
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
