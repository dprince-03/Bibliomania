package catalog

import (
	"context"
	"time"

	"github.com/dprince-03/Bibliomania/internal/cache"
	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/utils"
)

type AuthorService struct {
	authorRepo     AuthorRepository
	bookAuthorRepo BookAuthorRepository
	cache          cache.Cache
	reads          ReadReplicas // see BookService.ReadReplicas
}

// WithReadReplicas routes public reads to a replica.
func (s *AuthorService) WithReadReplicas(r ReadReplicas) *AuthorService {
	s.reads = r
	return s
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
		reads:          ReadReplicas{Authors: authorRepo, BookAuthors: bookAuthorRepo},
	}
}

// ── Get All Authors ───────────────────────────────────────

func (s *AuthorService) GetAll(ctx context.Context, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	resp, err := cache.ReadThrough(ctx, s.cache, cache.KeyAuthorList(pg.Page, pg.Limit),
		time.Duration(cache.TTLAuthorList)*time.Minute,
		func() (utils.PaginatedResponse, error) {
			authors, total, err := s.reads.Authors.GetAll(ctx, pg.Limit, pg.Offset)
			if err != nil {
				return utils.PaginatedResponse{}, err
			}
			items := make([]AuthorResponse, len(authors))
			for i, a := range authors {
				items[i] = mapAuthorToResponse(a)
			}
			return utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit), nil
		})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ── Get Author By ID ──────────────────────────────────────

func (s *AuthorService) GetByID(ctx context.Context, id uint64) (*AuthorResponse, error) {
	resp, err := cache.ReadThrough(ctx, s.cache, cache.KeyAuthorSingle(id),
		time.Duration(cache.TTLAuthorSingle)*time.Minute,
		func() (AuthorResponse, error) {
			author, err := s.reads.Authors.GetByID(ctx, id)
			if err != nil {
				return AuthorResponse{}, err
			}
			return mapAuthorToResponse(author), nil
		})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ── Get Books By Author ───────────────────────────────────

func (s *AuthorService) GetBooksByAuthor(ctx context.Context, authorID uint64, pg utils.Pagination) (*utils.PaginatedResponse, error) {
	// Verify author exists first (itself cached).
	if _, err := s.GetByID(ctx, authorID); err != nil {
		return nil, err
	}
	resp, err := cache.ReadThrough(ctx, s.cache, cache.KeyAuthorBooks(authorID, pg.Page, pg.Limit),
		time.Duration(cache.TTLBookList)*time.Minute,
		func() (utils.PaginatedResponse, error) {
			books, total, err := s.reads.BookAuthors.GetBooksByAuthorID(ctx, authorID, pg.Limit, pg.Offset)
			if err != nil {
				return utils.PaginatedResponse{}, err
			}
			items := make([]BookResponse, len(books))
			for i, b := range books {
				items[i] = mapBookToResponse(b, nil)
			}
			return utils.NewPaginatedResponse(items, total, pg.Page, pg.Limit), nil
		})
	if err != nil {
		return nil, err
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

// invalidateAuthorCache also drops every cached book (list, single, search,
// per-author page) — books embed their authors, so a renamed author would
// otherwise stay stale inside cached book responses.
func (s *AuthorService) invalidateAuthorCache(ctx context.Context, id uint64) {
	_ = s.cache.Delete(ctx, cache.KeyAuthorSingle(id))
	for _, prefix := range []string{cache.PrefixAuthorBooks, cache.PrefixBookList, cache.PrefixSearch, "books:single:"} {
		_ = s.cache.DeletePrefix(ctx, prefix)
	}
}

func (s *AuthorService) invalidateAuthorListCache(ctx context.Context) {
	_ = s.cache.DeletePrefix(ctx, cache.PrefixAuthorList)
}
