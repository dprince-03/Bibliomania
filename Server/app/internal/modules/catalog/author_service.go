package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dprince-03/Bibliotheca/internal/cache"
	apperrors "github.com/dprince-03/Bibliotheca/internal/errors"
	"github.com/dprince-03/Bibliotheca/internal/utils"
)

type AuthorService struct {
	authorRepo     AuthorRepository
	bookAuthorRepo BookAuthorRepository
	cache          cache.Cache
}

func NewAuthorService(
	authorRepo AuthorRepository,
	bookAuthorRepo BookAuthorRepository,
	cache cache.Cache,
) *AuthorService {
	return &AuthorService{
		authorRepo:     authorRepo,
		bookAuthorRepo: bookAuthorRepo,
		cache:          cache,
	}
}

// ── Get All Authors ───────────────────────────────────────

func (s *AuthorService) GetAll(ctx context.Context, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	cacheKey := fmt.Sprintf("authors:list:%d:%d", pg.Page, pg.Limit)

	// 1. Try cache first
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var resp utils.PaginatedResponse
		if err := json.Unmarshal([]byte(cached), &resp); err == nil {
			return &resp, nil
		}
	}

	// 2. Cache miss — fetch from DB
	authors, total, err := s.authorRepo.GetAll(ctx, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	// 3. Map to response DTOs
	items := make([]AuthorResponse, len(authors))
	for i, a := range authors {
		items[i] = mapAuthorToResponse(a)
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)

	// 4. Store in cache
	if data, err := json.Marshal(resp); err == nil {
		_ = s.cache.Set(ctx, cacheKey, string(data),
			time.Duration(cache.TTLAuthorList)*time.Minute)
	}

	return &resp, nil
}

// ── Get Author By ID ──────────────────────────────────────

func (s *AuthorService) GetByID(ctx context.Context, id uint64) (*AuthorResponse, error) {
	cacheKey := cache.KeyAuthorSingle(id)

	// 1. Try cache first
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var resp AuthorResponse
		if err := json.Unmarshal([]byte(cached), &resp); err == nil {
			return &resp, nil
		}
	}

	// 2. Cache miss — fetch from DB
	author, err := s.authorRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := mapAuthorToResponse(author)

	// 3. Store in cache
	if data, err := json.Marshal(resp); err == nil {
		_ = s.cache.Set(ctx, cacheKey, string(data),
			time.Duration(cache.TTLAuthorSingle)*time.Minute)
	}

	return &resp, nil
}

// ── Get Books By Author ───────────────────────────────────

func (s *AuthorService) GetBooksByAuthor(ctx context.Context, authorID uint64, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	// Verify author exists first
	if _, err := s.authorRepo.GetByID(ctx, authorID); err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("authors:%d:books:%d:%d", authorID, pg.Page, pg.Limit)

	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var resp utils.PaginatedResponse
		if err := json.Unmarshal([]byte(cached), &resp); err == nil {
			return &resp, nil
		}
	}

	books, total, err := s.bookAuthorRepo.GetBooksByAuthorID(ctx, authorID, pg.Limit, pg.Offset)
	if err != nil {
		return nil, err
	}

	items := make([]BookResponse, len(books))
	for i, b := range books {
		items[i] = mapBookToResponse(b, nil)
	}

	resp := utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit)

	if data, err := json.Marshal(resp); err == nil {
		_ = s.cache.Set(ctx, cacheKey, string(data),
			time.Duration(cache.TTLBookList)*time.Minute)
	}

	return &resp, nil
}

// ── Create Author ─────────────────────────────────────────

func (s *AuthorService) Create(ctx context.Context, req CreateAuthorRequest) (*AuthorResponse, error) {
	author := &Author{
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		MiddleName: req.MiddleName,
		Image:      req.Image,
		Biography:  req.Biography,
		Phone:      req.Phone,
		Email:      req.Email,
	}

	// Parse date of birth if provided
	if req.DateOfBirth != nil {
		t, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err != nil {
			return nil, apperrors.BadRequest("invalid date_of_birth format, use YYYY-MM-DD", err)
		}
		author.DateOfBirth = &t
	}

	id, err := s.authorRepo.Create(ctx, author)
	if err != nil {
		return nil, err
	}
	author.ID = id

	// Invalidate author list cache
	s.invalidateAuthorListCache(ctx)

	resp := mapAuthorToResponse(author)
	return &resp, nil
}

// ── Update Author ─────────────────────────────────────────

func (s *AuthorService) Update(ctx context.Context, id uint64, req UpdateAuthorRequest) (*AuthorResponse, error) {
	// 1. Fetch existing record
	author, err := s.authorRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 2. Apply only the fields that were provided (partial update)
	if req.FirstName != nil {
		author.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		author.LastName = *req.LastName
	}
	if req.MiddleName != nil {
		author.MiddleName = req.MiddleName
	}
	if req.Image != nil {
		author.Image = req.Image
	}
	if req.Biography != nil {
		author.Biography = req.Biography
	}
	if req.Phone != nil {
		author.Phone = req.Phone
	}
	if req.Email != nil {
		author.Email = req.Email
	}
	if req.DateOfBirth != nil {
		t, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err != nil {
			return nil, apperrors.BadRequest("invalid date_of_birth format, use YYYY-MM-DD", err)
		}
		author.DateOfBirth = &t
	}

	if err := s.authorRepo.Update(ctx, author); err != nil {
		return nil, err
	}

	// 3. Invalidate caches for this author
	s.invalidateAuthorCache(ctx, id)

	resp := mapAuthorToResponse(author)
	return &resp, nil
}

// ── Delete Author ─────────────────────────────────────────

func (s *AuthorService) Delete(ctx context.Context, id uint64) error {
	// Verify author exists
	if _, err := s.authorRepo.GetByID(ctx, id); err != nil {
		return err
	}

	if err := s.authorRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.invalidateAuthorCache(ctx, id)
	s.invalidateAuthorListCache(ctx)

	return nil
}

// ── Cache invalidation helpers ────────────────────────────

func (s *AuthorService) invalidateAuthorCache(ctx context.Context, id uint64) {
	_ = s.cache.Delete(ctx, cache.KeyAuthorSingle(id))
}

func (s *AuthorService) invalidateAuthorListCache(ctx context.Context) {
	// Flush all author list pages from cache
	_ = s.cache.Delete(ctx,
		"authors:list:1:20",
		"authors:list:1:10",
		"authors:list:2:20",
	)
}
