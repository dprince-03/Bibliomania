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

// GetAll godoc
//
//	@Summary	List authors
//	@Tags		authors
//	@Produce	json
//	@Param		page	query		int	false	"Page number"		default(1)
//	@Param		limit	query		int	false	"Items per page"	default(10)
//	@Success	200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]AuthorResponse}}
//	@Router		/authors [get]
func (h *AuthorHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	pg := utils.GetPagination(r)

	resp, err := h.service.GetAll(r.Context(), pg)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "authors retrieved", resp)
}

// GetByID godoc
//
//	@Summary	Get an author
//	@Tags		authors
//	@Produce	json
//	@Param		id	path		int	true	"Author ID"
//	@Success	200	{object}	utils.APIResponse{data=AuthorResponse}
//	@Failure	400	{object}	utils.APIError	"invalid id"
//	@Failure	404	{object}	utils.APIError	"author not found"
//	@Router		/authors/{id} [get]
func (h *AuthorHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid author id", nil))
		return
	}

	resp, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "author retrieved", resp)
}

// GetBooksByAuthor godoc
//
//	@Summary	List an author's books
//	@Tags		authors
//	@Produce	json
//	@Param		id		path		int	true	"Author ID"
//	@Param		page	query		int	false	"Page number"		default(1)
//	@Param		limit	query		int	false	"Items per page"	default(10)
//	@Success	200		{object}	utils.APIResponse{data=utils.PaginatedResponse{items=[]BookResponse}}
//	@Failure	400		{object}	utils.APIError	"invalid id"
//	@Failure	404		{object}	utils.APIError	"author not found"
//	@Router		/authors/{id}/books [get]
func (h *AuthorHandler) GetBooksByAuthor(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid author id", nil))
		return
	}

	pg := utils.GetPagination(r)

	resp, err := h.service.GetBooksByAuthor(r.Context(), id, pg)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "books retrieved", resp)
}

// Create godoc
//
//	@Summary	Create an author
//	@Tags		authors
//	@Accept		json
//	@Produce	json
//	@Param		request	body		CreateAuthorRequest	true	"Author details"
//	@Success	201		{object}	utils.APIResponse{data=AuthorResponse}
//	@Failure	400		{object}	utils.APIError	"invalid body"
//	@Failure	401		{object}	utils.APIError	"missing/invalid token"
//	@Failure	403		{object}	utils.APIError	"librarian/admin only"
//	@Failure	422		{object}	utils.APIError	"validation failed"
//	@Security	BearerAuth
//	@Router		/authors [post]
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
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusCreated, "author created", resp)
}

// Update godoc
//
//	@Summary	Update an author
//	@Tags		authors
//	@Accept		json
//	@Produce	json
//	@Param		id		path		int					true	"Author ID"
//	@Param		request	body		UpdateAuthorRequest	true	"Fields to update (all optional)"
//	@Success	200		{object}	utils.APIResponse{data=AuthorResponse}
//	@Failure	400		{object}	utils.APIError	"invalid id/body"
//	@Failure	401		{object}	utils.APIError	"missing/invalid token"
//	@Failure	403		{object}	utils.APIError	"librarian/admin only"
//	@Failure	404		{object}	utils.APIError	"author not found"
//	@Failure	422		{object}	utils.APIError	"validation failed"
//	@Security	BearerAuth
//	@Router		/authors/{id} [put]
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
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "author updated", resp)
}

// Delete godoc
//
//	@Summary	Delete an author
//	@Tags		authors
//	@Produce	json
//	@Param		id	path		int	true	"Author ID"
//	@Success	200	{object}	utils.APIResponse
//	@Failure	400	{object}	utils.APIError	"invalid id"
//	@Failure	401	{object}	utils.APIError	"missing/invalid token"
//	@Failure	403	{object}	utils.APIError	"admin only"
//	@Failure	404	{object}	utils.APIError	"author not found"
//	@Security	BearerAuth
//	@Router		/authors/{id} [delete]
func (h *AuthorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := utils.GetPathID(r, "id")
	if id == 0 {
		utils.Error(w, apperrors.BadRequest("invalid author id", nil))
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		utils.HandleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, "author deleted", nil)
}
