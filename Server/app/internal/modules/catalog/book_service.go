package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dprince-03/Bibliotheca/internal/cache"
	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/utils"
)

type BookService struct {
	bookRepo       BookRepository
	authorRepo     AuthorRepository
	bookAuthorRepo BookAuthorRepository
	cache          cache.Cache
	storagePath    string
	maxUploadMB    int64
}

func NewBookService(
	bookRepo BookRepository,
	authorRepo AuthorRepository,
	bookAuthorRepo BookAuthorRepository,
	cache cache.Cache,
	storagePath string,
	maxUploadMB int64,
) *BookService {
	return &BookService{
		bookRepo:       bookRepo,
		authorRepo:     authorRepo,
		bookAuthorRepo: bookAuthorRepo,
		cache:          cache,
		storagePath:    storagePath,
		maxUploadMB:    maxUploadMB,
	}
}

// ── Get All Books ─────────────────────────────────────────

func (s *BookService) GetAll(ctx context.Context, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	cacheKey := fmt.Sprintf("books:list:%d:%d", pg.Page, pg.Limit)

	// 1. Try cache
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var resp utils.PaginatedResponse
		if err := json.Unmarshal([]byte(cached), &resp); err == nil {
			return &resp, nil
		}
	}

	// 2. Fetch from DB
	books, total, err := s.bookRepo.GetAll(ctx, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	// 3. Enrich each book with its authors
	items, err := s.enrichBooks(ctx, books)
	if err != nil {
		return nil, err
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)

	// 4. Cache the result
	if data, err := json.Marshal(resp); err == nil {
		_ = s.cache.Set(ctx, cacheKey, string(data),
			time.Duration(cache.TTLBookList)*time.Minute)
	}

	return &resp, nil
}

// ── Get Book By ID ────────────────────────────────────────

func (s *BookService) GetByID(ctx context.Context, id uint64) (*BookResponse, error) {
	cacheKey := cache.KeyBookSingle(id)

	// 1. Try cache
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var resp BookResponse
		if err := json.Unmarshal([]byte(cached), &resp); err == nil {
			return &resp, nil
		}
	}

	// 2. Fetch book
	book, err := s.bookRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 3. Fetch its authors
	authors, err := s.bookAuthorRepo.GetAuthorsByBookID(ctx, id)
	if err != nil {
		return nil, err
	}

	authorDTOs := make([]AuthorResponse, len(authors))
	for i, a := range authors {
		authorDTOs[i] = mapAuthorToResponse(a)
	}

	resp := mapBookToResponse(book, authorDTOs)

	// 4. Cache
	if data, err := json.Marshal(resp); err == nil {
		_ = s.cache.Set(ctx, cacheKey, string(data),
			time.Duration(cache.TTLBookSingle)*time.Minute)
	}

	return &resp, nil
}

// ── Create Book ───────────────────────────────────────────

func (s *BookService) Create(ctx context.Context, req CreateBookRequest) (*BookResponse, error) {
	// 1. Validate all author IDs exist before creating the book
	for _, authorID := range req.AuthorIDs {
		if _, err := s.authorRepo.GetByID(ctx, authorID); err != nil {
			return nil, apperrors.BadRequest(
				fmt.Sprintf("author with id %d not found", authorID), nil,
			)
		}
	}

	// 2. Create the book
	book := &Book{
		Title:           req.Title,
		ISBN:            req.ISBN,
		Genre:           req.Genre,
		Description:     req.Description,
		CoverImage:      req.CoverImage,
		PublishedYear:   req.PublishedYear,
		TotalCopies:     req.TotalCopies,
		AvailableCopies: req.TotalCopies, // available = total on creation
		IsDigital:       req.IsDigital,
	}

	bookID, err := s.bookRepo.Create(ctx, book)
	if err != nil {
		return nil, err
	}
	book.ID = bookID

	// 3. Assign authors via junction table
	authorDTOs := []AuthorResponse{}
	for i, authorID := range req.AuthorIDs {
		role := "primary"
		if req.AuthorRoles != nil && i < len(req.AuthorRoles) {
			role = req.AuthorRoles[i]
		}

		ba := &BookAuthor{
			BookID:   bookID,
			AuthorID: authorID,
			Role:     role,
		}
		if err := s.bookAuthorRepo.AssignAuthor(ctx, ba); err != nil {
			return nil, err
		}

		author, err := s.authorRepo.GetByID(ctx, authorID)
		if err != nil {
			return nil, err
		}
		authorDTOs = append(authorDTOs, mapAuthorToResponse(author))
	}

	// 4. Invalidate book list cache
	s.invalidateBookListCache(ctx)

	resp := mapBookToResponse(book, authorDTOs)
	return &resp, nil
}

// ── Update Book ───────────────────────────────────────────

func (s *BookService) Update(ctx context.Context, id uint64, req UpdateBookRequest) (*BookResponse, error) {
	// 1. Fetch existing record
	book, err := s.bookRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 2. Apply partial updates
	if req.Title != nil {
		book.Title = *req.Title
	}
	if req.Genre != nil {
		book.Genre = *req.Genre
	}
	if req.Description != nil {
		book.Description = req.Description
	}
	if req.CoverImage != nil {
		book.CoverImage = req.CoverImage
	}
	if req.PublishedYear != nil {
		book.PublishedYear = req.PublishedYear
	}
	if req.TotalCopies != nil {
		// Adjust available copies proportionally
		diff := *req.TotalCopies - book.TotalCopies
		book.TotalCopies = *req.TotalCopies
		book.AvailableCopies = max(0, book.AvailableCopies+diff)
	}
	if req.IsDigital != nil {
		book.IsDigital = *req.IsDigital
	}

	if err := s.bookRepo.Update(ctx, book); err != nil {
		return nil, err
	}

	// 3. Fetch updated authors
	authors, err := s.bookAuthorRepo.GetAuthorsByBookID(ctx, id)
	if err != nil {
		return nil, err
	}

	authorDTOs := make([]AuthorResponse, len(authors))
	for i, a := range authors {
		authorDTOs[i] = mapAuthorToResponse(a)
	}

	// 4. Invalidate caches
	s.invalidateBookCache(ctx, id)
	s.invalidateBookListCache(ctx)

	resp := mapBookToResponse(book, authorDTOs)
	return &resp, nil
}

// ── Delete Book ───────────────────────────────────────────

func (s *BookService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.bookRepo.GetByID(ctx, id); err != nil {
		return err
	}

	if err := s.bookRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.invalidateBookCache(ctx, id)
	s.invalidateBookListCache(ctx)

	return nil
}

// ── Assign Author to Book ─────────────────────────────────

func (s *BookService) AssignAuthor(ctx context.Context, bookID uint64, req AssignAuthorRequest) error {
	// Verify both exist
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return err
	}
	if _, err := s.authorRepo.GetByID(ctx, req.AuthorID); err != nil {
		return err
	}

	ba := &BookAuthor{
		BookID:   bookID,
		AuthorID: req.AuthorID,
		Role:     req.Role,
	}

	if err := s.bookAuthorRepo.AssignAuthor(ctx, ba); err != nil {
		return err
	}

	s.invalidateBookCache(ctx, bookID)
	return nil
}

// ── Remove Author from Book ───────────────────────────────

func (s *BookService) RemoveAuthor(ctx context.Context, bookID, authorID uint64) error {
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return err
	}

	// Make sure at least one author remains
	authors, err := s.bookAuthorRepo.GetAuthorsByBookID(ctx, bookID)
	if err != nil {
		return err
	}
	if len(authors) <= 1 {
		return apperrors.BadRequest("a book must have at least one author", nil)
	}

	if err := s.bookAuthorRepo.RemoveAuthor(ctx, bookID, authorID); err != nil {
		return err
	}

	s.invalidateBookCache(ctx, bookID)
	return nil
}

// ── Search ────────────────────────────────────────────────

func (s *BookService) Search(
	ctx context.Context,
	params BookSearchParams,
	pg utils.Pagination,
) (*utils.PaginatedResponse, error) {
	cacheKey := cache.KeySearchBooks(params.Query, params.Genre, params.Format, params.AuthorID, params.Year, pg.Page, pg.Limit)

	// Try cache
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var resp utils.PaginatedResponse
		if err := json.Unmarshal([]byte(cached), &resp); err == nil {
			return &resp, nil
		}
	}

	books, total, err := s.bookRepo.Search(ctx, params, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	items, err := s.enrichBooks(ctx, books)
	if err != nil {
		return nil, err
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)

	if data, err := json.Marshal(resp); err == nil {
		_ = s.cache.Set(ctx, cacheKey, string(data),
			time.Duration(cache.TTLSearchResult)*time.Minute)
	}

	return &resp, nil
}

// ── E-Library: Upload / Download ──────────────────────────

// UploadFile validates and saves a book's digital copy, then updates the
// book record. The stored path is relative to storagePath (not absolute) so
// a differently-configured storagePath (e.g. between dev and prod) doesn't
// orphan existing records — GetDownloadPath re-joins it at read time.
func (s *BookService) UploadFile(ctx context.Context, bookID uint64, filename string, size int64, src io.Reader) (*BookResponse, error) {
	if _, err := s.bookRepo.GetByID(ctx, bookID); err != nil {
		return nil, err
	}

	format, err := utils.ValidateFileFormat(filename)
	if err != nil {
		return nil, apperrors.BadRequest(err.Error(), err)
	}
	if format != "pdf" && format != "epub" {
		return nil, apperrors.BadRequest("only pdf and epub files are supported for books", nil)
	}

	if err := utils.ValidateFileSize(size, s.maxUploadMB); err != nil {
		return nil, apperrors.BadRequest(err.Error(), err)
	}

	relDir := filepath.Join("books", fmt.Sprint(bookID))
	absDir := filepath.Join(s.storagePath, relDir)
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, apperrors.Internal(err)
	}

	safeName := utils.SanitizeFilename(filename)
	relPath := filepath.Join(relDir, safeName)
	absPath := filepath.Join(s.storagePath, relPath)

	dst, err := os.Create(absPath)
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	if err := s.bookRepo.UpdateFilePath(ctx, bookID, relPath, written, format); err != nil {
		return nil, err
	}

	s.invalidateBookCache(ctx, bookID)

	updated, err := s.bookRepo.GetByID(ctx, bookID)
	if err != nil {
		return nil, err
	}
	authors, err := s.bookAuthorRepo.GetAuthorsByBookID(ctx, bookID)
	if err != nil {
		return nil, err
	}
	authorDTOs := make([]AuthorResponse, len(authors))
	for i, a := range authors {
		authorDTOs[i] = mapAuthorToResponse(a)
	}

	resp := mapBookToResponse(updated, authorDTOs)
	return &resp, nil
}

// GetDownloadPath returns the absolute on-disk path and a display filename
// for a book's digital copy, or a 404 if it doesn't have one.
func (s *BookService) GetDownloadPath(ctx context.Context, bookID uint64) (absPath, filename string, err error) {
	book, err := s.bookRepo.GetByID(ctx, bookID)
	if err != nil {
		return "", "", err
	}
	if book.FilePath == nil || *book.FilePath == "" {
		return "", "", apperrors.NotFound("digital file for this book")
	}

	absPath = filepath.Join(s.storagePath, *book.FilePath)
	filename = filepath.Base(absPath)
	return absPath, filename, nil
}

// ── Helpers ───────────────────────────────────────────────

// enrichBooks fetches authors for a slice of books efficiently.
// Instead of N+1 queries, we fetch authors per book in one pass.
func (s *BookService) enrichBooks(ctx context.Context, books []*Book) ([]BookResponse, error) {
	items := make([]BookResponse, len(books))

	for i, b := range books {
		authors, err := s.bookAuthorRepo.GetAuthorsByBookID(ctx, b.ID)
		if err != nil {
			return nil, err
		}

		authorDTOs := make([]AuthorResponse, len(authors))
		for j, a := range authors {
			authorDTOs[j] = mapAuthorToResponse(a)
		}

		items[i] = mapBookToResponse(b, authorDTOs)
	}

	return items, nil
}

func (s *BookService) invalidateBookCache(ctx context.Context, id uint64) {
	_ = s.cache.Delete(ctx, cache.KeyBookSingle(id))
}

func (s *BookService) invalidateBookListCache(ctx context.Context) {
	_ = s.cache.Delete(ctx,
		"books:list:1:20",
		"books:list:1:10",
		"books:list:2:20",
	)
}
