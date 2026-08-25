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

// GetAll godoc
//
//	@Summary	List books
//	@Tags		books
//	@Produce	json
//	@Param		page	query		int	false	"Page number"		default(1)
//	@Param		limit	query		int	false	"Items per page"	default(10)
//	@Success	200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]BookResponse}}
//	@Router		/books [get]
func (h *BookHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	pg := utils.GetPagination(r)

	resp, err := h.service.GetAll(r.Context(), pg)
	if err != nil {
		h.handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "books retrieved", resp)
}

// GetByID godoc
//
//	@Summary	Get a book
//	@Tags		books
//	@Produce	json
//	@Param		id	path		int	true	"Book ID"
//	@Success	200	{object}	utils.APIResponse{data=BookResponse}
//	@Failure	400	{object}	utils.APIError	"invalid id"
//	@Failure	404	{object}	utils.APIError	"book not found"
//	@Router		/books/{id} [get]
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

// Create godoc
//
//	@Summary		Create a book
//	@Description	author_ids is required (every author must already exist); author_roles is optional and positional. available_copies is set equal to total_copies and cannot be set directly.
//	@Tags			books
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateBookRequest	true	"Book details"
//	@Success		201		{object}	utils.APIResponse{data=BookResponse}
//	@Failure		400		{object}	utils.APIError	"invalid body, or an author_id doesn't exist"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		403		{object}	utils.APIError	"librarian/admin only"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Security		BearerAuth
//	@Router			/books [post]
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

// Update godoc
//
//	@Summary		Update a book
//	@Description	Use the assign/remove-author endpoints for authors, not this one. Changing total_copies adjusts available_copies by the same delta.
//	@Tags			books
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Book ID"
//	@Param			request	body		UpdateBookRequest	true	"Fields to update (all optional)"
//	@Success		200		{object}	utils.APIResponse{data=BookResponse}
//	@Failure		400		{object}	utils.APIError	"invalid id/body"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		403		{object}	utils.APIError	"librarian/admin only"
//	@Failure		404		{object}	utils.APIError	"book not found"
//	@Failure		422		{object}	utils.APIError	"validation failed"
//	@Security		BearerAuth
//	@Router			/books/{id} [put]
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

// Delete godoc
//
//	@Summary	Delete a book
//	@Tags		books
//	@Produce	json
//	@Param		id	path		int	true	"Book ID"
//	@Success	200	{object}	utils.APIResponse
//	@Failure	400	{object}	utils.APIError	"invalid id"
//	@Failure	401	{object}	utils.APIError	"missing/invalid token"
//	@Failure	403	{object}	utils.APIError	"admin only"
//	@Failure	404	{object}	utils.APIError	"book not found"
//	@Security	BearerAuth
//	@Router		/books/{id} [delete]
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

// AssignAuthor godoc
//
//	@Summary	Assign an author to a book
//	@Tags		books
//	@Accept		json
//	@Produce	json
//	@Param		id		path		int					true	"Book ID"
//	@Param		request	body		AssignAuthorRequest	true	"Author + role"
//	@Success	200		{object}	utils.APIResponse
//	@Failure	400		{object}	utils.APIError	"invalid id/body"
//	@Failure	401		{object}	utils.APIError	"missing/invalid token"
//	@Failure	403		{object}	utils.APIError	"librarian/admin only"
//	@Failure	404		{object}	utils.APIError	"book or author not found"
//	@Failure	422		{object}	utils.APIError	"validation failed"
//	@Security	BearerAuth
//	@Router		/books/{id}/authors [post]
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

// RemoveAuthor godoc
//
//	@Summary		Remove an author from a book
//	@Description	Refuses if it would leave the book with zero authors
//	@Tags			books
//	@Produce		json
//	@Param			id			path		int	true	"Book ID"
//	@Param			authorId	path		int	true	"Author ID"
//	@Success		200			{object}	utils.APIResponse
//	@Failure		400			{object}	utils.APIError	"invalid ids, or would leave zero authors"
//	@Failure		401			{object}	utils.APIError	"missing/invalid token"
//	@Failure		403			{object}	utils.APIError	"librarian/admin only"
//	@Security		BearerAuth
//	@Router			/books/{id}/authors/{authorId} [delete]
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

// Upload godoc
//
//	@Summary		Upload a book's digital file
//	@Description	multipart/form-data with a single field named "file". Only .pdf/.epub are accepted, up to MAX_UPLOAD_SIZE_MB. Sets is_digital=true on success; re-uploading replaces the previous file.
//	@Tags			books
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			id		path		int		true	"Book ID"
//	@Param			file	formData	file	true	"PDF or EPUB file"
//	@Success		200		{object}	utils.APIResponse{data=BookResponse}
//	@Failure		400		{object}	utils.APIError	"invalid id, unsupported format, too large, or missing file"
//	@Failure		401		{object}	utils.APIError	"missing/invalid token"
//	@Failure		403		{object}	utils.APIError	"librarian/admin only"
//	@Failure		404		{object}	utils.APIError	"book not found"
//	@Security		BearerAuth
//	@Router			/books/{id}/upload [post]
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

// Download godoc
//
//	@Summary		Download a book's digital file
//	@Description	Streams the file (supports range requests). 404 if the book has no uploaded file yet.
//	@Tags			books
//	@Produce		application/octet-stream
//	@Param			id	path		int	true	"Book ID"
//	@Success		200	{file}		file
//	@Failure		400	{object}	utils.APIError	"invalid id"
//	@Failure		401	{object}	utils.APIError	"missing/invalid token"
//	@Failure		404	{object}	utils.APIError	"book not found, or no file uploaded"
//	@Security		BearerAuth
//	@Router			/books/{id}/download [get]
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

// Search godoc
//
//	@Summary		Search books
//	@Description	q matches book title/description (full-text) OR author name. genre/format/author/year are exact-match filters, independent of q and each other.
//	@Tags			books
//	@Produce		json
//	@Param			q		query		string	false	"Free text — matches title/description or author name"
//	@Param			genre	query		string	false	"Exact genre match"
//	@Param			format	query		string	false	"digital or physical"	Enums(digital, physical)
//	@Param			author	query		int		false	"Author ID (not a name — use q for name search)"
//	@Param			year	query		int		false	"Exact published_year match"
//	@Param			page	query		int		false	"Page number"		default(1)
//	@Param			limit	query		int		false	"Items per page"	default(10)
//	@Success		200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]BookResponse}}
//	@Failure		400		{object}	utils.APIError	"invalid format/author/year parameter"
//	@Router			/search [get]
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
